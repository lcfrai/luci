### Task 17: W1's Hub end to end through `devissuer`

W1, stream WH; no seam. Depends on tasks 10 (the fact moves and the done rule, SW.4), 13 (the Work
event kinds on the change feed, SW.5), 14 (erase), 16 (`hubclient`'s Work methods over HTTP), and
Hub task H43 (the Hub's own end-to-end proof through `devissuer`, its harness and results record).

Every W1 Hub task proved its piece against a scratch database. This one proves **the story**: the
Hub half of 05 §5.3's demo, driven the way the desktop will drive it, through **task 16's
`hubclient` over HTTP** and never the store, by four people holding real PKCE tokens from
`devissuer`. After each step it asserts two things: the response, and **the exact event the step
appended** to the change feed. A route that answers correctly and appends two events, or none, is
the bug this test exists to catch, because the desktop's loader (task 37) invalidates on events and
a missing one leaves a stale board.

**The one thing this task must not get wrong is who can see what.** The story runs in team T; a
member of team U in the same org watches the same feed throughout and must see **no** Work event
and get **404** for EP-142. A readability hole across teams is the defect that would ship
unnoticed by every single-team test in tasks 08-15.

**Files:**
- `control-plane/internal/hub/work_e2e_integration_test.go` — `TestWorkE2E`, `//go:build
  integration` (new)
- nothing else. The results of the run go in H43's record,
  `docs/verification/2026-09-23-phase-h-results.md` (H43 `task-43.md:16`), as one appended section
  **by task 70**, which records W1's parity rows with their desktop halves; this task writes no
  document.

**What is on the tree today (verified 2026-09-25).** `control-plane/internal/hub/` on `feat/hub`
(`.claude/worktrees/hub` at `9f2fa31`) holds only `hubapi` and `hubclient`; `hubtest`, `server`,
`devissuer`, `store` and `control-plane/cmd/hub/compose.yaml` are **planned** (H-plan tasks 02-16,
11-12, 43) and **UNVERIFIED until merge**. H43's part names its world
(`task-43.md:22-38`): `hubtest.Migrated`, `devissuer` in-process with `Auto` and `AccessTTL`, the
Hub's real handler (`server.New` + `entra.New`, `AllowInsecureAuthority`) on an `httptest.Server`,
and one frozen test clock `clk` shared by the issuer and the Hub's validator. This task builds the
same world and reuses H43's helpers by their merged names; the names below are H43's part's and are
re-read at sync.

**Interfaces:**
- Produces: `TestWorkE2E` and its step list, the proof task 70 cites.
- Consumes: task 16's `hubclient.Client` Work methods; H34's `Briefs`, `Projects`, `Teams` and
  `Evidence` methods; H43's world; `devissuer.Options`, `Config`, `User{Roles}` (H11
  `task-11.md:66-129`) and its `Auto` + `login_hint` code flow (H12 `task-12.md:47`); task 10's
  `StateMove` causes and `proof` words; task 13's Work event kinds; task 14's erase.
- Required by: task 70 (W1's local-mode demo and its results record).

#### Contract — the world and the four people

```go
// control-plane/internal/hub/work_e2e_integration_test.go
//go:build integration

package hub_test

// workPeople is devissuer's one tenant for this run: four users, one org.
// No secret is in it; devissuer's config refuses unknown fields (H11).
var workPeople = []devissuer.User{
	{OID: "5b0c1c9e-0000-4000-8000-00000000d71a", Name: "Dylan Example",
		Username: "dylan@example.test", Email: "dylan@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-0000000a11ce", Name: "Alice Example",
		Username: "alice@example.test", Email: "alice@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-00000000ad01", Name: "Olga Example",
		Username: "olga@example.test", Email: "olga@example.test",
		Roles: []string{"Hub.OrgAdmin"}}, // O: org admin, member of no team
	{OID: "5b0c1c9e-0000-4000-8000-000000000bea", Name: "Bea Example",
		Username: "bea@example.test", Email: "bea@example.test"}, // N: team U only
}

// person is one signed-in principal: a real hubclient.Client over HTTP whose
// TokenSource holds the access token the code + PKCE S256 flow returned.
type person struct {
	name string
	oid  string
	hub  *hubclient.Client
}

// signIn runs devissuer's /authorize (Auto + login_hint, H12) and /token with
// a fresh verifier, and returns a person whose every call carries that token.
// It never calls MintAccessToken: the proof is that the browserless code flow
// the desktop's sign-in uses yields a token the Hub accepts for Work routes.
func signIn(t *testing.T, w *workWorld, username string) person
```

**The roles, set up in step 0 through the Hub's own routes.** Dylan creates team **T** (so he is
its `admin`) and project **P** in it with remote `github.com/lcfrai/panaceaapp`; he invites Alice,
who accepts as a `member` (H43's invite binding). Bea creates team **U** and a project there. Olga
is in no team and holds `Hub.OrgAdmin`: the matrix's `O` column (hub-contract §5.3, `:337-341`),
who may erase but may **not** read project content she is not a member of (`:331`).

**The frozen clock.** `clk` is H43's: frozen through every step (`task-43.md:29-35`), so no token
expires and no refresh grant is made mid-story. Every timestamp this test compares is read from a
response, never from the wall clock.

#### Contract — the story, one `t.Run` per step

Each step names who acts, the call (a task 16 or H34 method), what the response must hold, and the
Work events it must append to the feed **as Dylan reads it**, in order. `—` means the step
appends **no** Work event Dylan can see. The kind names are task 13's registry
(`hubapi/workevents.go`, `task-13.md:120-139`), and three of its rules shape the column: a write
that inserts a subscription row appends `subscription.set` after its own event; one that finds a
row appends nothing and keeps the first cause (`task-13.md:52-62`); and `subscription.set` is shown
**only to its subject** (`:32-37`). A comment appends `comment.created`, then one
`comment.mentioned` per mention (`task-12.md:376`).

| # | Step | As | Call | Response holds | Work events appended |
|---|---|---|---|---|---|
| 0 | `teams_and_project` | Dylan, Alice, Bea | H34 `CreateTeam`, `CreateProject`, `CreateInvite`; Alice accepts | T has Dylan `admin`, Alice `member`; U has Bea | — |
| 1 | `project_key` | Dylan | `SetProjectKey(P, "EP")` | `key: EP`, `setBy` Dylan | `project.key_set` |
| 2 | `triage_policy` | Dylan | `PutTeamPolicy(T, {priorityToLeaveTriage: true})`; `CreateLabel(T, {name: "resume", color: "accent"})` | the policy echoed; label `lb-…` | `team.work_policy_set`, `label.created` |
| 3 | `briefs` | Dylan | H34 `CreateBrief` (project, then branch `feature/resume`), `WriteRevisions` with criteria `c-91e2…` and `c-40aa…` | both heads; revision 1 `complete` | — (brief events are not Work kinds) |
| 4 | `import_142` | Dylan | `ImportBatch(P, {source: csv, items: 142})` | `created` holds 142 refs; the last is `EP-142`; every item `state: triage`, `threadState {not_tied, imported}` | 142 × (`issue.created`, Dylan's `subscription.set` cause `create`), in item order |
| 5 | `triage_needs_priority` | Dylan | `MoveIssueState(EP-142, {to: todo, cause: hand})` | 422 `policy_refused`, `details.policy: priorityToLeaveTriage` | — |
| 6 | `priority_then_accept` | Dylan | `PatchIssue(EP-142, {priority: high, labels: [lb-…]})`, then the same move | `priority: high`, the label; then `state: todo` | `issue.updated`, `issue.state` |
| 7 | `serves_the_brief` | Dylan | `WriteIssueRevision(EP-142, {baseRevision: 1, content: {…, serves: [{briefKey, criterionId: c-91e2…}]}})` | revision 2; `threadState.kind: on_thread` on the head | `issue.revised` |
| 8 | `alice_comments` | Alice | `CreateComment(EP-142, {body: {author: H, …}, mentions: [Dylan]})`, `React(…, {emoji: "\U0001F44D", on: true})` | the comment; one reaction by Alice | `comment.created`, `comment.mentioned` (subject Dylan), `comment.reacted` (Alice's `subscription.set` is hers alone) |
| 9 | `subscriptions` | Alice, Dylan | `Subscriptions(page)` for each | Alice's EP-142 row `cause: comment`; Dylan's `cause: create`, from the import, **unchanged by the mention** (task 13 decision 2) | — |
| 10 | `lane_bound` | Dylan | `MoveIssueState(EP-142, {to: started, cause: lane_bound})` | `state: started` | `issue.state` |
| 11 | `lane_queued` | Dylan | `{to: in_review, cause: lane_queued}` | `state: in_review` | `issue.state` |
| 12 | `land_trailer` | Dylan | `{to: landed, cause: land_trailer, landCommit: <40 hex, L>}` | `state: landed`, `proof: waiting` | `issue.state` |
| 13 | `done_waits_for_evidence` | Dylan | `{to: done, cause: evidence_observed}` with no evidence | 422; `GET` shows `landed`, `proof: waiting` | — |
| 14 | `second_issue_no_criteria` | Dylan | `CreateIssue(P, {title: CANARY, serves: []})` → EP-143; `AddRelation(EP-143, {kind: relates, issueId: EP-142})`; `lane_bound`, `lane_queued`, `land_trailer` at commit `L2` | the relation on both heads; the last move answers `state: done`, `proof: no_criteria` (no criterion to wait for) | `issue.created`, Dylan's `subscription.set`, `issue.related`, `issue.state` × 3 |
| 15 | `sibling_evidence_refused` | Dylan | H34 `RecordEvidence` for `c-91e2…` at `L2`; then EP-142 `{to: done, cause: evidence_observed, evidence: [that id]}` | 422; EP-142 stays `landed` | — |
| 16 | `evidence_then_done` | Dylan | `RecordEvidence` for `c-91e2…` at `L`; then the move citing it | `state: done`, `proof: proven` | `issue.state` |
| 17 | `search_and_refs` | Dylan | `SearchIssues({q: "EP-142"})`; H34 `Resolve(["issue:<id>@2"])` | one head, EP-142; the ref `resolved`, `kind: issue`, `serves` as step 7 | — |
| 18 | `erase` | Olga | `EraseIssue(EP-143)`; then Dylan `Resolve(["issue:<EP-143 id>@1"])` | Olga 200; the ref `erased`, key and digest kept | `issue.erased` |
| 19 | `the_chain_verifies` | — | `hub verify` against the database, as H43 step 12 | exit 0, `ok revisions`, `ok audit` | — |

Steps 5 and 13 are the refusals the story has to survive: each is asserted to leave the feed
**unchanged**, which is the only way to tell a refused write from one that half-happened.
`L` and `L2` are two fixed 40-hex commit ids declared in the test; no git repository is needed,
because the Hub records the commit a fact cause names and does not read it (task 10).

The `StateMove` fields `landCommit` and `evidence` are task 10's (SW.4; the skeleton's gap 3 is why
the evidence travels on the move and not on the record). This part writes them as task 10's scope
drafts them and takes the merged names at sync (**UNVERIFIED until merge**).

#### Tests to write first — the assertions that ride on the story

`TestWorkE2E` is one test; these are the `t.Run` blocks and checks inside it that each kill a
named mutant. They are written before steps 0-19 are made to pass, so each fails first for the
reason it names.

| Check | Where | Asserts | Mutant it kills |
|---|---|---|---|
| `events_in_order` | after step 19 | Dylan's `/v1/changes` from the cursor taken after step 0, filtered to Work kinds, equals `expectedWorkEvents` exactly, in order (the last column above, flattened) | a route that appends two events, or none; an import that appends one batch event instead of 142 |
| `refusals_append_nothing` | steps 5, 13, 15 | the feed's `next` cursor is the same before and after each refusal | a refused move that still wrote an event, or a half-applied state |
| `bea_sees_nothing` | after step 19 | Bea's `/v1/changes` over the run holds **no** Work event; her `Issue(EP-142's id)` is 404 `issue_not_found`; her `SearchIssues({q: "EP"})` is empty | a readability hole across teams; a feed filtered by org and not by team |
| `olga_reads_nothing` | step 18 | before erasing, Olga's `Issue(EP-143's id)` is 404; the erase itself is 200 | an org admin who can read content she erases (hub-contract `:331`) |
| `done_needs_its_own_evidence` | steps 13, 15 | both 422; EP-142 reads `landed`, `proof: waiting` after each | a done rule that trusts the cause alone; sibling evidence from another commit accepted |
| `canary_is_gone` | after step 18 | the canary title `"CANARY 7f3a erased title"` appears in no page of anyone's feed, no `Resolve` answer and no `SearchIssues` result; the raw response bytes are searched, not decoded fields | erase that leaves text in an event payload or a search index |
| `every_actor_is_a_person` | after step 19 | every Work event's `actor` is non-null and is the `User` of the person who made that step's call | a write recorded as "the system" (05 §3.6); an actor taken from the wrong token |
| `import_is_idempotent` | after step 4 | the same batch again answers `created: []`, `existing` 142 refs with the same ids, and appends nothing | a re-run of an import that duplicates 142 issues (SW.6) |

`expectedWorkEvents` is built from one table in the test, not typed out 160 times:

```go
// expectedWorkEvents is the flattened last column of the story table. Each
// entry is (kind, the step that appended it); the check compares both.
func expectedWorkEvents() []wantEvent {
	var w []wantEvent
	add := func(step, kind string, n int) {
		for range n {
			w = append(w, wantEvent{step: step, kind: kind})
		}
	}
	add("project_key", hubapi.EventProjectKeySet, 1)
	add("triage_policy", hubapi.EventTeamWorkPolicySet, 1)
	add("triage_policy", hubapi.EventLabelCreated, 1)
	for range 142 { // one issue, then the importer's subscription (task 13 :388)
		add("import_142", hubapi.EventIssueCreated, 1)
		add("import_142", hubapi.EventSubscriptionSet, 1)
	}
	add("priority_then_accept", hubapi.EventIssueUpdated, 1)
	add("priority_then_accept", hubapi.EventIssueState, 1)
	add("serves_the_brief", hubapi.EventIssueRevised, 1)
	add("alice_comments", hubapi.EventCommentCreated, 1)
	add("alice_comments", hubapi.EventCommentMentioned, 1) // Alice's subscription.set is hers alone
	add("alice_comments", hubapi.EventCommentReacted, 1)
	add("lane_bound", hubapi.EventIssueState, 1)
	add("lane_queued", hubapi.EventIssueState, 1)
	add("land_trailer", hubapi.EventIssueState, 1)
	add("second_issue_no_criteria", hubapi.EventIssueCreated, 1)
	add("second_issue_no_criteria", hubapi.EventSubscriptionSet, 1)
	add("second_issue_no_criteria", hubapi.EventIssueRelated, 1)
	add("second_issue_no_criteria", hubapi.EventIssueState, 3)
	add("evidence_then_done", hubapi.EventIssueState, 1)
	add("erase", hubapi.EventIssueErased, 1)
	return w
}
```

304 events in Dylan's feed; the constants are task 13's, so a typo is a compile error. Alice's feed
is checked by the same rule with her own `subscription.set` rows (one, from her comment) in place
of Dylan's. The step each event belongs to is known because the test records the feed cursor at
the start of every step and reads the events between two cursors; a mismatch names the step.

#### Steps

- [ ] **Step 1: confirm the sync base.** H43's `e2e_integration_test.go` and task 16's
  `methods_work.go` are on it: `grep -n "func TestTwoTeammatesShareTheThread"
  control-plane/internal/hub/e2e_integration_test.go` and `grep -n "var WorkRoutes"
  control-plane/internal/hub/hubclient/methods_work.go` each print one line. If not, stop (rule H2).
- [ ] **Step 2: read H43's world and helpers by their merged names** (its test clock, the
  `devissuer` start-up, the Hub's `httptest.Server`, the `hub verify` call) and reuse them. If they
  are unexported inside H43's test file, move them into a shared `e2e_world_integration_test.go` in
  the same package **with H43's owner's agreement**, never a second copy (rule H2).
- [ ] **Step 3: write `signIn`, the people, step 0 and the `bea_sees_nothing` check first.** Run
  the acceptance command; expect step 0 to pass and the check to fail on no events at all (the
  story has not run), which proves it reads Bea's own feed.
- [ ] **Step 4: write steps 1-19 in order, each with its response assertion.** Run after each
  group of three; a failure names its step. A step that fails because a W1 route disagrees with
  its part is a finding for that route's task, recorded in the commit message, never a change of
  expectation here.
- [ ] **Step 5: write the eight checks of the table above**, and `expectedWorkEvents`. Run; expect
  every check to pass.
- [ ] **Step 6: prove two checks bite.** Temporarily make task 13's feed filter return every
  event of the org (one line); `bea_sees_nothing` must fail. Temporarily make task 14's
  `hub.erase_issue` skip blanking `canonical` (task 07's owner function); `canary_is_gone` must fail
  on the `Resolve` answer. Revert both; the run is green. (Events carry no text at all, task 13's
  rule, so the feed half of `canary_is_gone` guards a regression, not today's code.)
- [ ] **Step 7: commit** `control-plane/internal/hub/work_e2e_integration_test.go` by path.

#### Acceptance

```bash
docker compose -f control-plane/cmd/hub/compose.yaml up -d --wait
go test ./control-plane/internal/hub/... -tags integration -race -count=1 -run WorkE2E -v
# expect, in order: "--- PASS: TestWorkE2E/teams_and_project" … one line per step 0-19,
# then one per check (events_in_order, refusals_append_nothing, bea_sees_nothing,
# olga_reads_nothing, done_needs_its_own_evidence, canary_is_gone,
# every_actor_is_a_person, import_is_idempotent), then "--- PASS: TestWorkE2E" and "ok"
docker compose -f control-plane/cmd/hub/compose.yaml down
```

H43's world may run on `hubtest.Migrated` without compose (`task-43.md:22`); if so the two
`docker compose` lines are dropped and the `go test` line is the whole acceptance.

#### What this task does NOT claim

- The desktop: no surface is rendered and no `expectNoPrediction` runs here (task 70 does both).
- The control plane's phase 3 and phase 4 deltas (tasks 18-23): the fact moves are made by the
  test as the person, as the control plane will make them; that the control plane makes them at
  the right moment is task 21-23's proof.
- A real Entra tenant (H43's real-tenant checklist covers that) or any cloud run.
- Load: 142 items is the demo's number, not a performance claim.

Parity rows served: it proves, against the real Hub, the **Hub halves** of L-3, L-8, L-9, L-13,
L-17, L-24, L-34, L-38, L-41, L-42, L-46, L-48, L-106, L-117, L-220 and L-229. None is closed
here; task 70 records each with its desktop half. L-17 (labels) is steps 2 and 6; L-24
(relations) is step 14, where the relation is `relates` because the demo's `EP-150 blocks EP-142`
lives in the desktop fixtures (task 20), not on this Hub. Every kind named here is in task 13's
registry; a rename there is a break of SW.5 (`task-13.md:116-118`) and shows up here as a failing
`events_in_order`.

#### Gaps found while writing this part

1. **H43's helpers may be unexported** in its own test file; step 2 says how to share them
   without a copy, which needs H43's owner.
2. **Scope 17 says Dylan is subscribed "by the mention"**, but task 13's part subscribes the
   person who ran an import (`task-13.md:388`) and never overwrites a row's first cause, so
   Dylan's cause is `create`. This part follows task 13; the scope sentence is superseded.
3. **The `every_actor_is_a_person` check covers `subscription.set` too**: its actor is the person
   whose write inserted the row (Dylan for the import), not its subject. Task 13's part should say
   so explicitly if it does not already.

