# Task 28 — The Design lens: its states, the approval bar, the side panel, presence, comments

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 21, 22, 26, 27 · **External:** phase 4 task 17 (`ContextChip`) · **Hours:** 3 · **UI:** yes — `docs/DESIGN-SYSTEM.md` read first (G12); **the owner's eyes are the only acceptance**, batched to the end (`OD-12`)

## Why

The cockpit's chrome around the canvas: where the thread is **visible** — the brief chip before
approval, the approved key after it, *superseded* when the brief moves, *changed after approval*
when the design does — and where a person approves, re-reviews, reverts an agent's work, answers
comments and starts Develop from the design. `prototype/phase-5-design.html` drew this chrome around
an embedded Northstar viewport; its states stand, and the viewport is now ours (spec §7.8).

## Files

- `desktop/src/design/DesignLens.tsx`, `DesignTop.tsx`, `ApprovalBar.tsx`, `DesignSide.tsx`,
  `Presence.tsx`, `Activity.tsx`, `Comments.tsx`, `DesignRenderers.tsx`, `DesignContextChip.tsx`,
  `design.css` + tests (new)
- `desktop/src/components/AppShell.tsx` — the `design` arm (`:36-43`) renders `<DesignLens/>`; one
  `<DesignRenderers/>` is mounted beside `<Toasts/>` so hidden renderers live whatever the lens (P5-Q).
  Both the lens's responder and every hidden one get `onProgress: designApprovalProgress` (task 24,
  task 27; review I5), so the toast counts whichever renderer the server chose

## The states — the prototype's seven, mapped

| # | Prototype state (`phase-5-design.html`) | Our lens | What changed and why |
|---|---|---|---|
| 1 | No canvas: *Create canvas* binds, writes the brief mirror, enables design sessions | **Same**, without the mirror line. The list reads: binds a canvas to `<branch>`; design sessions read the branch's brief through the canvas; lets you start design sessions | no mirror (spec §7.6) |
| 2 | Drawing: *Start design session*, *Approve design rev N*; side panel: design sessions, brief in the canvas, approving records | **Same**, plus the editor (tasks 25–26). Side panel: **Design sessions** (presence rows), **Brief** (the chip and *"agents read it with get_brief; nothing is copied into the canvas"*), **Review requests** (agents' `request_review` notes), **Approving records** | the prototype's *"Needs you: approve its access to this canvas"* row (MCP OAuth, brief R7) **is deleted**: the relay channel is the credential |
| 3 | Approved: the bar, the handoff bundle card, *A Develop session started here carries* | **Same**. The bundle card lists screens (HTML + PNG), JSX (inline and Tailwind), tokens (CSS + DTCG) and **the layer tree** (`tree.json`) | the prototype's *"Component tree: not produced"* is now produced (`PROGRAM.md` lists it for the bundle) |
| 4 | Superseded: *Approved against brief rev 5 … moved to rev 6*, *Open rev 5*, *Open rev 6*, *Re-review against rev 6* | **Same**. *Re-review* is route 23 then an approval of the new revision | the mechanism is a `review_mark` op, not a mirror rewrite (brief D3) |
| 5 | Changed after approval: *… <frame> was edited by design-1* | **Same**, and the actor is **known**: `ApprovalState.changedBy` names every author since (task 21) | the prototype marked the actor unverified; the op log makes it a fact |
| 6 | Canvas server not reachable | **Split in two.** The canvas lives in the control plane, so *control plane down* is the cockpit's own state (`control-plane-down`, `problems.ts`). **Hub unavailable or signed out** is new: the canvas keeps working; *Approve*, *Re-review* and *Start Develop from this design* are disabled with the sentence from the problem | the failure that matters moved from the canvas to the Hub |
| 7 | The chip specimen | **Not a product state**: the six (seven with `erased`) chip states are phase 4's `ContextChip`, reused | — |

The prototype's *"Tell design-1 …"* box (a line typed into a session's terminal) is **not built**:
comments are the canvas's channel to an agent, and they are attributed and resolvable.

**Every other state designed** (`DESIGN-SYSTEM.md` floor): loading (a skeleton of the three
columns, exact geometry), no artboards yet (*"Start a design session, or press F and drag on the
canvas"*), not shared (phase 4's OQ-1 sentence: *"this repository has no shared remote, so its design
cannot be approved"*), no brief (*"this branch has no complete brief yet; approval needs one"* with
*Open Discover*), signed out (*Sign in to approve*), capturing (after the Undo window, the same
toast reads *"Capturing 3 of 12 · Billing"* from the slice's `approvalProgress`, `tabular-nums`,
no spinner: the count is the progress, review I5; its second line reads *"Keep this window open
until capture ends; you can leave the Design lens"*, P5-I as extended by re-review RR-I5-1),
renderer failure during approval (the server's sentence, which names the artboard, its index and
the cause — no renderer, the canvas closed, a timeout or a render error, task 21 step 4), and
success (the approval toast with **Undo**, P5-I).

## Contracts

```tsx
// desktop/src/design/DesignLens.tsx — reads selection.projectId/branchId; owns no focus and no scroll (§6.8)
export function DesignLens(): JSX.Element;

// desktop/src/design/ApprovalBar.tsx — from ApprovalState (task 21); null when state is "none"
export function ApprovalBar(props: { canvasId: string }): JSX.Element | null;
// "approved": ✓ glyph (ink-2), "Approved against brief rev {r} by {name} · design rev {n}", the design key and digest in mono
// "superseded": half-filled disc in var(--drift) + the two Open buttons + Re-review
// "changed": triangle in var(--need) + "Design changed after approval. …" + Approve design rev {head}
// Colour plus shape, never colour alone (DESIGN-SYSTEM); amber means a human must act, nothing failed.

// desktop/src/design/Presence.tsx — one row per authority.Presence
// A status dot (disc, var(--run)) whose opacity follows the ONE heartbeat (useHeartbeat), the
// handle, the artboards it holds, and its last label with a relative time in tabular-nums. A row
// action "Release claim" (route 9) for a stuck agent. No spinner (G10).

// desktop/src/design/Activity.tsx — the feed (route 10), newest first, virtualised past 50 rows;
// each session's rows carry "Revert {handle}'s changes" (route 8) with the RevertResult message as a toast.

// desktop/src/design/Comments.tsx — threads (open / resolved), reply, resolve, and "Comment" on the
// selected node (route 13). Pins on the canvas come from the editor's onOpenThread (task 25).

// desktop/src/design/DesignRenderers.tsx — mounts <HiddenRenderer/> for selectHiddenRendererCanvases (task 27)

// desktop/src/design/DesignContextChip.tsx — phase 4's <ContextChip/> for one key, reading the
// provenance slice by key (the loader resolves it through selectDesignBriefKeys, S4-P5.1)
export function DesignContextChip(props: { keyText: string }): JSX.Element;
```

**The header's chip** is the branch's head brief key before any approval and the approval's
`briefKey` after (the prototype's notes 2 and 3: *"chip = head Ref"*, *"chip = pinned key"*).

**Start a design session** opens the cockpit's own create flow (`hooks/useCreateSession.ts`) for a
worktree on this branch with the opening prompt *"Design on the canvas for this branch. Start with
get_canvas_info and get_brief."* — a normal session (the prototype's note: *"a design session is a
normal session on this branch, pointed at this canvas"*).

**Start a Develop session from this design** runs `designStartDevelop` (task 27; P5-N) with the
opening prompt *"Implement the approved design in .panacea/design/{canvasId}@{rev}/ — read
manifest.json first, then build it in this repository's own conventions."*; if the link fails, the
carry card shows *Not linked* with *Link*.

**Layout.** Header row; the approval bar when present; then three columns: left 240px (pages,
layers, Theme — task 26's `LeftPanel`), the viewport (flex 1, `var(--canvas)`, no dot grid), right
300px with four tabs — **Inspect** (`RightPanel`), **Review** (sessions, brief, review requests,
records, bundle, carry card), **Comments**, **Code** (`CodePanel`). Elevation: panels sit on
`var(--surface)` with `var(--e-card)` where they overlap the canvas; radius by size; `tabular-nums`
on revisions, counts and times.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `DesignLens.test.tsx › state1` | no canvas → the three-item list without a mirror line, and *Create canvas* calls route 2 | the withdrawn mirror promised in copy |
| `DesignLens.test.tsx › state2` | a canvas with artboards and no approval → *Approve design rev N* primary; no access-approval row exists anywhere | the deleted R7 state resurrected |
| `ApprovalBar.test.tsx` (one per state) | approved / superseded / changed each render their glyph **and** text; changed names every `changedBy` author | colour-only states; an actor fabricated |
| `DesignLens.test.tsx › hubDown` | a 503 `upstream_unavailable` `upstream: hub` → canvas editable, three actions disabled with the sentence | the canvas locked because the Hub is down |
| `DesignLens.test.tsx › notShared` | `design_not_shared` → phase 4's OQ-1 sentence | a raw code on screen |
| `approve.test.tsx` (fake timers) | *Approve* shows a toast with Undo; Undo within 5 s sends nothing | a confirm dialog, or an approval that cannot be undone |
| `approve.test.tsx › capturing` | with `approvalProgress` `{index: 3, total: 12, name: "Billing"}` the toast reads *"Capturing 3 of 12 · Billing"* and *"Keep this window open until capture ends; you can leave the Design lens"*; a `409 renderer_unavailable` whose details name artboard *"Billing"* (3 of 12) shows the server's sentence, for each `details.cause` (`no_renderer`, `renderer_gone`, `timeout`, `render_error`) unchanged | a silent twelve-artboard wait, or a failure that does not say which artboard or why (review I5, re-review RR-I5-1) |
| `Presence.test.tsx` | rows use `beatDelayStyle`/`useHeartbeat`; no `setInterval`; *Release claim* calls route 9 | a per-row timer |
| `Activity.test.tsx` | *Revert {handle}'s changes* calls route 8 with that session id | revert by the wrong author |
| `AppShell.test.tsx` (amended) | the design arm renders `DesignLens`; the phase 2 guard `grep -cE '^[[:space:]]+return <DevelopLens' …` is still `1` | the Develop arm disturbed |
| `design.css` check | no hex colours, no `-gradient(`, no `transition: all`, no `backdrop-filter`; radii only `var(--r-*)` | DESIGN-SYSTEM regressions |

## Steps

1. Read `docs/DESIGN-SYSTEM.md` and the prototype (states 1–6 and their notes, `:486-524`).
2. Tests, then components. 3. Acceptance; commit by pathspec. **Do not merge** (`OD-1`).

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/desktop && npx vitest run && npx tsc --noEmit && grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx
```
Expected: every test passes; typecheck clean; `1`.

**Owner's eyes (batched, `OD-12`; unverified until then):** states 1–6 against the prototype; the
approval bar's three states read at a glance; presence reads calm with five agents working.

## Not claimed

- Visual acceptance is the owner's, not these tests' (`CLAUDE.md`).
- The cockpit-wide ranked queue for review requests is a seam (S5-Q.1).
