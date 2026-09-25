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
the desktop rows), **L-34** (immutable, digested revisions; closed with 09 and 47), **L-124**
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
`evidence_observed`, `evidence_recorded`; task 10's complete set) **or** the move is `done` with cause `hand` and a non-empty `state_reason`;
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
immutable digested revisions; closed with the issue view's `Revisions`, task 47), **L-45**
(attachments; closed with task 52), **L-124** (author class stored on every text), and the Hub half
of **L-191** (what the agent was told is recoverable byte for byte; closed with task 19).

**10 — states, thread state and the done rule on the server.** W1, stream WH, seam **SW.4** (the
state move, the done rule's inputs and its result words; tasks 04's fake and 22-23's callers
conform to it); depends on 09 and Hub task H25 (evidence, hub-contract §7.10). Files: new
`store/work_states.go`, `server/work_states.go`, `server/work_states_integration_test.go`; a
small table `issue_proofs (org_id, issue_id, land_commit, landed_at, landed_by, evidence_ids
text[], proof, decided_at)` appended to the W1 migration (or the next number, rule H1);
`hubapi/work.go` gains, additively, `IssueHead.proof` and `IssueHead.landCommit`. Route: `POST
/v1/issues/{issueId}/state` with `StateMove{to, cause, reason, landCommit, evidence []string}`.
**Causes** (the complete W1 set): `hand` (a person's move among `triage`, `backlog`, `todo`,
`cancelled`, `duplicate`, a reopen, or a hand-set `done`); `lane_bound` → `started`; `lane_queued`
→ `in_review`; `land_trailer` → `landed`, carrying `landCommit` (task 22); `evidence_observed` and
`evidence_recorded` → `done`, carrying `evidence` ids (task 23). The store sets 07's
`hub.work_cause` and `hub.work_actor_kind = 'person'` before the update, so the database guard and
the Go rule agree. **The done rule** (05 §3.3, revision 2): an issue becomes `done` by evidence
only when it is `landed` with a recorded `landCommit`, and for **every** criterion its head
revision serves there is an evidence record in `evidence` that is in the same project, names that
`briefKey` and `criterionId`, has `verdict: settled`, was recorded at or after `landed_at`, and
has `commit` equal to `landCommit` — the Hub cannot test git ancestry, so a later commit is
accepted only when the move names it and phase 3's evaluator (task 23) checked the ancestry; that
check is the evaluator's claim, not the Hub's. Hub evidence carries no issue id today
(`EvidenceRecord`, `types.go:517-529`), so **the citation is the move's `evidence` list, recorded in
`issue_proofs`**: a sibling issue's move cannot reuse a record already cited by another issue for
the same criterion unless both issues landed in the same commit. **Result words** on
`IssueHead.proof`: `proven` (every record `evidence_observed`), `proven_self_attested` (a record
`evidence_recorded` by the person who created the issue **and** made its `started` move; observed
evidence is never self-attested), `not_proven` (a hand-set `done`, which needs a non-empty `reason`
when the revision serves criteria), `no_criteria` (empty `serves`: the `landed` move lands at
`done` in the same write, unless the team policy's `stopAtLandedWithoutServes` holds it at
`landed`), `waiting` (landed, some criterion without evidence), and `""` before `landed`. A reopen
clears `proof`. **Thread state** is computed on every head read from the head revision: `on_thread`
when it serves a criterion of a branch-scoped brief, `project_level` when it serves only the
project brief's criteria (brief `scope: project`), otherwise `not_tied` with the stored reason; a
`not_tied` issue with no reason is refused at create and at `serves` removal (05 §2.3 point 4).
Each move appends **one** `issue.state` event `{"from", "to", "cause", "proof", "landCommit"}`.
**Tests**, each with its mutant: evidence recorded before `landed_at` leaves the issue `landed`
(kills a rule without the time bound); a sibling issue's evidence for the same criterion, at a
different commit, is 422 (kills matching evidence by criterion alone); two served criteria with
evidence for one stays `landed` with `proof: waiting` (kills *any* criterion instead of *every*);
`to: started` with cause `hand` is 422 (kills a hand-faked fact); a hand-set `done` with a reason
reads `not_proven` and without one is 400 naming `reason`; the same person creates, starts and
presses Record → `proven_self_attested`, while the same person with `evidence_observed` →
`proven` (kills marking observed evidence as self-attested); an empty-`serves` issue reaches
`done` at `landed`, and with the team switch stays `landed` (kills the switch ignored); an issue
serving a project-brief criterion reads `project_level` (kills scope ignored). It **does not
claim** that a test run was watched (task 23), that the lane facts arrive (22), custom states (W2)
or parent roll-up (L-21, W2). Parity rows served: **L-8** (fixed fact states; closed with 22 and
the glyphs, task 28), **L-10** (the `duplicate` state), and the Hub half of 05 §2.3's *done means
proven*, which no L row names alone; it underlies **L-146** and **L-201** (closed with 21-22).

**11 — relations, team labels, `triage` and team policies.** W1, stream WH; no seam; depends on
08. Files: new pairs `store/work_relations.go` + `server/work_relations.go`,
`store/work_labels.go` + `server/work_labels.go`, `store/work_policies.go` +
`server/work_policies.go`, and `server/work_social_integration_test.go`; it touches 08's create
path (the default state) and 10's move path (the triage exit) in this plan's own files, in task
order. Routes (07's list): `POST /v1/issues/{issueId}/relations` (`RelationRequest{kind,
issueId}`) and `DELETE /v1/issues/{issueId}/relations/{kind}/{otherId}`; `GET`/`POST
/v1/teams/{teamId}/labels` (`CreateLabelRequest{name, color}`), `POST /v1/labels/{labelId}/archive`;
`GET`/`PUT /v1/teams/{teamId}/work-policy` (`PutTeamPolicyRequest` → `TeamPolicy`). **Rules:** a
relation joins two issues of projects **in the same team** (a project's access is exactly its
team's membership, hub-contract:333), never an issue to itself, never the same `(kind, pair)`
twice (a repeat is 200 with the existing relation and **no** event), and `A blocks B` refuses `B
blocks A`; `relates` and `duplicates` are stored once and read from both ends, `blocks` reads as
`blocked_by` from the other end; marking `duplicates` never moves the state by itself (the triage
`2` key does both, as two writes, task 42). When the other issue's assignee is not the writer,
the event carries `otherAssigneeId` so the Desk can show that owner an *inform* in solo mode (05
§3.7 rule 3; W3 turns it into an Orchestrator proposal, task 99 onward). Labels are team-owned
`lb-`, names unique per team case-insensitively, `color` a palette token name from
`docs/DESIGN-SYSTEM.md` (never a hex), archived and never deleted, and an issue carries only its own
team's unarchived labels (`PatchIssueRequest.labels`, validated here). The team policy
(`TeamPolicy`, absent row = defaults `defaultState: backlog`, `priorityToLeaveTriage: false`,
`stopAtLandedWithoutServes: false`) is read by 08's create when the request names no state (L-9;
intake and import always use `triage`, whatever the policy) and by 10's move: leaving `triage` for
any state but `cancelled` or `duplicate` with `priority: none` under `priorityToLeaveTriage` is 422
`policy_refused` with `details.policy = "priorityToLeaveTriage"` (L-48). **Triage verbs** on the
server: *accept* is a `hand` move out of `triage`; *decline* is `hand` → `cancelled` with a
`reason`; *snooze* adds, additively, `IssueHead.snoozedUntil` and a `PatchIssueRequest.snoozedUntil`
(an RFC 3339 time; a snoozed issue stays in `triage` and is listed after the rest, never hidden,
because deferred items age visibly, AX §7.4). Events: `issue.related`, `issue.unrelated` `{"kind",
"otherIssueId", "otherAssigneeId"}`, `label.created`, `label.archived` `{"name"}` (a label name is
team metadata, not customer text — the part confirms against hub-contract §8's rule, **UNVERIFIED**),
`team.work_policy_set` `{"fields"}`. **Tests**, each with its mutant: a relation to an issue in
another team's project is 404 (kills a relation that leaks another team's issue id); the same
`blocks` twice gives one row and one event (kills a double event); `B blocks A` after `A blocks
B` is 422 (kills a two-cycle); `GET` of B shows `blocked_by A` (kills a one-sided read); a label
`Bug` then `bug` is 409 (kills case-sensitive uniqueness); an issue given another team's label or
an archived label is 422 naming `labels[0]`; under the policy, `triage → todo` with `priority:
none` is 422 and appends no event, `triage → cancelled` passes (kills a policy that blocks
declining); a create with no state lands in the policy's `defaultState`, an import lands in
`triage` (kills intake using the default); a `member` `PUT`ting the policy is 403 (07's row). It
**does not claim** label groups or org labels (L-18, L-19, W2), custom states (W2), or the triage
UI (task 42). Parity rows served: **L-2**, **L-9**, **L-10**, **L-17**, **L-24** (blocks,
related, duplicate; the Desk cost-of-waiting half closes with task 63), **L-46** and **L-47**
(the server half; closed with task 42), **L-48**.

**12 — comments `ic-`: mentions, reactions, anchors, resolve.** W1, stream WH; no seam of its own
(it fills SW.1's `Comment`; W3's comment kinds are SW.19, task 99); depends on 08. Files: new
`store/work_comments.go`, `server/work_comments.go`,
`server/work_comments_integration_test.go`. Routes (07's list): `GET`/`POST
/v1/issues/{issueId}/comments` (`Page[Comment]`, oldest first; `CreateCommentRequest{body, parentId,
anchor, mentions}`); `POST /v1/comments/{commentId}/reactions` (`ReactRequest{emoji, on}`); `POST
/v1/comments/{commentId}/resolve` (`ResolveRequest{resolved}`). **There is no edit or delete
route**: a comment is append-only (05 §3.2), a correction is a new comment, and removal is task 14's
erase. **Rules:** `body` is `AuthoredText`, ≤ 16 KiB, `kind` is `comment` in W1 (a `note`,
`evidence_proposal` or `negotiation` is 400 until task 99 adds them); `author: "A"` is accepted
only from the person's own principal (an agent draft the person posts), and W3 adds the
Orchestrator path; a reply's `parentId` names a **top-level** comment on the same issue (one level
of thread, L-41); `anchor` is checked against the record it names — `text` needs `revision` to
exist on this issue and `0 ≤ start < end ≤` that revision's description length in UTF-16 code
units (the unit the editor reports, task 49; **UNVERIFIED** until 49 fixes it), `criterion` needs a
`criterionId` that revision serves, `span` and `hunk` carry a `ref` (a transcript span id or `commit
path:lines`) stored verbatim and resolved by the desktop, never parsed by the Hub (L-42; 05 §3.2);
`mentions` are `u-` ids of current team members (L-41's mentions; a subscription follows in task
13); reactions are idempotent per `(comment, user, emoji)`, `emoji` one Unicode emoji sequence of at
most 32 bytes, and a `:shortcode:` is refused (custom emoji is *later*, L-41 PARTIAL); resolve
applies to top-level comments only and records `resolvedBy`. **Events**, one per changed record
(hub-contract:1133), values only strings, integers and booleans (hub-contract §8, the line after
its table): `comment.created` `{"issueId", "parentId", "kind", "author", "anchorKind"}`, one
`comment.mentioned` per mention row with `subject` the mentioned user and `{"commentId",
"issueId"}` (so task 13 and the Desk can route it without an array in `data`),
`comment.reacted` `{"emoji", "on"}`, `comment.resolved` `{"resolved"}` — never the body. **Tests**,
each with its mutant: the route table has no `PATCH` or `DELETE` on `/v1/comments/…` (kills an
editable comment, read from `server.Patterns()`); a reply to a reply is 422 naming `parentId`; an
anchor with `end` past the revision's text or a `criterionId` the revision does not serve is 422
(kills an unchecked anchor that would point at nothing); a mention of a non-member is 400 naming
`mentions[0]` and a comment with two mentions appends three events (kills one event for the batch,
or a mention without its event); reacting twice `on` keeps one row and one event (kills a
non-idempotent toggle); `:partyparrot:` is 400; resolving a reply is 422; no event's `data` contains
the body string. It **does not claim** the comment UI (task 51), subscriptions (13), erase (14) or
W3's kinds (99). Parity rows served: **L-41** (threads, resolve, reactions; custom emoji stays
*later*; closed with task 51), **L-42** (anchors to text, a criterion, a span or a hunk; closed with
51 and 60), **L-124** (author class on every comment).

**13 — subscriptions; the change feed's Work kinds, filtered per caller.** W1, stream WH, seam
**SW.5** (the Work feed kinds and their metadata-only payloads, consumed by the replica 55, the
Desk 62, Fleet 98, W4's handover 123 and W5's webhooks 156); depends on 11, 12 and Hub task H26 (the
change feed, hub-contract §7.11). Files: new `store/work_subscriptions.go`,
`server/work_subscriptions.go`, `hubapi/workevents.go` (the kind constants and one `data` struct per
kind, so the Hub writes and the desktop's types, task 37, read one list), and
`server/work_feed_integration_test.go`; `server/changes.go` ⚑ (Hub task 26's file, **UNVERIFIED
until merge**) gains one additive rule. Routes (07's list): `PUT /v1/issues/{issueId}/subscription`
(`SubscribeRequest{subscribed}` → `Subscription`) and `GET /v1/me/subscriptions`
(`Page[Subscription]`). **Subscription rules** (L-117, pulled into W1 by 05 §5.3 row 4): the
creator on create, the assignee on assign, each mentioned user on mention and the commenter on
comment are subscribed in the **same** transaction as the triggering write, with that `cause`; an
explicit `subscribed: false` is kept and no later trigger turns it back on; a person leaving the
team keeps the row and stops receiving (readability already filters). **SW.5, the W1 kinds**, every
one with non-null `teamId` and `projectId` and `data` values that are only strings, integers or
booleans (hub-contract §8): `issue.created` `{number, key, digest, state, source}` ·
`issue.updated` `{fields}` (a comma-joined list of field names, since `data` carries no arrays) ·
`issue.revised` `{key, revision, digest, servesCount, restoredFrom}` · `issue.state` `{from, to,
cause, proof, landCommit}` · `issue.related` / `issue.unrelated` `{kind, otherIssueId,
otherAssigneeId}` · `issue.attached` `{digest}` · `issue.erased` `{erasedRevisions}` ·
`project.key_set` `{key, previousKey}` · `label.created` / `label.archived` `{name}` ·
`team.work_policy_set` `{fields}` · `comment.created` `{issueId, parentId, kind, author,
anchorKind}` · `comment.mentioned` (subject the user) `{commentId, issueId}` · `comment.reacted`
`{emoji, on}` · `comment.resolved` `{resolved}` · `comment.erased` `{}` · `subscription.set`
(subject the user) `{issueId, subscribed, cause}`. Tasks 08-12 used these names as drafts; this
task makes them constants and fixes any drift in those files. **Per caller:** hub-contract §7.11
already shows an event only to readers of its project or team; this task adds that
`subscription.set` (and, from W2, every draft kind, `df-`) is shown **only to its subject**, the
one per-caller rule, marked on the kind in `workevents.go` as `Visibility: SubjectOnly`. **Tests**,
each with its mutant: a canary string written as a title, description, comment body and import
text across every W1 route never appears in any `/v1/changes` page (kills content in the feed,
hub-contract:1133-1134); every Work event in a full run has non-null `teamId` and `projectId`
(kills an event that nobody, or everybody, sees); a reflection test over `workevents.go` refuses
any `data` field that is not a string, integer or boolean (kills an array in `data`); user B's feed
never shows user A's `subscription.set` while it does show A's `comment.created` on the same issue
(kills a private kind in the team feed); unsubscribe, then a mention of that user, and
`GET /v1/me/subscriptions` still reads `subscribed: false` (kills a trigger overriding the choice);
assigning C subscribes C with `cause: assign` in the same transaction (kills a subscription written
after commit, which a crash would lose). It **does not claim** the Desk digest or any notification
delivery (W2, tasks 81 onward; AX anti-pattern 8), the replica (55) or webhooks (156). Parity rows
served: **L-117** (automatic and manual subscriptions; closed with task 52), **L-229** (the delta
sync log gains the Work kinds; closed with the replica, task 55), and the metadata-only base of
**L-251** (W5).

**14 — erase for issues and comments; title search.** W1, stream WH; no seam; depends on 12 and
Hub task H27 (the audit route, erasure, the full `hub verify`). Files: `server/erase.go` ⚑ (Hub
task 27's file, **UNVERIFIED until merge**; two routes added beside `POST
/v1/briefs/{briefId}/erase`, never a change to it); new `store/work_erase.go`,
`store/work_search.go`, `server/work_search.go`, `server/work_erase_integration_test.go`; and one
additive column in this plan's W1 migration, `issues.head_title text` (the head revision's title,
kept in step by task 09's revision write, the only text outside `canonical`), with 07's
`hub.erase_issue` extended to blank it. **Erase** follows hub-contract §7.12's brief erase
(:1115-1127) exactly: `POST /v1/issues/{issueId}/erase` and `POST /v1/comments/{commentId}/erase`,
body `{"reason"}` 1-500 characters, org admin only (07's rows: 403 for every team role), 404
`issue_not_found` / `comment_not_found` outside the admin's org; in one transaction through the
owner functions of task 07 — an issue's erase removes every revision's `canonical`, `head_title`,
`external_url`, every comment body and anchor on it, and the import's source text if any; **keys,
digests, revision numbers, numbers, `writtenAt`/`writtenBy` stay**, so every `Panacea-Issue`
trailer, chip and `issue:` key still resolves, as `erased` (05 §3.6, 04 §5.5); a comment's erase does
the same for one comment. → `200 {"issueId", "erasedRevisions", "erasedComments", "erasedAt"}`; a
second erase answers zeros. Events `issue.erased` / `comment.erased` in the feed **without**
`reason`; the audit keeps `reason` (as `brief.erased`, hub-contract §8). Attached blobs are not
deleted here: whether the Hub deletes a project blob on erase is the Hub plan's (**UNVERIFIED**),
and the part names it as open. **Search**: `GET /v1/issues/search?q=&projectId=&limit=` →
`Page[IssueHead]`, `q` 1-200 characters, a case-insensitive substring match on `head_title` over the
caller's **readable** projects only (optionally one), erased issues excluded, newest first, at most
50; a `q` that equals a current display alias (`EP-142`) returns that issue first, resolved through
`project_keys` at query time and never stored or treated as a key; W1 matches titles only, never
descriptions or comments (L-106's Hub half; the desktop's `/` searches the replica, task 58). The
match is `ILIKE` with no index; its speed at 10,000 issues is **unmeasured**, and a trigram index is
a later task if a measurement asks for it. **Tests**, each with its mutant: a team `admin` erasing is
403 and an org admin of another org gets 404 (kills a role or org check left out); after an erase
`/v1/refs` answers every revision key `erased` with the same digest, and `GET` of the issue shows
number and key intact (kills deleting rows, which orphans trailers); a canary title, description,
comment and import URL are absent from **every** text column of every Work table after the erase,
read from `information_schema` rather than a list (kills a denormalised copy such as `head_title`
left behind); the feed's `issue.erased` has no `reason` while `/v1/audit` does (kills the reason in
the feed); `hub verify` passes after the erase (kills an erase that breaks the audit chain); search
by a user of team B never returns team A's title (kills search before readability), an erased issue
never matches, and `EP-142` returns issue 142 first. It **does not claim** request erase (W5, task
146), blob deletion, or the replica's erase handling (task 57). Parity rows served: **L-106** (the
Hub half of search and jump; closed with task 58), **L-239** (the erase is audited with its reason);
05 §3.6's erase has no L row of its own.

**15 — the import route: a normalised batch into `triage`, *not tied · imported*.** W1, stream
WH, seam **SW.6** (the import batch: one normalised item per external issue, idempotent by
`(source, externalId)`; consumed by task 25's adapters, task 69's dialog and W5's sync, 150-153);
depends on 11 (labels and the policy it must override). Files: new `store/work_imports.go`,
`server/work_imports.go`, `server/work_imports_integration_test.go`; `hubapi/work.go` gains,
additively, `ImportResult.unmatchedLabels []string` and `ImportItem.authorName` if task 03 did not
declare them. Route: `POST /v1/projects/{projectId}/imports` (`ImportBatch{source, items}` →
`ImportResult{created []IssueRef, existing []IssueRef, unmatchedLabels []string}`), member role
(07's row). **SW.6, fixed here:** `source` is `linear` or `csv` in W1 (W5 adds `jira`,
`azure_boards`, `github`); 1 to 500 items per call (the control plane pages, task 25); each
`ImportItem{externalId (1-200 chars), url (https or empty), title (≤ 200), description (≤ 16 KiB),
priority (hubapi.Priority, already mapped by the adapter), labels []string (names), authorName,
createdAt}`; idempotency is per project by `(source, externalId)`, enforced by 06's partial unique
index, so a re-run returns the same issue ids under `existing` and writes nothing for them. **What an
import creates:** every new issue lands in **`triage`** whatever the team's `defaultState` (L-9's
"intake always lands in triage"), with `threadState {not_tied, imported}` (05 §2.3 point 4: sync does
not cut the thread), `source: intake`, `external {source, externalId, url}`, the next project
number, revision 1 with both texts `author: "H"` and the original author's name kept as
`external.authorName` (display text, erasable, never a user id), and `createdBy` the person who ran
the import. Label names match the team's unarchived labels case-insensitively; an unmatched name is
**reported** in `unmatchedLabels` and never created, because creating a label is an admin write
(07's row) and an import is a member's. The external state is **not** imported (every item is
un-triaged here); comments, attachments and relations are not imported in W1, so L-220 stays
PARTIAL and the part says so. The batch is **all or nothing** for its new items, as the Hub's brief
batch (hub-contract's H3 row, :2250): one invalid item writes none of them, and the error names
`items[n].field`. Each created issue appends its own `issue.created` event (13's kinds); an item
already present appends none. **Tests**, each with its mutant: the same 3-item batch twice gives 3
created, then 3 existing and zero new events (kills a non-idempotent import); a batch whose item 3
has a 201-character title writes nothing and names `items[2].title` (kills a partial import);
imported issues sit in `triage` under a policy whose default is `todo` (kills intake using the
default); each reads `not_tied · imported` (kills an import that silently claims the thread); a
member's batch naming label `Infra`, absent from the team, creates no label and returns it in
`unmatchedLabels` (kills a member creating labels through import); a `javascript:` or `http:` URL
is 400 naming `items[0].url` (kills a link the issue view would render unsafely); 501 items is 400;
a viewer is 403; the same `externalId` imported into a second project creates a second issue there
(the scope is per project, stated in the part). It **does not claim** the Linear API pager or the CSV
parser (task 25), the dialog (69), or two-way sync (W5, which never writes a fact state or `done`).
Parity rows served: **L-220** (importers: Linear and CSV into `triage`, PARTIAL; closed with tasks 25
and 69).

**16 — `hubclient`'s Work methods over HTTP; the control plane's `/v1/hub/work/*` relay.** W1,
streams WH and WP, seam **SW.2** (third part: the relay paths); depends on 05 (the frozen interface
and rows), 15 (the last W1 route) and Hub tasks H33-H34 (`hubclient` over HTTP: transport, retries,
idempotency, refresh, the relay rule; the methods and the digest re-check). Files: new
`control-plane/internal/hub/hubclient/methods_work.go` and `methods_work_test.go`;
`control-plane/internal/api/work.go` and `work_test.go`; `internal/api/api.go` ⚑ (`Deps.Work
hubclient.Work`, optional, and the registrations), `internal/api/routes_audit_test.go` ⚑ (the
`catalogReadOnlyRoutes` map, `routes_audit_test.go:32`), `cmd/api/main.go` ⚑ (wires the Hub
client's `Work` into `Deps`; no new variable here — `PANACEA_APP_WORK_*` begins in tasks 24-25).
**Client:** one method per `WorkIssues` method, each on the exact method and path of 07's route list,
through H33's transport (so the `Idempotency-Key`, the single refresh on `expired` and the 10 s
timeout come for free); every response carrying an `IssueRevision` with content is re-hashed with
`hubapi.IssueRevisionDigest` and a mismatch is an error, never a value (the brief methods' re-check,
hub-contract §9.2). **Relay:** the control plane serves the same routes under `/v1/hub/work/…` — the
Hub path with `/v1` replaced by `/v1/hub/work` (for example `POST
/v1/hub/work/projects/{projectId}/issues`, `POST /v1/hub/work/issues/{issueId}/state`, `GET
/v1/hub/work/issues/search?q=`) — each handler a thin call to `Deps.Work` that decodes strictly,
passes Hub ids verbatim (no local-to-Hub id mapping here; `hublink`, H-task 35, owns that), and
answers by **the relay rule** unchanged (hub-contract §4.3, :277-295): a Hub 4xx problem with the
same status, code, message and details; 401 → `signed_out`; 5xx, timeout or refusal → 503
`upstream_unavailable` with `details.upstream = "hub"`; a nil `Deps.Work` (no Hub configured, which
local mode without a Hub still is, `OD-20`) → 503 `hub_not_configured`. Every route goes through
the `routes` recorder so the route audit sees it; every `GET` is in `catalogReadOnlyRoutes` and no
write is. The desktop's feed stays `/v1/hub/changes` (hub-contract §9.3 :1372); this task adds no
second feed. **Tests**, each with its mutant: an `httptest` Hub records each client method's method
and path, compared with 07's list read from one table (kills a path typo or a `PUT` for a `POST`); a
revision whose stored content was altered in the fake server fails the call (kills a skipped digest
re-check); a Hub 422 `policy_refused` with `details.policy` reaches the desktop with the same status,
code and details (kills a relay that rewraps problems); a Hub 500 becomes 503 `upstream_unavailable`
naming `hub` (kills leaking the Hub's 500); a nil `Work` answers 503 `hub_not_configured` and the
process still starts (kills a startup failure in local mode with no Hub); the route audit fails if a
`/v1/hub/work` write is added to `catalogReadOnlyRoutes` (kills an unaudited write); and, with
`-tags integration` against the compose stack, **every conformance row of tasks 04-05 passes against
the real Hub** through this client (kills fake-versus-real drift, the WS0 promise). Acceptance: `go
test ./control-plane/... -race -count=1` and the integration run both pass. It **does not claim** the
desktop's calls (task 37), Start (24) or import adapters (25). Parity rows served: none closed alone;
it carries every W1 Hub row to the desktop and serves **L-248** (the public API is the app's own
API).

**17 — W1's Hub end to end through `devissuer`.** W1, stream WH; no seam; depends on 10, 13, 14,
16 and Hub task H43 (the Hub's own end-to-end proof through `devissuer`, its harness and results
record). One new file, `control-plane/internal/hub/work_e2e_integration_test.go` (`-tags
integration`, against `control-plane/cmd/hub/compose.yaml` and `devissuer`, H-tasks 11-12), built
on H43's harness (**UNVERIFIED until merge**: the part reads its helper names on the sync base). It
signs in, as real PKCE tokens from `devissuer`, four people: Dylan and Alice (members of team T),
an org admin who is not a member, and Bea (a member of team U in the same org); it drives the Hub
through **task 16's `hubclient` over HTTP**, never the store, and asserts after each step both the
response and the exact event appended. **The story** (05 §5.3's demo, Hub half): an admin sets
`EP` on project P; Dylan creates a project brief and a branch brief with criterion `c-…` through
`Briefs`; Dylan imports a 142-item CSV batch (task 15) so the last item is **EP-142**, and every
item reads `triage` and `not_tied · imported`; under `priorityToLeaveTriage` he sets priority,
then accepts EP-142 into `todo`; he writes revision 2 with `serves [{briefKey, criterionId}]`, and
its thread state reads `on_thread`; Alice comments with a mention of Dylan, reacts, and is
subscribed by commenting while Dylan is subscribed by the mention; the moves `lane_bound` →
`started`, `lane_queued` → `in_review` and `land_trailer` (with a 40-hex `landCommit`) → `landed`
are made as Dylan; an evidence record is posted through `/v1/evidence` (H25) at `landCommit` for
the served criterion, then `evidence_observed` moves EP-142 to `done` with `proof: proven`; a
second issue with empty `serves` lands straight at `done` with `proof: no_criteria`; `GET
/v1/issues/search?q=EP-142` finds it; `/v1/refs` resolves `issue:is-…@2`; the org admin erases the
second issue, which then resolves `erased`; and `hub verify` passes at the end. **Tests**, each with
its mutant: the full sequence of Work event kinds in Dylan's `/v1/changes` equals one expected list,
in order (kills a route that appends two events, or none); Bea's feed over the same run holds **no**
Work event and her `GET` of EP-142 is 404 (kills a readability hole across teams); the `done` move
tried before the evidence is posted is 422 and EP-142 stays `landed` with `proof: waiting` (kills a
done rule that trusts the cause alone); a `done` move carrying the evidence of the other issue is 422
(kills sibling evidence); the canary title of the erased issue appears in no feed page and no
`/v1/refs` answer (kills erase leaving text); every event's `actor` is the person who acted, and
no Work event has `actor: null` in W1 (kills writes as "the system", 05 §3.6). Acceptance: `go test
./control-plane/internal/hub/... -tags integration -race -count=1 -run WorkE2E -v` passes, and its
output lists each step. It **does not claim** the desktop, the control plane's phase 3 and 4
deltas, a real Entra tenant (H43's checklist covers that), or any cloud run. Parity rows served: it
proves, against the real Hub, the Hub halves of L-3, L-8, L-9, L-13, L-17, L-24, L-34, L-38, L-41,
L-42, L-46, L-48, L-106, L-117, L-220 and L-229; task 70 records them with the desktop halves.

**18 — phase 4 delta: `issue:` in `provenance/key.go`; `Ref.Kind` `"issue"`.** W1, stream WP,
seam **SW.1** (the parser side); depends on 02 (the grammar and `hubapi.KeyIssue`) and phase 4 task
06 (`provenance/key.go` and `ref.go`, the only key parser, P5; phase 4 plan `:312-313`, **planned,
not on the `feat/phase-4` tree at `6ffa6bf`**: the package directory is absent there). Under rule
H2 the task **waits** until P4-06 is on the sync base, never writes a second copy, and the owner's
file wins every conflict. Files: `control-plane/internal/provenance/key.go` ⚑ and `key_test.go` ⚑,
`ref.go` ⚑ and `ref_test.go` ⚑; `desktop/src/api/discover.ts` ⚑ (phase 4 task 15's mirror of the
wire, **planned**) gains `kind` on its `ProvenanceRef` type if that type mirrors `Ref`. **In
`key.go`** (P4 task-06 part, `task-06.md:24-54`): `KindIssue Kind = "issue"` beside `KindBrief`,
`KindDesign`, `KindNone`; the grammar comment gains `issue ^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$`,
verbatim the Hub's (task 02; hub-contract §3.2 once 01's proposal lands); `Parse` fills `Key{Kind:
KindIssue, ID: "is-…", Revision: r}`; `String` round-trips; one constructor `IssueKey(issueID
string, revision int) string` that panics on a bad id or revision below 1, as `BriefKey` does
(`task-06.md:54`). **In `ref.go`**: `Ref` (`task-06.md:122-148`) has **no** kind field today, so
this task adds `Kind Kind "kind"`, set by every constructor (`Build`, `MissingRef`, `InvalidRef`,
`UnavailableRef`, `NoneRef`, `ErasedRef`; `""` only for `InvalidRef`, as hub `RefResult.kind` is
null exactly when invalid) and one field group for an issue: `IssueID`, `IssueNumber`,
`IssueAlias`, `Serves []Serve` (the revision's `serves`, as `{briefKey, criterionId}`), filled from
`hubapi.RefResult.Issue` (task 09); an issue ref carries no brief sections. **Tests**, each with its
mutant: `Parse("issue:is-3f2a91c07b1e4d2a@3")` gives `KindIssue` with ID and revision, and
`String` returns the input (kills a missing case); `issue:is-…@0`, upper-case hex and `EP-142` are
`ErrInvalidKey` (kills a looser grammar than the Hub's, which P5 forbids); a table test feeds every
golden key of `hubapi`'s tests and of this package to both parsers and requires the same verdict
(kills the two grammars drifting); every constructor sets `Kind` and a JSON test finds `"kind"` on
each (kills a zero-valued kind the desktop would read as a brief); `IssueKey("bf-…", 1)` panics;
phase 4's own tests still pass unchanged (kills an edit that breaks the owner's behaviour). It
**does not claim** the Materialiser, `Session.IssueRef` or the loader (task 19), nor any surface.
Parity rows served: the parser half of **L-191** (what the agent was told is named by key; closed
with 19) and of 05 §2.3's thread; no L row alone.

**19 — phase 4 delta: `Session.IssueRef`; the Materialiser's issue section; the loader's second
key.** W1, stream WP, seam **SW.7** (the managed block's issue section: what an agent is told, byte
for byte; consumed by Start, task 24, and W3's Delegate, task 101); depends on 05 (the fake's
`IssueRevisionAt`), 18 and phase 4 tasks 09 (`contextwrite/materialise.go`, plan `:319`), 10 (the
`api.Materialiser` seam in create and restart, `internal/api/materialiser.go`) and 16 (the desktop
loader and key sources, plan `:330-332`) — all **planned, UNVERIFIED until merge**; the task waits
for them (rule H2). Files: `control-plane/internal/api/materialiser.go` ⚑ (P4-10's
`MaterialiseRequest`/`MaterialiseResult`, P4 `task-10.md:88-110`), `internal/api/sessions.go` ⚑
(built: the wire `Session` at `sessions.go:619`, the create request at `:706`, the stamp at
`:921`), `internal/registry/session.go` ⚑ (built: `ContextRef` at `session.go:102-104`),
`internal/contextwrite/materialise.go` ⚑ and a new `contextwrite/issue.go` with its test and golden
`contextwrite/testdata/golden-issue.md`; `desktop/src/store/selectors.ts` ⚑ (P4-16's key sources),
`desktop/src/store/types.ts` ⚑ and `desktop/src/api/client.ts` ⚑ (the session's `issueRef`, where
`contextRef` is declared today). **Server:** `MaterialiseRequest` gains `IssueKey string` (an
`issue:is-…@r` key the person adopted — Start, task 24; `""` otherwise) and `MaterialiseResult`
gains `IssueRef string`; the create request gains `issueKey`; the handler forwards it and stamps
`Session.IssueRef` **from the result, never from the request** (phase 4's P6 rule for
`ContextRef`), on both the wire record and `registry.Session`. The Materialiser reads the revision
through `hubclient.Work.IssueRevisionAt` and renders an issue section **only** when the revision
resolves, belongs to the session's Hub project, and both texts are `author: "H"` (a `text·A`
revision is never rendered until a person accepts it, 05 §3.4); otherwise it refuses with the
existing `*MaterialiseError` and a new reason `issue_not_adoptable`, and the session does not
start (no silent partial block). Comments and requests are **never** rendered (05 §3.4, rule 6).
**SW.7, the bytes:** the section goes inside the managed block, after the brief's *Out of scope*
section and before its `---` footer (P4 `task-05.md:262-304`'s golden), as `## The issue you are
working on`, then the line `EP-142 · issue:is-…@3 · written by <display name>`, the title as a `###`
line, the description verbatim, and `Serves:` with one `- c-… — <criterion text>` line per `serves`
pair; the footer line gains ` Issue issue:is-…@3.` The title and description pass through phase 4's
comment-opener neutraliser (`<!--` → `<!-\-`, P4 `task-05.md:328`), so `strings.Count(block,
"<!--") == 2` still holds. Whether `claudemd.Render` takes the section as a parameter or
`contextwrite` splices it is decided against P4-05's merged code (the part says which). **Desktop:**
`selectSessionIssueRefs` is appended to `keySources` (P4 `task-16.md:150-159`: *"a later lens adds
ONE entry"*), so the one loader fetches `issue:` keys in the same batch as brief keys; no key is
split (P4's `selectors: no key is ever split` test still passes). **Tests**, each with its mutant: a
create request carrying `issueRef` directly is ignored and the session's `issueRef` is the
Materialiser's (kills a stamp from the request); the golden block is reproduced byte for byte from a
Go-built revision (kills any drift in SW.7 — fix the renderer, never the golden); a revision whose
description holds `<!-- panacea:discover:end -->` renders neutralised and `Inspect` stays ok (kills
an injected end marker); a revision with `author: "A"` refuses with `issue_not_adoptable` and writes
no file (kills rendering agent text into another agent's context); a comment on the issue never
appears in the block (kills comments rendered); an issue of another Hub project refuses (kills a
cross-project brief); ten hostile strings (fixture, until TM-05's injection corpus is merged,
**UNVERIFIED** path) each render inert; the loader test with a session carrying both refs makes
**one** `/v1/refs` call holding both keys (kills a second loader). It **does not claim** Start
(task 24), the issue chip (67), or that a hostile corpus is complete. Parity rows served: **L-191**
(ready-made prompt context that names `issue:is-…@r` beside the brief key; closed here with 09 and
18), and the context half of **L-175** (Start opens the issue in a session with the issue in
context; closed with tasks 24 and 61).

**20 — desktop test kit: `expectNoPrediction` and the Work fixtures.** W1, stream WD1, seam
**SW.8** (called by every desktop surface task); no dependency. Files: new
`desktop/src/test/expectNoPrediction.ts` and `expectNoPrediction.test.ts`,
`desktop/src/test/workFixtures.ts` and `workFixtures.test.ts`; nothing else. It follows the kit's
existing rules (`desktop/src/test/fixtures.ts:1-9`: builders declared **once**, a variant is an
override object, `FIXED_NOW` reused from `fixtures.ts:23-24`) and runs under the existing
`setupFiles` (`desktop/vitest.config.ts:11`). **`expectNoPrediction(container: HTMLElement):
void`** (`OD-27`; AX anti-pattern 2, `AGENTIC-UX.md:237`) throws with the offending text and a CSS
path when the container holds: a percentage (`/\d+(\.\d+)?\s?%/`) in text, `aria-label`,
`aria-valuetext` or `title`; any `role="progressbar"` or `role="meter"`, a `<progress>` or
`<meter>` element, or an `aria-valuenow`; an "N of M" completion figure (`/\b\d+\s+of\s+\d+\b/i`
and `/\b\d+\s*\/\s*\d+\s+(done|complete|proven|met|closed)\b/i`); or an ETA (`/\bETA\b/`,
`/\b(\d+\s?(s|m|min|h|d)|\d+\s+(minutes?|hours?|days?))\s+(left|remaining|to go)\b/i`,
`/\b(done|ready|finished|complete) by\b/i`, `/\bestimated\b/i`). **Two exemptions, each by an
explicit attribute and never by a heuristic:** an element with `data-verbatim` (quoted transcript,
file or diff text, which may legitimately contain `50%`) is skipped whole; an element with
`data-projection-rule` may carry a time projection such as *"next decision ~14:22 · rule: migration
gate"* (Q13 amendment 4), and is still checked for percentages, bars and "N of M". Plain counts per
group (*"4 issues"*), times (*"14:02"*), aging (*"waiting 2h 10m"*) and ids (*"EP-142"*) pass.
**`workFixtures.ts`** exports `WORK_IDS` (Hub-shaped: `pj-`, `bf-`, `is-` + 16 hex, `ic-`, `lb-`,
`c-` criterion ids, `u-` users, matching task 02's grammar), and one builder per SW.1 record —
`makeIssueHead`, `makeIssueRevision`, `makeServe`, `makeCriterion`, `makeComment`, `makeLabel`,
`makeTeamPolicy`, `makeWorkEvent` — plus `workScenario()`, the demo's world (05 §5.3): project
key `EP`; a branch brief with criteria `c-91e2…` (*the resume test passes*) and `c-40aa…`; EP-142
serving `c-91e2…` at revision 3, `landed` then `done` with `proof: proven` at `14:02`; EP-150
`blocks` EP-142 and is not landed; an imported `triage` item reading `not_tied · imported`; one
hand-set `done` with `proof: not_proven` and its reason; one `proven_self_attested`; a teammate's
issue with Alice's comment and mention. Until task 37 (SW.15) exists, the builders are typed by
local types in this file that mirror task 03's JSON field names exactly; task 37 replaces them with
imports from `src/api/work.ts`, and its typecheck proves the two agree. **Tests**, each with its
mutant: `expectNoPrediction` throws on `"40%"`, on `aria-label="60% done"` (kills a text-only
check), on `<div role="progressbar">`, on `<progress>`, on `"3 of 5 criteria"`, on `"ETA 14:22"`,
on `"about 5 min left"` and on `"done by Friday"` (each kills a missing rule); it passes on
`"EP-142"`, `"14:02"`, `"4 issues"`, `"waiting 2h 10m"` and `"c-91e2 still open (EP-150 not
landed)"` (kills an over-broad rule that would ban facts); `"50%"` inside `data-verbatim` passes
and `"~14:22 · rule: migration gate"` inside `data-projection-rule` passes while `"40%"` inside the
same element still throws (kills an exemption that disables every rule); every `WORK_IDS` id matches
the Hub's grammar (kills a fixture id the real Hub would refuse); `workScenario()` holds every one of
the nine issue states and every `proof` word at least once (kills a fixture set that hides a glyph
or a state from surface tests); no fixture string trips `expectNoPrediction` when rendered as text
(kills a fixture that would force exemptions). Acceptance: `npm --prefix desktop test --
src/test/` and `npm --prefix desktop run typecheck` pass. It **does not claim** to detect a bar drawn
by CSS widths alone (the owner's review, task 39, covers the look), and it closes no L row alone; it
enforces `OD-27` on every surface task that closes one.


