package host

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"

	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/oidc"
)

const (
	sessionCookie = "nex_session"
	sessionTTL    = 30 * 24 * time.Hour
)

// Config is host runtime configuration.
type Config struct {
	StorageDir               string
	BaseURL                  string
	CDNBaseURL               string // optional CDN / mirror base for package download links
	MaxUploadBytes           int64
	ListenAddr               string
	AppDir                   string
	WebDir                   string // directory containing templates/ and static/
	PublishRateLimitMinutes  int    // successful publishes per user cooldown (default 30, max 60)
}

// Host bridges Nexus programs to HTTP, Postgres, FS, crypto, and templates.
type Host struct {
	DB     *database.DB
	Cfg    Config
	Logger *slog.Logger
	Env    *evaluator.Environment
	Router *chi.Mux

	RouteCount int
	metrics    hostMetrics

	templates map[string]*template.Template
	md        goldmark.Markdown
	policy    *bluemonday.Policy
	oidc      *oidc.Verifier
	limiter   *rateLimiter
	provenance ProvenanceVerifier

	forms sync.Map // request id -> *multipart.Form
}

// New creates a Host and registers builtins into the evaluator.
func New(db *database.DB, cfg Config, logger *slog.Logger) *Host {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = 64 << 20
	}
	if cfg.StorageDir == "" {
		cfg.StorageDir = "./storage"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.CDNBaseURL == "" {
		cfg.CDNBaseURL = cfg.BaseURL
	}
	cfg.CDNBaseURL = strings.TrimRight(cfg.CDNBaseURL, "/")
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.PublishRateLimitMinutes <= 0 {
		cfg.PublishRateLimitMinutes = 30
	}
	if cfg.PublishRateLimitMinutes > 60 {
		cfg.PublishRateLimitMinutes = 60
	}
	if cfg.WebDir != "" {
		if abs, err := filepath.Abs(cfg.WebDir); err == nil {
			cfg.WebDir = abs
		}
	}

	audiences := []string{cfg.BaseURL, "nex-registry"}
	if extra := os.Getenv("NEX_OIDC_AUDIENCE"); extra != "" {
		audiences = append(audiences, extra)
	}

	h := &Host{
		DB:     db,
		Cfg:    cfg,
		Logger: logger,
		Env:    evaluator.NewEnvironment(),
		Router: chi.NewRouter(),
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(goldhtml.WithHardWraps(), goldhtml.WithXHTML()),
		),
		policy:     bluemonday.UGCPolicy(),
		oidc:       oidc.DefaultVerifier(audiences...),
		limiter:    newRateLimiter(),
		provenance: StubProvenanceVerifier{},
	}
	h.registerBuiltins()
	h.setupRouterBase()
	return h
}

func (h *Host) setupRouterBase() {
	r := h.Router
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(h.rateLimitMiddleware)
	r.Use(h.requestLogger)

	if h.Cfg.WebDir != "" {
		staticDir := filepath.Join(h.Cfg.WebDir, "static")
		if st, err := os.Stat(staticDir); err == nil && st.IsDir() {
			fileServer := http.FileServer(http.Dir(staticDir))
			r.Handle("/static/*", http.StripPrefix("/static/", fileServer))
		}
	}
	h.registerMetricsRoutes()
	h.registerGitHubOAuthRoutes()
}

func (h *Host) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := ww.Status()
		// Skip scraping noise from metrics path in request totals? Keep all for accuracy.
		h.observeRequest(status)
		if strings.HasSuffix(r.URL.Path, "/download") && status >= 200 && status < 300 {
			h.IncDownload()
		}
		h.Logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

// Handler returns the HTTP handler.
func (h *Host) Handler() http.Handler { return h.Router }

func (h *Host) registerBuiltins() {
	b := map[string]*evaluator.Builtin{}
	cfg := h.Cfg

	b["env"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("env", 1, args); err != nil {
			return err
		}
		key, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "env expects string"}
		}
		return &evaluator.String{Value: os.Getenv(key)}
	}}
	b["config"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		out := evaluator.NewHash()
		out.SetString("storage_dir", &evaluator.String{Value: cfg.StorageDir})
		out.SetString("base_url", &evaluator.String{Value: cfg.BaseURL})
		out.SetString("cdn_base_url", &evaluator.String{Value: cfg.CDNBaseURL})
		out.SetString("listen_addr", &evaluator.String{Value: cfg.ListenAddr})
		out.SetString("max_upload_bytes", &evaluator.Integer{Value: cfg.MaxUploadBytes})
		out.SetString("publish_rate_limit_minutes", &evaluator.Integer{Value: int64(cfg.PublishRateLimitMinutes)})
		out.SetString("app_dir", &evaluator.String{Value: cfg.AppDir})
		out.SetString("web_dir", &evaluator.String{Value: cfg.WebDir})
		return out
	}}

	b["http_get"] = h.routeBuiltin("GET")
	b["http_post"] = h.routeBuiltin("POST")
	b["http_put"] = h.routeBuiltin("PUT")
	b["http_patch"] = h.routeBuiltin("PATCH")
	b["http_delete"] = h.routeBuiltin("DELETE")
	b["http_not_found"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("http_not_found", 1, args); err != nil {
			return err
		}
		fn, ok := args[0].(*evaluator.Function)
		if !ok {
			return &evaluator.Error{Message: "http_not_found expects function"}
		}
		h.Router.NotFound(h.wrapHandler(fn, false))
		return evaluator.NULL
	}}

	b["json"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("json", 1, args); err != nil {
			return err
		}
		status := int64(200)
		payload := args[0]
		if len(args) == 2 {
			if s, ok := AsInt(args[0]); ok {
				status = s
				payload = args[1]
			}
		}
		out := evaluator.NewHash()
		out.SetString("status", &evaluator.Integer{Value: status})
		out.SetString("json", payload)
		return out
	}}
	b["html"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("html", 2, args); err != nil {
			return err
		}
		status := int64(200)
		pageIdx, dataIdx := 0, 1
		if len(args) == 3 {
			if s, ok := AsInt(args[0]); ok {
				status = s
				pageIdx, dataIdx = 1, 2
			}
		}
		page, ok := AsString(args[pageIdx])
		if !ok {
			return &evaluator.Error{Message: "html page must be string"}
		}
		out := evaluator.NewHash()
		out.SetString("status", &evaluator.Integer{Value: status})
		out.SetString("html", &evaluator.String{Value: page})
		out.SetString("data", args[dataIdx])
		return out
	}}
	b["redirect"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("redirect", 1, args); err != nil {
			return err
		}
		url, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "redirect expects string"}
		}
		out := evaluator.NewHash()
		out.SetString("status", &evaluator.Integer{Value: 303})
		out.SetString("redirect", &evaluator.String{Value: url})
		return out
	}}
	b["file_response"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("file_response", 2, args); err != nil {
			return err
		}
		path, ok1 := AsString(args[0])
		name, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "file_response expects (path, filename [, content_type])"}
		}
		ct := "application/octet-stream"
		if len(args) >= 3 {
			if s, ok := AsString(args[2]); ok {
				ct = s
			}
		}
		out := evaluator.NewHash()
		out.SetString("status", &evaluator.Integer{Value: 200})
		out.SetString("file", &evaluator.String{Value: path})
		out.SetString("filename", &evaluator.String{Value: name})
		out.SetString("content_type", &evaluator.String{Value: ct})
		return out
	}}
	b["with_cookie"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("with_cookie", 3, args); err != nil {
			return err
		}
		resp, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "with_cookie expects response hash"}
		}
		name, ok1 := AsString(args[1])
		value, ok2 := AsString(args[2])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "cookie name/value must be strings"}
		}
		cookie := evaluator.NewHash()
		cookie.SetString("name", &evaluator.String{Value: name})
		cookie.SetString("value", &evaluator.String{Value: value})
		cookie.SetString("max_age", &evaluator.Integer{Value: int64(sessionTTL.Seconds())})
		resp.SetString("set_cookie", cookie)
		return resp
	}}
	b["clear_cookie"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("clear_cookie", 2, args); err != nil {
			return err
		}
		resp, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "clear_cookie expects response hash"}
		}
		name, ok1 := AsString(args[1])
		if !ok1 {
			return &evaluator.Error{Message: "cookie name must be string"}
		}
		resp.SetString("clear_cookie", &evaluator.String{Value: name})
		return resp
	}}
	b["with_header"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("with_header", 3, args); err != nil {
			return err
		}
		resp, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "with_header expects response hash"}
		}
		name, ok1 := AsString(args[1])
		value, ok2 := AsString(args[2])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "with_header name/value must be strings"}
		}
		headers := resp.Get("headers")
		var h *evaluator.Hash
		if existing, ok := headers.(*evaluator.Hash); ok {
			h = existing
		} else {
			h = evaluator.NewHash()
			resp.SetString("headers", h)
		}
		h.SetString(name, &evaluator.String{Value: value})
		return resp
	}}

	b["json_parse"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("json_parse", 1, args); err != nil {
			return err
		}
		s, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "json_parse expects string"}
		}
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return &evaluator.Error{Message: "json_parse: " + err.Error()}
		}
		return FromGo(v)
	}}
	b["json_stringify"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("json_stringify", 1, args); err != nil {
			return err
		}
		data, err := json.Marshal(ToGo(args[0]))
		if err != nil {
			return &evaluator.Error{Message: "json_stringify: " + err.Error()}
		}
		return &evaluator.String{Value: string(data)}
	}}
	b["toml_parse"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("toml_parse", 1, args); err != nil {
			return err
		}
		s, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "toml_parse expects string"}
		}
		var v map[string]any
		if err := toml.Unmarshal([]byte(s), &v); err != nil {
			return &evaluator.Error{Message: "toml_parse: " + err.Error()}
		}
		return FromGo(v)
	}}

	b["sha256"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("sha256", 1, args); err != nil {
			return err
		}
		s, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "sha256 expects string"}
		}
		sum := sha256.Sum256([]byte(s))
		return &evaluator.String{Value: hex.EncodeToString(sum[:])}
	}}
	b["sha256_bytes"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("sha256_bytes", 1, args); err != nil {
			return err
		}
		s, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "sha256_bytes expects string/bytes"}
		}
		sum := sha256.Sum256([]byte(s))
		return &evaluator.String{Value: "sha256:" + hex.EncodeToString(sum[:])}
	}}
	b["bcrypt_hash"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("bcrypt_hash", 1, args); err != nil {
			return err
		}
		s, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "bcrypt_hash expects string"}
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: string(hash)}
	}}
	b["bcrypt_check"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("bcrypt_check", 2, args); err != nil {
			return err
		}
		hash, ok1 := AsString(args[0])
		pw, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "bcrypt_check expects (hash, password)"}
		}
		return FromGo(bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil)
	}}
	b["random_hex"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		n := int64(32)
		if len(args) == 1 {
			if v, ok := AsInt(args[0]); ok {
				n = v
			}
		}
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: hex.EncodeToString(buf)}
	}}
	b["gravatar_url"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("gravatar_url", 1, args); err != nil {
			return err
		}
		email, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "gravatar_url expects string"}
		}
		normalized := strings.ToLower(strings.TrimSpace(email))
		sum := sha256.Sum256([]byte(normalized))
		return &evaluator.String{Value: "https://www.gravatar.com/avatar/" + hex.EncodeToString(sum[:]) + "?d=identicon&s=160"}
	}}

	b["fs_read"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("fs_read", 1, args); err != nil {
			return err
		}
		path, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "fs_read expects string"}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: string(data)}
	}}
	b["fs_write"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("fs_write", 2, args); err != nil {
			return err
		}
		path, ok1 := AsString(args[0])
		data, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "fs_write expects (path, data)"}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		if err := os.WriteFile(path, []byte(data), 0o640); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
	b["fs_exists"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("fs_exists", 1, args); err != nil {
			return err
		}
		path, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "fs_exists expects string"}
		}
		_, err := os.Stat(path)
		return FromGo(err == nil)
	}}
	b["fs_mkdir"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("fs_mkdir", 1, args); err != nil {
			return err
		}
		path, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "fs_mkdir expects string"}
		}
		if err := os.MkdirAll(path, 0o750); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
	b["path_join"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			s, ok := AsString(a)
			if !ok {
				return &evaluator.Error{Message: "path_join expects strings"}
			}
			parts = append(parts, s)
		}
		return &evaluator.String{Value: filepath.Join(parts...)}
	}}

	b["markdown_html"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("markdown_html", 1, args); err != nil {
			return err
		}
		src, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "markdown_html expects string"}
		}
		var buf bytes.Buffer
		if err := h.md.Convert([]byte(src), &buf); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: string(h.policy.SanitizeBytes(buf.Bytes()))}
	}}

	b["re_match"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("re_match", 2, args); err != nil {
			return err
		}
		pat, ok1 := AsString(args[0])
		s, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "re_match expects (pattern, string)"}
		}
		ok, err := regexp.MatchString(pat, s)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(ok)
	}}

	b["multipart_text"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("multipart_text", 2, args); err != nil {
			return err
		}
		reqHash, ok := args[0].(*evaluator.Hash)
		field, ok2 := AsString(args[1])
		if !ok || !ok2 {
			return &evaluator.Error{Message: "multipart_text expects (req, field)"}
		}
		rid := HashGetString(reqHash, "request_id")
		raw, ok := h.forms.Load(rid)
		if !ok {
			return &evaluator.String{Value: ""}
		}
		form := raw.(*multipart.Form)
		vals := form.Value[field]
		if len(vals) == 0 {
			return &evaluator.String{Value: ""}
		}
		return &evaluator.String{Value: vals[0]}
	}}
	b["multipart_file"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("multipart_file", 2, args); err != nil {
			return err
		}
		reqHash, ok := args[0].(*evaluator.Hash)
		field, ok2 := AsString(args[1])
		if !ok || !ok2 {
			return &evaluator.Error{Message: "multipart_file expects (req, field)"}
		}
		rid := HashGetString(reqHash, "request_id")
		raw, ok := h.forms.Load(rid)
		if !ok {
			return evaluator.NULL
		}
		form := raw.(*multipart.Form)
		files := form.File[field]
		if len(files) == 0 {
			return evaluator.NULL
		}
		fh := files[0]
		f, err := fh.Open()
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		sum := sha256.Sum256(data)
		out := evaluator.NewHash()
		out.SetString("filename", &evaluator.String{Value: fh.Filename})
		out.SetString("size", &evaluator.Integer{Value: int64(len(data))})
		out.SetString("data", &evaluator.String{Value: string(data)})
		out.SetString("sha256", &evaluator.String{Value: "sha256:" + hex.EncodeToString(sum[:])})
		return out
	}}

	h.registerDBBuiltins(b)
	h.registerOwnershipBuiltins(b)
	h.registerSecurityBuiltins(b)
	h.registerAuthBuiltins(b)
	h.registerDocsBuiltins(b)
	h.registerMetricsBuiltins(b)
	h.registerUnsupportedDesignBuiltins(b)

	for name, builtin := range b {
		evaluator.ExtraBuiltins[name] = builtin
	}
}
