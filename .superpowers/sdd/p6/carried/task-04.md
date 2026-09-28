# Carried plan issues — task 04 (LiteLLM admin client on its own deadlines)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## R-6 (Minor, gap, b02) — the run-time checks read back the config and see two of the four figures

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:40-44`, finding `R-6`.
Shared with tasks 09 and 24; `GroupPrice` is yours, so lead the change.

> **R-6 (Minor, gap, tasks 04, 09 and 24). The run-time checks read back the config and see two of the four figures.**
>   - `GroupPrice` carries only `Input` and `Output` (`task-04.md:613`), and P06 counts only those two (`task-24.md:609`). The cache-read and cache-write figures, which carry most of a Claude Code session's input tokens, are gated at build time only. If those two keys went unread, spend would undercount rather than read zero.
>   - `/model/info` echoes the `model_info` block the config wrote. If the pinned LiteLLM prices from `litellm_params` (the location `task-02.md:1430` itself allows), both checks pass while spend is zero. Only task 18's P03 to P05 and P12 prove that a call is charged, and they run locally only.
>   - The comments overclaim: "what LiteLLM will charge" (`task-04.md:611-618`) and the skeleton at `:224`.
>   - Fix: (a) say "the price the config gives LiteLLM, as `/model/info` reports it"; (b) either add the two cache figures to `GroupPrice` and the checks, or list them under "does not claim"; (c) recommended: task 24 reads the key's `spend > 0` after its one real call per route. That closes the cloud gap for about 10 minutes of plan text.

Fix (a) also edits the skeleton at `:224`. Commit that by pathspec with your part.
