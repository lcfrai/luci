### Task 61: The team land queue over the mirror

In TM1 a team member lands the phase 3 way, in her own project repository (`05-design.md` §8.3).
From TM2 a team project has **one land queue for every member's lanes**, held by Fleet and run
over the lane mirror (task 58's pod, task 59's runner): one integration line that every member's
agents converge on, each change attributed to the person whose agent made it. The queue is
ordered by task 29's `sequence` hints. Each land is still **its owner's reviewed act** (task 49's
review). The integration merge is built in the mirror, handed as a bundle to the lane owner's
`machined`, and pushed by that owner with **that owner's own token** (phase 3 task 13). Every
member's repository then takes the new head from the mirror, as a bundle (phase 3 ruling R4).

Four properties, each pinned by a test.

1. **Order is the hints, then arrival.** An entry whose lane was the *second* side of a live
   `sequence` hint waits behind the *first* side; otherwise entries land in the order their
   owners asked.
2. **Nothing lands that its review has not passed.** At the head of the queue, and again just
   before the handoff, task 49's step 9a rule must pass (`parts/task-49.md:119-127`): an
   approved `la-…` by another current member, for this lane tip, covering every agent-config
   path the merge changes. The land is in the mirror, not in `converge.Land`, so `landq.Gate`
   applies that rule itself, over the mirror's typed path list and task 49's route 86.
3. **Fleet never holds a push credential.** The handoff to `machined` carries a bundle and a
   ref, and no credential field exists in its type. The push is the owner's own act, on the
   owner's own path to her machine, with her own token in memory.
4. **Fleet believes a push only when the remote shows it.** The mirror re-reads the customer's
   remote with the project's read-only fetch credential (task 48) and the entry is `landed` only
   when the branch names the merge the mirror built.

Five decisions the scope left open, made here.

**(a) The merge is phase 3's, moved.** Phase 3 task 09's `Land` builds a two-parent merge with
`merge-tree --write-tree`, `commit-tree -p old -p laneTip` and task 09's trailers, then
`update-ref` with the old tip as a compare-and-swap (phase 3
`parts/task-09.md:98-132`, **UNVERIFIED until merge**: the part exists, the code does not). The
team land runs the same steps 6 to 11 **in the mirror**, over two refs the mirror already holds:
`refs/panacea/integration/<branch>` (the last remote head it fetched with the fetch credential)
and the lane's `refs/panacea/lanes/<owner-hash>/<wt-id>` (task 59). The mirror never runs step 12's
`update-ref` on a branch anyone pushes from; it writes the merge to
`refs/panacea/landq/<entry>` and exports `old..merge` as a bundle. A textual conflict refuses the
entry with `merge_conflict` and the paths, never content, exactly as task 09 refuses one.

**(b) The commit is the lane owner's.** `commit-tree` takes `GIT_AUTHOR_*` and `GIT_COMMITTER_*`
from the identity the owner's `machined` reported with her lane's bundle (task 60's export;
phase 3 task 09 step 8's `Identity`, the user's own). Name and email are bounded (`teamtext`,
task 02, 200 and 254 characters) and hygiene-checked. The trailers add one line to task 09's:
`Panacea-Owner: <Hub principal id>`, so release attribution (task 68) reads the person from the
commit, never from the git author (`04` §7.3).

**(c) The push is a click, not a queue step.** OD-22 and the global constraint 8 keep push a human
action with the owner's token, per act, in memory only. So the queue stops at `ready`, and the
owner's Desk shows *"Your lane `auth-refresh` is next to land into `main`"* (task 54's NEEDS YOU
row). Her **Land** click goes from the desktop through her own local control plane, which reads
the token from phase 3 task 13's credential store, to her own machine through phase 8's broker
(the S8.1 path she already uses), and pushes the merge with `--force-with-lease=<branch>:<old>`.
Fleet is not on that path. A `ready` entry nobody clicks for 24 hours goes back to `waiting` and
yields the head; its owner is told once.

**(d) Every member's repository takes the new head from the mirror.** Once the push is verified,
Fleet asks the mirror for one bundle per member, `<that member's last told head>..merge`, and
hands each to that member's `machined` through the broker. `machined` applies it as phase 3 task
09 step 14's `RefreshClones` does: a bundle into each lane's runner (R4.5), never a host git in
a lane. A member whose machine is asleep is told on her next start; a failure is logged per
member and never fails the land.

**(e) One land at a time per project.** The queue has at most one entry in `building`, `ready`
or `pushing`. A push rejected because the remote moved (someone pushed outside the queue) sends
the entry back to `waiting` at the head; the mirror fetches the remote and rebuilds. Two rejects
in a row stop the queue with `remote_contended`, one inform to every member, and no retry loop.

**(f) Fleet runs no git over lane objects.** §8.2 keeps git's parsers out of the process that
holds every member's delegations. So the agent-config list is the mirror's, typed: its build
returns `git diff --name-status -z --no-renames` between `old` and the merge, and for every
changed `package.json` the names of the members that differ, from a member list **the request
carries** (`converge.PackageScriptMembers`, task 49's four, exported). Fleet classifies the paths
with task 49's `converge.IsAgentConfigPath`, so the table exists once, in `converge`.

**Files:**
- `control-plane/internal/fleet/landq/landq.go` — `Queue`, `Entry`, `State`, the transitions,
  `Enqueue`, `Withdraw`, `Advance`, `Pushed`, `Expire` (new; the file map's row)
- `control-plane/internal/fleet/landq/order.go` — `Order`: hints, then arrival (new; not in the
  file map, split out so its table test stands alone)
- `control-plane/internal/fleet/landq/gate.go` — `Gate`: task 49's rule over the mirror's paths
  (new; not in the file map)
- `control-plane/internal/fleet/landq/refresh.go` — the per-member bundle fan-out (new)
- `control-plane/internal/fleet/landq/{landq,order,gate,refresh}_test.go` (new)
- `control-plane/internal/fleet/server/landq.go` + test — routes F-L1 to F-L5 (new)
- `control-plane/internal/fleet/server/authz.go` (task 12) — `VerbLandQueue`, one table row (modify)
- `control-plane/internal/fleet/store/migrations/00NN_landq.sql`, `store.go`, `memstore.go`,
  `pgstore.go`, `storetest/suite.go` (task 10) — `fleet.land_queue` and its methods (modify;
  numbered after the last Fleet migration at build time)
- `control-plane/internal/fleet/mirror/client.go` (task 59) — `BuildLand`, `ExportRange`,
  `FetchRemote` (modify; **task 59's file, not yet on disk**)
- `sandboxd/cmd/panacea-mirror/main.go` (task 59) — the `land`, `export` and `fetch-remote` ops
  (modify; task 59's file)
- `control-plane/internal/fleet/protocol/resolve.go` (task 29) — `Resolver.FirstLanded` (modify;
  one method, below)
- `control-plane/internal/converge/agentconfig.go` (task 49) — export `PackageScriptMembers`
  (modify; one `var`)
- `control-plane/internal/api/teamland.go` + test — the local control plane's push act: the
  owner's token from phase 3 task 13's store, through her own machine (new)
- `desktop/src/api/team.ts`, `teamSlice.ts` (task 52) — `landQueue`, `landNow` (modify)

**Sibling files this task extends, each UNVERIFIED until merge:** phase 3 task 09's
`control-plane/internal/converge/land.go` (the steps copied into the mirror; phase 3
`parts/task-09.md:98-132`), task 13's credential `Store` (`parts/task-13.md:63-78`, the push
token), task 12's push (`parts/task-12.md`, the route the push act reuses on the machine), and
phase 8's `remotebe` and broker (tasks 17–22, planned), the path the push act travels.

**Depends on:** task 49 (the rule, `IsAgentConfigPath`, route 86 and `hubclient.ListLandApprovals`),
task 60 (lane bundles in the mirror with the owner's identity), task 59 (the mirror client), task
29 (`fleet.land_hints`), task 48 (the fetch credential the mirror's `fetch-remote` uses), task 10
(the store), task 12 (routes and verbs).

#### Interfaces — complete

```go
// control-plane/internal/fleet/landq/landq.go
package landq

import (
	"context"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

type State string

const (
	StateWaiting   State = "waiting"   // queued; not the head, or held by a hint
	StateBuilding  State = "building"  // the mirror is building the merge
	StateReady     State = "ready"     // built, gated, staged on the owner's machine; waits for her click
	StatePushing   State = "pushing"   // claimed by the owner's click; her machine is pushing
	StateLanded    State = "landed"    // the mirror read the merge on the remote branch
	StateRefused   State = "refused"   // Reason says why; the owner fixes and enqueues again
	StateWithdrawn State = "withdrawn" // the owner took it back
)

// Live is the set the partial unique indexes below count.
func (s State) Live() bool {
	return s == StateWaiting || s == StateBuilding || s == StateReady || s == StatePushing
}

// Entry is one lane asking to land. OwnerUserID comes from the token at
// enqueue, never from a body. Paths are repo-relative, never content.
type Entry struct {
	ID          string        `json:"id"`          // lq-…, ids.Wide("lq"): Fleet state, not a Hub record
	OrgID       string        `json:"orgId"`
	ProjectID   string        `json:"projectId"`
	Branch      string        `json:"branch"`      // the product branch, a task 03 ValidBranchRef leaf
	OwnerUserID string        `json:"ownerUserId"` // u-… (hubapi.PrefixUser)
	Agent       model.AgentID `json:"agentId"`     // the lane's worker, the owner's own
	WorktreeID  string        `json:"worktreeId"`  // wt-…
	LaneRef     string        `json:"laneRef"`     // refs/panacea/lanes/<owner-hash>/<wt-id>, task 03 ValidLaneRef
	LaneTip     string        `json:"laneTip"`     // the tip the owner reviewed; ValidSha
	BriefKey    string        `json:"briefKey"`    // "" or the key the owner saw (phase 3 task 09 step 5)
	BriefDigest string        `json:"briefDigest"`
	State       State         `json:"state"`
	Reason      string        `json:"reason"`      // a Code below; "" while all is well
	Paths       []string      `json:"paths"`       // a refusal's paths; [] never null; at most 50
	Base        string        `json:"base"`        // the remote head the merge was built on
	Merge       string        `json:"merge"`       // the merge commit, once built
	HeldBy      string        `json:"heldBy"`      // cf-… of the hint holding it, "" when free
	Rejects     int           `json:"rejects"`     // consecutive remote_moved push rejects (decision e)
	EnqueuedAt  time.Time     `json:"enqueuedAt"`
	ReadyAt     time.Time     `json:"readyAt"`     // zero until ready
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// Codes, all ST.1-style snake_case, each in the problem body's code.
const (
	CodeMergeConflict   = "merge_conflict"           // details: paths (task 09's)
	CodeSecondApproval  = "second_approval_required" // details: paths, laneTip (task 49's)
	CodeHubUnavailable  = "hub_unavailable"          // never read as "no approval needed"
	CodeLaneMoved       = "lane_moved"               // the mirror's lane tip is not the reviewed one
	CodeBriefMismatch   = "brief_unresolvable"       // task 09 step 5's family
	CodeReadyExpired    = "ready_expired"            // a requeue reason, not a refusal
	CodeRemoteMoved     = "remote_moved"             // a requeue reason
	CodeRemoteContended = "remote_contended"         // the queue stops
	CodePushUnverified  = "push_unverified"          // the owner said pushed; the remote disagrees
	CodePushFailed      = "push_failed"              // her machine's push failed for another reason
	CodeNotLive         = "entry_not_live"
	CodeAgentNotOwner   = "agent_not_owner"          // task 06's code, reused
)

const (
	ReadyFor       = 24 * time.Hour // decision c
	StopAfter      = 2              // consecutive remote_moved rejects, decision e
	VerifyAttempts = 3              // FetchRemote reads, 20 s apart, before push_unverified
	MaxBundle      = 256 << 20      // one handoff; task 59's bundle cap, UNVERIFIED until 59 is written
)

var ErrStopped = errors.New("landq: the queue is stopped")
```

```go
// control-plane/internal/fleet/landq/landq.go, continued: what the queue needs.

// Mirror is task 59's client (mirror.Client), three ops this task adds to it.
type Mirror interface {
	// FetchRemote fetches <branch> from the customer's remote with the project's
	// read-only fetch credential (task 48) into refs/panacea/integration/<branch>
	// and returns its sha. The credential never leaves the mirror pod.
	FetchRemote(ctx context.Context, project, branch string) (head string, err error)
	// BuildLand is phase 3 task 09 steps 6 to 11, in the mirror (decision a).
	BuildLand(ctx context.Context, project string, b LandBuild) (LandBuilt, error)
	// ExportRange bundles from..to (from may be "" for a first refresh).
	ExportRange(ctx context.Context, project, from, to string) (Bundle, error)
}

type LandBuild struct {
	Entry          string   `json:"entry"`   // the merge goes to refs/panacea/landq/<entry>
	Base           string   `json:"base"`    // integration/<branch>'s sha; the mirror refuses if it moved
	LaneRef        string   `json:"laneRef"`
	LaneTip        string   `json:"laneTip"` // must equal LaneRef's sha, else lane_moved
	AuthorName     string   `json:"authorName"`  // decision b, bounded, hygiene-checked
	AuthorEmail    string   `json:"authorEmail"`
	Message        string   `json:"message"` // built by Fleet with phase 3's gitx.Message, trailers included
	PackageMembers []string `json:"packageMembers"` // converge.PackageScriptMembers
}

type PathChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "A" | "M" | "D", --no-renames
}

// LandBuilt is ST.7's family: typed facts, no file content.
type LandBuilt struct {
	Merge          string              `json:"merge"`      // "" when Conflicted is non-empty
	Changes        []PathChange        `json:"changes"`    // Base..Merge
	PackageChanged map[string][]string `json:"packageChanged"` // package.json path → members that differ, sorted
	Conflicted     []string            `json:"conflicted"` // non-empty: nothing was written
	LaneMoved      bool                `json:"laneMoved"`
}

type Bundle struct {
	Prereq string `json:"prereq"` // "" for a full bundle
	Tip    string `json:"tip"`
	Bytes  []byte `json:"-"`      // at most MaxBundle; carried as the body, never in JSON
}

// Handoff is what reaches a member's machined through the broker (task 17's
// control relay). It has NO credential field, and landq imports no credential
// package: TestLandqHoldsNoCredential checks both (property 3).
type Handoff struct {
	Entry  string `json:"entry"`  // "" for a refresh
	Ref    string `json:"ref"`    // refs/panacea/landq/<entry> or refs/panacea/integration/<branch>
	Tip    string `json:"tip"`
	Prereq string `json:"prereq"`
	Bundle []byte `json:"-"`
}

// Machines is task 17's broker client, as Fleet's caller under each owner's delegation.
type Machines interface {
	Stage(ctx context.Context, ownerUserID, project string, h Handoff) error   // the lane owner only
	Refresh(ctx context.Context, memberUserID, project string, h Handoff) error // decision d
	Identity(ctx context.Context, ownerUserID, project string) (name, email string, err error)
}

// Approvals is task 49's route 86 through hubclient, under the owner's delegation.
type Approvals interface {
	ListLandApprovals(ctx context.Context, project string, agent model.AgentID) ([]Approval, error)
}

// Approval mirrors converge.Approval (task 49), copied here so landq never
// imports converge's git half; Gate imports only converge.IsAgentConfigPath.
type Approval struct {
	ID, State, LaneTip, ApproverUserID string
	Paths                              []string
}

// Hints is task 29's fleet.land_hints, read-only here, plus the one call that
// releases a hint once its first side has landed.
type Hints interface {
	Live(ctx context.Context, org, project string) ([]Hint, error)
	FirstLanded(ctx context.Context, org string, hintID int64) error // Resolver.FirstLanded
}

type Hint struct {
	ID       int64
	Conflict string           // cf-…
	First    model.AgentID    // Orchestrator of the side that lands first
	Second   model.AgentID
	Parties  [2]model.AgentID // the conflict's agentA, agentB (task 03's Conflict)
}

// Queue is one per fleetd; Advance runs under task 12's singleton loop, one
// project at a time, so at most one entry per project is ever past waiting.
type Queue struct {
	Store     Store     // task 10's store, the methods below
	Mirror    Mirror
	Machines  Machines
	Approvals Approvals
	Hints     Hints
	Agents    func(ctx context.Context, org string, id model.AgentID) (model.Agent, error) // task 10
	Notify    func(ctx context.Context, e Entry, template string) error                   // task 30's inform, Fleet-authored
	Now       func() time.Time
}

// Enqueue: the owner (from the token) asks to land one of her own lanes.
// agent_not_owner when the agent's OwnerUserID is not the caller; one live
// entry per worktree (a second call returns the first, 200 not 201).
func (q *Queue) Enqueue(ctx context.Context, org, project, ownerUserID string, r EnqueueRequest) (e Entry, created bool, err error)
func (q *Queue) Withdraw(ctx context.Context, org, project, ownerUserID, entry string) (Entry, error)
// Advance takes one step for one project (below) and returns what moved.
func (q *Queue) Advance(ctx context.Context, org, project string) ([]Entry, error)
// Claim: the owner's click. ready → pushing; returns Merge and Base for her
// push's --force-with-lease. Any other state → entry_not_live.
func (q *Queue) Claim(ctx context.Context, org, project, ownerUserID, entry string) (Entry, error)
// Report: what her machine's push did. Only the owner, only while pushing.
func (q *Queue) Report(ctx context.Context, org, project, ownerUserID, entry string, r PushReport) (Entry, error)
func (q *Queue) Resume(ctx context.Context, org, project, memberUserID string) error

type EnqueueRequest struct {
	Agent       model.AgentID `json:"agentId"`
	WorktreeID  string        `json:"worktreeId"`
	LaneTip     string        `json:"laneTip"`
	Branch      string        `json:"branch"`
	BriefKey    string        `json:"briefKey,omitempty"`
	BriefDigest string        `json:"briefDigest,omitempty"` // required with BriefKey
}

type PushReport struct {
	Outcome string `json:"outcome"` // "pushed" | "rejected" | "failed"
	Reason  string `json:"reason,omitempty"` // rejected: "remote_moved"; failed: git's class, ≤ 64, no text
}
```

```go
// control-plane/internal/fleet/landq/order.go
package landq

// Order returns one project's waiting entries in landing order, and for each
// held entry the cf-… holding it. An entry E is held by a live hint H iff
// E.Agent is one of H.Parties and parent(E.Agent) == H.Second (task 29's
// "second side"); held entries keep their place behind every free one that
// arrived before them. Otherwise order is EnqueuedAt, then ID. Pure.
func Order(waiting []Entry, hints []Hint, parent func(model.AgentID) model.AgentID) (order []Entry, heldBy map[string]string)
```

```go
// control-plane/internal/fleet/landq/gate.go
package landq

// Gate is task 49's step 9a over a mirror build (decision f). changes =
// b.Changes whose path converge.IsAgentConfigPath accepts, plus every
// package.json in b.PackageChanged with a non-empty member list. None → nil.
// Otherwise one approval a must have State "approved", LaneTip == e.LaneTip,
// ApproverUserID != e.OwnerUserID, and every change's path in a.Paths; else
// *Refusal{CodeSecondApproval, paths}. ListLandApprovals erroring →
// *Refusal{CodeHubUnavailable}: never "none needed".
func Gate(ctx context.Context, ap Approvals, e Entry, b LandBuilt) error

type Refusal struct {
	Code  string
	Paths []string
}

func (r *Refusal) Error() string { return "landq: " + r.Code }
```

**`Advance`, the order.** Every read of a sibling is before the state write it justifies.

1. Stopped (`fleet.land_queue_stops` has a row) → return `ErrStopped`; nothing moves.
2. A `ready` entry older than `ReadyFor` → `waiting`, `Reason: ready_expired`, `EnqueuedAt =
   now` (it yields the head), `Notify` once. A `building`, `ready` or `pushing` entry otherwise →
   return: one at a time. A `pushing` entry past its report deadline (10 minutes) is verified as
   step 9 below, and `push_unverified` if the remote does not show it.
3. `Order` the waiting entries; write `HeldBy` on each. No free head → return.
4. Head → `building`. `Mirror.FetchRemote(branch)` → `base`.
5. `Machines.Identity(owner)` → name and email, through `teamtext` (bounds, G3). The message is
   phase 3's `gitx.Message("Land <wt-id> into <branch>", …)` with task 09's trailers plus
   `Panacea-Owner: <ownerUserId>`. A brief key is re-resolved against the Hub's brief record; a
   digest that differs → `refused`, `brief_unresolvable`.
6. `Mirror.BuildLand`. `LaneMoved` → `refused`, `lane_moved`. `Conflicted` → `refused`,
   `merge_conflict`, `Paths`.
7. `Gate(e, built)` → a `*Refusal` → `refused` with its code and paths.
8. `Mirror.ExportRange(base, merge)` → `Machines.Stage(owner, …)` into `refs/panacea/landq/<id>`
   → `ready`, `ReadyAt = now`, `Notify(e, "land-ready")`.
9. On `Report{pushed}`: up to `VerifyAttempts` `FetchRemote(branch)` reads; `head == Merge` →
   `landed`, `Rejects = 0`, then `Hints.FirstLanded` for every live hint whose `First` is
   `parent(e.Agent)` and whose `Parties` hold `e.Agent`, then the refresh (below). Otherwise
   `refused`, `push_unverified`.
10. On `Report{rejected, remote_moved}`: `Rejects++`; `Rejects < StopAfter` → `waiting`, keeping
    `EnqueuedAt` (still the head, rebuilt on the new base). `Rejects == StopAfter` → `refused`,
    `remote_contended`, one stop row, `Notify` every member once. `Report{failed}` → `refused`,
    `push_failed`, `Reason` from the report.

**Refresh** (`refresh.go`, decision d): for each current member with a machine, `ExportRange(her
last told head for this branch, merge)` and `Machines.Refresh(member, …)` into
`refs/panacea/integration/<branch>`; `fleet.land_told` records the told head. A failure is
logged per member and retried on her machine's next start (task 20's start hook); it never
changes the entry's state.

**Fleet routes — complete wire.** One new verb in task 12's table, `VerbLandQueue Verb =
"landq.own"`, `{member: true, admin: true, own: true}`: the handler refuses an entry whose
`ownerUserId` is not the caller's. Reads use `VerbTeamRead`. Every route joins the Z5 matrix.

| Id | Route | Verb | Body → answer |
|---|---|---|---|
| F-L1 | `POST /v1/projects/{p}/landq` | `landq.own` | `EnqueueRequest` → 201 `Entry` (200 and the live one if the worktree has one); 403 `agent_not_owner`; 400 on a bad sha, ref or brief pair |
| F-L2 | `GET /v1/projects/{p}/landq` | `team.read` | → 200 `{"entries":[Entry…],"stopped":null\|{"reason","at"}}`, live first in `Order`, then the last 20 finished; a viewer sees the same |
| F-L3 | `DELETE /v1/projects/{p}/landq/{entry}` | `landq.own` | → 200 `Entry` `withdrawn`; `pushing` → 409 `entry_not_live` (the push is already hers to finish) |
| F-L4 | `POST /v1/projects/{p}/landq/{entry}/claim` | `landq.own` | `{}` → 200 `{"entry":Entry,"ref":"refs/panacea/landq/<id>","merge","base","branch"}`; not `ready` → 409 `entry_not_live` |
| F-L5 | `POST /v1/projects/{p}/landq/{entry}/report` | `landq.own` | `PushReport` → 200 `Entry` after step 9 or 10; not `pushing` → 409 |
| F-L6 | `POST /v1/projects/{p}/landq/resume` | `landq.own` without `own` narrowing (any member) | `{}` → 204; clears the stop row; the stop and the resume each log the caller |

```json
{"id":"lq-5d4c3b2a19087f6e","orgId":"org-1a2b3c4d5e6f7081","projectId":"pj-0f1e2d3c4b5a6978",
 "branch":"main","ownerUserId":"u-5b4c3d2e1f0a9b8c","agentId":"ag-9e8d7c6b5a493827",
 "worktreeId":"wt-3c2b1a0f","laneRef":"refs/panacea/lanes/7f3a9c1e/wt-3c2b1a0f",
 "laneTip":"4b825dc642cb6eb9a060e54bf8d69288fbee4904","briefKey":"","briefDigest":"",
 "state":"refused","reason":"second_approval_required","paths":[".claude/settings.json"],
 "base":"9fceb02d0ae598e95dc970b74767f19372d61af8","merge":"","heldBy":"","rejects":0,
 "enqueuedAt":"2026-09-25T09:41:07Z","readyAt":"0001-01-01T00:00:00Z","updatedAt":"2026-09-25T09:41:09Z"}
```

`readyAt` zero is written as Go's zero time; the desktop reads `state`, never `readyAt`, to decide
whether it is ready (gap 4 asks whether to make it `null`).

**The push act, on the local control plane** (`api/teamland.go`). `POST
/api/projects/{id}/team/land/{entry}` (loopback, phase 2's auth): (1) F-L4 through `fleetbe`
(task 21) → merge, base, ref, branch; (2) `credstore.Store.Credential(host)` (phase 3 task 13) →
the owner's own token, in memory; (3) through `backend.Backend` (S8.1) to her own machine,
phase 3 task 12's push with `--force-with-lease=refs/heads/<branch>:<base>` of `<ref>` to
`refs/heads/<branch>`, run by `machined` outside the VM (global constraint 8: no git credential
enters a sandbox); (4) F-L5 with the outcome. The token is never in a request body Fleet can
see: `credstore.Credential.MarshalJSON` refuses (`parts/task-13.md:63-72`), and Fleet is not on
the path. **How phase 8 carries a per-act token to a cloud `machined` is not in any sibling plan**
(gap 1); this part needs one header on the broker's machine RPC, read into memory, never logged.

**Migration** (task 10's directory, after its last):

```sql
CREATE TABLE IF NOT EXISTS fleet.land_queue (
  org_id text NOT NULL, entry_id text NOT NULL, project_id text NOT NULL,
  branch text NOT NULL, owner_user_id text NOT NULL, agent_id text NOT NULL,
  worktree_id text NOT NULL, lane_ref text NOT NULL, lane_tip text NOT NULL,
  brief_key text NOT NULL DEFAULT '', brief_digest text NOT NULL DEFAULT '',
  state text NOT NULL CHECK (state IN
    ('waiting','building','ready','pushing','landed','refused','withdrawn')),
  reason text NOT NULL DEFAULT '', paths jsonb NOT NULL DEFAULT '[]',
  base text NOT NULL DEFAULT '', merge text NOT NULL DEFAULT '',
  held_by text NOT NULL DEFAULT '', rejects integer NOT NULL DEFAULT 0 CHECK (rejects >= 0),
  enqueued_at timestamptz NOT NULL, ready_at timestamptz, updated_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, entry_id));
-- one live entry per lane, and at most one past waiting per project (decision e)
CREATE UNIQUE INDEX IF NOT EXISTS land_queue_live_lane ON fleet.land_queue (org_id, project_id, worktree_id)
  WHERE state IN ('waiting','building','ready','pushing');
CREATE UNIQUE INDEX IF NOT EXISTS land_queue_one_active ON fleet.land_queue (org_id, project_id)
  WHERE state IN ('building','ready','pushing');
CREATE TABLE IF NOT EXISTS fleet.land_queue_stops (
  org_id text NOT NULL, project_id text NOT NULL, reason text NOT NULL,
  stopped_at timestamptz NOT NULL, PRIMARY KEY (org_id, project_id));
CREATE TABLE IF NOT EXISTS fleet.land_told (
  org_id text NOT NULL, project_id text NOT NULL, branch text NOT NULL,
  member_user_id text NOT NULL, told_head text NOT NULL, told_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, project_id, branch, member_user_id));
```
