### Task 11: relations, team labels, `triage` and team policies

W1, stream WH; no seam of its own. Depends on task 08 (issue heads, the create path, `PATCH`) and,
in task order, touches task 10's move path in this plan's own files (the triage exit). Hub anchors
were read in the `feat/hub` worktree at `9f2fa31`: the contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md` (cited `hub-contract:N`) and the Hub plan's
task 14 part `docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-14.md` (cited `H14:N`), which
fixes the write helper every handler here uses. `server/` and `store/` are **planned, not on
`feat/hub`** at `9f2fa31` (only `hubapi/` and `hubclient/` exist there), so every path under them
is **UNVERIFIED until Hub tasks 08-14 land** (rule H1).

This task gives a team its working vocabulary. Three small aggregates, each one pair of files:
**relations** between two issues (`blocks`, `duplicates`, `relates`; L-24), **labels** that a team
owns (`lb-`; L-17), and **the team's work policy**, the three switches 05 §3.3 and L-9/L-48 name.
It also puts the server half of **triage** in place: where an issue lands when nobody names a
state, the one policy that can hold an issue in `triage`, and *snooze*, which defers without hiding.

Four decisions the scope left open, made here so no reader re-derives them.

1. **A relation is stored once, in a normal form, and read from both ends.** `relates` is
   symmetric, so its row always has the smaller id first. `blocks` and `duplicates` are directed:
   the row is `(issue_id = the writer's subject, other_issue_id = the target)`. On read, the other
   end sees the inverse word: `blocked_by` for `blocks`, `duplicated_by` for `duplicates`. The two
   inverse words are **read-only** members of `RelationKind`; a `RelationRequest` naming one is 400.
   Task 03 declares the three write kinds only, so this task adds the two read kinds additively
   (listed as a skeleton gap in the return).
2. **The cycle rule is the two-cycle, and only that.** `B blocks A` after `A blocks B` is 422
   `relation_cycle`, and so is `B duplicates A` after `A duplicates B`. A longer cycle (A→B→C→A) is
   **allowed** in W1: finding it needs a graph walk under the org lock, and 05 §3.7 asks for none.
   The part says so rather than implying a full check.
3. **Relations are same-team only.** A project's access is exactly its team's membership
   (hub-contract:333), so two issues of one team are both readable by anyone who can read either.
   A cross-team relation would show a reader an issue id of a project they cannot read. The other
   issue is looked up **inside the caller's readable set**, so another team's issue answers 404
   `issue_not_found`, never 403 (hub-contract:273-276's reason, verified at `9f2fa31`).
4. **A duplicate mark is a relation, not a state move.** `duplicates` writes the row and one event
   and leaves both states alone. Triage's `2` key (task 42) does the relation **and** a `hand` move
   to `duplicate`, as two writes, each with its own event and its own Undo.

**The one thing this task must not get wrong** is the triage exit under `priorityToLeaveTriage`.
The policy exists so an issue does not leave triage unprioritised; it must never stop a person
**declining** one. So the refusal applies to a move out of `triage` to any state except
`cancelled` and `duplicate`, only when the issue's priority is `none`, and it is 422
`policy_refused` with `details.policy = "priorityToLeaveTriage"`, appending **no** event. Task 16's
relay passes that problem through untouched (`task-16.md:25-32`), which is how the desktop tells a
person *why* EP-142 stayed in triage.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, modified additively): the two read
  kinds `RelationBlockedBy`, `RelationDuplicatedBy`; `IssueHead.SnoozedUntil`,
  `PatchIssueRequest.SnoozedUntil`; `TriageExitRefused`, the pure policy check both the Hub and the
  fake call; the Work codes this task needs, if task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_social_test.go` (new): the pure rules, no database
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` ⚑ (task 06's file, a section appended
  before it is applied anywhere shared; the next free number instead if 06 has merged, rule H1)
- `control-plane/internal/hub/store/work_relations.go`, `server/work_relations.go` (new)
- `control-plane/internal/hub/store/work_labels.go`, `server/work_labels.go` (new)
- `control-plane/internal/hub/store/work_policies.go`, `server/work_policies.go` (new)
- `control-plane/internal/hub/store/work_issues.go` ⚑ (task 08's file): the create reads the
  policy; the patch validates `labels` and `snoozedUntil`
- `control-plane/internal/hub/store/work_states.go` ⚑ (task 10's file): the triage exit check
- `control-plane/internal/hub/server/work_social_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ (task 05's file): only if a rule here
  differs from what 05 wrote; the conformance rows (task 05) are the arbiter

