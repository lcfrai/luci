### Task 23: Local end-to-end proof with unmodified Claude Code

Every earlier task proves its piece against fakes of its neighbours. This task is the one place where the **real, unmodified `claude` CLI** in the sandbox image talks to the real gatekeeper, keysmith and LiteLLM, on Docker Desktop, with zero cloud spend. It writes no code. It produces one record, `docs/verification/2026-09-25-phase-6-local-e2e.md`, in which each of eight items is **observed** or **not observed**, with the exact command and its output, and nothing is written from memory or expectation.

**The one thing this task must not get wrong is the difference between "passed" and "observed".** A screenshot is not evidence (`CLAUDE.md`, working agreements), and neither is a green test elsewhere. An item is *observed* only when the record holds the command that was run and what it printed. An item that could not be run is *not observed*, with the reason, and the phase closes with it listed (task 25). A result that contradicts an earlier task's assumption is recorded as found, with the task it breaks, and is **not** fixed inside this task.

**Files:**
- `docs/verification/2026-09-25-phase-6-local-e2e.md`: the record (new)

**Interfaces:**
- Consumes: task 15 (the local control plane mints beside start and revokes on delete), task 17 (the compose stack, `init.sh`, the published ports `127.0.0.1:4000` and `127.0.0.1:4100`), task 18 (the probe results, which say which LiteLLM behaviours this run may rely on), task 21 (the `PATCH` route), task 22 (the chip), task 14 (`panacea token` and the settings fragment in the sandbox image), task 16 (the fakes' `x-fake-route` label and `/_captured`)
- Produces: the record; a list of findings for task 25
- Required by: task 25 (the phase's "what stays unverified" list reads this record)

**Decisions where the skeleton is silent:**

**(a) Built binaries, not `go run`.** The control plane runs as a binary built from the branch. A `go run` in the background leaves its child running when the parent is stopped, and a stale control plane from an earlier attempt then answers on `:8730` with yesterday's code. Step 2 checks that nothing is on the port first.

**(b) Every item names the session, and the order is fixed.** One session, `S1` on project P1, carries items 1, 2, 3, 4, 7, 8 and 9; item 6 then deletes it. Item 5 runs last, on a second project P2 with sessions `S2` and `S3`, because a US$0.01 cap would end `S1` before the rest could run and a spent project budget would taint P1. The record gives every session id, so each log line quoted can be matched to the run.

**(c) Item 5 records what the CLI actually shows.** The skeleton says the cap "surfaces `budget_exhausted` in the CLI". Two different things refuse at a cap, and the record keeps them apart. **At the key**, LiteLLM refuses a request once the key's `max_budget` is spent; the gatekeeper passes that reply through unchanged, so the CLI shows **LiteLLM's** error text, whose status and wording are **UNVERIFIED** until this run records them. **At mint**, keysmith refuses a new key with `budget_exhausted` (tasks 06, 07) once task 09's sweep has seen the spend; the local control plane records it as the session's `gatewayError`, and `panacea token` exits 1. Both are recorded. Neither is presented as the other (gap 1).

**(d) A real Global call is optional and off by default.** It needs a developer's own Anthropic API key in `deploy/local/gateway/secrets/anthropic_api_key` and `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=anthropic` (task 17), and it spends real money. Without both, the record says **not run: no key provided**, and that is a complete record. No key is ever pasted into the record.

**(e) The canary is searched for, not assumed away.** The canary is a string that appears nowhere else: `CANARY-P6-` followed by eight random hex digits generated at the start of the run. It is searched for in a data-only dump of the whole `gateway` database (every table, whatever LiteLLM names them, so the check does not depend on task 18 having found the right column) and in every gateway container's logs. The sandbox's own transcript is expected to hold it and is not searched: it lives on this machine, and ZDR is about the gateway and the upstream.

- [ ] **Step 1: Preflight**

```bash
git log --oneline -1                                                   # record the commit the run is on
docker version --format '{{.Server.Version}}'                          # Docker Desktop is up
test -f docs/verification/2026-09-25-phase-6-litellm-probe.md && echo ok  # task 18's record exists
grep -E '^\| *P0[1-9]|^\| *P1[0-2]' docs/verification/2026-09-25-phase-6-litellm-probe.md   # read every probe verdict
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8730/v1/healthz   # expect 000: nothing on the control plane's port
```

Read task 18's verdicts before starting. A FAIL on P02 (key expiry), P05 (key `max_budget`), P06 (no prompt text stored) or P11 (delete effective at once) changes what items 5, 6 and 7 can show; the record says which, up front. A FAIL on P06, P13 or P14 is an owner blocker, and this task does not start until the owner answers (task 18, step 9).

- [ ] **Step 2: Bring up the local stack on the fake upstream**

```bash
cd deploy/local/gateway
./init.sh                                   # creates secrets/ once; never overwrites
docker compose up -d --wait                 # expect every service healthy
docker compose ps --format '{{.Service}} {{.State}} {{.Publishers}}'
cd ../../..
docker build -t panacea-app/sandbox:dev images/sandbox   # the image with task 14's panacea and settings fragment
docker run --rm --entrypoint sh panacea-app/sandbox:dev -c 'claude --version; command -v panacea'
```

Record: the `ps` lines (only `gatekeeper` and `keysmith` may show a publisher, both on `127.0.0.1`), the CLI version, and the sandbox image id (`docker image inspect -f '{{.Id}}' panacea-app/sandbox:dev`). The CLI version matters for OD-20: the record says which Claude Code release this proof holds for.

- [ ] **Step 3: Start the local control plane pointed at the stack**

```bash
go build -o bin/panacea-app-api ./control-plane/cmd/api
export PANACEA_APP_BACKEND=docker
export PANACEA_APP_PROJECTS_DIR=<a directory holding two throwaway git repositories, P1 and P2>
export PANACEA_APP_GATEWAY_KEYSMITH_URL=http://127.0.0.1:4100
export PANACEA_APP_GATEWAY_BASE_URL=http://gatekeeper:4000
export PANACEA_APP_GATEWAY_NETWORK=panacea-app-gateway
export PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE="$PWD/deploy/local/gateway/secrets/<task 17's control-plane token file>"
export PANACEA_APP_GATEWAY_ROUTE_SOURCE="file:$PWD/deploy/local/gateway/projects/projects.json"
./bin/panacea-app-api &
curl -s http://127.0.0.1:8730/v1/healthz        # expect "status":"ok"
curl -s http://127.0.0.1:8730/v1/projects       # note P1's and P2's ids
```

`BASE_URL` is the address a **sandbox** uses, on the compose network, not the host's `127.0.0.1:4000`. The route-source path is the file inside the directory keysmith and the gatekeeper mount (task 21's decision (h), task 21's gap 3).

- [ ] **Step 4: Classify both projects**

Local projects start unclassified (task 21's decision (a)); the cockpit cannot classify (task 22's gap 2), so this is `curl`:

```bash
for p in <P1> <P2>; do
  curl -s -X PATCH -d '{"dataClass":{"level":"internal","residency":"any"}}' \
    http://127.0.0.1:8730/v1/projects/$p/inference
done
# expect, twice: "route":"global","effectiveRoute":"global","gateway":"on"
```

Then start the desktop (`cd desktop && npm run tauri dev`) and select P1. The chip reads **Global**. Generate the canary now and keep it in the shell only: `CANARY=CANARY-P6-$(openssl rand -hex 4)`.

- [ ] **Step 5: Items 1 and 2, a Global reply through the helper**

In the cockpit, start a Claude Code session `S1` on P1's worktree and attach. At the CLI's prompt type: `say hello. $CANARY` (the canary's value, pasted). Then, on the host:

```bash
SB=$(docker ps --format '{{.Names}} {{.Image}}' | awk '$2=="panacea-app/sandbox:dev"{print $1; exit}')
docker compose -f deploy/local/gateway/compose.yaml logs gatekeeper | grep '"session":"<S1>"' | tail -3
docker exec "$SB" ls -l /run/panacea/gateway/
docker exec "$SB" sh -c 'env | grep -E "^(ANTHROPIC_|CLAUDE_CODE_|AWS_)" | sed "s/=.*//"'
docker exec "$SB" sh -c 'printenv ANTHROPIC_BASE_URL'
docker exec "$SB" sh -c 'grep -rl apiKeyHelper /etc/claude-code 2>/dev/null || echo "fragment path: see task 14"'
```

**Item 1 observed** when the CLI's reply reads `fake global reply` and a gatekeeper line for `S1` carries `"route":"global"` and `"status":200`. **Item 2 observed** when `token` is one file of mode `0600` in the mounted directory, the variable-name list contains `ANTHROPIC_BASE_URL` and the two `CLAUDE_CODE_` names from task 14's fragment and **no** `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_USE_BEDROCK` or `AWS_*`, and the base URL is `http://gatekeeper:4000`. The `sed` prints names only; no value of any variable goes into the record. The managed-settings path is task 14's to state (its Linux location is **UNVERIFIED** there); use the path task 14 installed.

- [ ] **Step 6: Item 3, a raise takes effect in the same session**

In the cockpit, open the chip on P1, choose *Keep inference in Australia*, confirm. The chip reads **AU**. Wait 30 s (`ROUTE_CACHE_TTL`, task 13), then type a second prompt into the **same** `S1`: `again`.

```bash
docker compose -f deploy/local/gateway/compose.yaml logs gatekeeper | grep '"session":"<S1>"' | tail -3
cat deploy/local/gateway/projects/projects.json
```

**Item 3 observed** when the reply reads `fake au reply`, the newest gatekeeper line for `S1` carries `"route":"au"`, and `projects.json` shows P1 on `au`. Record the seconds between the confirmation and the first AU line: that is the tightening latency task 13 bounds by 30 s, and the only place it is measured. If the reply is still `fake global reply` after 60 s, record it as **not observed** with the file's content, and look first at the mount (a file mounted in place of its directory never sees a rename, task 21's decision (h)).

- [ ] **Step 7: Item 4, a stolen token is refused off its pod**

```bash
TOKEN=$(docker exec "$SB" /usr/local/bin/panacea token)
BODY='{"model":"<a public id from GET /v1/models>","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}'
curl -s -w '\n%{http_code}\n' -H "Authorization: Bearer $TOKEN" -H 'anthropic-version: 2023-06-01' \
  -H 'content-type: application/json' -d "$BODY" http://127.0.0.1:4000/v1/messages
docker exec "$SB" sh -c "curl -s -o /dev/null -w '%{http_code}\n' -H \"Authorization: Bearer \$(panacea token)\" \
  -H 'anthropic-version: 2023-06-01' -H 'content-type: application/json' -d '$BODY' \$ANTHROPIC_BASE_URL/v1/messages"
unset TOKEN
```

**Item 4 observed** when the host's call answers `403` with `"error":"binding_mismatch"` and the same token from inside the sandbox answers `200`. The second call is the control: without it, a `403` could be any fault. The token never appears in the record; only statuses and the problem code do.

- [ ] **Step 8: Items 8 and 9, the CLI's side calls, and what a sandbox can reach**

```bash
docker compose -f deploy/local/gateway/compose.yaml logs gatekeeper \
  | grep -oE '"path":"[^"]*","status":[0-9]+' | sort | uniq -c
docker exec "$SB" sh -c 'for u in http://litellm:4001/health/liveliness http://keysmith:4100/healthz \
  http://fakeanthropic-global:4900/_captured http://gatekeeper:4000/healthz; do
  printf "%s " "$u"; curl -s -m 3 -o /dev/null -w "%{http_code}\n" "$u" || echo unreachable; done'
```

**Item 8 observed** when the path counts include `/v1/models` with status `200` and `/api/hello` with `204`; record each count. The docs describe both calls (`GET /v1/models?limit=1000`, `HEAD /api/hello`; https://code.claude.com/docs/en/llm-gateway-protocol, read 2026-09-25, as quoted in the skeleton), and whether this CLI release makes them, and when, is what this item records. Also record whether the CLI's model picker lists the catalogue's models (task 02's `ModelsResponse` field set is **UNVERIFIED** until this). If the gatekeeper does not log `HEAD /api/hello`, say so: item 8 is then half observed.

**Item 9** is not in the skeleton's list; it is the local counterpart of task 24's cloud P02, and it is here because local mode puts every container on one network (gap 2). It is **observed as the cloud requires** when only `gatekeeper:4000` answers and the other three are `unreachable` or `000`. If `litellm:4001` answers, the record says so plainly: a sandbox holding its own session key could then call LiteLLM directly, naming `global/<id>` on an AU project, and skip the gatekeeper's binding and route. That is a local-mode finding for task 17, not a cloud one (task 19's NetworkPolicies are what task 24's P02 checks).

- [ ] **Step 9: Item 7, the canary is nowhere in the gateway**

```bash
cd deploy/local/gateway
docker compose exec -T postgres pg_dump -U postgres --data-only gateway | grep -c "$CANARY"   # expect 0
for s in $(docker compose config --services); do
  printf "%s " "$s"; docker compose logs --no-log-prefix "$s" 2>&1 | grep -c "$CANARY"; done # expect 0 for each
cd ../../..
```

**Item 7 observed** when every count is `0`. The dump covers every table in the database whatever LiteLLM calls them (decision (e)); record the dump's line count too, so a `0` from an empty dump is visible as such. Postgres's user name comes from task 17's compose file; use the one it sets. A non-zero count is a **blocker for OD-19** (task 18's P06 rule): the record names the service or table, and task 25 raises it to the owner.

- [ ] **Step 10: Item 6, deleting the session kills its token**

```bash
TOKEN=$(docker exec "$SB" /usr/local/bin/panacea token)
curl -s -X DELETE http://127.0.0.1:8730/v1/sessions/<S1>          # the cockpit's kill does the same
sleep 11                                                           # BINDING_CACHE_TTL (10 s) plus one
docker run --rm --network panacea-app-gateway curlimages/curl -s -w '\n%{http_code}\n' \
  -H "Authorization: Bearer $TOKEN" -H 'anthropic-version: 2023-06-01' -H 'content-type: application/json' \
  -d "$BODY" http://gatekeeper:4000/v1/messages
unset TOKEN
```

The replay runs from a throwaway container on the gateway network because the sandbox is gone and the host's address would be refused as `binding_mismatch` whatever the token's state. So the expected answer is not `403`: it is **`401` with `"error":"token_revoked"`**, since the binding check (task 10's order) comes before the source check. **Item 6 observed** on that status and code. Record the seconds from `DELETE` to the first `401`; it is bounded by the gatekeeper's binding cache, not by LiteLLM (task 18's P11 covers LiteLLM's own delete).

- [ ] **Step 11: Item 5, a budget refuses, at the key and at mint**

Restart keysmith with a tiny session cap and a small project budget, then use P2, which has spent nothing:

```bash
cd deploy/local/gateway
PANACEA_APP_GATEWAY_SESSION_CAP_USD=0.01 PANACEA_APP_GATEWAY_DEFAULT_PROJECT_BUDGET_USD=0.05 \
  docker compose up -d --wait --force-recreate keysmith
cd ../../..
```

Start session `S2` on P2 and send prompts until the CLI shows an error. Record, verbatim, the **first error the CLI shows** and the gatekeeper line for it (its `status`). That is the key-level refusal: LiteLLM's reply on a spent `max_budget`, passed through unchanged (decision (c)). Each fake reply is priced from the request's size (task 16: `input_tokens` is a quarter of the body's bytes), so a Claude Code request with its system prompt is expected to spend past US$0.01 in one or two calls; the record gives the count.

Then wait 70 s (task 09's 60 s sweep plus margin) and start session `S3` on P2:

```bash
curl -s http://127.0.0.1:8730/v1/sessions/<S3> | grep -o '"gatewayError":"[^"]*"'
curl -s http://127.0.0.1:8730/v1/projects/<P2>/inference
```

**Item 5 observed** when `S2`'s CLI shows a refusal after a small number of calls, **and** `S3` records `gatewayError` carrying `budget_exhausted` for the project dimension, **and** the cockpit's panel for P2 shows the *"Budget reached."* sentence (task 22). Each of the three is recorded on its own line; any one alone is "partly observed". Restore the defaults afterwards (`docker compose up -d --wait --force-recreate keysmith` with the variables unset).

- [ ] **Step 12: Optional, one real Global call**

Only if a developer has put their own key in `secrets/anthropic_api_key`: restart with `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=anthropic`, run one short prompt in a new session on a Global project, and record the reply's first five words and the gatekeeper line. Otherwise record **not run: no key provided** (decision (d)). This is the only step that spends money, and the only one that touches the real Anthropic API; it proves nothing about Bedrock.

- [ ] **Step 13: Write the record**

Create `docs/verification/2026-09-25-phase-6-local-e2e.md` with exactly these sections, each item's commands and output pasted, and nothing paraphrased:

```markdown
# Phase 6 local end-to-end proof

Run on: <date and time, with zone> by <role, not a name>
Commit: <git log --oneline -1>
Claude Code: <claude --version, from the sandbox image>
Sandbox image: <image id>    LiteLLM image: <digest from compose.yaml>
Task 18 verdicts relied on: <P01 … P12, each PASS / FAIL / UNKNOWN>

| # | Item | Result | Evidence (step) |
|---|---|---|---|
| 1 | Global reply through the gatekeeper, route=global | observed / not observed | step 5 |
| 2 | Helper path; no credential in the environment | … | step 5 |
| 3 | Raise to AU takes effect in the same session; latency <n> s | … | step 6 |
| 4 | Token replayed off-pod is 403 binding_mismatch; on-pod 200 | … | step 7 |
| 5 | Budget: key-level refusal; mint-level budget_exhausted; panel sentence | … | step 11 |
| 6 | Deleted session's token is 401 token_revoked within <n> s | … | step 10 |
| 7 | Canary absent from the gateway database and every gateway log | … | step 9 |
| 8 | /v1/models and HEAD /api/hello seen and answered | … | step 8 |
| 9 | From a sandbox, only the gatekeeper is reachable | … | step 8 |
| — | One real Global call | run / not run: no key provided | step 12 |

## Findings
<each result that contradicts an earlier task, with the task id it breaks; "none" if none>

## Evidence
<one subsection per step, commands and output verbatim; statuses and codes only, never a token>
```

- [ ] **Step 14: Commit**

```bash
git add docs/verification/2026-09-25-phase-6-local-e2e.md
git commit -F <message file> -- docs/verification/2026-09-25-phase-6-local-e2e.md
```

Message: `docs(verification): phase 6 local end-to-end with unmodified Claude Code (phase 6 task 23)`, a body listing each item's result in one line, and the attribution line.

**Acceptance:** the record exists; every row of its table says *observed*, *not observed* or *partly observed*, and every such word is backed by a pasted command and output in "Evidence"; `grep -cE 'sk-|Bearer [A-Za-z0-9]' docs/verification/2026-09-25-phase-6-local-e2e.md` prints `0`.

**What this task does not claim:**

- **Anything in the cloud.** No Kata, no AKS, no NetworkPolicy, no Key Vault, no Entra. Task 24's scripts are the cloud's proof, and they are not run in this phase.
- **Anything about Bedrock.** The local AU route is an Anthropic-format fake labelled `au` (task 16). Item 3 proves the gatekeeper's route choice and LiteLLM's group routing, not the Anthropic-to-Bedrock translation (task 24's P04, seam S6.4).
- **That the next Claude Code release behaves the same.** The record names the CLI version it ran; OD-20 has the cloud track the CLI, and a re-run on each new release is the only way to keep this true.
- **Pod source-address binding on AKS.** Item 4 proves it on a Docker bridge network, where the source address survives by construction. On AKS it is task 24's P03.
- **ZDR upstream.** Item 7 covers our stores and logs. Anthropic's and AWS's retention are the org's contracts (owner question Q3).

**Gaps against the skeleton, for the orchestrator:**

1. **"`budget_exhausted` in the CLI" conflates two refusals.** The CLI sees LiteLLM's key-budget error, passed through; `budget_exhausted` is keysmith's code at mint (decision (c)). If the owner wants the CLI to see `budget_exhausted` too, task 11 would translate LiteLLM's budget reply, which is a second edit to a proxy whose rule is "one field rewritten"; an owner call.
2. **Local mode may let a sandbox reach LiteLLM directly.** Task 17 puts every service and the sandboxes on one network (`panacea-app-gateway`). A sandbox's own key is valid at LiteLLM for both model groups (task 07's `Models`), so from inside the sandbox it can skip the gatekeeper. Task 17 should use two networks, an edge one holding the gatekeeper and the sandboxes, and an internal one holding everything else with the gatekeeper on both. Item 9 detects it either way.
3. **Task 17's compose file must pass `PANACEA_APP_GATEWAY_SESSION_CAP_USD` and `DEFAULT_{USER,TEAM,PROJECT}_BUDGET_USD` through from the host** (`${VAR:-default}`), or step 11 cannot set them without editing the file.
4. **Task 17 must name the control plane's token file** for `PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE` (task 15's variable), and its caller line must include `usage` (task 21's gap 5).
5. **Items 5 and 9 go beyond the skeleton's wording** (item 9 is new; item 5 has three parts). Both are additions, not changes of intent.
