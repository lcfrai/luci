### Task 24: the sandboxd git runner: lane git inside the sandbox, lane work as a bundle (ruling R4)

Ruling R4 (`.superpowers/sdd/p3/orchestrator-rulings.md`) is structural: host git never executes
with an agent-writable directory as its repository, `--git-dir`, `-C`, working tree or alternate
(R4.2). A lane clone is such a directory from the moment a sandbox mounts it. So git work on a
lane runs **inside the sandbox**, through `sandboxd`, our own daemon there (R4.3). `sandboxd` is
not the agent CLI, so the "never parse the CLI or its PTY" rule does not apply to it.

This task builds that runner: a git RPC on `sandboxd`'s existing authenticated HTTP listener. It
answers with structured data. Lane work leaves the sandbox as a `git bundle`, which is data too
(R4.4). Updates go the other way through the same runner: the host writes a bundle from the
project and the runner fetches it inside the sandbox (R4.5, task 09's refresh). This half of the
part is the sandbox side. The host side (the client, host-side `bundle verify` and the fetch into
`refs/panacea/lanes/…`) is the second half, edit unit U2.

R4.3 names this as the seam phase 8's `machined` reuses (P8-D7: "the machine agent serves the
catalog and the git runner inside the machine pod"). Phase 3 builds the Docker half. P8-D7 is
R4's citation; no document in this tree carries it (grep, 2026-09-24).

**Order.** Deps: 02 (the pinned git, `gitx.MaxSentence` and `Sentence`, which the runner's error
copies), 04 (the lane clone the runner serves, mounted at `sessionWorkDir`). Users: 07 (`status`,
`refs` for the lane head), 09 (`bundle` out for landing, `fetch` in for refresh) and 10 through
09, 11 (`refs` for Elsewhere), 16 (`diff`). Plan order: after 06, before 07.

**Trust note.** Every runner answer is the agent's claim about its own lane. The agent runs as the
same uid as `sandboxd` (`dev`, uid 1001, `images/sandbox/Dockerfile:19` and `:24`; the entrypoint
`exec`s `sandboxd`, `images/sandbox/entrypoint.sh:13`), so it can read `sandboxd`'s token from
the process environment (`PANACEA_APP_ATTACH_TOKEN`, `sandboxd/cmd/sandboxd/main.go:44`). That is
true since phase 2 and this task does not change it. It follows that:
- the runner's validation and limits protect the runner and the lane from a malformed host
  request, and bound an honest answer; they protect the host from nothing;
- the host parses every answer as untrusted data, under the same limits, and derives no
  permission from it: a `status` or `refs` answer is a reading (task 07's "these are readings");
- the one thing the host trusts is a bundle it verified itself, with `git bundle verify` in the
  project under §HGH, before it fetches from it (U2).

**Files:**
- `sandboxd/internal/gitrunner/runner.go` — `Runner`, `New`, `ServeHTTP`: routing, POST-only,
  the op mutex, the per-op timeout, the error envelope (new)
- `sandboxd/internal/gitrunner/ops.go` — the five ops and the `git` argv each one runs (new)
- `sandboxd/internal/gitrunner/wire.go` — the request and response types, the JSON shapes (new)
- `sandboxd/internal/gitrunner/limits.go` — `MaxJSONBytes`, `MaxBundleBytes`, the timeouts (new)
- `sandboxd/internal/gitrunner/*_test.go` — the tests below (new)
- `sandboxd/internal/server/server.go` (p2) — `Config.GitWorktree`; one
  `s.mux.HandleFunc("/git/v1/", s.git)` beside `/attach` at `server.go:85`; `s.git` checks
  `s.authorised(r)` (`server.go:105-116`) first and answers 401 without it, exactly as
  `s.attach` does at `server.go:166-171`
- `sandboxd/cmd/sandboxd/main.go` (p2) — flag `-git-worktree`, default `/home/dev/work`, beside
  the flags at `main.go:40-48`; `serve_posix.go:11-18` and `serve_other.go:13` (p2) pass it
  through `newServer` into `server.Config`
- `testdata/gitrunner/*.json` — the wire golden fixtures, at the repo root (new; `go.work`
  already lists `./sandboxd` and `./control-plane`)

Stdlib only (`net/http`, `os/exec`, `encoding/json`, `context`, `sync`). No new dependency, so
`sandboxd/vendor` (committed; `images/sandbox/Dockerfile:8` builds with `-mod=vendor`) needs no
re-vendoring. The entrypoint passes no `-git-worktree`, and needs none: the default is the
control plane's `sessionWorkDir` (`control-plane/internal/api/sessions.go:33` in this tree;
`:35` at `feat/phase-2-cockpit`, which is not an ancestor of this branch; the value is the same).
