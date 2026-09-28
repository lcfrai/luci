# Carried plan issues — task 18 (LiteLLM capability, ZDR and pitfall probe)

Read this before you start task 18. These are residual findings the OD-31 convergence
ruling carried into the build instead of re-looping the plan. Fix each in your task,
and say in your task report what you did with it.

---

## S-1 (Important) — P14 can still PASS on a request that never reached the router

Source: `.superpowers/sdd/p6/plan-rereview-b01-2.md:7-20`, finding `S-1`.
**This is the one finding this task must not lose: P14 is the owner blocker for OD-19's
AU route, and a vacuous PASS clears the one thing it exists to measure.**

Verbatim:

> **S-1 (Important). P14 can still PASS on a request that never reached the router.**
> - `v_p14` (`task-18.md:248-266`) treats a status of `0`, `401` or `403` as UNKNOWN. Every other status with nothing recorded at either fake is a PASS (`:266`). The self-test's `p14 held` row passes on status 200 (`:109`).
> - The trap is a 4xx that LiteLLM returns before any routing, whatever the body says. Two concrete cases:
>   - **The model id drifts.** `MODEL` and `AU_MODEL` are hard-coded to `claude-opus-4-8` (`:296-297`), but the catalogue changes month to month (task 11 decision (b); task 02's example config at `task-02.md:1115` already shows `claude-opus-5`). `v_upstreams` checks only the group prefixes (`:274-283`), not the names. So if the catalogue drops `claude-opus-4-8`, the gate still passes. The P14 key still mints, because `models` names a group that does not exist. The call then gets a 400 of the "invalid model name" kind, reaches neither fake, and P14 prints PASS.
>   - **The mock is rejected.** A body refused with a 400 because of `mock_testing_fallbacks` itself also reaches neither fake. The fallback question was never asked, yet the result is PASS.
> - LiteLLM's status codes for both cases come from training knowledge and are **UNVERIFIED**.
> - The false claim sits in `task-18.md:32`: "both calls must reach the router … so 'nothing at either fake' is never a request that went nowhere." The review map repeats it at `review.md:144`. The round-1 review suggested the `0/401/403` set and it was built as written, so that set was incomplete from the start.
> - Why this matters: P14 is the owner blocker for OD-19's AU route. A vacuous PASS clears the one thing it exists to measure. This is the same class of defect as round 1's R-2 (Important).
> - Fix, about 20 minutes:
>   1. The `upstreams` gate also requires `MODEL` and `AU_MODEL` among `/model/info`'s `model_name`s. Otherwise it exits 2.
>   2. Add a positive control per key: the same key and body, without the two control fields, must add exactly one request at `fakeanthropic-au` with status 200. If it does not, the result is UNKNOWN. This proves the key, the group and the route.
>   3. A PASS then needs a status of 500 or more on the mocked call, meaning the mocked primary failed. That status is **UNVERIFIED**. A wrong guess costs an UNKNOWN, never a false PASS. Any other status with nothing recorded is UNKNOWN.
>   4. Self-test changes: `p14 held` moves to status 500. Add `p14 rejected before routing` (400, no records) → UNKNOWN and `p14 control missed` → UNKNOWN, taking the count from 40 to 42. Add a step 6 mutant (the control check → `if False:`).
>   5. Restate the rule in the P14 row (`:32`) and in skeleton `:258`.

Note the second half of fix item 5: skeleton `:258` is a line in
`docs/superpowers/plans/2026-09-25-phase-6-gateway.md`, so your commit touches the
skeleton as well as `task-18.md`.
