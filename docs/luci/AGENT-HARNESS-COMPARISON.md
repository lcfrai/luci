# Agent harness comparison and implementation choices

Luci's objective is a dependable local application for coordinating agents from
an understood idea to verified results, with a polished Windows experience first.
Managed hosting remains a subsequent product goal. “Best” is a target to assess
through explicit capabilities and measured work, not an established market claim.
The full completion standard remains in [GOAL.md](GOAL.md) and
[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md).

## Sources and licensing

This comparison records primary-source documentation and selected implementation
code inspected on 8 October 2026. Upstream workflows were not executed. A
documented feature is not evidence that it passed Luci's acceptance scenarios.

| Project | Inspected source | Repository license |
| --- | --- | --- |
| GSD Core | [open-gsd/gsd-core at `83aba446`](https://github.com/open-gsd/gsd-core/tree/83aba44627ba43aff77588894f7b81a9a7437e75), default branch `next` | [MIT, © 2026 Open GSD](https://github.com/open-gsd/gsd-core/blob/83aba44627ba43aff77588894f7b81a9a7437e75/LICENSE) |
| Get Shit Done | [gsd-build/get-shit-done at `bdcaab2c`](https://github.com/gsd-build/get-shit-done/tree/bdcaab2c752d9a33a1a1ca9acf3a3c81fb991815) | [MIT, © 2025 Lex Christopherson](https://github.com/gsd-build/get-shit-done/blob/bdcaab2c752d9a33a1a1ca9acf3a3c81fb991815/LICENSE) |
| GSD 2 | [gsd-build/gsd-2 at `33c00aaf`](https://github.com/gsd-build/gsd-2/tree/33c00aaffa56e5d394bccce1c8df59fb842e84c5) | [MIT, © 2026 Lex Christopherson](https://github.com/gsd-build/gsd-2/blob/33c00aaffa56e5d394bccce1c8df59fb842e84c5/LICENSE) |
| Nimbalyst | [nimbalyst/nimbalyst at `6f14adff`](https://github.com/nimbalyst/nimbalyst/tree/6f14adff484d5f0b6feb1033c190277532ec5350), v0.80.4 | [MIT, © 2024–2026 Nimbalyst Inc.](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/LICENSE) |

The selected integration approach uses original Luci implementation in its
existing Go and TypeScript architecture. No upstream code, prompts, assets,
installer or runtime dependency is imported by this comparison. MIT permits
commercial reuse, but copied substantial portions would require retaining the
copyright and permission notice. A repository license does not establish the
licenses of every bundled dependency.

Nimbalyst's [licensing statement](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/LICENSING.md)
identifies its collaboration sync server as a separate project; the client
license does not establish a reusable hosted sync service. Its predecessor
[Crystal at `1e18e0bc`](https://github.com/stravu/crystal/tree/1e18e0bc981225f75b5226f82a300fa741970c6f)
explicitly says it was deprecated in February 2026 and replaced by Nimbalyst.
The [Nimbalyst skills collection at `8822f1ef`](https://github.com/nimbalyst/skills/tree/8822f1ef9a9ea2382c158a0e7a26423046b641d6)
is an MIT workflow/prompt library, not an execution scheduler.

## Capability comparison

The Luci baseline here is application source `deeb55ad`, documented at
`a91d2c5e`, before the task-graph work prompted by this review. Later changes must
be assessed against their own implementation and validation receipts.

| Concern | Applicable upstream mechanism | Luci baseline and implication |
| --- | --- | --- |
| Persistent shared understanding | GSD Core's phase artifacts and per-workstream state keep decisions outside conversation memory. | Luci already has versioned brief, definition, design and task plan, shared project understanding, reviewed amendments and private attribution. Preserve this single context authority. |
| Task decomposition | GSD executes dependency-ordered plans; GSD 2 reports ready milestones and anticipated file overlap. | Luci's task list has stable IDs, progress and requirement links, but lacks task dependencies and write scope. This is an implementation gap. |
| Parallel execution | GSD 2 documents independent processes, branches, worktrees and contexts, with a shared dispatch ledger. | Luci has manual branch sessions and durable reviewed-run operations, but reviewed runs currently use the main checkout and serialize coding attempts. Independent terminals alone do not provide a coordinated swarm. |
| Context selection | GSD and GSD 2 declare phase/unit-specific artifacts, dependency summaries and context budgets. | Luci captures full reviewed context immutably. A compact task contract can improve navigation and handoffs while retaining all authoritative captured documents. |
| Amendments and drift | GSD Core warns or blocks when research predates new phase decisions. | Luci binds exact content digests, revisions and private approvals. Preserve these checks instead of replacing them with timestamp comparisons. |
| Verification | GSD Core persists pending, blocked and human-needed verification across phase transitions. | Luci already separates manually recorded task progress from source-bound checks and releases. Swarm handoffs must retain unresolved verification and integration state, including after selecting a newer run. |
| Recovery and intervention | GSD 2 documents heartbeat ownership, fenced leases, persisted commands and explicit merge reconciliation. | Luci has durable reservations, idempotent child recovery and explicit reconciliation. Extend these to individual workers without interpreting a missed heartbeat as permission to duplicate work. |
| Manager/worker relationships | Nimbalyst separates conversation parentage, reporting manager and worktree identity; a manager can coordinate roots in different worktrees. | Keep task dependencies, managing run, session conversation and execution worktree as distinct identities. A session tree is not the task graph. |
| Useful completion reports | Nimbalyst deduplicates and bounds child reports, retains pending questions and suppresses completion while more prompts remain. | Surface actionable waiting/error/review states under the agreed task; do not make users infer progress by searching terminal output. |
| Visual review | Nimbalyst offers visual document/file review and session links. | Luci's reviewed context amendments have an explicit before/after and stale-save boundary. Improve clarity without replacing that authority with after-write rollback. |

## Ideas selected for Luci

**Make the task graph understandable before execution.** Extend the existing plan
with stable-ID dependencies and anticipated repository write paths. Validate
unknown dependencies, self-dependencies, cycles and unsafe paths. Show what can
start, what is waiting, and where planned changes overlap. Missing write scope
means unknown overlap, not proven independence. Windows path case, separators
and folder descendants matter. Task readiness describes saved planning state;
it does not prove acceptance criteria passed.

This draws on GSD 2's
[parallel orchestration contract](https://github.com/gsd-build/gsd-2/blob/33c00aaffa56e5d394bccce1c8df59fb842e84c5/docs/user-docs/parallel-orchestration.md)
and
[eligibility implementation](https://github.com/gsd-build/gsd-2/blob/33c00aaffa56e5d394bccce1c8df59fb842e84c5/src/resources/extensions/gsd/parallel-eligibility.ts).
Its file-overlap warnings do not block execution because workers use separate
worktrees; they still anticipate merge conflicts. Its parallel mode is opt-in and
explicitly depends on single-host local SQLite/WAL semantics.

**Give each worker a precise, durable contract.** A selected task needs its own
purpose, scope, dependencies, linked acceptance criteria and known unresolved
questions. Give workers a compact view and references to complete frozen context.
Preserve the complete captured documents and identify the exact revision/source
base. Do not silently truncate requirements or let a summary override the
reviewed originals. A successful agent exit remains distinct from completion,
verification and integration.

The relevant GSD 2 mechanisms are its
[unit context manifest](https://github.com/gsd-build/gsd-2/blob/33c00aaffa56e5d394bccce1c8df59fb842e84c5/src/resources/extensions/gsd/unit-context-manifest.ts)
and
[composer](https://github.com/gsd-build/gsd-2/blob/33c00aaffa56e5d394bccce1c8df59fb842e84c5/src/resources/extensions/gsd/unit-context-composer.ts),
which distinguish inline, excerpt, computed and on-demand artifacts. The earlier
GSD
[context engine](https://github.com/gsd-build/get-shit-done/blob/bdcaab2c752d9a33a1a1ca9acf3a3c81fb991815/sdk/src/context-engine.ts)
selects artifacts by phase and narrows the roadmap to the active milestone.
Luci should measure supplied bytes and label token/window estimates explicitly;
it must not assume that every selected provider has a 200,000-token window.

**Integrate isolated changes with evidence.** Independent ready tasks should have
independent worktrees and owned sessions. Dependent integration must wait for
reviewed prerequisite results. Show the exact worker changes and base/current
heads, preserve conflicts for intervention, and run checks on the integrated
source. Cancellation, a crash, an uncertain dispatch and a changed plan must
remain distinguishable. No missed heartbeat or disappeared process may silently
authorize a second worker with the same assignment.

Nimbalyst's
[session hierarchy](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/docs/SESSION_HIERARCHY.md)
and
[manager reservation implementation](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/packages/electron/src/main/services/metaAgentSessionQueries.ts)
show why ownership and capacity belong in execution admission. The inspected
implementation caps a manager at four in-flight children, counting pending and
queued work, with a fifty-child lifetime backstop. Its immediate reservations are
memory-local while durable prompts/session rows retain launch accounting. Luci
should extend its existing durable private operation receipts rather than copy
memory-only reservation as crash authority.

**Keep the orchestrator focused on actionable updates.** Nimbalyst's
[manager service](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/packages/electron/src/main/services/MetaAgentService.ts)
supersedes older pending updates from the same child, includes edited files and
exact waiting questions, and avoids automatically waking the manager on errors
into a failure loop. Luci should use bounded provenance-linked handoffs with full
results available explicitly. Updates must preserve unresolved questions and
verification gaps; a compressed summary cannot silently supersede accepted
evidence.

**Keep unfinished verification visible.** GSD Core's
[verification-debt tracking](https://github.com/open-gsd/gsd-core/blob/83aba44627ba43aff77588894f7b81a9a7437e75/docs/features/verification-debt-tracking.md)
distinguishes partial verification, external blockers and human-needed review.
Luci should carry these facts through task handoffs and the project overview,
using its existing independent check evidence instead of a second competing
status ledger.

## Implementation follow-through — 8 October 2026

The later local candidate implements these mechanisms in Luci's existing
project/run/task records; it does not install an upstream orchestration runtime.
The [worklist](QUALITY-WORKLIST.md) binds changing verification status and the
[Windows matrix](LOCAL-WINDOWS-ACCEPTANCE.md) defines the remaining native checks.

| Selected mechanism | Implemented Luci behavior | Remaining evidence |
| --- | --- | --- |
| GSD task graph and scoped context | Explicit dependencies and write scope, isolated task execution, V2 frozen direct-dependency packets and separately attributed prior-attempt context. Complete reviewed originals remain available. | Installed join/continuation runs at the matching candidate, live-model output and context-quality comparison. |
| GSD Core unfinished verification | Older review, verification, release-review and blocked runs stay in bounded Home/orchestrator attention after a newer run is created. Attention is separate from active capacity. | Matching installed older-run workflow and native review. |
| Nimbalyst bounded reports and questions | Immutable task reports preserve decisions, limitations and questions. Attributed human answers retain the original question/impact; worker check claims stay separate from recorded checks. Forget preserves ownership/context until an explicit report status is captured. Earlier-attempt questions remain discoverable. | Real dependent-worker use and blinded semantic evaluation; fixture success does not establish usefulness. |
| Explicit recovery and visual review | Complete supported text changes can seed a distinct reserved continuation only after review. Starting and integrating remain separate actions. Conflicts show their returned unresolved state. | Installed lost-reply/restart/seed preservation and actual Git conflict, then Windows workflow tests. |

The [context-quality corpus and protocol](../../scripts/acceptance/context-quality/README.md)
provide six ambiguity/isolation/amendment/stale-source/check-claim/restart cases,
separate evaluator truth and explicit unscored human-review fields. Controlled
scorer checks pass; neither upstream tools nor live Luci model trials have been
run under that protocol. There is no measured market-leadership claim.

## Approaches not adopted

- A second writable `.planning` or `.gsd` specification would split Luci's
  authority. GSD Core's
  [workstream scoping](https://github.com/open-gsd/gsd-core/blob/83aba44627ba43aff77588894f7b81a9a7437e75/docs/how-to/work-in-parallel-with-workstreams.md)
  is useful as an idea; Luci should express it through existing project context
  and task-local records.
- GSD Core's
  [context drift gate](https://github.com/open-gsd/gsd-core/blob/83aba44627ba43aff77588894f7b81a9a7437e75/docs/features/context-drift-gate.md)
  compares commit time or file modification time. Luci's exact digest and private
  review checks remain authoritative.
- GSD Core's named “post-merge hunk verification” feature concerns installer
  patch reapplication, not swarm integration. It cannot substantiate swarm
  correctness.
- GSD 2's inspected `parallel-merge.ts` sorts its `by-completion` mode by worker
  `startedAt`. Luci should use explicit dependency/integration policy and real
  event timestamps, not copy that mismatch.
- Dispatch budgets and stop requests are not provider billing caps. Report
  observed usage, unknown usage and work still in flight separately; model and
  infrastructure charges remain separate.
- Nimbalyst's
  [Blitz implementation](https://github.com/nimbalyst/nimbalyst/blob/6f14adff484d5f0b6feb1033c190277532ec5350/packages/electron/src/main/ipc/BlitzHandlers.ts)
  runs the same prompt in competing worktrees, up to ten, followed by comparison.
  That is an alternatives experiment, not complementary task decomposition.
  Luci's first swarm workflow should avoid that duplicated cost and work. A
  future explicit alternatives mode must retain independent verification and
  deliberate selection/integration.
- Nimbalyst's older worktree reliability proposal is not proof of current
  defects: the inspected implementation already centralizes operation locking
  and preserves squash backups. Compare current source, not an obsolete issue
  list.

## Demonstrating improvement

A graph preview, this comparison and a passing fixture suite do not complete the
swarm objective. Validate a representative project with independent frontend and
backend tasks, a shared-interface conflict, a dependent integration task, a
teammate amendment, worker interruption/restart and a failing acceptance check.
The selected agents must create the application changes; the evaluator must not
substitute a prewritten result.

Compare serial Luci and parallel Luci using the same model, initial source,
requirements, machine and checks. Record:

- elapsed time from accepted dispatch to independently verified integrated result;
- peak active workers and measured memory, plus observed provider usage and
  unknown cost;
- context bytes supplied per worker and what remains available on demand;
- dispatched attempts, duplicate allocations, recovery events and interventions;
- acceptance outcomes bound to the exact integrated source digest.

Report measurements with their limits. Deterministic tests establish scheduling,
storage and permission contracts; real model workflows establish practical
delivery quality and speed. Native Windows acceptance remains separate from
cross-compilation or browser fixtures. The open release gates remain open until
their own evidence exists.

### Controlled local measurement

The first controlled measurement ran three sequential and three parallel trials
on the shared Linux development host. Each trial used the same fixture task
definition and two actual child processes. Each child read its delivered task
contract, performed 1,000,000 chained SHA-256 operations and wrote its assigned
result file. All twelve processes completed and each task's result was identical
across modes and trials. API admission, private reservations and real copied Git
checkouts were exercised; the session backend and readiness check were test
doubles. No model or Docker runtime was exercised.

| Measurement | Sequential | Parallel |
| --- | ---: | ---: |
| Median dispatch through received results | 520.6 ms | 825.5 ms |
| Observed range | 449.2–981.9 ms | 427.1–905.6 ms |
| Peak live controlled workers | 1 | 2 |
| Median overlapping computation | 0 ms | 96.6 ms |

This sample does **not** demonstrate a speed improvement. The workload is tiny,
controller/process startup is substantial, and shared-host timings vary widely.
Parallel workers used an explicit ready barrier; process overlap includes that
wait, while the separately reported computation overlap does not. Per-process
Linux high-water RSS ranged from 20.2 to 20.9 MiB. Captured files plus manifest
were 8,451–8,467 bytes per worker. Those bytes are not token usage or model cost.
No application integration, acceptance quality, deployment or competitor timing
was measured.

Raw trials, controller-stage timings, PIDs, process CPU time, workspace/context
size and limitations are retained in
`artifacts/quality-swarm/benchmark/controlled-workload.json`; `summary.json`
retains the first sample and identifies its post-measurement source snapshot.
The transient compiled test binary was not retained. The opt-in measurement is
reproducible from `control-plane`:

```sh
LUCI_SWARM_BENCHMARK=1 \
LUCI_SWARM_BENCHMARK_REPORT=/absolute/path/to/new-report.json \
go test ./internal/api -run '^TestProjectSwarmControlledBenchmark$' -count=1 -v
```

Use a new report path to preserve earlier measurements. Ordinary correctness
suites skip the repeated timing sample. This comparison supports measured local
coordination and result equivalence; deciding when parallel work pays off still
requires realistic tasks on the intended machine and model connection.
