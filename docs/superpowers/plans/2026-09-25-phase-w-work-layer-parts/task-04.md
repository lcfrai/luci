### Task 04: `hubclient.Work` and the fake, part 1: issues, revisions, numbers, the project key, states

**Slice:** W1. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 03; Hub tasks H04-H05 (the interfaces, `Error`, `ErrUnavailable`, `NewFake()` and
its feed), sync-merged into `feat/work-layer` first (rule H1). **Seams:** defines **SW.2**, first
half: the whole W1 method set of `hubclient.Work`, and the fake's issue rules. SW.2 freezes at WS0
with task 05; after that, a change that is not an added method, field or route reopens task 05
and is announced (skeleton, *Start lines*).

Every Work-backed handler in the control plane (tasks 19, 21-25), the relay (task 16) and the
desktop's contract tests (task 37) are written against one Go interface and one in-memory Hub.
If the interface lagged the routes, each stream would invent its own method; if the fake's rules
were looser than the Hub's, a handler green on the fake would fail on the server. So this task
fixes the **whole** W1 method set now, and gives the fake part 1's rules exactly as tasks 08-10
build them on the server. Task 05 adds the rest of the rules and the conformance rows that hold
the fake and the Hub to one standard.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`** (under
`control-plane/internal/hub/hubclient/`): `hubclient.go:52-64` (`Briefs`, the style: `ctx` first,
`hubapi` types in and out), `:63` (`Briefs.Resolve(ctx, keys)`), `:101-109` (`type Hub interface`),
`:111-121` (`Error`), `:146-149` (`_ Hub = (*Fake)(nil)`); `fake.go:56-71` (`fakeWorld`, one
`mu`), `:74-86` (`NewFake`), `:139-150` (`enter`), `:172-178` (`stamp`, `next`), `:184-197`
(`problem`, `badRequest`), `:200-223` (`forbidden`, `need`), `:239-262` (`pageLimit`, `pageOf`),
`:269-276` (`notInFakeYet`, 501 `not_in_fake_yet`); `fake_projects.go:42-48` (`fakeProject`),
`:72-79` (`projectOf`: the caller's role, or 404 `project_not_found`), `:81-84` (`canRead`),
`:108` (`userRef`). Under `hubapi/`: `canonical.go:282-296` (`FieldError{Field, Err}`),
`keys.go:32` (`ValidID`), `:110-116` (`Digest`, `DigestOf`), `envelope.go:394` (`FormatTime`),
`types.go:565-567` (`EraseRequest{Reason}`). Task 02's `IssueRevisionEnvelope`,
`IssueRevisionDigest`, `IssueKey`, `ValidProjectKey`, `DisplayAlias`, `PrefixIssue`; task 03's
`work.go` (every W1 record, request and enum, including `AttachRequest`, `IssueAttachment` and
`WorkEraseResponse`, which task 03 declared for this task).

**Not yet on any branch, so UNVERIFIED until merge:** Hub task H05's `fake_feed.go` (the feed
the fake appends to; its part, `phase-h-hub-parts/task-05.md:162-165`, fixes the behaviour, *"every
write … appends one event per changed record"*, and names no helper), and task 13's
`hubapi/workevents.go` (the Work event kinds and their data). Where this part names either, it
says so, and Step 1 takes the merged names.

Seven decisions the scope left open, made here.

**(a) `ResolveComment`, not `Resolve`.** The scope lists a `WorkIssues.Resolve`. `Hub` already
embeds `Briefs.Resolve(ctx, keys []string)` (`hubclient.go:63`), and Go refuses an interface that
embeds two methods of one name with different signatures, so `type Hub interface` would not
compile. The comment method is `ResolveComment(ctx, commentID, hubapi.ResolveRequest)`. Task 16's
part already uses this name (`task-16.md:936-937`).

**(b) `Attachments` is a method.** Task 07's route list has `GET /v1/issues/{issueId}/attachments`
and the scope's method list has no reader for it (task 16's gap 2). The set gains
`Attachments(ctx, issueID) ([]hubapi.IssueAttachment, error)`: task 03's type, not the
`hubapi.Attachment` task 16's part drafted, which exists nowhere.

**(c) Erase takes the Hub's own body and answers task 03's type.** `EraseIssue(ctx, issueID,
hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)` and `EraseComment(ctx, commentID,
hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)`: a reason is kept for every erase, as the
brief erase keeps one (`types.go:565`). Task 16's part drafted a bodyless erase answering a head;
it takes these at its Step 2 (`task-16.md:887`).

**(d) The stub has its own 501.** `fake.go:264-276`'s `notInFakeYet` is Hub part 1's placeholder
section, which Hub task H05 deletes. `fake_work_stub.go` declares `workNotInFakeYet` beside its
methods with the same answer, 501 `not_in_fake_yet`, so the file compiles whichever side of H05
it lands on and task 05 deletes it whole. The scope's `not_implemented` code would be a second
spelling of one idea; the fake's is kept.

**(e) The event kinds are task 13's (SW.5).** The scope drafted `issue.created`,
`issue.changed`, `issue.revised`, `issue.state`. Task 13's registry (`task-13.md:122-139`) spells
the head write `issue.updated` and adds `project.key_set`; this part uses task 13's five W1
issue kinds by constant, and their `data` shapes. Every fake write reaches the feed through one
function, `w.workEvent`, so Step 1's rename of H05's helper is a one-line edit.
Task 13 comes after this task, so the fake spells the kinds as its own unexported constants with
task 13's exact strings; task 05's conformance rows assert the kind strings, and task 16 runs
those rows against the real Hub, so a drift on either side fails a row.

**(f) The codes are the later tasks', spelled here as the fake's own constants.** Tasks 08, 09
and 10 declare `hubapi.CodeIssueNotFound`, `CodeProjectKeyTaken`, the revision conflict and
`CodeStateNotAllowed`, `CodeStateConflict`, `CodeEvidenceNotApplicable`. Declaring them in
`hubapi` here would make two declarations at their merge, so `fake_work.go` has unexported
`fk…` constants with the same strings (`task-08.md:171-172`, `task-09.md:113`,
`task-10.md:242-244`), and task 05's conformance rows assert the strings.

**(g) The move table is task 10's, drafted as the fake's own function until task 10 lands.**
Task 10 declares `hubapi.MoveAllowed(from, to, cause)` and says the fake calls it
(`task-10.md:20-22`, `:264-287`). This task comes first, so `fakeMoveAllowed` in `fake_work.go` is
that table line for line, over task 03's `MoveCause` names. Task 10's Step adds one edit to this
file: the body becomes `return hubapi.MoveAllowed(from, to, cause) == nil`. The evidence rule is
task 10's decisions 3-6 over what the fake can see: each cited record exists in the same project,
names a served `(briefKey, criterionId)`, is `settled`, was recorded at or after `landedAt`, and
has `commit` equal to the land commit. Task 03's `StateMove` has no `evidenceCommit` (task 10 adds
it), so here only the land commit counts.

**(h) Numbers are taken after every check.** A create that is refused, for any reason, consumes
no number, so *gapless* holds for refusals as well as for races: the counter moves in the same
locked section as the insert, after the last check.

**Files** (under `control-plane/internal/hub/hubclient/`):

| Path | Change |
|---|---|
| `work.go` | new: `type Work interface { WorkIssues }` and every W1 method of `WorkIssues` |
| `hubclient.go` ⚑ | one line: `type Hub interface` gains `Work` (the only edit to a Hub file) |
| `fake_work.go` | new: the fake's issue state, part 1's rules, `fakeMoveAllowed`, `w.workEvent` |
| `fake_work_stub.go` | new: part 2's methods answer 501 `not_in_fake_yet`; task 05 deletes it |
| `fake_work_test.go` | new: part 1's rules, each test with its mutant |

`fakeWorld` (`fake.go:56-71`) is Hub code, so it is **not** edited: the Work state hangs off one
pointer the fake creates on first use, `w.work`, declared in `fake_work.go` through a map keyed by
world (decision (i) below), so `fake.go` stays byte-identical.

**(i) The Work state lives beside the world, not in it.** `fake_work.go` declares
`var workWorlds sync.Map // *fakeWorld → *fakeWork` and `func (w *fakeWorld) wk() *fakeWork`,
which loads or stores one `*fakeWork` for that world. Every access is under `w.mu`, the fake's
one lock, like every other field; the `sync.Map` only finds the pointer. So no line of `fake.go`
or `NewFake` changes, H05's edits to `fake.go` cannot conflict with this task, and every view
(`As`) shares the one Work state because they share the world.

---

#### Interfaces, complete

`hubclient.go`'s one change, in `type Hub interface` (`hubclient.go:101-109`):

```go
type Hub interface {
	Projects
	Teams
	Briefs
	Designs
	Releases
	Evidence
	Feed
	Work // Phase W, task 04: the work layer (hubclient/work.go)
}
```

`var _ Hub = (*Fake)(nil)` (`hubclient.go:148`) then requires the fake to have every method below,
which `fake_work.go` (part 1) and `fake_work_stub.go` (part 2) give it. `var _ Hub = (*Client)(nil)`
(`:147`) requires the same of the HTTP client, which has none of them until task 16.

**(j) The client gets a stub file of its own.** `client_work_stub.go` gives `*Client` every Work
method, each answering `ErrNotConfigured` as H04's `client.go` stub does for every Hub method
(`client.go:11-16`, `:25-134`). It is its own file for the reason `client.go` is: H33 replaces
`client.go` file for file, and must not find Work methods in it. Task 16 deletes
`client_work_stub.go` in the same commit that adds `methods_work.go` (a gap for task 16's file
list, which does not name it).

| Path (continued) | Change |
|---|---|
| `client_work_stub.go` | new: `*Client`'s Work methods answer `ErrNotConfigured`; task 16 deletes it |
