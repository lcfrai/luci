### Task 18: LiteLLM capability, ZDR and pitfall probe

**Measure what the pinned LiteLLM actually does, before anything in the cloud depends on it.** Tasks 02, 04, 07, 09 and 11 each rest on a LiteLLM behaviour that this plan knows only from LiteLLM's documentation as remembered, or from Panacea's incident record: that a caller may choose a virtual key's value, that `duration` expires a key, that user, team and key budgets bite, that no prompt text is stored, which path a given `api_base` produces, whether a body or header can re-attribute spend, whether SSE is relayed unbuffered. Open risk 3 names this. This task writes one script, `probe.sh`, that asks the running local stack (task 17) each question and prints `PASS`, `FAIL` or `UNKNOWN` with its evidence, and one results document that records the answers **against the pinned image digest**. A new digest means a new run.

**A FAIL is a finding, not a broken build.** Each probe names the decision it tests and that decision's fallback, so a FAIL routes straight to the part that changes. Three FAILs stop the phase and go to the owner, and the script exits 3 on each: **P06** (prompt or response text stored by LiteLLM), which breaks OD-19's zero data retention on our side; **P13** (the Global path loses what the CLI sends), which breaks OD-20; and **P14** (a body fallback on an `au/` group reaches the Global upstream), which breaks OD-19's AU route (review C-1).

**Where the probes run.** LiteLLM publishes no port (task 17, P6-D1), and that stays true for the probe. The client half runs **inside the LiteLLM container**, with `docker compose exec -T litellm python3 -`: the image already carries Python (task 17's step 5 checks), so no probe image is pulled, and the master key is read from `/run/secrets/master_key` in that container, never passed on a command line. The database half runs `pg_dump` and `psql` inside the Postgres container. P07, P10 and P13 need LiteLLM configs the stack does not run, so the script starts a **throwaway LiteLLM** from the same digest, `gw-probe-litellm` on `:4011`, with no database and a throwaway master key, loaded by `docker cp` (a bind mount of a Git Bash temp path is unreliable on Docker Desktop for Windows), and removes it on exit.

**Evidence never includes a key or a body.** The client redacts every `sk-…` string and truncates every error to 160 characters; it prints statuses, counts, timings, field names and the fake's `/_captured` path and key lists (task 16), which hold no value. The canary for P06 is a fresh random string per run.

**Pricing is a precondition, not a probe.** P03 to P05 and P12 need LiteLLM to put a non-zero cost on a call to the fake. LiteLLM prices from its own model-cost table by model name (training knowledge, **UNVERIFIED**), and task 16's fake echoes the requested id. If the pinned table does not know the public id, spend stays `0`, budgets can never bite, and those probes report **UNKNOWN (unpriced)** rather than a false PASS. Task 02 now renders an explicit per-token price into every group's `model_info` on every target (its decision (f), review I-5), so an unpriced result here means the pinned LiteLLM does not read those `model_info` keys: record which keys it does read, and task 02 moves the prices (its "does not claim" list). The script checks spend before it judges any budget. The operator also writes into the evidence file whether the `/model/info` answer the fake-mode gate reads shows every group's `model_info.input_cost_per_token` and `output_cost_per_token` above zero: that is the shape task 04's `ModelPrices` and task 09's price check assume (**UNVERIFIED** until then).

**Probes, and what each FAIL changes.** P01 to P12 are the skeleton's; **P13 is added here**, because OD-20's "the cloud tracks the latest CLI" depends on it and no other probe covers it. **P14 is added by review C-1**: task 11 drops LiteLLM's body control fields (`proxy.ControlFields`), and P14 asks whether LiteLLM itself would keep an AU request in Australia if one got through. **P15 and P16 are added by review M-7**, the two budget clocks: keysmith sums a UTC calendar month while LiteLLM's own budgets run `"30d"` from creation, and nothing in the plan asked whether that window can be made the calendar's (P16) or whether keysmith's own ten-minute re-ensure keeps pushing it away (P15). Neither is a blocker: each names the part that changes.

**Every budget probe mints the production key shape** (review I-8). keysmith always mints a key with `user_id` **and** `team_id` and a `max_budget` slice (task 07, `mint`). A probe key missing either would ask a question production never asks: LiteLLM may apply a user's budget only to that user's personal keys (training knowledge, **UNVERIFIED**). So P03 to P05 each mint `user_id` + `team_id` + `max_budget` + `models`, and exhaust **one dimension at a time**. The dimension under test is at 1e-9 and the other two are at US$1,000, which one probe call cannot reach. So a budget refusal can only come from the dimension under test.

| Id | Question | PASS when | FAIL routes to |
|---|---|---|---|
| **P01** | Does `/key/generate` accept a caller-chosen `key`? | the reply's `key` equals the chosen `sk-pa-…` value | task 07's written fallback: read the generated key from the reply and bind after the call |
| **P02** | Does `duration` expire a key? | a `60s` key answers `200` at once and is refused (`401`/`403`/`400`) at 70 s | task 07 revokes at expiry itself; the gatekeeper's `token_expired` (task 10) is already the enforcement |
| **P03** | Does a user `max_budget` bite on a key that also belongs to a team? | a production-shaped key (`user_id` + `team_id` + a US$1,000 key `max_budget`) whose user is at 1e-9, and whose team is at US$1,000, is refused once spend is recorded | P6-D8's person dimension moves to keysmith's slice, as the team's already is |
| **P04** | Does a team `max_budget` bite on a key that also belongs to a user? | as P03, with the team at 1e-9 and the user at US$1,000 | P6-D8's project dimension moves to keysmith |
| **P05** | Does a key `max_budget` bite on a key with a user and a team? | as P03, with the key at 1e-9 and the user and team at US$1,000 | P6-D8's per-key slice becomes advisory; tasks 06 and 09 state a larger overrun bound |
| **P06** | Is prompt or response text stored? | after a canary call, the canary and the fake's reply text appear **nowhere** in a full `pg_dump` of the gateway database and in no line of LiteLLM's stdout, **and** the call's spend row exists | **blocker for OD-19**, raised to the owner; task 02's ZDR keys are wrong for this image |
| **P07** | Which path reaches the upstream for each `api_base` form? | exactly the form whose recorded path is `/v1/messages` is named; both is also a PASS | task 02's `apiBaseForm`; `host` needs this result as its `apiBaseEvidence` (`…-litellm-probe.md#p07`) |
| **P08** | Can a body `metadata.tags` or an `x-litellm-tags` header change attribution? | neither spoofed tag appears on the spend row, and its user and team are the key's | task 11 also drops `metadata.tags` from the body (the header is already dropped) |
| **P09** | Are SSE pings relayed unbuffered? | at least one `ping` arrives, and the first arrives ≥ 1.5 s before `message_stop` | the gatekeeper cannot fix LiteLLM's buffering; raised as open risk, with long-think sessions at risk of the CLI's 300 s abort |
| **P10** | Does `drop_params: true` drop a field the provider does not take? | on the `drop` model the OpenAI-only `frequency_penalty` does not reach the fake | task 02's AU group loses its safety net; P6-D3 then rests on `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` alone |
| **P11** | Is `/key/delete` effective at once? | the first call after the delete is refused | none needed: the gatekeeper's binding revocation (≤ 10 s) enforces; the delay measured is recorded for task 07 |
| **P12** | Does `/key/info` report per-key spend? | `info.spend > 0` within 60 s of a call | task 09's sweep needs another source (spend logs by key hash) |
| **P13** | Does the Global path forward what the CLI sends, unchanged? | a top-level `zz_future_field`, `thinking`, and the `anthropic-beta` header all reach the fake from a model **without** `drop_params` | task 02's Global group, and the gatekeeper's docs-mandated pass-through (task 11) is undone downstream; a blocker for OD-20 raised to the owner |
| **P14** | Can a body `fallbacks` on an `au/` group reach the Global upstream? | a call to `au/claude-opus-4-8` carrying `"fallbacks": ["global/claude-opus-4-8"]` and `"mock_testing_fallbacks": true`, made directly to LiteLLM with two production-shaped keys (an AU-minted key naming only `au/*`, and a Global-minted key naming both groups, which is what a raise leaves live), adds **no** request to `fakeanthropic-global` for either key, and adds none to `fakeanthropic-au` (so the mocked failure did fire). Both keys must mint (a key that did not is a FAIL) and both calls must reach the router (a status of 0, 401 or 403 is UNKNOWN), and the stack must be in task 17's fake mode (probe.sh exits 2 otherwise), so "nothing at either fake" is never a request that went nowhere or to the real provider | **blocker for OD-19's AU route**, raised to the owner like P06. A FAIL on the both-groups key alone means LiteLLM honours a body fallback within a key's models: task 11's `ControlFields` is then the only guard on the request, and task 07's decision (h) bounds how long such a key lives after a raise. A FAIL on the AU-only key means LiteLLM ignores the key's model list on a fallback, so decision (d) protects nothing |
| **P15** | Does re-sending a user's or team's budget move its reset date? | create a user with `budget_duration: "30d"` and a budget, read `budget_reset_at` from `/user/info`, make the **same** `EnsureUser` call again (the one keysmith repeats every `EnsureMemo`, 10 minutes, task 07), read it again: PASS is unchanged | keysmith ensures a person or team at most **once per period** — `memoKey` gains the period, so the repeat stops — or P6-D8's person and project dimensions drop their native budget and move to keysmith's slice (P03's fallback). Without one of those a window that keeps sliding never elapses, LiteLLM's `spend` never zeroes, and a person refused once is refused for the rest of the engagement while the cockpit reads US$0.00 (review M-7) |
| **P16** | Does the pinned image accept a calendar-month budget window? | create a user with `budget_duration: "1mo"`, then read `budget_reset_at` from `/user/info`: PASS is accepted **and** a date at 00:00 UTC on the first of a month | the two clocks stand, and task 22's reset date for the person and the project comes from LiteLLM's own `budget_reset_at` rather than from keysmith's UTC month (seam **S6.12**). A PASS makes `keysmith.BudgetDuration` that value and there is one clock (review M-7) |

**Files:**
- `deploy/local/gateway/probe.sh` — the driver (bash) and one embedded Python program, run in three modes: `self-test` on the host, `client` inside the LiteLLM container, `verdict` on the host for the database probes (new)
- `docs/verification/2026-09-25-phase-6-litellm-probe.md` — the results, per digest, one heading per probe so task 02 can cite `#p07` (new)

**Interfaces:**
- Produces:
  - `bash deploy/local/gateway/probe.sh --self-test` → `PASS: probe self-test (<n> cases)`, no Docker needed
  - `bash deploy/local/gateway/probe.sh` → one line per probe, `P<nn> PASS|FAIL|UNKNOWN <evidence>`, in id order, plus a header line `digest <repo@sha256:…>`; exit 0 unless P06, P13 or P14 is `FAIL` (exit 3), or the stack is not up or not in task 17's fake mode (exit 2, checked through `/model/info` before any probe runs)
  - the results document, whose `#p07` anchor task 02's loader accepts as `apiBaseEvidence` (task 02, `models.yaml`'s `litellm.apiBaseForm`)
- Consumes: task 17's running stack (`docker compose` in `deploy/local/gateway`, the secrets, the internal network `panacea-app-gateway-backend`, where LiteLLM, Postgres and the fakes live and no sandbox is (task 17, review I-6), services `litellm`, `postgres`, `fakeanthropic-global`, and `fakeanthropic-au` for P14, `task-17.md:47`); task 16's `GET /_captured` record fields `seq`, `path`, `headerNames`, `bodyKeys`; task 02's rendered group names `global/claude-opus-4-8` (the public id the docs use as their example) and `au/claude-opus-4-8` (`task-02.md:103`); task 11's `proxy.ControlFields`, of which P14 sends two.
- Consumed by: task 02 (`apiBaseForm`, pricing), task 07 (P01, P02, P11, P14, P15, P16), task 09 (P12), task 11 (P08, P14), task 06 (P03 to P05, P15, P16), task 22 (P16, the reset date it prints), task 23 (the digest it runs against), task 25 (what stays **UNVERIFIED**).

**LiteLLM's admin API as this script calls it.** Paths and fields are LiteLLM's documented proxy API **as remembered**, not read for this plan; a wrong one shows as a `FAIL` or `UNKNOWN` with the status, which is itself the finding task 04 needs.

| Call | Body (fields used) | Read back |
|---|---|---|
| `POST /key/generate` | `key`, `key_alias`, `duration`, `models`, `user_id`, `team_id`, `max_budget` | `key` |
| `POST /key/delete` | `keys: [key]` | status |
| `GET /key/info?key=<key>` | none | `info.spend` |
| `POST /user/new` | `user_id`, `max_budget` | status |
| `POST /team/new` | `team_id`, `max_budget` | status |
| `POST /v1/messages` | Anthropic Messages body, the virtual key as `Authorization: Bearer` | status, SSE events |

The spend-log table is read as `"LiteLLM_SpendLogs"` with columns `"user"`, `team_id` and `request_tags` (LiteLLM's Prisma schema, remembered, **UNVERIFIED**). The script lists the table's columns first and reports `UNKNOWN (no spend-log table)` rather than guessing if the name is wrong.

- [ ] **Step 1: Write the verdict self-test first**

The verdicts are pure functions of what a probe observed, so they are tested without Docker. This is the head of the embedded program in `probe.sh`, written before any verdict exists:

```python
# ---- self-test: every verdict on canned observations -----------------------
def self_test():
    cases = [
        # (name, got, want). Each comment names the mutant it kills.
        # P01: LiteLLM silently choosing its own key is a FAIL, not a PASS.
        ("p01 chosen", v_p01(200, "sk-pa-x", "sk-pa-x")[0], "PASS"),
        ("p01 replaced", v_p01(200, "sk-pa-x", "sk-other")[0], "FAIL"),
        # P02: a key still accepted after expiry fails.
        ("p02 expires", v_p02(200, 401)[0], "PASS"),
        ("p02 lives on", v_p02(200, 200)[0], "FAIL"),
        # Budgets: an unpriced model is UNKNOWN (kills blaming the budget for a pricing gap).
        ("budget bites", v_budget(True, 0.004)[0], "PASS"),
        ("budget ignored", v_budget(False, 0.004)[0], "FAIL"),
        ("budget unpriced", v_budget(False, 0.0)[0], "UNKNOWN"),
        # P06: no spend row means nothing was logged at all (kills a vacuous PASS).
        ("p06 clean", v_p06(1, 0, 0, 0)[0], "PASS"),
        ("p06 no row", v_p06(0, 0, 0, 0)[0], "UNKNOWN"),
        ("p06 prompt stored", v_p06(1, 2, 0, 0)[0], "FAIL"),
        ("p06 reply stored", v_p06(1, 0, 1, 0)[0], "FAIL"),
        ("p06 stdout", v_p06(1, 0, 0, 1)[0], "FAIL"),
        # P07: the Panacea 405 (bare host posted verbatim) picks the full form.
        ("p07 panacea", v_p07("/v1/messages", "/")[2], "full"),
        ("p07 appends", v_p07("/v1/messages/v1/messages", "/v1/messages")[2], "host"),
        ("p07 neither", v_p07("/", "/")[0], "FAIL"),
        # P08: a spoofed tag or a changed owner is a FAIL.
        ("p08 clean", v_p08(["probe"], "u", "t", "u", "t")[0], "PASS"),
        ("p08 tag", v_p08(["spoof-body"], "u", "t", "u", "t")[0], "FAIL"),
        ("p08 user", v_p08([], "someone-else", "t", "u", "t")[0], "FAIL"),
        ("p08 no row", v_p08(None, None, None, "u", "t")[0], "UNKNOWN"),
        # P09: every event inside 5 ms is buffering (kills "pings present" as the test).
        ("p09 spaced", v_p09([("ping", 1.0), ("ping", 2.0), ("message_stop", 3.1)])[0], "PASS"),
        ("p09 buffered", v_p09([("ping", 3.000), ("ping", 3.002), ("message_stop", 3.004)])[0], "FAIL"),
        ("p09 no pings", v_p09([("content_block_delta", 3.0), ("message_stop", 3.1)])[0], "FAIL"),
        # P10, P11, P12, P13.
        ("p10 dropped", v_p10(["max_tokens", "messages", "model"])[0], "PASS"),
        ("p10 kept", v_p10(["frequency_penalty", "model"])[0], "FAIL"),
        ("p11 at once", v_p11(True, None)[0], "PASS"),
        ("p11 late", v_p11(False, 3.5)[0], "FAIL"),
        ("p12 spend", v_p12(0.002)[0], "PASS"),
        ("p12 unpriced", v_p12(0.0)[0], "UNKNOWN"),
        ("p13 intact", v_p13(["thinking", "zz_future_field"], ["Anthropic-Beta"])[0], "PASS"),
        ("p13 beta lost", v_p13(["thinking", "zz_future_field"], ["Content-Type"])[0], "FAIL"),
        # P14: any request at the global fake is a FAIL, from either key (kills
        # judging one key only); a primary that reached the au fake means the
        # mocked failure never fired, so nothing was asked (kills a vacuous PASS).
        ("p14 held", v_p14([("au-only", 0, 0, 200), ("both", 0, 0, 200)])[0], "PASS"),
        ("p14 both leaks", v_p14([("au-only", 0, 0, 200), ("both", 1, 0, 200)])[0], "FAIL"),
        ("p14 au-only leaks", v_p14([("au-only", 1, 0, 200), ("both", 0, 0, 400)])[0], "FAIL"),
        ("p14 never fired", v_p14([("au-only", 0, 1, 200), ("both", 0, 0, 200)])[0], "UNKNOWN"),
        # A refused or absent key sends nothing to either fake (kills a PASS that
        # never looks at the status, and a mint failure read as "held"; re-review R-2).
        ("p14 unauthorised", v_p14([("au-only", 0, 0, 401), ("both", 0, 0, 200)])[0], "UNKNOWN"),
        ("p14 no key", v_p14([("au-only", 0, 0, None), ("both", 0, 0, 200)])[0], "FAIL"),
        # probe.sh runs nothing unless both groups go to their fakes (kills a P14
        # PASS, and real provider calls, in task 17's anthropic mode; re-review R-2).
        ("upstreams fake", v_upstreams([("global/claude-opus-4-8", "http://fakeanthropic-global:4900/v1/messages"),
                                        ("au/claude-opus-4-8", "http://fakeanthropic-au:4901")]), None),
        ("upstreams real global", v_upstreams([("global/claude-opus-4-8", "https://api.anthropic.com/v1/messages"),
                                               ("au/claude-opus-4-8", "http://fakeanthropic-au:4901")]) is None, False),
        ("upstreams no api_base", v_upstreams([("global/claude-opus-4-8", None),
                                               ("au/claude-opus-4-8", "http://fakeanthropic-au:4901")]) is None, False),
        # Evidence never carries a key (kills printing a reply that echoes one).
        ("redact", "sk-pa-abc123" in redact('{"key":"sk-pa-abc123"}'), False),
    ]
    bad = [(n, g, w) for n, g, w in cases if g != w]
    for n, g, w in bad:
        print(f"FAIL: {n}: got {g!r}, want {w!r}")
    if bad:
        sys.exit(1)
    print(f"PASS: probe self-test ({len(cases)} cases)")
```

- [ ] **Step 2: Run the self-test to verify it fails**

Write `probe.sh` as step 5's bash driver with only this `self_test` and step 4's entry point inside `emit_py`, then:

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/local/gateway/probe.sh --self-test'
```

Expected: exit 1 with `NameError: name 'v_p01' is not defined`.

- [ ] **Step 3: Write the helpers and the verdicts**

These go **above** `self_test` in the embedded program. Every verdict returns `(verdict, evidence)`; `v_p07` adds the form task 02 records.

```python
import json, os, re, secrets, sys, time, urllib.error, urllib.parse, urllib.request

def redact(s):
    """Evidence only: no sk- key survives, and nothing longer than 160 chars."""
    if isinstance(s, bytes):
        s = s.decode("utf-8", "replace")
    return re.sub(r"sk-[A-Za-z0-9_\-]+", "sk-<redacted>", s)[:160]

def v_p01(status, chosen, got):
    if status != 200:
        return "FAIL", f"/key/generate answered {status}"
    if got == chosen:
        return "PASS", "reply key equals the chosen value"
    return "FAIL", "LiteLLM replaced the chosen key: task 07's fallback applies"

def v_p02(first, after):
    if first != 200:
        return "UNKNOWN", f"the fresh 60s key was refused at once ({first})"
    if after in (400, 401, 403):
        return "PASS", f"accepted at 0 s, refused {after} at 70 s"
    return "FAIL", f"still answered {after} at 70 s"

def v_budget(refused_for_budget, spend):
    if refused_for_budget:
        return "PASS", f"refused for budget after spend {spend:.6f}"
    if not spend:
        return "UNKNOWN", "spend stayed 0: LiteLLM did not read task 02's model_info prices"
    return "FAIL", f"spend {spend:.6f} over a 1e-9 budget and still served"

def v_p06(rows, prompt_hits, reply_hits, log_hits):
    if rows == 0:
        return "UNKNOWN", "no spend row for the canary call: nothing to judge"
    if prompt_hits or reply_hits or log_hits:
        return "FAIL", (f"canary in db {prompt_hits}x, reply text in db {reply_hits}x, "
                        f"canary in stdout {log_hits}x: BLOCKER for OD-19")
    return "PASS", "spend row present; canary and reply text in no table and no log line"

def v_p07(full_path, host_path):
    ok_full, ok_host = full_path == "/v1/messages", host_path == "/v1/messages"
    ev = f"full form reached {full_path!r}, host form reached {host_path!r}"
    if ok_full:
        return "PASS", ev, "full"          # full wins a tie: no evidence needed
    if ok_host:
        return "PASS", ev, "host"
    return "FAIL", ev, None

def v_p08(tags, user, team, want_user, want_team):
    if tags is None:
        return "UNKNOWN", "no spend row for the spoof call"
    spoofed = [t for t in tags if t.startswith("spoof")]
    if spoofed or user != want_user or team != want_team:
        return "FAIL", f"tags {spoofed}, user {user!r}, team {team!r}: task 11 drops metadata.tags"
    return "PASS", "no spoofed tag; user and team are the key's"

def v_p09(events):
    pings = [t for n, t in events if n == "ping"]
    stops = [t for n, t in events if n == "message_stop"]
    if not stops:
        return "UNKNOWN", f"stream ended without message_stop ({len(events)} events)"
    if not pings:
        return "FAIL", "no ping relayed: long thinking pauses look silent to the CLI"
    lead = stops[0] - pings[0]
    if lead >= 1.5:
        return "PASS", f"{len(pings)} pings; first ping {lead:.2f}s before message_stop"
    return "FAIL", f"buffered: first ping only {lead * 1000:.0f} ms before message_stop"

def v_p10(body_keys):
    if body_keys is None:
        return "UNKNOWN", "the drop model's request never reached the fake"
    if "frequency_penalty" in body_keys:
        return "FAIL", "frequency_penalty reached the upstream despite drop_params"
    return "PASS", "frequency_penalty dropped"

def v_p11(refused_at_once, refused_after_s):
    if refused_at_once:
        return "PASS", "first call after /key/delete refused"
    if refused_after_s is not None:
        return "FAIL", f"refused only after {refused_after_s:.1f}s (LiteLLM's key cache)"
    return "FAIL", "still accepted 90 s after /key/delete"

def v_p12(spend):
    if spend is None:
        return "FAIL", "/key/info carried no info.spend"
    if spend > 0:
        return "PASS", f"info.spend {spend:.6f}"
    return "UNKNOWN", "info.spend is 0: the model is unpriced"

def v_p13(body_keys, header_names):
    if body_keys is None:
        return "UNKNOWN", "the keep model's request never reached the fake"
    missing = [k for k in ("thinking", "zz_future_field") if k not in body_keys]
    if "Anthropic-Beta" not in (header_names or []):
        missing.append("anthropic-beta header")
    if missing:
        return "FAIL", f"lost on the way upstream: {missing}: BLOCKER for OD-20"
    return "PASS", "unknown body field, thinking and anthropic-beta all forwarded"

def v_p14(results):
    """results: (key shape, new global-fake records, new au-fake records, status).
    A status of None means the key never minted (re-review R-2)."""
    leaked = [shape for shape, g, _, _ in results if g]
    if leaked:
        return "FAIL", (f"a body fallback on au/ reached the global upstream for {leaked}: "
                        "BLOCKER for OD-19's AU route")
    unminted = [shape for shape, _, _, st in results if st is None]
    if unminted:
        return "FAIL", f"could not mint the P14 key for {unminted}: BLOCKER for OD-19's AU route"
    # 0 is no answer at all, 401 and 403 a key LiteLLM refused: the request never
    # reached the router, so "nothing at either fake" measured nothing.
    refused = [(shape, st) for shape, _, _, st in results if st in (0, 401, 403)]
    if refused:
        return "UNKNOWN", f"the P14 request never reached the router: {refused}"
    unfired = [shape for shape, _, a, _ in results if a]
    if unfired:
        return "UNKNOWN", f"mock_testing_fallbacks ignored, the au primary answered for {unfired}"
    return "PASS", "; ".join(f"{shape}: status {st}, nothing at either fake" for shape, _, _, st in results)

def v_upstreams(rows):
    """rows: (model_name, api_base) pairs from LiteLLM's /model/info. None when every
    global/ group goes to fakeanthropic-global and every au/ group to fakeanthropic-au,
    else why not. Task 17's anthropic mode points global/ at the real provider: a
    leaked P14 fallback would reach it, record nothing at the fake, and pass, and
    every probe would be a real provider call (re-review R-2)."""
    want = {"global/": "fakeanthropic-global", "au/": "fakeanthropic-au"}
    seen = set()
    for name, base in rows:
        prefix = next((p for p in want if (name or "").startswith(p)), None)
        host = urllib.parse.urlparse(base or "").hostname
        if prefix is None or host != want[prefix]:
            return f"{name} goes to {host or 'no api_base'}, not its fake"
        seen.add(prefix)
    if seen != set(want):
        return f"groups found {sorted(seen)}, want both global/ and au/"
    return None
```

- [ ] **Step 4: Write the client, the half that runs inside the LiteLLM container**

Below the verdicts. `client(run, canary)` prints one `P<nn>` line per probe it owns and a `DATA` line for each probe the host finishes (P06, P08).

```python
LL = "http://127.0.0.1:4001"                 # the stack's LiteLLM, from inside its container
TMP = "http://gw-probe-litellm:4011"         # the throwaway LiteLLM (P07, P10, P13)
FAKE = "http://fakeanthropic-global:4900"
FAKE_AU = "http://fakeanthropic-au:4901"      # P14 only
MODEL = "global/claude-opus-4-8"
AU_MODEL = "au/claude-opus-4-8"
ROOMY = 1000.0                               # US$: a budget one probe call cannot reach

def call(method, url, key=None, body=None, headers=None, timeout=30):
    req = urllib.request.Request(url, method=method,
                                 data=None if body is None else json.dumps(body).encode())
    req.add_header("content-type", "application/json")
    req.add_header("anthropic-version", "2023-06-01")
    if key:
        req.add_header("authorization", "Bearer " + key)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except OSError as e:
        return 0, str(e).encode()

def msg(model, text, **extra):
    return dict({"model": model, "max_tokens": 16,
                 "messages": [{"role": "user", "content": text}]}, **extra)

def key(master, **fields):
    fields.setdefault("duration", "10m")
    st, b = call("POST", LL + "/key/generate", master, fields)
    return st, (json.loads(b).get("key") if st == 200 else None)

def spend_of(master, k):
    st, b = call("GET", LL + "/key/info?key=" + urllib.request.quote(k), master)
    if st != 200:
        return None
    return float((json.loads(b).get("info") or {}).get("spend") or 0)

def fake_seq(fake=FAKE):
    _, b = call("GET", fake + "/_captured")
    reqs = json.loads(b).get("requests") or []
    return reqs[-1]["seq"] if reqs else 0

def fake_after(seq, fake=FAKE):
    _, b = call("GET", fake + "/_captured")
    return [r for r in json.loads(b).get("requests") or [] if r["seq"] > seq]

def prod_key(master, user, team, models, key_usd=ROOMY):
    """keysmith's shape (task 07, mint): user_id AND team_id AND a key max_budget."""
    return key(master, user_id=user, team_id=team, max_budget=key_usd, models=models)[1]

def budget_probe(pid, master, run, user_usd, team_usd, key_usd):
    """One production-shaped key; exactly one of the three budgets is 1e-9 (review
    I-8). Spend is recorded asynchronously, so poll 60 s."""
    u, t = f"probe-{run}-{pid.lower()}", f"probe-{run}-{pid.lower()}-team"
    call("POST", LL + "/user/new", master, {"user_id": u, "max_budget": user_usd})
    call("POST", LL + "/team/new", master, {"team_id": t, "max_budget": team_usd})
    k = prod_key(master, u, t, [MODEL], key_usd)
    if not k:
        return say(pid, "FAIL", "could not mint the probe key")
    deadline, refused, spend = time.time() + 60, False, 0.0
    while time.time() < deadline:
        st, b = call("POST", LL + "/v1/messages", k, msg(MODEL, f"{pid} {run}"))
        if st in (400, 401, 403, 429) and b"budget" in b.lower():
            refused = True
            break
        spend = spend_of(master, k) or 0.0
        time.sleep(3)
    say(pid, *v_budget(refused, spend_of(master, k) or spend))

def say(pid, verdict, evidence, *_):
    print(f"{pid} {verdict} {redact(evidence)}", flush=True)
```

The probes themselves, in the same program:

```python
def sse_events(url, k, body):
    req = urllib.request.Request(url, method="POST", data=json.dumps(body).encode())
    for h, v in (("content-type", "application/json"), ("anthropic-version", "2023-06-01"),
                 ("authorization", "Bearer " + k)):
        req.add_header(h, v)
    out, t0 = [], time.monotonic()
    with urllib.request.urlopen(req, timeout=60) as r:
        for line in r:                        # readline returns as each line arrives
            if line.startswith(b"event:"):
                out.append((line[6:].strip().decode(), time.monotonic() - t0))
    return out

def client(run, canary, tmp_master):
    master = open("/run/secrets/master_key").read().strip()
    u = lambda s: f"probe-{run}-{s}"
    # P02 starts first so its 70 s wait overlaps the rest.
    _, k02 = key(master, duration="60s", key_alias=u("p02"), models=[MODEL])
    t02, first02 = time.time(), call("POST", LL + "/v1/messages", k02, msg(MODEL, "p02"))[0]
    chosen = "sk-pa-" + secrets.token_urlsafe(32)
    st, got = key(master, key=chosen, key_alias=u("p01"), models=[MODEL])
    say("P01", *v_p01(st, chosen, got))
    # P03 to P05: one dimension exhausted at a time, on keysmith's key shape.
    budget_probe("P03", master, run, user_usd=1e-9, team_usd=ROOMY, key_usd=ROOMY)
    budget_probe("P04", master, run, user_usd=ROOMY, team_usd=1e-9, key_usd=ROOMY)
    budget_probe("P05", master, run, user_usd=ROOMY, team_usd=ROOMY, key_usd=1e-9)
    # P14: a body fallback from au/ to global/, on an AU-minted and a Global-minted key.
    call("POST", LL + "/user/new", master, {"user_id": u("p14"), "max_budget": ROOMY})
    call("POST", LL + "/team/new", master, {"team_id": u("t14"), "max_budget": ROOMY})
    p14 = []
    for shape, models in (("au-only", [AU_MODEL]), ("both", [AU_MODEL, MODEL])):
        k14 = prod_key(master, u("p14"), u("t14"), models)
        if not k14:                           # v_p14 reads None as "never minted": FAIL
            p14.append((shape, 0, 0, None))
            continue
        g0, a0 = fake_seq(FAKE), fake_seq(FAKE_AU)
        st = call("POST", LL + "/v1/messages", k14, msg(AU_MODEL, "p14",
                  fallbacks=[MODEL], mock_testing_fallbacks=True))[0]
        time.sleep(1)                         # the fakes record on arrival; one second is slack
        p14.append((shape, len(fake_after(g0, FAKE)), len(fake_after(a0, FAKE_AU)), st))
    say("P14", *v_p14(p14))
    # P06 and P08: the calls here, the verdicts on the host after the spend rows land.
    _, k06 = key(master, user_id=u("p06"), models=[MODEL])
    call("POST", LL + "/v1/messages", k06, msg(MODEL, f"say {canary}"))
    print(f"DATA P06 user={u('p06')}", flush=True)
    _, k08 = key(master, user_id=u("p08"), team_id=u("t08"), models=[MODEL])
    call("POST", LL + "/v1/messages", k08,
         msg(MODEL, "p08", metadata={"tags": ["spoof-body"], "user_id": "someone-else"}),
         headers={"x-litellm-tags": "spoof-header"})
    print(f"DATA P08 user={u('p08')} team={u('t08')}", flush=True)
    # P09, with a control against the fake itself: a client that buffers makes it UNKNOWN.
    control = v_p09(sse_events(FAKE + "/v1/messages", "x", msg("claude-opus-4-8", "c", stream=True)))
    _, k09 = key(master, models=[MODEL])
    p09 = v_p09(sse_events(LL + "/v1/messages", k09, msg(MODEL, "p09", stream=True)))
    say("P09", *(p09 if control[0] == "PASS" else ("UNKNOWN", "control failed: " + control[1])))
    # P11: delete, then poll.
    _, k11 = key(master, models=[MODEL])
    call("POST", LL + "/v1/messages", k11, msg(MODEL, "p11"))
    call("POST", LL + "/key/delete", master, {"keys": [k11]})
    t11, at_once, after = time.time(), None, None
    while time.time() - t11 < 90:
        st = call("POST", LL + "/v1/messages", k11, msg(MODEL, "p11"))[0]
        if at_once is None:
            at_once = st != 200
        if st != 200:
            after = time.time() - t11
            break
        time.sleep(0.5)
    say("P11", *v_p11(bool(at_once), after))
    # P12: per-key spend, polled for 60 s.
    _, k12 = key(master, models=[MODEL])
    call("POST", LL + "/v1/messages", k12, msg(MODEL, "p12"))
    t12, s12 = time.time(), 0.0
    while time.time() - t12 < 60 and not s12:
        time.sleep(3)
        s12 = spend_of(master, k12)
    say("P12", *v_p12(s12))
    tmp_probes(tmp_master)
    time.sleep(max(0, 70 - (time.time() - t02)))
    say("P02", *v_p02(first02, call("POST", LL + "/v1/messages", k02, msg(MODEL, "p02"))[0]))
```

The throwaway LiteLLM's three probes, and the entry point, close the program. The fake records a request it refuses too (task 16), with its body keys and header names, so P10 and P13 are judged even when P07 shows a form that misses `/v1/messages`.

```python
def tmp_probes(tmp_master):
    def via(model, **extra):
        seq = fake_seq()
        call("POST", TMP + "/v1/messages", tmp_master, msg(model, "tmp", **extra),
             headers={"anthropic-beta": "probe-future-beta-2027"})
        after = fake_after(seq)
        return after[-1] if after else None
    full, host = via("p07-full"), via("p07-host")
    v = v_p07(full and full["path"], host and host["path"])
    say("P07", v[0], f"{v[1]}; task 02 apiBaseForm: {v[2]}")
    r10 = via("p10-drop", frequency_penalty=0.5)
    say("P10", *v_p10(r10 and r10["bodyKeys"]))
    r13 = via("p13-keep", max_tokens=2048, zz_future_field={"x": 1},
              thinking={"type": "enabled", "budget_tokens": 1024})
    say("P13", *v_p13(r13 and r13["bodyKeys"], r13 and r13["headerNames"]))

def parse_tags(s):
    s = (s or "").strip()
    if s.startswith("["):
        return json.loads(s)
    return [t.strip('"') for t in s.strip("{}").split(",") if t]

if __name__ == "__main__":
    mode = sys.argv[1]
    if mode == "self-test":
        self_test()
    elif mode == "upstreams":                 # probe.sh runs this first; not "OK" means exit 2
        master = open("/run/secrets/master_key").read().strip()
        st, b = call("GET", LL + "/model/info", master)
        rows = [] if st != 200 else [(m.get("model_name"), (m.get("litellm_params") or {}).get("api_base"))
                                     for m in json.loads(b).get("data") or []]
        print("OK" if st == 200 and not v_upstreams(rows) else f"status {st}: {v_upstreams(rows)}")
    elif mode == "client":
        client(sys.argv[2], sys.argv[3], sys.argv[4])
    elif mode == "verdict-p06":
        say("P06", *v_p06(*map(int, sys.argv[2:6])))
    elif mode == "verdict-p08":
        row, want_user, want_team = sys.argv[2], sys.argv[3], sys.argv[4]
        if not row:
            say("P08", *v_p08(None, None, None, want_user, want_team))
        else:
            tags, user, team = row.split("|", 2)
            say("P08", *v_p08(parse_tags(tags), user, team, want_user, want_team))
    else:
        sys.exit(f"probe: unknown mode {mode!r}")
```

The throwaway LiteLLM's config, emitted by the driver (no secret: its master key is a per-run throwaway, and it has no database):

```yaml
model_list:
  - model_name: p07-full
    litellm_params: { model: anthropic/claude-opus-4-8, api_key: fake-local, api_base: "http://fakeanthropic-global:4900/v1/messages" }
  - model_name: p07-host
    litellm_params: { model: anthropic/claude-opus-4-8, api_key: fake-local, api_base: "http://fakeanthropic-global:4900" }
  - model_name: p10-drop
    litellm_params: { model: anthropic/claude-opus-4-8, api_key: fake-local, api_base: "http://fakeanthropic-global:4900/v1/messages", drop_params: true }
  - model_name: p13-keep
    litellm_params: { model: anthropic/claude-opus-4-8, api_key: fake-local, api_base: "http://fakeanthropic-global:4900/v1/messages" }
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
```

- [ ] **Step 5: Write the driver around the program**

`deploy/local/gateway/probe.sh`, complete except for the two emitted bodies above (`emit_py` holds the Python program of steps 1, 3 and 4 in the order: imports and verdicts, `self_test`, client, `tmp_probes`, entry point; `tmp_config` holds the YAML). The heredocs live in the file, so they never cross the `claudezsh` boundary.

```bash
#!/usr/bin/env bash
# probe.sh [--self-test]: what the pinned LiteLLM actually does (phase 6, task 18).
# Runs against task 17's stack; prints "P<nn> PASS|FAIL|UNKNOWN <evidence>".
# Never prints a key or a body. Exit 0; 2 if the stack is not up or not in fake mode;
# 3 if P06, P13 or P14 FAILs.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
cd "$here"

emit_py() { cat <<'PY'
# (the program from steps 1, 3 and 4)
PY
}
tmp_config() { cat <<'YAML'
# (the throwaway config above)
YAML
}

if [ "${1:-}" = "--self-test" ]; then emit_py | python3 - self-test; exit; fi

ll="$(docker compose ps -q litellm 2>/dev/null || true)"
if [ -z "$ll" ] || [ "$(docker inspect -f '{{.State.Health.Status}}' "$ll")" != healthy ]; then
  echo "probe: task 17's stack is not up; run docker compose up -d --wait" >&2
  exit 2
fi
# Fake mode only (re-review R-2). In anthropic mode global/ is the real provider:
# every probe would be a real call, and a leaked P14 fallback would pass unseen.
up="$(emit_py | docker compose exec -T litellm python3 - upstreams 2>/dev/null || true)"
if [ "$up" != OK ]; then
  echo "probe: needs PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=fake; /model/info says: ${up:-no answer}" >&2
  exit 2
fi
img="$(docker inspect -f '{{.Config.Image}}' "$ll")"
echo "digest $(docker image inspect -f '{{index .RepoDigests 0}}' "$img")"

rnd() { head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'; }
run="$(rnd 6)"
canary="canary-$(rnd 12)"
tmpkey="sk-probe-throwaway-$(rnd 16)"
out="$(mktemp)"
trap 'docker rm -f gw-probe-litellm >/dev/null 2>&1 || true; rm -f "$out"' EXIT
pq() { docker compose exec -T postgres psql -U gateway -d gateway -At "$@"; }

# The throwaway LiteLLM: same digest, no database, config passed in the environment.
docker rm -f gw-probe-litellm >/dev/null 2>&1 || true
docker run -d --name gw-probe-litellm --network panacea-app-gateway-backend \
  -e LITELLM_MASTER_KEY="$tmpkey" -e PROBE_CFG="$(tmp_config | base64 | tr -d '\n')" \
  --entrypoint /bin/sh "$img" -c 'python3 -c "import base64,os; open(\"/tmp/c.yaml\",\"wb\").write(base64.b64decode(os.environ[\"PROBE_CFG\"]))" && exec litellm --config /tmp/c.yaml --port 4011' >/dev/null
for _ in $(seq 1 40); do
  docker exec gw-probe-litellm python3 -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:4011/health/readiness', timeout=2)" >/dev/null 2>&1 && break
  sleep 3
done

cols="$(pq -c "select string_agg(column_name, ',') from information_schema.columns where table_name = 'LiteLLM_SpendLogs'")"
echo "spendlog-columns ${cols:-<none>}"

emit_py | docker compose exec -T litellm python3 - client "$run" "$canary" "$tmpkey" | tee "$out"

# P06: wait for the canary call's spend row, then search everything LiteLLM keeps.
u06="probe-$run-p06"
rows=0
for _ in $(seq 1 30); do
  rows="$(pq -c "select count(*) from \"LiteLLM_SpendLogs\" where \"user\" = '$u06'" 2>/dev/null || echo 0)"
  [ "${rows:-0}" -gt 0 ] && break
  sleep 3
done
prompt_hits="$(docker compose exec -T postgres pg_dump -U gateway gateway | grep -c -F -- "$canary" || true)"
reply_hits="$(docker compose exec -T postgres pg_dump -U gateway gateway | grep -c -F -- 'fake global reply' || true)"
log_hits="$({ docker compose logs --no-color litellm 2>&1; docker logs gw-probe-litellm 2>&1; } | grep -c -F -- "$canary" || true)"
emit_py | python3 - verdict-p06 "${rows:-0}" "$prompt_hits" "$reply_hits" "$log_hits" | tee -a "$out"

# P08: the spoof call's spend row: tags, user, team.
u08="probe-$run-p08"
t08="probe-$run-t08"
row=""
for _ in $(seq 1 30); do
  row="$(pq -c "select request_tags::text, \"user\", team_id from \"LiteLLM_SpendLogs\" where \"user\" = '$u08' limit 1" 2>/dev/null || true)"
  [ -n "$row" ] && break
  sleep 3
done
emit_py | python3 - verdict-p08 "$row" "$u08" "$t08" | tee -a "$out"

echo "---- summary"
grep -E '^P[0-9]{2} ' "$out" | sort
if grep -qE '^P(06|13|14) FAIL' "$out"; then exit 3; fi
```

A row count of `0` because the table name is wrong would make P06 `UNKNOWN`, never `PASS` (the self-test's `p06 no row` case), and `spendlog-columns <none>` in the output says why.

- [ ] **Step 6: Run the self-test to verify it passes, and that it can fail**

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/local/gateway/probe.sh --self-test'
```

Expected: `PASS: probe self-test (40 cases)`. Then change `v_p06`'s first test to `if rows < 0:` and re-run: it must fail with `FAIL: p06 no row: got 'PASS', want 'UNKNOWN'`. Restore it. A P06 that passes with no spend row would declare zero retention on a LiteLLM that logged nothing at all. Do the same for P14: change `v_p14`'s `if unfired:` to `if False:` and re-run. It must fail with `FAIL: p14 never fired: got 'PASS', want 'UNKNOWN'`. Restore it. A P14 that passes when LiteLLM ignored `mock_testing_fallbacks` would clear the AU route on a fallback that never ran. Then change `v_p14`'s `if refused:` to `if False:`: it must fail with `FAIL: p14 unauthorised: got 'PASS', want 'UNKNOWN'`. Restore it. Then change `v_upstreams`'s `host != want[prefix]` to `False`: it must fail with `FAIL: upstreams real global: got True, want False`. Restore it. The first would clear the AU route on a request LiteLLM refused before routing; the second would run every probe against the real provider (re-review R-2).

- [ ] **Step 7: Run the probe against the stack**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd deploy/local/gateway && docker compose up -d --wait && bash probe.sh'
```

Run it with the Bash tool's timeout at 600000 ms: P02 alone waits 70 s, and each budget probe and P11 poll for up to 60 or 90 s. Expected shape (the verdicts are what this task exists to find out, so none is predicted):

```text
digest ghcr.io/berriai/litellm@sha256:<64 hex>
spendlog-columns request_id,call_type,api_key,spend,...
P01 <verdict> <evidence>
...
---- summary
P01 <verdict> <evidence>
...
P14 <verdict> <evidence>
```

Fourteen summary lines, one per probe. A probe line that is missing means the client died; its traceback is on stderr and is fixed before the results are recorded.

- [ ] **Step 8: Record the results**

`docs/verification/2026-09-25-phase-6-litellm-probe.md`. The template, complete; the executor fills every `<…>` from step 7's output, verbatim, and never from memory:

```markdown
# Phase 6 LiteLLM probe, 2026-09-25

Task 18 of `docs/superpowers/plans/2026-09-25-phase-6-gateway.md`. Run with
`bash deploy/local/gateway/probe.sh` against task 17's local stack, on the fake
upstream (zero spend). Results hold for this digest only; a new digest is a new run.

- **Image:** `<digest line>`
- **Tag chosen in task 17:** `<tag>`
- **Spend-log columns:** `<spendlog-columns line>`
- **Run by / date:** `<who>`, `<ISO date>`

| Id | Verdict | Evidence | Consequence |
|---|---|---|---|
| P01 | <v> | <evidence> | <"none" or the fallback adopted, with the part it changes> |
| … one row per probe, P01 to P16 … |

## P06

<The three counts and the row count. PASS keeps OD-19's own-store half; FAIL stops the phase and is raised to the owner.>

## P07

<Both recorded paths. The form task 02's `models.yaml` records as `litellm.apiBaseForm`, and why. This heading is the anchor `#p07` that task 02 accepts as `apiBaseEvidence`.>

## P09

<The ping timings. A FAIL is raised as an open risk: the gatekeeper cannot repair buffering upstream of it.>

## P13

<What reached the fake. A FAIL is raised to the owner: OD-20's "tracks the latest CLI" depends on it.>

## P14

<For each key shape, the status and the new records at each fake. A FAIL is raised to the owner as a blocker for OD-19's AU route, naming the shape that leaked. An UNKNOWN (mock ignored) means the AU route rests on task 11's `ControlFields` with no measurement behind it; say so in the owner's summary.>

## Not measured here

Bedrock, Kata, AKS, a real Anthropic upstream, and LiteLLM's behaviour under load.
```

- [ ] **Step 9: Route each FAIL and UNKNOWN**

For each non-PASS row, the "Consequence" cell names the part and the change, from the probe table at the top of this task. Two of them edit a file this task does not own and are therefore **recorded here and made by that part's owner**: an `apiBaseForm: host` result (task 02's `models.yaml`, with `apiBaseEvidence` pointing at `#p07`), and an `UNKNOWN (unpriced)` on P03 to P05 or P12 (task 02 already renders a price into every group's `model_info`, its decision (f), so this result means the pinned LiteLLM reads prices from other keys: task 02 moves them where the evidence says, then this probe is re-run). A P06, P13 or P14 FAIL is written to the owner as a blocker in the words of the row, and task 23 does not start until the owner answers.

- [ ] **Step 10: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add deploy/local/gateway/probe.sh docs/verification/2026-09-25-phase-6-litellm-probe.md'
~/.claude/tools/claudezsh/claudezsh 'git commit -F /tmp/msg-task18.txt -- deploy/local/gateway/probe.sh docs/verification/2026-09-25-phase-6-litellm-probe.md'
```

with `/tmp/msg-task18.txt` holding (the last paragraph written from the actual results):

```text
test(gateway): probe what the pinned LiteLLM actually does

Fourteen probes against task 17's local stack, each PASS, FAIL or UNKNOWN
with evidence, recorded against the image digest: a caller-chosen key,
duration expiry, user, team and key budgets (each on keysmith's key shape,
one dimension exhausted at a time), no stored prompt or reply text, the
api_base path form (the Panacea 405, measured), spoofed tags, unbuffered
pings, drop_params, immediate key delete, per-key spend, pass-through of
unknown fields and anthropic-beta on the Global path, and whether a body
fallback on an au/ group can reach the Global upstream.

The client runs inside the LiteLLM container, so LiteLLM stays unpublished
and the master key never reaches a command line. Evidence is redacted of
every sk- string. A missing spend row makes P06 UNKNOWN, never PASS, and an
unpriced model makes a budget probe UNKNOWN, never FAIL.

Results: <one line per non-PASS probe and the part it changes>.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

**What this task does NOT claim:**

- **Anything about Bedrock.** Every call here reaches an Anthropic-format fake. `drop_params` (P10) is measured on LiteLLM's Anthropic provider only; whether the AU group's Anthropic-to-Bedrock translation carries what Claude Code sends is task 24's P04.
- **Anything about Kata or AKS**, including whether the gatekeeper sees a pod's own source address (task 24's P03).
- **Behaviour under load.** One call per probe. Budget overrun under concurrency is bounded on paper (tasks 06 and 09), not measured.
- **That a PASS holds for any other digest.** LiteLLM changes quickly; task 17's pin and this document move together.
- **That P06's PASS is the whole of ZDR.** It covers LiteLLM's database and stdout. The upstreams' retention terms are the org's contracts (owner question Q3), and the gatekeeper's and keysmith's logs are tasks 12 and 08's canary tests.
- **That the admin paths and spend-log names are right.** They are LiteLLM's API as remembered; a wrong one shows as `FAIL` or `UNKNOWN` with its status, which tells task 04 what to change, but this task does not correct task 04 itself.
- **That `/model/info` shows each group's `api_base`.** The fake-mode gate reads `litellm_params.api_base` from it (training knowledge of LiteLLM, **UNVERIFIED** against the pinned image). If the image hides the field, the gate fails closed: probe.sh exits 2 with `no api_base` on every run, and the fix is to read the same pairs from the container's `/app/config.yaml`, never to drop the gate.
- **That P14 covers every way out of Australia.** It sends two of task 11's twenty-four `ControlFields` (`fallbacks`, `mock_testing_fallbacks`) straight to LiteLLM, and forces the primary to fail with LiteLLM's own mock. It measures neither a real upstream failure, nor `context_window_fallbacks`, nor the other twenty-two names (the Bedrock credential fields included). The gatekeeper's drop of all twenty-four is task 11's unit tests, not a probe.
- **That P03 to P05 name the dimension that refused.** Each exhausts one budget and sets the other two out of reach, so the dimension is known by construction. LiteLLM's refusal text is only matched for the word `budget`.
- **That P09's timing is precise.** It measures on one host, inside Docker, with a 1 s ping gap; it tells buffering from relaying and nothing finer.
