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
order within the handler is: materialise, then mint and record** — so a refusal leaves no record and
no sandbox, and the stamped key and the minted conversation id both describe the process that
actually starts (R1's seam note).

**Files:**
- `control-plane/internal/api/materialiser.go` — the interface and its types (new, in `api`)
- `control-plane/internal/api/api.go` — `Deps.Materialiser` (required), `Deps.Discover`
  (optional), the validation line, the mount line (modify, declared departure)
- `control-plane/internal/api/sessions.go` — both handlers (modify, declared departure)
- `control-plane/internal/httpx/problem.go` — `CodeBriefNotWritten` (modify)
- `control-plane/internal/api/materialise_test.go` — the new tests (new)
- `control-plane/internal/api/api_test.go` — its `Deps` builder gains a recording fake (modify)

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
// after the worktree resolves and before anything is recorded or created.
// Called after the process starts, the session inherits nothing, silently.
type Materialiser interface {
	Materialise(ctx context.Context, req MaterialiseRequest) (MaterialiseResult, error)
}

type MaterialiseRequest struct {
	WorktreeID   string
	WorktreePath string // absolute, from Deps.Worktrees.Path
	ProjectID    string // local, from sessionLocate; "" when unknown
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
	BriefReasonBlockBroken    = "block_broken"      // an unterminated or duplicated managed block
	BriefReasonTracked        = "tracked_elsewhere" // CLAUDE.md and .claude/CLAUDE.md both tracked (P4-D)
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
- `handleSessionRestart`: decode the optional body (an empty body is `{}`; an unknown field is
  `invalid_json`, as everywhere); after the worktree resolves (`:895`) and **before** the restart's
  record write, call `Materialise` with the session's `WorktreeID`, `ProjectID`, `BranchID`; set
  `sess.ContextRef` in that record write. Restart is a new process, so it **restamps** (contract
  §2.4).
- Resume and fork go through create, so they restamp too. Whether Claude Code re-reads `CLAUDE.md`
  on `--resume` is unverified (brief R7); restamping is correct either way for a new process, and
  task 23's canary tells us whether a resumed conversation *acts* on the new text.

**Tests first** (`materialise_test.go`, package `api_test`, with a recording fake that logs each
call's position relative to the fake backend's `Create` and the fake store's `Create`/`Update`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNewRefusesANilMaterialiser` | `api.New` errors naming `Materialiser` | a default no-op materialiser — silent non-inheritance |
| `TestCreateMaterialisesBeforeRecordingOrCreating` | the call log reads `materialise, store.create, backend.create` | R1 violated: the brief written after the PTY exists |
| `TestCreateStampsTheReturnedKeyNotTheRequested` | request `contextRef: "brief:bf-…@1"`, materialiser returns `@6` → the record and the response carry `@6` | P6: a client's guess recorded as inheritance |
| `TestCreateRefusalWritesNothing` | `*MaterialiseError{hub_unavailable}` → 409 `brief_not_written`, `details.reason`, no store record, no backend call | an orphan record for a session that never started |
| `TestCreateWithoutBriefPassesTheFlag` | `withoutBrief: true` reaches the fake; its `none:declined` is stamped | the override lost between the wire and the materialiser |
| `TestCreateOtherMaterialiseErrorIs500` | a plain error → 500 `internal`, the message generic | a raw path leaking into the problem body |
| `TestRestartRestamps` | a session at `@5`; materialiser returns `@6` on restart → the record is `@6` | the phase 2 behaviour of keeping `contextRef` across restart (contract §2.4) |
| `TestRestartMaterialisesBeforeTheSandbox` | log order `materialise, store.update, backend.create` | a restart that re-reads a stale file |
| `TestRestartAcceptsAnEmptyBody` | `POST …/restart` with no body is 200 | breaking phase 2's desktop, which sends none |
| `TestRestartRefusalLeavesTheRecordAlone` | 409; the record's status and `contextRef` are unchanged | a half-restarted session |
| `TestDiscoverRoutesAreMountedOnTheGuardedMux` | a fake `RouteMounter` registering `GET /v1/discover/ping` is reachable at `/v1/…` **and** `/api/v1/…`, and a request with a foreign `Origin` is 403 | a mount on `root`, outside the guard and the alias |

**Steps:**

- [ ] **Step 1: Preflight.** `git log -1 --format=%H -- control-plane/internal/api/sessions.go`
  — record it. `grep -n "func (a \*API) handleSessionCreate\|func (a \*API) handleSessionRestart\|sessionLocate(ctx" control-plane/internal/api/sessions.go`
  — confirm the three symbols; if phase 2 or 3 has moved the step order, stop and report.
- [ ] **Step 2:** Write `materialise_test.go`; add a recording fake to `api_test.go`'s `Deps`
  builder so every existing test still constructs a valid `Deps`. Confirm the new tests fail.
- [ ] **Step 3:** Write `materialiser.go`; edit `api.go`, `problem.go`, `sessions.go` as above.
- [ ] **Step 4:** Run the whole `api` suite and the route audit.
- [ ] **Step 5:** Commit the six files by pathspec. The message names R1, the base commit of
  `sessions.go`, and the two declared departures.

**Acceptance:**

```
$ go test ./control-plane/internal/api/... ./control-plane/internal/httpx/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/api
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx
$ grep -n "req.ContextRef" control-plane/internal/api/sessions.go
(no output — the request's value reaches nothing)
$ grep -c "Materialise(ctx" control-plane/internal/api/sessions.go
2
```

**Does NOT claim:** anything about what is written (task 09); the desktop's handling of the 409
(task 20); the Go and UI contract amendments (task 22).
