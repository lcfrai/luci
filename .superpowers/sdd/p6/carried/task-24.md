# Carried plan issues — task 24 (Cloud proving scripts, written and not run)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-6 (Minor, gap, b02) — the run-time checks read back the config and see two of the four figures

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:40-44`, finding `R-6`.
Shared with tasks 04 and 09. Full text in `.superpowers/sdd/p6/carried/task-04.md`. Your
halves are P06 and the recommended fix (c):

> - `GroupPrice` carries only `Input` and `Output` (`task-04.md:613`), and P06 counts only those two (`task-24.md:609`). The cache-read and cache-write figures, which carry most of a Claude Code session's input tokens, are gated at build time only. If those two keys went unread, spend would undercount rather than read zero.
> - `/model/info` echoes the `model_info` block the config wrote. If the pinned LiteLLM prices from `litellm_params` (the location `task-02.md:1430` itself allows), both checks pass while spend is zero. Only task 18's P03 to P05 and P12 prove that a call is charged, and they run locally only.
> - Fix: ... (c) recommended: task 24 reads the key's `spend > 0` after its one real call per route. That closes the cloud gap for about 10 minutes of plan text.

---

## R-7 (Minor, gap, b02) — the "does not claim" list omits `/model/info`

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:45`, finding `R-7`.

> **R-7 (Minor, gap, task 24). The "does not claim" list omits `/model/info`.** `task-24.md:679` lists the reply shapes it has not checked against the real image, down to "LiteLLM's `/team/info` and `/user/info`". P06 now also parses `/model/info` (`:605-613`), which task 04 marks **UNVERIFIED** (`:618`). Fix: add it.

---

## Related open owner question (recorded, not yours to answer)

b02's second owner question, carried in `.superpowers/sdd/p6/carried/plan-wide.md`: who
compares the AU prices with the AWS price list for `ap-southeast-2` before AU budgets are
trusted? Your operator check at `task-24.md:684` is where the answer lands. Keep the check
and keep the prices marked **UNVERIFIED** until it has an owner.
