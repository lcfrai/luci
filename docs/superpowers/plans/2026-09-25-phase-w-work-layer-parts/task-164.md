### Task 164: CI outside us: `panacea release`, an Azure Pipelines task and a GitHub Action

W5, stream WP, about 3 hours (an estimate, not a measurement). Depends on task 131 (the freeze
per environment, `task-131.md:91-135`, and *shipped* on the issue), task 160 (API keys, `pak_…`,
scope `tasks-141-160.md:776-807`; its part does not exist, so every key name here is its scope's
and **UNVERIFIED**) and phase 7's P7-08 (the deploy view; whether it shows a release no Panacea
deployment joined is **UNVERIFIED**, the skeleton's row `:584`). The Hub routes are the frozen
contract's: route 35 (reserve) and route 38 (confirm), `hub-contract.md:597-600`, `:994-1041`, read
on the hub worktree. Nothing in the Hub changes (rule H1).

Parity: closes **L-149** (05 §1 K, `05-design.md:306`). Design: §1 K, §5.4 W5.

**What the reader does.** Priya's team deploys *engagement-portal* from its own Azure Pipelines
YAML, not through Ship. She adds one step after the deploy stage:

```yaml
- task: PanaceaRelease@1
  inputs:
    project: pj-7c1e0a93d2b44f10
    environment: production
    panaceaProject: engagement-portal
    hubUrl: https://hub.example.test
    apiKeyVariable: PANACEA_API_KEY   # a secret variable; the key itself is never an input
```

Build 4242 runs; the step prints `rl-… confirmed · pinned · brief:bf-…@6` and exits 0. In the
cockpit the release sits on the thread like one Ship made: reserved by Priya's key, its pin taken
at reservation. A second try of the same stage records nothing new. When an admin has frozen
`production`, the step fails with *"production is frozen by Ada Admin since 09:12 · release
train"* and exit 3, before any reservation.

**Decisions made here, so nobody re-derives them:**

1. **Two binaries.** The CLI is `panacea-release`; the docs call it *panacea release*. Phase T's
   in-sandbox `panacea` (ST.1) must never hold a release command, so an agent in a sandbox cannot
   reach this one by a verb name.
2. **Three calls, in this order, and no others:** read the freezes (task 131's proposed `GET
   /v1/projects/{projectId}/freezes`, `task-131.md:105`), reserve (route 35, `kind:"run"`), confirm
   (route 38, `{pipelineRun}`, the reserver only). No abandon: a confirm refused leaves the
   reservation to expire in 15 minutes (`hub-contract.md:1021`, ADR R3), which is harmless.
3. **The freeze is read by the CLI, and that is not enforcement (gap 1).** Task 131 checks a
   freeze in phase 7's Ship handler, before its reserve (`task-131.md:51-56`); route 35 itself
   checks nothing. A caller with a key who skips this CLI can still reserve in a frozen
   environment. The CLI's read makes the honest path refuse; closing the other path is a Hub
   precondition on route 35, asked for, not built here.
4. **The run id names the environment.** `(panaceaProject, pipelineRun)` is unique in the org
   (`hub-contract.md:1040`), and one Azure build often deploys two stages. So `pipelineRun` is
   `ado-<Build.BuildId>.<environment>` or `gh-<run_id>-<run_attempt>.<environment>`, not the
   scope's `ado-<id>` (gap 2). Every part is checked to the Hub's grammar, 1-253 of `[a-z0-9.-]`;
   anything else is refused before any call, never rewritten.
5. **Idempotency keys are derived, not random.** Reserve sends `Idempotency-Key: rs-<first 40 hex
   of SHA-256(projectId, environment, pipelineRun, commit)>`, confirm `cf-<the same>`; both fit
   `[A-Za-z0-9_-]{1,64}` (`hub-contract.md:50-55`). A dropped connection, and a re-run of the same
   stage within the Hub's 24 h, replay the first answer instead of reserving twice.
6. **Retries.** A connection error, a 429 or a 5xx retries at 1, 2, 4 and 8 s, or `Retry-After`
   when larger, capped at 30 s; after the fourth, exit 4. A 4xx other than 429 never retries.
7. **The key is the key's person.** `PANACEA_API_KEY` goes only as `Authorization: Bearer pak_…`
   (task 160). It is read from the environment, never a flag, never logged: `--verbose` prints
   method, path, status and `Idempotent-Replay`, no header. Whether task 160 lets a key reserve a
   release is its matrix's to say (**UNVERIFIED**, gap 3).
8. **It sends no issue list.** A release's issues come from the `Panacea-Issue` trailers the
   product wrote (SW.9, tasks 71 and 131). The request body has exactly route 35's fields.
9. **The wrappers download one pinned binary and check it.** The version, the four asset digests
   and the download base live in `index.js` and `action.yml`, not in `task.json`: Microsoft's
   `task.json` schema is theirs, and whether an unknown key survives upload is **UNVERIFIED**.

**Files** (the skeleton's File map row `:268`):
- `control-plane/cmd/panacea-release/main.go` (new) — flags, the environment, exit codes
- `control-plane/cmd/panacea-release/release.go` (new) — the three calls, retries, the output
- `control-plane/cmd/panacea-release/runid.go` (new) — `pipelineRun` and the idempotency keys
- `control-plane/cmd/panacea-release/{release,runid,wrappers}_test.go` (new) — against an `httptest` Hub; the wrapper checks
- `integrations/azure-pipelines-task/{task.json,index.js,package.json,README.md}` (new; `package.json` pins `azure-pipelines-task-lib`, MIT, not in the scope's list)
- `integrations/github-action/{action.yml,README.md}` (new)

**What is on the tree today (verified 2026-09-26).** On `feat/work-layer`, `control-plane/cmd/`
holds only `api/`, and there is no `integrations/` directory. Task 131's part exists (the proposed
`EnvironmentFreeze` and its `GET …/freezes`, `task-131.md:91-107`); task 160's does not. Phase 7's
task 08 (`phase-7` worktree at `2427f7b5`) draws a confirmed run release that matches no
deployment as a `pending` row, with an age cap (`task-08.md:281-284`, `:353`). No Hub, phase 7 or
Phase T file is merged here.

#### `runid.go` (complete)

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	panaceaName = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)  // hub-contract.md:1008
	runGrammar  = regexp.MustCompile(`^[a-z0-9.-]{1,253}$`) // hub-contract.md:1029
	digits      = regexp.MustCompile(`^[0-9]{1,20}$`)
	commitHex   = regexp.MustCompile(`^[0-9a-f]{40}$`)          // hub-contract.md:1008-1009
	// pj-… and bf-… are opaque (hub-contract.md:46): the CLI checks the prefix only,
	// and the Hub's 404 or 422 is the answer to anything else (exit 3).
)

// RunIDError is a usage error: exit 2, nothing sent.
type RunIDError struct{ Field, Value, Want string }

func (e *RunIDError) Error() string {
	return fmt.Sprintf("%s %q is not %s; nothing was sent", e.Field, e.Value, e.Want)
}

// PipelineRun builds the Hub's pipelineRun for one environment (decision 4). source is
// "ado" (id = Build.BuildId) or "gh" (id = run_id, attempt = run_attempt). It never
// rewrites a value: "Build_42" is refused, not lowercased into something else.
func PipelineRun(source, id, attempt, environment string) (string, error) {
	if !panaceaName.MatchString(environment) {
		return "", &RunIDError{"--environment", environment, "1-63 of [a-z0-9-]"}
	}
	if !digits.MatchString(id) {
		return "", &RunIDError{"--pipeline-run", id, "digits"}
	}
	var run string
	switch source {
	case "ado":
		if attempt != "" {
			return "", &RunIDError{"--run-attempt", attempt, "empty for ado"}
		}
		run = "ado-" + id + "." + environment
	case "gh":
		if !digits.MatchString(attempt) {
			return "", &RunIDError{"--run-attempt", attempt, "digits"}
		}
		run = "gh-" + id + "-" + attempt + "." + environment
	default:
		return "", &RunIDError{"--source", source, `"ado" or "gh"`}
	}
	if !runGrammar.MatchString(run) {
		return "", &RunIDError{"pipelineRun", run, "1-253 of [a-z0-9.-]"}
	}
	return run, nil
}

// Keys derives the reserve and confirm Idempotency-Keys (decision 5). Same inputs,
// same keys, in any process: a re-run of a stage replays instead of reserving twice.
func Keys(projectID, environment, run, commit string) (reserve, confirm string) {
	sum := sha256.Sum256([]byte(strings.Join([]string{projectID, environment, run, commit}, "\x00")))
	h := hex.EncodeToString(sum[:])[:40]
	return "rs-" + h, "cf-" + h // 43 characters of [a-z0-9-]
}
```

`--pipeline-run` takes the CI's raw number, and `--source` says which CI; `--run-attempt` is
GitHub's only. The scope's single `--pipeline-run <id>` flag becomes these three (gap 2), because
the CLI, not each wrapper, owns the grammar.
