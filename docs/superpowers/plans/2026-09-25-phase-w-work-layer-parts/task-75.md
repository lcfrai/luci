### Task 75: custom states inside the human bands

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `custom_states` table, `issue_heads.sub_state` and its band trigger), after a
sync-merge of `feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `CustomState`,
`CustomStateSpec`, `StateBand`, `BandHolds`, `ValidCustomStateKey`, `CodeHeldBySubState`,
`CodeStateInUse`, `CodeSubStateOutOfBand`) and **SW.4** (task 10's state move and done rule);
adds two event kinds to **SW.5** (task 13's registry; additive, so SW.5's freeze allows it).

A team wants *Ready for design* before work starts and *In QA* after it lands. Linear lets a team
rename and reorder every state. We cannot: `started`, `in_review`, `landed` and `done` are facts
(skeleton rule 2), and a renamed fact would let a person type a fact into existence. So a team adds
**sub-states** that sit inside the two human bands only, and the one that matters most, an
`after_landed` sub-state, **holds `done`** until a person clears it (05 §3.3, `05-design.md:732-734`).
This task is the Hub half: the team's list, the per-issue sub-state, the band check, the hold on
task 10's done rule, and the release of that hold when a person clears it. Task 84 is the desktop.

**Anchors, re-read on 2026-09-26.** In this branch's parts (not yet code, so **UNVERIFIED until
merge**): task 72's `StateBand`, `BandHolds`, `ValidCustomStateKey` (`task-72.md:99-108`,
`:268-283`), `CustomState` and `CustomStateSpec` (`:488-503`), the three codes (`:358-361`);
task 10's route and rule (`task-10.md:402-435`) and store contract (`:353-400`); task 13's
registry (`task-13.md:250-270`) and `EventData` (`:297`); task 07's matrix helpers `wread`,
`wwrite`, `wadmin` and `workMatrixLive` (`task-07.md:362-391`). In the Hub worktree at `6463e568`
(read, not merged here): contract §5.2 roles (`2026-09-23-hub-contract.md:324-333`), §8's rule that
event `data` is metadata (`:1131` onward). **Task 74's part is not written at this writing**, so
its column names (`custom_states.key`, `.name`, `.band`, `.ord`; `issue_heads.sub_state`) are the
scope's (`tasks-61-80.md:474-483`), **UNVERIFIED**; a different spelling in task 74 is a
find-and-replace in this task's store file only.

#### Decisions the scope left open, made here

**(a) The list is replaced whole, and matched by key.** `PUT /v1/teams/{teamId}/states` sends every
custom state the team keeps. The server diffs by `key`: a key in both is an edit (name, band,
order); a key only in the stored list is a removal; a key only in the body is an addition. A key
never changes: renaming is a `name` change, so issues holding the key keep it.

**(b) Changing a held state's band is a removal plus an addition.** An issue in `todo` holding
`ready_for_design` would sit out of band if the state moved to `after_landed`. So a band change on
a key any head holds is `409 state_in_use`, the same answer as removing it. The admin clears the
holders first or makes a new key.

**(c) Order is re-derived, not trusted.** The body's `order` per band must be `0..n-1` without gaps
or repeats (400 naming `states[i].order`); the server stores it as sent. Gapless is checkable, so
a client bug cannot store two states at order 1.

**(d) `SetSubStateRequest.Key` is a `*string`; absent and `null` both clear.** The route has one
meaning, "the sub-state is now this", so an absent key clearing is not a surprise, and task 72's
`Change[T]` is not needed here. A key the team does not hold is `422 substate_out_of_band` with
`details.band: ""` (there is no band for an unknown key), not a 404: the issue exists.

**(e) Clearing an `after_landed` sub-state re-runs task 10's evidence decision, not the move.**
The clear calls task 10's `DecideProof` over the issue's live citations at its land commit (task
10 decision 5). `proven` or `proven_self_attested` moves the issue to `done` with cause
`evidence_recorded` if any cited record is recorded, else `evidence_observed`, written as the
**clearer** (skeleton rule 2: a fact is written as the person whose act caused it). Two events
follow, in one transaction: `issue.sub_state_changed`, then task 10's `issue.state`. The clearer
is a member; a viewer cannot release a hold.

**(f) A hand-set `done` is not held.** Task 10's `hand` cause into `done` needs a reason and reads
`not_proven`; an admin who hand-closes an issue sitting *In QA* has decided in words, so the hold
applies to the **evidence** causes only, and the hand move clears the sub-state (it leaves the
band). This keeps one rule: a sub-state never blocks a person, only the automatic close.

**(g) Removal counts are plain numbers.** `409 state_in_use` carries `details.key` and
`details.count` (task 72's code comment), and the message reads *"3 issues are In QA. Move them
first."* The reply does not list the issues: the count is enough to act on, and the desktop's
filter `subState is in_qa` (task 72's `FilterFields`) finds them in one step.
