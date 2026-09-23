### Task 21: `machined` git — phase 3's runner, on the machine

Phase 3 keys every git call by machine (its brief A6): `gitx.Machine` has one implementation,
`gitx.Local`, and "Phase 8 adds a Kata machine behind the same four methods: a pod gets a clone,
FetchURL names it, and the control plane fetches from it" (phase 3 plan, sibling `phase-3`
worktree, `2026-09-23-phase-3-git-convergence-parts/task-02.md:146-159`, seam S3-P8.1). On a
cloud machine the project repository and its lane clones share one disk (OD-2, P8-D6), so the
fetch-back is local **to the machine** — git just has to run there. This task gives `machined` one
route that runs **phase 3's own `gitx.Exec`** on the disk; task 25 wraps it as the laptop's
`gitx.Runner`.

**Depends on phase 3 task 02** (`internal/gitx`: `Call`, `Result`, `Runner`, `Exec`, `Error`,
`Class` — task-02.md `:40-140`). If it has not landed, this task does not start: no local copy of
`gitx` is written.

**Files:**
- `control-plane/internal/machineapi/wire.go` — `GitCall`, `GitResult` (modify)
- `control-plane/cmd/machined/git.go`, `git_test.go` (new)
- `control-plane/cmd/machined/server.go` — the route (modify)

#### Contract (complete)

```go
package machineapi

// GitCall is POST /v1/git's body: gitx.Call on the wire, with Dir relative to
// the machine root. Class is "read" | "local" | "remote" (gitx.Read/Local/Remote).
// Stdin is base64 in JSON ([]byte). Env keys must match ^GIT_[A-Z0-9_]+$; values
// may be secrets (phase 3 task 13 passes http.extraHeader through
// GIT_CONFIG_COUNT/KEY_n/VALUE_n) and are never logged, echoed or stored.
type GitCall struct {
	Dir    string            `json:"dir"`
	Args   []string          `json:"args"`
	Class  string            `json:"class"`
	Stdin  []byte            `json:"stdin,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
	OKExit []int             `json:"okExit,omitempty"`
}

// GitResult is a call that exited 0 or with a code in OKExit (gitx.Result).
type GitResult struct {
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// A call that did not answer is a problem body with code "git_failed" (phase 3's
// httpx.CodeGitFailed, its task 04) and these details, from which the laptop
// rebuilds a *gitx.Error (task 25):
//   details.gitSays   gitx.Error.Sentence()
//   details.exitCode  decimal, "-1" if git did not start or timed out
//   details.timedOut  "true" | "false"
// Args and Dir are NOT in the details: the caller already has them, and Args may
// name a URL with credentials in it.
const (
	DetailGitSays  = "gitSays"
	DetailExitCode = "exitCode"
	DetailTimedOut = "timedOut"
)
```

| Route | Answer | Failures |
|---|---|---|
| `POST /v1/git` | `200 GitResult` | `400 bad_request` (a `Dir` that escapes the root, an unknown `Class`, an `Env` key outside `GIT_*`, empty `Args`) · `422 git_failed` with the three details · `413` above 16 MiB of stdin |

**What `machined` adds to every call**, before phase 3's own headless defaults:
`-c core.hooksPath=/dev/null -c core.fsmonitor=false` at the front of `Args`. A lane clone's
`.git/hooks` and `core.fsmonitor` are written by an agent; on the machine pod they would run as
code the agent chose, outside any session's lifetime. (Whether phase 3 does the same for
`gitx.Local` on the laptop is phase 3's to decide; this plan records it as a question to it in
task 33.) The binary is fixed (`git` on the image's `PATH`); nothing but git ever runs.

Responses are capped at 64 MiB of stdout; a call that would exceed it fails `git_failed` with
`gitSays: "output exceeds 64 MiB"`, so a runaway `git log` cannot exhaust the pod.

**Adding a project to a cloud machine** is this route with `git clone <url> projects/<name>`,
`Class: "remote"`, and the user's host token in `Env` (from phase 3's credential store, which the
Hub's C1 builds — ADR-H1 `:344-352`). No clone-specific route exists; the cockpit surface that
starts it belongs to whichever phase owns "add project" (recorded as seam S8-P3.2, task 33).

#### Tests to write first (`git_test.go`; real `git`, skipped with a message if absent)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGitRunsInTheMachinePath` | `rev-parse --show-toplevel` with `Dir: "projects/a"` prints `<root>/projects/a` | a Dir resolved against the process's cwd |
| `TestADirOutsideTheRootIsRefused` | `Dir: "../x"`, `"/etc"` → 400, git never ran | git run anywhere on the pod |
| `TestEnvIsLimitedToGitKeys` | `LD_PRELOAD` → 400; `GIT_CONFIG_COUNT` accepted | arbitrary code loaded into git |
| `TestAnExtraHeaderReachesGit` | `GIT_CONFIG_COUNT=1`, `KEY_0=http.extraHeader`, `VALUE_0=Authorization: Bearer t0k` → `config --get http.extraHeader` prints it | phase 3's push credential path broken on cloud machines |
| `TestSecretsNeverLeaveInLogsOrErrors` | the same value, on a failing call → neither the log nor the problem body contains `t0k` | a host token in a log |
| `TestHooksNeverRun` | a clone whose `post-checkout` writes a marker file; `checkout -b x` through the route → no marker | agent-written hooks executed on the machine pod |
| `TestAFailureIsGitFailedWithGitsOwnSentence` | `rev-parse nosuchref` → 422, `details.gitSays` begins `fatal:`, `exitCode` non-zero, no `args` in details | a generic 500; argv (which may hold a URL) in the body |
| `TestOKExitIsAnAnswer` | a `merge-tree --write-tree` conflict with `OKExit: [1]` → 200, `exitCode: 1` | a forecast conflict reported as a failure |
| `TestStdinReachesGit` | `hash-object --stdin` with stdin `x` → the known hash | stdin dropped (commit-tree -F - in phase 3) |
| `TestReadCallsTakeNoOptionalLocks` | a `Class: "read"` call's environment includes `GIT_OPTIONAL_LOCKS=0` (via a fake `git` on `PATH` that dumps its env) | a status racing an agent for `index.lock` |
| `TestOutputIsCapped` | a fake `git` writing 65 MiB → 422 `output exceeds 64 MiB` | a pod killed by a runaway log |

#### Steps

- [ ] **Step 1:** Confirm phase 3 task 02 has landed: `git grep -n "type Runner interface" -- control-plane/internal/gitx/` returns one line. If not, stop.
- [ ] **Step 2:** Wire types; tests; the route over `gitx.NewExec()`; all pass.
- [ ] **Step 3:** Full run; commit.

#### Acceptance

```bash
go test ./control-plane/cmd/machined/ -run Git -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 11 (or SKIP lines without git)
go test ./sandboxd/... ./control-plane/... -race -count=1                                       # ok
```

#### What this task does not claim

- That phase 3's `gitx` behaves identically on the machine pod's Linux git and the laptop's
  Windows git (`MinVersion` 2.40 is checked by `gitx.CheckVersion` at `machined` start; the sandbox
  image's Debian bookworm git version is **UNVERIFIED** against it — task 27 checks it at build).
- A clone surface in the cockpit.
