### Task 83: desktop W2 wire types and replica kinds

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WD2.
**Depends on** task 55 (the replica and its no-persistence grep) and task 73 (the fake that fixes
SW.18's shapes). **Seams:** **SW.15** and **SW.16**, extended by **added types and kinds only** —
no existing type gains, loses or renames a field, and no existing function changes signature.
Contract dependencies it reads but does not wait for: task 72 (`hubapi/work_plan.go`, the JSON
tags this mirrors field for field), task 82 (the relay paths), task 56 (`ops.ts`) and task 57
(`visibility.ts`). Every test here runs on a fetch stub and the fake's fixtures, so none of the
four is a build dependency — the same shape task 37 had in W1, and the same gap.

The desktop learns the W2 wire here, once. After this task a W2 surface (84-96) imports its types
from `api/work` and calls `api.work.*`; nothing else in `src/` names a W2 path or a W2 field.

#### Anchors, re-read on 2026-09-27

`task-72.md:80-372` (the W2 records and enums, block by block), `:373-385`
(`WorkPlanCodeStatus`), `:1069-1128` (the golden field-name and never-omit tests whose fixtures
this task copies **by path**); `task-73.md:106-124` (the fake's files, the source of the
fixtures), `:126-140` (`conformance/work_plan.go`, rows `W-26` to `W-43`); `task-82.md` (this
branch) for `WorkPlanRoutes` and the relay prefix `/v1/hub/work`; `task-37.md:78-137` (the record
idiom and the header comment this file copies), `:257-312` (the route tables), `:313-367`
(`WorkClient`), `:368-458` (`makeWorkClient`), `:459-500` (`workProblem` and the wire key lists),
`:501-529` (the `client.ts` delta); `task-55.md:75-147` (the replica's types), `:148-210` (what an
event asks for), `:211-244` (the maps and `transact`), `:656-699` (`noPersistence.test.ts`);
`task-56.md:143-216` (`WorkOp`, `FieldPath`); `task-57.md:88-130` (`classify`, `FeedEventLike`,
`VisibilityAction`).

**UNVERIFIED until merge:** the fixture paths under `control-plane/internal/hub/hubapi/testdata/`
are task 72's plan, not files on the tree; if task 72 lands them elsewhere, decision (c)'s copy
step follows them and the test's failure message says so.

#### Decisions the scope left open, made here

**(a) W2's types live in `desktop/src/api/workPlan.ts`, which `work.ts` re-exports.** The scope
says `work.ts` ⚑ gains the W2 types. It gains them, but through one line: `export * from
"./workPlan"`. Three reasons, and the third is the deciding one. `work.ts` is already about 640
lines (task 37's eight parts); W2 adds roughly 300 more, and a 950-line wire file is read by
nobody. The Go side already split exactly here — task 72 (`task-72.md:13`): *"every W2 shape lives
in one file, `hubapi/work_plan.go`"* — so the two sides mirror at the file level, not only the
field level. And `work.ts` is ⚑: a shared, frozen-by-convention file that four merged W1 tasks
touch, so a 300-line append is a merge conflict waiting for tasks 84-96. Recorded under
*Corrections*: the scope's file list should read `workPlan.ts` (new) plus a one-line `work.ts`
delta.

**(b) The golden fixtures are copied by path, never retyped.** A `vitest` test reads the JSON
files task 72's Go golden tests read, from
`../../../control-plane/internal/hub/hubapi/testdata/work_plan/*.json`, parses each with the
matching TypeScript type, re-serialises with a key-sorted stable stringify, and compares to the
file. A renamed field — `targetDate` where the Go tag is `target_date` — fails, where `tsc` alone
would be perfectly happy with a type that describes a shape nothing sends. If the path does not
resolve the test fails loudly with the path it tried; it never skips.

**(c) Unknown feed kinds are ignored at the replica's door, and counted.** Hub contract §1 says a
consumer ignores kinds it does not know. The replica already routes by kind; the delta adds the
W2 kinds and an `else` that increments `replica.stats.unknownKinds` and returns, never throws.
The counter exists so a test can assert *ignored*, which is different from *handled*: without it,
"no throw" also passes when the kind silently mutated something.

**(d) A draft never reaches storage, and the grep is what proves it.** Task 55's
`noPersistence.test.ts` greps `src/work/` for `localStorage`, `sessionStorage` and `indexedDB`.
This task extends the grep's file set to `workPlan.ts` and the new replica branches, and adds one
positive test: a `CreateDraft` op held in the replica is gone after a remount with a fresh
`createReplica`. L-33's `404` hides a draft from a second **user**; this keeps it off a shared
**machine**.

**(e) Cycle ordering is asserted by `seq`, not by arrival.** `cycle.created` then `cycle.rolled`
applied out of order would leave a closed cycle open. The replica's feed apply already orders by
`seq` (task 55); this task's test drives the pair backwards through the same door and asserts the
end state, so the guarantee is tested at the W2 kinds rather than assumed from W1.

#### Files

| Path | Change |
|---|---|
| `desktop/src/api/workPlan.ts` | new: every W2 record, enum, request body, route row and client method |
| `desktop/src/api/workPlan.test.ts` | new: the golden round-trips, the route table tests, `workProblem`'s W2 codes |
| `desktop/src/api/work.ts` ⚑ (task 37) | one line: `export * from "./workPlan";` |
| `desktop/src/api/client.ts` ⚑ (task 37) | `api.work` gains the 46 W2 calls through `makeWorkPlanClient` |
| `desktop/src/work/replica/replica.ts` ⚑ (task 55) | the W2 feed kinds, the W2 maps, `stats.unknownKinds` |
| `desktop/src/work/replica/ops.ts` ⚑ (task 56) | the W2 op kinds and their `FieldPath`s |
| `desktop/src/work/replica/visibility.ts` ⚑ (task 57) | `draftsAreMine`: a draft not authored by the caller is dropped |
| `desktop/src/work/replica/replicaPlan.test.ts` | new: the kind tests, the order pair, the unknown kind |
| `desktop/src/work/replica/noPersistence.test.ts` ⚑ (task 55) | the file set gains `workPlan.ts` and the new branches; the draft remount test |
| `desktop/src/test/workPlanFixtures.ts` | new: the fake's W2 records as test data, in task 20's idiom |

#### Interfaces — complete: `workPlan.ts`, part 1: the enums and the small records

```ts
// desktop/src/api/workPlan.ts — SW.18's desktop half, task 83.
//
// The W2 wire, as task 82's relay serves it. Field names are task 72's JSON tags verbatim
// (hubapi/work_plan.go); nothing is renamed on the way in or out, so a decode is JSON.parse and
// the golden vectors of hubapi/testdata/work_plan round-trip byte for byte (decision (b)).
import type { Digest, IssueHead, IssueRevision, PageRequest, Priority, User } from "./work";

/** Which human band a custom state sits in (05 §3.3). The five fact states are fixed. */
export type StateBand = "before_started" | "after_landed";
export interface CustomState {
  key: string;          // lowercase, dash, unique per team (hubapi.ValidCustomStateKey)
  name: string;         // what the row says
  band: StateBand;
  order: number;        // within the band, 0-based, dense
}
export interface TeamStates { teamId: string; states: readonly CustomState[]; rev: number }

/** A person's word for a feature; the facts sit beside it (05 §1 L-70). */
export type FeatureStatus = "planned" | "active" | "paused" | "done" | "cancelled";
export interface Feature {
  id: string;           // "ft-…"
  projectId: string;
  name: string;
  branchRef: string;    // reserved by task 76; phase 3 creates the git branch at the first Start
  briefKey: string;     // the branch brief, seeded on promote
  status: FeatureStatus;
  leadId: string | null;
  memberIds: readonly string[];
  targetDate: string | null;   // "YYYY-MM-DD"; a date, never a forecast (OD-27)
  priority: Priority;
  labelIds: readonly string[];
  createdAt: string;
  updatedAt: string;
}

export type CycleState = "upcoming" | "current" | "closed";
export interface Cycle {
  id: string;           // "cy-…"
  teamId: string;
  number: number;
  name: string;
  state: CycleState;
  startsOn: string;     // "YYYY-MM-DD"
  endsOn: string;
  closedAt: string | null;
}
export interface Cadence {
  teamId: string;
  weeks: number;        // 1..8
  cooldownDays: number; // 0..14
  startsOn: string;
  lookahead: number;    // how many upcoming cycles the janitor keeps ahead
  rev: number;
}
```

#### Interfaces — complete: `workPlan.ts`, part 2: views, templates, drafts, the rest

```ts
/** One clause of a saved view. No free text: every filter is a named field and a set. */
export interface ViewFilter {
  field: "state" | "assignee" | "label" | "priority" | "cycle" | "feature" | "parent" | "estimate";
  op: "is" | "is_not" | "any_of" | "none_of" | "is_set" | "is_not_set";
  values: readonly string[];
}
export type ViewGroupBy = "none" | "state" | "assignee" | "priority" | "cycle" | "feature";
export type ViewOrderBy = "manual" | "priority" | "updated" | "created" | "due";
export interface View {
  id: string;           // "vw-…"
  teamId: string;
  name: string;
  filters: readonly ViewFilter[];
  groupBy: ViewGroupBy;
  orderBy: ViewOrderBy;
  isTeamDefault: boolean;
  pinnedAsTab: boolean;  // a feature tab, 05 §1 L-68 (task 85)
  createdBy: string;
  archivedAt: string | null;
}

export interface Template {
  id: string;           // "tpl-…"
  scope: "team" | "org";
  teamId: string | null; // null for an org template
  name: string;
  title: string;
  body: string;
  labelIds: readonly string[];
  priority: Priority;
  estimate: number | null;
  archivedAt: string | null;
}

/** A private, unsent issue. Never leaves its author: a second user gets 404, not 403 (L-33). */
export interface Draft {
  id: string;           // "df-…"
  authorId: string;
  projectId: string | null;
  title: string;
  body: string;
  expiresAt: string;    // the janitor's sweep; shown, never predicted
  updatedAt: string;
}

/** Labels grouped, with at most one of an exclusive group on an issue (task 78). */
export interface LabelGroup {
  id: string;           // "lg-…"
  scope: "team" | "org";
  teamId: string | null;
  name: string;
  exclusive: boolean;
  labelIds: readonly string[];
}

export type EstimateScale = "none" | "fibonacci" | "linear" | "tshirt";
export interface TeamEstimate { teamId: string; scale: EstimateScale; rev: number }

/** One line of the digest (task 81): what changed, who changed it, the issue it was. */
export interface DigestItem {
  at: string;
  actorId: string;
  kind: string;         // the feed kind verbatim; the renderer maps it to a sentence
  subject: string;
  issueId: string | null;
  summary: string;      // the Hub's own words; the desktop never composes one
}

/** A closure proposal the assignee has not answered, or a deferral they chose (task 77). */
export type DeferralChoice = "next_cycle" | "backlog" | "keep";
export interface Deferral {
  proposalId: string;
  issueId: string;
  cycleId: string;
  proposedAt: string;
  answeredAt: string | null;
  choice: DeferralChoice | null;
}
```

`IssuePlan` (the issue's W2 half: `featureId`, `cycleId`, `parentId`, `estimate`, `dueOn`,
`rank`) is task 72's and rides on `IssueHead`, so it is declared here as an `interface` and
`work.ts`'s `IssueHead` is **not** edited — the relay returns it as a nested `plan` object, which
is what task 72's tag says. An edit to `IssueHead` would be a SW.15 change, and SW.15 is frozen
to additions.

#### Interfaces — complete: `workPlan.ts`, part 3: the routes and the client

```ts
/** The relay routes, mirroring hubclient.WorkPlanRoutes (task 82) one row for one row.
 *  workPlan.test.ts asserts the two lists are the same length and the same paths. */
export const WORK_PLAN_ROUTES = {
  customStates:  { verb: "GET",    path: "/v1/hub/work/teams/:teamId/states" },
  putCustomStates:{ verb: "PUT",   path: "/v1/hub/work/teams/:teamId/states" },
  setSubState:   { verb: "PUT",    path: "/v1/hub/work/issues/:issueId/sub-state" },
  createFeature: { verb: "POST",   path: "/v1/hub/work/projects/:projectId/features" },
  features:      { verb: "GET",    path: "/v1/hub/work/projects/:projectId/features" },
  feature:       { verb: "GET",    path: "/v1/hub/work/features/:featureId" },
  patchFeature:  { verb: "PATCH",  path: "/v1/hub/work/features/:featureId" },
  setFeature:    { verb: "PUT",    path: "/v1/hub/work/issues/:issueId/feature" },
  promote:       { verb: "POST",   path: "/v1/hub/work/issues/:issueId/promote" },
  // … 37 more rows, one per WorkPlanRoutes row, in the same order.
} as const;
export type WorkPlanRouteName = keyof typeof WORK_PLAN_ROUTES;

export interface WorkPlanClient {
  customStates(teamId: string): Promise<TeamStates>;
  putCustomStates(teamId: string, states: readonly CustomState[], rev: number): Promise<TeamStates>;
  createFeature(projectId: string, req: CreateFeatureRequest): Promise<Feature>;
  promoteToFeature(issueId: string, req: PromoteRequest): Promise<PromoteResult>;
  exportIssuesCsv(projectId: string, viewId?: string): Promise<Blob>;
  // … one per row; every body-carrying call takes an idempotencyKey the caller mints.
}
export function makeWorkPlanClient(fetchJson: FetchJson, fetchBlob: FetchBlob): WorkPlanClient;
```

`fetchJson` is task 37's (`task-37.md:368-395`), unchanged. `fetchBlob` is one new transport
function, nine lines, for the three raw routes (the two CSV exports and `cycles.ics`) — task 82's
decision (e) on this side of the wire. `workProblem` (task 37) gains W2's 19 codes from
`WorkPlanCodeStatus`, and `workPlan.test.ts` asserts the count is 19, so a code added on the Go
side without a desktop sentence fails here rather than reaching a user as a raw string.

#### Interfaces — complete: the replica and ops deltas

```ts
// replica.ts ⚑ — added kinds only. The maps are new; nothing existing changes shape.
export const W2_FEED_KINDS = [
  "team.states_changed", "issue.sub_state_changed",
  "feature.created", "feature.updated", "issue.feature_set", "issue.promoted",
  "team.cadence_set", "cycle.created", "cycle.updated", "cycle.closed", "cycle.rolled",
  "issue.cycle_set", "closure.proposed", "closure.answered",
  "issue.parent_set", "template.created", "template.updated", "template.archived",
  "label_group.created", "issue.rolled_up",
  "issue.planned", "issue.ranked", "issue.moved", "issue.blocks_resolved",
  "view.created", "view.updated", "view.archived", "view.team_default_set",
] as const;

// ops.ts ⚑ — added WorkOp members, each with its FieldPath so a refusal takes back only itself.
| { kind: "setCycle"; issueId: string; cycleId: string | null }        // field "plan.cycle"
| { kind: "setFeature"; issueId: string; featureId: string | null }    // field "plan.feature"
| { kind: "setParent"; issueId: string; parentId: string | null }      // field "plan.parent"
| { kind: "setEstimate"; issueId: string; estimate: number | null }    // field "plan.estimate"
| { kind: "setDueOn"; issueId: string; dueOn: string | null }          // field "plan.due"
| { kind: "setSubState"; issueId: string; key: string | null }         // field "subState"
| { kind: "rank"; issueId: string; before: string | null; after: string | null } // "plan.rank"
```

`cycle.rolled` is in the list because the scope's test names it; **no route part spells it** —
tasks 73 (g) and 77 spell `cycle.closed` and `cycle.created` for a rollover. Reported as a gap:
either task 77 adds `cycle.rolled`, or this list drops it and the order test uses
`cycle.created` / `cycle.closed`, which proves the same thing.

#### Tests to write first

`desktop/src/api/workPlan.test.ts`:

| Test | The mutant it kills |
|---|---|
| `goldenRoundTripPerType` (one `it` per type, driven by the fixture directory) | a renamed field — `targetDate` for `target_date` — that `tsc` alone would miss |
| `everyFixtureFileHasAType` | a fixture added on the Go side with no desktop type: the test fails on the unmatched file, so the wire cannot grow silently |
| `routesMatchWorkPlanRoutes` (reads task 82's Go table through a generated JSON, or fails naming it) | a path typed `/sub_state` where Go has `/sub-state` |
| `everyBodyCallSendsAnIdempotencyKey` | a create that retries without one |
| `workProblemKnowsAll19Codes` | a Hub code reaching a user as a raw string |
| `csvCallsReturnABlob` | a CSV parsed as JSON, mirroring task 82 (e) on this side |

`desktop/src/work/replica/replicaPlan.test.ts`:

| Test | The mutant it kills |
|---|---|
| `appliesCreatedThenRolledInOrder` | an out-of-order apply leaves a closed cycle open (decision (e)) |
| `ignoresAnUnknownKind` — asserts `stats.unknownKinds === 1` **and** that no map changed | a `throw` on an unknown kind, and equally a kind that "handles" by mutating |
| `aRefusedPlanOpTakesBackOnlyItsField` | `setCycle` refused also reverts a concurrent `setEstimate` |
| `draftsAreMine` drops another author's draft from the feed | a draft listed because the Hub leaked it; the client is the second gate |

`noPersistence.test.ts` ⚑ gains `workPlan.ts` and the new replica branches to its grep set, and
one new test: a draft created into the replica is absent after a fresh `createReplica` on the
same process (decision (d)).

#### Steps

1. Write `workPlan.test.ts`'s golden test against an absent `workPlan.ts`. Run red.
2. Write parts 1 and 2 (the records). Run: the golden test passes or names the field.
3. Write part 3 (routes, `WorkPlanClient`, `makeWorkPlanClient`, `fetchBlob`). Run the rest green.
4. Add `export * from "./workPlan";` to `work.ts` and the `api.work` wiring in `client.ts`.
   `npx tsc --noEmit` green.
5. Write `replicaPlan.test.ts`. Run red. Add the kinds, the maps and `stats.unknownKinds`. Green.
6. Add the seven op members and their `FieldPath`s; add `draftsAreMine` to `visibility.ts`.
7. Extend `noPersistence.test.ts`'s file set; add the remount test. Run the whole desktop suite.

#### Acceptance

```
cd desktop && npx tsc --noEmit
  no output

cd desktop && npx vitest run src/api/workPlan.test.ts
  every golden fixture named in the output; 0 skipped — a missing fixture directory FAILS

cd desktop && npx vitest run src/work/replica
  replicaPlan + replica + noPersistence all pass; noPersistence names workPlan.ts in its file list

cd desktop && npx vitest run
  the whole suite green: SW.15 and SW.16 additions broke nothing that was already passing
```

#### What this task does NOT claim

- **No surface.** Nothing renders. Tasks 84-96 own every W2 screen.
- **No parity row closed.** It serves every W2 desktop task and closes none itself.
- **Not the Hub's rules.** A wrong shape here is found by the golden fixtures; a wrong *rule* is
  tasks 75-81's and is not testable from this side.
- **Not offline.** The replica is memory only, by decision (d). Nothing here makes W2 work without
  the Hub; local mode's W2 story is task 96's.

#### Gaps found while writing this part

1. **The scope's file list says `work.ts` ⚑ gains the W2 types.** Decision (a) puts them in
   `workPlan.ts` with a one-line re-export. The skeleton's file map needs the new row.
2. **`cycle.rolled` is spelled by no route part.** Either task 77 adds it or this list drops it.
3. **The golden fixture directory is task 72's plan, not a path on the tree** (UNVERIFIED). If it
   lands elsewhere, step 1's glob follows it.
4. **Nothing yet generates the Go route table as JSON for `routesMatchWorkPlanRoutes`.** The
   simplest answer is a `go test` that writes `WorkPlanRoutes` to `testdata/routes.json`, owned by
   task 82; until it exists the test compares against a hand list and says **UNVERIFIED** in its
   failure message.
