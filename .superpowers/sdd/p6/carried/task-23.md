# Carried plan issues — task 23 (Local end-to-end proof with unmodified Claude Code)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it. Task 23 carries the most of any task —
read all six before you start, because four of them share one root cause (the host shell).

---

## b08 I-1 (Important) — the new guard compares two path spellings that will not match on the target device

Source: `.superpowers/sdd/p6/plan-rereview-b08-1.md:35-56`, finding `I-1`.

> **I-1. b08's new guard compares two path spellings that will not match on the device this plan
> targets, and task 23 never states the host shell.** `task-23.md:66-70`:
>
> ```
> route="${PANACEA_APP_GATEWAY_ROUTE_SOURCE#file:}"
> elif docker compose -f deploy/local/gateway/compose.yaml config \
>      | grep -qF "$(cd "$(dirname "$route")" && pwd)"; then
> ```
>
> `$(cd ... && pwd)` in Git Bash on this device yields `/c/Users/.../deploy/local/gateway/state`, while
> `docker compose config` resolves a relative bind source with the native absolute form, on Windows
> `C:\Users\...\deploy\local\gateway\state`. `grep -qF` then misses and step 3 prints
> *"FAIL: the stack mounts some other directory"* on a correct setup. The plan says twice that the
> target is Docker Desktop on Windows (`task-17.md:505`, `:511`, `:659`; `task-21.md:1044`), and task
> 23 states no host shell anywhere. Consequence: the one guard b08 added to stop a wave-10 surprise
> becomes a false alarm at wave 10, and the likeliest response to it is to move the path back.
> I cannot execute Docker here to settle compose's output form, so the path-form claim is my
> training knowledge — but either the guard is separator-agnostic or it must carry an UNVERIFIED
> line saying what to do when it misfires; as written it carries neither.
> Fix, three characters of work: `| tr '\\' '/' | grep -qiF "deploy/local/gateway/state"` — the
> repo-relative tail is identical in both spellings — plus one sentence in task 23 naming the shell
> its exports assume.

---

## b08 M-1 (Minor) — the guard prints FAIL but does not stop the step

Source: `.superpowers/sdd/p6/plan-rereview-b08-1.md:60-63`, finding `M-1`.

> **M-1. The guard prints FAIL but does not stop the step** (`task-23.md:67-74`). `./bin/panacea-app-api
> &` at `:76` runs either way, so the operator can miss the line and land back at the step-5 symptom
> the guard exists to pre-empt. The prose at `:64` already says "Fail here"; make it true with an
> `exit 1` in the FAIL branches, which the scratch script the comment at `:65` mandates supports.

---

## b08 carried note (same root cause as I-1) — the `$PWD` exports

Source: `.superpowers/sdd/p6/plan-rereview-b08-1.md:81-84`.

> `task-23.md:61-62` exports two `$PWD`-rooted paths that a Windows-native Go binary cannot open in
> Git Bash form; pre-existing, same root cause as I-1, and both are settled by one sentence naming the
> host shell.

One sentence naming the host shell closes I-1's second half and this note together. Say which
shell the exports assume and what the Windows-native binary needs (`cygpath -w`, or the
native form spelled out), and mark the compose-output path form **UNVERIFIED** — nobody in
this stream could run Docker to settle it.

---

## m-2 (Minor, b03) — the hostile file overrides 3 of the 9 pinned `env` keys

Source: `.superpowers/sdd/p6/plan-rereview-b03-2.md:23-29`, finding `m-2`.

> **m-2. The hostile file overrides 3 of the 9 pinned `env` keys, yet `task-23.md:169` says it "tries every door task 14's decision (e) closes".**
> - My extraction of both JSON blocks from the parts (scratch `rr2check.py` / `.out`) shows which keys the hostile file overrides:
>   - Overridden: `ANTHROPIC_BASE_URL`, `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_ENABLE_TELEMETRY`.
>   - Never overridden: `NO_PROXY`/`no_proxy`, `_USE_VERTEX`, `_USE_FOUNDRY`.
> - A real attacker pairs `HTTP_PROXY` with its own `NO_PROXY`, and that pairing is exactly what the `NO_PROXY` pin exists to defeat.
> - The generic key-by-key precedence evidence probably covers these keys, so this is Minor.
> - Fix, about 5 minutes: add `"NO_PROXY": "localhost"` and `"no_proxy": "localhost"` to the hostile file (`task-23.md:178-203`). Either add a Vertex switch too, or reword `:169` to "one key per door".

---

## m-3 (Minor, b03) — item 10's door table misses two shapes the hostile file itself produces

Source: `.superpowers/sdd/p6/plan-rereview-b03-2.md:31-37`, finding `m-3`.

> **m-3. Item 10's door table (`task-23.md:220`) misses two shapes that the hostile file itself would produce.**
> - (a) The hook's `curl` inherits the hostile `http_proxy`, because the pinned `NO_PROXY` names only the gatekeeper. A hook that runs therefore logs `POST http://gw-attacker:4999/hook-S4 HTTP/1.1` (an absolute URI through the proxy), not `POST /hook-S4`. That this is how curl behaves is training knowledge, **UNVERIFIED**.
> - (b) The CLI's own HTTPS traffic, if any is left after `DISABLE_NONESSENTIAL_TRAFFIC`, logs `CONNECT <host>:443` through the hostile `HTTPS_PROXY`. That line names no door in the table.
> - Both still fall under "any other request line is a blocker", so nothing passes wrongly. The reader just cannot read the door from the table.
> - Fix, about 5 minutes:
>   1. Add both shapes to the table.
>   2. For `CONNECT`, record the host. It is a blocker if the host is the gatekeeper or an inference host. Otherwise, record it as the proxy door open for the CLI's non-inference traffic.

Task 14's `m-1` offers an optional third addition to the same hostile file — a `statusLine`
command and a `POST /status-S4` door shape. Take it if you take m-2 and m-3, since all three
edit the same two blocks.

---

## b07 Minor 5 — item 11 names the lazy cache but not the eager refresh

Source: `.superpowers/sdd/p6/plan-rereview-b07-2.md:108-113`, "Minor 5".

> **5. Item 11 names the lazy cache but not the eager refresh.** `task-23.md:267`, `:271`. The step
> now handles "message 2 was answered". It does not handle the other half of U8's own question: if the
> CLI re-runs the helper on a timer at the TTL, it runs it during the 2 min 15 s wait, with the file
> already gone, and message 2 then fails in far less than 20 s — at which point `:271`'s "measured
> lower bound on the CLI's helper timeout" is measured from the wrong clock. One sentence: a failure
> much faster than 20 s means the helper had already run and failed, and bounds nothing.
