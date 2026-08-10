package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyVersionedMigrations applies numbered *.sql files from dir in lexical order.
// Each file is recorded in schema_migrations by its stem (e.g. 000001_init).
// Already-applied versions are skipped. Safe to call repeatedly.
func (db *DB) ApplyVersionedMigrations(ctx context.Context, dir string) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database not connected")
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("migrations directory is empty")
	}
	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("migrations dir: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("migrations path is not a directory: %s", dir)
	}

	if _, err := db.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, filepath.Ext(name))
		var exists bool
		if err := db.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			if _, err := db.pool.Exec(ctx,
				`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, version,
			); err != nil {
				return fmt.Errorf("record empty migration %s: %w", version, err)
			}
			continue
		}

		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, sqlText); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}
	return nil
}

// ResolveMigrationsDir returns MIGRATIONS_DIR, or the first existing candidate.
func ResolveMigrationsDir(candidates ...string) string {
	if v := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); v != "" {
		return v
	}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			matches, _ := filepath.Glob(filepath.Join(c, "*.sql"))
			if len(matches) > 0 {
				if abs, err := filepath.Abs(c); err == nil {
					return abs
				}
				return c
			}
		}
	}
	return ""
}
