### Task 05: the fake, part 2 — designs, releases, evidence, the feed; and the conformance suite (start line S0)

**Lane A. Depends on: 04. About 3 hours. Contract task H0.**

This finishes the fake and writes **the one suite** both the fake and the real Hub must pass
(contract §9.2: *"`hubclient/conformance` is one table-driven suite run against the fake … and
against a real Hub"*). When it is merged, **start line S0** is reached: phases 3, 4, 5 and 7 can
build every Hub-backed handler against `hubclient.NewFake()` and trust that what passes there will
pass against the server, because the server tasks (15–27) are accepted by the same rows.

**Files:**
- `control-plane/internal/hub/hubclient/fake_designs.go` — blobs, approvals, session links, design keys in `Resolve` (new)
- `control-plane/internal/hub/hubclient/fake_releases.go` — reserve, pin, confirm, abandon, join, list (new)
- `control-plane/internal/hub/hubclient/fake_feed.go` — evidence and the change feed (new)
- `control-plane/internal/hub/hubclient/fake.go` — delete task 04's part-2 placeholders (modify)
- `control-plane/internal/hub/hubclient/conformance/conformance.go` — `Env`, `Group`, `Run`, the rows (new)
- `control-plane/internal/hub/hubclient/conformance/fake_test.go` — `Run` against the fake, every group (new)

#### Contract — complete

```go
// control-plane/internal/hub/hubclient/conformance/conformance.go

// Package conformance is the behaviour every hubclient.Hub must show a caller:
// the fake (fake_test.go) and the real Hub through the HTTP client
// (hub_integration_test.go, task 34). A row is written once and run against both.
// It is a normal package importing testing — like net/http/httptest — because a
// _test.go file cannot be imported by the integration test of another package.
package conformance

import (
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Env is one fresh world per row: a team T, and four principals of one org.
type Env struct {
	Admin    hubclient.Hub // admin of T
	Member   hubclient.Hub // member of T
	Viewer   hubclient.Hub // viewer of T
	Outsider hubclient.Hub // same org, in no team
	// TeamID is T.
	TeamID string
	// Advance moves the Hub's clock. nil on a real Hub, where every row that
	// needs it is skipped with t.Skip and says so.
	Advance func(d time.Duration)
	// ReservationTTL is what this Hub was configured with.
	ReservationTTL time.Duration
}

type Group string

const (
	GroupTeams     Group = "teams"     // server tasks 15, 16a
	GroupProjects  Group = "projects"  // 17
	GroupBriefs    Group = "briefs"    // 18, 19
	GroupRefs      Group = "refs"      // 20 (brief keys), 22 (design keys)
	GroupDesigns   Group = "designs"   // 21, 22
	GroupReleases  Group = "releases"  // 23, 24
	GroupEvidence  Group = "evidence"  // 25
	GroupFeed      Group = "feed"      // 26
)

// AllGroups is every group, in dependency order.
var AllGroups = []Group{GroupTeams, GroupProjects, GroupBriefs, GroupRefs, GroupDesigns,
	GroupReleases, GroupEvidence, GroupFeed}

// Run runs every row of the named groups (all of them when none are named),
// each as t.Run("<group>/<row>"), each with a fresh Env from newEnv.
func Run(t *testing.T, newEnv func(t *testing.T) Env, groups ...Group)
```

```go
// control-plane/internal/hub/hubclient/conformance/fake_test.go — complete.
package conformance_test

import (
	"context"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient/conformance"
)

func TestFakeConforms(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Env {
		// context.Background, not t.Context: the module's floor is Go 1.23 and
		// t.Context arrived in 1.24.
		ctx := context.Background()
		f := hubclient.NewFake()
		now := time.Date(2026, 9, 23, 4, 0, 0, 0, time.UTC)
		f.SetNow(func() time.Time { return now })
		f.SetReservationTTL(15 * time.Minute)
		admin := f.As(hubclient.FakeAda)
		team, err := admin.CreateTeam(ctx, hubapi.CreateTeamRequest{Name: "Conformance"})
		if err != nil {
			t.Fatal(err)
		}
		member, viewer := f.As(hubclient.FakeGrace), f.As(hubclient.FakeLinus)
		outsider := f.As(hubclient.FakeUser{ID: "u-00000000000000a4", DisplayName: "Out Sider", Email: "out@example.test"})
		for _, p := range []struct {
			who  *hubclient.Fake
			u    hubclient.FakeUser
			role hubapi.Role
		}{{member, hubclient.FakeGrace, hubapi.RoleMember}, {viewer, hubclient.FakeLinus, hubapi.RoleViewer}} {
			if _, err := admin.CreateInvite(ctx, team.ID, hubapi.CreateInviteRequest{Email: p.u.Email, Role: p.role}); err != nil {
				t.Fatal(err)
			}
			if _, err := p.who.Me(ctx); err != nil { // binds the invite
				t.Fatal(err)
			}
		}
		return conformance.Env{Admin: admin, Member: member, Viewer: viewer, Outsider: outsider,
			TeamID: team.ID, Advance: func(d time.Duration) { now = now.Add(d) },
			ReservationTTL: 15 * time.Minute}
	})
}
```

#### What the fake must do — part 2's rules

1. **Blobs (§7.6).** `PutBlob` refuses a content type outside `hubapi.BlobContentTypes` (400
   `bad_request`, `details.field=Content-Type`), a size over 16 MiB (413 `blob_too_large`), and bytes
   that do not hash to `digest` (422 `digest_mismatch`, nothing stored). Blobs dedupe per org and
   associate per project; a project past 1 GiB is 409 `quota_exceeded`. `PutBlob` seeks `body` to
   0 before it reads, as the transport does before every attempt (HA5-1, task 00), so a caller
   that passes a half-read reader gets the same answer from the fake and the Hub. `GetBlob` of a blob not
   associated with **this** project is 404 `blob_not_found`. Needs `member` to put, `viewer` to get.
2. **Approvals (§7.7).** The six checks **in contract order**, each with its error; the approver is
   the view's user; a second approval of one key is 409 `design_already_approved` with
   `details.approvalId`. An approval against a complete revision older than the head is allowed.
   `Resolve` of a `design:` key returns the approval, `resolved`; `heads` gains the approval's
   brief.
3. **Session links (§7.8).** `design_not_approved`, `key_unresolvable`; the same user repeating
   `(designKey, sessionId)` gets the existing link back.
4. **Releases (§7.9).** Reserve takes the pin by the table in §7.9 — `briefId`, else the live
   branch brief for `branchRef`, else the live project brief — at its **latest complete**
   revision, or `none` with `no_brief` / `no_complete_revision`; with `sourceDeploymentId`, it
   copies the pin of the **confirmed** release joined to that deployment (`inherited`), or `none`
   with `source_unpinned` / `source_unknown`, and a rollback also copies `commit`, `branchRef` and
   `designKey`. `designKey` without an approval → 422 `design_not_approved`. Confirm and abandon:
   reserver only (403 `not_reserver`); the state table of §7.9 exactly; confirm after `expiresAt`
   → 409 `release_expired`, and a `reserved` release read after `expiresAt` reports `expired`
   (the fake expires lazily; the Hub's janitor does it every 60 s — the difference is invisible
   through the interface). One release per `(panaceaProject, pipelineRun)` and per
   `(panaceaProject, deploymentId)` in the org (409 `conflict`). `Releases` filters — `Kind`
   included (HA7-5) — and lists newest first. **The environment (HA7-1, task 00):** `null` is
   accepted only on a run without `sourceDeploymentId` (else 400 `field: environment`);
   `JoinDeployment` with `Environment` sets a null one, is a no-op on an equal one, and is 409
   `conflict` (`details.field=environment`, nothing joined) on a different one; a confirmed,
   abandoned or expired release keeps a null environment until a join names it. **A rollback
   confirm (HA7-2):** `{}`; a `deploymentId` equal to the rollback's `sourceDeploymentId` → 409
   `conflict`, nothing joined.
5. **Evidence (§7.10).** `repo` must be a remote of a project the caller is a **member** of, else
   422 `repo_not_linked` (a viewer of that project: 403 `forbidden`); `briefKey` must resolve in it
   (422 `key_unresolvable`); `because` is 1–2,000 characters after trimming. `EvidenceByDigest`
   answers oldest first, and an empty list — never a 404 — for a repo the caller cannot read.
6. **The feed (§7.11, §8).** Every write in both parts appends one event per changed record, with
   §8's kind, subject and data. `Changes("latest", …)` returns no events and the head as `next`;
   `Changes(n, …)` returns later events the view can see (a readable project, or one of its teams),
   never an audit-only kind; `more` is true when `limit` cut the page.

#### The rows — `conformance.go`, by group

Each row names its assertion; together they are the mutants the fake and the server must both
kill. Rows marked ⏱ need `Advance` and skip on a real Hub.

| Group | Row | Asserts |
|---|---|---|
| teams | `me_binds_invites` | a fresh invite binds on the invitee's `Me`, once |
| teams | `outsider_sees_404` | `Outsider.Team(T)` → 404 `team_not_found` |
| teams | `member_cannot_invite` | `Member.CreateInvite` → 403 `forbidden`, `required=admin` |
| teams | `last_admin` | `Admin.RemoveMember(T, admin)` → 409 `last_admin` |
| projects | `repo_linked_once` | a second project with the same repo key → 409 `repo_already_linked` |
| projects | `repo_key_forms` | phase 3's Azure DevOps form accepted; the contract's old form 400 (A1) |
| projects | `for_repo_is_readable_only` | `Outsider.ProjectForRepo` → `ok == false` |
| briefs | `brief_exists` / `archive_frees_ref` / `archived_refuses_writes` | as §7.5 |
| briefs | `numbering_and_conflict` | base 0 → 1; stale base → 409 `revision_conflict`, `headRevision` |
| briefs | `batch_all_or_nothing` | a stale item 3 of 4 writes nothing |
| briefs | `viewer_cannot_write` | `Viewer.WriteRevisions` → 403 `forbidden` |
| refs | `five_states` | resolved, missing, unreadable-is-missing, erased (fake only: `EraseBrief`; real Hub: via the org-admin route once task 27 lands, else skipped), invalid; request order kept |
| refs | `design_key_resolves` | an approved design key → `resolved` with the approval; its brief in `heads` |
| designs | `blob_hash_checked` / `blob_type_allowlist` / `blob_other_project_404` | as §7.6 |
| designs | `approval_checks_in_order` | one row per check 1–6, each with its code |
| designs | `approver_is_the_caller` | `approvedBy` is the calling user, whatever the body says (the body has no field for it) |
| designs | `session_link_repeat_returns_existing` | same user, same `(designKey, sessionId)` → the same link id |
| releases | `pin_ship_head` / `pin_falls_back_to_project_brief` / `pin_no_brief` / `pin_no_complete_revision` | the §7.9 table |
| releases | `pin_does_not_move` | revise the brief after reserve; `Release(id).Pin` unchanged |
| releases | `confirm_by_reserver_only` | `Member.Confirm` of Admin's release → 403 `not_reserver` |
| releases | `confirm_idempotent` / `confirm_other_run_409` | as §7.9's state table |
| releases | `expiry` ⏱ | `Advance(TTL + 1 s)`; confirm → 409 `release_expired` |
| releases | `rollback_inherits` / `source_unknown` / `source_unpinned` | the pin copied from the confirmed release joined to the deployment |
| releases | `by_deployment_id` | `Releases(q.DeploymentID)` finds the joined release; an unjoined id → no items |
| releases | `environment_null_until_joined` | a run reserved with `environment: null` → confirm → still `null`; join `{d-1, "feat-x"}` → `"feat-x"`; a second join `{d-2, "feat-y"}` → 409 `conflict`, `d-2` not joined; a rollback with `null` → 400 `field: environment` (HA7-1) |
| releases | `environment_null_abandons_and_expires` ⏱ | a null-environment run abandons → 200 `abandoned`; another one, `Advance(TTL + 1 s)` → reads `expired` — neither refused for its null (HA7-1) |
| releases | `rollback_confirm_names_no_source` | a rollback confirmed with `deploymentId` = its `sourceDeploymentId` → 409 `conflict`; with `{}` → 200 (HA7-2) |
| releases | `kind_filter` | one run and one rollback confirmed in one environment → `Kind: rollback` returns only the rollback (HA7-5) |
| briefs | `list_filters` / `latest_complete` | as task 04's `TestFakeListBriefs` and `TestFakeForBranchCarriesLatestComplete` (HA4-2, HA4-4) |
| briefs | `batch_of_sixty_four` | 64 revisions in one call → 201; 65 → 400 `field: revisions` (HA4-3) |
| designs | `put_blob_rewinds` | a `bytes.Reader` already read to its end → `PutBlob` stores the whole blob, digest checked (HA5-1) |
| evidence | `repo_not_linked` / `viewer_forbidden` / `oldest_first` / `unreadable_repo_is_empty` | as §7.10 |
| feed | `latest_then_new_events` | `Changes("latest")` → none; after a revision write, `Changes(next)` holds `brief.revised` with `data.key` |
| feed | `outsider_sees_nothing` | the outsider's feed after the team's writes holds none of them |

#### Tests to write first

The suite **is** the tests. Write the rows first against task 04's placeholders — every part-2 row
fails with `not_in_fake_yet` — then implement the fake until `TestFakeConforms` passes.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFakeConforms` | every row of every group passes against the fake | any rule above missing from the fake |
| `TestRunNamesEveryRow` | `Run` with a recording `*testing.T` shim lists ≥ 40 subtests, each `group/row`, no duplicates | a row silently dropped from a group's table |
| `TestClockRowsSkipWithoutAdvance` | an `Env` with `Advance: nil` makes `releases/expiry` report skipped, not passed | a real-Hub run that "passes" a row it never ran |

#### Steps

- [ ] **Step 1:** Re-read contract §7.6–§7.11 and §8.
- [ ] **Step 2:** `conformance.go` with every row; `fake_test.go`. Part-2 rows fail.
- [ ] **Step 3:** `fake_designs.go`, `fake_releases.go`, `fake_feed.go`; delete the placeholders.
  `TestFakeConforms` passes under `-race`.
- [ ] **Step 4:** Commit by pathspec. **Announce S0**: the commit message's first line ends
  `— S0: phases 3, 4, 5, 7 may code against hubclient.NewFake()`.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubclient/... -race -count=1                        # ok (two packages)
go test ./control-plane/internal/hub/hubclient/conformance/ -run TestFakeConforms -v -count=1 2>&1 | grep -c "^    --- PASS"
# ≥ 40 — the number of rows; record the exact number in the commit message
grep -rn "not_in_fake_yet" control-plane/internal/hub/hubclient/                          # no output
```

#### What this task does not claim

- That the real Hub passes these rows. Each server task does that for its group, and task 34 runs
  the suite against the Hub through the HTTP client.
- Concurrency semantics (two writers racing): the fake serialises under one mutex. The real
  Hub's race behaviour is task 19's and task 26's database tests.

#### As built (2026-09-26, commit 277e6c9a)

- **D1 — `Env` gains three fields** (carried/task-05.md items 1 and 13; the quota): `OrgAdmin` (an
  org admin in no team; `nil` skips its column), `Erase(briefID, reason)` (the fake's `EraseBrief`
  through an org-admin view; on a real Hub task 27's route; `nil` skips the erased cases, and the
  row logs it), `BlobQuota` (the Hub's `PANACEA_APP_HUB_PROJECT_QUOTA_BYTES`; 0 skips
  `designs/blob_quota`). The fake gains `SetProjectQuota` for it (additive; task 04's surface is
  otherwise unchanged).
- **D2 — one file per group.** `conformance.go` holds `Env`, `Group`, `Run` and the shared helpers;
  the rows are in `teams.go`, `projects.go`, `briefs.go`, `refs.go`, `designs.go`, `releases.go`,
  `evidence.go`, `feed.go`. `fake_test.go` builds the part's `Env` in `newFakeEnv(t, clock)`, with
  the clock behind a mutex (`-race`: `Advance` and the fake's reads share it).
- **D3 — 83 rows, not "≥ 40".** The part's table is 48 rows. Carried items 4–13 add 20: teams 11
  (items 4, 5, 8–10, 12), briefs 5 (`self_rebind_is_noop`, `rebind_archived_first`,
  `resolve_zero_keys`, `list_cursor_is_this_projects`, `revisions_cursor`), and item 13 as one
  `org_admin_is_a_non_member` row in each of teams, projects, briefs and refs (re-review 3's m1 is
  in the teams one). The other 15 are rules of "What the fake must do" that no table row held:
  `blob_too_large`, `blob_roles`, `blob_quota`, `approval_of_an_older_revision`,
  `approvals_list_and_get`, `session_link_checks`, `reserve_refusals`, `one_release_per_run`,
  `abandon_from_reserved_only`, `join_rules`, `list_filters_and_order`, `evidence_checks`,
  `more_when_limit_cuts`, `audit_only_kinds_never_appear`, `every_write_is_its_events` (rule 6:
  every write of both parts, §8's kind, subject, team, project and exact data keys, in order).
- **D4 — where the part is silent, the server tasks' text is followed**, so the rows hold the
  Hub's order too: `PutBlob` in task 21's order (the digest's form 400 `field: sha`, the type, the
  size 413 with `limitBytes` before a byte is read, the body rewound, read and hashed 422 — all
  before the project's 404 and 403 and the 409 quota; a re-upload answers the stored type);
  approvals in task 22's order with the count, duplicate and manifest 400s inside check 5; a
  session link's brief key erased or not; `Reserve` by task 23's field table (422
  `key_unresolvable` names `details.briefId`); confirm, abandon and join by task 24 (a run's
  confirm with no or a bad `pipelineRun` is 400 `field: pipelineRun`; a run's confirm naming a
  `deploymentId` is 400); `release.deployment_joined` carries `{"deploymentId","environment"}`
  (task 24; contract §8 lists `deploymentId` alone — task 44a's to reconcile); `release.reserved`
  spells a null as `""` (task 23); binding emits `team.invite_bound` then `team.member_set`
  (task 16a), `CreateTeam` `team.created` then `team.member_set` (task 15); a `SetMember` to the
  same role and a second revoke change nothing and emit nothing; every erase emits `brief.erased`,
  a second one too (task 27), with no `reason`; the feed's `after` is a canonical decimal or
  `latest`, `limit` 0 is 200, and `next` is the head unless `more` (task 26).
- **D5 — expiry is swept at every call**, not rendered lazily: the fake's first call after
  `expiresAt` marks the reservation `expired` and appends `release.expired` with a null actor, as
  task 24's janitor does within 60 s. Through the interface the two are the same, and the feed
  row sees the event.
- **D6 — audit-only kinds.** The fake's log holds `auth.user_provisioned` and `blob.uploaded`
  (they take sequence numbers, as on the Hub, and never reach the feed); it has no audit route, no
  blob collection and no 403 audit, so `audit.read`, `blob.deleted` and `auth.forbidden` never
  occur in it.
- **Carried items 2 and 3.** An erased revision's content is asserted as `len == 0 || "null"`
  (`erasedContent`); every row that asserts an order makes its records on distinct ticks (`tick`
  moves `Advance` by 1 ms, or waits 2 ms on a Hub with none).
