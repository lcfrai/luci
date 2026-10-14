# Carried plan issues — task 02 (Model catalogue and the generated LiteLLM config)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-2 (Minor, new in b02) — step 5's expected output leaves out the new test

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:30`, finding `R-2`.

> **R-2 (Minor, new, task 02). Step 5's expected output leaves out the new test.** The PASS list (`task-02.md:750-758`) names nine tests. The file now defines ten (`:105-295`); `TestParseRefusesAnUnpricedModel` (`:203`) is missing, so a run that silently skipped it would still match the expected output. Fix: add its `--- PASS` line.

---

## R-3 (Minor, new in b02) — one restatement contradicts task 09's decision (g)

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:31`, finding `R-3`.

> **R-3 (Minor, new, task 02). One restatement contradicts task 09's decision (g).** `task-02.md:1430` ends: "Task 09's sweep flags a key with requests and no spend either way." That is the check task 09 rejected (`task-09.md:39`, `:966`). It is the only stale copy (grep of the parts for `requests and no spend|requests > 0|spend == 0`). Fix: "Task 09's sweep reports any group `GET /model/info` prices at zero (its decision (g))."

---

## R-4 (Minor, gap) — "Err high, never low" does not hold for AU

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:32-35`, finding `R-4`.
This one carries an owner question (see below).

> **R-4 (Minor, gap, task 02). "Err high, never low" does not hold for AU.**
>   - `task-02.md:20` and the `models.yaml` header (`:694`) state the rule. But AU copies Global's figures (`:706`, `:714`, `:722`, `:731`).
>   - Bedrock's geographic (`au.`) inference profiles are priced separately, and, by training knowledge (**UNVERIFIED**), about 10% above the global endpoint for current Claude models. The copied AU figures therefore probably err LOW, so the AU budgets would bite about 10% late until an operator compares them with the AWS price list.
>   - The operator check stays (`task-24.md:684`), but the stated rule is false as the data stands. Fix, about 5 minutes: seed each AU figure at a declared margin above Global (1.25x clears a 10% premium) and keep it **UNVERIFIED**, or delete "never low" and say that AU may undercount until that check.

**Open owner question attached to this finding** (b02's second, recorded in
`.superpowers/sdd/p6/carried/plan-wide.md`): who compares the AU prices with the AWS price
list for `ap-southeast-2` before AU budgets are trusted? Plan the recommended option (seed at
a declared 1.25x margin, marked UNVERIFIED) and leave the owner's answer behind that margin
constant.
