package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func randomToken(prefix string) (plaintext, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	plaintext = prefix + hex.EncodeToString(raw)
	return plaintext, HashToken(plaintext), nil
}

// CreateAuthToken mints a one-time email-verify or password-reset token.
func (db *DB) CreateAuthToken(ctx context.Context, userID int64, purpose string, ttl time.Duration) (plaintext string, err error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	plaintext, hash, err := randomToken("nxtk_")
	if err != nil {
		return "", fmt.Errorf("generate auth token: %w", err)
	}
	_, err = db.pool.Exec(ctx, `
INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
`, userID, purpose, hash, time.Now().UTC().Add(ttl))
	if err != nil {
		return "", fmt.Errorf("create auth token: %w", err)
	}
	return plaintext, nil
}

// ConsumeAuthToken validates and marks a token used; returns the owning user.
func (db *DB) ConsumeAuthToken(ctx context.Context, plaintext, purpose string) (*User, error) {
	hash := HashToken(plaintext)
	var userID int64
	err := db.pool.QueryRow(ctx, `
UPDATE auth_tokens
SET used_at = NOW()
WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > NOW()
RETURNING user_id
`, hash, purpose).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("consume auth token: %w", err)
	}
	return db.GetUserByID(ctx, userID)
}

// PeekAuthToken validates a token without consuming it.
func (db *DB) PeekAuthToken(ctx context.Context, plaintext, purpose string) (*User, error) {
	hash := HashToken(plaintext)
	var userID int64
	err := db.pool.QueryRow(ctx, `
SELECT user_id FROM auth_tokens
WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > NOW()
`, hash, purpose).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("peek auth token: %w", err)
	}
	return db.GetUserByID(ctx, userID)
}

// CreateOrganization creates an org and adds the creator as owner.
func (db *DB) CreateOrganization(ctx context.Context, slug, displayName, description, avatarURL string, creatorID int64) (*Organization, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = slug
	}
	if _, err := db.GetUserByUsername(ctx, slug); err == nil {
		return nil, fmt.Errorf("%w: slug conflicts with an existing username", ErrConflict)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var org Organization
	err = tx.QueryRow(ctx, `
INSERT INTO organizations (slug, display_name, description, avatar_url, created_by_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, slug, display_name, description, avatar_url, created_by_user_id, created_at, updated_at
`, slug, displayName, description, avatarURL, creatorID).Scan(
		&org.ID, &org.Slug, &org.DisplayName, &org.Description, &org.AvatarURL,
		&org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: organization slug already taken", ErrConflict)
		}
		return nil, fmt.Errorf("create organization: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, 'owner')
`, org.ID, creatorID)
	if err != nil {
		return nil, fmt.Errorf("add org owner: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &creatorID,
		OrgID:       &org.ID,
		EventType:   "org.create",
		Summary:     fmt.Sprintf("Created organization @%s", org.Slug),
	})
	return &org, nil
}

// GetOrganizationBySlug loads an org by slug.
func (db *DB) GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error) {
	var org Organization
	err := db.pool.QueryRow(ctx, `
SELECT o.id, o.slug, o.display_name, o.description, o.avatar_url, o.created_by_user_id, o.created_at, o.updated_at,
       (SELECT COUNT(*) FROM organization_members m WHERE m.org_id = o.id),
       (SELECT COUNT(*) FROM packages p WHERE p.owner_org_id = o.id
         OR EXISTS (SELECT 1 FROM package_owners po WHERE po.org_id = o.id AND po.package_id = p.id AND po.accepted_at IS NOT NULL))
FROM organizations o
WHERE LOWER(o.slug) = LOWER($1)
`, slug).Scan(
		&org.ID, &org.Slug, &org.DisplayName, &org.Description, &org.AvatarURL,
		&org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt, &org.MemberCount, &org.PackageCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get organization: %w", err)
	}
	return &org, nil
}

// GetOrganizationByID loads an org by id.
func (db *DB) GetOrganizationByID(ctx context.Context, id int64) (*Organization, error) {
	var org Organization
	err := db.pool.QueryRow(ctx, `
SELECT id, slug, display_name, description, avatar_url, created_by_user_id, created_at, updated_at
FROM organizations WHERE id = $1
`, id).Scan(
		&org.ID, &org.Slug, &org.DisplayName, &org.Description, &org.AvatarURL,
		&org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

// ListOrganizationsForUser returns orgs the user belongs to.
func (db *DB) ListOrganizationsForUser(ctx context.Context, userID int64) ([]Organization, error) {
	rows, err := db.pool.Query(ctx, `
SELECT o.id, o.slug, o.display_name, o.description, o.avatar_url, o.created_by_user_id, o.created_at, o.updated_at,
       (SELECT COUNT(*) FROM organization_members m WHERE m.org_id = o.id),
       (SELECT COUNT(*) FROM packages p WHERE p.owner_org_id = o.id)
FROM organizations o
JOIN organization_members om ON om.org_id = o.id
WHERE om.user_id = $1
ORDER BY o.slug
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Organization, 0)
	for rows.Next() {
		var org Organization
		if err := rows.Scan(
			&org.ID, &org.Slug, &org.DisplayName, &org.Description, &org.AvatarURL,
			&org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt, &org.MemberCount, &org.PackageCount,
		); err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	return out, rows.Err()
}

// OrgMemberRole returns the user's role in an org, or empty if not a member.
func (db *DB) OrgMemberRole(ctx context.Context, orgID, userID int64) (string, error) {
	var role string
	err := db.pool.QueryRow(ctx, `
SELECT role FROM organization_members WHERE org_id = $1 AND user_id = $2
`, orgID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

// ListOrgMembers lists members of an organization.
func (db *DB) ListOrgMembers(ctx context.Context, orgID int64) ([]OrgMember, error) {
	rows, err := db.pool.Query(ctx, `
SELECT m.id, m.org_id, m.user_id, m.role, m.created_at, u.username, u.avatar_url
FROM organization_members m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.username
`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OrgMember, 0)
	for rows.Next() {
		var m OrgMember
		if err := rows.Scan(&m.ID, &m.OrgID, &m.UserID, &m.Role, &m.CreatedAt, &m.Username, &m.AvatarURL); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddOrgMember adds a user to an organization.
func (db *DB) AddOrgMember(ctx context.Context, orgID, userID int64, role string, actorID int64) (*OrgMember, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "member"
	}
	if role != "owner" && role != "admin" && role != "member" {
		return nil, fmt.Errorf("invalid role")
	}
	var m OrgMember
	err := db.pool.QueryRow(ctx, `
INSERT INTO organization_members (org_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role
RETURNING id, org_id, user_id, role, created_at
`, orgID, userID, role).Scan(&m.ID, &m.OrgID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add org member: %w", err)
	}
	if u, err := db.GetUserByID(ctx, userID); err == nil {
		m.Username = u.Username
		m.AvatarURL = u.AvatarURL
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &actorID,
		OrgID:       &orgID,
		EventType:   "org.member_add",
		Summary:     fmt.Sprintf("Added %s as %s", m.Username, role),
	})
	return &m, nil
}

// RemoveOrgMember removes a user from an organization.
func (db *DB) RemoveOrgMember(ctx context.Context, orgID, userID, actorID int64) error {
	var owners int64
	_ = db.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM organization_members WHERE org_id = $1 AND role = 'owner'
`, orgID).Scan(&owners)
	var role string
	_ = db.pool.QueryRow(ctx, `
SELECT role FROM organization_members WHERE org_id = $1 AND user_id = $2
`, orgID, userID).Scan(&role)
	if role == "owner" && owners <= 1 {
		return fmt.Errorf("cannot remove the last organization owner")
	}
	tag, err := db.pool.Exec(ctx, `DELETE FROM organization_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &actorID,
		OrgID:       &orgID,
		EventType:   "org.member_remove",
		Summary:     "Removed organization member",
	})
	return nil
}

// CreateTeam creates a team under an organization.
func (db *DB) CreateTeam(ctx context.Context, orgID int64, name, description string) (*Team, error) {
	name = strings.TrimSpace(name)
	var t Team
	err := db.pool.QueryRow(ctx, `
INSERT INTO teams (org_id, name, description) VALUES ($1, $2, $3)
RETURNING id, org_id, name, description, created_at
`, orgID, name, description).Scan(&t.ID, &t.OrgID, &t.Name, &t.Description, &t.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: team name already exists", ErrConflict)
		}
		return nil, err
	}
	return &t, nil
}

// ListTeams lists teams for an organization.
func (db *DB) ListTeams(ctx context.Context, orgID int64) ([]Team, error) {
	rows, err := db.pool.Query(ctx, `
SELECT t.id, t.org_id, t.name, t.description, t.created_at,
       (SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id)
FROM teams t WHERE t.org_id = $1 ORDER BY t.name
`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Team, 0)
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.Description, &t.CreatedAt, &t.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddTeamMember adds a user to a team (must already be an org member).
func (db *DB) AddTeamMember(ctx context.Context, teamID, userID int64, role string) (*TeamMember, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "member"
	}
	var orgID int64
	if err := db.pool.QueryRow(ctx, `SELECT org_id FROM teams WHERE id = $1`, teamID).Scan(&orgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	roleInOrg, err := db.OrgMemberRole(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if roleInOrg == "" {
		return nil, fmt.Errorf("user must be an organization member first")
	}
	var m TeamMember
	err = db.pool.QueryRow(ctx, `
INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role
RETURNING id, team_id, user_id, role, created_at
`, teamID, userID, role).Scan(&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	if u, err := db.GetUserByID(ctx, userID); err == nil {
		m.Username = u.Username
		m.AvatarURL = u.AvatarURL
	}
	return &m, nil
}

// ListTeamMembers lists members of a team.
func (db *DB) ListTeamMembers(ctx context.Context, teamID int64) ([]TeamMember, error) {
	rows, err := db.pool.Query(ctx, `
SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.created_at, u.username, u.avatar_url
FROM team_members tm
JOIN users u ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY u.username
`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TeamMember, 0)
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.CreatedAt, &m.Username, &m.AvatarURL); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListPackageOwners returns owners/maintainers for a package.
func (db *DB) ListPackageOwners(ctx context.Context, packageID int64) ([]PackageOwner, error) {
	rows, err := db.pool.Query(ctx, `
SELECT po.id, po.package_id, po.user_id, po.org_id, po.role, po.invited_by_user_id, po.invite_email,
       po.accepted_at, po.created_at,
       COALESCE(u.username, ''), COALESCE(o.slug, ''), COALESCE(o.display_name, '')
FROM package_owners po
LEFT JOIN users u ON u.id = po.user_id
LEFT JOIN organizations o ON o.id = po.org_id
WHERE po.package_id = $1
ORDER BY CASE po.role WHEN 'owner' THEN 0 ELSE 1 END, po.created_at
`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PackageOwner, 0)
	for rows.Next() {
		var o PackageOwner
		if err := rows.Scan(
			&o.ID, &o.PackageID, &o.UserID, &o.OrgID, &o.Role, &o.InvitedByUserID, &o.InviteEmail,
			&o.AcceptedAt, &o.CreatedAt, &o.Username, &o.OrgSlug, &o.OrgDisplayName,
		); err != nil {
			return nil, err
		}
		o.Pending = o.AcceptedAt == nil
		out = append(out, o)
	}
	return out, rows.Err()
}

// UserCanPublish reports whether a user may publish to a package (or create it).
func (db *DB) UserCanPublish(ctx context.Context, userID int64, packageName string) (bool, error) {
	pkg, err := db.GetPackageByName(ctx, packageName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return true, nil // new package
		}
		return false, err
	}
	if pkg.OwnerID != nil && *pkg.OwnerID == userID {
		return true, nil
	}
	var n int64
	err = db.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM package_owners po
WHERE po.package_id = $1
  AND po.accepted_at IS NOT NULL
  AND (
    po.user_id = $2
    OR (
      po.org_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM organization_members om
        WHERE om.org_id = po.org_id AND om.user_id = $2
          AND om.role IN ('owner', 'admin', 'member')
      )
    )
  )
`, pkg.ID, userID).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	// Also allow via owner_org_id membership
	if pkg.OwnerOrgID != nil {
		role, err := db.OrgMemberRole(ctx, *pkg.OwnerOrgID, userID)
		if err != nil {
			return false, err
		}
		if role != "" {
			return true, nil
		}
	}
	return false, nil
}

// EnsurePackageOwner inserts a user as package owner if missing (used on first publish).
func (db *DB) EnsurePackageOwner(ctx context.Context, packageID, userID int64, role string) error {
	if role == "" {
		role = "owner"
	}
	_, err := db.pool.Exec(ctx, `
INSERT INTO package_owners (package_id, user_id, role, accepted_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT DO NOTHING
`, packageID, userID, role)
	// unique index is partial; ON CONFLICT DO NOTHING needs a constraint name.
	// Fall back to existence check if insert fails.
	if err != nil {
		var n int64
		_ = db.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM package_owners WHERE package_id = $1 AND user_id = $2
`, packageID, userID).Scan(&n)
		if n > 0 {
			return nil
		}
		_, err = db.pool.Exec(ctx, `
INSERT INTO package_owners (package_id, user_id, role, accepted_at)
SELECT $1, $2, $3, NOW()
WHERE NOT EXISTS (
  SELECT 1 FROM package_owners WHERE package_id = $1 AND user_id = $2
)
`, packageID, userID, role)
		return err
	}
	return nil
}

// InvitePackageOwner invites a user (by username) or pending email as maintainer/owner.
func (db *DB) InvitePackageOwner(ctx context.Context, packageID, actorID int64, username, email, role string) (*PackageOwner, string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "maintainer"
	}
	if role != "owner" && role != "maintainer" {
		return nil, "", fmt.Errorf("invalid role")
	}
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	var userID *int64
	if username != "" {
		u, err := db.GetUserByUsername(ctx, username)
		if err != nil {
			return nil, "", err
		}
		userID = &u.ID
		email = ""
	} else if email == "" {
		return nil, "", fmt.Errorf("username or email required")
	}

	plaintext, hash, err := randomToken("ninv_")
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	var accepted *time.Time
	inviteHash := &hash
	if userID != nil {
		accepted = &now
		inviteHash = nil
		plaintext = ""
	}

	var o PackageOwner
	err = db.pool.QueryRow(ctx, `
INSERT INTO package_owners (package_id, user_id, role, invited_by_user_id, invite_token_hash, invite_email, accepted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, package_id, user_id, org_id, role, invited_by_user_id, invite_email, accepted_at, created_at
`, packageID, userID, role, actorID, inviteHash, email, accepted).Scan(
		&o.ID, &o.PackageID, &o.UserID, &o.OrgID, &o.Role, &o.InvitedByUserID, &o.InviteEmail, &o.AcceptedAt, &o.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, "", fmt.Errorf("%w: already an owner", ErrConflict)
		}
		return nil, "", fmt.Errorf("invite owner: %w", err)
	}
	o.Pending = o.AcceptedAt == nil
	if userID != nil {
		if u, err := db.GetUserByID(ctx, *userID); err == nil {
			o.Username = u.Username
		}
	}
	pkgName := ""
	if pkg, err := db.getPackageNameByID(ctx, packageID); err == nil {
		pkgName = pkg
	}
	summary := "Added package maintainer"
	if userID != nil {
		summary = fmt.Sprintf("Added %s as %s", o.Username, role)
	} else {
		summary = fmt.Sprintf("Invited %s as %s", email, role)
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &actorID,
		PackageID:   &packageID,
		EventType:   "ownership.add",
		Summary:     summary,
		Meta:        mustJSON(map[string]any{"package": pkgName, "role": role}),
	})
	return &o, plaintext, nil
}

// AddOrgAsPackageOwner grants an organization ownership/maintainer rights.
func (db *DB) AddOrgAsPackageOwner(ctx context.Context, packageID, orgID, actorID int64, role string) (*PackageOwner, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "maintainer"
	}
	var o PackageOwner
	err := db.pool.QueryRow(ctx, `
INSERT INTO package_owners (package_id, org_id, role, invited_by_user_id, accepted_at)
VALUES ($1, $2, $3, $4, NOW())
RETURNING id, package_id, user_id, org_id, role, invited_by_user_id, invite_email, accepted_at, created_at
`, packageID, orgID, role, actorID).Scan(
		&o.ID, &o.PackageID, &o.UserID, &o.OrgID, &o.Role, &o.InvitedByUserID, &o.InviteEmail, &o.AcceptedAt, &o.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: org already an owner", ErrConflict)
		}
		return nil, err
	}
	if org, err := db.GetOrganizationByID(ctx, orgID); err == nil {
		o.OrgSlug = org.Slug
		o.OrgDisplayName = org.DisplayName
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &actorID,
		PackageID:   &packageID,
		OrgID:       &orgID,
		EventType:   "ownership.add",
		Summary:     fmt.Sprintf("Added organization @%s as %s", o.OrgSlug, role),
	})
	return &o, nil
}

// AcceptOwnerInvite accepts a pending email invite.
func (db *DB) AcceptOwnerInvite(ctx context.Context, plaintext string, userID int64) (*PackageOwner, error) {
	hash := HashToken(plaintext)
	var o PackageOwner
	err := db.pool.QueryRow(ctx, `
UPDATE package_owners
SET user_id = $2, accepted_at = NOW(), invite_token_hash = NULL, invite_email = ''
WHERE invite_token_hash = $1 AND accepted_at IS NULL AND user_id IS NULL
RETURNING id, package_id, user_id, org_id, role, invited_by_user_id, invite_email, accepted_at, created_at
`, hash, userID).Scan(
		&o.ID, &o.PackageID, &o.UserID, &o.OrgID, &o.Role, &o.InvitedByUserID, &o.InviteEmail, &o.AcceptedAt, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &userID,
		PackageID:   &o.PackageID,
		EventType:   "ownership.accept",
		Summary:     "Accepted package ownership invite",
	})
	return &o, nil
}

// RemovePackageOwner removes a user or org owner row.
func (db *DB) RemovePackageOwner(ctx context.Context, packageID, ownerRowID, actorID int64) error {
	var role string
	var userID *int64
	err := db.pool.QueryRow(ctx, `
SELECT role, user_id FROM package_owners WHERE id = $1 AND package_id = $2
`, ownerRowID, packageID).Scan(&role, &userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if role == "owner" {
		var owners int64
		_ = db.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM package_owners WHERE package_id = $1 AND role = 'owner' AND accepted_at IS NOT NULL
`, packageID).Scan(&owners)
		if owners <= 1 {
			return fmt.Errorf("cannot remove the last package owner")
		}
	}
	tag, err := db.pool.Exec(ctx, `DELETE FROM package_owners WHERE id = $1 AND package_id = $2`, ownerRowID, packageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &actorID,
		PackageID:   &packageID,
		EventType:   "ownership.remove",
		Summary:     "Removed package owner/maintainer",
	})
	return nil
}

// CreateOwnershipTransfer starts a primary-owner transfer.
func (db *DB) CreateOwnershipTransfer(ctx context.Context, packageID, fromUserID int64, toUsername, toOrgSlug string) (*OwnershipTransfer, string, error) {
	var toUserID *int64
	var toOrgID *int64
	toUsername = strings.TrimSpace(toUsername)
	toOrgSlug = strings.TrimSpace(toOrgSlug)
	if toUsername != "" {
		u, err := db.GetUserByUsername(ctx, toUsername)
		if err != nil {
			return nil, "", err
		}
		toUserID = &u.ID
	} else if toOrgSlug != "" {
		o, err := db.GetOrganizationBySlug(ctx, toOrgSlug)
		if err != nil {
			return nil, "", err
		}
		toOrgID = &o.ID
	} else {
		return nil, "", fmt.Errorf("destination username or org required")
	}
	plaintext, hash, err := randomToken("nxfr_")
	if err != nil {
		return nil, "", err
	}
	var t OwnershipTransfer
	err = db.pool.QueryRow(ctx, `
INSERT INTO ownership_transfers (package_id, from_user_id, to_user_id, to_org_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, package_id, from_user_id, to_user_id, to_org_id, status, expires_at, created_at, completed_at
`, packageID, fromUserID, toUserID, toOrgID, hash, time.Now().UTC().Add(7*24*time.Hour)).Scan(
		&t.ID, &t.PackageID, &t.FromUserID, &t.ToUserID, &t.ToOrgID, &t.Status, &t.ExpiresAt, &t.CreatedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, "", err
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &fromUserID,
		PackageID:   &packageID,
		EventType:   "ownership.transfer_start",
		Summary:     "Started package ownership transfer",
	})
	return &t, plaintext, nil
}

// AcceptOwnershipTransfer completes a transfer using the invite token.
func (db *DB) AcceptOwnershipTransfer(ctx context.Context, plaintext string, acceptorID int64) error {
	hash := HashToken(plaintext)
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var t OwnershipTransfer
	err = tx.QueryRow(ctx, `
SELECT id, package_id, from_user_id, to_user_id, to_org_id, status, expires_at, created_at, completed_at
FROM ownership_transfers
WHERE token_hash = $1 AND status = 'pending' AND expires_at > NOW()
FOR UPDATE
`, hash).Scan(
		&t.ID, &t.PackageID, &t.FromUserID, &t.ToUserID, &t.ToOrgID, &t.Status, &t.ExpiresAt, &t.CreatedAt, &t.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if t.ToUserID != nil && *t.ToUserID != acceptorID {
		return fmt.Errorf("transfer is addressed to a different user")
	}
	if t.ToOrgID != nil {
		role, err := db.OrgMemberRole(ctx, *t.ToOrgID, acceptorID)
		if err != nil {
			return err
		}
		if role != "owner" && role != "admin" {
			return fmt.Errorf("must be an org owner or admin to accept transfer")
		}
	}

	if t.ToUserID != nil {
		_, err = tx.Exec(ctx, `UPDATE packages SET owner_id = $2, owner_org_id = NULL, updated_at = NOW() WHERE id = $1`, t.PackageID, *t.ToUserID)
		if err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `
INSERT INTO package_owners (package_id, user_id, role, accepted_at)
VALUES ($1, $2, 'owner', NOW())
ON CONFLICT DO NOTHING
`, t.PackageID, *t.ToUserID)
		_, _ = tx.Exec(ctx, `
INSERT INTO package_owners (package_id, user_id, role, accepted_at)
SELECT $1, $2, 'owner', NOW()
WHERE NOT EXISTS (SELECT 1 FROM package_owners WHERE package_id = $1 AND user_id = $2)
`, t.PackageID, *t.ToUserID)
		_, _ = tx.Exec(ctx, `
UPDATE package_owners SET role = 'owner' WHERE package_id = $1 AND user_id = $2
`, t.PackageID, *t.ToUserID)
	} else if t.ToOrgID != nil {
		_, err = tx.Exec(ctx, `UPDATE packages SET owner_org_id = $2, owner_id = NULL, updated_at = NOW() WHERE id = $1`, t.PackageID, *t.ToOrgID)
		if err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `
INSERT INTO package_owners (package_id, org_id, role, accepted_at)
SELECT $1, $2, 'owner', NOW()
WHERE NOT EXISTS (SELECT 1 FROM package_owners WHERE package_id = $1 AND org_id = $2)
`, t.PackageID, *t.ToOrgID)
	}

	_, err = tx.Exec(ctx, `
UPDATE ownership_transfers SET status = 'accepted', completed_at = NOW() WHERE id = $1
`, t.ID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = db.RecordActivity(ctx, &ActivityEvent{
		ActorUserID: &acceptorID,
		PackageID:   &t.PackageID,
		EventType:   "ownership.transfer",
		Summary:     "Accepted package ownership transfer",
	})
	return nil
}

// ListPackagesByOrg returns packages owned by or associated with an org.
func (db *DB) ListPackagesByOrg(ctx context.Context, orgID int64) ([]PackageSummary, error) {
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
WHERE p.owner_org_id = $1
   OR EXISTS (
        SELECT 1 FROM package_owners po
        WHERE po.package_id = p.id AND po.org_id = $1 AND po.accepted_at IS NOT NULL
   )
ORDER BY p.updated_at DESC
`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPackageSummaries(rows)
}

// ListPackagesForUser returns packages the user owns or maintains.
func (db *DB) ListPackagesForUser(ctx context.Context, userID int64) ([]PackageSummary, error) {
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
WHERE p.owner_id = $1
   OR EXISTS (
        SELECT 1 FROM package_owners po
        WHERE po.package_id = p.id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
   )
   OR EXISTS (
        SELECT 1 FROM package_owners po
        JOIN organization_members om ON om.org_id = po.org_id AND om.user_id = $1
        WHERE po.package_id = p.id AND po.accepted_at IS NOT NULL
   )
ORDER BY p.updated_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPackageSummaries(rows)
}

// RecordActivity inserts an activity feed event.
func (db *DB) RecordActivity(ctx context.Context, ev *ActivityEvent) error {
	if ev == nil {
		return nil
	}
	meta := ev.Meta
	if meta == "" {
		meta = "{}"
	}
	_, err := db.pool.Exec(ctx, `
INSERT INTO activity_events (actor_user_id, org_id, package_id, event_type, summary, meta)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
`, ev.ActorUserID, ev.OrgID, ev.PackageID, ev.EventType, ev.Summary, meta)
	return err
}

// ListUserActivity returns recent activity for a user (as actor or package owner).
func (db *DB) ListUserActivity(ctx context.Context, userID int64, limit int) ([]ActivityEvent, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := db.pool.Query(ctx, `
SELECT e.id, e.actor_user_id, e.org_id, e.package_id, e.event_type, e.summary, e.meta::text, e.created_at,
       COALESCE(u.username, ''), COALESCE(p.name, ''), COALESCE(o.slug, '')
FROM activity_events e
LEFT JOIN users u ON u.id = e.actor_user_id
LEFT JOIN packages p ON p.id = e.package_id
LEFT JOIN organizations o ON o.id = e.org_id
WHERE e.actor_user_id = $1
   OR EXISTS (
        SELECT 1 FROM package_owners po
        WHERE po.package_id = e.package_id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
   )
   OR EXISTS (SELECT 1 FROM packages pk WHERE pk.id = e.package_id AND pk.owner_id = $1)
ORDER BY e.created_at DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActivity(rows)
}

// ListOrgActivity returns recent activity for an organization.
func (db *DB) ListOrgActivity(ctx context.Context, orgID int64, limit int) ([]ActivityEvent, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := db.pool.Query(ctx, `
SELECT e.id, e.actor_user_id, e.org_id, e.package_id, e.event_type, e.summary, e.meta::text, e.created_at,
       COALESCE(u.username, ''), COALESCE(p.name, ''), COALESCE(o.slug, '')
FROM activity_events e
LEFT JOIN users u ON u.id = e.actor_user_id
LEFT JOIN packages p ON p.id = e.package_id
LEFT JOIN organizations o ON o.id = e.org_id
WHERE e.org_id = $1
   OR EXISTS (
        SELECT 1 FROM package_owners po
        WHERE po.package_id = e.package_id AND po.org_id = $1 AND po.accepted_at IS NOT NULL
   )
   OR EXISTS (SELECT 1 FROM packages pk WHERE pk.id = e.package_id AND pk.owner_org_id = $1)
ORDER BY e.created_at DESC
LIMIT $2
`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActivity(rows)
}

// ListPackageActivity returns recent activity for a package.
func (db *DB) ListPackageActivity(ctx context.Context, packageID int64, limit int) ([]ActivityEvent, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := db.pool.Query(ctx, `
SELECT e.id, e.actor_user_id, e.org_id, e.package_id, e.event_type, e.summary, e.meta::text, e.created_at,
       COALESCE(u.username, ''), COALESCE(p.name, ''), COALESCE(o.slug, '')
FROM activity_events e
LEFT JOIN users u ON u.id = e.actor_user_id
LEFT JOIN packages p ON p.id = e.package_id
LEFT JOIN organizations o ON o.id = e.org_id
WHERE e.package_id = $1
ORDER BY e.created_at DESC
LIMIT $2
`, packageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActivity(rows)
}

func scanActivity(rows pgx.Rows) ([]ActivityEvent, error) {
	out := make([]ActivityEvent, 0)
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(
			&e.ID, &e.ActorUserID, &e.OrgID, &e.PackageID, &e.EventType, &e.Summary, &e.Meta, &e.CreatedAt,
			&e.ActorUsername, &e.PackageName, &e.OrgSlug,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *DB) getPackageNameByID(ctx context.Context, id int64) (string, error) {
	var name string
	err := db.pool.QueryRow(ctx, `SELECT name FROM packages WHERE id = $1`, id).Scan(&name)
	return name, err
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// UserIsPackageOwnerRole checks if user has an accepted owner/maintainer role.
func (db *DB) UserIsPackageOwnerRole(ctx context.Context, packageID, userID int64) (string, error) {
	var role string
	err := db.pool.QueryRow(ctx, `
SELECT po.role FROM package_owners po
WHERE po.package_id = $1 AND po.user_id = $2 AND po.accepted_at IS NOT NULL
`, packageID, userID).Scan(&role)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	// Via org membership on package_owners
	err = db.pool.QueryRow(ctx, `
SELECT po.role FROM package_owners po
JOIN organization_members om ON om.org_id = po.org_id AND om.user_id = $2
WHERE po.package_id = $1 AND po.accepted_at IS NOT NULL
ORDER BY CASE po.role WHEN 'owner' THEN 0 ELSE 1 END
LIMIT 1
`, packageID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}
