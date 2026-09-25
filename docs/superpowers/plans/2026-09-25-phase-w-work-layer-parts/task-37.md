### Task 37: desktop Work wire types, relay calls and `workSlice`; `Lens` gains `"work"`

Stream **WD1**, defines seam **SW.15** (the desktop Work types and `workSlice`), about 2.5 hours
(an estimate, not a measurement). Depends on task 05 (SW.2 frozen at WS0) and task 20
(`src/test/workFixtures.ts`). It also reads three contracts it does not wait to be built: task 16's
route table (`task-16.md:127-158`), task 24's start wire (SW.10, `task-24.md:94-115`) and task 25's
import wire (`task-25.md:98-113`). The skeleton's row names 05 and 20 only; 16, 24 and 25 are
contract dependencies, not build ones, because every test here runs on a fetch stub (a gap, named
in the return).

The desktop learns the Work wire here, once. After this task a Work surface imports its types from
`api/work` and calls `api.work.*`; nothing else in `src/` names a Work path or a Work field.

**Decisions made here, so nobody re-derives them:**

1. **No mapping layer.** Task 03's JSON tags are already camelCase (`task-03.md:266-411`), so each
   TypeScript type is the wire's field names verbatim, and a decode is `JSON.parse`. The one
   rename is the type name `IssueComment` for Go's `Comment`, because `Comment` is the DOM's
   global type in `lib.dom.d.ts` and shadowing it in a component file is a trap. `Serve` keeps
   Go's name, because tasks 43, 46 and 47 already import it (`task-47.md:121`); `ServesEntry` is
   exported as an alias for the skeleton's word.
2. **One method per route, derived paths.** `WORK_RELAY` mirrors task 16's thirty `WorkRoutes` rows
   in its order, as `[method, verb, hubPattern]`; the relay path is derived exactly as task 16's
   `Relay()` derives it (`/v1/hub/work` + the Hub pattern minus its leading `/v1`), so the two
   ends cannot drift by a typo. Method names are Go's in lower camel case (`MoveIssueState` →
   `moveIssueState`). The five local routes of tasks 24 and 25 are a second table, `WORK_LOCAL`.
3. **Every call goes to the loopback control plane.** `makeWorkClient` takes the client's own
   `send`, so a Work call inherits the base URL (`DEFAULT_BASE_URL`, `client.ts:30`), the timeout
   and the `ApiError` mapping (`client.ts:193-222`). There is no Hub URL anywhere in `desktop/src`
   (the relay rule, hub-contract §4.3), and a test holds that.
4. **Lists.** Every list route answers `{ items, next }` (hub-contract §1; the relay's `listBody`,
   `task-16.md:765`). Where the Go client returns a slice (`Labels`, `Attachments`,
   `SearchIssues`, `task-04.md:207, 222, 227`) the TypeScript method returns the array; where it
   returns a `Page`, so does this.
5. **Problems.** `ProblemCode` gains the eleven codes the Work wire can answer that the desktop must
   tell apart (listed at the interface). `toProblem` already passes an unknown code through intact
   (`client.ts:202-209`), so a Hub code this list does not name still arrives with its message.
   `workProblem(e)` sorts an `ApiError` into five kinds. Two are the scope's: **`first_write`**,
   the one answer where *Stop and discard* must be offered instead of *Undo* (task 24, `:37`), and
   **`refused`**, a reason the Hub or the control plane gave, shown in its own words with its
   `details` (a `policy_refused` naming `priorityToLeaveTriage`, `task-16.md:25-28`). The other
   three are `confirm` (discard's first call, `task-24.md:113`), `signed_out` and `unavailable`
   (nothing was decided). `refused` is a kind, not a wire code: no server sends it.
6. **`PUT` joins the client's `Method` union** (`client.ts:170`). Three Work routes use it
   (`SetProjectKey`, `PutTeamPolicy`, `Subscribe`); the built client has never sent one.
7. **`workSlice` holds view state only.** Where the reader is (project, branch), which issue is
   open at which revision, the multi-select and the cursor. It holds no Hub record: those come from
   the replica (SW.16, tasks 55-58), and until then task 38 reads task 20's fixtures. Nothing of it
   is persisted: "neither the renderer nor the control plane persists Hub data to disk"
   (`hub/docs/superpowers/adr/ADR-H1-hub-architecture.md:467-474`, read in the hub worktree).
   The cursor and selection are ids, but an id list of open issues is a list of Hub records the
   reader was looking at, so it is not persisted either.
8. **`"work"` is a `Lens`, not a tab.** The sub-rail's `LENSES` (`components/subrail/SubRail.tsx:9-16`)
   stays six, because tabs read as stages (05 §4.1, 07 D M7). Work opens from `G W` (task 54),
   Plan, and every issue chip. `AppShell.tsx`'s `LensRouter` has a `default` arm (`:67-68`) that
   renders nothing, so `"work"` compiles and draws nothing until task 38 adds its arm. The persisted
   `lens` (`store.ts:46`) may hold `"work"`; it is a preference, not Hub data.
9. **Task 20's hand-mirrored types are replaced by re-exports** from `api/work`, as task 20's
   header asks (`task-20.md:145-146`). The builders and `Criterion` and `WorkEvent` stay (neither is
   an SW.1 type); `Comment` stays as the fixtures' name, re-exported from `IssueComment`.

**Files** (the skeleton's file map lists `workSlice.ts`, `types.ts` and `store.ts` for this task;
it omits `api/work.ts`, `client.ts`, `workFixtures.ts` and the tests, a gap named in the return):
- `desktop/src/api/work.ts` — the wire types, the two route tables, `WorkClient`,
  `makeWorkClient`, `workProblem` (new)
- `desktop/src/api/work.test.ts` — seven tests (new)
- `desktop/src/api/client.ts` ⚑ — `Method` gains `"PUT"` (`:170`); `ProblemCode` gains eleven codes
  (`:40-48`); `CockpitClient` (`:132`) gains `work: WorkClient`; `createClient` (`:236`) builds it
  (modified)
- `desktop/src/store/slices/workSlice.ts` — the view model (new)
- `desktop/src/store/types.ts` ⚑ — `Lens` (`:173`) gains `"work"`; its comment is rewritten (modified)
- `desktop/src/store/store.ts` ⚑ — `CockpitState` (`:14`) adds `WorkSlice`; the store spreads
  `createWorkSlice`; `resetCockpit` restores `initialWorkState`; `partialize` is **not** touched (modified)
- `desktop/src/store/store.test.ts` ⚑ — two tests (modified)
- `desktop/src/components/subrail/SubRail.test.tsx` — one test (new; the sub-rail has none today)
- `desktop/src/test/workFixtures.ts` ⚑ (task 20) — the mirrored types become re-exports (modified)

#### Interfaces — complete: `desktop/src/api/work.ts`, part 1: the records

```ts
// desktop/src/api/work.ts — SW.15, task 37.
//
// The Work wire, as the loopback control plane relays it (task 16) and as tasks 24 and 25 serve
// it locally. Field names are task 03's JSON tags verbatim (hubapi/work.go); nothing is renamed on
// the way in or out, so a decode is JSON.parse and a golden vector round-trips byte for byte.

/** The nine states (05 §3.3). No "shipped": it is derived from the release, never stored. */
export type IssueState =
  | "triage" | "backlog" | "todo" | "started" | "in_review"
  | "landed" | "done" | "cancelled" | "duplicate";
export const ISSUE_STATES: readonly IssueState[] = [
  "triage", "backlog", "todo", "started", "in_review", "landed", "done", "cancelled", "duplicate",
];
export type Priority = "none" | "urgent" | "high" | "medium" | "low";
export type RelationKind = "blocks" | "duplicates" | "relates";
export type NotTiedReason = "chore" | "spike" | "imported" | "support";
export type IssueSource = "person" | "orchestrator" | "intake";
export type SubscriptionCause = "create" | "assign" | "mention" | "comment" | "manual";
export type MoveCause =
  | "hand" | "lane_bound" | "lane_queued" | "land_trailer" | "evidence_observed" | "evidence_recorded";
/** The done rule's result word; "" before landed (SW.4). */
export type Proof = "" | "proven" | "proven_self_attested" | "not_proven" | "no_criteria" | "waiting";
/** "sha256:" + 64 lowercase hex (hubapi.Digest). */
export type Digest = string;

export interface User { id: string; displayName: string; email: string }
export interface AuthoredText { text: string; author: "H" | "A" }
/** One brief criterion an issue revision serves: the thread's link (task 02's `Serve`). */
export interface Serve { briefKey: string; criterionId: string }
export type ServesEntry = Serve;
export interface IssueRevisionContent {
  title: AuthoredText; description: AuthoredText; serves: Serve[]; implements: string[];
}
/** `reason` is "" unless `kind` is not_tied, and then it is required. */
export interface ThreadState {
  kind: "on_thread" | "project_level" | "not_tied"; reason: "" | NotTiedReason;
}
export interface Relation { kind: RelationKind; issueId: string }
export interface ExternalRef { source: string; externalId: string; url: string; authorName: string }

export interface IssueHead {
  id: string; projectId: string; number: number; alias: string;
  state: IssueState; stateReason: string; proof: Proof; landCommit: string;
  assignee: User | null; priority: Priority; labels: string[]; relations: Relation[];
  headRevision: number; headKey: string; threadState: ThreadState;
  source: IssueSource; external: ExternalRef | null;
  createdAt: string; createdBy: User; updatedAt: string;
}
export interface IssueRevision {
  issueId: string; revision: number; key: string; digest: Digest; projectId: string;
  content: IssueRevisionContent; writtenAt: string; writtenBy: User;
  restoredFrom: number | null; erasedAt: string | null;
}
export interface IssueWithRevision { head: IssueHead; revision: IssueRevision }
export interface ProjectKey { projectId: string; key: string; setAt: string; setBy: User }
```

#### Interfaces — complete: `work.ts`, part 2: the team records, comments and lists

```ts
/** Team-owned (L-17). `color` is a token name ("accent-2"), never a hex. */
export interface Label { id: string; teamId: string; name: string; color: string }
export interface TeamPolicy {
  teamId: string; defaultState: IssueState; priorityToLeaveTriage: boolean;
  stopAtLandedWithoutServes: boolean; updatedAt: string | null; updatedBy: User | null;
}
/** Fields that do not apply are 0 or "", never omitted (task 12 declares the kinds). */
export interface CommentAnchor {
  kind: "text" | "criterion" | "span" | "hunk";
  revision: number; start: number; end: number; criterionId: string; ref: string;
}
export interface Reaction { emoji: string; users: User[] }
/** Go's `Comment`. Append-only: there is no edit route; a correction is a new comment. */
export interface IssueComment {
  id: string; issueId: string; parentId: string | null; kind: "comment"; body: AuthoredText;
  anchor: CommentAnchor | null; mentions: string[]; reactions: Reaction[];
  resolvedAt: string | null; resolvedBy: User | null;
  createdAt: string; createdBy: User; erasedAt: string | null;
}
export interface Subscription { issueId: string; user: User; cause: SubscriptionCause; subscribed: boolean }
export interface IssueRef { issueId: string; number: number; externalId: string }
export interface ImportResult { created: IssueRef[]; existing: IssueRef[]; unmatchedLabels: string[] }
export interface IssueAttachment {
  issueId: string; digest: Digest; contentType: string; size: number; addedAt: string; addedBy: User;
}
export interface WorkEraseResponse {
  issueId: string; commentId: string; erasedRevisions: number; erasedComments: number; erasedAt: string;
}

/** Every list route's body (hub-contract §1). `next` is null on the last page. */
export interface Page<T> { items: T[]; next: string | null }
/** `?after=` and `?limit=`; absent means "from the start" and 50 (hubapi.PageRequest). */
export interface PageRequest { after?: string; limit?: number }
```

#### Interfaces — complete: `work.ts`, part 3: request bodies and queries

```ts
export interface CreateIssueRequest {
  title: AuthoredText; description: AuthoredText; serves: Serve[]; implements: string[];
  /** "" is the team policy's defaultState; else one of triage, backlog, todo. */
  state: "" | "triage" | "backlog" | "todo";
  priority: "" | Priority; assigneeId: string | null; labels: string[];
  /** Required when `serves` is empty. */
  notTiedReason: "" | NotTiedReason;
}
/** Head fields only; never a state (a state moves only through StateMove). An absent key keeps.
 *  `assigneeId` is tri-state on the wire: absent keeps, null clears, "u-…" sets. JSON.stringify
 *  drops an `undefined` key, which is exactly "absent". */
export interface PatchIssueRequest {
  priority?: Priority; assigneeId?: string | null; labels?: string[]; notTiedReason?: NotTiedReason;
}
/** A stale baseRevision is 409 revision_conflict and writes nothing. */
export interface WriteIssueRevisionRequest { baseRevision: number; content: IssueRevisionContent }
/** SW.4. `landCommit` rides with land_trailer, `evidence` (ev- ids) with the evidence causes,
 *  `reason` with a hand-set done. The desktop only ever sends cause "hand" (rule 2). */
export interface StateMove {
  to: IssueState; cause: MoveCause; reason: string; landCommit: string; evidence: string[];
}
export interface RelationRequest { kind: RelationKind; issueId: string }
export interface CreateLabelRequest { name: string; color: string }
export interface PutTeamPolicyRequest {
  defaultState: "triage" | "backlog" | "todo"; priorityToLeaveTriage: boolean; stopAtLandedWithoutServes: boolean;
}
export interface CreateCommentRequest {
  body: AuthoredText; parentId: string | null; anchor: CommentAnchor | null; mentions: string[];
}
export interface ImportItem {
  externalId: string; url: string; title: string; description: string;
  priority: Priority; labels: string[]; authorName: string; createdAt: string;
}
export interface ImportBatch { source: "linear" | "csv"; items: ImportItem[] }

/** GET …/issues. An absent field does not filter. `states` repeats as ?state=. */
export interface IssueQuery { states?: IssueState[]; assignee?: string; label?: string }
/** GET /v1/issues/search: a title substring, readable projects only, newest first. */
export interface IssueSearchQuery { q: string; projectId?: string; limit?: number }
```

The query parameter names (`state`, `assignee`, `label`, `q`, `projectId`, `after`, `limit`) are
read from the Go comments (`task-03.md:557-569`, hub `types.go:201`); task 16 decodes them with
`DecodeQuery` (`task-16.md:609`, `:750`), whose name rule its part does not print. They are
**UNVERIFIED** until task 16 merges; a mismatch is a one-line change in `query()` below, and
task 17's end-to-end run is where it would show.

