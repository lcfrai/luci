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
	OwnerUserID string        `json:"ownerUserId"` // us-…
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
