### Task 76: features `ft-`: lead, dates, status, branch reservation, *promote to feature*

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `features` table and `issue_heads.feature_id`), and P4-18 (phase 4's task
18, where a brief's text is first drafted), after a sync-merge of `feat/hub` (rule H1).
**Seams:** consumes **SW.18** (task 72's `Feature`, `FeatureState`, `FeatureBranchRef`,
`PromoteResult`, `CodeFeatureNotFound`, `CodeSlugInvalid`) and the Hub's briefs (contract §7,
`2026-09-23-hub-contract.md:755-761`); adds four event kinds to **SW.5**.

Linear calls a planned body of work a *project*. We already have projects (the repository), so
ours is a **feature**: a name, a lead, a target date, a status a person sets, and **its own
branch** (04 §2.5). The branch is what ties a feature to the provenance thread: a feature owns a
branch brief, the brief's criteria are what its issues serve, and landing on that branch is what
turns them `landed`. So creating a feature **reserves the branch** by creating its branch brief in
the same transaction, and a second feature or brief on that ref is refused. *Promote to feature*
turns a large issue into a feature with the issue as its first child, without copying anything.

**Anchors, re-read on 2026-09-26.** In the Hub worktree at `6463e568` (read, not merged here):
brief creation and its `409 brief_exists` (`hub-contract.md:755-761`, `:253`); the ref rules
(`:131-137`); `BriefHead` has no title and a new brief has `headRevision: 0` (`:641-644`); the
`brief.created` event (`:1148`); the rebind route `PATCH /v1/briefs/{briefId}` (Hub plan
`task-18.md:26`); roles (`:324-333`). In this branch's parts (not yet code, **UNVERIFIED until
merge**): task 72's `Feature` (`task-72.md:505-522`), `FeatureState` (`:110-122`),
`FeatureBranchRef` and its slug rule (`:250-251`, `:303-309`), `PromoteResult` (`:659-664`), codes
(`:352`, `:370`); task 24's Start order (`task-24.md:253-270`) and its AU fail-safe
(`:22-27`, `:396-402`); task 11's labels (`task-11.md:280-284`). **Task 74's part is not written
at this writing**; its `features` columns are the scope's (`tasks-61-80.md:475`), **UNVERIFIED**.

#### Decisions the scope left open, made here

**(a) The brief is created exactly as `POST /v1/projects/{projectId}/briefs` creates it.** The
Hub's brief record has no title (`hub-contract.md:641-644`) and a new one has no revision. So the
feature route calls the Hub's own brief insert (Hub task 18's store function, in the same
transaction), writes the same `brief.created` event, and writes **no brief content**. The scope's
"the brief record with the issue's title" cannot be stored on a `BriefHead`; the Hub never writes
brief text on a person's behalf, because every brief answer is authored (`text·H` or `text·A`).
The title reaches the brief when the desktop's promote flow (task 86) opens phase 4's Discover
lens on the new brief with the issue's title as its first question's context (P4-18's path).

**(b) The pointer to the issue is a key, in the event.** `issue.promoted` carries `issueKey`
(`issue:is-…@rev`, 05 §3.6), the revision the feature came from. With the issue's `featureId`
set, that is the whole link: nothing is copied (L-23). The issue's `serves` stays on its own
revision, unchanged.

**(c) A feature's `branchRef` is its brief's, read by a join.** A brief can be rebound (Hub
`task-18.md:26`). If the feature kept its own copy, a rebind would leave the two disagreeing. So
`Feature.branchRef` is answered from `briefs.branch_ref` through `features.brief_id`, and the
reservation's uniqueness is the brief's own index (one live branch brief per `(project,
branchRef)`, `hub-contract.md:760`). Task 74's `features` unique `(project_id, branch_ref)` is
then a second copy this task never reads; see *Corrections to the scope*.

**(d) The slug rule on an AU-only project.** 04 §5.2 keeps titles out of trailers and branch
names where the data must stay in Australia. A **word** is a run of 3 or more `[a-z0-9]` in the
lower-cased feature name (and, for *promote*, the issue's title). On an AU-only project a slug any
of whose `-`-separated parts equals a word is 422 `slug_invalid`, `details.slug`, message *"AU
projects keep names out of branches. Try feature-12."* The project's class is read through
`store.ProjectAUOnly`; **a class the Hub cannot read is AU** (task 24's fail-safe,
`task-24.md:22-27`), so until the class has a home (Phase T task 43, **UNVERIFIED**) every
project takes the AU rule. That is visible and safe, and ends when the class lands.

**(e) The lead is a member or admin of the project's team.** A viewer cannot lead work they cannot
write. Anyone else is 422 `bad_request`, `details.field: "leadId"`. The lead defaults to the
creator.

**(f) Status moves freely among the five, by a person only.** `planned`, `active`, `paused`,
`done`, `cancelled` are a person's words about intent; nothing is derived from them and nothing
derives them. What the lanes did (live, landed, released where) is **derived at read time** by the
desktop from the feature's child issues (task 85), and no such field exists on the wire (task 72's
`Feature` has none), so none can be stored.
