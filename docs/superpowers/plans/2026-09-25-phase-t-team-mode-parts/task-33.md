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
