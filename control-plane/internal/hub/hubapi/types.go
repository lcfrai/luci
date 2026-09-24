package hubapi

import "encoding/json"

// One struct per JSON object of contract §7, with §7's names exactly (§9.2).
//
// Responses (contract §7.1): a field is always present, and a field with no
// value is null (objects, optional strings) or [] (lists), never omitted. So no
// response field carries omitempty, optional values are pointers, and
// EnsureLists fills every nil slice and map before the Hub writes a body.
// Readyz.Failing is the one exception, because §12.4's ready body has none.
//
// Requests are decoded with httpx.DecodeJSON's rules (unknown fields refused,
// contract §1); optional fields are pointers, and the client omits what it does
// not send. Every value Digest in a request is required: encoding/json leaves an
// absent or null one at Digest{}, so the Hub calls RequireDigests on every body.

// ── enums ───────────────────────────────────────────────────────────────────

type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

type InviteState string

const (
	InvitePending InviteState = "pending"
	InviteBound   InviteState = "bound"
	InviteRevoked InviteState = "revoked"
)

type Resolution string

const (
	Resolved Resolution = "resolved"
	Missing  Resolution = "missing"
	Erased   Resolution = "erased"
	Invalid  Resolution = "invalid"
)

type ReleaseKind string

const (
	KindRun      ReleaseKind = "run"
	KindRollback ReleaseKind = "rollback"
)

type ReleaseState string

const (
	StateReserved  ReleaseState = "reserved"
	StateConfirmed ReleaseState = "confirmed"
	StateAbandoned ReleaseState = "abandoned"
	StateExpired   ReleaseState = "expired"
)

type PinStatus string

const (
	PinPinned PinStatus = "pinned"
	PinNone   PinStatus = "none"
)

type PinReason string

const (
	PinShipHead           PinReason = "ship_head"
	PinInherited          PinReason = "inherited"
	PinNoBrief            PinReason = "no_brief"
	PinNoCompleteRevision PinReason = "no_complete_revision"
	PinSourceUnpinned     PinReason = "source_unpinned"
	PinSourceUnknown      PinReason = "source_unknown"
)

type AbandonReason string

const (
	AbandonDeployFailed AbandonReason = "deploy_failed"
	AbandonCancelled    AbandonReason = "cancelled"
)

type Verdict string

const (
	VerdictSettled   Verdict = "settled"
	VerdictContested Verdict = "contested"
)

// ── shared objects (contract §7.1) ──────────────────────────────────────────

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type Org struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	TenantID    string `json:"tenantId"`
}

// Team: Role is the CALLER's role in it.
type Team struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	CreatedAt string `json:"createdAt"`
	CreatedBy User   `json:"createdBy"`
}

type Member struct {
	User    User   `json:"user"`
	Role    Role   `json:"role"`
	AddedAt string `json:"addedAt"`
	AddedBy *User  `json:"addedBy"`
}

type Invite struct {
	ID        string      `json:"id"`
	TeamID    string      `json:"teamId"`
	Email     string      `json:"email"`
	Role      Role        `json:"role"`
	State     InviteState `json:"state"`
	CreatedAt string      `json:"createdAt"`
	CreatedBy User        `json:"createdBy"`
	BoundUser *User       `json:"boundUser"`
}

type Remote struct {
	RepoKey string  `json:"repoKey"`
	Host    GitHost `json:"host"`
	AddedAt string  `json:"addedAt"`
}

type Project struct {
	ID        string   `json:"id"`
	TeamID    string   `json:"teamId"`
	Name      string   `json:"name"`
	Remotes   []Remote `json:"remotes"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy User     `json:"createdBy"`
}

// BriefHead: the Head* fields are null while HeadRevision is 0.
type BriefHead struct {
	ID                     string  `json:"id"`
	ProjectID              string  `json:"projectId"`
	Scope                  string  `json:"scope"`     // ScopeProject | ScopeBranch
	BranchRef              string  `json:"branchRef"` // "" for a project brief
	HeadRevision           int     `json:"headRevision"`
	HeadKey                *string `json:"headKey"`
	HeadDigest             *Digest `json:"headDigest"`
	HeadStatus             *string `json:"headStatus"`
	LatestCompleteRevision int     `json:"latestCompleteRevision"` // 0 when none is complete
	ArchivedAt             *string `json:"archivedAt"`
	CreatedAt              string  `json:"createdAt"`
	CreatedBy              User    `json:"createdBy"`
	UpdatedAt              string  `json:"updatedAt"`
}

// BriefRevision: immutable. Content is null exactly when ErasedAt is set.
type BriefRevision struct {
	Key       string          `json:"key"`
	BriefID   string          `json:"briefId"`
	Revision  int             `json:"revision"`
	Digest    Digest          `json:"digest"`
	ProjectID string          `json:"projectId"`
	Scope     string          `json:"scope"`
	BranchRef string          `json:"branchRef"`
	Status    string          `json:"status"`
	Content   json.RawMessage `json:"content"`
	SavedAt   string          `json:"savedAt"`
	SavedBy   User            `json:"savedBy"`
	ErasedAt  *string         `json:"erasedAt"`
}

// RevisionSummary is a history row: a BriefRevision without content (§7.5).
type RevisionSummary struct {
	Key      string  `json:"key"`
	Revision int     `json:"revision"`
	Digest   Digest  `json:"digest"`
	Status   string  `json:"status"`
	SavedAt  string  `json:"savedAt"`
	SavedBy  User    `json:"savedBy"`
	ErasedAt *string `json:"erasedAt"`
}

// ── lists ───────────────────────────────────────────────────────────────────

// Page is every list route's body (contract §1): {"items":[…],"next":…|null}.
type Page[T any] struct {
	Items []T     `json:"items"`
	Next  *string `json:"next"`
}

// PageRequest is ?after= and ?limit=; zero values mean "from the start" and 50.
type PageRequest struct {
	After string
	Limit int
}

// ── /v1/me and teams (§7.2, §7.3) ───────────────────────────────────────────

type Me struct {
	User         User   `json:"user"`
	Org          Org    `json:"org"`
	OrgAdmin     bool   `json:"orgAdmin"`
	Teams        []Team `json:"teams"`
	InvitesBound int    `json:"invitesBound"`
}

type CreateTeamRequest struct {
	Name string `json:"name"`
}

type RenameTeamRequest struct {
	Name string `json:"name"`
}

type TeamDetail struct {
	Team    Team     `json:"team"`
	Members []Member `json:"members"`
}

type SetMemberRequest struct {
	Role Role `json:"role"`
}

type CreateInviteRequest struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

// ── projects and remotes (§7.4) ─────────────────────────────────────────────

type RemoteRequest struct {
	RepoKey string  `json:"repoKey"`
	Host    GitHost `json:"host"`
}

type CreateProjectRequest struct {
	TeamID  string          `json:"teamId"`
	Name    string          `json:"name"`
	Remotes []RemoteRequest `json:"remotes"`
}

type RenameProjectRequest struct {
	Name string `json:"name"`
}

// ── briefs (§7.5) ───────────────────────────────────────────────────────────

type CreateBriefRequest struct {
	Scope     string `json:"scope"`
	BranchRef string `json:"branchRef,omitempty"` // absent or "" for a project brief
}

type BriefWithRevision struct {
	Head     BriefHead      `json:"head"`
	Revision *BriefRevision `json:"revision"` // the head revision; null while HeadRevision is 0
	// LatestComplete is the latest complete revision (HA4-4, task 00): null exactly
	// when Head.LatestCompleteRevision is 0; the same revision as Revision when the
	// head is complete. Routes 21 and 22 both carry it.
	LatestComplete *BriefRevision `json:"latestComplete"`
}

// BriefQuery is route 19's query (HA4-2, task 00). Page is the shape every other
// query type uses; phase 4's ask spelt it as flat After/Limit fields.
type BriefQuery struct {
	Scope           string // "" = both; "project" or "branch"
	BranchRef       string // "" = any; else a valid ref (§3.3)
	IncludeArchived bool
	Page            PageRequest
}

// ForBranch is GET …/briefs/for-branch. Never a 404 for an absent brief.
type ForBranch struct {
	BranchBrief  *BriefWithRevision `json:"branchBrief"`
	ProjectBrief *BriefWithRevision `json:"projectBrief"`
}

type RebindRequest struct {
	BranchRef string `json:"branchRef"`
}

type ArchiveRequest struct{}

type RevisionWrite struct {
	BriefID      string          `json:"briefId"`
	BaseRevision int             `json:"baseRevision"`
	Content      json.RawMessage `json:"content"`
}

// MaxRevisionBatch and MaxRevisionBatchBytes bound POST …/brief-revisions
// (HA4-3, task 00): 64 items, and a 4 MiB body — 64 × the 64 KiB envelope cap.
// The body cap counts wire bytes; it holds for 64 maximum-size items only
// because hubclient sends each content canonical and encodes without HTML
// escaping (task 00, the wire rule).
const (
	MaxRevisionBatch      = 64
	MaxRevisionBatchBytes = 4 << 20
)

type WriteRevisionsRequest struct {
	Revisions []RevisionWrite `json:"revisions"` // 1–MaxRevisionBatch, distinct briefIds
}

// WriteRevisionsResponse carries every revision's Content. The copy stored for
// replay has every Content null, and a replay rebuilds it from brief_revisions
// (A17, tasks 14 and 19), so no stored response holds brief text.
type WriteRevisionsResponse struct {
	Revisions []BriefRevision `json:"revisions"` // in request order
}

// ── the resolver (§7.5) ─────────────────────────────────────────────────────

type RefResult struct {
	Key        string          `json:"key"`
	Kind       *KeyKind        `json:"kind"` // null exactly when Resolution is Invalid
	Resolution Resolution      `json:"resolution"`
	Revision   *BriefRevision  `json:"revision"`
	Approval   *DesignApproval `json:"approval"`
}

type HeadInfo struct {
	HeadRevision           int  `json:"headRevision"`
	LatestCompleteRevision int  `json:"latestCompleteRevision"`
	Archived               bool `json:"archived"`
}

type RefsResponse struct {
	Results []RefResult         `json:"results"` // one per requested key, in request order
	Heads   map[string]HeadInfo `json:"heads"`   // by brief id; {} when none
}

// ── blobs (§7.6) ────────────────────────────────────────────────────────────

type Blob struct {
	Digest      Digest `json:"digest"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

// BlobContentTypes is the upload allowlist, exactly (§7.6).
var BlobContentTypes = []string{
	"text/html; charset=utf-8", "text/css; charset=utf-8", "text/plain; charset=utf-8",
	"application/json", "image/png", "image/jpeg", "image/webp", "image/svg+xml",
	"font/woff2", "application/octet-stream",
}

const (
	MaxBlobBytes        = 16 << 20 // 16,777,216; PANACEA_APP_HUB_BLOB_MAX_BYTES may lower it
	DefaultProjectQuota = 1 << 30  // 1 GiB
)

// ── design approvals and session links (§7.7, §7.8) ─────────────────────────

type SnapshotRequest struct {
	Manifest Digest   `json:"manifest"`
	Blobs    []Digest `json:"blobs"` // 1–500, distinct, includes Manifest
}

type ApprovalRequest struct {
	DesignKey    string          `json:"designKey"`
	ReviewDigest Digest          `json:"reviewDigest"`
	BriefKey     string          `json:"briefKey"`
	BriefDigest  Digest          `json:"briefDigest"`
	BranchRef    string          `json:"branchRef"`
	Snapshot     SnapshotRequest `json:"snapshot"`
}

type Snapshot struct {
	Manifest Digest `json:"manifest"`
	Blobs    []Blob `json:"blobs"`
}

type DesignApproval struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"projectId"`
	DesignKey      string   `json:"designKey"`
	CanvasID       string   `json:"canvasId"`
	DesignRevision int      `json:"designRevision"`
	ReviewDigest   Digest   `json:"reviewDigest"`
	BriefKey       string   `json:"briefKey"`
	BriefDigest    Digest   `json:"briefDigest"`
	BranchRef      string   `json:"branchRef"`
	Snapshot       Snapshot `json:"snapshot"`
	ApprovedAt     string   `json:"approvedAt"`
	ApprovedBy     User     `json:"approvedBy"`
}

type ApprovalQuery struct {
	CanvasID  string // "" = any
	BranchRef string // "" = any
	Page      PageRequest
}

type SessionLinkRequest struct {
	DesignKey string `json:"designKey"`
	BriefKey  string `json:"briefKey"`
	SessionID string `json:"sessionId"`
	BranchRef string `json:"branchRef"`
}

type SessionLink struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	DesignKey string `json:"designKey"`
	BriefKey  string `json:"briefKey"`
	SessionID string `json:"sessionId"`
	BranchRef string `json:"branchRef"`
	CreatedAt string `json:"createdAt"`
	CreatedBy User   `json:"createdBy"`
}

type SessionLinkQuery struct {
	DesignKey string
	SessionID string
	Page      PageRequest
}

// ── releases (§7.9) ─────────────────────────────────────────────────────────

// ReserveRequest is both shapes of §7.9. A run sends Kind, PanaceaProject,
// Environment, BranchRef, Commit (unless SourceDeploymentID), and optionally
// BriefID, DesignKey, SourceDeploymentID. A rollback sends Kind, PanaceaProject,
// Environment and SourceDeploymentID only; any other field is 400 bad_request.
// Environment may be null only for a run without SourceDeploymentID (HA7-1,
// task 00): Panacea names a never-deployed branch's environment at deploy time.
type ReserveRequest struct {
	Kind               ReleaseKind `json:"kind"`
	PanaceaProject     string      `json:"panaceaProject"`
	Environment        *string     `json:"environment"`
	BranchRef          *string     `json:"branchRef,omitempty"`
	Commit             *string     `json:"commit,omitempty"`
	BriefID            *string     `json:"briefId,omitempty"`
	DesignKey          *string     `json:"designKey,omitempty"`
	SourceDeploymentID *string     `json:"sourceDeploymentId,omitempty"`
}

type Pin struct {
	Status             PinStatus `json:"status"`
	Reason             PinReason `json:"reason"`
	BriefKey           *string   `json:"briefKey"`
	BriefDigest        *Digest   `json:"briefDigest"`
	HeadRevisionAtShip *int      `json:"headRevisionAtShip"`
}

type DeploymentJoin struct {
	DeploymentID string `json:"deploymentId"`
	JoinedAt     string `json:"joinedAt"`
}

type Release struct {
	ID                 string           `json:"id"`
	ProjectID          string           `json:"projectId"`
	Kind               ReleaseKind      `json:"kind"`
	State              ReleaseState     `json:"state"`
	PanaceaProject     string           `json:"panaceaProject"`
	Environment        *string          `json:"environment"` // null until a join names it (HA7-1)
	BranchRef          string           `json:"branchRef"`   // "" when none
	Commit             *string          `json:"commit"`
	PipelineRun        *string          `json:"pipelineRun"`
	SourceDeploymentID *string          `json:"sourceDeploymentId"`
	SourceReleaseID    *string          `json:"sourceReleaseId"`
	Pin                Pin              `json:"pin"`
	DesignKey          *string          `json:"designKey"`
	Deployments        []DeploymentJoin `json:"deployments"`
	ReservedAt         string           `json:"reservedAt"`
	ReservedBy         User             `json:"reservedBy"`
	ExpiresAt          string           `json:"expiresAt"`
	ConfirmedAt        *string          `json:"confirmedAt"`
	AbandonedAt        *string          `json:"abandonedAt"`
	AbandonReason      *AbandonReason   `json:"abandonReason"`
	AbandonMessage     *string          `json:"abandonMessage"`
}

// ConfirmRequest: a run sends PipelineRun; a rollback sends {} or DeploymentID.
type ConfirmRequest struct {
	PipelineRun  *string `json:"pipelineRun,omitempty"`
	DeploymentID *string `json:"deploymentId,omitempty"`
}

type AbandonRequest struct {
	Reason  AbandonReason `json:"reason"`
	Message string        `json:"message"` // 0–500 characters
}

type DeploymentJoinRequest struct {
	DeploymentID string `json:"deploymentId"`
	// Environment sets Release.Environment when it is null; equal to a non-null
	// one is a no-op; different from a non-null one is 409 conflict (HA7-1).
	Environment *string `json:"environment,omitempty"`
}

type ReleaseQuery struct {
	Environment, PanaceaProject, DeploymentID, PipelineRun string       // "" = any
	Kind                                                   ReleaseKind  // "" = both (HA7-5)
	State                                                  ReleaseState // "" = any
	Page                                                   PageRequest
}

// ── evidence (§7.10) ────────────────────────────────────────────────────────

type EvidenceRequest struct {
	Repo            string  `json:"repo"`
	BriefKey        string  `json:"briefKey"`
	CriterionID     string  `json:"criterionId"`
	CriterionDigest Digest  `json:"criterionDigest"`
	Commit          string  `json:"commit"`
	Verdict         Verdict `json:"verdict"`
	Because         string  `json:"because"`
}

type EvidenceRecord struct {
	ID              string  `json:"id"`
	Repo            string  `json:"repo"`
	ProjectID       string  `json:"projectId"`
	BriefKey        string  `json:"briefKey"`
	CriterionID     string  `json:"criterionId"`
	CriterionDigest Digest  `json:"criterionDigest"`
	Commit          string  `json:"commit"`
	Verdict         Verdict `json:"verdict"`
	Because         string  `json:"because"`
	RecordedAt      string  `json:"recordedAt"`
	RecordedBy      User    `json:"recordedBy"`
}

// ── the change feed and the audit (§7.11, §7.12, §8) ────────────────────────

type Event struct {
	Seq       int64          `json:"seq"`
	At        string         `json:"at"`
	Kind      string         `json:"kind"`
	TeamID    *string        `json:"teamId"`
	ProjectID *string        `json:"projectId"`
	Subject   string         `json:"subject"`
	Actor     *User          `json:"actor"`
	Data      map[string]any `json:"data"` // {} when empty
}

type ChangesPage struct {
	Events []Event `json:"events"`
	Next   string  `json:"next"` // the cursor to send next, a decimal seq
	More   bool    `json:"more"`
}

type AuditEvent struct {
	Event
	Hash     string `json:"hash"`     // 64 lowercase hex
	PrevHash string `json:"prevHash"` // 64 lowercase hex
}

type AuditPage struct {
	Events []AuditEvent `json:"events"`
	Next   string       `json:"next"`
	More   bool         `json:"more"`
}

type EraseRequest struct {
	Reason string `json:"reason"` // 1–500 characters
}

type EraseResponse struct {
	BriefID         string `json:"briefId"`
	ErasedRevisions int    `json:"erasedRevisions"`
	ErasedAt        string `json:"erasedAt"`
}

// ── health (§12.4) ──────────────────────────────────────────────────────────

type Healthz struct {
	Status  string `json:"status"` // "ok"
	Version string `json:"version"`
}

type Readyz struct {
	Status  string   `json:"status"`            // "ready" | "not_ready"
	Failing []string `json:"failing,omitempty"` // "database" | "schema" | "jwks" | "shutting_down"
}
