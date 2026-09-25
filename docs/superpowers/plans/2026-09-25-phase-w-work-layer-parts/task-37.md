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

#### Interfaces — complete: `work.ts`, part 4: the local wire (tasks 24 and 25)

```ts
/** SW.10, task 24: POST /v1/work/start. `requestId` is 8-64 of [A-Za-z0-9._-]; a repeat of the
 *  same id returns the first result, so a double click starts one agent. */
export interface StartRequest {
  projectId: string; issueId: string; branch: string; requestId: string;
  accept?: boolean; route?: "global" | "au"; harness?: string;
}
export interface StartResult {
  issueKey: string; worktreeId: string; sessionId: string; branch: string;
  route: "global" | "au"; routeForced: boolean; undo: "undo_start";
}
/** `cost` is null in local mode: nothing is estimated (OD-27). */
export interface UndoStartResult { undone: true; stateRestored: IssueState | null; cost: null }
export interface DiscardStartResult { discarded: true; stateRestored: IssueState | null }

/** Task 25: one body for /v1/work/import/preview and /v1/work/import. The Linear key goes to the
 *  loopback control plane in this body and nowhere else; no slice holds it. */
export type ImportRequest =
  | { projectId: string; source: "linear"; linear: { apiKey: string; teamKey?: string } }
  | { projectId: string; source: "csv"; csv: { content: string } };
export interface ImportRefusal { at: string; reason: string }
export interface ImportPreview {
  ready: number; created: null; unchanged: null; refused: ImportRefusal[]; unmatchedLabels: null;
}
export interface ImportOutcome {
  created: number; unchanged: number; refused: ImportRefusal[]; unmatchedLabels: string[];
}
```

#### Interfaces — complete: `work.ts`, part 5: the route tables

```ts
export type Verb = "GET" | "POST" | "PATCH" | "PUT" | "DELETE";

/** Task 16's WorkRoutes, row for row and in its order (task-16.md:127-158): the Go method, the
 *  verb, the Hub's pattern. Never called directly: relayPath() derives the loopback path. */
export const WORK_RELAY = [
  ["SetProjectKey", "PUT", "/v1/projects/{projectId}/key"],
  ["CreateIssue", "POST", "/v1/projects/{projectId}/issues"],
  ["Issues", "GET", "/v1/projects/{projectId}/issues"],
  ["IssueByNumber", "GET", "/v1/projects/{projectId}/issues/by-number/{n}"],
  ["Issue", "GET", "/v1/issues/{issueId}"],
  ["PatchIssue", "PATCH", "/v1/issues/{issueId}"],
  ["WriteIssueRevision", "POST", "/v1/issues/{issueId}/revisions"],
  ["IssueRevisions", "GET", "/v1/issues/{issueId}/revisions"],
  ["IssueRevisionAt", "GET", "/v1/issues/{issueId}/revisions/{rev}"],
  ["RestoreIssueRevision", "POST", "/v1/issues/{issueId}/revisions/{rev}/restore"],
  ["MoveIssueState", "POST", "/v1/issues/{issueId}/state"],
  ["AttachBlob", "POST", "/v1/issues/{issueId}/attachments"],
  ["Attachments", "GET", "/v1/issues/{issueId}/attachments"],
  ["AddRelation", "POST", "/v1/issues/{issueId}/relations"],
  ["RemoveRelation", "DELETE", "/v1/issues/{issueId}/relations/{kind}/{otherId}"],
  ["Labels", "GET", "/v1/teams/{teamId}/labels"],
  ["CreateLabel", "POST", "/v1/teams/{teamId}/labels"],
  ["ArchiveLabel", "POST", "/v1/labels/{labelId}/archive"],
  ["TeamPolicy", "GET", "/v1/teams/{teamId}/work-policy"],
  ["PutTeamPolicy", "PUT", "/v1/teams/{teamId}/work-policy"],
  ["Comments", "GET", "/v1/issues/{issueId}/comments"],
  ["CreateComment", "POST", "/v1/issues/{issueId}/comments"],
  ["React", "POST", "/v1/comments/{commentId}/reactions"],
  ["ResolveComment", "POST", "/v1/comments/{commentId}/resolve"],
  ["Subscribe", "PUT", "/v1/issues/{issueId}/subscription"],
  ["Subscriptions", "GET", "/v1/me/subscriptions"],
  ["SearchIssues", "GET", "/v1/issues/search"],
  ["ImportBatch", "POST", "/v1/projects/{projectId}/imports"],
  ["EraseIssue", "POST", "/v1/issues/{issueId}/erase"],
  ["EraseComment", "POST", "/v1/comments/{commentId}/erase"],
] as const satisfies ReadonlyArray<readonly [string, Verb, `/v1/${string}`]>;

/** Task 16's Relay(): the Hub path with its leading "/v1" replaced by "/v1/hub/work". */
export const relayPath = (hubPath: string): string => "/v1/hub/work" + hubPath.slice("/v1".length);

/** Tasks 24 and 25: served by the control plane itself, never relayed. */
export const WORK_LOCAL = [
  ["start", "POST", "/v1/work/start"],
  ["undoStart", "POST", "/v1/work/start/{sessionId}/undo"],
  ["discardStart", "POST", "/v1/work/start/{sessionId}/discard"],
  ["importPreview", "POST", "/v1/work/import/preview"],
  ["importRun", "POST", "/v1/work/import"],
] as const satisfies ReadonlyArray<readonly [string, Verb, `/v1/work/${string}`]>;

export type RelayMethod = (typeof WORK_RELAY)[number][0];
export type LocalMethod = (typeof WORK_LOCAL)[number][0];
```

#### Interfaces — complete: `work.ts`, part 6: `WorkClient`

```ts
/** SW.15. One method per row of WORK_RELAY (Go's name in lower camel case) and per row of
 *  WORK_LOCAL. Every method rejects with the client's ApiError and nothing else. */
export interface WorkClient {
  setProjectKey(projectId: string, key: string): Promise<ProjectKey>;
  createIssue(projectId: string, req: CreateIssueRequest): Promise<IssueWithRevision>;
  issues(projectId: string, q?: IssueQuery, page?: PageRequest): Promise<Page<IssueHead>>;
  /** How a typed alias ("EP-142") becomes an issue: the caller knows the project. */
  issueByNumber(projectId: string, n: number): Promise<IssueWithRevision>;
  issue(issueId: string): Promise<IssueWithRevision>;
  patchIssue(issueId: string, req: PatchIssueRequest): Promise<IssueHead>;
  writeIssueRevision(issueId: string, req: WriteIssueRevisionRequest): Promise<IssueRevision>;
  issueRevisions(issueId: string, page?: PageRequest): Promise<Page<IssueRevision>>;
  issueRevisionAt(issueId: string, rev: number): Promise<IssueRevision>;
  /** Writes a NEW revision copying `fromRevision`; the path's {rev} is that number. */
  restoreIssueRevision(issueId: string, fromRevision: number): Promise<IssueRevision>;
  moveIssueState(issueId: string, move: StateMove): Promise<IssueHead>;
  attachBlob(issueId: string, digest: Digest): Promise<IssueAttachment>;
  attachments(issueId: string): Promise<IssueAttachment[]>;
  addRelation(issueId: string, req: RelationRequest): Promise<IssueHead>;
  removeRelation(issueId: string, kind: RelationKind, otherId: string): Promise<IssueHead>;
  labels(teamId: string): Promise<Label[]>;
  createLabel(teamId: string, req: CreateLabelRequest): Promise<Label>;
  archiveLabel(labelId: string): Promise<Label>;
  teamPolicy(teamId: string): Promise<TeamPolicy>;
  putTeamPolicy(teamId: string, req: PutTeamPolicyRequest): Promise<TeamPolicy>;
  comments(issueId: string, page?: PageRequest): Promise<Page<IssueComment>>;
  createComment(issueId: string, req: CreateCommentRequest): Promise<IssueComment>;
  react(commentId: string, emoji: string, on: boolean): Promise<IssueComment>;
  resolveComment(commentId: string, resolved: boolean): Promise<IssueComment>;
  subscribe(issueId: string, subscribed: boolean): Promise<Subscription>;
  subscriptions(page?: PageRequest): Promise<Page<Subscription>>;
  searchIssues(q: IssueSearchQuery): Promise<IssueHead[]>;
  importBatch(projectId: string, batch: ImportBatch): Promise<ImportResult>;
  /** Org admin only. Ids, keys, digests and revision numbers stay; the texts read as erased. */
  eraseIssue(issueId: string, reason: string): Promise<WorkEraseResponse>;
  eraseComment(commentId: string, reason: string): Promise<WorkEraseResponse>;

  // ── local: tasks 24 and 25 ──
  start(req: StartRequest): Promise<StartResult>;
  /** 409 first_write once the agent has written: offer Stop and discard (workProblem). */
  undoStart(sessionId: string): Promise<UndoStartResult>;
  /** `confirm` "" asks for the token (409 confirm_required); the token discards. */
  discardStart(sessionId: string, confirm: string): Promise<DiscardStartResult>;
  importPreview(req: ImportRequest): Promise<ImportPreview>;
  importRun(req: ImportRequest): Promise<ImportOutcome>;
}
```

`importRun`, not `import`: `import` is a reserved word, legal as a property name but not as a
destructured binding, and a caller will destructure. `start` and the four others keep task 24's
and 25's words (`undo_start`, `/import/preview`).

#### Interfaces — complete: `work.ts`, part 7: `makeWorkClient`

```ts
/** The client's own transport, handed in by createClient: verb, route (path + query), optional
 *  JSON body → the parsed body, or an ApiError. It carries the base URL and the timeout. */
export type WorkSend = (verb: Verb, route: string, body?: unknown) => Promise<unknown>;

/** Fills each {…} in order with encodeURIComponent, so "is-1/../teams" stays one segment. A
 *  count mismatch is a bug in this file, not a refusal; the route test catches it first. */
function fill(pattern: string, args: ReadonlyArray<string | number>): string {
  let i = 0;
  const out = pattern.replace(/\{[^}]+\}/g, () => {
    if (i >= args.length) throw new Error(`work: too few arguments for ${pattern}`);
    return encodeURIComponent(String(args[i++]));
  });
  if (i !== args.length) throw new Error(`work: too many arguments for ${pattern}`);
  return out;
}

/** An absent or "" value adds nothing; an array repeats its key (?state=a&state=b). */
function query(pairs: ReadonlyArray<readonly [string, string | number | readonly string[] | undefined]>): string {
  const q = new URLSearchParams();
  for (const [k, v] of pairs) {
    if (v === undefined || v === "") continue;
    for (const one of Array.isArray(v) ? v : [v]) q.append(k, String(one));
  }
  const s = q.toString();
  return s === "" ? "" : `?${s}`;
}
const pageQ = (p?: PageRequest) => query([["after", p?.after], ["limit", p?.limit]]);

export function makeWorkClient(send: WorkSend): WorkClient {
  const relay = <T,>(m: RelayMethod, args: ReadonlyArray<string | number>, body?: unknown, q = ""): Promise<T> => {
    const [, verb, hub] = WORK_RELAY.find((r) => r[0] === m)!;
    return send(verb, relayPath(fill(hub, args)) + q, body) as Promise<T>;
  };
  const local = <T,>(m: LocalMethod, args: ReadonlyArray<string>, body?: unknown): Promise<T> => {
    const [, verb, path] = WORK_LOCAL.find((r) => r[0] === m)!;
    return send(verb, fill(path, args), body) as Promise<T>;
  };
  const items = async <T,>(p: Promise<Page<T>>): Promise<T[]> => (await p).items;

  return {
    setProjectKey: (projectId, key) => relay("SetProjectKey", [projectId], { key }),
    createIssue: (projectId, req) => relay("CreateIssue", [projectId], req),
    issues: (projectId, q, page) =>
      relay("Issues", [projectId], undefined,
        query([["state", q?.states], ["assignee", q?.assignee], ["label", q?.label],
               ["after", page?.after], ["limit", page?.limit]])),
    issueByNumber: (projectId, n) => relay("IssueByNumber", [projectId, n]),
    issue: (issueId) => relay("Issue", [issueId]),
    patchIssue: (issueId, req) => relay("PatchIssue", [issueId], req),
    writeIssueRevision: (issueId, req) => relay("WriteIssueRevision", [issueId], req),
    issueRevisions: (issueId, page) => relay("IssueRevisions", [issueId], undefined, pageQ(page)),
    issueRevisionAt: (issueId, rev) => relay("IssueRevisionAt", [issueId, rev]),
    restoreIssueRevision: (issueId, fromRevision) =>
      relay("RestoreIssueRevision", [issueId, fromRevision], { fromRevision }),
    moveIssueState: (issueId, move) => relay("MoveIssueState", [issueId], move),
    attachBlob: (issueId, digest) => relay("AttachBlob", [issueId], { digest }),
    attachments: (issueId) => items(relay("Attachments", [issueId])),
    addRelation: (issueId, req) => relay("AddRelation", [issueId], req),
    removeRelation: (issueId, kind, otherId) => relay("RemoveRelation", [issueId, kind, otherId]),
    labels: (teamId) => items(relay("Labels", [teamId])),
    createLabel: (teamId, req) => relay("CreateLabel", [teamId], req),
    archiveLabel: (labelId) => relay("ArchiveLabel", [labelId]),
    teamPolicy: (teamId) => relay("TeamPolicy", [teamId]),
    putTeamPolicy: (teamId, req) => relay("PutTeamPolicy", [teamId], req),
    comments: (issueId, page) => relay("Comments", [issueId], undefined, pageQ(page)),
    createComment: (issueId, req) => relay("CreateComment", [issueId], req),
    react: (commentId, emoji, on) => relay("React", [commentId], { emoji, on }),
    resolveComment: (commentId, resolved) => relay("ResolveComment", [commentId], { resolved }),
    subscribe: (issueId, subscribed) => relay("Subscribe", [issueId], { subscribed }),
    subscriptions: (page) => relay("Subscriptions", [], undefined, pageQ(page)),
    searchIssues: (q) =>
      items(relay("SearchIssues", [], undefined,
        query([["q", q.q], ["projectId", q.projectId], ["limit", q.limit]]))),
    importBatch: (projectId, batch) => relay("ImportBatch", [projectId], batch),
    eraseIssue: (issueId, reason) => relay("EraseIssue", [issueId], { reason }),
    eraseComment: (commentId, reason) => relay("EraseComment", [commentId], { reason }),
    start: (req) => local("start", [], req),
    undoStart: (sessionId) => local("undoStart", [sessionId]),
    discardStart: (sessionId, confirm) => local("discardStart", [sessionId], { confirm }),
    importPreview: (req) => local("importPreview", [], req),
    importRun: (req) => local("importRun", [], req),
  };
}
```

`ArchiveLabel` and `undoStart` send no body, as the Go routes take none (`task-24.md:107`). The
`!` after each `find` is safe because `m` is typed to a row's name.

#### Interfaces — complete: `work.ts`, part 8: `workProblem` and the wire key lists

```ts
import type { ApiError } from "./client";   // type-only: client.ts imports this file at runtime

/** What a Work surface does with a failure (decision 5). `refused` carries the reason in the
 *  server's own words; `unavailable` means nothing was decided and a retry is honest. */
export type WorkProblem =
  | { kind: "first_write"; message: string }                                   // offer Stop and discard
  | { kind: "confirm"; message: string; details: Record<string, string> }      // discard's token step
  | { kind: "signed_out"; message: string }                                    // sign in again (OD-6)
  | { kind: "unavailable"; code: string; message: string }
  | { kind: "refused"; code: string; message: string; details: Record<string, string> };

const UNAVAILABLE: ReadonlySet<string> = new Set([
  "network_unavailable", "timeout", "upstream_unavailable", "hub_not_configured",
  "import_source_unavailable", "backend_unavailable",
]);

export function workProblem(e: ApiError): WorkProblem {
  const { code, message, details } = e;
  if (code === "first_write") return { kind: "first_write", message };
  if (code === "confirm_required") return { kind: "confirm", message, details };
  if (code === "signed_out") return { kind: "signed_out", message };
  if (UNAVAILABLE.has(code)) return { kind: "unavailable", code, message };
  return { kind: "refused", code, message, details };
}

/** The wire's key lists, as values, for the golden-vector test. `Record<keyof T, true>` makes the
 *  compiler hold each list to its type both ways: a missing key and an extra key are both errors,
 *  so renaming a field on the desktop changes the list, and the vectors then disagree. */
const keys = <T,>(r: Record<keyof T, true>): string[] => Object.keys(r).sort();
export const WIRE_KEYS = {
  content: keys<IssueRevisionContent>({ title: true, description: true, serves: true, implements: true }),
  text: keys<AuthoredText>({ text: true, author: true }),
  serve: keys<Serve>({ briefKey: true, criterionId: true }),
};
```

The `import type` line sits at the top of `work.ts` with the file's other imports; it is printed
here beside the one function that uses it.

#### Interfaces — complete: the deltas on ⚑ `desktop/src/api/client.ts`

```ts
// :10, a new import beside the store types
import { makeWorkClient, type WorkClient } from "./work";

// :40-48, ProblemCode: these eleven join the union, after "non_loopback_publish"
  /* the Work wire (task 37): the relay's (task 16), the local start's (task 24), import's (task 25) */
  | "hub_not_configured" | "upstream_unavailable" | "signed_out"
  | "first_write" | "confirm_required" | "start_not_found" | "issue_not_startable" | "git_not_ready"
  | "import_source_refused" | "import_source_unavailable" | "too_large"

// :132, CockpitClient gains one member, after openAttach
  /** SW.15: the Work relay and the local Work routes, over this client's transport. */
  readonly work: WorkClient;

// :170
type Method = "GET" | "POST" | "PATCH" | "PUT" | "DELETE";

// inside createClient, after `createSession` (:301-302), and `work` in the returned object
  const work = makeWorkClient(async (verb, route, body) => parseJson<unknown>(await send(verb, route, body)));
  // …
    work,
```

The comment above `ProblemCode` (`:35-39`) says "The sixteen codes"; it becomes "twenty-seven".
Hub codes the list does not name (`policy_refused`, `revision_conflict`, `issue_not_found`, …)
still arrive intact through `toProblem`, and `workProblem` files them as `refused`.

