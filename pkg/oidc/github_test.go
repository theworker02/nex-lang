package oidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyGitHubOIDCClaims(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kid := "test-key"

	v := &Verifier{
		Issuer:         GitHubIssuer,
		Audiences:      []string{"http://localhost:8080", "nex-registry"},
		TestPublicKeys: map[string]*rsa.PublicKey{kid: &priv.PublicKey},
	}

	now := time.Now().Unix()
	payload := map[string]any{
		"iss":              GitHubIssuer,
		"sub":              "repo:acme/httpkit:ref:refs/heads/main",
		"aud":              "nex-registry",
		"exp":              now + 600,
		"iat":              now,
		"repository":       "acme/httpkit",
		"repository_owner": "acme",
		"job_workflow_ref": "acme/httpkit/.github/workflows/publish.yml@refs/heads/main",
		"workflow_ref":     "acme/httpkit/.github/workflows/publish.yml@refs/heads/main",
		"environment":      "release",
		"ref":              "refs/heads/main",
		"event_name":       "push",
		"actor":            "octocat",
	}
	token := mustSignJWT(t, priv, kid, payload)

	claims, err := v.Verify(t.Context(), token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.RepositoryOwnerName() != "acme" || claims.RepositoryName() != "httpkit" {
		t.Fatalf("repo parse: %q / %q", claims.RepositoryOwnerName(), claims.RepositoryName())
	}
	if got := claims.WorkflowFilename(); got != ".github/workflows/publish.yml" {
		t.Fatalf("workflow filename: %q", got)
	}
	if claims.Environment != "release" {
		t.Fatalf("environment: %q", claims.Environment)
	}
}

func TestVerifyRejectsBadAudience(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v := &Verifier{
		Issuer:         GitHubIssuer,
		Audiences:      []string{"nex-registry"},
		TestPublicKeys: map[string]*rsa.PublicKey{"k1": &priv.PublicKey},
	}
	now := time.Now().Unix()
	token := mustSignJWT(t, priv, "k1", map[string]any{
		"iss":        GitHubIssuer,
		"aud":        "wrong-audience",
		"exp":        now + 600,
		"iat":        now,
		"repository": "acme/httpkit",
	})
	if _, err := v.Verify(t.Context(), token); err == nil {
		t.Fatal("expected audience error")
	}
}

func TestWorkflowFilenameExtraction(t *testing.T) {
	c := Claims{JobWorkflowRef: "org/repo/.github/workflows/release.yml@refs/tags/v1"}
	if got := c.WorkflowFilename(); got != ".github/workflows/release.yml" {
		t.Fatalf("got %q", got)
	}
}

func mustSignJWT(t *testing.T, priv *rsa.PrivateKey, kid string, payload map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + enc.EncodeToString(sig)
}
