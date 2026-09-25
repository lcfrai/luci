### Task 169: desktop W5 wire types and replica kinds

W5, stream WD, the desktop half of seam **SW.21**, as tasks 37, 83 and 125 were for W1, W2 and W4.
SW.15 (the desktop Work types) and SW.16 (the replica) grow by **added** types, calls and kinds
only. Depends on task 125 (the W4 wire, the last one added before this: `task-125.md`) and task 144
(the fake's W5 rules, whose event kinds this task copies by name). It also reads task 143's types
and codes, task 168's route table, task 37's `WORK_RELAY`, `relayPath`, `WorkSend`, `WIRE_KEYS`
and `workProblem` (`task-37.md`), and task 57's `classify` and `admit` (`task-57.md`). Estimate
0.25 (2 h).

After this task the desktop can name every W5 record, call every W5 relay route, and hold the
**Work** W5 records (requests, their thread, links and loops) in its replica. The **admin** W5
records (apps, keys, webhooks, deliveries, connectors, the harness allow-list and team skills)
never enter the replica. It draws nothing: tasks 170 to 173 do.

**Decisions made here, so nobody re-derives them:**

1. **Names come from the task that owns the route.** This task's scope names `SyncLogEntry`,
   `LoopVersion`, `LoopTrigger`, `LoopBound`, `TeamAgentPolicy` and `DigestSettings.teams`; task
   143's paragraph (`…-scopes/tasks-141-160.md:139-174`) names `SyncRun`, a flat `Loop`,
   `HarnessAllow` plus `TeamSkill`, and `TeamsBinding`; tasks 162-163 (the loop routes) name
   `Loop`, `LoopVersion`, `LoopTrigger`, `LoopBound` and `LoopRun` field by field. The desktop
   mirrors Go, so: the loop types are 162-163's, the rest task 143's. `SyncLogEntry` is a desktop
   **view** derived from a `SyncRun` (decision 7), and `TeamAgentPolicy` is task 143's
   `HarnessAllow` plus `TeamSkill[]`. The mismatches are G1, below.
2. **Field for field with Go, by value, not by eye.** Each W5 type has a `WIRE_KEYS` entry held to
   its type both ways by task 37's `keys<T>()`, and a golden test reads task 143's
   `control-plane/internal/hub/hubapi/testdata/work-intake/*.json` **by path** and requires every
   golden's keys to equal the type's list. A field renamed on either side (`quoteText` for
   `quote`) fails that test even where `tsc` passes, because `tsc` never sees the Go side.
3. **Kinds are copied by name, never retyped.** `W5_FEED_KINDS` is a literal list, and a test reads
   task 144's `fake_work_intake.go` by path, extracts every `Kind: "…"` literal it appends, and
   requires the Work subset and the admin subset together to equal that set. A kind the Hub adds
   later and this list lacks is **ignored** by the replica, never thrown (hub-contract §1).
4. **Admin records are not replica kinds.** Apps, keys, webhooks, deliveries, connectors, the
   harness allow-list and team skills are fetched on demand through `WorkClient` by task 173 and
   held in component state only. Their feed kinds are in `W5_ADMIN_KINDS`, which the replica reads
   **only** to skip; nothing in the replica, `workSlice` or `localStorage` keeps them (§3.1, *no Hub
   data on disk*; ADR-H1). Task 55's no-persistence grep is extended to the new modules.
5. **A secret lives in one variable, once.** `CreatedAPIKey`, `CreatedWebhook` and
   `CreatedHostHook` are the only W5 types that carry `secret` (task 143). The client returns them
   and nothing else stores them: `ops.ts` has no op for them, `workSlice` has no field typed with
   them, and a test walks the slice and the replica after the create resolves and finds no string
   equal to the secret.
6. **A loop edit is a new version.** `ops.ts` gains `loop.version`, which posts a whole
   `LoopVersion` body to task 162's `POST …/versions` (owner only); the Hub answers version n+1 and
   the old one stays listed. There is no op that patches a version in place; the only patch is
   the head's `state` (task 162's `PATCH`), which a team admin may use to pause.
7. **The sync log is a view over `SyncRun`, not a wire type.** Task 143's `SyncRun` carries counts
   (`created`, `existing`, `commentsIn`, `commentsOut`) and `errors [{externalId, code}]`. Task
   171's log needs one row per external item. `syncLogRows(run)` derives one `SyncLogEntry` per
   `errors` element (result `refused`, the reason from the code) and one summary row per run; it
   never invents per-item `imported` rows the Hub did not name. The gap, below, asks task 150 for
   per-item rows.
8. **The request quote is team-scoped.** A `Request` enters the replica only when the caller is a
   member of `request.teamId` (task 57's `admit`, team branch); a `request.*` event for another
   team's request is skipped, even when the caller can read the linked issue's project by grant.

**Files:**
- `desktop/src/api/work.ts` ⚑ (task 37; tasks 83 and 125 before this) — the W5 records, request
  bodies, `WORK_RELAY_W5`, the `WorkIntakeClient` members, `WIRE_KEYS` entries (modified)
- `desktop/src/api/work.test.ts` ⚑ — the W5 goldens, the relay table against task 168's (modified)
- `desktop/src/api/client.ts` ⚑ — the ten W5 codes join `ProblemCode` (modified)
- `desktop/src/work/replica/kinds.ts` ⚑ (task 125) — `W5_FEED_KINDS`, `W5_ADMIN_KINDS` (modified)
- `desktop/src/work/replica/kinds.test.ts` ⚑ (task 125) — the copied-by-name test for W5 (modified)
- `desktop/src/work/replica/replica.ts` ⚑ (task 55) — the W5 record maps and their load (modified)
- `desktop/src/work/replica/ops.ts` ⚑ (task 56) — the W5 ops, no loop patch, no secret op (modified)
- `desktop/src/work/replica/visibility.ts` ⚑ (task 57) — the request team rule (modified)
- `desktop/src/work/replica/intake.test.ts` — order, admin skip, secret absence, unknown kind (new)

**What is on the tree today (verified 2026-09-26).** On this branch's tree, parts exist for tasks
37, 42, 56, 57 and 125 (`…-parts/task-37.md`, `task-42.md`, `task-56.md`, `task-57.md`,
`task-125.md`). There is **no part** for tasks 55, 143, 144 or 146-168, so every name taken from
them is their scope's (`…-scopes/tasks-141-160.md:124-229`, `tasks-161-180.md:29-365`) and
**UNVERIFIED**; task 143's part wins any difference, and this file follows it. The desktop source
files named above are planned by those tasks and do not exist on `main` yet.

#### Interfaces — complete: `work.ts`, part 13: requests, rules and forms

```ts
// desktop/src/api/work.ts (appended) — SW.21's desktop half. Every field is task 143's JSON tag.

export type RequesterKind = "person" | "email" | "teams";
/** Who asked. userId is set for a person known to Entra; email for mail intake; displayName always. */
export interface Requester { kind: RequesterKind; userId: string | null; email: string | null; displayName: string }

export type RequestChannel = "form" | "email" | "teams" | "tracker";
export type RequestState = "open" | "accepted" | "linked" | "declined";
export interface FormAnswer { fieldId: string; value: string }
/** Task 42's CriterionRef shape: which criterion of which brief. A suggestion, never a score. */
export interface SuggestedCriterion { briefKey: string; criterionId: string }

export interface Request {
  id: string;                  // rq-…
  teamId: string;
  projectId: string | null;
  requester: Requester;
  quote: AuthoredText;         // task 37's {text, author}; author is always "H"; the words verbatim
  channel: RequestChannel;
  channelRef: string;          // the form id, the mail message id, the Teams message ref
  important: boolean;
  answers: FormAnswer[];
  state: RequestState;
  issueId: string | null;      // is-…, set by accept or link
  suggestedCriterion: SuggestedCriterion | null;
  ruleId: string | null;       // ir-… of the rule that routed it; null when none matched
  createdAt: string;
}

/** Append-only: task 157's Teams replies. body is text·H, the external author's words. */
export interface RequestMessage {
  requestId: string; channelRef: string; authorName: string;
  userId: string | null; body: string; at: string;
}

export interface RuleMatch {
  channel: RequestChannel | null; formId: string | null;
  senderDomain: string | null; textContains: string | null;
}
export interface RuleRoute {
  projectId: string; labels: string[]; priority: Priority | null; assigneeId: string | null;
}
/** position orders the list; the lowest matching position wins (task 147). */
export interface IntakeRule {
  id: string;                  // ir-…
  teamId: string; position: number;
  match: RuleMatch; route: RuleRoute;
  author: User;                // the person who last wrote it; the event names them
  version: number;
}

export const FORM_FIELD_KINDS = ["text", "longtext", "select", "date", "number", "person"] as const;
export type FormFieldKind = (typeof FORM_FIELD_KINDS)[number];
export interface FormField {
  id: string; label: string; kind: FormFieldKind; required: boolean;
  options: string[];           // [] unless kind is "select"
}
export interface FormDefault { labels: string[]; priority: Priority | null; assigneeId: string | null }
export interface IntakeForm {
  id: string;                  // fm-…
  teamId: string; name: string;
  fields: FormField[];
  defaults: { member: FormDefault; nonMember: FormDefault };
}
/** The team's mail intake address (task 149). */
export interface IntakeAddress { teamId: string; address: string; formId: string | null }
```

`Priority` is task 37's (the four words and `none`); `User` and `AuthoredText` are task 37's. Task
143 writes the optional Go fields as `projectId?`, `issueId?` and so on; this file follows task
125's convention and types them `T | null`, because the Go tags carry no `omitempty` in task 114's
goldens and the golden test fails if task 143 chooses otherwise, which is the signal to switch the
field to `?:`.

#### Interfaces — complete: `work.ts`, part 14: trackers and loops

```ts
/** SW.6's ImportSource gains these three in task 143 (task 15 fixed "linear" and "csv"). */
export type TrackerSource = "azure_boards" | "github" | "jira";
/** The external project in the tracker's own terms (task 171 validates each before the call). */
export type TrackerTarget =
  | { source: "azure_boards"; organisation: string; project: string; areaPath: string | null }
  | { source: "github"; owner: string; repository: string }
  | { source: "jira"; site: string; projectKey: string };
/** "in" = one way into Triage; "in_and_comments" adds comments both ways. Never a state mode. */
export type TrackerMode = "in" | "in_and_comments";
export type TrackerState = "active" | "disconnected";
export interface TrackerConnection {
  id: string;                  // tc-…
  teamId: string; projectId: string;
  source: TrackerSource; target: TrackerTarget; mode: TrackerMode;
  principalId: string;         // the integration principal (task 150); never a token
  state: TrackerState;
  lastRunAt: string | null;
  createdBy: User;
}
/** An issue imported from outside: *not tied · imported* until a person ties it (task 42). */
/** Task 150's UpstreamChange: an outside close, reassignment or edit, shown and never applied. */
export interface UpstreamChange { externalId: string; field: string; at: string }
export interface ExternalLink {
  id: string;                  // xl-…
  source: TrackerSource; externalId: string; issueId: string; url: string; lastSeenAt: string;
  upstream: UpstreamChange | null;   // the latest, from task 150's upstreamChangedAt
}
/** code is a Hub code string (hub-contract §4 or task 143's), not the desktop's ProblemCode:
 *  it names why one external item was refused, and it is never thrown. */
export interface SyncError { externalId: string; code: string }
/** One engine pass (task 150): counts are measured facts of that pass, not a total to reach. */
export interface SyncRun {
  connectionId: string; at: string;
  created: number; existing: number; commentsIn: number; commentsOut: number;
  errors: SyncError[];
}

/** AX §7's rows and columns (`docs/AGENTIC-UX.md:179`), as wire words. */
export const ACTION_CLASSES = ["read", "edit_own_worktree", "edit_outside_worktree", "run_tests",
  "install_dependency", "run_migration", "touch_infra", "commit", "push", "deploy"] as const;
export type ActionClass = (typeof ACTION_CLASSES)[number];
export const AUTONOMY_LEVELS = ["ask_first", "propose_and_wait", "act_and_tell", "full"] as const;
export type AutonomyLevel = (typeof AUTONOMY_LEVELS)[number];
/** Task 162's LoopBound: turnsPerDay 1-24; a class absent from levels inherits the owner's own
 *  grid (shown greyed by task 172). */
export interface LoopBound { turnsPerDay: number; levels: Partial<Record<ActionClass, AutonomyLevel>> }

export type ScheduleEvery = "day" | "weekday" | "week";
export interface LoopTrigger {
  kind: "event" | "schedule";
  eventKinds: string[];        // SW.5 and SW.21 feed kinds only; [] for a schedule
  filter: { labelId: string | null; fromState: IssueState | null; toState: IssueState | null };
  schedule: { every: ScheduleEvery; at: string; tz: string } | null;   // HH:MM, IANA; null for an event
}
export type LoopState = "on" | "paused";
/** The loop's head (task 162). Its text lives in its versions. */
export interface Loop {
  id: string;                  // lp-…
  teamId: string; projectId: string;
  owner: User;
  name: string;                // 1-80 characters
  state: LoopState;
  pausedCause: string | null;  // "owner_left" is task 162's; others the part names
  currentVersion: number;
}
export type LoopAction = "turn" | "start";   // "start": Start on the accepted issue (task 163)
/** Append-only: an edit is version n+1 (task 162). instruction is text·H of the owner, ≤ 2000. */
export interface LoopVersion {
  loopId: string; version: number;
  trigger: LoopTrigger; instruction: AuthoredText;
  action: LoopAction; labelId: string | null;
  bound: LoopBound;
  savedAt: string; savedBy: User;
}

export type LoopOutcome = "queued" | "ran" | "skipped_budget" | "skipped_paused" | "refused_bound" | "failed";
/** SW.10's Start result (task 24), as task 163's `started` carries it. */
export interface StartedLane { issueKey: string; sessionId: string; branch: string; route: string }
/** Keyed by (loopId, run): no id prefix (task 163). */
export interface LoopRun {
  loopId: string; run: number; version: number; firedAt: string;
  trigger: { kind: "event" | "schedule" | "manual"; eventSeq: string | null; scheduledFor: string | null };
  outcome: LoopOutcome;
  turnId: string | null;       // ot-… (Phase T's Orchestrator turn)
  started: StartedLane[];      // [] unless the action was a Start
  endedAt: string | null;
}
```

The loop types follow **tasks 162 and 163's scopes** (`tasks-161-180.md:96-103`, `:139-143`),
which own the routes and name every field task 172 draws. Task 143's scope names a flatter `Loop`
(trigger, instruction and `grid` on the head, one level name) and an `lr-` prefix for `LoopRun`;
task 163 says *no new id prefix*. Task 162 says *"143 wins on names"*, but 143's part does not
exist and its scope carries less, so this part follows 162-163 and records the conflict as G1;
the golden test decides once task 143's part lands. `IssueState` is task 37's.

#### Interfaces — complete: `work.ts`, part 15: the admin records (never in the replica)

```ts
export type AppScope = "read" | "write";
export interface AppManifest { scopes: AppScope[]; redirectUris: string[]; webhookUrl: string | null }
export type AppState = "pending" | "approved" | "revoked";
/** An Entra OAuth app that acts as a person (task 159). actsAs is the consenting person. */
export interface App {
  id: string;                  // oa-…
  name: string; entraClientId: string; manifest: AppManifest;
  actsAs: User | null;
  state: AppState; decidedBy: User | null; decidedAt: string | null;
}

export type KeyScope = "personal" | "team";
/** No field holds the secret (task 143's reflection test; decision 5). shown = first 8 chars. */
export interface APIKey {
  id: string;                  // ak-…
  scope: KeyScope; ownerId: string; teamId: string | null;
  name: string; shown: string;
  createdAt: string; expiresAt: string;   // required, at most 365 days on
  lastUsedAt: string | null; revokedAt: string | null; revokedBy: User | null;
}
/** Returned once by createAPIKey. Held by task 173's dialog in component state, nowhere else. */
export interface CreatedAPIKey { key: APIKey; secret: string }
/** Task 160's per-token headers, read as facts: used this hour, and the limit. */
export interface RateReading { used: number; limit: number; resetsAt: string }

export type WebhookState = "active" | "disabled";
export interface Webhook {
  id: string;                  // wh-…
  teamId: string; projectId: string | null;
  url: string;                 // https only (task 143's Validate)
  kinds: string[];             // SW.5 kinds only
  state: WebhookState; createdBy: User; secretVersion: number;
}
export interface CreatedWebhook { webhook: Webhook; secret: string }
export type DeliveryStatus = "ok" | "failed" | "pending";
export interface WebhookDelivery {
  id: string;                  // wd-…
  webhookId: string; eventSeq: string; attempt: number;
  status: DeliveryStatus; responseCode: number | null;
  at: string; nextAttemptAt: string | null;   // a scheduled retry time, not a forecast
}

export interface MCPConnector {
  id: string;                  // mc-…
  name: string; url: string; allowed: boolean; decidedBy: User; at: string;
}

export interface HarnessPin { name: string; versionPin: string | null }
/** Order is the team's preference; "claude-code" first by default (task 166). */
export interface HarnessAllow { teamId: string; harnesses: HarnessPin[] }
/** source "library" pins ref to a 40-hex commit; "image" names the sandbox image digest. */
export interface TeamSkill { teamId: string; name: string; source: "image" | "library"; ref: string }
/** TM-14's policy route answer when task 173 publishes (UNVERIFIED until Phase T merges). */
export interface PublishedPolicy { version: number; digest: string }
```

`App.actsAs` is not in task 143's scope; task 173 needs *"the person it acts as"* on each manifest
(scope 173), so it is proposed to task 143 with G1. `RateReading` is not a wire record: task 160
sends it as response headers, and `rateOf(res)` below reads them.

#### Interfaces — complete: `work.ts`, part 16: request bodies

```ts
/** serves rides on the create so accept stays one op (task 168's end to end accepts into an
 *  issue that serves a criterion); it is SW.1's {briefKey, criterionId}, [] for none. */
export interface AcceptRequestBody {
  title: string; description: AuthoredText; projectId: string;
  serves: { briefKey: string; criterionId: string }[];
}
export interface LinkRequestBody { issueId: string }
export interface PutIntakeRulesBody { rules: Omit<IntakeRule, "id" | "author" | "version">[]; version: number }
export interface PutFormBody { name: string; fields: FormField[]; defaults: IntakeForm["defaults"] }
export interface ConnectTrackerBody { target: TrackerTarget; mode: TrackerMode }   // projectId is the path's
/** One version's content (task 162's POST …/versions); the Hub stamps loopId, version, savedAt/By. */
export type LoopVersionBody = Omit<LoopVersion, "loopId" | "version" | "savedAt" | "savedBy">;
export interface CreateLoopBody { name: string; first: LoopVersionBody }   // the caller becomes owner
export interface CreateAPIKeyBody { name: string; scope: KeyScope; teamId: string | null; expiresAt: string }
export interface CreateWebhookBody { projectId: string | null; url: string; kinds: string[] }
export interface DecideBody { decision: "approve" | "revoke" }
export interface PutTeamSkillBody { name: string; source: TeamSkill["source"]; ref: string }
```

Every create and every *run now* goes through task 56's `keyed(op.id)` client, which sends
`Idempotency-Key` on each attempt (`task-56.md:89-98`), so a double press is one op and one run.
The one body that also names the key is *run now*, because task 163's route takes `{requestId}`
and answers a repeat with the first run: the desktop sends the same op id in both places. `AcceptRequestBody.description` is what the **person** typed,
and an empty one is legal (task 170). The quote is never a field of it.

#### Interfaces — complete: `work.ts`, part 17: the relay table

```ts
/** Task 168's W5 relay rows, only the routes a desktop surface calls (170-173), in its order.
 *  Paths are task 168's, which has no part yet: every path is UNVERIFIED, and the route test
 *  below reads task 168's Go table by path and fails on any difference. */
export const WORK_RELAY_W5 = [
  ["ListRequests", "GET", "/v1/teams/{teamId}/requests"],
  ["RequestMessages", "GET", "/v1/requests/{requestId}/messages"],
  ["AcceptRequest", "POST", "/v1/requests/{requestId}/accept"],
  ["LinkRequest", "POST", "/v1/requests/{requestId}/link"],
  ["DeclineRequest", "POST", "/v1/requests/{requestId}/decline"],
  ["MarkImportant", "PUT", "/v1/requests/{requestId}/important"],
  ["ListIntakeRules", "GET", "/v1/teams/{teamId}/intake-rules"],
  ["PutIntakeRules", "PUT", "/v1/teams/{teamId}/intake-rules"],
  ["GetForm", "GET", "/v1/forms/{formId}"],
  ["PutForm", "PUT", "/v1/teams/{teamId}/forms/{formId}"],
  ["ConnectTracker", "POST", "/v1/projects/{projectId}/trackers"],   // task 150's scope, :441
  ["ListTrackers", "GET", "/v1/teams/{teamId}/trackers"],
  ["SetTrackerMode", "PUT", "/v1/trackers/{connectionId}/mode"],
  ["DisconnectTracker", "DELETE", "/v1/trackers/{connectionId}"],
  ["ListSyncRuns", "GET", "/v1/trackers/{connectionId}/runs"],
  ["ListTrackerLinks", "GET", "/v1/trackers/{connectionId}/links"],
  // loops: task 162's and 163's paths (tasks-161-180.md:104-108, :144-148)
  ["CreateLoop", "POST", "/v1/projects/{projectId}/work/loops"],
  ["ProjectLoops", "GET", "/v1/projects/{projectId}/work/loops"],
  ["GetLoop", "GET", "/v1/work/loops/{loopId}"],
  ["SetLoopState", "PATCH", "/v1/work/loops/{loopId}"],
  ["PutLoopVersion", "POST", "/v1/work/loops/{loopId}/versions"],
  ["ListLoopVersions", "GET", "/v1/work/loops/{loopId}/versions"],
  ["ListLoopRuns", "GET", "/v1/work/loops/{loopId}/runs"],
  ["RunLoopNow", "POST", "/v1/work/loops/{loopId}/runs"],
  ["ListApps", "GET", "/v1/apps"],
  ["DecideApp", "PUT", "/v1/apps/{appId}/decision"],
  ["CreateAPIKey", "POST", "/v1/keys"],
  ["ListAPIKeys", "GET", "/v1/keys"],
  ["RevokeAPIKey", "DELETE", "/v1/keys/{keyId}"],
  ["CreateWebhook", "POST", "/v1/teams/{teamId}/webhooks"],
  ["ListWebhooks", "GET", "/v1/teams/{teamId}/webhooks"],
  ["DisableWebhook", "DELETE", "/v1/webhooks/{webhookId}"],
  ["ListDeliveries", "GET", "/v1/webhooks/{webhookId}/deliveries"],
  ["Redeliver", "POST", "/v1/deliveries/{deliveryId}/redeliver"],
  ["ListMCPConnectors", "GET", "/v1/mcp-connectors"],
  ["DecideMCPConnector", "PUT", "/v1/mcp-connectors/{connectorId}"],
  ["GetHarnessAllow", "GET", "/v1/teams/{teamId}/harness-allow"],
  ["SetHarnessAllow", "PUT", "/v1/teams/{teamId}/harness-allow"],
  ["ListTeamSkills", "GET", "/v1/teams/{teamId}/skills"],
  ["PutTeamSkill", "PUT", "/v1/teams/{teamId}/skills/{name}"],
] as const satisfies ReadonlyArray<readonly [string, Verb, `/v1/${string}`]>;
export type RelayMethodW5 = (typeof WORK_RELAY_W5)[number][0];

/** Hub routes a desktop surface must NEVER reach through the relay (task 168's 404 rows). */
export const W5_NOT_RELAYED = ["/v1/hooks/", "/v1/teams-bot/", "/v1/forms/{formId}/submit", "/mcp"] as const;
```

Rows naming a method task 143's list lacks: `RequestMessages` (task 157's thread),
`SetTrackerMode` (task 153's per-connection switch), `ListTrackerLinks` (task 171's imported
rows and upstream changes; task 150 names the `ExternalLink` record and no list), `CreateLoop`,
`ProjectLoops`, `GetLoop` and `SetLoopState` (task 162's routes), `ListApps` and
`ListTeamSkills`. They are proposed to task 143 with G1. `SetLoopState` is a separate row from
`PutLoopVersion` because a team admin may pause a teammate's loop but never write a version of it
(OD-16, task 162). `ListLoopVersions` is task 143's method with no route in task 162's scope, so
its path is this part's. Task 162's paths start `/v1/work/…`, so `relayPath` makes them
`/v1/hub/work/work/loops/…`: mechanical and correct by task 37's rule, if unlovely (gap G7).
*Publish to sandboxes* is **not** a row here: it posts TM-14's policy route, which is Phase T's
relay, not the Work relay, and task 173 reaches it only once Phase T merges.

#### Interfaces — complete: `work.ts`, part 18: the `WorkIntakeClient` members

```ts
/** SW.21 on the desktop. WorkClient (task 37) extends it beside WorkRoadmapClient (task 125). */
export interface WorkIntakeClient {
  requests(teamId: string, q?: { state?: RequestState } & PageRequest): Promise<Page<Request>>;
  requestMessages(requestId: string): Promise<RequestMessage[]>;
  /** One op: the issue and the link together. The answer carries both. */
  acceptRequest(requestId: string, body: AcceptRequestBody): Promise<{ request: Request; issue: IssueWithRevision }>;
  linkRequest(requestId: string, body: LinkRequestBody): Promise<Request>;
  declineRequest(requestId: string): Promise<Request>;
  markImportant(requestId: string, important: boolean): Promise<Request>;
  intakeRules(teamId: string): Promise<{ rules: IntakeRule[]; version: number }>;
  putIntakeRules(teamId: string, body: PutIntakeRulesBody): Promise<{ rules: IntakeRule[]; version: number }>;
  form(formId: string): Promise<IntakeForm & { url: string }>;   // url: the Hub form's https link
  putForm(teamId: string, formId: string, body: PutFormBody): Promise<IntakeForm & { url: string }>;
  connectTracker(projectId: string, body: ConnectTrackerBody): Promise<{ connection: TrackerConnection; consentUrl: string | null }>;
  trackerLinks(connectionId: string, page?: PageRequest): Promise<Page<ExternalLink>>;
  trackers(teamId: string): Promise<TrackerConnection[]>;
  setTrackerMode(connectionId: string, mode: TrackerMode): Promise<TrackerConnection>;
  disconnectTracker(connectionId: string): Promise<TrackerConnection>;
  syncRuns(connectionId: string, page?: PageRequest): Promise<Page<SyncRun>>;
  createLoop(projectId: string, body: CreateLoopBody): Promise<{ loop: Loop; current: LoopVersion }>;
  projectLoops(projectId: string): Promise<Loop[]>;          // every loop the caller may read
  loop(loopId: string): Promise<{ loop: Loop; current: LoopVersion }>;
  setLoopState(loopId: string, state: LoopState): Promise<Loop>;   // owner; an admin may pause
  putLoopVersion(loopId: string, body: LoopVersionBody): Promise<LoopVersion>;   // owner only, n+1
  loopVersions(loopId: string): Promise<LoopVersion[]>;      // newest first
  loopRuns(loopId: string, page?: PageRequest): Promise<Page<LoopRun>>;
  /** Task 163's body is {requestId}; the desktop sends the op id as it and as Idempotency-Key. */
  runLoopNow(loopId: string, requestId: string): Promise<LoopRun>;   // outcome "queued"
  apps(): Promise<App[]>;
  decideApp(appId: string, body: DecideBody): Promise<App>;
  createAPIKey(body: CreateAPIKeyBody): Promise<CreatedAPIKey>;
  apiKeys(q?: { teamId?: string }): Promise<{ keys: APIKey[]; rate: RateReading | null }>;
  revokeAPIKey(keyId: string): Promise<APIKey>;
  createWebhook(teamId: string, body: CreateWebhookBody): Promise<CreatedWebhook>;
  webhooks(teamId: string): Promise<Webhook[]>;
  disableWebhook(webhookId: string): Promise<Webhook>;
  deliveries(webhookId: string, page?: PageRequest): Promise<Page<WebhookDelivery>>;
  redeliver(deliveryId: string): Promise<WebhookDelivery>;
  mcpConnectors(): Promise<MCPConnector[]>;
  decideMCPConnector(connectorId: string, allowed: boolean): Promise<MCPConnector>;
  harnessAllow(teamId: string): Promise<HarnessAllow>;
  setHarnessAllow(teamId: string, a: HarnessAllow): Promise<HarnessAllow>;
  teamSkills(teamId: string): Promise<TeamSkill[]>;
  putTeamSkill(teamId: string, body: PutTeamSkillBody): Promise<TeamSkill>;
}
// task 37's declaration becomes: export interface WorkClient extends WorkRoadmapClient, WorkIntakeClient { … }
```

`makeWorkClient` gains a `w5` helper exactly like task 125's `w4` (a row looked up by name, so a
renamed row fails the route test), and one line per member over it. `apiKeys` is the one member
that reads headers: task 37's `send` resolves to the body, so `w5` takes a `{ headers: true }`
flag for that row only and `rateOf(h)` parses `X-RateLimit-Used`, `X-RateLimit-Limit` and
`X-RateLimit-Reset` (task 160's names, **UNVERIFIED**), answering `null` when any is absent,
never a guessed limit. `acceptRequest` answers task 37's `IssueWithRevision` (`task-37.md:320`),
so task 168's digest re-check on the new revision runs in the relay before the desktop sees it.

#### Interfaces — complete: `work.ts`, part 19: `WIRE_KEYS` for W5, the goldens, the codes

```ts
// appended inside task 37's WIRE_KEYS object; keys<T>() holds each list to its type both ways
  request: keys<Request>({ id: true, teamId: true, projectId: true, requester: true, quote: true,
    channel: true, channelRef: true, important: true, answers: true, state: true, issueId: true,
    suggestedCriterion: true, ruleId: true, createdAt: true }),
  requestMessage: keys<RequestMessage>({ requestId: true, channelRef: true, authorName: true, userId: true, body: true, at: true }),
  intakeRule: keys<IntakeRule>({ id: true, teamId: true, position: true, match: true, route: true, author: true, version: true }),
  intakeForm: keys<IntakeForm>({ id: true, teamId: true, name: true, fields: true, defaults: true }),
  formField: keys<FormField>({ id: true, label: true, kind: true, required: true, options: true }),
  trackerConnection: keys<TrackerConnection>({ id: true, teamId: true, projectId: true, source: true,
    target: true, mode: true, principalId: true, state: true, lastRunAt: true, createdBy: true }),
  externalLink: keys<ExternalLink>({ id: true, source: true, externalId: true, issueId: true, url: true,
    lastSeenAt: true, upstream: true }),
  syncRun: keys<SyncRun>({ connectionId: true, at: true, created: true, existing: true, commentsIn: true, commentsOut: true, errors: true }),
  loop: keys<Loop>({ id: true, teamId: true, projectId: true, owner: true, name: true, state: true,
    pausedCause: true, currentVersion: true }),
  loopVersion: keys<LoopVersion>({ loopId: true, version: true, trigger: true, instruction: true,
    action: true, labelId: true, bound: true, savedAt: true, savedBy: true }),
  loopRun: keys<LoopRun>({ loopId: true, run: true, version: true, firedAt: true, trigger: true,
    outcome: true, turnId: true, started: true, endedAt: true }),
  app: keys<App>({ id: true, name: true, entraClientId: true, manifest: true, actsAs: true, state: true, decidedBy: true, decidedAt: true }),
  apiKey: keys<APIKey>({ id: true, scope: true, ownerId: true, teamId: true, name: true, shown: true,
    createdAt: true, expiresAt: true, lastUsedAt: true, revokedAt: true, revokedBy: true }),
  createdApiKey: keys<CreatedAPIKey>({ key: true, secret: true }),
  webhook: keys<Webhook>({ id: true, teamId: true, projectId: true, url: true, kinds: true, state: true, createdBy: true, secretVersion: true }),
  createdWebhook: keys<CreatedWebhook>({ webhook: true, secret: true }),
  webhookDelivery: keys<WebhookDelivery>({ id: true, webhookId: true, eventSeq: true, attempt: true,
    status: true, responseCode: true, at: true, nextAttemptAt: true }),
  mcpConnector: keys<MCPConnector>({ id: true, name: true, url: true, allowed: true, decidedBy: true, at: true }),
  harnessAllow: keys<HarnessAllow>({ teamId: true, harnesses: true }),
  teamSkill: keys<TeamSkill>({ teamId: true, name: true, source: true, ref: true }),

/** Task 143's goldens, file → the WIRE_KEYS entry it is held to. The file names are this part's
 *  (task 143 has no part); the test fails on a missing file, so a rename there shows here. */
export const W5_GOLDENS = {
  "request": "request", "request-erased": "request", "request-message": "requestMessage",
  "intake-rule": "intakeRule", "intake-form": "intakeForm",
  "tracker-connection": "trackerConnection", "external-link": "externalLink", "sync-run": "syncRun",
  "loop": "loop", "loop-version": "loopVersion", "loop-run": "loopRun", "app": "app", "api-key": "apiKey",
  "created-api-key": "createdApiKey", "webhook": "webhook", "created-webhook": "createdWebhook",
  "webhook-delivery": "webhookDelivery", "mcp-connector": "mcpConnector",
  "harness-allow": "harnessAllow", "team-skill": "teamSkill",
} as const satisfies Record<string, keyof typeof WIRE_KEYS>;
```

```ts
// desktop/src/api/client.ts ⚑ — ProblemCode gains task 143's ten appended codes (its scope's list)
// and the two tasks 162-163 name (tasks-161-180.md:109, :150)
  /* the W5 Work wire (tasks 143, 162, 163, 168) */
  | "request_closed" | "form_field_required" | "tracker_exists" | "principal_cannot_move_state"
  | "webhook_url_invalid" | "app_not_approved" | "key_revoked" | "connector_not_allowed"
  | "harness_not_allowed" | "loop_outside_grid" | "loop_bound_exceeds_owner" | "loop_paused"
```

`workProblem` files all twelve as `refused` with `details` intact, so task 170 can name the
missing field (`form_field_required`), task 172 the level (`loop_bound_exceeds_owner`, or task
143's `loop_outside_grid` for a run: two names for near kin, gap G1) and task 171 the refused
move (`principal_cannot_move_state`). `FormField` is checked inside `intake-form.json`
(`fields[i]`), as task 125 checks `InitiativeFeature`.

#### Interfaces — complete: the delta on ⚑ `desktop/src/work/replica/kinds.ts` (task 125)

```ts
// appended — the W5 feed kinds (SW.5, SW.21). COPIED BY NAME from task 144's
// fake_work_intake.go; kinds.test.ts reads that file and fails on any difference.

/** Kinds whose subject is a Work record the replica holds. */
export const W5_FEED_KINDS = [
  "request.created", "request.accepted", "request.linked", "request.declined",
  "request.important_set", "request.erased", "request.message_added",
  "intake_rules.set", "intake_form.set",
  "tracker.connected", "tracker.mode_set", "tracker.disconnected", "sync.run",
  "external_link.created",
  "loop.created", "loop.version_saved", "loop.state_set",
  "work.loop_fired",   // task 162's name (tasks-161-180.md:112): it opens a run, queued (task 163)
  "loop.run_ended",
] as const;
export type W5FeedKind = (typeof W5_FEED_KINDS)[number];

/** Admin kinds: the replica reads these only to skip them (decision 4). */
export const W5_ADMIN_KINDS = [
  "app.registered", "app.decided", "api_key.created", "api_key.revoked",
  "webhook.created", "webhook.disabled", "webhook.delivered", "mcp_connector.decided",
  "harness_allow.set", "team_skill.put",
] as const;

/** Signals the replica passes on and does not store: the Desk row of task 170. */
export const W5_SIGNAL_KINDS: ReadonlySet<string> = new Set(["request.unrouted"]);

export const W5_SUBJECT_PREFIX = { "rq-": "request", "ir-": "intakeRule", "fm-": "intakeForm",
  "tc-": "tracker", "xl-": "externalLink", "lp-": "loop" } as const;
// A run has no prefix (task 163): work.loop_fired and loop.run_ended name {loopId, run} in data.

export const isW5Kind = (k: string): k is W5FeedKind => (W5_FEED_KINDS as readonly string[]).includes(k);
export const isW5Admin = (k: string): boolean => (W5_ADMIN_KINDS as readonly string[]).includes(k);
```

Every kind above is this part's word, because task 144 has no part: **all of them are
UNVERIFIED** until it lands, and the copy test (fake kinds = `W5_FEED_KINDS` ∪ `W5_ADMIN_KINDS` ∪
`W5_SIGNAL_KINDS`, exactly) is what makes them true. `request.unrouted` is 05 §4.2's FAILED row
(*"an intake rule could not route a request"*), named by task 170's scope. A kind the Hub appends
that none of the three lists holds is **ignored**, as task 125's rule 2.

#### Interfaces — complete: the deltas on ⚑ `replica.ts` (task 55), ⚑ `ops.ts` (task 56), ⚑ `visibility.ts` (task 57)

```ts
// desktop/src/work/replica/replica.ts — task 169 adds these record maps beside task 125's.
import type { ExternalLink, IntakeForm, IntakeRule, Loop, LoopRun, Request, RequestMessage,
  SyncRun, TrackerConnection } from "../../api/work";

/** What the replica holds for W5. Keys are Hub ids; every record passes task 57's admit(). */
export interface IntakeRecords {
  requests: ReadonlyMap<string, Request>;                  // rq-…, only of the caller's teams
  requestThreads: ReadonlyMap<string, RequestMessage[]>;   // by requestId, append-only, by `at`
  rules: ReadonlyMap<string, IntakeRule[]>;                // by teamId, by position
  forms: ReadonlyMap<string, IntakeForm>;                  // fm-…
  trackers: ReadonlyMap<string, TrackerConnection>;        // tc-…
  links: ReadonlyMap<string, ExternalLink>;                // by issueId: at most one per issue
  syncRuns: ReadonlyMap<string, SyncRun[]>;                // by connectionId, newest first, last 50
  loops: ReadonlyMap<string, Loop>;                        // lp-…, heads of the caller's projects
  loopRuns: ReadonlyMap<string, LoopRun[]>;                // by loopId, newest first, last 50
  // A loop's versions are NOT held: task 172 fetches them on open (loopVersions, loop).
}
// Replica (task 55) gains:
//   intake(): IntakeRecords
//   requestsOf(issueId: string): Request[]      // accepted or linked to it: the issue's requesters
//   teamHasRequests(teamId: string): boolean    // task 170's "only once the team has a request"
```

```ts
// desktop/src/work/replica/ops.ts — task 56's Op union gains these; each has an inverse for U.
export type IntakeOp =
  | { kind: "request.accept"; requestId: string; body: AcceptRequestBody }   // one op, issue + link
  | { kind: "request.link"; requestId: string; issueId: string }
  | { kind: "request.decline"; requestId: string }
  | { kind: "request.important"; requestId: string; important: boolean; prev: boolean }
  | { kind: "rules.put"; teamId: string; body: PutIntakeRulesBody; prev: PutIntakeRulesBody }
  | { kind: "form.put"; teamId: string; formId: string; body: PutFormBody; prev: PutFormBody }
  | { kind: "tracker.mode"; connectionId: string; mode: TrackerMode; prev: TrackerMode }
  | { kind: "loop.create"; projectId: string; body: CreateLoopBody }
  | { kind: "loop.version"; loopId: string; body: LoopVersionBody; prev: LoopVersionBody }   // n+1
  | { kind: "loop.state"; loopId: string; state: LoopState; prev: LoopState }
  | { kind: "loop.runNow"; loopId: string };                                // no inverse
// Deliberately absent: any op for keys, apps, webhooks, connectors, harness or skills (decision
// 4: task 173 calls WorkClient directly and holds the answer in component state), and any
// "loop.patch" (decision 6). intake.test.ts asserts both.
```

**Inverses** (task 56's `inverseOf(op, before)`): `request.important` inverts to the `prev` value;
`rules.put`, `form.put`, `tracker.mode` and `loop.state` to a put of `prev`; `loop.version` to a
`loop.version` of `prev`, the version read before (a **new** version equal to the old one, so the history
keeps both, decision 6). `request.accept` and `request.link` invert to a Hub *unlink* only if task
146 has one; its scope names none, so `U` is refused with the fact line *"Accepting cannot be
undone here; decline the issue from its page"* (gap G3). `request.decline` has no reopen in task
143's list; `U` is refused the same way. `loop.runNow` has no inverse: a queued run is a fact.

**How W5 enters the replica**, in order, beside task 55's and task 125's paths:

1. **Load.** For each team the caller is a member of: `requests(teamId)` (all pages; open and the
   last 90 days of closed), `intakeRules`, `trackers`; for each tracker, the first page of
   `syncRuns`; then, per project, `projectLoops` and, for each, the first page of `loopRuns`. A request's thread
   (`requestMessages`) loads on first open by task 170, not at start. `ExternalLink`s arrive with
   the issues task 55 loads, where task 150 embeds them (**UNVERIFIED**: task 150 names the link
   record, not how it rides; gap G4).
2. **Each event** whose kind `isW5Kind` re-fetches its subject by the prefix map and puts it in one
   `transact`. `request.accepted` and `request.linked` apply **in feed order** (by `seq`), so a
   link after an accept adds a second issue's requester and never replaces the first. A kind that
   `isW5Admin` is skipped with no fetch and no store. A signal kind goes to `signals` listeners.
3. **`request.erased`** runs task 57's erase pattern: `quote.text` becomes `""`, the requester's
   `displayName` and `email` become `""`, and id, state and `issueId` stay.

```ts
// desktop/src/work/replica/visibility.ts — task 57's admit() gains the request team rule
// (decision 8). callerTeams is task 57's set of the caller's member teams.
export function admitRequest(r: Pick<Request, "teamId">, callerTeams: ReadonlySet<string>): boolean {
  // A grant (task 117) lets another team read a project's issues; it never lets them read the
  // quote of a request routed to that project. Only the request's own team sees it.
  return callerTeams.has(r.teamId);
}
/** Runs before task 57's classify for request.* kinds: another team's request is "none". */
export function classifyRequest(e: FeedEventLike & { data?: Record<string, unknown> },
                                callerTeams: ReadonlySet<string>): VisibilityAction | null {
  if (!e.kind.startsWith("request.")) return null;
  const teamId = typeof e.data?.teamId === "string" ? e.data.teamId : null;
  return teamId && callerTeams.has(teamId) ? null : { kind: "none" };
}
```

`classifyRequest` answering `null` hands the event on to the normal fetch path; `{ kind: "none" }`
drops it before any fetch, so another team's quote never crosses the loopback. `data.teamId` on a
`request.*` event is task 144's metadata (SW.5 carries ids, never text), **UNVERIFIED**.

#### Tests to write first — appended to ⚑ `desktop/src/api/work.test.ts`

```ts
// Task 143's W5 goldens, read where the Hub keeps them; the count stops an empty glob passing.
const W5 = import.meta.glob<string>(
  "../../../control-plane/internal/hub/hubapi/testdata/work-intake/*.json",
  { query: "?raw", import: "default", eager: true },
);
const HUB_TABLE_W5 = import.meta.glob<string>(
  "../../../control-plane/internal/api/work.go",
  { query: "?raw", import: "default", eager: true },
);

describe("SW.21 the W5 wire", () => {
  // Kills: a renamed field (quoteText for quote) that tsc alone misses, on either side.
  it("each W5 golden's keys are its desktop type's keys", () => {
    expect(Object.keys(W5)).toHaveLength(Object.keys(W5_GOLDENS).length);
    for (const [file, type] of Object.entries(W5_GOLDENS)) {
      const path = Object.keys(W5).find((p) => p.endsWith(`/${file}.json`));
      expect(path, file).toBeDefined();
      const v = JSON.parse(W5[path!]) as Record<string, unknown>;
      expect(Object.keys(v).sort(), file).toEqual(WIRE_KEYS[type]);
      if (file === "intake-form")
        for (const f of v.fields as Record<string, unknown>[])
          expect(Object.keys(f).sort(), "fields[i]").toEqual(WIRE_KEYS.formField);
    }
  });

  // Kills: the secret listed again on an ordinary record.
  it("only the three Created* types carry a secret key", () => {
    for (const [name, list] of Object.entries(WIRE_KEYS))
      if (!name.startsWith("created")) expect(list, name).not.toContain("secret");
  });

  // Kills: a W5 relay row that drifts from task 168's Go rows.
  it("WORK_RELAY_W5 is task 168's W5 rows, row for row", () => {
    const go = Object.values(HUB_TABLE_W5)[0] ?? "";
    const w5 = go.slice(go.indexOf("// W5"));
    const rows = [...w5.matchAll(/\{"(\w+)", http\.Method(\w+), "([^"]+)", (?:true|false)\}/g)]
      .map(([, m, verb, path]) => [m, verb.toUpperCase(), path]);
    expect(rows).toEqual(WORK_RELAY_W5.map((r) => [...r]));
  });

  // Kills: Hub-facing ingress exposed on loopback (task 168's 404 rows).
  it("no W5 relay row reaches host hooks, the Teams bot, the public submit or /mcp", () => {
    for (const [, , path] of WORK_RELAY_W5)
      for (const bad of W5_NOT_RELAYED) expect(path.startsWith(bad.replace("{formId}", "")), path).toBe(false);
  });

  // Kills: a double press on run now as two runs.
  it("runLoopNow through keyed() sends one key, as header and requestId, on every attempt", async () => {
    const send = vi.fn(async () => ({ loopId: "lp-1", run: 4, outcome: "queued" }));
    const c = makeWorkClient(send).keyed("op-7");
    await c.runLoopNow("lp-1", "op-7"); await c.runLoopNow("lp-1", "op-7");
    expect(send.mock.calls.map((a) => a[3])).toEqual([{ idempotencyKey: "op-7" }, { idempotencyKey: "op-7" }]);
    expect(send).toHaveBeenCalledWith("POST", "/v1/hub/work/work/loops/lp-1/runs", { requestId: "op-7" },
      { idempotencyKey: "op-7" });
  });

  // Kills: a rate reading guessed when a header is missing.
  it("rateOf answers null without all three headers", () => {
    expect(rateOf(new Headers({ "X-RateLimit-Used": "412" }))).toBeNull();
    expect(rateOf(new Headers({ "X-RateLimit-Used": "412", "X-RateLimit-Limit": "1000",
      "X-RateLimit-Reset": "2026-09-26T15:00:00Z" }))).toEqual({ used: 412, limit: 1000, resetsAt: "2026-09-26T15:00:00Z" });
  });
});
```

The golden test reads the files by path, never a retyped copy: the fixture **is** task 143's
file (mutant: `quoteText` for `quote`, which `tsc` alone never sees). The `// W5` marker in task
168's `work.go` is this part's convention (task 124 used a separate Go table); task 168 writes the
marker or the test's slice is empty and the row test fails, which is the signal.

#### Tests to write first — `kinds.test.ts` (appended) and `desktop/src/work/replica/intake.test.ts`

1. **Copied by name** — reads `control-plane/internal/hub/hubclient/fake_work_intake.go` through
   `import.meta.glob(…, { query: "?raw" })`, collects every `Kind: "…"` literal, and requires the
   set to equal `W5_FEED_KINDS ∪ W5_ADMIN_KINDS ∪ W5_SIGNAL_KINDS`. **Kills** "a kind retyped
   wrong on the desktop".
2. **Accept then link, in order** — with task 57's fake Hub, team `tm-pay`, the feed carries
   `request.accepted` (rq-1 → is-1, seq 10) then `request.linked` (rq-2 → is-1, seq 11) in one
   page. After apply, `requestsOf("is-1")` lists both requesters, rq-1 first. The same pair
   delivered as seq 11 before seq 10 in one page is sorted and gives the same answer. **Kills**
   "applied out of order", where the second write replaces the first issue's requester list.
3. **Unknown kind** — a page carrying `{kind: "request.escalated_v9", …}` applies the rest of the
   page, throws nothing, and leaves every `IntakeRecords` map unchanged (hub-contract §1). **Kills**
   "a new Hub kind breaks old desktops".
4. **No admin kind creates a record** — a page of every `W5_ADMIN_KINDS` kind, each with a subject
   id, causes **zero** `send` calls and leaves every map in the replica, including task 55's and
   task 125's, the same size (a deep count before and after). **Kills** "keys cached in the
   replica".
5. **The secret is nowhere after the dialog closes** — `createAPIKey` answers
   `{key: {…}, secret: "fixture-secret-7f3a9c"}` from a test double; the test mounts a `Keys`
   dialog stub, closes it, then walks the store's `work` state (task 37's `workSlice`,
   `desktop/src/store/slices/workSlice.ts`) and every replica map with a recursive string search:
   no value equals or contains `"fixture-secret-7f3a9c"`.
   **Kills** "the secret stored". (The stub is a 10-line component in this test file; task 173's
   real dialog runs the same assertion on itself.)
6. **Another team's request is never fetched** — `request.created` with `data.teamId: "tm-other"`
   and a caller of `tm-pay` only causes zero `send` calls and no record. **Kills** "a request's
   quote visible by grant".
7. **No persistence** — task 55's no-persistence grep runs over `kinds.ts`, `replica.ts`, `ops.ts`,
   `visibility.ts` and `work.ts` and passes; then a spy on `Storage.prototype.setItem` during a
   full load plus one `putLoopVersion` records zero calls. A test double whose `createLoop` writes the draft
   to `localStorage` makes the same check fail. **Kills** "a loop draft in `localStorage`".
8. **No loop patch, no admin op** — `INTAKE_OP_KINDS` (the runtime list of `IntakeOp["kind"]`)
   holds no kind starting `loop.patch`, `key.`, `app.`, `webhook.`, `connector.`, `harness.` or
   `skill.`, and `inverseOf` of a `loop.runNow` returns the refusal. **Kills** "a loop edited in
   place" and "an admin write through the replica".

#### Steps

1. Append the wire tests and write `intake.test.ts`; run them; they fail (no `WORK_RELAY_W5`, no
   W5 kinds).
2. Append parts 13-19 to `work.ts` and the ten codes to `client.ts`; the wire tests pass once task
   143's goldens and task 168's rows are on the tree (before that they fail by design).
3. Append the W5 lists to `kinds.ts`; test 1 passes once task 144's fake is on the tree.
4. Add the `replica.ts`, `ops.ts` and `visibility.ts` deltas; tests 2-8 pass.
5. `npm --prefix desktop run typecheck`; commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- src/api/work.test.ts src/work/replica/kinds.test.ts src/work/replica/intake.test.ts
# expect: work.test.ts six more passed than before; kinds.test.ts one more passed;
#         intake.test.ts 7 passed; no failed
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- A surface: tasks 170 to 173 draw these records.
- That the W5 kinds, golden file names, relay paths and header names are the Hub's: tasks 143,
  144, 160 and 168 have no part, and the copy, golden and row tests make them so when those land,
  failing until then.
- That a request's thread is complete: it loads on open and grows by `request.message_added`.

#### Gaps found while writing this part

1. **G1 — task 143's scope, tasks 162-163's scopes and this task's scope name the W5 wire three
   ways.** Loops: task 143 has a flat `Loop` with `grid` as one level name, `LoopRun` with an `lr-`
   id and `loop_outside_grid`; tasks 162-163 have `Loop` + `LoopVersion` + `LoopBound{turnsPerDay,
   levels}` + `LoopRun` keyed `(loopId, run)` and `loop_bound_exceeds_owner`, `loop_paused`. This
   part follows 162-163 (they own the routes; decision 1). Beyond both, tasks 170-173 need:
   `App.actsAs`, `ExternalLink.upstream` (task 150's `UpstreamChange`),
   `AcceptRequestBody.serves`, and the methods `RequestMessages`, `SetTrackerMode`,
   `ListTrackerLinks`, `CreateLoop`, `ProjectLoops`, `GetLoop`, `SetLoopState`, `ListApps`,
   `ListTeamSkills`. Task 143's part must take one set, and the goldens fail here until it does.
2. **G2 — task 144 and task 170 disagree on the accepted issue's description.** Task 144's scope
   says `AcceptRequest` writes revision 1 with **the quote** as the description; task 170's scope
   and 05 §3.5 say the quote is customer content and **never** copied into the issue. This part
   follows 170 (`AcceptRequestBody.description` is the person's text); task 144's rule must change.
3. **G3 — no undo for accept, link or decline.** Task 146's scope names no unlink and task 143's
   list no reopen, so `U` refuses with a fact line. If one is wanted, task 146 adds the route.
4. **G4 — how an `ExternalLink` reaches the desktop.** Task 150 names the record, not whether it
   rides on the issue or needs its own list route; task 171's *not tied · imported* needs it.
5. **G5 — the sync log is per run, task 171 wants per item.** `SyncRun` carries counts and
   `errors`; per-item `imported`/`updated`/`skipped` rows need task 150 to record them. Until then
   task 171 shows counts per run and a row per refusal.
6. **Parts missing.** Tasks 55, 143, 144, 146-168 have no part on this tree; every name from them
   is their scope's and **UNVERIFIED**. The File map's `desktop/src/components/…` row has no home
   for `loops/` (scope 172 names it).
7. **G7 — task 162's paths carry `/work/` after `/v1`.** Every other Work route is `/v1/<noun>`,
   so task 37's `relayPath` gives `/v1/hub/work/<noun>`; task 162's `/v1/work/loops/…` becomes
   `/v1/hub/work/work/loops/…`. It works, but task 162 should drop the `/work` segment (to
   `/v1/loops/…` and `/v1/projects/{projectId}/loops`) so the relay stays uniform.
