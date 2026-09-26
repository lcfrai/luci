### Task 07: `discoverhub` — binding and reads, against the Hub's fake

The control plane's side of `OD-3`. This package is the **only** phase 4 code that talks to the
Hub, and it talks only through `hubclient.Briefs` and `hubclient.Projects` (Hub contract §9.2). It
turns local ids (`p-…`, `b-…`, path-hashed, never shared — ADR-H1 §1.5) into Hub ids (`pj-…`,
`refs/heads/…`), reads briefs, resolves keys, and answers the Materialiser's one question: *what
does a session on this branch inherit now?* Every test runs against `hubclient.NewFake()`, through
the phase-4-owned wrapper in `fakehub_test.go` (its test hooks, below; a `List` of its own only
if H0 had lacked HA4-2's, which as built, note 1, it did not).

**Depends on the Hub's task H0** (interfaces, fake, conformance skeleton — ADR-H1 §5.9 ships them
first). **Blocked by Hub contract ask HA4-2** (`Briefs.List`) for the real Hub — the same ask that
blocks task 08 (plan §6, §9). Phase 4 writes **nothing** under `control-plane/internal/hub/` (plan
§5, "Never touched"), so until HA4-2 lands the task builds against phase 4's **own** seam:

- `Hub` below is phase 4's interface, and it declares `List` itself. It still embeds
  `hubclient.Briefs`; Go (1.14 and later) accepts an identical method from two embedded sources, so
  the **method** compiles whether or not the Hub's `Briefs` has `List` yet. If H0's `List` lands
  with a different signature, that is a compile error here, which is the right place to learn it.
- The seam supplies HA4-2's **method**, not its **types**. `List` names `hubapi.BriefQuery`, which
  exists only as part of HA4-2, and phase 4 may not write it into `internal/hub/hubapi` (plan §5).
  So H0 without `BriefQuery` blocks this task exactly as no H0 does (Step 1 checks both). The Hub
  plan puts `BriefQuery` and `Briefs.List` in H0 (`phase-h-hub-parts/task-03.md:283-290`,
  `task-04.md:82`), so the wrapper's `List` is expected to be needed only if H0 slips HA4-2's
  method while keeping its type.
- `fakehub_test.go` is a thin phase-4-owned wrapper: a struct that embeds `hubclient.NewFake()`,
  adds `List` over the revisions the test seeded, and adds the four hooks named in **Files** below
  (`renameProject`, `failResolve`, `failList`, `failCreateProject`). It lives in `discoverhub`, not in the Hub's package.
- When HA4-2 lands, the wrapper's `List` is deleted and the embedded fake's own `List` shows
  through; `Hub` stays as it is, and so do the four hooks. That is a one-file change, and the
  unchanged tests prove it.

The orchestrator carries HA4-1..13 to the Hub planner (plan §9; HA4-13, `branchRef` on `/v1/refs`'
heads, is this task's rule 8; HA4-12, the per-team name rule's
error code, is this task's: `NameTakenError` below); this task does not wait on the
reply to start, but it cannot claim the real Hub until HA4-2 is in the Hub contract.

**Files:**
- `control-plane/internal/discoverhub/service.go` — `Config`, `Service`, `New`, the cache (new)
- `control-plane/internal/discoverhub/binding.go` + `binding_test.go` (new)
- `control-plane/internal/discoverhub/read.go` + `read_test.go` (new)
- `control-plane/internal/discoverhub/fixtures_test.go` — the fact sheet's world in the fake (new)
- `control-plane/internal/discoverhub/fakehub_test.go` — `fakeHub{*hubclient.Fake}`, whose own
  `List` shows through (HA4-2 is in H0: as built, note 1; had it not been, the wrapper would add a
  `List` over the seeded revisions), and four test hooks H0's fake does not plan
  (`phase-h-hub-parts/task-04.md:188-226` lists its whole surface, and none is in it) (new;
  phase 4's, not the Hub's). Rule 8's rows need `List` to answer each brief's **current**
  head: a `Rebind` or an `Archive` made through the fake shows through, and `SetUnavailable`
  fails it. The Hub branch's built fake already does all three and logs `List` in `Calls()`
  (hub `3df9fab`, `hubclient/fake_briefs.go:176-216`, read for task 06 fix round 1). A wrapper
  `List`, if Step 1 still needs one, must do the same, and counts its own calls for those rows.
  The hooks:
  - `renameProject(id, name string)` — from then on the wrapper's `Project` and `ProjectForRepo`
    return the embedded fake's answer with `Name` replaced. It stands in for route 16,
    `PATCH /v1/projects/{id}`, which `hubclient.Projects` has no method for.
  - `failResolve(n int, err error)` — the n-th `Resolve` call made through the wrapper (1-based,
    counted from the hook) returns `err` without reaching the embedded fake; every other call
    passes through. It gives a chosen `*hubclient.Error` status and a failure on the second call
    only; `SetUnavailable` fails every call. The wrapper logs every `Resolve`'s keys in its own
    `resolves()`, the intercepted calls included, because the fake's `Calls()` never sees those
    (re-review b05-2 R2-I1, item 2); `lists()` does the same for `List`.
  - `failList(n int, err error)` (task 06 fix round 2, re-review 1 R1-I1) — the same for the
    n-th `List` call, so a walk can fail on a later page while its first page answered. It
    wraps whichever `List` is in use, the wrapper's or the embedded fake's.
  - `failCreateProject(err error)` (b10 re-review I-2) — the next `CreateProject` through the
    wrapper returns `err` without reaching the embedded fake, once. It stands in for the Hub's
    per-team name rule (`hub-contract.md:710`), which H0's fake is not known to enforce and whose
    code the contract does not name (HA4-12).

**Assumed `hubapi` shapes (Hub contract ask HA4-1).** The contract names these structs (§9.2) and
fixes their JSON (§7.1, §7.5) but writes no Go. Phase 4 codes against this reading; a field-name
difference when H0 lands is a mechanical fix, not a design change:

```go
package hubapi // as ASSUMED by phase 4; the Hub's H0 is the authority

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}
type Team struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"` // the caller's
	CreatedAt string `json:"createdAt"`
	CreatedBy User   `json:"createdBy"`
}
type Me struct {
	User         User   `json:"user"`
	OrgAdmin     bool   `json:"orgAdmin"`
	Teams        []Team `json:"teams"`
	InvitesBound int    `json:"invitesBound"`
}
type Remote struct {
	RepoKey string `json:"repoKey"`
	Host    string `json:"host"` // "github" | "azure-devops"
	AddedAt string `json:"addedAt"`
}
type Project struct {
	ID        string   `json:"id"`
	TeamID    string   `json:"teamId"`
	Name      string   `json:"name"`
	Remotes   []Remote `json:"remotes"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy User     `json:"createdBy"`
}
type RemoteRequest struct {
	RepoKey string `json:"repoKey"`
	Host    string `json:"host"`
}
type CreateProjectRequest struct {
	TeamID  string          `json:"teamId"`
	Name    string          `json:"name"`
	Remotes []RemoteRequest `json:"remotes"`
}
type BriefHead struct {
	ID                     string  `json:"id"`
	ProjectID              string  `json:"projectId"`
	Scope                  string  `json:"scope"`     // "project" | "branch"
	BranchRef              string  `json:"branchRef"` // "" for project scope
	HeadRevision           int     `json:"headRevision"`
	HeadKey                *string `json:"headKey"` // null while headRevision is 0
	HeadDigest             *string `json:"headDigest"`
	HeadStatus             *string `json:"headStatus"`
	LatestCompleteRevision int     `json:"latestCompleteRevision"`
	ArchivedAt             *string `json:"archivedAt"`
	CreatedAt              string  `json:"createdAt"`
	CreatedBy              User    `json:"createdBy"`
	UpdatedAt              string  `json:"updatedAt"`
}
type BriefRevision struct {
	Key       string          `json:"key"`
	BriefID   string          `json:"briefId"`
	Revision  int             `json:"revision"`
	Digest    string          `json:"digest"`
	ProjectID string          `json:"projectId"`
	Scope     string          `json:"scope"`
	BranchRef string          `json:"branchRef"`
	Status    string          `json:"status"`
	Content   json.RawMessage `json:"content"` // null exactly when ErasedAt is set
	SavedAt   string          `json:"savedAt"`
	SavedBy   User            `json:"savedBy"`
	ErasedAt  *string         `json:"erasedAt"`
}
type BriefWithRevision struct {
	Head     BriefHead      `json:"head"`
	Revision *BriefRevision `json:"revision"`
}
type ForBranch struct {
	BranchBrief  *BriefWithRevision `json:"branchBrief"`
	ProjectBrief *BriefWithRevision `json:"projectBrief"`
}
type CreateBriefRequest struct {
	Scope     string `json:"scope"`
	BranchRef string `json:"branchRef"`
}
type RevisionWrite struct {
	BriefID      string          `json:"briefId"`
	BaseRevision int             `json:"baseRevision"`
	Content      json.RawMessage `json:"content"`
}
type RevisionSummary struct {
	Key      string  `json:"key"`
	Revision int     `json:"revision"`
	Digest   string  `json:"digest"`
	Status   string  `json:"status"`
	SavedAt  string  `json:"savedAt"`
	SavedBy  User    `json:"savedBy"`
	ErasedAt *string `json:"erasedAt"`
}
type PageRequest struct {
	After string
	Limit int
}
type Page[T any] struct {
	Items []T     `json:"items"`
	Next  *string `json:"next"`
}
type RefResult struct {
	Key        string          `json:"key"`
	Kind       *string         `json:"kind"`       // "brief" | "design" | null
	Resolution string          `json:"resolution"` // "resolved" | "missing" | "erased" | "invalid"
	Revision   *BriefRevision  `json:"revision"`
	Approval   json.RawMessage `json:"approval"` // phase 5's; phase 4 ignores it
}
type RefHead struct {
	HeadRevision           int  `json:"headRevision"`
	LatestCompleteRevision int  `json:"latestCompleteRevision"`
	Archived               bool `json:"archived"`
}
type RefsResponse struct {
	Results []RefResult        `json:"results"`
	Heads   map[string]RefHead `json:"heads"`
}
type BriefQuery struct { // HA4-2 — superseded by the Hub's Page shape, Step 2 (read below)
	Scope           string
	BranchRef       string
	IncludeArchived bool
	After           string // superseded: the Hub's BriefQuery carries Page hubapi.PageRequest
	Limit           int    // superseded: as After
}
```

*Read 2026-09-24 from the Hub's own plan (`.claude/worktrees/hub`, `feat/hub` at `d83ff43`,
`2026-09-23-phase-h-hub.md:502-512`):* the Hub planner has answered HA4-1..9 in its plan (not
yet in the Hub contract, and not built). HA4-2 is answered with the same `List` signature as
above (`phase-h-hub-parts/task-04.md:82`), but `BriefQuery` carries `Page hubapi.PageRequest`
(`task-03.md:285-290`) instead of flat `After`/`Limit`. Step 2 takes the Hub's shape — a mechanical
rename, as for HA4-1 — and the wrapper fake's `List` follows it.

**Interfaces — complete source** (`discoverhub`):

```go
// control-plane/internal/discoverhub/service.go

// Package discoverhub is the control plane's Hub-backed brief service. It is
// the only phase 4 package that calls the Hub, and it calls it as the
// signed-in user through hubclient (ADR-H1 §5.4). It persists nothing: the
// only state it keeps is an in-memory cache of immutable revisions.
package discoverhub

// Hub is the part of hubclient this package uses. It is phase 4's own
// interface. HA4-2 reached the Hub in H0 (hubclient.Briefs declares List,
// hub 3df9fab8), so *hubclient.Client and *hubclient.Fake satisfy Hub
// unchanged; List is still declared here, identically, so a change to its
// signature in the Hub is a compile error in this package, the right place to
// learn it. Nothing in this package writes under internal/hub.
type Hub interface {
	hubclient.Briefs
	hubclient.Projects
	List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) // HA4-2
}

// RepoKey is one remote of a local repository, in the Hub's form.
type RepoKey struct {
	Key    string // githost.Remote.RepoKey() (phase 3 task 13)
	Host   string // "github" | "azure-devops"
	Remote string // the git remote's name: "origin", "upstream", …
}

// RepoIdentity lists a local repository's recognised remotes, in no order
// this package relies on (task 14 returns git's, which is alphabetical).
// Unrecognised hosts are left out, not errors. Task 14 wires it over phase
// 3's githost.ParseRemote and a read-only `git remote` /
// `git remote get-url` through project.Runner.
type RepoIdentity interface {
	RepoKeys(ctx context.Context, projectPath string) ([]RepoKey, error)
}

// Locator maps local ids to what this package needs. Task 14 adapts
// *project.Scanner to it.
type Locator interface {
	ProjectPath(ctx context.Context, projectID string) (string, error) // project.ErrUnknownProject when absent
	ProjectName(projectID string) string
	BranchName(branchID string) string // "evidence-upload"; "" for "" 
}

type Config struct {
	Hub      Hub // nil: the Hub is not configured (PANACEA_APP_HUB_URL unset)
	Repos    RepoIdentity
	Locate   Locator
	Model    modelclient.Completer // nil: no model; follow-ups degrade
	Evaluate provenance.Evaluate   // nil: every criterion open
	Now      func() time.Time
	NewID    func(prefix string) string
	Log      *slog.Logger
	// CacheTTL bounds how long a resolved revision is held. Revisions are
	// immutable, but an org admin can ERASE one (ADR-H1 §5.6); until the
	// change-feed hook of ask HA4-8 exists, the TTL is how an erasure
	// reaches this process. Default 5 minutes; at most 4096 entries.
	CacheTTL time.Duration
}

type Service struct{ /* unexported: cfg, cache, binding memo */ }

func New(cfg Config) (*Service, error) // errors name the missing field; Hub may be nil

var (
	ErrHubOff      = errors.New("discoverhub: the Hub is not configured")
	ErrNotBound    = errors.New("discoverhub: this project is not shared through the Hub")
	ErrNoRemote    = errors.New("discoverhub: this repository has no GitHub or Azure DevOps remote")
	ErrNoBrief     = errors.New("discoverhub: no brief at this scope")
	// ErrPinUnreadable (task 07 as built): a branch revision's pin the Hub
	// answers missing. Inheritable and HeadRef return it rather than resolve
	// the branch against anything but its pin (rule 3); Resolve, which has no
	// error, answers that slot unavailable (rule 5).
	ErrPinUnreadable = errors.New("discoverhub: the project revision this branch revision pins cannot be read")
)
```

```go
// control-plane/internal/discoverhub/binding.go

type BindingState string

const (
	BindingBound    BindingState = "bound"
	BindingUnbound  BindingState = "unbound"   // it has remotes; no Hub project has its primary one (Matches may name projects that have another)
	BindingNoRemote BindingState = "no_remote" // OQ-1
	BindingHubOff   BindingState = "hub_off"
)

// RemoteMatch is a Hub project found through a remote that is NOT the
// primary one: typically a fork's clone, whose origin is the person's fork
// and whose upstream is the team's repository (review M8). It is shown as a
// fact beside the choice to share, never bound to on its own, because
// binding a fork to the team's project would put the fork's briefs there.
type RemoteMatch struct {
	Remote         string `json:"remote"`         // "upstream"
	RepoKey        string `json:"repoKey"`
	HubProjectID   string `json:"hubProjectId"`
	HubProjectName string `json:"hubProjectName"`
}

type TeamChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // the caller's; Bind needs member or above
}

type Binding struct {
	State          BindingState  `json:"state"`
	HubProjectID   string        `json:"hubProjectId"`   // "" unless bound
	HubProjectName string        `json:"hubProjectName"` // "" unless bound
	RepoKeys       []string      `json:"repoKeys"`       // what was looked up, in order; [0] is the primary remote's
	Teams          []TeamChoice  `json:"teams"`          // unbound only: where it could be created
	Matches        []RemoteMatch `json:"matches"`        // unbound only: projects found through a non-primary remote; [] otherwise
	DefaultName    string        `json:"defaultName"`    // unbound only: DefaultName(local name, this binding), what Share pre-fills (b10 re-review I-2); "" otherwise
}

// orderRemotes is the one place the PRIMARY remote is chosen (review M8;
// b10 re-review I-1): the entry whose Remote is "origin" when there is one,
// otherwise the first by Remote name; the rest follow by Remote name. The
// primary is the one Binding binds by and the only one Bind registers. It
// lives here, beside its only callers, and not in RepoIdentity, so no
// implementation's order can move it: git lists "company" before "origin",
// and a fork whose team remote sorts first would otherwise bind to the
// team's project, the defect M8 exists to stop. Pure; it does not mutate
// keys.
func orderRemotes(keys []RepoKey) []RepoKey

// Binding orders RepoIdentity's answer with orderRemotes, then looks the
// project up by its PRIMARY remote's key (RepoKeys[0]) and
// is bound only to the project that key names (review M8). When there is
// none, it looks up each other remote's key in order and lists every hit in
// Matches, state unbound: the person chooses (task 18), nothing binds on its
// own. It is memoised in memory per local project for 60 s, and forgotten on
// Bind. A lookup that was in flight when Bind forgot the memo does not
// store its answer (review t07 m2): it read the Hub before the Bind, and
// would hold a stale unbound for 60 s.
func (s *Service) Binding(ctx context.Context, projectID string) (Binding, error)

// Bind creates the Hub project in teamID with the PRIMARY remote's key as
// its only remote (review M8: a fork's clone must not claim the team's
// repository through its upstream remote), named name, or DefaultName when
// name is "" (after trimming). It reads the binding through Binding's memo,
// which may predate a teammate's Share. So each 409 that a teammate's race
// can cause re-reads the binding past the memo, and a re-read that is bound
// forgets the memo and is returned instead of the error (review t07 I1):
// repo_already_linked, and a Hub 409 that refuses the name (b10 re-review
// I-2: `name` is "unique per team", hub-contract.md:710), because a teammate
// whose clone has the same directory name Shared under the same default name,
// and the contract does not fix which of the two checks the Hub runs first
// (the fake runs the name's). When the re-read is not bound,
// repo_already_linked is returned as it came (the project is one the caller
// cannot read), and the name refusal is *NameTakenError, nothing bound and
// the memo unchanged, so the person can Share again under another name. A
// re-read that fails returns its own error, unchanged. Joining a project that
// a non-primary remote matched is not built (OQ-12).
func (s *Service) Bind(ctx context.Context, projectID, teamID, name string) (Binding, error)

// DefaultName is the name Bind uses when the person gives none, and the one
// task 18 pre-fills. The local project's name (project.Scanner's directory
// name), unless the binding has Matches: then a fork's clone and the team's
// clone share a directory name ("portal"), and the team's project, if a
// teammate Shared it, holds that name already. So a binding with Matches
// gets the local name, a space, and the primary key without its host in
// parentheses: "portal (me/portal)". The whole is cut to the Hub's 120
// characters (hub-contract.md:710) by shortening the local name, never the
// key. It is a default, not a guarantee: two unrelated repositories named
// "portal" can still meet in one team, which NameTakenError covers.
func DefaultName(localName string, b Binding) string

// NameTakenError is a Bind the Hub refused because teamID already holds a
// project called Name. Matched by isNameTaken: a 409 whose details.field is
// "name". The contract states the rule (hub-contract.md:710) but names no
// code for it; team create's is 409 `conflict` with details.field "name"
// (:672), so isNameTaken is the seam that ask HA4-12 fixes.
type NameTakenError struct{ Name string }

func (e *NameTakenError) Error() string

// BranchRef is "refs/heads/" + the branch's name; "" for "".
func (s *Service) BranchRef(branchID string) string
```

```go
// control-plane/internal/discoverhub/read.go

// Target is one scope, by LOCAL ids. BranchID "" is the project scope.
type Target struct {
	ProjectID string
	BranchID  string
}

type BriefState struct {
	ID                     string              `json:"id"`
	Scope                  discover.Scope      `json:"scope"`
	HeadRevision           int                 `json:"headRevision"` // 0: created, nothing answered
	HeadKey                string              `json:"headKey"`      // "" while HeadRevision is 0
	Status                 discover.Status     `json:"status"`
	LatestCompleteRevision int                 `json:"latestCompleteRevision"`
	LatestCompleteKey      string              `json:"latestCompleteKey"` // "" when none
	Questions              []discover.Question `json:"questions"`
	Answers                []discover.Answer   `json:"answers"`
	Inherits               *discover.Inherit   `json:"inherits"`
	Change                 discover.Change     `json:"change"`
	SavedAt                string              `json:"savedAt"`
	SavedBy                string              `json:"savedBy"`
}

// InheritedState is, for a branch, the project revision a session on it
// would inherit through right now: the project's latest complete revision.
type InheritedState struct {
	Key       string              `json:"key"`
	BriefID   string              `json:"briefId"`
	Revision  int                 `json:"revision"`
	Questions []discover.Question `json:"questions"`
	Answers   []discover.Answer   `json:"answers"`
}

// Preview is the managed block a session started now would get.
type Preview struct {
	Key   string `json:"key"`
	Block string `json:"block"` // claudemd.Render output, verbatim
}

type View struct {
	Scope     discover.Scope  `json:"scope"`
	ProjectID string          `json:"projectId"` // local
	BranchID  string          `json:"branchId"`  // local; "" for project scope
	Binding   Binding         `json:"binding"`
	Brief     *BriefState     `json:"brief"`     // nil: no brief at this scope yet
	Inherited *InheritedState `json:"inherited"` // branch scope with a complete project brief; else nil
	Step      discover.Step   `json:"step"`      // over Brief, or over NewContent() when Brief is nil
	Preview   *Preview        `json:"preview"`   // nil: a session started now inherits nothing
}

func (s *Service) View(ctx context.Context, t Target) (View, error)

type HistoryRow struct {
	Key      string          `json:"key"`
	Revision int             `json:"revision"`
	Status   string          `json:"status"`
	Change   discover.Change `json:"change"` // from the revision's content; zero for an erased one
	SavedAt  string          `json:"savedAt"`
	SavedBy  string          `json:"savedBy"`
	Erased   bool            `json:"erased"`
}

type History struct {
	Rows []HistoryRow `json:"rows"` // oldest first
	Next string       `json:"next"` // "" when there is no more
}

func (s *Service) History(ctx context.Context, briefID string, after string, limit int) (History, error)

// Resolve answers every key, in order, in at most two /v1/refs calls, plus,
// only when Config.Evaluate is set, one List walk per Hub project that a
// resolved branch slot names (rule 8). The first /v1/refs call
// carries EVERY requested brief key, cached or not, and the pin key
// (Inherits.Key) of every cached branch revision among them, whether or not
// the pin is still cached. The second carries the pins of the other branch
// revisions the first call returned, cached or not. So heads names every
// brief a requested key or its pin names (Hub contract §7.5 :829): content
// is immutable, heads are not, and a cache hit must not cost the desktop its
// head (re-review b06-1 R-I1). A cached piece's content comes from the cache;
// the cache still saves the pin's bytes, and keeps a slot resolved while the
// Hub fails (rule 5). "", none:declined and invalid keys never reach the Hub.
// heads holds one entry per brief the Hub's answers named.
// states holds provenance.Readings(ctx, key, src, at, Config.Evaluate) for
// every resolved slot whose readings were taken, cached revision or not, and
// nothing for any other slot. at is "" for a project key, which Resolve never
// evaluates, and for a branch key the branch its brief is bound to now, never
// the revision's envelope BranchRef (task 06 as built; re-review b08-2 R2-I1,
// R2-M1): the BranchRef of the brief's live BriefHead, read by rule 8's List
// because /v1/refs carries no current ref (its revisions hold the envelope's,
// and its heads hold none: Hub contract §7.1 :641-648, §7.5 :829 at hub
// da8c7da; ask HA4-13). An archived brief gets at "" and is not evaluated:
// its binding is known to be gone. A branch slot whose binding rule 8 could
// not read gets NO states entry: "binding unknown" is not a reading, and an
// entry would overwrite the desktop's last one (task 06 fix round 2, re-review
// 1 R1-I1).
// criterion state moves, so it is computed on every call and is never part
// of the cached Ref (review I14).
// There is no error result: a Hub failure, a signed-out user or a nil Hub
// makes each slot the cache cannot answer "unavailable" (rule 5, review I6).
func (s *Service) Resolve(ctx context.Context, keys []string) (refs []provenance.Ref, heads map[string]provenance.Head, states map[string][]provenance.CriterionReading)

// HeadRef is the Ref of what a session on t would inherit now - the head
// reading for DefinitionOfDone and Discover (contract §3.1). resolution
// "none" when nothing would be inherited: with a nil Hub (rule 6), and, as
// built, for a project that is not shared (unbound or no_remote), where a
// session started now inherits nothing too (task 09 maps Inheritable's
// ErrNotBound and ErrNoRemote to nothing); none of the three is an error. states is
// provenance.Readings(ctx, ref.Key, src, <t's branch ref>, Config.Evaluate),
// a project key inherited by t included (task 06 as built; re-review b08-2
// R2-I1), when the Ref is
// resolved, and nil otherwise: the moving part served beside the Ref, never
// inside it, computed on every call like Resolve's (review I14). It is the
// one surface that shows criterion states, so it must not depend on some
// session having stamped the same key (re-review b06-1 R-I3).
func (s *Service) HeadRef(ctx context.Context, t Target) (provenance.Ref, []provenance.CriterionReading, error)

// Inheritable is P4-A, the Materialiser's one question: the latest complete
// branch revision, else the latest complete project revision, else none.
type Inheritable struct {
	Key      string            // "" when none
	Document discover.Document // what claudemd.Render writes; zero when none
	// PinErased: Key is a branch revision whose pinned project revision is
	// erased, so Document holds the branch's own text alone (rule 7). Task 09's
	// Inspect reads it, so a block written before the erasure is not called
	// edited (re-review b06-2 R2-I1; task 07 as built; task 09 carries its half).
	PinErased bool
}

func (s *Service) Inheritable(ctx context.Context, t Target) (Inheritable, error)
```

**The rules, each a test below.**

1. Local ids never reach the Hub; Hub ids never reach `Locator`.
2. **Titles come from the revision, never from a name** (review I5, task 05's `Document.Title`):
   every `discover.Revision` this package builds carries the envelope's own `branchRef`, and
   `discover.Resolve` (through `discover.Title`, the rule's one copy) titles a branch key
   `Branch brief — <branchRef minus refs/heads/>` and a
   project key `Project brief`. Neither the Hub project's current name (`PATCH /v1/projects/{id}`)
   nor the branch's current ref (`Rebind`) is read for a render, so a key renders the same bytes on
   every teammate's machine, before and after a rename (P4-G).
3. `Inheritable` reads `ForBranch` once; the latest complete revision's content comes from
   `ForBranch`'s own `latestComplete` (HA4-4 is in H0: `hubapi.BriefWithRevision.LatestComplete`,
   hub `3df9fab8`), so no `/v1/refs` call is made for it (task 07 as built; the part's "else from
   one `/v1/refs` call" is what HA4-4 removed). A branch revision's pinned project revision is
   fetched by its `inherits.key`, in one `/v1/refs` call whose answer wins over the cache (rule 5),
   **never** by the project's current head.
4. The cache holds `resolved` revisions only, keyed by key, for `CacheTTL`. `missing` and `erased`
   are never cached: a missing key may appear (a teammate's write not yet visible), and an erased
   one must not keep content.
5. **One rule for the Hub failing during `Resolve`** (review I6; task 12 rule 1 restates it).
   `Resolve` **never returns a Hub error** — it has no error result. Slot by slot:
   - a slot is `resolved` only when **everything it needs is in hand**, each piece from the cache
     or from a call that succeeded. A project key needs its revision. A branch key needs its
     revision **and** its pinned project revision (rule 3; `provenance.Build` reads both, task 06).
     A key whose pieces are all cached stays `resolved`, whatever the Hub's **failure** (its
     `erased` or `missing` answer for a cached key wins, below; re-review b06-2 N1);
   - every other brief key is `unavailable` when the Hub call that would answer it fails for any
     reason (`*hubclient.Error` of any status, `hubclient.ErrUnavailable`, a transport error or
     the route's deadline), when it returns `signin.ErrSignedOut`, and when `Config.Hub` is nil
     (rule 6). That covers each split case the same way: the first call succeeds and the second
     (the pinned project revisions) fails; or the branch revision is cached while its pin has
     expired or been evicted (the entries have their own TTLs and the 4096 cap, so the window is
     real), and the one call for the pin fails. Either way the branch key is `resolved` when its
     pin is in hand and `unavailable` when it is not — never resolved against anything but its
     pin, and never a branch `Ref` with no inherited text. A cached pin's expiry is **not**
     extended to cover the branch revisions that name it: that would let an erased project
     revision outlive `CacheTTL` for as long as its branches keep being read (ADR-H1 §6.8);
   - `missing` comes **only** from the Hub's own answer: a key `/v1/refs` answered as missing.
     Nothing in this package manufactures it, because the chip's words for `missing` say the text
     is lost (task 17), and a Hub that is off, down or signed out has lost nothing;
   - `""` and `none:declined` stay `none` and a malformed key stays `invalid`, Hub or no Hub;
   - `heads` holds what the Hub's answers carried; a failed call adds no entry for its briefs.
     Because the first call carries cached keys too (the doc comment above), a cached key's
     brief and its pin's brief get a head on every successful request (re-review b06-1 R-I1);
   - **the Hub's answer for a cached key wins over the cache.** `resolved` leaves the entry as it
     is (same bytes, same expiry; a key's content is immutable, so the answer's copy is not
     re-read). `erased` or `missing` evicts the entry, and the slot takes the Hub's answer; for a
     cached pin, the branch slot that needs it follows the pin's answer (an `erased` pin is
     rule 7's case, a `missing` one makes the branch slot `unavailable`: its pin is not in
     hand). So an erasure reaches `Resolve` at its next successful call, not only at the
     TTL. A Hub that is failing still leaves a cached erased revision servable until `CacheTTL`;
     that gap is HA4-8's.

   Every **other** method (`Binding`, `Bind`, `View`, `History`, `HeadRef`, `Inheritable`) returns
   each Hub error **unchanged** (`*hubclient.Error`, `hubclient.ErrUnavailable`,
   `signin.ErrSignedOut`): the relay is task 11's.
6. A nil `Config.Hub` makes `Binding` answer `hub_off`, `View` answer a view with that binding and
   no brief, `Resolve` answer `unavailable` for every brief key (rule 5; **not** `missing`),
   `Inheritable` answer none, and `HeadRef` answer a `none` Ref with **no error** — the same
   answer as `Inheritable`, whose answer it reports, and treated like `Binding`'s `hub_off`, which
   is a 200 view and not an error (re-review b05-1 N2). So a Hub that is not configured is not task 12 rule 2's
   "cannot be reached": the head reading is 200 `none`, never 503, because a session started now
   inherits nothing (task 09 step 4) and Develop is not to be blocked or toasted for a Hub it was
   never given (Hub contract §9.5, cited for its no-block clause only: its lens-state clause is
   task 17's words, where `DefinitionOfDone` on a `hub_off` machine reads *no brief*, accepted
   because the head reading is what a session would inherit; re-review b05-2 R2-M1; task 18's
   `hub_off leaves Develop alone`). A stamped key is
   different: it names a revision that may exist, so `Resolve` says `unavailable`, not `none`.
7. **A branch key whose pinned project revision is erased** (re-review b06-1 R-I2; OQ-9). The
   Hub erases every revision of a brief at once and keeps each key resolvable as `erased` (Hub
   contract §7.12 :1110-1111), so a branch revision can outlive the project text it pins. When
   the pin's answer is `erased`, the branch slot is `resolved` from **its own revision alone**:
   `Build` gets `Source.Project` nil and `Source.PinErased` true (task 06), `Inherits` keeps the
   pin's key, briefId, revision and digest with `erased: true`, and no field or section holds
   the pin's text. The pin's cached copy is never used for it: the `erased` answer evicted it
   (rule 5). `HeadRef` builds its `Ref` by the same rule. `Inheritable` returns the same
   own-text document for such a revision, with `Key` the branch key (the recommended answer to
   OQ-9). Only a Hub that is failing, with both pieces cached, still yields the old text until
   `CacheTTL`; that is HA4-8's gap, and the desktop refuses that answer (task 16 `putRefs`).
8. **The branch a by-key reading is evaluated at** (task 06 fix round 1, review I1; re-review
   b08-2 R2-M1). A rebind (`PATCH /v1/briefs/{id}`, `Rebind`) writes no revision (Hub contract
   §7.5 :780-782 at hub `da8c7da`), so a branch key's envelope `branchRef` is the ref its brief had
   when that revision was saved: after a rename it names a ref that is gone, or an unrelated branch
   that took the name. The ref the brief is bound to now is on its `BriefHead` only (§7.1
   :641-642). `/v1/refs` carries none: its `BriefRevision` holds the envelope's (:647-648), and
   its `heads` hold `headRevision`, `latestCompleteRevision` and `archived` only (:829). So:
   - With `Config.Evaluate` nil, `Resolve` reads nothing more. `Readings` calls no `Evaluate`,
     so `at` changes nothing, every `resolved` slot gets its all-`open` entry (the true answer
     where nothing evaluates), and phase 4's default configuration costs no extra call.
   - Otherwise, after its `/v1/refs` calls, `Resolve` calls `Hub.List(ctx, projectID,
     hubapi.BriefQuery{Scope: hubapi.ScopeBranch, Page: hubapi.PageRequest{Limit: 200}})` once
     for each distinct `Source.ProjectID` among the `resolved` branch slots, follows `next` until
     it is null (§1 :48-49: 200 is a page's maximum), and maps each `BriefHead.ID` to its
     `BranchRef`. `IncludeArchived` stays false. (`hubapi.ScopeBranch` is `"branch"`,
     `hubapi/envelope.go:25` at hub `3df9fab`.)
   - A branch slot's `at` is that map's entry for its `BriefID`: **never** the revision's
     envelope `BranchRef`, and never `ForBranch` of it (which answers the brief bound to that
     ref now: after a rename, another brief or none).
   - An archived brief is not in the list, so its `at` is `""`. Archiving frees its ref for a new
     brief (§7.5 :785-788), so a verdict read there would be a stranger's.
   - A `List` walk that fails, for any reason in rule 5's list, `signin.ErrSignedOut` included,
     and on any page (a failure after the first page fails the whole walk: a partial map would
     call a brief on a later page archived), leaves the binding of every branch slot of that
     project **unknown**. Such a slot gets **no** `states` entry and no `Readings` call. It is
     still `resolved` or not by rule 5 alone, and there is no error (`Resolve` has none).
     "Unknown" is not a reading: an `open` sent for it would overwrite the desktop's last
     verdict (task 16 `putRefs`) with what reads as "nothing is done", for as long as the Hub is
     down (task 06 fix round 2, re-review 1 R1-I1). A reading at a guessed ref, the envelope's
     included, would be a verdict. An archived brief is not this case: its binding is known to
     be gone, so it keeps `at` `""` and its `open` readings.
   - The `List` answer is not cached: a head moves, and a rebind is a head change.
   - Ask **HA4-13** (plan §9) puts `branchRef` on `/v1/refs`' `heads`. When it lands, this rule
     reads the first call's heads, the `List` walk is deleted, and the rows below pass
     unchanged but for their `List` counts and the `failList` clause of
     `TestCriterionStatesAreRecomputedOnACachedKey`, which goes with the walk.
   - `HeadRef(t)` is unaffected: it passes `t`'s own branch (task 06).

**Tests first** (`fakeHub` from `fakehub_test.go` — `hubclient.NewFake()` embedded, plus `List`
until HA4-2 lands, plus the `renameProject`, `failResolve`, `failList` and `failCreateProject` hooks — seeded by `fixtures_test.go` with the fact sheet's world —
project `Engagement Portal`, `b-7` revs 1–6, `b-42` revs 1–6 with the pins of fact sheet §2–§3,
chosen once (re-review b06-2 R2-M1; the fact sheet names none for `b-42@6`): `b-42` revs 1–5 pin
`b-7@5` and `b-42@6` pins `b-7@6`, saved in that order):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBindingFindsTheProjectByItsPrimaryRemote` (review M8, replaces `…ByItsRepoKey`) | `origin` linked to `pj-1` → `bound` to `pj-1`, `Matches` `[]`; `origin` linked to `pj-1` and `upstream` to `pj-2` → still `pj-1`, with no Hub lookup of `upstream` | a lookup that stops at any remote, so the fork's upstream wins when it sorts first |
| `TestAnotherRemotesProjectIsAChoiceNotABinding` (review M8) | the fork shape: `origin` = `github.com/me/portal` unlinked, `upstream` = `github.com/team/portal` linked to `pj-2` → `unbound`, `HubProjectID` `""`, `Matches == [{upstream, github.com/team/portal, pj-2, "Engagement Portal"}]`, `Teams` filled; a write (task 08) is `ErrNotBound` | the old rule, which bound a fork to the team's project and wrote the fork's briefs there |
| `TestBindRegistersThePrimaryRemoteOnly` (review M8) | the fork shape with `upstream` unlinked → `CreateProject`'s `remotes` is exactly `[{github.com/me/portal, github}]`; a repository with no `origin` and two recognised remotes `a` and `b` → `[a's key]`, and `Binding` then reads `bound` | a Bind that claims the team's repository under the fork's new project, so the team's own later Share meets `repo_already_linked` |
| `TestThePrimaryRemoteIsOriginInAnyOrder` (b10 re-review I-1) | the fake `RepoIdentity` answers in git's alphabetical order, `company` (`github.com/team/portal`, linked to `pj-2`) then `origin` (`github.com/me/portal`, unlinked) → `Binding` is `unbound`, `RepoKeys[0]` is `github.com/me/portal`, `Matches` names `company` and `pj-2`; `Bind` registers `origin`'s key alone; the same with `azure`, `github` and `team` as the team remote's name; with no `origin`, `zeta` then `alpha` → `alpha` is primary; `orderRemotes` leaves its input slice as it was | a primary taken as `RepoKeys[0]` of whatever `RepoIdentity` returned, so a team remote that sorts before `origin` binds the fork to the team's project (M8 again); an order fixed only in task 14, which this package's fake cannot see |
| `TestAForkSharesUnderItsOwnName` (b10 re-review I-2) | local project `portal`, the fork shape with `upstream` linked to `pj-2` (named `portal`, team `tm-1`): `DefaultName` and `Binding`'s `defaultName` are `portal (me/portal)`, and a bound binding's is `""`; `Bind(tm-1, "")` creates that name with `origin`'s key only and reads `bound`; `Bind(tm-1, "Portal fork")` uses `Portal fork`, and a name of white space alone takes the default (t07 mutant P2); with no Matches, `DefaultName` is `portal`; a 125-character local name gives a 120-character default ending in `(me/portal)`; characters are **code points** (re-review b10-2 R-M2): 125 `é` give 108 `é` and the key, valid UTF-8, and 120 `é` with no Matches; a key too long to leave the local name one code point (GitHub's 39 + 1 + 100 after its host) drops the qualifier, so the default is the local name alone and `NameTakenError` covers a clash | a fork's Share that reuses the directory name and is refused in the very team that holds the upstream's project |
| `TestANameTheTeamHoldsIsNameTaken` (b10 re-review I-2) | `failCreateProject` with a `*hubclient.Error` 409 `conflict` whose `details.field` is `name` → `*NameTakenError{Name: "portal (me/portal)"}` (Bind's re-read past the memo is still unbound), `Binding` still `unbound` with the same `Matches`, no memo change (no Hub call from the `Binding` after the `Bind`: review t07 I1 moved the mark there, because `Bind` itself now re-reads), its expiry included (the refusal comes 50 s into the memo, and a `Binding` 61 s after the memo was taken reads the Hub: fix round 1, mutant SB); the same refusal with the re-read failing (`SetUnavailable(true)`) → `hubclient.ErrUnavailable` as it came, never `NameTakenError`, and the memo unchanged; a later `Bind` with another name succeeds; the Hub fake's **own** rule gives the same (it enforces the per-team name, case-insensitive, 409 `conflict` `field: "name"`, `fake_projects.go:487-491` at hub `3df9fab8`); a 409 `conflict` with no `field`, and a 400 `bad_request` whose `field` is `name` (a length refusal; re-review b10-2 mutant 2), are returned as they came, not as `NameTakenError` | a name refusal relayed as a generic `conflict` the desktop cannot place, or cached so the retry never reaches the Hub |
| `TestBindingWithNoRemoteSaysSo` | `no_remote`, no Hub call | a Hub project created for a repository nobody else can reach (OQ-1) |
| `TestBindingOffWhenTheHubIsNotConfigured` | nil Hub → `hub_off`; `HeadRef` on `b-42` → a `none` Ref, nil states and a nil error (rule 6); `Inheritable` → `(Inheritable{}, nil)`, none and no error, the answer `HeadRef` reports (re-review b05-2 N-c); `View` → that binding, no brief; `Bind` and `History` → `ErrHubOff`; no Hub call | a nil dereference at the first lens open; a `HeadRef` error that task 12 would turn into a 503 on Develop's head reading for a Hub it was never given |
| `TestBindRaceReturnsTheTeammatesProject` | fake answers `repo_already_linked`; `Bind` returns that project `bound`; when the project holding the repository is in a team the caller is not in, the re-read stays `unbound` and `Bind` returns the Hub's `repo_already_linked` as it came, never an unbound `Binding` with a nil error (t07 mutant P1); `repo_already_linked` whose re-read fails (`SetUnavailable(true)` after the refusal) → `hubclient.ErrUnavailable` as it came, never the 409, and the memo unchanged (fix round 1, mutant SA); the commonest race (review t07 I1): local project `portal`, `origin` `github.com/team/portal2`, `Binding` memoised `unbound` with `defaultName` `portal`, then the teammate Shares that key under the same name `portal` in the same team → `Bind(team, "")` is `bound` to theirs, not `NameTakenError` (the fake checks the name before the repository, `fake_projects.go:487-495`), and `Binding` then reads theirs | a second project for one repository; a person told to choose another name for a repository their team already shares |
| `TestABindingReadInFlightDoesNotOutliveABind` (review t07 m2) | a `Binding` lookup reads `unbound`; before it returns (the fixture `Locator`'s `duringName` hook, run inside the lookup's last step), a `Bind` creates the project and forgets the memo → the in-flight call returns its `unbound`, and the next `Binding` is `bound`, never the stale `unbound` memoised for 60 s | a lookup that memoises what it read before a `Bind` it outlived |
| `TestInheritableIsTheBranchsLatestComplete` | `b-42` head 7 open, latest complete 6 → key `@6` | materialising an unfinished edit (C9) |
| `TestInheritableFallsBackToTheProject` | branch brief open with no complete revision → the project's latest complete key | a branch session that inherits nothing while the project has a brief |
| `TestInheritableIsNoneWithNoCompleteBrief` | key `""`, zero document | an empty block written as if it were a brief |
| `TestInheritableUsesThePinnedProjectRevision` | project head moved to 7 after `b-42@6` pinned `b-7@6`: the document's audience is rev 6's text | resolving against the project head — critique-2 §1.1 |
| `TestResolveBatchesInTwoCallsAtMost` | twelve keys, six branch revisions pinning three project revisions, all in one Hub project: the fake records ≤ 2 `Resolve` calls; with `Config.Evaluate` nil, no `List` call; with it set, exactly one `List` call (rule 8) | a call per key, or per inherited key (P8's cost, one layer down); a `List` per branch key, or one where nothing is evaluated |
| `TestResolveNeverSendsLocalForms` | `""`, `none:declined`, `b-42` → `none`, `none`, `invalid`, and the fake saw none of them | a local form leaking to the Hub, which answers `invalid` for `none:declined` |
| `TestResolveCachesOnlyResolved` | a `missing` key and a `resolved` one, each asked once while the Hub is healthy and again with `SetUnavailable(true)` → the `missing` one is `unavailable`, the `resolved` one `resolved` from the cache | a cached `missing` hiding a teammate's write |
| `TestResolveCarriesCachedKeysForTheirHeads` (re-review b06-1 R-I1; b06-2 R2-M1: the asserted head differs from the first answer's) | resolve `b-42@6` (pins `b-7@6`) → `heads["bf-…b7"]` `{6, 6}`; the fake writes `b-7@7` `complete`; resolve `b-42@6` again inside `CacheTTL` → the wrapper saw that call's one `/v1/refs` carry `b-42@6` and `b-7@6`, `heads["bf-…b7"]` is `{7, 7}` and `heads["bf-…b42"]` is present, and the `refs` slot is equal to the first answer's | a cache hit that costs the desktop its head, so a project edit never reaches a branch chip (rule 4 of task 16's selector reads `current` for ever) |
| `TestCriterionStatesAreRecomputedOnACachedKey` (review I14) | `Config.Evaluate` answers `open` on the first call and every criterion `settled` with `Because` `{"CI run 88 passed"}` on the second; `Resolve` the same resolved **branch** key twice (`Resolve` evaluates no project key, task 06), the second on a healthy Hub, so the key rides the first `/v1/refs` call for its head and rule 5 leaves the cached entry as it is → its `refs` slot is equal to the first, and `states[key]` changes from `open` to `settled`; a third with `SetUnavailable(true)` → the slot is still `resolved` from the cache, `states` has **no** entry for the key, and `Evaluate` is not called (rule 8: the binding cannot be read, the envelope ref is never the fallback, and "unknown" is not sent as `open`; task 06 fix rounds 1 and 2); a fourth on a healthy Hub, with 200 more branch briefs seeded after `b-42`'s (so `b-42` is on the walk's first page of two) and `failList(2, hubclient.ErrUnavailable)` failing the second page → no entry either, and `Evaluate` not called, never a verdict from the first page's partial map; a `missing`, `none` or `unavailable` slot has no `states` entry | a verdict cached with the revision, so evidence never moves it; a cached branch key evaluated at its envelope ref while the Hub is down; an unknown binding sent as all-`open`, which overwrites the desktop's `settled` for the whole outage (re-review 1 R1-I1); a walk that keeps its first page's map when a later page fails |
| `TestResolveEvaluatesABranchKeyWhereItsBriefIsBoundNow` (task 06 fix round 1, review I1; re-review b08-2 R2-M1; rule 8) | `Config.Evaluate` a recording fake. Rebind `b-42`'s brief from `refs/heads/evidence-upload` to `refs/heads/evidence-upload-v2` with the fake's `Rebind` (no revision written), then `Resolve([b-42@5])` → `resolved`, and `Evaluate` was called once, its query's `BranchRef` `refs/heads/evidence-upload-v2`. Create a new branch brief on `refs/heads/evidence-upload`, the old name reused, and resolve again → still `-v2`, never the new brief's ref. `Archive` `b-42`'s brief and resolve again → still `resolved`, `Evaluate` not called, and `states` **has** the key's entry, every reading `open` (the binding is known to be gone, unlike a failed `List`). `b-7@5` in the same batch → no `Evaluate` call for it. The fake's `Calls()` show one `List` per `Resolve`, for `b-42`'s Hub project, with `Scope` `branch` | `at = src.Revision.BranchRef` (R2-M1: after a rename the envelope names a ref that is gone, or someone else's branch); `at` from `ForBranch` of the envelope ref, which finds the new brief; an archived brief evaluated at a ref it no longer holds |
| `TestHeadRefServesCriterionStatesBesideTheRef` (re-review b06-1 R-I3) | `Config.Evaluate` answers every criterion `settled` with `Because` `{"CI run 88 passed"}` for `b-42@6`'s criteria; `HeadRef` on `b-42` → `states` has one `settled` reading per `Criterion` of the `Ref`, same ids and order, and the `Ref`'s JSON has no `state` key; `Evaluate` then answers `contested, "run 91 failed"` → a second `HeadRef` gives `contested` and a byte-equal `Ref`; a `none` reading has nil `states` | a `DefinitionOfDone` whose states depend on some session having stamped the same key, so a settled criterion reads `open` on every mount |
| `TestErasedIsNeverServedFromCache` | resolve, erase in the fake, resolve again inside `CacheTTL` → `erased`, no sections, and the entry is evicted (the Hub's answer wins); resolve, erase, `SetUnavailable(true)`, advance the injected clock past `CacheTTL`, resolve → `unavailable`, never `resolved` | erased text outliving its erasure on a laptop (ADR-H1 §6.8), or a cache that outranks the Hub's own `erased` |
| `TestABranchKeyWhosePinIsErasedKeepsOnlyItsOwnText` (re-review b06-1 R-I2; rule 7) | resolve `b-42@6` (pins `b-7@6`, audience skipped on the branch) so both are cached; erase `b-7` in the fake; resolve `b-42@6` again → `resolved`, `inherits` `{key: brief:…b7@6, erased: true}`, `audience` `""`, no section whose origin is `project` or with a `replaced`, and no byte of `b-7`'s own answers (those no `b-42` revision also holds) anywhere in the JSON; `HeadRef` on `b-42` gives the byte-equal `Ref`, and `Inheritable` gives `b-42@6` with `PinErased` true and a block with none of that text; a second `Service` that cached both before the erasure and meets it first through `Inheritable` (no `Resolve` after the erasure) gets `PinErased` true and no longer caches `b-7@6` (review t07 m1: the pin read evicts it, mutant S1) | erased project text served inside every branch key that pinned it, from the cache or from a rebuild |
| `TestViewOfABranchShowsWhatItInherits` | `Inherited` is `b-7`'s latest complete; `Preview.Block` equals `claudemd.Render` of `Inheritable`'s document | a preview the materialiser would not write |
| `TestHistoryIsOldestFirstWithChanges` | six rows, `Change.Kind` from each content, erased rows marked, and each erased row's key, cached by the earlier reads, gone from the cache (review t07 m1, mutant S2) | a history that has to diff revisions |
| `TestHubErrorsPassThroughUnchanged` | from `View`, `History` and `HeadRef`, a fake 409 and a fake `ErrUnavailable` reach the caller as `errors.As`/`errors.Is` match | a wrapped error the relay cannot see |
| `TestHubOffIsUnavailableNotMissing` (review I6; re-review b05-1 R-I1, R-M1) | a table of rows, each on a fresh `fakeHub`, hooks installed after the seeding calls. **Whole-call failures** — a nil `Config.Hub`; `SetUnavailable(true)`; `failResolve(1, &hubclient.Error{Status: 500})`; `SetSignedOut(true)` — over keys `[b-42@6 (it and its pin cached by an earlier, healthy call), b-42@5 (not cached), "", none:declined, "b-42"]` → `resolved`, `unavailable`, `none`, `none`, `invalid` (in the nil-Hub row nothing can have been cached, so the first is `unavailable` too). **Second call fails** — two branch revisions seeded for the row, each pinning a different project revision; one pin cached by an earlier healthy call, the other not; `failResolve(2, &hubclient.Error{Status: 500})`: the first call answers both branch revisions, the pin call fails → the branch key whose pin was cached is `resolved`, the other `unavailable`. **Branch cached, pin not** — resolve `b-7@6` at t0 and `b-42@6` at t0 + `CacheTTL`/2 (its second call carries the cached pin for its head, and leaves the pin's expiry at t0 + `CacheTTL`); advance the injected clock to t0 + `CacheTTL` + 1 s; `failResolve(1, hubclient.ErrUnavailable)` → `b-42@6` is `unavailable`, and the wrapper saw one `Resolve` call, carrying `b-42@6` and `b-7@6` (the cached branch revision's pin key, rule 5's first call). In every row: `Resolve` has no error to return, **no** slot is `missing`, and no `resolved` branch slot lacks its pinned project revision's sections. And a healthy fake that does not hold `bf-…40@5` answers it `missing` | rule 6's old `missing` for every key, which showed a Hub switched off as data loss (task 17's words); a Hub error returned from `Resolve`, which task 12 would turn into a non-200; a failed call that drops a cached slot; a branch slot called `resolved` with no pin in hand, the chip with no audience that rule 3 forbids (critique-2 §1.1); a status-specific branch that answers `missing` for a 500 |
| `TestAnExpiredPinIsFetchedAgain` (re-review b05-2 R2-I1, item 3) | the "branch cached, pin not" timeline with **no** hook → `b-42@6` `resolved` against `b-7@6`, and the fake's own `Calls()` show one `Resolve`, carrying `b-42@6` and `b-7@6` | a cached branch revision whose expired pin is never asked for again, so a healthy Hub answers it `unavailable` for up to one `CacheTTL`; a kill that rests on the wrapper's log alone |
| `TestRevisionCacheExpiresAndCaps` (task 07 as built) | a cache of 2: a third entry evicts the soonest to expire; a second `put` of a live key leaves its bytes and its expiry; an expired entry is gone; `evict` removes one | a cap that evicts at random or not at all, or a `put` that extends an expiry (rule 5: a pin's expiry is never stretched) |
| `TestRenameDoesNotLookLikeAHandEdit` (review I5) | render `Inheritable`'s document for `b-42` with `claudemd.Render` → block A. Rename the Hub project with the wrapper's `renameProject` (route 16, `PATCH /v1/projects/{id}`, which `hubclient.Projects` has no method for; `Engagement Portal` → `Evidence Portal`) and rebind `b-42`'s brief to `refs/heads/evidence-intake` with the fake's own `Rebind` (`PATCH /v1/briefs/{id}`, which writes no revision). The fixture `Locator` then answers `BranchName(b-42) = "evidence-intake"`, as the scanner would after the local branch is renamed; without that, `Inheritable` asks `ForBranch` for the old ref and the test proves nothing about the rebind. `Inheritable` on the rebound branch → the same key, `@6`, and a block **byte-equal** to A (title `# Branch brief — evidence-upload`, from rev 6's envelope); `claudemd.Inspect` on a file holding A, compared as task 09 compares, is not `edited`. The project key `b-7@6` renders byte-equal before and after too. Two guards that the test changed what it claims to: after the steps, and with the injected clock advanced 61 s past `Binding`'s 60 s memo, `Binding.HubProjectName` is `Evidence Portal`; and `ForBranch` was asked for `refs/heads/evidence-intake` (the fake's `Calls`), so a hook that did nothing fails the test instead of passing it | a title read from the Hub project's current name or the branch's current ref, which makes every worktree's block report `edited`, with a diff, after a rename |

**Steps:**

- [ ] **Step 1:** Confirm H0 has landed: `go doc ./control-plane/internal/hub/hubclient NewFake`.
  If not, stop — this task cannot start (§6 dependency). Then confirm HA4-2's **type** is there:
  `go doc ./control-plane/internal/hub/hubapi BriefQuery`. If it is absent, stop: the task is
  blocked by HA4-2 exactly as by no H0, because `Hub.List` cannot name the type and phase 4 may
  not write it (re-review b05-1 R-M3). Then read
  `go doc ./control-plane/internal/hub/hubclient Briefs`: if it already has HA4-2's `List`, write
  `fakehub_test.go` without its own `List`; if not, write the wrapper's `List`. Either way, write
  the wrapper's `renameProject`, `failResolve`, `failList` and `failCreateProject` hooks (H0's fake has none: its surface,
  `phase-h-hub-parts/task-04.md:188-226`, offers `SetUnavailable` for every call and no
  rename), and write nothing under `control-plane/internal/hub/` (review I11).
- [ ] **Step 2:** Reconcile part 07's assumed `hubapi` shapes with the real ones; record any
  rename in the commit message.
- [ ] **Step 3:** Write `fixtures_test.go` and the tests; confirm they fail.
- [ ] **Step 4:** Write `service.go`, `binding.go`, `read.go`. The cache is a map plus a
  min-heap on expiry under one mutex; no goroutine (G8).
- [ ] **Step 5:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverhub/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub
$ go list -deps ./control-plane/internal/discoverhub | grep "internal/hub/"
github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi
github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient
(and nothing else under internal/hub — the Hub contract §2 boundary)
$ grep -rn "os.WriteFile\|os.Create\|statepath" control-plane/internal/discoverhub/
(no output — nothing Hub-derived is persisted, ADR-H1 §5.4)
```

**Does NOT claim:** anything against a real Hub (task 23 runs `compose.yaml` if H3 has landed);
`githost` (phase 3's; task 14 wires it).

**As built (2026-09-27, `a8220c5a`, on `2cada669` = phase 2 + hub `3df9fab8`).**

1. Step 1: H0 is merged; `hubclient.Briefs` already declares HA4-2's `List`, and
   `hubapi.BriefQuery` exists. So `fakehub_test.go` has no `List` of its own: the fake's shows
   through. `Hub` still declares `List`, identically, so a signature change in the Hub is a
   compile error here.
2. Step 2, the assumed `hubapi` shapes against the real ones: `RefHead` is `HeadInfo`;
   `Remote.Host` and `RemoteRequest.Host` are `GitHost`; `Team.Role` is `Role`; digests are
   `hubapi.Digest` (`String()` gives the wire form); `RefResult.Kind` is `*KeyKind` and
   `Approval` is `*DesignApproval`; `Me` carries `Org`; `BriefQuery` carries `Page`;
   `BriefWithRevision` carries `LatestComplete` (HA4-4), which rule 3 now reads. `signin.ErrSignedOut`
   is `hubclient.ErrSignedOut` (no `signin` package exists).
3. The fixture's one pin choice (re-review b06-2 R2-M1): `b-42` revisions 1-5 pin `b-7@5`,
   `b-42@6` pins `b-7@6`. Every row above that named `b-7@5` as `b-42@6`'s pin now says `b-7@6`.
   The fake mints its own ids, so `b-7` and `b-42` are whatever `ids.Wide` gave.
4. Departures, each the root-cause option:
   - `HeadRef` answers `none` with no error for an **unbound** or **no_remote** project, as it
     does for a nil Hub: a session started now inherits nothing, and a head reading that errors
     would block `DefinitionOfDone` for a project nobody shared. `Inheritable` still returns
     `ErrNotBound` and `ErrNoRemote`, which task 09 maps to nothing.
   - `ErrPinUnreadable` (new): a pin the Hub answers `missing`, from `Inheritable`, `HeadRef` and
     `View` (whose `Preview` needs the pin). Carried to task 09 (the Materialiser), task 11 (the
     routes' error table) and task 12 (the head reading), none of which has a row for it yet, so
     it would reach the desktop as a 500 (review t07 m3).
   - `Inheritable.PinErased` (new; re-review b06-2 R2-I1, this task's half). Task 09's `Inspect`
     half, and OQ-9's note that a materialised file keeps the erased text until the next start
     rewrites it, are task 09's.
   - A branch `Target` whose `BranchID` the `Locator` does not know is an error
     (`errUnknownBranch`), never read as the project scope.
   - A design key rides the first `/v1/refs` call and gets the Hub's resolution with phase 4's
     fields empty (task 12's "Does NOT claim"); it is never cached.
   - `Config`: `Repos` and `Locate` are required; `Now`, `NewID`, `Log` and `CacheTTL` default.
   - The project scope's brief is read with `List` (scope project, limit 1) and then `Brief`;
     `ForBranch` needs a ref.
   - A write through the fork's binding (`TestAnotherRemotesProjectIsAChoiceNotABinding`) is task
     08's; the row checks a read, `Inheritable`, which is `ErrNotBound` by the same `Binding`.
5. Fix round 1 (review t07, 2026-09-27):
   - I1: `Bind` re-reads the binding past the memo on a name refusal too, not only on
     `repo_already_linked`, because a teammate whose clone has the same directory name Shares
     under the same default name, and the Hub may check the name first. A bound re-read wins; an
     unbound one leaves the memo and answers `NameTakenError`; a re-read that fails returns its
     own error. The `Bind` doc comment and two rows above say so.
   - m2: `forget` bumps a per-project generation, and `Binding` memoises a lookup only while the
     generation is what it was when the lookup began (`TestABindingReadInFlightDoesNotOutliveABind`).
   - m1: the erased-pin and History rows now assert the cache, killing mutants S1 and S2.
   - The fix round's own predicted survivors: SA (`repo_already_linked` with a failed re-read
     returns the 409, not the re-read's error) survived and SB (a name refusal re-memoises its
     unbound re-read, so the memo's expiry moves) could not be judged (vet.exe crashed, exit
     0xc0000142). Each now has a row: the `TestBindRace…` and `TestANameTheTeamHolds…` rows above.
   - m5, m6: the contract blocks follow the code (the `Hub` comment, **Files**, `orderRemotes` in
     `binding.go`, `HeadRef`'s unnamed results).
