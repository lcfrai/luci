### Task 16: Desktop — two slices, the key sources, the one loader

Contract §6.4, P8 and ruling R6, as code. **Deletes the old plan's `useProvenance`** (a fetch per
component, `:5716`). In its place: a non-persisted `provenanceSlice`, a non-persisted
`discoverSlice`, the key-source rule, the provenance selectors, and **one loader** that runs on the
poller's good tick and issues **at most one** batched `GET /v1/discover/refs` — and zero on a
steady tick. **No timer is added** (G8): there are exactly two `setInterval`s in the non-test tree
and there stay two.

**Files:**
- `desktop/src/store/slices/provenanceSlice.ts`, `discoverSlice.ts` + tests (new)
- `desktop/src/provenance/loader.ts` + `loader.test.ts` (new)
- `desktop/src/provenance/useHeadRef.ts` + `useHeadRef.test.tsx` — the one reader of head readings
  into `headRefByTarget` (review I10) (new)
- `desktop/src/api/hubFeed.ts` — the Hub change feed's control-plane route (Hub C2) (new)
- `desktop/src/store/store.ts` — `CockpitState` gains both slices; `partialize` unchanged (modify)
- `desktop/src/store/selectors.ts` — the key sources and selectors below (modify)
- `desktop/src/App.tsx` — the good-tick callback also calls the loader (modify)
- `desktop/src/api/poller.ts` — **header comment only**, the UI contract §4.7 amendment (modify)
- `desktop/src/api/client.ts` — `ClientOptions.onHubCursor` (modify)

**Interfaces — complete source:**

```ts
// desktop/src/store/slices/provenanceSlice.ts
// NOT PERSISTED (contract A5, widened by ADR-H1 §5.4; ruling R3: phase 4 claims no persist
// version). A cache of control-plane truth: refs are immutable once filled; heads and
// criterion states move, and are kept beside the refs, never inside them (review I14).
import type { ProvenanceRef, Head, RefsResponse, CriterionReading } from "../../api/discover";

export interface ProvenanceSlice {
  /** Filled once per key, never overwritten except by an identical ref, and emptied of a whole
   *  brief by forgetBrief (a brief.erased event; review I10). Never holds a ref whose
   *  resolution is "unavailable" — those go to unavailableAt. */
  refsByKey: Record<string, ProvenanceRef>;
  /** The one moving number per brief (P4-B compares with latestCompleteRevision). */
  headsByBrief: Record<string, Head>;
  /** key -> phase 3's criterion states for it (S4-P3.4; review I14). MOVING, unlike refsByKey:
   *  putRefs OVERWRITES a key's entry whenever res.criterionStates carries it, and forgetBrief
   *  deletes the brief's entries. Never read from a ProvenanceRef, which holds no state. */
  criterionStatesByKey: Record<string, CriterionReading[]>;
  /** Keys whose criterion states must be re-read. Written only through markCriteriaStale,
   *  phase 3's seam: its evidence path calls it when a verdict on a key may have moved. Phase 4
   *  has no caller (every state is open), so it adds no request of its own. */
  staleCriteria: Record<string, true>;
  /** Head readings by targetKey ("<projectId>/<branchId or empty>"). */
  headRefByTarget: Record<string, ProvenanceRef>;
  /** targetKey -> the Hub project id it belongs to, learnt from setBriefView's
   *  binding.hubProjectId and from a head reading whose projectId is not "". A target absent here
   *  is matched by EVERY brief.* event: its Hub project is not known yet (review I10). */
  hubProjectByTarget: Record<string, string>;
  /** Head readings a brief.* event may have moved; useHeadRef re-reads each once (review I10). */
  staleHeadRefs: Record<string, true>;
  /** briefId -> the erasedAt of its brief.erased event. Erasure is of every revision and is
   *  irreversible (Hub contract §7.12 at hub 2f84662, :1106-1118; the event, §8 :1143), so
   *  this only grows. putRefs stores
   *  a "resolved" ref of such a brief in its erased form (no text; key, digest, savedAt, savedBy
   *  kept), because the control plane may answer from its cache until HA4-8 lands (task 07). */
  erasedBriefs: Record<string, string>;
  /** key -> the loader tick on which it last came back unavailable; retried after 15 ticks. */
  unavailableAt: Record<string, number>;
  /** Briefs whose head must be re-read: a feed event or a Discover write said it moved. */
  staleHeads: Record<string, true>;
  /** The latest Panacea-Hub-Cursor seen on any response, and the last one the feed was read at. */
  hubCursor: string | null;
  hubCursorRead: string | null;

  putRefs(res: RefsResponse, tick: number): void;
  putHeads(heads: Record<string, Head>): void;
  /** Also clears staleHeadRefs[targetKey] and, when ref.projectId !== "", sets
   *  hubProjectByTarget[targetKey]. */
  putHeadRef(targetKey: string, ref: ProvenanceRef): void;
  markStale(briefIds: readonly string[]): void;
  markHeadRefsStale(targetKeys: readonly string[]): void;
  /** S4-P3.4, phase 3's seam (review I14): the loader's next tick re-requests these keys, even
   *  though refsByKey holds them, and putRefs overwrites only their criterion states. */
  markCriteriaStale(keys: readonly string[]): void;
  /** brief.erased: deletes every refsByKey entry whose briefId is briefId (every key of the
   *  brief, whatever its revision) and the criterionStatesByKey entries of those keys, records
   *  erasedBriefs[briefId] = erasedAt, and marks the
   *  brief's head stale. The loader's next step 2 re-resolves the keys a surface still shows. */
  forgetBrief(briefId: string, erasedAt: string): void;
  setHubCursor(cursor: string): void;
  setHubCursorRead(cursor: string): void;
}

export const initialProvenanceState: Pick<ProvenanceSlice,
  "refsByKey" | "headsByBrief" | "criterionStatesByKey" | "staleCriteria" | "headRefByTarget"
  | "hubProjectByTarget" | "staleHeadRefs" | "erasedBriefs" | "unavailableAt" | "staleHeads"
  | "hubCursor" | "hubCursorRead">;
```

```ts
// desktop/src/store/slices/discoverSlice.ts — NOT PERSISTED
import type { BriefView } from "../../api/discover";

export type DiscoverLoad = "idle" | "loading" | "ready" | "error";

export interface DiscoverSlice {
  briefViews: Record<string, BriefView>;         // by targetKey
  discoverLoad: Record<string, DiscoverLoad>;
  discoverError: Record<string, { code: string; message: string } | null>;
  /** Written by the loader when a feed event names a brief a view shows; the mounted lens
   *  re-fetches that view once and clears it. */
  discoverStale: Record<string, true>;
  /** The key a chip asked Discover to open read-only (contract §4.1's one click); null for the
   *  live brief. Not persisted: a restart opens the live brief. */
  pastKey: string | null;

  setBriefView(targetKey: string, view: BriefView): void; // also puts the view's head into headsByBrief,
                                                          // and a non-"" binding.hubProjectId into hubProjectByTarget
  /** Sets pastKey and switches the lens to "discover" through phase 2's existing lens action. */
  openPast(key: string): void;
  closePast(): void;
  setDiscoverLoad(targetKey: string, load: DiscoverLoad, error?: { code: string; message: string }): void;
  markViewStale(targetKey: string): void;
  clearViewStale(targetKey: string): void;
}

export const targetKey = (projectId: string, branchId?: string): string => `${projectId}/${branchId ?? ""}`;
```

```ts
// desktop/src/store/selectors.ts — additions

/** R6. A key source returns the keys a surface currently shows. The loader unions them.
 *  A later lens adds ONE entry to keySources and changes nothing else (phase 5: bound and
 *  approved design keys; phase 7: release pins). */
export type KeySource = (s: CockpitState) => readonly string[];

/** Every session's contextRef that is non-empty. "none:declined" is included: the server
 *  resolves it (P4-C) and the desktop never looks inside it (G7). */
export const selectSessionContextRefs: KeySource;

export const keySources: readonly KeySource[] = [selectSessionContextRefs];

/** The eight states a provenance element can show (contract §4.1, plus erased and declined). */
export type ProvenanceState =
  | "current" | "superseded" | "none" | "declined" | "missing" | "erased" | "resolving" | "unavailable";

/** THE ONE PLACE superseded is decided. Heads are read through headsByBrief, never from the
 *  ref's own headRevision/latestCompleteRevision (those went stale when the ref was cached).
 *  The first rule that applies wins:
 *  1. "" is none; an unknown key is resolving, or unavailable if the last answer said so.
 *  2. A ref that is not resolved shows its resolution: none, declined, missing (invalid shows
 *     as missing), erased.
 *  3. Resolved, and revision < latestCompleteRevision of ITS OWN brief's head: superseded (P4-B).
 *  4. Resolved, inherits non-null, and inherits.revision < latestCompleteRevision of the
 *     INHERITED brief's head (headsByBrief[inherits.briefId]): superseded. The branch revision
 *     pins project text the project has since replaced: an open branch head's latest complete
 *     revision, which keeps its pin (4-D3's narrowing, C4, OQ-8; review I8), or a pin a race
 *     left behind (task 08 steps 8-9).
 *  5. Otherwise current.
 *  A head absent from headsByBrief decides nothing in rule 3 or 4; the loader fetches it (step 2). */
export const selectProvenanceState: (key: string) => (s: CockpitState) => ProvenanceState;
export const selectRef: (key: string) => (s: CockpitState) => ProvenanceRef | undefined;
/** The criterion states of one key, by criterion id, from criterionStatesByKey and never from
 *  the ref (review I14). A criterion id absent from the result reads as { state: "open",
 *  because: "" }, phase 4's only state and the one P12 allows without a reason; a key with no
 *  entry returns one frozen empty record. Memoised per key on the entry's identity, so a
 *  surface re-renders only when the states change. */
export const selectCriterionStates: (key: string) =>
  (s: CockpitState) => Readonly<Record<string, CriterionReading>>;
export const selectHead: (briefId: string) => (s: CockpitState) => Head | undefined;
export const selectSessionRefState: (sessionId: SessionId) =>
  (s: CockpitState) => { key: string; state: ProvenanceState; ref: ProvenanceRef | undefined; head: Head | undefined };
export const selectHeadRef: (target: Target) => (s: CockpitState) => ProvenanceRef | undefined;
export const selectBriefView: (target: Target) => (s: CockpitState) => BriefView | undefined;

/** Frozen, so a component comparing by identity never re-renders on an absent ref. */
export const NO_REF: Readonly<ProvenanceRef>;
```

`selectSessionRefState` builds an object, so its consumers read it with `useShallow`
(substrate §3.2's rule).

```ts
// desktop/src/provenance/loader.ts

/** Runs on the poller's good tick, from App.tsx's callback. Never on its own timer (R6, G8).
 *  1. If hubCursor !== hubCursorRead: ONE hubFeed.changes(after) call. For each "brief.*"
 *     event (subject is the brief id, Hub contract §8 :1139-1143 at hub 2f84662):
 *     a. markStale([subject]);
 *     b. "brief.erased": forgetBrief(subject, event.at) (review I10);
 *     c. markViewStale for EVERY view whose binding.hubProjectId equals event.projectId or is
 *        "", and markHeadRefsStale for EVERY target whose hubProjectByTarget equals
 *        event.projectId or is absent. Matched by project, not by the brief a view shows: a
 *        brief.created or brief.rebound names a brief no view shows yet (a branch showing
 *        brief: null), and the Hub contract invalidates the head reading for every brief.*
 *        (§9.3 :1373-1375; review I10). An event with projectId null matches every entry.
 *  2. Collect keys: union of keySources; drop "", keys in refsByKey, keys in flight, and keys
 *     unavailable fewer than 15 ticks ago; add one held key per stale-head brief. putRefs
 *     marks stale (markStale) the inherits.briefId of each resolved ref it stores whose brief
 *     has no entry in headsByBrief, so selectProvenanceState rule 4 gets the project head to
 *     compare with (review I8); the held key for it is ref.inherits.key. A held key is sent
 *     ONCE per markStale: the call that carried it clears that brief from staleHeads whether
 *     or not a head came back, so a head the answer omits never becomes a request per tick.
 *     Add every key in staleCriteria (review I14), even one refsByKey holds; the call that
 *     carried it clears it, the same once-per-mark rule.
 *  3. None left: return. ZERO requests on a steady tick.
 *  4. Otherwise ONE discover.refs() per 100 keys (in practice one), then putRefs.
 *  A failed call marks its keys unavailable at this tick and raises no toast: every chip already
 *  says "unavailable" in its own words (contract §4.1). */
export function loadProvenance(
  discover: DiscoverClient,
  feed: HubFeedClient,
  store: typeof useCockpit,
): Promise<void>;
```

```ts
// desktop/src/provenance/useHeadRef.ts — the one reader of head readings (review I10)

/** Returns selectHeadRef(target). Calls discover.headRef(target) ONCE, then putHeadRef, when
 *  the target has no entry in headRefByTarget or staleHeadRefs names it; one call in flight per
 *  target, shared by every mounted reader. Until the answer lands it keeps returning the old
 *  reading, so a stale mark never blanks DefinitionOfDone. A failed call leaves the flag set
 *  and does not retry on its own; the next brief.* event or remount tries again. Event-driven,
 *  never on a timer (G8). Phase 3's DefinitionOfDone mount and Discover read head readings
 *  through this hook and nowhere else. */
export function useHeadRef(discover: DiscoverClient, target: Target): ProvenanceRef | undefined;
```

```ts
// desktop/src/api/hubFeed.ts — the Hub task C2's control-plane route, as the Hub contract §9.3
// and §7.11 give it. Phase 4 reads it; the Hub owns it.
export interface HubEvent {
  seq: number; at: string; kind: string; teamId: string | null; projectId: string | null;
  subject: string; actor: { id: string; displayName: string; email: string } | null;
  data: Record<string, string | number | boolean>;
}
export interface HubChanges { events: HubEvent[]; next: string; more: boolean }
export interface HubFeedClient { changes(after: string): Promise<HubChanges> }
export function createHubFeedClient(opts?: ClientOptions): HubFeedClient; // GET /v1/hub/changes?after=
```

`ClientOptions` gains `onHubCursor?: (cursor: string) => void`; the transport calls it whenever a
response carries `Panacea-Hub-Cursor` (Hub contract §9.3; the header is readable only once C2's
`Access-Control-Expose-Headers` amendment lands — until then it is simply never seen, and heads move
only through Discover's own writes and new keys).

**The `poller.ts` header, amended** (the whole of this task's edit to that file):

```ts
// The one interval for the whole app (§4.7). Started when AppShell mounts (task 24), stopped
// when it unmounts. Each tick makes ONE listSessions request. On a good tick App.tsx's callback
// also runs phase 4's provenance loader, which makes AT MOST one batched /v1/discover/refs and,
// when the Hub cursor moved, one /v1/hub/changes — and nothing at all on a steady tick
// (provenance contract §6.4; ruling R6). There is still one clock.
```

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `store: neither slice is persisted` | after writing both slices, `partialize(state)` has exactly phase 2's twelve keys; `PERSIST_VERSION === 1` | brief text in `localStorage` (P4, G3) |
| `loader: steady tick makes no request` | two ticks with the same resolved key set → `refs` called once in total | P8's twelve-requests failure, one level up |
| `loader: twelve panes, one request` | twelve sessions with six distinct keys → one `refs` call with six keys | a request per pane |
| `loader: unions every key source` | a test-only source added to a copy of `keySources` contributes its keys to the same single call | a loader that only knows sessions (critique-2 §1.2) |
| `loader: unavailable backs off` | a key answered `unavailable` is not re-requested for 14 ticks and is on the 15th | a request every 2 s against a down Hub |
| `loader: never caches unavailable` | the slot's ref is not in `refsByKey` | a chip stuck on unavailable after the Hub returns |
| `loader: feed event marks the head stale and re-reads it` | cursor moves; one `changes` call; a `brief.revised` for `bf-…a2` → next `refs` call carries one held `bf-…a2` key; `headsByBrief` updated | a teammate's edit never reaching the chips |
| `loader: brief.erased drops the brief's text and re-resolves` (review I10) | `refsByKey` holds `brief:…a2@5` and `@6` resolved and `brief:…b7@5`; a `brief.erased` for `bf-…a2` → both `a2` entries gone, `b7@5` kept; the next tick's one `refs` call carries `a2@5` (a session still shows it); the fake answers it **`resolved`** with its old sections (the control plane's cache, until HA4-8) → stored `erased`, `sections` `[]`, `headline` `""`, key and digest kept; `selectProvenanceState` → `erased` | erased text on a laptop until restart (ADR-H1 §6.8; `task-07.md` rule 4), or a desktop that trusts the control plane's pre-HA4-8 cache over the Hub's own event |
| `loader: brief.* marks every view and head reading of the project stale` (review I10) | views `pj-1/b-9` (`brief: null`, binding `pj-hub-1`), `pj-1/` (binding `pj-hub-1`), `pj-2/` (binding `pj-hub-2`) and head readings for the same three targets (`pj-1/b-9`'s a `none` Ref, known only through its view's binding); a `brief.created` for a new brief of `pj-hub-1` → `discoverStale` and `staleHeadRefs` name `pj-1/b-9` and `pj-1/`, never `pj-2/`; a target with no known Hub project is named too; a `projectId: null` event names all | a teammate's new branch brief never reaching `DefinitionOfDone` (*no brief* until navigation), or an event for one project re-reading every project |
| `loader: criterion states move, the ref does not` (review I14) | tick 1 answers `a2@5` with `criterionStates` `open`; `markCriteriaStale(["brief:…a2@5"])` → tick 2's one `refs` call carries that key although `refsByKey` holds it; it answers `settled, "CI run 88 passed"` → `selectCriterionStates` gives `settled` and its `because`; `refsByKey["brief:…a2@5"]` is the same object as after tick 1; tick 3 makes **no** request | a verdict frozen in the forever-cached ref, or a states re-read that becomes a request per tick |
| `selectors: criterion states come from the map, never the ref` (review I14) | a ref whose `criteria` items carry a stray `state: "settled"` and no map entry → every id reads `open`; the same frozen empty record on two calls | a reader that falls back to a field the wire no longer has |
| `useHeadRef: one read per stale mark` (review I10) | two mounted readers of one target, no entry → one `headRef` call; `markHeadRefsStale([t])` → one more call, and the old reading is returned until it lands; the answer clears the flag → no third call; a failed call → no retry until the next mark | a `DefinitionOfDone` that never re-reads, one that blanks on every event, or a head read per render |
| `loader: no timer` | `vi.getTimerCount()` is unchanged by importing and running the loader | a third interval (G8) |
| `selectors: superseded compares with latest complete` | ref rev 5, head `{headRevision: 7, latestCompleteRevision: 5}` → `current`; `{…: 6}` → `superseded` | P4-B inverted: every chip superseded during an unfinished edit |
| `selectors: superseded by the inherited project revision` (review I8) | branch ref `b-51@6`, own head `{headRevision: 7, latestCompleteRevision: 6}`, `inherits` `b-7@5`; project head `b-7` `{6, 6}` → `superseded`; `{5, 5}` → `current`; `{7, 5}` (project edit open) → `current`; `b-7` absent from `headsByBrief` → `current`; the ref's own `headRevision`/`latestCompleteRevision` fields set to 99 change none of these | an open branch head's latest complete revision (OQ-8), or a raced pin, shown `current` over replaced project text; or rule 4 reading the cached ref's frozen head numbers instead of `heads` |
| `loader: a missing project head is fetched once` (review I8) | `putRefs` stores that branch ref with no `b-7` head → the next tick's one `refs` call carries `brief:…b7@5`; its answer carries no `b-7` head → the tick after makes **no** request | rule 4 never getting a project head; or a head the server omits turning into a request every 2 s |
| `selectors: the eight states` — table | each input → its state; `""` → `none`, `none:declined` resolved → `declined`, `invalid` → `missing` | two states merged, which the contract calls lying about one |
| `selectors: no key is ever split` | a source scan of `desktop/src/**/*.ts(x)` finds no `.split("@")`, `.split(":")` or regex over a variable named `*key*`/`contextRef` | P5 on the desktop (G7) |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail.
- [ ] **Step 2:** The two slices; add them to `createCockpitStore`'s spread (`store.ts:62-66` at
  `3303fd6`) and to `CockpitState`; leave `Persisted`, `partialize` and `migrate` untouched.
- [ ] **Step 3:** Selectors; the loader; `useHeadRef.ts`; `hubFeed.ts`; `onHubCursor`.
- [ ] **Step 4:** `App.tsx`: the good-tick callback calls `void loadProvenance(discoverApi,
  hubFeed, useCockpit)` after its existing work. `poller.ts`: the header comment above, nothing else.
- [ ] **Step 5:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
$ grep -rn "setInterval" desktop/src --include=*.ts --include=*.tsx | grep -v "\.test\." | wc -l
2
$ git diff HEAD -- desktop/src/api/poller.ts | grep "^[+-][^+-]" | grep -v "^[+-]//"
(no output — only comment lines changed)
```

**Does NOT claim:** anything rendered (task 17); the feed's behaviour against a real Hub (C2).
