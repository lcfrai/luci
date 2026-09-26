### Task 55: the replica, part 1: in memory, full load, the cursor on `/v1/changes`; the no-persistence test

Stream **WD1**, the read half of seam **SW.16**, about 3 hours (an estimate, not a measurement).
Depends on task 37 (SW.15: `WorkClient`, `IssueHead`, `IssueRevision`, `IssueComment`, `Label`,
`Page`, `workProblem`, `task-37.md:134-173`, `:318-361`, `:464-485`). Implements task 40's
`WorkReader` and installs it with `setWorkSource` (`task-40.md:140-176`); task 40's gap 2 asks for
exactly that. Tasks 56, 57 and 58 extend `replica.ts` (`task-56.md:312-387`, `task-57.md:167-214`,
`task-58.md:159-197`); their guesses at this part's names are aligned below, and where they differ
this part says so. Reads the relayed feed `GET /v1/hub/changes?after=` (`hub-contract.md:1372`)
and phase 4 task 16's `hubCursor` (`phase-4-discover-parts/task-16.md:77-79`, **UNVERIFIED until
merge**, rule H2; the skeleton's row 55 names only task 37 as a dependency).

Parity: closes **L-228** with task 56 and **L-229**'s W1 part with task 57; **L-230** stays a SKIP
(05 row 230, `05-design.md:407`: surviving a restart means Hub data on disk, which ADR-H1 forbids).
Design: 05 §5.3 (*The replica, reconciled with 02*), §4.4 (*One correction to 02*: in memory),
§3.1; `ADR-H1:467-474`; 02 C8 `hub.replica` and `sync.checkpoint` (`02:274-275`); hub-contract §7.11
(`hub-contract.md:1089-1107`) and §9.3 (`:1377-1385`).

**What the reader gets.** Nothing on screen of its own. After this task the Work lens, Plan, the
issue view and every `useWork` caller read real Hub records instead of task 20's fixture, a
teammate's change reaches the list within one poll of the control plane (5 s,
`PANACEA_APP_HUB_POLL_INTERVAL`, `hub-contract.md:1378`), and closing the app leaves no Work data
anywhere on the machine.

**Decisions made here, so nobody re-derives them:**

1. **The cursor is taken before the load, never after.** The first call is
   `changes("latest")`, which returns no events and the head as `next` (`hub-contract.md:1101`).
   Only then does the replica list every project's issues. Then it reads the feed **after that
   cursor**, so an event committed while the lists were loading is applied, not lost. Taking the
   cursor after the load would skip it; the first test kills that order.
2. **Apply by `seq`, once.** The replica keeps `lastSeq`. An event with `seq <= lastSeq` is dropped
   (a repeated page, a retried read). `more: true` means ask again at once (`:1099-1100`), in the
   same read, not on the next poll. A caller's feed is filtered, so gaps in `seq` are normal; this
   task never reads one as missing data (task 57's decision 4).
3. **An event names what changed; the Hub says what it is now.** Feed events carry metadata only
   (SW.5, `task-13.md:115-118`). Applying one **re-fetches its subject** in one batch per page, and
   the event's `data` is used only to route (which issue a comment belongs to), never as the
   record. An `issue.revised` event therefore costs one `GET /v1/hub/work/issues/{id}`, and the
   head revision in memory is the Hub's, digest and all.
4. **One read per change of the header.** After the first load the replica does not poll. The
   control plane already polls the Hub and stamps `Panacea-Hub-Cursor` on `GET /v1/sessions`
   (`:1377-1381`); phase 4's loader stores it as `hubCursor`. The replica subscribes to that value
   and makes **one** `changes(after=cursor)` per change. A change that arrives mid-read is
   coalesced: when the read ends, one more read runs if the value moved. Phase 4's loader reads
   the feed for `brief.*` on the same change with its own cursor; two reads per change is the
   cost of not sharing one, recorded as a gap.
5. **In memory, beside the store, never in it.** The replica is a module object, not a slice of
   `useCockpit`, so `partialize` (`store/store.ts:34-48`) can never reach it. Task 37's
   `workSlice` stays the non-persisted UI state. Nothing under `src/work/**` or the Work
   components touches `localStorage`, `sessionStorage`, IndexedDB, the Cache API or the Tauri fs
   and store plugins; the no-persistence test proves it both statically and at run time.
6. **`version` and one notify per `transact`.** Every change to the maps happens inside
   `transact(fn)`, which bumps `version` once and notifies `useWork`'s subscribers once, so a page
   of 40 events is one re-render, and task 56's batch paint is one too (`task-56.md:187-190`).
   `useWork` is task 40's (`source.ts:202-223`), re-exported from `replica.ts` as SW.16 names it;
   its `sameData` is what keeps a caller whose selection did not change from re-rendering.
7. **Comments and subscriptions load on demand.** The full load is heads, head revisions, labels,
   members and the caller's subscriptions. An issue's comments load when a surface calls
   `loadThread(issueId)` (task 51's thread), and from then on `comment.*` events for that issue
   re-fetch its thread. A comment event for an issue whose thread is not loaded changes nothing.
8. **Which projects.** This task's `readableProjects()` dep answers `{projectId, teamId, key}[]`,
   the shape of task 57's `ProjectScope` (`task-57.md:95`), so task 57's `scope` and `admit` wrap it
   without a second list. Before task 57, the answer is taken as given.
