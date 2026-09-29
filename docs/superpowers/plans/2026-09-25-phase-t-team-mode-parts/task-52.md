### Task 52: Team read models and actions on the local control plane; the desktop client

**Slice:** TM1. **Size:** 3 focused hours. **dependsOn:** 21 (`fleetbe`, `TeamLauncher`), 43 (the
Hub's role-to-verb rows and team policy), 51 (the prototype's sample keys). **Seam:** defines
**ST.8**, the team read model the cockpit renders and its author-class labels, consumed by 53–56,
63 and 67.

The cockpit never talks to Fleet or the Hub directly. Phase 2's rule is that the desktop has one
server, the local control plane on loopback (`desktop/src/api/client.ts:30`,
`DEFAULT_BASE_URL = "http://127.0.0.1:8730"`), and that `desktop/src/store/types.ts` mirrors its JSON
field for field (`types.ts:3-5`: *"Renaming a field here breaks at runtime, not at compile time, so
the names are binding."*). This task keeps both rules. It adds:

1. **One Fleet route for live facts**, `GET /v1/projects/{p}/team` on `fleetd`, under task 12's
   `VerbTeamRead` (`task-12.md:319`, *"the strip and task cards"*; viewer and up). Its body,
   `verbs.TeamHuman`, is the third sibling of task 22's `TeamWorker` and `TeamOrchestrator`, built
   from the **same** `verbs.Sources` (`task-22.md:77-88`; task 22 lists 52 under "Required by",
   `task-22.md:60`), so the three views cannot disagree about who holds what.
2. **Two Fleet routes for the caller's own threads**, under `VerbThreadRead` (own only,
   `task-12.md:346`): the open-thread list as typed state, and one thread's messages. Fleet reads
   them from the Hub's route 63 as Fleet·O under the caller's own Orchestrator delegation, the
   only principal route 63 admits for a participant's owner (`task-01.md:274`), and relays them.
3. **The local routes** in `control-plane/internal/api/team.go`, which read those three Fleet
   routes through `fleetbe` and the Hub's own-only routes (66, 68, 71, 74) through task 08's
   `hubclient.Team` as the signed-in user, and **assemble** ST.8: the strip, a person's detail,
   the Desk's four row kinds, the Orchestrator conversation, a thread, and the project's team
   settings. Six actions write, each to exactly one Hub route, as the user. **Task 21 step 6 already serves four local team routes** (fix batch b07, G33-1), bare relays for TM1-α: `GET /v1/team/escalations`, `POST /v1/team/escalations/{escalationId}/answer`, `POST /v1/team/human-messages` and `GET /v1/team/directives` (`task-21.md:578-615`). This task keeps all four at their paths and registers none of them a second time (the mux panics on a duplicate pattern): the answer row of the route table below **is** task 21's route, whose `200` this task widens to `DecideItem` in `teamrelay.go`, and the ask route writes Hub route 70 through the same `hubclient.Team.WriteHumanMessage` as task 21's message relay. Neither reads an owner from a body.
4. **The desktop client**: `desktop/src/api/team.ts`, `store/slices/teamSlice.ts`, and the team
   block of `store/types.ts`, field for field, with every `text·A` field typed as such.

Four rules hold everywhere in this task:

- **No teammate's terminal, transcript, prompt, directive text, reply or conversation reaches the
  read model** (§9.2, `05-design.md:982-983`). The only agent words a teammate's row carries are
  the task title and a `blocked --on human` report, both quoted with their class (§9.1). A key
  allowlist test pins every ST.8 type (Step 1).
- **Every free-text value is a `TeamText`**: the body, its author class, the authoring agent, the
  owner, the time, and a **label assembled from typed fields** (*"Alice's Orchestrator (agent)"*,
  *"Priya's export worker (agent)"*, *"You"*). No model writes a label and no label quotes a body.
- **The owner comes from the token** (global constraint 3). No local route takes a user id in a
  body; the one path parameter that names a person (`…/people/{userId}`) selects a row to read.
- **Nothing is decided here that Fleet or the Hub decides.** The local control plane assembles and
  labels; it never filters by role (Fleet and the Hub already have), never infers a status, and
  never writes a record in anyone's name but the user's.

#### Decisions this part makes

**(a) One live read, assembled locally.** Fleet serves one `TeamHuman` per project; the strip and
every person's detail are cut from it on the local side. A second Fleet route per person would
double the Entra checks for no new fact. The strip polls it every 5 s, which matches `machined`'s
5–10 s sweep (skeleton diagram, `2026-09-25-phase-t-team-mode.md:57`); polling faster shows
nothing new.

**(b) The five-row cap is the UI's.** ST.8 carries every person with a live agent, ordered; task
53 renders five and *"+N more"*. A cap in the model would hide the sixth person from click-in.

**(c) OD-27 removes HORIZON's time.** §9.3's row reads *"…converge on `auth/session.go`, ~20
min"* (`05-design.md:992`). OD-27 (`owner-decisions.md:164`) bans ETAs, so ST.8's `HorizonItem`
has no time-to-collision field at all, only the tier, which says **how** it was predicted.

**(d) DECIDED WITHOUT YOU is found by typed fields, and Undo is a message to your own
Orchestrator.** Task 29 writes it as an `inform` escalation with `fromAgentId` absent and
`threadId` set (`task-29.md:54-58`). The local side classifies by exactly that, never by the
question's text prefix. Undo cannot touch the other side (OD-16), so it writes one `hm-…` to the
user's own Orchestrator from a fixed template naming the conflict and the resolution; that starts
a **human-origin** turn, in which the Orchestrator may renegotiate (§7.2). `undo.available` is true
only while the conflict is `accepted` and the resolution is `yield` or `sequence`; once it is
`resolved` (landed) there is nothing to reverse.

**(e) The read cursor is the desktop's.** SINCE YOU LAST LOOKED is a delta against a per-user
cursor (`docs/AGENTIC-UX.md` §6 item 1). No Hub record holds one today, so in TM1 the cursor is
`teamSeenAt`, persisted by the store beside the other prefs, and sent as `?since=`. It is per
device, which is wrong for a user on two laptops and right for everything else; the Hub read
cursor is a gap, listed, not built here.

**(f) The route badge's state is read, never inferred.** `TeamSettings.route` comes from the Hub's
team policy (route 74, `task-01.md:285`) and phase 6's inference panel state, never from which
gateway a session happened to call. Task 56 renders it.

**Files:**
- `control-plane/internal/fleet/verbs/teamhuman.go` — `TeamHuman`, `BuildHuman` over task 22's
  `Sources` (new; in task 22's package)
- `control-plane/internal/fleet/server/desk.go` — the three desktop routes on `fleetd` (new; not in
  the skeleton's file map, listed as a gap)
- `control-plane/internal/backend/fleetbe/team.go` — `Team`, `Threads`, `ThreadMessages` on the
  `fleetbe` client (new; in task 21's package)
- `control-plane/internal/api/teamview.go` — every ST.8 wire type (new)
- `control-plane/internal/api/team.go` — `TeamReader`, its consumed interfaces, the routes and
  their handlers (new)
- `control-plane/internal/api/teamassemble.go` — the pure assembly: strip, detail, Desk,
  conversation, labels (new)
- `control-plane/internal/api/{team,teamassemble}_test.go`, `control-plane/internal/fleet/verbs/teamhuman_test.go`,
  `control-plane/internal/fleet/server/desk_test.go` (new)
- `control-plane/internal/api/testdata/team-view/v1/{strip,person,desk,orchestrator,thread,settings}.json`
  — the golden ST.8 bodies both the Go and the desktop tests read (new)
- `control-plane/internal/api/api.go` — `Deps.Team *TeamReader`; `registerTeam(mux)` when set
  (modify, one anchored hunk beside the sessions registration); `control-plane/internal/api/teamrelay.go` — task 21's answer relay, its `200` widened to `DecideItem` (modify, b07)
- `desktop/src/store/types.ts` — the team block below, and `Session.teamScope` (task 21's field)
  (modify)
- `desktop/src/api/team.ts`, `desktop/src/api/team.test.ts` (new)
- `desktop/src/store/slices/teamSlice.ts`, `desktop/src/store/slices/teamSlice.test.ts` (new)
- `desktop/src/store/store.ts` — the fourth slice; `teamSeenAt` joins `Persisted` (modify)
- `desktop/src/store/selectors.ts` — `selectTeamSettings(state, projectId)`, `selectTeamStrip`
  (modify; task 56 reads the first, `task-56.md:127-130`)

**Interfaces:**
- Consumes: task 22's `verbs.Sources`, `BriefRef`, `LeaseView`, `Quoted` (`task-22.md:77-246`);
  task 24's `Report` (`task-24.md:117-125`) through a new `Sources.Reports` reader; task 12's
  `Server.Route`, `Principal`, `VerbTeamRead`, `VerbThreadRead` (`task-12.md:316-375`, `:569`);
  task 21's `fleetbe.Backend` and `TeamProjects` (`task-21.md:307-329`); task 08's `hubclient.Team`
  reads and writes (`task-08.md:80-122`); task 03's record types (`task-03.md:477-952`); task 02's
  `teamtext.Text` wire shape (`task-02.md:242-250`).
- Produces: ST.8 as written below, verbatim; the fixtures; `teamSlice` for 53–56.

#### The Fleet side: `verbs/teamhuman.go` (complete)

```go
package verbs

// TeamHuman is GET /v1/projects/{p}/team for a member's desktop (VerbTeamRead).
// It is task 22's TeamWorker and TeamOrchestrator's third sibling, built from
// the same Sources. It has no field for a terminal, a transcript, an opening
// prompt, a directive, a thread message, a report other than blocked --on
// human, or an Orchestrator's reply (§9.2). TestHumanKeysAllowlist pins it.
type TeamHuman struct {
	V           int             `json:"v"` // 1
	GeneratedAt time.Time       `json:"generatedAt"`
	Project     string          `json:"project"`
	Agents      []HumanAgent    `json:"agents"`    // live only, ordered by owner then startedAt
	Conflicts   []HumanConflict `json:"conflicts"` // open only: raised|negotiating|accepted|escalated
}

type HumanAgent struct {
	Agent     string      `json:"agent"`   // ag-…
	OwnerID   string      `json:"ownerId"` // u-…, the Hub user id
	Owner     string      `json:"owner"`   // display name, cleaned (People)
	Role      string      `json:"role"`    // orchestrator | worker
	Parent    string      `json:"parent"`  // the Orchestrator's ag-…; "" for an Orchestrator
	Harness   string      `json:"harness"` // id@version
	Task      string      `json:"task"`    // tk-… or ""
	TaskState string      `json:"taskState,omitempty"`
	Brief     *BriefRef   `json:"brief,omitempty"`
	Title     *Quoted     `json:"title,omitempty"` // the task title, text·A or text·H (gap G52-1)
	Status    string      `json:"status"`          // starting | working | waiting | idle
	WaitingOn string      `json:"waitingOn,omitempty"` // lease | peer | human
	Words     *Quoted     `json:"words,omitempty"`     // the open blocked --on human report, text·A
	Files     []HumanFile `json:"files"`               // newest edit first, ≤ 20
	Leases    []LeaseView `json:"leases"`
	StartedAt time.Time   `json:"startedAt"`
}

type HumanFile struct {
	Path     string    `json:"path"`
	LastEdit time.Time `json:"lastEdit"`
}

type HumanConflict struct {
	ID       string   `json:"id"`   // cf-…
	Tier     string   `json:"tier"` // P | H | M | S
	State    string   `json:"state"`
	A        string   `json:"a"`
	B        string   `json:"b"`
	Paths    []string `json:"paths"`
	ThreadID string   `json:"threadId,omitempty"`
}

// Reports is task 24's store, read for one fact: a worker's newest report of
// kind "blocked" with On "human" that no later report or status has closed.
type Reports interface {
	OpenBlockedOnHuman(ctx context.Context, worker model.AgentID) (teamtext.Text, bool)
}

// BuildHuman assembles TeamHuman. It reads Sources exactly as task 22's
// builders do; it adds only the Reports read and the owner's user id.
func BuildHuman(ctx context.Context, s Sources, r Reports, project string) (TeamHuman, error)
```

`Sources` does not change shape: `Reports` is a second argument, so task 22's tests are untouched.
`Title` is filled from `Cards.Card(...).Title`; `Words` only when the agent's status is `waiting`
with `WaitingOn == human` **and** `OpenBlockedOnHuman` answers, so a stale report never reappears
under a working agent. An agent whose status is `ended` is not listed; `starting` is.

#### The Fleet side: `server/desk.go` (complete)

Three routes, each registered through task 12's `Server.Route`, so Entra runs per request and the
verb table (`task-12.md:343-346`) decides who may call. None takes a body; none reads an owner from
anything but the `Principal`.

| Route | Verb | 200 body | Refusals |
|---|---|---|---|
| `GET /v1/projects/{p}/team` | `VerbTeamRead` (viewer and up) | `verbs.TeamHuman` | `403 forbidden` (not on the team); `404 project_not_found` |
| `GET /v1/projects/{p}/threads?party=me` | `VerbThreadRead` (own) | `{"threads":[DeskThread]}` | `403 forbidden`; `400 bad_request` for any other `party` |
| `GET /v1/projects/{p}/threads/{th}/messages?after=&limit=` | `VerbThreadRead` (own) | `hubapi.Page[hubapi.Message]`, relayed | `404 thread_not_found` when the caller's Orchestrator is not a participant (existence is not leaked); the Hub's code otherwise |

```go
package server

// DeskThread is one open thread the caller's own Orchestrator is party to,
// typed state only (§9.4): no message, no text.
type DeskThread struct {
	ID           string               `json:"id"` // th-…
	State        string               `json:"state"` // open | escalated
	Participants []string             `json:"participants"` // two ag-…
	OtherUserID  string               `json:"otherUserId"`
	Subject      hubapi.ThreadSubject `json:"subject"`
	Rounds       protocol.Rounds      `json:"rounds"`    // task 28: mine, theirs, cap (task-28.md:207-211)
	Deadline     time.Time            `json:"deadline"`
}

// DeskDeps is what the three routes read. Threads is task 28's working state;
// Messages reads route 63 as Fleet·O under the given delegation (task 11's client).
type DeskDeps struct {
	Sources    verbs.Sources
	Reports    verbs.Reports
	Threads    func(ctx context.Context, orch model.AgentID) []DeskThread
	Orch       func(ctx context.Context, project string, owner model.Owner) (model.Agent, bool)
	Messages   func(ctx context.Context, delegation, project, thread string, page hubapi.PageRequest) (hubapi.Page[hubapi.Message], error)
}

// RegisterDesk adds the three routes. It is called from task 12's New.
func (s *Server) RegisterDesk(d DeskDeps)
```

The two thread routes find the caller's own Orchestrator with `Orch(project, Owner(p.UserID))`; no
live Orchestrator answers `{"threads":[]}` and `404 thread_not_found`. The message relay passes the
Orchestrator's own `Delegation` (task 09's `Agent.Delegation`) to `Messages`, so the Hub's
route 63 checks participation a second time and the Hub's audit names the Orchestrator's owner
as the principal.

#### The `fleetbe` client: `fleetbe/team.go` (complete)

```go
package fleetbe

// Team, Threads and ThreadMessages call the three desk routes as the signed-in
// user, through the same do() helper and error mapping as the rest of fleetbe
// (task-21.md:530-536). projectID is the Hub's pj-… id.
func (b *Backend) Team(ctx context.Context, projectID string) (verbs.TeamHuman, error)
func (b *Backend) Threads(ctx context.Context, projectID string) ([]server.DeskThread, error)
func (b *Backend) ThreadMessages(ctx context.Context, projectID, threadID string,
	page hubapi.PageRequest) (hubapi.Page[hubapi.Message], error)
func (b *Backend) Spend(ctx context.Context, projectID string) (budget.SplitView, error)
```

`fleetbe` imports `verbs` and `server` for their types only, which is legal inside one module
(`cp/`); the types are the wire, so a rename breaks the build, not the cockpit.

#### ST.8 on the Go side: `api/teamview.go` (complete)

Every time is RFC 3339 UTC as a string, as the rest of the local API (`types.ts:156`: *"Kept as
strings; parsed only at the point of display."*). Every slice is non-nil on the wire (`[]`, never
`null`), which one marshal test pins.

```go
package api

// TeamText is ST.8's one free-text shape: task 02's teamtext.Text on the wire
// (task-02.md:244-250) plus Label, assembled from typed fields by labelFor.
type TeamText struct {
	Body          string  `json:"body"`
	Author        string  `json:"author"`        // "agent" | "human"
	AuthorAgentID *string `json:"authorAgentId"` // ag-… for agent text, null for human
	OwnerUserID   string  `json:"ownerUserId"`
	At            string  `json:"at"`
	Label         string  `json:"label"` // "Alice's Orchestrator (agent)", "You"
}

type PersonRef struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type AgentRef struct {
	AgentID string `json:"agentId"`
	Name    string `json:"name"` // "Orchestrator" | "<brief section title, ≤ 40> worker" | "worker"
}

type BriefSection struct {
	Ref   string `json:"ref"`   // the brief section id the Hub task's reason names
	Title string `json:"title"` // ≤ 120, the approved brief's words, not an agent's
}

type RouteState struct {
	Current   string  `json:"current"`   // "global" | "au"
	Required  bool    `json:"required"`  // the data class allows AU only
	RaisedAt  *string `json:"raisedAt"`  // when the team ratchet moved it to au; null otherwise
	CanLower  bool    `json:"canLower"`  // the caller holds org compliance
}
```

```go
// ── the strip and click-in (task 53) ────────────────────────────────────────

// TeamStrip is GET /v1/projects/{projectId}/team. People holds every person
// with a live agent on the project, the viewer included, ordered: the viewer
// last, others by their earliest live agent's startedAt. The UI caps at 5.
type TeamStrip struct {
	V           int         `json:"v"` // 1
	ProjectID   string      `json:"projectId"`
	Team        bool        `json:"team"` // false: a solo project, People is []
	GeneratedAt string      `json:"generatedAt"`
	Route       RouteState  `json:"route"`
	People      []PersonRow `json:"people"`
}

type PersonRow struct {
	UserID      string     `json:"userId"`
	DisplayName string     `json:"displayName"`
	You         bool       `json:"you"`
	Status      string     `json:"status"` // "working" | "waiting" | "idle": the row's one dot
	WaitingOn   *WaitingOn `json:"waitingOn"`
	Line        Line       `json:"line"`
	Files       RowFiles   `json:"files"`
	ChangedAt   string     `json:"changedAt"` // last change of task, files or status; drives the flash
}

// WaitingOn: Kind "human" names the person the agent waits on (its own owner)
// and may carry the agent's words; "lease" and "peer" are Fleet's own facts
// and carry neither.
type WaitingOn struct {
	Kind  string    `json:"kind"`  // "human" | "lease" | "peer"
	Name  *string   `json:"name"`  // kind human only
	Words *TeamText `json:"words"` // kind human only, text·A
}

// Line is the assembled line (§9.1): never a model summary.
type Line struct {
	BriefSection *BriefSection `json:"briefSection"` // the person's newest active task's section
	TaskTitle    *TeamText     `json:"taskTitle"`    // null when Fleet withholds it (gap G52-1)
	Agents       int           `json:"agents"`       // live agents, the Orchestrator excluded
}

type RowFiles struct {
	Recent       []string `json:"recent"`       // ≤ 3, newest edit first
	More         int      `json:"more"`         // the rest of the person's observed paths
	HeldByOthers []string `json:"heldByOthers"` // the subset of Recent someone else also holds
}

// PersonDetail is GET /v1/projects/{projectId}/team/people/{userId}.
type PersonDetail struct {
	V           int              `json:"v"`
	ProjectID   string           `json:"projectId"`
	UserID      string           `json:"userId"`
	DisplayName string           `json:"displayName"`
	You         bool             `json:"you"`
	Tasks       []DetailTask     `json:"tasks"`
	Agents      []DetailAgent    `json:"agents"`
	Files       []DetailFile     `json:"files"`
	Leases      []DetailLease    `json:"leases"`
	Conflicts   []DetailConflict `json:"conflicts"`
}

type DetailTask struct {
	ID           string        `json:"id"` // tk-…
	State        string        `json:"state"`
	Title        *TeamText     `json:"title"`
	BriefSection *BriefSection `json:"briefSection"` // the provenance link
}

// Card is the Desk's three-line agent card (docs/AGENTIC-UX.md:148), each line
// a template over typed fields (cardFor), never an agent's words.
type Card struct {
	State   string `json:"state"`   // "WORKING" | "WAITING ON PRIYA" | "WAITING ON A LEASE" | "IDLE" | "STARTING"
	Meaning string `json:"meaning"` // "<brief section title> · tk-…"
	Next    string `json:"next"`    // "holds auth/** until 04:40" | "no lease held"
}

type DetailAgent struct {
	AgentRef
	Role    string `json:"role"`
	Harness string `json:"harness"`
	TaskID  string `json:"taskId"`
	Status  string `json:"status"`
	Card    Card   `json:"card"`
}

type DetailFile struct {
	Path         string `json:"path"`
	LastEdit     string `json:"lastEdit"`
	HeldByOthers bool   `json:"heldByOthers"`
}

type DetailLease struct {
	ID        string   `json:"id"` // ls-…
	AgentID   string   `json:"agentId"`
	Globs     []string `json:"globs"`
	Mode      string   `json:"mode"`  // intent | hold | bulk
	State     string   `json:"state"`
	ExpiresAt string   `json:"expiresAt"`
}

// DetailConflict: WithMine is the viewer's agent on the other side, or ""
// when the conflict is between two agents neither of which is the viewer's.
type DetailConflict struct {
	ID       string   `json:"id"`
	Tier     string   `json:"tier"`
	State    string   `json:"state"`
	Paths    []string `json:"paths"`
	WithMine string   `json:"withMine"`
}
```

```go
// ── the Desk (task 54): GET /v1/team/desk?since=<RFC 3339> ─────────────────

// TeamDesk is account-level (the Desk is above projects, docs/AGENTIC-UX.md
// §5): every team project the viewer is a member of, merged, newest first
// within each list. No section is new; each list lands in an existing one.
type TeamDesk struct {
	V                  int            `json:"v"`
	GeneratedAt        string         `json:"generatedAt"`
	Horizon            []HorizonItem  `json:"horizon"`
	NeedsYou           []DecideItem   `json:"needsYou"`
	SinceYouLastLooked SinceBlock     `json:"sinceYouLastLooked"`
	DecidedWithoutYou  []DecidedItem  `json:"decidedWithoutYou"`
}

// HorizonItem is an open conflict one of the viewer's agents is party to, kept
// until the conflict is resolved or cleared (never dropped on accepted). No
// time-to-collision field exists (decision c, OD-27).
type HorizonItem struct {
	ConflictID string    `json:"conflictId"`
	ProjectID  string    `json:"projectId"`
	Tier       string    `json:"tier"`  // P | H | M | S: how it was predicted
	State      string    `json:"state"` // raised | negotiating | accepted | escalated
	Paths      []string  `json:"paths"`
	Mine       AgentRef  `json:"mine"`
	Theirs     TheirSide `json:"theirs"`
	At         string    `json:"at"` // the conflict's createdAt
}

type TheirSide struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AgentID     string `json:"agentId"`
}

// DecideItem is an unanswered decide escalation (route 66, own only).
type DecideItem struct {
	EscalationID string         `json:"escalationId"`
	ProjectID    string         `json:"projectId"`
	ThreadID     *string        `json:"threadId"`
	FromAgentID  *string        `json:"fromAgentId"` // null when Fleet raised it
	Question     TeamText       `json:"question"`
	Options      []DecideOption `json:"options"` // 1–4, the Hub's bound
	Default      int            `json:"default"` // index into Options; the safe default
	Deadline     string         `json:"deadline"`
	At           string         `json:"at"` // the escalation's createdAt: the Desk's aging
}

type DecideOption struct {
	Label       TeamText `json:"label"`
	Consequence TeamText `json:"consequence"`
}

// SinceBlock: Items are informs (not DECIDED WITHOUT YOU) and the free-text
// answers the viewer's Orchestrator sent a peer, after Since, oldest first.
type SinceBlock struct {
	Since string      `json:"since"`
	Items []SinceItem `json:"items"`
}

// SinceItem is a flat union on Kind. "inform": EscalationID, Question.
// "answer": MessageID, ThreadID, To, Text. Keys of the other kind are omitted.
type SinceItem struct {
	Kind         string     `json:"kind"` // "inform" | "answer"
	ProjectID    string     `json:"projectId"`
	At           string     `json:"at"`
	EscalationID string     `json:"escalationId,omitempty"`
	Question     *TeamText  `json:"question,omitempty"`
	MessageID    string     `json:"messageId,omitempty"`
	ThreadID     string     `json:"threadId,omitempty"`
	To           *PersonRef `json:"to,omitempty"`
	Text         *TeamText  `json:"text,omitempty"`
}

// DecidedItem is task 29's Fleet-written inform (fromAgentId null, threadId
// set), joined to its conflict by threadId. Globs: the yielded globs for a
// yield, the conflict's paths otherwise.
type DecidedItem struct {
	EscalationID string    `json:"escalationId"`
	ConflictID   string    `json:"conflictId"`
	ProjectID    string    `json:"projectId"`
	Resolution   string    `json:"resolution"` // split | sequence | yield | build-on | proceed-both
	Other        PersonRef `json:"other"`
	Globs        []string  `json:"globs"`
	At           string    `json:"at"`
	Summary      string    `json:"summary"` // the question body: task 29's template over typed fields
	Undo         Undo      `json:"undo"`
}

type Undo struct {
	Available bool `json:"available"` // conflict accepted, resolution yield or sequence
}
```

```go
// ── the Orchestrator panel (task 55) ────────────────────────────────────────

// OrchestratorView is GET /v1/projects/{projectId}/orchestrator: the viewer's
// OWN Orchestrator only. There is no route that reads anyone else's.
type OrchestratorView struct {
	V            int          `json:"v"`
	ProjectID    string       `json:"projectId"`
	Orchestrator *OrchState   `json:"orchestrator"` // null: none live yet
	Conversation []ConvEntry  `json:"conversation"` // oldest first, the last 100
	Threads      []ThreadRow  `json:"threads"`
}

type OrchState struct {
	AgentID string `json:"agentId"`
	Status  string `json:"status"`
	CanDive bool   `json:"canDive"` // the viewer owns it and task 40's DIVE is served
}

// ConvEntry is a flat union on Kind; keys of the other kinds are omitted.
//   "human":     ID (hm-…), Text (text·H)
//   "reply":     TurnID (ot-…), Text (text·A), Exit, Truncated
//   "directive": ID (dv-…), DirectiveKind, To, Params, Text (text·A, free only)
// Directives are the viewer's own with origin "human" only: issued in a turn
// the viewer started (§9.4, §7.2).
type ConvEntry struct {
	Kind          string                  `json:"kind"`
	At            string                  `json:"at"`
	ID            string                  `json:"id,omitempty"`
	TurnID        string                  `json:"turnId,omitempty"`
	Text          *TeamText               `json:"text,omitempty"`
	Exit          string                  `json:"exit,omitempty"` // ok | failed | timeout | lost
	Truncated     bool                    `json:"truncated,omitempty"`
	DirectiveKind string                  `json:"directiveKind,omitempty"`
	To            *AgentRef               `json:"to,omitempty"`
	Params        *hubapi.DirectiveParams `json:"params,omitempty"`
}

// ThreadRow is DeskThread with names: "round 2 of 3" is Round of MaxRounds,
// where Round = max(rounds.mine, rounds.theirs) and MaxRounds = rounds.cap.
type ThreadRow struct {
	ID           string               `json:"id"`
	State        string               `json:"state"`
	Other        PersonRef            `json:"other"`
	Subject      hubapi.ThreadSubject `json:"subject"`
	SubjectLabel string               `json:"subjectLabel"` // the path, "tk-…" or "cf-… · auth/session.go"
	Round        int                  `json:"round"`
	MaxRounds    int                  `json:"maxRounds"`
	Deadline     string               `json:"deadline"`
}

// ThreadView is GET /v1/projects/{projectId}/orchestrator/threads/{threadId}.
type ThreadView struct {
	Thread   ThreadRow       `json:"thread"`
	Messages []ThreadMessage `json:"messages"` // oldest first
}

type ThreadMessage struct {
	ID         string               `json:"id"`
	Kind       string               `json:"kind"` // ask | answer | inform | propose | accept | reject | counter | withdraw
	Templated  bool                 `json:"templated"`
	At         string               `json:"at"`
	Tombstoned bool                 `json:"tombstoned"`
	Sender     Sender               `json:"sender"`
	Fields     hubapi.MessageFields `json:"fields"` // the Hub's flat union, verbatim
	Text       *TeamText            `json:"text"`   // null when absent or tombstoned
}

type Sender struct {
	AgentID string `json:"agentId"`
	UserID  string `json:"userId"`
	Label   string `json:"label"` // "Alice's Orchestrator (agent)" | "Your Orchestrator (agent)"
}

// ── settings (task 56): GET /v1/projects/{projectId}/team/settings ─────────

type TeamSettings struct {
	V         int            `json:"v"`
	ProjectID string         `json:"projectId"`
	DataClass *DataClassView `json:"dataClass"` // null: the Hub has none recorded (gap G52-6)
	Route     RouteState     `json:"route"`
	Spend     Spend          `json:"spend"`
}

// DataClassView is phase 6's DataClass shape (phase-6 task-22.md:62-66).
type DataClassView struct {
	Level     string `json:"level"`     // rendered verbatim, never switched on
	Residency string `json:"residency"` // "any" | "au-only"
}

// Spend is task 37's SplitView cut to what task 56 renders (task-37.md:431-440),
// in cents, the UTC day. A nil figure is "not known", never 0. Never a ratio.
type Spend struct {
	Day               string `json:"day"`
	WorkersCents      *int64 `json:"workersCents"`
	CoordinationCents *int64 `json:"coordinationCents"`
	QuotaCents        *int64 `json:"quotaCents"` // team policy's memberDailyCents; nil: none set
}

// ── action bodies ───────────────────────────────────────────────────────────

// AskRequest is POST …/orchestrator/messages. Subject is optional; when set it
// becomes a fixed first line ("About path auth/session.go:") of the text·H body.
type AskRequest struct {
	Text    string   `json:"text"` // 1–2,000 after the subject line is added
	Subject *Subject `json:"subject,omitempty"`
}

type Subject struct {
	Kind string `json:"kind"` // "task" | "path" | "agent" | "conflict"
	Ref  string `json:"ref"`  // tk-…, a repo-relative path, ag-…, cf-…
}

type AnswerRequest struct {
	Choice int     `json:"choice"`
	Text   *string `json:"text,omitempty"` // ≤ 300
}

type NoteRequest struct {
	Target hubapi.NoteTarget `json:"target"`
	Text   string            `json:"text"` // ≤ 1,000
}

// Created answers every action that writes one Hub record.
type Created struct {
	ID string `json:"id"` // hm-… | nt-… | es-…
}
```

#### The local routes (complete)

All under phase 2's origin guard (`api.go:379`, so `/api/v1/…` aliases each), all phase 2 problem
bodies. `{projectId}` is the **cockpit's** project id; `TeamReader` maps it to the Hub's `pj-…`
through task 21's `TeamProjects` production type (see gap G52-4). A solo project answers the read
routes with an empty, `team: false` body and the write routes with `409 not_team_project`.

| Route | Reads or writes | 200/201 body |
|---|---|---|
| `GET /v1/projects/{projectId}/team` | Fleet `…/team`; Hub route 74 | `TeamStrip` |
| `GET /v1/projects/{projectId}/team/people/{userId}` | Fleet `…/team` (same call, cached ≤ 2 s) | `PersonDetail`; `404 person_not_found` when the person has no live agent |
| `GET /v1/team/desk?since=` | per team project: Hub 59, 66; Fleet `…/threads?party=me`, `…/messages` | `TeamDesk` |
| `GET /v1/projects/{projectId}/orchestrator` | Hub 71, 68, turn replies (gap G52-2); Fleet `…/team`, `…/threads` | `OrchestratorView` |
| `GET /v1/projects/{projectId}/orchestrator/threads/{threadId}` | Fleet `…/threads/{th}/messages` | `ThreadView` |
| `GET /v1/projects/{projectId}/team/settings` | Hub 74 (data class, route, quota); Fleet `GET /v1/projects/{p}/spend` (task 37, `task-37.md:484`) | `TeamSettings` |
| `POST /v1/projects/{projectId}/orchestrator/messages` | Hub 70, `toAgentId` = the viewer's own Orchestrator | `201 Created` |
| `POST /v1/team/escalations/{escalationId}/answer` — task 21's route (step 6), widened here, not registered again | Hub 65 | `200 DecideItem` answered; a second answer is the Hub's `409 conflict` (`task-01.md:436`), relayed |
| `POST /v1/team/decided/{escalationId}/undo` | Hub 70, the fixed Undo template | `201 Created`; `409 not_reversible` |
| `POST /v1/projects/{projectId}/notes` | Hub 72 (task 46) | `201 Created` |
| `POST /v1/projects/{projectId}/team/route/raise` | Hub 79, `{"route":"au"}` | `200 TeamSettings` |

The ask route refuses `400 bad_request` for a subject whose `ref` fails its grammar (task 03's
`ValidID` for ids, task 02's `teamtext.ValidPath` for a path), before any Hub call; the Hub then
applies its own 2,000-rune bound and G3 hygiene, and its code is relayed unchanged. The Undo body
is exactly:

```go
// undoText is the only prose this task writes into a record. Every value is
// typed: the conflict id, the resolution kind, the other person's display
// name (cleaned by the Hub), and the globs, each already validated.
const undoTemplate = "Undo: reopen %s (%s with %s on %s). I did not agree to this."
```

#### `api/team.go` (complete public surface)

```go
package api

// TeamFleet is the slice of fleetbe the reads call (fleetbe/team.go).
type TeamFleet interface {
	Team(ctx context.Context, projectID string) (verbs.TeamHuman, error)
	Threads(ctx context.Context, projectID string) ([]server.DeskThread, error)
	ThreadMessages(ctx context.Context, projectID, threadID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Message], error)
	Spend(ctx context.Context, projectID string) (budget.SplitView, error) // task 37's route
}

// TeamHub is the slice of task 08's hubclient.Team this file calls, as the
// signed-in user, plus three methods other tasks add (each marked).
type TeamHub interface {
	Conflicts(ctx context.Context, projectID string, q hubapi.ConflictQuery) (hubapi.Page[hubapi.Conflict], error)
	Escalations(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Escalation], error)
	Directives(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Directive], error)
	HumanMessages(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.HumanMessage], error)
	AnswerEscalation(ctx context.Context, escalationID string, req hubapi.AnswerEscalationRequest) (hubapi.Escalation, error)
	WriteHumanMessage(ctx context.Context, projectID string, req hubapi.CreateHumanMessageRequest) (hubapi.HumanMessage, error)
	TeamPolicy(ctx context.Context, projectID string) (hubapi.TeamPolicy, error)               // route 74, task 43
	RaiseRoute(ctx context.Context, projectID string) (hubapi.TeamPolicy, error)               // route 79, task 43
	WriteNote(ctx context.Context, projectID string, req hubapi.CreateNoteRequest) (hubapi.Note, error) // route 72, task 46
	TurnReplies(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[TurnReply], error) // gap G52-2
}

// TurnReply is the owner's read of a human-origin turn's kept reply (task 26's
// TurnRecord.Reply, task-26.md:584-594). No Hub route serves it yet: gap G52-2.
type TurnReply struct {
	ID        string        `json:"id"` // ot-…
	Exit      string        `json:"exit"`
	Reply     teamtext.Text `json:"reply"`
	Truncated bool          `json:"replyTruncated"`
	At        string        `json:"at"`
}

// Viewer is who is asking, from the Hub session, never from a request.
type Viewer interface {
	Me(ctx context.Context) (userID, displayName string, compliance bool, err error)
}

// TeamReader is Deps.Team; nil unless PANACEA_APP_FLEET_URL is set (as TeamLauncher).
type TeamReader struct {
	Projects HubProjects // task 21 TeamProjects plus the id mapping
	Fleet    TeamFleet
	Hub      TeamHub
	Viewer   Viewer
	Now      func() time.Time
	seen     sync.Map // (hub project, userId) -> row signature and changedAt
}

// HubProjects extends task 21's TeamProjects with the id mapping (gap G52-4).
type HubProjects interface {
	TeamProjects
	HubProject(ctx context.Context, projectID string) (string, error)
}

func (a *API) registerTeam(mux *routes) // the ten routes above that task 21 does not already register (not the answer route); a no-op when Deps.Team is nil
```

`TeamReader.Projects` is typed `HubProjects`; task 21's `TeamProjects` stays as it is. Every Hub
error is relayed with its own status and code (`hubclient.Code`), and every `fleetbe` error by
its sentinel (`task-21.md:479`), as `teamlaunch.go` does.

#### `api/teamassemble.go` (complete surface; every function pure)

The handlers fetch; these assemble. Nothing here does I/O, so every rule of ST.8 is a table test.

```go
package api

// labelFor names a text's author from typed fields only. viewer is the
// signed-in user; names maps u-… to display name; agents maps ag-… to its
// role and owner (from TeamHuman). Unknown agent: "<owner>'s agent (agent)".
//   human, owner == viewer                 -> "You"
//   human, other                           -> "<name>"
//   agent, orchestrator, owner == viewer   -> "Your Orchestrator (agent)"
//   agent, orchestrator, other             -> "<name>'s Orchestrator (agent)"
//   agent, worker, owner == viewer         -> "Your <agentName> (agent)"
//   agent, worker, other                   -> "<name>'s <agentName> (agent)"
func labelFor(t teamtext.Text, viewer string, names map[string]string, agents map[string]verbs.HumanAgent) string

// teamText converts a stored text to ST.8's shape, with its label.
func teamText(t teamtext.Text, viewer string, names map[string]string, agents map[string]verbs.HumanAgent) TeamText

// agentName: "Orchestrator" for one; else the brief section's title cut at
// 40 runes plus " worker"; else "worker". Never the task title (text·A).
func agentName(a verbs.HumanAgent) string

// rowStatus is the person's one dot (§9.1): "working" if any live agent is
// working or starting; else "waiting" if any is waiting; else "idle". The
// WaitingOn it returns is the first waiting agent's, human before lease
// before peer, so "waiting on Priya" wins over a lease wait.
func rowStatus(agents []verbs.HumanAgent, names map[string]string) (string, *WaitingOn)

// stripFrom cuts TeamStrip from one TeamHuman. heldByOthers: a Recent path
// another person's agent has in its Files or covers with a granted or
// contested lease glob (teamtext.Match). changedAt comes from seen: it
// moves only when the row's signature (task id, Recent, Status, WaitingOn)
// changes; the first sight stamps now.
func stripFrom(h verbs.TeamHuman, viewer string, route RouteState, now time.Time, seen *sync.Map) TeamStrip

// detailFrom cuts one person's PersonDetail; ok is false when the person has
// no live agent in h.
func detailFrom(h verbs.TeamHuman, userID, viewer string) (PersonDetail, bool)

// cardFor fills STATE / MEANING / NEXT from typed fields:
//   STATE   "WORKING" | "STARTING" | "IDLE" | "WAITING ON <NAME>" | "WAITING ON A LEASE" | "WAITING ON A THREAD"
//   MEANING "<brief section title> · <tk-…>", or "<tk-…>", or "no task"
//   NEXT    "holds <first glob> until <HH:MM>" for its newest granted lease,
//           "asked for <glob>" for a requested one, else "no lease held"
// HH:MM is Australia/Sydney, the viewer's clock (phase 2 displays local time).
func cardFor(a verbs.HumanAgent, names map[string]string, loc *time.Location) Card

// isDecidedWithoutYou is decision (d): kind inform, fromAgentId null,
// threadId set. Nothing reads the question's text to decide it.
func isDecidedWithoutYou(e hubapi.Escalation) bool

// deskFrom merges one project's reads into the account Desk's four lists.
// horizon: open conflicts (raised|negotiating|accepted|escalated) where one
// side is the viewer's agent. needsYou: decide, Answer == nil. since: inform
// and not decided-without-you, CreatedAt > since; plus answer messages the
// viewer's Orchestrator sent with text, CreatedAt > since. decided: every
// decided-without-you escalation, joined to its conflict by ThreadID;
// undo.available = conflict.State == accepted && resolution in {yield, sequence}.
func deskFrom(in DeskInputs, viewer string, since time.Time) TeamDesk

type DeskInputs struct {
	ProjectID   string
	Human       verbs.TeamHuman
	Conflicts   []hubapi.Conflict
	Escalations []hubapi.Escalation
	Threads     []server.DeskThread
	Sent        []hubapi.Message // answers by the viewer's Orchestrator, one page per open thread
	Names       map[string]string
}

// conversationFrom interleaves the viewer's human messages, human-origin
// turn replies and human-origin directives by time, oldest first, the last 100.
func conversationFrom(hm []hubapi.HumanMessage, replies []TurnReply, dv []hubapi.Directive,
	viewer string, names map[string]string, agents map[string]verbs.HumanAgent) []ConvEntry

// threadRow names a DeskThread for the viewer.
func threadRow(t server.DeskThread, names map[string]string, others map[string]string) ThreadRow
```

`TeamDesk` for more than one team project is the concatenation of each project's `deskFrom`,
then each list sorted newest first by `At` (HORIZON and DECIDED WITHOUT YOU) or deadline soonest
first (NEEDS YOU). The sort is done once per response, and the order is stable for equal keys
(`sort.SliceStable`), so an unchanged Desk never reorders under the cursor
(`docs/AGENTIC-UX.md:146`).

#### ST.8 on the desktop: the team block of `store/types.ts` (complete)

Appended after the `Session` interface, which gains one field. `TextA` and `TextH` are distinct
types, so a component that takes `TextH` (a human's words) cannot be handed an agent's line
without a compile error, and every component that renders agent text must say so in its props.

```ts
/* ── team mode (ST.8, Phase T task 52) ─────────────────────────────────────────────── */

export type TeamScope = "" | "team" | "outside";
// Session gains:  /** task 21: "" solo, "team" a Fleet session, "outside" local on a team project. */
//                 teamScope: TeamScope;

export type HubUserId = string;   // "u-…"
export type AgentId = string;     // "ag-…"

interface TeamTextBase {
  body: string;
  ownerUserId: HubUserId;
  at: string;
  /** Assembled from typed fields by the control plane. Render beside the body, always. */
  label: string;
}
/** An agent's words (text·A). Plain text only: no markdown, no HTML, no terminal. */
export interface TextA extends TeamTextBase { author: "agent"; authorAgentId: AgentId; }
/** A human's words (text·H). */
export interface TextH extends TeamTextBase { author: "human"; authorAgentId: null; }
export type TeamText = TextA | TextH;

export interface PersonRef { userId: HubUserId; displayName: string; }
export interface AgentRef { agentId: AgentId; name: string; }
export interface BriefSection { ref: string; title: string; }
export interface RouteState {
  current: "global" | "au"; required: boolean; raisedAt: string | null; canLower: boolean;
}

export type RowStatus = "working" | "waiting" | "idle";
export type WaitingOn =
  | { kind: "human"; name: string; words: TextA | null }
  | { kind: "lease" | "peer"; name: null; words: null };

export interface PersonRow {
  userId: HubUserId; displayName: string; you: boolean;
  status: RowStatus; waitingOn: WaitingOn | null;
  line: { briefSection: BriefSection | null; taskTitle: TeamText | null; agents: number };
  files: { recent: string[]; more: number; heldByOthers: string[] };
  changedAt: string;
}
export interface TeamStrip {
  v: 1; projectId: string; team: boolean; generatedAt: string;
  route: RouteState; people: PersonRow[];
}

export interface AgentCard { state: string; meaning: string; next: string; }
export interface PersonDetail {
  v: 1; projectId: string; userId: HubUserId; displayName: string; you: boolean;
  tasks: { id: string; state: string; title: TeamText | null; briefSection: BriefSection | null }[];
  agents: (AgentRef & { role: "orchestrator" | "worker"; harness: string; taskId: string;
                        status: string; card: AgentCard })[];
  files: { path: string; lastEdit: string; heldByOthers: boolean }[];
  leases: { id: string; agentId: AgentId; globs: string[]; mode: "intent" | "hold" | "bulk";
            state: string; expiresAt: string }[];
  conflicts: { id: string; tier: "P" | "H" | "M" | "S"; state: string; paths: string[]; withMine: string }[];
}

export interface HorizonItem {
  conflictId: string; projectId: string; tier: "P" | "H" | "M" | "S";
  state: "raised" | "negotiating" | "accepted" | "escalated"; paths: string[];
  mine: AgentRef; theirs: PersonRef & { agentId: AgentId }; at: string;
}
export interface DecideItem {
  escalationId: string; projectId: string; threadId: string | null; fromAgentId: AgentId | null;
  question: TextA; options: { label: TextA; consequence: TextA }[];
  default: number; deadline: string; at: string;
}
export type SinceItem =
  | { kind: "inform"; projectId: string; at: string; escalationId: string; question: TextA }
  | { kind: "answer"; projectId: string; at: string; messageId: string; threadId: string;
      to: PersonRef; text: TextA };
export type Resolution = "split" | "sequence" | "yield" | "build-on" | "proceed-both";
export interface DecidedItem {
  escalationId: string; conflictId: string; projectId: string; resolution: Resolution;
  other: PersonRef; globs: string[]; at: string; undo: { available: boolean };
  /** Fleet's own sentence from typed fields (task-29.md:278-296), never an agent's. */
  summary: string;
}
export interface TeamDesk {
  v: 1; generatedAt: string; horizon: HorizonItem[]; needsYou: DecideItem[];
  sinceYouLastLooked: { since: string; items: SinceItem[] }; decidedWithoutYou: DecidedItem[];
}
```

```ts
export type ConvEntry =
  | { kind: "human"; at: string; id: string; text: TextH }
  | { kind: "reply"; at: string; turnId: string; text: TextA;
      exit: "ok" | "failed" | "timeout" | "lost"; truncated?: boolean }
  | { kind: "directive"; at: string; id: string;
      directiveKind: "pause" | "resume" | "narrow" | "wait-for" | "rebase" | "free";
      to: AgentRef; params: { globs: string[]; leaseId: string | null }; text?: TextA };
export interface ThreadRow {
  id: string; state: "open" | "escalated"; other: PersonRef;
  subject: { kind: "conflict" | "lease" | "free"; id: string | null };
  subjectLabel: string; round: number; maxRounds: number; deadline: string;
}
export interface OrchestratorView {
  v: 1; projectId: string;
  orchestrator: { agentId: AgentId; status: string; canDive: boolean } | null;
  conversation: ConvEntry[]; threads: ThreadRow[];
}
export type MessageKind =
  "ask" | "answer" | "inform" | "propose" | "accept" | "reject" | "counter" | "withdraw";
export interface ResolutionProposal {
  kind: Resolution; sides: Record<AgentId, string[]>; first: AgentId | null;
  globs: string[]; base: string | null; paths: string[];
}
export interface ThreadMessage {
  id: string; kind: MessageKind; templated: boolean; at: string; tombstoned: boolean;
  sender: { agentId: AgentId; userId: HubUserId; label: string };
  /** The Hub's flat union, verbatim (task 03 Block 5): every key present. */
  fields: {
    subject: { kind: "task" | "lease" | "path" | "conflict"; ref: string } | null;
    inReplyTo: string | null; resolution: ResolutionProposal | null; proposal: string | null;
  };
  text: TextA | null;
}
export interface ThreadView { thread: ThreadRow; messages: ThreadMessage[]; }

/** dataClass is phase 6's DataClass, written structurally so both parts do not declare it. */
export interface TeamSettings {
  v: 1; projectId: string; route: RouteState;
  dataClass: { level: string; residency: "any" | "au-only" } | null;
  /** UTC day. null figures mean "not known", never 0 (task 37). */
  spend: { day: string; workersCents: number | null; coordinationCents: number | null;
           quotaCents: number | null };
}

export interface AskSubject { kind: "task" | "path" | "agent" | "conflict"; ref: string; }
export interface Created { id: string; }
```

Every `ConvEntry` of kind `human` carries a `TextH` and every `reply` a `TextA`; the key set of
each kind is exactly the Go struct's non-omitted keys for that kind (Step 1's fixture test checks
it both ways).

#### `desktop/src/api/team.ts` (complete surface)

A second client beside `createClient`, built on the same `send`, timeout and `Problem` handling,
so the team routes fail with the same `ProblemCode`s the cockpit already renders
(`client.ts:40-50`). It is not merged into `CockpitClient`, so phase 2's interface does not move.

```ts
import type { ClientOptions, Problem } from "./client";
import type { AskSubject, Created, DecideItem, OrchestratorView, PersonDetail, ProjectId,
  TeamDesk, TeamSettings, TeamStrip, ThreadView } from "../store/types";

export interface TeamClient {
  strip(projectId: ProjectId): Promise<TeamStrip>;
  person(projectId: ProjectId, userId: string): Promise<PersonDetail>;
  /** since: RFC 3339, the persisted teamSeenAt. */
  desk(since: string): Promise<TeamDesk>;
  orchestrator(projectId: ProjectId): Promise<OrchestratorView>;
  thread(projectId: ProjectId, threadId: string): Promise<ThreadView>;
  settings(projectId: ProjectId): Promise<TeamSettings>;
  ask(projectId: ProjectId, text: string, subject?: AskSubject): Promise<Created>;
  /** Once per click: the caller disables the control until it settles (task 54). */
  answer(escalationId: string, choice: number, text?: string): Promise<DecideItem>;
  undo(escalationId: string): Promise<Created>;
  note(projectId: ProjectId, target: { kind: "project" | "task" | "user"; id: string },
       text: string): Promise<Created>;
  /** Hub route 79, {"route":"au"}: the team ratchet (task 56). */
  raiseTeamRoute(projectId: ProjectId): Promise<TeamSettings>;
}

export function createTeamClient(opts?: ClientOptions): TeamClient;
export type { Problem };
```

Path segments are `encodeURIComponent`-ed; no method builds a query string by hand except
`desk`, which sends `?since=` through `URLSearchParams`.

`send` is today a closure inside `createClient` (`client.ts:245`). This task lifts it, unchanged,
into an exported `createTransport(opts): { baseUrl; send }` in the same file, which `createClient`
calls first; phase 2's `client.test.ts` passes untouched, which is the proof that nothing moved.
`client.ts` therefore joins the modified files.

#### `desktop/src/store/slices/teamSlice.ts` (complete surface)

```ts
import type { StateCreator } from "zustand";
import type { CockpitState } from "../store";
import type { OrchestratorView, PersonDetail, ProjectId, TeamDesk, TeamSettings, TeamStrip,
  ThreadView, AskSubject } from "../types";

export interface TeamSlice {
  teamStrips: Record<ProjectId, TeamStrip>;
  teamPeople: Record<`${ProjectId}:${string}`, PersonDetail>;
  teamDesk: TeamDesk | null;
  orchestrators: Record<ProjectId, OrchestratorView>;
  threads: Record<`${ProjectId}:${string}`, ThreadView>;
  teamSettings: Record<ProjectId, TeamSettings>;
  /** Persisted. The SINCE YOU LAST LOOKED cursor (decision e). "" = never looked. */
  teamSeenAt: string;
  /** The Orchestrator panel's open state and attached subject (task 55 renders it). */
  orchestratorPanel: { projectId: ProjectId; subject: AskSubject | null } | null;
  /** Answers in flight, by escalation id: the one-call guard task 54 tests. */
  answering: Record<string, true>;

  setTeamStrip(s: TeamStrip): void;
  setTeamPerson(d: PersonDetail): void;
  setTeamDesk(d: TeamDesk): void;
  setOrchestrator(v: OrchestratorView): void;
  setThread(projectId: ProjectId, v: ThreadView): void;
  setTeamSettings(s: TeamSettings): void;
  markTeamSeen(at: string): void;
  openOrchestrator(projectId: ProjectId, subject?: AskSubject): void;
  closeOrchestrator(): void;
  beginAnswer(escalationId: string): boolean; // false if already in flight
  endAnswer(escalationId: string): void;
}

export const initialTeamState: Pick<TeamSlice, "teamStrips" | "teamPeople" | "teamDesk" |
  "orchestrators" | "threads" | "teamSettings" | "teamSeenAt" | "orchestratorPanel" | "answering">;

export const createTeamSlice: StateCreator<CockpitState, [], [], TeamSlice>;
```

`store.ts` adds `TeamSlice` to `CockpitState` and `teamSeenAt` to `Persisted` and `partialize`. The
root-level shallow merge (`store.ts:1-5`) is why a snapshot without the key rehydrates to `""`
safely, the same argument `store.ts` already makes for `sendChordsToTerminal`; no version bump.
Setters **replace** a view whole; nothing merges a new strip into an old one, so a person who left
the project cannot linger in the store. Polling lives in the components that show a view (task
53's strip polls `strip` every 5 s while mounted, with phase 2's `startPolling` pattern,
`poller.ts:30`); the slice holds no timer.

- [ ] **Step 1: Write the failing tests first**

**Fleet side** (`teamhuman_test.go`, `desk_test.go`), over task 22's fakes for `Sources` and task
12's test server:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHumanKeysAllowlist` | reflection over `TeamHuman`, `HumanAgent`, `HumanFile`, `HumanConflict` JSON tags equals a written list; none of `transcript`, `terminal`, `openingPrompt`, `prompt`, `args`, `reply`, `directive`, `text`, `why`, `lastCommitSubject` | a later field that ships a teammate's words or prompt to every desktop (§9.2) |
| `TestHumanWordsOnlyWhenWaitingOnHuman` | a worker `working` with an open blocked report: `words` absent; `waiting`/`human`: present, class `text·A`, `by` = the worker | a stale report shown under a working agent |
| `TestHumanOmitsEnded` | an `ended` agent is absent; a `starting` one is present | a finished agent kept on the strip |
| `TestTeamRouteIsViewerAndUp` | viewer 200; a non-member 403 `forbidden` | the route registered under `VerbPrivateRead` or with no verb |
| `TestThreadsOnlyTheCallersOwn` | Alice's call lists only threads her Orchestrator is party to; Bob's thread with Carol is absent | listing every open thread in the project |
| `TestMessagesHidesExistence` | Alice asks for Bob–Carol's thread → 404 `thread_not_found`, and `Messages` was never called | a 403 that confirms the thread exists, or a relay that trusts the Hub alone |
| `TestMessagesUseTheCallersOrchDelegation` | the fake `Messages` receives Alice's Orchestrator's `dl-…`, never Fleet's own or Bob's | reading route 63 under a delegation that is not the caller's |

**Local side** (`teamassemble_test.go` pure; `team_test.go` over `httptest`, a fake `TeamFleet`, a
fake `TeamHub` recording every call, and a fixed `Viewer` = Dylan):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestST8KeysAllowlist` | reflection over every ST.8 type's JSON tags equals the written list; no `transcript`, `terminal`, `openingPrompt`, `prompt`, `args`, `attachPath`, `delegation`, `token` anywhere | a view that grows a teammate's terminal or a secret |
| `TestStripNeverCarriesTeammateWords` | a `TeamHuman` whose fake also holds a directive text, a reply and a message body (planted strings `DV-PLANT`, `RP-PLANT`, `MS-PLANT`); the marshalled strip and every person detail contain none of them | the strip built from the Orchestrator view instead of `TeamHuman` |
| `TestLabelTable` | each row of `labelFor`'s six cases, and an unknown agent | a label that quotes a body, or calls a teammate's agent "Your" |
| `TestEveryTextHasAClass` | walk every `TeamText` in all six fixtures: `author` ∈ {agent, human}, `authorAgentId` non-null iff agent, `label` non-empty | a text that reaches the cockpit without its class (§3.3 rule 5) |
| `TestRowStatusTable` | working beats waiting beats idle; `human` before `lease`; a `starting` agent counts as working | a row that says idle while one agent works, or guesses `waiting on <name>` from a lease wait |
| `TestHeldByOthersIsARing` | Alice's `auth/session.go` is in Bob's `Files` → in Alice's `heldByOthers`; a path only Alice has is not; a path covered by Bob's `granted` glob `auth/**` is | `heldByOthers` computed from the same person's own leases |
| `TestChangedAtMovesOnlyOnChange` | two identical `TeamHuman`s 5 s apart: `changedAt` equal; a new first path: moved | a flash on every poll |
| `TestCardIsTemplated` | `cardFor` for each STATE row; the NEXT line's time is `04:40` in Australia/Sydney for `18:40Z` | a card that prints an agent sentence, or UTC |
| `TestHorizonKeepsAccepted` | a conflict `accepted` stays in `horizon`; `resolved` and `cleared` leave | dropping HORIZON on `accepted` (§9.3) |
| `TestHorizonHasNoTime` | `HorizonItem` has no field whose tag contains `eta`, `minutes`, `until` or `expected` | OD-27's banned ETA reintroduced |
| `TestDecidedIsTypedNotTextual` | an inform whose question starts *"DECIDED WITHOUT YOU:"* but has `fromAgentId` set lands in `since`; one with `fromAgentId` null and `threadId` set lands in `decided`, whatever its text | classifying by the text prefix |
| `TestUndoAvailability` | accepted+yield → true; accepted+sequence → true; accepted+proceed-both → false; resolved+yield → false | an Undo that tries to reverse a landed lane |
| `TestUndoWritesOneOwnMessage` | `POST …/undo` calls `WriteHumanMessage` once, `toAgentId` = Dylan's Orchestrator, body = `undoTemplate` filled from typed fields; no other Hub write | an Undo that patches the conflict or messages the other side (OD-16) |
| `TestAnswerCallsHubOnce` | `POST …/answer` calls `AnswerEscalation` once with `{choice, text}`; a Hub `409` is relayed as `409` with the Hub's code | a retry loop that double-answers |
| `TestAskTakesNoOwner` | a body carrying `ownerUserId`, `toAgentId` or `userId` is `400 bad_request` (strict decode); the Hub call's `toAgentId` is the viewer's own Orchestrator from `TeamHuman` | an ask addressed to a teammate's Orchestrator |
| `TestAskSubjectGrammar` | `{kind:"path", ref:"../etc/passwd"}` → 400 before any Hub call; `{kind:"task", ref:"tk-…"}` → the first line `About task tk-…:` | a subject line that carries unvalidated text into a record |
| `TestSoloProjectIsEmpty` | a solo project: strip `team:false, people:[]`, desk contributes nothing, `ask` → `409 not_team_project`, and no Fleet call was made | a Fleet call on every solo project's poll |
| `TestFixturesRoundTrip` | each `testdata/team-view/v1/*.json` decodes strictly into its Go type and re-marshals byte-equal (after `json.Indent`) | a fixture drifted from the type |

**Desktop side** (Vitest):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `team.test.ts › fixtures satisfy the types` | each fixture, imported as a JSON module from `control-plane/internal/api/testdata/team-view/v1/` (`resolveJsonModule`, `desktop/tsconfig.json:12`; `node:fs` does not typecheck here, `styles/designsystem.test.ts:9-11`), is assigned to its type through a `satisfies` check in a `.ts` helper that `npm run typecheck` compiles | a TS field renamed without the Go one (`types.ts:3-5`) |
| `team.test.ts › routes and methods` | each client method's URL and method against an injected `fetch`; `userId` and `threadId` are URL-encoded | a hand-built path that breaks on `u-…/../` |
| `team.test.ts › desk sends since` | `desk("2026-09-24T22:41:00.000Z")` requests `/v1/team/desk?since=2026-09-24T22%3A41%3A00.000Z` | a cursor that is never sent, so everything is new |
| `teamSlice.test.ts › setters replace` | `setTeamStrip` with one person after three leaves one | a merge that keeps people who left |
| `teamSlice.test.ts › one answer in flight` | `beginAnswer("es-1")` true, again false, after `endAnswer` true | a double click that answers twice |
| `store.test.ts › teamSeenAt persists` | `partialize` includes `teamSeenAt` and nothing else new; a snapshot without it rehydrates to `""` | a cursor lost on restart, or the whole team view persisted |

```bash
cd control-plane && go test ./internal/api/... ./internal/fleet/verbs/... ./internal/fleet/server/... 2>&1 | head -5
cd desktop && npm test -- team 2>&1 | tail -5
```

Expected: `undefined: verbs.TeamHuman` and similar; Vitest fails to resolve `./team`. That is the red.

- [ ] **Step 2: The Fleet side**

`teamhuman.go` as above; `BuildHuman` reads `s.Store` for live agents of the project, then per
agent `Cards`, `Briefs`, `People`, `Activity` (nil reads as no files, TM1-α's state), leases from
`s.Store`, and `Reports` only for a waiting-on-human worker. `desk.go` registers the three routes
from task 12's `New` (one call, `s.RegisterDesk(deps)`). Both run under `go test` with task 22's
fakes; no Postgres.

- [ ] **Step 3: `fleetbe/team.go`**

Three methods on task 21's `do` helper. The thread-message call passes `after` and `limit`
through as the Hub's `PageRequest` query, never re-paging.

- [ ] **Step 4: `teamview.go`, `teamassemble.go`, then `team.go`**

Types first, then the pure functions until `teamassemble_test.go` is green, then the handlers.
Each handler: `Viewer.Me`; `Projects.IsTeam` (solo: the empty body, no Fleet call);
`Projects.HubProject`; the reads (in parallel with `errgroup` for the Desk, one goroutine per team
project, bounded at 4); assemble; write. The person route reuses the strip's `TeamHuman` if it is
under 2 s old (one cached value per project, keyed by the Hub project id), so a click-in right
after a poll makes no Fleet call. `api.go` gains one line beside `a.registerAttach(guarded)`:
`a.registerTeam(guarded)`.

Write the six fixtures by running the assembly over the test inputs once and **reading** each
output before committing it; the fixture is then the contract, and task 51's sample must match its
keys (a key present in one and not the other is a review finding for both tasks).

- [ ] **Step 5: The desktop client, types and slice**

`types.ts` team block and `Session.teamScope`; `createTransport` lifted in `client.ts`; `team.ts`;
`teamSlice.ts`; `store.ts`. The fixtures helper `desktop/src/test/teamFixtures.ts` imports each JSON by relative path
and checks it with `satisfies` so `npm run typecheck` fails on a drifted field.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./... && go test ./internal/api/... ./internal/fleet/verbs/... ./internal/fleet/server/... ./internal/backend/fleetbe/... -count=1
cd desktop && npm run typecheck && npm test
```

Expected: `ok` for the four Go packages, and every test named above under `-v`; `tsc` prints
nothing; Vitest reports all files passed, phase 2's `client.test.ts` and `store.test.ts` among
them unchanged in count. Then the leak guard, which must print nothing:

```bash
cd control-plane && grep -n "Transcript\|OpeningPrompt\|AttachToken\|Terminal" internal/api/teamview.go internal/fleet/verbs/teamhuman.go
```

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/api/team.go control-plane/internal/api/teamview.go \
  control-plane/internal/api/teamassemble.go control-plane/internal/api/team_test.go \
  control-plane/internal/api/teamassemble_test.go control-plane/internal/api/testdata/team-view \
  control-plane/internal/fleet/verbs/teamhuman.go control-plane/internal/fleet/verbs/teamhuman_test.go \
  control-plane/internal/fleet/server/desk.go control-plane/internal/fleet/server/desk_test.go \
  control-plane/internal/backend/fleetbe/team.go desktop/src/api/team.ts desktop/src/api/team.test.ts \
  desktop/src/store/slices/teamSlice.ts desktop/src/store/slices/teamSlice.test.ts desktop/src/test/teamFixtures.ts
git commit -F <msgfile> -- control-plane/internal/api control-plane/internal/fleet/verbs/teamhuman.go \
  control-plane/internal/fleet/verbs/teamhuman_test.go control-plane/internal/fleet/server/desk.go \
  control-plane/internal/fleet/server/desk_test.go control-plane/internal/backend/fleetbe/team.go \
  desktop/src/api desktop/src/store desktop/src/test/teamFixtures.ts
```

**What this task does NOT claim.**
- That any of it runs against a real Fleet or Hub. Every test uses fakes; task 57 is the first
  time two real people's desktops read these routes.
- That activity is present. Until task 35 lands, `Sources.Activity` is nil and every `files` list
  is empty; the strip then shows the line and status only. Task 53 renders that state correctly,
  and nothing here pretends otherwise.
- That the read cursor follows a user across devices (decision e).
- That the Hub serves turn replies, desk reads of task titles, or a local-to-Hub project id. Each is
  a gap below; until closed, the matching field is `null` or the route answers `501
  not_implemented`, and the fixtures show both states.
- Rendering. Tasks 53–56 own every pixel; this task ships no component.

**Gaps (for the skeleton's owner).**
- **G52-1.** Task 01 §7.14 (`task-01.md:356-357`) returns a task's `title` only to Fleet·O, but
  §9.1 shows it on the strip to humans and the scope of task 52 names it in the assembled line.
  Fleet serves it here from its own `Card` to a desktop under `VerbTeamRead`. Either task 01 admits
  `desk` readers (recommended: "Fleet·O and members' desktops; never Fleet·W"), or `HumanAgent.Title`
  is dropped and the line is the brief section and the agent count only. Owner call.
- **G52-2.** No Hub route reads a turn's kept reply (task 26's *"readable by the owner and an org
  auditor"*, `task-26.md:678-683`, names no route). Proposed: route 80, `GET
  /v1/projects/{projectId}/turns?origin=human`, own only, `turns.read` never granted to Fleet·W.
  Until then `conversation` holds human messages and directives, and no replies.
- **G52-3.** The file map lacks `cp/internal/fleet/server/desk.go`, `verbs/teamhuman.go` and
  `fleetbe/team.go`; the three are required because only Fleet holds activity, presence and open
  thread state.
- **G52-4.** No task maps the cockpit's project id (`p-…`, `types.ts:10`) to the Hub's `pj-…`
  (since b01, task 21 resolves it through Hub task 35's S-LINK `Resolver`, `task-21.md:442-452`).
- **G52-5.** Task 12's `Member.UserID` comment says `us-…` (`task-12.md:359`) where the Hub's
  prefix is `u` (`.claude/worktrees/hub/control-plane/internal/hub/hubapi/keys.go:18`, checked).
- **G52-6.** `TeamSettings.dataClass` is phase 6's `{level, residency}` (task 56's request,
  `task-56.md:171-173`), but the Hub's team policy holds `dataClass` as a bare string (task 03,
  grammar only). `residency` then needs task 43's class table, **UNVERIFIED** until task 43 is
  written; until then `dataClass` is `null` and `route.required` is false.
