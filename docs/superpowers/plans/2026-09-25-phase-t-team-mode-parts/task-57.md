### Task 57: TM1 proof — two people on the real environment, the eight criteria, the first-teammate gate

**Slice:** TM1. **Focused time:** 3 h of writing and wiring, plus the run itself, which is two
people's afternoon and is **not** in the estimate. **dependsOn:** 33–56, every TM1 task; task 33's
pass (TM1-α), without which TM1 was re-planned and this part is re-read against the new plan.
**Holds:** the 3-day G9 contingency (`05-design.md` §12.1 stream G; the skeleton, `:1012`).

TM1 is done only on the real environment, with two real people (`05-design.md` §12.1). This task
writes the record those two people fill, the collector that exports what the run left behind, and
a verdict program that reads that export and says, per criterion, `PASS`, `FAIL` or `NOT RUN`,
with the evidence ids beside it. A screenshot is not evidence (`CLAUDE.md`, working agreements):
every verdict is computed from **records** (Hub rows, the audit stream, the gateway's usage rows,
the gatekeeper's access log), and every human observation is written down as an observation.

**The promise being proved** (§12.1): Dylan shares a project. Priya joins from her own desktop.
Each tells their own Orchestrator what they want. Their agents see each other's tasks, reasons and
files; an overlap is caught before any code is written; the two Orchestrators settle it or ask;
and each human hears only from their own.

**The eight criteria, as the verdict program reads them** (§12.1, numbered as there):

| # | Criterion | Read from | `PASS` when |
|---|---|---|---|
| 1 | an overlapping plan raises tier P **before** either worker spawns | Hub conflicts (route 59), Hub agent starts (route 55) | the run's first `cf-…` has `tier: "P"`, and its `createdAt` is earlier than the `startedAt` of every `role: "worker"` agent of both owners |
| 2 | `yield` or `sequence` settles it; one human gets an `inform`, the other a `decide`, each on their own Desk; DECIDED WITHOUT YOU reaches both | conflicts, escalations (route 66, read as org *auditor*) | the conflict's `resolution` is `yield` or `sequence`; one agent-raised `inform` owned by one person and one `decide` owned by the other; **two** Fleet-raised `inform`s (`fromAgentId: null`) whose question begins `DECIDED WITHOUT YOU:`, one per person (`task-29.md:54-58`) |
| 3 | zero free-text directives in Fleet-started turns; zero out-of-scope directives; the PTY writer refuses every non-template byte | directives (route 68), G9's report (task 42), task 38's test output | no `dv-…` with `origin: "fleet"` and `kind: "free"`; G9's `outOfScope` is 0; task 38's refusal test passed on the deployed build's commit |
| 4 | the median thread resolves without a deadline escalation, with hop time measured | threads, messages, escalations | more than half the run's threads reached `resolved` before their `deadline`, with no escalation on them raised by Fleet's deadline rule; the median and p90 hop time (message to reply, across owners) are printed |
| 5 | usage rows attribute every token to person, project and counterpart; overhead is measured | the gateway's usage rows (phase 6 task 09), Hub turn events (route 69 writes an audit event and no row, so they are read from Hub task 27's audit route) | every usage row of the run carries a person, the project and an agent; every Orchestrator row carries a counterpart or `several`; overhead = coordination ÷ (workers + coordination) is printed, and judged against §14.2's 10% claim **separately** |
| 6 | an AU project never makes a Global call; a token copied off its pod is refused | the gatekeeper's access log (phase 6 task 12), task 47's P28 | no log line for the AU project with `route: "global"`; the copied-token call answered `403 binding_mismatch` |
| 7 | a killed Orchestrator pod loses no lease, thread or directive | the before and after counts the collector takes around the kill | the three counts equal, and the queue's next turn consumed the events queued before the kill (task 27's test, run live) |
| 8 | every directive is a `dv-…` record that answers *"who asked for this?"* | directives, turn events | every directive has an `origin`, and its `read` ids resolve to messages or human messages in the export; for `origin: "fleet"`, at least one `read` id is a peer message |

**Plus, not one of the eight but required by the scope:**
- **The C6 rate** (§5.3, §14.2): of N first edits by workers, how many a `panacea check` on that
  path preceded. Printed as `k of N first edits were preceded by a check`, with the hook on and,
  in one session with the hook off, again. It has no pass mark; it is a measured number the owner
  reads (**not** an "N of M" on a product surface, so OD-27 does not apply to this record).
- **The gate before the first real teammate** (§12.1, `07` §3, `04` §8.2): Z5 passes (task 43's
  matrix), the attach tests pass (task 39), audit streams to Sentinel (task 45), the lane-mirror
  isolation is in place **if TM2 has started** (task 58), and KPMG InfoSec has approved the product
  managed file (owner question Q12). Any gate row not `PASS` means no real teammate is invited,
  whatever the eight say.

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) Verdicts are Go, over an export, with no cluster.** Phase 8's proving scripts split a
collector from a verdict so the verdicts can be tested on canned text (`task-47.md:299-302`). This
task does the same, but the verdicts are Go (`control-plane/internal/fleet/proof`), because they
join records across five sources and `jq` is not on the owner's machine (checked 2026-09-25: `command -v jq`
exits 1 in Git Bash here; `go` is present). The collector is shell and only fetches; the verdict program only
reads files. So every verdict has a Go test with the mutant it kills, and the run itself adds no
code path that a test has not covered.

**(b) The auditor reads; nobody writes.** The collector authenticates as a third principal holding
the org *auditor* capability (task 43), which reads escalations, directives, human messages and
thread messages across owners (routes 63, 66, 68, 71) and nothing else. Neither person's token is
used to read the other's rows: that would be the very cross-owner read Z5 refuses.

**(c) The script is fixed before the run.** The two people follow `infra/smoke/team/tm1-script.md`,
written in this task: the project, the two opening prompts (both touching `web/auth/session.go` in this repository, so
the overlap is certain; task 33's scripted conflict may name another path), which of them answers
the `decide` and how, when the Orchestrator pod is killed (between two turns, by the collector's
`kill` step), and when G9's corpus is replayed. A free-form run would prove only that nothing broke
that day. The opening prompts are private (OD-24): the script names their **intent**, and each
person types their own words.

**(d) Every criterion also fails loudly.** A verdict never passes on absence: criterion 1 with no
conflict is `FAIL (no conflict raised)`, criterion 3 with no directive at all is `FAIL (no
directive in the run)`, criterion 6 with no AU log line is `FAIL (no AU call observed)`. A proof
whose evidence is empty proves nothing, and saying `PASS` then would be the unverified claim the
working agreements forbid.

**(e) The G9 contingency is a budget, not a retry loop.** If criterion 3 fails, the three days go
to task 41's instructions and task 31's taint rule, in that order, and the run is repeated in full.
The record keeps every run's table, in order; a later `PASS` never overwrites an earlier `FAIL`.
If the three days are spent and 3 still fails, TM1 is not done and the owner is told, with the
failing corpus lines.

**Files:**
- `docs/verification/2026-09-25-phase-t-tm1-results.md` — the record: the run's header, the eight
  rows, C6, the gate rows, observations (new)
- `infra/smoke/team/tm1-script.md` — the fixed run script (new)
- `infra/smoke/team/tm1-collect.sh` — the collector: exports the run as JSON files (new)
- `control-plane/internal/fleet/proof/{export.go,tm1.go}` — `Export`, `Load`, `Verdict`, `TM1`
  (new)
- `control-plane/internal/fleet/proof/tm1_test.go`, `.../proof/testdata/tm1/{pass,…}/*.json` (new)
- `control-plane/cmd/teamproof/main.go` — `teamproof tm1 -dir <export>` prints the table (new)

The skeleton's file map names only the record for this task; the collector, script and verdict
program are added here, under task 33's `infra/smoke/team/` and Fleet's tree, and listed as gap 1.

**Interfaces:**
- Produces: `proof.Export`, `proof.Load(dir)`, `proof.Verdict`, `proof.TM1(Export) []Verdict`,
  `proof.Gate(Export) []Verdict`, `proof.C6(Export) C6Rate`, `proof.C6Row(Export) Verdict`; the
  record's row format. Task 69's TM2 proof adds its rows to the same package.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 03: `hubapi.AgentStart`, `Conflict`, `Thread`, `Message`, `Escalation`, `Directive`,
    `TurnOrigin`, `EscalationKind` (`task-03.md:573-836`);
  - task 01: routes 55, 59, 63, 66, 68, 71 and their auditor rows (`task-01.md:266-282`);
  - task 29: the DECIDED WITHOUT YOU template prefix (`task-29.md:296`);
  - phase 6 tasks 09 and 12: the usage row's person, project, agent and counterpart tags, and the
    gatekeeper's `slog` line (`session`, `project`, `route`, `path`, `status`, `ms`,
    `phase-6 …/2026-09-25-phase-6-gateway.md:242`);
  - task 42 (**not yet written; required of task 42**): G9's `report.json` with `freeTextFleet`
    and `outOfScope` counts; task 38 (**not yet written**): its refusal test's name;
  - task 47: P25 and P28's verdict lines (`task-47.md:308`, `:311`);
  - Hub task 27 (**UNVERIFIED until merge**): the audit route that lists `turn.recorded` events.

#### `export.go` (complete)

One JSON file per source in the export directory; `Load` refuses a directory missing any of them,
so a half-collected run cannot be judged.

```go
// Package proof judges a team-mode run from what it left behind. It reads
// files only: no network, no cluster, no clock.
package proof

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

type Person struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

// TurnEvent is one turn.recorded audit event's metadata (route 69).
type TurnEvent struct {
	ID                  string            `json:"id"`
	OrchestratorAgentID string            `json:"orchestratorAgentId"`
	OwnerUserID         string            `json:"ownerUserId"`
	Origin              hubapi.TurnOrigin `json:"origin"`
	Consumed            []string          `json:"consumed"`
	Issued              []string          `json:"issued"`
	At                  string            `json:"at"`
}

// UsageLine is one gateway usage row, by its tags (phase 6 task 09).
type UsageLine struct {
	Person      string `json:"person"`
	Project     string `json:"project"`
	Agent       string `json:"agent"`
	Class       string `json:"class"`       // solo | worker | orchestrator
	Counterpart string `json:"counterpart"` // a userId, "several", or "" for a worker
	Input       int64  `json:"input"`
	Output      int64  `json:"output"`
}

// GateLine is one gatekeeper access-log line (phase 6 task 12's slog fields).
type GateLine struct {
	Session string `json:"session"`
	Project string `json:"project"`
	Route   string `json:"route"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
}

// Probes are the results the run's scripts wrote: G9 (task 42), the PTY
// writer test (task 38), the copied token, the pod kill, C6, and the gate.
type Probes struct {
	G9FreeTextFleet  *int              `json:"g9FreeTextFleet"` // nil: G9 not run
	G9OutOfScope     *int              `json:"g9OutOfScope"`
	PTYRefusalPassed *bool             `json:"ptyRefusalPassed"`
	CopiedToken      *GateLine         `json:"copiedToken"`
	PodKill          *PodKill          `json:"podKill"`
	FirstEdits       []FirstEdit       `json:"firstEdits"`
	Gate             map[string]string `json:"gate"` // row → "pass" | "fail" | "n/a" | ""
}

type Counts struct {
	Leases     int `json:"leases"`
	Threads    int `json:"threads"`
	Directives int `json:"directives"`
}

type PodKill struct {
	Before        Counts   `json:"before"`
	After         Counts   `json:"after"`
	QueuedBefore  []string `json:"queuedBefore"`  // event ids queued at the kill
	ConsumedAfter []string `json:"consumedAfter"` // the next turn's consumed ids
}

type FirstEdit struct {
	AgentID       string `json:"agentId"`
	Path          string `json:"path"`
	CheckedBefore bool   `json:"checkedBefore"`
	HookOn        bool   `json:"hookOn"`
}

// Export is one run. Messages are keyed by thread id.
type Export struct {
	RunID       string                      `json:"runId"`
	ProjectID   string                      `json:"projectId"`
	AUProjectID string                      `json:"auProjectId"` // criterion 6's second project
	People      [2]Person                   `json:"people"`
	Agents      []hubapi.AgentStart         `json:"-"`
	Conflicts   []hubapi.Conflict           `json:"-"`
	Threads     []hubapi.Thread             `json:"-"`
	Messages    map[string][]hubapi.Message `json:"-"`
	HumanMsgIDs []string                    `json:"-"`
	Escalations []hubapi.Escalation         `json:"-"`
	Directives  []hubapi.Directive          `json:"-"`
	Turns       []TurnEvent                 `json:"-"`
	Usage       []UsageLine                 `json:"-"`
	Gatekeeper  []GateLine                  `json:"-"`
	Probes      Probes                      `json:"-"`
}

// Files is every file Load needs, and the field each fills.
var Files = []string{"run.json", "agents.json", "conflicts.json", "threads.json",
	"messages.json", "human-messages.json", "escalations.json", "directives.json",
	"turns.json", "usage.json", "gatekeeper.json", "probes.json"}

func Load(dir string) (Export, error) {
	var e Export
	dst := []any{&e, &e.Agents, &e.Conflicts, &e.Threads, &e.Messages, &e.HumanMsgIDs,
		&e.Escalations, &e.Directives, &e.Turns, &e.Usage, &e.Gatekeeper, &e.Probes}
	for i, name := range Files {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return Export{}, fmt.Errorf("proof: %s: %w (a half-collected run is not judged)", name, err)
		}
		if err := json.Unmarshal(b, dst[i]); err != nil {
			return Export{}, fmt.Errorf("proof: %s: %w", name, err)
		}
	}
	return e, nil
}
```

`human-messages.json` holds the ids only (route 71 as auditor): criterion 8 needs to know that a
`read` id is a real human message, and never needs its text. The collector writes no message text
it does not have to: `messages.json` keeps `text` because route 63 returns it, and the export
directory is deleted after the verdict (step 6), since it holds two people's agents' words.

#### `tm1.go` (complete public surface; criteria 1, 2, 3, 6 and 8 in full)

```go
package proof

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

type Result string

const (
	Pass   Result = "PASS"
	Fail   Result = "FAIL"
	NotRun Result = "NOT RUN"
)

// Verdict is one row of the record. Evidence is ids only, never text.
type Verdict struct {
	Row      string   // "1".."8", "C6", "G-Z5", …
	Result   Result
	Why      string   // one line; on FAIL, the first thing that failed
	Evidence []string // cf-…, es-…, dv-…, th-…, session ids
}

const decidedPrefix = "DECIDED WITHOUT YOU:" // task-29.md:296

// TM1 returns the eight rows, in order. It never returns fewer.
func TM1(e Export) []Verdict {
	return []Verdict{c1(e), c2(e), c3(e), c4(e), c5(e), c6(e), c7(e), c8(e)}
}

func ts(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }

// c1: the first conflict is tier P and predates every worker start.
func c1(e Export) Verdict {
	v := Verdict{Row: "1"}
	if len(e.Conflicts) == 0 {
		return fail(v, "no conflict raised")
	}
	cs := append([]hubapi.Conflict(nil), e.Conflicts...)
	sort.Slice(cs, func(i, j int) bool { return ts(cs[i].CreatedAt).Before(ts(cs[j].CreatedAt)) })
	first := cs[0]
	v.Evidence = []string{first.ID}
	if first.Tier != hubapi.TierP {
		return fail(v, fmt.Sprintf("first conflict is tier %s, not P", first.Tier))
	}
	workers := 0
	for _, a := range e.Agents {
		if a.Role != hubapi.AgentRoleWorker {
			continue
		}
		workers++
		if !ts(first.CreatedAt).Before(ts(a.StartedAt)) {
			v.Evidence = append(v.Evidence, a.ID)
			return fail(v, "worker "+a.ID+" started before the conflict was raised")
		}
	}
	if workers == 0 {
		return fail(v, "no worker started, so 'before' was never tested")
	}
	v.Result, v.Why = Pass, fmt.Sprintf("P raised before all %d workers started", workers)
	return v
}

func fail(v Verdict, why string) Verdict { v.Result, v.Why = Fail, why; return v }
```

`c1`'s `!Before` makes a conflict raised in the same instant as a spawn a failure: "before" is
strict. The constants are task 03's (`task-03.md:157`, `:217`, `:225-236`, `:266`).

```go
// c2: settled by yield or sequence; an agent inform to one person, a decide to
// the other; a Fleet-raised DECIDED WITHOUT YOU inform to each.
func c2(e Export) Verdict {
	v := Verdict{Row: "2"}
	var settled *hubapi.Conflict
	for i, c := range e.Conflicts {
		if c.State == hubapi.ConflictResolved && c.Resolution != nil &&
			(*c.Resolution == hubapi.ResolutionYield || *c.Resolution == hubapi.ResolutionSequence) {
			settled = &e.Conflicts[i]
			break
		}
	}
	if settled == nil {
		return fail(v, "no conflict resolved by yield or sequence")
	}
	v.Evidence = []string{settled.ID}
	informTo, decideTo, decided := map[string]string{}, map[string]string{}, map[string]string{}
	for _, x := range e.Escalations {
		switch {
		case x.FromAgentID == nil && x.Kind == hubapi.EscalationInform &&
			strings.HasPrefix(x.Question.Body, decidedPrefix):
			decided[x.OwnerUserID] = x.ID
		case x.FromAgentID != nil && x.Kind == hubapi.EscalationInform:
			informTo[x.OwnerUserID] = x.ID
		case x.FromAgentID != nil && x.Kind == hubapi.EscalationDecide:
			decideTo[x.OwnerUserID] = x.ID
		}
	}
	a, b := e.People[0].UserID, e.People[1].UserID
	split := (informTo[a] != "" && decideTo[b] != "") || (informTo[b] != "" && decideTo[a] != "")
	if !split {
		return fail(v, "no inform to one person with a decide to the other")
	}
	if decided[a] == "" || decided[b] == "" {
		return fail(v, "DECIDED WITHOUT YOU did not reach both people")
	}
	v.Evidence = append(v.Evidence, informTo[a]+informTo[b], decideTo[a]+decideTo[b], decided[a], decided[b])
	v.Result, v.Why = Pass, "settled as "+string(*settled.Resolution)+"; inform, decide, and both DECIDED WITHOUT YOU items"
	return v
}

// c3: no free-text directive in a Fleet-started turn; G9's out-of-scope count
// is zero; the PTY writer's refusal test passed on the deployed commit.
func c3(e Export) Verdict {
	v := Verdict{Row: "3"}
	if len(e.Directives) == 0 {
		return fail(v, "no directive in the run")
	}
	for _, d := range e.Directives {
		if d.Origin == hubapi.OriginFleet && d.Kind == hubapi.DirectiveFree {
			v.Evidence = []string{d.ID}
			return fail(v, "a free-text directive in a Fleet-started turn")
		}
	}
	p := e.Probes
	if p.G9FreeTextFleet == nil || p.G9OutOfScope == nil || p.PTYRefusalPassed == nil {
		v.Result, v.Why = NotRun, "G9's report or the PTY writer result is missing"
		return v
	}
	if *p.G9FreeTextFleet != 0 || *p.G9OutOfScope != 0 {
		return fail(v, fmt.Sprintf("G9: %d free-text, %d out of scope", *p.G9FreeTextFleet, *p.G9OutOfScope))
	}
	if !*p.PTYRefusalPassed {
		return fail(v, "the PTY writer accepted a non-template byte")
	}
	v.Result, v.Why = Pass, fmt.Sprintf("%d directives, none free-text from a Fleet turn; G9 clean", len(e.Directives))
	return v
}
```

`x.Question.Body` is `teamtext.Text`'s string (`task-02.md:229-230`). The prefix match is the only
place a verdict reads text, and it reads Fleet's own template, never an agent's words.

```go
// c6: no Global call for the AU project, at least one AU call seen, and the
// copied token refused as a binding mismatch.
func c6(e Export) Verdict {
	v := Verdict{Row: "6"}
	au := 0
	for _, g := range e.Gatekeeper {
		if g.Project != e.AUProjectID {
			continue
		}
		if g.Route == "global" {
			v.Evidence = []string{g.Session}
			return fail(v, "the AU project made a Global call")
		}
		if g.Route == "au" && g.Status == 200 {
			au++
		}
	}
	if au == 0 {
		return fail(v, "no AU call observed")
	}
	c := e.Probes.CopiedToken
	if c == nil {
		v.Result, v.Why = NotRun, "the copied-token probe did not run"
		return v
	}
	if c.Status != 403 || c.Code != "binding_mismatch" {
		return fail(v, fmt.Sprintf("the copied token got %d %s", c.Status, c.Code))
	}
	v.Result, v.Why = Pass, fmt.Sprintf("%d AU calls, no Global; copied token refused", au)
	return v
}

// c8: every directive names its origin, every id it read exists in the run,
// a Fleet-origin directive read at least one peer message, and some turn of
// the same origin issued it.
func c8(e Export) Verdict {
	v := Verdict{Row: "8"}
	if len(e.Directives) == 0 {
		return fail(v, "no directive in the run")
	}
	peer, human := map[string]bool{}, map[string]bool{}
	for _, ms := range e.Messages {
		for _, m := range ms {
			peer[m.ID] = true
		}
	}
	for _, id := range e.HumanMsgIDs {
		human[id] = true
	}
	issuedBy := map[string]hubapi.TurnOrigin{}
	for _, t := range e.Turns {
		for _, id := range t.Issued {
			issuedBy[id] = t.Origin
		}
	}
	for _, d := range e.Directives {
		v.Evidence = []string{d.ID}
		if d.Origin != hubapi.OriginHuman && d.Origin != hubapi.OriginFleet {
			return fail(v, "a directive with no turn origin")
		}
		if o, ok := issuedBy[d.ID]; !ok || o != d.Origin {
			return fail(v, "no turn of the same origin issued "+d.ID)
		}
		sawPeer := false
		for _, r := range d.Read {
			if !peer[r] && !human[r] {
				return fail(v, d.ID+" read "+r+", which is not in the run")
			}
			sawPeer = sawPeer || peer[r]
		}
		if d.Origin == hubapi.OriginFleet && !sawPeer {
			return fail(v, "a Fleet-origin directive that read no peer message")
		}
	}
	v.Evidence = nil
	v.Result, v.Why = Pass, fmt.Sprintf("all %d directives answer who asked", len(e.Directives))
	return v
}
```

`c6` counts only `200` AU lines, so a run whose AU calls all failed (the route down, which fails
closed) is not taken as proof that AU works. The copied-token probe is task 47's P25 run with a
token read out of the pod and replayed from the collector's own pod; its line is the gatekeeper's.

The remaining rows, by rule (each is 15 to 30 lines in the same shape; the rule is the contract):

- **`c4`.** A thread is *settled* when its state is `resolved` and no escalation with
  `fromAgentId: null` names its `threadId` other than a DECIDED WITHOUT YOU item (task 30's
  deadline escalation is Fleet-raised). `PASS` when settled threads are **more than half** of all
  threads, with at least one thread; `Why` prints the count in words (*"3 of 4 threads settled
  before their deadline"* is a record, not a product surface). A **hop** is two consecutive
  messages in one thread whose `senderUserId` differ; `Why` adds the median and p90 hop in seconds.
  A run with no hop is `FAIL (no negotiation happened)`.
- **`c5`.** `FAIL` on the first usage row with an empty `person`, `project` or `agent`, or with
  `class: "orchestrator"` and an empty `counterpart`; `FAIL` if there are no rows. Overhead is
  Σ(orchestrator input + output) ÷ Σ(all input + output), printed to one decimal as *"coordination
  was 6.2% of tokens"*. The 10% claim of §14.2 is a separate `Why` clause, *"within"* or *"over"*
  it, and does not change the row's result: criterion 5 is attribution; overhead is measured.
- **`c7`.** `NOT RUN` without `probes.podKill`. `FAIL` when `before` and `after` differ in any
  count, naming it, or when any `queuedBefore` id is missing from `consumedAfter`.
- **`C6(e) C6Rate`** returns `{HookOn, HookOff struct{Checked, Total int}}` from
  `probes.firstEdits`; its row is always `PASS` if `Total > 0` for the hook-on session, with the
  rate in `Why`, and `NOT RUN` otherwise. It has no threshold (§5.3: the hook is an enhancer).
- **`Gate(e) []Verdict`** returns five rows, `G-Z5`, `G-attach`, `G-sentinel`, `G-mirror`,
  `G-infosec`, from `probes.gate`: `"pass"` → `PASS`, `"fail"` → `FAIL`, `""` → `NOT RUN`.
  `"n/a"` is `PASS` **only** for `G-mirror`, and only while no TM2 task has started; the collector
  writes it from `git log --oneline -- control-plane/internal/fleet/mirror | wc -l` being 0.
  `G-infosec` is typed by the owner from KPMG InfoSec's written approval (Q12); nothing computes it.

#### `cmd/teamproof/main.go` (complete)

```go
// Command teamproof prints a team-mode run's verdicts as the record's rows.
//
//	teamproof tm1 -dir ./export
//
// Exit 0 only when every TM1 row and every gate row is PASS.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/proof"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "tm1" {
		fmt.Fprintln(os.Stderr, "usage: teamproof tm1 -dir <export>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("tm1", flag.ExitOnError)
	dir := fs.String("dir", "", "the collector's export directory")
	_ = fs.Parse(os.Args[2:])
	e, err := proof.Load(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rows := append(proof.TM1(e), proof.C6Row(e))
	rows = append(rows, proof.Gate(e)...)
	ok := true
	fmt.Printf("| Row | Result | Why | Evidence |\n|---|---|---|---|\n")
	for _, v := range rows {
		fmt.Printf("| %s | %s | %s | %s |\n", v.Row, v.Result, v.Why, strings.Join(v.Evidence, ", "))
		ok = ok && (v.Result == proof.Pass)
	}
	if !ok {
		os.Exit(1)
	}
}
```

`C6Row` wraps `C6` as a `Verdict`. The table is pasted into the record as printed; nothing is
retyped by hand.

- [ ] **Step 1: Write the failing tests first**

`control-plane/internal/fleet/proof/tm1_test.go`, `package proof`. Fixtures are directories
under `testdata/tm1/`: `pass/` is a complete, hand-written run that every row passes, and each
failing case is `pass/` with one file replaced, built by a helper `withFile(t, "pass", name, body)`
that copies `pass/` to `t.TempDir()`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPassFixturePassesEveryRow` | `TM1`, `C6Row` and `Gate` over `pass/` → all `PASS`; `TM1` has 8 rows | a verdict that fails its own happy path |
| `TestLoadRefusesAHalfCollectedRun` | `pass/` minus `probes.json` → an error naming `probes.json` | a missing file read as "no probes", then judged |
| `TestC1WorkerFirstFails` | one worker's `startedAt` 1 s before the conflict → `FAIL`, evidence holds that `ag-…` | comparing against the first worker only, or against the Orchestrators |
| `TestC1SameInstantFails` | equal timestamps → `FAIL` | `!After` in place of `Before` (a tie counted as "before") |
| `TestC1NoConflictFails` | empty `conflicts.json` → `FAIL (no conflict raised)` | a pass on absence (decision (d)) |
| `TestC2OneDecidedItemFails` | drop one DECIDED WITHOUT YOU → `FAIL` | checking that the item exists, not that it reached both |
| `TestC2BothInformsFails` | the decide becomes an inform → `FAIL` | counting escalations without their kind |
| `TestC2AgentWrittenDecidedIgnored` | the DECIDED WITHOUT YOU items carry `fromAgentId` → `FAIL` | an agent able to fake Fleet's line by typing its prefix |
| `TestC3FreeFleetDirectiveFails` | one `origin: "fleet"`, `kind: "free"` → `FAIL`, that `dv-…` | reading `kind` alone, or `origin` alone |
| `TestC3MissingG9IsNotRun` | `g9FreeTextFleet: null` → `NOT RUN`, not `PASS` | a nil read as zero |
| `TestC4NoHopFails` | every thread's messages from one user → `FAIL (no negotiation happened)` | a median over nothing |
| `TestC4DeadlineEscalationUnsettles` | a Fleet-raised non-DWY escalation on a resolved thread → that thread not settled | counting `resolved` alone, hiding a deadline default |
| `TestC5OrchestratorWithoutCounterpartFails` | one orchestrator row with `counterpart: ""` → `FAIL` | checking person and project only (§4.4's third tag) |
| `TestC5OverheadIsNotAPassMark` | overhead 14% with full attribution → `PASS`, `Why` holds *"over"* | the 10% claim turned into criterion 5's result |
| `TestC6GlobalLineFails` | one AU-project line with `route: "global"` → `FAIL`, that session | filtering on `status == 200` before the route check |
| `TestC6AllAUCallsFailedFails` | every AU line `503` → `FAIL (no AU call observed)` | a run with the route down taken as proof |
| `TestC6CopiedTokenWrongCodeFails` | `401 token_expired` → `FAIL` | any 4xx counted as "refused" (an expired token proves nothing about binding) |
| `TestC7LostLeaseFails` | `after.leases` one less → `FAIL` naming leases | comparing threads and directives only |
| `TestC8UnknownReadIdFails` | a `read` id absent from the run → `FAIL` | trusting a directive's own account of what it read |
| `TestC8FleetDirectiveWithoutPeerFails` | `origin: "fleet"`, `read` only human ids → `FAIL` | "who asked?" answered by a human id on a Fleet turn |
| `TestC8TurnOriginMismatchFails` | the issuing turn is `human`, the directive says `fleet` → `FAIL` | reading the directive's origin without its turn |
| `TestGateEmptyIsNotRun` | `gate: {}` → five `NOT RUN` rows | an unchecked gate read as passed |
| `TestGateNAOnlyForMirror` | `"G-Z5": "n/a"` → `FAIL`; `"G-mirror": "n/a"` → `PASS` | `n/a` accepted anywhere |
| `TestEvidenceHoldsNoText` | every `Evidence` entry over `pass/` matches `^(cf|es|dv|th|ms|ag|hm)-[0-9a-z]+$` or a session id | a verdict that pastes an agent's words into the record |

`cmd/teamproof/main_test.go`: over `pass/` it exits 0 and prints 14 table rows (8, C6, 5 gate);
over `withFile(…, "conflicts.json", "[]")` it exits 1 (kills an exit code that ignores `FAIL`).

- [ ] **Step 2: `export.go`, `tm1.go`, `cmd/teamproof`**, until every test passes.

- [ ] **Step 3: The collector, `infra/smoke/team/tm1-collect.sh`**

Owner-gated like phase 8 task 32's collectors (it talks to the real environment). It takes the
Hub and Fleet base URLs, the project ids and the run's start time from `tm1-run.env`, a file the
run script's first step writes, and an auditor token from `PANACEA_APP_PROOF_TOKEN_FILE`, read
with `cat` and never echoed. Four sub-commands, each writing into `$OUT` (default
`./tm1-export-<runId>`):

| Sub-command | When | Writes |
|---|---|---|
| `start` | before the two people begin | `run.json` (run id, project ids, the two people) |
| `kill` | at the script's kill point | counts before; deletes the owner's Orchestrator pod with `kubectl delete pod -l panacea-app/class=orchestrator,panacea-app/owner-hash=<h> --wait=false`; waits for the next `turn.recorded`; counts after; the queued and consumed ids → the `podKill` half of `probes.json` |
| `export` | after the run | the Hub files through routes 55, 59, 63 (per thread id found in conflicts and escalations), 66, 68, 71 and Hub task 27's audit route; `usage.json` from phase 6's keysmith `GET /v1/usage` rows **with their tags** (a raw row export, required of phase 6 task 09, gap 3); `gatekeeper.json` from the gatekeeper's logs for the run's window (`kubectl logs -n panacea-gateway -l app=gatekeeper --since-time=…`, JSON lines filtered by project) |
| `probes` | after `export` | merges G9's `report.json` (task 42), task 38's test result for the deployed commit, the copied-token line (task 47's P25 collector), C6's first edits, and the gate map, into `probes.json` |

Every `curl` has `--fail-with-body --max-time 20`; on Windows it also takes `--ssl-no-revoke`
(the corporate network's revocation check, owner memory, **UNVERIFIED** in this tree). A failed
fetch exits non-zero naming the route and leaves the file absent, so `Load` refuses the export
(decision (d)). The collector writes no token to any file under `$OUT`.

**C6's first edits** come from Fleet's activity table (task 35) joined with the verbs log (task
22): for each worker, the first path its observed edits touched, and whether a `check` verb on
that path came from the same agent before the edit's sample time. The join runs in Fleet's own
admin route `GET /v1/admin/proof/first-edits?project=` (**required of task 35**, gap 4), which
returns ids, paths and booleans only.

- [ ] **Step 4: The run script, `infra/smoke/team/tm1-script.md`**

Fixed text, read aloud by the two people (decision (c)); a summary of its steps:

1. **Setup (10 min).** Dylan runs `tm1-collect.sh start`. He shares this repository's project into
   a new team, choosing the class *Internal, may use Global* (task 56); invites Priya. Priya signs in
   on her own desktop and opens the project. A second project, class *Confidential, Australia
   only*, is shared the same way for criterion 6.
2. **The overlap (20 min).** Each tells their own Orchestrator, in their own words, to change
   session expiry in `web/auth/session.go`: Dylan to shorten it, Priya to add a refresh path,
   within a minute of each other. Both watch their strip. Neither holds a spawn back by hand:
   criterion 1 tests that task 24's plan-time intent leases order it, not the people.
3. **The settlement (20 min).** Whichever person receives the `decide` picks the option that sequences
   the two; the other reads their `inform`. Both confirm DECIDED WITHOUT YOU on their own Desk.
4. **The kill (5 min).** With both Orchestrators idle between turns, the operator runs
   `tm1-collect.sh kill`. Dylan then sends his Orchestrator one message.
5. **AU (10 min).** Priya runs one worker on the AU project; the operator runs task 47's P25 with
   the copied token.
6. **G9 (15 min).** The operator runs task 42's scripted two-Orchestrator run against the same
   Fleet, in a third project, so its hostile messages never reach the two people.
7. **Collect (10 min).** `tm1-collect.sh export`, then `probes`, then `go run
   ./control-plane/cmd/teamproof tm1 -dir <export>`.

Neither person is asked to comment on how it felt during the run; observations are written after
step 7, each person on their own, in the record's *Observations* section.

- [ ] **Step 5: The record, `docs/verification/2026-09-25-phase-t-tm1-results.md`** (complete as
  committed; every result cell is `NOT RUN` until a run fills it)

```markdown
# Phase T — TM1 results: two people, one repo, on the real environment

Plan: `docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`, task 57.
Design: `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` §12.1.

**Status: NOT RUN.** No real teammate is invited until every gate row below is PASS.

## Runs

Each run is appended, never replaced. A later PASS does not remove an earlier FAIL.

### Run 1 — NOT RUN

| Field | Value |
|---|---|
| Run id | — |
| Date (UTC) | — |
| People | — (two, by role: the sharer and the joiner) |
| Deployed commit | — (`git rev-parse HEAD` of the images the cluster runs) |
| Projects | — (the team project; the AU-only project; G9's project) |
| Operator | — |

Paste `teamproof tm1` output here, unedited:

| Row | Result | Why | Evidence |
|---|---|---|---|
| 1 | NOT RUN | | |
| 2 | NOT RUN | | |
| 3 | NOT RUN | | |
| 4 | NOT RUN | | |
| 5 | NOT RUN | | |
| 6 | NOT RUN | | |
| 7 | NOT RUN | | |
| 8 | NOT RUN | | |
| C6 | NOT RUN | | |
| G-Z5 | NOT RUN | | |
| G-attach | NOT RUN | | |
| G-sentinel | NOT RUN | | |
| G-mirror | NOT RUN | | |
| G-infosec | NOT RUN | | |

**G9 contingency used:** 0 of 3 days. (A record, not a product surface.)

### Observations

Each person, on their own, after the run. Written as what was seen, never as evidence.

- The sharer: —
- The joiner: —

## What these results do not show

- That the fences stop a determined attacker (`05-design.md` §7.3).
- That the product works for more than two people, or on more than one repository.
- Anything about TM2's mirror, tier M or the team land queue (task 69).
```

The record is committed **empty** in this task; the run fills it. The people are named by role
in the committed file; their names may be written in by the owner.

- [ ] **Step 6: Acceptance (in this task, without the cluster)**

```bash
cd control-plane
go test ./internal/fleet/proof/... ./cmd/teamproof/... 2>&1 | tail -3   # ok, ok
go vet ./internal/fleet/proof/... ./cmd/teamproof/...                    # no output
go run ./cmd/teamproof tm1 -dir internal/fleet/proof/testdata/tm1/pass | tail -1; echo "exit=$?"
                                                                        # | G-infosec | PASS | …   exit=0
bash -n ../infra/smoke/team/tm1-collect.sh && echo syntax-ok             # syntax-ok
grep -c "NOT RUN" ../docs/verification/2026-09-25-phase-t-tm1-results.md # 16 (status, run heading, 14 rows)
```

**On the cluster** (the run, owner-gated, two people): the script in step 4, then the record. The
export directory is deleted after the table is pasted: `rm -rf ./tm1-export-<runId>`, since it
holds two people's agents' words.

- [ ] **Step 7: Commit** by pathspec: the record, the script, the collector, the proof package
  with its testdata, and `cmd/teamproof`.

**What this task does NOT claim:**
- That TM1 is done. This task writes the proof; only a run with every row `PASS` says so.
- That two people are representative. The run is the design's minimum (§12.1), not a user study.
- That criterion 4's threshold is the design's. §12.1 says "the median thread resolves without a
  deadline escalation"; *more than half* is this part's reading of "median", stated so it can be
  argued.
- That the collector's `kubectl` selectors are right. `panacea-app/class` is task 15's label;
  `panacea-app/owner-hash` is **UNVERIFIED** until task 15's part is complete.
- That the verdicts see what the product hid. They read the Hub, the gateway and the logs; a
  fault that writes nothing to any of the three is outside this proof.

**Gaps found in the skeleton and in sibling parts:**
1. The file map lists only the record for task 57; the collector, run script and `proof` package
   are added here.
2. Task 01's route table has no thread list route (`task-01.md:271-274`); the collector finds
   thread ids from conflicts, escalations and the audit route.
3. Phase 6 task 09 must export raw usage rows **with** their `agent`, `class` and `counterpart`
   tags, not only rollups by person, project and team.
4. Task 35 must add `GET /v1/admin/proof/first-edits` (ids, paths, booleans), or C6 cannot be
   measured from records.
5. Task 42 must write `report.json` with `freeTextFleet` and `outOfScope` counts; task 38 must
   name its refusal test so the collector can find its result.

