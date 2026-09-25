# Phase W scopes: tasks 01 to 20 (W0, W1's Hub, the phase 4 deltas, the test kit)

One paragraph per task. A part writer reads its paragraph and the skeleton's row
(`2026-09-25-phase-w-work-layer.md`, *Task list*); where the two disagree, **the row's id,
dependsOn and seams win** and this paragraph is corrected. `§` means a section of
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2); `L-NN` is a row of
`01-linear-feature-inventory.md`, as 05 §1 prices it. A row listed as *closed* here is closed only
when the task's acceptance commands pass; a row closed jointly names the other tasks.

Paths are repository-relative. Hub server paths (`server/`, `store/`, `migrate/` under
`control-plane/internal/hub/`) are planned by the Hub plan and **UNVERIFIED until they land on
`feat/hub`**; sibling-plan paths are **UNVERIFIED until merge** (rules H1, H2). Every Go test runs
as `go test ./control-plane/... -race -count=1`, Hub integration rows with `-tags integration`
against `control-plane/cmd/hub/compose.yaml`; every desktop test as `npm --prefix desktop test`.

"Mutant" names the one wrong implementation a test exists to kill. Hub anchors below were read in
the `feat/hub` worktree at `48351f8` (`docs/superpowers/contracts/2026-09-23-hub-contract.md`,
cited `hub-contract:N`; the Hub plan `docs/superpowers/plans/2026-09-23-phase-h-hub.md`, cited
`H-plan:N`); a part re-reads them at its own sync base (rule H1).

---

**01 — W0 and the documents.** A documents-only task, W0 (05 §5.2, §5.5's *Hub*, *Docs*, *02* and
*`AGENTIC-UX.md`* rows), that defines seam **SW.0** and has no dependency. It writes one new file,
`docs/superpowers/planning/2026-09-25-linear-parity/w0-hub-contract-proposal.md`: a proposed edit
to the `feat/hub` owner (never a direct change, because the contract is under live edit) carrying
W0's two items. **Item 1** adds the `issue` case to hub-contract §3.2's grammar (`### 3.2` is at
hub-contract:114; the block the Hub mints and accepts sits under it), written out in full as
`issue key ^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$  e.g. issue:is-3f2a91c07b1e4d2a@3`, with
the rule that the revision number is the Hub's, gapless per issue from 1, 0 never in a key, and
that `EP-142` is a display alias and **never** a key (05 §3.6); it names the matching
`KeyIssue KeyKind = "issue"` and `issueKeyRe` it would add beside `briefKeyRe`/`designKeyRe` in
`hubapi/keys.go:47-58` and the `ParseKey` case at `keys.go:96-105` (task 02 builds them). **Item
2** replaces the stale `"criterionId": "done:2"` at hub-contract:1065 and :1079 with a `c-` id,
using one worked value (for example `c-91e2a0b3c4d5e6f7`), and states it is harmless whatever the
owner rules on Phase W. The proposal also lists, as *what the later WH tasks will ask*, the id
prefixes of 05 §3.6 (`is- ic- ft- ml- cy- in- rq- lb- tp- vw- lp- df-`, checked disjoint from
hub-contract §3.1's table and TM-05's `ag- tk- ls- cf- th- ms- es- nt- dl-`), the `Work` embed in
`type Hub interface` (hub-contract:1329) and new route rows after row 43 (hub-contract:605-609),
so the owner sees the whole ask once. The task then edits four documents: `docs/PROGRAM.md` gains
a **W** row in the *Revised phase order* table (`PROGRAM.md:54-65`, needs Azure: no; built
alongside team mode, `OD-25`) and a short Phase W section naming W0-W5 and the owner decisions
`OD-25` to `OD-28`; `docs/FUNCTIONAL-SPEC.md`'s object model (`FUNCTIONAL-SPEC.md:5-16`) gains
**Feature** (`ft-`, the spec is its branch brief) and **Issue** (`is-`, head and revision,
`serves`), each with its cardinality and "User sees the word? Yes"; `docs/AGENTIC-UX.md` gets
**Q13's four one-line amendments** (05 §6.1 row Q13, accepted under `OD-27`): (1) at anti-pattern
10 (`AGENTIC-UX.md:245`) plain group sizes for sets people committed to are allowed; (2)
anti-pattern 1 (`:236`) reads *"… a nameable transcript span, or the Hub record and revision it
shows"*; (3) §5 (`:85`) allows three-line cards for agents that changed since you looked and one
folded line for the steady; (4) anti-pattern 2 (`:237`) says a projection that names its rule and
source span is not an ETA; and `02-linear-design-language.md` gets **its three corrections** (05
§5.5 row *02*): C8's replica "on disk" becomes in memory (ADR-H1:467-474, Q5); C7's
`arrivals.hold` moves from `R` to `N`, `N` applies every held change, and `moves.hold` is added
beside it (D M10); C3's faces question is closed by `OD-28` (Commit Mono, OFL, bundled; Segoe UI
Variable, the system face, never bundled). Each amendment is a one-line insertion marked with its
decision id, never a rewrite. **Tests** are acceptance greps, each with the mutant it kills:
`grep -c "^issue key" …/w0-hub-contract-proposal.md` prints `1` (kills a proposal that names the
case but gives no grammar line); `grep -n "done:2"` finds the string only in the *before* half of
item 2 (kills a proposal that forgets one of the two sites); `grep -c "OD-27" docs/AGENTIC-UX.md`
prints at least `4` (kills an amendment added without its decision id); `grep -n "Tauri stores"
…/02-linear-design-language.md` returns nothing outside a struck *was:* note (kills a correction
that leaves the on-disk claim live); `grep -n "^| \*\*W\*\*" docs/PROGRAM.md` prints one row. It
**does not claim** that the Hub owner accepted the proposal, that any code parses `issue:` (task 02
and task 18 build it), or that the three `02` corrections change any token. Parity rows: it closes
none alone; it is the precondition for L-3 and L-34 (tasks 02, 08, 09). Note for the proposal:
`hubapi/types_test.go:173` and `:531` also carry `"criterionId":"done:2"`; item 2 names them so
the owner fixes the code examples with the contract's.

**02 — `hubapi`: Work id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision`
canonical kind and golden vectors.** W1, stream WH, seam **SW.1** (first half); depends on 01
(SW.0's grammar line) and Hub tasks H01-H03 (`ids.Wide`, `hubapi.Canonical`, the wire types), all
sync-merged from `feat/hub` first (rule H1). Files: `control-plane/internal/hub/hubapi/keys.go` ⚑
(Hub task 01's file, **additive only**), `keys_test.go` ⚑, and new `hubapi/workids.go`,
`workids_test.go`, `workcanonical.go`, `workcanonical_test.go` and
`hubapi/testdata/issue-revision/{v1,v2}.json`. In `keys.go` it appends `KeyIssue KeyKind =
"issue"` to the const block at `keys.go:49-52`, `issueKeyRe =
^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$` beside `briefKeyRe`/`designKeyRe` (`:56-58`), a
`type IssueKey struct { IssueID string; Revision int }` with `String()` and `ParseIssueKey(s)
(IssueKey, error)` shaped exactly like `BriefKey`/`ParseBriefKey` (`:62-77`), and one `case` in
`ParseKey` (`:96-105`). `workids.go` holds the prefix constants of 05 §3.6 (`PrefixIssue = "is"`,
`PrefixIssueComment = "ic"`, `"ft"`, `"ml"`, `"cy"`, `"in"`, `"rq"`, `"lb"`, `"tp"`, `"vw"`,
`"lp"`, `"df"`; `dc-` is reserved in a comment for L-120, *later*), each minted with
`ids.Wide(prefix)` and checked with the existing `ValidID(prefix, id)` (`keys.go:31-40`);
`ValidProjectKey(k string) bool` for `^[A-Z]{2,5}$` (04 §2.4: 2 to 5 capitals, set by a team
admin); and `DisplayAlias(projectKey string, number int) string` → `"EP-142"`, with a doc comment
that **no parser for it exists or may be written** (05 §3.6: an admin can change the key).
`workcanonical.go` gives `IssueRevisionEnvelope{V int "v"; Kind string "kind"; IssueID string
"issueId"; Revision int "revision"; ProjectID string "projectId"; Content IssueRevisionContent
"content"}` with `Kind` always `"issue-revision"`, and `IssueRevisionContent{Title AuthoredText
"title"; Description AuthoredText "description"; Serves []Serve "serves"; Implements []string
"implements"}` where `AuthoredText{Text string "text"; Author string "author"}` takes `"H"` or
`"A"` (05 §3.2, TM-05's `text·H`/`text·A`) and `Serve{BriefKey string "briefKey"; CriterionID
string "criterionId"}` (the types themselves are declared once, here, and reused by task 03);
`IssueRevisionDigest(env) (Digest, []byte, error)` calls `hubapi.Canonical` and `DigestOf`, the
one function the Hub writes with and `hubclient` verifies with (hub-contract §3.4: no TypeScript
implementation). `ValidateIssueRevisionContent` enforces title ≤ 200 and description ≤ 16 KiB (04
§2.4), `serves` and `implements` present as arrays (never `null`), each `briefKey` a valid brief key
and each `implements` entry a valid design key. The golden vectors follow hub-contract §3.5's
method (Python `json.dumps(sort_keys=True, separators=(",",":"), ensure_ascii=False)`, vector 1
also through `printf '%s' … | sha256sum`): **v1** an ASCII revision with one `serves` pair and an
empty `implements`; **v2** a title with `é` (U+00E9) and a description with a TAB and U+0001, an
`"A"` author and two `serves` pairs; each file holds the envelope, the exact canonical bytes and
the digest, and the part records the computed digests (never a placeholder). **Tests, each with
its mutant:** `ParseKey("issue:is-3f2a91c07b1e4d2a@3")` is `KeyIssue` and round-trips through
`ParseIssueKey`/`String` (kills a missing `case`); `@0`, `@01`, 15 hex, upper-case hex and
`issue:bf-…@1` are `ErrInvalidKey` (kills a loosened regex); `ParseKey(DisplayAlias("EP", 142))`
is `ErrInvalidKey` (kills an alias accepted as a key); a disjointness test lists hub-contract
§3.1's prefixes, the control plane's `s- p- b- w- m-`, TM-05's `ag- tk- ls- cf- th- ms- es- nt-
dl-` and the twelve new ones and fails on any repeat (kills `ms` reused for milestones);
`ValidProjectKey` rejects `E`, `EPICS1` and `ep` (kills a lower-case or 6-letter key);
both golden vectors reproduce byte for byte and digest for digest (kills a renamed or dropped
envelope field, a `kind` of `"issue"`, and `omitempty` on `serves`); a 201-character title and a
16 KiB + 1 description fail validation (kills an off-by-one limit). It **does not claim** that the
Hub stores or serves an issue (tasks 08-09), that `/v1/refs` resolves `issue:` (task 09), or that
phase 4's parser accepts it (task 18). Parity rows served: **L-3** (the alias; closed with 08 and
the desktop rows), **L-34** (immutable, digested revisions; closed with 09 and 46), **L-124**
(author class on every text; closed with 03, 12 and the issue view).

**03 — `hubapi`: the W1 wire types and request bodies.** W1, stream WH, seam **SW.1** (second
half; frozen at WS0 with task 05); depends on 02. One new file pair,
`control-plane/internal/hub/hubapi/work.go` and `work_test.go`, following `types.go`'s idiom
(camelCase JSON tags, `User` for every actor, RFC 3339 strings for times, `Page[T]` at
`types.go:196-205` for lists) and reusing 02's `AuthoredText`, `Serve` and `IssueRevisionContent`.
It declares, with complete Go in the part: the enums `IssueState` (`triage`, `backlog`, `todo`,
`started`, `in_review`, `landed`, `done`, `cancelled`, `duplicate`; 05 §3.3 — **no `shipped`**,
which is derived) with `FactStates` (`started`, `in_review`, `landed`, `done`, skeleton rule 2) and
`InitialStates` (`triage`, `backlog`, `todo`); `Priority` (`none`, `urgent`, `high`, `medium`,
`low`; L-13); `RelationKind` (`blocks`, `duplicates`, `relates`; L-24); `ThreadStateKind`
(`on_thread`, `project_level`, `not_tied`) and `NotTiedReason` (`chore`, `spike`, `imported`,
`support`; 05 §2.3 point 4); `IssueSource` (`person`, `orchestrator`, `intake`; 04 §2.4);
`SubscriptionCause` (`create`, `assign`, `mention`, `comment`, `manual`; L-117). Records:
`IssueHead{id, projectId, number, alias, state, stateReason, assignee *User, priority, labels
[]string, relations []Relation, headRevision, threadState ThreadState, source, external
*ExternalRef, createdAt, createdBy, updatedAt}` (`alias` is `DisplayAlias` at read time, never
stored or accepted; W2's estimate, cycle, feature, parent, due date and rank are added to it
additively by task 72); `IssueRevision{issueId, revision, key, digest, content, writtenAt,
writtenBy, restoredFrom *int}`; `ThreadState{kind, reason}`; `Relation{kind, issueId}`;
`ExternalRef{source, externalId, url}`; `Label{id lb-, teamId, name, color}` (a token name, never
a hex); `TeamPolicy{teamId, defaultState, priorityToLeaveTriage bool, stopAtLandedWithoutServes
bool, updatedAt, updatedBy}` (L-9, L-48, 05 §3.3's team switch); `Comment{id ic-, issueId,
parentId *string, kind, body AuthoredText, anchor *CommentAnchor, mentions []string, reactions
[]Reaction, resolvedAt *string, resolvedBy *User, createdAt, createdBy}` with W1's only `kind`
`"comment"` (W3's `note`, `evidence_proposal`, `negotiation` are SW.19, task 99);
`CommentAnchor{kind text|criterion|span|hunk, revision int, start, end int, criterionId, ref}`
(`ref` a provenance key or span id; L-42); `Reaction{emoji, users []User}`;
`Subscription{issueId, user, cause, subscribed bool}`; `ProjectKey{projectId, key, setAt, setBy}`.
Request bodies: `CreateIssueRequest{title, description AuthoredText, serves, state, priority,
assigneeId, labels, notTiedReason}`, `PatchIssueRequest` (pointer fields: `priority`,
`assigneeId`, `labels`, `notTiedReason`; **no `state`**, which moves only through `StateMove`),
`WriteIssueRevisionRequest{baseRevision, content}` (409 on a stale base, as briefs,
hub-contract §7.5), `RestoreRevisionRequest{fromRevision}`, `StateMove{to, cause, reason}` (the
shape only; task 10 owns its causes and the done rule, **SW.4**), `RelationRequest`,
`CreateLabelRequest`, `PutTeamPolicyRequest`, `SetProjectKeyRequest{key}`,
`CreateCommentRequest{body, parentId, anchor, mentions}`, `ReactRequest{emoji, on bool}`,
`ResolveRequest{resolved bool}`, `SubscribeRequest{subscribed bool}`, `ImportBatch{source
linear|csv, items []ImportItem}` with `ImportItem{externalId, url, title, description, priority,
labels []string, createdAt}` and `ImportResult{created, existing []IssueRef}` (the shape only;
task 15 owns idempotency by `(source, externalId)`, **SW.6**), and `IssueSearchQuery{projectId, q,
limit}`. **Tests, each with its mutant**, in the four styles `types_test.go` already runs
(`TestFieldNamesMatchTheContractExamples` :179, `TestResponsesNeverOmitAField` :257,
`TestRequestsRejectUnknownFields` :390, the required-digest rows :574): a table of one JSON example
per record, decoded strictly and re-encoded equal (kills a tag like `criterion_id` or `issueID`);
`serves`, `labels`, `relations`, `mentions` and `reactions` encode as `[]` when empty (kills
`omitempty` on a list the desktop indexes); every request type rejects an unknown field (kills a
lenient decoder, and specifically a `"state":"done"` smuggled into `PatchIssueRequest`);
`ValidInitialState` refuses every fact state (kills an issue created already `done`, which would
skip the done rule); `IssueState` has no `shipped` member (kills 05 §3.3's derived fact stored as a
state); `AuthoredText` refuses an author other than `H` or `A`; `ThreadState` with `not_tied`
and no reason is invalid (kills 05 §2.3's "a silent open end"); a `:custom:` emoji is refused (kills
custom emoji, which L-41 leaves *later*). It **does not claim** any route, store or authorisation
(tasks 06-15), nor the W2-W5 types (72, 99, 114, 143). Parity rows served (each closed by the
route task and its desktop surface): L-8, L-9, L-10, L-13, L-17, L-24, L-38, L-41, L-42, L-46,
L-48, L-117, L-124, L-220.

**04 — `hubclient.Work` and the fake, part 1: issues, revisions, numbers, the project key,
states.** W1, stream WH, seam **SW.2** (first half); depends on 03 and Hub tasks H04-H05 (the
interfaces, `Error`, `ErrUnavailable`, `NewFake()` and its feed). Files: new
`control-plane/internal/hub/hubclient/work.go`, `fake_work.go`, `fake_work_stub.go`,
`fake_work_test.go`; and one line in `hubclient/hubclient.go` ⚑, where `type Hub interface`
(`hubclient.go:101`; hub-contract:1329 reads `{ Projects; Briefs; Designs; Releases; Evidence;
Feed }`) gains `Work` — the only edit to a Hub file. `work.go` declares `type Work interface {
WorkIssues }` (W2-W5 append `WorkPlan`, `WorkAgents`, `WorkRoadmap`, `WorkIntake`, tasks 72, 99,
114, 143) and the **whole** W1 method set on `WorkIssues`, in the style of `Briefs`
(`hubclient.go:52-64`: `ctx` first, `hubapi` types in and out): `SetProjectKey(ctx, projectID,
key) (hubapi.ProjectKey, error)`; `CreateIssue(ctx, projectID, req) (hubapi.IssueWithRevision,
error)`; `Issue(ctx, issueID)`; `IssueByNumber(ctx, projectID, number)`; `Issues(ctx, projectID,
q hubapi.IssueQuery, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueHead], error)`;
`PatchIssue`; `WriteIssueRevision`; `IssueRevisions`; `IssueRevisionAt(ctx, issueID, rev)`;
`RestoreIssueRevision`; `MoveIssueState(ctx, issueID, hubapi.StateMove) (hubapi.IssueHead,
error)`; then, implemented by task 05, `AddRelation`, `RemoveRelation`, `Labels`, `CreateLabel`,
`ArchiveLabel`, `TeamPolicy`, `PutTeamPolicy`, `Comments`, `CreateComment`, `React`, `Resolve`,
`Subscribe`, `Subscriptions`, `ImportBatch`, `SearchIssues`, `AttachBlob`, `EraseIssue`,
`EraseComment`. (`IssueWithRevision`, `IssueQuery` and `ProjectKey` are added to `hubapi/work.go`
here if task 03 did not declare them; additive only.) `fake_work.go` extends `fakeWorld`
(`fake.go:56-72`) with `issues`, `issueRevisions`, `projectKeys` and a per-project number counter,
behind the fake's one lock, and implements part 1's rules exactly as the real Hub will (task 08-10
re-run them): numbers **gapless per project from 1**; a project key only by a team `admin`
(`need(have, want)`, `fake.go:218`), `ValidProjectKey`, unique per team, and a changed key changes
every `alias` but no key; a create writes the head **and** revision 1 in one write; a
`PatchIssue` never writes a revision (04 §2.4: reprioritising all afternoon invalidates no agent's
context); a revision write with a stale `baseRevision` is 409 `revision_conflict` and writes
nothing; each revision's digest is `hubapi.IssueRevisionDigest` of what is stored; a restore writes
a **new** revision with `restoredFrom`; `MoveIssueState` follows 05 §3.3 as drafted for SW.4
(human moves among `triage`, `backlog`, `todo`, `cancelled`, `duplicate`; a fact state only with
a fact cause; a hand-set `done` on an issue whose revision serves criteria needs a non-empty
`reason`, kept as `stateReason`, and is recorded as hand-set so task 10's result word reads *done,
not proven*), and task 10 is where the real server fixes
SW.4 and any drift is corrected here; **every write appends exactly one event** to the fake's feed
(hub-contract:1131), with the Work kind names task 13 fixes (SW.5; drafted here as
`issue.created`, `issue.changed`, `issue.revised`, `issue.state`). `fake_work_stub.go` makes
part 2's methods return a 501 `not_implemented` problem so the package compiles; task 05 deletes
the file. **Tests** (`fake_work_test.go`), each with its mutant: 20 goroutines create in one project
and get numbers 1 to 20 with no gap or repeat (kills a counter read outside the lock); a `viewer`
creating is 403 and a `member` setting the key is 403 (kills a missing role check); after
`SetProjectKey("EP"→"PAY")` issue 142's `alias` is `PAY-142` and its revision keys are unchanged
(kills a stored alias); a stale-base write is 409 and `IssueRevisions` still lists one revision
(kills last-writer-wins); `PatchIssue` of priority leaves `headRevision` at 1 (kills a revision per
head write); the stored digest equals a fresh `IssueRevisionDigest` (kills a digest over non-canonical
bytes); a reasonless hand-set `done` on an issue with `serves` is refused with `details.field =
"reason"` (kills the silent done); a create appends exactly one event (kills head and revision as
two events). It **does not claim** the HTTP client (task 16), the real Hub (tasks 08-10) or part 2's
rules (task 05). Parity rows served: L-3, L-8, L-9, L-13, L-34, L-38 (closed by 08-10 and the
desktop).

**05 — the fake, part 2, and the Work conformance rows (WS0).** W1, stream WH, seam **SW.2**
(second half) and start line **WS0**: when this task is green, streams WP, WD1 and WD2 build every
Work-backed handler and surface against `hubclient.Work` and `NewFake()`, and a later change that
is not an added method, field or route reopens this task and is announced (skeleton *Start
lines*). Depends on 04. Files: new `hubclient/fake_work_social.go` (relations, labels, policies,
comments, subscriptions), `fake_work_intake.go` (import, search, attachments, erase),
`fake_work_social_test.go`, and `hubclient/conformance/work.go` with its rows; it **deletes**
`fake_work_stub.go`. The conformance file adds rows to the Hub's one table-driven suite
(`hubclient/conformance/conformance.go`, H-plan:279, built by Hub task 05, **UNVERIFIED until
merge**), so the same rows run against the fake now and against the real Hub in task 16. Rules the
fake implements, each also a conformance row named `W-NN`: **relations** `blocks`, `duplicates`,
`relates` between issues of readable projects, no self-relation, no duplicate pair, `A blocks B`
and `B blocks A` refused as a cycle of two; marking `duplicates` offers but never forces the
`duplicate` state (L-10); **team labels** `lb-`, unique name per team, an issue may carry only its
own team's labels (L-17); **team policy** `defaultState` applied on create when the request names
none (L-9), and `priorityToLeaveTriage` refusing a move out of `triage` with `priority: none`
(L-48, 422 `policy_refused` with `details.policy`); **comments** `ic-` are append-only (no edit
route; a correction is a new comment), `body.author` `H` or `A`, `mentions` only of team members,
an `anchor` checked against the issue revision it names (range within that revision's text, or a
criterion the revision serves), `React` idempotent per `(comment, user, emoji)`, `Resolve` on a
top-level comment only (L-41, L-42); **subscriptions** created on create, assign, mention and
comment with that `cause`, and an explicit unsubscribe is **not** undone by a later comment
(L-117); **import** lands every item in `triage` with `threadState {not_tied, imported}`,
`source: intake` and `external {source, externalId, url}`, idempotent by `(source, externalId)`
(a re-run returns the same ids under `existing`) — SW.6's semantics as drafted, fixed by task 15
(L-220); **search** matches titles of the caller's readable projects only, case-insensitive,
newest first, never descriptions or comments in W1 (L-106's Hub half); **attachments** name a
project blob by digest (hub-contract's blob routes, H-plan task 21) and are listed on the issue
(L-45, pulled into W1 by 05 §5.3 row 12); **erase** is org-admin only, removes title,
description, comment and import text, keeps ids, keys, digests and revision numbers, and every
later read of the erased text resolves `erased` (hub-contract:2254's H7 row). Every write here
also appends exactly one event. **Tests**, each with its mutant: a cross-team label on an issue is
refused (kills a label check by name only); moving out of `triage` with `priority: none` under the
policy is 422 and appends no event (kills a check after the write); editing a comment has no method
and `CreateComment` twice gives two ids (kills an upsert); a mention of a non-member is 400 naming
`mentions[0]` (kills leaking a name outside the team); unsubscribe, then comment, and
`Subscriptions` still shows `subscribed: false` (kills auto-resubscribe); the same import batch
twice yields one issue per `externalId` (kills a non-idempotent import); a search from a user of
team B never returns team A's title (kills search before the readability check); an erase by a team
`admin` is 403 and by an org admin leaves `IssueRevisionAt` returning the same digest with text
`erased` (kills deleting the row, which breaks every trailer); a `go vet`-clean build with
`fake_work_stub.go` absent (kills a stub left behind). Acceptance: `go test
./control-plane/internal/hub/hubclient/... -race -count=1` passes with the new rows listed by
`-run Conformance -v`. It **does not claim** that the real Hub passes these rows (tasks 16-17), or
any desktop surface. Parity rows served: L-9, L-10, L-17, L-24, L-41, L-42, L-45, L-46, L-48,
L-106, L-117, L-220.

**06 — migration, part 1: the Work tables, row-level security and grants.** W1, stream WH; no
seam; depends on 03 and Hub tasks H06-H07b (the migrate runner, `roles.sql`, the integration
harness and `sql/0001_init.sql`). One new file,
`control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql`, taking **the next free number at sync
time** (renumbered on a clash; rule H1), applied by `hub migrate` as `hub_owner` in one transaction
like `0001_init.sql` (hub-contract §10.1-10.2, `SET LOCAL search_path = hub` at :1466), and its
database test `migrate/work_w1_integration_test.go` (`-tags integration`). **No Hub table is
altered**: the project key and the number counter get their own tables, so no grant on
`projects` changes. The tables, each with `org_id text NOT NULL` first and a composite key that
leads with it: `project_keys (org_id, project_id PK, key, set_at, set_by)` with `UNIQUE (org_id,
team_id, key)`; `project_issue_counters (org_id, project_id PK, last_number)`; `issues (org_id,
id PK, project_id, team_id, number, state, state_reason, assignee_user_id, priority,
head_revision, not_tied_reason, source, external_source, external_id, external_url, created_at,
created_by, updated_at, erased_at)` with `UNIQUE (project_id, number)`, a partial `UNIQUE (org_id,
project_id, external_source, external_id) WHERE external_id IS NOT NULL` (SW.6's idempotency), and
`CHECK`s for the state, priority and reason sets of task 03; `issue_revisions (org_id, issue_id,
revision, project_id, canonical bytea, digest, written_at, written_by, restored_from, erased_at,
PRIMARY KEY (issue_id, revision))` — `canonical` is the stored `C`, byte for byte, as
`brief_revisions` (hub-contract §3.4); `issue_serves (org_id, issue_id, revision, brief_key,
criterion_id)`, an index derived from each revision's `serves` at write time, so the Work lens can
group by criterion without parsing canonical bytes; `labels (org_id, id PK, team_id, name, color,
archived_at, created_at, created_by)`; `issue_labels (org_id, issue_id, label_id)`;
`issue_relations (org_id, issue_id, other_issue_id, kind, created_at, created_by)`;
`team_work_policies (org_id, team_id PK, default_state, priority_to_leave_triage,
stop_at_landed_without_serves, updated_at, updated_by)`; `issue_comments (org_id, id PK, issue_id,
parent_id, kind, author_class, body, anchor jsonb, created_at, created_by, resolved_at,
resolved_by, erased_at)`; `comment_mentions`; `comment_reactions (org_id, comment_id, user_id,
emoji, PRIMARY KEY (comment_id, user_id, emoji))`; `issue_subscriptions (org_id, issue_id,
user_id, subscribed, cause, updated_at)`; `issue_attachments (org_id, issue_id, digest, added_at,
added_by)`. **Row-level security** repeats hub-contract §10.3's loop (:1905-1920) over exactly
these tables: `ENABLE ROW LEVEL SECURITY` and the `org_isolation` policy `TO hub_app USING (org_id
= current_setting('hub.org_id', true)) WITH CHECK (…)`. **Grants** follow §10.5 (:2001-2016):
`SELECT` and `INSERT` on every new table to `hub_app`; `UPDATE` column-limited — `issues (state,
state_reason, assignee_user_id, priority, head_revision, not_tied_reason, updated_at)`, never
`number`, `project_id`, `team_id`, `created_*` or `external_*`; `project_keys (key, set_at,
set_by)`; `project_issue_counters (last_number)`; `labels (name, color, archived_at)`;
`team_work_policies` (every column but the key); `issue_comments (resolved_at, resolved_by)`;
`issue_subscriptions (subscribed, cause, updated_at)`; `DELETE` only on `issue_labels`,
`issue_relations` and `comment_reactions` (the three toggles). **Tests**, each with its mutant: a
`pg_policies` query finds `org_isolation` on every table the migration creates, read from
`pg_tables` rather than a list (kills a table left out of the loop); with `hub.org_id` unset, a
`SELECT` from `issues` as `hub_app` returns zero rows (kills a permissive policy); as `hub_app`,
`UPDATE issues SET number = 9` fails with `insufficient_privilege` (kills a table-wide `UPDATE`
grant); `DELETE FROM issues` and `DELETE FROM issue_comments` fail (kills a `DELETE` grant beyond
the toggles); an `INSERT` with another org's `org_id` fails the `WITH CHECK` (kills `USING`
without `WITH CHECK`); two issues with the same `(project_id, number)` fail (kills a missing
unique); the migration applies twice through the runner with no second effect (kills a migration
the runner re-runs). It **does not claim** the guards (task 07), any route or store (08-15), or
that the number it took is final before merge. Parity rows served: none closed alone; it is the
storage behind L-3, L-8 to L-10, L-13, L-17, L-24, L-34, L-41, L-42, L-45, L-48, L-117, L-220.

**07 — migration, part 2: the guards, and the authorisation-matrix rows for every W1 Work
route.** W1, stream WH, seam **SW.3** (who may write what); depends on 06 and Hub task H16b (the
matrix harness, `server/matrix_integration_test.go`, H-plan:298). Files: the same
`NNNN_work_w1.sql` (appended, before it is ever applied anywhere shared) or a second file
`NNNN+1_work_w1_guards.sql` if 06 has been merged; `migrate/work_guards_integration_test.go`; and
`server/matrix_integration_test.go` ⚑ plus a new `server/work_matrix_rows.go`. **Guards**, in the
shape of hub-contract §10.2's `forbid_change`/`guard_revision` triggers (:1800-1836) and §10.4's
owner functions (:1934-1996): `issue_revisions`, `issue_serves`, `comment_mentions` and
`issue_attachments` are append-only (`hub.forbid_change()` on `UPDATE OR DELETE` and `TRUNCATE`);
`issue_revisions` changes once, by erasure, only inside `hub.erase_issue(p_issue text)`, a
`SECURITY DEFINER` function with a pinned `search_path` that checks the issue is in
`current_setting('hub.org_id')`, sets `canonical = NULL, erased_at = now()` on every revision and
blanks `issues.external_url`, keeping ids, digests and revision numbers (as `erase_brief`,
:1978-1995); `hub.erase_comment(p_comment text)` blanks `body` and `anchor` the same way; both
granted `EXECUTE` to `hub_app` only. **The fact-state guard** `guard_issue_state()`, `BEFORE UPDATE
OF state ON issues`: a move **into** `started`, `in_review`, `landed` or `done` is refused unless
the transaction set `hub.work_cause` (transaction-local, `set_config(…, true)`, set only by the
Work store in task 10) to a fact cause (`lane_bound`, `lane_queued`, `land_trailer`,
`evidence_observed`) **or** the move is `done` with cause `hand` and a non-empty `state_reason`;
and it is refused whatever the cause when `hub.work_actor_kind` is anything but `person`, so the
sync principal W5 adds (tasks 145, 150) can never write a fact state or `done` (05 §3.3, §3.6).
`number`, `project_id` and `created_by` are also frozen by the trigger (belt to 06's grants). The
two settings are **new names**, not edits to the Hub's pipeline; the Work store sets them after
the Hub's `hub.org_id` (hub-contract:1925) inside the same write transaction (Hub task 14).
**Matrix rows.** `work_matrix_rows.go` holds one row per W1 Work route, in the §5.3 columns
(`V M A O N X`, hub-contract:336-345): reads (`GET` issues, one issue, by number, revisions, a
revision, attachments, comments, labels, the work policy, search) ✓ for `V M A`, 404 for `O N
X`; issue and comment writes (create, patch, revise, restore, state, relations, attach, comment,
react, resolve, import) 403 for `V`, ✓ for `M A`, 404 for `O N X`; team settings (project key,
labels create and archive, the work policy) 403 for `V M`, ✓ for `A`; the subscription `PUT` ✓
for `V M A`; erase ✓ for `O` only (the brief erase's row). Each row names the task that registers
its route (08-15); the harness runs a row once `server.Patterns()` (H-plan:212) lists its pattern,
and a second test fails when a registered `/v1/…issues…`, `/v1/comments/…`, `/v1/labels/…` or
`…/work-policy` pattern has **no** row (kills a route shipped without its authorisation). The W1
route list, appended to hub-contract §7's table as rows **46 onward, at the next free number at
sync time**: `PUT /v1/projects/{projectId}/key` · `POST`, `GET /v1/projects/{projectId}/issues` ·
`GET /v1/projects/{projectId}/issues/by-number/{n}` · `GET`, `PATCH /v1/issues/{issueId}` ·
`POST`, `GET /v1/issues/{issueId}/revisions` · `GET /v1/issues/{issueId}/revisions/{rev}` · `POST
/v1/issues/{issueId}/revisions/{rev}/restore` · `POST /v1/issues/{issueId}/state` · `POST`, `GET
/v1/issues/{issueId}/attachments` · `POST /v1/issues/{issueId}/relations`, `DELETE
/v1/issues/{issueId}/relations/{kind}/{otherId}` · `GET`, `POST /v1/teams/{teamId}/labels` · `POST
/v1/labels/{labelId}/archive` · `GET`, `PUT /v1/teams/{teamId}/work-policy` · `GET`, `POST
/v1/issues/{issueId}/comments` · `POST /v1/comments/{commentId}/reactions` · `POST
/v1/comments/{commentId}/resolve` · `PUT /v1/issues/{issueId}/subscription` · `GET
/v1/me/subscriptions` · `GET /v1/issues/search?q=` · `POST /v1/projects/{projectId}/imports` ·
`POST /v1/issues/{issueId}/erase` · `POST /v1/comments/{commentId}/erase`; `/v1/refs` (row 27) and
`/v1/changes` (row 43) are extended, not added. **Tests**, each with its mutant: as `hub_app`,
`UPDATE issues SET state = 'done'` with no `hub.work_cause` raises (kills a guard on the Go side
only); with `hub.work_cause = 'hand'` and an empty `state_reason` it raises, with a reason it
passes (kills a reasonless hand-set done); with `hub.work_actor_kind = 'integration'` and cause
`evidence_observed` it raises (kills a sync token writing `done`, SW.3's pin); an `UPDATE` of
`issue_revisions.canonical` outside `erase_issue` raises (kills a mutable revision);
`erase_issue` on another org's issue raises `no_data_found` (kills an owner function that trusts
its argument); the no-row meta-test fails on a fixture route with no row. It **does not claim** that
the routes exist (08-15) or that Entra issues an integration principal (W5). Parity rows served:
none closed alone; it pins 05 §3.6's authorisation rows behind L-8 and L-239 (every write attributed
to a person).

**08 — issue heads, numbers and `project.key`: routes and store, one event per write.** W1,
stream WH, seam **SW.1** (the server side of the head); depends on 07 and Hub tasks H14 (the write
transaction: audit lock, hash chain, idempotency, hub-contract §10.6) and H17 (projects). Files:
new `control-plane/internal/hub/store/work_issues.go`, `server/work_issues.go` and
`server/work_issues_integration_test.go`; `server/matrix_integration_test.go` ⚑ goes live for
this task's rows (07's table); the route rows are appended to hub-contract §7's table in the WH
proposal, never edited into a Hub row. Routes: `PUT /v1/projects/{projectId}/key`
(`SetProjectKeyRequest` → `ProjectKey`); `POST /v1/projects/{projectId}/issues`
(`CreateIssueRequest` → `IssueWithRevision`, 201); `GET /v1/projects/{projectId}/issues?state=
&assignee=&label=&after=&limit=` (`Page[IssueHead]`, keyset by `number` descending, the Hub's
`PageRequest`); `GET /v1/projects/{projectId}/issues/by-number/{n}`; `GET /v1/issues/{issueId}`;
`PATCH /v1/issues/{issueId}` (`PatchIssueRequest`; no `state`). The store, in the one write
transaction: the number comes from `UPDATE project_issue_counters SET last_number = last_number
+ 1 … RETURNING` under the row lock (seeded on the project's first issue), **never a Postgres
sequence**, which leaves gaps on rollback; a create inserts the head, revision 1 (canonical bytes
from `hubapi.IssueRevisionDigest`), its `issue_serves` rows and **one** event; an assignee must be
a current member of the project's team and a `u-` id, never an agent (05 §3.6, `OD-16`); a patch
writes no revision; `alias` is computed on read from `project_keys`, and a project with no key yet
answers `alias: ""` so the desktop shows the number alone. **Events** (hub-contract §8 style,
metadata only, `data` never holds a title or description): `issue.created` `{"number", "key",
"digest", "state", "source"}`, `issue.updated` `{"fields": [names]}`, `project.key_set` `{"key",
"previousKey"}`. hub-contract §8 says *exactly one event per changed record* (:1133); this task
treats a new issue's head and first revision as one record, one event, and the part asks the Hub
owner to confirm it (**UNVERIFIED**). Task 13 fixes the Work kinds for the feed (SW.5); a rename
there is a find-and-replace in this file. **Tests** (integration, `-tags integration`, against
the compose stack), each with its mutant: 20 concurrent creates give 1 to 20 with no gap, and a
create that fails after numbering (a forced `revision_conflict` on a bad `serves` key) leaves the
next create at the next number (kills a sequence, and a counter bumped outside the transaction);
the same `Idempotency-Key` twice returns the same issue and one event (kills a double create on
retry); each write's events are counted in `audit_events` and are exactly one, and no event's
`data` contains the title string (kills content in the feed, hub-contract:1133-1134); an
assignee who is not a team member, or an `ag-…` id, is 400 naming `assigneeId` (kills an agent
as assignee); after the key moves `EP` → `PAY`, `GET …/by-number/142` and `GET /v1/issues/{id}`
both show `PAY-142` and the revision key is unchanged (kills a stored alias); a `PATCH` carrying
`"state"` is 400 (kills a state move that skips task 10's rule); the conformance rows of tasks 04
and 05 that touch these routes pass against the real Hub once task 16's client exists. Acceptance:
`go test ./control-plane/internal/hub/... -tags integration -race -count=1 -run
'WorkIssues|Matrix'` passes. It **does not claim** revisions, restore or `/v1/refs` (task 09),
state moves (10), or any desktop surface. Parity rows served: **L-2** (the team gains Work
settings, with 11), **L-3** (the key and the alias; closed with the desktop row, tasks 40 and
46), **L-13** and **L-38** (priority and the one human assignee; closed with the issue view and
the lens), **L-239** (every write's attributed event; the Hub's audit route already exists).

**09 — issue revisions, `serves`, restore and attachments; `/v1/refs` resolves `issue:`.** W1,
stream WH, seam **SW.1** (the revision's server side); depends on 08 and Hub tasks H19 (the batch
revision write and history), H20 (`/v1/refs`, brief half) and H21 (blobs). Files: new
`store/work_revisions.go`, `server/work_revisions.go`, `server/work_revisions_integration_test.go`;
`server/refs.go` ⚑ (Hub task 20's file, the path per the Hub plan's file map, **UNVERIFIED until
merge**) gains one `case hubapi.KeyIssue`; `hubapi/types.go` ⚑ gains, additively, `Issue
*IssueRevision "issue"` on `RefResult` (`types.go:322-328`, null for every other kind) and
`IssueHeads map[string]IssueHeadInfo "issueHeads"` on `RefsResponse` (`:336-339`, `{}` when none;
`IssueHeadInfo{headRevision, erased}`), so the brief `heads` map keeps its meaning. Routes: `POST
/v1/issues/{issueId}/revisions` (`WriteIssueRevisionRequest{baseRevision, content}`: 409
`revision_conflict` on a stale base, gapless revision numbers under two concurrent writers, as
hub-contract's H3 row :2250 for briefs); `GET /v1/issues/{issueId}/revisions`
(`Page[IssueRevision]` without `content`, newest first); `GET /v1/issues/{issueId}/revisions/{rev}`;
`POST /v1/issues/{issueId}/revisions/{rev}/restore` (writes a **new** revision whose content is
`rev`'s and whose `restoredFrom` is `rev`; L-34); `POST`/`GET /v1/issues/{issueId}/attachments`
(`{digest}` naming a blob already `PUT` to the project, hub-contract routes 28-29; the blob's
existence and project are checked; L-45). The store validates each revision with
`hubapi.ValidateIssueRevisionContent`, checks that every `serves.briefKey` resolves `resolved` in
the **same** project and that its `criterionId` is a criterion of that brief revision (via the
brief store's read, never by parsing the brief in the handler), writes `canonical` and `digest`
from `IssueRevisionDigest`, rewrites `issue_serves` for the new revision only, moves
`issues.head_revision`, and appends **one** event `issue.revised` `{"key", "revision", "digest",
"servesCount", "restoredFrom"}` (no text). `/v1/refs` answers an `issue:` key `resolved` (with the
revision and its content) when readable and its bytes re-hash to the stored digest; `missing` when
absent, unreadable or failing the re-hash (hub-contract:817-840's rules, unchanged); `erased`
with `content: null` after task 14's erase. **Tests**, each with its mutant: two writers on base 3
give one 201 and one 409, and revisions are 1..4 with no gap (kills an unlocked head read);
restoring `@2` at head `@5` creates `@6` equal in content to `@2` and leaves `@2`'s digest
untouched (kills an in-place restore); a `serves` naming a brief in another project, or a
criterion id absent from that brief revision, is 422 naming `content.serves[0]` (kills an issue
serving a criterion it cannot see, which would break the thread); `/v1/refs` with a mix of 100
`brief:`, `design:` and `issue:` keys answers each slot in order, and an `issue:` key of another
team's project is `missing`, not 403 (kills a readability leak, hub-contract T4 :2072); an
owner-role edit of one revision's `canonical` makes that key `missing` (kills a resolver that
trusts the stored digest without re-hashing); an attachment naming an unknown digest is 404 and one
from another project is 404 (kills attaching someone else's blob); `PATCH`ing priority 50 times
leaves `headRevision` unchanged (the agent's context stays valid; kills a revision per head write).
It **does not claim** phase 4's parser or `Ref.Kind` (task 18), the editor or the revisions UI
(tasks 46-50), or erase (14). Parity rows served: **L-34** (autosave history and restore, as
immutable digested revisions; closed with the issue view's `Revisions`, task 46), **L-45**
(attachments; closed with task 52), **L-124** (author class stored on every text), and the Hub half
of **L-191** (what the agent was told is recoverable byte for byte; closed with task 19).


