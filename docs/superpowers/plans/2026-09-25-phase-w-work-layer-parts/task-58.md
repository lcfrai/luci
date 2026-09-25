### Task 58: the replica, part 4: `/` search and `O I` open by number

Stream **WD1**, the last of seam **SW.16**'s four signatures, `search(q)`, about 2.5 hours (an
estimate, not a measurement). Depends on task 55 (the replica's records, `status`, `useWork`) and
task 53 (`registerVerb`, `isListKeyTarget`, `KEYMAP`). It reads task 57's `scope()` for each
project's current key (`task-57.md`, `ProjectScope.key`), task 54's `O` then `I` sequence, which
calls the verb `issue.openByNumber` that this task registers, and task 37's `searchIssues`
(`task-37.md:347`). Task 53's, 54's and 55's parts were not on the tree when this was written:
their names here are their **scopes'** words (`tasks-41-60.md:457-490` and the 53 and 54
paragraphs), **UNVERIFIED** until their parts land, and they win any disagreement.

After this task a person finds any issue they can read by typing, without a round trip, and opens
one by its number without the mouse.

**What the reader sees** (05 §4.3, §3.6; row 106 of 05's parity table, `05-design.md:240`):

- `/` in a work list: a search field opens over the current view, at the top of the list, with the
  caret in it. Typing `resume` lists EP-142 *Resume an upload after a drop* first, because its title
  matches, and below it an issue whose description alone mentions resuming. Each hit is one line:
  alias in mono, title in sans, and a quiet `in description` fact where the hit is not the title.
- `EP-142` or `142` puts EP-142 first. `resumé` finds `resume`.
- `O` then `I`: a small mono prompt `EP-` with the caret after it. `142` `Enter` opens EP-142 at its
  head. `999` `Enter` says *No EP-999 in EP* on the prompt and goes nowhere.
- Just after a restart, while the replica still loads: the same field answers from the Hub's title
  search, and its one fact line says *from the Hub · titles only*.
- `/` inside the editor is still task 50's slash menu; the search field never opens there.

**Decisions made here, so nobody re-derives them:**

1. **An alias is formatted, never parsed.** Task 02 forbids an alias parser (`task-02.md:176-178`)
   and task 14's Hub search formats each readable alias and compares it (`task-14.md:270-276`).
   The desktop does the same: the index keeps `(projectId, number)` and formats `KEY-n` from the
   project's **current** key (task 57's `scope()`), never from a stored alias string. So after EP
   becomes UP, `UP-142` resolves and `EP-142` does not (the scope's key-change test).
   `resolveAlias(projectKey, n)` takes the key and the number **apart**: the `O I` prompt already
   knows the current project's key and asks only for digits.
2. **Matching.** Both sides are folded the same way: `normalize("NFKD")`, combining marks
   (`\p{M}`) removed, lower case. The query splits on anything that is not a letter or a digit;
   each query token must be a **prefix** of some token of the field. Tiers, best first: (a) the
   whole folded query equals a formatted alias, or equals a number the index holds, (b) every token
   matches in the title, (c) every token matches in the title and description together. Inside a
   tier: newest `updatedAt` first, then the alias, so the order is stable. The desktop compares
   aliases after folding, so `ep-142` finds EP-142 here, where the Hub compares case-sensitively
   (`task-14.md:275`): a person typing in a search field should not need Shift.
3. **The index is derived, not stored.** `createSearchIndex()` is fed by the replica on every
   `transact` (`put` and `drop`), from records it already holds. An erased record's title and
   description are empty (task 57, decision 6), so it has no tokens and no text match; its number
   still resolves, so `O I` still opens it as *erased*. Nothing is written anywhere but memory
   (task 55's no-persistence test covers `src/work/**`).
4. **While loading, ask the Hub once.** When `status` is `"loading"` or `"empty"`, `useSearch`
   calls `searchIssues({q, projectId, limit: 20})` **once per settled query** (one in flight; a
   newer query replaces the older answer), and labels the hits *from the Hub · titles only*. Once
   `status` is `"ready"`, the replica answers and the label goes. The Hub's `IssueHead` carries
   `title` on the wire (`task-14.md:279-287`); task 37's TypeScript `IssueHead` does not declare it
   (`task-37.md:121-128`), a gap named at the end.
5. **Keys.** `/` is the verb `list.search` and `O I` reaches `issue.openByNumber`, both through task
   53's `registerVerb`, so both inherit `isListKeyTarget`: false in an input, a textarea, a
   contenteditable (the editor) and a terminal host. That is the whole reason `/` in the editor stays
   the slash menu: this task adds no check of its own. `list.search` is this part's name for the `/`
   row of task 53's `KEYMAP` (**UNVERIFIED**); `issue.openByNumber` is task 54's scope's.
6. **Budgets are recorded, not asserted.** `budget.palette ≤ 50 ms` (05 §4.6,
   `05-design.md:1103`) is timed over a 10,000-issue fixture with `performance.now()` and written
   to the test's output; no test fails on it (`OD-27`: a number is proof only when measured).

**Files** (the skeleton's File map lists `src/work/replica/{…,search}.ts` for 55-58, `:304`; it
omits `SearchField.tsx`, `OpenByNumber.tsx` and `searchVerbs.ts`, gaps named at the end):
- `desktop/src/work/replica/search.ts` — `fold`, `tokens`, `createSearchIndex`, `SearchHit` (new)
- `desktop/src/work/replica/search.test.ts` — ranking, folding, erase, key change, the budget (new)
- `desktop/src/work/replica/replica.ts` (task 55) — feeds the index; `search`, `resolveAlias` (modified)
- `desktop/src/work/replica/useSearch.ts` — the replica or, while loading, the Hub once (new)
- `desktop/src/components/work/SearchField.tsx` — the `/` field over a view (new)
- `desktop/src/components/work/OpenByNumber.tsx` — the `O I` prompt (new)
- `desktop/src/components/work/search.test.tsx` — keys, prompt, loading fallback (new)
- `desktop/src/work/keys/searchVerbs.ts` — registers `list.search` and `issue.openByNumber` (new)
- `desktop/src/components/work/work.css` ⚑ — `.wk-search*` and `.wk-open*` (modified)

#### Interfaces — complete: `desktop/src/work/replica/search.ts`

```ts
// desktop/src/work/replica/search.ts — the in-memory index. No I/O; the replica feeds it.

/** One hit. `alias` is formatted from the project's current key at query time (decision 1). */
export interface SearchHit {
  issueId: string;
  alias: string;
  title: string;
  matched: "alias" | "title" | "description";
  rev: number;
}
export interface SearchOptions { projectId?: string; limit?: number }

/** What the index needs of one issue. `title` and `description` are "" once erased. */
export interface IndexInput {
  issueId: string; projectId: string; number: number;
  title: string; description: string; rev: number; updatedAt: string;
}

/** NFKD, combining marks removed, lower case: "Resumé" → "resume". */
export function fold(s: string): string {
  return s.normalize("NFKD").replace(/\p{M}+/gu, "").toLowerCase();
}
/** Folded tokens: runs of letters or digits. "EP-142 resumé!" → ["ep", "142", "resume"]. */
export function tokens(s: string): string[] {
  return fold(s).match(/[\p{L}\p{N}]+/gu) ?? [];
}
/** Every query token is a prefix of some field token. An empty query matches nothing. */
export function prefixMatch(query: readonly string[], field: readonly string[]): boolean {
  return query.length > 0 && query.every((q) => field.some((f) => f.startsWith(q)));
}

export interface SearchIndex {
  put(rec: IndexInput): void;
  drop(issueId: string): void;
  /** The issue with this number in this project, or null. The only way a number becomes an id. */
  byNumber(projectId: string, n: number): string | null;
  /** `keyOf(projectId)` is the project's current key, from task 57's scope(); "" when unknown. */
  query(q: string, keyOf: (projectId: string) => string, opts?: SearchOptions): SearchHit[];
  size(): number;
}

export function createSearchIndex(): SearchIndex;
```

#### Reference implementation: `query` (the tests are the contract; this is one way to pass them)

```ts
interface Entry extends IndexInput { titleT: string[]; allT: string[] }
const TIER = { alias: 0, title: 1, description: 2 } as const;

function query(entries: ReadonlyMap<string, Entry>, q: string, keyOf: (p: string) => string,
               opts: SearchOptions = {}): SearchHit[] {
  const whole = fold(q.trim());
  const qt = tokens(q);
  const hits: (SearchHit & { updatedAt: string })[] = [];
  for (const e of entries.values()) {
    if (opts.projectId && e.projectId !== opts.projectId) continue;
    const key = keyOf(e.projectId);
    const alias = key ? `${key}-${e.number}` : `${e.number}`;
    const matched: SearchHit["matched"] | null =
      whole !== "" && (whole === fold(alias) || whole === String(e.number)) ? "alias"
      : prefixMatch(qt, e.titleT) ? "title"
      : prefixMatch(qt, e.allT) ? "description"
      : null;
    if (matched) hits.push({ issueId: e.issueId, alias, title: e.title, matched, rev: e.rev, updatedAt: e.updatedAt });
  }
  hits.sort((a, b) => TIER[a.matched] - TIER[b.matched]
    || (a.updatedAt < b.updatedAt ? 1 : a.updatedAt > b.updatedAt ? -1 : 0)
    || a.alias.localeCompare(b.alias));
  return hits.slice(0, opts.limit ?? 50).map(({ updatedAt: _u, ...h }) => h);
}
```

`put` stores `titleT = tokens(title)` and `allT = [...titleT, ...tokens(description)]`, and keeps a
second map `projectId → number → issueId` for `byNumber`; `drop` removes both. `whole ===
fold(alias)` compares the formatted alias with the whole query; the query is never split into a key
and a number. A linear scan over 10,000 entries is the first cut; the budget test records its time,
and an inverted index is a later change only if the recorded number says so.

#### Interfaces — complete: the deltas on `replica.ts` (task 55) and `useSearch.ts`

```ts
// desktop/src/work/replica/replica.ts — task 58 adds these; task 55's Replica keeps its members.
import { createSearchIndex, type SearchHit, type SearchOptions } from "./search";

export interface ReplicaSearch {
  /** In memory, synchronous. Empty while status is "empty" or "loading": use useSearch there. */
  search(q: string, opts?: SearchOptions): SearchHit[];
  /** The issue numbered n in the project whose CURRENT key is projectKey, else null. */
  resolveAlias(projectKey: string, n: number): string | null;
}
// `interface Replica extends ReplicaSearch`. Inside every transact, each put head or revision
// calls index.put({issueId, projectId, number, title, description, rev, updatedAt}) and each
// dropped record calls index.drop(issueId): one index, fed where the records change.
```

```ts
// desktop/src/work/replica/useSearch.ts
import { useEffect, useState } from "react";
import type { WorkClient } from "../../api/work";
import type { Replica } from "./replica";
import type { SearchHit, SearchOptions } from "./search";

export interface SearchView { hits: SearchHit[]; from: "replica" | "hub"; waiting: boolean }

/** The replica answers when ready. Before that, the Hub's title search, once per query
 *  (decision 4). `waiting` is true only while that one Hub call is in flight. */
export function useSearch(r: Replica, work: Pick<WorkClient, "searchIssues">, q: string,
                          opts?: SearchOptions): SearchView;

/** Task 37's IssueHead lacks `title`; the Hub's wire has it (task-14.md:279-287). Until task 37
 *  declares it, read it through this one function so the gap has one place to close. */
export function hubHit(h: { id: string; alias: string; headRevision: number; title?: string }): SearchHit {
  return { issueId: h.id, alias: h.alias, title: h.title ?? "", matched: "title", rev: h.headRevision };
}
```
