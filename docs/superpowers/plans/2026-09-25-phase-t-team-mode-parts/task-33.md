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
