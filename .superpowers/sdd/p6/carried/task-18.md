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

---

## b06 Important 2 — P15's reading (b) mints against a team that is never created

Source: `.superpowers/sdd/p6/plan-rereview-b06-2.md:76-91`, "Important 2".

> **2. `task-18.md:415` — P15's reading (b) mints a production-shaped key against a team that is never
> created, and never checks whether the key minted.**
> `p15_reset` does `ids, team = {"repeated": u("p15r"), "quiet": u("p15q")}, u("p15t")` (`:412`) and
> then `call("POST", LL + "/v1/messages", prod_key(master, uid, team, [MODEL]), msg(MODEL, "p15"))`
> (`:415`). `prod_key` (`:369`) posts `/key/generate` with that `team_id`, and the part's own
> `budget_probe` creates the team first — `call("POST", LL + "/team/new", ...)` at `:378` — and guards
> the result with `if not k:` at `:380`. P15 does neither.
> Failure scenario: the pinned LiteLLM refuses `/key/generate` for an unknown `team_id` (plausible
> enough that `budget_probe` guards against it); `prod_key` returns `None`; `call` sends the message
> with no key and gets a 401 whose status is discarded; neither user accrues spend; the 120 s poll at
> `:416`-`:418` times out; `v_p15` takes its second branch (`:275`) and reports
> `(b) spend stayed 0 before the wait: unpriced, so no reset is visible` — UNKNOWN, with the wrong
> cause named. Seam S6.12 reads that as "take option 2", so the phase drops LiteLLM's native person
> and project budgets on a reading that was never taken, and the evidence line blames the model price
> table. Fix: `call("POST", LL + "/team/new", master, {"team_id": team, "max_budget": ROOMY})` before
> the loop, and a `if not k: return None` guard so `v_p15` can say "(b) the probe key did not mint".

---

## b06 Important 3 — the table promises (b) on a one-hour window; the program runs it only on a one-minute window

Source: `.superpowers/sdd/p6/plan-rereview-b06-2.md:93-107`, "Important 3".

> **3. `task-18.md:437` — the table promises reading (b) on a one-hour window; the program runs it only
> on a one-minute window, and then tells the operator to set a flag they already set.**
> The probe cell says "create **two** users with the shortest window the image accepts (try `"1m"`; if
> it refuses, `"1h"`, and record which)" (`:33`), and `clock_probes` duly detects either
> (`:434`-`:436`). But the call is
> `b = p15_reset(master, u, window) if window == "1m" and os.environ.get("GW_PROBE_SLOW") else None`
> (`:437`), and `p15_reset`'s wait is the hard-coded `60 + 2 * RESET_JOB_S` (`:420`) — one minute, not
> one hour.
> Failure scenario: the pinned image rejects `"1m"` and accepts `"1h"`. The operator sets
> `GW_PROBE_SLOW=1` and waits; (b) never runs; `v_p15`'s first branch (`:274`) prints
> `(b) not run (shortest window accepted: 1h); set GW_PROBE_SLOW=1`; the results document records
> UNKNOWN(b), and S6.12 takes option 2 on an image where reading (b) was obtainable in about 80
> minutes. Fix: gate on `window` being truthy rather than `== "1m"`, pass the window's length in
> seconds into `p15_reset` instead of the literal `60`, and make the not-run message distinguish "flag
> unset" from "no window the image accepts".

---

## b06 Minors 1 to 4

Source: `.superpowers/sdd/p6/plan-rereview-b06-2.md:111-141`.

> **1. `task-18.md:283` — an admin API that cannot be read is recorded as a measured FAIL(a).**
> `user_info` returns `(None, None)` on any non-200 (`:394`-`:399`), so `before` is `None`, the guard
> `if before is not None and before == after` fails, and the function returns
> `FAIL, "the repeat moved budget_reset_at None -> None; ..."`. Every other verdict in this program
> reports UNKNOWN when the reading could not be taken — `v_p14` for a request that never reached the
> router (`:262`), `v_p02` for a key refused at once (`:168`). Fix: `if before is None or after is
> None: return "UNKNOWN", ".../user/info unreadable..."` before the comparison.

> **2. `task-18.md:277` — the cell's PASS(b) and the code's PASS(b) are not the same test.**
> The cell says "PASS(b) is **both** users at `spend == 0` with `budget_reset_at` moved" (`:33`), but
> `v_p15` checks the moved date only for the quiet user (`b["quiet"][2]`, `:277`); the repeated user's
> third element is computed at `:425` and never read. Defensible — with (a) FAILing the repeated
> user's date has moved by construction — but the restatement disagrees with the code, and this batch
> was raised because a restatement disagreed with the code.

> **3. `task-18.md:435` — the window detection asks a different question from the one production
> asks, and the interval the cell says to record is never recorded.**
> The detection posts `{"user_id": ..., "budget_duration": w}` with **no** `max_budget` (`:435`-`:436`),
> unlike `ensure_user` (`:403`) and `budget_probe` (`:377`); an image that refuses a duration without a
> budget yields `window = None` and reading (b) never runs even where it would have worked. And the
> cell says "record the interval observed; allow two of them" (`:33`) while `RESET_JOB_S = 600` is an
> assumed constant (`:392`) that no verdict string carries into the results document, so the reader of
> `docs/verification/2026-09-25-phase-6-litellm-probe.md` cannot tell what was waited out.

> **4. `task-18.md:527` — the slowest and newest code runs in the middle of `client()`, not at its
> end.** `clock_probes` sits between P12 and `tmp_probes` (`:527`-`:528`), so under `GW_PROBE_SLOW=1`
> it blocks about 21 minutes before P07, P10, P13 and P02 are emitted, and any exception in code that
> has never been executed loses those four probes as well as P15 and P16. P02's own timing survives
> (its `max(0, 70 - elapsed)` at `:530` simply becomes 0, and a 60 s key is still expired), so this is
> ordering, not correctness. Moving the call below `say("P02", ...)` costs nothing — `probe.sh` sorts
> the lines into id order at `:687`.

---

## b07 Minors 2, 3, 4 and 6

Source: `.superpowers/sdd/p6/plan-rereview-b07-2.md:84-120`. Minor 3 is the same class as
S-1 above, one door over — settle both with one rule.

> **2. `P06B_POLL_S` pins the evidence text, not the poll.** `task-18.md:431` declares the 90 seconds
> and `v_p06b` interpolates it into the PASS line (`:223`), while the window is actually
> `for _ in $(seq 1 30)` plus `sleep 3` in the driver (`:769-773`) — two independent literals. Change
> the constant and the probe reports a window it never waited. (P06's loop at `:729-733` has the same
> shape but makes no claim about it, so only P06b can lie.) Either derive `seq`'s bound from the
> constant, or say in the comment at `:431` that the two move together.

> **3. "Reached the router" is a three-code blacklist, not evidence that the upstream was tried.**
> `task-18.md:216`. The probe wants the one failure this stack can produce — a connection error to a
> stopped upstream, which LiteLLM surfaces as a 5xx. Any other non-2xx LiteLLM raises **on its own
> account** (request validation, a rate limit) is not in `(0, 401, 403)`, so it falls through to the
> PASS branch on a call the upstream never saw: the same shape as the hole round 1 closed, one door
> over. `if not (500 <= status < 600): UNKNOWN` says what the probe means. Narrow in practice —
> `v_upstreams` exits 2 before any probe runs unless both groups point at their fakes (`:619-624`),
> which pre-empts the likeliest 404 — so this is a wording-of-the-guard issue, not a live hole.

> **4. A p06b process that dies is escalated to the owner as a ZDR breach.** `task-18.md:756` runs the
> exec with `|| true`, and `:776` defaults a missing DATA line to `mint=0`, which `v_p06b` reads as a
> non-200 mint -> **FAIL** -> `exit 3` (`:780`) -> task 23 stops (`task-23.md:37`) -> the owner is
> written to "as a blocker in the words of the row" (`task-18.md:866`), and the row's consequence cell
> (`:25`) asserts that LiteLLM's exception path keeps the prompt. Failing closed is right; the words
> sent to the owner are wrong. `mint == 0` (LiteLLM unreachable, or the probe never printed) is
> **UNKNOWN**; a real non-200 stays FAIL.

> **6. 85 seconds were added to a run whose timeout is already the Bash tool's maximum.**
> `task-18.md:799` still says 600000 ms while listing more waiting than that: P02's 70 s, up to 90 s
> each for the budget probes and P11, the throwaway's 120 s readiness loop (`:716-719`), P06's and
> P08's 90 s polls, and now P06b's 90 s poll plus a 120 s call timeout (`:586`). Every poll is
> expected to break in seconds, so this is a worst-case remark, not a prediction — but the sentence
> should say that, because an implementer who hits it has no second attempt inside one tool call.
