### Task 186: the in-app changelog; making it public is KPMG's call (L-254)

Stream **WL**, desktop and docs only, **no Hub change**, about 2 hours (an estimate, not a
measurement). Depends on task **174** (W5's proof), and uses task **31**'s `ViewHeader` (seam
**SW.13**, `task-31.md:93-98`), task **49**'s `Editor` (`EditorProps`, `task-49.md:522-528`), task
**20**'s `expectNoPrediction` and task **59**'s palette. 05 L-254 (*public changelog*, slice
`later`) and L-253 (the method, which is `AGENTIC-UX.md` and `DESIGN-SYSTEM.md`).

The changelog is **our product's own**, shipped inside the app, so a person can see what changed
in the version they are running. It is not Hub data (ADR-H1's rule that no client persists Hub
data does not bite), it needs no network, and there is **no public page**: publishing is KPMG's
decision, not a build item (05 §1 L-254).

**Decisions made here, so nobody re-derives them:**

1. **Entries are Markdown files in the repository, read at build time.** `docs/changelog/*.md`,
   one per release, each with front matter the loader validates. The desktop imports them through
   Vite's `import.meta.glob("../../../docs/changelog/*.md", { query: "?raw", import: "default",
   eager: true })`, so a bad entry fails `npm --prefix desktop test` and the build, never a
   person's screen. `README.md` in that folder is excluded by name.
2. **Only shipped work is listed** (`OD-27`). An entry dated after the build date, or whose version
   is above `desktop/package.json`'s (`0.1.0` on 2026-09-26, verified), is refused at load, so
   *coming soon* cannot be written. The build date is `__BUILD_DATE__`, a Vite `define` set from
   the build's clock; tests pass it explicitly.
3. **One Markdown renderer.** Each body renders through task 49's `Editor` with `readOnly`, so the
   changelog uses task 49's schema (CommonMark, `html: false`, images off, links limited to
   `LIVE_LINK`, `task-134.md:46-48`). A `<script>` in an entry is text.
4. **The *new* word is quiet.** The last version the person opened is kept in the desktop's local
   preferences (`PrefsSlice.changelogLastSeen`, persisted with the rest of the slice through
   `desktop/src/store/store.ts`'s `localStorage` storage); entries newer than it carry the word
   `new`, cleared when *What's new* is opened. There is **no toast, no dialog on update and no
   badge that counts**: one notification per person per 15 minutes is the product's rule
   (`docs/AGENTIC-UX.md:243`, anti-pattern 8) and a changelog is not worth spending it; the rail
   carries names, not numbers (`:245`, anti-pattern 10).
5. **Opened from the palette only.** *What's new* is one command in task 59's `buildCommands`; it
   opens a view in the main area. Nothing opens it on its own.
6. **Our own words.** Entries describe our product in our words (clean room, rule 4); the format
   borrows no one's changelog template.

**Files:**
- `docs/changelog/README.md` — the entry format, one screen (new)
- `docs/changelog/2026-09-26-0.1.0.md` — the first entry; its date is the date the task lands and
  its version `desktop/package.json`'s at that commit (new)
- `desktop/src/changelog/schema.ts`, `schema.test.ts` — `ChangelogEntry`, `parseEntry` (new)
- `desktop/src/changelog/load.ts`, `load.test.ts` — `loadChangelog`, `sortEntries` (new)
- `desktop/src/components/changelog/WhatsNew.tsx`, `WhatsNew.test.tsx`, `changelog.css` (new)
- `desktop/src/store/slices/prefsSlice.ts` ⚑ — `changelogLastSeen` and `setChangelogLastSeen`;
  `desktop/src/store/store.ts` ⚑ — `partialize` (`store.ts:34`) and its `Persisted` type gain the
  key. The store is flat and persist merges shallowly at the root (`store.ts:3-6`), so a new
  root key rehydrates as its default for an older snapshot; `PERSIST_VERSION` does not move
- `desktop/src/components/palette/commands.ts` ⚑ (task 59) — *What's new*
- `desktop/vite.config.ts` ⚑ and `desktop/vitest.config.ts` ⚑ — `define: { __BUILD_DATE__:
  JSON.stringify(new Date().toISOString().slice(0, 10)) }` in both, because `vitest.config.ts` is
  its own `defineConfig` from `vitest/config` and does not merge the Vite one (verified
  2026-09-26: no `mergeConfig` in either); `desktop/src/vite-env.d.ts` ⚑ — `declare const
  __BUILD_DATE__: string;`

#### The entry format (`docs/changelog/README.md`, complete)

```markdown
# Changelog entries

One file per release: `YYYY-MM-DD-<version>.md`. The desktop reads every file here at build time
and refuses the build if one is wrong. Only shipped work goes in: an entry dated after the build,
or for a version above `desktop/package.json`'s, fails the tests.

---
version: 0.2.0
date: 2026-10-14
items:
  - kind: new
    text: The Work lens groups issues by the criterion they serve.
  - kind: fixed
    text: A held move no longer drops when the list is filtered.
---
Optional Markdown body: a paragraph or two on why. No images, no raw HTML.

`kind` is `new`, `improved` or `fixed`. `text` is one line, at most 140 characters, no Markdown.
Say what changed for the person, in plain words, and nothing about what is coming.
```

#### Interfaces — complete: `desktop/src/changelog/schema.ts`

```ts
export type ChangeKind = "new" | "improved" | "fixed";
export interface ChangeItem { kind: ChangeKind; text: string }
export interface ChangelogEntry {
  version: string; // semver MAJOR.MINOR.PATCH, ≤ the app's version
  date: string;    // YYYY-MM-DD, ≤ the build date
  items: ChangeItem[];
  body: string;    // Markdown, possibly ""
  file: string;    // the source file's name, for error messages
}
export type EntryError =
  | { file: string; code: "no_front_matter" }
  | { file: string; code: "bad_version" | "version_above_app"; value: string }
  | { file: string; code: "bad_date" | "date_after_build"; value: string }
  | { file: string; code: "bad_kind"; value: string }
  | { file: string; code: "bad_text"; index: number }
  | { file: string; code: "no_items" }
  | { file: string; code: "name_mismatch"; value: string }; // file name ≠ `${date}-${version}.md`

export const SEMVER = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
export const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;
export const KINDS: readonly ChangeKind[] = ["new", "improved", "fixed"];
export const MAX_TEXT = 140;

/** −1, 0 or 1; numeric per part, so 0.10.0 > 0.9.0. */
export function compareSemver(a: string, b: string): -1 | 0 | 1;

/** Parses one file. The front matter is the restricted form README.md shows (three
 *  keys, `items` a list of two-key maps), read by a small hand parser, not a YAML
 *  library, so nothing outside that form is accepted. */
export function parseEntry(
  file: string, raw: string, ctx: { appVersion: string; buildDate: string },
): { ok: true; entry: ChangelogEntry } | { ok: false; error: EntryError };
```

#### Interfaces — complete: `desktop/src/changelog/load.ts`

```ts
import pkg from "../../package.json";
import { parseEntry, compareSemver, type ChangelogEntry, type EntryError } from "./schema";

const RAW = import.meta.glob("../../../docs/changelog/*.md", {
  query: "?raw", import: "default", eager: true,
}) as Record<string, string>;

/** Newest first by date, then by version; stable for equal pairs. */
export function sortEntries(entries: readonly ChangelogEntry[]): ChangelogEntry[];

/** Pure core: tests pass their own files and context. README.md is skipped by name.
 *  Any EntryError fails the whole load: a changelog with a wrong entry is not shown. */
export function loadFrom(
  files: Record<string, string>, ctx: { appVersion: string; buildDate: string },
): { ok: true; entries: ChangelogEntry[] } | { ok: false; errors: EntryError[] };

/** The shipped changelog. Throws on any error; load.test.ts calls it, so a bad
 *  entry in docs/changelog fails `npm --prefix desktop test`. */
export function loadChangelog(): ChangelogEntry[] {
  const r = loadFrom(RAW, { appVersion: pkg.version, buildDate: __BUILD_DATE__ });
  if (!r.ok) throw new Error(`changelog: ${r.errors.map((e) => `${e.file} ${e.code}`).join(", ")}`);
  return r.entries;
}
```

Importing `package.json` needs `resolveJsonModule`, which the desktop has
(`desktop/tsconfig.json:12`, verified 2026-09-26). `buildDate` is UTC's date, so an entry dated
today in Sydney before 10:00 fails until UTC reaches that date; the README says so, and dating an
entry the day it lands in UTC is the rule.

#### Interfaces — complete: the prefs delta (`prefsSlice.ts` ⚑)

```ts
// PrefsSlice gains:
/** The newest changelog version the person has opened *What's new* on, or null for never.
 *  Local only; never sent anywhere (task 186). */
changelogLastSeen: string | null;
setChangelogLastSeen(version: string): void; // keeps the higher of the stored and the given
// initialPrefsState gains: changelogLastSeen: null
```

#### Interfaces — complete: `desktop/src/components/changelog/WhatsNew.tsx`

```tsx
import { ViewHeader } from "../subrail/ViewHeader"; // task 31's module path; UNVERIFIED until 31 lands
import { Editor } from "../editor/Editor";          // task 49
import { docFromAuthored } from "../editor/authorClass";
import type { ChangelogEntry } from "../../changelog/schema";

export const KIND_WORD: Readonly<Record<ChangelogEntry["items"][number]["kind"], string>> = {
  new: "New", improved: "Improved", fixed: "Fixed",
};

export interface WhatsNewProps {
  entries: readonly ChangelogEntry[]; // loadChangelog(), newest first; injected for tests
  lastSeen: string | null;            // the store's changelogLastSeen, read once at mount
  onSeen(version: string): void;      // setChangelogLastSeen, called once on mount
}

/** The view. On mount it notes which entries are above lastSeen (kept in a ref, so
 *  the `new` words stay for this visit), then calls onSeen(entries[0].version).
 *  Each entry: an <article> with its version and date in mono, tabular-nums, the
 *  items as a list with KIND_WORD in the muted tone, then the body through
 *  <Editor readOnly value={docFromAuthored({text: body, author: "human"})}
 *  onCommit={() => {}} ariaLabel={`Notes for ${version}`}/> when body is not "".
 *  No fetch, no toast, no aria-live region, no count anywhere. */
export function WhatsNew(props: WhatsNewProps): JSX.Element;
```

`docFromAuthored` and `AuthoredText` are task 49's (`task-49.md:111-127`); the `author` value for
our own shipped text is whatever task 49's `AuthoredText` names a person's text (**UNVERIFIED**
spelling until part 49's `authorClass.ts` lands; `"human"` above is a placeholder for it).

The header is `<ViewHeader title="What's new" route={null} />`: no route badge, because nothing
here reaches inference (`task-31.md:93-98`).

#### `changelog.css` (complete)

```css
.cl { max-inline-size: 72ch; padding: var(--s-4) var(--s-5); }
.cl-entry { padding-block: var(--s-4); }
.cl-entry + .cl-entry { box-shadow: 0 -1px 0 var(--ring); }
.cl-head { display: flex; gap: var(--s-3); align-items: baseline; }
.cl-version, .cl-date { font-family: var(--mono); font-variant-numeric: tabular-nums; }
.cl-date { color: var(--fg-muted); }
.cl-new { font-family: var(--sans); color: var(--fg-muted); font-size: var(--t-xs); }
.cl-items { margin: var(--s-2) 0 0; padding: 0; list-style: none; font-family: var(--sans); }
.cl-kind { color: var(--fg-muted); min-inline-size: 7ch; display: inline-block; }
```

Tokens only (`--s-*`, `--t-*`, `--mono`, `--sans`, `--fg-muted`, `--ring`; SW.11 and task 26's
names, **UNVERIFIED** spellings until 26 lands). No gradient, no motion, no border: the divider is
a 1px ring shadow (`docs/DESIGN-SYSTEM.md`). OD-28's faces arrive through `--sans` and `--mono`.

#### The palette (`commands.ts` ⚑)

`buildCommands(state)` (`desktop/src/components/palette/commands.ts:72`, verified) gains one
command, `{ id: "whats-new", label: "What's new", run: () => openView("whats-new") }`, with no
`hint` (no chord). `openView` is task 59's name for opening a main-area view (**UNVERIFIED**; part
59 is not written).

**Correction to the line above, so the build does not trip on it:** the id is
`"verb:whats-new"`, not `"whats-new"`. The existing test *returns the fixed block order* splits
every id at `:` (`desktop/src/components/palette/commands.test.ts:45-46`, verified), and the
verbs are `verb:layout`, `verb:rail`, `verb:machines` (`commands.ts:85-131`). `Command` also
requires `glyph` (`commands.ts:16`); the row uses the glyph the other view verbs use (task 59
picks it). The row sits in the verb block after *Machines*, so the block order does not change.

#### Tests to write first, part 1 — `schema.test.ts` and `load.test.ts` (pure)

`ctx` is `{ appVersion: "0.2.0", buildDate: "2026-10-14" }` unless a row says otherwise.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `refuses an entry dated after the build` | `date: 2026-10-15` → `{ok: false, error: {code: "date_after_build", value: "2026-10-15"}}` | a *coming soon* entry |
| `accepts an entry dated the build day` | `date: 2026-10-14` → `ok: true` | a `<` written for `≤` |
| `refuses a version above the app` | `version: 0.3.0` → `version_above_app` | an unshipped version |
| `compares semver per part` | `compareSemver("0.10.0", "0.9.0") === 1`; `("1.0.0", "1.0.0") === 0` | a string compare |
| `refuses an unknown kind` | `kind: coming` → `bad_kind`, `value: "coming"` | a loose schema |
| `refuses long or Markdown text` | 141 characters → `bad_text, index: 0`; `text: "**bold**"` → `bad_text` | an item that renders as markup outside the `Editor` |
| `refuses an empty list` | `items:` with no rows → `no_items` | a release with nothing in it |
| `refuses a name that disagrees` | file `2026-10-14-0.1.9.md` holding `version: 0.2.0` → `name_mismatch` | two files for one release |
| `refuses keys outside the form` | a fourth key `upcoming:` → `no_front_matter` | a YAML-library parse that accepts anything |
| `returns newest first` | three entries dated 09-26, 10-14, 10-01 → 10-14, 10-01, 09-26 | an unsorted list |
| `breaks a date tie by version` | two entries dated 10-14, `0.2.0` and `0.2.1` → `0.2.1` first | a sort on date only |
| `skips README.md by name` | `loadFrom` over `{ "…/README.md": "# Changelog entries", … }` → no error | the format page read as an entry |
| `one bad entry fails the load` | two good, one dated tomorrow → `ok: false`, one error, no entries | a partial changelog that hides the bad one |
| `the shipped changelog loads` | `loadChangelog()` does not throw and returns ≥ 1 entry | a broken entry in `docs/changelog/` reaching a build |

#### Tests to write first, part 2 — `WhatsNew.test.tsx`, `prefsSlice` and `commands.test.ts` ⚑

`entries` are three fixtures, `0.2.1`, `0.2.0`, `0.1.0`, newest first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `no prediction` | `expectNoPrediction(container)` (task 20, SW.8) | a *coming soon* or *next release* line |
| `a script in a body is text` | body `<script>window.x=1</script>` → `window.x` undefined; the text `<script>` is in the DOM as text; no `script` element under the view | a second renderer that bypasses task 49's schema |
| `marks only what is above lastSeen` | `lastSeen: "0.2.0"` → the word `new` once, on `0.2.1`'s article | a marker on every entry |
| `a first visit marks everything` | `lastSeen: null` → `new` on all three | a null read as "seen" |
| `opening records the newest` | `onSeen` called once, with `"0.2.1"`; a re-render does not call it again | a marker that never clears, or a write per render |
| `the marks stay for this visit` | after `onSeen`, re-render with `lastSeen: "0.2.1"` → `new` still on `0.2.1` | marks that vanish while the person reads |
| `makes no fetch` | `vi.spyOn(globalThis, "fetch")` → not called during mount and a re-render | a remote or public feed |
| `raises nothing` | no `[aria-live]`, no `role="status"`, no `role="alertdialog"`, and the toast store (task 26's, **UNVERIFIED** name) receives no entry | an update popup |
| `numbers are tabular` | each `.cl-version` and `.cl-date` has class names that resolve to `tabular-nums` in `changelog.css` (a string check on the CSS file) | a version column that jitters |
| `setChangelogLastSeen keeps the higher` | stored `0.2.1`, given `0.2.0` → stays `0.2.1`; stored `null`, given `0.1.0` → `0.1.0` | an older build rolling the marker back |
| `the key persists` | `partialize(state)` holds `changelogLastSeen` | a marker lost on restart |
| `lists What's new` ⚑ | `buildCommands(state)` has one row `id: "verb:whats-new"`, label `What's new`, no `hint`; running it opens the view | a command registered nowhere |
| `block order holds` ⚑ | the existing *returns the fixed block order* test passes unchanged | an id outside the verb block |

#### Steps

- [ ] **Step 1: confirm the base.** Tasks 20, 26, 31, 49 and 59 committed on this branch; read
  task 49's `authorClass.ts` for the `author` spelling and task 59's name for opening a view, and
  replace the two placeholders above. Stop and report if 49 or 59 has not landed.
- [ ] **Step 2: the define.** Add `__BUILD_DATE__` to `vite.config.ts`, `vitest.config.ts` and
  `vite-env.d.ts`; `npm --prefix desktop run typecheck` is exit 0 before anything else moves.
- [ ] **Step 3: pure tests first.** Write `schema.test.ts` and `load.test.ts` (part 1); they fail
  to compile. Write `schema.ts` (the hand parser, `compareSemver`), then `load.ts`; run until
  green except `the shipped changelog loads`.
- [ ] **Step 4: the entries.** Write `docs/changelog/README.md` (the block above, plus the UTC
  sentence) and the first entry, dated the day this lands in UTC, version `desktop/package.json`'s
  at that commit, its items saying what shipped in our words. `the shipped changelog loads` goes
  green.
- [ ] **Step 5: the preference.** The two `prefsSlice` rows first, then `changelogLastSeen`,
  `setChangelogLastSeen` and the `partialize` key. `PERSIST_VERSION` stays `1`
  (`store.ts:17`, verified).
- [ ] **Step 6: the view.** Part 2's `WhatsNew` rows first, then `WhatsNew.tsx` and
  `changelog.css`; run until green.
- [ ] **Step 7: the palette.** The two `commands.test.ts` rows, then the row in `buildCommands`.
- [ ] **Step 8: acceptance**, then commit by path, each message ending with the `Co-Authored-By`
  line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
npm --prefix desktop test -- changelog
# expect: every test passes, 23 of them (14 in schema and load, 9 in WhatsNew; the filter
#         matches the two changelog/ folders)
npm --prefix desktop test -- prefsSlice commands
# expect: every test passes, including the 2 prefs rows and the 2 palette rows
npm --prefix desktop test
# expect: every test passes (the whole suite; a bad entry in docs/changelog fails here)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rn "fetch(" desktop/src/changelog desktop/src/components/changelog
# expect: no output
```

#### What this task does NOT claim

- A public changelog, a web page or a feed: publishing is KPMG's call (05 §1 L-254).
- A weekly cadence: that is a team habit, not code.
- That any entry's words are true: each release's review owns that.
- That entries link to Hub issues or releases on the provenance thread; that would need the
  product's own Hub org (**UNVERIFIED**).
- That *What's new* looks right: that is the owner's eyes (`CLAUDE.md`, working agreements), and
  task 187 records it as not run until the owner has looked.

**Parity.** Closes **L-254** (slice `later`, `05-design.md:446`, verified); the command and output
are recorded by task 187.

#### Gaps found while writing this part

1. **The storage key.** The scope names a `localStorage` key `panacea.changelog.lastSeen`; this
   part keeps the value in `PrefsSlice`, persisted under the store's one key `panacea-app.cockpit`
   (`store.ts:16`, verified), so the desktop has one persisted object, not two.
2. **The palette id.** The scope's and this part's first `"whats-new"` would break the block
   order test; the correction above makes it `verb:whats-new`.
3. **Tasks 26 and 59 are not written.** The toast store's name, `openView`, and the token names
   are **UNVERIFIED** until they land.
4. **The File map has no WL rows** for `docs/changelog/`, `desktop/src/changelog/` or
   `components/changelog/`; they should be listed.

