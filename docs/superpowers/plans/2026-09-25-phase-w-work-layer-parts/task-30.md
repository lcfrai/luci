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

#### Interfaces — complete: `SessionPane.tsx`, the overlay's body

`AttachOverlay`'s one text line (`:101`) becomes:

```tsx
      {failed ? <span className="pane-voice">{attach.reason}</span> : <AttachWords state={attach} nowMs={nowMs} />}
```

with `AttachWords` joining the existing `attachLabel` import from `./SessionPaneFooter`. The comment
above the line (`:97-100`) gains one sentence: "Sans since Phase W task 30: it is a sentence, and
voice.split sets human language in the sans." The overlay's `role`, phase attribute and
centring are unchanged; its text is unchanged (`SessionPane.test.tsx:411` pins
`/^connecting \d+s$/`).

#### Tests to write first — `designsystem.test.ts`, the three rules

Beside the built predicates (after `numericFaults`), three more, in the file's own style:

```ts
/* ── voice.split (Phase W task 30; 05 §4.4 C3) ─────────────────────────────────────────── */

/** The kept old look (task 30), deleted at task 39. Every part of the selector list must be under
 *  it; one new-look selector in the list makes the rule a new-look rule. */
const OLD_LOOK = /^:root\[data-look="old"\]\s/;
const isOldLook = (selector: string): boolean =>
  selector.split(",").every((part) => OLD_LOOK.test(part.trim()));
/** A kbd hint: the kbd element, a .kbd class, or a class ending -kbd. Nothing else. */
const KBD_HINT = /(?:^|[\s,>+~(])kbd\b|\.kbd\b|-kbd\b/;
const setsMono = (r: Rule): boolean =>
  r.decls.some((d) => (d.property === "font-family" || d.property === "font") && d.value.includes("var(--mono)"));

/** Rule A: a mono rule that sets a size sets --t-fact; a kbd hint alone may use --t-micro. */
function monoFactSize(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const rule of rulesOf(t.read(file))) {
      if (!setsMono(rule) || isOldLook(rule.selector)) continue;
      const ok = KBD_HINT.test(rule.selector) ? /var\(--t-(?:fact|micro)\)/ : /var\(--t-fact\)/;
      for (const d of rule.decls) {
        const sized = d.property === "font-size" || (d.property === "font" && d.value !== "inherit");
        if (sized && !ok.test(d.value)) offenders.push(`${file}  ${rule.selector}  ${d.property}: ${d.value}`);
      }
    }
  }
  return offenders;
}

/** Rule B: no mono rule sets --ink-3; a kbd hint is chrome, not a fact, and is excepted. */
function monoFactInk(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const rule of rulesOf(t.read(file))) {
      if (!setsMono(rule) || isOldLook(rule.selector) || KBD_HINT.test(rule.selector)) continue;
      for (const d of rule.decls) {
        if (d.property === "color" && d.value.includes("var(--ink-3)")) offenders.push(`${file}  ${rule.selector}  color: ${d.value}`);
      }
    }
  }
  return offenders;
}

/** Rule C: Phase W stylesheets space only with --s-*. The Phase 2 sheets are not held to it. */
const PHASE_W_DIRS = /^src\/components\/(?:work|issue|desk|glyphs|thread|devtools)\//;
const SPACING = /^(?:padding|margin)(?:-(?:top|right|bottom|left|block|inline)(?:-(?:start|end))?)?$|^(?:row-|column-)?gap$/;
const SPACE_TOKEN = /^(?:0|auto|inherit|initial|unset|var\(--s-[1-6]\))$/;
function phaseWSpacing(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css.filter((f) => PHASE_W_DIRS.test(f))) {
    for (const rule of rulesOf(t.read(file))) {
      for (const d of rule.decls) {
        if (!SPACING.test(d.property)) continue;
        const bad = d.value.split(/\s+/).filter((v) => v !== "" && !SPACE_TOKEN.test(v));
        if (bad.length > 0) offenders.push(`${file}  ${rule.selector}  ${d.property}: ${d.value}`);
      }
    }
  }
  return offenders;
}
```

`rulesOf`, `Rule`, `Tree` and `TOKENS` are the file's own (`:62`, `:68-73`, `:109-115`). A mono
rule inside `@keyframes` cannot occur (keyframes set no font), so these rules need no `within`
filter.

#### Tests to write first — `designsystem.test.ts`, over the real tree and the planted one

Inside `describe("the design-system audit")` (`:596`), after the tabular-figures test:

```ts
  // Kills: .pane-foot, .pane-attach or .cv-coords left at 11px or 10px (05 §4.4 C3), and
  // .pal-hint left at a literal 11px under a selector that does not say it is a kbd hint.
  it("sets every mono fact at --t-fact, and --t-micro only on a kbd hint", () => {
    expect(monoFactSize(REAL)).toEqual([]);
  });

  // Kills: facts left in --ink-3 (.pane-foot, .cv-coords today).
  it("never sets a mono fact in --ink-3", () => {
    expect(monoFactInk(REAL)).toEqual([]);
  });

  // Kills: a literal padding, margin or gap in a Phase W stylesheet (task 26's deferred rule).
  it("spaces the Phase W stylesheets only with --s-*", () => {
    expect(phaseWSpacing(REAL)).toEqual([]);
  });
```

Appended to `CANARIES` (`:761`), with one new constant beside `PLANT_CSS` (`:731`):

```ts
const PLANT_WORK_CSS = "src/components/work/planted.css";
```

```ts
  // Phase W task 30. Each proves its rule fires; none can pass on the real tree by accident,
  // because the planted files do not exist there.
  { fault: "a mono fact at 11px", rule: monoFactSize, file: PLANT_CSS, text: sheet(".p { font: 400 11px/1.4 var(--mono); }") },
  { fault: "a mono size that is a literal, even the right one", rule: monoFactSize, file: PLANT_CSS, text: sheet(".p { font-family: var(--mono); font-size: 12px; }") },
  { fault: "--t-micro on a selector that is not a kbd hint", rule: monoFactSize, file: PLANT_CSS, text: sheet(".p { font-family: var(--mono); font-size: var(--t-micro); }") },
  { fault: "a mono fact in --ink-3", rule: monoFactInk, file: PLANT_CSS, text: sheet(".p { font-family: var(--mono); color: var(--ink-3); }") },
  { fault: "a literal padding in a Phase W stylesheet", rule: phaseWSpacing, file: PLANT_WORK_CSS, text: sheet(".w { padding: 10px; }") },
  { fault: "a literal gap in a Phase W stylesheet", rule: phaseWSpacing, file: PLANT_WORK_CSS, text: sheet(".w { gap: 6px var(--s-2); }") },
```

`sheet()` (`:754`) opens with `HEAD`, `@import "../../styles/tokens.css";`, which is also right for
`src/components/work/`, the same depth. And one test after the canary loop in
`describe("the audit's own rules, fed a planted tree")` (`:849`), because an exemption a rule
never exercises is a hole nobody sees:

```ts
  // Kills: an exemption that has grown (a kbd hint excused from --t-fact but NOT from --t-micro,
  // or the old look excusing a new-look selector that shares its list).
  it("excuses a kbd hint and the kept old look, and nothing else", () => {
    const planted = plant(PLANT_CSS, sheet([
      ".x-kbd { font-family: var(--mono); font-size: var(--t-micro); color: var(--ink-3); }",
      ':root[data-look="old"] .p { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }',
      '.q, :root[data-look="old"] .q { font-family: var(--mono); font-size: 11px; }',
      ".y-kbd { font-family: var(--mono); font-size: 11px; }",
    ].join("\n")));
    const on = (o: string[]) => o.filter((x) => x.startsWith(`${PLANT_CSS}  `));
    expect(on(monoFactSize(planted)).map((o) => o.split("  ")[1])).toEqual([
      '.q, :root[data-look="old"] .q',
      ".y-kbd",
    ]);
    expect(on(monoFactInk(planted))).toEqual([]);
  });
```

#### Tests to write first — `SessionPane.test.tsx`

jsdom applies no stylesheet (`vitest.config.ts`, `css` limited to `?raw`), so a voice is proven the
way the audit proves everything: which class carries the text, and what the stylesheet says that
class is. One import joins the file's head, `import developCss from "./develop.css?raw";`, and one
helper sits above the `describe`:

```ts
/** The declarations of one rule in develop.css, by exact selector; "" when absent. */
function ruleIn(css: string, selector: string): string {
  const esc = selector.replace(/[.[\]"=()]/g, (c) => `\\${c}`);
  return new RegExp(`(?:^|\\n)${esc}\\s*\\{([^}]*)\\}`).exec(css.replace(/\/\*[\s\S]*?\*\//g, ""))?.[1] ?? "";
}
```

```tsx
  // Kills: the human word set in mono, and a fact moved out of the mono strip.
  it("renders facts mono and the attach word sans", () => {
    attachState.current = { phase: "detached", attempts: 0, ladderStartedAt: 0, reason: "" };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);
    const foot = container.querySelector(".pane-foot")!;
    const voices = [...foot.querySelectorAll(".pane-voice")].map((e) => e.textContent);
    expect(voices).toEqual(["detached"]);                          // only the word is sans
    expect(foot.textContent).toBe("claude · opus · 41m · detached"); // nothing else moved
    expect(ruleIn(developCss, ".pane-foot")).toContain("var(--t-fact)/var(--t-fact-lh) var(--mono)");
    expect(ruleIn(developCss, ".pane-foot")).toContain("color: var(--ink-2)");
    expect(ruleIn(developCss, ".pane-voice")).toContain("font-family: var(--sans)");
  });

  // Kills: attach.reason, a sentence, left in the mono overlay; and retry seconds set in sans.
  it("sets the attach reason sans and the retry seconds mono", () => {
    attachState.current = { phase: "failed", attempts: 9, ladderStartedAt: 0, reason: "The sandbox is not running." };
    const first = render(<SessionPane sessionId={seed().id} variant="column" />);
    const overlay = first.container.querySelector(".pane-attach")!;
    expect(overlay.querySelector(".pane-voice")?.textContent).toBe("The sandbox is not running.");
    expect(overlay.querySelector(".pane-fact")).toBeNull();
    first.unmount();
    attachState.current = { phase: "connecting", attempts: 1, ladderStartedAt: Date.now() - 3_000, reason: "" };
    const second = render(<SessionPane sessionId={seed({ status: "starting" }).id} variant="column" />);
    const again = second.container.querySelector(".pane-attach")!;
    expect(again.querySelector(".pane-voice")?.textContent).toBe("connecting");
    expect(again.querySelector(".pane-fact")?.textContent).toMatch(/^\d+s$/);
    expect(ruleIn(developCss, ".pane-attach")).toContain("var(--sans)");
    expect(ruleIn(developCss, ".pane-fact")).toContain("font-size: var(--t-fact)");
  });

  // Kills: the header's split reversed (a name a person chose, set in mono).
  it("keeps the worktree name and the harness name sans", () => {
    const { container } = render(<SessionPane sessionId={seed().id} variant="column" />);
    expect(container.querySelector(".pane-title")?.textContent).toBe("evidence-upload");
    for (const sel of [".pane-title", ".pane-sub", ".pane-head"]) {
      expect(ruleIn(developCss, sel), sel).not.toContain("var(--mono)");
    }
    expect(ruleIn(developCss, ".pane-sub")).toContain("font-size: var(--t-meta)");
  });

  // Kills: a change that cannot be flipped back (task 32's toggle needs every old value).
  it("restores the old declarations under data-look=old", () => {
    const old = (sel: string) => ruleIn(developCss, `:root[data-look="old"] ${sel}`);
    expect(old(".pane-foot")).toMatch(/color: var\(--ink-3\);[\s\S]*font-size: 11px;[\s\S]*line-height: 1\.4;/);
    expect(old(".pane-attach")).toMatch(/font-family: var\(--mono\);[\s\S]*font-size: 11px;/);
    expect(old(".cv-coords")).toMatch(/color: var\(--ink-3\);[\s\S]*font-size: 10px;/);
    expect(old(".pane-sub")).toMatch(/font-family: var\(--sans\);[\s\S]*font-size: 11px;/);
  });
```

The model fact `"opus"` and the elapsed `"41m"` come from the file's own `seed()` fixtures, as
the built test at `:368-373` reads them. Each new test sets `attachState.current` before it
renders, as the built tests at `:397-411` do, so none depends on the order the file runs in.

#### Steps

1. Write the three audit predicates, the three real-tree tests, the six canaries and the
   exemption test. Run `npm --prefix desktop test -- designsystem`: the canaries and the exemption
   test pass (the predicates are right); the real-tree tests fail, naming `.pane-attach`,
   `.pane-foot`, `.cv-coords` and `.pal-hint` for rule A and `.pane-foot`, `.cv-coords` for rule B.
   That list *is* the audit of the built tree; if it names anything else, stop and add it to this
   task's table before fixing it.
2. Write the four `SessionPane.test.tsx` tests. They fail (no `.pane-voice`, no old look).
3. Change the four `develop.css` rules; add `.pane-voice`, `.pane-fact` and the old-look block.
   Change `palette.css`. Re-run the audit: all pass.
4. Add `attachParts` and `AttachWords` to `SessionPaneFooter.tsx`; change the footer body and
   `AttachOverlay`'s line. Re-run `npm --prefix desktop test -- designsystem develop`: all pass,
   the built footer and overlay tests included.
5. Mutate by hand and watch the named test go red, then revert: `.pane-foot` back to `11px`;
   `.cv-coords` back to `--ink-3`; `padding: 10px` in `glyphs.css`; the reason back in a bare
   text node; `.pane-title` given `var(--mono)`; delete the old `.pane-sub` rule; drop `-kbd\b`
   from `KBD_HINT` (the rail's `.rail-kbd` then fails rule B).
6. Typecheck; run the whole desktop suite; commit.

#### Acceptance

```bash
npm --prefix desktop test -- designsystem develop
# expect: all pass; the audit reports 3 new real-tree tests, 6 new canaries, 1 exemption test

npm --prefix desktop run typecheck
# expect: exit 0

grep -n "var(--mono)" desktop/src/components/develop/develop.css
# expect: .pane-foot and .cv-coords with var(--t-fact) in the same declaration, .pane-fact,
# and the old-look .pane-attach rule; no line with 10px or 11px beside var(--mono)

grep -n "11px" desktop/src/components/develop/develop.css
# expect: every hit that belongs to .pane-foot, .pane-attach or .pane-sub is inside a
# :root[data-look="old"] rule; the other hits (:216, :226, :451, sans rules) are out of scope
```

The scope's acceptance grepped for the literal `11px/1.4 var(--mono)` inside the old block. The
old look is written in longhands (correction 4), so that string no longer exists anywhere; the two
greps above are the check.

#### What this task does not claim

- **Not the rail or the Discover lens.** Tasks 31 (the rail's `chrome.dim`) and the Discover
  tasks touch them. Rules A and B pass on the rail today only because `.rail-kbd` sets no size and
  is a kbd hint.
- **Not the three sans candidates** (task 32), nor the toggle that sets `data-look` (task 32).
  Nothing sets `data-look` here, so every user sees the new look.
- **Not that the split reads better.** The owner judges at task 39, which also deletes the old
  look.
- **Not the other `11px` sans rules in `develop.css`** (`:216`, `:226`, `:451`): human language,
  outside rules A and B. Whether they rise to `--t-meta` is a type-scale pass, not this one.
- **Not the terminal's own text**, which xterm renders from `--mono` at 12px (task 27).
- Design: 05 §4.4 C3; 02 C3, D2 step 3; 07 D I7. Parity: no 01 row; it is one of 02 C13's five
  things every screen carries that Linear does not, `voice.split`.

**Interfaces:**
- Produces:
  - `SessionPaneFooter.tsx`: `attachParts(state, nowMs): { word: string; elapsed: string }`,
    `AttachWords({ state, nowMs })`; `attachLabel` unchanged in contract
  - `develop.css`: `.pane-voice` (sans) and `.pane-fact` (mono, `--t-fact`, `--ink-2`), the two
    classes any later Develop surface uses for voice.split
  - `designsystem.test.ts`: `monoFactSize`, `monoFactInk`, `phaseWSpacing`, `PHASE_W_DIRS`,
    `KBD_HINT`, `isOldLook`; every later Phase W stylesheet is audited by them from the moment it
    exists
  - the `:root[data-look="old"]` convention, which task 32's toggle sets and task 39 deletes
- Consumes: task 26's `--t-fact`, `--t-meta` and their `-lh`, `--s-*`; task 27's `--mono`,
  `--sans-small`; the audit's `rulesOf`, `Rule`, `Tree`, `plant`, `sheet`, `CANARIES`
- Amends: nothing in the Hub. No wire shape changes.
