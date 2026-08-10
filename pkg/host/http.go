package host

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/oidc"
)

func (h *Host) routeBuiltin(method string) *evaluator.Builtin {
	return &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("http_"+strings.ToLower(method), 2, args); err != nil {
			return err
		}
		path, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "path must be string"}
		}
		fn, ok := args[1].(*evaluator.Function)
		if !ok {
			return &evaluator.Error{Message: "handler must be function"}
		}
		requireAuth := strings.Contains(path, "/settings") ||
			strings.HasPrefix(path, "/getting-started") ||
			strings.HasPrefix(path, "/api/user") ||
			strings.HasPrefix(path, "/orgs/new") ||
			strings.HasPrefix(path, "/invites/") ||
			(method != "GET" && (strings.Contains(path, "/owners") ||
				strings.Contains(path, "/members") ||
				strings.Contains(path, "/teams") ||
				strings.Contains(path, "/transfer") ||
				strings.HasPrefix(path, "/api/v1/orgs") ||
				strings.HasPrefix(path, "/orgs/"))) ||
			path == "/api/v1/publish" ||
			path == "/api/v1/trusted-publishing/token" ||
			strings.HasPrefix(path, "/admin") ||
			strings.Contains(path, "/yank") ||
			strings.Contains(path, "/unyank") ||
			strings.Contains(path, "/unpublish")

		handler := h.wrapHandler(fn, requireAuth)
		switch method {
		case "GET":
			h.Router.Get(path, handler)
		case "POST":
			h.Router.Post(path, handler)
		case "PUT":
			h.Router.Put(path, handler)
		case "PATCH":
			h.Router.Patch(path, handler)
		case "DELETE":
			h.Router.Delete(path, handler)
		}
		h.RouteCount++
		return evaluator.NULL
	}}
}

func (h *Host) wrapHandler(fn *evaluator.Function, requireAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, via, claims, apiKey := h.resolveAuth(r)
		if requireAuth && user == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
			return
		}

		reqObj := h.buildRequest(r, user, via)
		if apiKey != nil {
			reqObj.SetString("api_key_scope", &evaluator.String{Value: apiKey.Scope})
			reqObj.SetString("api_key_id", &evaluator.Integer{Value: apiKey.ID})
		} else {
			reqObj.SetString("api_key_scope", &evaluator.String{Value: ""})
			reqObj.SetString("api_key_id", evaluator.NULL)
		}
		if claims != nil {
			reqObj.SetString("oidc", oidcClaimsHash(claims))
		}

		// Parse multipart for publish / form posts
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(h.Cfg.MaxUploadBytes); err == nil && r.MultipartForm != nil {
				rid := middleware.GetReqID(r.Context())
				h.forms.Store(rid, r.MultipartForm)
				defer func() {
					h.forms.Delete(rid)
					_ = r.MultipartForm.RemoveAll()
				}()
			}
		} else if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			_ = r.ParseForm()
			form := evaluator.NewHash()
			for k, vals := range r.PostForm {
				if len(vals) > 0 {
					form.SetString(k, &evaluator.String{Value: vals[0]})
				}
			}
			reqObj.SetString("form", form)
		} else if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			reqObj.SetString("body", &evaluator.String{Value: string(body)})
		}

		result := evaluator.ApplyFunction(fn, []evaluator.Object{reqObj})
		if errObj, ok := result.(*evaluator.Error); ok {
			h.Logger.Error("nex handler error", "path", r.URL.Path, "error", errObj.Message)
			if wantsHTML(r) {
				h.writeHTML(w, r, "error.html", 500, map[string]any{
					"Title":   "Error",
					"Status":  500,
					"Message": "Internal error",
				})
				return
			}
			writeJSON(w, 500, map[string]string{"error": "internal error", "details": errObj.Message})
			return
		}
		h.writeResponse(w, r, result, user)
	}
}

func (h *Host) buildRequest(r *http.Request, user *database.User, via string) *evaluator.Hash {
	req := evaluator.NewHash()
	req.SetString("method", &evaluator.String{Value: r.Method})
	req.SetString("path", &evaluator.String{Value: r.URL.Path})
	req.SetString("request_id", &evaluator.String{Value: middleware.GetReqID(r.Context())})
	req.SetString("auth_via", &evaluator.String{Value: via})

	query := evaluator.NewHash()
	for k, vals := range r.URL.Query() {
		if len(vals) > 0 {
			query.SetString(k, &evaluator.String{Value: vals[0]})
		}
	}
	req.SetString("query", query)

	headers := evaluator.NewHash()
	for k, vals := range r.Header {
		if len(vals) > 0 {
			headers.SetString(strings.ToLower(k), &evaluator.String{Value: vals[0]})
		}
	}
	req.SetString("headers", headers)

	params := evaluator.NewHash()
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		for i, key := range rctx.URLParams.Keys {
			params.SetString(key, &evaluator.String{Value: rctx.URLParams.Values[i]})
		}
	}
	req.SetString("params", params)

	cookies := evaluator.NewHash()
	for _, c := range r.Cookies() {
		cookies.SetString(c.Name, &evaluator.String{Value: c.Value})
	}
	req.SetString("cookies", cookies)

	if user != nil {
		req.SetString("user", userToHash(user))
	} else {
		req.SetString("user", evaluator.NULL)
	}

	req.SetString("form", evaluator.NewHash())
	req.SetString("body", &evaluator.String{Value: ""})
	return req
}

func userToHash(u *database.User) *evaluator.Hash {
	h := evaluator.NewHash()
	h.SetString("id", &evaluator.Integer{Value: u.ID})
	h.SetString("ID", &evaluator.Integer{Value: u.ID})
	h.SetString("username", &evaluator.String{Value: u.Username})
	h.SetString("Username", &evaluator.String{Value: u.Username})
	h.SetString("email", &evaluator.String{Value: u.Email})
	h.SetString("Email", &evaluator.String{Value: u.Email})
	h.SetString("password_hash", &evaluator.String{Value: u.PasswordHash})
	h.SetString("avatar_url", &evaluator.String{Value: u.AvatarURL})
	h.SetString("AvatarURL", &evaluator.String{Value: u.AvatarURL})
	h.SetString("bio", &evaluator.String{Value: u.Bio})
	h.SetString("Bio", &evaluator.String{Value: u.Bio})
	h.SetString("use_gravatar", FromGo(u.UseGravatar))
	h.SetString("UseGravatar", FromGo(u.UseGravatar))
	h.SetString("github_login", &evaluator.String{Value: u.GitHubLogin})
	h.SetString("GitHubLogin", &evaluator.String{Value: u.GitHubLogin})
	if u.GitHubID != nil {
		h.SetString("github_id", &evaluator.Integer{Value: *u.GitHubID})
		h.SetString("GitHubID", &evaluator.Integer{Value: *u.GitHubID})
	} else {
		h.SetString("github_id", evaluator.NULL)
		h.SetString("GitHubID", evaluator.NULL)
	}
	h.SetString("has_password", FromGo(strings.TrimSpace(u.PasswordHash) != ""))
	h.SetString("HasPassword", FromGo(strings.TrimSpace(u.PasswordHash) != ""))
	h.SetString("email_verified", FromGo(u.EmailVerified))
	h.SetString("EmailVerified", FromGo(u.EmailVerified))
	h.SetString("totp_enabled", FromGo(u.TOTPEnabled))
	h.SetString("TOTPEnabled", FromGo(u.TOTPEnabled))
	h.SetString("is_admin", FromGo(u.IsAdmin))
	h.SetString("IsAdmin", FromGo(u.IsAdmin))
	h.SetString("created_at", &evaluator.String{Value: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	h.SetString("CreatedAt", &evaluator.String{Value: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	return h
}

func publicUserHash(u *database.User) *evaluator.Hash {
	h := evaluator.NewHash()
	h.SetString("id", &evaluator.Integer{Value: u.ID})
	h.SetString("ID", &evaluator.Integer{Value: u.ID})
	h.SetString("username", &evaluator.String{Value: u.Username})
	h.SetString("Username", &evaluator.String{Value: u.Username})
	h.SetString("email", &evaluator.String{Value: u.Email})
	h.SetString("Email", &evaluator.String{Value: u.Email})
	h.SetString("avatar_url", &evaluator.String{Value: u.AvatarURL})
	h.SetString("AvatarURL", &evaluator.String{Value: u.AvatarURL})
	h.SetString("bio", &evaluator.String{Value: u.Bio})
	h.SetString("Bio", &evaluator.String{Value: u.Bio})
	h.SetString("use_gravatar", FromGo(u.UseGravatar))
	h.SetString("UseGravatar", FromGo(u.UseGravatar))
	h.SetString("github_login", &evaluator.String{Value: u.GitHubLogin})
	h.SetString("GitHubLogin", &evaluator.String{Value: u.GitHubLogin})
	if u.GitHubID != nil {
		h.SetString("github_id", &evaluator.Integer{Value: *u.GitHubID})
		h.SetString("GitHubID", &evaluator.Integer{Value: *u.GitHubID})
	} else {
		h.SetString("github_id", evaluator.NULL)
		h.SetString("GitHubID", evaluator.NULL)
	}
	h.SetString("has_password", FromGo(strings.TrimSpace(u.PasswordHash) != ""))
	h.SetString("HasPassword", FromGo(strings.TrimSpace(u.PasswordHash) != ""))
	h.SetString("email_verified", FromGo(u.EmailVerified))
	h.SetString("EmailVerified", FromGo(u.EmailVerified))
	h.SetString("totp_enabled", FromGo(u.TOTPEnabled))
	h.SetString("TOTPEnabled", FromGo(u.TOTPEnabled))
	h.SetString("is_admin", FromGo(u.IsAdmin))
	h.SetString("IsAdmin", FromGo(u.IsAdmin))
	h.SetString("created_at", &evaluator.String{Value: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	h.SetString("CreatedAt", &evaluator.String{Value: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	return h
}

func (h *Host) resolveAuth(r *http.Request) (*database.User, string, *oidc.Claims, *database.APIKey) {
	if h.DB == nil {
		return nil, "", nil, nil
	}
	if key := strings.TrimSpace(r.Header.Get("X-Api-Key")); key != "" {
		if u, k, err := h.DB.UserByAPIKey(r.Context(), key); err == nil {
			return u, "api_key", nil, k
		}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token := strings.TrimSpace(auth[7:])
		if strings.HasPrefix(token, "nex_") {
			if u, k, err := h.DB.UserByAPIKey(r.Context(), token); err == nil {
				return u, "api_key", nil, k
			}
		}
		if strings.HasPrefix(token, "nxs_") || strings.HasPrefix(token, "nxt_") {
			if u, err := h.DB.UserBySessionToken(r.Context(), token); err == nil {
				via := "session"
				if strings.HasPrefix(token, "nxt_") {
					via = "trusted_publisher_token"
				}
				return u, via, nil, nil
			}
		}
		if looksLikeJWT(token) && h.oidc != nil {
			claims, err := h.oidc.Verify(r.Context(), token)
			if err == nil {
				tp, err := h.DB.MatchTrustedPublisherByClaims(
					r.Context(),
					"",
					claims.RepositoryOwnerName(),
					claims.RepositoryName(),
					claims.WorkflowFilename(),
					claims.Environment,
				)
				if err == nil && tp != nil {
					if u, err := h.DB.GetUserByID(r.Context(), tp.UserID); err == nil {
						return u, "github_oidc", claims, nil
					}
				} else {
					reason := h.DB.ExplainTrustedPublisherMismatch(
						r.Context(),
						"",
						claims.RepositoryOwnerName(),
						claims.RepositoryName(),
						claims.WorkflowFilename(),
						claims.Environment,
					)
					_ = h.DB.RecordTrustedPublisherFailure(
						r.Context(),
						claims.RepositoryOwnerName(),
						claims.RepositoryName(),
						reason,
					)
				}
			} else {
				h.Logger.Debug("github oidc verify failed", "error", err.Error())
			}
		}
		if u, err := h.DB.UserBySessionToken(r.Context(), token); err == nil {
			return u, "session", nil, nil
		}
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if u, err := h.DB.UserBySessionToken(r.Context(), c.Value); err == nil {
			return u, "session", nil, nil
		}
	}
	return nil, "", nil, nil
}

func looksLikeJWT(token string) bool {
	if strings.Count(token, ".") != 2 {
		return false
	}
	if strings.HasPrefix(token, "nex_") || strings.HasPrefix(token, "nxs_") || strings.HasPrefix(token, "nxt_") {
		return false
	}
	return true
}

func oidcClaimsHash(c *oidc.Claims) *evaluator.Hash {
	h := evaluator.NewHash()
	h.SetString("issuer", &evaluator.String{Value: c.Issuer})
	h.SetString("subject", &evaluator.String{Value: c.Subject})
	h.SetString("repository", &evaluator.String{Value: c.Repository})
	h.SetString("repository_owner", &evaluator.String{Value: c.RepositoryOwnerName()})
	h.SetString("repository_name", &evaluator.String{Value: c.RepositoryName()})
	h.SetString("workflow_filename", &evaluator.String{Value: c.WorkflowFilename()})
	h.SetString("environment", &evaluator.String{Value: c.Environment})
	h.SetString("ref", &evaluator.String{Value: c.Ref})
	h.SetString("actor", &evaluator.String{Value: c.Actor})
	h.SetString("job_workflow_ref", &evaluator.String{Value: c.JobWorkflowRef})
	return h
}

func (h *Host) writeResponse(w http.ResponseWriter, r *http.Request, result evaluator.Object, user *database.User) {
	hash, ok := result.(*evaluator.Hash)
	if !ok {
		writeJSON(w, 200, map[string]any{"result": ToGo(result)})
		return
	}

	if c := hash.Get("set_cookie"); c != evaluator.NULL {
		if ch, ok := c.(*evaluator.Hash); ok {
			http.SetCookie(w, &http.Cookie{
				Name:     HashGetString(ch, "name"),
				Value:    HashGetString(ch, "value"),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(mustInt(ch.Get("max_age"), int64(sessionTTL.Seconds()))),
			})
		}
	}
	if c := hash.Get("clear_cookie"); c != evaluator.NULL {
		if name, ok := AsString(c); ok {
			http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
		}
	}

	status := int(mustInt(hash.Get("status"), 200))

	if hdrs := hash.Get("headers"); hdrs != evaluator.NULL {
		if hh, ok := hdrs.(*evaluator.Hash); ok {
			if m, ok := ToGo(hh).(map[string]any); ok {
				for k, v := range m {
					w.Header().Set(k, fmt.Sprint(v))
				}
			}
		}
	}

	if redir := hash.Get("redirect"); redir != evaluator.NULL {
		if url, ok := AsString(redir); ok {
			http.Redirect(w, r, url, status)
			return
		}
	}

	if file := hash.Get("file"); file != evaluator.NULL {
		path, _ := AsString(file)
		name := HashGetString(hash, "filename")
		ct := HashGetString(hash, "content_type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		f, err := os.Open(path)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "file not found"})
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", ct)
		if name != "" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		}
		w.WriteHeader(status)
		_, _ = io.Copy(w, f)
		return
	}

	if page := hash.Get("html"); page != evaluator.NULL {
		pageName, _ := AsString(page)
		data := map[string]any{}
		if d := hash.Get("data"); d != evaluator.NULL {
			if m, ok := ToGo(d).(map[string]any); ok {
				data = m
			}
		}
		if user != nil {
			data["CurrentUser"] = publicUserMap(user)
		}
		h.writeHTML(w, r, pageName, status, data)
		return
	}

	if j := hash.Get("json"); j != evaluator.NULL {
		writeJSON(w, status, ToGo(j))
		return
	}

	if body := hash.Get("body"); body != evaluator.NULL {
		ct := HashGetString(hash, "content_type")
		if ct == "" {
			ct = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body.Inspect()))
		return
	}

	writeJSON(w, status, ToGo(hash))
}

func publicUserMap(u *database.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"ID":             u.ID,
		"username":       u.Username,
		"Username":       u.Username,
		"avatar_url":     u.AvatarURL,
		"AvatarURL":      u.AvatarURL,
		"bio":            u.Bio,
		"Bio":            u.Bio,
		"use_gravatar":   u.UseGravatar,
		"UseGravatar":    u.UseGravatar,
		"email_verified": u.EmailVerified,
		"EmailVerified":  u.EmailVerified,
		"created_at":     u.CreatedAt,
		"CreatedAt":      u.CreatedAt,
	}
}

func mustInt(obj evaluator.Object, def int64) int64 {
	if i, ok := AsInt(obj); ok {
		return i
	}
	return def
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return !strings.HasPrefix(r.URL.Path, "/api/")
	}
	return strings.Contains(accept, "text/html")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(payload)
}
