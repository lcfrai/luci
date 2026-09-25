### Task 120: milestones `ml-` as criterion subsets; feature dependencies; feature and brief templates

W4, stream WH; no seam. Depends on task 116 (the W4 tables and their guards). It also reads task
76's features and task 78's templates, both W2 and both before 116 in task order; neither part is
written at this writing, so every name taken from them is their **scope's** (`tasks-61-80.md:527`,
`:598`) and **UNVERIFIED** until those parts land. Hub anchors were read in the `feat/hub` worktree
at `1271ddb` (contract cited `hub-contract:N`; built `hubapi/envelope.go`). The wire is task 114's
part (`task-114.md`, in progress at this writing, cited `T114` by type name, since its line
numbers are still moving). Every `server/` and `store/`
path is **UNVERIFIED until merge** (rule H1).

Three planning records that each hang off a feature. A **milestone** is a named subset of the
feature's brief criteria with a date: *reached* when those criteria are settled, and never a bar
(L-72, PARTIAL on purpose, `OD-27`). A **feature dependency** says one feature blocks another, and
the timeline (task 129) colours a violated order. A **brief template** is a team's starting spec:
criteria plus the issue templates that serve them; *new feature from template* makes the feature,
its brief and its issues in one step, already on the thread (L-75, L-122).

Six decisions the scope left open, made here.

1. **The Hub refuses what it can know is wrong, and no more.** The Hub reads exactly one field of
   brief content, `status` (`hub-contract:744-745`), and criterion ids are positional, made by
   phase 4's parser. Task 09 decided that `serves` is therefore checked at the key level by the
   Hub, with membership checked by the writer (`task-09.md:28-39`). Task 114 comments that
   membership is "the Hub's check" (its `validCriterionList` comment); the two disagree. This task follows task 09:
   `criterion_not_on_brief` is answered when the Hub **holds** the brief's criterion list, which is
   true for a brief made from a template (this task records the list, decision 5), and a
   hand-written brief's criteria are checked by the writer (task 130's picker offers only parsed
   criteria). A Hub-wide check needs phase 4's writer to publish each revision's criterion ids, a
   Hub-contract edit that task 09's gap already names; it is not applied here (rule H1).
2. **The word per criterion is the Hub's own record, and says less than the design.** 04 §2.5 says
   *reached when those criteria are settled at the branch head*. The Hub has no git, so it cannot
   tell which evidence is at the head. `MilestoneView` takes each criterion's **latest recorded
   verdict** (`evidence.recorded_at`, the Hub's clock) under any revision of the feature's branch
   brief: `settled` or `contested`; with none, `open` when a live issue's head revision serves it
   and `uncovered` when none does. `drifting` (05 §3.3) needs criterion text, which only the
   desktop has; the Hub never answers it, and task 130 draws it from the replica. These four words
   are the Hub's subset of task 45's five (`task-45.md:81`); 114's `CriterionWord.Word` is a
   string (T114, `CriterionWord`), and this task fixes its values.
3. ***Reached* is every listed criterion `settled` at read**, never stored (T114 decision 6). The
   view prints the words and the flag, never "2 of 3" (`OD-27`).
4. **Dependencies stay inside one team.** Both features must be readable by the caller as a
   member of their team, and in the **same** team; a task 117 grant is read-and-comment, so a
   granted member cannot add one (403). A feature cannot block itself (422). Adding an existing
   pair answers 200 with it and appends no event (no record changed). A cycle is 422
   `dependency_cycle` from task 116's trigger, mapped by the handler, so the route and the database
   agree. `orderViolated` is `blocks.targetDate < from.targetDate` with both set, derived at read.
5. **A brief template carries the content the Hub cannot write.** The Hub never authors phase 4
   content, so *new feature from template* writes revision 1 from **content the template already
   holds**: when a person saves a brief template, the control plane (which has phase 4's schema)
   builds the content object with the template's criteria and the positional id of each, and sends
   both. The Hub validates the content as it validates any revision (`hubapi.ValidateContent`,
   `envelope.go:71`, status `open`) and stores it opaque. **Gap G1, fixed in 114:**
   `BriefTemplate` and its create and patch requests gain `content json.RawMessage`,
   `criterionIds []string` (parallel to `criteria`) and `issueCriteria []string` (parallel to
   `issueTemplateIds`: the criterion id each issue serves, or `""`); and
   `CreateBriefTemplateRequest` gains `kind` (always `"brief"`), because task 78's route decides
   the template kind from the body (T114's route table: *"`kind: "brief"`; task 78's route"*).
6. **The create is one transaction, in task 76's order.** Task 76's feature create (which reserves
   `branchRef` by creating the branch brief at head revision 0, `tasks-61-80.md:533-537`); the
   brief's revision 1 from the template's content, through the Hub's own revision write (Hub task
   19's store path for route 26, `hub-contract:588`; name **UNVERIFIED**); the criterion list
   (decision 1); then one issue per issue template through task 08's create, each `serves
   [{briefKey: brief:<bf>@1, criterionId}]` from `issueCriteria`, an empty entry leaving `serves`
   empty. Each write appends its own event (`hub-contract:1133`). Any failure rolls all of it back,
   the branch reservation included.

**The one thing this task must not get wrong** is a milestone that looks reached when it is not. A
default word (`settled` for a criterion nobody has proven), a count that stands in for the words,
or a criterion from another brief silently accepted would each tell a lead a checkpoint is met.
Decision 2's words start at `uncovered` or `open`, and only a recorded `settled` verdict moves one.
