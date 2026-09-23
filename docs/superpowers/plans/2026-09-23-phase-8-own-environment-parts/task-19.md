### Task 19: Broker machines — a disk, a machine pod, the states, sleep and wake, the agent proxy

`docs/FUNCTIONAL-SPEC.md:42-44` defines the machine the user sees: "One cloud machine exists at
first sign-in with no create form … States: `provisioning · running · idle · sleeping · stopped ·
error`. **Wake is implicit**: typing into a sleeping machine's session wakes it, with inline
'waking… 6s'." Phase 2 returns only the local machine and only `running` or `error`, and the Go
contract holds the full enum for this phase (`2026-09-18-phase-2-go-contract.md:1590-1600`, open
question 7 at `:2286-2290`).

P8-D6 makes a cloud machine **a disk plus a machine pod**; this task makes the broker own that
lifecycle. **Kubernetes objects are the broker's only state** — there is no broker database: a
machine *is* its PersistentVolumeClaim, labelled with its owner, and everything else is derived.

**Files:**
- `control-plane/internal/sandbox/machine.go`, `machine_test.go` — the machine's objects (new; `manager.go` is untouched, per the plan's shared-file rule)
- `control-plane/internal/brokerapi/wire.go` — machine types (modify)
- `control-plane/cmd/broker/machines.go`, `agentproxy.go`, `sleeper.go` and tests; `machinecheck.go` gains the real implementation (new/modify)
- `control-plane/cmd/broker/server.go` — routes (modify)

#### The objects of one machine (all in `panacea-sandboxes`, all labelled `panacea-app/component: machine`, `panacea-app/machine-id: m-…`, `panacea-app/owner: …`)

| Object | Name | What |
|---|---|---|
| PersistentVolumeClaim | `machine-<id>` (`sandbox.MachineClaimName`, task 15) | `ReadWriteOnce`, `MACHINE_STORAGE_CLASS`, `MACHINE_DISK_GIB`. Annotations: `panacea-app/name`, `panacea-app/default: "true"` on the default machine, `panacea-app/idle-since` (RFC 3339, set by the sleeper) |
| Secret | `machine-<id>-token` | `token`: 32 random bytes, hex; owner reference → the PVC, so deleting the disk collects it |
| Pod | `machine-<id>` | the machine image (digest), `command: ["/usr/local/bin/machined"]`, env `PANACEA_APP_MACHINED_TOKEN` from the Secret and `PANACEA_APP_MACHINED_ROOT=/machine`; the PVC at `/machine`; port `8081`; readiness `httpGet /healthz :8081`; `runtimeClassName`; `priorityClassName: panacea-machine`; the sandbox toleration and node selector; task 14's hardening (no SA token, seccomp, non-root 1001, `fsGroup: 1001`, read-only root with `emptyDir` `/tmp`); requests = limits = `500m` / `1Gi`; `safe-to-evict: "false"` |
| PodDisruptionBudget | `machine-<id>` | selects `panacea-app/machine-id: <id>`, `maxUnavailable: 0` — the machine pod **and its sessions** are never voluntarily evicted; a node-image upgrade cordons the node instead (task 06's `undrainable_node_behavior = "Cordon"`) and the machine moves at its next wake |

#### State, derived (the only derivation — `machineState` in `machines.go`)

| Condition, checked in order | `state` | `message` |
|---|---|---|
| PVC `Lost`, or machine pod `Failed` | `error` | one sentence naming which |
| PVC `Pending`, or pod exists and is not Ready | `provisioning` | — |
| no machine pod | `sleeping` | — |
| pod Ready, ≥ 1 session pod labelled with this machine | `running` | — |
| pod Ready, no session pod | `idle` | — |

`stopped` is never emitted in v1: a user who stops a machine puts it to sleep; nothing else
exists to distinguish. (Recorded; the enum keeps the value.)

#### Contract — wire types added to `brokerapi` (complete)

```go
// Machine is one cloud machine as the broker reports it.
type Machine struct {
	ID        string    `json:"id"`      // m- + 8 hex, minted by the broker
	Name      string    `json:"name"`    // "Cloud machine" for the default; the user's text otherwise
	State     string    `json:"state"`   // provisioning | running | idle | sleeping | error
	Message   string    `json:"message,omitempty"`
	Default   bool      `json:"default"`
	DiskGiB   int       `json:"diskGib"`
	CreatedAt time.Time `json:"createdAt"`
}

type MachineList struct {
	Machines []Machine `json:"machines"`
}

// CreateMachine is POST /v1/machines' body.
type CreateMachine struct {
	Name string `json:"name"` // 1-40 printable characters
}
```

#### Contract — the routes

| Route | Success | Failures |
|---|---|---|
| `GET /v1/machines` | `200 MachineList`, this owner's | 503 |
| `POST /v1/machines/default` | ensures the owner's default machine: **201** + `Machine` if it had to create it, **200** if it existed; a sleeping one is woken. Idempotent — a retried first sign-in never makes two | 429 `quota_exceeded`, 503 |
| `POST /v1/machines` | `201 Machine` (a second machine: FUNCTIONAL-SPEC's "sizing is only ever asked for a second machine") | 400, 429, 503 |
| `GET /v1/machines/{id}` | `200 Machine` | 404 `machine_not_found` (also another owner's) |
| `POST /v1/machines/{id}/wake` | `202 Machine` — returns at once with `provisioning`; the caller polls `GET` (implicit wake: "waking… 6s") | 404, 503 |
| `POST /v1/machines/{id}/sleep` | `202 Machine` (`sleeping` once the pod is gone) | 404, **409 `machine_busy`** while session pods exist |
| `DELETE /v1/machines/{id}` | `204` — pod, PDB, Secret and **the disk** | 404, 409 `machine_busy` |
| `GET\|POST\|DELETE /v1/machines/{id}/agent/{rest...}` | the machine agent's response, streamed | 404, **409 `machine_asleep`** (the proxy never wakes a machine — only `wake` does), 502 if the agent does not answer |

The agent proxy forwards `/{rest...}` to `http://<machine pod IP>:8081/{rest...}` with
`Authorization: Bearer <machine token>` read from the Secret; the caller's `Authorization` is
already gone (task 16's middleware); request bodies are capped at 16 MiB; responses stream
unbuffered. `GET` on the proxy is safe to allow in the read-route audit because every `GET` the
machine agent serves is itself read-only, and task 20 pins that with its own audit test.

`machineChecker.Awake` (task 18's seam): the PVC exists for the owner → else `errMachineNotFound`;
its machine pod is Ready → else `errMachineAsleep`.

#### The sleeper (`sleeper.go`)

Every minute, over **all** owners (the unscoped manager): for each machine whose pod is Ready and
which has no session pod, set `panacea-app/idle-since` if absent; clear it when a session pod
exists; delete the machine pod when `now − idle-since ≥ MACHINE_IDLE_SLEEP`. `0` disables it. The
sleeper never deletes a disk and never touches a machine with a session pod. Sessions themselves
are ended by the laptop's reaper (`PANACEA_APP_IDLE_TIMEOUT`), which is where session idleness is
known.

#### Tests to write first

`sandbox/machine_test.go`: `TestMachineDiskIsRWOAtTheConfiguredSizeAndClass`;
`TestMachineTokenSecretIsOwnedByTheDisk` (kills a token that outlives its machine);
`TestMachinePodMountsTheDiskAtMachineRootAndReadsTheTokenFromTheSecret` (no token literal in the
pod spec); `TestMachinePodIsHardenedLikeASession` (the task 14 fields, kills a machine pod that
fails `restricted`); `TestMachinePDBProtectsTheMachineAndItsSessions` (selector on machine-id,
`maxUnavailable: 0`); `TestDeleteMachineRemovesAllFourObjects`.

`cmd/broker/machines_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEnsureDefaultCreatesOnceAndReturns200After` | first call 201, second 200 with the same id; one PVC | two default machines after a retried sign-in |
| `TestEnsureDefaultWakesASleepingDefault` | PVC with no pod → 200 and a machine pod created | a sleeping machine the user must wake by hand at sign-in |
| `TestStateDerivation` | the five rows of the table, each from a constructed PVC/pod set | one state computed two ways |
| `TestStoppedIsNeverEmitted` | no constructed input yields `stopped` | an undefined state shown in the rail |
| `TestWakeReturns202Immediately` | the response arrives before the pod is Ready, state `provisioning` | a wake that blocks the cockpit for a cold start |
| `TestSleepRefusesWhileSessionsRun` | a session pod labelled with the machine → 409 `machine_busy`, pod intact | an agent killed by a sleep |
| `TestDeleteRefusesWhileSessionsRunAndOtherwiseDeletesTheDisk` | 409 with a session; 204 without, and the PVC is gone | a disk deleted under a running agent |
| `TestSecondMachineRespectsTheCap` | at `MAX_MACHINES_PER_USER` → 429 | unbounded disks per user |
| `TestAnotherOwnersMachineIs404OnEveryRoute` | B on A's id: get, wake, sleep, delete, agent → 404 | cross-user machine access |
| `TestAgentProxyNeverWakes` | sleeping machine → 409 `machine_asleep`, no pod created | a GET that creates a pod (the audit rule) |
| `TestAgentProxySendsTheMachineTokenNotTheCallers` | the stand-in agent sees `Bearer <machine token>` only | the Entra token forwarded into a Kata pod |
| `TestAgentProxyStreamsLargeResponses` | a 32 MiB response arrives whole and starts before the agent finishes | a buffered proxy that holds a git log in memory |

`cmd/broker/sleeper_test.go`: `TestAnIdleMachineSleepsAfterTheWindow` (clock injected),
`TestASessionClearsIdleSince`, `TestTheSleeperNeverDeletesADisk`, `TestZeroDisablesTheSleeper`.

#### Steps

- [ ] **Step 1:** `sandbox/machine.go` tests, then the code.
- [ ] **Step 2:** Wire types; broker tests; `machines.go`, `agentproxy.go`, `sleeper.go`, the real
  `machineChecker`; routes into `server.go`; update the read-route allowlist.
- [ ] **Step 3:** Full run; commit.

#### Acceptance

```bash
go test ./control-plane/internal/sandbox/ ./control-plane/cmd/broker/ ./control-plane/internal/brokerapi/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                                  # ok
```

#### What this task does not claim

- "Claimed from a warm pool" (FUNCTIONAL-SPEC `:44`) **for the disk**: a first machine provisions
  a new disk, which takes Azure some seconds. Node-warm (task 12) covers the pod. A pool of
  pre-provisioned, unclaimed disks is a later refinement if task 32's first-sign-in timing asks
  for it.
- That RWO disk sharing between the machine pod and its sessions works under Kata (P8-D6, task 32).
- That a cordoned node with a PDB-protected machine is ever upgraded: it is, when the machine
  next sleeps. How long nodes stay cordoned in practice is a task 32 observation.
