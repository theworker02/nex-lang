package database

import "time"

// User is a registry account that can own and publish packages.
type User struct {
	ID               int64     `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	PasswordHash     string    `json:"-"`
	AvatarURL        string    `json:"avatar_url"`
	Bio              string    `json:"bio"`
	UseGravatar      bool      `json:"use_gravatar"`
	GitHubID          *int64     `json:"github_id,omitempty"`
	GitHubLogin       string     `json:"github_login,omitempty"`
	EmailVerified     bool       `json:"email_verified"`
	EmailVerifiedAt   *time.Time `json:"email_verified_at,omitempty"`
	TOTPSecret        string     `json:"-"`
	TOTPEnabled       bool       `json:"totp_enabled"`
	TOTPPendingSecret string     `json:"-"`
	IsAdmin           bool       `json:"is_admin"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Session is a server-side auth session (cookie or Bearer token).
type Session struct {
	ID        int64     `json:"-"`
	UserID    int64     `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey scopes for long-lived credentials.
const (
	APIKeyScopePublish = "publish"
	APIKeyScopeRead    = "read"
	APIKeyScopeFull    = "full"
)

// APIKey is a scoped credential; only the hash is stored after creation.
type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	KeyHash    string     `json:"-"`
	Scope      string     `json:"scope"` // publish | read | full
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// TrustedPublisher statuses.
const (
	TrustedStatusPending  = "pending"
	TrustedStatusVerified = "verified"
)

// TrustedPublisher is a CI identity allowed to publish for a user/package.
type TrustedPublisher struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	Provider          string     `json:"provider"`
	RepositoryOwner   string     `json:"repository_owner"`
	RepositoryName    string     `json:"repository_name"`
	WorkflowFilename  string     `json:"workflow_filename"`
	Environment       string     `json:"environment"`
	PackageScope      string     `json:"package_scope"` // empty = all packages owned by user
	Status            string     `json:"status"`        // pending | verified
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	LastFailureReason string     `json:"last_failure_reason,omitempty"`
	LastFailureAt     *time.Time `json:"last_failure_at,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Package is a published Nexus package registry entry.
type Package struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Author        string    `json:"author"`
	License       string    `json:"license"`
	Repository    string    `json:"repository"`
	Homepage      string    `json:"homepage"`
	Keywords      []string  `json:"keywords"`
	Categories    []string  `json:"categories"`
	Readme        string    `json:"readme,omitempty"`
	DownloadCount int64     `json:"download_count"`
	OwnerID       *int64    `json:"owner_id,omitempty"`
	OwnerOrgID    *int64    `json:"owner_org_id,omitempty"`
	OwnerUsername string    `json:"owner_username,omitempty"`
	OwnerOrgSlug  string    `json:"owner_org_slug,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Version is a concrete immutable release of a package (.nex artifact).
type Version struct {
	ID                 int64      `json:"id"`
	PackageID          int64      `json:"package_id"`
	Version            string     `json:"version"`
	Checksum           string     `json:"checksum"`
	StoragePath        string     `json:"-"`
	Filename           string     `json:"filename"`
	FileSize           int64      `json:"file_size"`
	ContentType        string     `json:"content_type"`
	Yanked             bool       `json:"yanked"`
	YankReason         string     `json:"yank_reason"`
	YankedAt           *time.Time `json:"yanked_at,omitempty"`
	YankedByUserID     *int64     `json:"yanked_by_user_id,omitempty"`
	Deprecated         bool       `json:"deprecated"`
	DeprecationMessage string     `json:"deprecation_message"`
	PublishedByUserID  *int64     `json:"published_by_user_id,omitempty"`
	TrustedPublisherID *int64     `json:"trusted_publisher_id,omitempty"`
	PublishedVia       string     `json:"published_via"`
	ProvenanceJSON     []byte     `json:"provenance,omitempty"`
	ProvenanceSource   string     `json:"provenance_source,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// AuditLog records security-relevant registry events.
type AuditLog struct {
	ID           int64     `json:"id"`
	ActorUserID  *int64    `json:"actor_user_id,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	PackageName  string    `json:"package_name"`
	Version      string    `json:"version"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	Metadata     []byte    `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AbuseReport is a user-submitted trust & safety report.
type AbuseReport struct {
	ID             int64      `json:"id"`
	ReporterUserID *int64     `json:"reporter_user_id,omitempty"`
	ReporterEmail  string     `json:"reporter_email"`
	PackageName    string     `json:"package_name"`
	Version        string     `json:"version"`
	Category       string     `json:"category"`
	Details        string     `json:"details"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// AuditEvent is the input for writing an audit log row.
type AuditEvent struct {
	ActorUserID  int64
	Action       string
	ResourceType string
	ResourceID   string
	PackageName  string
	Version      string
	IP           string
	UserAgent    string
	Metadata     map[string]any
}

// Dependency is a package dependency declared for a specific version.
type Dependency struct {
	ID             int64  `json:"id"`
	VersionID      int64  `json:"version_id"`
	DependencyName string `json:"name"`
	Name           string `json:"-"` // alias populated after scan for templates/API convenience
	VersionReq     string `json:"version_req"`
	Optional       bool   `json:"optional"`
	Dev            bool   `json:"dev"`
}

// ReverseDependency is a package/version that depends on a given package name.
type ReverseDependency struct {
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	VersionReq  string `json:"version_req"`
	Optional    bool   `json:"optional"`
	Dev         bool   `json:"dev"`
}

// DailyDownload is a per-day download counter for charts.
type DailyDownload struct {
	Day   time.Time `json:"day"`
	Count int64     `json:"count"`
	Pct   int       `json:"pct,omitempty"` // 0–100 relative height for simple charts
}

// PackageVersion joins package metadata with a specific version release.
type PackageVersion struct {
	Package      Package      `json:"package"`
	Version      Version      `json:"version"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

// PackageSummary is a search/browse row with the latest version attached.
type PackageSummary struct {
	Package
	LatestVersion string `json:"latest_version"`
}

// SearchParams controls registry package discovery (filters + ranking).
type SearchParams struct {
	Query        string    `json:"q"`
	Category     string    `json:"category"`
	Keyword      string    `json:"keyword"`
	License      string    `json:"license"`
	UpdatedAfter time.Time `json:"updated_after,omitempty"`
	Sort         string    `json:"sort"` // relevance | downloads | recent
	Limit        int       `json:"limit"`
	Offset       int       `json:"offset"`
	Browse       bool      `json:"browse"` // when true, empty filters still return a sorted index
}

// SearchResult is a ranked/filtered package page.
type SearchResult struct {
	Packages []PackageSummary `json:"packages"`
	Total    int64            `json:"total"`
	Params   SearchParams     `json:"params"`
}

// UserStats summarizes a publisher's registry activity.
type UserStats struct {
	PackageCount   int64 `json:"package_count"`
	VersionCount   int64 `json:"version_count"`
	TotalDownloads int64 `json:"total_downloads"`
	APIKeyCount    int64 `json:"api_key_count"`
	TrustedCount   int64 `json:"trusted_publisher_count"`
}

// Page holds pagination metadata for list endpoints.
type Page struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
	HasNext bool  `json:"has_next"`
	HasPrev bool  `json:"has_prev"`
}

// TagCount is a category or keyword with how many packages use it.
type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// HubStats is the npm-style aggregate strip shown on the registry home page.
type HubStats struct {
	Packages  int64      `json:"packages"`
	Versions  int64      `json:"versions"`
	Downloads int64      `json:"downloads"`
	Users     int64      `json:"users"`
	Tags      []TagCount `json:"tags"`
}

// PublishInput carries the metadata required to register a package version.
type PublishInput struct {
	Name               string
	Description        string
	Author             string
	License            string
	Repository         string
	Homepage           string
	Keywords           []string
	Categories         []string
	Readme             string
	Version            string
	Checksum           string
	StoragePath        string
	Filename           string
	FileSize           int64
	ContentType        string
	OwnerID            int64
	PublishedByUserID  int64
	TrustedPublisherID *int64
	PublishedVia       string
	Dependencies       []DependencyInput
}

// DependencyInput is a dependency declared at publish time.
type DependencyInput struct {
	Name       string
	VersionReq string
	Optional   bool
	Dev        bool
}

// Organization is a multi-user publishing namespace.
type Organization struct {
	ID              int64     `json:"id"`
	Slug            string    `json:"slug"`
	DisplayName     string    `json:"display_name"`
	Description     string    `json:"description"`
	AvatarURL       string    `json:"avatar_url"`
	CreatedByUserID int64     `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	MemberCount     int64     `json:"member_count,omitempty"`
	PackageCount    int64     `json:"package_count,omitempty"`
}

// OrgMember is a user's membership in an organization.
type OrgMember struct {
	ID        int64     `json:"id"`
	OrgID     int64     `json:"org_id"`
	UserID    int64     `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	Username  string    `json:"username,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
}

// Team is a named group within an organization.
type Team struct {
	ID          int64     `json:"id"`
	OrgID       int64     `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	MemberCount int64     `json:"member_count,omitempty"`
}

// TeamMember is a user's membership in a team.
type TeamMember struct {
	ID        int64     `json:"id"`
	TeamID    int64     `json:"team_id"`
	UserID    int64     `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	Username  string    `json:"username,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
}

// PackageOwner is a user or org collaborator on a package.
type PackageOwner struct {
	ID              int64      `json:"id"`
	PackageID       int64      `json:"package_id"`
	UserID          *int64     `json:"user_id,omitempty"`
	OrgID           *int64     `json:"org_id,omitempty"`
	Role            string     `json:"role"`
	InvitedByUserID *int64     `json:"invited_by_user_id,omitempty"`
	InviteEmail     string     `json:"invite_email,omitempty"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	Username        string     `json:"username,omitempty"`
	OrgSlug         string     `json:"org_slug,omitempty"`
	OrgDisplayName  string     `json:"org_display_name,omitempty"`
	Pending         bool       `json:"pending,omitempty"`
}

// OwnershipTransfer is an ownership transfer request.
type OwnershipTransfer struct {
	ID          int64      `json:"id"`
	PackageID   int64      `json:"package_id"`
	FromUserID  int64      `json:"from_user_id"`
	ToUserID    *int64     `json:"to_user_id,omitempty"`
	ToOrgID     *int64     `json:"to_org_id,omitempty"`
	Status      string     `json:"status"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ActivityEvent is an audit/activity feed row.
type ActivityEvent struct {
	ID            int64     `json:"id"`
	ActorUserID   *int64    `json:"actor_user_id,omitempty"`
	OrgID         *int64    `json:"org_id,omitempty"`
	PackageID     *int64    `json:"package_id,omitempty"`
	EventType     string    `json:"event_type"`
	Summary       string    `json:"summary"`
	Meta          string    `json:"meta,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ActorUsername string    `json:"actor_username,omitempty"`
	PackageName   string    `json:"package_name,omitempty"`
	OrgSlug       string    `json:"org_slug,omitempty"`
}

// Auth token purposes for auth_tokens.purpose.
const (
	AuthTokenEmailVerify   = "email_verify"
	AuthTokenPasswordReset = "password_reset"
)
