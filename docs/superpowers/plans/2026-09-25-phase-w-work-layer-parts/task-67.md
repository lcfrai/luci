### Task 67: issue chips in Develop and Merge

Stream **WD2**, about 2 hours (an estimate, not a measurement). Deltas on phase 4's and phase 3's
desktop files under rule H2: the owner's file wins, and this task **waits** for it on the sync base
and never writes a second copy. Depends on task 48 (the issue view's revision pin, beside tasks 46
and 47), P4-17 (`ContextChip`, `SessionContextChip`) and P3-19 (`LaneCard`), and through their seams
on task 18 (`Ref.Kind`, the issue fields), task 19 (`Session.issueRef`, the second key source) and
task 21 (the lane index's issue id).

After this task a session's pane header shows what the agent was told twice, side by side: the
brief chip it always had, and `EP-142@3` beside it. A lane in Merge names its issue. Every chip
opens the one issue view at the revision it names, and says so when that is not the head.

**Decisions made here, so nobody re-derives them:**

1. **One chip component, two arms.** P4-17's `ContextChip` takes a `ProvenanceRef`
   (P4 `task-17.md:49-61`). The issue arm is chosen by `refValue.kind === "issue"` (task 18's
   field, `task-18.md:104-108`), never by the key's prefix: the desktop never splits a key
   (`task-19.md:300-303`). A brief ref renders exactly as P4-17 wrote it.
2. **The alias is read, never kept.** The chip prints the replica head's `alias` when the head is
   in the replica, else the ref's `issueAlias` (resolve-time, `task-18.md:113`). An admin's
   project-key change moves the head's alias at once and the key not at all (05 §3.6), so the chip
   follows it without a re-resolve. The alias is never a cache key, a map key or a route.
3. **The revision is the key's, not the head's.** `EP-142@3` prints `refValue.revision`. When the
   replica's `headRevision` is higher the chip adds `· head is @4` in the same mono run. The
   issue view opens through task 37's `openIssue(id, revision)` (`task-37.md:560`) with the chip's
   revision, so the view shows `@3`, and task 48's pin says it is not the head.
4. **The seven states map onto P4's words.** `resolving` and `unavailable` render as P4-17 renders
   them for a brief (the key in mono, P4's words); `missing` prints the key verbatim; `erased`
   prints `EP-142@3 · erased` with no title (hub-contract §7.12) and still opens the view, where
   task 46 shows the erased state. `superseded` does not apply: P4-16's rule 3 reads
   `headsByBrief`, which holds no issue; the issue arm uses decision 3's head check instead. `none`
   and `declined` never reach the issue arm: a session with no `issueRef` renders no issue chip.
5. **Beside, never instead.** `SessionPaneHeader` mounts `SessionContextChip` for the brief (P4
   plan `:364`) and, **after it**, `SessionIssueChip` for `Session.issueRef`. A session with no
   issue ref renders nothing in that slot: no placeholder, no empty chip, no reserved width.
6. **Lanes name their issue from the lane index.** The lane's `issueId` is task 21's `Lane.IssueID`
   (`task-21.md:183`, `json:"issueId,omitempty"`). P3's `LaneObservation` does not carry it (P3
   `task-07.md:228-236`, `task-18.md:42-47`), so this task adds `IssueID` to the observation
   (Go and TS, one field each, copied from the lane index by the observer). The branch name is
   never parsed: an AU-only project's branch is `ep-142` with no title (task 21, `OD-24`), and a
   parsed branch would break the day a project key changes.
7. **A lane chip names no revision of its own.** A lane is the work, not the brief the agent read.
   The chip prints the alias; the revision comes from the lane's newest session's `issueRef` when
   the store holds one (`LaneView.sessions`, P3 `task-18.md:63`), and the chip then reads like the
   pane's. With no session record the chip reads `EP-142` and opens the head, which it says in
   its `title`.
8. **The thread node lights with its criterion.** The issue chip renders `<ThreadNode/>` (SW.14)
   for the first criterion the ref's `serves` names, bound with `useThreadLit` (as task 47 binds
   its serve rows, `task-47.md:451`), so hovering the chip lights that criterion everywhere; a ref
   that serves nothing renders no node.

**Files:**
- `desktop/src/provenance/ContextChip.tsx` ⚑ (P4-17) — the `kind: "issue"` arm, `IssueChipBody`
  (modified; **UNVERIFIED until P4 merges**)
- `desktop/src/provenance/SessionIssueChip.tsx` (+ `SessionIssueChip.test.tsx`) — the store-reading
  mount for `Session.issueRef` (new; **not in the skeleton's file map**)
- `desktop/src/provenance/provenance.css` ⚑ (P4-17) — `.pv-chip-issue`, `.pv-chip-behind` (modified)
- `desktop/src/components/develop/SessionPaneHeader.tsx` ⚑ — one mount after the brief chip (modified)
- `desktop/src/components/lens/merge/LaneCard.tsx` ⚑ (P3-19, `task-19.md:140`) — `LaneIssueChip`
  (modified; **UNVERIFIED until P3 merges**)
- `control-plane/internal/converge/observe.go` ⚑ (P3-07's `LaneObservation`, `task-07.md:228`) and
  `desktop/src/api/converge.ts` ⚑ (P3-18's mirror, `task-18.md:42`) — `issueId` (modified; the file
  names are P3's, **UNVERIFIED**; the field goes where the type is declared)
- `desktop/src/components/lens/merge/LaneCard.test.tsx` ⚑ — tests 5 and 6 (modified)
