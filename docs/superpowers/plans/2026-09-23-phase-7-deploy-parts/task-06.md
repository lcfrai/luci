### Task 06: The binding — which Panacea project deploys this repository

**Half:** 7a. **Hours:** 2. **Depends on:** 03, 05. **Elsewhere:** phase 3 task 13
(`githost.ParseRemote`, `Remote.RepoKey()`) — **hard, no fallback**: this task does not start
until it is merged, and there is no by-name binding to fall back on (plan §10.1, review M15);
Hub H0 (`hubclient.Projects` and its fake).

Plan decision 7P-6. The owner's bar is *"powerful but a wall of configuration has failed"*, so the
common case — one Panacea project deploys this repository — binds with **no question asked**. A
local project is identified three ways, and each has one owner: the local `p-…` id (phase 2,
`project.go:29`, a hash of a path on one laptop), the **repository key** (phase 3's `githost`, the
same for every teammate, both hosts — `OD-4`), and the **Hub project** (`pj-…`, found by repository
key, Hub contract §7.4). The Panacea project is found by matching repository keys too.

**Files:**
- `control-plane/internal/deploy/binding/binding.go` (new) — `Remotes`, `HubProjects`, `Config`, `Resolver`, `Binding`, `State`
- `control-plane/internal/deploy/binding/choices.go` (new) — the local choice file
- `control-plane/internal/deploy/binding/binding_test.go`, `choices_test.go` (new)
- `control-plane/internal/api/deploy_binding.go` (new) — `PUT /v1/projects/{projectId}/deploy/binding`
- `control-plane/internal/api/deploy_binding_test.go` (new)
- `control-plane/internal/api/api.go`, `control-plane/cmd/api/main.go` (modified) — `Deps.DeployBinding` (the one `*binding.Resolver`; the watcher, task 07, and tasks 09, 11 and 12 read the same one) and `a.registerDeployBinding(guarded)` in `New`; `main.go` builds the resolver over phase 3's `gitx` remotes and `hubclient.Projects`
- No `statepath` change: task 05 does ruling R2's lift, if phase 4 has not, because its origin file is the first writer (review M10)

#### Interfaces — complete

```go
// Package binding decides which deploy-target project deploys a local project, by repository.
package binding

// Remotes reads a local project's origin URL. main.go satisfies it with phase 3's gitx:
// `git -C <path> remote get-url origin`. "" with a nil error when there is no origin.
type Remotes interface {
	OriginURL(ctx context.Context, projectPath string) (string, error)
}

// HubProjects is the one hubclient.Projects method this package needs (Hub contract §9.2).
type HubProjects interface {
	ProjectForRepo(ctx context.Context, repoKey string) (hubapi.Project, bool, error)
}

type State string

const (
	Bound          State = "bound"
	NoOrigin       State = "no_origin"       // the local repository has no origin remote
	UnknownHost    State = "unknown_host"    // origin is neither GitHub nor Azure DevOps (githost.ErrUnknownHost)
	NoMatch        State = "no_match"        // no Panacea project this key can see deploys this repository (a restricted key sees one)
	Ambiguous      State = "ambiguous"       // several do; the user chooses once
	NoRepoInfo     State = "no_repo_info"    // Panacea reported no repoUrl for any project; the user chooses from all
	NotConnected   State = "not_connected"   // connection.Manager has no working credential
	Unavailable    State = "unavailable"     // Panacea or the Hub could not be reached; the last binding is kept
)

// Binding is one local project's answer. It is recomputed, never persisted, except for a user's
// choice between candidates (choices.go).
type Binding struct {
	ProjectID    string            // local "p-…"
	RepoKey      string            // githost.Remote.RepoKey(); "" in NoOrigin and UnknownHost
	HubProjectID string            // "pj-…"; "" when the repository is not a Hub project — reads work, Ship does not.
	//                                The fix is the Hub's share (Hub task 35's POST …/hub-link, task 42's screen;
	//                                task 14 links to it, review M17); the next resolve after its project.* event
	//                                picks the new id up (task 07 HubEvents)
	// HubMiss says why HubProjectID is "". FOUR values (amended, carried b05-2 N-1 and N-2):
	//   ""             — HubProjectID is set; or no Hub is configured; or the Hub has never
	//                    answered for this key (a failure on the first read). The binding does
	//                    NOT tell the last two apart and must not pretend to: the WATCHER does,
	//                    because it knows whether a Hub is configured, and reports releasesState
	//                    no_hub or unavailable (task 07). Task 09's readiness therefore takes
	//                    that cause from the view's releasesState, NOT from HubMiss — the
	//                    original text here said readiness told them apart, and it cannot.
	//   "not_linked"   — the Hub answered: no project it lets this caller read has this key.
	//   "key_spelling" — the key is in no form the Hub accepts, so no Hub project can ever hold
	//                    it (HA7-6 is the Azure DevOps case; a GitHub EMU owner with "_" is
	//                    another). Set two ways: the Hub answered no AND
	//                    hubapi.HostOfRepoKey(RepoKey) is false; OR the Hub ANSWERED 400 with
	//                    field "repo" (Hub task 17 route 13, task-17.md:23), which is the Hub
	//                    itself refusing the key and outranks any local check.
	//   "signed_out"   — the Hub refused for want of a session. It exists because this is the one
	//                    cause the watcher cannot infer: a configured, reachable Hub that refuses
	//                    a signed-out caller looks exactly like an outage, and the Ship bar would
	//                    say "The Hub is not answering" instead of "Sign in first."
	// A Hub error that is a BLIP keeps the previous HubMiss together with the previous
	// HubProjectID (step 3). A 400 field: repo is not a blip; it is an answer.
	// json:"-": HubMiss is never on the binding wire (below); TestHubKeyIsRepoKeyVerbatim pins it.
	HubMiss      string `json:"-"`
	Panacea      deploy.ProjectRef // "" unless Bound
	State        State
	ChosenBy     string            // "auto" | "local" | "" (HA7-3 adds "hub")
	Candidates   []deploy.Project  // Ambiguous and NoRepoInfo only; the Panacea projects to choose from
	Message      string            // one sentence for the lens, for every State but Bound
}

// LocalProjects is the one phase 2 method this package needs. *project.Scanner satisfies it, as
// it satisfies api.Enumerator; declaring it here keeps binding from importing api (api imports
// binding for the route — the other direction would be a cycle).
type LocalProjects interface {
	Projects(ctx context.Context) ([]project.Project, error)
}

type Config struct {
	Projects   LocalProjects     // gives the local project's Path (project.go:28-32)
	Remotes    Remotes
	Deployer   deploy.Identity   // connection.Manager.Current()
	Hub        HubProjects       // nil until the Hub is configured: HubProjectID stays ""
	// AMENDED — the four seams the "Departures" section at the end of this part explains. Each
	// carries a fact this package may NOT own: the spelling of a key, the Hub's rule for which
	// spellings it accepts, what a hubclient error means, and the address a choice is scoped to.
	ParseRemote      ParseRemote                // githost.ParseRemote(u).RepoKey(), through main.go
	HubKeyAccepted   func(string) bool          // hubapi.HostOfRepoKey; REQUIRED when Hub is set
	ClassifyHubError func(error) HubErrorClass  // nil = every failure is a blip
	HubReadTimeout   time.Duration              // per lookup; 0 takes DefaultHubReadTimeout (5 s)
	PanaceaOrigin    func() string              // connection.Manager.Status().Origin
	ChoicesDir string            // statepath.Dir("deploy")
	Now        func() time.Time
	Log        *slog.Logger
}

type Resolver struct{ /* cfg, mu, cache map[projectID]Binding, choices */ }

func New(cfg Config) (*Resolver, error)

// Resolve computes one binding: ResolveMany(ctx, []string{projectID})[projectID], for a caller
// acting on one project. Tasks 09, 11 and 12 read Cached.
// A Panacea or Hub failure returns the LAST good binding with State Unavailable and the
// error — never a silently unbound project.
func (r *Resolver) Resolve(ctx context.Context, projectID string) (Binding, error)

// AMENDED (carried b09-2 I-1): every Hub lookup ResolveMany makes is bounded by
// Config.HubReadTimeout, and the FIRST lookup that fails as a blip or signed-out settles the
// cycle for every key left — a Hub that never answers costs one timeout per cycle, not one per
// repository inside the watcher's 30 s budget.

// ResolveMany computes the bindings of several local projects from ONE LocalProjects.Projects
// read, ONE Deployer.Projects read and one HubProjects.ProjectForRepo per distinct RepoKey,
// whatever len(projectIDs) is (review M16: N local projects were N GET /api/v1/projects every
// 15 s). The watcher (task 07) calls it once per cycle with every project due and reads Cached in
// between. Every id gets an entry. A Panacea failure gives every entry its LAST good binding with
// State Unavailable, and the one error; a Hub failure follows step 3 per key.
func (r *Resolver) ResolveMany(ctx context.Context, projectIDs []string) (map[string]Binding, error)

// Cached is the last computed binding, or (Binding{}, false).
func (r *Resolver) Cached(projectID string) (Binding, bool)

// Choose records the user's pick for an Ambiguous or NoRepoInfo project. The pick must be one
// of the current Candidates (else deploy.KindRejected, "That project is not one of the choices.").
// It is stored keyed by (RepoKey, origin): a choice made against one Panacea is never applied
// to another.
func (r *Resolver) Choose(ctx context.Context, projectID string, p deploy.ProjectRef) (Binding, error)
```

**Matching, exactly:**

1. Local: `Projects` → the project's `Path`; `Remotes.OriginURL(Path)`; `""` → `NoOrigin`.
2. `githost.ParseRemote(url)`; `ErrUnknownHost` → `UnknownHost`. `RepoKey := remote.RepoKey()`.
3. Hub (when configured): `ProjectForRepo(RepoKey)` → `HubProjectID` or `""`. A Hub error is first
   CLASSIFIED (amended, carried b05-2 N-1 and N-2; `Config.ClassifyHubError`, wired in `main.go`
   over `hubclient`'s own error type — this package never reads a status code or a problem body):
   - `HubRefusedKey` — the Hub answered **`400 field: repo`** (Hub task 17, route 13,
     `task-17.md:23`, passed back as `*hubclient.Error{Status, Problem}`, contract `:1325-1329`,
     NOT as `ok == false`). This is an ANSWER: `HubProjectID ""`, `HubMiss "key_spelling"`, no
     *"unreachable"* `Message`, and the cycle is not treated as failed. Without this row every Hub
     lookup error is a blip, a refused key keeps `""` forever, and `key_spelling` is never set
     against the real Hub — the survivor *"every Hub lookup error is a blip"*.
   - `HubSignedOut` — the previous `HubProjectID` is kept and `HubMiss` becomes `"signed_out"`.
   - `HubBlip` (the default, and what an unwired classifier gives) — keeps
   the previous `HubProjectID` **and the previous `HubMiss`** (a key that has never had an answer
   keeps `""`, so a first-read failure is never reported as `not_linked`), and sets `Unavailable`
   only if Panacea's side also failed; a Hub failure alone leaves the Panacea binding intact and is
   reported in `Message` (plan re-review b05 R-4).
   - **The key sent is `RepoKey` verbatim** — `hubKey(remote)` in `binding.go` is the one place it
     is chosen, and it returns `remote.RepoKey()`. Phase 7 never re-spells a key (that would be a
     fourth canonicaliser beside phase 3's `githost`, the Hub's and Panacea's). Both answers to
     HA7-6 (plan §9) keep the key verbatim, so neither changes `hubKey`.
   - A miss sets `HubMiss` through one unexported function, `missCause(repoKey string) string`:
     `"key_spelling"` when `_, ok := hubapi.HostOfRepoKey(repoKey); !ok`, else `"not_linked"`.
     `hubapi.ValidRepoKey` and `HostOfRepoKey` are the Hub's own published rule (Hub plan task 01,
     *"repository keys (contract §7.4, plan correction A1)"*, feat/hub at 52f0773), which the Hub
     fake also applies at `CreateProject` (Hub task 04 step 2, 400 `bad_request`). Phase 7 copies
     **no** pattern, so the sentence can never disagree with the Hub that answered the lookup. The
     check only **chooses a sentence**: the lookup is always made first, so a key the Hub accepts
     binds at once and the flag never appears.
   - Under the Hub's correction A1, `HostOfRepoKey` accepts phase 3's Azure DevOps key
     `azure-devops/<org>/<project>/<repo>` (phase 3 task 13, the `RepoKey` doc comment) and
     refuses the old `<org>.visualstudio.com/<project>/<repo>` form. So an Azure DevOps repository
     binds when a Hub project holds its key, and reads `key_spelling` only if phase 3 and the Hub
     disagree again (HA7-6, review I6).
4. Panacea: `Deployer.Projects`; for each with `RepoURL != ""`, `githost.ParseRemote(RepoURL)` —
   a parse failure skips that project for matching (it stays a candidate) — and compare
   `RepoKey()` with `==` (both sides are `githost`'s lowercased output).
5. Exactly one match → `Bound`, `ChosenBy "auto"`. Zero matches and **every** project had
   `RepoURL == ""` → `NoRepoInfo` with every project as a candidate. Zero otherwise → `NoMatch`.
   Several → a stored choice for `(RepoKey, origin)` that is still among them → `Bound`,
   `ChosenBy "local"`; else `Ambiguous` with the matches as candidates.

**Sentences** (`Message`), exactly:

| State | Message |
|---|---|
| `NoOrigin` | *"This repository has no origin remote, so there is nothing for Panacea to build."* |
| `UnknownHost` | *"Panacea builds from GitHub or Azure DevOps; this repository's origin is neither."* |
| `NoMatch` | *"No Panacea project your key can see deploys this repository. A key restricted to one project sees only that project; otherwise, create one in the Panacea portal, then come back."* — **your key**, because a restricted key's `/projects` holds one project whatever its owner can see (task 03's `projects-scoped.json`; review I5), and the connector cannot tell which kind of key it holds: no body phase 7 reads says so |
| `Ambiguous` | *"Several Panacea projects deploy this repository. Choose the one this cockpit should use."* |
| `NoRepoInfo` | *"Panacea did not say which repository each project deploys. Choose the one for this repository."* |
| `NotConnected` | *"Connect to Panacea to see this project's deployments."* |
| `Unavailable` | *"Panacea could not be reached; showing the last binding."* |

#### The choice file — `statepath.Dir("deploy")/bindings.json`

```json
{
  "version": 1,
  "choices": {
    "github.com/lcfrai/panaceaapp https://dev.panacea.kpmg": {
      "panaceaProject": "engagement-portal",
      "chosenAt": "2026-09-23T10:02:11.123Z"
    }
  }
}
```

Written with the shared `writeAtomic` (ruling R2) after every `Choose`. Read at `New`. A file that
does not parse is **renamed** to `bindings.json.unreadable-<unix>` and treated as empty, with one
`Warn` log line — the user re-chooses; nothing crashes at start. It holds configuration only: no
Panacea response, no Hub data, no token (plan 7P-13). **No persist version is claimed** — this is
not the desktop's store.

#### Control-plane route

```
PUT /v1/projects/{projectId}/deploy/binding
  request { "panaceaProject": "engagement-portal" }
  200 {
    "projectId": "p-8c1d0e42",
    "repoKey": "github.com/lcfrai/panaceaapp" | null,
    "hubProjectId": "pj-3f2a91c07b1e4d2a" | null,
    "panaceaProject": "engagement-portal" | null,
    "state": "bound" | "no_origin" | "unknown_host" | "no_match" | "ambiguous" | "no_repo_info" | "not_connected" | "unavailable",
    "chosenBy": "auto" | "local" | null,
    "candidates": [ { "ref": "engagement-portal", "displayName": "Engagement Portal", "repoUrl": "https://github.com/lcfrai/panaceaapp" } ],
    "message": ""
  }
  404 project_not_found  (phase 2's code — an unknown local project id)
  422 deploy_rejected    — not one of the candidates
  409 deploy_not_connected
  503 upstream_unavailable
```

The same `Binding` object appears as the `binding` field of the view (task 08). There is no `GET`
of its own: the view carries it. `HubMiss` is **not** on this wire: its field is tagged
`json:"-"`, and `TestHubKeyIsRepoKeyVerbatim` asserts the key is absent. The desktop learns it as the
view's `releasesState "hub_key_spelling"` (task 07) and as readiness's sentence (task 09), so the
desktop's binding type does not change (review I6).

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestSingleMatchBindsSilently` | one Panacea project whose `repoUrl` is the SSH form of the local HTTPS origin → `Bound`, `ChosenBy "auto"` | comparing URLs as strings, so SSH and HTTPS never match |
| `TestADOAndGitHubBothMatch` | an `https://dev.azure.com/org/proj/_git/repo` origin against `https://org.visualstudio.com/proj/_git/repo` → `Bound` | a GitHub-only matcher (`OD-4`) |
| `TestAmbiguousAsksOnce` | two matches → `Ambiguous`; `Choose` one → `Bound`, `ChosenBy "local"`; a fresh `Resolver` over the same dir → still `Bound` | a choice that does not survive a restart |
| `TestChoiceScopedToOrigin` | a choice made for origin A is ignored after reconnecting to origin B | applying a dev choice to prod |
| `TestStaleChoiceDropped` | the chosen project disappears from `Projects` → `Ambiguous` again (or `Bound` to the one left) | binding to a project the token can no longer see |
| `TestChooseOutsideCandidates` | `Choose("other")` → `KindRejected`; nothing written | a free-text binding that points Ship at any project name |
| `TestNoRepoInfoOffersAll` | every project lacks `repoUrl` → `NoRepoInfo`, all projects in `Candidates` | an empty-handed `NoMatch` when Panacea simply withholds the field (plan §12) |
| `TestNoMatchUnderARestrictedKey` | `deploytest.Fake` seeded with one project through `AddProject` (the shape task 03's `projects-scoped.json` records; the binding package never loads a Panacea wire body, so the OD-13 swap stays clean — re-review b06 M-3) and a second local repository whose origin is not that project's → `NoMatch` with the table's sentence exactly, which names the key (review I5) | a sentence that tells the holder of a one-project key that the project does not exist |
| `TestPanaceaDownKeepsLastBinding` | `Bound`, then `Projects` fails `KindUnavailable` → returns the previous `Panacea` with `State Unavailable` | the lens flipping to "not bound" during a Panacea blip |
| `TestHubDownKeepsPanaceaBinding` | the Hub fails, Panacea answers → `Bound`, `HubProjectID` kept from the last good read, `Message` says the Hub was unreachable. Two more cases: a binding whose last answer was `HubMiss "key_spelling"` keeps `"key_spelling"` through a Hub failure; a key whose **first** Hub read fails reads `HubMiss ""`, never `"not_linked"` (plan re-review b05 R-4) | Ship disabled by a Hub blip that the Ship path would detect anyway; a blip resetting `HubMiss`, so the Ship bar and the lens flip to *"not a shared project"* for one cycle; a first-read failure reported as *"not a shared project"* about a repository that may be one |
| `TestResolveManyReadsProjectsOnce` | five local projects (four repositories, two sharing one), `deploytest.Fake` and the Hub fake behind call-counting spies → `ResolveMany` of all five: every id has an entry, `Deployer.Projects` was called **once**, `LocalProjects.Projects` once, `ProjectForRepo` four times (one per distinct key); each binding equals what `Resolve` gives that id alone. `Projects` failing `KindUnavailable` → five entries, each its last good binding with `State Unavailable`, and one error (review M16) | one `GET /api/v1/projects` per local project per cycle; a batch that drops an id, or answers one project with another's binding |
| `TestUnreadableChoicesFile` | garbage in `bindings.json` → `New` succeeds, the file is renamed, one warning logged | a crash at start |
| `TestHubBindsADOWithPhase3sKey` (the HA7-6 binding test) | local origin `https://dev.azure.com/kpmgcomau/Engagement%20Portal/_git/portal`, parsed by the **real** `githost`, gives `key := remote.RepoKey()`. `hubclient.NewFake()` is seeded through `CreateProject` with project `pj-ado` whose one remote is `{repoKey: key, host: "azure-devops"}`; the fake applies `hubapi.ValidRepoKey` there (Hub task 04 step 2). `CreateProject` must succeed — a refusal fails the test with *"HA7-6: phase 3's RepoKey <key> is not a form the Hub accepts"* — and `Resolve` then gives `HubProjectID "pj-ado"`, `HubMiss ""`. The seed is never a literal, so the test holds under either answer to HA7-6 and goes red only when phase 3's key and the Hub's rule disagree. Under the Hub's correction A1 and phase 3 task 13 as planned, it passes; HA7-6 closes when it passes against the built code (task 20) | the silent miss of review I6: an Azure DevOps repository that *is* a Hub project read as *"not a shared project in the Hub yet"*, with Ship off and no cause named; and a seed that restates one side's spelling, so the test agrees with that side instead of testing the agreement (plan re-review b05 R-1) |
| `TestHubMissNotLinked` | a GitHub origin, the fake holding no project for it → `HubProjectID ""`, `HubMiss "not_linked"`; the same with no Hub configured → `HubMiss ""`. Directly, under the Hub's rule as of correction A1: `missCause("azure-devops/kpmgcomau/engagement portal/portal") == "not_linked"`; `missCause("kpmgcomau.visualstudio.com/engagement portal/portal") == "key_spelling"` (the contract's old form); `missCause("github.com/lcfrai/panaceaapp") == "not_linked"`; `missCause("github.com/dmackie1_kpmg/portal") == "key_spelling"` (a GitHub owner with `_`, outside the Hub's owner class; plan re-review b05 R-3). If the Hub ever changes A1, the two Azure DevOps literals flip, and nothing else in phase 7 does | a check that marks every miss `key_spelling`; a check keyed on the **host** ("Azure DevOps → key_spelling"); a pattern that checks Azure DevOps only (R-3's survivor, killed by the GitHub `_` case); and a stale copy of the contract's old §7.4 regex in place of `hubapi.HostOfRepoKey` (killed by the first two cases) |
| `TestHubKeyIsRepoKeyVerbatim` | a spy `HubProjects` wrapping the fake records the key `ProjectForRepo` received: exactly `remote.RepoKey()` for a GitHub origin, an Azure DevOps origin, and the GitHub `_` origin whose key the Hub refuses (the lookup is still made). And the wire: `json.Marshal` of a `Binding` with `HubMiss "key_spelling"`, and the `PUT …/deploy/binding` 200 body, decoded into `map[string]any`, have no key equal to `hubMiss` under `strings.EqualFold` (plan re-review b05 R-5) | phase 7 re-spelling keys itself — a fourth canonicaliser that would drift from phase 3's; a lookup skipped when `HostOfRepoKey` refuses the key, which would never bind once the Hub widened its rule; and a `json:"hubMiss"` tag, or no tag, putting `HubMiss` on a wire this task says it is not on |

#### Steps

- [ ] **R2 check first.** `grep -rn "func WriteAtomic" control-plane/internal/statepath` finds task
  05's lift (or phase 4's). Use `statepath.WriteAtomic`. If it is missing, task 05 is not done:
  stop, because this task depends on it (review M10 moved the lift there).
- [ ] Tests first, with a fake `Remotes`, `deploytest.Fake` for `Identity`, and `hubclient.NewFake()`.
- [ ] `binding.go`, `choices.go`, `deploy_binding.go`; register through `routes`.
- [ ] Run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/binding/ ./control-plane/internal/api/ ./control-plane/internal/statepath/ ./control-plane/internal/registry/ -race -count=1
→ ok
```

#### What this task does NOT claim

- That Panacea's `repoUrl` for a real project parses with phase 3's `ParseRemote` — Panacea
  normalises clone URLs (`clients/pipeline.go:174`, `normalizeGitCloneURL`); task 20 checks one
  real project.
- A shared binding. That is HA7-3; until then each teammate with an ambiguous repository picks once.
- That an Azure DevOps repository finds its Hub project against **built** code. The plans agree
  (the Hub's correction A1 takes phase 3 task 13's `azure-devops/…` form; HA7-6, plan §9), but the
  Hub contract's §7.4 text still shows the old `.visualstudio.com` regex until Hub task 44a folds
  A1 in, and phase 3's plan H-3 row still shows it until phase 3 fixes its review I10. If the
  built code disagrees, such a repository reads `HubMiss "key_spelling"`, its Panacea binding and
  reads still work, Ship is off with the cause named (task 09), and `TestHubBindsADOWithPhase3sKey`
  is red. The rule is imported, never copied: `hubapi.HostOfRepoKey` (Hub task 01). Read from plan
  text on feat/hub at 52f0773; no Hub code was read.
- That phase 3's `ParseRemote` accepts a GitHub owner containing `_` (task 13 states no owner
  character rule). If it refuses one, `TestHubKeyIsRepoKeyVerbatim`'s `_` case uses the direct
  `missCause` literal only, and the implementer says so.
- Creating a Panacea project from the cockpit. Out of scope; the `NoMatch` sentence says where.

#### Departures declared when this part was built (2026-09-28)

The part's two hard dependencies are in **no tree**: `control-plane/internal/githost` and
`control-plane/internal/hubapi`/`hubclient` do not exist in this worktree, in `main`, in the
`phase-3` worktree (which has `gitx` and `lanes`, and no `ParseRemote` anywhere) or in the `hub`
worktree. Evidence: `.superpowers/sdd/p7/t06-dep-probe.txt`.

The part says this task "does not start until [phase 3 task 13] is merged". It was built anyway,
because the thing that could not be built is **one function** — the spelling of a repository key —
and the thing the task is actually for is **matching by that key**, which is phase 7's own. So
every fact this package may not own enters through `Config`, and the stand-ins for them live only
in `_test.go` files, which is the one place a fourth canonicaliser cannot be written by accident:

| Seam | `main.go` wires it with | While it is missing |
|---|---|---|
| `Config.ParseRemote func(cloneURL string) (Remote, error)` | `githost.ParseRemote(u).RepoKey()`, mapping `githost.ErrUnknownHost` to `binding.ErrUnknownHost` | the resolver cannot be constructed: `New` requires it |
| `Config.HubKeyAccepted func(repoKey string) bool` | `func(k string) bool { _, ok := hubapi.HostOfRepoKey(k); return ok }` | `New` **refuses** a `Hub` with no rule, so "not_linked" is never shown about a key the Hub could never hold |
| `Config.ClassifyHubError func(error) HubErrorClass` | `hubclient`'s own error type: 400 `field: repo` → `HubRefusedKey`, `ErrSignedOut` → `HubSignedOut`, else `HubBlip` | every failure is a blip, which is the pre-amendment behaviour |
| `HubProjects.ProjectForRepo` returns `(hubProjectID string, ok bool, err error)` | one line over `hubclient.Projects.ProjectForRepo`, taking `.ID` | — |
| `Config.PanaceaOrigin func() string` | `connection.Manager.Status().Origin` | a choice is scoped to `""`, which is one scope, not the wrong one |

Three further rulings taken at root cause and declared:

1. **`NoRepoInfo` also covers a `repoUrl` phase 3 cannot parse.** The part's rule is "every project
   had `RepoURL == ""`". A project whose `repoUrl` does not parse is Panacea not saying which
   repository it deploys in a form this cockpit can use — not Panacea saying it deploys none — and
   the part's own "does NOT claim" row warns that Panacea normalises clone URLs
   (`clients/pipeline.go:174`). Falling to `NoMatch` would tell the user to create a project that
   already exists. The rule built is: no project yielded a usable key → `NoRepoInfo`, all projects
   as candidates.
2. **A stored choice applies to `NoRepoInfo` as well as `Ambiguous`**, which `Choose`'s own doc
   comment already implies and the state table did not say.
3. **The Panacea read happens before the Hub read**, not after as steps 3 and 4 are numbered. A
   Panacea failure then skips the Hub phase entirely, which keeps the previous Hub answer — the
   behaviour step 3 asks for — and costs no Hub call during a Panacea outage. `Bound` shows the
   Hub's sentence when it has none of its own; every other state's sentence wins over it.

`TestHubBindsADOWithPhase3sKey` is **not written**: with both `githost` and `hubapi.ValidRepoKey`
absent it could only test the stand-ins against each other, which is the survivor plan re-review
b05 R-1 named ("a seed that restates one side's spelling"). HA7-6 stays open and belongs to the
task that has both sides. `TestMissCauseRoutesOnTheHubsRule` pins the ROUTING with the plan's four
literals; the rule's own cases belong to the Hub.
