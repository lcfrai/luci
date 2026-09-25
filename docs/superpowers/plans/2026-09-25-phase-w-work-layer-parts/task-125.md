### Task 125: desktop W4 wire types and replica kinds

W4, stream WD, the desktop half of seam **SW.20**, as tasks 37 and 83 were for W1 and W2. SW.15
(the desktop Work types) and SW.16 (the replica) grow by **added** types, calls and kinds only.
Depends on task 83 (W2's desktop types: `Feature`, `Cycle`, `Template`; its part was not on the
tree when this was written, so every task 83 name below is its scope's and **UNVERIFIED**) and task
115 (the fake's W4 rules, whose event kinds this task copies by name). It also reads task 124's
route table (`task-124.md`, *the route table, complete*), task 37's `WORK_RELAY`, `relayPath`,
`WorkSend`, `WIRE_KEYS` and `workProblem` (`task-37.md:257-496`), and task 57's `classify`,
`ProjectScope` and `admit` (`task-57.md:88-215`). Estimate 0.25 (2 h).

After this task the desktop can name every W4 record, call every W4 relay route, and hold the W4
records in its replica, dropping them the moment the caller loses sight of their project. It
draws nothing: tasks 126 to 141 do.

**Decisions made here, so nobody re-derives them:**

1. **Field for field with Go, by value, not by eye.** Each W4 type has a `WIRE_KEYS` entry held to
   its type both ways by task 37's `keys<T>()`, and a golden test reads task 114's
   `control-plane/internal/hub/hubapi/testdata/work-roadmap/*.json` **by path** and requires every
   golden's keys to equal the type's list. A field renamed on either side (`target` for
   `targetDate`) fails that test even where `tsc` passes, because `tsc` never sees the Go side.
2. **Kinds are copied by name, never retyped.** `W4_FEED_KINDS` is a literal list, and a test reads
   task 115's `fake_work_roadmap.go` by path, extracts every `Kind: "…"` literal it appends, and
   requires the two sets to be equal. A kind the Hub adds later and this list lacks is **ignored**
   by the replica (decision 5), never thrown.
3. **Insights and read cursors are not replica kinds.** `InsightsResult` and `ReadCursor` are
   fetched on demand through `WorkClient` and held in component memory only; nothing in the
   replica, the store or `localStorage` keeps them (§3.1, *no Hub data on disk*; ADR-H1). Task 55's
   no-persistence grep is extended to the new modules and must still pass.
4. **An update has no edit op.** `ops.ts` gains `postUpdate` (a create) and no op that writes an
   existing `nt-`: a `StatusUpdate` is immutable (task 114), and a correction is a new update.
5. **A grant is visibility, not data.** `grant.created` naming one of the caller's teams re-lists
   and loads the project in full (task 57's `relist` path); `grant.revoked` naming one of the
   caller's teams drops every record of that project in **one** `transact` and sets `accessLost`,
   exactly as task 57's `drop_team` drops a team's projects. The caller's own team's grants to
   others change nothing the caller can see, so they only update the grant list.

**Files:**
- `desktop/src/api/work.ts` ⚑ (task 37; task 83 before this) — the W4 records, request bodies,
  `WORK_RELAY_W4`, the `WorkClient` members, `WIRE_KEYS` entries (modified)
- `desktop/src/api/work.test.ts` ⚑ — the W4 goldens, the relay table against task 124's (modified)
- `desktop/src/api/client.ts` ⚑ — the four W4 codes join `ProblemCode` (modified)
- `desktop/src/work/replica/kinds.ts` — `W4_FEED_KINDS`, `W4_SUBJECT_PREFIX` (new)
- `desktop/src/work/replica/kinds.test.ts` — the copied-by-name test (new)
- `desktop/src/work/replica/replica.ts` ⚑ (task 55) — the W4 record maps and their load (modified)
- `desktop/src/work/replica/ops.ts` ⚑ (task 56) — the W4 ops, no update edit (modified)
- `desktop/src/work/replica/visibility.ts` ⚑ (task 57) — `grant.created`/`grant.revoked` (modified)
- `desktop/src/work/replica/roadmap.test.ts` — grant, revocation, unknown kind, no persistence (new)

**What is on the tree today (verified 2026-09-25).** On this branch's tree, parts exist for tasks
37, 56 and 57 (`…-parts/task-37.md`, `task-56.md`, `task-57.md`); there is no part for tasks 55,
83, 114 or 115, so their names are their scopes' (`…-scopes/tasks-41-60.md`, `tasks-81-100.md`,
`tasks-101-120.md:352-405`) and **UNVERIFIED**. The desktop source files named above are planned
by those tasks and do not exist on `main` yet. The Go JSON tags mirrored here are the ones task
124's part calls (`task-124.md`); task 114's part wins any difference, and this file follows it.

#### Interfaces — complete: `work.ts`, part 9: the W4 records

```ts
// desktop/src/api/work.ts (appended) — SW.20's desktop half. Every field is task 114's JSON tag.

/** The three health words (task 114). A person picks one; nothing defaults it. */
export const HEALTH_WORDS = ["on_track", "at_risk", "off_track"] as const;
export type HealthWord = (typeof HEALTH_WORDS)[number];

/** The feature's state words, reused for an initiative (task 114 fixes them). */
export type InitiativeState = "planned" | "active" | "paused" | "done" | "cancelled";

/** 05 §3.3's five convergence words for a criterion, from task 10's thread state. */
export const CONVERGENCE_WORDS = ["settled", "open", "contested", "drifting", "uncovered"] as const;
export type ConvergenceWord = (typeof CONVERGENCE_WORDS)[number];
export interface CriterionWord { criterionId: string; word: ConvergenceWord }

export interface Initiative {
  id: string;            // in-…
  teamId: string;        // the owning team
  name: string;
  owner: string;         // u-…, a member of teamId
  targetDate: string | null; // YYYY-MM-DD, set by a person
  state: InitiativeState;
}
export interface InitiativeTeams { initiativeId: string; teamIds: string[] }

/** One feature row of an initiative's view. latestUpdate is null when none was posted: never a
 *  default word. There is no aggregate field anywhere in the view (L-84). */
export interface InitiativeFeatureRow {
  featureId: string; name: string; lead: string | null;
  latestUpdate: { health: HealthWord; at: string } | null;
  criteria: CriterionWord[];
}
export interface InitiativeView { initiative: Initiative; features: InitiativeFeatureRow[] }

export type UpdateSubjectKind = "feature" | "initiative";
export interface UpdateSubject { kind: UpdateSubjectKind; id: string }

/** Immutable once posted. body is text·H; pinnedBrief is "brief:bf-…@r"; cites are keys. */
export interface StatusUpdate {
  id: string;            // nt-…
  kind: "update";
  subject: UpdateSubject;
  health: HealthWord;
  body: AuthoredText;    // task 37's {text, author}; empty text when erased
  pinnedBrief: string;
  cites: string[];
  author: string;        // u-…, always a person (task 119)
  at: string;
  erasedAt: string | null;
}

export interface Milestone {
  id: string;            // ml-…
  featureId: string; name: string;
  date: string;          // YYYY-MM-DD
  criterionIds: string[];
}
/** reached is derived by the Hub at read; the desktop never computes or stores it. */
export interface MilestoneView { milestone: Milestone; criteria: CriterionWord[]; reached: boolean }

/** `from` blocks `blocks`. orderViolated is the Hub's, derived from the two target dates. */
export interface FeatureDependency { from: string; blocks: string; orderViolated: boolean }

export interface BriefTemplate {
  id: string;            // tp-…
  kind: "brief";
  criteria: AuthoredText[];
  issueTemplateIds: string[];
}
export type ScheduleUnit = "day" | "week" | "month";
export interface TemplateSchedule {
  templateId: string; every: { n: number; unit: ScheduleUnit };
  at: string;            // HH:MM, local to tz
  tz: string;            // IANA, e.g. Australia/Sydney
}
export interface ReminderPolicy { subject: UpdateSubject; everyDays: number }
export interface TriageRotation { teamId: string; members: string[]; periodDays: number; startsOn: string }
/** The read adds who is on duty and the policy's next handover date: a date, not a forecast. */
export interface TriageRotationView { rotation: TriageRotation; onDuty: string | null; nextHandoverOn: string }
```

#### Interfaces — complete: `work.ts`, part 10: insights, cursors, grants, request bodies

```ts
export type InsightsMeasure = "count" | "cycle_time" | "lead_time" | "triage_time";
export type InsightsGroupBy = "state" | "assignee" | "label" | "feature" | "cycle" | "priority" | "criterion";
export interface InsightsQuery {
  teamId: string; projectId?: string;
  from: string; to: string;          // RFC 3339; from < to, at most 366 days (task 122)
  measure: InsightsMeasure; groupBy?: InsightsGroupBy;
}
/** Measured only. The percentiles are null when there is no point, never 0. Held in memory. */
export interface InsightsResult {
  groups: { key: string; count: number }[];
  points: { issue: string; ms: number }[];
  p50Ms: number | null; p75Ms: number | null; p90Ms: number | null;
}

export type ReadScopeKind = "project" | "feature" | "issue";
export interface ReadScope { kind: ReadScopeKind; id: string }
/** The caller's own cursor; seq is a hubapi.Event.Seq. Held in memory, never on disk. */
export interface ReadCursor { scope: ReadScope; seq: number }
/** Task 123's scoped read: at most 200 events on a first look, then truncated is true. */
export interface ScopedChangesPage { events: FeedEvent[]; next: string; more: boolean; truncated: boolean }
export const scopeParam = (s: ReadScope): string => `${s.kind}:${s.id}`;

export interface ProjectGrant {
  projectId: string; teamId: string;
  scope: "read_comment";             // the only grant there is (Q12)
  grantedBy: string; at: string;
}

export interface CreateInitiativeRequest { name: string; owner: string; targetDate: string | null }
export interface PatchInitiativeRequest {
  name?: string; owner?: string; targetDate?: string | null; state?: InitiativeState;
}
/** health has no default: the composer (task 127) cannot build this without a person's pick. */
export interface PostUpdateRequest { health: HealthWord; body: AuthoredText; pinnedBrief: string; cites: string[] }
export interface CreateMilestoneRequest { name: string; date: string; criterionIds: string[] }
export interface PatchMilestoneRequest { name?: string; date?: string }
export interface CreateFeatureFromTemplateRequest { templateId: string; name: string; branchRef: string }
/** Task 120's one-transaction create: the feature, its brief at revision 1, one issue per template. */
export interface FeatureFromTemplate { feature: Feature; brief: BriefRevision; issues: IssueWithRevision[] }
```

`FeedEvent` is task 55's decoded `hubapi.Event` (task 57's `FeedEventLike` plus `actor` and
`data`); `Feature` is task 83's; `BriefRevision` is the H-plan's desktop type
(`desktop/src/api/hub.ts`, **UNVERIFIED until merge**); `AuthoredText`, `IssueWithRevision` and
`Page` are task 37's.

#### Interfaces — complete: `work.ts`, part 11: the relay table and the `WorkClient` members

```ts
/** Task 124's WorkRoadmapRoutes, row for row and in its order. relayPath() derives the loopback
 *  path, so "/v1/changes" becomes "/v1/hub/work/changes", never H's "/v1/hub/changes". */
export const WORK_RELAY_W4 = [
  ["GrantProject", "POST", "/v1/projects/{projectId}/grants"],
  ["ProjectGrants", "GET", "/v1/projects/{projectId}/grants"],
  ["RevokeProjectGrant", "DELETE", "/v1/projects/{projectId}/grants/{teamId}"],
  ["SetInitiativeTeams", "PUT", "/v1/initiatives/{initiativeId}/teams"],
  ["CreateInitiative", "POST", "/v1/teams/{teamId}/initiatives"],
  ["Initiatives", "GET", "/v1/teams/{teamId}/initiatives"],
  ["Initiative", "GET", "/v1/initiatives/{initiativeId}"],
  ["PatchInitiative", "PATCH", "/v1/initiatives/{initiativeId}"],
  ["SetFeatureInitiative", "PUT", "/v1/features/{featureId}/initiative"],
  ["PostFeatureUpdate", "POST", "/v1/features/{featureId}/updates"],
  ["FeatureUpdates", "GET", "/v1/features/{featureId}/updates"],
  ["PostInitiativeUpdate", "POST", "/v1/initiatives/{initiativeId}/updates"],
  ["InitiativeUpdates", "GET", "/v1/initiatives/{initiativeId}/updates"],
  ["CreateMilestone", "POST", "/v1/features/{featureId}/milestones"],
  ["Milestones", "GET", "/v1/features/{featureId}/milestones"],
  ["PatchMilestone", "PATCH", "/v1/milestones/{milestoneId}"],
  ["SetMilestoneCriteria", "PUT", "/v1/milestones/{milestoneId}/criteria"],
  ["AddFeatureDependency", "POST", "/v1/features/{featureId}/dependencies"],
  ["FeatureDependencies", "GET", "/v1/features/{featureId}/dependencies"],
  ["RemoveFeatureDependency", "DELETE", "/v1/features/{featureId}/dependencies/{blocks}"],
  ["CreateFeatureFromTemplate", "POST", "/v1/projects/{projectId}/features:fromTemplate"],
  ["SetTemplateSchedule", "PUT", "/v1/templates/{templateId}/schedule"],
  ["ClearTemplateSchedule", "DELETE", "/v1/templates/{templateId}/schedule"],
  ["SetReminderPolicy", "PUT", "/v1/reminder-policies"],
  ["SetTriageRotation", "PUT", "/v1/teams/{teamId}/triage-rotation"],
  ["TriageRotation", "GET", "/v1/teams/{teamId}/triage-rotation"],
  ["QueryInsights", "POST", "/v1/teams/{teamId}/insights/query"],
  ["GetReadCursor", "GET", "/v1/me/read-cursors"],
  ["AdvanceReadCursor", "PUT", "/v1/me/read-cursors"],
  ["MarkSeen", "POST", "/v1/me/read-cursors/mark-seen"],
  ["ScopedChanges", "GET", "/v1/changes"],
] as const satisfies ReadonlyArray<readonly [string, Verb, `/v1/${string}`]>;
export type RelayMethodW4 = (typeof WORK_RELAY_W4)[number][0];
```

```ts
/** SW.20 on the desktop. WorkClient (task 37) extends it, so createClient's `work` has every
 *  member. One member per method of task 124's client, lower camel case; two rows each serve
 *  postUpdate and updates, by subject.kind. */
export interface WorkRoadmapClient {
  grantProject(projectId: string, teamId: string): Promise<ProjectGrant>;
  projectGrants(projectId: string): Promise<ProjectGrant[]>;
  revokeProjectGrant(projectId: string, teamId: string): Promise<void>;
  setInitiativeTeams(initiativeId: string, teamIds: string[]): Promise<InitiativeTeams>;
  createInitiative(teamId: string, req: CreateInitiativeRequest): Promise<Initiative>;
  initiatives(teamId: string, page?: PageRequest): Promise<Page<Initiative>>;
  initiative(initiativeId: string): Promise<InitiativeView>;
  patchInitiative(initiativeId: string, req: PatchInitiativeRequest): Promise<Initiative>;
  setFeatureInitiative(featureId: string, initiativeId: string | null): Promise<Feature>;
  postUpdate(subject: UpdateSubject, req: PostUpdateRequest): Promise<StatusUpdate>;
  updates(subject: UpdateSubject, page?: PageRequest): Promise<Page<StatusUpdate>>;
  createMilestone(featureId: string, req: CreateMilestoneRequest): Promise<Milestone>;
  milestones(featureId: string): Promise<MilestoneView[]>;
  patchMilestone(milestoneId: string, req: PatchMilestoneRequest): Promise<Milestone>;
  setMilestoneCriteria(milestoneId: string, criterionIds: string[]): Promise<Milestone>;
  addFeatureDependency(featureId: string, blocks: string): Promise<FeatureDependency>;
  featureDependencies(featureId: string): Promise<FeatureDependency[]>;
  removeFeatureDependency(featureId: string, blocks: string): Promise<void>;
  createFeatureFromTemplate(projectId: string, req: CreateFeatureFromTemplateRequest): Promise<FeatureFromTemplate>;
  setTemplateSchedule(templateId: string, s: TemplateSchedule): Promise<TemplateSchedule>;
  clearTemplateSchedule(templateId: string): Promise<void>;
  setReminderPolicy(p: ReminderPolicy): Promise<ReminderPolicy>;
  setTriageRotation(teamId: string, r: TriageRotation): Promise<TriageRotationView>;
  triageRotation(teamId: string): Promise<TriageRotationView>;
  queryInsights(teamId: string, q: InsightsQuery): Promise<InsightsResult>;
  readCursor(scope: ReadScope): Promise<ReadCursor>;
  advanceReadCursor(scope: ReadScope, seq: number): Promise<ReadCursor>;
  markSeen(scopes: ReadScope[], seq: number): Promise<ReadCursor[]>;
  scopedChanges(scope: ReadScope, after?: string, limit?: number): Promise<ScopedChangesPage>;
}
// task 37's declaration becomes: export interface WorkClient extends WorkRoadmapClient { … }
```

`makeWorkClient` (task 37, part 7) gains one helper and the 29 members, each one line over it.
`fill` is task 37's segment filler (`encodeURIComponent` per `{…}`); `w4` looks a row up by name
so a renamed row fails the route test, not a person's click.

```ts
// inside makeWorkClient(send), after task 37's members
  const row = (m: RelayMethodW4) => WORK_RELAY_W4.find((r) => r[0] === m)!;
  /** q is task 37's query() output: "" or "?…". */
  const w4 = (m: RelayMethodW4, args: string[], q = "", body?: unknown) => {
    const [, verb, hub] = row(m);
    return send(verb, relayPath(fill(hub, args)) + q, body);
  };
  // task 37's own `items` takes a Page; these routes answer a bare {items}, so a second name.
  const listOf = async <T,>(p: Promise<unknown>) => ((await p) as { items: T[] }).items;
  const byKind = (s: UpdateSubject, f: RelayMethodW4, i: RelayMethodW4) => (s.kind === "feature" ? f : i);
  const roadmap: WorkRoadmapClient = {
    grantProject: (p, teamId) => w4("GrantProject", [p], "", { teamId }) as Promise<ProjectGrant>,
    projectGrants: (p) => listOf<ProjectGrant>(w4("ProjectGrants", [p])),
    revokeProjectGrant: async (p, t) => void (await w4("RevokeProjectGrant", [p, t])),
    setInitiativeTeams: (i, teamIds) => w4("SetInitiativeTeams", [i], "", { initiativeId: i, teamIds }) as Promise<InitiativeTeams>,
    createInitiative: (t, req) => w4("CreateInitiative", [t], "", req) as Promise<Initiative>,
    initiatives: (t, page) => w4("Initiatives", [t], pageQ(page)) as Promise<Page<Initiative>>,
    initiative: (i) => w4("Initiative", [i]) as Promise<InitiativeView>,
    patchInitiative: (i, req) => w4("PatchInitiative", [i], "", req) as Promise<Initiative>,
    setFeatureInitiative: (f, initiativeId) => w4("SetFeatureInitiative", [f], "", { initiativeId }) as Promise<Feature>,
    postUpdate: (s, req) => w4(byKind(s, "PostFeatureUpdate", "PostInitiativeUpdate"), [s.id], "", req) as Promise<StatusUpdate>,
    updates: (s, page) => w4(byKind(s, "FeatureUpdates", "InitiativeUpdates"), [s.id], pageQ(page)) as Promise<Page<StatusUpdate>>,
    createMilestone: (f, req) => w4("CreateMilestone", [f], "", req) as Promise<Milestone>,
    milestones: (f) => listOf<MilestoneView>(w4("Milestones", [f])),
    patchMilestone: (m, req) => w4("PatchMilestone", [m], "", req) as Promise<Milestone>,
    setMilestoneCriteria: (m, criterionIds) => w4("SetMilestoneCriteria", [m], "", { criterionIds }) as Promise<Milestone>,
    addFeatureDependency: (f, blocks) => w4("AddFeatureDependency", [f], "", { blocks }) as Promise<FeatureDependency>,
    featureDependencies: (f) => listOf<FeatureDependency>(w4("FeatureDependencies", [f])),
    removeFeatureDependency: async (f, b) => void (await w4("RemoveFeatureDependency", [f, b])),
    createFeatureFromTemplate: (p, req) => w4("CreateFeatureFromTemplate", [p], "", req) as Promise<FeatureFromTemplate>,
    setTemplateSchedule: (t, s) => w4("SetTemplateSchedule", [t], "", s) as Promise<TemplateSchedule>,
    clearTemplateSchedule: async (t) => void (await w4("ClearTemplateSchedule", [t])),
    setReminderPolicy: (p) => w4("SetReminderPolicy", [], "", p) as Promise<ReminderPolicy>,
    setTriageRotation: (t, r) => w4("SetTriageRotation", [t], "", r) as Promise<TriageRotationView>,
    triageRotation: (t) => w4("TriageRotation", [t]) as Promise<TriageRotationView>,
    queryInsights: (t, q) => w4("QueryInsights", [t], "", q) as Promise<InsightsResult>,
    readCursor: (s) => w4("GetReadCursor", [], query([["scope", scopeParam(s)]])) as Promise<ReadCursor>,
    advanceReadCursor: (s, seq) => w4("AdvanceReadCursor", [], "", { scope: s, seq }) as Promise<ReadCursor>,
    markSeen: (scopes, seq) => listOf<ReadCursor>(w4("MarkSeen", [], "", { scopes, seq })),
    scopedChanges: (s, after, limit) =>
      w4("ScopedChanges", [], query([["scope", scopeParam(s)], ["after", after], ["limit", limit]])) as Promise<ScopedChangesPage>,
  };
  // returned object: { ...task 37's members, ...roadmap }
```

`query`, `pageQ` and `fill` are task 37's (`task-37.md:377-397`): an absent or `""` value adds
nothing, so `scopedChanges(s)` sends only `?scope=`. No member reads or writes `localStorage`,
`sessionStorage` or IndexedDB.

#### Interfaces — complete: `work.ts`, part 12: `WIRE_KEYS` for W4, and the codes

```ts
// appended inside task 37's WIRE_KEYS object; keys<T>() holds each list to its type both ways
  initiative: keys<Initiative>({ id: true, teamId: true, name: true, owner: true, targetDate: true, state: true }),
  initiativeTeams: keys<InitiativeTeams>({ initiativeId: true, teamIds: true }),
  initiativeFeatureRow: keys<InitiativeFeatureRow>({ featureId: true, name: true, lead: true, latestUpdate: true, criteria: true }),
  initiativeView: keys<InitiativeView>({ initiative: true, features: true }),
  statusUpdate: keys<StatusUpdate>({ id: true, kind: true, subject: true, health: true, body: true,
    pinnedBrief: true, cites: true, author: true, at: true, erasedAt: true }),
  milestone: keys<Milestone>({ id: true, featureId: true, name: true, date: true, criterionIds: true }),
  milestoneView: keys<MilestoneView>({ milestone: true, criteria: true, reached: true }),
  featureDependency: keys<FeatureDependency>({ from: true, blocks: true, orderViolated: true }),
  briefTemplate: keys<BriefTemplate>({ id: true, kind: true, criteria: true, issueTemplateIds: true }),
  templateSchedule: keys<TemplateSchedule>({ templateId: true, every: true, at: true, tz: true }),
  reminderPolicy: keys<ReminderPolicy>({ subject: true, everyDays: true }),
  triageRotation: keys<TriageRotation>({ teamId: true, members: true, periodDays: true, startsOn: true }),
  insightsQuery: keys<InsightsQuery>({ teamId: true, projectId: true, from: true, to: true, measure: true, groupBy: true }),
  insightsResult: keys<InsightsResult>({ groups: true, points: true, p50Ms: true, p75Ms: true, p90Ms: true }),
  readCursor: keys<ReadCursor>({ scope: true, seq: true }),
  projectGrant: keys<ProjectGrant>({ projectId: true, teamId: true, scope: true, grantedBy: true, at: true }),

/** The golden file of task 114 each W4 type is held to: testdata/work-roadmap/<file>.json. */
export const W4_GOLDENS = {
  initiative: "initiative", initiativeTeams: "initiative-teams", initiativeView: "initiative-view",
  statusUpdate: "status-update", milestone: "milestone", milestoneView: "milestone-view",
  featureDependency: "feature-dependency", briefTemplate: "brief-template",
  templateSchedule: "template-schedule", reminderPolicy: "reminder-policy",
  triageRotation: "triage-rotation", insightsQuery: "insights-query",
  insightsResult: "insights-result", readCursor: "read-cursor", projectGrant: "project-grant",
} as const satisfies Partial<Record<keyof typeof WIRE_KEYS, string>>;
```

The fifteen types the scope lists each have a golden; `InitiativeFeatureRow` is checked inside
`initiative-view.json`. The golden file names are this part's; task 114's part fixes them and
this map follows (a gap if 114 names none).

```ts
// desktop/src/api/client.ts ⚑ — ProblemCode gains task 114's four appended codes
  /* the W4 Work wire (tasks 114, 124) */
  | "dependency_cycle" | "criterion_not_on_brief" | "update_immutable" | "grant_exists"
```

The comment above `ProblemCode` (task 37 made it "twenty-seven") becomes "thirty-one", plus
whatever task 83 added before this. `workProblem` files all four as `refused`, with `details`
intact, so the timeline and the composer can say which dependency and which criterion.

#### Interfaces — complete: `desktop/src/work/replica/kinds.ts`

```ts
// desktop/src/work/replica/kinds.ts — the W4 feed kinds the replica applies (SW.5, SW.20).
// COPIED BY NAME from task 115's fake_work_roadmap.go; kinds.test.ts reads that file and fails
// on any difference. Never retype one of these from memory.
export const W4_FEED_KINDS = [
  "initiative.created", "initiative.updated", "initiative.teams_set",
  "update.posted", "update.erased",
  "milestone.created", "milestone.updated",
  "feature_dep.added", "feature_dep.removed",
  "grant.created", "grant.revoked",
  "update.reminder", "triage.handover",
] as const;
export type W4FeedKind = (typeof W4_FEED_KINDS)[number];

/** The record a kind's subject names, by id prefix (05 §3.6). A kind whose subject has none of
 *  these is a signal, not a record: update.reminder and triage.handover go to the Desk only. */
export const W4_SUBJECT_PREFIX = { "in-": "initiative", "nt-": "update", "ml-": "milestone", "ft-": "feature" } as const;

/** Signals the replica passes on and does not store (the Desk rows of tasks 127 and 134). */
export const W4_SIGNAL_KINDS: ReadonlySet<string> = new Set(["update.reminder", "triage.handover"]);

/** Kinds the fake appends that the replica deliberately does not hold: policy writes, read by the
 *  settings surfaces on demand (tasks 127, 134). Listed so the copy test can require
 *  fake kinds = W4_FEED_KINDS ∪ W4_NOT_HELD exactly. */
export const W4_NOT_HELD = [
  "reminder_policy.set", "template_schedule.set", "template_schedule.cleared", "triage_rotation.set",
] as const;

export const isW4Kind = (k: string): k is W4FeedKind => (W4_FEED_KINDS as readonly string[]).includes(k);
```

Of the thirteen held kinds, eight are the ones task 124's story appends (`task-124.md`, *the
story*); five it does not reach (`initiative.updated`, `update.erased`, `milestone.updated`,
`feature_dep.removed`, `triage.handover`). **All seventeen names are UNVERIFIED until task 115's
part lands**; the copy test is what makes them true, and it fails until they are.

#### Interfaces — complete: the deltas on ⚑ `replica.ts` (task 55), ⚑ `ops.ts` (task 56)

```ts
// desktop/src/work/replica/replica.ts — task 125 adds these record maps beside task 55's.
import type { FeatureDependency, Initiative, MilestoneView, ProjectGrant, StatusUpdate } from "../../api/work";

/** What the replica holds for W4. Keys are Hub ids; every record passes task 57's admit(). */
export interface RoadmapRecords {
  initiatives: ReadonlyMap<string, Initiative>;          // in-…
  updates: ReadonlyMap<string, StatusUpdate>;            // nt-…, per subject newest first on read
  milestones: ReadonlyMap<string, MilestoneView>;        // ml-…
  featureDeps: ReadonlyMap<string, FeatureDependency>;   // `${from}>${blocks}`
  grants: ReadonlyMap<string, ProjectGrant>;             // `${projectId}/${teamId}`
}

/** Task 55's ReplicaDeps gains one member: every project granted to one of the caller's teams.
 *  No Hub route lists these yet (a gap for task 117, below); until it lands the replica learns
 *  grants only from grant.created events seen while it runs. */
export interface ReplicaGrantDeps { grantedProjects(): Promise<ProjectScope[]> }

// Replica (task 55) gains:
//   roadmap(): RoadmapRecords
//   updatesOf(subject: UpdateSubject): StatusUpdate[]     // newest first, by `at` then id
//   milestonesOf(featureId: string): MilestoneView[]      // by date
//   depsOf(featureId: string): FeatureDependency[]        // both directions
//   signals(listener: (e: FeedEvent) => void): () => void // update.reminder, triage.handover
```

**How W4 enters the replica**, in order, beside task 55's paths:

1. **Load.** For each `scope` project, after task 55's `listAll`, the replica loads the team's
   `initiatives` (one call per team, not per project), and for each feature: `updates`,
   `milestones`, `featureDependencies`; for each project the caller's team **owns**:
   `projectGrants`. A granted project (`ProjectScope.access === "granted"`) loads the same
   records except grants, which the Hub refuses to a granted team.
2. **Each event** whose kind `isW4Kind` re-fetches its subject by the prefix map and puts it in
   one `transact`; a signal kind is passed to `signals` listeners and stored nowhere. A kind that is
   neither W1-W3's nor `isW4Kind` nor in `W4_NOT_HELD` is **ignored**: no throw, no log line per
   event (one `console.debug` per distinct unknown kind per session).
3. **`update.erased`** runs task 57's erase pattern on the update: `body.text` becomes `""`,
   `erasedAt` is set, and id, health, pin, cites and author stay.

```ts
// desktop/src/work/replica/ops.ts — task 56's Op union gains these; each has an inverse for U.
export type RoadmapOp =
  | { kind: "initiative.create"; teamId: string; req: CreateInitiativeRequest }
  | { kind: "initiative.patch"; initiativeId: string; req: PatchInitiativeRequest; prev: PatchInitiativeRequest }
  | { kind: "initiative.teams"; initiativeId: string; teamIds: string[]; prev: string[] }
  | { kind: "feature.initiative"; featureId: string; initiativeId: string | null; prev: string | null }
  | { kind: "update.post"; subject: UpdateSubject; req: PostUpdateRequest }   // no inverse: U is refused
  | { kind: "milestone.create"; featureId: string; req: CreateMilestoneRequest }
  | { kind: "milestone.patch"; milestoneId: string; req: PatchMilestoneRequest; prev: PatchMilestoneRequest }
  | { kind: "milestone.criteria"; milestoneId: string; criterionIds: string[]; prev: string[] }
  | { kind: "featureDep.add"; featureId: string; blocks: string }
  | { kind: "featureDep.remove"; featureId: string; blocks: string }
  | { kind: "grant.create"; projectId: string; teamId: string }
  | { kind: "grant.revoke"; projectId: string; teamId: string };
// There is deliberately no "update.edit", "update.patch" or "update.delete": an update is
// immutable (task 114). roadmap.test.ts asserts no RoadmapOp kind starts with "update." other
// than "update.post".
```

Each op's inverse is task 56's `inverseOf(op, before)` (`task-56.md:45-54`; `prev` is the `before` it reads): `create` inverts to the
Hub's archive or delete where one exists, `patch`/`teams`/`criteria` to a patch with `prev`,
`add` to `remove` and back, `grant.create` to `grant.revoke` and back (task 126's *Undo* is a fresh
grant, written as the person). `update.post` has **no** inverse: `U` after a post is refused with
the fact line *"An update cannot be undone once posted; post a correction"*. `milestone.create`
has no Hub delete in task 120's routes, so its inverse is refused the same way (a gap for 120).

#### Interfaces — complete: the delta on ⚑ `visibility.ts` (task 57)

```ts
// desktop/src/work/replica/visibility.ts — ProjectScope and classify gain the grant path.
export interface ProjectScope { projectId: string; teamId: string; key: string; access: "member" | "granted" }

export type VisibilityAction =
  | /* task 57's four */ { kind: "none" } | { kind: "drop_team"; teamId: string }
  | { kind: "relist"; teamId: string } | { kind: "erase"; what: "issue" | "comment"; subject: string }
  | { kind: "drop_project"; projectId: string }   // grant.revoked for one of the caller's teams
  | { kind: "load_project"; projectId: string };  // grant.created for one of the caller's teams

/** Runs before task 57's classify. data.teamId is the grantee (task 117's event data). */
export function classifyGrant(e: FeedEventLike & { data?: Record<string, unknown> },
                              callerTeams: ReadonlySet<string>): VisibilityAction | null {
  if (e.kind !== "grant.created" && e.kind !== "grant.revoked") return null;
  const grantee = typeof e.data?.teamId === "string" ? e.data.teamId : null;
  if (!grantee || !callerTeams.has(grantee) || !e.projectId) return { kind: "none" };
  return e.kind === "grant.revoked"
    ? { kind: "drop_project", projectId: e.projectId }
    : { kind: "load_project", projectId: e.projectId };
}
```

`drop_project` runs `drop_team`'s transaction for one project: every issue, feature, milestone,
update and dependency of it goes in **one** `transact`, `scope` loses it, `accessLost` is set.
`load_project` adds it to `scope` with `access: "granted"` and runs a full `listAll`.

#### Tests to write first — appended to ⚑ `desktop/src/api/work.test.ts`

```ts
// Task 114's W4 goldens, read where the Hub keeps them; the count stops an empty glob passing.
const W4 = import.meta.glob<string>(
  "../../../control-plane/internal/hub/hubapi/testdata/work-roadmap/*.json",
  { query: "?raw", import: "default", eager: true },
);
const HUB_TABLE = import.meta.glob<string>(
  "../../../control-plane/internal/hub/hubclient/methods_work_roadmap.go",
  { query: "?raw", import: "default", eager: true },
);

describe("SW.20 the W4 wire", () => {
  // Kills: a renamed field (target for targetDate) that tsc alone misses, on either side.
  it("each W4 golden's keys are its desktop type's keys", () => {
    expect(Object.keys(W4)).toHaveLength(Object.keys(W4_GOLDENS).length);
    for (const [type, file] of Object.entries(W4_GOLDENS)) {
      const path = Object.keys(W4).find((p) => p.endsWith(`/${file}.json`));
      expect(path, file).toBeDefined();
      const v = JSON.parse(W4[path!]) as Record<string, unknown>;
      expect(Object.keys(v).sort(), file).toEqual(WIRE_KEYS[type as keyof typeof W4_GOLDENS]);
      expect(canon(pick(v, WIRE_KEYS[type as keyof typeof W4_GOLDENS])), file).toBe(canon(v));
    }
  });

  // Kills: a W4 relay row that drifts from task 124's Go table.
  it("WORK_RELAY_W4 is task 124's WorkRoadmapRoutes, row for row", () => {
    const go = Object.values(HUB_TABLE)[0];
    const rows = [...go.matchAll(/\{"(\w+)", http\.Method(\w+), "([^"]+)", (?:true|false)\}/g)]
      .map(([, m, verb, path]) => [m, verb.toUpperCase(), path]);
    expect(rows).toEqual(WORK_RELAY_W4.map((r) => [...r]));
  });

  // Kills: the scoped row relayed onto H's whole feed.
  it("scopedChanges goes to /v1/hub/work/changes with a scope", async () => {
    const send = vi.fn(async () => ({ events: [], next: "0", more: false, truncated: false }));
    await makeWorkClient(send).scopedChanges({ kind: "feature", id: "ft-1" });
    expect(send).toHaveBeenCalledWith("GET", "/v1/hub/work/changes?scope=feature%3Aft-1", undefined);
  });
});
```

#### Tests to write first — `desktop/src/work/replica/kinds.test.ts` and `roadmap.test.ts`

1. **Copied by name** — reads `control-plane/internal/hub/hubclient/fake_work_roadmap.go` through
   `import.meta.glob(…, { query: "?raw" })`, collects every `Kind: "…"` literal, and requires the
   set to equal `W4_FEED_KINDS ∪ W4_NOT_HELD`. **Kills** "a kind retyped wrong on the desktop".
2. **Revocation** — with task 57's fake Hub (`access.test.ts`'s), the caller's team `tm-pay` holds
   a grant on `EP`; the replica holds `EP-1`, feature `ft-ep` and its milestone. A `grant.revoked`
   event with `data.teamId = "tm-pay"` leaves no `EP` record in any map after **one** `transact`
   (a spy counts one), and `accessLost("pj-ep")` is set. **Kills** "revocation ignored on the
   desktop".
3. **Another team's grant** — the same event with `data.teamId = "tm-other"` drops nothing.
   **Kills** "any revocation drops the project".
4. **Unknown kind** — a page carrying `{kind: "initiative.archived_v9", …}` applies the rest of the
   page, throws nothing, and leaves the maps unchanged. **Kills** "a new Hub kind breaks old
   desktops".
5. **Signals are not stored** — `update.reminder` reaches a `signals` listener and appears in no
   `RoadmapRecords` map. **Kills** "a reminder kept as a record".
6. **No persistence** — task 55's no-persistence grep runs over `kinds.ts`, `replica.ts`,
   `ops.ts`, `visibility.ts` and `work.ts` and passes; then a test double that calls
   `localStorage.setItem("cursor", …)` inside `readCursor` makes the same check fail (the check
   spies on `Storage.prototype.setItem` during a full load plus one `readCursor` and one
   `queryInsights`). **Kills** "cursor kept on disk" (ADR-H1).
7. **No update edit** — `RoadmapOp["kind"]` values starting `update.` are exactly `["update.post"]`
   (a type-level list checked at runtime through `ROADMAP_OP_KINDS`), and `inverseOf` of an
   `update.post` returns the refusal. **Kills** "updates editable on the client".

#### Steps

1. Append the tests above; run them; they fail (no `WORK_RELAY_W4`, no `kinds.ts`).
2. Append parts 9-12 to `work.ts` and the four codes to `client.ts`; the wire tests pass once task
   114's goldens and task 124's Go table are on the tree (before that, they fail by design).
3. Write `kinds.ts`; test 1 passes once task 115's fake is on the tree.
4. Add the `replica.ts`, `ops.ts` and `visibility.ts` deltas; tests 2-7 pass.
5. `npm --prefix desktop run typecheck`; commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- src/api/work.test.ts src/work/replica/kinds.test.ts src/work/replica/roadmap.test.ts
# expect: work.test.ts three more passed than before; kinds.test.ts 1 passed;
#         roadmap.test.ts 6 passed; no failed
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- A surface: tasks 126 to 141 draw these records.
- That the thirteen held kinds and four not-held kinds are the Hub's: the copy test makes them so
  when task 115 lands, and fails until then.
- That a granted project survives a desktop restart before the grant-list route exists (gap 1).

#### Gaps found while writing this part

1. **No route lists the projects granted to the caller's teams.** `GET /v1/projects` answers own
   teams' projects only (`hub-contract.md:347`), and task 117's `GET …/grants` is per project, for
   the owning team. A restarted desktop cannot find its granted projects. Task 117 needs a
   caller-scoped list (for example `GET /v1/me/project-grants`), and task 124's table a 32nd row.
2. **Task 114** must carry `StatusUpdate.erasedAt`, `MilestoneView.reached` and the golden file
   names; its scope lists neither field.
3. **Task 120** names no milestone delete, so `milestone.create` has no Undo.
4. **Task 55's** part is not written; `ReplicaDeps`, `listAll`, `transact` and the
   no-persistence grep are its scope's names.
