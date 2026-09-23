### Task 07: `discoverhub` — binding and reads, against the Hub's fake

The control plane's side of `OD-3`. This package is the **only** phase 4 code that talks to the
Hub, and it talks only through `hubclient.Briefs` and `hubclient.Projects` (Hub contract §9.2). It
turns local ids (`p-…`, `b-…`, path-hashed, never shared — ADR-H1 §1.5) into Hub ids (`pj-…`,
`refs/heads/…`), reads briefs, resolves keys, and answers the Materialiser's one question: *what
does a session on this branch inherit now?* Every test runs against `hubclient.NewFake()`.

**Depends on the Hub's task H0** (interfaces, fake, conformance skeleton — ADR-H1 §5.9 ships them
first) and on Hub contract ask **HA4-2** (`Briefs.List`). If `List` has not landed when this task
starts, add it to the fake in the Hub's package **as a separate commit named as a Hub ask** — do
not reach into `hubclient` silently.

**Files:**
- `control-plane/internal/discoverhub/service.go` — `Config`, `Service`, `New`, the cache (new)
- `control-plane/internal/discoverhub/binding.go` + `binding_test.go` (new)
- `control-plane/internal/discoverhub/read.go` + `read_test.go` (new)
- `control-plane/internal/discoverhub/fixtures_test.go` — the fact sheet's world in the fake (new)

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
type BriefQuery struct { // HA4-2
	Scope           string
	BranchRef       string
	IncludeArchived bool
	After           string
	Limit           int
}
```

**Interfaces — complete source** (`discoverhub`):

```go
// control-plane/internal/discoverhub/service.go

// Package discoverhub is the control plane's Hub-backed brief service. It is
// the only phase 4 package that calls the Hub, and it calls it as the
// signed-in user through hubclient (ADR-H1 §5.4). It persists nothing: the
// only state it keeps is an in-memory cache of immutable revisions.
package discoverhub

// Hub is the part of hubclient this package uses. *hubclient.Client and
// *hubclient.Fake both satisfy it.
type Hub interface {
	hubclient.Briefs
	hubclient.Projects
	List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) // HA4-2
}

// RepoKey is one remote of a local repository, in the Hub's form.
type RepoKey struct {
	Key  string // githost.Remote.RepoKey() (phase 3 task 13)
	Host string // "github" | "azure-devops"
}

// RepoIdentity lists a local repository's recognised remotes, origin first,
// then the rest by name. Unrecognised hosts are left out, not errors.
// Task 14 wires it over phase 3's githost.ParseRemote and a read-only
// `git remote` / `git remote get-url` through project.Runner.
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
)
```

```go
// control-plane/internal/discoverhub/binding.go

type BindingState string

const (
	BindingBound    BindingState = "bound"
	BindingUnbound  BindingState = "unbound"   // it has remotes; no Hub project has any of them
	BindingNoRemote BindingState = "no_remote" // OQ-1
	BindingHubOff   BindingState = "hub_off"
)

type TeamChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // the caller's; Bind needs member or above
}

type Binding struct {
	State          BindingState `json:"state"`
	HubProjectID   string       `json:"hubProjectId"`   // "" unless bound
	HubProjectName string       `json:"hubProjectName"` // "" unless bound
	RepoKeys       []string     `json:"repoKeys"`       // what was looked up, in order
	Teams          []TeamChoice `json:"teams"`          // unbound only: where it could be created
}

// Binding looks the project up by each repo key in order and returns the
// first Hub project found. It is memoised in memory per local project for
// 60 s, and forgotten on Bind.
func (s *Service) Binding(ctx context.Context, projectID string) (Binding, error)

// Bind creates the Hub project in teamID with every repo key as a remote,
// named after the local project. repo_already_linked (a race with a
// teammate) re-reads and returns that binding instead of failing.
func (s *Service) Bind(ctx context.Context, projectID, teamID string) (Binding, error)

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

// Resolve answers every key, in order, in at most two Hub calls: one for the
// keys not in the cache, and one for the project revisions the branch
// revisions among them inherit. "", none:declined and invalid keys never
// reach the Hub. heads holds one entry per brief named.
func (s *Service) Resolve(ctx context.Context, keys []string) (refs []provenance.Ref, heads map[string]provenance.Head, err error)

// HeadRef is the Ref of what a session on t would inherit now - the head
// reading for DefinitionOfDone and Discover (contract §3.1). resolution
// "none" when nothing would be inherited.
func (s *Service) HeadRef(ctx context.Context, t Target) (provenance.Ref, error)

// Inheritable is P4-A, the Materialiser's one question: the latest complete
// branch revision, else the latest complete project revision, else none.
type Inheritable struct {
	Key      string            // "" when none
	Document discover.Document // what claudemd.Render writes; zero when none
}

func (s *Service) Inheritable(ctx context.Context, t Target) (Inheritable, error)
```

**The rules, each a test below.**

1. Local ids never reach the Hub; Hub ids never reach `Locator`.
2. **Titles use the Hub's names**: `Hub project name — branch name` (the ref minus `refs/heads/`),
   so a key renders the same first line on every teammate's machine (P4-G).
3. `Inheritable` reads `ForBranch` once; the latest complete revision's content comes from the head
   revision when `headRevision == latestCompleteRevision`, else from one `/v1/refs` call (HA4-4
   would remove it). A branch revision's pinned project revision is fetched by its `inherits.key`,
   **never** by the project's current head.
4. The cache holds `resolved` revisions only, keyed by key, for `CacheTTL`. `missing` and `erased`
   are never cached: a missing key may appear (a teammate's write not yet visible), and an erased
   one must not keep content.
5. Every Hub error reaches the caller **unchanged** (`*hubclient.Error`, `hubclient.ErrUnavailable`,
   `entraauth.ErrSignedOut`): the relay is task 11's.
6. A nil `Config.Hub` makes `Binding` answer `hub_off`, `View` answer a view with that binding and
   no brief, `Resolve` answer `missing` for every brief key, and `Inheritable` answer none.

**Tests first** (`hubclient.NewFake()` seeded by `fixtures_test.go` with the fact sheet's world —
project `Engagement Portal`, `b-7` revs 1–6, `b-42` revs 1–6 with the pins of fact sheet §2–§3):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBindingFindsTheProjectByItsRepoKey` | a local project whose second remote is linked is `bound` to it | a lookup of origin only |
| `TestBindingWithNoRemoteSaysSo` | `no_remote`, no Hub call | a Hub project created for a repository nobody else can reach (OQ-1) |
| `TestBindingOffWhenTheHubIsNotConfigured` | nil Hub → `hub_off` | a nil dereference at the first lens open |
| `TestBindRaceReturnsTheTeammatesProject` | fake answers `repo_already_linked`; `Bind` returns that project `bound` | a second project for one repository |
| `TestInheritableIsTheBranchsLatestComplete` | `b-42` head 7 open, latest complete 6 → key `@6` | materialising an unfinished edit (C9) |
| `TestInheritableFallsBackToTheProject` | branch brief open with no complete revision → the project's latest complete key | a branch session that inherits nothing while the project has a brief |
| `TestInheritableIsNoneWithNoCompleteBrief` | key `""`, zero document | an empty block written as if it were a brief |
| `TestInheritableUsesThePinnedProjectRevision` | project head moved to 7 after `b-42@6` pinned `b-7@6`: the document's audience is rev 6's text | resolving against the project head — critique-2 §1.1 |
| `TestResolveBatchesInTwoCallsAtMost` | twelve keys, six branch revisions pinning three project revisions: the fake records ≤ 2 `Resolve` calls | a call per key, or per inherited key (P8's cost, one layer down) |
| `TestResolveNeverSendsLocalForms` | `""`, `none:declined`, `b-42` → `none`, `none`, `invalid`, and the fake saw none of them | a local form leaking to the Hub, which answers `invalid` for `none:declined` |
| `TestResolveCachesOnlyResolved` | a `missing` key asked twice reaches the Hub twice; a `resolved` one once | a cached `missing` hiding a teammate's write |
| `TestErasedIsNeverServedFromCache` | resolve, erase in the fake, advance the injected clock past `CacheTTL`, resolve: `erased`, no sections | erased text outliving its erasure on a laptop (ADR-H1 §6.8) |
| `TestViewOfABranchShowsWhatItInherits` | `Inherited` is `b-7`'s latest complete; `Preview.Block` equals `claudemd.Render` of `Inheritable`'s document | a preview the materialiser would not write |
| `TestHistoryIsOldestFirstWithChanges` | six rows, `Change.Kind` from each content, erased rows marked | a history that has to diff revisions |
| `TestHubErrorsPassThroughUnchanged` | a fake 409 and a fake `ErrUnavailable` reach the caller as `errors.As`/`errors.Is` match | a wrapped error the relay cannot see |

**Steps:**

- [ ] **Step 1:** Confirm H0 has landed: `go doc ./control-plane/internal/hub/hubclient NewFake`.
  If not, stop — this task cannot start (§6 dependency).
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
