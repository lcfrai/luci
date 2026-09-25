# Phase W task scopes: tasks 21 to 40

Scope paragraphs for tasks 21-40 of `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`
(W1: the phase 3 deltas, Start from an issue, import, the look, and the Work lens). A part writer
reads one paragraph plus the skeleton's row; where they disagree, the row's id, dependsOn and seams
win (skeleton, *Task scopes*). `§` means a section of
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2); `L-NN` is row NN of
`01-linear-feature-inventory.md`. Paths are relative to the repository root. A sibling path marked
**UNVERIFIED until merge** is named only by its owner's plan (rule H2).

Phase 3 anchors were read in the `phase-3` worktree at `7c71b7d`: the plan
`2026-09-23-phase-3-git-convergence.md` (cited `P3-plan:N`) and its parts (cited `P3 task-NN.md:N`).
None of phase 3's packages (`gitx`, `lanes`, `converge`, `evidence`) exists in code yet; a part
re-reads them at its sync base. Go tests run as `go test ./control-plane/... -race -count=1`;
desktop tests as `npm --prefix desktop test` and `npm --prefix desktop run typecheck`.

---

**21 — phase 3 delta: the `Panacea-Issue` trailer; the lane index names its issue; AU-only branch
names.** Stream WP, seam **SW.9**, 3 h. Depends on 19 (`Session.IssueRef`, SW.7) and on P3-05, P3-07
and P3-08 sync-merged; if their files are not on the sync base the task waits and never writes a
second copy (rule H2). It touches only phase 3 files, each **UNVERIFIED until merge**:
`control-plane/internal/gitx/trailers.go` (P3 task-02.md:309-316 lists `TrailerSession` …
`TrailerBringIn`) gains `TrailerIssue = "Panacea-Issue"` and `func ValidIssueTrailer(v string) bool`
(exactly `^is-[0-9a-f]{16}$`: the bare id, never a key, never parsed as one, contract P5);
`images/sandbox/githooks/` (P3 task-05.md:147-187) stamps `Panacea-Issue: $PANACEA_APP_ISSUE_ID` once,
beside `Panacea-Session`, only when the variable is set and valid; task 19's `internal/api/sessions.go`
delta sets `PANACEA_APP_ISSUE_ID` from `Session.IssueRef` through `hubapi.ParseKey` (task 02, the one
parser); `control-plane/internal/lanes/store.go` (`Lane` at P3 task-03.md:61-69) gains
`IssueID string \`json:"issueId,omitempty"\`` and `CreateRequest.IssueID`, so a worktree made from an
issue names it in the lane index (04 §7.3; the skeleton files this under `converge`, but the index
is `lanes`); and P3-09's land in `internal/converge` adds one `Panacea-Issue` per distinct valid value
in the landed range (`git log --format=<gitx.TrailerFormat(TrailerIssue)> M^1..<lane tip>`, plus the
lane's own `IssueID`), sorted and deduplicated, invalid values dropped before git sees them. A new
`control-plane/internal/lanes/issuebranch.go` holds `func IssueBranchName(alias, title string, auOnly
bool) string`: `ep-142-<slug>` (lower-case `[a-z0-9-]`, at most 40 characters; an empty slug gives
`ep-142`), and only `ep-142` for an *AU data only* project (`OD-24`, Q8), since an erase cannot reach
a branch name on a customer's remote. **SW.9:** key `Panacea-Issue`, value the bare `is-` id, on every
commit of a bound session and once per issue on the land commit; the revision the session was told
is recovered through `Panacea-Session` → `Session.IssueRef`, not from the trailer (04 §5.2 wrote
`issue:is-…@r`; this plan follows the skeleton's SW.9, and the return lists the difference). **Tests
first:** `TestValidIssueTrailerRejectsKeysAndFlags` (`issue:is-…@3`, `-h`, upper case, a value
holding a newline → false; kills a validator that accepts any `is-` prefix);
`TestDispatchStampsTheIssueOnce` (docker, P3-05's rig; kills a hook that stamps again on amend);
`TestDispatchOmitsTheIssueWhenUnset` (kills an empty `Panacea-Issue:` line); `TestLaneRecordsItsIssue`
(kills a field the store's schema drops); `TestLandAddsOnePerDistinctIssue` (three commits, two
issues, one forged `--octopus` → exactly two, sorted; kills a land that copies every value);
`TestIssueBranchNameAUOnlyHasNoSlug` (kills a slug on an AU project);
`TestIssueBranchNameSlugIsBounded` (a 200-character non-ASCII title; kills an unbounded or illegal ref
name). **Acceptance:** `go test ./control-plane/internal/gitx/... ./control-plane/internal/lanes/...
./control-plane/internal/converge/... -race -count=1` prints `ok` for each package. It does **not**
claim the `landed` move (task 22), any Hub write, or where a project's data class lives: that home is
**UNVERIFIED until merge** (Phase T task-01.md:454-456 says the same), so `auOnly` is a parameter
here and task 24 supplies it. Design: 05 §2.3, §3.3 (`landed`), §3.6 (trailers carry ids); 04 §5.2,
§7.3. Parity: with task 22 it closes the W1 parts of **L-146** and **L-200**; with task 24 the W1
part of **L-198**.

**22 — phase 3 delta: the `started`, `in_review` and `landed` fact moves, written as the person.**
Stream WP, seam **SW.4** (consumed; task 10 defines `StateMove{to, cause, reason}`), 3 h. Depends on
05 (the Work fake), 21 (the lane's `IssueID`, the land trailers) and P3-09 (land). One new file,
`control-plane/internal/converge/issuefacts.go` (+ `issuefacts_test.go`), holds
`type IssueMover interface { MoveState(ctx context.Context, issueID string, m hubapi.StateMove) error }`
(satisfied by `hubclient.Work` from task 04, so `converge` imports only `hubapi` and `hubclient`, the
boundary of `hub-contract.md:78-82`) and `type IssueFacts struct{ Mover IssueMover; Clock func()
time.Time }` with three calls: `LaneBound(ctx, lane lanes.Lane)` → `started`, cause `lane_bound`;
`LaneReady(ctx, lane lanes.Lane)` → `in_review`, cause `lane_ready`; `Landed(ctx, landCommit string,
issueIDs []string)` → `landed`, cause `land`, one move per trailer value task 21 wrote. The
callers are three one-line additions in files phase 3 or phase 2 own: P3-04's create-worktree route
(when `CreateRequest.IssueID` is set), the built `handlePatchWorktree`
(`control-plane/internal/api/catalog.go:33`) when `registry.WorktreeFlags.Ready`
(`registry/filestore.go:34`) turns true on a lane with an `IssueID` (the queue holds exactly the ready
lanes, P3 task-08.md:115-143), and P3-09's land after `update-ref` succeeds. **Every move is written as
the person whose act caused it** (05 §3.6): the call runs on the signed-in person's Hub token through
the task 16 relay, never a service token, and never with `actor: null`. **Facts are monotonic and
re-derivable:** a move to a state the issue has passed is not sent; a refused or failed send never
fails the git act (the land already happened, and git is the truth); and a sweep on P3-07's lane-sync
tick re-sends `landed` for any issue whose trailer is on the branch while the Hub still shows an
earlier state, so a lost send heals without an outbox on disk. **Tests first:**
`TestLandMovesEachIssueToLandedAsTheLander` (the fake records the lander as actor; kills a move written
as the system); `TestReadyMovesToInReview` (kills a move sent on queue reorder rather than on ready);
`TestBindWithoutIssueMovesNothing` (kills a move on every lane); `TestMovesNeverGoBack` (issue at
`landed`, lane marked ready again → nothing sent; kills a regression to `in_review`);
`TestLandSucceedsWhenTheHubRefuses` (fake returns 503 → the land's result is unchanged; kills a land
gated on the Hub); `TestSweepResendsALostLanded` (kills a fact lost after one failed send);
`TestLandedIgnoresInvalidTrailers` (kills a move for a forged value). **Acceptance:** `go test
./control-plane/internal/converge/... ./control-plane/internal/api/... -race -count=1` prints `ok`. It
does **not** claim `done` (task 23), the authorisation row that stops a sync token writing a fact
state (task 07, SW.3), or the desktop's flash (task 41). Design: 05 §3.3 (the state table), §3.6
(written as the person), §2.1 point 3 (an agent cannot claim progress); 04 §7.3. Parity: closes the
W1 parts of **L-8** (fact states set by facts) and **L-201** (states follow lane facts), and with
task 21 **L-146** and **L-200**.

**23 — phase 3 delta: the done evaluator; observed evidence at the landed commit; *self-attested*.**
Stream WP, seam **SW.4** (consumed: the done rule's inputs and result words), 3 h. Depends on 22 and
P3-15. It adds two new files to phase 3's `control-plane/internal/evidence` (**UNVERIFIED until
merge**; `Record`, `Store` and `Evaluator.States` at P3 task-15.md:48-163): `issuedone.go` and
`observe.go`, with tests. `issuedone.go`: `type IssueDoneInput struct{ IssueID, LandCommit, FiledBy,
StartedBy string; Serves []briefref.Criterion; StopAtLanded bool }` and `func (e *Evaluator)
IssueDone(ctx, repo, projectPath string, in IssueDoneInput) (IssueDone, error)`, whose result carries
`Word` in the closed set `proven` · `proven_self_attested` · `not_proven` · `no_criteria`, plus
`Open []string` (criterion ids still unproven) and `Evidence []string`. The rule (05 §3.3): for each
criterion served, some record **cites `IssueID`**, sits at or after `LandCommit` (`merge-base
--is-ancestor LandCommit r.Commit`, exit 0), is `settled`, and is not retired by States' revert walk,
which is reused and never copied; a sibling's evidence and evidence recorded before the land never
count. Empty `Serves` gives `no_criteria`, which moves to `done` at `landed` unless the team switch
`StopAtLanded` holds it. *Self-attested* means a non-observed record whose recorder is both `FiledBy`
and `StartedBy`, which is every solo-mode Record; any observed record makes the word `proven`.
`observe.go`: `type Check struct{ CriterionID, Command string; Timeout time.Duration }`,
`type Observation struct{ Command string; ExitCode int; Commit string; RanAt time.Time }` and
`func (o *Observer) Run(ctx, repo, projectPath, commit string, c Check) (Observation, error)`, which
runs the command in a **fresh detached clone at the landed commit** inside a sandbox (never the
agent's lane, never on the host), keeps only the exit code, and discards output unread (`OD-8`). Exit 0
appends a Record as the lander, `verdict: settled`, `because` "<command>, exit 0, run by the platform
at <time>", marked observed and citing the issue; any other exit writes nothing. After a land (task
22's `Landed`) and after a person's Record (P3-15's `POST /v1/projects/{projectId}/evidence`),
`IssueDone` runs for each landed issue serving that criterion, and a `proven*` or `no_criteria` word
sends `StateMove{to: done, cause: evidence}` as that person. **Two inputs the Hub does not have
yet:** hub-contract §7.10's evidence body (`hub-contract.md:1060-1083`) has no issue citation and no
observed mark, and SW.1 has no field naming a criterion's check. This task assumes task 10 adds
`issueId` and `observed` to the evidence record and task 03 adds `Check` to a `serves` entry, both
added fields (allowed after WS0); a part that finds them absent raises it, it does not invent a
store. **Tests first:** `TestIssueDoneIgnoresEvidenceBeforeLand` (kills a missing at-or-after
check); `TestIssueDoneIgnoresSiblingEvidence` (kills a rule without the citation);
`TestObservedExitZeroClosesWithNoClick` (kills a rule that needs Record);
`TestObservedNonZeroWritesNothing` (kills a failure recorded as evidence);
`TestSelfAttestedNeedsAllThree` (kills a flag built from two of the three people);
`TestObservedIsNeverSelfAttested`; `TestEmptyServesDoneAtLandedUnlessHeld` (kills an ignored team
switch); `TestRevertedLandRetiresIssueEvidence` (kills bypassing the revert walk);
`TestObserverRunsInAFreshClone` (a lane holds an uncommitted file that makes the test pass → the check
still fails; kills running in the lane). **Acceptance:** `go test ./control-plane/internal/evidence/...
-race -count=1` prints `ok`. It does **not** claim the *done, not proven* glyph (tasks 28, 48), a
hand-set `done` (task 10's server rule), W3's proposed evidence (task 104), or which runner the
sandbox uses in local mode (P3-24's runner or the phase 1 Docker sandbox, **UNVERIFIED**). Design:
05 §3.3, §2.1 point 4, §5.3 row 7. Parity: closes the W1 part of **L-8** (`done` is a fact) with task
22; it is what makes the demo's no-click `done` (05 §5.3) true.

