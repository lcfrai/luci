# CLOUD-071 — browser Restart refusal diagnostics

The original C069 warm/cold response listener asserted 200 before reading the body. Installed b94 warm-attempt2 therefore lost its Restart409 problem diagnosis; that original failure and all fixture bytes remain unchanged. The original warm launcher snapshot is byte-identical to that attempt's accepted harness.

Only two external launchers change. They read the known exact Restart response within a five-second budget and retain the existing allowlisted problem projection before rejecting non-200. Refusals, malformed/error/oversized bodies and late outcomes retain pending mutation state. Valid200 settlement still checks session/project/harness/status/generation and active workflow, then durably records before clearing pending. A409 supplies diagnosis only; it does not prove allocation absence or certify cleanup.

Accepted observations are tracked and drained within5.5seconds before browser closure because the scenarios independently reject non-200. The response gate then seals. No scenario, outer-runner, admission, ownership, mutation or cleanup guard is relaxed; restoration invariants prove every other launcher byte unchanged.

Playwright materializes response.body() before its size check. The2MiB bound is for parsing/retention and bounded hash work, not a streaming transport-memory cap. Oversized responses retain byte count and a clearly labeled bounded-prefix hash only; raw bodies/errors never enter public receipts.

Frozen validation uses the exact installed page.on response-handler source inside an inert VM with controlled responses. Original warm and cold each fail the same409 diagnosis assertion as expected. Fixed warm and cold each pass35 cases, including200 identity, malformed/oversized/private/error responses, timeouts, duplicate/unrelated requests, scenario-failure/drain ordering, and late resolution/rejection. Each pair uses the identical frozen test source. All stderr files are empty. No browser, Docker or compiler was launched; matching installed validation remains pending.

Runtime generator inputs are warm/launch.mjs and cold/launch.mjs with their unchanged sibling scenario.mjs and outer-run.py. Exact original/fixed SHA256 values are in source-binding.json. CLOUD-071.patch contains the two launcher-only diffs. Original snapshots, initial diagnostic runs, final paired receipts, source bindings and original actual-failure hashes are retained; the original private actual result body is not copied here.

Supplemental exact-drain-boundary test passes once per variant: callback acquisition is sealed before awaiting the older accepted observation. Runtime launcher/scenario pins are unchanged.
