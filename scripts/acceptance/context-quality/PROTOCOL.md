# Repeatable same-model comparison protocol

This prepared protocol has not been run on a model, Luci or any competitor.
The six cases evaluate context-grounded orchestration judgments. They do not
substitute for agents building and independently verifying a real application,
the sustained four-project runtime acceptance, or native Windows testing.

## Freeze before execution

1. Record exact evaluator/corpus/oracle hashes, harness source revision, provider,
   model name and resolved snapshot when available, generation parameters,
   context/tool limits, seed support, retry policy, machine and capture adapter.
   A model alias without a resolved snapshot is recorded as unknown and cannot
   support a claim about a fixed model version.
2. Use three or more trials per case/condition, with the number fixed before
   seeing output. Publish every trial, refusal, timeout and provider error in
   the denominator. Keep failed raw artifacts; no best-of selection, invisible
   prompt repair or replacement trial. Request retries retain their own usage
   and timing; do not call an overwritten output the first attempt.
3. Predeclare conditions: full authorized context baseline; current Luci task
   context; omitted-brief ablation; omitted-handoff ablation for cases 5/6 only
   when the source kind exists (the executable preparation step is authoritative).
   No condition is allowed evaluator-only answers. Full context means the full
   *authorized* project context, never all projects' private data.
4. Predeclare serial and parallel scheduling on the same model/settings and
   hardware. Use exactly the same cases, initial source contents, frozen
   requirements and scoring rules in both modes. Serial permits one active
   request/worker; parallel permits at most four under the normal global
   resource/provider budget. No workload or quality requirement is dropped to
   make parallel appear faster. Randomize/interleave condition order and retain
   the order; allow provider cache effects and host contention to remain visible.

The current corpus has six independent judgments. Grouping those across four
project workspaces is a delivery/isolation exercise, **not** evidence that six
workers solving one shared build can be run independently. Actual task graph
parallelism still obeys dependencies, shared interfaces and write scope.

## Actual Luci capture, not a generic prompt labelled as the app

Seed synthetic project briefs, approved specifications, source revisions, task
graphs and access roles through the same project APIs/flows that real work uses.
Record the mutation receipts and source hashes. Preserve corpus source IDs in
the seeded documents. Synthetic evidence in the *scenario* is labelled as such;
do not inject fake passing receipts into Luci's real verification store.

Capture the actual reviewed native context manifest and files, private
project/run/task/operation binding, selected compact context and direct-dependency
packet, then the actual provider request bodies and responses. Bind selected
source text to its exact origin artifact and JSON pointer. Retain raw native
digests separately from this evaluator's projection digest. Do not just paste
the full prepared case in a last user prompt while claiming Luci assembled it.
If the harness cannot deliver a source, record that omission and its effect;
do not repair the capture to meet the intended condition.

For C011, exercise the report-to-context path: prior worker output is attributed
as reported, actual recorded checks remain separate, direct dependencies use the
current integrated operation, and unanswered questions survive compaction. The
observed dependency packet must retain the exact receipt revision used at
reservation. Read a on-demand packet through the worker's normal tool path and
capture the subsequent provider request; file existence alone is not delivery.

For case 2, seed four distinct synthetic projects. Place the evaluator-only
canaries in the three *other* private project fixtures, then stop exposing seed
tools/files to the worker. The clinic viewer receives only authorized clinic
context. Retain attempted read/mutation denial observations separately. Search
every actual context/request/response capture for the known synthetic markers;
the executable validator checks them in declared request/context/answer artifacts.
Missing marker leakage does not prove all ACLs or all possible data isolation.

For case 6, record actual source/context/operation state before and after an
authorized fixture-only restart, rather than merely adding the word 'restart'
to a prompt. The original operation stays identifiable, its output and pending
review survive, and no duplicate or capacity release is hidden by a newer run.
The model's recommendation is scored separately from runtime enforcement. Until
these real product actions are captured, label the run a direct scenario
baseline, not installed Luci recovery acceptance.

The adapter must record every model call involved in the decision, including
orchestrator, workers, retries, background summary and tool-output followups.
Capture provider JSON before credential headers and capture the actual final
answer without hand editing. Save source/context snapshots before and after.
The current evaluator validates supplied lineage and exact selected-text delivery;
it cannot authenticate arbitrary files or prove there were no omitted calls.
Independently review the adapter and transport capture completeness first.

## Score without mixing unlike claims

Run the offline validator for each retained trial. Invalid schema, missing
captures or inconsistent provenance remain failed/incomplete trials. Report the
counts, not a mean of only valid answers. Preserve separate dimensions:

- constraint and action retention, including source-reference correctness;
- material required questions, unnecessary topics and safe independent work;
- evidence attribution and explicit unsupported verified/approved claims;
- durable retained obligations and exact current dependencies;
- blinded human semantic ratings and disagreement counts.

There is no automatic weighted overall score. A critical authority or false
verification claim cannot be offset by more correct IDs. Ablation outputs report
both full-context expectation coverage and coverage whose required sources were
delivered; do not call correct 'unknown' behavior hallucination when context was
deliberately omitted. Human reviewers assess whether the model asks for the
missing material facts rather than guessing.

Report performance separately: complete batch wall time from first accepted
dispatch through all required *judgments*, per-request windows, actual observed
request overlap, all attempts, supplied bytes, token counts with unknowns and
observed charges with unknowns. Do not sum parallel durations as elapsed time,
infer actual concurrency from a configured limit, or label judgment latency as
verified-build latency. Resource observations need the real owned-process
sampler, including workers and browsers; this evaluator does not generate them.

Pair trials by case, seed, source/context contents and model/settings. Report
all sample values, median/range and failure counts per dimension. Small samples
and shared-host/provider variance preclude a reliable speed-superiority claim.
Record provider caching, snapshot uncertainty and machine contention. Preserve
semantic differences and quality failures alongside any faster result.

## Competitors and publication

The primary-source GSD Core, GSD, GSD 2 and Nimbalyst comparison is a capability
study, not executed quality evidence. Running an upstream harness later requires
its exact revision, configuration, license compliance, same-model cases, source
fixtures, quality rubric, complete captures and bounded resource policy. Do not
publish a competitor score or 'best' claim from this corpus or documentation.

Publish only allowlisted counts, ratings, case/corpus hashes, source and adapter
identities, artifact hashes and qualified metrics. Keep raw model/provider bodies
and native private state in isolated acceptance storage. Public source receipts
must identify which checks were controlled parsers, actual product delivery,
real model reasoning, human review or platform runtime tests. At preparation
time all live-model, competitor and semantic-review result fields are unrun.
