package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a PostgreSQL connection pool and registry data access.
type DB struct {
	pool     *pgxpool.Pool
	readPool *pgxpool.Pool // optional read replica / mirror
}

// Connect opens a pooled PostgreSQL connection, verifies connectivity, and
// applies the registry schema if it is not already present.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	pool, err := openPool(ctx, databaseURL, 20, 2)
	if err != nil {
		return nil, err
	}

	db := &DB{pool: pool}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return db, nil
}

func openPool(ctx context.Context, databaseURL string, maxConns, minConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// AttachReadReplica opens an optional read-only pool (DATABASE_URL_READ).
// Read-heavy queries use it when set; writes always use the primary pool.
func (db *DB) AttachReadReplica(ctx context.Context, readURL string) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database not connected")
	}
	if readURL == "" {
		return nil
	}
	pool, err := openPool(ctx, readURL, 20, 1)
	if err != nil {
		return fmt.Errorf("read replica: %w", err)
	}
	if db.readPool != nil {
		db.readPool.Close()
	}
	db.readPool = pool
	return nil
}

// reader returns the read replica pool when configured, otherwise primary.
func (db *DB) reader() *pgxpool.Pool {
	if db != nil && db.readPool != nil {
		return db.readPool
	}
	return db.pool
}

// HasReadReplica reports whether DATABASE_URL_READ (or equivalent) is attached.
func (db *DB) HasReadReplica() bool {
	return db != nil && db.readPool != nil
}

// Ping verifies primary (and read replica, if configured) connectivity.
func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database not connected")
	}
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("primary: %w", err)
	}
	if db.readPool != nil {
		if err := db.readPool.Ping(ctx); err != nil {
			return fmt.Errorf("read replica: %w", err)
		}
	}
	return nil
}

// Close releases the underlying connection pool(s).
func (db *DB) Close() {
	if db == nil {
		return
	}
	if db.readPool != nil {
		db.readPool.Close()
		db.readPool = nil
	}
	if db.pool != nil {
		db.pool.Close()
		db.pool = nil
	}
}

// Pool exposes the underlying primary pgx pool for advanced use.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

const coreSchemaSQL = `
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    avatar_url    TEXT NOT NULL DEFAULT '',
    bio           TEXT NOT NULL DEFAULT '',
    use_gravatar  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL DEFAULT '',
    prefix       TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS trusted_publishers (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL DEFAULT 'github_actions',
    repository_owner  TEXT NOT NULL,
    repository_name   TEXT NOT NULL,
    workflow_filename TEXT NOT NULL DEFAULT '',
    environment       TEXT NOT NULL DEFAULT '',
    package_scope     TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS packages (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    description    TEXT NOT NULL DEFAULT '',
    author         TEXT NOT NULL DEFAULT '',
    license        TEXT NOT NULL DEFAULT '',
    repository     TEXT NOT NULL DEFAULT '',
    homepage       TEXT NOT NULL DEFAULT '',
    keywords       TEXT[] NOT NULL DEFAULT '{}',
    categories     TEXT[] NOT NULL DEFAULT '{}',
    readme         TEXT NOT NULL DEFAULT '',
    download_count BIGINT NOT NULL DEFAULT 0,
    owner_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS versions (
    id                   BIGSERIAL PRIMARY KEY,
    package_id           BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    version              TEXT NOT NULL,
    checksum             TEXT NOT NULL,
    storage_path         TEXT NOT NULL,
    filename             TEXT NOT NULL DEFAULT '',
    file_size            BIGINT NOT NULL DEFAULT 0,
    content_type         TEXT NOT NULL DEFAULT 'application/x-nexus-package',
    yanked               BOOLEAN NOT NULL DEFAULT FALSE,
    deprecated           BOOLEAN NOT NULL DEFAULT FALSE,
    deprecation_message  TEXT NOT NULL DEFAULT '',
    published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    trusted_publisher_id BIGINT REFERENCES trusted_publishers(id) ON DELETE SET NULL,
    published_via        TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, version)
);

CREATE TABLE IF NOT EXISTS dependencies (
    id               BIGSERIAL PRIMARY KEY,
    version_id       BIGINT NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    dependency_name  TEXT NOT NULL,
    version_req      TEXT NOT NULL DEFAULT '*',
    optional         BOOLEAN NOT NULL DEFAULT FALSE,
    is_dev           BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS download_daily (
    package_id BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    count      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (package_id, day)
);

CREATE INDEX IF NOT EXISTS idx_versions_package_id ON versions (package_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_version_id ON dependencies (version_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_name_lower ON dependencies (LOWER(dependency_name));
CREATE INDEX IF NOT EXISTS idx_download_daily_package_day ON download_daily (package_id, day DESC);
`

const schemaUpgradeSQL = `
ALTER TABLE users ADD COLUMN IF NOT EXISTS use_gravatar BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_id BIGINT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_login TEXT NOT NULL DEFAULT '';
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';

ALTER TABLE packages ADD COLUMN IF NOT EXISTS license TEXT NOT NULL DEFAULT '';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS repository TEXT NOT NULL DEFAULT '';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS homepage TEXT NOT NULL DEFAULT '';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS keywords TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS categories TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS readme TEXT NOT NULL DEFAULT '';
ALTER TABLE packages ADD COLUMN IF NOT EXISTS download_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE packages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE packages ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE versions ADD COLUMN IF NOT EXISTS filename TEXT NOT NULL DEFAULT '';
ALTER TABLE versions ADD COLUMN IF NOT EXISTS file_size BIGINT NOT NULL DEFAULT 0;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS content_type TEXT NOT NULL DEFAULT 'application/x-nexus-package';
ALTER TABLE versions ADD COLUMN IF NOT EXISTS yanked BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS deprecated BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS deprecation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE versions ADD COLUMN IF NOT EXISTS published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS trusted_publisher_id BIGINT REFERENCES trusted_publishers(id) ON DELETE SET NULL;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS published_via TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS download_daily (
    package_id BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    count      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (package_id, day)
);

CREATE INDEX IF NOT EXISTS idx_dependencies_name_lower ON dependencies (LOWER(dependency_name));
CREATE INDEX IF NOT EXISTS idx_download_daily_package_day ON download_daily (package_id, day DESC);

-- Migrate legacy sessions.token -> token_hash if needed
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'sessions' AND column_name = 'token'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'sessions' AND column_name = 'token_hash'
  ) THEN
    ALTER TABLE sessions RENAME COLUMN token TO token_hash;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users (LOWER(username));
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions (token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys (prefix);
CREATE INDEX IF NOT EXISTS idx_trusted_publishers_user_id ON trusted_publishers (user_id);
CREATE INDEX IF NOT EXISTS idx_packages_name_lower ON packages (LOWER(name));
CREATE INDEX IF NOT EXISTS idx_packages_updated_at ON packages (updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_packages_download_count ON packages (download_count DESC);
CREATE INDEX IF NOT EXISTS idx_packages_owner_id ON packages (owner_id);
CREATE INDEX IF NOT EXISTS idx_packages_keywords ON packages USING gin (keywords);
CREATE INDEX IF NOT EXISTS idx_packages_categories ON packages USING gin (categories);

-- Trust & safety / publishing maturity (additive)
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_pending_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'publish';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

ALTER TABLE trusted_publishers ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE trusted_publishers ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
ALTER TABLE trusted_publishers ADD COLUMN IF NOT EXISTS last_failure_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE trusted_publishers ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ;
ALTER TABLE trusted_publishers ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

ALTER TABLE versions ADD COLUMN IF NOT EXISTS yank_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE versions ADD COLUMN IF NOT EXISTS yanked_at TIMESTAMPTZ;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS yanked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS provenance_json JSONB;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS provenance_source TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS totp_challenges (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id            BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    package_name  TEXT NOT NULL DEFAULT '',
    version       TEXT NOT NULL DEFAULT '',
    ip            TEXT NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL DEFAULT '',
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS abuse_reports (
    id               BIGSERIAL PRIMARY KEY,
    reporter_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reporter_email   TEXT NOT NULL DEFAULT '',
    package_name     TEXT NOT NULL DEFAULT '',
    version          TEXT NOT NULL DEFAULT '',
    category         TEXT NOT NULL DEFAULT 'other',
    details          TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'open',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_expires ON api_keys (expires_at);
CREATE INDEX IF NOT EXISTS idx_trusted_publishers_status ON trusted_publishers (status);
CREATE INDEX IF NOT EXISTS idx_versions_yanked ON versions (yanked) WHERE yanked = TRUE;
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_abuse_reports_status ON abuse_reports (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_totp_challenges_token ON totp_challenges (token_hash);

CREATE TABLE IF NOT EXISTS publish_rate_limits (
    user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_success_at TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_publish_rate_limits_last
    ON publish_rate_limits (last_success_at DESC);
`

// ownershipSocialSchemaSQL adds orgs/teams, multi-owner packages, auth tokens, and activity.
const ownershipSocialSchemaSQL = `
CREATE TABLE IF NOT EXISTS auth_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organizations (
    id                 BIGSERIAL PRIMARY KEY,
    slug               TEXT NOT NULL UNIQUE,
    display_name       TEXT NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    avatar_url         TEXT NOT NULL DEFAULT '',
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organization_members (
    id         BIGSERIAL PRIMARY KEY,
    org_id     BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, user_id)
);

CREATE TABLE IF NOT EXISTS teams (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

CREATE TABLE IF NOT EXISTS team_members (
    id         BIGSERIAL PRIMARY KEY,
    team_id    BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, user_id)
);

ALTER TABLE packages ADD COLUMN IF NOT EXISTS owner_org_id BIGINT REFERENCES organizations(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS package_owners (
    id                 BIGSERIAL PRIMARY KEY,
    package_id         BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    user_id            BIGINT REFERENCES users(id) ON DELETE CASCADE,
    org_id             BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    role               TEXT NOT NULL DEFAULT 'maintainer',
    invited_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    invite_token_hash  TEXT UNIQUE,
    invite_email       TEXT NOT NULL DEFAULT '',
    accepted_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL)
        OR (user_id IS NULL AND org_id IS NOT NULL)
        OR (user_id IS NULL AND org_id IS NULL AND invite_email <> '')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_package_owners_pkg_user
    ON package_owners (package_id, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_package_owners_pkg_org
    ON package_owners (package_id, org_id) WHERE org_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS ownership_transfers (
    id           BIGSERIAL PRIMARY KEY,
    package_id   BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    from_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user_id   BIGINT REFERENCES users(id) ON DELETE CASCADE,
    to_org_id    BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    status       TEXT NOT NULL DEFAULT 'pending',
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS activity_events (
    id            BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    org_id        BIGINT REFERENCES organizations(id) ON DELETE SET NULL,
    package_id    BIGINT REFERENCES packages(id) ON DELETE SET NULL,
    event_type    TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',
    meta          JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id ON auth_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_purpose ON auth_tokens (purpose);
CREATE INDEX IF NOT EXISTS idx_org_members_user_id ON organization_members (user_id);
CREATE INDEX IF NOT EXISTS idx_org_members_org_id ON organization_members (org_id);
CREATE INDEX IF NOT EXISTS idx_teams_org_id ON teams (org_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members (user_id);
CREATE INDEX IF NOT EXISTS idx_package_owners_package_id ON package_owners (package_id);
CREATE INDEX IF NOT EXISTS idx_package_owners_user_id ON package_owners (user_id);
CREATE INDEX IF NOT EXISTS idx_packages_owner_org_id ON packages (owner_org_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_created ON activity_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_events_actor ON activity_events (actor_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_events_org ON activity_events (org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_events_package ON activity_events (package_id, created_at DESC);

INSERT INTO package_owners (package_id, user_id, role, accepted_at)
SELECT p.id, p.owner_id, 'owner', NOW()
FROM packages p
WHERE p.owner_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM package_owners po
      WHERE po.package_id = p.id AND po.user_id = p.owner_id
  );
`

const trgmIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_packages_name_trgm
    ON packages USING gin (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_packages_description_trgm
    ON packages USING gin (description gin_trgm_ops);
`

const searchIndexSQL = `
DROP FUNCTION IF EXISTS nex_packages_search_tsv(text, text) CASCADE;
DROP FUNCTION IF EXISTS nex_packages_search_tsv(text, text[]) CASCADE;
DROP FUNCTION IF EXISTS nex_packages_search_tsv(text, text[], text) CASCADE;

-- STABLE (not IMMUTABLE): to_tsvector(regconfig, text) depends on dictionary state.
CREATE OR REPLACE FUNCTION nex_packages_search_tsv(name text, keywords text[], description text)
RETURNS tsvector
LANGUAGE sql
STABLE
PARALLEL SAFE
AS $$
  SELECT
    setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(array_to_string(coalesce(keywords, '{}'::text[]), ' '), '')), 'B') ||
    setweight(to_tsvector('english', coalesce(description, '')), 'C');
$$;

CREATE INDEX IF NOT EXISTS idx_packages_license_lower
    ON packages (LOWER(license));
`

// searchTSVIndexSQL is best-effort so a stubborn Postgres install cannot block boot.
const searchTSVIndexSQL = `
DO $$
BEGIN
  BEGIN
    CREATE INDEX IF NOT EXISTS idx_packages_search_tsv
      ON packages USING gin (nex_packages_search_tsv(name, keywords, description));
  EXCEPTION WHEN others THEN
    NULL;
  END;
END $$;
`

// Migrate applies the registry schema idempotently.
// Prefer versioned SQL under MIGRATIONS_DIR / ./migrations via ApplyVersionedMigrations
// after Connect; this embedded path remains the bootstrap fallback.
func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	if _, err := db.pool.Exec(ctx, coreSchemaSQL); err != nil {
		return fmt.Errorf("apply core schema: %w", err)
	}

	if _, err := db.pool.Exec(ctx, schemaUpgradeSQL); err != nil {
		return fmt.Errorf("apply schema upgrades: %w", err)
	}

	if _, err := db.pool.Exec(ctx, ownershipSocialSchemaSQL); err != nil {
		return fmt.Errorf("apply ownership/social schema: %w", err)
	}

	if _, err := db.pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`); err != nil {
		return nil
	}

	if _, err := db.pool.Exec(ctx, trgmIndexSQL); err != nil {
		return fmt.Errorf("apply trigram indexes: %w", err)
	}

	// Search helper SQL is best-effort: a stuck dependency must not block registry boot.
	if _, err := db.pool.Exec(ctx, searchIndexSQL); err != nil {
		_, _ = db.pool.Exec(ctx, `
CREATE OR REPLACE FUNCTION nex_packages_search_tsv(name text, keywords text[], description text)
RETURNS tsvector
LANGUAGE sql
STABLE
PARALLEL SAFE
AS $$
  SELECT
    setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(array_to_string(coalesce(keywords, '{}'::text[]), ' '), '')), 'B') ||
    setweight(to_tsvector('english', coalesce(description, '')), 'C');
$$;
`)
		_, _ = db.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_packages_license_lower ON packages (LOWER(license))`)
	}
	_, _ = db.pool.Exec(ctx, searchTSVIndexSQL)

	return nil
}
