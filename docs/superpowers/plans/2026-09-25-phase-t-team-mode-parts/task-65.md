### Task 65: Phase 8 delta: the CLI bump job and its four gates

**Slice TM2. Depends on: 42, P8-27, P8-27a. About 3 hours. Seam: none new; extends P8-D8 (images
by digest) and task 27's `image-digests` artefact.**

OD-20 wants the cloud to track the latest Claude Code CLI quickly, because about 90% of people
should never run locally (`05-design.md:1098-1113`). Today the pin is one literal line,
`RUN npm install -g @anthropic-ai/claude-code@2.1.274` (`images/sandbox/Dockerfile:16-17` on
`.claude/worktrees/env`, read 2026-09-25, "observed via `npm view … version` on 2026-09-17"), and
the sandbox image digest is also OD-8's adapter pin (P8-D8; phase 8 `parts/task-27.md:5-8`). This
task adds the job that notices a release, builds it by digest, runs **four gates**, and only then
lets the digest reach the warm pool (phase 8 task 12's balloons and pre-pull,
`parts/task-12.md:30-47`).

Everything it extends is phase 8's **plan**, not code: `images/build.sh`, `pipelines/images.yml`
and the `warm` values are tasks 27, 27a and 12 (**UNVERIFIED until merge**; only
`images/sandbox/Dockerfile` exists on the sibling today). This task starts after `feat/env`'s
sync-merge.

The four gates (`05-design.md:1104-1108`, the scope's wording):

| Gate | What it proves | Where it runs |
|---|---|---|
| **G1 canary** | OD-8: phase 3 task 17's transcript-adapter canary passes against the new image | the CI agent, task 27's `canary` job, unchanged |
| **G2 channel** | the `panacea` channel still works: the managed file loads, the `PreToolUse` hook runs `panacea check --hook` **before** an edit (task 13's invariant I5, `task-13.md:78`), and the verbs answer over the socket | the CI agent, the new image, phase 6 task 16's fake upstream |
| **G3 G9** | task 42's injection corpus, the live run, passes on the new CLI (`task-42.md:340-380`) | a Job in the dev cluster on the pushed digest |
| **G4 gateway** | a real call through phase 6's gatekeeper succeeds, and the gatekeeper refused nothing the CLI sent | a Job in the dev cluster on the pushed digest |

Four properties, each pinned by a test.

1. **A failing gate blocks promotion.** The `image-digests` artefact, the only thing task 30's
   deploy consumes (phase 8 `parts/task-27.md:56-59`), is published **after** the four gates and
   only when each wrote `pass` for the same digest. For a CLI version change, `skipped` is a
   failure: the gates exist for that change.
2. **The promoted reference is a digest, never a tag.** Every value the artefact carries matches
   task 12's schema, `…@sha256:[a-f0-9]{64}$` (`parts/task-12.md:50-51`); the publisher refuses
   anything else.
3. **A running session keeps its version.** Promotion changes what new pods start from. No
   running pod is restarted, patched or signalled; its agent record keeps the `Harness.Version`
   it started with, which is what its card shows.
4. **Nothing parses the CLI.** The version comes from the image label the build writes
   (`PANACEA_APP_CLAUDE_VERSION`, the name task 42 already reads, `task-42.md:352`). G2 and G4
   judge by our own components (the socket's record, the gatekeeper's body-free log) and by exit
   status, never by the CLI's output (OD-8, global constraint 10).

Five decisions the scope left open, made here.

**(a) How the job learns of a release — UNVERIFIED as §10.4 says.** `pipelines/scripts/cli-latest.sh`
asks the pool's configured npm registry for `dist-tags.latest` of `@anthropic-ai/claude-code`,
exactly the source the pin's comment already cites. It accepts only a stable `MAJOR.MINOR.PATCH`
strictly greater than the pin; a pre-release, the `next` tag, or an older version prints why and
exits 0 with nothing to do. Whether the Managed DevOps Pool reaches that registry through the
corporate proxy is **UNVERIFIED**; the Dockerfile's own `npm install` already needs it, so the
first images run answers it.

**(b) The pin moves by pull request, not by a build argument.** A version that is built but not
written down would be undone by the next ordinary `dev` push, which rebuilds from the repository.
So the pin becomes one file, `images/sandbox/claude-code.version`, and a scheduled pipeline,
`pipelines/cli-bump.yml`, opens a PR to `dev` that changes that one line, with auto-complete set.
The PR build runs G1 and G2 (no connection, as task 27 requires of PR builds); G3 and G4 run after
the merge, on `dev`, before the artefact is published. The PR needs a repository right the four
pipeline identities do not have (P8-D16): gap 1.

**(c) The mirror moves with the CLI.** §8.2 pins the lane mirror's git by digest and bumps it with
the CLI image (`05-design.md:898`). Task 58's mirror image has its own pinned AzureLinux base and
git package (`task-58.md:392-405`), not the sandbox base, and task 58 says its digest "moves only
with the CLI bump job's gates" (`task-58.md:72-75`). So this task adds `images/mirror` to
`build.sh` and `mirror` to `digests.json`: every run that publishes a new sandbox digest publishes
the mirror digest built beside it, through the same four gates. A git bump in the mirror's
Dockerfile is an ordinary PR that rides the next publish. Fleet passes the digest in task 58's
`PUT /v1/mirrors/{projectId}` body (`task-58.md:316`) from `PANACEA_APP_FLEET_MIRROR_IMAGE`.

**(d) Promotion is task 30's deploy, as ever.** On `dev` the deploy that consumes a newly published
`image-digests` sets `warm.images.sandbox` and `warm.images.machine`, the broker's session image,
and Fleet's launch version (below). Prod takes the same artefact through task 30's Environment
approval, a human click. The 24-hour target (`05-design.md:1112`) is for `dev`; prod's is the
approver's (gap 2).

**(e) The version a card shows is the one its pod started with.** `digests.json` gains
`"claudeCode": "<the label>"`. The deploy passes it to Fleet as `PANACEA_APP_FLEET_CLAUDE_VERSION`,
and task 20's `LaunchRequest.Version` (`task-20.md:154`) is read from that config at each launch,
so the Hub's agent record carries the version of the digest that pod was created from. A Restart is
a new pod and a new record.

**Files:**
- `images/sandbox/claude-code.version` — one line, `2.1.274` today (new)
- `images/sandbox/Dockerfile` — the pin read from a build argument, and the label (modify; built)
- `images/build.sh` (phase 8 task 27) — reads the version file into `--build-arg`; `digests.json`
  gains `claudeCode` from the label (modify; planned)
- `pipelines/cli-bump.yml` + `pipelines/scripts/cli-latest.sh`, `cli-pr.sh` — the scheduled
  detector and the PR (new)
- `pipelines/images.yml` (phase 8 task 27) — the `channel` job (G2) beside `canary` (G1); on
  `dev`/`main`, `gates-live` (G3, G4) after `push`; `publish` after all four (modify; planned)
- `pipelines/scripts/gate-channel.sh`, `gate-live.sh`, `publish-digests.sh` + one test script,
  `pipelines/scripts/cli-bump_test.sh` (new)
- `infra/smoke/bump/{g9-job.yaml,gateway-job.yaml}` — the two dev-cluster Jobs (new)
- `deploy/charts/fleet/values.yaml`, `templates/deployment.yaml` (task 32) —
  `PANACEA_APP_FLEET_CLAUDE_VERSION` from the artefact (modify)
- `control-plane/cmd/fleetd/config.go` (task 12), `internal/fleet/sessions/sessions.go` (task 20) —
  the version read at each launch (modify; one field, one line)

**`images/sandbox/Dockerfile`, the changed lines, complete** (replacing `:15-17`):

```dockerfile
# Agent harnesses. Pinned so a sandbox image is reproducible. The version is
# images/sandbox/claude-code.version, passed by images/build.sh; the CLI bump
# pipeline (pipelines/cli-bump.yml) moves it by pull request, never by hand-edit here.
ARG CLAUDE_CODE_VERSION
RUN test -n "$CLAUDE_CODE_VERSION" \
    && npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"
LABEL PANACEA_APP_CLAUDE_VERSION="${CLAUDE_CODE_VERSION}"
```

`test -n` makes a build without the argument fail loudly rather than install `latest`. The label
is written by the build from the same argument; nothing reads `claude --version`.

**`pipelines/cli-bump.yml`, complete.**

```yaml
# pipelines/cli-bump.yml — notices a Claude Code release and opens the one-line pin PR (task 65).
# It builds nothing and holds no registry or cluster connection. The PR's own images build runs
# G1 and G2; G3 and G4 run on dev after the merge (pipelines/images.yml).
trigger: none
pr: none
schedules:
  - cron: "0 */6 * * *"          # every 6 hours, UTC: four chances inside the 24-hour target
    displayName: claude-code release check
    branches:
      include: [dev]
    always: true                 # run even when dev has not changed; a release is outside the repo

jobs:
  - job: detect
    pool:
      name: $(PANACEA_APP_CI_POOL)  # task 30's Managed DevOps Pool (phase 8 parts/task-30.md:200)
    steps:
      - checkout: self
        persistCredentials: false
      - bash: pipelines/scripts/cli-latest.sh images/sandbox/claude-code.version > $(Agent.TempDirectory)/new.txt
        displayName: latest stable release, or nothing
      - bash: pipelines/scripts/cli-pr.sh "$(cat $(Agent.TempDirectory)/new.txt)"
        displayName: open or refresh the bump PR
        condition: and(succeeded(), ne(variables['Build.Reason'], 'PullRequest'))
        env:
          SYSTEM_ACCESSTOKEN: $(System.AccessToken)   # the build service's token; gap 1
```

`cli-latest.sh <pinfile>` prints the new version, or nothing and exit 0 when there is none
(decision a). `cli-pr.sh <version>` does nothing on an empty argument; otherwise, through the Azure
DevOps REST API with that token only, it creates branch `bump/claude-code-<version>` from `dev`
with the one-line change, opens a PR titled `images: Claude Code <old> → <version>` with
auto-complete, and if that PR already exists, does nothing. It never force-pushes and never
touches another branch. A second, newer release while a bump PR is open opens a second PR; the
older one is abandoned by the script with a comment naming the newer.

**`pipelines/images.yml`, the job graph after this task** (task 27's jobs unchanged unless named):

```text
PR and dev/main:  build → sbom → scan → canary (G1) ─┐
                                      channel (G2) ──┴→ [PR stops here: no connection]
dev/main only:    push (task 27; digests.json kept in the run, NOT yet the artefact)
                  → gates-live: g9 (G3), gateway (G4)   (cluster connection, dev environment)
                  → publish: publish-digests.sh → artefact image-digests  (task 30 consumes it)
```

`push` still pushes by digest to the environment's registry, so the Jobs in `gates-live` can
pull what they test; a pushed digest that never gets an `image-digests` artefact is inert,
because task 30's deploy reads only that artefact and `helm-values.sh` takes digests from nowhere
else (phase 8 `parts/task-27.md:69-71`). On `main`, `gates-live` runs against **dev's** cluster
and gateway: prod is never the test bed. G3 runs only when the sandbox image's label differs
from the `claudeCode` of the last published `image-digests` for that branch (a sandboxd-only
change still runs G1, G2 and G4); `publish-digests.sh` records a skipped G3 as `not-required`
and accepts it only in that case.

**The gate result, one file per gate** (`images/.out/gates/<gate>.json`, complete):

```json
{"gate":"channel","image":"acrpanaceaappdevk7q2.azurecr.io/panacea-app/sandbox@sha256:<64 hex>",
 "claudeCode":"2.1.280","result":"pass","reason":"","at":"2026-09-25T06:14:09Z","run":"20260925.3"}
```

`result` is `pass`, `fail`, `skipped` (task 27's canary rule when phase 3 task 17 is absent) or
`not-required` (G3 only, above). `reason` is ≤ 200 characters of our own scripts' words, never a
tool's output.

**G2, `gate-channel.sh`.** In the new sandbox image, as `dev` (1001): the worker managed file
(task 13's `policy.Render(policy.Layers{}, model.ClassWorker)` bytes, the same file task 15
mounts), phase 6 task 16's fake upstream scripted to answer the first request with one `Edit`
`tool_use` on `src/a.ts` and the second with `end_turn`, and a fake control socket that records
each request and denies `check --hook` for `src/a.ts`. `claude -p "edit src/a.ts"` runs with a
workspace holding `src/a.ts` and `PANACEA_APP_CONTROL_SOCKET` set. Pass iff: the socket saw
`check --hook` for `src/a.ts`; the fake upstream's second request arrived **after** it; `src/a.ts`
is byte-identical afterwards (the hook's deny held); `panacea team` over the same socket returned
task 09's fixture. The judgement reads the socket's record, the upstream's request log and the
file, never the CLI's output, and the exit status only as "it ended".

**G3, `infra/smoke/bump/g9-job.yaml`.** A Job in `panacea-sandboxes` on the pushed digest, in Kata
(`runtimeClassName` from values), running task 42's `TestG9Live` (`task-42.md:421`) with two
per-run gateway tokens minted for it (S6.3) and `PANACEA_APP_G9_REPORT` on an `emptyDir`;
`gate-live.sh` reads the Job's exit and the report's `pass`. The run's tokens are budgeted like
any session's (OD-24), so a failing version retried every six hours costs at most four G9 runs a
day; the cheap gates run first and a failure there stops the run before G3.

**G4, `infra/smoke/bump/gateway-job.yaml`.** A Job on the pushed digest with one per-run gateway
token, running `claude -p "Reply with the single word ready."` through the private gateway path
(task 32). Pass iff the Job exits 0 **and** the gatekeeper's body-free log (phase 6 task 12) for
that key hash shows at least one `/v1/messages` upstream `200` and **zero** refusals (path not
allowed, header refused, model not in the catalogue). The docs warn that *"a gateway that doesn't
forward them breaks the corresponding features"* (`05-design.md:1106-1108`); zero refusals is how
that shows up on our side.

**Files, two more** (omitted from the list above): `pipelines/scripts/lib/gate.sh`, the one writer
and reader of gate files, with `lib/gate_test.sh` (new); `images/mirror` joins `build.sh` (c).

**`pipelines/scripts/lib/gate.sh`, complete.** No `jq`, as phase 8's scripts have none
(`parts/task-30.md:489`). The reader is sound only because this is the only writer, and it
refuses any value that could break the fixed shape.

```bash
#!/usr/bin/env bash
# pipelines/scripts/lib/gate.sh — sourced. The ONLY writer and reader of images/.out/gates/*.json.
#   gate_write <dir> <gate> <image> <claudeCode> <result> <reason>     exit 2 on a bad value
#   gate_field <file> <key>                                            prints the value, or nothing
readonly GATE_NAMES="canary channel g9 gateway"
readonly GATE_IMAGE_RE='^[a-z0-9.-]+(:[0-9]+)?/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
readonly GATE_VERSION_RE='^[0-9]+\.[0-9]+\.[0-9]+$'

gate_write() {
  local dir="$1" gate="$2" image="$3" cc="$4" result="$5" reason="$6"
  case " $GATE_NAMES " in *" $gate "*) ;; *) echo "gate: unknown gate $gate"; return 2 ;; esac
  [[ "$image" =~ $GATE_IMAGE_RE ]] || { echo "gate: $gate: image is not a digest: $image"; return 2; }
  [[ "$cc" =~ $GATE_VERSION_RE ]] || { echo "gate: $gate: claudeCode is not MAJOR.MINOR.PATCH"; return 2; }
  case "$result" in pass|fail|skipped) ;;
    not-required) [ "$gate" = g9 ] || { echo "gate: not-required is g9's only"; return 2; } ;;
    *) echo "gate: $gate: result $result"; return 2 ;; esac
  [[ "$reason" =~ ^[A-Za-z0-9\ .,:/_()=-]{0,200}$ ]] || { echo "gate: $gate: reason has a character the reader cannot carry"; return 2; }
  mkdir -p "$dir"
  printf '{"gate":"%s","image":"%s","claudeCode":"%s","result":"%s","reason":"%s","at":"%s","run":"%s"}\n' \
    "$gate" "$image" "$cc" "$result" "$reason" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${BUILD_BUILDNUMBER:-local}" \
    > "$dir/$gate.json"
}

gate_field() {  # every value is from gate_write's alphabet, so no quote or backslash can occur
  sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p" "$1" 2>/dev/null | head -n 1
}
```

**`images/build.sh`, the changed lines** (task 27's header, `parts/task-27.md:37-47`, steps 2, 3a, 5):

```bash
#   2. docker build images/sandbox --build-arg CLAUDE_CODE_VERSION="$(cat images/sandbox/claude-code.version)"
#      (refused first unless the file is one MAJOR.MINOR.PATCH line)
#   3a. docker build images/mirror (task 58's Dockerfile, its own pinned base)
#   5. with --push: docker push each, then write images/.out/digests.json, ONE line, this key order:
#        {"sandbox":"…@sha256:…","machine":…,"broker":…,"hub":…,"mirror":…,"claudeCode":"<label>"}
#      <label> is docker image inspect's .Config.Labels.PANACEA_APP_CLAUDE_VERSION of the sandbox
#      image built in step 2: the build's own record, never the CLI's output.
```

`digests.json` stays task 27's file; `gate_field` reads it too, which is why it is one line with
no spaces inside a value.

**`pipelines/scripts/publish-digests.sh`, complete.** The only step that writes the artefact.

```bash
#!/usr/bin/env bash
# pipelines/scripts/publish-digests.sh <digests.json> <gates dir> <last published claudeCode, or "">
#   Copies <digests.json> to images/.out/publish/digests.json (the artefact image-digests) iff every
#   gate judged THIS sandbox digest and THIS CLI version and passed. Otherwise it writes nothing.
#   "" as the last version (no earlier artefact on the branch) counts as a CLI change.
# Output: publish: <sandbox digest> claudeCode <v> (change|same)                     exit 0
#         publish: refused: <why>                                                    exit 2
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/gate.sh
. "$here/lib/gate.sh"
digests="$1" gates="$2" last="${3:-}"
refuse() { echo "publish: refused: $*"; exit 2; }

[ -s "$digests" ] || refuse "$digests is missing or empty"
for k in sandbox machine broker hub mirror; do
  v="$(gate_field "$digests" "$k")"
  [[ "$v" =~ $GATE_IMAGE_RE ]] || refuse "$k is not a digest: ${v:-<absent>}"
done
sandbox="$(gate_field "$digests" sandbox)"
cc="$(gate_field "$digests" claudeCode)"
[[ "$cc" =~ $GATE_VERSION_RE ]] || refuse "claudeCode is not MAJOR.MINOR.PATCH: ${cc:-<absent>}"
if [ "$cc" = "$last" ]; then change=same; else change=change; fi

for g in $GATE_NAMES; do
  f="$gates/$g.json"
  [ -s "$f" ] || refuse "gate $g wrote nothing"
  [ "$(gate_field "$f" image)" = "$sandbox" ] || refuse "gate $g judged another image"
  [ "$(gate_field "$f" claudeCode)" = "$cc" ] || refuse "gate $g judged another CLI version"
  r="$(gate_field "$f" result)"
  case "$g:$r:$change" in
    *:pass:*) ;;
    canary:skipped:same) ;;       # task 27's rule stands only when the CLI did not move
    g9:not-required:same) ;;
    *) refuse "gate $g is $r on a $change of CLI: $(gate_field "$f" reason)" ;;
  esac
done
mkdir -p images/.out/publish
cp "$digests" images/.out/publish/digests.json
echo "publish: $sandbox claudeCode $cc ($change)"
```

`last` comes from the previous `image-digests` of the same branch (the `publish` job below). A
first publish on a branch has none, so it needs all four gates to `pass`: until phase 3 task 17's
canary exists, **no image publishes on a fresh branch** (gap 3).

**(f) The live gates mint their own per-run tokens, as a keysmith caller of their own.** A gateway
binding is pinned to the pod's address (`wire.MintRequest.PodAddr`, phase 6 `parts/task-03.md:30`),
so the token must be minted after the Job's pod has one. Each Job's init container runs
`gatemint`, a new ~50-line `control-plane/cmd/gatemint` on phase 6 task 15's
`gatewayclient.New(keysmithURL, callerTokenFile, o)` and `Mint` (`parts/task-15.md:763,792`),
shipped in the **broker** image (`build.sh` step 1 gains it), never in the sandbox image. It runs as
service account `bump-gate` in `panacea-sandboxes`, a row in keysmith's caller table
(`name credential routes`, phase 6 `parts/task-08.md:45`) with routes `mint` only. It mints for
`status.podIP` with session id `gate-<run>-<gate>-<n>`, the owner, team and project in
`bumpGate.*` values (a dev-only "platform gate" project whose budget is the G9 cost cap), and
writes the token to an `emptyDir` with `gatewayclient.WriteToken`. Tokens die at their TTL and
with the pod; nothing else holds them. Gap 4 is the keysmith row and the gate project.

**`pipelines/images.yml`, the three new jobs** (appended to task 27's `jobs:`; `canary`, `push`
and the rest unchanged but for `push` no longer publishing `image-digests`):

```yaml
  - job: channel                      # G2; no connection, so PR builds run it
    dependsOn: build
    pool: { name: $(PANACEA_APP_CI_POOL) }
    steps:
      - download: current
        artifact: images-local        # task 27's build output (name UNVERIFIED until merge)
      - bash: pipelines/scripts/gate-channel.sh images/.out/gates
        displayName: G2 the panacea channel on the new image

  - job: gates_live                   # G3, G4; dev's cluster even on main (never prod)
    dependsOn: [push, canary, channel]
    condition: and(succeeded(), in(variables['Build.SourceBranch'], 'refs/heads/dev', 'refs/heads/main'))
    pool: { name: $(PANACEA_APP_CI_POOL) }
    steps:
      - download: current
        artifact: pushed-digests      # push's digests.json, kept in the run (renamed by this task)
      - download: current
        artifact: gates               # canary's and channel's result files
      - task: DownloadPipelineArtifact@2
        inputs: { buildType: specific, project: $(System.TeamProjectId), definition: $(System.DefinitionId),
                  buildVersionToDownload: latestFromBranch, branchName: $(Build.SourceBranch),
                  artifactName: image-digests, targetPath: $(Agent.TempDirectory)/last }
        continueOnError: true         # none yet on this branch: publish treats it as a CLI change
      - task: AzureCLI@2
        inputs:
          azureSubscription: $(PANACEA_APP_DEV_GATE_SERVICE_CONNECTION)   # gap 1: a fifth identity
          scriptType: bash
          scriptLocation: scriptPath
          scriptPath: pipelines/scripts/gate-live.sh
          arguments: $(Pipeline.Workspace)/pushed-digests/digests.json images/.out/gates $(Agent.TempDirectory)/last

  - job: publish                      # the ONLY writer of image-digests
    dependsOn: gates_live
    pool: { name: $(PANACEA_APP_CI_POOL) }
    steps:
      - download: current             # pushed-digests and gates, as above
      - bash: >
          pipelines/scripts/publish-digests.sh $(Pipeline.Workspace)/pushed-digests/digests.json
          $(Pipeline.Workspace)/gates-final "$(cat $(Pipeline.Workspace)/gates-final/last-claudeCode)"
      - publish: images/.out/publish
        artifact: image-digests
```

`gate-live.sh` publishes the four result files as `gates-final`, with `last-claudeCode` beside
them (the `claudeCode` of the downloaded `last/digests.json`, empty when there is none or it is
task 27's older shape without the key); `publish` needs no connection. The YAML shape follows
Azure Pipelines' documented schema from training knowledge (**UNVERIFIED** until the first run,
as task 27's own is).

**`control-plane/cmd/gatemint/main.go`, complete.** Two verbs. `policy` prints the worker managed
file, so G2 on the CI agent and G3/G4 in the cluster judge the same bytes task 15 mounts. `mint`
is decision (f). It never prints a token.

```go
// Command gatemint serves the CLI bump job's gates (task 65), nothing else.
//
//	gatemint policy                    the worker managed file on stdout
//	gatemint mint <session-id> <dir>   mints for $POD_IP, writes <dir>/token; exit 1 on refusal
package main

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatewayclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/policy"
)

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr)) }

func run(args []string, getenv func(string) string, out, errw *os.File) int {
	switch {
	case len(args) == 1 && args[0] == "policy":
		file, _, err := policy.Render(policy.Layers{}, model.ClassWorker)
		if err != nil {
			fmt.Fprintln(errw, "gatemint: render:", err)
			return 1
		}
		_, _ = out.Write(file)
		return 0
	case len(args) == 3 && args[0] == "mint":
		if _, err := netip.ParseAddr(getenv("POD_IP")); err != nil {
			fmt.Fprintln(errw, "gatemint: POD_IP is not an address")
			return 1
		}
		c, err := gatewayclient.New(getenv("PANACEA_APP_GATEMINT_KEYSMITH_URL"),
			getenv("PANACEA_APP_GATEMINT_CALLER_TOKEN_FILE"), gatewayclient.Options{})
		if err != nil {
			fmt.Fprintln(errw, "gatemint:", err)
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := c.Mint(ctx, wire.MintRequest{
			SessionID: args[1], Generation: 1,
			Owner:     wire.Owner{TID: getenv("PANACEA_APP_GATEMINT_OWNER_TID"), OID: getenv("PANACEA_APP_GATEMINT_OWNER_OID")},
			TeamID:    getenv("PANACEA_APP_GATEMINT_TEAM_ID"), ProjectID: getenv("PANACEA_APP_GATEMINT_PROJECT_ID"),
			AgentID:   args[1], SessionClass: wire.ClassWorker, PodAddr: getenv("POD_IP"),
		})
		if err != nil {
			fmt.Fprintln(errw, "gatemint: mint refused:", err) // gatewayclient's errors carry no token
			return 1
		}
		if err := gatewayclient.WriteToken(args[2], res.Token); err != nil {
			fmt.Fprintln(errw, "gatemint:", err)
			return 1
		}
		fmt.Fprintln(out, "gatemint: minted key", res.KeyID) // the key id, never the token
		return 0
	}
	fmt.Fprintln(errw, "usage: gatemint policy | gatemint mint <session-id> <dir>")
	return 2
}
```

`PodAddr` is a string in `MintRequest` with an `Addr()` accessor (`parts/task-03.md:30`), so the
pre-check only gives a clearer error than keysmith's `400`.

**`infra/smoke/bump/gateway-job.yaml`, complete (G4).** `gate-live.sh` fills the `${…}` with
`envsubst` from the pushed digests and the dev values; keysmith and gatekeeper URLs are task 32's
(`task-32.md:168-169`). keysmith's caller row is `bump-gate system:serviceaccount:panacea-sandboxes:bump-gate mint`
(kube mode, audience `keysmith`, phase 6 `parts/task-08.md:45,59`).

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gate-gateway-${RUN}
  namespace: panacea-sandboxes
  labels: { app.kubernetes.io/part-of: panacea-app, panacea.app/gate: gateway }
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 600
  ttlSecondsAfterFinished: 900
  template:
    metadata:
      labels: { panacea.app/gate: gateway, panacea.app/class: worker }  # the worker egress policy applies
    spec:
      runtimeClassName: ${RUNTIME_CLASS}
      serviceAccountName: bump-gate
      automountServiceAccountToken: false
      restartPolicy: Never
      securityContext: { runAsUser: 1001, runAsGroup: 1001, runAsNonRoot: true, fsGroup: 1001 }
      volumes:
        - { name: gateway, emptyDir: { medium: Memory, sizeLimit: 1Mi } }
        - { name: managed, emptyDir: { medium: Memory, sizeLimit: 1Mi } }
        - name: keysmith-sa
          projected: { sources: [ { serviceAccountToken: { audience: keysmith, expirationSeconds: 600, path: token } } ] }
      initContainers:
        - name: policy
          image: ${BROKER_IMAGE}
          command: [/bin/sh, -c, "/gatemint policy > /etc/claude-code/managed-settings.json"]
          volumeMounts: [ { name: managed, mountPath: /etc/claude-code } ]
        - name: mint
          image: ${BROKER_IMAGE}
          command: [/gatemint, mint, "gate-${RUN}-gateway-1", /run/panacea/gateway]
          env:
            - { name: POD_IP, valueFrom: { fieldRef: { fieldPath: status.podIP } } }
            - { name: PANACEA_APP_GATEMINT_KEYSMITH_URL, value: "http://keysmith.panacea-gateway.svc.cluster.local:4100" }
            - { name: PANACEA_APP_GATEMINT_CALLER_TOKEN_FILE, value: /var/run/keysmith/token }
            - { name: PANACEA_APP_GATEMINT_OWNER_TID, value: "${GATE_OWNER_TID}" }
            - { name: PANACEA_APP_GATEMINT_OWNER_OID, value: "${GATE_OWNER_OID}" }
            - { name: PANACEA_APP_GATEMINT_TEAM_ID, value: "${GATE_TEAM_ID}" }
            - { name: PANACEA_APP_GATEMINT_PROJECT_ID, value: "${GATE_PROJECT_ID}" }
          volumeMounts:
            - { name: gateway, mountPath: /run/panacea/gateway }
            - { name: keysmith-sa, mountPath: /var/run/keysmith, readOnly: true }
      containers:
        - name: cli
          image: ${SANDBOX_IMAGE}
          command: [claude, -p, "Reply with the single word ready."]
          env:
            - { name: ANTHROPIC_BASE_URL, value: "http://gatekeeper.panacea-gateway.svc.cluster.local:4000" }
          volumeMounts:
            - { name: gateway, mountPath: /run/panacea/gateway, readOnly: true }
            - { name: managed, mountPath: /etc/claude-code, readOnly: true }
          resources: { limits: { cpu: "1", memory: 2Gi } }
```

The broker image's shell (`/bin/sh`) is **UNVERIFIED**: if task 29's broker image is distroless,
`gatemint policy` gains an `--out <path>` flag instead. The keysmith token is mounted into the
init container only; the CLI's container sees the gateway token file, as a real sandbox does.

**`infra/smoke/bump/g9-job.yaml`** is the same Job but for: `name: gate-g9-${RUN}`, two `mint`
init containers (`gate-${RUN}-g9-ada`, `gate-${RUN}-g9-linus`, each into its own `emptyDir`),
`activeDeadlineSeconds: 3600`, and a `cli` container that runs task 42's test binary
(`g9.test -test.run '^TestG9Live$'`, built `linux && liveeval` into the sandbox image's
`/opt/panacea/g9/`, gap 5) with `PANACEA_APP_G9_REPORT=/run/g9/report.json` on an `emptyDir`.
The downward API cannot read an image label, so `PANACEA_APP_CLAUDE_VERSION` is set to
`${CLAUDE_CODE}`, which `gate-live.sh` takes from `digests.json`. A finished pod cannot be copied
from, so the report leaves the pod as one log line the container's wrapper prints after the test,
`G9-REPORT <base64 of report.json>`: our own format, read by `gate-live.sh`, never the CLI's.

**`pipelines/scripts/gate-live.sh`, the contract** (header, complete; the body is these steps):

```bash
#!/usr/bin/env bash
# pipelines/scripts/gate-live.sh <pushed digests.json> <gates dir> <last image-digests dir>
#   Runs G4 then G3 on DEV's cluster (never prod's, even for main), writes gateway.json and
#   g9.json with lib/gate.sh, and uploads <gates dir> + last-claudeCode as artefact gates-final.
#   1. az aks get-credentials for dev (no --admin, P8-D2); kubelogin convert-kubeconfig -l azurecli
#   2. last = gate_field <last dir>/digests.json claudeCode   ("" when absent or task 27's shape)
#   3. G4: envsubst < infra/smoke/bump/gateway-job.yaml | kubectl apply -f -; wait <= 10 min for
#      Complete or Failed. pass iff Complete AND, over every gatekeeper replica's log since the
#      Job's start (kubectl logs -n panacea-gateway -l app.kubernetes.io/name=gatekeeper), the
#      lines whose "session" is gate-<run>-gateway-1 hold >= 1 {"path":"/v1/messages","status":200}
#      and none with a 4xx status or "path":"(other)". reason names the first offending line's
#      path and status, nothing else.
#   4. G3: if claudeCode == last, gate_write g9 not-required. Else if G4 failed, gate_write g9
#      skipped "G4 failed first". Else run g9-job.yaml the same way; pass iff Complete AND the
#      decoded G9-REPORT line's "pass" is true.
#   5. trap: kubectl delete job -l panacea.app/gate --wait=false for this run, always.
# Exit 0 whatever the gates say (the files say it; publish refuses). Exit 2 on a setup error:
# no cluster, no digests, a Job that never started. A setup error fails the run, and nothing publishes.
set -euo pipefail
```

The log-line match is on the gatekeeper's own JSON keys, `level,ms,msg,path,project,route,session,
status,time` (phase 6 `parts/task-12.md:208`, the `request` line at `:874`), which task 12 pins
by test. The gate identity must be able to read those logs (gap 1).

**Fleet, the launch version and the mirror image** (`cmd/fleetd/config.go`, task 12's block at
`task-12.md:95-130`, two constants, two fields, two checks):

```go
	envClaudeVersion = "PANACEA_APP_FLEET_CLAUDE_VERSION" // required: MAJOR.MINOR.PATCH, image-digests' claudeCode
	envMirrorImage   = "PANACEA_APP_FLEET_MIRROR_IMAGE"   // required: <repo>@sha256:<64 hex>, image-digests' mirror

// in Config:
	ClaudeCodeVersion string
	MirrorImage       string

// in loadConfig, after the existing checks:
	c.ClaudeCodeVersion, c.MirrorImage = getenv(envClaudeVersion), getenv(envMirrorImage)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(c.ClaudeCodeVersion) {
		return Config{}, fmt.Errorf("%s: %q is not MAJOR.MINOR.PATCH", envClaudeVersion, c.ClaudeCodeVersion)
	}
	if !regexp.MustCompile(`^[a-z0-9.\-]+(:[0-9]+)?/[a-z0-9._/\-]+@sha256:[a-f0-9]{64}$`).MatchString(c.MirrorImage) {
		return Config{}, fmt.Errorf("%s: not an image by digest", envMirrorImage)
	}
```

`internal/fleet/sessions/sessions.go` (task 20): `Config` gains `ClaudeCodeVersion string`, set
from the above by `fleetd`'s wiring; `Launch` gains one line before the agent record is built
(`task-20.md:391`), so a caller cannot choose the version a record shows:

```go
	if r.Harness == "claude-code" {
		r.Version = s.cfg.ClaudeCodeVersion // the digest this pod starts from; never the caller's
	}
```

Task 59's `EnsureMirror` caller sends `cfg.MirrorImage` in task 58's `PUT` body (gap 6).

**The chart** (task 32's `deploy/charts/fleet`): `values.yaml` gains `images: { claudeCode: "",
mirror: "" }`; `templates/deployment.yaml` sets the two variables from them; the schema requires
`claudeCode` to match the version pattern and `mirror` task 12's digest pattern. Task 30's
`helm-values.sh` sets both from the same `image-digests` it already reads for `warm.images`
(`--set fleet.images.claudeCode=… --set fleet.images.mirror=…`), so Fleet's launch version and
the warm pool's image come from one artefact, in one deploy.

- [ ] **Step 1: Write the tests, each with the mutant it kills**

`pipelines/scripts/cli-bump_test.sh` and `lib/gate_test.sh` follow `secrets-pr_test.sh`'s shape
(phase 8 task 28): each case builds a temp directory of fixtures, runs the script, checks exit
status, output and files. `npm` and `curl` are fakes on `PATH` that record their arguments.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `publish_refuses_each_failed_gate` | four runs, one gate `fail` each: exit 2, `publish: refused: gate <g> is fail`, no `images/.out/publish/digests.json` | a publisher that checks only the canary; one that needs any gate to pass |
| `publish_refuses_a_missing_gate` | `g9.json` absent → `gate g9 wrote nothing` | a loop over the files present instead of `GATE_NAMES` |
| `publish_refuses_a_gate_on_another_digest` | `gateway.json`'s image differs in its last hex digit → refused | a stale gate file from an earlier run accepted |
| `publish_refuses_a_gate_on_another_version` | `channel.json` says `2.1.279`, digests `2.1.280` → refused | the version check dropped |
| `publish_refuses_a_tag` | each of `sandbox`, `machine`, `broker`, `hub`, `mirror` in turn as `…:2.1.280`, then as `…@sha256:<64 hex>x` → refused naming the key | only `sandbox` checked; the pattern without its `$` anchor |
| `skipped_canary_blocks_a_cli_change` | canary `skipped`, last `2.1.274`, now `2.1.280` → refused; last `2.1.280` → published, `(same)` | task 27's skip rule applied to a CLI change |
| `not_required_g9_only_without_a_change` | g9 `not-required` with last `2.1.274` → refused | G3 skipped for the very change it exists for |
| `empty_last_is_a_change` | last `""`, canary `skipped` → refused | a missing earlier artefact read as "same" |
| `gate_reason_cannot_forge_a_pass` | `gate_write … fail 'x","result":"pass'` → exit 2, no file | a looser reason alphabet, which lets `gate_field` read `pass` |
| `gate_write_refuses_bad_values` | a tag image, `2.1`, result `ok`, `not-required` for `channel` → exit 2 each | any one check removed |
| `latest_compares_numerically` | pin `2.1.99`, registry `2.1.100` → prints `2.1.100`; pin `2.1.274`, registry `2.1.274`, `2.1.270`, `2.2.0-beta.1` → prints nothing, exit 0 | a string comparison; `>=`; a pre-release accepted |
| `pr_nothing_on_empty` | `cli-pr.sh ""` → the fake `curl` saw no request | a PR opened with no version |
| `pr_existing_is_left_alone` | the fake API lists an active PR from `bump/claude-code-2.1.280` → no `POST` | a duplicate PR every six hours |
| `pr_is_one_line_to_dev` | the push changes only `images/sandbox/claude-code.version`, to `2.1.280\n`; target `refs/heads/dev`; `autoCompleteSetBy` set; no `force` | a PR to `main`; a second file touched |
| `pr_abandons_the_older_bump` | an active `bump/claude-code-2.1.277` PR and new `2.1.280` → that PR `abandoned`, one comment naming `2.1.280` | an older bump completing after a newer one (a downgrade) |

`images/images_test.sh` (task 27's suite) gains three cases:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `sandbox_build_needs_the_version` | `docker build images/sandbox` with no `--build-arg` exits non-zero | `ARG CLAUDE_CODE_VERSION=latest` |
| `label_is_the_version_file` | the built image's `PANACEA_APP_CLAUDE_VERSION` label equals `claude-code.version` | a hard-coded label |
| `digests_json_has_mirror_and_claudeCode` | `build.sh --push` to a local registry writes one line with the six keys in order; `gate_field` reads each | the mirror left out of the run (c) |

`pipelines/scripts/gate-channel_test.sh` runs G2 twice more on the new image, with a test-only
`GATE_CHANNEL_SOCKET_MODE=allow` (the fake socket allows the edit, so `src/a.ts` changes) must
write `fail`; `GATE_CHANNEL_POLICY=testdata/no-hook.json` (a managed file without the
`PreToolUse` hook, so the socket never sees `check --hook`) must write `fail`. Both variables are
refused unless `GATE_CHANNEL_TEST=1`, which `images.yml` never sets. Mutant: a G2 that judges by
the CLI's exit status alone.

Go tests, on task 12's and task 20's fakes and phase 6 task 15's fake keysmith:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `fleetd` · `TestClaudeVersionRequired` | absent, `latest`, `2.1` → an error beginning `PANACEA_APP_FLEET_CLAUDE_VERSION` | a default of `""` or `latest` |
| `fleetd` · `TestMirrorImageMustBeADigest` | `…/mirror:1` → an error beginning `PANACEA_APP_FLEET_MIRROR_IMAGE` | a tag accepted |
| `sessions` · `TestLaunchVersionIsFleetsNotTheCallers` | `LaunchRequest.Version = "9.9.9"`, config `2.1.280` → the stored agent and the Hub outbox item carry `2.1.280` | the caller's value kept |
| `sessions` · `TestRunningAgentKeepsItsVersion` | launch under `2.1.274`; a new `Sessions` on the same store with `2.1.280` (a deploy); the first record still `2.1.274`, its sandbox spec unchanged, no broker call for it; a Restart makes a new record at `2.1.280` | a reconcile that rewrites versions; a Restart that reuses the record |
| `gatemint` · `TestMintNeverPrintsTheToken` | fake keysmith answers token `tok-7f…`: neither stdout nor stderr holds it; `<dir>/token` does; stdout names the key id | printing the response |
| `gatemint` · `TestMintSendsThePodAddress` | `POD_IP=10.4.0.9` → the request's `podAddr` is it; `POD_IP` empty → exit 1, no request | minting for no address, or for a fixed one |
| `gatemint` · `TestPolicyIsTheWorkerFile` | stdout equals `policy.Render(policy.Layers{}, model.ClassWorker)`'s bytes | the orchestrator class; a stale embedded copy |

The chart gains `tests/fleet_images_test.yaml` (helm-unittest, as task 32's): `claudeCode:
latest` and a tagged `mirror` each fail the schema; good values set both variables. Mutant: the
schema without the patterns.

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && bash pipelines/scripts/cli-bump_test.sh; bash pipelines/scripts/lib/gate_test.sh
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./cmd/fleetd/ ./cmd/gatemint/ ./internal/fleet/sessions/ -run "ClaudeVersion|MirrorImage|Version|Mint|Policy"
```

Expected: the shell suites fail with `No such file or directory` for the scripts; Go fails to
compile on `undefined: envClaudeVersion`, `no Go files in …/cmd/gatemint`, and
`s.cfg.ClaudeCodeVersion undefined`.
