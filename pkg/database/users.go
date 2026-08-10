package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser inserts a new user account.
func (db *DB) CreateUser(ctx context.Context, username, email, passwordHash, avatarURL, bio string, useGravatar bool) (*User, error) {
	u, err := scanUser(db.pool.QueryRow(ctx, `
INSERT INTO users (username, email, password_hash, avatar_url, bio, use_gravatar)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING `+userSelectCols+`
`, username, email, passwordHash, avatarURL, bio, useGravatar))
	if err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("%w: username or email already taken", ErrConflict)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: username or email already taken", ErrConflict)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

const userSelectCols = `id, username, email, password_hash, avatar_url, bio, use_gravatar, github_id, github_login, email_verified, email_verified_at, totp_secret, totp_enabled, totp_pending_secret, is_admin, created_at, updated_at`

// GetUserByID loads a user by primary key.
func (db *DB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
SELECT `+userSelectCols+`
FROM users WHERE id = $1
`, id))
}

// GetUserByUsername loads a user by username (case-insensitive).
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
SELECT `+userSelectCols+`
FROM users WHERE LOWER(username) = LOWER($1)
`, username))
}

// GetUserByEmail loads a user by email (case-insensitive).
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
SELECT `+userSelectCols+`
FROM users WHERE LOWER(email) = LOWER($1)
`, email))
}

// GetUserByLogin resolves username or email for authentication.
func (db *DB) GetUserByLogin(ctx context.Context, login string) (*User, error) {
	login = strings.TrimSpace(login)
	return scanUser(db.pool.QueryRow(ctx, `
SELECT `+userSelectCols+`
FROM users WHERE LOWER(username) = LOWER($1) OR LOWER(email) = LOWER($1)
`, login))
}

// GetUserByGitHubID loads a user linked to a GitHub account.
func (db *DB) GetUserByGitHubID(ctx context.Context, githubID int64) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
SELECT `+userSelectCols+`
FROM users WHERE github_id = $1
`, githubID))
}

// UpdateUserProfile updates bio, avatar, and gravatar preference.
func (db *DB) UpdateUserProfile(ctx context.Context, userID int64, bio, avatarURL string, useGravatar bool) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET bio = $2, avatar_url = $3, use_gravatar = $4, updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID, bio, avatarURL, useGravatar))
}

// UpsertGitHubUser creates or updates a user from GitHub OAuth profile data.
func (db *DB) UpsertGitHubUser(ctx context.Context, githubID int64, login, email, avatarURL string) (*User, error) {
	verified := email != "" && !strings.Contains(email, "@users.noreply.github.com")
	if u, err := db.GetUserByGitHubID(ctx, githubID); err == nil {
		return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET github_login = $2,
    avatar_url = CASE WHEN use_gravatar THEN avatar_url ELSE $3 END,
    email = CASE WHEN $4 <> '' THEN $4 ELSE email END,
    email_verified = CASE WHEN $5 THEN TRUE ELSE email_verified END,
    email_verified_at = CASE WHEN $5 AND NOT email_verified THEN NOW() ELSE email_verified_at END,
    updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, u.ID, login, avatarURL, email, verified))
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	username, err := db.uniqueUsername(ctx, login)
	if err != nil {
		return nil, err
	}
	if email == "" {
		email = fmt.Sprintf("%d+github@users.noreply.github.com", githubID)
		verified = false
	}
	// If email already taken by a password account, use noreply instead.
	if existing, err := db.GetUserByEmail(ctx, email); err == nil && existing != nil {
		email = fmt.Sprintf("%d+github@users.noreply.github.com", githubID)
		verified = false
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	u, err := scanUser(db.pool.QueryRow(ctx, `
INSERT INTO users (username, email, password_hash, avatar_url, bio, use_gravatar, github_id, github_login, email_verified, email_verified_at)
VALUES ($1, $2, '', $3, '', FALSE, $4, $5, $6, CASE WHEN $6 THEN NOW() ELSE NULL END)
RETURNING `+userSelectCols+`
`, username, email, avatarURL, githubID, login, verified))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: username or email already taken", ErrConflict)
		}
		return nil, fmt.Errorf("create github user: %w", err)
	}
	return u, nil
}

// LinkGitHubAccount attaches a GitHub identity to an existing user.
func (db *DB) LinkGitHubAccount(ctx context.Context, userID, githubID int64, login, avatarURL string) (*User, error) {
	if other, err := db.GetUserByGitHubID(ctx, githubID); err == nil && other.ID != userID {
		return nil, fmt.Errorf("%w: github account already linked", ErrConflict)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET github_id = $2, github_login = $3,
    avatar_url = CASE WHEN use_gravatar THEN avatar_url ELSE $4 END,
    updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID, githubID, login, avatarURL))
}

// UnlinkGitHubAccount clears GitHub linkage. Refuses if the user has no password.
func (db *DB) UnlinkGitHubAccount(ctx context.Context, userID int64) (*User, error) {
	u, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return nil, fmt.Errorf("set a password before disconnecting GitHub")
	}
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET github_id = NULL, github_login = '', updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID))
}

func (db *DB) uniqueUsername(ctx context.Context, base string) (string, error) {
	base = sanitizeGitHubUsername(base)
	candidate := base
	for i := 0; i < 50; i++ {
		_, err := db.GetUserByUsername(ctx, candidate)
		if errors.Is(err, ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s%d", base, i+2)
	}
	return "", fmt.Errorf("could not allocate username")
}

func sanitizeGitHubUsername(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if ok {
			if i == 0 && r >= '0' && r <= '9' {
				b.WriteByte('u')
			}
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) < 3 {
		out = out + "gh"
	}
	if len(out) > 32 {
		out = out[:32]
	}
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "gh" + out
	}
	return out
}

// UserStats returns activity counters for a publisher.
func (db *DB) UserStats(ctx context.Context, userID int64) (*UserStats, error) {
	var s UserStats
	err := db.pool.QueryRow(ctx, `
SELECT
    (SELECT COUNT(DISTINCT p.id) FROM packages p
      LEFT JOIN package_owners po ON po.package_id = p.id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
     WHERE p.owner_id = $1 OR po.id IS NOT NULL),
    (SELECT COUNT(*) FROM versions v
      JOIN packages p ON p.id = v.package_id
      LEFT JOIN package_owners po ON po.package_id = p.id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
     WHERE p.owner_id = $1 OR po.id IS NOT NULL),
    (SELECT COALESCE(SUM(download_count), 0) FROM (
        SELECT DISTINCT p.id, p.download_count FROM packages p
        LEFT JOIN package_owners po ON po.package_id = p.id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
        WHERE p.owner_id = $1 OR po.id IS NOT NULL
    ) x),
    (SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL),
    (SELECT COUNT(*) FROM trusted_publishers WHERE user_id = $1)
`, userID).Scan(&s.PackageCount, &s.VersionCount, &s.TotalDownloads, &s.APIKeyCount, &s.TrustedCount)
	if err != nil {
		return nil, fmt.Errorf("user stats: %w", err)
	}
	return &s, nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	var passwordHash *string
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &passwordHash, &u.AvatarURL, &u.Bio, &u.UseGravatar,
		&u.GitHubID, &u.GitHubLogin, &u.EmailVerified, &u.EmailVerifiedAt,
		&u.TOTPSecret, &u.TOTPEnabled, &u.TOTPPendingSecret, &u.IsAdmin,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	if passwordHash != nil {
		u.PasswordHash = *passwordHash
	}
	return &u, nil
}

// SetUserPassword updates a user's password hash.
func (db *DB) SetUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	tag, err := db.pool.Exec(ctx, `
UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1
`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkEmailVerified marks a user's email as verified.
func (db *DB) MarkEmailVerified(ctx context.Context, userID int64) (*User, error) {
	return scanUser(db.pool.QueryRow(ctx, `
UPDATE users
SET email_verified = TRUE, email_verified_at = COALESCE(email_verified_at, NOW()), updated_at = NOW()
WHERE id = $1
RETURNING `+userSelectCols+`
`, userID))
}

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateSession stores a hashed session token and returns the plaintext token once.
func (db *DB) CreateSession(ctx context.Context, userID int64, ttl time.Duration) (plaintext string, sess *Session, err error) {
	return db.createPrefixedSession(ctx, userID, ttl, "nxs_")
}

// CreateTrustedPublishToken mints a short-lived Bearer token (nxt_…) for CI publish after OIDC exchange.
func (db *DB) CreateTrustedPublishToken(ctx context.Context, userID int64, ttl time.Duration) (plaintext string, sess *Session, err error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return db.createPrefixedSession(ctx, userID, ttl, "nxt_")
}

func (db *DB) createPrefixedSession(ctx context.Context, userID int64, ttl time.Duration, prefix string) (plaintext string, sess *Session, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	plaintext = prefix + hex.EncodeToString(raw)
	hash := HashToken(plaintext)
	expires := time.Now().UTC().Add(ttl)

	var s Session
	err = db.pool.QueryRow(ctx, `
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at
`, userID, hash, expires).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}
	return plaintext, &s, nil
}

// UserBySessionToken resolves a valid non-expired session token to a user.
func (db *DB) UserBySessionToken(ctx context.Context, plaintext string) (*User, error) {
	hash := HashToken(plaintext)
	var userID int64
	err := db.pool.QueryRow(ctx, `
SELECT user_id FROM sessions
WHERE token_hash = $1 AND expires_at > NOW()
`, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	return db.GetUserByID(ctx, userID)
}

// DeleteSession revokes a session by plaintext token.
func (db *DB) DeleteSession(ctx context.Context, plaintext string) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, HashToken(plaintext))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired session rows.
func (db *DB) DeleteExpiredSessions(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	return err
}

// CreateAPIKey stores a hashed API key and returns the plaintext once.
// scope: publish | read | full. expiresInDays <= 0 means no expiry.
func (db *DB) CreateAPIKey(ctx context.Context, userID int64, name, scope string, expiresInDays int64) (plaintext string, key *APIKey, err error) {
	scope, err = NormalizeAPIKeyScope(scope)
	if err != nil {
		return "", nil, err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate api key: %w", err)
	}
	plaintext = "nex_" + hex.EncodeToString(raw)
	prefix := plaintext[:12] // "nex_" + 8 hex chars
	hash := HashToken(plaintext)

	var expires any
	if expiresInDays > 0 {
		expires = time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
	}

	var k APIKey
	err = db.pool.QueryRow(ctx, `
INSERT INTO api_keys (user_id, name, prefix, key_hash, scope, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, name, prefix, key_hash, scope, expires_at, created_at, last_used_at, revoked_at
`, userID, name, prefix, hash, scope, expires).Scan(
		&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.KeyHash, &k.Scope, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("create api key: %w", err)
	}
	return plaintext, &k, nil
}

// ListAPIKeys returns non-secret API key metadata for a user.
func (db *DB) ListAPIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, err := db.pool.Query(ctx, `
SELECT id, user_id, name, prefix, key_hash, scope, expires_at, created_at, last_used_at, revoked_at
FROM api_keys WHERE user_id = $1
ORDER BY created_at DESC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	out := make([]APIKey, 0)
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.KeyHash, &k.Scope, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		k.KeyHash = ""
		out = append(out, k)
	}
	return out, rows.Err()
}

// UserByAPIKey resolves a plaintext API key to its owning user.
func (db *DB) UserByAPIKey(ctx context.Context, plaintext string) (*User, *APIKey, error) {
	hash := HashToken(plaintext)
	var k APIKey
	err := db.pool.QueryRow(ctx, `
SELECT id, user_id, name, prefix, key_hash, scope, expires_at, created_at, last_used_at, revoked_at
FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL
`, hash).Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.KeyHash, &k.Scope, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("lookup api key: %w", err)
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(time.Now().UTC()) {
		return nil, nil, ErrNotFound
	}
	if k.Scope == "" {
		k.Scope = APIKeyScopePublish
	}
	_, _ = db.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, k.ID)
	u, err := db.GetUserByID(ctx, k.UserID)
	if err != nil {
		return nil, nil, err
	}
	k.KeyHash = ""
	return u, &k, nil
}

// RevokeAPIKey soft-revokes an API key owned by the user.
func (db *DB) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	tag, err := db.pool.Exec(ctx, `
UPDATE api_keys SET revoked_at = NOW()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
`, keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateTrustedPublisher inserts a pending trusted publisher configuration (RubyGems-style).
// Requires repository owner/name, exact workflow filename, and package name to claim.
func (db *DB) CreateTrustedPublisher(ctx context.Context, tp TrustedPublisher) (*TrustedPublisher, error) {
	tp.RepositoryOwner = strings.TrimSpace(tp.RepositoryOwner)
	tp.RepositoryName = strings.TrimSpace(tp.RepositoryName)
	tp.WorkflowFilename = strings.TrimSpace(tp.WorkflowFilename)
	tp.Environment = strings.TrimSpace(tp.Environment)
	tp.PackageScope = strings.TrimSpace(tp.PackageScope)
	if tp.Provider == "" {
		tp.Provider = "github_actions"
	}
	if tp.RepositoryOwner == "" || tp.RepositoryName == "" {
		return nil, fmt.Errorf("repository owner and name are required")
	}
	if tp.WorkflowFilename == "" {
		return nil, fmt.Errorf("workflow filename is required (e.g. .github/workflows/release.yml)")
	}
	if tp.PackageScope == "" {
		return nil, fmt.Errorf("package name to claim is required for a pending trusted publisher")
	}
	if !reMatchPackageName(tp.PackageScope) {
		return nil, fmt.Errorf("invalid package name %q", tp.PackageScope)
	}

	// Cannot claim a package already owned by someone else.
	if pkg, err := db.GetPackageByName(ctx, tp.PackageScope); err == nil && pkg != nil {
		if pkg.OwnerID != nil && *pkg.OwnerID != tp.UserID {
			return nil, fmt.Errorf("package %q is already owned by another user", tp.PackageScope)
		}
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var out TrustedPublisher
	err := db.pool.QueryRow(ctx, `
INSERT INTO trusted_publishers (
    user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status
) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending')
RETURNING id, user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status, last_used_at, last_failure_reason, last_failure_at, verified_at, created_at
`, tp.UserID, tp.Provider, tp.RepositoryOwner, tp.RepositoryName, tp.WorkflowFilename, tp.Environment, tp.PackageScope).Scan(
		&out.ID, &out.UserID, &out.Provider, &out.RepositoryOwner, &out.RepositoryName,
		&out.WorkflowFilename, &out.Environment, &out.PackageScope, &out.Status,
		&out.LastUsedAt, &out.LastFailureReason, &out.LastFailureAt, &out.VerifiedAt, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create trusted publisher: %w", err)
	}
	return &out, nil
}

func reMatchPackageName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for i, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			return false
		}
		if i == 0 && !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

// ListTrustedPublishers returns trusted publisher configs for a user.
func (db *DB) ListTrustedPublishers(ctx context.Context, userID int64) ([]TrustedPublisher, error) {
	rows, err := db.pool.Query(ctx, `
SELECT id, user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status, last_used_at, last_failure_reason, last_failure_at, verified_at, created_at
FROM trusted_publishers WHERE user_id = $1
ORDER BY created_at DESC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list trusted publishers: %w", err)
	}
	defer rows.Close()

	out := make([]TrustedPublisher, 0)
	for rows.Next() {
		var tp TrustedPublisher
		if err := rows.Scan(
			&tp.ID, &tp.UserID, &tp.Provider, &tp.RepositoryOwner, &tp.RepositoryName,
			&tp.WorkflowFilename, &tp.Environment, &tp.PackageScope, &tp.Status,
			&tp.LastUsedAt, &tp.LastFailureReason, &tp.LastFailureAt, &tp.VerifiedAt, &tp.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, tp)
	}
	return out, rows.Err()
}

// DeleteTrustedPublisher removes a trusted publisher config owned by the user.
func (db *DB) DeleteTrustedPublisher(ctx context.Context, userID, id int64) error {
	tag, err := db.pool.Exec(ctx, `DELETE FROM trusted_publishers WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete trusted publisher: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MatchTrustedPublisher finds a config matching CI identity claims for a user/package.
func (db *DB) MatchTrustedPublisher(ctx context.Context, userID int64, packageName, owner, repo, workflow, environment string) (*TrustedPublisher, error) {
	rows, err := db.pool.Query(ctx, `
SELECT id, user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status, last_used_at, last_failure_reason, last_failure_at, verified_at, created_at
FROM trusted_publishers
WHERE user_id = $1
  AND LOWER(provider) = 'github_actions'
  AND LOWER(repository_owner) = LOWER($2)
  AND LOWER(repository_name) = LOWER($3)
`, userID, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("match trusted publisher: %w", err)
	}
	defer rows.Close()

	var candidates []TrustedPublisher
	for rows.Next() {
		var tp TrustedPublisher
		if err := rows.Scan(
			&tp.ID, &tp.UserID, &tp.Provider, &tp.RepositoryOwner, &tp.RepositoryName,
			&tp.WorkflowFilename, &tp.Environment, &tp.PackageScope, &tp.Status,
			&tp.LastUsedAt, &tp.LastFailureReason, &tp.LastFailureAt, &tp.VerifiedAt, &tp.CreatedAt,
		); err != nil {
			return nil, err
		}
		if trustedPublisherMatches(tp, packageName, workflow, environment) {
			candidates = append(candidates, tp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pickBestTrusted(candidates)
}

// MatchTrustedPublisherByClaims finds a trusted publisher from GitHub OIDC identity
// without knowing the user yet (used for OIDC-only publish).
func (db *DB) MatchTrustedPublisherByClaims(ctx context.Context, packageName, owner, repo, workflow, environment string) (*TrustedPublisher, error) {
	rows, err := db.pool.Query(ctx, `
SELECT id, user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status, last_used_at, last_failure_reason, last_failure_at, verified_at, created_at
FROM trusted_publishers
WHERE LOWER(provider) = 'github_actions'
  AND LOWER(repository_owner) = LOWER($1)
  AND LOWER(repository_name) = LOWER($2)
`, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("match trusted publisher by claims: %w", err)
	}
	defer rows.Close()

	var candidates []TrustedPublisher
	for rows.Next() {
		var tp TrustedPublisher
		if err := rows.Scan(
			&tp.ID, &tp.UserID, &tp.Provider, &tp.RepositoryOwner, &tp.RepositoryName,
			&tp.WorkflowFilename, &tp.Environment, &tp.PackageScope, &tp.Status,
			&tp.LastUsedAt, &tp.LastFailureReason, &tp.LastFailureAt, &tp.VerifiedAt, &tp.CreatedAt,
		); err != nil {
			return nil, err
		}
		if trustedPublisherMatches(tp, packageName, workflow, environment) {
			candidates = append(candidates, tp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pickBestTrusted(candidates)
}

func trustedPublisherMatches(tp TrustedPublisher, packageName, workflow, environment string) bool {
	// Pending publishers always require an exact workflow filename match.
	if strings.EqualFold(tp.Status, TrustedStatusPending) || tp.WorkflowFilename != "" {
		if tp.WorkflowFilename == "" {
			return false
		}
		if workflow == "" || !workflowNamesEqual(tp.WorkflowFilename, workflow) {
			return false
		}
	}
	if tp.Environment != "" && !strings.EqualFold(tp.Environment, environment) {
		return false
	}
	// Empty packageName means auth-time match: allow scoped configs (enforced at publish).
	if packageName != "" && tp.PackageScope != "" && !strings.EqualFold(tp.PackageScope, packageName) {
		return false
	}
	// Pending publishers must claim a specific package; skip only when packageName is empty (auth).
	if strings.EqualFold(tp.Status, TrustedStatusPending) && tp.PackageScope == "" {
		return false
	}
	return true
}

// ExplainTrustedPublisherMismatch returns a human-readable near-miss reason for CI debugging.
func (db *DB) ExplainTrustedPublisherMismatch(ctx context.Context, packageName, owner, repo, workflow, environment string) string {
	rows, err := db.pool.Query(ctx, `
SELECT id, user_id, provider, repository_owner, repository_name, workflow_filename, environment, package_scope, status, last_used_at, last_failure_reason, last_failure_at, verified_at, created_at
FROM trusted_publishers
WHERE LOWER(provider) = 'github_actions'
  AND LOWER(repository_owner) = LOWER($1)
  AND LOWER(repository_name) = LOWER($2)
`, owner, repo)
	if err != nil {
		return "no trusted publisher matched repository/workflow/environment"
	}
	defer rows.Close()

	var any bool
	for rows.Next() {
		var tp TrustedPublisher
		if err := rows.Scan(
			&tp.ID, &tp.UserID, &tp.Provider, &tp.RepositoryOwner, &tp.RepositoryName,
			&tp.WorkflowFilename, &tp.Environment, &tp.PackageScope, &tp.Status,
			&tp.LastUsedAt, &tp.LastFailureReason, &tp.LastFailureAt, &tp.VerifiedAt, &tp.CreatedAt,
		); err != nil {
			continue
		}
		any = true
		if tp.WorkflowFilename != "" && (workflow == "" || !workflowNamesEqual(tp.WorkflowFilename, workflow)) {
			return fmt.Sprintf("workflow mismatch: configured %q, got %q", tp.WorkflowFilename, workflow)
		}
		if tp.Environment != "" && !strings.EqualFold(tp.Environment, environment) {
			return fmt.Sprintf("environment mismatch: configured %q, got %q", tp.Environment, environment)
		}
		if packageName != "" && tp.PackageScope != "" && !strings.EqualFold(tp.PackageScope, packageName) {
			return fmt.Sprintf("package mismatch: configured %q, got %q", tp.PackageScope, packageName)
		}
	}
	if !any {
		return fmt.Sprintf("no trusted publisher configured for %s/%s", owner, repo)
	}
	return "no trusted publisher matched repository/workflow/environment/package"
}

func workflowNamesEqual(configured, actual string) bool {
	if strings.EqualFold(configured, actual) {
		return true
	}
	// Allow "publish.yml" to match ".github/workflows/publish.yml"
	base := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "/")
		if i := strings.LastIndex(s, "/"); i >= 0 {
			return s[i+1:]
		}
		return s
	}
	return strings.EqualFold(base(configured), base(actual))
}

func pickBestTrusted(candidates []TrustedPublisher) (*TrustedPublisher, error) {
	if len(candidates) == 0 {
		return nil, ErrNotFound
	}
	best := candidates[0]
	score := func(tp TrustedPublisher) int {
		n := 0
		if tp.WorkflowFilename != "" {
			n += 4
		}
		if tp.Environment != "" {
			n += 2
		}
		if tp.PackageScope != "" {
			n += 1
		}
		return n
	}
	for _, tp := range candidates[1:] {
		if score(tp) > score(best) {
			best = tp
		}
	}
	return &best, nil
}

// HashToken returns a hex-encoded SHA-256 digest of a secret token.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
