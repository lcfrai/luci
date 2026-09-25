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
   `VerbTeamRead` (`task-12.md:272`, *"the strip and task cards"*; viewer and up). Its body,
   `verbs.TeamHuman`, is the third sibling of task 22's `TeamWorker` and `TeamOrchestrator`, built
   from the **same** `verbs.Sources` (`task-22.md:77-88`; task 22 lists 52 under "Required by",
   `task-22.md:60`), so the three views cannot disagree about who holds what.
2. **Two Fleet routes for the caller's own threads**, under `VerbThreadRead` (own only,
   `task-12.md:299`): the open-thread list as typed state, and one thread's messages. Fleet reads
   them from the Hub's route 63 as Fleet·O under the caller's own Orchestrator delegation, the
   only principal route 63 admits for a participant's owner (`task-01.md:268`), and relays them.
3. **The local routes** in `control-plane/internal/api/team.go`, which read those three Fleet
   routes through `fleetbe` and the Hub's own-only routes (66, 68, 71, 74) through task 08's
   `hubclient.Team` as the signed-in user, and **assemble** ST.8: the strip, a person's detail,
   the Desk's four row kinds, the Orchestrator conversation, a thread, and the project's team
   settings. Six actions write, each to exactly one Hub route, as the user.
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
team policy (route 74, `task-01.md:279`) and phase 6's inference panel state, never from which
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
  (modify, one anchored hunk beside the sessions registration)
- `desktop/src/store/types.ts` — the team block below, and `Session.teamScope` (task 21's field)
  (modify)
- `desktop/src/api/team.ts`, `desktop/src/api/team.test.ts` (new)
- `desktop/src/store/slices/teamSlice.ts`, `desktop/src/store/slices/teamSlice.test.ts` (new)
- `desktop/src/store/store.ts` — the fourth slice; `teamSeenAt` joins `Persisted` (modify)

**Interfaces:**
- Consumes: task 22's `verbs.Sources`, `BriefRef`, `LeaseView`, `Quoted` (`task-22.md:77-246`);
  task 24's `Report` (`task-24.md:117-125`) through a new `Sources.Reports` reader; task 12's
  `Server.Route`, `Principal`, `VerbTeamRead`, `VerbThreadRead` (`task-12.md:269-328`, `:522`);
  task 21's `fleetbe.Backend` and `TeamProjects` (`task-21.md:285-306`); task 08's `hubclient.Team`
  reads and writes (`task-08.md:79-115`); task 03's record types (`task-03.md:470-945`); task 02's
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
verb table (`task-12.md:296-299`) decides who may call. None takes a body; none reads an owner from
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
	Rounds       int                  `json:"rounds"`    // rounds used so far, both sides
	MaxRounds    int                  `json:"maxRounds"` // 2 × team policy's roundsPerSide
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
// (task-21.md:415-421). projectID is the Hub's pj-… id.
func (b *Backend) Team(ctx context.Context, projectID string) (verbs.TeamHuman, error)
func (b *Backend) Threads(ctx context.Context, projectID string) ([]server.DeskThread, error)
func (b *Backend) ThreadMessages(ctx context.Context, projectID, threadID string,
	page hubapi.PageRequest) (hubapi.Page[hubapi.Message], error)
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
