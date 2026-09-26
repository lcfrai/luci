# 06 — Critique: product (adversarial)

**Date:** 2026-09-25. **Reviewer:** adversarial product critic (workflow agent). **Reviews:**
`05-design.md` and `00-summary.md` in this folder, checked against `01-linear-feature-inventory.md`,
`02` to `04`, OD-1 to OD-20 (`../2026-09-23-owner-decisions.md`), `AGENTIC-UX.md`,
`DESIGN-SYSTEM.md`, `CLAUDE.md` and the team-mode design (`../2026-09-24-team-collaboration/`).

**Clean room.** No Linear or Plane page was opened for this critique. Every Linear fact below is
taken from 01 or 02 (their URLs and read dates are theirs, all 2026-09-25) and is only as verified
as those files say. Anything they mark UNVERIFIED stays UNVERIFIED here.

**Checked by script, 2026-09-25.** The matrix in 05 §1 has 257 rows, numbered 1 to 257 with no
gap, and counts MATCH 155, EXCEED 75, SKIP 27, with first slices W1 42, W2 52, W3 16, W4 37,
W5 36, exists 22, other phases 13, later 12. **The arithmetic is right.** What the numbers are
used to claim is the problem.

## Verdict

**Not ready for the owner as written. The direction holds; the promises attached to it do not.**
The core call (issues as Hub objects that point at brief criteria, so work sits on the thread) is
right and should stay. Four things would mislead the owner's two decisions (Q1, Q2):

1. The parity headline counts what W1 to W5 would deliver, but the recommended target (W1 to W3)
   delivers **135 of 257**, and about 28 of the MATCH and EXCEED rows are only partial.
2. W1 is called small and lovable. It needs five unbuilt phases and a live cloud environment, and
   05's own Q2 calls it "a demo".
3. The "~64 focused days" for W1 to W3 leaves out team mode's TM1 (**≈95.5 days** in TM-05
   revision 3), which W3 cannot ship without. It is also not sequenced against OD-17.
4. **"Done means proven"**, the headline "more", gives wrong states in the common case (several
   issues serving one criterion) and can be bypassed three ways.

All four are fixable in the documents, in about a day, without changing the design's direction.

---

## Critical

### C1. W1 is neither small nor self-contained: its demo depends on five unbuilt phases

**Where:** 05 §5.3 (the demo), §5.1 (the "Depends on" column), 00 "Delivery".

The W1 demo: *"Press `C`, type a title, pick the criterion it serves. Press `Shift+S`. Within 10
seconds a cloud agent is working on it … Deploy, and the issue says shipped to production."*
Each clause needs something 05 either leaves out or counts as outside the 27.5 days:

| Demo clause | Needs | In W1's "Depends on"? |
|---|---|---|
| "pick the criterion it serves" | a **complete** brief, because only a complete revision is inherited or pinned, and "complete" means all five questions settled (04:255). So the user must finish Define first | phase 4 tasks, yes. **The user's own prerequisite is not stated** |
| "a cloud agent … within 10 seconds" | phase 8's environment with Kata warm pools. It is "never applied until the owner provides the environment" (PROGRAM.md:64; owner-decisions:56) | **No** |
| the agent reaches a model | phase 6's gateway. OD-19 retires `/login` and personal subscriptions, so a cloud agent has no inference without it | **No**. 05 says W1 excludes "anything that needs the gateway (W4)", but the agent itself needs it |
| "it lands" | phase 3 tasks 05 and 09 (the `feat/phase-3-git` worktree is in flight) | yes |
| "shipped to production, 14:02" | phase 7, which calls Panacea's API (OD-13) | task 19, yes |

**Why it matters.** 00 tells the owner *"W1 is about 3 weeks elapsed on three streams"*. That is
the size of the increment, not the time until the demo can run. TM-05 sizes the gateway's
deployment at 8.5 days and the phase 8 work that cloud agents need at 14.5 days, on top of phase
8's existing plan (TM-05 §12.1, streams A and C). Neither appears in any W-slice.

**Fix.** Give W1 a full "ready when" line: Hub through task 43, phases 3, 4 and 7 at the named
tasks, **phase 6 stream A and phase 8 stream C (or local mode)**, and a finished brief. Alternatively,
make W1's demo run in local mode (OD-20 keeps it supported) and say so. Put the elapsed time to the
first demo in 00, not only the increment.

### C2. The recommended target omits TM1 and is not ordered against OD-17

**Where:** 05 §5.1 (W3 "~11.5 d; ships with TM1"), §5.7, Q2; 00 "Decide these two first".

- **The number is stale.** 05 read TM-05 while TM-05 was being revised (05 §7.1 item 6). TM-05
  revision 3 now puts TM1 at **≈95.5 focused days**, about 7 to 8 weeks on five streams, behind a
  two-week TM1-α spike on the real cluster (TM-05:20, §12.1). W3 cannot ship before TM1 does.
  So the owner is asked to approve "W1 to W3, ~64 days" when the elapsed path to the cross-owner
  magic is W1 + W2 + **TM1** + W3. The magic is the part the owner asked for by name.
- **The order is not reconciled.** OD-17 says team mode is built *"straight after the current
  wave, as the product's next phase"*. 05 §5.7 puts W1 "after Hub task 43" and W2 next. That is
  about 52 focused days on the same Hub and desktop streams that TM1's streams B and F need. 05
  never says whether Phase W runs before TM1, beside it, or after it. Only the owner can reorder
  OD-17.
- **Two Desk builds.** W1 row 13 builds a minimal Desk (3 d), and TM1 stream F builds "the Desk
  rows" (TM-05:1264). Q4 notices the gap but does not say which one builds on the other.

**Fix.** Add one table to 00 and to 05 §5.7 with the elapsed path for each stopping point,
**including TM1, phase 6 and phase 8**. Put the order against OD-17 to the owner as its own question:
"W1 before TM1, W1 inside TM1's streams, or TM1 first". Recommend one. **Suggested:** build W1's Hub
records inside TM1's stream B, and W1's desktop surfaces inside stream F, so that TM1 is also W1's
cloud path.

### C3. The parity headline overstates what the recommended build delivers

**Where:** 00 "The answer" (*"MATCH 155 · EXCEED 75 · SKIP 27"*), 05 §0 item 2, §1 tally.

The owner asked for *"everything they are doing"*. 00 answers with 230 of 257 matched or
exceeded, and then recommends stopping at W1 to W3. Counted from 05's own slice column:

| Stop at | MATCH or EXCEED whose first slice is reached | Of which only "exists" in unbuilt in-flight phases, or TM |
|---|---|---|
| W1 to W3 (recommended) | **135 of 257 (53%)** | 22 exist + 3 TM |
| W1 to W5 | 218; plus 12 *later* rows | as above |

Missing at the recommended stop: every importer (L-220), the roadmap and timeline (L-89 to
L-92), initiatives (L-83 to L-88), project updates (L-76 to L-79), insights (L-150, L-151), GitHub
for work done outside the product (L-199, L-202, L-203, and the outside-PR halves of L-200 and
L-201), Slack and Teams (L-206, L-207), and the public API's SDK, OAuth apps, webhooks and limits
(L-249 to L-252).

**About 28 rows marked MATCH or EXCEED are only partial.** Either a named part of the Linear item
is deferred, or the row rests on an UNVERIFIED plan:

- Deferred part: L-8 (no added custom statuses, see I4), 41, 50, 67, 72, 74, 83, 94, 98, 115, 116, 118,
  119, 120, 127, 138, 156, 166, 208, 220, 231, 235, 236, 237, 239.
- UNVERIFIED plan: L-164, 204, 240.

The clearest overclaim is **L-231, marked EXCEED**. Today we have Windows only and no tabs, and the
macOS and Linux builds are unmade (**UNVERIFIED**). A Linear user on macOS has nothing.

**Fix.** Add a PARTIAL verdict, recount, and show in 00 the parity reached **at each stopping
point**, not only at W5. "Everything they are doing" should be a row in that table (W5, ~117
days plus dependencies), so the owner can see the price.

### C4. "Done means proven" gives wrong states in the common case, and can be bypassed

**Where:** 05 §3.3 (the rule), §2.1 point 4, §2.3, §3.6 (authorisation), Q9; 04:264, 04 §3.3.

The rule: *"An issue with a non-empty `serves` moves to `done` by itself only when … every served
criterion [is] settled by recorded evidence at a commit containing the issue's landed work."*
This rule ties **an issue's** completion to **a criterion's** settlement. 04 states the normal
case plainly: *"Several issues serve one criterion; a refactor serves none"* (04:264). As written:

1. **Siblings hold each other hostage.** EP-139, EP-142 and EP-150 all serve `c-91e2`. EP-139
   lands first, but the criterion cannot settle until all three have landed, so EP-139 sits in
   `landed` for days. Cycle convergence shows it as unfinished, rollover (L-60) carries it
   forward, and its assignee's Desk has nothing to act on.
2. **Early evidence closes irrelevant work.** If `c-91e2` was already settled at an earlier
   commit, the next issue that serves it and lands turns `done` at once, whatever it did.
3. **An edited criterion can never close its issues.** A criterion's id lives only while its exact
   text survives (04:263). Editing the brief line gives it a new id, so every issue still serving
   the old id waits forever. 04 names this state **drifting** (04 §3.3), but 05 §2.3 keeps only
   three states (settled, open, contested) and drops *uncovered* and *drifting*.
4. **Three bypasses.**
   - A person may still move an issue to `done` by hand (§3.3 lists "a person" first; §3.6 allows
     it), and nothing says that is forbidden or shown differently on an issue that serves criteria.
   - Two-way sync (Q9, W5) lets Jira or Azure Boards write `done` with no evidence.
   - In solo mode the person who filed the issue and started the agent also presses **Record** on
     the agent's proposal. "Proven" then means one click by the same person. 05 names this risk
     ("rubber-stamp") and leaves it open.
5. **Small fixes get more friction than in Linear.** An issue with an empty `serves` stops at
   `landed` until a person closes it. Linear closes it on merge. Most bugs and chores will have
   an empty `serves`, so most issues get an extra manual step.

**Fix.**
- Separate the two facts. **An issue is `done` when its own work is landed and its own evidence
  is recorded.** A criterion is *settled* when all of its evidence holds at the head.
- Show both on the issue: *"done · c-91e2 still open (EP-150 not landed)"*.
- Keep *drifting* and *uncovered* from 04.
- Make a hand-set `done` on a serving issue carry a reason and show as **"done, not proven"**.
- Never let sync write fact states or `done`.
- Close empty-`serves` issues at `landed` by default, as Linear does at merge, with a team switch.
- Let evidence be an **observed fact** where one exists: a test run the platform watched, with its
  exit code at the landed commit (AX §8.1). Keep the click only where a person must attest.

---

## Important

### I1. What a Linear power user misses on day one (W1)

W1 leaves out, on purpose (05 §5.3, "Out of W1"), most of what a Linear user touches every hour:

| Missing in W1 | Linear item | Arrives in |
|---|---|---|
| **Import of their Linear, Jira or Azure Boards backlog.** Without it a team cannot switch | L-220 | **W5**, the last slice |
| Comments, mentions, reactions | L-41, L-42 | W2 |
| A Markdown description editor | L-126 | W2 ("The editor", 05 §5.4) |
| Cycles, sub-issues, estimates, due dates, saved views | L-57, L-20, L-16, L-37, L-93 | W2 |
| Linking PRs they make **outside** the product (local mode, their own IDE) | L-199, L-202, L-203 | W5 |
| Notifications beyond the Desk (email digest, Teams) | L-116 | W2, W5 |
| A link that opens for a colleague without the app | L-98, L-233 | not planned (see I10) |
| An assistant in solo mode (Linear's `Ctrl+J`) | L-155 | TM (team mode only) |

05 itself says *"W1 alone is a demo"* (Q2), yet §5.3 is titled "minimum lovable". **Both cannot be
true.** A Linear user will love *Start → it lands → done*, but not in a tracker with no comments,
no Markdown and an empty backlog.

**Fix.** Either rename W1 "the demo slice", or reshape it into a slice a team could adopt.
**Suggested:** move comments and the editor from W2 (2 d + 2.5 d), and **Linear and CSV import
into `triage`** from W5 (1 d), into W1. That is about 5.5 d at 05's own sizes. Move *shipped to production* (row 6, phase 7, 1 d) to a
W1b, so W1 does not wait on phase 7. The
first thing a Linear user sees is their own backlog, fast, with `Shift+S` on any row.

### I2. The key map collides with Linear muscle memory, twice on keys that launch agents

**Where:** 05 §4.3. Linear's bindings as recorded in 01 §15.1 (partly **UNVERIFIED**, from a
search summary).

| A Linear user presses | Expecting (01 §15.1) | 05 does |
|---|---|---|
| `Shift+D` | set the due date | **Delegate to my Orchestrator**: agents start and the bill starts |
| `M` then `B` | make a "blocks" relation | **ask my Orchestrator**, then `B` |
| `G` `M` | my issues | go to **my Orchestrator** |
| `G` `I` | Inbox | my issues (the Inbox is `G` `D`, the Desk) |
| `H` | snooze or remind | nothing (`4` in Triage, `D` on the Desk) |

Two more problems:

- **05's own rule is broken.** *"Lower case edits the record; `Shift` hands work to your agents.
  A reader can tell from the key alone whether an agent will run."* Yet `M` (lower case) talks to
  an agent, and `Shift+X` (range select) runs none.
- **The "not copied" claim is wrong in fact.** 05 says Linear's bindings *"are not copied"*.
  Yet `C`, `I`, `X`, `J`/`K`, `Space`, `/`, `?`, `G`-go, `O`-open, `O` `I` and number-key triage
  all match 01's record. Matching is fine: shortcuts are functional, and Gmail and GitHub share
  most of them. The claim is what is wrong.

**Fix.**
- **No agent-launching key on any key Linear uses for a property.** Move Delegate off `Shift+D`.
- Give relations `M` as Linear does; move *ask my Orchestrator* elsewhere; bind `H` to defer.
- State the policy plainly: "we match common list conventions on purpose, and depart only where
  an agent or a cost is involved".
- Slice the table: `M` and `Shift+D` do nothing in W1, which has no Orchestrator.

### I3. The thread has open ends that the design allows, and one path that cuts it

`CLAUDE.md`: *"Any feature that breaks that thread is wrong regardless of how good it is in
isolation."* 05 keeps the thread for work that starts from a brief. Other work arrives with none:

- **Issues with an empty `serves`.** They are allowed and "shown as such" (§2.3). They will be
  most issues: bugs, chores, and everything imported.
- **Requests** (`rq-`) join the thread only if triage picks a criterion.
- **Imports and one-way sync** (W5) arrive in `triage` with no `serves`, hundreds at a time.
- **Two-way sync** (Q9) lets the other tracker set state, so the plan of record splits between two
  tools. 05 §5.7 says of the sync-only option *"the thread stops at the mirror"*. Two-way sync
  gives the same result, only more quietly.

An open end is acceptable; a *silent* one is not. **Fix.**
- Give every issue a visible **thread state**: *on the thread* (serves a criterion), *project-level*
  (serves the project brief's standing criteria, such as "no regression"), or *not tied*, with a
  reason chosen at triage.
- Make "attach to a criterion" a one-key triage act, suggested as early as W1 (by text match
  first; the gateway can suggest in W4).
- Recommend **one-way sync in, two-way for comments only**. Sync never moves a fact state or
  `done` (C4).

### I4. Two enterprise basics are marked matched but are missing: custom states and cross-team planning

- **Custom workflow states (L-8).** Linear has "six fixed status categories with custom named
  statuses per team" (01 L-8): a team can add *In QA* or *Design review*. 05 marks it EXCEED with
  nine fixed, **renameable** states, several set only by facts. A team cannot add a state, so its
  real process has nowhere to live, and it goes back to labels or to Jira.
  **Fix:** allow custom states inside the human categories (`backlog`, `todo`, and after `landed`,
  for example *In QA*). Fact states stay fixed. Then mark L-8 PARTIAL until that is built.
- **Cross-team planning.** 05 skips sub-teams (L-4), sharing with a non-member (L-6) and
  sub-initiatives (L-86); defers multi-team features to "later" (L-67); and makes initiatives
  **per team** (L-83). An initiative that cannot span teams is a label. In a KPMG engagement, where
  several teams work for one client, this is the planning level people use most.
  **Fix:** make the cross-team grant a named W4 item with a size, or state as a product decision
  that Phase W is for one team, and put it to the owner. Do not leave it as three SKIPs and a
  "later".

### I5. The "more" adds nouns faster than it removes them

`CLAUDE.md` sets the wedge as *"usability and iteration on powerful tools … If a feature is
powerful but takes a wall of configuration to reach, it has failed our bar."*

- **Nouns.** A person meets project, feature, branch, brief, criterion, issue, sub-issue, task,
  lane, worktree, session, cycle, initiative, milestone, request, Orchestrator, worker and
  evidence. That is 18 before org and team. Linear's conceptual model has eight, with workspace and
  team included, and Linear sells that as its strength: *"a new user can hold the whole model in
  their head"* (01 §1).
- **Duplicate words.** *Lane* and *worktree* name the same local clone (05 §2.1, OD-2). A *feature*
  is a branch. *Work* names two different surfaces: the project surface and the seventh lens.
- **Two hierarchies of one project.** The rail runs project → branch → worktree → session. The
  Work surface runs project → feature → issue → task. Each is shown with its own nouns.

None of the "more" needs the user to learn all 18. **Fix.**
- Pick a person-facing vocabulary and hide the rest. *Task* and *worker* are the Orchestrator's
  words, not the user's. *Lane* or *worktree*, not both. Show a feature and its branch as one row.
- Rename one of the two Work surfaces, for example **Plan** (project) and **Work** (feature lens).
- Add to C13's not-a-clone review: "a new user can say what each visible noun means after five
  minutes".

### I6. "Prettier", the owner's first adjective, has no slice

**Where:** 05 §4.4, §5.3 to §5.4; 02 C6, C12, D1, D2.

- **W1's state glyphs are placeholders.** 05 §4.1 notes that `◇` and `▷` stand in for "our own
  drawn set". The drawn set, about 12 glyphs (02 C6), is **not sized in any slice**. Neither is the
  icon family's licence check (02:391, UNVERIFIED).
- **The visible uplift waits until W2.** `chrome.dim`, `ViewHeader`, `voice.split` and the
  generated themes are W2's 5 days, so the demo the owner sees first has the current look.
- **The owner's own review tools are unscheduled.** `stress.sets` and `flag.compare` (02 D2 step
  7, ~2 d) are what let the owner compare old and new in two seconds, and no slice includes them.
- **Q10 hides a cost.** It asks the owner to accept Berkeley Mono without restating 02's note that
  it is a paid licence (~$100, 02:222), against Commit Mono (OFL, free).

**Fix.** Move 02 D2 steps 2, 3 and 7, plus the drawn state glyphs (a first set of about 9), into
W1 **before** the prototype review (row 7). The owner's eyes are the acceptance test (`CLAUDE.md`),
so they should judge the new look, not the old. Restate the font licence in Q10.

### I7. The Work surface fails two AGENTIC-UX anti-patterns, and 05 amends only one

`AGENTIC-UX.md` §10: *"Any one of these fails a design review."*

- **Anti-pattern 1:** *"No UI element without a nameable transcript span. Zero exceptions."* An
  issue a person wrote, a cycle header, a label and a request have no transcript span. Read
  literally, every row of the Work surface fails review. 05 cites AX Principle 7 for insights but
  never reconciles anti-pattern 1 with the human-authored data it adds.
- **Anti-pattern 10:** *"No badge or count outside the queue."* Q1 amends AX:245 to allow plain
  group sizes, and 05 §4.4 rule 3 says *"No count outside a group header or the Desk."* The Work
  sketch in 05 §4.1 then puts counts inside rows: `● 2 agents`, `files … +2`, `queued 2nd`, and
  `2 new · N to show`.

**Fix.** Propose a second one-line amendment for anti-pattern 1: *"… a nameable transcript span,
or the Hub record and revision it shows."* Then redraw the sketch to 05's own rule 3: name the
agents' owner rather than counting them, and list files rather than writing `+2`.

### I8. What happens when a person edits someone else's in-flight issue

§3.6 lets any team member write issues, and §2.2 point 4 turns a cross-owner `blocks` link into
*"a `sequence` proposal automatically"*. Take Alice and Dylan's EP-142, which Dylan's agents are
working on. 05 does not say what happens when Alice:

- **edits its description or `serves`.** The agent keeps the revision it was given (good, §3.4).
  But nothing tells Dylan *"EP-142 changed since your agent read @3"*, so nothing asks him whether
  to re-brief the agent;
- **reassigns it to herself.** Dylan's agents are still bound to it. Is Alice now accountable for
  Dylan's agents' work? Are they stopped, and by whom? OD-16 says nobody steers another user's
  agent;
- **adds `EP-150 blocks EP-142`.** That automatic proposal pauses or sequences Dylan's agents. It
  was started by Alice's edit, not by Alice's Orchestrator. OD-16 makes Orchestrators *"the ONLY
  cross-user channel"*.

**Fix.** Add a short §3.7 with three rules:
1. A human edit to another owner's in-flight issue reaches that owner as a Desk *inform*, never
   the agents.
2. Reassigning an issue with agents on it asks the current owner's Orchestrator to wind them down
   or hand the work over.
3. A cross-owner relation becomes a proposal **from the editor's own Orchestrator**, which can then
   negotiate under OD-16.

### I9. Two sizes drop without explanation; W1's Work surface looks thin

- **The Hub replica.** W1 row 11 sizes the in-memory Hub replica, with undo and in-row confirm, at
  **2.5 d**. 02 D2 step 6 sizes `hub.replica` + `sync.checkpoint` at **~2 weeks**. Moving from disk
  to memory removes the storage work, but not the checkpoint, the ordering, conflict handling or
  permission filtering. That is a factor of four with no reason given.
- **The Work surface.** W1 row 8 builds triage, list, board, grouping, display options and the bulk
  bar at **3 d**. That is the surface a Linear user judges the whole product by (01 §22 items 1-2).

05 already marks every size unmeasured; the problem is the direction of the error. **Fix.** Either
reconcile the replica with 02 or explain the cut, and resize row 8 once the prototype (row 7) has
been reviewed.

### I10. `panacea://` links do not open for anyone without the app

L-98 is marked EXCEED because *"every screen is a `panacea://` link"*. Linear's links are https:
pasted in chat, they open for anyone with access. With the web app skipped (L-233), a link pasted
in Teams opens nothing for a business owner, a reviewer or a colleague without the desktop app.
05 §1 L-233 says the Hub serves "read-only issue pages for people without the app", but never
joins the two up. **Fix.** Make every shared link an **https Hub URL** that opens the app when it
is installed and the read-only page when it is not. Keep `panacea://` as the internal form. Mark
L-98 MATCH until that is built.

---

## Minor

1. **Q1 bans more than AGENTIC-UX does.** Q1 says *"anything that predicts stays banned"*, but 05
   itself uses *"Next decision expected ~14:22"* (§4.2) and a horizon forecast, *"will meet in
   `upload.go` ~14:20"* (§2.5). Both come from AX itself (AX:145, :149). What AX:237 bans is a
   completion prediction: percent, progress bar, ETA, "N of M". Word Q1 the same way.
2. **00 does not match 05 on two points.**
   - 00's "more" table has four rows but says *"two of the five parts"*; name the five.
   - 00's list of skip reasons leaves out L-195 (partner agents) and L-205 (GitLab).
3. **`dc-` is used but not defined.** Free documents `dc-` appear in L-120 but not in §3.2's
   records or §3.6's prefix collision check.
4. **W0 edits a contract that is being edited.** W0 changes `hub-contract` "now". That contract is
   under live edit in the `feat/hub` worktree, and the change comes before the owner has said yes to
   Phase W. §5.5 says the work arrives as *"new Hub tasks after task 43, not edits to tasks in
   flight"*. The change is harmless, but either say it is the one exception or hold it.
5. **Undo on `Shift+S` has a race.** Start is *"reversible until the session's first write"*, and
   `budget.ack` wants a first sign of work within 10 s. The undo window is 6 s. Say whether undo
   cancels the session and bills nothing, and what happens when the first write falls inside the
   window.
6. **L-235 is marked MATCH but is a deliberate difference.** Linear lets a person choose a home view;
   05 fixes it to the Desk. Call it a difference, with AX §5 as the reason.
7. **L-8 wording.** *"Fixed categories, renameable"* reads as parity. Nine fixed states against
   Linear's six categories with added states is not parity (I4).
8. **The Desk's zero state promises something W1 cannot show.** It includes *"Next decision
   expected ~14:22"*, which needs the horizon bar, and 05 §5.6 puts that outside W1's minimal Desk.

---

## The six questions asked of this review, answered

| Question | Answer |
|---|---|
| Is parity real? | **The count is real; the parity is not, yet.** 257 rows, no gap, and the tally matches. But 135 are reached at the recommended stop, about 28 are partial, and nothing is built (C3, I4) |
| Is the "more" better, or complexity? | **Better in kind, over-built in words.** Start and Delegate, fact-driven states and the Desk are real gains. "Done means proven" is the right idea with the wrong rule (C4). The nouns are too many (I5) |
| One product, with the thread intact? | **One product: yes.** The seventh lens, chips everywhere, one issue view and no second tree in the rail are right. **The thread:** intact for brief-led work, open for everything else, and cut by two-way sync (I3) |
| Is the first slice lovable and small? | **No on both counts** (C1, I1). A reshaped W1 would be lovable: comments, the editor and import (+5.5 d, I1), the new look (+~4 d plus the drawn glyphs, I6), and *shipped* moved to W1b (−1 d). It can only be small if it runs in local mode or inside TM1's streams |
| What would a Linear power user miss on day one? | Their imported backlog, comments, Markdown, cycles, links that open in a browser, outside-PR linking, and their keys: `Shift+D` and `M` do something else, and both involve agents (I1, I2, I10) |
| Does it contradict an owner decision or in-flight plan? | **OD-17**: the order against team mode is not stated (C2). **OD-19 and OD-20**: W1 needs the gateway and the cloud environment it does not list (C1). **OD-16**: cross-owner issue edits (I8). **TM-05 revision 3**: TM1 is ≈95.5 d, not the figure 05 plans around (C2). **AGENTIC-UX** anti-patterns 1 and 10 (I7) |

## What holds up, and should not be reworked

- The object placement (Hub records against Fleet state) and the immutable, digested issue
  revision that the agent's context names.
- Fact-driven `started`, `in_review` and `landed`, each written as the person whose act caused it.
- *Shipped* derived from the manifest, never set as a state.
- The Desk absorbing Inbox and My Issues. One place to empty beats three.
- The Linear project called a *feature* (Q7), and the SKIPs for CRM, SLAs and support-desk
  connectors (Q6). Both are well argued.
- The honesty of 05 §2.6 and §7. Most of this critique's evidence came from 05's own caveats.

## Unverified, and sources

**Unverified.**
- Linear's key bindings in I2 come from 01 §15.1, which marks the `G` and `O` lists as UNVERIFIED
  (from a search summary).
- "Most issues will have an empty `serves`" (C4, I3) is a judgement. It is not measured.
- Whether the Hub's read-only pages could serve https links (I10) was not checked in
  `hub-contract`.
- TM-05 line and section numbers are as read on 2026-09-25, and TM-05 is under revision.

**No web page was read for this critique.** Linear facts come from `01-linear-feature-inventory.md`
(§1, §15.1, §21 to §23), whose URLs and read dates (2026-09-25) are its own. Plane's source was
not opened.

**Read in this repository, 2026-09-25:**
- This folder: `00-summary.md` and `05-design.md` in full; `01` §1, §15.1, §21 to §23; `02` lines
  150 to 156, 222, 254, 356 to 391; `04` lines 255 to 290.
- Planning: `../2026-09-23-owner-decisions.md` (OD-1 to OD-20); `../2026-09-24-team-collaboration/05-design.md`
  lines 12 to 31, 995 and 1238 to 1292.
- Product docs: `docs/AGENTIC-UX.md` lines 145, 149 and 230 to 251; `docs/PROGRAM.md` lines 50 to 94.
- The branch and worktree list (`git worktree list`, `git branch -a`).
- The matrix tally was recounted by a script that parsed 05 §1's rows, run through `claudezsh`.
