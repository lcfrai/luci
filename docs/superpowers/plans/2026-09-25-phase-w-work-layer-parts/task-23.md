### Task 23: Phase 3 delta: the done evaluator; observed evidence at the landed commit; *self-attested*

Stream **WP**, seam **SW.4** (consumed: the done rule's inputs, its causes and its result words, all
task 10's), about 3 hours (an estimate, not a measurement). Depends on task 22 (the `landed` move
and `LandResult.Issues`) and **P3-15** (evidence) sync-merged. Phase 3's files are **UNVERIFIED
until merge**, cited from its plan at `9935278`; the owner phase wins every conflict (rule H2).

This is the task that makes the demo's last frame true: *"EP-142 landed and proven at 14:02"* with
no click (05 §5.3). **`done` means proven** (the skeleton's rule 2): for every criterion the issue
serves, there is evidence that is **the issue's own**, at or after **its own** land commit, settled,
and not taken back by a revert. When the platform itself ran the criterion's check at the landed
commit and saw exit 0, that evidence is **observed**, and the issue closes with nobody pressing
anything. When the only evidence is a Record by the one person who both filed and started the
issue, the word is ***self-attested***: true, and visibly weaker.

**Two halves, both in phase 3's `evidence` package.**

1. **`issuedone.go`, the evaluator.** `IssueDone` decides, from git and the evidence store, whether
   an issue's served criteria are all proven, which records prove them, and which cause the `done`
   move carries. It **reuses** `Evaluator.States` for the revert walk (P3 `task-15.md:111-163`):
   it narrows the records to the issue's own, then hands them to `States` through a store that
   holds only those, so the walk, the pruning and the cache rules are phase 3's, never copied.
2. **`observe.go`, the observer.** `Observer.Run` bundles the landed commit out of the project,
   hands the bundle to a **`CheckRunner`** that clones it fresh inside a sandbox, checks out the
   commit detached, runs the check command, and returns **only its exit code**. The output is
   discarded unread (`OD-8`: nothing derives meaning from a process's bytes). Exit 0 appends one
   Record as the lander, `verdict: settled`, observed, citing the issue. Any other exit writes
   nothing: a failed check is not evidence of anything, and a "failed" record would read as a
   verdict nobody gave.

A **`Closer`** joins them: after a land (task 22's `Landed`) and after a person's Record that names
an issue, it runs the observer for each served criterion that has a check, then `IssueDone`, and
when the answer is proven it sends `StateMove{to: done, cause, evidence}` through `Deps.Work`, as
that person (the control plane's one Hub client, as task 22 explains).

**Four corrections to the scope, each made against the seam's owner (task 10) or the tree.**

1. **Causes.** The scope wrote `cause: evidence`. Task 10's complete W1 set names two:
   `evidence_observed` (every cited record observed) and `evidence_recorded` (otherwise). This part
   sends task 10's, assumed to be `hubapi.CauseEvidenceObserved` and `hubapi.CauseEvidenceRecorded`
   in Go.
2. **Words.** The scope's closed set was `proven` · `proven_self_attested` · `not_proven` ·
   `no_criteria`. Task 10's `IssueHead.proof` has `not_proven` for a **hand-set** `done` and
   `waiting` for *landed, some criterion without evidence*. The evaluator never hand-sets anything,
   so its set is `proven` · `proven_self_attested` · `waiting` · `no_criteria`, the same strings
   as the Hub's, so a test can compare the two.
3. **`no_criteria` is the Hub's move, not this task's.** Task 10 moves an empty-`serves` issue to
   `done` **in the same write** as its `landed` move, unless the team switch
   `stopAtLandedWithoutServes` holds it. So the Closer sends nothing for `no_criteria`; its test
   proves the Hub (the fake) did it, and did not with the switch on.
4. **Where the citation lives.** The scope assumed task 10 adds `issueId` and `observed` to the
   Hub's evidence record. Task 10's scope does not: the Hub's `EvidenceRecord` has neither
   (`types.go:520-532` on `feat/hub`, the scope's `:517-529` having moved; hub-contract
   `:1060-1083`; both verified at `9f2fa31`), and the
   citation reaches the Hub only as the `done` move's `evidence` list, kept in `issue_proofs`.
   This part therefore adds `IssueID` and `Observed` to **phase 3's local** `evidence.Record`,
   additively, where the evaluator reads them. That is enough for a repository whose store is
   phase 3's `FileStore` (`Shared() == false`). For a Hub-backed store the two fields do not survive
   a round trip, so a citation made in one event (an observed check at land) is not found again by
   a later event (a person's Record an hour after). **This is a gap, raised, not solved here**: the
   Hub's evidence body needs `issueId` and `observed` (additive, allowed after WS0), in task 10 or
   a new WH task. The part's return names it; nothing here invents a second store.

**A fifth input, and it is in the wire: the check.** The observer needs a command per criterion
("`npm test -- resume.spec.ts`"). SW.1's `Serve` carries it — `check`, `omitempty`, at most 1000
bytes, with golden vector v4 (`task-02.md:255`, `:557`) — added by **task 02** rather than here,
because it is part of the `issue-revision` canonical form and SW.1 freezes at WS0. This part reads
it off the head revision the Closer evaluates (no `Checks` field, so no wiring can leave it nil)
and does not decide who may author one: that rule is SW.3's, task 07, and task 02 names it.

**And one the tree does not have: a runner that runs a command and reports its exit.**
`backend.Backend` (`backend.go:337-345`, built) creates and destroys sandboxes but reports no exit
code; P3-24's `sandboxd` git runner has five git operations and no "run" (P3 `task-24.md:138-196`).
So `CheckRunner` is an interface here, with a test double that clones on the host into
`t.TempDir()`; **the product implementation is task 23b's**, which depends on this task and runs
the check through `sandboxd` in a check sandbox of its own. This task lands with `Runner` nil, so
`Run` returns `ErrNoRunner` and nothing is observed until 23b turns it on.

**Trust, stated plainly.** In local mode the observer runs in the person's own control plane, and
the Hub records the resulting `evidence_observed` move as that person's. The Hub cannot tell a
watched run from a claimed one; the claim is the control plane's, on the person's token, exactly
as a Record is. What makes *observed* stronger is that no human typed it and the command, the
commit and the time are in the record. In the cloud (phase 6, phase 8) the observer moves to the
platform, and that is when *observed* means *observed by someone other than you*.

**Files** (every phase 3 path is **UNVERIFIED until merge**):
- `control-plane/internal/evidence/record.go` ⚑ (P3 `task-15.md:26`, `Record` at `:48-60`) —
  `IssueID`, `Observed`; `Validate` checks `IssueID` (modified)
- `control-plane/internal/evidence/issuedone.go` + `issuedone_test.go` — `IssueDoneInput`,
  `Served`, `IssueDone`, the words, `(*Evaluator).IssueDone`, `Closer` (new)
- `control-plane/internal/evidence/observe.go` + `observe_test.go` — `Check`, `Observation`,
  `CheckRun`, `CheckRunner`, `Observer`, `ErrNoRunner` (new)
- `control-plane/internal/api/evidence.go` ⚑ (P3 `task-15.md:30`) — the POST body gains `issueId`;
  after a Record naming an issue, `Closer.AfterRecord` (modified)
- `control-plane/internal/api/lands.go` ⚑ (P3-09; task 22's call) — `Closer.AfterLand` after
  `IssueFacts.Landed` (modified)
- `control-plane/internal/api/gitdeps.go` ⚑ — `Closer *evidence.Closer` in task 22's Phase W block
  (modified)
- `control-plane/cmd/api/main.go` ⚑ — build the `Closer` beside task 22's `IssueFacts` (modified)
- `control-plane/internal/api/issuedone_test.go` — the handler rows (new)

**Interfaces:**
- Produces: `evidence.Record.IssueID`, `.Observed`; `evidence.Served`, `IssueDoneInput`,
  `IssueDone`, the four `Word*` constants, `(*Evaluator).IssueDone`; `evidence.Check`,
  `Observation`, `CheckRun`, `CheckRunner`, `Observer`, `ErrNoRunner`; `evidence.Closer`,
  `DoneMover`, `IssueReader`. Consumed by task 48 (the issue view reads `IssueHead.proof`, not these),
  task 71 (the manifest names proven issues) and task 104 (W3's proposed evidence feeds `Closer`)
- Consumes:
  - P3-15 — `evidence.Record`, `Verdict` (`settled`), `Store` (`Record`, `ByDigest`, `Shared`),
    `Evaluator{Machine, Store}`, `(*Evaluator).States` (P3 `task-15.md:159`), `State`
  - P3-03 — `briefref.Criterion{ID, Digest}`, `briefref.Resolver` (P3 `task-03.md:459-477`)
  - P3-02 — `gitx.Call`, `gitx.Read`, `gitx.Local`, `gitx.Project`, `gitx.Ident`
  - task 03/10 — `hubapi.StateMove`, `hubapi.StateDone`, the two evidence causes,
    `hubapi.IssueHead.{State, LandCommit, CreatedBy}`, `hubapi.TeamPolicy`
  - task 16 — `hubclient.Work`'s `Issue` and `MoveIssueState` (task 16 part `:258`, `:309`)
  - task 21 — `gitx.ValidIssueTrailer`; task 22 — `LandResult.Issues`, `IssueFacts.Landed`
- Amends: phase 3's `Record` gains two optional fields (schemaVersion 1 unchanged); P3-15's POST
  body gains one optional field.

#### Interfaces — complete

```go
// evidence/record.go — two fields appended to Record. omitempty keeps an evidence.json written
// before this task byte-identical on the next write.
//	IssueID  string `json:"issueId,omitempty"`  // the bare Hub issue this record cites; "" cites none
//	Observed bool   `json:"observed,omitempty"` // true only for a record Observer.Run wrote

// Validate gains one rule: IssueID is "" or matches ^is-[0-9a-f]{16}$ (gitx.ValidIssueTrailer),
// else ErrInvalid naming "issueId". Observed is not validated: no route can set it (below).
```

```go
// evidence/issuedone.go
package evidence

// The words, the same strings as task 10's IssueHead.proof, so a test can compare them.
const (
	WordProven             = "proven"               // every served criterion proven, not only by the doer
	WordProvenSelfAttested = "proven_self_attested" // some criterion proven only by the doer's own Record
	WordWaiting            = "waiting"              // landed; some criterion has no qualifying evidence
	WordNoCriteria         = "no_criteria"          // serves is empty; the Hub decides at landed
)

// Served is one criterion an issue's head revision serves, resolved: the brief key verbatim, and
// the criterion with the digest phase 4's resolver gave for it.
type Served struct {
	BriefKey  string
	Criterion briefref.Criterion
}

// IssueDoneInput is everything IssueDone needs; the Closer builds it from the Hub head.
type IssueDoneInput struct {
	IssueID    string
	LandCommit string   // IssueHead.LandCommit; "" means not landed, and the answer is waiting
	Tip        string   // the branch tip now: the subject States evaluates at (40 hex)
	FiledBy    string   // the issue's creator's email, lower case
	StartedBy  string   // who made the started move, email lower case; "" when unknown
	Serves     []Served // in the revision's order
}

// IssueDone is the answer. Evidence is every deciding record's id, in Serves order, deduplicated:
// what the done move cites. Open is the criterion ids still without qualifying evidence.
type IssueDone struct {
	Word     string
	Open     []string // [] when none
	Evidence []string // [] when none
	Observed bool     // every record in Evidence is Observed: the move's cause is evidence_observed
}

// IssueDone applies the done rule (05 §3.3; task 10's rule, checked here against git):
//  1. Serves empty → no_criteria, and nothing else is read.
//  2. LandCommit "" → waiting, every criterion open.
//  3. records = Store.ByDigest(repo, every served digest); keep r when all hold:
//       r.IssueID == in.IssueID                       (the issue's own: a sibling's never counts)
//       r.Verdict == settled
//       merge-base --is-ancestor LandCommit r.Commit  (exit 0: at or after its own land; a
//                                                      record on the lane before the land fails)
//  4. states = (&Evaluator{Machine: e.Machine, Store: onlyThese(kept)}).States(ctx, repo,
//     projectPath, in.Tip, criteria): phase 3's revert walk and pruning, reused, never copied.
//  5. a criterion is proven when its state is settled; its deciding records are State.Evidence.
//     It is proven by others when some deciding record is Observed, or its recorder's email is
//     not both FiledBy and StartedBy.
//  6. every criterion proven → proven_self_attested when some criterion is proven only by the
//     doer, else proven; otherwise waiting.
// A git or store error is returned; the Closer then sends nothing.
func (e *Evaluator) IssueDone(ctx context.Context, repo, projectPath string, in IssueDoneInput) (IssueDone, error) {
	out := IssueDone{Open: []string{}, Evidence: []string{}}
	if len(in.Serves) == 0 {
		out.Word = WordNoCriteria
		return out, nil
	}
	criteria := make([]briefref.Criterion, len(in.Serves))
	digests := make([]string, len(in.Serves))
	for i, s := range in.Serves {
		criteria[i], digests[i] = s.Criterion, s.Criterion.Digest
	}
	if in.LandCommit == "" || !gitx.ValidIssueTrailer(in.IssueID) {
		out.Word = WordWaiting
		for _, c := range criteria {
			out.Open = append(out.Open, c.ID)
		}
		return out, nil
	}
	all, err := e.Store.ByDigest(ctx, repo, slices.Compact(slices.Sorted(slices.Values(digests))))
	if err != nil {
		return IssueDone{}, err
	}
	byID := map[string]Record{}
	var kept []Record
	for _, r := range all {
		if r.IssueID != in.IssueID || r.Verdict != Settled {
			continue
		}
		after, err := e.isAncestor(ctx, projectPath, in.LandCommit, r.Commit)
		if err != nil {
			return IssueDone{}, err
		}
		if after {
			kept = append(kept, r)
			byID[r.ID] = r
		}
	}
	states, err := (&Evaluator{Machine: e.Machine, Store: onlyThese(kept)}).States(ctx, repo, projectPath, in.Tip, criteria)
	if err != nil {
		return IssueDone{}, err
	}
	out.Observed = true
	selfOnly := false
	for _, st := range states {
		if st.State != "settled" {
			out.Open = append(out.Open, st.CriterionID)
			continue
		}
		byOthers := false
		for _, id := range st.Evidence {
			r := byID[id]
			if !slices.Contains(out.Evidence, id) {
				out.Evidence = append(out.Evidence, id)
			}
			out.Observed = out.Observed && r.Observed
			byOthers = byOthers || r.Observed || !isDoer(r.RecordedBy, in.FiledBy, in.StartedBy)
		}
		selfOnly = selfOnly || !byOthers
	}
	switch {
	case len(out.Open) > 0:
		out.Word, out.Observed = WordWaiting, false
	case selfOnly:
		out.Word = WordProvenSelfAttested
	default:
		out.Word = WordProven
	}
	return out, nil
}

// isAncestor is merge-base --is-ancestor a b in the project: exit 0 true, exit 1 false, exit 128
// (the repository lacks b: a teammate's unfetched commit) false, as States treats it. Both are
// 40-hex shas before any call; anything else is false with no git call.
func (e *Evaluator) isAncestor(ctx context.Context, projectPath, a, b string) (bool, error) {
	if !sha40.MatchString(a) || !sha40.MatchString(b) {
		return false, nil
	}
	res, err := e.Machine.Git().Run(ctx, gitx.Call{Dir: projectPath,
		Args:  []string{"merge-base", "--is-ancestor", a, b},
		Class: gitx.Read, Scope: gitx.Project, OKExit: []int{1, 128}})
	if err != nil {
		return false, err
	}
	return res.ExitCode == 0, nil
}

var sha40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// onlyThese is a read-only Store over a fixed slice: ByDigest filters it by digest, oldest
// first, as FileStore does; Record is refused (it is never called by States); Shared is false.
type onlyThese []Record

func (s onlyThese) Record(context.Context, Record) (Record, error) {
	return Record{}, errors.New("evidence: onlyThese is read-only")
}
func (s onlyThese) Shared() bool { return false }
func (s onlyThese) ByDigest(_ context.Context, _ string, digests []string) ([]Record, error) {
	out := []Record{}
	for _, r := range s {
		if slices.Contains(digests, r.CriterionDigest) {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b Record) int { return a.RecordedAt.Compare(b.RecordedAt) })
	return out, nil
}

// isDoer reports whether recordedBy ("Name <email>", P3-15's git identity) is the one person who
// both filed and started the issue. An unknown StartedBy ("") is never the doer: the local word
// then reads proven, and task 10's proof on the Hub head is the word every surface shows.
func isDoer(recordedBy, filedBy, startedBy string) bool {
	e := emailOf(recordedBy)
	return e != "" && startedBy != "" && e == filedBy && e == startedBy
}

// emailOf is the text between the first "<" and the next ">", lower case; "" when absent.
func emailOf(ident string) string {
	i := strings.IndexByte(ident, '<')
	if i < 0 {
		return ""
	}
	j := strings.IndexByte(ident[i+1:], '>')
	if j < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(ident[i+1 : i+1+j]))
}
```

**Why a sub-evaluator and not a filter inside `States`.** `States` reads `Store.ByDigest` itself
and has no filter parameter; adding one would edit phase 3's signature, which three phases call
(P4, P7, the Merge lens). The sub-evaluator costs its cache: a fresh `Evaluator` has empty LRUs, so
each `IssueDone` pays the revert walk once. It runs only after a land or a Record, never on a poll,
so that is a handful of `git log` calls per event (an estimate, not a measurement).

```go
// evidence/observe.go
package evidence

// Check is one criterion's check: a serves pair's Serve.Check (SW.1, task 02) on the head revision,
// run at the landed commit. Who may author one is SW.3's (task 07), never this task's.
type Check struct {
	CriterionID string
	Command     string        // run as `sh -c <Command>` in the fresh clone's root; 1-1000 bytes
	Timeout     time.Duration // zero means 10 minutes; at most 30 minutes
}

// Observation is what a run leaves: never its output.
type Observation struct {
	Command  string
	ExitCode int
	Commit   string
	RanAt    time.Time
}

// CheckRun is one run a CheckRunner performs. Bundle is a host path to a git bundle holding
// Ref, which names Commit; the runner copies it into a sandbox, clones it fresh there, checks
// out Commit detached, runs `sh -c Command` with stdout and stderr discarded, and reports the
// exit code. It never mounts a lane, a project or any host directory but the bundle, read-only.
type CheckRun struct {
	Bundle  string
	Ref     string
	Commit  string
	Command string
	Timeout time.Duration
}

// CheckRunner runs one check in a sandbox. A timeout is exit code 124 (coreutils timeout's
// convention), not an error; an error is a runner that could not run at all.
type CheckRunner interface {
	RunCheck(ctx context.Context, run CheckRun) (exitCode int, err error)
}

// ErrNoRunner is Run with a nil Runner: nothing is observed, and nothing is recorded.
var ErrNoRunner = errors.New("evidence: no check runner is configured")

// Observer watches checks run and records exit 0 as observed evidence.
type Observer struct {
	Machine gitx.Machine
	Runner  CheckRunner // nil until task 23b's SandboxRunner is wired in cmd/api
	Store   Store
	Scratch string // an OwnedRoot directory for bundles; each is removed after its run
	// Identity is the lander's git identity for the project, as Lander.Identity (P3-09).
	Identity func(ctx context.Context, path string) (gitx.Ident, error)
	Now      func() time.Time // nil means time.Now
	Nonce    func() string    // nil means 8 lowercase hex from crypto/rand
}

// Run, in order:
//  1. Runner nil → ErrNoRunner. Command empty or over 1000 bytes, commit not 40 hex, or the
//     issue id not valid → ErrInvalid, before any git call.
//  2. in the PROJECT (Class Local, Scope Project): update-ref refs/panacea/observe/<nonce> commit;
//     bundle create <Scratch>/observe-<nonce>.bundle refs/panacea/observe/<nonce>; then
//     update-ref -d refs/panacea/observe/<nonce>, deferred so it runs on every path. The
//     temporary ref exists because a bundle's tips must be refs; a bare sha bundles nothing.
//  3. exit, err = Runner.RunCheck(ctx, CheckRun{…}); the bundle file is removed, deferred.
//  4. exit 0 → Store.Record(Record{Repo: repo, ProjectID: projectID, BriefKey: s.BriefKey,
//     CriterionID, CriterionDigest, Commit: commit, Verdict: Settled, Because: <below>,
//     RecordedBy: id.Name + " <" + id.Email + ">" for id = Identity(projectPath),
//     IssueID: issueID, Observed: true}).
//     Any other exit → no Record, no error: Observation.ExitCode says what happened.
// Because is "<Command>, exit 0, run by the platform at <RanAt, RFC 3339 UTC>", with Command cut
// on a rune boundary so the whole sentence fits P3-15's 2000-character limit.
func (o *Observer) Run(ctx context.Context, repo, projectID, projectPath, commit, issueID string, s Served, c Check) (Observation, *Record, error)
```

The signature is wider than the scope's `Run(ctx, repo, projectPath, commit, c)`: a Record needs
the project id, the brief key, the criterion digest and the issue it cites, and the caller has all
four. It returns the Record it wrote (`nil` on a non-zero exit) so the Closer can cite its id
without reading the store back, and passes it to `IssueDone` as `Fresh` (below). That is what keeps
the observed-at-land path working on a Hub-backed store despite gap 4: the store's copy of the
record lacks `IssueID` and `Observed`, and the fresh copy replaces it by id.

```go
// issuedone.go — one field appended to IssueDoneInput, and one step in IssueDone between
// reading `all` and filtering it.
//	Fresh []Record // records this event wrote; each replaces the store's copy with the same ID

//	for _, f := range in.Fresh {
//		if i := slices.IndexFunc(all, func(r Record) bool { return r.ID == f.ID }); i >= 0 {
//			all[i] = f
//		} else {
//			all = append(all, f)
//		}
//	}
```

`Identity`'s `gitx.Ident` fields are assumed to be `Name` and `Email` (P3-03's `ParseIdent` result,
P3 `task-03.md:40-44`); follow the merged names.

```go
// issuedone.go, continued — the Closer: the one place a done move is sent from.

// IssueReader and DoneMover are the parts of hubclient.Work the Closer needs; hubclient.Work
// satisfies both, so evidence imports only hubapi (hub-contract.md:78-82).
type IssueReader interface {
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
}
type DoneMover interface {
	MoveIssueState(ctx context.Context, issueID string, m hubapi.StateMove) (hubapi.IssueHead, error)
}

// Closer evaluates and closes. Hub is the control plane's one Hub client (Deps.Work): every
// move is the signed-in person's. Resolver turns a serves pair's brief key into the criterion's
// digest (phase 4's resolver, P3's briefref seam). Its checks are the evaluated revision's own
// Serve.Check values (AfterLand step 3), not a field. A Closer never fails the act that called it.
type Closer struct {
	Hub interface {
		IssueReader
		DoneMover
	}
	Evaluator *Evaluator
	Observer  *Observer
	Resolver  briefref.Resolver
	// no Checks field: a nil one would switch observation off with every test still green
	Log       *slog.Logger // metadata only: issue id, word, cause, code
}

// Scope names the repository an event happened in.
type Scope struct {
	Repo, ProjectID, ProjectPath, Branch, Tip string
}

// AfterLand runs for each issue a land named (LandResult.Issues), after task 22's Landed:
//  1. head = Hub.Issue(id); skip unless head.State is landed and head.LandCommit is the land's
//     merge (a later land of the same issue owns it then);
//  2. served = resolve every serves pair of the head revision (a pair the resolver cannot
//     answer is open: the issue waits, it is never proven by a guess);
//  3. per served pair with a non-empty Serve.Check: Observer.Run at head.LandCommit; collect Records;
//  4. done = Evaluator.IssueDone(…, IssueDoneInput{…, Fresh: those Records});
//  5. send(done).
func (c *Closer) AfterLand(ctx context.Context, sc Scope, issueIDs []string)

// AfterRecord runs after a person's Record that names an issue (the POST's issueId): steps 1, 2,
// 4 and 5 of AfterLand, with Fresh holding the person's Record, and no check run.
func (c *Closer) AfterRecord(ctx context.Context, sc Scope, issueID string, rec Record)

// send moves the issue to done only for proven and proven_self_attested:
//	StateMove{To: hubapi.StateDone, Cause: evidence_observed when done.Observed else
//	          evidence_recorded, LandCommit: head.LandCommit, Evidence: done.Evidence}
// waiting and no_criteria send nothing (no_criteria is the Hub's, in the landed write). A
// refusal is logged with its code and dropped: the evidence stays, and the next Record or land
// runs the rule again. Nothing is queued.
func (c *Closer) send(ctx context.Context, issueID string, head hubapi.IssueHead, done IssueDone)
```

`FiledBy` is `head.CreatedBy.Email`, lower-cased (`hubapi.User.Email`, `types.go:98` on `feat/hub`,
verified). `StartedBy` is `""` until SW.4 exposes who made the `started` move (a gap, raised: task
10 computes *self-attested* from it on the Hub, and the head does not show it). So the **local**
word never reads `proven_self_attested` in the product today; the Hub's `proof` does, and it is
what every surface shows. The evaluator's own tests pass `StartedBy` explicitly, so the rule is
built and pinned now.

```go
// api/evidence.go — P3-15's POST body gains one optional field. The decoder stays strict, so
// "observed" in a body is 400 unknown field: no route can mark a record observed.
//	IssueID string `json:"issueId,omitempty"` // copied to Record.IssueID; Validate checks it

// After the Record is stored and before the 201 is written:
	if a.deps.Git != nil && a.deps.Git.Closer != nil && rec.IssueID != "" {
		a.deps.Git.Closer.AfterRecord(r.Context(), scopeOf(project, branch), rec.IssueID, rec)
	}

// api/lands.go — after task 22's IssueFacts.Landed:
	if g.Closer != nil {
		g.Closer.AfterLand(r.Context(), scopeOf(project, req.Branch, res.Merge), res.Issues)
	}
```

`AfterLand` runs checks, and a check may take minutes. It therefore runs on a goroutine with
`context.WithoutCancel(r.Context())` and a 35-minute bound per check run, and the land's 201 never waits
for it. `AfterRecord` runs no check and is synchronous within task 22's 3-second Hub bound.
`scopeOf` fills `Scope` from the handler's resolved project and branch; its spelling follows P3's
merged handlers (**UNVERIFIED until merge**).

- [ ] **Step 1: Write the failing evaluator tests**

Create `control-plane/internal/evidence/issuedone_test.go` on P3-15's rig (a `gitxtest.Repo`
project; P3-10's revert helper; a `FileStore` in `t.TempDir()`). `rigLand(t)` makes a lane commit
`T`, lands it as merge `M` with `Panacea-Issue: <a>`, and returns both; `rec(...)` appends a Record.

```go
func TestIssueDoneIgnoresEvidenceBeforeLand(t *testing.T) {
	rg := newDoneRig(t)                  // issue a serves c1; filed and started by dev@example.com
	T, M := rg.land(t, rg.a)
	rg.rec(t, evidence.Record{Commit: T, IssueID: rg.a, CriterionID: "c1", Verdict: evidence.Settled,
		RecordedBy: "Reviewer <rev@example.com>"}) // on the lane, before the land existed
	got, err := rg.ev.IssueDone(ctx, rg.repo, rg.path, rg.input(rg.a, M, M))
	if err != nil {
		t.Fatal(err)
	}
	if got.Word != evidence.WordWaiting || !slices.Equal(got.Open, []string{"c1"}) {
		t.Fatalf("IssueDone = %+v, want waiting with c1 open", got)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestIssueDoneIgnoresEvidenceBeforeLand` | a settled record at the lane commit `T` (an ancestor of `M`) → `waiting`, `c1` open | a missing at-or-after check (`M` ancestor of `r.Commit`) |
| `TestIssueDoneIgnoresSiblingEvidence` | issues `a` and `b` both serve `c1`; `a` lands in `M1`, then `b` in `M2` (a descendant, so at-or-after `M1` holds); a settled record at `M2` citing `b` → `a` is `waiting` | a rule without the citation, matching by criterion and ancestry alone |
| `TestIssueDoneNeedsEveryCriterion` | `a` serves `c1`, `c2`; evidence for `c1` only → `waiting`, `Open == ["c2"]`, `Evidence == []` | *any* criterion instead of *every* |
| `TestSelfAttestedNeedsAllThree` | a Record by the doer, `FiledBy == StartedBy == doer` → `proven_self_attested`; the same with `FiledBy` another person → `proven`; with `StartedBy` another → `proven`; with `StartedBy == ""` → `proven` | a flag built from two of the three people |
| `TestObservedIsNeverSelfAttested` | the doer's lander identity, an `Observed` record → `proven`, `Observed == true` | observed evidence marked self-attested |
| `TestMixedEvidenceIsRecordedNotObserved` | `c1` observed, `c2` a reviewer's Record → `proven`, `Observed == false` (cause `evidence_recorded`) | an `evidence_observed` cause over a human record |
| `TestRevertedLandRetiresIssueEvidence` | an observed record at `M`; P3-10 reverts `M` as `R` → at `Tip == R` the answer is `waiting` | bypassing `States`' revert walk |
| `TestIssueDoneEmptyServesIsNoCriteria` | `Serves == nil` → `no_criteria`, and the recording Runner shows no git call | a store or git read for nothing |
| `TestFreshReplacesTheStoresCopy` | a store whose `ByDigest` strips `IssueID` and `Observed` (the Hub-backed shape); the same record in `Fresh` with both → `proven` | a Hub-backed land path that can never close |

Run: `go test ./control-plane/internal/evidence/... -race -count=1 -run 'IssueDone|SelfAttested|Observed|Mixed|Reverted|Fresh'`
Expected: FAIL, `undefined: evidence.IssueDoneInput`.

- [ ] **Step 2: Implement `record.go`'s fields and `issuedone.go`'s evaluator**

Add the two `Record` fields and the `Validate` rule; write `IssueDone`, `isAncestor`,
`onlyThese`, `isDoer`, `emailOf` and the `Fresh` merge. Re-run step 1's command. Expected: `ok`.
P3-15's own tests must still pass unchanged: run `go test ./control-plane/internal/evidence/...
-race -count=1`; expected `ok`.

- [ ] **Step 3: Write the failing observer tests**

In `observe_test.go`, `hostRunner` is the test double: it clones `run.Bundle` into a fresh
`t.TempDir()`, `git checkout --detach run.Commit`, runs `sh -c run.Command` there with output to
`io.Discard`, and returns the exit code. It exists only in the test file.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestObservedExitZeroClosesWithNoClick` | through `Closer.AfterLand` on `hubclient.NewFake()`: issue at `landed` serving `c1` with check `test -f resume.ok`, the file committed in `M` → one Record (`Observed`, `IssueID`, `Commit == M`, recorder the lander, `Because` starting `test -f resume.ok, exit 0, run by the platform at`), and the fake's head is `done`, `proof: proven`, cause `evidence_observed`; no Record call came from a route | a rule that needs a person's Record |
| `TestObservedNonZeroWritesNothing` | check `false` → `Observation.ExitCode == 1`, `*Record == nil`, the store is empty, the head stays `landed` | a failure recorded as evidence |
| `TestObserverRunsInAFreshClone` | the lane clone holds an **uncommitted** `resume.ok`; `M` does not → the check exits 1 | running in the lane, or on the project's worktree |
| `TestObserverKeepsNoOutput` | a check `echo SECRET-TOKEN; exit 0` → the Record's `Because` and the logger's buffer hold no `SECRET-TOKEN` | output read or kept (`OD-8`) |
| `TestObserverCleansUp` | after a run, `refs/panacea/observe/*` is empty in the project and `Scratch` holds no bundle, on exit 0, exit 1 and a runner error | a temporary ref or bundle left behind |
| `TestObserverWithoutRunner` | `Runner == nil` → `ErrNoRunner`, no git call, no Record | a silent pass that looks observed |

Run: `go test ./control-plane/internal/evidence/... -race -count=1 -run 'Observ'`
Expected: FAIL, `undefined: evidence.Observer`. Write `observe.go`; re-run. Expected: `ok`.

- [ ] **Step 4: The Closer and the two routes, tests first**

In `issuedone_test.go` (the Closer on `hubclient.NewFake().As(lander)`) and
`control-plane/internal/api/issuedone_test.go` (the routes on P3's API rig):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEmptyServesDoneAtLandedUnlessHeld` | an issue with `serves: []`; task 22's `Landed` then `AfterLand` → the head is `done`, `proof: no_criteria`, and `Calls()` holds no `MoveIssueState` with `to: done` from the Closer; with the team policy's `stopAtLandedWithoutServes` on → the head stays `landed` | an ignored team switch; a Closer that sends its own done |
| `TestRecordNamingTheIssueCloses` | `POST …/evidence` with `issueId` for the last open criterion → 201 and the head is `done`, cause `evidence_recorded`, actor the person who posted | a person's Record that never closes |
| `TestRecordRouteRefusesObserved` | the same body plus `"observed": true` → 400 naming the unknown field; the store is unchanged | a client marking its own record observed |
| `TestRecordWithoutIssueClosesNothing` | a POST with no `issueId` → 201 and no `Issue` read in `Calls()` | a Record proving every issue that serves its criterion |
| `TestLandResponseDoesNotWaitForChecks` | a `hostRunner` that sleeps 2 s → the land's 201 arrives in under 1 s (the check finishes after) | a land gated on its checks |
| `TestCloserRefusalIsDropped` | the fake answers the done move 422 → logged with the code, no retry list, the Record stays in the store | a done retried forever, or a lost Record |

Run: `go test ./control-plane/internal/evidence/... ./control-plane/internal/api/... -race -count=1 -run 'EmptyServes|Record|LandResponse|Closer'`
Expected: FAIL, `undefined: evidence.Closer`. Write the Closer, the `GitDeps` field, the two route
calls and the `cmd/api` wiring (`Closer{Hub: deps.Work, Evaluator: git.Evaluator, Observer:
&evidence.Observer{…, Runner: nil}, Resolver: git.Resolver, Log: log}`: no checks to wire, as
each is its pair's `Serve.Check`, and `Runner` stays nil until 23b). Re-run. Expected: `ok`.

- [ ] **Step 5: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/evidence/... ./control-plane/internal/converge/... ./control-plane/internal/api/... -race -count=1'
```

Expected: one `ok` per package, no `FAIL`. `git diff -- go.work control-plane/go.mod` is empty.
Commit with `git commit -F <msgfile> -- <the paths above>`.

#### What this task does not claim

- **Not that anything is observed in the product yet.** The runner here is the test double; the
  product one is **task 23b**, which depends on this task. Until 23b lands, the demo's no-click
  `done` (05 §5.3) is shown only by `TestObservedExitZeroClosesWithNoClick`, on the host double,
  and the product closes an issue only on a Record — which is what 23b exists to end.
- **Not the citation on a shared store across events** (gap 4). On a Hub-backed store, a citation
  made at land is used at land; a later Record cannot find it again until the Hub's evidence body
  carries `issueId` and `observed`.
- **Not *self-attested* in the local word** while SW.4 does not say who started the issue; the
  Hub's `proof` carries it.
- **Not the *done, not proven* glyph** (tasks 28, 48), a hand-set `done` (task 10's server rule),
  W3's proposed evidence (task 104), or reopening a `done` issue whose land is later reverted: the
  evaluator reads `waiting` after a revert, and no move is sent back.
- **Not the product runner, nor where a check is typed**: task 23b owns both — the runner (through
  `sandboxd`, in a check sandbox, `backend.Backend` unchanged) and the check line on a serves row.
- Design: 05 §3.3, §2.1 point 4, §5.3 row 7. Parity: closes the W1 part of **L-8** (`done` is a
  fact) with task 22; with task 23b it is what makes 05 §5.3's no-click `done` true.
