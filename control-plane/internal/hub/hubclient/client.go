package hubclient

import (
	"context"
	"io"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Client is the HTTP Hub client. THIS FILE IS A STUB: task 33 replaces it
// file for file with the transport, and task 34 adds the methods. Until then
// every method answers ErrNotConfigured, so the package compiles and phases can
// wire New today. It lives in its own file, not hubclient.go, so that task 33
// (lane C) replaces a file of its own and never edits lane A's hubclient.go
// (plan, "the shared files"; task 04 departure).
type Client struct{}

// New is the HTTP client (task 33). httpClient nil means a client with no
// overall timeout; each attempt has its own 10 s deadline.
func New(baseURL, scope string, tokens TokenSource, httpClient *http.Client) *Client {
	return &Client{}
}

func (*Client) Me(context.Context) (hubapi.Me, error) { return hubapi.Me{}, ErrNotConfigured }
func (*Client) ProjectForRepo(context.Context, string) (hubapi.Project, bool, error) {
	return hubapi.Project{}, false, ErrNotConfigured
}
func (*Client) Project(context.Context, string) (hubapi.Project, error) {
	return hubapi.Project{}, ErrNotConfigured
}
func (*Client) CreateProject(context.Context, hubapi.CreateProjectRequest) (hubapi.Project, error) {
	return hubapi.Project{}, ErrNotConfigured
}
func (*Client) AddRemote(context.Context, string, hubapi.RemoteRequest) (hubapi.Remote, error) {
	return hubapi.Remote{}, ErrNotConfigured
}

func (*Client) Teams(context.Context) ([]hubapi.Team, error) { return nil, ErrNotConfigured }
func (*Client) CreateTeam(context.Context, hubapi.CreateTeamRequest) (hubapi.Team, error) {
	return hubapi.Team{}, ErrNotConfigured
}
func (*Client) Team(context.Context, string) (hubapi.TeamDetail, error) {
	return hubapi.TeamDetail{}, ErrNotConfigured
}
func (*Client) SetMember(context.Context, string, string, hubapi.Role) (hubapi.Member, error) {
	return hubapi.Member{}, ErrNotConfigured
}
func (*Client) RemoveMember(context.Context, string, string) error { return ErrNotConfigured }
func (*Client) Invites(context.Context, string) ([]hubapi.Invite, error) {
	return nil, ErrNotConfigured
}
func (*Client) CreateInvite(context.Context, string, hubapi.CreateInviteRequest) (hubapi.Invite, error) {
	return hubapi.Invite{}, ErrNotConfigured
}
func (*Client) RevokeInvite(context.Context, string, string) error { return ErrNotConfigured }

func (*Client) CreateBrief(context.Context, string, hubapi.CreateBriefRequest) (hubapi.BriefHead, error) {
	return hubapi.BriefHead{}, ErrNotConfigured
}
func (*Client) List(context.Context, string, hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) {
	return hubapi.Page[hubapi.BriefHead]{}, ErrNotConfigured
}
func (*Client) ForBranch(context.Context, string, string) (hubapi.ForBranch, error) {
	return hubapi.ForBranch{}, ErrNotConfigured
}
func (*Client) Brief(context.Context, string) (hubapi.BriefWithRevision, error) {
	return hubapi.BriefWithRevision{}, ErrNotConfigured
}
func (*Client) Rebind(context.Context, string, string) (hubapi.BriefHead, error) {
	return hubapi.BriefHead{}, ErrNotConfigured
}
func (*Client) Archive(context.Context, string) (hubapi.BriefHead, error) {
	return hubapi.BriefHead{}, ErrNotConfigured
}
func (*Client) Revisions(context.Context, string, hubapi.PageRequest) (hubapi.Page[hubapi.RevisionSummary], error) {
	return hubapi.Page[hubapi.RevisionSummary]{}, ErrNotConfigured
}
func (*Client) WriteRevisions(context.Context, string, []hubapi.RevisionWrite) ([]hubapi.BriefRevision, error) {
	return nil, ErrNotConfigured
}
func (*Client) Resolve(context.Context, []string) (hubapi.RefsResponse, error) {
	return hubapi.RefsResponse{}, ErrNotConfigured
}

func (*Client) PutBlob(context.Context, string, string, string, int64, io.ReadSeeker) (hubapi.Blob, error) {
	return hubapi.Blob{}, ErrNotConfigured
}
func (*Client) GetBlob(context.Context, string, string) (io.ReadCloser, hubapi.Blob, error) {
	return nil, hubapi.Blob{}, ErrNotConfigured
}
func (*Client) Approve(context.Context, string, hubapi.ApprovalRequest) (hubapi.DesignApproval, error) {
	return hubapi.DesignApproval{}, ErrNotConfigured
}
func (*Client) Approval(context.Context, string) (hubapi.DesignApproval, error) {
	return hubapi.DesignApproval{}, ErrNotConfigured
}
func (*Client) Approvals(context.Context, string, hubapi.ApprovalQuery) (hubapi.Page[hubapi.DesignApproval], error) {
	return hubapi.Page[hubapi.DesignApproval]{}, ErrNotConfigured
}
func (*Client) LinkSession(context.Context, string, hubapi.SessionLinkRequest) (hubapi.SessionLink, error) {
	return hubapi.SessionLink{}, ErrNotConfigured
}
func (*Client) SessionLinks(context.Context, string, hubapi.SessionLinkQuery) (hubapi.Page[hubapi.SessionLink], error) {
	return hubapi.Page[hubapi.SessionLink]{}, ErrNotConfigured
}

func (*Client) Reserve(context.Context, string, hubapi.ReserveRequest) (hubapi.Release, error) {
	return hubapi.Release{}, ErrNotConfigured
}
func (*Client) Confirm(context.Context, string, hubapi.ConfirmRequest) (hubapi.Release, error) {
	return hubapi.Release{}, ErrNotConfigured
}
func (*Client) Abandon(context.Context, string, hubapi.AbandonRequest) (hubapi.Release, error) {
	return hubapi.Release{}, ErrNotConfigured
}
func (*Client) JoinDeployment(context.Context, string, hubapi.DeploymentJoinRequest) (hubapi.DeploymentJoin, error) {
	return hubapi.DeploymentJoin{}, ErrNotConfigured
}
func (*Client) Release(context.Context, string) (hubapi.Release, error) {
	return hubapi.Release{}, ErrNotConfigured
}
func (*Client) Releases(context.Context, string, hubapi.ReleaseQuery) (hubapi.Page[hubapi.Release], error) {
	return hubapi.Page[hubapi.Release]{}, ErrNotConfigured
}

func (*Client) RecordEvidence(context.Context, hubapi.EvidenceRequest) (hubapi.EvidenceRecord, error) {
	return hubapi.EvidenceRecord{}, ErrNotConfigured
}
func (*Client) EvidenceByDigest(context.Context, string, []string) ([]hubapi.EvidenceRecord, error) {
	return nil, ErrNotConfigured
}

func (*Client) Changes(context.Context, string, int) (hubapi.ChangesPage, error) {
	return hubapi.ChangesPage{}, ErrNotConfigured
}
