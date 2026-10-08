# Blinded semantic review

Deterministic matching establishes whether an answer contains the expected
structured decision, source ID or retained item. It cannot establish that the
reason explains that decision correctly, that a question is well phrased, or
that the answer is useful. Do not present keyword or ID matching as semantic
quality. The evaluator always leaves semantic quality and overall pass unscored.

Before seeing harness name, model identity, serial/parallel condition, resource
observations or structured scores, two reviewers independently read the frozen
visible case, the *actual unedited captured answer*, and evaluator-only facts.
Use randomized answer labels. They record the following dimensions separately,
with exact answer excerpts and source IDs for each finding:

Hide packet metadata naming the variant/condition/harness in the review display,
while retaining the original captured bytes and hashes. Show the actual delivered
sources and explicit omissions needed to judge uncertainty. Those omissions may
make an ablation inferable; disclose this limit to blinding rather than claiming
reviewers could not infer a condition.

| Dimension | Required judgment |
| --- | --- |
| Constraint retention | The explanation retains material approved constraints, project/role scope, and source identities. Correct IDs with contradictory prose fail this criterion. |
| Appropriate ambiguity | Questions concern decisions that affect the solution; already-settled preferences are not asked again. The answer explains what each question blocks and what safe independent work can continue. |
| Evidence attribution | Historical passes, worker claims, real failures, missing checks, integration and acceptance are distinguished. No prose implies a completed action or permission that the case does not establish. |
| Durable handoff | A later worker can recover the current operation/dependency, frozen inputs, unresolved question, failed/missing checks and next action without repeating the whole conversation. |
| Operator usefulness | The user can tell the stage, what is being built, what changed, what needs attention, and how to amend/review/continue. |

For each dimension use **meets / partial / fails / cannot-assess**, not an
unexplained numerical score. `cannot-assess` requires the missing information.
Record critical issues separately: cross-project data or authority leakage,
unsupported verified/released claims, silently approved amendments, duplicate
unknown work, unsafe loss of retained changes, or erased unresolved obligations.
One critical error cannot be averaged away by good wording elsewhere.

Each reviewer records an opaque reviewer ID, review time, answer artifact SHA,
visible-input SHA, rule-set SHA, individual ratings, findings and conflicts of
interest. Preserve both original reviews and disagreements. A third reviewer
can adjudicate; do not replace disagreeing original ratings. Report agreement
counts and unresolved disagreements rather than selecting the favorable rating.
These are human observations, not claims independently authenticated by this
offline evaluator.

Before running live trials, preregister any acceptance rule. A reasonable local
release *gate for this corpus* requires no critical finding, every material
constraint/evidence/handoff requirement met, and questions that do not stop
unaffected work. This does not establish the full app's readiness, model coding
ability, four-project runtime performance or superiority to other products.

No human semantic grades have been collected with this prepared corpus.
