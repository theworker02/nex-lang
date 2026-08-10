package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ListReverseDependencies returns packages whose latest non-yanked version
// depends on dependencyName ("used by").
func (db *DB) ListReverseDependencies(ctx context.Context, dependencyName string, limit int) ([]ReverseDependency, error) {
	dependencyName = strings.TrimSpace(dependencyName)
	if dependencyName == "" {
		return []ReverseDependency{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := db.pool.Query(ctx, `
WITH latest AS (
    SELECT DISTINCT ON (v.package_id)
        v.id AS version_id, v.package_id, v.version
    FROM versions v
    WHERE NOT v.yanked
    ORDER BY v.package_id, v.created_at DESC, v.version DESC
)
SELECT p.name, l.version, d.version_req, d.optional, d.is_dev
FROM dependencies d
JOIN latest l ON l.version_id = d.version_id
JOIN packages p ON p.id = l.package_id
WHERE LOWER(d.dependency_name) = LOWER($1)
ORDER BY p.name ASC
LIMIT $2
`, dependencyName, limit)
	if err != nil {
		return nil, fmt.Errorf("list reverse dependencies: %w", err)
	}
	defer rows.Close()

	out := make([]ReverseDependency, 0)
	for rows.Next() {
		var rd ReverseDependency
		if err := rows.Scan(&rd.PackageName, &rd.Version, &rd.VersionReq, &rd.Optional, &rd.Dev); err != nil {
			return nil, fmt.Errorf("scan reverse dependency: %w", err)
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

// SetVersionDeprecated marks or unmarks a version as deprecated (owner only).
func (db *DB) SetVersionDeprecated(ctx context.Context, name, version string, ownerID int64, deprecated bool, message string) (*PackageVersion, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	message = strings.TrimSpace(message)
	if name == "" || version == "" {
		return nil, fmt.Errorf("package name and version are required")
	}
	if ownerID <= 0 {
		return nil, fmt.Errorf("owner id is required")
	}
	if !deprecated {
		message = ""
	}

	pv, err := db.GetPackageVersion(ctx, name, version)
	if err != nil {
		return nil, err
	}
	if pv.Package.OwnerID == nil || *pv.Package.OwnerID != ownerID {
		return nil, fmt.Errorf("you are not the owner of package %q", name)
	}

	_, err = db.pool.Exec(ctx, `
UPDATE versions SET deprecated = $1, deprecation_message = $2 WHERE id = $3
`, deprecated, message, pv.Version.ID)
	if err != nil {
		return nil, fmt.Errorf("set version deprecated: %w", err)
	}
	_, _ = db.pool.Exec(ctx, `UPDATE packages SET updated_at = NOW() WHERE id = $1`, pv.Package.ID)

	pv.Version.Deprecated = deprecated
	pv.Version.DeprecationMessage = message
	return pv, nil
}

// ListDailyDownloads returns up to days rows of daily download counts (oldest first),
// filling missing days with zeros for a continuous chart window.
func (db *DB) ListDailyDownloads(ctx context.Context, packageID int64, days int) ([]DailyDownload, error) {
	if days <= 0 || days > 366 {
		days = 30
	}
	rows, err := db.pool.Query(ctx, `
WITH days AS (
    SELECT (CURRENT_DATE - ($2::int - 1 - g.i))::date AS day
    FROM generate_series(0, $2::int - 1) AS g(i)
)
SELECT d.day, COALESCE(dd.count, 0)::bigint
FROM days d
LEFT JOIN download_daily dd ON dd.package_id = $1 AND dd.day = d.day
ORDER BY d.day ASC
`, packageID, days)
	if err != nil {
		return nil, fmt.Errorf("list daily downloads: %w", err)
	}
	defer rows.Close()

	out := make([]DailyDownload, 0, days)
	var max int64
	for rows.Next() {
		var dd DailyDownload
		if err := rows.Scan(&dd.Day, &dd.Count); err != nil {
			return nil, fmt.Errorf("scan daily download: %w", err)
		}
		if dd.Count > max {
			max = dd.Count
		}
		out = append(out, dd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if max <= 0 {
			out[i].Pct = 0
		} else {
			out[i].Pct = int((out[i].Count * 100) / max)
			if out[i].Count > 0 && out[i].Pct < 4 {
				out[i].Pct = 4
			}
		}
	}
	return out, nil
}

// GetVersionFlags returns yanked/deprecated flags for a package version.
func (db *DB) GetVersionFlags(ctx context.Context, name, version string) (yanked, deprecated bool, yankReason, deprecationMessage string, err error) {
	err = db.pool.QueryRow(ctx, `
SELECT v.yanked, COALESCE(v.yank_reason, ''), v.deprecated, COALESCE(v.deprecation_message, '')
FROM versions v
JOIN packages p ON p.id = v.package_id
WHERE LOWER(p.name) = LOWER($1) AND v.version = $2
`, name, version).Scan(&yanked, &yankReason, &deprecated, &deprecationMessage)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, "", "", ErrNotFound
		}
		return false, false, "", "", fmt.Errorf("get version flags: %w", err)
	}
	return yanked, deprecated, yankReason, deprecationMessage, nil
}
