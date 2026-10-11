# LOCAL-025 optional context export and return: frontend evidence

Luci's integrated upstream coding-tool flow remains primary. The new optional
**Context export and returns** control follows existing Develop run controls.
It captures selected saved/approved context for another tool and records a
reviewed return as **unverified** claims. It does not launch tools, run uploaded
commands, apply returned code, approve scope or mark project checks passed.

## Recorded validation

- `final-tests.json`: 75 assertions in four affected API/component/ProjectWork/seam
  files passed. TypeScript exited 0. All 448 selected repository frontend inputs
  match before/after. This batch precedes the final two API contract files below.
- `contract-followup-tests.json`: 26 assertions in the two affected API/component
  files passed, TypeScript exited 0, all 448 inputs stable. This adds response
  request-key validation and aligns the summary-only return count. The 25 earlier
  affected assertions overlap; do not add these totals as unique coverage.
- `earlier-typecheck-tool-output.txt` retains the initial UUID-state type error
  copied from the terminal tool output. It is labelled as such, not fabricated
  as an original redirected compiler log. The explicit string type correction
  is covered by both successful typechecks.

The tests exercise task selection without Execute authority, approval blockers,
report project/handoff/digest ownership, separate source observation versus check
claims, context amendment refusal, explicit consent reset, lost mutation replies,
exact-key history reconciliation and original-review retry, edit authority churn,
fresh read authority, project change while a download is pending, browser draft
reopening/isolation, read-only downloads, and existing ProjectWork navigation.
Persistent report drafts include their exact handoff and request identity; they
do not grant permission or verification. The authority and HTTP results are
controlled fixtures.

`source-bindings.json` records seven final changed frontend inputs and the two
existing base files. New-file absence at the base is explicit. Frozen copies
are under `source/after` and `source/before`. Broader selected input manifests
cover repository frontend source/configuration; installed dependencies and
compiler/tool bytes are not claimed as captured. The root backend API tests
and production route authority remain separate.

No browser screenshot, accessibility audit, native Windows, external provider,
Docker, performance or end-to-end acceptance was executed in this batch.
Commands and raw test/typecheck reports are retained. Frontend files were not
committed by this agent; the integrating commit must bind these hashes.
