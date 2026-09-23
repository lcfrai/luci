# Prototype fact sheet — the one sample world behind phases 4, 5 and 7

**Date:** 2026-09-23. **Status:** sample data for owner review, not product data and not a spec.
**Why it exists:** `completeness-critique-2.md` §2.1 found `brief:b-42` was three different briefs
in the three prototypes. This sheet is the one list of facts all three now draw from. Check a
prototype against this sheet, not against the other two prototypes.

**Invariant.** Every brief key a prototype shows names the text in §2 below. One key, one text,
in every file. The only exceptions are the forks in §8, and each says on screen that it is one.

**Authority.** Where this sheet and `provenance-contract-draft.md` disagree, the contract wins. One
rule below is **not in the contract** and is marked **proposed**.

---

## 1. Project and branches

| Fact | Value |
|---|---|
| Project (display name) | **Engagement Portal** — rail initials `EP` |
| Project (Panacea name) | `engagement-portal` — used only in the Deploy lens, where Panacea's names appear |
| Branch this story follows | **`evidence-upload`** |
| Other branches, in rail order | `main`, then `evidence-upload`, `fix-session-timeout`, `feat/request-reminders-email`, `feat/request-reminders-sms` |
| Environment for `evidence-upload` | `evidence-upload`, host `engagement-portal-evidence-upload.panacea.kpmg` |
| The 20-character collision (Deploy only) | `feat/request-reminders-email` and `feat/request-reminders-sms` both shorten to `feat-request-reminde` |
| The person | Dylan Mackie — rail footer in Design and Deploy, and the approver in §4 |

## 2. The brief — `b-42`

**Brief id** `b-42`. **Project** Engagement Portal. **Branch** `evidence-upload`.
**Headline** (the purpose answer, the same in every revision):
*"Let client users upload evidence files against an open request."*

Every answer, skip and edit saves a revision, as the phase 4 plan does (contract §0.2).

| Rev | What changed | Complete? | Pinnable, approvable, materialisable? |
|---|---|---|---|
| 1 | Purpose recorded | no | no |
| 2 | Audience skipped — keeps the project brief's answer | no | no |
| 3 | Constraints recorded: *"Existing uploads and their download links"* | no | no |
| 4 | Definition of done recorded — criteria A below | no | no |
| **5** | Out of scope recorded: *"Bulk upload; the reviewer side of evidence"*. **The brief is born** | **yes** | **yes** |
| **6** | Definition of done edited — criteria B below. **The head in the thread** | **yes** | **yes** |

**Criteria A** (revisions 4 and 5):
1. Upload works for PDF, XLSX and images
2. A virus scan runs before any file is visible
3. Tests pass

**Criteria B** (revision 6):
1. Upload works for PDF, XLSX and images
2. A virus scan runs before any file is visible
3. An interrupted upload resumes
4. Tests pass

**Rule used for the fourth column — proposed, not in the contract.** Only a complete revision (all
five questions answered or skipped) may be pinned, approved against or materialised. Discover's
own model implies it (a branch brief is *"not inherited yet"* until complete); the contract never
says it (critique §2.2, decision 3). All three prototypes obey it: every pin, approval and
`contextRef` in them is rev 5 or rev 6.

## 3. The project brief — `b-7`

Project-scoped (`branchId ""`). `b-42` skips audience, so it inherits this brief's audience answer.

| Rev | What changed |
|---|---|
| 1 | Purpose: *"Client-facing portal for audit engagements: requests, evidence and status."* |
| 2 | Audience: *"KPMG staff broadly"* |
| 3 | Constraints: *"Existing client data; the published API"* |
| 4 | Definition of done: *"Tests pass"*, *"No change to the published API without a version bump"* |
| 5 | Out of scope: *"Billing and invoicing"* — complete |
| **6** | Audience edited: *"Client users, and the KPMG staff who serve them"* — **the head in the thread** |

## 4. The canvas — `cv-7`

| Fact | Value |
|---|---|
| Bound to | (Engagement Portal, `evidence-upload`) |
| Brief mirror guideline | written at `brief:b-42@5`, marker `panacea:discover brief=b-42 rev=5` |
| Frames | *Open request*, *Scanning*, *Evidence list* |
| Approved | **design rev 11**, by Dylan Mackie, against **`brief:b-42@5`**, review digest `sha256:9c41…e07a` |
| Design key | **`design:cv-7@11`** — the only design key any file shows |
| After `b-42` moved to rev 6 | the approval stands, marked `superseded` (5-D3 option A). Nobody re-reviewed |

## 5. Sessions on `evidence-upload`

| Session | Lens | `contextRef` | How it got it |
|---|---|---|---|
| `upload-api` | Develop | `brief:b-42@5` | Started at rev 5. Its first start failed to write `CLAUDE.md` (Discover state 10); the retry started it |
| `scan-hook` | Develop | `brief:b-42@5` | Started at rev 5. Its managed block was later edited by hand (Discover state 11) |
| `upload-ui` | Develop | **`brief:b-42@6`** | **Started from the approved design** (Design state 4) after rev 6 was saved. It inherits the head, like any session on the branch; a separate `session → design:cv-7@11` ledger link records the design (contract §3.2) |
| `design-1`, `design-2` | Design | `brief:b-42@5` (not drawn) | Normal sessions on the branch, pointed at `cv-7` |

**Specimen names** in Discover state 15 and state 13 — `spike-thumbnails` (none), `old-upload`
(missing), `docs-pass` (resolving), `lint-fix` (unavailable) — exist only to show a chip state.
They are not part of the thread.

## 6. Releases — environment `evidence-upload`, newest first

| Deployment | Version | Commit | Shipped | Origin | Pin | Design link | Merge link | Sessions that produced it (7-D3) |
|---|---|---|---|---|---|---|---|---|
| `run` (Building state only) | — | `a90d3f4` | 23 Sep 15:31 | cockpit | `b-42@6` | — | — | — |
| `d-6f2` | v1.6.0 | `4e7a91c` | 23 Sep 14:02 | cockpit | **`b-42@6`** current | `design:cv-7@11`, reviewed against rev 5 | rev 5 | `upload-api` 3, `upload-ui` 2, 1 outside any session |
| `d-5e1` | v1.5.3 | `0b3d5e8` | 23 Sep 11:40 | outside the cockpit | none | — | — | — |
| `d-4d0` | v1.5.2 | `c81f02a` | 22 Sep 16:25 | cockpit | `b-42@5` superseded | `design:cv-7@11`, reviewed against rev 5 | rev 5 | `upload-api` 4, `scan-hook` 2 |
| `d-3c9` | v1.5.1 | `77aa310` | 22 Sep 10:13 | cockpit, **failed** at `build-image` | `b-42@5` superseded | — | — | — |
| `d-2b8` | version unknown | — | 19 Sep 09:02 | cockpit | **`brief:b-40@5`** missing | — | — | — |

**v1.6.0 shows disagreement on purpose.** It was merged and designed against rev 5 and shipped at
rev 6, so its manifest shows the pin (rev 6) beside the design and merge links (rev 5), unresolved —
contract §3.4's *"two facts, not a choice"*.

**Rollback and rebuild** (Deploy's rollback states) re-run v1.5.2 and inherit its pin, `b-42@5`.

## 7. The one key that does not resolve — `brief:b-40@5`

There is no brief `b-40` anywhere in this world. It is the `missing` specimen in all three files:
Discover state 13 and the `old-upload` chip in state 15, Design's chip specimen, and Deploy's
`d-2b8`. Each file frames it as the contract does — data loss in our own store.

**Why not a `b-42` key?** Every `b-42` revision from 1 to 6 exists, so a missing `b-42` key would
have to be past the head, `@7` or later. Deploy lists releases by date, and no release dated before
v1.6.0's pin to rev 6 can hold a later revision without breaking the order revisions are saved in.

## 8. Forks and alternative moments — each labelled on screen

| File · state | What differs | Why it is not a contradiction |
|---|---|---|
| Discover 3, *Asking* (interactive), and every *Edit an answer* button | Revisions come from what the reviewer types; an edit from state 4 saves a rev 6 of the reviewer's choosing | An interactive demo of the flow; its text is the reviewer's, not the sample's |
| Discover 5, *Follow-up* | Forks after rev 3: its rev 4 is the added follow-up | Canonical rev 4 is never drawn by any file; the review bar names the fork |
| Discover 7, *Drafted from the code* | A repository with code and no project brief | A different starting point, so it has its own id, **`b-43`**. No `b-43` key appears elsewhere |
| Discover 9, *Project brief changed* (4-D3) | Continues the story past §2: `b-7` rev 7 re-inherited as **`b-42@7`** | After every other file's "now"; no other file shows rev 7 |
| Design 5, *Changed after approval* | Design rev 12, a frame edit after the approval | Not a fork: rev 12 is never approved, so every link still names `design:cv-7@11` |
| Design 4, the *Re-review* action | Also produces a design rev 12 — the mirror rewrite, not state 5's frame edit | Two alternative futures of rev 11, both inside the canvas. No design key other than `@11` leaves Design |
| Deploy *Branch has no brief* / *Never deployed* | The moment before Discover / before the first Ship | Earlier moments, shown for their empty states |
| Deploy *Environment taken by another branch* | Branch `feat/request-reminders-email`, brief **`b-51`** (rev 5, complete) | A different branch with its own brief |

## 9. Which state shows which moment

| Moment | Discover | Design | Deploy |
|---|---|---|---|
| `b-42` head at rev 1–4 | 3, 6, 14 | — | — |
| `b-42` head at rev 5 | 4, 10, 11, 13 | 1, 2, 3, 5, 6 | — |
| `b-42` head at rev 6 — "now" | 8, 12, 15 | 4, 7 | every other state |

## 10. One vocabulary for status and provenance (critique §2.4)

Every file draws these with CSS shapes, not glyphs, so the same state is the same shape everywhere.
Class names differ per file; the shapes and colours do not.

| Meaning | Shape | Colour | Discover | Design | Deploy |
|---|---|---|---|---|---|
| running | disc, breathing on the one clock; an outer ring under reduced motion | `--run` | `.sd.run.live` | `.st.run.live` | `.st.run.live` |
| idle | disc, still | `--ink-3` | `.sd.idle` | `.st.idle` | `.st.idle` |
| succeeded, serving, pass | disc, still | `--run` | — | — | `.st.ok` |
| **needs you**, warning | **triangle** | `--need` | `.sd.need` | `.st.need` | `.st.warn` |
| **superseded** | **half-filled disc** (left half filled) | `--drift` | `.sd.drift` | `.st.drift` | `.st.drift` |
| **missing**, failed | **square** | `--fail` | `.sd.fail` | `.st.fail` | `.st.bad` |
| nothing measured, not run | hollow ring | `--ink-4` | — | — | `.st.none` |

The other provenance states carry no status shape (contract §4.1):

| State | How it is drawn |
|---|---|
| current | neutral ink, the headline and `rev n` |
| none | an outlined, unfilled element with its words — *"no brief"*, *"not bound to a brief"*, *"no brief is recorded for this release"* |
| resolving | a fixed-width static bar; no spinner, no width change |
| unavailable | a **`!`** mark and the call site's sentence, neutral ink |

**Diamonds are lens glyphs only** (◇ ◈ ◆, below). No status uses one. Discover's `◌` marks a
drafted answer and nothing else.

**Not settled here:** phase 2's `StatusDot` draws *starting* as a hollow ring, and Deploy draws
*nothing measured* as one. The table's permanent home is contract §4.1 or `tokens.css`'s status
block (critique §2.4), neither of which this pass may edit.

## 11. Lenses — as phase 2 ships them

`desktop/src/components/subrail/SubRail.tsx:9-16` and `types.ts:173`. All three files, in this
order, with these glyphs:

`◇ Discover` · `◈ Design` · `◆ Develop` · `▤ Files` · `⑂ Merge` · `▲ Deploy`

## 12. Radius

Surfaces use only `--r-chip` 6, `--r-ctl` 8, `--r-card` 10 and the window step 14
(`DESIGN-SYSTEM.md:29`). Status shapes, avatars, radio dots and the toggle thumb are shapes, not
surfaces: they use `50%` or `1px`, as phase 2's `common.css` does for its dots. No other radius
value appears in any of the three files.

## 13. Decision labels on screen

The three briefs each number their decisions D1–D3, and the numbers mean different things. On
screen every decision carries its phase: `4-D1`, `5-D3`, `7-D2`, as `completeness-critique-2.md`
§7 does.
