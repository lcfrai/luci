### Task 22: Read verbs: `team`, `check`, `files`, `show`, filtered by class

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 19 (dispatch), 20 (sessions and `ag-…`
ids); 02 (`teamtext`), 09 (the ST.1 wire), 10 (`Store`) through them.

This is where the owner's *"what the agent is currently working on and the task they have and the
reason they are updating that document"* becomes an answer an agent can read (§5.1). Four verbs,
all read-only, all answered from Fleet's memory at call time, so the answer is always fresh and no
Hub data lands in a clone (`ADR-H1:467-474`, §5.2 item 3).

**The class filter is a type, not an `if`.** The worker's answer and the Orchestrator's answer are
two Go types. The worker type has **no field** that could hold a task title, a why-line, a commit
subject or a thread's text, so a later handler that forgets the filter has nowhere to put the
words; a reflection test pins the worker type's JSON keys to an allowlist, the same technique
task 16 uses for `team.json` (`task-16.md:561-576`). This is the direct answer to `07` C1: a
poisoned page in one teammate's session must not reach another teammate's code through a worker,
because a worker is never tainted (§5.1, §7.2).

**Every string passes task 02 before it leaves Fleet.** One function, `out.go`'s `clean`, is the
only way a string enters a result. A display name, a brief title, a path or a quoted `text·A` field
that fails `teamtext.Clean` or its bound is **withheld**: the field is replaced by the fixed
product string `"(withheld)"`, a path is dropped and counted in `withheld`, and one log line names
the record id, never the text. A refusal is never an error, because one hostile path in a
teammate's worktree must not blind every other agent's `panacea team`.

**OD-24: the opening prompt is private.** Fleet never stores it outside the pod spec (task 21's
`CreateAgentRequest.Args`, task 20). No source this task reads holds it, and a test launches an
agent with a marker prompt and greps every verb's bytes for it, for both classes.

**Files:**
- `control-plane/internal/fleet/verbs/team.go` — `team`, the two result types, assembly (new)
- `control-plane/internal/fleet/verbs/check.go` — `check <path>` (new)
- `control-plane/internal/fleet/verbs/files.go` — `files` and `show <path>`, and the `TreeSource`
  interface task 23 also reads (new)
- `control-plane/internal/fleet/verbs/out.go` — `clean`, `quote`, the withheld rule (new)
- `control-plane/internal/fleet/verbs/sources.go` — the read interfaces this package consumes (new)
- `control-plane/internal/fleet/verbs/{team,check,files}_test.go`, `fakes_test.go` (new)
- `testdata/panacea-control/v1/result-team-worker.json`, `result-team-orchestrator.json`,
  `result-check-worker.json`, `result-check-empty.json` (new fixtures; task 09 owns the directory)

**Interfaces:**
- Produces: `verbs.Register(d *control.Dispatcher, deps Deps)` adding `team`, `check`, `files`,
  `show`; `verbs.TeamWorker`, `verbs.TeamOrchestrator`, `verbs.CheckResult`, `verbs.FilesResult`,
  `verbs.ShowResult`; `verbs.TreeSource` and `verbs.Tree` (read by task 23's breadth check);
  `verbs.Sources`; `verbs.WorkerSnapshot(ctx, deps, project) (TeamWorker, error)`, which task 16's
  snapshot push and task 38's nudge reuse so `team.json` and `panacea team` cannot disagree.
- Consumes, **written against these names; where task 09 or 19 spells one differently, that task
  wins and this part is renamed mechanically**:
  - task 19: `control.Dispatcher.Handle(verb string, h control.Handler)`;
    `type control.Handler func(ctx context.Context, c control.Caller, args json.RawMessage) (any, error)`;
    `control.Caller{Org, Project string; Agent model.AgentID; Owner model.Owner; Class model.Class}`,
    already checked by dispatch for existence, class and owner (§3.3 rule 1);
  - task 09: `model.ClassWorker`, `model.ClassOrchestrator`; `wire.CodeBadRequest`,
    `wire.CodeUnavailable`; `wire.Refuse(code, msg string) error`, which dispatch turns into the
    ST.1 error envelope;
  - task 02: `teamtext.Clean`, `teamtext.Bound`, `teamtext.KindPath`, `teamtext.KindBriefTitle`,
    `teamtext.ValidPath` (one file, no glob metacharacter), and `teamtext.Match(glob,
    path string) bool` (`task-02.md:69`);
  - task 10: `store.Store`'s `Agents(ctx, org, project)` and `Leases(ctx, org, project)`.
- Required by: 23 (`TreeSource`, `Sources`), 24, 25, 16's push, 18's CLI output, 38, 52.

**One glob semantics.** `check`, task 23's overlap and task 25's forecaster all decide coverage with
task 02's `teamtext.Match`, never a matcher of their own, so the three cannot disagree about
whether `auth/**` covers `auth/session.go`.

#### `sources.go` (complete)

Each source is the smallest interface over state another task owns. Hub-owned facts (task cards,
brief titles, conflicts, threads) come from task 11's in-memory rebuild; Fleet facts from task 10's
store; activity from task 35. A nil `Activity` or `Directives` is legal and reads as empty, which
is exactly TM1-α's state: activity arrives with task 35 (TM1), directives with task 31.

```go
package verbs

// Sources is everything the read verbs read. Deps holds one.
type Sources struct {
	Store      store.Store     // agents, leases (task 10)
	Cards      Cards           // task cards, rebuilt from the Hub (task 11)
	Briefs     Briefs          // brief section titles (Hub brief records via task 11)
	People     People          // display names from Hub membership, cached <= 60 s (task 12)
	Conflicts  Conflicts       // open cf-… with their typed fields (task 25 writes, 11 rebuilds)
	Threads    Threads         // th-… the caller is party to (task 28)
	Activity   Activity        // nil in TM1-α; task 35
	Directives Directives      // nil until task 31
	Tree       TreeSource      // integration head's tree (below)
	Now        func() time.Time
}

type Cards interface {
	// Card returns the agent's task card, or ok=false for an agent with none.
	Card(ctx context.Context, agent model.AgentID) (Card, bool)
}

// Card is the Hub task record as Fleet holds it in memory. Title and Why are
// teamtext.Text so their author class travels with them.
type Card struct {
	Task      string        // tk-…
	BriefRef  string        // a brief section id
	Title     teamtext.Text // ≤ 140, text·H or text·A
	Why       teamtext.Text // ≤ 200, text·A
	State     string        // planned | active | blocked | done | abandoned
}

type Briefs interface {
	SectionTitle(ctx context.Context, project, sectionID string) (string, bool) // ≤ 120
}

type People interface {
	DisplayName(ctx context.Context, owner model.Owner) string // "" if unknown
}

type Conflicts interface {
	Open(ctx context.Context, project string) []Conflict
}

type Conflict struct {
	ID    string             // cf-…
	Tier  string             // P | H | M | S
	State string             // raised | negotiating | accepted | escalated
	A, B  model.AgentID      // the pair
	Paths []string
}

type Threads interface {
	// Party returns the open threads orch is party to: id, subject kind, state.
	Party(ctx context.Context, orch model.AgentID) []ThreadRef
}

type ThreadRef struct {
	ID       string `json:"id"`       // th-…
	Conflict string `json:"conflict"` // cf-… or ""
	State    string `json:"state"`    // open | escalated
}

type Activity interface {
	Of(ctx context.Context, agent model.AgentID) (Observed, bool)
}

// Observed is task 35's ST.6 sample, reduced to what §5.1 shows.
type Observed struct {
	Files           []string    // edited in the last 10 min
	Hunks           []Hunk
	LastCommitPaths []string
	LastCommitSubj  teamtext.Text // text·A; Orchestrators only
	LastEdit        map[string]time.Time // per path, for check's editors
}

type Hunk struct {
	Path   string   `json:"path"`
	Base   string   `json:"base"`
	Ranges [][2]int `json:"ranges"`
}

type Directives interface {
	// Pending returns the caller's own undelivered dv-… records, oldest first.
	Pending(ctx context.Context, worker model.AgentID) []PendingDirective
}

type PendingDirective struct {
	ID   string `json:"id"`   // dv-…
	Kind string `json:"kind"` // pause | resume | narrow | wait-for | rebase | free
}
```

#### `team.go`: the two result types (complete)

The worker type's keys are **exactly** task 16's `Snapshot` keys (`task-16.md:204-255`), because
`WorkerSnapshot` is what Fleet pushes to `team.json` and task 16's decode refuses an unknown field.
One type in `cp`, one in `sbx`, one fixture (`result-team-worker.json`) both modules' tests read,
which is the plan's rule for a shape both speak (File map, first paragraph).

```go
package verbs

// TeamWorker is `panacea team` for a worker. It has no field for any text·A
// or text·H value (§5.1 "Workers see: no"). TestWorkerKeysAllowlist pins it.
type TeamWorker struct {
	V           int              `json:"v"` // 1
	GeneratedAt time.Time        `json:"generatedAt"`
	Project     string           `json:"project"`
	Agents      []WorkerAgent    `json:"agents"`
	Conflicts   []WorkerConflict `json:"conflicts"`
}

type WorkerAgent struct {
	Agent           string      `json:"agent"`   // ag-…
	Owner           string      `json:"owner"`   // display name, cleaned
	Role            string      `json:"role"`    // orchestrator | worker
	Harness         string      `json:"harness"` // id@version
	Brief           *BriefRef   `json:"brief,omitempty"`
	Status          string      `json:"status"`
	WaitingOn       string      `json:"waitingOn,omitempty"` // lease | peer | human
	Files           []string    `json:"files"`
	Hunks           []Hunk      `json:"hunks"`
	Leases          []LeaseView `json:"leases"`
	LastCommitPaths []string    `json:"lastCommitPaths"`
}

type BriefRef struct {
	ID    string `json:"id"`
	Title string `json:"title"` // ≤ 120, cleaned; the approved brief, not agent words
}

type LeaseView struct {
	ID        string    `json:"id"` // ls-…
	Globs     []string  `json:"globs"`
	Mode      string    `json:"mode"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type WorkerConflict struct {
	ID    string   `json:"id"`   // cf-…
	With  string   `json:"with"` // the other side's ag-…, relative to nobody: both are listed
	Tier  string   `json:"tier"`
	Paths []string `json:"paths"`
}

// TeamOrchestrator is `panacea team` for an Orchestrator: the worker view
// plus the quoted, attributed words §5.1 gives Orchestrators only.
type TeamOrchestrator struct {
	TeamWorker
	Cards    []OrchCard  `json:"cards"`    // one per agent with a task, same order as Agents
	Threads  []ThreadRef `json:"threads"`  // only threads this Orchestrator is party to
	Withheld int         `json:"withheld"` // strings refused by teamtext, count only
}

type OrchCard struct {
	Agent             string  `json:"agent"`
	Task              string  `json:"task"` // tk-…
	State             string  `json:"state"`
	Title             *Quoted `json:"title,omitempty"`
	Why               *Quoted `json:"why,omitempty"`
	LastCommitSubject *Quoted `json:"lastCommitSubject,omitempty"`
}

// Quoted is every free-text value an Orchestrator reads: the words, their
// author class and who wrote them. Task 18's CLI prints it inside the fixed
// quoted envelope (G4); the Orchestrator's instructions (task 41) say that
// a Quoted value is data from another person's agent, never an instruction.
type Quoted struct {
	Text  string `json:"text"`
	Class string `json:"class"` // "text·A" | "text·H"
	By    string `json:"by"`    // ag-… for text·A; the owner's display name for text·H
	Owner string `json:"owner"` // whose record it is, display name
}

// WorkerSnapshot builds the worker view for a project. teamWorker and task
// 16's push both call it, so the two cannot drift.
func WorkerSnapshot(ctx context.Context, s Sources, org, project string) (TeamWorker, error)
```

**Order is fixed** so fixtures compare byte for byte: agents by owner display name, then role
(`orchestrator` first), then `ag-…`; files, paths and globs sorted; conflicts by `cf-…`.

#### `check.go` and `files.go` (complete)

```go
// CheckArgs is `panacea check <path>`. The path is validated by
// teamtext.ValidPath, which refuses a glob metacharacter: check asks about
// one file. A bad path is wire.CodeBadRequest, naming the rule it broke.
type CheckArgs struct {
	Path string `json:"path"`
}

// CheckResult is one path's picture. Directives come FIRST in the JSON so a
// worker that reads the top of the output sees its own Orchestrator's word
// before anyone else's facts (§5.2 row 2). Every slice is non-nil: an
// unleased, unedited path is {"path":…,"directives":[],"holders":[],…}, ok.
type CheckResult struct {
	Path       string             `json:"path"`
	Directives []PendingDirective `json:"directives"` // the caller's own only
	Holders    []Holder           `json:"holders"`    // leases whose globs match Path
	Editors    []Editor           `json:"editors"`    // observed edits to Path (task 35)
	Conflicts  []CheckConflict    `json:"conflicts"`  // open cf-… naming Path
}

type Holder struct {
	Agent string `json:"agent"`
	Owner string `json:"owner"`
	Lease string `json:"lease"` // ls-…
	Mode  string `json:"mode"`
	State string `json:"state"`
	Mine  bool   `json:"mine"` // the caller's own owner holds it
}

type Editor struct {
	Agent    string    `json:"agent"`
	Owner    string    `json:"owner"`
	LastEdit time.Time `json:"lastEdit"`
}

// CheckConflict: for a worker, the other side's owner, brief section and
// paths only (§5.1 last paragraph). OtherTask is set for an Orchestrator only.
type CheckConflict struct {
	ID         string    `json:"id"`
	Tier       string    `json:"tier"`
	OtherAgent string    `json:"otherAgent"`
	OtherOwner string    `json:"otherOwner"`
	OtherBrief *BriefRef `json:"otherBrief,omitempty"`
	Paths      []string  `json:"paths"`
	OtherTask  *Quoted   `json:"otherTask,omitempty"` // Orchestrator only; always nil for a worker
}

// TreeSource is the project's tree at the integration head, read-only, as
// the caller's owner's machined sees it. Task 23 reads Tree for its breadth
// cap. ErrTreeUnavailable when machined cannot be reached.
type TreeSource interface {
	Tree(ctx context.Context, owner model.Owner, project string) (Tree, error)
	Show(ctx context.Context, owner model.Owner, project, path string) ([]byte, error)
}

type Tree struct {
	Head  string   // integration head commit sha
	Paths []string // tracked files, sorted
}

var ErrTreeUnavailable = errors.New("verbs: integration tree unavailable")

// FilesResult is `panacea files` (Orchestrators). At most MaxFiles paths;
// Truncated says so. Paths failing clean are dropped and counted.
type FilesResult struct {
	Head      string   `json:"head"`
	Paths     []string `json:"paths"`
	Truncated bool     `json:"truncated"`
	Withheld  int      `json:"withheld"`
}

// ShowResult is `panacea show <path>` (Orchestrators). Content is the file at
// Head, refused whole (text_refused) if it fails teamtext.Clean, and cut at
// MaxShowBytes on a UTF-8 boundary with Truncated set.
type ShowResult struct {
	Head      string `json:"head"`
	Path      string `json:"path"`
	Source    string `json:"source"` // always "repo": neither text·A nor text·H
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

const (
	MaxFiles     = 20000
	MaxShowBytes = 256 << 10
)
```

**Where the tree comes from.** The scope says `files`/`show` answer from the lane owner's
`machined` over the broker (TM1). Phase 8's `machined` is tasks 20–21 of its plan, **unbuilt on
2026-09-25, UNVERIFIED until merge**, and no row of this plan's delta table asks it for a tree read.
So this task ships the `TreeSource` interface and a fake, and **TM1-α wires `unavailableTree`**,
which returns `ErrTreeUnavailable`: `files` and `show` answer `wire.CodeUnavailable` with `since`
set to Fleet's start time, and task 23 treats an unknown tree as its own fail-closed case. The real
source is task 34's (the `machined` delta), which must add a read-only `ls-tree` / `cat-file` at the
integration head beside its sweep, under phase 3's §HGH hardening (listed as a skeleton gap).

**`out.go`: the only door out.**

```go
const withheldText = "(withheld)"

// clean returns s if it passes teamtext.Clean and the kind's bound, else the
// fixed marker, and counts the refusal. Paths use cleanPath, which drops.
func (o *out) clean(kind teamtext.Kind, ref, s string) string
func (o *out) cleanPath(ref, p string) (string, bool)
// quote wraps a teamtext.Text as Quoted, cleaning Body and By.
func (o *out) quote(ref string, t teamtext.Text, owner string) *Quoted
```

- [ ] **Step 1: Write the failing tests first**

`fakes_test.go` builds one project with Alice (Orchestrator `ag-a0`, worker `ag-a1` on `tk-a` whose
title is `text·H` *"Session refresh"* and why-line `text·A` *"brief §2.3 asks for silent renew"*)
and Bob (worker `ag-b1` on `tk-b`), a lease `ls-1` on `auth/**` held by `ag-b1`, and conflict `cf-1`
between `ag-a1` and `ag-b1` on `auth/session.go`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorkerKeysAllowlist` | the JSON keys reachable from `TeamWorker` are exactly a fixed set of 28 distinct names; `title` occurs once, under `brief` (the task 16 technique); none of `why`, `subject`, `text`, `task` occurs | adding `Title` to `WorkerAgent` |
| `TestWorkerTeamHasNoAgentText` | for every fixture, the worker's `team` bytes contain neither card string nor the commit subject | a handler that builds the Orchestrator type for every class |
| `TestWorkerMatchesSnapshotFixture` | `WorkerSnapshot` marshals to `result-team-worker.json` byte for byte, and that file decodes with task 16's `Snapshot` under `DisallowUnknownFields` (the `sbx` test reads the same file) | the two modules' types drifting |
| `TestOrchestratorQuotesAndAttributes` | `tk-a`'s title arrives as `{"text":"Session refresh","class":"text·H","by":"Alice",…}`, the why-line with `"class":"text·A","by":"ag-a0"` | dropping the author class, or labelling all text `text·A` |
| `TestOrchestratorThreadsPartyOnly` | Alice's Orchestrator sees only threads it is party to; a Bob–Carol thread is absent | returning every open thread on the project |
| `TestCheckUnleasedIsEmptyNotError` | `check README.md` is `ok:true` with four empty arrays, matching `result-check-empty.json` | returning `not_found`, or `null` slices |
| `TestCheckGlobIsRefused` | `check auth/*` is `bad_request` | treating a glob as a path and matching every lease |
| `TestCheckDirectivesFirst` | with a pending `dv-1`, the marshalled result's first key after `path` is `directives` | field order, which the CLI's first screen depends on |
| `TestCheckWorkerConflictHasNoTask` | a worker's `check auth/session.go` lists `cf-1` with `otherOwner: "Bob"`, `otherBrief`, paths, and `otherTask` absent | copying the Orchestrator branch |
| `TestHostilePathWithheld` | an observed path `"auth/\u001b[31mx.go"` is absent from `files` in the result, `withheld` is 1 for the Orchestrator, and no error | failing the whole verb on one bad path |
| `TestBidiDisplayNameWithheld` | a display name holding U+202E renders `"(withheld)"` | trusting Hub membership strings |
| `TestOpeningPromptNeverAppears` | an agent launched with args `["-p","PROMPT-MARKER-7"]` through task 20's fake: the marker is absent from `team`, `check`, `files` for both classes | any path that copies launch args into a card |
| `TestFilesUnavailableInAlpha` | with `unavailableTree`, `files` is `unavailable` with `since` set | returning `[]`, which reads as "an empty repo" |
| `TestShowRefusesAnsi` | a file holding `ESC[2J` gives `text_refused`, not its content | passing repo bytes through unchecked |
| `TestShowTruncatesOnRuneBoundary` | a 300 KiB file of `é` returns valid UTF-8, `truncated:true` | cutting mid-rune |

Watch them fail:

```bash
cd control-plane && go test ./internal/fleet/verbs/... 2>&1 | head -3
```

Expected: `undefined: WorkerSnapshot` (build failure).

- [ ] **Step 2: `out.go`, then `team.go` until the team tests pass**

`teamWorker(ctx, c, args)` refuses non-empty `args` (`{}` only, `bad_request`), then returns
`WorkerSnapshot(ctx, deps.Sources, c.Org, c.Project)`. `teamOrchestrator` builds the same value,
then for each agent with a `Card` appends an `OrchCard` whose three texts go through `out.quote`,
then `Threads.Party(ctx, c.Agent)`. The handler registered for `team` switches on `c.Class` with a
`default` that returns `wire.Refuse(wire.CodeBadRequest, …)`, so a `mirror`-class caller, which
task 19's table already refuses, still gets nothing if the table ever regresses.

Sources are read once per call, never cached in the verb: the answer is as fresh as Fleet's memory
(§5.2 item 3). A 50-agent project is 50 map reads; no index is built here.

- [ ] **Step 3: `check.go`**

Order: validate `Path` (`teamtext.ValidPath`, which refuses a glob metacharacter), then directives
(`Directives.Pending(c.Agent)`, a worker's own only; an Orchestrator's list is always empty,
because directives address workers), then holders (every live lease in `Store.Leases` whose glob
`teamtext.Match`es the path; `Mine` when the holder's owner is `c.Owner`), then editors
(`Activity.Of` for each agent, keeping those whose `LastEdit` has the path), then conflicts
(`Conflicts.Open` naming the path). For an Orchestrator caller each `CheckConflict.OtherTask` is the
other side's quoted title.

- [ ] **Step 4: `files.go`**

`files` and `show` are registered for `model.ClassOrchestrator` only; task 19's table already
refuses a worker, and the handler double-checks. Both call `TreeSource` with **the caller's own
owner**, never the other side's: an Orchestrator reads the project at the integration head as its
own human's machine holds it. `show` validates the path as `check` does, and answers `not_found`
for a path not in `Tree.Paths`, so it cannot read an untracked file such as `.env`.

- [ ] **Step 5: Fixtures and `Register`**

Write the four fixtures from the passing tests' output (`go test -run TestWorkerMatchesSnapshotFixture
-update`), review them by eye for any agent text, and add `sandboxd`'s read of
`result-team-worker.json` to task 16's snapshot test as one extra case in its table (anchored on
`snapshot-worker.json`'s case; this is a test-data line in a file task 16 created, and it is the
only edit this task makes there).

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./internal/fleet/... && go test ./internal/fleet/verbs/... -count=1 -v 2>&1 | grep -c "^--- PASS"
cd ../sandboxd && go test ./internal/control/... -count=1
```

Expected: `15` passes in `verbs` (the table above) and `ok` for `sandboxd/internal/control`. Then
the words guard, which must print nothing:

```bash
cd control-plane && grep -n "Title\|Why\|Subject" internal/fleet/verbs/team.go | grep "WorkerAgent\|TeamWorker"
```

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/fleet/verbs testdata/panacea-control/v1/result-*.json
git commit -F <msgfile> -- control-plane/internal/fleet/verbs testdata/panacea-control/v1 \
  sandboxd/internal/control/snapshot_test.go
```

**What this task does NOT claim.**
- That any agent is shown observed files in TM1-α. `Activity` is nil until task 35 (TM1), so
  `files`, `hunks` and `editors` are empty; the fields exist so no consumer changes when they fill.
- That `files` or `show` work in TM1-α. They answer `unavailable` until task 34 gives `machined` a
  tree read (the gap above).
- That the brief section title is true to the brief. It is the Hub's approved record, cleaned and
  capped; whether the Orchestrator chose the right section is task 41's eval.
- That a worker cannot learn a teammate's words another way. This task closes the `panacea` door;
  the lane mirror, the land review and the cockpit are other doors (§7.3), fenced by their tasks.
- That hygiene catches persuasive text. `teamtext.Clean` refuses bytes, not meaning (§7.3, task 02).
