# Context-quality evaluation — prepared, no model results

This is an original synthetic six-case corpus and an offline executable
evaluator for CLOUD-014. It is not a model, Luci integration, benchmark result,
competitor ranking or product-completion claim. No GSD or Nimbalyst code,
prompts or assets are imported. The design borrows the *ideas* of task-scoped
context, persistent verification gaps and attributed handoffs described in
the repository's [agent harness comparison](../../../docs/luci/AGENT-HARNESS-COMPARISON.md).

The scenarios concern realistic application decisions:

| Case | Decision being evaluated |
| --- | --- |
| `01-ambiguity` | Client portal sharing, tenancy, payments and release are unresolved; useful accessible local work can still proceed. |
| `02-isolation` | Four similarly named projects and a clinic viewer role must retain their own data and authority. |
| `03-amendment` | A public-sharing proposal conflicts with approved private access while a frozen task is running. |
| `04-stale-source` | A real historical check passed, but source and timezone requirements have changed. |
| `05-dependency-handoff` | The current worker claims all checks passed; independently recorded integration checks disagree. |
| `06-restart` | An unknown old worker, retained lease, unanswered question and older review survive restart and a newer run. |

`cases/` contains worker-visible sources, task and assessment IDs. The prompts
ask for structured recommendations plus a natural operator explanation.
`evaluator-only/` contains expected dispositions, source references, question
topics, retained obligations and synthetic canaries. **Never expose that
directory, tests or evaluator source to workers.** Copy only a prepared packet
into an isolated model/harness fixture. Visible assessment IDs deliberately make
structural scoring reproducible; they are not hidden evidence that a model
discovered every issue unaided.

## Prepare one immutable input

Python 3.11+ and its standard library suffice. No dependency installation is
needed. Use fresh private output files outside this source directory.

From the repository root, first run `cd scripts/acceptance/context-quality`.
Create your private evidence parent directory separately; the evaluator refuses
overwriting existing files and never makes a model call.

```sh
python3 evaluate.py prepare --case 05-dependency-handoff \
  --variant full-authorized --output /ABS/FRESH/worker-input.json
```

The output includes the response contract and `inputSHA256`. That identity is
the SHA-256 of canonical worker-visible JSON before adding the identity field.
The evaluator-only answers do not contribute to, or appear in, this packet.
The output must be absent and its existing parent must resolve canonically.

Variants are `full-authorized`, `luci-task-context`, `omit-brief`, and
`omit-handoff`. The first two deliver the same authorized source set through
different harness conditions; the Luci condition must use actual product context
delivery. Ablation removes only the named source kind and retains assessment
questions. It refuses a case with no source of that kind. Gold expectations
remain frozen; scores report full-context recall and available-source recall
separately. A model must mark missing information unknown rather than inventing
citations. Ablation measures delivery loss, not a new optimal-policy reference.

## Evaluate retained captures

```sh
python3 evaluate.py evaluate --run /ABS/PRIVATE-RUN \
  --output /ABS/FRESH/result.json
```

The private run directory must contain `run.json` and every declared bounded
artifact, with exact SHA-256 hashes. See [RUN-FORMAT.md](RUN-FORMAT.md) for the
contract. The evaluator performs no model calls or product actions. It refuses
unknown/duplicate fields, identity mismatch, unknown citations, altered response
text, missing source delivery, hash mismatch, linked/escaping evidence, oversized
artifacts and inconsistent serial timestamps. A refused run is a retained failed
trial, not a reason to repair a model answer and rerun its grading invisibly.

Observed request overlap must stay within the declared concurrency limit, using
half-open intervals. This checks one case capture; a parallel multi-case batch
still needs a sweep across all case intervals. Every result retains exact
evaluator, oracle, case and isolation-rule hashes alongside artifact lineage.

Live labels require source/context/request/response captures and pinned harness
and adapter identities. These are **provenance-consistency and required-evidence
checks**, not a proof that an external run really happened or that captures were
not fabricated. An evaluator cannot establish all authorization enforcement from
a model's text. Independent capture/review remains essential.

Results report constraint retention, action classifications, material questions,
unnecessary question topics, attributed evidence and handoff completeness
separately. Unsupported `recorded-pass` claims and immediate forbidden actions
are explicit critical structured violations. Prose correctness and useful
ambiguity handling are left **not scored** until [blinded human review](HUMAN-REVIEW.md).
There is no combined quality score, automatic quality pass or leaderboard.

Observed request duration, source/request bytes, response-reported tokens and
actual billing evidence are separate. An unavailable metric remains `null`;
known subtotals do not turn missing usage into zero. These six judgment exercises
do not measure verified application-build latency, whole-computer responsiveness
or owned-process memory; those fields remain unknown. Bytes are not token counts.
See [PROTOCOL.md](PROTOCOL.md) for the same-model serial/parallel experiment and
real Luci capture requirements.

## Controlled validation only

```sh
python3 -m unittest discover -s . -p test_evaluate.py -v
```

These tests create bounded synthetic JSON files in temporary directories. They
invoke no process, network, model, Luci, browser or Docker. Positive schema
specimens are generated from evaluator expectations and explicitly labelled
`controlled-parser-fixture`; they are **not** example model answers or live
quality successes. The tests include false verification, source/role mismatch,
tampering, omitted context, request leakage, unknown metrics and response editing.

The recorded check status and exact source hashes belong in a separate
preparation receipt only after this command actually runs. There are currently
no live-model scores, executed GSD/Nimbalyst comparisons or Windows observations
from this corpus.
