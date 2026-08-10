package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NormalizeSearchParams cleans and defaults search inputs.
func NormalizeSearchParams(p SearchParams) SearchParams {
	p.Query = strings.TrimSpace(p.Query)
	p.Category = strings.TrimSpace(strings.ToLower(p.Category))
	p.Keyword = strings.TrimSpace(strings.ToLower(p.Keyword))
	p.License = strings.TrimSpace(p.License)
	p.Sort = strings.TrimSpace(strings.ToLower(p.Sort))

	switch p.Sort {
	case "downloads", "recent", "relevance":
	default:
		if p.Query != "" {
			p.Sort = "relevance"
		} else {
			p.Sort = "downloads"
		}
	}

	if p.Limit <= 0 {
		p.Limit = 25
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

// ParseUpdatedAfter accepts RFC3339, YYYY-MM-DD, or relative windows like 7d/30d/90d.
func ParseUpdatedAfter(raw string) (time.Time, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(raw, "d") {
		num := strings.TrimSuffix(raw, "d")
		var days int
		if _, err := fmt.Sscanf(num, "%d", &days); err != nil || days <= 0 {
			return time.Time{}, fmt.Errorf("invalid updated_after duration %q", raw)
		}
		return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid updated_after %q", raw)
}

// SearchPackages finds packages by name, keywords, description, and optional filters.
// Ranking prefers exact/prefix name matches, then keyword hits, then description/full-text,
// with download-count and recency boosts. Sort may override to downloads or recent.
func (db *DB) SearchPackages(ctx context.Context, query, category string, limit int) ([]PackageSummary, error) {
	res, err := db.Search(ctx, SearchParams{
		Query:    query,
		Category: category,
		Limit:    limit,
		Sort:     "relevance",
	})
	if err != nil {
		return nil, err
	}
	return res.Packages, nil
}

// Search runs a filtered, ranked package query.
func (db *DB) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	p := NormalizeSearchParams(params)

	hasFilter := p.Query != "" || p.Category != "" || p.Keyword != "" || p.License != "" || !p.UpdatedAfter.IsZero()
	// Empty search with no filters returns nothing unless Browse is set (sorted index).
	if !hasFilter && !p.Browse {
		return &SearchResult{Packages: []PackageSummary{}, Total: 0, Params: p}, nil
	}

	escaped := escapeLike(p.Query)
	pattern := "%" + escaped + "%"
	prefix := escaped + "%"

	var updated any
	if p.UpdatedAfter.IsZero() {
		updated = nil
	} else {
		updated = p.UpdatedAfter.UTC()
	}

	orderBy := `
relevance_score DESC,
download_count DESC,
updated_at DESC,
name ASC`
	switch p.Sort {
	case "downloads":
		orderBy = `
download_count DESC,
updated_at DESC,
name ASC`
	case "recent":
		orderBy = `
updated_at DESC,
download_count DESC,
name ASC`
	}

	// relevance_score:
	//  - exact name: 1000
	//  - name prefix: 500
	//  - trigram name similarity: up to ~280
	//  - exact keyword: 220
	//  - keyword contains: 90
	//  - full-text (name A / keywords B / description C): up to ~120
	//  - description ILIKE fallback: 40
	//  - download boost: up to 40
	//  - recency boost: up to 18
	sql := fmt.Sprintf(`
WITH scored AS (
  SELECT
    p.id, p.name, p.description, p.author, p.license, p.repository, p.homepage,
    p.keywords, p.categories, '' AS readme, p.download_count, p.owner_id, p.created_at, p.updated_at,
    COALESCE(u.username, '') AS owner_username,
    COALESCE((
        SELECT v.version FROM versions v WHERE v.package_id = p.id
        ORDER BY v.yanked ASC, v.created_at DESC, v.version DESC LIMIT 1
    ), '') AS latest_version,
    (
      CASE
        WHEN $1 = '' THEN 0
        WHEN LOWER(p.name) = LOWER($1) THEN 1000
        WHEN LOWER(p.name) LIKE LOWER($4) ESCAPE '\' THEN 500
        ELSE 0
      END
      + CASE
          WHEN $1 = '' THEN 0
          ELSE LEAST(COALESCE(similarity(p.name, $1), 0) * 280.0, 280.0)
        END
      + CASE
          WHEN $1 = '' THEN 0
          WHEN EXISTS (
            SELECT 1 FROM unnest(p.keywords) kw WHERE LOWER(kw) = LOWER($1)
          ) THEN 220
          WHEN EXISTS (
            SELECT 1 FROM unnest(p.keywords) kw WHERE kw ILIKE $2 ESCAPE '\'
          ) THEN 90
          ELSE 0
        END
      + CASE
          WHEN $1 = '' THEN 0
          ELSE LEAST(
            ts_rank_cd(
              nex_packages_search_tsv(p.name, p.keywords, p.description),
              websearch_to_tsquery('english', $1)
            ) * 180.0,
            120.0
          )
        END
      + CASE
          WHEN $1 <> '' AND p.description ILIKE $2 ESCAPE '\' THEN 40
          ELSE 0
        END
      + LEAST(LN(p.download_count + 1) * 8.0, 40.0)
      + CASE
          WHEN p.updated_at > NOW() - INTERVAL '14 days' THEN 18
          WHEN p.updated_at > NOW() - INTERVAL '45 days' THEN 10
          WHEN p.updated_at > NOW() - INTERVAL '120 days' THEN 5
          ELSE 0
        END
    )::float8 AS relevance_score
  FROM packages p
  LEFT JOIN users u ON u.id = p.owner_id
  WHERE
    (
      $1 = ''
      OR LOWER(p.name) = LOWER($1)
      OR p.name ILIKE $2 ESCAPE '\'
      OR p.description ILIKE $2 ESCAPE '\'
      OR EXISTS (SELECT 1 FROM unnest(p.keywords) kw WHERE kw ILIKE $2 ESCAPE '\')
      OR (
        websearch_to_tsquery('english', $1) <> ''::tsquery
        AND nex_packages_search_tsv(p.name, p.keywords, p.description) @@ websearch_to_tsquery('english', $1)
      )
      OR COALESCE(similarity(p.name, $1), 0) > 0.28
    )
    AND ($3 = '' OR EXISTS (SELECT 1 FROM unnest(p.categories) c WHERE LOWER(c) = $3))
    AND ($5 = '' OR EXISTS (SELECT 1 FROM unnest(p.keywords) kw WHERE LOWER(kw) = $5))
    AND ($6 = '' OR LOWER(TRIM(p.license)) = LOWER(TRIM($6)))
    AND ($7::timestamptz IS NULL OR p.updated_at >= $7::timestamptz)
)
SELECT
  id, name, description, author, license, repository, homepage,
  keywords, categories, readme, download_count, owner_id, created_at, updated_at,
  owner_username, latest_version,
  COUNT(*) OVER() AS total_count
FROM scored
ORDER BY %s
LIMIT $8 OFFSET $9
`, orderBy)

	rows, err := db.pool.Query(ctx, sql,
		p.Query,   // $1
		pattern,   // $2
		p.Category, // $3
		prefix,    // $4
		p.Keyword, // $5
		p.License, // $6
		updated,   // $7
		p.Limit,   // $8
		p.Offset,  // $9
	)
	if err != nil {
		return nil, fmt.Errorf("search packages: %w", err)
	}
	defer rows.Close()

	out := make([]PackageSummary, 0, p.Limit)
	var total int64
	for rows.Next() {
		var s PackageSummary
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.Author, &s.License, &s.Repository, &s.Homepage,
			&s.Keywords, &s.Categories, &s.Readme, &s.DownloadCount, &s.OwnerID, &s.CreatedAt, &s.UpdatedAt,
			&s.OwnerUsername, &s.LatestVersion, &total,
		); err != nil {
			return nil, fmt.Errorf("scan search row: %w", err)
		}
		s.Keywords = nullStrings(s.Keywords)
		s.Categories = nullStrings(s.Categories)
		s.Readme = ""
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &SearchResult{Packages: out, Total: total, Params: p}, nil
}

// ListLicenses returns distinct non-empty licenses with package counts.
func (db *DB) ListLicenses(ctx context.Context, limit int) ([]TagCount, error) {
	if limit <= 0 || limit > 50 {
		limit = 24
	}
	rows, err := db.pool.Query(ctx, `
SELECT TRIM(license) AS name, COUNT(*)::bigint AS cnt
FROM packages
WHERE TRIM(license) <> ''
GROUP BY TRIM(license)
ORDER BY cnt DESC, name ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list licenses: %w", err)
	}
	defer rows.Close()

	out := make([]TagCount, 0, limit)
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan license: %w", err)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// TopKeywords returns the most-used package keywords (categories excluded).
func (db *DB) TopKeywords(ctx context.Context, limit int) ([]TagCount, error) {
	if limit <= 0 || limit > 100 {
		limit = 36
	}
	rows, err := db.pool.Query(ctx, `
SELECT keyword, COUNT(*)::bigint AS cnt
FROM (
    SELECT lower(trim(unnest(keywords))) AS keyword FROM packages
) t
WHERE keyword IS NOT NULL AND keyword <> ''
GROUP BY keyword
ORDER BY cnt DESC, keyword ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("top keywords: %w", err)
	}
	defer rows.Close()

	out := make([]TagCount, 0, limit)
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan keyword: %w", err)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}
