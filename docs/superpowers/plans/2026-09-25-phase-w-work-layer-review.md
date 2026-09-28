# Phase W plan review — the work layer (Linear parity-plus)

**Date:** 2026-09-28
**Plan under review:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` and its 187 parts
in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-NN.md`, with the scope files in
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/`.

## How this review was done

Adversarial, read-only, on `feat/work-layer`, in twenty-one passes assembled here unedited:

1. **Nineteen section reviews**, one per ten tasks (section 01 = tasks 01-10 … section 19 = tasks
   181-187). Each reviewer read its parts against the matching scope file and the skeleton, and
   ends with its own `Section verdict:` line. All nineteen exist and all nineteen say *Needs fixes*.
2. **One cross-cutting review** (`sec-00`) of what no section reviewer can see: coverage of THE
   BRIEF and of every slice of `05-design.md` revision 2, the 257-row parity matrix in
   `coverage.md`, dependency order across the whole plan, fit with the sibling plans (Hub, phases
   3, 4, 5, 6, 8, Phase T), and security across the plan. It is placed first below.
3. **One OD-30 identity review** (`sec-90-identity`), covering the Keycloak-broker ruling against
   every Phase W task that touches sign-in, tokens or the Work MCP server. It is placed last.

This assembly judges nothing new. Severities, wording and "how to fix" text are each reviewer's.

## Counts

| Severity | Findings |
| --- | --- |
| Critical | 20 |
| Important | 153 |
| Minor | 158 |
| **Total** | **331** |

One further heading, `S19-M4`, was withdrawn by its own reviewer on verification and is not counted.

## The two defects a first implementer would hit

1. **Which table do I obey on day one (X-C2).** The streams table gates stream WH on Hub task 43;
   task 02's row gates it on `H01-H03`, which are on `feat/hub` today. One reading starts Phase W
   this week, the other after the whole Hub plan lands, and the **WS0** start line only holds under
   the second. The plan does not resolve it.
2. **Who owns `useWork` (X-I2).** The seams table hands SW.16 to tasks 55-58, but `task-40.md:111`
   already writes its read half into `desktop/src/work/source.ts` — a file in no file-map row — and
   `task-55.md:56` re-exports it from there. Task 55 taken from the skeleton writes a second
   `useWork` and collides with task 40 at merge.

---
# sec-00: cross-cutting adversarial review of Phase W (the work layer)

Scope: what no section reviewer can see. Coverage of THE BRIEF and of every slice of
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2); the coverage map
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/coverage.md` against all 257 parity
rows; dependency order across the whole plan; fit with the sibling plans (the Hub, phases 3, 6 and
8, and Phase T); and security across the plan. OD-30 identity is sec-90-identity.md and is not
repeated here.

Read on `feat/work-layer`. All 187 parts exist on the tree at this reading, so every
"no part on this tree" claim in the section reviews is stale.

## Findings

### X-C1 (Critical): the Design lens is the one slice of 05 with no task, no delta row and no `P5-` dependency, so the provenance thread breaks at Design

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:7`
**Problem:** The plan's own Goal states the thread as *brief → criterion → **design frame** → issue →
session → land → evidence → release*. `05-design.md:1082` fixes what that link is: *"a frame shows
the issues whose revision `implements` it"*, and *"a design approval can close a design issue
(04 §7.7)"*. Every other lens in that table has a delta row in *Deltas on sibling code*: Discover and
Develop (phase 4, tasks 18, 19, 67), Merge (phase 3, tasks 21-23, 67, 104), Deploy (phase 7, tasks
71, 131). **Design has none.** `grep -n 'design frame' ` over the skeleton returns exactly one hit,
line 7; `grep -n 'P5-'` returns none; no scope paragraph names a frame, an `implements` relation or
a design approval. `.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md`
exists on the tree, so this is not a "plan not yet written" case. CLAUDE.md's product spine makes
this the load-bearing one: *"the spec written in Define is visibly referenced in Design, carried into
Develop"* — an issue that cannot be reached from a frame breaks the thread at exactly that hop.
**How to fix:** Add a phase 5 row to *Deltas on sibling code* and one or two W2 tasks: the
`implements` relation between an issue revision and a canvas frame key (a new relation kind in task
11's set, or a second `serves`-like field on the revision, decided in SW.1 not invented later), the
Design-lens list of issues per frame, and the design-approval → design-issue close path from 04 §7.7.
If the owner instead rules Design out of Phase W, say so in the skeleton and strike "design frame"
from the Goal sentence, so the thread the plan claims is the thread it builds.

### X-C2 (Critical): the WH stream gate says "after Hub task 43", every WH task's `dependsOn` says otherwise, and the start-line WS0 is impossible under the gate

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:122`
**Problem:** The streams table gates stream WH (tasks 02-17) on *"Hub task 43 merged into `feat/hub`,
synced here (rule H1)"* — the Hub plan's **last** task. The task rows gate the same tasks far
earlier: 02 on `H01-H03`, 04 on `H04-H05`, 06 on `H06-H07b`, 08 on `H14, H17`, 17 on `H43`. The two
cannot both hold. The consequence is not cosmetic: the **WS0** start line promises that tasks 01-05
(*"about 13 h"*) unblock streams WP, WD1 and WD2, i.e. the whole desktop and both sibling-delta
streams. Under the H43 gate WS0 opens only after the entire Hub plan lands, and Phase W's 516 hours
serialise behind it. `feat/hub` is at Hub task 07 today (`.claude/worktrees/hub`, `cafe1303`,
`control-plane/internal/hub/` holds `hubapi hubclient hubtest migrate` and no `server/` or `store/`),
so an implementer starting now gets opposite answers from the two tables on day one.
**How to fix:** Make the streams table's *Starts after* cell say what the rows say — "the Hub tasks
each WH row names, sync-merged here (rule H1); no WH task waits for H43 except task 17" — and state
in WS0 that tasks 02-05 need `H01-H05` only, which are on `feat/hub` today. Then re-check the
per-row `H` ids against the Hub plan's list at `2026-09-23-phase-h-hub.md:396-446`, because they
were written when nothing was built and none has been verified since.

### X-C3 (Critical): two Gaps rows are discharged to Phase T, whose plan contains neither of them

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/coverage.md:319`
**Problem:** The Gaps list closes **L-121** (*real-time multiplayer editing with cursors*, MATCH) and
**L-155** (*agent chat with full workspace context, `Cmd/Ctrl+J`*, EXCEED) with *"05 stop: TM"*, and
the map's closing paragraph then concludes *"under `OD-26` no Phase W task is missing, provided 05's
stops hold"*. The stop does not hold. Over
`.claude/worktrees/team/docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`, a case-insensitive
grep for `multiplayer`, `cursors`, `Ctrl+J` and `Cmd/Ctrl` returns **nothing**. Phase T owns neither
row. `OD-26` commits all 230 non-SKIP rows, so two of them currently belong to no plan in the
programme, and the map records them as covered. L-121 is the more expensive of the two: multiplayer
editing with cursors is a CRDT or OT decision that has to be made before the editor lands (tasks
49-50), not after.
**How to fix:** For each row, do one of three things and record which: (a) add a Phase W task —
L-155 is nearly free next to task 59's palette and W3's guidance, L-121 is not; (b) get Phase T to
add it, and cite the Phase T task id in the map, not the stop word `TM`; or (c) take it to the owner
as a SKIP or a *later*, which changes the 27/230 counts and 05 §1's recount with it. Then change the
map's method so a stop naming another plan must cite **that plan's task id**, and a stop that cannot
be cited is a gap, not a discharge.

### X-C4 (Critical): three parity rows and the "W1 in the cloud" start line are gated on phase 8, which has no plan and no worktree

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:368`
**Problem:** The start-lines table's third row makes W1 in the cloud wait on *"phase 6 stream A and
phase 8 stream C"*, and the Gaps list sends **L-164** (per-repo environments), **L-240** (data
regions) and **L-242** (IP restrictions) to *phase 8*. `ls .claude/worktrees` returns
`env hub phase-3 phase-4 phase-5 phase-6 phase-7 sandbox-git-fix team work` — there is no phase-8
worktree, and no `*phase-8*` plan file anywhere under `docs/superpowers/plans/` in any tree. Phase 6
by contrast now has a plan (`2026-09-25-phase-6-gateway.md`, tasks 01-24), so the asymmetry is real
and not a snapshot artefact. The effect is that the product's headline delivery claim — Phase W's W1
demo moving from local mode to the cloud — rests on work nobody has planned, and three `OD-26` rows
are recorded closed against it.
**How to fix:** Two separate fixes. (1) In the start-lines table, replace *"phase 8 stream C"* with
"phase 8, **not yet planned at this writing**" and mark the row UNVERIFIED, as the plan does honestly
elsewhere for `P6-`. (2) In the map, the three rows are gaps until a phase 8 plan exists and names
their tasks; list them under Gaps as *no owning plan*, not as owned. Neither fix is a Phase W task —
this is a finding the orchestrator has to carry to the owner, because it is the only place in the
programme where the cloud path is missing.

### X-I1 (Important): the four `exists / later` rows have no *later* half anywhere, and WL — the stream that exists for 05's *later* rows — does not carry them

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/coverage.md:318`
**Problem:** Four Gaps rows carry a split stop: **L-120** (documents on projects, teams and
initiatives — *"free documents `dc-` … come later"*, `05-design.md:264`), **L-231** (desktop apps
with tabs — *"the same code builds for macOS and Linux … **UNVERIFIED**: neither build has been made.
Tabs later"*, `:408`), **L-236** (roles — *"guest later"*, `:418`) and **L-237** (SSO — *"SAML for
other IdPs later"*, `:419`). The map counts all four as discharged by *exists*. But the WL stream was
created for precisely these leftovers: *"05 §1 marks twelve MATCH or EXCEED items later … `OD-26`
commits all 257 items less the 27 skips, so each has a task here"* (skeleton line 598). WL's twelve
rows are L-7, L-40, L-152, L-154, L-171, L-172, L-176, L-206, L-209, L-218, L-238, L-254 — **none of
these four**. So the *later* half of each is committed by `OD-26`, claimed by WL's own sentence, and
built by nothing. L-231 is the sharpest: the map reads it as *exists*, and 05 says in the same cell
that neither the macOS nor the Linux build *"has been made"*.
**How to fix:** Either add four WL tasks (a `dc-` document object; a macOS/Linux build proof and the
tab shell; the guest role in the authorisation matrix; a SAML IdP behind the Keycloak broker — note
this last one touches `OD-30`, so coordinate with sec-90 rather than duplicating it), or take the
four to the owner as explicit deferrals recorded in the map with a date. What cannot stand is a stop
word that reads as coverage while the sentence it cites says the work has not been done.

### X-I2 (Important): SW.16's read half is owned by task 40, not 55-58, and neither of its two files is in the file map

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:390`
**Problem:** The seams table says SW.16 — *"the replica: `useWork(selector)`, `mutate(op) → {undo}`,
`onRefused`, `search(q)`"* — is **defined in 55-58** and consumed by 40-69. The parts say otherwise.
`task-40.md:111` creates `desktop/src/work/source.ts` holding `WorkReader`, `useWork`,
`setWorkSource` and `sameData`, and `task-55.md:56` confirms it: *"`useWork` is task 40's
(`source.ts:202-223`), re-exported from `replica.ts` as SW.16 names it"*. So the read half of the
seam is defined 15 tasks before its declared owner, in a file the plan's **File map** never lists —
`grep -n 'src/work/source' ` over the skeleton returns nothing, and `replicaMount.ts`
(`task-55.md:72`) is missing from it too. The file map's only replica row is
`src/work/replica/{replica,ops,undo,visibility,search}.ts` at tasks 55-58, and that row carries no ⚑,
so a parallel-worktree reader is told these files have a single owner when two tasks write them.
This is the second defect a first implementer hits: building task 55 from the seams table, they
write their own `useWork` in `replica.ts`, and task 40's `source.ts` then collides with it. (The
**write** half of the same seam is sec-06's S06-C2; this finding is the skeleton's two tables, which
no section reviewer reads.)
**How to fix:** Change the SW.16 row to *"defined in 40 (the read half, `work/source.ts`) and 55-58
(the replica, the write half)"*, add `src/work/source.ts` ⚑ (tasks 40, 55) and
`src/work/replica/replicaMount.ts` (task 55) to the file map, and put `40` in front of `55-58`
wherever SW.16's owner is named. Then re-check task 40's `dependsOn` — it names 39 only, which is
right once the read half is its own, and wrong the moment anyone "corrects" it back to 55.

### X-I3 (Important): SW.3 is frozen at task 07 and then amended by four later slices, and unlike SW.1 and SW.2 it has no re-open rule

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:377`
**Problem:** SW.3 fixes *"who may write what: the authorisation rows and the guards (fact states,
`done`, the sync principal)"* in task 07, and lists consumers `10, 22, 23, 99, 145, 150, 159` —
four of them in W3, W4 and W5. The **WS0** start line gives SW.1 and SW.2 an explicit amendment
rule (*"a later change that is not an added method, field or route reopens task 05 and is
announced"*). SW.3 has none, and the Global constraint that would cover it is narrower than the
problem: *"every new route gets its authorisation-matrix rows in the same task"* covers a **new
route**, not a **new principal class** against the routes W1 already froze. W3 adds the agent
principal (99), W5 adds the integration/sync principal (150), the app-and-key principal (159) and
the inbound-mail requester (145-149) — each of which needs a verdict on **every W1 route**, not just
its own. The section reviews already show the damage this causes as local contradictions: S15-I3
(*"app and key may make hand state moves here and in task 144, and may not in task 160"*) and
S16-I9 (*"`TestAppCannotCloseOrFakeAFact` expects an app to move a state, which task 145's guard
refuses outright"*). Those are two symptoms of one missing rule.
**How to fix:** Give SW.3 the same sentence SW.1 and SW.2 have, with one addition: *adding a
principal class reopens task 07*, whose matrix test must then carry a row for the new class against
**every** Work route, defaulting to refuse. Make `server/matrix_integration_test.go` ⚑ say so (it is
already marked shared and owned by *"07 and every WH route task"*, which is the right hook), and
have tasks 99, 145, 150 and 159 each state in their steps that they add a full column, not a row.

### X-I4 (Important): the skeleton's sibling-state claims are stale, and task 135 ships with no dependency at all

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:554`
**Problem:** Task 135's `dependsOn` is *"24; P6 (UNVERIFIED id)"*, justified at line 356 by *"phase 6
… its plan is still being written, so every `P6-` id is **UNVERIFIED**"*. That is no longer true:
`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md` is on the tree, with
tasks 01-24 and named owners for exactly what task 135 needs — the minted session key and its
metadata (P6-D6: *"`metadata` = `{tid, oid, team, project, agent, session, route}`"*, tasks 06-07),
the route source (task 15) and the budgets (tasks 13, 21-22). Task 135 wants the token to carry the
**issue id**, which means one more key in that metadata map and a matching ask on phase 6 — and with
no id in `dependsOn`, no one on either side is holding that thread. Two neighbouring claims are stale
the same way: the File map's *"only `hubapi/` and `hubclient/` exist there"* (the hub tree also has
`migrate/` and `hubtest/` today), and the streams table's phase-6 citation. `OD-27`'s "no
predictions" and CLAUDE.md's *"state what is unverified"* both point the same way — an UNVERIFIED
mark is honest only while the check is impossible.
**How to fix:** Re-read the phase 6 plan's task list, put real `P6-` ids in task 135's `dependsOn`
and in the *Deltas on sibling code* row, and add the issue-id key to phase 6's metadata map as an
explicit **ask** (phase 6 already uses that device: *"asks H-1, H-2, P7-1"*). Refresh the two stale
sentences about the hub tree while you are there. Then sweep the plan once for `UNVERIFIED` and split
it into "cannot be checked" and "has not been checked" — sections 14, 15 and 16 each found the second
kind (S14-I10, S15-M7, S16-M1), so this is a plan-wide habit, not four accidents.

### X-I5 (Important): ten Gaps rows are discharged by the word *exists*, and the things they say exist are contract text and unbuilt plans

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/coverage.md:308`
**Problem:** The map closes the Gaps list with *"under `OD-26` **no Phase W task is missing**,
provided 05's stops hold. What still needs proof is that the `exists` rows really exist (UNVERIFIED
here)"* — and then counts them as covered anyway. Ten rows rest on that word: L-1, L-5, L-123,
L-125, L-143, L-167, L-179, L-236, L-237, L-243. Checked against the tree, *exists* means three
different things, only one of which is code. L-1 and L-5 cite `hub-contract:324-333` — a **contract**,
and the Hub service that would implement it is not written (`.claude/worktrees/hub/control-plane/internal/hub/`
holds `hubapi hubclient hubtest migrate`; no `server/`, no `store/`; `feat/hub` is at Hub task 07 of
43). L-143 cites *"Phase 7 **deploys**"*, a plan in flight. L-123 is the sharpest: the Linear row is
*version history **with revert***, and 05's answer (`:267`) offers only *"Brief revisions are already
immutable, digested and keyed"* — immutability is not revert, and the only revision **restore** in
the programme is the one Phase W builds for issues in task 09. So the map's own caveat is the
finding: under `OD-26` and the brief's *"every MATCH, EXCEED and PARTIAL item maps to a task"*, ten
rows are discharged on a claim the map declines to make.
**How to fix:** Replace the stop word with a citation, row by row: a file and line of built code, or
an owning plan **and its task id**. Any row that gets neither moves into Gaps as *no owner*, and the
27/230 counts move with it. Start with L-123, which looks like a real missing feature (brief revision
revert) rather than a bookkeeping error, and L-143, which is a phase 7 task id or nothing.

### X-M1 (Minor): the WP stream gate omits P3-15, which task 23 depends on

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:106`
**Problem:** The streams table gates WP (18-19, 21-25) on *"phase 4 tasks 06, 09-10, 16-17; phase 3
tasks 05, 07-09"*. Task 23 — the done evaluator, observed evidence at the landed commit — has
`dependsOn` *"22; P3-15"*, and P3-15 (phase 3's evidence package) is not in the gate. Task 67 adds
`P3-19` and `P4-17`, also absent. A reader who starts the stream when the gate opens finds task 23
unbuildable.
**How to fix:** Make the WP gate the union of its rows' sibling ids (`P4-06, 09, 10, 16, 17; P3-05,
07, 08, 09, 15, 19`), or say the gate is per-task and the cell is only the earliest.

### X-M2 (Minor): the map's extraction rule drops rows a part cites on its *Design:* line, so the Gaps list overstates

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/coverage.md:347`
**Problem:** The map counts a row closed only when a scope paragraph cites it after *Closes*,
*Serves* or *Carries*. Scope paragraphs also cite rows on a `Design: §1 L-…` line, and those are
dropped. **L-248** is listed as *"served by 16, closed by no Phase W task"*, but task 158 — the
public API and the generated SDKs — cites it twice (`tasks-141-160.md:717, 741`), including the
design's own words for it. The mechanical method is the right method; its citation set is one form
short, so an unknown number of the 27 Gaps rows are artefacts.
**How to fix:** Add `Design:` (and any other citation form the scopes use) to the method's list, then
re-extract and re-count. State the new Gaps count, so the difference between a real hole and a
parsing miss is visible.

### X-M3 (Minor): the plan's own claudezsh rule is not kept by the parts, and no task checks it

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:216`
**Problem:** Global constraints say *"Every shell command goes through
`~/.claude/tools/claudezsh/claudezsh` (device rule)"*, and the acceptance blocks of whole ranges of
parts give bare commands: sec-05 found it for tasks 46-50 (S05-M7), sec-16 for 157, 159 and 160
(S16-M7). Two sections finding the same thing in disjoint ranges makes it a plan-wide pattern rather
than two slips, and no task owns a sweep.
**How to fix:** One line in the Global constraints — *every acceptance command in every part is
written wrapped* — and one grep in each slice's proof task (70, 97, 113, 142, 174, 187) over its own
parts, so the rule is checked where the slice is proved.

## The two defects a first implementer would hit

1. **Which table do I obey on day one (X-C2).** The streams table says stream WH waits for Hub task
   43; task 02's row says it waits for `H01-H03`, which are on `feat/hub` today. One reading starts
   Phase W this week, the other starts it after the whole Hub plan lands, and the **WS0** start
   line — the promise that 13 hours unblocks the desktop and both delta streams — only holds under
   the second. This is the first thing anyone opening the plan has to resolve, and the plan does not
   resolve it.
2. **Who owns `useWork` (X-I2).** The seams table hands SW.16 to tasks 55-58; `task-40.md:111`
   already writes its read half into `desktop/src/work/source.ts`, a file in no file-map row, and
   `task-55.md:56` re-exports it from there. An implementer who takes task 55 from the skeleton
   writes a second `useWork` in `replica.ts` and collides with task 40 at merge — the collision the
   ⚑ convention exists to prevent, on a file the ⚑ convention was never applied to.

## What I checked and found sound

Recorded so the next reviewer does not re-derive it. The **brief's own list** is covered: W0 to W5
and WL all have tasks (187 parts exist on the tree, none missing); the design-language uplift is in
W1 (tasks 26-36); `OD-28`'s two faces are task 27; `OD-27`'s guard is task 20's `expectNoPrediction`,
required of every surface task by Global constraints; the Hub deltas are their own tasks numbered
after Hub task 43 under rule H1, as `OD-25` requires; W1's demo is local mode (task 70). The task
table has **no forward dependency**: extracted mechanically, all 187 rows name only lower-numbered
tasks. Intake from outside the org is not the security hole it looks like — task 149 treats an
inbound address as a non-person by default and refuses to promote on an unverified `From`
(`task-149.md:38, 490`), and task 148's form is Entra-protected. Task 25's Linear import takes a
personal API key only, checks `teamKey` against `^[A-Za-z0-9]{1,10}$` before it reaches the GraphQL
filter, and says plainly that it is UNVERIFIED against the live API (`task-25.md:283-290`).

Two things in my scope are already held by section reviewers and are **not** repeated here: agent
text entering the work layer without hygiene (S10-C3, S02-I9, S03-I7) and the absence of a
`CheckRunner`, without which *done means proven* cannot run in the product (S03-C1). The second
carries W1's headline demo, so it should rank with the criticals above even though it is a section
finding. `OD-30` identity is sec-90's.

Section verdict: Needs fixes
# Phase W plan review — section 01 (tasks 01-10)

Reviewer: adversarial plan review. Read-only except this file.
Scope files: docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-01-20.md
Skeleton: docs/superpowers/plans/2026-09-25-phase-w-work-layer.md

## Task 01 — W0 and the documents

Anchors re-verified against the real tree, all correct: hub-contract §3.1 at :91, prefix table
:103-109, §3.2 at :114, grammar block :119-122, rules :124-129, `done:2` at :1065 and :1079,
*"Nothing else is routed"* at :609, last route row **45** at :607, `type Hub interface` at :1329;
`PROGRAM.md:54-65` (row 9 at :65, row H at :59), `FUNCTIONAL-SPEC.md:5-16`, `AGENTIC-UX.md` §5 at
:85, anti-patterns 1/2/3/10 at :236/:237/:238/:245, `02` C3 at :209 (faces :222), C7 at :256
(`arrivals.hold` :267), C8 at :270 (on-disk replica :274), D1 item 1 at :360. Phase T task 01s
twelve prefixes verified in the team worktree (`task-01.md:185-188`), so the parts citation is
more accurate than the scopes nine. `OD-28` verified at owner-decisions:165. Baselines verified:
`grep -c OD-27 docs/AGENTIC-UX.md` = 0, `R to show` in `02` = 1 line, `OD-28` in `02` = 0.

### S01-I1 (Important): the C3 correction closes the mono question and leaves the sans claim live, and T14d pins the omission
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-01.md:190
**Problem:** `OD-28` decides two faces (owner-decisions:165: Commit Mono **and** Segoe UI
Variable). The part strikes only `**Owner decision needed:** the mono licence` at `02`:222 and
inserts a line naming both faces. It leaves three live, unstruck contradictions in the same file:
`:222` *"keep the DESIGN-SYSTEM pairing, **Geist Sans** + one licensed mono"*, `:223` *"**Geists**
variable weight axis gives headings their own weight (560)"*, and `:150` *"Spec: Geist + licensed
mono"*. This is exactly the failure T7 guards for the replica (*"a correction that leaves the
on-disk claim live"*) with no equivalent guard for faces. Worse, `T14d` asserts the `02` deletion
count is **exactly 3**, so an implementer who does strike the Geist claims makes the acceptance
script fail and will revert the correct edit to go green.
**How to fix:** strike `Geist Sans` at `:222` and the `:223` heading-optical-size sentence (and
add a *was:* note at `:150`), raise T14ds expected deletions to match, and add a test
`grep Geist "$L02" | grep -vc "~~"` = 0 with the mutant *"the sans left as Geist against OD-28"*.

### S01-M1 (Minor): D1 item 1 is declared closed but nothing checks it, and it still recommends the losing face
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-01.md:192
**Problem:** the inserted line says *"D1 item 1 (`:360`) is closed by this line"*, but `02`:360
still reads *"Recommend Berkeley Mono, per DESIGN-SYSTEMs own argument that the paid face is the
taste marker"* and is not struck or annotated. A reader landing on D1 sees the superseded
recommendation with no marker. No acceptance row covers it.
**How to fix:** add a one-line *Closed (OD-28)* note after `:360` and a T-row counting it; or drop
the "is closed by this line" clause.

### S01-M2 (Minor): step 8 forbids opening the hub worktree that step 1 requires reading
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-01.md:239
**Problem:** step 8 says *"The implementer does not open, edit or commit anything in
`.claude/worktrees/hub`"*, but step 1 requires re-reading every hub-contract anchor *"at the
current `feat/hub` head"*, which is a read inside that worktree, and the ADR-H1 citation
(`:467-474`) is also there.
**How to fix:** make step 8 read *"does not edit or commit anything in `.claude/worktrees/hub`;
read-only is expected"*.

## Task 02 — `hubapi`: Work ids, the `issue:` key, the `issue-revision` digest

Anchors re-verified in `.claude/worktrees/hub`, all correct: `keys.go` `Prefix*` block :15-26,
`ValidID` :31-35, `KeyKind` :47-52, `ErrInvalidKey` :54, regex block :56-59, `BriefKey`/
`ParseBriefKey` :61-77, `ParseKey` :96-105, `DigestOf` :114; `canonical.go:39` `Canonical`;
`ids/ids.go:61` `Wide`; `keys_test.go:683` `constValues`, `:719` `prefixEntity`, `:737-775`
`TestPrefixesAndReasonsAreTheContracts` with the ten-check at `:756-758`. No §3.2
grammar-versus-code test exists, so decision (b) (appending to `keys.go`) is safe today.

**The three golden vectors were recomputed and match exactly** (Python `json.dumps(sort_keys=True,
separators=(",",":"), ensure_ascii=False)` + SHA-256): v1 402 bytes
`a77f7648208b0456120f438ceecde358da929e501f0bfeee5cad8246fb604ab6`; v2 451 bytes
`528b7d48da2a74a65653828fc78e9cad0422b9ba2f8854aa8e0f6ef4f5c4eb01`; v3 252 bytes
`0ef4698eec15b67a4c25b580545bf4a548586cf0dfe88e6ae70dc177fbe53f9f`. Canonical strings byte-equal.

### S01-C1 (Critical): `TestWorkPrefixesAreDisjoint` fails the moment Phase T task 03 reaches the sync base
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-02.md:686
**Problem:** the test claims the Hub ten from `constValues(t, "keys.go", "Prefix")` **and** claims
Phase T's twelve from a hard-coded `phaseTPrefixes` literal. Phase T task 03 appends those exact
twelve to `keys.go` as `Prefix*` constants in a second `const` block
(`.claude/worktrees/team/.../phase-t-team-mode-parts/task-03.md:62`, `:1454-1470`), and Phase T's
own part states *"`constValues` reads every `const` declaration in the file, so a second block is
counted like the first"* and changes the ten-check to `22` (`task-03.md:39`, `:1525`). After that
merge `constValues` yields 22, `claim()` runs twice for each of `dl th hm ms nt es tk dv ag ot ls
cf`, and the test emits twelve *"prefix is both the Hub's PrefixX and Phase T's"* errors. The
`len(owner) != 10+5+12+12` guard does **not** catch it (the map still ends at 39), so the failure
reads as a prefix collision that does not exist. Both branches merge into `feat/hub`, so this is
scheduled, not hypothetical. The acceptance line states the assumption and gets it wrong:
*"TestPrefixesAndReasonsAreTheContracts still passes (ten, not twenty-two)"* (`task-02.md:990`).
**How to fix:** derive rather than duplicate. Read the Hub set once, then subtract before claiming
Phase T's: `for _, p := range phaseTPrefixes { if _, taken := owner[p]; !taken { claim(p, "Phase
T's") } }`; replace the `39` equality with `len(owner) >= 10+5+12` plus an explicit assertion that
no `WorkPrefixes` entry is already in `owner` before Phase W's loop; and change the acceptance
comment to *"passes at whatever count the sync base carries"*.

### S01-I2 (Important): Phase W's twelve prefixes never reach hub-contract §3.1, so the Hub's own "a §3.1 row first" rule is bypassed
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-02.md:57
**Problem:** decision (a) keeps the twelve out of `keys.go` and out of §3.1 *because*
`TestPrefixesAndReasonsAreTheContracts` pins ten. Phase T faced the identical constraint and
resolved it the other way: it adds its twelve to §3.1 and raises the count to 22
(`phase-t-team-mode-parts/task-03.md:35-39`). The Hub test's comment is normative
(`keys_test.go:717-718`: *"A new constant with no entry here fails: a new id prefix is a §3.1 row
first"*). With Phase W opting out, the contract's prefix table ends up describing 22 of 34 live
prefixes, nothing holds `is-`/`ic-`/… to a contract row, and the disjointness guarantee rests on a
hand-maintained literal in one test file.
**How to fix:** have W0's proposal (task 01) also ask for the twelve §3.1 rows, and have task 02
append them to `keys.go` with `prefixEntity` rows and the count raised, as Phase T does —
sequenced after Phase T task 03 in the sync order, or coordinated with it in one count change.

### S01-M3 (Minor): the Phase T prefix citation has drifted off its lines
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-02.md:681
**Problem:** `phase-t-team-mode-parts/task-01.md:176-183` was right at the commit the part names
(`af569a8`, confirmed with `git show`), but Phase T has moved it: at `feat/team-mode` head
`4fbcf535` the twelve are at `:184-189` and `:176-183` is now the Step 4 heading and prose.
**How to fix:** cite `:184-189` and re-read at the sync base, as the part's own Step 1 requires.

## Task 03 — `hubapi`: the W1 wire types and request bodies

Anchors verified: `types.go:95-99` `User`, `:196-205` `Page[T]`/`PageRequest`, `types.go:3-15` the
request/response idiom, `BriefRevision.ErasedAt` at `:179`; `types_test.go:155` `contractExamples`,
`:179`, `:257`, `:390` the three style tests. Decision (c)'s tri-state is sound: the part does tag
`AssigneeID json.RawMessage \`json:"assigneeId,omitempty"\`` (`task-03.md:438`), which is what makes
"omitted on marshal when nil" true — a bare `json.RawMessage` marshals as `null`, so the tag is
load-bearing and correctly present. `TestAssigneeChangeIsTriState` (`:860`) names the right mutant.
The part's "Corrections to the scope paragraph" block resolves every scope drift it introduces.

### S01-I3 (Important): no Phase W task ever writes the Work routes into hub-contract §7, so §7.0's "Nothing else is routed" stays false
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-07.md:334
**Problem:** task 07's route list says it is appended to §7's table "as rows 46 onward … **by the
WH proposal**", and task 01's proposal lists those rows only under *"for information; nothing to
do now"* (`task-01.md:112-149`). No task in 01-10 — and no task the skeleton names — actually
appends rows 46+, the §5.3 authorisation rows or the §7.x body sections to the contract. The
result: roughly twenty live W1 routes with `hub-contract:609` still reading *"Nothing else is
routed"*; task 03's wire shapes pinned to `workExamples` typed into `work_test.go` (`:642-665`)
rather than to contract examples the way every Hub type is (`TestFieldNamesMatchTheContractExamples`);
and task 07's authorisation matrix living only in SQL and a test. The same gap as S01-I2, one
layer up. (No route-table conformance test exists **yet** — the hub worktree has no `server`
package — so this does not break a test today; it breaks the contract's status as the source.)
**How to fix:** add one WH task, sequenced after the owner accepts W0, that appends §7.0 rows 46+,
the §5.3 rows and the §7.x sections for W1 to the contract on the sync base, and make tasks 03 and
07 cite it; or state explicitly in the skeleton that Phase W's contract text is deferred to a named
later task and record it as a known divergence.

### S01-M4 (Minor): `ParseKey`'s switch is cited one line early
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-02.md:85
**Problem:** the part says "Into `ParseKey`'s `switch` (`keys.go:97-102`)"; `:97` is
`func ParseKey(s string) (KeyKind, error) {` and the switch runs `:98-103` with the two cases at
`:99-102`.
**How to fix:** cite `keys.go:99-102`.

## Tasks 04-10 — the fake, the migration, the guards, the routes, the done rule

Verified as real: `hubclient.go:52-64` `Briefs` (the style), `:63` `Briefs.Resolve`, `:101-109`
`type Hub interface` (note it already carries `Teams`, seven members, where hub-contract:1329 lists
six — a pre-existing Hub drift, not Phase W's), `:146-149` `_ Hub = (*Fake)(nil)`;
`fake_projects.go:72-79` `projectOf`, `:81-84` `canRead`, `:108` `userRef`. Task 06's RLS matches
the Hub's own idiom exactly (`ENABLE`, policy `TO hub_app` with `USING`/`WITH CHECK`, no `FORCE`,
the same loop shape as `migrate/sql/*.sql:401`), and its column-level `GRANT UPDATE` list excludes
`number`, `project_id`, `team_id`, `source`, `external_*`, `created_*` and `erased_at`, which is
what task 07's `guard_issue` then pins. Task 07's `guard_issue_state` is sound: decision 1's
default-deny on a `NULL` `hub.work_actor_kind` fails closed, and `state_reason` is
`NOT NULL DEFAULT ''` (`task-06.md:163`), so the `btrim(NEW.state_reason) <> ''` clause cannot go
three-valued and silently pass a reasonless hand-set `done` — I checked that specifically. Task
09's refusals are right for a multi-tenant Hub: another team's `issue:` key answers `missing`, not
403 (`TestAnotherTeamsIssueKeyIsMissing`), a tampered `canonical` answers `missing` after a
re-hash, an erased key answers `erased`, and `serves` naming another project's brief is 422.

### S01-C2 (Critical): tasks 03 and 10 both declare the proof enum, its six constants, the `StateMove`/`IssueHead` fields and `EventIssueState` — `hubapi` will not compile
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-10.md:233
**Problem:** task 03's `work.go` already declares `type Proof string` with `ProofNone`,
`ProofProven`, `ProofSelfAttested`, `ProofNotProven`, `ProofNoCriteria`, `ProofWaiting`
(`task-03.md:222-231`), `type MoveCause` with `CauseLandTrailer` etc. (`:214`),
`IssueHead.Proof`/`LandCommit` (`:289-290`) and `StateMove{To, Cause, Reason, LandCommit,
Evidence}` (`:482-490`) — and task 03 decision (a) states task 10 *"declares none of these names
again"*. Task 10's own complete source declares `type StateCause` (`:211`), `type ProofWord` with
**the same six constant names** (`:233-239`), lists `LandCommit`, `Evidence`, `Proof` and
`LandCommit` as *"Additive fields on task 03's structs"* (`:233-240`) with different tags
(`json:"landCommit,omitempty"` against task 03's `json:"landCommit"`), and declares
`EventIssueState = "issue.state"` (`:257`) which task 13 also declares (`task-13.md:125`) in the
same package. An implementer following task 10's part literally gets duplicate constants,
duplicate struct fields and two names for one enum: `go build` fails. Task 04 spotted **part** of
this (gap 2, `task-04.md:1710-1711`, the two type names only) but the note lives in task 04's
return, not in task 10, so the implementer of task 10 never sees it, and it does not cover the six
constants, the four fields, the tag mismatch or `EventIssueState`.
**How to fix:** edit task 10's part directly. Delete its `StateCause`/`ProofWord` blocks and the
six constants, use task 03's `MoveCause` and `Proof` throughout (`MoveAllowed`, `DecideProof`,
`MoveError.Cause`), reduce the *"additive fields"* note to `StateMove.EvidenceCommit` only (the
one field task 03 does not declare, and give it task 03's tag style), and drop `EventIssueState`
in favour of task 13's. Then state in task 10's "does not claim" that it adds no enum.

### S01-I4 (Important): task 05 is two or three tasks priced as one
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-05.md:3
**Problem:** the header says **3 focused hours**. The part is 2255 lines and produces five new Go
files — `fake_work_social.go` (`:221-960`), `fake_work_intake.go` (`:961-1327`),
`conformance/work.go` with 25 `W-NN` rows (`:1328-1885`), `conformance/work_fake_test.go`
(`:1886-1955`) and `fake_work_social_test.go` (`:1992-2139`) — covering relations, labels, team
policies, comments, mentions, reactions, resolve, subscriptions, import idempotency, search,
attachments and erase, and it also deletes the stub and announces WS0. That is roughly 1500 lines
of new Go across nine feature areas. The brief's bar is "1-3 focused hours of work each" and every
other part in this section is a fraction of the size; task 05 is out of family by 2-4x, and it is
the gate for three downstream streams (WS0), so an overrun there stalls WP, WD1 and WD2.
**How to fix:** split into 05a (`fake_work_social.go` + its test: relations, labels, policies,
comments, subscriptions) and 05b (`fake_work_intake.go` + `conformance/work.go` + the rows: import,
search, attachments, erase, the stub deletion and the WS0 announcement), renumber the later WH
tasks or use `05a`/`05b` as the skeleton does elsewhere, and move WS0 to 05b.

### S01-M5 (Minor): task 04's `fake.go` anchors have all drifted by about +28 lines
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-04.md:19
**Problem:** the anchors were correct at the commit the part names (`feat/hub` `9f2fa31`:
`forbidden` :200, `need` :218, `pageLimit` :240 — confirmed with `git show`), but `feat/hub` has
since merged H05 (`277e6c9a`, "the fake's part 2 and the conformance suite, 83 rows"). At the
current head every one is off: `fakeWorld` :54 (part says :56-71), `mu` :55, `NewFake` :81 (part
says :74-86), `stamp` :200 (part says :172-178), `problem` :212 (part says :184-197), `forbidden`
:228 and `need` :246 (part says :200-223 for both), `pageLimit` :268 / `pageOf` :279 (part says
:239-262). `notInFakeYet` (part says `fake.go:269-276`) no longer exists anywhere in the package —
H05 removed it, exactly as decision (d) predicted, so the decision is right and only the anchor is
stale. The same drift is in the scope file (`need(have, want)`, `fake.go:218`).
**How to fix:** re-read at the sync base, as Step 1 already says; correct the *Anchors* block so
the drift does not mislead a reader who trusts it before Step 1 runs.

### S01-M6 (Minor): the fake models roles but not principal classes, and does not say so
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-04.md:1678
**Problem:** SW.3's pin — *no sync or integration principal writes a fact state or `done`* — lives
only in task 07's database trigger, keyed on `hub.work_actor_kind`. The fake has `viewer`/`member`/
`admin` and no actor kind, so a handler written against `NewFake()` (which WS0 makes the standard
for streams WP, WD1 and WD2) cannot exercise the rule at all. Task 04's "What this task does not
claim" lists six exclusions and none of them is this one. W1 only ever sets `person`, so nothing
breaks now; the risk is that W5's tasks 145/150/159 inherit a fake that has silently blessed the
path.
**How to fix:** add one line to task 04's "does not claim": *"that a non-person principal is
refused a fact state — the fake has no actor kind; task 07's trigger and its
`TestAnIntegrationNeverWritesAFact` are the only check, and W5's tasks must re-prove it."*

### S01-M7 (Minor): task 07 over-reads its own guard as "only because evidence was observed"
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-07.md:10
**Problem:** the lead says the rule *"done only because evidence was observed, and never for a sync
token. That is a trigger's job"*. `guard_issue_state` checks only the **cause word** the server
itself put in `hub.work_cause`: a caller that sets `evidence_recorded` writes `done` with no
evidence row anywhere, because the trigger never looks at `issue_proof_citations` or `evidence`.
The real rule is task 10's Go (decision 3), and the trigger is the actor half only. The sentence
does qualify itself with *"the database half"*, but the clause before it reads as the whole rule.
**How to fix:** reword to *"never for a sync token, and never with a cause that does not match the
target state; whether the cited evidence exists is task 10's Go rule, not the trigger's."*

Section verdict: Needs fixes
# Section 02 review: Phase W tasks 11 to 20

Reviewer: adversarial plan reviewer (read-only except this file). Scope source:
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-01-20.md` (lines 516-910);
skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

Verified against the tree: task-06.md:247 / :259-273 / :276-289, task-08.md:45-51,
task-16.md:25-32, hub-contract :273-276, :277-295, :333, :1115-1127, :1133-1146, :1372, :2250 (hub
worktree now at `da8c7da`; the lines cited at `9f2fa31` still hold), `desktop/src/styles/tokens.css`
:46/:57/:63/:74-77, `desktop/src/test/fixtures.ts:1-9` and `:25-26` (`FIXED_NOW`, `FIXED_NOW_ISO`;
task 20's part has it right, the scope's `:23-24` is two lines early), `desktop/vitest.config.ts:11`.

## Task 11: relations, team labels, triage and team policies

### S02-I1 (Important): the triage policy is bypassed by every fact move out of `triage`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-11.md:379
**Problem:** The scope (tasks-01-20.md:538-540) says leaving `triage` for any state but
`cancelled`/`duplicate` with `priority: none` under `priorityToLeaveTriage` is 422. The part
narrows the check to `hand` moves and leaves the fact path "UNVERIFIED, task 24's to prevent or
allow". Task 10's `MoveAllowed` (task-10.md:274-278) explicitly allows `lane_bound`, `lane_queued`
and `land_trailer` **from `triage`**. So a Start on an un-triaged, unprioritised issue moves it
straight to `started`, and L-48 does not hold. The two parts leave the question to each other, so
neither will decide it.
**How to fix:** Decide it in this part. Either (a) `TriageExitRefused` runs for every cause, and
task 24's Start sends the `hand` accept first (then the policy refusal reaches the person through
task 16's relay), or (b) the part states that fact moves bypass the policy on purpose (a fact is
never refused) and the scope's sentence is amended. Add an integration row for the decision
(`lane_bound` from an unprioritised triage issue under the policy) and a task 05 conformance row.

### S02-I2 (Important): oversized for a 1-3 hour task
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-11.md:48
**Problem:** Seven routes, three store/server pairs, a migration section, edits to task 08's
create/patch and task 10's move path, a fake re-check, 4 pure tests and 20 integration tests. That
is a working day, not 1-3 hours, and a watchdog-killed implementer loses the whole batch.
**How to fix:** Split into 11a (relations: two routes, `NormaliseRelation`, the two-cycle, 9 rows)
and 11b (labels, the policy, the create default, the triage exit, snooze: 11 rows). Task 13 and 15
depend on 11b only for the policy and labels, so the split changes no downstream task.

### S02-M1 (Minor): the recorded task 06 default mismatch is stale
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-11.md:237
**Problem:** The part says task 06's `team_work_policies.default_state` has `DEFAULT 'triage'`.
Task 06 now reads `DEFAULT 'backlog'  -- task 11's DefaultTeamPolicy` (task-06.md:279). The gap
it lists for task 06's owner no longer exists.
**How to fix:** Delete the mismatch sentences (237-241) and the matching skeleton-gap entry.

### S02-M2 (Minor): snooze on a non-triage issue is "422 `bad_request`"
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-11.md:376
**Problem:** Every other `bad_request` in the plan is a 400; pairing it with 422 gives the desktop
an error code that means two statuses. The integration row (:492) asserts 422 without a code.
**How to fix:** Use a named 422 code (for example `snooze_not_in_triage`, declared with the other
codes at :202-210) and assert it in `TestSnoozeKeepsTheIssueInTriageAndLast`.

### S02-M3 (Minor): an issue carrying an archived label can never be re-patched without losing it
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-11.md:353
**Problem:** "Issues keep an archived label they already carry" but `PATCH labels` replaces the
whole list and refuses any archived id (:372-374). A person adding one live label to such an
issue must drop the archived one, which rewrites the history the sentence promises to keep.
**How to fix:** Allow an archived label in the replacement list when the issue already carries
it; refuse only a newly added archived label. Add that case to
`TestAnIssueCarriesOnlyItsTeamsLiveLabels`.

## Task 12: comments `ic-`

Checked: the emoji validator against its own good/bad table (every case traced by hand, all hold);
the UTF-16 fixture (43 runes, 44 code units, correct); hub-contract `:235`, `:251` (`conflict`
409), `:263`, `:1133-1134`, `:1166`; `hubapi/types_test.go:257` `TestResponsesNeverOmitAField`
exists in the hub tree; `task-08.md:276-277` `IssueRevisionAt`; `ids.Wide` exists
(`hubclient/fake.go:78`). Scope coverage is complete: no edit/delete route with a
`Patterns()` test, one-level threads, all four anchor kinds checked, mentions as one event per
row, idempotent reactions, resolve on top-level only, no body in `data`, and replay without text.

### S02-M4 (Minor): task 03 and task 12 describe the anchor `ref` differently
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-12.md:138
**Problem:** Task 12 fixes a hunk `ref` as `"<commit> <path>:<lines>"` and a span `ref` as a span
id. Task 03's `CommentAnchor.Ref` comment (task-03.md:383) says "a provenance key or a span id".
Task 60 (the Merge lens) and task 51 read one of the two and may build the wrong form.
**How to fix:** Change task 03's field comment to match task 12 (span id, or `commit path:lines`),
or have task 12 name task 03's comment as the one to update in its Files list.

### S02-M5 (Minor): the "no body in the feed" test does not cover the resolve route's replay
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-12.md:491
**Problem:** `TestACommentReplayHoldsNoText` covers a comment and a reaction, but not `resolve`,
which decision 5 (:34-40) also routes through `WriteReplayable`. A resolve stored with
`server.Write` would keep the body in `idempotency_keys` and no test would fail.
**How to fix:** Add a `k3` resolve case to the same test.

## Task 13: subscriptions and the Work feed kinds (SW.5)

Checked: hub-contract `:1103-1104`, `:1135`, `:1139-1144`, `:1166`; H14 `:67-75` (`Event`,
`Data` "strings, int64s and bools only"), `:103-115` (a plain error from `fn` is not described,
as the part says); H26 `:7-37` (files, visibility, `next = H`); `hubapi/lists.go` and
`required.go` import `reflect`; task-08.md:197-201 and :300-301; task-06.md:339-352;
`docs/AGENTIC-UX.md:243`. Decision 1 (team-scoped kinds carry a null `projectId`) is a sound
correction of the scope and is consistent with task 11's `label.created` emit.

### S02-I3 (Important): relation events cannot tell a consumer which way a `blocks` points
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-13.md:167
**Problem:** `IssueRelationData.Kind` is "the stored kind: blocks | duplicates | relates", and
the event's `Subject` is the path issue (task-11.md:329 for `POST`; `DELETE` emits "the same
`data`", task-11.md:341-343). A `DELETE /v1/issues/EP-2/relations/blocked_by/EP-1` normalises to
the stored row `(EP-1 blocks EP-2)`. If the handler emits the stored kind with subject EP-2 and
`otherIssueId` EP-1, the event reads "EP-2 blocks EP-1", the reverse of the truth. If it emits
the path word `blocked_by`, it breaks this struct's comment and the registry test's intent. The
replica (task 55) and the Desk's *inform* (task 63) apply these events, so a wrong direction
gives a wrong blocker on screen.
**How to fix:** Fix one rule in both parts: relation events always carry the **stored** row,
`Subject` = stored `issue_id`, `otherIssueId` = stored `other_issue_id`, `kind` = stored kind,
whichever end the request came from; `otherAssigneeId` is the assignee of whichever end is not
the writer's path issue. Add an integration row in task 11 (`DELETE` from the `blocked_by` end,
then assert the event's subject is the blocker) and a task 55 note.

### S02-M6 (Minor): "an import subscribes nobody" departs from the scope without being listed
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-13.md:389
**Problem:** The scope (tasks-01-20.md:609-611) says the creator is subscribed on create, and
names no import exception. The part's reason is good (a 500-item import would flood W2's
digest), but unlike decision 1 it is not marked as a correction for the return, and task 15 has
to know it (its `issue.created`-only event count depends on it).
**How to fix:** Make it decision 4, "listed as a correction to the scope", and have task 15's
idempotency test assert zero `subscription.set` events for an import.

### S02-M7 (Minor): the emitWork 500 path depends on an undescribed Hub behaviour
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-13.md:349
**Problem:** The part marks it UNVERIFIED, but gives no test that holds it. If Hub task 14's
`Write` treats a plain error as something other than ROLLBACK + 500, a mis-declared event could
be half-handled and no test here would show it.
**How to fix:** Add one integration row that forces `emitWork` to fail (a test-only kind swap
behind a build tag, or a unit test of `Write` with a plain-error `fn`) and asserts 500
`internal` and zero new `audit_events` rows.

## Task 14: erase and title search

Scope coverage is complete: both erase routes (org admin only, 404 across orgs), keys/digests/
numbers kept, `head_title` added and blanked, reason in the audit and not the feed, `hub verify`
after erase, readable-only search with the alias first by formatting (no alias parser, task 02),
literal `ILIKE`, bounded page, and blobs named as open.

### S02-I4 (Important): the state-move reply keeps erasable text in `idempotency_keys`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-14.md:47
**Problem:** Decision 4 blanks `head.title` in the stored replies of tasks 08, 09 and 11 only.
Task 10's `POST /v1/issues/{issueId}/state` runs under plain `server.Write` because "the reply
carries no text" (task-10.md:404-405). But that reply is an `IssueHead`, which holds
`stateReason` (a hand-set done reason, which decision 3 itself calls "a person's words" and
erases), `external.url` (the reason task 11 moved its relation reply to `WriteReplayable`,
task-11.md:334-339) and, after this task, `title`. This part's own fixture sends the hand-set
`done` with the canary reason **with an `Idempotency-Key`** (:338-340), so
`TestEraseLeavesNoTextAnywhere` goes red on `idempotency_keys.response` and no step says what to
change. The "before" list at :350 also omits that column, so the test's first assertion is wrong
as written.
**How to fix:** Add task 10's state route to decision 4 and Step 5: it moves to
`server.WriteReplayable` with a `rebuildIssueHead` that blanks `title`, `stateReason` and
`external.url` in `Stored`. Correct task-10.md:404-405's "carries no text". Audit the other Work
`POST` replies that return a head (task 09's attachment route) the same way, and list them.

### S02-M8 (Minor): the Hub feed-projection edit is missing from the Files list
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-14.md:67
**Problem:** The reason is stripped from the feed by adding `issue.erased` and `comment.erased`
to Hub task 26's projection (:257-263, Step 6), which is Hub task 26's file (`store/changes.go`
or `server/changes.go`, UNVERIFIED). The Files list does not name it or mark it ⚑, so the Hub
delta rule (wait for the owner's file; the owner wins conflicts) is not visible where an
implementer looks first.
**How to fix:** Add the file as `⚑ (Hub task 26's file, UNVERIFIED until merge)` to Files, with
the same one-additive-rule wording task 13 uses.

Checked for task 14: H27 `:15`, `:31-39`, `:72` (`markerHits`), `:94` (route 45's row);
`hubapi/types.go:565-567` `EraseRequest`; task-07.md:164 (the `erased_at` guard) and :273-303;
task-02.md:187-192 (no alias parser) and :216. H27 `:118-126` is about dead tuples in
**storage**, which the part paraphrases as "backups" at :401; the point holds either way.

## Task 15: the import route (SW.6)

Scope coverage is complete: `triage` whatever the default, `not_tied · imported`, `source:
intake`, idempotent per `(project, source, externalId)` under the org lock, all or nothing with
`items[n].field`, labels matched and never created, safe `https` links only (userinfo refused
too), 1-500 items, erased imports not resurrected, the author name erasable, and a reply with no
text. Checked: task-06.md:155-196 (`issues`, `issues_external_once` on
`(org_id, project_id, external_source, external_id)`) and :234-246.

### S02-M9 (Minor): three different label-name limits
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-15.md:183
**Problem:** `MaxLabelNameChars = 60` says "task 06's CHECK on labels.name", which is right
(task-06.md:238, `BETWEEN 1 AND 60`), but task 11's `postLabel` caps names at 1-40 characters
(task-11.md:345). A 50-character Linear label name passes import validation here, is reported
unmatched, and then cannot be created by the admin the report sends there.
**How to fix:** Pick one limit (60, the table's) and use `hubapi.MaxLabelNameChars` in task 11's
`postLabel` as well.

### S02-M10 (Minor): stale claim that task 05's part does not exist
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-15.md:99
**Problem:** Files and *does not claim* (:739-740) say task 05's part "is not written at this
writing", so its import rows are UNVERIFIED. `task-05.md` is in the parts folder now.
**How to fix:** Read task 05's import conformance rows and cite them, or keep UNVERIFIED with the
real reason (the rows were not read).

### S02-M11 (Minor): at the top of the size range
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-15.md:644
**Problem:** A validator with 27 cases, a migration that replaces `erase_issue` again, edits to
task 08's row and projection and task 13's `fullRun`, and 19 integration rows including a
concurrency test. More than 3 hours for one implementer.
**How to fix:** Move the erase-related rows and the `erase_issue` replacement
(`TestEraseTakesTheAuthorName`, `TestAReimportDoesNotResurrectAnErasedIssue`) into a small 15b,
or accept the size and say so in the part.

## Task 16: `hubclient` Work methods and the `/v1/hub/work/*` relay

Scope coverage: the route table read by client, relay and tests; path escaping; one
`Idempotency-Key` across retries; the digest re-check on every revision-bearing method and in
`Resolve`; the relay rule (4xx verbatim, 401 `signed_out`, 5xx `upstream_unavailable`, nil `Work`
`hub_not_configured`); strict decoding; the read-only allowlist guard; the real-Hub conformance
run. All present.

### S02-I5 (Important): the erase methods do not match task 04's interface or task 14's route
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:408
**Problem:** Task 16 writes `EraseIssue(ctx, issueID) (hubapi.IssueHead, error)` and
`EraseComment(ctx, commentID) (hubapi.Comment, error)`, sending **no body** (:410, :415), and the
relay handlers use `noBody` (:726-731). Task 04 freezes `EraseIssue(ctx, issueID,
hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)` (task-04.md:230-231), and task 14's route
requires `{"reason"}` 1-500 characters and answers `IssueEraseResponse` / `CommentEraseResponse`
(task-14.md:155-169). So `var _ WorkIssues = (*Client)(nil)` does not compile, and if forced, every
erase through the client is 400 `field: reason`, which breaks task 17's erase step. Three parts
name three reply types.
**How to fix:** Take task 04's signatures (a `hubapi.EraseRequest` argument sent as the body,
relay handlers `withBody[hubapi.EraseRequest]`). Settle one reply type across tasks 04, 14 and 16
(either task 04's `WorkEraseResponse` for both, or task 14's two types in task 04's interface).

### S02-I6 (Important): `erasedAt` set skips the digest re-check even when content is present
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:465
**Problem:** `VerifyIssueRevision` returns nil whenever `r.ErasedAt != nil`. Issue `Content` is a
value, not a pointer, so "content null" cannot be observed; a Hub (or anything between) that sets
`erasedAt` on a revision with altered, non-empty text passes the check. The part says this
re-check "is the only thing that stops a Hub that altered a stored revision from putting the
altered text in front of a person or an agent" (:14-15), and task 19's Materialiser reads
`IssueRevisionAt` through it. `TestAnErasedRevisionIsNotHashed` only covers the honest case.
**How to fix:** When `ErasedAt != nil`, require both texts `""` and `serves`/`implements` empty,
else `ErrDigestMismatch`. Add a row: `erasedAt` set with a non-empty title → error. Task 19 should
also refuse any revision with `ErasedAt != nil`.

### S02-I7 (Important): oversized: thirty methods, thirty relay handlers, a query codec and a conformance run
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:37
**Problem:** Eleven files across two packages, 30 client methods, 30 relay handlers, a reflective
query encoder/decoder, the verify hook in H34's file, route-audit changes, `main.go` wiring, 17
unit rows and the full real-Hub conformance run. This is two days, not 1-3 hours, and a
watchdog-killed run loses all of it.
**How to fix:** Split into 16a (the client: `WorkRoutes`, `workPath`, methods, `verify_work.go`,
the `Resolve` line, `methods_work_test.go`), 16b (the query rule, `query.go` + tests), 16c (the
relay, `Deps.Work`, the audit map, `main.go`) and 16d (the real-Hub conformance run). Task 17
depends on 16a and 16d only.

### S02-M12 (Minor): the relay answers the table's status, not the Hub's
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:557
**Problem:** `rt.Created` decides 201 or 200. Task 11's `AddRelation` answers 201 for a new
relation and 200 for an existing one (task-11.md:103); the table has it `false`, so a new relation
reads 200. Task 09's `AttachBlob` answers 200 on a repeat, but the table has `true`, so a repeat
reads 201. "The relay is the same routes, the same bodies" (L-248, :932) is not true of statuses.
**How to fix:** Have the client method return the Hub's status beside the value (or a
`created bool`), and relay that; keep `Created` only as the test's expectation for rows with one
success status.

### S02-M13 (Minor): `Labels` cannot read archived labels or a second page
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:343
**Problem:** `Labels(ctx, teamID)` sends no query and returns the first page's items. Task 11's
route takes `?archived=true` and pages (task-11.md:117), and task 44's display menu needs the
archived label an issue still carries. `Attachments` has the same first-page-only shape.
**How to fix:** Take `(ctx, teamID, withArchived bool, page PageRequest) (Page[Label], error)` in
task 04's interface, or state the first-page-only limit in *does not claim*.

### S02-M14 (Minor): the closing "gaps for task 04's owner" are already closed
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-16.md:936
**Problem:** Gaps 1-2 (`ResolveComment`, `Attachments`) say task 04 lacks them; task-04.md now
declares both (12 matches). A reader acting on the list would re-open settled names.
**How to fix:** Drop gaps 1-2, or mark them "closed in task 04".

Checked for task 16 (all hold): `api/api.go:341` (`a.registerAttach(guarded)`), `:295-300`,
`routes_audit_test.go:32` and `:133`, `cmd/api/main.go:147`, `httpx/problem.go:22`
(`MaxBodyBytes = 1 << 20`); hub tree `hubclient.go:63` (`Resolve`), `:101` (`type Hub`), `:113`
(`type Error`), `hubapi/types.go:172` and `:202-205` (`PageRequest{After, Limit}`, no tags).

## Task 17: W1's Hub end to end through `devissuer`

Scope coverage: the four people with real PKCE tokens, every story step of the scope, the
response and exact event after each step, Bea's 404 and empty feed, done refused before evidence
and with sibling evidence, the canary gone after erase, every actor a person, `hub verify` at the
end. Task 10's `no_criteria` landing at `done` in one move (task-10.md:424-425) matches step 14's
three `issue.state` events.

### S02-C1 (Critical): the expected event list assumes imports subscribe the importer; tasks 13 and 15 say they do not
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-17.md:110
**Problem:** Step 4 expects "142 × (`issue.created`, Dylan's `subscription.set` cause
`create`)", step 9 expects Dylan's row `cause: create` "from the import", `expectedWorkEvents`
adds 142 `subscription.set` (:168-171), and gap 2 (:258-260) cites `task-13.md:388` for it. But
task 13 says the opposite at :389: "An import (task 15) subscribes **nobody**", task 15 decision 4
repeats it, and task 15's `TestTheImporterIsNotSubscribed` asserts it. With the parts as written,
Dylan has no row after the import. Alice's mention in step 8 then **inserts** Dylan's row with
cause `mention` and appends a `subscription.set` that Dylan (its subject) sees. So
`events_in_order` fails on 142 missing events and one extra; step 9 fails on the cause; the
304-event count is wrong. Step 4 of the part (:209-212) then tells the implementer the failure is
"a finding for that route's task, never a change of expectation here", which points them at
changing task 15 against its own test. The phase's end-to-end proof cannot go green as written.
**How to fix:** Follow tasks 13 and 15. Step 4: 142 × `issue.created` only. Step 8: add Dylan's
`subscription.set` (cause `mention`) after `comment.mentioned`. Step 9: Dylan's cause `mention`
(which is what the scope said, tasks-01-20.md:775-776). Rebuild `expectedWorkEvents` and the
count (163 in Dylan's feed by this table). Delete gap 2.

### S02-I8 (Important): step calls that the routes they name would refuse
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-17.md:108
**Problem:** Step 2's `PutTeamPolicy(T, {priorityToLeaveTriage: true})` sends one field, but task
11's `PUT` needs all three ("a missing one is 400 naming it", task-11.md:355). Step 14's
`CreateIssue(P, {title: CANARY, serves: []})` names no `notTiedReason`, which task 03 requires when
`serves` is empty (task-03.md:419). Both steps would fail with 400 before the story starts. The
EP-143 move chain also depends on which `defaultState` step 2 sets, and the call leaves it
unstated.
**How to fix:** Write step 2 as `{defaultState: "backlog", priorityToLeaveTriage: true,
stopAtLandedWithoutServes: false}` and step 14 with `notTiedReason: "chore"` (the reason task 10's
`TestNoCriteriaLandsAtDone` uses). State that EP-143 is created in `backlog`, so `lane_bound` runs
from `backlog`, not `triage` (S02-I1's open question does not hit the demo).

### S02-M15 (Minor): the `refusals_append_nothing` check reads `next`, which also moves on audit-only events
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-17.md:145
**Problem:** H26 sets `next` to the org's head `H` when nothing visible follows (H26 `:32-34`).
Any audit-only event appended between the two reads (an `auth.forbidden` from another step, an
`audit.read`) moves `H` with no Work event, so the check can fail on a correct refusal. It also
would not catch a refused write whose event is filtered from Dylan's view.
**How to fix:** Compare the Work events between the two cursors (empty list), not the cursor
value, and add a second read of `hub.audit_events` as the owner for the refusal's window.

## Task 18: `issue:` in `provenance/key.go`; `Ref.Kind`

Scope coverage is complete: `KindIssue`, the Hub's grammar verbatim, `Parse`/`String`,
`IssueKey` panicking like `BriefKey`, `Ref.Kind` on every constructor (`""` only for
`InvalidRef`), the issue field group, the two-parser table, the JSON `kind` test, phase 4's tests
unchanged, rule H2 (wait, no second copy). Checked: `hubapi/types.go:322-328` (`RefResult.Kind
*KeyKind`, null exactly when invalid) and `hubapi/keys.go:96` (`ParseKey`) on the hub tree;
`.claude/worktrees/phase-4` has no `provenance` directory (true today at `0452ba8`; the part
names `f700613`, the scope `6ffa6bf`, so the anchor has moved twice but the fact holds).

### S02-M16 (Minor): `Build` hard-codes `KindBrief` instead of `kindOf(key)`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-18.md:150
**Problem:** "`Build` and `ErasedRef` set `KindBrief` (they take brief sources only)". Phase 5
resolves `design:` keys, and if its resolution goes through P4's `Build`, a design ref would read
`kind: brief`, the exact zero-kind confusion this task exists to prevent. The rule is also two
rules where one would do.
**How to fix:** Every constructor sets `Kind = kindOf(key)`, and `Build`/`ErasedRef` panic when
`kindOf(key) != KindBrief` (a programming error), so a design source can never be mislabelled
silently. Add one row to `TestEveryConstructorSetsKind`.

Gap 1 (the resolver's `issue:` branch) is picked up in task 19's Files (task-19.md:68-69), so it
is closed. Checked P4 anchors: `task-06.md:24-54` (`Kind`, `Parse`, `String`, `BriefKey` panics),
`:122-148` (`Ref`, no kind field), `:234-238` (the constructors), `task-15.md:160-162`
(`ProvenanceRef`, "Every field is always present").

## Task 19: `Session.IssueRef`, the Materialiser's issue section (SW.7), the loader's second key

Scope coverage is complete: stamp from the result (a direct `issueRef` is refused, stronger than
the scope's "ignored"), the golden reproduced byte for byte, the neutraliser keeping two `<!--`,
`text·A` refused with no file written, comments never read, another project refused, the hostile
fixture, one `/v1/refs` call for both keys. The renderer-parameter decision (:45-50) is sound and
keeps P4-05's bytes. Checked on the phase 4 tree: P4 `task-05.md:262-304` (the branch golden, "fix
the renderer, never the golden") and `:328` (the neutraliser test), `task-10.md:88-110`
(`MaterialiseRequest`), `task-16.md:150-159` (`keySources`, "A later lens adds ONE entry").

### S02-I9 (Important): imported (intake) text reaches an agent's context with no person accepting it
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-19.md:250
**Problem:** The only authorship guard is "both texts `H`". Task 15 writes every imported
revision 1 with both texts `author: "H"` and `writtenBy` = the person who ran the import
(task-15.md:445-457), though the words came from an outside tracker, and may be a customer's (a
Linear intake form; W5 adds Jira, GitHub, Azure Boards and sync). Such an issue can sit in
`triage`, never read by anyone on the team, and still be adopted: its description goes verbatim
into an agent's managed block, under the line "written by <the importer>". That is the path 05
§3.4 closes for requests and comments ("customer content, read only as data"), reopened through
import. It also misattributes the text to a teammate inside the agent's context.
**How to fix:** Add a fifth adoption rule: a revision is adoptable only when a person on the team
has taken responsibility for it: refuse `issue_not_adoptable` when the head's `source` is
`intake` and the adopted revision is the imported revision 1 (require a person's revision 2,
which task 49's editor writes on accept or edit), or at least when the issue is still in `triage`.
Render "imported from <source> by <importer>" instead of "written by" for such a revision. Add
`TestAnImportedRevisionIsNotAdoptable` with the message fixed for task 24, and state the rule in
SW.7.

### S02-M18 (Minor): an erased revision is refused with the wrong sentence
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-19.md:246
**Problem:** Rule 3 treats a digest mismatch as not adoptable, but task 16's re-check skips an
erased revision (see S02-I6), so an erased revision reaches rule 4 and fails the author check.
The person then reads "An agent wrote part of this revision", which is false.
**How to fix:** Refuse `ErasedAt != nil` in rule 3, with its own message ("That issue revision was
erased, so it was not given to the agent."), and add one row.

### S02-M19 (Minor): five packages across three owners in one task
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-19.md:52
**Problem:** `claudemd`, `contextwrite`, `api`/`registry`, `discoverhub` and the desktop, waiting
on four planned phase 4 tasks, with 16 tests. The per-package commits (Step 7) limit the damage,
but the task is above 3 hours.
**How to fix:** Split at the existing step boundaries: 19a `claudemd` (SW.7's bytes), 19b
`contextwrite` + `api`/`registry`, 19c `discoverhub` + the desktop loader.

## Task 20: `expectNoPrediction` and the Work fixtures (SW.8)

Scope coverage of `expectNoPrediction` is complete and careful: every banned form of the scope,
the two attribute-only exemptions (a projection may say *when*, never *how far*), the phrasing
join that catches `<b>3</b> of 5` without dragging a verbatim span in, `findPredictions` for
precise asserts, and the CSS-width bar named as not caught. Every scope row has a test with its
mutant; the 13 + 5 counts match the tables. `docs/AGENTIC-UX.md:237` checked.

### S02-I10 (Important): the fixture types and world are out of step with the W1 wire they stand in for
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-20.md:178
**Problem:** The mirror types copy task 03 only, but tasks 11, 14 and 15 change the W1 wire
additively, and every surface task from 26 to 37 builds against these fixtures:
- `IssueHead` has no `title` (task 14 put it on the head so "a list row" is not "a number with no
  name", task-14.md:45-50) and no `snoozedUntil` (task 11); `Label` has no `archivedAt`;
  `ExternalRef` has no `createdAt` (task 15).
- `Relation.kind` is `"blocks" | "duplicates" | "relates"`, so the read words `blocked_by` and
  `duplicated_by` (task 11 decision 1) cannot be expressed. The world puts `EP-150 blocks EP-142`
  on EP-150's head only and gives EP-142 `relations: []`, and EP-169 `duplicates` EP-142 likewise.
  The Hub serves EP-142 with `{blocked_by, EP-150}` and `{duplicated_by, EP-169}`
  (task-11.md:108-109). A surface built on this fixture shows EP-142's blocker by scanning other
  issues, the one-sided read task 11's test exists to kill, and breaks on the real wire.
**How to fix:** Mirror the W1 wire as of task 15 (task 03 plus the additive fields of tasks 11, 14
and 15), widen `Relation.kind` to the five words, give EP-142 `blocked_by EP-150` and
`duplicated_by EP-169`, give every head a `title`, and add a `workFixtures.test.ts` row that
every relation in the world has its inverse on the other end.

### S02-M20 (Minor): the `comment.created` fixture event contradicts task 13's registry
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-20.md:332
**Problem:** The fixture event has `subject` = the issue id and `data: {commentId}`. Task 13 fixes
`comment.created` with `subject` = the comment id and `data {issueId, parentId, kind, author,
anchorKind}` (task-13.md:188-194, task-12.md:392). `data` is typed
`Record<string, string | number | boolean>`, so task 37's typecheck will not catch it, and the
replica (55) and Desk (62) tests would learn the wrong shape. `issue.state`'s fixture data also
lacks `from`, `proof` and `landCommit`.
**How to fix:** Build each fixture event's `data` with every key of its task 13 struct, and fix the
`comment.created` subject. Add a row that each fixture event's `data` keys equal task 13's list
for its kind (the list copied as data until task 37 imports it).

### S02-M21 (Minor): `makeTeamPolicy` reads "never set" while holding non-default values
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-20.md:273
**Problem:** The builder's default is `defaultState: "triage"`, `priorityToLeaveTriage: true`,
with `updatedAt: null`, `updatedBy: null`. Task 11 defines null `updatedAt`/`updatedBy` as "the
defaults" (`backlog`, every switch off, task-11.md:132-135). A settings surface tested on this
fixture would show "defaults" beside values that are not the defaults.
**How to fix:** Make the builder's default equal `DefaultTeamPolicy` (backlog, switches off,
nulls); let `workScenario()` override it with the demo's `priorityToLeaveTriage: true` plus a
non-null `updatedAt` and `updatedBy`.

## Section summary

Critical 1 (S02-C1). Important 10 (S02-I1 to S02-I10). Minor 20 (S02-M1 to S02-M16 and S02-M18
to S02-M21; M17 was withdrawn as closed by task 19). No part file of 11-20 is missing or cut off:
each ends with its *does not claim* and parity rows (tasks 16-19 with a gaps list).

Section verdict: Needs fixes

# Section 03 — adversarial plan review, Phase W tasks 21 to 30

Reviewer: S03 (read-only except this file). Parts read in line ranges against
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-21-40.md` and the skeleton
`docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.


## Task 21 — the `Panacea-Issue` trailer, the lane's issue, AU-only branch names

Scope `tasks-21-40.md:19-56`. Coverage is complete: every named test in the scope appears
(`TestValidIssueTrailerRejectsKeysAndFlags`, `TestDispatchStampsTheIssueOnce`,
`TestDispatchOmitsTheIssueWhenUnset`, `TestLaneRecordsItsIssue`, `TestLandAddsOnePerDistinctIssue`,
`TestIssueBranchNameAUOnlyHasNoSlug`, `TestIssueBranchNameSlugIsBounded`), each with a mutant, each
red first with a named compiler error. Verified against the real trees: `sessions.go:951` and
`:1637` are the two `backend.Spec{` sites; `backend.go:62` is `envReservedPrefix`; `ValidateSpec`
returns `ErrUnsupported` on a reserved `Spec.Env` key (`backend.go:276-277`); `hubapi.ParseIssueKey`
and `hubapi.IssueKey` are Phase W task 02's (`task-02.md:48`, `:119`, `:126`), so the call is real
even though `hub`'s `keys.go` at `cafe1303` has only `ValidID` (`:32`) and `ParseKey` (`:96`).

### S03-I1 (Important): `Spec.IssueID` is a new environment input with no check at the boundary

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-21.md:132-143`
**Problem:** the part adds `Spec.IssueID`, exported as `PANACEA_APP_ISSUE_ID`, and argues the
grammar is enforced "before git sees it". It is enforced in exactly one place — `issueIDOf` in
`sessions.go` — and nowhere at the boundary. `ValidateSpec` grows no row for `Spec.IssueID`, and
`EnvEntries` writes whatever the field holds. Every other `PANACEA_APP_*` variable has a loud check
plus a silent backstop (`backend.go:215-230` says so in as many words); this one has neither. The
part's own test table proves only `EnvEntries` ordering and the `Env`-shadowing case.
**Failure it allows:** any later caller of `backend.Create` — phase 8's remote backend, a restart
path added after task 19, a test rig — sets `Spec.IssueID` from an unvalidated field; the hook's
POSIX `case` silently drops it, and the commit carries no `Panacea-Issue`. The land still attributes
through `Lane.IssueID`, so nothing fails loudly and the loss is invisible until a release manifest
(task 71) is short an issue.
**How to fix:** add one `ValidateSpec` row — `spec.IssueID != "" && !gitx.ValidIssueTrailer(...)` →
`ErrUnsupported` — or, if `backend` must not import `gitx`, inline the same anchored expression
there and let `TestValidIssueTrailerAgreesWithHubapi` pin all three. Add the red test row
`TestValidateSpecRefusesABadIssueID` (mutant: a caller-chosen or malformed id reaching `EnvEntries`).

### S03-M1 (Minor): every phase 3 anchor is pinned to a commit the phase 3 tree has moved past

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-21.md:60`
**Problem:** "Phase 3 anchors were re-read in the `phase-3` worktree at `9935278`". That commit is
real and is an ancestor of the phase 3 head, but the head is now `3676625a`, and the ranges have
moved: the trailer const block the part cites as `task-02.md:305-342` is `:366-397` at head
(`TrailerSession` `:366`, `TrailerBringIn` `:373`, `RecordSep`/`ValueSep` `:392-393`,
`FieldSep` `:391`). The same ~57-line drift applies to the ⚑ citations in tasks 21, 22 and 23. The
part does say "UNVERIFIED until merge" and names its commit, so this is honest, not wrong — but an
implementer reading `task-02.md:305-342` at the sync base lands in `MachineFS`/`Locks`, not in the
trailers.
**How to fix:** re-pin the three parts' ⚑ ranges at the phase 3 head, or replace line numbers with
grep anchors (`grep -n "TrailerBringIn"`) that survive drift.

### S03-M2 (Minor): thirteen files, five packages and a Docker rig in a "3 hour" task

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-21.md:48-59`
**Problem:** the file list is 13 entries across `gitx`, `backend`, `api`, `lanes`, `converge` and
the sandbox image, with 7 steps, 11 named tests and one step that needs Docker and a rebuilt
sandbox image. Task 22, by comparison, is 2 new files plus 4 one-line edits for the same 3 hours.
The estimate is flagged "an estimate, not a measurement", so this is sizing pressure, not a
contradiction.
**How to fix:** split at the natural seam — `21a` = `gitx` + `backend` + `sessions.go` + the hook
(the grammar and the stamp), `21b` = `lanes` + `converge` (the index and the land). They share only
`gitx.ValidIssueTrailer`, which `21a` produces.

## Task 22 — the `started`, `in_review` and `landed` fact moves

Scope `tasks-21-40.md:58-91`. Names check out against their owners: `hubapi.CauseLaneBound`,
`CauseLaneQueued`, `CauseLandTrailer` are task 10's (`task-10.md:215-217`), `StateMove.LandCommit`
is task 10's (`:234`), the nine `IssueState` constants are task 03's (`task-03.md:101-109`),
`gitx.FieldSep` is P3-02's (`task-02.md:391` at the phase 3 head), and
`catalog.go:414` really is `a.deps.Flags.SetWorktreeReady(worktreeID, ready)` in this worktree. The
two declared corrections to the scope (task 10's cause spellings; `LaneBound` called by Start, not
by P3-04's route) are both improvements and both argued. Monotonicity matches design 05 §3.3, which
names no move backwards out of `landed`.

### S03-I2 (Important): the scope's per-tick re-derivation from git is not built

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-22.md:341-355`
**Problem:** the scope requires "a sweep on P3-07's lane-sync tick re-sends `landed` for any issue
whose trailer is on the branch while the Hub still shows an earlier state, so a lost send heals
without an outbox on disk" (`tasks-21-40.md:84-86`). `Run` calls `Rescan` **once**, before the
first tick, and the ticker then calls only `Sweep`, which retries the in-memory `pending` map. So
the tick heals nothing that is not already in memory. The part's own closing section concedes the
resulting hole ("heals at start **only if** its branch still has a lane in the index, or on the
branch's next land"), but the scope asked for the tick to close it and the part narrows it without
raising the change the way it raised its other two corrections.
**Failure it allows:** the control plane sends `landed` for EP-142, the Hub 503s, the operator
restarts the desktop before the backoff elapses, and the lane is removed after the land (the normal
end of a lane). No further land happens on that branch. EP-142 stays at `in_review` for ever while
git says it landed — exactly the loss `Rescan` exists to prevent, and no test covers it
(`TestRescanHealsAfterARestart` runs `Rescan` by hand).
**How to fix:** either call `Rescan` on the tick for each `ProjectBranch` (cheap: one `git log`
per branch, and `move` reads the Hub only for issues the branch actually names), on a slower
multiple of the interval; or state the narrowing as a third declared correction with the owner's
line, and add `TestTickRescansEachBranch` as a red test for whichever is chosen.

### S03-I3 (Important): `Rescan` inside the land handler makes the land's latency the Hub's latency

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-22.md:388-396`
**Problem:** the land handler runs `Landed` **and** `Rescan` synchronously on the request goroutine
before the 201. `move` gives each Hub call `f.timeout()` (3 s by default) on a
`context.WithoutCancel` context, so a client disconnect does not shorten it, and both calls are
serial per issue (`Issue` then `MoveIssueState`). The part costs `Rescan` as "one `git log` and at
most 20 Hub reads" — that is wrong twice: the bound is one `Issue` read per **distinct issue named
across 20 land commits**, not 20, and each is up to 3 s, plus a `MoveIssueState` for any that moved.
**Failure it allows:** a branch whose last 20 lands name 30 distinct issues, against a Hub that is
reachable but slow (a 3 s timeout, not a refusal): the land's 201 is held about 90 s behind git work
that already finished. `TestLandBodyIsTheSameWhenTheHubIsDown` cannot catch it, because
`SetUnavailable` fails fast; the test proves the body, not the deadline.
**How to fix:** run both off the request — hand `(merge, res.Issues)` and the `ProjectBranch` to a
buffered channel the `Run` goroutine drains — and keep the handler's own work to the enqueue. Add
`TestLandDoesNotWaitOnASlowHub` (a fake whose `Issue` blocks; the land returns 201 within the test's
deadline) as the red test. If it must stay inline, cap the whole post-land block with one budget
(`context.WithTimeout` around `Landed`+`Rescan` together), not one per call, and correct the cost
sentence.

### S03-M3 (Minor): `IssueFacts.Log` is dereferenced without a nil guard

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-22.md:166-176`
**Problem:** `move` calls `f.Log.Warn(...)` on both failure paths. `Log` is a plain
`*slog.Logger` field with no default; only `cmd/api` and the test rig set it. Task 24 (Start) and
task 23 both construct or receive an `IssueFacts` and neither part shows the field being set.
**Failure it allows:** a `IssueFacts` built without `Log` panics on the first refused or failed
send — which is the failure path, so it only shows up when the Hub is already unhappy.
**How to fix:** a `func (f *IssueFacts) log() *slog.Logger` returning `slog.New(slog.DiscardHandler)`
(or `slog.Default()`) when the field is nil, used at both call sites; assert it with
`TestAFactsValueWithNoLoggerDoesNotPanic`.

## Task 23 — the done evaluator, observed evidence, *self-attested*

Scope `tasks-21-40.md:93-135`. The part is the strongest in this section: every scope test appears
plus five more, the revert walk is reused rather than copied (P3 `task-15.md:111-163`), the
citation-and-ancestry rule is exactly the design's, and the four corrections to the scope are each
argued against the seam's owner. The `Fresh` mechanism is a real repair, not a fudge. Everything
below is about what the part itself flags and what the plan then fails to own.

### S03-C1 (Critical): nothing in the plan builds a `CheckRunner` or a criterion's check, so observed evidence never runs in the product

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-23.md:70-77`
**Problem:** two inputs the observer needs have no owner anywhere in Phase W. Verified by grep over
the skeleton, all eleven scope files and all 187 parts: `CheckRunner` occurs only inside
`task-23.md`, and `Serve` (`task-02.md:252`) has no `check` field. `cmd/api` is wired
`Runner: nil, Checks: nil` by the part's own step 4, so `Observer.Run` returns `ErrNoRunner` on
every product path. Meanwhile the skeleton opens with the opposite claim — "turns `done` by itself
only when the platform **observed** its own evidence at its own landed commit"
(`2026-09-25-phase-w-work-layer.md:11`, and again `:74`) — and the scope closes task 23 with "it is
what makes the demo's no-click `done` (05 §5.3) true".
**Failure it allows:** Phase W ships, every acceptance command prints `ok`, and the no-click `done`
in 05 §5.3 row 7 exists only in `TestObservedExitZeroClosesWithNoClick` against a host test double
in `observe_test.go`. In the product an issue closes only when a person presses Record — which is
`proven_self_attested` in solo mode, the visibly weaker word. The phase's headline differentiator
is unbuilt and the plan does not say so above part level.
**How to fix:** two additions, both W1, both before WS0 freezes SW.1. (a) Add `check` to task 02's
`Serve` (it changes the `issue-revision` canonical form and its golden vectors, which is why it
cannot wait), with the authoring rule written down — see S03-I5. (b) Add one task owning a product
`CheckRunner` over the phase 1 Docker sandbox or P3-24's `sandboxd`, sized like task 23, depending
on it. If the owner defers both, say so in the skeleton at `:11` and `:74` and in the coverage file,
and re-word the L-8 parity row: as written the plan claims a capability it does not build.

### S03-I4 (Important): a refused or lost `done` never heals, unlike every other fact move

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-23.md:462-468`
**Problem:** `Closer.send` says "A refusal is logged with its code and dropped … Nothing is queued",
and `TestCloserRefusalIsDropped` pins that. Task 22, sixty lines of plan earlier, built
`pending`/`Sweep`/`Rescan` for exactly this failure on `started`/`in_review`/`landed`, and the two
tasks are the same seam (SW.4) in the same package family. Worse, `AfterLand` step 1 skips an issue
unless `head.State` is already `landed` — but task 22's `landed` move is itself best-effort and may
only land later, on a `Sweep`, and nothing re-runs the Closer when it does.
**Failure it allows:** EP-142 lands with an observed check at exit 0. The Hub 503s on the `done`
move (or on task 22's `landed`, so `AfterLand` skips). The evidence Record is in the store, git is
correct, and the issue sits at `landed` for ever: `AfterLand` needs another land of the same issue
(there will not be one) and `AfterRecord` needs a person to press Record for evidence that already
exists. The 5xx path is untested — `TestCloserRefusalIsDropped` uses 422, a permanent code, where
dropping is right.
**How to fix:** separate permanent from transient exactly as task 22 does (`permanent(err)`); on a
transient failure, hand the issue id to task 22's `IssueFacts` retry list, or give the Closer a
`Recheck(ctx, issueID)` that task 22's `Sweep` calls for any issue it moves to `landed`. Add
`TestADoneLostToA503IsRetried` and `TestLandedOnSweepRunsTheCloser` as red tests.

### S03-I5 (Important): the check is `sh -c` on platform-chosen text, and no part says who may author it

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-23.md:341-357`
**Problem:** `CheckRun.Command` is run as `sh -c Command` by the platform, on the person's machine
in local mode, and the resulting Record is written **as the lander** and sent as that person's
`evidence_observed` move. The part bounds the blast radius well (a fresh clone in a sandbox, no host
mount but the read-only bundle, output discarded, 1000 bytes, a 30-minute cap) but never names the
authority: who may set a criterion's `check`. The surrounding design makes that gap load-bearing —
§3.5 says a request from outside the org arrives as verbatim `text·H` and is routed into an issue,
and §3.4 says an agent's `text·A` revision exists. A criterion is spec-class text on the same
thread.
**Failure it allows:** a check authored from, or adopted with, a revision that originated outside
the team (an accepted request, an agent proposal a person waved through) becomes a shell command the
platform runs and then attests to under that person's Hub token. The person's name is on an
`evidence_observed` move for a command they never read.
**How to fix:** write the rule in this part even though the field is task 02's: a `check` may be set
only by a team member with the brief's write verb, never carried in by intake or by a `text·A`
revision, and a revision whose `check` changed needs explicit acceptance before the observer will
run it (the same shape as §3.4's "never rendered until a person accepts it"). Add
`TestACheckFromAnUnacceptedRevisionIsNotRun` and `TestTheObserverRefusesACheckOnARequestSourcedCriterion`.

### S03-M4 (Minor): a reverted land leaves a `done` the evaluator no longer agrees with

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-23.md:600-602`
**Problem:** the part states it plainly as a non-claim — "not … reopening a `done` issue whose land
is later reverted: the evaluator reads `waiting` after a revert, and no move is sent back". Design
05 §3.3 does not cover a revert either, and no task in Phase W owns it (grep for `reopen` across the
scopes returns only unrelated rows).
**Failure it allows:** P3-10 reverts the land. `TestRevertedLandRetiresIssueEvidence` proves the
evaluator now says `waiting`, but the Hub's head still reads `done · proven`, and every surface that
shows the issue (the Work lens, the Desk, the release manifest of task 71) shows a proven issue whose
work is not on the branch. The provenance thread contradicts git, which is the one thing the thread
must not do.
**How to fix:** decide it at the owner level and record it: either a `revert` cause that moves
`done` → `landed` (fact-driven, so it does not break "only facts set fact states"), or an explicit
line in 05 §3.3 that `done` is sticky across a revert and the criterion — not the issue — carries the
regression. Whichever it is, name it in this part's non-claims with the owner's ruling id rather
than as an open end.

## Task 24 — Start from an issue, *Undo Start*, the data-class route

Scope `tasks-21-40.md:137-180`. Full coverage: all ten scope tests plus five more, the SW.10 wire
matches the scope field for field, the revision pin is proved by a Hub write after Start, the
undo has no clock anywhere (a good, testable way to kill the timer mutant), the first-write signal
is read through P3-24's runner and never the PTY (`OD-8`), and the unknown-class fail-safe is
proved twice (error and nil). Anchors verified in this worktree: `sessionCreateRequest`
`sessions.go:696`, `handleSessionCreate` `:759`, `handleSessionDelete` `:1681`, the session route's
path value is `{id}` (`attach.go:42`), `guarded` and `Origin.Middleware` at `api.go:337`, `:393`.

### S03-I6 (Important): this section needs three additions to frozen seams that no task owns

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-24.md:47-50`
**Problem:** tasks 23 and 24 each declare a change to a seam another task owns, and in all three
cases the owning task does not contain it. Verified by grep over every part:
1. `hubapi.CauseStartUndone` — task 24 says plainly "task 10's cause list, which calls itself *the
   complete W1 set*, does not have it, so **SW.4 must gain it**". It is absent from
   `task-10.md:215-217`.
2. `Serve.check` — task 23 needs it; `task-02.md:252`'s `Serve` has no such field, and task 02 says
   nothing about adding one.
3. `issueId` and `observed` on the Hub's evidence record — task 23's gap 4; `task-10.md` has no
   `issueId` outside the state route's path.
The skeleton freezes SW.1 and SW.2 at **WS0** (`2026-09-25-phase-w-work-layer.md:366`): "a later
change that is not an added method, field or route reopens task 05 and is announced". `Serve.check`
is an added field, but it changes the `issue-revision` canonical form and its golden vectors, which
task 02 owns and WS0 freezes — so it is exactly the change that reopens task 05 if it arrives late.
**Failure it allows:** WS0 closes on 01-05 (about 13 h) with SW.1 frozen. Tasks 23 and 24 then start
and each finds a field it needs is not there. Either they land disabled (the part's own fallback:
`Checks: nil`, `Runner: nil`, `stateRestored: null` because the Hub 422s `start_undone`) and Phase W
ships Start with no backward move and `done` with no observed evidence, or task 05 is reopened
mid-phase and the fake, the conformance rows and the golden vectors are rewritten under three
downstream streams.
**How to fix:** fold all three into tasks 02 and 10 **before** WS0, as one short amendment task in
the 01-20 band (they are three fields and one constant). The parts that need them already specify
the exact shapes, so the work is small; what is not small is discovering it after the freeze. If the
owner would rather defer, say so in the skeleton's WS0 row and mark the dependent parity rows
(**L-8**, **L-181**) as not closed in W1.

### S03-M5 (Minor): the in-process session adapter has three small sharp edges

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-24.md:320-350`
**Problem:** `apiSessions` synthesises requests and calls `handleSessionCreate`/`handleSessionDelete`
directly. The intent is right — reuse every check phase 2 and task 19 wrote rather than copy them —
but three details are wrong or unspecified:
(a) `return out.ID, json.Unmarshal(w.body.Bytes(), &out)` reads `out.ID` in the same return
statement that fills it. The Go spec orders function calls against each other, not a plain operand
against a call, so this is unspecified; it happens to work on gc today (probed: returns `"s-1"`),
which is the worst kind of correct.
(b) `net/http/httptest` is imported by production code. The part offers the alternative in the same
breath, so just take it: `http.NewRequestWithContext` plus `RequestURI`.
(c) the synthesised request carries none of the caller's headers, so any header-borne check in a
future session handler is silently skipped. The outer route is on `guarded`
(`api.go:337`, `:393`), so the origin middleware has already run for the real request and nothing is
exposed today — but the bypass is invisible and nothing pins it.
**Failure it allows:** (a) a toolchain or compiler change returns `""` as the session id while
`err` is nil, and Start reports 201 with `"sessionId": ""`; the desktop then attaches to nothing.
**How to fix:** unmarshal on its own line and return `out.ID, nil`; swap `httptest` for
`http.NewRequestWithContext`; add one line to the adapter comment naming what the in-process call
deliberately skips, and a test row `TestInProcessCreateReturnsTheID` asserting a non-empty id.

### S03-M6 (Minor): it is not said whether `Undo` clears the `requestId` entry

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-24.md:287-291`
**Problem:** `Starter` keeps two maps, `byRequest` and `bySession`. Step 7 of `Start` writes both.
`Undo` step 5 says "delete the record", singular, and `Discard` says "the record is deleted". Which
map — or both — is not stated, and no test covers it (`TestUndoBeforeFirstWriteRemovesAll` proves a
second `Undo` gives `ErrNotFound`, which only exercises `bySession`).
**Failure it allows:** if only `bySession` is cleared, a retry of the same `requestId` after an undo
returns the first `Result` verbatim — a 201 naming a session and a worktree that no longer exist —
and the desktop opens a dead lane instead of starting again.
**How to fix:** say "delete both entries" in `Undo` step 5 and in `Discard`, and add
`TestAnUndoneRequestIDStartsAgain` (same `requestId` after an undo → a new lane and a new session).

## Task 25 — the Linear public-API and CSV import adapters

Scope `tasks-21-40.md:182-215`. The best-covered part in the section: all nine scope tests plus
four more, and the secret handling is designed rather than asserted — one constant endpoint with no
configuration variable, `CheckRedirect` refusing every redirect so the header cannot follow a 302,
the key zeroed after the handler returns, and `TestLinearKeyNeverLeaves` walking the log buffer,
every problem body, a temp `statepath` root and a second server's received headers. Clean room is
respected (`OD-11`): hand-written fixtures from public documentation, with the *Done* node placed in
a field the query never asks for so `TestLinearStateIsNotCarried` proves the omission. The three
declared corrections to the scope (no `externalKey` in SW.6, `unmatchedLabels` added, no dry run) are
each checked against task 15 and each raised as a gap rather than invented.

### S03-I7 (Important): imported text reaches an agent's managed block and no part names the check

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-25.md:10-16`
**Problem:** a Linear description is up to 16 KiB of text written by people outside this org, and a
CSV `description` is whatever was in the file. Task 25 validates length, title, URL scheme and
`externalId` — size and shape, not content — and says the Hub decides everything else. Task 24 then
puts the adopted issue revision into the session's managed block and cites 05 §3.4 as the authority.
But §3.4 authorises that only "after the Hub's schema and **character checks**", and closes with
"This waits on the owner's ruling (Q3) and on TM-05's injection corpus covering hostile issue text".
Neither task 25 nor task 24 names the check, the corpus or the ruling, and no test in either part
feeds hostile text end to end.
**Failure it allows:** a person imports a backlog from a Linear workspace shared with a client. One
description carries agent-directed text. It lands in `triage` as `text·H`, a person triages it
without reading 16 KiB, presses `Shift+R`, and task 24's Start copies the revision verbatim into the
agent's managed block. The whole provenance thread says a member adopted it, which is true and
beside the point.
**How to fix:** name the dependency in both parts — "the Hub's character check (task 03/15) runs on
every imported field; Start adopts only a revision that passed it" — and add one red test per side:
`TestImportedDescriptionIsCharacterChecked` here (a fixture holding the corpus's shapes → refused or
sanitised, with the reason), and `TestStartRefusesAnUncheckedRevision` in task 24. If Q3 is still
open, say so in the non-claims of both parts with the ruling id, as task 23 does for its gaps.

### S03-M7 (Minor): a preview followed by an import walks the Linear API twice

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-25.md:368-374`
**Problem:** `Preview` and `Import` take the same `Source` and each call `LinearPager.All`, which
walks every page to the end (up to 100 requests for 5,000 issues). The dialog (task 69) will
naturally preview and then import, so the normal path is two full walks, sharing one
`linearMaxWaits` budget of five 429 waits each. Nothing says so, and `TestWorkImportPreviewWritesNothing`
proves only that the Hub is untouched.
**Failure it allows:** a 5,000-issue workspace: preview walks 100 pages, the person presses Import,
the second walk hits Linear's rate limit part-way, five waits are exhausted and the import ends
`503 import_source_unavailable` after having created some issues — with the person having just seen
a successful preview.
**How to fix:** either state the cost plainly in the wire section and have the dialog warn, or let
`Preview` return an opaque handle the following `Import` consumes (the items are already in memory
for the duration of one dialog). Add `TestPreviewThenImportWalksOnce` for whichever is chosen.

## Task 26 — tokens: 02 D2 step 1, `--t-fact`, the provisional `paper` block

Scope `tasks-21-40.md:217-252`. Every citation checks out in this worktree: `tokens.css` is 191
lines with `:root` at `:19`, `--ink-3` 3.43:1 at `:47`, `--sans`/`--mono` at `:151-152`, `--numeric`
at `:156`, `:root` closing at `:157`, the focus ring at `:187`; `tokens.test.ts` is 91 lines with
`describe("the token layer")` at `:41` and the documented-hex test at `:68-70`; the prototype sets
`--t-body:13px` at `work-layer.html:40`. Counted independently: `tokens.css` declares exactly the 34
colour tokens `THEME_COLOUR_NAMES` lists, so `toHaveLength(34)` is right today. The contrast helper
is correct — it goes OKLab → LMS → **linear** sRGB and feeds those to the WCAG coefficients, which
is what WCAG relative luminance wants, with no gamma step wrongly applied. Three scope corrections
(six spacing steps, the amended hex test, `:root[data-theme]` specificity) are each argued, and the
part fixes the scope's own acceptance (`grep -c "^  --t-fact"` prints 2, not 1). Step 4's
hand-mutation list is the strongest red-first evidence in the section.

### S03-M8 (Minor): `--icon-ink` is a colour that paper does not redeclare, and the 34-name list is hand-kept

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-26.md:18` and `:283-292`
**Problem:** two small holes in an otherwise airtight test set.
(a) `--icon-ink` is added to `:root` in the icons group and is not in `THEME_COLOUR_NAMES`, so the
paper block never redeclares it. If its value is a literal OKLCH rather than `var(--ink-2)`, every
icon on the paper sheet keeps graphite's ink. The part never says which it is, and no test
distinguishes them.
(b) `THEME_COLOUR_NAMES` is a hand-written list whose comment claims it is "every colour graphite
declares". It is today — I counted the file — but nothing derives it from `ROOT`, so a 35th colour
added to graphite later leaves paper short of it and every test still passes.
**Failure it allows:** (a) task 33 renders the paper sheet for the owner and every icon is the dark
theme's ink on a light ground — a contrast failure the ink-floor test does not look at, because it
only checks `--ink`, `--ink-2`, `--ink-3`.
**How to fix:** declare `--icon-ink: var(--ink-2)` (indirection, so it follows the theme) or add it
to `THEME_COLOUR_NAMES` and to the paper block; and derive the expected set —
`expect(declaredCustomProperties(PAPER)).toEqual(colourNamesIn(ROOT))` with a small helper that
keeps only `oklch(`/`color-mix(` declarations — so graphite and paper cannot drift apart.

## Task 27 — faces: Commit Mono bundled with its licence, Segoe UI Variable as the sans

Scope `tasks-21-40.md:254-288`. All five scope tests appear plus four more, and the part earns its
keep twice: it catches that the scope's "before its first fit" is not enough (xterm may measure the
cell in `open()`, so both are gated), and it catches that `Terminal.tsx:38` hard-codes a stack that
ignores `--mono` at all — both verified in this worktree (`Terminal.tsx` is 178 lines, `:38` is the
literal stack, `:120` is the `fit()`; `tauri.conf.json:21` really is `"csp": null`). The licence
handling is right: no Segoe file or `@font-face`, an `OFL-<Family>.txt` per family, and a lock file
with a release tag and per-file SHA-256. The "if the release ships no WOFF2, stop and report"
branch is the correct call — a TTF conversion is a Reserved Font Name question, not an
implementer's.

### S03-M9 (Minor): the lock-file test breaks if the licence is recorded, and no test checks a hash

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-27.md:288-295`
**Problem:** `expect(entries.map((e) => e.ours).sort()).toEqual(FACES)` requires the lock's file
list to be **exactly** the `.woff2` set. The part's own file description says the lock records
"each file's upstream name, our name and SHA-256", and the licence text is a file with an upstream
name too — recording it, which is the provenance-minded thing to do, turns this test red for the
right reason and wrong outcome. Separately, the hashes are matched against the files only by the
acceptance command (the part explains why a `?raw` read of a binary is useless), so nothing in CI
catches a face replaced after the lock was written.
**Failure it allows:** an implementer records `OFL-CommitMono.txt` in the lock, the test fails, and
the fix that looks obvious is to drop the licence from the lock — losing exactly the provenance the
lock exists for.
**How to fix:** compare only the woff2 entries (`entries.filter((e) => e.ours.endsWith(".woff2"))`)
and assert separately that every `OFL-*.txt` present is also recorded. For the hashes, put the
`sha256sum -c` into the desktop package's pretest or a CI step rather than a one-off acceptance
line, so a swapped face fails a build and not only the day it was bundled.

## Task 28 — drawn glyphs, part 1: nine issue states, *done, not proven*, the priority notch

Scope `tasks-21-40.md:290-322`. No findings. Every claim about the prototype is true at the byte
level: `work-layer.html:393-401` really is `g-triage` … `g-cancel`, `g-dnp` really is a dashed ring
(which would read as `backlog` with a tick), `g-review`'s centre really is `fill="currentColor"`
(a fill, so the part replaces it with a bar), and there really is no `duplicate` symbol. The nine
`IssueState` strings match task 03's `hubapi.IssueState` exactly. The scope's clip-versus-mask
correction is right for the stated reason — `designsystem.test.ts`'s `namedColours` rule would fire
on a luminance mask's `white`/`black`. The test set is airtight in a way worth copying: the
no-progress mutant is killed at `tsc` by `@ts-expect-error` (and the part says so, because Vitest
strips types without checking them), the notch test proves one `<svg>`, an identical body across all
five priorities and monotonic tip depth, and `it("uses currentColor and no status colour")` scans the
stylesheet's own rules rather than the rendered element. Its `/(?:^|[\s;])(?:color|fill|stroke)\s*:/`
correctly does not fire on the `stroke-width: var(--icon-stroke)` that the 14-versus-16 test
requires — checked, because those two tests would otherwise contradict each other.

**Addendum to S03-M8** (from reading this part): task 28 depends on `--icon-ink` and sets
`color: var(--icon-ink)` on the row, which makes the paper-theme hole in S03-M8 load-bearing rather
than theoretical. Also, `tokens.css:106-107` declares `--idle: var(--ink-3)` and
`--exited: var(--ink-3)` — two more colour tokens absent from `THEME_COLOUR_NAMES`. They happen to
be safe (the indirection follows the theme), but the list's comment, "every colour graphite
declares", is already inaccurate on the day it is written.

## Task 29 — drawn glyphs, part 2: agent shapes, marks, the licence check

Scope `tasks-21-40.md:324-360`. Strong part. It finds a real built defect and proves it from the
tree: `StatusDot.tsx:18-27` maps `running` and `idle` both to `dot-disc`, and `tokens.css:100-105`
admits it in a comment ("the shape vocabulary does NOT separate idle from RUNNING"). Both verified
here. The two declared scope corrections are right: `package-lock.json` is the only readable licence
source without `@types/node`, and two existing `StatusDot` tests do pin the old premise and must be
rewritten rather than deleted. `SessionStatus` really is four values (`store/types.ts:134`), so
`failed` really has no wire behind it yet, which the part says.

### S03-I8 (Important): the rail keeps a second colour-only pair, so "one vocabulary" is not reached

**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-29.md:13-19` and `:83-85`
**Problem:** the part changes exactly one entry of `StatusDot`'s shape map, `running` → `dot-run`,
and says that makes "the rail and the Work lens speak one vocabulary". `DotStatus` is
`SessionStatus | MachineState`, **eight** values, and its shape map collapses them into three shapes
(verified, `StatusDot.tsx:18-27`). After this task:
- `exited`, `stopped` and `error` are all still `dot-square`; `.dot-tone-exited, .dot-tone-stopped`
  is `--exited` and `.dot-tone-error` is `--fail` (`common.css:74-80`). So **`error` and `exited`
  differ by colour alone** — the identical defect, one row down in the same map.
- `AgentGlyph` meanwhile gives `failed` its own shape, "a square with its top-right corner cut". So
  the two components now disagree about the one status a reader most needs to catch.
- `starting`, `provisioning` and `sleeping` are all `dot-ring`, with `starting`/`provisioning` on
  `--accent` and `sleeping` on `--idle`.
The scope's test `it("separates all five statuses with colour removed")` runs over `AgentGlyph`
only, so nothing catches it, and `tokens.css:100-105`'s comment is amended for the running pair
while the failing pair keeps its silence.
**Failure it allows:** a session errors. In the Work lens the reader sees a cut square. In the rail,
three feet away, they see the same square as a cleanly exited session, told apart by a red a Windows
high-contrast theme (task 34) repaints — which is precisely 05 §4.4 rule 1's case.
**How to fix:** give `error` the cut square in `StatusDot` too (the geometry is already drawn in
`paths.ts` for `AgentGlyph.failed`, so this is a class, a CSS rule and a map entry), and say in the
part how the other three `MachineState` values map onto the five agent shapes — or state plainly
that `StatusDot` keeps eight values and only `running`/`idle` and `error`/`exited` are in scope here.
Add `it("separates error from exited with colour removed")` to `StatusDot.test.tsx`.

## Task 30 — `voice.split` across Develop and the session pane

Scope `tasks-21-40.md:362-392`. No findings. The part's value is almost entirely in its four
corrections, and each is checked against the tree here: `develop.css` is 474 lines with `.pane-sub`
at `:120-124` (`400 11px/1.4 var(--sans)`, `--ink-3`), `.pane-attach` at `:166`, `.pane-foot` at
`:244`, `.cv-coords` at `:386`; `palette.css:115-118` really is `.pal-hint` with
`font-family: var(--mono); font-size: 11px`, so rule A really would fire on the real tree and the
selector fix (`kbd.pal-hint`) is the honest answer rather than an exemption; `SessionPaneHeader.tsx`
is 64 lines and renders no branch, so the scope's "keeps the branch mono" test genuinely cannot be
written and the part says why instead of inventing a branch; `designsystem.test.ts` is 886 lines.
Rules A, B and C are each stated with their exemption and each planted in the `:849` block so the
rule is shown to fire — the one thing that separates an audit rule from a comment. Keeping every
replaced declaration under `:root[data-look="old"]`, with rules A and B skipping that selector and
task 39 owning the deletion, is the right shape for a look change the owner has not accepted yet.

---

## Section summary

Ten parts read in full against `tasks-21-40.md:19-392` and the skeleton. Coverage of the scope is
complete in every part: I found no scope test missing, no acceptance command dropped, and no part
truncated. Citations are unusually reliable — I checked roughly forty against the real trees and
found one class of drift (S03-M1, phase 3 anchors pinned to an ancestor commit) and no invention.
Where a part disagrees with its scope it says so, gives the reason and names the owner it checked
against; that happened sixteen times across the ten parts and was right or defensible every time.

What stops this being an approval is not the parts' craft but what sits between them: one headline
capability with no owner (S03-C1), three seam amendments the owning tasks do not contain (S03-I6),
and two places where a best-effort Hub write has no path back (S03-I2, S03-I4) in a phase whose
whole claim is that the provenance thread and git agree.

Section verdict: Needs fixes
# Phase W plan review — section 04 (tasks 31-40)

Reviewer: adversarial plan review, read-only except this file.
Scope file: `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-21-40.md:394-710`
Skeleton: `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:433-442`
Parts read: `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-31.md` .. `task-40.md`

---

## Tasks 31-35 (chrome, flags, stress sets, forced colours, grain)

All five parts are present, complete (interfaces, tests-first with named mutants, steps,
acceptance, does-not-claim, Interfaces block) and none is cut off. Citations spot-checked and
found real: `app.css` 82 lines, `rail.css` 147, `subrail.css` 62, `designsystem.test.ts` 886,
`Rail.test.tsx` 181, `LensPlaceholder.tsx:15`'s `<h2>`, `shell.css:44-47`, `tokens.test.ts:85-90`,
`SubRail.tsx:9-16`'s six lenses, `store/types.ts:173`, `App.tsx:240`, `defects.test.tsx:193-201`,
`SessionPane.tsx:125-133`, `main.tsx:7-9`, `store.ts:91`, `DESIGN-SYSTEM.md:90`'s eleven states,
`AppShell.tsx:82`, `AppShell.test.tsx:119` and `:192-193`, `tokens.css` grain at `:163-171`.
Three scope deviations are argued rather than silent and each is right: task 31's "there is no
Develop title row to move"; task 33's refusal to create an empty `GrainMeter.tsx` (deferred to
task 35, which does create it); task 34's `data-changed="unseen"` over the scope's
`data-unseen="true"`, which matches the written task 41 (`task-41.md:53-56`, verified).
The `designsystem` audit really does glob `/src/**/*.css` and `/src/**/*.{ts,tsx}`
(`designsystem.test.ts:25-30`), so task 34's "the existing `transition: all` rule covers the new
file" holds, and task 31's `accentOutsideCurrent` is scoped to the two chrome files, so task 32's
`devtools.css` use of `--accent` does not trip it.

### S04-I1 (Important): task 35 creates the phase results file that task 70 also creates, with no merge rule

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-35.md:56
**Problem:** task 35's file list has `docs/verification/2026-09-25-phase-w-results.md` as **new**
("the skeleton gives this task no results file") and writes a `## Grain (task 35)` section into it.
Task 70 also declares the same path new: "Creates `docs/verification/2026-09-25-phase-w-results.md`"
(`task-70.md:5`) with "new: the lead, *What was tested*, and the W1 section" (`task-70.md:66`).
Tasks 97, 113, 142, 174 and 187 all call it "task 70's file" and each names its own append rule;
task 35 is the only writer with no such rule, and it runs 35 tasks *earlier* than the creator. An
implementer following task 70 literally writes the lead over a file that already holds the Grain
section, and the measured numbers — the only evidence 05 §4.4 work rule 6 asks for — are lost.
Task 97 itself notes the gap: "the skeleton should name a merge rule, as it does for the Hub"
(`task-97.md:311`).
**How to fix:** in task 35's file list mark the results file ⚑ "new here, owned jointly with task
70", state that task 35 creates it with only the `## Grain (task 35)` section if absent and
appends otherwise, and add the reciprocal line to task 70's part ("if the file exists, insert the
lead above the existing sections; never truncate"). Add task 35 to the joint-owner list the other
five parts already carry.

### S04-M1 (Minor): `fonts.lock.json` records a SHA-256 that nothing ever checks

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-32.md:448
**Problem:** step 1 writes each face's release, source and SHA-256 into
`desktop/src/devtools/fonts/fonts.lock.json`, and no test, script or acceptance command reads it
back. The five `flags` and five `FlagCompare` tests never touch the lock. A `woff2` swapped in the
tree later — by a rebase, a bad merge, or a hand edit — leaves the lock saying the file is the
fetched release when it is not, and the record that `OD-11`'s licence check was done points at a
file that has changed.
**How to fix:** add one test to task 32's list, `it("matches each font file to its locked hash")`,
hashing each `woff2` with `node:crypto` and comparing to `files[].sha256`, with the mutant "a byte
changed in a font file". Its per-face skip when the licence check dropped a candidate keeps it
honest if one face is never fetched.

## Tasks 36-40 (thread primitives, wire types, the two prototypes, the Work lens)

All five parts are present and none is cut off. Citations spot-checked and real:
`tokens.css:57` (`--thread` at alpha 0.16), `prototype/work-layer.html:28` (`--thread-lit`
`rgba(255,241,224,.58)`), `:140-146` (lit is not selected, the gutter only), `:379-385` (the
forced-colours sketch), `StatusDot.test.tsx:7` (the `?raw` idiom), `client.ts:30`, `:40`, `:132`,
`:170` (`Method` has no `PUT`, so task 37's addition is needed), `:236`, `store.ts:34-49`
(`partialize`, `lens` at `:46`), `store.test.ts:27-30`, `AppShell.tsx:67-68` (the `default` arm
renders nothing, so `"work"` compiles before task 38), `SubRail.tsx:9-16`,
`ADR-H1:467-474` (read in the hub worktree: "neither the renderer nor the control plane persists
Hub data to disk"), `task-16.md:127-158` (exactly 30 `WorkRoutes` rows, matching task 37's
`WORK_RELAY` 30 + `WORK_LOCAL` 5), `task-41.md:53-56`, `05-design.md:849` (4.1), `:879-912` (the
Work lens drawing), `:930` (4.2). Security reads are right: task 37 keeps every call on the
loopback relay with a fetch-spy test, adds no Hub URL to `desktop/src`, persists no Hub record
(the `partialize` test), and keeps agent output out of the meaning line by structure -- task 40's
`AgentLine` takes no text prop and a structural test forbids importing `terminal`,
`attachRegistry`, `xterm` or `transcript` (`OD-8`). Tasks 38-39 keep the prototype out of
production behind three gates with a dist grep. Task 39 is `OD-27`-clean: it writes no verdict, the
file says *not yet reviewed*, and its acceptance reads "0 quotes before the session" as the honest
state rather than a pass.

### S04-I2 (Important): the skeleton's dependency column omits shared files tasks 35 and 36 must find already written

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:437
**Problem:** the skeleton gives task 35 the dependency `31` and task 36 the dependencies `29, 30`.
Neither is enough for the part as written. Task 35's test
`it("keeps grain on the shell only, with one forced-colours rule")` counts the single
`.grain::after { display: none }` that **task 34** writes in `forced-colors.css`
(`task-35.md:29-33`), and task 35 also registers through **task 33**'s `screens/index.ts` and adds
a group to **task 32**'s `FlagCompare`. Task 36 appends its thread block to the same
`forced-colors.css` and says so itself: "Runs after task 34 as well ... the skeleton's row names 29
and 30 only" (`task-36.md:6-9`). The streams table lets a slice start as soon as its listed
dependencies are met, so an implementer who starts 35 or 36 from that column finds no
`forced-colors.css` to append to and a test that counts a rule nobody wrote. Tasks 37, 38, 39 and
40 each name the same omission for their own rows ("the skeleton's row names 05 and 20 only",
"names 32, 36 and 37 only", "names 38 only"), so this is the column, not one row.
**How to fix:** in the skeleton's task list set row 35's dependencies to `33, 34` and row 36's to
`29, 30, 34`; add tasks 16, 24 and 25 to row 37 marked as contract dependencies; add 20, 28, 29,
31 and 33 to row 38 and 20, 28, 29, 31, 33, 36 to row 39 as placement dependencies. One sentence
in *Streams, and where each slice can start* should distinguish a build dependency (the file must
exist) from a contract dependency (only the shape is needed).

### S04-I3 (Important): the route badge SW.13 exists for has no producer anywhere in the phase

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-40.md:553
**Problem:** task 31 builds `ViewHeader`'s route badge as "the visible half of `OD-19`" and says
"task 24 computes it". Task 24 returns `route` and `routeForced` only inside `StartResult` for one
start (`task-24.md:104`, `:150-151`); there is no per-project read of a data class or a route
anywhere in the phase -- a grep for `dataClass` over all 187 parts returns nothing, and the
skeleton's `OD-24` row points only at tasks 21 and 24. So task 40's Work lens, the one W1 surface
the brief calls the visible half of `OD-19`, hard-codes `route={null}` (its own gap 4), the lens
placeholder passes `null` (`task-31.md:207`), and tasks 46, 47, 126, 127 and 132 each take `route`
as a prop from a caller that has no source for it. `OD-19`'s per-project switch is therefore
invisible for the whole of Phase W, while the plan reads as though it is shown.
**How to fix:** either add one task to W1 -- a project read (`dataClass` to `Route`) on
`WorkReader` and the relay, so `WorkMount` can pass a real route -- or state plainly in the
skeleton's `OD-19` row and in task 31's *does not claim* that no surface shows a route badge
before phase 6, and take the badge out of 05 4.1's W1 drawing so the plan stops promising it.

### S04-I4 (Important): nothing deletes `desktop/src/prototype/work/`; tasks 38 and 40 contradict each other

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-40.md:955
**Problem:** the scope for task 40 says `AppShell` mounts `WorkLens` "in place of task 38's
prototype, which this task removes after promoting what the owner kept"
(`tasks-21-40.md:681-683`), and task 38 repeats it: "Task 40 promotes what the owner kept into
`components/work/` and deletes the rest" (`task-38.md:12`). Task 40's gap 6 then refuses: "tasks
45, 46 and 62 still compare against it, so this part keeps it and no task row removes it", and
that is true -- `task-46.md:933`, `task-62.md:17` and `:452` all cite the prototype. The result is
four dev-only screens, a fixture world and two test files living in `src/` for the rest of the
phase with no owner and no deletion task, while two parts state they were deleted at task 40.
**How to fix:** pick one and write it down. Either give the last consumer (task 62) a step that
deletes `desktop/src/prototype/work/` and its tests, and change task 38's sentence and the scope
to name task 62; or keep the directory deliberately, say so in task 38 and in the skeleton's file
map, and add the audit rule that no module outside `prototype/` and `devtools/` may import from it,
so it cannot leak into the product.

### S04-M2 (Minor): task 40 has no `Interfaces:` block

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-40.md:962
**Problem:** every part from task 30 to task 39 ends with an `**Interfaces:**` block listing
produces / consumes / amends, and the house style does too
(`docs/superpowers/plans/phase-2-parts/task-05.md:22`). Task 40 ends at *Gaps found while writing
this part* with no such block. It is the part that defines `WorkReader`, `useWork`,
`setWorkSource`, `selectWorkView`, `windowOf`, `agentMeaning` and `livenessWord` -- the surface
tasks 41 to 47 and 55 all build on -- so the one part that most needs the summary is the one
without it.
**How to fix:** append the block, listing `WorkReader`, `useWork`, `setWorkSource` (SW.16's read
half, which task 55 implements), `selectWorkView` and `WorkView`, `windowOf`, `agentMeaning`,
`livenessWord`, `WorkLens`, `WorkMount`, `CriterionGroup`, `IssueRow`, `AgentLine` and the `.wk-*`
classes tasks 41-45 append to, then "Amends: nothing in the Hub".

### S04-M3 (Minor): task 37 at 2.5 hours is the section's largest part

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-37.md:2
**Problem:** the part is 971 lines and carries eight complete interface sections: about twenty wire
types, thirty relay rows plus five local rows, `WorkClient`, `makeWorkClient`, `workProblem`, the
`client.ts` deltas (adding `PUT` to `Method` and eleven `ProblemCode`s), `workSlice`, the
`types.ts` and `store.ts` deltas and the `workFixtures.ts` re-export, then ten tests across three
files. Its neighbours at 3 h (31, 33, 36) each produce one component family. 2.5 h is a
transcription estimate, not a build one, and the rule is 1-3 focused hours.
**How to fix:** either raise it to 3 h, or split at the seam already in the part -- 37a the types
and route tables (`work.ts` parts 1-5 with the golden-vector and `IssueState` tests), 37b the
client, `workProblem`, `workSlice` and the store deltas -- since only 37a is what task 38 consumes.

Section verdict: Needs fixes
# Phase W plan review — section 05: tasks 41 to 50

Reviewer pass over `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-41.md` .. `task-50.md`, each read against its scope paragraph in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-41-60.md` and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

Finding ids: S05-C\* critical, S05-I\* important, S05-M\* minor.

## Task 41 — `arrivals.hold`, `moves.hold` and departures

Scope covered: the three `HeldChange` kinds, `reconcile`/`applyHeld`, `N` on the list element,
`data-changed="unseen"`, D M9's header flash, the six named model mutants and the four surface
mutants. Citations checked and real: `task-03.md:211-216` (the six `MoveCause` values),
`task-20.md:330-331` (the feed carries `to` and `cause` only), `tokens.css`, and the prototype's
held words at `prototype/work-layer.html:632`.

### S05-M1 (Minor): the part's own table writes a `HeldLine` sentence its code never produces
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-41.md:16
**Problem:** The *What the reader sees* table says the held line reads *One new issue is held, so no
row moves under you.* `heldWords(n)` (`task-41.md:282`) returns *One change is held, so no row moves
under you.*, which is the prototype's exact string (`prototype/work-layer.html:632`). An implementer
reading the table first writes the wrong sentence, and no test in the part pins the words.
**How to fix:** Change the table cell to the prototype's wording, and add `expect(heldWords(1))` and
`expect(heldWords(2))` string assertions to `hold.test.ts` so the words are pinned once.

### S05-M2 (Minor): a departed issue that comes back is drawn twice until `N`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-41.md:200
**Problem:** The ghost branch of `reconcile` returns before `wasLive.add(id)`, so on the next
reconcile the same issue re-entering `next` fails `wasLive.has(r.issueId)` and is counted as an
**arrival**. The group then shows both *EP-128 left this view · cancelled* and *One change is held*
for the same issue, and the held list carries a `departure` and an `arrival` for one id.
**How to fix:** In the ghost branch, add the id to `wasLive`, and in the layout loop drop the ghost
(and push no arrival) when `fact.has(id)`; add a model test: ghost, then the issue returns, and the
group holds one entry for it.

### S05-M3 (Minor): `data-changed="unseen"` outlives the change it marks
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-41.md:208
**Problem:** `carried` keeps a non-null `cause` on a row after attention leaves and the row
relocates, but the matching `move` is no longer in `held`. The row therefore keeps
`data-changed="unseen"` (and, under task 34, the static tick) with no `HeldLine` on screen and
nothing for `N` to apply, so the mark can only be cleared by a blind `N`.
**How to fix:** Clear `cause` for a row the same reconcile that relocates it, or state in decision 6
that the cause line survives relocation on purpose and rename the attribute value (e.g.
`data-changed="moved"`) so task 34 does not draw an unseen tick on a change the reader has seen.

## Task 42 — Triage: the view, number keys, `T`

Scope covered: `suggestCriteria` with matched words and no score, `1`-`4` on the Triage list only,
`T` through `CriterionPicker`, the `priorityToLeaveTriage` refusal shown in place, *not tied ·
imported*. Citations checked and real: `task-11.md:175-176`, `:184`, `:187-199`, `:379-381`;
`task-20.md:273`, `:304-308`; `05-design.md:1006`, `:1008`, `:613-618`, `:1175`, `:885`, `:1237`.
The snooze arithmetic is right: `FIXED_NOW` is `2026-09-18T09:00:00.000Z` (`task-20.md:28`), so the
next Sydney 09:00 is `2026-09-18T23:00:00.000Z`, the value the test pins.

### S05-I1 (Important): the part inverts the skeleton's dependsOn, which the conventions say wins
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-42.md:4
**Problem:** The skeleton row is `| 42 | … | 40 | — |` (`2026-09-25-phase-w-work-layer.md:444`), and
the scope's conventions say *the row's id, dependsOn and seams win*. This part instead declares it
also depends on tasks 41 and **46** and imports `Picker`, `handMove`, `DuplicateOfPicker` and
`PriorityPicker` from a task with a higher number, which is built later in task order. Tasks 43 and
44 do the same (`task-43.md:3`, `task-44.md:3`). As written the four parts cannot be executed in
the order the skeleton gives: task 42 fails to compile against a tree where task 46 has not landed.
**How to fix:** Decide once, in the skeleton: either move the shared picker chassis (`Picker`,
`stateOptions`, `handMove`, `DuplicateOfPicker`) out of 46 into a small task before 42, or renumber
so 46 precedes 42-44, and update the skeleton's `dependsOn` column for 42, 43, 44 and 45 to match.

### S05-M4 (Minor): the Triage surface never calls `expectNoPrediction`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-42.md:289
**Problem:** The scope's conventions say *every surface test calls `expectNoPrediction(container)`*
(SW.8). In task 42 only the `CriterionPicker` test does; none of the six `TriageView.test.tsx` rows
asserts it, so a snoozed-until countdown or a *3 of 7 triaged* counter added to the Triage header
would ship green.
**How to fix:** Add `expectNoPrediction(container)` to the *an imported row reads not tied ·
imported* row (or one dedicated row) and raise the expected count in the acceptance block to 11.

## Task 43 — the board, and drag that offers a fact instead of faking it

Scope covered: criterion columns by default with *Serves no criterion* last, the eight drop rules as
one pure `dropIntent`, pointer-only drag on `transform`, the empty-criterion sentence, the eight
named mutants. Prototype citations checked and real: `work-layer.html:553` (the *Layout* group),
`:201-212` (board CSS), `:662` (*No issue serves this criterion. C files one.*).

### S05-I2 (Important): `FACT_WAITS_ON` is imported from a module that does not export it
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-43.md:79
**Problem:** `import { FACT_WAITS_ON, handMove } from "../issue/pickers/StatePicker"`. Task 46 puts
`FACT_WAITS_ON` in `pickers/stateOptions.ts` (`task-46.md:113`) and `StatePicker.tsx` imports only
`stateOptions` from it and re-exports nothing (`task-46.md:236-242`). The named acceptance command
`npm --prefix desktop run typecheck` fails with "has no exported member FACT_WAITS_ON", and every
`offer` line on the board is undefined at runtime.
**How to fix:** Import `FACT_WAITS_ON` (and `isFactState`) from `"../issue/pickers/stateOptions"`
and keep `handMove` from `StatePicker`, or add an explicit re-export line to task 46's
`StatePicker.tsx` and say so in task 46's exported surface.

### S05-I3 (Important): a refused board drop rolls back with no reason anywhere on screen
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-43.md:136
**Problem:** The board sends `setState` and `serves` ops through `mutate` and never subscribes to
SW.16's `onRefused`. On a state board the `triage` column is real (`task-43.md:121` routes `triage`
through the default `handMove` arm), so dragging a priority-less issue from `triage` to `todo` is
exactly `TriageExitRefused` (`task-11.md:187-199`, 422 `policy_refused`,
`details.policy = "priorityToLeaveTriage"`). The replica rolls the optimistic move back (task 56)
and the card silently returns to its old column: the reader is told nothing, which is the failure
class task 42 spends a whole decision (`task-42.md:57-64`) preventing on the same write.
**How to fix:** Give `BoardView` the same `onRefused` effect task 42 specifies — flash the card
`--fail`, print the reason in the target column's `role="status"` line, and open `PriorityPicker`
in place for `priorityToLeaveTriage` — and add a test row: a refused `triage`-to-`todo` drop leaves
the card in `triage` with `data-fail` and the reason.

### S05-M5 (Minor): a hand move into `triage` is offered without a rule that allows it
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-43.md:121
**Problem:** The scope's human band is `backlog`, `todo` and a W2 custom state; this part's default
arm adds `triage` and `cancelled`. `cancelled` is justified (Triage's `3`); `triage` is not
justified in this part, and it is only task 46 decision 1 (`task-46.md:76-78`, citing
`task-10.md:17-23`) that makes it legal.
**How to fix:** Cite `task-10.md:17-23` in decision 1's table beside the `triage` arm, or drop
`triage` from the board's droppable columns and leave the re-open to `S`.

### S05-M6 (Minor): the board column header drops the convergence word the prototype shows
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-43.md:255
**Problem:** *What this task does NOT claim* removes the convergence word from a criterion column
header, but `renderBoard` in the prototype this part cites prints it
(`prototype/work-layer.html:652`, a `conv` span in `.bh .m`, beside the plain count). The brief
says to follow `prototype/work-layer.html`; a board whose columns are criteria but which hides
*uncovered* and *contested* is a weaker thread view than the list (task 40) and Plan (task 45),
both of which show it.
**How to fix:** Either render the word from the same source task 45 uses (`PlanCriterion.reading`
plus `servedBy`) and drop the exclusion, or record in the skeleton that the word is deliberately
Plan-only and note the prototype divergence so the owner's visual review is not surprised.

## Task 44 — display options and the bulk bar

Scope covered: `DisplayOptions` exactly as the scope types it, persisted per view in `prefsSlice`,
density from `--row-h` and `--row-h-compact`, FLIP on an explicit re-sort only, `X` and
`Shift+Down`/`Up` on the list element, one batch and one Undo, *Start N worktrees* with
`ConfirmInPlace` and no cost figure, the ten named mutants. The `selection` to `issueSelection`
rename (decision 6) is a real collision caught against `uiSlice.ts:35`, and is the right call.

### S05-I4 (Important): the two-step Start confirm can be satisfied by one held keypress
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-44.md:147
**Problem:** `ConfirmInPlace` focuses its confirm button on mount, and a `button` fires `click` on
`keydown` for `Enter`. Pressing and holding `Enter` on *Start 3 worktrees* mounts `ConfirmInPlace`,
moves focus to the *Start 3 worktrees* button inside it, and the next auto-repeat `keydown`
activates that button: three agents on three worktrees from one keypress, on the one act in this
task the design calls consequential (02 C8). The part's own test passes because `userEvent.click`
issues two discrete clicks.
**How to fix:** Focus *Cancel* on mount (or focus nothing and leave the consequence as a
`role="status"` line), or ignore activation of the confirm button for one frame after mount; add a
test that an auto-repeating `Enter` calls `onStart` zero times.

### S05-I5 (Important): the bulk assignee batch calls `assignOp`, which task 46 does not define
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-44.md:208
**Problem:** `mutate(heads.map((h) => assignOp(h, me, to)).filter(...))` needs
`assignOp(head, me, to): WorkOp | null`. Task 46 exports `assign` (`task-46.md:358`), which calls
`mutate` itself and returns `void`, and exports no op builder. The part names this as its own gap 3
(`task-44.md:307`) but leaves task 46 unchanged, so the two parts as written cannot both be right.
**How to fix:** Add `assignOp` to task 46's exported surface (its `AssigneePicker` section), with
`assign` rewritten over it, and delete task 44's gap 3. Task 46's *asks the owner to hand over*
test then covers both callers.

## Task 45 — Plan (`G P`)

Scope covered: exactly four sections in order with the W2 nouns absent and that absence tested, the
convergence word read from phase 4 and never derived from issue states, *My issues* as the assignee
filter only, plain counts, `expectNoPrediction`. Decision 4 (`GET /v1/me` rather than the scope's
`/v1/auth/status`, whose account carries no `u-` id) is a correct catch.

### S05-I6 (Important): the *Criteria* section's own selector and key function have no source
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-45.md:124
**Problem:** House style requires complete fenced source for every interface a part introduces.
`servedByCount(r, projectId)` is named as *this task's* (`task-45.md:139`) with no signature, no
return type beyond a comment and no implementation; `key(c)` at `task-45.md:132` is used twice and
defined nowhere; `HeldGroups` and `PlanCriterionRow` are this task's components with no props type.
The implementer must invent the criterion composite key, and the three parts that already build one
disagree: `c:briefKey#criterionId` (`task-43.md:88`), `briefKey#criterionId` (`task-45.md:131`) and
`briefKey criterionId` (`task-47.md:133`). A mismatch yields `0 issues` and `uncovered` for every
criterion, which the tests would not catch because they build `criteria` from the same helper.
**How to fix:** Add one fenced block with an exported `serveKey`, `servedByCount(r, projectId):
ReadonlyMap<string, readonly string[]>`, and the props types of `HeldGroups` and
`PlanCriterionRow`; export `serveKey` from one module (task 42's `suggestCriterion.ts` or a new
`work/serves.ts`) and have tasks 43 and 47 import it rather than each rolling its own.

## Task 46 — the issue view, part 1: head fields and their pickers

Scope covered in full: fact states listed with the fact each waits on and never selectable,
priority as the glyph's notch with no signal colour, people-only assignees with *Assign to me*
first, the team's labels, the alias as display only, facts in mono at `--t-fact` and never
`--ink-3`, the reassign request, `expectNoPrediction`. Nineteen tests, each with its mutant, and
the count adds up (5 + 8 + 6). Citations checked and real: `task-03.md:296` (`headKey`),
`task-31.md:84` and `:108` (`Route` and *Inference route: AU*), `task-29.md:145`,
`hub-contract.md` routes 3 and 6. Decision 1 (`triage` is choosable, on `task-10.md:17-23`) is a
correct catch against the scope.

### S05-I7 (Important): four parts' "complete" CSS uses six custom properties nothing defines
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-46.md:675
**Problem:** `issue.css` (tasks 46, 47, 48) and `editor.css` (task 49) use `--r-sm`, `--r-md`,
`--ring`, `--surface-2`, `--shadow-2` and `--t-small`. None is in `desktop/src/styles/tokens.css`
(its radii are `--r-chip`, `--r-ctl`, `--r-card`, `--r-window` at `:110-113`; its elevations are
`--e-hair`, `--e-btn`, `--e-card`, `--e-raised` at `:118-121`) and none is added by task 26, whose
block adds `--dur-*`, `--s-*`, `--row-h*` and `--t-*` only (`task-26.md:83-105`, pinned by the test
list at `:289-292`). An undefined custom property fails **silently**: `box-shadow: 0 0 0 1px
var(--ring), var(--shadow-2)` drops the whole declaration, so every picker, the reason step and the
editor ship with no ring and no shadow — the exact thing `CLAUDE.md` calls a non-negotiable
("elevation is a 1px ring plus layered shadow, never a flat border"). Task 46's own note calls the
names UNVERIFIED and defers the rename to step 1; task 50's gap 3 finds the same thing
independently and quietly uses different names, so the four files would not even agree. No test
catches it: they all read the CSS as text.
**How to fix:** Pick the built names once — `--e-card` for a popover's elevation, `--r-ctl`/
`--r-card` for radii, `--surface` for a popover ground, `--t-meta` for the small step — and rewrite
the CSS blocks in tasks 46-49 to them; or add the six to task 26's block and to its
`expect(names)` list. Then add one structural check (the `StatusDot.test.tsx` pattern already used
here): every `var(--…)` in `issue.css` and `editor.css` appears in `tokens.css` or task 26's block.

### S05-M7 (Minor): the acceptance commands of tasks 46 to 50 are unwrapped
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-46.md:949
**Problem:** Tasks 41-45 run every acceptance command through
`~/.claude/tools/claudezsh/claudezsh '…'` (four each); tasks 46, 47, 48, 49 and 50 use bare `npm`,
`grep` and `node -e` (zero each). The plan's own rule 1 makes the wrapper the audit trail, and the
bare `grep` and `node -e` lines are the ones a run would execute verbatim.
**How to fix:** Wrap the acceptance blocks of tasks 46-50 the way 41-45 already are, using the
double-quoted outer form for the `node -e` lines, which contain single quotes.

### S05-M8 (Minor): "another owner's live agents" is silently redefined as a state test
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-46.md:66
**Problem:** The scope derives the reassign-request rule from *the other owner's sessions'
`IssueRef`* (task 19). Decision 2 replaces it with `state ∈ {started, in_review} && assignee ≠ me`
and argues local mode. The reasoning is sound for W1, but the two rules differ in a case the
product cares about: a teammate whose managed team-mode session (OD-25, built alongside) is on an
issue still at `todo` is reassigned **silently**, with no *Ask … to hand over*.
**How to fix:** Record the narrowing in the skeleton beside task 46 (not only in the part), and
name the W3 task that widens it to Fleet's sessions so the rule is not lost.

## Task 47 — the issue view, part 2: revisions, restore, `serves`, pinned links

Scope covered: restore as a new revision, the reading line, `issueLink`/`parseIssueLink` refusing
aliases, `serves` rows that light on hover **or** focus without a selection ring, *dylan · agent 1
read @3* from the reader's own sessions only, 21 tests each with a mutant, counts that add up
(6 + 4 + 7 + 4). `lineDiff`'s LCS and `agentsWhoRead`'s numbering are both correct as written.

### S05-I8 (Important): `CriterionPicker` is called with props task 42 does not have
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-47.md:436
**Problem:** `Serves.tsx` renders `<CriterionPicker exclude={serves} onChoose={(s: Serve) => …}
onClose={…} />`. Task 42 defines `CriterionPicker(props: {issue: {title, description}; criteria;
serving; onChoose(c: BriefCriterion); onClose})` (`task-42.md:126-133`). Three of the five props
differ, `onChoose`'s argument is a different type, and — the substantive half — task 47 passes no
`criteria` and no `issue`, so the picker it mounts has nothing to list and nothing to suggest from.
Task 47's gap 4 marks the props "assumed", but task 42's part is on the tree, so this is a
contradiction between two written parts, not an open question.
**How to fix:** Fix task 47's call to task 42's signature (`issue` from the head revision's title
and description, `criteria` from the same brief-order source task 45 names, `serving={serves}`) and
map its `BriefCriterion` back to a `Serve`; or change task 42's props to `{exclude, criteria,
issue, onChoose(serve), onClose}` and update task 42's own test.

### S05-I9 (Important): two different ops write the same `serves` change
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-47.md:410
**Problem:** Task 42 introduces `{kind: "serves", issueId, add, remove}` for attaching a criterion
(`task-42.md:213`) and task 43's board sends the same op for a cross-criterion drop
(`task-43.md:137`). Task 47 writes the identical change as `{kind: "revise", issueId, baseRev,
patch: {serves}}` (decision 3) and its test pins that shape. Task 56's `WorkOp` union would have to
carry both, `U`'s inverse differs between them, and only the `revise` form carries a `baseRevision`
— so the same act is lost-update-safe from the issue view and not from Triage or the board, where
two people attaching criteria concurrently would clobber each other with no 409.
**How to fix:** Keep one op. `revise {baseRev, patch:{serves}}` is the stronger form (it has the
conflict base task 56 already has to handle); make tasks 42 and 43 build it from the head revision
they already read, delete `{kind:"serves"}` from task 42's op list and from task 44's bulk
*attach a criterion* batch, and say so once in the skeleton's seam note for SW.16.

## Task 48 — relations, the live agent line, *done, not proven*

Scope covered: the five relation words with one record read from both ends, `M`-then-letter kinds
named for task 54, the stalled-agent fact in mono, one line per own agent on the one heartbeat, a
teammate as the person with no dot and no count, the required reason step, `glyphPropsOf` as the
one glyph rule, and the three source-level checks (no attach registry, no terminal, no timer).
The `proof` words match `task-03.md:226-231` and `task-10.md:222-230` exactly.

### S05-I10 (Important): one of the three result sentences the scope names cannot be built
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-48.md:149
**Problem:** The scope requires three rendered result words, the third being *done · c-91e2 still
open (EP-150 not landed)*. `ProofLine`'s `waiting` arm instead reads *landed at 7c1f0a2 · waiting
for evidence on every criterion it serves*: no criterion, no blocking issue. The part's gaps 4 and
5 explain why — no W1 read returns an issue's cited evidence (`issue_proof_citations` stays on the
server) and no replica read lists the issues serving a criterion — and **no task in the phase owns
either read**. The same two gaps also empty the `(test run, exit 0)` parenthesis the scope puts on
the `proven` sentence and that task 41's `causeLine` needs (task 41 gap 3), so the shortfall is
three parts wide, not one.
**How to fix:** Add one Hub task in W1 (a `proofCitations` array on `IssueHead`, or
`GET /v1/issues/{id}/proof`) plus the replica read `issuesServing(criterionId)`, place them before
task 48 in `dependsOn`, and fold task 41's gap 3 into the same task; or amend the scopes of tasks
41 and 48 to drop the two sentences and record the loss against L-24.

### S05-M9 (Minor): a cross-owner `blocked by` writes to the other person's issue with no refusal path
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-48.md:217
**Problem:** `relationOp("blocked_by")` sends `{kind: "relation", issueId: other, add: {kind:
"blocks", issueId: subject}}` — the write's subject is the **other** issue, which may belong to
another team or another person. If the Hub refuses for want of write authority on it, `Relations`
subscribes to no `onRefused`, so the row appears optimistically and then vanishes with no reason
(the same class as S05-I3).
**How to fix:** Name the authorisation the Hub applies to a relation whose subject is another
team's issue (task 11 or task 13's rule), and give `Relations` the `onRefused` line that prints the
refusal beside the add button.

### S05-M10 (Minor): the step list says six test files and the part writes five
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-48.md:777
**Problem:** Step 2 says *the six test files first*; the Files list and the acceptance note both
give five (`reason`, `glyphOf`, `ProofLine`, `Relations`, `IssueAgentLine`, 3+2+6+4+4 = 19 tests).
**How to fix:** Change *six* to *five*, or name the sixth.

## Task 49 — one Markdown editor on `prosemirror-*`

Scope covered: every named dependency with its licence and a lock-file check that fails the build
on a copyleft addition, `html: false`, no images, live schemes limited to `https`/`http`/`panacea`,
the author-class plugin that makes typed, pasted and dropped text the person's, the commit cadence,
the key-isolation tests, and the byte-for-byte round trip. `EditorProps` matches the scope exactly
plus `onOpenIssue`. The two decisions worth keeping are 1 (a mixed text commits as `A`, never
laundered to `H`) and 5 (`Ctrl+Enter` is the cockpit's *New session* chord, stopped here).

### S05-I11 (Important): L-124 is claimed closed while the author class is lost on every reload
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-49.md:28
**Problem:** Decision 1 and gap 1 establish that `AuthoredText` carries one class per field
(`task-02.md:237-242`), so `AuthorSpan`s live only in memory. After a commit and a remount,
`docFromAuthored` re-marks the **whole** field `text·A` whenever any agent span survived: the
paragraphs the person wrote are then drawn with the agent gutter mark and read *text·A · agent*.
The part's own rule 3 ("a person's edit inside it turns only the edited range `text·H`") therefore
holds for one session and is reversed by a refresh, yet the Parity block closes **L-124** (*every
text keeps its author class*) on it. The brief says this phase carries its own Hub deltas as tasks;
no task in the phase changes `issue-revision`'s canonical form to hold spans, so nothing is queued
to close the gap either.
**How to fix:** Either add a W1 Hub-delta task (an `authorship: []AuthorSpan` field on the revision
content, its canonical-form rule and new golden vectors, owned with tasks 02 and 03, placed before
49 in `dependsOn`), or move L-124 to PARTIAL in the coverage table with the in-session scope
written out, and say in task 49's *does NOT claim* that a reload re-marks the field.

### S05-M11 (Minor): the sentinels are literal invisible characters in the fenced source
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-49.md:226
**Problem:** `export const OPEN = "…"`, `CLOSE` and the `clean` character class hold raw U+E000 and
U+E001 (verified: the bytes `ee 80 80` are on line 226). They are invisible in an editor, a diff
and a review; a copy through a normalising tool, a lint autofix or a careless retype drops them
silently, and then `serializeWithAuthorship` finds no spans and every text commits as the person's.
**How to fix:** Write them as escapes — `const OPEN = "\u{E000}"`, `const CLOSE = "\u{E001}"`,
`/[\u{E000}\u{E001}]/gu` — and add one assertion (`expect(OPEN.codePointAt(0)).toBe(0xe000)`) so a
lost character fails a test rather than a revision.

## Task 50 — the slash menu and tables

Scope covered: `/` only at a block start or after whitespace, filter, `Enter`, `Esc` leaving the
literal `/`, every hint read from task 49's one `SHORTCUTS` table with a parity test, a listbox
with `aria-activedescendant`, GFM pipe tables that round-trip, `Tab`/`Shift+Tab` between cells,
*Link an issue* by `is-` id, and a no-scroll test. Decision 2 (link candidates come from the caller,
so the editor reads no replica) keeps the editor free of the Work layer, which is right. Gap 3 in
this part is the independent confirmation of S05-I7 above.

### S05-M12 (Minor): the design puts tables in W2 and this task builds them in W1
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-50.md:1094
**Problem:** The part's own gap 2 records that `05-design.md:271` prices L-127's table part as W2
while the skeleton schedules it here, in W1. Left as written, the coverage table and the design
disagree about what W1 contains, and the 257-row matrix is the phase's coverage checklist.
**How to fix:** Make one of the two right: either mark L-127's table half W1 in the coverage table
with a one-line note, or move the table half of task 50 to a W2 task and leave the slash menu here.

## What this section does well, so it is not undone

- **Every part is present and none is cut off.** Tasks 41-50 all end in *What this task does NOT
  claim* and *Gaps found while writing this part*; sizing is plausible for 2.5-3 hours each, and
  the two smallest (43 at 267 lines, 45 at 223) are small because they reuse tasks 40-42, not
  because they are thin — except for the one hole recorded as S05-I6.
- **Citations are real.** Every cite spot-checked against the tree held: `task-03.md:211-216` and
  `:226-231`, `task-10.md:222-230`, `task-11.md:175-176`, `:184`, `:187-199`, `:379-381`,
  `task-20.md:28`, `:273`, `:304-308`, `:330-331`, `task-26.md:83-105`, `task-31.md:84`, `:108`,
  `task-46.md:113`, `tokens.css:110-121`, `:135-138`, `05-design.md:613-618`, `:885`, `:1006`,
  `:1008`, `:1175`, `:1237`, and `prototype/work-layer.html:201-212`, `:553`, `:632`, `:652`,
  `:662`. Two off-by-one cites (`task-41.md`'s prototype line, task 42's `task-46.md:328-331`) are
  not worth a finding.
- **The arithmetic in the tests holds.** Task 42's snooze (`FIXED_NOW` + the Sydney offset =
  `2026-09-18T23:00:00.000Z`), task 41's `causeLine` clock, task 47's `lineDiff` LCS and agent
  numbering, and task 48's `glyphPropsOf` were each worked through and are right.
- **The security-shaped rules are kept.** Agent output is never a fact source (task 48's
  source-level test against `attachRegistry`, the terminal and transcripts, `OD-8`); an agent is
  never an assignee (task 46, `OD-16`); another person's agents are never steered (tasks 46-48);
  the replica keeps no Hub datum on disk (task 44 test 2 greps the persisted blob); the editor runs
  `html: false` with a scheme allow-list and no `parseDOM` on the agent mark, so a paste cannot
  forge authorship; every added dependency's licence is read from the lock file, not assumed.
- **Nine of the eleven Important findings are cross-part seams, not defects inside a part.** The
  parts each found and named their own half; what is missing is one pass that reconciles them in
  the skeleton. The two that are defects inside a part are S05-I3 (a refused board drop says
  nothing) and S05-I4 (the two-step Start confirm can be satisfied by one held keypress).

## Two things the plan owner should decide once, because five parts wait on them

1. **The pickers' home.** Tasks 42, 43, 44 and 47 all import from task 46, which the skeleton runs
   after them (S05-I1, S05-I2, S05-I5, S05-I8). One decision — move `Picker`, `stateOptions`,
   `handMove` and `DuplicateOfPicker` into a task before 42, or renumber — closes four findings.
2. **The token names.** Six custom properties used across `issue.css` and `editor.css` exist
   nowhere (S05-I7). One decision — map them onto the built `--r-*`, `--e-*`, `--surface`,
   `--t-meta` — closes it in four parts at once, and a one-line structural test keeps it closed.

Section verdict: Needs fixes
# Phase W plan review — section 06 (tasks 51-60)

Adversarial read of `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-51.md` through `task-60.md` against their scopes in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-41-60.md` (lines 331-640) and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

All ten parts exist. Line counts: 51/963, 52/739, 53/640, 54/810, 55/823, 56/833, 57/498, 58/412, 59/384, 60/791.

## Task 51 — comments: the thread, mentions, reactions, anchors, resolve

Scope coverage is good: all eight scope mutants appear as named tests, the four refusals
(no edit, no re-map, names not counts, no scroll jump) each carry a decision and a test, and the
scope's fifth anchor kind is correctly named as a gap rather than invented. Every citation I
checked is real and exact: `task-12.md:27-31` (anchors checked against the revision they name),
`:106-113` (the four anchor shapes, oldest-first list), `:141` (`MaxMentions = 20`), `:167-179`
(`ValidReactionEmoji`), `:68-69` (no `PATCH`, no `DELETE`). The anchor constructors match
`ValidateAnchor` (`task-12.md:222-260`) field for field, including the empty-string `criterionId`
and `ref` that the Go switch accepts.

### S06-I1 (Important): task 51's comment CSS uses two custom properties that do not exist, so the comment card renders with no ring at all

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-51.md:656
**Problem:** the appended `issue.css` block writes `border-radius: var(--r-sm)` and
`box-shadow: 0 0 0 1px var(--ring)` (`:656-657`), and `box-shadow: inset 2px 0 0 var(--ring)`
(`:663`) and `var(--r-sm)` again at `:671`. Neither `--r-sm` nor `--ring` is defined:
`desktop/src/styles/tokens.css:110-113` defines `--r-chip`, `--r-ctl`, `--r-card`, `--r-window`,
and the ring lives in `--e-hair`, `--e-card` and friends (`tokens.css:117-121`); the only `ring`
name is `--accent-ring` (`tokens.css:68`). An undefined custom property is invalid at
computed-value time, so both declarations fall back to their initial values: radius 0 and
`box-shadow: none`. A comment then has a square corner and **no edge at all**, which is the exact
failure DESIGN-SYSTEM.md forbids ("Elevation is a 1px ring plus layered shadow"), and no test in
this part looks at CSS, so it ships silently. Task 52 spotted the same thing from the outside
(`task-52.md:733-735`) but cannot fix another part's file.
**How to fix:** in task 51's `issue.css` block use `--r-card` for the comment card and
`--r-chip` for the picker row, and replace both hand-rolled `0 0 0 1px var(--ring)` shadows with
`var(--e-card)` (the card) and `var(--line)` (the inset quote bar). Add one acceptance line:
`git grep -nE "var\(--(r-sm|ring)\)" -- desktop/src/components/issue` expecting no output.

### S06-M1 (Minor): task 51's acceptance says "2 files" where the part adds six

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-51.md:922
**Problem:** the acceptance comment reads `this part adds 2 files and 17 tests`, but the **Files**
list (`:70-79`) adds six new files (`commentModel.ts`, `commentModel.test.ts`,
`mentionPlugin.ts`, `CommentComposer.tsx`, `Comments.tsx`, `Comments.test.tsx`). A reviewer
checking the count against `git status` reads a mismatch and cannot tell which number is the
plan and which is the slip.
**How to fix:** write `2 test files (6 new files in all) and 17 tests`.

## Task 52 — attachments by blob digest; subscribe and unsubscribe

Strong part. Every hub-contract citation is exact against
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md`: `:590-591` are
routes 28 and 29, `:559` is "`{sha}` is 64 lowercase hex", `:260-261` are `quota_exceeded` and
`blob_too_large` with `limitBytes`, `:850` is the ten-type `Content-Type` allowlist (which
`HUB_BLOB_TYPES` reproduces exactly, GIF and PDF correctly absent), `:853-854` is the streaming
re-hash and `digest_mismatch`. `ADR-H1:467-474` really does say neither renderer nor control
plane persists Hub data to disk. The sniff-not-type rule (decision 3) is the right shape for the
SVG question, and hashing before the `PUT` (decision 1) is the right order.

### S06-I2 (Important): nothing in task 52 names the Hub janitor, which deletes an attached blob 24 hours after upload

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-52.md:56
**Problem:** the Hub contract says plainly that "an association that no **design approval**
references is deleted by the janitor (§12.5) 24 hours after upload; a blob with no association
left is deleted with it"
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:868-870`). An issue
attachment is not a design approval, so on the contract as written every attachment's bytes are
swept the day after they are uploaded and `fetchVerified` starts answering `blob_not_found`.
Task 06 saw this (`task-06.md:60-63`: "the janitor's reference list must name
`issue_attachments`") and explicitly disclaims owning the change (`task-06.md:536`), and task 07
disclaims it too (`task-07.md:580`); `grep -l "reference list"` over all 187 parts matches only
`task-06.md`, so **no task in Phase W owns it**. Task 52 is the part that depends on the answer
and it is silent: decision 8 ("attachments are fetched, not replicated") and decision 5 ("bytes
live in memory; nothing is cached") together mean the Hub blob is the only copy. Best case the
FK fires and the janitor's `DELETE` errors with `23503` every sweep; worst case the sweep is a
`DELETE` on `blobs` that cascades.
**How to fix:** add a decision to task 52 that names the dependency and the owner
("attached blobs survive only if the janitor's reference set names `issue_attachments`; task 06
declares the key, the janitor edit belongs to task &lt;id&gt;"), add it to *What this task does NOT
claim*, and raise a task row (or widen task 09's scope) for the one-line janitor change plus a
Go test `TestTheJanitorSkipsAnAttachedBlob`.

## Task 53 — keys, part 1

The hardest constraint in this range is handled correctly and for the stated reason: `CHORD_KEYS`
and `SHIFTED_CHORD_KEYS` are untouched and the extension is a sibling branch on the existing
early return, so no key is taken from xterm. I verified every line citation against the real
files: `useKeyboardMap.ts:55-56` are the two frozen lists, `:63-71` is the "a key added to the
two lists is a key the agent stops receiving" comment, `:126` is exactly
`if (!isCockpitChord(event)) return;`, `:14-17` is the Escape note, `prefsSlice.ts:40`, `:69`,
`:79`, `:155-158` and `store.ts:21-28`, `:30-33`, `:34-48` all land where the part says, and
`commands.ts:129-144` is the OD-9 toggle row (a more precise pointer than the scope's
`:19-28`). `r()`'s default `shift = false` means every record letter really does test
`shiftKey === false`, so rule 2 holds against `Shift`+letter, and the three `Shift` rows in
`KEYMAP` are all agent verbs, which is the structural test the scope asks for.

### S06-I3 (Important): task 53's acceptance forbids `isBareListKey` in `WorkLens.tsx`, but its own decision 8 keeps task 44's `Esc` handler there

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-53.md:606
**Problem:** decision 8 (`:79-83`) says "**`Esc` stays on task 44's list element**: the window map
never handles Escape … so clearing a selection with `Esc` remains the list's own handler, and
`KEYMAP` has no `Esc` row". Task 44 puts that handler in `WorkLens.tsx` and guards it with
`isBareListKey`: "add `X`, `Shift+ArrowDown`/`ArrowUp` and `Esc` to the lens's list `onKeyDown`
after `isBareListKey`" (`task-44.md:269-271`), and `task-44.md:75` names `WorkLens.tsx` as the
file. The acceptance block then runs
`git grep -n "isBareListKey" -- desktop/src/components/work/TriageView.tsx desktop/src/components/work/WorkLens.tsx`
with `# expected: no output`, and hedges it in the same breath ("task 44's `Esc` and its confirm
keys, **if they still use it**, live elsewhere"). A faithful implementation — `X` and `Shift+↓`
moved into `KEYMAP`, `Esc` left where decision 8 says to leave it — fails this acceptance
command, and an implementer who makes the command pass has to move `Esc` somewhere the part
never names. An acceptance command whose expected output depends on an unanswered question is
not an acceptance command.
**How to fix:** decide it in the part. Either keep `Esc` in `WorkLens.tsx` and change the
command to assert the *absence of the moved keys* instead — e.g.
`git grep -nE "\"KeyX\"|ArrowDown" -- desktop/src/components/work/WorkLens.tsx` expecting no
output, with a sentence saying the one surviving `isBareListKey` use is `Esc`'s — or give `Esc`
its own home and say which file, and keep the grep as written.

### S06-M2 (Minor): the Files list omits `keymap.cycle.test.ts`, which the acceptance counts

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-53.md:88
**Problem:** **Files** names `desktop/src/work/keys/keymap.test.ts` and no cycle file, but the
acceptance expects `2 files (keymap.test.ts, keymap.cycle.test.ts), 11 tests` (`:590-592`) and the
gap list calls `keymap.cycle.test.ts` a *skeleton* omission (`:624`) rather than one of its own.
The implementer reads Files, writes one test file, and the acceptance count is wrong.
**How to fix:** add `desktop/src/work/keys/keymap.cycle.test.ts` (new) to **Files**, and say in
one line why it is a second file (a cycle/lint test that imports the whole table).

### S06-C1 (Critical): the replica throws away the subscription cause, so task 52's subscribe line and its named mutant cannot be built

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-55.md:119
**Problem:** scope 52 requires "the head shows the Hub's reason (*Subscribed · you commented*)"
and its named mutant is "the subscribe reason is the Hub's field verbatim (**kills a reason
guessed on the desktop**)" (`tasks-41-60.md:381-386`). Task 52 builds exactly that:
`useSubscription(issueId)` returns `Subscription | null` from `useWork((r) =>
r.subscription(issueId))` and `Subscribe.tsx` renders `CAUSE_WORDS[sub.cause]` and
`data-cause={sub.cause}` (`task-52.md:396-411`, `:415-419`). Task 55 — the part that owns the
replica, and which did not exist when task 52 was written — supplies neither:
- `ReplicaDeps.subscriptions(): Promise<string[]>` is **ids only** (`task-55.md:118-119`), and
  `hubReplicaDeps` discards the rest on purpose:
  `.filter((s) => s.subscribed).map((s) => s.issueId)` (`task-55.md:459-460`). The `cause` is
  dropped, and every `subscribed: false` row is filtered out, so task 52's *Not subscribed ·
  your choice* state (an explicit unsubscribe) is indistinguishable from "no row at all".
- `Replica` exposes only `subscribed(issueId): boolean` (`task-55.md:135`); there is no
  `subscription(issueId)`, and `WorkReader` (`task-40.md:140-157`) has none either.
- `ReplicaWriter.setSubscribed(issueId, on: boolean)` (`task-55.md:233`) carries no cause, so
  task 52's optimistic paint of `{subscribed, cause: "manual"}` has nowhere to land.
Task 52 filed this as its own gap 4 (`task-52.md:729-732`) addressed to task 55, and task 55's
part does not answer it — it is not in task 55's gap list either. As the plan stands, an
implementer of task 52 either fails typecheck or invents a cause on the desktop, which is the
mutant the scope names.
**How to fix:** in task 55, widen `ReplicaDeps.subscriptions()` to
`Promise<readonly Subscription[]>` (keep every row, `subscribed: false` included), hold them in a
`Map<string, Subscription>`, add `subscription(issueId): Subscription | null` to `Replica` (and,
if surfaces read it through `useWork`, to `WorkReader`), keep `subscribed(id)` as
`subscription(id)?.subscribed === true`, and change `ReplicaWriter.setSubscribed(issueId, on,
cause: SubscriptionCause)`. Add a task 55 test that a row whose `subscribed` is `false` survives
the load with its cause. Then delete task 52's gap 4 and point its `useSubscription` at the real
name.

### S06-C2 (Critical): `mutate` has two incompatible homes, and task 56 — the part that owns the choice — picked the one seven other parts do not import

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-56.md:346
**Problem:** task 56 declares the write half as **methods on the replica instance**:
`interface ReplicaWrites { mutate(op | readonly op[]): MutateHandle; … }` with the comment
"`interface Replica extends ReplicaWrites` — task 55's `Replica` gains these members"
(`task-56.md:346-358`), every one of its own tests calls `r.mutate(…)` (`:554-680`), and its
Files list puts the implementation in `replica.ts`, not `ops.ts` (`:74`). `ops.ts` exports
`WorkOp`, `applyLocal`, `revertLocal`, `sendOp`, `undoable`, `inverseOf` (`:321`) and **no
`mutate`**: `grep -n "export function mutate|export const mutate" task-56.md` is empty.
Meanwhile seven parts write `import { mutate } from "…/work/replica/ops"` and call it as a free
function — tasks 47, 48, 49, 51, 52, 60 and 68 (e.g. `task-51.md:255`, `task-52.md:391`). Every
one fails to compile against task 56 as written. Task 51 saw it and filed gap 1 ("`mutate` has
two homes … one module-level export is needed; **task 56 owns the choice**",
`task-51.md:951-953`); task 56's own gap list (`:825-833`) does not mention it, so the question
was asked and never answered. This is not a rename: `r.mutate` needs a replica handle in scope,
which none of the seven consumer components has.
**How to fix:** task 56 decides in one line and prints the module-level wrapper. Cheapest that
keeps both sides: keep `ReplicaWrites.mutate` as the implementation and add to `ops.ts`
`export const mutate = (op: WorkOp | readonly WorkOp[]): MutateHandle =>
requireReplica().mutate(op);` (with `onRefused` beside it), on the same singleton accessor task
55's `startReplica`/`setWorkSource` already installs (`task-55.md:486-492`, `task-40.md:158-165`),
and a test that calling it before `startReplica` throws a named error rather than silently
dropping a write. Then delete task 51's gap 1.

### S06-I4 (Important): peek on `Space` ignores the *Single-key shortcuts* switch, so the accessibility escape hatch task 53 built does not cover the key task 54 advertises on the sheet

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-60.md:232
**Problem:** task 53's decision 7 is that `singleKeyShortcuts` off "turns off every single key
and, because task 54's sequence state is fed only by `dispatchListKey`, every sequence too"
(`task-53.md:73-78`), and the check lives inside `dispatchListKey`
(`if (!useCockpit.getState().singleKeyShortcuts) return;`, `task-53.md:301`). Task 60 does **not**
go through `dispatchListKey`: `usePeekController` is a pair of React `onKeyDown`/`onKeyUp`
handlers bound on the list element, and its only gate is `if (!isListKeyTarget(e.target)) return;`
(`task-60.md:211`, `:232`), with decision 3 saying in as many words "This part adds no gate of
its own" (`:55`). `Space` is a single key. With the switch off, `Space` still opens peek and
still calls `e.preventDefault()` — and task 54's generated sheet prints *"`Space` peeks: tap to
open, hold to look"* right beside the switch's state (`task-54.md:423`, test at `:674`), so the
sheet tells the reader single keys are off while one of them keeps firing. WCAG 2.1.4 asks for a
mechanism to turn **character key shortcuts** off; a mechanism that misses one is not one.
**How to fix:** one line in `usePeekController.onKeyDown`, beside the `isListKeyTarget` gate:
`if (!useCockpit.getState().singleKeyShortcuts) return;` — or better, export a named predicate
from task 53 (`listKeysAreOn(target)` = `isListKeyTarget(target) && singleKeyShortcuts`) and have
both `dispatchListKey` and peek call it, so a third caller cannot get it wrong. Add task 60's
test: with the switch off, `Space` on a row opens nothing and is not prevented; and change task
54's sheet so the *Space peeks* line is omitted when the switch is off, the same as every other
single key.

### S06-M3 (Minor): a focused `<button>` inside a work list is a list key target, so `Space` on task 40's title button peeks instead of activating it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-53.md:247
**Problem:** `isListKeyTarget` excludes only terminal hosts and `isTextTarget`, and
`isTextTarget` (`task-41.md:324-328`) covers contenteditable, `input`, `textarea` and `select` —
not `button`, `a[href]`, `[role="button"]` or `summary`. Task 40 renders the row title as a real
`<button type="button" className="wk-title" onClick={() => openIssue(issueId)}>`
(`task-40.md:677`), which Tab reaches. Focus it and press `Space`: task 60's controller calls
`preventDefault()` and peeks, and the button never activates — `Space` is the only keyboard way
to press a `<button>`. `Enter` on the same button is the milder twin: task 53's `open.issue`
(`KEYMAP`, scope `issueRow`) runs and prevents the default, which happens to do the same thing
today and will not the first time a row carries a second button.
**How to fix:** either widen `isTextTarget` (or add `isActivationTarget`) to return true for
`button`, `a[href]`, `summary` and `[role="button"|"link"|"checkbox"]`, so a focused control keeps
its own `Space` and `Enter`; or make the title a `<span>` inside a row that is itself the only
tab stop, and say so in task 40. Whichever, add the case to task 53's
`keymap.test.ts`: `Space` and `Enter` on a focused in-row button are not prevented.

### S06-I5 (Important): four of task 59's palette rows name verb ids that do not exist, one of them duplicating a row the generator already emitted

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-59.md:134
**Problem:** decision 1 makes the work rows generated from `KEYMAP`, `SEQUENCES` and `KEYLESS`,
and decision 1's own safety net is "any name this part guessed is caught by `tsc`, because
`VerbId` is a union" (`:152`). Four names are guessed, and `tsc` catches all four, which means
this part does not build as written. `VerbId` is task 53's union (`task-53.md:111-119`) plus task
54's eleven `go.*`/`open.*` ids (`task-54.md:232-233`); it contains none of these:
1. `"changes.apply"` in `KEYLESS` (`:141`) is task 53's **`held.apply`** (`task-53.md:177`, and
   the runner task 62 registers, `task-62.md:335`). Worse than a typo: `held.apply` is already in
   `KEYMAP`, is not in `NAV_ONLY`, and so is pushed by the first loop with label *Apply held
   changes* and hint `N`. Rename it and the `push`-skips-what-it-has-seen rule drops the second
   row; leave it and the palette shows *Apply held changes* twice, the second with no hint and no
   registered runner.
2. `"peek.toggle"` in `NAV_ONLY` (`:134`). Task 60 binds `Space` as its own React handler and
   registers no verb at all (`task-60.md:208-240`); nothing anywhere defines `peek.toggle`.
3. and 4. `"issue.subscribe"` and `"issue.unsubscribe"` in `KEYLESS` (`:139-140`). Beyond the
   union, **no part registers a runner for either**: task 52 builds subscribe as a plain button
   calling `mutate` (`task-52.md:406-411`) and states "subscribe has **no single key** … it is a
   palette verb in task 59" (`tasks-41-60.md:385-386`). By task 53's decision 3 `runVerb` returns
   `false` and does nothing — silent on a key, but on a **clicked palette row** the reader presses
   *Subscribe* and gets no change and no message.
Separately, the "What the reader sees" list promises *Comment*, *Copy link* and *Move to project*
(`:18-21`), and none of the three is in `KEYMAP`, `SEQUENCES` or `KEYLESS`: *Copy link* exists
only as a button in the issue view (`task-47.md:515-516`), and `open.project` ("Open a project",
`task-54.md:133`) is navigation, not moving an issue between projects.
**How to fix:** (a) rename `changes.apply` to `held.apply` and delete it from `KEYLESS`, since
the `KEYMAP` loop already emits it; (b) drop `peek.toggle` from `NAV_ONLY`, or have task 60
register a real `peek.toggle` verb and add it to `VerbId`; (c) add `issue.subscribe`,
`issue.unsubscribe` (and `issue.comment`, `copy.link`, `issue.moveProject` if the reader-facing
list is to stand) to task 53's `VerbId`, and name in each owning part the component that calls
`useVerb` for it — task 52 for the two subscribe verbs, task 47 for `copy.link`, task 51 for
`issue.comment`; (d) add one structural test to task 59: every `KEYLESS` and `NAV_ONLY` id has a
registered runner after the W1 surfaces mount, so a palette row that does nothing fails the
suite instead of the reader.

## Task 54 — keys, part 2

Covers its scope. The sequence window is a timestamp compared on the next key, not a timer, so
the `git grep setTimeout` acceptance is honest; `menuAim.ts` is plain triangle geometry written
from the idea, which is the right answer to `OD-11`; the sheet is generated from `KEYMAP`,
`SEQUENCES` and `DEAD_LINEAR_CHORDS` and its test asserts the rows equal the active ones, so it
cannot drift. Two counts that could have clashed do not: task 54 says "Task 53's
`keymap.cycle.test.ts` gains a" case (`:244`) and its acceptance expects 3 where task 53 expected
2. Its gap 2 (no part builds a submenu, so `L-227` ships a tested primitive with no consumer) is
an honest call rather than a false close. No findings of its own beyond S06-I5, which lands on
the palette that reads its `chordFor`.

## Tasks 55-58 — the replica

Read as a group. The strongest work in the section: `hub-contract.md:333`, `:347`, `:349`,
`:357`, `:1099`, `:1101`, `:1103-1104`, `:1140-1141` are all exact, and task 57's decision 4
("a gap is never read from `seq`", because the org sequence is gapless but the feed is filtered
per caller) is the correct reading of `:1099` against `:1103` and is the kind of thing a plan
usually gets wrong. Task 57's decision 3 fallback — the contract does not say whether the feed
filter reads membership at write time or read time, so every resync re-lists and a project
missing from the new list is dropped exactly as a removal is — is a genuine adversarial catch
against the contract, not a restatement of it. Task 58's "an alias is formatted, never parsed"
holds the `05 §3.6` line and is checked against `task-02.md:187-189` and `task-14.md:270-276`.
Two things about task 52's `Subscribe` tests worth naming beside S06-C1: they render
`<Subscribe sub={…}/>` with a literal prop and never touch `useSubscription`, so the suite goes
green on a replica that cannot supply a `cause` — the test does not defend the mutant its scope
names.

### S06-M4 (Minor): the same record shape has two names in the same file, `ReplicaProject` and `ProjectScope`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-57.md:95
**Problem:** task 55 declares `export interface ReplicaProject { projectId: string; teamId:
string; key: string }` and `ReplicaDeps.readableProjects(): Promise<ReplicaProject[]>`
(`task-55.md:93`, `:110`). Task 57, written before task 55's part landed, declares
`export interface ProjectScope { projectId: string; teamId: string; key: string }` in
`visibility.ts` and `ReplicaVisibilityDeps.readableProjects(): Promise<ProjectScope[]>`
(`task-57.md:95`, `:178`) — field for field the same type, a second name for it, and a second
`readableProjects` on a second deps interface, both of which end up wired into the one
`replica.ts`. Task 58 then imports the task 57 name (`task-58.md:6`, `:116`). Structurally
harmless, but it guarantees a merge-time rename across three parts and a reviewer asking which
of two `readableProjects` a call site means.
**How to fix:** task 57 drops `ProjectScope`, imports `ReplicaProject` from `replica.ts` and
folds its deps into task 55's `ReplicaDeps` (task 57 already says task 55 wins). Task 58's
`scope()` reference follows. One line in each of the three parts.

### S06-M5 (Minor): task 55's timer grep has no test exclusion, so task 56 turns it red

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-55.md:781
**Problem:** task 55's acceptance runs
`git grep -nE "setInterval|setTimeout" -- desktop/src/work/replica` expecting no output, while
the very next grep in the same block does add `":!*.test.*"`. Task 56 writes
`await new Promise((t) => setTimeout(t, 0));` in `desktop/src/work/replica/mutate.test.ts`
(`task-56.md:591`), which is a legitimate microtask flush. Task 55's acceptance is green when it
runs and red for anyone who re-runs it after task 56, which is what a reviewer does.
**How to fix:** add `":!*.test.*"` to the first grep, matching the second.

## Task 59 — the palette

The generation rule (one source, `KEYMAP` plus `SEQUENCES`, hint from `chordFor`) is right and
its first test — every work command's hint equals `chordFor` across all active rows — really does
kill the drifting hand-written hint. The four built-palette facts it extends are cited and were
verified on the tree on 2026-09-27 (`commands.ts:72` `buildCommands`, `:219` `filterCommands`,
`:17` `Command.hint`, `CommandPalette.tsx:4` by-id selection). Its one defect is S06-I5.

## Task 60 — peek on `Space`

`OD-8` and `OD-15` are handled the way the scope asks and better: source tests assert
`PeekPerson.tsx` matches none of `/attachRegistry|xterm|sessions|useCockpit|transcript/`
(`:689`) and `Peek.tsx` none of `/api\/attachRegistry|@xterm|TerminalPane/` (`:693`), and the
part refuses 02 C9's twelve transcript lines outright because "no W1 source satisfies `OD-8`"
(`:769-770`) rather than inventing one. `peekFor` is pure and ordered, `HOLD_MS` is declared a
design value rather than a measurement, and nothing starts an interval. Its defects are S06-I4
and, shared with task 53, S06-M3.

## What I checked and found sound

- **Citations.** I spot-verified 30-odd line citations across the ten parts against the real
  files — `2026-09-23-hub-contract.md`, `ADR-H1-hub-architecture.md`, `desktop/src/styles/tokens.css`,
  `desktop/src/hooks/useKeyboardMap.ts`, `desktop/src/store/store.ts`,
  `desktop/src/store/slices/prefsSlice.ts`, `desktop/src/components/palette/commands.ts`, and
  sibling parts 02, 06, 07, 09, 12, 13, 14, 20, 37, 40, 41, 44, 47, 48, 62. Every one landed on
  what the part said it did except the two I raise. The parts also mark UNVERIFIED honestly:
  `ValidReactionEmoji`'s base ranges, the WebView2 download flow, task 16's missing blob relay
  row, `01 §15.1`'s Linear column, and each dependency whose part was not yet on the tree.
- **Sizing.** All ten are 2.5-3 h, inside the 1-3 h rule, and each has one `#### Acceptance`
  block with expected output. None is cut off; all ten end with a *Gaps found while writing this
  part* list.
- **Tests red first, with named mutants.** Every part names its mutants, either inline
  (`// kills: …`) or as a Mutant table (52, and the summary tables in 53-60). Task 51 alone
  carries 16.
- **Security.** Agent edits stay proposals: task 51 decision 4 routes the comment body through
  task 49's `toAuthored`, so any surviving agent span makes the whole comment `"A"`, and task 60
  never reads a transcript. The provenance thread holds: anchors carry the revision they were
  made against and are never re-mapped (51), erase keeps key, digest and number and loses only
  the words (57), and search's index is derived so an erased record has no tokens (58). The
  local-mode boundary is not crossed by any of the ten. Task 57 is the only part that reasons
  about per-caller authorisation, and it reasons correctly.
- **Not findings, checked and cleared:** task 51's anchor constructors against
  `ValidateAnchor` (`task-12.md:222-260`) — the empty-string `criterionId` and `ref` pass every
  arm; `threads()` orphan handling against `TestAReplyToAReplyIs422` (`task-12.md:480`) — the Hub
  guarantees one level, so only a reply whose root is off the page becomes a root; task 53's
  `r()` default `shift = false`, which is what makes rule 2 hold; `HUB_BLOB_TYPES` against the
  contract's ten (`:850`) — exact, GIF and PDF correctly sent as `application/octet-stream` and
  still previewed by magic number.

Section verdict: Needs fixes
# Phase W plan review — section 07 (tasks 61-70)

Reviewer pass: adversarial, read-only. Parts read one at a time in line ranges against their scope in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-61-80.md` and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

Findings follow, appended per part.

## What was read

All ten parts (`task-61.md` … `task-70.md`, 7,722 lines) against
`…-scopes/tasks-61-80.md:25-360` and the skeleton `…-phase-w-work-layer.md` (rows 461-470,
seams SW.10, SW.16, SW.17, the file map `:302-322`). Every part is complete: each has Files,
COMPLETE fenced interfaces, tests-first with named mutants, Steps, Acceptance with expected
output, *What this task does NOT claim* and *Gaps found while writing this part*. None is cut
off. Scope test coverage is 1:1 or better in all ten (61: 9/9 as 19; 62: 8/8 as 12; 63: 8/8 as
11; 64: 7/7 as 17; 65: 7/7 as 16 + 5 Go rows; 66: 7/7 as 12; 67: 7/7 as 8; 68: 7/7 as 15;
69: 7/7 as 13; 70: the sweep plus 8 check rows).

Citations spot-checked against the real tree and found correct: `heartbeat.ts:64`,
`useKeyboardMap.ts:90` (`isInsideTerminal`), `httpx/problem.go:33` (`Details map[string]string`),
`store/types.ts:155` and `:173` (`Lens`), `registry/session.go:88-89`, `:98-99`,
`reaper/reaper.go:630-640`, `api/client.ts:153` (`restartSession`), `api/sessions.go:723`,
`lib/time.ts:49-52` (`formatWaiting`), `StatusDot.tsx:56-58`, `Toasts.tsx:19`, `:42`,
`AppShell.tsx:98`, `AGENTIC-UX.md:142-149`, `prototype/work-layer.html:758-787`,
`05-design.md:824-840` and `:1313-1321`, P4 `task-17.md:49-61`, P3 `task-19.md:140`,
`task-24.md:92-115`, `task-25.md:99-110` and `:433-434`, `task-13.md:145-152`, `task-37.md:118`.

Three things the parts got right that the scope got wrong, and said so: task 65 restores 05
§3.7 rule 2's *"pauses **nothing** and moves nothing"* where the scope dropped the *nothing*;
task 70 finds a fifty-first W1 row (L-175) the scope's list omitted; task 61 replaces the
scope's invented `StartResult` with task 24's real wire.

## Findings

### S07-I1 (Important): `classify(e).detail` does not exist; four reference-implementation lines will not compile, and the Hub's own sentence renders as `undefined`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-64.md:336

**Problem:** `classify` is `(error: ApiError) => ProblemKind`, and `ProblemKind` is a **string
union** — `"control-plane-down" | "backend-down" | "server-said"`
(`desktop/src/api/problems.ts:12-22`, `:37-40`, verified on the tree). It has no `.detail` and
no `.status`. Four lines call it as an object and one test asserts that shape:

- `task-64.md:336` and `:341` — `catch (e) { setSaid(classify(e).detail); }` on *Restart from
  checkpoint* and *Abandon*;
- `task-64.md:559` — test 11, *"shows the control plane's own sentence from `classify(e).detail`"*;
- `task-69.md:148` — `catch (e) { setFailed(classify(e).detail); setStep("ready"); }` in
  `ImportDialog`, and `task-69.md:311` — test 4 asserting it;
- `task-68.md:502` — test 6 stubs the rejection as `problems.classify → {status:422,
  detail:"title is required by this team's template"}`, the same wrong shape.

Two failures, not one. `tsc` fails on `.detail` (TS2339) and again because `catch (e)` binds
`unknown`, which is not `ApiError` under `strict` (`tsconfig.json`). Were the type erased, the
surface would print `undefined` in the one place these three parts each promise the server's own
words: task 64's test 11 mutant is *"a written-here sentence, so the real reason never reaches
the person"*, and this code is that mutant with a different spelling. Task 68's test 6 would
also pass against an implementation that renders nothing, because the stub matches the code.

**How to fix:** follow the tree's own pattern, which `problems.ts:24-34` spells out: `switch
(classify(error))` over the union with the prose at the call site, as
`desktop/src/components/develop/SessionPane.tsx:49`, `EmptyDevelop.tsx:58` and
`sessionVerbs.ts:86` all do. The server's sentence is `error.message` on `ApiError`
(`desktop/src/api/client.ts:58-66`), narrowed with `e instanceof ApiError` first. In task 64 and
task 69 replace `setSaid(classify(e).detail)` with a small `saidOf(e)` that returns
`e instanceof ApiError && classify(e) === "server-said" ? e.message : <this surface's own
sentence for that kind>`; in task 68 restate test 6's stub as an `ApiError(422, …)` and assert
`e.message` is rendered verbatim.

### S07-I2 (Important): `DeskZero` feeds task 64's `LandedPayload` straight into `zeroSentence`, whose fields it does not have — W1's demo's last frame renders *"undefined landed_proven at 14:02."*

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-65.md:900

**Problem:** task 65 writes `zeroSentence({ landed: newest ? newest.payload : null, live:
props.live, time: props.time })`, where `ZeroFacts.landed` is `{ alias: string; word: string; at:
string }` and `word` is *"the SW.4 result word verbatim (`landed and proven`)"*
(`task-65.md:540-543`, `:565-568`). Task 64's `LandedPayload`, now on the tree, is `{ head:
IssueHead; word: LandedWord; at: string; openSibling }` (`task-64.md:131-136`): there is **no
`alias`** (it is `head.alias`) and `word` is a **code** — `"landed_proven" | "done_proven" |
"done_self" | "done_unproven" | "done_no_criterion" | "landed_waiting"` (`task-64.md:127-129`).
The code→phrase table lives in `LandedRow.tsx` as `WORDS[…]`, is not exported, and task 64's own
acceptance pins it there with `grep -c "WORDS\[" …/LandedRow.tsx  # expected: 1`
(`task-64.md:612-613`).

So `zeroSentence` composes `${f.landed.alias} ${f.landed.word} at …` from a payload that
supplies neither: the zero state reads *"Nothing needs you. undefined landed_proven at 14:02."*
Task 65 anticipated a rename (`task-65.md:565-568`, *"`DeskZero` adapts task 64's payload in one
line if its part names them otherwise"*) but its code block does not adapt, and its gap 3 lists
task 64 as *"no part on the tree"* — it is on the tree now, and it disagrees.

This is not cosmetic: task 70's W1 sweep asserts the exact sentence,
`expect(text).toBe("Nothing needs you. EP-142 landed and proven at 14:02.")`
(`task-70.md:503-506`), and calls it *"the demo's last frame"* — 05 §5.3's closing frame and the
row that closes L-112. On the plan as written that test is red and W1's demo cannot be signed
off.

**How to fix:** in task 64, export the word table from `work/desk/agents.ts` as
`export const LANDED_WORDS: Record<LandedWord, string>` and have `LandedRow.tsx` import it
(its acceptance grep then points at `agents.ts`). In task 65, make `DeskZero`'s one adapting
line explicit: `landed: newest ? { alias: newest.payload.head.alias, word:
LANDED_WORDS[newest.payload.word], at: newest.payload.at } : null`. Add the round trip to task
65's `DeskZero.test.tsx` — build the facts from a real `LandedPayload` fixture, not a literal —
so the mutant *"the payload passed through unadapted"* is killed where the bug actually is, not
only in task 70's sweep two dozen tasks later.

### S07-I3 (Important): task 65 is two tasks wearing one budget — a Go Hub delta plus five desktop surfaces in a claimed 3 hours

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-65.md:1

**Problem:** the part is 1,423 lines against 647 (task 63) and 651 (task 64), each also budgeted
3 h, and it is the only W1 desk task that crosses the Go/TypeScript line. It builds, in one
task: a `hubapi` type, an `IssueHead` field, two event kinds with their data structs, two
`hubclient` methods, two `WorkRoutes` rows, two `workCalls` entries and two `methods_work.go`
methods (`task-65.md:106-187`); the fake's two methods plus three clearing lines
(`:188-288`); a new Go conformance file `work_reassign.go` with five rows (`:1168-1304`); the
desktop wire in `api/work.ts` and `client.ts` (`:289-336`); two replica op kinds on `ops.ts`
(`:748-768`); `informs.ts` in three parts (`:337-567`); `answer.tsx`, `InformRow.tsx`,
`ReassignDecide.tsx`, `ReassignMark.tsx`, the filled `DeskZero.tsx`, two `deskRows.tsx` blocks
and the CSS (`:568-987`); and 16 desktop tests (`:988-1167`). Its own Steps list is seven
bullets, four of which are each a task 63-sized piece of work.

The house rule is 1-3 focused hours per task, and the watchdog and the spend limit both punish
a task that runs long: a part this size is the one most likely to be abandoned half-built, which
is worse than either half shipped whole. It is also the task whose Hub half is *admittedly
absent* — its gap 1 asks for a WH task to own the store column, migration, real route and matrix
rows — so the Go work here is a fake and a conformance suite with no server behind it, sitting
in the same commit stream as the Desk's last row kinds.

**How to fix:** split it in the skeleton's task table, keeping the numbering room W1 already
has (`:466` onward): **65a — the reassign request: `hubapi`, `hubclient`, the fake's rules, the
five conformance rows and the two desktop wire calls** (depends on 16, 37, 56; hands over
`RequestReassign`/`AnswerReassign` and `IssueHead.reassignRequest`), and **65b — the Desk's
informs, the reassign decide, answer in place and the zero state** (depends on 63, 64, 65a).
Both stay inside 3 hours, 65a's Go acceptance (`go test ./control-plane/... -count=1`) is
separable from 65b's desktop acceptance, and the WH task the part asks for attaches cleanly to
65a. Carry the part's own *Reconciliation with task 62's part* section (`:1381-1423`) into 65b
unchanged.

### S07-M1 (Minor): task 64 imports `Session` from `api/client`, which does not export it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-64.md:123

**Problem:** `import type { Session } from "../../api/client";` at the head of
`work/desk/agents.ts`. On the tree, `desktop/src/api/client.ts:10-28` **imports** `Session` from
`../store/types` and re-exports nothing (no `export type {…}`, no `export *`; grepped). Task 62,
which owns the `DeskSnapshot` this file reads, gets it right: `import type { Session } from
"../../store/types"` (`task-62.md:113`). So `agents.ts` fails `npm --prefix desktop run
typecheck` — task 64's own third acceptance line — on its first import, before any of the
seventeen tests run.

**How to fix:** change the import to `"../../store/types"`, matching task 62. Nothing else in
the part depends on the path: `Session`'s members it uses (`status`, `exitReason`, `updatedAt`,
`lastOutputAt`, `issueRef`) are all on the `store/types` declaration (`:140-164`).

### S07-M2 (Minor): the phase 4 plan citation is three lines off

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-67.md:154

**Problem:** *"after P4-17's mount (P4 plan `:364`)"*. In the phase 4 worktree the
`SessionPaneHeader.tsx` row is at
`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md:367`, and
`ContextChip.tsx`'s file-map row is at `:335`, not the `:332` the scope
(`tasks-61-80.md:245`) and the skeleton (`…-phase-w-work-layer.md:280`, `:330-332`) both give.
The worktree is clean at that path, so the lines are the real ones and the citations are stale
by three. Everything else task 67 cites checks out, including P4 `task-17.md:49-61`
(`ContextChipProps`) and P3 `task-19.md:140` (`LaneCard`), so this is drift, not invention —
but rule 4 is that a citation is verified, and this one points a reader at the wrong row in the
one file the task is a delta on.

**How to fix:** re-point to `:367` and `:335` in the part, and correct the same two numbers in
the scope and the skeleton's phase-4 row while the sync base is being read (task 67 already
waits on that base under rule H2).

### S07-M3 (Minor): task 66 changes the scope's `aria-rowcount` number without the scope's test changing with it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-66.md:44-47

**Problem:** the scope's test 4 is *"120 issues virtualised render `aria-rowcount="120"` and row
61 carries `aria-rowindex="61"`"* (`tasks-61-80.md:233-235`). The part counts group heads as
rows, so 120 issues in 4 groups say `aria-rowcount="124"`, and an issue's `aria-rowindex` is its
place among heads-plus-issues — row 61 of the scope's count is not row 61 of the part's. The
part flags the difference in the same breath (*"the scope's example says 120: that is the issue
count, which the head rows add to"*) and the reasoning is right — an ARIA grid counts the rows
it renders, and task 40's `items` really does interleave `head` and `row` entries
(`task-40.md:531-536`) — but the part's test 4 (`:436`) is then written as *"say the full
count"* in prose, with no literal, so nothing pins which number is correct and a later
implementer reading the scope will write 120.

**How to fix:** keep the part's rule and put the arithmetic in the test: assert
`aria-rowcount="124"` for a 4-group / 120-issue fixture, and assert the `aria-rowindex` of a
named issue computed from `items.indexOf`, not a literal 61. Then correct the scope's example to
124 so the two agree.

### S07-M4 (Minor): task 70 marks task 64's LANDED payload UNVERIFIED on a claim that is no longer true

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-70.md:520-521

**Problem:** *"`LANDED`'s `word` (landed and proven) is task 64's payload (**UNVERIFIED**: task
64 has no part on the tree at this writing, `task-65.md:7`)"*. `task-64.md` is on the tree (651
lines) and its `LandedPayload.word` is a code, not that phrase — see S07-I2. The part inherited
the claim from task 65's preamble rather than checking it, which is how the sweep test came to
pin a string no surface produces. Same pattern, harmless on its own, in task 65's gap 3.

**How to fix:** when task 70 is next touched, re-read `task-64.md:127-136` and either drop the
UNVERIFIED mark or restate it against what task 64 actually declares. If S07-I2 is fixed by
exporting `LANDED_WORDS`, task 70's assertion becomes `LANDED_WORDS.landed_proven` rather than a
literal, and the drift cannot recur.

## Checked and clean

**Hub authorisation per verb.** Task 65's two added verbs are gated in the fake and pinned by
conformance rows: `AnswerReassign` by anyone but the current assignee is `403 not_assignee`
(`task-65.md:1266`), `RequestReassign` on an unbound state is `422 reassign_not_bound`
(`:1221`), by the assignee herself is refused, and a plain `PatchIssue` may not leave a request
pending (`:1290`). The part is explicit that the real route, its store column, migration and
**matrix rows** are a WH task it does not own and that the skeleton does not list (its gap 1) —
the right call, and the gap is reported rather than papered over.

**Agent edits as proposals / OD-16.** Held throughout: task 61 sends nothing on another
person's issue and says whose it is in place (`startRule` → `not_mine`, test at
`task-61.md:679`); task 63 never reads a teammate's sessions and says so; task 65's rule 3
(cross-owner relations as an Orchestrator proposal) is correctly deferred to W3; nothing reaches
a running agent until its owner presses *re-brief* (`task-65.md:22-30`), and the part marks
UNVERIFIED whether a live CLI re-reads the block.

**OD-8.** No part parses PTY bytes. Task 64 pins it with an acceptance grep
(`grep -n "attachRegistry" …RunningCard.tsx …agents.ts  # expected: no output`) and its MEANING
line is the person's own title, not a read of the terminal.

**The local-mode boundary.** Task 61 prints *"cost not metered in local mode"* rather than a
figure; task 69 runs the rehearsal in local mode and disables the verb because the starter repo
does not exist (searched and reported, its gap 2) rather than faking an agent; task 70's
decision 1 states *"a local run proves nothing about the cloud path"* in the results file
itself.

**OD-27, no predictions.** Every part carries an `expectNoPrediction` test, and task 70 adds a
sweep that fails when a W1 surface's own test file does not call it (`task-70.md:494-500`).
Task 69's decision 2 is the sharpest reading of the rule in the section: a CSV's row count is
known and is still not shown as a denominator.

**No Hub data on disk.** Task 62's visit marker, task 61's Start entries and task 69's Linear
key are all in memory, each with a spy test, and task 70's run includes a `git grep` for
`localStorage|indexedDB|writeTextFile` under the work surfaces.

**Tests red first, mutants named.** All ten parts list the mutant each test kills, and the
Steps sections instruct applying each mutant and reverting. The mutants are real defects, not
paraphrases — task 62's *"registering one kind twice: last wins, which would let TM-54 silently
replace an issue row"* and task 64's *"foldLine says all live whatever the idle count, which is
a lie on the screen"* are the two best in the section.

Section verdict: Needs fixes
# Phase W plan review — section 08 (tasks 71-80)

Reviewer pass over the ten parts task-71.md to task-80.md against docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-61-80.md, the skeleton docs/superpowers/plans/2026-09-25-phase-w-work-layer.md, and the design 05-design.md. Read-only except this file.

## Scope and skeleton coverage

All ten parts exist (task-71 507 lines to task-72 1377; none cut off). Each carries files, complete
fenced source, tests-first tables with a named mutant per row, steps, acceptance commands with
expected output, and a "does not claim" section. Citations spot-checked against the real trees and
held: `hubapi/types_test.go:213` (`jsonFields`) and `:361` (`decodeInto`), `codes.go:35`
(`CodeBriefExists`), `keys.go:172` (`ValidBranchRef`) on `feat/hub` cafe1303; `task-03.md:282`
(`IssueHead`), `:339` (`Label`), `:499-508` (`CreateLabelRequest`, `PutTeamPolicyRequest`),
`:665` (`TestWorkFieldNamesMatchTheExamples`); `task-02.md:157-172` (the twelve prefixes);
`task-06.md:156` (`CREATE TABLE issues`).

### S08-C1 (Critical): task 80's shared filter vectors do not decode into SW.18's `FilterGroup`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-80.md:98
**Problem:** The vector's filter object is `{"op":"or","conditions":[{"field":"state","cmp":"eq","value":"todo"}]}`.
Task 72 freezes `FilterGroup{Op, Terms []FilterTerm `json:"terms"`, Groups []FilterGroup `json:"groups"`}`
and `FilterTerm{Field, Op `json:"op"`, Values []string `json:"values"`}` (task-72.md:634-645), with
term ops `is`, `is_not`, `before`, `after`, `empty`, `not_empty` (task-72.md:224). `conditions`,
`cmp` and `value` exist nowhere in SW.18. These twelve files are the cross-language contract that
tasks 83 and 91 also run, so both evaluators decode an empty `Terms` and every vector silently
matches everything (or the strict decode of task-72.md:1129 rejects the file outright). The vector
named `not-of-one` has no wire form at all: `FilterOps` is `and`/`or` only, and there is no
`not` term op. The same drift is in the route text, which names the error path `conditions[2].field`
where `FilterGroup.validate` builds `terms[2].field` (task-72.md:326-339).
**How to fix:** Rewrite the vector shape in task 80 to `{"op","terms":[{"field","op","values"}],"groups":[]}`,
map `eq`→`is`, replace `not-of-one` with `is_not` (or ask task 72's owner for a `not` op as an SW.18
addition, which the freeze allows), and change the `details.field` example to `terms[2].field`.

### S08-I1 (Important): task 80's request bodies contradict SW.18's frozen request types, so its matrix rows cannot pass
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-80.md:145
**Problem:** Four bodies do not match task 72, which task 72 enforces with
`TestWorkPlanRequestsRejectUnknownFields` (`DisallowUnknownFields`, task-72.md:1129).
(1) `PUT /v1/views/{viewId}/team-default` sends `{"isDefault": true}`; the type is
`TeamDefaultRequest{IsTeamDefault bool `json:"isTeamDefault"`}` (task-72.md:847-851) — an unknown
field, so 400, while the matrix row `wadmin(... 200 ...)` expects 200.
(2) `POST /v1/me/drafts` is documented as `{projectId, body}` and the matrix row sends
`{"projectId":…,"body":h("{}")}`; `CreateDraftRequest` requires `kind` and `title` and its `Body`
is `AuthoredText` (task-72.md:853-861), which `{}` does not satisfy — so 422, not the 201 the row
expects.
(3) `PUT /v1/me/drafts/{draftId}` is documented as replacing `body` only and its store signature
takes `body json.RawMessage`; `PutDraftRequest` is `{Title, Body AuthoredText}` (task-72.md:864-867).
(4) `POST /v1/teams/{teamId}/views` is written as `CreateViewRequest{name, filter, grouping, sort}`
and the matrix body sends only name and filter; the real type needs `layout`, `groupBy`,
`subGroupBy`, `orderBy` and `display` (task-72.md:825-834), and `ValidViewLayout("")` is false.
**How to fix:** In task 80, quote the four request types from task 72 verbatim in the route list and
in the `workMatrixRows["80"]` bodies; change `Where`'s draft store signature to take
`hubapi.AuthoredText` and a title.

### S08-I2 (Important): a draft expires in 30 days in task 80 and in six months everywhere else
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-80.md:130
**Problem:** Task 80 route 6 sets `expires_at = now() + 30 days`. Task 72 says `PutDraftRequest`
"moves ExpiresAt six months on" and `Draft`'s comment says "the janitor deletes it at ExpiresAt
(six months after UpdatedAt, task 77)" (task-72.md:646-648, :863). The scope for task 77 says
"expire drafts after six months" (tasks-61-80.md:585). Task 77's `ExpireDrafts` deletes on
`expires_at < now()` and asks no questions, so the shorter value wins silently and a person's
unsaved issue body is deleted five months early. No test in either part pins the interval.
**How to fix:** Make six months the one value, put it in `hubapi` as a named constant
(`DraftLifetime`) that task 80's create and put both use, and add a test in task 80 asserting
`expiresAt - updatedAt` is that constant.

### S08-I3 (Important): tasks 75-79 write `issue_heads`, a table that does not exist; task 77's literal DDL would fail
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-77.md:139
**Problem:** Task 74 proved the head table is `issues` (`task-06.md:156`, `CREATE TABLE issues`;
`grep -c issue_heads task-06.md` is 0, re-run and confirmed here) and recorded the wrong name as a
gap against task 79 only. Four more parts in this section still use it: task-75.md:211, :215, :227,
:334, :356; task-76.md:248, :386, :404, :414; task-77.md:84, :105, :139-141, :154; task-79.md's
`TestTheSyncActorSetsNoPlanField`. Task 77's is not prose but SQL appended to task 74's migration
file: `ALTER TABLE hub.issue_heads ADD COLUMN archived_at timestamptz;`, `CREATE INDEX
issue_heads_unarchived ON hub.issue_heads (project_id, state) WHERE archived_at IS NULL;` and
`GRANT UPDATE (archived_at) ON hub.issue_heads TO hub_app;`. That migration aborts with `42P01
relation "hub.issue_heads" does not exist`, and because tasks 75-80 all depend on task 74's file,
every W2 task is blocked behind it.
**How to fix:** Replace `issue_heads` with `issues` in tasks 75, 76, 77, 78 and 79 (prose, SQL, test
bodies, Files tables), and add one line to each part's step 1 saying the head table is `issues`, so
the correction is not re-derived five times.

### S08-M1 (Minor): `lb-` is the id prefix of both `Label` and `LabelGroup`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-72.md:602
**Problem:** `Label.ID` is `lb-...` (task-03.md:339-341) and task 72 gives `LabelGroup.ID` the same
prefix, with `Label.GroupID` also `lb-...` (task-72.md:434). No prefix check can then tell a label
id from a group id: `PATCH` an issue's labels with a group id, or a `groupId` pointing at a label,
both pass a `PrefixLabel` check. Task 74's `labels (org_id, group_id) REFERENCES label_groups
(org_id, id)` catches the second case at write, which is why this is Minor; the first relies on a
store lookup no part names. Task 72 raised the missing `ClosureProposal` prefix as a gap but not
this one.
**How to fix:** Add `PrefixLabelGroup = "lg"` to task 02's list (SW.18's freeze allows an added
prefix, and task 72 already proposes `cp-` on those grounds), or state in task 72 that `LabelGroup`
ids are namespaced by route only, and name the store lookup that enforces the kind.

### S08-M2 (Minor): the AU fail-safe makes every W2 feature slug refuse its own name
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-76.md:52
**Problem:** Decision (d) reads a project whose data class the Hub cannot read as AU-only (task 24's
fail-safe), and says "until the class has a home (Phase T task 43, UNVERIFIED) every project takes
the AU rule". In W2 the class has no home, so every feature slug repeating a word of its own name is
422 `slug_invalid` and the only accepted slugs look like `feature-12`. The part's own test
`TestAUProjectsKeepTitlesOutOfSlugs` pins that as W2's state. The 05 §5.4 demo and
`prototype/work-layer.html` both name branches after the work, so the W2 walkthrough hits the
refusal.
**How to fix:** Either say in task 76's "does not claim" that W2 branches are numbered, not named,
and carry that into task 86's copy and the W2 demo script; or give the fail-safe a per-project
override a team admin sets until Phase T lands.

### S08-M3 (Minor): `Where` has two signatures inside task 80
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-80.md:33
**Problem:** Decision (a) declares `Where(g hubapi.FilterGroup) (sql string, args []any, err error)`;
the store section declares `Where(g hubapi.FilterGroup, next int) (clause string, args []any, err
error)` (task-80.md:227). An implementer reading top-down writes the first and the call site does
not compile. The `next` argument matters: without it a filter clause composed after other
placeholders renumbers `$1` and binds the wrong value.
**How to fix:** Delete the two-argument form from decision (a), keep the three-argument one, and say
in one sentence that `next` is the first free placeholder index.

### S08-M4 (Minor): `children_closed` bends skeleton rule 2 and ships unflagged while the amendment is unapproved
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-78.md:57
**Problem:** Decision (e) states plainly that a container parent reaching `done` with cause
`children_closed` bends the skeleton's rule 2 (a fact `done` needs the issue's own evidence at its
own land commit), and that this is "reported to the orchestrator as a rule-2 amendment for the
owner, not assumed". The route, the cause, the database guard and the event are nonetheless built
unconditionally. Task 80 handles the same shape of unapproved change (the `/p/` page against the
contract's *No browsers*) behind `PANACEA_APP_HUB_PAGES=off` with a written proposal; task 78 has no
equivalent, so an owner who refuses the amendment must unpick a fact cause that issues already
carry.
**How to fix:** Either write the amendment as its own proposal document the way task 80 does and
gate the roll-up behind a team policy flag defaulted off, or get the ruling into the owner-decisions
file before task 78 is executed and cite it in the part.

### S08-M5 (Minor): `GET /v1/me/drafts` (the list) has no matrix row
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-80.md:145
**Problem:** Route 8 is `GET /v1/me/drafts[/{draftId}]`, which is two routes. `workMatrixRows["80"]`
covers the single-draft `GET`, `PUT` and `DELETE` and the `POST`, but not the list. Task 07's
`TestLiveWorkRowsMatchTheRouteTable` would surface it as a failure rather than a leak, but the list
is exactly the route where a scoping bug returns another person's drafts, and it is the one with no
principal row.
**How to fix:** Add a `wmine`-shaped row for `GET /v1/me/drafts` asserting each principal sees only
its own rows (a list has no 404, so the assertion is on the body, not the status).

### S08-M6 (Minor): three parts are sized at three hours and are visibly larger
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-72.md:3
**Problem:** Task 77 says it is "the largest of the W2 route tasks, and the first to split" and task
80 revises itself to 4-5 hours, so the sizing discipline exists in this section. Task 72 does not
use it: 1377 lines defining every W2 record, eight validators, 46 interface methods and three test
blocks, where task 03's comparable W1 file was a task of its own. Task 79 (661 lines) carries rank
arithmetic with three key functions and a rebalance, the move with its alias table, the
resolved-blocker rule, 19 tests and deltas into tasks 08, 10, 11 and 13, and still says three hours.
**How to fix:** Re-size 72 and 79 the way 80 re-sized itself, and name the split line if they run
over (72: types and validators, then the `WorkPlan` interface; 79: plan fields and rank, then move
and the blocker rule).

## What I checked and found sound

- **Tests red first, with a named mutant per row.** All ten parts have a test table where every row
  names the mutant it kills, and a step that runs them before the code. The acceptance greps agree
  with the tables: 72 -> 13, 74 -> 15, 75 -> 11, 77 -> 1 unit + 14 integration, 78 -> 1 + 13,
  79 -> 3 + 16, 80 -> 12 vectors + 13. Task 71 captures a golden of phase 7's wire on the unchanged
  base *before* its refactor, which is the right order.
- **Security per verb.** Every route task carries `workMatrixRows` over the contract's six
  principals with the sync principal refused, plus a database half (`TestTheSyncActorSetsNoSubState`,
  `...PromotesNothing`, `...SetsNoCycle`, `...SetsNoParent`, `...SetsNoPlanField`). Task 74 re-enforces
  every Go rule as a `CHECK`, a trigger or a column grant, so a missing handler check still fails.
- **No-leak rules.** Drafts answer 404 to everyone but the author at the route, in the store and in a
  `RESTRICTIVE` row-level-security policy (74 (e), 80 (d)); closure proposals take the same shape
  (77 correction 5); the read-only page is 404, never 403, for a reader outside the project; the
  iCal feed carries cycle numbers, names and dates only, refuses `Origin`, and is revocable.
- **The provenance thread holds.** Shipped is derived and never stored, with
  `TestShippedWritesNothingToTheHub` and a `git grep` acceptance line proving no `shipped` field
  exists on a Hub record or a replica op (71); promote copies nothing and leaves the issue's revision
  byte-equal (76); the janitor writes `actor: null` with the policy author named, and a person's act
  is written as the person (75 (e), 77, 78 (c), 79 (h)).
- **Two surfaces outside `/v1` are proposals, not assumptions.** The iCal feed (77 (i)) ships with a
  bearer-only fallback built beside it; the read-only page (80 (i)) ships behind a flag defaulted off
  with a proposal document written before the route.
- **Citations are real.** Every line I could reach reads as cited.
  `docs/superpowers/contracts/2026-09-23-hub-contract.md` lines 36-39 (bearer, *No browsers*),
  131-137 (ref format), 253 (`brief_exists`), 331 (org admin reads no project content), 641-644
  (`BriefHead` has no title), 760 (one live branch brief per `(project, branchRef)`) and 2229
  (§12.5 the janitor) all match. On `feat/hub` cafe1303, `hubapi/types_test.go:213` (`jsonFields`),
  `:361` (`decodeInto`), `codes.go:35` (`CodeBriefExists`) and `keys.go:172` (`ValidBranchRef`)
  match. In this branch's parts, `task-02.md:157-172` (the twelve prefixes), `task-03.md:282`
  (`IssueHead`), `:339` (`Label`), `:499-508` (`CreateLabelRequest`, `PutTeamPolicyRequest`), `:665`
  (`TestWorkFieldNamesMatchTheExamples`) and `task-06.md:156` (`CREATE TABLE issues`) match. The
  parts cite the contract by the short name `hub-contract.md`; the real path is
  `docs/superpowers/contracts/2026-09-23-hub-contract.md`, and it reaches the `work` worktree only
  through the sync-merge each part's step 1 already performs.
- **No part is missing or cut off.** task-71 507 lines, 72 1377, 73 359, 74 593, 75 419, 76 473,
  77 614, 78 514, 79 661, 80 357; each ends with its acceptance block, a "does not claim" section and
  its parity rows.

Section verdict: Needs fixes
# Phase W plan review — section 09 (tasks 81-90)

Reviewer: adversarial plan review, read-only. Parts read one at a time in line ranges against
their scope in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-81-100.md` and
the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:488-498`.

Parts present: task-81 (375 lines), 82 (371), 83 (373), 84 (254), 85 (246), 86 (208), 87 (351),
88 (293), 89 (301), 90 (251). None is missing.

## Findings

### S09-C1 (Critical): task 81 redeclares `hubapi.Digest`, which already exists as `[32]byte`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:155
**Problem:** the part writes `type Digest struct { UserID string; Since, Until time.Time; Items []DigestItem }` into `hubapi`. `hubapi` already owns that identifier: verified on the tree at `.claude/worktrees/hub/control-plane/internal/hub/hubapi/keys.go:110`, `type Digest [32]byte` — the content hash the whole revision chain is built on (task-37.md:104 mirrors it as `export type Digest = string`; task-13.md:145, :155 and :172 carry `Digest` fields of that kind). Two declarations of one name in one package do not compile, so task 81 as written cannot build, and its Step 1 instruction that "task 72 must add them to `hubapi`" propagates the clash into task 72. The contract is affected too: `GET /v1/work/digest` returns `Digest`, which a reader of the route list will now read as a hash.
**How to fix:** rename the mail aggregate to `hubapi.MailDigest` (or `WorkDigest`), keeping `DigestItem`, `DigestCause`, `DigestEmail` and `DigestSettings` as they are, and change `BuildDigest`'s return type and the route's 200 body with it. Check task-83.md:208 and task 94 for the same name before either is written.

### S09-C2 (Critical): task 83's `Feature` contradicts task 72's `hubapi.Feature` on six fields

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-83.md:109-123
**Problem:** the part's whole promise is that the desktop types mirror "task 72's JSON tags verbatim" (:97), and its golden round-trip test (decision (b)) exists to catch exactly a renamed field. The `Feature` interface does not mirror task-72.md:507-522: `briefKey` vs `briefId`; `status: FeatureStatus` vs `state: FeatureState`; `leadId: string | null` vs `lead: User`; `labelIds` vs `labels`; `memberIds` is invented (task 72 puts no member list on a feature); and `initiativeId` and `createdBy`, both in the Go struct, are missing. The enum values agree (task-72.md:114-120) but the type name does not. Every `Feature` golden fixture therefore fails on the first run, and tasks 85 and 86 — which the scope points straight at this type — are written against names the Hub never sends.
**How to fix:** regenerate the `Feature` block field for field from task-72.md:507-522 (`id, projectId, name, branchRef, briefId, lead, targetDate, state, priority, labels, initiativeId, createdAt, createdBy, updatedAt`), rename the TS enum to `FeatureState`, and re-read `Cycle`, `Cadence`, `View`, `Template` and `Draft` against their task 72 blocks in the same pass — one type diverging this far suggests the block was written from the scope's prose rather than from task 72.

### S09-I1 (Important): task 81 marks the digest sent before it sends, so a failed send loses that window for good

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:252-256
**Problem:** `sendOneDigest` "marks first and sends second, inside one transaction for the mark and outside it for the send", and a send error only increments `rep.DigestsFailed` ("logged, never fatal"). The window is not derived from `last_sent_at`: the janitor computes `due = now.Add(-time.Hour)` (`:225-229`) and passes it as `since`. So a Graph 5xx, a timeout, or any janitor restart between the mark and the send writes `last_sent_at` and drops the hour's items permanently — the next sweep builds `now-1h` again and never revisits them. The same hole opens with no failure at all: a recipient whose sweep was skipped for three hours gets one hour of items and silently loses two. That defeats L-37 (a due date passing in silence is the exact failure the part's own `TestADueIssueWithNoEventsStillAppears` guards against) and no test covers it.
**How to fix:** derive `since` from the recipient's `last_sent_at` (falling back to `now - mode window` on first send) rather than a fixed `now-1h`, and either mark after a successful send or roll `last_sent_at` back on error. Add a test: a mailer that returns an error on the first sweep and succeeds on the second must deliver the first window's items in the second message (mutant: the items are dropped).

### S09-I2 (Important): task 81's `mailer.ErrRateLimited` can never be returned, so the 15-minute rule has no enforcement point

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:100-104
**Problem:** `mailer.go` declares `ErrRateLimited` as what a `Mailer` returns "when a message for this recipient was sent inside the window", and the janitor switches on it (`:232-234`, `rep.DigestsRateLimited++`). No implementation can honour that contract: `graph.go` holds no state by design (decision (f): "idempotency is a key, not a memory") and `fake.go` only records. The actual gate is `store.MarkDigestSent` returning `false` (`:219-220`), which `sendOneDigest` turns into `sent == false, err == nil` — the `default` arm, counted as `DigestsEmpty`. So `DigestsRateLimited` is dead, a refused send is miscounted as "nothing to say", and the 15-minute rule of `docs/AGENTIC-UX.md:243` is enforced nowhere distinguishable from the hourly cadence (`DigestRecipients(mode, now-1h)` already excludes anyone sent inside the hour). `TestASecondSendInsideTheWindowIsRefused`'s "two sweeps 5 minutes apart -> one message" passes on the cadence filter alone, so it does not kill its stated mutant.
**How to fix:** move `ErrRateLimited` out of `mailer` and into the store/janitor boundary (`MarkDigestSent` returning `(false, ErrRateLimited)` when the refusal is the 15-minute window, and a distinct "another replica won" signal), or delete the error and the counter. Then make the test bite: a recipient with `last_sent_at` 5 minutes ago and `hourly` mode is refused with the window reason, distinct from the loser-replica case, which `TestTwoReplicasSendOnce` already covers.

### S09-I3 (Important): `GET /v1/work/digest` is in no `WorkPlan` method, no `WorkPlanRoutes` row and no relay row

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-82.md:113-175
**Problem:** task 81 adds the route on the Hub and a `workMatrixRows["81"]` row for it, but nothing reaches it from outside the Hub. `WorkPlanRoutes` (task 82, task 72's order) has no digest row — verified by grep over task-82.md, whose only `digest` hits are the revision-hash re-check and the acceptance `-run` pattern — and task 72 carries no `Digest*` identifier at all (verified: `grep -c 'DigestSettings\|Deferral\|DigestItem' task-72.md` is 0). Two consequences. Task 82's own end-to-end step 9 ("the digest builder names steps 3, 4 and 6", `:305`) has no client call to make it with. And task 94, the desktop half that L-116 is closed jointly with, would have to reach a Hub path the control plane does not relay, which §9.4's caller-key rule makes impossible from the desktop.
**How to fix:** fold the gap report in task 81's "A gap found in task 72" into the route surface as well: task 72 gains `Digest(ctx, since)`, `DigestSettings(ctx)` and `PutDigestSettings(ctx, ...)` on `WorkPlan` with their route rows; task 82's `WorkPlanRoutes` gains them (making it 49, not 46, in every count in that part); and `routes_audit_test.go` gains the rows. Otherwise state in task 81 that the digest read is Hub-local only and name which task relays it.

### S09-I4 (Important): the skeleton makes task 81 depend on 13 and 74, but the part cannot start without task 72

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:488
**Problem:** the row reads `| 81 | ... | 13, 74 | — |`. The part's Step 1 is "close the task 72 and 74 gaps first" and says plainly: "if they have not landed, this task waits rather than writing them in the wrong file" (task-81.md:288-292). `DigestSettings`, `Deferral`, `DigestItem` and the digest aggregate all live in task 72's `hubapi/work_plan.go`, and `Subscriptions`, `BuildDigest` and the route all take them. A scheduler reading the skeleton will start 81 the moment 13 and 74 are done and find it blocked. Task 82's row (`:487`, "73, 75-81") inherits the same slip.
**How to fix:** change the row to `13, 72, 74`, and add the three types plus their four methods to task 72's own part and the two tables to task 74's, so the gap is closed where it lives rather than described in the consumer.

### S09-M1 (Minor): task 81's matrix row pins a `since` that expires four days after the plan was written

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:262-266
**Problem:** the row sends `path: "/v1/work/digest?since=2026-09-01T00:00:00Z"` and expects `{200, 200, 200, 200, 200, 403}`. The route answers `422 since_too_old` for a `since` more than 30 days back (`:196-198`). From 2026-10-01 that literal is 30 days old and every 200 in the row becomes a 422, so the matrix test fails on the calendar rather than on a change to the code.
**How to fix:** build the query from `time.Now().Add(-24 * time.Hour).Format(time.RFC3339)` in the row's construction, and keep the fixed date only inside `TestSinceTooOldIsRefused`, where an absolute boundary is the point and is computed from the test clock.

### S09-M2 (Minor): task 81's decision (e) says the mark happens in the send's transaction; the code and the seam say the opposite

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:48-52
**Problem:** decision (e) reads "`digest_settings.last_sent_at` is checked and written in the same transaction as the send". `mailer.go`'s own doc comment says `Send` "is never called while a transaction is open" (`:112-113`), and the janitor section says the mark is "inside one transaction ... and outside it for the send" (`:252-253`). Three readings of one rule in one part; the one a reader meets first is the wrong one, and it is the reading that would hold a Postgres transaction open across an HTTPS call to Graph.
**How to fix:** rewrite decision (e) to match `:252-253` — mark inside a transaction, send outside it — and fold in the ordering that S09-I1 settles, so the decision states one sequence.

### S09-I5 (Important): task 86's promote preview invents `dryRun` on a 201-Created route and reuses one idempotency key for both calls

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-86.md:57-64
**Problem:** decision (b) calls `PromoteToFeature` "with `dryRun: true`", then "the confirming press sends the same request with `dryRun: false` and the same idempotency key". Two defects. First, `dryRun` exists nowhere: `grep -n 'dryRun\|DryRun'` over task-72.md, task-76.md and task-83.md returns nothing, and the route is declared `POST /v1/issues/{issueId}/promote member 201 PromoteResult` (task-76.md:93) with `Created: true` in task 82's route table — a create that answers 201 cannot also be a preview. The part's gap 2 admits `dryRun` is missing but decisions (b) and the steps are still written as if it exists. Second, and worse if task 76 does add it: an idempotent create keyed on `IdempotencyKey` will either replay the dry run's stored response for the real press (so no feature is ever created, and the board shows one because the client painted it optimistically) or answer `409 idempotency_conflict` on the changed body (task-82.md:221-223). Either way the confirming press does not promote.
**How to fix:** decide the preview shape with task 76 before either part is built — a separate `GET /v1/issues/{issueId}/promote-preview` (no idempotency, no 201) is the clean answer — and if `dryRun` is kept, state that the two calls carry **different** idempotency keys and that only the `dryRun: false` key is retried. Add a test: the preview call and the confirm call are asserted to send different keys (mutant: one key, and the confirm replays the preview).

### S09-I6 (Important): the keymap rows in tasks 86 and 89 are two invented shapes, neither of which is task 53's `KeyBinding`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-86.md:117-127
**Problem:** task 53 owns `desktop/src/work/keys/keymap.ts` and prints its row shape in full (task-53.md:159-185): rows are built by `r(verb, keys, scope, slice, label, linear, shift?, code?)` and frozen, with `scope` drawn from `"list" | "issueRow" | "deskDecision" | "everyRow"`. Task 86 writes `{ key: "b", when: "keys.list", run: "work.setFeature" }`; task 89 writes `{ key: "D", scope: "list", target: "issue", verb: "plan.due", label: "Due date…" }` (task-89.md:214-216). Neither has `slice` or `linear`, both of which `r` requires; `when`, `run` and `target` are not fields of `KeyBinding` at all. Task 89's stated rationale — "AX §5's `A`/`D` on a decision row keep their meaning because their rows carry `target: "decision"`" (`:218-220`) — describes a mechanism that does not exist; task 53 already separates them by `scope` (`r("decision.defer", ["D"], "deskDecision", ...)` versus `r("issue.assign", ["A"], "issueRow", ...)`). Compounding it, task 86's anchors assert "**task 59 has no part file on the tree**" (`:26-30`) and attribute `keymap.ts` to task 59; `task-59.md` does exist and says of that file "nothing added; read only" (task-59.md:77). So the part's one UNVERIFIED flag points at the wrong task while the real interface, printed in task 53, was available to quote.
**How to fix:** rewrite both deltas as `r(...)` calls against task 53's signature — `r("issue.setFeature", ["B"], "issueRow", "W2", "Feature", "match")`, `r("plan.due", ["D"], "issueRow", "W2", "Due date", "match")`, `r("plan.estimate", ["E"], "issueRow", "W2", "Estimate", "match")` — correct task 86's anchor to task 53, and delete the `target` rationale in favour of `scope: "issueRow"` versus `scope: "deskDecision"`. Task 87's `Y` row (task-87.md, decision 7, "one `list` scope row") needs the same treatment and should be `issueRow`, since it acts on a focused issue.

### S09-I7 (Important): task 87 imports `ScopeEntry` from the wire, but task 83 never defines it and the scope never lists it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-87.md:96
**Problem:** `desktop/src/work/cycles.ts` opens with `import type { Cycle, IssueHead, ScopeEntry } from "../api/work";`, and `ScopeLog.tsx` renders one line per entry. Task 83's type list — the file that exists to fix SW.18's desktop wire once — carries `CustomState`, `Feature`, `Cycle`, `Cadence`, `View`, `Template`, `Draft`, `LabelGroup`, `EstimateScale`, `DigestItem` and `Deferral`, and no `ScopeEntry` (verified by grep over task-83.md). The scope's own list (tasks-81-100.md:92-94) omits it too. So task 87's central surface, the scope log, has no wire type, and task 83's rule that "nothing else in `src/` names a W2 field" is broken the moment 87 declares one locally.
**How to fix:** add `ScopeEntry` (and `ScopeChange`, which task-72.md:134 declares on the Go side) to task 83's part and to the scope's type list, mirroring task 77's `ScopeEntry` field for field including the actor and the policy author that decision 5 renders. Until then, task 87 must carry the type as an explicit UNVERIFIED stub naming task 83 as its owner, not as a plain import that reads as settled.

### S09-I8 (Important): task 86's central test asserts `childIds`, a field on no `Feature` type

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-86.md:137
**Problem:** `servesIsUnchangedAndIssueIsFirstChild` asserts `features[0].childIds[0] === issue.id`. Neither task 72's `hubapi.Feature` (task-72.md:507-522) nor task 83's TS `Feature` (task-83.md:109-123) has a `childIds` field, and the parent/child link in this phase is `parentId` **on the issue**, which is task 78's route and the field task 86's own decision (c) says the op touches ("`plan.feature` and `parentId` on the issue"). The test as printed does not compile, and the property it means to assert — "the issue becomes the feature's first child", which is the second half of L-23 — is therefore unproven.
**How to fix:** assert the fact where it lives: the promoted issue's `parentId` (or `plan.feature`) equals the new `ft-` id, and the feature's children read through task 78's `children` call returns that issue first. Name which of the two carries "first", since L-23's ordering claim needs a rank or a creation order that no part yet specifies — a gap for task 76.

### S09-M3 (Minor): tasks 85 and 86 state counts that disagree with their own lists

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-85.md:97
**Problem:** task 85's file map row reads "the six built-in tabs plus pinned views" and two tests say "every one of the six tabs" and "all six tabs", while `BUILT_IN_TABS` holds five (`brief`, `issues`, `criteria`, `worktrees`, `release`, task-85.md:115-119) — which is also what the scope lists. Task 86's file map says "the six tests below" over a table of seven, and its acceptance expects "7 passed". A reader auditing coverage by the prose gets a different answer from the code.
**How to fix:** make the prose read from the list in both parts — "the five built-in tabs" and "the seven tests below" — or, in task 85, say explicitly that the sixth is the pinned-view slot so the count is defensible.

### S09-M4 (Minor): tasks 87, 88 and 90 each assert that `task-83.md` does not exist

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-87.md:17-19
**Problem:** all three say some form of "Task 83's types are the scope's ... **UNVERIFIED** (no `task-83.md` at this writing)" (task-88.md:15-16, task-90.md:14-15). `task-83.md` is on the tree, 373 lines, and prints every type these parts name. The flag was true when written and is false now, and it matters: the three parts took their type names from the scope's prose, which is exactly how task 83's `Feature` diverged (S09-C2), and an implementer trusting the flag will not go and check.
**How to fix:** re-read task-83.md's type blocks and replace the flag with line citations in all three parts, resolving `ScopeEntry` (S09-I7) and `Label`/`LabelGroup` in the same pass. The pattern is worth a rule for the phase: a part may not declare a sibling part missing without a check at write time.

### S09-I9 (Important): every digest link points at a surface that is off by default, and nothing checks

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-81.md:187-188
**Problem:** `BuildDigest` rule 6 makes every item's `Link` `subs.LinkBase + "/p/issues/" + issueID`, and decision (d) makes the link the *only* way a reader gets the content ("a reader who wants the text follows the link"). Task 80 ships that page behind its own switch: `TestThePageIsOffByDefault` asserts "`PANACEA_APP_HUB_PAGES` unset → `GET /p/issues/{id}` is 404 and the mux is not mounted" (task-80.md:272). Task 81 adds five `PANACEA_APP_HUB_MAIL_*` keys including `_BASE_URL`, and neither the config, the janitor guard (`if s.mail == nil`) nor any test relates the two. A tenant that enables `MAIL_MODE=graph` without `PANACEA_APP_HUB_PAGES` sends mail whose every link 404s, and the failure is invisible to the sender.
**How to fix:** refuse the combination at startup — `MAIL_MODE != off` with pages unmounted is a config error at `New`, named in the message — or make the digest render the alias with no link when pages are off. Add a test for whichever is chosen (mutant: a digest built with links while the page mux is unmounted).

### S09-I10 (Important): task 89 moves an issue between projects, clearing five things and renumbering it, with no Undo and no statement that it is reversible

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-89.md:180-199
**Problem:** `MoveDialog`'s button calls `r.move(...)`, closes and navigates on success. `clearsOnMove` enumerates what the act destroys — team labels, cycle, sub-state, feature, and the assignee when they are not a member of the target team (decision 7, from task 79 (f)) — and the issue gets a new number. Every other act in tasks 84-90 states its reversibility and tests it: task 86 (e) undoes a promote, task 89's own test 6 undoes a drag, task 88 (7) undoes a whole split. The move states nothing. It is not in `optimistic.undo`'s set (no op through `mutate`; it awaits a promise) and it is not `confirm.inplace` either, so it falls between 02's two optimism classes (`02-linear-design-language.md:278`) with no rule covering it. The ten tests include none for undoing or re-moving.
**How to fix:** decide the class and say it. If a move is reversible, route it through `mutate` so Undo inside `--undo-window` restores the project, number and the five cleared fields, and add the test. If it is not, make it `confirm.inplace` with the clears list as the consequence line and say in *does not claim* that a move cannot be undone, so the person reads it before pressing. Either way, name what happens to the cleared labels on a re-move — task 79 (f) may not restore them.

### S09-M5 (Minor): task 82's decision (f) calls the issue page an unauthenticated read; it is not

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-82.md:76-80
**Problem:** the reason given for not relaying `GET /p/issues/{issueId}` is that "relaying them would put the control plane in the path of an unauthenticated read". Task 80 gives that page a `HttpOnly; Secure; SameSite=Strict` session cookie from the Entra auth-code exchange at `GET /p/auth/callback`, and "a reader with no project read gets `404`, never `403`" (task-80.md:172-177). The decision reaches the right answer — a server-rendered HTML surface with its own cookie session is not a JSON route to relay — from a false premise, and `routes_audit_test.go` is to record the false premise as the standing note a later reader will act on.
**How to fix:** restate the reason as what it is: the page and the calendar feed are browser surfaces on the Hub's origin with their own session and token auth, so a relay would have to forge a cookie it does not hold. Same for `/cal/{token}.ics`, whose auth task 77 owns.

## Coverage notes (not findings)

- All ten parts exist and none is cut off: each ends with a *does not claim* section and, where it found one, a gaps section.
- Scope coverage is otherwise complete. Task 85 records the *Lanes* vs *Worktrees* correction the scope asked for; task 87 builds L-62's SKIP substitute (the last three cycles' closed counts, no average) as the scope specified; task 88 keeps `tk-` out of the sub-issue list (L-20) with a pure, testable `isPersonIssue`; task 90 honours 02 C1 (colour never the only signal) with the mono `org` marker. `expectNoPrediction` appears in 84, 85, 87 and is argued for, not merely invoked — task 85's refusal to render `issuesLanded` beside `issuesTotal` is the sharpest example.
- Sizing is credible for all ten; task 82 at 3 hours for 46 client methods, a relay delta and a nine-step end-to-end is the one that looks light, but the methods are four lines each from a table, so it is defensible.
- Security: Hub authorisation per verb is checked at the route, not the client, in every part; task 84 (e) and task 90 (7) both refuse to duplicate a Hub rule on the desktop, and task 90 renders no control rather than a disabled one. Task 81 raises the right unresolved question (may an issue title leave the tenant in mail) instead of deciding it. No part reads or parses an agent CLI, and no part copies Linear or Plane material.
- S09-I7 is partly self-reported: task 87's gap 2 asks task 83 for `ScopeEntry`. The finding stands because task 83 is written and does not carry it, so the ask has nowhere to land unless 83 is reopened.

Section verdict: Needs fixes
# Section 10 — adversarial review of Phase W tasks 91 to 100

Reviewer pass over `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-91.md`
through `task-100.md`, each read against its scope in
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-81-100.md`
and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

Tasks in range: 91 views and the view editor; 92 drafts, callouts, copy-as-Markdown;
93 https links and the prefill deep link; 94 digest, defer-with-reason, CSV export;
95 themes part 1 (generated tokens); 96 themes part 2 (high contrast, preferences);
97 W2's proof and results; 98 Phase T delta (`task.issueId`, Orchestrator messages);
99 the W3 Hub wire, fake and conformance; 100 the W3 migration, routes and matrix rows.

All ten parts exist and none is cut off: each ends with its closing section.
Line counts: 91/292, 92/394, 93/352, 94/302, 95/395, 96/305, 97/318, 98/318, 99/440, 100/958.

## Findings

### S10-I1 (Important): no task owns the *subscribe to this view* control, so L-96 has no writer

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-91.md:292
**Problem:** the scope for 91 names *"subscribe to a view, delivered by the digest (L-96, tasks 81
and 94)"* (`tasks-81-100.md:252`) among what this task builds. `task-91.md` never mentions L-96,
subscription or a subscribe control — grep over the part returns nothing. The other two ends
disclaim it: `task-81.md:351` says *"Not per-view subscriptions as a route. `DigestSettings.ViewIDs`
is read here and written by task 72's method; this task adds no write route of its own"*, and
`task-94.md:273` says *"Not a subscription editor. What a person is subscribed to is task 81's
`Subscriptions`"*. So `DigestSettings.ViewIDs` (`task-81.md:131`) is read by the digest builder and
written by nobody a person can reach. L-96 is a MATCH row in the parity matrix and is in task 97's
W2 list (`task-97.md:47`), so W2's proof will have a row nothing can demonstrate.
**How to fix:** give task 91's `ViewList`/view header a *Subscribe* toggle that calls task 72's
write method through task 83's relay, add a test (mutant: the toggle writes a personal preference
instead of `DigestSettings.ViewIDs`, so the digest never changes), and name L-96 in 91's closes
list. If the control is meant to live in task 94's digest settings pane instead, say so in both
parts and delete 94's disclaimer.

### S10-I2 (Important): task 97's rows table maps parity rows to the wrong groups, one group out

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-97.md:81
**Problem:** the *rows W2 closes* table is offset. *Views, saving, favourites* is given
`L-30, L-33, L-35, L-37, L-44` (deep link, drafts, autosave, due-soon) closed by 91-93;
*Defer with a reason* is given `L-93 to L-96` (which are the view rows) closed by 89, 94;
*The digest* is given `L-98, L-99, L-102, L-103, L-105, L-107` (views, grouping, ordering,
set-as-default, search prefixes); *Notifications and callouts* is given `L-109 to L-111` (rail
favourites and ad hoc lists); *Themes and preferences* is given `L-153` (CSV export). The parts
themselves are consistent — 91 owns L-93 to L-111, 94 owns L-115 and L-153, 95-96 own L-234/L-235
— so the table, not the work, is wrong. Its *proof shape* column inherits the error: the reason a
deferral is stored (frame 8) is promised as the proof for the saved-view rows.
**How to fix:** re-cut the table from `coverage.md:45-277`, or drop the group column and list
`row → task → proof` one row per line. Decision 3 already says Step 1 re-derives the list from
`check-slice.sh W2`; make the table the output of that step rather than a hand-written map.

### S10-M1 (Minor): task 91's acceptance says `10 passed` while its own vector runner adds a test per file

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-91.md:262
**Problem:** test 2 is a loop that emits one `it` per JSON file in task 80's `view-filter`
directory (`task-91.md:117-127`), and test 2's own assertion is that the vector count is greater
than zero. The acceptance line's expected output is `10 passed`, which can only be true when the
directory is empty — exactly the case the task says must fail. The reviewer running acceptance
gets a mismatch on a green suite.
**How to fix:** expect `10 + N passed`, or assert on `Test Files 7 passed` plus a separate line
counting the vectors, and keep the numeric expectation off the generated block.

### S10-M2 (Minor): task 91 omits the *Current cycle, by state* saved view its scope names

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-91.md:22
**Problem:** the scope ends its build list with *"the saved view `Current cycle, by state`, never
the default"* (`tasks-81-100.md:253-254`), from 05 §4.1's ruling that the W1 lens tab survives as a
saved view in W2. The part cites the sentence in its anchors and then never builds the view: no
decision, no file, no test, and no seed of the view at team creation. Nothing else in the range
creates it, so the W1 affordance disappears when the tab becomes a view.
**How to fix:** add a decision and a test — creating a team seeds the shared view *Current cycle,
by state*, `isDefault` false — and name which task seeds it if team creation is task 72's.

### S10-M3 (Minor): task 91 is the only part in this range with no *Parity rows served* line

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-91.md:284
**Problem:** tasks 92 to 96 each end with *"Parity rows served: …. Task 97 proves them"*
(`task-92.md:392`, `task-93.md:350`, `task-94.md:300`, `task-95.md:394`, `task-96.md:303`). Task 91
ends at *Gaps reported to the skeleton* with no such line, and it is the part that carries the most
rows (L-93 to L-111 plus L-12's desktop half). The brief makes the parity matrix the coverage
checklist; the one part that closes nineteen rows is the one that does not state which.
**How to fix:** append the line, listing L-93, L-95, L-99, L-105, L-107, L-109 as closed here and
L-94, L-102 (W2 part), L-110 (W2 part), L-111, L-12 (desktop half) as joint with 80, 77 and 93.

### S10-M4 (Minor): task 92's backoff formula and its test expectations disagree by one doubling

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-92.md:202
**Problem:** the state machine says the retry re-arms at `min(IDLE_MS * 2 ** attempts,
RETRY_CEILING_MS)` with `attempts` already incremented. The test row expects *"delays 1.5 s, 3 s,
6 s"* for attempts 1, 2, 3 and *"the tenth is 30 s, not 768 s"* (`task-92.md:314`) — that is
`2 ** (attempts - 1)`. Written as specified, the first retry is 3 s and the red test stays red
after a correct implementation.
**How to fix:** make the formula `min(IDLE_MS * 2 ** (attempts - 1), RETRY_CEILING_MS)`, or change
the expected sequence to 3 s, 6 s, 12 s. The `768 s` in the test line is the arithmetic that
settles it.

### S10-C1 (Critical): task 93 depends on `GET /v1/issues/resolve?ref=` — no task builds it, no matrix row, no authorisation rule

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-93.md:344
**Problem:** decision (c) makes alias resolution the Hub's job — *"The handler sends the whole
string to `GET /v1/issues/resolve?ref=` (task 83's relay call)"* (`task-93.md:64-70`) — and the
test *"an alias resolves through the Hub"* asserts `resolve?ref=EP-142` is called and that no
`split("-")` appears in `links.ts`. The route exists nowhere: `grep -rn "issues/resolve"` over all
187 parts, the scopes and the skeleton returns only `/v1/comments/{commentId}/resolve` (task 12)
and `/v1/guidance/resolve` (task 100). The nearest real route is
`GET /v1/projects/{projectId}/issues/by-number/{n}` (`task-08.md:320`, matrix row at
`task-07.md:405`), which requires the caller to have **already** turned `EP` into a project id —
exactly the parse decision (c) forbids and the test's mutant kills. So the task's central test
cannot go green as written, and the part's own correction 3 leaves it as *"step 1 confirms it
exists or raises it with the WH stream"*, i.e. an unowned dependency on the critical path.
A route bolted on at implementation time also arrives with no `wread` row in task 07's conformance
matrix and no stated authorisation: ref-to-key resolution over an org-wide alias space is an
existence oracle for issues the caller may not read, and `issue_aliases` is insert-only
(`task-74.md:62`), so every alias a project ever had stays resolvable.
**How to fix:** give the route an owner and a part — the natural home is task 08 (numbers and
`project.key`) or task 80 — specified as `GET /v1/issues/resolve?ref=<alias|key>` scoped to the
caller's org, returning `404` (never `403`) when the resolved issue is outside the caller's project
reads, with a `wread` matrix row and a conformance test (mutant: resolving another project's alias
returns the key). Then make task 93's dependency list name it. Until that part exists, mark
decision (c) UNVERIFIED in 93 and price the fallback.

### S10-I3 (Important): task 93's URL prefill lands in the editor with no rule that it is plain text

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-93.md:148
**Problem:** `Prefill.description` carries up to 16,384 characters from outside the org — a URL in
an email is the one untrusted input this phase accepts — and is handed to the composer task 92
wires to the ProseMirror editor. The part bounds the **length** and drops unknown **keys**, but
never says how the value is inserted. If the composer runs it through the Markdown/HTML path (task
92's callout `parseDOM` accepts `blockquote` and `aside[data-callout]`, `task-92.md:221-224`, and
task 49's paste handler reads `data-author`), a crafted link can seed a composer with a callout, a
link, or a run that renders as `text·A` — agent-authored provenance forged from a URL.
**How to fix:** state in decision (b) that `title` and `description` are inserted as **plain text
nodes**, never parsed as Markdown or HTML, and add a test: a prefill of
`<aside data-callout="stop" data-author="A">x</aside>` and of `> [!CAUTION]` renders as literal
characters, with mutants *the prefill is parsed as Markdown* and *the prefill arrives as `text·A`*.

### S10-C2 (Critical): task 94 opens a `/v1` CSV route in the system browser, which the Hub contract refuses and no browser can authenticate

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-94.md:205
**Problem:** decision (f) composes `${hubOrigin}/v1/projects/${projectId}/issues.csv` and hands it
to `@tauri-apps/plugin-opener`. Every `/v1` route *"requires `Authorization: Bearer <Entra access
token for Hub.Access>`"* and the contract's **No browsers** rule refuses a request carrying an
`Origin` header, 403 `forbidden_origin`, before authentication
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:36-39`). The system
browser holds no bearer token — the desktop's live in Windows Credential Manager (`OD-6`) — so the
download is a 401, and the Hub page session task 80 builds is scoped to `/p/`, *outside* `/v1`,
precisely because of that rule (`tasks-61-80.md:674-678`). The part's *does not claim* covers only
*"whether the export page signs in"*, which reads as a session question; the real answer is that
the route as addressed cannot be reached from a browser at all. The dangerous repair is the one an
implementer reaches for under time pressure: accept the `/p/` cookie session on `/v1`, which puts
every `/v1` route behind an ambient credential and re-opens CSRF across the whole API.
**How to fix:** move export to the page surface — `GET /p/projects/{projectId}/issues.csv?viewId=`
alongside `GET /p/issues/{issueId}`, same `HttpOnly`, `SameSite=Strict` session, same RLS and the
same formula-injection prefix — and have `exportCsv.ts` open that. Raise it as a correction to task
80 (its route list and its `matrix_integration_test.go` row), and add the test *the export URL
never contains `/v1`* with mutant *the API path opened in a browser*.

### S10-I4 (Important): task 94's digest test expects `3 changes across 2 issues` for 43 events

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-94.md:222
**Problem:** the row reads *"40 events on one issue plus 3 on another → `3 changes across 2
issues`… and two popover groups, `EP-142 — 40 changes`"*. The line's own format
(`Since yesterday: 14 changes across 6 issues`, decision (a)) makes `changes` the event count, so
the expected text is `43 changes across 2 issues`. As written the assertion is satisfied by an
implementation that counts only the smaller group — and the mutant this test exists to kill (one
item per event) would also be killed by the wrong number, so the error survives a green run.
**How to fix:** change the expected string to `43 changes across 2 issues`, and add the second
group's count to the popover assertion so both group counts are pinned.

### S10-I5 (Important): the digest read mark and digest settings are raised as gaps but left with no owner inside the section

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-94.md:290
**Problem:** corrections 3 and 4 name two pieces of Hub state with no writer: `DigestSettings`
(*"Nobody owns change my digest settings"*) and the per-person read mark behind
`GET /v1/work/digest?since=` (*"Task 81's scope does not say who stores it"*). Both are load-bearing
for decision (b) — `since` must survive a restart or the line repeats yesterday's counts on every
launch — and both are left as *raised as a skeleton gap* with no task, while task 96 (preferences)
is ruled out in the same paragraph because the data is Hub-side, not a device preference. Task 97
then lists L-96 and L-37's digest halves as W2 rows to prove.
**How to fix:** name the owner in the part and in the skeleton's file map: `readAt` returned and
advanced by task 81's digest route (`POST /v1/work/digest/read`), and `PUT /v1/me/digest-settings`
on task 72 or 81, each with a matrix row. If they land later, say in this part that the fallback is
a session-lifetime `since` and record the consequence (the line repeats after a restart).

### S10-I6 (Important): task 95's generator pins `--mono` to Cascadia Mono, which undoes OD-28 and task 27

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-95.md:317
**Problem:** the test row reads *"the emitted `--mono` still ends `monospace` and still begins
`"Cascadia Mono"` (`tokens.css:152`)"*, and decision (a) emits the type stacks from a **preamble
kept verbatim**. Task 27 — *"faces: Commit Mono bundled with its licence; Segoe UI Variable as the
sans"* (`task-27.md:1`, skeleton `:428`, W1, before this task) — replaces that line under `OD-28`
(*"Every id, path and time is set in Commit Mono"*, `2026-09-23-owner-decisions.md:165`). Task 95
depends on 33 only (skeleton `:502`), so an implementer reading this part after 27 has merged
either fails the test or "fixes" it by emitting the pre-27 preamble, silently reverting the owner's
font decision across every id, path and time in the app.
**How to fix:** change the assertion to *the emitted `--mono` is byte-identical to the preamble's,
whatever it is, and ends `monospace`* — the mutant is *the preamble is rewritten or dropped*, not a
particular family — and add 27 to this task's `dependsOn` in both the part and skeleton `:502`.

### S10-I7 (Important): task 96 raises a skeleton gap for the two faces that task 27 already owns

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-96.md:289
**Problem:** correction 3 says *"OD-28's two faces are not in the tree, and this task does not
install them … it belongs to no task in the skeleton that this agent can find. Raised as a skeleton
gap."* Task 27 is that task, in W1, with the same title, and the skeleton lists it at `:428` with
seam SW.11 — the same seam tasks 95 and 96 extend. The correction as written sends the reader
hunting for work that is planned, and its reasoning (*"it changes the metrics of every row"*) is
the argument task 27 already answers. A false gap in a part is worse than a silent one: the next
agent adds a duplicate font task.
**How to fix:** replace correction 3 with a dependency note — *the faces land in task 27 (W1); this
task emits whatever the preamble holds and asserts nothing about the family* — and cite
`task-27.md` and skeleton `:428`.

### S10-I8 (Important): the light ramp pushes `--field` and `--page` past 100 % lightness, where the clamp silently merges them

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-95.md:76
**Problem:** decision (c) gives `--field` Δ −3.5 and `--page` Δ −2.0 from the base, *"multiplied by
`--seed-contrast` and clamped to `[0, 100]`"*; decision (d) derives `paper` by reading the same
table **with the sign inverted** from `--seed-base: oklch(97.2% …)`. Run the arithmetic:
`paper`'s `--field` is 97.2 + 3.5 = **100.7**, clamped to 100; `--page` is 99.2. At task 96's
`paper-hc` (contrast 1.5, same base, `task-96.md:115`) `--field` is 102.45 and `--page` is 100.2 —
**both clamp to 100 and emit the same colour**, so the input well, the window ground and pure white
become one plane in the theme an accessibility user is most likely to run. The sign inversion is
also wrong in kind for `--field`: on a dark ground a well recedes by going darker, and on a light
ground it should also recede, not become the brightest surface in the theme. No test catches it —
the *derived, not inverted* test asserts only that `paper`'s `--surface` is darker than its
`--canvas`, and nothing asserts that the seven surfaces are distinct or that no clamp fired.
**How to fix:** give the Δ table a per-token direction rather than one per theme (`--field` and
`--page` recede on both grounds), or move `paper`'s base down to about 94 % so the whole ramp fits.
Then add two tests: *the clamp never fires* (the generator throws when a derived lightness leaves
`[0, 100]` before clamping) and *the seven surfaces are pairwise distinct by at least 1 % L in
every theme*, with mutant *`--field` and `--page` collapse at contrast 1.5*.

### S10-I9 (Important): task 98's `ON DELETE SET NULL` nulls `project_id` too, so an issue delete fails instead of unbinding the task

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-98.md:150
**Problem:** the migration declares the composite key
`FOREIGN KEY (project_id, issue_id) REFERENCES hub.issues (project_id, id) ON DELETE SET NULL`,
and the prose promises *"The task stays, its `issueId` becomes null, and the Desk shows it
unbound."* Postgres applies `SET NULL` with no column list to **every** referencing column, so the
delete tries to null `hub.tasks.project_id` as well; against a `NOT NULL project_id` the issue
delete errors, and against a nullable one the task loses its project. Neither is the stated
behaviour, and the failure only appears the first time somebody deletes an issue.
The server is `postgres:16-alpine`
(`.claude/worktrees/hub/.superpowers/sdd/hub/task-02-report.md:200`), so the column-list form is
available.
**How to fix:** write `ON DELETE SET NULL (issue_id)`, and add a test to the list: *deleting an
issue leaves its tasks with `project_id` intact and `issue_id` null*, mutant *`SET NULL` without a
column list*.

### S10-I10 (Important): task 98's migration is called re-runnable but its `ADD CONSTRAINT` is not

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-98.md:141
**Problem:** the part says *"Written to be re-runnable so a re-ordered merge cannot break it"* and
step 4 *"applied to the dev database, then rolled back and applied again to prove it is
re-runnable"*. The column and index use `IF NOT EXISTS`; `ALTER TABLE … ADD CONSTRAINT` has no such
form in Postgres, so a second run fails `42710 constraint "tasks_issue_same_project" already
exists` and the transaction aborts — inside `BEGIN … COMMIT`, that also loses the index creation.
Step 4's own procedure is the thing that breaks.
**How to fix:** guard the constraint —
`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname =
'tasks_issue_same_project') THEN ALTER TABLE hub.tasks ADD CONSTRAINT … ; END IF; END $$;` — or
drop the re-runnable claim and rely on the migrator's version table. Whichever, say which, because
step 4's acceptance depends on it.

### S10-M5 (Minor): task 97's acceptance command 3 cannot be run as written

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-97.md:213
**Problem:** the check is
`grep -n '…' docs/verification/2026-09-25-phase-w-results.md | awk -F: '$1 > <W2 head line>'`.
`<W2 head line>` is a placeholder inside a shell command, so pasting the acceptance block gives an
awk syntax error rather than a result — and this is the check that catches an unfilled ellipsis,
the section's main failure mode.
**How to fix:** derive the line number in the command, e.g.
`awk '/^## W2: the team/{f=1} f && /…/{print NR": "$0}' docs/verification/2026-09-25-phase-w-results.md`
with expected output *no output*.

### S10-C3 (Critical): agent-written text enters the work layer as a bare `string`, bypassing Phase T's `teamtext` hygiene

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-99.md:285
**Problem:** `AgentCommentInput.Body` is `string` with the comment `// text·A`, and
`EvidenceProposal.Because` is the same (`task-99.md:156`); `Guidance.Text` is `string` marked
*"text·H"* (`:190`). Phase T's TM-02 builds `cp/internal/teamtext` for exactly these fields —
*"Bounded text with its author class (`text·A`, `text·H`); G3 hygiene (C0/C1, ANSI, bidi,
U+E0000–E007F, whitespace runs in names, secret-shaped text)"*
(`.claude/worktrees/team/docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:199`) — and TM-03
rules *"Every `text·A`/`text·H` field is a `teamtext.Text`"* (`:452`), with the refusal code
`text_refused`. Task 99 declares eight new codes and eighteen conformance rows and neither
`text_refused` nor any hygiene row is among them. So the one surface where an agent writes free
text onto a shared, human-read, agent-re-read thread accepts ANSI escapes, bidi overrides and
U+E0000–E007F tag characters — the invisible-instruction smuggling vector that `teamtext` exists to
stop — while Phase T refuses the same bytes one package away. W3's comments are read back by other
people's Orchestrators through task 104, so a smuggled instruction crosses an owner boundary.
**How to fix:** type `AgentCommentInput.Body`, `EvidenceProposal.Because` and `Guidance.Text` as
`teamtext.Text` once `feat/team-mode` is sync-merged (this task already depends on that merge
through 98), add `text_refused` to the code list, and add two rows: `WA-19` a comment body carrying
`U+E0001` and an ANSI `CSI` sequence → 422 `text_refused`, mutant *hygiene applied in Phase T only*;
`WA-20` the same bytes in `Guidance.Text` → 422, mutant *guidance is exempt*. If typing it is a
Phase T dependency this task cannot take, state the plain-string fallback (validate with
`teamtext.Check` at the boundary) rather than leaving the field unguarded.

### S10-M6 (Minor): task 99's decision (c) heading contradicts its own body and its code

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-99.md:60
**Problem:** the heading reads *"`comment` is the default kind and the only one a person may
write"*, and the next sentence reads *"A person writing `note` is allowed (people take notes)"* —
which is what `PermittedFor` implements (`note` and `evidence_proposal` return true for both
principals, `:103-113`). An implementer who reads the bold heading and writes the check from it
produces a `kind_not_permitted` on every human note, and no `WA-NN` row catches it: WA-03 and WA-04
only pin the two refusals.
**How to fix:** change the heading to *"`comment` is the default; a person may not write
`negotiation`, an agent may not write bare `comment`"*, and add a row — a person writing `note`
→ 200 — with mutant *the heading implemented instead of the function*.

### S10-I11 (Important): tasks 99 and 100 disagree on three W3 body shapes

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-100.md:266
**Problem:** task 100 writes the agent-comment body as an object —
`"body": { "text": "…", "author": "A" }`, with `400 invalid_request` *"for … a `body.author`
other than `A`"* (`:279`), and its matrix fixture builds the same shape
(`agentNote`, `:797`). Task 99's `AgentCommentInput.Body` is a bare `string`
(`task-99.md:285`). Two more: task 100's guidance body is `{ "text": …, "baseVersion": 3 }` with
`409 revision_conflict {headVersion}` (`:301-304`), while task 99's `GuidanceInput` is
`{Scope, ScopeID, Text}` with no `baseVersion` and no conflict code among its eight; and task 99's
`ResolveGuidance(ctx, orgID, teamID, personID)` takes a person id the route
`GET /v1/guidance/resolve?team=tm-…` has no parameter for. Task 99 is the wire and the conformance
rows; task 100 is *"what pass means"* run against the real Hub. Where they differ, the fake goes
green and the real Hub fails the same row — the exact failure task 05's conformance design exists
to prevent. The `{text, author}` shape corroborates S10-C3: task 100 already assumes a typed,
class-carrying text value.
**How to fix:** settle all three in task 99, since it owns SW.19 — `Body teamtext.Text` (or an
explicit `CommentBody{Text, Author}`), `GuidanceInput.BaseVersion int` with
`CodeRevisionConflict`, and either a `person` query parameter on `/v1/guidance/resolve` or a
signature without `personID` — then have task 100's Step 1 assert the spellings rather than
re-declare them. Add a golden vector per changed type.

### S10-I12 (Important): person-scoped guidance has no read rule and no matrix row

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-100.md:308
**Problem:** guidance is scoped `org | team | person` and the write side names its rule —
`403 forbidden {required:"org_admin"|"team_admin"|"self"}` (`:305`). The read side,
`GET /v1/guidance/{scope}/{scopeId}/versions`, lists **no** refusal at all, and the matrix rows
exercise only the `team` scope (`wread(… "/v1/guidance/team/{TeamID}/versions" …)`, `:775`). The
only other guard is the RLS policy, which isolates by `org_id` alone
(`:489-497`), so on the plan as written any member of the org can read — and list the version
history of — another person's personal guidance, which is text about how that person wants their
agents to behave. Task 99's conformance rows have the same hole: WA-09 and WA-10 test team and org
layering only.
**How to fix:** state the read rule beside the write rule (*person scope: the subject or an org
admin; team scope: a member of the team; org scope: a member of the org*), add matrix rows for
`/v1/guidance/person/{OtherUserID}/versions` on both verbs with `want` 404 for everyone but the
subject and the org admin, and add a conformance row `WA-21` (another member reading a person's
guidance → 404), mutant *org-wide read on a personal layer*.

### S10-I13 (Important): task 100 is priced at 3 hours and is three times the size of any other part in this range

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-100.md:3
**Problem:** the part is 958 lines against 292-440 for tasks 91-99, and its *Files* carry a
two-part migration (three tables, three trigger functions, RLS, grants), `store/work_agents.go`,
`server/work_agents.go` with the delegated-caller gate, ten client methods, the relay rows, ten
matrix rows, a Fleet-column integration file and nineteen tests — each of which Step 5 requires be
mutated by hand and reverted. The house rule is 1-3 focused hours per task. Three hours is not a
measurement (the part says so), but the mismatch matters: this is the last task of the section, so
an over-run lands on the spend limit that already stopped one run of this plan, and a half-applied
migration is the worst place to be interrupted.
**How to fix:** split on the seam the part already has — **100a** the migration, the store and the
database guards (the three triggers, RLS, grants), with the direct-insert half of the tests; and
**100b** the routes, the gate, the client methods, the relay and the matrix rows. The skeleton row
at `:510` becomes two, and 100b depends on 100a.

### S10-M7 (Minor): task 98's database acceptance proves the foreign key, not the same-project rule

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-98.md:281
**Problem:** the psql check inserts
`('tk-x','pr-a','is-in-project-b','us-1')` and expects the composite FK to fire. None of those ids
is well formed — the Hub mints `pj-` for a project and `u-` for a user
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:104-108`), and
`is-in-project-b` is not `is-` + 16 hex — and, more to the point, no issue of that id exists, so
the constraint fails because the referenced row is missing, not because the projects differ. The
command passes while the rule it claims to prove is untested.
**How to fix:** insert a real issue in project B first, then attempt the task row in project A with
that issue's id, and expect `tasks_issue_same_project`. Use well-formed ids
(`pj-…`, `u-…`, `is-…`) so the insert reaches the constraint under test.

Section verdict: Needs fixes
# Phase W plan review — section 11 (tasks 101 to 110)

Adversarial review of W3 (the verbs, Fleet on issues, the agent surfaces, proof) against
the scope paragraphs in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-101-120.md`,
the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`, and the parts they depend on.

## Task 101 — the work verbs

Scope covered: three verbs, the `panacea team` issue line, class rules, the rate cap, the alias
rule, the "Fleet down = exit 3" rule, and every named mutant has a test row. Citations spot-checked
against `.claude/worktrees/team` (TM-09 `model.Verb`, TM-19 `Caller`, TM-22's worker-snapshot
rule) and `04-internal-fit.md:355-360`: all real.

### S11-I1 (Important): decision (b) widens the owner check past the scope, and the corrections list hides it
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-101.md:44
**Problem:** the scope is flat — *"every verb acts only on issues the caller's **owner**
delegated (OD-16)"*. Decision (b) adds a third way in: `InTurn`, the subject issue of a running
`mention` turn, which lets Alice's Orchestrator `issue.show` **Dylan's** issue — title,
description, `serves` and the newest 50 comments — on the strength of a comment Alice wrote.
The part's own *Corrections to the scope paragraph* lists four corrections and does not list this
one, so a reader diffing scope against part will not see the widening. Nothing in the part states
the Hub still applies its own read rule on that path, so the reviewer cannot tell whether
`InTurn` is a second gate or a bypass of task 100's matrix.
**How to fix:** add the widening to *Corrections to the scope paragraph*; state explicitly that
the Hub's read authorisation on `GET /v1/issues/{id}` still runs under the caller's owner
delegation (so `InTurn` can only ever narrow, never widen, what that owner may read); and add a
test row *a mention turn on an issue the caller's owner may not read → `agent_not_owner`*
(mutant: "InTurn replaces the Hub's read rule").

### S11-M1 (Minor): the "complete" Go block calls two packages it does not import
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-101.md:151
**Problem:** `issue.go`'s import block is given as complete (context, encoding/json, time,
control, model, hubapi), but the `Note` body calls `wire.RefuseWith`, `wire.Refuse`,
`wire.CodeRateLimited`, `wire.CodeAgentNotOwner` and `hubclient.RouteWorkAgentComment`.
`wire` is real (TM-09's `fleet/wire/codes.go`) and `hubclient` is task 16's, so this is a
missing import, not a wrong name — but the house rule is complete source.
**How to fix:** add `fleet/wire` and `hub/hubclient` to the import block, or drop the
`hubclient.` qualifier by naming the route constant locally.

### S11-M2 (Minor): `TestEveryNamedWorkVerbIsServed` cannot fail on the half the scope named
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-101.md:496
**Problem:** the scope's mutant is *"block names a verb Fleet does not serve"*, against SW.7's
golden managed block. The part's own gap 1 records that task 19's `golden-issue.md` names no
verb, so that half of the assertion is vacuously true on day one and kills nothing; only the
`orchestrator.md` half bites. A later edit that adds a verb name to the golden is what the test
was for, and it would still pass today either way.
**How to fix:** either state in the test row that the golden half is a guard for later and the
kill comes from `orchestrator.md`, or make gap 1 a blocking precondition on task 19.

## Task 102 — `Shift+O` delegate, `Shift+A` ask, the strip line

Scope covered: both chords, the four refusal rows, the Undo window, the strip title rule, and the
five gaps are declared rather than hidden. Decision (e) correctly moves the title rule off TM-53
(which `task-53.md:10-11` forbids from computing a line) onto TM-52's assembly.

### S11-I2 (Important): the Undo guard is a lint test, not an authorisation
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-102.md:124
**Problem:** decision (c) puts the first-write rule in a new control-plane route
(`POST /v1/work/issues/{id}/delegation/undo`), then still mounts the raw Hub row
`WithdrawDelegation DELETE /v1/issues/{issueId}/delegate` on the relay, with nothing but
`TestTheDesktopUndoesOnlyThroughTheGuard` — *"a source scan of `desktop/src` finds
`WithdrawDelegation` only in `api/work.ts`'s table"* — standing between a caller and the
unguarded route. A source scan is a style check: it does not stop the relay answering the DELETE,
and any non-`desktop/src` caller on the loopback (a later task, a script, the CLI) withdraws a
delegation after the first file write, which is exactly the state 05 §4.6 (`:1105`) says is
irreversible. The tests `TestUndoAfterAFileWriteIsRefused` and
`TestUndoWithoutTheReaderIsRefused` both drive the guarded route, so neither bites here.
**How to fix:** do not relay `WithdrawDelegation` at all (the part already says the desktop
never calls it), or have the relay's W3 mount refuse that row unless the request carries the
guard's proof; add a Go test *the relayed DELETE is 404/405 from the desktop's relay* with mutant
"the raw withdraw stays reachable".

### S11-M3 (Minor): the desktop acceptance cannot tell 13 tests from 3
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-102.md:330
**Problem:** the acceptance is `npx vitest run … | tail -3` with expected output
`Test Files  N passed (N)`, and the part says *"N is the count Vitest prints for the named
files; it is not predicted here"*. Thirteen test rows are specified; the command passes if two of
them exist. Task 101's acceptance pins an exact `grep -c` count, so the house bar is available.
**How to fix:** add a second acceptance line pinning the test count, e.g.
`npx vitest run … --reporter=json | …` or `grep -c '✓'` against the 13 named tests.

## Task 103 — Orchestrator notes, the agent byline, `@me:orchestrator`

Scope covered: every scope mutant has a test row; decision (b) (the chip is relative to the
writer) and decision (e) (read on another person's issue, write only on your own) are stronger
than the scope asked and are the right reading of OD-16. Citations to `task-104.md:358-362`,
`task-109.md:84-131` and `:198-203` and `task-51.md:28-34` all resolve.

### S11-I3 (Important): `mention.go` will not compile against task 109's `package work`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-103.md:135
**Problem:** `mention.go` is declared `package work` and then refers to `killswitch.Turn`
(twice, in `Enqueuer` and `Refusals`) and re-declares `type Enqueuer interface` and
`type Refusals interface` *"restated so this file compiles against them"*. Task 109 puts
`Turn`, `Origin`, `Enqueuer`, `Refusals` and `Gate` in **the same package** — its own
header reads `// Package work holds Fleet's rules for agents on issues (Phase W, W3: tasks 103,
105, 109-111)` (`task-109.md:85`) and its file is `fleet/work/killswitch.go`
(`task-109.md:768`). So `killswitch.` names a package that does not exist, and the two
restated interfaces are duplicate declarations in `package work`. The restatement trick is
legitimate across packages (task 101 uses it for `DelegationKey`); inside one package it is a
build error.
**How to fix:** delete the restated `Enqueuer`/`Refusals` block and the `killswitch.`
qualifier from task 103; write `Gate Enqueuer` and `Turn{…}` unqualified, and say in the
anchors that task 109 declares them in this package.

### S11-I4 (Important): 103's `dependsOn` omits 109 and 104, yet it cannot be built without them
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:517
**Problem:** the skeleton row gives `dependsOn: 100` and no seam. The part's `Mentions`
struct takes task **109**'s `Gate`, `Origin`/`OriginMention` and `Refusals`, and its
composer calls task 109's `WorkAgentSettingsApi.agentSwitch(orgId)`; its `work.ts` delta
takes task **104**'s `CommentKind` and `AgentAttribution`. Built in id order, 103 lands six
tasks before the package and the gate it compiles against, so Step 2's *"see each fail for its
reason"* cannot be distinguished from *"fails because nothing exists"*, and
`TestAMentionWhileOffEnqueuesNothing` has no gate to refuse it. Task 104 at least names the
collision (its gap 1: *"the first W3 desktop task to land adds these"*); task 109's Go types have
no such note.
**How to fix:** change the skeleton's row 103 to `dependsOn: 100, 104, 109` (or move task 109
ahead of 103 in the W3 order) and record the reorder in the skeleton's task list.

### S11-M4 (Minor): `ORCH_INPUT_RULE` as written can never match
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-103.md:206
**Problem:** `/(^|\s)@me:orchestrator(?=[\s,.;:!?])$/` asserts, at the same position, both
*the next character is a space or punctuation* and *end of string*. Those cannot both hold, so the
rule never fires and the literal stays plain text — the exact behaviour the part reserves for
`@dylan:orchestrator`. The test `mentionPlugin › @me:orchestrator becomes the token` would
catch it, but the part offers the regex as complete source.
**How to fix:** consume the terminator instead of asserting it:
`/(^|\s)@me:orchestrator([\s,.;:!?])$/`, and keep the captured character in the replacement.

## Task 104 — proposed evidence under the criterion, *Record* as the person

Scope covered and then some: the three rules (a proposal is a comment, Record is the person's, an
observed record needs no click) are right, the Go tests are written out in full with their mutants,
and `criterion_gone` is a case the scope did not ask for and should have. `hub-contract.md:1074`
(*"recordedBy is the principal"*) and `task-109.md`/`task-99.md` citations resolve.

### S11-I5 (Important): tasks 101, 104 (and 100) use Hub type names SW.19's owner does not declare
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-104.md:357
**Problem:** the skeleton makes **task 99** the owner of SW.19 (`…phase-w-work-layer.md:393`),
so 100-112 consume its names. Task 99 declares
`AddAgentComment(… hubapi.AgentCommentInput) (hubapi.IssueComment, error)`,
`ListEvidenceProposals(… hubapi.ProposalFilter) ([]hubapi.EvidenceProposal, error)` and
`LinkEvidenceProposal(ctx, commentID, evidenceID) **error**` (`task-99.md:256-262`). Task 104
writes `hubapi.EvidenceProposalComment`, `hubapi.CommentEvidenceProposal` and a `Linker`
returning `(hubapi.EvidenceProposalComment, error)`, and says these are *"as task 99 names
it (SW.19)"* — they are not; they are task 100's (`task-100.md:193-215`). Task 101 has the same
divergence (`hubapi.AgentCommentRequest`, `task-101.md:112`). Three of the four parts agree
with each other and disagree with the seam owner, and task 100's own Client would not satisfy
task 99's `WorkAgents` interface (one return value against two).
**How to fix:** pick one set in the seam owner (task 99), rename in 99 or in 100/101/104, and
delete the "as task 99 names it" claim where it is false. Add a conformance row to task 99's
table: *`hubclient.Client` satisfies `WorkAgents`* (a compile-time `var _ WorkAgents =
(*Client)(nil)`), mutant "the client drifts from its interface".

### S11-M5 (Minor): gap 2 is stale — the link route already exists in tasks 99 and 100
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-104.md:733
**Problem:** gap 2 reads *"Task 99's method list has no link from a proposal to its record …
tasks 99 and 100 should add it, with a matrix row"*, and *What this task does NOT claim* repeats
it. Both already have it: `task-99.md:260` and conformance row `WA-08`, and
`task-100.md:130` (`POST /v1/comments/{commentId}/recorded-as`), `:213`, `:602`,
`:691-695` with `TestALinkChecksTheRecord`. A reader acting on the gap would re-add a route
that exists.
**How to fix:** replace gap 2 with the real residue — the signature and type-name divergence of
S11-I5 — and cite `task-100.md:290-298` as the route this part calls.

### S11-M6 (Minor): *Link again* rejects unhandled
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-104.md:490
**Problem:** `onLinkAgain` is `void api.recordEvidence(…).finally(reload)` in `LaneCard`
and `relink(v, c).finally(reload)` in `Serves`. The control plane answers *Link again* with
409 `already_settled` by design (decision 3 and the route table), so the promise rejects on
every successful relink; `.finally` reloads but nothing catches, so each click logs an
unhandled rejection and, under Vitest's default, can fail the suite.
**How to fix:** `.catch(() => {})` after `.finally(reload)`, or have `recordEvidence`
resolve `already_settled` as a value (it already carries `evidenceId`) rather than throwing.

## Task 105 — Fleet: §3.7 rules 2 and 3, the `sequence` proposal

Scope covered: rule 3 seeds the **editor's** Orchestrator and pauses nothing, rule 2's wind-down
goes to the **former** owner, `duplicates` is a locked *decide*, and the solo path is untouched.
Decision 1 is the only part in W3 that checked Phase T's real `turns.Origin` (verified:
`team/…/task-26.md:158-163` has exactly `human` and `fleet`).

### S11-C1 (Critical): the kill switch has two implementations, and task 109's proof grep cannot see this one
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-105.md:200
**Problem:** task 109 declares *"One gate, one place … Fleet's wiring hands every caller this,
never `Next`"* (`task-109.md:40`, `:158`) and proves it with
`grep -rnE "\.Next\.Enqueue|queue\.Enqueue" control-plane/internal/fleet … # expect: no
output` (`task-109.md:768`). Task 105 does not go through the gate: it declares its own
`Switch` interface, reads `AgentsEnabled` itself, and calls `d.Queue.Enqueue(ctx, e)` on
TM-26's queue directly. The grep is case-sensitive and matches lowercase `queue.Enqueue`, so
`d.**Q**ueue.Enqueue` slips past it and 109's acceptance still prints nothing. The switch is
therefore enforced twice, by two code paths with two failure modes, and the one test that is
supposed to prove single-chokepoint (`TestSwitchOffEnqueuesNoTurnForAnyOrigin`, which iterates
`OriginRelation` and `OriginHandover`) exercises a gate that the relation and handover paths
never call. A later edit to 105's own check — or a third W3 path that copies it — turns the switch
off for that path with nothing failing.
**How to fix:** make 105 enqueue through task 109's `Gate` (delete 105's `Switch` interface
and its two `AgentsEnabled` calls), reconciling the two `Enqueue` signatures first (see
S11-I6); and change 109's grep to case-insensitive and anchored on the method, e.g.
`grep -rniE "\.(next|queue)\.Enqueue\("`, so a renamed field cannot hide a second caller.

### S11-I6 (Important): three W3 parts declare conflicting symbols in one Go package `work`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-105.md:100
**Problem:** tasks 103, 105 and 109 all write `package work` under
`control-plane/internal/fleet/work/`, and each declares its own `Enqueuer`:
105's is `Enqueue(ctx, e model.Event) error` (`task-105.md:101`), 109's is
`Enqueue(ctx, t Turn) error` (`task-109.md:123`), 103's is a third copy
(`task-103.md:150`). `Refusals` is declared twice (103, 109) and the switch reader twice under
two names (105's `Switch`, 109's `SwitchReader`). In Go these are duplicate declarations in
one package: the stream does not build once two of the three land. The parts also disagree on
what a turn is — 105 enqueues a `model.Event` on TM-26's queue, 109 enqueues its own `Turn`
with a six-value `Origin` that TM-26 does not have.
**How to fix:** give one part (109, the gate's owner) the package's shared types —
`Enqueuer`, `Refusals`, `Switch`, `Turn`/`Origin` — in one file, make 103 and 105
consume them unqualified, and add the reconciliation to the skeleton as a precondition of
whichever of the three lands first.

### S11-I7 (Important): 105's acceptance count is already wrong when 103 has landed
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-105.md:556
**Problem:** `go test ./control-plane/internal/fleet/work/... -race -count=1 -v | grep -c
"^--- PASS"` with `# expect: 9` counts **the whole package**, not this task's tests. Task
103 adds four tests to the same package (`TestAMentionEnqueuesTheWritersOrchestrator`,
`TestAMentionIsEnqueuedOnce`, `TestAMentionWhileOffEnqueuesNothing`,
`TestAnUnroutedCommentIsIgnored`) and lands first, so the command prints 13 and the acceptance
fails on correct code; task 109 adds more later.
**How to fix:** pin the count with `-run` over this task's ten named tests, as task 101's
acceptance does, or count `^--- PASS` filtered by name.

### S11-M7 (Minor): an unassigned side is audited as `same_owner`
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-105.md:165
**Problem:** `Why` is introduced as *"a closed set: every non-enqueue says which rule stopped
it, for the audit and the tests"*, then `if from == "" || to == "" || from == to { return
Outcome{Why: WhySameOwner} }` reports an **unassigned** issue as `same_owner`. The audit row
then names a rule that did not fire, and no test distinguishes the two.
**How to fix:** add `WhyUnassigned` and split the branch; add a test row *a `blocks` where one
side is unassigned → `WhyUnassigned`, no enqueue* (mutant: "unassigned read as same owner").

## Task 106 — the negotiation row

Scope covered: both issues, the fold, `moves.hold`, the one polite announcement, the typed-state
link and no transcript, and `round 2 · limit 3` instead of 02's `round 2 of 3`. The TM-52
citation is real and the types line up (`team/…/task-52.md:892` `Resolution` is a string union,
`:925-939` `ResolutionProposal`/`ThreadMessage`/`ThreadView`), so `proposed?.kind` is
correctly typed.

### S11-I8 (Important): the row's only data source is a record no task in the plan writes
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-106.md:4
**Problem:** `useNegotiations` is driven by the issue's `negotiation` comments, and both this
part (gap 3) and task 105 (gap 7) record that **nothing writes them**: *"neither 105's scope nor
this part writes it … the skeleton should give it to 105 as a follow-up row or to a new W3
task"*. The scope for 106 named a different source — *"the thread's typed state from Phase T's
team read model (TM task 52)"* — with no comment in it, so the part added a producer dependency
the scope did not have and then left it unowned. As the plan stands, every acceptance in 106
passes against fixtures while the surface renders nothing on a real issue, and the declared W3
proof (task 113) has no way to reach this row.
**How to fix:** either (a) add the writer to task 105 as an explicit step — one `negotiation`
comment on each issue through `AddAgentComment`, written as each side's Orchestrator when the
thread opens — and give 105 a test *a thread opened in a `work.relation` turn writes exactly one
`negotiation` comment on each of the two issues* (mutant: "the row has no data"); or (b) drop
the comment and resolve the thread from TM-52's read model keyed on the issue, as the scope said.
Whichever is chosen, the skeleton's task list needs the row.

## Task 107 — the agent task card, `agentsOf`, `who.label`

Scope covered in full, and the security rules are the strongest in the section: no steer control
exists on any card, `open session` is gated on `m.mine`, the source scan forbids
`attachRegistry|xterm|Terminal|transcript|openAttach|prompt`, and `agentsOf` is evaluated at
render time so nothing agent-shaped is stored on an issue. `AGENTIC-UX.md:75` (the closed set of
seven) and `05-design.md:855-859` verify; so do the parity rows it claims
(`05-design.md:349` L-182).

### S11-I9 (Important): 106 and 107 call store actions TM-52 does not declare
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-107.md:167
**Problem:** TM-52's team slice declares exactly `openOrchestrator(projectId: ProjectId,
subject?: AskSubject): void` (verified at `team/…/task-52.md:1028`, and used by TM-53 and
TM-55). Task 102 calls it by that name. Task 107 calls
`useCockpit.getState().openOrchestratorPanel(projectId, {kind:"agent", ref})` and task 106
calls `useCockpit.getState().openTeamThread(threadId)` — neither exists. 106's is the worse
case: its scope requires the row to *"link to the thread's typed state"*, and TM-52's
`AskSubject.kind` is `task | path | agent | conflict` with no thread member, so there is no
API for the link at all and 106 never files it as a gap (its gap list covers `resolvedAt`,
`informed`, `asked`, not this).
**How to fix:** rename 107's call to `openOrchestrator(projectId, {kind: "agent", ref})`;
in 106, either add `{kind: "thread", ref: th-…}` to `AskSubject` as a declared Phase T
delta beside task 102's `"issue"` member (rule H2), or route through `setThread` +
`openOrchestrator(projectId)`; and add the delta to 106's gap list.

## Task 108 — `step.fold`, the ephemeral line, the links, the budget

Scope covered: the four-field fold, the commits degradation with no error banner, the one
heartbeat, the 10 s budget as a fact, and a source scan that forbids PTY and terminal imports.
The parity rows it closes verify (`05-design.md:352-357`, L-185/186/188/190) and none of them
requires the delegated half.

### S11-M8 (Minor): the `worktree` link has no `mine` guard and no test covers a teammate's session
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-108.md:230
**Problem:** decision 6 is *"a link renders only where its fact exists"* and the mutant is
"dead link; *open session* on another person's agent". `open session` is gated on `mine`;
the `worktree` button is gated only on `s.worktreeId`, so a teammate's or a delegated cloud
session renders a button that calls `selectWorktree` with an id that is not on this machine
(worktrees are local clones, OD-2). The test *"renders a link only where its fact exists"\*
uses `mine={false}` with `worktreeId: ""` and then `mine` with a worktree, so the one
combination that fails — `mine={false}` **with** a worktree id — is never rendered. Latent
today because `IssueAgentLine` passes `mine` hard-coded true, live as soon as task 107's card
or any later surface mounts it.
**How to fix:** gate the worktree button on `mine && s.worktreeId`, and change the test's first
case to `mine={false}` with `worktreeId: "w-1"` expecting zero links.

### S11-M9 (Minor): the scope's "delegated ones included" is dropped without a corrections line
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-108.md:57
**Problem:** the scope reads *"this task brings the same budget and the fold to every agent on
the issue view, **delegated ones included**"*. Decision 7 gives delegated agents the line and no
fold, for a good reason (the transcript lives in the sandbox), and gap 2 records the ask to Phase
T — but unlike tasks 101-104 this part has no *Corrections to the scope paragraph* section, so the
reduction is only visible to a reader who reads all three places. The parity rows it closes do not
need the delegated half, so this is bookkeeping, not coverage.
**How to fix:** add a *Corrections to the scope paragraph* line naming the reduction, as tasks
101-103 do.

## Task 109 — guidance, the kill switch, the published rules

Scope covered: three layers most-local-first, versions never overwritten, an org-admin-only flip
with a required reason, fail-closed on an unreadable switch, *pause all* scoped to the caller's own
agents, and a doc test whose citations I checked by hand — every heading exists
(`AGENTIC-UX.md:64` *4. Principles*, `:232` *10. Anti-patterns*;
`05-design.md:313` *L. Linear's own AI*, `:513` *2.1*;
`team/…/05-design.md:269`, `:360`, `:776`). Its decision 5 (*"Tasks 103, 105 and 110 call
the wrapped queue; none reads the switch itself"*) is the sentence task 105 contradicts — see
S11-C1.

### S11-M10 (Minor): the published rules file ships worktree-local source paths
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-109.md:441
**Problem:** rules 1, 4 and 5 of `docs/work/agent-rules.md` cite
`.claude/worktrees/team/docs/superpowers/planning/…/05-design.md`. That path exists on one
machine, in a git-ignored worktree directory; the file is the **published** statement of how our
agents behave, so anyone outside this checkout reads three dead citations, and the doc test
(`existsSync(path)`) fails on any clone without that worktree — including CI. Step 5 says to
rewrite them *"if `feat/team-mode` is merged"*, which makes the task's own acceptance
conditional on a branch it does not control.
**How to fix:** make the three rules cite a path that exists in this repository (the Phase W
design's §2.1/§3.4, or `docs/PROGRAM.md`) and add the team-design citation only after the
merge; or make step 5's rewrite a hard precondition, so the task cannot go green on a tree where
the paths are wrong.

### S11-M11 (Minor): 109's acceptance `-run` pattern catches task 105's tests
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-109.md:762
**Problem:** `go test ./control-plane/internal/fleet/work/ -run 'Switch|Hub|Every' -v
# expect: 4 tests, all PASS` runs over the shared `work` package, where task 105 has already
added `TestTheSwitchOffOrUnreadableEnqueuesNothing` — which `Switch` matches. The count is
wrong before the task starts. Same defect as S11-I7, from the other end of the package.
**How to fix:** name this task's four tests in `-run` explicitly.

## Task 110 — the read-only *ask* worker

The strongest security work in the section: `safePath` rejects absolute, `..`, backslash,
leading `-` and `:`; the git spec starts with 40 hex so no argument can be read as an option;
only git **objects** are read, so an uncommitted edit cannot make a citation look right; an
upstream `check` is cleared and re-derived; the answer renders as text with no markdown or
`dangerouslySetInnerHTML`; `Read` returns `not_found` to anyone but the asker; `Start`
calls `Gate.Allow` with `OriginAsk` and uses task 109's types unqualified, correctly. The
parity claim verifies (`05-design.md:324`).

### S11-I10 (Important): `subject` is accepted on three hops and dropped on the fourth
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-110.md:157
**Problem:** the scope's interface is `AskRequest {project pj-, question, subject?: {issue is-,
path}}`. The part carries it through the wire example (`:84`), the loopback handler
(`:435`, `:451` — `StartAsk(… , body.Subject)`) and the palette command (`:578`, which
sends `{issue: ctx.focusedIssue.id}`), then loses it: `Asks.Start` never reads
`req.Subject`, `AskLauncher.LaunchAsk(ctx, owner, delegation, project, commit, question)` has
no subject parameter, and `StoredAsk` has no subject field. So the ask worker is never told
which issue or path the question is about, and `AskResult.openIssueComposer` — documented at
`:519` as *"Present only when the ask had an issue subject"* — can never be populated, which
also breaks decision 8's *Post as my comment* for the issue case. No test covers the subject and
no gap records it.
**How to fix:** add `Subject` to `LaunchAsk` and to `StoredAsk`, run `subject.path`
through `safePath` and `subject.issue` through the `is-` shape check on the way in, echo it
in `AskResult`, and add a test *an ask with an issue subject stores it and `AskResult` names
that issue* (mutant: "the subject is dropped") plus *a subject path with `..` is refused*
(mutant: "the subject skips path validation").

---

Section verdict: Needs fixes
# Phase W plan review — section 12 (tasks 111-120)

Reviewer pass: adversarial. Read-only except this file.
Parts reviewed: task-111 .. task-120 in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/`.
Scope: `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-101-120.md` lines 283-530; skeleton rows 521-535.

## Task 111 — risk facts, the repository proposal, Orchestrator-filed issues

Scope coverage: all three rows (L-174, L-192, L-193) are carried, the three fact names are closed,
the facts are derived and never stored (skeleton rule 3), and no score, level or colour appears.
Citations checked and real: `05-design.md:336`, `:359`, `:360`; `AGENTIC-UX.md:194` (AX §7 point 6);
`AGENTIC-UX.md:247` (anti-pattern 12); `tasks-81-100.md:485-486` (task 100 forces `triage`).
Tests are red-first with named mutants and the delete case is right (a deleted migration raises
nothing; a deleted prod-config path still raises `touches-prod-config`, `task-111.md:143-154`).

### S12-I1 (Important): `issue.file` is wired to a hubclient method that task 99 does not define
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-111.md:383
**Problem:** the part declares `IssueFiler.FileIssueAsAgent` and comments *"the method name is task
99s"*. Task 99s `WorkAgents` (`task-99.md:256-265`) has no create method at all: `DelegateIssue`,
`AddAgentComment`, `ListEvidenceProposals`, `LinkEvidenceProposal`, `PutGuidance`,
`ResolveGuidance`, `GuidanceVersions`, `GetAgentSwitch`, `SetAgentSwitch` and (adopted later)
`WithdrawDelegation`. Task 100 files an Orchestrator issue through the ordinary route `POST
/v1/projects/{p}/issues` with the delegated caller (`task-100.md:823`), i.e. task 04s
`WorkIssues.CreateIssue` (`task-04.md:179`). An implementer following this part will either add an
undeclared method to the W3 seam SW.19 (whose owner is task 99, frozen once 100-112 consume it) or
stall.
**How to fix:** bind `IssueFiler` to `WorkIssues.CreateIssue` with a `Delegated` context, name that
in the comment, and add it to Step 1s bind list; or raise the extra method to task 99 as a seam
gap in the same form the part uses for gaps 1-4.

### S12-M1 (Minor): a fourth work verb against a skeleton that says three
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-111.md:340
**Problem:** `issue.file` is a fourth Fleet verb; the skeleton delta row (`2026-09-25-phase-w-work-layer.md:287`)
says *"three verbs"* and names consumers 98 and 101 only, and task 112 still tests *"every ST.1
fixture for the three verbs"*. The part flags this as its gap 6 but no task owns the skeleton edit.
**How to fix:** name the skeleton delta row edit (verb count and consumer 111) in the parts gap
list as an explicit ask to the plan owner, as tasks 117 and 119 do for their contract proposals.

## Task 112 — the three verbs in solo mode

Scope coverage: the loopback boundary, the per-session token, the ignore-the-body stamp, the
Start-bound one-voice rule and the `solo_project` refusal are all here, and the runner question is
a real spike with a hard stop (`task-112.md:706-717`). Security reads well: bind check twice,
token never logged or returned, credential in the environment only.

### S12-I2 (Important): the tasks two write verbs cannot ship, and no task owns the missing piece
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-112.md:57
**Problem:** decision 6 finds that task 100 refuses a persons own token on `agent-comments`, so
solo `issue.note` and `evidence.propose` answer `unavailable` until a delegation whose holder is
the local control plane (`via: local`) exists; gap 2 says that needs a TM-06 delta and *"a WH task
after task 100"*. No such task exists: `via: local` and *local delegation* appear only in
task-112.md and task-113.md across the whole plan (parts, scopes and skeleton). As written, W3s
solo half is `issue.show` alone, the scopes own test *"the Hub write is as the persons own agent,
`text·A`"* has nothing to run against, and L-156 and L-166 stay open for solo with no owner.
**How to fix:** either add the WH task (the `via: local` delegation holder, its Hub guard and its
matrix rows) to the skeleton and make 112 depend on it, or state in the skeletons W3 row and in
task 113 that solo ships read-only in W3 and name the phase that closes it.

### S12-M2 (Minor): the task-100 citation points at the wrong line
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-112.md:57
**Problem:** `TestAPersonCannotWriteAnAgentKind` is cited as `task-100.md:796`; line 796 is
`func missingEvidence`. The test row is at `task-100.md:840`. The same `:796` is repeated in gap 2.
**How to fix:** change both to `task-100.md:840`.

## Task 113 — W3 proof and results

No findings. The eighteen W3 rows and their line numbers were re-derived independently from
`05-design.md` (`awk` over the §1 matrix on the slice column): L-39:148, L-156:318, L-158:320,
L-162:324, L-166:328, L-173:335, L-174:336, L-182:349, L-183:350, L-185:352, L-186:353, L-187:354,
L-188:355, L-190:357, L-192:359, L-193:360, L-194:361, L-246:428 — exactly the parts table.
`check-w3.sh` fails on a fixture with an empty *Observed* cell and on a non-sha commit cell, the
`[cut]` convention keeps the `…` ban usable (`task-113.md:288`), and the *does NOT claim* list
names the environment limit, the injection corpus and the owners eyes. OD-27 is enforced
mechanically, not asserted.

## Task 114 — the W4 wire (SW.20)

Anchors verified against `feat/hub` in the hub worktree, every one real: `hubapi/codes.go` is 67
lines; `canonical.go:282` is `type FieldError struct`; `keys.go:32` is `func ValidID`;
`envelope.go:394` is `FormatTime`; `types.go:95` is `type User`, `:196-202` is `Page`/`PageRequest`,
`:536-537` is `Event`/`Seq int64`; `hubclient.go:52` `Briefs`, `:101` `type Hub interface`, `:113`
`type Error`. Contract citations `:253`, `:309`, `:333`, `:341-359`, `:356` also check out.

### S12-I3 (Important): task 120 emits event kinds the seam does not define
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-120.md:99
**Problem:** SW.20 fixes the W4 event kinds in `hubapi/workevents.go` as `work.milestone_created`,
`work.milestone_updated`, `work.milestone_criteria_set`, `work.dependency_added` and
`work.dependency_removed` (`task-114.md:757-761`). Task 120 writes and *asserts* `milestone.created`,
`milestone.updated`, `milestone.criteria_set`, `feature.dependency_added` and
`feature.dependency_removed` (`task-120.md:99-100` and its tests at `:292-306`). Task 118 spotted
the clash in its gap 4; task 120 has not. Unreconciled, task 120s handlers write kinds no registry
knows, task 13s feed filter drops them for subscribers, and task 124s story table fails.
**How to fix:** task 120 takes SW.20s spelling (the seam owner wins, skeleton *Start lines*), in
both its route text and its tests; or 114 appends the alternate spellings and 118 follows.

## Tasks 115-120 — the W4 fake, migration and routes

Verified: task 115s `fake.go` and `fake_briefs.go` anchors are exact **at the commit it names**
(`git show 1271ddb` confirms `:139 enter`, `:172 stamp`, `:175 next`, `:184 problem`,
`:199-202 forbidden`, `:204 rank`); the hub worktree has moved since, so the drift is the trees,
not the parts. Task 116s guards are strong: `pg_advisory_xact_lock` on the cycle trigger closes
the two-concurrent-inserts race the scope did not mention, `work_read_cursors` gets a second
`hub.user_id` policy, and `guard_work_w4_actor()` extends SW.3 to every new table. Task 117s
`workAccess` is default-deny (`roleGranted` ranks 0, `Write` refuses it) and its feed clause is a
kind allowlist, not a project widening. Tasks 118-120 hold the OD-27 line: no aggregate health, no
"N of M", percentiles as pointers.

### S12-I4 (Important): the granted-principal matrix expects 200 on initiative routes the handlers 404
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-116.md:418
**Problem:** `grantedExpect` defaults **every** Work `GET` that is not under `/v1/teams/{teamId}`
to 200 for G, a member of a project-granted team. Initiatives are team-scoped, not project-scoped:
task 118s `readableInitiative` admits the owning team or a *spanned* team only and answers 404
otherwise (`task-118.md:147-173`), and task 119s `listInitiativeUpdates` follows it
(`task-119.md:288-290`). `GET /v1/initiatives/{initiativeId}` and `GET
/v1/initiatives/{initiativeId}/updates` therefore land in `TestWorkGrantedPrincipal` with `want`
200 and a handler that returns 404. Task 117 registers its override for `GET
/v1/projects/{projectId}/grants` (`task-117.md:378`); tasks 118 and 119 register none. Either the
meta-test fails and someone "fixes" the handler — a cross-team initiative read, exactly what Q12
did not grant — or the suite is amended late and unreviewed.
**How to fix:** add `grantedRowFor` overrides (404) for the two initiative route families in tasks
118 and 119, or make `grantedExpect` enumerate the granted-readable patterns (issues, comments,
features, milestones, feature updates) instead of defaulting `GET` to 200.

### S12-I5 (Important): the fake and the server disagree on who may add a feature dependency
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-115.md:625
**Problem:** the fakes `AddFeatureDependency` requires `RoleMember` on the `from` feature only and
accepts any `blocks` feature the caller can *see*, which after decision 2 includes a feature reached
through a project grant; a self-edge answers 400 `bad_request`. Task 120s handler requires both
features readable **as a member of the same team**, refuses a granted caller with 403, and answers
**422** on a self-edge (`task-120.md:42-46`). These are the rows task 124 runs against the real Hub,
so two of them cannot both pass, and a desktop surface built on the fake (129) will offer a link the
Hub refuses. Task 118 spotted the reach half (its gap 3); the 400-versus-422 half is unspotted.
**How to fix:** take task 120s rule into the fake (both features, same team, member; 422 on a
self-edge) and amend row `W4-05`, or change 120 and say so in 05 §5.4; either way one part owns it
before task 124.

### S12-M3 (Minor): two parts carry more than three hours of work
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-114.md:3
**Problem:** task 114 declares fifteen record kinds, their requests and views, nineteen goldens,
twenty-nine interface methods, sixteen event kinds, seven codes, a status map and two stub files at
3 hours; task 116 lays twelve tables with row-level security, four trigger functions, the granted
fixture and the meta-test at 3 hours. Both are above the 1-3 hour bar even with complete source
given.
**How to fix:** split 114 into wire-types and interface-plus-stubs, and 116 into tables-and-RLS and
guards-and-matrix; the seam freeze still happens at the end of the pair.

## Coverage and completeness

All ten parts exist and none is cut off: each ends with its *Gaps* or *Gaps for the skeleton and
siblings* section (111-120, 334-1140 lines each). Every scope line item has a home: 111s three
facts, proposal and `issue.file`; 112s loopback verbs and the spike; 113s eighteen rows; 114s
fifteen record kinds and twenty-nine methods (plus `MarkSeen`, declared early so the seam does not
grow); 115s nine fake rules as fourteen conformance rows; 116s twelve tables, four guards and the
`G` principal; 117s three grant routes and the span `PUT`; 118s five initiative routes; 119s five
update routes; 120s eight milestone, dependency and template routes. House style holds throughout:
complete fenced source, tests first with a named mutant per row, steps, acceptance commands with
expected output, and *What this task does not claim*.

Not found, having looked: a stored prediction field anywhere in W4; an agent principal that can
post a status update (119 refuses it, `RequireDesktop` plus a `user` principal); a grant that writes;
a brief that crosses teams without the contract proposal (117 decision 7); intake from outside the
org in this range (W5, tasks 143+); a local-mode boundary crossed by 112 (loopback twice, token in
the environment only, never in `sessionJSON`).

Section verdict: Needs fixes
# Section 13 — adversarial review of Phase W tasks 121-130

Reviewer pass over `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-121.md` to `task-130.md`, against the scopes in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-121-140.md` (lines 32-318) and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` (rows 536-545).

All ten parts exist and none is truncated (each ends with a "Does NOT claim" section).

## Findings

### S13-I1 (Important): task 121 drops the scope's open end — a recurring issue and the provenance thread

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-121.md:214
**Problem:** the scope for 121 requires the recurring create land "into `triage` (on the thread only
if the template pre-fills `serves`, else the person picks the thread state there; an open end, not a
silent one, §2.3 point 4)" (`…-scopes/tasks-121-140.md:47-49`). A grep of the whole part for
`serves` and for `thread` finds nothing. `passRecurring`'s contract says only `state "triage"` and
"task 78's prefill copy". So the part never decides whether a janitor-made issue joins the
provenance thread, never says what the surface shows when it does not, and has no test for either.
This is the one place in 121 that touches 05 §2.3's thread rule, and it is the part's job to close
it, not 78's: only this task creates an issue with no person present to pick.
**How to fix:** add a decision 8 — the created issue copies the template's `serves` when it has one
and carries no `serves` otherwise, in which case the issue is *off the thread* and reads so on its
own row (05 §2.3 point 4's open end); add `template.schedule_ran.data.onThread` (a bool) to G2's
table; and add a test pair to the integration table: `TestARecurringIssueCopiesServes` (template
pre-fills `serves` → the issue serves the same criterion; mutant: "the thread lost at creation") and
`TestARecurringIssueWithNoServesIsOpenlyOffThread` (no prefill → `serves` empty and the event says
so; mutant: "a silent end").

### S13-I2 (Important): `RunJanitorOnce` is in an `export_test.go` file, so task 124's `hub_test` end to end cannot call it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-121.md:245
**Problem:** 121 puts `RunJanitorOnce` in `control-plane/internal/hub/server/janitor_w4_export_test.go`,
"a wrapper in a `_test.go` file of package `server`, so it never ships in the binary" (task-121.md:29-31,
:243-256). Go compiles `_test.go` files only into that package's own test binary; they are not part of
the importable package. Task 124's end to end is `package hub_test` (task-124.md:741) and calls
`server.RunJanitorOnce(ctx, clk.Now())` (task-124.md:802). That does not compile: `undefined:
server.RunJanitorOnce`. Both parts note the tension as prose — 121 says "That is a note for 124", 124
marks it "UNVERIFIED until task 121's part" — but neither resolves it, so 124's acceptance command
fails at build, not at assertion.
**How to fix:** make it a build-tagged non-test file, `server/janitor_hook.go` with
`//go:build integration` (the same tag 124's e2e already carries), holding
`func RunJanitorOnce(ctx context.Context, st *store.Store, blobs blobstore.Store, now time.Time) (SweepReport, error)`;
it is then exported, importable from `hub_test`, and still absent from the shipped binary. Keep the
`export_test.go` file only if 121's own tests need something 124 does not. State the resolution in
both parts' *Gaps*, and add the arguments 124 must pass (`st`, `blobs`) to 124's call site.

### S13-I3 (Important): task 122's `count` measure counts only closed issues, which empties five of its seven groupings and narrows L-150

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-122.md:20-23
**Problem:** decision 1 fixes the window event for `count` as "the **standing done** … on an issue
whose head state is now `done`", and `windowAndDuration` returns `ok: false` for every issue that is
not done (task-122.md:135-141). So `measure: "count"` is a count of **completed** issues only.
Decision 3 then offers `groupBy` over `state`, `assignee`, `label`, `feature`, `cycle`, `priority`,
`criterion`: under `count`, `state` can only ever return `done` and `shipped`, and every other
grouping silently omits all open work. `01-linear-feature-inventory.md` L-150 is
`Insights: count, effort, cycle, lead, triage time | MATCH` and 05 §1 K:305 words it "Measured
durations and counts only" — a count of the team's issues, not of its closures. The part claims
"**Parity rows served:** L-150" on that reading. `TestAReopenedIssueHasNoDuration` even asserts a
reopened issue is "not in `count`", which locks the narrowing in.
**How to fix:** give `count` its own window event — `issue.created.at` — so the population is every
issue created in `[from, to)` in the reach, closed or not, and leave the three duration measures
exactly as decision 1 has them. Reword decision 1 to name two window rules, not one; change
`windowAndDuration`'s `MeasureCount` branch to return `(created, 0, true, false)`; add
`TestCountCountsOpenWorkToo` (three issues created in the window, one done → `count` groups total 3,
`groupBy: state` returns three keys; mutant: "count is closures"). If the owner in fact wants
closure counts, say so in *What this task does not claim* and stop claiming L-150 whole.

### S13-M1 (Minor): task 122's population guard is off by one between the store comment, the SQL and the route

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-122.md:224-229
**Problem:** `WindowIssues`' comment says the read is "capped at limit+1 so the handler can refuse a
population above limit", but the fenced SQL ends `LIMIT $4` with `$4` bound to the `limit` argument,
and the route passes "`WindowIssues` with `limit 5000` (5,001 ids → 422)" (task-122.md:256-258). As
written the query returns at most 5,000 rows, so the 5,001st never appears and
`TestARangeIsBounded`'s `too_many_issues` row can never go green.
**How to fix:** make the SQL `LIMIT $4 + 1` and say in the comment that the handler refuses when
`len(ids) > limit`, or change the route to pass `5001` and compare against 5,000. One of the two, in
the fenced source, since the part claims it complete.

### S13-M2 (Minor): task 122 leaves `IssueFacts`'s package undecided, so its "complete" source does not compile

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-122.md:236-240
**Problem:** `IssueFactsOf` is declared in `package store` returning `[]server.IssueFacts`, and the
prose then says "the build puts `IssueFacts` and `Move` in a small shared package or in `store`
itself and `server` aliases them. Which of the two is a build-time choice". The house style asks each
part for complete source for every interface and type; an import cycle left to the implementer is the
kind of choice two sibling parts resolve differently (task 121's store types sit in `store` with no
`server` reference at all).
**How to fix:** decide it here — put `Move` and `IssueFacts` in `store` (they are rows the store
read), have `server.ComputeInsights` take `[]store.IssueFacts`, and delete the alias sentence. `store`
already carries row types in 121 (`ScheduleRow`, `ReminderRow`, `RotationRow`), so this matches the
sibling.

### S13-C1 (Critical): three spellings of the same W4 event kinds across 114, 121 and 124; task 124's end to end asserts nine kinds that no task emits

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-124.md:764-784
**Problem:** task 114 (written, on the tree) declares the W4 kinds as constants with a `work.`
prefix: `work.grant_added`, `work.grant_revoked`, `work.initiative_created`,
`work.initiative_teams_set`, `work.feature_initiative_set`, `work.update_posted`,
`work.milestone_created`, `work.dependency_added`, `work.template_schedule_set`,
`work.template_schedule_clear`, `work.reminder_policy_set`, `work.triage_rotation_set`
(`task-114.md:750-765`). Task 124's end-to-end story asserts the kinds each step appends, and its
column uses a different spelling for nine of its ten: `grant.created`, `grant.revoked`,
`initiative.created`, `initiative.teams_set`, `feature.updated`, `milestone.created`,
`feature_dep.added`, `reminder_policy.set`, `template_schedule.set`. Only `work.update_posted`
(step 11) matches. Task 121's G2 table is a **third** spelling — `template.schedule_set`,
`template.schedule_cleared`, `reminder.policy_set`, `triage.rotation_set` — and its prose claims
"the seven kinds, **none in T114 at this writing**" (task-121.md:110), which is false for four of
them. Since 124's `roadmapStep.kinds` is compared against the Hub's feed ("a step whose kinds differ
from the Hub's fails with both lists printed"), 124's own acceptance cannot go green, and task 125's
replica — which copies kinds "by name from task 115's fake, never retyped" — will learn whichever
spelling 115 chose, a fourth chance to diverge.
**How to fix:** make task 114's constants the single source and cite them by name, not by string, in
every sibling. In 121: delete the "none in T114" sentence, rewrite G2 to add only the three kinds
114 genuinely lacks — the janitor's `work.template_schedule_ran`, `work.update_reminder`,
`work.triage_handover` — and reference `hubapi.EventWorkTemplateScheduleSet` and friends for the
rest. In 124: replace the `kinds` column strings with the `hubapi.EventWork*` identifiers so a
rename is a compile error, not a red test. Add to 124's test list
`TestEveryStoryKindIsAHubapiConstant` (every value in every `roadmapStep.kinds` is in
`hubapi.WorkEventKinds`; mutant: "a kind typed as a string").

Corroboration, read on the tree: `task-125.md:431-438` lists `"initiative.created"`,
`"feature_dep.added"`, `"grant.created"`, `"grant.revoked"`, `"update.reminder"` and
`"triage.handover"` as bare literals, while task 115's fake writes `work.`-prefixed kinds and greps
its own feed with the prefix `"work."` (`task-115.md:413`, `:668`, `:845`). Task 129's gap 3 names
the same rot from the other end: "The dependency event kind has three names" (`task-129.md:558-561`).
Two of the five parts that name kinds agree with 114; three do not.

### S13-I4 (Important): every line citation task 124 makes into task 114 points at the wrong lines

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-124.md:88-91
**Problem:** 124 says "the 31-method `WorkRoadmap` interface (`task-114.md:657-703`), its route
table (`:715-738`) and its codes (`:633-645`)", and later "task 114's † paths
(`task-114.md:724,727`)". Read on the tree today, `task-114.md:657-703` is the middle of
`Validate` bodies, `:715-738` is more validation, `:633-645` is `validName`/`validDate` switches,
and `:724`/`:727` are a bare `default:` and a `}`. The real anchors are: the codes at
`task-114.md:791-794`, the `WorkRoadmap` interface at `:808`, the route table with the † rows at
`:886`. Rule 4 asks that every citation be verified against the real tree; a reader following these
lands on nothing, and the next agent cannot tell a moved anchor from an invented one.
**How to fix:** re-grep and rewrite the four citations (`:791-794`, `:808-…`, the route table's real
span, `:886` for `SetMilestoneCriteria †`), and add a line to 124's Step 1 that re-reads task 114 at
the sync base and follows any move, as task 121's Step 1 does for `H24:66-112`.

### S13-I5 (Important): task 124's insights step contradicts task 122 — `count` yields no points

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-124.md:782
**Problem:** story step 18 is `QueryInsights(Checkout, count, closed in range)` expecting "one
group, `EP-1` its one point". Task 122 states the opposite in its fenced source and its prose: "The
`count` measure keeps its points empty (T114: *'empty for `count`'*), and its percentiles `nil`,
because `hasMs` is false for every issue it counts" (task-122.md:207-209), and
`ComputeInsights` appends to `Points` only when `hasMs` (task-122.md:167-170). So the step as
written fails against a correct 122.
**How to fix:** split step 18 into two assertions on one scenario — `measure: "count"` returns one
group of 1, `points: []` and three `null` percentiles; `measure: "cycle_time"` over the same range
returns one point for `EP-1` with `p50 = p75 = p90 = ms`. That also exercises the two window rules
S13-I3 asks 122 to separate.

### S13-I6 (Important): the timeline and the milestone list disagree on *reached*; task 128 ignored the rule task 130 exported for it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-128.md:387-389
**Problem:** task 130 decides that *reached* is `shownReached(view, head)` — "`view.reached` **and**
every `criterionId` is in the head brief" — because a criterion that left the brief must keep a
milestone unreached (task-130.md:41-47, its own test "a criterion removed from the brief does not
make it reached"). It exports `shownReached` and `MilestoneGlyph` for exactly this reason and names
the risk: "Task 128 should draw its marks with `shownReached` and `MilestoneGlyph`. Its scope reads
*reached* from `MilestoneView` alone … which shows a milestone holding a criterion off the head
brief as reached" (task-130.md:394-397). Task 128 was written **after** 130 and did not take it: it
renders `data-reached={m.reached}` and the accessible name `${m.reached ? "reached" : "not
reached"}` straight from the view, and a grep of the whole part for `shownReached` and
`MilestoneGlyph` finds nothing. So one milestone reads *reached* on the timeline and *not reached*
in the Criteria tab, on the same screen, for the exact case 130's test pins. L-72 is claimed closed
by 130 "with task 120" while 128's surface contradicts it.
**How to fix:** in task 128, import `shownReached` and `MilestoneGlyph` from
`desktop/src/components/roadmap/MilestoneList.tsx` (or move both to a
`desktop/src/components/roadmap/milestoneReached.ts` that 128 and 130 share), pass the feature's
head brief revision into `TimelineProps.inputs`, and render the mark from `shownReached`. Add to
128's `Timeline.test.tsx`: `TestADriftedCriterionKeepsTheMarkUnreached` — a `MilestoneView` with
`reached: true` and one `criterionId` absent from the head brief renders the hollow glyph and
`aria-label` "not reached" (mutant: "the timeline trusts `view.reached`"). Add 128 to 130's
dependency line, or make the shared module a task-125 export so neither depends on the other.

### S13-I7 (Important): `update.reminder`'s `data` has two shapes, and task 127's is a nested object inside a scalars-only event

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-127.md:523-524
**Problem:** task 121 fixes the event as `update.reminder`, subject `ft-` or `in-`, `data`
`{subjectKind, lead, daysSince, by}`, under task 13's rule that event data is "scalars only"
(task-121.md:104-108, :113). Task 127's reminder test feeds the Desk
`{ kind: "update.reminder", data: { subject: { kind: "feature", id: "ft-ep" }, lead: "u-a",
daysSince: 9 } }` — a nested object where 121 has a scalar `subjectKind`, no `by`, and the subject
moved out of the event's `subject` field into `data`. Task 127's `reminders.ts` and its
`registerDeskRow` row are built on that shape, so the row that 121's janitor actually appends will
not render, and the nested `subject` breaks the scalars-only rule the whole feed depends on.
**How to fix:** pick 121's shape (it is the one the Hub writes and the one that satisfies task 13):
change 127's fixture and `reminders.ts` to read `e.subject` for the id, `e.data.subjectKind` for the
kind, and carry `by` so the row can say whose rule asked. Add to 127's test list
`TestTheReminderRowReadsTheHubsShape` — the fixture is built from task 121's G2 table, by the same
constant name S13-C1 asks for, not hand-typed (mutant: "a desk row built on a shape the Hub never
sends").

### S13-M3 (Minor): task 126 claims L-67's W4 half while its own gap 2 leaves a team admin unable to name the team they would grant to

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-126.md:420-424
**Problem:** the part's headline claim is "Closes **L-67** (its W4 half, with task 117)"
(task-126.md:9-10), and its own gap 2 says no route lists the org's teams, so `otherTeams` falls back
to "the teams the caller belongs to other than the owner". Cross-team sharing whose picker can only
offer teams you are already in is not the capability L-67 names, and the part's copy example
("Payments can read and comment on everything in EP") is a team the granting admin need not be in.
**How to fix:** make the claim conditional — "closes L-67's W4 half **once** H or task 117 adds a
team directory read (gap 2); until then the picker offers only the caller's other teams" — and put
gap 2 in the skeleton's gap list against task 117, with the route it needs
(`GET /v1/teams?directory=true`, names and ids only).

### S13-M4 (Minor): tasks 128, 129 and 130 declare parts missing that are now on the tree, and 128's `Feature` shape no longer matches task 83

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-128.md:750-752
**Problem:** 128 says "No parts exist for tasks 53, 54, 72, 77 or 83", 129 says "No part for tasks
128, 83, 59, 55 or 89" and 130 says "No part for tasks 85, 55 or 128"; `task-83.md`, `task-55.md` and
`task-128.md` are all on the tree now. That matters beyond tidiness: task 83 types `leadId: string |
null` and `targetDate: string | null` (task-83.md:116, :118), while 128's fixtures use `lead: PRIYA`
(a `User` object) and its gap 3 says "`Feature.lead` is **required** … either 72 makes `lead`
nullable or the group is dead code" — 83 already made it nullable, so gap 3 is closed and the *No
lead* group is live, but 128's own fixture shape is now wrong.
**How to fix:** re-read `task-83.md:110-125` and rewrite 128's `feat()` fixture and `TimelineProps`
to `leadId: string | null` with `nameOf(leadId)`; delete 128's gap 3 and say 83 resolved it; drop
the stale "no part exists" lines from 129 and 130 or date them ("as at 2026-09-26").

### S13-M5 (Minor): task 124 is one of the largest parts in the section at the smallest credible estimate

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-124.md:6
**Problem:** 124 estimates 0.375 (3 h) for 31 client methods, 31 relay handlers plus
`roadmapCalls`, `scopeParam` and `queryLimit`, `verifyPins`, the audit-catalog rows, task 115's
conformance rows run against the real Hub, and a 21-step end-to-end story on `compose.yaml` with its
own `roadmapWorld` harness and three `devissuer` users. That is the widest surface in tasks 121-130
(its part is 921 lines, against 130's 402 at 2 h) and the only one whose acceptance needs a live
compose stack.
**How to fix:** split at the seam the part already draws: 124a — the `hubclient` methods, the route
table, `verifyPins` and `methods_work_roadmap_test.go` (the recording-`httptest` tests); 124b — the
relay rows, the audit catalog and `work_roadmap_test.go`; 124c — `TestWorkRoadmapE2E` and the
conformance run. Give each 1.5 h and renumber the skeleton's dependency column (125 needs only
124a's table).

Section verdict: Needs fixes
# Section 14 — adversarial review of Phase W tasks 131-140

Reviewer pass over `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-131.md` to `task-140.md`, against the scopes in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-121-140.md` (lines 319-620) and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

All ten parts exist. Line counts: 131/988, 132/441, 133/495, 134/635, 135/800, 136/1181, 137/844, 138/1063, 139/1068, 140/851.

## Findings

### S14-I1 (Important): the release-notes route reads Hub issue heads with no caller scope, so another team's titles can reach a reader

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-131.md:205
**Problem:** `Heads func(ctx context.Context, ids []string) (map[string]IssueFacts, error)` takes ids only. Nothing in the part carries the caller through to that Hub read: grepping `readable|caller|membership|unreadable` over task-131.md matches once, on an unreadable *file*. A release range is whatever the commits carry, so `Panacea-Issue:` trailers can name issues of a team the caller cannot read (task 14's readability is the rule everywhere else; task 137 states it explicitly for its own candidates). `GET {P}/notes` would answer with those titles as `text·H`, and `notesMarkdown` copies them out of the product. `GET {P}/shipped` has the same hole: it answers for any `is-` id with no readability test at all.
**How to fix:** give `Heads` the caller (`Heads(ctx, caller, ids)`) and answer only issues the caller can read; render the rest as the bare alias with no title, counted (*"3 issues you cannot see"*). Scope `GET {P}/shipped` to a readable issue, 404 otherwise. Add a test row: an issue of a team the caller is not in is in the range, and its title is absent from both the JSON and the Markdown (kills "release notes as a cross-team title dump").

### S14-M1 (Minor): "the four reads" declares five

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-131.md:196
**Problem:** the comment reads *"The four reads Build makes"*, then declares `Walk`, `States`, `Criteria`, `Heads` and `RecordIssues`.
**How to fix:** say five, or drop the count.

### S14-M2 (Minor): the "insights on the rail" test skips itself when a path is wrong, so it can pass without checking anything

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-132.md:350
**Problem:** `if (!existsSync(path)) continue;` is there for `Desk.tsx` (task 62, not landed), but it covers `Rail.tsx` too. If `rail/Rail.tsx` is ever renamed or moved, the loop skips both files and the test is green with nothing asserted — the same skip-into-PASS shape the phase 2 verification work just removed (`1e4fb7ff`). The scope names this mutant explicitly ("kills 'insights on the rail'").
**How to fix:** assert `Rail.tsx` exists (`expect(existsSync(railPath)).toBe(true)`) and keep the skip only for `Desk.tsx`; better, count the files actually read and assert it is at least 1.

### S14-M3 (Minor): the counts table, half the panel, has no test

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-132.md:288
**Problem:** all eight tests drive `InsightsScatter` or `InsightsQueryBar`; `result(...)` always sets `groups: []`. Decision 3 carries real failure modes nothing kills: a `none` key must read *No assignee* rather than `none`, and a total row must never appear because an issue in two groups is counted in each (`task-122.md:29-35`), so a total would lie.
**How to fix:** add one test over `InsightsPanel` with two groups including `none`: the resolved names render, the numbers carry `tabular-nums`, and no cell is the sum of the others (kills "a total that double-counts" and "a raw key shown as a group name").

### S14-I2 (Important): "named in" involvement rests on an undeclared subject for `comment.mentioned`, and nothing fails when the guess is wrong

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-133.md:126
**Problem:** `classify` decides a mention involves the person with `e.kind === "comment.mentioned" && e.subject === inv.meId`. Task 13 never declares what `Subject` holds for a comment kind: `CommentMentionedData` is `{CommentID, IssueID}` (`task-13.md:195-198`), `comment.mentioned` is `ScopeProject, VisibleToReaders` (`task-13.md:265`), and `emitWork(tx, kind, teamID, projectID, subject, data)` (`task-13.md:332`) only ever pins subject for `subscription.set` (`:393`). Task 133's own line 113, `e.kind.startsWith("comment.") ? d.issueId : e.subject`, says subject is *not* the issue for comment kinds but does not say what it is. If task 13 emits the issue, or the comment, or the author, `e.subject === inv.meId` is never true, decision 1's "named in" rule silently never fires, and the band drops every mention. No test catches it: `since.test.ts` builds its own `FeedEvent` fixtures under the same assumption. The part names this as gap 3 and does-not-claim, which is honest, but the plan still ships a rule that cannot be checked.
**How to fix:** make it an explicit declared amendment on task 13 — either `CommentMentionedData` gains `UserID` (the mentioned user) and `classify` reads that, or task 13's kind table states `comment.mentioned`'s subject is the mentioned user and adds a Go test asserting it. Then add a `since` row: a mention of the person on an issue they neither own nor review puts that issue in *waiting* (kills "mentions silently dropped").

### S14-M4 (Minor): "Task 62 has no part" is false, and the part reasons from that

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-133.md:71
**Problem:** the part says *"There is **no part for task 62**"* and repeats it as gap 1 at :488. `task-62.md` exists on this tree (42,654 bytes, written 2026-09-26 17:36), it owns `desktop/src/components/desk/deskModel.ts`, defines `DeskBand = "horizon" | "since"` (`task-62.md:124`), `registerDeskRow(kind, place, render, rank)` (`:198`), builds the `since` group (`:241-243`) and already re-exports the registry from `deskRows.tsx` (`:411-412`) precisely to settle the `deskFrame` / `registry` split. The import task 133 chose is the right one, so nothing breaks — but the stated reason is wrong and gap 1 is already closed.
**How to fix:** replace the claim with a citation to `task-62.md:124, :198, :411`, and drop gap 1 except for the `DeskSnapshot.since` member if task 62 still lacks it.

### S14-I3 (Important): the rotation's handover date has two names across the wire, so the duty line reads `undefined`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-134.md:132
**Problem:** verified on the tree: task 114's Go struct is `NextHandoverOn string \`json:"nextHandoverOn"\`` (`task-114.md:285`); task 125's TypeScript is `TriageRotationView { rotation, onDuty, handoverOn }` (`task-125.md:154`). Task 134 renders `until ${dayWords(v.handoverOn)}` and its `set` fake returns `handoverOn`, so against the real Hub the field is `undefined` and the line reads *"until Invalid Date"* or throws in `dayWords`. Task 134's own tests pass because its fake supplies `handoverOn`, so the defect survives every test in this part. The part names it as gap 5 but nothing in the plan assigns the fix.
**How to fix:** pick one name in the skeleton — `nextHandoverOn` reads better against a date and matches the Go — and amend the loser's part. Add a golden-shape row to task 125 that decodes a literal Hub JSON body (not a hand-built fake) into `TriageRotationView`, so a renamed field fails there instead of in the UI.

### S14-I4 (Important): task 121 writes `source: "recurring"`, which is not in the `IssueSource` union

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-134.md:114
**Problem:** `IssueSource = "person" | "orchestrator" | "intake"` (`task-37.md:97`, verified), and task 134's `originLine(head, schedule)` returns a line only when `head.source === "recurring"`. Task 134's acceptance admits the hole in a comment — *"exit 0 (gap 4's 'recurring' source is in task 37's union, or this fails at originLine)"* — which means the stated expected output is conditional on a change no task owns. On the Hub side the same value has to be accepted by task 121's writer and by W1's issue schema. The visible failure is the scope's own mutant surviving: a recurring issue shows no origin line at all, which reads as *created by nobody*.
**How to fix:** add `"recurring"` to `IssueSource` in task 37 (and to the Hub's allowed `source` values in the W1 migration) as a declared amendment, cited from task 121 and task 134; then make task 134's acceptance unconditional.

### S14-M5 (Minor): the part's own words disagree with the scope's on-duty sentence, and only the part records the choice

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-134.md:36
**Problem:** the scope writes *"On triage duty: Priya, until Tue 09:00 · rotation set by Sam"*; decision 3 changes it to *"until Tue 6 Oct"* because task 121 makes a boundary a local midnight, and gap 3 notes `setBy` does not exist on `TriageRotationView` (confirmed: `task-125.md:154` has no `setBy`). So two of the three clauses in the scope's example cannot be produced, and the scope still reads as the specification.
**How to fix:** amend the scope line (or the skeleton row) to the date form, and either add `setBy: User` to tasks 114 and 125 or drop *"rotation set by Sam"* from the scope so the sentence the tests assert is the sentence the plan promises.

### S14-M6 (Minor): `0003` is claimed from phase 6's migration sequence with no guard against a collision

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-135.md:232
**Problem:** the part writes `control-plane/internal/binding/migrations/0003_key_spend_issue.sql` "after P6's `0002`" and records version 3. Phase 6 is a plan, not merged (the part says so at :10), so nothing stops phase 6 landing its own `0003_*.sql` first. Two files at version 3 in one directory is either a duplicate-version error at boot or, worse, a silently skipped migration on an estate that already recorded 3 — and the symptom is `issue_id` missing at runtime, seen only as a 503 on the spend line.
**How to fix:** state the rule in the part's step 1 (sync-merge phase 6, then renumber to the next free version and rename the file before writing it), and make task 05's runner test assert that migration versions are unique and gapless so the collision fails at `go test`, not at boot.

### S14-M7 (Minor): two acceptance greps pass when their path does not exist

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-135.md:764
**Problem:** `grep -n -E 'ScopeIssue|ScopeLoop' internal/budget/*.go` with `# expect: no output` is satisfied both when the rule holds and when `internal/budget/` does not exist — which is its state on `feat/work-layer` today, since every phase 6 file is unmerged. The keysmith title grep at :762 has the same shape. Neither proves decision 1 or decision 5 on the tree the reviewer is standing in.
**How to fix:** front each with an existence assertion (`test -d internal/budget && grep ... ; echo "grep exit $?"`) and state the expected exit code, so "the file is not there" and "the rule holds" are different outputs.

### S14-I5 (Important): *Auto-apply* can overwrite a value a person just set, which is the one thing decision 12 promises it never does

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-136.md:105
**Problem:** decision 12 is titled *"never over a person"*, and its guard is that the property "is still empty on the replica's head". Two holes, both in the decision's own text. (1) The replica is the desktop's eventually-consistent copy, so "empty" can be minutes stale. (2) `PatchIssueRequest` is `{priority?, assigneeId?, labels?, notTiedReason?}` (`task-37.md:190-192`, verified) — no `baseRevision`, no precondition — so the write cannot be made conditional, and the part concedes "the Hub's last write wins". The failure: Priya sets priority `urgent`; the desktop's auto-apply, holding a snapshot from before her event arrived, writes `medium` **as Priya**. The event's actor is Priya, so the activity reads as Priya changing her own mind, and the DECIDED WITHOUT YOU row with Undo is the only trace — an assist quietly demoting a person's own priority under their own name is exactly the trust failure AX §7 exists to prevent.
**How to fix:** make auto-apply conditional. Either give `PatchIssueRequest` an optional `baseRevision` (or an `If-Match` on the head's revision) in task 08/task 37 and make a mismatch `409 revision_conflict`, or route every auto-apply through `writeIssueRevision`, which already carries one. Then add the row: with the head's revision ahead of the suggestion's, auto-apply writes nothing and says so (kills "auto-apply beats a person to the property").

### S14-I6 (Important): five tasks and roughly 14 hours of W4 deliver nothing outside a loopback fake, and no task closes the gap

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-136.md:61
**Problem:** decision 4 is explicit and honest: `DirectGateway` "is wired only when both upstreams are loopback addresses", and "every other deployment builds `Service` with `Gateway: nil`, and every assist answers `502 gateway_unavailable`". Tasks 137, 138, 139 and 140 all route through this one `Service` (`task-136.md:41`, decision 1), so the same holds for all five: the duplicate hints, the natural-language filter bar, the drafted update, the thread summary and both proposers are dark in any real deployment. The blocker is a single missing `SessionClass` on phase 6 (`task-03.md:386-392`: `solo`, `worker`, `orchestrator`), written up as gap 1 "a proposal to the phase 6 owner ... **does not apply it**". No task in the Phase W skeleton owns landing it, and the skeleton's rows 551-555 say only "136; P6 gateway", which reads as a satisfied dependency rather than an unresolved decision.
**How to fix:** add one W task before 136 that either (a) lands `ClassAssist` on phase 6's wire, keysmith and gatekeeper as a declared cross-phase amendment, or (b) records the owner's ruling that assists mint no key and run on a named control-plane credential with per-person attribution. Until one exists, mark rows 136-140 in the skeleton as *blocked on P6-D8* and say in W4's proof task (142) that no assist criterion can be proved outside local mode.

### S14-M8 (Minor): the org kill switch and the assists switch are the same bit, so turning off orchestrators silently turns off every assist

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-136.md:52
**Problem:** decision 3 reads `AgentSwitch.Orchestrators` (`task-100.md:314`) as *assists off*, and gap 3 notes SW.19 has no assists member. An admin stopping runaway orchestrators also, with no warning in the switch's own copy, removes duplicate detection, composer hints, the filter bar and every draft — and the only sign is *"Suggestions unavailable: assists are off for this org"* on a surface the admin is not looking at.
**How to fix:** add `assists bool` to `AgentSwitch` in task 99/100 with its own default, have `Complete` read that member, and make the switch's UI copy name both effects. If the owner wants one bit, say so in the switch's copy.

### S14-I7 (Important): L-55 is marked closed without the project layer it names, and the design contradicts itself about that layer

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-137.md:13
**Problem:** the part says *"Closes **L-54** ... **L-55** (layered guidance, most local wins) and **L-56**"*, then gap 1 says there is no project layer and *"the brief is the only layer below the team"*. Verified in the design: row 55 reads *"org → team → project guidance"* (`05-design.md:169`), while rows 158 and 183 read *"Org, team and personal guidance"* and *"org → team → person"* (`:320`, `:350`), and SW.19's built record is org / team / person (`task-109.md:176`). So the design names two different third layers for one feature, W3 built `person`, W4's L-55 asks for `project`, and task 137 closes L-55 with neither. Under OD-27 (proof per criterion) a parity row cannot be closed by a part that states it did not build the row's own content.
**How to fix:** get the owner's ruling on the third layer, then make the docs agree: either amend `05-design.md:169` to `org → team → person` and keep L-55 closed here, or add a Hub task for a project guidance layer on task 99's records and task 109's editor and move L-55's close to it. Either way, name the ruling in the part.

### S14-M9 (Minor): `TestProjectGuidanceWins` asserts a level the package does not define

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-137.md:693
**Problem:** the test is named for a project layer and its comment kills "org wins", but its body seeds `org` and `team` and asserts `LevelOrg, LevelTeam, LevelBrief`. `GuidanceLevel` has no project constant (`task-137.md:108-113`). A reader grepping for the project layer finds a green test with its name.
**How to fix:** rename to `TestTheMostLocalLayerWins` (or `TestTheBriefWinsBelowTheTeam`) so the name matches the assertion, and leave the project layer to the S14-I7 ruling.

### S14-M10 (Minor): eight of the eleven desktop tests are prose bullets, not fenced source

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-137.md:775
**Problem:** the acceptance counts *"DuplicateHint 4 passed; ComposerHints 4 passed; TriageView's rows plus 1 new"*, but only `hintClock.test.ts`'s three tests are written as code; the rest are three bullets of prose. The house style asks for the tests first, in full. The scope's own mutants "auto-close", AX anti-pattern 12 and `expectNoPrediction` on the composer therefore have no written assertion an implementer can copy, and "a candidate with `thatTitle: ''` renders nothing" has no stated mutant.
**How to fix:** expand the three bullets into fenced `it(...)` blocks with their mutant comments, in the shape tasks 131, 132 and 136 use.

### S14-I8 (Important): three of task 138's acceptance commands cannot run — they call `go test` from a directory with no module

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-138.md:1023
**Problem:** verified on the tree: there is no root `go.mod`, only `control-plane/go.mod`. Task 138's acceptance runs `go test ./control-plane/internal/workassist/ ...`, `go test ./control-plane/internal/api/ ...` and `go test ./control-plane/... -race` with no `cd`, so all three fail with *"go.mod file not found in current directory or any parent directory"* — not a red test, a broken command. Tasks 131, 135, 136 and 137 all `cd control-plane` first, so this part is the odd one out. Under OD-27 the acceptance block is the proof, and this one cannot produce any.
**How to fix:** prefix the three commands with `cd control-plane &&` and drop the `control-plane/` path segment, matching tasks 136 and 137. While there, the last grep should be `grep -c ... ; echo "exit $?"` or `! grep -q ...`, since `grep -c` printing `0` also exits 1.

### S14-M11 (Minor): "tasks 72, 83, 91 and 136 have no part on this tree" is now false for all four

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-138.md:1054
**Problem:** gap 1 and the part's opening paragraph (:6) both say the parts these names come from do not exist. `task-72.md`, `task-83.md`, `task-91.md` and `task-136.md` are all on the tree today. The part was written before them (its own note at :9 says so), so the claim is honest history, but a reviewer reading the gap list now cannot tell which UNVERIFIED names are still unverified — and the whole "UNVERIFIED names table" at :57-64 is checkable against real parts.
**How to fix:** on the reconcile pass, walk the table at :57-64 against `task-136.md` (`Service.Complete`, `Call`, `Reply`, `assistProblem`, `readableTeam`), `task-83.md` (`ViewFilter`, `ViewClause`) and `task-91.md` (`evaluateFilter`, `ViewEditor` props), mark each verified or fix the name, and cut gap 1 to whatever is genuinely left.

### S14-I9 (Important): tasks 139 and 140 repeat task 138's unrunnable `go test` acceptance

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-139.md:1027
**Problem:** same root cause as S14-I8, in two more parts. `go test ./control-plane/internal/workassist/... ./control-plane/internal/api/... -race -count=1` (task-139.md:1027 and task-140.md:812) runs from the repo root, which has no `go.mod` — only `control-plane/go.mod` exists. Both commands fail before compiling anything, so neither part has a working proof block, and the named tests (`TestErasedCommentNeverPrompted`, `TestCycleProposalHasNoForecastField` and the rest) are never actually invoked by the stated acceptance.
**How to fix:** `cd control-plane && go test ./internal/workassist/... ./internal/api/... -race -count=1` in both parts. Worth a one-line house rule in the skeleton so the remaining W4 parts do not repeat it.

### S14-I10 (Important): every "no part on this tree" claim in this section is now false, leaving ~40 load-bearing names marked UNVERIFIED that are checkable today

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-140.md:838
**Problem:** task 140's gap 2 says *"Tasks 87, 80, 72, 77, 83, 92 and 136 have no part on this tree"*; task 139's gap 2 says the same of 136; task 138's gap 1 of 72, 83, 91 and 136; task 137 of 68 and 11; task 136 of 64, 83, 84 and 99; task 134 of 59, 84 and 89; task 133 of 62, 63, 64 and 71. I checked all twenty: `task-56, 62, 68, 72, 77, 80, 83, 84, 87, 89, 91, 92, 99, 100, 109, 114, 121, 123, 124, 127` all exist in the parts directory. Every one of those claims was true when written and is false now. This is not cosmetic: each stale claim carries a block of names marked UNVERIFIED, and the parts say "the owning part wins at build". S14-I3 is the proof that the drift is real — `handoverOn` (125) against `nextHandoverOn` (114) is exactly the class of mismatch these markers hide, and it was only found by opening both files.
**How to fix:** one reconcile pass over tasks 133-140 before W4 starts: for each UNVERIFIED name, open the owning part, either confirm it with a `file:line` citation or fix the name here, and delete the "no part" gaps that have closed. Keep UNVERIFIED only for phase 6 and phase 7, which really are unmerged. Budget about 90 minutes.

### S14-M12 (Minor): the cycle-Undo test only proves the `null` inverse

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-140.md:783
**Problem:** decision 8 says the inverse of a `setCycle` is *"the issue's previous `cycleId`"*, but the test asserts three writes with `body.cycleId === null`. If the implementation hard-codes `null` instead of capturing the previous value, the test still passes, and an issue moved from Cycle 13 into the proposal loses Cycle 13 on Undo. The mutant it claims to kill is "partial undo", not "wrong inverse".
**How to fix:** seed one candidate already in Cycle 13 and assert its Undo write carries `cycleId: "cy-13"` while the other two carry `null` (kills "Undo clears instead of restoring").

## Coverage against the scope, part by part

| Part | Scope covered | Scope mutants written | Sizing | Note |
|---|---|---|---|---|
| 131 | yes, plus `IssueShipped.tsx`, `types.ts`, `ShipBar.tsx`, `GET {P}/shipped` beyond the scope's list | all ten, in a table and fenced tests | 3 h, fair | phase 7 line numbers re-derived and correct (verified 583-616 of `2026-09-23-phase-7-deploy.md`); freeze is honestly a fake until the Hub record lands |
| 132 | yes | 7 of 7, as fenced tests | 3 h, fair | `startOfDayIso` handles the 2026-10-04 Sydney DST boundary correctly |
| 133 | yes | 9 of 9 | 3 h, fair | the mention rule rests on an undeclared field (S14-I2) |
| 134 | yes | 6 of 6 (two as a table) | 2.5 h, fair | two blocking name mismatches (S14-I3, S14-I4) |
| 135 | yes | 6 of 6 | 2 h, fair | the strongest citation work in the section; every phase 6 line number checks out |
| 136 | yes | 10 of 10 | 3 h, fair | auto-apply race (S14-I5); the whole cluster is dark without P6 (S14-I6) |
| 137 | yes, minus the project guidance layer | 8 of 8, three in prose | 3 h, fair | L-55 closed without its third layer (S14-I7) |
| 138 | yes | 8 of 8 | 2.5 h, fair | acceptance cannot run (S14-I8); `@me` and ISO weeks handled well |
| 139 | yes | 9 of 9 | 3 h, fair | acceptance cannot run (S14-I9) |
| 140 | yes | 8 of 8 | 3 h, fair | acceptance cannot run (S14-I9) |

No part is missing and none is cut off: all ten end with a "What this task does NOT claim" section and a gap list. All ten carry fenced source for their interfaces, types, routes and migrations. Every phase 7 and phase 6 line citation I sampled resolved exactly; the stale citations are the intra-Phase-W "no part on this tree" claims (S14-I10) and task 131's own note that the scope's phase 7 line numbers are 7-8 lines out, which is correct.

Security read: Hub authorisation per verb is handled on tasks 136-140 (`readableTeam`, readable candidates only, readable briefs only, `df-` drafts 404 to another person) and on 135 (the issue read gates the spend read). Task 131 is the one hole (S14-I1). Agent edits as proposals holds everywhere except *Auto-apply*'s unconditional patch (S14-I5). The provenance thread is intact: `serves` is set by code, not by the model, on 131, 136 and 140. Intake from outside the org is out of scope for this section (W5). The local-mode boundary is respected — task 136 decision 4 is explicit that the real gateway is not wired.

Section verdict: Needs fixes
# Section 15 — adversarial review of Phase W tasks 141-150

Reviewer pass over `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-141.md` .. `task-150.md`, each read against its scope in `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-141-160.md` and the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.

## Task 141 — an agent's edit is a proposal, shown as a diff

Scope coverage is good: L-130, the `edit_proposal` kind, the three acts, stale handling, the
colour-free diff, the SW.7 byte-identical Go row, and five named gaps are all present. Citations
spot-checked and real: `task-47.md:151-172` (`lineDiff`), `task-46.md:520-527` (`IssueView` props),
`task-49.md:101-112` (`authorClass.ts`), `task-51.md:466` (the erased branch), `task-56.md:157-168`
(the `WorkOp` union, `revise` and `resolve`), `task-37.md:153-159` (`IssueComment`),
`task-19.md:55-56` (`render_test.go`).

### S15-I1 (Important): Accept fires `resolve` unconditionally, so a refused revision eats the proposal

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-141.md:358
**Problem:** `accept()` calls `mutate({kind:"revise",...})` and then `mutate({kind:"resolve",...})`
back to back, without waiting on the first. Task 56 rolls a `revise` back on `revision_conflict`
(`task-56.md:42`, `:381`) but the `resolve` is a separate op the Hub accepts, so the proposal
comment is resolved while nothing was written. `openProposals` filters resolved comments, so the
strip disappears and decision 4's promise ("a head that moves again during review is task 56's
ordinary 409 and a fresh stale line") cannot happen: there is no open proposal left to re-show.
The prose at `:400` weighs only the opposite ordering ("a resolve that the Hub refuses must not
roll back an accepted revision"), so this case is unconsidered rather than decided.
**How to fix:** sequence Accept — await the `revise` op's settle and resolve only on success; on
`revision_conflict` re-render the stale line against the new head with the person's kept words
(`task-56.md:328`). Add the mutant row: a `revise` refused with `revision_conflict` leaves the
proposal comment unresolved and the strip on screen (kills "a refused accept eats the proposal").

### S15-M1 (Minor): the `Comments.tsx` branch contradicts its own prose on erased proposals

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-141.md:419
**Problem:** the prose says "The branch comes first so an erased proposal also renders as the link
line's *erased* fact (task 51's erased line, same markup)", but the complete source has no
`erasedAt` branch: an erased, unresolved `edit_proposal` renders a live *Review* link, which calls
`focusProposalStrip()` on a strip that is not rendered, because `editProposalOf` returns null for
`c.erasedAt !== null` (`:157`). A dead control, and the house style asks for complete source.
**How to fix:** make the first line of the branch task 51's erased markup
(`task-51.md:466-468`), and add a test row: an erased proposal shows the erased fact and no
*Review* button.

### S15-M2 (Minor): the surface tests render `IssueView` without its required props

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-141.md:519
**Problem:** `renderWith(<IssueView issueId="is-142" route={null} {...extra} />, ...)` omits
`revision` and `me`, both required and non-optional in task 46's signature
(`task-46.md:520-527`). As written the tests do not typecheck, and step 7 runs
`npm --prefix desktop run typecheck`.
**How to fix:** pass `revision={null}` and a `me` user in the `view()` helper, or have
`renderWith` supply both.

## Task 142 — W4's proof and results

Strong part. I re-ran decision 2's selection against the real matrix
(`awk -F'|' '/^\| [0-9]+ \|/ && $(NF-1) ~ /W4/'` over
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md`) and it returns exactly the
forty-one rows the scope names, in the scope's order; `$(NF-1)` is the slice column (NF=7) and it
correctly picks up the split cells `W2 / W4` (L-67, `:191`), `exists / W4` (L-82 `:206`, L-253
`:445`) and `W1 / W4` (L-101, `:235`). L-255's slice really is `phase 6` (`:447`), so decision 3's
carried row is right, and 41 + 1 = the 42 the acceptance line expects. `check-w4.sh`'s field
indices line up with the template's six-column rows table. No Critical or Important finding.

### S15-M3 (Minor): the sweep omits task 131's `IssueShipped` surface, so L-148's guard escapes it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-142.md:322
**Problem:** `W4_SURFACES` lists `deploy/ReleaseNotes.test.tsx` and `deploy/FreezeToggle.test.tsx`
but not `deploy/IssueShipped.test.tsx`, which task 131 does own (`task-131.md:85`, `:852`) and
which is the surface closing L-148 (*shipped to production at hh:mm*, this part's own row for
L-148). The sweep's stated purpose — every W4 surface calls `expectNoPrediction` — has a hole
exactly where a predicted ship time would most plausibly be written.
**How to fix:** add `["deploy/IssueShipped.test.tsx", 131]` to `W4_SURFACES` and change the
acceptance line to 26 tests.

### S15-M4 (Minor): decision 9 and gap 1 are stale — every part they call missing now exists

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-142.md:51
**Problem:** decision 9 and gap 1 say tasks 128 and 135-137 have no part on this tree and 134's is
"73 lines, no tests yet". All are now written: `task-128.md` 754 lines, `task-134.md` 635,
`task-135.md` 800, `task-136.md` 1181, `task-137.md` 844, `task-138.md` 1063. A reader takes the
`(scope)` cells and `// scope` comments as still-open work.
**How to fix:** re-run step 1 now and replace the `(scope)` cells with the real test names. I
checked the guessed sweep paths against those parts and every one matches
(`components/roadmap/Timeline.test.tsx` `task-128.md`; `plan/RepeatField`,
`plan/TriageRotationSettings`, `work/MethodPage` `task-134.md`; `issue/IssueHead` `task-135.md`;
`work/AssistSuggestion`, `work/AssistDial` `task-136.md:132-133`; `work/DuplicateHint`,
`work/ComposerHints` `task-137.md:95-96`), so only the prose and the `(scope)` cells need
updating, not the paths.

## Task 143 — `hubapi` and `hubclient.WorkIntake`: the W5 wire

Covers SW.21 well and its ten decisions are the right kind of ruling: the route owner's Go wins
(decision 1), the scope's `tracker` channel is refused with a reason (decision 3), `lr-` and `mc-`
are dropped with reasons (decisions 4 and 6), and `HarnessAllow`+`TeamSkill` become
`TeamAgentPolicy` (decision 5). Citations checked: task 02's prefix block really is dash-free
(`task-02.md:155-174`), `task-146.md:193-202` really declares `member|form|email|teams`,
`task-15.md:177` really is `var ImportSources = []string{ImportSourceLinear, ImportSourceCSV}` read
at `:207` inside `ValidateImportBatch`. Security reads right: only three `Created*` types carry a
secret, the webhook URL must be https with no userinfo, an agent-authored quote is refused, and
`principal_cannot_move_state` is a 403.

### S15-M5 (Minor): the Files table says eleven W5 codes; the block declares twelve

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-143.md:86
**Problem:** the `codes.go` ⚑ row reads "appended: eleven W5 codes", but the complete block at
`:770-784` declares twelve (`request_closed`, `request_erased`, `form_field_required`,
`tracker_exists`, `principal_cannot_move_state`, `webhook_url_invalid`, `webhook_not_found`,
`app_not_approved`, `key_revoked`, `connector_not_allowed`, `harness_not_allowed`,
`loop_outside_grid`), and `WorkIntakeCodeStatus` and `TestEachW5CodeMapsToItsStatus` both carry
twelve. The scope named ten; decision 7 adds `webhook_not_found` and task 146 adds
`request_erased`, so twelve is right and the table is wrong.
**How to fix:** change the Files row to "appended: twelve W5 codes (the scope's ten, plus
`request_erased` from task 146 and `webhook_not_found` from decision 7)".

### S15-M6 (Minor): extending `ImportSources` from `init()` is initialisation-order fragile

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-143.md:124
**Problem:** `func init() { ImportSources = append(ImportSources, …) }` runs *after* every
package-level `var` in `hubapi`, including in `_test.go` files. Any present or future
`var x = something(ImportSources)` — a set, a golden, a joined string — captures `linear, csv`
only, and the bug is silent. The part chooses `init()` to stay additive under rule H1, but
`hubapi/work.go` is already ⚑ in this same task, so there is no additive-only reason to avoid the
literal.
**How to fix:** either edit task 15's literal in place (the file is already modified here), or make
it a function `ImportSources()` returning the built slice, so nothing can read it half-built. Keep
`TestImportBatchAcceptsW5Sources` either way and add a row that `len(ImportSources) == 5`.

### S15-M7 (Minor): gap 3's "no part on this tree" list is stale

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-143.md:1150
**Problem:** gap 3 names tasks 150, 154, 159, 160, 162, 163 and 166 as having no part. All seven
now exist (`task-150.md` 1214 lines, `154` 876, `159` 339, `160` 568, `162` 505, `163` 422, `166`
631), so "their routes are proposed" understates what can be checked before step 1.
**How to fix:** re-read those parts at step 1 and either confirm the mirrored types or record the
differences the way decision 1 records tasks 146-161's.

## Task 144 — the fake's W5 rules and conformance rows

Covers the scope's rule list row for row (eighteen rows against the scope's ~sixteen rules, with
the extras earned), and the file-name collision with task 05's `fake_work_intake.go`
(`task-05.md:76`, `:961`) is caught and corrected to `fake_work_w5.go`. I counted task 143's
`WorkIntake`: exactly fifty-one methods, so the Files table's "all fifty-one" is right. The
"Expected, not observed" note under Acceptance is the OD-27 discipline done properly.

### S15-C1 (Critical): SW.21 is frozen around `ApplySyncBatch`, which task 150 says has no route and no caller

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-144.md:585
**Problem:** four of the eighteen conformance rows — W5-08 `sync_only_by_its_principal`, W5-09
`sync_idempotent_by_external_id`, W5-10 `sync_principal_moves_nothing`, W5-11
`sync_comments_by_mode` — are driven through `WorkIntake.ApplySyncBatch` (decisions 4 and 10),
which task 143 froze into SW.21 (`task-143.md:828`, the 51-method interface). Task 150, the owner
of sync, withdraws it: *"`ApplySyncBatch` and `POST …/batches` have no route and no caller
(decision 1) and should be withdrawn, with task 144's fake rule for them"*
(`task-150.md:1192-1195`). Its engine pulls on a janitor transaction with `actor: null` and no
client call at all (`task-150.md:35-44`). So those four rows cannot run against the real Hub in
task 168 — the very thing the rows exist for — and the two sides of the seam disagree about
whether sync is a client method. This is not a rename: W5-10 is the row that proves "sync never
writes a state, a fact or `done`", the rule the whole slice rests on.
**How to fix:** pick one before 144 builds. Either (a) keep `ApplySyncBatch` as a real route,
integration-principal only, and have task 150's Sink call it — then 150's decisions 1 and 3 change;
or (b) drop it from SW.21 and re-express W5-08 to W5-11 against the janitor path, which means
`WorkEnv` gains a `RunSyncOnce(t, conn, page)` hook that the fake implements directly and task 168
drives through `RunJanitorOnce` (`task-150.md:1090`). Whichever wins, task 143's interface, task
144's rows and task 150's gap 1 must be updated in the same pass, and 143's "frozen after this
task" line needs the exception recorded.

### S15-I2 (Important): the fake attaches labels on a synced issue; task 150's SQL guard refuses them

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-144.md:91
**Problem:** decision 10 has `ApplySyncBatch` create through task 05's `insertImported`
(`task-05.md:1030-1041`), which matches labels from the import item. Task 150's guard refuses *any*
`issue_labels` insert while `hub.work_sync_connection` is set (`task-150.md:36-39`), and its own
row `TestSyncAttachesNoLabels` says in as many words that labels passed to `importOne` would make
"the guard fail every page". So a page that names a label passes against the fake and fails against
the Hub — exactly the divergence the shared conformance rows exist to prevent, and it will only
surface in task 168. Task 145 records the same conflict as its gap 2.
**How to fix:** have the fake's sync path strip `Labels` from every item before `insertImported`
(one line), and add a row to `WorkRowsW5`: a synced item naming an existing label creates the issue
with no labels (kills "the fake attaches what the Hub refuses").

## Task 145 — the W5 migration, its guards and matrix rows

The strongest security part of the section. Decision 6's host rule (three cause/state pairs, only
forward, only with `hub.work_actor_user` a user of the issue's org, `done` never a host's) and the
"an outside cause under any other kind raises" clause close the forge-a-host-move hole properly.
Decision 8 (`work_actor_consistent()`) is the right answer to two settings that can drift. Decision
10 keeps every secret a reference. Citations to `hub-contract.md` (`:1904`, `:2083-2085`) match the
file. Nine gaps, all real.

### S15-I3 (Important): app and key may make hand state moves here and in task 144, and may not in task 160

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-145.md:73
**Problem:** decision 5 lets an `app` or a `key` make a person's `hand` moves and refuses only the
four fact states and `done`, matching task 144's decision 3 and row W5-12. Task 160 decides the
opposite on the same scope sentence: its decision 7 says *"Task 145's guard refuses `key` every
state column write"* (`task-160.md:57`) and its test `TestKeysNeverMoveState` asserts
`backlog → todo refused` (`task-160.md:475`). Two of the three parts are in this section, and the
migration is the one that decides: whichever way 145's `guard_issue_state()` is written, one of
`TestKeysNeverMoveState` and `W5-12` fails on the day 160 builds. The part names this as its gap 1
and recommends 160 change, but nothing has changed it, so the plan still carries both.
**How to fix:** make the call in 145 (its recommendation, `backlog → todo` allowed for `app` and
`key`, is the one L-245 and L-250 need) and edit `task-160.md:57` and its test row in the same pass;
or take 160's reading and drop the `hand` allowance from 145 decision 5 and 144 decision 3. Leaving
the recommendation unactioned is the one outcome that breaks a build.

### S15-I4 (Important): four W5 route tasks need Key Vault and no Hub-delta task proposes it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-145.md:864
**Problem:** decision 10 puts every HMAC or signing secret behind a `secret_ref`, and gap 6 records
that the Hub process has no Key Vault access (`hub-contract.md:2083-2085`) so tasks 151, 152, 154
and 156 cannot read one. The brief for this phase says the Hub deltas are to be *"its own tasks,
with the same sync-merge rule as team mode"*, and the skeleton's task table has no W5 Key Vault
delta. So four route tasks are planned against a capability the plan never asks anyone to add, and
the W0 proposal file (`skeleton :320`) carries only W0's two contract edits.
**How to fix:** add a WH task — *"W5's Hub-contract proposal: Key Vault under the Hub's workload
identity for `secret_ref`"* — beside W0's proposal, owned by this slice, and make tasks 151, 152,
154 and 156 depend on it; or move signing to a secret the control plane holds and the Hub only
verifies a version of, and say so in 145 decision 10.

## Tasks 146, 147, 148 — requests, intake rules, intake forms

No Critical or Important finding in the three. They cover their scope lines and their design
sections, they size right (0.3 to 0.375), and the security reading is careful in each:

- **146** decision 2 reconciles 05 §3.5's *"its text never enters an agent's managed block"* with
  the scope's *"revision 1 whose description is the quote"* by ruling that the **record** never
  enters and the accepter adopts the words, and raises the stricter reading as owner question
  Q-146 rather than deciding it quietly. Decision 5 refuses a `projectId` outside the request's
  team *"whether or not the project exists"*, so the route is no oracle. Decision 7 keeps an
  erased request's state and `issue_id` so counts still add up. Decision 8 keeps `channel_ref`
  through erase, which is what stops a mail retry re-creating an erased request.
- **147** decision 4's `senderDomain` rule is stated at the right boundary (`client.example`
  matches `mail.client.example`, never `evilclient.example`) and `textContains` is a
  case-insensitive substring, *"never a regular expression, so no rule can be made slow"*.
  Decision 1's versioned document plus decision 2's `baseVersion` 409 is the right shape for
  "editing rules leaves already-routed requests as they were". Decision 6 takes membership from
  the Hub's own table, never the client's claim, with a named test.
- **148** decision 4 is a good page-security block: no script, `application/x-www-form-urlencoded`,
  a per-session CSRF token compared in constant time and checked *before the body is read
  further*, `HttpOnly` + `SameSite=Strict`, and a test that the page cookie is accepted by no
  `/v1` route. Decision 5 answers another org's principal with `404 form_not_found`, *"the same
  bytes as a form that does not exist"*. Decision 6 refuses the scope's promise and forbids a date,
  an estimate or a percentage with a test (`OD-27`).

### S15-M8 (Minor): five parts still open with "task 143 / 145 is not written at this writing"

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-146.md:21
**Problem:** tasks 146 (`:21`, gap 1), 147 (`:16`, gap 1), 148 (gap 4), 149 (gap 2) and 150 (gap 4)
each open by saying task 143 or 145 does not exist and that their block is a *requirement* on it.
Both now exist (`task-143.md` 1155 lines, `task-145.md` 868), and each has already adopted those
blocks by name — 143's decision 1 table cites `task-146.md:188-367`, `task-147.md:114-172`,
`task-148.md:117-198`, `task-149.md:274-317`; 145's decision 1 table cites `task-146.md:97-172`,
`task-147.md:79-103`, `task-148.md:77-109`, `task-149.md:98-170`, `task-150.md:90-187`. A reader
of 146-150 alone cannot tell that the requirement was met, or that 143 deleted two of their lines
(`task-146.md:191`'s `PrefixRequest`, `task-148.md:120`'s `PrefixForm`, `task-143.md:47-49`).
**How to fix:** in each of the five, replace the preamble and the gap with one line naming where
143 and 145 adopted the block, and drop the two prefix `const` lines 143 decision 2 removes, so no
builder writes a line that will not compile.

## Task 149 — mail intake and the *shipped* notice

Decision 3 is the right call and well argued: a mail's `From` is not authenticated, so a mail is
**always** an `email` requester and never a `person`, against the scope's *"a `person` when the
address matches an org user"* — with the unverified-lookup shown at read time instead, and the
promotion raised as owner question Q-149. Decision 2's global cursor outside the per-org loop,
under `hub_janitor` with no row-level security and no `hub_app` grant, is the correct shape for a
mailbox that is not an org's. Decision 4 guarantees a long mail always becomes a request.

### S15-I5 (Important): `PUT /v1/releases/{releaseId}/issues` has no caller, so L-140 does not close

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-149.md:58
**Problem:** decision 6 makes the whole *shipped* half depend on the Ship flow calling
`PUT /v1/releases/{releaseId}/issues {issueIds}` after confirm, because the Hub's release holds a
commit and not issues (`hub-contract.md:976-991`). The part's own gap 1 says that call is in
neither the skeleton's File map nor task 131's part, and I confirmed task 131 does not carry it
(its desktop files are `deploy/{ReleaseNotes,FreezeToggle,IssueShipped}.tsx`, `task-131.md:80-88`).
So the route and the table land with nothing writing to them, `sendNotices` has no issues to notify
against, and L-140 (*"The requester hears when it shipped, with the release that proves it"*) is
recorded closed by a route no flow calls. Its own test `AConfirmedReleaseTellsEachRequesterOnce`
would pass by calling the route directly from the test, which is not the product path.
**How to fix:** add the call to task 131 (or 71, whichever owns confirm) as a named ⚑ delta with
its own row — *"after confirm, PUT the derived issue ids to the Hub, reserver only, once"* — and
make task 149 depend on it; then add an end-to-end row to task 142's or 168's run that ships a
release and observes the mail in the fake mailer, rather than calling the route.

## Task 150 — tracker sync, part 1

Decision 2 (the principal marked in the database, the guard refusing every state write, every
label and every revision past 1), decision 3 (`actor: null`, the connection's creator as
`created_by`), decision 4 (disconnect keeps links, drops the credential reference) and the canary
row `TestAnUnknownTargetKeyIsRefused` (a pasted `"token"` is 400 and no column holds the canary)
are all right. The integration table is the strongest in the section: fourteen rows, each with a
named mutant, including a deliberate commit failure to prove the cursor does not move.

### S15-I6 (Important): `UpstreamChange.To` carries upstream title and description text, against the part's own rule

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-150.md:652
**Problem:** decision 5 refuses to carry an upstream value at all — *"never the new value, since a
new title is a customer's words and would be written by nobody"* (`:52-56`). The reconciliation
then adds `To string // the Sink keeps at most 60 code points` and keeps `title` and `description`
in `UpstreamFields` (`:657`), with no rule limiting `To` to the non-text fields. A Jira title edit
therefore lands 60 code points of a customer's words in `work_external_links` and on the issue
view, through a path no person wrote and no erase route reaches (task 146's erase covers
`work_requests` only). The single test (`TestAnUpstreamChangeIsShownNotApplied`) uses `state`, so
nothing pins it. The adapters emit only `state` and `assignees` today (`task-152.md:309-310`), so
the fix is free now and expensive later.
**How to fix:** make `To` empty for `title`, `description` and `deleted` — in `Pulled`'s validation
in the Sink, not in each adapter — and add the row: an upstream `title` change stores an empty
`to` and the view reads *"title changed in Jira at 11:02"* (kills "upstream text stored as a
value").

### S15-M9 (Minor): half the part is superseded in place, and the size claim no longer holds

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-150.md:580
**Problem:** the part is 1214 lines and the *Reconciliation* section at `:580` withdraws or
replaces six of the blocks above it (`:325-420` the engine, `:96-151` the tables, `:478-506` the
Sink, `:296-319` the events, `:517-537` the routes, `:546-573` the wire shapes), plus decisions 5,
7 and 8 and `Adapter.CheckTarget`, `More`, `PullError`'s home and the `credential_*` columns. A
builder reading top-down writes the withdrawn version; the part's own gap 6 says so. It also
breaks the house rule of 1-3 focused hours: the estimate still reads 0.375 (3 h) for an engine, a
fake adapter, a Sink, three tables with a guard, six routes, fourteen integration rows and six
matrix rows.
**How to fix:** delete the superseded blocks and fold the *Reconciliation* points into the
decisions they replace (the record lives in git history, which is what gap 6 wants), then re-split:
the engine, `links.go` and the fake in one task; the tables, guard, Sink, routes and matrix rows in
a second.

Section verdict: Needs fixes

1 Critical (S15-C1), 6 Important (S15-I1 to S15-I6), 9 Minor (S15-M1 to S15-M9). Every part in 141-150 exists and none is cut off; the two largest, 143 and 150, are complete and reconciled. The blockers are cross-part, not within a part: SW.21 is frozen around a sync method task 150 withdraws; the app-and-key state guard is decided two ways; the fake attaches labels the Hub refuses; upstream text leaks through UpstreamChange.To; the shipped notice has no caller; and W5 needs a Key Vault Hub delta nobody owns.
# Plan review — section 16: tasks 151-160 (Phase W, work layer)

Reviewer: adversarial plan review, read-only except this file.
Scope file: `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-141-160.md`
Skeleton: `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`
Parts: `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-{151..160}.md`

All ten parts exist and none is cut off mid-sentence (line counts 339-1490).

## Task 151 — Azure Boards, one way in

Scope coverage: complete. All six scope tests appear (`TestBoardsReadsEveryBatch`, `TestBoardsPriority`, `TestBoardsDescriptionHoldsNoTag`, `TestBoardsStateIsUpstreamOnly`, `TestBoardsThrottleWaitsOnce`, `TestBoardsPlainHTTPLinkIsRefusedBySW6`), each with a named mutant. Citations checked and real: `task-03.md:543-552` (`ImportItem`), `task-02.md:221-222` (200 / 16 KiB), `task-169.md:160-164` (`TrackerTarget`), `task-15.md:265-274` (`validImportURL`), `control-plane/go.mod:53` (`golang.org/x/net v0.26.0 // indirect`), module path `github.com/kpmg-au-emu/panacea-app/control-plane`. Sizing 3 h is fair for three source files plus tests.

### S16-I1 (Important): task 151 declares `PullError`, `TokenFunc` and the four tracker codes that task 150 also declares in the same Go package
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-151.md:110
**Problem:** `worksync/fetch.go` in this part declares `PullError`, its `Error`/`Unwrap` methods and `CodeTrackerThrottled`, `CodeTrackerUnreachable`, `CodeTrackerRefused`, `CodeTrackerShape` (`task-151.md:110-132`). Task 150's reconciled `engine.go` declares the identical names in the same package `worksync` (`task-150.md:689-714`) and records the resolution one way only: *"task 151's block drops those lines (gap 3)"* (`task-150.md:690-692`). Task 151 carries no matching note, and its step 4 says "Write `fetch.go`". An implementer who follows task 151 after task 150 has landed gets `PullError redeclared in this block` and four duplicate consts — the package does not compile, and task 151's whole acceptance block fails before any test runs.
**How to fix:** add a decision to task 151 that `PullError`, its methods and the four `Code*` constants live in `worksync/engine.go` (task 150), cut lines 110-132 from the `fetch.go` block down to `TokenFunc`, `fetcher`, `getJSON`, `postJSON`, and say in step 1 that a landed task 150 already supplies them.

### S16-M1 (Minor): task 151 and 152 still say task 150 is "scope only, no part on this tree"
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-151.md:3
**Problem:** both parts open by marking every name taken from task 150 **UNVERIFIED** because *"scope only, no part on this tree"* (`task-151.md:3-5`, `task-152.md:3-5`). `task-150.md` exists on this tree, 1,214 lines, and pins `Adapter`, `Cursor`, `Pulled`, `UpstreamChange`, `Sink` and `RunRef` (`task-150.md:639-700`). The blanket UNVERIFIED now hides the one name that really did drift (S16-I1) and tells the implementer to trust the scope text over the committed part.
**How to fix:** replace the opening sentence in both with a citation to `task-150.md:639-714`, and keep UNVERIFIED only for names task 150 does not pin.

## Task 152 — GitHub Issues, one way in

Scope coverage: complete, and it exceeds the scope usefully (off-host `Link` refusal, the app-token mint and cache). All six scope tests are present with named mutants. Sizing 2.5 h is tight but fair; the `GitHubAppToken` RS256 mint is the part that could overrun.

### S16-I2 (Important): all three adapters emit `UpstreamChange` with no `To`, so the sync log can never say *closed in Azure Boards*
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-151.md:494
**Problem:** the scope's whole point for upstream changes is that the issue reads *"closed in Azure Boards at 11:02"* (`tasks-141-160.md:439-440`). Task 150 added the field that carries the word — `UpstreamChange.To string`, *"the tracker's word, shown and never mapped to ours"* (`task-150.md:648-655`) — and its `ApplyPage` stores `cut(To, 60)` (`task-150.md:890-891`). Every adapter literal omits it: `task-151.md:494`, `task-152.md:331`, `task-153.md:406`. Task 151 decision 5 even promises *"its `System.State` is carried only as the display word"* (`task-151.md:43-44`) and then never carries it. Result: `upstream_to` is always empty, the UI shows *state changed at 11:02* with no word, and no test catches it because the golden `want` values (`task-151.md:804`, `task-152.md:570`) omit `To` as well.
**How to fix:** set `To` in all three adapters — `System.State` for Boards, the event's resulting `state`/assignee login for GitHub, the Jira transition's `to.name` — and add `To` to the three golden `want` structs so a dropped word fails a test. Retire task 151's gap 4, which task 150 already closed.

### S16-M2 (Minor): `TestGitHubRecordedShape` is named in acceptance but given only in prose
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-152.md:673
**Problem:** the acceptance block expects `TestGitHubRecordedShape` to PASS (`task-152.md:709`), but the tests section defines eight functions and not that one; it exists only as a sentence in the Testdata section (`:673-675`). House style is complete fenced source for every test. Task 151 gives its twin in full (`task-151.md:877`).
**How to fix:** write `TestGitHubRecordedShape` out in the tests section, modelled on `task-151.md:877-896`.

### S16-M3 (Minor): two gaps recorded in 151 and 152 are already closed by the committed parts
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-152.md:733
**Problem:** task 152 gap 2 says *"`SyncRun.errors` has no time … Proposed: `retryAt?`"*; `hubapi.SyncError` already carries `RetryAt *string` (`task-143.md:251-255`). Task 151 gaps 2, 3 and 4 are likewise answered by task 150's reconciled sections (per-item `item_refused` at `task-150.md:884-886`, the page loop at `:740-760`, `To` at `:648-655`). Live gap lists that are already closed train the implementer to skip the list, which hides the ones that are real (152 gap 4: the Hub's own GitHub App manifest is still unassigned).
**How to fix:** strike the closed gaps with a citation to where each was answered, and keep 152 gap 4 with an owner named.

## Task 153 — Jira in; two-way comments on all three

Scope coverage: complete and then some. The coordination-class rule (05 §3.4) is enforced in a pure `ShouldPost` with three tests, the echo loop is cut twice, and decision 9 (an *AU data only* project posts nothing out) is a correct extension of Q8 the scope did not ask for. Citations spot-checked and real: `task-169.md:164`, `task-141.md:97`, `task-21.md:203-212`.

### S16-I3 (Important): task 153 puts store calls inside `worksync.Engine`, which task 150 deliberately keeps behind the `Sink` interface
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-153.md:905
**Problem:** the section opens *"Written against task 150's scope … this logic moves over unchanged"* and then gives `func (e *Engine) commentsIn(ctx, tx store.Tx, …)` calling `store.LinksByExternal`, `store.KnownExternalComments` and `store.InsertExternalComment` from package `worksync`, plus `e.emit` and `e.db` (`task-153.md:912-960`). Task 150's committed design does the opposite: `worksync` holds no store and no transaction, and every write goes through `Sink.ApplyPage`, implemented in package `server` (`task-150.md:869-896`); its `Engine` struct has exactly `Adapters`, `Sink`, `MaxPages` (`task-150.md:719-723`). So `e.db` and `e.emit` do not exist, `worksync` would have to import `store`, and the inbound comments must run *inside* `ApplyPage`'s transaction, which is `server`'s. The logic does not move over unchanged: it must be re-homed, and step 5 (*"Add `commentsIn`, `push` and `eraseUpstream` to `engine.go`"*, `:1310`) sends the implementer the wrong way.
**How to fix:** re-anchor the section on `task-150.md:869-896`: put `commentsIn` in `server.ApplyPage` after step 6, add `PushComments`/`EraseUpstream` to the `Sink` interface (or a second `CommentSink`), and keep in `worksync` only the pure parts that already are pure (`NewInbound`, `ShouldPost`, `OutboundText`).

### S16-I4 (Important): task 153 invents a second name for the tracker mode constants that task 143 already committed
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-153.md:106
**Problem:** task 143 pins `TrackerIn` and `TrackerInAndComments` (`task-143.md:186`, `:583`) and task 150 uses them (`task-150.md:221`, `:403`, `:747`). Task 153 declares `TrackerModeIn` / `TrackerModeInAndComments` (`:103-107`) and uses them in fourteen places including its complete test source (`:215`, `:414`, `:692`, `:952`, `:1027`, `:1064`, `:1182`, `:1208`, `:1227`), and its gap 2 asks task 143 to *add* them (`:1349`). Its own rule at `:100` is *"applied here only if absent"* — they are not absent under another name, so the implementer either gets undefined identifiers or lands two constant pairs for the same two strings, and a later `switch` on one pair silently misses the other.
**How to fix:** rename every occurrence to `hubapi.TrackerIn` / `hubapi.TrackerInAndComments`, drop the const block at `:103-107` and drop the mode half of gap 2.

### S16-I5 (Important): task 153 is two to three tasks in one, sized as three hours
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-153.md:1
**Problem:** the part is 1,359 lines and delivers a whole Jira adapter (pull, `cloudId` resolution, time-zone-floored JQL, an ADF converter, a priority table) — on its own the size of task 151, which is scoped at 3 h for Azure Boards alone — plus `comments.go` both ways, plus the comment halves of `boards.go` and `github.go`, plus three engine steps, plus a `hubapi` wire block, plus SQL for task 145's migration, plus five pure tests, five adapter tests and seven integration rows. The skeleton bills it 0.375 (3 h) (`2026-09-25-phase-w-work-layer.md:577`). The house rule is 1-3 focused hours.
**How to fix:** split into 153a (Jira one way in: `jira.go`, ADF, the priority table, its five tests) and 153b (two-way comments on all three: `comments.go`, the adapters' comment halves, the sink steps, the integration rows), and add the row to the skeleton's task table so the numbering stays true.

## Task 154 — host webhooks in, commit links, magic words

Scope coverage: complete, with all eight scope tests present as named integration rows plus three more. Security is the strongest part of the section: authentication strictly before parsing, one opaque 401 body for all three failures, `hmac.Equal`, a 1 MiB cap enforced before decode, `(project, host, deliveryId)` inserted in the effects transaction, the trailer treated as a link only (SW.9), and `TestOutsideLandNeverDone` proving the guard raises `42501` in the database and not only in Go. Hub-contract citations verified exactly: `:36`, `:337-339`, `:609`.

### S16-I6 (Important): task 154 calls `emitWork` with the wrong arity and names the host actor kind `ActorKindHost`, which does not exist
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-154.md:364
**Problem:** two committed names are wrong in task 154's complete source. (a) It writes `emitWork(ctx, tx, "host_hook.set", map[string]any{…})` at `:364` and `emitWork(ctx, tx, "issue.outside_unmatched", …)` at `:681`; task 13's signature is `func emitWork(tx *Tx, kind, teamID, projectID, subject string, data any) error` (`task-13.md:332`) — no `ctx`, and `teamID`, `projectID` and `subject` are positional, so the event would carry no scope even if it compiled. (b) It uses `hubapi.ActorKindHost` at `:716` and asserts at `:728` that it is task 143's name; task 143 pins `ActorHost` (`task-143.md:163`) and lists it in `ActorKinds` (`:166`). Task 155 found both and recorded them in its own gap 4 (`task-155.md:1476-1480`), but task 154 lands first and its implementer never reads 155.
**How to fix:** rewrite both `emitWork` calls to the six-argument form with the team, project and subject named, and replace `ActorKindHost` with `hubapi.ActorHost` at `:99`, `:716` and `:728`. Then delete task 155's gap 4 items (a) and (c).

### S16-M4 (Minor): the same merge is attributed to two different people depending on the host
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-154.md:868
**Problem:** gap 7 records *"for Azure DevOps the fact names the PR's creator; for GitHub, the sender (the merger). One rule for both would be better"*, and leaves it. The `landed` fact is on the provenance thread, so the same act reads as two different actors depending on which host the repository sits on, and nothing in the plan ever resolves it. Task 155 then adds a third rule for filing (`pull_request.user`, not `sender`, `task-155.md:1482-1484`).
**How to fix:** pick one rule now — the person who caused the transition (the merger / the pusher), falling back to the creator when the host does not name one — state it in decision 8, and give it a row in the integration table.

## Task 155 — linkbacks, autolinks, the filing word, the first push

Scope coverage: complete, and the strongest single part in this section on the rule it names as its own bar. The linkback builder takes no title parameter *at all* so no caller can pass one (`:265-350`), the acceptance proves it with `grep -rn "Title" … work_linkback.go` expecting no output, the redirect target is built from a store-returned `is-` id so the by-alias page cannot be an open redirect (`:965-968`), and `pageByAlias` returns the same 404 for unreadable and non-existent, so an alias cannot be used to probe another project. Decision 1 (never call a host inside a transaction; claim, commit, then post) is right and is not in the scope.

### S16-I7 (Important): task 155 is sized at 3 hours and delivers two streams, nine modified files and five test files
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-155.md:1
**Problem:** 1,490 lines covering, in one task: the parser appends; `work_linkback.go` with two host implementations and the autolink call; four separate deltas to `work_hooks.go`; two janitor statements and an autolink step; store appends; task 80's by-alias page; task 16's branch-name route; and three desktop files (`api/work.ts`, `IssueHead.tsx`, `palette/commands.ts`) — with four test files and ten integration rows. Its own step list runs to seven steps, two of which cross from Go into the desktop. The skeleton bills it 0.375 (3 h) (`2026-09-25-phase-w-work-layer.md:579`); the house rule is 1-3 focused hours, and a task this wide is what the 180-second watchdog and the spend limit punish.
**How to fix:** split into 155a (host side: linkback, autolink, filing word, first push, the janitor steps) and 155b (the reader side: the by-alias page, the branch-name route, *Copy branch name* and the palette row), and add the row to the skeleton's table.

## Task 156 — outbound webhooks

Scope coverage: complete, and the security work exceeds it. The SSRF guard covers `10/8`, `172.16/12`, `192.168/16`, loopback, `169.254.169.254`, Azure's `168.63.129.16`, CGNAT `100.64/10`, `::1`, `fe80::/10`, `fd00::/8` and the IPv4-mapped bypass `::ffff:10.0.0.5`, is re-checked in the dialer's `Control` so DNS rebinding fails before a packet, and redirects are refused. `TestServeNeverSetsWebhookAllow` greps `cmd/hub/serve.go` so the test-only allow hook cannot reach production. The canary test writes a marked string into every W1-W5 text field and reads every delivered body; the actor is reduced to ids because the feed's `Actor` is a `*User` with an email — verified: `hub/control-plane/internal/hub/hubapi/types.go:543` is `Actor *User`. Hub-contract citations `:2083-2085` and `:2092-2096` verified exactly. No Critical or Important finding.

### S16-M5 (Minor): the webhook send phase holds the janitor lock for up to 250 s, starving steps 1-4 and 6
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-156.md:604
**Problem:** the part states it plainly — *"With 200 attempts, 8 at a time and 10 s each, a worst-case send phase is 250 s; `Interval` is 60 s (`H24:72`), so a flood of dead endpoints slows the janitor's other steps"*. Those steps are task 149's mail, task 150's tracker sync and task 157's Teams posts (step 6, queued behind step 5 by name). One team pointing a webhook at a black-holing endpoint therefore delays every other team's sync and channel posts by up to 4 minutes a tick, until `DisableAfter` fires 20 events later. It is disclosed, not mitigated.
**How to fix:** either give phase (b) its own budget (`min(remaining interval, 20 s)`) and carry the rest to the next sweep, or release the janitor lock before (b) and rely on `LockWebhook` alone, which already makes the send phase safe for two replicas.

## Task 157 — the Teams app, part 1

Scope coverage: all seven scope tests present. Bot authentication is right: RS256 only (`none` and `HS256`-with-the-JWKS-modulus both tested), `kid` from the endorsed key set, `iss`/`aud` pinned, the `serviceurl` claim bound to the activity's `serviceUrl`, `channelData.tenant.id` pinned single-tenant, and an explicit test that a realm token is refused here and a bot token never reaches `oidcauth`. `RequesterTeams` is real (`task-146.md:209`), so decision 4 is consistent.

### S16-M6 (Minor): the Teams bot writes as `integration`, indistinguishable on the thread from a tracker sync
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-157.md:52
**Problem:** decision 5 gives the bot `hub.work_actor_kind = integration`. Task 150 defines that kind as *the tracker connection itself*, with `data.via` carrying the `tc-` id so a reader can tell which connection acted (`tasks-141-160.md:435-438`). A Teams-made request has no `tc-`, so its events carry `actor.kind: "integration"` with no `via` at all, and a reader of the provenance thread cannot distinguish *imported from Jira* from *sent from a Teams channel*. Task 143's `ActorKinds` has seven entries and none for Teams (`task-143.md:166`).
**How to fix:** keep `integration` but require `data.via` to be the binding's `channelRef` (or a `tb-` id), and add a row to task 143's event-data rules so an integration event without a `via` fails a test.

### S16-M7 (Minor): tasks 157, 159 and 160 give acceptance commands unwrapped, against the section's own convention and rule 1
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-157.md:455
**Problem:** tasks 151-156 write every acceptance command as `~/.claude/tools/claudezsh/claudezsh '…'`; tasks 157 (`:453-465`), 159 (`:315-325`) and 160 (`:513-530`) give bare `go test`, `export` and `python -c` lines. The device rule is that every shell command goes through the wrapper, which is the only audit trail; an implementer copying these lines silently leaves the trail, and the `export` line in 157 and 159 does not survive the wrapper's `bash -c` boundary anyway, so the next command sees no database URL.
**How to fix:** wrap each line, and replace the two `export` lines with the variable inline in the wrapped command (`PANACEA_APP_HUB_TEST_DATABASE_URL=… go test …`).

## Task 158 — the public API, docs and SDKs

Scope coverage: complete, with all seven scope tests present. Decision 2 (a route-to-type table checked both ways, so adding a route without an entry fails CI) and decision 3 (roles exported from task 07's matrix test file rather than retyped) are the right answers to *"an undocumented route"*. The 3.1 → 3.0.3 deviation from the scope is deliberate, reasoned (`kin-openapi`) and reversible in one constant. Section 1 correctly follows hub-contract §17 and not the superseded `:36`; §17 verified on this tree at `contracts/2026-09-23-hub-contract.md:2301-2381`.

### S16-I8 (Important): the published signature recipe omits the rotation window, so every receiver that follows it breaks on the first rotate
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-158.md:218
**Problem:** README section 4 lists *"the signature recipe with a worked example … the 5-minute window; at-least-once delivery … the retry schedule and the disable rule"* and never mentions that a header can carry **two** `v1=` values. Task 156 decision 5 sends two for 24 h after `rotate`, *"new, then old"*, and its own `Verify` accepts a match on either (`task-156.md:53-56`, and `TestVerifyWindowAndRotation` asserts *"a header with two `v1`, only the second matching → nil"*). A receiver written from the documented recipe parses one `v1`, still holds the old secret at the moment of rotation, and rejects every delivery — exactly the 24 h the overlap exists to protect. The drift test cannot catch it: section 4 is fixed prose in `readme.go`.
**How to fix:** add to section 4's fixed text that `Panacea-Signature` may carry more than one `v1=` and that a receiver must accept if **any** matches, with the rotation procedure (store both secrets for 24 h), and add a second worked vector with two `v1` values.

## Task 159 — apps

Scope coverage: complete and exceeded. It is the only part in the section that notices the scope predates `OD-30` and rewrites itself against hub-contract §17 (`:12-16`), and it adds six tests the scope did not ask for: reserved client ids, approval not leaking across orgs, an app never wielding its person's org-admin role, an app never provisioning a user, every `/v1` pattern classified for apps, and the two `oidcauth` rows proving the hook can never be a bypass (it runs only after every other check). Decision: correct.

### S16-I9 (Important): `TestAppCannotCloseOrFakeAFact` expects an app to move a state, which task 145's guard refuses outright
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-159.md:267
**Problem:** the row asserts *"a `work.write` app: `todo` → `canceled` by hand 200"*. Task 145's scope is unconditional: *"`integration`, `app` and `key` are refused every state column write at all"* (`tasks-141-160.md:257-258`), and task 159's own Interfaces block repeats it — *"task 145's guard (`app` is refused every state column write)"* (`task-159.md:78-79`). So the part contradicts itself one page apart, and the 200 branch of that test fails against the migration. Task 160 found it and recorded it as its gap 4 (`task-160.md:558-561`); task 159 has no matching note.
**How to fix:** change the row to assert `todo` → `canceled` is **refused** as well, keeping `done` and `landed`, and rename it `TestAppNeverMovesAState`. If the owner instead wants apps to make hand moves, that is an amendment to task 145's guard and must be raised there, not assumed here.

### S16-I10 (Important): task 159 adds four `/v1` routes and never adds them to task 158's route table, so 158's drift test fails the moment 159 lands
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-159.md:60
**Problem:** task 158 decision 2 is explicit: *"`TestEveryV1PatternIsDocumented` fails on a pattern with no entry … Adding a route without its entry fails CI"* (`task-158.md:32-36`). Task 159 produces `POST /v1/apps`, `GET /v1/apps`, `GET /v1/apps/{appId}` and `POST /v1/apps/{appId}/decision` (`:71-73`) and its Files list holds no `cmd/hubopenapi/routes.go` and no `docs/api/*` (`:60-69`); its step 4 runs the neighbours but not `hubopenapi -check`. Task 160, which lands next, gets this right — `cmd/hubopenapi/routes.go` ⚑ is in its files (`task-160.md:82-83`) and step 5 regenerates (`:507`) — which shows the omission in 159 is an oversight, not a decision. Tasks 156 and 157 also add `/v1` routes but land before 158, so 158's own generation covers them; 159 is the first that does not.
**How to fix:** add `cmd/hubopenapi/routes.go` ⚑ and the three regenerated `docs/api/` files to task 159's Files, copy task 160's step 5 as a step, and add `go run ./control-plane/cmd/hubopenapi -check` expecting `0` to its acceptance. Make 158 a dependency of 159 in the skeleton (`2026-09-25-phase-w-work-layer.md:583`), as task 160's gap 5 asks for 160.

## Task 160 — keys and limits

Scope coverage: complete, and the strongest part in the section. It names the bypass class it exists to close — *"a prefix that routes a credential is fine; a prefix that admits one is the bypass"* — and proves both directions with call counters (`TestAKeyNeverReachesTheValidator` asserts the `Validator` counter is 0; `TestAPersonTokenNeverReachesTheKeys` asserts the `Keys` counter is 0). `TestUnknownIDAndWrongSecretMatch` closes the existence oracle, `TestHashCoversTheWholeSecret` closes cross-id replay, the team-key narrowing has two independent layers with a test that drops the policy to prove the Go layer alone still holds, and `TestKeyLivesOnItsOwnersSignIns` closes an offboarding hole the scope never raised. Its gaps 1-6 are all real and all correctly aimed at other parts. No Critical, Important or Minor finding.

## Section summary

Ten parts, all present, none cut off. Coverage against the scope is complete for all ten: every scope-named test appears with a named mutant, and several parts add tests the scope did not ask for. The recurring weakness is not coverage but **cross-part consistency**: four parts (151, 153, 154, 159) were written against the *scope text* for a neighbour whose *part* already exists and pins different names, and three of those four would not compile as written (S16-I1, S16-I4, S16-I6). The second theme is **parts that found a neighbour's defect and recorded it only in their own gap list** (155 → 154, 160 → 159), where the earlier task lands first and its implementer never reads the later part.

Counts: Critical 0 · Important 10 · Minor 7.

Section verdict: Needs fixes
# Plan review — section 17 (tasks 161 to 170)

Adversarial review of `docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-161.md` to `task-170.md`, against `docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-161-180.md` (scopes 161-170), the skeleton `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` (rows 161-170, lines 581-590), and the parts they depend on.

All ten parts exist and none is cut off: 161 (661 lines), 162 (505), 163 (422), 164 (446), 165 (396), 166 (631), 167 (686), 168 (1059), 169 (863), 170 (479).

## Task 161 — the Work MCP server, read-only mode, the connector allow-list

Scope `tasks-161-180.md:29-83`; skeleton row `:584`. Coverage is complete: both halves (the
`cmd/workmcp` binary and the Hub allow-list), all nine tools with the scope's names, the RFC 9728
document, read-only as a route, the two layers against a fact state, `ConnectorFragment`, the
matrix rows, and every mutant the scope named. The part correctly overrides the scope's *Entra
OAuth* with OD-30's realm (`hub-contract.md` §17), and says so at the top. Citations spot-checked
and real: `05-design.md:722-734` (person-set states are exactly `triage`, `backlog`, `todo`,
`cancelled`, `duplicate`), `task-37.md:87-89` (`IssueState`'s nine), `docs/AGENTIC-UX.md:179`
(the grid's four level names).

### S17-M1 (Minor): the schema validator is the first of the two layers and has no test that it refuses anything but an enum

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-161.md:533
**Problem:** `schema.go` validates `minLength`, `maxLength`, `pattern`, `minimum`, `maximum` and
`minProperties` before any Hub call, and decision 6 calls it one of two layers. The fifteen unit
rows test only `TestMoveToDoneFailsTheSchema` (an enum) and `TestSchemasUseOnlyTheSubset` (that the
schemas use no keyword the validator lacks). No row kills the mutant "the validator parses the
keyword and never applies it" — e.g. `work_comment {body: 30000 chars}` or `work_set {}` with one
property passing through to the Hub.
**How to fix:** add one row to the unit table, `TestTheValidatorAppliesEveryKeywordItParses`: a
table over `schema.go`'s keywords, each with a value that must be refused (`-32602`, no Hub call)
and one that must pass; mutant: a keyword parsed and ignored, so `work_set {issue}` alone (its
`minProperties: 2`) reaches `PatchIssue` as a no-op patch.

### S17-M2 (Minor): a connector rename is granted but has no event and no test

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-161.md:430
**Problem:** the required table grants `UPDATE (name, removed_by, removed_at)`, so an admin's
second `PUT` with a new `name` may rewrite the row; but the wire defines only
`work.mcp_connector_allowed` and `work.mcp_connector_removed`, and
`TestANoOpPutWritesNothing` asserts a repeated `PUT` writes one event. A `PUT` that changes only
`name` therefore either writes nothing (an un-evented admin change, against
`hub-contract.md:1131`'s one-event-per-write rule) or writes an `allowed` event the test forbids.
**How to fix:** say which: either drop `name` from the `UPDATE` grant and make a rename a remove
plus an allow, or name the behaviour (`allowed` event with the new name) and split the test into
`TestAnIdenticalPutWritesNothing` and `TestARenameIsAnEvent`.

## Tasks 162 and 163 — loops: the record, versions, triggers, the run log, *run now*, the triage loop

Scopes `tasks-161-180.md:85-131` and `:132-178`; skeleton rows `:585`, `:586`. Coverage is
complete and the hard parts are right: append-only versions in SQL and in Go, the `(loop, version,
cause)` unique keys, the missed-window rule, `actor: null` with `data.author`, the owner-left
pause, the fixed turn template with the instruction fetched as data by `panacea loop show`,
`action ∈ {turn, start}` with `move` a 422, the delegated-caller-only outcome route, the
person-only match for `start`, and the pinned `@r`. Citations spot-checked and real:
`task-10.md:247` and `task-13.md:125` (`issue.state`), `docs/AGENTIC-UX.md:179` (the grid),
`05-design.md:722-734` (§3.3). `refused_bound` is spelled identically in tasks 143, 145, 162,
163, 169 and 172.

### S17-I1 (Important): the loop's `bound.levels` is validated and then never read by anything

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-162.md:107
**Problem:** `LoopBound.Levels` (AX §7's grid) is stored on every version and checked against
`OwnerGrid` at save (`task-162.md:286`), but no later step carries it anywhere.
`work.loop_fired`'s `data` is ids, integers and one time (`:353`); the Phase T delta puts only
`Ref = "lp-…#<run>"` on the queue (`:370`); `panacea loop show` returns `{loop, version, run,
action, issue?, instruction}` and no bound (`task-163.md:173-220`); no Fleet or TM-24 delta in
either part passes it to the spawned worker. `turnsPerDay` is enforced (`task-162.md:344`);
`Levels` is not. So the skeleton row's *"within their grid"* (`:585`) and 05 §3.2's *"autonomy
bound"* are a save-time cap on a value nothing consumes: delete `Levels` from the wire and every
test in both parts still passes.
**How to fix:** either (a) carry it — add `levels` to `loop.show`'s result and name the TM-24
delta that applies it to the worker the Start spawns, with a test `TestTheWorkerRunsAtTheLoopsBound`
(mutant: the worker takes the project default); or (b) say plainly in 162's *What this task does
NOT claim* that the bound is recorded and capped only, that nothing enforces it until Phase T owns
a grid, and add a row to gap 2 asking the owner which phase enforces it. (a) is the honest
reading of the skeleton row; (b) is acceptable only if the skeleton row is corrected too.

### S17-I2 (Important): the loop feed kinds are spelled two ways, and two kinds exist in no list

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-162.md:190
**Problem:** task 162's wire names `work.loop_created`, `work.loop_version_saved`,
`work.loop_state_set`, `work.loop_fired`, `work.loop_paused`; task 163 adds
`work.loop_run_closed` (`task-163.md:136`). Task 169's `W5_FEED_KINDS`
(`task-169.md:570-573`) names `loop.created`, `loop.version_saved`, `loop.state_set`,
`work.loop_fired`, `loop.run_ended`. Four of six differ. Task 169's rule is that an unlisted kind
is **ignored, never thrown** (`task-169.md:33-34`), so the desktop replica would silently drop
every loop create, version save and state change; and `work.loop_paused` — the owner-left pause,
the one event that tells a person their loop stopped — is in no desktop list under either
spelling. Task 169's copy test (fake kinds = the three lists, exactly) then fails against task
144 whichever spelling 144 picks.
**How to fix:** pick one spelling in task 143 (the parts agree 143 wins on names) and state it in
162, 163 and 169 in the same edit; add `work.loop_paused` to `W5_FEED_KINDS` and a desktop row for
it in task 172's run log or Desk; and add a row to 169's `kinds.test.ts` asserting the six loop
kinds are present by name, so the mutant "a loop kind added on the Hub and forgotten on the
desktop" dies loudly rather than being ignored.

### S17-M3 (Minor): `work.loop_capped` is emitted by the janitor and declared nowhere

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-162.md:344
**Problem:** `passLoops` emits `work.loop_capped` and `TestTheCapRefusesAndSaysSo` asserts it
(`:420`), but it is absent from the wire's *Event kinds* const block (`:190-196`), from task 169's
`W5_FEED_KINDS` and `W5_ADMIN_KINDS`, and from every other part (checked: it appears only at
`task-162.md:344`, `:360` and `:420`). Its `data` shape is given in prose only.
**How to fix:** add `EventLoopCapped = "work.loop_capped"` to the const block with its `{loopId,
version, run, author}` data, and list it wherever S17-I2's spelling lands.

### S17-M4 (Minor): `work.loop_fired`'s data is declared complete, then grows a field in prose

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-162.md:353
**Problem:** the wire block declares `work.loop_fired`'s data as `{loopId, version, trigger,
eventSeq?, scheduledFor?, author}`; forty lines later the prose adds *"`work.loop_fired`'s `data`
also carries `run`"* (`:359`), which Fleet's `Ref = "lp-…#<run>"` and task 163's outcome route
both depend on. A part that says its wire is complete must not add a load-bearing field later.
**How to fix:** move `run` into the declared data shape at `:353` and delete the prose addition.

## Task 164 — CI outside us: `panacea release`, the Azure Pipelines task, the GitHub Action

Scope `tasks-161-180.md:179-224`; skeleton row `:587`. Coverage is complete and the two
deviations from the scope are both justified and declared: `.<environment>` appended to
`pipelineRun` (because `(panaceaProject, pipelineRun)` is unique in the org) and
`--panacea-project` made required (route 35's body requires it). Every Hub citation checks out
against `feat/hub` at `cafe1303`, `docs/superpowers/contracts/2026-09-23-hub-contract.md`:
`:998-1000` is route 35's run body with exactly the eight keys `reserveBody` declares, `:1021` is
the 15-minute TTL, `:1040` is *"`(panaceaProject, pipelineRun)` is unique in the org"*, `:1029` is
the confirm table. Security is handled: the key is environment-only, never a flag, never logged,
with a test over stdout, stderr and error strings; plain http is exit 2 before any request;
derived idempotency keys; a 4xx never retries. No Critical or Important finding.

### S17-M5 (Minor): the injection guard on the GitHub Action is an acceptance grep, not a test row

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-164.md:381
**Problem:** `TestKeyIsNeverAPlainInput` asserts that **`action.yml`**'s `run:` text holds no
`${{ inputs.`. Gap 6 moved the action's body to `integrations/github-action/run.sh`, so
`action.yml`'s `run:` is now one line that invokes the script and the assertion passes whatever
`run.sh` contains. The mutant the row names (*an injectable step*) is killed only by the
acceptance command `grep -c '\${{ inputs\.' integrations/github-action/run.sh` at `:412`, which is
not a test and does not run in CI with the others.
**How to fix:** point the row at both files — `action.yml`'s `run:` **and** `run.sh`'s whole text
hold no `${{ ` at all, and every input reaches `run.sh` through `env:` — so a later edit that
inlines an expression fails `go test`, not just a hand-run grep.

## Task 165 — the Teams app part 2: the Desk digest, ask your own Orchestrator

Scope `tasks-161-180.md:225-264`; skeleton row `:588`. Coverage is complete and the OD-16 point
is the sharp one and is right: the addressee is always the sender's own Orchestrator, the text is
never read for a name, the project card's `askProject` is re-checked in full on submit
(`TestASubmitForAnotherProjectIsRefused`), the reply goes only to the asker's own chat
(`TestTheReplyGoesToTheAskersChatOnly`), and the reply is posted and never parsed (OD-8). The
one-clock-per-person rule is the scope's *"at most once per 15 minutes"* and is tested across
both channels. The bot's reach is bounded by `TestTheBotCannotWriteAnythingElse`, and intake in a
personal chat is refused. No Critical or Important finding.

### S17-M6 (Minor): the Teams manifest is edited by Step 5 and checked by Acceptance, but is not in the Files list

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-165.md:78
**Problem:** Step 5 says *"The manifest's `personal` scope"* (`:349`) and Acceptance runs
`grep -n '"scopes"' integrations/teams-app/manifest.json` expecting `["team", "personal"]`
(`:363`), but the Files list names ten files and none of them is
`integrations/teams-app/manifest.json`. A reader following the Files list alone ships a bot that
Teams never delivers a personal-scope activity to, and every personal-scope test passes because
they post to the endpoint directly.
**How to fix:** add `integrations/teams-app/manifest.json` ⚑ (task 157's) to the Files list, and
note in gap 5 that the missing File map row covers it too.

## Task 166 — the per-team harness allow-list; team skills in the image or a library

Scope `tasks-161-180.md:265-300`; skeleton row `:589`. Coverage is complete and the security work
is the best in this section: the gate fails closed on a Hub outage (503, not open), unset means
`["claude-code"]` and never *all*, the library must be 40 lower-case hex so a floating ref cannot
be fetched, the staged copy is `0444`/`0555` outside the worktree, `filepath.IsLocal` kills path
traversal, an oversize library is refused rather than truncated, and `TestBuildIsTheSameWithAndWithoutSkills`
is a real OD-8 guard that reads the live `harness.Request` by reflection. The local-mode boundary
is explicit: a project bound to no Hub team is not gated (OD-20). Citations verified against the
work tree: `control-plane/internal/harness/harness.go:17-25` is exactly the five-id `const` block,
and `control-plane/internal/api/sessions.go:784-794` is exactly the harness-resolution step the
gate replaces.

### S17-I3 (Important): three parts give W5-17 three different meanings, and 168 runs it over HTTP

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-166.md:614
**Problem:** task 144's conformance row W5-17 is *"`PutTeamAgentPolicy` with `harnesses:
["claude-code","nope"]` → 403 `harness_not_allowed`, `details.harness: nope`"*
(`task-144.md:762`, row registered at `:600` as `harness_outside_catalogue`). Task 166 decision 4
answers **400 `bad_request`, `details.field: harnesses`** and says in gap 7 that Step 4 will
rewrite 144's row in `conformance/work.go` — a file its own Files list does not name. Task 168
then quotes W5-17 as *"a 409 from the Hub arrives as `hubclient.Error{Status:409,
Code:"conflict"}`"* (`task-168.md:896`), which matches neither, and lists it among the five rows
that are *"the reason the file exists"*. Task 168 runs `conformance.WorkRowsW5` against the real
Hub, so whichever of the three is implemented, one of the other two tasks fails at its acceptance.
**How to fix:** settle W5-17 once, in task 144's row table, before 166 starts: 400
`bad_request`/`details.field` for a body the validator refuses, 403 `harness_not_allowed` reserved
for the session gate (166's own split). Then add `control-plane/internal/hub/hubclient/conformance/work.go`
⚑ to task 166's Files list, and correct `task-168.md:896` to quote the row it actually means.

### S17-M7 (Minor): L-159 is declared closed by a function with no call site

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-166.md:598
**Problem:** *Parity rows served* says **L-159** (skills in the image or a team library) is
**closed**, while gap 6 says *"`Stage` has no call site on this branch"* and Step 5 defers the
call to TM-15's mount, which is **UNVERIFIED until merge**. Nothing in the *does not claim* list
says team-library skills do not reach any sandbox in this phase; it says only that the **gate**
does not reach Fleet. So the parity matrix records a row as closed on the strength of a tested but
uncalled function.
**How to fix:** mark L-159 *closed jointly with the TM-15 wiring* the way L-197 and L-256 are
marked joint with 161 and 173, and add one line to *does not claim*: *"that a team library reaches
any sandbox: `Stage` has no call site until TM-15 merges (gap 6)."*

## Task 167 — audit: person or agent, and whose agent; streaming to Microsoft Sentinel

Scope `tasks-161-180.md:301-331`; skeleton row `:590`. Coverage is complete and then some: every
mutant the scope named has a row, plus four the scope did not (an oversize record stops rather
than truncating, an edited canonical is caught as well as a broken link, the cursor never rewinds,
and cursors are RLS-scoped per org). The exporter reading `audit_events` in-process rather than
route 44 (decision 4) is the right call and its reason — route 44 appends `audit.read`, which
would then be exported forever — is correct against `feat/hub`'s H27. `TestTheRecordHasFifteenKeys`
over every Work event fixture is a strong leak test, and `TestTheTokenIsNeverLogged` covers the
bearer. Test counts add up: 13 unit rows against the acceptance's 13, 7 integration against 7.
The scope's *"with an alert"* is openly scoped out in *does not claim* (the paging rule is phase
8's), which is an acceptable answer.

### S17-I4 (Important): `ActorOf` labels every synced issue `janitor`, and the test that would catch it uses an envelope task 150 never writes

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-167.md:516
**Problem:** `TestASyncWriteIsIntegrationNotJanitor` asserts `actor ""` plus `data.via "tc-…"`
→ `integration`. Task 150 does not write that envelope: *"Task 13's `IssueCreatedData` has no
`via` field and this task does not change a W1 type; the connection's id and creator ride on one
**`tracker.page`** event"* (`task-150.md:38-43`, verified). So the real `issue.created` row for an
imported issue has `actor: null` and no `data.via` and no `data.author`, and `ActorOf` returns
`{janitor, ""}` — an unattributed act in Sentinel for the largest class of W5 writes there is.
Task 153's comments do carry it (`task-153.md:936`, verified), so one write path is right and the
other is wrong, and the unit test passes either way because it builds the envelope by hand. The
task nevertheless claims L-239's Sentinel half **closed**, and L-239 is exactly *"each event says
person or agent, and whose agent"*. Gap 6 names the split but does not treat it as a defect in
this task's own claim.
**How to fix:** make one of these true before the task is closed — either (a) task 150's
`issue.created` carries `data.via` the `tc-` id and `data.author` the connection's creator, so
`ActorOf` answers every row alone; or (b) `ActorOf` follows the companion `tracker.page` row for
a page-committed `issue.created`. Either way, replace the hand-built envelope in
`TestASyncWriteIsIntegrationNotJanitor` with the fixture task 150's emitter actually produces, so
the mutant *"an imported issue reads as the janitor"* dies; and until it does, list L-239 as
closed jointly with task 150.

### S17-I5 (Important): the harness gate opens whenever `sessionLocate` cannot name the project

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-166.md:356
**Problem:** `harnessPolicy` is `projectID, _ := a.sessionLocate(ctx, req.WorktreeID); if projectID
== "" { return nil, false, nil }` — not gated, session allowed. Read on the work tree,
`control-plane/internal/api/sessions.go:997-1019` returns `"", ""` and only **logs a warning** when
`Projects(ctx)` errors, and `continue`s past a `Worktrees(ctx, p.ID)` error. So a transient
enumeration failure turns the gate off entirely and the caller's own `req.Harness` is honoured,
while the very next paragraph of the part argues *"a gate that opens when the Hub is down is not a
gate"* and returns 503 for that case. The five gate tests cover a fixed policy, an unbound project
and a Hub error; none covers `sessionLocate` returning `""` for a worktree that does exist.
**Failure:** `Projects()` errors once (a locked config file, a slow disk); a member of a team whose
policy is `["claude-code"]` posts `{"worktreeId":"w-…","harness":"codex"}` and gets 201.
**How to fix:** distinguish *this worktree is on no project* from *the projects could not be read*.
Give `harnessPolicy` its own locate that returns an error (or check `a.deps.Projects` health), and
treat the error arm as 503 `agent_policy_unavailable` like the Hub arm. Add a sixth gate row,
`TestAnUnlocatableWorktreeIsNotAFreePass`: `Projects` returns an error, the create is 503 and the
backend saw no start; mutant: the locate failure read as *not gated*.

## Task 168 — `hubclient`'s W5 methods over HTTP, the relay rows, W5's Hub end to end

Scope `tasks-161-180.md:332-364`; skeleton row `:591`. Coverage is complete: all 51 methods, both
route lists, the relay rows, the route audit, the conformance rows over HTTP, and an eleven-step
end to end that hits every item the scope listed. The relay security rules are the strongest part
and are each a test: Hub-facing ingress is 404 on all five paths, a Hub 409 passes through with
its code and details, a created key's secret crosses once and is in no log line, and **no**
incoming header is copied outbound except the idempotency key (hub-contract §9.4's local caller
key). Decision 5 is right and its citations check out: `AcceptRequestResponse.Issue` is an
`IssueRef` of id, number and external id (`task-146.md:289-291`, `task-03.md:399-403`, both
verified), so there is no revision to re-hash and the scope's digest re-check does not apply.
Decision 9 discharges task 166's gap 9, and gap 11 raises the fail-closed/fail-open question for
the owner rather than deciding it quietly — the right move.

### S17-I6 (Important): the end to end is where 144's quote rule and 05 §3.5 collide, and it neither names the conflict nor asserts the rule

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-168.md:963
**Problem:** e2e step 3 asserts only that `AcceptRequestResponse`'s `IssueRef` names the new issue
and the request is linked. Task 144's scope says `AcceptRequest` writes revision 1 with **the
quote** as the description (`tasks-141-160.md:206-208`); 05 §3.5, task 170's scope and task 170's
own test *"the accept body's description is empty until the person types"* (`task-170.md:356-365`)
say the quote is customer content and is **never** copied into the issue. Tasks 169 (G2) and 170
(gap 2) both record the conflict and both follow 170; task 168 — the one task that runs task 144's
conformance rows against the real Hub and would therefore fail first — does not mention it. If 144
is implemented as written, a requester's verbatim words are copied into an issue body that then
flows into the feed, the replica, releases and webhooks.
**How to fix:** name it in this part's *Gaps* as a precondition on task 144, and add the assertion
to e2e step 3: the accepted issue's revision 1 description holds no byte of the request's quote,
and the request still holds its quote; mutant: the quote copied into the issue (05 §3.5).

### S17-M8 (Minor): an acceptance command cannot produce the output it expects

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-168.md:1010
**Problem:** `grep -nE "slog|log\.|Printf" control-plane/internal/api/work.go` with *"expected: no
line inside `intakeCalls`"*. The command greps the whole file, which is task 16's relay and has log
lines outside `intakeCalls`, so it prints lines and the reader must judge by eye which function
each falls in. Every other acceptance line in this section is mechanical.
**How to fix:** make it mechanical — `awk '/^func \(a \*API\) intakeCalls/,/^}/' control-plane/internal/api/work.go
| grep -cE 'slog|log\.|Printf'` with expected `0` — or promote it to a row of
`api/work_intake_test.go` beside test 4.

## Task 169 — desktop W5 wire types and replica kinds

Scope `tasks-161-180.md:365-387`; skeleton row `:592`. Coverage is complete: all nineteen types,
the relay table, the feed/admin/signal kind lists, the replica, ops and visibility deltas. The
tests are the right shape and several are better than the scope asked: the goldens are read from
the Hub's `testdata` **by path** so a rename on either side fails, the relay rows are parsed out of
task 168's Go file so the two tables cannot drift, admin kinds are proved to create **zero**
records by a deep count, the secret is hunted by a recursive string walk over the slice and every
replica map after the dialog closes, and the no-persistence check adds a `Storage.prototype.setItem`
spy on top of task 55's grep. Decision 4 (admin records never enter the replica, ADR-H1's *no Hub
data on disk*) and decision 8 (a request's quote only for members of its team, task 57's `admit`)
are both correct readings. Its G1 and G2 are real cross-part conflicts, correctly resolved and
correctly escalated to task 143 and task 144 — see S17-I2 and S17-I6, which are where they bite.
No finding of its own.

## Task 170 — requests in Triage: the quote, *important*, accept or link, the form and rules editors

Scope `tasks-161-180.md:388-422`; skeleton row `:593`. Coverage is complete and every mutant the
scope named has a test, plus five the scope did not (no number key acts on a request row, the row
carries no quote text at all, the link sends the `is-` id and never the alias, reorder is one
write carrying the version it read, `KEYMAP` is unchanged after the intake modules load). The
`text·H` verbatim rule is enforced the right way: `<blockquote data-verbatim>` with the text as a
child, asserted by a test that checks `container.querySelector("img")` is null, that `__pwned` is
undefined, and that `textContent` equals the raw string — plus an acceptance grep for
`dangerouslySetInnerHTML|innerHTML` over the whole directory. The `data-verbatim` exemption it
relies on is real: task 20 defines it at `task-20.md:102` and tests it at `:370-371`. Test counts
match the acceptance (5 panel rows, 11 elsewhere). The visual bar is stated as the owner's eyes,
per CLAUDE.md. No finding of its own.

## Sizing

### S17-M9 (Minor): task 168 is not three hours of work, and two others are close

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:592
**Problem:** the skeleton prices 168 at 0.375 (3 h). The part is 1059 lines and contains: 51
client methods and a two-list route table, the deletion of the stub, `methods_work.go`'s edits,
the whole `intakeCalls()` relay, the route audit, `agentpolicy.go` plus its `main.go` wiring, the
conformance suite over HTTP, and an eleven-step end-to-end story each of whose steps exercises a
different task's Hub half (146-167) and commits separately. Its own Steps list seven bullets, one
of which is *"step by step, each step's assert before the next step's call. Commit after each
step's assert is green."* Three hours does not buy that, and a watchdog-killed agent mid-way
leaves the stub deleted and the client half-written. Tasks 166 (2.5 h for two Hub routes, a new
`workpolicy` package with staging and the fragment, the session gate, and a conformance row
change) and 167 (2.5 h for `ActorOf`, a store, a table, an HTTP client, an exporter loop, config
parsing and serve wiring) are the same shape, one notch smaller.
**How to fix:** split 168 in three along the seams it already has — (a) the table and the 51
methods with the stub deletion, (b) the relay, the route audit and `agentpolicy.go`, (c) the
conformance-over-HTTP run and `TestWorkIntakeE2E` — and let (c) depend on (a) and (b). Renumber or
suffix as the skeleton's convention allows, and reprice 166 and 167 to 3 h.

## Summary

Ten parts, all present, none cut off. The section is strong: every part gives complete fenced
source for its interfaces, types, schemas, routes and wire shapes; tests come first, in tables,
each naming the mutant it kills; every part carries acceptance commands with expected counts that
match its own test tables; and every part ends with an honest *What this task does NOT claim* and
a *Gaps* list that names what it could not verify. Citations are unusually accurate — every one I
checked against a real file was exact, including six into `feat/hub`'s
`docs/superpowers/contracts/2026-09-23-hub-contract.md` (`:337-341`, `:994-1000`, `:1036-1042`,
`:1131`, `:2301`), three into the work tree's live Go (`harness.go:17-25`,
`sessions.go:784-794`, `sessions.go:997-1019`), and several across sibling parts
(`task-10.md:247`, `task-13.md:125`, `task-20.md:102`, `task-146.md:289-291`,
`task-150.md:38-43`, `task-153.md:936`, `task-144.md:762`).

Security is handled with real depth: OD-30's realm replaces Entra in 161 with the contract's §17
cited; read-only is a route, not a token flag; two independent layers stop an app writing a fact
state; a loop is its owner's turn on its owner's budget and a team admin may pause but never edit
or resume (OD-16); the outcome route takes a delegated caller only; an agent's delegated move
never starts a lane; a Teams ask always goes to the sender's own Orchestrator and its reply only
to the asker's own chat; team skills are a read-only copy at a 40-hex commit with traversal and
size refusals, and `harness.Build`'s argv is proved byte-identical (OD-8); the relay forwards no
header and logs no body; a request's quote is rendered as a text child under `data-verbatim` and
never as HTML. The local-mode boundary (OD-20) is stated in 162, 163, 166 and 168, and OD-27's
`expectNoPrediction` runs on every surface test.

Six Important findings, no Critical. Four are cross-part naming or rule conflicts that must be
settled once, in task 143 or 144, before these tasks are built (S17-I2, S17-I3, S17-I6, and the
143-vs-162/163 loop shape that S17-I2 covers). One is a control that is validated and then never
applied (S17-I1). One is a control that opens on a local read failure (S17-I5). All six have a
concrete, small fix.

Section verdict: Needs fixes
# Phase W plan review — section 18 (tasks 171–180)

Reviewer pass: adversarial. Read-only except this file.

## Task 171 — trackers on the desktop

Scope covered: connect form for the three sources, no credential field, direction in words, comments-both-ways off by default with an in-place confirm, the sync log, disconnect keeps links. Wire types check out against task-169.md:159-195 (TrackerTarget, TrackerConnection, ExternalLink, UpstreamChange, SyncRun) and the client methods at task-169.md:452-457. Citations to task-42.md:196-202 (threadWords), task-44.md:130-150 (ConfirmInPlace) and task-20.md:126-131 (expectNoPrediction) are real and say what the part claims.

### S18-M1 (Minor): upstreamWords gives every upstream change the state reason
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-171.md:158-162
**Problem:** upstreamWords maps field to an act word (closed / reassigned / X changed) but always appends the same tail, not applied: sync never moves a state. An assignee change then reads reassigned in Jira, not applied: sync never moves a state, which states a rule that is not the one that refused it. The only test covers field state, so the wrong sentence for the other two branches is never exercised.
**How to fix:** give each branch its own reason (state: sync never moves a state; anything else: sync never writes back), and add the assignee case to the SyncLog test so the mutant one reason for every field dies.

### S18-M2 (Minor): two name tables for one tracker inside SyncLog.tsx
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-171.md:155-156
**Problem:** SyncLog.tsx imports SOURCE_NAME (Azure Boards / GitHub Issues / Jira) for the log header and then defines a private trackerName with different words (GitHub, not GitHub Issues) for the row text, so one log calls the same connection two names.
**How to fix:** drop trackerName and use SOURCE_NAME, or export both from trackerTarget.ts with a comment saying which surface takes which, and assert the header and row words in the SyncLog test.

### S18-M3 (Minor): the scope asks for updated and skipped rows; the part draws neither
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-171.md:329-331
**Problem:** scope 171 (tasks-161-180.md:436-437) names the log result vocabulary imported, updated, skipped, refused, and names task 150 SyncLogEntry as the row. The part draws imported, refused, upstream and run, redefines SyncLogEntry as a desktop view type, and records the difference as gap 4. The deviation is honest but the scope words stay unmet and no task is named to close them.
**How to fix:** state in gap 4 which task owns the per-item record (task 150) and add the one row it needs, or have the skeleton amend the scope vocabulary so the coverage checklist is not left half-open.

## Task 172 — loops on the desktop

Scope covered: append-only versions, the AX section 7 grid with inherited greyed and levels above the owner disabled, the interruption projection deliberately omitted (OD-27), owner-only edit with admin Pause, run-now idempotency, every outcome shown, no spinner. Types match task-169.md:200-245 (Loop, LoopTrigger, LoopBound, LoopVersion, LoopOutcome, LoopRun, CreateLoopBody, LoopVersionBody) and the ops at task-169.md:638-641. Citations verified: AGENTIC-UX.md:179 is the grid and :181 is the interruptions line the part refuses; heartbeat.ts:13 BEAT_MS and :64 useHeartbeat exist; task-13.md:112-138 holds the W1 kinds the event list copies.

### S18-I1 (Important): Save calls versionDiff with a LoopVersionBody, which does not typecheck
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-172.md:271
**Problem:** versionDiff is declared (a: LoopVersion, b: LoopVersion) at task-172.md:137, and LoopVersion requires loopId, version, savedAt and savedBy (task-169.md:229-236). The editor builds body as a LoopVersionBody (task-169.md:344 omits those four) and the part then says Save with nothing changed sends nothing because versionDiff(current, candidate) is empty. That call fails TypeScript, and the acceptance command npm --prefix desktop run typecheck expects exit 0, while component test 10 (No changes, mutate not called) is written against exactly that call.
**How to fix:** declare versionDiff(a: LoopVersionBody, b: LoopVersionBody) — its body only reads trigger, instruction, action, labelId and bound — and keep the LoopVersions.tsx call site, which passes two LoopVersions and still satisfies the wider type.

### S18-M4 (Minor): the run log drops the started session id the scope names
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-172.md:310-311
**Problem:** scope 172 (tasks-161-180.md:470-471) asks for the ot- turn and any Start session and worktree, in mono. LoopRuns lists firedAt, trigger, outcome, turnId and issueKey plus branch, so StartedLane.sessionId (task-169.md:236) is never shown and no test asks for it; a reader cannot get from a run to the session it opened.
**How to fix:** add sessionId to the started columns in mono and assert it in component test 6, or say in What this task does NOT claim why the session id is withheld.

### S18-M5 (Minor): 2.5 hours is short for six new modules and seventeen tests
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-172.md:3
**Problem:** the part builds loopModel.ts, LoopEditor.tsx (trigger radio pair, ten event checkboxes, schedule picker, the Markdown editor, action, turns-a-day, a ten-by-four radio grid with inherited and disabled states, the consequence line), LoopList.tsx, LoopVersions.tsx with a side-by-side diff, LoopRuns.tsx with run-now and a heartbeat child, and loops.css, against 7 model tests and 10 component tests. That is the largest surface in this section at the same estimate as task 171, which builds three simpler components.
**How to fix:** either split the editor (name, trigger, instruction, action) from the grid and the consequence line into two parts of about 2 hours each, or state the estimate as 4 hours so the stream plan does not under-book W5.

## Task 173 — admin on the desktop

Scope covered: who sees what, the once-shown secret, rate as facts, the 40-hex commit refused in the form, in-place confirms, one press one call, the two MCP addresses, Publish absent until Phase T. Citations verified on the tree: harness.go:19-25 holds exactly the five ids HARNESS_IDS lists; time.ts exports only formatElapsed and formatWaiting, so gap 3 is true; hub-contract 2026-09-23 line 320 is the Hub.OrgAdmin sentence, :625 is Team with the caller role, :653-666 is GET /v1/me with orgAdmin and invitesBound; ADR-H1:789 is the hub.org_admin reversal. Decision 2 (read /v1/me, never a token claim) is the right call and is better than the scope wording it departs from.

### S18-I2 (Important): the double-press tests cannot kill the double-send mutant as written
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-173.md:449-454
**Problem:** usePressOnce clears out.current in a finally, so the guard only holds while the call is in flight. The test presses with await Promise.all([user.click(btn), user.click(btn)]) against a fake whose createAPIKey resolves on the next microtask, and userEvent.setup() puts a delay between its own events, so the first call has already settled and re-enabled the button before the second click dispatches: the correct implementation is called twice and the test fails. The same shape is specified for Redeliver (task-173.md:501) and for Run now in task 172 (task-172.md:443-446, two clicks within one tick).
**How to fix:** hold the fake open — a deferred promise the test resolves after both clicks — or dispatch both clicks in one tick with fireEvent.click, and assert the button is disabled between them. State in the part which of the two the test uses, because only the open-fake version proves the guard.

### S18-M6 (Minor): five of the seven components are prose-only at a 3 hour estimate
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-173.md:263-273
**Problem:** Keys.tsx is given complete, but Apps, Webhooks, Deliveries, Connectors and AgentPolicy are four bullet lines each, with one or two test rows apiece in a table rather than written tests. The part builds twelve new files and nine test files in the same 3 hours. The house style asks for the tests first, each with the mutant it kills; a table row is the mutant without the test.
**How to fix:** either split Apps plus Connectors (org admin) from Webhooks plus Deliveries plus AgentPolicy (team admin) into two parts of about 2 hours each, or write the six table rows out as test bodies the way Keys.test.tsx is written.

## Task 174 — W5 proof and results

Verified by re-running the part own selection over the matrix: awk over 05-design.md section 1 with the slice column matching W5 returns exactly the forty-one ids the part lists, L-116 included. Decision 2 is correct and the scope count of forty is the one that is wrong. The cited matrix lines check out (05-design.md:111 L-7, :149 L-40, :255 L-116, :309 L-152, :311 L-154, :333 L-171, :334 L-172). The sweep list holds 18 surfaces, matching the acceptance line of 19 tests.

### S18-I3 (Important): check-w5.sh has no state for a row that honestly could not be run
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-174.md:196-204
**Problem:** the check fails a row whose cells are empty and fails a row whose last cell is not a 7-40 hex sha, and the acceptance command expects exit 0 with all forty-one rows. w5-run.md tells the runner to write not run and why when a frame cannot be run in the environment, and decision 7 lists things that stay unverified; L-159 (a team skill reaching a sandbox) needs a sandbox this local stack does not have. So the gate is either unsatisfiable or it pressures the runner to paste a sha for a row nobody proved, which is the exact OD-27 failure the check exists to prevent.
**How to fix:** accept a third cell form, not run: <reason>, treat it as neither closed nor a failure, print it on its own line of the coverage summary (for example 39 closed, 2 not run), and add a third fixture w5-not-run.md whose expected output is that summary at exit 0, with the mutant a not-run row counted as closed.

### S18-M7 (Minor): the scope count of forty rows is wrong and is quoted downstream
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-161-180.md:530-534
**Problem:** scope 174 lists thirty-five whole rows plus five halves and calls it forty rows. The matrix selection returns forty-one: L-116 has the slice W2 / W5 (05-design.md:255) and its W5 half is the Teams digest. Task 174 caught this as its gap 1, but the scope and the coverage checklist still say forty, and task 165 scope does not claim L-116 either, so the row is unowned in the checklist.
**How to fix:** correct the scope count to forty-one, name L-116 W5 half in task 165 Closes sentence, and re-run the awk selection when scopes/coverage.md is next touched.

## Task 175 — Copy from team on templates, views and policies (L-7)

Scope covered: copy not reference, match by name and never create, criterion and project always dropped, 404 for an unreadable source, admin-only for a policy, one event with copiedFrom, the dialog listing what was left out. Citations verified: hub-contract 2026-09-23 line 50 is the Idempotency-Key paragraph and :273-275 is the 404-not-403 rule, so decision 6 and decision 4 both rest on the contract. The security shape is right: source read under RLS, one 404 body for missing and unreadable alike, policy behind admin, no vocabulary written into the target.

### S18-M8 (Minor): a kept-but-widened state clause is reported under Left out
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-175.md:252-258
**Problem:** PlanCopy returns Keep true with To set to the band for a sub-state the target lacks, and also appends a DroppedField with reason state_by_band. The dialog renders every dropped entry under the heading Left out, so a clause that was kept and broadened reads as one that did not arrive, and the copied view silently shows more rows than the source did. The desktop test only asserts that both lines appear, so the wrong heading is never caught.
**How to fix:** either split the reply into dropped and widened, or keep one list and have the dialog group by reason, with state_by_band under a second heading such as Widened, and assert the heading in the CopyFromTeam test.

### S18-M9 (Minor): an untaught reference kind is reported as a project
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-175.md:264-266
**Problem:** the default arm of the switch drops RefProject and every future kind with reason DropProject, so a later filter.cycle reference would tell the person that project belongs to the other team. The part own test TestPlanCopyDropsAnUnknownRefKind asserts only that one entry was dropped, never its reason, so the wrong words are unasserted.
**How to fix:** give RefProject its own case, add a DropUnknownRef reason (words: this team has nothing of that kind) for the default arm, and assert the reason in that test.

## Task 176 — personal auto-assign rules (L-40)

Scope covered: one rule per moment, applied inside the person own write, never over a named assignee, never for a via, one event, no person field on the route. The SQL follows task 06 shape (task-06.md:367-387 has the same unqualified CREATE with qualified ALTER and explicit grants), the DELETE-then-INSERT PUT justifies the missing UPDATE grant, and decision 1 (move the preference off the device into the Hub) is a real improvement on task 96 scope. The route design makes the mutant a userId body field unwritable by construction.

### S18-I4 (Important): i_accept is defined as a state move, but the scope and the switch label read as the intake accept
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-176.md:35-37
**Problem:** decision 2 defines i_accept as a hand move the person makes out of the triage state. Scope 176 says an issue the person creates, accepts from Triage or starts, and in W5 accepting from Triage is task 146 accept of a request, which creates a NEW issue in the triage state (L-133). Under the part reading, accepting a request fires i_create, not i_accept, so a person who turns on only the switch labelled Issues I accept from Triage is never assigned, and a person who turns on only Issues I create is assigned by an act they call accepting. The word Triage names both the intake lens and the issue state, and the part never says which it means. The integration test only covers a triage to todo hand move, so neither reading is proved against the intake path.
**How to fix:** decide and write it down: if i_accept is the intake accept, call ApplyPersonalRule in task 146 accept path and relabel the state-move case; if it is the state move, rename the switch to Issues I move out of Triage and add an integration row proving that accepting a request fires i_create and not i_accept.

## Task 177 — dashboards (L-152)

Scope covered: a dashboard is a view with two added fields, four block types and no room for a percentage or burn-up, no rail node and no set-as-home, the Desk stays the start route, a block the viewer cannot read draws no number, every number opens its issues. AX section 5 at AGENTIC-UX.md:87 does say the Desk is the default landing surface, and 05-design.md:309 is L-152 as quoted.

### S18-I5 (Important): the rule line omits the block filter, so two blocks measuring different populations read identically
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-177.md:236-248
**Problem:** ruleWords takes only measure, groupBy and windowDays, yet DashboardBlock.Filter narrows the population on the Hub (decision 6). A block filtered to label Bug and an unfiltered block over the same measure and window render the same sentence, lead time: created to done; issues done in the last 30 days, and the title is optional, so the reader is given a number whose narrowing clause is nowhere on the block. That contradicts decision 4 (a number is never shown without what it measured) and the scope mutant a number without its source. The ruleWords test pins the filter-free sentence, so the omission is locked in by the test.
**How to fix:** pass the whole block to ruleWords and append the filter in words (for example ... in the last 30 days, label Bug), reusing whatever clause-to-words helper task 91 uses for view filters, and add a test row for two blocks that differ only by filter having different rule lines.

### S18-M10 (Minor): the count measure sentence repeats itself
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-177.md:239
**Problem:** MEASURE maps count to count: issues done and ruleWords then appends issues done in the last N days, so a count block reads count: issues done; issues done in the last 30 days. It also hard-codes done as the count population, which is not stated anywhere in decision 2 and is contradicted by a count block whose filter selects an open state.
**How to fix:** make the count entry the measure alone (count of issues) and let the population clause carry done or the filter, then assert the count sentence in ruleWords.test.ts, which currently tests lead_time and triage_time only.

### S18-M11 (Minor): an absent kind on an existing view create falls into the kind refusal
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-177.md:130-133
**Problem:** View.Kind is documented as list when absent on the wire, but ValidateBlocks is specified to run after decode and returns the field kind for anything that is neither list nor dashboard, so an empty string from an omitted key is refused. Every existing task 80 client that does not send kind would get 422 on create. Step 4 reruns task 80 integration tests, which would catch it, but no test in this part pins the default.
**How to fix:** state in the route delta that the decoder defaults an empty kind to list before validation, and add one pure test that ValidateBlocks of an empty kind with no blocks returns the empty string.

## Task 178 — warehouse and spreadsheet sync, and the first named connector (L-154)

Scope covered: no new Hub route, the feed is the source and the webhook is only a nudge, text fetched by key with the key own rights, tombstones on erase, cursor after the write, credentials from the environment. VerifyNudge implements SW.22 correctly: it MACs t, a dot and the raw body, compares in constant time, and refuses a timestamp more than five minutes either side.

### S18-I6 (Important): the acceptance grep for text fields cannot return 0
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-178.md:376-377
**Problem:** the command greps apiDefinition.swagger.json for the quoted words title, description, body, text or quote and expects a count of 0, but the connector file this part specifies contains quoted description keys in info, in every response and in x-ms-notification-content, and a body parameter named body in the trigger. The file as written makes its own acceptance command print at least five. Whoever runs it will either delete legitimate OpenAPI keys or ignore the check, and the check is the one that guards customer text in the trigger schema.
**How to fix:** point the check at the payload schema only, for example a jq over definitions.Delivery property names at every depth, or grep for the property-name forms (a quoted word followed by a colon and a type object) inside the definitions block, and keep the expected output 0 against the file as shipped.

### S18-I7 (Important): KindOf is given by three examples, so every head update is silently skipped
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-178.md:207-210
**Problem:** KindOf is specified as issue.created to issues, issue.revised to revisions, issue.erased to issues-with-erase, and an unknown kind to the empty string, skipped. The full table is nowhere in the part. Under that spec issue.updated, issue.state, issue.attached, issue.related, comment.reacted, comment.resolved and every feature, cycle, release and request update map to nothing, so a warehouse row is written once at create and never tracks the Hub again: a state change or a re-assignment never reaches it, and no test catches that because the ten tests cover resume, crash, erase, unreadable, nudge, snapshot, key and connector, never a plain update.
**How to fix:** write the complete kind-to-file table in the part, taken from task 13 W1 kinds and task 169 W5 feed kinds, with a test that every kind in those two lists is either mapped or on an explicit ignore list, and one test that an issue.state event rewrites the issue line.

### S18-M12 (Minor): go build -o /dev/null does not work on the Windows dev machine
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-178.md:372-373
**Problem:** the acceptance step builds to /dev/null. Go on Windows uses NUL as the null device, so the command fails to create the output file on the machine this plan is executed on, and the same idiom is in task 164. The expected output built would never print.
**How to fix:** build into the scratch directory and delete it, or use go vet plus go build ./control-plane/cmd/panacea-export with the binary left in a git-ignored path.

## Task 179 — syntax-aware diff and file categories (L-171)

Scope covered, and two scope corrections that are right: a zero-length Lezer error node is normal for a hunk fragment so only a non-empty one falls back, and plain mode maps git own lines rather than running a second line differ that could disagree with the diff that landed. The invariant (spans concatenate to the hunk text byte for byte) is the strongest test in this section. Citations verified: owner-decisions.md:96 is OD-11 rule 3 with exactly the five licences, tokens.css:75 is --need needs you, vitest.config.ts:9 is jsdom, so the node-environment note is needed.

### S18-M13 (Minor): the config rules miss .yml, so azure-pipelines.yml reads as source
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-179.md:192-194
**Problem:** CATEGORY_RULES names *.yaml, *.toml, *.tf, *.json, .github/** and deploy/**, but not *.yml. This repository root holds azure-pipelines.yml, the pipeline the owner prefers over Cloud Shell, so a lane that edits it shows it as source, first in task 180 core-first order, ahead of the code the criterion is about. The first match wins test covers deploy/x.yaml and would not catch it.
**How to fix:** add { category: config, glob: *.yml, re: /\.yml$/ } beside *.yaml and add azure-pipelines.yml to the first match wins test row.

## Task 180 — guided review (L-172)

Scope covered: deterministic grouping with no gateway call, the criterion from the commit trailer through the issue serves, core first, every hunk exactly once with also serves for the rest, category order inside a group taken from task 179, a hunk with no tier-2 back-link saying no span recorded, no reviewed count. The two phase 3 wire deltas it adds are argued well (the 50-commit cap on P3-07 lane commits would lose issues on exactly the large lane this task is for), and the correction that an unreadable issue is not the same as serving no criterion is right. 05-design.md:762 is the five convergence words as cited.

### S18-M14 (Minor): local mode turns every group into Issue not read, and the part never says so
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-180.md:235-239
**Problem:** guide() reads IssueServes from the Work replica, which is the team Hub replica. The Merge lens is a phase 3 local surface and OD-20 keeps local mode, so a person reviewing a lane with no team Hub signed in has an empty issues map: step 1 sends every trailered hunk to unread with the why issue is- could not be read, and the panel reads as though the Hub lost the issues rather than as though there is no Hub. Nothing in the part, its tests or its does-not-claim list mentions local mode.
**How to fix:** take a signed-in flag (or an empty-replica signal) in GuidedReviewMount, and when there is no Hub show one sentence, no team Hub is signed in, so no criterion could be read, with the flat path order instead of the criterion groups; add a test row for it.

### S18-M15 (Minor): AGENTIC-UX section 8.5 does not exist and was left UNVERIFIED instead of checked
**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-180.md:381-383
**Problem:** the part cites AGENTIC-UX section 8.5 from the scope and marks it UNVERIFIED because the section number was not re-read. docs/AGENTIC-UX.md has one heading at section 8, Keeping the CLI real while aggregating many, and no numbered subsections, so 8.5 names nothing. The citation was one grep away, and rule 4 asks for UNVERIFIED only where a check was impossible.
**How to fix:** drop the AX 8.5 citation or replace it with the section that actually carries the rule the panel follows (Principle 7 for a number that opens its source, section 5 for the Desk), and correct scope 180 the same way.

---

Section verdict: Needs fixes
# Section 19 review: Phase W tasks 181 to 187 (WL later rows and WL proof)

Reviewer: adversarial plan review, read-only. Parts read one at a time against
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/tasks-181-187.md` and the skeleton
`docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:613-619`.

All seven parts exist and none is cut off (task-181 295 lines, 182 375, 183 331, 184 456, 185 447,
186 335, 187 474).

## Task 181 — signed land commits for agent sessions (L-176)

Scope coverage is good and the phase 3 citations check out against `.claude/worktrees/phase-3`:
`task-02.md:121-133` is `Hardening`/`MachineHardening`, `task-09.md:93-95` is `Identity`,
`task-09.md:121` is step 11's `commit-tree`, `task-04.md:85-107` is the closed reason list
(`ReasonNoGitIdentity` inside it). H1's `commit.gpgSign=false` is real, at `task-02.md:615`
(the part cites `:526`, where the H1 section starts — close enough, not a defect). Hub citations
check out too: `task-03.md:348-355` `TeamPolicy`, `:504-508` `PutTeamPolicyRequest`,
`task-06.md:276` `team_work_policies`, `task-11.md:123` the full-replace PUT, `task-11.md:288`
`PutTeamPolicy(...) (changed []string, ...)`, `task-07.md:175` `projects.team_id`.
The row's UNVERIFIED is genuinely answered. Sizing (2.5 h) is optimistic for gitx + land + a Hub
migration + a desktop switch, but not out of band for this plan.

### S19-I1 (Important): the team's signing rule fails open when the Hub cannot be read

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-181.md:36
**Problem:** `RequireSignedLand` is an enforcement control set by a team `admin`, but the part
makes it advisory in practice: with the Hub unreachable and no cached value the land proceeds on
the machine rule alone and reports `policyRead: false`. Land is the desktop's local git act, so
the person the rule binds is the person who can make the Hub unreachable (offline, proxy off,
sign-out). Step 8a computes `sign := Signing.Always || required` with `required=false` on an
unread policy, so 8a's refusal never fires and an unsigned merge lands into a team that requires
signatures. Nothing under *What this task does not claim* admits this, so a reader takes
`requireSignedLand` for a guarantee.
**How to fix:** either (a) fail closed — when `read` is false and the project's team is known,
refuse with `signing_unavailable`, `details.why: "policy_unread"`; or (b) keep fail-open but say
so in *does not claim* ("not an enforced control: a land made while the Hub is unreachable and
uncached may be unsigned"), carry `policyRead:false` into the land's Hub event so the thread shows
it, and add `TestPolicyUnreadIsRecordedOnTheThread` to kill a silent drop. Either way add
`TestTeamRuleSignsWithoutMachineRule` (machine `Always` false, `SignPolicy` required → `%G?` is
`G`); no row today proves the team rule alone can make a land sign.

### S19-M1 (Minor): where the `nil`-keeps merge happens is unspecified, and the store signature fights it

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-181.md:191
**Problem:** `PutTeamPolicyRequest.RequireSignedLand *bool` is the right call, but
`store.PutTeamPolicy(ctx, tx, org, p hubapi.TeamPolicy)` (`task-11.md:288`) takes a *full*
`TeamPolicy`, so the nil-keeps decision must happen before the store call. The part does not say
where, and a read outside the transaction loses a concurrent admin's write.
**How to fix:** name it — the handler resolves `nil` by reading the stored row inside the same
`pgx.Tx` that writes it (`SELECT … FOR UPDATE`, or `COALESCE` on the UPDATE) — and have the
conformance row run two interleaved PUTs, one with the field absent, to kill a read-then-write
outside the transaction.

### S19-M2 (Minor): no desktop copy for the new 409 reason

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-181.md:159
**Problem:** `ReasonSigningUnavailable` joins a closed list whose contract is "the desktop
switches on it" (P3 `task-04.md:85-86`). The Files list touches only `SignedLand.tsx` and
`work.ts`, so an unmapped reason reaches the user as a bare code and a `signer_failed` land looks
like an unexplained failure.
**How to fix:** add the land surface's reason map to the Files list with the four `details.why`
sentences (`no_format`, `no_key`, `no_program`, `signer_failed` carrying `gitSays`), and one
desktop test asserting each renders a sentence, not a code.

## Task 182 — Slack, after Teams (L-206, and the *Slack next* clause of L-80, L-97, L-137)

Every scope-named test is present and the data rule (inbound text welcome, outbound metadata
only, no fact from chat, acts only as a linked person) is carried through the types so a
formatter *cannot* hold a title. Citations verify: `tasks-141-160.md:680` is task 157's scope,
`:241-245` is task 144's `secret_ref` + the Key Vault caveat, `tasks-161-180.md:230` is task
165's `teamsapp/` (and the part correctly flags `teams/` vs `teamsapp/` as a skeleton gap),
`docs/AGENTIC-UX.md:243` is anti-pattern 8, and the Hub contract's
`docs/superpowers/contracts/2026-09-23-hub-contract.md:2083-2084` really does say "no Key Vault
access in v1". The `hooks.slack.com` prefix check on the webhook sink is a good SSRF guard.

### S19-C1 (Critical): nothing binds a Slack workspace to an org, so a channel link crosses orgs

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-182.md:216
**Problem:** `slack_channel_links` is `(slack_team_id, channel_id, team_id, linked_by, linked_at)`
— no org column, no join to the team's org — and `PUT /v1/teams/{teamId}/slack-channels/{channelId}`
is authorised as "team admin" on `teamId` only, with `slackTeamId` taken from the body. A team
admin in org A who knows org B's workspace id (`T…`, visible to anyone in that workspace) links
org B's channel to their own team. Then every *Send to PanaceaApp* in that org B channel creates
an `rq-` **with org B's verbatim message text** in org A's Hub, and a `slack` feed binding posts
org A's issue metadata into org B's channel. The migration's own comment says "row-level security
follows task 06's pattern for org-owned tables", but `slack_channel_links`, `slack_person_links`
and `slack_link_nonces` carry no org, so that RLS cannot attach — and the contract makes org
isolation "row-level security **and** Go" (`hub-contract.md:2081`). The matrix rows test role
(`member` → 403) and never org, so no test catches it. The same hole is on the link nonce: a
nonce is bound to `(slack_team_id, slack_user_id)` and the Entra POST that redeems it is not
stated to check the signed-in user's org against the install's.
**How to fix:** put `org` on `slack_channel_links`, `slack_person_links` and `slack_link_nonces`,
take it from the *install* row inside the write transaction, and refuse with 404 when the
install's org is not the caller's; add task 06's RLS policy to all four tables (the install row
already needs a `REFERENCES orgs (id)`). Add two red-first tests: `TestChannelLinkCrossOrgRefused`
(admin of a team in org A, install in org B → 404, no row) and `TestPersonLinkCrossOrgRefused`
(nonce from org B's workspace redeemed by an org A user → 410, no row).

### S19-I2 (Important): dedup is committed before the work, so a failed event is never retried

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-182.md:238
**Problem:** "An event inserts `event_id` into `slack_events_seen` **first**; a conflict answers
200 and does nothing else." If the insert is its own statement/transaction and the request create
or thread append then fails (a routing-rule error, a database blip, a panic mid-handler), Slack's
retry finds the row, answers 200 and drops the message permanently — the user's intake silently
disappears with a 200 on the wire. `TestRetriedEventNoDouble` only proves the happy-path duplicate
is suppressed, so the mutant "dedup outside the work's transaction" survives.
**How to fix:** insert the `event_id` in the **same** `pgx.Tx` as the request/message write, so a
rollback releases the key and Slack's retry does the work; keep the unique violation as the
at-most-once guard. Add `TestFailedEventIsRetryable`: a first delivery whose store call is made to
fail answers 5xx and leaves no `slack_events_seen` row; the retry with the same `event_id` creates
exactly one `rq-`.

### S19-I3 (Important): the part is two or three tasks wearing one id

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-182.md:45
**Problem:** the stated budget is 3 h, but the Files list is 12 new files plus 4 modified, and the
work is: a new provider-neutral `chat` package, a re-pointing of two *already-merged* steps (task
157's feed, task 165's digest) with their suites as the proof, request-signature verification, 8
routes, 5 tables and a migration, a person-link page with nonces, a webhook sink kind, golden
payloads, and 17 test rows. That is not "1-3 focused hours" by the plan's own house rule, and the
first thing to be cut under pressure is the cross-org and dedup care above.
**How to fix:** split on the seam the part already drew. **182a** — `chat.Poster`, `ClaimDigest`,
the re-pointing of tasks 157 and 165, and their unchanged suites (about 2 h). **182b** — inbound:
`verify.go`, `events.go`, `interact.go`, the two Slack-signed routes, the install/channel/person
tables and the link page (about 3 h). **182c** — outbound: `feeds.go`, `unfurl.go`, `digest.go`,
the `slack` sink and the canary row (about 2 h). Keep the acceptance commands whole on 182c and
give 182a and 182b their own.

## Task 183 — alert intake into `triage`: Azure Monitor first, then Sentry (L-209)

The strongest part in the section. Every scope-named test is present with a named mutant, the
two scope corrections (per-source `audience` / `secretRef`, `quoteTruncated` as a flag not text)
are real corrections and are argued rather than asserted, and SW.3 is held all the way down:
`Ingest`'s four cases write no state, the matrix row proves the alert principal is 403 on state,
assignee and priority, and `TestDeclinedStaysDeclined` keeps a person's decision. `als-` is
genuinely free — `05-design.md:816-821` and task 143's eleven at `tasks-141-160.md:131-138`
both check out. The 413 body cap before the parser, the UTF-8-boundary cut, the write-only
Sentry secret with a reflection test, and 401-not-404 on an unknown source are all right.

### S19-I4 (Important): `alert_sources` and `alert_links` carry no `org_id`, so the RLS they claim cannot attach

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-183.md:228
**Problem:** the migration ends "Row-level security follows task 06's pattern for team-owned
tables (its policy text wins)", but task 06's rule 2 is *"Every foreign key carries `org_id`, at
the same position as its target's"* (`task-06.md:19`) and every table it writes opens with
`org_id text NOT NULL REFERENCES orgs (id)` (e.g. `:136`, `:149`, `:158`). `alert_sources` has
`team_id text NOT NULL` with no `org_id` and no composite FK to `teams (org_id, id)`;
`alert_links` has neither, only `source_id`. With no `org_id` column there is no predicate for
an org policy to use, so the stated RLS is unbuildable and org isolation for alert intake falls
to Go alone — against the contract's "row-level security **and** Go"
(`2026-09-23-hub-contract.md:2081`). The same defect sits in task 182's five Slack tables
(S19-C1), so it reads as a section-wide habit, not a one-off.
**How to fix:** give both tables an `org_id text NOT NULL` first column, as every sibling table
in this plan already has (`task-06.md:136`, `task-116.md:190`), set it from the source's team
inside the write transaction, and copy task 06's policy text verbatim over both. Add one
integration row that runs the ingest under a second org's RLS role and sees zero rows.
(The composite-FK half of task 06's rule 2 is *not* what the plan builds in practice — task 116's
tables use single-column `REFERENCES users (id)` — so the missing `org_id` column, not the FK
shape, is the defect.)

### S19-M3 (Minor): the Azure Monitor bearer check lets any caller in the tenant fire a source

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-183.md:187
**Problem:** ingest accepts any Entra token whose signature, tenant and `aud` match the source's
`Audience`, and `Audience` is a free-text field a team `admin` sets. Any principal in the org's
tenant that can obtain a token for that audience can post alerts into that team's `triage`, and
`TestAzureWithoutBearer401` only covers absent, expired and wrong-audience. The part discloses
this under *does not claim*, which is honest, but leaves no shape for closing it later.
**How to fix:** add an optional `callerAppIds []string` to `AlertSource` (empty = accept any
caller in the tenant, as today), check `appid`/`azp` against it when non-empty, and record in
step 1 that the first real test fire fills it. One test: a token with the right audience and an
`appid` outside a non-empty list → 401, nothing written.

## Task 184 — triage rotation imported from PagerDuty or Opsgenie (L-218)

Decision 6 is the best piece of work in the section: it catches that task 121's passes run inside
the per-org write transaction (`task-121.md:18-21`) and moves the vendor call out of it, keeping
the session advisory lock, with `TestTwoSweepsMakeOneCall` as the proof. The *Gaps found while
writing this part* list is honest and specific (including task 114 vs task 121 disagreeing on
`NextHandoverOn`/`handoverOn`). `Match` is whole-string, lower-cased, current-members-only, with
`alice@kpmg.com.au.evil` named as the mutant; an outage keeps duty; the token is asserted absent
from errors, logs, events and `lastError`. Scope coverage is complete.

### S19-I5 (Important): the vendor credential is one per process, not one per org, so decision 9 is not what the interface builds

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-184.md:305
**Problem:** decision 9 says the credential is "one per org and vendor" — "an org admin sets it
once" and "a team whose org has no credential for the vendor gets 422
`rotation_source_unconfigured`". But the interface is `JanitorConfig` gaining
`Sources map[string]rotation.Source // by Kind`, and the sync's step 2 is `src :=
cfg.Sources[kind]; nil → the org has no credential`. Keyed by kind alone, one PagerDuty token
loaded into the Hub process serves **every** org: org B's team admin sets
`external {kind: "pagerduty", scheduleRef: <an id in org A's PagerDuty account>}` and the Hub
reads org A's schedule with org A's token. The emails only match org B's own members, so the
visible leak is bounded, but a credential is used across a tenancy boundary and a per-org 422 is
unimplementable against a process-wide map. `PANACEA_APP_HUB_OPSGENIE_BASE` as a single process
variable has the same shape (an org on Opsgenie EU forces every org onto the EU host).
**How to fix:** make it `Sources func(ctx context.Context, org, kind string) (rotation.Source,
error)`, resolving the org's `secret_ref` (and its Opsgenie base) per call, with "no credential
for this org and kind" as its own error that `putRotation` turns into the 422. Add
`TestAnotherOrgsCredentialIsNotUsed`: org A configured, org B not, a due source in org B → zero
vendor calls and `lastError` `vendor_rejected`.

### S19-M4: WITHDRAWN on verification

`ext_on_duty text NULL REFERENCES users(id)` looked like a breach of task 06's decision 2
("every foreign key carries `org_id`"), but task 116 — the owner of `work_triage_rotations` —
writes exactly that shape itself (`task-116.md:191`, `:196`: `team_id text PRIMARY KEY REFERENCES
teams (id)`, `updated_by text NOT NULL REFERENCES users (id)`), as does the Hub contract's own
`team_members` (`2026-09-23-hub-contract.md:1508-1517`). Task 184 matches its parent table. No
finding. The id is retired rather than reused.

### S19-M5 (Minor): 2 hours is not the size of this part

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-184.md:3
**Problem:** the budget is 2 h for two vendor readers with golden fixtures, a seven-column
migration with five CHECKs and an index, three new store functions plus a changed `PutRotation`,
a new `Sweep` step with its own lock reasoning, a `passHandovers` skip, two route changes, the
fake, a conformance row, a desktop segmented control and `dutyLine`'s five cases — 19 named
tests across three suites. The same under-budgeting sits on 182 (S19-I3).
**How to fix:** either raise the estimate to 4 h in both the part and the skeleton row
(`2026-09-25-phase-w-work-layer.md:616`), or split the desktop half (the choice, `dutyLine`,
part 3's four tests) into its own task depending on this one.

## Task 185 — Entra group to team mapping (L-238)

The row's UNVERIFIED is answered from the contract itself and every anchor checks out in
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md`: `:543`
`groupMembershipClaims: None`, `:1508-1517` `team_members`, `:2014` `GRANT UPDATE (role), DELETE`,
`:2211` "No Microsoft Graph, nothing else". Decision 1 (Graph over workload identity, never token
claims, dark behind `PANACEA_APP_HUB_GRAPH=off`) and decision 7 (pending joins in Go rather than
`CREATE OR REPLACE` on a reviewed `SECURITY DEFINER` function) are both real corrections to the
scope, argued rather than asserted. The "a partial page list is never returned" rule and
`TestAFailedSecondPageFailsTheRead` close the worst failure mode. The grant widening is named as a
Hub delta and taken to the `feat/hub` owner, and gap 3 proposes the threat-model row this needs.

### S19-I6 (Important): nothing prunes `pending_group_members`, so a person who left the group still joins at first sign-in

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-185.md:302
**Problem:** the sync's step 3 applies the plan as "each `Set` is an upsert …, each `Pending` an
upsert", and `Reconcile`'s `Pending` is only ever *added to*. No step deletes a pending row for an
oid the latest good read no longer lists, and `ConsumePending` at first sign-in
(`store.ConsumePending(ctx, tx, org, userID, oid)`) re-checks nothing — it "deletes the person's
pending rows and, per team, upserts the highest-ranked role". Failure: Priya is added to
`Engineering-AU` (role `admin`) on day 1 and never signs in; on day 3 she is removed from the
group; on day 40 she signs in and is made a team `admin` from a row that has been wrong for 37
days. `pending_group_members` is deleted only by the link's `ON DELETE CASCADE`. The contract's
`accountEnabled` guard does not help — she may be a live employee who simply left that group.
`TestAPendingPersonJoinsAtFirstSignIn` only proves the happy path, so the mutant survives.
**How to fix:** make `Plan.Pending` authoritative per link — in the write transaction, delete the
link's pending rows that are not in this run's `Pending` whenever `Err == ""` (and leave them all
alone when `Err != ""`, as decision 5 does for removals). Add
`TestAStalePendingRowIsDropped`: pending for Priya, the next sweep's read no longer lists her, the
row is gone and her first sign-in joins no team.

### S19-I7 (Important): decision 5 guards removal but not demotion, so a Graph outage silently drops an admin to member

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-185.md:236
**Problem:** `Reconcile` computes Want from "every enabled person in a link read **with
Err == \"\"**", and a group-sourced row is exempted from `Remove` when its source link failed —
but `Set` has no such exemption. Priya is `admin` through link B and `viewer` through link A; on a
run where B answers 503 and A succeeds, Want holds only A's `viewer`, her row is group-sourced
with a different role, so she is in `Set` and is demoted to `viewer` until B recovers. Decision 5's
own words are "nobody is removed on doubt", and a silent privilege drop from an incomplete read is
the same defect wearing a different verb; it also writes a misleading `team.member_set {role:
"viewer"}` onto the thread. `TestAFailedReadRemovesNobody` covers only `Remove`.
**How to fix:** extend the exemption to `Set`: when any link that currently sources a member, or
that granted them a higher rank at the last good read, has `Err != ""` this run, leave the row
untouched. Store the rank the last good read gave per link (the row already keeps `last_seen`, so
a `last_role` beside it is cheap). Add `TestAFailedLinkNeverDemotes`: two links, the admin one
returns 503 → the member keeps `admin`, zero events.

### S19-M6 (Minor): `GroupLink.CreatedBy` is a bare id where the rest of the section uses `User`

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-185.md:100
**Problem:** `CreatedBy string // u-…`, while every sibling wire type in this section embeds the
resolved person — `AlertSource.CreatedBy User` (task-183), `SlackInstall.InstalledBy User` and
`SlackChannelLink.LinkedBy User` (task-182), `TeamPolicy.UpdatedBy *User` (task-03:353). The
desktop's `GroupLinks.tsx` would have to render `u-7f3a…` or fetch separately, and no test catches
it.
**How to fix:** make it `CreatedBy User` and add it to the `GET` row's assertion, or state in the
part why a raw id is right here.

## Task 186 — the in-app changelog (L-254)

The most carefully verified part in the section: every desktop citation is real in this worktree —
`desktop/package.json:4` is `0.1.0`, `commands.ts:16` is `glyph`, `:72` is `buildCommands`, `:85`
is `verb:layout`, `commands.test.ts:45-46` is the block-order test splitting on `:`,
`store.ts:3-6` is the flat-store warning, `:16` `PERSIST_KEY`, `:17` `PERSIST_VERSION`, `:34`
`partialize`, `tsconfig.json:12` `resolveJsonModule: true`, `05-design.md:446` is L-254 *later*,
and neither config calls `mergeConfig`. The part catching its own palette-id mistake
(`whats-new` → `verb:whats-new`) before writing it is exactly the behaviour this plan wants.
Scope coverage is complete and the 23/27 test arithmetic in the acceptance block checks out.

### S19-I8 (Important): `import.meta.glob` reaches outside the Vite root, which the dev server will refuse

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-186.md:120
**Problem:** `load.ts` globs `"../../../docs/changelog/*.md"` — the path resolves correctly to the
repo's `docs/changelog/`, but that is **outside** the Vite root, which is `desktop/`. Vite's
default `server.fs.allow` is the detected workspace root, found by walking up for
`pnpm-workspace.yaml`, `lerna.json` or a `package.json` with `workspaces`; this repo has none —
the only lockfile is `desktop/package-lock.json` and there is no root `package.json` — so the
search falls back to the nearest package root, `desktop/`. An eager `?raw` glob outside that
directory is served over `/@fs/…` in dev and is blocked. `vitest` and `vite build` resolve the
files through Rollup and pass, so the whole test suite is green while `tauri dev` shows a failed
import: the one path the owner uses for visual acceptance is the one path not covered. The Files
list touches `vite.config.ts` only for `define`.
**How to fix:** add `server: { fs: { allow: [".."] } }` to `desktop/vite.config.ts` in the same
step 2 that adds `__BUILD_DATE__`, and make it explicit in the acceptance block that
`npm --prefix desktop run dev` (or `tauri dev`) must render *What's new* once, recorded by task
187 as an owner-eyes row. If the dev server turns out to serve it without the allow entry, keep
the line anyway and say so — it costs nothing and documents the reach.

### S19-M7 (Minor): an empty `docs/changelog/` crashes the view on mount

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-186.md:176
**Problem:** `loadFrom` returns `{ok: true, entries: []}` for a folder with no entry files (only
`no_items` guards an entry with an empty `items:` list, and `README.md` is skipped by name), and
`WhatsNew` "on mount … calls `onSeen(entries[0].version)`" with no empty guard — a `TypeError` on
`undefined.version` that takes the main area down. `the shipped changelog loads` asserts `≥ 1`
entry in *this* repo, which is not the same as the component being safe, and no row passes
`entries: []`.
**How to fix:** make `WhatsNew` render *No entries yet.* and skip `onSeen` when `entries` is
empty, and add the row `an empty changelog renders one sentence` (mutant: an unguarded
`entries[0]`).

## Task 187 — WL's proof and results

The mechanical claims all hold in this worktree: `grep -c "| later |" 05-design.md` really is
**12**, at exactly the twelve line numbers the part tabulates (`:111`, `:149`, `:309`, `:311`,
`:333`, `:334`, `:338`, `:378`, `:381`, `:390`, `:420`, `:446`); the four clause rows are at
`:164` (L-50 PARTIAL), `:204`, `:231`, `:289`; the Q1 list at `:499` holds L-152; the 188 forecast
is at `:480`; `## 1. The parity matrix: all 257 items` is at `:85` so both scripts' `/^## 1\. /`
fence matches and stops at `## 2.` (`:506`); `coverage.md:10` is the 27 SKIPs, `:11` the
`c5841bf` pin, `:22-23` the proof-tasks-are-never-closers rule, and `:163`, `:170`, `:209`,
`:274` are the four mixed-stop rows. Both shell checks read correctly under `set -e` (the failing
operand of each `&&` chain is exempt), the inner `read < <(…)` does not eat the outer loop's
stdin, and `US=$'\037'` genuinely stops an empty cell collapsing into its neighbour. The
verdict-vs-05 and recount-the-counts-line mutants are the two right ones to kill.

### S19-I9 (Important): four *later* halves have no task anywhere, and the ledger's `partial` rule lets that ship quietly

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-187.md:466
**Problem:** the part's own gap 1 is correct and load-bearing: L-120 (`exists / later`,
`coverage.md:163`), L-127 (`W1 / later`, `:170`), L-166 (`W3 / later`, `:209`) and L-231
(`exists / later`, `:274`) each have a designed-for *later* half that **no task in the plan
builds**, and the skeleton has no WL row for any of them. `OD-26` commits every item less the 27
skips, so this is a four-item coverage hole in the phase, not a formatting detail. The part's
answer is to file them `partial`, and `check-ledger.sh`'s rule for `partial` is only "*Note*
names the deferred part" — free text. So the phase closes, the check exits 0, and four committed
items are recorded in prose that nothing reconciles against `OD-26`.
**How to fix:** two changes. (a) Raise it to the plan owner as a skeleton decision before 187
runs: four WL rows, or four owner-recorded skips; the part should say which it is asking for,
not only that it noticed. (b) Tighten the check: a `partial` row's *Note* must name either a
task id (`^task [0-9]+`) or an owner decision id (`^OD-[0-9]+`), and add the fixture
`ledger-partial-no-owner.md` (the four rows with prose notes) whose expected exit is 1. Without
(b) the mutant "a deferred half with no plan, described in words" survives.

### S19-I10 (Important): the ledger is built from a map pinned before the parts it must reconcile were written

**Where:** docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-187.md:335
**Problem:** decision 6 and step 7 make `…-scopes/coverage.md` the ledger's source, and
`coverage.md:11` pins it to `feat/work-layer` at **`c5841bf`**, which the part's own gap 2 says
is "before parts 143-186 were written" — and then: "No task owns re-running it." So a row a part
closes that its *scope paragraph* did not cite files as `gap`, and the ledger under-reports the
phase in exactly the direction that hides work. This is also a deviation from the scope, which
asked 187 to find the closers live: *"found by `grep -n \"Closes\"`
docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/*.md"*
(`tasks-181-187.md`, the ledger paragraph) — the part substituted a pinned artefact for that
grep.
**How to fix:** make step 7 re-derive *Closed by* from the **parts** at the sync base
(`grep -n "Closes \*\*L-" docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/*.md`),
using `coverage.md` only as a cross-check, and add to `check-ledger.sh` a comparison of the two
lists that fails on any row the parts close and the map does not (mutant: a stale map turning a
built row into a `gap`). If the map is meant to stay authoritative, then give some task the job
of re-running it and name that task here.

Section verdict: Needs fixes
## Section 90: OD-30 identity (orchestrator)

Source: OD-30 (`docs/superpowers/planning/2026-09-23-owner-decisions.md`, main tree) and its ruling,
main tree `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` (below: "ruling §N").
People sign in through a Keycloak realm brokering Entra; the issuer is `PANACEA_APP_OIDC_ISSUER`
(ruling §2); the Hub validates realm tokens with `oidcauth` under `ProfileKeycloak`, audience
`panacea-app-hub` (ruling §5). Paths are relative to this worktree; `scopes/` means
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/`. The person key `(tid, oid)` is
unchanged (ruling §4), so no Work row keyed on `entra_oid` changes.

### ID-I1 (Important): the Work MCP server names Entra as its authorization server
**Where:** `scopes/tasks-161-180.md:38-46`, `:70`, `:78-83`, `:530-535`;
`docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:257`, `:582`
**Problem:** Task 161's RFC 9728 document lists
`authorization_servers:["https://login.microsoftonline.com/<tid>/v2.0"]` and
`scopes_supported:["api://<hub-client-id>/Hub.Access"]`, and validates with the Hub's
`entra.Validator` (`aud` the Hub, `scp` holds `Hub.Access`). Under OD-30 the Hub's issuer is the
realm and `Hub.Access` no longer exists (ruling §4); an MCP client following this document would
fetch a token the Hub refuses.
**How to fix:** `authorization_servers:[<PANACEA_APP_OIDC_ISSUER>]` read from config, never a
literal host (ruling §2, "the issuer is configuration"); `scopes_supported:["panacea-app-hub"]`;
validation by `oidcauth` under `keycloak`: `aud` contains `panacea-app-hub`, `scope` contains
`panacea-app-hub`, `azp` an approved app's realm client id (ID-I2), service tokens refused. Tests:
the document's `authorization_servers[0]` equals the configured issuer (kills a hard-coded host);
the `:70` mutant becomes "any realm client accepted". Rewrite the UNVERIFIED at `:81-82` and
`:533`: each named MCP client is registered as a realm client (task 159); the ruling enables no
dynamic client registration.

### ID-I2 (Important): third-party apps are realm clients, not Entra OAuth apps
**Where:** `scopes/tasks-141-160.md:162`, `:196-200`, `:746-760`; `scopes/tasks-161-180.md:623-625`
(task 178's connector); `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:583`
**Problem:** Task 159's `App.entraClientId` and manifest `entraClientId`, "Entra OAuth apps", the
`AppClients` hook on `entra.Validator` and "App-only tokens (`roles` with no `scp`) stay refused"
all assume Entra issues the app's token. Under OD-30 an app that acts as a person gets a realm
token with its own `azp` (ruling §4); Keycloak's service tokens are `Service` true (ruling §5).
**How to fix:** Rename `entraClientId` -> `clientId` (a realm client id string, not a GUID). The
hook reads `oidcauth.Claims.ClientID`; "app-only tokens" -> "service tokens (`Claims.Service`)
stay refused, so an app never acts without a person". The app's realm client is created by the
realm's admin (Panacea's team now, ours in phase 8, ruling §9-§10); the Hub's `pending`/`approved`
approval stands. Ruling §10 lists no app clients: the part says so and marks that step
UNVERIFIED. Task 178's connector uses realm OAuth through the same approved-app path.

### ID-I3 (Important): Hub web pages sign in by Entra auth code; they sign in through the realm
**Where:** `scopes/tasks-61-80.md:678`, `:691`; `scopes/tasks-141-160.md:345-356`;
`scopes/tasks-181-187.md:107`, `:114`; `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:254`, `:569`
**Problem:** Task 80's browser session ("signed in by Entra auth code into an `HttpOnly` ...
cookie"), task 148's "Entra-protected" form and task 182's page "behind Entra" name Entra as the
issuer. Ruling §11: Hub pages sign in through the realm.
**How to fix:** The page session runs auth code + PKCE at the realm's `authorization_endpoint`
(from discovery of `PANACEA_APP_OIDC_ISSUER`) with `kc_idp_hint=entra`, so a browser already
signed in to Entra sees no click (ruling §2 step 4, §3); the session's person is the id_token's
`(tid, oid)`. "Entra-protected" -> "realm-protected (Entra through the broker)". This needs a realm
client for the Hub's pages, which ruling §10 does not list: the part names it as a new ask for
Panacea's realm and phase 8's realm file, marked UNVERIFIED. The cookie rules (no `/v1` route
accepts it, CSP, CSRF) stand.

### ID-I4 (Important): group mapping cites a Hub API registration that no longer exists
**Where:** `scopes/tasks-181-187.md:239-249`; `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:617`
**Problem:** Task 185 reasons from "the Hub API registration has `groupMembershipClaims: None`".
OD-30 makes no Hub API registration (ruling §1); the realm token carries no `groups` (ruling §4).
Ruling §4 adds no groups mapper and says Hub membership is the Hub's own; ruling §11's work row
says `groups` stays absent.
**How to fix:** Keep the Graph read under the Hub's workload identity: that is Azure-plane access,
which ruling §6 leaves unchanged, and its user key `entra_oid` is the realm token's `oid` (§4).
Replace the registration sentence with "the realm token carries no `groups` (ruling §4)". A
broker-mapper source for groups would add a mapper §4 does not list: raise it with the owner
rather than build it.

### ID-I5 (Important): sign-in and validator wording still names Entra and `internal/entra`
**Where:** `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:127`;
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-parts/task-17.md:33`, `task-22.md:16`,
`task-07.md:578`; `scopes/tasks-01-20.md:377`, `:793`
**Problem:** "signs in with Entra PKCE" (OD-6 row), `server.New` + `entra.New`, "that person's
Entra token from Credential Manager", "Entra issues an integration principal", "a real Entra
tenant (H43's checklist)".
**How to fix:** OD-6 row: "signs in through the Keycloak realm brokering Entra, PKCE, zero clicks
(OD-6, OD-30)". Task 17's world: `oidcauth.New` with `Profile: ProfileKeycloak` and devissuer's
`keycloak` profile; `AllowInsecureAuthority` now also admits the loopback http issuer (ruling §5).
Task 22: Credential Manager holds the person's realm **refresh** token; the Hub client sends a
single-audience `panacea-app-hub` access token held in memory (ruling §2 step 7). "Entra issues"
-> "the realm issues". H43's check becomes ruling §10's check against Panacea's dev realm.

### ID-I6 (Important): Azure Monitor's secure webhook sends an Entra token, which no realm validator accepts
**Where:** `scopes/tasks-181-187.md:165-168` (task 183)
**Problem:** Task 183 authenticates Azure Monitor alerts as "a secure webhook, an Entra bearer for
the app the source names". Azure Monitor cannot obtain a realm token, and the ruling is silent on
inbound tokens from Azure services.
**How to fix:** Owner call before task 183 is written; the part must not invent one. Either (a) a
second `oidcauth` instance under `ProfileEntra` for that one ingress route, which needs an Entra
app registration and so an IT ask the ruling does not list; or (b) Azure Monitor's plain webhook
with a secret path token, like Slack's and Sentry's signature routes. Record the choice as UNVERIFIED.

Checked, no change: `(tid, oid)` mapping of host and Teams identities (`scopes/tasks-141-160.md:23`,
`:276`; `scopes/tasks-161-180.md:243`); the Hub's workload identity for Graph mail and Azure Boards
(`scopes/tasks-141-160.md:389`, `:472-473`), Azure-plane per ruling §6. Users' own Azure DevOps
access is a personal access token (ruling §7); no Work task relied on an Entra ADO token.

Section verdict: Needs fixes
**Plan quality: Needs fixes**

---

# Fix batch b01 (2026-09-28)

One finding, **X-C1**, fixed at its root: the link is built, not the Goal struck. Every line number
below is post-fix, on `feat/work-layer`.

| Finding | Verdict | Change | File:line |
|---|---|---|---|
| X-C1 | Fixed, and narrower than written | phase 5 row in *Deltas on sibling code* | `2026-09-25-phase-w-work-layer.md:289` |
| X-C1 | " | task **81b**, the `implements` write and read paths and the design-approval close (W2, WH, 4 h) | `…-work-layer.md:489` |
| X-C1 | " | task **96b**, the Design lens's issue list and *implements* on the issue view (W2, 3 h) | `…-work-layer.md:505` |
| X-C1 | " | `P5-` added to the `dependsOn` legend, cited against phase 5's task table | `…-work-layer.md:356` |
| X-C1 | " | SW.1 records that `implements` is already in the wire; SW.4 gains 81b | `…-work-layer.md:375`, `:378` |
| X-C1 | " | totals 187 → 189 tasks, 516 → 521 h; W2 26 → 28 tasks; streams and scope-file rows | `…-work-layer.md:110`, `:346`, `:351`, `:636` |
| X-C1 | " | file map: `work_implements.go`, and `Implements.tsx` in the issue view | `…-work-layer.md:246`, `:308` |
| X-C1 | " | the two scope paragraphs, the vocabulary ruling and the owner fork | `…-scopes/tasks-81-100.md:529-687` |
| X-C1 | " | the two parts | `…-parts/task-81b.md`, `…-parts/task-96b.md` |
| X-C1 | " | why `implements` is **not** a `RelationKind` | `…-parts/task-11.md:555-572` |
| X-C1 | " | 81b and 96b close no parity row; the counts do not move; L-208 stays Figma's | `…-scopes/coverage.md:351-367` |

**The finding was right about the hole and wrong about the wire.** `implements` is not missing:
`IssueRevisionContent.Implements []string` (`design:…` keys) is declared at `task-02.md:264`, is
inside the canonical bytes every revision digest covers, is in all three golden vectors
(`task-02.md:458`, `:507`, `:548`), and is on the API type at `task-03.md:424` with its field-name
row at `task-03.md:738`. So **SW.1 is not reopened and task 05 is not reopened** under the WS0 rule.
What was missing is everything that *uses* the field: no write path, no reverse read, no surface, no
close path for 04 §7.7, no phase 5 delta row and no `P5-` id. That is what 81b and 96b build.

**Vocabulary, settled rather than left open.** 05 says *frame*; phase 5's document model word is
**`Page`** (`P5:359`; round 1 wrote *artboard* — wrong, see RR-M1), and its design key
`design:<canvasId>@<rev>` names a **canvas revision, not a subset**
(phase 5 plan `:232`, decision P5-H). So `implements` names a canvas revision — which is exactly the
shape `task-02.md:264` already had — and there is no per-frame key to invent. The Design lens lists
the canvas's issues; a frame is a locator inside the revision.

**Owner fork, recorded not built (rule 8).** The owner may instead rule Design out of Phase W. Then
strike *design frame* from the Goal (`…-work-layer.md:7`) and delete the two tasks, the phase 5 delta
row and the `P5-` legend entry. The default taken is the root-cause option, because `CLAUDE.md`
calls the thread the product spine, the Goal names the hop, and 04 §7.7 already sizes the work —
*"about 0.5 day, later"* (05 `:1305`). W2 **is** that *later*; nothing in W1 needs it. The 5 h here
is 1 h over 04's estimate, and like every size in this plan it is an estimate, not a measurement.

**Ids.** `81b` and `96b` are letter-suffixed, as the Hub plan's `H07b` is. Renumbering 72-187 would
have broken this plan's ~1,678 cross-file `task-NN.md:LINE` citations.

**Citations re-anchored.** Three rows were inserted into the skeleton — at old lines 288, 486 and
501 — so every inbound citation below old line 289 is unmoved and every one above shifted by 1, 2 or
3. They were found by `grep -rno "phase-w-work-layer\.md:[0-9]*"` — **which was the wrong scan; round 2
below replaces it** — and rewritten by the shift `new = old + (old≥289) + (old≥487) + (old≥502)`, ranges
included, both ends. **24 citations in 5 files** moved: `task-109.md:794`, `task-39.md:603`,
`task-62.md:4`, `task-97.md:8` and 20 in this file. `task-62.md:10` (`:100`) was below the first
insert and is untouched; the skeleton cites itself nowhere (`grep -c` returns 0); nothing outside
`docs/superpowers/plans/` cites it.

**Unverified.** Phase 5's `internal/designapi/` and `src/design/` paths are read from its plan on
another branch, not from a built tree, and both parts mark them so. `P5-21`, `P5-27` and `P5-28` are
ids from that plan's task table, read 2026-09-28; no phase 5 task is built. Nothing here was
executed: this is a plan change only.

## Fix batch b01, round 2 (2026-09-28) — RR-C1, RR-C2, RR-C3, RR-I1, RR-I2, RR-M1, RR-M2

Round 1's re-review (`.superpowers/sdd/work/plan-rereview-b01-1.md`) returned **Needs fixes**: the
hole was correctly diagnosed and the bookkeeping was clean, but the close path was not buildable and
the citation re-anchor had used an incomplete scan. Both are fixed at the root here.

### The close path: an approval is a landing, not a citation (RR-C1 + RR-C2)

Round 1 closed a design issue by moving it to `done` with cause `observed`. Two independent walls:

1. `MoveAllowed` allows an evidence cause only when `from == StateLanded` (`task-10.md:275-281`). A
   design issue has no land commit, so it sits in `todo` or `started` and the Hub refuses the move
   422 `state_not_allowed`.
2. The done rule's input is `Cited{BriefKey, CriterionID, Cause, RecordedBy}` — **keyed on a brief
   criterion**. An approval names a design key, and nothing in any phase maps one to the other:
   phase 4's `Criterion` is `{ID, Text}` with no class, and the phase 5 plan contains the string
   *criteri* **zero** times, so a canvas revision does not record what it serves. Round 1's *"one
   design criterion and one code criterion"* test named a distinction that is not representable, and
   its *"done, proven by approval `design:…@rev`"* was a seventh `ProofWord` that does not exist.

Making that path work needs synthesised `Cited` rows — fabricated proof, which is what `OD-27`
exists to forbid. So the design approval now does what a **land trailer** does: it moves the issue
to `landed` with one new cause, `design_approved`, and **task 10's rule 6 decides the word with no
edit at all**:

- `serves: []` (a `notTiedReason` design chore) → task 10's `no_criteria` branch lands it **at
  `done` in the same write**, unless the team's `stopAtLandedWithoutServes` holds it at `landed`
  (`task-10.md:52`, `TestNoCriteriaLandsAtDone` at `:462`). *This is the case 04 §7.7 means by "a
  design approval can close a design issue", and it closes.*
- an issue that serves criteria → `landed`, `proof: waiting`, until each criterion gets its own
  evidence. The approval got the design in; it did not prove the brief.

What that buys: no seventh proof word, no synthesised citation, no invented criterion class, and
`DecideProof` is byte-identical — a test in 81b now asserts exactly that. The new `MoveAllowed` arm
ends at `landed`, **never** `done`, and omits the terminal states, so no existing arm is loosened.
`landCommit` stays `""`; the pair (`landed`-or-beyond, empty `landCommit`, non-empty `implements`)
**is** the design-approved issue, and no field was added to say so.

The added `StateCause` is declared rather than discovered: the WS0 rule (skeleton `:366`) permits an
added constant, and 81b now names the three other places the word must appear — task 04's two cause
switches (`task-04.md:876`, `:1009`), `MoveCauses` (`task-03.md:220`) and task 28's glyph set.

### The call site, and the writer (RR-I2 + RR-I1)

The close ran *"inside the Hub's design-approval write"* with no file named. That write is **Hub task
22's** `control-plane/internal/hub/server/approvals.go` (hub plan `:304`, `:425`; hub contract §7.7,
routes 30-32). It is now in 81b's Files with `⚑`, `dependsOn` reads `10, 23, 74; H22; P5-21`, and
the skeleton's delta row names the Hub file instead of phase 5's `internal/designapi/approve.go` —
which is that route's **caller**, not the writer, and which 81b does not touch.

`implements` also had no writer anywhere in 189 tasks: every one of the plan's twelve write sites
sends `[]`, so round 1's read route, close path and *Issues* group rendered nothing for a real user.
96b now builds `desktop/src/components/work/DesignPicker.tsx` on task 42's `Picker`, the same way
`CriterionPicker` is built, under task 47's decision 3 — adding or removing a key writes a **new
revision, never a patch**. If it is cut, the field stays `[]` and the Design hop stays dark; the
part says so.

### Citations: the other form, and the method b02/b03 must use (RR-C3)

Round 1 scanned `grep -rno "phase-w-work-layer\.md:[0-9]*"` and fixed 24. The plan's **other**
shorthand for its own skeleton is ``skeleton `:N` ``, and none of those moved — `task-161.md:645`
and `task-164.md:7` were left pointing at a **different task's row**. **40 citations** are shifted
here, across 18 parts and this review, each new target read on the skeleton first.

**The corrected method, which supersedes the one above.** Scan **line-level, not by proximity**:
every line that mentions the skeleton has all of its `` `:N` `` listed, then each is classified by
hand. A 24-character proximity window misses `:4852`, where the citation follows a quoted phrase.
Classification matters as much as the shift — three citations round 1's re-review called stale are
not the skeleton's: `:2176`'s `:624`/`:590-592` are lines in `task-53.md`, `:2581`'s `:330-332` is
phase 4's plan quoted out of skeleton line 280, and `:2905`'s `:487` already points at task 81's
row. The scanner and the applied shift are kept at `.superpowers/sdd/work/cite-scan3.sh` and
`cite-shift.sh`; **b02 and b03 run those, not the grep above.**

### Minors and bookkeeping

`P5:359` names `Page, Node, Token, Asset, Thread, ReviewRequest, Author, Document` — phase 5's model
word is **`Page`**, not *artboard*; 81b and the scope both say so now (RR-M1). `task-67.md:368` no
longer files Design's `implements` under *later* — it is task 96b (RR-M2). The added call site,
cause constant and picker move W2 from 74.5 to **76.5 h** and All from 521 to **523 h** (9.6 and
65.4 days); task and parity counts are unmoved, and the skeleton is still **640 lines**, so this
round shifts no citation of its own.

**Still unverified.** Phase 5's paths, and now the Hub's `server/approvals.go`, are read from plans
on other branches, not from built trees. Nothing here was executed: plan change only.

---

## Fix batch b01, round 3 (2026-09-28)

Round 2's re-review (`.superpowers/sdd/work/plan-rereview-b01-2.md`) closed round 1 and left **2
Critical, 3 Important, 1 Minor** on the two new tasks. All six are answered below. Every line number
is post-fix on `feat/work-layer`; the skeleton's `:488` and `:504` in the round-1 table above became
`:489` and `:505` when another stream inserted the `23b` row at `:425`, and are corrected there.

| Finding | Verdict | Change | File:line |
|---|---|---|---|
| RR2-C1 | Fixed | decision 5: task 74's W2 migration replaces `guard_issue_state()`, both new arms gated on `NOT v_serves` | `…-parts/task-81b.md:111-130`, the file at `:178-187` |
| RR2-C1 | " | the two task-07 rows that move, and the seventh cause in its published set | `…-parts/task-81b.md:120-130` |
| RR2-C2 | **Ruled, and half of round 1's ruling withdrawn** | decision 2: an approval lands a design chore and **moves nothing** on an issue that serves criteria | `…-parts/task-81b.md:60-95`, test 2 at `:218-226` |
| RR2-C2 | " | decision 3: no `issue_proofs` row; the approver is on the event's `who` | `…-parts/task-81b.md:96-108`, test 3 at `:227-230` |
| RR2-I1 | Fixed | `task-37.md:99-100`'s union and `task-41.md:260-267`'s `PHRASE`, not task 28's glyph set | `…-parts/task-96b.md:53-62`, `…-parts/task-81b.md:31-37` |
| RR2-I2 | Fixed, with proof | `81b` gains `H22` and 4 h; `96b` 3 h; the rows now sum to the stream table slice by slice | `…-work-layer.md:489`, `:505` |
| RR2-I3 | Fixed | the picker is a validated typed-key field until phase 5 merges; no invented route | `…-parts/task-96b.md:42-48`, tests at `:92-98` |
| RR2-M1 | Fixed | decision 6: the cause appends in `init()` (task 144's pattern); `want["MoveCauses"]` 6 → 7 | `…-parts/task-81b.md:131-136`, the file at `:160-165` |
| — | Checked, no change | `coverage.md`'s b01 block: 81b and 96b still close no row and the 257 counts still do not move | `…-scopes/coverage.md:351-367` |

**RR2-C2 is a ruling, not a repair, and it is declared as one (rule 8).** Round 1 ruled *an approval
is a landing*, which is right for a design chore and wrong for an issue that also serves brief
criteria, for two reasons that are task 10 rules rather than preferences: a later `land_trailer` at
a different commit on an already-`landed` issue is 409 `state_conflict` (`task-10.md:56-60`,
`:413-416`), so a design landing locks the Merge lens out until a human reopens the issue; and
`issue_proof_citations.land_commit` is `NOT NULL` 40-hex (`task-10.md:137`), so *landed, waiting*
with no commit is terminal, not waiting. The two alternatives the re-review offered — a tenth
`design_landed` state, or carve-outs in task 10 rules 3 and 7 — both reopen SW.4 for a case no
acceptance criterion asks for. The narrow form reopens nothing, and the SQL guard's `NOT v_serves`
makes the database enforce it.

**The arithmetic, since RR2-I2 was an assertion both earlier rounds got wrong.** Summing `(N h)`
over every task row of the skeleton: W0 1/2, W1 70/195, W1b 1/3, W2 28/76.5, W3 16/46, W4 29/79.5,
W5 32/91, WL 13/33 — **190 tasks, 526 h**, each slice equal to its stream row at `:344-351`. The
scanner is `.superpowers/sdd/work/sumhours.py`; re-run it after any task row changes.

**Citations.** The 81b and 96b scope block grew from `529-623` to `529-687`. Every inbound
`tasks-81-100.md:N` citation in the plan tree points at `N ≤ 486` (32 found, none in the shifted
region), so **no plan citation moved**; the owner fork stayed at `:554-559`. `task-81b.md` and
`task-96b.md` have **no** inbound `:N` citation anywhere in the plan tree, so their inserts are
free. The three stale ranges are in the re-review reports under `.superpowers/`, which are dated
records and are left as written.

**The owner decision this batch does not take.** X-C1's fork stands unchanged and is the only thing
here that needs the owner: **is Design in Phase W at all?** The default taken, per rule 8, is the
root-cause option — build the link, 7 h across 81b and 96b, both in W2, nothing in W1 waiting on
them. Ruling Design out instead means striking *design frame* from the Goal (skeleton `:7`) and
deleting the two tasks, the phase 5 delta row (`:289`) and the `P5-` legend entry; the undo is
written at `…-scopes/tasks-81-100.md:554-559`. Nothing else in the plan depends on the answer.

**Still unverified, and it has not changed.** Phase 5's paths and the Hub's `server/approvals.go`
are read from plans on other branches. Nothing was executed: plan change only.
