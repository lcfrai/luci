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

