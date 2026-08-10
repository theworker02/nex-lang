package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	GitHubIssuer  = "https://token.actions.githubusercontent.com"
	GitHubJWKSURL = "https://token.actions.githubusercontent.com/.well-known/jwks"
)

// Claims are the GitHub Actions OIDC claims used for trusted publishing.
type Claims struct {
	Issuer          string `json:"iss"`
	Subject         string `json:"sub"`
	Audience        flexAud `json:"aud"`
	Expiry          int64  `json:"exp"`
	IssuedAt        int64  `json:"iat"`
	Repository      string `json:"repository"` // owner/repo
	RepositoryOwner string `json:"repository_owner"`
	JobWorkflowRef  string `json:"job_workflow_ref"`
	WorkflowRef     string `json:"workflow_ref"`
	Workflow        string `json:"workflow"`
	Environment     string `json:"environment"`
	Ref             string `json:"ref"`
	EventName       string `json:"event_name"`
	RunID           string `json:"run_id"`
	Actor           string `json:"actor"`
}

// flexAud accepts JWT aud as either a string or []string.
type flexAud []string

func (a *flexAud) UnmarshalJSON(b []byte) error {
	b = bytesTrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*a = nil
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*a = []string{s}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	*a = arr
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// RepositoryOwnerName returns the owner portion of repository.
func (c Claims) RepositoryOwnerName() string {
	if c.RepositoryOwner != "" {
		return c.RepositoryOwner
	}
	owner, _ := splitRepo(c.Repository)
	return owner
}

// RepositoryName returns the repo name portion of repository.
func (c Claims) RepositoryName() string {
	_, name := splitRepo(c.Repository)
	return name
}

// WorkflowFilename extracts .github/workflows/….yml from job_workflow_ref or workflow_ref.
func (c Claims) WorkflowFilename() string {
	for _, ref := range []string{c.JobWorkflowRef, c.WorkflowRef} {
		if ref == "" {
			continue
		}
		// owner/repo/.github/workflows/publish.yml@refs/heads/main
		at := strings.Index(ref, "@")
		path := ref
		if at >= 0 {
			path = ref[:at]
		}
		idx := strings.Index(path, ".github/workflows/")
		if idx >= 0 {
			return path[idx:]
		}
	}
	if c.Workflow != "" {
		if strings.Contains(c.Workflow, "/") {
			return c.Workflow
		}
		return ".github/workflows/" + c.Workflow
	}
	return ""
}

func splitRepo(repo string) (owner, name string) {
	parts := strings.SplitN(strings.TrimSpace(repo), "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// Verifier validates GitHub Actions OIDC JWTs.
type Verifier struct {
	JWKSURL   string
	Issuer    string
	Audiences []string
	Client    *http.Client

	// TestPublicKeys maps kid -> key for unit tests (skips JWKS fetch when set).
	TestPublicKeys map[string]*rsa.PublicKey

	mu       sync.Mutex
	keys     map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// DefaultVerifier returns a verifier for GitHub Actions with the given audiences.
func DefaultVerifier(audiences ...string) *Verifier {
	if len(audiences) == 0 {
		audiences = []string{"nex-registry"}
	}
	return &Verifier{
		JWKSURL:   GitHubJWKSURL,
		Issuer:    GitHubIssuer,
		Audiences: audiences,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Verify parses and validates a GitHub Actions OIDC JWT.
func (v *Verifier) Verify(ctx context.Context, raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("oidc: token is not a JWT")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oidc: decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("oidc: parse header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("oidc: unsupported alg %q", header.Alg)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: decode payload: %w", err)
	}

	key, err := v.publicKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("oidc: decode signature: %w", err)
	}
	if err := verifyRS256(key, signingInput, sig); err != nil {
		return nil, err
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("oidc: parse claims: %w", err)
	}

	issuer := v.Issuer
	if issuer == "" {
		issuer = GitHubIssuer
	}
	if claims.Issuer != issuer {
		return nil, fmt.Errorf("oidc: unexpected issuer %q", claims.Issuer)
	}
	if claims.Expiry == 0 || time.Now().Unix() >= claims.Expiry {
		return nil, errors.New("oidc: token expired")
	}
	if !audienceMatch([]string(claims.Audience), v.Audiences) {
		return nil, fmt.Errorf("oidc: audience mismatch (got %v, want one of %v)", claims.Audience, v.Audiences)
	}
	if claims.Repository == "" && claims.RepositoryOwner == "" {
		return nil, errors.New("oidc: missing repository claims")
	}
	return &claims, nil
}

func audienceMatch(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		for _, g := range got {
			if g == w {
				return true
			}
		}
	}
	return false
}

func (v *Verifier) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if v.TestPublicKeys != nil {
		if k, ok := v.TestPublicKeys[kid]; ok {
			return k, nil
		}
		if k, ok := v.TestPublicKeys[""]; ok {
			return k, nil
		}
		return nil, fmt.Errorf("oidc: test key not found for kid %q", kid)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys != nil && time.Since(v.fetchedAt) < time.Hour {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
	}
	if err := v.fetchJWKSLocked(ctx); err != nil {
		return nil, err
	}
	k, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("oidc: jwks missing kid %q", kid)
	}
	return k, nil
}

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) fetchJWKSLocked(ctx context.Context) error {
	url := v.JWKSURL
	if url == "" {
		url = GitHubJWKSURL
	}
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("oidc: fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc: jwks status %d", resp.StatusCode)
	}
	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("oidc: decode jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSA(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("oidc: no RSA keys in jwks")
	}
	v.keys = keys
	v.fetchedAt = time.Now()
	return nil
}

func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	var eInt int
	for _, b := range eb {
		eInt = eInt<<8 + int(b)
	}
	if eInt == 0 {
		eInt = 65537
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: eInt}, nil
}

func verifyRS256(pub *rsa.PublicKey, signingInput, sig []byte) error {
	sum := sha256.Sum256(signingInput)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return fmt.Errorf("oidc: invalid signature: %w", err)
	}
	return nil
}
