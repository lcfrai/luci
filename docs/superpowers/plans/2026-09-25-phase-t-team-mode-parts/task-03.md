### Task 03: Hub wire types for team records; id prefixes and codes

**Slice:** TM1-α. **Size:** 2.5 focused hours. **dependsOn:** 01 (the contract text), 02
(`teamtext`), Hub tasks 01–03 (`hubapi`, built). **Seam defined:** ST.2, in Go.

Task 01 wrote the team records into the Hub's contract. This task writes them into
`control-plane/internal/hub/hubapi`, the package every Hub caller already speaks (the Hub's
server, `hubclient`, and from now on Fleet), so that no route, store or client re-declares a team
field. It adds no route and no table; tasks 04 to 08 do.

`hubapi` is **built** on the Hub branch (Hub tasks 01–04: `types.go` 585 lines, `codes.go` 67,
`keys.go` 246, `envelope.go` 400, read at `.claude/worktrees/hub/control-plane/internal/hub/hubapi/`
on 2026-09-25). This task starts only after `feat/hub` is sync-merged into `feat/team-mode`
(Global constraints, "Where the code is built"), and extends those files where they stand.

Five decisions, each forced by what `hubapi` already does.

**(a) The package's own response and request rules apply unchanged** (`hubapi/types.go:3-15`). A
response never omits a field: optional values are pointers and are `null`; lists are `[]` through
`EnsureLists`; so no response field carries `omitempty`. A request's optional fields are pointers
with `omitempty`, and strict decoding refuses unknown keys. Consequence for task 01's examples:
`threadId`, `resolution`, `revokedReason`, `exitClass` and `renewedAt` are **`null`** while unset,
not `""`. The keys are the contract's; this task fixes their empty value. And a task read by anyone
but Fleet·O carries `"title": null` and `"reason": {"briefRef": …, "why": null}`, not a record with
the keys missing.

**(b) Type names are the ones task 06 already consumes** (`task-06.md:50-54`): `Delegation`,
`CreateDelegationRequest`, `Task`, `CreateTaskRequest`, `PatchTaskRequest`, `AgentStart`,
`AgentStartRequest`, `AgentEndRequest`, `LeaseEvent`, `LeaseEventRequest`, `Conflict`,
`CreateConflictRequest`, `PatchConflictRequest`, `AgentRole`, `AgentRoleOrchestrator`. The rest
follow the same pattern: record `X`, request `CreateXRequest` / `PatchXRequest` /
`AppendXRequest`.

**(c) Three of `hubapi`'s tests read the contract, and this task moves their numbers.**
`TestPrefixesAndReasonsAreTheContracts` wants exactly ten `Prefix` constants and ten §3.1 entities
and maps each constant through `prefixEntity` (`keys_test.go:737-775`);
`TestHubCodesAreContractSection42s` wants 5 shared and 33 own codes, in the table's order, with each
comment's `details.x` in the row's `details` cell (`keys_test.go:530-625`). After task 01 the
contract has 22 prefixes and 48 own codes, so Step 5 changes `10` to `22` and `33` to `48` and adds
twelve `prefixEntity` entries. These are the only edits to an existing test.

**(d) The team types are pinned to the contract by key name, not by parsing its bodies.** The
existing `TestEveryBodyHasTheContractsKeys` parses fenced JSON with values; several of task 01's
team bodies are inline key lists (`{"seq", "holderAgentId", …}`), which its `jsoncKeyPaths` reads
as no keys. So a new test checks the direction that catches drift in Go: **every JSON key of a team
type appears as `"key"` in its contract section**. The other direction, every design field in the
contract, is task 01's field check. Both together pin the three layers.

**(e) The audit event gains a version 2, and version 1 does not move by one byte.** Task 01's
§5.10 rule 4 records a delegated write as `v: 2` with `via` and `delegationId`.
`AuditEnvelope` (`envelope.go:264-331`) gains two fields; `Canonical` writes `v: 1` and the eleven
keys exactly as now when `Via` is empty, and `v: 2` with thirteen keys when it is not.
`ParseAuditEnvelope` reads both. The existing golden vectors (`canonical_test.go`, contract §3.5)
stand; one v2 vector is added.

**Files** (all under `control-plane/internal/hub/hubapi/`, after the Hub's merge):
- `team.go` — every enum with `Valid()`, the route names and class sets, the header and role
  constants, every record and request of contract §7.13–§7.26, four grammar helpers (new)
- `team_test.go` — enums, goldens, strict decoding, never-omit, contract keys, class sets,
  grammars, the time-layout pin (new)
- `keys.go` — twelve `Prefix` constants appended to the const block (modified)
- `codes.go` — fifteen `Code` constants appended, in §4.2's order (modified)
- `envelope.go` — `AuditEnvelope.Via`, `.DelegationID`; `Canonical` and `ParseAuditEnvelope` for v2
  (modified)
- `envelope_test.go` — the v2 golden vector and its three refusals (modified)
- `keys_test.go` — `10` → `22`, `33` → `48`, twelve `prefixEntity` rows (modified)

**Interfaces:**
- Produces: everything in the `team.go` blocks below, verbatim; the twelve prefixes; the fifteen
  codes; `AuditEnvelope.Via`, `AuditEnvelope.DelegationID`, `ViaFleet`.
- Consumes: `teamtext.Text` (task 02); `hubapi.Digest`, `ValidID`, `ValidBranchRef`,
  `ParseBriefKey`, `FormatTime` (Hub tasks 01–03, built).
- Required by: tasks 04 (column names mirror the JSON names), 05 (`HeaderDelegation`,
  `AppRoleFleet`, `RouteName`, `ClassRoutes`), 06 and 07 (every request and record), 08
  (`hubclient.Team` signatures), 11 (the outbox's payloads), 43, 46, 52.

`team.go` is the concatenation of blocks 1 to 5, in order. **Block 1, enums:**

```go
package hubapi

import (
	"regexp"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/teamtext"
)

// Team records (contract §7.13–§7.26, Phase T task 01). The file's rules are
// types.go's: responses never omit a field (optional values are pointers and
// null, lists []), requests use pointers with omitempty for optional fields,
// and strict decoding refuses unknown keys, which is what keeps an owner, an
// actor, a via or a delegation out of every body (ADR-H1 §5.10 rule 4).

// ── enums ───────────────────────────────────────────────────────────────────

type DelegationClass string

const (
	DelegationClassOrchestrator DelegationClass = "orchestrator"
	DelegationClassWorker       DelegationClass = "worker"
)

func (v DelegationClass) Valid() bool {
	return v == DelegationClassOrchestrator || v == DelegationClassWorker
}

type RevokeReason string

const (
	RevokeReasonOwner           RevokeReason = "owner"
	RevokeReasonAdmin           RevokeReason = "admin"
	RevokeReasonMemberRemoved   RevokeReason = "member_removed"
	RevokeReasonAccountDisabled RevokeReason = "account_disabled"
)

func (v RevokeReason) Valid() bool {
	switch v {
	case RevokeReasonOwner, RevokeReasonAdmin, RevokeReasonMemberRemoved, RevokeReasonAccountDisabled:
		return true
	}
	return false
}

type TaskState string

const (
	TaskPlanned   TaskState = "planned"
	TaskActive    TaskState = "active"
	TaskBlocked   TaskState = "blocked"
	TaskDone      TaskState = "done"
	TaskAbandoned TaskState = "abandoned"
)

func (v TaskState) Valid() bool {
	switch v {
	case TaskPlanned, TaskActive, TaskBlocked, TaskDone, TaskAbandoned:
		return true
	}
	return false
}

// Terminal reports a state no PATCH may leave (409 team_state).
func (v TaskState) Terminal() bool { return v == TaskDone || v == TaskAbandoned }

type AgentRole string

const (
	AgentRoleOrchestrator AgentRole = "orchestrator"
	AgentRoleWorker       AgentRole = "worker"
)

func (v AgentRole) Valid() bool { return v == AgentRoleOrchestrator || v == AgentRoleWorker }

type ExitClass string

const (
	ExitDone          ExitClass = "done"
	ExitStopped       ExitClass = "stopped"
	ExitFailed        ExitClass = "failed"
	ExitAccessRemoved ExitClass = "access_removed"
	ExitLapsed        ExitClass = "lapsed"
)

func (v ExitClass) Valid() bool {
	switch v {
	case ExitDone, ExitStopped, ExitFailed, ExitAccessRemoved, ExitLapsed:
		return true
	}
	return false
}
```

**Block 2, the rest of the enums.** Each is the same shape, so it is written compactly: a type, its
constants, and a `Valid` over a set built once.

```go
type (
	LeaseMode          string
	LeaseState         string
	ConflictTier       string
	ConflictState      string
	ResolutionKind     string
	ThreadState        string
	SubjectKind        string
	MessageSubjectKind string
	MessageKind        string
	EscalationKind     string
	DirectiveKind      string
	DeliveryRung       string
	TurnOrigin         string
	NoteTargetKind     string
	InferenceRoute     string
	EscalationFloor    string
)

const (
	LeaseIntent LeaseMode = "intent"
	LeaseHold   LeaseMode = "hold"
	LeaseBulk   LeaseMode = "bulk"

	LeaseRequested     LeaseState = "requested"
	LeaseGranted       LeaseState = "granted"
	LeaseContested     LeaseState = "contested"
	LeaseYielded       LeaseState = "yielded"
	LeaseReleased      LeaseState = "released"
	LeaseExpired       LeaseState = "expired"
	LeaseAdminReleased LeaseState = "admin-released"

	TierP ConflictTier = "P"
	TierH ConflictTier = "H"
	TierM ConflictTier = "M"
	TierS ConflictTier = "S"

	ConflictRaised      ConflictState = "raised"
	ConflictNegotiating ConflictState = "negotiating"
	ConflictAccepted    ConflictState = "accepted"
	ConflictResolved    ConflictState = "resolved"
	ConflictEscalated   ConflictState = "escalated"
	ConflictCleared     ConflictState = "cleared"

	ResolutionSplit       ResolutionKind = "split"
	ResolutionSequence    ResolutionKind = "sequence"
	ResolutionYield       ResolutionKind = "yield"
	ResolutionBuildOn     ResolutionKind = "build-on"
	ResolutionProceedBoth ResolutionKind = "proceed-both"

	ThreadOpen      ThreadState = "open"
	ThreadResolved  ThreadState = "resolved"
	ThreadEscalated ThreadState = "escalated"
	ThreadExpired   ThreadState = "expired"

	SubjectConflict SubjectKind = "conflict"
	SubjectLease    SubjectKind = "lease"
	SubjectFree     SubjectKind = "free"

	MessageSubjectTask     MessageSubjectKind = "task"
	MessageSubjectLease    MessageSubjectKind = "lease"
	MessageSubjectPath     MessageSubjectKind = "path"
	MessageSubjectConflict MessageSubjectKind = "conflict"

	MessageAsk      MessageKind = "ask"
	MessageAnswer   MessageKind = "answer"
	MessageInform   MessageKind = "inform"
	MessagePropose  MessageKind = "propose"
	MessageAccept   MessageKind = "accept"
	MessageReject   MessageKind = "reject"
	MessageCounter  MessageKind = "counter"
	MessageWithdraw MessageKind = "withdraw"

	EscalationInform EscalationKind = "inform"
	EscalationDecide EscalationKind = "decide"

	DirectivePause   DirectiveKind = "pause"
	DirectiveResume  DirectiveKind = "resume"
	DirectiveNarrow  DirectiveKind = "narrow"
	DirectiveWaitFor DirectiveKind = "wait-for"
	DirectiveRebase  DirectiveKind = "rebase"
	DirectiveFree    DirectiveKind = "free"

	RungPiggyback   DeliveryRung = "piggyback"
	RungInboxSocket DeliveryRung = "inbox-socket"
	RungPTYNudge    DeliveryRung = "pty-nudge"

	OriginHuman TurnOrigin = "human"
	OriginFleet TurnOrigin = "fleet"

	NoteTargetProject NoteTargetKind = "project"
	NoteTargetTask    NoteTargetKind = "task"
	NoteTargetUser    NoteTargetKind = "user"

	InferenceGlobal InferenceRoute = "global"
	InferenceAU     InferenceRoute = "au"

	FloorYieldUnedited EscalationFloor = "yield-unedited"
	FloorAcceptSplit   EscalationFloor = "accept-split"
	FloorProceedBothPS EscalationFloor = "proceed-both-ps"
	FloorSequenceShort EscalationFloor = "sequence-short"
)

// oneOf builds a Valid function over a closed set.
func oneOf[T ~string](vs ...T) func(T) bool {
	set := make(map[T]bool, len(vs))
	for _, v := range vs {
		set[v] = true
	}
	return func(v T) bool { return set[v] }
}

var (
	validLeaseMode  = oneOf(LeaseIntent, LeaseHold, LeaseBulk)
	validLeaseState = oneOf(LeaseRequested, LeaseGranted, LeaseContested, LeaseYielded,
		LeaseReleased, LeaseExpired, LeaseAdminReleased)
	validTier          = oneOf(TierP, TierH, TierM, TierS)
	validConflictState = oneOf(ConflictRaised, ConflictNegotiating, ConflictAccepted,
		ConflictResolved, ConflictEscalated, ConflictCleared)
	validResolution = oneOf(ResolutionSplit, ResolutionSequence, ResolutionYield,
		ResolutionBuildOn, ResolutionProceedBoth)
	validThreadState = oneOf(ThreadOpen, ThreadResolved, ThreadEscalated, ThreadExpired)
	validSubject     = oneOf(SubjectConflict, SubjectLease, SubjectFree)
	validMsgSubject  = oneOf(MessageSubjectTask, MessageSubjectLease, MessageSubjectPath,
		MessageSubjectConflict)
	validMessageKind = oneOf(MessageAsk, MessageAnswer, MessageInform, MessagePropose,
		MessageAccept, MessageReject, MessageCounter, MessageWithdraw)
	validEscalation = oneOf(EscalationInform, EscalationDecide)
	validDirective  = oneOf(DirectivePause, DirectiveResume, DirectiveNarrow, DirectiveWaitFor,
		DirectiveRebase, DirectiveFree)
	validRung       = oneOf(RungPiggyback, RungInboxSocket, RungPTYNudge)
	validOrigin     = oneOf(OriginHuman, OriginFleet)
	validNoteTarget = oneOf(NoteTargetProject, NoteTargetTask, NoteTargetUser)
	validRoute      = oneOf(InferenceGlobal, InferenceAU)
	validFloor      = oneOf(FloorYieldUnedited, FloorAcceptSplit, FloorProceedBothPS,
		FloorSequenceShort)
)

func (v LeaseMode) Valid() bool          { return validLeaseMode(v) }
func (v LeaseState) Valid() bool         { return validLeaseState(v) }
func (v ConflictTier) Valid() bool       { return validTier(v) }
func (v ConflictState) Valid() bool      { return validConflictState(v) }
func (v ResolutionKind) Valid() bool     { return validResolution(v) }
func (v ThreadState) Valid() bool        { return validThreadState(v) }
func (v SubjectKind) Valid() bool        { return validSubject(v) }
func (v MessageSubjectKind) Valid() bool { return validMsgSubject(v) }
func (v MessageKind) Valid() bool        { return validMessageKind(v) }
func (v EscalationKind) Valid() bool     { return validEscalation(v) }
func (v DirectiveKind) Valid() bool      { return validDirective(v) }
func (v DeliveryRung) Valid() bool       { return validRung(v) }
func (v TurnOrigin) Valid() bool         { return validOrigin(v) }
func (v NoteTargetKind) Valid() bool     { return validNoteTarget(v) }
func (v InferenceRoute) Valid() bool     { return validRoute(v) }
func (v EscalationFloor) Valid() bool    { return validFloor(v) }

// Terminal: nothing follows it on a lease (contract §7.16).
func (v LeaseState) Terminal() bool {
	return v == LeaseYielded || v == LeaseReleased || v == LeaseExpired || v == LeaseAdminReleased
}

// Terminal: a conflict no PATCH may leave.
func (v ConflictState) Terminal() bool { return v == ConflictResolved || v == ConflictCleared }

// Typed reports whether a Fleet-started turn may issue the directive without
// an answered decide (05-design.md §7.2 item 2; contract §7.20).
func (v DirectiveKind) Typed() bool { return v.Valid() && v != DirectiveFree }
```

**Block 3, delegation constants, route names, and the grammar helpers:**

```go
// ── delegation (ADR-H1 §5.10) ───────────────────────────────────────────────

const (
	// HeaderDelegation names the delegation a Fleet request acts under.
	HeaderDelegation = "X-Panacea-Delegation"
	// ViaFleet is the audit event's via for a delegated write (v2).
	ViaFleet = "fleet"
	// App roles, read from each token's roles claim (contract §5.2).
	AppRoleFleet      = "Hub.Fleet"
	AppRoleAuditor    = "Hub.Auditor"
	AppRoleCompliance = "Hub.Compliance"
)

// RouteName is what a delegation lists (contract §7.0, team routes table).
type RouteName string

const (
	RouteDelegationsRead   RouteName = "delegations.read"
	RouteTasksCreate       RouteName = "tasks.create"
	RouteTasksUpdate       RouteName = "tasks.update"
	RouteTasksRead         RouteName = "tasks.read"
	RouteAgentsStart       RouteName = "agents.start"
	RouteAgentsEnd         RouteName = "agents.end"
	RouteAgentsRead        RouteName = "agents.read"
	RouteLeasesEvents      RouteName = "leases.events"
	RouteConflictsCreate   RouteName = "conflicts.create"
	RouteConflictsUpdate   RouteName = "conflicts.update"
	RouteConflictsRead     RouteName = "conflicts.read"
	RouteThreadsCreate     RouteName = "threads.create"
	RouteThreadsUpdate     RouteName = "threads.update"
	RouteMessagesAppend    RouteName = "messages.append"
	RouteMessagesRead      RouteName = "messages.read"
	RouteEscalationsCreate RouteName = "escalations.create"
	RouteEscalationsRead   RouteName = "escalations.read"
	RouteDirectivesAppend  RouteName = "directives.append"
	RouteDirectivesRead    RouteName = "directives.read"
	RouteTurnsRecord       RouteName = "turns.record"
	RouteHumanMessagesRead RouteName = "human_messages.read"
	RouteNotesRead         RouteName = "notes.read"
	RoutePolicyRead        RouteName = "policy.read"
)

// workerRoutes is a worker-class delegation's whole set: no message,
// directive or escalation, which are the cross-user channel (OD-16).
var workerRoutes = []RouteName{
	RouteDelegationsRead, RouteTasksRead, RouteAgentsStart, RouteAgentsEnd, RouteAgentsRead,
	RouteLeasesEvents, RouteConflictsCreate, RouteConflictsUpdate, RouteConflictsRead,
	RoutePolicyRead,
}

// ClassRoutes is each delegation class's allowed set (contract §7.0). A
// delegation's routes must be a subset of its class's.
var ClassRoutes = map[DelegationClass][]RouteName{
	DelegationClassOrchestrator: {
		RouteDelegationsRead, RouteTasksCreate, RouteTasksUpdate, RouteTasksRead,
		RouteAgentsStart, RouteAgentsEnd, RouteAgentsRead, RouteLeasesEvents,
		RouteConflictsCreate, RouteConflictsUpdate, RouteConflictsRead, RouteThreadsCreate,
		RouteThreadsUpdate, RouteMessagesAppend, RouteMessagesRead, RouteEscalationsCreate,
		RouteEscalationsRead, RouteDirectivesAppend, RouteDirectivesRead, RouteTurnsRecord,
		RouteHumanMessagesRead, RouteNotesRead, RoutePolicyRead,
	},
	DelegationClassWorker: workerRoutes,
}

// Allows reports whether class c may list route r.
func (c DelegationClass) Allows(r RouteName) bool {
	for _, x := range ClassRoutes[c] {
		if x == r {
			return true
		}
	}
	return false
}

// MaxDelegationRoutes bounds a delegation's list (contract §7.13).
const MaxDelegationRoutes = 25

// ── grammars ────────────────────────────────────────────────────────────────

var (
	sectionIDRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	harnessIDRe   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	harnessVerRe  = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,32}$`)
	shaRe         = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	dataClassRe   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	laneRefPrefix = "refs/panacea/lanes/"
)

// ValidBriefRef: a brief key (contract §3.2), "#", a section id. The section
// grammar is UNVERIFIED against phase 4's section ids until P4's merge.
func ValidBriefRef(s string) bool {
	key, section, ok := strings.Cut(s, "#")
	if !ok || !sectionIDRe.MatchString(section) {
		return false
	}
	_, err := ParseBriefKey(key)
	return err == nil
}

// ValidLaneRef: OD-2's lane namespace, with a branch ref's component rules.
func ValidLaneRef(s string) bool {
	rest, ok := strings.CutPrefix(s, laneRefPrefix)
	return ok && rest != "" && ValidBranchRef("refs/heads/"+rest)
}

// ValidHarness: the harness id and pinned version of an agent start.
func ValidHarness(h Harness) bool {
	return harnessIDRe.MatchString(h.ID) && harnessVerRe.MatchString(h.Version)
}

// ValidSha: a git object id, SHA-1 or SHA-256.
func ValidSha(s string) bool { return shaRe.MatchString(s) }

// DataClass is the project's classification (OD-24). Its values are
// UNVERIFIED until the merged project record fixes them (task 43), so only
// the grammar is checked here.
type DataClass string

func (v DataClass) Valid() bool { return dataClassRe.MatchString(string(v)) }
```

**Block 4, delegation, task, agent, lease event and conflict (contract §7.13–§7.17):**

```go
// ── §7.13 delegations ───────────────────────────────────────────────────────

type Delegation struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"projectId"`
	OwnerUserID   string          `json:"ownerUserId"`
	Class         DelegationClass `json:"class"`
	Routes        []RouteName     `json:"routes"`
	CreatedAt     string          `json:"createdAt"`
	ExpiresAt     string          `json:"expiresAt"`
	RenewedAt     *string         `json:"renewedAt"`
	RevokedAt     *string         `json:"revokedAt"`
	RevokedReason *RevokeReason   `json:"revokedReason"`
}

type CreateDelegationRequest struct {
	ProjectID string          `json:"projectId"`
	Class     DelegationClass `json:"class"`
	Routes    []RouteName     `json:"routes"`
}

// ── §7.14 tasks ─────────────────────────────────────────────────────────────

// TaskReason: Why is null to every reader but Fleet·O (05-design.md §5.1).
type TaskReason struct {
	BriefRef string         `json:"briefRef"`
	Why      *teamtext.Text `json:"why"`
}

// Task: Title is null to every reader but Fleet·O, like Reason.Why.
type Task struct {
	ID                  string         `json:"id"`
	ProjectID           string         `json:"projectId"`
	OwnerUserID         string         `json:"ownerUserId"`
	OrchestratorAgentID string         `json:"orchestratorAgentId"`
	Title               *teamtext.Text `json:"title"`
	Reason              TaskReason     `json:"reason"`
	Scope               []string       `json:"scope"`
	State               TaskState      `json:"state"`
	Revision            int            `json:"revision"`
	CreatedAt           string         `json:"createdAt"`
	UpdatedAt           string         `json:"updatedAt"`
}

type TaskReasonRequest struct {
	BriefRef string `json:"briefRef"`
	Why      string `json:"why"`
}

type CreateTaskRequest struct {
	ID                  string            `json:"id"`
	OrchestratorAgentID string            `json:"orchestratorAgentId"`
	Title               string            `json:"title"`
	TitleSource         *string           `json:"titleSource,omitempty"` // the owner's hm-…
	Reason              TaskReasonRequest `json:"reason"`
	Scope               []string          `json:"scope"`
	TrackedFiles        int               `json:"trackedFiles"`
	State               *TaskState        `json:"state,omitempty"` // default planned
}

type PatchTaskRequest struct {
	Revision     int        `json:"revision"`
	State        *TaskState `json:"state,omitempty"`
	Scope        []string   `json:"scope,omitempty"`
	TrackedFiles *int       `json:"trackedFiles,omitempty"`
	Why          *string    `json:"why,omitempty"`
}

// ── §7.15 agents ────────────────────────────────────────────────────────────

type Harness struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// PolicyRef is the OD-23 policy the sandbox started with (Phase T task 13).
type PolicyRef struct {
	Version int    `json:"version"`
	Digest  Digest `json:"digest"`
}

type AgentStartRequest struct {
	ID            string    `json:"id"`
	Role          AgentRole `json:"role"`
	ParentAgentID *string   `json:"parentAgentId,omitempty"`
	TaskID        *string   `json:"taskId,omitempty"`
	Harness       Harness   `json:"harness"`
	LaneRef       string    `json:"laneRef"`
	Policy        PolicyRef `json:"policy"`
}

// AgentStart is the Hub's agent record: its start, its end and its role.
// Status (working, waiting, …) is Fleet state and is never here.
type AgentStart struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	OwnerUserID   string     `json:"ownerUserId"`
	Role          AgentRole  `json:"role"`
	ParentAgentID *string    `json:"parentAgentId"`
	TaskID        *string    `json:"taskId"`
	Harness       Harness    `json:"harness"`
	LaneRef       string     `json:"laneRef"`
	Policy        PolicyRef  `json:"policy"`
	StartedAt     string     `json:"startedAt"`
	EndedAt       *string    `json:"endedAt"`
	ExitClass     *ExitClass `json:"exitClass"`
}

type AgentEndRequest struct {
	ExitClass ExitClass `json:"exitClass"`
}

// ── §7.16 lease events ──────────────────────────────────────────────────────

type LeaseEventRequest struct {
	Seq           int        `json:"seq"`
	HolderAgentID string     `json:"holderAgentId"`
	Mode          LeaseMode  `json:"mode"`
	Globs         []string   `json:"globs"`
	TrackedFiles  int        `json:"trackedFiles"`
	State         LeaseState `json:"state"`
	ExpiresAt     string     `json:"expiresAt"`
	At            string     `json:"at"`
}

type LeaseEvent struct {
	LeaseID string     `json:"leaseId"`
	Seq     int        `json:"seq"`
	State   LeaseState `json:"state"`
	At      string     `json:"at"`
}

// ── §7.17 conflicts ─────────────────────────────────────────────────────────

// LineRange is [start, end], 1 ≤ start ≤ end.
type LineRange [2]int

type ConflictPath struct {
	Path  string      `json:"path"`
	Lines []LineRange `json:"lines"`
}

type ConflictBasis struct {
	A string `json:"a"`
	B string `json:"b"`
}

type CreateConflictRequest struct {
	ID     string         `json:"id"`
	AgentA string         `json:"agentA"`
	AgentB string         `json:"agentB"`
	Tier   ConflictTier   `json:"tier"`
	Paths  []ConflictPath `json:"paths"`
	Basis  ConflictBasis  `json:"basis"`
}

type Conflict struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"projectId"`
	AgentA     string          `json:"agentA"`
	AgentB     string          `json:"agentB"`
	Tier       ConflictTier    `json:"tier"`
	Paths      []ConflictPath  `json:"paths"`
	Basis      ConflictBasis   `json:"basis"`
	State      ConflictState   `json:"state"`
	Resolution *ResolutionKind `json:"resolution"`
	ThreadID   *string         `json:"threadId"`
	Revision   int             `json:"revision"`
	CreatedAt  string          `json:"createdAt"`
	UpdatedAt  string          `json:"updatedAt"`
}

type PatchConflictRequest struct {
	Revision   int             `json:"revision"`
	State      ConflictState   `json:"state"`
	Resolution *ResolutionKind `json:"resolution,omitempty"`
	ThreadID   *string         `json:"threadId,omitempty"`
}
```

**Block 5, thread, message and escalation (contract §7.18–§7.19).** A message's `fields` is a flat
union: every key is present on the wire, and only the kind's own keys carry a value (the table in
contract §7.18); the rest are `null`, `[]` or `{}`. Task 07's handler refuses a request that sets a
key its kind does not own.

```go
// ── §7.18 threads and messages ──────────────────────────────────────────────

// ThreadSubject: ID is null for a free thread.
type ThreadSubject struct {
	Kind SubjectKind `json:"kind"`
	ID   *string     `json:"id"`
}

type CreateThreadRequest struct {
	ID            string        `json:"id"`
	OpenerAgentID string        `json:"openerAgentId"`
	Participants  []string      `json:"participants"` // exactly 2
	Subject       ThreadSubject `json:"subject"`
	Deadline      string        `json:"deadline"`
}

type Thread struct {
	ID            string        `json:"id"`
	ProjectID     string        `json:"projectId"`
	OpenerAgentID string        `json:"openerAgentId"`
	Participants  []string      `json:"participants"`
	Subject       ThreadSubject `json:"subject"`
	State         ThreadState   `json:"state"`
	Rounds        int           `json:"rounds"`
	Deadline      string        `json:"deadline"`
	Revision      int           `json:"revision"`
	CreatedAt     string        `json:"createdAt"`
}

type PatchThreadRequest struct {
	Revision int          `json:"revision"`
	State    *ThreadState `json:"state,omitempty"`
	Rounds   *int         `json:"rounds,omitempty"`
}

type MessageSubject struct {
	Kind MessageSubjectKind `json:"kind"`
	Ref  string             `json:"ref"` // tk-…, ls-…, cf-…, or a repo-relative path
}

// ResolutionProposal is contract §7.18's Resolution: Sides for split, First
// for sequence, Globs for yield, Base for build-on, Paths for proceed-both.
type ResolutionProposal struct {
	Kind  ResolutionKind      `json:"kind"`
	Sides map[string][]string `json:"sides"`
	First *string             `json:"first"`
	Globs []string            `json:"globs"`
	Base  *string             `json:"base"`
	Paths []string            `json:"paths"`
}

type MessageFields struct {
	Subject    *MessageSubject     `json:"subject"`    // ask, inform
	InReplyTo  *string             `json:"inReplyTo"`  // answer
	Resolution *ResolutionProposal `json:"resolution"` // propose, counter
	Proposal   *string             `json:"proposal"`   // accept, reject, withdraw
}

type AppendMessageRequest struct {
	ID            string        `json:"id"`
	SenderAgentID string        `json:"senderAgentId"`
	Kind          MessageKind   `json:"kind"`
	Fields        MessageFields `json:"fields"`
	Text          *string       `json:"text,omitempty"`
	Templated     bool          `json:"templated"`
}

type Message struct {
	ID            string         `json:"id"`
	ThreadID      string         `json:"threadId"`
	ProjectID     string         `json:"projectId"`
	SenderAgentID string         `json:"senderAgentId"`
	SenderUserID  string         `json:"senderUserId"`
	Kind          MessageKind    `json:"kind"`
	Fields        MessageFields  `json:"fields"`
	Text          *teamtext.Text `json:"text"`
	Templated     bool           `json:"templated"`
	CreatedAt     string         `json:"createdAt"`
	TombstonedAt  *string        `json:"tombstonedAt"`
}

// ── §7.19 escalations ───────────────────────────────────────────────────────

type EscalationOptionRequest struct {
	Label       string `json:"label"`       // ≤ 80
	Consequence string `json:"consequence"` // ≤ 200
}

type CreateEscalationRequest struct {
	ID          string                    `json:"id"`
	FromAgentID *string                   `json:"fromAgentId,omitempty"` // absent when Fleet raises it
	Kind        EscalationKind            `json:"kind"`
	ThreadID    *string                   `json:"threadId,omitempty"`
	Question    string                    `json:"question"`
	Options     []EscalationOptionRequest `json:"options"`
	Default     *int                      `json:"default,omitempty"` // required for decide
	Deadline    string                    `json:"deadline"`
}

// EscalationOption: both texts are an agent's, so both carry their class.
type EscalationOption struct {
	Label       teamtext.Text `json:"label"`
	Consequence teamtext.Text `json:"consequence"`
}

type EscalationAnswer struct {
	Choice     int            `json:"choice"`
	Text       *teamtext.Text `json:"text"`
	AnsweredAt string         `json:"answeredAt"`
}

type Escalation struct {
	ID          string             `json:"id"`
	ProjectID   string             `json:"projectId"`
	OwnerUserID string             `json:"ownerUserId"`
	FromAgentID *string            `json:"fromAgentId"`
	Kind        EscalationKind     `json:"kind"`
	ThreadID    *string            `json:"threadId"`
	Question    teamtext.Text      `json:"question"`
	Options     []EscalationOption `json:"options"`
	Default     *int               `json:"default"`
	Deadline    string             `json:"deadline"`
	Answer      *EscalationAnswer  `json:"answer"`
	CreatedAt   string             `json:"createdAt"`
}

type AnswerEscalationRequest struct {
	Choice int     `json:"choice"`
	Text   *string `json:"text,omitempty"` // ≤ 300
}
```
