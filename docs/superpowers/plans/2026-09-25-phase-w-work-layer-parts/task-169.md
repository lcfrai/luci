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

1. **Task 143's list wins over this task's scope paragraph.** The scope names `SyncLogEntry`,
   `LoopVersion`, `LoopTrigger`, `LoopBound`, `TeamAgentPolicy` and `DigestSettings.teams`; task
   143's paragraph (`…-scopes/tasks-141-160.md:139-174`) names `SyncRun`, `Loop.version`,
   `Loop.trigger`, `Loop.grid`, `HarnessAllow` plus `TeamSkill`, and `TeamsBinding`. The desktop
   mirrors Go, so the types below carry task 143's names. `LoopVersion` survives as a desktop-only
   name for "a `Loop` as it was at one `version`" (decision 6), and `SyncLogEntry` as a desktop
   **view** derived from a `SyncRun`, never a wire type (decision 7). The mismatch is a gap, below.
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
6. **A loop edit is a new version.** `ops.ts` gains `putLoop`, which sends the whole loop with the
   `version` it was read at; the Hub answers the next `version` (task 162). There is no op that
   patches a loop in place, and `ListLoopVersions` answers `Loop[]`, newest first; the desktop
   aliases one element as `LoopVersion`.
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
export interface ExternalLink {
  id: string;                  // xl-…
  source: TrackerSource; externalId: string; issueId: string; url: string; lastSeenAt: string;
}
export interface SyncError { externalId: string; code: ProblemCode }
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
/** A class absent from the map inherits the owner's own grid (shown greyed by task 172). */
export type LoopGrid = Partial<Record<ActionClass, AutonomyLevel>>;

export type ScheduleEvery = "day" | "weekday" | "week";
export interface LoopTrigger {
  kind: "event" | "schedule";
  eventKinds: string[];        // SW.5 and SW.21 kinds; [] for a schedule
  filter: { labels: string[]; states: string[] };   // event triggers only
  every: ScheduleEvery | null; at: string | null;   // HH:MM
  tz: string | null;           // IANA, the owner's
}
export type LoopAction = { kind: "turn" } | { kind: "start_on_accept"; label: string };
export type LoopState = "active" | "paused";
export interface Loop {
  id: string;                  // lp-…
  ownerId: string; teamId: string;
  name: string;                // 1-80 characters
  trigger: LoopTrigger;
  instruction: AuthoredText;   // text·H, at most 2000 characters
  action: LoopAction;
  grid: LoopGrid;
  version: number;             // the Hub's; a write sends the version it read
  state: LoopState;
  author: User; at: string;    // who wrote this version, and when
}
/** One element of ListLoopVersions: the loop as it was at `version`. A desktop name only. */
export type LoopVersion = Loop;

export type LoopCause = "event" | "schedule" | "run_now";
export type LoopOutcome = "queued" | "ran" | "skipped_budget" | "skipped_paused" | "refused_bound" | "failed";
export interface LoopRun {
  id: string;                  // lr-…
  loopId: string; version: number; cause: LoopCause;
  startedAt: string; endedAt: string | null;
  outcome: LoopOutcome;
  turnRef: string | null;      // ot-… (Phase T's Orchestrator turn)
  sessionId: string | null; worktree: string | null;   // set when the action was a Start
}
```

Task 143's scope types `Loop` without `name`, `action`, `trigger.filter`, `author` or `at`,
`Loop.grid` as one level name, `LoopRun.outcome` as an unnamed string, and `LoopRun` without the
Start's session and worktree. Task 172 needs every one of them. They are written here as the
desktop needs them and **proposed to task 143** (gap G1); the golden test fails until task 143
carries them, which is the signal the two parts disagree, exactly as task 125 did for
`featurePins`.
