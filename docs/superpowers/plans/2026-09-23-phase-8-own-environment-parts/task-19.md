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
| PersistentVolumeClaim | `machine-<owner[:8]>-<id>` (`sandbox.MachineClaimName(owner, id)`, task 15) | `ReadWriteOnce`, `MACHINE_STORAGE_CLASS`, `MACHINE_DISK_GIB`. Annotations: `panacea-app/name`, `panacea-app/default: "true"` on the default machine, `panacea-app/idle-since` (RFC 3339, set by the sleeper) |
| Secret | `machine-<owner[:8]>-<id>-token` | `token`: 32 random bytes, hex; owner reference → the PVC, so deleting the disk collects it |
| Pod | `machine-<owner[:8]>-<id>` | the machine image (digest), `command: ["/usr/local/bin/machined"]`, env `PANACEA_APP_MACHINED_TOKEN` from the Secret and `PANACEA_APP_MACHINED_ROOT=/machine`; the PVC at `/machine`; port `8081`; readiness `httpGet /healthz :8081`; `runtimeClassName`; `priorityClassName: panacea-machine`; the sandbox toleration and node selector; task 14's hardening (no SA token, seccomp, non-root 1001, `fsGroup: 1001`, read-only root with `emptyDir` `/tmp`); requests = limits = `500m` / `1Gi`; `safe-to-evict: "false"` |
| PodDisruptionBudget | `machine-<owner[:8]>-<id>` | selects `panacea-app/machine-id: <id>` **and** `panacea-app/owner: <owner>`, `maxUnavailable: 0` — the machine pod **and its sessions** are never voluntarily evicted; a node-image upgrade cordons the node instead (task 06's `undrainable_node_behavior = "Cordon"`) and the machine moves at its next wake |

**Names carry the owner, as task 14's sandbox names do (review m12).** Every name is
`machine-<owner[:8]>-<id>`, where `owner` is `OwnerLabelValue` (task 14) and `owner[:8]` is the same
prefix task 14 puts in `sandbox-<owner[:8]>-<id>`. A bare `machine-<id>` let one owner's create
collide with another owner's object, and the `AlreadyExists` then proved that the other owner's
id existed. With the prefix, a name can collide only with this owner's own objects (or, with odds
near 2^-64, an owner sharing both the prefix and the id). The longest name, the Secret's, is
8 + 8 + 1 + 10 + 6 = 33 characters. Every selector on `panacea-app/machine-id` (the PDB, the
session count in `machineState`, the sleeper, task 15's pod affinity) also selects on
`panacea-app/owner`, so two owners whose ids happen to match never share a PDB or a node
affinity. A second machine's random id that still meets `AlreadyExists` is retried once with a
fresh id, then answers 503; it never answers with anything read from the object it collided with.

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
	ID        string    `json:"id"`      // m- + 8 hex: derived for the default (defaultMachineID), random for a second machine
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
| `POST /v1/machines/default` | ensures the owner's default machine: **201** + `Machine` if it had to create it, **200** if it existed; a sleeping one is woken. Idempotent — a retried first sign-in never makes two, and neither do two sign-ins racing on two broker replicas: the id is derived (below), so the loser's create is `AlreadyExists` and answers 200 | 429 `quota_exceeded`, 503 |
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

#### Two replicas: no check-then-create (review m11)

The broker runs two replicas (task 29), each with its own clients, so nothing may rely on a
check made before a create:

- **The default machine's id is derived:** `defaultMachineID(owner) = "m-" + the first 8 hex of
  sha256(owner + "/default")`. Ensure-default creates the PVC by that name and never lists first
  to decide. `Created` → 201. `AlreadyExists` → read the PVC; if its `panacea-app/owner` label is
  this owner's and it is annotated `panacea-app/default: "true"`, → 200 (and wake it if it is
  sleeping). Anything else answers 503 and is logged, never shown.
- **Every create is `AlreadyExists`-tolerant.** The PVC's winner goes on to create the Secret, the
  pod and the PDB, while a loser (or a wake on the other replica) may reach the same objects. An
  `AlreadyExists` on any of them means "done". The token is only ever read back from the Secret,
  never from what this request generated, so a Secret written by the other replica is the one
  every reader uses.
- **Caps are re-counted after the create.** `MAX_MACHINES_PER_USER`: after its PVC create
  succeeds, the request lists the owner's claims with a **consistent read** (no
  `resourceVersion`, never an informer cache) and counts those without a `deletionTimestamp`.
  Over the cap → delete the claim it just created (and anything else it created) → 429
  `quota_exceeded`. Two racers for the last slot either both see the excess and both roll back,
  which fails closed and is retryable, or the later one sees the earlier and only it rolls back.
  Neither leaves the owner over the cap. The count before the create stays, as the cheap early
  refusal. Task 18 applies the same rule to `MAX_SANDBOXES_PER_USER`.
- **Wake on a terminating pod** (the sleeper's delete on the other replica, a `deletionTimestamp`
  set): a create that meets the terminating pod's name reads `AlreadyExists`, which here does
  **not** mean "done". The wake still answers 202 `provisioning` at once, and re-creates the pod
  in the background once the old one is gone, bounded by `READY_TIMEOUT`. If the replica dies
  first, the machine reads `sleeping` and the cockpit's next implicit wake does it.
- **The sleeper runs on one replica only**, below.

`machineChecker.Awake` (task 18's seam): the PVC exists for the owner → else `errMachineNotFound`;
its machine pod is Ready → else `errMachineAsleep`.

#### The sleeper (`sleeper.go`)

Every minute, over **all** owners (the unscoped manager): for each machine whose pod is Ready and
which has no session pod, set `panacea-app/idle-since` if absent; clear it when a session pod
exists; delete the machine pod when `now − idle-since ≥ MACHINE_IDLE_SLEEP`. `0` disables it. The
sleeper never deletes a disk and never touches a machine with a session pod. Sessions themselves
are ended by the laptop's reaper (`PANACEA_APP_IDLE_TIMEOUT`), which is where session idleness is
known.

**One sleeper across replicas (review m11).** Each replica runs the loop only while it holds the
`coordination.k8s.io/v1` Lease `broker-sleeper` in the broker's own namespace
(`PANACEA_APP_BROKER_LEASE_NAMESPACE`, task 17), through client-go's
`tools/leaderelection` with a `resourcelock.LeaseLock` (already in the broker's `client-go`, no
new module). Identity is the pod name (`PANACEA_APP_BROKER_POD_NAME`); lease 30 s, renew
deadline 20 s, retry 5 s; `ReleaseOnCancel: true`, so a rolling update hands over at once. A
replica that loses the Lease stops the loop and contends again; it keeps serving requests, and
never exits for it. It logs `sleeper lease acquired` and `sleeper lease lost` at info, with its
identity, so task 31 §7a can see which replica holds it. `MACHINE_IDLE_SLEEP = 0` starts no
elector. Two sleepers without it would
both write `idle-since` and both delete the same pod, and each would act on a list the other had
already changed.

The sleeper reads a machine's session pods again, by consistent read, just before it deletes the
machine pod, and deletes only if there are still none. A session created in the milliseconds
between that read and the delete keeps its disk mount. Its machine then reads `sleeping`, and the
cockpit's implicit wake brings the agent back (recorded, not closed: closing it needs a lock that
every session create takes).

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
| `TestConcurrentEnsureDefaultMakesOne` | two broker servers on **one** fake clientset, one `POST /v1/machines/default` each for one owner, with a `PrependReactor` on PVC `create` that holds each create at a barrier until both requests reach it, so the interleaving is forced, not hoped for → exactly one 201, the other 200, every body the same id (`defaultMachineID(owner)`), one PVC, one Secret, one pod, one PDB (review m11) | a minted id with a list-then-create, where two sign-ins on two replicas make two default machines |
| `TestDefaultMachineIDIsDerivedFromTheOwner` | the same owner → the same id on every call and replica; two owners → different ids; the form is `m-` + 8 hex | a random default id, which makes the race above possible again |
| `TestConcurrentSecondMachinesNeverExceedTheCap` | owner at `MAX_MACHINES_PER_USER - 1`, one `POST /v1/machines` on each of two servers, held at the same PVC-create barrier after both have made the early count → at most one 201, the other 429; afterwards the owner's claims ≤ the cap and no Secret, pod or PDB is left without its claim | a count-then-create cap that two replicas overrun together |
| `TestMachineNamesCarryTheOwner` | every object is `machine-<owner[:8]>-<id>` (the Secret `…-token`); two owners given the same id both succeed; the PDB's selector holds both labels | `machine-<id>`, where the second owner's create fails with a conflict that proves the first exists (review m12) |
| `TestAWakeOnATerminatingPodRecreatesIt` | a machine pod with a `deletionTimestamp` → wake answers 202; once the fake removes the old pod, a new one is created | a wake that treats the terminating pod's `AlreadyExists` as "done" and leaves the machine asleep |
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
`TestASessionClearsIdleSince`, `TestTheSleeperNeverDeletesADisk`, `TestZeroDisablesTheSleeper`
(and starts no elector), `TestOnlyTheLeaseHolderSleeps` (two sleepers on one fake clientset with
short lease timings: exactly one deletes the idle pod, one `Lease` exists; kills a sleeper started
outside the elector), `TestALostLeaseStopsTheLoop` (the holder's lease is taken over → it deletes
nothing more and its server still answers), `TestTheSleeperRereadsSessionsBeforeDeleting` (a
session pod added between the tick's list and its delete → the machine pod stays).

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
- That client-go's `LeaseLock` needs only `get`, `create` and `update` on `leases` (training
  knowledge, **UNVERIFIED**; task 29's Role grants exactly those, so a missing verb shows as a
  `forbidden` in the broker's log on the first install, and `TestOnlyTheLeaseHolderSleeps` cannot
  see RBAC). That a fake clientset's `AlreadyExists` on a duplicate name matches the API server's.
- That a sleeper-versus-new-session race is closed: it is narrowed to the window between the
  re-read and the delete, and recovered by the next wake (above).
- That a cordoned node with a PDB-protected machine is ever upgraded: it is, when the machine
  next sleeps. How long nodes stay cordoned in practice is a task 32 observation.
