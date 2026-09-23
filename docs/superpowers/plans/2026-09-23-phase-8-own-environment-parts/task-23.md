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
- `control-plane/internal/machines/{machines.go,cloud.go,installid.go}` and `machines_test.go`, `installid_test.go` (new)
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

// InstallID returns this control plane's install id: 32 lowercase hex
// characters from crypto/rand, kept in <dir>/install-id with mode 0600 (dir is
// statepath.Dir("machines")). The first call creates the file (O_CREATE|O_EXCL,
// then fsync, so two starts racing agree on one id); every later call reads it.
// A file that exists but does not hold exactly one valid id is an error naming
// the file, and it is never replaced: a new id would turn every running cloud
// sandbox of this laptop into another control plane's (review I6).
func InstallID(dir string) (string, error)
```

**Why an install id (review I6).** The broker scopes every route by **owner** (`tid/oid`,
task 14), and one owner can run two control planes: on a second PC, or after a reinstall that
starts a fresh registry. The reaper destroys any listed sandbox that its registry does not name
(rule 4, `destroyOrphan`, `reaper.go:452-463`). Scoped by owner alone, the second control plane
would list the first one's running cloud sessions, find no record of them, and destroy them
within one reap interval. So every cloud sandbox carries its creator's install id as the label
`panacea-app/control-plane` (task 18), remotebe asks the broker for its own only (task 22), and
the reaper acts only on its own (below). The id is created only when `PANACEA_APP_BROKER_URL` is
set, so without a broker phase 2 is unchanged byte-for-byte.

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
//
//	// InstallID is this control plane's install id; required when Machines
//	// has a cloud client. On a cloud machine, rule 4 (a listed sandbox with
//	// no record) destroys a sandbox only if its
//	// Labels[backend.KubeLabelControlPlane] equals InstallID. Any other
//	// sandbox, one with no such label included, is left running and logged
//	// once per id at info ("left a cloud sandbox another control plane
//	// created"). The broker already filters the list (task 22's query); this
//	// is the second check, so a broker that ignored the query cannot make this
//	// laptop destroy another's sessions. Rules 1-3 and the local backend's
//	// sweep are unchanged.
//	InstallID string
```

Configuration added to `cmd/api/config.go` (neither is backend-scoped: they add cloud machines
beside whichever local backend is selected):

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_BROKER_URL` | `""` | the broker, `https://broker.<zone>`; empty = no cloud machines |
| `PANACEA_APP_BROKER_SCOPE` | `""` | required when the URL is set: task 09's `broker_api_scope` |

`main.go`: when the URL is set, `machines.InstallID(statepath.Dir("machines"))` first (an error
exits 2 naming the file); `remotebe.NewClient` with that id as `ControlPlane` and the Hub's token
source adapted to `remotebe.TokenSource` if the Hub's C1 has landed, else `remotebe.SignedOut{}`;
`machines.New(be, client)`; `Deps.Machines`; the reaper's `Machines` and `InstallID`;
`recoverSessions` per machine with the same leave-it-alone rule.

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

`installid_test.go` (review I6):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestInstallIDIsCreatedOnceAndKept` | two calls on one `t.TempDir()` → the same id, 32 lowercase hex; the file holds it; mode `0600` (asserted on non-Windows only) | an id minted per start, so every restart's reaper treats its own sessions as another's |
| `TestTwoStartsRacingAgreeOnOneID` | 8 goroutines call `InstallID` on a fresh dir → one distinct id | a create without `O_EXCL`, where the last writer wins and the earlier start stamps a lost id |
| `TestAMalformedInstallIDIsAnErrorAndIsKept` | the file holds `"x"`, then 32 hex plus a second line → an error naming the file each time, and the file's bytes unchanged | a silent regeneration that makes this laptop's running cloud sessions foreign |

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
`TestADeletedMachineEndsItsSessions` (`ErrUnknownMachine` → exited with the reason);
`TestTheReaperNeverDestroysAnotherControlPlanesCloudSandbox` (review I6: a fake cloud backend
lists three sandboxes that no record names, one labelled with the reaper's `InstallID`, one with
another install id, one with no control-plane label. After a sweep, only the first was
destroyed; `Destroy` was never called with the other two ids; each of the two was logged once
across two sweeps. The fake ignores any filter, as a broker that dropped the query would. Kills:
rule 4's label check dropped; a missing label read as "ours". The third mutant, the check
applied to the local backend too, where Docker sandboxes carry no such label, is killed by
phase 2's own `TestSweepDestroysOrphansAndOnlyOrphans` (`reaper_test.go:417`), which then goes
red. All three sandboxes are older than rule 4's mid-create grace
(`TestSweepSparesAnOrphanYoungEnoughToBeMidCreate`, `:455`), so the grace cannot be what spares
them.)

`cmd/api` config tests: `TestBrokerURLNeedsAScope`, `TestBrokerURLMustBeHTTPS`,
`TestNoBrokerIsPhase2` (which also asserts that no `install-id` file was created), and
`TestAMalformedInstallIDStopsTheStart` (exit 2 naming the file, review I6).

#### Steps

- [ ] **Step 1:** `machines` package and its tests.
- [ ] **Step 2:** `Deps.Machines`, `backendFor`, the handler swaps from the table — one commit-sized
  diff; run the whole `api` suite after each file.
- [ ] **Step 3:** The reaper; `main.go` wiring; config.
- [ ] **Step 3a (review I6):** mutation check on a `$TEMP` copy, each verified applied and the
  named test red: drop rule 4's label check → `TestTheReaperNeverDestroysAnotherControlPlanesCloudSandbox`;
  read a missing label as ours → the same test; apply the check to the local backend too →
  `TestSweepDestroysOrphansAndOnlyOrphans`; `InstallID` without `O_EXCL` →
  `TestTwoStartsRacingAgreeOnOneID`; regenerate a malformed file →
  `TestAMalformedInstallIDIsAnErrorAndIsKept`. The two named beforehand as likeliest to
  survive: (a) a missing label read as ours, because a fake that lists only labelled sandboxes
  never exercises it, and the unlabelled row exists to kill it; (b) the racing create, because a
  sequential test passes without `O_EXCL`, and the 8-goroutine test exists for it. It can still
  pass by luck on a mutant, so run it with `-count=20`.
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
- **More than one laptop per user, for sessions (review I6).** A cloud session belongs to the
  control plane that created it. A second laptop signed in as the same user, or this laptop
  after a reinstall with a fresh state directory, does not list, attach to, reap or end it. The
  sandbox keeps its pod (running while its agent runs), holds one of the owner's
  `MAX_SANDBOXES_PER_USER` slots, and keeps its
  machine awake (the sleeper refuses while a session pod exists, task 19), until the owner
  deletes it. The broker's unfiltered `GET /v1/sandboxes` and its `DELETE` are the seam for
  that; the surface is the owner question in the review's b06 section. Lanes have the same limit
  (task 25, P3A-2).
