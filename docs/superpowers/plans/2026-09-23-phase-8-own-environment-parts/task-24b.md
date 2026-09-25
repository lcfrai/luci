### Task 24b: A cloud machine's catalog, a session on it, and its conversation store (closes S8.3)

Task 24a put cloud machines in `GET /v1/machines`, with ensure-default and wake. This task makes
them **usable** through the control plane's existing REST surface, every change additive. It is
the second half of what was one task 24, split because the whole was nearer 5 h than 3 (review
m17):

1. The catalog routes take an optional `machineId`; for a cloud machine they are answered by that
   machine's agent (task 20) through the broker's proxy (task 19).
2. **Session create on a cloud machine sends machine mounts, never host mounts** — the worktree
   and its conversation store as sub-paths of the machine's disk. That is the control-plane half
   of S8.3; task 15 was the Kata half. `katabe`'s host-mount refusal is still there, still tested,
   and now never reached by a cloud create.
3. **Implicit wake** (`docs/FUNCTIONAL-SPEC.md:44`): a create on a sleeping machine wakes it
   first, through task 24a's `wakeMachine`, which also drops the machine list's cache.

**Files:**
- `control-plane/internal/api/catalog.go` — `machineId` on the catalog routes (modify, after task 24a moved `handleMachines` out)
- `control-plane/internal/api/sessions.go` — the cloud branch of mount selection, wake-before-create (modify)
- `control-plane/internal/api/cloudstore.go` — the conversation store on a cloud machine (new)
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

`POST /v1/sessions` is unchanged on the wire: it already carries `machineId`
(`internal/api/sessions.go:215`).

```go
// cloudstore.go — the conversation store on a cloud machine (SX.10), and the OD-10 deletes.
type conversationStore interface {
	// Mounts returns what the sandbox for worktreeID gets, creating the store
	// first if needed. Local: phase 2's host mounts (sessionMounts). Cloud: the
	// worktree's machine path at /home/dev/work; when the agent's WorktreePath
	// names a GitConfig (task 20), that file read-only at
	// /home/dev/work/.git/config (hygiene after phase 3's ruling R4, as phase 3's
	// task 05 sessionMounts bind is for Docker; nothing is claimed from it); and
	// "conversations/<worktreeID>" at /home/dev/.claude/projects — after
	// POST /v1/dirs on the machine agent creates the latter (task 15's rule).
	Mounts(ctx context.Context, worktreeID string) ([]backend.Mount, []backend.MachineMount, error)
	// Forget deletes one conversation (OD-10); Remove deletes a worktree's store.
	Forget(ctx context.Context, worktreeID, conversationID string) error
	Remove(ctx context.Context, worktreeID string) error
}

func (a *API) conversationStoreFor(ctx context.Context, machineID string) (conversationStore, error)
```

#### Create on a cloud machine, in order

1. Resolve the backend (task 23); resolve the worktree **on that machine**: machine agent
   `GET /v1/worktrees/{worktreeId}/path` → `machinePath` and `gitConfig` (404 → the existing
   `worktree_not_found`; 409 `git_config_not_a_file` → `409 conflict` with that reason, and no
   sandbox is created).
2. If the machine is `sleeping`: wake it through task 24a's `wakeMachine(ctx, id)`, the same
   function `POST /v1/machines/{machineId}/wake` calls, which drops the 5 s list cache on success
   (plan re-review b08-1 issue 5). Then poll the machine every 500 ms until `idle` or `running`,
   bounded by the ready timeout; the poll asks the broker for that one machine, never through the
   cached list, so it cannot read 5 s of `sleeping`. The session record is `starting` throughout, so the
   cockpit's inline "waking…" (task 26) is truthful.
3. `conversationStoreFor(machine).Mounts(worktree)` → `MachineMounts`; `WorkDir: sessionWorkDir`
   (`sessions.go:33`), `Mounts: nil`, `Machine: machineID`. When `gitConfig` is set, the
   `MachineMounts` are three, in this order: `{MachinePath: machinePath, ContainerPath:
   "/home/dev/work"}`, `{MachinePath: gitConfig, ContainerPath: "/home/dev/work/.git/config",
   ReadOnly: true}`, and the conversation store. The file mount is listed after the worktree
   mount it sits inside. The runtime must mount the parent first; containerd's CRI sorts a
   container's mounts by destination depth, from its source (**UNVERIFIED** here, and on Kata). **Why, after R4 (review I7, b06 re-review 1, Q18):**
   the bind was written as the defence against an agent racing `machined`'s H3 on a lane's
   `.git/config`. Under phase 3's ruling R4 no host git reads a lane's config at all: lane git
   runs only through the `/git/v1/` runner (P8-D7, task 21), so the bind is hygiene, as phase 3
   keeps its own (phase 3 plan `:245`, `:712`), and nothing is claimed from it. The cost is
   phase 3's (its Q7): the agent's own `git config` in its checkout fails. **R4.1:** no session
   mounts `projects/<name>`; a session on the primary worktree runs in its backing lane (phase 3
   task 05), so this create must mount that lane, never the primary's `machinePath`. How the
   laptop's lane choice reaches this step is **open**, for the task 21 and 25 re-plan.
4. `be.Create` — the broker answers after the pod is ready (task 18).

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
