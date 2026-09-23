### Task 10: `api` — the `Materialiser` seam in create and restart (ruling R1)

The guarantee the whole phase rests on: **`CLAUDE.md` is on disk before the harness process
starts, and `Session.ContextRef` names exactly what was written.** Today nothing enforces it — the
survey's blocker (§4.1): `api.Deps` has no `Materialiser`, and `handleSessionCreate` goes from the
worktree to `Backend.Create` with no step between. Ruling R1 fixes the shape: an interface declared
**in `api`**, held in `Deps`, called between worktree-resolve and `backend.Create`, returning the
key; the handlers stamp `ContextRef` from the return value and **never from the request** (P6).

**This task edits `sessions.go`, which three phases touch** (phase 3 ask P4-7). Start it **after
phase 2 task 26 has merged** (brief R5), take the file as it stands on your base, and name the base
commit in the commit message. Phase 2's resume fix (`--session-id`, `sessions.go:337` at
`3303fd6`) is already in the file; phase 3 task 05 edits `sessionEnv`, not the step order. **The
order within the handler is: materialise, then mint and record; on restart, materialise also comes
before the old sandbox's destroy** — so a refusal leaves no record and no sandbox on create, leaves
the running sandbox untouched on restart, and the stamped key and the minted conversation id both describe the process that
actually starts (R1's seam note).

**Files:**
- `control-plane/internal/api/materialiser.go` — the interface and its types (new, in `api`)
- `control-plane/internal/api/api.go` — `Deps.Materialiser` (required), `Deps.Discover`
  (optional), the validation line, the mount line (modify, declared departure)
- `control-plane/internal/api/sessions.go` — both handlers, and `sessionLocate` gains an error
  return (modify, declared departure)
- `control-plane/internal/httpx/problem.go` — `CodeBriefNotWritten` (modify)
- `control-plane/internal/api/materialise_test.go` — the new tests (new)
- `control-plane/internal/api/api_test.go` — its `Deps` builder gains a recording fake (modify)
- `control-plane/internal/api/attach_test.go` — its `New(Deps{…})` (`:303` at `2d6d95b`) gains its
  own recording fake (modify). It is `package api`, so it cannot see `api_test.go`'s builder
  (`package api_test`) (review I1)
- `control-plane/internal/api/sessions_test.go` — its `Deps{…}` (`:584` at `2d6d95b`) gains its own
  recording fake, `sessionsLocator` gains a per-project `worktreesErr`, and the `sessionLocate`
  tests below (modify; `package api`) (review I1, I2)
- `control-plane/cmd/api/materialiser_interim.go` — `materialiserUntilTask14`, the named interim
  value (new; **task 14 deletes it**) (review I1)
- `control-plane/cmd/api/materialiser_interim_test.go` — its one test (new; task 14 deletes it)
- `control-plane/cmd/api/main.go` — `Materialiser: newMaterialiserUntilTask14(log)` in the
  `api.New(api.Deps{…})` literal (`:132` at `2d6d95b`) (modify) (review I1)

**Interfaces — complete source:**

```go
// control-plane/internal/api/materialiser.go
package api

// Materialiser writes the brief a session inherits into its worktree BEFORE
// the harness process starts, and says what it wrote (ruling R1; provenance
// contract §2.4). Phase 4's contextwrite satisfies it; api imports nothing of
// phase 4.
//
// THE ORDERING RULE: handleSessionCreate and handleSessionRestart call it
// after the worktree resolves and before anything is recorded or created,
// and, on restart, before the old sandbox's destroy (a refusal must leave
// the running agent running). Called after the process starts, the session
// inherits nothing, silently.
type Materialiser interface {
	Materialise(ctx context.Context, req MaterialiseRequest) (MaterialiseResult, error)
}

type MaterialiseRequest struct {
	WorktreeID   string
	WorktreePath string // absolute, from Deps.Worktrees.Path
	// ProjectID is local: sessionLocate's on create, the record's on restart
	// (re-located when ""; re-review R2-M2 item 4). "" is a worktree outside
	// every project, enumerated cleanly: nothing to inherit (task 09 step 4).
	// An enumeration FAILURE never arrives as "": the handler refuses it as
	// project_unknown first (review I2), unless WithoutBrief is set.
	ProjectID    string
	BranchID     string // local; "" for a detached worktree
	// WithoutBrief is the person's "Start without the brief" (4-D1). The
	// Materialiser removes any managed block it can and returns
	// ContextRef "none:declined"; it does not ask the Hub.
	WithoutBrief bool
}

type MaterialiseResult struct {
	// ContextRef is stamped on the session record VERBATIM: a brief key,
	// "" when no brief exists to inherit, or "none:declined". The handler
	// never interprets it (types.ts's promise, kept on the server too).
	ContextRef string
}

// MaterialiseError is a refusal the person can act on: the handler answers
// 409 brief_not_written with Reason in details and Message as the sentence.
// Any other error from Materialise is a 500.
type MaterialiseError struct {
	Reason  string
	Message string
}

func (e *MaterialiseError) Error() string { return "api: brief not written: " + e.Reason }

// The reasons, closed. The desktop switches on them (task 20).
const (
	BriefReasonHubUnavailable = "hub_unavailable"   // the Hub did not answer; a brief may exist
	BriefReasonSignedOut      = "signed_out"        // no Hub token; a brief may exist
	BriefReasonWriteFailed    = "write_failed"      // the file could not be written
	BriefReasonBlockBroken    = "block_broken"      // a marker with no partner (a begin with no end, a stray or early end) or two blocks
	BriefReasonTracked        = "tracked_elsewhere" // CLAUDE.md and .claude/CLAUDE.md both tracked (P4-D)
	// BriefReasonProjectUnknown is raised by the HANDLER, never by a
	// Materialiser: sessionLocate could not enumerate the projects, or could
	// not read a project's worktrees and did not find this one elsewhere, so
	// which project's brief applies is unknown (review I2). A brief may exist.
	BriefReasonProjectUnknown = "project_unknown"
)

// RouteMounter lets a package outside api register routes through the
// recorder, so contract §10a's audit sees every pattern (the recorder is
// unexported, and a *http.ServeMux registration would escape it).
type RouteMounter interface {
	Mount(handle func(pattern string, h http.Handler))
}
```

```go
// control-plane/internal/api/api.go — Deps gains exactly these two fields
	// Materialiser writes a session's inherited brief before its process
	// starts (R1). Required: a nil one is a session that silently inherits
	// nothing, which is the failure this field exists to make impossible.
	Materialiser Materialiser

	// Discover mounts phase 4's /v1/discover routes on the guarded mux, so
	// the /api/v1 alias and the §10a audit both reach them. Optional: nil
	// mounts nothing.
	Discover RouteMounter
```

In `New`: `{"Materialiser", d.Materialiser == nil}` joins the validation table, and after
`a.registerAttach(guarded)`: `if d.Discover != nil { d.Discover.Mount(guarded.Handle) }`.

```go
// control-plane/internal/httpx/problem.go — one constant, beside CodeConflict
	CodeBriefNotWritten = "brief_not_written" // 409 the brief could not be written before the session started (4-D1)
```

```go
// control-plane/internal/api/sessions.go — sessionLocate gains an error (review I2).
// At 2d6d95b it is :540-567 and returns ("", "") for a failed Projects()
// (:549-553) and for a failed Worktrees() (:555-558, continue) alike.

// sessionLocate finds the project and branch a worktree belongs to. Three
// answers, never confused:
//   - found: its ids and nil, even if another project's Worktrees() failed
//     (a worktree id belongs to one project);
//   - outside every project, and every enumeration succeeded: "", "", nil;
//   - not found, and Projects() or some Worktrees() failed: "", "",
//     errProjectUnknown wrapping the first failure.
// The two Warn logs stay: the error is for the handler, the log for the
// operator. A nil Projects (a hand-built API; New rejects it) is "", "", nil.
func (a *API) sessionLocate(ctx context.Context, worktreeID string) (projectID, branchID string, err error)

var errProjectUnknown = errors.New("api: the projects could not be enumerated")
```

```go
// control-plane/cmd/api/materialiser_interim.go — new in task 10, DELETED by task 14
package main

// materialiserUntilTask14 is the Materialiser the binary runs between phase 4
// task 10, which makes Deps.Materialiser required, and task 14, which wires
// contextwrite and deletes this file (review I1). Its name is its deadline,
// so it cannot pass for the real thing: it writes nothing. It is not a
// default inside api: api.New still refuses a nil Materialiser, and only
// main chooses this value.
type materialiserUntilTask14 struct{}

// newMaterialiserUntilTask14 logs ONE line, at start-up, that sessions
// inherit nothing until phase 4 task 14. Materialise itself never logs.
func newMaterialiserUntilTask14(log *slog.Logger) materialiserUntilTask14 {
	log.Warn("sessions inherit no brief until phase 4 task 14 wires contextwrite")
	return materialiserUntilTask14{}
}

// Materialise returns "" (nothing inherited), or "none:declined" when the
// person declined the brief, as MaterialiseRequest.WithoutBrief says. No code
// before task 14 writes a managed block (at 2d6d95b nothing in control-plane
// imports claudemd or contextwrite), so there is no block to strip.
func (materialiserUntilTask14) Materialise(ctx context.Context, req api.MaterialiseRequest) (api.MaterialiseResult, error) {
	if req.WithoutBrief {
		return api.MaterialiseResult{ContextRef: "none:declined"}, nil
	}
	return api.MaterialiseResult{}, nil
}

var _ api.Materialiser = materialiserUntilTask14{}
```

`main.go` adds one line to the `api.New(api.Deps{…})` literal (`:132` at `2d6d95b`):
`Materialiser: newMaterialiserUntilTask14(log),`. No test in `cmd/api` reaches `api.New` (at
`2d6d95b`, `TestRunRefusesANonLoopbackBindWithExitCode2` `main_test.go:387` and
`TestRunRefusesAMissingCABundleWithExitCode2` `config_sandbox_test.go:189` both exit in config),
so a missing line would show only when the binary starts ("api: Deps.Materialiser is nil"). The
acceptance below therefore greps for the line and builds the binary.

**Wire changes** (UI contract §3.9 and §3.14 amendments, written by task 22):

```jsonc
// POST /v1/sessions — request: contextRef is IGNORED (kept in the struct so an
// old client is not a 400); one new optional field
{ "worktreeId": "w-…", "withoutBrief": false }

// POST /v1/sessions/{id}/restart — request: an OPTIONAL body; empty is {}
{ "withoutBrief": false }

// Both, when Materialise refuses — nothing was recorded, nothing was started
// 409
{ "error": "brief_not_written",
  "message": "The brief could not be written because the Hub did not answer.",
  "details": { "reason": "hub_unavailable", "worktreeId": "w-…" } }
```

**The handler changes, in order:**

- `handleSessionCreate`: after `sessionLocate` (`sessions.go:376` at `3303fd6`) and **before**
  `Store.Create` (`:382`), call `Materialise` with the resolved path, `projectID`, `branchID` and
  `req.WithoutBrief`. `*MaterialiseError` → 409 as above and return; any other error →
  `a.internal`. On success, `ContextRef: result.ContextRef` replaces `ContextRef: req.ContextRef`
  (`:401`).
- `handleSessionCreate`, `sessionLocate`'s error (review I2; at `2d6d95b` the call is
  `sessions.go:452`): when it is `errProjectUnknown` and `req.WithoutBrief` is false, the handler
  answers through the same 409 writer with `&MaterialiseError{Reason: BriefReasonProjectUnknown,
  Message: "The brief could not be written because the projects could not be read."}`, and
  returns. `Materialise` is not called; nothing is recorded or created. When `req.WithoutBrief`
  is true, the error does not refuse: the person declined the brief, so no project's brief is
  needed (task 09 step 3 never asks the Hub), and refusing would make *Start without the brief* a
  dead end, as N1 is on `tracked_elsewhere`. `Materialise` then gets `ProjectID: ""`, and the record
  carries empty ids, which is what phase 2 records today for every enumeration failure. A clean
  `""` (outside every project) is passed through unchanged.
- `handleSessionRestart`: decode the optional body (an empty body is `{}`; an unknown field is
  `invalid_json`, as everywhere). Call `Materialise` with all four request fields:
  `WorktreeID: sess.WorktreeID`, `WorktreePath: path` (the restart's own `Worktrees.Path` result,
  `sessions.go:972` at `2d6d95b`; an empty path is task 09's 500 on every restart),
  `ProjectID: sess.ProjectID`, `BranchID: sess.BranchID`, and `WithoutBrief: body.WithoutBrief`
  (the decoded body; dropping it makes task 20's *Start without the brief* ask the Hub again and
  get a second 409). Make the call **after the argv rebuild and the `sessionEnv` rebuild, and before
  `a.deps.Backend.Destroy(ctx, sess.SandboxID)`** — never merely "before the record write", because
  the destroy sits between the two. Locate by symbol, not line: at `2d6d95b` the worktree resolves
  at `sessions.go:972`, the argv rebuild is `:998-1018`, `sessionEnv` is `:1022`, the destroy is
  `:1025-1032` and the record write is `:1067`. The handler's own rule for argv (`:996-997`, "a
  restart that cannot build its argv must fail while the old sandbox is still alive") applies to
  `Materialise` for the same reason: a Hub outage mid-restart must leave the running agent running,
  not kill it and then answer 409 with a record that names a destroyed sandbox. Set
  `sess.ContextRef` in the record write. Restart is a new process, so it **restamps** (contract
  §2.4). **Accepted side effect:** the new block is written while the old process is still running.
  That process read `CLAUDE.md` when it started, so its stamp stays true until the destroy ends it.
- `handleSessionRestart`, a record with `ProjectID == ""` (review I2): that `""` is ambiguous. It
  may be a worktree outside every project, or an enumeration failure at create (phase 2 records
  those as `""`, and so does the `WithoutBrief` path above). So the restart calls
  `sessionLocate(ctx, sess.WorktreeID)` again, before `Materialise` and so before the destroy. The
  error rule is the create rule: `errProjectUnknown` without `body.WithoutBrief` is the 409
  `project_unknown`, and the old sandbox stays alive. Found ids go to `Materialise` and into the
  record write. A clean `""` stays `""`. A record that already names a project is not re-located.
- Resume and fork go through create, so they restamp too. Whether Claude Code re-reads `CLAUDE.md`
  on `--resume` is unverified (brief R7); restamping is correct either way for a new process, and
  task 23's canary tells us whether a resumed conversation *acts* on the new text.

**Tests first** (`materialise_test.go`, package `api_test`, with a recording fake that logs each
call's position relative to the fake backend's `Create` **and `Destroy`** and the fake store's
`Create`/`Update`; the log entries are `materialise`, `backend.destroy`, `backend.create`,
`store.create`, `store.update`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNewRefusesANilMaterialiser` | `api.New` errors naming `Materialiser` | a default no-op materialiser — silent non-inheritance |
| `TestCreateMaterialisesBeforeRecordingOrCreating` | the call log reads `materialise, store.create, backend.create` | R1 violated: the brief written after the PTY exists |
| `TestCreateStampsTheReturnedKeyNotTheRequested` | request `contextRef: "brief:bf-…@1"`, materialiser returns `@6` → the record and the response carry `@6` | P6: a client's guess recorded as inheritance |
| `TestCreateRefusalWritesNothing` | `*MaterialiseError{hub_unavailable}` → 409 `brief_not_written`, `details.reason`, no store record, no backend call | an orphan record for a session that never started |
| `TestCreateWithoutBriefPassesTheFlag` | `withoutBrief: true` reaches the fake; its `none:declined` is stamped | the override lost between the wire and the materialiser |
| `TestCreateOtherMaterialiseErrorIs500` | a plain error → 500 `internal`, the message generic | a raw path leaking into the problem body |
| `TestRestartRestamps` | a session at `@5`; materialiser returns `@6` on restart → the record is `@6` | the phase 2 behaviour of keeping `contextRef` across restart (contract §2.4) |
| `TestRestartWithoutBriefPassesTheFlag` | `POST …/restart` with `{"withoutBrief": true}` → the fake receives `WithoutBrief: true` **and** a non-empty `WorktreePath` equal to the fake `Worktrees.Path` result; its `none:declined` is stamped on the record and the response | the restart override lost between the wire and the materialiser (re-review I1), so *Start without the brief* asks the Hub again; a restart that passes no path |
| `TestRestartMaterialisesBeforeTheDestroy` | the seeded session holds a `SandboxID` the fake backend knows (an empty id skips the destroy, `sessions.go:1025`); the exact log is `materialise, backend.destroy, store.update, backend.create, store.update` (restart writes the store twice: before `Backend.Create` and after it) | `Materialise` placed beside the record write, after the destroy (review C1); a restart that re-reads a stale file |
| `TestRestartAcceptsAnEmptyBody` | `POST …/restart` with no body is 200 | breaking phase 2's desktop, which sends none |
| `TestRestartRefusalLeavesTheSandboxAlive` | `*MaterialiseError{hub_unavailable}` → 409 `brief_not_written`; the log holds no `backend.destroy`; the old `SandboxID` is still in the fake backend; the record's `SandboxID`, status and `contextRef` are unchanged | a Hub outage that kills the running agent, then leaves a record naming a destroyed sandbox |
| `TestDiscoverRoutesAreMountedOnTheGuardedMux` | a fake `RouteMounter` registering `GET /v1/discover/ping` is reachable at `/v1/…` **and** `/api/v1/…`, and a request with a foreign `Origin` is 403 | a mount on `root`, outside the guard and the alias |

**The two `package api` files** (review I1). `attach_test.go` and `sessions_test.go` build `Deps`
themselves, so each gets its **own** recording fake. `attach_test.go`'s fake asserts it was never
called: the relay starts no process. `sessions_test.go`'s fake records each request and returns a
set result. Its env (`sessionsNewEnvOverWith`, `:556` at `2d6d95b`) already drives create and
restart end to end through `sessionsLocator`, whose `err` (`:495`) fails all three methods. So the
`sessionLocate` tests live there (review I2). `sessionsLocator` gains `worktreesErr
map[string]error`, a failure for one project's `Worktrees` only.

| Test (`sessions_test.go`) | Asserts | Mutant it kills |
|---|---|---|
| `TestSessionLocateEnumerationFailureIsAnError` | `Projects()` fails → `"", ""` and `errors.Is(err, errProjectUnknown)`; separately, the only project's `Worktrees()` fails → the same | today's `"", ""` for a failure: the session inherits nothing, silently (R1's failure) |
| `TestSessionLocateFindsTheWorktreeDespiteAnotherProjectsFailure` | project A's `Worktrees()` fails, the worktree is in B → B's ids and `nil` | one broken repository refusing every session in every other project |
| `TestSessionLocateOutsideEveryProjectIsACleanEmpty` | every enumeration succeeds, no project holds the id → `"", "", nil` | a clean miss reported as `project_unknown` |
| `TestCreateRefusesAnUnknownProject` | `sessionsLocator.err` set → 409 `brief_not_written`, `details.reason: "project_unknown"`, `details.worktreeId`; the materialiser fake never called; no store record; no backend call | an enumeration failure turned into a session with no brief and no warning |
| `TestCreateWithoutBriefStartsDespiteAnUnknownProject` | `sessionsLocator.err` set, `withoutBrief: true` → 201; the fake gets `ProjectID: ""` and `WithoutBrief: true`; `none:declined` stamped | *Start without the brief* made a dead end, as N1 is on `tracked_elsewhere` |
| `TestCreateOutsideEveryProjectMaterialisesWithNoProject` | a clean miss → 201; the fake gets `ProjectID: ""`; its `""` is stamped | the clean `""` refused, or never passed through |
| `TestRestartRelocatesARecordWithNoProject` | a seeded record with `ProjectID: ""`. The locator finds the worktree → the fake gets the found ids, and the record write carries them. With `sessionsLocator.err` set → 409 `project_unknown`, no `backend.destroy`, the old sandbox still in the fake backend | a restart that reads an ambiguous `""` as "outside every project" |

| Test (`cmd/api/materialiser_interim_test.go`) | Asserts | Mutant it kills |
|---|---|---|
| `TestMaterialiserUntilTask14InheritsNothingAndSaysSoOnce` | constructed over a buffer logger → exactly one Warn line, naming "task 14"; two `Materialise` calls add no line; `WithoutBrief: false` → `""`, nil; `WithoutBrief: true` → `"none:declined"`, nil | a fabricated key; a log line per session start; the override's stamp lost |

**Steps:**

- [ ] **Step 1: Preflight.** `git log -1 --format=%H -- control-plane/internal/api/sessions.go`
  — record it. `grep -n "func (a \*API) handleSessionCreate\|func (a \*API) handleSessionRestart\|sessionLocate(ctx" control-plane/internal/api/sessions.go`
  — confirm the three symbols. Then `grep -n "Backend.Destroy(ctx, sess.SandboxID)\|harness.Build(harness.Request\|sessionEnv(" control-plane/internal/api/sessions.go`
  — inside `handleSessionRestart` the order must read argv rebuild, `sessionEnv`, destroy (at
  `2d6d95b`: `:1002`, `:1022`, `:1026`). If phase 2 or 3 has moved the step order, stop and report.
  Then list every direct `Deps` construction:
  `git grep -n "New(Deps{\|:= Deps{\|api.Deps{" -- control-plane`. At `2d6d95b` there are four:
  `api_test.go:142` (the builder), `attach_test.go:303`, `sessions_test.go:584`,
  `cmd/api/main.go:132` (review I1). Any other one joins this task's file list with its own fake.
- [ ] **Step 2:** Write `materialise_test.go`; add a recording fake to `api_test.go`'s `Deps`
  builder, to `attach_test.go` and to `sessions_test.go`, so every existing test still constructs
  a valid `Deps`; write the `sessionLocate` tests and `materialiser_interim_test.go`. Confirm the
  new tests fail.
- [ ] **Step 3:** Write `materialiser.go` and `materialiser_interim.go`; edit `api.go`,
  `problem.go`, `sessions.go` (both handlers and `sessionLocate`) and `main.go` as above.
- [ ] **Step 4:** Run the whole `api` suite, the route audit, the `cmd/api` suite, and build the
  binary.
- [ ] **Step 5:** Commit the eleven files by pathspec (`git add` the three new ones first). The
  message names R1, review I1 and I2, the base commit of `sessions.go`, the two declared
  departures, and that `materialiserUntilTask14` is interim, deleted by task 14.

**Acceptance:**

```
$ go test ./control-plane/internal/api/... ./control-plane/internal/httpx/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/api
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx
$ grep -n "req.ContextRef" control-plane/internal/api/sessions.go
(no output — the request's value reaches nothing)
$ grep -c "Materialise(ctx" control-plane/internal/api/sessions.go
2
$ go test ./control-plane/cmd/api/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api
$ grep -n "Materialiser:" control-plane/cmd/api/main.go
(one line: Materialiser: newMaterialiserUntilTask14(log),)
$ go build -o $TEMP/pa-api.exe ./control-plane/cmd/api && echo built
built
```

The binary starts after this task and logs the interim line once. No test reaches `main`'s `Deps`
literal, so the `grep` above is what keeps it. Starting the binary is not part of this acceptance:
its reaper sweeps at start-up, so it needs the shared Docker daemon lock
(`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/docker-daemon-lock.md` in the main tree;
git-ignored, local only). Task 23's live run is where it starts. Until task 14 every session is
stamped `""` or `none:declined`, and that is honest, because nothing is written.

**Does NOT claim:** anything about what is written (task 09); the desktop's handling of the 409
(task 20); the Go and UI contract amendments (task 22); a real Materialiser in the binary
(task 14, which replaces `materialiserUntilTask14`).
