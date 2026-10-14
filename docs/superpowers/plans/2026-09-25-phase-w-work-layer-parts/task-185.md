### Task 185: Entra group to team mapping (L-238)

Stream **WL**, a **Hub delta** on team membership, about 2.5 hours (an estimate, not a
measurement). Depends on task **174** (W5's proof). 05 L-238 (*Entra group to team mapping*,
slice `later`) with L-237 (users arrive by first sign-in, so just-in-time provisioning is
inherent). Hub anchors were read in the `feat/hub` worktree at `c9b7da47` (cited
`hub-contract:N`). `server/`, `store/` and `migrate/` are **planned, not on `feat/hub`**, so every
name taken from them is **UNVERIFIED** until they land (rule H1).

**The skeleton row's UNVERIFIED is answered: the Hub contract has no group mapping.** Team access
is `team_members.role` (`viewer`·`member`·`admin`, `hub-contract:324-333`), set by `PUT` and
`DELETE /v1/teams/{teamId}/members/{userId}` (matrix rows 8-9, `:570-571`; routes `:679-685`),
events `team.member_set {role}` and `team.member_removed {}` (`:1140-1141`). A user exists only
after first sign-in, through `resolve_principal` on `(org_id, entra_oid)` (`:305-308`,
`:1953-1972`). The Hub API registration has `groupMembershipClaims: None` (`:543`), and the Hub
process has **no Microsoft Graph access in v1** (`:2083-2085`, `:2211`).

A team that already has an Entra group for its people should not re-enter them by hand. After
this task a team admin links a group to the team with a role, and once an hour the Hub brings the
team's membership in line with the group, never touching a member someone added by hand.

**Decisions made here, so nobody re-derives them:**

1. **Groups are read from Microsoft Graph with the Hub's own workload identity, never from token
   claims.** No group-overage problem, no claim to trust, and a person's rights do not depend on
   which token they last presented. This is a **Hub-contract change**: the Hub gains outbound
   HTTPS to `graph.microsoft.com` and an application permission, which `hub-contract:2083-2085`
   rules out in v1. It ships behind `PANACEA_APP_HUB_GRAPH=off|workload`, default `off`; with
   `off` the three routes answer 409 `directory_off` and the pass does nothing.
2. **Transitive members, users only.** The pass reads a group's transitive user members (`id`,
   `accountEnabled`), so a nested group's people join. Devices, service principals and guests
   without a `users` row are ignored as the rules below say.
3. **Manual always wins.** `team_members.source` is `'manual'` or `'group:gl-…'`. The pass never
   changes or removes a `manual` member. An admin's own `PUT` or `DELETE` on a group-sourced member
   makes the row `manual` (a `PUT`) or removes it; a removed person is re-added by the next pass
   only if still in the group, which is the link doing what it says. To keep someone out, remove
   the link or the person from the group.
4. **One person, many links: the highest role wins.** A person in two linked groups gets the
   higher of the two roles (`admin` > `member` > `viewer`), sourced to the link that grants it.
5. **Nobody is removed on doubt.** A Graph error, a 404 group, or an empty member list after a
   non-empty one removes **nobody**: the link records `lastError` (`graph_unreachable`,
   `graph_rejected`, `group_not_found`, `group_emptied`) and the next run retries. An emptied group
   stays `group_emptied` until an admin deletes the link or the group has members again.
6. **`last_admin` still holds.** A group-sourced last admin who leaves the group stays, and the
   link records `last_admin_kept` with the user id, so the team is never left without an admin.
7. **Pending joins are Go, not a changed definer function.** A group member with no `users` row
   gets a `pending_group_members` row. At first sign-in, when `resolve_principal` returns
   `created: true`, the Hub's request transaction (step 5-6 of `hub-contract:303-309`) consumes
   that person's pending rows, adds the memberships and appends their events. The scope proposed
   a `CREATE OR REPLACE` of the sign-in upsert; that would change one of the three reviewed
   `SECURITY DEFINER` functions (`hub-contract:2071`, T3) and could not append a chained event, so
   this part does not.
8. **Events are the existing kinds.** `team.member_set {role}` and `team.member_removed {}` with
   `actor: null` and two added scalars, `by` (the link's author) and `source` (`group:gl-…`),
   per 05 §3.6. Link changes are three new kinds: `team.group_linked {groupOid, role}`,
   `team.group_unlinked {groupOid, removed}` and `team.group_link_failed {groupOid, error}`, the
   last written once per change of `lastError`, not per run.

**Files** (the File map has no WL rows; these are this part's proposals):
- `control-plane/internal/hub/directory/graph.go` — the Graph reader over workload identity (new)
- `control-plane/internal/hub/directory/reconcile.go` — the pure plan `Reconcile` (new)
- `control-plane/internal/hub/directory/{graph,reconcile}_test.go` and `directory/graphtest/` — a
  Graph fake over `httptest` with golden pages (new)
- `control-plane/internal/hub/hubapi/work_groups.go` — `GroupLink`, the request, the codes (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_wl_groups.sql` — the tables and column (new;
  the next free number at sync time)
- `control-plane/internal/hub/store/work_groups.go`, `server/work_groups.go` — the three routes
  (new)
- `control-plane/internal/hub/server/janitor.go` ⚑ — one call, `syncGroupLinks(ctx, now,
  &report)`, after the janitor lock and before the per-org loop, as task 184's
  `syncRotationSources` (no Graph call inside a transaction)
- `control-plane/internal/hub/server/principal.go` ⚑ (the Hub's §5.1 step-5 code; its file name is
  **UNVERIFIED**) — consume pending rows when `created`
- `control-plane/internal/hub/server/teams.go` ⚑ (the Hub's member routes; **UNVERIFIED** name) —
  a `PUT` sets `source = 'manual'`; the reply's `Member` gains `source`
- `control-plane/internal/hub/server/work_matrix_rows.go` ⚑ — three rows (task 07's file)
- `control-plane/internal/hub/server/work_groups_integration_test.go` — `-tags integration` (new)
- `control-plane/cmd/hub/config.go` ⚑ — `PANACEA_APP_HUB_GRAPH`
- `control-plane/internal/hub/hubclient/work_intake.go` ⚑ (task 143's `WorkIntake`, beside apps
  and keys; **UNVERIFIED**, part 143 not written) and `fake_work*.go` ⚑ — the three methods; the
  fake stores links and never calls Graph
- `desktop/src/api/work.ts` ⚑ — `GroupLink` and three `WorkClient` members
- `desktop/src/components/admin/GroupLinks.tsx` (+ `.test.tsx`) — one section per team on task
  173's admin surface (`…-scopes/tasks-161-180.md:478-481`; part 173 not written) (new)

#### Wire shapes (`hubapi/work_groups.go`, complete)

```go
package hubapi

// GroupLink ties one Entra group to one team with one role. The Hub reads the
// group's transitive user members once an hour and never writes to Entra.
type GroupLink struct {
	ID           string  `json:"id"`           // gl-…
	TeamID       string  `json:"teamId"`       // tm-…
	GroupOID     string  `json:"groupOid"`     // the group's object id, a UUID
	DisplayName  string  `json:"displayName"`  // Graph's, refreshed each sync
	Role         string  `json:"role"`         // "viewer" | "member" | "admin"
	CreatedBy    string  `json:"createdBy"`    // u-…
	CreatedAt    string  `json:"createdAt"`    // RFC 3339 UTC
	LastSyncedAt *string `json:"lastSyncedAt,omitempty"`
	LastError    *string `json:"lastError,omitempty"` // one of the GroupErr* codes
	KeptAdmin    *string `json:"keptAdmin,omitempty"` // u-… held by last_admin, see LastError
}

// PutGroupLinkRequest is the body of PUT /v1/teams/{teamId}/group-links/{groupOid}.
type PutGroupLinkRequest struct {
	Role string `json:"role"`
}

const (
	GroupErrUnreachable = "graph_unreachable" // network error, timeout, 5xx, 429
	GroupErrRejected    = "graph_rejected"    // 401 or 403: the Hub's permission
	GroupErrNotFound    = "group_not_found"   // 404 on a sync, after the link was made
	GroupErrEmptied     = "group_emptied"     // zero members after a non-empty sync
	GroupErrLastAdmin   = "last_admin_kept"   // a leaver kept because they are the last admin
)

// Codes (appended to hubapi/codes.go ⚑):
//	404 group_not_found  — PUT: Graph finds no such group in the org's tenant
//	409 directory_off    — PANACEA_APP_HUB_GRAPH=off
//	422 field "role"     — not one of the three roles
//	422 field "groupOid" — not a UUID

// Member (hub-contract §7.3) gains one read-only field:
//	Source string `json:"source"` // "manual" | "group:gl-…"
```

| # | Verb | Path | Role | Reply | Event |
|---|---|---|---|---|---|
| G1 | PUT | `/v1/teams/{teamId}/group-links/{groupOid}` | A | `201 GroupLink` (new) or `200` (role changed) | `team.group_linked` |
| G2 | GET | `/v1/teams/{teamId}/group-links` | A | `200 {"items": GroupLink[]}` | — |
| G3 | DELETE | `/v1/teams/{teamId}/group-links/{groupOid}` | A | `204` | `team.group_unlinked`, then one `team.member_removed` per member the link alone added |

`G2` is admin-only because a link names a directory group, which a viewer has no need to see. A
`PUT` resolves the group first (`GET /groups/{oid}?$select=id,displayName` in the org's tenant)
and stores nothing when Graph says 404. A role change on `G1` takes effect at the next sync, not in
the request (no Graph read of members in a request).

#### The migration (`migrate/sql/NNNN_work_wl_groups.sql`, complete)

```sql
-- Task 185: Entra group to team mapping. RLS on org_id as every Hub table (hub-contract §10.3).
CREATE TABLE team_group_links (
  id             text        PRIMARY KEY,                      -- gl-…
  org_id         text        NOT NULL REFERENCES orgs (id),
  team_id        text        NOT NULL REFERENCES teams (id),
  group_oid      uuid        NOT NULL,
  display_name   text        NOT NULL CHECK (length(display_name) <= 256),
  role           team_role   NOT NULL,
  created_by     text        NOT NULL REFERENCES users (id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  last_synced_at timestamptz NULL,
  last_error     text        NULL CHECK (last_error IS NULL OR last_error IN ('graph_unreachable',
                   'graph_rejected', 'group_not_found', 'group_emptied', 'last_admin_kept')),
  kept_admin     text        NULL REFERENCES users (id),
  last_seen      integer     NOT NULL DEFAULT 0,               -- members at the last good read
  UNIQUE (team_id, group_oid)
);
CREATE INDEX team_group_links_due ON team_group_links (org_id, last_synced_at NULLS FIRST);

ALTER TABLE team_members
  ADD COLUMN source text NOT NULL DEFAULT 'manual'
    CHECK (source = 'manual' OR source ~ '^group:gl-[A-Za-z0-9]+$');

CREATE TABLE pending_group_members (
  org_id    text      NOT NULL REFERENCES orgs (id),
  team_id   text      NOT NULL REFERENCES teams (id),
  entra_oid uuid      NOT NULL,
  link_id   text      NOT NULL REFERENCES team_group_links (id) ON DELETE CASCADE,
  role      team_role NOT NULL,
  PRIMARY KEY (team_id, entra_oid, link_id)
);

-- RLS: the same org_id policy as §10.3's tables, for both new tables (the policy text is the
-- Hub's; copy it from 0001_init.sql's loop at hub-contract:1911, adding the two names).
GRANT SELECT, INSERT, DELETE ON team_group_links, pending_group_members TO hub_app;
GRANT UPDATE (display_name, role, last_synced_at, last_error, kept_admin, last_seen)
  ON team_group_links TO hub_app;
GRANT UPDATE (role, source) ON team_members TO hub_app;
```

The last line widens the Hub's `GRANT UPDATE (role), DELETE ON team_members` (`hub-contract:2014`)
by one column: without it the pass could not record a source, and a manual `PUT` could not turn a
group-sourced row manual. This is the Hub delta's one grant change; it goes to the `feat/hub`
owner with the sync-merge (rule H1). `team_role`, `orgs`, `teams` and `users` are the Hub's
(`hub-contract:1508-1517`).

#### The pure plan (`directory/reconcile.go`, complete)

```go
package directory

// Role order: admin > member > viewer.
func Rank(role string) int

// Person is one transitive user member of a linked group, as Graph returns it.
type Person struct {
	OID     string // the user's Entra object id
	Enabled bool   // accountEnabled
}

// LinkRead is one link and what the last Graph read said. Err non-empty means
// the read failed or was refused (decision 5) and People is ignored.
type LinkRead struct {
	LinkID, Role string
	People       []Person
	Err          string // "" or a hubapi.GroupErr* code
}

// Current is one team_members row plus the user's oid.
type Current struct {
	UserID, OID, Role, Source string // Source "manual" | "group:gl-…"
	AddedAt                   time.Time
}

// Plan is the whole change for one team; the store applies it in one tx.
type Plan struct {
	Set       []Change // add or change a group-sourced member
	Remove    []string // user ids, group-sourced only
	Pending   []Pending
	KeptAdmin map[string]string // link id → user id, kept by last_admin
}
type Change struct{ UserID, Role, Source string }
type Pending struct{ OID, LinkID, Role string }

// Reconcile is pure. users maps oid → user id for the org's users rows.
//   - Want: for every enabled person in a link read with Err == "", the highest
//     Rank role over all such links, sourced to the first link (by LinkID) that
//     grants it (decision 4).
//   - A wanted oid with a users row: Set when absent, or when group-sourced with a
//     different role or source. A manual row is never in Set (decision 3).
//   - A wanted oid with no users row: Pending.
//   - A group-sourced row not wanted: Remove, unless its source link failed this
//     run (Err != "") — then it stays (decision 5).
//   - After Remove and Set, if the team would have no admin, the removed or
//     demoted admin with the oldest AddedAt stays as admin and KeptAdmin records
//     it against its source link (decision 6).
func Reconcile(current []Current, reads []LinkRead, users map[string]string) Plan
```

`Reconcile` reads no clock and no database, so every rule is a table row.

#### The Graph reader (`directory/graph.go`, contract complete)

Written from Microsoft Graph's public v1.0 documentation, **not** from a tenant run; every path,
field and permission below is **UNVERIFIED** until a tenant answers (step 1 records the doc pages
and the date read).

```go
package directory

// Graph reads groups with the Hub's workload identity: a federated token for
// https://graph.microsoft.com/.default, exchanged as the Hub already does for
// Postgres (hub-contract:2162, PANACEA_APP_HUB_DB_AUTH=entra). Base is a field
// so graphtest points it at httptest.
type Graph struct {
	Base   string // "https://graph.microsoft.com/v1.0"
	Token  func(ctx context.Context, tenant string) (string, error)
	Client *http.Client // 10 s timeout
}

// Group is GET {Base}/groups/{oid}?$select=id,displayName. 404 → GroupErrNotFound.
func (g *Graph) Group(ctx context.Context, tenant, oid string) (displayName string, err error)

// Members is GET {Base}/groups/{oid}/transitiveMembers/microsoft.graph.user
// ?$select=id,accountEnabled&$top=999, following @odata.nextLink to the end.
// Any failed page fails the whole read: a partial list is never returned,
// because a partial list would remove the people on the missing pages.
func (g *Graph) Members(ctx context.Context, tenant, oid string) ([]Person, error)
```

Status mapping: 401, 403 → `graph_rejected`; 404 → `group_not_found`; 429, 5xx, a transport
error or the timeout → `graph_unreachable`. `Retry-After` is not honoured inside a run; the next
hourly run is the retry. The permission the reader needs is Graph's `GroupMember.Read.All`
application permission; whether `accountEnabled` is returned under it alone, or needs
`User.Read.All` too, is **UNVERIFIED**.

#### The sync (`server/janitor.go` ⚑ and `server/work_groups.go`)

`syncGroupLinks(ctx, now, &report)` runs once per `Sweep`, holding the session-level janitor lock,
before the per-org loop, as task 184's `syncRotationSources` does and for the same reason (no
network call inside a transaction). Per org, per team with a due link (last sync `NULL` or an hour
old):

1. A read transaction lists the team's links, its `team_members` with `users.entra_oid`, and the
   org's `entra_tenant_id`; commit.
2. With no transaction open, `Members` for each link. A read of zero people for a link whose
   `last_seen > 0` becomes `Err: group_emptied`.
3. A write transaction (`hub.org_id` set, `store.LockHead` as task 121) re-reads the team's
   members `FOR UPDATE`, maps oids to user ids, runs `Reconcile`, and applies the plan: each `Set`
   is an upsert with `source`, each `Remove` a delete, each `Pending` an upsert, and each link's
   `last_synced_at`, `last_error`, `kept_admin`, `last_seen` and `display_name`. Pending rows whose
   oid now has a `users` row are applied as `Set` and deleted (the heal for a sign-in during the
   run).
4. Events, one per changed record: `team.member_set {role, by, source}` per `Set`,
   `team.member_removed {by, source}` per `Remove`, `team.group_link_failed {groupOid, error}` when
   a link's `last_error` changed to a non-null value. `actor` is null on all of them.

`SweepReport.GroupSynced` counts teams written. A failure in one team never stops the others.

#### First sign-in (`server/principal.go` ⚑)

After step 6 of `hub-contract:303-309`, when `resolve_principal` returned `created: true`, the
same transaction runs `store.ConsumePending(ctx, tx, org, userID, oid)`: it deletes the person's
pending rows and, per team, upserts the highest-ranked role with its link's `source`, appending one
`team.member_set` per team with `actor: null`. It runs before the handler, so the person's first
request already sees the team.

#### The desktop (`GroupLinks.tsx`)

One section per team the person admins, on task 173's admin surface: each link's group name, its
role, and **Last synced 14:02** (`tabular-nums`, a fact, never *next sync in …*); a `lastError`
shows its words (*Graph could not be reached; nobody was removed*, *the group is empty; nobody was
removed*, *Priya kept as the last admin*). *Link a group* takes a group object id and a role. A
team member row whose `source` is a group reads *from Engineering-AU* in the members list; removing
one says *They return at the next sync while they are in Engineering-AU.* The section is drawn only
for a team admin with `PANACEA_APP_HUB_GRAPH` on (a `directory_off` reply draws one line, *Group
links are off for this Hub*).

#### `desktop/src/api/work.ts` ⚑, appended (complete)

```ts
export type TeamRole = "viewer" | "member" | "admin";
export type GroupLinkError =
  | "graph_unreachable" | "graph_rejected" | "group_not_found" | "group_emptied" | "last_admin_kept";
export interface GroupLink {
  id: string; teamId: string; groupOid: string; displayName: string; role: TeamRole;
  createdBy: string; createdAt: string;
  lastSyncedAt?: string; lastError?: GroupLinkError; keptAdmin?: string;
}
// WorkClient gains (relayed like task 125's rows; the verbs and paths are G1-G3):
//   putGroupLink(teamId: string, groupOid: string, role: TeamRole): Promise<GroupLink>;
//   groupLinks(teamId: string): Promise<GroupLink[]>;
//   deleteGroupLink(teamId: string, groupOid: string): Promise<void>;
// The member type gains: source: "manual" | `group:gl-${string}`
```

Records are fetched on open and dropped on close, never put in the replica (task 173's rule).

#### Tests to write first, part 1 — `directory/*_test.go` (pure and `graphtest`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAMemberWithARowJoins` | a person with a `users` row and no membership → `Set` with the link's role and `group:gl-1` | a missing add |
| `TestAMemberWithoutARowIsPending` | no `users` row → `Pending`, not `Set` | a lost pending row |
| `TestLeavingRemovesOnlyGroupSourced` | two people gone from the group, one `group:gl-1`, one `manual` → `Remove` holds the first only | removal of manual members |
| `TestADisabledAccountIsRemoved` | `Enabled: false` → `Remove` | keeping a disabled account |
| `TestAFailedReadRemovesNobody` | `Err: graph_unreachable` on the only link → empty `Remove` | an outage that empties a team |
| `TestTheLastAdminStays` | the only admin is group-sourced and leaves → not in `Remove`, `KeptAdmin[gl-1]` is them | a bypassed `last_admin` |
| `TestTheHighestRoleWins` | one person in a `viewer` link and an `admin` link → `Set` role `admin`, source the admin link | a role that depends on link order |
| `TestManualIsNeverChanged` | a `manual` `viewer` in an `admin` link → not in `Set` | a pass that overrides a hand edit |
| `TestNestedGroupsAreRead` | `graphtest` serves a user reached only through a nested group, over two pages → `Members` returns both pages' people | a direct-members-only read, or a first-page-only read |
| `TestAFailedSecondPageFailsTheRead` | page 2 answers 503 → error `graph_unreachable`, no people | a partial list that removes the missing page |

#### Tests to write first, part 2 — `server/work_groups_integration_test.go` (`//go:build integration`)

Against the compose stack with `graphtest` as Graph and `PANACEA_APP_HUB_GRAPH=workload`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLinkingJoinsMembersAtTheNextSweep` | `PUT` a link, `RunJanitorOnce` → two `team_members` rows with `source`, two `team.member_set` with `actor` null and `by` the admin | a missing add or an unattributed event |
| `TestAPendingPersonJoinsAtFirstSignIn` | a group member with no user signs in through `devissuer` → their first `GET /v1/teams/{teamId}` answers 200 | a pending row that is never consumed |
| `TestASecondSweepWritesNoEvent` | two sweeps an hour apart, no Graph change → zero new events | a non-idempotent pass |
| `TestAGraphOutageRemovesNobody` | Graph 503 then a deleted group → members unchanged, `lastError` set, one `team.group_link_failed` per change | an outage that empties a team |
| `TestAnEmptiedGroupRemovesNobody` | a group that had two people returns none → both stay, `lastError: group_emptied` | a silent mass removal |
| `TestUnlinkRemovesWhatTheLinkAdded` | `DELETE` the link → its group-sourced members gone in the same tx, manual ones stay, one `team.group_unlinked` | an unlink that leaves ghost members |
| `TestAManualPutMakesTheRowManual` | an admin `PUT`s a group-sourced member's role → `source: manual`; the next sweep leaves it | a hand edit reverted by the pass |
| `TestAMemberCannotLink` | a `member`'s `PUT` → 403; a `viewer`'s `GET` → 403 | a missing matrix row |
| `TestGraphOffIs409` | `PANACEA_APP_HUB_GRAPH=off` → all three routes 409 `directory_off`, the pass calls `graphtest` zero times | a directory read the operator did not enable |
| `TestAnUnknownGroupIs404` | `PUT` with an oid `graphtest` does not know → 404 `group_not_found`, no row | a link to nothing |

#### Tests to write first, part 3 — `desktop/src/components/admin/GroupLinks.test.tsx`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `shows the last sync as a fact` | *Last synced 14:02*; `expectNoPrediction(container)` | a *next sync in* countdown |
| `says nobody was removed on an error` | `lastError: graph_unreachable` → the words, exactly | an error that reads like an empty team |
| `hidden from a member` | a non-admin sees no section | an action drawn that the Hub refuses |
| `fetches on open, keeps nothing` | closing the section leaves no link in the replica store | Hub data persisted by the client |

#### Steps

- [ ] **Step 1: confirm the base** (rule H1): `feat/hub` sync-merged; tasks 07, 143 and 173
  committed; record the Graph doc pages and the date read; raise the grant change and the Graph
  permission with the `feat/hub` owner.
- [ ] **Step 2: pure tests first**, then `reconcile.go`; `graphtest`, then `graph.go`.
- [ ] **Step 3: the migration**, run against the compose stack.
- [ ] **Step 4: integration tests**, then the store, the routes, the matrix rows, the sync, the
  sign-in consumption and the manual `PUT` rule. Run until green.
- [ ] **Step 5: the fake and one conformance row** (a `PUT` link reads back on `GET`).
- [ ] **Step 6: the desktop**, part 3 first.
- [ ] **Step 7: acceptance**, then commit by path, each message ending with the `Co-Authored-By`
  line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/directory/... -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# expect: 10
go test ./control-plane/internal/hub/directory/... -race -count=1
# expect: ok  .../internal/hub/directory
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(LinkingJoins|APendingPerson|ASecondSweepWritesNo|AGraphOutage|AnEmptiedGroup|UnlinkRemoves|AManualPutMakes|AMemberCannotLink|GraphOffIs409|AnUnknownGroupIs404)' \
  2>&1 | grep -c "^--- PASS"
# expect: 10
go test -tags integration ./control-plane/internal/hub/... -race -count=1 -run 'Matrix|Conformance'
# expect: ok for every package
npm --prefix desktop test -- components/admin/GroupLinks
# expect: 4 tests pass
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- That KPMG IT grants the Hub's workload identity Graph's `GroupMember.Read.All` application
  permission (and `User.Read.All` if `accountEnabled` needs it). That is a change to hub-contract
  §6.5's registrations and to phase 8's task 09, **UNVERIFIED**, for the owner and IT to decide.
  Until then `PANACEA_APP_HUB_GRAPH=off` and the feature is dark.
- SCIM: users arrive by first sign-in, so just-in-time provisioning is inherent (05 §1 L-237).
- Changes faster than hourly; a real tenant run; Graph's shapes, which are from public docs.
- Group-to-org-admin mapping: `Hub.OrgAdmin` stays an app role on the token (`hub-contract:331`).
- That a removed person is kept out while still in a linked group (decision 3 says they return).

**Parity.** Closes **L-238** (slice `later`); the command and output are recorded by task 187.

#### Gaps found while writing this part

1. **The scope's `CREATE OR REPLACE` of the sign-in upsert** would change a reviewed `SECURITY
   DEFINER` function and could not append a chained event; decision 7 consumes pending rows in Go
   instead, in the same request transaction.
2. **The grant.** `hub-contract:2014` grants `UPDATE (role)` on `team_members`; `source` needs one
   more column. Raised with the Hub delta (rule H1).
3. **The Hub contract rules out Graph in v1** (`:2083-2085`, `:2211`); this task is the first
   outbound call beyond `login.microsoftonline.com` and needs a threat-model row (T-series) for a
   Graph read that removes members. Proposed text: *a wrong group or a spoofed Graph reply removes
   people; control: removal only on a complete, successful, non-empty read; manual rows never
   touched; last admin kept*.
4. **The scope routes `GroupLink` through task 143's `WorkIntake`**, which is about outside intake;
   a team-membership method fits the team client better. Kept as the scope says; part 143 is not
   written.
5. **The scope's reconcile "reads the group's transitive members"** in the pass; the pass also
   needs the org's `entra_tenant_id` to scope the Graph token, which the Hub has (`orgs`).
6. **The File map has no WL rows**: `directory/`, `server/work_groups.go`, `store/work_groups.go`,
   `admin/GroupLinks.tsx` and the migration should be listed.
