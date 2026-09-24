### Task 24a: Cloud machines in the API — the list, ensure-default and wake

Task 23 made a machine id route. This task makes cloud machines **visible** through the control
plane's existing REST surface, every change additive. Task 24b then makes them usable (the
catalog, session create, the conversation store). The two were one task 24 until review m17
found the whole nearer 5 h than 3.

1. `GET /v1/machines` lists the user's cloud machines beside the local one, with the states the
   Go contract reserved for this phase (`2026-09-18-phase-2-go-contract.md:1593-1600`) and
   `isolation: "microvm"` (seam S8.5 — `isolationFor` already maps `kata` to it,
   `internal/api/catalog.go:174-183`).
2. `POST /v1/machines/default` proxies the broker's ensure-default (task 19), and
   `POST /v1/machines/{machineId}/wake` proxies its wake. Both are POSTs: a GET never creates or
   boots a machine.

**Files:**
- `control-plane/internal/api/machines.go` — `GET /v1/machines` (moved out of `catalog.go`), `POST /v1/machines/default`, `POST /v1/machines/{machineId}/wake` (new)
- `control-plane/internal/api/catalog.go` — `handleMachines` leaves; nothing else changes here (task 24b adds `machineId`) (modify)
- `control-plane/internal/api/api.go` — calls `machines.go`'s register function on the guarded mux beside `registerCatalog` (`catalog.go:40`), so the `*routes` recorder (seam SX.4) sees the moved GET and both POSTs (modify)
- `control-plane/internal/httpx/problem.go` — `CodeQuotaExceeded = "quota_exceeded"` (429) (modify; task 24b adds `machine_full` after it)
- `control-plane/internal/api/*_test.go`, both route-audit maps (modify/new)

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
  poll does not become a broker call per tick. A successful ensure-default or wake **drops the
  cache**, so the next poll shows the new machine or the waking state rather than 5 s of the old
  one.
- `signed_out`: the token source said so (`remotebe.ErrSignedOut`). `unavailable`: any other
  failure. Neither is an error status: `GET /v1/machines` still answers 200 with the local machine.

```jsonc
// POST /v1/machines/default — no body. Proxies the broker's ensure-default (task 19).
// 200 { machine } existing · 201 { machine } created · 503 backend_unavailable (signed out or
// broker down; details.cloud = "signed_out" | "unavailable") · 429 quota_exceeded.
// The desktop calls it once when it sees cloud.status == "ok" and no cloud machine (task 26).

// POST /v1/machines/{machineId}/wake — no body. 202 { machine } · 404 machine_not_found · 503.
```

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMachinesWithoutABrokerIsPhase2sBody` | no `cloud` key; one machine, `default: true` — the byte-level JSON of today | a phase 2 client broken by phase 8 |
| `TestCloudMachinesAreListedWithTheirState` | a stand-in broker with one sleeping default → two machines, the cloud one `default`, `isolation: microvm`, `state: sleeping` | the `unsandboxed` badge on a microVM (S8.5) |
| `TestSignedOutListsLocalWithAReason` | `SignedOut{}` → 200, local only, `cloud.status == "signed_out"` and a message | a 503 that blanks the rail when the token expires |
| `TestTheMachineListIsCached` | ten GETs within 5 s → one broker call | a broker call per poll tick per user |
| `TestEnsureDefaultIsAPost` | `GET /v1/machines` never calls the broker's ensure-default; `POST /v1/machines/default` does | a side-effecting GET (`routes_audit_test.go:133`'s rule) |
| `TestEnsureDefaultAnswersWithTheBrokersOutcome` | broker "existing" → 200; "created" → 201; `ErrQuotaExceeded` → 429 `quota_exceeded` with `details.limit`; signed out → 503 `backend_unavailable`, `details.cloud = "signed_out"` | a created machine reported as existing; a quota shown as "unavailable" |
| `TestWakeIsAPostThatReachesTheBroker` | `POST /v1/machines/m-…/wake` → exactly one broker wake, 202 with the machine; an id the machine set does not know → 404 `machine_not_found`, no broker call | a 202 that never woke anything |
| `TestEnsureDefaultAndWakeDropTheListCache` | a cached list, then either POST, then `GET /v1/machines` → a fresh broker call | the rail showing `sleeping` for 5 s after the user woke the machine |

#### Steps

- [ ] **Step 1:** Tests; they fail.
- [ ] **Step 2:** `api/machines.go`; `handleMachines` out of `catalog.go`; `CodeQuotaExceeded`.
- [ ] **Step 3:** Register the two POST routes through the recorder (seam SX.4); update both audit
  maps; full run; commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1        # ok
go test ./control-plane/internal/api/ -run 'TestRoutes|TestNoRegisteredGet' -count=1 -v   # the audits pass with the two new POSTs
```

#### What this task does not claim

- **Anything on screen**: task 26.
- A cloud machine's catalog or a session on it: task 24b.
- That the UI contract's `types.ts` already carries the `cloud` status — task 26 amends
  `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` §2 and `types.ts` together
  (rule SX.2).
