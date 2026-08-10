package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when a row cannot be located.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a unique constraint is violated.
	ErrConflict = errors.New("conflict")
)

func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

func nullStrings(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// UpsertPackageVersion creates or updates the package row and inserts a new
// version. Existing versions are rejected to preserve immutability of releases.
func (db *DB) UpsertPackageVersion(ctx context.Context, in PublishInput) (*PackageVersion, error) {
	in.Keywords = nullStrings(in.Keywords)
	in.Categories = nullStrings(in.Categories)
	if in.ContentType == "" {
		in.ContentType = "application/x-nexus-package"
	}
	if in.PublishedVia == "" {
		in.PublishedVia = "api_key"
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin publish transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var pkg Package
	var ownerID *int64
	if in.OwnerID > 0 {
		ownerID = &in.OwnerID
	}

	err = tx.QueryRow(ctx, `
INSERT INTO packages (
    name, description, author, license, repository, homepage,
    keywords, categories, readme, owner_id, updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, NOW())
ON CONFLICT (name) DO UPDATE
SET
    description = EXCLUDED.description,
    author = CASE WHEN packages.author = '' THEN EXCLUDED.author ELSE packages.author END,
    license = CASE WHEN EXCLUDED.license <> '' THEN EXCLUDED.license ELSE packages.license END,
    repository = CASE WHEN EXCLUDED.repository <> '' THEN EXCLUDED.repository ELSE packages.repository END,
    homepage = CASE WHEN EXCLUDED.homepage <> '' THEN EXCLUDED.homepage ELSE packages.homepage END,
    keywords = CASE WHEN cardinality(EXCLUDED.keywords) > 0 THEN EXCLUDED.keywords ELSE packages.keywords END,
    categories = CASE WHEN cardinality(EXCLUDED.categories) > 0 THEN EXCLUDED.categories ELSE packages.categories END,
    readme = CASE WHEN EXCLUDED.readme <> '' THEN EXCLUDED.readme ELSE packages.readme END,
    owner_id = COALESCE(packages.owner_id, EXCLUDED.owner_id),
    updated_at = NOW()
RETURNING id, name, description, author, license, repository, homepage,
          keywords, categories, readme, download_count, owner_id, created_at, updated_at
`, in.Name, in.Description, in.Author, in.License, in.Repository, in.Homepage,
		in.Keywords, in.Categories, in.Readme, ownerID).Scan(
		&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Author, &pkg.License, &pkg.Repository, &pkg.Homepage,
		&pkg.Keywords, &pkg.Categories, &pkg.Readme, &pkg.DownloadCount, &pkg.OwnerID, &pkg.CreatedAt, &pkg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert package: %w", err)
	}
	pkg.Keywords = nullStrings(pkg.Keywords)
	pkg.Categories = nullStrings(pkg.Categories)

	// Ownership enforcement: primary owner_id OR package_owners / org membership.
	if in.OwnerID > 0 {
		can, err := db.UserCanPublish(ctx, in.OwnerID, in.Name)
		if err != nil {
			return nil, err
		}
		if !can {
			return nil, fmt.Errorf("%w: you are not an owner of package %q", ErrConflict, in.Name)
		}
	}

	var pubBy *int64
	if in.PublishedByUserID > 0 {
		pubBy = &in.PublishedByUserID
	}

	var ver Version
	err = tx.QueryRow(ctx, `
INSERT INTO versions (
    package_id, version, checksum, storage_path, filename, file_size, content_type,
    published_by_user_id, trusted_publisher_id, published_via
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, package_id, version, checksum, storage_path, filename, file_size, content_type,
          yanked, yank_reason, published_by_user_id, trusted_publisher_id, published_via, created_at
`, pkg.ID, in.Version, in.Checksum, in.StoragePath, in.Filename, in.FileSize, in.ContentType,
		pubBy, in.TrustedPublisherID, in.PublishedVia).Scan(
		&ver.ID, &ver.PackageID, &ver.Version, &ver.Checksum, &ver.StoragePath, &ver.Filename,
		&ver.FileSize, &ver.ContentType, &ver.Yanked, &ver.YankReason, &ver.PublishedByUserID, &ver.TrustedPublisherID,
		&ver.PublishedVia, &ver.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: version %q of package %q already exists", ErrConflict, in.Version, in.Name)
		}
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if in.OwnerID > 0 {
		_, _ = tx.Exec(ctx, `
INSERT INTO package_owners (package_id, user_id, role, accepted_at)
SELECT $1, $2, 'owner', NOW()
WHERE NOT EXISTS (
  SELECT 1 FROM package_owners WHERE package_id = $1 AND user_id = $2
)
`, pkg.ID, in.OwnerID)
	}

	deps := make([]Dependency, 0, len(in.Dependencies))
	for _, d := range in.Dependencies {
		var dep Dependency
		err = tx.QueryRow(ctx, `
INSERT INTO dependencies (version_id, dependency_name, version_req, optional, is_dev)
VALUES ($1,$2,$3,$4,$5)
RETURNING id, version_id, dependency_name, version_req, optional, is_dev
`, ver.ID, d.Name, d.VersionReq, d.Optional, d.Dev).Scan(
			&dep.ID, &dep.VersionID, &dep.DependencyName, &dep.VersionReq, &dep.Optional, &dep.Dev,
		)
		if err != nil {
			return nil, fmt.Errorf("insert dependency: %w", err)
		}
		deps = append(deps, dep)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit publish transaction: %w", err)
	}

	if in.PublishedByUserID > 0 {
		actor := in.PublishedByUserID
		pkgID := pkg.ID
		_ = db.RecordActivity(ctx, &ActivityEvent{
			ActorUserID: &actor,
			PackageID:   &pkgID,
			EventType:   "package.publish",
			Summary:     fmt.Sprintf("Published %s@%s", pkg.Name, ver.Version),
			Meta:        mustJSON(map[string]any{"package": pkg.Name, "version": ver.Version}),
		})
	}

	return &PackageVersion{Package: pkg, Version: ver, Dependencies: deps}, nil
}

// GetPackageVersion returns package and version metadata for a name/version pair.
func (db *DB) GetPackageVersion(ctx context.Context, name, version string) (*PackageVersion, error) {
	var pv PackageVersion
	err := db.reader().QueryRow(ctx, `
SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, p.readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, ''),
    v.id, v.package_id, v.version, v.checksum, v.storage_path, v.filename, v.file_size, v.content_type,
    v.yanked, v.yank_reason, v.published_by_user_id, v.trusted_publisher_id, v.published_via, v.created_at
FROM packages p
JOIN versions v ON v.package_id = p.id
LEFT JOIN users u ON u.id = p.owner_id
WHERE LOWER(p.name) = LOWER($1) AND v.version = $2
`, name, version).Scan(
		&pv.Package.ID, &pv.Package.Name, &pv.Package.Description, &pv.Package.Author, &pv.Package.License,
		&pv.Package.Repository, &pv.Package.Homepage, &pv.Package.Keywords, &pv.Package.Categories,
		&pv.Package.Readme, &pv.Package.DownloadCount, &pv.Package.OwnerID, &pv.Package.CreatedAt, &pv.Package.UpdatedAt,
		&pv.Package.OwnerUsername,
		&pv.Version.ID, &pv.Version.PackageID, &pv.Version.Version, &pv.Version.Checksum, &pv.Version.StoragePath,
		&pv.Version.Filename, &pv.Version.FileSize, &pv.Version.ContentType, &pv.Version.Yanked, &pv.Version.YankReason,
		&pv.Version.PublishedByUserID, &pv.Version.TrustedPublisherID, &pv.Version.PublishedVia, &pv.Version.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get package version: %w", err)
	}
	pv.Package.Keywords = nullStrings(pv.Package.Keywords)
	pv.Package.Categories = nullStrings(pv.Package.Categories)

	deps, err := db.ListDependencies(ctx, pv.Version.ID)
	if err != nil {
		return nil, err
	}
	pv.Dependencies = deps
	return &pv, nil
}

// ListDependencies returns dependencies for a version.
func (db *DB) ListDependencies(ctx context.Context, versionID int64) ([]Dependency, error) {
	rows, err := db.reader().Query(ctx, `
SELECT id, version_id, dependency_name, version_req, optional, is_dev
FROM dependencies WHERE version_id = $1
ORDER BY dependency_name
`, versionID)
	if err != nil {
		return nil, fmt.Errorf("list dependencies: %w", err)
	}
	defer rows.Close()

	out := make([]Dependency, 0)
	for rows.Next() {
		var d Dependency
		if err := rows.Scan(&d.ID, &d.VersionID, &d.DependencyName, &d.VersionReq, &d.Optional, &d.Dev); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		d.Name = d.DependencyName
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListPackagesPage returns a paginated package index.
func (db *DB) ListPackagesPage(ctx context.Context, page, perPage int) ([]PackageSummary, Page, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 25
	}

	var total int64
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM packages`).Scan(&total); err != nil {
		return nil, Page{}, fmt.Errorf("count packages: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := db.pool.Query(ctx, `
SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, '' AS readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, ''),
    COALESCE((
        SELECT v.version FROM versions v WHERE v.package_id = p.id
        ORDER BY v.yanked ASC, v.created_at DESC, v.version DESC LIMIT 1
    ), '') AS latest_version
FROM packages p
LEFT JOIN users u ON u.id = p.owner_id
ORDER BY p.name ASC
LIMIT $1 OFFSET $2
`, perPage, offset)
	if err != nil {
		return nil, Page{}, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	items, err := scanPackageSummaries(rows)
	if err != nil {
		return nil, Page{}, err
	}

	meta := Page{
		Page:    page,
		PerPage: perPage,
		Total:   total,
		HasPrev: page > 1,
		HasNext: int64(offset+len(items)) < total,
	}
	return items, meta, nil
}

// ListRecentPackages returns packages ordered by most recent update.
func (db *DB) ListRecentPackages(ctx context.Context, limit int) ([]PackageSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 12
	}
	rows, err := db.pool.Query(ctx, `
SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, '' AS readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, ''),
    COALESCE((
        SELECT v.version FROM versions v WHERE v.package_id = p.id
        ORDER BY v.yanked ASC, v.created_at DESC, v.version DESC LIMIT 1
    ), '') AS latest_version
FROM packages p
LEFT JOIN users u ON u.id = p.owner_id
ORDER BY p.updated_at DESC, p.created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent packages: %w", err)
	}
	defer rows.Close()
	return scanPackageSummaries(rows)
}

// ListPopularPackages returns packages ordered by download count.
func (db *DB) ListPopularPackages(ctx context.Context, limit int) ([]PackageSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 12
	}
	rows, err := db.pool.Query(ctx, `
SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, '' AS readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, ''),
    COALESCE((
        SELECT v.version FROM versions v WHERE v.package_id = p.id
        ORDER BY v.yanked ASC, v.created_at DESC, v.version DESC LIMIT 1
    ), '') AS latest_version
FROM packages p
LEFT JOIN users u ON u.id = p.owner_id
ORDER BY p.download_count DESC, p.updated_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list popular packages: %w", err)
	}
	defer rows.Close()
	return scanPackageSummaries(rows)
}

// ListPackagesByOwner returns packages owned or maintained by a user.
func (db *DB) ListPackagesByOwner(ctx context.Context, ownerID int64) ([]PackageSummary, error) {
	return db.ListPackagesForUser(ctx, ownerID)
}

// GetPackageByName returns a package row by name (case-insensitive).
func (db *DB) GetPackageByName(ctx context.Context, name string) (*Package, error) {
	var pkg Package
	err := db.pool.QueryRow(ctx, `
SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, p.readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, '')
FROM packages p
LEFT JOIN users u ON u.id = p.owner_id
WHERE LOWER(p.name) = LOWER($1)
`, name).Scan(
		&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Author, &pkg.License, &pkg.Repository, &pkg.Homepage,
		&pkg.Keywords, &pkg.Categories, &pkg.Readme, &pkg.DownloadCount, &pkg.OwnerID, &pkg.CreatedAt, &pkg.UpdatedAt,
		&pkg.OwnerUsername,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get package: %w", err)
	}
	pkg.Keywords = nullStrings(pkg.Keywords)
	pkg.Categories = nullStrings(pkg.Categories)
	return &pkg, nil
}

// ListPackageVersions returns all versions for a package, newest first.
func (db *DB) ListPackageVersions(ctx context.Context, packageID int64) ([]Version, error) {
	rows, err := db.pool.Query(ctx, `
SELECT id, package_id, version, checksum, storage_path, filename, file_size, content_type,
       yanked, yank_reason, published_by_user_id, trusted_publisher_id, published_via, created_at
FROM versions WHERE package_id = $1
ORDER BY created_at DESC, version DESC
`, packageID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	versions := make([]Version, 0)
	for rows.Next() {
		var ver Version
		if err := rows.Scan(
			&ver.ID, &ver.PackageID, &ver.Version, &ver.Checksum, &ver.StoragePath, &ver.Filename,
			&ver.FileSize, &ver.ContentType, &ver.Yanked, &ver.YankReason, &ver.PublishedByUserID, &ver.TrustedPublisherID,
			&ver.PublishedVia, &ver.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, ver)
	}
	return versions, rows.Err()
}

// YankPackageVersion marks a published version as yanked (owner only).
// Yanked versions remain downloadable for existing lockfiles but should not be newly resolved.
func (db *DB) YankPackageVersion(ctx context.Context, name, version string, ownerID int64, reason string) (*PackageVersion, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	reason = strings.TrimSpace(reason)
	if name == "" || version == "" {
		return nil, fmt.Errorf("package name and version are required")
	}
	if ownerID <= 0 {
		return nil, fmt.Errorf("owner id is required")
	}

	pv, err := db.GetPackageVersion(ctx, name, version)
	if err != nil {
		return nil, err
	}
	if pv.Package.OwnerID == nil || *pv.Package.OwnerID != ownerID {
		return nil, fmt.Errorf("you are not the owner of package %q", name)
	}
	if pv.Version.Yanked {
		_, err := db.pool.Exec(ctx, `
UPDATE versions SET yank_reason = $1 WHERE id = $2
`, reason, pv.Version.ID)
		if err != nil {
			return nil, fmt.Errorf("update yank reason: %w", err)
		}
		pv.Version.YankReason = reason
		return pv, nil
	}

	_, err = db.pool.Exec(ctx, `
UPDATE versions
SET yanked = TRUE, yank_reason = $1, yanked_at = NOW(), yanked_by_user_id = $3
WHERE id = $2
`, reason, pv.Version.ID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("yank version: %w", err)
	}
	now := time.Now().UTC()
	pv.Version.Yanked = true
	pv.Version.YankReason = reason
	pv.Version.YankedAt = &now
	pv.Version.YankedByUserID = &ownerID
	return pv, nil
}

// IncrementDownloadCount atomically bumps the package download counter and today's daily bucket.
func (db *DB) IncrementDownloadCount(ctx context.Context, packageID int64) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin download increment: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `UPDATE packages SET download_count = download_count + 1 WHERE id = $1`, packageID)
	if err != nil {
		return fmt.Errorf("increment download count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `
INSERT INTO download_daily (package_id, day, count)
VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (package_id, day) DO UPDATE SET count = download_daily.count + 1
`, packageID)
	if err != nil {
		return fmt.Errorf("increment daily download: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit download increment: %w", err)
	}
	return nil
}

// CountPackages returns the total number of registered packages.
func (db *DB) CountPackages(ctx context.Context) (int64, error) {
	var n int64
	if err := db.reader().QueryRow(ctx, `SELECT COUNT(*) FROM packages`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count packages: %w", err)
	}
	return n, nil
}

// CountVersions returns the total number of published versions.
func (db *DB) CountVersions(ctx context.Context) (int64, error) {
	var n int64
	if err := db.reader().QueryRow(ctx, `SELECT COUNT(*) FROM versions`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count versions: %w", err)
	}
	return n, nil
}

// CountUsers returns the number of registered accounts.
func (db *DB) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	if err := db.reader().QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// SumDownloads returns the sum of package download counters.
func (db *DB) SumDownloads(ctx context.Context) (int64, error) {
	var n int64
	if err := db.reader().QueryRow(ctx, `SELECT COALESCE(SUM(download_count), 0) FROM packages`).Scan(&n); err != nil {
		return 0, fmt.Errorf("sum downloads: %w", err)
	}
	return n, nil
}

// TopTags returns the most-used categories and keywords across packages.
func (db *DB) TopTags(ctx context.Context, limit int) ([]TagCount, error) {
	if limit <= 0 || limit > 50 {
		limit = 16
	}
	rows, err := db.reader().Query(ctx, `
SELECT tag, COUNT(*)::bigint AS cnt
FROM (
    SELECT lower(trim(unnest(categories))) AS tag FROM packages
    UNION ALL
    SELECT lower(trim(unnest(keywords))) AS tag FROM packages
) t
WHERE tag IS NOT NULL AND tag <> ''
GROUP BY tag
ORDER BY cnt DESC, tag ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("top tags: %w", err)
	}
	defer rows.Close()

	out := make([]TagCount, 0, limit)
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// HubStats returns aggregate registry counters plus top tags.
func (db *DB) HubStats(ctx context.Context) (*HubStats, error) {
	s := &HubStats{}
	var err error
	if s.Packages, err = db.CountPackages(ctx); err != nil {
		return nil, err
	}
	if s.Versions, err = db.CountVersions(ctx); err != nil {
		return nil, err
	}
	if s.Downloads, err = db.SumDownloads(ctx); err != nil {
		return nil, err
	}
	if s.Users, err = db.CountUsers(ctx); err != nil {
		return nil, err
	}
	if s.Tags, err = db.TopTags(ctx, 16); err != nil {
		return nil, err
	}
	return s, nil
}

func scanPackageSummaries(rows pgx.Rows) ([]PackageSummary, error) {
	out := make([]PackageSummary, 0)
	for rows.Next() {
		var s PackageSummary
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.Author, &s.License, &s.Repository, &s.Homepage,
			&s.Keywords, &s.Categories, &s.Readme, &s.DownloadCount, &s.OwnerID, &s.CreatedAt, &s.UpdatedAt,
			&s.OwnerUsername, &s.LatestVersion,
		); err != nil {
			return nil, fmt.Errorf("scan package summary: %w", err)
		}
		s.Keywords = nullStrings(s.Keywords)
		s.Categories = nullStrings(s.Categories)
		s.Readme = ""
		out = append(out, s)
	}
	return out, rows.Err()
}
