### Task 118: initiatives `in-`: owner, target, status, the health roll-up

**Slice:** W4. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 116 (`work_initiatives`, `work_initiative_teams`, `features.initiative_id`, the
guards). It reads task 119's latest update per feature and task 120's criterion words when they
land; until then those two fields read `null` and `[]` and the tests that need them wait (Step 4).
**Seams:** consumes SW.20 (task 114's `Initiative`, `InitiativeView`, `InitiativeFeature`,
`UpdateStamp`, `CriterionWord`, the requests and `work.initiative_*` kinds); no new seam.

An initiative groups features toward one outcome, per team (05 §3.2, 04 §2.5). Linear's version
rolls the features up into a health and a progress graph; we do neither, because both would be a
number nobody measured (Q1, OD-27). The roll-up here is **each feature's own latest health word
and its date**, set by a person in a status update, and **each feature's criteria with their
words**, recorded facts. A lead reads convergence across features; the Hub computes no score.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`:** `hub-contract.md:341-359` (§5.3's
rows: a team's reads are `V M A` ✓ and `O N X` 404), `:333` (a project's access is its team's).
**Read in this branch's parts, UNVERIFIED until merge:** task 08's server and store idiom
(`task-08.md:253-400`); task 114's types (`task-114.md`) and `WorkRoadmapCodeStatus`; task 116's
tables (`task-116.md`, the tables block); task 117's `workAccess` (`task-117.md`, `work_access.go`)
and its span route, which calls this task's `store.InitiativeByID`; task 119's store read of the
latest update per subject (`task-119.md:212-226`, `UpdateMetaRow`); task 120's
`store.CriterionWords` and its four words `settled`, `contested`, `open`, `uncovered`
(`task-120.md:31-39`, `:97-98`, `:185`).

#### Decisions the scope left open, made here

1. **No aggregate field, by type.** `InitiativeView` (task 114) has `Initiative`, `Teams` and
   `Features`; nothing else. A test decodes the reply into `map[string]any` and fails on any key
   outside those three, so a later "helpful" `health` or `progress` breaks the build's tests, not
   just a review.
2. **A feature with no update reads `latestUpdate: null`.** Never a default word, never the
   initiative's own state.
3. **The criterion words are task 120's.** `settled`, `contested`, `open` (a live issue serves it,
   no verdict yet) and `uncovered` (nothing serves it). The scope's test *"a criterion with no
   evidence reads `open`"* holds for a served criterion; an unserved one reads `uncovered`, the
   stricter word. This part says which vocabulary the Hub sends (task 125's gap 3): these four,
   never `drifting`, which only the desktop can compute.
4. **Who reads.** A member (viewer or better) of the owning team, or of a team in the initiative's
   span (task 117); anyone else is 404 `initiative_not_found`, as if it did not exist. Each
   feature row is included only if `workAccess` on the feature's project passes for the caller, so
   a spanned reader sees the features their own team or a grant lets them read, and no other name.
5. **Who writes.** Create and patch: a member of the owning team (`RoleMember`); the owner must be
   a member of the team, checked at create and at every change of owner. Attaching a feature: a
   member of the **feature's** team, and the feature's team must be in the initiative's span, else
   422 `bad_request`, `field: "initiativeId"`.
6. **State words are the feature's five** (task 114, decision 1). A person sets them; nothing moves
   an initiative's state by itself, and `done` needs no condition (a person's word, like a
   feature's).
7. **List order** is newest first by `created_at`, through H's `Page` (`types.go:196-205`).
