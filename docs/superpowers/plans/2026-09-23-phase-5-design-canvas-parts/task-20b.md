# Task 20b — configuration and the `cmd/api` wiring of the canvas stack

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 17, 19, 20a · **External:** **Hub C3 — Hub plan tasks 37–38 (`localkey`), hard** (`newDesignAPI` builds `KeyGuard` from the key `main.go` mints; plan rule 4) · **Hours:** 1.5

## Why

Task 20a (`task-20.md`) builds the design API against tasks 11–14 and 19 (14 for `ports.go`, re-review RR-I6-1). This task joins it to
everything else in one process: the store, the authority, the MCP server with every tool and its
instructions, the relay supervisor and the design routes. It is its own task because the wiring
calls code that 20a's dependencies do not provide (review I6):

| Call | Made by task | Reached through |
|---|---|---|
| `mcp.NewServer(info, instructions, allowedHosts, log)` (`task-14.md:43`) | 14 | 17 (17 → 15 → 14) |
| `mcp.RegisterAll(s, d)` (`task-15.md:93`), all 37 tools | 15, 16, 17 | 17 |
| `mcp.Instructions` (`task-17.md:61`) | 17 | 17 |
| `mcprelay.NewSupervisor(mcprelay.SupervisorConfig{…})` (`task-19.md:86-106`) | 19 | 19 |
| `designapi.New`, `(*API).Approvals()`, `designapi.BriefsPort` | 20a | 20a |

It does **not** depend on task 21; task 21 depends on it. The `Approvals` port is 20a's
`(*API).Approvals()` method, which answers "no approval" until task 21 replaces its body inside
`designapi`; this task wires the method's result, so 21 changes no MCP wiring (`task-20.md`, *The
`Approvals` seam*). `designapi.Config.Hub` does not exist yet: task 21 adds it and the one
`design.go` line that passes Hub task 32's `hubclient` value (nil when `PANACEA_APP_HUB_URL` is
unset). Likewise `designapi.Config.HandoffGit`: task 22 adds it and its `design.go` line (nil
until phase 3 is in the tree; fix batch b03, review I1). This task passes every other `Config`
field: `Authority`, `Briefs`, `Sessions` (`registry.Store`, `registry.go:48`), `Identity`, `Log`,
`KeyGuard`.

## Files

- `control-plane/cmd/api/config.go`, `main.go` — two variables; the wiring
- `control-plane/cmd/api/design.go`, `design_test.go` (new) — `newDesignAPI`, the one builder
  `main.go` uses, so the `KeyGuard` it passes is testable without `api.New`'s outer wrap (re-review I-2);
  `newMCPServer` and `newMCPDeps`, so the MCP wiring is testable without starting the process;
  `newAuthorityConfig`, so the branch-lifecycle port it passes is testable (review I7)
- `control-plane/cmd/api/main_test.go` — `TestMain_wiring`

## Contracts

```go
// control-plane/cmd/api/config.go — two variables; neither is ever silently ignored (config.go:69-70)
	envCanvasMCP = "PANACEA_APP_CANVAS_MCP" // "on" (default) | "off"
	envCanvasDir = "PANACEA_APP_CANVAS_DIR" // absolute; default statepath.Dir("canvas")
```

**Wiring, in `main.go`**, in this order: `canvasstore.NewFile(canvasDir, log)` (the import is
aliased `canvasstore "github.com/kpmg-au-emu/panacea-app/canvas/store"`, because `main.go`'s local
`store` is already the `*registry.FileStore`, `control-plane/cmd/api/main.go:78`, which
`Sessions: store` below relies on; the signature is `NewFile(dir string, log *slog.Logger)`,
`task-10.md:92`; re-review RR-M2) →
`authority.New(authority.Config{…, Branches: branches})`, where `branches` is
`designapi.Phase3Branches(…)` over phase 3's lane store and git runner when task 20a wrote
`branches_phase3.go`, else nil (seam S5-P3.2, review I7) →
`newDesignAPI(cfg, key, deps)` (the design API first, because the MCP deps read its `Approvals()`) →
`newMCPServer(auth, designAPI, briefs, log)`, which is
`mcp.NewServer(info, mcp.Instructions, []string{"127.0.0.1:8731", "localhost:8731"}, log)` then
`mcp.RegisterAll(server, newMCPDeps(auth, designAPI, briefs))`, and `newMCPDeps` is
`mcp.Deps{Authority: auth, Briefs: designapi.BriefsPort(briefs), Approvals: designAPI.Approvals()}`
(both in `design.go`) → `mcprelay.NewSupervisor(mcprelay.SupervisorConfig{Sessions: store, Handler: server.Handler(), Release: auth.ReleaseSession, Log: log})`,
its `Run` under the process context → `api.Deps.Design = designAPI`, and
`api.Deps.CanvasMCP = cfg.CanvasMCP`. Shutdown closes the supervisor, then the authority.

`newDesignAPI(cfg Config, key localkey.Key, deps designDeps) (*designapi.API, error)` (in
`control-plane/cmd/api/design.go`) is the ONLY place `main.go` builds the design API (re-review
I-2). It calls `designapi.New` with `KeyGuard` built from `key` — the key `main.go` already mints
for `api.Deps.LocalKey` (Hub task 37; one key, one `Mint` call) — and returns its result unchanged.
It exists so a test can check the guard `main.go` really passes without `api.New`'s outer wrap
(Hub task 37, `.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-37.md:70`,
wraps the whole guarded mux in `localkey.Middleware`). Through the
real process that outer wrap answers 401 even when `KeyGuard` is the identity function, so no test
over the process can tell the two apart.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestNewAuthority_branchesWired` (`design_test.go`; only when `branches_phase3.go` exists) | the `authority.Config` `main.go` builds (through a `newAuthorityConfig` helper in `design.go`) has a non-nil `Branches`; without phase 3 in the tree the test is skipped with that reason | a phase-3 tree whose renames and deletes the canvas never follows because the adapter was never passed (review I7) |
| `TestConfig_canvasVars` | `PANACEA_APP_CANVAS_MCP=maybe` exits with a message; `off` sets `CanvasMCP` false; a relative `PANACEA_APP_CANVAS_DIR` exits | a silently ignored variable |
| `TestMain_wiring` (the `main_test.go` pattern) | the process starts with a temp state dir and serves route 1 **with** the key read from `PANACEA_APP_LOCAL_KEY_FILE`; route 1 and route 5 **without** it, and with `Bearer ` (empty), → 401 `local_key_required` | a stack that compiles but is never mounted (it cannot kill an identity `KeyGuard`: `api.New`'s outer `localkey.Middleware` answers 401 first — re-review I-2) |
| `TestNewDesignAPI_guardsBareMux` (`control-plane/cmd/api/design_test.go`) | `newDesignAPI` with a key minted into `t.TempDir()` and fake deps; its result mounted with `Mount(mux.Handle)` on a bare `http.ServeMux` — **no `api.New`, no outer wrap**; route 5 with no `Authorization`, no `Origin` and `Host: 127.0.0.1:8730` → 401 `local_key_required` and the authority unchanged; `Bearer ` (empty) → the same; the right key → not 401 | `main.go` (through `newDesignAPI`) passing an identity `KeyGuard`, or a guard over some other key (re-review I-2) |
| `TestNewMCPServer_wired` (`design_test.go`) | `newMCPServer(auth, designAPI, briefs, log)` — the one place `main.go` calls `mcp.NewServer` and `mcp.RegisterAll` — driven over `server.Handler()` with `httptest`: `initialize` returns `mcp.Instructions` exactly; `tools/list` returns the same names as a reference server built in the test by calling `mcp.RegisterAll` directly (so no count is hard-coded here) | a `main.go` that builds the server but registers nothing, or passes an empty instructions string (review I6: the calls this task exists for) |
| `TestNewMCPDeps_approvalsFromDesignAPI` (`design_test.go`) | `newMCPDeps(auth, designAPI, briefs)` returns `mcp.Deps` whose `Approvals == designAPI.Approvals()` (interface equality: a stub of another type is unequal) and whose `Authority` is `auth` | a `main.go` passing a local stub of its own, which would keep answering "none" after task 21 lands |

## Steps

1. Tests. 2. `config.go`, `design.go`, then `main.go` in the order above.
3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./control-plane/... -race -count=1 && go vet ./control-plane/...
```
Expected: every package `ok`; vet silent.

**The live check starts a control plane, and a control plane's reaper destroys at startup every
sandbox it does not own** (re-review, carried over). So, in this order:
1. Take the stream's Docker-daemon lock and check for foreign `panacea-app.managed` containers, as
   the local lock procedure says (`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/docker-daemon-lock.md`
   in the main tree; git-ignored, so not in this repository). Stop if the lock is held or a foreign
   container exists.
2. Build a binary (never `go run`: a background `go run` orphans its child when stopped):
   `go build -o "$P5/api-20b.exe" ./control-plane/cmd/api`, where `$P5` is the stream's scratch
   directory.
3. Start it in the background on a **stream port**, never the default 8730, with its own state and
   the reaper held off, and record its PID:
   `PANACEA_APP_ADDR=127.0.0.1:8781 PANACEA_APP_STATE_DIR="$P5/state-20b"
   PANACEA_APP_REAP_INTERVAL=24h PANACEA_APP_LOCAL_KEY_FILE="$P5/state-20b/local.key" "$P5/api-20b.exe"`.
4. `curl -s -o "$P5/r1.json" -w '%{http_code}' http://127.0.0.1:8781/v1/design/canvases/cv-0000000000000000 > "$P5/r1.code"`
   → `401` and `local_key_required`: the curl sends no key. Then the same with
   `-H "Authorization: Bearer $(cat "$P5/state-20b/local.key")"` → `404` and `canvas_not_found`.
5. Kill the binary by the recorded PID, purge only this run's sessions, release the lock.

Hub task C3 is a hard dependency, so there is no "before the key" answer to expect.

## Not claimed

- The routes, the stream, the codes and the audit are task 20a's; approval, handoff and session
  links are tasks 21 and 22.
- That the tool count is 37 (14 read, 15 write, 8 more; review M19 added `rename_page` and
  `delete_page`) is tasks 15–17's claim; this task only checks that `main.go` registers
  whatever `RegisterAll` registers.
