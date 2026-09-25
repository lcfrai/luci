### Task 111: named risk facts on lanes; the repository proposal; Orchestrator-filed issues into `triage`

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 100 (the W3 Hub guards: *"an issue created by a delegated caller is forced to `triage` with
`source: orchestrator`"*, `tasks-81-100.md:485-486`). Uses phase 3's lane index and changed paths
(`control-plane/internal/converge`, P3 tasks 07-09; skeleton delta row `:282`; **UNVERIFIED until
merge**), phase 3's Merge lens `LaneCard.tsx` (P3 task 19; **UNVERIFIED until merge**), task 48's
`IssueAgentLine.tsx`, task 42's `TriageView.tsx`, task 101's `verbs/issue.go`, task 103's
`AgentByline`, task 107's `AgentTaskCard`, and TM-05 §6.3's task scope globs and leases.

Three rows of 05 §1, each of which Linear prices as a number and we price as a fact:

1. **L-174** (`05-design.md:336`): *"Named risk facts, not a score (AX anti-pattern 12): touches
   prod config, runs a migration, edits outside declared scope (AX §7.6)"*.
2. **L-192** (`:359`): *"A project already knows its repos; with several, the Orchestrator proposes
   one, with no confidence number unless calibrated"*.
3. **L-193** (`:360`): *"An Orchestrator may file an issue (`source: orchestrator`, 04 §2.4); it
   lands in `triage` for a person"*.

**Decisions made here, so nobody re-derives them:**

1. **Three names, closed.** `touches-prod-config`, `runs-migration`, `outside-declared-scope`.
   Each fact carries the paths that raised it and one `because` line the product assembles from
   the glob that matched (*"matches `deploy/prod/**` in this project's prod-config globs"*). It is
   never a model's words. A fourth name is a code change with its own test.
2. **Mechanical only** (AX §7 point 6, `AGENTIC-UX.md:194`: *"Detected mechanically as divergence
   between declared intent and observed action"*). A fact is a glob match on a changed path, or a
   changed path outside every scope glob and lease of the task. No weighting, no ranking between
   facts, no *high/medium/low*, no colour scale, no number (`AGENTIC-UX.md:247`, anti-pattern 12).
   The facts render in the fixed order of decision 1, not by severity.
3. **Derived, never stored.** The facts are computed on read from the lane's changed paths and
   the globs, on the local control plane, and are never written to the issue or the Hub (skeleton
   rule 3). A saved view cannot filter by them in W3.
4. **The globs belong to the project.** Prod-config and migration globs come from the project's
   work settings; **no default globs are shipped**, so an unconfigured project shows no
   `touches-prod-config` or `runs-migration` fact, and the lane card says *"no risk globs set for
   this project"* once, as a fact. Where the settings live on the Hub is **UNVERIFIED** (gap 1).
5. **Declared scope exists only where a task declares it.** `outside-declared-scope` needs the
   task's scope globs and its leases (TM-05 §6.3), so it appears on team projects only; a solo
   lane has no declared scope and never carries it.
6. **Glob syntax is `doublestar`'s** (`github.com/bmatcuk/doublestar/v4`, MIT), a new dependency
   of the control-plane module, `**` included, matched on forward-slash repository paths. If
   Phase T's lease globs (TM-05 §6.3) land with a different matcher, this part switches to it so
   one glob means one thing across the product.
7. **The repository proposal is a field of the Orchestrator's task, dropped on a one-repo
   project.** With one bound remote there is nothing to propose and Fleet drops the field. With
   several, the proposal names one remote the project binds and a `because` (`text·A`, at most
   300 characters). A `confidence` value is kept **only** with a `calibrationId` that resolves;
   no calibration record exists in W3, so in W3 every confidence is dropped (anti-pattern 12).
   Whether a Hub project binds several remotes today is **UNVERIFIED** (gap 2).
8. **An Orchestrator files through one added verb, `issue.file`**, beside task 101's three. The
   verb has no `state`, `assignee`, `priority` or `cycle` argument at all, so an Orchestrator
   cannot ask for them; the Hub forces `triage` and `source: orchestrator` whatever the body says
   (task 100). The scope's *"create outside `triage` is refused"* is therefore tested as *stored
   in `triage`*, task 100's behaviour, and the difference is named (gap 3).
9. **An Orchestrator never assigns, starts or moves an issue.** No verb offers it (dispatch
   refuses an unknown verb), and the Hub refuses a delegated caller's assign and state move; the
   Hub row is asked of task 100 (gap 4) and run here as conformance rows against the fake and the
   real Hub.
10. **Triage shows who filed it.** An Orchestrator-filed row carries task 103's byline
    (`alice · orchestrator (agent)`), the `text·A` mark on title and description, and the
    Orchestrator's `because`. Accepting it is task 42's accept, done by the person; the revision
    stays `text·A` until a person edits it.

**Files:**
- `control-plane/internal/work/riskfacts.go` (+ `_test.go`) — the derivation (new; **path
  UNVERIFIED**, not in the file map)
- `control-plane/internal/api/workrisk.go` (+ `_test.go`), `api.go` ⚑, `routes_audit_test.go` ⚑ —
  `GET /v1/work/lanes/{lane}/risk` on loopback (new, modified)
- `control-plane/go.mod` ⚑, `go.sum` ⚑ — `doublestar/v4`, MIT (modified)
- `desktop/src/components/work/RiskFacts.tsx` (+ test) — the facts line (new)
- phase 3's `desktop/src/components/lens/merge/LaneCard.tsx` ⚑ and task 48's
  `desktop/src/components/issue/IssueAgentLine.tsx` ⚑ — mount `RiskFacts` (modified)
- `control-plane/internal/fleet/work/repo.go` (+ `_test.go`) — the repository-choice rule (new)
- `desktop/src/components/work/RepoProposal.tsx` (+ test), mounted in task 107's
  `AgentTaskCard.tsx` ⚑ (new, modified)
- `control-plane/internal/fleet/verbs/issue.go` ⚑ (task 101's), TM-19's dispatch table ⚑,
  `testdata/panacea-control/v1/issue.file.*.json` — the `issue.file` verb (modified, new)
- `desktop/src/components/work/TriageView.tsx` ⚑ (task 42's) — the Orchestrator-filed row (modified)
- `control-plane/internal/hub/hubclient/conformance/work_agents.go` ⚑ (task 99's) — three rows,
  appended under rule H1 (modified)

#### Interfaces — complete: `control-plane/internal/work/riskfacts.go`

```go
// Package work holds the local control plane's derived work facts (Phase W task 111).
// Nothing here is stored: every fact is computed on read (skeleton rule 3).
package work

import (
	"fmt"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
)

// RiskName is closed: three names, in this display order (decision 1).
type RiskName string

const (
	TouchesProdConfig    RiskName = "touches-prod-config"
	RunsMigration        RiskName = "runs-migration"
	OutsideDeclaredScope RiskName = "outside-declared-scope"
)

var order = []RiskName{TouchesProdConfig, RunsMigration, OutsideDeclaredScope}

type RiskFact struct {
	Name    RiskName `json:"name"`
	Paths   []string `json:"paths"`   // sorted, repository-relative, forward slashes
	Because string   `json:"because"` // assembled product text, never a model's words
}

// ChangedPath is one entry of phase 3's lane diff against its base (P3 tasks 07-09's index).
type ChangedPath struct {
	Path   string `json:"path"`
	Status string `json:"status"` // A, M, D or R; a rename is reported under its new path
}

// ProjectGlobs are the project's own risk globs (decision 4). Empty means not set.
type ProjectGlobs struct {
	ProdConfig []string `json:"prodConfig"`
	Migrations []string `json:"migrations"`
}

// DeclaredScope is a task's scope globs and its held leases (TM-05 §6.3); nil on a solo lane.
type DeclaredScope struct {
	Globs  []string `json:"globs"`
	Leases []string `json:"leases"`
}

// Facts derives the risk facts for one lane. The result is empty, never nil, when nothing matches.
func Facts(changed []ChangedPath, g ProjectGlobs, scope *DeclaredScope) ([]RiskFact, error) {
	hits := map[RiskName]map[string]string{} // name → path → the glob that matched
	add := func(n RiskName, path, glob string) {
		if hits[n] == nil {
			hits[n] = map[string]string{}
		}
		if _, dup := hits[n][path]; !dup {
			hits[n][path] = glob
		}
	}
	for _, c := range changed {
		if glob, ok, err := firstMatch(g.ProdConfig, c.Path); err != nil {
			return nil, err
		} else if ok {
			add(TouchesProdConfig, c.Path, glob)
		}
		if c.Status == "A" || c.Status == "M" || c.Status == "R" {
			if glob, ok, err := firstMatch(g.Migrations, c.Path); err != nil {
				return nil, err
			} else if ok {
				add(RunsMigration, c.Path, glob)
			}
		}
		if scope != nil {
			_, inGlobs, err := firstMatch(scope.Globs, c.Path)
			if err != nil {
				return nil, err
			}
			_, inLease, err := firstMatch(scope.Leases, c.Path)
			if err != nil {
				return nil, err
			}
			if !inGlobs && !inLease {
				add(OutsideDeclaredScope, c.Path, "")
			}
		}
	}
	out := []RiskFact{}
	for _, n := range order {
		if len(hits[n]) == 0 {
			continue
		}
		paths := make([]string, 0, len(hits[n]))
		for p := range hits[n] {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		out = append(out, RiskFact{Name: n, Paths: paths, Because: because(n, hits[n][paths[0]])})
	}
	return out, nil
}

// firstMatch reports the first glob that matches path. A malformed glob is an error, never a miss.
func firstMatch(globs []string, path string) (string, bool, error) {
	for _, g := range globs {
		ok, err := doublestar.Match(g, path)
		if err != nil {
			return "", false, fmt.Errorf("risk glob %q: %w", g, err)
		}
		if ok {
			return g, true, nil
		}
	}
	return "", false, nil
}

func because(n RiskName, glob string) string {
	switch n {
	case TouchesProdConfig:
		return fmt.Sprintf("matches `%s` in this project's prod-config globs", glob)
	case RunsMigration:
		return fmt.Sprintf("adds or changes a file matching `%s` in this project's migration globs", glob)
	default:
		return "outside the task's declared scope globs and its leases"
	}
}
```

`because` quotes the first matching glob of the first path only; the facts line lists every path.
A deleted migration file raises nothing: deleting a migration is not running one, and the lane's
diff shows it.

#### Interfaces — complete: `control-plane/internal/api/workrisk.go`

```go
package api

import (
	"context"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/work"
)

// RiskInputs gathers the three inputs; each method names the seam that serves it.
type RiskInputs interface {
	// LaneChanges reads phase 3's lane index: the lane's changed paths against its base.
	LaneChanges(ctx context.Context, laneID string) ([]work.ChangedPath, error)
	// ProjectGlobs reads the project's risk globs from its work settings (gap 1).
	ProjectGlobs(ctx context.Context, laneID string) (work.ProjectGlobs, bool, error) // bool: set at all
	// Scope reads the task's scope and leases through TM-52's team read model; nil on a solo lane.
	Scope(ctx context.Context, laneID string) (*work.DeclaredScope, error)
	// IsTeamLane: the lane's session came through TM-21's team launch.
	IsTeamLane(ctx context.Context, laneID string) bool
}

type riskReply struct {
	Lane         string          `json:"lane"`
	Facts        []work.RiskFact `json:"facts"`
	GlobsSet     bool            `json:"globsSet"`
	ScopeChecked bool            `json:"scopeChecked"` // false on a solo lane or with the team view down
	TeamLane     bool            `json:"teamLane"`
}

func riskHandler(in RiskInputs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lane := r.PathValue("lane")
		changed, err := in.LaneChanges(r.Context(), lane)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "lane_not_found", "no such lane")
			return
		}
		globs, set, err := in.ProjectGlobs(r.Context(), lane)
		if err != nil {
			writeProblem(w, http.StatusBadGateway, "settings_unavailable", "project settings could not be read")
			return
		}
		team := in.IsTeamLane(r.Context(), lane)
		scope, err := in.Scope(r.Context(), lane)
		if err != nil {
			scope = nil // team view down: the scope fact is absent, never guessed
		}
		facts, err := work.Facts(changed, globs, scope)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "bad_risk_glob", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, riskReply{Lane: lane, Facts: facts, GlobsSet: set,
			ScopeChecked: scope != nil, TeamLane: team})
	}
}
```

Registered as `GET /v1/work/lanes/{lane}/risk` when `Deps.WorkRisk` is set,
with its catalog row. `writeProblem` and `writeJSON` are the package's helpers under their real
names (task 110's note). A team lane with `scopeChecked: false` shows *"scope not checked: team
view unavailable"*; a solo lane shows nothing about scope (decision 5).

#### Interfaces — complete: `control-plane/internal/fleet/work/repo.go`

```go
package work

import (
	"context"
	"errors"
	"unicode/utf8"
)

// RepoChoice is the repository proposal on an Orchestrator's task (L-192). Because is text·A.
type RepoChoice struct {
	Remote        string   `json:"remote"`            // one of the project's bound remotes, by its Hub id
	Because       string   `json:"because"`           // at most 300 characters
	Confidence    *float64 `json:"confidence,omitempty"`
	CalibrationID string   `json:"calibrationId,omitempty"`
}

// Calibrations resolves a calibration record (AX §7: "Trust is calibrated from the user's own
// history"). No store exists in W3, so Fleet wires NoCalibrations and every confidence is dropped.
type Calibrations interface {
	Exists(ctx context.Context, id string) bool
}

type NoCalibrations struct{}

func (NoCalibrations) Exists(context.Context, string) bool { return false }

var ErrUnboundRemote = errors.New("unbound_remote")
var ErrBadBecause = errors.New("invalid_args")

// CheckRepoChoice runs where TM-18's `task` verb creates or edits an Orchestrator's task.
// It returns the choice to store: nil on a one-remote project (decision 7).
func CheckRepoChoice(ctx context.Context, remotes []string, c *RepoChoice, cal Calibrations) (*RepoChoice, error) {
	if c == nil || len(remotes) <= 1 {
		return nil, nil
	}
	bound := false
	for _, r := range remotes {
		bound = bound || r == c.Remote
	}
	if !bound {
		return nil, ErrUnboundRemote
	}
	if n := utf8.RuneCountInString(c.Because); n == 0 || n > 300 {
		return nil, ErrBadBecause
	}
	out := *c
	if out.Confidence != nil && (out.CalibrationID == "" || !cal.Exists(ctx, out.CalibrationID)) {
		out.Confidence, out.CalibrationID = nil, "" // anti-pattern 12: no number without its history
	}
	return &out, nil
}
```

The choice rides on Phase T's task record as one **added** optional field `repository`
(`hubapi/team.go`, TM-03; an added field, as task 98 adds `issueId`), a delta listed as gap 2.

#### Wire — complete: the `issue.file` verb (TM-09's envelope; Orchestrator class only)

```json
{ "v": 1, "id": "12", "verb": "issue.file",
  "args": { "project": "pj-01J…", "title": "Lease cap is not applied to renamed paths",
            "description": "Seen while planning EP-142: …", "because": "found during EP-142",
            "serves": [ { "briefKey": "brief:bf-01J…@4", "criterionId": "c-40aa…" } ] } }
```

```json
{ "id": "12", "ok": true, "result": { "issue": "is-01J…", "alias": "EP-163", "state": "triage",
  "source": "orchestrator", "revision": "issue:is-01J…@1", "titleClass": "A" } }
```

Args are closed: an unknown field (`state`, `assignee`, `priority`, `cycle`, `labels`) is refused
with ST.1's unknown-field code before the handler runs (TM-19). Limits: `title` at most 140
characters, `description` at most 4,000, `because` at most 300, `serves` at most 5 and each
criterion one the caller's owner may read. The handler writes through task 99's create method as
the delegated caller (TM-05), so the Hub records `via: fleet` and the owner; the stored `state`
and `source` are the Hub's (task 100), and the result echoes what the Hub stored, never the args.

#### Interfaces — complete: appended to task 101's `control-plane/internal/fleet/verbs/issue.go`

```go
// IssueFileArgs is issue.file's args. There is no state, assignee, priority or cycle field
// (decision 8); TM-19 refuses any field not named here.
type IssueFileArgs struct {
	Project     string        `json:"project"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Because     string        `json:"because"`
	Serves      []ServesEntry `json:"serves,omitempty"` // task 101's {briefKey, criterionId}
}

type IssueFileResult struct {
	Issue      string `json:"issue"`
	Alias      string `json:"alias"` // display only; never written into any record (skeleton rule 1)
	State      string `json:"state"`
	Source     string `json:"source"`
	Revision   string `json:"revision"`
	TitleClass string `json:"titleClass"`
}

// IssueFiler is the one hubclient call issue.file needs (task 99's create for a delegated caller;
// the method name is task 99's).
type IssueFiler interface {
	FileIssueAsAgent(ctx context.Context, d Delegated, a IssueFileArgs) (IssueFileResult, error)
}

func FileIssue(ctx context.Context, s Stamp, d Delegated, h IssueFiler, a IssueFileArgs) (IssueFileResult, error) {
	if s.Class != "orchestrator" {
		return IssueFileResult{}, ErrClass // a worker reports to its Orchestrator instead
	}
	if n := utf8.RuneCountInString(a.Title); n == 0 || n > 140 ||
		utf8.RuneCountInString(a.Description) > 4000 || utf8.RuneCountInString(a.Because) > 300 || len(a.Serves) > 5 {
		return IssueFileResult{}, ErrArgs
	}
	return h.FileIssueAsAgent(ctx, d, a) // the Hub forces triage and source: orchestrator
}
```

`Stamp`, `Delegated`, `ServesEntry`, `ErrClass` and `ErrArgs` are task 101's names in the same
package (its scope: *"`issue note` and `evidence propose` are **Orchestrator class only**"*); step 1
renames here if task 101 chose others. The dispatch row names class `orchestrator` only, and the
issue.file handler runs behind task 109's switch because every delegated write does (task 100's
`agents_disabled`).

#### Interfaces — complete: `desktop/src/components/work/RiskFacts.tsx`

```tsx
export type RiskName = "touches-prod-config" | "runs-migration" | "outside-declared-scope";
export interface RiskFact { name: RiskName; paths: string[]; because: string }
export interface RiskReply { lane: string; facts: RiskFact[]; globsSet: boolean; scopeChecked: boolean; teamLane: boolean }

const WORD: Record<RiskName, string> = {
  "touches-prod-config": "touches prod config",
  "runs-migration": "runs a migration",
  "outside-declared-scope": "outside declared scope",
};

/** Named facts, in the fixed order the control plane sent, one ink for all (decision 2). */
export function RiskFacts({ reply, dense = false }: { reply: RiskReply; dense?: boolean }) {
  const notes: string[] = [];
  if (!reply.globsSet) notes.push("no risk globs set for this project");
  if (reply.teamLane && !reply.scopeChecked) notes.push("scope not checked: team view unavailable");
  if (reply.facts.length === 0 && notes.length === 0) return null;
  return (
    <div className="risk" data-dense={dense || undefined}>
      {reply.facts.map((f) => (
        <details key={f.name} className="risk-fact" data-risk={f.name}>
          <summary><span className="risk-name">{WORD[f.name]}</span> · <span className="risk-paths">{f.paths[0]}{f.paths.length > 1 ? ` +${f.paths.length - 1} paths` : ""}</span></summary>
          <p className="risk-because">{f.because}</p>
          <ul className="risk-list">{f.paths.map((p) => <li key={p}>{p}</li>)}</ul>
        </details>
      ))}
      {notes.map((n) => <p key={n} className="risk-note">{n}</p>)}
    </div>
  );
}
```

Every fact uses one ink (`--ink-2`) and one mark; no fact is red, amber or green, because a colour
would rank them. Paths are Commit Mono at `--t-fact`. `+2 paths` counts listed paths inside a
collapsed fact that expands to all of them, which AX anti-pattern 13 (`AGENTIC-UX.md:248`) asks
of a collapsed row; it is not a completion count.

The two mounts are one line each: `LaneCard.tsx` (P3 task 19) renders `<RiskFacts reply={…}/>`
under the lane's commit line, and task 48's `IssueAgentLine.tsx` renders it `dense` under each
own agent's line whose session has a lane. Both read the reply through one added call,
`workRisk(laneId)`, in `desktop/src/api/work.ts`.

#### Interfaces — complete: `desktop/src/components/work/RepoProposal.tsx`

```tsx
export interface RepoChoice { remote: string; because: string; confidence?: number; calibrationId?: string }
export interface BoundRemote { id: string; label: string } // e.g. "github · kpmg/panacea-app"

/**
 * The Orchestrator's repository proposal on a task card (task 107). Renders nothing on a
 * one-remote project. A confidence shows only beside a calibration id (anti-pattern 12);
 * Fleet already dropped any other, and this is the second fence.
 */
export function RepoProposal({ choice, remotes }: { choice?: RepoChoice; remotes: BoundRemote[] }) {
  if (!choice || remotes.length <= 1) return null;
  const remote = remotes.find((r) => r.id === choice.remote);
  if (!remote) return <p className="repo-fact">proposed repository is not bound to this project</p>;
  const calibrated = choice.confidence !== undefined && !!choice.calibrationId;
  return (
    <div className="repo">
      <p className="repo-line">
        <span className="repo-label">{remote.label}</span> · proposed by the Orchestrator (agent)
        {calibrated && <span className="repo-conf"> · {choice.confidence!.toFixed(2)} (calibration {choice.calibrationId})</span>}
      </p>
      <p className="repo-because" data-class="A">{choice.because}</p>
    </div>
  );
}
```

The confidence is a raw ratio printed with its calibration id, never a `%` (SW.8's guard fails on
`%`), and in W3 it never renders because no calibration record exists (decision 7).

#### Interfaces — complete: the edit to task 42's `desktop/src/components/work/TriageView.tsx`

```tsx
// In task 42's triage row, where the row renders its title and reporter:
{head.source === "orchestrator" ? (
  <>
    <AgentByline who={head.createdBy} />               {/* task 103: "alice · orchestrator (agent)" */}
    <span className="tri-title" data-class={head.titleClass}>{head.title}</span>
    {head.fileReason && <p className="tri-because" data-class="A">{head.fileReason}</p>}
  </>
) : (
  task42Row(head) /* task 42's own rendering, unchanged */
)}
```

`head.source`, `head.titleClass` and `head.createdBy` are W1's head fields (tasks 03 and 37);
`fileReason` is the verb's `because`, which the Hub stores only if task 99 adds it as an added
field on the create body (gap 5). Task 42's *Accept* and *Decline* stay as they are: they are the
person's acts, sent with the person's token.

#### Tests to write first — `control-plane/internal/work/riskfacts_test.go`

```go
package work

import (
	"reflect"
	"strings"
	"testing"
)

var globs = ProjectGlobs{
	ProdConfig: []string{"deploy/prod/**", "**/*.prod.yaml"},
	Migrations: []string{"control-plane/internal/hub/migrate/sql/*.sql"},
}

func TestAMigrationFileRaisesRunsMigrationWithItsPath(t *testing.T) {
	got, err := Facts([]ChangedPath{{Path: "control-plane/internal/hub/migrate/sql/0042_work_agents.sql", Status: "A"}}, globs, nil)
	if err != nil || len(got) != 1 || got[0].Name != RunsMigration ||
		!reflect.DeepEqual(got[0].Paths, []string{"control-plane/internal/hub/migrate/sql/0042_work_agents.sql"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if !strings.Contains(got[0].Because, "control-plane/internal/hub/migrate/sql/*.sql") {
		t.Fatalf("because %q does not name its glob", got[0].Because)
	}
}

func TestADeletedMigrationRaisesNothing(t *testing.T) {
	got, _ := Facts([]ChangedPath{{Path: "control-plane/internal/hub/migrate/sql/0041_x.sql", Status: "D"}}, globs, nil)
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestAPathOutsideTheTaskScopeAndLeases(t *testing.T) {
	scope := &DeclaredScope{Globs: []string{"control-plane/internal/fleet/**"}, Leases: []string{"docs/work/*.md"}}
	got, _ := Facts([]ChangedPath{
		{Path: "control-plane/internal/fleet/leases/glob.go", Status: "M"},
		{Path: "docs/work/agent-rules.md", Status: "M"},
		{Path: "desktop/src/App.tsx", Status: "M"},
	}, ProjectGlobs{}, scope)
	if len(got) != 1 || got[0].Name != OutsideDeclaredScope || !reflect.DeepEqual(got[0].Paths, []string{"desktop/src/App.tsx"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestASoloLaneNeverCarriesTheScopeFact(t *testing.T) {
	got, _ := Facts([]ChangedPath{{Path: "anything/at/all.go", Status: "A"}}, ProjectGlobs{}, nil)
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestFactsComeInTheFixedOrderAndCarryNoNumber(t *testing.T) {
	got, _ := Facts([]ChangedPath{
		{Path: "desktop/x.ts", Status: "M"},
		{Path: "control-plane/internal/hub/migrate/sql/0042_a.sql", Status: "A"},
		{Path: "deploy/prod/values.yaml", Status: "M"},
	}, globs, &DeclaredScope{Globs: []string{"deploy/**", "control-plane/**"}})
	names := []RiskName{}
	for _, f := range got {
		names = append(names, f.Name)
	}
	if !reflect.DeepEqual(names, []RiskName{TouchesProdConfig, RunsMigration, OutsideDeclaredScope}) {
		t.Fatalf("order %v", names)
	}
	if typ := reflect.TypeOf(RiskFact{}); typ.NumField() != 3 {
		t.Fatalf("RiskFact grew a field: a score or a level does not belong on it")
	}
}

func TestAMalformedGlobIsAnErrorNotAMiss(t *testing.T) {
	if _, err := Facts([]ChangedPath{{Path: "a", Status: "M"}}, ProjectGlobs{ProdConfig: []string{"deploy/[prod"}}, nil); err == nil {
		t.Fatal("a broken glob silently matched nothing")
	}
}
```

#### Tests to write first — `control-plane/internal/fleet/work/repo_test.go`

```go
package work

import (
	"context"
	"errors"
	"testing"
)

type someCalibrations map[string]bool

func (c someCalibrations) Exists(_ context.Context, id string) bool { return c[id] }

func TestAOneRepoProjectGetsNoProposal(t *testing.T) {
	got, err := CheckRepoChoice(context.Background(), []string{"rm-1"}, &RepoChoice{Remote: "rm-1", Because: "only one"}, NoCalibrations{})
	if err != nil || got != nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestAnUncalibratedConfidenceIsDropped(t *testing.T) {
	c := 0.91
	got, _ := CheckRepoChoice(context.Background(), []string{"rm-1", "rm-2"},
		&RepoChoice{Remote: "rm-2", Because: "the API lives there", Confidence: &c, CalibrationID: "cal-x"}, NoCalibrations{})
	if got == nil || got.Confidence != nil || got.CalibrationID != "" {
		t.Fatalf("kept %+v", got)
	}
	got, _ = CheckRepoChoice(context.Background(), []string{"rm-1", "rm-2"},
		&RepoChoice{Remote: "rm-2", Because: "x", Confidence: &c, CalibrationID: "cal-x"}, someCalibrations{"cal-x": true})
	if got.Confidence == nil {
		t.Fatal("a calibrated confidence was dropped")
	}
}

func TestAnUnboundRemoteIsRefused(t *testing.T) {
	_, err := CheckRepoChoice(context.Background(), []string{"rm-1", "rm-2"}, &RepoChoice{Remote: "rm-9", Because: "x"}, NoCalibrations{})
	if !errors.Is(err, ErrUnboundRemote) {
		t.Fatalf("err %v", err)
	}
}
```

#### Tests to write first — appended to task 101's `verbs/issue_test.go`, and the conformance rows

```go
func TestIssueFileIsOrchestratorOnly(t *testing.T) {
	_, err := FileIssue(ctx, Stamp{Class: "worker"}, dylanDelegated, &fakeFiler{}, IssueFileArgs{Project: "pj-1", Title: "x"})
	if !errors.Is(err, ErrClass) {
		t.Fatalf("a worker filed an issue: %v", err)
	}
}

func TestIssueFileResultIsWhatTheHubStored(t *testing.T) {
	f := &fakeFiler{stored: IssueFileResult{Issue: "is-1", State: "triage", Source: "orchestrator", TitleClass: "A"}}
	got, _ := FileIssue(ctx, Stamp{Class: "orchestrator"}, dylanDelegated, f, IssueFileArgs{Project: "pj-1", Title: "x"})
	if got.State != "triage" || got.Source != "orchestrator" {
		t.Fatalf("got %+v", got)
	}
}

func TestIssueFileRefusesAStateArgument(t *testing.T) {
	// Through TM-19's dispatch with the real args decoder: the unknown field never reaches the handler.
	res := dispatchFixture(t, "issue.file.with-state.json") // args carry "state": "todo"
	if res.OK || res.Error.Code != "unknown_field" {
		t.Fatalf("got %+v", res)
	}
}
```

Appended to task 99's `hubclient/conformance/work_agents.go` ⚑ (run against the fake and, with
`-tags integration`, the real Hub; rule H1, additive):

```go
{Name: "an Orchestrator-filed issue is stored in triage whatever the body says", Run: func(t *testing.T, h Harness) {
	got := h.AsDelegated("u-dylan").FileIssueAsAgent(t, map[string]any{"title": "x", "state": "todo"})
	requireEqual(t, "triage", got.State); requireEqual(t, "orchestrator", got.Source)
}},
{Name: "a delegated caller cannot assign an issue", Run: func(t *testing.T, h Harness) {
	is := h.AsPerson("u-dylan").NewIssue(t)
	requireCode(t, "forbidden", h.AsDelegated("u-dylan").Assign(is, "u-dylan"))
}},
{Name: "a delegated caller cannot move an issue's state", Run: func(t *testing.T, h Harness) {
	is := h.AsPerson("u-dylan").NewIssue(t)
	requireCode(t, "forbidden", h.AsDelegated("u-dylan").MoveState(is, "started"))
}},
```

`Harness`, `AsDelegated`, `AsPerson`, `requireEqual` and `requireCode` stand for task 05's and
99's conformance helpers under their real names. If the last two rows fail on task 100's server,
that is a finding against task 100 (gap 4), not a reason to weaken the row.

#### Tests to write first — `desktop/src/components/work/{RiskFacts,RepoProposal}.test.tsx`

```tsx
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { RiskFacts } from "./RiskFacts";
import { RepoProposal } from "./RepoProposal";
import src from "./RiskFacts.tsx?raw";

const reply = (over = {}) => ({ lane: "ln-1", globsSet: true, scopeChecked: true, teamLane: true, facts: [
  { name: "runs-migration" as const, paths: ["m/0042.sql"], because: "adds or changes a file matching `m/*.sql` …" },
], ...over });

describe("RiskFacts", () => {
  it("names the fact and its path, with no number, level or colour", () => {
    const { container } = render(<RiskFacts reply={reply()} />);
    expect(screen.getByText("runs a migration")).toBeTruthy();
    expect(screen.getAllByText("m/0042.sql").length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/\b(high|medium|low|score|risk \d)/i);
    expect(src).not.toMatch(/--(red|amber|green|danger|warn)|severity|score/);
    expectNoPrediction(container);
  });
  it("says so once when the project has no risk globs", () => {
    render(<RiskFacts reply={reply({ facts: [], globsSet: false })} />);
    expect(screen.getAllByText("no risk globs set for this project").length).toBe(1);
  });
  it("says scope was not checked on a team lane with the team view down, and nothing on a solo lane", () => {
    const { rerender } = render(<RiskFacts reply={reply({ facts: [], scopeChecked: false })} />);
    expect(screen.getByText("scope not checked: team view unavailable")).toBeTruthy();
    rerender(<RiskFacts reply={reply({ facts: [], scopeChecked: false, teamLane: false })} />);
    expect(screen.queryByText(/scope/)).toBeNull();
  });
});

describe("RepoProposal", () => {
  const two = [{ id: "rm-1", label: "github · kpmg/api" }, { id: "rm-2", label: "ado · kpmg/web" }];
  it("renders nothing on a one-repo project", () => {
    const { container } = render(<RepoProposal choice={{ remote: "rm-1", because: "x" }} remotes={[two[0]]} />);
    expect(container.textContent).toBe("");
  });
  it("shows no confidence without a calibration id, and the reason as agent text", () => {
    const { container } = render(<RepoProposal choice={{ remote: "rm-2", because: "the form lives there", confidence: 0.9 }} remotes={two} />);
    expect(container.textContent).not.toMatch(/0\.9|%/);
    expect(container.querySelector("[data-class='A']")?.textContent).toBe("the form lives there");
    expectNoPrediction(container);
  });
});
```

Task 42's `TriageView.test.tsx` gains one row: an Orchestrator-filed issue shows
`alice · orchestrator (agent)`, its title carries `data-class="A"`, and *Accept* is sent with the
person's token through task 42's accept op (mutant: accept sent as the agent, or the byline lost).

| Test | Mutant it kills |
|---|---|
| a migration file raises `runs-migration` with its path and glob | fact missed; a fact with no *because* |
| a deleted migration raises nothing | a delete read as running a migration |
| a path outside scope and leases | scope unchecked; a lease ignored |
| a solo lane never carries the scope fact | a guessed scope on a lane that declared none |
| fixed order; `RiskFact` has three fields | facts ranked by severity; a score field added |
| a malformed glob is an error | a broken glob silently matches nothing |
| one-repo project, no proposal | a needless proposal |
| uncalibrated confidence dropped; calibrated kept | an uncalibrated number (anti-pattern 12); a rule that drops every number forever |
| unbound remote refused | a proposal naming a repository the project does not bind |
| `issue.file` Orchestrator only | a worker files issues |
| result is what the Hub stored | the verb echoes its args, hiding the Hub's triage |
| a `state` argument refused | the agent files into the backlog |
| conformance: stored in triage; no assign; no state move | the agent files into the backlog; the agent triages its own issue |
| RiskFacts: no number, level or colour | a risk score, a traffic light |
| globs not set, said once | silence read as *no risk* |
| scope not checked on a team lane | a missing scope check read as *in scope* |
| RepoProposal: nothing on one repo; no confidence | a needless proposal; an uncalibrated number |

#### Steps

- [ ] **Step 1: read on the sync base** phase 3's lane index and changed-path read
  (`internal/converge`) and `LaneCard.tsx`, task 48's `IssueAgentLine.tsx`, task 42's
  `TriageView.tsx`, task 101's `verbs/issue.go` names, task 99's create method and conformance
  helpers, TM-03's task type and TM-05 §6.3's scope and lease fields, and whether the Hub project
  record binds more than one remote. Rename in this part's files only.
- [ ] **Step 2: add `doublestar/v4`** (`go get github.com/bmatcuk/doublestar/v4`; MIT, named in
  the commit message); check `git diff control-plane/go.mod` keeps `go 1.23`.
- [ ] **Step 3: the Go tests first**, then `riskfacts.go`, `workrisk.go` with its registration and
  catalog row, `repo.go`, and `FileIssue` with its dispatch row and fixtures.
- [ ] **Step 4: the conformance rows**; run them against the fake, then with `-tags integration`
  against the real Hub; a failing assign or move row is reported to task 100's owner.
- [ ] **Step 5: the desktop tests first**, then `RiskFacts.tsx`, `RepoProposal.tsx`, the two mounts,
  the `TriageView` edit and `workRisk` in `work.ts`.
- [ ] **Step 6: run the acceptance commands.** Commit by path after steps 3, 4 and 5, each message
  ending with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/work/ ./control-plane/internal/fleet/work/ -race -count=1 -v -run 'Migration|Scope|Solo|Order|Malformed|Repo|Calibrated|Unbound'
# expect: riskfacts 6 and repo 3 tests, all PASS
go test ./control-plane/internal/fleet/verbs/ -race -count=1 -run IssueFile
# expect: 3 tests PASS
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
go test -tags integration ./control-plane/internal/hub/hubclient/conformance/ -run 'triage|assign|state' -count=1
# expect: 3 rows PASS against the real Hub (compose.yaml up)
npm --prefix desktop test -- work/RiskFacts work/RepoProposal work/TriageView issue/IssueAgentLine
# expect: every file passes; this part adds 5 tests in two new files, plus 1 in TriageView
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "score|severity|confidence" control-plane/internal/work/riskfacts.go desktop/src/components/work/RiskFacts.tsx
# expect: no output
```

The counts are this part's plan, not a run.

#### What this task does NOT claim

- That the globs catch every production config or migration: the project owns them, and an
  unconfigured project raises neither fact (decision 4).
- That a fact means the lane is dangerous, or its absence that it is safe. A fact says a path
  matched a glob.
- That any repository proposal is right, or that a calibration record will ever exist; W3 shows
  no confidence at all.
- That an Orchestrator files useful issues. It claims only that what it files lands in `triage`,
  marked `text·A`, for a person.
- That Phase T's task record carries `repository` until its delta lands (gap 2).

**Parity** (`05-design.md` rows). Closes **L-174** (`:336`), **L-192** (`:359`, on team projects
with more than one bound remote) and **L-193** (`:360`).

#### Gaps found while writing this part

1. **Where a project's risk globs live** is undesigned: no W1 settings record names prod-config or
   migration globs. They need an added field on the project's work settings (task 03's wire) and a
   settings row (task 109's settings surface).
2. **Several remotes per Hub project, and `repository` on Phase T's task**, are both unverified:
   the Hub contract was not read for multi-remote binding here, and TM-03's task has no such field.
3. **The scope says an Orchestrator's create outside `triage` is *refused*; task 100 *forces* it
   to `triage`.** This part follows task 100 (the Hub owner) and makes the verb carry no `state`
   at all; the skeleton scope should say *stored in triage*.
4. **No W3 Hub guard forbids a delegated caller's assign or state move.** Task 100's guards list
   triage-forcing and assignee-only delegation, not these; the conformance rows here ask for them.
5. **The Orchestrator's `because` for filing** has no field on W1's create body; an added field
   (`fileReason`, `text·A`) on task 99's wire, or the verb drops it.
6. **`issue.file` is a fourth work verb**; the skeleton's delta row (`:287`) says *"three verbs"*.
7. **The file map has no row** for `control-plane/internal/work/`, `internal/api/workrisk.go`,
   `components/work/{RiskFacts,RepoProposal}.tsx` or Fleet's `work/repo.go`.

