### Task 24b: A cloud machine's catalog, a session on it in its lane, and its conversation store (closes S8.3)

Task 24a put cloud machines in `GET /v1/machines`, with ensure-default and wake. This task makes
them **usable** through the control plane's existing REST surface, every change additive. It is
the second half of what was one task 24, split because the whole was nearer 5 h than 3 (review
m17):

1. The catalog routes take an optional `machineId`; for a cloud machine they are answered by that
   machine's agent (task 20) through the broker's proxy (task 19).
2. **Session create and restart on a cloud machine send machine mounts, never host mounts**: the
   worktree's **lane** and its conversation store as sub-paths of the machine's disk. That is the
   control-plane half of S8.3; task 15 was the Kata half. `katabe`'s host-mount refusal is still
   there, still tested, and now never reached by a cloud create.
3. **Every cloud session runs in a lane (R4.1).** A session on a worktree of kind `primary` or
   `linked` runs in that worktree's **backing lane**, made on the first start and found again on
   every later one. This is phase 3 task 05's rule unchanged (phase-3 `task-05.md:96-112`),
   applied to a cloud machine (env R4 rulings, `.superpowers/sdd/env/r4-env-rulings.md`, ruling
   5). No sandbox mounts `projects/<name>` or anything under it (phase-3
   `.superpowers/sdd/p3/orchestrator-rulings.md:14-17`, R4.1).
4. **Implicit wake** (`docs/FUNCTIONAL-SPEC.md:44`): a create on a sleeping machine wakes it
   first, through task 24a's `wakeMachine`, which also drops the machine list's cache.

**Files:**
- `control-plane/internal/api/catalog.go` — `machineId` on the catalog routes (modify, after task 24a moved `handleMachines` out)
- `control-plane/internal/api/sessions.go` — the cloud branch of create's and restart's mount selection, wake-before-create (modify, after phase 3 task 05)
- `control-plane/internal/api/cloudlane.go` — the lane a cloud session runs in: a lane, or a primary's or linked worktree's backing lane (new)
- `control-plane/internal/api/api.go` — `Deps.CloudLanes`, which task 25 sets (modify, after task 23)
- `control-plane/internal/api/cloudstore.go` — the conversation store on a cloud machine, and `cloudMounts` (new)
- `control-plane/internal/machines/agent.go` — typed calls to the machine agent through `remotebe.Client.Stream` (new)
- `control-plane/internal/httpx/problem.go` — `CodeMachineFull = "machine_full"` (409, review I5) (modify, after task 24a)
- `control-plane/internal/api/*_test.go` (modify/new)

#### Contract — the wire, additive only

```text
Catalog routes — each gains an optional query parameter machineId. Absent or "local": exactly
phase 2's behaviour. A cloud id: the same JSON, from the machine agent, with one additive field
"machineId" on every project, branch and worktree object (local objects gain it too, "local"):

  GET   /v1/projects?machineId=m-0123abcd
  GET   /v1/projects/{projectId}/branches?machineId=…
  GET   /v1/projects/{projectId}/worktrees?machineId=…      (sessionCount and ready decorated
                                                            locally, exactly as today)
  PATCH /v1/projects/{projectId}/worktrees/{worktreeId}?machineId=…   (the ready flag: stored in
                                                            the laptop's registry, keyed by
                                                            machine + worktree id)

A sleeping machine's catalog is 409 conflict with details.state = "sleeping" — the catalog never
wakes a machine (a GET must not), and the cockpit shows the machine asleep.
```

`POST /v1/sessions` is unchanged on the wire: it already carries `machineId` and `worktreeId`
(`internal/api/sessions.go:287-288` in this tree), and no project id.

```go
// agent.go — typed calls to one machine's agent (task 20's routes), through
// remotebe.Client.Stream (task 22) and the broker's agent proxy (task 19).
package machines

// AgentError is a non-2xx answer from the agent, decoded from its httpx.Problem
// body (Details is map[string]string, httpx/problem.go:33).
type AgentError struct {
	Status  int
	Code    string            // the httpx code, e.g. "conflict", "worktree_not_found"
	Message string
	Details map[string]string // a 409's "reason", and task 20's NotALane keys
}

func (e *AgentError) Error() string // "machine agent: <status> <code>: <message>"

// Agent is one cloud machine's agent. Every method is one request; none retries.
type Agent struct {
	MachineID string
	Client    *remotebe.Client
}

func (a *Agent) Projects(ctx context.Context) (machineapi.Projects, error)
func (a *Agent) Branches(ctx context.Context, projectID string) (machineapi.Branches, error)
func (a *Agent) Worktrees(ctx context.Context, projectID string) (machineapi.Worktrees, error)

// WorktreePath asks GET /v1/worktrees/{worktreeID}/path, with ?lane=<lane>
// (url.QueryEscape'd) when lane != "". Task 20 answers a lane path only.
func (a *Agent) WorktreePath(ctx context.Context, worktreeID, lane string) (machineapi.WorktreePath, error)

func (a *Agent) EnsureDirs(ctx context.Context, paths ...string) error // POST /v1/dirs
func (a *Agent) ForgetConversation(ctx context.Context, worktreeID, conversationID string) error
func (a *Agent) RemoveConversations(ctx context.Context, worktreeID string) error

// AsNotALane reports whether err is a 409 conflict whose details.reason is
// worktree_not_a_lane, and decodes task 20's machineapi.NotALane from its keys.
func AsNotALane(err error) (machineapi.NotALane, bool)
```

```go
// cloudlane.go — the lane a cloud session runs in (R4.1; env ruling 5; phase-3 task-05.md:96-112).
package api

// laneCreator is the one method of phase 3's *lanes.Creator this task calls
// (phase-3 task-03.md:199), so a test can pass a fake that records the request.
type laneCreator interface {
	Create(ctx context.Context, req lanes.CreateRequest) (lanes.Lane, error)
}

// CloudLanes answers the creator that makes lanes on one cloud machine. It shares
// phase 3's one lane index (GitDeps.Lanes, phase-3 task-04.md:65), so a cloud lane
// is found by the same ByID and ByProject as a local one. Task 25 sets it to
// &lanes.Creator{Machine: <that machine's gitx.Remote>, Store: GitDeps.Lanes}.
// A nil CloudLanes, or ok false, is errCloudLanesUnwired.
type CloudLanes func(machineID string) (c laneCreator, ok bool)

// errCloudLanesUnwired is 503 backend_unavailable, details.cloud "lanes_unwired":
// this machine cannot make a lane yet, so no session starts. It is never answered
// by mounting the worktree itself.
var errCloudLanesUnwired = errors.New("api: this cloud machine cannot make lanes yet")

// detachedError is 409 conflict, details.reason "worktree_detached" (phase 3 task
// 04's ReasonWorktreeDetached), details.worktreeId, with phase 3 task 05's sentence:
// "This worktree has no branch checked out, so the cockpit cannot make its lane.
// Check out a branch in <Path>, or start the session in a new worktree."
type detachedError struct{ WorktreeID, Path string }

func (e *detachedError) Error() string

// cloudLane answers the lane a create or restart on machineID mounts, for the
// rail's worktree id railID, in phase 3 task 05's order:
//  1. GitDeps.Lanes.ByID(railID) with MachineID == machineID: that lane (the rail
//     shows a lane). A lane of another machine is the existing worktree_not_found.
//  2. Otherwise agent.WorktreePath(ctx, railID, ""): AsNotALane gives d; an
//     AgentError 404 is worktree_not_found; any other error is returned as is.
//  3. The lane in GitDeps.Lanes.ByProject(d.ProjectID) whose Name == railID and
//     MachineID == machineID: that lane (a later start makes no second lane).
//  4. d.Branch == "": &detachedError{railID, d.Path}. No lane, no pod, no record.
//  5. p, the project agent.Projects lists with ID d.ProjectID (the enumerator's
//     answer, never a path from the request: phase-3 task-03.md:130-131; absent is
//     project_not_found); c, ok := CloudLanes(machineID) (not ok is
//     errCloudLanesUnwired); then c.Create(ctx, lanes.CreateRequest{Project: p,
//     Branch: d.Branch, Name: railID}). Its errors map as phase 3 task 04's
//     create route maps them (lanes.ErrNameTaken is 409, reason name_taken).
// A restart finds the lane at step 1 or 3.
func (a *API) cloudLane(ctx context.Context, agent *machines.Agent, machineID, railID string) (lanes.Lane, error)
```

```go
// cloudstore.go — the conversation store on a cloud machine (SX.10), and the OD-10 deletes.
type conversationStore interface {
	// Forget deletes one conversation (OD-10); Remove deletes a worktree's store.
	// The id is always the rail's WorktreeID, never a backing lane's.
	Forget(ctx context.Context, worktreeID, conversationID string) error
	Remove(ctx context.Context, worktreeID string) error
}

func (a *API) conversationStoreFor(ctx context.Context, machineID string) (conversationStore, error)

// cloudStore is the cloud conversationStore. Ensure is create's and restart's:
// POST /v1/dirs {"paths":["conversations/<railID>"]} on the agent, so the store
// exists, agent-owned and 0700, before its pod mounts it (task 15's rule).
type cloudStore struct{ agent *machines.Agent }

func (s cloudStore) Ensure(ctx context.Context, railID string) error
func (s cloudStore) Forget(ctx context.Context, worktreeID, conversationID string) error
func (s cloudStore) Remove(ctx context.Context, worktreeID string) error

// cloudMounts is a cloud session's MachineMounts, in this order:
//  1. {MachinePath: lane.MachinePath, ContainerPath: "/home/dev/work"};
//  2. when lane.GitConfig != "": {MachinePath: lane.GitConfig,
//     ContainerPath: "/home/dev/work/.git/config", ReadOnly: true} (hygiene, below);
//  3. {MachinePath: "conversations/" + railID, ContainerPath: "/home/dev/.claude/projects"}.
// lane is task 20's answer for the lane cloudLane resolved, so MachinePath is
// lanes/<projectId>/<dir> and never under projects/ (R4.1). railID keeps the
// store the rail's (phase-3 task-05.md:105-106), so a primary worktree's
// conversations stay under its own id, not its backing lane's.
func cloudMounts(lane machineapi.WorktreePath, railID string) []backend.MachineMount
```

#### Create and restart on a cloud machine, in order

1. Resolve the backend (task 23). If the machine is `sleeping`: wake it through task 24a's `wakeMachine(ctx, id)`, the same
   function `POST /v1/machines/{machineId}/wake` calls, which drops the 5 s list cache on success
   (plan re-review b08-1 issue 5). Then poll the machine every 500 ms until `idle` or `running`,
   bounded by the ready timeout; the poll asks the broker for that one machine, never through the
   cached list, so it cannot read 5 s of `sleeping`. The session record is `starting` throughout, so the
   cockpit's inline "waking…" (task 26) is truthful. **Wake comes first** because the agent runs
   in the machine pod, and a sleeping machine is "the disk with no pods" (P8-D6, plan `:136`).
   The pre-R4 order asked the agent for the path before waking.
2. **Resolve the lane** (env ruling 5): `cloudLane(ctx, agent, machineID, worktreeID)`, above.
   `worktree_detached`, `lanes_unwired`, `worktree_not_found` and `project_not_found` answer
   here, and no sandbox is created and no session is recorded.
3. **Ask for the lane's path:** `agent.WorktreePath(ctx, lane.ID,
   machineapi.LaneMachinePath(lane.Path))` → `WorktreePath` (task 20). The id sent is the
   lane's, never the rail's: task 20 refuses a primary's id whatever `lane` names. A
   `LaneMachinePath` error is `409 conflict`, reason `lane_path_invalid`. The agent's 409 with
   reason `lane_path_invalid`, `git_config_not_a_file` or `lane_creating` (task 21) is `409
   conflict` with that reason. Its 404 is `worktree_not_found`. On any of them no sandbox is
   created.
4. `cloudStore.Ensure(ctx, worktreeID)`, then `cloudMounts(path, worktreeID)` → `MachineMounts`;
   `WorkDir: sessionWorkDir` (`sessions.go:33`), `Mounts: nil`, `Machine: machineID`. When
   `gitConfig` is set the `MachineMounts` are three, the file mount listed after the lane mount
   it sits inside. The runtime must mount the parent first; containerd's CRI sorts a container's
   mounts by destination depth, from its source (**UNVERIFIED** here, and on Kata). **Why the
   bind, after R4 (review I7, b06 re-review 1, Q18):** it was written as the defence against an
   agent racing `machined`'s H3 on a lane's `.git/config`. Under phase 3's ruling R4 no host git
   reads a lane's config at all, and lane git runs only in `sandboxd` in the pod that mounts the
   lane (env ruling 2, which amends P8-D7). So the bind is hygiene, as phase 3 keeps its own
   (phase 3 plan `:245`, `:712`), and nothing is claimed from it. The cost is phase 3's (its Q7):
   the agent's own `git config` in its checkout fails.
5. `be.Create` — the broker answers after the pod is ready (task 18).

**Restart** on a cloud machine runs the same five steps with the session's recorded
`WorktreeID`, so `cloudLane` finds the lane at its step 1 or 3 and makes none. Create and restart
keep the rail's `WorktreeID` in the registry; only the mounted path is the lane's (phase-3
`task-05.md:105-106`). This is the cloud arm of phase 3 task 05's create and restart change
(`sessions.go:508`, `:1101` in this tree, where both call `sessionMounts(path, …)`); the local
arm is phase 3's, unchanged.

An `ErrQuotaExceeded` from the backend is `429 quota_exceeded` (task 24a's code) with the
broker's `details.limit`; `ErrUnavailable` wrapping `ErrSignedOut` is `503 backend_unavailable`
with `details.cloud = "signed_out"`.

An `ErrMachineFull` (review I5) is **`409 machine_full`**, checked before the other 409s, with
`details.sessionsPerMachine` from the `*backend.MachineFullError` (omitted when 0) and a message
the cockpit renders verbatim (its `server-said` arm, `desktop/src/hooks/useCreateSession.ts:37-46`):
three forms, chosen by the count (b05 re-review 2, 6). The node is shared, so other users' pods
use room too, and a cordoned node (P12) has room for nothing; so no form says this machine's
sessions fill the node, and only a form with a count suggests stopping one:
- count 2 or more: "This cloud machine's node has no room for another session right now (it
  already runs this machine's 3 sessions), so the session was not created. Stop one of them, or
  try again later." (`3` is the count.)
- count 1: "This cloud machine's node has no room for another session right now (it already runs
  this machine's one session), so the session was not created. Stop it, or try again later."
- count 0 or absent: "This cloud machine's node has no room for another session right now, so the
  session was not created. Try again later."

It is not `backend_unavailable`: the cockpit would say the backend is down, and the
user would wait for a recovery that is not coming.

**Forget and worktree removal (OD-10).** If the phase 2 OD-10 follow-up has landed, its deletes
go through `conversationStoreFor` in this task (`git grep` for its delete call first); if it has
not, the local implementation of `Forget`/`Remove` is written here to the same behaviour and the
follow-up uses the interface. Either way a cloud session's Forget reaches the machine agent's
`DELETE /v1/conversations/…` (task 20).

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCatalogWithoutMachineIDIsUnchanged` | every existing catalog test passes; local objects gain only `machineId: "local"` | a changed phase 2 shape |
| `TestCloudCatalogComesFromTheMachineAgent` | `?machineId=m-…` → the stand-in agent's projects, each with `machineId` | a cloud catalog read from the laptop's disk |
| `TestASleepingMachinesCatalogIs409AndDoesNotWake` | 409, `details.state = "sleeping"`, no wake call | a GET that boots a VM |
| `TestCloudCreateSendsMachineMountsNeverHostMounts` | a stand-in agent whose `WorktreePath` has no `gitConfig`: the fake backend received `Mounts == nil`, two `MachineMounts` with the agent's path and `conversations/<w>`, `WorkDir == "/home/dev/work"` | S8.3 "fixed" by dropping katabe's refusal |
| `TestCloudCreateBindsTheCloneConfigReadOnly` | the stand-in answers `gitConfig: "lanes/p-1/w-2/.git/config"` → three `MachineMounts`, the second exactly `{MachinePath: "lanes/p-1/w-2/.git/config", ContainerPath: "/home/dev/work/.git/config", ReadOnly: true}`, listed after the worktree mount; the stand-in answering 409 `git_config_not_a_file` → `409 conflict` with that reason and no `Create` | the hygiene bind dropped on cloud machines, or made writable; a session created over a config Kubernetes would make a directory (a `projects/a` mount is retired by R4.1, above) |
| `TestCloudCreateMakesTheStoreBeforeTheSandbox` | the agent saw `POST /v1/dirs` for `conversations/<w>` before the backend saw `Create` | a root-owned store the agent cannot write |
| `TestCreateOnASleepingMachineWakesItFirst` | wake, then polls, then `Create`; the record was `starting` meanwhile; with the list cached beforehand, the next `GET /v1/machines` after the wake makes a fresh broker call (plan re-review b08-1 issue 5) | "machine asleep" shown to a user who just typed; an implicit wake that leaves the rail at `sleeping` for 5 s, so task 26's waking state never shows |
| `TestQuotaIs429WithTheLimit` | `ErrQuotaExceeded` from the backend's `Create` → 429 `quota_exceeded`, `details.limit` | a quota shown as "unavailable" |
| `TestAFullMachineIs409MachineFullWithASentence` | the fake backend returns `&backend.MachineFullError{SessionsPerMachine: 3}` → 409 `machine_full`, `details.sessionsPerMachine = "3"`, the message exactly the first form above; with `1` → `"1"` and exactly the singular form (never "1 sessions"); with `0` → no `sessionsPerMachine` key and exactly the third form, which says nothing of stopping a session (review I5; b05 re-review 6) | a full machine shown as "backend unavailable" or as `conflict` |
| `TestLocalCreateIsUnchanged` | a local create still sends phase 2's host mounts (`TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv`, `sessions_test.go:749`, passes unchanged) | the local machine broken by the cloud branch |
| `TestCloudForgetReachesTheMachineAgent` | Forget on a cloud session → `DELETE /v1/conversations/<w>/<id>` at the agent | OD-10 honoured on the laptop only |

#### Steps

- [ ] **Step 1:** Tests; they fail.
- [ ] **Step 2:** `machines/agent.go` (typed wrappers over `Client.Stream` for the agent routes);
  the catalog's `machineId`; `cloudstore.go`; the create path; `CodeMachineFull`.
- [ ] **Step 3:** Full run; commit. This task registers no route, so the route audits are
  unchanged and still pass.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1        # ok
go test ./control-plane/internal/api/ -run 'TestRoutes|TestNoRegisteredGet' -count=1 -v   # unchanged since task 24a
```

#### What this task does not claim

- **Anything on screen**: task 26.
- That the UI contract's `types.ts` already carries `machineId` on projects — task 26 amends
  `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` §2 and `types.ts` together
  (rule SX.2).
- Adding a project to a cloud machine (a clone through task 21's route): seam S8-P3.2.
- **UNVERIFIED:** that a Kata guest enforces a `readOnly` `subPath` **file** mount, so that the
  agent's `git config` write and git's `config.lock` rename onto `/home/dev/work/.git/config`
  both fail inside the microVM. Phase 3's `TestSandboxCloneConfigIsReadOnly` covers Docker
  only. Nothing here runs against a cluster (the stream's rule); the first live Kata session
  must run `sh -c 'echo x >> /home/dev/work/.git/config'` and `git config x.y z` in the sandbox
  and see both refused, and task 33 carries that check into the results doc.
