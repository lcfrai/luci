### Task 15: Desktop — the Discover client, the wire types, the problem codes

The TypeScript mirror of tasks 11–13's JSON, and the two phase 2 edits that stop the desktop
sending a `contextRef` the server now ignores (contract §6.3). **Nothing is installed and
`desktop/vitest.config.ts` is not touched** (C12): the survey found that file carries
`setupFiles`, `clearMocks` and the `?raw` CSS include, and the old plan's task 10 would have
deleted all three.

**Files:**
- `desktop/src/api/discover.ts` — types and `createDiscoverClient` (new)
- `desktop/src/api/discover.test.ts` (new)
- `desktop/src/api/client.ts` — `createTransport` extracted; `ProblemCode` extended; `contextRef`
  no longer sent on resume and fork; `restartSession` takes options (modify, declared departure)
- `desktop/src/store/defaults.ts` — `toCreateRequest` no longer sets `contextRef` (modify)
- `desktop/src/api/index.ts` — exports `discoverApi = createDiscoverClient()` (modify)

**`client.ts` changes:**

1. **Extract the transport.** `createClient` (`client.ts:233`) closes over its request, timeout and
   problem machinery (`resolveBaseUrl` `:172`, `toProblem` `:190`, `parseJson` `:221`, and the
   `get`/`send` helpers the method table calls). Move them, unchanged, into an exported
   `createTransport(opts?: ClientOptions): Transport`; `createClient` calls it. Read `:172-300`
   before moving: this plan did not trace every helper's name.
2. **`ProblemCode` gains** (grouped by who emits them):

```ts
  /* phase 4 (control plane) */
  | "brief_not_written" | "project_not_shared" | "fanout_too_large" | "branch_not_found"
  /* the Hub's task C1 and ruling R4 (control plane) */
  | "signed_out" | "hub_not_configured" | "upstream_unavailable"
  /* the Hub, relayed unchanged (Hub contract §4.2) */
  | "revision_conflict" | "brief_exists" | "brief_archived" | "brief_not_found"
  | "repo_already_linked" | "forbidden" | "tenant_not_enrolled" | "rate_limited"
  | "idempotency_mismatch" | "key_unresolvable" | "digest_mismatch"
```

   `problems.ts`'s `classify` is **not** changed by this task: every new code is `server-said`
   (its sentence is written for a human) except `upstream_unavailable`, whose kind is R4's owner's
   to add. If R4 has not landed when this task does, `upstream_unavailable` stays `server-said`
   and task 18 renders the lens's own `unavailable` state from the code, not the kind.
3. **Stop sending `contextRef`** from `resumeSession` (`:359`) and `forkSession`; the request
   field stays in `CreateSessionRequest` for one release, marked `@deprecated — ignored by the
   server since phase 4 (contract P6)`. `CreateSessionRequest` gains `withoutBrief?: boolean`.
4. `restartSession(id, opts?: { withoutBrief?: boolean })` sends `{"withoutBrief": true}` only
   when asked, so phase 2's empty-body restart is byte-identical.

**`defaults.ts`:** `toCreateRequest` (`:110`, `:127`) drops the `contextRef` option and line.

**Wire types — complete source** (`desktop/src/api/discover.ts`; field for field with tasks 03,
06, 07, 09, 11, 12, 13 — any difference is a bug in one of them):

```ts
// desktop/src/api/discover.ts
// The TypeScript mirror of the control plane's /v1/discover JSON. Keys are OPAQUE strings here:
// nothing in the desktop splits one (contract P5, G7). There is no digest code in TypeScript.

import type { ProjectId, BranchId, WorktreeId } from "../store/types";
import { createTransport, type ClientOptions } from "./client";

export type Scope = "project" | "branch";
export type BriefStatus = "open" | "complete";
export type Source = "picked" | "typed" | "drafted";
export type Author = "person" | "model_drafted" | "model_accepted";
export type StepKind = "question" | "follow_up" | "review" | "complete";
export type ChangeKind =
  | "record" | "edit" | "skip" | "accept" | "add_follow_up" | "dismiss_follow_up"
  | "restore" | "reinherit" | "draft";
export type BindingState = "bound" | "unbound" | "no_remote" | "hub_off";
export type AssistOutcome =
  | "" | "proposed" | "model_off" | "model_failed" | "nothing_to_ask" | "already_asked" | "not_yet";
export type DraftOutcome = "proposed" | "model_off" | "model_failed";
export type Resolution = "resolved" | "missing" | "erased" | "invalid" | "none" | "unavailable";
export type CriterionState = "open" | "settled" | "contested";
export type Origin = "project" | "branch" | "override";
export type WorktreeBlockState =
  | "absent" | "pending" | "current" | "superseded" | "edited" | "stale" | "broken"
  | "committed"; // a TRACKED file holds a block; task 09's StateCommitted (review I3)

export interface Question {
  key: string; projectPrompt: string; branchPrompt: string; help: string;
  options: string[]; followUp: boolean; because: string;
}
export interface Criterion { id: string; text: string }
export interface Answer {
  key: string; text: string; skipped: boolean; source: Source | ""; author: Author;
  criteria: Criterion[]; draftedFrom: string[]; answeredAt: string;
}
export interface Inherit { key: string; digest: string }
export interface Change { kind: ChangeKind; key: string; restores: number }
export interface Step {
  kind: StepKind; ordinal: number; total: number; key: string; prompt: string;
  help: string; options: string[]; because: string; pending: string[];
}
export interface TeamChoice { id: string; name: string; role: string }
export interface Binding {
  state: BindingState; hubProjectId: string; hubProjectName: string;
  repoKeys: string[]; teams: TeamChoice[];
}
export interface BriefState {
  id: string; scope: Scope; headRevision: number; headKey: string; status: BriefStatus;
  latestCompleteRevision: number; latestCompleteKey: string;
  questions: Question[]; answers: Answer[]; inherits: Inherit | null; change: Change;
  savedAt: string; savedBy: string;
}
export interface InheritedState {
  key: string; briefId: string; revision: number; questions: Question[]; answers: Answer[];
}
export interface Preview { key: string; block: string }
export interface BriefView {
  scope: Scope; projectId: ProjectId; branchId: BranchId | "";
  binding: Binding; brief: BriefState | null; inherited: InheritedState | null;
  step: Step; preview: Preview | null;
}
export interface Op {
  kind: "record" | "edit" | "skip" | "accept" | "dismiss_follow_up" | "restore" | "propose_follow_up";
  key?: string; text?: string; source?: "picked" | "typed"; restoreRevision?: number;
}
export interface OpsResponse { view: BriefView; written: string[]; assist: AssistOutcome }
export interface HistoryRow {
  key: string; revision: number; status: string; change: Change;
  savedAt: string; savedBy: string; erased: boolean;
}
export interface History { rows: HistoryRow[]; next: string }

export interface Section {
  key: string; heading: string; body: string; items: Criterion[]; origin: Origin; replaced: string;
}
export interface RefCriterion {
  id: string; text: string; digest: string; state: CriterionState; because: string;
}
export interface InheritedRef { key: string; briefId: string; revision: number; digest: string }
/** provenance.Ref. Every field is always present (task 12's rule). */
export interface ProvenanceRef {
  key: string; resolution: Resolution; noneReason: "" | "declined"; present: boolean;
  briefId: string; revision: number; digest: string; scope: Scope | ""; projectId: string;
  branchRef: string; status: string; headRevision: number; latestCompleteRevision: number;
  inherits: InheritedRef | null; title: string; headline: string; audience: string;
  constraints: string; outOfScope: string; sections: Section[]; criteria: RefCriterion[];
  savedAt: string; savedBy: string; erasedAt: string;
}
export interface Head { headRevision: number; latestCompleteRevision: number; archived: boolean }
export interface RefsResponse { refs: ProvenanceRef[]; heads: Record<string, Head> }

export interface DiffLine { op: "same" | "added" | "removed"; text: string }
export interface WorktreeInspection {
  worktreeId: WorktreeId; path: string; state: WorktreeBlockState;
  markerKey: string; expectedKey: string; diff: DiffLine[];
}
export interface Materialisations { expectedKey: string; worktrees: WorktreeInspection[] }

export interface Findings {
  root: string; filesSeen: number; truncated: boolean;
  languages: { language: string; files: number }[];
  manifests: string[]; topLevel: string[]; testFiles: number; hasClaudeMd: boolean;
}
export interface AnalyseResponse {
  findings: Findings; draft: DraftOutcome; view: BriefView; written: string[]; note: string;
}

/** The target of a brief: a project, or a branch of it. */
export interface Target { projectId: ProjectId; branchId?: BranchId }

export interface DiscoverClient {
  binding(projectId: ProjectId): Promise<Binding>;
  bind(projectId: ProjectId, teamId: string): Promise<Binding>;
  view(target: Target): Promise<BriefView>;
  apply(target: Target, baseRevision: number, op: Op): Promise<OpsResponse>;
  history(briefId: string, after?: string): Promise<History>;
  refs(keys: readonly string[]): Promise<RefsResponse>;     // 1–100 keys; callers batch
  headRef(target: Target): Promise<ProvenanceRef>;
  materialisations(projectId: ProjectId, branchId: BranchId): Promise<Materialisations>;
  analyse(projectId: ProjectId): Promise<AnalyseResponse>;
}

export function createDiscoverClient(opts?: ClientOptions): DiscoverClient;
```

**Tests first** (`discover.test.ts`, a stubbed `fetch`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `paths` — one per method | each method hits the exact path of tasks 11–13 (branch targets nested under the project) | a flat `/branches/{id}` path the server does not route |
| `refs sends each key as its own parameter` | `?key=a&key=b`, URL-encoded; an empty key is sent as `key=` | a comma-joined list the server reads as one key |
| `refs refuses more than 100 keys before sending` | `ApiError("bad_request")`, no fetch | a request the server will 400 |
| `apply sends baseRevision and op only` | the body is exactly `{"baseRevision":5,"op":{…}}` | an extra field the server's strict decoder rejects |
| `a Hub conflict arrives as an ApiError with details` | a 409 `revision_conflict` body → `code`, `details.headRevision` | a client that drops `details` |
| `resume and fork no longer send contextRef` | the create body for both has no `contextRef` key | the desktop still asserting what the agent inherited (P6) |
| `restart sends no body unless asked` | `restartSession(id)` sends no body; with `{withoutBrief: true}` sends that | a phase 2 regression |

**Steps:**

- [ ] **Step 1:** Write the tests; `npx vitest run src/api/discover.test.ts` — expect failures.
- [ ] **Step 2:** Extract the transport; add the types, the client, the codes; edit
  `defaults.ts`; update the phase 2 tests that asserted `contextRef` was carried
  (`grep -rn "contextRef" desktop/src --include=*.test.ts*` — each is repointed to assert its
  absence, and the commit names every file touched).
- [ ] **Step 3:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all suites pass; tsc prints nothing)
$ git diff --stat HEAD -- desktop/src/store/types.ts desktop/vitest.config.ts desktop/package.json
(no output — G2, C12, nothing installed)
```

**Does NOT claim:** anything rendered; the loader (task 16).
