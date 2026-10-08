# CLOUD-014 context-quality evaluation preparation

The [six-case corpus and offline evaluator](../../../scripts/acceptance/context-quality/README.md)
are implemented. Fifteen controlled Python tests passed in **0.085 seconds**
(0.316 seconds including the interpreter harness). All fourteen source files
matched their before/after hashes in the [preparation receipt](2026-10-08-cloud014-context-quality-preparation.json).

A subsequent independent review identified two evaluator gaps: the parallel
capture did not enforce its declared concurrency limit, and result files did
not carry their scoring-source hashes. Both are corrected. The current
seventeen-test suite passed in **0.136 seconds** (0.423 seconds including the
harness), with all source hashes stable; the [follow-up receipt](2026-10-08-cloud014-context-quality-followup.json)
preserves exact lineage and the prior receipt. Overlap checking covers each case
capture; batch-wide checking still requires all cases' intervals. No live score
was involved in either check.

The cases cover consequential ambiguity, project/role isolation, contradictory
amendments, stale source/context, misleading dependency handoffs and unresolved
work after restart. Worker-visible inputs and evaluator-only expected behavior
are separate. The evaluator checks explicit dispositions, exact citations,
retained obligations, evidence classification and supplied source/request/response
lineage. It detects the controlled examples of false verification and leaked
synthetic canaries, refuses altered answers and missing captures, and preserves
unknown usage/cost/resource values.

Reproduce the controlled check from the repository root:

```sh
python3 -m unittest discover -s scripts/acceptance/context-quality -p test_evaluate.py -v
```

The checks use temporary JSON files and start no model, Luci, browser, Docker,
compiler or external process from the test cases. Their positive specimens are
derived from the evaluator rules solely to test the parser/scorer. They are not
model answers or quality results. The raw log remains in ignored
`artifacts/cloud014/tracked-preparation-tests.log`; its SHA-256 travels in the
public receipt.

The [capture contract](../../../scripts/acceptance/context-quality/RUN-FORMAT.md)
requires actual selected-source origins, native Luci scope/context artifacts,
provider requests and exact response extraction for a Luci live label. The
[comparison protocol](../../../scripts/acceptance/context-quality/PROTOCOL.md)
defines same-model serial/parallel trials and full-context/omission comparisons.
These controls check consistency of supplied evidence; they cannot authenticate
an external run or prove all permission boundaries from an answer.

**Still unrun:** live-model cases, real Luci context-delivery trials for this
corpus, competitor executions, human semantic grading and native Windows
evaluation. Semantic quality remains explicitly unscored. There is no overall
quality pass, product-completion claim, latency improvement or market ranking.
