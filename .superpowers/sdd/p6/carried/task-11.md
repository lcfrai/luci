# Carried plan issues — task 11 (gatekeeper proxy: allowlist, `model` rewrite, headers, streaming)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## S-2 (Minor) — task 11 detects the decoded escape but never says how to write it

Source: `.superpowers/sdd/p6/plan-rereview-b01-2.md:22-25`, finding `S-2`.

> **S-2 (Minor). Task 11 detects the decoded escape but never says how to write it.**
> - The fixer's own root cause (ledger, round 2): the Edit and Write tools decode `a` in their arguments. The implementer of task 11 writes `proxy_test.go` with the same tools. The part says so only at step 6 (`task-11.md:1007`): if `grep -c u0061` prints 0, "that row tests nothing". It gives no remedy, and step 1 has no warning.
> - The older `model` row (`:254`, `TestTheRouteCannotBeChosenByTheBody/escaped_key`) carries the same hazard and has no check. Decoded, it becomes a copy of `model twice`, still passes, and silently tests nothing.
> - Fix, about 5 minutes. Step 1 says to write both escaped rows with a script that builds the backslash with `chr(92)`, then run `grep -c 'u0061'` and `grep -c 'u006d'`, and expect `1` each. Step 6's check says "stop and rewrite the row", not only "tests nothing".

(The two `\u00xx` spellings above are written here with a literal backslash. If this file
shows them decoded, that is the same defect the finding is about — re-derive them from the
source report with `grep -n u006 .superpowers/sdd/p6/plan-rereview-b01-2.md`.)

---

## S-3 (Minor, pre-existing) — task 11's `:5` prose lost its escape

Source: `.superpowers/sdd/p6/plan-rereview-b01-2.md:27-29`, finding `S-3`.

> **S-3 (Minor, pre-existing, outside the batch). Task 11's `:5` prose lost its escape.**
> - It reads "`"model"` decodes to `model` in both languages". The escaped spelling was lost before b01 (same bytes at `8b8a7d7`, checked with `od -c`). Only prose is affected, because the test at `:254` keeps `model`.
> - Fix: the same script edit as S-2.
