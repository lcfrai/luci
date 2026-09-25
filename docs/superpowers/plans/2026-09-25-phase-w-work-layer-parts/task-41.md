### Task 41: `arrivals.hold`, `moves.hold` and departures; `N` applies

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work lens,
`CriterionGroup`, `IssueRow`, `work.css`). Uses SW.8 (task 20's `expectNoPrediction` and
`workScenario()`), SW.11 (`--dur-flash`, task 26), SW.12 (`StateGlyph`, task 28) and SW.16's
signatures (`useWork(selector)`), through task 20's fixtures until task 55 lands. No Hub dependency.

This is Architecture rule 4, *rows never move under the reader* (05 §4.1, work rule 3; 02 C7
`arrivals.hold`, `02:267`). It closes no parity row alone; L-101's list (task 40), the board
(task 43), Plan (task 45) and the Desk (task 62) all read the model this task builds.

**What the reader sees** (05 §4.1 D C2, §4.4 C7; the prototype's words, `prototype/work-layer.html:620-638`):

| Change arriving from the replica | What happens on screen | Until |
|---|---|---|
| a new issue joins a group (an **arrival**) | it is not drawn; one `HeldLine` at the top of its group reads *One new issue is held, so no row moves under you.* with *Apply `N`* | `N`, or the view unmounts |
| an issue's group or state changes (a **move**) | the row **stays at its index**, takes its new `StateGlyph` at once, flashes once over `--dur-flash`, and shows one mono cause line, e.g. `→ done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02`; its group header adds `· 1 moving` | `N`, unmount, or the reader's attention is outside its group |
| an issue leaves the view (a **departure**) | the row becomes a one-line ghost: `EP-128 left this view · cancelled`, with *`N` applies* | `N`, or unmount |
| more than three changes in one group in one update | the **header** flashes once and **no row** flashes (D M9) | — |

**Decisions made here, so nobody re-derives them:**

1. **The model is pure and lives in `desktop/src/work/hold.ts`.** It knows nothing of React, the
   replica or the DOM. Task 40's `WorkLens` owns one `HoldState` in component state (`useState`),
   so it dies with the view: remounting starts from `initialHold(next)`, which is everything
   applied (the scope's *held state that outlives the view*). It is **not** in `workSlice`: a hold
   is a fact about this mount, not a view model other surfaces read.
2. **Attention, not just focus.** The scope says a row relocates when *focus is outside its group*.
   A reader reading with the pointer has no keyboard focus, and a row that jumps under the pointer
   breaks rule 4 as surely as one that jumps under the caret. So `focus.groupKey` is the group
   holding keyboard focus **or else** the group under the pointer; the lens tracks both on each
   `CriterionGroup` (`onFocus`/`onBlur` bubble; `onPointerEnter`/`onPointerLeave`). `null` means
   the reader is in neither, and held moves relocate at once. Arrivals and ghosts are held
   regardless of attention: an arrival is invisible until asked for, and a ghost is the only
   record that something left.
3. **The cause line is formatted here, from SW.4's words.** `causeLine(move, timeZone)` maps task
   03's six `MoveCause` values (`task-03.md:211-216`) to fixed phrases and never guesses:
   `hand` → *set by <name>*, `lane_bound` → *a lane started*, `lane_queued` → *worktree queued*,
   `land_trailer` → *landed at <commit7>*, `evidence_observed` → *own evidence at <commit7>*,
   `evidence_recorded` → *evidence recorded at <commit7>*. The parenthesis `(test run, exit 0)` is
   printed only when the caller passes `detail`; W1's feed data carries `to` and `cause`
   (`task-20.md:330-331`), and the evidence detail's source is task 23's evidence record
   (**UNVERIFIED** until task 37 fixes the desktop shape). No detail, no parenthesis: nothing is
   invented.
4. **`N` is bound on the list element's own `onKeyDown`** in `WorkLens`, never on `document`. It
   ignores an event whose target is an `input`, `textarea`, `select` or `[contenteditable]`, and any
   event with `ctrlKey`, `metaKey` or `altKey`. It tests `event.code === "KeyN"` without
   `shiftKey`, the agent-key rule of Global constraints. Task 53 moves it into `KEYMAP` unchanged.
5. **One flash per update, by sequence number.** `HoldState.flash` carries the ids to flash and a
   `seq`; a row renders `data-flash={seq}` only while it is in `flash.rows`. CSS restarts the
   animation because the element's attribute value changes. The animation is on a pseudo-element's
   `opacity` only, over `var(--dur-flash)`.
6. **`data-changed="unseen"`** sits on every row whose move is held (it has a cause line). Task 34's
   reduced-motion layer draws it as the static 2px tick (05 §4.4 C7); this task sets the attribute
   and styles nothing for it. Task 34 is not upstream of task 41 in `dependsOn`, so under reduced
   motion today the row shows its cause line and no tick (named, not a gap: the tick is task 34's).
7. **Group counts are plain.** A header reads `3 issues` or `3 issues · 1 moving`. The count is the
   live rows shown in that group, ghosts excluded. No `of`, no total, no fraction.

**Files** (the skeleton's file map lists `HeldLine.tsx` against tasks 40-41; it omits `hold.ts` and
`hold.test.ts`, a gap named in the return):
- `desktop/src/work/hold.ts` — the pure model (new)
- `desktop/src/work/hold.test.ts` — six model tests (new)
- `desktop/src/components/work/HeldLine.tsx` — the per-group held line and the ghost row (new)
- `desktop/src/components/work/WorkLens.tsx` — owns `HoldState`, binds `N`, tracks attention (modified, task 40's)
- `desktop/src/components/work/CriterionGroup.tsx` — renders `HeldLine`, ghosts, `· N moving`, the header flash (modified, task 40's)
- `desktop/src/components/work/IssueRow.tsx` — `data-flash`, `data-changed`, the cause line (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-held`, `.wk-ghost`, `.wk-cause`, the flash keyframes (modified, task 40's)
- `desktop/src/components/work/hold.test.tsx` — four surface tests (new)

