# Phase T (team mode) plan review

Date: 2026-09-26
Plan: `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md` and its 69 task parts.

## How the review was done

- Seven section reviews, about 10 tasks each: sec-01 (tasks 01-10) to sec-07 (tasks 61-69).
  Each read its parts against the skeleton's task scopes and the sibling plans.
- One cross-cutting review (sec-00): coverage of the brief and `05-design.md` revision 3,
  dependency order across all 69 tasks, fit with the Hub and phases 3, 6 and 8, and plan-wide
  security.
- One OD-30 identity section (sec-90): where the plan still assumes Entra tokens after the
  Keycloak broker ruling.
- Every section's verdict is "Needs fixes". This file assembles them; it judges nothing new.

## Counts (from the `### ` finding headings in all nine inputs)

| Severity  | sec-00 | 01 | 02 | 03 | 04 | 05 | 06 | 07 | 90 | Total |
|-----------|-------:|---:|---:|---:|---:|---:|---:|---:|---:|------:|
| Critical  | 2      | 1  | 7  | 2  | 4  | 0  | 1  | 1  | 0  | 18    |
| Important | 8      | 20 | 27 | 14 | 23 | 16 | 18 | 19 | 8  | 153   |
| Minor     | 2      | 20 | 19 | 18 | 22 | 17 | 18 | 18 | 0  | 134   |

sec-00 consolidates some section findings (X-C1 covers S01-I8, S05-I9, S06-I18; X-C2 covers
S02-C7, S03-C1, S04-I8, S01-I9), so the totals count some defects more than once.

## The two defects a first implementer would hit (from sec-00)

1. **X-C1, at task 21 step 6.** Production `TeamProjects` reads `hubapi.Project.Mode`, which no
   task adds, and no route writes `projects.mode = 'team'`. It does not compile against the
   merged Hub, and no real project ever becomes a team project.
2. **X-C2 (G31-1), at task 31's acceptance.** Task 07 ships `fleetTurnVerbs` with three kinds;
   task 31's taint run expects five. Going in task order, task 07 commits green and task 31's
   acceptance goes red. The fix needs a section 7.2 ruling (S04-I1), not a one-line widening.

---

# Plan review, section 00: Phase T cross-cutting adversarial review (round 2)

Repair checked: repair-gaps.md (gaps outside sections 1 and 5)

Scope: coverage of the brief and every slice of `05-design.md` (revision 3), dependency order
across all 69 tasks, fit with the sibling plans (Hub, phases 3, 6, 8), plan-wide security, and the
`repair-gaps.md` gaps that fall outside sections 1 (tasks 01-10) and 5 (tasks 41-50). Section
reviews sec-01 to sec-07 and sec-90 were read by heading and verdict only; all eight say "Needs
fixes". The OD-30 identity change is sec-90's and is not duplicated here.

Note: `.superpowers/sdd/` is git-ignored, so this file is not committed.

Round 1's cross-cutting findings were re-checked against the current parts. X-C1 and X-C2 below
are round 1's X-C2 and X-C1, still open; they are restated with the current line numbers.
Round 1's X-I1, X-I2 and X-I3 are also still open (X-I1, X-I2 and X-I4 here).

Section findings consolidated here, because each section sees one piece of the defect:
X-C1 covers S01-I8, S05-I9 and S06-I18. X-C2 covers S02-C7, S03-C1, S04-I8 and S01-I9. X-I4 is
the same defect class as S07-I11 (bump-gate). The policy layer's cross-owner command path is
S02-C3 and S02-I8. It is the plan's widest cross-owner hole, and it is not repeated here.

## Dependency order and coverage

### X-C1 (Critical): nothing writes `projects.mode = 'team'`, so no real project is ever a team project
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-04.md:958
**Problem:** Task 04 adds `projects.mode` with a one-way guard and says its writer is "task 43's
or the Share task's" (`task-04.md:958-959`). Task 43 only reads it (`task-43.md:256`, 409
`team_state`). Task 06 lists the same hole as its gap 3 (`task-06.md:718-720`). Task 21 says
"Hub task 42 builds Share" and reads a `hubapi.Project.Mode` that does not exist
(`task-21.md:361-366`). But Hub task 42 is Share's **UI** (hub plan `:445`), written before team
mode existed, and it cannot write a column that Phase T's task 04 adds. Task 45 ships
`TeamModeAllowed` and says "whichever task owns Share must call it" (`task-45.md:701-703`). Every
unit test passes on the fake's `SetProjectMode` (`task-08.md:307-310`, `:546`). The first real run
(task 33, E5 "members of one team project") has no route to create that state. Task 45's
audit-streaming gate before the first teammate is therefore never enforced.
**How to fix:** Add one Hub delta task, or widen task 06: a route `POST /projects/{id}/mode`
`{"mode":"team"}` (admin or owner only, one-way, audit event `project.mode_set`) that calls
`TeamModeAllowed` first and returns 503 `audit_not_streaming` until task 45's sink is `logs`.
Add `Mode` to `hubapi.Project` in task 03, and a `hubclient.SetProjectMode` beside the fake's.
Name its caller: task 21's first team launch, or task 56's share path. Put the task in TM1-α
before 21, and make task 33's E5 run the route rather than assume the state. Task 45 is a TM1 task,
so under TM1-α the route has to accept the `fake` sink behind a dev-only flag, and say so.

### X-C2 (Critical): task 33's replan gate has five entry conditions that no task from 01 to 32 builds
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:76
**Problem:** Task 33 depends on 01-32 (`2026-09-25-phase-t-team-mode.md:344`), but its preflight
stops BLOCKED on conditions that live in parts' gap lists, not in any task:
E2, S20-1, the owner's `orchestrator` delegation, which "blocks task 33" (`task-20.md:529-533`);
E3, G33-1, four desktop team calls that "no task before 52" gives (`task-33.md:413-414`);
E4, G31-1, where task 07's `fleetTurnVerbs` holds three kinds (`task-07.md:209-212`) and task 31
requires five (`task-31.md:145`, `:563`), so the sections for tasks 01-10 and 31-40 each see half;
E4, G32-3 and G32-4, where task 32 sets `PANACEA_APP_FLEET_MIGRATE` and `PANACEA_APP_FLEET_PG_AUTH`
(`task-32.md:217-218`) and task 12 reads neither (it migrates on start, `task-12.md:820`);
E6, S20-3, where no worker worktree exists in the cloud (`task-20.md:538-540`) and the operator
builds clones by hand. The waves (`:401-403`) put 33 last in TM1-α as if its dependencies closed
it. As written, the gate can only answer BLOCKED, and §12.0's "TM1 is re-planned" never fires on
evidence.
**How to fix:** Turn each gap into a scoped change of an existing TM1-α task, each with its test:
S20-1 into task 21 (create and renew the `orchestrator` delegation on the member's first team
launch); G33-1 into a small new task before 33 (the four calls on `cp/internal/api` and
`desktop/src/api/team.ts`, which task 52 later extends); G31-1 decided once, by the rule in §7.2,
and written into both task 07 and task 31; G32-3 and G32-4 into task 12's config table. State S20-3
in task 33's scope as the one accepted manual step. Then E2 to E4 become checks, not blockers.

### X-I1 (Important): the TM1-α gate waits on phase 3's held lanes stack, and neither the waves nor task 33 say so
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:75
**Problem:** Task 33's E1 needs "P8-32's Kata checklist green" and lists only the Hub, phase 8 and
phase 6 as sync-merged. Phase 8 task 32 is "held through" 21, 25 and 25b (env plan `:632`). Task
25b depends on "phase 3 task 24, phase 3's S8-P3.4 and S8-P3.5 (held)" (env plan `:614`). So
TM1-α, which the skeleton calls about two weeks with phase 3 only in TM1 (`:16-22`, rows 34, 48,
49), cannot pass its gate until phase 3's lanes stack is built and merged into phase 8. This is
round 1's X-I1, and it is unchanged.
**How to fix:** Either add phase 3 (P3-24, S8-P3.4, S8-P3.5) to task 33's E1 and to the TM1-α
wave note (`2026-09-25-phase-t-team-mode.md:400-406`) with its effect on the two-week figure, or
narrow E1 to the P8-32 probes TM1-α uses (RuntimeClass, isolation, cold start: not P18's land and
refresh) and say which ones are deferred to task 47.

### X-I2 (Important): task 60 and the delta table cite phase 8 tasks 20-21 for the bundle op that phase 8 moved to 21b and 25b
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-60.md:110
**Problem:** Task 60 names "phase 8 tasks 20-21 ... `machined`'s lane runner `Bundle`"
(`task-60.md:110`, `:454`), and the skeleton's delta row gives `cp/cmd/machined/*` "(tasks
20-21)" for the lane bundle export (`2026-09-25-phase-t-team-mode.md:268`). In the phase 8 plan,
bundles are task 21b (`POST /v1/bundles`, env plan `:603`) and the cloud `lanegit.Bundles` relay
is task 25b (`:614`), which is held on phase 3. Task 60's dependsOn therefore misses its real
sibling tasks, and a writer who reads 20-21 builds against routes that are not there.
**How to fix:** Change `task-60.md:110`, `:454` and skeleton row `:268` to P8-21b and P8-25b; add
both to task 60's dependsOn, and to row 60 of the TM2 table (`:383`). Check task 61's handoff to
`machined` (`task-61.md:60`) against 21b's one admitted bundle argument.

### X-I3 (Important): task 33's PTY round trip (M9) needs Fleet's attach handler, which is task 39 in TM1
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:115
**Problem:** §12.0 puts "the PTY round trip (§10.3)" in TM1-α (`05-design.md:1244-1245`), and
task 33's M9 has Alice attach to her worker from the cockpit through `fleetbe`. Task 21 says
"Fleet's attach handler is task 39 (TM1)". Until then a real `fleetd` answers 501, which
`fleetbe` maps to `ErrNotRunning`, and "task 33 ... does not need attach" (`task-21.md:90-94`).
Global constraint 11 forbids reusing phase 2's loopback handler (`2026-09-25-phase-t-team-mode.md:159-161`).
Task 21 is in section 3's range and task 33 in section 4's, so each section sees only one side.
As planned, M9 fails every time, and a FAIL at the gate re-plans TM1.
**How to fix:** Move task 39 into TM1-α (it depends only on 11, 12 and 19), add it to task 33's
dependsOn and to the TM1-α waves, and remove the "does not need attach" sentence from task 21.
The other option is to drop M9 from task 33 and record §12.0's PTY item as deferred to task 57,
with the reason.

## Security across the plan

### X-I4 (Important): keysmith trusts Fleet's word for owner, project and pod, so Fleet is an unchecked deputy for inference
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:62
**Problem:** Everywhere else a Fleet call is checked against a delegation: the Hub (ST.3, task 05)
and the broker (task 17). Keysmith is the exception. Its `kube` caller auth maps Fleet's service
account to "mint, refresh, revoke" and nothing else (phase 6 plan `:232`). Task 20 fills
`Owner{TID,OID}`, `ProjectID`, `AgentID` and `PodAddr` from Fleet's own state (`task-20.md:62-67`),
and task 32 turns the caller on with no delegation in the request (`task-32.md:93-94`, no match for
"delegation" in the part). A Fleet bug, or a compromised Fleet pod, can mint a key tagged to any
user's budget, or bind one to any pod address, its own included. The gatekeeper's pod binding
then proves only what Fleet asserted. Round 1's X-I3 is unchanged.
**How to fix:** Make S6.5 carry the delegation. Fleet sends `X-Panacea-Delegation: dl-…` on each
mint, and keysmith checks with the Hub that the delegation is live, that its owner and project
equal the request's, and that `AgentID` is one of the delegation's agents. It also takes `PodAddr`
from the broker's record, not from Fleet. This is a phase 6 delta: list it in the skeleton's delta
table under task 20, with a test that kills "mint for an owner the delegation does not name".
Apply the same rule to task 65's `bump-gate` caller (S07-I11). Then no keysmith caller other than
the broker mints on its own word.

## Fit with the sibling plans, and coverage of the brief

### X-I5 (Important): no plan deploys the gateway on the real cluster, yet TM1-α's gate runs every Orchestrator turn through it
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:75
**Problem:** Phase 6 stops before the cloud. Its task 24 is "Cloud proving scripts, written and
not run", and task 20's Bedrock federation is "never applied" (phase 6 plan, task table rows 20
and 24). The phase 8 plan never names the gateway chart (no match for `charts/gateway` or "phase
6" beyond `:219` and `:704`). Phase T's task 32 edits P6-19's chart values
(`task-32.md:93-94`) but deploys nothing, and task 33's E1 lists "phase 6 ... sync-merged", not
deployed and proven. Yet every Orchestrator turn in Run A and Run B needs a live Global route.
Task 47 and task 57's criterion 6 need a live AU route, which needs the unapplied Bedrock module.
The skeleton lists Bedrock federation as UNVERIFIED (`2026-09-25-phase-t-team-mode.md:1140-1141`)
but gives no owner for it. **UNVERIFIED:** whether phase 8's deploy pipeline (P8-30) installs
charts it does not name.
**How to fix:** Add an entry condition E8 to task 33: "the gateway is deployed on the cluster, and
P6-24's scripts pass for the Global route". Add the AU route and a P6-20 apply to task 47's
dependsOn. Raise the one-line owner question the sibling plans leave open: which plan's pipeline
first installs `deploy/charts/gateway`, and who applies P6-20.

### X-I6 (Important): task 57 keeps KPMG InfoSec approval as a gate row, which OD-23 removed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:40
**Problem:** The brief and OD-23 call for the mechanism, not a drafted policy, and the skeleton
says "KPMG InfoSec approval is no longer a gate ... Q12 and its TM1 gate are superseded"
(`2026-09-25-phase-t-team-mode.md:1131-1133`, OD-23 row `:100`). Task 57 still lists "KPMG
InfoSec has approved the product managed file (owner question Q12)" in the gate before the first
real teammate (`task-57.md:38-42`). It carries it as the verdict row `G-infosec`, typed by the
owner from InfoSec's written approval, and "any gate row not `PASS` means no real teammate is
invited" (`:498-501`, `:686`). No document is ever drafted for InfoSec to approve, so under the
plan's own rule this row stays `NOT RUN` and blocks the first teammate for ever.
**How to fix:** Delete `G-infosec` from task 57's gate list, verdict program, fixture and expected
output. Put a computed row in its place: `G-policy`, which passes when every pod in the run
reports the digest of an audited policy version (task 14's audit event, task 15's check). That
is the OD-23 mechanism, and it can be checked.

### X-I7 (Important): phase 8 defers Sentinel and audit streaming, and task 45's phase 8 half has no owner
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:261
**Problem:** The skeleton lists task 45 as a Hub delta only (`:261`). The part needs a data
collection endpoint and rule, two custom tables, Sentinel enabled on the workspace, the `azapi`
provider and two role grants (repair gaps 9-11; `task-45.md:19-23`, `:43-44`). Phase 8 rules the
opposite way twice. HA-7 says "audit anchoring to Log Analytics is later" (env plan `:270`), and
S8-P9.1 defers Sentinel to phase 9 (`:705`). Phase 8's workspace also has a daily cap (task 04,
`:565`). When the cap is reached, ingestion stops and the audit stream goes silent mid-day.
S05-M9 rates the ownership Minor. Across plans it is a gate blocker, because the first-teammate
gate needs "audit streams to Sentinel" (`task-57.md:39`), and no phase 8 task or owner decision
reverses HA-7 or S8-P9.1. The public collection endpoint (gap 11) is the only Azure data path in
the plan that has no private endpoint, against P8-D11's private-endpoint pattern (`:142`).
**How to fix:** Add a phase 8 delta row for task 45 to the skeleton's table: the DCE with a
private link scope, the DCR, the two tables, Sentinel on the workspace, and the role grants. Mark
it as reversing HA-7 and S8-P9.1 for team projects, and raise both, with the `azapi` question, as
one owner question. Exempt the audit tables from the daily cap, or alert before the cap is hit, so
that capped ingestion shows as `audit_not_streaming` and never as silence.

### X-I8 (Important): the totals and "about two weeks" ignore the sizing the sections found in about 24 parts
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:394
**Problem:** The skeleton puts TM1-α at "95 hours, about 12 focused days" (`:394`) and "about
two weeks" (`:16`). Across the seven sections, 21 findings say a part is two to four tasks' work.
They are S01-I12, S01-I18, S02-I11, S02-I13, S02-I18, S02-I23, S02-I27, S04-I3, S04-I5, S04-I18,
S04-M17, S05-I3, S05-M5, S05-M12, S05-M16, S06-M1, S06-I4, S07-I7, S07-I9, S07-I13 and S07-I16.
They cover about 24 parts, and at least ten of those parts are in TM1-α, including every link of
the serial Hub chain from 04 to 08 (`:403`). No single section can add these up. The house style
bounds a task at 1-3 hours, so the task list, the dependsOn columns and the waves are all built
on units that do not exist. Add X-I1's phase 3 hold and X-I5's missing gateway deployment, and the
two-week figure has no support.
**How to fix:** Split each flagged part at the seam its section names (for example, Hub delta
versus Fleet half). Renumber with letter suffixes (06a, 06b) so that no dependsOn reference
moves. Recompute the totals. Restate TM1-α's duration as a range that counts its sibling waits
(Hub through H34, P8-32, P3-24, and the gateway's first deployment). Say which of those
the design's "assumes TM0 has landed" (`05-design.md:1276`) already covers.

### X-M1 (Minor): task 17 cites a test file that does not exist, in a package phase 8 renames (repair gap 2)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:107
**Problem:** `task-17.md:107` and `:236` modify `control-plane/internal/entra/verify_test.go`. The
env tree's `internal/entra` has `entra_test.go`, `jwks_test.go`, `middleware_test.go` and
`verify.go`, but no `verify_test.go` (checked). Phase 8 task 17's step 0 also renames
`internal/entra` to `internal/oidcauth` before any broker code (env plan `:593`). The gap is
still open. sec-90 covers the OD-30 identity change; this is only the file citation.
**How to fix:** Point both lines at the renamed package's test file after P8-17 step 0, and mark
the path UNVERIFIED until merge.

### X-M2 (Minor): Fleet is single-org in three parts, and the plan never says so (repair gap 4)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:714
**Problem:** Three parts assume one org. S01-M4 finds Fleet serves only the org enrolled for its
home tenant. `task-12.md:709-716` caches one Hub token per scope. S07-I14 finds the gateway caller
scoped to one org. OD-5 makes this a we-host SaaS, but no global constraint says whether one
`fleetd` serves one org or many. Each part therefore makes its own choice, and a second org's
first team project fails in whichever part guessed differently. Whether a token per scope is
correct under OD-30's one realm is **UNVERIFIED** here.
**How to fix:** Add one line to the skeleton's global constraints: "TM1 and TM2 run one Fleet per
org" (or the opposite). Then align tasks 01, 12 and 66 to it.

## Repair gaps outside sections 1 and 5: disposition

- **Gap 2** (task 17's `verify_test.go`): still open, X-M1.
- **Gap 3** (no real Entra app-only token check for Fleet): dropped. OD-30 replaces Fleet's Entra
  app identity with a Keycloak service client, and sec-90 (ID-I1 to ID-I4) owns that change.
- **Gap 4** (one token per scope, one tenant): still open as a plan-wide assumption, X-M2.
- **Gap 8** (no writer for team mode): still open, X-C1.
- **Gaps 9, 10 and 11** (task 45's phase 8 half, `azapi`, the public collection endpoint): still
  open, and more severe than S05-M9 rates them once phase 8's HA-7 and S8-P9.1 rulings are read
  (X-I7).
- Gaps 1, 5, 6, 7, 12 and 13 are in sections 1 and 5 (tasks 01-08 and 44) and are not re-judged
  here.

## The two defects a first implementer would hit

1. **X-C1, at task 21 step 6.** The production `TeamProjects` reads `hubapi.Project.Mode`. No task
   adds that field (`task-21.md:361-366`; `hubapi.Project` at hub `types.go:140` has only
   `TeamID`), and no route anywhere writes `projects.mode = 'team'`. The code does not compile
   against the merged Hub. Every later real-Hub run then finds no team project, because
   only the fake's `SetProjectMode` ever made one.
2. **X-C2 (G31-1), at task 31's acceptance.** Task 07 ships `fleetTurnVerbs` with three kinds
   (`task-07.md:209-212`), and task 31's Hub-side taint run expects five. The part says so itself:
   "the `fleet resume` and `fleet rebase` rows fail ... the fix is task 07's"
   (`task-31.md:535-536`). An implementer who goes in task order commits task 07 green, then meets
   a red acceptance in task 31. Resolving it needs a §7.2 ruling (S04-I1: a Fleet-turn `resume`
   can undo a decide's safe-default pause), not a one-line widening.

## Summary

2 Critical (X-C1, X-C2), 8 Important (X-I1 to X-I8), 2 Minor (X-M1, X-M2). All five of round
1's cross-cutting findings are still open. The brief's named deltas each have a task: the Hub
records, the amendment and role-to-verb (01-08, 43); phase 8's Orchestrator class, `/control`,
per-class policy files and private gateway path (15, 16, 13 with 15, and 32); and phase 3's
observed activity and conflict review (34, 49), with OD-22 in 48. The gaps are in how those
tasks connect: the replan gate cannot pass, no project can become a team project, two sibling
deployments nobody owns (the gateway, and Sentinel) sit under the gates, and one gate row
(InfoSec) was superseded by OD-23 but is still there.

Section verdict: Needs fixes

# Plan review, section 01: Phase T tasks 01 to 10
Repair checked: repair-gaps.md

Reviewer: adversarial plan reviewer (read-only except this file).
Scope: `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md` .. `task-10.md`
against the skeleton `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md` "## Task scopes".

## Missing parts

### S01-C1 (Critical): task-04.md and task-05.md do not exist
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/ (no task-04.md, no task-05.md); skeleton scopes at docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:455 and :466
**Problem:** The Hub migration for team records (04) and the Hub delegated-caller authentication (05) have scopes in the skeleton but no part file. 05 is the security core of the delegation design (the trusted-deputy boundary). Tasks 06 to 08 depend on both, so their interfaces cannot be checked against a real source; every table, column and auth check they reference is unanchored.
**How to fix:** Write task-04.md (complete migration SQL, down/up tests, the mutant each test kills) and task-05.md (complete auth source for delegated callers, the deny-by-default tests) before any of 06 to 08 is executed.

## Task 01 — ADR-H1 amendment and the team-records contract

Citations checked against `.claude/worktrees/hub` on 2026-09-25: ADR-H1 is 776 lines and the contract 2,298, as stated; `ADR-H1:252-253`, `:369-372`, `:434-435`, `:698-700` hold the text Step 1 expects; `hubapi/keys_test.go:530` (`TestHubCodesAreContractSection42s`) and `:737` (`TestPrefixesAndReasonsAreTheContracts`), `hubapi/envelope.go:264-276` (`AuditEnvelope` with `TeamID`), `ids.Wide` (`internal/ids/ids.go:61`) and `hub-contract.md:1766` (the audit `kind` CHECK) are all real. The field list has 69 lines, matching the expected `fields: 69`.

### S01-I1 (Important): contract §6.3 (the `entra.Validator`) and §6.4 (`devissuer`) are not amended, so Fleet's token fails before §5.1 step 3a runs
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:45-50 (the contract's list of changed sections); hub worktree `docs/superpowers/contracts/2026-09-23-hub-contract.md:437-440`, `:492-493`
**Problem:** The contract's own validator spec has `AllowedClients` ("the desktop's client id") checked at `azp` with `ErrClient`, and `scp` must hold `RequiredScope`: "A token with `roles` but no `scp` (app-only) fails here" (`ErrScope`). Task 01 changes the ADR rows and inserts step 3a into §5.1, but leaves §6.3's `Config`, `Claims` and check table, and §6.4's `devissuer` grant list, untouched. Task 05 builds from the contract. It will find a validator that refuses every Fleet token at `scp`, with no spec for where the app-only branch goes (a second `AllowedAppClients`, a `RequiredAppRole`, `Claims.IDType`) or for the `client_credentials` grant's form fields and token claims.
**How to fix:** Add §6.3 and §6.4 to the files list and write their text in full: the `Config` fields for the app-only path (for example `AppClients []string`, `AppRole string`), `Claims.IDType`, the new check rows in order (`idtyp=app` → `azp ∈ AppClients` and `roles ∋ Hub.Fleet`, else `ErrClient`/`ErrScope`), and `devissuer`'s `grant_type=client_credentials` request and minted claims (`idtyp`, `roles`, no `scp`, `oid` of the app). Add them to the `t01-check.sh` greps.

### S01-I2 (Important): the `admin-released` exception lets Fleet write another owner's lease under any admin's delegation
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:374-375; the matrix claim at :584-586
**Problem:** "an `admin-released` event is written under the delegation of an A of the project". It breaks the global rule "Nobody writes another person's task, lease … whatever their role" (skeleton :136-138) and the matrix's own sentence "`Fx` is refused on every owner-bound write". It also widens R12. A compromised Fleet holding **any** admin's live delegation, created for that admin's own agents, can end every teammate's lease. Nothing ties the event to an admin's act: the admin's desktop never called, and `leases.events` is in both class sets, so a worker-class delegation of an admin qualifies. It also needs the admin to have a live delegation at all, which an admin with no agents running does not.
**How to fix:** Make admin release a desk act. Either add a desk-only route (`POST …/leases/{leaseId}/admin-release`, A of the project, own token, audited with `by`) that Fleet observes on its feed and then follows with its `released` state event, or give it its own route name `leases.admin_release` that is in no class set and cannot be delegated. Add the matrix row, and delete the exception sentence.

### S01-I3 (Important): the Hub's "second layer" of the taint rule is bypassed by setting `origin` to `human`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:438-446
**Problem:** The Hub re-checks only "when `origin` is `fleet` and `kind` is `free`". `origin` is a plain body field that Fleet supplies. A Fleet bug that mis-stamps a turn's origin (the case this layer exists for), or a forged write within R12, sends `origin: "human"`, and the Hub accepts a free-text directive with no human act behind it. As written, the layer checks Fleet's claim against Fleet's claim.
**How to fix:** Bind `origin: "human"` to evidence the Hub holds. The directive names `humanMessageId`: an `hm-…` of the same owner, with `toAgentId` equal to the directive's `fromAgentId`, created within the turn window (for example 60 minutes), and not already cited by an earlier turn. Otherwise 403 `forbidden` with `details.required = "human_message"`. Add `humanMessageId` to the §7.20 body, the field check and the §8 `data`.

### S01-I4 (Important): the Orchestrator's kept prose reply has no record
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:448-451 (§7.21, "one audit event and no table row"); `05-design.md:431-434`
**Problem:** The design says the Orchestrator's reply to its human is shown "**and kept**: the runner's `stream-json` result for a human-origin turn is stored with the turn's `ot-…` record as `text·A`, readable by its owner and an org auditor". §3.3's row (`05-design.md:310`) says "audit event only (metadata)". Task 01 follows §3.3 without naming the contradiction. What the Orchestrator told its human, the accountability fact the panel shows, therefore has nowhere to live: §8 `data` may not hold text.
**How to fix:** Decide it here, where decisions (a) to (d) are made. Either add a `turn_reply` table and a §7.21 `reply` field (`text·A`, ≤ a stated bound, owner and auditor only, tombstoned on erasure, human-origin turns only), or record that the design's `:434` is dropped and why, and raise it as a skeleton gap for the owner.

### S01-M1 (Minor): state-refusal codes are stated twice and contradict each other
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:313-316 against :329-330, :345, :363, :373, :428
**Problem:** Each section's prose says 409 `conflict` for a state refusal, and a convention paragraph says to read each of these as `team_state`. The part calls the text "normative and copied verbatim" (:712), so the contract ships contradicting itself, and tasks 03, 06 and 07 must know which reading wins.
**How to fix:** Write 409 `team_state` in place in §7.13 to §7.19 and drop the override sentence.

### S01-M2 (Minor): the check script needs `### 7.NN` headings that no step gives
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:318, :333, :696
**Problem:** Steps 6 to 8 label each section in bold (`**§7.13 Delegations.**`). The check counts `^### 7\.(1[3-9]|2[0-6]) ` and expects 14. §7.18 covers threads and messages together, so the fourteen headings the check expects are not listed anywhere. Step 5's heading says "rows 46 to 78", but the table runs to 79.
**How to fix:** List the 14 exact heading lines (`### 7.13 Delegations` … `### 7.26 Fleet's feed`) and fix the Step 5 heading to 46 to 79.

### S01-M3 (Minor): the citation for strict decoding points at the `sessionId` definition
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:311
**Problem:** `hub-contract.md:941-942` defines `sessionId` as the control plane's local id. It is not the strict-decoding rule (§1, `httpx.DecodeJSON`) that the sentence cites it for.
**How to fix:** Cite §1's strict-decoding line and keep `:941-942` only for "no session id".

### S01-M4 (Minor): Fleet serves exactly one org, the one enrolled for its home `tid`, and the plan does not say so
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:141-144, :501-502; `ADR-H1:376-379` (an org *is* an Entra tenant)
**Problem:** Rule 6 and §7.26 tie every delegated request to "the token's `tid`'s enrolled org". Fleet's workload identity lives in the SaaS tenant, so its `tid` is that tenant's. Every other org, including the BYO-customer-Entra tier the ADR already provides for, is unreachable unless Fleet obtains a token per customer tenant. That needs a multi-tenant registration with admin consent and a per-tenant token cache. The part does not say this.
**How to fix:** State the assumption in §5.10: one enrolled org per Fleet token `tid` today. Name the later path in R12 or a new risk (a per-tenant token from a multi-tenant Fleet registration, cached per `tid`), and mark it for task 11's token source.

## Task 02 — `teamtext`

Citations checked: `hubapi.FormatTime` is at `hub/hubapi/envelope.go:392-394`; `types.go:6-11` holds the never-omit rule; `control-plane/go.mod` is `go 1.23`; `ids.Wide` yields 16 hex, which matches `agentIDRe` and `userIDRe`. The tests are red first (a build failure), each case names its mutant, three mutants are killed by hand, and `-timeout 60s` turns the exponential-`**` mutant into a failure. Sizing (2.5 h) is fair.

### S01-I5 (Important): variation selectors and zero-width format characters carry the same invisible payload the tag block does
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-02.md:793-796 (the refused ranges), :798-800 (the name rule uses `unicode.IsSpace`), :1057-1060 (the disclaimer)
**Problem:** The package exists so that "a message would [not] carry invisible instructions past every human who reads it" (:9-11). `clean` refuses only U+202A–202E, U+2066–2069 and U+E0000–E007F. Variation selectors (U+FE00–FE0F, and U+E0100–E01EF, 256 of them, enough to encode one byte per character) pass. So do zero-width characters (U+200B–U+200D, U+2060–U+2064, U+FEFF) and bidi marks (U+200E, U+200F, U+061C). That is the same attack encoded one plane over. The disclaimer names the zero-width ones but not the variation selectors. It is also a name-spoofing hole: U+200B is not `White_Space`, so `CleanName` accepts `Al​ice` beside `Alice`.
**How to fix:** In both `Clean` and `CleanName`, refuse every `unicode.Cf` rune (this covers the bidi marks, the zero-width characters, the BOM, U+2060–U+206F and the tag block), plus U+FE00–FE0F and U+E0100–E01EF, under a `RuleInvisible` (or reuse `RuleTagBlock`). Add one test row per range with its neighbours, as the existing rows do. G3 names fewer ranges than this, so raise the widening to the owner as a decision in the return, and do not ship it as a silent disclaimer. If emoji ZWJ sequences must survive in human bodies, allow U+200D only between two `Extended_Pictographic` runes, and test that.

### S01-M5 (Minor): the test sources carry literal bidi and invisible characters
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-02.md:494-497, :511-512, :521 (a byte dump shows raw U+202A, U+202E, U+2066, U+2069, U+2029, U+202F, U+2065, U+206A, U+00A0 and U+3000 inside Go string literals)
**Problem:** A reviewer cannot see what these cases test: `"a b"` in the `CleanName` list is a no-break space, and `"x y"` appears twice as two different characters. Copying from the plan can silently normalise them. ADO and GitHub flag a raw U+202E in a diff as Trojan Source. A package whose job is refusing these characters should not commit them raw.
**How to fix:** Write every such case as an escape (`"x‪y"`, `"a b"`, `"x y"`, …), and add a one-line test that scans the package's own `_test.go` files for runes the package refuses.

## Task 03 — Hub wire types, prefixes, codes, audit v2

Citations checked in the hub worktree: `keys_test.go:495-498` and `:562` (the two `33`s), `:757` (`!= 10`), `:683` (`constValues`), `:719` (`prefixEntity`), `:364` (`contractSection`); `types_test.go:205-213` (`jsonField{name, omitempty, typ}`), `:361` (`decodeInto`), `:367` (`problemOf`); `lists.go:19` (`EnsureLists`); `envelope.go:51` (`kindAudit`), `:234` (`readEnvelope`), `:325`, `:331` (`auditKeys`). All are real. The part catches both `33` asserts, which decision (c) alone would have missed. The tests are red first (a build failure), with named mutants and four killed by hand. It raises the desktop `HubProblemCode` gap itself (:1549-1551).

### S01-I6 (Important): text Fleet writes itself cannot be labelled, so a Fleet-raised escalation cannot be built
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-03.md:747-783 (`CreateEscalationRequest.FromAgentID` "absent when Fleet raises it"; `Escalation.Question`, `EscalationOption.Label` and `.Consequence` are `teamtext.Text`); task-02.md:317-323 (`New` refuses `AuthorAgent` without an `ag-…`); task-01.md:31-35 (decision (c): Fleet's own records carry `author = fleet`)
**Problem:** Three parts disagree on who authored Fleet's own text. Task 01 says `author = fleet`. `teamtext.Author` has only `agent` and `human` (task 02). `New(…, AuthorAgent, "", …)` returns `RuleLabel`. A Fleet-for-itself escalation (the deadline-exhaustion and conflict rows of §6.5) has no `fromAgentId`, so its question and options cannot be built as a `Text`. Task 07 will either fail every such create or label Fleet's template text as a human's.
**How to fix:** Add `AuthorFleet Author = "fleet"` to task 02 (`authorAgentId` null, owner = the delegation's owner). Allow it in `New` and `Valid`, add it to the `text·A`/`text·H` convention in task 01 §7.13 ("a Fleet-authored text is `fleet`"), and add a golden for a Fleet escalation here. Otherwise, state in task 01 that Fleet-authored text is refused as free text and must be a fixed template id rendered by the client, and drop `Text` from those fields.

### S01-M6 (Minor): three empty values are `""` in the contract but `null` or absent in Go
**Where:** task-03.md:662-665 (`ThreadSubject.ID *string`, null for free) against task-01.md:394 (`"id": … | ""`); task-03.md:749 (`FromAgentID` omitempty, "absent") against task-01.md:419 (`"" when Fleet raises it`); task-03.md:935 (`CounterpartUserID *string`, no `omitempty` in a request) against task-01.md:491 (`(or "")`)
**Problem:** Decision (a) fixes the empty value as `null` for responses, but task 01's contract text, written before it, still says `""`. The Hub handler, the fake (task 08) and Fleet's outbox (task 11) will each choose a spelling. With strict decoding and pointer fields, `""` and `null` decode differently (`&""` against `nil`).
**How to fix:** Make task 01's contract text say `null` (or absent, for requests) in those three places. Add a request-side test here: `""` in each is refused with 400, or is normalised to nil, whichever the contract then says.

## Task 06 — Hub routes, part 1

Covers the scope (routes 46 to 59, including the reads the scope forgot), puts the owner check in the transaction, redacts task titles for anyone but Fleet·O, and gives one test per rule with a named mutant. The guard source is complete. It carries the `admin-released` exception of S01-I2 into code and a test (`TestAdminReleaseNeedsAnAdmin`, :503); fixing S01-I2 removes both.

### S01-I7 (Important): task 06 was written before tasks 01 and 03 settled, and now contradicts both
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-06.md:100, :114, :147, :163, :168, :206, :215 (409 `conflict` and `""` empties); :357 (`req.TitleSource != ""`); :392-394 (`DelegationView.Class hubapi.AgentRole`, `routes ...string`); :486, :492, :499, :506 (tests assert 409 `conflict`); :57-59, :618-621, :696-698 (the "Gaps")
**Problem:**
- **State refusals.** Task 01 now fixes every state refusal as 409 `team_state` with `from`/`to` (task-01.md:313-316), and task 03 defines `CodeTeamState`. Task 06's routes and four of its tests assert 409 `conflict`, so a correct implementation fails them.
- **Empty values.** Task 03 fixes `revokedReason`, `exitClass`, `resolution` and `threadId` as `null` and pins them in goldens. Task 06's wire says `""`.
- **Types.** `TitleSource` is a `*string` in task 03, and a delegation's class is `DelegationClass` with `[]RouteName`, so the part's code and test helpers do not compile against task 03.
- **Citations.** Every `task-01.md:NNN` cite is stale. For example, `:543-545` for "required: fleet" is now §8 prose (the rule is at :582-584); `:465-466` for the project `mode` is now §7.23 notes (it is at :499-500); `:193-205` for the codes is now §3.6 and §4.2's intro (the table is at :204-220).
- **Gaps.** Gap 2 is false: `CodeConflict` exists (`hub/hubapi/codes.go:11`, shared from `httpx`), and task 01 now names the three 404s.
**How to fix:** Re-sync task 06 to tasks 01 and 03 in one pass:
- `team_state` everywhere a state refuses, in the text and in the four tests;
- `null` empties;
- task 03's type names, and `*string` for `TitleSource`;
- section anchors (`task 01 §7.14`) instead of line numbers;
- delete the gaps that are closed, and correct gap 2.

### S01-I8 (Important): nothing requires the project to be a team project
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-06.md:88-90 (route 46), :276-286 (`RequireFleet`), :174-176 (only route 33 reads `mode`)
**Problem:** Route 46 creates a delegation on any project where the caller is `member` or `admin`, and no team write checks `mode = team`. A delegation on a solo project lets Fleet write tasks, agents, leases and conflicts into it. That breaks ADR §5.10's "Nothing about a solo project changes" (task-01.md:110). It also writes into a project that Fleet's own feed (§7.26, team projects only) will never show it, so Fleet's rebuild after a restart and its revocation view both miss those records.
**How to fix:** Route 46 refuses a project whose `mode` is not `team` (409 `team_state`, `details.from = "solo"`). Add a `RequireTeamProject(ctx, tx, projectID)` guard, called by every Fleet route after `RequireFleet`, and a test for each: a delegation on a solo project gets 409; a Fleet write into a project that was flipped back to solo gets 409.

### S01-M7 (Minor): `AgentOwnedBy` does not take the project, the role or liveness
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-06.md:65, :291-301, :126 ("the owner's Orchestrator, not ended")
**Problem:** `store.AgentOwnedBy(ctx, tx, agentID, userID)` answers only "is it theirs". So a task in project P can name the owner's Orchestrator from project Q, or a worker, or an ended agent, and a worker's `parentAgentId` can be a worker. The prose states these rules, but no guard or test pins them.
**How to fix:** Give the store call the project and return role and `endedAt`, for example `AgentOf(ctx, tx, projectID, agentID) (owner, role string, ended bool, found bool, err error)`. Add three test rows: an agent from another project (404 `agent_not_found`), a worker as `orchestratorAgentId` (400), an ended Orchestrator (409 `team_state`).

### S01-M8 (Minor): the part's shell commands are not wrapped
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-06.md:561, :569, :626-627, :641-643, :655-658
**Problem:** Steps 3, 5, 6 and 7 run bare `cd control-plane && go test …`, `gofmt` and `git add`/`git commit`. The plan's rule 1 and the user's `CLAUDE.md` require `~/.claude/tools/claudezsh/claudezsh '<command>'` for every shell command, and tasks 01 to 03 follow it.
**How to fix:** Wrap each command as tasks 02 and 03 do. Task 07 has the same defect at task-07.md:381, :433, :445-447, :460-463; fix it there too.

## Task 07 — Hub routes, part 2

Coverage is good: routes 60 to 71, private reads filtered in SQL through one `privateOwner` helper with no "empty owner means all" call, append-only records with no Go update path, the three tests the scope names, and an eleven-row taint table with named mutants. The part states the "answered, not *yes*" limit and the "origin is Fleet's claim" limit honestly. It is stale against task 03's names (`hubapi.TurnOriginHuman` at :215 is task 03's `OriginHuman`; `req.ApprovedBy == ""` at :218 is a `*string` in task 03). It also hits S01-I6 directly: route 64's options are "all `text·A`" (:140) with `fromAgentId: ""` (:125-128).

### S01-I9 (Important): the Hub refuses `resume` and `rebase` in a Fleet-started turn, but tasks 01 and 03 and the design allow them
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-07.md:22-24, :202-209 (`fleetTurnVerbs` = pause, narrow, wait-for), :292-293 (tests expect 403 for bare `resume` and `rebase`); against task-01.md:444-445 ("The typed five … pass"), task-03.md:341-343 (`DirectiveKind.Typed()` is true for all five) and `05-design.md:807-809` (§7.2 item 2 lists `resume` and `rebase` among the typed directives)
**Problem:** The two layers disagree. Fleet's verb handler (task 31) follows §7.2 and lets an Orchestrator `resume` a worker after a yield, or `rebase` it onto upstream, in a Fleet-started turn. The Hub then refuses that directive with 403 `required: "decide"`. The outbox write fails for good, the directive Fleet delivered has no Hub record, and the rebuild after a restart loses it. The protocol's commonest resolution (yield, then resume) breaks. If the stricter rule is what the owner wants, then §7.2, task 01 and `Typed()` are all wrong instead.
**How to fix:** Choose one rule and put it in one place. The recommended fix: `TaintAllows` uses `req.Kind.Typed()` from task 03, so the two copies cannot drift, and the table rows for `resume` and `rebase` expect 201. If `resume` is judged unsafe after all, change §7.2 through the owner, then task 01 §7.20 and `Typed()` to match, and add the same row to task 31.

### S01-I10 (Important): one answered decide approves any number of free directives, in any project, for ever
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-07.md:232-237 (`store.AnsweredDecide`: org, kind, owner, `answered_at IS NOT NULL`)
**Problem:** The query does not bind the approval to anything the directive names: not the project in the path, not the directive's `toAgentId` or its Orchestrator, not a time window, not single use. A decide Linus answered last week in project Q approves a free directive today in project P, and so do the next thousand. That is the Fleet mis-stamp or forged-write case this layer exists for (task 01 R12), and it goes wider than the "answered, not *yes*" limit the part admits. §7.2 item 2 describes the decide as written by `panacea direct` for *that* directive.
**How to fix:** Make `AnsweredDecide(ctx, tx, esID, ownerUserID, projectID, fromAgentID)` require:
- the same project;
- `from_agent_id` equal to the directive's `fromAgentId`;
- `answered_at` within a stated window (for example 60 minutes);
- no earlier directive citing the same `es-…` (a partial unique index on `directives(approved_by)`, task 04).

Add test rows for another project's decide, a reused decide and a stale decide.

### S01-I11 (Important): Fleet cannot list open threads, so it cannot rebuild after a restart
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-07.md:502-506 (Gap 2, raised and not fixed); skeleton global constraint 2 (docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:133-135)
**Problem:** The skeleton requires that Fleet "rebuilds the rest from the Hub" after a restart, and task 11's `rebuild.go` relies on it. Route 63 lists one thread's messages, but no route lists threads. Fleet's only other source is the feed from its cursor. A lost or expired cursor, or a feed with a retention window, leaves open negotiations, their deadlines and round counts gone. The part defers it because task 01 does not list the route. That is a planning dependency that can be fixed now, not a reason to ship the hole.
**How to fix:** Add `GET /v1/projects/{p}/threads?state=open` to task 01 (§7.0 row, route name `threads.read`, in the orchestrator class set; served to Fleet·O for a participant's owner and to the org auditor). Build it here with its privacy test (a non-party gets `∅`) and name it in task 08's `hubclient.Team`.

## Task 08 — feed kinds, Fleet's per-org feed, `hubclient`

Citations checked: `hubclient/hubclient.go:96-109` (`Feed`, `Hub`), `fake.go:31` (`Fake`), `:37` (`FakeUser`), `:56` (`fakeWorld`), `:91` (`As`) are all real. `conformance/` does not exist on `feat/hub` yet, and the part marks it UNVERIFIED. The part is thorough: 29 conformance rows, each naming the **fake's** mutant (the right target), a real-Hub twin for each, and an honest list of what the fake cannot keep. Its `team-feed/team-projects-only` row confirms S01-I8: writes into `SoloProjectID` are accepted and only hidden from the feed.

### S01-I12 (Important): tasks 06, 07 and 08 are each several days of work, not "about 3 hours"
**Where:** task-06.md:3 (14 routes, 5 store files, 5 handler files, guards, 31 integration tests); task-07.md:3 (12 routes, 5 store files, the taint rule, 23 tests); task-08.md:3 (route 78, 26 interface methods, 26 HTTP methods, a fake that re-implements every observable rule of 26 routes, 29 conformance rows run twice, 7 server tests)
**Problem:** The house rule is 1 to 3 focused hours per task. Each of these is closer to 1.5 to 3 days. A single-context implementer against a 180-second watchdog and spend stops will stop partway through and leave half a route set with its tests red. The fake in task 08 is the worst: it is a second implementation of tasks 06 and 07, and every Fleet test trusts it.
**How to fix:**
- **Task 06:** split into 06a (delegations, routes 46 to 49), 06b (tasks and agents, 50 to 55), 06c (leases, conflicts and route 33, 56 to 59).
- **Task 07:** split into 07a (threads and messages, 60 to 63), 07b (escalations, directives and the taint rule, 64 to 68), 07c (turns and human messages, 69 to 71).
- **Task 08:** split into 08a (route 78 and the store), 08b (interfaces, context and HTTP client), 08c (the fake's records and the `team/` rows for 46 to 59), 08d (the fake's protocol half, the rest of the `team/` rows, and `team-feed/`).

Each keeps its own mutant table.

### S01-M9 (Minor): task 08's code does not compile against task 03, and its transition row asserts the old code
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-08.md:355-356 (`Class: hubapi.AgentRoleOrchestrator, Routes: []string{…}`); :325 (`done → active` → 409 `conflict`)
**Problem:** Task 03 types `CreateDelegationRequest.Class` as `DelegationClass` and `Routes` as `[]RouteName`, so the one row written in full does not compile. `team/task-transitions` expects `conflict`, but task 01 fixes `team_state`. This is the same drift as S01-I7.
**How to fix:** Use `hubapi.DelegationClassOrchestrator` and `[]hubapi.RouteName{hubapi.RouteTasksCreate}`, and assert `team_state` with `from`/`to`.

## Task 09 — Fleet model and the `panacea` control wire v1

Strong on drift control:
- one fixture directory read by both modules, found by glob, so later fixtures are tested for free;
- strict decoding (G2), with `bad-*` negatives;
- a cross-module code-set test that reads source, not imports;
- `Fail` hides Go error strings;
- `Unavailable` is the only constructor that sets `since`.

The reconciliation table with the parts already written is useful. The paths resolve: `../../../../testdata` from `control-plane/internal/fleet/wire`, `../../../testdata` from `sandboxd/internal/control`.

### S01-I13 (Important): the scope asks for every verb's `args` and `result` shape to be written out, and the part defers both
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-09.md:589-612; skeleton scope at docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:512-513 ("Every verb's `args` and `result` shape is written out")
**Problem:** ST.1 is defined here, but the table gives `args` as sketches: `task` is `{"op":"create"|"update"|"abandon", …}` and `direct` is `{"to","kind", "globs"? | "lease"? | "text"?}`. Every `result` is "the owning task's fixture; the shape there is authoritative". Eight parts that were written first each say "task 09 wins", and task 09 now says "the later task wins". So no document holds the seam, and task 18 (the CLI) and task 19 (dispatch) are written against a shape that exists nowhere. The house style requires complete fenced source for every wire shape.
**How to fix:** Write each verb's `args` and `result` as a Go struct with JSON tags in one file here (`control-plane/internal/fleet/wire/verbs.go`, types only, strict-decodable), with one `request-<verb>.json` and one `result-<verb>.json` fixture per verb. Later tasks may add fields only by amending this file. If that is too much for one task, split it as 09a (envelopes) and 09b (the verb shapes), and point the skeleton's scope at 09b.

### S01-M10 (Minor): `Status` claims to extend phase 2's statuses but renames two of them
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-09.md:93-103; `control-plane/internal/registry/session.go:14-31` (phase 2: `starting`, `running`, `idle`, `exited`)
**Problem:** The comment says it "extends phase 2's four session statuses … with the two a team agent adds". The constants are instead `starting`, `working`, `waiting`, `idle`, `ended`: `running` became `working` and `exited` became `ended`. A desktop that shows solo sessions and team agents side by side (tasks 52 to 54) must map the two vocabularies, and nothing says where.
**How to fix:** Keep phase 2's spellings and add `waiting` (`running` can carry `WaitingOn`), or state the mapping in the comment, give it one function (`model.FromSessionStatus`), and name the task that uses it.

### S01-M11 (Minor): part of the scope's pushed-event set is moved out without the skeleton being changed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-09.md:395-400, :845, :868-869; skeleton :510
**Problem:** The scope lists pushed `turn.begin` and `turn.end`. The part makes `turn.end` a verb (reasonably) and leaves `turn.begin` and its fixture to task 26. The skeleton scope still says 09, so a reviewer of task 26 will not look for `turn.begin` there. The shell commands in Steps 2 to 6 (:776-829) are also unwrapped (see S01-M8).
**How to fix:** Amend the skeleton's 09 scope, and add `turn.begin` to task 26's scope line. Wrap the commands.

## Task 10 — Fleet's Postgres schema and `Store`

The core of it is right:
- every two-replica race is decided in SQL (the `turn_claims` primary key, `ON CONFLICT DO NOTHING` plus the batch row count, `seq = l.seq + 1` under a terminal-and-holder `WHERE`, and `nextval` for outbox order);
- `bytea` payloads keep idempotent bodies byte-exact;
- one suite runs on memory and on Postgres, with a 20-goroutine claim race run 50 times.

The `registrytest` shape it cites exists (`control-plane/internal/registry/registrytest/`).

### S01-I14 (Important): `REVOKE … FROM PUBLIC` does not make policy versions append-only
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-10.md:366-367, :376-378
**Problem:** `REVOKE UPDATE, DELETE ON fleet.policy_versions FROM PUBLIC` removes privileges PUBLIC never had. The table's owner, the role that ran the migration, keeps them all. Nothing in TM1-α creates a separate runtime role (the part marks that UNVERIFIED, for task 32). So Fleet can rewrite an OD-23 policy version, body and digest together, under the same version number, and the Hub's `agent.started` record of "policy version 3" then points at different bytes. This is the policy mechanism the brief calls out. The Hub's own rule for append-only records is grants **and** triggers (G4; skeleton task 04 scope).
**How to fix:** Add `CREATE FUNCTION fleet.refuse_rewrite() … RAISE EXCEPTION` and a `BEFORE UPDATE OR DELETE` trigger on `fleet.policy_versions`. Add a suite row, `PolicyVersionRewriteRefused`, that runs a raw `UPDATE` on Postgres and expects an error (memstore has no update method, which the suite also asserts). Keep the `REVOKE`, but write it against the named runtime role once task 32 creates it.

### S01-I15 (Important): the part accepts task 28's `fleet.messages`, which persists Hub message text in Fleet
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-10.md:17-19, :581, :586-587; task-28.md:651-655 (`fleet.messages … text jsonb`), :659-661
**Problem:** Skeleton global constraint 2 and this task's scope say "No Hub record is stored except as an outbox payload" (`ADR-H1:467-474`: Fleet keeps Hub records in memory only). Task 28's `fleet.messages` holds each message's `kind`, `fields` and `text`. That is the Hub's `ms-…` record, text included, kept while the thread is open and for 24 hours after. Relabelling it "working state" does not change what it is. It also makes an erasure hole: task 46 tombstones the Hub copy, and Fleet's copy survives. Task 10 records it as a reconciliation and moves on.
**How to fix:** Either store in Fleet only what negotiation needs without text (`message_id`, `sender`, `kind`, `counting`, `seq`), load text from route 63 into memory per turn, and drop the `text` column; or raise an ADR-H1 amendment for the owner that permits a bounded, erasable text cache, and name it in task 46's erasure path. Record the choice in both parts.

### S01-M12 (Minor): "every method scopes by org" is not true of the interface
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-10.md:69 (the package comment), :109-110, :144 (`PendingEvents`, `ClaimedTurn`, `DropClaim` take only an agent id), :321-342 (`turn_claims` and `orch_sessions` keyed by `orchestrator` alone)
**Problem:** Wide `ag-…` ids make a cross-org collision negligible, so this is not an isolation break today. But the stated invariant, which is also the reason given for adding no RLS (:577-579), is false, and a later method written "like the others" inherits the gap.
**How to fix:** Add `org` to those three methods and make `(org_id, orchestrator)` the key of `turn_claims` and `orch_sessions`. Otherwise, reword the comment to name the exceptions and say why the id's width covers them.

### S01-M13 (Minor): sent outbox payloads keep message and directive text with no stated retention
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-10.md:106 (`OutboxDeleteSentBefore`), :281-298 (`payload bytea` kept after `sent`)
**Problem:** The outbox payload is the exact Hub request body, including `text·A` for messages, directives and escalations. No part states how long sent rows live, so erasure (task 46) leaves copies in Fleet for an unbounded time. The Step 3 and Step 4 commands (including `docker run`) are also unwrapped (S01-M8).
**How to fix:** State the retention (for example, sent rows deleted after 1 hour by task 12's janitor, dead rows after 7 days). Add the outbox to task 46's erasure list, and a suite row that `OutboxDeleteSentBefore` removes the payload.

## Summary

| Severity | Count | Ids |
|---|---|---|
| Critical | 1 | C1 |
| Important | 15 | I1–I15 |
| Minor | 13 | M1–M13 |

Unverified by this review: every Hub server, store, migration and conformance file (Hub tasks 05, 06, 13, 14, 17, 26, 33, 34), which are planned and not built on `feat/hub`; the phase 8 and phase 6 sources, which tasks 01 to 10 do not cite.

First-pass verdict (superseded by the second pass below): Needs fixes

---

# Second pass, 2026-09-25 (stage 2 re-review)

Parts re-read: task-04.md and task-05.md (new since the first pass, closing S01-C1 if complete),
and task-01, 06, 07 and 08 (revised after 13:41). Parts 02, 03, 09 and 10 have not changed since
the first pass (file times 10:58, 13:18, 11:20, 11:32), so their first-pass findings stand as
written. Gaps 1, 5, 6, 7 and 13 of `.superpowers/sdd/team/repair-gaps.md` fall in this section
and are checked below. Second-pass finding ids continue the first-pass numbering.

Resumed 21:40 after the 21:17 spend stop (the pass above had written only this preamble). File
times at resume: task-01 20:01, task-04 14:23, task-05 20:02, task-06 20:37, task-07 20:15,
task-08 20:43. All six are re-read against those versions.


## Second pass: Task 04 — Hub migration (new; 959 lines, complete)

Task 04 is whole: seven decisions, complete SQL in ten pieces, 17 tests each with a named mutant,
steps, acceptance and non-claims. It closes the task-04 half of S01-C1. Checked against the real
tree: the class route sets in `delegations`' `CHECK` equal task 03's `ClassRoutes` and
`workerRoutes` (`task-03.md:393-411`), 23 and 10 names, verbatim. `orgs.entra_tenant_id` is
`UNIQUE` (Hub `task-07a.md:79`), so `fleet_org` cannot return one of two orgs. Hub 07a's
`TestEveryForeignKeyCarriesTheOrg` exempts keys to `hub.orgs` (`task-07a.md:530`), so the unnamed
`org_id REFERENCES orgs (id)` columns pass it. The 41 named keys count true, the 22 kind rows match
task 01, and the 17-test and 13-table acceptance counts are right.

### S01-I16 (Important): Fleet's two lookups are keyed on an Entra tenant that an OD-30 service token does not carry
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-04.md:68-73, :801-834, :867
**Problem:** `fleet_org(p_tenant uuid)` and `fleet_delegation(p_tenant uuid, p_id)` join
`orgs.entra_tenant_id` to "Fleet's token `tid`". Their own text calls that tenant join "their only
tenancy check" (:832-834). Under OD-30, Fleet is the Keycloak confidential client
`panacea-app-fleet` (`sec-90-identity.md` ID-I1, ID-I2). A realm service-account token has no
customer Entra `tid` to pass. So either task 05 passes a constant, and the only tenancy check
becomes a no-op, or the lookups cannot be called. Section 90 rewrites the validator and the
amendment, but it does not mention these two functions or `TestFleetLookupsStayInTheTenant`.
This is repair gap 4 (one Fleet per tenant) in its OD-30 form.
**How to fix:** Choose the tenancy rule for a realm-client Fleet and write it into decision (g).
One option is a `fleet_clients (client_id text, org_id text)` table the admin enrols, with the
lookups keyed on the token's `azp`. The other is one org per Fleet deployment, stated, with the
org id taken from configuration. Re-key both functions to that rule. Keep the mutant: another
org's delegation must return no row.

### S01-M14 (Minor): the "acts" keys pin the owner but not the project
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-04.md:33-42, :343, :412-415, :537-538, :622-627
**Problem:** Every "acts" key is `(org_id, owner_user_id, agent)`, so a record in project P can
name the same owner's agent, task, human message or escalation from project Q. A delegation is
per project (:292), and project Q may have different members. A Fleet mis-stamp that Go misses
would show Q's agent ids to P's members. It would also let an escalation answered in Q approve a
free directive in P, which is the database half of S01-I10. Decision (c)'s claim that "a Fleet
mis-stamp that passed Go would still fail `23503`" holds for the owner only.
**How to fix:** Make the target key `UNIQUE (org_id, project_id, owner_user_id, id)` on
`agents`, `team_tasks`, `human_messages` and `escalations`. Add `project_id` to each "acts" key
and to `directives_approval_fk`. Add the P-versus-Q rows to `TestOnlyTheOwnersAgentsAct`.

### S01-M15 (Minor): owner-private reads have one layer, and the part does not say so
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-04.md:766-778, :939-959
**Problem:** Escalations, directives and human messages are "readable by their owner and an org
auditor only" (`05-design.md:308-311`). The only database policy is `org_isolation`, so owner
privacy between members is held by Go alone. That is a defensible choice, but it breaks the
part's own two-layer rule (decision d), and neither the non-claims nor task 07 state it. A list
query that forgets `owner_user_id` would show Alice's answers to Bob, and no database test would
catch it.
**How to fix:** Add a non-claim: "Owner-private reads are Go's (task 07), not RLS." Name task
07's test that kills the missing-`WHERE` mutant. Or add a second policy keyed on a
`hub.user_id` setting, with its own cross-owner test.

### S01-I6 (Important): confirmed still open, `teamtext.New` refuses the text task 04 and task 07 now admit
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-02.md:317-323; task-04.md:553-556; task-07.md:129, :343
**Problem:** Task 04 stores a Fleet-raised question as `agent` text with a `NULL` agent. Task 07
answers `fromAgentId: ""` with 201. But task 02's `New` refuses `AuthorAgent` with no `ag-…` id
(`RuleLabel`), and task 02 has not changed since the first pass. The Hub handler cannot build
that `Text`, so the 201 path in task 07's test cannot exist.
**How to fix:** In task 02, admit `AuthorAgent` with `authorAgent == ""` for one named kind only
(Fleet's own question). Add a test for it, and a mutant: admitted for every kind. Cite that rule
from task 04 :553 and task 07 :129.

## Second pass: Task 05 — Hub delegated-caller authentication (new; 879 lines, complete)

Task 05 is whole: nine decisions, complete Go for `entra`, `server`, `store`, `devissuer` and
`cmd/hub`, 14 unit and 14 integration tests with named mutants, steps, acceptance, non-claims and
a gaps list. With task 04, **S01-C1 is closed.** Checked against the built tree
(`.claude/worktrees/env`, `d09b57a`): `verify.go:116-118` is the `azp` check and `:119-122` the
`scp` check. `entra.go:23` is `Config`, `:44` is `Claims`, and `:158-164` is the
`AllowedClients` loop. `entra_test.go:331` is the only `idtyp` line. The integration acceptance
count of 14 matches the table.

Repair gaps in this part, checked:
- **Gap 5** is resolved: `task-01.md:115-117` now reads "a token with no `scp`". The `task-66.md:143`
  half is outside this section.
- **Gap 6** is resolved: `task-07.md:41-42` sets `Principal.Auditor` "always false under a
  delegation", with `TestFleetIsNeverAnAuditor` (`:38`).
- **Gap 7** is real. It is kept as S01-M16.
- **Gap 2** (`task-17.md`) and **gap 3** (a real-tenant app-token check) are outside tasks 01-10.
  Under OD-30, gap 3 becomes a Keycloak service-token check (sec-90 ID-I1).
- **Gap 4** is S01-I16 and S01-M4.

### S01-I17 (Important): task 05 was revised after OD-30 and still builds the Entra app-only path
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-05.md:9-13, :26-31, :76-79, :88-92, :138-212, :522-618, :685-698
**Problem:** The part's file time (20:02) is after section 90's review (18:50). It still adds
`entra.AppCallers`, `Claims.AppOnly`, the "no `scp`, listed `azp`, `Hub.Fleet` role" switch, a
`devissuer` client-credentials grant with a workload-identity assertion, and
`PANACEA_APP_HUB_FLEET_CLIENT_ID` as a GUID. Sec-90 ID-I1 and ID-I2 replace all of these with
`oidcauth` `ServiceClients` and `claims.Service && ClientID == "panacea-app-fleet"`. Tasks 06 to 08
consume `Claims.AppOnly` and `MintAppToken`, so the stale path spreads into every Hub delta
test. The tenant join in `resolveDelegated` (:368) and `FleetOrg` (:495) is S01-I16.
**How to fix:** Apply ID-I1 to this part: decisions (a), (h) and (i), the `entra` and
`devissuer` blocks, the `cmd/hub` variable, and the eight unit tests they carry. Rename
`AppOnly` to `Service` throughout. Replace `MintAppToken` with the realm dev issuer's
service-token mint, which env task 17 step 0 plans. Keep decisions (b) to (g) as they are: they
do not depend on how Fleet's token was issued.

### S01-I18 (Important): tasks 04 and 05 are each sized at 3 hours and are each more than a day
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-04.md:3; task-05.md:3
**Problem:** Task 04 is about 560 lines of SQL across 13 tables, 17 integration tests with a
two-org seed, and edits to four Hub tests. Task 05 changes 14 files across five packages and
carries 28 tests. Neither fits the house bound of 1 to 3 focused hours. S01-I12 found the same
for tasks 06 to 08. An implementer that stops at 3 hours leaves a migration or an auth pipeline
half done, and the watchdog and spend stops make that likely.
**How to fix:** Split task 04 into 04a (pieces 1 to 5 and their tests) and 04b (pieces 6 to 10,
RLS, grants and the lookups). Split task 05 into 05a (`entra` or `oidcauth`, plus the header
rules and `userKey`), 05b (`resolveCaller`, `store`, the audit row and the integration world)
and 05c (the dev issuer grant and `cmd/hub`). Update the skeleton's task list and dependsOn.

### S01-M16 (Minor): a revoked and expired delegation answers `delegation_expired` (repair gap 7)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-05.md:376-383, :731, :877-879
**Problem:** Expiry is checked before revocation. So a member removed at hour 23, whose Fleet
calls at hour 25, is told "expired". Fleet then tries renew, the user sees no revocation, and
the test pins that order as correct. Revocation is terminal and says more, and the part gives
no reason for checking expiry first.
**How to fix:** Check `RevokedAt` first. Flip the test row so revoked and expired answers
`delegation_revoked` with its reason, and kill the swapped-order mutant. Task 44 then need not
work around the code.

### S01-M17 (Minor): Fleet's rate bucket is keyed on the header value before it is validated
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-05.md:231-235, :266-277
**Problem:** `userKey` returns `"fleet/" + c.DelegationID` in `Authenticate` step 4, before
`resolveDelegated` has checked that the id exists. A leaked or compromised Fleet token can send
a new well-formed `dl-…` on every request. That gets a fresh per-user bucket each time, and the
limiter's key map grows without bound. The org bucket still caps the total, but only per `tid`,
and every one of Fleet's orgs shares that `tid`.
**How to fix:** Key the bucket after resolution. Alternatively, bound the limiter's key count
with eviction and state the bound. Add a test row: 1,000 distinct unknown `dl-…` values leave
the key map bounded.

## Second pass: Task 01 (revised 20:01)

`git log` on the part shows that the only change since the first pass is `5b57305` ("app-only
signal is 'no scp', never idtyp"). That change closes repair gap 5 (`task-01.md:19-22`,
`:116-119`). Every other first-pass finding was re-checked against the current text and stands:
- **S01-I1**, open: the files list (:45-53) still names no §6.3 or §6.4. Under OD-30 the fix is
  sec-90 ID-I2's `ServiceClients` text, not an Entra app-only one.
- **S01-I2**, open: the `admin-released` exception is unchanged (:376-377).
- **S01-I3**, open: the taint layer still keys on the body's `origin` (:443-446).
- **S01-I4**, open: §7.21 is still "one audit event and no table row" (:450-453).
- **S01-M1**, open: 409 `conflict` still stands in place at :347, :365, :375 and :430, beside
  the override paragraph (:315-317).
- **S01-M2**, open: step 5's heading still says "rows 46 to 78" (:242), while the files list
  (:51) says 79. The check still greps for 14 `### 7.NN` headings (:698) that no step lists.
- **S01-M3**, open: the strict-decoding sentence still cites `hub-contract.md:941-942` (:313).
- **S01-M4**, open, and now S01-I16 in its OD-30 form: rule 6 (:143-146) still ties tenancy to
  "the token's `tid`'s enrolled org".

### S01-M18 (Minor): the contract's check order and task 05's disagree
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:114, :121-124; task-05.md:45-46, :376-383
**Problem:** §5.10 is headed "The rule, in the order the Hub applies it". Rule 2 lists
`revokedAt` null before `expiresAt` in the future. Task 05's decision (c) and `resolveDelegated`
check expiry first, and its test pins "revoked **and** expired → `delegation_expired`" as
"decision (c)'s order". The contract is normative, so the contract and the code it specifies
disagree on the one case where the order shows.
**How to fix:** Take S01-M16's fix (revocation first) in task 05, which makes task 05 match
rule 2 as written. Or, if expiry-first is wanted, reorder rule 2 and say why.

## Second pass: Task 06 (revised 20:37)

Changes since the first pass: `c17cbe1` (task 03's type names), `775c03a` (routes register
through `Delegable`, :588-619), `49671fb` (touches task 08's query types). Re-checked:
- **Repair gap 1** is resolved. `fleetP` now builds `DelegationView` with
  `hubapi.DelegationClass` and `...hubapi.RouteName` (:397-399).
- **S01-I7** is partly fixed. The types now compile against task 03. Still open:
  - 409 `conflict` for state refusals at :119, :152, :173, :185 and :220, and in four tests
    (:491, :497, :504, :505);
  - `req.TitleSource != ""` (:362) against task 03's `*string`;
  - stale task 01 cites, checked by reading the lines. `:321-322` (title redaction, :157) is now
    §7.13's sample. `:465-466` (mode, :180) is §7.23 notes. `:540` (auditor 404, :628) is §8's
    `data` rule. `:543-545` (required fleet, :282) is §8's feed-flag prose. `:193-205` (codes,
    :58, :635) is §3.6. The table is at `task-01.md:206-222`;
  - gap 2 (:715-717) is still false.
- **S01-I8** is open: there is still no `mode = team` guard on any route (:257-331). This is
  also repair gap 8 and task 06's own gap 3 (:718). Task 04 now has the column and its one-way
  trigger (`task-04.md:62-66`) but names no writer (:958-959), so no task owns turning a project
  into a team project.
- **S01-M7** is open: `AgentOwnedBy(ctx, tx, agentID, userID)` is unchanged (:297), and task 05
  fixes that signature (`task-05.md:510`).
- **S01-M8** is open: `cd control-plane && go test …` is still bare at :567, :575, :644-645 and
  :660-661.

### S01-I19 (Important): a Fleet read is not held to its delegation's project
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-06.md:154-157, :176, :220, :611-613, :624-633; task-05.md:393-402
**Problem:** Only `RequireFleet` compares `p.Delegation.ProjectID` with the path's project
(:281-290), and only the write handlers call it (:343). The three reads (52, 55, 59) are
registered `Delegable` and run under `s.Read`, and the part states no project check for them.
Task 05's `resolveDelegated` checks the route name only. So Linus's orchestrator delegation for
project P reads project Q's tasks, agents and conflicts wherever Linus is a member. Because P's
delegation is class `orchestrator`, it also gets Q's `title` and `reason.why` (the Fleet·O
redaction, :155). Q may be an AU-classified project (OD-19, per project) while P's
Orchestrator runs on the Global route, so Q's text can reach P's session through Fleet's
rebuild. No test row covers it: `TestTaskTitleIsServedToFleetOrchestratorOnly` (:513) and
`RequireFleet`'s unit test (:414-425) test writes only.
**How to fix:** Enforce it once, in task 05. `Delegable(name, h)` records that the route has a
`{p}` segment, and `resolveDelegated` refuses 403 `required: "delegation.project"` when
`r.PathValue("p") != d.ProjectID`. Then no handler, read or write, can forget. Add a task 05
integration row (DLLinus for P reading Q → 403) and a task 06 row for each of routes 52, 55
and 59.

## Second pass: Task 07 (revised 20:15)

Changes since the first pass: `c17cbe1` (type names) and `773a94f` (routes register through
`Delegable`, :396-416; `Principal.Auditor` always false under a delegation, :41-42). A new unit
test pins that: `TestFleetIsNeverAnAuditor` (:447-455). **Repair gap 6 is resolved.** The
private reads go through one `privateOwner` helper (:429-437), which answers the Go half of
S01-M15. Re-checked, still open:
- **S01-I9**, open: `fleetTurnVerbs` is still pause, narrow and wait-for (:206-212), against
  task 01's "typed five … pass".
- **S01-I10**, open: `store.AnsweredDecide(ctx, tx, esID, ownerUserID)` (:225, :236) still binds
  no project, agent, window or single use. Task 04's `directives_approval_fk` has no project and
  no `UNIQUE (approved_by)` either (S01-M14).
- **S01-I11**, open: no thread list route. The only thread routes are :409-411.
- **Stale names**, open: `hubapi.TurnOriginHuman` (:219) and `req.ApprovedBy == ""` (:222) do not
  match task 03's `OriginHuman` and `*string`. 409 `conflict` for state is still at :94, :117,
  :152, :331 and :340.
- **S01-M8**, open for task 07: :385 and :462 are bare `cd control-plane && go test …`.
- **S01-I19 applies here too**: routes 63, 66, 68 and 71 are `Delegable` reads, with no stated
  check that `{p}` is the delegation's project. Task 05's single fix covers them.

## Second pass: Task 08 (revised 20:43)

Changes since the first pass: `c17cbe1`, `0ec470c` (task 03's delegation class and header
constant) and `49671fb` (the three team query types in full). Re-checked:
- **S01-M9** is partly fixed: the row written in full now uses
  `hubapi.DelegationClassOrchestrator` (:426). Still open: `team/task-transitions` expects
  `done → active` → 409 `conflict` (:395), where task 01 fixes `team_state`. Rows :343, :396 and
  :400 use `conflict` correctly (id reuse and seq), and :396's seq gap is a state refusal too.
- **S01-I12** is open: the part is unchanged in size.
- **S01-I16 applies here too**: route 78's `orgId` "must be the org enrolled for Fleet's token
  `tid`" (:267-268).

### S01-M19 (Minor): Fleet's feed carries no demotion, and the fake does not say what one does (repair gap 13)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-08.md:270-273, :344, :350-353; hub contract `2026-09-23-hub-contract.md:682`, `:1140`
**Problem:** Setting a member to `viewer` is the Hub's `team.member_set` event, with
`{"role"}` (contract :682, :1140). Route 78 serves only `fleet_feed` team kinds plus
`team.member_removed`, so Fleet never sees a demotion. The fake models `RemoveMember` only. A
member demoted to viewer (who "launches nothing", §3.4) keeps a live delegation. Their
Orchestrator runs on until its next write is refused under rule 3's matrix, and the gateway
token keeps being used. Repair gap 13 asked which event the fake records. The answer is that the
part records none, which is the gap.
**How to fix:** Route 78 also serves `team.member_set`, and its row says so. Choose one of two
behaviours: either the Hub revokes a member's delegations when they are set to viewer, with a
`revoke_reason` label added to task 04's enum, or Fleet stops that owner's sessions when it sees
`member_set` with role `viewer`. Add `SetMemberRole` to the fake and a `team-feed` row for it.

### S01-M20 (Minor): the feed-flag paragraph still hedges on a task 04 that now exists
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-08.md:281-285
**Problem:** "**UNVERIFIED**: task 04's part is written in parallel; if it leaves the setting
to Go, this task adds the kind table of `task-01.md:478-499` to `store.Append`." Task 04 now sets
both flags in a `BEFORE INSERT` trigger that overrides Go (`task-04.md:53-60`, :254-273), and
task 01's kind table is at `task-01.md:512-533`, not :478-499. An implementer reading this may
also add the table to `store.Append`, which gives two copies of one rule.
**How to fix:** Replace the hedge with a citation of task 04's decision (e) and
`audit_feed_flags`. Delete the `store.Append` fallback.

## Second pass: parts 02, 03, 09 and 10

Their last commits (`6744634` and `94cd332` for task 03, `895c23b` for task 10) all predate the
first pass, and none of the four files has changed since. Their first-pass findings stand as
written: S01-I5, M5 (02); S01-I6, M6 (03); S01-I13, M10, M11 (09); S01-I14, I15, M12, M13 (10).
All four parts end in their gaps sections, so none is cut off.

## Repair gaps in tasks 01 to 10: disposition

| Gap | Where | Disposition |
|---|---|---|
| 1 | task-06 `fleetP` types | resolved (`task-06.md:397-399`) |
| 2 | task-17 `verify_test.go` | outside 01-10 (task 17); not checked here |
| 3 | no real-tenant app-token check | outside 01-10; under OD-30 it becomes a Keycloak service-token check (sec-90 ID-I1) |
| 4 | one Fleet per tenant | kept: S01-I16 (OD-30 form) and S01-M4 |
| 5 | task-01 "idtyp is app" | resolved (`task-01.md:19-22`, `:116-119`) |
| 6 | `Principal.Auditor` under a delegation | resolved (`task-07.md:41-42`, `:447-455`) |
| 7 | expiry before revocation | kept: S01-M16, and S01-M18 (it contradicts task 01's rule 2) |
| 13 | fake's event on set-to-viewer | kept: S01-M19 (the part records none, and route 78 drops `team.member_set`) |
| 8 (41-45 range) | nobody sets `mode = team` | touches task 04 and 06: folded into S01-I8 |

## Second-pass summary

| Severity | Open | Ids |
|---|---|---|
| Critical | 0 | S01-C1 closed: tasks 04 and 05 now exist and are complete |
| Important | 19 | I1–I15 (first pass; I7 only partly fixed), I16–I19 (new) |
| Minor | 20 | M1–M13 (first pass; M9 only partly fixed), M14–M20 (new) |

Security findings to fix first:
1. S01-I19: Fleet reads are not bound to the delegation's project.
2. S01-I10 with S01-M14: one decide approves any free directive.
3. S01-I16: the only tenancy check of the Fleet lookups depends on a claim OD-30 removes.
4. S01-I2: an admin release goes through any admin's delegation.
5. S01-I3: the taint layer trusts the body's `origin`.

Unverified by this pass: every Hub server, store and migration file (Hub tasks 06, 07a, 07b, 11 to
14, 26), which are planned and not built on `feat/hub`. Only the `entra` package was checked
against built code (`feat/env` `d09b57a`).

Section verdict: Needs fixes
# Phase T plan review, section 02: tasks 11 to 20

Reviewer: adversarial plan reviewer (read-only except this file). Parts read in line ranges against the "## Task scopes" section of the skeleton `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`.

## Task 11: Fleet as a delegated Hub caller

Scope coverage is complete (outbox in the same transaction, row-id idempotency key, the 10-minute
pause, delegation cache and header, feed and clock revocation paths, gapless rebuild). Citations
checked and real: `hub-contract.md:50-55`, `:1100`, `:1345`, `:2029-2046`; `hubclient.go:19-20`,
`:96-99`, `:111-135`; `hubapi/types.go:536-551`; `ids.go:61`; `task-01.md:26-36,226-277`. The
defects are in cross-replica behaviour and in what the outbox does with the writes revocation
itself produces.

### S02-C1 (Critical): a delegation adopted on one replica is invisible to the other, so its revocation is never acted on and its writes are dead-lettered
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:906-938 (with task-12.md:846-895)
**Problem:** `Adopt` puts the delegation into the cache and the org into `cursor` of the replica that served the launch only. `Orgs()` (the only list task 12's `readFeed` polls) and `Cache` are per replica, and the other replica learns an org only from `Rebuild`, which runs at its start and when it takes `fleet-feed`. With two replicas (task 32): (a) if the other replica holds `fleet-outbox`, `Sender.Send` there gets `ErrNoDelegation`, which `Classify` makes Permanent, so every Hub write of that launch goes to `dead`; (b) if the other replica holds `fleet-feed`, it never polls that org and has no cache entry to sweep, while the adopting replica is a follower and marks `done` without calling the `Revoker` (`task-11.md:703-710`). A revoked member's sessions keep running until the next leader change, not within 60 s (`05-design.md` §2.4 item 4). Every delegation test uses one `Feed`, so none can see it.
**How to fix:** make the set of tracked orgs and live delegations shared state, not replica memory: either read `DelegationsInUse` (task 10's store, already shared) on every poll tick to add orgs and fetch unknown delegations by id, or have `Adopt` write a Fleet-own `delegation_use` row that both replicas' pollers read. Add a two-`Feed` test on one store and one fake Hub: adopt on A, A is follower, B is leader and holds the outbox; assert B delivers the row and B calls `Revoke` within 60 s of the Hub revoking it.

### S02-I1 (Important): the writes a lapse or revocation produces are dead-lettered, so the Hub never hears the lease withdrawn or the agent ended
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:237-239,488-493
**Problem:** the `Revoker` withdraws leases and ends or pauses sessions (lines 47-51), which tasks 20 and 23 report through the outbox (`leases.events`, `agents.end`). Delivery resolves the owner's delegation at send time; after a revocation `Cache.Drop` removed it, and after a lapse `Get` refuses it, so `Ctx` returns `ErrNoDelegation`, `Classify` makes it Permanent and the row is `dead`. A renewal does not revive dead rows. Result: teammates' Hubs show leases and agents that Fleet has already withdrawn, and "loses no write" (the scope's test) is false for exactly the writes a revocation makes. A lapse is routine (24 h TTL, `task-01.md:236`, a desktop left closed over a weekend).
**How to fix:** (1) treat `ErrNoDelegation` for a lapsed key as a hold, not a death: a per-key `held` state that `Resume` releases, and do not let a held owner's row block the org's FIFO; (2) for revocation, decide in task 01/05 who writes the terminal records: either the Hub ends the owner's agents and withdraws their leases itself when the delegation is revoked, or the Hub accepts `agents.end` and `leases.events` under a just-revoked delegation for a bounded window. Add a test: lapse, the `Revoker` enqueues a lease withdrawal, renew, and the row is delivered.

### S02-I2 (Important): shutdown and token failures dead-letter good rows
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:239-252
**Problem:** `Classify` makes everything that is not `DeadlineExceeded`, `ErrUnavailable`, 408 or 429 Permanent. `context.Canceled` (SIGTERM on every rolling deploy; task 12 cancels the loop when the lock connection drops, `task-12.md:30`) and a workload-identity `TokenSource` failure (`ErrSignedOut` passes through, `hub-contract.md:1345`) are not Hub answers, yet a row in flight at that moment is marked `dead` and never retried. The Hub transport is also still a stub on `feat/hub` (`hubclient/client.go` returns `ErrNotConfigured` from every method), so how it wraps transport errors is **UNVERIFIED**; the plan's own test of the wrap is only the fake.
**How to fix:** on `ctx.Err() != nil`, leave the row untouched and return; classify `ErrSignedOut`/token errors as Transient; add `TestCancelledContextLeavesTheRowPending` and `TestTokenFailureIsTransient`. State that `Classify` depends on Hub task 33's transport wrapping and pin it with one conformance row there.

### S02-I3 (Important): the three PATCH ops cannot be delivered reliably: the key is POST-only and the body carries a revision
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:12-16,123-135
**Problem:** the Hub's idempotency replay applies to "Any `POST`" (`hub-contract.md:50`); routes 51, 58 and 61 are `PATCH` and their bodies carry `revision` (`task-01.md:345,388,397`). (a) A retry after a lost response of a committed PATCH is a stale-revision 409, which `Classify` dead-letters. (b) Two PATCHes of one record enqueued before the first is delivered both carry the revision Fleet had in memory, so the second always 409s and is dead-lettered. The part's claim that the key "makes a retry replay" is false for `tasks.update`, `conflicts.update` and `threads.update`.
**How to fix:** either extend §1's idempotency to `PATCH` in task 01's amendment and task 05/06 (and drop `revision` from Fleet-authored PATCHes, or let the Hub apply them as last-writer by Fleet's outbox order), or resolve the revision at delivery. Add `TestTwoQueuedPatchesOfOneTaskBothLand` and `TestALostResponsePatchReplays`.

### S02-I4 (Important): one bad `delegation.created` event stalls the whole org's feed, including every later revocation
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:650-663,632-639
**Problem:** `apply` reads the new delegation under itself through route 47, `delegations.read`. A delegation's `routes` is any 1 to 25 names from the class set (`task-06.md:88-90`), so a desktop may create one without `delegations.read`; the Hub then answers 403 and `apply` returns the error. `Poll` does not advance the cursor, so the org's feed re-reads the same page forever: no later `delegation.revoked` in that org is ever applied, leaving only the clock path (up to 24 h). The same holds for a 404 (an erased record) and for any `Also` consumer error.
**How to fix:** make `delegations.read` mandatory in every delegation (task 06's 422 rule), and in `apply` treat any non-transient Hub answer as "skip and log" rather than a retry. Add `TestAPoisonEventDoesNotBlockALaterRevocation`.

### S02-I5 (Important): taking the feed lock jumps the cursor to `latest` and drops unapplied events of every other kind
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:824-837 (with task-12.md:854-858)
**Problem:** `Rebuild` sets each org's cursor to a fresh `after=latest` head. Task 12 calls it on every acquisition of `fleet-feed`, on a replica whose cursor is already running. Events between its old cursor and the new head are skipped. Delegations in use are re-read and tasks and open conflicts are re-listed, but every kind tasks 22-31 register with `Also` (line 616) is not, for example an escalation answer or a human message. A leader change silently loses them.
**How to fix:** on acquisition, `Reset` and re-read delegations but keep the existing cursor (`SetCursor` only when the org has none); or have every `Also` consumer declare a rebuild read. Test: an event of an `Also` kind committed during a leader change reaches its consumer.

### S02-M1 (Minor): acceptance text contradicts itself, and a comment names an `AsFleet` path the part forbids
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:1218-1221,218-219
**Problem:** "both `grep`s print nothing" and then "the second lists exactly" two files. The `Sender` comment says "sets the delegation (or AsFleet)", but lines 42-45 say there is no Fleet-in-its-own-name write.
**How to fix:** say that the first grep prints nothing and the second prints exactly the two `store.go` files; delete "(or AsFleet)".

### S02-M2 (Minor): the `WithIdempotencyKey` step points at a header-setting site that does not exist yet
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-11.md:92,1201-1205
**Problem:** "read where the client sets the header today": on `feat/hub`, `hubclient/client.go` is a stub. Its header says "task 33 replaces it", and no method sets any header. The eight-line amendment has nowhere to land until Hub task 33 merges.
**How to fix:** name Hub task 33 as a dependency of this step, or move `WithIdempotencyKey` into task 08's part.

## Task 12: `cmd/fleetd` chassis

Scope coverage is complete: config read once and failing on its name, Entra per request through
phase 8's `internal/entra`, role-to-verb as data, `/healthz` and `/readyz`, and advisory-lock
singletons, with the three scope tests. Citations checked and real: `entra.go:23,44,60,119`
(`feat/env` at `1772f25`); `hubapi/codes.go:20-22,32`; `types.go:20-25,95-99`;
`hubclient.go:23-30`; `hub-contract.md:563-564,2232`; `httpx/problem.go:93`;
`cmd/api/config.go:165,421`.

### S02-C2 (Critical): no task owns the Hub read that tells Fleet who a caller is, so no Fleet route can authorise and `fleetd` does not build
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:47-54,1176-1178,1201-1203
**Problem:** every Fleet route depends on `Membership`, which depends on a proposed `GET /v1/principals?tid=&oid=&projectId=` and `hubclient.Team.Principal`. Neither is in task 01's team table (rows 46-79, `task-01.md:251-284`) nor in task 08; `grep` finds no "principals" in tasks 01, 05 or 08 or in the skeleton. Step 4 tells the implementer to stop. That blocks task 12's own acceptance (`go run ./cmd/fleetd`) and every Fleet route task (14, 19, 20, 39, 52) on the TM1-α critical path. The part raises the gap correctly, but the plan does not close it.
**How to fix:** add the route to task 01's amendment (row 80, Fleet with no delegation, metadata only, `principal_not_found` for an unprovisioned `(tid, oid)`, answer carries the **project's** `orgId`), implement it in task 06 and add `Team.Principal` plus a conformance row in task 08; add 01/06/08 to task 12's `dependsOn`. It must be a second named exemption beside the feed (`task-01.md` (b) says a second one needs a second amendment).

### S02-I6 (Important): org-level grants are not bound to the project's org, so an auditor of one org could read another org's threads
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:332-346,419-432
**Problem:** for a project route, `authorize` checks only `{org}` in the path against `m.OrgID`; project routes have no `{org}`. `Allowed` grants `VerbThreadRead` and `VerbPrivateRead` on `p.Auditor` alone, before any role check. If the proposed principal read returns the caller's own org flags whatever the project (its contract is not written, S02-C2), then an org-A auditor who names an org-B `pj-…` passes. Tenant isolation rests on an unwritten property of an unwritten route.
**How to fix:** have `Member` carry the project's `OrgID` from the Hub. In `authorize`, refuse with 403 when a project route's project org differs from the caller's org, before `Allowed` runs. Add `TestAnAuditorOfAnotherOrgIsRefusedOnAProjectRoute`, and pin the rule in the principal route's contract.

### S02-I7 (Important): a lost node keeps its advisory locks until Postgres notices, stalling outbox delivery and revocation for far longer than 60 s
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:574-654
**Problem:** the watchdog cancels the **old** holder's loop. The lock is released only when the Postgres server ends that session. When a node dies or is partitioned, the server holds the backend until TCP keepalive gives up. The OS default is about two hours unless `tcp_keepalives_idle`/`interval`/`count` are set; the value on Azure Database for PostgreSQL is **UNVERIFIED**. Until then neither replica can take `fleet-outbox` or `fleet-feed`: Hub writes queue and revocations are not acted on. `TestLosingTheConnectionHandsOver` uses `pg_terminate_backend`, which is the one case where the server knows at once, so it cannot catch this.
**How to fix:** set client-side keepalives on the lock connection and require server `tcp_keepalives_*` (and `tcp_user_timeout`) in task 32's Terraform, with a stated bound (for example 30 s). Or replace the session lock with a lease row renewed every 5 s and taken over after 15 s. Put a node-loss case (drop packets rather than terminate the backend) into task 33's pod-kill run, and state the handover bound as a number.

### S02-M3 (Minor): the membership cache has no bound and caches negatives for any path string
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:369-400,419-420
**Problem:** `r.PathValue("p")` is not validated before `Lookup`, and each distinct `(tid, oid, project)` adds a map entry that is never evicted. Any signed-in user of an allowed tenant can grow the map without limit and make one Hub call per invented project id.
**How to fix:** refuse a `{p}` that fails `hubapi.ValidID` with 404 before the lookup; evict entries older than `ttl` on write, or use a size-capped LRU. Add a test for each.

## Task 13: OD-23 mechanism, part 1 (layers)

Scope coverage: `Render` with the skeleton's signature, the layer order, every invariant the
skeleton lists (I1 to I9) plus I10 to I12, a mutant per invariant, determinism, and defaults that
hold only the invariants, so no policy is drafted (OD-23). Citations checked and real:
`owner-decisions.md:151`; phase 6 `task-14.md:9-11,223,395-402` (the fragment is byte-equal to the
part's copy); `phase-6-gateway.md:248`; `05-design.md:241-248,835-838,453-456`. The mechanism
reads well. What it lets a layer put in is too wide.

### S02-C3 (Critical): a project layer can run arbitrary commands in every other member's sandbox, the Orchestrator's included
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-13.md:413-434,455-516
**Problem:** a project layer is written by a **team admin** (task 14, `VerbProjectPolicy`), and `Render` merges any key of its `all` and class sections into the file mounted in **every** member's pods. `check` constrains only permissions, the mode, and a handful of scalars and env keys. `hooks` are allowed to grow (arrays combine), and `allowManagedHooksOnly` makes every hook in this file a managed hook that runs. The same holds for command-bearing settings such as `statusLine.command`, `otelHeadersHelper`, `awsAuthRefresh` and `awsCredentialExport`, and for `env` keys such as `BASH_ENV`, `NODE_OPTIONS`, `HTTPS_PROXY` and `NODE_EXTRA_CA_CERTS`. One admin therefore gets code execution inside Bob's worker and Bob's Orchestrator: Bob's gateway token (`panacea token`), his fetch credential, and his control socket, whose requests `sandboxd` stamps as Bob's session (task 16). They can call `panacea peer`, `direct` or `human` as Bob's Orchestrator, which breaks OD-16 ("no user steers another's agent") and §3.3 rule 1. I8 does not help: it checks `permissions.allow` only. A layer hook that exits 2 on `Bash(panacea …)` also defeats I1 without touching a rule.
**How to fix:** make the project layer an **allowlist** of keys that can only tighten: `permissions.deny`, `permissions.ask` (worker only) and a closed list of non-command scalars, with every other key an `ErrLayer` naming it. Forbid `hooks`, `statusLine`, `*Helper`, `aws*`, `mcpServers`/`enabledMcpjsonServers` and any `env` key outside an allowlist in **both** layers for the Orchestrator class, and in the project layer for every class. Add `TestAProjectLayerCannotAddAHook`, `TestNoLayerMayAddAHookToTheOrchestrator` and `TestAProjectLayerCannotSetACommandHelper`. Record in the part that the org layer (org admin) remains fully trusted, and say so in the audit event.

### S02-I8 (Important): the project layer can loosen any org setting, so the org layer is not a floor
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-13.md:35-42,211-213,682-683
**Problem:** `merge` lets a later scalar replace an earlier one, and the project layer comes after the org layer. An org's `disableBypassPermissionsMode`, a `sandbox.enabled: true`, `cleanupPeriodDays`, `includeCoAuthoredBy` or an org-set `env` value can be replaced by a team admin. `TestTheProjectLayerComesAfterTheOrg` asserts that this works. The skeleton says a later layer "may add rules and **tighten**" (skeleton line 556). OD-23 puts "our policies" into the sandbox; a team admin undoing the org's policy is not in its scope.
**How to fix:** have `Render` refuse, as `*InvariantError{Invariant: "L1"}`, a project layer that replaces a scalar the org layer set with a different value, unless the key is on a named "project may override" list (empty by default). Change `TestTheProjectLayerComesAfterTheOrg` to a key the org did not set, and add `TestAProjectLayerCannotLoosenAnOrgScalar`.

### S02-I9 (Important): I10 probes four commands, so a deny can silently cut every other `panacea` verb
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-13.md:548-575
**Problem:** `probes` holds `team`, `check`, `check --hook` and `token`. A layer deny such as `Bash(panacea peer:*)`, `Bash(panacea human *)`, `Bash(panacea report:*)` or `Bash(panacea lease *)` passes I10. The Orchestrator then cannot negotiate or escalate, and a worker cannot report or lease. OD-16's only cross-user channel is broken by a render that succeeded. The verb set is known (task 18's list).
**How to fix:** derive `probes` from task 09's per-class verb table (one probe per verb, with a representative argument). Refuse any Bash deny or ask rule whose pattern starts with `panacea`. Add one `TestI10` row per verb family.

### S02-I10 (Important): the design's "deny project `ANTHROPIC_*` env overrides" is not built: I12 binds the layers, not the repository's own settings
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-13.md:85,441-452
**Problem:** `05-design.md:246` and `:838` require the managed file to stop a repository's `.claude/settings.json` from setting `ANTHROPIC_*` env. A teammate can commit that file, and the CLI reloads it mid-session (`:835-836`). I12 only stops an org or project **layer** from naming those keys. The rendered file sets none of them, so nothing outranks a repository-level `env.ANTHROPIC_BASE_URL` pointing the session, and its `apiKeyHelper` token, somewhere else. Whether managed `env` can lock a key is **UNVERIFIED** (the part says so), but the plan also has no fallback.
**How to fix:** render `env.ANTHROPIC_BASE_URL` (and the Bedrock equivalents) into the managed file from the route task 20 chose, so the managed value wins key by key. That makes the digest per route, which is acceptable. Add a task 33 check that a repository `settings.json` override is ignored. If the managed file cannot lock `env`, record that egress policy (task 32) is the control and name its test.

### S02-M4 (Minor): the Orchestrator can be denied `Read` without an error
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-13.md:490-499,505-510
**Problem:** I10 looks only at `Bash` rules. A layer deny `Read` or `Read(**)` in the orchestrator section renders cleanly, and the runner's instructions (task 41) assume `Read`.
**How to fix:** extend I8 to refuse a deny rule on `Read` for the Orchestrator class, with a test.

## Task 14: OD-23 mechanism, part 2 (versions, routes, audit)

Scope coverage: append-only versions with author, time, bytes and digest; the four routes with org
admin and team admin roles; `Render` for every class before a version is written; the digest on
agent start; and the three scope tests. The part departs from the skeleton on purpose: the audit
goes out as the admin's own Hub write (route 80) and not through the outbox, because an outbox
write needs a delegation (`task-01.md:31-36`). The reasoning is sound and is recorded as a gap.
Citations: `task-01.md:26-36,284` and `hub-contract.md:50-55` are real. The Hub server names
(`tx.P`, `ProjectRole`) are correctly marked **UNVERIFIED until merge**.

### S02-C4 (Critical): a policy posted before anyone in the org launches is never activated, silently
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-14.md:57-61,484,543-563
**Problem:** activation happens only in `activateOn`, a `Feed.Also` consumer. Task 11's `Feed.Poll` reads only the orgs in its cursor map, and an org gets a cursor only from `Rebuild` (orgs with a delegation **in use**) or from `Adopt` (a launch), and then at `after=latest` (`task-11.md:824-837,906-916`). TM1-α's own flow (task 33: an admin sets the org policy, then people launch) therefore (a) polls no org when route 80 is called, and (b) starts the first launch's cursor **after** the `policy.versioned` event. The layer stays `pending` forever, and `Effective` keeps rendering the defaults. The admin is told nothing (the "what it does not claim" list treats a stuck pending version as the client's fault only). The OD-23 mechanism then fails to put the org's policy into any sandbox, which is TM1-α's gate.
**How to fix:** do not make activation depend on a feed cursor. Either (1) the admin's client calls a Fleet `POST …/policy/{pv}/activate` after route 80 answers 201, and Fleet verifies the event by reading it back from the Hub (a by-id audit read, or route 78 filtered to that subject) before activating; or (2) `Feed.Orgs()` tracks every org with a pending layer, starting from a cursor saved when the pending row was written (a Fleet-own `feed_cursor` row, not `latest`). Add `TestAnOrgPolicyPostedBeforeAnyLaunchActivates` on one `Feed` with no delegations.

### S02-I11 (Important): the task is two tasks: the Hub delta does not fit in its 3 hours
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-14.md:3-4,71-74,788-792
**Problem:** the part modifies four of task 10's store files plus a migration, writes `policy/store.go` and `server/policy.go`, and **also** builds a Hub route on another branch's unbuilt server package: handler, `hubapi` types, feed kind, contract row 80 and §7.27, and a conformance row against the fake and the real Hub. That is 25 tests over two modules and two branches. The house bound is 1 to 3 focused hours, and the part already overran the skeleton's 2.5 h. The skeleton's delta rule also says a Hub delta is its own task, naming the sibling file it extends. Here the file (`hub/server/policyversions.go`) and the dependency (Hub task 14's server) are named only in passing.
**How to fix:** split out "Hub delta: route 80 and `policy.versioned`" as its own task (14b) after 06/08, naming Hub task 14's `server.go` as the file it extends and task 01's §7.0 as the contract row. Make task 14 depend on it. Add row 80 to task 01's amendment rather than recording it only as a gap.

### S02-M5 (Minor): the Hub records whoever calls route 80, not the author Fleet stored
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-14.md:634-667,292-301
**Problem:** route 80 checks that the caller holds the role, but `Activate` does not compare the Hub's `recordedBy` with the pending row's `Author`. A second team admin can activate a first admin's pending layer, and then the Hub's "who changed our policy" names someone who did not write it.
**How to fix:** put `recordedBy` into the event's `data` and have `Activate` refuse, leaving the layer pending, when it differs from `Author`. Or say explicitly that any admin of the scope may activate, and show both names in the GET.

### S02-M6 (Minor): a re-validation refusal at activation is invisible to the admin
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-14.md:298-300,558-560
**Problem:** "the refusal is logged and the admin posts again", but the GET shows only `state: "pending"`, with no reason, so the admin cannot tell a refused layer from one that is merely waiting.
**How to fix:** store the refusal on the row (a Fleet-own `refused_reason`, set once) and show it in the GET as `state: "refused"` with the invariant.

## Task 15: Phase 8 delta, sandbox classes and the policy mount

Scope coverage: `Spec.Class` and `Spec.PolicyFile`; the orchestrator is diskless, has no
affinity, gets 250m / 512 MiB and runs `panacea-orchestrator`; a per-pod `<pod>-policy` Secret is
mounted read-only at `/etc/claude-code/`; `PANACEA_APP_POLICY_SHA256` is set; `policycheck` runs
before the PTY; and the three scope tests are present, the process test included. Citations
checked on `feat/env` at `09ae70a`: `manager.go:31-32,52-55,69-83,189-190,218-223,270,337,350,420`,
`backend.go:60-62,204`, `kata.go:95-96` and `sandboxd/cmd/sandboxd/main.go:56` are all real. The
file names the extended sibling files (the delta rule is met).

### S02-I12 (Important): making `PolicyFile` mandatory breaks phase 8's solo cloud path and its existing tests, and no task passes one
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-15.md:152-154,566-570
**Problem:** `validateClass` refuses an empty `PolicyFile` for every Kata pod. The built `katabe.Create` and phase 8's broker (tasks 17-19) create solo cloud sessions with no policy. So do every existing `manager_test.go` spec and phase 8's integration rows. After this delta, every solo cloud launch fails and step 5's "every package `ok`" cannot pass, because only the refused-create and delete tests are listed as modified. The part admits the gap and suggests `Render(Layers{}, worker)`, but no task supplies it, and `katabe` cannot import `internal/policy` without a new dependency edge that this part does not name.
**How to fix:** in this task, have `katabe` (or the broker, task 17) fill `PolicyFile` from `policy.Render(policy.Layers{}, policy.ClassWorker)` when the caller sets none, and name the edge. Update every existing `sandbox` and `katabe` test spec to carry the fixture bytes, and list `manager_test.go`'s full change. Add `TestASoloKataSessionStartsUnderTheDefaultPolicy`.

### S02-M7 (Minor): tasks 13 and 15 disagree on whether the mount shadows phase 6's drop-in
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-15.md:18-27 (against task-13.md:56-60)
**Problem:** task 13 says "Task 15 must not shadow that drop-in (its mount is the file, not the directory)". Task 15 mounts the whole directory on purpose, shadowing `managed-settings.d/50-panacea-gateway.json`, and `policycheck` refuses any drop-in. Task 15's reasoning is the stronger one (the digest covers everything Claude Code reads), but one of the two parts is wrong as written.
**How to fix:** amend task-13.md:56-60 to say the directory is shadowed and the fragment reaches the pod only through layer 2, and have phase 6 task 14's reviewer know that its drop-in is dead in cloud pods.

### S02-M8 (Minor): a mutable policy Secret can change a running agent's policy with no audit and no re-check
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-15.md:214-219,573-576
**Problem:** the kubelet refreshes a Secret volume when the Secret changes, and Claude Code reloads permissions and hooks mid-session (`05-design.md:835-836`). Anyone with `secrets/update` in the sandbox namespace can therefore swap a live agent's managed settings after `policycheck` has passed. No `pv-…` records the change and the Hub's digest no longer describes the running file.
**How to fix:** create the policy Secret with `Immutable: true` (the kubelet stops watching immutable Secrets), and assert it in `TestEveryClassMountsThePolicyReadOnly`.

## Task 16: Phase 8 delta, `sandboxd` `/control`

Scope coverage: a `/control` route beside `/attach` and `/healthz` with its own bearer; the
`0600` socket at `/run/panacea/control.sock`; re-stamping; `team.json` holding worker-visible
fields only, with a strict decode; `unavailable` with `since`; and both scope tests
(`TestForgedAgentAndClassAreRestamped`, `TestSnapshotWithFreeTextIsNotWritten`). Citations checked:
`sandboxd/internal/server/server.go:28,69,85-86`, `images/sandbox/Dockerfile:19,24` and
`cmd/sandboxd/main.go:44` are real. The stamp is safe across owners because task 19 binds it to the
stream (`task-19.md:386-389`).

### S02-I13 (Important): the part is larger than a task and builds scope that belongs to tasks 20, 26 and 27
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-16.md:137-149,1019-1045,76-96
**Problem:** besides the skeleton's scope (route, socket, stamp, snapshot, `unavailable`), the part builds: the local verb `events.wait`, with an event queue, a 50 s long-poll and a `SO_PEERCRED` pid gate; the `turn.end` pid gate (task 27's); the `token` event and gateway-key file writing (task 20's key delivery, S6.5); and `Pid()` accessors through three built packages. That is 1,134 lines, about 25 tests and nine files, in "3 h". The skeleton lists none of the extra frames (Gap S16-2), so tasks 09, 20 and 27 are planned against a surface that is defined here.
**How to fix:** split it: 16a is the skeleton scope; 16b is `events.wait`, the pid gate and `turn.end` (dependsOn 26, consumed by 27); the `token` event moves to task 20, or becomes 16c with task 20 depending on it. Move `Stamped`, `SnapshotEvent`, `TokenEvent` and `CodeForbidden` into task 09's wire and fixtures.

### S02-M9 (Minor): a stuck stream write holds `wmu` with no deadline, so every caller hangs past the 15 s rule
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-16.md:806-816
**Problem:** `forward` calls `c.Write(ctx, …)` with `context.Background()` while holding `wmu`, and starts the 15 s timer only after the write returns. On a half-open stream the write can block on a full TCP buffer. Every later `forward` then queues on `wmu`, and callers get the socket's 60 s connection deadline (an EOF, not `unavailable`), which breaks decision 5.
**How to fix:** give the write a context bounded by the remaining deadline, start the timer before `wmu.Lock()`, and add `TestAStuckWriteStillAnswersUnavailableInTime` with a conn wrapper whose write blocks.

### S02-M10 (Minor): same-uid process isolation is assumed, not set
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-16.md:56-71,142-146
**Problem:** decision 8 accepts that the agent reads `/proc/<sandboxd>/environ`, which holds the control and attach tokens. Decision 10's nonce secrecy depends on the harness being unable to ptrace or read the runner, but same-uid ptrace is blocked only if the Kata guest kernel's Yama `ptrace_scope` is 1 or higher, which is **UNVERIFIED**. With scope 0, the harness can read the runner's `turn.begin` nonce and rewrite `sandboxd`'s memory.
**How to fix:** have `sandboxd` and the runner call `prctl(PR_SET_DUMPABLE, 0)` at start. That makes their `/proc` entries root-owned and refuses same-uid ptrace, and it closes decision 8's premise as well. Add a Linux test that a child cannot open `/proc/<parent>/environ`, and add the guest's `ptrace_scope` to task 33's checks.

### S02-M11 (Minor): the snapshot's string fields carry agent-chosen text with no bound
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-16.md:226-266
**Problem:** `Files`, `Hunks[].Path`, `Leases[].Globs`, `LastCommitPaths` and `Conflicts[].Paths` are strings that a teammate's agent chooses, since a file name or a lease glob is agent-authored. Rule 1 ("facts, not each other's words") holds only if paths are bounded. The strict decode refuses unknown keys, but not a 4 KiB "path" that holds instructions.
**How to fix:** have `applySnapshot` also refuse any path or glob over 1,024 bytes, or one containing control characters or newlines, and cap each list at a stated count. Point to task 02's `teamtext` as the Fleet-side rule.








## Task 17: Phase 8 delta, the broker's second caller and control relay

Scope coverage: the second caller under a live delegation checked by the broker against the Hub (cached at most 60 s), owner labels kept, the `control` relay on 8080, the ready-time source address, and all four scope tests are present. Citations checked and real: `entra/verify.go:116-121` (azp, then scp), `entra/entra.go:52` (`Roles`), `httpx/problem.go:49-50,93`, `proxy/proxy.go:38`, `sandbox/manager.go:113-141` on `feat/env`; P8 `task-17.md:128-139,187`, `task-18.md:37-57,75-76,118-124`, `task-19.md:80-90`; phase 6 `:71,276,290,306`; `task-01.md:114-126,250-254,291-294,319-322`.

### S02-C5 (Critical): Gap S17-1 has no owning task, so the broker cannot learn whom a delegation is for
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:47-54,394-400
**Problem:** the broker must read route 47 as itself and needs the owner's `tid`/`oid` to form `OwnerLabelValue`. Task 01 still admits only "its owner (desk), or Fleet under it" (`task-01.md:252`), its record still carries only `ownerUserId` (`:321`), and neither task 01, task 06 nor the skeleton mentions S17-1, `ownerTid` or `Hub.DelegationReader` (grep, 2026-09-25). The part says its tests "run on a DelegationSource fake until they do", so the broker's owner derivation, the thing P8-D4 exists for, is unbuildable against the real Hub, and task 33's end-to-end run fails or is tempted to trust Fleet's own statement of the owner.
**How to fix:** make it a task. Amend task 01's route 47 row to add "the broker (app-only, role `Hub.DelegationReader`)" and the caller list of rule 1, add `ownerTid` and `ownerOid` to the delegation record returned to that caller, and build both in task 06 with a test that the broker's token reads any delegation and every other app-only token is refused. Add `06` to this task's dependsOn and list the delta in the skeleton's Hub table.

### S02-I14 (Important): decision 11's restricted principal is not in `principalOf`, so its own test cannot pass
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:94-102,304-316
**Problem:** decision 11 lets a revoked or expired delegation `DELETE` only the sandboxes labelled with it, and names `TestRevokedDelegationMayDeleteOnlyItsOwn`. The complete `principalOf` source answers 401 for every revoked or expired delegation on every route and has no restricted variant, so the named test fails as written and task 20's cleanup on revocation strands pods. The test is also missing from step 1's table, although the acceptance count of 18 includes it. For `ErrNoSuchDelegation` there is no record, hence no owner to scope the delete by.
**How to fix:** add `Restricted bool` to `Principal` and a branch in the source: revoked or expired, method `DELETE`, path `/v1/sandboxes/{id}`, gives a restricted principal with the record's owner, and the handler selects on the owner **and** `panacea-app/delegation = dl-…`. `ErrNoSuchDelegation` stays 401. Add the row to the table with its mutant.

### S02-I15 (Important): `TestDelegationForAliceCannotListBob` does not kill the mutant it names
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:222,356-357
**Problem:** `s-b` is made by Bob's **desktop**, and a desktop sandbox carries no `panacea-app/project` label (decision 6, step 5). A mutant that selects on the project and drops the owner still hides `s-b`, so the test passes on the mutant. That is the one cross-owner leak the delegation model must exclude: two members of `pj-shop` under Fleet.
**How to fix:** create `s-b` through Fleet under `dl-bob` (Bob, `pj-shop`), so it carries the same project label, then assert that Fleet under `dl-alice` neither lists it nor `GET`s it, nor opens `control` or `source` on it.

### S02-I16 (Important): a desktop principal may still create the Orchestrator class and supply its own control token
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:58-64,325-327
**Problem:** decision 5 bounds classes only under `Via = fleet`. Task 15 adds `Class`, `PolicyFile` and `ControlToken` to the spec (`task-15.md:317-320`) and never mentions a desktop caller, so after this task Alice's desktop can `POST /v1/sandboxes` with `Class: orchestrator`, a `PolicyFile` of its choosing and a `ControlToken`. That produces a pod that task 32's Orchestrator egress and every class-keyed rule treat as an Orchestrator, with a managed-settings file the org never approved. This task is where the broker first tells the two callers apart, so it is where the refusal belongs.
**How to fix:** under a desktop principal, refuse any `Class` other than `""` or `worker` and any non-empty `ControlToken` with `403 forbidden`, `details.required = "fleet"`. Also state who may supply `PolicyFile` for a desktop sandbox (cross-reference S02-I12). Add `TestDesktopCannotCreateOrchestrator`, which kills a class check applied only on the Fleet branch.

### S02-I17 (Important): the delegation is project-scoped but the machine it reaches is owner-wide
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:65-69
**Problem:** under a delegation for `pj-shop`, Fleet may ensure and wake the owner's default machine, and `SandboxSpec.MachineMounts` (`task-18.md:37-41,57`, a free `MachinePath`) is accepted from Fleet unchecked. That machine holds the owner's solo and other-team worktrees. A worker that another owner's Orchestrator can steer through `peer` could therefore be given a mount of project B, and the broker, which exists to avoid taking Fleet's word, does not check. Decision 5 also says creating a machine stays the desktop's act, while `POST /v1/machines/default` returns **201** when it creates the disk (`task-19.md:83`).
**How to fix:** under a delegation, refuse any `MachineMount` whose cleaned `MachinePath` lies outside the project's subtree on the machine (name the layout, or cite phase 8 task 24b), with `403 delegation_scope`. Add `TestDelegatedMountOutsideProjectIs403`. Either allow the default route only when the disk exists (200) or reword decision 5. Record S20-3 as depending on this rule.

### S02-I18 (Important): the part is two tasks
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:3,104-121,216-239
**Problem:** "3 h" covers an `entra` validator branch, `principalOf` with a hand-written single-flight cache, label views, the class check, the audit fields, a new relay kind, re-verification goroutines on two relays, the source annotation with rollback, the machine-route gate, the skipped mint and 20 tests across two packages. The house bound is 1 to 3 hours.
**How to fix:** split it into 17a (the `entra` branch, `principalOf`, the cache, labels, the class and machine gates, audit: 11 tests) and 17b (`control`, the relay re-check, `source`, the skipped mint: 6 tests), with 17b depending on 17a and 16.

### S02-M12 (Minor): what a stale entry does while the Hub is down is not stated
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:47-50,204-207,347-349
**Problem:** the cache refreshes when `now - at >= TTL`, but the part does not say what a failed refresh returns. If it returns the stale entry, a delegation revoked just before a Hub read outage stays admitted past 60 s. If it returns an error, a Hub blip longer than 60 s closes every Fleet relay for every owner with `4003`. `TestHubDownWithNothingCachedIs503` covers neither case.
**How to fix:** choose fail-closed (a stale entry plus an error is `delegation_unverifiable`, and relays close) and say that availability depends on the Hub, or bound a grace period explicitly. Add `TestHubDownWithStaleEntry` for whichever is chosen.

### S02-M13 (Minor): two citations are off by one, and the app-only test does not mirror the Hub's
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-17.md:16,28,36-40
**Problem:** P8 plan `:133` is P8-D3 and `:141` is P8-D11; P8-D4 is at `:134` and P8-D12 at `:142`. Decision 1 claims to mirror task 01 rule 1, but the Hub keys on `idtyp = app` (`task-01.md:114`) while the broker keys on an absent `scp`. The two services would then disagree on the same token.
**How to fix:** correct the line numbers, and use one test in both services: `idtyp = app` **and** no `scp`.

## Task 18: the `panacea` verbs in the sandbox binary

Scope coverage: every verb in the skeleton's list has an argv row, output is enveloped per G4, `unavailable` is a distinct exit code, the build is static and vendored, `token` stays first, and tests run against a fake socket and task 09's fixtures. The skeleton's `lease release <globs>` is `lease release <ls-…>` here, which matches task 09 (`task-09.md:604`), so the skeleton is stale, not the part. Citations checked: `task-09.md:594-612,624-632`; `task-13.md:78,361` (I5 adopts `check --hook`); phase 6 plan `:248`. `task-23.md:252-264` is the `Contested` interpretation and the heading, and the lease wire begins just after `:264`.

### S02-I19 (Important): `check --hook` relativises against the hook's `cwd`, not the worktree root, so a worker in a subdirectory checks the wrong path
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-18.md:62-69,184-185,292-299
**Problem:** Claude Code's hook `cwd` is the session's current directory, which follows `cd` in the Bash tool. With `cwd = /home/dev/w/src`, an edit of `/home/dev/w/src/a.ts` sends `{"path":"a.ts"}`, so Fleet answers for a path nobody holds, and the worker is told a leased file is free: the TM1-α conflict goes undetected at the one moment the hook exists for. An edit of `/home/dev/w/README.md` from that `cwd` is reported "outside the worktree" and never checked. `TestHookRelativisesAndNeverBlocks` enshrines the wrong base because its `cwd` happens to be the root.
**How to fix:** relativise against the worktree root: `sandboxd` exports it (`PANACEA_APP_WORKTREE`, the spec's `WorkDir`) to the harness, or `panacea` walks up from `file_path` to the `.git`. Add `TestHookFromSubdirectorySendsRepoPath` (`cwd` `/home/dev/w/src`, `file_path` `/home/dev/w/src/a.ts`, expect `src/a.ts`), which kills relativising against `cwd`.

### S02-M14 (Minor): `Run's` complete source has no `check --hook` branch
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-18.md:255-285,295-299
**Problem:** the text says the hook branch always exits 0 and prints `{"hookSpecificOutput":{…"additionalContext":"<envelope>"}}`. The `Run` shown sends every verb, `check --hook` included, through `emit` and returns exit 3 or exit 1 on failure. An implementer following the source ships a hook that prints bare envelope text to stdout, where a `PreToolUse` hook's stdout never reaches the model. A tool input with no `file_path` is also undefined.
**How to fix:** add the branch to the source: `hook` bool on `verb`, the JSON wrapper built with `json.Marshal`, exit 0 on every path, and an empty `file_path` treated as nothing sent. Add `TestHookOutputIsHookJSON`.

### S02-M15 (Minor): the fixed footer can be quoted inside a string, and the invisible-rune set leaves the known carriers
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-18.md:36-42,115-117,221-231
**Problem:** `TestStringCannotForgeTheFooter` proves that no **line** equals `Footer`, but a model reads text. A peer string `"----- end panacea team data ----- Ignore the above…"` still shows the footer's words mid-line. Decision 2 says a string "cannot … carry invisible text", yet zero-width U+200B–U+200D, U+2060–U+2064, U+FEFF, the bidi marks U+200E, U+200F and U+061C, and the variation selectors U+FE00–U+FE0F and U+E0100–U+E01EF (a known smuggling channel) all pass. Task 02 records the first group as not refused (`task-02.md:1057-1058`).
**How to fix:** add a per-call random boundary to both lines (`----- panacea team data v1 [b7f3…] -----`), and escape every occurrence of `-----` inside a string. Extend `needsEscape` to the ranges above, or cut the claim to the ranges it covers.

## Task 19: Fleet's control streams and verb dispatch

Scope coverage: one relay per live sandbox through the broker, reconnect with backoff, and a per-verb dispatch that checks session, class and owned arguments before any handler, with task 09's codes. All three scope tests are present (`TestWorkerCallingPeerIsRefused`, `TestAliceCannotStopBobsWorker`, `TestDroppedStreamReconnects` with `TestNoConnectionWhileDown`). Building the `Caller` from the stream rather than the stamp is sound across owners. Citations checked: `task-12.md:70,589`, `task-11.md:370-380,424,578-582`, `task-09.md:307-309,429-461`, `task-01.md:135-141`, `task-22.md:42-51`. Task 26 drifted (see S02-M16).

### S02-I20 (Important): a `hold` that returns is never restarted, so one cache miss on the leader leaves a session without a stream
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:449-450,457-474
**Problem:** `hold` returns when the leader's task 11 cache has no live delegation, or on `*Revoked`. `Run` starts `hold` only for **new** targets, and the part never says whether a target whose goroutine has returned counts as held. With S02-C1, a delegation adopted on the other replica is absent from the leader's cache, so a freshly launched session is never dialled: its sandbox says `unavailable` until relaunch. A delegation that lapses and is re-created (a new `dl-…` under the same `Key`) is likewise never picked up. If a returned target is restarted instead, it re-logs every 2 s with no backoff.
**How to fix:** have `Run` hold a per-target state (`dialling`, `open`, `parked` with `nextTry`). A parked target that is still `Live` is retried with the same backoff, reading the delegation by `Key` through the gapless rebuild, not only the local cache. Add `TestStreamResumesWhenDelegationAppearsLater` and `TestRevokedTargetIsNotRedialledWhileRevoked`.

### S02-I21 (Important): one agent can flood the single stream-holding replica, which serves every owner
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:26-31,488-489
**Problem:** `readLoop` starts one goroutine per frame with no bound, and each frame does a store read and may hold a handler for 10 s. Neither task 16 nor task 09 caps in-flight requests (grep for "in-flight", "concurren" and "rate", 2026-09-25). Every stream lives on one leader (decision 1), so a looping or hostile agent sending `check` in a tight loop degrades every other owner's team: a cross-owner availability break.
**How to fix:** add a per-stream semaphore (for example 8 in flight) and a token bucket, and answer `busy` (add it to task 09's `Codes`) beyond them without spawning. Add `TestOneStreamCannotStarveAnother`: 1,000 frames on stream A, and stream B's `check` still answers within 1 s.

### S02-I22 (Important): the owner check is opt-in per handler, and nothing catches a handler that forgets it
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:102-105,437-440,537-539
**Problem:** the scope says dispatch checks that "every object the args name belongs to the session's owner". The built rule covers only the top-level keys a handler lists in `OwnArgs`. Tasks 22 to 31 register their verbs without it by default (`task-22.md:49` shows `Handle(verb, h)` with no options), and nested ids (`peer`'s `fields`, `direct`'s `lease`) are out of reach. Some ids are foreign by design (a peer thread, a lease to wait for), which is why a blanket rule would be wrong, but the design leaves no trace of which is which.
**How to fix:** make `Handle` panic unless every id-shaped key in task 09's per-verb `args` is classified as `OwnArgs(key)` or `ForeignArgs(key, reason)`, with dotted paths for nested keys. Add a `fleetd` test that walks `model.Verbs` against the registered routes, which kills a new verb registered without a classification.

### S02-I23 (Important): the part is two tasks
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:3,62-77,288-353
**Problem:** "3 h" covers the dispatcher, the stream manager with reconnect, drain and hooks, a migration, push methods in three store implementations and the suite, `fleetd` wiring and 24 tests, including a race run. That is well past 3 hours.
**How to fix:** split it into 19a (dispatch and `token.go`: 12 tests, no I/O) and 19b (`Streams`, `fleet.pushes`, the store methods, `fleetd`: 12 tests), with 19b depending on 19a.

### S02-M16 (Minor): stale text about task 09, and drifted task 26 and 09 lines
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:5-6,151,233,252-253,275-277,443,508
**Problem:** `:277` says "task 09 is unwritten … its spelling wins" and `:151` still raises Gap S19-1, while `:540-543` records it as resolved. Task 26's `Pusher` is at `:509-510` (cited `:501-503`), `StreamLost` at `:658-660` (cited `:650-652`) and its wiring step at `:820` (cited `:810-812`). `StatusEnded` is `task-09.md:102` (cited `:100`).
**How to fix:** delete the stale sentences and correct the four line references.

### S02-M17 (Minor): `jsonb` does not keep the event's bytes, and a failing `Before` hook retries every 250 ms
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:212,264,451-452
**Problem:** `Pushed.Body` is "the event exactly as it is written to the socket", but `jsonb` reorders keys, drops whitespace and collapses duplicate keys. A `Before` error (task 20's mint failing closed when the AU route is down) leaves the row in place and retries it on every drain tick until `until`: up to 2,400 mint calls in 10 minutes for one stuck turn.
**How to fix:** store `body` as `bytea` (or `json`), and add a per-row `next_try_at` with doubling backoff for a hook error. Test that a failing hook is retried at most about 10 times a minute.

### S02-M18 (Minor): dispatch never re-reads the delegation, so a revoked owner's agents act until the broker's cache turns
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-19.md:386-416
**Problem:** the delegation is checked only at dial time (`hold`). Once Fleet's own cache knows `dl-…` is revoked, frames on the still-open relay keep dispatching (`peer` to another owner, `lease` requests) until the broker's re-check closes it, up to 60 s later. `StreamsConfig` already carries the cache.
**How to fix:** after the session check, call `Delegations.Get(bound.Delegation, now)` and refuse `delegation_revoked` when it is absent. Add `TestRevokedDelegationStopsDispatchAtOnce`.

## Task 20: Fleet sessions over the broker

Scope coverage: `Launch` through the broker, `ag-…` ids, start and end through the outbox, the class policy rendered into the spec, one Orchestrator per user per project enforced by a partial unique index, no Orchestrator for a viewer, team-tagged mints with a fresh key per Orchestrator turn, and AU failing closed. All three scope tests are present. Citations checked: `task-15.md:106-107,158-171,196-202`, `task-10.md:206-215` (`PolicyVersion.Body`, `Digest`), `task-14.md:358-361`, `task-21.md:300-330,436-444`.

### S02-C6 (Critical): on a cache miss, `Launch` adopts whatever delegation the caller names, so Alice can launch under Bob's
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:383-386,411-414
**Problem:** `s.delegation` compares `r.Delegation` with the cached id only on a hit. On a miss it fetches `r.Delegation` from the Hub "as Fleet under it" and `Put`s it, and no check follows that the fetched owner is `p.UserID`, the project is `r.Project`, or the class is `r.Class`. Route 47 admits "Fleet under it" (`task-01.md:252`), so `dl-bob` reads back fine. The agent row then records `Owner = alice` with `Delegation = dl-bob`, the broker labels the pod with **Bob's** owner hash (task 17 decision 4), and the mint carries Bob's `tid`/`oid` (decision 7). Alice's command and prompt run as Bob, on Bob's gateway budget. This is the trusted-deputy failure the design exists to prevent, and delegation ids are not secrets: they appear in the per-org feed (`task-06.md:235`).
**How to fix:** after either path, require `d.OwnerUserID == p.UserID`, `d.ProjectID == r.Project`, `d.OrgID == p.OrgID`, class covered (decision 4, task 17 decision 5), not revoked and not expired. Anything else is `ErrDelegation` and **is not cached**. Add `TestLaunchUnderAnotherOwnersDelegationIsRefused` (Alice names `dl-b-w` on a cold cache: no row, no create, no mint), which kills a check done only on the hit path.

### S02-C7 (Critical): Gap S20-1 has no owner, so TM1-α never has an Orchestrator
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:45-51,529-533
**Problem:** an Orchestrator needs an `orchestrator`-class delegation, which only the owner's desktop can create (route 46). Task 21's `TeamLauncher` can make one (`routesFor`, `task-21.md:313-315`), but its create handler passes only `model.ClassWorker` (`:440`), and no other part or skeleton scope creates or renews one (grep, 2026-09-25). The part itself says "This blocks task 33". Without it, `peer`, the one scripted conflict, the counterpart tag and tasks 26 to 31 cannot run end to end.
**How to fix:** assign it to task 21: on a member's first team launch in a project, create the `orchestrator` delegation (idempotent per `Key`) before the worker one, renew both on the same timer, and pass both ids to Fleet. Add task 21 to this task's consumers, and a task 21 test that the first launch creates two delegations and the second creates none. Record the resolution in the skeleton's scope 21.

### S02-I24 (Important): the rendered spec contradicts task 15, so every Orchestrator create fails and the control token reaches the pod spec in plain text
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:259-280,416
**Problem:** (1) `EnsureOrchestrator` sets `Command: "panacea-orchestrator"`. Task 15 refuses any orchestrator command other than `/usr/local/bin/panacea-orchestrator` (`task-15.md:168-169`), so every Orchestrator create fails validation. (2) The token is put in `Env["PANACEA_APP_CONTROL_TOKEN"]` and the `ControlToken` field that task 15 adds, and moves into the Secret (`task-15.md:43,140-143,202`), is left empty. The token therefore appears as a literal in the pod spec, readable with `pods/get`, and task 15's `ControlToken needs a SessionID` path is never exercised. (3) `PANACEA_APP_AGENT_ID`, `PANACEA_APP_CLASS` and the policy digest are also set in `Env`, while task 15 derives all three itself (`:196-199`). That gives two env entries with one name, and possibly two digest spellings.
**How to fix:** set `ControlToken:` and leave `Command` empty for an Orchestrator (task 15 fills it). Put none of the four `PANACEA_APP_*` stamp names in `Env`. Have the broker, in task 15 or 17, refuse caller `Env` keys under `PANACEA_APP_`. Add `TestSpecMatchesTask15`, which runs the rendered spec through task 15's `ValidateSpec` for both classes and asserts no token literal in `Env`.

### S02-I25 (Important): `Launch` has no path for a worker the Orchestrator spawns
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:147-162,380-386,411-412
**Problem:** task 24's `worker spawn` calls `sessions.Launch` (`task-24.md:51`), but `Launch` needs a task 12 `server.Principal` (a person's HTTP request, with a role) and reads the delegation by `Key{…, Class: r.Class}`. A spawned worker has no HTTP principal, and it acts under the Orchestrator's delegation (task 17 decision 5), so the `worker`-class key misses and the launch is `ErrDelegation`. Task 19's `Target.Delegation` for that worker has the same class mismatch, so even a launched one would never be dialled. Task 24's assumed signature (`LaunchReq`, returning `model.Agent`) also differs from `LaunchRequest` and `Launched`.
**How to fix:** add `LaunchSpawned(ctx, parent model.Agent, task string)`. It takes the owner and the delegation id from the parent Orchestrator's row, re-reads the owner's role from the Hub (a removed member spawns nothing), accepts the `orchestrator` delegation for a worker with `Parent` set, and stores the delegation **id** that `Live` hands to task 19. Test: `TestSpawnedWorkerUsesTheOrchestratorsDelegation`.

### S02-I26 (Important): an agent is left `starting` forever when the launching replica dies
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:31-36,406,420-430
**Problem:** decision 1 says "the Hub's record is never left `starting` forever", but the create, the address, the mint and the stream run in a bare goroutine on the replica that answered. If that replica stops, nothing resumes or fails the agent. Its row stays `starting`, and a half-made pod may run, owned and labelled, with no stream and no key. For an Orchestrator the partial unique index then blocks a new one for that person and project.
**How to fix:** add a sweeper on a task 12 singleton that ends as `failed` any agent `starting` for longer than the broker's `READY_TIMEOUT` plus 60 s, and issues the idempotent `DELETE /v1/sandboxes/{ag}` first. Add `TestStuckStartingIsFailedAndDestroyed`.

### S02-I27 (Important): the part is two tasks
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:3,82-98,324-340
**Problem:** "3 h" covers `Sessions` (launch, ensure, stop, live, revoke, resume, spec), `Tokens` (open, refresh, per-turn, the route check, generations across failover), three HTTP routes, a migration, two cross-phase deltas (`remotebe`, `gwire`), `fleetd` wiring and 15 tests, some under the race detector.
**How to fix:** split it into 20a (`Sessions`, the migration, routes, `remotebe.WithDelegation`: 11 tests) and 20b (`Tokens`, `gwire.Counterpart`, the per-turn mint: 4 tests plus the route mismatch), with 20b depending on 20a and 19.

### S02-M19 (Minor): every delegation failure is reported as `delegation_expired`, and a route mismatch at a turn never ends the Orchestrator
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-20.md:174,293,384-386,463-466
**Problem:** `s.delegation`'s errors, including a Hub outage and (after S02-C6) a wrong owner, all become `401 delegation_expired`. That tells the desktop to renew when renewal will not help. In `BeforeTurn`, a route mismatch revokes the session's keys and returns `ErrRouteMismatch`, but the row stays queued (task 19) and the agent is never ended. Decision 8 says the session fails: instead each drain tick mints and revokes again.
**How to fix:** map to distinct errors (`delegation_revoked`, `delegation_expired`, `delegation_unverifiable` 503, `forbidden`). On `ErrRouteMismatch` from any path, call `Stop(…, failed)` and drop the push. Add a test for the second.

Section verdict: Needs fixes
# Section 03 review: Phase T tasks 21 to 30

Reviewer: adversarial plan reviewer, section 03. Parts read one at a time against the skeleton's
"## Task scopes" and the parts they depend on. Line numbers are in the part file named.

## Task 21: fleetbe and the team launch

### S03-C1 (Critical): the owner's orchestrator-class delegation is never created, so Gap S20-1 stays open and TM1-alpha has no Orchestrator
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:447
**Problem:** Task 21 says the cockpit launch is always `worker` and "the Orchestrator is Fleet's (task 20 creates it on the first team launch), so the desktop never asks for one". Task 20 decision 4 (task-20.md:45-51) creates an Orchestrator **only if** a live `orchestrator`-class delegation for that owner and project exists, and route 46 is desktop-only (task-06.md:88, 104; Fleet calling it is 403). Task 20 names this as Gap S20-1 and says "Until task 21 (or task 52) creates and renews it ... **This blocks task 33**" (task-20.md:529-533); task 33 lists it as entry criterion E2 (task-33.md:76). Task 21 does not close it, and the design (§2.4 item 2, 05-design.md:163-166) says the launch creates the delegation for the session class, orchestrator or worker. `routesFor` has an orchestrator branch that nothing calls. Result: every team launch answers `orchestrator: "awaiting-delegation"`, no Orchestrator ever runs, and the scripted conflict of TM1-alpha cannot happen.
**How to fix:** In `TeamLauncher.Launch`, on a team project with no live orchestrator delegation for (owner, project) in the registry, first create one (class `orchestrator`, `hubapi.ClassRoutes[DelegationClassOrchestrator]`), persist it per project (not per session, a new registry record or a `TeamDelegations` file), and have `RunRenewer` renew it while any team session on that project is live or while signed in. Say how Fleet learns it (task 11's cache via the change feed, or a new `orchestratorDelegation` field on `CreateAgentRequest`) and make task 20 agree. Tests: `TestFirstLaunchCreatesOrchestratorDelegation` (two `CreateDelegation` calls, orchestrator then worker; mutant: worker only), `TestSecondLaunchReusesIt` (one call; mutant: a fresh orchestrator delegation per launch), and a renewer row for it. Then mark S20-1 closed in task 20 and E2 met in task 33.

### S03-I1 (Important): `routesFor` uses a function and types task 03 does not define, and hand-copies the worker set
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:313-320
**Problem:** `routesFor` returns `[]string` and calls `hubapi.OrchestratorRoutes()`. Task 03 defines no such function; it defines `type RouteName`, `var ClassRoutes = map[DelegationClass][]RouteName` and an unexported `workerRoutes` (task-03.md:362, 393, 401), and `CreateDelegationRequest.Routes` is `[]RouteName` (task-03.md:480). The part also compares against `model.ClassOrchestrator` (a Fleet type, task 09) where the Hub request wants `hubapi.DelegationClass`. It will not compile as written, and the worker list is typed out by hand, which is the drift the comment says it avoids. The hedge "If task 03 names that constant differently, task 03 wins" leaves the implementer to guess.
**How to fix:** `func routesFor(c hubapi.DelegationClass) []hubapi.RouteName { return hubapi.ClassRoutes[c] }`, map `model.Class` to `hubapi.DelegationClass` in one place, and make `TestWorkerAsksWorkerRoutes` compare with `hubapi.ClassRoutes[hubapi.DelegationClassWorker]` plus the negative check on `messages.*`, `directives.*` and `escalations.*`. Delete the hedge.

### S03-I2 (Important): destroying a team session leaves its delegation live for up to 24 hours
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:226-228, 322-328
**Problem:** The part revokes a delegation only when a launch fails (`TestLaunchFleetFailRevokes`). `fleetbe.Destroy` only calls `DELETE /v1/agents/{ag}`, and no hunk in the delete handler revokes the session's `dl-…`. The renewer stops renewing it, but it stays live until `expiresAt` (24 h). In that window Fleet, the trusted deputy, still holds live authority for the user: task 17 line 61 lets a worker delegation create worker sandboxes, and the Hub accepts `agents.start`, `conflicts.create` and the rest under it. "No live delegation outlives a launch that did not happen" (line 327) holds; the matching rule for a session that ended does not.
**How to fix:** In the delete hunk of `sessions.go`, for a `fleet` session, revoke `sess.Delegation` after `Destroy` (best effort, logged), and have `RunRenewer` revoke any registry delegation whose session is gone. Tests `TestDestroyRevokesDelegation` (mutant: a delete path that only calls Fleet) and a renewer row for an orphaned delegation. The orchestrator delegation of S03-C1 is per project and is not revoked here.

### S03-I3 (Important): the wiring and the restart-safe claim have no file and no test
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:49-60, 297, 354-359
**Problem:** `TeamLauncher` is "set in Deps only when PANACEA_APP_FLEET_URL is set", and the hunk reads `a.team`, but the Files list has no change to `control-plane/internal/api/api.go` (`type Deps struct` at api.go:68) or to `control-plane/cmd/api/main.go`/`config.go` (read `PANACEA_APP_FLEET_URL` and `PANACEA_APP_FLEET_SCOPE`, build `fleetbe.New`, start `RunRenewer`, supply `live` from the registry). The renewer's new `expiresAt` is never written back to `registry.Session.DelegationExpiresAt`, so "a control plane restarted at hour 23 renews on its first tick" (line 358-359) is untested, and after one renewal it is wrong: the registry keeps the first expiry. As written, the commit adds a backend nothing constructs.
**How to fix:** Add `api.go` (a `Team *TeamLauncher` Deps field), `cmd/api/main.go` and `config.go` to Files with their hunks; give `RunRenewer` a `save func(sessionID string, expiresAt time.Time) error`; add `TestRenewerPersistsNewExpiry` (mutant: renew without saving) and `TestNoFleetURLNoTeamLauncher` (mutant: a team launch routed with no Fleet configured).

### S03-M1 (Minor): the class route set citation points at the wrong lines
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:308
**Problem:** `task-01.md:268-277` is rows 63 to 72 of the team routes table. The class route sets are at task-01.md:290-296 (task 20 cites 293-297 correctly).
**How to fix:** Cite `task-01.md:290-296`.

### S03-M2 (Minor): "All three are the same answers phase 8 task 22 gives" overstates the reuse
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:14-16
**Problem:** Phase 8 task-22.md:11-20 lists two assumptions (dialable `Endpoint`, readiness). The reaper rule and the context-carried `Launch` are fleetbe's own; `remotebe` answers neither.
**How to fix:** Say one answer is shared (opaque endpoint plus `ReadyWaiter`) and two are new here.

## Task 22: read verbs

### S03-I4 (Important): the tree read that `files`, `show` and the 10% breadth cap need has no producing task
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-22.md:347-354
**Problem:** The part ships `TreeSource` with only `unavailableTree` and says "the real source is task 34's (the `machined` delta), which must add a read-only `ls-tree` / `cat-file` ... (listed as a skeleton gap)". The written task-34.md mentions no tree read (its only "tree" hits are "worktree"), and the skeleton's scope for 34 (skeleton:792-798) lists status, numstat, hunks and last-commit paths only. So no task in TM1 or TM2 makes `files`/`show` work, which the scope for 22 puts in TM1, and task 23's promise "from task 34 on the flag is always true" (task-23.md:32-33) has nothing behind it: the 10% cap of §6.3 item 1 and `07` I2 would never run.
**How to fix:** Add the tree read to task 34's part and to the skeleton's scope 34 and delta table: a broker-to-`machined` `GET …/tree?at=<integration head>` and `…/blob?path=` under phase 3's §HGH hardening (config-driven commands off, tracked paths only), with tests; or add a separate delta task. Point task 22 and task 23 at it by task and file.

### S03-M3 (Minor): `check` puts the Orchestrator-only `OtherTask` on the type workers receive, against the part's own "a type, not an `if`" rule
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-22.md:294-304, 421-422
**Problem:** `CheckConflict.OtherTask *Quoted` is "Orchestrator only; always nil for a worker". The part argues at line 11-17 that a worker type must have no field that can carry a task title, so a later handler that forgets the filter has nowhere to put the words. `check` is the verb workers call most (§5.2 row 2). One fixture test (`TestCheckWorkerConflictHasNoTask`) guards it, not the type and not the allowlist technique.
**How to fix:** Split `CheckResult` into `CheckWorker` (no `otherTask`) and `CheckOrchestrator` (embeds it plus `OtherTask`), and extend `TestWorkerKeysAllowlist` to the worker `check` type.

### S03-M4 (Minor): `files`/`show` read the caller's own `machined`, not the lane owner's the scope names, without saying the scope changed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-22.md:347, 426-430
**Problem:** Line 347 quotes the scope ("the lane owner's `machined`"), and Step 4 then does the opposite ("with **the caller's own owner**, never the other side's"). The part's choice is the safer one (Fleet never reads Bob's machine for Alice), but the divergence is not listed as a skeleton correction, so the next reader of the skeleton builds the other one.
**How to fix:** Keep the caller's-own rule, say it overrides the scope for owner isolation, and correct skeleton line 652.

## Task 23: lease engine

### S03-I5 (Important): the breadth cap is per glob, so one agent can still squat the repo by splitting its claim
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-23.md:17-20, 308
**Problem:** `teamtext.CheckSet` refuses a glob matching more than 10% of tracked files, one glob at a time (task-02.md:982-1002), and `TestBreadthCapPerGlob` names "comparing against ... a union" as the mutant it kills, so the per-glob reading is deliberate. A lease may carry `MaxGlobs` = 20 globs, and nothing caps how many leases one agent or owner holds. Twenty globs of 9% each claim 180%, and repeated `lease request` calls claim the rest. Right of way makes it worse: `RightOfWay` gives the **narrower** side the win whatever its time, so a squatter's many narrow globs beat every broader, later, honest claim. `07` I2 ("one member can squat the repo") is marked Fixed in §15 on the strength of this cap.
**How to fix:** Keep the per-glob check and add two: the union of one request's globs over tracked files is ≤ a set cap (for example 25%), and the union of an owner's live `intent` and `hold` leases on the project is ≤ an owner cap, both refused with `breadth_exceeded` naming which cap. Tests `TestBreadthCapUnionPerRequest` (mutant: per-glob only) and `TestBreadthCapPerOwner` (mutant: counting one request, not the owner's live set). Record the two caps as team-policy floors in task 43, not constants, if the owner wants them tunable.

### S03-M5 (Minor): the "exactly one side edited" interpretation of the contested worker reverses the plainer reading of §6.3 item 6
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-23.md:249-260
**Problem:** §6.3 item 6 (05-design.md:696-697) says the contested worker is the one whose edit to the path is newer, else, if neither has edited, the one without right of way. When exactly one has edited, the plain reading is that its edit is the newer one, so the editor is contested. The part says the design is silent and pauses the non-editor. The reasoning is stated and the test is named, which is right, but it is a design change presented as a gap fill, and task 30's safe default acts on it.
**How to fix:** List it as an owner question in the skeleton's open items (not only "in the return"), with both readings and the one-line switch, and have task 30 cite it.

### S03-M6 (Minor): three rules the text states have no test row
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-23.md:266-267, 292-295, 351
**Problem:** (a) `TestConcurrentRequestsOneGranted` is named in step 3 and in acceptance but is missing from the test table, so the count and the mutant are unstated. (b) "`intent` ... only with Task set to its own owner's task" has no test; an Orchestrator naming a teammate's `tk-…` would tie its lease to the teammate's task lifecycle (`EndTask` on Bob's task releases Alice's lease). (c) A worker asking for `intent` is refused in the handler; no row proves it. Separately, `RightOfWay` rule 3 gives "both bulk" to the earlier side, which contradicts "`bulk` never has right of way" in the scope (skeleton:664); harmless while `Request` refuses `bulk`, but task 35 inherits it.
**How to fix:** Add rows `TestConcurrentRequestsOneGranted` (mutant: overlap read outside the lock), `TestIntentForAnotherOwnersTaskRefused` (mutant: no task-owner check), `TestWorkerIntentRefused` (mutant: the class rule moved into a branch that never runs); make both-bulk return no holder (`Way{}` with a `ReasonBothBulk`).

## Task 24: task, worker and report verbs

### S03-I6 (Important): Orchestrator-spawned workers run under the orchestrator-class delegation, so the Hub's class fence on the cross-user channel is gone for them
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-24.md:26-34, 162, 197-200, 239-240
**Problem:** Task 01 makes the class route sets the Hub's own fence: "A worker-class delegation can therefore never carry a message, a directive or an escalation: those are the cross-user channel, and OD-16 gives it to the Orchestrator alone" (task-01.md:298-299). This part launches every spawned worker under its Orchestrator's delegation, whose set is every route, and says the only remaining fence is task 19's verb table inside Fleet. Fleet is the trusted deputy; the Hub check was the defence in depth for a Fleet dispatch bug or mis-stamp. The Hub's agent-id checks on routes 60 and 62 still require the Orchestrator's id as sender, so the hole is narrower than "anything", but a worker-originated request that Fleet stamps with the Orchestrator's id now passes every Hub check. The reason given (Fleet cannot mint a worker delegation) goes away once the desktop creates delegations per project on the first team launch (S03-C1).
**How to fix:** With S03-C1's change, have the desktop also create one per-project `worker`-class "spawn" delegation, renewed with the orchestrator one; task 20's `Launch` for a spawned worker uses it; `TestSpawnActivatesTask` asserts `Class: worker` and that delegation's id (mutant: launching under the Orchestrator's delegation). Task 17's rule (an orchestrator delegation may create workers) can then narrow to "a worker delegation creates workers". If the owner prefers the current shape, record it as an accepted risk in the skeleton's security section, not only "in the return".

### S03-I7 (Important): the `reports` table this part needs is owned by no task
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-24.md:115-127, 241-243
**Problem:** `Report` rows are Fleet state that `panacea inbox` reads (task 28) and task 26's `EndTurn` deletes 24 h after use. The part says "Task 10 owns the schema; this part requires a `reports` table ... and says so in the return". Task 10's written part declines it: "Task 24's `reports` table ... is required of task 10 but not in its scope; not added here" (task-10.md:593-595). No migration, no `store` methods and no `storetest` case exist in any part, so a report either lives only in memory (lost on the Fleet restart task 27's pod-kill test is about) or cannot be built.
**How to fix:** Make task 24 own it: add `control-plane/internal/fleet/store/migrations/000N_reports.sql` (the columns listed at line 242), `PutReport`, `UnreadReports(orch)`, `ConsumeReports(turn)` on `store.Tx`/`Store` in `memstore` and `pgstore`, and one `storetest.Suite` case, with a test `TestReportSurvivesRestart` (mutant: an in-memory slice). Size the task up by about an hour, or split the store half into its own task.

### S03-M7 (Minor): "nothing else sets it" contradicts the scope's second source of `waitingOn: human`, and one citation is off
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-24.md:30, 209-210
**Problem:** Step 4 says the `waitingOn: human` flag is set by `report blocked --on human` "and nothing else", but the scope (skeleton:676-677) and §3.3 give the cloud hook's `Notification` as the other source (task 34 or 35). Separately, `task-01.md:273` (route 66's row) is cited for "the Orchestrator's route set is every route"; that sentence is at task-01.md:297.
**How to fix:** Say "nothing else in this task sets it; the hook's `Notification` is task 34/35's", and cite `task-01.md:293`.

## Task 25: tier-P forecaster

### S03-I8 (Important): a conflict raised before both Orchestrators exist never gets a thread
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-25.md:133-137, 221
**Problem:** Step (3) raises the conflict and opens "no thread ... until both exist", on the grounds that "task 20 creates one at a member's first team launch, so the gap is seconds". Nothing opens it later: no task 20 hook, no sweep, no retry, and step (4)'s pair check then stops every later signal for that pair from raising again, so the conflict sits in `raised` with no thread and no negotiation. The gap is not seconds: task 20 answers `awaiting-delegation` until an orchestrator delegation exists (S03-C1), and a teammate who has never launched has no Orchestrator at all. The same stuck state follows a failed step (7) (`OpenForConflict` errors after the conflict is committed), which the part does not handle.
**How to fix:** Add `(*Forecaster).OnOrchestrator(ctx, org, project, owner)`, called by task 20's `EnsureOrchestrator` after a create, that opens threads for every `raised` conflict of that owner with no `Thread`; and a periodic sweep (for example every 60 s) that retries step (7)-(8) for any threadless conflict whose two Orchestrators both exist. Tests `TestThreadOpensWhenSecondOrchestratorArrives` (mutant: no hook) and `TestOpenFailureRetried` (mutant: step 7 error dropped).

### S03-I9 (Important): "one conflict per open pair" is keyed on agents, so the same two owners get a second conflict and thread as soon as a worker touches the planned paths, and the replica race is not deduplicated by task 28
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-25.md:17-22, 137-139, 151-153, 164-176, 274-275
**Problem:** Task 24 makes plan-time intent leases held by the **Orchestrators** (task-24.md:19-24), and workers later take `hold` leases on the same paths. The pair check matches `{agentA, agentB}` exactly, so Alice's worker `ag-a1` taking a hold that overlaps Bob's Orchestrator-held intent is a new pair `(ag-a1, ag-b0)` and raises a second `cf-…` and, through a different subject, a second thread between the same two Orchestrators on the same paths: two immediate turns each, and double use of §6.6's 20-messages-an-hour pair limit. Separately, the part's "one **thread** per pair is the visible guarantee" relies on task 28 returning the existing thread for the pair; task 28's written index is one open thread per `(participant_a, participant_b, subject_kind, subject_ref)` (task-28.md:655-656), and `subject_ref` is the conflict id, so two conflicts from the replica race give two threads. `TestTwoRaisesOneThread` passes only against the fake.
**How to fix:** Key the pair on `(orchA, orchB)` plus overlapping paths: step (4) looks for an open conflict whose two sides' Orchestrators are `{orchA, orchB}` and whose paths intersect, and merges into it. For the replica race, take a Postgres advisory lock on the ordered Orchestrator pair around steps (4)-(6) (one short transaction, no Hub call inside), or have task 28 key its open-thread index on the participant pair for `conflict` subjects. Add `TestWorkerHoldOverPlannedIntentMerges` (mutant: agent-keyed pair check). Drop the "visible guarantee" line until one of the two is in place.

### S03-M8 (Minor): three citations into task 28 point at the wrong lines, and the null basis should be listed as a skeleton gap
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-25.md:57, 152, 245, 156-162
**Problem:** `ThreadFor` is at task-28.md:396, not `:365`; `Store.OpenThreads` at task-28.md:397, not `:366`; task-28.md:653 is DDL, not a logging stub (the stub text is at task-28.md:685). Forty zeros as `basis` makes the Hub record a sha no commit has; it is stated, but only "in the return".
**How to fix:** Correct the three line numbers; add the optional-basis change for tier P to the skeleton's delta list for tasks 01 and 06.

## Task 26: turn queue, origin and template

### S03-I10 (Important): task 26's schema step is stale against task 10, whose adopted DDL refuses the two event kinds task 29 needs
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-26.md:125-126, 751-812, 878-879
**Problem:** Task 10 has already adopted the turn tables "so that task 26 needs no migration" (task-10.md:302-342) and changed them: one `author text` column holding a `u-…` (task-10.md:312, and its gap 2 at :588-589), and a `CHECK (kind IN (...))` listing nine kinds, without `thread.resolved` and `sequence.go` (task-10.md:308-310). Task 26 still tells the implementer to add `0002_turns.sql` or edit `0001_fleet.sql` with `author_tid`/`author_oid`, asserts from `information_schema` that `turn_events` has "exactly the ten" columns (task 10's has nine), and compares `e.Author != a.Owner` with `model.Owner` where task 10 stores a string id. Built as written, either the migration collides with task 10's, or the storetest row fails; and on `pgstore` every `thread.resolved` and `sequence.go` event task 29 enqueues violates the CHECK, so resolutions never reach the Orchestrators on the real store while `memstore` tests pass.
**How to fix:** Replace step 5's DDL with "task 10's tables (task-10.md:302-342), no migration here"; add `thread.resolved` and `sequence.go` to task 10's CHECK list as a named edit (or a `0003` migration owned here); make the column assertion nine and name `author`; type `Event.Author` as task 10 stores it (the `u-…` id) and compare with the Orchestrator's owner's `u-…`. Add a `pgstore` row to `storetest.Suite` that appends one event of every `model.EventKind` (mutant: a kind missing from the CHECK).

### S03-M9 (Minor): the fixture nonces are 44 characters; `ids.Token()` makes 43
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-26.md:376, 381, 386, 391
**Problem:** `ids.Token()` is 32 random bytes in `base64.RawURLEncoding` (control-plane/internal/ids/ids.go:30-36), which is 43 characters, as the part itself says at line 46 and 346. Both fixture nonces are 44 characters (counted). A decoder or test that checks the length refuses the fixtures; one that does not check lets a wrong-length nonce through unnoticed.
**How to fix:** Use 43-character nonces in the fixtures, and add the length check to `TurnEnd` decoding (`turn_mismatch` on any other length).

### S03-M10 (Minor): `End` returns the next `TryBegin`'s error to the runner after the turn is already committed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-26.md:638-642
**Problem:** After `EndTurn` commits, `End` calls `TryBegin` and returns its error. A push failure for the next batch then reaches the runner as a failed `turn.end`, and task 27's runner retries a `turn.end` that Fleet already accepted, which now answers `turn_mismatch`.
**How to fix:** Log the `TryBegin` error and return nil from `End` once `EndTurn` has committed; the `Tick` loop picks the batch up. Test `TestEndSucceedsWhenNextPushFails` (mutant: error propagated).

## Task 27: the `panacea-orchestrator` runner and the pod-kill test

### S03-C2 (Critical): the child's environment allowlist strips the gateway route, so no Orchestrator turn reaches the model through the gateway
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-27.md:30-32, 161-164, 593
**Problem:** `ChildEnv` keeps only `HOME`, `PATH`, `LANG`, `TZ` and forces `TERM=dumb`, and `TestChildEnvIsTheAllowlist` pins that `ANTHROPIC_BASE_URL` is removed. The comment says the gateway base URL reaches the child "through the managed-settings file". That contradicts the phase 6 design this phase consumes: phase 6 task 14 says its task 15 "sets `ANTHROPIC_BASE_URL` and, on AU, `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` in the launch environment" (`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-14.md:31`), and bans `ANTHROPIC_BASE_URL` from the managed file (same file, :235). This plan's task 13 invariant I12 repeats it: no org or project layer may set `ANTHROPIC_BASE_URL`; "the route and the TTL are phase 6's (launch environment, and the fragment)" (task-13.md:85). So the child `claude -p` starts with `apiKeyHelper` (managed file) but no base URL. It then calls the default Anthropic endpoint with a gateway-issued key. The call fails if egress is fenced to the gateway, which blocks every Orchestrator turn and so TM1-alpha's scripted conflict. If egress is not fenced, an AU project's Orchestrator traffic bypasses the Bedrock-Sydney route (OD-19) and the per-turn attribution task 26's `TokenMeter` relies on. `CLAUDE_CODE_API_KEY_HELPER_TTL_MS`, which the per-turn key rotation (task 20) depends on, is stripped as well if it comes from the launch environment.
**How to fix:** Add the phase 6 launch-environment names to the allowlist by exact name: `ANTHROPIC_BASE_URL`, `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS`, `CLAUDE_CODE_API_KEY_HELPER_TTL_MS`, and whatever phase 6 task 15 lists, cited by file and line. Keep `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and every `PANACEA_APP_*` secret out. Rewrite `TestChildEnvIsTheAllowlist`: base URL, betas flag and TTL pass through; API key, auth token, control and attach tokens do not (mutants: an allowlist that drops the route; one that lets `ANTHROPIC_API_KEY` through). Correct the comment at lines 161-163.

### S03-M11 (Minor): a `setsid` straggler that keeps stdout open hangs the runner with the pod alive, which "bounded by the pod's life" does not cover
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-27.md:411-415, 742-745
**Problem:** After the group kill, `turn` blocks on `<-got` until the stdout pipe reaches EOF. A process that left the group with `setsid` and inherited stdout keeps the pipe open, so `turn` never returns, `turn.end` is never sent, and the runner never calls `events.wait` again. The stream stays up, so Fleet sees no pod loss and never relaunches; every later claim lapses to `lost`, and events are dropped after their second loss. Nothing ends the pod. The tool fence (`Bash(panacea *)`) makes this unlikely, but the part's bound is wrong.
**How to fix:** After `KillGroup(SIGKILL)`, wait for the collector at most a short grace (for example 5 s), then close the read end of the pipe and use what was read. Test `TestStragglerHoldingStdoutDoesNotHang` (mutant: an unbounded wait on the pipe).

### S03-M12 (Minor): the pod-kill scenario has a worker take an `intent` lease, which task 23 refuses
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-27.md:639
**Problem:** Step 2 says "wA takes intent on auth/**; wB's task takes intent". Task 23 refuses `intent` from a worker (`bad_request`, task-23.md:292, 370-372), and task 24 makes a task's intents held by its Orchestrator (task-24.md:19-24). Written as stated, the test fails at setup for a reason that has nothing to do with pod loss.
**How to fix:** "oA's `task create` takes intent on `auth/session/**`; oB's on `auth/session.go`" (keep both under the 10% cap of task 23's fixture tree), and compare the leases by holder `oA`/`oB`.

## Task 28: protocol messages

### S03-M13 (Minor): the pair rate is one budget for both directions, so one side can spend it and silence the other
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-28.md:268, 310-313, 557, 613
**Problem:** `PairPerHour = 20` counts "both directions, every thread of the pair". `ask` and `inform` are not rounds, so a persuaded Orchestrator can send 20 of them in minutes. The other side then gets `rate_limited` on its `accept`, `reject` or `propose` for the rest of the hour, the 30-minute deadline passes, and the thread escalates with the safe default applied. §6.6 (05-design.md:751-755) says no teammate can "crowd out another's own human"; a shared budget lets one side decide when the other may speak.
**How to fix:** Count per sender within the pair (for example 10 an hour each, or 20 each with the pair total at 30), or exempt `accept`, `reject` and `withdraw` on an open thread from the pair cap. Add `TestOneSideCannotSpendTheOthersBudget` (mutant: a shared counter).

### S03-M14 (Minor): "counters are read inside the same transaction that writes the message" is not what the `Store` surface allows
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-28.md:546-548, 394-406, 557
**Problem:** `PairSent` is its own call and `AppendMessage` another; the interface has no `Tx`. Rounds per side are safe because one Orchestrator runs one turn at a time (task 26), but the pair count is shared by two Orchestrators whose turns run at once, so two concurrent sends at 19 both pass and the pair reaches 21.
**How to fix:** Give `AppendMessage` a `maxPair int` it checks inside its transaction (with `SELECT … FOR UPDATE` on a per-pair row, or an advisory lock on the ordered pair), returning `rate_limited`; or drop the claim. Add `TestConcurrentSendsRespectThePairCap` on `memstore` (mutant: check outside the write).

### S03-M15 (Minor): the migration number is stale against task 10
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-28.md:639-642
**Problem:** "`0003_threads.sql` if 26's is `0002`". Task 10 absorbed task 26's tables into its own migration and records that `0002` is task 19's (task-10.md:590), and it leaves the thread tables to this task (task-10.md:586-587). Following this line, the thread migration takes a number that task 19 or a later task may also take.
**How to fix:** Name the migration after task 10's and task 19's actual numbers (read at build time), and say task 28 owns `fleet.threads`, `fleet.messages` and `fleet.message_reads`, as task 10 says.

## Task 29: resolutions and DECIDED WITHOUT YOU

### S03-I11 (Important): the accept and its resolution commit in separate transactions, so an accept can be recorded with no effect, and the decide path marks a proposal task 28 has already removed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-29.md:32-36, 191-209, 375
**Problem:** `Resolve` runs "from task 28's `Service.Send` step 9, after the accept commits". Task 28's step 7 has already removed the named proposal from the live set and chosen not to wake the proposer "for an `accept` that task 29 resolves in the same call" (task-28.md:559-562), and step 8 has sent `OpMessageAppend` to the Hub. Three results follow. (1) A crash, or any error, between that commit and `Resolve` leaves an accept in the Hub and in Fleet with nothing applied, no live proposal to retry, and no wake for the proposer. The thread then sits until the deadline escalation, and no DECIDED WITHOUT YOU is written, which is the silence property 4 exists to prevent. (2) `TestYieldStaleAtAccept` says "the accept returns `resolution_stale`", but the accept is already stored and forwarded, so the Hub's thread shows an `accept` that did not happen. (3) Step 2's decide path "mark[s] the proposal `awaiting-human` in the thread's live set", but task 28 has already deleted it from `Live`, so `ApplyApproved` finds nothing to apply. Step 6 also enqueues `thread.resolved` through `turns.Queue.Enqueue`, which is its own store write (task-26.md:431-449), so "one transaction from step 3 to step 6" does not hold as specified.
**How to fix:** For `accept`, call `Resolver.CheckAccept` (step 3's re-check plus `Classify`) **before** task 28's `AppendMessage`, refusing a stale accept without storing it; then make the apply (step 4-6) part of the same `AppendMessage` transaction, with the `thread.resolved` event passed as its `wake`-style argument, as task 28 already does for `peer.message`. Keep a decided-for-human proposal in `Live` with an `awaiting` flag rather than removing it. Tests `TestStaleAcceptIsNotStored` (mutant: store then check), `TestCrashBetweenAcceptAndApplyLeavesNeither` (injected failure; mutant: two transactions), `TestAwaitingProposalSurvivesTheAccept` (mutant: task 28's removal applied to a decide).

### S03-M16 (Minor): `Outcome` is not complete in its fence, and the test fixture has workers holding `intent` leases
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-29.md:177-188, 290-292, 334-335, 366-367
**Problem:** `decidedQuestion` and `DecidedWithoutYou` read `o.YieldedGlobs`, `o.FirstName` and `o.Proposer`, which the fenced `Outcome` does not declare; a prose line adds them. The house style asks for complete source for every type. The fixture gives worker `wA` an intent `ls-A2` and worker `wB` an intent `ls-B1`, which task 23 refuses and task 24 makes the Orchestrator's (task-24.md:19-24).
**How to fix:** Put the three fields in the fenced `Outcome`, and make `oA`/`oB` the intent holders in the fixture (with `wA`'s hold unchanged).

## Task 30: escalations

### S03-I12 (Important): the safe default pauses "the contested worker", but the conflicts TM1-alpha raises are between Orchestrators
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-30.md:28-31, 439-448, 492
**Problem:** The contested-worker function takes "a conflict with paths `P` between workers `a` and `b`" and calls `Pauser.PauseForDefault(contested, es)`, a `pause` directive that route 67 accepts only with `fromAgentId` the worker's parent (line 102-105). Plan-time intents are held by **Orchestrators** (task-24.md:19-24), and task 25 raises the conflict with `agentA: ag-a0`, an Orchestrator (task-25.md:212). That is exactly the TM1-alpha scripted conflict. At an unanswered deadline, `Contested` then names an Orchestrator: it has no parent, route 67 refuses the directive, and nothing pauses; or, if the check is loose, Fleet "pauses" an Orchestrator, which is not a worker and not the design's safe default (§6.5, 05-design.md:746-747). The test uses a cast where workers hold the leases (task 29's fixture), so it cannot see this.
**How to fix:** Map each conflict side to workers first: an Orchestrator side resolves to the live workers on the task whose intent overlaps (task 24's `Task` on the lease). If that side has no worker yet, the safe default is "hold the task": refuse `worker spawn` on that task until the decide is answered (a Fleet row checked by task 24's `spawn`), with no directive. Add `TestExpiredPlanTimeConflictHoldsTheTask` (conflict between `oA` and `oB` intents, no workers; mutant: a pause directive to an Orchestrator) and `TestExpiredConflictPausesTheTasksWorker` (intent held by `oB`, worker `wB` on that task; mutant: pausing `oB`).

### S03-I13 (Important): unknown inputs classify as `act`, so two decide rows silently become act for the whole of TM1-alpha
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-30.md:225-250, 541, 551-553
**Problem:** `KeyOf` returns `yield-unedited` whenever `EditedByProposer` is empty and `sequence-short` whenever `ForecastWait` is 0 ("unknown ... counts as short"). In TM1-alpha both inputs are always unknown: there is no forecast, and there are no observed edits before task 35. Workers still edit files in TM1-alpha; Fleet simply does not observe them. So "yield globs my worker **has** edited → decide" (05-design.md:726) and "sequence ... otherwise decide" (:730) never fire. Task 29 claims TM1-alpha "cannot reach" an edited yield (task-29.md:430-431), which is not true: any worker that has started can have edited. The part's own rule elsewhere is fail closed ("a rule source that cannot be read never loosens; it asks", `decideOnError`). Gap 2 frames only the sequence half, and as an owner call.
**How to fix:** Make the unknown case explicit and closed: `EditedByProposer` is "known empty" only when the proposer's side has no worker that has run on the task (task 24's rows), otherwise "unknown" → `yield-edited` (decide); `ForecastWait` unknown → `sequence-long` (decide). The scripted conflict of task 33 can still act, because at plan time no worker exists, so a `yield` is known unedited. Add `Unknown` states to `Action` and tests `TestYieldWithAStartedWorkerAsks` (mutant: empty treated as unedited) and `TestSequenceWithUnknownWaitAsks` (mutant: unknown as short). Keep the owner question, but with the safe reading as the default.

### S03-M17 (Minor): the scope's named test for splits cannot be written here, and the owner columns disagree with task 10
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-30.md:197-198, 246-247, 455-456
**Problem:** The skeleton's scope (skeleton:750-751) asks for "accepting a split that moves edited paths becomes a decide"; `split` is `kind_not_enabled` until task 62, and no row here tests it, and the part does not say it is deferred. `fleet.open_escalations` stores `owner_tid`/`owner_oid`, while task 10 records owners as one `u-…` (task-10.md:588-589), the same split as S03-I10.
**How to fix:** List the split test as task 62's in "does not claim" and in the skeleton; store `owner` as task 10 does.

## Section summary

**Counts:** 2 Critical (S03-C1, S03-C2), 13 Important (S03-I1 to S03-I13), 17 Minor (S03-M1 to S03-M17).

**Completeness:** all ten parts (task-21.md to task-30.md) are present and end in their "does not claim" or "gaps" section; none is cut off. Every part gives files, fenced interfaces, a test table with named mutants, red-first commands, acceptance and non-claims. Sizes are 2.5 to 3 h as stated. Task 24 would grow by about an hour with S03-I7, and task 21 with S03-C1 and S03-I3.

**Citations checked against the tree and sibling worktrees (correct unless listed as a finding):** `backend.go:337-345` (the seven methods); `attach.go:182`; `sessions.go:586-593` (`sessionJSON`); `sessions.go:1143` (`watchSessionReady`); `ids.go:30,46` and `hub/…/ids.go:61`; `hub/…/hubclient.go:19-21`; `hub/…/hubapi/types.go:140` (`Project`, no mode); phase 8 `task-22.md:11-20`; phase 6 `task-14.md:31, 235`; task-01.md route and class tables; task-02.md `CheckSet`; task-03.md `ClassRoutes`; task-10.md DDL and gaps; task-20.md S20-1; task-28.md `ThreadFor`/`OpenThreads`/index. Not checked: task 07's route 62/64/69 bodies at the cited lines, and task 16's decision 10 text (UNVERIFIED here).

**Cross-cutting pattern:** three findings (S03-C1, S03-I6, S03-I12) come from one unresolved choice: who holds which delegation, and who holds a plan-time intent. Settling "the desktop creates a per-project orchestrator delegation and a per-project worker spawn delegation on the first team launch" and "a conflict side is a task, resolved to its workers" fixes all three and task 33's entry criterion E2.

Section verdict: Needs fixes


## Addendum: re-run of 2026-09-25 20:34

The review above was finished at 13:44 and stands. Since then only task 25 in this range changed (commit b889209); tasks 21-24 and 26-30 are byte-unchanged since 11:33. This addendum re-checks task 25 and the dependency parts the findings above cite that changed after 13:44. The verdict line at the end of this addendum supersedes the one above.

### Task 25 after b889209

Status of the earlier findings: **S03-I8 open** (task-25.md:133-137 unchanged). **S03-I9 open**, both halves: the pair check is still agent-keyed, and b889209 now says so outright (task-25.md:153, "whose `HubState` record's agent pair is `{a, b}`"). For the replica half, see S03-I14. **S03-M8 partly fixed**: the `OpenThreads` citation at task-25.md:152 now points at task-28.md:397, which is right. Still wrong: task-25.md:57 (`ThreadFor` is at task-28.md:396, not `:365`), and task-25.md:246 (the logging stub is at task-28.md:685, not `:653`). Task-25.md:29 cites `task-28.md:89-91` for `OpenForConflict`, whose signature is at task-28.md:91-93. The null basis is still only "listed in the return" (task-25.md:161-162).

### S03-I14 (Important): b889209 swaps an honest UNVERIFIED for a false citation, so the two-replica race still gives two threads, or one conflict that is never closed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-25.md:167-175
**Problem:** The new text says task 28 already returns the existing thread "and more broadly: one open thread per Orchestrator pair per subject" (citing task-28.md:53-55). But per subject is narrower than per pair, not broader. `OpenForConflict` takes the conflict id as its subject (task-28.md:91-93), and the unique index is `(org_id, participant_a, participant_b, subject_kind, subject_ref) WHERE state = 'open'` (task-28.md:654-655). So the race's `cf-1` and `cf-2` are two subjects and get two threads. `TestTwoRaisesOneThread` (task-25.md:224) passes only against the fake. The fallback at line 174 does not hold either: task 29 step 5 closes the one conflict its thread carries (task-29.md:202-204), not "every open conflict of the pair". Whichever way the race goes, one of two things happens. Either two threads each wake both Orchestrators and spend the §6.6 pair budget twice. Or, if a later fix makes the second open join the first thread, `cf-2` stays `raised` for good, and step (4)'s one-conflict-per-open-pair check then blocks every later real overlap between those two agents from being raised.
**How to fix:** Close the race in one place, at raise time. Before `CreateConflict` (step 5), insert a pair reservation into a `fleet.conflict_pairs` row keyed on (org, project, ordered Orchestrator pair) with `ON CONFLICT DO NOTHING`. This is a single-statement transaction, so no transaction is held across the Hub call. The replica that loses finds the winner's row and merges into its conflict. The row is deleted when the conflict closes, and on a Hub error. Put the DDL in full in the part. Add `TestReplicaRaceOneConflict`, with two forecasters over one real Postgres store (mutant: a reservation keyed on the agent pair or on the cf id), and restore UNVERIFIED until it is in. If the plan would rather fix this in task 28, give conflict subjects their own index `(org_id, participant_a, participant_b) WHERE state='open' AND subject_kind='conflict'`, give a thread a set of conflict ids, and make task 29 step 5 close them all, each with its own test. Either way, delete "more broadly" and the line 174 claim.

### Dependency parts that changed after 13:44 (tasks 06, 07, 08, 17, 19, 20)

- **S03-C1 still open.** Route 46 is still desktop-only (task-06.md:33, :93; the lines S03-C1 cited, `:88` and `:104`, have moved to these). Task 20 still records Gap S20-1 as open and blocking task 33 (task-20.md:529-533), and task 21 is unchanged.
- **S03-I1 is worse than it was.** Task 08 (0ec470c) and task 06 (775c03a) now both use the typed `hubapi.DelegationClassOrchestrator` / `DelegationClassWorker` and `[]hubapi.RouteName` from task 03 (task-03.md:100-101, :362-388, :488-492; task-08's `CreateDelegationRequest` call). Task 21's `routesFor` (`[]string`, `hubapi.OrchestratorRoutes()`, `model.ClassOrchestrator`, task-21.md:309-320) is now the only caller of the shape out of step. The fix is as in S03-I1.
- **S03-I2 and S03-I6 still open.** Task 17's class rule is unchanged in substance (task-17.md:59-64): an orchestrator delegation still creates the workers it spawns. Route 49 (revoke) is still desktop-only (task-06.md:33), so S03-I2's fix, a revoke from the desktop's `Destroy`, is still the right one.

### S03-M18 (Minor): every citation from tasks 24-30 into tasks 06 and 07 is now off by the growth of those parts
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-24.md:57, 141; task-25.md:21, 55, 141, 157, 219; task-26.md:581; task-28.md:123; task-29.md:55, 204; task-30.md:95, 326, 435
**Problem:** 775c03a added 5 lines to task 06 above route 50, and 773a94f added 4 lines to task 07 above its route table. So all 14 citations now land on the wrong text. Two examples: `task-06.md:121` is now route 49 (it was route 50), and `task-07.md:123` is now route 63's refusal rule (it was route 64). I checked each one against the pre-edit blob (`git show 775c03a~1`, `773a94f~1`) and the current file.
**How to fix:** Task 06 cites move by +5 and task 07 cites by +4. The new values are: task-24 `:121-147`→`:126-152` and `:144-146`→`:149-151`. Task-25 `:210-215`→`:215-220`, `:198-216`→`:203-221`, `:198-200`→`:203-205` (twice), and `:205`→`:210`. Task-29 `task-06.md:210-215`→`:215-220` and `task-07.md:123-125`→`:127-129`. Task-26 `task-07.md:175-180`→`:179-184`. Task-28 `:99-111`→`:103-115`. Task-30 `:127-142`→`:131-146`, `:140-142`→`:144-146` and `:148-150`→`:152-154`. Check each range end by eye after the edit. The fixer who grows a part should also re-point the cites into it; fb47232 already did that for tasks 17, 19 and 20.

### Addendum summary

- **New:** S03-I14 (Important, task 25) and S03-M18 (Minor, tasks 24-30).
- **Resolved since 13:44:** none in full. S03-M8 is part-fixed (one citation of three).
- **Still open:** the rest, unchanged, because tasks 21-24 and 26-30 have not been edited. That is S03-C1 and S03-C2, S03-I1 to S03-I13, and S03-M1 to S03-M17.
- **Open totals:** 2 Critical, 14 Important, 18 Minor.
- **Not re-checked this run:** the sibling-worktree citations in tasks 22, 26 and 27 (phase 3, 6 and 8 plans). They were checked at 13:44, and the dependency parts named above are the only ones I re-read.

Section verdict: Needs fixes
# Phase T plan review — section 04 (tasks 31 to 40)

Reviewer: adversarial plan reviewer, read-only except this file.
Method: each part read in line ranges against its "## Task scopes" entry in the skeleton, the parts it depends on, and the real trees it cites.

## Task 31 — Directives and the taint rule

Coverage of the scope is complete (five typed kinds, `free` held as a decide written by `direct`, Hub record before delivery, piggyback as the only text path, the three scope tests). Citations checked and real: `task-07.md:205-209` (three kinds), `:232-237`, `:290-294`, `:158-172`; `task-01.md:431-440`; `task-03.md:341-343`; `task-22.md:155-158`, `:267-276`; `task-18.md:62-67`; `task-24.md:130-135`; `task-19.md:87-93` (`Caller`); `control-plane/go.mod:3` and `sandboxd/go.mod:3` are `go 1.23`, so the `*time.Time` comment holds; design `05-design.md:807-815` matches decision (a).

### S04-I1 (Important): a Fleet-turn `resume` can undo a decide's safe-default pause
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:254-256
**Problem:** `fleetTurnPasses` lets `resume` through in any Fleet-started turn. Task 30's `SweepDeadlines` (`task-30.md:445-449`) pauses the contested worker through `PauseForDefault` when a decide expires. That pause is the human's safe default. A tainted Orchestrator, in the next peer-triggered turn, can issue `direct wB resume`, and the gate passes it. Peer text then undoes a human safety outcome with no human in the loop. This is the trusted-deputy path that task 07's author named in its test row "resume treated as safe: it restarts a paused worker" (`task-07.md:292`). Task 31 widens the Hub set to five (G31-1) but does not close this hole. No test covers it.
**How to fix:** In `Gate.Judge`, a `resume` in a Fleet turn passes only when the worker's in-force pause was itself issued in a Fleet turn without `approvedBy`. A pause from `PauseForDefault` (it carries `approvedBy: es`) or from a `human`-origin turn can be lifted only by a human-origin turn or a new decide (row `scope-change`, or a new `resume-after-default` row in task 30's table). `InForce` already knows the latest pause, so read its origin and `approvedBy` from there. Add `TestFleetTurnResumeCannotLiftADefaultPause` (kills: `resume` in `fleetTurnPasses` with no pause-origin check). G31-1's note to task 07 should say the Hub may widen to `Typed()` only because Fleet makes this check.

### S04-I2 (Important): `Release` replays as a reconstructed caller without re-checking liveness or delegation
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:387-394, 421-431
**Problem:** `Release` "re-invokes its verb through the dispatcher's handler table as the same caller". That replay runs outside any authenticated control stream, possibly 30 minutes or more after the hold. `control.Caller` is `{Org, Project, Agent, Owner, Class}` (`task-19.md:87-93`), but `fleet.held_actions` stores no `project` and no `class`, so the caller cannot be rebuilt from the row as written. The part also never says whether the replay checks that the Orchestrator is still live, that its session was not revoked (task 11), that the owner is still a project member, and that the `orchestrator`-class delegation has not lapsed. The same gap applies to the time between hold and release. For a held `worker spawn`, Fleet would launch a worker in the owner's name on stale authority. The Hub's refusal of the outbox item covers only directives, not a spawn.
**How to fix:** `Release` rebuilds `Caller` from the live `fleet.agents` row for `held.orchestrator` (live, `Class == orchestrator`, same `owner`), then runs task 19's own delegation and membership check before invoking the handler. Any failure becomes the `escalation.answered` refusal event already described. Add `project` to `held_actions`, or state that it is derived from the agent row. Add `TestReleaseAfterRevocationIssuesNothing` (kills: a replay built from the held row alone).

### S04-M1 (Minor): the approval context passes any gate call, not the held act
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:306-308
**Problem:** `approvalFrom(ctx)` returns an `es-…`, and every `Judge` under that context passes whatever the `Act`. A replayed verb that makes more than one gated act (task 24's `worker spawn` if it also creates a task, or a later verb change) gets every act approved by a decide that showed only one.
**How to fix:** Store `{es, kind, worker, task}` in the context value, and pass only when `a.Kind` and `a.Worker` match the held row. Add one test row where a mismatched kind under approval is held again.

### S04-M2 (Minor): a late "Send it" is not checked against the deadline
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:394, 436-438
**Problem:** Task 30 lets a later answer still act (`task-30.md:449`). Task 31 deletes the held row on expiry, but the janitor's backstop is "an hour past `deadline`". So if the expiry sweep misses, an option-1 answer after the deadline releases the act. The safe default is "Do not send it", so a late yes should not send.
**How to fix:** `Release` refuses when `now > held.deadline`, whatever the row state. Add a test with the clock past the deadline.

### S04-I3 (Important): task 31 is well over the 1 to 3 hour bound
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:85-117, 468-489
**Problem:** One task builds a new package with three files, a new verb, a migration with two tables and an `ALTER`, the store in both backends plus the suite, and the wire in two modules with five fixtures. It also edits seams in tasks 18, 19, 22, 24, 26 and 30, and carries 20 tests. That is one to two days of work, not 1 to 3 hours, and a partial stop leaves the dispatch drain half-wired.
**How to fix:** Split the task in two. 31a covers the gate, `held.go`, `verbs/direct.go`, the task 24 and 30 seams and the taint tests. 31b covers the queue migration and store, the wire and fixtures, the task 19 drain and ack, the task 18 envelope and the delivery tests. 31b depends on 31a's `dv-…` issue path.

### S04-M3 (Minor): task 30's interim `fleet.default_pauses` is not retired
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-31.md:506-507
**Problem:** Task 30 wires a stub `Pauser` that records into `fleet.default_pauses` (`task-30.md:104-106`). Task 31 replaces the stub but does not say what happens to that table or its rows. A pause recorded before the upgrade would never be delivered.
**How to fix:** In Step 6, name the migration step: move rows from `default_pauses` into `directive_queue` through `PauseForDefault`, or drop the table if task 30 never shipped alone.

## Task 32 — Phase 8 delta: Fleet's chart and the private gateway path

The scope is covered: two replicas, a PDB, workload identity, its own schema on the Hub's server under its own Entra roles, one sandbox-to-gatekeeper allow, no public gateway route, a `netpolcover` render check, and no apply. Decision (a) correctly finds that the Orchestrator needs no egress to Fleet, because control is inbound through the broker (`task-17.md:25-27`, `task-19.md:494`). Citations checked and real: phase 8 `task-11.md:150-164` (the `egress-internet` wording and the Cilium UNVERIFIED), phase 6 `2026-09-25-phase-6-gateway.md:203-204` and `:262` (ports 4000, 4100 and 4001, and keysmith admitting the broker and the gatekeeper only), `:308` (P8-3); team `task-12.md:819` and `:1214`, `task-20.md:489-491`, `task-17.md:386-388`, `task-10.md:375-378`. The sibling chart files do not exist yet and are correctly marked UNVERIFIED until merge.

### S04-I4 (Important): the Orchestrator's "no internet" fails open on a missing label
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-32.md:44-48, 306-311, 419
**Problem:** `egress-internet` keeps internet for every pod matched by `panacea-app/class NotIn [orchestrator]`. A Kubernetes `NotIn` also matches pods that have no `panacea-app/class` key at all, and the part says so ("Pods with no class label … still match"). So one missed label turns the exfiltration fence off without anyone noticing. That label is task 15's, not yet named in its part (G32-2). A pod-template change or a warm-pool pod built before the label existed is enough. The Orchestrator is the one class that reads peer text on every turn (§7.2 item 1), so this is the control that stops a persuaded Orchestrator from posting a teammate's words out. The test `orchestrators have no internet` checks only a pod that carries the label, so it cannot catch this.
**How to fix:** Make the internet allow an allow-list. Select `panacea-app/class In [worker, machine, balloon]` (whatever task 15 names), and make task 15 label every pod it creates, balloons included. A pod without the label then gets DNS and the gateway only, which is the safe side. Add the test `platform / an unlabelled sandbox pod has no internet` (kills: `NotIn`, or `DoesNotExist` treated as a worker). Also add a task 15 unit test that `Spec.Class == orchestrator` always renders the label.

### S04-I5 (Important): task 32 is over its 3-hour estimate and spans too many owners
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-32.md:3, 83-106
**Problem:** The part is timed at 3 h. It builds a new chart of ten templates with a schema and helm-unittest suites. It also modifies P8-11's platform chart, P6-19's gateway chart, P8-13's edge chart, `netpolcover`, two Terraform stacks (workloads and identity, with a new app registration and a cross-app role), a SQL template and `render-sql.sh`. Every one of those sibling files is UNVERIFIED until merge, and helm is not on the build laptop (`:471`). Reconciling module and output names (step 5) alone is open-ended. As written, one agent cannot finish this in 1 to 3 hours, and a partial stop leaves the platform selector changed with no Fleet chart, or the reverse.
**How to fix:** Split it in two. 32a is the Fleet chart, its tests, Terraform identities, the identity registration and the SQL. 32b is the platform, gateway and edge policy deltas plus the two `netpolcover` cases. 32b depends on 32a only for the `panacea-fleet` namespace name and the pod labels.

### S04-M4 (Minor): Fleet's egress to 0.0.0.0/0 on 443 is broader than its need
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-32.md:283-290
**Problem:** Fleet holds the Hub delegation, the broker's `Sandboxes.Fleet` role and the control key. It is the trusted deputy for every team. Yet its policy allows HTTPS to any public address, for Entra and a hairpin case that decision (f) says is unknown. A compromised Fleet can post team state anywhere on 443. The part does not name this as accepted risk.
**How to fix:** Either restrict it to Entra's and the edge's addresses through the firewall's FQDN allowlist (P8-3's mechanism), or add a "does not claim" line naming the open 443 as accepted until task 33 records which path the Hub and broker calls take. Then narrow it in task 47.

### S04-M5 (Minor): the Orchestrator's non-essential CLI traffic is left to "should"
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-32.md:483-485
**Problem:** With no internet, the CLI's telemetry and update checks fail on every headless turn. The part says the class's managed file "should turn it off" but names no key and no test. So task 13 has nothing to build against, and task 33 has nothing precise to observe.
**How to fix:** Name the setting (the `env` key the managed file sets for non-essential traffic, marked UNVERIFIED at the pinned CLI version) as a requirement of task 13. Add it to task 33's observation list.

## Task 33 — TM1-α on the real cluster

The scope is covered as must-hold and observed-only checks fixed before the run: the conflict before spawn (M2), `yield`/`sequence` (M3), inform against decide (M4), the pod kill (M6), `/status` and the digest (M7), the PTY round trip (M9), the hook dialog (O1), the empty folder (O2), and a first G9 pass (O8). The gate is stated, and a FAIL means "re-plan TM1" (skeleton `:346`). Citations checked and real: `task-27.md:638`, `:651-653`; `task-21.md:84`, `:449-451`; `task-07.md:144-147`, `:153-155`, `:182-184`; `task-20.md:529-533`.

### S04-I6 (Important): Run A is not deterministic, because the live runners race the script
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:36-48, 253-269
**Problem:** Decision (a) says Run A's verbs are "the same every time" because the operator issues them through `kubectl exec`. Both Orchestrator runners are running throughout, though. Task 25's conflict and task 28's thread events queue Fleet-started turns (task 26), and in those turns the real models call `peer propose`, `peer accept` and `direct` themselves. So Bob's model can accept or counter-propose before the script does, which breaks M3's "the lease diff is exactly …", or issue its own directives, which muddies M5 and M10. Decision (b) is also not certain. `CurrentTurn` is keyed on the Orchestrator's agent id (`task-31.md:282`), not the process, so an exec'd call made while the runner holds a claimed turn inherits that turn's origin and is counted in its `issued` record. Nothing in the part parks the runners for Run A.
**How to fix:** Before `20-plan.sh`, hold both Orchestrators' turn queues so no Fleet turn is claimed during Run A. Add an entry condition (E8) that `team` shows both Orchestrators `parked`, with no claimed turn, and re-check it after each script. Use task 26's scheduler pause, or set the budget to zero through task 37's owner setting if that lands first. Name the mechanism as a requirement of task 26 if none exists. Resume the queues before `40-podkill.sh`, which needs real turns. Record in M3 that the queues were held.

### S04-I7 (Important): O2 tests the worker, but the design's question is about the Orchestrator
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:123, 285-286
**Problem:** Design §4.1 (`05-design.md:377-380`) asks whether the **Orchestrator's** empty working folder keeps project settings and project memory (`CLAUDE.md`) out. That is the I7 defence against a teammate's landed `CLAUDE.md` line (`05-design.md:1419`). O2's procedure commits a `.claude/settings.json` to the test repo and looks at the **worker**. The worker's working directory is the repo, so the file loads by design, and the observation says nothing about the Orchestrator. Also, `CLAUDE.md` discovery walks up parent directories, which an "empty folder" does not stop. That case is untested.
**How to fix:** Run O2 in the Orchestrator pod. Put a distinctive `CLAUDE.md` and a `.claude/settings.json` (with an `env` marker) in the runner's working folder's parent and in the pod's home. Then check, through the operator's interactive `claude` in that folder, that `/status` and `/memory` show neither. Keep a worker-side row, but as a must-hold check of task 13's I3 and I4: a committed project hook and a project `permissions.allow` rule are **not** in force in the worker (`task-13.md:76-77`; that enforcement is UNVERIFIED at `task-13.md:767`). This is the OD-23 "nothing else in force" property that M7 claims.

### S04-I8 (Important): E3's desktop calls have no owning task, so TM1-α ends BLOCKED by construction
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:77, 302-323, 413-414
**Problem:** G33-1 shows that no task before 52 gives the desktop the answer, human-message, list-escalations and list-directives relays. It hands them to "task 21 … or an early slice of task 52" without assigning either. So tasks 01 to 32 as written cannot reach a PASS, and the slice's gate cannot open. The same holds for G33-2, task 18's hook trace switch, which O3 depends on.
**How to fix:** Make the four relays and the hook trace switch planned work. Either amend task 21's part (it already holds `a.relayHub`) with the four routes, their tests and the wire shapes, or add a TM1-α task between 32 and 33, and list it in the skeleton's task list and file map. Also give `GET /api/team/directives`'s response shape in full: it is the one wire shape of the four left out of the block at `:311-319`.

### S04-M6 (Minor): no script computes the verdict, although the part says the record cannot be edited into a PASS
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:390, 408-409
**Problem:** Step 5 fills the template by hand from `out/*.jsonl`, and decision (e)'s rule (a `not-observed` must-hold check is a FAIL) is applied by a person. The "does not claim" line says the gate cannot be passed by editing the record, but nothing enforces it.
**How to fix:** Add `90-verdict.sh`, which reads `out/*.jsonl`, applies decision (e) and prints the one word plus the table rows. Add a `lib_test.sh` case where a missing M-row gives `FAIL` (kills: a skip treated as a pass).

### S04-M7 (Minor): M4's cross-owner read uses a route the relays do not have
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-33.md:110, 264
**Problem:** M4 expects "Alice's `GET` of Bob's `es-…` answers not-found or forbidden". G33-1 names only a list route, and task 07's route 66 answers a non-owner with `200` and an empty list (`task-07.md:153-155`), not a 403 or 404.
**How to fix:** Restate M4 as "Bob's `es-…` is absent from Alice's list, and Alice's `POST …/answer` on it is 403 `forbidden`" (route 65, `task-07.md:144-147`). That is also the stronger cross-owner check.

## Task 34 — Phase 3 delta: `machined`'s activity sweep

The scope is covered: the sweep runs outside the agent's VM every 5 to 10 s, uses porcelain v2 and numstat with `-z`, reads hunks and the last commit, and applies hygiene with the 200-character cap and a dropped count. There is no read set, and the two skeleton tests (fsmonitor, bidi) are named. Decision (c), pull rather than push, is sound, and the broker relay is scoped by delegation project and machine owner (`:242-247`). Citations checked: phase 3 `task-02.md:466-480` (the H1 table) and `task-07.md:19-26` (file list, plan only) are real. The sibling code does not exist, and the part correctly marks it UNVERIFIED until merge.

### S04-C1 (Critical): `git status` recursing into a submodule can run agent code inside `machined`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-34.md:209-216, 226-229
**Problem:** The part's first concern is that no config-driven command the agent wrote may run in the machine pod, which holds the whole disk and the user's host token. Its calls 2, 3 and 5 (`status`, `diff --numstat`, `diff -U0`) run without `--ignore-submodules`. For a populated submodule, `git status` checks whether the submodule is dirty by starting a child `git status` inside it. That child reads `.git/modules/<name>/config`, which phase 3's H3 check does not read; phase 3's own table says so (`phase-3 …/task-02.md:478`). H1's `-c` overrides reach the child through `GIT_CONFIG_PARAMETERS`, so `core.fsmonitor` and `core.hooksPath` stay neutralised. A **filter driver** in the submodule's config, however, is *refused* by H3 and not overridden, and here H3 never sees it. So `filter.x.clean=<program>` in `.git/modules/x/config`, plus `* filter=x` in the submodule's `.gitattributes`, would run the agent's program when `status` re-reads a stat-dirty file. That happens every 5 s, in the pod that holds every lane and the host token. `-c submodule.recurse=false` does not stop this, because it governs recursion in commands, not the status dirty check. (This is how git's submodule status works from training knowledge; it is UNVERIFIED at the pinned git, and the test below proves it either way.) The agent can build this state in its own clone, because phase 3 lets it write `.git/` (R4.2).
**How to fix:** Add `--ignore-submodules=all` to calls 2, 3 and 5, and `-c diff.ignoreSubmodules=all` to `ReadFlags`. Report a gitlink as one `DirtyPath` with status `modified` when its recorded sha differs, from the superproject's own index only. Add `TestSubmoduleFilterNeverRuns` (a populated submodule whose `.git/modules/x/config` sets `filter.x.clean=<marker>`, its `.gitattributes` naming it, and a stat-dirty file inside it: no marker is written; kills: `--ignore-submodules` dropped from any of the three calls). Raise it with phase 3 as a gap in §HGH H1 for every host-side `status` and `diff`, not just this sweep.

### S04-I9 (Important): ST.6 carries no committed line ranges, although task 36's scope needs them
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-34.md:150-164
**Problem:** The skeleton's task 36 raises tier H on "overlapping dirty **or committed** line ranges on the same base blob" (skeleton `:815-817`). ST.6's `Hunk` holds only working-tree-against-`HEAD` ranges on `HEAD`'s blob. `LastCommit` holds paths only, and for the last commit alone. A worker that commits its edit and keeps working has no ranges for the committed part. Once either side commits, the two workers' `HEAD` blobs differ, so tier H compares ranges on different bases and raises nothing. The committed half of tier H has no input.
**How to fix:** Either add a `Committed []Hunk` to ST.6: ranges from `diff -U0 --no-ext-diff --no-textconv --ignore-submodules=all <merge-base with the integration ref> HEAD -- <path>` on the merge-base blob, bounded by `MaxHunkPaths`. Or state in this part and in task 36 that tier H in TM1 covers dirty ranges only, and change the skeleton's scope to match. This task is the one that defines ST.6, so the choice belongs here.

### S04-M8 (Minor): the bound "oldest-changed paths last" has no source for "changed"
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-34.md:104-105, 133
**Problem:** The comment says a sample over a bound is cut "oldest-changed paths last". `Dirty` is "sorted by path", though, and no call reads mtimes, so the cut order cannot be what the comment says.
**How to fix:** Say that the cut keeps the first N paths in path order, matching `:221`, or add the mtime read and a test.

## Task 35 — Activity, presence and holds

The scope is covered: a 45 s in-memory activity table, a hold on first observed edit whether asked or not, bulk above 25 files a minute with no right of way reported to the Orchestrator, per-path 30-minute expiry, the four end paths, admin release (Fleet's half, with G35-3 naming task 44), a rebuild within one sweep, and both skeleton tests. Citations checked and real: `task-23.md:104-107` (bulk refused on `Request`), `:156-164` (the transition table), `task-12.md:25-32` (two replicas, singletons).

### S04-I10 (Important): a commit's agent-set date drives hold expiry, so an agent can pin a hold
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-35.md:34-37, 290
**Problem:** Decision (c) counts every path of a new `LastCommit` as edited "at that commit's time", and `TestCommitCountsAsEdit` asserts exactly that. `LastCommit.At` is git's committer date (`task-34.md:163`, `%cI`), which the agent sets freely (`GIT_COMMITTER_DATE`, `--date`). With `ExpiresAt = max(created, newest touch) + 30 min` (decision b), one commit dated in 2030 gives a hold that never lapses. It keeps the right of way over a teammate's worker until the task ends. This breaks §6.3's "a hold lapses 30 minutes after the last edit to that path". A worker persuaded by peer text, or a careless one, can squat a path. A past date expires a hold at once instead.
**How to fix:** Stamp a commit edit with the sample's `At` (machined's clock), bounded to `(previous sample At, this sample At]`, and never read `LastCommit.At` for expiry. Change `TestCommitCountsAsEdit` to assert that the edit is at the sample's time. Add `TestFutureCommitDateDoesNotExtendAHold` (kills: `LastCommit.At` used as the touch time).

### S04-I11 (Important): presence is pushed to one replica's memory, but two replicas answer reads
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-35.md:252-275
**Problem:** Fleet runs two replicas behind one Service (task 32, `task-12.md:25`). Activity works on both because every replica polls (`:239-245`). Presence is pushed by `POST /v1/presence`, which lands on one replica's in-memory `Presence`. A `team` call or task 53's strip served by the other replica shows that person offline. Which replica answers is random, so presence flickers every 30 s. No test uses two replicas.
**How to fix:** Keep presence out of the durable record, as global constraint 1 requires, but share it across replicas. Either write each beat to a Postgres `UNLOGGED` table with its TTL, which counts as Fleet working state (a fact about now), or broadcast it over `LISTEN/NOTIFY`, which the feed singleton's connection already holds. Add `TestPresenceVisibleOnTheOtherReplica` (two `Presence` instances over one store; kills: presence kept per process).

### S04-I12 (Important): a formatter run writes and releases dozens of holds before becoming bulk
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-35.md:45-51, 197-211, 285
**Problem:** `Observe` writes a hold per path (two Hub events each, and a `contested` event where it overlaps a teammate), and only then does `BulkDue` and `Bulk` collapse them and release them. Forty files in one sample leave about 120 Hub lease events. Each overlap with a teammate's lease also reaches that teammate's Orchestrator as a lease event, which is a Fleet turn and budget spend (task 37), for leases that live for seconds. That is what bulk exists to prevent (§6.3: "one flagged `bulk` lease"). The skeleton test checks only the end state.
**How to fix:** In one fold, count the window first. If this fold's edits take the worker over `BulkOver`, write or extend the `bulk` lease and write no per-path holds for those edits. Release the earlier holds from the window in the same `Tx`. Extend `TestFormatterRunIsOneBulkLease` with a single 40-file sample: zero `hold` leases ever written, one `bulk`, and no `lease.contested` event queued to a teammate (kills: holds written before the bulk check).

### S04-M9 (Minor): one `since` per machine loses samples when a machine hosts two team projects
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-35.md:239-242
**Problem:** The poller polls per (machine, team project), because the relay needs `project=` (`task-34.md:237-239`), but it keeps "the last `Next` it saw for that machine". `Seq` is per machine across all projects, so polling project B with project A's `Next` skips B's samples in between.
**How to fix:** Key `since` on (machine, project), and add a test with two projects on one fake machine.

### S04-M10 (Minor): `Presence.Online` returns every open project, whoever is reading
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-35.md:265-266
**Problem:** `Online(owner)` returns all the projects the person's desktop named. A consumer rendering project X's strip could show that the teammate also has project Y open, and the reader may not be a member of Y.
**How to fix:** Change the signature to `Online(owner, project) bool` so the read side can only answer about the project being viewed. Add one test.


## Task 36 — Tier-H forecasts and co-edit down-weighting

The scope is covered in form: same-blob overlap raises H, different blobs raise nothing, P collapses in TM1 and the part says so plainly, lockfiles (task 25's `Quiet`) and generated or noisy files are quiet at P only, and HORIZON items persist until `resolved` or `cleared`. OD-27 correctly overrides the design's "~20 min". Both skeleton tests are present (`TestDifferentBlobsRaiseNothingAtH`, `TestLockfileQuietAtPLoudAtH`). Citations checked and real: `task-22.md:149-153` (`verbs.Hunk`), `task-16.md:246-250` (`SnapHunk`, `[start, end]`), `task-06.md:203-205` (route 57, `1 ≤ start ≤ end`, ≤ 20 ranges), `task-25.md:124-127` and `:217` (`Quiet`, `package-lock.json`), `task-30.md:239-243` (`a.Tier` switch). The input side does not hold together, though.

### S04-C2 (Critical): task 34 sends `[start, count]`, and task 36 reads `[start, end]`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:83-84, 107-108, 186
**Problem:** Task 34's ST.6 `Hunk.Ranges` is `[start, count]`, taken from the `-a,b` side of `@@`, and a count of 0 means a pure insertion after `start` (`task-34.md:155`, test `:265`: `[[3,2],[8,0]]`). Task 36 says the pairs are `[start, end]` inclusive (after `task-16.md:246-250`), and `overlaps` compares them that way. With the real input, a 30-line edit at line 100, which arrives as `[100,30]`, is read as lines 30 to 100 backwards. Tier H then misses real overlaps and raises false ones. The tier-H lines sent to route 57 also break its `1 ≤ start ≤ end` rule, so the Hub refuses the `conflicts.create` with a 422 and the conflict is never recorded. A pure insertion `[8,0]` becomes an inverted range. An insertion at the top of a file, `[0,0]`, has `start = 0`, which route 57 refuses too. None of task 36's tests catch this, because they are built by hand as `[start, end]`.
**How to fix:** Fix one format for the whole thread and convert at one boundary. The simplest choice is for task 34's `machined` (or task 35's fold) to change `[s, c]` to `[s, s+c-1]` for `c ≥ 1`, and a pure insertion to `[max(s,1), max(s,1)]` (the line it follows), so both sides of the seam say `[start, end]`. Then change task 34's comment and `TestHunkRangesAreOnHEADsBlob`. Add a task 36 test fed with a real task-34-shaped sample (`[[100,30]]` against `[[120,5]]` must overlap, and `[[8,0]]` against `[[9,1]]` must overlap as adjacent). It kills: the count read as an end.

### S04-I13 (Important): nothing ever calls `OnHunks` or `CoEdit.Observe`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:85-88, 450-456, 498-499
**Problem:** Task 36 needs task 35 to call `OnHunks` and `CoEdit.Observe` after each sample, and to expose `Activity.Hunks(ctx, project) []AgentHunks`. Task 35 is already written and does none of these things: it has no `OnHunks`, `Hunks(` or `CoEdit` anywhere, and nothing calls the forecaster on hunks. Task 36 lists this as gap 2 but leaves it for task 35, which comes first and is finished. So once both are built, tier H and the co-edit rate are dead code. `Deps.Activity` stays nil, and `CoEdit` never gains a row, so `Noisy` never quiets anything. Every task-36 test still passes, because they drive `OnHunks` by hand. Task 60 also consumes `Activity.Hunks` and `AgentHunks` (`task-60.md:100`), so the gap spreads.
**How to fix:** Have task 36 own the wiring as an anchored modify of task 35's fold. After the `OnPaths` call, call `OnHunks` with the sample's hunks, and call `CoEdit.Observe` once for each new `LastCommit.SHA` per agent. Also add `(*Table).Hunks` returning `[]forecast.AgentHunks` (or `verbs.Hunk`, if that avoids an import cycle). Add it to the Files list and add `TestSampleReachesTierH` (a fake `machined` page, two owners, one blob, overlapping ranges, and one `conflicts.create` at `H`). It kills: the fold not calling the forecaster.

### S04-I14 (Important): one owner's own workers can make a teammate's conflict path "noisy"
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:36-41, 265-303
**Problem:** The rate counts distinct **agents**, and an owner can run five workers that each commit to `src/auth.ts`. That makes `src/auth.ts` noisy for the whole project for 14 days, rate 1.0 over 5 lanes, and tier P stays silent on it for every other owner. The reason could be innocent (one person's fan-out), or it could be a persuaded Orchestrator directing workers. Either way one owner's behaviour switches off the cross-owner warning, and the cross-owner warning is the one thing tier P exists for (§8.2). ConE's co-edit signal is across developers, not across one developer's parallel branches.
**How to fix:** Count distinct **owners** as well as agents. Add `owner` to `fleet.coedit` and to `ObserveCommit`, and make a path noisy only when the rate holds over agents **and** it was touched by at least 3 distinct owners (a constant beside `CoEditMinLanes`). Add `TestOneOwnersFanOutIsNotNoise` (five of Alice's workers commit `auth.ts`, and a Bob and Alice P overlap on `auth.ts` still raises). It kills: a rate over agents alone.

### S04-I15 (Important): the H upgrade reaches `proceed-both` only if an unnamed caller changes
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:237-241, 500-502
**Problem:** Task 30's `KeyOf` routes `proceed-both` to the always-decide row only when `protocol.Action.Tier` is `H` or `M` (`task-30.md:239-243`). Task 36 makes `Strongest` the real tier, but it leaves "fill `Action.Tier` from `Strongest`" as a requirement on whichever part builds the `Action`, and it names no file. If that caller copies the Hub record's tier, which is frozen at `P` (gap 4), a pair whose lines overlap on one blob is let through as an **act**, with no human asked. That is exactly the case task 30 says must always decide.
**How to fix:** Find the part that builds `protocol.Action` for `proceed-both` and make it read `forecast.Open(...).Tier` (which task 36 fills from `Strongest`). Make that a modify hunk in this task, citing file and line. Add `TestPRaisedThenHOverlapProceedBothDecides` (a P conflict, then an H overlap, then an accepted `proceed-both`: one `proceed-both-hm` decide, and no act). It kills: `Action.Tier` read from the Hub record.

### S04-M11 (Minor): only a sample's last commit feeds the co-edit rate
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:282-287, 495-497
**Problem:** ST.6 carries `LastCommit` alone (`task-34.md:158-164`). An agent that commits three times between two sweeps adds only the third commit's paths. Rebases and amends swap the sha without adding a commit. So the rate undercounts, and it undercounts most for the fast agents that edit the most.
**How to fix:** State this in "does not claim". Or have task 34 carry the commits since the previous sample's head (`git log --format=%H --name-only <prev>..HEAD`, bounded), and call `Observe` once per sha.

### S04-M12 (Minor): the Hub conflict record keeps saying `P` after Fleet upgrades it to `H`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-36.md:503-505
**Problem:** Gap 4 leaves the Hub's `cf-…` at `P` while Fleet acts on `H`. The provenance thread (CLAUDE.md "Product spine") then shows a weaker tier than the one that set a decide going, so an auditor reading the Hub cannot see why a human was asked.
**How to fix:** Either add `tier` (it may only rise) to route 58 through task 06, or have the decide's Hub record (task 30) carry the tier it was judged at, so the audit trail explains itself.

## Task 37 — Budgets

The scope's rules are all covered: the human bypasses the budget and the queue (made `Immediate`, sorted first, admitted whatever the gate says), a human `pause` is done by Fleet with no turn, cost is metered per counterpart and resolved by Fleet rather than from `Event.Author`, there is a digest past 10 an hour every 15 minutes, a spent budget queues events and tells the owner once, and the split is served. Both skeleton tests are present. Decision (c), a spend cap instead of a token cap, is a justified deviation under OD-8, and the part names it. Citations checked and real: `task-26.md:161-162`, `:181`, `:284-294` (`OriginOf`, human iff an own `human.message`), `:439`; `task-10.md:117`, `:147`; `task-03.md:908`; `task-38.md:373-374` (`Ladder.Pause`/`Resume`, which exist). Owner-from-token and the Z5 membership of the four routes are tested.

### S04-I16 (Important): the per-turn cap and the team ceiling are shown to people but not enforced
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-37.md:44-52, 630, 677-685
**Problem:** Three of the four controls depend on changes that no part makes.
- **Per-turn cap.** Task 20 has no `TurnCapCents` and no ceiling on its per-turn mint (grep finds nothing in `task-20.md`). Phase 6's mint has no `capUSD` (gap 2). Step 4 has task 20 read `gate.TurnCapCents`, but `task-20`'s mint file is not in this part's Files list. So the owner sees *"Each coordination turn may spend up to A$0.50"*, and nothing enforces it.
- **Team ceiling.** Task 11's `HubState` has no `TeamPolicy` read (gap 3), so the gate falls back to 30. A team admin who sets `orchestratorTurnsPerHour: 10` is silently ignored, and `GET …/budget` reports `teamTurnsPerHour: 30`, which is not the admin's number. That fails open on a team policy.
- **Human pause.** `IssueHuman` is also required of task 31 (gap 4). Task 31 is finished and has no such method (grep finds only `Issue` and `PauseForDefault`), so `HumanPause` has nothing to call.

These are the same unowned-delta pattern as S04-I13.
**How to fix:** Make each missing piece an anchored modify in this task, listed under Files, with a test.
- Task 20's mint gains `CapCents` (and `TestTurnMintCarriesCap`).
- Task 11 gains `TeamPolicy(project)` from the team-policy feed (and `TestTeamCeilingBinds`, which would kill the fallback when a policy exists).
- Task 31 gains `IssueHuman` for `pause`/`resume` only.

Until P6 lands `capUSD`, the product must not show the per-turn line: `GET` returns `turnCapEnforced: false`, and task 56 renders nothing. When the team policy is unknown, `GET` returns `teamTurnsPerHour: null`, never `30`.

### S04-I17 (Important): a partial human pause sets a project-wide flag, and a partial resume clears it
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-37.md:393-404, 409-411, 562-567
**Problem:** `HumanPause(owner, project, [w1])` sets the one `(owner, project)` flag. That silences **every** fleet-origin turn, not just w1's. Say Alice pauses w1, then w2, then resumes w1. `HumanResume` clears the flag, so fleet-origin turns start again while w2 is still paused by Alice. In the next peer-triggered turn the tainted Orchestrator may `direct w2 resume`, because `resume` is in task 31's `fleetTurnPasses` (`task-31.md:254-256`). Peer text then undoes a human's own pause. This is the trusted-deputy path of S04-I1, made reachable from the UI.
**How to fix:**
1. Keep the flag for "pause everything" (`agents` empty) only.
2. Record per-worker human pauses as `dv-…` records with origin `human`. `InForce` already sees them.
3. Make task 31's gate refuse a fleet-turn `resume` of any pause whose origin is `human` or whose `approvedBy` is set; it becomes a decide.
4. `HumanResume` clears the flag only when the call resumes everything.

Add `TestPartialResumeKeepsOtherHumanPause`: pause w1 and w2, resume w1, then a fleet turn's `direct w2 resume` is a decide, not a directive. It kills a project-wide flag cleared by a partial resume.

### S04-I18 (Important): task 37 is over its 3-hour estimate
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-37.md:3, 71-92, 586-605
**Problem:** The task has four new source files and four routes. It has a migration with four tables, all implemented in both stores plus the suite. It changes three hunks in task 26 (and reverses one of task 26's tests) and wires `fleetd`, and it carries 18 tests. To close S04-I16 it also needs three more modify hunks. That is well past 1 to 3 hours. A partial stop could leave `Immediate` changed with no gate, which would put every human message ahead of the budget with no budget at all. That is harmless, but it is untested.
**How to fix:** Split it in two. 37a covers the gate, `counterpart.go`, the task 26 hunks, `orch_budgets` and `turn_log`, and the budget tests. 37b covers `pause.go`, `split.go`, `owner_pauses`, `budget_notices`, the four routes and the missing deltas from S04-I16.

### S04-M13 (Minor): a human-bypass turn is not metered to its counterparts
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-37.md:238-241, 256
**Problem:** Step 1 admits the whole batch before step 4 runs, and nothing says step 1 resolves `Admitted.Counterparts`. An over-cap counterpart's held events ride along with any human message. Its digest hold is skipped, no `AddCounterpartTurn` is written, and the turn's tokens are not charged to that counterpart. A peer who floods an active owner is never shown in "coordination with Bob".
**How to fix:** Resolve the counterparts for bypass batches as well, before returning at step 1, and record them. Add a case to `TestMixedTurnCountsEach` for a bypass batch.

### S04-M14 (Minor): the counterpart cap is per owner, but the digest clock is per Orchestrator
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-37.md:313, 325-329
**Problem:** `CounterpartTurns` is keyed on `(org, owner, counterpart)` across projects, while `LastCounterpartTurn` is keyed on `orch`, which is per project. So after ten Bob turns in project A, Bob's first event in project B finds `ok == false` and runs as a digest at once. The cap and the digest window then disagree about scope.
**How to fix:** Pick one scope. Per Orchestrator (per project) matches the budget and §4.4's per-Orchestrator wording. Add a two-project test.

## Task 38 — The template-only PTY writer and the delivery ladder

The writer is well built. Its API takes a template id and never bytes. There is one unexported table, and the refusal table covers `!`, `/`, `#`, `ESC [ Z`, every control byte, near-misses and bidi, plus a fuzzer, a structural `TestOneTable` and a one-writer grep guard. It never decodes what the terminal sends back. Rung 2 is off behind C4, and rung 3 fires on a byte-count idle signal and is capped at one nudge per worker per 3 minutes. Every scope test is present. Citations checked against the tree and real: `sandboxd/internal/wire/frame.go:16` (`Stdin Kind = 0x01`), `sandboxd/internal/server/server.go:191` (the replay write), `control-plane/internal/registry/session.go:22-23`, `control-plane/internal/reaper/reaper.go:650-658`, `sandboxd/internal/hub/hub.go:84` (`publish`), `task-39.md:344` (`Attached`, which is Postgres-backed across replicas at `:395`). The weak point is not the writer. It is **when** the ladder writes.

### S04-C3 (Critical): an idle nudge's Enter can answer a permission prompt with the human's authority
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:58-65, 401-403, 504-508
**Problem:** Rung 3 fires when the PTY has written nothing for 15 s. A worker blocked on a tool-permission dialog, a "trust this folder" dialog or a hook-approval dialog (task 33's O1) is silent. So it is exactly the state that the byte-count rule calls idle. Every template ends in `\r`, and in a selection dialog Enter confirms the highlighted option, which is usually "Yes". That is from training knowledge, UNVERIFIED at the pinned CLI. A directive, or any team update, would then approve whatever the worker was waiting to be allowed. It happens with the human's keystroke authority, and it defeats the `ask` rules that the OD-23 policy mechanism exists to put in force. Directives come from a tainted Orchestrator, so this is also a trusted-deputy path. Peer text leads to a directive, the directive causes a nudge, and the nudge approves a gated tool call. The part sees the risk (`:505-507`), but argues the quiet terminal makes it rarer. In fact the quiet terminal is the trigger.
**How to fix:** Do not infer "at the input prompt" from silence.
1. Have `sandboxd` report a typed harness state from hooks that the managed policy installs (task 13's layer): `Notification` with a permission-prompt type means "awaiting permission", and `Stop` means "turn ended". This reads no PTY bytes. The hook and matcher names are UNVERIFIED at the pinned CLI.
2. Rung 3 fires only when the last state is "turn ended", with no permission, `PreToolUse` or `UserPromptSubmit` signal since, and the byte-count idle also holds.
3. Until gap 6's probe records that a template typed into each dialog does not approve it, ship rung 3 **off** (like rung 2) behind `PANACEA_APP_FLEET_NUDGE=on`.

Add `TestNoNudgeWhileAwaitingPermission` (a permission state after the last `Stop`: zero frames). It kills idle-from-silence alone.

### S04-I19 (Important): a nudge submits whatever is already in the input box
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:46-53, 67-70
**Problem:** Decision (e) only stops rung 3 while the owner is attached. An owner who types half a prompt and then detaches (closes the Desk, loses the network) leaves that text in the CLI's input box, with nothing sent. The next nudge appends its template and presses Enter, so the owner's unfinished words are submitted as a turn, with a product sentence glued on. If the owner had typed a leading `!`, the input box is in bash mode, and the line runs as a shell command. The template lint only checks the template's own first byte, so it cannot see this. It is the same "keystroke with the human's authority" hazard that the part opens with.
**How to fix:** Track in task 39's relay only whether the owner has sent any `Stdin` frame since the harness last reported a submitted turn (`UserPromptSubmit`, from the typed-state feed in S04-C3). Never read the bytes. Suppress rung 3 while that flag is set. Add the residual-input case, including leading `!`, to gap 6's probe. Add `TestOwnerTypedThenDetachedNoNudge`, which kills a check on `Attached` alone.

### S04-I20 (Important): rung 4 depends on a task 20 API that does not exist, and the nearest one destroys the worker
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:116-117, 345-348, 405-410, 525-528
**Problem:** Task 38 needs `Sessions.Stop(ctx, agent, keepDisk bool)` and `Sessions.Resume(ctx, agent)`. Task 20 is finished. Its `Stop(ctx, org, id, exit ExitClass)` means "destroy, revoke its keys, end it" (`task-20.md:187-188`). Its `Resume(ctx, d delegation.Delegation)` is the delegation revoker's half (`:196-198`). So the obvious mechanical rename in "Consumes" turns every `pause`, including a human's pause and task 30's safe default, into a permanent end of the worker, and `resume` then has nothing to relaunch. Gaps 2 and 4 are also unowned:
- task 16 has no `pty.stats`;
- task 19 has no route for it;
- task 31 has no `AfterIssue` hook (grep finds none in `task-16.md`, `task-19.md` or `task-31.md`).

So the idle table stays empty and rung 3 never fires. That fails quiet, but the feature is dead. This is the same pattern as S04-I13 and S04-I16.
**How to fix:** Own the deltas here as anchored modifies, listed under Files:
- task 20 gains `Suspend(ctx, org, id)`: the pod stops, the disk and the agent row stay, the status becomes `paused`, and the turn keys are revoked;
- task 20 gains `Relaunch(ctx, org, id)`: it relaunches on the same disk with the same conversation id;
- task 16 sends `pty.stats`;
- task 19 accepts it from `sandboxd`'s own stream only;
- task 31 gains `AfterIssue`.

Use new names so that nothing can map onto the destructive `Stop`. Add `TestPauseThenResumeKeepsTheWorker` (same `ag-…`, same conversation id, and `agents.end` never written). It kills: rung 4 calling task 20's `Stop`.

### S04-M15 (Minor): two pauses close together send the double Ctrl+C that exits the CLI
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:253, 405-408
**Problem:** The refusal table itself names `"\x03\x03"` as "a double Ctrl+C exits the CLI". `Pause` sends one `0x03` per call, with no per-worker debounce. Two things can pause at about the same time: a human pause and a task 30 `PauseForDefault`, or two retried directives. Their two separate frames land within the CLI's exit window. The CLI then exits before `Suspend`, and phase 2 may treat the exit as the end of the session.
**How to fix:** Keep a per-worker `lastInterrupt` and skip a second `TemplateInterrupt` within `StopGrace`. The stop still runs. Add a test with two `Pause` calls 1 s apart that gives one `0x03` frame.

### S04-M16 (Minor): "no process in the VM can forge" `pty.stats` is asserted, not shown
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:269-272, 444
**Problem:** `sandboxd` runs inside the same Kata VM as the agent. Whether the agent can read `sandboxd`'s control credential or reach its connection depends on uid separation and file modes. Neither is cited. `TestStatsOnlyFromSandboxd` covers only the in-pod-socket path.
**How to fix:** Cite the uid split and the credential's file mode from task 16 or phase 3 (mark UNVERIFIED if neither states it), or state the residual. A forged "busy" only suppresses nudges, and a forged "idle" is bounded by the 3-minute cap.

### S04-M17 (Minor): 2.5 h spans two modules and a phase 2 file
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-38.md:3, 80-99
**Problem:** The writer and fuzzer, the ladder and idle table, the wire type in two modules, `sandboxd`'s reporter, the `hub.go` counter, the dispatch hooks, the wiring and 16 tests will not fit into 2.5 h. S04-I20's deltas add more.
**How to fix:** Move the `sandboxd` half (`hub.BytesOut`, `stats.go`, `control/ptystats.go`, and the task 16 sender) into its own task that this one depends on.

## Task 39 — Fleet's own attach handler and socket revocation

This is the soundest part in the section. It is a new handler in a new package, and it shares nothing with phase 2's `api/attach.go`. Access is decided per request, owner against owner, and an `admin` does not see a teammate's terminal (OD-15). "Not yours" and "not there" give the same bytes, including after the `RouteAgent` fold. Only `Stdin` and `Resize` frames go up. Keystrokes are dropped once `live` is false. Revocation is swept on each replica, with a fast path on the leader, and a closed token closes its socket at `exp`. `Attached` works across replicas and fails towards "attached". The route is in Z5. All three scope tests are present. `entra.Middleware` wraps the mux (`task-12.md:514`), so an unauthenticated probe gets a 401 before `Resolve` runs, and there is no oracle for callers without a token. Citations checked against the tree and real: `control-plane/internal/api/attach.go:61-85` and `:127-140`, `sandboxd/internal/server/server.go:19-25` and `:200-203`, `task-12.md:410-420`, `:1076`, `task-21.md:87`, `task-11.md:591-593`.

### S04-I21 (Important): worker attach depends on two deltas that no part owns
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-39.md:166-169, 218-221, 429-431, 488-491
**Problem:** The handler refuses every attach for which `Sandboxes.Current(ctx, a.ID)` is false. Task 20's `Current` is documented as "task 26's `turns.Sandboxes`" for an **Orchestrator** (`task-20.md:193-194`). Gap 4 asks task 20 to cover workers too, but task 20 is finished and does not. So attaching to a worker, which is the cockpit's main use of this route, answers `409 agent_not_running` for every worker. Gap 3 (a replica id in task 12's config, and `POD_NAME` in task 32's chart) is also left to finished parts. Without it every replica writes the same `replica` value, and `DeleteReplicaAttachments` at one replica's start deletes the other replica's live rows. `Attached` then reads false while an owner is typing, which is the exact case the table exists to prevent (task 38 decision e).
**How to fix:** Own both deltas here as anchored modifies, listed under Files:
- task 20's `Current` answers for any live agent (`TestCurrentForAWorker`);
- task 12's config gains `ReplicaID` from `POD_NAME`, and it refuses to start when that is empty;
- task 32's Deployment passes `POD_NAME` through the downward API (one helm-unittest assert).

Add `TestTwoReplicasDistinctIDs`, which kills a shared default replica id.

### S04-M18 (Minor): the sweep's behaviour when the membership re-check fails is not stated
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-39.md:46-53, 336-340
**Problem:** The sweep reads membership uncached, with one Hub call per owner and project. The part does not say what a Hub error does. If the socket is kept, a removed member keeps their view until the token expires (about an hour for Entra). If the socket is closed, every attach drops during a Hub blip.
**How to fix:** State the rule. Keep the socket through up to N consecutive failures (for example, three sweeps, 30 s), then close with `4003`. Add a test for each side.

### S04-M19 (Minor): during a Hub outage an existing id and a missing id get different answers
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-39.md:265-266, 274-278
**Problem:** `Resolve` answers 404 for a missing agent before `authorize` runs. An existing agent reaches `authorize`, which answers 503 while the Hub is down. So during an outage a member can tell which `ag-…` ids exist. The effect is small, because the ids are wide and random.
**How to fix:** For a missing agent, run the same membership call, or answer 503 whenever the Hub is unhealthy. Otherwise, state this residual in "does not claim".

## Task 40 — Runner rotation and DIVE

What is written matches the scope. Fleet decides when to rotate (by Sydney calendar day, or at a task boundary), and the runner only obeys `mode: "new"`. Each session gets its own `HOME`, and the old one is deleted whole, so no transcript byte survives and the runner reads nothing inside it (OD-8). The seed is a typed verb built from Fleet state, with the human's text filtered by owner. A DIVE is a claimed turn with origin `human`, runs interactive `--resume` and never forks, and `dive.end` is accepted from the runner's pid only. Citations checked and real: `task-26.md:558` (`SessionFor(…, false) // task 40 supplies rotateDue`), `task-31.md:41-44`, `task-27.md:30-41` and `:149-157`, `docs/AGENTIC-UX.md:147`.

### S04-C4 (Critical): the task 40 part is cut off after the seed shape
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:424
**Problem:** The file ends at line 424, at the `seed` result shape. It has no "Step 1: Write the failing tests first", no test table with mutants, no steps, no acceptance commands with expected output, no commit step, no "does NOT claim" and no gaps. `grep` for `Step 1|Mutant|NOT claim|Acceptance|Gaps` finds nothing. The three scope tests are missing: "the seed contains no byte of the previous transcript", "a turn event during a DIVE waits" and "a non-owner cannot DIVE". The source is also incomplete: `PrepareHome`, `PruneHomes` and `dive` are signatures only, the `00NN_rotation.sql` migration is never given, and neither are the `seed` verb's handler or registration nor `seed-result.json`. The "Requires of task 07" at `:356-358` says "lists it", but there is no list to put it in. Git shows three commits for the part (`c80da56`, `04e6944`, `48fcbe7`), and the last is "the DIVE wire and runner side", so the writer stopped before the tests. That fits the org-spend-limit stop.
**How to fix:** Finish the part by appending, and do not rewrite what is there. Add the rotation migration and `seed.go` handler, the test table and Steps 1 to 7 in the house style, the acceptance commands for both modules with expected output, "does NOT claim", and the gaps list, which should carry S04-I22 to S04-M22 below. The test table needs at least:
- `TestSeedHasNoTranscriptByte`: write a marker into the old session's transcript through a fake harness, rotate, and neither `seed` nor the new `HOME` holds it;
- `TestTurnEventDuringDiveWaits`;
- `TestNonOwnerCannotDive` (the 404 fold);
- `TestRotateDueSydneyMidnight`;
- `TestPruneHomesKeepsCurrent`;
- `TestPruneOnlyAfterNewChildStarts`;
- `TestDiveBeginRefusedMidTurn`;
- `TestDiveEndOnlyFromRunnerPid`;
- `TestSeedHumanFilteredByOwner`.

### S04-I22 (Important): a DIVE keeps `human` origin for 2 hours whether or not a human is present
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:59-66, 334-338, 376-381
**Problem:** Starting a DIVE neither attaches the owner nor needs the owner to be attached. The claim runs for `DiveMax`, which is 2 h. The whole time, `CurrentTurn` answers `human`, so task 31's gate passes a `free` directive, a new task or a wider scope (`task-31.md:303`). Interactive Claude Code carries on working after one prompt. An owner who types "catch up on the inbox and sort it out" and then closes the laptop leaves a tainted Orchestrator that reads peer text through `panacea inbox` and then issues free-text directives with the human's authority, while nobody sees them. §7.2 item 3 allows free text in a human turn only because each directive "is shown inline … so the human sees exactly what was sent". Task 31's author expected DIVE to register "an **attached** session as a `human` turn" (`task-31.md:43`). Task 40 drops the word "attached".
**How to fix:** Tie `human` origin to presence. `CurrentTurn` returns `human` for a DIVE only while task 39's `Attached(orch)` is true. After 60 s detached, the scheduler calls `StopDive(orch, "detached")`, and until then the origin reads `fleet`. Add `TestDetachedDiveIsFleetOrigin` (a DIVE with no attachment row: `direct w free` is a decide) and `TestDetachedDiveStops` (60 s of fake time: `dive.stop` with reason `detached`). Add `detached` to `DiveStop.Reason`.

### S04-I23 (Important): a DIVE has no gateway key, and reusing a turn's key charges the wrong counterpart
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:237-243, 333-338
**Problem:** Task 20 mints an Orchestrator's key per turn, in `Streams.Before("turn.begin", …)`, tagged with that turn's counterpart (`task-20.md:57-61`, `:225`, `:334`). A DIVE pushes `dive.begin`, which has no key field, and no hook mints one. So the interactive child either has no credential and cannot reach the model, or it uses the last turn's key. That key is capped by task 37's per-turn spend cap (50 cents), which would cut a 2 h DIVE short. It may also be tagged `counterpart: Bob`, so the owner's own DIVE spend is charged to Bob in task 37's split. Task 37's `Gate.Record` is not called on this path either (BeginDive does not go through `TryBegin`), so a DIVE appears in no `turn_log`.
**How to fix:** Register `Streams.Before("dive.begin", …)` in task 20 to mint a DIVE key: origin `human`, no counterpart, and a cap of its own (the owner's setting, or a fixed ceiling named here). Have `BeginDive` call `Gate.Record` with `Origin: human` and no counterparts. Add `TestDiveMintsItsOwnUntaggedKey`, which kills a DIVE reusing the last turn's key.

### S04-M20 (Minor): rotation clears `HOME`, but the child can write elsewhere
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:38-46
**Problem:** The working folder is kept empty (task 27's check), and `HOME` is deleted. But the child's Bash tool can write to `/tmp` or any other writable path in the pod, and those survive rotation. An old session could leave `/tmp/notes.md` holding its summary of peer text. A peer message could later tell the new session to read it. That is the "agent-written summary carried forward" that §4.2 and `06` I9 forbid.
**How to fix:** Put `TMPDIR` under the session directory (`/run/panacea/sessions/<uuid>/tmp`) and add it to `ChildEnv`. Name the pod's other writable mounts (task 15) as read-only, or pruned along with the session. Add one test: a file in the old session's `TMPDIR` is gone after a rotation.

### S04-M21 (Minor): a DIVE child reads keystrokes typed before the DIVE began
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:68-75, 376-381
**Problem:** Task 39 lets the owner attach to the parked runner, and the runner never reads its stdin. Keystrokes typed while no DIVE is running sit in the PTY's input queue. The interactive child reads them as soon as it starts, so a stale line with its Enter is submitted as the DIVE's first prompt.
**How to fix:** Flush the terminal's input queue (`tcflush(TCIFLUSH)` on the runner's stdin) just before `DiveArgv`'s child starts. Test it with a pty pair.

### S04-M22 (Minor): the DIVE record needs a Hub change to route 69 that no part makes
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-40.md:353-358
**Problem:** The preferred shape is a second write to the same `ot-…` id, and that needs task 07's route 69 to accept a revision. Task 07 is finished and has no such rule. There is no gaps list to carry the requirement, so the first DIVE's end record may be refused as a duplicate.
**How to fix:** Take the alternative that needs no Hub change: the start and the end are two `ot-…` records, and the end names the start (`diveOf: ot-…`, typed). If a revision is wanted, own the task 07 delta here.

## Section verdict

Critical: S04-C1 (task 34, submodule filter), S04-C2 (task 36, range format), S04-C3 (task 38, nudge answers a permission prompt) and S04-C4 (task 40 cut off). The Important findings follow one pattern: a part requires a delta of an earlier, finished part, and no part owns it (S04-I13, I16, I20, I21, I23). The other theme is trusted-deputy paths where peer-driven turns undo human decisions or act with human authority (S04-I1, I17, I22).

Section verdict: Needs fixes

## Task 40 re-check (the part was finished after this review)

Parts 31 to 39 are unchanged since this review was written (their files date from 11:33 to 12:46; the review from 18:35). Task 40 grew from 424 to 678 lines in commits ad9210c, 8f86691 and 5d67d8f (20:41 to 20:56). This section reviews lines 425 to 678 and re-checks S04-C4 and S04-I22 to S04-M22 against them.

# Plan review, section 05: tasks 41 to 50 of Phase T (team mode), second pass

Repair checked: repair-gaps.md

Reviewer: adversarial plan reviewer, 2026-09-25, after the sign-out rewrite. Parts read one at a
time against their skeleton scope (`docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`,
"## Task scopes") and the parts they depend on. Repair gaps 8 to 13 fall in this section.

## Task 41: the Orchestrator's instructions and the replay eval (750 lines, complete)

Coverage is good: both texts in full, I13, the append sentence pinned by one fixture, the eval
types, the scorer rules, eight scenarios, and a test table with named mutants. The part is
complete, not cut off. Citations checked: `05-design.md:381-389`, `:582-588`, `task-13.md:296-439`,
`task-09.md:287-320`, `:596-611`, `task-19.md:137`, `task-18.md:8`, `:100`, `task-16.md:42-43`,
`task-27.md:24-29`, `:185-190`; all real.

### S05-I1 (Important): The team pointer paragraph lands in every solo cloud worker
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-41.md:203-213, :274-287
**Problem:** `ProductLayer()` always sets the worker section's `claudeMd` to `worker-cloud.md`, and
`Render(layers, class)` has no team or solo input, so every worker render carries §5.3's team
paragraph. Task 15 makes the policy file mandatory for every Kata pod, including solo ones
(`task-15.md:566-569`). A solo cloud worker is then told other people are editing the project, to
run `panacea check` before each first edit, and to report `blocked` when the team view is
unavailable. That contradicts §5.3 (`05-design.md:580-581`: the paragraph is for a team project's
managed block) and task 50's rule that solo is unchanged. No test in the table renders a solo
worker. This finding was open at the first pass and the rewrite did not close it.
**How to fix:** Give the product layer a mode input: add a `Team bool` to `Layers` (or a second
function `RenderTeam`) and apply the worker `claudeMd` only when it is set. Add
`TestSoloWorkerRenderHasNoTeamParagraph`, which kills a product layer that ignores the mode. State
which caller (task 14 or task 15) reads the project's mode and passes it.

### S05-I2 (Important): `seed` is not in task 09's verb table, so the text names a verb Fleet refuses
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-41.md:99, :647, :670, :680
**Problem:** The Orchestrator text says run `panacea seed` first, and `01-seed-first` makes `seed`
with code `ok` a `must`. But `model.Verbs` (`task-09.md:287-304`) has no `seed` row, and task 40
only registers a handler in `fleetd` (`task-40.md:591`); it adds no row to `Verbs` and nothing to
the sandbox CLI (grep finds `seed` in neither task 18 nor task 19). So `ClassMay(ClassOrchestrator,
"seed")` is false, `TestTheTextNamesOnlyOrchestratorVerbs` and `TestEveryScenarioLoads` fail as
written, dispatch answers `unknown_verb`, and the live canary `01-seed-first` can never pass. It
would then report "the managed `claudeMd` did not load" when the real cause is the verb table.
**How to fix:** Give one task the row: task 40 (or this task) adds `{Name: "seed", Classes:
[]Class{ClassOrchestrator}}` to `model.Verbs`, the `seed` subcommand to task 18's CLI, and `seed` to
`TestVerbTable`. Then list that task as this part's dependency, with its line.

### S05-I3 (Important): Task 41 is two to three tasks of work
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-41.md:3
**Problem:** "About 3 hours" covers two texts and a fixture, a new invariant with a product merge
and two moved goldens in `policy`, an argv change in the other module, moving every verb
registration out of `fleetd`, an in-process Fleet `World` wired from six tasks with two fixture
worlds and their `cf-`/`ms-`/`th-` records, the scorer, eight scenarios and a Linux-only live
driver that serves the control socket. The `World` alone (memstore, Hub fake, queue, scheduler on
a fake clock, dispatcher, every handler, and fakes for the broker, `machined` and the gateway) is a
focused afternoon. The house rule is 1-3 hours per task.
**How to fix:** Split into 41a (texts, `instructions.go`, I13, goldens, argv; 2 hours), 41b
(`RegisterVerbs`, `World`, scorer, scripted driver, scenarios; 3 hours) and 41c (the live driver and
its test; 1-2 hours). Update task 42's dependency to 41b and 41c.

### S05-M1 (Minor): `productExact` does not compile against `Render`'s `doc`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-41.md:246, :284
**Problem:** In task 13's `Render`, `doc` is `any` (`decode` then `merge(dst, src any) any`,
`task-13.md:186`, `:395`). `productExact(class model.Class, doc map[string]any)` is called with it
directly, which is a compile error.
**How to fix:** Take `doc any` and assert `m, _ := doc.(map[string]any)` inside, or assert at the
call site.

### S05-M2 (Minor): The id alphabet disagrees with itself
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-41.md:367, :520
**Problem:** `Pattern.Gets` lists `dv es ms ls tk ag`, but the id walk's pattern also accepts
`th-`. A scenario that sets `gets: "th"` is then undocumented but scores.
**How to fix:** Name one list and use it in both places, and have `TestEveryScenarioLoads` reject
an unknown `gets` prefix.

## Task 42: G9, the corpus and the two-Orchestrator run (478 lines, complete)

The corpus is complete (33 lines across five files, secrets assembled at load time), the counts
read Fleet's typed state only, and both halves are specified. Citations checked:
`task-31.md:190-200`, `:239-255`, `:318-322`, `task-38.md:166-170`, `task-37.md:430-455`; all real.

### S05-I4 (Important): `Obedient` is not the worst model: half its acts are `bad_request`, and it never crosses owners
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-42.md:317-331, :475-477
**Problem:** `direct`'s kinds are `pause|resume|narrow|wait-for|rebase|free` (`task-31.md:196`,
strict decode). `Obedient` sends `direct` with kinds `scope-widen`, `task-create` and
`worker-spawn`, so those three calls fail decoding and never reach task 31's gate. The real paths
for a new task and a widened scope are the `task` verb (`create`, `update`), and `Obedient` never
calls `task`, although line 14 says it issues "a task whose title is the payload". Gap 4 says
"`Obedient` sends them there" if task 31 routes them elsewhere; task 31 is written and does, but the
code was not changed. `Obedient` also never names the other owner's objects (`direct` to Linus's
worker, `worker stop` of it, `task update` of Linus's task, a `lease request` on the held
`src/auth/**`). Task 41 moves every registration into `RegisterVerbs`; a registration that loses
`OwnArgs("to")` in that move would pass G9. So "a pass with Obedient is a pass whatever a real
model does" (`:296`) is not true. The same vacuity sits in task 41's `04-conflict-yield`
`mustNot` pattern `widen` (`task-41.md:621`), which matches a `direct` kind that cannot exist.
**How to fix:** Replace the three bad `direct` acts with `task` `create` (title = payload), `task`
`update` widening the scope to `**`, and `worker` `spawn`. Add cross-owner acts against Linus's
worker, task and lease, and assert each answers `agent_not_owner` (or `lease_contested`) and adds
no `dv-…`. Add a test that every `Obedient` act decodes (no `bad_request`), which kills a vacuous
act list. In task 41, make `widen` a `task` `update` pattern.

### S05-M3 (Minor): The out-of-scope definition and `outOfScope` disagree on origin
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-42.md:35-37, :283
**Problem:** The definition counts a `scope-widen`, `task-create` or `worker-spawn` with no approval
"of any origin"; the code counts them only when `Origin == fleet`. An owner's own widen in a
human turn is legitimate, so the code is right and the prose is wrong. `TestCountDefinitions` only
checks the any-origin case for `narrow`, so the drift is untested.
**How to fix:** Change the prose to "in a Fleet-started turn" for those three kinds, and add a
`TestCountDefinitions` row: widen, human origin, no approval → 0.

### S05-M4 (Minor): Owner-message exfiltration is in the corpus but never counted
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-42.md:87, :411-419
**Problem:** Corpus line 87 asks the Orchestrator to send its owner's messages. No CI test puts a
marked owner message in the world and checks that no `peer`, `direct`, `task` or `human` args
carry it; only task 41's live scenario `06` scores that, and only for a real model.
**How to fix:** Add `TestG9OwnerTextNeverLeavesTheOwner`: seed an owner message holding a marker,
let `Obedient` echo `seed` and `inbox` output into its `peer` text, and assert the Hub fake's
`ms-…` bodies hold no 8-rune window of the marker. If Fleet cannot stop a model relaying its own
owner's words, say so as a stated limit instead.

## Task 43: Hub delta, role-to-verb, auditor and compliance, team policy, Z5 (539 lines, complete)

The §3.4 table as data, `Can`, the compliance capability, routes 74, 75 and 79 in full, the Z5 team
harness and fourteen tests are all present; both scope tests are named first. Hub citations
checked on `.claude/worktrees/hub` at `8495925`: `task-16b.md:50-58` (`want [6]int`),
`hubapi/types.go:23-25`, `hubapi/codes.go:9`, `:25`; `task-01.md:562-579`; `task-03.md:388`.

### S05-I5 (Important): Task 04 already creates `team_policies` with a `route` column, so this migration fails and the route has two homes
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-43.md:216-243, :525-528
**Problem:** Gap 1 says task 04's part "is not written". It is: `task-04.md:669-685` creates
`team_policies`, and task 04 lists task 43 as the consumer of that table (`task-04.md:100`). So this
part's `CREATE TABLE team_policies` fails with "relation already exists". The two tables also
disagree: task 04's has `route inference_route NOT NULL`, `revision DEFAULT 1`, `updated_at DEFAULT
now()` and `hub.all_distinct(escalation_floors)`; this part's decision (a) says the route is read
through from the project record and is "not a column here". With task 04's table merged, route 79
writes one route and the gateway may read another. Step 1's "if task 04's lands with one, drop this
migration" leaves the conflict to the builder.
**How to fix:** Drop this part's `CREATE TABLE`. Either amend task 04 to remove `route` (decision
a), or have this part ship an `ALTER TABLE team_policies DROP COLUMN route` migration, and say which.
Add RLS here only if task 04's loop (`task-04.md:772`) does not already cover the table. Delete
Gap 1's stale sentence.

### S05-I6 (Important): The structural rule-5 walk fails on two rows other tasks require
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-43.md:419-425, :486-488
**Problem:** The walk requires every non-`GET` row's `tcMp`, `tcA`, `tcAu` and `tcFx` cells to be
≥ 400, except routes 72, 74-79 and the `ownA` cells of 46 and 70. Two write rows break it:
1. Route 49, revoke a delegation, has `A` = ✓ on another member's delegation (`task-01.md:565`).
2. Task 06's admin stale-lease release is 201 under Grace's delegation on Linus's lease
   (`task-06.md:508`). That is an `Fx` ✓ on route 56, where task 01's table says `403a`.
So `TestNoWriteRowLetsAnyoneWriteAnothersRecord` fails as written. Step 3 then says "a failing
cell is a real finding: fix the handler, never the row", which tells the builder to remove the
admin's revoke and the stale release. Both are required: leavers (task 44) and §3.4's
`CapReleaseStale`. Rule 5 names tasks, leases, messages, directives and escalations; a delegation
is not on the list, but a lease is, so the stale release is a real exception that needs an owner's
ruling, not a silent pass.
**How to fix:** Add route 49's `tcA` to the exceptions with its reason (a delegation is team
administration, not an owner's record). For the stale release, add a separate row `admin-release
of a stale lease (56)` with its own cells, list it as the one named exception to rule 5 on a lease,
and have task 01's table show it. Change Step 3 to "a failing cell is a finding; stop and report it
if the row and the scope disagree".

### S05-M5 (Minor): Three hours is short for the team matrix plus three routes
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-43.md:3
**Problem:** Eleven columns across every team route (46-79), generated rows for tasks 06 and 07,
`orgRows`, fourteen integration tests, a migration, a store, three routes, auth changes, three
client methods with conformance rows, and contract edits. Three hours is at the top of the house
range with no slack for the reconciliation Step 1 asks for.
**How to fix:** Split the matrix harness and rows (43a) from the policy routes, store and client
(43b), or state four to five hours and why.

## Task 44: Hub delta, leavers and revocation end to end (422 lines, complete)

The part is whole now (the first pass found it cut off after the Hub store half). The removal
and its revocations share one transaction, the viewer downgrade emits `team.member_set` (the first
pass's M10 is fixed), the lease effect is written, and the 60-second chain runs on the real Hub.
Citations checked: `task-11.md:47-56`, `:568-570`, `:580-585`, `:693-700`; `task-12.md:850`,
`:976-999`; `task-23.md:143`; `task-39.md:330-334`; `task-04.md:289-304` (every column
`RevokeForLeaver` names exists).

### S05-I7 (Important): The account check guards renewal only, so a disabled leaver creates a fresh delegation instead
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-44.md:148-153, :200-223, :398-401
**Problem:** The account check runs on route 48 (renew) and in the daily sweep, never on route 46
(create). Under OD-30 a disabled Entra account keeps its offline refresh token until the desktop's
7-day cap (`sec-90-identity.md`, ID-I8), so the leaver's desktop can still call route 46. When a
renewal answers 403 `account_disabled`, the desktop can create a new delegation, which lives 24
hours until the next sweep revokes it, and then do it again. The "does not claim" line says a
disabled leaver is "bounded by the 24-hour lapse alone" while the check is `off`; under OD-30 the
real bound is the 7-day offline cap, because the lapse is only a lapse if nobody renews.
**How to fix:** Run the same `AccountState` check on route 46 (a disabled or unknown account → 403
`account_disabled`, nothing created) and add `TestCreateByADisabledAccountIsRefused`, which kills a
check wired to renewal only. Restate the bound with the check `off` as "the desktop's 7-day offline
cap (OD-30)", and cite ID-I8.

### S05-M6 (Minor): The downgrade to viewer has no fake and no conformance row (repair gap 13)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-44.md:428-430
**Problem:** Gap 2 worries that task 08's fake records `team.member_removed` for a downgrade. Task
08's fake has no role-change function at all, only `RemoveMember` (`task-08.md:350-371`). So nothing
in the fake revokes on a downgrade, and no conformance row checks that the fake and the real Hub
agree on it. Fleet code tested on the fake would never see a downgrade revoke.
**How to fix:** Add `SetRole(userID, role)` to task 08's fake, revoking on `viewer` and recording
`team.member_set`, plus a conformance row `team-feed/downgrade-revokes` that runs on both. Replace
Gap 2 with that.

### S05-M7 (Minor): Stale citations and one untested ordering
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-44.md:17, :22, :39, :348
**Problem:** (1) `task-08.md:280-283` and `:343` are the feed flags and the idempotency row; the
conformance row `team-feed/member-removed-revokes` is at `task-08.md:413`. (2) `task-06.md:111-114`
is route 47; route 48 (renew) starts at `:116`. (3) Line 39 cites `task-11.md:565-567` (imports);
the bound is `:568-570`, as line 344 has it. (4) Repair gap 7: a delegation that lapsed and is then
removed must stop, not stay paused. Task 11's "a revocation outranks a lapse" (`task-11.md:693-695`)
covers it, and `RevokeForLeaver` has no expiry filter, but no test here proves it on the chain.
(5) Repair gap 12: the skeleton scope (`2026-09-25-phase-t-team-mode.md:897-902`) names only the
renewal check; the part's Gap 1 says so, but the skeleton is not amended.
**How to fix:** Fix the three citations. Add `TestLapsedThenRemovedStops` to the chain table.
Amend the skeleton scope to name the sweep.

## Task 45: Hub delta, audit to Sentinel and the anchored head (710 lines, complete)

New since the first pass (which found it missing). The metadata-only mapping is a real allowlist
with a grammar check, the cursor sits outside the write path, the Terraform is written, and the
scope's two tests (`TestAConsistentRewriteIsCaught`, `TestNoTextLeaves`) are present. Checked:
`azurerm 4.81.0` matches `.claude/worktrees/env` at `b6b7568`; phase 8 has no rule that forbids a
public PaaS endpoint (`task-02.md:88-93` is the state account's own switch).

### S05-I8 (Important): Check A4 keeps the latest copy per `seq`, so a re-sent forged record wins
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-45.md:262, :418-419
**Problem:** The events query is `summarize arg_max(TimeGenerated, *) by Seq`, and A4 compares only
that one record per `seq`. `TimeGenerated` is the Hub's own value (the event's `at`), not the time
Azure received it. Whoever holds `id-hub` (any pod run under the Hub's service account, which is
the attacker the part names) can rewrite rows 12-30 and then re-send records for 12-30 with a later
`TimeGenerated`. A4 then passes. Anchors (A1) still catch a rewrite older than the last anchor, so
the gap is the rewrite in the hour after the last anchor, which is exactly what A4 exists to close
(`TestARewriteBeforeTheNextAnchorIsCaught`). A4 also has no completeness rule: a `seq` at or below
an anchor's `StreamedSeq` that never reached the workspace is not a fault.
**How to fix:** Read every copy per `seq` (no `arg_max`), and FAULT when two copies of one `seq`
carry different `Hash` values (an honest re-send after a crash has the same hash). Order by
`ingestion_time()`, not `TimeGenerated`. Add A6: every `seq` from 1 to the largest anchored
`StreamedSeq`, within retention, has a streamed record. Add `TestAForgedResendIsCaught`, which
kills `arg_max`.

### S05-I9 (Important): The team-mode gate is unenforced, and it checks a setting, not delivery (repair gap 8)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-45.md:57-61, :386-395, :696-698
**Problem:** (1) No part calls `TeamModeAllowed`: grep finds it only in task 45, and task 06's gap 3
(`task-06.md:718`) says no task owns the write that sets `mode` to `team`. So "part of the gate
before the first real teammate" is a document, not code, as Gap 1 admits. (2) Even once called, it
answers nil whenever `PANACEA_APP_HUB_AUDIT_SINK=logs`. A Hub whose sends all fail (a missing role:
`403` is logged at ERROR, and the cursor stops) still admits team mode while nothing leaves the
building.
**How to fix:** Give the mode write an owner in this phase (task 21's Share, or a named Hub delta
task) that calls `TeamModeAllowed` in the same transaction, with a test that the write is refused
when the sink is `off`. Make `TeamModeAllowed` also require, for the org, a successful send within
the last `2 × Every` and an anchor within the last `2 × Anchor`, from `audit_stream_cursors`. Add a
test with sink `logs` and `FailN` large, which kills a setting-only check.

### S05-M8 (Minor): The Sentinel rule cannot see an org that never anchored, or one silent for a day
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-45.md:525-530
**Problem:** The query reads only `PanaceaHubAuditAnchor_CL` rows from the last day. An org that
never anchored has no row, so the rule never fires for it (A5 catches it only when someone runs
`hub verify`). An org whose last anchor is older than 24 hours drops out of the query, so the alert
stops after 22 runs.
**How to fix:** Base the rule on the orgs seen in `PanaceaHubAudit_CL` over `P7D`, left-joined to
their latest anchor, and fire when the anchor is missing or older than 2 hours.

### S05-M9 (Minor): Least privilege, the cursor trigger, and the phase 8 ownership (repair gaps 9, 10, 11)
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-45.md:43-45, :307-308, :466, :534, :541
**Problem:** (1) `Log Analytics Reader` on the whole workspace lets the Hub read every platform log
(Container Insights and the rest), not just its two tables. (2) The forward-only trigger fires
`BEFORE UPDATE` only; an `INSERT` with `streamed_seq` past the head is not refused.
(3) Repair gap 9 is real: the skeleton file map's row for task 45 (`2026-09-25-phase-t-team-mode.md:261`)
names only "Hub task 27's audit route and chain". It has no phase 8 row for `modules/observability`
or task 10's `id-hub` roles. (4) Repair gap 10: `azapi` is a new provider for phase 8, which is an
owner call, and the part asks for one. (5) Repair gap 11: the collection endpoint is public, with
only Entra auth in front of it. The data is metadata only, so this is acceptable for TM1 if the
owner agrees.
**How to fix:** Scope the reader role to the two tables (table-level RBAC), or state why the
workspace is needed. Make the trigger `BEFORE INSERT OR UPDATE`. Add the phase 8 rows to the
skeleton's file map. List gaps 3 and 4 in the owner-question list for phase 8.

## Task 46: Hub delta, notes and erasure tombstones (379 lines, complete)

Routes 72, 73 and 81-83, the guard exception, three `SECURITY DEFINER` tombstones, the stored-reply
fix and the Fleet inbox field are all present; both scope tests come first. Task 04's names match
this part's provisional ones (`guard_note`, `guard_message`, `guard_human_message`, `text_body`,
`tombstoned_at`: `task-04.md:87-88`, `:647-663`). The first pass's I5 is half fixed: the test (`:284`)
and the Z5 row (`:307`) now agree on 404 for a human message.

### S05-M10 (Minor): The erase routes' visibility prose contradicts their Z5 rows
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-46.md:113-121, :304-308
**Problem:** The route table says a caller who may not erase gets 403 `forbidden` on all three
routes, including route 82, and says the record is found "under row-level security alone, with no
team check". RLS is per org, so a same-org non-member (`N`) finds the record and gets 403. But the
Z5 rows say `N` is 404 on 81-83, and `V`, `M′` and `A` are 404 on route 82. A builder who follows
the prose fails the matrix, and one who follows the matrix must invent the team check. Ids are 64
random bits, so the existence leak is small. The contradiction is still a spec defect on a privacy
rule.
**How to fix:** Rewrite the table's "Otherwise" column to match Z5: route 82 answers 404
`human_message_not_found` to everyone but the owner, the auditor (403) and the org admin; routes
81 and 83 answer 404 to a non-member of the record's team; and say where the team check runs.

### S05-M11 (Minor): Stale "task 04 is not written", and the notes cursor after a restart does not exist
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-46.md:65-66, :155-157, :267-268, :374-375
**Problem:** (1) Three places say task 04's part is not written; it is, and the names already
match, so gap 3 is closed. (2) "A restart rebuilds the set from route 73 with `after` at the
owner's last-read id, which task 28 already persists for messages." Task 28 persists per-message
`read_log` rows (`task-28.md:403`, `:540`), not a last-read id, and nothing persists which notes
were read. After a Fleet restart, every note is either re-delivered, quoting a colleague's words
into the Orchestrator again, or none is.
**How to fix:** Delete the three stale sentences and gap 3. Persist note reads in task 28's
`read_log` (kind `note`) and rebuild the unread set from it. Add
`TestARestartDoesNotRedeliverANote`, which kills the in-memory-only set.

### S05-M12 (Minor): Two hours is too small
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-46.md:3
**Problem:** Five routes, a migration with three guards and three definer functions written out in
full, fourteen integration tests, Z5 rows, the stored-reply change on task 07's route 70, and a
Fleet inbox change with three tests.
**How to fix:** State three hours, or move the Fleet half (the `Envelope.Notes` field, recipients,
the unread set) to its own small task.

## Task 47: Phase 8 delta, AU egress, G11 and the probes (424 lines, complete)

The default is the right way round (an unlabelled pod gets no internet), the broker stamps the
label and refuses it from callers, G11 is written against the real `guards.sh` (365 lines, rules
`begin 9`/`begin 10` at `:324`/`:345`, the `TFVKEY` rule at `:179`: checked on
`.claude/worktrees/env` at `b6b7568`), and P28 tests the allowlist itself. Phase 8 citations
checked: `task-11.md:155-159`, `task-32.md:17-19`; this plan's `task-17.md:325` (`WithLabels`).

### S05-I10 (Important): The AU allowlist leaves DNS open to any name, so an agent can tunnel project text out through lookups
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-47.md:177-184, :196-199, :160
**Problem:** The Cilium DNS rule is `matchPattern: "*"`, and the part says so: "The DNS rule allows
every lookup". CoreDNS forwards to Azure's resolver, so an AU agent can send the project's text as
`<chunk>.attacker.example` lookups at a few hundred bytes a query. That needs no allowlisted host
and no upload. Task 11's `allow-dns` policy is also kept for every pod (`:160`), and Kubernetes and
Cilium allows are a union. So narrowing the Cilium rule alone would not close it: an L4-only
allow on port 53 is a wildcard at L7. "Does not claim" lists uploads to github.com and the
registries, but not DNS. P28 does not test it.
**How to fix:** Make the AU DNS rule list the allowlist names, the cluster suffix and the gateway's
name only (`matchName`/`matchPattern` per entry). Exclude `route: au` pods from task 11's
`allow-dns` so the Cilium proxy is the only DNS path. Add a P28 probe that a lookup of an unlisted
name gets `REFUSED`, and a helm-unittest case that no policy gives an AU pod port 53 without the
DNS rule. If this is deferred, add DNS to "does not claim".

### S05-I11 (Important): Raising a project to AU leaves its running sessions on `global` egress
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-47.md:201-219
**Problem:** The route label is stamped only at pod create. Route 79 (task 43) lets any member raise
a project to AU at any time, and grep finds `team_policy.route_changed` handled only in tasks 01,
04 and 43, which re-read quotas and budgets. No task relabels, drains or recreates a running
session when its project becomes AU. Its pods keep open internet for the rest of their life. That
is the moment a member has just said "this data must stay onshore".
**How to fix:** On `team_policy.route_changed` to `au`, have Fleet (task 20's `Sessions`) stop and
relaunch, or pause, every session of the project whose pod label is `global`. Add a test that a
route change to `au` leaves no running pod labelled `global` for that project within the feed's
60-second bound. State the lowering direction too (a pod labelled `au` on a project lowered to
Global is only too tight, so leaving it is safe).

### S05-M13 (Minor): Both git hosts for every AU project, and a stale checklist count
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-47.md:94-99, :384
**Problem:** (1) Every AU pod may reach `github.com` and `dev.azure.com`/`*.visualstudio.com`,
whichever host its project uses, so an ADO project's agent can push to any GitHub repository. §2.5
says "the git host only". (2) The acceptance expects 28 `NOT RUN` lines ("task 32's 22"), but phase
8 task 32's rows are P01-P16 and P20-P22, which is 19 rows, so the expected count is 25 unless the
record repeats the phrase. Take the number from the merged file.
**How to fix:** Render the git-host entries per project host, from the project's `gitHost`, into
a per-project policy, or state why one list is accepted. Replace the literal count with "task 32's
count plus six".

## Task 48: Phase 3 delta, the read-only per-project fetch credential (376 lines, complete)

The write wall is in our code first (a distinct `Auth` and `Source`, refused before any git call or
HTTP request), the GitHub token's answer is checked and revoked if wide, no credential reaches a
session sandbox, and no token is logged. Checked: `guards.sh:33` is G9's provider list on
`.claude/worktrees/env`; OD-22 is `2026-09-23-owner-decisions.md:150`.

### S05-I12 (Important): The re-proof is a boolean the desktop reports, so the broker trusts the client for the one control §10.2 relies on
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-48.md:206-219, :274-279
**Problem:** The proof route takes `{"repoKey", "ok": true}` from the machine owner's desktop token
and stores it. No evidence travels with it: the broker never sees her host token or the host's
answer. A person removed from the GitHub organisation, or from the ADO repository, keeps a valid
Panacea desktop token. She can `PUT ok: true` every day (a modified client, or one `curl` with her
token), and the broker keeps minting the project's read token and fetching the repository into her
machine's clones. Her lost host access is then routed around by our own service account. That is
the trusted-deputy case, and it makes the "does not claim" bound (`:348-351`, "until the next
sign-in or the proof TTL") unbounded. `TestProofRouteIsTheOwnersDesktopOnly` checks who may send
the proof, never whether it is true.
**How to fix:** Have the broker run `ProveAccess` itself. The desktop sends her own short-lived
host token in the `PUT`; the broker makes the one read call, keeps only the result, and zeroes the
token in memory (the same rule as a push's per-act token, phase 3 task 13). Store `ok` from the
host's answer. Add `TestAProofNeedsTheHostsAnswer`: a `PUT` claiming `ok: true` with a token the
stand-in host answers 404 is stored as `ok: false`. This kills a broker that stores the client's
boolean.

### S05-I13 (Important): No task stops a leaver's fetches: Gap 5 hands it to task 44, which does not do it
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-48.md:236-243, :375-376
**Problem:** `fetchLoop` fetches for "the team projects of the machine that have a binding AND a
fresh proof", but it never checks the owner's team membership or a live delegation. Gap 5 says
task 44 should delete a leaver's proofs. Task 44 has no mention of proofs or of the broker (its file
list, `task-44.md:41-58`). So a member removed from the team keeps up to 24 hours of fetches of the
team's repository into her machine, after the Hub has revoked every delegation she held. The
first pass raised this, and it is still open.
**How to fix:** Gate `fetchLoop` and the fetch route on a live delegation of the machine's owner
for that project (task 17's delegation check, which the fetch route already uses for Fleet). Or
add to task 44 a fan-out effect that deletes the proof entry on `delegation.revoked`. Add
`TestARemovedMemberIsNotFetched` on the chain test's fake clock, which kills a loop keyed on the
proof alone.

### S05-M14 (Minor): One ADO identity per team project is wider than "one repository"
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-48.md:51-53, :268-270, :369-371
**Problem:** The Files list says one user-assigned identity "per Azure DevOps team project", and the
runbook grants it Read "on one repository". Two Panacea projects on repositories in the same ADO
team project would share one identity, so each project's fetch credential reads the other's
repository. The Terraform map is keyed `ado_fetch_projects`, which does not say which.
**How to fix:** Key the identity by Panacea project (one identity, one repository), and have
`TestBindingsValidateAtStart` refuse two bindings that share a `clientId`.

## Task 49: Phase 3 delta, the team land review (362 lines, complete)

The rule is enforced in `Land` at step 9a and not in the lens. A Hub error refuses, and never
means "no approval needed". The approval is bound to the lane tip and covers its paths. The
database refuses a self-approval. The three scope tests come first.

### S05-I14 (Important): The approver reviews a diff from a base the owner chose, and `Land` never checks that base
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-49.md:112-126, :141-143, :266-268
**Problem:** Route 84 takes `base` from the lane owner's body, and the approver's diff is
`base..laneTip` (`:252`, `:268`). Step 9a computes the changes as `old..laneTip` (`old` = the
integration head at land time) and checks only `LaneTip`, `ApproverUserID` and path coverage. The
converge `Approval` has no `Base`. So Linus commits a hostile `CLAUDE.md` at T1, adds a harmless
edit to it at T2, and asks with `base = T1`. Grace sees a one-line diff and approves. The land then
applies the whole hostile file, since `CLAUDE.md` is covered and the tip matches. The same gap
opens without intent when the integration head moves between approval and land.
**How to fix:** Add `Base` to `converge.Approval`. Require in step 9a that `a.Base == old` (the
integration head `Land` diffs against), or recompute the changes from `a.Base` and refuse unless
it equals `old`. Have route 84 refuse a `base` that is not the project's current integration head.
Add `TestAnApprovalOfAnotherBaseDoesNotCount`, which kills a step 9a that ignores the base.

### S05-I15 (Important): Path matching is case-sensitive and blind to imports and symlinks, so the second-approver rule is easy to step round
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-49.md:34-48, :294
**Problem:** (1) The table is matched case-sensitively, and `docs/claude.md` is pinned as "none". On
a case-insensitive disk (Windows or macOS teammates in local mode, which stays per OD-20),
`claude.md` or `.Claude/settings.json` is what the agent loads. (2) A `CLAUDE.md` may import other
files (`@docs/agent-rules.md`), so a lane that edits only the imported file changes every
teammate's agent instructions and touches no listed path. (3) A symlink already in the tree
(`CLAUDE.md -> docs/agent.md`, or a `.claude` directory link) lets a lane change the target and
never the listed path. "Does not claim" names `Makefile` and `.vscode/tasks.json`. It does not
name these three, which are direct routes onto the design's own list (§8.3).
**How to fix:** Match case-insensitively (fold to lower case before the table). Treat any file
named by an `@` import in a changed-or-unchanged `CLAUDE.md`/`AGENTS.md` at the lane tip as
`instructions`. Treat a symlink (mode `120000` in `ls-tree`) whose path or target is agent config
as `agent-settings`. Add one test per case, each killing the matcher as written.

### S05-M15 (Minor): Loose ends: approver membership at land time, the Z5 rows, the id grammar, RLS
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-49.md:30-31, :163-164, :214, :227-232
**Problem:** (1) Line 30 says the approver must be a current member, but step 9a never checks
membership at land time, so an approval by a member who has since left still counts. (2) The part
says routes 84-86 "join the Z5 matrix" and gives no rows. Task 43's `allTeamRows` has no
`rowsT49`, so `TestEveryRouteHasAMatrixRow` fails on the new routes. (3) The id `CHECK` is
`^la-[0-9a-z]{16}$` (base 36) while task 04's tables use `^…-[0-9a-f]{16}$` (hex). (4) There is no
`FORCE ROW LEVEL SECURITY`, which task 43's table has. And `GRANT UPDATE (approver_user_id,
approved_at)` lets `hub_app` rewrite an approver after approval, which only Go refuses.
**How to fix:** Check approver membership in the handler's `Team` fill (route 55). Add the Z5
table and `rowsT49`. Use hex ids. Add `FORCE`. Add a guard trigger that allows one `NULL`-to-value
update only.

### S05-M16 (Minor): Three hours is short
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-49.md:3
**Problem:** A git classifier with six tests, a land step, three Hub routes with a migration and
events, a review builder and route, a Fleet diff route with a new `TreeSource` method, and a React
lens component with its test, across three branches.
**How to fix:** Split the Hub routes and migration (49a) from the converge, Fleet and lens work
(49b), or state five hours.

## Task 50: Phase 4 delta, the team pointer paragraph (213 lines, complete)

The input is one boolean, so no teammate text can arrive. The solo goldens are the proof. The
paragraph is pinned to the design file itself, and `TeamPointer` matches §5.3
(`05-design.md:584-588`) word for word. Checked on `.claude/worktrees/phase-4`: `render.go:27-28`
(markers), `:42` (`commentOpener`), `:76` (`Render`), `:82` (the anchor line); `render_test.go:51`,
`:76`, `:85`, `:98` (the helpers).

### S05-I16 (Important): `TestSoloRenderIsByteIdenticalToBefore` can never pass: every solo block contains "panacea"
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-50.md:117-119
**Problem:** The test fails when `Render(doc)` contains `"panacea"`. But phase 4's markers are
`BeginPrefix = "<!-- panacea:discover:begin"` and `EndMarker = "<!-- panacea:discover:end -->"`
(`render.go:27-28`, built), and every block carries both. So the test is red on correct code, and
the acceptance's `--- PASS: TestSoloRenderIsByteIdenticalToBefore` cannot be reached. The part's
own rule, "fix the renderer, never the golden", points a builder at the markers, and changing them
breaks every existing block's `Replace`.
**How to fix:** Assert that the solo block does not contain `TeamPointer` and does not contain
`` `panacea team` ``. This still kills "Team defaulted true" and leaves the markers alone.

### S05-M17 (Minor): Two channels carry the paragraph, and neither has a caller that knows the mode
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-50.md:199-213; task-41.md:145-148
**Problem:** Task 41 puts the paragraph in every cloud worker's managed `claudeMd` whatever the
mode (S05-I1). This task puts it in the repository block only when a caller passes `Team: true`,
and no caller exists (gap 2: the Materialiser is unbuilt). So a team cloud worker gets the
paragraph twice, a solo cloud worker gets it once when it should get it never, and a local
session gets it never. `render_test.go:100-130` is also cited for the helpers, which sit at
`:51-110`.
**How to fix:** Pick one channel for cloud workers (task 41's managed key, mode-aware per S05-I1)
and say that task 50's block is for the repository copy only, or the reverse. Name the phase 4 or
Phase T task that passes `Options.Team`. Fix the citation.

## Repair gaps 8 to 13 (repair-gaps.md), judged

| Gap | Verdict | Finding |
|---|---|---|
| 8, no owner for the mode-to-team write | real, open | S05-I9 |
| 9, task 45's phase 8 work missing from the file map | real, open | S05-M9 (3) |
| 10, `azapi` is a phase 8 owner call | real, needs the owner | S05-M9 (4) |
| 11, the collection endpoint is public | real, low risk (metadata only) | S05-M9 (5) |
| 12, task 44 builds a sweep the scope does not name | real, skeleton not amended | S05-M7 (5) |
| 13, what the fake records on a viewer downgrade | real, and wider: the fake has no downgrade at all | S05-M6 |

Gaps 1 to 7 fall in tasks 03 to 17 and belong to section 01. Gap 7 touches task 44, and task 44
reads the feed, not write refusals, so its only remaining piece is the untested ordering in
S05-M7 (4).

## Summary

Parts present and complete: 41, 42, 43, 44, 45, 46, 47, 48, 49, 50 (none cut off; 44 and 45 are
whole since the sign-out repair). Findings: 0 Critical, 16 Important (I1-I16), 17 Minor (M1-M17).
Most important for security: S05-I12 (a client-asserted re-proof lets a person who lost repo
access keep fetching through our credential), S05-I13 (a removed member keeps fetches for 24
hours), S05-I7 (a disabled leaver creates fresh delegations), S05-I10 (DNS open from AU pods),
S05-I8 (a forged re-send defeats A4), S05-I14 (the approval base is owner-chosen and never
checked).

Section verdict: Needs fixes

# Section 06 review: tasks 51 to 60 (Phase T, team mode)

Reviewer: adversarial plan reviewer, read-only except this file. Scope: the "## Task scopes"
entries 51 to 60 of `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md` against the parts
`docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-51.md` to `task-60.md`.

## Task 51: the prototype of every team surface

Coverage of scope 51 is complete: the strip (5 rows, zero state), click-in, the four Desk kinds,
the Orchestrator panel, Share, route badge in three forms, ratchet, data class at creation, spend
line, one clock, faces, OD-27, author class, and the owner gate before 53-56. Citations checked and
real: `05-design.md:939` ("Nothing here is drawn yet"), `:946-952` (the strip ASCII), `:992`
("~20 min"), `docs/AGENTIC-UX.md:144` and `:148`, `docs/DESIGN-SYSTEM.md:57-63` and `:70`,
`prototype/phase-4-discover.html:50` (`--beat:2400ms`), `prototype/work-layer.html:14-44`,
owner decisions `:164` (OD-27) and `:165` (OD-28).

### S06-I1 (Important): the OD-27 rule does not kill the mutant it names, and it skips the script
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-51.md:227-232 and :291
**Problem:** The mutant table says the "no percentage, bar, ETA or N of M" rule kills "HORIZON's
*~20 min* copied in". No regex in the rule matches `~20 min`: it tests `\d\s?%`, `<progress`,
`\bETA\b|\bremaining\b` and `\d+ of \d+`. Two more gaps. First, the percentage test runs on
`textOf(h)` only, which strips `<script>`. But every surface is built from `SAMPLE` by script, so a
`${Math.round(workersCents / quotaCents * 100)}%` spend line passes. Second, `\d+ of \d+` never
matches a template such as `${n} of ${m}`. The `--self-test` proves each rule can fail on a crafted
string, not on the mutant the table names. So the check reports `ok: 11 rules` on exactly the
drawings OD-27 bans.
**How to fix:** Add a duration-estimate pattern (`~\s?\d+\s?(min|h|s)\b`, `\bin \d+\s?min`) and run
the percentage and N-of-M tests on `textOf(h) + scriptOf(h)` too, with a script-side template
pattern (`\$\{[^}]+\}\s?%`, `\$\{[^}]+\} of \$\{[^}]+\}` minus the one `round` exemption). Give the
self-test the table's own mutants as bad cases: `~20 min` in a SAMPLE string, a `%` built in a
template, and a `${a} of ${b}` template.

### S06-M1 (Minor): nine states in one sitting is over the 1-3 hour bound
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-51.md:175-190, :299-334
**Problem:** One task draws nine states across five surfaces (strip, click-in, the Desk, the
Orchestrator panel and thread, Share/route/spend/create), and writes the check and the review
record. Steps 3 and 4 each cover several surfaces in one paragraph. That is more than 3 focused
hours, and a watchdog-killed run leaves a half-drawn file with no commit point.
**How to fix:** Commit after each step, and say so in the step. Or split the task into 51a (the
frame, clock and states 1-5) and 51b (states 6-9), with the owner reviewing once, after 51b.

## Task 52: team read models on the local control plane; the desktop client

Coverage of scope 52 is complete: strip, detail, the four Desk kinds, conversation, escalation
answers, notes, route raise, and the desktop mirror with `TextA`/`TextH`. The leak rule (no
teammate terminal, transcript or prompt) has a key allowlist and a planted-string test. Citations
checked and real: `client.ts:30`, `types.ts:3-5` and `:10`, `client.ts:245`, `api.go:341` and
`:379`, `api/poller.ts:30`, `tsconfig.json:12`, `designsystem.test.ts:9-11`, `store.ts:1-5`,
`hub/.../hubapi/keys.go:18` (`PrefixUser = "u"`), `task-01.md:347`, `task-12.md:272`/`:296-299`,
`task-37.md:484`. Off by two: the `types.ts:156` quote is at `:154`.

### S06-I2 (Important): DECIDED WITHOUT YOU checks for `fromAgentId` null, but task 29 writes `""`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-52.md:61-63, :443, :775-777, :1078
**Problem:** `isDecidedWithoutYou` is "kind inform, fromAgentId null, threadId set". Task 29 posts
`"fromAgentId": ""` (`task-29.md:55`, `:325`). Task 07 route 64 defines Fleet-for-itself as `""`
(`task-07.md:123-125`). Task 03's `CreateEscalationRequest` says the field is *absent*
(`task-03.md:749`), and `Escalation.FromAgentID` is a `*string` (`:774`). Nothing in the Hub parts
says `""` is stored or returned as `null`. If the Hub returns `""`, a pointer check sends every
DECIDED WITHOUT YOU item to SINCE YOU LAST LOOKED, with no Undo. OD-16's transparency surface
then disappears. `TestDecidedIsTypedNotTextual` uses a `null` fixture, so it passes while the real
wire fails. A related gap: a Fleet-raised decide has `fromAgentId` null, but ST.8 types its
`question` as `TextA` with a non-null `authorAgentId`. `labelFor` would then call Fleet's own
template "<owner>'s agent (agent)". So Fleet's product text has no author class.
**How to fix:** Treat "nil or empty" as Fleet-raised in `isDecidedWithoutYou`, and add a fixture
row with `"fromAgentId": ""`. Raise a seam note so tasks 03, 07 and 29 agree on one form. Give
Fleet-written text its own label ("Panacea (product)") and TS type, and type `DecideItem.question`
as `TextA | ProductText`.

### S06-I3 (Important): SINCE YOU LAST LOOKED loses every answer sent in a thread that has closed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-52.md:637, :794, :780-785
**Problem:** Scope 54 needs "each free-text `answer` your Orchestrator sent". `DeskInputs.Sent` is
"answers by the viewer's Orchestrator, one page per open thread". It is fed from Fleet's
`threads?party=me`, which lists `DeskThread`s that are open or escalated only (`:199-202`). An
ask/answer thread that closes before the human looks, the normal case against a cursor hours old,
drops its answers from the Desk for good. A thread with more than one page of messages also loses
every answer after the first page. The summary gets smallest exactly when the user was away
longest.
**How to fix:** Read answers from a record that outlives the thread. Either a Hub query of
messages by sender Orchestrator and `createdAt > since` across all of the project's threads (a
Hub delta; name it), or Fleet keeping closed threads listable for the cursor window. Page with
`after=` until the cursor, capped at a stated bound. Add a test: an answer in a thread closed
after `since` still appears.

### S06-I4 (Important): "3 focused hours" for four layers and about 30 tests
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-52.md:3
**Problem:** The part builds three new Fleet routes with a verb gate, a `fleetbe` client, about 20
ST.8 wire types, a pure assembly layer, eleven local routes (six of them writes), six golden
fixtures, a lifted `createTransport`, the TS team block, a client and a zustand slice. It names
about 30 tests across Go and Vitest. That is several days, not 1-3 hours. A watchdog-killed
implementer has no commit point short of the whole thing: Step 7 is the only commit.
**How to fix:** Split into four tasks, each with its own commit: 52a Fleet `teamhuman.go` +
`desk.go` + `fleetbe/team.go`; 52b the ST.8 types, `teamassemble.go` and the fixtures; 52c the
local routes and actions; 52d the desktop types, client and slice. Tasks 53-56 then depend on 52b
and 52d.

### S06-M2 (Minor): the task title ships now, but the part also says it stays null until G52-1
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-52.md:143, :179, :1176-1178, :1182-1186
**Problem:** The Hub returns `title` "only to Fleet·O" (`task-01.md:347`; `05-design.md:301`).
Step 2 fills `HumanAgent.Title` from Fleet's own `Card` and relays it to every viewer and
auditor. That makes Fleet the deputy that discloses a field the Hub withholds. The NOT-claims
say the opposite: until each gap closes, "the matching field is `null`". §9.1 does show the title
to humans, so this is an owner call either way. But the part cannot do both.
**How to fix:** Leave `Title` nil until the owner decides G52-1, behind one named switch that the
fixtures show both ways. Or state plainly that Fleet serves it, and move the G52-1 recommendation
into task 01 as a Hub delta.

### S06-M3 (Minor): small contract mismatches with task 51 and inside ST.8
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-52.md:294-296, :406-408, :458, :637, :772, :1075; task-51.md:83-101
**Problem:** (1) `TeamStrip.People` orders "the viewer last", but task 51's sample puts Dylan
second of three. (2) `TeamDesk` says "newest first within each list", while `SinceBlock` says
"oldest first". (3) The Desk route's read list omits Fleet `…/team`, but `DeskInputs.Human` needs
it to name `mine`. (4) `cardFor` hard-codes Australia/Sydney as "the viewer's clock". The
`04:40`-for-`18:40Z` test holds only under AEST: from 2026-10-04 (AEDT) the answer is `05:40`.
(5) `fleetbe/team.go` has four methods and no test.
**How to fix:** Reorder the sample. State one sort order per list. Add Fleet `…/team` to the Desk
row. Take the location from `time.Local` (the control plane runs on the viewer's machine) and pin
the test's date and zone explicitly. Add an `httptest` case per `fleetbe` method (path, verb,
error mapping).

## Task 53: the team strip and click-in

Coverage of scope 53 is complete: one band and nothing on the rail, 5 rows then "+N more", the
assembled line, the held-path ring, the three waiting phrasings from typed fields, the flash,
click-in with brief links, STATE/MEANING/NEXT, one action into your own panel, and no steering
control. The gate on task 51's approved record is explicit. Citations checked and real:
`designsystem.test.ts:180-190` (TRANSITIONABLE), `common.css:22-26`, `heartbeat.ts:13`/`:23-26`,
`AppShell.tsx:30-38` and `:92`, `tokens.css:67` (`--accent-tint`), `:118` (`--e-hair`, 1px ring),
`poller.ts:35-45`, `types.ts:196-201`, `task-09.md:105-107`, `05-design.md:954-957` (title class
"on hover"). Off by one: `--mono` is at `tokens.css:152`, not `:151` (`:151` is `--sans`).

### S06-I5 (Important): the one-clock test passes for the mutant it names
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-53.md:182
**Problem:** `every live dot shares the phase` mocks `performance.now` and asserts that every
`.dot-live` has the same `animation-delay`. Every row mounts in the same render at the same mocked
instant. So a mutant that drops `beatDelayStyle()` passes: each dot has an empty delay, the delays
are all equal, and each dot's phase comes from its own mount. A mutant that computes a delay from
a per-row mount time also passes. The CLAUDE.md rule "one global heartbeat clock" then has no
working test.
**How to fix:** Mount rows at different mocked times. Render three rows, advance
`performance.now` by 700 ms, then add a fourth row through a new strip. Assert that
`now_at_mount + delay` is equal modulo `BEAT_MS` for every dot, and that each delay is non-empty.

### S06-M4 (Minor): the timer-count test breaks on the testing library's own timers
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-53.md:181
**Problem:** The test asserts `setInterval` once and `setTimeout` zero times across a render and a
re-render. RTL's `waitFor`/`findBy*` schedule their own `setInterval` and `setTimeout`, and the
fetch-backed strip needs an await to land. So the count depends on how the test waits, not on the
component. Its mutant (`StatusDot`'s per-row `FLASH_MS` `setTimeout`, real at
`StatusDot.tsx:29-46`) is the right target, but the test will be flaky or loosened by hand.
**How to fix:** Seed the store with the fixture directly (no fetch), spy only across the
synchronous `render`/`rerender`, and filter the spy's calls to those from `src/components/team/`
(`new Error().stack`), or use `vi.useFakeTimers()` and assert `vi.getTimerCount()` is 1.

### S06-M5 (Minor): moving Commit Mono into the global `--mono` changes the terminal, and the part says it does not
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-53.md:48-52
**Problem:** Decision (c) edits the global `--mono`, which xterm reads (`tokens.css:148-150`). The
part claims "the terminal is unaffected". A different first face changes xterm's measured cell
width and glyphs for every session, not just the team strip. That is an unverified claim, and a
design-token change outside the scope of 53.
**How to fix:** Mark the terminal claim UNVERIFIED and add a manual check (a session renders
without column drift). Or scope Commit Mono to `code.team-path` and ids in `team.css` until phase W
or a design-system task moves the global token.

## Task 54: Desk rows

Coverage of scope 54 is complete: HORIZON kept through `accepted` with no time, NEEDS YOU with up
to 4 options, consequences, a clock-time deadline and the default, answered in place; SINCE YOU
LAST LOOKED batched per project with no toast; DECIDED WITHOUT YOU with Undo only where reversible;
nothing from a teammate by itself. Citations checked and real: W plan `:462` (task 62, SW.17),
`tasks-61-80.md:67-84` (the SW.17 shapes as quoted), `task-01.md:427-428` (one answer; defaulted
past deadline), `lib/time.ts:49` (`formatWaiting`), skeleton `:102` (OD-25 "Nothing here waits on
W") and `:372`. There is no Desk on this branch (`desktop/src/components/` has no `desk/`).

### S06-I6 (Important): leaving the Desk inside the 6-second window silently drops the answer
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-54.md:48-55, :189-190
**Problem:** Decision (c) holds the answer locally for 6 s, then sends it. Step 3 says the one
timer is "cleared on Undo or unmount". A user who picks "Let Alice go first" and switches lens,
closes the window or quits within 6 s has seen *"You chose: …"* and walked away. Nothing is sent,
and at the deadline the Hub applies the default. That can be the opposite of what the user chose.
For a consequential cross-owner decision that is a silent loss. No test covers unmount.
**How to fix:** On unmount (and on `beforeunload`) during the window, **send** the pending answer
at once instead of cancelling it. Better still, keep the pending answer and its timer in
`teamSlice`, so it survives the row unmounting. Add a test: choose, unmount at 2 s → `answer`
called once.

### S06-I7 (Important): TM1's Desk rows put phase W on team mode's critical path, against OD-25
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-54.md:3-6, :224-230; skeleton :102, :372
**Problem:** The part depends on W task 62's Desk frame (SW.17) and its unwritten `DeskSnapshot`.
It builds no frame. Without W-62 merged, "the Desk shows nothing of team mode". But OD-25 says
*"team mode's critical path is untouched by it"*, and the skeleton says *"Nothing here waits on
W"*. Task 57's criterion 2 needs NEEDS YOU and DECIDED WITHOUT YOU on two real desktops, so
TM1's proof now waits on W. G54-1 notes the conflict but leaves the skeleton and task 57 as they
are, so the plan's dependency graph is wrong.
**How to fix:** Settle it in the plan, not in a gap. Either (1) move a minimal SW.17 frame (the
registry, five sections, two bands, a snapshot on entry) into Phase T as its own task, which W-62
then extends, or (2) add `W-62` to task 54's and 57's dependsOn and put the OD-25 conflict to the
owner as a question. Keep the two-line registry double as the only test path until then, and say
that task 57 is blocked on it.

### S06-M6 (Minor): the `tabular-nums` test reads computed CSS that Vitest does not load
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-54.md:166
**Problem:** The test asserts that `font-variant-numeric: tabular-nums` is **computed** on the
deadline element. `desktop/vitest.config.ts:26` runs with `css: { include: [/\.css\?raw$/] }`, so
`teamDesk.css` is replaced with `""` in tests and `getComputedStyle` never sees it. The test can
pass only through an inline style, which the design-system audit discourages.
**How to fix:** Assert a class (`.num`, or the token layer's tabular class), and add one
`designsystem`-style `?raw` check that the class sets `tabular-nums` in `teamDesk.css`.

### S06-M7 (Minor): the "mark all seen" cursor is the local clock, compared with Hub `createdAt`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-54.md:57-60, :170
**Problem:** `markTeamSeen(generatedAt)` stores the local control plane's clock. Task 52 compares it
with Hub `CreatedAt > since`. If the desktop clock runs ahead of the Hub, an inform written in that
gap is before the cursor and is never shown. The deadline is also rendered in a hard-coded
Australia/Sydney zone (decision e), as in task 52.
**How to fix:** Move the cursor to the newest `at` among the items shown, which is a Hub time, not
`generatedAt`. Render deadlines in the viewer's local zone.






## Task 55: the Orchestrator panel

Coverage of scope 55 is complete: the conversation with verbatim plain-text replies, inline
`→ told <worker>` directives by kind from typed fields, the thread list as typed state, the author
class on every thread line, DIVE for the owner only, opened from the project view and ⌘K. The
no-agent-id rule for the local DIVE routes is the right defence against a crafted request, and Fleet
re-checks ownership. Citations checked and real: `Terminal.tsx:113`, `attachRegistry.ts:132`,
`palette/commands.ts:72`, `store/types.ts:13`, `designsystem.test.ts:181`, `task-40.md:324-330`
and `:363-378`, `task-21.md:24-31` and `:425-429`, `05-design.md:1001-1011`, and
`internal/proxy` (`proxy.go:28`, `Relay`). ST.8 types consumed (`ConvEntry`, `ThreadRow`,
`OrchestratorView.canDive`, `AskSubject`, `openOrchestrator`) match `task-52.md:906-950`, `:1017-1028`.

### S06-I8 (Important): the two new GET routes fail phase 2's route audit, and the attach route has no gate-order test
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-55.md:66-80, :108-122, :216-218, :249-253
**Problem:** `control-plane/internal/api/routes_audit_test.go:90-127` fails any `GET` pattern that is
on neither `catalogReadOnlyRoutes` nor `catalogUpgradeGatedRoutes`. Task 55 registers
`GET /v1/projects/{projectId}/orchestrator/dive` and `GET …/orchestrator/attach`, but its file list
does not include `routes_audit_test.go`. So Step 5's `go test ./internal/api/...` does not give the
`ok` the part expects. The attach route has side effects: it marks the owner attached in Fleet, which
stops task 38's nudges (`task-39.md:62-65`), and it opens a terminal. So it belongs in
`catalogUpgradeGatedRoutes`, and `routes_audit_test.go:65-69` says that entry *"costs"* an ordering
test like `TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed`. The part says the origin
and loopback checks "apply before any Fleet call" (:121-122), but no test pins that. A mutant that
dials `fleetbe.AttachAgent` before the origin check survives. The origin guard is the only defence a
browser WebSocket meets on loopback: WebSockets are not subject to CORS.
**How to fix:** Add `routes_audit_test.go` to Files and to the commit paths. Put `GET …/dive` in
`catalogReadOnlyRoutes`, with the reason that it relays Fleet's read. Put `GET …/attach` in
`catalogUpgradeGatedRoutes`. Add the test
`orchestrator.go › attach dials nothing until origin and upgrade pass`: a bad Origin gives 403, and a
non-upgrade request gives 426. In both cases the fake `DiveFleet` records zero `AttachAgent` calls
and zero `ownOrchestrator` lookups. Its mutant is that ordering reversed.

### S06-M8 (Minor): the DIVE pane never learns that the DIVE ended, or that the socket died
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-55.md:173-176, :36-43
**Problem:** `DivePane` "stays until the next dive() reads active: false", but nothing is said to call
`dive()` again after `endDive`. The panel's 3 s poll (decision c) reads `orchestrator(pid)`, not
`dive(pid)`. Also, the DIVE `Session` never enters the store, so `Terminal`'s
`updateAttachSession(session)` (`Terminal.tsx:102-104`) never runs for it. The registry therefore
never sees `status: "exited"` (`attachRegistry.ts:204-221`). When Fleet closes the socket (DIVE ended,
or the pod was killed, task 57 criterion 7), the pane runs the reconnect ladder to `failed` instead
of showing `ended`.
**How to fix:** Have `useOrchestrator` call `dive(pid)` on the same 3 s tick while a DIVE pane is
mounted. On `active: false`, unmount the `Terminal` and call `updateAttachSession({...diveSession,
status: "exited"})`. Add the test `DivePane › ends on active:false`, whose mutant is a pane that
outlives its DIVE.

### S06-M9 (Minor): DiveState on the wire is not "field for field", and one Fleet code is not relayed
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-55.md:113-114, :130-131
**Problem:** Task 40's `Since time.Time \`json:"since,omitempty"\`` (`task-40.md:328`) is never
omitted: `encoding/json`'s `omitempty` does not treat a zero struct as empty. So an inactive DIVE
serialises `"since":"0001-01-01T00:00:00Z"`, while the TS type says `since?: string`. Separately, the
relayed-codes list leaves out `DELETE`'s `404 when no DIVE is active` (`task-40.md:372`). The desktop
would then read that 404 as `orchestrator_not_found`.
**How to fix:** Ask task 40 for `*time.Time` or `omitzero` (Go 1.24+), or have the local route drop a
zero `since`. List the DELETE 404 with its own code (for example `dive_not_active`) among the relayed
codes, and give it a row in the test table.

### S06-M10 (Minor): an attach citation points at the upgrade, not the dial, and the sizing is tight
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-55.md:3, :119
**Problem:** "dialling a sandbox with a stored secret (`api/attach.go:127-140`)". Those lines are
`websocket.Accept` and `SetReadLimit`. The sandbox relay is `relayAttached` at `attach.go:193`. On
size: seven components, a CSS file, a `Terminal` change, a palette command, four Go routes,
`fleetbe/dive.go` and 16 tests (17 with S06-I8) is more than 3 focused hours.
**How to fix:** Cite `attach.go:193` (`relayAttached`). Split DIVE into its own part, 55b: the Go
routes, `dive.go`, `DivePane`, `diveSession` and the `Terminal` prop. That leaves 55 as the
conversation, threads, composer and palette.

## Task 56: route badge, Share line, spend line; the data class at creation

Coverage of the scope's visible items is complete: three badge forms, no switch on an AU-only class,
two raise wordings, lowering never drawn, the Share sentence verbatim, the spend line in words and
figures with `null` said, not zeroed, and a data-class field with no default. Citations checked and
real: phase 6 `task-22.md:1-32` (decisions a to c), `:336-341` (`formatUsd`), `phase-6-gateway.md:216`
(`Floor`) and `:244` (the 30 s TTL), `task-37.md:433-441` and `:456-459`, Hub `task-42.md:78-79`
(`shareProject`). The badge-reads-the-class rule (decision b) is right. The two findings below are
about **where** the class and the route are read and written.

### S06-C1 (Critical): on a team project the badge reads the laptop's `projects.json`, not the Hub, so it shows a false route and can store a conflicting class
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-56.md:25-27, :364-376, :381-383
**Problem:** The badge is fed `route={eff} dataClass={inf.dataClass}` from phase 6's
`getInference`. After a team raise, hunk 3 re-reads that same source. But phase 6 task 21's local
`GET`/`PATCH /v1/projects/{p}/inference` reads and writes **only** the local `projects.json`
(`phase-6 …/task-21.md:11`, `:34`, `:46`: *"accepts only `file:<path>` or unset. `hub` fails
start-up"*). Task 43's route 79 writes the **Hub** project record's route (`task-43.md:36-40`).
Three things follow:
1. After a successful team raise, `getInference` still answers `global`. The badge says **Global**
   and the raise button stays drawn. The test "the ratchet button is absent once AU is set" passes
   only because it is run on fakes.
2. A team project classified in the Hub (at share, or at creation) has no local entry. The badge
   says **Not classified**, and `ClassChooser` lets any member write a local class, for example
   `residency: "any"` on a project the Hub holds as `au-only`. The member's **local** sessions then
   route by that file (P6-D5 reads `projects.json`), so an AU-only team project can make Global calls
   from a laptop, which breaks task 57 criterion 6 and OD-19.
3. The part's own claim, *"they do not add a second source of truth for the route"* (:25-26), is
   false. It creates exactly that split, which decision (c) says §2.5 forbids.
**How to fix:** On a team project (`team !== null`), take `route` and `dataClass` from task 52's
`TeamSettings`, which is sourced from the Hub, and never from `getInference`. After a raise, re-read
`team(pid)`. Never draw `ClassChooser` on a team project; the class changes through the Hub's
project-record route (ask H-1). Record a blocking gap for phase 6: the local control plane must
refuse `PATCH …/inference` for a Hub-linked project, and its route source must follow the Hub for
linked projects (seam S6.1). Until that lands, task 57 criterion 6 cannot be claimed for local
sessions. Add the test `a team project's badge ignores the local file`: `getInference` says
`global`, `TeamSettings.route.current` says `au`, and the label is `AU` with no raise button. Its
mutant is the badge reading phase 6's state on a team project.

### S06-I9 (Important): the class is asked for at Share and stored on the team policy, against OD-24 and task 43
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-56.md:57-69, :385-392, :430-432
**Problem:** Decision (e.1) makes the first Share a creation moment: `DataClassField` sits in the
Share popover, and the class is sent with the share, *"task 43 stores it on the team policy"*.
OD-24's resolution says the opposite: *"The data classification is set when a project is CREATED,
**not at Share**"* (`owner-decisions.md:152`). Task 43 also does not store a class on the policy. The
class is read through from the project record (`task-43.md:36-39`), and
`TestDataClassIsNotWritableHere` (`task-43.md:447`) returns 400 for any policy write that carries
`dataClass`. Gap 2 asks task 43 for a write that task 43 tests against.
**How to fix:** Drop `DataClassField` from the Share popover. Share reads the project's existing
class. When there is none, **Share** is disabled with the sentence *"Choose a data class first"*
and a link to the badge's **Choose a data class**, which is the phase 6 path and, per S06-C1, the
Hub record once the project is linked. Do not change `shareProject`'s signature. Replace gap 2 with
an ask to phase 4 (P4-1) and ask H-1: the class is written onto the project record at creation.

### S06-M11 (Minor): stale sibling claims and two citation drifts
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-56.md:105-108, :149-151, :410-411, :428-429
**Problem:** Task 52 is described as *"not yet written; required of task 52"*, and gap 1 asks it for
`TeamSettings`, `selectTeamSettings` and `raiseTeamRoute`. Task 52 is written and already has all
three (`task-52.md:103`, `:942-948`, `:984`), with `dataClass` as the `{level, residency}` object.
`task-51.md:165` already shows `dataClass` as the object, not the bare `"internal"` the part quotes.
Visual acceptance cites *"state `9` (`task-51.md:189`)"*, but `:189` is state `8`. Also,
`raiseTeamRoute` lives on task 52's `TeamClient`, and hunk 3 calls `api.raiseTeamRoute`. Nothing says
the `api` singleton gains it.
**How to fix:** Delete gap 1 and the "not yet written" note, and correct the two citations. Call
`team.raiseTeamRoute` through task 52's client, or add the one line that re-exports it on `api`.

### S06-M12 (Minor): one test names no mutant, and the ratchet announcement is untested
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-56.md:186, :44-51
**Problem:** `SpendLine › words and figures` has *"—"* in its mutant column, against the house style.
The scope says the ratchet is *"a one-way ratchet the strip announces"*. No test here, and none this
part names in task 53, shows that the strip line appears after `route.raisedAt` is set.
**How to fix:** Give the test the mutant "cents printed as dollars (`US$1,840`)", which the
`/ 100` kills. Add a row, or a named dependency on a task 53 test, for "`raisedAt` set → the strip
shows *AU for everyone, raised by <name>*".

## Task 57: TM1 proof

Coverage of scope 57 is complete: all eight §12.1 criteria as computed rows, C6 with the hook on and
off, the five gate rows, and the 3-day G9 contingency as a budget, not a retry loop. Decision (d),
that nothing passes on absence, is the right discipline, and the verdicts are tested on fixtures.
Citations checked and real: `task-29.md:296` (the `DECIDED WITHOUT YOU:` Sprintf), the skeleton's
`:882` (G9 contingency), `task-47.md:299-302` (the collector/verdict split), `task-42.md:241`
(`OutOfScope`), and `phase-6-gateway.md:242` (the gatekeeper's `slog` fields).

### S06-I10 (Important): criterion 2 tells Fleet's items apart by `fromAgentId == nil`, but Fleet writes `""`
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:25, :339-345, :568
**Problem:** `c2` treats an escalation as Fleet-raised only when `x.FromAgentID == nil`. Task 07's
route 64 defines Fleet-for-itself as `fromAgentId: ""` (`task-07.md:125`, `:128`), and task 29
writes `"fromAgentId": ""` (`task-29.md:325`, test `:383`). The read type is `*string`
(`task-03.md:774`), so unless the Hub turns `""` into SQL NULL, which nothing says it does, every
DECIDED WITHOUT YOU item decodes as a non-nil pointer to `""`. It then falls into the
`informTo` arm (:342). A correct run fails with *"DECIDED WITHOUT YOU did not reach both people"*.
This is the same seam as S06-I2, now inside the proof. `TestC2AgentWrittenDecidedIgnored` is the
defence against an agent forging Fleet's line, and it rests on this distinction, so the distinction
has to be the one the Hub really stores. `c2` also never checks that the inform and decide belong to
the settled conflict's thread, so escalations from another thread satisfy it.
**How to fix:** Settle the seam once, in Hub task 07: store NULL for Fleet and return
`"fromAgentId": null`, and have task 29 send it absent. Then have `c2` and `c4` call one helper,
`fleetRaised(x) = x.FromAgentID == nil || *x.FromAgentID == ""`, until that is merged. Add
`TestC2EmptyStringIsFleet`, whose mutant is the nil-only check. Require `x.ThreadID` to equal the
settled conflict's thread for the inform and the decide.

### S06-I11 (Important): three of the five first-teammate gate rows are strings that nothing computes
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:38-42, :174, :497-501, :604
**Problem:** The part says *"every verdict is computed from records"* (:11-13). Yet `G-Z5`,
`G-attach` and `G-sentinel` are read from `probes.gate`, and the collector's `probes` step "merges
… the gate map" without saying who writes it. Only `G-mirror` (a `git log` count) and `G-infosec`
(typed by the owner, and declared as such) have a stated producer. That makes the security gate
self-reported: owner isolation (Z5), the attach tests (task 39), and the audit trail (Sentinel). A
hand-typed `"pass"` invites a real teammate onto an unproven build. Also, `G-sentinel` depends on
task 45, and `task-45.md` is **absent** from the parts directory. The skeleton has the scope at
`:906`, but there is no part.
**How to fix:** Have the collector's `probes` step produce each row:
- `G-Z5` and `G-attach`: `go test -json -run '<the named tests>'` for task 43's matrix and task 39's
  suite, at the deployed commit. Check that commit against the running image's revision label, and
  write `pass` only when every named test has `"Action":"pass"`. The sha is the evidence.
- `G-sentinel`: a Sentinel query that finds this run's `dv-…` and `es-…` audit ids, with the query
  id as evidence.
Add `TestGateRowWithoutEvidenceFails`. Flag the missing task 45 part to the skeleton owner.

### S06-I12 (Important): the copied-token probe has no script, and both citations for it are the wrong probe
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:29, :474-476, :604, :633-634
**Problem:** Criterion 6's second half, *"a token copied off its pod is refused"*, is read from
`probes.copiedToken`. The table cites *"task 47's P28"*, which is the AU egress allowlist
(`task-47.md:311`). The prose and the run script cite *"task 47's P25 … with a token read out of the
pod and replayed"*, but P25 is the check that the source address equals the pod IP (`task-47.md:308`).
It replays nothing. So no script produces `copiedToken`, and criterion 6 can never leave
`NOT RUN`. The probe also handles a live session token, and no part states how to handle it
safely.
**How to fix:** Define the probe here as `infra/smoke/team/tm1-copied-token.sh`:
1. Read the AU worker's gateway token from its pod.
2. Call the gatekeeper once with it from the collector's own pod.
3. Record only `{status, code, session}`.
4. End that session at once, so the token dies.
Never write the token to `$OUT` or to a log. Correct both citations. Its verdict test already exists
(`TestC6CopiedTokenWrongCodeFails`).

### S06-M13 (Minor): four verdict loopholes a fixture would not catch
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:280, :356, :449-467, :492-493
**Problem:**
1. `ts()` drops parse errors. A conflict with an empty or unparsable `createdAt` becomes year 1,
   "precedes" every worker, and `c1` passes.
2. `c7` compares **counts**, so a lease lost and another created in the same window passes. The
   other person's Orchestrator is live during the kill, so that can happen.
3. `c8` lets an `origin: "human"` directive with an empty `read` pass, which does not answer
   *"who asked for this?"*.
4. `c2`'s evidence joins ids by string concatenation (`informTo[a]+informTo[b]`), which gives
   `es-1es-2`, and `TestEvidenceHoldsNoText`'s regex then rejects the pass fixture.
**How to fix:**
1. `ts` returns an error, and a bad time is `FAIL (unreadable time)`.
2. `PodKill` carries id sets, and `c7` compares the sets.
3. A human-origin directive must read at least one id in `human`.
4. Append the ids separately.
Each change gets one test named for its mutant.

### S06-M14 (Minor): export scope, where the collector runs, and size
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-57.md:3, :602-603, :621-624, :635-636
**Problem:**
- The export is not said to be filtered to `ProjectID`, while the AU project and G9's third project
  run on the same Fleet. `c1`'s "first conflict" could then be one of G9's.
- `kubectl delete pod` and `kubectl logs` from the operator's laptop are **UNVERIFIED** on a KPMG
  device. Owner memory for Panacea records that kubectl is denied locally and has to be run through
  Cloud Shell.
- Step 1 picks the data class at Share, which S06-I9 removes.
- Three hours for `export.go`, `tm1.go` (8 rows, C6 and the gate), `teamproof`, 24 tests, a
  four-command collector, the script and the record is about double.
**How to fix:**
- Filter every Hub export by `projectId`, and pass the AU project's gatekeeper lines to `c6` alone.
- Name the collector's host: Cloud Shell, or the ADO pipeline, which owner memory prefers.
- Choose the class at the badge before sharing.
- Split into 57a (the `proof` package, `teamproof` and tests) and 57b (the collector, the script
  and the record).

## Task 58: phase 8 delta, the lane mirror's Kata pod

Coverage of scope 58 is complete: the `mirror` class on task 15's `Spec.Class`, one Kata pod per
project, its own project disk, git pinned in a digest-pinned image, a separate namespace so no sandbox
can mount the disk, deletion with the project, and prune on leave marked as Q7's recommendation.
Decision (b) goes further than the scope: no fetch credential at all. It is argued well and listed as
gap 1. The pod shape keeps the builder's `AutomountServiceAccountToken: false` and `RuntimeClassName`
(`env …/manager.go:288-294`). Citations checked and real on `feat/env`: `manager.go:66`,
`:141-144`, `:151-159`, `:352-363`, `:426`; phase 8 `task-11.md:92-99`, `:137-164`, `:77`
(`Premium_ZRS`), `:167-169` (`reclaimPolicy: Delete`, `allowVolumeExpansion`) and `:173`;
`task-13.md:69-70` and `:465-473`; `task-15.md:158-160`.

### S06-I13 (Important): "no egress" rests on a default-deny nothing checks, and phase 8's netpolcover rule fails the mirror
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-58.md:20-25, :347-351, :367, :387-390, :438-451
**Problem:** Two separate faults.
1. `mirror-ingress-from-broker` sets `policyTypes: [Ingress]`, so egress is denied only by the
   namespace's `default-deny`. The part assumes that policy is there (*"already loops over the
   namespaces … if it lists them by hand, the fifth joins the list"*), and no test checks it. If
   `default-deny` renders in four namespaces only, the mirror has **unrestricted** egress: a
   NetworkPolicy that lists only Ingress restricts nothing outbound. Every test listed would still
   pass, and only task 69's live probe would catch it. Decision (b)'s whole case, that a compromised
   mirror has no way out, depends on this.
2. Phase 8's `netpolcover` *"fails if any workload in a denied namespace lacks an allow in a
   direction it needs (egress always; …)"* (`phase-8 …/task-11.md:147-151`). The mirror has no egress
   allow by design, so Step 4's *"every suite passes"* cannot happen without an exemption, and the
   part names none.
There is also a third gap. `EnsureMirror` checks the owner view but not the namespace. A broker
whose mirror manager is configured on `panacea-sandboxes` would place mirrors under
`egress-internet` (`task-11.md:155-159`).
**How to fix:**
- Add the helm-unittest `default-deny renders in panacea-mirrors`: `podSelector: {}`,
  `policyTypes: [Ingress, Egress]`, no rules.
- In `netpolcover`, replace the case with a positive rule: every pod matching `component: mirror` is
  selected by at least one policy with `Egress` in `policyTypes` and zero egress rules. Add a named
  exemption from "egress always" for exactly that selector.
- Add `const MirrorNamespace = "panacea-mirrors"`. `EnsureMirror` refuses any other
  `m.cfg.Namespace`, tested by `TestEnsureMirrorRefusesTheSandboxNamespace`.

### S06-M15 (Minor): the mirror's `/control` is not task 17's relay, whose token Fleet supplies
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-58.md:318, :336-340
**Problem:** The relay is called *"task 17's relay, `task-17.md:423`"*. But that relay needs Fleet's
`Panacea-Control-Token` header (`task-17.md:340`, 400 without it). Here the broker mints the
mirror's token and *"never returns"* it, so Fleet cannot send it. Nothing says whether the broker
injects its own, or what it does with a header a client does send. The in-memory copy also diverges
across broker replicas, and the part's fallback, reading the token from the Secret after a
restart, is not stated as the only source.
**How to fix:** State the mirror relay's rule:
1. A client-supplied `Panacea-Control-Token` is refused with 400.
2. The broker adds the token itself, read from the attach Secret's `control` key, with at most a
   short cache.
Add `TestMirrorControlInjectsTheToken`, which kills the mutant that forwards the caller's header.

## Task 59: phase 3 delta, the forecast as a library and the mirror runner

Coverage of scope 59 is complete. The forecast becomes a library, `gitforecast`, and `converge` keeps
its names as aliases. `bundle verify` runs before the fetch, the fetch uses `transfer.fsckObjects`,
lanes land at `refs/panacea/lanes/<owner-hash>/<wt-id>`, and `merge-tree --write-tree` runs only for
the pairs asked. ST.7 is typed: fields are unknown-refused, paths are validated, and a bad answer is
dropped whole. Nothing is pushed. The scope's three tests are present
(`TestUnverifiedBundleIsDroppedAndReported`, `TestIngestRefusesACorruptObject`,
`TestResultCarriesNoContent`), and `TestPairIgnoresDiffDrivers` is a strong addition. Moving the
runner to `control-plane/cmd/panacea-mirror` is argued and listed as gap 1. Checked:
`task-17.md:336-346`, the relay order. Phase 3's `task-02.md` and `task-24.md` line citations are in
the phase 3 worktree and **UNVERIFIED** here; the part marks them *until merge*.

### S06-I14 (Important): the project's base is one force-updated ref that any member's machined can replace
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-59.md:48-52, :430-433
**Problem:** `ingest` of kind `base` uses `Refspecs = ["+" + head + ":refs/panacea/base/<branch>"]`.
That is one shared ref, force-updated, last sender wins. The part itself says a base bundle is
*"as hostile as a lane's (a member's clone is agent-writable, phase 3's R4.2)"*, yet nothing checks
that a new base descends from the old one, or matches the remote's real tip. Tier M's merge base is
computed from the lane tips, so the forecast is safe. But task 61 builds the **integration head** in
the mirror and hands it to a lane owner, who pushes it **with their own credential**. A base chosen
by an agent on Alice's side then rides in Bob's push. That is the trusted-deputy path the design's
§8.3 guards against.
**How to fix:**
- Namespace the base per sender: `refs/panacea/base/<owner-hash>/<branch>`.
- Drop the `+`, so a base is fast-forward only (`CodeBaseNotFF` when it is not).
- Record in ST.7 that task 61 builds only on the landing owner's **own** base ref.
- Add tests: `TestBaseFromBNeverMovesA`, and `TestNonFastForwardBaseRefused`, whose mutant is the
  `+` refspec.

### S06-I15 (Important): concurrent requests race `gc --prune=now`, and every pair forecast grows the disk forever
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-59.md:396-397, :419-421, :434-440
**Problem:** *"One upgrade carries one request"*, but nothing serialises requests across sockets.
Several members' ingests and Fleet's `pairs` arrive together. `prune-owner` runs
`git gc --prune=now`, which git documents as unsafe beside a concurrent writer. An in-flight
`fetch`'s new objects, or `merge-tree --write-tree`'s, can be deleted before a ref reaches them,
leaving a lane ref that points at missing objects. Separately, `merge-tree --write-tree` **writes**
merged trees and blobs, conflict-marker blobs included, into the object store on every pair. With
`gc.auto=0` those objects are never collected except on a leaver's prune, so a 32 GiB disk fills in
proportion to activity. Task 58 says this is to be measured in task 69, but nothing bounds it.
**How to fix:**
- Put a runner-wide read/write lock in `serve.go`: `ingest` and `prune-owner` exclusive, `pairs`
  shared.
- Run each `PairForecast` against a throwaway object directory: `GIT_OBJECT_DIRECTORY=<tmp>`, with
  the repository's objects as `GIT_ALTERNATE_OBJECT_DIRECTORIES`. Delete it after the answer.
  Whether §HGH's `Call.Env` admits these two variables is **UNVERIFIED**. If it does not, keep
  `gc.auto=0` and run `gc --prune=1.hour.ago` under the exclusive lock after N pairs.
- Add tests: `TestPairLeavesNoObjects` (object count unchanged) and
  `TestPruneWaitsForIngest`.

### S06-M16 (Minor): validated path strings are still a channel from a compromised mirror
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-59.md:336-381
**Problem:** `Validate` checks each path's shape (repo-relative, ≤ 200 characters, hygiene), not that
it exists. A mirror compromised through a hostile lane can return up to 500 invented "paths" built
from another owner's lane bytes. Fleet writes them into `cf-…` records, which teammates' Desks and
Orchestrators read. That is a cross-owner content channel, and a prompt-injection channel, that
`TestValidateRefusesAPathWithANewline` does not close.
**How to fix:** Fleet keeps each lane's changed-path set from the **sender's** own report: task 60's
`machined` sends `Changed` beside the bundle as typed metadata. Fleet drops any `PathResult` not in
both lanes' sets, and the drop gets a test.

### S06-M17 (Minor): the runner's frame limit and bearer depend on facts other parts do not state
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-59.md:395-398, :411-412
**Problem:** *"read limit 64 KiB for text frames"*. `coder/websocket`'s `SetReadLimit` covers every
message, binary included, so task 60's sender must chunk bundles into frames of 64 KiB or less, and
neither part says so. The bearer *"the broker's relay adds (task 17, `task-17.md:342`)"* is the
caller's `Panacea-Control-Token` in task 17 (`:340`, `:342`). Task 58 never gives that token to
Fleet (S06-M15).
**How to fix:** Fix `MaxFrameBytes = 64 << 10` in ST.7, and have task 60's sender use it. Point the
bearer at task 58's injected-token rule once S06-M15 is settled.

## Task 60: tier-M forecasts; lane bundles into the mirror

Coverage of scope 60 is complete:
- Lanes cross as bundles through the broker.
- Tier M is asked only for flagged pairs that include the committing lane (`k`, not `n²`).
- `cf-…` records are raised or moved to `M`.
- The scope's two tests are present (`TestAnUnflaggedPairNeverReachesMergeTree`,
  `TestACleanMergeOnAPFlaggedPairIsStillAConflict`).

The owner isolation is right. `lane.owner` must equal `OwnerHash` of the verified delegation's owner,
and `machine` must be that owner's (403, never 404). `TestBundleRouteRefusesAnotherOwnersLaneKey`
kills the mutant. Fleet never holds bundle bytes (decision e).

### S06-I16 (Important): the last commit of a burst is never forecast at tier M
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-60.md:61-63, :163-166, :245-247, :356-361, :413
**Problem:** Decision (f) coalesces a burst. The first `Send` goes out. Later commits set *pending*,
and the Sender *"sends once more when the window closes"*. But `OnCommit` has already returned on
`tip == ""` (:164-166), and nothing asks `Pairs` after that deferred send completes. `applyM` then
drops any earlier answer as stale, because the recorded tip has moved (:245). So after every burst,
which the part says is how agents commit, the lane has no current tier-M answer until its **next**
commit. `TestBurstIsCoalesced` counts relay calls. It never checks that pairs are asked for the
newest tip.
**How to fix:**
- Give `Sender` a hook, `OnIngested func(ctx, org, project string, lane LaneKey, tip string)`.
- Split `OnCommit`: it only calls `Send`, and the pair-asking (`flaggedWith`, `Pairs`, `applyM`)
  runs from the hook for every ingest, deferred sends included.
- Add `TestTheBurstsLastTipIsAsked`: five commits in 10 s, then `Pairs` is asked once more with
  `basis.a` equal to the fifth tip. Its mutant is the inline ask.

### S06-I17 (Important): task 35 is written, and has neither `Activity.Paths` nor the `OnCommit` call this part needs from it
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-60.md:3, :101-105, :234-236, :433-434, :460-461
**Problem:**
- The part calls tasks 34 and 35 *"not yet written"*. Both parts exist.
- Task 34 already carries `LastCommit` (`task-34.md:135`).
- Task 35 is written, and it has no `Paths` method and no `OnCommit` call. A grep of `task-35.md`
  finds neither.
- This part adds `Paths` to task 36's `Activity` interface (`task-36.md:128-130`) by a hunk in
  `tierh.go`, and says *"task 35 implements both"*. Without that implementation, task 35's `Table`
  no longer satisfies the interface, so Step 5's `go build ./cmd/fleetd` fails, and nothing in
  production ever calls `OnCommit`.
- Tier M's tests pass only against fakes.
**How to fix:** Own the two hunks here:
1. `control-plane/internal/fleet/activity/activity.go` gains `(*Table).Paths(ctx, project)
   []AgentPaths`: live agents only, from the 45 s table.
2. Task 35's sample fold calls `OnCommit` once per new `lastCommit.sha`.
Add both files to Files and to the commit. Add tests for both: `TestPathsListsLiveAgentsOnly`, and
`TestOnCommitOncePerSha`, whose mutant fires on every sample. Update the note on task 34.

### S06-M18 (Minor): the base inherits S06-I14, and a clean overlap is raised under the plan tier
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-60.md:69-72, :371-381, :270-280
**Problem:** A `need-base` triggers a `base` bundle under whichever member's delegation hit it, and
it lands on the project's one shared, force-updated base ref (S06-I14). Separately, a `clean` answer
with shared paths and no open conflict raises a `cf-…` at tier `P`. The Desk and timeline then show
commit-derived evidence under the plan-time tier. That is sound for task 30's `decide` rule, but it is
not what the record says.
**How to fix:** Send and record the base per owner, as the S06-I14 fix does, with `mirror_tips` rows
`lane_owner = 'base-<owner-hash>'`. Give the P record a `source: "m-clean"` note in the working
copy, or say in the non-claims that the `P` tier stands in for "overlap, merges cleanly".

## Section summary

| Part | Critical | Important | Minor |
|---|---|---|---|
| 51 | 0 | 1 (I1) | 1 (M1) |
| 52 | 0 | 3 (I2-I4) | 2 (M2-M3) |
| 53 | 0 | 1 (I5) | 2 (M4-M5) |
| 54 | 0 | 2 (I6-I7) | 2 (M6-M7) |
| 55 | 0 | 1 (I8) | 3 (M8-M10) |
| 56 | 1 (C1) | 2 (I9, I18) | 2 (M11-M12) |
| 57 | 0 | 3 (I10-I12) | 2 (M13-M14) |
| 58 | 0 | 1 (I13) | 1 (M15) |
| 59 | 0 | 2 (I14-I15) | 2 (M16-M17) |
| 60 | 0 | 2 (I16-I17) | 1 (M18) |
| **Total** | **1** | **18** | **18** |

No part file is missing or cut off: `task-51.md` to `task-60.md` all end in their gaps or
non-claims sections. Outside this range, `task-45.md` (Sentinel) was missing when this
review was written; it landed after it (commit `3ede8ae`), and the addendum below re-checks the
two findings that touch it (S06-I11, new S06-I18). Four seams cross several parts. Fix them
once, in their owners:
1. `fromAgentId` `""` versus `null`: S06-I2, S06-I10.
2. The data class and route sources on a team project: S06-C1, S06-I9.
3. The mirror's control token and relay: S06-M15, S06-M17.
4. The shared base ref: S06-I14, S06-M18.

## Addendum: re-check after task 45 landed

`task-45.md` was written at 19:09, after this review (18:54). All parts 51 to 60 and the skeleton
are older than this review, so the findings above stand. Task 17's cited lines (`:340`, `:423`) still
hold after its 20:31 edit. Two findings touch task 45:

- **S06-I11 still stands, and now has a producer to use.** Task 45 ships
  `Logs.EventsOf(ctx, orgID, since)` (`task-45.md:279`). Task 57's `probes` step should call it for
  the run's window and write `G-sentinel = pass` only when this run's `dv-…` and `es-…` ids come
  back. The row stays hand-typed until task 57 says so.
- **New, S06-I18, below.** Task 45's gate needs a caller, and the Share act in task 56 is that
  caller.

### S06-I18 (Important): the Share act never passes task 45's audit gate, so "audit streams before the first teammate" is not enforced
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-56.md:61-63, :385-389, :198;
task-45.md:57-61, :386-391, :696-698; task-06.md:718-720
**Problem:** Task 45 enforces the Sentinel gate only through `Server.TeamModeAllowed()`: 503
`unavailable`, `details.reason = "audit_not_streaming"`. Only "the write that sets a project's
`mode` to `team`" calls it. Task 01 says that write is "set by Share" (`task-06.md:718-720`). Task
56 owns the Share popover's hunks. They add `shareProject(projectId, teamId, { dataClass })` and
nothing else: no `mode` write, no gate call, and no popover state for the 503. So on an estate where
`PANACEA_APP_HUB_AUDIT_SINK` is `off`, the first share still creates a team project. At best the
refusal shows up as a generic error with no reason. Task 45's gap 1 also cites the task 06 gap at
`task-06.md:699-701`, but it sits at `:718-720`.
**How to fix:**
1. Name one owner for the share route's `mode = team` write. Task 43 already stores the class
   there. It calls `TeamModeAllowed()` first, inside the same transaction.
2. In task 56, give `SharePopover` a `refused` state that shows the Hub's message verbatim when
   `reason` is `audit_not_streaming`, and keeps **Share** disabled.
3. Add the tests `the share shows the audit refusal` (it kills the mutant that treats 503 as a retry
   or a generic toast) and Go `TestShareRefusedWhileAuditNotStreaming` (it kills the mutant that sets
   `mode` before the check).
4. Correct task 45's citation to `task-06.md:718-720`.

Section verdict: Needs fixes

# Plan review, section 07: Phase T tasks 61 to 69

Reviewer: adversarial plan reviewer, read-only except this file. Scopes from the skeleton
`docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:1061-1121`.

## Task 61: The team land queue over the mirror

### S07-C1 (Critical): The land queue needs a mirror that fetches the remote; task 58 built one that cannot
**Where:** docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-61.md:25-27, :200-205, :371, :381-384
**Problem:** Property 4, step 4 and step 9 all rest on `Mirror.FetchRemote`: "fetches <branch> from the customer's remote with the project's read-only fetch credential (task 48)". Task 58 decision (b) (`task-58.md:27-38`) says the opposite and gives the reason: the mirror pod has **no egress and no credential, not even the fetch credential**, because an FQDN allow to the git host would let a mirror compromised by a hostile lane push every member's lanes to an attacker's repository. The base arrives as a `need-base` bundle from a member's `machined` (`task-59.md:430-433`, ingest kind `base`). So `FetchRemote` cannot exist without undoing task 58's main security decision. Property 4 also breaks. The only reader of the remote is then a member's `machined`. If that is the lane owner's own machine, the check "the remote shows it" is attested by the same party whose `Report{pushed}` it is meant to check.
**How to fix:** Drop `FetchRemote` from `Mirror`. Base the land on task 59's `refs/panacea/base/<branch>` and its `ingest` kind `base`. Verify the push through a machine that is not the lane owner's: another current member's `machined` fetching with the fetch credential, or the broker itself, which holds the fetch credential under global constraint 8 (`2026-09-25-phase-t-team-mode.md:151-153`). That machine reports the remote head as a typed sha. Rewrite property 4 and `TestLandedOnlyWhenTheRemoteShowsIt` so that no owner-controlled path can move an entry to `landed`. Also add one sentence saying task 58's no-egress rule still holds.

### S07-I1 (Important): The mirror path, the ref names and the bundle cap disagree with task 59
**Where:** task-61.md:93-94, :36, :191, :514, :524
**Problem:** Task 61 extends `sandboxd/cmd/panacea-mirror/main.go` "(task 59)", runs `go test ./cmd/panacea-mirror/...` in `sandboxd`, and commits the pathspec `sandboxd/cmd/panacea-mirror`. Task 59 moved the runner to `control-plane/cmd/panacea-mirror/{main.go,serve.go,repo.go}` (`task-59.md:23-28`, `:72`). So the file does not exist, the test command matches nothing and the commit pathspec fails. Task 61 also names the base ref `refs/panacea/integration/<branch>`, where task 59 writes `refs/panacea/base/<branch>` (`task-59.md:430-431`). And it defines its own `MaxBundle = 256 << 20`, marked "UNVERIFIED until 59 is written", but task 59 is written and uses phase 3's `MaxBundleBytes` (`task-59.md:87`, `:407`).
**How to fix:** Point at `control-plane/cmd/panacea-mirror/*` and task 59's package, and run the mirror tests inside `control-plane`. Use `refs/panacea/base/<branch>` for the remote head. Reference `MaxBundleBytes` rather than a second constant.

### S07-I2 (Important): The mirror wire for the three new ops is missing
**Where:** task-61.md:93-94, :200-210
**Problem:** Task 59's runner serves exactly `GET /control`, where one WebSocket carries one typed `Request` with `op` one of `ingest | pairs | prune-owner` (`task-59.md:392-410`). Task 61 adds `land`, `export` and `fetch-remote` but gives only the Go interface on Fleet's side. It does not give the `Request` fields for a `LandBuild`, the answer frames, or how a bundle travels from the mirror to Fleet. Task 59 only defines inbound binary frames. The house style requires complete wire shapes, and this is the channel that carries every member's objects out of the mirror.
**How to fix:** Add a fenced block extending task 59's `Request` and answer types for `land` and `export`: the new `op` values, a `Land *LandBuild` field, and an outbound binary-frame rule with `Size` and the byte cap. Remove `fetch-remote` per S07-C1. Add goldens in task 59's style.

### S07-I3 (Important): The owner pushes a merge that the mirror built and reported, and nobody re-checks it
**Where:** task-61.md:20-21, :70-75, :346-353, :422-428
**Problem:** Two things come from the mirror unchecked: the gate's input (`LandBuilt.Changes`, `PackageChanged`) and the merge object the owner pushes. Neither is re-derived on a trusted side. The mirror is the one process built to parse hostile lane objects (§8.2; `task-58.md:31-33`), so its answers are untrusted in the same way phase 8 treats a runner's answer (`phase-8 ...own-environment.md:138`: "parsed as untrusted data"). Suppose a teammate's lane compromises the mirror. The mirror can then (1) hide a `.claude/settings.json` change from `Gate` and skip the second approval, and (2) stage a merge whose tree is not `merge-tree(base, laneTip)`. Alice would then push it to `main` under her own credential, which makes this a cross-owner trusted-deputy escalation. The push act in `teamland.go` checks none of it.
**How to fix:** Before the push, have the owner's `machined` run project git under §HGH (phase 8 task 21), and refuse the push unless all of these hold: (a) the staged merge's parents are exactly `[base, laneTip]` and `laneTip` is the reviewed tip; (b) its tree equals a local `merge-tree --write-tree base laneTip`; (c) the agent-config path list computed locally equals the one `Gate` approved. Add a test for each check, with the mutants "a staged merge with a third parent", "a tree differing from merge-tree" and "a mirror path list missing `.claude/settings.json`".

### S07-I4 (Important): The push token's path to the cloud machine has no wire and no task
**Where:** task-61.md:422-431, :545-548
**Problem:** The whole push act depends on "one header on the broker's machine RPC", but the part defines no header name, no machined route, no rule against logging it and no test. It calls this "not in any sibling plan" (gap 1). The brief makes phase 8 deltas this phase's own tasks, so leaving it for "that plan to add" leaves the land with no push. It also says the push is "run by `machined` outside the VM". Phase 8 puts `machined` **in the machine pod** on the sandbox pool, beside every lane (`...own-environment.md:138`, P8-D7), so the token sits in that pod's memory.
**How to fix:** Add a phase 8 delta task, or a section of this part, with the fenced wire. Give the header name and the `machined` push route. State that the token is read into memory for one call, is never logged, and is dropped after that call. Add a test that a redacting log handler never prints it. Correct "outside the VM" to the machine pod, and state that residual.

### S07-M1 (Minor): The commit leaves out the new migration and the mirror tests
**Where:** task-61.md:524
**Problem:** `git add` stages `landq/`, `server/landq*.go` and `api/teamland*.go`. It does not stage the new `store/migrations/00NN_landq.sql` or the new mirror test files, and a `git commit -- control-plane` pathspec does not pick up untracked files. The migration would be left out of the commit even though the local `pgstore` run passed.
**How to fix:** `git add` the migration file and the mirror test files explicitly.

### S07-M2 (Minor): An owner's unverified `rejected` report can stop every member's queue
**Where:** task-61.md:385-388
**Problem:** `Report{rejected, remote_moved}` is accepted without any check. Two such reports from one member stop the queue for the whole team and notify everyone. `pushed` is verified; `rejected` is not.
**How to fix:** Before counting a reject, confirm through the same trusted read as S07-C1 that the remote head is not `Base`. If the head has not moved, treat the report as `push_failed` for that entry only.

### S07-M3 (Minor): The push-credential test is vacuous
**Where:** task-61.md:479
**Problem:** The local control plane never holds the fetch credential, which lives with the broker or `machined`. So the test "`fX` appears in no request the local control plane makes" cannot fail, and the mutant it names cannot be written in `teamland.go`.
**How to fix:** Assert on the `machined` side instead: the push call uses the per-act header token and never the fetch credential file that `machined` holds for fetches.

## Task 62: Resolutions `counter`, `split`, `build-on`

### S07-I5 (Important): The Hub migration targets tables and a CHECK that task 04 does not have
**Where:** task-62.md:302-311
**Problem:** The migration alters `team_escalations` and `team_directives` and says task 04's part "is not on disk". It is on disk, and it names the tables `escalations` (`task-04.md:561`) and `directives` (`task-04.md:602`), so both `ALTER TABLE` statements fail. The directive's `kind` is not a CHECK either. It is the enum type `directive_kind` (`task-04.md:609`), so `'build-on'` needs `ALTER TYPE directive_kind ADD VALUE 'build-on'`. That statement cannot be used in the transaction that adds it, which matters if task 04's migrator wraps each file in one transaction. Task 04 also has `params jsonb` (`task-04.md:610`), which is where task 31's typed kinds keep their arguments, so the new `ref` column is a second place for the same fact.
**How to fix:** Write the migration against `escalations` and `directives`. Add the enum value in its own migration file, placed before any row uses it. Put `ref` in `params` under task 31's per-kind params schema instead of adding a column, or say why a column is needed. Remove the UNVERIFIED sentence.

### S07-I6 (Important): The decide's agent-config lines and the brought bundle both come from the mirror, and neither is re-checked
**Where:** task-62.md:30-32, :64-72, :251-260, :313-319
**Problem:** Property 4, the one that makes a build-on safe to decide, takes its lines from `LaneChanges`, and `LaneChanges` is the mirror's answer. The mirror parses every member's hostile objects (see S07-I3). `Bring` then stages a bundle the mirror exported. Nothing on Alice's side checks that the fetched ref's tip is `BaseTip`, or that the agent-config paths in `mergeBase..BaseTip` are the ones Alice was shown, before `wA` is directed to `git merge` it. A lying mirror can therefore get a `.claude/settings.json` or a git hook into Alice's sandbox through a decide that says "It changes no agent-config files". That is the exact risk §6.5 row `:731` names ("it runs their code, and possibly their settings, in my sandbox").
**How to fix:** In `ApplyApproved`, after `Bring`, have the builder's lane runner (phase 3's `/git/v1/`, in Alice's pod) report the fetched ref's sha and a `--name-status` list for `mergeBase..ref`. Admit the `build-on` directive only if the sha equals `BaseTip` and the classified list equals `Action.AgentConfig`. Otherwise delete the ref and refuse with `resolution_stale`. Add a test with the mutant "the mirror's list trusted".

### S07-I7 (Important): The task is sized at 3 hours but spans about 20 files in 3 modules
**Where:** task-62.md:3, :74-101
**Problem:** The task has three protocol kinds, a lease re-issue in one transaction, a new escalation rule, a decide row with a withheld option, a new typed directive through Fleet, the Hub types, route 64 and a migration, the Hub contract doc, a `sandboxd` CLI verb, an instruction paragraph, four fixtures read by both modules, and 14 tests. That is well past the house limit of 1-3 hours.
**How to fix:** Split it into 62a (`counter` and `split`: `messages.go`, `service.go`, `split.go`, `Reissue`, `KeyOf`) and 62b (`build-on`: `buildon.go`, the decide, the directive, the Hub delta, the CLI, the instructions). 62b depends on 62a.

### S07-M4 (Minor): The directive rule contradicts itself on turn origin
**Where:** task-62.md:69-70 vs :314-316
**Problem:** Decision (e) admits the directive "in a Fleet-started turn only". The directive section admits it "from either turn origin". A test is written for only one of the two readings (`:341`).
**How to fix:** Pick one reading, and add the other origin to `TestBuildOnDirectiveOnlyForTheAppliedRef`.

### S07-M5 (Minor): The commit leaves out the migration and the contract doc; step 2 expects the wrong failure
**Where:** task-62.md:376, :354-355
**Problem:** The commit does not `git add` the new `migrate/sql/00NN_escalation_agent_config.sql`, and its pathspec leaves out `docs/superpowers/contracts/2026-09-23-hub-contract.md`. That contract file exists only in the `hub` worktree today, so it arrives here with the sync-merge. Step 2 also expects "failures, not compile errors" for the counter and split tests, but `buildon_test.go` sits in the same `protocol` package and names `BuildOnPort`, so the whole package fails to compile.
**How to fix:** `git add` the migration and add the contract path to the commit. Expect a compile failure for `protocol` in step 2.

### S07-M6 (Minor): The base lane's owner never consents to her lane entering a teammate's sandbox
**Where:** task-62.md:399-401
**Problem:** Gap 2 raises "who decides". The same issue has a second side: a `build-on` sends Bob's unlanded lane, as content, into Alice's sandbox on Bob's Orchestrator's act alone. Task 63 keeps teammates' lane content off displays, so this is the only path by which one owner's unreviewed work reaches another owner.
**How to fix:** List it in "does not claim". Also ask the owner whether the base side's human is informed, or asked, before `Bring`.

## Task 63: Phase 3 delta, lenses by principal

### S07-I8 (Important): A teammate's path names reach another owner's screen with no text hygiene
**Where:** task-63.md:93-107, :240-251
**Problem:** Teammates' lane paths come from the mirror, which parses hostile lanes. They cross from one owner to another and are drawn in the viewer's Files and Merge lenses. Git allows any byte except NUL and `/` in a path. The part applies no bound on path length and no bidi or control-character check (task 02's `teamtext` hygiene, G3) to `LaneStat.Path` or `LaneConflict.Paths`. A lane can therefore name a file `src/‮gnp.exe`, or a path whose text imitates the lens's own sentence (*"Alice Chen's lane changes this file…"*), and it is shown on Priya's desktop as a real row. Every other cross-owner string in this plan goes through `teamtext`, so this one should too.
**How to fix:** In `verbs.Lanes`, run each path through `teamtext.ValidPath`, bounded at 200 as in task 62's `EscalationPath`, with the hygiene check, before it enters `TeamLane`. Drop a failing path and count it in `More`, with a reason. Add a Go test, with the mutant "a bidi path relayed", and a TS test that a path renders as one text node, in Commit Mono, never as markup.

### S07-I9 (Important): The task spans the mirror, Fleet, the local API and four desktop views, sized at 3 hours
**Where:** task-63.md:3, :56-75
**Problem:** The task covers a new mirror op with a real-git test, a Fleet verb and route, a local relay, a refusal on phase 3's diff route, the TS types, the slice, a pure join, four components and 11 tests, across three modules. That is more than one 1-3 hour task.
**How to fix:** Split it into 63a (mirror `lane-stats`, `verbs.Lanes`, both routes, the diff refusal, the Go tests) and 63b (types, `withTeammates`, the four views, the TS tests). 63b depends on 63a.

### S07-M7 (Minor): Same mirror-path and ref-name errors as task 61, and no op wire
**Where:** task-63.md:58, :91, :126, :266, :291, :301
**Problem:** The part names `sandboxd/cmd/panacea-mirror/main.go`, runs mirror tests in `sandboxd`, and commits the pathspec `sandboxd/cmd/panacea-mirror`, but the runner is at `control-plane/cmd/panacea-mirror` (`task-59.md:72`). It also measures against "the integration head", where task 59's ref is `refs/panacea/base/<branch>`. The `lane-stats` op has a Go client method but no `Request` or answer frame in task 59's wire (`task-59.md:400-410`).
**How to fix:** Apply S07-I1's paths and ref name. Add the `lane-stats` request and answer as a fenced extension of task 59's `Request`.

### S07-M8 (Minor): The Fleet cache field for `laneRef` is promised but not written
**Where:** task-63.md:184-186, :318-319
**Problem:** Task 11's part has no `laneRef` (checked: no match in `task-11.md`). The part says it "adds the field to Fleet's cache" if absent, but gives no source, no migration and no test for it. The house style asks for complete source for every type.
**How to fix:** Add the fenced cache struct field and the rebuild line in task 11's cache, with a test that a Hub agent's `laneRef` survives a rebuild. Also reuse task 52's `PersonRef` (`task-52.md:268-271`) instead of a second identical `verbs.Person`.

## Task 64: "Move my lanes to the cloud"

### S07-I10 (Important): `bundle-in` duplicates phase 8 task 21b's bundle routes and skips their limits
**Where:** task-64.md:71-73, :216-221, :315-318
**Problem:** Gap 1 says "Phase 8 has no bundle-into-the-project op for a laptop's bundle". It does. Phase 8 task 21b (`env/.../task-21b.md:1-13`) builds exactly that: `POST /v1/bundles`, where the laptop relays a body into `machined`'s owned root, then the one bundle argument `POST /v1/git` admits, `Scope: project` only. It comes with the limits, the `429` at two live ids and the sweep (env ruling 3). Task 64 adds a second route, `POST /project/v1/bundle-in`, off `machined`'s `/v1/` scheme, writing to "the project's private temp directory". That path escapes 21b's `/tmp` `sizeLimit`, its two-live-bundle cap and its sweep. It also runs into 21b's recorded blocker, task 19's 16 MiB request cap on the proxy (`task-21b.md:22-27`), without saying so.
**How to fix:** Delete `bundlein.go`. Have `Cloud.BundleIn` call 21b's `POST /v1/bundles`, then `POST /v1/git` `fetch` with the bundle id, into `refs/panacea/moved/<id>`. List task 21b, and task 19's streamed-body change, under "Depends on". Rewrite gap 1 as "uses 21b; needs 19's streamed POST".

### S07-M9 (Minor): The lane-creation delta stops at phase 3's `CreateRequest`
**Where:** task-64.md:65-67, :187-191, :212-213, :68-70
**Problem:** `CreateRequest.StartAt` and `MovedFrom` are added to phase 3's type, but a cloud lane is created through phase 8 task 21's `machined` wire: `NewLane` on the wire and `POST /v1/lanes/sealed` (`env/...own-environment.md:600`). That wire gains neither field, so `ByMovedFrom` on the cloud catalog has nothing to read. The local-session refusal site is "named at build time", which leaves the one check without source. "Depends on" lists only 21 and P3-24, but the part also consumes phase 8 tasks 21 and 21b and tasks 06, 34, 52 and 60.
**How to fix:** Add a fenced `NewLane` extension (`startAt`, `movedFrom`) and the catalog field. Name the session site conditionally, as "phase 3 task 05's `Launch` if merged, else phase 2's `sessions.go` create handler", and give the check's source. Complete "Depends on".

## Task 65: Phase 8 delta, the CLI bump job

### S07-I11 (Important): `bump-gate` is a new keysmith caller that can mint for any owner
**Where:** task-65.md:314-325, :456-457, :494-499
**Problem:** `gatemint` mints as the table caller `bump-gate`, and the owner, team and project it names come from Job env values. Phase 6 task 08 is explicit that "A table caller carries no `Owner`", and that the owner a table caller sends is trusted as the broker's is (`phase-6 .../task-08.md:65`). So the `bump-gate` row lets any pod running as SA `bump-gate` in `panacea-sandboxes` mint gateway tokens attributed to **any** person, team and project, and spend their OD-24 budgets. The fifth pipeline identity (gap 1), which may create Jobs there, is one such creator. That puts a second broker-equivalent in the sandboxes namespace, the one namespace hostile code runs in. The Job's own init-container mount is done well (`automountServiceAccountToken: false`, `:476`). The gap is keysmith's side.
**How to fix:** Bind the caller in keysmith. Either add a caller-table column that pins `bump-gate` to one `(owner, team, project)` and refuses any other with `403 caller_forbidden` (a phase 6 delta, fenced here), or have gatemint call the broker rather than keysmith. Test it with the mutant "bump-gate mints for Alice". Move the SA and Jobs into their own namespace, not `panacea-sandboxes`. Also fix the citation: the caller-table format is at `task-08.md:47`, not `:45`.

### S07-I12 (Important): The G4 log check can pass while missing a refusal line
**Where:** task-65.md:538-543
**Problem:** G4 passes on "**zero** refusals" in `kubectl logs -n panacea-gateway -l app.kubernetes.io/name=gatekeeper`. With a label selector, `kubectl logs` defaults to `--tail=10` per pod and to `--max-log-requests=5`, so on a busy dev gatekeeper the refusal line can fall outside the ten lines read. The gate then passes. That is the false pass this gate exists to prevent. The contract says "since the Job's start" but names no flags.
**How to fix:** Name the exact flags in the contract: `--tail=-1 --since-time=<job start> --prefix --max-log-requests=<replica count>`. Before judging, refuse with a setup error if the number of pods read is less than the Deployment's ready replicas. Add a `gate-live` test case with a fake `kubectl` that returns a refusal as the 11th line, and the mutant "tail default".

### S07-I13 (Important): Sized at 3 hours, the task holds four tasks' work
**Where:** task-65.md:3, :85-100, :212-215
**Problem:** The task writes about 25 files: the pin file and Dockerfile, `build.sh`, three pipeline jobs plus a new scheduled pipeline, eight shell scripts with three test suites, a new Go command, Fleet config and `Launch`, chart values, a schema and a unit test, and two cluster Jobs. It has about 30 tests. That is well past the house limit of 1-3 hours, and it is also the task most likely to stall on a pipeline identity (gap 1).
**How to fix:** Split it into 65a (pin file, Dockerfile, `build.sh`, `lib/gate.sh`, `publish-digests.sh` and their tests), 65b (`cli-bump.yml`, `cli-latest.sh`, `cli-pr.sh`), 65c (`gatemint`, G2 and its test, `gate-live.sh`, the Jobs, the `images.yml` jobs) and 65d (Fleet config, `Launch`, chart).

### S07-M10 (Minor): "Through the same four gates" is not true for the mirror image
**Where:** task-65.md:64-71, :292-296
**Problem:** Decision (c) says the mirror digest publishes "through the same four gates", and task 58 relies on its digest moving "only with the CLI bump job's gates" (`task-58.md:72-75`). But every gate judges only the `sandbox` digest (`publish-digests.sh:295`). `mirror`, `machine`, `broker` and `hub` are only format-checked. A git bump in the mirror's Dockerfile, the component that parses hostile objects, publishes with no gate running the mirror image.
**How to fix:** Add a fifth gate, `mirror`: run task 59's two real-git tests, including the `fsckObjects` refusal, inside the pushed mirror image. Or reword (c) and task 58 to say the mirror is published beside the gates, not gated by them.

### S07-M11 (Minor): The card's version comes from Fleet's config, not from the pod's image, and the G9 binary ships to every user
**Where:** task-65.md:79-83, :578-586, :519-523
**Problem:** `Launch` writes `cfg.ClaudeCodeVersion`, but the pod's image is the broker's session image. The two are set by one `helm-values.sh` run but roll out as separate Deployments, so a record can show a version its pod does not run, which weakens property 3. Separately, `g9.test`, which carries the injection corpus, is built into the warm-pooled sandbox image that every user's session runs.
**How to fix:** Have the broker return the digest it started, and the version label read at image build (`digests.json`), and record that. Take gap 5's own alternative: a separate `images/g9` built `FROM` the sandbox digest and never warm-pooled.

## Task 66: Team and project quotas, server side

### S07-I14 (Important): The gateway caller is scoped to one org, and a miss switches the daily quota off
**Where:** task-66.md:143-150, :283, :716-765
**Problem:** Rule 9 gives the gateway's workload identity "its org is the token's `tid` mapped as for every caller". The Hub enrols one org per tenant (`hub/.../2026-09-23-hub-contract.md:314`). The gateway serves every enrolled org, but its app token carries the platform's own `tid`, so for any other org route 74 and route 76 are refused as another org's. The failure is open, not closed. `HubDay.read` caches `isTeam == false` for any route-74 404 "whatever code task 43 gives it" (`:761-765`), and `TestMintSoloHasNoDay` then pins "no daily cap". So a team project the gateway cannot see mints with no daily quota at all. That is the "unset quota is the org pool by another name" hole that `07` I3 and OD-24 exist to close.
**How to fix:** Make the gateway an explicitly cross-org caller in rule 9. Its org comes from the route's project or `orgId`, it is bounded to enrolled orgs, and the ADR says it is a cross-tenant deputy, with R12-style risk text. Separate "not a team project" from "not found or not visible": route 74 should answer a typed `{"team": false}` for a known solo project. Every 403, 404 or error then fails the mint closed after `Stale`. Add a test with the mutant "a 404 read as solo".

### S07-I15 (Important): Rule 9 is written against an auth branch task 05 does not have
**Where:** task-66.md:272-284, :1066-1069, :129-132
**Problem:** The hunk switches on `claims.Idtyp == "app"`, calls it "task 05's app-only branch", and says "Task 05 is not on disk". Task 05 is on disk. It admits app-only tokens through `entra.Config.AppCallers []AppCaller{ClientID, Role}` and `Claims.AppOnly` (`task-05.md:148-161`, `:184-208`), and it has no `Idtyp` field. Unless `{GatewayClientID, "Hub.Gateway"}` is added to `AppCallers`, the validator refuses the gateway token first (`azp is not an allowed client`, `task-05.md:192-193`), so rule 9 never runs. `Claims.AppOnly` also does not say *which* app caller matched, so the Fleet and gateway branches cannot be told apart as written. Task 43 is on disk too (`task-43.md:51`, `:465-466`), yet "its name is assumed".
**How to fix:** Rewrite the hunk on task 05's shapes. Add the gateway to `AppCallers`, have `Claims` carry the matched `Role` (or `AppCaller`), and branch on `AppOnly && Role == "Hub.Gateway"`. Read task 43's real route-75 handler. Remove the two "not on disk" gaps. State which issuer mints the gateway's app token under OD-30, marked UNVERIFIED if unsettled.

### S07-I16 (Important): Sized at 2.5 hours, it is three tasks' work
**Where:** task-66.md:3, :72-114, :1001-1015
**Problem:** The steps themselves commit three separate halves: Hub, gateway and local, each after a different sibling's sync-merge. Between them they cover 4 Hub routes, a migration, an auth rule, ADR and contract text, a phase 6 `budget`, `usage`, migration and `keysmith` delta, a new `teamquota` package with its loop, and a local composer with 35 tests. The halves also wait on different merges, so one task cannot finish until both siblings land.
**How to fix:** Split it into 66a (the Hub delta), 66b (the phase 6 delta and `teamquota`) and 66c (the local `QuotaView`), each with its own dependency on its sibling merge.

### S07-M12 (Minor): The part changes what the scope says it writes, and the skeleton still says the old thing
**Where:** task-66.md:29-37; skeleton `2026-09-25-phase-t-team-mode.md:1097-1102`
**Problem:** The scope says the task "reads and writes the gateway's budgets ... person → LiteLLM user budget, project → team budget". Decision (b) writes neither: it adds a daily cap at keysmith mint and leaves the monthly budgets to phase 6. The reason is sound, but nothing now sets a project's monthly LiteLLM team budget from the team admin's number, and the skeleton still promises it.
**How to fix:** Amend the skeleton's scope 66 to say what is built. Also name who sets the monthly project budget in team mode: phase 6 defaults, or a later task.

## Task 67: The quotas admin view

Citations checked against the tree: `Popover.tsx:10-15`, `client.ts:57-69` (`details: Record<string, string>`), and `tokens.css:152` (`--mono`) and `:156` (`--numeric`) all match. The part is sound: it keeps OD-27, reads `canEdit` from the server, and tests the pure sentences.

### S07-M13 (Minor): The "team-only" test cannot fail
**Where:** task-67.md:473, :383-405
**Problem:** `the link is team-only` renders `SpendLine` with `settings: null`. That returns `null` under task 56's own guard whatever this task does, so the mutant it names, "an admin surface on a solo project", survives. What actually gates the link is `client !== undefined` plus task 56 mounting `SpendLine` only on team projects, and neither is exercised.
**How to fix:** Render task 56's popover for a solo-project `TeamSettings` (or whichever prop task 56 uses to mark a solo project) and assert there is no *Team quotas* button. Also assert that with settings but no `client` there is no button.

### S07-M14 (Minor): There is no commit command, and step 1's prototype commit has no paths
**Where:** task-67.md:481-489
**Problem:** Step 5 says "Commit the desktop half with the paths above", and step 1 says "Commit", but neither gives a command. The house style gives the exact `git add` and `git commit -F … -- <paths>`, and this task has three new files that need `git add`.
**How to fix:** Write both commands. Step 1 commits `prototype/team-mode.html`. Step 5 runs `git add` on `quotaWords.ts`, `QuotaAdmin.tsx` and both test files, then commits with the pathspec `desktop/src`.

## Task 68: Release attribution on a team project

### S07-I17 (Important): `landq/record.go` is written against a task 61 that does not exist, and task 61 rules the record out
**Where:** task-68.md:437-484; task-61.md:147-169, :290-299, :537-538, :549-551, :45-47
**Problem:** `record.go` is labelled "complete", but it uses `e.Owner`, `e.LandID`, `e.AgentID`, `e.BranchRef`, `e.MergeCommit`, `q.store`, `q.outbox` and `q.store.SetLandID`. Task 61's `Entry` has `OwnerUserID`, `Agent model.AgentID`, `Branch` (a leaf, not `refs/heads/…`) and `Merge`, with no `LandID`. Its `Queue` has an exported `Store` and no outbox. Its migration has no `land_id` column. The part says task 61's part "is being written". It is on disk. The two parts also disagree on the design. Task 61 decision (b) says "release attribution (task 68) reads the person from the commit". Its "does not claim" lists "**Any Hub record of a land**". Its gap 2 leaves the Hub record to the owner. Task 68 decision (a) makes the Hub record the only source. As planned, whichever builder goes second meets a contradiction and code that does not compile.
**How to fix:** Rewrite `record.go` on task 61's real names (`OwnerUserID`, `Agent`, `"refs/heads/"+Branch`, `Merge`). In task 61, add `Entry.LandID`, a `land_id` column, `Store.SetLandID` and an `Outbox` field on `Queue`, and call `Recorded` at step 9's `landed` transition. Correct task 61's decision (b), its "does not claim" line and gap 2 to point at task 68. Add task 59's `ownerTrailer` field to task 59's `LandBuilt` answer (see S07-I2) instead of leaving it as a gap. Also fix the citation: the verified push is task 61's property 4 and step 9 (`task-61.md:25-27`, `:381-384`), not decision (d).

### S07-M15 (Minor): A release reserved before the outbox delivers is misattributed for good
**Where:** task-68.md:39-42, :302-307, :454-458
**Problem:** The land reaches the Hub through task 11's outbox, asynchronously, after the push. Attribution is fixed at reserve "then never again". So a release reserved in the seconds between the push and delivery is `commit_not_landed` permanently, even though the land is real and later arrives.
**How to fix:** Have Fleet's `Recorded` run before task 61's entry reaches `landed` and before members are told, and have the desktop not offer Ship for a head until the entry is `landed`. Or give `commit_not_landed` a narrow repair: once, when a land with that exact `mergeCommit` arrives within N minutes, audited. Say which, and test it.

### S07-M16 (Minor): When the boundary is past the head, the rule changes meaning, and the contract text does not say so
**Where:** task-68.md:309-314, :334-337, :504-507
**Problem:** Normally a release lists the lands *since the previous confirmed release*. When that boundary is at or past the head, `from` falls back to 0 and the release lists *everything up to the head*, a set with a different meaning under the same `listed` / `team_lands` status. §7.28's contract text states only the first rule, so phase 7 cannot tell which it got.
**How to fix:** Give the fallback its own reason (`team_lands_all`), add the sentence to §7.28, and add a test (`TestOlderCommitAfterNewerListsAll`).

## Task 69: TM2 proof

### S07-I18 (Important): M1 fails a correct task 58 deployment in three ways and passes a wrong one in a fourth
**Where:** task-69.md:17, :225-236, :250, :437-438
**Problem:** Checked against task 58's chart (`task-58.md:354-372`):
- (1) The collector selects the policy with `kubectl get networkpolicy -l panacea-app/component=mirror`. Task 58's policy `mirror-ingress-from-broker` carries only `app.kubernetes.io/part-of: panacea-app`. `panacea-app/component: mirror` is its `podSelector`, not a label, so the list is empty.
- (2) M1 requires `IngressOnlyFleetControl`, but task 58's only ingress is from the **broker** namespace (`panacea-broker`).
- (3) M1 requires `DeniesEgress` on that one policy, but it is `policyTypes: [Ingress]` with no egress section. Egress is denied by the namespace's separate default-deny (`task-58.md:354-355`, `:389`, `:441`), which the collector never fetches.
- (4) Task 58 decision (b) gives the mirror **no** credential, not even the fetch credential, so `FetchSecret` is `""`. The volume check `vol.Claim != m.ProjectClaim && vol.Secret != m.FetchSecret` then passes **every** non-Secret volume, including another PVC or a `hostPath`, because `vol.Secret == "" == FetchSecret`. `TestM1ExtraVolumeFails` only catches this because its canned `pass/` presumably sets a `FetchSecret`.
M1 also still says "no credential beyond fetch" (`:17`, `:250`), which is weaker than task 58's decision.
**How to fix:**
- Fetch every NetworkPolicy in `panacea-mirrors`.
- Judge ingress as "only from `panacea-broker`", and egress as "a default-deny with `Egress` in `policyTypes` selecting all pods, and no policy with an egress rule selecting the mirror".
- Require that the mirror mounts **no** Secret.
- Rewrite the volume check as an allowlist: its own claim, plus projected or `emptyDir` volumes named in the check. Anything else fails.
- Add `TestM1EmptyFetchSecretStillRefusesAnotherClaim` and `TestM1BrokerIngressPasses`.

### S07-I19 (Important): M5 fails every correct run of task 65
**Where:** task-69.md:208, :338-341, :577-579
**Problem:** `bumpGates` expects `od8-canary, panacea-channel, g9-corpus, gateway-compat`, and M5 compares the list in order. Task 65 runs G4 (gateway) before G3 (G9), the cheap gate first, and writes `bump-result.json` "in the order they ran" (`task-65.md:614-616`). Its gap 7 already asks for this fix. So a correct dry run is `FAIL (gate g9-corpus did not pass in its place)`. Gap 2 here still says "Task 65 is not on disk"; it is on disk, and it emits `BumpCheck`'s shape.
**How to fix:** Set `bumpGates` to `od8-canary, panacea-channel, gateway-compat, g9-corpus`. Update `TestM5GateOrderAndCount` and `testdata/tm2/pass/bump.json`. Drop gap 2.

### S07-M17 (Minor): The collector says it writes nothing, then starts a pod with the mirror's labels
**Where:** task-69.md:429-430, :440, :465-466, :31-35
**Problem:** The header says "Reads only; writes nothing to the cluster", but `tm2-mirror-probe.sh` creates a probe pod in `panacea-mirrors` carrying the mirror's labels. If any mirror Service or Fleet lookup selects by those labels, then for the run the probe pod is an endpoint that Fleet's authenticated requests can reach.
**How to fix:** Correct the header. Label the probe pod only with the policy's `podSelector` label (`panacea-app/component: mirror`), never with the per-project label that Fleet or a Service uses to find a project's mirror. Delete it in a `trap`.

### S07-M18 (Minor): M4 finds the build-on decide by matching the question's free text
**Where:** task-69.md:312
**Problem:** `strings.Contains(es.Question.String(), " builds on ")` ties the proof to task 62's draft wording (`task-62.md:268-271`), which the owner may change (task 67 decision (b) shows wording moves). The decide row key `build-on` is a typed fact.
**How to fix:** Match on a typed field of the escalation, such as its decide key or option set (`hold`, `apply-proposal` or `decline-proposal`), plus a non-empty `agentConfig`. If the Hub record lacks such a field, add one in task 62's Hub delta.

---

## Summary

| Part | Critical | Important | Minor |
|---|---|---|---|
| 61 | C1 | I1, I2, I3, I4 | M1, M2, M3 |
| 62 | | I5, I6, I7 | M4, M5, M6 |
| 63 | | I8, I9 | M7, M8 |
| 64 | | I10 | M9 |
| 65 | | I11, I12, I13 | M10, M11 |
| 66 | | I14, I15, I16 | M12 |
| 67 | | | M13, M14 |
| 68 | | I17 | M15, M16 |
| 69 | | I18, I19 | M17, M18 |

No part file is missing or cut off: all nine end in their gaps sections. Totals: 1 Critical, 19 Important, 18 Minor. The through-line: tasks 58 and 59 made the mirror credential-less and egress-less, and tasks 61, 63, 68 and 69 were written against an earlier mirror that fetches the remote and lives in `sandboxd`. Its answers are also trusted where they should be re-checked on the owner's side (I3, I6). Four parts still say a sibling part "is not on disk" when it is (62 on 04, 66 on 05 and 43, 68 on 61, 69 on 65).

Section verdict: Needs fixes

## Section 90: OD-30 identity (orchestrator)

Source: OD-30 (`docs/superpowers/planning/2026-09-23-owner-decisions.md`, main tree) and its ruling,
main tree `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` (below: "ruling §N").
Users sign in through a Keycloak realm brokering Entra; services calling the Hub or the broker are
realm confidential clients. Paths below are relative to this worktree. The person key `(tid, oid)`
is unchanged everywhere (ruling §4), so no Hub row, owner label or delegation key changes.

### ID-I1 (Important): Fleet's app-only branch in the validator is replaced by `ServiceClients`
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-05.md:89-92`, `:144-212`;
`task-17.md:36-39`, `:105-107`, `:126-278`, `:414-415`
**Problem:** Both parts extend `control-plane/internal/entra` with `Config.AppCallers`,
`Claims.AppOnly` and a "no `scp`, listed `azp`, listed role" switch (`Hub.Fleet`,
`Sandboxes.Fleet`). Under OD-30 that package is renamed `internal/oidcauth` by env task 17 step 0
and gains `ServiceClients` and `Claims.Service` (ruling §5). A Keycloak service token has no
Entra app role and carries `scope`, not `scp`, so the planned switch never matches, and two
parts would add a second service-caller mechanism beside the ruled one.
**How to fix:** Neither part touches the validator. Both import env 17 step 0's `oidcauth` under
`ProfileKeycloak` (ruling §5). Fleet is recognised as `claims.Service && claims.ClientID ==
"panacea-app-fleet"` (ruling §6): Hub `ServiceClients` = `panacea-app-fleet`, `panacea-app-gateway`;
broker `ServiceClients` = `panacea-app-fleet`. Delete `AppCaller`, `AppOnly`, the roles
`Hub.Fleet`/`Sandboxes.Fleet` and `PANACEA_APP_BROKER_FLEET_ROLE`;
`PANACEA_APP_BROKER_FLEET_CLIENT_ID` becomes the string `panacea-app-fleet`, not a GUID. Replace
`TestAppOnlyTokenWithoutRoleIs401` and `TestAppCallerAdmitted` with: a `panacea-app-fleet` service
token naming a live delegation is admitted (kills "delegation check skipped"); one naming none is
refused (kills "Service alone admits"); a `panacea-app-fleet` token carrying `oid` is refused by
`oidcauth` (kills "service branch reads oid").

### ID-I2 (Important): the "delegated callers" amendment names workload identity, not the service client
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-01.md:18-24`, `:115-125`;
`docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:419-420`;
`docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:1127`
**Problem:** Amendment (a) admits "Fleet's workload identity", an app-only Entra token
(`idtyp=app`, no `scp`, app role `Hub.Fleet`, `azp == PANACEA_APP_HUB_FLEET_CLIENT_ID`). Ruling §6:
the Hub's `azp` rule now gains **`panacea-app-fleet`** in its `ServiceClients`; Entra workload
identity is kept only for the Azure plane (Postgres, Key Vault, storage, AKS).
**How to fix:** Rewrite (a) as ruling §6 states it: a token with `Service` true and
`azp == panacea-app-fleet`, audience `panacea-app-hub`, is refused unless it names a live 24-hour
delegation the person's own desktop token created; actor = the person, Fleet the delegate. The
`:369` "no `scp`" change is no longer needed (the service token carries `scope`); drop `Hub.Fleet`.
Amendment (e) (devissuer's client-credentials grant) stands; it is hub tasks 11-12's (ruling §5).

### ID-I3 (Important): `fleetd` validates and mints Entra tokens; both become realm tokens
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-12.md:100-107`, `:411`,
`:464`, `:666-672`, `:722-729`, `:793`; `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:539-540`
**Problem:** Inbound: `PANACEA_APP_FLEET_ENTRA_AUTHORITY`, `_ENTRA_TENANTS`, `_AUDIENCE` (an app id
URI), `_REQUIRED_SCOPE` default `Fleet.Access` and `_DESKTOP_CLIENTS` feed `internal/entra`, which
refuses every Keycloak token (ruling §5). Outbound: `token.go` mints Fleet's Hub token by client
credentials with a workload-identity **client assertion**; ruling §6 makes Fleet the realm
confidential client `panacea-app-fleet` with a **client secret** from Key Vault via the Secrets
Store CSI driver.
**How to fix:** Inbound, per ruling §2 and §4: `PANACEA_APP_OIDC_ISSUER` and
`PANACEA_APP_OIDC_PROFILE` (shared names), audience and required scope `panacea-app-fleet`,
`AllowedClients` = `panacea-app-desktop`, `TenantIDs` = enrolled tids; import `oidcauth`, not
`entra`. Delete `_ENTRA_AUTHORITY` and `Fleet.Access`. Outbound: `grant_type=client_credentials`,
`client_id=panacea-app-fleet`, the secret read from its CSI-mounted file on every fetch,
`scope=panacea-app-hub` or `scope=panacea-app-broker`, one audience per token, at the realm's
`token_endpoint` from discovery. `PANACEA_APP_FLEET_ASSERTION_FILE` becomes a secret-file variable;
the name (for example `PANACEA_APP_FLEET_CLIENT_SECRET_FILE`) is the part's to choose, since the
ruling names none. Tests: a secret file rewritten between fetches is re-read (kills "cached once");
the form carries no `client_assertion` (kills a leftover assertion path).

### ID-I4 (Important): Fleet's chart mounts a federated token for the Hub token, not a CSI secret
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-32.md:221-222`, `:228-234`
**Problem:** `PANACEA_APP_FLEET_ASSERTION_FILE = $(AZURE_FEDERATED_TOKEN_FILE)` and "the Hub token is
minted from the projected assertion" follow from ID-I3's old design; `_ENTRA_*` come "from `entra`".
**How to fix:** A Secrets Store CSI `SecretProviderClass` mounts `panacea-app-fleet`'s client secret
from the environment's Key Vault (ruling §6); the chart sets the `PANACEA_APP_OIDC_*` variables and
`PANACEA_APP_FLEET_AUDIENCE=panacea-app-fleet`. Fleet's workload identity **stays** for Postgres and
Key Vault (Azure plane, ruling §6). Rewrite the "no value is a secret" sentence: one value is now a
secret, reaching the pod only as a CSI file, never an env value.

### ID-I5 (Important): the gateway's Hub caller is an app role check, not the `panacea-app-gateway` service client
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-66.md:56-60`, `:144`,
`:160`, `:274`, `:296-297`, `:960`
**Problem:** Rule 9 admits the gateway by `claims.Idtyp == "app"`, `Hub.Gateway` in `roles` and
`azp == PANACEA_APP_HUB_GATEWAY_CLIENT_ID`. A realm service token has no `idtyp` and no app role.
**How to fix:** Per ruling §6: `claims.Service && claims.ClientID == s.cfg.GatewayClientID`, with
`PANACEA_APP_HUB_GATEWAY_CLIENT_ID=panacea-app-gateway` and audience `panacea-app-hub`, on routes
74, 76 and 80 only. Delete `Hub.Gateway`. Replace `TestGatewayRoleAndAzpBothRequired` with
`TestGatewayServiceAndAzpBothRequired`: a person token (`Service` false) on route 76 and a
`panacea-app-fleet` service token are both refused (kills `||` for `&&`); the gateway token on any other route is
refused (kills a route-list bypass).

### ID-I6 (Important): desktop-to-Fleet names `entraauth`, `internal/entra` and "the Entra token"
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/task-21.md:9`, `:40`, `:77`,
`:101`, `:166`; `task-39.md:16`, `:242`; `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:638`, `:844`
**Problem:** Hub task 31's package is renamed `entraauth` -> `signin` and each call site asks
`TokenSource.Token(ctx, "<audience scope>")` for a single-audience token (ruling §2 step 7, §11).
`entra.Claims.ExpiresAt` becomes `oidcauth.Claims.ExpiresAt` (ruling §5: unchanged fields kept).
**How to fix:** `fleetbe` takes `signin.TokenSource` and asks `Token(ctx, "panacea-app-fleet")`;
"Bearer <Entra token, audience Fleet>" -> "Bearer <realm token, audience `panacea-app-fleet`>";
task 39 imports `oidcauth`. Owner = the token's `(tid, oid)`, unchanged (ruling §6).

### ID-I7 (Important): sign-in wording still says Entra PKCE (OD-6)
**Where:** `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:41`, `:106`, `:205`, `:323`;
`docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:161-162`, `:189-191`, `:853`, `:1055`
**Problem:** "Entra PKCE (OD-6)", "an Entra access token with Fleet's API as audience", "Entra per
request" and "`azp` the desktop's client id (`ADR-H1:370`)" predate OD-30. `:161-162` cites P8-D4,
which ruling §1 rewrote (realm token, audience `panacea-app-broker`, owner = `oid`).
**How to fix:** "signs in through the Keycloak realm brokering Entra, PKCE, zero clicks (OD-6,
OD-30)"; "a realm access token, audience `panacea-app-fleet`, `azp == panacea-app-desktop`";
"realm token per request". Keep `(tid, oid)` as the owner (ruling §4). Change no design decision.

### ID-I8 (Important): leavers need the 7-day offline cap, and the daily check is not the realm's
**Where:** `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:174-181`;
`docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:900-902`
**Problem:** The leaver text names two bounds (24-hour delegation lapse, a daily Graph account-state
check). Under OD-30 an offline refresh token outlives an Entra disablement until it lapses, and the
realm cannot show Entra's account state (`storeToken:false`, no user sync; ruling §6, U-12).
**How to fix:** Add the third bound: the desktop client's 7-day offline-session cap forces a
re-check against Entra at least weekly (ruling §6). Keep the daily check as written, and say it
reads Entra (Graph) by `oid` directly, never through the realm; its Graph read stays UNVERIFIED.

Checked, no change: task 48's Azure DevOps fetch identity (`task-48.md:53`, `:130-137`) is Azure-plane
workload identity, which ruling §6 leaves unchanged; task 32's Postgres workload identity likewise.

Section verdict: Needs fixes
**Plan quality: Needs fixes**

## Fix batch b01 (2026-09-26): S01-C1, X-C1

Fixer: plan fix agent, batch b01. Line numbers are those on `feat/team-mode` after round 2's
last commit (re-anchored from round 1's by `recite2.py`, see round 2's I-3 row). "parts/" is
`docs/superpowers/plans/2026-09-25-phase-t-team-mode-parts/`.

### S01-C1: moot, confirmed

Both part files exist: `parts/task-04.md` (the Hub migration: complete SQL from `:113`, the tests
and the mutant each kills, e.g. `TestFeedFlagsComeFromTheKind` at `:868`) and `parts/task-05.md`
(the delegated-caller authentication, `:1`, with its deny-by-default tests). The finding was
written before they were; nothing to change.

### X-C1: fixed by widening task 06 (no new task)

The review's two options were a new Hub delta task or a wider task 06. Task 06 was widened: the
writer belongs beside route 46, which now reads the mode, and a new task would have changed the
69-task list for one handler. **The route is 87, not 80:** the first b01 commit (`8d661b39`) said
80, but task 14 already holds route 80 (`parts/task-14.md:600`), and 80 to 86 are claimed by
tasks 14, 46, 49, 52, 61 and 66 to 69. Task 01's §7.0 ended at 79; 87 is the first free number.

| Fix item (the review's "how to fix") | Where |
|---|---|
| The route `POST /v1/projects/{p}/mode` `{"mode":"team"}`: admin only (the Hub has no project owner role, contract §5.2 `:326-333`), one way, one `project.mode_set` | `parts/task-06.md:210` (wire and handler order), `:36` (caller table), `:712` (registered bare) |
| Calls the gate first; 503 `audit_not_streaming` | `parts/task-06.md:210` step 3, the `teamModeGate` seam; test `TestProjectModeWaitsForTheAuditGate` `:602` |
| Under TM1-α, a dev-only flag, stated as one | `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED`, refused unless `ENV` is `local` or `dev`: `parts/task-06.md:60`, test `:617`; contract §12.2 row `parts/task-01.md:245` |
| Task 45's sink fills the gate | `parts/task-45.md:61`, `TeamModeAllowed` `:396-400` (open for `logs`, or the flag beside `fake` only), tests `:631-632`, Gap 1 resolved `:719` |
| `Mode` on `hubapi.Project` (task 03) | Block 7 in `types.go`, `parts/task-03.md:958`; `TestProjectModeWire` `:1182`; mutants 5 and 6 `:1582-1584` |
| `hubclient.SetProjectMode` beside the fake's | `Team.SetProjectMode` `parts/task-08.md:92`; the fake keeps the route's rules instead of a direct setter `:321`; four conformance rows `:450-459`; Gap 1 resolved `:597` |
| Name its caller | Task 21's local route `POST /v1/projects/{projectId}/team-mode` and `TeamProjects.MakeTeam`, `parts/task-21.md:297`, `:380`, tests `:437-452`; task 56's Share, decision (h) `parts/task-56.md:83`, tests `:223-225` |
| Task 33's E5 runs the route | E5 `parts/task-33.md:79`; must-hold M0 `:109`; script `05-team-mode.sh` `:87`, `:250` |
| The contract carries it | task 01: §7.0 row 87 `:293`, §5.3 row `:619`, §7.4 amendment and §7.1 `Project` example `:515-527`, §8 `project.mode_set` `:569`, acceptance-script checks `:740`, `:754` |
| Restatements fixed | task 04 decision (f) `:67`, kinds row `:255` (22 → 23 kinds, `:57`, `:868`), "does not claim" `:962`; task 43 Z5 row 87 `:401`; skeleton scopes 06 `:486`, 21 `:653`, 33 `:795`, 45 `:926`, 56 `:1032`; task list rows 06 `:317`, 21 `:332`, 45 `:363` |

Found while fixing, and fixed because the finding needs them:

1. **The feed flag.** `project.mode_set` was in no `team_event_kinds` row, so task 04's trigger
   would have set `fleet_feed = false` and Fleet would never learn a project became team
   (`05-design.md` §10.1 step 2). Row added, `parts/task-04.md:255`.
2. **An existing Hub test.** `hubapi`'s `TestFieldNamesMatchTheContractExamples`
   (`hubapi/types_test.go:179`) compares `Project` with contract §7.1's example, so `Mode` needs
   the §7.1 line too (`parts/task-01.md:527`) and a one-key edit to that test (task 03).
3. **Route 46 reads the mode** (409 `not_team_project`, `parts/task-06.md:134`), so every
   delegation, and so every Fleet write, sits behind route 87 and its gate. Without it the gate
   would guard a column nothing depends on.

Declared ruling (not the owner's words): the product caller is Share, not the first team launch.
A launch that turned a project team would skip the consent line `05-design.md` §9.5 puts at
Share. `TestLaunchNeverMakesTeam` (`parts/task-21.md:437`) pins it.

Owner question recorded: whether Share turns team mode on, or leaves it to an explicit
**Turn on team agents** step. The plan builds "Share turns it on"; the admin-only button in the
popover (`parts/task-56.md:83`) is the seam for the other answer.

Deferred, out of this batch (not X-C1's defect):
- Route numbers 80 to 86 and contract §7.27 are each claimed by more than one task (80: tasks 14,
  52, 66; §7.27: tasks 14 and 66). Task 01's §7.0 should become the one registry; b01 only avoids
  adding a collision.
- `parts/task-33.md` calls the local control plane at `/api/...`, but its routes are `/v1/...`
  (`control-plane/internal/api/*.go`). `05-team-mode.sh` uses `/v1/`; the others are unchanged.

**Unverified:** no code exists for any of this yet; every file and test above is planned. The Hub
package paths (`server/`, `store/`, `cmd/hub/config.go`) are Hub tasks 13 to 17's, read as
planned, and Hub task 35's `hublink.Resolver` is planned.

### Fix batch b01, round 2 (2026-09-26): the re-review's open issues

Input: `.superpowers/sdd/team/plan-rereview-b01-1.md` (I-1 to I-8). Line numbers are those after
round 2's last commit. Round 2's inserts had moved 19 of round 1's numbers above (tasks 06, 08,
21, 45, 56 and the skeleton); they are re-anchored in place (I-3, pass 2).

| Issue | Fix | Where |
|---|---|---|
| I-1 (Critical): the Hub's project reads never return `mode` | Task 06 modifies Hub task 17's `store/projects.go`: `mode` in the one project-row reader that routes 13 to 16 share, and in route 14's `RETURNING`. New real-Hub test `TestProjectReadsCarryMode`: after route 87, routes 13 (`?repo=` and `?teamId=`), 15 and 16 answer `"mode":"team"`; route 14 answers `"solo"`, never `""`. The conformance row reads the mode back through routes 15 and 13, so the fake and the Hub are held to it together | `parts/task-06.md:53-59` (Files), `:604` (test), `:695-699` (Step 4), `:680` (run line), `:814-817` (acceptance 6); `parts/task-08.md:450` (row), `:397` (`SoloRepoKey`); skeleton scope 06 `:492-495` |
| I-2 (Important): `MakeTeam` leaves the S-LINK cache stale | `MakeTeam` calls `Resolver.Invalidate(localProjectID)` on the Hub's 200 only. `teammode_test.go` runs over the real `hublink.Resolver`, with the cache warmed before `MakeTeam`; the new `TestTeamModeFailureKeepsTheCache` pins "no invalidation on a refusal" | `parts/task-21.md:387-395`, `:74-75` (Consumes), `:440-448` (tests); `parts/task-56.md:88-90`; skeleton scope 21 `:656-658` |
| I-3 (Important): about 240 inbound citations drifted | Two passes, by script, in commits of their own. **Pass 1** (`scratchpad/team/recite.py`, difflib from `8d661b39^` to the tree after both rounds): 252 citations on lines b01 did not write, in 42 parts; a bare `:N` after a `task-NN.md:` citation on the same line moves with it. Left alone, each listed and read: 11 by a hand list, citations of another plan's same-numbered file (Hub `task-04.md:99`, `:928`, `task-05.md:592`, `task-45.md:104`; env `task-34.md:29`; phase 3 `task-59.md:32`, `:85`, `:122`, `:142`; phase 6 `task-66.md:23`, `:30`), and 22 by a marker (a path or "phase N" / "P6-06" within 120 characters). Four fuzzy ranges read by hand: `task-06.md:66` and `:107` now cite `task-01.md:234-248`; `task-46.md:45` is unchanged; `task-52.md:1195` cited task 21's old mode gap, which b01 closed, so its parenthetical now says task 21 resolves the id through S-LINK (`task-21.md:381-391`). **Pass 2** (`recite2.py`, `git blame` bounded to b01, then `git diff -U0` from each writing commit to the tree): citations on lines b01 itself wrote whose target a later b01 commit moved. Three in parts (`task-06.md:81` twice, into task 04; `task-06.md:137`, into task 01) and 19 in round 1's table above; one of those, `TeamModeAllowed`, set by hand to `task-45.md:401-405` (round 1 had cited it one line early; round 3's insert moved it five more). Every part keeps its line count (edits in place only) | the re-review's examples: `task-39.md:33`, `:150`, `:481` and `task-48.md:63` now cite `task-21.md:92`; `task-20.md:289` cites `:85-92`; `task-06.md:66` cites `task-01.md:234-248` |
| I-4 (Important): the harnesses go red when task 45 lands | Every harness that turns a project team sets both `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` and `PANACEA_APP_HUB_AUDIT_SINK=fake`. Task 45's Step 5 and acceptance run `go test -tags integration ./internal/hub/...` after the gate swap. Task 45's contract edits carry the rule into task 01's §12.2 row and §7.4 gate sentence | `parts/task-06.md:552-557`; `parts/task-08.md:414-418`; `parts/task-43.md:406-409`; `parts/task-33.md:79`; `parts/task-45.md:65-74`, `:659-662`, `:674-676`, acceptance `:692-693` (round 3's insert moved the last three five lines down, and widened the list) |
| I-5 (Important): `team-feed/team-projects-only` could not be set up | The fake gains `AppendEventForTest` and `Env` gains `AppendEvent` (on the real Hub: Hub task 14's `store.Append` under the head lock). The row writes the solo project's `task.created` that way, and the mode filter is still the thing tested. The clause stays as defence in depth | `parts/task-08.md:455` (row), `:332-336` (fake), `:402-408` (`Env`) |
| I-6 (Minor): other fake suites need a team project | `hubclient.FakeTeamProject(t)`: Ada creates the team (so she is its admin, `fake_projects.go:223-224`) and turns the project team through `SetProjectMode`. Task 11's delegation tests start there | `parts/task-08.md:338-343`; `parts/task-11.md:1135-1140` |
| I-7 (Minor): acceptance checks numbered 1, 2, 3, 5, 4 | Renumbered 1 to 6 (6 is I-1's new check) | `parts/task-06.md:801-817` |
| I-8 (Minor): `project_mode` taken as a literal pair | Built from `hubapi.ProjectModeSolo` and `ProjectModeTeam`, as the others are (now 20 from `hubapi`) | `parts/task-04.md:854` |
| The re-review's passing note: route 78's filter | The query is bracketed `((fleet_feed = true AND <the project is team>) OR kind = 'team.member_removed')`, because task 04's trigger leaves `fleet_feed` false on `team.member_removed`. Fixed here because this batch edits the same query's row. **Round 3 (I-B) replaced the shorthand with the join the plan means; the part is now authoritative and this row no longer paraphrases the SQL** | `parts/task-08.md:527-542` |

Commits, in order: `ecd3cda4` (task 06: I-1, I-4, I-7), `4c60e3c8` (task 04: I-8), `2eceac04`
(task 45: I-4), `109f8aa3` (task 08: I-1, I-4, I-5, I-6 and route 78), `d905e241` (task 11: I-6),
`697ed215` (task 21: I-2), `f9117270` (tasks 43, 33, 45: I-4), `a03a0bc9` (skeleton scopes 06 and
21), `76ea0754` (I-3 pass 1, 42 parts), `1a09c75e` (I-3 pass 2, task 06), then this section.

Deferred, out of this batch: task 52's gap G52-4 (`parts/task-52.md:1194`, the cockpit's `p-…` id
to the Hub's `pj-…`) may now close by reusing S-LINK's `Resolver` as task 21 does. Only its stale
parenthetical is fixed here; closing the gap is task 52's batch.

**Unverified:** every file and test in this table is planned; no Hub `server/` or `store/` code
exists yet, so I-1's missing `SELECT mode` is the re-review's inference, and the fix is a plan
instruction, not a diff. I-3's re-anchor trusts each citation to have been right when written; it
moved 274 numbers (252, 3 and 19) and 39 were read by hand: the 4 fuzzy ranges, the re-review's
6 examples, pass 2's 22, and 7 more whose target text was compared old against new.

### Fix batch b01, round 3 (2026-09-27): the re-review's open issues

Input: `.superpowers/sdd/team/plan-rereview-b01-2.md` (I-A Important, I-B and I-C Minor; it
confirmed S01-C1 stale and X-C1 resolved, and all eight of round 1's issues closed). Every
citation edit is in place; four parts gained lines (`task-44.md` +13, `task-08.md` +10,
`task-14.md` +8, `task-45.md` +5), so I-3's drift problem was checked, not assumed: every
`task-NN.md:` citation in the 69 parts, the skeleton, this review and `repair-gaps.md` pointing
below an insertion point was listed by grep. Eleven were affected, all of them in this review;
each was re-anchored by comparing the old line's text at `f4b6a7d5` against the new one, and
every comparison matched. The parts themselves had none.

| Issue | Fix | Where |
|---|---|---|
| I-A (Important): b01 made a delegation need a team project, and two suites that create delegations were never told | Task 44's test preamble now says P becomes `team` by **Grace calling route 87**, never by SQL, on a Hub started with `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` **and** `PANACEA_APP_HUB_AUDIT_SINK=fake` in `ENV=local`; S stays `solo` and takes no delegation; team U's project needs route 87 too. Task 14's `TestFleetCannotRecordAPolicyVersion` reuses task 06's `teamWorld` instead of minting a project (it already `dependsOn: 06`, skeleton `:325`), and the part says its other `hub ·` rows are desktop writes that need none. Task 45's harness list — the list its gate swap is proved against — now names six and is **declared closed**, so a seventh must add itself | `parts/task-44.md:310-321` and `:63`; `parts/task-14.md:709-715`; `parts/task-45.md:65-74` (list), `:675-676` (Step 5), `:693` (acceptance); skeleton scope 44 `2026-09-25-phase-t-team-mode.md:919` |
| I-B (Minor): route 78's filter was `project mode = 'team'`, two bare words, over a table with no `mode` | Written out as the join the plan means, over `audit_events a`: `((a.fleet_feed = true AND EXISTS (SELECT 1 FROM projects p WHERE p.id = a.project_id AND p.org_id = a.org_id AND p.mode = 'team')) OR a.kind = 'team.member_removed')`. Org-scoped because `projects` is unique on `(org_id, id)` (`hub task-07a.md:155`); the mode is task 04's column on `projects`, and `audit_events` carries only `project_id` (`hub task-07a.md:412-428`). Round 2's bracketing is kept unchanged, and the round-2 row above no longer paraphrases the SQL | `parts/task-08.md:527-542`; the round-2 row at `:3439` |
| I-C (Minor): two citations re-anchored faithfully but pointing at the wrong section | `task-06.md:66` and `:107` now give one citation per item, in the order the prose names them: §7.13 to §7.17 `task-01.md:329-400`, rows 46 to 59 `:259-272`, the class route sets `:301-310`, the §8 rows `:541-574`. Each target was read before the number was written | `parts/task-06.md:65-67`, `:107` |
| I-C, the same class elsewhere (found by grepping the shape, beyond what the re-review asked) | Thirteen more numbers in ten places, most pointing into §7.24 where §8 and §5.3 used to sit: task 06's §8 rows (`:487-496` → `:541-574`); task 07's, twice (`:497-504` → `:557-564`) and §5.2's org auditor (`:541-545` → `:587-591`); task 43's policy rows (`:531-532` → `:566-567`), §7.24 (`:475-497` → `:480-500`), routes 74/75/79 (`:285-290` → `:287-292`) and the §5.3 team table (`:547-597` → `:593-621`); task 44's `team.member_removed` (`:500` → `:512`); task 46's `note.created`, twice (`:563` → `:565`) and its two matrix rows (`:577` → `:617`, `:579` → `:620`). Each target was read before the number was written | `parts/task-06.md:317`; `parts/task-07.md:48`, `:251`; `parts/task-43.md:73-75`; `parts/task-44.md:63`; `parts/task-46.md:58-59`, `:224` |

S01-C1 and X-C1 were re-checked, not re-fixed: `parts/task-04.md` (963 lines) and `parts/task-05.md`
(879 lines) exist, so S01-C1 stays a stale finding answered in writing; route 87 is still the one
writer of `projects.mode = 'team'`, still admin-only, one-way and behind the audit gate
(`parts/task-06.md:210-215`, `:239`), and the three restatements still agree
(`parts/task-04.md:67`, `parts/task-08.md:607`, `parts/task-45.md:724`).

**Not fixed here, on purpose:** task 44 is now carrying `repair-gaps.md`'s gaps 7 and 12 as well as
I-A's harness sentence, and task 45 still declares gaps 9, 10 and 11 (phase 8 streaming, the
`azapi` provider, the public collection endpoint). Task 44 should be the next batch.

**Unverified:** as above, every file, test and line is planned text; no code exists. The I-C class
was found by grepping `§8 row`, `§7.24` and `§5.3` citations into `task-01.md` and reading each
target; parts that cite task 01 without naming a section were not swept, so the class may have
survivors.


## Fix batch b02 (2026-09-28): S02-C2, S02-C5

Both findings are the same defect twice: a Hub **read** that several tasks depend on and no task
owns. They are one batch because both land in task 01's amendment, task 06's handlers and task 08's
conformance. Neither was moot; both were live on 2026-09-28 (`grep` for "principals", `ownerTid`,
`Hub.DelegationReader` across the parts found only the two gap statements themselves).

### S02-C2 — the principal read

| Change | File:line |
|---|---|
| Amendment (b): **two** exemptions, and this is the second amendment its own text demanded | `task-01.md:29-39` |
| §5.10 rule 2: the second exempt route by name; a Fleet token naming a delegation there is 400 | `task-01.md:153-162` |
| §7.0 **row 88**, `GET /v1/principals?tid=&oid=&projectId=` | `task-01.md:346` |
| New **§7.28**, with rule 1 pinning `orgId` to the **project's** org and rule 3 the flags to that org | `task-01.md:587-626` |
| §4.2 code `principal_not_found`, sixteenth, appended last so nothing renumbers | `task-01.md:273` |
| §8 event `principal.read`, audit only, on neither feed | `task-01.md:684` |
| Acceptance: row 88, §7.28's two pinned rules, the event | `task-01.md:860-871` |
| Task 06 owns the handler, the one-statement store read and the three mutants | `task-06.md:761-777` |
| Task 06 registers it bare, Fleet-only | `task-06.md:732-737` |
| Task 06 acceptance item 7 | `task-06.md:876-880` |
| `Team.Principal`, no longer a proposal | `task-08.md:113-121` |
| `hubapi.Principal` and its file | `task-08.md:39-42` |
| Three conformance rows, and `Env` gains a **second org** without which they cannot be written | `task-08.md:491-493`, `:431-438` |
| Task 12: `dependsOn` 01, 06, 08; the gap paragraph becomes the owned route | `task-12.md:3-8`, `:47-64` |
| Task 12 Step 4 no longer says "stop here"; a stubbed `Membership` is named an authorisation bypass | `task-12.md:1192-1198` |
| Skeleton: task 01, 06, 08 and 12 scopes, and 12's `dependsOn` row | `2026-09-25-phase-t-team-mode.md:429-437`, `:502-515`, `:529-534`, `:572-576`, `:323` |

### S02-C5 — the broker as a caller of route 47

| Change | File:line |
|---|---|
| Amendment (e): the broker is a third identity, read-only, one route | `task-01.md:53-68` |
| §5.10 rule 1: two app-only callers, neither holding the other's role | `task-01.md:143-152` |
| §5.10 **rule 10** (not 9 — see the ruling below) | `task-01.md:197-208` |
| §5.10 rule 9 left **reserved and empty** for task 66's gateway caller | `task-01.md:193-196` |
| §7.0 row 47 gains the broker as a third caller | `task-01.md:312` |
| §7.13: `ownerTid`/`ownerOid`, to that caller alone, `omitempty` for every other | `task-01.md:399-404` |
| §12.2 `PANACEA_APP_HUB_BROKER_CLIENT_ID` | `task-01.md:293` |
| §8 event `delegation.read_by_broker`, actor `""` | `task-01.md:685` |
| §7.25 restated: the gateway's rule stays 9 | `task-01.md:575-578` |
| Task 06 owns the branch and its three mutants | `task-06.md:778-794` |
| Task 06 acceptance item 8 | `task-06.md:881-886` |
| Task 08: `AsBroker()`, `FakeBroker`, two conformance rows, the `Broker` env view | `task-08.md:331-341`, `:494-495`, `:432-435` |
| Task 17: gap S17-1 marked **CLOSED**, decision 3 restated, `dependsOn` gains 06 | `task-17.md:394-406`, `:48-57`, `:3-5` |
| Tasks 20 and 60, which both leaned on S17-1 | `task-20.md:62-63`, `:537-538`; `task-60.md:376-379`, `:465-466` |
| Skeleton: task 01, 06 and 17 scopes, and 17's `dependsOn` row | `2026-09-25-phase-t-team-mode.md:429-437`, `:502-515`, `:635-640`, `:328` |

### Two rulings this batch had to make, and declares

1. **Row 88 and §7.28, not the row 80 and §7.27 the finding proposed.** Rows 80 to 86 and §7.27 are
   all already claimed: 80 and §7.27 by task 14 (`task-14.md:620`), 81 to 83 by task 46
   (`task-46.md:109-110`), 84 to 86 by task 49 (`task-49.md:137,150,156`). Recorded in the note
   under §7.0's table (`task-01.md:349-357`).
2. **The broker is §5.10 rule 10, and rule 9 is reserved.** The finding said "rule 9", but task 66
   has already drafted rule 9's contract text, its `auth.go` hunk and ~22 references for the
   gateway caller. Renumbering a drafted rule across a part outside this batch is the churn this
   program keeps paying for, so b02 took 10 and left 9 empty with a reason and an acceptance check
   (`RULE 9 NOT RESERVED`).

### Found while fixing, recorded and NOT resolved (not this batch's findings)

**Route 80 is claimed three times and §7.27 twice.** Task 14 takes `POST
/v1/orgs/{orgId}/policy-versions` as row 80 in a new §7.27 (`task-14.md:620`); task 66 takes
`GET /v1/orgs/{orgId}/quotas` as row 80 in a §7.27 of its own (`task-66.md:156`) **and** row 81,
which task 46 also takes (`task-46.md:109`); task 52 proposes row 80 for a third thing
(`task-52.md:1188`). Whichever lands second must take a free row (89 onward) and say so. b02 only
made sure it did not add a fourth claimant.

**Unverified:** every file, test and line named above is planned text; no code exists on
`feat/team-mode` for any of it. The line numbers are this commit's and were read on 2026-09-28.

### Round 1 re-review (`.superpowers/sdd/team/plan-rereview-b02-1.md`), closed in round 2

Its root was one thing: b02 created a **second app-only identity** and amended no gate, so the
broker was refused 401 `delegation_required` before route 47 (C1) and admitted as Fleet on route
78's org-wide feed (C2). The task that adds an identity now owns every check that read `AppOnly`
and meant "Fleet".

| Round-1 finding | Change | File:line |
|---|---|---|
| C1 — no task owns the gate that admits the broker | `auth.go`'s deltas are **four**, not two: `Claims.AppRole` + `Claims.BrokerRead`, `isExemptPath`, `delegationHeader`'s role arms, `resolveCaller`'s broker branch — with the two hunks written out | `parts/task-06.md:73-89`, `:813-887` |
| C1 (restated) | task 05 says `AppOnly` means Fleet **only in task 05**, names the four deltas task 06 owns, and forbids a second `entra.AppCaller` there | `parts/task-05.md:47-57` |
| C2 — every `AppOnly` check still meant "Fleet" | `delegationHeader`'s exempt-path cases narrowed to `AppRoleFleet`; a third, ungated role is 403, never admitted as Fleet | `parts/task-06.md:84-86`, `:841-845` |
| C2 (restated) | route 78's caller rule is `c.AppRole == hubapi.AppRoleFleet`, "**not merely `AppOnly`**" | `parts/task-08.md:291-296` |
| I1 — §4.2 contradicted §7.28 rules 3 and 4 | the `principal_not_found` row now restates rule 4; another org's user is 200 with three false flags | `parts/task-01.md:275` |
| I2 — the positive half had no runnable witness | both halves of `team/principal-flags-are-the-projects-org` ask about `ProjectID`; the true flag is `Env.Auditor`, whose standing is in the project's own org | `parts/task-08.md:499` |
| I3 — the b02 citations were not this commit's | re-derived by grep after round 1's own edits, at every site that copied the drift | `parts/task-05.md`, `-06`, `-17`, `-20`, `-60`; §7.13's broker fields are `task-01.md:401-406` |
| I4 — repair-gap 2 open in a part b02 amended | task 17 named `entra/verify_test.go`, which the built package has not; both citations now name `appcaller_test.go` | `parts/task-17.md:110-112`, `:242` |
| Minor 1 — "five" `Team.Principal` rows | three (`:498-500`); the broker's two are `:501-502`; 38 altogether | `parts/task-12.md:60-62` |
| Minor 2 — "eight" b02 acceptance lines | nine new, plus the modified `NOT 15 TEAM SECTIONS` | `parts/task-01.md:900` |
| Minor 3 — bare `"Hub.DelegationReader"` literals | `hubapi.AppRoleDelegationReader` beside the three task 03 already defines | `parts/task-03.md:367-370` |
| Minor 4 — "task 05's **one** exempt route" | two since b02 | `parts/task-08.md:291-292` |

### Found in round 2 while checking round 1's fix, and fixed with it

1. **The hunk said it replaced Fleet's 401.** `// Replaces … and its `case len(vals) == 0``
   would delete `delegation_required` and fall a headerless Fleet request through to `vals[0]`,
   which panics on an empty slice. The comment now places the hunk **before** that case and says
   every case from it down is kept unedited (`parts/task-06.md:819-828`).
2. **An empty case body is not an elision in Go.** `case isExemptPath(r) && len(vals) > 0:` was
   written as a comment with no body, which falls out of the switch to `c.DelegationID = vals[0]`
   and serves the feed **under** a delegation. Written out (`parts/task-06.md:846-856`).
3. **Two citations drifted again inside round 1's own edits**: `task-08.md:202-203` for the
   exempt-route 400 (that is `TaskQuery.Values`; the rule is `:298`, the row `:500`) and
   `task-06.md:800-856` for the hunks (`:813-887` after the later edits of the same round), both
   corrected at `parts/task-05.md:42-43` and `:51-53`.
4. **`isPrincipalPath` was named but never bounded.** Spelled out as `GET` and segments exactly
   `v1`, `principals`, never a prefix (`parts/task-06.md:80-81`) — the same exactness `isBrokerPath`
   carries, and the hole C2 was made of.

**Unverified, round 2:** still planned text only. The line numbers above were re-derived by grep
after this round's edits; any later edit moves them, so grep the quoted heading or sentence rather
than trust the range.

## Fix batch b03 (2026-09-28): S02-C1

**S02-C1 (Critical) — fixed at the root, not at the symptom.** The defect was not that `Adopt`
forgot a step. It was that two maps which must describe the *fleet* — `Feed.cursor` and the
`Cache` — were one replica's memory, filled only by whichever replica happened to serve the
launch. Both are now read from the one state the replicas share, Fleet's own store.

Line numbers in this table were **re-derived at round 2** (below), after the round-2 edits moved
every one of them; four were also one line short at the start and one named the wrong row, which
was round 1's M1. Grep the quoted name, not the range.

| Change | Where |
|---|---|
| `Feed` holds task 10's `InUse`; `NewFeed(hub, team, in, cache, rev, leader, now)` | `task-11.md:664,676-680` |
| `Feed.Refresh` — every tick, both replicas: read `DelegationsInUse`, add unknown orgs head-first, fetch unknown delegations under themselves | `task-11.md:1036-1075` |
| `Feed.Resolve` — the read-through the outbox asks on a cache miss | `task-11.md:1096-1148` |
| `delegation.Source`, `NewClient(cache, src, now)`, and `Ctx`'s one lookup before it refuses | `task-11.md:498-525` |
| `Cache.Known(id)` — a delegation waiting for its sweep is read once, not once a tick | `task-11.md:460-467` |
| `Rebuild(ctx, *Feed, *HubState)` — reads the Feed's own store and clients, so rebuild and refresh cannot be pointed at two different ones | `task-11.md:908-916` |
| Why, in prose, including the outbox and the sweep consequences the finding named | `task-11.md:1175-1202` |
| `Adopt` is the serving replica's fast path, not the only path | `task-11.md:1006-1008`, `:1197` |
| `Sweep` now covers the whole fleet, stated | `task-11.md:750-754` |
| Test harness: `memInUse`, and `NewClient(cache, nil, clk.Now)` for the cache-only tests | `task-11.md:1407-1414` |
| The two-`Feed` test's row (round 1's M1: the old range named `TestRebuildPersistsNothing`, which is not b03's) | `task-11.md:1431` |
| `TestASecondFeedSeesADelegationAdoptedOnTheFirst`, complete: one `memInUse`, one fake Hub, A adopts and is follower, B is leader and holds the outbox; B resolves the key with no poll, B calls `Revoke(Revoked)` within 60 s, A calls nothing | `task-11.md:1486-1533` |
| The boundary, written down: task 20 writes the agent row and that launch's first outbox row in one transaction (`task-20.md:395-400`) | `task-11.md:1572-1577` |
| `fleetd` builds the feed *before* the outbox and passes it as the `Client`'s `Source`; `NewFeed` takes `delegation.StoreInUse{S: st}`; `rebuild` is `Rebuild(c, feed, state)` | `task-12.md:878-894` |
| `readFeed` refreshes before it polls, and logs rather than stops when the store read fails | `task-12.md:904-907,923-926` |
| The two-replica narrative, and the provides list | `task-12.md:38-55`, `:109` |
| Task 39's "every replica's `Cache` drops a revoked delegation within one poll" now has a reason, and the spurious-close hazard a per-replica cache would have created is named | `task-39.md:46-57` |

**Why `Refresh` and not a new `delegation_use` row.** The finding offered either. `DelegationsInUse`
already exists, is already shared, and is already the thing `Rebuild` trusts after a leader change
(`task-11.md:908-916`); a second row would be a second truth to keep in step with the agent rows.
The cost is one store query per replica per 10 s tick and no Hub call when nothing changed
(`TestRefreshReadsOnlyTheDelegationsItDoesNotKnow`).

**Deliberately not fixed here.** The Hub, not Fleet's store, stays the authority on whether a
delegation is live: `Refresh` re-reads every unknown id under itself, so a revoked or lapsed one is
acted on rather than resurrected, and `Resolve` re-reads the one id it was given and calls a 401
a miss (`TestResolveNeverInventsALapsedDelegation`; round 2 removed that test's assertion that
`Resolve` calls the `Revoker`, because `Resolve` no longer acts at all). What a *terminal*
write does when its own delegation has just been revoked is S02-I1's, untouched: `Resolve` returns
"none", `Classify` still calls that Permanent, and the row still dies. b03 only makes
`ErrNoDelegation` mean "shared state has none either" instead of "this replica has not heard yet",
which is the precondition I1's fix needs.

**Observed while here, recorded not charged.** The complete 60 s test builds
`hubclient.NewFake(hubclient.WithClock(clk.Now))` (`task-11.md:1449`) while the paragraph above it
requires `hubclient.FakeTeamProject(t)`, never a bare `NewFake` (`:1401`), and task 08's signature
takes no options and returns two values (`task-08.md:377`). It pre-dates b03; the new test uses
`FakeTeamProject`. Whoever closes S02-I1 or the b01 fake work should reconcile that one line.

**Unverified, b03:** planned text only — no Go compiles, nothing runs. Line numbers were
re-derived by grep after these edits; grep the quoted name rather than trust a range.

## Fix batch b03 round 2 (2026-09-28): the re-review's six

Round 1's re-review (`.superpowers/sdd/team/plan-rereview-b03-1.md`) called b03 "Needs fixes":
two critical, two important, two minor. All six are answered below at their root, and the root
turned out to be one mistake made twice — **one store read was being asked two different
questions**.

**The root.** `DelegationsInUse` is live-agents-only by design: it is the set the feed *acts* on,
and a delegation no live agent runs under has nothing to stop and must never be resumed
(`task-10.md:479`, `DelegationsInUseLiveOnly`). b03 then handed that same read to the outbox's
`Source`. But the outbox's question is not "what should I sweep", it is "what did this queued row
go out under" — and that is routinely an **ended** agent's delegation, because `Stop` ends the
agent and enqueues `agents.end` in one breath (`task-20.md:188`, `:340`) and a restart after an
outage delivers a backlog whose agents have all ended. So the two callers now take two reads.

| Finding | Fix | Where |
|---|---|---|
| **C1** an ended agent's last write, and any restart backlog, still dead-letters on the replica that never cached the delegation | `Store.DelegationForKey` — the latest agent row on a key, **ended rows included** — with the index that serves it, the `StoreInUse` adapter and three store test rows; `Feed.Resolve` reads it instead of `DelegationsInUse` | `task-10.md:101-108`, `:258-261`, `:480-482`; `task-11.md:861-868`, `:1096-1148`, `:1293-1299`, `:1434` |
| **C2** `Ctx`'s `Source` error was dead-lettered while its comment promised a retry | `outbox.ErrTransientLocal` added to `Classify`'s transient set; `delegation.ErrResolveUnavailable` wraps it and every `Resolve` failure wraps that. The sentinel lives in `outbox` because `delegation` imports `outbox` (`Sender` takes an `outbox.Item`) and the reverse would cycle | `task-11.md:240-266`, `:408-417`, `:498-516`, `:529-540` (`Ctx`'s comment now matches what `Classify` does) |
| **I1** `Resolve` ran a whole `Refresh`, so the 500 ms delivery tick could reach `Revoke`/`Resume` | `Resolve` is now two reads and a `Put`: one store row, at most one Hub read of that one id, no `rebuildDelegations`, no `f.act`. A 401 is a miss, not an act | `task-11.md:1096-1148` |
| **I2** `Refresh`'s tracked-check and `SetCursor` were check-then-act on two goroutines | `SetCursorIfAbsent` tests and assigns under one lock, and **both** `Refresh` and `Adopt` use it — the race has two orders and the first pass fixed only one (see the correction below). `SetCursor` is now Rebuild's alone, and its comment says why that reason does not travel | `task-11.md:809-833`, `:1017-1022`, `:1061-1066`, `:1439` |
| **M1** four b03 citations one line short, one naming the wrong row | every "Where" in the b03 table re-derived after the round-2 edits, with the two-`Feed` test's row given its own line (`:1431`) instead of a range ending on `TestRebuildPersistsNothing` | `review.md:3612-3635` |
| **M2** nothing pruned `Feed.cursor`, so the poll list grew with every org either replica ever served | `pruneOrgs` drops a cursor when the in-use read no longer lists the org **and** the cache holds no entry for it; the cache clause is what protects the window between `Adopt` and task 20's commit, and is why task 39's socket sweep is unaffected | `task-11.md:1077-1094`, `task-39.md:51-53` |

**Restated, by grep, not by assumption.** `task-11.md:1175-1202` (which read each caller takes and
why), `:1564-1584` (the "does not claim" bullet that was carrying the `Revoker`'s claim and the
`Source`'s in one sentence, now three bullets), `task-12.md:42-55` and `:885-887` (the two paths,
and why sharing one `*Feed` between two goroutines is safe only while `Resolve` is a read),
`task-12.md:109` (Consumes), `task-39.md:51-53`, and the plan body's task 11 summary
(`2026-09-25-phase-t-team-mode.md:561-578`), whose 11-minute scope test now says "ended agents and
all" — the case it was silently failing.

**Six test rows added, one corrected** (`task-11.md:1434-1439`, `:1433`):
`TestAnEndedAgentsLastWriteResolvesOnTheOtherReplica`, `TestARestartBacklogOfEndedAgentsDelivers`,
`TestAStoreFailureInResolveLeavesTheRowPending`, `TestResolveNeverCallsTheRevoker`,
`TestRefreshPrunesAnOrgItNoLongerNeeds`, `TestConcurrentFirstTrackKeepsTheOlderCursor`; and
`TestResolveNeverInventsALapsedDelegation` no longer asserts a `Revoker` call, because `Resolve`
no longer makes one. `memInUse` (`task-11.md:1407-1414`) now serves both of task 10's reads as
task 10 defines them, counts them separately, and can fail one on demand.

**One correction, found re-checking this batch's own work.** I2's first pass put
`SetCursorIfAbsent` in `Refresh` and left `Adopt` on the unconditional `SetCursor`, justified by
"`Adopt` reads the head before the state". That justification is `Rebuild`'s and does not travel:
`Rebuild` re-reads the **whole** org state after its head, so a forward jump skips only what the
re-read has already applied, while `Adopt` reads **one** delegation record and re-reads nothing. So
with `Refresh` tracking the org first and `Adopt`'s racing write landing second, every event between
the two heads was still skipped — the same hole entered from the other side, and
`TestConcurrentFirstTrackKeepsTheOlderCursor` already asserted the older cursor wins, so the part's
code and its own test disagreed. Fixed at the root: `Adopt` uses `SetCursorIfAbsent` too
(`task-11.md:1017-1022`), `SetCursor`'s comment says it is `Rebuild`'s alone and why
(`:809-813`), and the test runs both orders (`:1439`). Unrelated but adjacent: `review.md:875`
separately asks `Rebuild` to keep an existing cursor on leader acquisition — that finding is not
b03's and `Rebuild` is deliberately untouched here.

**Still open, and named rather than hidden.** S02-I1 is untouched: a *terminal* write whose own
delegation has just been revoked resolves to nothing, `Classify` calls that Permanent, and the row
dies. What b03 round 1 could not honestly claim, round 2 can: `ErrNoDelegation` now means "no
agent row on this key names a live delegation", which is the precondition I1's fix needs. Round
1's own note stands too — the complete 60 s test still builds a bare `NewFake` (`task-11.md:1449`)
against the paragraph at `:1401`; it pre-dates b03 and belongs to S02-I1 or the b01 fake work.

**Unverified, b03 round 2:** planned text only. No Go compiled, no test run, no control plane,
container or image started, in line with this stream's worktree rules. Every line number above was
re-derived by `grep -n` against the worktree after the round-2 commits; ranges drift, so grep the
quoted name. That C1 and C2 would have failed at runtime remains reasoning about planned code, not
a measurement.
