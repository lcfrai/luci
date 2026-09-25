### Task 55: The Orchestrator panel

**Slice:** TM1. **Size:** 3 focused hours. **dependsOn:** 39 (Fleet's attach handler, `GET
/v1/agents/{ag}/attach`), 40 (DIVE: `POST`/`GET`/`DELETE /v1/agents/{ag}/dive`, `DiveState`,
`task-40.md:320-330`, `:363-378`), 52 (ST.8's `OrchestratorView`, `ThreadView`; `teamSlice`;
`TeamClient`), and 53 (`AuthoredText`) and 51 (**approved**, states `7` and `8`). **Seam:** consumes
ST.8 and task 40's ST.5 extension.

The panel is a conversation with **your** Orchestrator (§9.4, `05-design.md:1001-1011`). You type
a human message; it answers in its next human-origin turn, and its kept reply is shown **verbatim,
as plain text**. Every directive it issues in a turn **you** started is shown inline as it is
issued, *"→ told mobile worker: …"* (§7.2), so *"who asked for this?"* is answered on the screen
where you asked. Beside the conversation sits the list of its open threads with other
Orchestrators, as typed state only: the other party, the subject, *"round 2 of 3"*, the deadline.
Opening a thread shows its messages with the author class on every line (*"Alice's Orchestrator
(agent)"*), rendered as plain text: no markdown links, no HTML, no terminal rendering (§7.3).
**DIVE** opens the Orchestrator's own terminal, for its owner only (§4.2). The panel is opened from
the project view (the strip's *"ask mine ↗"*, click-in's *"Ask my Orchestrator about this"*, task
53) or with ⌘K.

There is no panel for anyone else's Orchestrator. ST.8 has no route that reads one
(`OrchestratorView` is the viewer's own, task 52), and this task adds none.

#### Decisions this part makes

**(a) DIVE needs three local routes and one attach route, and they take no agent id.** The
Orchestrator is Fleet-spawned, so it is not in the laptop's registry and phase 2's
`/v1/sessions/{id}/attach` cannot reach it (task 21 hides it from `List` on purpose,
`task-21.md:27-35`). This task adds `control-plane/internal/api/orchestrator.go`:
`POST`/`GET`/`DELETE /v1/projects/{projectId}/orchestrator/dive` and `GET
/v1/projects/{projectId}/orchestrator/attach`. **None takes an `ag-…` from the desktop**: each
finds the viewer's own live Orchestrator in the project's `TeamHuman` (task 52), so a crafted
request cannot name another agent, and Fleet checks ownership again (task 39's `RouteAgent`,
`task-40.md:365-367`). The files are not in the skeleton's map for 55 (gap G55-1).

**(b) The DIVE terminal is phase 2's `Terminal`, handed a session value.** `Terminal` reads its
session from the store by id (`components/Terminal.tsx:113`), and the attach registry dials
`session.attachPath` (`api/attachRegistry.ts:132`, `acquireAttach(session, …)`). The Orchestrator
is not a registry session, and putting a fake one into `sessions` would be erased by the next poll
and shown on the rail. So `Terminal` gains one optional prop, `session?: Session`, used in place
of the store lookup when present, and the panel passes `diveSession(projectId, orch)`: a `Session`
value whose `attachPath` is the local attach route above, `backend: "fleet"`,
`teamScope: "team"`. It never enters the store, so the rail never shows it.

**(c) The conversation polls only while the panel is open, and never jumps.** `orchestrator(pid)`
every 3 s while mounted and visible, the same pause-while-hidden rule as `poller.ts`. The list
sticks to the bottom only if the reader is already there (`CLAUDE.md` frontend rules: *"never
scroll-jump when the user has scrolled up"*); otherwise a still line, *"New below"*, appears at
the bottom edge.

**(d) A sent message is pending until the view returns its id.** `ask` answers `201 {id: "hm-…"}`;
the composer shows the text at once as *"You · sending"*, and the pending line is dropped when a
poll's `conversation` contains that id. No toast. A Hub refusal (`400 text_refused`, G3 hygiene)
is shown under the composer in the problem's own words, and the text stays in the box.

**(e) Directives are shown by kind, from typed fields.** `free` prints *"→ told <to.name>:
"* then the text as `AuthoredText inline`. The typed kinds print one fixed line each, never agent
words: *"→ paused <name>"*, *"→ resumed <name>"*, *"→ narrowed <name> to <globs>"*, *"→ asked
<name> to wait for <leaseId>"*, *"→ asked <name> to rebase"*. Only `origin: human` directives
reach the view at all (task 52's `conversationFrom`).

**(f) ⌘K gets one command, on team projects only.** `buildCommands` (`palette/commands.ts:72`)
gains `verb:ask-orchestrator`, label *"Ask my Orchestrator"*, shown only when the selected
project's strip says `team: true`; `run()` is `openOrchestrator(projectId)`.
