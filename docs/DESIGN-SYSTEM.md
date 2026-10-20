# Luci — design system

This document records the original design direction. [The current implementation authority](../DESIGN.md) resolves it against Luci's shipped application and UAT findings.

## The tells we must never ship

Verified anti-patterns that mark an interface as AI-generated:

1. **Indigo→violet gradient** (`#6366f1`/`#8b5cf6`). The single strongest tell. Banned outright.
2. **Gradient hero + centred headline + two buttons + three-card feature grid.** The default composition of every generated page.
3. **Blanket glassmorphism.** `backdrop-blur` on nav, cards, modals and sidebar at once. Also expensive — forces a GPU layer per element and degrades text rendering.
4. **One radius and one shadow everywhere** (`rounded-2xl shadow-lg`).
5. **Motion with no job.** Entrance animations replaying on every visit, staggered fade-ups on every card, decorative beams/meteors inside app chrome, `transition: all`.
6. Emoji as icons; mixed icon weights; dark mode made by inverting rather than re-deriving surfaces; copy that says "Unlock the power of"; centred text in data-dense regions.

## Token structure

Modelled on the Beautiful UI (beautifului.dev) token layer, which is the best-considered vocabulary in the ecosystem for agent interfaces. We rebuild it rather than depend on it — that site ships no repo, and the community mirrors are reconstructions of a minified bundle.

**Surfaces, back to front:** `page` → `canvas` → `surface` → `inset` → `hover` → `hover-2` → `field`.
Depth comes from stacking these, not from blur.

**Ink ramp:** `ink` (primary text) → `ink-2` (secondary) → `ink-3` (tertiary/disabled).

**Hairlines:** `line-soft` → `line` → `line-strong`. Hairlines carry structure; shadows carry elevation.

**Elevation:** flat surfaces and hairlines establish the workspace. Reserve layered shadows for overlays. Scale tokens remain available for controls that need elevation.

**Radius scale tied to element size:** chip 6 / control 8 / card 10 / window 14.

**Semantics:** accent, green, amber, red — each with a matching low-alpha `-tint` for backgrounds. Colour is signal, never decoration. If a colour is not carrying state, it should be neutral.

**Colour space:** OKLCH, so light and dark are derived rather than inverted. Equal numeric steps in OKLCH read as equal *perceptual* steps; hand-picked hex ramps go muddy or banded in the low-lightness range these tools live in.

The token rule governs Luci's interface, not the contents of a saved Design document. The scene wire schema stores literal six-digit colours or `transparent`, so documents retain their appearance across themes and exports. `api/designScene.ts` keeps its four existing default colours in one private, frozen document palette. The audit pins those exact values and location, checks factory output against the scene schema, and still rejects literals elsewhere in that file or in interface code. This is not a second UI palette.

### Locked palette

- **Base: warm graphite, not blue-tinted.** Nearly every generated dark UI is cold-tinted, so warmth reads as considered on sight.
- **Accent: deep teal `#2fb3bd`** — maximally distant from the indigo/violet slop tell, and ~50° off running-green so the two never read as the same signal. Used **only** for interactive and current state. Never on cards, icons, borders or chart fills.
- **Status is a separate vocabulary that never collides with the accent:** running `#43b95e`, needs-you `#e2a33c`, failed `#ef5f45`, idle/done neutral. Each carries a shape as well as a colour — colour alone fails for colourblind users.

If the accent starts appearing decoratively, the hierarchy is broken. When everything is emphasised, nothing is.

### Typography

The shipped app uses system sans-serif and monospace stacks for offline operation and zero additional font cost. Bundled open-licensed Excalidraw fonts serve the design canvas. No purchased font is required.

Mono is also permitted **once** at display scale (the zero-state headline). A product called a cloud computer is allowed to speak in machine voice exactly once; doing it everywhere is costume.

### Surface clarity

The application has no decorative grain layer. Warm surfaces, readable type, consistent controls and restrained teal state accents establish its identity.

**Easing:** one house curve on nearly every transition — `cubic-bezier(0.23, 1, 0.32, 1)`. Durations under 300ms, `ease-out` by default. Springs only where a cursor or finger supplies the velocity (drag, sheets, reorder, resize).

## Designing for many live processes

This is the hardest part of the product and where most tools fail.

- **One global heartbeat clock.** Every live indicator shares a phase. Ten agents breathing on one timeline reads as a system under control; ten independently-phased spinners read as noise.
- **One low-amplitude signal per row** — a 1px indeterminate hairline, or opacity oscillation on a status dot. Never a spinner per row.
- **State changes flash and decay** over ~600ms (`opacity`/`background` only). The eye catches transitions, not steady states.
- **Tabular numerals** (`font-variant-numeric: tabular-nums`) on every live number — tokens, cost, elapsed, queue depth. Without it digits jitter and the interface reads cheap.
- **Throttle render.** Log panes batch appends at ~10Hz and never scroll-jump when the user has scrolled up.

## Interaction standards

- **Optimistic mutation with an Undo window**, not confirm dialogs, for agent actions (pause, kill, retry, approve).
- **Shared-element transitions** on drill-in via `view-transition-name` (row → detail). Native View Transitions API, ~92% support, capability-checked, zero JS payload.
- **FLIP layout animation** for queue reordering (Motion `layout`, ~200ms, interruptible). Never animate `top/left/width/height`.
- **⌘K palette scoped to the current selection.** Stable ranking; rows never reflow while filtering; hotkey hints inline so it teaches itself.
- **Hover intent:** first tooltip delayed ~300ms, peers instant. Hover never changes layout — change background or ring, not padding or border-width.
- **Skeletons mirror final geometry exactly.** A skeleton that causes one pixel of layout shift is worse than no skeleton.
- **Focus rings are a designed element.** `:focus-visible` and `:focus-within` grouping — keyboard is a primary input here, not an accessibility afterthought.

## Stack

- Tailwind v4 + shadcn/ui (MIT) as the base — we own the code.
- Our own token layer, per the structure above. This does more anti-templating work than any component library.
- Motion v13 (MIT) for React-stateful motion only: `layout` reorder, `AnimatePresence`, drag. Import via `LazyMotion` + `m`.
- Native View Transitions + CSS transitions/WAAPI for everything else. Preference order: **CSS > WAAPI > JS library**.
- Kibo UI (MIT) for app-grade composites (code block, table, kanban, dropzone).
- Beautiful UI as a *specification* to rebuild: thinking state, streaming text, tool chips, approval card, task rows, prompt bar, diff table.

**Excluded deliberately:** Aceternity UI and React Bits (their visual signature *is* the AI-generated look; both also non-OSI). Lenis (hijacks native scroll, breaks dense lists). GSAP (non-OSI licence for capability we do not need). HeroUI (inherits a look wholesale). **Blocked pending legal:** coss.com/ui, formerly Origin UI — now AGPL-3.0.

## Floor

`prefers-reduced-motion` variants on everything. `transform`/`opacity` only. No `transition: all`. Virtualise any list over 50 rows. Every state designed: loading, empty, sparse, dense, partial, stale, offline, unauthorized, error, blocked, success.
