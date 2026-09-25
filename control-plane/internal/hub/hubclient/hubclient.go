// Package hubclient is how the control plane calls the Hub, as the signed-in
// user (ADR-H1 §5.4). Handlers depend on the smallest interface they need;
// New returns the HTTP implementation (task 33), NewFake the in-memory one.
//
// Imports: hubapi, internal/httpx, internal/ids and the standard library only
// (plan correction A9; the boundary test in task 09 holds it).
package hubclient

import (
	"context"
	"errors"
	"io"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// TokenSource is satisfied by *entraauth.Authenticator (task 31).
type TokenSource interface {
	Token(ctx context.Context, scope string) (string, error)
}

// Invalidator is optionally implemented by a TokenSource (entraauth does, task 31).
// The client (task 33) calls it before its one retry on 401 unauthenticated,
// reason "expired". Declared here, in lane A, so that no lane-C task edits this
// file (review M15; it was task 31's one-declaration edit).
type Invalidator interface {
	Invalidate(scope string)
}

type Projects interface {
	Me(ctx context.Context) (hubapi.Me, error)
	// ProjectForRepo answers the one project in the caller's org whose remote is
	// repoKey and which the caller can read; ok is false when there is none.
	ProjectForRepo(ctx context.Context, repoKey string) (p hubapi.Project, ok bool, err error)
	Project(ctx context.Context, projectID string) (hubapi.Project, error)
	CreateProject(ctx context.Context, req hubapi.CreateProjectRequest) (hubapi.Project, error)
	AddRemote(ctx context.Context, projectID string, req hubapi.RemoteRequest) (hubapi.Remote, error)
}

type Teams interface {
	Teams(ctx context.Context) ([]hubapi.Team, error)
	CreateTeam(ctx context.Context, req hubapi.CreateTeamRequest) (hubapi.Team, error)
	Team(ctx context.Context, teamID string) (hubapi.TeamDetail, error)
	SetMember(ctx context.Context, teamID, userID string, role hubapi.Role) (hubapi.Member, error)
	RemoveMember(ctx context.Context, teamID, userID string) error
	Invites(ctx context.Context, teamID string) ([]hubapi.Invite, error)
	CreateInvite(ctx context.Context, teamID string, req hubapi.CreateInviteRequest) (hubapi.Invite, error)
	RevokeInvite(ctx context.Context, teamID, inviteID string) error
}

type Briefs interface { // phase 4
	CreateBrief(ctx context.Context, projectID string, req hubapi.CreateBriefRequest) (hubapi.BriefHead, error)
	// List is route 19 (HA4-2, task 00): oldest first; archived heads only when asked.
	List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error)
	ForBranch(ctx context.Context, projectID, branchRef string) (hubapi.ForBranch, error)
	Brief(ctx context.Context, briefID string) (hubapi.BriefWithRevision, error)
	Rebind(ctx context.Context, briefID, branchRef string) (hubapi.BriefHead, error)
	Archive(ctx context.Context, briefID string) (hubapi.BriefHead, error)
	Revisions(ctx context.Context, briefID string, page hubapi.PageRequest) (hubapi.Page[hubapi.RevisionSummary], error)
	WriteRevisions(ctx context.Context, projectID string, writes []hubapi.RevisionWrite) ([]hubapi.BriefRevision, error)
	// Resolve takes brief: and design: keys, any number; the client batches by 100.
	Resolve(ctx context.Context, keys []string) (hubapi.RefsResponse, error)
}

type Designs interface { // phase 5
	// digest is the wire form, "sha256:" + 64 hex, exactly as contract §9.2 declares it.
	// body is an io.ReadSeeker (HA5-1, task 00): the transport seeks it to 0 before
	// every attempt, so a retried upload never sends a consumed or short body.
	PutBlob(ctx context.Context, projectID string, digest string, contentType string, size int64, body io.ReadSeeker) (hubapi.Blob, error)
	GetBlob(ctx context.Context, projectID string, digest string) (io.ReadCloser, hubapi.Blob, error)
	Approve(ctx context.Context, projectID string, req hubapi.ApprovalRequest) (hubapi.DesignApproval, error)
	Approval(ctx context.Context, approvalID string) (hubapi.DesignApproval, error)
	Approvals(ctx context.Context, projectID string, q hubapi.ApprovalQuery) (hubapi.Page[hubapi.DesignApproval], error)
	LinkSession(ctx context.Context, projectID string, req hubapi.SessionLinkRequest) (hubapi.SessionLink, error)
	SessionLinks(ctx context.Context, projectID string, q hubapi.SessionLinkQuery) (hubapi.Page[hubapi.SessionLink], error)
}

type Releases interface { // phase 7
	Reserve(ctx context.Context, projectID string, req hubapi.ReserveRequest) (hubapi.Release, error)
	Confirm(ctx context.Context, releaseID string, req hubapi.ConfirmRequest) (hubapi.Release, error)
	Abandon(ctx context.Context, releaseID string, req hubapi.AbandonRequest) (hubapi.Release, error)
	// JoinDeployment takes the request, not a bare id: the join may name the
	// environment a null-environment run was deployed to (HA7-1, task 00).
	JoinDeployment(ctx context.Context, releaseID string, req hubapi.DeploymentJoinRequest) (hubapi.DeploymentJoin, error)
	Release(ctx context.Context, releaseID string) (hubapi.Release, error)
	Releases(ctx context.Context, projectID string, q hubapi.ReleaseQuery) (hubapi.Page[hubapi.Release], error)
}

type Evidence interface { // phase 3, H-2
	RecordEvidence(ctx context.Context, req hubapi.EvidenceRequest) (hubapi.EvidenceRecord, error)
	// digests are wire-form strings, 1–100 per call; the client batches more.
	EvidenceByDigest(ctx context.Context, repo string, digests []string) ([]hubapi.EvidenceRecord, error)
}

type Feed interface {
	// Changes reads the change feed. after is a decimal seq, or "latest".
	Changes(ctx context.Context, after string, limit int) (hubapi.ChangesPage, error)
}

type Hub interface {
	Projects
	Teams
	Briefs
	Designs
	Releases
	Evidence
	Feed
}

// Error is every problem the Hub answered, carried unchanged for the relay
// rule (contract §4.3). Status is the HTTP status; Problem is the body.
type Error struct {
	Status  int
	Problem httpx.Problem
	// RetryAfter is the Hub's Retry-After on a 429, else "" — so WriteError and a
	// caller of Relay (task 33, HA4-5) can copy it.
	RetryAfter string
}

func (e *Error) Error() string { return "hub: " + e.Problem.Error + ": " + e.Problem.Message }

// Code returns the Hub's problem code when err is (or wraps) an *Error, else "".
func Code(err error) string {
	var he *Error
	if errors.As(err, &he) {
		return he.Problem.Error
	}
	return ""
}

var (
	// ErrUnavailable wraps a transport failure, a timeout, or a 5xx. The relay
	// maps it to 503 upstream_unavailable, details.upstream = "hub".
	ErrUnavailable = errors.New("hubclient: hub unavailable")
	// ErrSignedOut: nobody is signed in, or the refresh token was refused.
	// entraauth.ErrSignedOut is this same value (task 31).
	ErrSignedOut = errors.New("hubclient: signed out")
	// ErrNotConfigured: PANACEA_APP_HUB_URL is unset. The relay maps it to 503
	// hub_not_configured (contract §12.2).
	ErrNotConfigured = errors.New("hubclient: the hub is not configured")
)

// New (the HTTP client) is declared in client.go, which task 33 replaces file
// for file; NewFake (the in-memory Hub, this task and task 05) in fake.go.
var (
	_ Hub = (*Client)(nil)
	_ Hub = (*Fake)(nil)
)
