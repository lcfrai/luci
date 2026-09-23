### Task 23: `internal/machines` — one control plane, a backend per machine

Phase 2's control plane has exactly one backend (`api.Deps.Backend`, `internal/api/api.go:72-74`)
and one machine, the literal `"local"` (`api.go:43-45`); a session already records its
`MachineID` (`registry/session.go:45`, defaulted in `sessions.go:370-375`) but nothing ever routes
by it. Every session-path call goes to `a.deps.Backend` — create (`sessions.go:422`, `:1010`),
destroy (`:616`, `:945`, `:1080`), attach (`attach.go:148`, `:199`) — and the reaper and start-up
recovery take one backend too (`cmd/api/main.go:202-212`, `:277`).

This task makes the machine id **route**: sessions on `"local"` go to today's backend, unchanged;
sessions on `m-…` go to that machine's `remotebe.Backend`. It changes no route, no JSON and no
status code, and with no broker configured the control plane behaves byte-for-byte as in phase 2.
Task 24 then puts cloud machines in front of the user.

**Files:**
- `control-plane/internal/machines/{machines.go,cloud.go}` and `machines_test.go` (new)
- `control-plane/internal/api/api.go` — `Deps.Machines`, `backendFor` (modify)
- `control-plane/internal/httpx/problem.go` — `CodeMachineNotFound = "machine_not_found"` (404), the seventeenth code (modify)
- `control-plane/internal/api/sessions.go`, `attach.go` — route by `MachineID`; the readiness watch prefers `backend.ReadyWaiter` (modify)
- `control-plane/internal/reaper/reaper.go` and its tests — sweep per machine (modify)
- `control-plane/cmd/api/config.go`, `main.go` and tests — `PANACEA_APP_BROKER_URL`, `PANACEA_APP_BROKER_SCOPE`; build the set (modify)

#### Contract (complete)

```go
// Package machines is the set of machines one control plane drives: the local
// machine, always, and the signed-in user's cloud machines when a broker is
// configured. It is the only place a machine id becomes a backend.
package machines

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/remotebe"
)

// LocalID is the local machine's id — the same literal as api.LocalMachineID,
// which is redefined as this constant so there is one spelling.
const LocalID = "local"

// ErrUnknownMachine: no machine with that id, for this user. The API maps it
// to 404 machine_not_found: this task adds httpx.CodeMachineNotFound.
var ErrUnknownMachine = errors.New("machines: no such machine")

// Set resolves machine ids to backends. It is safe for concurrent use.
type Set struct{ /* unexported: local backend, optional *remotebe.Client, per-id backend cache */ }

// New returns a set. cloud may be nil: then the set is the local machine only
// and every non-local id is ErrUnknownMachine — phase 2's behaviour exactly.
func New(local backend.Backend, cloud *remotebe.Client) *Set

// Backend returns the backend for id. "" and LocalID are the local backend. A
// cloud id is accepted if it matches ^m-[0-9a-f]{8}$ and the broker has
// confirmed it for this user once (GET /v1/machines/{id}, cached for the
// process's life, forgotten on a 404); its backend is a remotebe.NewBackend,
// cached per id.
func (s *Set) Backend(ctx context.Context, id string) (backend.Backend, error)

// Local is the local backend.
func (s *Set) Local() backend.Backend

// Cloud returns the cloud client, or nil when no broker is configured.
func (s *Set) Cloud() *remotebe.Client

// Known returns every cloud machine id the set has resolved, for the reaper.
func (s *Set) Known() []string
```

```go
// package api — Deps gains one field; nothing else in Deps changes.
//
//	// Machines routes a session's MachineID to its backend. nil means the
//	// local machine only (every existing test), and then backendFor returns
//	// Backend for "" and "local" and ErrUnknownMachine otherwise.
//	Machines *machines.Set

// backendFor is the ONLY way a session-path handler reaches a backend.
func (a *API) backendFor(ctx context.Context, machineID string) (backend.Backend, error)
```

```go
// package reaper — Config gains:
//
//	// Machines, when set, makes the sweep per machine: sessions whose
//	// MachineID is "local" (or "") are reconciled against Backend exactly as
//	// before; sessions on each id in Machines.Known() against that machine's
//	// backend. A cloud backend that fails to List (signed out, broker down) is
//	// SKIPPED — its sessions are left exactly as they are, the rule main.go's
//	// recoverSessions already states for a backend that does not answer
//	// (cmd/api/main.go:286-293). Only an explicit ErrUnknownMachine (the broker
//	// said 404: the machine was deleted) marks that machine's sessions exited,
//	// with ExitReason "the cloud machine was deleted".
//	Machines *machines.Set
```

Configuration added to `cmd/api/config.go` (neither is backend-scoped: they add cloud machines
beside whichever local backend is selected):

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_BROKER_URL` | `""` | the broker, `https://broker.<zone>`; empty = no cloud machines |
| `PANACEA_APP_BROKER_SCOPE` | `""` | required when the URL is set: task 09's `broker_api_scope` |

`main.go`: when the URL is set, `remotebe.NewClient` with the Hub's token source adapted to
`remotebe.TokenSource` if the Hub's C1 has landed, else `remotebe.SignedOut{}`; `machines.New(be,
client)`; `Deps.Machines`; the reaper's `Machines`; `recoverSessions` per machine with the same
leave-it-alone rule.

#### What changes in the handlers (each a one-line swap, listed so review is mechanical)

| Site | Before | After |
|---|---|---|
| create, `sessions.go:398` | `Backend: string(a.deps.Backend.Kind())` | the resolved backend's `Kind()` |
| create, `sessions.go:422` | `a.deps.Backend.Create` | `be.Create`, `be` from `backendFor(ctx, machineID)`; an `ErrUnknownMachine` is a 404 before any record is written |
| create/restart destroys, `sessions.go:616`, `:945`, `:1080` | `a.deps.Backend.Destroy` | `backendFor(sess.MachineID)` |
| restart, `sessions.go:1010` | `a.deps.Backend.Create` | as create |
| attach lookup and dial, `attach.go:148`, `:199` | `a.deps.Backend.Get` / `.Attach` | `backendFor(sess.MachineID)` |
| readiness, `sessions.go:569-610` | always `sessionWaitReady(endpoint)` | if the session's backend implements `backend.ReadyWaiter`, `WaitReady(id, timeout)`; else unchanged. The empty-endpoint SECURITY branch (`:597-602`) is untouched and still runs first |

`/healthz`'s `backend` field and `handleMachines` keep reporting the **local** backend in this
task; task 24 changes `handleMachines`.

#### Tests to write first

`machines_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLocalAndEmptyResolveToTheLocalBackend` | `""` and `"local"` → the local backend | a session with no machine id (every phase 2 record) routed nowhere |
| `TestWithoutABrokerEveryCloudIDIsUnknown` | `New(local, nil)`, `"m-0123abcd"` → `ErrUnknownMachine` | a remotebe built with no broker |
| `TestACloudIDIsConfirmedOnceThenCached` | two resolutions → one `GET /v1/machines/{id}` at the stand-in | a broker call on every keystroke's attach |
| `TestAMalformedIDNeverReachesTheBroker` | `"m-../x"` → `ErrUnknownMachine`, zero requests | an id interpolated into a URL path |
| `TestA404ForgetsTheMachine` | stand-in answers 404 after deletion → `ErrUnknownMachine`, and `Known()` drops it | a deleted machine resolved from cache forever |

`api` tests (existing harness, a fake cloud backend injected through a `machines.Set` built on a
stand-in broker):
- `TestCreateOnACloudMachineUsesItsBackend` — the fake cloud backend saw the Create; the local did
  not; the record's `backend` is `kata`.
- `TestCreateOnAnUnknownMachineIs404AndWritesNoRecord` — kills a record orphaned before the
  backend was resolved.
- `TestAttachUsesTheSessionsMachine` and `TestDeleteUsesTheSessionsMachine`.
- `TestReadinessPrefersTheReadyWaiter` — a backend implementing it is asked; `sessionWaitReady` is
  never called with `remote:` (kills a laptop dialling `http://remote:s-…/healthz`).
- **Every existing api test passes unchanged** with `Deps.Machines == nil` — that is the proof of
  "byte-for-byte phase 2 without a broker".

`reaper` tests: `TestTheSweepIsPerMachine` (a cloud session is not reconciled against the local
list — kills every cloud session marked exited because Docker does not know it);
`TestAnUnreachableCloudMachineIsSkipped` (List error → its sessions untouched);
`TestADeletedMachineEndsItsSessions` (`ErrUnknownMachine` → exited with the reason).

`cmd/api` config tests: `TestBrokerURLNeedsAScope`, `TestBrokerURLMustBeHTTPS`,
`TestNoBrokerIsPhase2`.

#### Steps

- [ ] **Step 1:** `machines` package and its tests.
- [ ] **Step 2:** `Deps.Machines`, `backendFor`, the handler swaps from the table — one commit-sized
  diff; run the whole `api` suite after each file.
- [ ] **Step 3:** The reaper; `main.go` wiring; config.
- [ ] **Step 4:** Full run; `git grep -n "a.deps.Backend\." -- control-plane/internal/api/` lists
  only `/healthz` and `handleMachines`; commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1                 # ok — every phase 2 test unchanged
git grep -n "a.deps.Backend\." -- control-plane/internal/api/ ':!*_test.go'
# catalog.go:… (healthz)   catalog.go:… (handleMachines)   — and nothing else
```

#### What this task does not claim

- That a user can see or choose a cloud machine — task 24 lists them, task 26 draws them.
- That the Hub's token source exists: until C1 lands, every cloud call is signed-out, and the
  reaper skips cloud machines, which is the safe reading.
