package database

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	totpPeriod     = 30
	totpDigits     = 6
	totpWindow     = 1
	challengeTTL   = 10 * time.Minute
	unpublishWindow = 72 * time.Hour
)

// NormalizeAPIKeyScope returns a valid scope or an error.
func NormalizeAPIKeyScope(scope string) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		scope = APIKeyScopePublish
	}
	switch scope {
	case APIKeyScopePublish, APIKeyScopeRead, APIKeyScopeFull:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid api key scope %q (use publish, read, or full)", scope)
	}
}

// APIKeyAllowsPublish reports whether the key may call publish endpoints.
func APIKeyAllowsPublish(scope string) bool {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return scope == APIKeyScopePublish || scope == APIKeyScopeFull || scope == ""
}

// APIKeyAllowsRead reports whether the key may call authenticated read endpoints.
func APIKeyAllowsRead(scope string) bool {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return scope == APIKeyScopeRead || scope == APIKeyScopeFull || scope == APIKeyScopePublish || scope == ""
}

// GenerateTOTPSecret returns a base32-encoded secret suitable for authenticator apps.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPCode computes the current TOTP for secret at unix time t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", err
	}
	counter := uint64(t.Unix() / totpPeriod)
	return hotp(key, counter, totpDigits), nil
}

// VerifyTOTP checks code against the secret with a small time window.
func VerifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if secret == "" || len(code) < 6 {
		return false
	}
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return false
	}
	counter := uint64(now.Unix() / totpPeriod)
	for d := -totpWindow; d <= totpWindow; d++ {
		c := int64(counter) + int64(d)
		if c < 0 {
			continue
		}
		if hotp(key, uint64(c), totpDigits) == code {
			return true
		}
	}
	return false
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	secret = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
}

func hotp(key []byte, counter uint64, digits int) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, truncated%mod)
}

// BeginTOTPSetup stores a pending TOTP secret for the user.
func (db *DB) BeginTOTPSetup(ctx context.Context, userID int64) (secret string, err error) {
	secret, err = GenerateTOTPSecret()
	if err != nil {
		return "", err
	}
	_, err = db.pool.Exec(ctx, `
UPDATE users SET totp_pending_secret = $2, updated_at = NOW() WHERE id = $1
`, userID, secret)
	if err != nil {
		return "", fmt.Errorf("begin totp setup: %w", err)
	}
	return secret, nil
}

// ConfirmTOTPSetup enables TOTP after verifying a code against the pending secret.
func (db *DB) ConfirmTOTPSetup(ctx context.Context, userID int64, code string) (*User, error) {
	u, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.TOTPPendingSecret) == "" {
		return nil, fmt.Errorf("no pending 2FA setup")
	}
	if !VerifyTOTP(u.TOTPPendingSecret, code, time.Now()) {
		return nil, fmt.Errorf("invalid authenticator code")
	}
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET totp_secret = totp_pending_secret,
    totp_pending_secret = '',
    totp_enabled = TRUE,
    updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID))
}

// DisableTOTP turns off 2FA after verifying the current code (or admin override with empty code when already disabled).
func (db *DB) DisableTOTP(ctx context.Context, userID int64, code string) (*User, error) {
	u, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.TOTPEnabled {
		return u, nil
	}
	if !VerifyTOTP(u.TOTPSecret, code, time.Now()) {
		return nil, fmt.Errorf("invalid authenticator code")
	}
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET totp_enabled = FALSE, totp_secret = '', totp_pending_secret = '', updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID))
}

// CreateTOTPChallenge issues a short-lived challenge token after password login.
func (db *DB) CreateTOTPChallenge(ctx context.Context, userID int64) (plaintext string, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plaintext = "nxc_" + hex.EncodeToString(raw)
	_, err = db.pool.Exec(ctx, `
INSERT INTO totp_challenges (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
`, userID, HashToken(plaintext), time.Now().UTC().Add(challengeTTL))
	if err != nil {
		return "", fmt.Errorf("create totp challenge: %w", err)
	}
	return plaintext, nil
}

// ConsumeTOTPChallenge validates the challenge token and TOTP code, returning the user.
func (db *DB) ConsumeTOTPChallenge(ctx context.Context, challenge, code string) (*User, error) {
	challenge = strings.TrimSpace(challenge)
	if !strings.HasPrefix(challenge, "nxc_") {
		return nil, fmt.Errorf("invalid challenge")
	}
	var userID int64
	err := db.pool.QueryRow(ctx, `
SELECT user_id FROM totp_challenges
WHERE token_hash = $1 AND expires_at > NOW()
`, HashToken(challenge)).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("invalid or expired challenge")
		}
		return nil, err
	}
	u, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.TOTPEnabled || !VerifyTOTP(u.TOTPSecret, code, time.Now()) {
		return nil, fmt.Errorf("invalid authenticator code")
	}
	_, _ = db.pool.Exec(ctx, `DELETE FROM totp_challenges WHERE token_hash = $1`, HashToken(challenge))
	_, _ = db.pool.Exec(ctx, `DELETE FROM totp_challenges WHERE expires_at <= NOW()`)
	return u, nil
}

// InsertAuditLog appends a security audit event.
func (db *DB) InsertAuditLog(ctx context.Context, ev AuditEvent) error {
	meta := []byte("{}")
	if ev.Metadata != nil {
		if b, err := json.Marshal(ev.Metadata); err == nil {
			meta = b
		}
	}
	var actor any
	if ev.ActorUserID > 0 {
		actor = ev.ActorUserID
	}
	_, err := db.pool.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, resource_type, resource_id, package_name, version, ip, user_agent, metadata)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)
`, actor, ev.Action, ev.ResourceType, ev.ResourceID, ev.PackageName, ev.Version, ev.IP, ev.UserAgent, string(meta))
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ListAuditLogsForUser returns recent audit events for an actor.
func (db *DB) ListAuditLogsForUser(ctx context.Context, userID int64, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.pool.Query(ctx, `
SELECT id, actor_user_id, action, resource_type, resource_id, package_name, version, ip, user_agent, metadata, created_at
FROM audit_logs
WHERE actor_user_id = $1
ORDER BY created_at DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// ListAuditLogsAdmin returns recent audit events (admin).
func (db *DB) ListAuditLogsAdmin(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.pool.Query(ctx, `
SELECT id, actor_user_id, action, resource_type, resource_id, package_name, version, ip, user_agent, metadata, created_at
FROM audit_logs
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

func scanAuditLogs(rows pgx.Rows) ([]AuditLog, error) {
	out := make([]AuditLog, 0)
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(
			&a.ID, &a.ActorUserID, &a.Action, &a.ResourceType, &a.ResourceID,
			&a.PackageName, &a.Version, &a.IP, &a.UserAgent, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAbuseReport stores a trust & safety report.
func (db *DB) CreateAbuseReport(ctx context.Context, r AbuseReport) (*AbuseReport, error) {
	r.PackageName = strings.TrimSpace(r.PackageName)
	r.Version = strings.TrimSpace(r.Version)
	r.Category = strings.TrimSpace(r.Category)
	r.Details = strings.TrimSpace(r.Details)
	r.ReporterEmail = strings.TrimSpace(r.ReporterEmail)
	if r.Category == "" {
		r.Category = "other"
	}
	if r.Details == "" {
		return nil, fmt.Errorf("details are required")
	}
	if len(r.Details) > 4000 {
		return nil, fmt.Errorf("details exceed 4000 characters")
	}
	var out AbuseReport
	err := db.pool.QueryRow(ctx, `
INSERT INTO abuse_reports (reporter_user_id, reporter_email, package_name, version, category, details, status)
VALUES ($1,$2,$3,$4,$5,$6,'open')
RETURNING id, reporter_user_id, reporter_email, package_name, version, category, details, status, created_at, resolved_at
`, r.ReporterUserID, r.ReporterEmail, r.PackageName, r.Version, r.Category, r.Details).Scan(
		&out.ID, &out.ReporterUserID, &out.ReporterEmail, &out.PackageName, &out.Version,
		&out.Category, &out.Details, &out.Status, &out.CreatedAt, &out.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create abuse report: %w", err)
	}
	return &out, nil
}

// ListAbuseReports returns reports for admin review.
func (db *DB) ListAbuseReports(ctx context.Context, status string, limit int) ([]AbuseReport, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	status = strings.TrimSpace(strings.ToLower(status))
	var rows pgx.Rows
	var err error
	if status == "" || status == "all" {
		rows, err = db.pool.Query(ctx, `
SELECT id, reporter_user_id, reporter_email, package_name, version, category, details, status, created_at, resolved_at
FROM abuse_reports ORDER BY created_at DESC LIMIT $1
`, limit)
	} else {
		rows, err = db.pool.Query(ctx, `
SELECT id, reporter_user_id, reporter_email, package_name, version, category, details, status, created_at, resolved_at
FROM abuse_reports WHERE status = $1 ORDER BY created_at DESC LIMIT $2
`, status, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AbuseReport, 0)
	for rows.Next() {
		var r AbuseReport
		if err := rows.Scan(
			&r.ID, &r.ReporterUserID, &r.ReporterEmail, &r.PackageName, &r.Version,
			&r.Category, &r.Details, &r.Status, &r.CreatedAt, &r.ResolvedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkTrustedPublisherVerified marks a publisher verified after successful OIDC use.
func (db *DB) MarkTrustedPublisherVerified(ctx context.Context, id int64) error {
	_, err := db.pool.Exec(ctx, `
UPDATE trusted_publishers
SET status = 'verified',
    verified_at = COALESCE(verified_at, NOW()),
    last_used_at = NOW(),
    last_failure_reason = '',
    last_failure_at = NULL
WHERE id = $1
`, id)
	return err
}

// RecordTrustedPublisherFailure records a failure reason for publishers matching a repo.
func (db *DB) RecordTrustedPublisherFailure(ctx context.Context, owner, repo, reason string) error {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	reason = strings.TrimSpace(reason)
	if owner == "" || repo == "" || reason == "" {
		return nil
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := db.pool.Exec(ctx, `
UPDATE trusted_publishers
SET last_failure_reason = $3, last_failure_at = NOW()
WHERE LOWER(repository_owner) = LOWER($1) AND LOWER(repository_name) = LOWER($2)
`, owner, repo, reason)
	return err
}

// RecordTrustedPublisherFailureForUser records a failure on a specific user's configs for a repo.
func (db *DB) RecordTrustedPublisherFailureForUser(ctx context.Context, userID int64, owner, repo, reason string) error {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	reason = strings.TrimSpace(reason)
	if userID <= 0 || owner == "" || repo == "" || reason == "" {
		return nil
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := db.pool.Exec(ctx, `
UPDATE trusted_publishers
SET last_failure_reason = $4, last_failure_at = NOW()
WHERE user_id = $1
  AND LOWER(repository_owner) = LOWER($2)
  AND LOWER(repository_name) = LOWER($3)
`, userID, owner, repo, reason)
	return err
}

// SetVersionProvenance stores optional provenance metadata on a version.
func (db *DB) SetVersionProvenance(ctx context.Context, versionID int64, source string, raw json.RawMessage) error {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		source = "oidc"
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	_, err := db.pool.Exec(ctx, `
UPDATE versions SET provenance_json = $2::jsonb, provenance_source = $3 WHERE id = $1
`, versionID, string(raw), source)
	return err
}

// UnyankPackageVersion clears the yanked flag for a version (owner only).
func (db *DB) UnyankPackageVersion(ctx context.Context, name, version string, ownerID int64) (*PackageVersion, error) {
	pv, err := db.GetPackageVersion(ctx, name, version)
	if err != nil {
		return nil, err
	}
	if pv.Package.OwnerID == nil || *pv.Package.OwnerID != ownerID {
		return nil, fmt.Errorf("you are not the owner of package %q", name)
	}
	_, err = db.pool.Exec(ctx, `
UPDATE versions
SET yanked = FALSE, yank_reason = '', yanked_at = NULL, yanked_by_user_id = NULL
WHERE id = $1
`, pv.Version.ID)
	if err != nil {
		return nil, err
	}
	pv.Version.Yanked = false
	pv.Version.YankReason = ""
	pv.Version.YankedAt = nil
	pv.Version.YankedByUserID = nil
	return pv, nil
}

// UnpublishPackageVersion hard-deletes a version within the unpublish window (owner only).
func (db *DB) UnpublishPackageVersion(ctx context.Context, name, version string, ownerID int64) (storagePath string, err error) {
	pv, err := db.GetPackageVersion(ctx, name, version)
	if err != nil {
		return "", err
	}
	if pv.Package.OwnerID == nil || *pv.Package.OwnerID != ownerID {
		return "", fmt.Errorf("you are not the owner of package %q", name)
	}
	if time.Since(pv.Version.CreatedAt) > unpublishWindow {
		return "", fmt.Errorf("unpublish only allowed within 72 hours of publish; yank instead")
	}
	storagePath = pv.Version.StoragePath
	tag, err := db.pool.Exec(ctx, `DELETE FROM versions WHERE id = $1`, pv.Version.ID)
	if err != nil {
		return "", fmt.Errorf("unpublish version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return storagePath, nil
}
