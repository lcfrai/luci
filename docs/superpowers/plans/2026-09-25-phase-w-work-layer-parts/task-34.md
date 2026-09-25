### Task 34: the `forced-colors` layer; the reduced-motion running shape and static tick

Stream **WD1**, about 2 hours (an estimate, not a measurement). Depends on task 29 (the running
shape, `.gl-agent-live`, `.dot-run`) and task 33 (the `data-motion="reduce"` hook and the
forced-colours screen this layer is looked at through). No Hub, no wire shape.

Two floors land together because both are about a signal surviving when the usual carrier is
gone (05 §4.4 work rule 4 and C7, revision 2; 07 D C4, D I8):

- **Windows contrast themes.** Under `forced-colors: active` Chromium forces every `box-shadow`
  to `none` and repaints backgrounds in `Canvas`. This design system draws elevation, the focused
  pane, the selected palette row and every panel edge as a `box-shadow`, and the rail's status
  dots as a background. In a contrast theme today they all vanish. A `@media (forced-colors:
  active)` layer redraws each as a `border` or `outline` in system colours.
- **Reduced motion.** With the breath stopped, running keeps task 29's own shape (a disc in an
  open ring), so shape alone separates running, idle, exited and failed. The state flash becomes
  a **static 2px leading tick in `--ink`** that stays while the row is unseen. And every
  reduced-motion rule is written twice, as the media query and as the `:root[data-motion="reduce"]`
  twin that task 33's set writes, so a reviewer can see the reduced look without changing Windows.

**Five corrections to the scope, verified against the tree at `b8bd64e` and the written parts:**

1. **The unseen attribute is `data-changed="unseen"`,** not the scope's `data-unseen="true"`.
   Task 41's part is written and sets `data-changed="unseen"` on every held row, saying task 34
   draws it (`task-41.md:53-56`). One attribute, the one already in a plan: this task keys on it.
2. **The tick is in `tokens.css`, not `glyphs.css`.** It marks a row, not a glyph, and
   `glyphs.css` owns `.gl-*` and no other prefix (task 28's file map). The only global
   reduced-motion rules in the tree are the floor at `tokens.css:173-183`; the tick is the floor's
   second rule and sits beside it. `glyphs.css` gains only the `.gl-agent-live` twin.
3. **The ring rule covers every `box-shadow`, not only `--e-*`, `--thread` and `--accent-ring`.**
   Every one of the tree's 36 non-token `box-shadow` declarations is a ring, an elevation or a
   one-pixel edge (`rail.css:10`, `subrail.css:9`, `topbar.css:10` and nine more are `--line`
   edges; `common.css:56`, `:101` and `:121` are the dots' `currentColor` rings). Chromium drops all
   of them. A rule that checked three token families would pass a contrast theme with no panel
   edges and no hollow *starting* dot. Two exclusions: a value of `none`, and a selector under
   `:root[data-look="old"]` (task 30's convention; task 39 deletes the old look).
4. **The dots need an opt-out, not an outline.** `.dot`'s fill is `background: currentColor`
   (`common.css:37-47`), which forced colours repaint as `Canvas`: every rail dot would disappear,
   running or not. `.dot` takes `forced-color-adjust: none` and `color: CanvasText`, so the fill,
   the ring and task 29's `.dot-run` outline all draw in the one system ink.
5. **`lease.ring` does not exist yet.** 02 C9 puts it on the team strip's chip, which no Phase W
   task before this one builds. The ring audit fails it the day its stylesheet lands without a
   forced-colours line, which is the point of an audit over the tree rather than a list.

**What stays as it is.** The focus ring is already an `outline` (`tokens.css:187-191`) and forced
colours keep outlines, so it is not touched. Glyphs are SVG in `currentColor` (tasks 28, 29), which
forced colours map to `CanvasText`, so their shapes carry the meaning on their own.
`prototype/work-layer.html:379-386` sketches forced-colours rules; they are rewritten here against
the real class names, not copied.

**Files:**
- `desktop/src/styles/forced-colors.css` — the layer (new)
- `desktop/src/styles/app.css` ⚑ (82 lines; the imports at `:10-11` after task 27) — imports the
  layer after `fonts.css` (modified)
- `desktop/src/styles/tokens.css` ⚑ (191 lines; the floor at `:173-183`) — the floor's twin, the
  tick and its twin, after `:183` (modified)
- `desktop/src/components/glyphs/glyphs.css` ⚑ (task 29) — the `.gl-agent-live` twin (modified)
- `desktop/src/components/common/common.css` ⚑ (179 lines; the reduced block at `:119-123`) —
  its twin (modified)
- `desktop/src/components/palette/palette.css` ⚑ (the reduced block at `:131-135`) — its twin
  (modified)
- `desktop/src/components/devtools/devtools.css`, `stress.css` ⚑ (tasks 32, 33) — each gains its
  own forced-colours lines, so no production file names a dev class (modified)
- `desktop/src/styles/designsystem.test.ts` ⚑ — four rules and their canaries (modified)
- `desktop/src/styles/tokens.test.ts` ⚑ (91 lines) — the tick (modified)
- `desktop/src/components/glyphs/AgentGlyph.test.tsx` ⚑ (task 29) — one test (modified)
