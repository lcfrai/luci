### Task 55: the replica, part 1: in memory, full load, the cursor on `/v1/changes`; the no-persistence test

Stream **WD1**, the read half of seam **SW.16**, about 3 hours (an estimate, not a measurement).
Depends on task 37 (SW.15: `WorkClient`, `IssueHead`, `IssueRevision`, `IssueComment`, `Label`,
`Page`, `workProblem`, `task-37.md:134-173`, `:318-361`, `:464-485`). Implements task 40's
`WorkReader` and installs it with `setWorkSource` (`task-40.md:140-176`); task 40's gap 2 asks for
exactly that. Tasks 56, 57 and 58 extend `replica.ts` (`task-56.md:312-387`, `task-57.md:167-214`,
`task-58.md:159-197`); their guesses at this part's names are aligned below, and where they differ
this part says so. Reads the relayed feed `GET /v1/hub/changes?after=` (`hub-contract.md:1372`)
and phase 4 task 16's `hubCursor` (`phase-4-discover-parts/task-16.md:77-79`, **UNVERIFIED until
merge**, rule H2; the skeleton's row 55 names only task 37 as a dependency).

Parity: closes **L-228** with task 56 and **L-229**'s W1 part with task 57; **L-230** stays a SKIP
(05 row 230, `05-design.md:407`: surviving a restart means Hub data on disk, which ADR-H1 forbids).
Design: 05 §5.3 (*The replica, reconciled with 02*), §4.4 (*One correction to 02*: in memory),
§3.1; `ADR-H1:467-474`; 02 C8 `hub.replica` and `sync.checkpoint` (`02:274-275`); hub-contract §7.11
(`hub-contract.md:1089-1107`) and §9.3 (`:1377-1385`).

**What the reader gets.** Nothing on screen of its own. After this task the Work lens, Plan, the
issue view and every `useWork` caller read real Hub records instead of task 20's fixture, a
teammate's change reaches the list within one poll of the control plane (5 s,
`PANACEA_APP_HUB_POLL_INTERVAL`, `hub-contract.md:1378`), and closing the app leaves no Work data
anywhere on the machine.

**Decisions made here, so nobody re-derives them:**

1. **The cursor is taken before the load, never after.** The first call is
   `changes("latest")`, which returns no events and the head as `next` (`hub-contract.md:1101`).
   Only then does the replica list every project's issues. Then it reads the feed **after that
   cursor**, so an event committed while the lists were loading is applied, not lost. Taking the
   cursor after the load would skip it; the first test kills that order.
2. **Apply by `seq`, once.** The replica keeps `lastSeq`. An event with `seq <= lastSeq` is dropped
   (a repeated page, a retried read). `more: true` means ask again at once (`:1099-1100`), in the
   same read, not on the next poll. A caller's feed is filtered, so gaps in `seq` are normal; this
   task never reads one as missing data (task 57's decision 4).
3. **An event names what changed; the Hub says what it is now.** Feed events carry metadata only
   (SW.5, `task-13.md:115-118`). Applying one **re-fetches its subject** in one batch per page, and
   the event's `data` is used only to route (which issue a comment belongs to), never as the
   record. An `issue.revised` event therefore costs one `GET /v1/hub/work/issues/{id}`, and the
   head revision in memory is the Hub's, digest and all.
4. **One read per change of the header.** After the first load the replica does not poll. The
   control plane already polls the Hub and stamps `Panacea-Hub-Cursor` on `GET /v1/sessions`
   (`:1377-1381`); phase 4's loader stores it as `hubCursor`. The replica subscribes to that value
   and makes **one** `changes(after=cursor)` per change. A change that arrives mid-read is
   coalesced: when the read ends, one more read runs if the value moved. Phase 4's loader reads
   the feed for `brief.*` on the same change with its own cursor; two reads per change is the
   cost of not sharing one, recorded as a gap.
5. **In memory, beside the store, never in it.** The replica is a module object, not a slice of
   `useCockpit`, so `partialize` (`store/store.ts:34-48`) can never reach it. Task 37's
   `workSlice` stays the non-persisted UI state. Nothing under `src/work/**` or the Work
   components touches `localStorage`, `sessionStorage`, IndexedDB, the Cache API or the Tauri fs
   and store plugins; the no-persistence test proves it both statically and at run time.
6. **`version` and one notify per `transact`.** Every change to the maps happens inside
   `transact(fn)`, which bumps `version` once and notifies `useWork`'s subscribers once, so a page
   of 40 events is one re-render, and task 56's batch paint is one too (`task-56.md:187-190`).
   `useWork` is task 40's (`source.ts:202-223`), re-exported from `replica.ts` as SW.16 names it;
   its `sameData` is what keeps a caller whose selection did not change from re-rendering.
7. **Comments and subscriptions load on demand.** The full load is heads, head revisions, labels,
   members and the caller's subscriptions. An issue's comments load when a surface calls
   `loadThread(issueId)` (task 51's thread), and from then on `comment.*` events for that issue
   re-fetch its thread. A comment event for an issue whose thread is not loaded changes nothing.
8. **Which projects.** This task's `readableProjects()` dep answers `{projectId, teamId, key}[]`,
   the shape of task 57's `ProjectScope` (`task-57.md:95`), so task 57's `scope` and `admit` wrap it
   without a second list. Before task 57, the answer is taken as given.

**Files** (the skeleton's file map lists `src/work/replica/{replica,…}.ts` for 55-58, `:304`; it
omits `hubDeps.ts` and `replicaMount.ts`, gaps named below):
- `desktop/src/work/replica/replica.ts` — the types, `createReplica`, the `WorkReader` over it, `useWork` re-exported (new)
- `desktop/src/work/replica/replica.test.ts` — load order, apply, the header, the re-fetch, renders (new)
- `desktop/src/work/replica/noPersistence.test.ts` — the static scan and the runtime spy (new)
- `desktop/src/work/replica/hubDeps.ts` — `hubReplicaDeps(work, hub)`: the deps over task 37's client and the relay (new)
- `desktop/src/work/replica/replicaMount.ts` — `startReplica()`: builds it, `setWorkSource`, stops on sign-out (new)
- `desktop/src/components/work/WorkLens.tsx` (task 40) — `WorkMount` calls `startReplica()` in place of its not-connected state (modified)

#### Interfaces — complete: `desktop/src/work/replica/replica.ts`, part 1: the types

```ts
// desktop/src/work/replica/replica.ts — task 55. SW.16's read half: an in-memory copy of the
// Hub's Work records the caller may read, kept current from /v1/changes. NOTHING here persists.
import type { IssueComment, IssueHead, IssueRevision, IssueWithRevision, Label, User } from "../../api/work";
import type { CriterionView, TeamMemberView, WorkReader } from "../source";
export { useWork } from "../source";                    // SW.16's hook, one implementation

/** hub-contract §7.11's Event (`hub-contract.md:1096-1097`). `data` routes, never records. */
export interface HubEvent {
  seq: number; at: string; kind: string;
  teamId: string | null; projectId: string | null; subject: string;
  actor: User | null; data: Record<string, unknown>;
}
export interface ChangesPage { events: HubEvent[]; next: string; more: boolean }

/** task 57's ProjectScope, the same three fields (`task-57.md:95`). */
export interface ReplicaProject { projectId: string; teamId: string; key: string }

/** One project's full load: every readable issue's head and head revision. */
export interface WorkSnapshot { heads: IssueHead[]; revisions: IssueRevision[]; comments: IssueComment[] }

/** One re-fetched subject. `comments` is present only when the thread was asked for. */
export interface WorkRecord { head: IssueHead; revision: IssueRevision; comments?: IssueComment[] }

/** Phase 4's thread reads (the criterion words and the brief a branch hangs from). */
export interface ThreadSource {
  briefKeyOf(projectId: string, branch: string | null): string | null;
  criteria(briefKey: string): readonly CriterionView[];
  subscribe(listener: () => void): () => void;
}

export interface ReplicaDeps {
  me(): Promise<User>;
  readableProjects(): Promise<ReplicaProject[]>;
  listAll(projectId: string): Promise<WorkSnapshot>;
  /** `after` is a cursor or "latest". Throws the client's ApiError on failure. */
  changes(after: string): Promise<ChangesPage>;
  /** Omits an id the Hub answers 403 or 404 (task 57's decision 2); throws only on transport. */
  fetchSubjects(ids: readonly string[], opts?: { threads?: readonly string[] }): Promise<WorkRecord[]>;
  labels(teamId: string): Promise<Label[]>;
  members(teamId: string): Promise<TeamMemberView[]>;
  /** Issue ids the caller is subscribed to. */
  subscriptions(): Promise<string[]>;
  /** Phase 4's hubCursor: the latest Panacea-Hub-Cursor seen (decision 4). */
  cursorHint: { current(): string | null; subscribe(listener: () => void): () => void };
  thread: ThreadSource;
}

export type ReplicaStatus = "empty" | "loading" | "ready" | "resyncing";

export interface Replica extends WorkReader {
  readonly status: ReplicaStatus;
  /** The cursor after the last applied page; null before the first. */
  readonly cursor: string | null;
  get(issueId: string): IssueWithRevision | undefined;
  /** Loads the issue's comments once; afterwards comment events keep them current. */
  loadThread(issueId: string): Promise<void>;
  comments(issueId: string): readonly IssueComment[];
  subscribed(issueId: string): boolean;
  /** Stops listening to the hint and drops every record (sign-out). */
  stop(): void;
}
```

`get` and task 40's `issue` answer the same record; `get` is the scope's name, kept because tasks
57 and 58 wrote against it. `WorkSnapshot.comments` is always `[]` from `hubDeps` in W1
(decision 7); it exists because task 56's harness passes it (`task-56.md:534-535`). Task 56's
harness also passes `cursor: "c-0"` inside `listAll`'s answer, which this type does not carry: the
cursor comes from `changes("latest")` (decision 1). That field must go from task 56's harness, or
the excess-property check fails its typecheck (a gap).

#### Interfaces — complete: `replica.ts`, part 2: what an event asks for

```ts
/** What one page of events asks the replica to re-read. Pure, so the routing is testable alone. */
export interface Refetch {
  issues: Set<string>;          // re-fetch head and head revision
  threads: Set<string>;         // and the thread, for issues whose thread is loaded
  labelsOf: Set<string>;        // team ids
  projects: Set<string>;        // re-list (a project key changed: every alias moved)
  subscriptions: boolean;
}

export const emptyRefetch = (): Refetch =>
  ({ issues: new Set(), threads: new Set(), labelsOf: new Set(), projects: new Set(), subscriptions: false });

/** `threadOfComment` answers the issue a HELD comment belongs to; `threadLoaded` says whether an
 *  issue's thread is held. Kinds are task 13's W1 list (`task-13.md:121-140`). */
export function route(e: HubEvent, into: Refetch, threadOfComment: (id: string) => string | null,
  threadLoaded: (issueId: string) => boolean): void {
  const issueIdIn = (k: string): string | null => (typeof e.data[k] === "string" ? (e.data[k] as string) : null);
  if (e.kind.startsWith("issue.")) {
    into.issues.add(e.subject);
    const other = issueIdIn("otherIssueId");               // issue.related and issue.unrelated
    if (other !== null) into.issues.add(other);
    return;
  }
  switch (e.kind) {
    case "comment.created": case "comment.mentioned": {
      const issue = issueIdIn("issueId") ?? e.subject;      // task 20's fixture puts it in subject
      if (threadLoaded(issue)) { into.issues.add(issue); into.threads.add(issue); }
      return;
    }
    case "comment.reacted": case "comment.resolved": case "comment.erased": {
      const issue = threadOfComment(e.subject);
      if (issue !== null) { into.issues.add(issue); into.threads.add(issue); }
      return;
    }
    case "label.created": case "label.archived":
      if (e.teamId !== null) into.labelsOf.add(e.teamId);
      return;
    case "project.key_set":
      if (e.projectId !== null) into.projects.add(e.projectId);
      return;
    case "subscription.set":
      into.subscriptions = true;
      return;
    default:
      return;       // brief.*, release.*, design.*: phase 4's loader; team.member_*: task 57
  }
}
```

`issue.*` routes by `subject`, which task 13's `emitWork` sets to the issue id for every issue
kind (`task-13.md:332-343`). `comment.created` and `comment.mentioned` carry `issueId` in their
data (`task-13.md:189-198`), while task 20's fixture writes `comment.created` with the issue as its
`subject` and no `issueId` (`task-20.md:332`); the route reads `issueId` first and the subject
second, and acts only when that id is a loaded thread, so a user id in `subject` (the
`comment.mentioned` fixture, `:333`) matches nothing. The sources disagree, a gap. The other three comment kinds do not, so they route through the held
comment's own `issueId`, and a comment the replica never held changes nothing (decision 7).
`issue.related` and `issue.unrelated` are emitted **once**, on the subject, with the other issue
in `otherIssueId` (`task-11.md:329-332`). Both heads list the relation (each end reads its own
word, `task-48.md:206-211`), so both are re-fetched; the id routes, the heads come from the Hub.

#### Interfaces — complete: `replica.ts`, part 3: the maps and `transact`

```ts
interface Maps {
  heads: Map<string, IssueHead>;
  revisions: Map<string, IssueRevision>;         // the head revision only; task 47 reads older ones itself
  byProject: Map<string, Set<string>>;
  threads: Map<string, IssueComment[]>;          // present = loaded (decision 7)
  commentIssue: Map<string, string>;             // comment id -> issue id, for routing
  labels: Map<string, Label[]>;
  members: Map<string, TeamMemberView[]>;
  projects: Map<string, ReplicaProject>;
  subscribed: Set<string>;
}

/** Task 56's ReplicaWriter (`task-56.md:191-198`) is this, and task 55's apply uses it too. */
export interface ReplicaWriter {
  putHead(h: IssueHead): void;
  putRevision(r: IssueRevision): void;
  putComment(c: IssueComment): void;
  dropComment(issueId: string, commentId: string): void;
  setSubscribed(issueId: string, on: boolean): void;
  /** Task 55's own: replaces a loaded thread wholesale, and drops an issue entirely. */
  putThread(issueId: string, comments: IssueComment[]): void;
  dropIssue(issueId: string): void;
}
```

`putHead` files the head under `byProject` by its `projectId`, moving it if the project changed.
`putRevision` keeps a revision only when its `revision` is at least the held one's, so a slow
answer never rolls the head revision back. `transact(fn)` runs `fn(writer)`, then, if any writer
call changed a map, bumps `version` and notifies every subscriber **once**. A `transact` that
changes nothing notifies nobody, which is what keeps a repeated event from re-rendering anyone.

#### Reference implementation: the load and the feed, inside `createReplica` (the tests are the contract)

```ts
export function createReplica(deps: ReplicaDeps): Replica {
  const m: Maps = emptyMaps();
  let status: ReplicaStatus = "empty", cursor: string | null = null, lastSeq = 0;
  let me: User | null = null, stopped = false, reading = false, again = false;
  let hintSeen: string | null = null;
  const { transact, subscribe, version } = makeStore(m);           // decision 6

  async function applyRefetch(r: Refetch): Promise<void> {
    const [records, labels, listed, subs] = await Promise.all([
      r.issues.size ? deps.fetchSubjects([...r.issues], { threads: [...r.threads] }) : [],
      Promise.all([...r.labelsOf].map(async (t) => [t, await deps.labels(t)] as const)),
      Promise.all([...r.projects].map(async (p) => [p, await deps.listAll(p)] as const)),
      r.subscriptions ? deps.subscriptions() : null,
    ]);
    if (stopped) return;
    transact((w) => {
      for (const rec of records) {
        w.putHead(rec.head); w.putRevision(rec.revision);
        if (rec.comments) w.putThread(rec.head.id, rec.comments);
      }
      for (const [t, ls] of labels) m.labels.set(t, ls);
      for (const [, snap] of listed) putSnapshot(w, snap);
      if (subs) { for (const id of [...m.subscribed]) w.setSubscribed(id, false); for (const id of subs) w.setSubscribed(id, true); }
    });
  }

  /** Decisions 2 and 4: read until more is false; coalesce a read asked for mid-read. */
  async function readFeed(): Promise<void> {
    if (reading) { again = true; return; }
    reading = true;
    try {
      do {
        again = false;
        let more = true;
        while (more && !stopped) {
          const page = await deps.changes(cursor!);
          const want = emptyRefetch();
          for (const e of page.events) {
            if (e.seq <= lastSeq) continue;                                  // decision 2
            route(e, want, (c) => m.commentIssue.get(c) ?? null, (i) => m.threads.has(i));
            lastSeq = e.seq;
          }
          await applyRefetch(want);
          cursor = page.next;
          more = page.more;                                                  // at once, same read
        }
      } while (again && !stopped);
    } finally { reading = false; }
  }

  async function start(): Promise<void> {
    status = "loading";
    cursor = (await deps.changes("latest")).next;                            // decision 1: BEFORE
    me = await deps.me();
    const projects = await deps.readableProjects();
    const teams = [...new Set(projects.map((p) => p.teamId))];
    const [snaps, labels, members, subs] = await Promise.all([
      Promise.all(projects.map((p) => deps.listAll(p.projectId))),
      Promise.all(teams.map(async (t) => [t, await deps.labels(t)] as const)),
      Promise.all(teams.map(async (t) => [t, await deps.members(t)] as const)),
      deps.subscriptions(),
    ]);
    if (stopped) return;
    transact((w) => {
      for (const p of projects) m.projects.set(p.projectId, p);
      for (const s of snaps) putSnapshot(w, s);
      for (const [t, ls] of labels) m.labels.set(t, ls);
      for (const [t, ms] of members) m.members.set(t, ms);
      for (const id of subs) w.setSubscribed(id, true);
    });
    await readFeed();                                                        // the events of the load
    status = "ready";
    hintSeen = deps.cursorHint.current();
    offHint = deps.cursorHint.subscribe(() => {                              // decision 4
      const h = deps.cursorHint.current();
      if (h === hintSeen) return;
      hintSeen = h;
      void readFeed();
    });
  }
  let offHint = (): void => {};
  void start().catch(onLoadFailure);
  return makeReader(m, { status: () => status, cursor: () => cursor, me: () => me, version,
    subscribe, transact, deps, stop: () => { stopped = true; offHint(); transact((w) => clearAll(w, m)); } });
}
```

The private helpers, by what they must do rather than how:

- **`makeStore(m)`** returns `transact`, `subscribe` and a `version` getter. The writer it hands
  to `fn` sets a `changed` flag on any call that alters a map; direct `m.labels.set`,
  `m.members.set` and `m.projects.set` inside a `transact` call the same private `touch()`. Every
  `status` change is made inside a `transact` that touches, so a component reading `status`
  re-renders exactly once when it moves.
- **`putSnapshot(w, snap)`** puts every head and revision of one project. When a project is
  re-listed (`project.key_set`), heads of that project missing from the new snapshot are dropped
  with `dropIssue`, so a re-list converges rather than accumulates.
- **`onLoadFailure(e)`** keeps `workProblem(e)` (`task-37.md:466-485`) as `loadProblem`, sets
  `status` back to `"empty"`, and arms one retry of `start()` on the next change of the cursor
  hint. A load that fails is therefore retried at the control plane's poll rate, never in a
  tight loop, and with no timer of its own. `WorkMount` shows task 40's not-connected line with
  `loadProblem`'s own words while `status` is `"empty"`.
- **`deps.thread.subscribe`** is forwarded: a change in phase 4's criterion readings runs one
  empty `transact` that touches, so `version` moves and the criterion words re-render.

#### Interfaces — complete: `replica.ts`, part 4: the `WorkReader` it is

```ts
function makeReader(m: Maps, x: ReaderInternals): Replica {
  const withRev = (id: string): IssueWithRevision | undefined => {
    const head = m.heads.get(id), revision = m.revisions.get(id);
    return head && revision ? { head, revision } : undefined;
  };
  return {
    get version() { return x.version(); },
    get status() { return x.status(); },
    get cursor() { return x.cursor(); },
    subscribe: x.subscribe,
    me: () => x.me(),
    issue: withRev,
    get: withRev,
    byProject: (p) => [...(m.byProject.get(p) ?? [])].flatMap((id) => withRev(id) ?? []),
    teamOfProject: (p) => m.projects.get(p)?.teamId ?? null,
    labels: (t) => m.labels.get(t) ?? [],
    members: (t) => m.members.get(t) ?? [],
    briefKeyOf: (p, branch) => x.deps.thread.briefKeyOf(p, branch),
    criteria: (bk) => x.deps.thread.criteria(bk),
    criterion: (bk, cid) => x.deps.thread.criteria(bk).find((c) => c.id === cid),
    comments: (id) => m.threads.get(id) ?? [],
    subscribed: (id) => m.subscribed.has(id),
    loadThread: async (id) => {
      if (m.threads.has(id)) return;
      const [rec] = await x.deps.fetchSubjects([id], { threads: [id] });
      if (rec) x.transact((w) => { w.putHead(rec.head); w.putRevision(rec.revision); w.putThread(id, rec.comments ?? []); });
    },
    stop: x.stop,
  };
}
```

`byProject` returns a fresh array each call, which `useWork`'s `sameData` compares by content
(`task-40.md:178-199`), so a caller re-renders only when a record in its selection changed. The
arrays inside a record are the held objects, never copies: a record that did not change is the
same reference, which is what makes `sameData`'s `Object.is` first test cheap.
`ReaderInternals` is the private bag `createReplica` passes (`status`, `cursor`, `me`, `version`,
`subscribe`, `transact`, `deps`, `stop`).

#### Interfaces — complete: `desktop/src/work/replica/hubDeps.ts`

```ts
// hubDeps.ts — task 55. ReplicaDeps over task 37's WorkClient and the relay. The one file that
// knows the wire; replica.ts knows only ReplicaDeps.
import { ApiError } from "../../api/client";
import type { WorkClient } from "../../api/work";
import type { ChangesPage, ReplicaDeps } from "./replica";

export interface HubReads {
  get(route: string): Promise<unknown>;               // the client's GET, ApiError on failure
}

/** Every page of a list, in order. 200 is the Hub's maximum (`hub-contract.md:48`). */
export async function allPages<T>(page: (after?: string) => Promise<{ items: T[]; next: string | null }>): Promise<T[]> {
  const out: T[] = [];
  let after: string | undefined;
  for (;;) {
    const p = await page(after);
    out.push(...p.items);
    if (p.next === null) return out;
    after = p.next;
  }
}

/** At most `n` requests in flight; the answers keep their input order. */
export async function inBatches<A, R>(xs: readonly A[], n: number, f: (a: A) => Promise<R>): Promise<R[]> {
  const out: R[] = new Array(xs.length);
  let i = 0;
  await Promise.all(Array.from({ length: Math.min(n, xs.length) }, async () => {
    while (i < xs.length) { const k = i++; out[k] = await f(xs[k]); }
  }));
  return out;
}
```

```ts
// hubDeps.ts, continued.
const REVISION_READS_IN_FLIGHT = 6;                   // a proposal, recorded not measured

export function hubReplicaDeps(work: WorkClient, hub: HubReads,
  extra: Pick<ReplicaDeps, "cursorHint" | "thread" | "me" | "readableProjects" | "members">): ReplicaDeps {
  const record = async (id: string, thread: boolean) => {
    const { head, revision } = await work.issue(id);
    const comments = thread ? await allPages((after) => work.comments(id, { after, limit: 200 })) : undefined;
    return { head, revision, ...(comments ? { comments } : {}) };
  };
  return {
    ...extra,
    listAll: async (projectId) => {
      const heads = await allPages((after) => work.issues(projectId, undefined, { after, limit: 200 }));
      const full = await inBatches(heads, REVISION_READS_IN_FLIGHT, (h) => work.issue(h.id));
      return { heads: full.map((r) => r.head), revisions: full.map((r) => r.revision), comments: [] };
    },
    changes: async (after) => (await hub.get(`/v1/hub/changes?after=${encodeURIComponent(after)}`)) as ChangesPage,
    fetchSubjects: async (ids, opts) => {
      const threads = new Set(opts?.threads ?? []);
      const got = await inBatches(ids, REVISION_READS_IN_FLIGHT, async (id) => {
        try { return await record(id, threads.has(id)); }
        catch (e) { if (isGone(e)) return null; throw e; }     // task 57's decision 2
      });
      return got.filter((r): r is NonNullable<typeof r> => r !== null);
    },
    labels: (teamId) => work.labels(teamId),
    subscriptions: async () => (await allPages((after) => work.subscriptions({ after, limit: 200 })))
      .filter((s) => s.subscribed).map((s) => s.issueId),
  };
}
```

`isGone(e)` is `e instanceof ApiError && (e.status === 403 || e.status === 404)`. **The full load
is one list per project plus one read per issue**, because `issues` answers heads only
(`task-37.md:321`) and no Work route answers heads with their revisions. For a project of 500
issues that is 3 list pages and 500 revision reads, 6 at a time. That is the cost 05 §4.4 marks
**UNVERIFIED** for `budget.return` (task 70 records it); a list route that includes the head
revision would remove it, and is named as a gap for the Hub. `Subscription`'s `subscribed` and
`issueId` fields are **UNVERIFIED** against task 37's type (`task-37.md:345-346` names the
methods, not the fields here). `me`, `readableProjects` and `members` come in through `extra`,
because their routes (`GET /v1/me`, `GET /v1/projects`, a team's members) are phase H's and task
16's relay rows, **UNVERIFIED** here; `replicaMount.ts` wires them.

#### Interfaces — complete: `desktop/src/work/replica/replicaMount.ts`

```ts
// replicaMount.ts — task 55. The one place a Replica is built, installed and torn down.
import { setWorkSource } from "../source";
import { createReplica, type Replica, type ReplicaDeps } from "./replica";

let live: Replica | null = null;

/** Idempotent: a second call while one runs returns it. WorkMount calls this on sign-in. */
export function startReplica(deps: () => ReplicaDeps): Replica {
  if (live === null) { live = createReplica(deps()); setWorkSource(live); }
  return live;
}

/** Sign-out: stop, drop every record, uninstall. Nothing is written anywhere on the way out. */
export function stopReplica(): void {
  live?.stop();
  live = null;
  setWorkSource(null);
}
```

`stopReplica` runs on the auth state leaving `signed_in` (the control plane's `/v1/auth/status`,
`hub-contract.md:1371`). The records are dropped in memory; closing the window drops the process,
and there is nothing on disk to delete (decision 5).

#### Tests to write first, part 1 — `desktop/src/work/replica/replica.test.ts`

The harness is a fake Hub the test drives: `hub.feed` is an array of events with `seq`, and
`changes(after)` answers the events above `after` (or none and the head for `"latest"`), at most
`hub.pageSize` per page with `more` set when some remain; `hub.heads` is the current records;
`hub.hint` is a tiny store with `set(v)`. Every dep is a `vi.fn`, so the test counts calls.

```ts
function fakeHub() {
  const feed: HubEvent[] = [];
  let heads = [makeIssueHead({ id: EP142, headRevision: 1 })];
  const head = () => String(feed.at(-1)?.seq ?? 0);
  let hintValue: string | null = null;
  const hintListeners = new Set<() => void>();
  const hub = {
    feed, pageSize: 200, heads: () => heads, setHeads: (h: IssueHead[]) => { heads = h; },
    hint: { set(v: string) { hintValue = v; for (const l of hintListeners) l(); } },
    beforeListAll: async () => {},
  };
  const deps: ReplicaDeps = {
    me: vi.fn(async () => DYLAN),
    readableProjects: vi.fn(async () => [{ projectId: WORK_IDS.project, teamId: WORK_IDS.team, key: "EP" }]),
    listAll: vi.fn(async () => { await hub.beforeListAll();
      return { heads: heads.slice(), revisions: heads.map(revOf), comments: [] }; }),
    changes: vi.fn(async (after: string) => {
      if (after === "latest") return { events: [], next: head(), more: false };
      const rest = feed.filter((e) => e.seq > Number(after));
      const events = rest.slice(0, hub.pageSize);
      return { events, next: String(events.at(-1)?.seq ?? after), more: rest.length > events.length };
    }),
    fetchSubjects: vi.fn(async (ids: readonly string[]) => heads.filter((h) => ids.includes(h.id))
      .map((h) => ({ head: h, revision: revOf(h) }))),
    labels: vi.fn(async () => []), members: vi.fn(async () => []), subscriptions: vi.fn(async () => []),
    cursorHint: { current: () => hintValue,
      subscribe: (l) => { hintListeners.add(l); return () => { hintListeners.delete(l); }; } },
    thread: { briefKeyOf: () => null, criteria: () => [], subscribe: () => () => {} },
  };
  return { hub, deps };
}
/** The Hub's head revision for a head: its number is what a test reads to see which copy it holds. */
const revOf = (h: IssueHead) => makeIssueRevision({ issueId: h.id, revision: h.headRevision });
const ev = (seq: number, kind: string, subject: string, data = {}) => makeWorkEvent({ seq, kind, subject, data });
```

`makeIssueHead`, `makeIssueRevision`, `makeWorkEvent`, `DYLAN` and `WORK_IDS` are task 20's
(`task-20.md:217-285`, in `test/workFixtures`); task 20's `WorkEvent` is assignable to `HubEvent`
(its `data` is the narrower scalar record, `task-20.md:203-206`). A head's `headRevision` is the
number a test bumps on the Hub side, so the revision the replica holds says which copy it has.

```ts
const bump = (hub: ReturnType<typeof fakeHub>["hub"], n: number) =>
  hub.setHeads(hub.heads().map((h) => ({ ...h, headRevision: n })));

it("an event committed during the load is applied", async () => {
  const { hub, deps } = fakeHub();
  hub.beforeListAll = async () => { bump(hub, 2); hub.feed.push(ev(1, "issue.revised", EP142)); };
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  expect(r.get(EP142)?.revision.revision).toBe(2);
  expect(vi.mocked(deps.changes).mock.calls[0]).toEqual(["latest"]);
});
// kills: loading first and then asking after=latest, which skips seq 1.

it("a repeated event changes nothing and notifies nobody", async () => {
  const { hub, deps } = fakeHub();
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  hub.feed.push(ev(1, "issue.revised", EP142));
  hub.hint.set("1");
  await vi.waitFor(() => expect(r.cursor).toBe("1"));
  const seen = vi.fn(); r.subscribe(seen);
  const v = r.version;
  (deps.changes as Mock).mockResolvedValueOnce({ events: [ev(1, "issue.revised", EP142)], next: "1", more: false });
  hub.hint.set("1b");
  await vi.waitFor(() => expect(deps.changes).toHaveBeenCalledTimes(4));
  expect([r.version, seen.mock.calls.length, vi.mocked(deps.fetchSubjects).mock.calls.length]).toEqual([v, 0, 1]);
});
// kills: double-apply (a second fetch, a second notify for the same seq).

it("more: true reads again in the same turn", async () => {
  const { hub, deps } = fakeHub();
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  hub.pageSize = 1;
  hub.feed.push(ev(1, "issue.state", EP142), ev(2, "issue.state", EP142), ev(3, "issue.state", EP142));
  const before = vi.mocked(deps.changes).mock.calls.length;
  hub.hint.set("3");
  await vi.waitFor(() => expect(r.cursor).toBe("3"));
  expect(vi.mocked(deps.changes).mock.calls.length - before).toBe(3);
});
// kills: waiting for the next hint (a poll tick) when more is true.

it("an unchanged hint reads nothing; a changed one reads exactly once", async () => {
  const { hub, deps } = fakeHub();
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  const n = () => vi.mocked(deps.changes).mock.calls.length;
  const base = n();
  hub.hint.set("0"); hub.hint.set("0");
  await Promise.resolve();
  expect(n() - base).toBe(1);
  hub.hint.set("0");                                 // the same value again: nothing
  await Promise.resolve();
  expect(n() - base).toBe(1);
});
// kills: a feed read per tick, and a second read for one change.

it("applying issue.revised re-fetches the subject and never uses the event's data", async () => {
  const { hub, deps } = fakeHub();
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  bump(hub, 4);
  hub.feed.push(ev(1, "issue.revised", EP142, { revision: 9, key: `issue:${EP142}@9` }));
  hub.hint.set("1");
  await vi.waitFor(() => expect(r.get(EP142)?.revision.revision).toBe(4));
  expect(deps.fetchSubjects).toHaveBeenCalledWith([EP142], { threads: [] });
});
// kills: using event metadata as content (revision 9 would be held).
```

The first `hub.hint.set("0")` counts as a change because the hint starts at `null`; the second
identical `set` is not one. `Mock` is vitest's type. In the repeat case the fourth `changes` call
is the one the test stubs; its page repeats seq 1, which the replica drops.

```tsx
it("useWork re-renders a component only when its selection changes", async () => {
  const { hub, deps } = fakeHub();
  hub.setHeads([makeIssueHead({ id: EP142, headRevision: 1 }), makeIssueHead({ id: WORK_IDS.ep150, headRevision: 1 })]);
  const r = createReplica(deps);
  setWorkSource(r);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  let renders = 0;
  function Probe() { renders += 1; const n = useWork((x) => x.issue(EP142)?.revision.revision); return <i>{n}</i>; }
  render(<Probe />);
  const base = renders;
  hub.setHeads(hub.heads().map((h) => (h.id === WORK_IDS.ep150 ? { ...h, headRevision: 2 } : h)));
  hub.feed.push(ev(1, "issue.revised", WORK_IDS.ep150));
  hub.hint.set("1");
  await vi.waitFor(() => expect(r.get(WORK_IDS.ep150)?.revision.revision).toBe(2));
  expect(renders).toBe(base);                                   // EP-150 moved; the probe reads EP-142
  bump(hub, 3);
  hub.feed.push(ev(2, "issue.revised", EP142));
  hub.hint.set("2");
  await screen.findByText("3");
  expect(renders).toBe(base + 1);
  setWorkSource(null);
});
// kills: whole-tree renders (a subscriber notified per transact re-rendering on every change).
```

`bump` sets every head's number, so the second step moves EP-150 too; only EP-142's change is
in the probe's selection, and it re-renders once. `setWorkSource` is task 40's (`source.ts:163`).

#### Tests to write first, part 2 — `desktop/src/work/replica/noPersistence.test.ts`

```ts
// The static half: the source of every Work module, read as text through Vite's glob.
const SOURCES = {
  ...import.meta.glob("/src/work/**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }),
  ...import.meta.glob("/src/components/{work,issue,editor,desk}/**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }),
} as Record<string, string>;
const FORBIDDEN = /localStorage|sessionStorage|indexedDB|caches\.open|@tauri-apps\/plugin-fs|@tauri-apps\/plugin-store|\bwriteFile\b|\bwriteTextFile\b/;

it("no Work module names a storage API", () => {
  const files = Object.entries(SOURCES).filter(([p]) => !/\.test\.tsx?$/.test(p));
  expect(files.length).toBeGreaterThan(5);                       // the glob matched real files
  expect(files.filter(([, src]) => FORBIDDEN.test(src)).map(([p]) => p)).toEqual([]);
});
// kills: a disk cache added for a fast return (ADR-H1:467-474).

// The runtime half: a full load and 100 events, with every write to storage watched.
it("a full load and 100 events write no Work data anywhere", async () => {
  const setItem = vi.spyOn(Storage.prototype, "setItem");
  const idbOpen = vi.fn();
  vi.stubGlobal("indexedDB", { open: idbOpen });
  const { hub, deps } = fakeHub();
  const r = createReplica(deps);
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  for (let i = 1; i <= 100; i += 1) hub.feed.push(ev(i, i % 2 ? "issue.state" : "issue.revised", EP142));
  hub.hint.set("100");
  await vi.waitFor(() => expect(r.cursor).toBe("100"));
  useCockpit.getState().setLens("work");                         // a persisted write, on purpose
  expect(idbOpen).not.toHaveBeenCalled();
  expect(new Set(setItem.mock.calls.map(([k]) => k))).toEqual(new Set([PERSIST_KEY]));
  for (const [, value] of setItem.mock.calls) expect(value).not.toContain(EP142);
  expect(Object.keys(partialize(useCockpit.getState())).sort()).toEqual([...PERSISTED_KEYS].sort());
  vi.unstubAllGlobals();
});
// kills: a replica that writes through the store, a Work key added to partialize, and IndexedDB.
```

`PERSIST_KEY` is `store.ts`'s export (`:76`); `PERSISTED_KEYS` is `store.test.ts`'s list
(`:410-417`), moved to a shared `test/persistedKeys.ts` so both tests read one list, which is
the pattern `store.test.ts:335` relies on. The `setLens` line makes the store persist once, so
the key-set assertion is not vacuous. The glob's `query: "?raw"` form is Vite 5's (**UNVERIFIED**
against the repo's Vite version; the older `as: "raw"` form is the fallback).

#### Interfaces — complete: the delta on `WorkMount` (task 40's `WorkLens.tsx:585-592`)

```tsx
// WorkLens.tsx — task 55's edit. The not-connected line now names why; the replica starts on sign-in.
import { api } from "../../api";
import { createHubFeedClient } from "../../api/hubFeed";          // phase 4 task 16 (UNVERIFIED path)
import { hubReplicaDeps, type HubReads } from "../../work/replica/hubDeps";
import { startReplica, stopReplica } from "../../work/replica/replicaMount";

function replicaDeps(): ReplicaDeps {
  const feed = createHubFeedClient();
  // HubReads is one GET; phase 4's client already owns the feed route, so the adapter reads `after`.
  const hub: HubReads = { get: (route) => feed.changes(new URL(route, "http://x").searchParams.get("after") ?? "latest") };
  return hubReplicaDeps(api.work, hub, {
    cursorHint: hubCursorHint(),                                   // phase 4's hubCursor, below
    thread: phase4ThreadSource(),                                  // phase 4's brief readings (UNVERIFIED names)
    me: () => api.me(),
    readableProjects: () => api.readableProjects(),
    members: (teamId) => api.teamMembers(teamId),
  });
}

export function WorkMount(): JSX.Element {
  const signedIn = useCockpit((s) => s.auth.status === "signed_in");  // UNVERIFIED slice name
  useEffect(() => {
    if (!signedIn) return;
    startReplica(replicaDeps);
    return stopReplica;
  }, [signedIn]);
  const live = useSyncExternalStore(onWorkSourceChange, () => workSource() as Replica | null);
  const problem = useSyncExternalStore(live?.subscribe ?? noSubscribe, () => live?.loadProblem ?? null);
  const projectId = useCockpit((s) => s.work.projectId);
  const branch = useCockpit((s) => s.work.branch);
  if (live === null || problem !== null)
    return <p className="wk-empty">{problem?.message ?? "Work reads your team's issues from the Hub. This app is not connected to it yet."}</p>;
  if (projectId === null || branch === null) return <p className="wk-empty">Open a feature to see its work.</p>;
  return <WorkLens projectId={projectId} branch={branch} />;
}
```

`hubCursorHint()` is four lines over the store: `current` reads phase 4's `hubCursor`
(`phase-4-discover-parts/task-16.md:79`), and `subscribe` is `useCockpit.subscribe` filtered to a
change of that one value. Phase 4 writes `hubCursor` from every response's `Panacea-Hub-Cursor`
header, so the replica needs no timer. `api.me`, `api.readableProjects` and `api.teamMembers` are
the phase H relay reads this part names but no merged `client.ts` has (`desktop/src/api/client.ts`
exports none of the three today, a grep at this commit); their spelling is the merged client's.
`problem.message` is task 37's `WorkProblem` field (`task-37.md:467-472`, every kind has one).
`noSubscribe` is `() => () => {}`. **`Replica` gains one member here** that part 1's interface
(`:127-139`) omits though decision text at `:345-349` uses it:
`readonly loadProblem: WorkProblem | null;`, null except after a failed load, cleared inside the
`transact` of the next good one. Task 40's fixture source is not a `Replica`, so `loadProblem` reads
`undefined ?? null` there and task 40's suite passes unchanged.

#### Steps

- [ ] **Step 1: read on the sync base** task 37's merged `api/work.ts` and `client.ts`, task 40's
      `source.ts` and `WorkLens.tsx`, phase 4 task 16's merged slice (`hubCursor`) and its
      `createHubFeedClient`; correct any name above that the merged files spell differently (theirs win).
- [ ] **Step 2: write `replica.test.ts` and `noPersistence.test.ts`**; run them and see each fail
      for its stated reason (the static scan passes vacuously only if the glob matches nothing,
      which its first assertion forbids).
- [ ] **Step 3: write `replica.ts` part 1 and part 2** (the types, `route`, `emptyRefetch`); the
      file typechecks with `createReplica` a stub that throws.
- [ ] **Step 4: write part 3 and the load and feed** (`Maps`, `transact`, `start`, the coalesced
      read); the load-order, repeat, `more` and hint cases pass.
- [ ] **Step 5: write part 4, the reader**; the re-fetch and render cases pass.
- [ ] **Step 6: write `hubDeps.ts` and `replicaMount.ts`**, then the `WorkMount` delta; task 40's
      suite passes unchanged; `Replica` gains `loadProblem`.
- [ ] **Step 7: move `PERSISTED_KEYS`** from `store.test.ts:410-417` to `test/persistedKeys.ts`,
      import it in both tests; the store suite passes unchanged.
- [ ] **Step 8: commit** with `git add` on the new files and `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/replica'
# expected: 2 files, 8 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/work src/store'
# expected: every suite passing (task 40's lens and the store's persistence cases unchanged)
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'git grep -nE "setInterval|setTimeout" -- desktop/src/work/replica'
# expected: no output (the replica reads on the header's change, never on a timer of its own)
~/.claude/tools/claudezsh/claudezsh 'git grep -nE "localStorage|sessionStorage|indexedDB" -- desktop/src/work desktop/src/components/work ":!*.test.*"'
# expected: no output (the static half of the no-persistence test, by hand)
```

The count: 6 in `replica.test.ts` (the load order, the repeat, `more`, the hint, the re-fetch, the
renders) and 2 in `noPersistence.test.ts` (the static scan, the runtime spy): 8.

#### What this task does NOT claim

- Writes (task 56), scope and revocation (task 57), open by number and the head's own reads
  (task 58). This task reads; it never writes the Hub.
- That the full load is fast. 500 issues cost 500 revision reads, 6 in flight (the constant is a
  proposal); 05 §4.4's budget is **UNVERIFIED** and task 70 measures it.
- That a teammate's change arrives in 5 s. It arrives one control-plane poll after the Hub commits
  it, and the poll interval is the control plane's setting, not this task's.
- That the replica survives a restart (L-230 stays a SKIP, ADR-H1:467-474). A restart is a full
  load.
- That the wiring names in the `WorkMount` delta are right: `api.me`, `api.readableProjects`,
  `api.teamMembers`, the auth slice and phase 4's thread source are named for phase H and phase 4
  code that has not merged (rule H2).

#### Gaps found while writing this part

1. **File map omissions** (`:304`): `hubDeps.ts`, `replicaMount.ts`, `noPersistence.test.ts`,
   `test/persistedKeys.ts`, and the ⚑ edits to `WorkLens.tsx` and `store.test.ts`.
2. **Task 56's harness passes `cursor: "c-0"`** inside `listAll`'s answer (`task-56.md:534-535`),
   which `WorkSnapshot` does not carry (decision 1); task 56 drops it or its typecheck fails.
3. **The `comment.*` data disagree.** Task 13 puts `issueId` in `comment.created`'s data; task
   20's fixture puts the issue in `subject`. The route reads both; task 13 or task 20 should settle it.
4. **No list route answers heads with their revisions**, so the full load is N+1. A Hub delta
   (`GET /v1/work/projects/{id}/issues?include=revision`) removes it; no Hub task in the skeleton
   adds it.
5. **Two feed reads per header change** (decision 4): phase 4's loader and this replica each read
   `/v1/hub/changes` with their own cursor. One shared reader would halve it; not in any task.
6. **`HubReads` is a generic GET** while phase 4 ships `HubFeedClient.changes(after)`
   (`phase-4-discover-parts/task-16.md:272-273`); the `WorkMount` delta adapts one to the other.
   `hubDeps` taking `HubFeedClient` directly would remove the adapter.
7. **No auth state exists in `desktop/src` today** (a grep for `signed_in` finds none); phase H's
   sign-in (OD-6) owns it, and `WorkMount`'s start and stop follow whatever it names.
8. **The skeleton's row 55 names only task 37** as a dependency; this part also needs task 40
   (`setWorkSource`) and phase 4 task 16 (`hubCursor`, `createHubFeedClient`).
