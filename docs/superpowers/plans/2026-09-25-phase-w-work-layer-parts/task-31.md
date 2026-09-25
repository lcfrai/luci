### Task 31: `chrome.dim` and one `ViewHeader` with the route badge (02 D2 step 2)

Stream **WD1**, defines seam **SW.13**, about 3 hours (an estimate, not a measurement). Depends on
task 26 (`--t-head`, `--t-fact`, `--s-*`, `--row-h`). No Hub, no Work table, no route computation:
the header shows the route it is given and nothing else.

Two things land together because they are one idea, 02 C5: **the chrome recedes and the work is
the brightest thing on screen.** `chrome.dim` turns the rail and sub-rail down to `--ink-2` labels
and `--ink-3` icons, and marks the current item with a 2px `--accent` tick on its leading edge.
`ViewHeader` is the one header every new surface uses: title, scope chip, route badge, controls on
the right. A second header pattern anywhere is a defect (AGENTIC-UX §9 item 8, *repeated
breadcrumb chrome*), and an audit rule here makes it one.

**The route badge is the visible half of `OD-19`.** A project on the AU path reads `AU`; the
default reads `Global`. When the route is not known the badge is **hidden**, never defaulted: a
header that guesses `Global` tells a person their Protected data went to the direct API when
nobody knows where it went. `null` means unknown, and the type forces every caller to say which.

**Four corrections to the scope, verified against the tree at `d1262e2`:**

1. **There is no Develop title row to move.** `DevelopLens.tsx` renders no title (its only
   `className` hits are layout and `cols-empty`), and `topbar/TopBar.tsx:22-43` renders the layout
   toggle, two disabled buttons, *New session* and the picker, with no title. The only built lens
   title is `LensPlaceholder.tsx:15`'s `<h2>`. This task moves that one. Giving Develop a header
   row is a layout change to a built lens and goes to the owner at task 39 (see *does not claim*).
2. **The rail's current item is `aria-selected="true"`, the sub-rail's is `aria-current="page"`.**
   `BranchRow.tsx:41`, `ProjectRow.tsx:34` and `SessionRow.tsx:34` set `aria-selected`;
   `SubRail.tsx:45` sets `aria-current`. The tick keys on each file's own attribute. Both rules
   today paint `--accent-tint` plus an inset `--accent-ring` (`rail.css:33-37`,
   `subrail.css:54-58`); the new look drops both, because a tinted, ringed row is the chrome
   shouting, and keeps the tick alone.
3. **`.rail-add` is accent on a resting row.** `rail.css:138` colours the hover-revealed `+` in
   `--accent-hi`. That is accent outside the current item, which the audit below forbids. It
   moves to `--ink-2`, `--ink` on hover and focus.
4. **`app.css` gains a heading reset, not a colour.** `:where(h1, h2, h3) { margin: 0; font:
   inherit; }` so no heading carries the browser's default size and `.vh-title` is the one place
   header type is set. It declares no colour, so `tokens.test.ts:85-90` (token import first, no
   colour of its own) still holds. The heading's weight and size come only from `--t-head`.

**Why a pseudo-element tick, not an inset shadow.** `box-shadow: inset 2px 0 0` paints the left
edge, which is the *trailing* edge in a right-to-left layout. A `::before` placed with
`inset-inline-start: 0` is the leading edge in both. It also gives task 34 one element to repaint
under `forced-colors`, where Chromium drops every `box-shadow`.

**Files:**
- `desktop/src/components/common/ViewHeader.tsx` — the component, `Route` (new)
- `desktop/src/components/common/viewheader.css` — owns `.vh-*` (new; the contract §5.5 rule: one
  component, one prefixed stylesheet, `@import "../../styles/tokens.css";` first)
- `desktop/src/components/common/ViewHeader.test.tsx` (new)
- `desktop/src/components/lens/LensPlaceholder.tsx` ⚑ (20 lines; the `<h2>` at `:15`) — the title
  moves onto `ViewHeader` (modified)
- `desktop/src/components/shell.css` ⚑ (`.lens-placeholder h2` at `:44-47`) — that rule is deleted
  (modified)
- `desktop/src/styles/app.css` ⚑ (82 lines; `code, kbd, pre, samp` at `:39-44`) — the heading reset
  after `:44` (modified)
- `desktop/src/components/rail/rail.css` ⚑ (147 lines; `.rail-row` `:17-29`, current `:33-37`,
  `.rail-chev` `:42-46`, `.rail-add` `:132-147`) — `chrome.dim`, the tick, the old look (modified)
- `desktop/src/components/subrail/subrail.css` ⚑ (62 lines; `.sub-back` `:18-26`, `.sub-row`
  `:36-50`, current `:54-58`, `.sub-glyph` `:59-62`) — the same (modified; not in the skeleton's
  file map, which 02 C5 needs)
- `desktop/src/styles/designsystem.test.ts` ⚑ (886 lines; `REAL: Tree` at `:73`) — four rules
  (modified)
- `desktop/src/components/rail/Rail.test.tsx` ⚑ (181 lines) — one render test (modified)

`AppShell.tsx` is untouched: it passes `lens`, `phase` and `what` to `LensPlaceholder` today
(`:30-62`) and still does; the placeholder passes `route={null}`, because no lens knows its project's
route before task 24.
