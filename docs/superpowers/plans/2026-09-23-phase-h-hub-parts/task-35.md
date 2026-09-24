### Task 35: `hublink` — which Hub project a local project is; and the share routes (seam S-LINK)

**Lane C. Depends on: 32, 34; phase 3 task 13's `githost` (or its copy, below). About 3 hours. Contract task C2.**

A local project's id is a hash of a path on one laptop (`control-plane/internal/project/project.go:29`),
so no Hub record can name it. Every Hub-backed handler — phase 4's Discover, phase 5's Design,
phase 7's Deploy, phase 3's evidence — first asks: **which Hub project is this?** The answer comes
from the project's git remotes: phase 3's `githost.RepoKey` for each, then
`GET /v1/projects?repo=` (contract §7.4). This task is that resolver — **seam S-LINK** — and the
control-plane routes the share surface (task 42) calls to link an unshared project to a team
(owner question Q8's recommendation: explicit sharing, automatic linking when the repository is
already shared).

**`githost` is phase 3's.** Phase 2's scanner reads no remotes. Phase 3 task 13 writes
`control-plane/internal/githost/remote.go` (`Remote`, `ParseRemote`, `RepoKey`, `Host`). If it has
not landed, this task writes it **from phase 3 task 13's contract** — the same
"whichever lands first" rule as tasks 10 and 29 — together with phase 3's two tests for it
(`TestParseRemoteTable`, `TestRepoKeyIsMachineIndependent`). The contract holds the declarations,
not the bodies, so the later merge is an add/add conflict, not a no-op, and **phase 3 wins it**:
the orchestrator keeps phase 3's file whole and re-runs this task's tests (plan, "Files that
belong to a sibling phase"; review M5).

**Files:**
- `control-plane/internal/githost/remote.go` + `remote_test.go` — **phase 3's**; only if absent (new)
- `control-plane/internal/hublink/hublink.go` + `hublink_test.go` (new)
- `control-plane/internal/api/hublink.go` + `hublink_test.go` — the routes (new)
- `control-plane/internal/api/api.go` — `Deps.HubLink`; `a.registerHubLink(guarded)` (modify ⚑: one field, one line)
- `control-plane/internal/api/routes_audit_test.go` — four `GET`s (modify ⚑)
- `control-plane/cmd/api/main.go` — build the resolver (modify ⚑)

#### Contract — complete

```go
// control-plane/internal/hublink/hublink.go

// Package hublink answers which Hub project a local project is (seam S-LINK).
// It reads remotes with `git config --get-regexp` — no network, no fetch — and
// asks the Hub by repository key. It caches in memory only (ADR-H1 §5.4).
package hublink

type State string

const (
	Linked   State = "linked"    // a Hub project the user can read holds one of the remotes
	Unlinked State = "unlinked"  // none does: the project can be shared
	NoRemote State = "no_remote" // no GitHub or Azure DevOps remote: it cannot be shared
)

type RepoRef struct {
	RepoKey string         // githost.Remote.RepoKey()
	Host    hubapi.GitHost // github | azure-devops
	Remote  string         // the git remote's name, e.g. "origin"
}

type Link struct {
	State    State
	Project  *hubapi.Project // set when Linked
	RepoKeys []RepoRef       // origin first, then by remote name
}

// Remotes reads a local project's remotes. main.go's implementation runs
//   git -C <project path> config --get-regexp ^remote\..*\.url$
// through project.Runner and parses each URL with githost.ParseRemote,
// skipping what it does not recognise.
type Remotes interface {
	RepoRefs(ctx context.Context, localProjectID string) ([]RepoRef, error)
}

type Resolver struct{ /* unexported: hub, remotes, cache, clock, log */ }

func New(hub hubclient.Projects, remotes Remotes, log *slog.Logger) *Resolver

// Project resolves localProjectID. Each repo key is asked of the Hub in order;
// the first Linked answer wins. Results are cached per local project for 5
// minutes, and dropped at once by Invalidate. Hub errors are returned as they
// are (hubclient's), so the caller relays them with hubclient.WriteError.
func (r *Resolver) Project(ctx context.Context, localProjectID string) (Link, error)

// Share links an Unlinked project: CreateProject in teamID, named name (the
// local project's name when ""), with every RepoRef as a remote. A remote that
// is already a readable project's makes it Linked to that project instead
// (Q8: the same repository is the same project). A 409 repo_already_linked with
// no projectId — the repository is shared by a team the user is not in — is
// returned as the Hub's *hubclient.Error, unchanged.
func (r *Resolver) Share(ctx context.Context, localProjectID, teamID, name string) (Link, error)

// Invalidate drops the cache for one local project, or for all when id is "".
// Task 36's feed subscriber calls it with "" on any project.* event.
func (r *Resolver) Invalidate(localProjectID string)

var ErrUnknownProject = project.ErrUnknownProject
```

**The routes** (control plane, canonical `/v1`; `projectId` is the **local** `p-…` id; every Hub
failure answers through `hubclient.WriteError`, task 33):

| Method | Path | Body | Success | Errors (besides the relay's) |
|---|---|---|---|---|
| GET | `/v1/projects/{projectId}/hub-link` | — | `200 {"state":"linked"\|"unlinked"\|"no_remote","project":Project\|null,"repoKeys":[{"repoKey","host","remote"}]}` | 404 `project_not_found` (the local id) |
| POST | `/v1/projects/{projectId}/hub-link` | `{"teamId":"tm-…","name":"…"}` (`name` optional) | `201` the link, `state: linked` (or `200` when it auto-linked to an existing project) | 404; 409 `conflict` when the state is `no_remote`; the Hub's 409 `repo_already_linked` and 404 `team_not_found` unchanged |
| GET | `/v1/hub/teams` | — | `200 {"items":[Team]}` | |
| POST | `/v1/hub/teams` | `CreateTeamRequest` | `201 Team` | |
| GET | `/v1/hub/teams/{teamId}` | — | `200 TeamDetail` | |
| GET | `/v1/hub/teams/{teamId}/invites` | — | `200 {"items":[Invite]}` | |
| POST | `/v1/hub/teams/{teamId}/invites` | `CreateInviteRequest` | `201 Invite` | |
| DELETE | `/v1/hub/teams/{teamId}/invites/{inviteId}` | — | `204` | |

The four `GET`s go in `catalogReadOnlyRoutes`. A `Hub`-backed route with `Deps.Hub` nil answers
503 `hub_not_configured`. The desktop client for all of them is task 40.

#### Tests to write first

`hublink_test.go` (the resolver, over `hubclient.NewFake()` and a scripted `Remotes`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLinkedWhenARemoteIsShared` | the fake holds a project with `github.com/lcfrai/panaceaapp`; the local project's `origin` is that repo → `Linked`, the project | a Discover pane writing into a new Hub project instead of the team's |
| `TestOriginIsAskedFirst` | two remotes, both shared by different projects → `origin`'s wins | a link that flips with remote ordering |
| `TestUnlinkedAndNoRemote` | an unshared GitHub remote → `Unlinked`; only a GitLab remote → `NoRemote` | a share offered for a repository the Hub cannot key |
| `TestShareCreatesTheProjectWithEveryRemote` | `Share` → the Hub project holds both repo keys with their hosts; the link is `Linked` | a project that one teammate's second remote cannot find |
| `TestShareAutoLinks` | a remote already shared by a team the user is in → `Share` answers that project, creates nothing | two projects for one repository (Q8) |
| `TestShareElsewhereIsTheHubsError` | the fake answers 409 `repo_already_linked` without `projectId` → the `*hubclient.Error` unchanged | a generic failure where the user needs "ask for an invite" |
| `TestTheCacheAndItsInvalidation` | two `Project` calls → one `ProjectForRepo`; `Invalidate("")` → the next call asks again; 5 min 1 s later → asks again | a stale link after a teammate shares the repository |
| `TestRemotesAreReadWithoutNetwork` | a Script runner: the argv is exactly `config --get-regexp ^remote\..*\.url$` | a `git fetch` or `ls-remote` on the rail's path (phase 2's non-blocking rule) |

`api/hublink_test.go` (the routes, over a fake `Resolver` and the fake Hub):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHubLinkReadsTheState` | each of the three states → the JSON above | — |
| `TestShareRoute` | POST → 201 linked; auto-link → 200; `no_remote` → 409 `conflict`; the Hub's 409 passes unchanged | — |
| `TestAnUnknownLocalProjectIs404` | → 404 `project_not_found` | a Hub call for a project that does not exist here |
| `TestTeamAndInviteRoutesPassThrough` | create a team, list it, invite, list, revoke — through the control plane, against the fake | — |
| `TestSignedOutAndUnconfigured` | the fake signed out → 401 `signed_out`; `Deps.Hub` nil → 503 `hub_not_configured` | a share surface that fails silently |
| `TestTheNewGetsAreAudited` | phase 2's route audit passes | a `GET` outside the audit |

#### Steps

- [ ] **Step 1: Preflight.** `ls control-plane/internal/githost/remote.go`. Absent → write phase 3
  task 13's `remote.go` and its two tests from its contract (sibling `feat/phase-3-git`, read only). Record
  which case applied.
- [ ] **Step 2:** Tests; they fail. Then `hublink.go`, the routes, the wiring (`main.go`'s
  `Remotes` over `project.Scanner`'s paths and `project.NewExecRunner()`).
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hublink/ ./control-plane/internal/api/ ./control-plane/internal/githost/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                                # ok
```

#### What this task does not claim

- That phase 3's `RepoKey` and the Hub's validation agree on every remote form: the conformance
  row `projects/repo_key_forms` and phase 3's own table test check the pair.
- A sharing UI: task 42.
- Members' role changes and removal through the control plane: the Hub routes exist (8, 9); v1's
  share surface does not offer them.
