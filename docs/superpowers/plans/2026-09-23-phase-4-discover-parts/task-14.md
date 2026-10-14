### Task 14: Wiring and configuration in `cmd/api`

Turns the packages on. After this task a control plane started with the Hub's variables serves
Discover, and every session start materialises. **Depends on** tasks 09–13; on the Hub's **C2**
(`hubclient.New`, the real client) and **C1** (`signin`, the `TokenSource`); on **phase 3 task
13** (`githost.ParseRemote`, `Remote.RepoKey`). Each missing dependency has a stated fallback
below, so this task never waits silently and never fabricates.

**Files:**
- `control-plane/cmd/api/discover.go` — `wireDiscover`, the three adapters (new)
- `control-plane/cmd/api/discover_test.go` (new)
- `control-plane/cmd/api/main.go` — one call to `wireDiscover`, two `Deps` fields; the
  `Materialiser` field's value `newMaterialiserUntilTask14(log)` (task 10) becomes
  `wiring.Materialiser` (modify)
- `control-plane/cmd/api/config.go` — the model variables (modify)
- `control-plane/cmd/api/evidence.go` + `evidence_test.go` — the adapter to phase 3's
  `evidence.Evaluator` (task 06, "The adapter"; S4-P3.4), **only if** step 1 finds phase 3's
  `evidence` package. Otherwise `discoverhub.Config.Evaluate` stays nil and phase 3's task 15
  writes this file (new, conditional)
- `control-plane/cmd/api/materialiser_interim.go` and `materialiser_interim_test.go` — task 10's
  interim `materialiserUntilTask14` (**delete**, `git rm`; review I1)

**Configuration** — every variable, read in `config.go` exactly as phase 2 reads its own:

| Variable | Meaning | Default |
|---|---|---|
| `PANACEA_APP_MODEL_API_KEY` | the person's own key or a gateway key (G4) | unset → no model; Discover degrades |
| `PANACEA_APP_MODEL_BASE_URL` | an origin: the corporate gateway, or wherever the owner of the deployment chose | **none** (review I13, task 02 amendment 4): unset with a key set → no model, and the log line below names both variables. Brief answers are engagement text; a key alone never sends them to a public API |
| `PANACEA_APP_MODEL_ID` | model id | `modelclient.DefaultModel` |
| `PANACEA_APP_DISCOVER_CACHE_TTL` | the revision cache's TTL (task 07) | `5m` |
| `PANACEA_APP_HUB_URL`, `PANACEA_APP_HUB_AUDIENCE`, `PANACEA_APP_OIDC_*` (`OD-30`) | **the Hub's** (Hub contract §12.2), read by C1/C2's code, not here | unset → `hub_off` |

**Interfaces — complete source:**

```go
// control-plane/cmd/api/discover.go
package main

// discoverWiring is what main needs from phase 4.
type discoverWiring struct {
	Materialiser api.Materialiser
	Discover     api.RouteMounter
	// model is the one Completer that wireDiscover hands to both
	// discover.ProposeFollowUp and repoanalysis.DraftBrief. It is an
	// untyped nil interface when the model is off: never a nil
	// *modelclient.Client, which would not compare equal to nil. main does not read it. It
	// is the seam TestAKeyWithoutABaseURLIsModelOff asserts at (re-review
	// b04-1 M3), so that test needs no bound project, remote or githost.
	model modelclient.Completer
}

// wireDiscover builds phase 4's graph. hub may be nil (the Hub is not
// configured): Discover then answers hub_off and Materialise inherits
// nothing, and Develop is unaffected.
func wireDiscover(cfg config, scanner *project.Scanner, git project.Runner, hub discoverhub.Hub, log *slog.Logger) (discoverWiring, error)

// scannerLocator adapts *project.Scanner to discoverhub.Locator. ProjectPath
// walks Scanner.Projects once per call; the list is small and already
// memoised by the scanner.
type scannerLocator struct{ s *project.Scanner }

// gitRepos implements discoverhub.RepoIdentity with two read-only commands
// through project.Runner - `git remote` and `git remote get-url <name>` -
// and githost.ParseRemote (phase 3 task 13). It returns the remotes in the
// order `git remote` lists them and does not reorder them; each RepoKey
// carries its remote's name in Remote. Choosing the primary remote is task
// 07's orderRemotes alone (review M8, b10 re-review I-1), so no order here
// can move it.
type gitRepos struct{ git project.Runner }

// gitQuestions implements contextwrite.Git with
// `git --icase-pathspecs ls-files -- <rel>` and
// `git rev-parse --git-path info/exclude`, each run in the worktree.
// ls-files WITHOUT --error-unmatch exits 0 whether or not <rel> is tracked:
// empty standard output is "untracked", any output is "tracked".
// --icase-pathspecs because a pathspec stays case-sensitive under
// core.ignorecase=true, so a tracked claude.md would read as untracked while
// NTFS/APFS opens it for CLAUDE.md (re-review R-I1); not :(icase), which
// GIT_LITERAL_PATHSPECS=1 turns into a silent "untracked". No exit
// code is read, because project.ExecRunner.Run wraps a failure with %v
// (gitrunner.go:74) and errors.As cannot find the *exec.ExitError (review
// I7). The only string match is "not a git repository" in either command's
// error, which becomes contextwrite.ErrNotARepo; any other error is returned
// as it is (a 500 at the handler, never a guess at "untracked").
type gitQuestions struct{ git project.Runner }
```

**Fallbacks, each stated and each tested:**

| Missing | Behaviour | Test |
|---|---|---|
| Hub C2 (`hubclient.New`) not merged | `hub` is nil → `hub_off` everywhere; a start-up log line says Discover needs the Hub | `TestNilHubServesHubOff` |
| C1 (`signin`) not merged | as above — no token source, no client | same |
| phase 3 task 13 (`githost`) not merged | `gitRepos` returns no keys → every project is `no_remote`, and the start-up log says why; **it does not parse remotes itself** (no second canonicaliser — HA4-6's reason) | `TestNoGithostMeansNoRemote` |
| `PANACEA_APP_MODEL_API_KEY` unset | nil `Completer`; one log line | `TestNoModelKeyIsNotFatal` |
| `PANACEA_APP_MODEL_API_KEY` set, `PANACEA_APP_MODEL_BASE_URL` unset or blank (`modelclient.ErrNoBaseURL`) | nil `Completer`, not a start-up failure; one `Warn` line: `"model off: PANACEA_APP_MODEL_API_KEY is set but PANACEA_APP_MODEL_BASE_URL is not; Discover will not call a model. Set PANACEA_APP_MODEL_BASE_URL to the gateway's origin."`. The key's value is never logged | `TestAKeyWithoutABaseURLIsModelOff` |

**Tests first** (`discover_test.go`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAKeyWithoutABaseURLIsModelOff` | `t.Setenv("PANACEA_APP_MODEL_API_KEY", "sk-test-not-a-key")`, base URL unset, `hub` nil: `wireDiscover` succeeds, and `w.model == nil` (the interface itself, so a typed-nil `*modelclient.Client` fails); the captured log holds exactly one line naming both `PANACEA_APP_MODEL_API_KEY` and `PANACEA_APP_MODEL_BASE_URL`, and not `sk-test-not-a-key`. Then, with the base URL set to an `httptest` server's origin (the server answers one minimal Messages response), `wireDiscover` again: `w.model != nil`, and one `w.model.Complete` reaches that server exactly once, with the test key in `x-api-key`. That proves the off state is the missing URL, not a broken wiring. The seam is the `model` field, not the `propose_follow_up` route: reaching the model there needs a Hub, a bound project and brief, a remote and phase 3's `githost` (task 08 `:75-83`; the fallback row above), and none of that is this test's subject. That a nil `Completer` answers `assist: "model_off"` is task 04's `TestNilCompleterIsModelOff` (`task-04.md:91`; built at `3eceb55`) | a key-only configuration that reaches `api.anthropic.com` (review I13), a typed-nil `Completer` that the model-off checks miss, or a log line that leaks the key |
| `TestWireDiscoverAlwaysGivesAMaterialiser` | with every dependency nil, `Materialiser` is non-nil and `api.New` accepts it | a wiring that passes nil and trips task 10's check at start-up |
| `TestNilHubServesHubOff` | `GET /v1/discover/projects/{id}/brief` over the wired handler → `binding.state: "hub_off"` | a panic on the first lens open |
| `TestGitQuestionsReadsUntrackedAsFalse` | a fake runner answering `ls-files` with empty output and a nil error → `(false, nil)`; answering `CLAUDE.md\n` → `(true, nil)`; the recorded args are exactly `--icase-pathspecs ls-files -- CLAUDE.md` (the worktree goes in `Run`'s `dir`, which `ExecRunner` sets as `cmd.Dir`, `gitrunner.go:55`; there is no `-C`), with no `--error-unmatch` and no `:(icase)` magic; a fake error holding `fatal: not a git repository` → `errors.Is(err, contextwrite.ErrNotARepo)`; any other fake error → returned, not `(false, nil)` | every fresh repository read as "tracked" and every brief sent to `.claude/`; an error swallowed as "untracked" |
| `TestGitQuestionsOverARealRepository` | over `project.NewExecRunner()` and real git in `t.TempDir()` (skipped when git is not on `PATH`, as `project`'s `requireGit` does; the fixture neutralises the developer's config with `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` as `makeGitRepo` does, `gitrunner_test.go:26-53`, which this package cannot import): a repository with `CLAUDE.md` committed → `Tracked(CLAUDE.md)` is `(true, nil)`; `Tracked(.claude/CLAUDE.md)` is `(false, nil)`; an untracked `CLAUDE.md` in a second repository is `(false, nil)`; a third repository whose index holds `claude.md` (lower case, committed; the fixture leaves `core.ignorecase` as `git init` sets it) → `Tracked(CLAUDE.md)` is `(true, nil)` on every platform (re-review R-I1); `ExcludePath` is absolute and ends in `info/exclude`; a plain directory, with `GIT_CEILING_DIRECTORIES` set to its parent by `t.Setenv` so no repository above `%TEMP%` can answer, → `ErrNotARepo` from both questions | the fake-runner test passing while the real runner errors on every untracked file, because a fake can return any error type (review I7) |
| `TestGitQuestionsNeverWrites` | the fake runner records only `remote`, `ls-files`, `rev-parse` subcommands; the subcommand is the first recorded argument that does not start with `-`, so `--icase-pathspecs` is skipped and `ls-files` is read (re-review R-I1) | a write through the read-only runner |
| `TestNoModelKeyIsNotFatal` | unset key → `wireDiscover` succeeds | a control plane that will not start without a model |
| `TestTheInterimMaterialiserIsGone` | a source scan of `control-plane/cmd/api/*.go`, skipping `discover_test.go` itself, finds, case-insensitively, no `materialiserUntilTask14` (so no `newMaterialiserUntilTask14` either), and `materialiser_interim.go` does not exist. The test spells the name as `"materialiser" + "UntilTask14"`, so it does not match itself (review I1) | the interim left in place or still wired, so the binary starts and every session silently inherits nothing |
| `TestOnlyProvenanceParsesKeys` | a source scan of `control-plane/internal/**/*.go` (excluding tests and `internal/provenance`, `internal/hub`) finds no `"brief:"` literal used with `strings.Split`/`Cut`/`TrimPrefix`/a regexp | P5, enforced by a test rather than a review |

**Steps:**

- [ ] **Step 1:** Check what has landed: `go doc ./control-plane/internal/hub/hubclient New`,
  `go doc ./control-plane/internal/signin`, `go doc ./control-plane/internal/githost ParseRemote`,
  `go doc ./control-plane/internal/evidence Evaluator`. Record each in the commit message. When
  the last one answers, write `evidence.go` by task 06's four key-to-subject steps, set
  `discoverhub.Config.Evaluate: evidenceEvaluate(…)`, and test each step's failure. Each failure
  test asserts an error, which `Readings` shows as `open`, and one bound clone with the branch
  asserts the `(repo, projectPath, subject)` that reaches a fake `States`.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** `discover.go`; `config.go`'s three model variables; `main.go`: build the scanner
  and runner as today, call `wireDiscover`, set `Deps.Materialiser: wiring.Materialiser` (it
  replaces task 10's `newMaterialiserUntilTask14(log)`) and `Deps.Discover`. `git rm` task 10's
  `materialiser_interim.go` and `materialiser_interim_test.go`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. The two deleted files are in the
  pathspec too.

**Acceptance:**

```
$ go test ./control-plane/... -race -count=1
ok   (every package)
$ go build -o $TEMP/pa-api.exe ./control-plane/cmd/api && echo built
built
$ git diff --stat HEAD -- control-plane/go.mod go.work
(no output — G1)
$ git grep -n "UntilTask14" -- control-plane
(one line only: discover_test.go's split spelling, "materialiser" + "UntilTask14")
$ git grep -n -e '"--error-unmatch"' -e '\.ExitCode()' -- control-plane/cmd/api control-plane/internal/contextwrite ':!*_test.go'
(no output — tracked-ness is read from ls-files' output, never from an exit code; review I7.
 The patterns are the forms code would use: the Go string literal WITH its quotes, and a method
 call. The prescribed doc comments, which say "--error-unmatch" and "*exec.ExitError" in plain
 words, do not match them; re-review R-I2, probed on git 2.53.0)
```

**Does NOT claim:** a running Hub (task 23); a real sign-in (the Hub's C1 acceptance, and `OD-12`).
