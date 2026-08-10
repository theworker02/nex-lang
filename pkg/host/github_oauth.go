package host

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	oauthStateCookie = "nex_oauth_state"
	oauthNextCookie  = "nex_oauth_next"
	oauthModeCookie  = "nex_oauth_mode"
)

type githubTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type githubUserResp struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmailResp struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (h *Host) githubOAuthConfigured() bool {
	return strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")) != "" &&
		strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")) != ""
}

func (h *Host) githubRedirectURI() string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_REDIRECT_URI")); v != "" {
		return v
	}
	base := strings.TrimRight(h.Cfg.BaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/auth/github/callback"
}

func (h *Host) registerGitHubOAuthRoutes() {
	h.Router.Get("/auth/github", h.handleGitHubOAuthStart)
	h.Router.Get("/auth/github/callback", h.handleGitHubOAuthCallback)
}

func (h *Host) handleGitHubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.githubOAuthConfigured() {
		http.Redirect(w, r, "/login?error=github_not_configured", http.StatusSeeOther)
		return
	}
	state, err := randomHex(24)
	if err != nil {
		http.Error(w, "could not start oauth", http.StatusInternalServerError)
		return
	}
	next := strings.TrimSpace(r.URL.Query().Get("next"))
	if next == "" || !strings.HasPrefix(next, "/") {
		next = "/settings"
	}
	mode := "login"
	if u, _, _, _ := h.resolveAuth(r); u != nil {
		mode = "link"
		next = "/settings"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthNextCookie,
		Value:    next,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthModeCookie,
		Value:    mode,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	q := url.Values{}
	q.Set("client_id", os.Getenv("GITHUB_CLIENT_ID"))
	q.Set("redirect_uri", h.githubRedirectURI())
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

func (h *Host) handleGitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !h.githubOAuthConfigured() {
		http.Redirect(w, r, "/login?error=github_not_configured", http.StatusSeeOther)
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Redirect(w, r, "/login?error=github_denied", http.StatusSeeOther)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	cState, err := r.Cookie(oauthStateCookie)
	if err != nil || cState.Value == "" || state == "" || subtleEqual(cState.Value, state) == false {
		http.Redirect(w, r, "/login?error=github_state", http.StatusSeeOther)
		return
	}

	next := "/settings"
	if c, err := r.Cookie(oauthNextCookie); err == nil && strings.HasPrefix(c.Value, "/") {
		next = c.Value
	}
	mode := "login"
	if c, err := r.Cookie(oauthModeCookie); err == nil && c.Value != "" {
		mode = c.Value
	}

	clearOAuthCookies(w)

	token, err := exchangeGitHubCode(code, h.githubRedirectURI())
	if err != nil {
		h.Logger.Error("github oauth token", "error", err)
		http.Redirect(w, r, "/login?error=github_token", http.StatusSeeOther)
		return
	}
	ghUser, email, err := fetchGitHubProfile(token)
	if err != nil {
		h.Logger.Error("github oauth profile", "error", err)
		http.Redirect(w, r, "/login?error=github_profile", http.StatusSeeOther)
		return
	}
	if h.DB == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	if mode == "link" {
		current, _, _, _ := h.resolveAuth(r)
		if current == nil {
			http.Redirect(w, r, "/login?error=github_link_login", http.StatusSeeOther)
			return
		}
		if _, err := h.DB.LinkGitHubAccount(r.Context(), current.ID, ghUser.ID, ghUser.Login, ghUser.AvatarURL); err != nil {
			h.Logger.Error("github link", "error", err)
			http.Redirect(w, r, "/settings?flash=github_link_failed", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings?flash=github_linked", http.StatusSeeOther)
		return
	}

	user, err := h.DB.UpsertGitHubUser(r.Context(), ghUser.ID, ghUser.Login, email, ghUser.AvatarURL)
	if err != nil {
		h.Logger.Error("github upsert", "error", err)
		http.Redirect(w, r, "/login?error=github_user", http.StatusSeeOther)
		return
	}
	sess, _, err := h.DB.CreateSession(r.Context(), user.ID, sessionTTL)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func clearOAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{oauthStateCookie, oauthNextCookie, oauthModeCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	}
}

func exchangeGitHubCode(code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", os.Getenv("GITHUB_CLIENT_ID"))
	form.Set("client_secret", os.Getenv("GITHUB_CLIENT_SECRET"))
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr githubTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		if tr.Error != "" {
			return "", fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
		}
		return "", fmt.Errorf("empty access token")
	}
	return tr.AccessToken, nil
}

func fetchGitHubProfile(token string) (*githubUserResp, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nex-registry")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("github user status %d", resp.StatusCode)
	}
	var u githubUserResp
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, "", err
	}
	email := strings.TrimSpace(u.Email)
	if email == "" {
		email = fetchPrimaryGitHubEmail(client, token)
	}
	return &u, email, nil
}

func fetchPrimaryGitHubEmail(client *http.Client, token string) string {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nex-registry")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var emails []githubEmailResp
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}
	var fallback string
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		if e.Primary {
			return e.Email
		}
		if fallback == "" {
			fallback = e.Email
		}
	}
	return fallback
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
