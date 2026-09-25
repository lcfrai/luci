### Task 142: W4's proof and results

Stream **W4** (roadmap, release and reporting), about 2.5 hours of work beside the run itself (an
estimate, not a measurement). Depends on tasks **114-141**. Writes the W4 section of
`docs/verification/2026-09-25-phase-w-results.md` ⚑, owned jointly with tasks 70, 97, 113, 174
and 187 and appended in task order, after W3's (task 113). The section keeps task 113's shape
(`task-113.md:86-128`) so the six sections read alike; task 70 creates the file on the model of
`docs/verification/2026-09-17-phase-1-results.md` (built).

`OD-27` is the rule: **proof per criterion, no prediction.** A row this section marks closed
carries the command that was run, an excerpt of what it printed, and the commit it ran at. A row
with no output beside it is not closed, whatever a part or a person says.

**Decisions made here, so nobody re-derives them:**

1. **The environment is named, first line.** The run uses the local Hub stack
   (`docker compose -f control-plane/cmd/hub/compose.yaml up -d`, task 124's step 5; `compose.yaml`
   and `devissuer` are Phase H's, **planned**) with sign-in through `devissuer`, and the control
   plane in **local mode** (`OD-20`). Where a row needs the gateway, it uses phase 6's gateway
   **only if** phase 6 is on the sync base; the lead line says whether it was. A local run proves
   nothing about a cluster, and the section says so.
2. **The rows are re-derived, not copied.** `check-w4.sh` selects every row of 05 §1 whose slice
   column names W4. On 2026-09-26 that is **forty-one** rows (checked while writing this part by
   running the selection over `05-design.md`; the list matches the scope's, `tasks-141-160.md:106-109`).
   Four of them are **halves** whose other half is an earlier slice or an existing phase (L-67 with
   W2, L-101 with W1, L-82 and L-253 *exists*); the row says which half.
3. **One row that is not W4's is carried**: L-255's per-issue and per-loop half (task 135). 05 §1
   places L-255 in phase 6 (`05-design.md:447`), so the selection does not find it; the check reads
   it from an explicit carried-rows list, and the row says *W4 half; phase 6 closes*.
4. **PARTIAL stays PARTIAL.** L-50, L-72, L-74, L-118 and L-119 are recorded with the word
   *PARTIAL* in their row and the open part named (the table below). A PARTIAL row with no open
   part named fails the check.
5. **Conditional rows are recorded as found.** L-53 closes only once the calibration record's home
   exists (task 114's noted gap); L-144 only once P7 task 19's freeze record has merged; L-147 only
   in the half task 131 builds (notes by criterion) unless the drafted prose has a task (decision 9).
   On the day, each is *closed*, *PARTIAL (the open part)* or *not run (why)*, never assumed.
6. **One scripted run, fifteen frames** (`docs/verification/phase-w/w4-run.md`), following the
   scope (`tasks-141-160.md:90-103`). Frames 10-15 need phase 6 (the spend rollup or the gateway);
   without it they are **not run**, and their rows are *not run*, never *passed*.
7. **Observed, never inferred.** A frame's observation is what a person saw or a command printed,
   quoted. *"The draft looked right"* is not an observation; *"the update's body read `text·A` in
   `GET /v1/features/ft-…/updates` until Post"* is. Nothing is read from a harness's terminal (`OD-8`).
8. **The owner's look is its own row.** The W4 surfaces (sharing, initiatives, the composer, the
   timeline, milestones, release notes and freeze, insights, *since you last looked*, recurring
   issues, spend, the assists, drafts, proposals and the diff) are shown to the owner and the
   verdict is recorded in the owner's words (`OD-1`; `CLAUDE.md`: *"Visual acceptance is the
   owner's eyes"*).
9. **Gaps are carried, not fixed here.** Task 139 found that L-147's drafted release-note prose has
   no task (`task-139.md`, gap 1); task 134's part is unfinished on this tree (73 lines, no tests
   yet, uncommitted on 2026-09-26), so whether its three surfaces call `expectNoPrediction` is
   **UNVERIFIED**. The section lists the first under *What is still unproven*, and the sweep below
   fails until every listed surface's test calls the helper.
10. **What stays unverified is listed by name**: every assist row if phase 6 is not on the sync
    base (136-140); phase 7's manifest and freeze if P7 task 19 has not merged (131); the
    calibration dial's accept counts' home (task 114's gap); the Hub's `edit_proposal` kind (task
    141's gap 1); the owner's visual acceptance of the timeline, insights and diff surfaces.

**Files:**
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the W4 section (modified; task 70's file)
- `docs/verification/phase-w/w4-run.md` — the fifteen frames a person runs (new)
- `docs/verification/phase-w/check-w4.sh` — the coverage check (new)
- `docs/verification/phase-w/testdata/{w4-complete,w4-missing-output,w4-partial-unnamed}.md` —
  the check's fixtures (new)
- `desktop/src/test/w4Sweep.test.ts` — every W4 surface's test calls `expectNoPrediction` (new)

#### The rows W4 closes, and what closes each

Test names come from each part's mutant table where the part exists on this tree (114-127,
129-133, 139-141). Tasks **128 and 135-138 have no part on this tree** on 2026-09-26, and 134's
is unfinished: for them the row names the behaviour their scope's test holds (`tasks-121-140.md`),
and step 1 replaces it with the real test name. The run pastes each test's `PASS` line.

| Row | 05 §1 (`05-design.md`) | Closed by | The test that must pass |
|---|---|---|---|
| L-32 | `:141` MATCH | 121, 134 | `TestAJanitorActHasANullActor`; `TestAWeeklyScheduleRunsOncePerOccurrence`; 134's *Repeat* field (scope) |
| L-43 | `:152` EXCEED | 139 | `TestSummaryLinksOnlyItsThread`; `ThreadSummary` *is absent at 12, offered at 13, and writes nothing* |
| L-50 (PARTIAL) | `:164` | 121, 134 | `TestTheRotationHandsOverAtTheBoundary`. Open: rotation synced from PagerDuty or Opsgenie (L-218, task 184) |
| L-51 | `:165` EXCEED | 136 | (scope) a criterion on no brief is dropped; accept writes one patch whose actor is the person |
| L-52 | `:166` EXCEED | 136 | (scope) an item with no why is not rendered |
| L-53 | `:167` EXCEED | 136 | (scope) the dial shows no number while the calibration route is absent; closed only once the record lands (decision 5) |
| L-54 | `:168` MATCH | 137 | (scope) candidates never include an unreadable issue; accepting a duplicate leaves both states unchanged |
| L-55 | `:169` MATCH | 137 | (scope) project guidance follows team and org and is marked winning |
| L-56 | `:170` MATCH | 137 | (scope) typing 30 characters makes at most two calls; a hint for superseded text is discarded |
| L-66 | `:185` EXCEED | 140 | `TestCycleProposalHasNoForecastField`; `CycleProposal` *groups by criterion, speaks no forecast, and Undo reverts the set* |
| L-67, W4 half | `:191` MATCH | 117, 126 | `TestTheGrantedTeamReadsAndComments`; `TestAGrantedWriteIs403Not404`; `FeatureSharing` *a granted reader has no write control* |
| L-72 (PARTIAL) | `:196` | 120, 130 | `TestReachedFlipsOnlyOnTheLast`; `MilestoneList` *the list shows words, never a count*. Open: Linear's progress percentage, not built by design (`OD-27`) |
| L-74 (PARTIAL) | `:198` | 120, 129 | `TestOrderViolatedIsDerived`; `DependencyLines` *blocker %s, blocked %s → %s*. Open: dependencies across teams (`TestDependenciesStayInOneTeam`) |
| L-75 | `:199` MATCH | 120 | `TestATemplateMakesAThreadedFeature` |
| L-76 | `:200` EXCEED | 119, 127 | `TestAnUpdateIsPinnedByDefault`; `TestCitesResolveOrTheUpdateIsRefused`; composer *Post sends the pin taken at open* |
| L-77 | `:201` MATCH | 121, 127 | `TestAStaleFeatureGetsOneReminder`; `TestAnUpdateResetsTheReminder`; *the reminder row opens the composer in place* |
| L-78 | `:202` EXCEED | 139 | `TestUncitedAndForeignCitesAreDropped`; `TestDraftNeverCarriesHealth`; *a draft enters as text·A, leaves health empty* |
| L-79 | `:203` MATCH | 139 | `TestFactsNeedNoGateway`; *the changes block renders with the gateway down* |
| L-82, W4 half | `:206` EXCEED | 140 | `TestOneDraftPerUncoveredCriterion`; `TestServesIsSetByCode`; *accept creates as the person with text·A and its own criterion* |
| L-83 | `:212` MATCH | 118, 117, 127 | `TestAttachNeedsTheSpan`; `TestASpanNeedsALink`; `TestAWiderSpanReadsButDoesNotWrite` |
| L-84 | `:213` MATCH | 118, 127 | `TestTheViewHasNoAggregate`; `TestNoUpdateReadsNull`; *latestUpdate null reads 'No update yet'* |
| L-85 | `:214` EXCEED | 118, 127 | `TestCriteriaWordsAreRecorded` |
| L-87 | `:216` MATCH | 119, 127 | `TestAnInitiativeUpdatePinsEachFeature` |
| L-88 | `:217` MATCH | 118, 127 | `TestAnotherTeamsInitiativeIs404`; `TestAnUnreadableFeatureNeverAppears` |
| L-89 | `:218` MATCH | 128 | (scope) quarter ticks fall on quarter starts in `Australia/Sydney`; 2026-10-04 has the width of 3 October |
| L-90 | `:219` MATCH | 129 | `TimelineDrag` *a drop sends exactly one patch, with the new date*; `useTimelineDrag` *a drop 2.4 days in lands on the nearest whole day* |
| L-91 | `:220` MATCH | 128 | (scope) a cooldown is a gap in the band, not a cycle |
| L-92 | `:221` MATCH | 128 | (scope) grouping by lead puts a lead-less feature in *No lead*, last |
| L-100 | `:234` MATCH | 138 | (scope) the prompt fixture holds no issue title; a gateway failure keeps the typed text |
| L-101, W4 half | `:235` MATCH | 128 | (scope) the display menu offers *timeline* beside list and board; a span holds one fill |
| L-118 (PARTIAL) | `:257` | 123, 133 | `TestACursorOnlyAdvances`; `TestTheScopeFilterNeverWidens`; `SinceIssues` *every sentence has a link*. Open: session and branch deltas |
| L-119 (PARTIAL) | `:258` | 133 (123) | *the handover module imports no gateway, inference or draft client*; the digest carries the same sentences. Open: audio playback (later) |
| L-122 | `:266` MATCH | 120 | `TestATemplateMakesAThreadedFeature` (brief templates per team) |
| L-130 | `:274` EXCEED | 141 | `proposal` *Accept writes one revision as the person*; *nothing is applied on arrival*; `TestEditProposalNeverReachesTheManagedBlock` |
| L-144 | `:301` MATCH | 131 | `TestShipToAFrozenEnvironmentCallsNothing`; `TestAnUnreadFreezeFailsClosed`; closed only once P7 task 19's record merges (decision 5) |
| L-145 | `:302` EXCEED | 131 | `TestFilterDropsCommitsOutsideItsPaths`; `TestNoFilterFileMeansNoFilter` |
| L-147 | `:304` EXCEED | 131 (139) | `TestOnlyCriteriaThisReleaseSettledAreListed`; `TestNotesCarryNoModelText`. The drafted prose has no task (decision 9): PARTIAL unless one lands |
| L-148 | `:305` EXCEED | 131 | `TestShippedNamesEachEnvironment`; `TestShippedAfterARollback` |
| L-150 | `:307` MATCH | 122, 132 | `TestCycleTimeRunsFromStartedToDone`; `TestNearestRankNotInterpolated`; *the p50 label is the Hub's value* |
| L-151 | `:308` MATCH | 122, 132 | `TestNoPointsMeansNoPercentiles`; `Insights` *clicking a mark opens that issue* |
| L-253, W4 half | `:445` EXCEED | 134 | (scope) `method.doc.test.ts`: every claim of `docs/work/method.md` cites a heading that exists |
| L-255, W4 half (carried) | `:447` phase 6 | 135 | (scope) the issue scope sums only keys carrying that issue; with the rollup unreachable the line is absent, not *$0.00* |

Rows W4 **carries but does not close**: task 124's `TestWorkRoadmapE2E` and
`TestWorkRoadmapConformance` hold the Hub side of every Hub-backed row above and are pasted once,
in *The suites*. Rows that close only **with phase 6** on the sync base: L-43, L-51 to L-54, L-56,
L-66, L-78, L-82, L-100, L-130 (its proposer is an assist) and L-255; L-55 too if task 137's
layering test runs through the assist client (**UNVERIFIED**: no part yet). L-79 does not (its
facts need no gateway, `TestFactsNeedNoGateway`).
