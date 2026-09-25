### Task 60: Tier-M forecasts; lane bundles into the mirror

**Slice:** TM2. **Focused time:** 2.5 h. **dependsOn:** 34 (ST.6's `lastCommit`, **not yet
written; required of task 34**), 36 (`raise` with lines, the strength order `P < H < M < S`,
`Open.Strongest`, `Open.Lines`, `dropNoise`, `task-36.md:194-241`), 59 (ST.7, `mirror.Client`,
the runner's `ingest` and `pairs`); through 59, task 58's pod and relay; and task 25's
`Forecaster`, `Signal`, `Raise`, `Open` (`task-25.md:64-202`). **Seams:** consumes ST.6 and ST.7.

Tier M is the exact answer: `git merge-tree` between two lanes of different owners, in the mirror
(`05-design.md` §8.2). This task feeds the mirror and asks it. After each commit on a team lane,
the lane owner's `machined` exports the lane as a bundle (phase 3 ruling R4: lane work crosses as
bundles) and the broker carries it into the project's mirror. Fleet then asks tier M **only for the
pairs that tier P or H already flagged**, so the work grows with activity, not with the square of
the team. The answer raises or updates a `cf-…` at tier `M`, or marks an open one *mergeable*.

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) "Flagged" means: an open conflict for the pair, or a live path overlap tier P saw.** Task 36
drops noisy paths (lockfiles, generated files, co-edited files) **at tier P only**, and says they
still raise at H and at M (`task-36.md:21-25`). A pair whose only shared paths are noisy therefore
has no open conflict, and a rule of "open conflicts only" would never ask M about it. So a pair is
flagged when it has an open conflict **or** the two agents' current observed paths intersect in
task 35's activity table, noisy or not. Both are read at the moment of the commit, from state
that exists already (the working copy and the 45-second activity table), so no new in-memory set
is kept (global constraint: only activity and presence are in memory).

**(b) Only pairs that include the committing lane are asked.** A commit changes one lane. Every
other flagged pair's answer is unchanged by it, so a commit on lane `L` asks M for at most the
flagged pairs `(L, X)`: one per teammate agent whose work overlaps `L`'s. With `k` overlapping
agents that is `k` merge-trees, never `n²`.

**(c) What an answer does.** For a pair `(a, b)` whose answer is valid (ST.7's `Validate`) and
**current** (its `basis.a` and `basis.b` are still the two lanes' latest ingested tips; otherwise
it is dropped, since a newer commit's ask follows):
- `conflicts` → `raise(…, "M", a, b, paths, lines)` with the conflicted paths and every path
  whose `lines` are non-empty, and M's lines. Noise is **not** dropped at M. On an open conflict
  this moves `Strongest` to `M` and replaces the pair's lines with M's, which are exact on the
  merge base (task 36's step (4) merge; tier M's lines win over H's for the same path).
- `clean` with paths both sides changed → if the pair has an open conflict, its working copy is
  marked `Mergeable: true` with `MBasis`, and nothing is sent to the Hub (route 58 cannot patch
  tier or paths, `task-36.md:217-219`). If it has none, the paths are filtered through
  `dropNoise`, and any left raise at tier `P` marked `Mergeable`, so a clean textual merge on
  overlapping real paths **still yields a conflict record**, and a lockfile that merges cleanly
  still raises nothing (the scope's second test, and task 36's noise rule).
- `clean` with no shared paths → an open conflict for the pair is marked `Mergeable`; nothing else.
- `unknown` → nothing changes; the pair stays at its tier. Fleet logs the code.

**(d) Mergeable is Fleet state.** Whether two lanes merge cleanly *now* is a fact about now, and
changes with the next commit, so it lives in the working copy (the placement rule, global
constraint 1), not in the Hub record. `panacea check` shows it (*"overlap on auth/session.go;
merges cleanly as of 3f2a…"*) through `verbs.Conflict`, which gains one field (gap 2). Task 30's
`proceed-both` rule reads `Strongest`: a pair at `P` that M found mergeable stays `P`, and a pair M
found conflicting is `M`, which task 30 already treats as a `decide` (`task-36.md:237-241`).

**(e) The bytes never pass through Fleet.** Fleet sends one instruction to the broker, *bundle this
owner's lane into this project's mirror*, under the lane owner's delegation; the broker reads the
bundle from `machined` and writes it into the mirror's `/control`, copying and never decoding, as
it does for attach (task 17). Fleet receives only the mirror's `IngestResult`. So no hostile
object is ever in Fleet's memory, which holds every member's delegations (`07` I11).

**(f) One send per lane per 10 seconds.** Agents commit in bursts. A commit arriving while a send
for the same lane is in flight, or within 10 s of the last, sets a *pending* mark; the sender sends
once more when the window closes, with the newest tip. The window is a constant (`SendEvery`).

**(g) The basis is what the mirror already holds.** Each lane bundle is asked `over` the lane's
last ingested tip and the mirror's base tip, the shas Fleet recorded from earlier
`IngestResult`s, which phase 3's runner drops if the lane lacks them (`task-24.md:794-796`). So the
bundle carries the lane *since what the mirror has*, which is at most *since its merge base* once
the base is in. When the mirror answers `need-base`, Fleet asks the same owner's `machined` for a
base bundle of the project's default branch (kind `base`, task 59) over the mirror's base tip, then
resends the lane bundle once. A second `need-base` for the same lane clears Fleet's record of what
the mirror holds for the project and resends with an empty basis: the mirror lost its disk.

**Files:**
- `control-plane/internal/fleet/forecast/tierm.go` — `MirrorAsker`, `LaneOf`, `OnCommit`,
  `flaggedWith`, `applyM` (new)
- `control-plane/internal/fleet/forecast/tierm_test.go` (new)
- `control-plane/internal/fleet/forecast/conflicts.go` — `Open` gains `Mergeable`, `MBasis`;
  `Open(ctx, project)` fills `verbs.Conflict.Mergeable` (modify, task 25's file, one hunk)
- `control-plane/internal/fleet/forecast/tierp.go` — `Deps` gains `Mirror MirrorAsker` and
  `Lanes LaneOf` (modify, task 36's `Deps` hunk)
- `control-plane/internal/fleet/mirror/sender.go`, `sender_test.go` — `Sender`: coalescing,
  basis, need-base, the ingested-tips record (new)
- `control-plane/internal/fleet/store/migrations/00NN_mirror.sql` — `fleet.mirror_tips`, numbered
  after the last Fleet migration at merge (new); `store/{memstore,pgstore}.go`, `storetest/suite.go`
  gain `MirrorTipStore`'s three methods (modify, task 10's files)
- `control-plane/cmd/broker/mirrorbundles.go`, `mirrorbundles_test.go` — `POST
  /v1/mirrors/{projectId}/bundles` (new, beside task 58's `mirrors.go`)
- `control-plane/cmd/fleetd/main.go` — builds the `Sender` and passes `Mirror` and `Lanes` into
  `forecast.Deps`; task 35's sample handler calls `OnCommit` when `lastCommit` changes (modify)

**Interfaces:**
- Produces: `forecast.MirrorAsker`, `forecast.LaneOf`, `(*Forecaster).OnCommit`,
  `Open.Mergeable`, `Open.MBasis`; `mirror.Sender` with `Send`; `store.MirrorTipStore`; the
  broker's bundle route.
- Consumes, **written against these names; where the owning task spells one differently, it wins
  and this part is renamed mechanically**:
  - task 25: `Forecaster`, `Deps`, `Signal`, `Open` (`task-25.md:79-197`);
  - task 36: `raise(ctx, org, project, tier, a, b, paths, lines)`, `dropNoise`, `Range`,
    `Activity.Hunks`, `AgentHunks` (`task-36.md:107-222`);
  - task 35 (**not yet written; required of task 35**): `Activity.Paths(ctx, project)
    []AgentPaths{Agent, Owner, Paths}`, every live agent's observed paths, and a call to
    `OnCommit` when a sample's `lastCommit.sha` differs from the agent's previous one;
  - task 34 (**not yet written; required of task 34**): ST.6's `lastCommit{sha, at}` and the
    sample's `worktree` and `machine` ids;
  - task 59: `mirror.LaneKey`, `OwnerHash`, `PairRequest`, `PairResult`, `IngestResult`,
    `Client.Pairs`, and the codes;
  - task 17: `principalOf`, the delegation check, the relay's copy loop (`task-17.md:43-73`,
    `:342`); task 58: the mirror manager and its control relay;
  - phase 8 tasks 20–21 (**UNVERIFIED until merge**, planned): `machined`'s lane runner `Bundle`
    op (phase 3's S3-P8.1 contract, `phase-3 …/task-24.md:168-178`), and a project-repository
    bundle of one branch in phase 3's `SendBranch` shape (`task-24.md:742-745`).

#### `tierm.go` (complete)

```go
package forecast

import (
	"context"
	"log/slog"
	"sort"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/mirror"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// MirrorAsker is what tier M needs: send the lane in, then ask pairs. The
// production value is mirror.Sender plus mirror.Client; tests use a fake that
// records every pair it is asked.
type MirrorAsker interface {
	Send(ctx context.Context, org, project string, lane mirror.LaneKey, owner model.Owner,
		machine, head string) (tip string, err error) // "" tip: not ingested this time
	Pairs(ctx context.Context, project string, pairs []mirror.PairRequest) ([]mirror.PairResult, error)
	Tip(ctx context.Context, project string, lane mirror.LaneKey) (string, bool) // last ingested
}

// LaneOf maps a live agent to its lane (the Hub agent record's laneRef, task
// 06) and back. ok is false for an Orchestrator, which has no lane.
type LaneOf interface {
	Lane(ctx context.Context, a model.AgentID) (k mirror.LaneKey, owner model.Owner, ok bool)
}

// CommitSignal is one ST.6 sample whose lastCommit moved.
type CommitSignal struct {
	Agent   model.AgentID
	Owner   model.Owner
	Machine string // the owner's machine that holds the lane
	Head    string // "refs/heads/<branch>" of the lane
	Commit  string // lastCommit.sha
}

// OnCommit is decision (b): send the lane, then ask M for the flagged pairs
// that include it, and apply each current answer (decision (c)).
func (f *Forecaster) OnCommit(ctx context.Context, org, project string, s CommitSignal) error {
	if f.d.Mirror == nil || f.d.Lanes == nil {
		return nil // TM1: no mirror; tiers P and H only
	}
	key, _, ok := f.d.Lanes.Lane(ctx, s.Agent)
	if !ok {
		return nil
	}
	tip, err := f.d.Mirror.Send(ctx, org, project, key, s.Owner, s.Machine, s.Head)
	if err != nil || tip == "" {
		return err // coalesced (decision (f)) or refused; the sender logged which
	}
	others := f.flaggedWith(ctx, project, s.Agent, s.Owner)
	if len(others) == 0 {
		return nil
	}
	var reqs []mirror.PairRequest
	var peers []model.AgentID
	for _, o := range others {
		k2, _, ok := f.d.Lanes.Lane(ctx, o)
		if !ok {
			continue
		}
		if _, held := f.d.Mirror.Tip(ctx, project, k2); !held {
			continue // the teammate's lane is not in the mirror yet; its own commit will ask
		}
		reqs, peers = append(reqs, mirror.PairRequest{A: key, B: k2}), append(peers, o)
	}
	if len(reqs) == 0 {
		return nil
	}
	res, err := f.d.Mirror.Pairs(ctx, project, reqs)
	if err != nil {
		return err
	}
	for i, r := range res {
		f.applyM(ctx, org, project, s.Agent, peers[i], reqs[i], r)
	}
	return nil
}

// flaggedWith is decision (a): every other-owner agent with an open conflict
// with a, or whose live observed paths meet a's, noisy or not. Sorted.
func (f *Forecaster) flaggedWith(ctx context.Context, project string, a model.AgentID,
	owner model.Owner) []model.AgentID {
	set := map[model.AgentID]bool{}
	for _, c := range f.Open(ctx, project) {
		if c.A == a && f.ownerOf(ctx, c.B) != owner {
			set[c.B] = true
		}
		if c.B == a && f.ownerOf(ctx, c.A) != owner {
			set[c.A] = true
		}
	}
	live := f.d.Activity.Paths(ctx, project) // task 35
	var mine map[string]bool
	for _, p := range live {
		if p.Agent == a {
			mine = toSet(p.Paths)
		}
	}
	for _, p := range live {
		if p.Agent != a && p.Owner != owner && intersects(mine, p.Paths) {
			set[p.Agent] = true
		}
	}
	out := make([]model.AgentID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
```

`ownerOf` reads task 10's `Store` agent row, as `Raise` step (3) already does; `toSet` and
`intersects` are three-line helpers. `f.Open` returns `verbs.Conflict`, whose `A` and `B` are agent
ids (`task-25.md:200`). A flagged pair whose teammate lane the mirror does not hold yet is skipped,
not waited for: the teammate's own next commit sends that lane and asks the same pair from the
other side, so no pair is lost and none blocks. `f.d.Activity` is task 36's `Activity` interface,
which gains one method here, `Paths(ctx, project) []AgentPaths` (a one-line hunk in `tierh.go`;
task 35 implements both).

```go
// applyM is decision (c). r has already passed ST.7's Validate inside
// mirror.Client (task 59); here it is also checked for currency.
func (f *Forecaster) applyM(ctx context.Context, org, project string, a, b model.AgentID,
	q mirror.PairRequest, r mirror.PairResult) {
	ta, okA := f.d.Mirror.Tip(ctx, project, q.A)
	tb, okB := f.d.Mirror.Tip(ctx, project, q.B)
	if r.State == "unknown" || !okA || !okB || r.Basis.A != ta || r.Basis.B != tb {
		slog.Debug("tier M answer not applied", "project", project, "state", r.State, "code", r.Code)
		return // stale or unanswerable: a newer commit's ask follows
	}
	var paths []string
	lines := map[string][]Range{}
	for _, p := range r.Paths {
		if p.Conflicted || len(p.Lines) > 0 {
			paths = append(paths, p.Path)
			for _, l := range p.Lines {
				lines[p.Path] = append(lines[p.Path], Range{l[0], l[1]})
			}
		}
	}
	sa := Signal{Agent: a, Owner: f.ownerOf(ctx, a), Head: r.Basis.A, Paths: paths}
	sb := Signal{Agent: b, Owner: f.ownerOf(ctx, b), Head: r.Basis.B, Paths: paths}
	if r.State == "conflicts" {
		_, _ = f.raise(ctx, org, project, "M", sa, sb, paths, lines) // errors are raise's to log
		f.setMergeable(project, a, b, false, r.Basis)
		return
	}
	// clean
	if f.setMergeable(project, a, b, true, r.Basis) {
		return // an open conflict: marked, nothing sent (route 58 cannot patch tier or paths)
	}
	var shared []string
	for _, p := range r.Paths {
		shared = append(shared, p.Path)
	}
	if len(shared) == 0 {
		return
	}
	sa.Paths, sb.Paths = shared, shared
	if id, _ := f.raise(ctx, org, project, "P", sa, sb, shared, nil); id != "" { // raise drops noise at P
		f.setMergeable(project, a, b, true, r.Basis)
	}
}
```

`setMergeable(project, a, b, m, basis) bool` sets `Mergeable` and `MBasis` on the pair's open
working copy, in either agent order, and reports whether one existed. A `clean` answer after a
`conflicts` one (the agents fixed it) sets `Mergeable: true` but does not lower `Strongest`: the
strength order only rises within one conflict (`task-36.md:224-225`), and task 29's resolution is
what closes it. Clearing a conflict because it now merges is a resolution, which belongs to the
two Orchestrators, not to the forecaster.

#### `conflicts.go` (modify: two fields on `Open`)

```go
type Open struct {
	// … task 25's and task 36's fields, unchanged …
	Mergeable bool         // tier M's last current answer was clean (decision (d))
	MBasis    mirror.Basis // the two tips and merge base that answer was for; zero before any M
}
```

`(*Forecaster).Open` fills `verbs.Conflict.Mergeable` from it. That field does not exist in task
22's `verbs.Conflict` (`task-25.md:200`) or in ST.1's `check` answer; both gain `mergeable`
(boolean, default `false`) in their owners' files (gap 2).
