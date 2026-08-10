package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProvenanceVerifier validates optional Sigstore/OIDC provenance attachments.
// Full Sigstore verification is intentionally stubbed; StoreProvenance still
// persists caller-supplied metadata for later verification.
type ProvenanceVerifier interface {
	// Verify checks provenance bytes. The stub accepts any well-formed JSON object.
	Verify(ctx context.Context, source string, raw []byte) error
}

// StubProvenanceVerifier records that verification is not enforced yet.
type StubProvenanceVerifier struct{}

// Verify implements ProvenanceVerifier.
func (StubProvenanceVerifier) Verify(ctx context.Context, source string, raw []byte) error {
	_ = ctx
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "oidc"
	}
	switch source {
	case "oidc", "sigstore", "github_oidc", "none":
	default:
		return fmt.Errorf("unsupported provenance source %q", source)
	}
	if len(raw) == 0 {
		return nil
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("provenance must be JSON: %w", err)
	}
	// Stub: accept without cryptographic verification.
	return nil
}
