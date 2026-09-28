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

**Files** (the skeleton's File map lists `src/work/replica/{…,search}.ts` for 55-58, `:305`; it
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

#### Interfaces — complete: `SearchField`, `OpenByNumber` and the two verbs

```tsx
// desktop/src/components/work/SearchField.tsx
export interface SearchFieldProps {
  replica: Replica;
  work: Pick<WorkClient, "searchIssues">;
  /** The view's project, when the view has one; search spans every readable project otherwise. */
  projectId?: string;
  onOpen(issueId: string): void;
  /** Esc, or a click outside. Focus returns to the list row that had it (task 40's roving tab). */
  onClose(): void;
}
/** <div class="wk-search" role="search">
 *    <input aria-label="Search issues" aria-controls="wk-search-hits" aria-activedescendant=…/>
 *    <p class="wk-search-from">from the Hub · titles only</p>        (only when from === "hub")
 *    <ul id="wk-search-hits" role="listbox">
 *      <li role="option" id="hit-{issueId}" aria-selected>
 *        <span class="wk-search-alias">EP-142</span> <span>Resume an upload after a drop</span>
 *        <span class="wk-search-where">in description</span>        (only when matched === "description")
 *  `↓`/`↑` move the selection BY ID (the id survives a re-rank as the replica changes); `Enter`
 *  opens the selected hit. No count of hits is shown: a list of fifty is its own evidence. */
export function SearchField(props: SearchFieldProps): JSX.Element;
```

```tsx
// desktop/src/components/work/OpenByNumber.tsx
export interface OpenByNumberProps {
  /** The current view's project key, printed as the prompt's fixed prefix: "EP-". */
  projectKey: string;
  /** replica.resolveAlias bound to projectKey. */
  resolve(n: number): string | null;
  onOpen(issueId: string): void;
  onClose(): void;
}
/** <form class="wk-open"><label><span class="wk-open-key">EP-</span>
 *    <input inputMode="numeric" aria-label="Issue number" /></label>
 *    <p class="wk-open-miss">No EP-999 in EP</p></form>              (only after a miss)
 *  Only digits enter the field (a non-digit is ignored, not an error). Enter on a miss keeps the
 *  prompt open with the digits selected, so the next number replaces them. */
export function OpenByNumber(props: OpenByNumberProps): JSX.Element;
```

```ts
// desktop/src/work/keys/searchVerbs.ts — both verbs through task 53's one registry.
import { registerVerb } from "./keymap";
export function registerSearchVerbs(open: { search(): void; byNumber(): void }): () => void {
  const offSearch = registerVerb("list.search", open.search);
  const offNumber = registerVerb("issue.openByNumber", open.byNumber);
  return () => { offSearch(); offNumber(); };
}
```

```css
/* desktop/src/components/work/work.css ⚑ — appended */
.wk-search, .wk-open { border-radius: var(--r-panel); background: var(--raised);
  box-shadow: 0 0 0 1px var(--ring), var(--shadow-2); padding: 6px; }
.wk-search-alias, .wk-open-key { font-family: var(--mono); font-variant-numeric: tabular-nums; color: var(--ink-2); }
.wk-search-where, .wk-search-from, .wk-open-miss { font-size: var(--t-fact); color: var(--ink-3); }
.wk-search [aria-selected="true"] { background: var(--inset); }
```

The token names `--r-panel`, `--raised`, `--ring` and `--shadow-2` are **UNVERIFIED** here against
`tokens.css`; task 26 owns the names, and the rule's shape (a 1px ring plus a layered shadow, a radius
from the size scale) is the design system's, whatever the names are.

`WorkLens.tsx` ⚑ (task 40) owns the two open states: it calls `registerSearchVerbs` in an effect,
renders `SearchField` above the list and `OpenByNumber` in the view header while open, and passes
`onOpen` through to task 46's issue route. One more file joins the list: `WorkLens.tsx` (modified).

#### Tests to write first, part 1 — `desktop/src/work/replica/search.test.ts` (the index)

```ts
import { describe, expect, it } from "vitest";
import { WORK_IDS } from "../../test/workFixtures";
import { createSearchIndex, fold, type IndexInput } from "./search";

const P = WORK_IDS.project;
const rec = (over: Partial<IndexInput>): IndexInput => ({ issueId: "is-x", projectId: P, number: 1,
  title: "", description: "", rev: 1, updatedAt: "2026-09-25T00:00:00Z", ...over });
function fixture(key = "EP") {
  const ix = createSearchIndex();
  ix.put(rec({ issueId: WORK_IDS.ep142, number: 142, title: "Resume an upload after a drop" }));
  ix.put(rec({ issueId: WORK_IDS.ep150, number: 150, title: "Chunk sizes for large files",
    description: "so a client can resume mid-file", updatedAt: "2026-09-25T09:00:00Z" }));
  ix.put(rec({ issueId: WORK_IDS.ep160, number: 160, title: "Résumé upload on the profile page" }));
  let k = key;
  return { ix, keyOf: () => k, rekey: (nk: string) => { k = nk; } };
}

describe("search ranks, folds and formats", () => {
  it("a title hit ranks above a newer description-only hit", () => {
    const { ix, keyOf } = fixture();
    const hits = ix.query("resume", keyOf);
    expect(hits.map((h) => [h.alias, h.matched])).toEqual([
      ["EP-142", "title"], ["EP-160", "title"], ["EP-150", "description"]]);
  }); // kills: unranked results (EP-150 is the newest and would lead by time alone)
  it("EP-142, ep-142 and 142 all put EP-142 first", () => {
    const { ix, keyOf } = fixture();
    for (const q of ["EP-142", "ep-142", "142"]) expect(ix.query(q, keyOf)[0]).toMatchObject({ alias: "EP-142", matched: "alias" });
  }); // kills: alias as plain text (tokens "ep" and "142" match nothing in any title)
  it("resumé finds resume, and resume finds Résumé", () => {
    const { ix, keyOf } = fixture();
    expect(fold("Résumé")).toBe("resume");
    expect(ix.query("resumé", keyOf).map((h) => h.alias)).toContain("EP-142");
    expect(ix.query("resume", keyOf).map((h) => h.alias)).toContain("EP-160");
  }); // kills: a naive includes() with no folding
  it("an erased issue's old title finds nothing; its number still resolves", () => {
    const { ix, keyOf } = fixture();
    ix.put(rec({ issueId: WORK_IDS.ep142, number: 142, title: "", description: "" }));   // task 57's erase
    expect(ix.query("upload after", keyOf)).toEqual([]);
    expect(ix.byNumber(P, 142)).toBe(WORK_IDS.ep142);
  }); // kills: an index that keeps text after an erase
  it("after the key changes to UP, UP-142 resolves and EP-142 does not", () => {
    const { ix, keyOf, rekey } = fixture();
    rekey("UP");
    expect(ix.query("UP-142", keyOf)[0]).toMatchObject({ alias: "UP-142", matched: "alias" });
    expect(ix.query("EP-142", keyOf).filter((h) => h.matched === "alias")).toEqual([]);
  }); // kills: parsing or storing the alias
  it("records the palette budget over 10,000 issues (recorded, not asserted)", () => {
    const ix = createSearchIndex();
    for (let n = 1; n <= 10_000; n++) ix.put(rec({ issueId: `is-${n}`, number: n, title: `Issue ${n} upload retry`,
      description: "a resumable chunked upload", updatedAt: `2026-09-${String(1 + (n % 28)).padStart(2, "0")}T00:00:00Z` }));
    const t0 = performance.now();
    ix.query("resum up", () => "EP");
    console.info(`budget.palette search over ${ix.size()} issues: ${(performance.now() - t0).toFixed(1)} ms`);
    expect(ix.size()).toBe(10_000);
  }); // records, never fails on time (OD-27)
});
```

In the first test EP-142 and EP-160 share an `updatedAt`, so the alias breaks their tie; EP-150 is
the newest of the three and still comes last, which is what makes time-only ordering fail.

#### Tests to write first, part 2 — `desktop/src/components/work/search.test.tsx` (keys and surfaces)

The lens is mounted as task 41's lens tests mount it, through task 40's seam
`setWorkSourceForTest(world)` over `workScenario()` (SW.8; `task-41.md:462-466`, **UNVERIFIED**
until task 40's part lands). Keys go through task 53's real keymap, never a stub: the test fires
`keydown` with both `key` and `code` set, as task 41's `N` test does (`task-41.md:472`).

| Test | What it asserts | Mutant it kills |
|---|---|---|
| `slash in a list opens search; slash in the editor does not` | focus the list, `keyDown({key: "/", code: "Slash"})`: `getByRole("search")` exists and its input has focus. Then with task 49's editor focused (a `contenteditable`), the same key: no `role="search"`, and the editor's text or slash menu got the key | a global `/` listener on `document` |
| `typing ranks a title hit first and marks a description hit` | type `resume`; the first `option` reads `EP-142` in `.wk-search-alias`; the option for the description-only hit carries `.wk-search-where` with `in description`; `expectNoPrediction(container)` | unranked hits; a hit count or `N of M` in the field |
| `the selection is by id across a re-rank` | select the second option with `↓`; the world changes the first hit's title so it no longer matches; the selected option is still the same issue's `hit-{issueId}` | selection by index, which silently moves to a different issue |
| `O I 142 Enter opens EP-142` | keys `O`, `I` (task 54's sequence), then type `142`, `Enter`: `onOpen` received `WORK_IDS.ep142` exactly once | an unrouted `issue.openByNumber`; opening a wrong issue |
| `O I 999 Enter goes nowhere and says so` | the same with `999`: `onOpen` not called; `.wk-open-miss` reads exactly `No EP-999 in EP`; the input keeps focus with `999` selected | a navigation to a blank view; a silent miss |
| `the number prompt takes digits only` | type `1a4-2`: the input's value is `142` | a prompt that passes `EP-142`-style text on to a parser |

```tsx
// the loading fallback, in the same file
it("while the replica loads, the Hub's title search is asked once and labelled", async () => {
  const replica = { status: "loading", search: () => [], resolveAlias: () => null } as unknown as Replica;
  const searchIssues = vi.fn(async () => [{ ...makeIssueHead({ id: WORK_IDS.ep142 }), title: "Resume an upload after a drop" }]);
  render(<SearchField replica={replica} work={{ searchIssues }} onOpen={vi.fn()} onClose={vi.fn()} />);
  fireEvent.change(screen.getByRole("searchbox", { name: "Search issues" }), { target: { value: "resume" } });
  expect(await screen.findByText("Resume an upload after a drop")).toBeTruthy();
  expect(screen.getByText("from the Hub · titles only")).toBeTruthy();
  expect(searchIssues).toHaveBeenCalledTimes(1);
  expect(searchIssues).toHaveBeenCalledWith({ q: "resume", limit: 20 });
}); // kills: empty results after a restart; kills: a Hub call per render
```

The input is `type="search"`, so its role is `searchbox`. `makeIssueHead` returns task 20's head
without `title`; the spread adds it as the Hub's wire does (decision 4's gap).

#### Steps

- [ ] Write `search.test.ts`; it fails to import `./search`. Write `search.ts` until the six pass.
      Mutants, one at a time, then revert: sort by `updatedAt` alone (test 1); drop the `whole ===`
      branch (test 2); replace `fold` with `toLowerCase` (test 3); keep a `lastTitle` per issue that
      `put` never clears (test 4); format the alias from a stored `head.alias` (test 5).
- [ ] Feed the index from `replica.ts`'s `transact`; add `search` and `resolveAlias`. Task 55's
      tests still pass.
- [ ] Write `useSearch.ts`, `SearchField.tsx`, `OpenByNumber.tsx`, `searchVerbs.ts`; append the CSS.
- [ ] Write `search.test.tsx`; wire `WorkLens.tsx`. Mutants: bind `/` on `document` (test 1);
      select by index (test 3); call `onOpen` before `resolve` answers (test 5); call the Hub in the
      render body (the fallback test).
- [ ] Commit each file as it goes green. Paste the recorded `budget.palette` line into the commit
      message of `search.ts`, as a measurement with its machine named, not as a pass.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/replica/search.test.ts src/components/work/search.test.tsx'
# expected: 2 files, 13 tests, all passing (6 index, 7 surface), and one line
# "budget.palette search over 10000 issues: <n> ms" in the output: a record, not a verdict
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 40, 41, 53, 55 and 57 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -rnF "split(\"-\")" desktop/src/work desktop/src/components/work'
# expected: no output (no alias parser, task-02.md:176-178; a review still reads search.ts for one)
```

#### What this task does NOT claim

- **`@person`, `#label` and `EP-` filter prefixes inside search** (L-107): W2. The palette's own
  `#`, `@`, `>` and `~` scopes are task 59's.
- **Natural-language filters** (L-100): W4.
- **A moved issue's old number redirecting** (L-35): W2. Here an old alias simply does not resolve.
- **Search over comments**: not in 05 §4.3's `/`; comments are not indexed.
- **`budget.palette ≤ 50 ms` as met**: the test records the time on the machine that ran it.
- **Parity**: closes L-106's W1 part (row 106, `05-design.md:240`).

#### Gaps found while writing this part

1. The skeleton's file map omits `SearchField.tsx`, `OpenByNumber.tsx`, `useSearch.ts`,
   `searchVerbs.ts` and the `WorkLens.tsx` edit.
2. Task 37's `IssueHead` (`task-37.md:121-128`) has no `title`, but the Hub's projection puts one on
   every head (`task-14.md:279-287`); `hubHit` reads it defensively until task 37 declares it.
3. `list.search` is this part's name for the `/` row of task 53's `KEYMAP`; task 53 should print the
   row with that id or say its own.
4. Tasks 40, 53, 54 and 55 had no part files at this writing; their seams (`setWorkSourceForTest`,
   `registerVerb`, the `O I` sequence, the replica's `transact`) are their scopes' words.
