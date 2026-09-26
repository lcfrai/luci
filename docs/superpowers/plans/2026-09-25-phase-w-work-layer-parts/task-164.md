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
