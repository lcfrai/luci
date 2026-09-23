### Task 06: The binding — which Panacea project deploys this repository

**Half:** 7a. **Hours:** 2. **Depends on:** 03, 05. **Elsewhere:** phase 3 task 13
(`githost.ParseRemote`, `Remote.RepoKey()`); Hub H0 (`hubclient.Projects` and its fake).

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
- `control-plane/internal/statepath/atomic.go` (new, **only if ruling R2's lift has not landed**) — see Steps

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
	NoMatch        State = "no_match"        // no Panacea project this token can see deploys this repository
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
	HubProjectID string            // "pj-…"; "" when the repository is not a Hub project — reads work, Ship does not
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
	ChoicesDir string            // statepath.Dir("deploy")
	Now        func() time.Time
	Log        *slog.Logger
}

type Resolver struct{ /* cfg, mu, cache map[projectID]Binding, choices */ }

func New(cfg Config) (*Resolver, error)

// Resolve computes the binding. It calls Deployer.Projects at most once per call and caches the
// result; the watcher (task 07) calls it once per cycle per local project and reads Cached in
// between. A Panacea or Hub failure returns the LAST good binding with State Unavailable and the
// error — never a silently unbound project.
func (r *Resolver) Resolve(ctx context.Context, projectID string) (Binding, error)

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
3. Hub (when configured): `ProjectForRepo(RepoKey)` → `HubProjectID` or `""`. A Hub error keeps
   the previous `HubProjectID` and sets `Unavailable` only if Panacea's side also failed; a Hub
   failure alone leaves the Panacea binding intact and is reported in `Message`.
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
| `NoMatch` | *"No Panacea project you can see deploys this repository. Create one in the Panacea portal, then come back."* |
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
of its own: the view carries it.

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
| `TestPanaceaDownKeepsLastBinding` | `Bound`, then `Projects` fails `KindUnavailable` → returns the previous `Panacea` with `State Unavailable` | the lens flipping to "not bound" during a Panacea blip |
| `TestHubDownKeepsPanaceaBinding` | the Hub fails, Panacea answers → `Bound`, `HubProjectID` kept from the last good read, `Message` says the Hub was unreachable | Ship disabled by a Hub blip that the Ship path would detect anyway |
| `TestUnreadableChoicesFile` | garbage in `bindings.json` → `New` succeeds, the file is renamed, one warning logged | a crash at start |

#### Steps

- [ ] **R2 check first.** `grep -rn "func WriteAtomic\|func writeAtomic" control-plane/internal`.
  If only `registry/filestore.go:460` has it, do R2's lift in this task: move it with its tests to
  `control-plane/internal/statepath/atomic.go` as `WriteAtomic`, make `registry` call it, and say so
  in the commit message as a declared departure into phase 2's closed file. If `statepath` already
  exports it, use it.
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
- Creating a Panacea project from the cockpit. Out of scope; the `NoMatch` sentence says where.
