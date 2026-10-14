### Task 179: Syntax-aware diff and file categories through a permissively licensed differ, behind the licence gate (L-171)

Stream **WL**, desktop, about 3 hours (an estimate, not a measurement). Depends on task 174 (W5's
proof, so WL starts on a proven base) and on phase 3 tasks **P3-16** (the Files read model:
per-path hunks with tier-1 back-links) and **P3-21** (the Files lens) sync-merged into
`feat/work-layer`. If their files are not on the sync base, this task **waits**; it never writes a
second copy of a phase 3 file (rule H2). The owner phase wins every conflict.

05 row L-171 (`05-design.md:333`): *"Later, through a permissively licensed differ (licence gate,
OD-11 rule 3)"*. Today P3-21's `HunkView` shows git's hunk line by line (P3 `task-21.md:57` at
`94f954d`): a renamed identifier paints the whole line red and green, and a re-indented block looks
like a rewrite. This task makes the change visible at the **token**, inside git's own hunk, and
names each path's **category** so a generated lock file stops competing with source for the eye.

**What stays git's.** Hunk boundaries, line numbers and each hunk's back-link are P3-16's, and this
task changes none of them. The token diff runs **inside one hunk** only: `before` is the hunk's
context and `-` lines, `after` its context and `+` lines. So a hunk still has exactly one back-link
(P3-16 `task-16.md:88-120`), which task 180 depends on.

**The invariant that makes the view honest.** For every hunk, the concatenated text of the old
side's spans equals `before`, and of the new side's spans equals `after`, byte for byte. A token
view may choose a finer grain than git; it may never lose, add or reorder a character. The tests
check it on every fixture.

**Two corrections to the scope, made here so nobody re-derives them.**

1. **"A parse error falls back" needs a definition, or it fires on every hunk.** A hunk is a
   fragment: it starts mid-function and ends before the closing brace. Lezer is error-tolerant and
   marks the missing brace with a **zero-length** error node; that is normal for a fragment and
   the tokens are still right. What makes tokens wrong is a **non-empty** error node: text the
   tokenizer skipped because it could not read it. So the rule is: *any error node that covers at
   least one character* sends the hunk to plain mode with the reason `parse error`. A zero-length
   one does not. Test rows pin both.
2. **Plain mode shows git's lines, not a second line differ.** The scope said the fallback is
   jsdiff's line diff. Git already computed the hunk's line diff (P3-16's `Line.Kind`), and it is
   the diff that lands; a second line differ could disagree with git about which line moved. So
   plain mode maps git's `" "`, `"+"`, `"-"` to `same`, `add`, `del` with one whole-line span, and
   jsdiff is used only for the token arrays. The label is still *plain diff*, never silent.

**Why jsdiff and Lezer, and not difftastic.** `diff` (jsdiff) is BSD-3-Clause; the `@lezer/*`
grammars are MIT (both **UNVERIFIED at the pin** until the licence test below reads the installed
`package.json` files; that test is the verification). difftastic (MIT) was considered and is not
used: its JSON display is documented as unstable (**UNVERIFIED** at any pin), and it would be a
second binary to ship and sign in the desktop. Lezer runs in the renderer with no binary at all.

**The licence gate.** OD-11 rule 3 names *"a licence gate in CI"*
(`docs/superpowers/planning/2026-09-23-owner-decisions.md:96`, verified in this worktree). No such
gate is in the tree at this writing (this worktree at `0901bab9` has no `.github/` directory, and
`grep -rln "licen" desktop/src/test/` finds nothing; **re-check at the sync base**). So this task adds `desktop/src/test/licences.test.ts`: it walks the
new dependencies and **their dependencies** in `desktop/node_modules`, reads each installed
`package.json`, and fails on any licence outside **MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause and
ISC**. The allow-list is a named export so the CI gate's owner can adopt it unchanged; the return
of this part says so.

**Files** (every phase 3 path is **UNVERIFIED until merge**; the phase 3 part that plans it is
cited):
- `desktop/src/diff/tokens.ts` (+ `tokens.test.ts`) — `langFor`, `tokenize` (new)
- `desktop/src/diff/syntaxDiff.ts` (+ `syntaxDiff.test.ts`) — `syntaxDiff`, `hunkSides` (new)
- `desktop/src/diff/categories.ts` (+ `categories.test.ts`) — `CATEGORY_RULES`, `categorize`,
  `CATEGORY_DISPLAY_ORDER`, `collapsedByDefault` (new; task 180 imports the display order)
- `desktop/src/diff/licences.ts` — `ALLOWED_LICENCES`, `checkLicences` (new; pure over a root dir)
- `desktop/src/test/licences.test.ts` — the gate over the real `node_modules` and over fixtures (new)
- `desktop/src/test/fixtures/licences/node_modules/{ok-mit,gpl-thing,no-licence,or-expr,and-expr}/package.json`
  and `desktop/src/test/fixtures/licences-transitive/node_modules/{ok-mit,gpl-thing}/package.json`
  — the planted fixtures (new)
- `desktop/src/diff/bench.ts` — the one timing script, run by hand, asserting nothing (new)
- `desktop/package.json`, `desktop/package-lock.json` — `diff` and the `@lezer/*` grammars (modified)
- `desktop/src/components/lens/files/HunkView.tsx` ⚑ (P3 `task-21.md:14`, `:57`) — spans, the
  *plain diff* label, whitespace dimmed (modified)
- `desktop/src/components/lens/files/PathList.tsx` ⚑ (P3 `task-21.md:14`) — the category chip and
  its hover rule (modified)
- `desktop/src/components/lens/files/PathDiff.tsx` ⚑ (P3 `task-21.md:14`) — `generated` collapsed
  with its line count; `migration` has no collapse control (modified)
- `desktop/src/components/lens/files/files.css` ⚑ (P3 `task-21.md:15`) — `.tok-add`, `.tok-del`,
  `.tok-ws`, `.cat-chip` (modified)
- `desktop/src/components/lens/files/syntax.test.tsx` — the view rows, each calling
  `expectNoPrediction` (task 20) (new)

**Dependencies** (`desktop/package.json`, `dependencies`, added with `npm install --save-exact`
so the pin is the version the licence test read; the versions are whatever is current at install
and are recorded by the lock file, **UNVERIFIED here**): `diff`, `@lezer/common`,
`@lezer/javascript` (JavaScript, TypeScript and JSX through its dialects), `@lezer/go`,
`@lezer/python`, `@lezer/json`, `@lezer/css`, `@lezer/html`, `@lezer/markdown`. If the installed
`diff` ships no type declarations, `@types/diff` (MIT) goes into `devDependencies` and into the
gate's root list; which `diff` major ships its own types is **UNVERIFIED here**.

#### Interfaces — complete

```ts
// desktop/src/diff/tokens.ts
import type { Tree } from "@lezer/common";

/** The languages this task tokenises. Anything else is plain mode, reason "unknown language". */
export type Lang = "javascript" | "typescript" | "jsx" | "tsx" | "go" | "python" | "json" | "css" | "html" | "markdown";

/** One token. `text` is the exact slice [from, to) of the input; `ws` is true for a run of
 *  spaces and tabs (never a newline); a newline is always its own token, text "\n". */
export interface Token { from: number; to: number; text: string; ws: boolean }

/** The outcome of tokenising: tokens that concatenate to the input exactly, or a refusal. */
export type Tokenized =
  | { ok: true; tokens: Token[] }
  | { ok: false; reason: "parse error" };

/** Extension to language, by the path's final extension, lower-cased. "" and unknown → null.
 *  .js .mjs .cjs → javascript; .ts .mts .cts → typescript; .jsx; .tsx; .go; .py; .json;
 *  .css; .html .htm; .md .markdown. No sniffing of content, no shebang reading. */
export function langFor(path: string): Lang | null;

/** Parses `text` with the language's Lezer parser and returns its leaf tokens. Leaves are the
 *  tree's nodes with no children; the text between two leaves (Lezer does not keep every
 *  punctuation mark or blank as a node) is split by GAP_RE, so no character is ever dropped.
 *  Any error node with to > from refuses with "parse error"; a zero-length error node (a
 *  fragment ending mid-structure) does not. */
export function tokenize(text: string, lang: Lang): Tokenized;

/** The gap splitter: a run of spaces or tabs, one newline, a run of word characters, or one
 *  other character. Exported so the test can prove it covers every code point. */
export const GAP_RE: RegExp; // /[ \t]+|\r?\n|[\p{L}\p{N}_$]+|[^]/gu

/** Exported for the test only: the Lezer tree for (text, lang), so a row can plant a
 *  non-empty error node and check the refusal without a grammar-specific accident. */
export function parseFor(text: string, lang: Lang): Tree;
```

```ts
// desktop/src/diff/syntaxDiff.ts
import type { Hunk } from "../api/convergence"; // P3-18 (task-18.md:166-169), the mirror of P3-16s Hunk
import type { Lang } from "./tokens";

export type SpanKind = "same" | "add" | "del" | "ws";
/** A span is [from, to) in its TokenLine's `text`. "ws" is a whitespace-only difference, drawn dimmed. */
export interface Span { from: number; to: number; kind: SpanKind }

export type LineKind = "same" | "add" | "del" | "change";
export interface TokenLine {
  kind: LineKind;
  /** "both" only for kind "same"; a "change" is emitted as an "old" line then a "new" line. */
  side: "old" | "new" | "both";
  oldNo: number | null; // 1-based line number in the old file, from Hunk.OldStart
  newNo: number | null; // 1-based line number in the new file, from Hunk.NewStart
  text: string;         // the line without its newline
  spans: Span[];        // cover [0, text.length) exactly, in order, no gaps
  /** true when every add, del or ws span on the line is whitespace: a re-indent, not a rewrite. */
  wsOnly: boolean;
}

export type DiffMode = "syntax" | "plain";
export type PlainReason = "" | "unknown language" | "parse error" | "too large";
export interface SyntaxDiff { mode: DiffMode; reason: PlainReason; lang: Lang | null; lines: TokenLine[] }

/** Above this many tokens on either side the hunk is plain, reason "too large". A named
 *  constant, not a speed claim: bench.ts records what it costs, and the owner may move it. */
export const MAX_SYNTAX_TOKENS = 20_000;

/** before = context + "-" lines; after = context + "+" lines; each line ends "\n". */
export function hunkSides(hunk: Hunk): { before: string; after: string };

/** The whole differ: tokenise both sides, diffArrays over token text, cut back into lines.
 *  Plain mode maps git's own lines (see correction 2) and never calls diffArrays. */
export function syntaxDiff(hunk: Hunk, path: string): SyntaxDiff;
```

P3-18's `DiffLine` (`task-18.md:160`) is git's line, `{kind: " " | "+" | "-", text}`; this
module's output line is named `TokenLine` so the two never meet under one name.

```ts
// desktop/src/diff/categories.ts
export type Category = "source" | "test" | "migration" | "config" | "docs" | "generated";

/** One path rule. `glob` is what the hover shows, in the scope's own words; `re` is what runs. */
export interface CategoryRule { category: Category; glob: string; re: RegExp }

/** MATCH order: first match wins, so the narrow rules come before the wide ones.
 *  go.sum is "generated", never "config"; *.gen.ts is "generated", never "source";
 *  package-lock.json is "generated", never "config" through *.json. "source" is the default,
 *  with no rule, so a path no rule names is source and says "no rule matched" on hover. */
export const CATEGORY_RULES: readonly CategoryRule[] = [
  { category: "generated", glob: "go.sum",             re: /(^|\/)go\.sum$/ },
  { category: "generated", glob: "package-lock.json",  re: /(^|\/)package-lock\.json$/ },
  { category: "generated", glob: "*.gen.ts",           re: /\.gen\.ts$/ },
  { category: "migration", glob: "**/migrate/sql/*.sql", re: /(^|\/)migrate\/sql\/[^/]+\.sql$/ },
  { category: "test",      glob: "*_test.go",          re: /_test\.go$/ },
  { category: "test",      glob: "*.test.ts",          re: /\.test\.ts$/ },
  { category: "test",      glob: "*.test.tsx",         re: /\.test\.tsx$/ },
  { category: "test",      glob: "test/**",            re: /(^|\/)test\// },
  { category: "config",    glob: ".github/**",         re: /(^|\/)\.github\// },
  { category: "config",    glob: "deploy/**",          re: /(^|\/)deploy\// },
  { category: "config",    glob: "*.json",             re: /\.json$/ },
  { category: "config",    glob: "*.yaml",             re: /\.yaml$/ },
  { category: "config",    glob: "*.toml",             re: /\.toml$/ },
  { category: "config",    glob: "*.tf",               re: /\.tf$/ },
  { category: "docs",      glob: "*.md",               re: /\.md$/ },
];

/** DISPLAY order, which is not the match order: what a reviewer reads first. Task 180 sorts
 *  inside each criterion group by this list. */
export const CATEGORY_DISPLAY_ORDER: readonly Category[] = ["source", "test", "migration", "config", "docs", "generated"];

/** The path's category and the rule that chose it (null for the default "source"). Paths are
 *  repository-relative with "/" separators, as P3-16 returns them; case is kept, not folded. */
export function categorize(path: string): { category: Category; rule: CategoryRule | null };

/** The hover text: 'generated, because it matches "go.sum"' or 'source: no rule matched'. */
export function categoryWhy(path: string): string;

/** generated → true; every other category → false. A migration is never collapsed, and the
 *  lens renders no collapse control for it at all (a hidden schema change is a real risk). */
export function collapsedByDefault(c: Category): boolean;
```

```ts
// desktop/src/diff/licences.ts — node-only (fs, path); imported by the test, never by the app
/** OD-11 rule 3's list, as SPDX identifiers. The CI gate's owner adopts this array unchanged. */
export const ALLOWED_LICENCES: readonly string[] = ["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC"];

/** The dependency roots this task added. The walk follows each one's `dependencies` (not its
 *  devDependencies, which are never installed for a consumer), so a transitive GPL fails too. */
export const GATED_ROOTS: readonly string[] = [
  "diff", "@lezer/common", "@lezer/javascript", "@lezer/go", "@lezer/python",
  "@lezer/json", "@lezer/css", "@lezer/html", "@lezer/markdown",
];

export interface LicenceFinding { name: string; version: string; licence: string; dir: string }
export interface LicenceReport { checked: LicenceFinding[]; violations: LicenceFinding[]; missing: string[] }

/** Reads <root>/node_modules/<name>/package.json for each root and, recursively, each of its
 *  `dependencies`, resolving a nested <pkg>/node_modules/<dep> before the flat one, as Node does.
 *  A package's licence is its `license` string, or its legacy `licenses[].type` joined " OR ".
 *  An SPDX expression is allowed when, after removing parentheses, every "AND" term is allowed
 *  and at least one "OR" alternative inside each term is. No field, "UNLICENSED" or "SEE LICENSE
 *  IN …" is a violation, never a pass. A root that is not installed goes to `missing`. */
export function checkLicences(root: string, roots: readonly string[]): LicenceReport;
```

#### The differ, step by step (`syntaxDiff`)

1. `lang = langFor(path)`. Null → plain, reason `unknown language`.
2. `{before, after} = hunkSides(hunk)`. Tokenise each. Either refuses → plain, reason
   `parse error`. Either side over `MAX_SYNTAX_TOKENS` → plain, reason `too large`.
3. `diffArrays(oldTokens, newTokens, { comparator: (a, b) => a.text === b.text })` from `diff`
   (jsdiff's array diff). Each change is `same`, `removed` or `added`.
4. Walk the changes, filling one pending old line and one pending new line with marked tokens.
   A `same` token goes to both; a removed one to the old line only; an added one to the new line
   only. A **newline token closes a line**: a removed `\n` closes the old line into a pending
   deletions list, an added `\n` closes the new line into a pending additions list, and a `same`
   `\n` closes both.
5. On a `same` newline, the two closed lines are compared. Identical marks, all `same` → one line
   `{kind: "same", side: "both"}`. Otherwise the pending deletions are emitted first, then the
   pending additions, then the pair as `change` (old, then new). A line whose every non-whitespace
   token is removed is `del`; every non-whitespace token added is `add`.
6. Spans are the merged runs of marks within a line: `same`; `del` on the old side and `add` on
   the new; and `ws` for a changed token whose `ws` is true. `wsOnly` is true when the line has at
   least one non-`same` span and every one of them is `ws`.
7. Line numbers count from `hunk.oldStart` and `hunk.newStart`: an old or both line takes the next
   old number, a new or both line the next new number.
8. The invariant is asserted in development builds only (`import.meta.env.DEV`): the old side's
   texts joined with `\n` must equal `before` without its final newline, and likewise the new
   side. A breach throws in tests and logs once in production, never repairs.

Plain mode is step 7 alone over git's lines: `" "` → `same`/`both`, `"-"` → `del`/`old`,
`"+"` → `add`/`new`, each with one span over the line and `wsOnly: false`.

#### The view (P3-21's files, **UNVERIFIED until merge**)

- **`HunkView`** renders `syntaxDiff(hunk, path)` instead of `hunk.lines`. A span is a `<span>`
  with class `tok-add`, `tok-del` or `tok-ws`, rendered as **text, never HTML** (P3-21's rule for
  the drawer applies to every line). The line's gutter keeps P3-21's `+`/`-` marks; a `change`
  pair gets both. When `mode` is `plain`, the hunk header carries the words *plain diff* and,
  after a middle dot, the reason: *plain diff · unknown language*, *plain diff · parse error*,
  *plain diff · too large*. When `mode` is `syntax`, the header names the language in words
  (*TypeScript*), so the reader knows which grammar cut the tokens. `wsOnly` lines are drawn with
  `.tok-ws` and the header adds *whitespace only* when every changed line in the hunk is `wsOnly`.
- **`PathList`** gains a chip per row: the category's word, `title` and `aria-describedby` both
  set to `categoryWhy(path)`. The chip is a word, never a colour alone (DESIGN-SYSTEM's colour
  rule): *generated*, *migration*, *test*, *config*, *docs*; `source` shows no chip, since it is
  the default and a chip on every row is noise.
- **`PathDiff`** collapses a `generated` path by default into one row: *generated · 1,284 lines
  changed · show*. The count is the sum of `+` and `-` lines across the path's hunks, measured
  from P3-16's lines, `tabular-nums`. *show* expands it for this path in this session only. A
  `migration` path renders no collapse control, and its header says *migration* in `--need`'s
  ink (`tokens.css:75`: *needs you*; not `--fail`, since nothing has failed), the word carrying it too.
- **`files.css`** adds, and only adds, four rules. No token is invented: the marks use P3-21's
  `--diff-add` and `--diff-del` (P3 `task-21.md:16`, added only if absent) at a stronger mix, and
  whitespace uses `--ink-3` with no background.

```css
/* files.css — appended; transform/opacity-only motion rule unaffected (no motion here) */
.tok-add { background: color-mix(in oklch, var(--diff-add) 55%, transparent); border-radius: 2px; }
.tok-del { background: color-mix(in oklch, var(--diff-del) 55%, transparent); border-radius: 2px; }
.tok-ws  { color: var(--ink-3); background: none; text-decoration: underline dotted var(--line-strong); }
.cat-chip { font: 500 11px/16px var(--sans); color: var(--ink-2); border-radius: var(--r-chip);
            box-shadow: var(--e-hair); padding: 0 6px; }
```

#### Tests first — each with the mutant it kills

A test hunk is built by one helper in `syntaxDiff.test.ts`, `hunk(oldStart, newStart, lines)`,
where `lines` is P3-18's `DiffLine[]` and `back` is a fixed tier-1 `BackLink`.

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `renamed identifier marks one token` (`syntaxDiff.test.ts`) | `.ts` hunk, `-const total = sum(a)` / `+const grandTotal = sum(a)`: two `change` lines; the old line's only non-`same` span is `total` (`del`), the new line's only one is `grandTotal` (`add`) | a whole-line change |
| `re-indented block is whitespace only` | `.go` hunk re-indenting three lines by one tab: every changed line is `change` with `wsOnly: true` and only `ws` spans; the header helper says *whitespace only* | a re-indent shown as a rewrite |
| `unknown extension says plain diff` | `notes.xyz`: `mode: "plain"`, `reason: "unknown language"`, `lang: null`, lines equal git's lines 1:1 | a crash, or a syntax label it did not earn |
| `a fragment is not a parse error` | a `.ts` hunk that opens `function f() {` and ends before `}`: `mode: "syntax"` | a zero-length error node treated as a failure, so syntax mode never runs on a real hunk |
| `a non-empty error node is a parse error` | `parseFor` on a JSON side containing an unreadable character yields an error node with `to > from`; `syntaxDiff` returns `plain`, `parse error` | tokens trusted after the tokenizer skipped text |
| `too large is plain, at the constant` | a side of `MAX_SYNTAX_TOKENS + 1` tokens is `plain`, `too large`; one of exactly `MAX_SYNTAX_TOKENS` is `syntax` | a magic number, or an off-by-one at the bound |
| `the invariant holds` | for all eight fixtures of this file (one per grammar, from `testdata` strings in the test), the old spans' text joined with `\n` equals `before` minus its final newline, and the new likewise | a lost, added or reordered character |
| `gap splitter covers everything` (`tokens.test.ts`) | `tokenize` on a string with emoji, CJK, `\r\n`, a tab run and a lone `\` returns tokens whose texts concatenate to the input, and `\n` is always its own token | GAP_RE dropping a code point, or a newline inside a word token |
| `line numbers follow the hunk` | a hunk at `oldStart 40, newStart 42` numbers its first `same` line `40`/`42` and an `add` line `null`/`43` | numbers counted from 1, or both sides sharing one counter |
| `langFor by extension only` | `a.mts` → `typescript`, `A.TSX` → `tsx`, `Makefile` → null, `x.json5` → null | a guess from content, or case-sensitive extensions |
| `go.sum is generated and collapsed` (`categories.test.ts`) | `categorize("go.sum")` is `generated`, rule glob `go.sum`; `collapsedByDefault("generated")` | a lock file shown open among the source |
| `a migration is never hidden` | `control-plane/internal/hub/store/migrate/sql/0007_work.sql` is `migration`; `collapsedByDefault("migration")` is false | a migration hidden as generated or config |
| `first match wins` | `package-lock.json` is `generated` (not `config`); `web/x.gen.ts` is `generated` (not `source`); `ui/a.test.tsx` is `test`; `deploy/x.yaml` is `config`, rule `deploy/**` | a table re-sorted so the wide rule wins |
| `no rule is source, and says so` | `src/main.rs` → `source`, `rule: null`, `categoryWhy` is `source: no rule matched` | a default that pretends a rule matched |
| `display order is fixed` | `CATEGORY_DISPLAY_ORDER` deep-equals the six words in the scope's order | task 180's sort drifting from this task's list |
| `the gate fails on GPL` (`licences.test.ts`) | `checkLicences(fixtures, ["ok-mit", "gpl-thing"])` has one violation, `gpl-thing`, `GPL-3.0` | a gate that passes everything |
| `no licence field is a violation` | the `no-licence` fixture is a violation | missing treated as allowed |
| `an OR expression passes on one allowed side` | `or-expr` with `(MIT OR GPL-3.0)` passes; `and-expr` with `MIT AND GPL-3.0` fails | an expression parsed as a single token, or AND read as OR |
| `the real dependencies are allowed` | `checkLicences(desktopRoot, GATED_ROOTS)`: `violations` and `missing` are both `[]`, and `checked` has at least `GATED_ROOTS.length` entries | an installed dependency outside OD-11 rule 3 |
| `the gate walks transitive dependencies` | a fixture root `ok-mit` depending on `gpl-thing` yields the violation | a gate that checks roots only |
| view rows (`syntax.test.tsx`) | a rendered `HunkView` shows `grandTotal` inside `.tok-add`; a plain hunk's header reads *plain diff · unknown language*; a `PathList` row for `go.sum` has the chip *generated* whose `title` is `generated, because it matches "go.sum"`; `PathDiff` for `go.sum` shows *generated · N lines changed · show* with N the measured count; a migration path has no collapse button; each row ends `expectNoPrediction(container)` | a colour-only chip; a hunk rendered as HTML; a collapsed migration; a predicted or estimated count |

`licences.test.ts` opens with `// @vitest-environment node`, because the desktop's vitest
environment is `jsdom` (`desktop/vitest.config.ts:9`, verified) and the test reads the disk.
The four fixtures are plain `package.json` files, each `{ "name", "version": "0.0.0",
"license" }`, with `ok-mit`'s `dependencies` naming `gpl-thing` for the transitive row only in
its own fixture copy (`fixtures/licences-transitive/`), so the first row stays one violation.

#### Steps

- [ ] **Step 1: Preflight.** Rule H2 for P3-21's `HunkView.tsx`, `PathList.tsx`, `PathDiff.tsx`
  and `files.css`, and for P3-18's `desktop/src/api/convergence.ts`: each must be on the sync
  base, or this task waits. Re-read `Hunk` and `DiffLine` there; if a field name differs from
  `task-18.md:160-169`, the phase 3 name wins and this part's code follows it.
- [ ] **Step 2: The gate first, red.** Write `licences.ts`'s test rows and fixtures. Run:
  `cd desktop && npx vitest run src/test/licences.test.ts`. Expected: FAIL,
  `Failed to resolve import "../diff/licences"`.
- [ ] **Step 3: The gate, green, then the dependencies.** Implement `licences.ts`. Re-run: the
  fixture rows pass and `the real dependencies are allowed` fails with every `GATED_ROOTS` entry
  in `missing`. Now `npm install --save-exact diff @lezer/common @lezer/javascript @lezer/go
  @lezer/python @lezer/json @lezer/css @lezer/html @lezer/markdown`. Re-run. Expected: `ok`. A
  violation here **stops the task**: the offending package is removed and named in the return; no
  row is weakened to make it pass.
- [ ] **Step 4: Tokens and categories, tests first.** Write `tokens.test.ts` and
  `categories.test.ts`; run them; expected FAIL on the missing modules. Implement both. Re-run.
  Expected: `ok`.
- [ ] **Step 5: The differ, tests first.** Write `syntaxDiff.test.ts`; expected FAIL on the
  missing module. Implement steps 1-8 above. Re-run. Expected: `ok`.
- [ ] **Step 6: The view, tests first.** Write `syntax.test.tsx`; expected FAIL (no `.tok-add`
  in the rendered hunk). Modify `HunkView`, `PathList`, `PathDiff`, `files.css`. Re-run P3-21's own
  lens tests as well, unchanged: `npx vitest run src/components/lens/files`. Expected: `ok`.
- [ ] **Step 7: Measure once, claim nothing.** `bench.ts` builds one 5,000-line TypeScript side
  and a copy with 200 identifiers renamed, runs `syntaxDiff` on it ten times and prints the median
  wall time and the token counts. Run `npx tsx src/diff/bench.ts` (or the repo's TS runner if
  `tsx` is absent; which one is installed is **UNVERIFIED here**) and paste the printed line into
  the commit message body. The number is a record, not a threshold; nothing asserts it.
- [ ] **Step 8: The whole acceptance, then commit** (`git add` the new files, then
  `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
cd desktop && npx vitest run src/diff src/test/licences.test.ts src/components/lens/files
# expected: every file passes; the summary line reads "Test Files  N passed (N)" with no "failed"
npm run typecheck
# expected: exit 0, no output from tsc
node -e "const p=require('./package.json');for(const n of ['diff','@lezer/javascript','@lezer/go'])if(!/^\d/.test(p.dependencies[n]))throw n"
# expected: no output (each dependency is an exact pin, not a range)
```

#### What this task does not claim

- **Not a speed.** Step 7 records one median on one machine; it is unclaimed until measured and
  nothing depends on it. `MAX_SYNTAX_TOKENS` is a named bound, not a measured one.
- **Not a structural diff.** Tokens are compared as text in order; a moved function is still a
  deletion and an addition. difftastic-class tree matching is not built.
- **Not token accuracy inside a construct that began outside the hunk.** A hunk that starts
  inside a block comment or a template string is tokenised as code; the invariant keeps every
  character, but the grain there may be wrong.
- **Not that the licences are what the packages' authors intend.** The gate reads the installed
  `package.json` field; a wrong field is not detected.
- **Not a CI gate.** No CI tree exists here (`0901bab9`); the list and the checker are ready for
  whoever owns CI to call. Named in the return as a skeleton gap.
- **Not that P3-21's files look as cited.** Every phase 3 path and line here is from the plan at
  `94f954d`, **UNVERIFIED until merge**.
- Design: 05 §1 L (rows L-170, L-171; `05-design.md:333`); OD-11 rule 3
  (`2026-09-23-owner-decisions.md:96`). Parity: closes **L-171**.

**Interfaces:**
- Produces:
  - `diff/tokens.ts` — `Lang`, `Token`, `Tokenized`, `langFor`, `tokenize`, `GAP_RE`, `parseFor`
  - `diff/syntaxDiff.ts` — `Span`, `SpanKind`, `TokenLine`, `LineKind`, `SyntaxDiff`,
    `PlainReason`, `MAX_SYNTAX_TOKENS`, `hunkSides`, `syntaxDiff`
  - `diff/categories.ts` — `Category`, `CategoryRule`, `CATEGORY_RULES`,
    `CATEGORY_DISPLAY_ORDER`, `categorize`, `categoryWhy`, `collapsedByDefault` (task 180 consumes
    the order and `categorize`)
  - `diff/licences.ts` — `ALLOWED_LICENCES`, `GATED_ROOTS`, `checkLicences` (the next task to add
    a dependency appends it to `GATED_ROOTS`)
- Consumes:
  - task 20 — `expectNoPrediction(container)` from `desktop/src/test/expectNoPrediction`
  - P3-16/P3-18 — `Hunk`, `DiffLine`, `BackLink` from `desktop/src/api/convergence.ts`
  - P3-21 — `HunkView`, `PathList`, `PathDiff`, `files.css`; the `--diff-add`/`--diff-del` tokens
- Amends: nothing in the Hub. No wire shape changes; this task is renderer-only.
