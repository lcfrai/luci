# Prototype reconcile report — three prototypes, one thread

**Date:** 2026-09-23. **Work order:** `completeness-critique-2.md` §2.1–§2.5 and §4.4.
**Authority:** `provenance-contract-draft.md`. **Not edited:** the contract, anything under
`desktop/`, `sandboxd/`, `control-plane/`, `docs/superpowers/contracts/` or `docs/superpowers/plans/`.
**No git command that writes was run.**

**The one list of facts** is `prototype-fact-sheet.md`, in this directory. Each prototype now
conforms to it; check a prototype against the sheet, not against the other two.

---

## 1. Critique items

| Item | Status | What resolved it |
|---|---|---|
| **§2.1** — `b-42` three briefs, `cv-7` two canvases | **Resolved** | One project (Engagement Portal), one branch (`evidence-upload`), one headline, one revision history (Discover's, unchanged), one set of criteria, one canvas (`cv-7`, approved at design rev 11), one set of sessions and one release history. Design and Deploy adopted Discover's facts |
| **§2.2** — Design and Deploy pin `b-42@3`, which has no definition of done | **Resolved in the prototypes; the rule is a contract decision** | Every approval, pin and `contextRef` is now rev 5 (born) or rev 6 (definition of done edited). The rule that makes that right — *only a complete revision may be pinned, approved against or materialised* — is written in the fact sheet as **proposed**. It is not in the contract (§3 below) |
| **§2.3** — a Develop session started from Design inherits the approval's revision, not the head | **Resolved without changing the contract** | Design now starts the session on the **head**, with a separate `session → design:cv-7@11` ledger link. That is contract §3.2's own wording, *"like any other session"*, and Discover's *"every session started on this branch from now on inherits the head"*. The carry card, the toast and the notes all say *"inherits rev 6, not rev 5"* in the superseded state. The contract should still say it in one sentence (§3 below) |
| **§2.4** — one state, three markers; one shape, two meanings | **Resolved across the three files; home still needed** | One table (fact sheet §10), drawn with CSS shapes in all three: triangle needs-you, half-filled disc superseded, square missing/failed, disc running/idle, `!` unavailable. **No status uses a diamond** — diamonds are lens glyphs only (◇ ◈ ◆). The table's permanent home is contract §4.1 or `tokens.css`, which this pass may not edit |
| **§2.5** — lens set drifts from phase 2 | **Resolved** | All three show phase 2's six lenses in `SubRail.tsx`'s order and glyphs: `◇ Discover · ◈ Design · ◆ Develop · ▤ Files · ⑂ Merge · ▲ Deploy`. Deploy gained Files and its *"Four lenses"* comment is gone; Design moved Files and fixed its glyphs |
| **§4.4** — briefs overstate radius compliance | **Resolved** | Every surface radius in all three files is now a `--r-*` token. The only other values are `50%`/`1px`/`0` on status shapes, avatars, radio dots and the toggle thumb — the same exemption phase 2's `common.css` uses. The Discover and Deploy brief sentences are corrected and dated; Design's brief is updated because its 4px step is gone |

## 2. What changed, per file

### `prototype/phase-4-discover.html` — the source; its facts did not move

- **Status shapes (§2.4):** chips, notes and the changed-since marker now use CSS shapes — the
  half-filled disc replaces `◐`, the square replaces `✕`, the triangle replaces `△`. Status dots
  grew from 6px to 8px so the half-disc reads; idle is `--ink-3`, as `tokens.css` requires.
- **Unavailable** chip lost its amber ring, so amber means needs-you only.
- **Radius (§4.4):** 2px, 3px ×2 and 5px ×2 literals replaced by tokens (the 2px one was a 2px-wide
  bar; its radius was removed).
- **Missing specimen:** now `brief:b-40@5` held by `old-upload` (states 13 and 15). Before, it named
  `upload-api` and `scan-hook`, which the fact sheet gives real keys.
- **State 7 (drafted from the code)** is its own brief, `b-43`. It was a second `b-42` with a
  different purpose — §2.1 in miniature inside one file.
- **State 9 (project brief changed, 4-D3)** now continues from rev 6 and saves **rev 7**. Before,
  it saved a second, different rev 6. Its diff, toast and session list follow.
- **State 5 (follow-up)** is labelled a fork on screen: a follow-up mid-asking is a revision, so its
  rev 4 is not the sample brief's. No other file shows rev 4.
- **Decision labels** carry their phase (`4-D1`, `4-D2`, `4-D3`). Rail lists the fact-sheet branches.

### `prototype/phase-5-design.html`

- **Sample data (§2.1):** project, branch, headline, sub-rail purpose and rail sessions now follow
  the fact sheet. The frames no longer show reviewer or classification fields — the brief puts
  *"the reviewer side of evidence"* out of scope. `walkthrough.docx` became a PNG, because the
  brief accepts PDF, XLSX and images.
- **Revisions (§2.2):** mirror and approval at `b-42@5`; the brief moves to rev 6 in state 4; the
  specimen uses rev 5/6 and `brief:b-40@5` for missing.
- **Develop start (§2.3):** carry card, toast and notes give `contextRef` as the head. The session
  is named `upload-ui` only in the superseded state, which is the moment the fact sheet gives it.
- **Shapes (§2.4):** superseded is now the half-filled disc (it was the diamond Deploy used for
  needs-you); idle is a disc, as phase 2's `StatusDot` has it (it was a ring); unavailable is `!`.
  The `◇` glyph is gone from the chips, because Discover and Deploy draw no marker for current.
- **Lenses (§2.5):** phase 2's order and glyphs.
- **Radius (§4.4):** the 4px `--r-xs` step removed; its five uses are `--r-chip`.
- The rail's non-design session in needs-you (*test harness*) became `scan-hook`, running, as
  Discover draws it. `design-2`'s needs-you (canvas access) stays, so the state is still drawn.

### `prototype/phase-7-deploy.html`

- **Sample data (§2.1):** project `engagement-portal`, branch and environment `evidence-upload`,
  host, groups, registry path and URL-prefix placeholder. The collision state uses two
  `feat/request-reminders-*` branches, which still shorten to one 20-character name; its brief
  `b-51` is now at rev 5, because rev 2 could not be complete.
- **Revisions (§2.2):** head is rev 6. v1.6.0 is pinned to rev 6 (current). v1.5.2 and v1.5.1 are
  pinned to rev 5 (superseded). The criteria rendered for each pin are that revision's own. The
  missing row is `brief:b-40@5` — it had been `brief:b-42@1`, which exists.
- **The thread:** design link `design:cv-7@11` approved by Dylan Mackie against rev 5 (it had
  been `cv-7@4` by *J. Tran*). v1.6.0 shows design and merge at rev 5 beside the pin at rev 6 —
  contract §3.4's *"two facts, not a choice"*, and exactly what Design state 4 leaves behind.
  Rollback and rebuild copy compute the inherited pin instead of hard-coding *"rev 3"*.
- **Sessions that produced it** name `upload-api`, `scan-hook` and `upload-ui`, with the status
  Discover draws.
- **Shapes (§2.4):** needs-you is a triangle (it was the diamond Design used for superseded); the
  `◆` risk marker on access postures is the same triangle; superseded pins carry the half-filled
  disc and read *"rev 5 · brief now at rev 6"* (it was `↻ now rev 5`); missing pins carry the
  square; *"brief not read"* carries `!`. Idle is `--ink-3`.
- **Lenses (§2.5):** six, with glyphs.
- **Radius (§4.4):** 3px, 4px ×2, 5px ×2, 8px and 9px literals replaced by tokens.
- **Decision labels** carry their phase (`7-D2`, `7-D3`).

### All three

- Under `prefers-reduced-motion`, a running dot gains an outer ring, as phase 2's `common.css`
  does, so running stays apart from idle and from a succeeded build when nothing breathes. This is
  the one change taken from §3.4, because the shape table needs it to hold.
- The review strip in Discover and Deploy names the fact sheet.

### Briefs

- `scope-and-risk-phase-4-discover.md` — radius sentence corrected and dated; token-name claim
  qualified; shape sentence updated; the `brief:b-42@3` example became `@5`.
- `scope-and-risk-phase-5-design.md` — sample revisions in option A and state 3 now 5/6; state 3
  says a Develop session inherits the head; shape sentence and radius sentence updated.
- `scope-and-risk-phase-7-deploy.md` — radius sentence corrected and dated; motion sentence now
  says the toggle's transition fails phase 2's rule 4; a shape line added.

## 3. Left for a contract decision

1. **The completeness rule (§2.2, critique decision 3).** The contract never says which revisions
   may be pinned, approved against or materialised. The prototypes assume *only complete ones*.
   The contract must adopt or reject it, and then say what Ship does when the head is incomplete —
   after a *Reopen*, for instance. §3.4 pins *"the head key at the moment of Ship"* whatever it is.
   Follow-up-only revisions and 4-D3 re-inherit revisions are complete under this rule, so they are
   pinnable; the contract should say whether that is intended.
2. **Which revision Materialise writes (§2.3, decision 6 / 0c).** The prototypes follow contract
   §3.2's *"like any other session"*: always the head. §2.4 should say it in one sentence. If the
   owner chooses the other option (Materialise takes a key), Design's carry card flips back, and
   phase 4 must render a past revision into a worktree, not only read-only.
3. **Where the shape table lives (§2.4).** It is in the fact sheet because that is the only file
   this pass could write. Its home is contract §4.1 or `tokens.css`'s status block. One collision
   with shipped phase 2 remains open there: `StatusDot` draws *starting* as a hollow ring, and
   Deploy draws *not measured* with one.
4. **Revision history is linear, so the prototypes can show only one line.** A follow-up mid-asking
   is a revision (plan `AddFollowUp`), so it shifts every later revision number. Discover's
   follow-up state is therefore a labelled fork. Not a contract defect, but a reason decision 3
   should say how a follow-up revision is treated.
5. **`missing` is still overloaded** (critique §6.2 item 5). The specimen key is `brief:b-40@5`
   because no `b-42` key can be missing without breaking the order revisions are saved in (fact
   sheet §7). Its copy still says *"data loss in our own store"*, as the contract requires.

## 4. Checks run

| Check | Discover | Design | Deploy |
|---|---:|---:|---:|
| `gradient` | 0 | 1 — the comment *"no gradient dot grid (R13)"*, 0 in CSS | 0 |
| `transition: all` | 0 | 0 | 0 |
| Hex, `rgb(a)`, `hsl(a)`, `oklch(` outside `:root` | 0 | 0 | 0 |
| Network references | 0 | 0 | 0 |
| Surface radius other than a `--r-*` token | 0 | 0 | 0 |
| Transitions other than the reduced-motion reset | 0 | 0 | 1 — toggle `transform`, unchanged (critique §3.4) |

**Script and render.** `vm.Script` parsed all three scripts. A jsdom pass (from
`desktop/node_modules`, read only) rendered all 64 state views — 16 Discover (15 states plus the
interactive answering path), 14 Design (7 states plus 7 actions), 34 Deploy (17 states × 2 roles,
plus every release row opened in every state). There were no script errors and no `undefined`,
`NaN` or `[object Object]` in rendered text. The keys it found are all on the fact sheet.

**Empty and failure states are still drawn.** Discover: no brief, write failed, revision not
found, control plane unreachable, no model. Design: no canvas, no frames, no design sessions,
canvas unreachable, changed after approval, missing and unavailable chips. Deploy: not connected,
403, 401, Panacea unreachable, never deployed, no releases, build failed, site down, scan
blocking, scans incomplete, collision, rollback refused, brief store unavailable, no brief.

## 5. Not verified

- **How any of it looks.** Nothing was opened in a browser. The half-filled disc, the triangle
  and the 8px dots are reasoned from CSS, not seen. Visual acceptance is the owner's.
- **Left unchanged, and still open from critique §3.4:** Discover's `requestAnimationFrame`
  heartbeat; Deploy's `transform` transition; `--ink-4` used as text; token names that
  `tokens.css` lacks (`--drift-tint`, `--r-win`, `--on-accent` and others); Design's flat
  `1px solid` borders. Also untouched: §2.7's three unhomed claims and §3.2's two facts the model
  cannot carry.
