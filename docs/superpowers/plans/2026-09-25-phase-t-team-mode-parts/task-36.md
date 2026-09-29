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
  - task 06: route 57's `paths[].lines`, `≤ 20` ranges, `1 ≤ start ≤ end` (`task-06.md:286-288`);
  - task 12: `server.NewSingleton(pool, name, log)` and `(*Singleton).Run`
    (`task-12.md:102`, `:635-644`);
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
// cf-… and no re-send: route 58 cannot patch paths or tier (task-06.md:293-298).
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

#### `coedit.go` (complete)

```go
package forecast

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// Decision (b). Constants, not configuration.
const (
	CoEditWindow   = 14 * 24 * time.Hour
	CoEditMinRate  = 0.6
	CoEditMinLanes = 5
)

// CoEditStore is the three methods task 10's memstore and pgstore gain.
type CoEditStore interface {
	// ObserveCommit upserts (project, path, agent) → at for every path.
	ObserveCommit(ctx context.Context, org, project string, agent model.AgentID,
		paths []string, at time.Time) error
	// Rates returns, for the given paths, how many distinct agents committed
	// each since `since`, and how many distinct agents committed anything.
	Rates(ctx context.Context, org, project string, paths []string,
		since time.Time) (perPath map[string]int, lanes int, err error)
	// PruneCoEdit deletes rows older than before, across every project.
	PruneCoEdit(ctx context.Context, before time.Time) (int64, error)
}

type CoEdit struct {
	S   CoEditStore
	Now func() time.Time
}

// Observe is task 35's call when a sample's last-commit sha is new for that
// agent: every changed path of that commit, once.
func (c *CoEdit) Observe(ctx context.Context, org, project string,
	agent model.AgentID, paths []string) error {
	return c.S.ObserveCommit(ctx, org, project, agent, paths, c.Now())
}

// Noisy returns the subset of paths tier P must not raise on: rate ≥ 0.6
// with ≥ 5 lanes in the window. A store error returns nil (nothing noisy):
// failing open here means one extra conflict, never a missed one.
func (c *CoEdit) Noisy(ctx context.Context, org, project string, paths []string) map[string]bool {
	per, lanes, err := c.S.Rates(ctx, org, project, paths, c.Now().Add(-CoEditWindow))
	if err != nil || lanes < CoEditMinLanes {
		return nil
	}
	out := map[string]bool{}
	for _, p := range paths {
		if float64(per[p])/float64(lanes) >= CoEditMinRate {
			out[p] = true
		}
	}
	return out
}

// Generated reports a path that is, by name alone, machine-written. The
// table is fixed so a reviewer can read all of it; content is never read.
func Generated(p string) bool {
	base := path.Base(p)
	for _, s := range []string{".pb.go", "_gen.go", ".gen.go", ".pb.ts", "_pb2.py",
		".min.js", ".min.css", ".snap", ".designer.cs", ".g.cs", ".g.dart"} {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	return strings.HasPrefix(base, "zz_generated.") ||
		strings.Contains(base, ".generated.")
}
```

"Fails open" is deliberate and one-directional: a co-edit store outage makes tier P **louder**,
never quieter, so an outage can cost the humans an extra thread but never hide an overlap. Task
25's `Quiet` (lockfiles) and `Generated` read no state and cannot fail.

`dropNoise` in `tierp.go` is `Quiet(p) || Generated(p) || noisy[p]`, with `noisy` from one
`Noisy` call per `raise`. Tier H and task 60's tier M never call it.

#### `00NN_coedit.sql` (complete; `NN` is the next Fleet migration number at merge)

```sql
CREATE TABLE fleet.coedit (
  org_id          text        NOT NULL,
  project_id      text        NOT NULL,
  path            text        NOT NULL CHECK (char_length(path) BETWEEN 1 AND 200),
  agent_id        text        NOT NULL CHECK (agent_id ~ '^ag-'),
  last_commit_at  timestamptz NOT NULL,
  PRIMARY KEY (org_id, project_id, path, agent_id)
);
CREATE INDEX coedit_by_time ON fleet.coedit (last_commit_at);
CREATE INDEX coedit_by_project ON fleet.coedit (org_id, project_id, last_commit_at);
```

`Rates` is two queries in one read-only transaction: `count(DISTINCT agent_id) … GROUP BY path`
for the asked paths, and `count(DISTINCT agent_id)` over the project, both `WHERE last_commit_at
>= $since`. The 200-character check is task 02's path cap, repeated so the table enforces it too.
`ObserveCommit` is one `INSERT … ON CONFLICT (org_id, project_id, path, agent_id) DO UPDATE SET
last_commit_at = GREATEST(…)`. The path column holds repo-relative paths that task 34 already put
through `teamtext` hygiene; nothing else is stored, so the table holds no text, no content and no
host path.

The prune runs as its own singleton, `server.NewSingleton(pool, "fleet-coedit-prune", log)`,
hourly, `PruneCoEdit(now - CoEditWindow)`. One replica prunes; a missed hour costs nothing.

#### `horizon.go` (complete)

```go
package forecast

import (
	"context"
	"sort"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// HorizonItem is one "collision between agents" for task 54's HORIZON row.
// It carries no time estimate (OD-27) and no agent words: ids, tier, paths,
// ranges and the conflict's state only.
type HorizonItem struct {
	Conflict string             // cf-…
	Tier     string             // Strongest
	State    string             // raised | negotiating | accepted | escalated
	A, B     model.AgentID
	Paths    []string           // sorted
	Lines    map[string][]Range // tier H and up; empty at P
	Approx   bool               // task 25: a glob prefix, not a file
	Thread   string             // th-…, "" until opened
}

// Horizon returns every conflict on project that is not resolved or cleared,
// accepted included (decision d), sorted by conflict id.
func (f *Forecaster) Horizon(ctx context.Context, project string) []HorizonItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []HorizonItem
	for _, o := range f.working(project) { // task 25's working copy, by project
		if o.State == "resolved" || o.State == "cleared" {
			continue
		}
		out = append(out, HorizonItem{Conflict: o.ID, Tier: o.Strongest, State: o.State,
			A: o.A, B: o.B, Paths: o.Paths, Lines: o.Lines, Approx: o.Approx, Thread: o.Thread})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Conflict < out[j].Conflict })
	return out
}
```

`working(project)` is whatever task 25 names its per-project map of `Open`; if it is a field, not
a method, this is a one-line rename. The working copy must **not** drop an entry when the feed
reports `accepted`; task 25's `Open` state list already includes `accepted` (`task-25.md:187`),
and `TestHorizonKeepsAccepted` pins it here so a later edit to task 25's feed handler cannot drop
it silently. Who sees an item is task 52's read model: both owners of the pair, nobody else.

- [ ] **Step 1: Write the failing tests first**

Fixture: Alice (Orchestrator `ag-a0`, worker `ag-a1`), Bob (Orchestrator `ag-b0`, worker `ag-b1`),
Alice's second worker `ag-a2`; blob shas `b1…` (forty hex) and `b2…`; a fake `Activity`; task
25's recording outbox, `Queue` and `Threads`; `memstore` behind `CoEdit` with a fake clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestDifferentBlobsRaiseNothingAtH` | `ag-a1` on `src/a.ts` base `b1`, `[10,20]`; `ag-b1` on `src/a.ts` base `b2`, `[10,20]`: `OnHunks` makes zero outbox rows | comparing line numbers across versions of a file |
| `TestSameBlobOverlapRaisesH` | both on base `b1`, `[10,20]` and `[15,30]`: one `conflicts.create`, tier `H`, `paths: [{"path":"src/a.ts","lines":[[10,30]]}]` | dropping the ranges, or sending tier `P` |
| `TestAdjacentRangesOverlap` | `[10,20]` and `[21,25]` raise; `[10,20]` and `[22,25]` do not | strict intersection (decision a), or a wider slack |
| `TestEmptyBaseNeverMatches` | both sides with `Base: ""`: nothing raised | treating "unknown" as "same" |
| `TestSameOwnerHunksRaiseNothing` | `ag-a1` and `ag-a2` on one blob and one range: nothing | tier H ignoring owner |
| `TestHOnOpenPUpgradesWorkingCopy` | an open tier-P conflict for the pair, then an H overlap: no second `cf-…`, no route 58 call; `Open(ctx, p)` now reports `Tier: "H"` and the ranges | a second conflict per tier, or `Open` still saying `P` |
| `TestRangesCappedAt20` | 30 disjoint overlapping ranges on one path: 20 sent, the last covering `[start₂₀, maxEnd]` | a Hub 422, or silently narrowing the overlap |
| `TestLockfileQuietAtPLoudAtH` | `package-lock.json` alone through `OnPaths` raises nothing; the same file with a same-blob hunk overlap through `OnHunks` raises tier `H` | `Quiet` applied to every tier, or not at all |
| `TestGeneratedQuietAtP` | `api/v1/user.pb.go` alone at P: nothing; with `api/v1/user.go` too: paths `["api/v1/user.go"]` | generated files raising at P |
| `TestNoisyPathQuietAtP` | five agents' commits touch `routes.ts`, three touch `x.ts`, of five agents: `routes.ts` shared at P raises nothing; `x.ts` (rate 0.6) raises nothing; 15 days later, with all five agents' recent commits on `routes.ts` only, `x.ts` (rate 0) raises | a rate over sweeps, the wrong threshold, or no window |
| `TestFewLanesNothingNoisy` | four agents, all touching `routes.ts`: it still raises at P | down-weighting before there is evidence |
| `TestCoEditOutageFailsLoud` | `Rates` returns an error: a shared `routes.ts` raises at P | failing closed, which would hide an overlap |
| `TestHorizonKeepsAccepted` | a conflict moved to `accepted`: still in `Horizon`; moved to `resolved`: gone; `cleared`: gone | dropping HORIZON on `accepted` (§9.3, `05-design.md:992`) |
| `TestHorizonHasNoEstimate` | reflection over `HorizonItem`: no field of type `time.Duration` or `time.Time`, none named `ETA`/`Eta`/`Minutes` | an ETA sneaking back in against OD-27 |
| `TestCoEditPrune` | rows at 15 days are deleted, rows at 13 days kept (`storetest.Suite`, both stores) | an unbounded table, or a prune that eats the window |

```bash
cd control-plane && go test ./internal/fleet/forecast/... 2>&1 | head -3
```

Expected: `undefined: HunkSignal` (build failure).

- [ ] **Step 2: `coedit.go`, the migration and the three store methods**

Write `coedit.go`; add `00NN_coedit.sql` with the next free number; add `ObserveCommit`, `Rates`
and `PruneCoEdit` to `memstore` (a map keyed by the primary key) and `pgstore` (the SQL above);
add `TestCoEditPrune` and one `Rates` case to `storetest.Suite` so both stores pass the same
table.

- [ ] **Step 3: `tierh.go`, then the two `tierp.go` hunks and the `conflicts.go` fields**

Move task 25's `Raise` body into `raise` unchanged, then make the three edits: `dropNoise` only at
P, lines into route 57's `paths[].lines`, and step (4)'s merge of lines and `Strongest`. Run task
25's tests before and after the move: they must pass unchanged, which proves the wrapper changed
nothing at tier P for a project with no co-edit history.

- [ ] **Step 4: `horizon.go`**

- [ ] **Step 5: Wire it**

In `cmd/fleetd/main.go`, anchored on task 25's `fc := forecast.New(…)` line: pass
`CoEdit: &forecast.CoEdit{S: st, Now: time.Now}` and `Activity: act` (task 35's table) in
`forecast.Deps`, and start the `fleet-coedit-prune` singleton beside task 12's others. Until task
35 lands, `Activity` is nil and task 35 is the only caller of `OnHunks` and `CoEdit.Observe`, so
nothing calls either; that is TM1's honest state until wave 35.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... ./cmd/fleetd/... && go test ./internal/fleet/forecast/... ./internal/fleet/store/... -count=1 -race
```

Expected: `ok` for each package, `-race` clean. The Postgres half of `storetest.Suite` runs where
task 10's `PANACEA_APP_FLEET_TEST_PG_DSN` is set, and skips with its name where it is not. Then the
no-ETA guard, which must print nothing:

```bash
cd control-plane && grep -nE "ETA|Eta|time\.Duration|Minutes" internal/fleet/forecast/horizon.go
```

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/fleet/forecast control-plane/internal/fleet/store
git commit -F <msgfile> -- control-plane/internal/fleet/forecast control-plane/internal/fleet/store control-plane/cmd/fleetd/main.go
```

**What this task does NOT claim.**
- That tier H catches much in TM1. It mostly collapses to P, because bases diverge after the first
  land and nothing re-bases ranges (§8.2, `06` Minor 4). It earns its keep in TM2, with task 60's
  mirror.
- That adjacency is exactly git's conflict rule. Decision (a) is **UNVERIFIED** against a real
  merge; task 60's `merge-tree` result is ground truth, and a disagreement changes `overlaps`.
- That the co-edit thresholds are right. `0.6` and `5` lanes over 14 days are a first setting after
  ConE's discipline, not a measured one; TM1's proof (task 57) records how many P conflicts they
  silenced, and they move only on that evidence.
- That `Generated` finds every generated file. It is a name table; a generated file with an
  ordinary name is caught only by the co-edit rate, if at all.
- A time estimate on any HORIZON item. OD-27 forbids it, over the design's *"~20 min"* example.
- Any caller. `OnHunks` and `CoEdit.Observe` are called by task 35; tested here with hand-built
  signals.

**Gaps against the skeleton, raised here and not resolved alone:**
1. **ST.6 is not written yet** (task 34). This part requires it to carry, per dirty path, the
   base blob sha and `[start, end]` ranges (task 16's `SnapHunk` shape), and the last commit's
   sha and changed paths, so task 35 can call `CoEdit.Observe` once per new commit.
2. **Task 35 must call `OnHunks` and `CoEdit.Observe`** beside its `OnPaths` call, and expose
   `Hunks(ctx, project) []AgentHunks`. Task 35's scope names only holds and leases.
3. **Task 30 must read the tier from the working copy.** Its `proceed-both` switch reads `a.Tier`
   (`task-30.md:240`); it must be `forecast.Open`'s `Strongest`, or a P conflict whose hunks later
   overlap would let `proceed-both` act without a human.
4. **The Hub record's tier is frozen at raise time.** Route 58 cannot patch tier or paths
   (`task-06.md:293-298`), so the Hub's `cf-…` can say `P` while Fleet knows `H`. If the audit
   trail must show the upgrade, tasks 01 and 06 need a `tier` on route 58; not added here.
5. The design's HORIZON example carries *"~20 min"* (`05-design.md:992`); OD-27 removes it. Task
   54's part should render no estimate.

