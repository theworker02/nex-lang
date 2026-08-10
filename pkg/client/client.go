// Package client is the HTTP client for nex-registry (publish/install/login/yank).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nex-lang/pkg/config"
)

const defaultTimeout = 60 * time.Second

// Client talks to a remote Nexus package registry.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Options configures a registry client.
type Options struct {
	BaseURL string
	Token   string
}

// New creates a registry client from options (empty fields use config/env defaults).
func New(opts Options) *Client {
	base := strings.TrimSpace(opts.BaseURL)
	token := strings.TrimSpace(opts.Token)
	if base == "" || token == "" {
		if cfg, err := config.Load(); err == nil {
			if base == "" {
				base = cfg.RegistryBase()
			}
			if token == "" {
				token = cfg.AuthToken()
			}
		}
	}
	if base == "" {
		base = config.DefaultRegistryURL
	}
	base = strings.TrimRight(base, "/")

	return &Client{
		baseURL: base,
		token:   token,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// BaseURL returns the configured registry base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// SetToken sets the bearer token used for authenticated requests.
func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
}

// Token returns the configured bearer token.
func (c *Client) Token() string {
	return c.token
}

// LoginRequest is the JSON body for POST /api/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// User is a subset of the public user object returned by auth endpoints.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// LoginResult is returned by Login.
type LoginResult struct {
	Message string `json:"message"`
	User    User   `json:"user"`
	Token   string `json:"token"`
}

// Login authenticates with username/email + password and returns a session token (nxs_…).
func (c *Client) Login(ctx context.Context, login, password string) (*LoginResult, error) {
	body, err := json.Marshal(LoginRequest{Login: login, Password: password})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}
	var result LoginResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("login response missing token")
	}
	c.token = result.Token
	return &result, nil
}

// Profile fetches the authenticated user's profile (validates a stored token).
func (c *Client) Profile(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/user/profile", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read profile response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}
	var wrap struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	return &wrap.User, nil
}

// CreateAPIKey mints a long-lived API key (nex_…) using a session token.
func (c *Client) CreateAPIKey(ctx context.Context, name string) (plaintext string, err error) {
	if name == "" {
		name = "cli"
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/user/api-keys", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read api key response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apiError(resp.StatusCode, data)
	}
	var wrap struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return "", fmt.Errorf("decode api key response: %w", err)
	}
	if wrap.APIKey == "" {
		return "", fmt.Errorf("api key response missing api_key")
	}
	return wrap.APIKey, nil
}

// VersionInfo is a published package version (handles Go field-name hashes from the registry).
type VersionInfo struct {
	Version  string
	Checksum string
	Filename string
	Yanked   bool
	Reason   string
}

// PackageVersionResponse is GET /api/v1/packages/{name}/{version}.
type PackageVersionResponse struct {
	Package     map[string]any `json:"package"`
	Version     map[string]any `json:"version"`
	DownloadURL string         `json:"download_url"`
}

// ResolvePackage returns metadata for name@version. Empty version picks the newest non-yanked release.
func (c *Client) ResolvePackage(ctx context.Context, name, version string) (*VersionInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if version == "" {
		return c.resolveLatest(ctx, name)
	}

	endpoint := fmt.Sprintf("%s/api/v1/packages/%s/%s", c.baseURL, url.PathEscape(name), url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query registry: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package %s@%s not found", name, version)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}
	var wrap PackageVersionResponse
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("decode package version: %w", err)
	}
	info := versionFromMap(wrap.Version)
	if info.Version == "" {
		info.Version = version
	}
	if info.Checksum == "" {
		return nil, fmt.Errorf("registry response for %s@%s is missing checksum", name, version)
	}
	return info, nil
}

func (c *Client) resolveLatest(ctx context.Context, name string) (*VersionInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/packages/%s", c.baseURL, url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query registry: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package %q not found in registry", name)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}
	var wrap struct {
		Versions []map[string]any `json:"versions"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("decode package: %w", err)
	}
	for _, raw := range wrap.Versions {
		info := versionFromMap(raw)
		if info.Version == "" || info.Yanked {
			continue
		}
		if info.Checksum == "" {
			continue
		}
		return info, nil
	}
	return nil, fmt.Errorf("package %q has no installable (non-yanked) versions", name)
}

// DownloadPackage downloads the .nex artifact for name@version to destPath.
func (c *Client) DownloadPackage(ctx context.Context, name, version, destPath string) error {
	if name == "" || version == "" {
		return fmt.Errorf("package name and version are required")
	}
	if destPath == "" {
		return fmt.Errorf("destination path is required")
	}
	endpoint := fmt.Sprintf("%s/api/v1/packages/%s/%s/download",
		c.baseURL, url.PathEscape(name), url.PathEscape(version))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("package %s@%s not found", name, version)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return apiError(resp.StatusCode, body)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write download: %w", err)
	}
	return nil
}

// PublishOptions configures multipart publish to POST /api/v1/publish.
type PublishOptions struct {
	ManifestPath string // nexus.toml
	PackagePath  string // .nex artifact
	ReadmePath   string // optional
}

// PublishResult is returned after a successful publish.
type PublishResult struct {
	Message     string `json:"message"`
	Checksum    string `json:"checksum"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	PackageName string
	Version     string
}

// Publish uploads nexus.toml + .nex via multipart POST /api/v1/publish.
func (c *Client) Publish(ctx context.Context, opts PublishOptions) (*PublishResult, error) {
	if c.token == "" {
		return nil, fmt.Errorf("authentication required: run `nex login` or set NEX_API_KEY")
	}
	if opts.ManifestPath == "" {
		return nil, fmt.Errorf("manifest path is required")
	}
	if opts.PackagePath == "" {
		return nil, fmt.Errorf("package path is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writeFilePart(writer, "nexus.toml", opts.ManifestPath); err != nil {
		return nil, err
	}
	if err := writeFilePart(writer, "package", opts.PackagePath); err != nil {
		return nil, err
	}
	if opts.ReadmePath != "" {
		if _, err := os.Stat(opts.ReadmePath); err == nil {
			if err := writeFilePart(writer, "readme", opts.ReadmePath); err != nil {
				return nil, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/publish", &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("publish to registry: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return &PublishResult{Message: strings.TrimSpace(string(data))}, nil
	}
	result := &PublishResult{
		Message:     strAny(raw["message"]),
		Checksum:    strAny(raw["checksum"]),
		Filename:    strAny(raw["filename"]),
		DownloadURL: strAny(raw["download_url"]),
	}
	if pkg, ok := raw["package"].(map[string]any); ok {
		result.PackageName = firstString(pkg, "Name", "name")
	}
	if ver, ok := raw["version"].(map[string]any); ok {
		result.Version = firstString(ver, "Version", "version")
		if result.Checksum == "" {
			result.Checksum = firstString(ver, "Checksum", "checksum")
		}
		if result.Filename == "" {
			result.Filename = firstString(ver, "Filename", "filename")
		}
	}
	return result, nil
}

// YankResult is returned after yanking a version.
type YankResult struct {
	Message string
	Name    string
	Version string
	Reason  string
}

// Yank marks name@version as yanked with a required reason.
func (c *Client) Yank(ctx context.Context, name, version, reason string) (*YankResult, error) {
	if c.token == "" {
		return nil, fmt.Errorf("authentication required: run `nex login` or set NEX_API_KEY")
	}
	if name == "" || version == "" {
		return nil, fmt.Errorf("package name and version are required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("yank reason is required")
	}

	payload, _ := json.Marshal(map[string]string{"reason": reason})
	endpoint := fmt.Sprintf("%s/api/v1/packages/%s/%s/yank",
		c.baseURL, url.PathEscape(name), url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yank: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, data)
	}

	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	out := &YankResult{
		Message: strAny(raw["message"]),
		Name:    name,
		Version: version,
		Reason:  reason,
	}
	if pkg, ok := raw["package"].(map[string]any); ok {
		if n := firstString(pkg, "Name", "name"); n != "" {
			out.Name = n
		}
	}
	if ver, ok := raw["version"].(map[string]any); ok {
		if v := firstString(ver, "Version", "version"); v != "" {
			out.Version = v
		}
		if r := firstString(ver, "YankReason", "yank_reason"); r != "" {
			out.Reason = r
		}
	}
	return out, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func writeFilePart(w *multipart.Writer, field, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	part, err := w.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return fmt.Errorf("create form field %s: %w", field, err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("write form field %s: %w", field, err)
	}
	return nil
}

func apiError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var wrap struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &wrap) == nil && wrap.Error != "" {
		msg = wrap.Error
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("registry returned %d: %s", status, msg)
}

func versionFromMap(m map[string]any) *VersionInfo {
	if m == nil {
		return &VersionInfo{}
	}
	info := &VersionInfo{
		Version:  firstString(m, "Version", "version"),
		Checksum: firstString(m, "Checksum", "checksum"),
		Filename: firstString(m, "Filename", "filename"),
		Reason:   firstString(m, "YankReason", "yank_reason"),
	}
	switch v := m["Yanked"].(type) {
	case bool:
		info.Yanked = v
	default:
		if b, ok := m["yanked"].(bool); ok {
			info.Yanked = b
		}
	}
	return info
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strAny(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func strAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

// NormalizeChecksum strips an optional "sha256:" prefix for comparison.
func NormalizeChecksum(sum string) string {
	sum = strings.TrimSpace(sum)
	sum = strings.TrimPrefix(sum, "sha256:")
	return strings.ToLower(sum)
}
