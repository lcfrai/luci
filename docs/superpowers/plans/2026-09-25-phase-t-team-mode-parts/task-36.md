### Task 36: Tier-H forecasts and co-edit down-weighting

**Slice:** TM1. **Focused time:** 2 h. **dependsOn:** 25 (`Forecaster`, `Raise`, `Quiet`,
`Open`), 35 (the activity table and its call into the forecaster); 34 (ST.6's hunks) through 35.

Tier H is §8.2's second tier: **overlapping hunks on the same base blob**. Tier P says two agents
of different owners touch the same file; tier H says they touch the same **lines of the same
version** of it, which is the signal that a text merge will actually fail. It needs no content:
`machined`'s sweep (task 34) reports, per dirty path, the base blob sha and `[start, end]` line
pairs, and that is all this task reads.

**Say it plainly: in TM1 tier H mostly collapses to tier P** (`05-design.md` §8.2, `06` Minor 4).
Two members' worktrees start on the same base, so their hunks are comparable at first. After
either of them lands, the other's base blob for that file is the old one and the lander's next
edit is on the new one, and nothing in TM1 re-bases line ranges across blobs (that is the lane
mirror, TM2's tier M, task 60). Different base blobs raise **nothing** at H by design; tier P
still raises on the shared path. So in TM1 tier H adds line ranges to the conflicts P already
raised in a pair's first hours, and little more. It earns its keep in TM2. The part does not
claim more than that.

The second half is §8.2's **noise** rule, after Microsoft's ConE (Maddila et al., TOSEM 2022,
cited by §8.2): warn rarely, and down-weight files that are often edited at the same time. A
lockfile, a generated file, or a file that most lanes touch (a route table, a changelog) raises
**nothing at tier P**. It still raises at H, because two agents rewriting the same lines of the
same version of even a noisy file is a real merge failure, and at M (task 60).

Four decisions the scope left open, made here so nobody re-derives them.

**(a) Overlap includes adjacency.** Two ranges overlap when `a.start ≤ b.end+1` and
`b.start ≤ a.end+1`. A three-way text merge treats changes to adjacent lines, with no unchanged
line between them, as one conflicting region; strict intersection would miss exactly the case a
reviewer sees most. Whether git's default `merge` driver conflicts on every adjacent pair is
**UNVERIFIED** here (no `gitx` code exists yet to run it against); `TestAdjacentRangesOverlap`
pins the rule, and task 60's `merge-tree` result is the ground truth that can overturn it.

**(b) The co-edit rate is per project, per path, over lanes, not over sweeps.** A sweep every
5 to 10 s would count a file one agent keeps open as "often edited". The unit is an **agent**:
`rate(path) = agents whose observed commits touched path ÷ agents with any observed commit`, over
a rolling 14 days. A path is **noisy** when that rate is at least `0.6` and at least `5` agents
committed in the window. Both numbers are constants in `coedit.go`, not configuration: a team
admin tuning noise per project is a TM2 question, and a knob nobody asked for fails the bar.

**(c) The co-edit counts are Fleet state in Postgres, not memory.** The global constraint allows
only activity and presence in memory. A rate lost on every restart would make tier P noisy for
days after each deploy. The table holds `(project, path, agent, last_commit_at)`, no content and
no text, is pruned to 14 days by the existing singleton sweep (task 12), and is rebuilt by
nothing: it is a statistic that re-accumulates, so losing it costs only noise, never a missed
conflict (tier H does not consult it).

**(d) HORIZON items come from the working copy, not from a second list.** Task 54's HORIZON row
is *"collisions between agents"* (`docs/AGENTIC-UX.md:149`, cited by §9.3), kept until the
conflict is `resolved` or `cleared` and **never dropped on `accepted`**. Task 25's `Open` already
keeps every open conflict; `Horizon` reads the same working copy and adds the one thing `Open`
drops: an `accepted` conflict stays a HORIZON item until it is `resolved` or `cleared`. The design's
example line ends *"~20 min"* (§9.3 table); **OD-27 forbids ETAs**, so a HORIZON item carries
tier, paths, line ranges and the two agents, and no time estimate. The decision wins.

**Files:**
- `control-plane/internal/fleet/forecast/tierh.go` — `HunkSignal`, `OnHunks`, `overlaps`, the
  same-blob rule (new)
- `control-plane/internal/fleet/forecast/coedit.go` — `CoEdit`, `Generated`, `Noisy` (new)
- `control-plane/internal/fleet/forecast/horizon.go` — `HorizonItem`, `(*Forecaster).Horizon` (new)
- `control-plane/internal/fleet/forecast/{tierh,coedit,horizon}_test.go` (new)
- `control-plane/internal/fleet/forecast/tierp.go` — `Raise` step (2) drops quiet **and noisy**
  paths **at tier P only**; `Deps` gains `CoEdit` (modify, task 25's file, two anchored hunks)
- `control-plane/internal/fleet/forecast/conflicts.go` — `Open` gains `Lines` per path (modify)
- `control-plane/internal/fleet/store/migrations/00NN_coedit.sql` — numbered after the last Fleet
  migration at merge (new)
- `control-plane/internal/fleet/store/{memstore,pgstore}.go`, `.../storetest/suite.go` — the
  three `CoEditStore` methods (modify, task 10's files)
- `control-plane/cmd/fleetd/main.go` — passes `CoEdit` into `forecast.Deps`; registers the prune
  with task 12's singleton loop (modify, one anchored hunk after task 25's `forecast.New`)

**Interfaces:**
- Produces: `(*Forecaster).OnHunks` (task 35's second call per sample), `forecast.HunkSignal`,
  `forecast.Range`; `forecast.CoEdit` with `Observe` and `Noisy`; `forecast.Generated(path) bool`;
  `(*Forecaster).Horizon(ctx, project) []HorizonItem`, consumed by task 52's read model for task
  54's HORIZON row.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 25: `Forecaster`, `Deps`, `Signal`, `Raise`, `Quiet`, the working copy `Open`
    (`task-25.md:64-198`);
  - task 22: `verbs.Hunk{Path, Base, Ranges [][2]int}` (`task-22.md:149-153`), the reduced ST.6
    shape, whose `Ranges` are `[start, end]` pairs (`task-16.md:246-250`);
  - task 35: `Activity.Hunks(ctx, project) []AgentHunks`, every live agent's current observed
    hunks, and the call `OnHunks` after each sample (**not yet written; required of task 35**);
  - task 34: ST.6's `lastCommit` with its sha and changed paths, the co-edit input
    (**not yet written; required of task 34**, see the gaps);
  - task 06: route 57's `paths[].lines`, `≤ 20` ranges, `1 ≤ start ≤ end` (`task-06.md:203-205`);
  - task 12: `server.NewSingleton(pool, name, log)` and `(*Singleton).Run`
    (`task-12.md:70`, `:588-597`);
  - task 10: `store.Store.Tx` (`task-10.md:90-91`).

#### `tierh.go` (complete)

```go
package forecast

import (
	"context"
	"sort"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/verbs"
)

// Range is one [start, end] line pair, 1-based, inclusive, on a base blob.
type Range = [2]int

// HunkSignal is one agent's observed dirty and committed hunks, from one ST.6
// sample. Base is per path, so it lives on each hunk, not on the signal.
type HunkSignal struct {
	Agent model.AgentID
	Owner model.Owner
	Head  string // last commit sha, or ""
	Hunks []verbs.Hunk
}

// AgentHunks is task 35's view of another live agent's hunks.
type AgentHunks struct {
	Agent model.AgentID
	Owner model.Owner
	Head  string
	Hunks []verbs.Hunk
}

// Activity is the one method tier H reads from task 35's table.
type Activity interface {
	Hunks(ctx context.Context, project string) []AgentHunks
}

// OnHunks intersects s with every other owner's live hunks on project. For
// each other agent with at least one overlap on the same path AND the same
// base blob, it calls Raise once with tier "H" and the overlapping paths, each
// carrying the union of both sides' overlapping ranges. Different base blobs
// raise nothing here; tier P (OnPaths) still sees the shared path.
func (f *Forecaster) OnHunks(ctx context.Context, org, project string, s HunkSignal) error {
	others := f.d.Activity.Hunks(ctx, project)
	sort.Slice(others, func(i, j int) bool { return others[i].Agent < others[j].Agent })
	for _, o := range others {
		if o.Agent == s.Agent || o.Owner == s.Owner {
			continue
		}
		lines := sharedLines(s.Hunks, o.Hunks)
		if len(lines) == 0 {
			continue
		}
		a := Signal{Agent: s.Agent, Owner: s.Owner, Head: s.Head, Paths: keys(lines)}
		b := Signal{Agent: o.Agent, Owner: o.Owner, Head: o.Head, Paths: keys(lines)}
		if _, err := f.raise(ctx, org, project, "H", a, b, keys(lines), lines); err != nil {
			return err
		}
	}
	return nil
}

// sharedLines returns, per path, the merged ranges where a and b overlap on
// the same path and the same base blob. A path whose bases differ is absent.
func sharedLines(a, b []verbs.Hunk) map[string][]Range {
	type key struct{ path, base string }
	idx := map[key][]Range{}
	for _, h := range b {
		idx[key{h.Path, h.Base}] = append(idx[key{h.Path, h.Base}], h.Ranges...)
	}
	out := map[string][]Range{}
	for _, h := range a {
		theirs, ok := idx[key{h.Path, h.Base}]
		if !ok || h.Base == "" {
			continue // unknown base is never "the same" base
		}
		for _, r := range h.Ranges {
			for _, t := range theirs {
				if overlaps(r, t) {
					out[h.Path] = append(out[h.Path], Range{min(r[0], t[0]), max(r[1], t[1])})
				}
			}
		}
	}
	for p, rs := range out {
		out[p] = mergeRanges(rs) // sorted, touching ranges joined, ≤ 20 kept for route 57
	}
	return out
}

// overlaps is decision (a): intersection or adjacency.
func overlaps(a, b Range) bool { return a[0] <= b[1]+1 && b[0] <= a[1]+1 }
```

`mergeRanges` sorts by start and joins any two ranges that `overlaps` holds for; past 20 ranges on
one path it keeps the first 19 and folds the rest into one `[start₂₀, maxEnd]` range, so the Hub
never refuses a wide overlap with a 422 and nothing is dropped from the covered span. `keys`
returns a map's keys sorted. `min` and `max` are Go 1.21 built-ins, inside the 1.23 floor.

#### `tierp.go` (modify: two anchored hunks in task 25's file)

Task 25's `Raise` keeps its signature and becomes a wrapper; the body moves to `raise`, which takes
the per-path line ranges route 57 already accepts. Step (2) changes in one line: quiet and noisy
paths are dropped **only when `tier == "P"`**.

```go
type Deps struct {
	// … task 25's fields, unchanged …
	Activity Activity // task 35; nil until 35 is wired, and OnHunks is then never called
	CoEdit   *CoEdit  // nil means: nothing is noisy (the TM1-α state)
}

func (f *Forecaster) Raise(ctx context.Context, org, project, tier string,
	a, b Signal, paths []string) (string, error) {
	return f.raise(ctx, org, project, tier, a, b, paths, nil)
}

// raise is task 25's eight steps, with lines carried into route 57's
// paths[].lines and into the working copy. Step (2):
//
//	if tier == "P" { paths = dropNoise(project, paths) } // Quiet, Generated, Noisy
//
// Step (4), an open conflict for the pair: merge paths AND lines into the
// working copy, and raise Strongest to tier if tier is stronger. No second
// cf-… and no re-send: route 58 cannot patch paths or tier (task-06.md:210-215).
func (f *Forecaster) raise(ctx context.Context, org, project, tier string,
	a, b Signal, paths []string, lines map[string][]Range) (string, error)
```

The strength order is `P < H < M < S`. A tier-H `raise` on a pair with an open tier-P conflict
therefore changes the working copy's `Strongest` to `H` and adds the ranges, and nothing else.

#### `conflicts.go` (modify: two fields on `Open`)

```go
type Open struct {
	// … task 25's fields, unchanged; Tier stays the tier the cf-… was raised at …
	Strongest string             // the strongest tier observed for this pair since
	Lines     map[string][]Range // per path, merged; empty at tier P
}
```

`(*Forecaster).Open(ctx, project)` fills `verbs.Conflict.Tier` from `Strongest`, not from the Hub
record's tier. That is what makes `panacea check` say `H` once the lines overlap, and it is what
task 30's `proceed-both` row must read (`task-30.md:239-243` switches on `a.Tier`): a pair raised
at P whose hunks later overlap on one blob is a `proceed-both-hm` decide, never an act. This is a
requirement on task 30's caller, listed in the gaps; task 30's table itself does not change.

