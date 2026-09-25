### Task 30: `voice.split` across Develop and the session pane (02 D2 step 3)

Stream **WD1**, no seam of its own (it applies **SW.11**), about 2.5 hours (an estimate, not a
measurement). Depends on task 27 (`--mono` with Commit Mono, `--sans-small`) and, through it, task
26 (`--t-fact`, `--t-meta`, `--s-*`).

**The rule** (05 §4.4 C3; 02 C3; 07 D I7). Human language is sans. Every machine fact (an id, a
key, a path, a time, a duration, an exit code, a cost, a count of tokens, a coordinate) is mono at
`--t-fact`, 12px, in `--ink-2` or stronger: never 11px, never `--ink-3`. Evidence and claims look
different on sight, which is AGENTIC-UX §2 point 7 made visual, and a fact a person reads to
decide is set large and dark enough to read without leaning in.

**The built tree breaks it in three places**, all in `develop.css`, read at `f7ea2a2`:

| Rule | Line | Today | Carries |
|---|---|---|---|
| `.pane-attach` | `:166-173` | `400 11px/1.5 var(--mono)`, `--ink-2` | the attach word, the retry seconds, and on failure `attach.reason`, **a sentence**, set in mono |
| `.pane-foot` | `:244-255` | `400 11px/1.4 var(--mono)`, `--ink-3` | command, model, elapsed, attach word |
| `.cv-coords` | `:386-395` | `500 10px/1 var(--mono)`, `--ink-3` | a tile's live coordinates |

and `.pane-sub` (`:120-124`, `400 11px/1.4 var(--sans)`, `--ink-3`), the harness's product name,
is human language below the type scale's smallest sans step. One more mono rule outside Develop
breaks the size rule: `palette.css:115-123`, `.pal-hint`, `11px` mono. It is a kbd hint (it
styles `<kbd className="pal-hint">`, `CommandPalette.tsx:135`), so it is allowed `--t-micro`, but
its selector does not say so.

**Each field, classified** (the table the tests hold):

| Where | Field | Source | Voice |
|---|---|---|---|
| header `.pane-title` | worktree name, or session title | `SessionPaneHeader.tsx:36` | sans: a name a person chose |
| header `.pane-sub` | harness product name ("Claude Code") | `:37` | sans |
| footer, fact 1 | `harness.command` ("claude"), or the harness id | `SessionPaneFooter.tsx:57` | mono: a command |
| footer, fact 2 | `session.model` | `:58` | mono: an id |
| footer, fact 3 | session elapsed ("41m") | `:59` | mono: a duration |
| footer, fact 4 | attach word ("attached", "not attached") | `:60` | **sans**: a state in words |
| footer and overlay | retry seconds ("12s") | `:17-20` | mono: a duration |
| overlay, on failure | `attach.reason` | `SessionPane.tsx:101` | **sans**: a sentence written for a person |
| canvas | tile coordinates | `CanvasLayout.tsx:376` | mono |

**Three audit rules**, added to `designsystem.test.ts` beside the built ones, each with a planted
violation so the rule is shown to fire:

- **(A) size.** A rule that sets a mono face (`font-family` or `font` naming `var(--mono)`) *and*
  a size sets that size as `var(--t-fact)`. Only a **kbd hint** selector (the `kbd` element, a
  `.kbd` class, a class ending `-kbd`) may use `var(--t-micro)` instead. A mono rule that sets no
  size inherits one and is not an offender (`app.css:39-44`'s `code, kbd, pre, samp`, and
  `rail.css:120-126`'s `.rail-kbd`).
- **(B) ink.** No rule that sets a mono face sets `color: var(--ink-3)`, a kbd hint excepted: a
  hint is chrome, not a fact (02 C3 gives it its own step). `.rail-kbd` is that exception.
- **(C) spacing.** CSS under the Phase W directories (`src/components/work`, `issue`, `desk`,
  `glyphs`, `thread`, `devtools`) uses only `var(--s-1)` … `var(--s-6)`, `0` or `auto` for
  `padding`, `margin` and `gap` and their longhands. This is the rule task 26 deferred; the Phase 2
  stylesheets keep their literal spacing.

**The old look stays reachable.** Each replaced declaration is kept, in the same file, under a
`:root[data-look="old"]` selector, so task 32's toggle flips old and new on one screen and nothing
is lost until the owner accepts the new look (task 39 records that, and deletes the old rules).
Rules A and B skip any selector whose every part starts `:root[data-look="old"]`: the old look is
the thing they are replacing, kept on purpose, and bounded by task 39.
