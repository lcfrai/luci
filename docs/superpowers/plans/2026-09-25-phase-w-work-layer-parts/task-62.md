### Task 62: the Desk frame: five fixed sections, never live while open, the row-kind registry

Stream **WD2**, about 3 hours (an estimate, not a measurement). Defines seam **SW.17**
(`2026-09-25-phase-w-work-layer.md:390`). Depends on task 37 (the `Lens` edit and `workSlice`,
`task-37.md:530-624`). Uses task 40's `WorkReader` and `workSource()` (`task-40.md:140-170`), task
20's fixtures and `expectNoPrediction` (`task-20.md:126-131`, `:299-330`), task 53's `registerVerb`,
`isListKeyTarget` and the `focus.next`, `focus.prev` and `held.apply` rows (`task-53.md:168-177`,
`:225-253`), and the built store (`store/slices/uiSlice.ts:61`, `setLens`; `dataSlice.ts:31-34`).
Tasks 63, 64, 65, 71, 127, 133, 170 and team mode's TM-54 plug rows into this frame; none of them
builds a second Desk (`2026-09-25-phase-w-work-layer.md:100-101`; 05 §5.6, `05-design.md:1313-1321`).

After this task the Desk exists as a lens: five fixed sections and one band, in AX §5's order,
each with its header, a plain count and a one-line zero, filled by whatever row kinds are
registered, ranked once when the reader arrives, and still while the reader is on it.

**What the reader sees** (AX §5 *Rules for the Desk*, `docs/AGENTIC-UX.md:142-149`; 05 §4.2,
`05-design.md:930-960`; `prototype/work-layer.html:760-787`):

- **Headers in a fixed order**: NEEDS YOU, DECIDED WITHOUT YOU, FAILED, LANDED, RUNNING, then the
  band SINCE YOU LAST LOOKED. Each header carries its count in mono `tabular-nums`, the one place
  on the Desk a count is shown (05 §4.4 rule 3).
- **An empty section keeps its header** and one zero line: *"Nothing was decided for you."*,
  *"Nothing failed."*, *"Nothing of yours landed since you last looked."*, *"No agents of yours are
  running."* (the prototype's words, `:782-785`, and one new line for RUNNING). NEEDS YOU's zero is
  `DeskZero`, *"Nothing needs you."*, which task 65 fills with what W1 can back. FAILED is never
  hidden behind a calm state (07 D C5).
- **Nothing moves while the reader is on it.** When a row would arrive, the Desk shows one line
  above the sections, *"A new item arrived. The Desk does not re-rank while you read it."*, with a
  *Show* button marked `N`; `N` or the button re-ranks. Leaving the Desk and coming back re-ranks.
- **Within a section**, rows run by tier (irreversible first, then *blocks someone*, then the rest,
  AX §5's *"ranked first: irreversible"*), and within a tier the longest-waiting first.
- **`j`/`k` walk every row** from the top of NEEDS YOU to the end of RUNNING and the band, across
  section edges; the cursor row has the ring (DESIGN-SYSTEM's selection shape).
- **The horizon band renders nothing** in W1: the horizon bar has no slice (Q4, 05 §5.6).

**Decisions made here, so nobody re-derives them:**

1. **SW.17 is this file's `deskModel.ts`, and it holds no JSX.** The scope allowed `.ts`
   (`tasks-61-80.md:69-70`); the skeleton's file map says `deskModel.tsx` (`:311`, gap 1). Every
   row kind imports `registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskRank` and
   `DeskSnapshot` from `desktop/src/components/desk/deskModel`. Task 127 wrote `./deskFrame` and
   task 170 `../desk/registry` (`task-127.md:400`, `task-170.md:295`): both follow this path (gap 2).
2. **A kind is registered once, as a row and a source, and a second registration throws.** The
   error names the kind (*"desk row kind issue.urgent_mine is already registered"*), so TM-54 can
   never silently replace an issue row (`team…/task-54.md:35-36`). A row without a source, or a
   source without a row, is a load-time error on the first `buildDesk`, naming the kind.
3. **The snapshot is taken once, on entry.** `Desk` calls `takeDeskSnapshot(inputs)` in its first
   render (`useState` initialiser) and builds the model from it. While mounted it subscribes to the
   cockpit store and the work source only to **compare**: a fresh build whose item keys
   (`kind + ":" + id`) include one not on screen sets `held`; nothing on screen changes. A row that
   would leave stays until `N` or the next visit (task 63's test 5 relies on this). `N` rebuilds.
4. **Leaving re-ranks because the frame unmounts.** `LensRouter` renders one arm, so switching lens
   unmounts `Desk` and drops its snapshot. On unmount the frame writes the time into
   `markDeskVisit(ms)`, an in-memory preference that tasks 64 and 133 read as *"since you last
   looked"*. Nothing is written to disk (Global constraints: no Hub data on disk).
5. **`DeskSnapshot` is fixed here, with every member a later part named.** Task 65 reads `me`,
   `sessions`, `heads`, `revisionAt`, `refOf`, `events` and `briefCriteria` and `headOf`
   (`task-65.md:354-362`, `:805-813`); task 170 reads `signals(kind)` (`task-170.md:303`); TM-54
   reads the cockpit state (`team…/task-54.md:37-40`). This task gives each a W1 value: `heads` and
   `headOf` from task 40's `WorkReader.byProject` over the store's projects; `sessions` from the
   store; `events` is `[]`, `revisionAt` and `refOf` return `null`, `briefCriteria` returns `null`
   and `signals` returns `[]` **until** a later task passes its source in `DeskInputs` (task 63 the
   feed, P4-16 the loader, task 127 and 170 their signals). A later task **adds** an input; it never
   renames a member. TM-54 reads `snap.cockpit.teamDesk`, not `snap.teamDesk` (gap 3).
6. **Rank is pure and total.** `rankItems` sorts by `tier` ascending, then `waitingSinceMs`
   descending (longest wait first), then `kind`, then `id`, so two runs over one snapshot give one
   order. A band is not ranked: `since` keeps `arrivedAt` ascending (TM-54's decision (b), task 65's
   `since` rank), and `horizon` draws nothing in W1.
7. **The count is the section's rows, drawn as a plain number.** No `of N`, no ratio, no bar: a
   count of rows on screen is an observation (`OD-27`). A folded row (task 64's *"23 steady"*) is one
   row; its own words carry its number.
8. **Keys go through task 53, never a listener of the Desk's own.** The frame's root is
   `[data-work-list]` with `data-key-scope="list"`, so task 53's `dispatchListKey` sees `J`, `K` and
   `N` there (`task-53.md:168-177`, `:249-253`). On mount the Desk registers `focus.next`,
   `focus.prev` and `held.apply` with `registerVerb` and unregisters on unmount; the registry runs
   the newest, and the Work lens is unmounted while the Desk is shown, so neither steals the other's
   keys. A row whose item carries `issueId` gets `data-issue-id` and `data-key-scope="issueRow"` on
   its wrapper, so task 53's issue verbs (`Enter`, `I`) reach it; what they do on the Desk is task 63's.
9. **The Desk is a lens, reached by `setLens("desk")`.** `Lens` gains `"desk"` beside task 37's
   `"work"` (`task-37.md:609`); `LensRouter` gains one arm. The sub-rail stays six rows, as task 37's
   decision 8 holds for `"work"`. How a person opens the Desk (`G D`, the palette, the landing
   surface) is not this task's: making it the landing surface waits on the owner's review (task 39;
   gap 4).
10. **Words only, and no motion that re-sorts.** The frame animates nothing: a held arrival is a
    line of text, not a slide. DESIGN-SYSTEM's elevation for a row is the 1px ring plus the layered
    shadow; the section header is type and a hairline, as the prototype draws it (`:236-237`).

**Files** (the skeleton's file map lists `{Desk,DeskSection,DeskZero,deskModel,deskRows}.tsx` and
`desk.css` for tasks 62-65, `:311`):

- `desktop/src/components/desk/deskModel.ts` — SW.17: the types, the registry, `rankItems`,
  `buildDesk`, `takeDeskSnapshot`, `markDeskVisit` (new; `.ts`, decision 1)
- `desktop/src/components/desk/deskModel.test.ts` — the pure tests (new)
- `desktop/src/components/desk/deskRows.tsx` — the one registration module; its head is this
  task's, and tasks 63, 64, 65 and 71 append in task order (new)
- `desktop/src/components/desk/Desk.tsx` — the frame: snapshot, hold, keys (new)
- `desktop/src/components/desk/DeskSection.tsx` — a header, its count, its rows or its zero (new)
- `desktop/src/components/desk/DeskZero.tsx` — NEEDS YOU's zero line; task 65 fills it (new)
- `desktop/src/components/desk/desk.css` — owns `.dk-*` (new)
- `desktop/src/components/desk/Desk.test.tsx` — the surface and key tests (new)
- `desktop/src/store/types.ts` ⚑ — `Lens` gains `"desk"` (modified)
- `desktop/src/components/AppShell.tsx` ⚑ — one `LensRouter` arm (modified)
