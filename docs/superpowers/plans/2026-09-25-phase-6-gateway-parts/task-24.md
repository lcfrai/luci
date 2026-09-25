### Task 24: Cloud proving scripts, written and not run

Six claims about the cloud gateway cannot be proven on Docker Desktop, and two of them decide whether the design stands: **P03**, whether the gatekeeper sees a sandbox pod's own address (the pod binding, open risk 2), and **P04**, whether a real AU session's requests survive LiteLLM's Anthropic-to-Bedrock translation (P6-D3, open risk 1). This task writes the scripts that will answer them, and tests every script's verdict logic on canned output. **No script is run in this phase**: there is no cluster, no AWS account (owner question Q3) and no Key Vault yet.

**The one thing this task must not get wrong is a script that passes for the wrong reason.** A reachability probe that treats "any answer" and "a 200" as the same thing reads a reachable service as blocked, or the reverse; a check that finds nothing because it looked in the wrong namespace reads as a pass. So every script follows phase 8's proving rules (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:464`, and its task 31 part's contract, `…-parts/task-31.md:42-92`): commands go through one `run_cmd` so tests can replace them; "answers" means curl exited 0, whatever the status, and "does not answer" means it exited non-zero; every script prints what it saw, not only the verdict; and a prerequisite that is missing is **NOT RUN**, never PASS.

**Files:**
- `infra/smoke/gateway/lib.sh`: `run_cmd`, `verdict`, `need`, `probe_pod`, `finish` (new)
- `infra/smoke/gateway/run.sh`: runs `p01` to `p06` in order, exits non-zero on any FAIL (new)
- `infra/smoke/gateway/p01-no-public-route.sh` (new)
- `infra/smoke/gateway/p02-sandbox-reaches-gatekeeper-only.sh` (new)
- `infra/smoke/gateway/p03-pod-source-address.sh` (new)
- `infra/smoke/gateway/p04-au-route-bedrock.sh` (new)
- `infra/smoke/gateway/p05-zdr-config.sh` (new)
- `infra/smoke/gateway/p06-budgets-on.sh` (new)
- `infra/smoke/gateway/gateway_smoke_test.sh`: every script against canned passing and failing output (new)

**Interfaces:**
- Consumes: task 19's chart (namespace `panacea-gateway`, Services `gatekeeper:4000`, `keysmith:4100`, `litellm:4001`, the ConfigMap holding task 02's generated `litellm.config.yaml`, the gatekeeper's labels); task 20's Bedrock federation (the role LiteLLM assumes, the `au.` profiles); task 12's log fields (`session`, `project`, `route`, `path`, `status`, `ms`); phase 8's namespace `panacea-sandboxes`, its NetworkPolicies and its sandbox image by digest (ask P8-1, P8-3); phase 8's human-path rule for pod creation (`task-31.md:61-80`)
- Produces: one line per check, `gateway <id>: PASS|FAIL|NOT RUN <what was seen>`, and `run.sh`'s exit status
- Required by: task 25 (lists every check as not run); phase 8's task 32 proving list, which should carry P02 and P03 (gap 1)

**Decisions where the skeleton is silent:**

**(a) P03 measures with an echo, not with the gatekeeper.** The gatekeeper does not say what source address it saw, and must not: its logs carry six named fields and nothing else (task 12), and a `403 binding_mismatch` body carries no address. So P03 stands up, for the length of the probe, an echo server in `panacea-gateway` behind its own ClusterIP Service, reached from a probe pod in `panacea-sandboxes` over the same kind of path a sandbox takes to the gatekeeper (pod → ClusterIP Service → pod, across the two namespaces, through the same CNI). The echo answers with the client address it saw; the probe pod compares it to its own pod IP. The echo is `agnhost netexec`, whose `/clientip` endpoint returns the caller's `ip:port` (**training knowledge, UNVERIFIED**; the script prints the raw reply so a different format is visible). A temporary NetworkPolicy admits the probe to the echo only. Everything P03 creates carries `panacea-app/smoke: "true"` and is deleted by an `EXIT` trap.

**(b) P04 reads the gatekeeper's record of a real session; it does not drive the CLI.** A real AU session needs a signed-in user and the broker (ask P8-1), which is the owner's hands-on path (phase 8's OD-12 batch). The operator runs the session and gives the script its id and pod; the script checks that the pod was launched with `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` and no `CLAUDE_CODE_USE_BEDROCK`, then reads the gatekeeper's lines for that session. The CLI's output is never read or interpreted (OD-8, global constraint 7).

**(c) Pods are created on the human path only.** P02 and P03 create pods in `panacea-sandboxes`, which the pipeline identity must never be able to do (phase 8, `task-31.md:61-80`). Each asks `kubectl auth can-i` first; on `no` it prints NOT RUN, or FAIL when `GW_REQUIRE_ALL=1`, exactly as phase 8's `--require-kata` does.

**(d) No secret is printed, and none is passed on a command line.** P05 and P06 need a database URL and the LiteLLM master key. Each is read from a file the operator names (`GW_DATABASE_URL_FILE`, `GW_MASTER_KEY_FILE`). The master key reaches `curl` as `-H @<file>`, a header file written at mode 0600 in a temporary directory the `EXIT` trap removes. The database URL is split by `lib.sh`'s `pg_env` into libpq's own environment variables (`PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`), which only the same user can read. So no value appears in a process's arguments, a log line or the record. A script that cannot read its file prints NOT RUN naming the variable. If phase 8 settles an Entra token login for this database (ask P8-4), `pg_env` leaves `PGPASSWORD` unset and the operator supplies the token the same way.

**(e) Canned output is a directory.** With `GW_CANNED=<dir>`, `run_cmd <key> …` prints `<dir>/<key>.out` and returns the number in `<dir>/<key>.rc` (0 when absent) instead of running anything. A test is a directory of small files and one expected verdict line; nothing is mocked in a script's own text.

| Id | Script | Check | PASS when |
|---|---|---|---|
| P01 | `p01-no-public-route.sh` | Nothing public leads to the gateway | no `HTTPRoute` or `Ingress` in `panacea-gateway`; no `LoadBalancer` or `NodePort` Service there; no `HTTPRoute` in any namespace has a `backendRef` into `panacea-gateway`; `gateway.<zone>` and `litellm.<zone>` do not resolve |
| P02 | `p02-sandbox-reaches-gatekeeper-only.sh` | A sandbox reaches the gatekeeper and nothing else in the namespace | from a probe pod in `panacea-sandboxes`, `gatekeeper:4000` answers; `litellm:4001`, `keysmith:4100` and the gateway database's private endpoint on 5432 do not |
| P03 | `p03-pod-source-address.sh` | The address a gateway pod sees is the sandbox pod's own | the echo's reported client IP equals the probe pod's `status.podIP` |
| P04 | `p04-au-route-bedrock.sh` | A real AU session's Claude Code requests succeed through Bedrock | the pod has the AU launch variable and not the Bedrock one; the gatekeeper logged at least `GW_P04_MIN` (default 5) `/v1/messages` lines for the session, all `route=au`, none with a status ≥ 400 |
| P05 | `p05-zdr-config.sh` | Our stores and the AWS account keep no prompt | the live LiteLLM config has no callback key and has `store_prompts_in_spend_logs: false`; a data-only dump of the gateway database holds no `GW_CANARY`; Bedrock model-invocation logging is off in `ap-southeast-2` and `ap-southeast-4` |
| P06 | `p06-budgets-on.sh` | A new project and person get non-zero budgets | LiteLLM's team (the project) and user (the person) both carry a `max_budget` > 0; keysmith's Deployment sets no budget variable to `0` or empty |

**What each script needs from the operator**, all environment variables, none a secret on its own except the two `_FILE`s' contents: `GW_ZONE` (the public DNS zone, P01); `GW_SANDBOX_IMAGE` (the sandbox image **by digest**, P02, P03); `GW_RUNTIME_CLASS` (phase 8's configured Kata class, so the probe runs in a microVM as a session does); `GW_SANDBOX_NODE_SELECTOR` (default `panacea-app/role=sandbox`, **UNVERIFIED** against phase 8's pool labels); `GW_DB_HOST` (the gateway database's private FQDN, P02); `GW_ECHO_IMAGE` (`agnhost` by digest, P03); `GW_P04_SESSION`, `GW_P04_POD`, `GW_P04_MIN` (P04); `GW_GATEKEEPER_SELECTOR` (default `app.kubernetes.io/name=gatekeeper`, task 19's label, **UNVERIFIED** until that chart exists); `GW_LITELLM_CONFIGMAP` (default `litellm-config`); `GW_CANARY`, `GW_DATABASE_URL_FILE` (P05); `GW_LITELLM_URL` (a `kubectl port-forward` the operator opens), `GW_MASTER_KEY_FILE`, `GW_PROJECT`, `GW_PERSON` (P06); `GW_REQUIRE_ALL` (`1` turns every NOT RUN into FAIL).

- [ ] **Step 1: Preflight**

```bash
test -d infra/smoke/gateway && echo "STOP: infra/smoke/gateway exists" || echo ok   # expect ok
command -v bash shellcheck                                                         # shellcheck optional; step 6 uses it if present
ls .claude/worktrees/env/infra/smoke 2>/dev/null || echo "phase 8's smoke not merged; nothing here depends on it"
```

Nothing in these scripts sources phase 8's `infra/smoke/lib.sh`: the two trees merge later (seam S6.9), and a shared library across them would be a dependency on a file this tree cannot see. The style is shared; the code is not.

- [ ] **Step 2: Write the failing tests**

Every case is a directory of canned outputs, the variables it sets, the script, the verdict line it must print and the exit status it must give. Each comment names the mutant the case kills.

Create `infra/smoke/gateway/gateway_smoke_test.sh`:

```bash
#!/usr/bin/env bash
# Every gateway proving script against canned output. Contacts no cluster, no cloud.
set -u
here=$(cd "$(dirname "$0")" && pwd)
failures=0

# can <dir> <key> <rc> [line...]: one canned command.
can() { local d=$1 k=$2 rc=$3; shift 3; printf '%s\n' "$@" >"$d/$k.out"; echo "$rc" >"$d/$k.rc"; }

# expect <name> <script> <dir> <regex> <exit> [VAR=value...]: run and judge.
expect() {
  local name=$1 script=$2 dir=$3 want=$4 code=$5; shift 5
  local out rc
  out=$(env -i PATH="$PATH" HOME="$HOME" GW_CANNED="$dir" "$@" bash "$here/$script" 2>&1); rc=$?
  if grep -Eq -- "$want" <<<"$out" && [ "$rc" = "$code" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name: want /$want/ and exit $code, got exit $rc:"; sed 's/^/     /' <<<"$out"
    failures=$((failures + 1))
  fi
}

fresh() { mktemp -d "${TMPDIR:-/tmp}/tmp.gwsmoke.XXXXXX"; }
trap 'rm -rf "${TMPDIR:-/tmp}"/tmp.gwsmoke.* 2>/dev/null' EXIT

# ── P01 ──────────────────────────────────────────────────────────────────────
p01_ok() { local d; d=$(fresh)
  can "$d" p01.ns 0 "namespace/panacea-gateway"
  can "$d" p01.routes 0
  can "$d" p01.svctypes 0 "gatekeeper=ClusterIP" "keysmith=ClusterIP" "litellm=ClusterIP"
  can "$d" p01.backendrefs 0 "panacea-edge/hub:panacea-hub," "panacea-edge/broker:panacea-broker,"
  can "$d" p01.dns.gateway 2
  can "$d" p01.dns.litellm 2
  echo "$d"; }
expect "P01 passes on a closed namespace" p01-no-public-route.sh "$(p01_ok)" 'gateway P01: PASS' 0 GW_ZONE=example.test
# Mutant: an HTTPRoute in the namespace goes unnoticed.
d=$(p01_ok); can "$d" p01.routes 0 "httproute.gateway.networking.k8s.io/gatekeeper"
expect "P01 fails on a route in the namespace" p01-no-public-route.sh "$d" 'gateway P01: FAIL .*httproute' 1 GW_ZONE=example.test
# Mutant: only the namespace's own routes are read, so an edge route pointing in is missed.
d=$(p01_ok); can "$d" p01.backendrefs 0 "panacea-edge/sneaky:panacea-gateway,"
expect "P01 fails on a cross-namespace backendRef" p01-no-public-route.sh "$d" 'gateway P01: FAIL .*sneaky' 1 GW_ZONE=example.test
# Mutant: a LoadBalancer Service is not a "route" and is not checked.
d=$(p01_ok); can "$d" p01.svctypes 0 "gatekeeper=LoadBalancer"
expect "P01 fails on a LoadBalancer Service" p01-no-public-route.sh "$d" 'gateway P01: FAIL .*LoadBalancer' 1 GW_ZONE=example.test
# Mutant: a kubectl that could not list is read as "nothing found".
d=$(p01_ok); can "$d" p01.routes 1 "Error from server (Forbidden)"
expect "P01 fails when it cannot list" p01-no-public-route.sh "$d" 'gateway P01: FAIL .*could not list' 1 GW_ZONE=example.test
# Mutant: a name that resolves publicly is not a route.
d=$(p01_ok); can "$d" p01.dns.gateway 0 "20.1.2.3 gateway.example.test"
expect "P01 fails when gateway.<zone> resolves" p01-no-public-route.sh "$d" 'gateway P01: FAIL .*resolves' 1 GW_ZONE=example.test

# ── P02 and P03 share the probe-pod prerequisites ─────────────────────────────
POD=(GW_SANDBOX_IMAGE=registry.example/sandbox@sha256:0000 GW_RUNTIME_CLASS=kata-vm-isolation GW_DB_HOST=gw-db.private.example)
p02_ok() { local d; d=$(fresh)
  can "$d" p02.cani 0 "yes"
  can "$d" p02.run 0 "pod/gwsmoke-p02 condition met"
  can "$d" p02.logs 0 "gatekeeper 0 0.001204" "litellm 28 0.000000" "keysmith 28 0.000000" "postgres 28 0.000000"
  can "$d" p02.delete 0
  echo "$d"; }
expect "P02 passes when only the gatekeeper connects" p02-sandbox-reaches-gatekeeper-only.sh "$(p02_ok)" 'gateway P02: PASS' 0 "${POD[@]}"
# Mutant: a script that only checks the negatives passes with the gatekeeper unreachable too.
d=$(p02_ok); can "$d" p02.logs 0 "gatekeeper 28 0.000000" "litellm 28 0.000000" "keysmith 28 0.000000" "postgres 28 0.000000"
expect "P02 fails when the gatekeeper does not connect" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: FAIL .*gatekeeper' 1 "${POD[@]}"
# Mutant: "answers" judged by a 200, so LiteLLM's 401 to a keyless probe reads as blocked.
d=$(p02_ok); can "$d" p02.logs 0 "gatekeeper 0 0.001204" "litellm 0 0.000950" "keysmith 28 0.000000" "postgres 28 0.000000"
expect "P02 fails when LiteLLM connects, whatever it answers" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: FAIL .*litellm' 1 "${POD[@]}"
# Mutant: a refused connection (exit 7) with a timing read as reachable.
d=$(p02_ok); can "$d" p02.logs 0 "gatekeeper 0 0.001204" "litellm 7 0.000000" "keysmith 7 0.000000" "postgres 28 0.000000"
expect "P02 passes on refused connections" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: PASS' 0 "${POD[@]}"
# Mutant: reachability judged by curl's exit status, so Postgres answering an HTTP probe with an
# empty reply (exit 52, after a completed TCP connect) reads as blocked.
d=$(p02_ok); can "$d" p02.logs 0 "gatekeeper 0 0.001204" "litellm 28 0.000000" "keysmith 28 0.000000" "postgres 52 0.004100"
expect "P02 fails when Postgres connects and answers nothing" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: FAIL .*postgres' 1 "${POD[@]}"
# Mutant: no pods create read as a pass; and GW_REQUIRE_ALL ignored.
d=$(p02_ok); can "$d" p02.cani 1 "no"
expect "P02 is NOT RUN without pods create" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: NOT RUN' 0 "${POD[@]}"
expect "P02 fails without pods create when required" p02-sandbox-reaches-gatekeeper-only.sh "$d" 'gateway P02: FAIL .*pods create' 1 "${POD[@]}" GW_REQUIRE_ALL=1
# Mutant: an image by tag, which the proving run cannot reproduce.
expect "P02 refuses an image without a digest" p02-sandbox-reaches-gatekeeper-only.sh "$(p02_ok)" 'gateway P02: NOT RUN .*digest' 0 \
  GW_SANDBOX_IMAGE=registry.example/sandbox:dev GW_RUNTIME_CLASS=kata-vm-isolation GW_DB_HOST=gw-db.private.example

ECHO=(GW_ECHO_IMAGE=registry.k8s.io/e2e-test-images/agnhost@sha256:1111)
p03_ok() { local d; d=$(fresh)
  can "$d" p03.cani.sandboxes 0 "yes"
  can "$d" p03.cani.gateway 0 "yes"
  can "$d" p03.cani.sbnetpol 0 "yes"
  can "$d" p03.apply 0 "pod/gwsmoke-p03-echo created" "service/gwsmoke-p03-echo created"
  can "$d" p03.run 0 "pod/gwsmoke-p03 condition met"
  can "$d" p03.logs 0 "podip=10.244.3.17" "seen=10.244.3.17:51234"
  can "$d" p03.delete 0
  echo "$d"; }
expect "P03 passes when the echo sees the pod's own address" p03-pod-source-address.sh "$(p03_ok)" 'gateway P03: PASS .*10\.244\.3\.17' 0 "${POD[@]}" "${ECHO[@]}"
# Mutant: a SNAT to the node's address accepted because "an address was seen".
d=$(p03_ok); can "$d" p03.logs 0 "podip=10.244.3.17" "seen=10.224.0.5:40112"
expect "P03 fails on a rewritten source, naming S6.10" p03-pod-source-address.sh "$d" 'gateway P03: FAIL .*10\.224\.0\.5.*S6\.10' 1 "${POD[@]}" "${ECHO[@]}"
# Mutant: an IPv6 reply compared with its port and brackets still attached.
d=$(p03_ok); can "$d" p03.logs 0 "podip=fd00:10:244::11" "seen=[fd00:10:244::11]:51234"
expect "P03 passes on IPv6" p03-pod-source-address.sh "$d" 'gateway P03: PASS' 0 "${POD[@]}" "${ECHO[@]}"
# Mutant: an empty reply compared as equal to an empty pod IP.
d=$(p03_ok); can "$d" p03.logs 0 "podip=" "seen="
expect "P03 fails when nothing was seen" p03-pod-source-address.sh "$d" 'gateway P03: FAIL .*no reply' 1 "${POD[@]}" "${ECHO[@]}"

# ── P04 ──────────────────────────────────────────────────────────────────────
S=(GW_P04_SESSION=s-au01 GW_P04_POD=sess-s-au01)
line() { printf '{"time":"t","level":"INFO","msg":"request","session":"%s","project":"p-9","route":"%s","path":"%s","status":%s,"ms":812}' "$1" "$2" "$3" "$4"; }
p04_ok() { local d i; d=$(fresh)
  can "$d" p04.env 0 "ANTHROPIC_BASE_URL=http://gatekeeper.panacea-gateway.svc.cluster.local:4000" \
    "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1"
  : >"$d/p04.logs.out"
  for i in 1 2 3 4 5 6; do line s-au01 au /v1/messages 200 >>"$d/p04.logs.out"; echo >>"$d/p04.logs.out"; done
  line s-other global /v1/messages 400 >>"$d/p04.logs.out"; echo >>"$d/p04.logs.out"; echo 0 >"$d/p04.logs.rc"
  echo "$d"; }
expect "P04 passes on six AU requests, all 2xx" p04-au-route-bedrock.sh "$(p04_ok)" 'gateway P04: PASS .*6' 0 "${S[@]}"
# Mutant: a Bedrock rejection (a 400 on a field it does not accept) read as success.
d=$(p04_ok); line s-au01 au /v1/messages 400 >>"$d/p04.logs.out"
expect "P04 fails on one 400, naming seam S6.4" p04-au-route-bedrock.sh "$d" 'gateway P04: FAIL .*400.*S6\.4' 1 "${S[@]}"
# Mutant: a request of this session that left on Global is not looked for.
d=$(p04_ok); printf '\n' >>"$d/p04.logs.out"; line s-au01 global /v1/messages 200 >>"$d/p04.logs.out"
expect "P04 fails on a Global request" p04-au-route-bedrock.sh "$d" 'gateway P04: FAIL .*global' 1 "${S[@]}"
# Mutant: zero requests is "no failures", so a session that never ran passes.
d=$(p04_ok); : >"$d/p04.logs.out"
expect "P04 fails with too few requests" p04-au-route-bedrock.sh "$d" 'gateway P04: FAIL .*0 of at least 5' 1 "${S[@]}"
# Mutant: the pod's launch environment is not checked (P6-D2, P6-D3).
d=$(p04_ok); can "$d" p04.env 0 "CLAUDE_CODE_USE_BEDROCK=1" "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1"
expect "P04 fails on CLAUDE_CODE_USE_BEDROCK in the pod" p04-au-route-bedrock.sh "$d" 'gateway P04: FAIL .*CLAUDE_CODE_USE_BEDROCK' 1 "${S[@]}"
expect "P04 is NOT RUN without a session" p04-au-route-bedrock.sh "$(p04_ok)" 'gateway P04: NOT RUN .*GW_P04_SESSION' 0

# ── P05 ──────────────────────────────────────────────────────────────────────
secrets=$(fresh)   # dummy values only; the scripts must read files, and must not print them
echo 'postgresql://gw:not-a-password@gw-db.private.example:5432/gateway?sslmode=require' >"$secrets/db_url"
echo 'sk-not-a-master-key' >"$secrets/master_key"
Z=(GW_CANARY=CANARY-P6-0badc0de GW_DATABASE_URL_FILE="$secrets/db_url")
p05_ok() { local d; d=$(fresh)
  can "$d" p05.config 0 "model_list:" "  - model_name: au/claude-opus-4-8" "general_settings:" \
    "  store_prompts_in_spend_logs: false" "litellm_settings:" "  turn_off_message_logging: true"
  can "$d" p05.dump 0 "COPY public.\"LiteLLM_SpendLogs\" (request_id) FROM stdin;" "abc" "\\."
  can "$d" p05.bedrock.ap-southeast-2 0 "{}"
  can "$d" p05.bedrock.ap-southeast-4 0 "{}"
  echo "$d"; }
expect "P05 passes on a clean config, dump and account" p05-zdr-config.sh "$(p05_ok)" 'gateway P05: PASS' 0 "${Z[@]}"
# Mutant: only "callbacks:" is matched, so success_callback slips through.
d=$(p05_ok); can "$d" p05.config 0 "litellm_settings:" "  success_callback: [\"langfuse\"]" "  store_prompts_in_spend_logs: false"
expect "P05 fails on success_callback" p05-zdr-config.sh "$d" 'gateway P05: FAIL .*callback' 1 "${Z[@]}"
# Mutant: prompt storage not asserted off, only callbacks.
d=$(p05_ok); can "$d" p05.config 0 "general_settings:" "  store_prompts_in_spend_logs: true"
expect "P05 fails when prompts are stored" p05-zdr-config.sh "$d" 'gateway P05: FAIL .*store_prompts' 1 "${Z[@]}"
# Mutant: the canary search reads only one table.
d=$(p05_ok); can "$d" p05.dump 0 "COPY public.\"LiteLLM_ErrorLogs\" (x) FROM stdin;" "say hello CANARY-P6-0badc0de" "\\."
expect "P05 fails when the canary is anywhere in the dump" p05-zdr-config.sh "$d" 'gateway P05: FAIL .*canary' 1 "${Z[@]}"
# Mutant: an unreadable database read as "no canary found".
d=$(p05_ok); can "$d" p05.dump 1 "pg_dump: error: connection failed"
expect "P05 fails when the dump fails" p05-zdr-config.sh "$d" 'gateway P05: FAIL .*dump' 1 "${Z[@]}"
# Mutant: only Sydney checked, though the au. profiles reportedly also use Melbourne.
d=$(p05_ok); can "$d" p05.bedrock.ap-southeast-4 0 '{"loggingConfig":{"textDataDeliveryEnabled":true}}'
expect "P05 fails on invocation logging in ap-southeast-4" p05-zdr-config.sh "$d" 'gateway P05: FAIL .*ap-southeast-4' 1 "${Z[@]}"

# ── P06 ──────────────────────────────────────────────────────────────────────
B=(GW_LITELLM_URL=http://127.0.0.1:4001 GW_MASTER_KEY_FILE="$secrets/master_key" GW_PROJECT=p-new GW_PERSON=t-1:o-2)
p06_ok() { local d; d=$(fresh)
  can "$d" p06.team 0 '{"team_id":"p-new","team_info":{"max_budget":1000.0,"budget_duration":"30d"}}'
  can "$d" p06.user 0 '{"user_id":"t-1:o-2","user_info":{"max_budget":200.0}}'
  can "$d" p06.keysmith.env 0 "PANACEA_APP_GATEWAY_SESSION_CAP_USD=25"
  echo "$d"; }
expect "P06 passes on non-zero budgets" p06-budgets-on.sh "$(p06_ok)" 'gateway P06: PASS' 0 "${B[@]}"
# Mutant: a null budget (unlimited in LiteLLM) read as "a budget exists".
d=$(p06_ok); can "$d" p06.team 0 '{"team_id":"p-new","team_info":{"max_budget":null}}'
expect "P06 fails on a null team budget" p06-budgets-on.sh "$d" 'gateway P06: FAIL .*team' 1 "${B[@]}"
# Mutant: zero read as a number and therefore a budget.
d=$(p06_ok); can "$d" p06.user 0 '{"user_id":"t-1:o-2","user_info":{"max_budget":0}}'
expect "P06 fails on a zero user budget" p06-budgets-on.sh "$d" 'gateway P06: FAIL .*user' 1 "${B[@]}"
# Mutant: keysmith's own defaults not checked, so DEFAULT_TEAM_BUDGET_USD=0 passes.
d=$(p06_ok); can "$d" p06.keysmith.env 0 "PANACEA_APP_GATEWAY_DEFAULT_TEAM_BUDGET_USD=0"
expect "P06 fails on a zero default in keysmith" p06-budgets-on.sh "$d" 'gateway P06: FAIL .*DEFAULT_TEAM_BUDGET_USD' 1 "${B[@]}"

# A secret never reaches the output, on any case (the scripts' own promise, decision (d)).
d=$(p06_ok); out=$(env -i PATH="$PATH" GW_CANNED="$d" "${B[@]}" bash "$here/p06-budgets-on.sh" 2>&1)
if grep -q 'not-a-master-key' <<<"$out"; then echo "FAIL P06 printed the master key"; failures=$((failures + 1)); else echo "ok   P06 prints no key"; fi
d=$(p05_ok); out=$(env -i PATH="$PATH" GW_CANNED="$d" "${Z[@]}" bash "$here/p05-zdr-config.sh" 2>&1)
if grep -q 'not-a-password' <<<"$out"; then echo "FAIL P05 printed the database password"; failures=$((failures + 1)); else echo "ok   P05 prints no password"; fi

echo "gateway smoke tests: $failures failed"
[ "$failures" -eq 0 ]
```

That is thirty-six cases over six scripts: thirty-four verdicts and two leak checks. No case contacts a cluster, a cloud or a network: with `GW_CANNED` set, `run_cmd` never executes its command, and `expect` runs each script under `env -i`, so a variable from the author's own shell cannot make a case pass.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `bash infra/smoke/gateway/gateway_smoke_test.sh`

Expected: every case `FAIL` (the scripts do not exist, so each exits 127), and the last line `gateway smoke tests: 34 failed` (the two leak checks pass vacuously on a missing script, which step 7's mutant 4 closes).

- [ ] **Step 4: Write `lib.sh`**

Create `infra/smoke/gateway/lib.sh`:

```bash
#!/usr/bin/env bash
# Shared by infra/smoke/gateway/p0*.sh. Phase 8's proving style (its task 31 contract), not its code.
# Every cluster, cloud and network call goes through run_cmd, so gateway_smoke_test.sh can replace it.
set -u

GW_FAILED=0
GW_TMP=$(mktemp -d "${TMPDIR:-/tmp}/gwsmoke.XXXXXX")
GW_CLEANUP=()

finish() {
  local c
  for c in "${GW_CLEANUP[@]+"${GW_CLEANUP[@]}"}"; do eval "$c" >/dev/null 2>&1 || true; done
  rm -rf "$GW_TMP"
}
trap finish EXIT

# run_cmd <key> <command...>. With GW_CANNED=<dir>: print <dir>/<key>.out, return <dir>/<key>.rc
# (0 when absent), and run nothing.
run_cmd() {
  local key=$1; shift
  if [ -n "${GW_CANNED:-}" ]; then
    cat "$GW_CANNED/$key.out" 2>/dev/null
    return "$(cat "$GW_CANNED/$key.rc" 2>/dev/null || echo 0)"
  fi
  "$@"
}

# verdict <id> PASS|FAIL|"NOT RUN" <what was seen...>. GW_REQUIRE_ALL=1 makes NOT RUN a FAIL.
verdict() {
  local id=$1 v=$2; shift 2
  if [ "$v" = "NOT RUN" ] && [ "${GW_REQUIRE_ALL:-0}" = 1 ]; then v=FAIL; fi
  printf 'gateway %s: %s %s\n' "$id" "$v" "$*"
  if [ "$v" = FAIL ]; then GW_FAILED=1; fi
  return 0
}

# need <id> VAR...: NOT RUN naming the first unset variable, and return 1.
need() {
  local id=$1 v; shift
  for v in "$@"; do
    if [ -z "${!v:-}" ]; then verdict "$id" "NOT RUN" "($v is not set)"; return 1; fi
  done
}

# pinned <id> VAR: NOT RUN unless the image in VAR is pinned by digest.
pinned() {
  case "${!2}" in *@sha256:*) return 0 ;; esac
  verdict "$1" "NOT RUN" "($2 must name an image by @sha256 digest)"; return 1
}

# can_i <id> <key> <verb> <resource> <namespace>: the human-path rule (phase 8 task 31).
can_i() {
  local ans
  ans=$(run_cmd "$2" kubectl auth can-i "$3" "$4" -n "$5" 2>/dev/null)
  [ "$ans" = yes ] && return 0
  verdict "$1" "NOT RUN" "(no $4 $3 in $5: run as a member of the cluster admin group, never --admin)"; return 1
}

exit_status() { exit "$GW_FAILED"; }
```

Append to `infra/smoke/gateway/lib.sh` the three helpers that keep secrets out of arguments and the probe pod:

```bash
# pg_env <file>: export libpq's variables from the postgres URL on the file's first line. Prints
# nothing. A percent-encoded password is NOT decoded; phase 8's URL format decides (ask P8-4).
pg_env() {
  local url re='^postgres(ql)?://([^:@/]+)(:([^@]*))?@([^:/?]+)(:([0-9]+))?/([^?]+)(\?(.*))?$'
  url=$(head -n1 "$1" 2>/dev/null) || return 1
  [[ $url =~ $re ]] || return 1
  export PGUSER="${BASH_REMATCH[2]}" PGHOST="${BASH_REMATCH[5]}"
  export PGPORT="${BASH_REMATCH[7]:-5432}" PGDATABASE="${BASH_REMATCH[8]}"
  if [ -n "${BASH_REMATCH[4]}" ]; then export PGPASSWORD="${BASH_REMATCH[4]}"; fi
  local q="${BASH_REMATCH[10]}" mode=require
  case "$q" in *sslmode=*) mode="${q#*sslmode=}"; mode="${mode%%&*}" ;; esac
  export PGSSLMODE="$mode"
}

# auth_header <file>: a 0600 header file holding the bearer from <file>, for curl -H @path.
# printf is a builtin, so the key is never in any process's arguments.
auth_header() {
  local h="$GW_TMP/auth.h"
  (umask 077; printf 'Authorization: Bearer %s\n' "$(head -n1 "$1")" >"$h")
  echo "$h"
}

# probe_pod <name> <script>: a pod in panacea-sandboxes shaped as a session is, so phase 8's
# policies select it as they select a session: the sandbox label, the Kata RuntimeClass, the
# sandbox toleration and node selector, restricted, sized, no service-account token.
probe_pod() {
  local sel="${GW_SANDBOX_NODE_SELECTOR:-panacea-app/role=sandbox}"
  cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $1
  namespace: panacea-sandboxes
  labels: {panacea-app/component: sandbox, panacea-app/smoke: "true"}
spec:
  runtimeClassName: $GW_RUNTIME_CLASS
  restartPolicy: Never
  automountServiceAccountToken: false
  nodeSelector: {"${sel%%=*}": "${sel#*=}"}
  tolerations: [{key: panacea-app/sandbox, operator: Equal, value: "true", effect: NoSchedule}]
  securityContext: {runAsNonRoot: true, runAsUser: 1001, seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: probe
      image: $GW_SANDBOX_IMAGE
      command: ["sh", "-c"]
      args:
        - |
$(printf '%s\n' "$2" | sed 's/^/          /')
      env: [{name: POD_IP, valueFrom: {fieldRef: {fieldPath: status.podIP}}}]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}
      resources: {requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 250m, memory: 256Mi}}
EOF
}

# run_probe <key> <name> <script>: apply, wait for Succeeded, print the logs; delete on EXIT.
run_probe() {
  local k=$1 name=$2
  GW_CLEANUP+=("run_cmd $k.delete kubectl delete pod -n panacea-sandboxes $name --ignore-not-found --wait=false")
  probe_pod "$name" "$3" >"$GW_TMP/$name.yaml"
  run_cmd "$k.run" sh -c "kubectl apply -f '$GW_TMP/$name.yaml' >/dev/null && kubectl wait -n panacea-sandboxes \
    --for=jsonpath='{.status.phase}'=Succeeded pod/$name --timeout=180s" >/dev/null || return 1
  run_cmd "$k.logs" kubectl logs -n panacea-sandboxes "$name"
}
```

- [ ] **Step 5: Write the six scripts**

Each starts `#!/usr/bin/env bash`, sources `lib.sh` from its own directory (`. "$(dirname "$0")/lib.sh"`), and ends `exit_status`. Those three lines are omitted below.

`infra/smoke/gateway/p01-no-public-route.sh`:

```bash
# P01: nothing public leads to the gateway (task 19: no HTTPRoute, no Ingress; P6-D1).
need P01 GW_ZONE || exit_status
if ! run_cmd p01.ns kubectl get namespace panacea-gateway -o name >/dev/null; then
  verdict P01 FAIL "namespace panacea-gateway is absent or unreadable: nothing was checked"; exit_status
fi
problems=()
routes=$(run_cmd p01.routes kubectl get httproute,ingress -n panacea-gateway -o name) \
  || { verdict P01 FAIL "could not list routes in panacea-gateway: $routes"; exit_status; }
[ -n "$routes" ] && problems+=("in namespace: $(echo $routes)")
svcs=$(run_cmd p01.svctypes kubectl get svc -n panacea-gateway \
  -o jsonpath='{range .items[*]}{.metadata.name}={.spec.type}{"\n"}{end}') \
  || { verdict P01 FAIL "could not list services"; exit_status; }
open=$(grep -Ev '=ClusterIP$' <<<"$svcs" | grep -v '^$')
[ -n "$open" ] && problems+=("services: $(echo $open)")
refs=$(run_cmd p01.backendrefs kubectl get httproute -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}:{range .spec.rules[*].backendRefs[*]}{.namespace},{end}{"\n"}{end}') \
  || { verdict P01 FAIL "could not list routes cluster-wide"; exit_status; }
into=$(grep -E ':(.*,)?panacea-gateway,' <<<"$refs" | cut -d: -f1)
[ -n "$into" ] && problems+=("routes into the namespace: $(echo $into)")
for name in gateway litellm; do
  if run_cmd "p01.dns.$name" getent hosts "$name.$GW_ZONE" >/dev/null; then
    problems+=("$name.$GW_ZONE resolves")
  fi
done
if [ "${#problems[@]}" -eq 0 ]; then
  verdict P01 PASS "no route, no public Service, no name; services seen: $(echo $svcs)"
else
  verdict P01 FAIL "${problems[*]}"
fi
```

`infra/smoke/gateway/p02-sandbox-reaches-gatekeeper-only.sh`:

```bash
# P02: from a pod shaped as a session, the gatekeeper connects and nothing else in the gateway does.
# "Connects" is TCP (curl's time_connect > 0), whatever the HTTP answer: phase 8's rule that a
# status code must never be read as reachability (task 31, S50.2).
need P02 GW_SANDBOX_IMAGE GW_RUNTIME_CLASS GW_DB_HOST || exit_status
pinned P02 GW_SANDBOX_IMAGE || exit_status
can_i P02 p02.cani create pods panacea-sandboxes || exit_status
ns=panacea-gateway.svc.cluster.local
script="for t in gatekeeper=http://gatekeeper.$ns:4000/healthz litellm=http://litellm.$ns:4001/ \
keysmith=http://keysmith.$ns:4100/healthz postgres=http://$GW_DB_HOST:5432/; do
  tc=\$(curl -s -o /dev/null --connect-timeout 3 -m 4 -w '%{time_connect}' \"\${t#*=}\"); rc=\$?
  echo \"\${t%%=*} \$rc \$tc\"; done"
out=$(run_probe p02 "gwsmoke-p02-$RANDOM" "$script") || { verdict P02 FAIL "probe pod did not complete"; exit_status; }
connected() { awk -v n="$1" '$1==n && $3+0 > 0 {f=1} END {exit !f}' <<<"$out"; }
problems=()
connected gatekeeper || problems+=("gatekeeper:4000 did not connect")
for t in litellm keysmith postgres; do connected "$t" && problems+=("$t connected"); done
if [ "${#problems[@]}" -eq 0 ]; then verdict P02 PASS "$(echo $out)"; else verdict P02 FAIL "${problems[*]}; saw: $(echo $out)"; fi
```

`infra/smoke/gateway/p03-pod-source-address.sh` (decision (a)):

```bash
# P03: the source address a gateway pod sees is the sandbox pod's own (open risk 2; else seam S6.10).
need P03 GW_SANDBOX_IMAGE GW_RUNTIME_CLASS GW_ECHO_IMAGE || exit_status
pinned P03 GW_SANDBOX_IMAGE || exit_status
pinned P03 GW_ECHO_IMAGE || exit_status
can_i P03 p03.cani.sandboxes create pods panacea-sandboxes || exit_status
can_i P03 p03.cani.gateway create networkpolicies panacea-gateway || exit_status
can_i P03 p03.cani.sbnetpol create networkpolicies panacea-sandboxes || exit_status
y="$GW_TMP/echo.yaml"
cat >"$y" <<EOF
apiVersion: v1
kind: Pod
metadata: {name: gwsmoke-p03-echo, namespace: panacea-gateway, labels: {panacea-app/smoke: "true", panacea-app/probe: p03-echo}}
spec:
  automountServiceAccountToken: false
  securityContext: {runAsNonRoot: true, runAsUser: 1001, seccompProfile: {type: RuntimeDefault}}
  containers:
    - {name: echo, image: $GW_ECHO_IMAGE, args: [netexec, --http-port=8080],
       securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}},
       resources: {requests: {cpu: 50m, memory: 32Mi}, limits: {cpu: 100m, memory: 64Mi}}}
---
apiVersion: v1
kind: Service
metadata: {name: gwsmoke-p03-echo, namespace: panacea-gateway, labels: {panacea-app/smoke: "true"}}
spec: {selector: {panacea-app/probe: p03-echo}, ports: [{port: 8080, targetPort: 8080}]}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: gwsmoke-p03-echo-in, namespace: panacea-gateway, labels: {panacea-app/smoke: "true"}}
spec:
  podSelector: {matchLabels: {panacea-app/probe: p03-echo}}
  policyTypes: [Ingress]
  ingress: [{from: [{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: panacea-sandboxes}},
             podSelector: {matchLabels: {panacea-app/smoke: "true"}}}], ports: [{port: 8080}]}]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: gwsmoke-p03-echo-out, namespace: panacea-sandboxes, labels: {panacea-app/smoke: "true"}}
spec:
  podSelector: {matchLabels: {panacea-app/smoke: "true"}}
  policyTypes: [Egress]
  egress: [{to: [{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: panacea-gateway}},
            podSelector: {matchLabels: {panacea-app/probe: p03-echo}}}], ports: [{port: 8080}]}]
EOF
GW_CLEANUP+=("run_cmd p03.delete kubectl delete -f '$y' --ignore-not-found --wait=false")
run_cmd p03.apply sh -c "kubectl apply -f '$y' && kubectl wait -n panacea-gateway --for=condition=Ready \
  pod/gwsmoke-p03-echo --timeout=120s" >/dev/null || { verdict P03 FAIL "echo did not become ready"; exit_status; }
script='echo "podip=$POD_IP"; echo "seen=$(curl -s -m 5 http://gwsmoke-p03-echo.panacea-gateway.svc.cluster.local:8080/clientip)"'
out=$(run_probe p03 "gwsmoke-p03-$RANDOM" "$script") || { verdict P03 FAIL "probe pod did not complete"; exit_status; }
podip=$(sed -n 's/^podip=//p' <<<"$out"); seen=$(sed -n 's/^seen=//p' <<<"$out")
ip=${seen%:*}; ip=${ip#[}; ip=${ip%]}
if [ -z "$seen" ] || [ -z "$podip" ]; then
  verdict P03 FAIL "no reply from the echo (raw '$seen', pod ip '$podip')"
elif [ "$ip" = "$podip" ]; then
  verdict P03 PASS "the echo saw $ip, the probe pod's own address (raw $seen)"
else
  verdict P03 FAIL "the echo saw $ip and the pod is $podip: the source is rewritten on this path, so pod binding needs seam S6.10 (mTLS)"
fi
```

The two temporary policies select **only** pods labelled `panacea-app/smoke: "true"`, so no session gains a path for the seconds P03 runs, and both are deleted by the `EXIT` trap. The probe measures pod → ClusterIP Service → pod across the two namespaces, which is the gatekeeper's path; if phase 8 puts anything between them later (a mesh, an egress gateway), P03 must be re-run.

`infra/smoke/gateway/p04-au-route-bedrock.sh` (decision (b)):

```bash
# P04: a real AU session's requests succeed through LiteLLM's Bedrock translation (P6-D3; else S6.4).
# Reads the pod's launch environment and the gatekeeper's lines for the session; never the CLI (OD-8).
need P04 GW_P04_SESSION GW_P04_POD || exit_status
min=${GW_P04_MIN:-5}
envs=$(run_cmd p04.env kubectl get pod -n panacea-sandboxes "$GW_P04_POD" \
  -o jsonpath='{range .spec.containers[*].env[*]}{.name}={.value}{"\n"}{end}') \
  || { verdict P04 FAIL "could not read pod $GW_P04_POD"; exit_status; }
problems=()
grep -qx 'CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1' <<<"$envs" \
  || problems+=("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 is missing from the pod")
for v in CLAUDE_CODE_USE_BEDROCK ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN; do
  grep -q "^$v=" <<<"$envs" && problems+=("$v is set in the pod")
done
grep -q '^AWS_' <<<"$envs" && problems+=("an AWS_ variable is set in the pod")
logs=$(run_cmd p04.logs kubectl logs -n panacea-gateway -l "${GW_GATEKEEPER_SELECTOR:-app.kubernetes.io/name=gatekeeper}" \
  --since=6h --tail=-1 --prefix=false --max-log-requests=10) \
  || { verdict P04 FAIL "could not read the gatekeeper's logs"; exit_status; }
mine=$(grep -F "\"session\":\"$GW_P04_SESSION\"" <<<"$logs" | grep -F '"path":"/v1/messages"')
n=$(grep -c . <<<"$mine")
bad=$(grep -oE '"status":[0-9]+' <<<"$mine" | cut -d: -f2 | awk '$1 >= 400' | sort | uniq -c)
global=$(grep -c '"route":"global"' <<<"$mine")
[ "$n" -ge "$min" ] || problems+=("$n of at least $min /v1/messages requests logged for $GW_P04_SESSION")
[ -n "$bad" ] && problems+=("statuses >= 400 (count status): $(echo $bad); a Bedrock rejection is seam S6.4")
[ "$global" -eq 0 ] || problems+=("$global requests of this AU session left on route global")
if [ "${#problems[@]}" -eq 0 ]; then
  verdict P04 PASS "$n requests for $GW_P04_SESSION, all route=au, none >= 400"
else
  verdict P04 FAIL "${problems[*]}"
fi
```

Only variable **names** from the pod reach the output. A status of 400 from Bedrock through LiteLLM is the specific failure P6-D3 predicts (a field an Amazon Bedrock upstream rejects); the operator then reads LiteLLM's own log for that request's error type, which this script does not print because it can quote the request.

`infra/smoke/gateway/p05-zdr-config.sh`:

```bash
# P05: our stores and the AWS account keep no prompt (P6-D9).
need P05 GW_CANARY GW_DATABASE_URL_FILE || exit_status
pg_env "$GW_DATABASE_URL_FILE" || { verdict P05 "NOT RUN" "(GW_DATABASE_URL_FILE does not hold a postgres URL)"; exit_status; }
problems=()
cfg=$(run_cmd p05.config kubectl get configmap -n panacea-gateway "${GW_LITELLM_CONFIGMAP:-litellm-config}" \
  -o jsonpath='{.data.config\.yaml}') || { verdict P05 FAIL "could not read the LiteLLM ConfigMap"; exit_status; }
grep -Eq '^[[:space:]]*(success_|failure_|service_)?callbacks?[[:space:]]*:' <<<"$cfg" \
  && problems+=("a callback key is configured")
grep -Eq '^[[:space:]]*store_prompts_in_spend_logs:[[:space:]]*false' <<<"$cfg" \
  || problems+=("store_prompts_in_spend_logs is not false")
dump=$(run_cmd p05.dump pg_dump --data-only --no-owner) \
  || { verdict P05 FAIL "pg_dump of $PGDATABASE on $PGHOST failed: $(head -c 200 <<<"$dump")"; exit_status; }
lines=$(grep -c . <<<"$dump"); hits=$(grep -cF "$GW_CANARY" <<<"$dump")
[ "$hits" -eq 0 ] || problems+=("the canary is in the database dump on $hits lines: a blocker for OD-19")
for r in ap-southeast-2 ap-southeast-4; do
  out=$(run_cmd "p05.bedrock.$r" aws bedrock get-model-invocation-logging-configuration --region "$r" --output json) \
    || { problems+=("could not read Bedrock invocation logging in $r"); continue; }
  grep -q '"loggingConfig"' <<<"$out" && problems+=("Bedrock model-invocation logging is configured in $r")
done
if [ "${#problems[@]}" -eq 0 ]; then
  verdict P05 PASS "no callback; prompts not stored; canary absent from a $lines-line dump; invocation logging off in both regions"
else
  verdict P05 FAIL "${problems[*]}"
fi
```

The canary is one the operator sent through a real session shortly before (task 23's step 9 is the local model). The ConfigMap is what the running pods mount; task 19's chart rolls the pods on a ConfigMap change (a checksum annotation), so the two agree unless a pod was not rolled, which P05 does not detect. `get-model-invocation-logging-configuration` and its `{}` reply when nothing is configured are **training knowledge, UNVERIFIED**; the script prints nothing from the reply, so a different shape shows as a PASS only if it lacks `loggingConfig`, which the operator confirms by hand once.

`infra/smoke/gateway/p06-budgets-on.sh`:

```bash
# P06: budgets are on by default (OD-24, P6-D8). A new project's LiteLLM team and a new person's
# LiteLLM user carry a positive max_budget, and keysmith sets no default to zero.
need P06 GW_LITELLM_URL GW_MASTER_KEY_FILE GW_PROJECT GW_PERSON || exit_status
[ -s "$GW_MASTER_KEY_FILE" ] || { verdict P06 "NOT RUN" "(GW_MASTER_KEY_FILE is empty or unreadable)"; exit_status; }
h=$(auth_header "$GW_MASTER_KEY_FILE")
budget_of() { grep -oE '"max_budget":[[:space:]]*(null|[0-9.eE+-]+)' <<<"$1" | head -n1 | sed 's/.*:[[:space:]]*//'; }
positive() { awk -v b="$1" 'BEGIN { exit !(b != "null" && b + 0 > 0) }'; }
problems=()
team=$(run_cmd p06.team curl -s -m 10 -H @"$h" "$GW_LITELLM_URL/team/info?team_id=$GW_PROJECT") \
  || problems+=("team/info did not answer")
tb=$(budget_of "$team"); positive "$tb" || problems+=("team $GW_PROJECT max_budget is ${tb:-absent}")
user=$(run_cmd p06.user curl -s -m 10 -H @"$h" "$GW_LITELLM_URL/user/info?user_id=$GW_PERSON") \
  || problems+=("user/info did not answer")
ub=$(budget_of "$user"); positive "$ub" || problems+=("user $GW_PERSON max_budget is ${ub:-absent}")
envs=$(run_cmd p06.keysmith.env kubectl get deploy -n panacea-gateway keysmith \
  -o jsonpath='{range .spec.template.spec.containers[*].env[*]}{.name}={.value}{"\n"}{end}')
zero=$(grep -E '^PANACEA_APP_GATEWAY_(SESSION_CAP|DEFAULT_(USER|TEAM|PROJECT)_BUDGET)_USD=(0+(\.0*)?)?$' <<<"$envs" | cut -d= -f1)
[ -z "$zero" ] || problems+=("keysmith sets $(echo $zero) to zero or empty")
if [ "${#problems[@]}" -eq 0 ]; then
  verdict P06 PASS "team $GW_PROJECT US\$$tb, user $GW_PERSON US\$$ub, keysmith defaults non-zero"
else
  verdict P06 FAIL "${problems[*]}"
fi
```

`GW_PROJECT` and `GW_PERSON` must be a project created and a person signed in **after** the gateway was deployed, and each must have minted at least one key (task 07 calls `EnsureTeam` and `EnsureUser` at mint), or LiteLLM has no team or user to report. The `/team/info` and `/user/info` paths and the `max_budget` key are LiteLLM's documented admin API, **UNVERIFIED** against the pinned image until task 18's P03 and P04; `budget_of` takes the first `max_budget` wherever it is nested.

`infra/smoke/gateway/run.sh`:

```bash
#!/usr/bin/env bash
# Every gateway proving script in order. Exits non-zero if any printed FAIL. Runs nothing in CI.
here=$(cd "$(dirname "$0")" && pwd)
rc=0
for s in "$here"/p0[1-6]-*.sh; do bash "$s" || rc=1; done
exit "$rc"
```

- [ ] **Step 6: Run the tests to verify they pass**

```bash
bash infra/smoke/gateway/gateway_smoke_test.sh | tail -1        # expect: gateway smoke tests: 0 failed
bash infra/smoke/gateway/gateway_smoke_test.sh | grep -c '^ok'  # expect 36
command -v shellcheck >/dev/null && shellcheck -x infra/smoke/gateway/*.sh   # expect nothing, if installed
```

- [ ] **Step 7: Kill four mutants by hand**

Each edit, made alone and reverted, must turn the named case red:

1. In `p02`, `connected()` testing `$2 == 0` (curl's exit status) instead of `$3+0 > 0` → "P02 fails when Postgres connects and answers nothing". It is the only case that kills this mutant, which is why it exists.
2. In `p03`, compare `"$seen" = "$podip"` (with the port) → "P03 passes when the echo sees the pod's own address".
3. In `p04`, drop the `[ "$n" -ge "$min" ]` line → "P04 fails with too few requests".
4. In `p06`, `verdict P06 PASS "… $(cat "$GW_MASTER_KEY_FILE")"` → "P06 prints no key".

- [ ] **Step 8: Commit**

```bash
git add infra/smoke/gateway/lib.sh infra/smoke/gateway/run.sh infra/smoke/gateway/p0*.sh infra/smoke/gateway/gateway_smoke_test.sh
git commit -F <message file> -- infra/smoke/gateway
```

Message: `test(smoke): gateway proving scripts P01 to P06, tested on canned output, not run (phase 6 task 24)`, a body naming decisions (a) to (e), and the attribution line.

**Acceptance:** step 6 prints `gateway smoke tests: 0 failed` and `36`; step 7's four edits each turn the named case red; `grep -rn 'kubectl exec\|--admin' infra/smoke/gateway/` prints nothing (no exec into a session, no admin credential).

**When the environment exists** (not in this phase), the operator runs, from an allowlisted workstation signed in through `kubelogin` as a member of the cluster admin group:

```bash
GW_REQUIRE_ALL=1 GW_ZONE=<zone> GW_SANDBOX_IMAGE=<image@sha256:…> GW_RUNTIME_CLASS=<class> \
  GW_DB_HOST=<fqdn> GW_ECHO_IMAGE=<agnhost@sha256:…> GW_P04_SESSION=<id> GW_P04_POD=<pod> \
  GW_CANARY=<canary> GW_DATABASE_URL_FILE=<file> GW_LITELLM_URL=http://127.0.0.1:4001 \
  GW_MASTER_KEY_FILE=<file> GW_PROJECT=<id> GW_PERSON=<tid:oid> bash infra/smoke/gateway/run.sh
```

and records each line in phase 8's first-apply results (`docs/verification/phase-8-first-apply-results.md`, phase 8 task 31 step 9) or a phase 6 cloud record beside it.

**What this task does not claim:**

- **That any script has run.** None has. Every verdict logic is tested on canned output only; every command, flag and reply shape it parses (`kubectl` jsonpath, `getent`, curl's `time_connect`, `agnhost netexec /clientip`, `aws bedrock get-model-invocation-logging-configuration`, LiteLLM's `/team/info` and `/user/info`) is **UNVERIFIED** against the real tools until the first run.
- **P03's answer.** Whether AKS with phase 8's CNI preserves the pod's address on pod → Service → pod is exactly what the script measures; nothing here predicts it. A FAIL moves pod binding to seam S6.10.
- **P04's answer.** Whether LiteLLM's Anthropic-to-Bedrock translation carries what Claude Code sends is open risk 1; a FAIL moves AU sessions to seam S6.4.
- **The upstreams' retention.** P05 checks our config, our database and the AWS account's invocation logging. Anthropic's and AWS's ZDR terms are the org's contracts (owner question Q3).
- **Budget enforcement.** P06 proves budgets are set, not that they bite; biting is task 18's P03 to P05 locally, and task 23's item 5.

**Gaps against the skeleton, for the orchestrator:**

1. **Phase 8's task 32 proving list should carry P02 and P03.** Both create pods in `panacea-sandboxes` on the human path, as its S50 does; running them in the same session costs one sign-in.
2. **P03 needs a temporary egress policy in `panacea-sandboxes`** selecting only smoke-labelled pods. That is a write to phase 8's namespace policy set, however brief; phase 8 should say it accepts it, or supply its own probe path.
3. **Task 19's chart must label the gatekeeper `app.kubernetes.io/name=gatekeeper`** (or P04's `GW_GATEKEEPER_SELECTOR` default changes), and name the LiteLLM ConfigMap `litellm-config` (or `GW_LITELLM_CONFIGMAP`'s default changes).
4. **The skeleton lists `*_test.sh` per script.** This part uses one `gateway_smoke_test.sh` for all six, which matches the glob and phase 8's single `smoke_test.sh`; say if one file per script is wanted.
5. **P06 needs a mint to have happened** for the fresh project and person (LiteLLM creates the team and user at mint, task 07), so "a new project gets non-zero budgets" is checked after its first session, not at creation. Checking at creation would need the Hub's H-2 policy read directly, which is phase H's.
