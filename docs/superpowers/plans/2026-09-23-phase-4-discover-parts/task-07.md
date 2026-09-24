### Task 07: `discoverhub` — binding and reads, against the Hub's fake

The control plane's side of `OD-3`. This package is the **only** phase 4 code that talks to the
Hub, and it talks only through `hubclient.Briefs` and `hubclient.Projects` (Hub contract §9.2). It
turns local ids (`p-…`, `b-…`, path-hashed, never shared — ADR-H1 §1.5) into Hub ids (`pj-…`,
`refs/heads/…`), reads briefs, resolves keys, and answers the Materialiser's one question: *what
does a session on this branch inherit now?* Every test runs against `hubclient.NewFake()`, through
the phase-4-owned wrapper in `fakehub_test.go` that adds `List` (HA4-2, below).

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
  adds `List` over the revisions the test seeded, and adds the two hooks named in **Files** below
  (`renameProject`, `failResolve`). It lives in `discoverhub`, not in the Hub's package.
- When HA4-2 lands, the wrapper's `List` is deleted and the embedded fake's own `List` shows
  through; `Hub` stays as it is, and so do the two hooks. That is a one-file change, and the
  unchanged tests prove it.

The orchestrator carries HA4-1..9 to the Hub planner (plan §9); this task does not wait on the
reply to start, but it cannot claim the real Hub until HA4-2 is in the Hub contract.

**Files:**
- `control-plane/internal/discoverhub/service.go` — `Config`, `Service`, `New`, the cache (new)
- `control-plane/internal/discoverhub/binding.go` + `binding_test.go` (new)
- `control-plane/internal/discoverhub/read.go` + `read_test.go` (new)
- `control-plane/internal/discoverhub/fixtures_test.go` — the fact sheet's world in the fake (new)
- `control-plane/internal/discoverhub/fakehub_test.go` — `fakeHub{*hubclient.Fake}` plus `List`
  over the seeded revisions, until HA4-2 lands, and two test hooks H0's fake does not plan
  (`phase-h-hub-parts/task-04.md:188-226` lists its whole surface, and neither is in it) (new;
  phase 4's, not the Hub's):
  - `renameProject(id, name string)` — from then on the wrapper's `Project` and `ProjectForRepo`
    return the embedded fake's answer with `Name` replaced. It stands in for route 16,
    `PATCH /v1/projects/{id}`, which `hubclient.Projects` has no method for.
  - `failResolve(n int, err error)` — the n-th `Resolve` call made through the wrapper (1-based,
    counted from the hook) returns `err` without reaching the embedded fake; every other call
    passes through. It gives a chosen `*hubclient.Error` status and a failure on the second call
    only; `SetUnavailable` fails every call.

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
// interface: List is declared here, not borrowed, because HA4-2 has not
// reached the Hub contract. Until it does, only the test wrapper
// (fakehub_test.go: hubclient.NewFake() embedded, plus List) satisfies Hub;
// *hubclient.Client and *hubclient.Fake satisfy it unchanged once HA4-2
// lands. Nothing in this package writes under internal/hub.
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

// Resolve answers every key, in order, in at most two Hub calls. The first
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
// states holds provenance.Readings(ctx, key, src, Config.Evaluate) for every
// resolved slot, cached revision or not, and nothing for any other slot:
// criterion state moves, so it is computed on every call and is never part
// of the cached Ref (review I14).
// There is no error result: a Hub failure, a signed-out user or a nil Hub
// makes each slot the cache cannot answer "unavailable" (rule 5, review I6).
func (s *Service) Resolve(ctx context.Context, keys []string) (refs []provenance.Ref, heads map[string]provenance.Head, states map[string][]provenance.CriterionReading)

// HeadRef is the Ref of what a session on t would inherit now - the head
// reading for DefinitionOfDone and Discover (contract §3.1). resolution
// "none" when nothing would be inherited. states is
// provenance.Readings(ctx, ref.Key, src, Config.Evaluate) when the Ref is
// resolved, and nil otherwise: the moving part served beside the Ref, never
// inside it, computed on every call like Resolve's (review I14). It is the
// one surface that shows criterion states, so it must not depend on some
// session having stamped the same key (re-review b06-1 R-I3).
func (s *Service) HeadRef(ctx context.Context, t Target) (ref provenance.Ref, states []provenance.CriterionReading, err error)

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
2. **Titles come from the revision, never from a name** (review I5, task 05's `Document.Title`):
   every `discover.Revision` this package builds carries the envelope's own `branchRef`, and
   `discover.Resolve` (through `discover.Title`, the rule's one copy) titles a branch key
   `Branch brief — <branchRef minus refs/heads/>` and a
   project key `Project brief`. Neither the Hub project's current name (`PATCH /v1/projects/{id}`)
   nor the branch's current ref (`Rebind`) is read for a render, so a key renders the same bytes on
   every teammate's machine, before and after a rename (P4-G).
3. `Inheritable` reads `ForBranch` once; the latest complete revision's content comes from the head
   revision when `headRevision == latestCompleteRevision`, else from one `/v1/refs` call (HA4-4
   would remove it). A branch revision's pinned project revision is fetched by its `inherits.key`,
   **never** by the project's current head.
4. The cache holds `resolved` revisions only, keyed by key, for `CacheTTL`. `missing` and `erased`
   are never cached: a missing key may appear (a teammate's write not yet visible), and an erased
   one must not keep content.
5. **One rule for the Hub failing during `Resolve`** (review I6; task 12 rule 1 restates it).
   `Resolve` **never returns a Hub error** — it has no error result. Slot by slot:
   - a slot is `resolved` only when **everything it needs is in hand**, each piece from the cache
     or from a call that succeeded. A project key needs its revision. A branch key needs its
     revision **and** its pinned project revision (rule 3; `provenance.Build` reads both, task 06).
     A key whose pieces are all cached stays `resolved`, whatever the Hub is doing;
   - every other brief key is `unavailable` when the Hub call that would answer it fails for any
     reason (`*hubclient.Error` of any status, `hubclient.ErrUnavailable`, a transport error or
     the route's deadline), when it returns `entraauth.ErrSignedOut`, and when `Config.Hub` is nil
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
   `entraauth.ErrSignedOut`): the relay is task 11's.
6. A nil `Config.Hub` makes `Binding` answer `hub_off`, `View` answer a view with that binding and
   no brief, `Resolve` answer `unavailable` for every brief key (rule 5; **not** `missing`),
   `Inheritable` answer none, and `HeadRef` answer a `none` Ref with **no error** — the same
   answer as `Inheritable`, whose answer it reports, and treated like `Binding`'s `hub_off`, which
   is a 200 view and not an error (re-review b05-1 N2). So a Hub that is not configured is not task 12 rule 2's
   "cannot be reached": the head reading is 200 `none`, never 503, because a session started now
   inherits nothing (task 09 step 4) and Develop is not to be blocked or toasted for a Hub it was
   never given (Hub contract §9.5; task 18's `hub_off leaves Develop alone`). A stamped key is
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

**Tests first** (`fakeHub` from `fakehub_test.go` — `hubclient.NewFake()` embedded, plus `List`
until HA4-2 lands, plus the `renameProject` and `failResolve` hooks — seeded by `fixtures_test.go` with the fact sheet's world —
project `Engagement Portal`, `b-7` revs 1–6, `b-42` revs 1–6 with the pins of fact sheet §2–§3):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBindingFindsTheProjectByItsRepoKey` | a local project whose second remote is linked is `bound` to it | a lookup of origin only |
| `TestBindingWithNoRemoteSaysSo` | `no_remote`, no Hub call | a Hub project created for a repository nobody else can reach (OQ-1) |
| `TestBindingOffWhenTheHubIsNotConfigured` | nil Hub → `hub_off`; `HeadRef` on `b-42` → a `none` Ref and a nil error (rule 6) | a nil dereference at the first lens open; a `HeadRef` error that task 12 would turn into a 503 on Develop's head reading for a Hub it was never given |
| `TestBindRaceReturnsTheTeammatesProject` | fake answers `repo_already_linked`; `Bind` returns that project `bound` | a second project for one repository |
| `TestInheritableIsTheBranchsLatestComplete` | `b-42` head 7 open, latest complete 6 → key `@6` | materialising an unfinished edit (C9) |
| `TestInheritableFallsBackToTheProject` | branch brief open with no complete revision → the project's latest complete key | a branch session that inherits nothing while the project has a brief |
| `TestInheritableIsNoneWithNoCompleteBrief` | key `""`, zero document | an empty block written as if it were a brief |
| `TestInheritableUsesThePinnedProjectRevision` | project head moved to 7 after `b-42@6` pinned `b-7@6`: the document's audience is rev 6's text | resolving against the project head — critique-2 §1.1 |
| `TestResolveBatchesInTwoCallsAtMost` | twelve keys, six branch revisions pinning three project revisions: the fake records ≤ 2 `Resolve` calls | a call per key, or per inherited key (P8's cost, one layer down) |
| `TestResolveNeverSendsLocalForms` | `""`, `none:declined`, `b-42` → `none`, `none`, `invalid`, and the fake saw none of them | a local form leaking to the Hub, which answers `invalid` for `none:declined` |
| `TestResolveCachesOnlyResolved` | a `missing` key and a `resolved` one, each asked once while the Hub is healthy and again with `SetUnavailable(true)` → the `missing` one is `unavailable`, the `resolved` one `resolved` from the cache | a cached `missing` hiding a teammate's write |
| `TestResolveCarriesCachedKeysForTheirHeads` (re-review b06-1 R-I1) | resolve `b-42@6` (pins `b-7@5`); the fake writes `b-7@6` `complete`; resolve `b-42@6` again inside `CacheTTL` → the wrapper saw the first call carry `b-42@6` and `b-7@5`, `heads["bf-…b7"]` is `{6, 6}` and `heads["bf-…b42"]` is present, and the `refs` slot is equal to the first answer's | a cache hit that costs the desktop its head, so a project edit never reaches a branch chip (rule 4 of task 16's selector reads `current` for ever) |
| `TestCriterionStatesAreRecomputedOnACachedKey` (review I14) | `Config.Evaluate` answers `open` on the first call and every criterion `settled` with `Because` `{"CI run 88 passed"}` on the second; `Resolve` the same resolved **branch** key twice (a project key is never evaluated, task 06), the second with `SetUnavailable(true)` → its `refs` slot is `resolved` from the cache and equal to the first, and `states[key]` changes from `open` to `settled`; a `missing`, `none` or `unavailable` slot has no `states` entry | a verdict cached with the revision, so evidence never moves it |
| `TestHeadRefServesCriterionStatesBesideTheRef` (re-review b06-1 R-I3) | `Config.Evaluate` answers every criterion `settled` with `Because` `{"CI run 88 passed"}` for `b-42@6`'s criteria; `HeadRef` on `b-42` → `states` has one `settled` reading per `Criterion` of the `Ref`, same ids and order, and the `Ref`'s JSON has no `state` key; `Evaluate` then answers `contested, "run 91 failed"` → a second `HeadRef` gives `contested` and a byte-equal `Ref`; a `none` reading has nil `states` | a `DefinitionOfDone` whose states depend on some session having stamped the same key, so a settled criterion reads `open` on every mount |
| `TestErasedIsNeverServedFromCache` | resolve, erase in the fake, resolve again inside `CacheTTL` → `erased`, no sections, and the entry is evicted (the Hub's answer wins); resolve, erase, `SetUnavailable(true)`, advance the injected clock past `CacheTTL`, resolve → `unavailable`, never `resolved` | erased text outliving its erasure on a laptop (ADR-H1 §6.8), or a cache that outranks the Hub's own `erased` |
| `TestABranchKeyWhosePinIsErasedKeepsOnlyItsOwnText` (re-review b06-1 R-I2; rule 7) | resolve `b-42@6` (pins `b-7@5`, audience skipped on the branch) so both are cached; erase `b-7` in the fake; resolve `b-42@6` again → `resolved`, `inherits` `{key: brief:…b7@5, erased: true}`, `audience` `""`, no section whose origin is `project`, and no byte of `b-7@5`'s answers anywhere in the JSON; `HeadRef` on `b-42` and `Inheritable` agree | erased project text served inside every branch key that pinned it, from the cache or from a rebuild |
| `TestViewOfABranchShowsWhatItInherits` | `Inherited` is `b-7`'s latest complete; `Preview.Block` equals `claudemd.Render` of `Inheritable`'s document | a preview the materialiser would not write |
| `TestHistoryIsOldestFirstWithChanges` | six rows, `Change.Kind` from each content, erased rows marked | a history that has to diff revisions |
| `TestHubErrorsPassThroughUnchanged` | from `View`, `History` and `HeadRef`, a fake 409 and a fake `ErrUnavailable` reach the caller as `errors.As`/`errors.Is` match | a wrapped error the relay cannot see |
| `TestHubOffIsUnavailableNotMissing` (review I6; re-review b05-1 R-I1, R-M1) | a table of rows, each on a fresh `fakeHub`, hooks installed after the seeding calls. **Whole-call failures** — a nil `Config.Hub`; `SetUnavailable(true)`; `failResolve(1, &hubclient.Error{Status: 500})`; `SetSignedOut(true)` — over keys `[b-42@6 (it and its pin cached by an earlier, healthy call), b-42@5 (not cached), "", none:declined, "b-42"]` → `resolved`, `unavailable`, `none`, `none`, `invalid` (in the nil-Hub row nothing can have been cached, so the first is `unavailable` too). **Second call fails** — two branch revisions seeded for the row, each pinning a different project revision; one pin cached by an earlier healthy call, the other not; `failResolve(2, &hubclient.Error{Status: 500})`: the first call answers both branch revisions, the pin call fails → the branch key whose pin was cached is `resolved`, the other `unavailable`. **Branch cached, pin not** — resolve `b-7@6` at t0 and `b-42@6` at t0 + `CacheTTL`/2 (its second call carries the cached pin for its head, and leaves the pin's expiry at t0 + `CacheTTL`); advance the injected clock to t0 + `CacheTTL` + 1 s; `failResolve(1, hubclient.ErrUnavailable)` → `b-42@6` is `unavailable`, and the wrapper saw one `Resolve` call, carrying `b-42@6` and `b-7@6` (the cached branch revision's pin key, rule 5's first call). In every row: `Resolve` has no error to return, **no** slot is `missing`, and no `resolved` branch slot lacks its pinned project revision's sections. And a healthy fake that does not hold `bf-…40@5` answers it `missing` | rule 6's old `missing` for every key, which showed a Hub switched off as data loss (task 17's words); a Hub error returned from `Resolve`, which task 12 would turn into a non-200; a failed call that drops a cached slot; a branch slot called `resolved` with no pin in hand, the chip with no audience that rule 3 forbids (critique-2 §1.1); a status-specific branch that answers `missing` for a 500 |
| `TestRenameDoesNotLookLikeAHandEdit` (review I5) | render `Inheritable`'s document for `b-42` with `claudemd.Render` → block A. Rename the Hub project with the wrapper's `renameProject` (route 16, `PATCH /v1/projects/{id}`, which `hubclient.Projects` has no method for; `Engagement Portal` → `Evidence Portal`) and rebind `b-42`'s brief to `refs/heads/evidence-intake` with the fake's own `Rebind` (`PATCH /v1/briefs/{id}`, which writes no revision). The fixture `Locator` then answers `BranchName(b-42) = "evidence-intake"`, as the scanner would after the local branch is renamed; without that, `Inheritable` asks `ForBranch` for the old ref and the test proves nothing about the rebind. `Inheritable` on the rebound branch → the same key, `@6`, and a block **byte-equal** to A (title `# Branch brief — evidence-upload`, from rev 6's envelope); `claudemd.Inspect` on a file holding A, compared as task 09 compares, is not `edited`. The project key `b-7@6` renders byte-equal before and after too. Two guards that the test changed what it claims to: after the steps, and with the injected clock advanced 61 s past `Binding`'s 60 s memo, `Binding.HubProjectName` is `Evidence Portal`; and `ForBranch` was asked for `refs/heads/evidence-intake` (the fake's `Calls`), so a hook that did nothing fails the test instead of passing it | a title read from the Hub project's current name or the branch's current ref, which makes every worktree's block report `edited`, with a diff, after a rename |

**Steps:**

- [ ] **Step 1:** Confirm H0 has landed: `go doc ./control-plane/internal/hub/hubclient NewFake`.
  If not, stop — this task cannot start (§6 dependency). Then confirm HA4-2's **type** is there:
  `go doc ./control-plane/internal/hub/hubapi BriefQuery`. If it is absent, stop: the task is
  blocked by HA4-2 exactly as by no H0, because `Hub.List` cannot name the type and phase 4 may
  not write it (re-review b05-1 R-M3). Then read
  `go doc ./control-plane/internal/hub/hubclient Briefs`: if it already has HA4-2's `List`, write
  `fakehub_test.go` without its own `List`; if not, write the wrapper's `List`. Either way, write
  the wrapper's `renameProject` and `failResolve` hooks (H0's fake has neither: its surface,
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
