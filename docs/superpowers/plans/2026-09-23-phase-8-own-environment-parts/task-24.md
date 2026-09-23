### Task 24: Cloud machines in the API — the list, the catalog, and a session on a cloud machine (closes S8.3)

Task 23 made a machine id route. This task makes cloud machines **visible and usable** through
the control plane's existing REST surface, every change additive:

1. `GET /v1/machines` lists the user's cloud machines beside the local one, with the states the
   Go contract reserved for this phase (`2026-09-18-phase-2-go-contract.md:1593-1600`) and
   `isolation: "microvm"` (seam S8.5 — `isolationFor` already maps `kata` to it,
   `internal/api/catalog.go:174-183`).
2. The catalog routes take an optional `machineId`; for a cloud machine they are answered by that
   machine's agent (task 20) through the broker's proxy (task 19).
3. **Session create on a cloud machine sends machine mounts, never host mounts** — the worktree
   and its conversation store as sub-paths of the machine's disk. That is the control-plane half
   of S8.3; task 15 was the Kata half. `katabe`'s host-mount refusal is still there, still tested,
   and now never reached by a cloud create.
4. **Implicit wake** (`docs/FUNCTIONAL-SPEC.md:44`): a create on a sleeping machine wakes it first.

**Files:**
- `control-plane/internal/api/machines.go` — `GET /v1/machines` (moved out of `catalog.go`), `POST /v1/machines/default`, `POST /v1/machines/{machineId}/wake` (new)
- `control-plane/internal/api/catalog.go` — `machineId` on the catalog routes; `handleMachines` leaves (modify)
- `control-plane/internal/api/sessions.go` — the cloud branch of mount selection, wake-before-create (modify)
- `control-plane/internal/api/cloudstore.go` — the conversation store on a cloud machine (new)
- `control-plane/internal/machines/agent.go` — typed calls to the machine agent through `remotebe.Client.Stream` (new)
- `control-plane/internal/httpx/problem.go` — `CodeQuotaExceeded = "quota_exceeded"` (429) (modify)
- `control-plane/internal/api/*_test.go` (modify/new)

#### Contract — the wire, additive only

```jsonc
// GET /v1/machines — the envelope gains "cloud"; each machine is phase 2's machine object.
{
  "machines": [
    { "id": "local", "name": "This PC", "kind": "local", "state": "running", "backend": "docker",
      "isolation": "container", "default": false, "limitations": ["…"] },
    { "id": "m-0123abcd", "name": "Cloud machine", "kind": "cloud", "state": "sleeping",
      "backend": "kata", "isolation": "microvm", "default": true,
      "limitations": ["Sessions run in a Kata microVM with its own kernel."] }
  ],
  // NEW. Absent when no broker is configured (PANACEA_APP_BROKER_URL unset), so a phase 2
  // client sees exactly the phase 2 body.
  "cloud": {
    "status": "ok",               // "ok" | "signed_out" | "unavailable"
    "message": ""                 // one sentence for signed_out / unavailable, rendered verbatim
  }
}
```

- `default` is `true` on the user's default cloud machine when one exists (FUNCTIONAL-SPEC:
  "one cloud by default, plus this Mac"), and on the local machine otherwise — so with no broker,
  or signed out, the body is phase 2's.
- A cloud machine's `state` is the broker's (task 19): `provisioning | running | idle | sleeping |
  error`; `message` only with `error`. The list is cached **5 s** per process, so the desktop's
  poll does not become a broker call per tick.
- `signed_out`: the token source said so (`remotebe.ErrSignedOut`). `unavailable`: any other
  failure. Neither is an error status: `GET /v1/machines` still answers 200 with the local machine.

```jsonc
// POST /v1/machines/default — no body. Proxies the broker's ensure-default (task 19).
// 200 { machine } existing · 201 { machine } created · 503 backend_unavailable (signed out or
// broker down; details.cloud = "signed_out" | "unavailable") · 429 quota_exceeded.
// The desktop calls it once when it sees cloud.status == "ok" and no cloud machine (task 26).

// POST /v1/machines/{machineId}/wake — no body. 202 { machine } · 404 machine_not_found · 503.
```

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
	// first if needed. Local: phase 2's host mounts (sessionMounts). Cloud: two
	// MachineMounts — the worktree's machine path at /home/dev/work, and
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
   `GET /v1/worktrees/{worktreeId}/path` → `machinePath` (404 → the existing
   `worktree_not_found`).
2. If the machine is `sleeping`: `POST …/wake`, then poll the machine every 500 ms until `idle` or
   `running`, bounded by the ready timeout. The session record is `starting` throughout, so the
   cockpit's inline "waking…" (task 26) is truthful.
3. `conversationStoreFor(machine).Mounts(worktree)` → `MachineMounts`; `WorkDir: sessionWorkDir`
   (`sessions.go:33`), `Mounts: nil`, `Machine: machineID`.
4. `be.Create` — the broker answers after the pod is ready (task 18).

An `ErrQuotaExceeded` from the backend is `429 quota_exceeded` with the broker's
`details.limit`; `ErrUnavailable` wrapping `ErrSignedOut` is `503 backend_unavailable` with
`details.cloud = "signed_out"`.

**Forget and worktree removal (OD-10).** If the phase 2 OD-10 follow-up has landed, its deletes
go through `conversationStoreFor` in this task (`git grep` for its delete call first); if it has
not, the local implementation of `Forget`/`Remove` is written here to the same behaviour and the
follow-up uses the interface. Either way a cloud session's Forget reaches the machine agent's
`DELETE /v1/conversations/…` (task 20).

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMachinesWithoutABrokerIsPhase2sBody` | no `cloud` key; one machine, `default: true` — the byte-level JSON of today | a phase 2 client broken by phase 8 |
| `TestCloudMachinesAreListedWithTheirState` | a stand-in broker with one sleeping default → two machines, the cloud one `default`, `isolation: microvm`, `state: sleeping` | the `unsandboxed` badge on a microVM (S8.5) |
| `TestSignedOutListsLocalWithAReason` | `SignedOut{}` → 200, local only, `cloud.status == "signed_out"` and a message | a 503 that blanks the rail when the token expires |
| `TestTheMachineListIsCached` | ten GETs within 5 s → one broker call | a broker call per poll tick per user |
| `TestEnsureDefaultIsAPost` | `GET /v1/machines` never calls the broker's ensure-default; `POST /v1/machines/default` does | a side-effecting GET (`routes_audit_test.go:133`'s rule) |
| `TestCatalogWithoutMachineIDIsUnchanged` | every existing catalog test passes; local objects gain only `machineId: "local"` | a changed phase 2 shape |
| `TestCloudCatalogComesFromTheMachineAgent` | `?machineId=m-…` → the stand-in agent's projects, each with `machineId` | a cloud catalog read from the laptop's disk |
| `TestASleepingMachinesCatalogIs409AndDoesNotWake` | 409, `details.state = "sleeping"`, no wake call | a GET that boots a VM |
| `TestCloudCreateSendsMachineMountsNeverHostMounts` | the fake backend received `Mounts == nil`, two `MachineMounts` with the agent's path and `conversations/<w>`, `WorkDir == "/home/dev/work"` | S8.3 "fixed" by dropping katabe's refusal |
| `TestCloudCreateMakesTheStoreBeforeTheSandbox` | the agent saw `POST /v1/dirs` for `conversations/<w>` before the backend saw `Create` | a root-owned store the agent cannot write |
| `TestCreateOnASleepingMachineWakesItFirst` | wake, then polls, then `Create`; the record was `starting` meanwhile | "machine asleep" shown to a user who just typed |
| `TestQuotaIs429WithTheLimit` | `ErrQuotaExceeded` → 429 `quota_exceeded`, `details.limit` | a quota shown as "unavailable" |
| `TestLocalCreateIsUnchanged` | a local create still sends phase 2's host mounts (`TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv`, `sessions_test.go:749`, passes unchanged) | the local machine broken by the cloud branch |
| `TestCloudForgetReachesTheMachineAgent` | Forget on a cloud session → `DELETE /v1/conversations/<w>/<id>` at the agent | OD-10 honoured on the laptop only |

#### Steps

- [ ] **Step 1:** Tests; they fail.
- [ ] **Step 2:** `machines/agent.go` (typed wrappers over `Client.Stream` for the agent routes);
  `api/machines.go`; the catalog's `machineId`; `cloudstore.go`; the create path.
- [ ] **Step 3:** Register the two POST routes through the recorder (seam SX.4); update both audit
  maps; full run; commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1        # ok
go test ./control-plane/internal/api/ -run 'TestRoutes|TestNoRegisteredGet' -count=1 -v   # the audits pass with the two new POSTs
```

#### What this task does not claim

- **Anything on screen**: task 26.
- That the UI contract's `types.ts` already carries `machineId` on projects and the `cloud` status —
  task 26 amends `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` §2 and `types.ts`
  together (rule SX.2).
- Adding a project to a cloud machine (a clone through task 21's route): seam S8-P3.2.
