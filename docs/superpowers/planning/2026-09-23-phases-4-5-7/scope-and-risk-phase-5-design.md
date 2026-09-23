# Scope and risk — Phase 5, the Design canvas (product journey: **Design**)

**Status: for product-owner review. Not an implementation plan.** Written 2026-09-23.
`CLAUDE.md`: *"The product owner reviews prototypes before implementation plans, and plans before
code."* This brief and `prototype/phase-5-design.html` are inputs to that gate.

**Sources, all read for this brief:** `survey-phase-5-design.md` (present, this directory),
`provenance-contract-draft.md` (present, DRAFT), `completeness-critique.md` §4.6 and §5,
`docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md` §2, and the `<style>` block of `prototype/app.html`.
Paperdesign itself was **not** re-read for this brief; every Paperdesign fact below is the survey's,
cited by its section.

---

## 0. The decisions, first

Three decisions are yours and each one changes the plan's shape. Full framing in §5.

| # | Decision | Recommendation | Why it blocks a plan |
|---|---|---|---|
| **D1** | **Where the canvas server runs** — beside the user's agents on their own machine, one per project in the tenant namespace, or one shared service | **Beside the agents, one per user machine** | It decides who is exposed to server-side execution of agent-written JavaScript, whether cross-user multiplayer exists, which database, and how three identity systems meet (§3, R4) |
| **D2** | **Northstar's second spine** — keep only the `VISION` gate and its digest, and disable the seven states outside it (contract A6 / P10) | **Keep `VISION` + digest; disable the rest; embed, port nothing** | Left on, the canvas and the cockpit answer *"where is this project?"* differently. The plan cannot draw the Design lens until it knows which states exist |
| **D3** | **What a brief edit does to an approved design** (contract open question 3) — void the approval automatically, or keep it pinned and mark it `superseded` until a human re-binds | **Keep it pinned, mark `superseded`, re-bind by one explicit action** | It decides when the brief mirror inside the canvas is rewritten, and therefore when Northstar's digest voids an approval. The prototype draws the recommended behaviour (state 4) |

**Not yours, but start the clock this week:**

- **Legal — AGPL-3.0-only, §13 Remote Network Interaction.** Under the locked we-host model §13
  fires the moment a user's browser talks to our modified canvas server; the Tauri shell is also
  classic distribution, so both triggers are in scope. The copy on disk is already modified (survey
  §0.2, §5.1). The survey's four-part question for legal is at `survey-phase-5-design.md` §5.5,
  ready to send. **Blocks hosting the canvas for users and blocks porting. Does not block this
  prototype or a plan written against an embedded, separable server.**
- **You — push the fork somewhere.** The whole Northstar re-purposing is 75 uncommitted entries in
  one working tree on one laptop, with no remote (survey §0.2). Cheap to fix, costly to lose. The
  known route is an internal repo under org `kpmg-au-emu`; an internal push is not a §13 event,
  but confirm with legal before anything public. Not plan-gating — do it regardless.

## 1. Scope

Phase 5 puts a design canvas inside the cockpit, bound to one branch's brief, where the user's
parallel agents edit real-HTML frames over MCP, a human approves the design against a named brief
revision, and the approved screens hand off to Develop with the thread intact.

### 1.1 In scope

| # | Item | What makes it phase 5's | Source |
|---|---|---|---|
| I1 | **The Design lens.** Our chrome — header, brief chip, agents panel, approval, handoff — around an embedded Northstar canvas viewport for the selected branch | Fills the `design` arm of `LensRouter`, today `LensPlaceholder phase={5}` | survey §6.1, §6.3 |
| I2 | **Canvas binding.** `(projectId, branchId) → canvasId`, one canvas per branch, created on first open. **New state in our store** — Northstar has no field for an external reference | Nothing else binds a canvas to a branch | survey §7.2 |
| I3 | **The brief chip, all six states** — `current`, `superseded`, `none`, `missing`, `resolving`, `unavailable` — drawn by our chrome, outside the iframe. Head `Ref` before approval; the design link's pinned key after | Contract §4.2 names phase 5 as the Design surface | contract §4.1, §4.2, P9, P11 |
| I4 | **The brief mirror.** The brief text written into the canvas as a read-only guideline carrying its key marker, overwritten from the Brief store, with a drift check because any canvas user can edit guidelines | Guidelines are inside Northstar's review digest, so the approval binds the exact brief text by digest | contract §3.3; survey §7.2 |
| I5 | **Design sessions.** Phase-2 sessions started against the canvas's MCP endpoint, passing our session handle as `agent_name` so canvas attributions line up with cockpit rows | Northstar sees N agents on one token as one human; the handle is the only thing that tells them apart | survey §2.1 |
| I6 | **Approval and the design link.** The human approves the `VISION` gate from our chrome; the design link `(canvasId, designRevision, reviewDigest) → brief key` is written to the provenance ledger **in the same operation** | Contract §2.2 makes phase 5 the design link's sole writer | contract §2.2, §8.3 |
| I7 | **Handoff bundle, v1** — the approved screens (the exact approved frame HTML Northstar keeps as `reviewArtifacts`, plus a rendered PNG each) and the guidelines as markdown tokens, materialised into the worktree with a marker. **Start a Develop session from the design**: its `contextRef` is the brief key; phase 5 writes a `session → design key` ledger link | `PROGRAM.md:92` names the bundle; contract §3.2 settles the carrier | survey §7.3; contract §3.2 |
| I8 | **Every failure state the lens can meet** — canvas server unreachable; agent blocked on canvas access; design changed after approval; brief superseded; key missing; control plane unavailable | `DESIGN-SYSTEM.md` floor: every state designed | prototype states 5–8 |

### 1.2 Explicitly out

| # | Out | Why | Reopened by |
|---|---|---|---|
| O1 | **Porting any Paperdesign code into `desktop/src`** | Copies AGPL code into our app; also React 18→19, zustand 4→5, Vite 5→8, Tailwind/shadcn→our tokens across 88 client files | Legal's answer (survey §5.5) |
| O2 | **Northstar's seven workflow states outside the `VISION` gate** — `DISCOVERING`, `DEFINING`, `BUILDING`, `VERIFYING`, `VERIFIED`, `DEPLOY_PENDING`, `DEPLOYED` — and the `VERIFICATION` and `DEPLOYMENT` gates. Whether `REJECTED` belongs to the `VISION` gate was not read | Discover owns Define; Merge and Deploy own the rest | D2 |
| O3 | **Northstar's resident agent and every model path it brings** — the server-wide `ANTHROPIC_API_KEY`, the Azure OpenAI option, and the ChatGPT-subscription connector that presents as the Codex CLI | Design work is done by the user's own Claude Code sessions over MCP, under the locked per-user capacity model. The connector also carries a terms question for legal | Owner; legal on terms (survey §3.4) |
| O4 | **Server-side egress features** — PostHog analytics, stock image / icon / logo / inspiration search, `view_website`, `import_webpage`, S3 assets, SMTP, GitHub App import, public share links | Each is egress or exposure a regulated customer will ask about; none is needed to design against a brief | Owner, with security (survey §3.4) |
| O5 | **A component tree in the handoff bundle** | It does not exist. A frame is an opaque HTML string; there is no component or layer model to export (survey §2.3). It would be new work — derived from HTML, or authored by an agent as a new artifact | Owner, as a later phase-5 slice |
| O6 | **More than one live canvas on screen** | Northstar's room pushes cursor and frame events uncoalesced. One open room; every other canvas is summarised from the phase-2 poller | Never — this is the heartbeat rule |
| O7 | **Cross-user multiplayer** | Depends on D1. Beside-the-agents hosting needs a route into the user's machine for a second human | D1 |
| O8 | **Reskinning Northstar's UI to `DESIGN-SYSTEM.md`** | Under embed, the viewport shows Northstar's own visual language. Reskinning means modifying the fork further, which is more §13 surface | Owner, after legal (R10) |

**One scope call inside I7 you may want to overrule:** v1 ships screens and tokens and says plainly
that there is no component tree. The alternative is to hold the handoff until O5 exists.

## 2. Seams

A seam is a place where phase 5 touches something it does not own. Each row names the owner.

### 2.1 Against phase 2 — the cockpit (live in this tree, being edited now)

| # | Seam | What phase 5 needs | Owner | Status |
|---|---|---|---|---|
| S1 | **Mount point** — the `design` arm of `LensRouter` in `AppShell.tsx` | Swap `LensPlaceholder phase={5}` for `<DesignLens />`. `AppShell.tsx` is modified in the working tree right now, so the survey's line references may be stale. The lens-seam task (`dependency-graph-and-order.md:147`) would make `DesignLens.tsx` a file only phase 5 opens | phase 2, then 0b | survey §6.1, §6.2 item 5 |
| S2 | **The keyboard map goes deaf inside the canvas.** `useKeyboardMap.ts:1-2` is *"the ONE window keydown listener in the application"*, attached at `:140` to `window`. A keydown inside a cross-origin iframe is delivered to the iframe's document and does not reach the parent window. While focus is in the canvas, `⌘K`, lens switching and every chord are dead | A designed focus-escape (for example a visible *"Esc returns to the cockpit"* affordance handled by our chrome), and a decision on whether Northstar's own shortcuts or ours win inside the viewport | phase 2 contract §6.8/§6.9 + phase 5 | **Listener read from source. Iframe delivery is standard DOM behaviour, not tested in WebView2** |
| S3 | **One heartbeat.** `hooks/heartbeat.ts` — `EPOCH`, `BEAT_MS = 2400`, `beatDelayStyle()` aligns every live indicator to one phase | Every live dot in our chrome uses it. Northstar's cursors inside the viewport are the product and are exempt, which is why only one canvas room may be open (O6) | phase 2 | read from source |
| S4 | **The poller's one-request rule.** `poller.ts:1-3`. Our chrome needs three Northstar facts for the **open** canvas — current `designRevision`, `VISION` gate state, frame list — to draw *"design changed after approval"* truthfully | Fetched through the control plane and folded into the tick, never a per-component fetch (P8). This is a **second** amendment to UI contract §4.7 — the provenance contract §6.4 already makes one. Two amendments to one rule should be written once | phase 2 contract + phase 4 + phase 5 | proposed |
| S5 | **The problem vocabulary.** `api/problems.ts` — eighteen codes, three kinds; adding a kind is by design a compile error at every call site (critique) | Two new situations: the canvas server is unreachable while the control plane is up, and an agent is blocked on canvas access. Whether each is a new **code** under an existing kind (`backend-down`, `server-said`) or a new **kind** is a contract decision | phase 2 contract | proposed |
| S6 | **Session creation.** Design sessions (I5) and *"start a Develop session from this design"* (I7) go through phase 2's create path | `contextRef` is stamped by the server path that materialised (contract A3), never by phase 5's UI. Phase 5 adds only the `session → design key` ledger link | phase 2 / phase 4 (decision 0c) | contract §2.4, §3.2 |
| S7 | **Tauri capabilities.** Our shell grants `opener:default` only; Paperdesign's grants IPC to its own URL | **The embedded canvas origin must get no Tauri IPC.** It renders agent-written HTML. Both apps run `"csp": null` | phase 9 hardening, flagged now | survey §1.2 |

### 2.2 Against the provenance contract (DRAFT — present, not approved)

| # | Seam | What the contract says | What phase 5 must decide in its plan |
|---|---|---|---|
| P-a | **Who writes the design link, and when** | Sole writer phase 5, *"when a human approves the `VISION` gate"* (§2.2) | **Recommended: the approval is made from our chrome, and the control plane performs Northstar's gate transition and writes the ledger link in one operation** — the design analogue of P7. The alternative, observing Northstar's approval and writing afterwards, is safer than a deploy observation (the approval record carries its own revision and digest) but still has a crash window. **Unverified: whether Northstar exposes the human gate transition over a REST route our control plane can call on the human's behalf.** MCP cannot — `request_workflow_transition` never passes a human gate (survey §2.2) |
| P-b | **Which `Ref` the header shows** | Head `Ref` before approval; the design link's pinned key after (§4.2) | Drawn as states 2 and 3. The switch happens on the approval response, not on a later poll |
| P-c | **The brief mirror** | Allowed as a render under four conditions: marker, overwritten from the store, never read back, drift check (§3.3) | The drift check needs a trigger. A human or agent editing the mirror guideline in Northstar bumps `designRevision` and — after approval — voids it. Draw it as a failure the chrome names, not a silent revert |
| P-d | **Brief edited after approval** | Open question 3 | D3. The mirror is the mechanism: rewrite it and the digest voids the approval; freeze it and the approval stands as `superseded` |
| P-e | **Frames never draw provenance** | P11 — frame scripts are untrusted and can already write as the viewer (survey §3.1) | Every chip, approval line and bundle marker lives in our chrome. The prototype's viewport contains frames and their names only |
| P-f | **`design:` keys and the session link** | `design:` reserved for phase 5, ledger links only; the `session → design key` link shape is *"reserved here and finalised in phase 5's plan"* (§3.2) | Shape to be written in the plan. Proposed: `{ sessionId, designKey, briefKey }`, the brief key repeated so a reader never parses the design key to find it |
| P-g | **A deleted canvas** | P13 keeps brief revisions for as long as a link names them. Nothing yet says the same of a **design** | If a canvas is deleted in Northstar, every design link to it — and every release that carries one — resolves `missing`. Either Northstar's `reviewArtifacts` are retained, or the bundle is **captured into our store at approval**, or the materialised bundle in git becomes the record. The same choice decides whether a handoff works while the canvas server is down (prototype state 6). **Unowned today — R8** |

### 2.3 Against phases 3, 4, 7 and 8

| Phase | Seam | Direction |
|---|---|---|
| **4 Discover** | Every chip in `current`, `superseded` or `missing` opens Discover **at that revision, read-only** (contract §4.1). That is a new obligation on phase 4, and the one-click half of the owner's provenance test depends on it | phase 5 consumes |
| **4 Discover** | The by-key resolver `GET /api/v1/discover/refs` and the head endpoint (contract §3.1) must exist before the Design chrome can render anything but `none` | phase 5 consumes; slice zero merges first |
| **3 Git/worktrees** | The handoff bundle is materialised into the worktree. Whether it is committed follows the same answer as contract open question 4 (*is `CLAUDE.md` committed?*) — and committing it puts approved screens into git history, which is also a retention answer for P-g | phase 3 decides |
| **7 Deploy** | A release link carries the design key when one exists (contract §2.2). The Deploy lens reads it from phase 5's canvas-binding endpoint, not from a third provenance endpoint (contract §3.1) | phase 7 consumes |
| **8 Kata** | Under D1's recommendation the warm-pool image grows by a Chromium, and Chrome runs `--no-sandbox` inside the microVM. That is defensible only once the microVM is the boundary. **Phase 5 is scheduled before phase 8**, so until then the boundary is whatever phase 1's sandbox provides — not read for this brief | phase 8 inherits |

## 3. Risks, and who decides each

### 3.1 Decide before the plan

| # | Risk | Severity | Decider | Evidence |
|---|---|---|---|---|
| R1 | **AGPL-3.0-only §13 is unanswered, and the copy on disk is already a modified version.** Hosting the canvas for users lands inside §13 under we-host; the Tauri shell is classic distribution; porting copies AGPL code into our app | **Blocker** for hosting and porting. Not for prototyping or planning an embedded, separable server | **Legal** | survey §5.1, §5.5; `package.json:5`, `LICENSE:540-548` |
| R2 | **The fork exists only in one uncommitted working tree**, 75 entries, no remote of ours. `data/` (1,114 files, most likely the PGlite store) is unbacked-up too if it holds real canvases | **High**, cheap to fix | **Product owner** — where to push | survey §0.2, §9 |
| R3 | **Two product spines.** Northstar's per-canvas `DISCOVERING … DEPLOYED` workflow duplicates Discover, Merge and Deploy. `CLAUDE.md` calls this class of defect wrong by definition | **High** | **Product owner** — D2 | survey §7.1; contract P10 |
| R4 | **Agent- and user-written JavaScript executes on the canvas server**, in headless Chrome, without Chrome's sandbox in containers, with public-internet egress for any HTTP method. Private ranges and IMDS are blocked | **High** if shared across users; **medium** beside one user's agents | **Security**, through D1 (owner + phase 8) | survey §3.2; `server/screenshot.ts:53`, `:102`; `server/publicUrl.ts:263` |
| R5 | **Parallel agents overwrite each other.** MCP clients get no per-agent frame scope (`agentScope` is enforced only for the resident agent) and frame writes carry no expected revision — last writer wins per whole frame, silently. Parallel agents are the product | **High** for this product specifically | **Phase 5 implementer**, once R1 allows Northstar server changes | survey §2.3 |

### 3.2 Later, but each needs a named owner

| # | Risk | Severity | Decider | Evidence |
|---|---|---|---|---|
| R6 | **A frame's own script can persist edits to itself as the viewing human.** For us that means a design approved under one name can be marked *"changed after approval"* with the approver's name on the change — a forged provenance event | **High for provenance integrity**; unverified by execution | Fork owner; the fix is a Northstar change, so it waits on R1 | survey §3.1; `src/components/FrameView.tsx:448-466` |
| R7 | **Headless agents cannot complete MCP OAuth.** Northstar mints MCP tokens after an interactive browser approval; our agents run headless on the user's machine. Unsolved in the fork | **Medium** — every design session hits it once per canvas server | Phase 5 implementer, with phase 9's auth design | survey §2.1, §6.2 item 3 |
| R8 | **A deleted canvas orphans its design links** — and every release that carries one (P-g) | **Medium** | Product owner (retention) + phase 5 | contract P13 by analogy |
| R9 | **A second transport in one window.** Northstar's WebSocket room runs beside the phase-2 poller; a second reconnect ladder and a second failure vocabulary | **Medium** | Orchestrator — critique §6 item 6 (does the event stream land before phase 5?) | substrate §8 item 5; survey §4.2 |
| R10 | **Two visual languages in one window.** Embedded, the viewport renders Northstar's Tailwind/shadcn UI, which fails `DESIGN-SYSTEM.md` on sight. Reskinning it is further modification of AGPL code | **Medium** — visible to every user, every day | Product owner, after R1 | survey §6.3 |
| R11 | **Egress and terms** — PostHog on every MCP call, the ChatGPT-subscription connector presenting as the Codex CLI, stock/icon/logo/webpage fetches | **Medium** for regulated tenants | Product owner to disable (O3, O4); legal on the connector's terms | survey §3.4 |
| R12 | **`$`-pattern corruption in `edit_frame_html`** and the resident agent's edit path — an edit to a line containing `$'` splices the rest of the document in, silently. Upstream's defect too | **Low** severity, high frequency on price-bearing screens; one-token fix | Fork owner | survey §2.4; `server/mcp.ts:1270` |
| R13 | **The canvas grid in the approved prototype fails the phase-2 design-system audit.** `app.html`'s `radial-gradient` dot grid is the construct task 24's audit failed (critique §5). This prototype draws no grid. Under embed the viewport background is Northstar's anyway | **Low** | Whoever owns the phase-2 audit | critique §5 |
| R14 | **The canvas server scales up, not out** — in-memory state, in-process rooms | **Medium**, decides hosting more than it blocks | Product owner + Panacea platform, through D1 | survey §4.2 |

## 4. What I could not verify

- **Nothing was run.** No Paperdesign build, no browser, no test. The prototype was **not opened in
  a browser by me** — the corporate network blocks headless screenshots, and `CLAUDE.md` makes
  visual acceptance the owner's eyes. What I did check mechanically is listed in §6.3.
- **Every Paperdesign fact is the survey's**, carried with its section number, not re-read for this
  brief. The survey itself ran nothing; its two defects (R6, R12) are read from source.
- **Whether Northstar has a REST route for the human `VISION` approval** that our control plane can
  call on the user's behalf (P-a). If it does not, the same-operation write needs a Northstar change,
  which waits on R1.
- **Whether a frame record carries its last editor.** The survey shows `actorFrom(agent_name)` on
  writes and an activity feed; it does not show a per-frame last-editor field. The prototype's
  *"changed by"* line in state 5 assumes the actor is recoverable from Northstar's workflow events
  or activity feed.
- **Whether the keyboard map is dead inside the iframe in WebView2** (S2). The window listener is
  read; the iframe behaviour is standard DOM, untested in Tauri.
- **Phase 1's sandbox isolation before Kata** (§2.3, phase 8 row). Not read.
- **Whether Northstar's generic OIDC works against Keycloak realm `panacea`** — inferred possible
  from the survey, never configured.
- **Whether a headless agent can complete MCP OAuth by any flow** (R7) — no device flow was found by
  the survey; I did not look further.
- **The provenance contract is a DRAFT.** Every seam in §2.2 moves if the owner overrules A1–A6.

## 5. Decisions the owner must make before a plan can be written

### D1 — Where the canvas server runs

A Northstar server process must run somewhere: the canvas state, the MCP endpoint and the WebSocket
room all live in it, and it cannot be replicated (R14). Phase 2 launches agent sessions; it has no
long-lived per-user service process (survey §6.2 item 1, inferred from the substrate survey).

| Option | Gains | Costs |
|---|---|---|
| **A. Beside the agents, one per user machine** — *recommended* | Server-side JavaScript (R4) reaches one user. PGlite fits with no Postgres to run. Agents reach MCP on localhost. Matches the machine chip already in the cockpit | Warm-pool image grows by a Chromium. A second human on the canvas needs a route into the user's machine (O7). Until phase 8 the boundary is phase 1's sandbox, not a microVM |
| **B. One per project, in the tenant namespace** | Real cross-user multiplayer. Aligns with Panacea tenant isolation | Needs real Postgres, its own ingress and SSO. Every project member's frames execute in one Chrome |
| **C. One shared service** | Cheapest to run | Every tenant's frames execute in one Chrome, every canvas in one in-memory `Map`. **Hard to defend for regulated tenants** — security would decide, and the survey does not recommend it |

**What changes with the answer:** the identity bridge (Northstar's own accounts against Keycloak),
the agent's MCP URL, the phase-8 image, and whether O7 is reachable at all.

### D2 — Northstar's second spine

Northstar ships a ten-state per-canvas workflow with three human gates. Its `VISION` gate is the
best provenance primitive in the programme: a human approves a SHA-256 digest over every frame's
full HTML, every guideline and every pinned reference, at one `designRevision`, and any later edit
invalidates it (survey §7.1). Its other states duplicate phases 4, 3 and 7.

| Option | Consequence |
|---|---|
| **A. Keep `VISION` and its digest; disable the other seven states and the `VERIFICATION` and `DEPLOYMENT` gates** — *recommended* | One spine. Contract P10 becomes binding. The plan builds only the approve-and-link path |
| **B. Map Northstar's ten states onto our lenses** | Two sources of truth for one fact, kept in step by code that has to exist forever |
| **C. Leave Northstar's workflow on as shipped** | The canvas and the cockpit disagree about where the project is, in front of the user |

**Coupled to it, and effectively forced until legal answers:** embed the Northstar client in the
Design lens, port nothing (O1). Porting is blocked by R1; embedding is the only choice that does
not make legal's answer harder (survey §6.3).

### D3 — What a brief edit does to an approved design

The brief mirror (I4) lives inside Northstar's review digest. So whether the mirror is rewritten
after approval **is** the decision about whether the approval survives.

| Option | Behaviour | Trade-off |
|---|---|---|
| **A. Pin and mark** — *recommended* | Before approval the mirror tracks the brief head. At approval it freezes with the design. When the brief moves on, our chrome says *"approved against brief rev 3 · brief now at rev 5"* (`superseded`, contract §4.1) and offers **Re-review against rev 5**, which rewrites the mirror and, through the digest, returns the canvas to review | A typo fix does not void an approval. The human decides when a brief change matters. Agents keep designing against rev 3 until re-bound, and the chrome says so |
| **B. Void automatically** | Every brief edit rewrites the mirror; Northstar's digest voids the approval; the design returns to review | Strictest. Every brief edit, however small, forces a re-approval |

Option A is the contract's stated default (*"does not silently re-point the approval"*, §4.2) and
is what the prototype draws in state 4. **Option B needs no new behaviour — it is what happens if
nobody decides**, because rewriting the mirror is the obvious implementation.

### Not decisions for you, recorded so nobody asks you

- **Same-operation design-link write vs observe-and-write (P-a)** — the plan author's, once the
  REST route is confirmed.
- **Code vs kind in `problems.ts` (S5)**, the **second §4.7 amendment (S4)** — phase 2 contract owner.
- **Does the event stream land before phase 5 (R9)** — orchestrator.

## 6. What the prototype shows, and why each element exists

`prototype/phase-5-design.html` — one self-contained file, opened by double-click. No external CSS,
no fonts fetched, no network calls, no build step. It draws **our** shell around the embedded
canvas; inside the viewport it draws only frames and their names, never Paperdesign's own UI.

### 6.1 The states

Switch with the bar at the top, or keys `1`–`7`. `N` toggles the notes layer, which outlines where
our chrome ends and the embedded canvas begins and lists the fact behind every element.

| # | State | Design-system floor state | What it proves |
|---|---|---|---|
| 1 | **No canvas yet** | empty | The brief chip is present before anything else exists (P9) |
| 2 | **Designing** — one agent running, one blocked on canvas access | live, partial, blocked | One heartbeat for every live dot; R7 drawn as a *needs-you* row, not hidden; the head `Ref` in the header |
| 3 | **Approved** — design link pinned, handoff bundle, start a Develop session | success | *"approved against brief rev 3"*; the bundle says plainly what it holds and what it does not |
| 4 | **Brief superseded after approval** | stale | D3 option A: the approval stands, marked `superseded`, with one re-bind action |
| 5 | **Design changed after approval** | error | Northstar's digest guard, drawn by our chrome, with the approved and current design revisions side by side |
| 6 | **Canvas server unreachable** | offline, error | Provenance survives a canvas outage because the design link is ours, not Northstar's |
| 7 | **The brief chip, six states** — `current`, `superseded`, `none`, `missing`, `resolving`, `unavailable` | loading, empty, error | Contract §4.1's vocabulary, each state visually distinct, none sharing a treatment |

### 6.2 Rules that shaped it, and the calls I made

- **Our chrome cannot draw inside the embedded canvas, and must not (P11).** So *"design changed
  after approval"* names the frame in our approval bar, not by outlining the frame. The notes layer
  outlines the embedded region in dashed amber and our chrome in dashed teal so you can see the
  boundary.
- **One heartbeat.** Every live dot gets a negative `animation-delay` measured from one `EPOCH`, the
  same mechanism as `desktop/src/hooks/heartbeat.ts` (`BEAT_MS = 2400`). No per-row timer, no spinner.
- **The brief chip has a fixed width** so `resolving` → `current` shifts nothing, and each of the six
  states has its own treatment. Status carries a shape as well as a colour: circle running, triangle
  needs-you, square failed/missing, ring idle, diamond superseded.
- **Design changed after approval is amber, not red.** A human must act; nothing failed.
- **No canvas dot grid.** `app.html`'s `radial-gradient` grid is the construct the phase-2 audit
  failed (R13). The viewport is a flat `--canvas` surface.
- **Colour:** every value is a token in `:root`, taken from `app.html`. Three tokens are new names
  for values `app.html` uses inline (`--on-accent`, `--on-need`, `--fail-fg`), shadow alphas became
  `--sh-*` tokens, and `--drift-tint` is the one derived value (the `--drift` hue at 12% alpha).
- **Radius follows `DESIGN-SYSTEM.md`'s 6 / 8 / 10 / 14**, not `app.html`'s 7 / 9, plus a 4px step
  for elements smaller than a chip. `50%` and `1px` remain only on status shapes.
- **Invented numbers:** none that do not map to a named field. Revisions, frame counts and digests
  are sample values of real fields (`Brief.Revision`, `headRevision`, `designRevision`, frame list,
  `reviewDigest`). I left out ages (*"waiting 4m"*) because I could not name the phase-2 field
  that holds a status-change time.
- **Not done:** film grain, and the licensed mono / Geist faces — system fonts, as `app.html` uses.

**A question the prototype surfaced, added to P-g:** is the handoff bundle **captured at approval**
into our store, or **read from the canvas at handoff**? State 6 draws the second — the approval
survives an outage, the bundle does not, and *Start Develop session* is disabled with its reason.
Capturing at approval makes the handoff and the P-g retention question both go away, at the cost of
a copy of the approved design in our store. Plan author's call; worth your view.

### 6.3 Mechanical checks — run against the file, not by eye

Run with Node against the file on 2026-09-23; the scripts are throwaway and not committed.

| Check | Result |
|---|---|
| `http://`, `https://`, `url(`, `@import`, `<link`, `src=`, `fetch(`, `WebSocket` | **0** |
| Colour literal (hex, `rgb(a)`, `hsl(a)`, `oklch`) outside `:root` | **0** |
| `gradient(` · `transition: all` · `backdrop-filter` | **0 · 0 · 0** |
| `transition` declarations | 1 — the `prefers-reduced-motion` reset to `none` |
| All seven states rendered, and every action fired, against a stub DOM | no runtime error |
| Open/close tag balance of every rendered region in every state | balanced |

**Not checked:** how it looks. I did not open it in a browser; visual acceptance is yours
(`CLAUDE.md`). Layout was reasoned for a window of about 1440px wide; below about 1280px the header
is tight.
