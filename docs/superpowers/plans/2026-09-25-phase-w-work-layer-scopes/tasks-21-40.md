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

**26 — tokens: 02 D2 step 1, `--t-fact`, the provisional `paper` block.** Stream WD1, defines the
token half of seam **SW.11**, 2 h, no dependency (the look needs no Hub). Files: ⚑
`desktop/src/styles/tokens.css` (191 lines today; `:root` from line 19, `--ink-3` 3.43:1 at `:47`,
motion at `:135-142`, faces at `:151-152`, the reduced-motion floor at `:174-183`, the focus ring
at `:187-191`) and ⚑ `desktop/src/styles/tokens.test.ts` (its `describe("the token layer")` at
`:41`, which already bans literal hex and `transition: all`). It adds to `:root`, each with a
one-line comment naming its source: `--dur-flash: 600ms`, `--dur-peek: 120ms`, `--undo-window: 6000ms`
(02 C7; 05 §4.6); the spacing scale `--s-1` … `--s-6` = 4 · 8 · 12 · 16 · 24 · 32 px (02 C4 names
`--s-1 … --s-8` but lists six values, so six are declared and the return lists it); `--row-h: 32px`,
`--row-h-compact: 28px`, `--card-3line: 76px` (02 C4; 05 §4.4 C4); the type scale `--t-micro` 11/14
(mono, **`kbd` hints only**), **`--t-fact` 12/16 mono** (new in 05 §4.4 C3: every id, key, path,
time, cost and duration), `--t-meta` 12/16, `--t-ui` 13/20, `--t-body` 14/22, `--t-head` 16/22 at
weight 560, `--t-display` 24/28 mono (02 C3), each as a size token and a line-height token; and
`--icon` 16px / 1.5px stroke, `--icon-sm` 14px / 1.25px (02 C6). The prototype
(`prototype/work-layer.html:40-41`) sets `--t-body` 13px; this task follows 02 C3, where 13px is
`--t-ui`. It then adds one **provisional** `[data-theme="paper"]` block: a light ground re-derived
by hand in OKLCH (not an inversion of graphite), `color-scheme: light`, the same token names, and
status colours `--run`, `--need`, `--fail`, `--drift` with lightness re-derived for the light ground
so their meaning never changes (02 C1). It is provisional because W2's generator (tasks 95, 96)
replaces it with emitted blocks; the names it declares are the contract the generator must keep.
**Tests first**, appended to `tokens.test.ts`: `it("declares 02 D2 step 1's tokens")` (kills a
missing token); `it("sets --t-fact to 12px and --t-micro to 11px")` (kills facts at 11px, 05 §4.4
C3); `it("keeps --undo-window at 6000ms and --dur-flash at 600ms")` (kills drift from 05 §4.6 and
DESIGN-SYSTEM's flash); a contrast helper (OKLCH → sRGB → WCAG relative luminance, written in the
test, no dependency) and `it("clears the ink floors in graphite and paper")`: `--ink` ≥ 15:1,
`--ink-2` ≥ 7:1 (the floor for any fact), `--ink-3` ≥ 3:1 on `--page`, `--canvas` and `--surface`
(kills a paper block whose inks fall below graphite's floors); `it("gives paper its own status
lightness")` (each status colour ≥ 3:1 on paper's `--page`; kills graphite's status values copied
unchanged); the existing no-hex rule now also covers the paper block (kills a hex literal there).
**Acceptance:** `npm --prefix desktop test -- tokens` passes; `npm --prefix desktop run typecheck`
is clean; `grep -c "^  --t-fact" desktop/src/styles/tokens.css` prints `1`. It does **not** claim
any visible change (nothing uses the tokens until tasks 30-31), a theme picker (C11 `first.keys`,
later), the high-contrast pair (W2), that existing component CSS uses only `--s-*` (only new
Phase W directories are held to it, by task 30's audit rule), or that paper looks right: that is
the owner's eyes at task 39. Design: 05 §4.4 (C1, C3, C4, C7); 02 C1, C3, C4, C6, C7, D2 step 1.
Parity: the W1 part of **L-234** (a light theme beside the dark one; the generated four are W2).

**27 — faces: Commit Mono bundled with its licence; Segoe UI Variable as the sans.** Stream WD1,
the face half of seam **SW.11** (`--mono`, `--sans`), 2 h. Depends on 26. `OD-28` decides both faces
(Q10, Q14). Files: new `desktop/src/assets/fonts/CommitMono-*.woff2` (the weights the tokens use,
400 and 500 at least; which files the current Commit Mono release ships, static or variable, is
**UNVERIFIED** until fetched, and a part records the release tag and each file's SHA-256), new
`desktop/src/assets/fonts/OFL-CommitMono.txt` (the licence text from that release, verbatim), new
`desktop/src/styles/fonts.css` (`@font-face` rules, `font-display: block`, local `url()` imports
Vite fingerprints) imported first by ⚑ `desktop/src/styles/app.css`, and ⚑ `tokens.css:151-152`:
`--mono: "Commit Mono", "Cascadia Mono", ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono",
monospace` (a real monospace chain, because xterm.js is configured with `--mono`, `tokens.css:149-150`)
and the sans kept on the system face with optical sizes: `--sans: "Segoe UI Variable Text", "Segoe
UI", -apple-system, system-ui, sans-serif`, plus `--sans-small` (`"Segoe UI Variable Small"`, for
`--t-meta`) and `--sans-display` (`"Segoe UI Variable Display"`, for `--t-head`), each falling back
to `--sans`. **Segoe UI Variable is never bundled**: it is the Windows system face and its licence
does not allow redistribution; on macOS and Linux the stack falls to the platform sans. The terminal
must not measure its cells from a fallback: `desktop/src/components/Terminal.tsx` hard-codes its
own stack today (`fontFamily: '"Cascadia Mono", …'` at `:38`, not `--mono`), so it gains Commit Mono
first, and it awaits `document.fonts.load('12px "Commit Mono"')` (bounded at 1 s, then proceeds)
before its first `fit()` (`:120`). The
Tauri CSP is `null` today (`desktop/src-tauri/tauri.conf.json:21`), so local fonts need no CSP
change; a part that finds a CSP adds `font-src 'self'`. The prototype lists `"Berkeley Mono"` first
(`prototype/work-layer.html:38`); production does not. **Tests first:** `it("ships a licence beside
every bundled face")` (every family in `assets/fonts/` has an `OFL-<Family>.txt` containing "SIL Open
Font License" and "Version 1.1"; kills a font with no licence); `it("bundles no Segoe face")` (no
`@font-face` or file names Segoe; kills redistributing a system font); `it("puts Commit Mono first
in --mono and keeps a monospace fallback")` (kills Berkeley first, and a chain ending in a
proportional face); `it("imports fonts.css before any rule")` (kills a late import that shifts
layout); `Terminal.test.tsx`: `it("fits after the mono face loads")` with `document.fonts` stubbed
(kills a fit measured on the fallback). **Acceptance:** `npm --prefix desktop test` passes;
`npm --prefix desktop run build` succeeds and `ls desktop/dist/assets | grep -c woff2` prints at
least `1`. It does **not** claim the sans candidates' comparison (task 32, where Segoe UI Variable
stands beside the two alternatives 05 §4.4 C3 names, for the owner's eyes), how the faces render at
125 % and 150 % (task 33), or that Commit Mono's licence was reviewed by legal (**UNVERIFIED**; OFL
1.1 is on `OD-11`'s allowed list). Design: 05 §4.4 C3 (faces, revision 2); 02 C3; `OD-28`, `OD-11`.
Parity: no 01 row names a face; it serves **L-234**'s look with task 26.

**28 — drawn glyphs, part 1: the nine issue states, *done, not proven*, the priority notch.**
Stream WD1, defines `<StateGlyph/>` in seam **SW.12**, 3 h. Depends on 26 (`--icon`, `--icon-sm`).
Files: new `desktop/src/components/glyphs/StateGlyph.tsx`, `glyphs/paths.ts` (our path data, with
a header saying it was drawn for PanaceaApp from no third-party source, `OD-11`), `glyphs/glyphs.css`
(owns the `.gl-*` prefix and no other) and `StateGlyph.test.tsx`. **SW.12's first member,
complete:** `export type IssueState = "triage" | "backlog" | "todo" | "started" | "in_review" |
"landed" | "done" | "cancelled" | "duplicate"`; `export type Priority = "none" | "urgent" | "high" |
"medium" | "low"`; `export function StateGlyph(props: { state: IssueState; priority?: Priority;
notProven?: boolean; size?: 14 | 16 }): JSX.Element`. Task 37's desktop issue-state type must be
assignable to `IssueState` (a type test there), since this task lands first. **The drawing** (05
§4.4 C6 and work rules 1-2): SVG in `currentColor`, `fill="none"` strokes, one **distinct outline
shape per state**, none that fills as work progresses, so the component takes no progress input at
all; the 14px and 16px sizes are drawn separately at 1.25px and 1.5px strokes, never one drawing
scaled. `prototype/work-layer.html:393-404` sketches `g-triage` … `g-cancel` and `g-dnp`; it has no
`duplicate`, which is drawn here. ***Done, not proven*** (05 §3.3) is its own shape, not a colour,
and applies only with `state="done"`. **Priority is a notch** cut into the glyph's leading edge, its
depth growing from `low` to `urgent`, `none` uncut, so a row carries one glyph, not a second set of
signal bars; urgent is ink like the rest, because `--need` means only *someone is waiting on you*.
Each glyph is `role="img"` with an `aria-label` such as "In review, high priority" or "Done, not
proven". **Tests first:** `it("draws nine distinct state shapes and a distinct not-proven")` (the
serialised geometry of all ten is unique; kills two states sharing a shape); `it("accepts no progress
input")` (`// @ts-expect-error` on a `progress` prop; kills a filling icon, 02 C13); `it("uses
currentColor and no status colour")` (no `--run`, `--need`, `--fail` in `.gl-*` rules; kills an issue
state coloured like an agent state, rule 1); `it("draws priority as one notched glyph")` (five
priorities → five leading edges, one `<svg>`, identical body; kills signal bars); `it("never colours
urgent")` (kills rule 2's breach); `it("shows not-proven only on done")` (`notProven` with `landed`
renders `landed`; kills the mark on a state it cannot mean); `it("names every glyph for assistive
technology")` (kills an unlabelled image); `it("draws 14 and 16 separately")` (kills a scaled
drawing). **Acceptance:** `npm --prefix desktop test -- glyphs` passes; typecheck is clean. It does
**not** claim the glyphs read well at 125 % and 150 % or on `paper` (task 33 renders them; the
owner's eyes judge at task 39), the forced-colors rendering (task 34), or any surface using them.
Design: 05 §3.3, §4.4 (C6, work rules 1 and 2); 02 C6, C13. Parity: gives **L-8**'s states and
**L-13**'s five priorities their shapes; it closes neither alone (tasks 40, 46 show them).

**29 — drawn glyphs, part 2: agent shapes, selection, needs-you, `thread.node`; the licence check.**
Stream WD1, completes seam **SW.12**, 3 h. Depends on 28. Files: new `glyphs/AgentGlyph.tsx`,
`glyphs/MarkGlyph.tsx` and tests; ⚑ `desktop/src/components/common/StatusDot.tsx` and `common.css`
(not in the skeleton's file map, see below); new `desktop/src/components/glyphs/licence.test.ts`.
**Complete:** `export type AgentStatus = "starting" | "running" | "idle" | "exited" | "failed"`;
`export function AgentGlyph(props: { status: AgentStatus; size?: 14 | 16; label: string }):
JSX.Element`; `export type MarkKind = "selected" | "needs_you" | "thread_node"`; `export function
MarkGlyph(props: { kind: MarkKind; size?: 14 | 16; lit?: boolean }): JSX.Element`. **Agent shapes**
(05 §4.4 C7, reduced motion): **running is a disc inside a 1px open ring**, idle a plain disc, starting
an open ring, exited a square, failed a square with a cut, so shape alone separates them; colour
(`--run`, `--fail`) is added on top. The built dot has the defect this fixes: `StatusDot.tsx:18-27`
maps `running` and `idle` both to `dot-disc`, so they differ by colour alone. `StatusDot` therefore
maps `running` to a new `dot-run` drawn from the same geometry, so the rail and the Work lens speak
one vocabulary; its flash (`FLASH_MS = 600`, `:29`) and its contract comment stay. The breath uses
`beatDelayStyle()` (`desktop/src/hooks/heartbeat.ts:23`), the one heartbeat, and the live part is
`aria-hidden` with `label` as its text. `failed` is shown only where the wire carries an observed
failure (a bound lane failed, 05 §4.4 rule 1); it is never inferred. **Marks:** `selected` is a
leading check, a shape, because *lit is not selected* (05 §4.1); `needs_you` is a shape in `--need`,
so an escalation is never amber alone (05 §4.4 rule 4); `thread_node` is the 5px ring in `--thread`
(`tokens.css:57`) at `--e-hair` geometry, brighter when `lit` (02 C2), which task 36's `ThreadNode`
renders. **The licence check** (05 §5.3 row 9; 02 C6): `licence.test.ts` reads
`desktop/package.json` and each direct dependency's own `package.json` and fails on any licence
outside MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC or OFL-1.1 (an SPDX `OR` passes if one
alternative does); W1 adds **no** icon family: every glyph, including the prototype's `i-*` icons,
is drawn by us. **Tests first:** `it("gives running and idle different shapes")` (kills today's
colour-only difference); `it("separates all five statuses with colour removed")` (kills any pair
told apart by colour); `it("breathes on the one heartbeat")` (a `setInterval` spy sees no call; kills a
per-row timer); `it("hides liveness from assistive technology")` (kills an announced breath);
`StatusDot.test.tsx`: `it("draws running as dot-run")` (kills the rail keeping the old shape);
`it("draws selection as a check")` (kills selection by ring alone); `it("gives needs-you a shape")`
(kills amber alone); `it("draws thread_node as a ring, not a disc")`; `it("shares no path with the
issue glyphs")` (kills one icon family for issue and agent states, rule 1); `it("allows only
permissive licences")` (kills a GPL, AGPL or commercial dependency). **Acceptance:** `npm --prefix
desktop test -- glyphs StatusDot` passes. It does **not** claim AX's seven statuses (the built wire
has four, `StatusDot.tsx:9-15`), the reduced-motion tick (task 34) or a licence review by legal
(**UNVERIFIED**). Design: 05 §4.1, §4.4 (C6, C7, rules 1 and 4); 02 C2, C6. Parity: gives **L-184**'s
states a shape each and **L-225**'s multi-select its check; it closes neither alone.

**30 — `voice.split` across Develop and the session pane (02 D2 step 3).** Stream WD1, no seam of
its own (it applies SW.11), 2.5 h. Depends on 27. Human-language lines are sans; every machine fact
(id, key, path, time, duration, exit code, cost, count of tokens, coordinates) is mono at `--t-fact`
12px in `--ink-2` or stronger, never 11px and never `--ink-3` (05 §4.4 C3; D I7). The built tree
breaks both rules today: `.pane-foot` (`desktop/src/components/develop/develop.css:244-255`) sets
`font: 400 11px/1.4 var(--mono)` in `--ink-3`, `.pane-sub` (`:120-124`) sets 11px sans `--ink-3`,
and `.cv-coords` (`:386`) carries live numbers. Files: ⚑ `develop.css`; `SessionPaneHeader.tsx`
(`.pane-title` at `:36`, `.pane-sub` at `:37`) and `SessionPaneFooter.tsx` (`.pane-foot-fact` at
`:65`), each field classified in a table in the part (a worktree name or a harness's product name
is sans; a branch, id, path, duration or cost is mono); ⚑ `desktop/src/styles/designsystem.test.ts`
(`describe("the design-system audit")` at `:596`, and `describe("the audit's own rules, fed a planted
tree")` at `:849`). **Three audit rules**, each with a planted violation in the `:849` block so the
rule is shown to fire: (A) a rule that sets a mono face uses `--t-fact`, unless its selector is a
`kbd` hint (`kbd`, `.kbd`, `-kbd`), which alone may use `--t-micro`; (B) no rule that sets a mono face
sets `color: var(--ink-3)`; (C) CSS under the new Phase W directories (`components/work`, `issue`,
`desk`, `glyphs`, `thread`, `devtools`) uses only `--s-*` for `padding`, `margin` and `gap` (the rule
task 26 deferred). **The old look stays reachable:** each replaced declaration is kept under a
`:root[data-look="old"]` selector in the same file, so task 32's toggle flips old and new on the same
screen; nothing is deleted until the owner accepts the new look (task 39 records it). **Tests
first:** audit rule A over the real tree (kills `.pane-foot` left at 11px); rule B (kills facts left
in `--ink-3`); rule C over the planted tree (kills a literal `padding: 10px` in a Phase W file);
each planted violation fails its rule (kills a rule that never fires); `SessionPaneFooter`:
`it("renders facts mono and the attach reason sans")` (kills the human sentence set in mono);
`SessionPaneHeader`: `it("keeps the worktree name sans and the branch mono")` (kills the split
reversed); `it("restores the old declarations under data-look=old")` (kills a change that cannot
be flipped back). **Acceptance:** `npm --prefix desktop test -- designsystem develop` passes;
`grep -n "11px/1.4 var(--mono)" desktop/src/components/develop/develop.css` finds the string only
inside a `[data-look="old"]` block. It does **not** claim the rail or the Discover lens (other tasks
touch them), the three sans candidates (task 32), or that the split reads better: the owner judges at
task 39. Design: 05 §4.4 C3; 02 C3, D2 step 3; 07 D I7. Parity: no 01 row; it is one of 02 C13's five
things every screen carries that Linear does not, `voice.split`.

**31 — `chrome.dim` and one `ViewHeader` with the route badge (02 D2 step 2).** Stream WD1, defines
seam **SW.13**, 3 h. Depends on 26. Files: new `desktop/src/components/common/ViewHeader.tsx` and
`ViewHeader.test.tsx`; ⚑ `desktop/src/styles/app.css` (82 lines; its token-first import and "no
colour of its own" rule are pinned by `tokens.test.ts:85`); ⚑ `desktop/src/components/rail/rail.css`;
and `desktop/src/components/subrail/subrail.css`, which the skeleton's file map omits although
02 C5 dims the sub-rail too. **SW.13, complete:** `export type Route = "global" | "au"`;
`export function ViewHeader(props: { title: string; route: Route | null; badge?: ReactNode;
actions?: ReactNode }): JSX.Element`: the title at `--t-head` in sans; `badge` is the scope chip
(for the Work lens, `EP · Resumable uploads` with the branch in mono, 05 §4.1); the route badge
reads `Global` or `AU` in mono at `--t-fact` and is hidden only when `route` is `null` (unknown; a
header never guesses `Global`); `actions` sit on the right. One layout, one component: every new
surface (tasks 38-48, 62, 85-91, 126-132) uses it, and this task moves the built lens titles onto it
(`components/lens/LensPlaceholder.tsx`, and the Develop lens's title row wherever it renders at the
sync base, `DevelopLens.tsx` or `topbar/TopBar.tsx`). **`chrome.dim`** (02 C5): rail and sub-rail
labels at `--ink-2`, their icons at `--ink-3`, the current item at `--ink` with a 2px `--accent`
tick on its leading edge (current state, the only place accent is legal); the work area keeps
`--ink`, so the work is the brightest thing on screen. Old declarations stay under
`:root[data-look="old"]` for task 32. **Tests first:** `it("renders the route badge in mono")`
(kills the badge set in sans or dropped); `it("hides the badge when the route is unknown")` (kills a
guessed `Global`); `it("puts actions after the title in one row")` (kills a second layout);
designsystem audit `it("has one view header")` (no `<h1>` or `*-view-title` class outside
`ViewHeader.tsx`; kills a second header pattern, 02 C5 and AX §9 item 8); `it("dims rail and
sub-rail labels to ink-2")` (kills chrome as bright as the work); `it("uses accent only on the
current item")` (kills accent on resting rows); `it("marks the current item with a tick, not colour
alone")` (kills a colour-only current state); `it("keeps the old chrome under data-look=old")`.
**Acceptance:** `npm --prefix desktop test -- ViewHeader designsystem rail` passes; typecheck is
clean. It does **not** claim the route is right: the badge shows what it is given (task 24 computes
it; phase 6 enforces it), nor the Desk's or Plan's headers (tasks 45, 62). Design: 05 §4.4 C5, §4.1;
02 C5, D2 step 2; `OD-19` (the switch made visible). Parity: no 01 row names chrome; it is the
visible half of `OD-19`'s per-project route.

**32 — `flag.compare` in the dev toolbar: old and new look, the three sans candidates (02 D2 step
7).** Stream WD1, 2 h. Depends on 27 and 31. Files: new `desktop/src/lib/flags.ts` (+ test), new
`desktop/src/components/devtools/FlagCompare.tsx`, `devtools.css` (+ test); the toolbar mounts in
`desktop/src/App.tsx` behind `import.meta.env.DEV` (App.tsx is not in the skeleton's file map; the
return lists it). **Complete:** `export type Look = "old" | "new"`; `export type SansCandidate =
"segoe" | "atkinson" | "geist"`; `export type ThemeName = "graphite" | "paper"`; `export interface
Flags { look: Look; sans: SansCandidate; theme: ThemeName }`; `export function getFlags(): Flags`;
`export function setFlag<K extends keyof Flags>(key: K, value: Flags[K]): void`; `export function
useFlags(): Flags` (a `useSyncExternalStore` hook). `setFlag` writes `data-look`, `data-sans` and
`data-theme` on `<html>`, which tasks 26, 30 and 31's rules read, so a flip is a two-second
comparison on the **same** screen with no remount and no reload (02 C12). Flags live in memory only:
they are preferences for a dev session, never persisted. **The sans candidates** (05 §4.4 C3,
revision 2): `OD-28` chose Segoe UI Variable, and the toolbar still sets it beside Atkinson
Hyperlegible Next and Geist so the owner's eyes confirm the choice at task 39. The two alternatives
are dev-only files under `desktop/src/devtools/fonts/`, each with its OFL text, loaded by a dynamic
`import()` inside the `DEV` branch, so a production build carries neither (Atkinson Hyperlegible
Next's licence and weights are **UNVERIFIED** until fetched; the part records the release and
SHA-256 of each file). **Tests first:** `it("writes each flag to the root element")` (kills a flag
that never reaches CSS); `it("flips the look without remounting")` (a typed value in an input
survives `setFlag("look", "old")`; kills a flip that reloads); `it("renders nothing in production")`
(with `DEV` false; kills shipping dev chrome); `it("applies each sans candidate")` (kills a candidate
the CSS ignores); `it("persists nothing")` (`localStorage` and `indexedDB` spies see no write; kills
a flag written to disk). **Acceptance:** `npm --prefix desktop test -- flags devtools` passes;
`npm --prefix desktop run build`, then `grep -rli "geist\|atkinson" desktop/dist` prints nothing.
It does **not** claim which look or face is better (the owner decides at task 39), a keyboard chord
for the toggle (none, so no cockpit chord is shadowed), or macOS rendering. Design: 05 §4.4 C3, C12;
02 C12, D2 step 7; `OD-1` (the review gate this makes cheap), `OD-28`. Parity: no 01 row; it serves
the review gate.

**33 — `stress.sets` with its three new screens.** Stream WD1, 3 h. Depends on 29 and 32. Files: new
`desktop/src/components/devtools/StressSets.tsx` (+ test), new `devtools/GrainMeter.tsx` (task 35
fills it; not in the skeleton's file map), and fixture screens built only from what exists by then:
the glyph sheet (every `StateGlyph`, `AgentGlyph` and `MarkGlyph` at 14 and 16px), `ViewHeader`, and a
Develop pane in each of DESIGN-SYSTEM's eleven states (loading, empty, sparse, dense, partial, stale,
offline, unauthorized, error, blocked, success; `docs/DESIGN-SYSTEM.md:90`). **Complete:** `export
function registerStressScreen(name: string, render: () => ReactNode): void` (later surfaces, tasks
38-48 and 62, register theirs); `export type StressSet = "themes" | "states" | "load" |
"reduced-deuteranopia" | "forced-colors" | "scaling"`; `export function StressSets(): JSX.Element`,
mounted in the dev toolbar beside `FlagCompare`. **The three new screens** (05 §4.4 C12; 07 D C4,
D I8): (1) **reduced motion × deuteranopia**: sets `data-motion="reduce"` on `<html>` (every reduced-
motion rule is written twice, as the media query and as this attribute, which task 34 holds to) and
applies an SVG `feColorMatrix` deuteranopia filter over the app root, using a published simulation
matrix the part cites by paper and severity; (2) **the Windows contrast theme**: `forced-colors`
cannot be simulated in page CSS, so the screen lays out every glyph, ring and selected row and tells
the reviewer to turn on a Windows contrast theme (or DevTools' *Emulate CSS forced-colors*, which
WebView2's DevTools is **UNVERIFIED** to offer); (3) **125 % and 150 % scaling**: a CSS `zoom`
preview, labelled *not the same as Windows display scaling*, beside the instruction to set Windows
scaling for the real check. Appearance covers `graphite` and `paper` (the high-contrast pair is W2).
**Tests first:** `it("applies each new set's attribute or filter")` (kills a set that changes
nothing); `it("uses the cited deuteranopia matrix")` (kills a wrong matrix); `it("renders every
glyph at both sizes on both grounds")` (kills a missing glyph); `it("lists all eleven states")`
(kills a partial floor); `it("keeps registered screens in registration order")`; `it("renders nothing
in production")` (kills shipping it). **Acceptance:** `npm --prefix desktop test -- StressSets`
passes. It does **not** claim that any screen passes: the sets are how the owner and reviewers look
(`CLAUDE.md`: visual acceptance is the owner's eyes), nor macOS WKWebView (no build exists,
**UNVERIFIED**). Design: 05 §4.4 C12, §5.3 row 8; 02 C12. Parity: no 01 row; 02 A10 point 8, the
quality habit.

**34 — the `forced-colors` layer; the reduced-motion running shape and static tick.** Stream WD1,
2 h. Depends on 29 and 33. Files: new `desktop/src/styles/forced-colors.css`, imported by ⚑ `app.css`
after the token layer; ⚑ `desktop/src/styles/tokens.css` (the reduced-motion floor at `:174-183`
gains its `:root[data-motion="reduce"]` twin, task 33's hook); ⚑ `glyphs/glyphs.css` (the tick); ⚑
`tokens.test.ts` and ⚑ `designsystem.test.ts`. **Forced colours** (05 §4.4 work rule 4; 07 D I8):
Chromium drops `box-shadow` under `forced-colors: active`, so every ring would vanish; inside
`@media (forced-colors: active)` each element that carries an `--e-*` elevation, `thread.node`,
`lease.ring` or the selected-row ring gets a 1px `border` or `outline` in `CanvasText`, the selected
row a 2px `Highlight` outline and the cursor row 1px, `thread.line` a `CanvasText` hairline, glyphs
stay `currentColor`, the grain is `display: none`, and an escalated lease keeps the needs-you
**shape** on its chip. The built focus ring is already an `outline` (`tokens.css:187-191`) and is not
touched. `prototype/work-layer.html:379-386` sketches the rules; they are rewritten against the real
class names. **Reduced motion** (05 §4.4 C7, revision 2; 07 D C4): with the breath stopped, running
keeps task 29's own shape, so shape alone separates running, idle, exited and failed; the state flash
is replaced by a **static 2px leading tick in `--ink`** that stays while the row carries
`data-unseen="true"` (the read cursor, AX §6.1; the lens that owns "seen" sets and clears it, tasks
40-41). The contract is the attribute: `data-unseen` shows the tick under reduced motion and the
`--dur-flash` flash otherwise. Every reduced-motion rule appears twice, as the media query and as
the `data-motion` twin, with identical declarations. **Tests first:** designsystem
`it("gives every ring a forced-colors border or outline")` (collects each selector using `--e-*`,
`--thread` or `--accent-ring` in a `box-shadow` and requires a forced-colors counterpart; kills a
ring that vanishes); `it("turns grain off under forced colours")` (kills grain over system colours);
`it("outlines the selected row in Highlight")` (kills an invisible selection);
`it("keeps each reduced-motion twin identical to its media rule")` (kills a twin that drifts);
glyphs `it("keeps running and idle apart with motion reduced")` (kills a colour-only difference);
`it("shows the tick only on an unseen row, in ink")` (kills a tick that never clears, or one drawn
in accent); `it("never transitions all")` (the existing rule covers the new file). **Acceptance:**
`npm --prefix desktop test -- designsystem tokens glyphs` passes. It does **not** claim how a real
Windows contrast theme looks (the owner checks it through task 33's screen), that WebView2 honours
`forced-colors` exactly as Chromium does (**UNVERIFIED**), or the live region (task 66). Design: 05
§4.4 (C7 reduced motion, work rule 4), §5.3 row 17; 07 D C4, D I8. Parity: no 01 row; with task 66 it
is W1's accessibility floor.

**35 — grain, measured at 150 % in WebView2.** Stream WD1, 2 h. Depends on 31. The grain is a fixed
2.2 % noise overlay on the shell today (`.grain::after` at `tokens.css:159-171`; applied once, by
`AppShell.tsx:82`, pinned by `AppShell.test.tsx:192-193`). 05 §4.4 work rule 6 (07 D M6) says it is
measured, not assumed, before the prototype review. Files: new `desktop/src/styles/grain.css`, which
takes the rule out of ⚑ `tokens.css` unchanged and adds: off under `:root[data-theme$="-hc"]`, off
under `forced-colors` (task 34 also says so; one rule, not two), and off under
`:root[data-grain="off"]`; ⚑ `lib/flags.ts` gains `grain: "on" | "off"`; new
`devtools/GrainMeter.tsx` (task 33's registered screen) scrolls a 500-row fixture list by
`requestAnimationFrame` for 5 s, twice, once with grain and once without, records each frame's
interval in the page, and prints p50 and p95 in ms, in `tabular-nums`, with the device pixel ratio
it ran at. The measurement runs **inside the real Tauri app in WebView2** at the laptop's own 150 %
Windows scaling, so no browser automation or remote debugging port is needed. The person running
it copies the two lines into a *Grain* section of `docs/verification/2026-09-25-phase-w-results.md`
(the skeleton gives this task no results file; the return lists it). **The rule, recorded rather
than asserted** (02 C8): if p95 with grain is within 1 ms of p95 without, and under 16.7 ms, grain
stays; otherwise the numbers go to the owner with the proposal to turn it off at 150 % and above.
**Tests first:** `it("moves the grain rule without changing it")` (the declaration block is byte-
identical; kills a tuned opacity slipped in); `it("turns grain off in high-contrast themes")` (kills
grain on `-hc`); `it("turns grain off with the flag")` (kills a meter that cannot compare);
`it("reports p50 and p95 from recorded intervals")` (a fed list of intervals gives known
percentiles; kills a mean reported as p95); `it("keeps grain on the shell only")` (the existing
`AppShell` test still passes; kills grain per component). **Acceptance:** `npm --prefix desktop test
-- grain GrainMeter AppShell` passes; the results file holds two measured lines with their DPR. It
does **not** claim a frame budget met: the numbers are recorded, **UNVERIFIED** until run, and the
owner accepts them. Design: 05 §4.4 work rule 6, C13 (grain is one of the five marks on dark; `paper`
carries the thread gutter instead); 02 C13; 07 D M6. Parity: no 01 row.

**36 — the thread primitives: node, line, lit, trace.** Stream WD1, defines seam **SW.14**, 3 h.
Depends on 29 (`MarkGlyph kind="thread_node"`) and 30. Files: new
`desktop/src/components/thread/ThreadNode.tsx`, `ThreadLine.tsx`, `ThreadTrace.tsx`, `useThreadLit.ts`,
`thread.css` and tests; ⚑ `tokens.css` gains `--thread-lit` (the prototype's value,
`prototype/work-layer.html:28`, rewritten in OKLCH) and raises `paper`'s `--thread` to full strength,
because on `paper` the thread gutter is the fifth mark in place of grain (05 §4.4 C13; `tokens.css`
is not listed for this task in the skeleton's file map); ⚑ `forced-colors.css` gains the thread
rules, since task 34's audit fails on a new ring without them. **SW.14, complete:** `export function
ThreadNode(props: { criterionId: string; href: string; label: string }): JSX.Element | null`: the 5px
ring, a link to its source span, `null` when `href` is empty, because a thread without a source is
deleted (02 C2; AX Principle 7); `export function ThreadLine(props: { criterionId: string; children:
ReactNode }): JSX.Element`: the 1px hairline in the leading gutter from which a group's rows hang
(05 §4.1); `export function ThreadTrace(props: { play: boolean; children: ReactNode }): JSX.Element`:
on drill-in the line grows from source to target with `transform: scaleY(0 → 1)`,
`transform-origin: top`, `--dur-slow`, `--ease` (motion that explains *this came from that*), and
appears at once under reduced motion; `export function useThreadLit(criterionId: string | null): {
lit: boolean; bind: { onPointerEnter(): void; onPointerLeave(): void; onFocus(): void; onBlur(): void
} }`, backed by one module-level store (`useSyncExternalStore`), so hovering **or focusing** a
criterion lights its node and line in every mounted surface at once, Work, Design and Develop.
**Lit is not selected** (05 §4.1, 07 D I1): lit brightens only the node and the line; it never rings
a row and never sets `aria-selected`. **Tests first:** `it("renders no node without a source")`
(kills a sourceless node); `it("makes the node a named link to its span")` (kills a decorative
node); `it("lights every node of a criterion across two trees")` (kills lit scoped to one lens);
`it("lights on focus as well as hover")` (kills a pointer-only thread); `it("never marks a lit row
selected")` (kills lit equal to selected); `it("traces with transform only, and not when motion is
reduced")` (kills animating `height`, and motion under `data-motion="reduce"`); `it("draws the node
as a 5px ring")` (kills a disc); `it("gives paper's thread full strength")` (kills paper at graphite's
alpha); the forced-colors audit from task 34 passes. **Acceptance:** `npm --prefix desktop test --
thread designsystem tokens` passes. It does **not** claim that any surface carries the thread yet
(tasks 40, 47, 67, 85 place it), Design's frames (phase 5), or that it reads as the signature: the
owner judges at task 39. Design: 05 §2.3, §4.1 (lit is not selected), §4.4 C2 and C13, §4.5 rule
4; 02 C2. Parity: no 01 row; this is the part Linear has no object to build (05 §2.3).

**37 — desktop Work wire types, relay calls and `workSlice`; `Lens` gains `"work"`.** Stream WD2
(the skeleton's WD1 row lists it with the lens), defines seam **SW.15**, 2.5 h. Depends on 05 (SW.2
frozen at WS0) and 20 (`src/test/workFixtures.ts`). Files: new `desktop/src/api/work.ts` and
`work.test.ts`; ⚑ `desktop/src/api/client.ts` (`CockpitClient` at `:132` gains `work: WorkClient`;
`createClient` at `:236`); new `desktop/src/store/slices/workSlice.ts`; ⚑ `store/types.ts` (`Lens` at
`:173` gains `"work"`); ⚑ `store/store.ts` (`CockpitState` at `:14` adds `WorkSlice`; `partialize` at
`:34-46` must not take any of it). **Wire types** mirror SW.1 (task 03's `hubapi/work.go`) field for
field in camelCase: `IssueHead`, `IssueRevision`, `ServesEntry`, `IssueState` (the same union as
task 28's, which it must be assignable to), `ThreadState`, `Relation`, `Label`, `TeamPolicy`,
`IssueComment`, `Subscription`, `StateMove`, `ImportResult`, plus task 24's `StartResult` (SW.10).
**`WorkClient`** has one method per relay path in task 16's table (`/v1/hub/work/*`, SW.2) and per
local route of tasks 24 and 25 (`start`, `undoStart`, `discardStart`, `importPreview`, `importRun`),
each calling the loopback control plane (`DEFAULT_BASE_URL`, `client.ts:30`), never the Hub (the
relay rule, contract §4.3), and mapping failures onto the built `ApiError` and `Problem`
(`client.ts:40-58`) with the new codes `first_write` and `refused`. **`workSlice`** holds view state
only, never a Hub record: `work: { projectId: string | null; branch: string | null; openIssue: {
id: string; revision: number | null } | null; selection: string[]; cursor: string | null }` and
`openWork`, `openIssue`, `select`, `moveCursor`; Hub records come from the replica (SW.16, tasks
55-58), and until then task 38 reads task 20's fixtures. **`Lens` gains `"work"`** but the sub-rail's
`LENSES` (`components/subrail/SubRail.tsx:9-16`) stays six: Work is not a seventh tab, because tabs
read as stages (05 §4.1, 07 D M7); it opens from `G W` (task 54), Plan and every issue chip. The
persisted `lens` (`store.ts:46`) may be `"work"`; it is a preference, not Hub data. **Tests first:**
`it("decodes and re-encodes each golden vector")` (task 03's `hubapi/testdata` JSON; kills a field
renamed on the desktop); a type test that `IssueState` equals task 28's union (kills drift between
the glyph and the wire); `it("calls only the loopback relay")` (a fetch spy sees each method and path;
kills a call straight to the Hub); `it("maps first_write to its own problem")` (kills a generic
error where *Stop and discard* must be offered); `it("persists nothing of workSlice")` (`partialize`
output has no `work` key; kills Hub-derived state on disk, ADR-H1); `it("keeps six lens tabs")`
(kills Work as a seventh tab); `it("restores a persisted work lens")` (kills a `migrate` that throws
on the new member). **Acceptance:** `npm --prefix desktop test -- work store SubRail` passes;
`npm --prefix desktop run typecheck` is clean. It does **not** claim the replica (55-58), any
rendered surface (38-48), or that the types match a running Hub (task 17 does). Design: 05 §4.1
(Work is a lens, not a tab), §3.1 (three homes: the desktop keeps no Hub data); `ADR-H1:467-474`.
Parity: with tasks 55-58 it serves **L-228** (the in-memory replica); it closes no row alone.

