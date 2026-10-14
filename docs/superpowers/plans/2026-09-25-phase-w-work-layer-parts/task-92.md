### Task 92: drafts, callouts and *Copy as Markdown* with the Hub key

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WD2.
**dependsOn:** 50 (the editor's second part) and 83 (the W2 wire types), after a sync-merge of
`feat/work-layer` (rule H1). **Seams:** consumes **SW.18**'s `Draft` through task 83's
`desktop/src/api/work.ts`, **SW.16**'s store read (`useWork`), and **SW.1**'s bounded lengths;
adds **no** new seam. Task 55's `noPersistence.test.ts` glob is widened by one directory.

Linear keeps what you were typing. Close the composer, come back tomorrow, the text is there. It
does that by writing to the browser's storage. **We cannot**, because no client persists Hub text
to disk (ADR-H1) and a draft is Hub text: it holds an issue's title and body before the issue
exists. So this task puts the draft **in the Hub**, private to its author, expiring after six
months, and the desktop holds the open one **in memory only**, saving it on an idle debounce and
on blur. The result reads the same to a person and stores nothing on the device.

The same task gives the editor its **callout** node with a Markdown round-trip (L-127's callout
part) and *Copy as Markdown* (L-129). Copied text carries the issue's **revision key** beside its
alias, `EP-142` · `issue:is-…@3`, because an alias is a display name and never a key (skeleton
rule 1): paste it into a Teams message six weeks later and the key still resolves after the
project has been renamed, and the `@3` says which revision the person actually read.

#### Anchors, re-read on 2026-09-27

In this tree, verified: `desktop/package.json` has `@tauri-apps/plugin-opener` (`:17`) and no
`prosemirror-*` of any kind; `desktop/src/components/` holds `common`, `develop`, `lens`, `menu`,
`palette`, `rail`, `subrail`, `topbar` and **no** `editor`, `work`, `issue` or `desk` directory
(`ls`, 2026-09-27); `desktop/src/lib/` holds `ids.ts`, `immutable.ts`, `pendingSessions.ts`,
`time.ts`, `wire.ts`; `localStorage` appears today in `desktop/src/store/store.ts`,
`desktop/src/lib/ids.ts` and `desktop/src/components/develop/CanvasLayout.tsx` — all three are
**desktop** state (layout, a device id, a window pref), none is Hub text, and task 55's test
allow-lists them.

In this branch's parts (**UNVERIFIED until merge**): task 83's `Draft` type and relay calls
(`tasks-81-100.md:90-104`); task 80's four draft routes `POST`, `PUT`, `GET`, `DELETE
/v1/me/drafts[/{draftId}]`, author-only with `404` to everyone else, expiry by task 77's janitor
(`tasks-61-80.md:672-675`); task 55's `noPersistence.test.ts` and `ReplicaDeps`
(`tasks-41-60.md:457-462`); task 50's slash menu and table nodes (`tasks-41-60.md:307`); task 49's
`text·H` / `text·A` authorship classes. **Task 50's part is not written at this writing**, so the
editor's schema file name is taken from the scope's `desktop/src/components/editor/*` and is
**UNVERIFIED**.

Design: `05-design.md` §1 L-33, L-44, L-127, L-129 and §3.2's `df-` record (*"Hub, private to its
author … expires after 6 months"*). ADR-H1:467-474 is the no-persistence rule.

#### Decisions the scope left open, made here

**(a) One draft per scope, keyed by what is being written**, not per keystroke stream. The scope
is `{kind: "new-issue", teamId}`, `{kind: "description", issueId}` or `{kind: "comment", issueId,
parentId}`. Opening a composer looks for an existing draft of that scope before it makes one, so
re-opening *New issue* on the same team finds yesterday's text rather than a second draft. The
Hub's `POST /v1/me/drafts` is called **once per scope**, on the first save, not on open: opening a
composer and closing it untouched leaves nothing behind.

**(b) The debounce is 1,500 ms idle, and the clock is injected.** `DraftDeps.schedule` wraps the
timer so every test drives it, and no test sleeps (task 77's rule, applied to the desktop). Blur
saves at once and cancels the pending timer. A save in flight when new text arrives is not
cancelled; the next save is queued and coalesced, so at most one `PUT` is ever outstanding per
draft and text cannot land out of order.

**(c) An unreachable Hub is said out loud, not worked around.** When a save fails with a transport
error the composer shows `not saved — the Hub is unreachable` in `--ink-3` beside the byline and
retries on the next idle tick with a ceiling of 30 s. The text stays in memory. If the window
closes it is gone, and the composer says so before it is: closing a composer in `unreachable`
asks *Discard unsaved text?* with **Discard** and **Keep editing**. This is the *does not claim*
made visible rather than hidden.

**(d) A draft list is a menu, not a screen.** `Drafts` hangs off the composer's overflow and the
palette (`Open a draft`), listing each draft's scope line, its age (`edited 3d ago`) and its
expiry as a **date**, not a countdown (*expires 27 Mar 2027*), because a countdown is a live
number on a resting surface and the Desk rule forbids one. Ten most recent, then *All drafts…*.

**(e) A callout has three tones and no colour of its own.** `note`, `warn` and `stop` reuse
`--ink-2`, `--need` and `--fail` — the status vocabulary of `tokens.css:74-77` — each with its
shape, never colour alone (`tokens.css` rule 5). The Markdown form is GitHub's alert syntax,
`> [!NOTE]`, so a callout pasted anywhere else degrades to a blockquote rather than to noise.

**(f) *Copy as Markdown* writes two flavours to one clipboard event**, `text/plain` (the Markdown)
and `text/html` (the same tree rendered), so a paste into Teams or Outlook keeps structure and a
paste into a terminal keeps text. Both carry the same first line: the alias, a middle dot, and the
revision key in `--mono`.

#### Files

| File | New / ⚑ | What it gains |
|---|---|---|
| `desktop/src/work/drafts.ts` | new | `createDraftController`, `DraftHandle`, `DraftState` |
| `desktop/src/work/drafts.test.ts` | new | the unit suite (9 tests) |
| `desktop/src/work/markdown/callout.ts` | new | the callout node spec and its Markdown pair |
| `desktop/src/work/markdown/callout.test.ts` | new | the round-trip suite |
| `desktop/src/work/markdown/copy.ts` | new | `copyAsMarkdown`, `markdownHeadLine` |
| `desktop/src/work/markdown/copy.test.ts` | new | the head-line and flavour suite |
| `desktop/src/components/work/DraftMenu.tsx` | new | decision (d)'s menu |
| `desktop/src/components/work/DraftMenu.test.tsx` | new | the menu suite |
| `desktop/src/components/work/NewIssue.tsx` | ⚑ (task 84-91's) | autosaves through the controller |
| `desktop/src/components/issue/Comments.tsx` | ⚑ (task 49's) | autosaves; *Copy as Markdown* |
| `desktop/src/components/editor/schema.ts` | ⚑ (task 50's) | registers the callout node |
| `desktop/src/components/editor/slashMenu.tsx` | ⚑ (task 50's) | three callout entries |
| `desktop/src/work/replica/noPersistence.test.ts` | ⚑ (task 55's) | globs `src/work/**` too |

**No `desktop/package.json` change.** The scope allows a `prosemirror-*` addition *"only if
needed, naming its licence"*. A callout is a plain block node with one attribute and a
`toDOM`/`parseDOM` pair on task 50's existing schema, and the alert syntax is eleven lines of
serialiser. Nothing new is installed, so no licence question arises. If task 50 lands on a
different editor core than ProseMirror, only `callout.ts`'s node spec changes; `copy.ts`,
`drafts.ts` and every test in this task are independent of it.

#### `desktop/src/work/drafts.ts`, complete

```ts
// desktop/src/work/drafts.ts
//
// Drafts live in the Hub (ADR-H1: no client persists Hub text to disk). This controller holds
// ONE open draft in memory, saves it on an idle debounce and on blur, and says so out loud when
// the Hub cannot be reached. It never touches localStorage, sessionStorage, IndexedDB or a file;
// task 55's noPersistence test enforces that by grep over this directory.

import type { Draft } from "../api/work";

/** What is being written. Two scopes are equal when every field is equal. */
export type DraftScope =
  | { readonly kind: "new-issue"; readonly teamId: string }
  | { readonly kind: "description"; readonly issueId: string }
  | { readonly kind: "comment"; readonly issueId: string; readonly parentId: string | null };

/** The body a composer hands over. `title` is present for `new-issue` only. */
export interface DraftBody {
  readonly title: string | null;
  readonly body: string;
}

export type DraftState =
  | { readonly phase: "empty" }
  | { readonly phase: "dirty"; readonly id: string | null }
  | { readonly phase: "saving"; readonly id: string | null }
  | { readonly phase: "saved"; readonly id: string; readonly at: string }
  | { readonly phase: "unreachable"; readonly id: string | null; readonly attempts: number };

export interface DraftDeps {
  /** Task 83's relay calls. `list` is the author's own drafts; the Hub 404s on anyone else's. */
  readonly listDrafts: () => Promise<readonly Draft[]>;
  readonly createDraft: (scope: DraftScope, body: DraftBody) => Promise<Draft>;
  readonly updateDraft: (id: string, body: DraftBody) => Promise<Draft>;
  readonly deleteDraft: (id: string) => Promise<void>;
  /** Injected so no test sleeps. Returns a cancel. */
  readonly schedule: (fn: () => void, ms: number) => () => void;
  readonly now: () => number;
}
```

```ts
/** One open composer's handle. A composer holds exactly one and closes it on unmount. */
export interface DraftHandle {
  readonly scope: DraftScope;
  state(): DraftState;
  /** Called on every keystroke. Cheap: it stores and (re)arms the debounce, nothing else. */
  edit(next: DraftBody): void;
  /** Saves now and cancels the pending timer. Idempotent when clean. */
  flush(): Promise<void>;
  /** The composer submitted or the person discarded: delete the Hub record, stop the timer. */
  discard(): Promise<void>;
  /** Unmount. Cancels the timer; does NOT save (blur already did) and does NOT delete. */
  close(): void;
  subscribe(fn: (s: DraftState) => void): () => void;
}

export interface DraftController {
  /** Finds the draft of this scope or prepares an empty one. No Hub write on open (decision a). */
  open(scope: DraftScope, seed?: DraftBody): Promise<DraftHandle>;
  /** Decision (d)'s menu data, newest first. */
  list(): Promise<readonly DraftSummary[]>;
}

export interface DraftSummary {
  readonly id: string;
  readonly scope: DraftScope;
  readonly line: string;      // "New issue in Uploads" / "Comment on EP-142"
  readonly preview: string;   // first 80 chars of body, single line
  readonly editedAgo: string; // "3d ago" — lib/time.ts
  readonly expiresOn: string; // "27 Mar 2027" — a DATE, never a countdown (decision d)
}

export const IDLE_MS = 1_500;
export const RETRY_CEILING_MS = 30_000;

export function createDraftController(deps: DraftDeps): DraftController;
export function sameScope(a: DraftScope, b: DraftScope): boolean;
export function summarise(d: Draft, now: number): DraftSummary;
```

**The state machine, in words.** `edit` puts the handle in `dirty` and arms `IDLE_MS`. The timer
fires `save`. `save` sets `saving`, calls `createDraft` (first time) or `updateDraft`, and on
success sets `saved` with the Hub's `updatedAt`. On a transport error it sets `unreachable` with
`attempts + 1` and re-arms at `min(IDLE_MS * 2 ** attempts, RETRY_CEILING_MS)`. On a `409` it
re-reads the draft and keeps the local text (last writer wins, one author, two windows). If
`edit` arrives while `saving`, the handle stays `dirty` underneath and one more save is issued
when the first settles — **never two in flight**, so the Hub's `updatedAt` is monotonic.

#### The callout node and its round-trip: `work/markdown/callout.ts`, complete

```ts
// desktop/src/work/markdown/callout.ts
export type CalloutTone = "note" | "warn" | "stop";

export const CALLOUT_TONES: readonly CalloutTone[] = ["note", "warn", "stop"] as const;

/** GitHub alert syntax, so a callout degrades to a blockquote anywhere else (decision e). */
export const CALLOUT_MARKER: Readonly<Record<CalloutTone, string>> = {
  note: "[!NOTE]",
  warn: "[!WARNING]",
  stop: "[!CAUTION]",
};
```

```ts
/** Task 50's schema gains this node. Block, holds block content, one attribute. */
export const calloutNodeSpec = {
  content: "block+",
  group: "block",
  defining: true,
  attrs: { tone: { default: "note" as CalloutTone } },
  parseDOM: [
    { tag: "aside[data-callout]", getAttrs: (el: HTMLElement) => ({ tone: readTone(el) }) },
    // A pasted GitHub alert arrives as a blockquote whose first line is the marker.
    { tag: "blockquote", getAttrs: (el: HTMLElement) => markerTone(el.textContent ?? "") ?? false },
  ],
  toDOM: (node: { attrs: { tone: CalloutTone } }) => [
    "aside",
    { "data-callout": node.attrs.tone, role: "note", class: `callout callout--${node.attrs.tone}` },
    // The shape, never colour alone: a glyph element the CSS draws per tone.
    ["span", { class: "callout__glyph", "aria-hidden": "true" }],
    ["div", { class: "callout__body" }, 0],
  ],
} as const;

export function readTone(el: HTMLElement): CalloutTone;
export function markerTone(firstLine: string): { tone: CalloutTone } | null;
/** Markdown out: "> [!NOTE]\n> line\n> line". Nested blocks keep their own "> " prefix. */
export function calloutToMarkdown(tone: CalloutTone, innerMarkdown: string): string;
/** Markdown in: a blockquote whose first line is a marker becomes a callout; others stay quotes. */
export function markdownToCallout(block: string): { tone: CalloutTone; inner: string } | null;
```

The round-trip is **lossless both ways and is tested byte for byte**: `markdownToCallout(
calloutToMarkdown(t, inner))` equals `{tone: t, inner}` for every tone and for inner text that
itself contains a blockquote, a list, a code fence and a table (task 50's node). An unknown
marker — `[!TIP]`, which we do not have — stays a blockquote with its text intact rather than
being dropped or coerced to `note`.

#### *Copy as Markdown*: `work/markdown/copy.ts`, complete

```ts
// desktop/src/work/markdown/copy.ts
export interface CopySubject {
  readonly issueId: string;      // "is-7Q3FK2…" — the KEY (skeleton rule 1)
  readonly alias: string;        // "EP-142"     — display only
  readonly revision: number;     // the revision the person is looking at
  readonly revisionCount: number;
  readonly title: string;
  readonly bodyMarkdown: string;
  readonly hubOrigin: string;    // "https://hub.panacea.kpmg" — from the session, never hard-coded
}

/** Line one of every copy. The alias FIRST because a person reads it; the key after it. */
export function markdownHeadLine(s: CopySubject): string;
// → "**EP-142** · `issue:is-7Q3FK2…@3` — Uploads stall above 2 GB"

export function copyAsMarkdown(s: CopySubject): { text: string; html: string };
```

The `text` flavour, complete, for a two-revision issue:

```md
**EP-142** · `issue:is-7Q3FK2QW9N@3` — Uploads stall above 2 GB
<https://hub.panacea.kpmg/p/issues/is-7Q3FK2QW9N@3>

> [!WARNING]
> The 2 GB ceiling is the gateway's, not ours.

Reproduced on dev twice.
```

Three rules a reviewer can check. **One:** the key is present and the alias never stands alone,
so a paste survives a project rename (L-129). **Two:** the URL is `https://` to the Hub's page and
is **pinned to the revision** (`@3`), which is what gives L-98 — *what did they read?* — an
answer; task 93 owns the scheme and `toHubUrl`, and this function calls it rather than building a
URL itself. **Three:** no author name, no avatar, no timestamp is copied. Markdown carries the
text, and provenance is the key, not a rendered byline.

#### Wiring the two composers

`NewIssue.tsx` ⚑ and `Comments.tsx` ⚑ each hold one `DraftHandle` for the life of the composer:
`open` on mount (seeded from a prefill, task 93), `edit` from the editor's `onChange`, `flush` on
blur of the editor's root, `discard` on submit **after** the Hub confirms the issue or comment,
and `close` on unmount. The byline row gains one `--ink-3` status word driven by `state()`:
nothing when `empty`, `saving…` when `saving`, `saved 14:32` when `saved`, and decision (c)'s
sentence when `unreachable`. The word is rendered inside task 66's live region as `polite` and is
**not** announced more than once per state change.

`Comments.tsx` ⚑ also gains the *Copy as Markdown* item in a comment's overflow and in the issue
header's, calling `copyAsMarkdown` and writing both flavours with one `ClipboardItem`. Pasting
**into** the editor keeps each run's authorship class (task 49): the paste handler copies
`data-author` from the source HTML when it is ours and marks unmarked text `text·H`, because the
person is the one pasting — it never re-labels a run that already says `text·A`.

#### Tests to write first, each with the mutant it kills

`drafts.test.ts`, `callout.test.ts`, `copy.test.ts` and `DraftMenu.test.tsx` under vitest +
`@testing-library/react` (`desktop/package.json:26-28`), with `deps.schedule` and `deps.now`
driven by hand. Each is seen failing first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `typing a new issue writes nothing to the device` | after 40 `edit` calls and three debounce fires, `localStorage.setItem`, `sessionStorage.setItem` and `indexedDB.open` spies are never called; task 55's grep over `src/work/**` finds no storage identifier | a local autosave "just for safety" |
| `a draft survives a cleared replica` | `open` with an empty replica calls `listDrafts` and returns yesterday's text; the replica's own store is never read | the draft lived only in the zustand slice |
| `no draft of another person is ever held` | the replica fed a `df-` record whose `authorId` is not the session's drops it; `list()` returns only the author's | the visibility filter skips `df-` |
| `one save is in flight at a time` | `edit`, fire, `edit` while the promise is pending, settle → exactly two `updateDraft` calls, in order, and the second carries the later text | two concurrent `PUT`s; a lost keystroke |
| `blur saves and cancels the timer` | `edit` then `flush` → one call; the armed timer's cancel was invoked; a later fire is a no-op | a double save on blur |
| `an unreachable Hub is said, and retried with a ceiling` | three failures → state `unreachable` with `attempts` 1, 2, 3 and delays 1.5 s, 3 s, 6 s; the tenth is 30 s, not 768 s | a silent swallow; unbounded backoff |
| `a callout survives save and reload byte for byte` | each tone, inner text with a quote, a list, a fence and a table → `markdownToCallout(calloutToMarkdown(...))` is identical; the document's serialised Markdown is byte-identical after a parse/serialise cycle | the serialiser drops the node, or coerces the tone |
| `an unknown alert stays a blockquote` | `> [!TIP]\n> x` parses to a blockquote whose text still begins `[!TIP]` | coerced to `note`; text dropped |
| `copied Markdown carries the key and the revision` | `markdownHeadLine` contains `issue:is-` and `@3`; the URL is `https://` and ends `@3`; the string `panacea://` is absent | only `EP-142`; an unpinned link |
| `copy writes two flavours` | one `ClipboardItem` with `text/plain` and `text/html`; neither carries an author name or a timestamp | a single flavour; a rendered byline |
| `pasted agent text keeps its class` | HTML with `data-author="A"` pasted into a comment keeps `text·A`; unmarked HTML becomes `text·H` | re-labelled `text·H` (task 49's rule) |
| `the draft menu shows an age and an expiry date` | a 3-day-old draft renders `edited 3d ago` and `expires 27 Mar 2027`; no element updates on a timer | a live countdown on a resting surface |

#### Steps

- [ ] **Step 1: Check the base.** Task 83's `Draft` type and relay calls as merged; task 80's four
  draft routes reachable through the fake (task 73); task 50's schema file and slash-menu
  registration point; task 55's `noPersistence.test.ts` glob list. If task 50 has not merged,
  write `callout.ts` and its test against the node spec alone and leave the two ⚑ editor files to
  a follow-up commit — `drafts.ts` and `copy.ts` do not wait on it.
- [ ] **Step 2: The tests.** All twelve, plus the one-line widening of task 55's glob. Run
  `npm --prefix desktop test`: they fail.
- [ ] **Step 3: `drafts.ts`**, then `callout.ts`, then `copy.ts` — three pure modules, no React,
  each green before the next starts.
- [ ] **Step 4: The surfaces.** `DraftMenu.tsx`, then the composer wiring in `NewIssue.tsx` and
  `Comments.tsx`, then the two editor registrations.
- [ ] **Step 5: Green and commit by pathspec.** This task's suites, task 55's, task 49's paste
  suite and task 50's editor suite; `npm --prefix desktop run typecheck`; commit the thirteen
  files of *Files*, naming the five ⚑ as touched.

#### Acceptance

```bash
npm --prefix desktop test -- --run src/work/drafts.test.ts 2>&1 | tail -5
npm --prefix desktop test -- --run src/work/markdown 2>&1 | tail -5
npm --prefix desktop test -- --run src/work/replica/noPersistence.test.ts 2>&1 | tail -5
npm --prefix desktop test 2>&1 | tail -5
npm --prefix desktop run typecheck
grep -rn "localStorage\|sessionStorage\|indexedDB" desktop/src/work/ | wc -l          # 0
grep -rn "panacea://" desktop/src/work/markdown/copy.ts | wc -l                       # 0
```

Expected: each `--run` ends `Test Files  1 passed` / `2 passed` with no failures; the whole suite
passes; `typecheck` prints nothing and exits 0; both greps print `0`. **None has been run; none of
these files exists yet** (OD-27).

#### What this task does not claim

- **Not offline drafts.** With the Hub unreachable the composer says so (decision c) and keeps the
  text in memory until the window closes. There is no local queue, because a local queue is Hub
  text on disk (ADR-H1).
- **Not a Mermaid diagram.** L-127's diagram half stays the PARTIAL's named *later* part, as
  priced. Only the callout lands here.
- **Not a rich paste from Word or Confluence.** HTML paste is task 50's normaliser; this task only
  adds the callout's `parseDOM` and the authorship rule on top of it.
- **Not draft sharing.** A draft is private to its author and the Hub 404s to everyone else (task
  80); there is no *share a draft* surface and no seam for one.
- **Not the six-month expiry itself**, which is task 77's janitor pass. This task only *shows* the
  date the Hub returns.
- **No measured keystroke cost.** `edit` does no work beyond a store and a timer re-arm, but
  nobody has profiled it at a 20,000-character body.

#### Corrections to the scope

1. **`desktop/package.json` is not touched.** The scope allows a `prosemirror-*` addition if one
   is needed; none is (see *Files*), so the licence question does not arise and the file is not in
   this task's list.
2. **The draft record needs a scope, and the scope paragraph does not name one.** Without it
   *"reopen the composer and the text is there"* cannot be implemented, because nothing says which
   composer a `df-` belongs to. `DraftScope` is added here and must appear in task 72's Go record
   and task 83's TypeScript type. **If task 80 has already frozen the draft wire without it, this
   task's first step is that one additive field.**
3. **Task 55's no-persistence glob must widen.** It covers `src/work/replica/**` in its own scope;
   drafts are the first Hub text outside that directory, so the glob becomes `src/work/**`. One
   line, in task 55's file, named as touched.
4. **`copyAsMarkdown` calls task 93's `toHubUrl`** rather than composing a URL, so there is one
   place that knows the Hub's origin. That makes 93 a soft dependency of this task's copy half;
   the two are in the same slice and the fallback is a one-line local join if 93 lands after.

**Parity rows served:** the desktop halves of **L-33** (drafts kept, held in the Hub) and **L-44**
(autosave), jointly with task 80; **L-127**'s callout part; and **L-129** (*Copy as Markdown*,
carrying the key). Task 97 proves them.
