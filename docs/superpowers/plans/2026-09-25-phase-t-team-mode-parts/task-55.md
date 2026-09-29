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
`task-21.md:24-31`). This task adds `control-plane/internal/api/orchestrator.go`:
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

**Files:**
- `desktop/src/components/orchestrator/OrchestratorPanel.tsx` — the sheet: header, conversation,
  composer, thread list, thread pane, DIVE (new)
- `desktop/src/components/orchestrator/{Conversation,ConvLine,Composer,ThreadList,ThreadPane,
  DivePane}.tsx`, `useOrchestrator.ts`, `diveSession.ts`, `orchestrator.css` (new)
- `desktop/src/components/orchestrator/{OrchestratorPanel,ThreadPane,DivePane}.test.tsx` (new)
- `desktop/src/api/orchestrator.ts`, `orchestrator.test.ts` — the DIVE client, on task 52's
  `createTransport` (new)
- `desktop/src/components/Terminal.tsx` — the optional `session` prop (modify, decision b)
- `desktop/src/components/AppShell.tsx` — mounts `<OrchestratorPanel />` when
  `orchestratorPanel !== null` (modify, one line)
- `desktop/src/components/palette/commands.ts` — `verb:ask-orchestrator` (modify, decision f)
- `control-plane/internal/api/orchestrator.go`, `orchestrator_test.go` — the four local routes (new)
- `control-plane/internal/backend/fleetbe/dive.go` — `BeginDive`, `Dive`, `StopDive`,
  `AttachAgent` (new; in task 21's package)

**Interfaces:**
- Consumes: ST.8's `OrchestratorView`, `ConvEntry`, `ThreadRow`, `ThreadView`, `ThreadMessage`,
  `TextA`, `TextH`, `AskSubject` (task 52); `teamSlice.orchestrators`, `threads`,
  `orchestratorPanel`, `setOrchestrator`, `setThread`, `closeOrchestrator`; `TeamClient.orchestrator`,
  `.thread`, `.ask`; `AuthoredText` (task 53); task 40's `DiveState` and its three routes; task 39's
  `GET /v1/agents/{ag}/attach`; phase 2's `Terminal`, `acquireAttach`, `internal/proxy`.
- Produces: the panel; `DiveState` on the local wire (task 40's shape, unchanged).

#### The local routes: `api/orchestrator.go` (complete surface)

```go
package api

// DiveFleet is the slice of fleetbe these routes call.
type DiveFleet interface {
	BeginDive(ctx context.Context, agentID string) (turns.DiveState, error) // POST  /v1/agents/{ag}/dive
	Dive(ctx context.Context, agentID string) (turns.DiveState, error)      // GET   /v1/agents/{ag}/dive
	StopDive(ctx context.Context, agentID string) (turns.DiveState, error)  // DELETE /v1/agents/{ag}/dive
	AttachAgent(ctx context.Context, agentID string) (*websocket.Conn, error) // GET /v1/agents/{ag}/attach
}

// ownOrchestrator finds the viewer's live Orchestrator in the project's
// TeamHuman (Role orchestrator, OwnerID == viewer). ok is false when none is
// live; the routes then answer 404 orchestrator_not_found.
func (t *TeamReader) ownOrchestrator(ctx context.Context, projectID string) (agentID string, ok bool, err error)

// registerOrchestrator adds, under the origin guard:
//   POST   /v1/projects/{projectId}/orchestrator/dive    -> 202 DiveState
//   GET    /v1/projects/{projectId}/orchestrator/dive    -> 200 DiveState
//   DELETE /v1/projects/{projectId}/orchestrator/dive    -> 202 DiveState
//   GET    /v1/projects/{projectId}/orchestrator/attach  -> 101, then phase 2's attach frames
// Fleet's codes are relayed unchanged: 409 orchestrator_busy (with details.retry),
// 409 agent_not_running, 404 agent_not_found. A solo project: 409 not_team_project.
func (a *API) registerOrchestrator(mux *routes)
```

The attach route is phase 2's local attach shape with one difference: instead of dialling a
sandbox with a stored secret (`api/attach.go:127-140`), it dials `fleetbe.AttachAgent` with the
user's bearer and hands both sockets to `internal/proxy`, which relays the frames unchanged
(`task-21.md:540-544`: *"the proxy relays them without knowing the backend"*). The origin check
and the loopback-only rule of phase 2's handler apply before any Fleet call.

#### `desktop/src/api/orchestrator.ts` (complete surface)

```ts
import { createTransport, type ClientOptions } from "./client";
import type { ProjectId } from "../store/types";

/** task 40's DiveState, field for field (task-40.md:324-330). since is RFC 3339. */
export interface DiveState { active: boolean; turn?: string; since?: string; waiting: number; }

export interface OrchestratorClient {
  beginDive(projectId: ProjectId): Promise<DiveState>;
  dive(projectId: ProjectId): Promise<DiveState>;
  endDive(projectId: ProjectId): Promise<DiveState>;
  /** "/v1/projects/<pid>/orchestrator/attach", for diveSession's attachPath. */
  attachPath(projectId: ProjectId): string;
}

export function createOrchestratorClient(opts?: ClientOptions): OrchestratorClient;
```

#### Components (complete props)

```tsx
/** Mounted by AppShell while orchestratorPanel is set. A right-hand sheet over the project view,
 *  --e-overlay elevation, closed by Esc or its close button; focus moves into the composer. */
export function OrchestratorPanel(): JSX.Element | null;

export function Conversation(props: { entries: ConvEntry[]; pending: PendingAsk[] }): JSX.Element;
export interface PendingAsk { id: string; text: string; }

/** One entry. human: "You" then the body (TextH). reply: the body (TextA) with
 *  "Your Orchestrator (agent)", white-space pre-wrap, "(cut at 8,000 bytes)" when truncated,
 *  and "This turn did not finish (<exit>)" when exit !== "ok". directive: decision (e). */
export function ConvLine(props: { entry: ConvEntry }): JSX.Element;

export function Composer(props: {
  projectId: ProjectId;
  subject: AskSubject | null;
  onSent(p: PendingAsk): void;
}): JSX.Element;

/** "Alice · auth/session.go · round 2 of 3 · by 04:40": the other party, subjectLabel,
 *  round of maxRounds, the deadline as a clock time in tabular-nums. A button per row. */
export function ThreadList(props: { threads: ThreadRow[]; onOpen(id: string): void }): JSX.Element;

/** Every message: sender.label, the kind as a word, typed fields as a fixed sentence, then the
 *  text (AuthoredText inline) when present; "(removed)" when tombstoned. */
export function ThreadPane(props: { projectId: ProjectId; threadId: string }): JSX.Element;

/** Rendered only when orchestrator.canDive. Start: beginDive, then Terminal with
 *  session={diveSession(...)}; shows "DIVE · N team events waiting" (DiveState.waiting, a
 *  count). End: endDive; the pane stays until the next dive() reads active: false. */
export function DivePane(props: { projectId: ProjectId; orch: { agentId: string; status: string } }): JSX.Element | null;

// diveSession.ts
export function diveSession(projectId: ProjectId, agentId: string, attachPath: string): Session;
```

The typed-field sentence `ThreadPane` prints for each message kind, fixed here so no agent words
stand in for it (the text, when present, follows it as `AuthoredText`):

| Kind | Sentence (names from `ThreadRow.other` and the viewer) |
|---|---|
| `ask` | *"asks about <subject.kind> <subject.ref>"* |
| `answer` | *"answers <inReplyTo>"* |
| `inform` | *"informs about <subject.kind> <subject.ref>"* |
| `propose`, `counter` | *"proposes <resolution.kind>"*, then per kind: `sequence` *"<first's side> first"*; `split` *"<side>: <globs>; <side>: <globs>"*; `yield` *"yield <globs>"*; `build-on` *"build on <base>"*; `proceed-both` *"both proceed on <paths>"* |
| `accept`, `reject`, `withdraw` | *"accepts <proposal>"*, *"rejects <proposal>"*, *"withdraws <proposal>"* |

A side is named *"your side"* or *"<name>'s side"* from the agent's owner in `TeamHuman`, never
from the message.

- [ ] **Step 1: Write the failing tests first**

Vitest + RTL over task 52's `orchestrator.json` and `thread.json` fixtures and a fake transport;
Go over a fake `DiveFleet` and a fake `TeamFleet`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `ConvLine › a markdown link is text` | the fixture reply containing `[the thread](https://example.invalid)` renders no `a` element; `textContent` holds the brackets and the URL | markdown rendering of `text·A` (§7.3) |
| `ConvLine › HTML and escapes are text` | a reply body `<img src=x onerror=1>` and one holding `\u001b[31m` render no `img`, and the characters as text | HTML or terminal rendering of an agent's words |
| `ThreadPane › the author class on every line` | every message element contains its `sender.label`; the fixture's two lines carry *"Alice's Orchestrator (agent)"* and *"Your Orchestrator (agent)"* | a thread line without its author (§9.4) |
| `ThreadPane › typed fields, not words, say what was proposed` | the counter with `text: null` still reads *"proposes split"* and both sides' globs | a message shown only through its text |
| `ThreadList › typed state` | the row reads `Alice · auth/session.go · round 2 of 3 · by 04:40`, the time in `tabular-nums` | a thread list built from message bodies |
| `Conversation › directives inline, in time order` | order is human `hm-01aa`, directive `dv-01ab` as *"→ told mobile worker: Leave store/ alone until I say otherwise."*, reply `ot-01ac` | directives shown in a separate log, or out of order |
| `Conversation › no jump when scrolled up` | with `scrollTop` set above the bottom, a new poll with one more entry leaves `scrollTop` unchanged and shows *"New below"* | scroll-jump on every poll (`CLAUDE.md`) |
| `Composer › sends once, with the subject` | Enter with subject `{path, auth/session.go}` calls `ask(pid, text, subject)` once; the chip's remove button clears it; Shift+Enter inserts a newline | a double send, or a subject that cannot be removed |
| `Composer › a refusal stays in the box` | the fake answers `400 text_refused`: the problem message shows under the box, the text is still there, `store.toasts` is `[]` | a lost message, or a toast |
| `DivePane › absent for a non-owner` | `canDive: false`: no DIVE button in the panel | DIVE offered to anyone who can open a panel |
| `DivePane › start, then attach` | click DIVE: `beginDive(pid)` once, then a `Terminal` mounted with a session whose `attachPath` is `/v1/projects/<pid>/orchestrator/attach`; the store's `sessions` is unchanged | a fake session written into the store, which the rail would show |
| `DivePane › waiting is a count` | `DiveState.waiting = 3` renders *"3 team events waiting"*; no `%`, no bar | a progress display (OD-27) |
| `palette › one command, team projects only` | `buildCommands` on a solo project has no `verb:ask-orchestrator`; on a team project it has one, and `run()` opens the panel | ⌘K offering a panel that cannot answer |
| `orchestrator.go › takes no agent id` | the four routes have no `{ag}` path value and ignore an `?agent=` query; each calls `DiveFleet` with the viewer's own Orchestrator from `TeamHuman`, even when a teammate's Orchestrator is listed first | a DIVE or attach into someone else's Orchestrator |
| `orchestrator.go › no Orchestrator is 404` | no live Orchestrator of the viewer's: `404 orchestrator_not_found`, and `DiveFleet` was not called | a DIVE request sent to Fleet with an empty id |
| `orchestrator.go › Fleet's busy is relayed` | `BeginDive` answering `409 orchestrator_busy` with `details.retry: "after_turn"` reaches the desktop with both | a busy DIVE shown as "Fleet is down" |

```bash
cd desktop && npm test -- orchestrator 2>&1 | tail -5
cd control-plane && go test ./internal/api/... -run Orchestrator 2>&1 | head -5
```

Expected: imports fail (`./OrchestratorPanel`, `undefined: DiveFleet`). That is the red.

- [ ] **Step 2: The local routes and `fleetbe/dive.go`**

`dive.go` is three calls on task 21's `do` helper and one dial mirroring `fleetbe/attach.go`
(task 21 Step 3) with the agent id in place of a resolved ref.

- [ ] **Step 3: `api/orchestrator.ts`, `diveSession.ts`, the `Terminal` prop**

`Terminal`'s change is one line at the store lookup (`Terminal.tsx:113`):
`const current = props.session ?? useCockpit.getState().sessions[sessionId];`, and `sessionId`
for a DIVE is `diveSession(...).id`, `"orch-<agentId>"`, which no registry id can equal (phase 2
ids are `s-…`, `store/types.ts:13`).

- [ ] **Step 4: The panel**

`ConvLine`, `Conversation`, `Composer`, `ThreadList`, `ThreadPane`, `DivePane`, then
`OrchestratorPanel` and the mount; then the palette command. `orchestrator.css` imports the token
layer first and adds no transition beyond the audited list (opacity at `var(--dur-fast)` for the
sheet's fade, `designsystem.test.ts:181`).

- [ ] **Step 5: Acceptance**

```bash
cd control-plane && go vet ./... && go test ./internal/api/... ./internal/backend/fleetbe/... -count=1
cd desktop && npm run typecheck && npm test
```

Expected: `ok` for both Go packages; `tsc` silent; every Vitest file passes, phase 2's
`Terminal.test.tsx` and `commands.test.ts` and the design-system audit unchanged. Visual
acceptance against task 51's states `7` and `8` is the owner's.

- [ ] **Step 6: Commit**

```bash
git add desktop/src/components/orchestrator desktop/src/api/orchestrator.ts \
  desktop/src/api/orchestrator.test.ts control-plane/internal/api/orchestrator.go \
  control-plane/internal/api/orchestrator_test.go control-plane/internal/backend/fleetbe/dive.go
git commit -F <msgfile> -- desktop/src/components/orchestrator desktop/src/api/orchestrator.ts \
  desktop/src/api/orchestrator.test.ts desktop/src/components/Terminal.tsx \
  desktop/src/components/AppShell.tsx desktop/src/components/palette/commands.ts \
  control-plane/internal/api/orchestrator.go control-plane/internal/api/orchestrator_test.go \
  control-plane/internal/backend/fleetbe/dive.go
```

**What this task does NOT claim.**
- That replies appear. Until the Hub serves turn replies (task 52's gap G52-2), `conversation`
  holds human messages and directives only; the panel renders that state, and the fixture has both.
- That DIVE works end to end. It is proven against fakes; tasks 39 and 40 are the Fleet halves,
  and task 57 is the first real DIVE.
- That the panel works for an org auditor. An auditor's reads of other people's conversations are
  the Hub's (routes 63, 66, 68, 71 admit *auditor*), and no auditor surface is in TM1.

**Gaps (for the skeleton's owner).**
- **G55-1.** The skeleton maps task 55 to `desktop/src/components/orchestrator/*` only; DIVE also
  needs `control-plane/internal/api/orchestrator.go` and `fleetbe/dive.go` (decision a), because the
  Orchestrator is outside the laptop's registry by design (task 21).
- **G55-2.** Task 40's `DiveState` lives in Fleet's `turns` package as read here
  (`task-40.md:324-330`); if task 40 moves it to `wire`, `DiveFleet`'s return type follows.
- **G55-3 (carried from fix batch b07's re-review 2, RR2-I2).** A decide shows `evidence.text` as
  the Orchestrator's quoted words, with "after reading N teammate messages" from `readTotal`, so a
  person approves only words they can read. It needs task 52's G52-7 (the type field and the
  held-act read). Recorded at `2026-09-25-phase-t-team-mode-review.md:4387-4395`.
