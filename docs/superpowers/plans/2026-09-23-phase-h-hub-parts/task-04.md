### Task 04: `hubclient` — the consumer interfaces, `Error`, and the in-memory fake, part 1

**Lane A. Depends on: 02, 03. About 3 hours. Contract task H0.**

Phases 3, 4, 5 and 7 code their Hub-backed handlers **tonight**, before any Hub route exists
(contract §9.2). They code against these interfaces and this fake. So the interfaces must be the
ones the HTTP client (tasks 33–34) will implement, and the fake must obey every rule of contract §7
a caller can observe — or each phase's tests pass against a Hub that does not exist.

This task writes every interface, the error model, and the fake's **identity, team, project, brief
and resolver** behaviour. Task 05 adds designs, releases, evidence and the feed, and the
conformance suite that holds the fake and the real Hub to one standard.

**Additions to contract §9.2, all additive, apart from HA5-1's and HA7-1's two signature changes**
(task 00's amendments, which task 44a folds into §9.2: `PutBlob`'s body is an `io.ReadSeeker`, and
`JoinDeployment` takes a `DeploymentJoinRequest`). No other method the contract names changes, not
even a parameter type — digests stay wire-form strings in `PutBlob`, `GetBlob` and
`EvidenceByDigest`, because phases 3 and 5 are coding to §9.2's text tonight. The additions:
`Teams` (task 35's share flow needs it; the routes exist, §7.3), and `Projects.Project` and
`Projects.AddRemote`, `Designs.Approval` and `Designs.SessionLinks`, `Releases.Release` — each a
route the contract already defines (§7.4, §7.7, §7.8, §7.9) with no client method. `ErrSignedOut`
is declared **here**, and task 31's `entraauth.ErrSignedOut` is the same value, so `errors.Is`
works across both without `hubclient` importing `entraauth`.

**Files:**
- `control-plane/internal/hub/hubclient/hubclient.go` + `hubclient_test.go` (new)
- `control-plane/internal/hub/hubclient/client.go` — the `Client` stub and `New`, which task 33
  replaces file for file (new; *As built*, D1)
- `control-plane/internal/hub/hubclient/fake.go` — the `Fake` type, `NewFake`, principals, clock, switches (new)
- `control-plane/internal/hub/hubclient/fake_projects.go` — teams, invites, projects (new)
- `control-plane/internal/hub/hubclient/fake_briefs.go` — briefs, revisions, `Resolve` (brief keys) (new)
- `control-plane/internal/hub/hubclient/fake_test.go` (new)

#### Contract — complete

```go
// control-plane/internal/hub/hubclient/hubclient.go

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
	"net/http"

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

// New is the HTTP client (task 33). httpClient nil means a client with no
// overall timeout; each attempt has its own 10 s deadline.
func New(baseURL, scope string, tokens TokenSource, httpClient *http.Client) *Client

// NewFake is the in-memory Hub (this task and task 05).
func NewFake() *Fake

var (
	_ Hub = (*Client)(nil)
	_ Hub = (*Fake)(nil)
)
```

(`Client` is declared in task 33's `client.go`; until task 33 lands, **`client.go`** declares
`type Client struct{}`, `New`, and every `Hub` method returning `ErrNotConfigured`, so the package
compiles and `New` exists for phases to wire. Task 33 replaces that stub file-for-file. As built,
`New` and the stub are in `client.go`, not `hubclient.go`, and `NewFake` is in `fake.go`: task 33
(lane C) declares `New` in its own `client.go`, and `hubclient.go` is lane A's alone (plan, "the
shared files"), so a stub in `hubclient.go` would have made task 33 edit it — *As built*, D1.)

```go
// control-plane/internal/hub/hubclient/fake.go — the fake's exported surface, complete.

// Fake is an in-memory Hub: one org ("Example Org", tenant
// 00000000-0000-0000-0000-00000000000e), one principal at a time. It is safe for
// concurrent use; every view (As) shares the same state.
type Fake struct{ /* unexported: mutex, state, principal, clock, switches, call log */ }

// FakeUser names a principal. NewFake starts as FakeAda.
type FakeUser struct{ ID, DisplayName, Email string; OrgAdmin bool }

var (
	FakeAda   = FakeUser{ID: "u-00000000000000a1", DisplayName: "Ada Example", Email: "ada@example.test"}
	FakeGrace = FakeUser{ID: "u-00000000000000a2", DisplayName: "Grace Example", Email: "grace@example.test"}
	FakeLinus = FakeUser{ID: "u-00000000000000a3", DisplayName: "Linus Example", Email: "linus@example.test"}
)

// As returns a view of the same Hub acting as u. A user is provisioned on first sight,
// as resolve_principal does (contract §5.1 step 5).
func (f *Fake) As(u FakeUser) *Fake

// SetNow replaces the clock (default time.Now), for reservation expiry.
func (f *Fake) SetNow(now func() time.Time)

// SetSignedOut makes every call return ErrSignedOut until cleared.
func (f *Fake) SetSignedOut(on bool)

// SetUnavailable makes every call return an error wrapping ErrUnavailable until cleared.
func (f *Fake) SetUnavailable(on bool)

// SetReservationTTL overrides the 15-minute default (contract §7.9).
func (f *Fake) SetReservationTTL(d time.Duration)

// EraseBrief is the org-admin erasure (contract §7.12), for tests of the erased state.
func (f *Fake) EraseBrief(briefID, reason string) (hubapi.EraseResponse, error)

// Calls lists "Method(arg0, …)" for every call made through any view, oldest first.
func (f *Fake) Calls() []string
```

#### What the fake must do — part 1's rules, each with its contract section

Every error is an `*Error` whose `Status` and `Problem.Error` are exactly the Hub's; every id is
`ids.Wide(prefix)`; every time is `hubapi.FormatTime(now())`.

1. **Principals and teams (§7.2, §7.3, §5.3).** `Me` returns the view's user, the org, `orgAdmin`,
   the user's teams, and binds pending invites for the user's email (`invitesBound`). `CreateTeam`
   makes the caller `admin`; names are unique per org case-insensitively (409 `conflict`,
   `details.field=name`). A team the caller is not in is 404 `team_not_found` for every team
   method. `SetMember`, `RemoveMember`, `CreateInvite`, `RevokeInvite` need `admin` (403
   `forbidden`, `details.required=admin`), except that anyone removes themself; demoting or
   removing the last admin is 409 `last_admin`. A second pending invite for one email is 409
   `conflict`; revoking a bound invite is 409 `conflict`.
2. **Projects and remotes (§7.4).** `CreateProject` needs `member` of `teamId` (a team the caller is
   not in: 404 `team_not_found`); names unique per team; every remote passes
   `hubapi.ValidRepoKey` (400 `bad_request`, `details.field=remotes`); a repo key linked anywhere in
   the org is 409 `repo_already_linked`. `ProjectForRepo` answers only a project the caller can
   read. `Project`/`AddRemote` on a project the caller cannot read: 404 `project_not_found`;
   `AddRemote` needs `admin`.
3. **Briefs (§7.5).** `CreateBrief`: one live project brief per project and one live branch brief
   per `(project, branchRef)`, else 409 `brief_exists` with `details.briefId`; `scope` `project`
   with a non-empty ref, or an invalid ref, is 400. `List` answers route 19: the project's heads
   oldest first, filtered by `scope` and `branchRef`, archived ones only with `IncludeArchived`,
   paged by `Page`; a project the caller cannot read is 404 `project_not_found`; a bad `scope` or
   ref is 400 naming it. `ForBranch` never 404s for an absent brief, and each
   `BriefWithRevision` it (and `Brief`) returns carries `LatestComplete` by task 03's rule.
   `Rebind` is for branch briefs only and moves no revision. `Archive` frees the ref; a write to an
   archived brief is 409 `brief_archived`.
4. **Revisions (§7.5).** `WriteRevisions`: 1–`hubapi.MaxRevisionBatch` (64) items with distinct brief ids, all of this project,
   **all or nothing**; every `baseRevision` must equal its head, else 409 `revision_conflict`
   naming the first failing brief (in brief-id order) and its `headRevision`; content passes
   `hubapi.ValidateContent`; each new revision is `base + 1`, its envelope built and digested with
   `hubapi.BriefEnvelope`; `latestCompleteRevision` moves only on a `complete` revision.
   `Revisions` lists oldest first, without content.
5. **The resolver, brief half (§7.5).** `Resolve`: one result per key in request order;
   `invalid` for a non-key; `missing` for a key naming nothing **or anything the caller cannot
   read**; `erased` after `EraseBrief` (content `null`; key, digest, `savedAt`, `savedBy` kept);
   `resolved` otherwise. `heads` carries each brief named by a resolved or erased brief key.
   Design keys are `missing` until task 05.
6. **Switches.** `SetSignedOut(true)` and `SetUnavailable(true)` win over every rule above.

#### Tests to write first — `fake_test.go`, `hubclient_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestErrorCarriesTheProblemUnchanged` | `(&Error{409, Problem{"revision_conflict","…",{"headRevision":"6"}}}).Error()` names the code; `Code(fmt.Errorf("x: %w", e))` = `revision_conflict`; `Code(io.EOF)` = `""` | a relay that loses the code when an error is wrapped |
| `TestFakeMeProvisionsAndBindsInvites` | Ada creates a team and invites `grace@example.test` as `member`; `As(Grace).Me()` returns `invitesBound: 1` and the team with role `member`; a second `Me` binds 0 | invites that never bind, or bind twice |
| `TestFakeTeamRoles` | Grace (member) cannot `CreateInvite` (403 `forbidden`, `required=admin`); Linus (not in the team) gets 404 `team_not_found`; Ada cannot remove herself as last admin (409 `last_admin`) | a 403 that confirms a team exists to an outsider |
| `TestFakeProjectsAndRemotes` | Ada creates a project with remote `github.com/lcfrai/panaceaapp`; a second project with it → 409 `repo_already_linked`; `ProjectForRepo` finds it for Ada, not for Linus (`ok == false`); `azure-devops/kpmgcomau/panacea app/panaceaapp` is accepted, `kpmgcomau.visualstudio.com/p/r` is 400 | the contract's old repo-key form (A1); cross-team visibility |
| `TestFakeBriefExistsAndArchiveFreesTheRef` | a second live branch brief on one ref → 409 `brief_exists` with the first's id; after `Archive` a new one succeeds; writing to the archived one → 409 `brief_archived` | a unique constraint that ignores archival |
| `TestFakeRevisionNumberingAndConflict` | two writes on base 0 → revision 1, then base 0 again → 409 `revision_conflict`, `headRevision: "1"`; numbers are gapless | a revision number taken from the request |
| `TestFakeBatchIsAllOrNothing` | four briefs, item 3's base stale → 409 and **no** head moved | a partial batch commit (critique-2 §7.1 #3) |
| `TestFakeBatchHoldsSixtyFour` | 64 branch briefs re-revisioned in one call → 201, 64 revisions in request order; 65 → 400 `field: revisions`, nothing written | phase 4's fan-out refused at 17 (HA4-3), or a cap that is not `hubapi.MaxRevisionBatch` |
| `TestFakeListBriefs` | a project brief, two branch briefs, one archived → `List{}` holds three, oldest first; `Scope:"branch"` two; `IncludeArchived` four; `BranchRef` one; `Page.Limit:1` pages with `next`; Linus → 404 `project_not_found` | phase 4's fan-out missing an inheriting branch brief (HA4-2) |
| `TestFakeForBranchCarriesLatestComplete` | revisions 1 complete, 2 open → `branchBrief.revision` @2, `latestComplete` @1; revision 3 complete → both @3; a brief with no complete revision → `latestComplete: null` | a session materialised from an incomplete revision (HA4-4) |
| `TestFakeDigestIsTheEnvelopes` | a written revision's `Digest` equals `hubapi.BriefEnvelope{…}.Digest()` for its fields | a fake whose digests the real client would reject as `missing` |
| `TestFakeLatestCompleteMovesOnlyOnComplete` | open → complete → open: `latestCompleteRevision` is 0, 2, 2 | Q1's pin rule fed the wrong revision |
| `TestFakeResolveStates` | 5 keys: a readable revision (`resolved`), a revision 9 that does not exist (`missing`), another team's revision (`missing`, not 404), an erased one (`erased`, content null, digest kept), `b-42` (`invalid`, kind null); results in request order; `heads` has the two briefs | a resolver that leaks existence; one that drops erased revisions |
| `TestFakeSwitches` | `SetSignedOut(true)` → every method `errors.Is(err, ErrSignedOut)`; `SetUnavailable(true)` → `errors.Is(err, ErrUnavailable)` | a phase's signed-out path untestable against the fake |

#### Steps

- [ ] **Step 1:** Re-read contract §5.3, §7.2–§7.5 and §9.2.
- [ ] **Step 2:** `hubclient.go` with the interfaces, `Error`, the sentinels; `client.go` with the
  `Client` stub and `New` (D1). As built, the package cannot build yet: `var _ Hub = (*Fake)(nil)`
  needs the `Fake`, so Step 3's first red is that compile failure (`undefined: Fake`).
- [ ] **Step 3:** The tests above; they fail — first to compile, then against a temporary stub
  answering 501 for every part-1 method (15 of 16; `TestErrorCarriesTheProblemUnchanged` passes
  because Step 2 wrote `Error` and `Code`, and its red is the mutant `Code` without `errors.As`).
  The stub red is setup-level: each of the 15 fails at its first call (`CreateTeam` → 501), which
  proves each test drives the fake and no more. The per-assertion red is the mutation record
  (*As built*, "Mutants"), not the stub run.
- [ ] **Step 4:** `fake.go`, `fake_projects.go`, `fake_briefs.go`. Pass under `-race`. The
  `var _ Hub = (*Fake)(nil)` assertion needs part 2's methods: until task 05, `fake.go` gives each
  part-2 method a body returning `&Error{501, …"not_in_fake_yet"}`, which task 05 deletes.
- [ ] **Step 5:** Commit by pathspec, the seven files (`client.go` included, D1).

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubclient/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 16 (the table's 14 + D2's 2)
go vet ./control-plane/internal/hub/hubclient/                                                 # no output
go list -deps ./control-plane/internal/hub/hubclient | grep panacea-app
#   …/internal/httpx   …/internal/hub/hubapi   …/internal/hub/hubclient   …/internal/ids   — and nothing else
```

#### What this task does not claim

- That the fake is the Hub: it has no Postgres, no row-level security and no audit chain. The
  conformance suite (task 05) is what makes the two agree, and only for what a caller observes.
- Designs, releases, evidence and the feed: task 05.
- Any HTTP behaviour: tasks 33 and 34.

#### As built (2026-09-25, commit 142bc7b)

- **D1 — the stub's file.** `Client`, `New` and the 38 `ErrNotConfigured` methods are in
  `client.go`; `NewFake` is in `fake.go`. `hubclient.go` holds the interfaces, `Error`, `Code`, the
  three sentinels and the two `var _ Hub` lines. Reason above the Steps; seven files, not six.
- **D2 — the count** is 16, not 11: the table has 14 rows (the 11 predates HA4-2/3/4's three), and
  two tests are added. `TestFakeCallsListsEveryCall` pins `Calls()` (order across views, refused and
  switched calls included, a copy returned). `TestFakeIsSafeForConcurrentUse` pins "safe for
  concurrent use" under `-race` (16 writers through two views).
- **D3 — `EraseBrief` is held to §5.3's matrix.** A view whose `FakeUser` is not `OrgAdmin` gets 403
  `forbidden`, `required: org_admin`; `reason` is 1–500 characters (400 `field: reason`); a brief
  not in the org is 404 `brief_not_found`. It reaches every brief of the org, readable or not
  (§5.2: erasure is the org admin's; reading content is not). Carried to task 05
  (`carried/task-05.md` item 1): its `five_states` row needs an org-admin view.
- **D4 — where the part is silent, the server tasks' text is followed**, so the conformance suite
  can agree: a malformed id is the 404 of its kind (task 15); teams by `lower(name)`, members by
  `lower(display_name)`, invites newest first (15, 16a); `SetMember` adds with `addedBy` the caller,
  400 `field: role`, 404 `member_not_found` for no such user (15); invites 400 `field: email`
  (3–254, one `@`, lowercased) or `field: role`, 409 `conflict` `field: email` for a member's email
  (16a); `CreateProject` 403 `required: member` for a viewer, at most 20 distinct remotes, 409
  `conflict` `field: name` case-insensitively per team (17); `ProjectForRepo` 400 `field: repo`
  for a key of neither host (17); `AddRemote` 400 `field: host` or `field: repoKey` (17); a viewer
  is 403 `required: member` on every brief write (§5.3, 18); route 26 in task 19's order (count and
  distinct ids, then the role, then every content, then 404 for a brief not this project's, then
  409 `brief_archived`, then `revision_conflict` in brief-id order, then the envelope's 64 KiB); a
  content error's `field` is `revisions[i].content` plus the `hubapi.FieldError` path, as
  `FieldError`'s doc says. Lists: `limit` 0 is 50, 1–200 taken, else 400 `field: limit`. The
  `Revisions` cursor is the last revision in decimal (task 19: `after=1&limit=1` is `[2]`): any
  canonical decimal ≥ 0 that fits a Go `int` is taken — `"0"` is the whole history, the head or
  past it an empty last page with no `next` — and any other spelling (`"01"`, `"-1"`, `"+1"`,
  `"x"`), or a decimal too large for an `int` (`strconv.Atoi`'s range error; no list ever gave
  one), is 400 `field: after` (fix round 2, re-review m1). The `List` cursor is the last head's id: an id that is not a brief of this
  project — one that names nothing, or a brief of another project, readable or not — is 400
  `field: after`, the same answer either way, so the cursor tells an outsider nothing (*As built*,
  fix round 1; carried to tasks 18, 19 and 05).
- **D5 — three rulings on the facts.** `ForBranch` with `""` is 400 `field: branchRef` (route 21's
  ref is required; a project brief alone is `List{Scope: "project"}`). `Rebind` checks
  `brief_archived` before `brief_exists` (a write to an archived brief is always refused), and a
  rebind to the brief's own ref is a no-op. `Resolve` takes any number of keys, 0 included
  (`results: []`, `heads: {}`), as the interface says; the Hub's 1–100 is the client's to honour.
- **D6 — the switches.** Both on: `ErrSignedOut` (no token, no request). A switched call is still
  listed by `Calls()`. The part-2 placeholders are logged and switched like every call.
- **D7 — no digest re-check on read.** Nothing but the fake writes its bytes, so none can drift;
  the check would be code no test can reach.
- **Red for `TestErrorCarriesTheProblemUnchanged`** is mutant N01, not the stub run: Step 2 writes
  `Error` and `Code` before Step 3's tests, so the test passed against the stub fake.
- **Mutants** (142bc7b, then d51fdae): every mutant this part's table names is KILLED, by the test
  the table pairs it with (N01–N14; N04c, N11 and N13a in their compiling forms). The two named in
  advance as likeliest to survive did: S1, a write that leaves the head's `updatedAt`; S2, a
  default page of 200. d51fdae pins `updatedAt` on write, archive and rebind (a self-rebind is a
  no-op) and the default page of 50 over 65 heads; round 2 killed S1, S2 and R01–R04. The 20
  more on this task's own rules (X01–X23; X03, X13, X19 unused) are all KILLED, X08 and X23 in
  their compiling forms. No live survivor; none equivalent.
- **Fix round 1** (review `task-04-review.md`: 0 C, 2 I, 6 m; commits 4dff13e and after). No
  production code changed: the code was right, and eight of the reviewer's mutants passed the
  suite. `fake_test.go` now pins: a revoked invite never binds; a re-invite after a revoke is
  pending; one pending invite is per team; one `Me` binds every pending invite for the email; the
  member check on an invite is this team's; a team's revoke route never reaches another team's
  invite (I1, I2); a brief of another project is no `List` cursor (M1); the `Revisions` cursor rule
  above (M2); a second erase leaves the head (M3); `Rebind` of an archived brief onto a live ref is
  `brief_archived` (D5). The review's survivors RV-A, RV-B, RV-C, RV-D, RV-E, RV-F, RV-G2 and RV-L
  are KILLED, with RV-H, RB-1, RV-D2 and M2-0. The two named in advance, PS-1 (`Me` binds one
  invite per call) and PS-2 (the member check spans every team), survived as named and are killed
  by the round's second commit. Carried: I1, I2 → task 16a; M1 → 18; M2 → 19; M4 → 18 and 34;
  a conformance row for each → 05. M5 and M6 are this part's own wording, above.
- **Fix round 2** (re-review `task-04-review-1.md`: 0 C, 2 I, 2 m). No production code changed
  again: the code was right, and the re-review's MX-1 (`Me` without its email match), MX-2 and
  MX-2b (`RevokeInvite` without its admin check, or admitting a member) passed the suite.
  `fake_test.go` now pins: `Me` binds only the caller's own email, while another person's admin
  invite is pending (N1); a member and a viewer get 403 `required: admin` on `RevokeInvite`, and
  the invite stays pending (N2); a second revoke is 204 and changes nothing (m2, the ruling carried
  to 16a); the overflowing cursor above (m1). The two named in advance survived as named and are
  pinned: PS2-1, the role check after the invite lookup (a member now gets 403 for a missing and
  for a bound invite), and PS2-2, an email never refreshed after first sight (binding follows a
  changed address). Found beside PS2-1 and pinned: PS2-3, `CreateInvite` checking the body before
  the role. Carried: N1, N2, m2 → 16a; m1 → 19; a conformance row for each → 05.
