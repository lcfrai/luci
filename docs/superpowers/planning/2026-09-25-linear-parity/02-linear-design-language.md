# 02 — Linear's design language and UX craft, and what PanaceaApp does with it

**Date:** 2026-09-25. **Stream:** linear-parity, research 02 of the set. **Status:** research + proposal; nothing here is built.

**The owner's ask, verbatim:** *"I have just seen a product called linear.app. I need you to make sure that we are doing everything they are doing and more. Make it prettier, better, everything. Mixed with the other benefits we have found, and the ability for agents across owners to work together, and I think we have something magic."*

**Scope of this file:** why people call Linear beautiful and fast, and the exact mechanisms behind each claim. Then a point-by-point comparison with `docs/DESIGN-SYSTEM.md` and `docs/AGENTIC-UX.md`, and a named-token uplift for PanaceaApp. Feature parity (issues, cycles, projects, triage) belongs to the sibling research files, not this one.

## The answer in ten lines

1. Linear is **fast** because it is local-first: every click writes to a local copy and syncs later, so almost nothing waits (A6).
2. Linear is **beautiful** because of discipline, not decoration: generated LCH themes, neutral chrome that recedes, colour only as signal, fewer and smaller icons, obsessive alignment (A1–A4, A10).
3. Its keyboard never opens a page: `⌘K`, single-key verbs, `G`-sequences, `J`/`K`, and `Space` to peek (A7).
4. It shows agents one per issue, as a conversation, with 6 states and a 10-second first-response rule. It shows **no** agent-to-agent or cross-owner coordination anywhere we read (A9).
5. The "Linear look" that others copy (glow, gradient, glass, bento) is its marketing site, and it is our banned list (A11).
6. We already **match** it on colour discipline, elevation, motion rules and the palette: 12 of 30 points (Part B).
7. **Adopt 13:** a 3-seed theme generator, light and high-contrast themes, a dim rail and one view header, an icon rule, peek, key sequences (outside terminals only), a local Hub replica with speed budgets, and Linear's stress-test habit (C1, C4–C9, C12).
8. **Beat them 6 ways:** mono machine voice, evidence-peek, empty states that carry a projection, meaning lines instead of chat, 30 agents on one heartbeat with a projected horizon, and Orchestrators that negotiate across owners (C3, C9, C10, C11).
9. **Don't copy 6:** Inter, demo-data onboarding, progress-filling status icons, plan step counts, chat-thread activity, "optimistic everywhere" for irreversible acts.
10. Warmth stopped being our differentiator in March 2026, when Linear went warm-grey. Our signature becomes the provenance **thread** plus the mono machine voice (C2, C3, C13).

**Binding inputs:** `OD-1`…`OD-20` (`2026-09-23-owner-decisions.md`), especially `OD-14`…`OD-17` (team agents run centrally; one Orchestrator per user is the only cross-user channel; agent awareness is primary, the human view is a calm summary), `OD-19` (org inference, ZDR) and `OD-20` (cloud-first, the desktop is the portal).

## Clean-room statement

- Linear is proprietary. Only its public marketing site, docs, changelog, blog, method writing and public API docs were read. No Linear asset, icon, brand, copy or code is reproduced here. Where a mechanism is described, it is described in our words, from our reading, with the URL.
- Plane is AGPL-3.0. Its source was not opened. Nothing in this file depends on Plane.
- Every URL relied on is in **§ Sources**, with the date read (all 2026-09-25 unless stated).
- Every claim not confirmed from a fetched page is marked **UNVERIFIED**. The corporate network blocks some sites; where a fetch failed, § Sources says so.

---

# Part A — How Linear does it

## A1. Colour and theming

**Mechanism: a theme is generated from three inputs, not hand-tuned.** The 2024 redesign post (S1) says they cut each theme from 98 hand-set variables to **3: base colour, accent colour, contrast**, and generate the rest in **LCH**. Their stated reason: LCH is perceptually uniform, so a red and a yellow at the same lightness value read as equally light. The contrast input also generates high-contrast themes for accessibility for free. The same generator drives the built-in light and dark themes and every user custom theme.

**What that buys them.** One generator means every surface step, hover, border and selected state is derived, so no theme ships a muddy or banded ramp and a user-made theme cannot break the hierarchy. It is the reason "custom theme" in Linear does not look amateur.

**2026: they moved warm.** The March 2026 refresh post (S2, "A calmer interface for a product in motion") says the old palette was a cool, blue-ish hue and the new one inches "toward a warmer gray that still feels crisp, but less saturated". The named risk was muddiness, controlled by holding saturation down. Token values are kept as JSON and pushed into Figma by an in-house plugin; the colour picker they tuned with was itself built with Claude Code (S2).

**Colour discipline.** Colour sits almost only on status icons, priority, labels and the single accent; chrome is neutral. The 2026 refresh also **removed coloured backgrounds behind icons** (S2). This is the same "colour is signal" rule as ours.

> **Consequence for us, and it is material:** `DESIGN-SYSTEM.md` locks "warm graphite, not blue-tinted" because "nearly every generated dark UI is cold-tinted". Since 2026-03-12 the most-copied product in the category is also warm-grey. Warmth alone no longer differentiates; it now reads as "Linear-like". Part C answers this.

## A2. Typography

- **Inter for UI text, Inter Display for headings** (S1). Display adds "expression" at heading sizes while body stays neutral. Exact sizes and weights are not published: **UNVERIFIED** beyond the faces.
- The look people call "Linear style" leans on **medium-weight, small, tight** UI text and large, tightly-tracked marketing headlines. Specific sizes (13px UI, negative tracking on display) are widely repeated in third-party write-ups but are **UNVERIFIED** from Linear's own pages.
- Mono appears for IDs and code only (**UNVERIFIED**: observed in public screenshots, not documented).

## A3. Density, chrome and hierarchy

- **The inverted-L.** The 2024 post frames global chrome as an inverted L (sidebar down the left, header across the top) that frames and controls content (S1). The 2024 work was mostly *alignment*: labels, icons and buttons in the sidebar lined up on shared vertical and horizontal axes across a tiny area.
- **Chrome recedes, content leads.** 2026 made the sidebar "a few notches dimmer" so the work area takes precedence (S2), and made headers, navigation and view controls identical across projects, issues, reviews and documents (S3).
- **Softer separators.** Borders were given rounded ends and lower contrast: "structure without noise" (S2).
- **Tabs shrank.** Tabs stopped spanning the full width; they are compact, rounded, with smaller text and icons (S2).
- **Stress-tested, not workshopped.** The 2024 redesign was checked against three stress sets: environment (macOS, Windows, browser, removable nav), appearance (light, dark, custom, contrast) and hierarchy (list, board, timeline, split, fullscreen) (S1). Six weeks, feature-flagged, with a developer-toolbar toggle to flip old and new for instant A/B, then a daily percentage rollout (S1).

## A4. Iconography

- A single in-house icon family, drawn on one grid, one stroke weight (**UNVERIFIED** on exact grid and weight).
- 2026: icons "redrawn and resized" (S3); the refresh "reduces icon usage, scales their sizes down, and removes unnecessary visual treatments" (S2). The direction is **fewer, smaller, uncoloured icons**.
- **Status icons carry shape as well as colour.** The workflow-state icons are a circle that fills with progress through backlog, todo, in progress, done, cancelled (**UNVERIFIED** from docs; observed in public product screenshots). This is the same shape-plus-colour rule our status vocabulary already mandates.

## A5. Motion and its timing

- **Linear publishes no motion tokens.** No duration, easing or spring value appears in any post read. Every specific timing attributed to Linear in third-party write-ups is **UNVERIFIED**.
- What is observable and documented is **restraint**: in-app motion is short and functional (menus, peek, palette). Decorative motion lives on the marketing site, not in the product (**UNVERIFIED** as a stated policy; consistent with every public changelog clip).
- **The craft is in geometry, not animation.** The best-documented "motion" work is the submenu *safe triangle* (S7): a `clip-path` polygon drawn between the cursor and an open submenu so a diagonal mouse path does not close it. About 40 lines of React. No hover timeout needed, because the geometry replaces the timer. This is the kind of invisible detail people mean by "it feels right".

## A6. Perceived speed: the sync engine and optimistic UI

This is the single biggest reason people call Linear fast. It is architecture, not UI polish.

- **Local-first.** Each client keeps a local database, so creating an issue, changing its status or navigating "doesn't require a network round trip" (S8, August 2026). The UI reads from the local copy, never from a request.
- **Optimistic by construction.** A change is applied locally and shown at once; the server is a sync target (S8; the in-memory object-graph detail is from Linear's own sync-engine talk, S9, whose video content could not be read here: **UNVERIFIED** beyond the landing page).
- **Delta catch-up.** A client sends a checkpoint, the ID of the last change it applied, and receives only later changes, replayed from an application-level log into its local database (S8). The largest workspaces produce close to **one million sync actions per day**; a client that was offline for hours can be **hundreds of thousands of actions behind**, and must catch up fast (S8).
- **Offline is a failsafe, not a feature** (S10, Linear docs as quoted in search results; **UNVERIFIED** page not fetched directly).
- **Consequence:** there are few loading states to design, because almost nothing waits. Skeletons are rare. Perceived speed is the absence of waiting, not the presence of a clever spinner.

## A7. Command menu, keyboard and peek

- **`⌘K` / `Ctrl K` opens the command menu from anywhere** (S11). It is contextual: it acts on the current selection or focused item.
- **Single-key verbs on the focused item.** `C` creates an issue in any view; `X` selects; `Shift ↑/↓` or `Shift`-click extends selection; `Esc` goes back or clears (S11).
- **Two-key "go" and "open" sequences.** `G` then `I` (Inbox), `G` then `V` (current cycle), `G` then `B` (backlog); `O` then `F` / `P` / `C` (favourites, projects, cycles) (S11).
- **`?` opens the full shortcut sheet** (S11, and the 2021 changelog S12). Menus and tooltips show each item's shortcut inline, so the mouse path teaches the keyboard path (S7 mentions shortcuts shown in context menus).
- **Peek (S13).** `Space` on a focused row opens a preview. **Tap** toggles it open; **hold** previews and closes on release. `↑`/`↓` move through neighbouring rows and the preview follows. `Esc` closes. Peek works inside the command menu as you move. It is **keyboard-only**: hover does not trigger it. For an issue it shows description, assignee, status, priority, cycle, labels, estimate, created and updated.
- **`J`/`K` or `↑`/`↓` move through a list** (S16).
- **Why it feels fast:** the keyboard path never opens a page. Triage is `J`/`K` + `Space` + a verb key, with the list staying in place.

## A8. Empty states and onboarding

All of this section comes from **third-party teardowns** (S14, S15), not from Linear's own pages. Treat each point as **UNVERIFIED** against the current product.

- **Minimal first run:** sign-in, workspace, **light or dark first**, then the user is asked to press the shortcut for the command menu, before anything else exists. The interaction model is taught before the data model.
- **No role picker, no permission setup, no workflow configuration** at first run: opinionated defaults instead.
- **The workspace is not empty:** it is pre-populated with example issues that show a well-kept workspace, so the user learns the ideal state by seeing it.
- **Each empty state teaches one concept**, with a small illustration and a single call to action.
- The principle behind it is Linear's own, from the Linear Method: **"Simple first, then powerful"**, and "a tool should work for you, not the other way around" (S4, verified).

## A9. How Linear shows agent activity

Linear treats agents as **teammates inside the tracker**. The mechanism is a published contract, the Agent Interaction Guidelines (S17) and the agent-session API (S18).

- **Delegation, not assignment.** Delegating an issue to an agent keeps the human as primary assignee and adds the agent as a contributor working on their behalf (S19, S20). Custom views can filter by **Delegate**; Insights can slice by Delegate; each agent has a user page of its activity (S20).
- **Six session states**, set by Linear from what the agent emits: `pending`, `active`, `error`, `awaitingInput`, `complete`, `stale` (S18).
- **Five agent activity types:** `thought`, `action` (a tool call, with an optional result), `elicitation` (asks the human), `response` (done), `error`; plus the human's `prompt` (S18). Only `thought` and `action` may be **ephemeral**: shown until the next activity replaces them (S18).
- **A plan is a checklist** of steps, each `pending`, `inProgress`, `completed` or `canceled`, replaced whole on each update (S18).
- **Response-time contract:** acknowledge the webhook within **5 s**; emit a first activity within **10 s** or the session is marked unresponsive (S18). This is the "instant feedback" guideline made enforceable.
- **Six guidelines** (S17): disclose that you are an agent; inhabit the platform natively; give instant feedback; be clear about internal state; respect requests to disengage; an agent cannot be held accountable (a human always is).
- **Rendering:** activities appear in the issue's conversation thread, as Markdown (S18). An `externalUrls` field puts an "Open" button to the agent's own dashboard (S18). Coding sessions link to an issue; their diffs appear in a **Reviews** tab; you are notified when your input is needed (S21).
- **Linear Agent** (their own agent): a chat on `⌘J` / `Ctrl J`, `@Linear` in comments, open chats as toolbar **tabs with unread and working indicators** (S22).
- **Positioning:** agents are "artificial teammates" that "can work across multiple issues simultaneously" (S23, S24).
- **Their own stated view of chat:** Linear's co-founder writes that "a chat interface is a very weak and generic form", and that AI should sit on top of a structured "workbench" (S5, April 2025). Yet agent activity still renders as a conversation thread per issue. Our AGENTIC-UX anti-pattern 14 takes the argument to its end.

**What Linear does not show, from every page read:** agent-to-agent coordination, conflict between agents, cross-owner negotiation, a projection of what each agent will need next, or any fleet-level view of many agents at once beyond filtered issue lists. S23 and S24 were read specifically for this and contain **no** statement about agents coordinating with each other. The agent UI is **per issue, conversational, and human-to-agent**.

## A10. So why "beautiful and fast": the eight mechanisms

1. **Nothing waits.** Local-first sync makes every mutation and navigation local (A6). This is most of "fast".
2. **The keyboard never opens a page.** `⌘K`, single-key verbs, `G`/`O` sequences, `J`/`K` and Space-peek keep the user in the list (A7).
3. **Themes are generated, not painted.** Three inputs in LCH produce a ramp that cannot go muddy, in any theme (A1).
4. **Chrome recedes.** Dim sidebar, identical headers everywhere, softer separators, compact tabs, fewer smaller uncoloured icons (A3, A4). The content is the brightest thing on screen.
5. **Colour is signal.** Neutral chrome; colour on status, priority and labels only (A1).
6. **Alignment is obsessive.** The 2024 redesign was mostly alignment on shared axes (A3).
7. **Invisible geometry beats timers.** The submenu safe triangle (A5).
8. **Quality is a policy, not a phase.** A zero-bugs rule (fix within 7 days), 3–5 person teams with taste, MVPs internal-only, stress tests across OS, theme and layout before shipping (S1, S25).

## A11. The "Linear look" as imitated, which is not the same thing

The trend named after Linear (S26, S27) is mostly the **marketing site**, not the app: near-black backgrounds, one saturated accent, linear gradients and glows, blurred glass, bento grids, big tight sans headlines, micro-motion. The trend pieces themselves list glassmorphism, gradient glows and bento grids as the signature (S26, S27). Three of those items are on `DESIGN-SYSTEM.md`'s banned list (tells 1–3). **The imitated look is exactly the machine-generated look our design system exists to avoid.** What makes Linear itself good is A10, and most imitators copy none of it.

Linear's own public brand page lists two neutrals only, an off-white (`#F4F5F8`) and a near-black graphite (`#222326`), and no typeface (S28). The in-product accent is widely described as a desaturated indigo: **UNVERIFIED** from a Linear page, and irrelevant to us, because indigo is banned outright (tell 1).

---

# Part B — Point by point against `DESIGN-SYSTEM.md` and `AGENTIC-UX.md`

Verdicts: **MATCH** (we already have it) · **ADOPT** (take the mechanism, in our own form) · **BEAT** (we can go past them) · **DON'T COPY**. "Built" means it is in `desktop/src` today; everything else is spec only.

| # | Topic | Linear (Part A) | PanaceaApp today | Verdict |
|---|---|---|---|---|
| 1 | Colour space | LCH generator, 3 inputs (A1) | OKLCH, hand-set per token in `desktop/src/styles/tokens.css` (built) | **ADOPT** the generator: we have the right space but 30+ hand-picked values, so a light or high-contrast theme is a rewrite |
| 2 | Base hue | Warm grey since 2026-03-12 (A1) | Warm graphite, hue 67 (built) | **MATCH, and it stopped differentiating.** Keep it; differentiate elsewhere (C2) |
| 3 | Accent | Neutral brand; in-app accent UNVERIFIED | Teal `#2fb3bd`, interactive and current state only (built) | **MATCH** on discipline, **distinct** on hue |
| 4 | Colour as signal | Status, priority, labels; icon backgrounds removed (A1, A4) | Accent and status are separate vocabularies, each status has a shape (built) | **MATCH**, stricter than theirs |
| 5 | Themes | Light, dark, custom, high contrast from one generator (A1) | Dark only (`color-scheme: dark`, built) | **ADOPT**: light and high-contrast, generated, not a custom-theme picker |
| 6 | UI face | Inter; Inter Display headings (A2) | Spec: Geist + licensed mono. Built: Segoe UI / Cascadia fallbacks | **DON'T COPY** Inter. **ADOPT** a second optical size for headings, in our own face |
| 7 | Mono | IDs and code (UNVERIFIED) | Licensed mono, plus once at display scale (spec) | **BEAT**: make mono the voice of every machine fact (C3) |
| 8 | Density and chrome | Inverted-L, dim sidebar, one header pattern everywhere (A3) | Rail is "navigation only, no counts" (AGENTIC-UX §5) | **ADOPT** the dim rail and the single view header; **MATCH** on rail austerity |
| 9 | Separators | Softer, lower contrast (A3) | Hairlines `line-soft`/`line`/`line-strong` as alpha (built) | **MATCH** |
| 10 | Elevation | Not published | 1px ring + layered shadow, 5 steps (built) | **MATCH or better**: ours is specified |
| 11 | Icons | One family; fewer, smaller, uncoloured (A4) | No icon rule in DESIGN-SYSTEM beyond "no emoji, no mixed weights" | **ADOPT** the rule and a size/stroke token pair (C6) |
| 12 | Status glyphs | Circle that fills with progress (UNVERIFIED) | Disc / square shapes; idle still, running breathes (built) | **DON'T COPY**: a filling circle is a progress denominator, banned by AGENTIC-UX anti-pattern 2 |
| 13 | Motion values | None published (A5) | One curve, 120/180/260 ms, `transform`/`opacity` only (built) | **MATCH or better** |
| 14 | Menu geometry | Submenu safe triangle (A5) | Not specified | **ADOPT**, written by us (C7) |
| 15 | Perceived speed | Local-first replica, optimistic writes, checkpoint deltas (A6) | Optimistic mutation + Undo (spec); Hub is server-side (OD-3), agents are remote (OD-14) | **ADOPT** a local replica of Hub objects (C8); **DON'T COPY** "optimistic everywhere" for irreversible agent actions |
| 16 | Loading | Few loading states, because nothing waits (A6) | Skeletons mirror final geometry (spec) | **MATCH** on rule, **ADOPT** the budget (C8) |
| 17 | Command menu | `⌘K`, contextual (A7) | `⌘K` scoped to selection, stable ranking, inline hints (spec; palette built) | **MATCH** |
| 18 | Single-key verbs, `G`-sequences | `C`, `X`, `G I`, `O P`… (A7) | Desk: `j/k`, `a`, `d`, `?`, `Enter` (AGENTIC-UX §5); one keydown map with terminal carve-outs (built, `useKeyboardMap.ts`) | **ADOPT** sequences, **only when focus is not in a terminal**. Linear has no PTY to protect; we do |
| 19 | Peek | `Space` tap/hold, `↑`/`↓` follow (A7) | None | **ADOPT**, and **BEAT**: peek the evidence span, not a record (C9) |
| 20 | Onboarding | Theme first, then "press `⌘K`"; demo data (A8, third-party) | Desk zero state is the design target (AGENTIC-UX §5) | **ADOPT** keys-first; **DON'T COPY** demo data: a fake agent has no transcript span (AGENTIC-UX Principle 7) |
| 21 | Empty states | One concept, one action (A8, third-party) | "Nothing needs you. 7 agents running. Next decision expected ~14:22." (spec) | **BEAT**: ours carries a projection |
| 22 | Agent states | 6 session states, set by the platform (A9) | Closed set of 7, mechanically derived (AGENTIC-UX Principle 8) | **MATCH**; ours adds DRIFTING and splits blocked-on-you from blocked-on-machine |
| 23 | Agent activity | Conversation thread: thought, action, elicitation, response, error (A9) | STATE / MEANING / NEXT; four delta classes incl. ASSUMED; structural compression of tool calls (AGENTIC-UX §2, §8) | **BEAT**; **DON'T COPY** the chat thread (AGENTIC-UX anti-pattern 14) |
| 24 | Agent plan | Checklist with step statuses (A9) | Convergence: settled / open / contested / drifting (AGENTIC-UX Principle 10) | **DON'T COPY** step counts |
| 25 | Instant feedback | First activity within 10 s or "unresponsive" (A9) | Honest liveness: "running 6m, last output 3s ago" (AGENTIC-UX §8) | **ADOPT** the 10 s number as a budget (C8) |
| 26 | Disclosure | "Always disclose that it's an agent" (A9) | Author class on every Orchestrator line (`05-design` §9.4) | **MATCH** |
| 27 | Accountability | Human stays primary assignee (A9) | Accountability and billing stay with each agent's owner (OD-16) | **MATCH** |
| 28 | Many agents at once | Tabs with working dots; filter by Delegate (A9) | One heartbeat clock, 10 Hz throttle, ranked queue, horizon bar (DESIGN-SYSTEM, AGENTIC-UX) | **BEAT** |
| 29 | Agents across owners | Not present on any page read (A9) | Orchestrators negotiate, escalate to inform or decide; team strip (OD-16, `05-design` §6, §9) | **BEAT**: nobody else shows this (C10) |
| 30 | Quality process | Stress sets, flags, A/B toggle, zero-bugs in 7 days (A3, A10) | Adversarial review, tests; no UI stress set | **ADOPT** (C12) |

**Tally:** MATCH 12 · ADOPT 13 · BEAT 6 · DON'T COPY 6 (some rows carry two verdicts). We already hold Linear's discipline on colour, elevation, motion and the palette. The gaps are **generated themes, peek, sequences, speed budgets, an icon rule and a stress-test habit**. The wins are everything about **many live agents and many owners**.

---

# Part C — The uplift: named tokens and moves

Every value below is a **proposal**, unbuilt and unmeasured. Visual acceptance is the owner's eyes (`CLAUDE.md`, working agreements). Token names extend `desktop/src/styles/tokens.css`; none renames an existing token.

## C1. `theme.generate`: three seeds, every surface derived

Adopt the mechanism of A1 in OKLCH, which we already use.

| Token | Default (graphite) | Meaning |
|---|---|---|
| `--seed-base` | `oklch(16% 0.0045 67)` | the ground; every surface, ink and hairline derives from it |
| `--seed-accent` | `oklch(70.3% 0.1092 202.7)` | the teal; `-hi`, `-lo`, `-ink`, `-tint`, `-ring` derive from it |
| `--seed-contrast` | `1.0` (range 0.8–1.5) | multiplies every lightness step; `1.5` is the high-contrast theme |

- **Where it runs: build time, not runtime.** A `tokens.gen.ts` script emits `tokens.css`, one block per theme. That keeps the existing contrast assertions in `tokens.test.ts` meaningful, because they test emitted numbers. CSS relative-colour syntax would do it at runtime but needs a WKWebView floor we have not pinned: **UNVERIFIED**, so not relied on.
- **Four themes, no free-form picker:** `graphite` (dark, default), `paper` (light, re-derived, not inverted), `graphite-hc`, `paper-hc`. A custom-theme picker is Linear's feature, not our users' need; the generator makes it a one-screen add later.
- **Status colours are not seeded.** `--run`, `--need`, `--fail`, `--drift` stay fixed across themes, with lightness re-derived per ground so contrast holds. Status must mean the same thing on every screen.
- **The generator's first test:** every emitted ink clears the floors already written in `tokens.css` (ink-3 ≥ 3:1 on resting surfaces) in all four themes.

## C2. The signature: `thread`, because warmth no longer is

Linear went warm in March 2026 (A1). Our visible signature must be something Linear structurally cannot have. It is the provenance thread, which is the product spine (`CLAUDE.md`).

- **`thread.line`:** a 1px vertical hairline in `--thread` joining an artifact to what it came from: brief section → design frame → worker task → diff hunk → release note. It already exists as the rail's parent-child connector; promote it to a product-wide motif.
- **`thread.node`:** a 5px ring (`--e-hair` geometry, `--thread` colour) where the line meets an artifact.
- **`thread.lit`:** hovering or focusing a brief section rings every artifact that carries it, in `--accent-ring`. That is interactive state, so the accent rule holds.
- **`thread.trace`:** on drill-in, the line grows from source to target with `transform: scaleY(0→1)`, `transform-origin: top`, `--dur-slow`, `--ease`. Motion that explains causality: *this came from that* (DESIGN-SYSTEM "motion must explain causality").
- **Rule a reviewer can check:** any artifact that shows a `thread.node` must deep-link to its source span. A thread without a source is deleted (AGENTIC-UX Principle 7).

## C3. Type: two voices, one scale

**`voice.split` is the move.** Human-language lines (MEANING, NEXT, handover prose, brief text) are sans. Every machine fact (span id, path, duration, exit code, token count, cost, lane ref, route badge) is mono at `--t-micro`. Evidence and claims look different on sight, which is AGENTIC-UX §2 point 7 made visual. Linear has no reason to do this; we do.

| Token | Size / line | Face, weight | Use |
|---|---|---|---|
| `--t-micro` | 11 / 14 | mono 450 | machine facts, `kbd` hints, span refs |
| `--t-meta` | 12 / 16 | sans 450 | secondary labels, timestamps in words |
| `--t-ui` | 13 / 20 | sans 450 | rows, controls, the rail |
| `--t-body` | 14 / 22 | sans 400 | MEANING lines, briefs, Orchestrator prose |
| `--t-head` | 16 / 22 | sans 560, tracking −0.01em | view titles, section heads |
| `--t-display` | 24 / 28 | mono 500 | the zero-state line only, once (DESIGN-SYSTEM "Mono … once at display scale") |

- **Faces:** keep the DESIGN-SYSTEM pairing, Geist Sans + one licensed mono (Berkeley Mono or Commit Mono), not Inter. Ship them as bundled files, not webfonts, so the offline-first rule in `tokens.css` holds and there is no load shift. **Owner decision needed:** the mono licence (~$100 one-time for Berkeley; Commit Mono is OFL and free).
- **Heading optical size:** Geist's variable weight axis gives headings their own weight (560) instead of a second family. This is the job Inter Display does for Linear, done in one face.
- `tabular-nums` stays on `<body>` (built).

## C4. Density: a spacing scale and two row heights

| Token | Value | Use |
|---|---|---|
| `--s-1` … `--s-8` | 4 · 8 · 12 · 16 · 24 · 32 px | the only spacing values; nothing else is legal in component CSS |
| `--row-h` | 32px | a list row, a rail row, a RUNNING row |
| `--row-h-compact` | 28px | the same rows in `density: compact` |
| `--card-3line` | 76px (3 × 20 + 2 × 8) | the Desk agent card at rest: STATE / MEANING / NEXT, never taller (AGENTIC-UX anti-pattern 3) |

- **`density` is a preference with two values,** `comfortable` and `compact`. It changes row height and vertical padding only, never type size, so text never reflows when the user switches.
- **Density is set by the content, not the chrome.** A 30-agent RUNNING section at `--row-h` is 960px, one screen on a 1080p laptop. That is the budget the section is designed to.

## C5. Chrome recedes: `chrome.dim` and one `ViewHeader`

Adopt Linear's 2026 move (A3), in our tokens.

- **`chrome.dim`:** rail and sub-rail labels at `--ink-2`, their icons at `--ink-3`; the current item at `--ink` with a 2px `--accent` tick on its leading edge (current state, so accent is legal). The work area keeps `--ink` for its text. The brightest thing on screen is the work.
- **`ViewHeader`:** one component, one layout, on the Desk, a project, each lens, the Orchestrator panel and the team strip's click-in: title (`--t-head`) · scope chips · route badge (`Global`/`AU`, `05-design` §9.5) · view controls on the right. No second header pattern anywhere. Deletes AGENTIC-UX §9 item 8 ("repeated breadcrumb chrome") by construction.

## C6. Icons: `--icon` and a rule for when not to use one

| Token | Value |
|---|---|
| `--icon` | 16px box, 1.5px stroke |
| `--icon-sm` | 14px box, 1.25px stroke (inside rows and chips) |
| `--icon-ink` | `--ink-3` at rest, `--ink-2` on hover, `--ink` when current |

- **When:** an icon appears only where the label alone is ambiguous or the space cannot hold the label. Never on a row that already carries a status shape. Never coloured, unless the icon *is* the status shape. This is Linear's "fewer, smaller, uncoloured" (A4) as a checkable rule.
- **Which set:** a single permissive-licence family for generic glyphs, restroked to the tokens above, plus about 12 glyphs we draw ourselves for the domain: status shapes, `thread.node`, lease, span, hunk, worker, Orchestrator, route. **Not Lucide at its default stroke**: it is shadcn's default and so part of the generated look. The family choice needs a licence check: **UNVERIFIED**, owner or legal to confirm.

## C7. Motion: four additions to what we already have

The existing motion tokens (`--ease`, `--dur-fast` 120 / `--dur` 180 / `--dur-slow` 260, `--beat` 2400) already say more than Linear publishes (A5). Add:

| Token / move | Value | Job |
|---|---|---|
| `--dur-flash` | 600ms | state-change flash and decay, `opacity`/`background` (DESIGN-SYSTEM specifies ~600ms; `tokens.css` does not carry it yet) |
| `--dur-peek` | 120ms | peek in: `opacity` 0→1 and `translateY(4px)`→0; out at `--dur-fast` |
| `--undo-window` | 6000ms | the optimistic-undo countdown, shown as `tabular-nums` seconds on the Undo chip |
| `menu.aim` | geometry, no timer | a submenu safe triangle, our own implementation. The technique is general prior art in menu design, long predating Linear's 2020 post (S7); we write it from the idea, not from their code |

- **`arrivals.hold`:** a new item never pushes rows under the reader. It collects behind one line at the top of its section, *"2 new · R to show"*, and enters on explicit refresh (AGENTIC-UX Principle 11, anti-pattern 7). This is the Linear-grade "nothing jumps" feel, made compatible with a live fleet.
- **`beat.phase`:** every live dot's `opacity` breathes between 0.45 and 1 on `--beat`, phase-aligned to one shared epoch by a negative `animation-delay` (built: `desktop/src/hooks/heartbeat.ts`, `desktop/src/components/common/common.css`). Keep it. It is the thing Linear's per-tab "working" indicator cannot do at 30 agents.

## C8. Perceived speed: `hub.replica` and named budgets

Linear is fast because nothing waits (A6). Our constraint is different: agent state lives in the cloud (OD-14, OD-20) and a PTY stream cannot be local-first. But most of what a user *clicks* is not the PTY.

- **`hub.replica`:** the desktop keeps a local replica of the Hub objects this user may see: queue rows, read cursors, briefs, approvals, leases, team-strip lines, Orchestrator thread state. Tauri stores it on disk; the UI reads only from the replica. Writes apply to the replica first and sync after.
- **`sync.checkpoint`:** on reconnect, the desktop sends the ID of the last change it applied and receives only later changes. This is the general delta-log pattern Linear describes publicly (S8); we design our own protocol for the Hub. It is what makes the 4-hour RETURN (AGENTIC-UX §3) paint instantly instead of loading.
- **Two optimism classes, never one:**
  - **`optimistic.undo`** for reversible acts (approve a reversible action, defer, accept landed work, mark seen, pause): the row resolves in one frame, an Undo chip counts down `--undo-window`, a server refusal returns the row with a `--fail` flash and the reason.
  - **`confirm.inplace`** for irreversible acts (run a migration, push, deploy, anything the authority grid marks irreversible): no optimistic resolve. The row shows the consequence line and a two-step confirm inside the row, never a modal. Linear has no irreversible verbs in the tracker; we do, so we must not copy "optimistic everywhere".

| Budget | Target (proposed, unmeasured) | Measured at |
|---|---|---|
| `budget.paint` | ≤ 1 frame (16ms) from keypress to changed row, for any replica write | Desk, queue |
| `budget.palette` | ≤ 50ms from `⌘K` to first painted row | palette |
| `budget.peek` | ≤ 100ms to first paint from the replica; transcript lines stream in after | peek |
| `budget.ack` | ≤ 10s from launch to the agent's first transcript span, else the row states "no output yet · 10s" as a fact | RUNNING row. The 10s is Linear's published agent rule (S18) |
| `budget.return` | ≤ 2s from sign-in after 8h away to a painted handover | Desk |

A budget with no test is a wish. Each gets a Playwright timing test against the local app (the repo's recipe), recorded, not asserted, until the owner accepts the numbers.

## C9. Keyboard and peek: `keys.list` and `peek.span`

**`keys.list`: single keys and sequences live only in list focus.** They never fire while focus is inside a terminal, which extends the carve-out already built in `desktop/src/hooks/useKeyboardMap.ts` (`isCockpitChord`). Linear can bind `C` globally because it has no PTY; a cockpit that ate `C` would stop the user typing to their agent.

| Key | In list focus (Desk, queue, team strip) |
|---|---|
| `J` / `K`, `↑` / `↓` | move |
| `Space` tap / hold | peek open / peek while held (Linear's pattern, A7) |
| `A` · `D` · `?` · `Enter` | approve · defer · shortcut sheet · DIVE (already in AGENTIC-UX §5) |
| `R` | reverse an ASSUMED decision (rewind to here) |
| `U` | undo the last optimistic act while its window is open |
| `M` | ask **my** Orchestrator about this row (`05-design` §9.2's one action) |
| `G` then `D` / `P` / `M` | go to Desk / projects / my Orchestrator |
| `⌘K` | palette, from anywhere, including inside a terminal (built) |

`kbd` hints: `--t-micro` mono, `--inset` ground, `--e-hair`, `--r-chip`, shown inline in the palette, in menus and in the first tooltip (DESIGN-SYSTEM "hotkey hints inline so it teaches itself").

**`peek.*`: preview the evidence, not the record.** Linear's peek shows an issue's fields (A7). Ours shows the thing a decision rests on:

- **`peek.span`** on any queue or RUNNING row: the ≥ 12 transcript lines around the span that produced the row (AGENTIC-UX anti-pattern 6), watermark marked, mono.
- **`peek.hunk`** on a LANDED row: the diff hunks, each with its back-link span (AGENTIC-UX §8 point 5).
- **`peek.person`** on a team-strip row: that person's tasks, reasons and files (`05-design` §9.2). Never their transcript or terminal (OD-15).
- `↑`/`↓` move the peek down the list, as in Linear. Peek never moves the list, never marks anything seen, and closes on `Esc`.

## C10. Agent activity: where we beat them

Linear shows one agent per issue, as a conversation (A9). We show a fleet, and a fleet of fleets across owners. The moves:

- **`fleet.beat`:** the RUNNING section is one list on one heartbeat. Thirty agents breathe in phase; one that changes state flashes and decays over `--dur-flash`; one that stops is still. The eye finds change without a single spinner (DESIGN-SYSTEM "one global heartbeat clock").
- **`row.three`:** each agent row is STATE (mono `--t-micro`: `EXECUTING · 41m · last 3s`), MEANING (sans `--t-body`), NEXT (sans `--t-meta`, `--ink-2`: "autonomous for ~20m"). Linear's `thought` and `action` activities map to what feeds MEANING; they never appear raw on the Desk.
- **`step.fold`:** tool calls fold into one mono line with their cost: `pytest -q · 43s · exit 1 · 812 lines` (AGENTIC-UX §8 point 3). This is our answer to Linear's ephemeral activity: nothing disappears, it compresses.
- **`who.label`:** every line written by an agent names its owner and class in mono: `alice · orchestrator (agent)`, `you · worker 2 (agent)`. No avatar is needed; the label is the disclosure (Linear guideline 1, A9; `05-design` §9.4). Humans are the only rows with a name alone.
- **`negotiation.row`:** a cross-owner Orchestrator thread appears as one typed row, not a chat: `you · orch ⇄ alice · orch · auth/session.go · round 2 of 3 · decide by 14:40` (`05-design` §9.4). It sits in the Desk's existing sections, never its own (`05-design` §9.3): `decide` in NEEDS YOU, `inform` in SINCE YOU LAST LOOKED, agreed-within-authority in DECIDED WITHOUT YOU with Undo.
- **`collision.horizon`:** "Alice's worker and yours converge on `auth/session.go`, ~20 min" in the horizon bar (`05-design` §9.3). Linear shows no prediction of agent conflict anywhere read (A9).
- **`lease.ring`:** a path someone else also holds shows a 1px `--line-strong` ring on its chip in the team strip, never a colour flood (`05-design` §9.1). The ring turns `--need` only when a `decide` escalation exists for that path, so the status colour keeps its one meaning.

**The magic, in one sentence the owner can repeat:** Linear lets a person hand an issue to an agent; Panacea lets everyone's agents see each other, settle what they can, and bring each person only the decisions that are theirs, on one calm Desk.

## C11. First run and empty states: `first.keys` and `first.landing`

- **`first.keys`:** first run is sign-in (Entra, OD-6), then theme (`graphite` / `paper` / high contrast), then *"Press ⌘K"*, before any project exists. Adopted from A8's reported flow.
- **`first.landing`, instead of demo data:** a real 60-second rehearsal task runs in a real cloud sandbox (OD-20) against a tiny starter repo, so the user's first Desk item is a real LANDED row with a real span, a real hunk and a real thread back to a one-line brief. Linear's demo workspace works because issues are just records; a fake agent would be an element with no transcript span, which AGENTIC-UX Principle 7 forbids.
- **`empty.fact`:** every empty state is one fact, one projection, one verb. Desk: *"Nothing needs you. 7 agents running. Next decision expected ~14:22."* (AGENTIC-UX §5). Team strip: *"Only you on this project."* (`05-design` §9.1). Orchestrator threads: *"No open threads. Your Orchestrator will raise anything that needs you."* No illustration: an illustration is decoration, and decoration fails DESIGN-SYSTEM's "motion and colour have a job" standard.

## C12. The quality habit: `stress.sets` and `flag.compare`

Adopt Linear's process (A3, A10), which is cheap and which we lack.

- **`stress.sets`**, run on every UI change before owner review:
  1. **Environment:** Windows WebView2 at 100%, 125% and 150% scaling (KPMG laptops); macOS WKWebView; offline; signed out.
  2. **Appearance:** all four themes from C1.
  3. **Load:** a Desk with 0, 1, 7 and 30 agents; a team strip with 1, 5 and 8 people; a 30-row RUNNING section in `compact`.
  4. **State:** the eleven states DESIGN-SYSTEM's floor lists (loading, empty, sparse, dense, partial, stale, offline, unauthorized, error, blocked, success).
- **`flag.compare`:** every visual change lands behind a flag, with a dev-toolbar toggle to flip old and new on the same screen. The owner's eyes are the acceptance test (`CLAUDE.md`); a toggle makes that a two-second comparison.
- **A UI-defect clock** like Linear's zero-bugs rule (fix within 7 days, S25) is an **owner decision**, not ours to set.

## C13. The not-a-clone test

A reviewer puts any screen next to Linear and checks both lists. One hit on the second list fails the review.

**Five things on every screen that Linear does not have:** the teal accent on current state only; the provenance `thread`; mono machine facts beside sans meaning (`voice.split`); the shared heartbeat; 2% grain on the dark ground.

**Five Linear signatures we never ship:** Inter or Inter Display; an indigo or violet accent; a status icon that fills as work progresses; agent activity rendered as a chat thread; any marketing-site glow, gradient, glass or bento composition inside the product.

---

# Part D — Owner questions, order of work, and what is unverified

## D1. Owner questions, each with a recommendation

1. **Mono licence.** Berkeley Mono (~$100 one-time) or Commit Mono (OFL, free)? *Recommend Berkeley Mono*, per DESIGN-SYSTEM's own argument that the paid face is the taste marker.
2. **Light theme now or later?** *Recommend now,* because the generator (C1) makes it nearly free and enterprise users present on projectors and in bright offices.
3. **A UI-defect clock** (Linear: fix within 7 days)? *Recommend yes, 7 days for any defect against DESIGN-SYSTEM's floor,* so polish is a policy, not a phase.
4. **Signature change.** Accept that warmth no longer differentiates and make the `thread` the visible signature (C2)? *Recommend yes.*

## D2. Order of work, smallest visible win first

| Step | Work | Size (one agent, with tests) | Visible result |
|---|---|---|---|
| 1 | Add `--dur-flash`, `--dur-peek`, `--undo-window`, `--s-*`, `--row-h*`, `--t-*`, `--icon*` to `tokens.css` | ~2 hours | nothing yet; unblocks the rest |
| 2 | `chrome.dim` + one `ViewHeader` | ~1 day | the work area is brighter than the chrome on every view |
| 3 | `voice.split` across the existing Develop lens and session pane | ~1 day | machine facts read as mono evidence |
| 4 | `keys.list` + `peek.span` on the Desk queue | ~3 days | triage without opening anything |
| 5 | `tokens.gen.ts` + `paper` and high-contrast themes | ~3 days | a light theme that is re-derived, not inverted |
| 6 | `hub.replica` + `sync.checkpoint` | depends on the Hub (Phase H); ~2 weeks once the Hub API exists | the 4-hour return paints instantly |
| 7 | `stress.sets` + `flag.compare` in the dev toolbar | ~2 days | the owner flips old and new in two seconds |

Estimates are for execution by one agent against the current `desktop/` tree and are **UNVERIFIED** until step 1 is sized against the real code. Step 6 is blocked on the Hub; steps 1–5 and 7 are not.

## D3. What is unverified, and the test that settles each

| Claim | Status | Test |
|---|---|---|
| Linear UI type sizes, weights, tracking | UNVERIFIED (third-party only) | not needed: we do not copy them |
| Linear in-app accent is a desaturated indigo | UNVERIFIED | not needed: indigo is banned regardless |
| Linear status icons fill with progress | UNVERIFIED (screenshots, not docs) | not needed for our rule; we ban filling glyphs on AGENTIC-UX grounds |
| Linear's in-memory object graph and optimistic transactions | UNVERIFIED (talk video not readable here, S9) | watch S9 off the corporate network |
| Linear onboarding flow and demo data | UNVERIFIED (third-party teardowns, S14, S15) | sign up for a free Linear workspace from a personal machine |
| "Offline is a failsafe" | UNVERIFIED (search snippet of Linear docs, page not fetched) | fetch `linear.app/docs` offline page |
| Our speed budgets are achievable in WebView2 | UNVERIFIED, unmeasured | Playwright timing runs (C8) |
| Relative-colour CSS on our WKWebView floor | UNVERIFIED | avoided by generating at build time (C1) |
| Icon-family licence | UNVERIFIED | licence check before C6 step lands |

## D4. Fetch failures

- `https://linear.app/docs/keyboard-shortcuts`: HTTP 404 on 2026-09-25. Shortcut facts come from search results over Linear's own docs (S11, S12, S16) and are marked by source.
- `https://linear.app/now/scaling-the-linear-sync-engine` (S9): the page is a video; only its introduction was readable.
- `https://www.devtools.fm/episode/61`: only the title was readable; not relied on.
- No page failed for a corporate-network block during this run. Third-party reverse-engineering repositories of Linear's sync engine were **deliberately not opened**: they reconstruct proprietary code, which the clean-room rule excludes.

---

# Sources

All read on **2026-09-25**. "Fetched" means the page itself was read; "search" means only a search-engine summary of the page was read, so the claim is weaker. Plane was not consulted at all.

| # | Source | Published | How read |
|---|---|---|---|
| S1 | Linear, "How we redesigned the Linear UI (part II)" — https://linear.app/now/how-we-redesigned-the-linear-ui | 2024-03-28 | fetched |
| S2 | Linear, "A calmer interface for a product in motion" — https://linear.app/now/behind-the-latest-design-refresh | 2026-03-12 | fetched |
| S3 | Linear changelog, "UI refresh" — https://linear.app/changelog/2026-03-12-ui-refresh | 2026-03-12 | fetched |
| S4 | Linear Method, principles — https://linear.app/method/introduction | undated | fetched |
| S5 | Karri Saarinen, "Design for the AI age" — https://linear.app/now/design-for-the-ai-age | 2025-04-07 | fetched |
| S6 | Linear Method, index — https://linear.app/method | undated | fetched; not relied on beyond S4 |
| S7 | Andreas Eldh, "Invisible details: building contextual menus" — https://linear.app/now/invisible-details | 2020-09-17 | fetched |
| S8 | Peter Travers, "Rebuilding Linear's delta sync read path" — https://linear.app/now/rebuilding-delta-sync-read-path | 2026-08-18 | fetched |
| S9 | Tuomas Artman, "Scaling the Linear Sync Engine" — https://linear.app/now/scaling-the-linear-sync-engine | 2023-06-29 | fetched; video content not readable |
| S10 | Linear docs on offline support — via search over https://linear.app/docs/get-the-app and related | undated | search |
| S11 | Linear docs, selection and navigation shortcuts — https://linear.app/docs/select-issues ; https://linear.app/docs/editor ; https://linear.app/now/descript-internal-guide-for-using-linear | undated | search |
| S12 | Linear changelog, "Keyboard shortcuts help" — https://linear.app/changelog/2021-03-25-keyboard-shortcuts-help | 2021-03-25 | search |
| S13 | Linear docs, "Peek preview" — https://linear.app/docs/peek | undated | fetched |
| S14 | Candu, "Linear onboarding teardown" — https://www.candu.ai/blog/linear-onboarding-teardown | undated | search (third party) |
| S15 | Growth Dives, "The onboarding Linear built without any A/B testing" — https://www.growthdives.com/p/the-onboarding-linear-built-without ; Supademo teardown — https://supademo.com/user-flow-examples/linear | undated | search (third party) |
| S16 | Linear docs, `J`/`K` navigation — https://linear.app/docs/select-issues | undated | search |
| S17 | Linear, "Agent Interaction Guidelines (AIG)" — https://linear.app/developers/aig | 2025-07-30 (changelog) | fetched |
| S18 | Linear developers, "Developing the agent interaction" — https://linear.app/developers/agent-interaction | undated | fetched |
| S19 | Leela Senthil Nathan, "Our approach to building the Agent Interaction SDK" — https://linear.app/now/our-approach-to-building-the-agent-interaction-sdk | 2025-08-01 | fetched |
| S20 | Linear docs, "AI Agents" — https://linear.app/docs/agents-in-linear | undated | fetched |
| S21 | Linear docs, "Coding sessions" — https://linear.app/docs/coding-sessions | undated | fetched |
| S22 | Linear docs, "Linear Agent" — https://linear.app/docs/linear-agent | undated | fetched |
| S23 | Linear, agents page — https://linear.app/agents | undated | fetched |
| S24 | Linear, build page — https://linear.app/build | undated | fetched |
| S25 | Karri Saarinen, "Why is quality so rare?" — https://linear.app/now/why-is-quality-so-rare | 2025-05-27 | fetched |
| S26 | LogRocket, "Linear design: the SaaS design trend…" — https://blog.logrocket.com/ux-design/linear-design/ | undated | search (third party) |
| S27 | Arlene Xu, "The rise of Linear style design" — https://medium.com/design-bootcamp/the-rise-of-linear-style-design-origins-trends-and-techniques-4fd96aab7646 | undated | search (third party) |
| S28 | Linear, brand page — https://linear.app/brand | undated | fetched (colour names and values only) |

**Internal sources:** `docs/DESIGN-SYSTEM.md`; `docs/AGENTIC-UX.md`; `CLAUDE.md`; `docs/superpowers/planning/2026-09-23-owner-decisions.md` (OD-14 to OD-20); `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` §9; `desktop/src/styles/tokens.css`; `desktop/src/hooks/heartbeat.ts`; `desktop/src/components/common/common.css`; `desktop/src/hooks/useKeyboardMap.ts`.

