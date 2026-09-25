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
the CLI image. Task 58's mirror image is built `FROM` the sandbox base in the same `build.sh`
run, as the machine image is (phase 8 `parts/task-27.md:21`), so one bump moves all three.

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
