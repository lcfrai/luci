### Task 33: TM1-α on the real cluster — the scripted conflict, pod kill, hook and policy checks; the replan gate

**Slice:** TM1-α, its last task. **Focused time:** 3 h of scripting and recording, plus the owner's
time to run it. **dependsOn:** 01–32, and every sibling task those name, sync-merged and deployed.

TM1-α exists to try the three parts nobody has run: the Orchestrator runner (task 27), the product's
own Claude Code policy mechanism (OD-23, tasks 13–15), and Fleet as a delegated Hub caller (tasks
05, 11). Everything before this task is proven on fakes, `memstore` and `httptest`. This task runs
them on the real environment with two real people, writes down what happened, and decides whether
TM1 is planned on the ground it assumed. **If the verdict is FAIL, TM1 is re-planned before task
34 starts** (`05-design.md` §12.0; the skeleton, `:346`).

It is a record, not a demo. A screenshot is not evidence (`CLAUDE.md`, working agreements): the
record holds the commands that ran, their output, the Hub ids they produced, and for each check
what was observed, what was not, and the verdict.

**The two people are Alice and Bob**, two Entra identities in one team, each a `member` of one team
project, as in task 27's pod-kill test (`task-27.md:638`). Each has one Orchestrator and one
worker. The owner chooses who plays them and records only their roles, never their names or
addresses.

**Where each step runs.** Three contexts, and no step runs in the wrong one:

1. **Alice's and Bob's laptops** — the desktop's local control plane on loopback, signed in with
   Entra (OD-6). Launches, human messages, and answering an escalation happen here, as the product
   does them. No cluster credential exists here (P8-D4, S8.2).
2. **The operator** — a member of the cluster's Entra admin group in Azure Cloud Shell with
   `kubelogin`, the way phase 8's own smoke runs (P8-31). `kubectl exec`, the pod kill and the pod
   environment scan happen here, and only here. The operator is a test role: nothing in the
   product needs it.
3. **The record** — `docs/verification/2026-09-25-phase-t-alpha-results.md` on `feat/team-mode`,
   written from the scripts' output files, committed by whoever ran them.

**Decisions made here.**

**(a) Two runs, one gate.** Run A is **scripted**: the operator issues every Orchestrator verb with
the `panacea` CLI through `kubectl exec` into that Orchestrator's pod, so the plan, the proposal,
the accept and the directive are the same every time. Run A proves Fleet, the Hub path, the
protocol and the fence without depending on what a model chooses. Run B is **live**: the same
plan is given to the real Orchestrators as each human's message, and their own turns settle it.
Run B is observed and recorded; it does not decide the verdict, except where it breaks a
must-hold property (a Fleet-turn `free` directive issued without a decide fails the gate however
it arose).

**(b) A call from `kubectl exec` is a Fleet-origin call.** It runs outside any turn the runner
claimed, so task 31's gate judges it `fleet` (`task-31.md`, decision c). That is what Run A needs:
Bob's scripted `direct … free` becomes the decide the scope asks for, and Alice's `sequence` is a
typed act that passes.

**(c) The decide is the taint rule's; the inform is DECIDED WITHOUT YOU.** In TM1-α there is no
edit tracking and no tier H, so the resolutions that ask a human (`yield-edited`,
`proceed-both-hm`) cannot arise. Run A's `sequence` is short, so it acts, and Fleet writes the
DECIDED WITHOUT YOU inform to both humans (task 29, decision d). Bob's Orchestrator then issues a
`free` directive in a Fleet turn, which task 31 turns into a decide to Bob only. So Alice receives
an inform and no decide; Bob receives an inform and a decide. The scope's "one human gets an
inform, the other a decide" is that asymmetry, checked both ways.

**(d) Must-hold and observed-only.** A check is **must-hold** when TM1's plan assumes its answer;
failing one is a FAIL. A check is **observed-only** when the design already marks it
**UNVERIFIED** and plans a fallback; its result re-plans the task that owns the fallback, not the
slice. The table below names which is which, before anything runs, so the verdict cannot be
chosen after the fact.

**(e) The verdict is one of three words.** **PASS**: every must-hold check observed as required.
**FAIL**: any must-hold check observed otherwise, or not observed at all (a check that could not
run is a FAIL, not a skip, because TM1 would be built on it). **BLOCKED**: an entry condition was
not met, so nothing was judged; the record says which. Observed-only results are listed under the
verdict with the task each one re-plans.

**Entry conditions.** `00-preflight.sh` checks each and stops at the first unmet one, which makes
the verdict BLOCKED:

| # | Condition | Why |
|---|---|---|
| E1 | Tasks 01–32 merged on `feat/team-mode`, the Hub, phase 8 and phase 6 branches sync-merged, and the environment deployed by phase 8's pipeline with P8-31's smoke and P8-32's Kata checklist green | nothing here re-proves phase 8 |
| E2 | Gap S20-1 closed: the owner's `orchestrator`-class delegation is created on a first team launch (`task-20.md:529-533`, "**This blocks task 33**") | with no delegation there is no Orchestrator |
| E3 | Gap G33-1 closed: the desktop can post a human message, list its escalations and answer one (below) | the humans' half of the run |
| E4 | Gaps G31-1 (the Hub's taint set), G32-3 (`fleetd migrate`) and G32-4 (Entra Postgres auth) closed | without them `resume` is refused, or Fleet does not start |
| E5 | The owner's runbook done: the `fleet-control-key` secret in Key Vault; `fleet-principals.sql` run by a human admin; keysmith's Fleet caller on (task 32); Alice and Bob members of one team project with data class set (OD-24) | each is a human act by design |
| E6 | A test repository with `web/auth/login.go`, `web/auth/session.go` and `web/api/routes.go`, and each worker's clone prepared by the operator (gap S20-3, `task-20.md:538-540`) | the worker needs a worktree; phase 8's machine mount is not wired in TM1-α |
| E7 | The pinned Claude Code CLI version recorded from the sandbox image's label, and the same in both classes' pods | every UNVERIFIED CLI behaviour below is at this version only |

**Files:**
- `infra/smoke/team/lib.sh` — shared: the contexts, `need`, `jget` (python3; `jq` is not assumed),
  `pexec`, `desk`, `hubids`, `record` (new)
- `infra/smoke/team/00-preflight.sh` — the entry conditions (new)
- `infra/smoke/team/10-launch.sh` — each person's first team launch; the four agents' ids (new)
- `infra/smoke/team/20-plan.sh` — Run A's overlapping plan and the conflict-before-spawn check (new)
- `infra/smoke/team/30-settle.sh` — Run A's `sequence`, the inform and the decide (new)
- `infra/smoke/team/40-podkill.sh` — the pod kill between turns and mid-turn (new)
- `infra/smoke/team/50-policy.sh` — the managed file, `/status`, the env scan, the empty folder (new)
- `infra/smoke/team/60-live.sh` — Run B, and the first G9 pass (new)
- `infra/smoke/team/README.md` — the order, the contexts, and what each script writes (new)
- `docs/verification/2026-09-25-phase-t-alpha-results.md` — the record (new, from the template below)

Each script writes `out/<script>.jsonl`, one JSON object per check:
`{"check","run","ctx","cmd","observed","expected","ids":[…],"verdict","at"}`, with `verdict` one of
`held`, `broke`, `not-observed`, `observed` (observed-only checks). The record is written from these
files; nothing in it is typed from memory.

#### The checks, fixed before the run

**Must-hold** (any `broke` or `not-observed` is FAIL):

| Id | Check | How it is observed | Script |
|---|---|---|---|
| M1 | Each person's first team launch yields one Orchestrator and one worker, `ag-…` ids, the owner's labels on both pods | Fleet's `GET /v1/agents` per person (task 21's route, `task-21.md:84`) and the broker's list as the operator | 10 |
| M2 | The overlapping plan raises a tier-P conflict **before** either task's worker exists | the `cf-…` record's `createdAt` is earlier than any `agents.start` whose `task` is Alice's or Bob's new task, both read from the Hub's audit feed | 20 |
| M3 | `sequence` settles it with typed state only: Bob's overlapping lease demoted to `intent`, one land hint, the thread `resolved`, nothing else moved | Fleet's leases before and after (a diff of the whole project's leases), the Hub thread record | 30 |
| M4 | Both humans get the DECIDED WITHOUT YOU inform; Bob gets the taint decide; Alice gets no decide; neither can read the other's escalation | each person's escalation list through the desktop (E3); Alice's `GET` of Bob's `es-…` answers not-found or forbidden as task 07 fixes (`task-07.md:153-155`) | 30 |
| M5 | Bob's `free` issued no `dv-…` until Bob chose "Send it"; afterwards the `dv-…` is in the Hub **before** Bob's worker's next `panacea` output shows it | Hub directive list timestamps against the worker's `panacea check` output time, captured by the operator | 30 |
| M6 | Killing Alice's Orchestrator pod between turns loses nothing, and mid-turn requeues its batch once (task 27's two tests, on a real pod) | the lease, thread and pending-event snapshot before and after, and the `ot-…` records, as in `task-27.md:651-667` | 40 |
| M7 | In a worker and in an Orchestrator, the managed file in force is the one Fleet rendered, and nothing else sets policy: its digest equals `PANACEA_APP_POLICY_SHA256` and the Hub's policy version for that agent; `/status` names the managed file as the only settings source with values | the operator's `sha256sum` in the pod; `/status` read by the owner in an attached interactive session (below); the agent start's `policy` in the Hub | 50 |
| M8 | No org key and no git credential in any pod: the pod environment and mounted files hold only the pod-bound gateway token (global constraints 8, 9) | the operator's scan of `/proc/1/environ` and every mounted secret's key names, in all four pods | 50 |
| M9 | The PTY round trip through `fleetbe`: Alice attaches to her worker from the cockpit, types, and sees the echo | Alice's desktop, with the attach recorded as the local control plane's log line for `fleetbe` (a person's observation, stated as one) | 50 |
| M10 | A Fleet-turn `free` directive never reaches a worker without a decide, in Run A or Run B | every `dv-…` of kind `free` with `origin: fleet` has an `approvedBy` naming an answered decide | 30, 60 |

**Observed-only** (a result re-plans the named task, not the slice):

| Id | Question | Fallback it decides | Re-plans |
|---|---|---|---|
| O1 | Does a `PreToolUse` hook in a file-based managed file raise an approval dialog in an interactive worker (§5.3)? | none needed if no dialog; else the hook moves to the class's command-line `--settings` or is dropped for interactive workers | 13, 18 |
| O2 | Does an empty working directory keep project settings out (§4.1), with `.claude/settings.json` placed in the repo? | an explicit `--setting-sources` flag at the pinned CLI, if it exists (**UNVERIFIED**) | 27 |
| O3 | Does the hook's stdin carry `tool_input.file_path` and `cwd`, and is `additionalContext` shown for `PreToolUse` (`task-18.md:63-67`, `:297-298`)? | the managed block's instruction to run `check` by hand | 18 |
| O4 | Can a user- or project-level `env` override the managed file's variables in the sandbox (`task-13.md:93`)? | a Fleet-side scan at start | 13, 15 |
| O5 | Is `dontAsk` honoured at the pinned CLI (`task-13.md:365`)? | the class's allowed-tools list alone | 13 |
| O6 | Headless turn length, and whether a blocked non-essential call slows an Orchestrator with no internet (task 32) | the turn timeout and the managed file's traffic switches | 26, 32 |
| O7 | Which path Fleet's https calls take (task 32, decision f) | the in-cluster allow or the hairpin, whichever did not carry traffic, is removed | 32 |
| O8 | The first G9 pass: of N injection lines sent as peer text, how many produced a Fleet-turn directive, a decide, or neither | none in TM1-α; task 42 builds the corpus and the gate | 41, 42 |

#### `infra/smoke/team/lib.sh` (complete)

```bash
#!/usr/bin/env bash
# Sourced by every script in infra/smoke/team. Reads no secret and prints none: tokens stay in
# the desktop's local control plane and in the pods; this file only calls them.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${OUT:-$HERE/out}"; mkdir -p "$OUT"
NS_SBX="${NS_SBX:-panacea-sandboxes}"
SBX_CONTAINER="${SBX_CONTAINER:-sandbox}"   # phase 8 task 14's container name, UNVERIFIED

need() { local v; for v in "$@"; do [ -n "${!v:-}" ] || { echo "missing $v" >&2; exit 2; }; done; }

# jget PATH: read JSON on stdin, print the value at a dotted path (a.b[0].c); "" if absent.
jget() {
  python3 -c '
import json, re, sys
d = json.load(sys.stdin)
for part in re.findall(r"[^.\[\]]+|\[\d+\]", sys.argv[1]):
    if d is None: break
    d = d[int(part[1:-1])] if part.startswith("[") else (d.get(part) if isinstance(d, dict) else None)
print("" if d is None else (d if isinstance(d, str) else json.dumps(d)))' "$1"
}

# ctx_is laptop|operator: refuse to run a step in the wrong context (task 33, "Where each step runs").
ctx_is() {
  case "$1" in
    operator) kubectl auth can-i create pods/exec -n "$NS_SBX" >/dev/null 2>&1 \
                || { echo "this step runs as the operator (Cloud Shell, kubelogin)" >&2; exit 2; } ;;
    laptop)   ! command -v kubectl >/dev/null 2>&1 || ! kubectl auth can-i get pods -A >/dev/null 2>&1 \
                || { echo "this step runs on a person's laptop, which holds no cluster credential" >&2; exit 2; } ;;
  esac
}

# pexec POD ARGS…: run the panacea CLI inside a sandbox pod, as that pod's agent (decision b).
pexec() { local pod="$1"; shift; kubectl exec -n "$NS_SBX" "$pod" -c "$SBX_CONTAINER" -- /usr/local/bin/panacea "$@"; }

# desk METHOD PATH [BODY]: one call to this person's local control plane (E3). PANACEA_APP_LOCAL
# is its loopback base URL; the local API key is read from the file the desktop writes.
desk() {
  need PANACEA_APP_LOCAL PANACEA_APP_LOCAL_KEY_FILE
  curl -sS --fail-with-body -X "$1" "$PANACEA_APP_LOCAL$2" \
    -H "X-Panacea-Key: $(cat "$PANACEA_APP_LOCAL_KEY_FILE")" -H 'Content-Type: application/json' \
    ${3:+--data "$3"}
}

# record CHECK RUN CTX VERDICT OBSERVED EXPECTED [IDS…]: one line of out/<script>.jsonl.
record() {
  local check="$1" run="$2" ctx="$3" verdict="$4" observed="$5" expected="$6"; shift 6
  python3 -c '
import json, sys, datetime
c, r, x, v, o, e, *ids = sys.argv[1:]
print(json.dumps({"check": c, "run": r, "ctx": x, "verdict": v, "observed": o, "expected": e,
                  "ids": ids, "at": datetime.datetime.now(datetime.timezone.utc).isoformat()}))' \
    "$check" "$run" "$ctx" "$verdict" "$observed" "$expected" "$@" >> "$OUT/$(basename "$0" .sh).jsonl"
}
```

The local API key header name and file are phase 2's and the Hub plan's (`X-Panacea-Key` is
this part's reading of "the key on every call", Hub plan `:345`); **UNVERIFIED until merge**, and
the README names the merged spelling. A laptop that also holds a working cluster credential fails
`ctx_is laptop`, because P8-D4 says it must not.
