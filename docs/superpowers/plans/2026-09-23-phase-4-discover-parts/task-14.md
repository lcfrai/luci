### Task 14: Wiring and configuration in `cmd/api`

Turns the packages on. After this task a control plane started with the Hub's variables serves
Discover, and every session start materialises. **Depends on** tasks 09–13; on the Hub's **C2**
(`hubclient.New`, the real client) and **C1** (`entraauth`, the `TokenSource`); on **phase 3 task
13** (`githost.ParseRemote`, `Remote.RepoKey`). Each missing dependency has a stated fallback
below, so this task never waits silently and never fabricates.

**Files:**
- `control-plane/cmd/api/discover.go` — `wireDiscover`, the three adapters (new)
- `control-plane/cmd/api/discover_test.go` (new)
- `control-plane/cmd/api/main.go` — one call to `wireDiscover`, two `Deps` fields; the
  `Materialiser` field's value `newMaterialiserUntilTask14(log)` (task 10) becomes
  `wiring.Materialiser` (modify)
- `control-plane/cmd/api/config.go` — the model variables (modify)
- `control-plane/cmd/api/materialiser_interim.go` and `materialiser_interim_test.go` — task 10's
  interim `materialiserUntilTask14` (**delete**, `git rm`; review I1)

**Configuration** — every variable, read in `config.go` exactly as phase 2 reads its own:

| Variable | Meaning | Default |
|---|---|---|
| `PANACEA_APP_MODEL_API_KEY` | the person's own key or a gateway key (G4) | unset → no model; Discover degrades |
| `PANACEA_APP_MODEL_BASE_URL` | an origin | `https://api.anthropic.com` |
| `PANACEA_APP_MODEL_ID` | model id | `modelclient.DefaultModel` |
| `PANACEA_APP_DISCOVER_CACHE_TTL` | the revision cache's TTL (task 07) | `5m` |
| `PANACEA_APP_HUB_URL`, `PANACEA_APP_HUB_SCOPE`, `PANACEA_APP_ENTRA_*` | **the Hub's** (Hub contract §12.2), read by C1/C2's code, not here | unset → `hub_off` |

**Interfaces — complete source:**

```go
// control-plane/cmd/api/discover.go
package main

// discoverWiring is what main needs from phase 4.
type discoverWiring struct {
	Materialiser api.Materialiser
	Discover     api.RouteMounter
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
// and githost.ParseRemote (phase 3 task 13). origin first, then by name.
type gitRepos struct{ git project.Runner }

// gitQuestions implements contextwrite.Git with `git ls-files
// --error-unmatch -- <rel>` and `git rev-parse --git-path info/exclude`.
// Exit status 1 from ls-files is "untracked", not an error; "not a git
// repository" in git's message is contextwrite.ErrNotARepo.
type gitQuestions struct{ git project.Runner }
```

**Fallbacks, each stated and each tested:**

| Missing | Behaviour | Test |
|---|---|---|
| Hub C2 (`hubclient.New`) not merged | `hub` is nil → `hub_off` everywhere; a start-up log line says Discover needs the Hub | `TestNilHubServesHubOff` |
| C1 (`entraauth`) not merged | as above — no token source, no client | same |
| phase 3 task 13 (`githost`) not merged | `gitRepos` returns no keys → every project is `no_remote`, and the start-up log says why; **it does not parse remotes itself** (no second canonicaliser — HA4-6's reason) | `TestNoGithostMeansNoRemote` |
| `PANACEA_APP_MODEL_API_KEY` unset | nil `Completer`; one log line | `TestNoModelKeyIsNotFatal` |

**Tests first** (`discover_test.go`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWireDiscoverAlwaysGivesAMaterialiser` | with every dependency nil, `Materialiser` is non-nil and `api.New` accepts it | a wiring that passes nil and trips task 10's check at start-up |
| `TestNilHubServesHubOff` | `GET /v1/discover/projects/{id}/brief` over the wired handler → `binding.state: "hub_off"` | a panic on the first lens open |
| `TestGitQuestionsReadsUntrackedAsFalse` | a fake runner exiting 1 for `ls-files` → `(false, nil)` | every fresh repository read as "tracked" and every brief sent to `.claude/` |
| `TestGitQuestionsNeverWrites` | the fake runner records only `remote`, `ls-files`, `rev-parse` subcommands | a write through the read-only runner |
| `TestNoModelKeyIsNotFatal` | unset key → `wireDiscover` succeeds | a control plane that will not start without a model |
| `TestTheInterimMaterialiserIsGone` | a source scan of `control-plane/cmd/api/*.go`, skipping `discover_test.go` itself, finds, case-insensitively, no `materialiserUntilTask14` (so no `newMaterialiserUntilTask14` either), and `materialiser_interim.go` does not exist. The test spells the name as `"materialiser" + "UntilTask14"`, so it does not match itself (review I1) | the interim left in place or still wired, so the binary starts and every session silently inherits nothing |
| `TestOnlyProvenanceParsesKeys` | a source scan of `control-plane/internal/**/*.go` (excluding tests and `internal/provenance`, `internal/hub`) finds no `"brief:"` literal used with `strings.Split`/`Cut`/`TrimPrefix`/a regexp | P5, enforced by a test rather than a review |

**Steps:**

- [ ] **Step 1:** Check what has landed: `go doc ./control-plane/internal/hub/hubclient New`,
  `go doc ./control-plane/internal/entraauth`, `go doc ./control-plane/internal/githost ParseRemote`.
  Record each in the commit message.
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
```

**Does NOT claim:** a running Hub (task 23); a real sign-in (the Hub's C1 acceptance, and `OD-12`).
