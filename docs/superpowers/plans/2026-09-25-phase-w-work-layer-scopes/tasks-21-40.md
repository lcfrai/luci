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

**24 — control plane: Start from an issue, *Undo Start*, the data-class route.** Stream WP, defines
seam **SW.10**, 3 h. Depends on 16 (the relay, `Deps.Work`), 19 (`Session.IssueRef`, SW.7) and 21
(`IssueBranchName`, `Lane.IssueID`); P3-04 and P3-11 (create and remove a worktree) arrive through 21.
Files: new `control-plane/internal/workstart/start.go`, `undo.go` and tests; new
`control-plane/internal/api/workstart.go` (+ test); ⚑ `internal/api/api.go` (`Deps.WorkStart`,
registration), ⚑ `routes_audit_test.go` (catalog rows), ⚑ `cmd/api/{config,main}.go` (wiring).
**SW.10, the wire:** `POST /v1/work/start` with `{projectId, issueId, branch, accept?: bool,
route?: "global"|"au", requestId}` → `201 {issueKey: "issue:is-…@r", worktreeId, sessionId, branch,
route, undo: "undo_start"}`; `POST /v1/work/start/{sessionId}/undo` → `200 {undone: true, cost:
null|{cents}}`, or `409 first_write` with `{next: "discard"}` once the session has written a file;
`POST /v1/work/start/{sessionId}/discard` with `{confirm}`, P3-11's two-step confirm token. A repeat
`requestId` returns the first result (a double press is one Start). **Start**, in order: read the
issue head and its latest revision through `Deps.Work`, and pin that revision as the `IssueRef` (the
agent is told `@r`, never "latest"); with `accept`, move `triage → todo` first, as the person (L-47's
*accept and Start*); create the lane through `lanes.Creator` with `Name: IssueBranchName(…)` and
`IssueID`; create the session on it through the built `POST /v1/sessions` path
(`sessionCreateRequest`, `internal/api/sessions.go:696-707`, which task 19 extends with `IssueRef`,
and `ContextRef` set to the first `serves` entry's brief key); then task 22's `LaneBound` writes
`started`. A failure at any step undoes the steps before it. **The route:** a `DataClass` interface
(`AUOnly(ctx, projectID) (bool, error)`) forces `au` and a slug-free name for an *AU data only*
project, and **an unknown class fails safe to AU** (its home is **UNVERIFIED until merge**, as task
21 says); otherwise the person's choice, default `global` (`OD-19`). In local mode no gateway exists,
so the route is returned and shown, and phase 6's token mint consumes it later. ***Undo Start***
(05 §4.6): allowed until the **first file write**, observed mechanically as the lane's `git status
--porcelain=v1 -z` turning non-empty or its `HEAD` moving (read through the lane runner, never the
PTY, `OD-8`), with no timer, so it holds past six seconds. It stops the session, removes the
worktree, and moves the issue back to its state before Start with cause `start_undone`, as the
person who started it; SW.4 must accept that one backward cause (task 10). `cost` is `null` in local
mode (provider rate through the gateway, phase 6, **UNVERIFIED**). **Tests first:**
`TestStartBindsLaneSessionAndIssue` (kills a session without `IssueRef`); `TestStartPinsTheRevision`
(the Hub writes `@4` after Start → the session keeps `@3`; kills re-reading latest);
`TestStartAUOnlyForcesAUAndNoSlug` (kills the person's choice overriding the class);
`TestStartUnknownClassFailsSafe` (kills a Global default); `TestStartFromTriageAcceptsFirst` (kills
starting an unaccepted issue); `TestStartRollsBackOnSessionFailure` (kills a half-made Start);
`TestUndoBeforeFirstWriteRemovesAll` (kills a clone left behind); `TestUndoAfterFirstWriteRefused`
(kills an undo that throws work away); `TestUndoHoldsPastSixSeconds` (kills a timer-based undo);
`TestDiscardNeedsConfirm` (kills a one-step discard); the catalog row in `routes_audit_test.go`.
**Acceptance:** `go test ./control-plane/internal/workstart/... ./control-plane/internal/api/... -race
-count=1` prints `ok`. It does **not** claim the desktop chip or `Shift+R` (task 61), the 10-second
first sign (task 61), Delegate (W3, task 102), or cloud sessions (phase 6 stream A, phase 8 stream C).
Design: 05 §2.1, §3.4 (issue text enters only after its owner adopts it, here by Start), §4.5
(Develop row), §4.6. Parity: closes the W1 parts of **L-39** (the delegate is the assignee's own
agent), **L-175** (Start with the issue in context, local mode first), **L-181** and **L-198**; with
task 19 **L-191**; with tasks 42 and 61 **L-47**'s *accept and Start*.

**25 — control plane: the Linear public-API and CSV import adapters.** Stream WP, consumes seam
**SW.6** (task 15's normalised batch, idempotent by `(source, externalId)`), 3 h. Depends on 15 and 16.
Files: new `control-plane/internal/workimport/linear.go`, `csv.go`, `batch.go` and tests with
hand-written fixtures under `workimport/testdata/` (written from Linear's public API documentation,
never copied responses, `OD-11`); new `control-plane/internal/api/workimport.go` (+ test); ⚑ `api.go`
(`Deps.WorkImport`), ⚑ `routes_audit_test.go`, ⚑ `cmd/api/{config,main}.go`. **Wire:** `POST
/v1/work/import/preview` and `POST /v1/work/import`, body `{projectId, source: "linear"|"csv",
linear?: {apiKey, teamKey?}, csv?: {content}}` → `200 {created, unchanged, refused: [{at, reason}]}`
(`at` is a CSV line number or a Linear identifier; plain counts only, `OD-27`). **Linear:**
`LinearPager` walks the public GraphQL API's `issues` connection by cursor to the end, 50 per page,
honouring `Retry-After` on 429, and maps each issue to one batch item: title, description (as
`text·H` of the importing person, the source identifier kept as `externalKey`), priority (Linear's
0-4 to `none`·`urgent`·`high`·`medium`·`low`), labels by name; Linear's workflow state is **not**
carried, because every import lands in `triage` as *not tied · imported* and nothing imported may
set a state (05 §2.3 point 4). The API key lives in memory for one request: never logged, never
written under `statepath`, never sent anywhere but the Linear host. **CSV:** `encoding/csv`, BOM
stripped, a header row with `title` required and `description`, `priority`, `labels` (`;`-separated),
`id` optional; without `id`, `externalId` is a digest of the row's normalised fields, so the same
file imported twice creates nothing the second time; at most 5,000 rows; every refusal names its
line. **Batch:** items go to the relay in chunks of at most 100. **Tests first:**
`TestLinearPagerFollowsTheCursorToTheEnd` (kills stopping at page one);
`TestLinearPagerHonoursRetryAfter` (kills retrying at once); `TestLinearKeyNeverLeaves` (captures
the logger and the state directory; kills a key in a log line or a file);
`TestLinearPriorityMapping` (kills an off-by-one between `none` and `urgent`);
`TestLinearStateIsNotCarried` (kills an import that arrives `done`); `TestCSVRequiresTitle` (kills a
silent drop); `TestCSVReimportIsIdempotent` (kills duplicates on re-import);
`TestCSVDigestIgnoresLineEndings` (CRLF and LF give one id; kills a digest over raw bytes);
`TestBatchChunksAtOneHundred` (kills one unbounded request). **Acceptance:** `go test
./control-plane/internal/workimport/... ./control-plane/internal/api/... -race -count=1` prints `ok`.
It does **not** claim the import dialog (task 69), Jira, Azure Boards or GitHub Issues import (W5,
tasks 150-153), comment import, or that `api.linear.app` is reachable from the corporate network
(**UNVERIFIED**; the CSV path is the fallback). Design: 05 §5.3 (row 5 and the demo's first step),
§2.3 point 4, §3.6. Parity: closes the W1 part of **L-220** (Linear and CSV importers), and with task
15 **L-9**'s *intake always lands in `triage`*.

