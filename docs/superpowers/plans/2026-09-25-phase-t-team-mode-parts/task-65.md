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
      name: $(PANACEA_APP_POOL)  # task 30's Managed DevOps Pool; the variable name is UNVERIFIED
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
