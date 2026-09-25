### Task 101: the verbs: `panacea issue show`, `panacea issue note`, `panacea evidence propose`; `panacea team` names each agent's issue

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WF (Fleet).
**dependsOn:** 100; TM-18, TM-19, TM-22. **Seams:** consumes **SW.7** (task 19's managed block)
and **SW.19** (tasks 99-100). Every Phase T path is `cp/`→`control-plane/`, `sbx/`→`sandboxd/`
(Phase T plan `:190-191`) and is **UNVERIFIED until merge** (rule H2: the owner phase's file wins).

An agent on a delegated issue needs three things it cannot do today: read the issue as data, say
one thing on it, and propose evidence for a criterion it serves. This task adds them as three verbs
on Phase T's `panacea` control wire (ST.1), served by Fleet, and makes `panacea team` name the
issue each agent works on. The verbs are **our binary** (`sandboxd/cmd/panacea`, TM-18): no agent
CLI is parsed or modified (OD-8).

**Anchors, re-read on 2026-09-25 at `feat/team` `64ab2c4`:** TM-09's verb table (`task-09.md:275-325`:
`model.Verb{Name, Classes, Local}`, `ClassMay`) and code list (`:420-455`: `CodeForbiddenClass`,
`CodeAgentNotOwner`, `CodeNotFound`, `CodeTextRefused`, `CodeRateLimited`, `CodeUnavailable`);
TM-19's dispatch (`task-19.md:84-145`: `Caller{Org, Project, Agent, Owner, Class}`, `Handler`,
`Handle(verb, h, opts...)`, which panics on a verb *"that no class may call in task 09's table"*,
`Decode[T]`); TM-22's `team` result (`task-22.md:166-245`: `TeamWorker`, `WorkerAgent`,
`TeamOrchestrator`, `OrchCard`, `Quoted{Text, Class, By, Owner}`; *"Order is fixed so fixtures
compare byte for byte"*); TM-18's CLI (`task-18.md:36-60`: four exit codes, exit `3` prints
*"team view unavailable since <t>"*, `--args '<json>'` for every verb; `:249-272`: `verb{name,
usage, build}`, `table[argv[0]]`); TM-11's `delegation.Client.Ctx(ctx, k, route)` and
`hubclient.WithDelegation` (`task-11.md:36-42`, `:460-479`); TM-41's
`fleet/instructions/orchestrator.md` (`task-41.md:39`). Phase W: task 19's golden
(`task-19.md:200-232`) and its Q3 rule that comments are read *"only as data, through `panacea
issue show` (task 101)"* (`:22-24`); task 23's done evaluator; task 100's routes and route names;
04 §4.5 (`04-internal-fit.md:355-372`) and §4.6 steps 1-2 (`:374-383`).

**Not on any branch, so UNVERIFIED:** all of Phase T's code; task 99's `hubclient.WorkAgents` and
`hubapi` W3 types (this part uses task 100's restated names); task 98's Fleet store rows for
`issue.delegated` and `task.issueId`.

Seven decisions the scope left open, made here.

**(a) Wire names are dotted, like `turn.end`.** `issue.show`, `issue.note`, `evidence.propose`.
The CLI's `table[argv[0]]` lookup (TM-18) gains one step: with two or more argv words it tries
`argv[0] + "." + argv[1]` first, so `panacea issue show EP-142` sends `issue.show`. `panacea
issue` alone is exit `2` with the usage of both sub-verbs.

**(b) What "delegated" means to Fleet.** An issue is in the caller's scope when its owner
delegated it (a stored `issue.delegated` event with `ownerId` = the caller's owner, task 98), or
one of the owner's tasks is bound to it (`task.issueId`, task 98, which the Hub checked is the
owner's). Anything else is `agent_not_owner`, one code for "not yours" and "no such issue"
(TM-09's rule). `issue.show` has a third, read-only way in: the subject issue of the Orchestrator's
current `mention` turn (task 103), so it can read the comment it was called to. The Hub holds the
write half underneath: an agent comment needs the owner to be the assignee (task 100 (c)).

**(c) Class rules live in TM-09's table.** `issue.show` is Orchestrator and worker (a worker
reads its task's issue as data); `issue.note` and `evidence.propose` are Orchestrator only. Three
rows are appended to `model.Verbs`; TM-19's dispatch then refuses a worker with
`forbidden_class` before any handler runs.

**(d) A note is ≤ 600 characters, one per issue per owner per 5 minutes.** 04 §4.5 says
*"≤ 600, attributed, rate-limited per issue"* and gives no rate; five minutes is this part's
choice, a constant (`NoteEvery`), so a chatty Orchestrator cannot flood a person's issue. A refusal
is `rate_limited` with `retryAt`. Text goes through TM-02's `teamtext.Clean` first; a refused
string is `text_refused`.

**(e) `evidence.propose` names its issue.** The scope's args are `{criterion, commit, because}`;
a criterion may be served by two issues (task 02's `serves` is many-to-many), so `issue` is
required. The criterion must be one the issue's **head revision** serves; Fleet fills `briefKey`
from that `serves` row, so the agent never types a brief key.

**(f) `issue.show` answers the key, never writes the alias.** It accepts `is-…` or the display
alias (`EP-142`); the alias is resolved with `IssueByNumber` at call time, and the result carries
`key: "issue:is-…@r"` (skeleton rule 1). Comments come back as **data**: kind, author, class,
body, inside TM-18's quoted envelope, never rendered into a managed block (05 §3.4).

**(g) `panacea team` names issues on the Orchestrator's view only.** `OrchCard` gains `issue
{key, alias, title, titleClass}` and `criterion {briefKey, criterionId}`; the title is the
issue's, `titleClass: "H"` where a person wrote it, shown instead of the task's. The worker view
is **unchanged**: TM-22 pins its keys to task 16's `team.json` snapshot, whose decode refuses an
unknown field (`task-22.md:168-171`), so one added key would break every live pod's snapshot. A
worker reads its own task's issue with `panacea issue show` and no argument.

**Files:**

| Path | Change |
|---|---|
| `control-plane/internal/fleet/verbs/issue.go`, `issue_test.go` | new: `issue.show`, `issue.note`, `IssueScope`, `NoteLimiter` |
| `control-plane/internal/fleet/verbs/evidence.go`, `evidence_test.go` | new: `evidence.propose` |
| `control-plane/internal/fleet/verbs/team.go` ⚑ (TM-22) | `OrchCard` gains `Issue` and `Criterion`; assembly fills them |
| `control-plane/internal/fleet/model/verbs.go` ⚑ (TM-09) | three rows in `Verbs` |
| `control-plane/internal/fleet/control/dispatch.go` ⚑ (TM-19) | none if registration is by `Handle` in `fleetd`; else three `Handle` calls (Step 1) |
| `control-plane/cmd/fleetd/main.go` ⚑ (TM-19) | three `d.Handle(...)` calls with the verbs' dependencies |
| `control-plane/internal/fleet/instructions/orchestrator.md` ⚑ (TM-41) | one paragraph naming the three verbs |
| `sandboxd/internal/panaceacli/issue.go`, `evidence.go`, `work_test.go` | new: argv → `args` for the three verbs |
| `sandboxd/internal/panaceacli/cli.go` ⚑, `verbs.go` ⚑ (TM-18) | the dotted lookup (decision (a)); three table rows |
| `testdata/panacea-control/v1/request-issue-show.json`, `request-issue-note.json`, `request-evidence-propose.json`, `result-issue-show.json` | new: fixtures both modules read |

The scope put the three dispatch rows in `dispatch.go`; TM-19 registers verbs with
`Dispatcher.Handle` and checks classes against TM-09's `model.Verbs`, so the class rows go to
`model/verbs.go` and the registrations to wherever TM-19's `fleetd` wiring calls `Handle`. TM-41's
`orchestrator.md` is added: without it no Orchestrator is told the verbs exist (gap 1).

#### The wire, complete: ST.1 requests and results

```jsonc
// panacea issue show [EP-142 | is-…]  →  {"verb":"issue.show","args":{"issue":"EP-142"}}
// args.issue may be omitted: the caller's own task's issue (a worker's usual call)
// result
{ "key": "issue:is-3f2a91c07b1e4d2a@3", "alias": "EP-142", "state": "started",
  "assignee": "Dylan Example", "priority": "high",
  "title":       { "text": "Resume a session after a restart", "class": "text·H", "by": "Dylan Example", "owner": "Dylan Example" },
  "description": { "text": "A restarted sandbox must pick up …", "class": "text·H", "by": "Dylan Example", "owner": "Dylan Example" },
  "serves": [ { "briefKey": "brief:bf-00000000000000a2@5", "criterionId": "c-91e2a4b0" } ],
  "comments": [
    { "id": "ic-…", "kind": "comment", "at": "…",
      "body": { "text": "the 8 MB chunk is a guess", "class": "text·H", "by": "Alice Example", "owner": "Alice Example" } },
    { "id": "ic-…", "kind": "note", "at": "…",
      "body": { "text": "assumed chunk size 8 MB", "class": "text·A", "by": "ag-…", "owner": "Dylan Example" },
      "attribution": { "agent": "ag-…", "model": "claude-opus-4-8", "session": "…" } } ],
  "commentsMore": false }
// every free-text value is TM-22's Quoted, so TM-18's envelope quotes it like any teammate's
// words; comments are the newest 50, oldest first; commentsMore says there are older ones

// panacea issue note EP-142 "assumed chunk size 8 MB"  →
{"verb":"issue.note","args":{"issue":"EP-142","text":"assumed chunk size 8 MB"}}
// result
{ "comment": "ic-…", "issue": "issue:is-…@3", "nextNoteAt": "2026-09-25T10:05:00Z" }

// panacea evidence propose --issue EP-142 --criterion c-91e2a4b0 --commit 7c1f0a2… \
//   --because "resume test, exit 0" [--artefact test:test/upload.spec.ts]
{"verb":"evidence.propose","args":{"issue":"EP-142","criterion":"c-91e2a4b0",
  "commit":"7c1f0a2000000000000000000000000000000000","because":"resume test, exit 0",
  "artefact":{"kind":"test","ref":"test/upload.spec.ts"}}}
// result
{ "comment": "ic-…", "briefKey": "brief:bf-…@5", "criterion": "c-91e2a4b0", "counts": false }
// counts is always false: a proposal never settles a criterion; a person records it (task 104)

// refusals, TM-09's codes, each with a message that names the field
// forbidden_class (a worker on note or propose) · agent_not_owner (outside decision (b))
// text_refused {field} · rate_limited {retryAt} · bad_request {field} (601 characters, a
// commit that is not 40 lowercase hex, a criterion the head revision does not serve)
// unavailable {since} (Fleet or the Hub path is down: CLI exit 3)
```

#### `control-plane/internal/fleet/verbs/issue.go`, interfaces complete

```go
package verbs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/control"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

const (
	VerbIssueShow       = "issue.show"
	VerbIssueNote       = "issue.note"
	VerbEvidencePropose = "evidence.propose"

	NoteMax      = 600             // characters, after teamtext.Clean (04 §4.5)
	NoteEvery    = 5 * time.Minute // one note per (issue, owner); decision (d)
	ShowComments = 50              // newest 50, returned oldest first
)

// IssueScope answers decision (b). Task 98's Fleet store rows satisfy it: the owner's
// issue.delegated events, the owner's tasks' issueId, and the running turn's events.
type IssueScope interface {
	Delegated(ctx context.Context, org string, owner model.Owner, issueID string) (bool, error)
	BoundTask(ctx context.Context, org string, owner model.Owner, issueID string) (bool, error)
	// TaskIssue is the issue of the calling agent's own task, "" when it has none.
	TaskIssue(ctx context.Context, org string, agent model.AgentID) (string, error)
	// InTurn is true when the Orchestrator's running turn holds a mention event on the issue
	// (task 103). Read-only: issue.show consults it, issue.note never does.
	InTurn(ctx context.Context, org string, agent model.AgentID, issueID string) (bool, error)
}

// WorkHub is the part of hubclient.Work the verbs call, every call under the owner's
// delegation (TM-11's delegation.Client.Ctx). Names are task 04's and task 100's.
type WorkHub interface {
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
	IssueByNumber(ctx context.Context, projectID string, number int) (hubapi.IssueWithRevision, error)
	Comments(ctx context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error)
	AddAgentComment(ctx context.Context, issueID string, req hubapi.AgentCommentRequest) (hubapi.Comment, error)
}

// Delegator is TM-11's delegation.Client as the verbs see it.
type Delegator interface {
	Ctx(ctx context.Context, k DelegationKey, route string) (context.Context, error)
}

// DelegationKey is TM-11's delegation.Key, restated so this file compiles against the
// interface; Step 1 replaces it with the real type.
type DelegationKey struct {
	OrgID     string
	Owner     model.Owner
	ProjectID string
	Class     model.Class
}

// NoteLimiter is the per-(issue, owner) window. The in-memory one is enough: Fleet runs one
// dispatcher per org (TM-19's server.Singleton), and a restart forgets at most one window.
type NoteLimiter interface {
	// Take reports ok, or the time the next note may be written.
	Take(org string, owner model.Owner, issueID string, now time.Time) (ok bool, retryAt time.Time)
}

// Sessions names the model and session of the calling agent for attribution (L-173). Task
// 20's session record holds both; the verbs never take them from args.
type Sessions interface {
	Attribution(ctx context.Context, org string, agent model.AgentID) (modelName, sessionID string, err error)
}

type IssueVerbs struct {
	Scope    IssueScope
	Hub      WorkHub
	Deleg    Delegator
	Limit    NoteLimiter
	Sessions Sessions
	Now      func() time.Time
}

// ShowArgs, NoteArgs are decoded with control.Decode (strict, G2).
type ShowArgs struct {
	Issue string `json:"issue,omitempty"` // is-… or EP-142; "" = the caller's task's issue
}
type NoteArgs struct {
	Issue string `json:"issue"`
	Text  string `json:"text"`
}

func (v *IssueVerbs) Show(ctx context.Context, c control.Caller, args json.RawMessage) (any, error)
func (v *IssueVerbs) Note(ctx context.Context, c control.Caller, args json.RawMessage) (any, error)
```
