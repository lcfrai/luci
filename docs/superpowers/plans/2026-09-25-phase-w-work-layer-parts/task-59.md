### Task 59: the palette: every work verb with its chord, the four prefixes, opens beside the row

Stream **WK**, about 3 hours (an estimate, not a measurement). Depends on task 54 (`chordFor`,
`printKey`, `SEQUENCES`, `__setKeymapForTest`, `task-54.md:200-215`) and task 58 (`search`), and
through them on task 53 (`KEYMAP`, `KeyBinding`, `RowContext`, `ACTIVE_SLICES`,
`task-53.md:106-171`). Extends the built palette: `buildCommands`
(`desktop/src/components/palette/commands.ts:72`), `filterCommands` (`:219`), `Command.hint`
(`:17`) and `CommandPalette.tsx`'s by-id selection (`:4`) — all four verified on the tree
2026-09-27.

After this task the palette is the second way to reach every W1 verb, and it cannot drift from
the first: every work row in it is generated from `KEYMAP`, and its hint is `chordFor`'s.

**What the reader sees** (05 §4.3 *The palette*, §4.1's Palette row, §4.6;
`prototype/work-layer.html`):

- `Ctrl+K` with no row focused opens the palette as it opens today, centred, with the work
  verbs now in it: *Change state*, *Assign*, *Set priority*, *Move to project*, *Add a label*,
  *Add a relation*, *Comment*, *Copy link*, *Go to Work*, *Go to Plan*, *Go to the Desk*,
  *Apply held changes*, *Subscribe*, *Unsubscribe*, *Start* — each with the key it also has.
- `Ctrl+K` **with a row focused**, or the row's menu, opens it **beside that row**, and that
  row's verbs are first. The row stays visible and the list does not scroll.
- Four prefixes: `#` an issue by alias or title (task 58's `search`), `@` a person of my teams,
  `>` commands only, `~` a session. The prefix is stripped from the needle; the scope shows as
  a chip in the input.
- *Subscribe* and *Unsubscribe* carry **no hint**, because they have no key.
- *Delegate* and *Ask* are not there in W1.

**Decisions made here, so nobody re-derives them:**

1. **The work rows are generated, never written.** `workCommands(state, row)` walks
   `activeKeymap()` and `SEQUENCES`, keeps the rows whose `slice` is in `ACTIVE_SLICES`, and
   makes one `Command` per verb with `label` from the binding's `label` and `hint` from
   `chordFor(verb)`. A verb with no key still gets a row, with no hint: `chordFor` returns
   `null` and `hint` is left undefined, which is how *Subscribe* reads. Nothing in this file
   spells a key, so a re-bound key moves the hint with it and test 1 proves it.
2. **`buildCommands` gains one call, and the block order is unchanged.** The work block is
   pushed first, before the built verbs, and only when `state.lens` is a work lens
   (`"work"`, `"plan"` or `"desk"`), so the Develop palette is exactly what it is today. §6.7's
   rule that ranking is fixed here and filtering only removes rows is untouched.
3. **A verb needs a row, so a row is the argument.** `Command.run` takes no argument
   (`commands.ts:23`), so the work rows close over the `RowContext` the palette was opened
   with — task 53's own shape (`task-53.md:139-147`), the same one the key path passes. From an
   anchored open it is the anchored row, **not** whatever the list cursor moved to afterwards:
   the context is captured at open. Test 7 is that sentence.
4. **The prefix is parsed, never guessed.** `parsePrefix(query)` reads one leading character and
   returns `{scope, needle}`; anything else is `{scope: "all", needle: query}`. The four
   characters are `#`, `@`, `>` and `~`, and a query of exactly one of them is a scope with an
   empty needle, which lists that scope whole. The prefix never re-ranks: within a scope the
   order is the order `buildCommands` and `search` gave.
5. **`filterCommands` keeps its label-only rule, and its comment keeps saying why.** The scopes
   do not widen matching: `>` narrows the set and then `filterCommands` runs on it, unchanged.
   `#`, `@` and `~` do not go through `filterCommands` at all — they are not commands — so the
   built palette's tests stay green with no edit (`:219-222`).
6. **Beside the row is a position, not a different component.** `openPalette({anchor, row})`
   writes both into the store beside `paletteOpen`; `PaletteBody` reads the rect and sets
   `--pl-x` and `--pl-y` on its own element, clamped to the viewport. There is one palette, one
   focus owner and one `useDismissable` (`CommandPalette.tsx:34`); §6.8's rule that the palette
   owns DOM focus while open and returns it on close is untouched.
7. **Selection stays by id.** Every position that enters `CommandPalette.tsx` is converted back
   through `idAtVisiblePosition` (`:12`, known defect 10). The scoped lists are lists of rows
   with ids — a command id, an `is-` id, a `u-` id, a session id — and the highlighted row is
   remembered by that id across a keystroke that removes rows above it.
8. **`budget.palette ≤ 50 ms` is recorded, not asserted.** The open-to-first-paint time is
   written to the budget log task 44 added; no test fails on it (OD-27: no predictions, and a
   timing assert on a loaded machine is a coin toss). Task 70 reads the log.

**Files:**
- `desktop/src/components/palette/commands.ts` ⚑ — `workCommands`, `parsePrefix`, the call in
  `buildCommands` (modified)
- `desktop/src/components/palette/commands.test.ts` ⚑ — the generated rows and the prefixes
- `desktop/src/components/palette/CommandPalette.tsx` ⚑ — the anchor, the scope chip, the four
  scoped lists (modified)
- `desktop/src/components/palette/CommandPalette.test.tsx` ⚑ — the anchored open and the scopes
- `desktop/src/components/palette/palette.css` ⚑ — `.pl-anchored`, `.pl-scope` (modified)
- `desktop/src/store/store.ts` ⚑ — `paletteAnchor` and `paletteRow` beside `paletteOpen`
- `desktop/src/work/keys/keymap.ts` ⚑ (task 53) — nothing added; read only

#### Interfaces — complete: `commands.ts` ⚑, part 1: `parsePrefix` and the scope

```ts
// desktop/src/components/palette/commands.ts ⚑ — appended; the file's existing exports stand.
import { ACTIVE_SLICES, KEYMAP, type RowContext, type VerbId } from "../../work/keys/keymap";
import { SEQUENCES, chordFor } from "../../work/keys/sequences";
import { runVerb } from "../../work/keys/verbs";          // task 53's runner registry

export type PaletteScope = "all" | "issues" | "people" | "commands" | "sessions";

const PREFIX: Readonly<Record<string, PaletteScope>> = {
  "#": "issues", "@": "people", ">": "commands", "~": "sessions",
};

/** Decision 4. One leading character, stripped; anything else is the whole query, scope all. */
export function parsePrefix(query: string): { scope: PaletteScope; needle: string } {
  const head = query.slice(0, 1);
  const scope = PREFIX[head];
  return scope ? { scope, needle: query.slice(1).trimStart() } : { scope: "all", needle: query };
}
```

A query of `"#"` alone is `{scope: "issues", needle: ""}`, which lists the reader's issues
whole; that is the cheapest way to see what `#` means, and it is why the needle is trimmed on
the left but not tested for emptiness here. A `#` in the middle of a query is not a prefix: the
head is the only place it is read, so *fix #4 in the parser* searches for that text.

#### Interfaces — complete: `commands.ts` ⚑, part 2: `workCommands`, generated from `KEYMAP`

```ts
/** Decision 1. One row per active verb, in KEYMAP's own order, then SEQUENCES'. The hint is
 *  chordFor's, so a re-bound key moves it; nothing here spells a key. */
export function workCommands(state: CockpitState, row: RowContext | null): Command[] {
  const seen = new Set<VerbId>();
  const out: Command[] = [];
  const push = (verb: VerbId, label: string) => {
    if (seen.has(verb)) return;
    seen.add(verb);
    const hint = chordFor(verb) ?? undefined;
    out.push({
      id: `work:${verb}`, label, glyph: GLYPH.verb,
      ...(hint ? { hint } : {}),
      run: () => runVerb(verb, row ?? emptyRow(state)),   // decision 3: the captured row
    });
  };
  for (const b of KEYMAP) if (ACTIVE_SLICES.has(b.slice) && !NAV_ONLY.has(b.verb)) push(b.verb, b.label);
  for (const s of SEQUENCES) if (ACTIVE_SLICES.has(s.slice)) push(s.verb, s.label);
  return out;
}
```

```ts
/** Movement and typing are not palette rows: nobody reaches "Next row" through ⌘K. */
const NAV_ONLY: ReadonlySet<VerbId> = new Set<VerbId>([
  "focus.next", "focus.prev", "select.toggle", "select.extendDown", "select.extendUp",
  "list.search", "keys.sheet", "peek.toggle", "open.issue",
]);

/** The three verbs with no key, so no hint (05 §4.3's table has no Subscribe chord). */
const KEYLESS: readonly { verb: VerbId; label: string }[] = [
  { verb: "issue.subscribe", label: "Subscribe" },
  { verb: "issue.unsubscribe", label: "Unsubscribe" },
  { verb: "changes.apply", label: "Apply held changes" },
];

const emptyRow = (state: CockpitState): RowContext =>
  ({ issueId: null, projectId: state.selection.projectId ?? null, scopes: ["list"], element: null });
```

`KEYLESS` is pushed through the same `push`, after the two loops, so a verb that later gains a
key drops out of the constant and keeps its row: `push` skips what it has seen, and `chordFor`
starts answering. *Apply held changes* is in it only until task 57 binds it a key; if it has one
by then, `chordFor` prints it and the constant's row is a no-op. `VerbId`s here are task 53's;
any name this part guessed is caught by `tsc`, because `VerbId` is a union, not `string`.

```ts
// inside buildCommands(state), decision 2 — the first block, before the built verbs:
  const WORK_LENSES = new Set(["work", "plan", "desk"]);
  if (WORK_LENSES.has(state.lens)) {
    commands.push(...workCommands(state, state.paletteRow));
  }
```

`state.paletteRow` is `null` for a centred open, so the same call serves both and there is no
second code path for the anchored palette. The rows that move to the front when a row is
anchored are moved by `orderForRow` below, not by a second build.

#### Interfaces — complete: `commands.ts` ⚑, part 3: the row's verbs first

```ts
/** Decision 3's second half: from an anchored open, that row's verbs lead. Stable within each
 *  group, so §6.7's "rows never reflow while filtering" still holds: this runs once, at build. */
export function orderForRow(commands: readonly Command[], row: RowContext | null): Command[] {
  if (!row) return [...commands];
  const scopes = new Set(row.scopes);
  const rank = (c: Command): 0 | 1 => {
    if (!c.id.startsWith("work:")) return 1;
    const verb = c.id.slice("work:".length) as VerbId;
    const b = KEYMAP.find((k) => k.verb === verb) ?? SEQUENCES.find((k) => k.verb === verb);
    return b && (scopes.has(b.scope) || b.scope === "everyRow") ? 0 : 1;
  };
  return [...commands].sort((a, b) => rank(a) - rank(b));   // Array.sort is stable (ES2019)
};
```

The sort is by one bit and nothing else, so two rows of the same rank keep the order
`buildCommands` gave them: the palette's block order survives, and only the anchored row's
verbs rise above it. `orderForRow` is called by `PaletteBody` once, in the same `useMemo` as
`buildCommands`, which is why it takes and returns an array rather than reading the store.

#### Interfaces — complete: `CommandPalette.tsx` ⚑, the anchor and the four scopes

```tsx
// desktop/src/components/palette/CommandPalette.tsx ⚑ — inside PaletteBody.
const anchor = useCockpit((s) => s.paletteAnchor);          // DOMRect | null
const row = useCockpit((s) => s.paletteRow);                // RowContext | null
const all = useMemo(() => orderForRow(buildCommands(useCockpit.getState()), row), []);
const { scope, needle } = parsePrefix(query);

const rows = useMemo(() => {
  switch (scope) {
    case "commands": return filterCommands(all.filter((c) => c.id.startsWith("work:") || c.id.startsWith("verb:")), needle);
    case "issues":   return issueRows(search(needle, { projectId: row?.projectId ?? null }));
    case "people":   return peopleRows(needle);
    case "sessions": return filterCommands(all.filter((c) => c.id.startsWith("session:")), needle);
    case "all":      return filterCommands(all, needle);
  }
}, [scope, needle, all]);

// and on the shell element:
<div ref={shellRef}
     className={anchor ? "pl pl-anchored" : "pl"}
     style={anchor ? ({ "--pl-x": `${clampX(anchor)}px`, "--pl-y": `${anchor.bottom + 4}px` } as CSSProperties) : undefined}>
  {scope !== "all" && <span className="pl-scope">{SCOPE_WORD[scope]}</span>}
```

`issueRows` and `peopleRows` turn task 58's results and the replica's people into the same
`Command` shape the palette already renders — id, label, meta, glyph, `run` — so the list, the
by-id selection (`CommandPalette.tsx:4`) and the keyboard are one code path for all four
scopes. An issue row's `run` opens the issue; a person's `run` filters the current list to
their work, which is a W1 read and not an assign.

```tsx
/** The store, beside paletteOpen. openPalette carries both or neither. */
openPalette(arg?: { anchor: DOMRect; row: RowContext }): void {
  set({ paletteOpen: true, paletteAnchor: arg?.anchor ?? null, paletteRow: arg?.row ?? null });
},
closePalette(): void { set({ paletteOpen: false, paletteAnchor: null, paletteRow: null }); },
```

`closePalette` clears both, so the next `Ctrl+K` from anywhere else is centred with no row: a
stale anchor is the one way this feature could put the palette over a row that has scrolled
away. The key path calls `openPalette({anchor: el.getBoundingClientRect(), row})` from task 53's
list dispatcher when a row has focus, and the row menu (task 54) calls the same function.

```css
/* desktop/src/components/palette/palette.css ⚑ — appended */
.pl-anchored { position: fixed; left: var(--pl-x); top: var(--pl-y); transform: none; }
.pl-scope { font-family: var(--font-mono); padding: 0 var(--sp-1); border-radius: var(--r-1); }
.pl-anchored { animation: pl-in var(--dur-pop) var(--ease-out); }
@keyframes pl-in { from { opacity: 0; transform: translateY(-4px); } }
```

`transform: none` overrides the centred palette's centring transform; the entrance animates
`opacity` and `transform` only (DESIGN-SYSTEM.md). `clampX` keeps the left edge inside the
viewport minus the palette's own width, so a row at the right edge opens leftward.

#### Tests to write first, part 1 — `commands.test.ts` ⚑ (pure, no rendering)

```ts
// 1. THE DRIFT TEST. For every active KEYMAP and SEQUENCES row that is not NAV_ONLY, the
//    generated command's hint equals chordFor(verb). Then __setKeymapForTest rebinds one verb
//    from "S" to "Shift+S" and the same assertion runs again.
//    Mutant it kills: a hand-written hint, which passes once and then drifts the day a key
//    moves — the defect this whole task exists to make impossible.
// 2. Subscribe and Unsubscribe have no `hint` member at all (not "", not undefined-but-present
//    in a snapshot). Mutant: a "⇧S" hint invented for symmetry, teaching a key that does not
//    exist.
// 3. Delegate and Ask produce no row: their KEYMAP rows are slice W3, and ACTIVE_SLICES is
//    {W1}. Mutant: the slice filter dropped, so the palette offers two verbs that do nothing.
// 4. Every W1 verb of 05 §4.3's table has a row, by count: the generated set's verbs equal
//    KEYMAP's active non-NAV_ONLY verbs plus SEQUENCES' plus KEYLESS', with no extra and none
//    missing. Mutant: a verb quietly left out of the palette but reachable by key.
// 5. parsePrefix: "#resume" → {issues, "resume"}; "@al" → {people, "al"}; ">" → {commands, ""};
//    "~s-1" → {sessions, "s-1"}; "fix #4" → {all, "fix #4"}; "" → {all, ""}.
//    Mutant: indexOf("#") anywhere in the query, so a literal # is unsearchable.
// 6. filterCommands is unchanged: the built tests pass untouched, and a needle matching only a
//    `meta` value still returns nothing. Mutant: matching widened to meta (the comment at
//    :219-222 says why not).
// 7. orderForRow with a row whose scopes are ["issueRow","everyRow"] puts the issue verbs
//    first and keeps the relative order inside each group; with row null it changes nothing
//    (a deep-equal against buildCommands' own output). Mutant: a sort that re-ranks the whole
//    list, which §6.7 forbids.
// 8. buildCommands on a Develop snapshot (lens "develop") has no work: rows at all — the
//    existing commands.test.ts snapshot is unchanged. Mutant: work verbs in the Develop
//    palette, where no row can answer them.
```

#### Tests to write first, part 2 — `CommandPalette.test.tsx` ⚑ (the surface)

```tsx
// 9.  `#resume` lists EP-142 and no command rows: every row's id starts "is-". Mutant: the
//     prefix ignored and the needle matched against labels, so "resume" lists Restart.
// 10. `>` lists only commands, `@al` lists Alice and nobody else, `~` lists the built session
//     rows. Each asserts the id prefix of every row.
//     Mutant: a prefix mapped to the wrong scope, e.g. @ to issues, which is a wrong answer
//     that still looks like a list.
// 11. Opened from a focused row: the palette's element has class pl-anchored, its --pl-y is
//     the row's bottom + 4, and the first three rows are that row's verbs (state, assign,
//     priority). Mutant: centred, in global order, so the row is covered and its verbs are
//     buried.
// 12. THE ANCHORED-ROW TEST. Open from EP-142's row, then move the list cursor to EP-150 with
//     J (the list still has keys), then run *Change state*: runVerb was called with
//     issueId "is-142". Mutant: the runner reads the live cursor, which changes the wrong
//     issue and is invisible until someone notices the wrong row moved.
// 13. Escape closes and the next centred open has no anchor and no row (the store is clear).
//     Mutant: a stale anchor, so a later palette opens over empty space.
// 14. Selection survives filtering: highlight the third row, type a character that removes the
//     first two, and the same command is still highlighted (by id, not by index).
//     Mutant: the index kept, which is known defect 10 reintroduced.
// 15. A11y: the scope chip is in the input's accessible description, the list keeps its
//     role=listbox and the anchored palette still returns focus to the row on close (§6.8).
//     Mutant: focus dropped to the body, which strands a keyboard reader.
// 16. expectNoPrediction(palette): no row's label, meta or hint holds "~", "%" or "will".
```

Tests 9-11 and 14 use the built harness in `CommandPalette.test.tsx`; the new fixtures are one
work lens snapshot, three heads and two people. `search` is stubbed with task 58's own fake so
this file tests the palette and not the index.

#### Steps

- [ ] Write test 5, then `parsePrefix`. Mutant: `indexOf("#")`.
- [ ] Write tests 1-4, then `workCommands`, `NAV_ONLY` and `KEYLESS`, then the call in
      `buildCommands`. Mutants, one at a time, then revert: a literal `"⇧S"` hint (1); a hint on
      Subscribe (2); drop the `ACTIVE_SLICES` test (3); remove one verb from the loop (4).
- [ ] Write tests 7 and 8, then `orderForRow` and the `WORK_LENSES` test. Mutants: sort by
      label (7); drop the lens test (8).
- [ ] Add `paletteAnchor` and `paletteRow` to the store, and `openPalette`'s argument. The
      built palette tests pass with no edit (the argument is optional).
- [ ] Write tests 9-13, then the scope switch, the chip and the anchored class and CSS.
      Mutants: ignore the scope (9); swap two entries in `PREFIX` (10); drop the anchor class
      (11); read the live cursor in `run` (12); leave the anchor on close (13).
- [ ] Write tests 14-16. Mutant: index selection (14); no focus return (15).
- [ ] Record `budget.palette` around the first paint (decision 8). Do not assert it.
- [ ] Tasks 53, 54 and 58's suites and the built palette suite all pass. Commit each file as it
      goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/palette'
# expected: 2 files, 16 new tests plus the built palette tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 53, 54 and 58 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0 (a VerbId this part guessed wrong fails here)
~/.claude/tools/claudezsh/claudezsh "grep -nE '\"⇧|Shift\\+|Ctrl\\+' desktop/src/components/palette/commands.ts"
# expected: no output (decision 1: every chord comes from chordFor)
~/.claude/tools/claudezsh/claudezsh 'grep -n "command.meta" desktop/src/components/palette/commands.ts'
# expected: only inside metaText-style rendering, never inside filterCommands (decision 5)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "transition: all|position: absolute" desktop/src/components/palette/palette.css'
# expected: no output (the anchored palette is fixed, and motion is opacity/transform only)
```

#### What this task does NOT claim

- **An assistant in the palette** (L-155): asking the palette a question in words is team
  mode's, and W1 has no gateway.
- **W2's *Save as view*** (L-95): the palette lists views once task 95 builds them.
- **Create from the palette.** The *New issue* row is task 68's; this part's generated set is
  the verbs `KEYMAP` holds, and `C` is task 68's binding.
- **Fuzzy matching.** `filterCommands` is a case-insensitive substring over `label`, unchanged;
  a fuzzy rank would re-rank, which §6.7 forbids.
- **A measured open time.** `budget.palette` is recorded, never asserted (decision 8).
- **That 50 ms is met.** The number is a target in 05; task 70 reads the log and says what it
  was on the owner's machine.
- **Search quality.** `#` shows exactly what task 58's `search` returns, in its order.
- **Visual acceptance:** the owner's eyes (CLAUDE.md, working agreements).
- **Parity:** closes L-221's W1 part and L-222, and L-226's palette half (task 54 did the
  menus); serves L-29 (`C` from the palette, whose create is task 68).

#### Gaps found while writing this part

1. **`runVerb` and `desktop/src/work/keys/verbs.ts` are this part's names** for task 53's runner
   registry: `task-53.md` prints `VerbRunner` (`:148`) and the dispatcher but no module that
   holds the registered runners. Task 53 should name the module and the call, and this part and
   task 54's menus both follow it. **UNVERIFIED** until then.
2. **`state.lens` has no work value on the built store.** `CockpitState.lens` is Develop's today
   (`store/store.ts`); `"work"`, `"plan"` and `"desk"` are added by tasks 40, 41 and 62, and
   decision 2's `WORK_LENSES` test depends on all three existing. The skeleton should say which
   task widens the union.
3. **`CockpitState.selection.projectId`** is read by `emptyRow`; the built selection carries
   `branchId` (`commands.ts:82`) and a project id is reached through the branch. If there is no
   `projectId` member, `emptyRow` resolves it the way `projectName` does (`:79`) and the line
   changes; either way no verb runs on a guess.
4. **`GLYPH.verb` is the built kind marker** (`commands.ts:30`); a work verb reuses it rather
   than adding a kind, so the palette's glyph column keeps meaning *kind*, not *status*. If
   work rows should read differently, that is a design question for 05 §4.3, not a code one.
5. **No part owns the people list.** `peopleRows` needs *the people of the reader's teams* from
   the replica; task 57's visibility rules cover what is readable, but no task names the
   selector. Task 58 is the nearest owner.
6. **`Command.run` takes no argument** (`commands.ts:23`), which is why decision 3 closes over
   the row. If a later task needs the live row instead, the field gains a parameter and every
   built row ignores it; that is a one-line change here and a signature change there.
7. The skeleton's file map has no `work/keys/verbs.ts`; see gap 1.
