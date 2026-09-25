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
