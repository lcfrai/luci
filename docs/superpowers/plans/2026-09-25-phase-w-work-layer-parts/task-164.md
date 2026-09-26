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

#### The command line — `main.go` (contract complete)

```text
panacea-release create --project pj-… --environment <name> --panacea-project <name>
                       --commit <40 hex> --source ado|gh --pipeline-run <digits>
                       [--run-attempt <digits>] [--branch refs/heads/…] [--brief bf-…]
                       [--format json|env] [--verbose]
environment:  PANACEA_HUB_URL (https only; http only when PANACEA_RELEASE_INSECURE_LOOPBACK=1
              and the host is 127.0.0.1), PANACEA_API_KEY (required; begins pak_)
stdout:       --format json → one object, below; --format env → "release_id=rl-…\nstate=confirmed\n"
stderr:       one plain line: "rl-… confirmed · pinned · brief:bf-…@6", or the refusal sentence
exit:         0 confirmed · 2 usage · 3 refused · 4 the Hub unreachable after retries
```

`--panacea-project` is **required**, not the scope's optional flag: route 35's body requires
`panaceaProject` (`hub-contract.md:998`, `:1008`) and the CLI invents no default (gap 4).

#### The wire and the calls — `release.go` (complete in shape)

```go
package main

// reserveBody is route 35's run body, exactly its fields (hub-contract.md:998-1000); there is
// no issues field and no other key (decision 8). designKey and sourceDeploymentId are null:
// a CI outside us knows no approved design and re-runs no Panacea deployment.
type reserveBody struct {
	Kind               string  `json:"kind"` // "run"
	PanaceaProject     string  `json:"panaceaProject"`
	Environment        string  `json:"environment"`
	BranchRef          string  `json:"branchRef"` // "" when --branch is absent
	Commit             string  `json:"commit"`
	BriefID            *string `json:"briefId"`
	DesignKey          *string `json:"designKey"`
	SourceDeploymentID *string `json:"sourceDeploymentId"`
}

type confirmBody struct {
	PipelineRun string `json:"pipelineRun"`
}

// release is the part of the Hub's Release the CLI reads (hub-contract.md:976-991).
type release struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Pin   struct {
		Status   string  `json:"status"`
		Reason   string  `json:"reason"`
		BriefKey *string `json:"briefKey"`
	} `json:"pin"`
}

// Output is stdout's one object.
type Output struct {
	ReleaseID string    `json:"releaseId"`
	State     string    `json:"state"` // "confirmed" on exit 0
	Pin       OutputPin `json:"pin"`
}
type OutputPin struct {
	Status   string  `json:"status"`
	Reason   string  `json:"reason"`
	BriefKey *string `json:"briefKey,omitempty"`
}

// Refusal is exit 3: the Hub's problem code and its sentence, never the request.
type Refusal struct {
	Status int
	Code   string // forbidden, not_reserver, key_revoked, deploy_frozen, release_expired, conflict, …
	Line   string
}

// Unreachable is exit 4: the last cause, after decision 6's four retries.
type Unreachable struct{ Last error }

type Config struct {
	HubURL  string
	Key     string // never formatted into any error, line or log
	Client  *http.Client
	Sleep   func(time.Duration) // tests pass a recorder
	Verbose io.Writer           // nil unless --verbose
}

// Create runs decision 2's three calls in order and returns Output, *Refusal,
// *Unreachable or *RunIDError. A freeze answers *Refusal{409, "deploy_frozen",
// "<env> is frozen by <name> since <HH:MM> · <reason>"} with no reserve sent, the
// sentence being task 131's FrozenError text (task-131.md:126-129).
func Create(ctx context.Context, cfg Config, a Args) (Output, error)

// Args are the parsed flags, validated by runid.go before any call.
type Args struct {
	ProjectID, Environment, PanaceaProject, Commit, PipelineRun, BranchRef string
	BriefID                                                            *string
}
```

**How each answer maps**, in `Create`:

| Call | Answer | Result |
|---|---|---|
| `GET …/freezes` | 200, an item for `environment` | `*Refusal` code `deploy_frozen` (the CLI's, status 0: the Hub said 200), task 131's sentence; nothing reserved |
| `GET …/freezes` | 404 whose code is not `project_not_found` (the route is not yet on this Hub, task 131's gap 1) | one stderr line *"freeze control not available on this Hub"*, then reserve (task 131 decision 5: no record, nothing frozen) |
| any | 401, 403, 404, 409, 422 | `*Refusal`, the Hub's `code` and `detail`; no retry |
| any | a connection error, 429, 5xx | decision 6's retries, the same `Idempotency-Key`; then `*Unreachable` |
| reserve | 201, or 200 with `Idempotent-Replay: true` | confirm next, with the confirm key |
| confirm | 200 `confirmed` | `Output`, exit 0 |

That 404 arm is the one place the CLI proceeds past a failed read, and it is **deliberate and
temporary**: it mirrors task 131's *nothing is frozen until the record exists*, and the step that
lands task 131's gap 1 deletes it (Step 1 checks). Which problem code the Hub's router gives an
unknown path is **UNVERIFIED**.

#### The Azure Pipelines task — `integrations/azure-pipelines-task/`

```jsonc
// task.json (complete; the id is fixed once, at Step 5, and never changes after)
{ "id": "<a GUID minted at Step 5>", "name": "PanaceaRelease", "friendlyName": "Panacea release",
  "description": "Record the release this pipeline made on the Panacea thread.",
  "category": "Deploy", "author": "Panacea", "version": { "Major": 1, "Minor": 0, "Patch": 0 },
  "instanceNameFormat": "Panacea release: $(environment)",
  "inputs": [
    { "name": "project", "type": "string", "label": "Panacea project id (pj-…)", "required": true },
    { "name": "environment", "type": "string", "label": "Environment", "required": true },
    { "name": "panaceaProject", "type": "string", "label": "Deploy project name", "required": true },
    { "name": "hubUrl", "type": "string", "label": "Hub URL", "required": true },
    { "name": "apiKeyVariable", "type": "string", "label": "Secret variable holding the key",
      "defaultValue": "PANACEA_API_KEY", "required": true },
    { "name": "brief", "type": "string", "label": "Brief id (bf-…)", "required": false } ],
  "execution": { "Node20_1": { "target": "index.js" } } }
```

```js
// index.js (complete in shape). azure-pipelines-task-lib is MIT; the only dependency.
const tl = require("azure-pipelines-task-lib/task");
const PIN = { version: "<set at Step 5>", base: "<the artefact host, gap 5>",
  sha256: { "linux-x64": "<hex>", "linux-arm64": "<hex>", "win32-x64": "<hex>" } };

async function run() {
  const key = tl.getVariable(tl.getInput("apiKeyVariable", true)); // a secret: masked in logs
  if (!key) return tl.setResult(tl.TaskResult.Failed, "the secret variable is empty or unset");
  const bin = await fetchPinned(PIN);            // download, SHA-256, refuse on mismatch
  const code = await tl.execAsync(bin, ["create",
    "--project", tl.getInput("project", true), "--environment", tl.getInput("environment", true),
    "--panacea-project", tl.getInput("panaceaProject", true),
    "--commit", tl.getVariable("Build.SourceVersion"), "--source", "ado",
    "--pipeline-run", tl.getVariable("Build.BuildId"),
    ...(/^refs\/heads\//.test(tl.getVariable("Build.SourceBranch") || "")  // a PR merge ref sends none
      ? ["--branch", tl.getVariable("Build.SourceBranch")] : []),
    ...(tl.getInput("brief") ? ["--brief", tl.getInput("brief")] : [])],
    { env: { ...process.env, PANACEA_HUB_URL: tl.getInput("hubUrl", true), PANACEA_API_KEY: key },
      ignoreReturnCode: true });
  tl.setResult(code === 0 ? tl.TaskResult.Succeeded : tl.TaskResult.Failed, `exit ${code}`);
}
run();
```

`fetchPinned` picks the asset for `process.platform`-`process.arch`, refuses any other pair,
downloads to the agent's temp directory, and compares SHA-256 before `chmod 755`; a mismatch fails
the step and runs nothing. That `tl.getVariable` returns a secret variable to a task, and that
`execAsync` takes `env`, are the library's documented behaviour as remembered, **UNVERIFIED**
against its current release (Step 5 checks).

#### The GitHub Action — `integrations/github-action/action.yml` (complete)

```yaml
name: Panacea release
description: Record the release this workflow made on the Panacea thread.
inputs:
  project:         { description: "Panacea project id (pj-…)", required: true }
  environment:     { description: "Environment name", required: true }
  panacea-project: { description: "Deploy project name", required: true }
  hub-url:         { description: "Hub URL", required: true }
  api-key:         { description: "The pak_ key; pass a secret, never a literal", required: true }
  brief:           { description: "Brief id (bf-…)", required: false, default: "" }
outputs:
  release-id: { description: "The confirmed release", value: "${{ steps.rel.outputs.release_id }}" }
runs:
  using: composite
  steps:
    - id: rel
      shell: bash
      env:
        PANACEA_HUB_URL: ${{ inputs.hub-url }}
        PANACEA_API_KEY: ${{ inputs.api-key }}
        PIN_VERSION: "<set at Step 5>"
        PIN_BASE: "<the artefact host, gap 5>"
        PIN_SHA256_LINUX_X64: "<hex>"
        PIN_SHA256_LINUX_ARM64: "<hex>"
        PIN_SHA256_WIN32_X64: "<hex>"
        IN_PROJECT: ${{ inputs.project }}
        IN_ENV: ${{ inputs.environment }}
        IN_PPROJ: ${{ inputs.panacea-project }}
        IN_BRIEF: ${{ inputs.brief }}
      run: bash "$GITHUB_ACTION_PATH/run.sh"
```

Inputs reach the script only as environment variables, never interpolated into the `run:` text,
so a crafted input cannot become shell. `run.sh` (in the same directory; one more file than the
scope lists, gap 6) picks the asset for `$RUNNER_OS`-`$RUNNER_ARCH`, downloads it, compares
`sha256sum` to the pin and exits 1 on a mismatch, then runs `create --source gh --pipeline-run
"$GITHUB_RUN_ID" --run-attempt "$GITHUB_RUN_ATTEMPT" --commit "$GITHUB_SHA"`, `--branch "$GITHUB_REF"`
only when it begins `refs/heads/`, `--format env >> "$GITHUB_OUTPUT"`, and exits with the CLI's code.

#### Tests to write first (`control-plane/cmd/panacea-release/`, against an `httptest` Hub)

The fake Hub records every request and **fails the test on any method and path outside the three**
(`GET …/freezes`, route 35, route 38). `Sleep` is a recorder, so retry tests take no wall time. The fake is `httptest.NewTLSServer`, its
`Client()` passed as `Config.Client`, so no test needs the loopback escape.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestReserveThenConfirmInOrder` | freezes, reserve, confirm, in that order; exit 0; stdout's `releaseId` is the reserved id | confirm skipped, so the reservation expires |
| `TestAForbiddenIsExitThreeWithNoRetry` | reserve → 403 `forbidden`: exit 3, one reserve request, `Sleep` never called | a refusal retried |
| `TestADroppedConnectionRetriesWithTheSameKey` | the first reserve's connection is hijacked and closed; the second answers 201 → two requests, one `Idempotency-Key`, `Sleep(1s)` once | a second reservation under a new key |
| `TestAStageRerunReplaysTheRelease` | two `Create` calls, fresh clients, same args → both send `rs-<h>` then `cf-<h>`; the fake replays with `Idempotent-Replay: true`; one release | a random key per process |
| `TestAFrozenEnvironmentExitsThreeAndNamesWho` | freezes lists `production` by Ada at 09:12 → exit 3, stderr *"production is frozen by Ada Admin since 09:12 · release train"*, no reserve | the freeze swallowed as exit 1, or read after the reserve |
| `TestNoFreezeRouteIsNotAFreeze` | freezes → 404 `not_found` → one stderr line, then reserve; 404 `project_not_found` → exit 3 | a missing project read as *not frozen* |
| `TestTheKeyNeverAppears` | with `--verbose`, over a 201, a 403 and an unreachable run: stdout, stderr and every error string hold no substring of the key | the key echoed under `--verbose` |
| `TestAnUnderscoreRunIsRefused` | `--pipeline-run Build_42` → exit 2, zero requests | a silent rewrite |
| `TestTwoEnvironmentsAreTwoRuns` | build 4242 to `staging` then `production` → `ado-4242.staging`, `ado-4242.production` | one run id, so the second is 409 `conflict` |
| `TestTheBodyHasNoIssues` | the reserve body's keys are exactly route 35's eight | an issue list, or a state move, from CI |
| `TestUnreachableIsExitFour` | every attempt refused → five requests, `Sleep` 1, 2, 4, 8 s; exit 4 | endless retries, or exit 3 on an outage |
| `TestRetryAfterIsHonoured` | 429 with `Retry-After: 12` → `Sleep(12s)`; `Retry-After: 90` → `Sleep(30s)` | the header ignored, or unbounded |
| `TestAnExpiredConfirmIsExitThree` | confirm → 409 `release_expired` → exit 3, the code on stderr | an expired reservation reported confirmed |
| `TestPlainHTTPIsRefused` | `PANACEA_HUB_URL=http://hub.example.test` → exit 2, zero requests | the key sent in clear |

`wrappers_test.go` (pure file reads, no network): `TestTaskJSONParsesAndTargetsIndexJS` (kills a
`task.json` typo), `TestActionNamesEveryDocumentedInput` (each backticked input in `README.md`'s
inputs table is a key under `inputs:`, both ways; kills a drifted README), `TestKeyIsNeverAPlainInput`
(`task.json` has no input named like `key` other than `apiKeyVariable`; `action.yml`'s `run:` text
holds no `${{ inputs.`; kills a key in a pipeline log and an injectable step) and
`TestBothWrappersPinOneVersion` (kills one wrapper left on an old binary).
