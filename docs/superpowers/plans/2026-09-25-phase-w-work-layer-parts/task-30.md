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

**Four corrections to the scope, made here so nobody re-derives them:**

1. **The header shows no branch.** `SessionPaneHeader.tsx:36-37` renders the worktree name (or
   session title) and the harness's product name; both are human language. The scope's test
   "keeps the worktree name sans and the branch mono" becomes "keeps the worktree name and the
   harness name sans". Adding a branch to the header is a design change for the owner, not this
   task. `SessionPaneHeader.tsx` is therefore **not** modified; only its CSS is.
2. **The attach reason is rendered by `SessionPane.tsx`, not the footer.** The footer carries it
   only as a `title` (`SessionPaneFooter.tsx:63`); the `AttachOverlay` renders it (`SessionPane.tsx:101`)
   in the mono `.pane-attach`. So `SessionPane.tsx` is a file this task touches, and the footer's
   test is "renders facts mono and the attach word sans".
3. **`palette.css` is a file this task touches.** Rule A over the real tree fails on `.pal-hint`
   (mono, `11px`). The fix is its selector, `kbd.pal-hint`, which names the element it already
   styles, and `var(--t-micro)` for its size. No markup changes.
4. **The kept old look is written in longhands where it can be,** so it never re-sets the face and
   never resets `font-variant-numeric` (the shorthand does; `designsystem.test.ts:674-691` pins
   the restatement on the three live-number carriers, `LIVE_NUMBER_CARRIERS` at `:224-231`).
   `.pane-attach`'s old rule does re-set the mono face, which is why rules A and B skip the
   old-look selectors.

**Files:**
- `desktop/src/components/develop/develop.css` ⚑ (474 lines) — four rules changed, `.pane-voice`
  and `.pane-fact` added, the old-look block appended (modified)
- `desktop/src/components/palette/palette.css` ⚑ (`.pal-hint` at `:115-123`) — the selector and
  size; its old-look rule (modified)
- `desktop/src/components/develop/SessionPaneFooter.tsx` ⚑ (72 lines) — `attachParts`, the split
  render; `attachLabel` kept as their join (modified)
- `desktop/src/components/develop/SessionPane.tsx` ⚑ (`AttachOverlay` at `:74-103`) — the two
  voices in the overlay (modified)
- `desktop/src/components/develop/SessionPane.test.tsx` ⚑ — four tests (modified)
- `desktop/src/styles/designsystem.test.ts` ⚑ (886 lines; `describe("the design-system audit")`
  at `:596`, `CANARIES` at `:761`, `describe("the audit's own rules, fed a planted tree")` at
  `:849`) — three rule functions, three real-tree tests, six canaries, one exemption test (modified)

#### Interfaces — complete: `develop.css`, the new declarations

The four rules keep every declaration not shown; only the lines below change or are added.

```css
.pane-sub {
  color: var(--ink-3);                         /* human language: rule B does not apply */
  font-family: var(--sans-small);              /* 12px optical size (task 27) */
  font-size: var(--t-meta);
  line-height: var(--t-meta-lh);
  white-space: nowrap;
}

/* The overlay's base voice is sans: it shows a state word, and on failure a sentence. Its one
   machine fact, the retry seconds, is a .pane-fact child. Still a live-number carrier, so the
   numeric token is restated (LIVE_NUMBER_CARRIERS). */
.pane-attach {
  position: absolute;
  z-index: 1;
  pointer-events: none;
  color: var(--ink-2);
  font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans);
  font-variant-numeric: var(--numeric);
}

/* The footer's base voice is mono: three of its four facts are machine facts. The attach word
   is a .pane-voice child. --ink-2, never --ink-3, for a fact (05 §4.4 C3). */
.pane-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  background: var(--surface);
  box-shadow: 0 -1px 0 0 var(--line-soft);
  color: var(--ink-2);
  font: 400 var(--t-fact)/var(--t-fact-lh) var(--mono);
  font-variant-numeric: var(--numeric);
}

.cv-coords {
  display: none;
  font: 500 var(--t-fact)/1 var(--mono);
  font-variant-numeric: var(--numeric);
  color: var(--ink-2);
}

/* voice.split's two spans (Phase W task 30). Longhands, so tabular figures inherit. */
.pane-voice {
  font-family: var(--sans);
}

.pane-fact {
  font-family: var(--mono);
  font-size: var(--t-fact);
  line-height: var(--t-fact-lh);
  color: var(--ink-2);
}
```

`.cv-coords` keeps its comment about the shorthand resetting `font-variant-numeric` (`:389-392`);
`.pane-foot` keeps its padding and gap literals, because `develop.css` is a Phase 2 stylesheet and
rule C does not reach it. `.pane-foot-sep` stays `--ink-4`: a separator is not text.

#### Interfaces — complete: the kept old look, appended to `develop.css`

```css
/* ── The old look (Phase W task 30) ─────────────────────────────────────────────────────
 * Every declaration voice.split replaced, kept so task 32's toggle can show old and new on one
 * screen. :root[data-look="old"] is (0,2,0) plus the selector's own, so it beats the new rule
 * wherever it sits. DELETE THIS BLOCK when the owner accepts the new look (task 39); nothing
 * else depends on it. Rules A and B in designsystem.test.ts skip these selectors by name. */
:root[data-look="old"] .pane-sub {
  font-family: var(--sans);
  font-size: 11px;
  line-height: 1.4;
}

:root[data-look="old"] .pane-attach {
  font-family: var(--mono);
  font-size: 11px;
  line-height: 1.5;
}

:root[data-look="old"] .pane-foot {
  color: var(--ink-3);
  font-size: 11px;
  line-height: 1.4;
}

:root[data-look="old"] .cv-coords {
  color: var(--ink-3);
  font-size: 10px;
}

/* In the old look there was one voice per strip: the spans take their strip's. */
:root[data-look="old"] .pane-voice,
:root[data-look="old"] .pane-fact {
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
  color: inherit;
}
```

#### Interfaces — complete: `palette.css`

`.pal-hint {` (`:115`) becomes `kbd.pal-hint {`, and `font-size: 11px;` becomes
`font-size: var(--t-micro);` (02 C3: kbd hints only). Appended at the end of the file:

```css
/* The old look (Phase W task 30); delete with develop.css's block at task 39. */
:root[data-look="old"] kbd.pal-hint {
  font-size: 11px;
}
```

#### Interfaces — complete: `SessionPaneFooter.tsx`

`ladderElapsed` (`:17-20`) is replaced by `attachParts`; `attachLabel` (`:33-48`) becomes its join,
so its exported contract, and every test that names a phase, is unchanged.

```tsx
/** Task 30 (voice.split). The attach phase in its two voices: the word a person reads (sans)
 *  and the retry seconds, a machine fact (mono). Seconds and not formatElapsed, for the reason
 *  the replaced ladderElapsed gave: a minute count prints "0m" for a whole 60 s ladder. */
export function attachParts(state: AttachState, nowMs: number): { word: string; elapsed: string } {
  const elapsed =
    state.ladderStartedAt === 0 ? "" : `${String(Math.max(0, Math.floor((nowMs - state.ladderStartedAt) / 1000)))}s`;
  switch (state.phase) {
    case "detached":     return { word: "detached", elapsed: "" };
    case "connecting":   return { word: "connecting", elapsed };
    case "attached":     return { word: "attached", elapsed: "" };
    case "reconnecting": return { word: "reconnecting", elapsed };
    case "ended":        return { word: "ended", elapsed: "" };
    case "failed":       return { word: "not attached", elapsed: "" };
  }
}

export function attachLabel(state: AttachState, nowMs: number): string {
  const { word, elapsed } = attachParts(state, nowMs);
  return elapsed === "" ? word : `${word} ${elapsed}`;
}

/** The two voices as spans. Shared with SessionPane's AttachOverlay, so the strip and the
 *  overlay cannot split them differently. */
export function AttachWords({ state, nowMs }: { state: AttachState; nowMs: number }): JSX.Element {
  const { word, elapsed } = attachParts(state, nowMs);
  return (
    <>
      <span className="pane-voice">{word}</span>
      {elapsed === "" ? null : <>{" "}<span className="pane-fact">{elapsed}</span></>}
    </>
  );
}
```

The long comment above `attachLabel` (`:22-32`) moves above `attachParts` unchanged. In the
footer body, the attach entry leaves the `facts` array and is rendered after it:

```tsx
  const facts: string[] = [harness?.command ?? session.harness];
  if (session.model !== "") facts.push(session.model);
  facts.push(formatElapsed(session.createdAt, nowMs));

  return (
    <footer className="pane-foot" title={attach.reason === "" ? undefined : attach.reason}>
      {facts.map((fact, i) => (
        <span key={fact + String(i)} className="pane-foot-fact">
          {i > 0 ? <span className="pane-foot-sep" aria-hidden="true">{" · "}</span> : null}
          {fact}
        </span>
      ))}
      <span className="pane-foot-fact">
        <span className="pane-foot-sep" aria-hidden="true">{" · "}</span>
        <AttachWords state={attach} nowMs={nowMs} />
      </span>
    </footer>
  );
```

The footer's text is unchanged character for character (`SessionPane.test.tsx:536` pins
`"claude · 41m · detached"`), so the built tests hold.
