### Task 66: Team and project quotas and the spend split, server side

**Slice:** TM2. **Focused time:** 2.5 h. **dependsOn:** 37 (the spend split, the `capUSD` ask),
43 (the Hub's team policy routes 74, 75, 79 and `teampolicy.go`), P6-06 (`budget.Limits`,
`budget.Slice`), P6-09 (`usage.Store`, `keysmith.key_spend`); 03 (`hubapi.TeamPolicy`,
`hubapi.Quotas`, `hubapi.UsageRow`) and 05 (the client-credentials caller rule) through them.
**Seam:** S6.2 (phase 6's budgets), consumed and extended.

OD-24 says budgets are on by default. `05-design.md` Q3 (`:1330`) gives the shape: *"The org
admin sets a ceiling per person; a team admin sets project budgets inside it; both ship non-zero
defaults. Coordination counts against the member whose Orchestrator spent it, shown per
counterpart."* P4 (`:106`) says every token is attributed to its owner, team and project against a
non-zero default quota **at the gateway**, and `07` I3 says why the default matters: an unset quota
is the org pool by another name. Q3 is the design's recommendation; `OD-24` accepts "budgets on
by default" and nothing narrower, so the ceiling-then-budget order is **the design's, not an owner
decision**, and this part says so where it matters.

Task 01 left three things to this task: routes 76 and 77 (`task-01.md:281-282`), the caller rule
for route 76 as a §5.10 rule 9 (`task-01.md:490-495`), and the usage day's time zone
(`task-03.md:947-949`). Six decisions close them and the scope, so nobody re-derives them.

**(a) The unit is the US cent, everywhere.** LiteLLM records spend in US dollars
(`task-06.md:319-320` on `feat/phase-6-gateway`), the Hub's quotas and `costCents` are integer
cents (`task-03.md:892-895`, `:938`), and no exchange-rate source exists in any plan. So every
cent in team mode is a **US** cent, and every surface says `US$`. Converting to A$ would print a
number the gateway does not enforce. Task 37's owner line *"up to A$0.50"* (`task-37.md:51-52`)
disagrees; it is listed in the gaps and task 67 renders `US$`.

**(b) Two periods, one enforcement point.** Phase 6 enforces **monthly** person, project and team
limits (`task-06.md:313-317`); the team policy carries **daily** member and project quotas
(`task-03.md:892-895`). Neither replaces the other. keysmith's mint takes the least of phase 6's
monthly `Slice` and this task's `DaySlice`, so the tighter of the two always binds. The daily
quotas are enforced **by keysmith at mint** (the key's `max_budget`), not by Fleet: a laptop
session on a team project (local mode stays, OD-20) mints through the same keysmith, so a
Fleet-only cap would leave a hole. LiteLLM's native user and team budgets stay phase 6's monthly
ones; this task writes neither, because the same LiteLLM user spans a person's solo and team
projects and a daily `budget_duration` on it would cap their solo work too.

**(c) The day is the UTC day.** The gateway's rollup boundary is the UTC month
(`task-09.md:27`, `:270-272` there), and task 37's `SplitView.Day` is UTC (`task-37.md:434`).
Task 03 chose `Australia/Sydney` provisionally and left it to this task; this task changes the
comment to UTC. A key's spend belongs to the UTC **day it was minted in**, as P6-09's decision (a)
files it under the month it was minted in: a key minted at 23:30 counts wholly against that day,
and at most one key TTL (one hour) of each session lands on the previous day.

**(d) The org ceiling is a Hub record of its own.** Nothing in task 01's contract holds an
org-wide number; the team policy is per project (`task-01.md:469-478`). This task adds one record
per org, `OrgQuotas`, with two ceilings (per member per day, per project per day), two new routes
80 and 81, and a contract §7.27. Its `PUT` needs `Hub.OrgAdmin` (contract `:320`); a team admin's
`PUT` of the team policy (route 75, task 43) is refused when either quota is above the
corresponding ceiling. Lowering a ceiling below a project's quota **does not rewrite** the
project's record: keysmith uses the lower of the two, and the read model shows both (as task 37
does for turns, `task-37.md:488-491`). A per-person ceiling that differs person by person is not
built (see *does not claim*).

**(e) Route 76's caller is the gateway's own workload identity, rule 9.** It is the third caller
task 01 foresaw (`task-01.md:493-494`). It is admitted on exactly three routes (74 to read a team
policy, 80 to read the org ceiling, 76 to post roll-ups), carries no delegation, and is never a
principal. Routing the roll-up through Fleet instead was considered and refused: it would make
Fleet read keysmith's spend store, and a Fleet outage would then stop the gateway from reading
the quotas it must enforce.

**(f) "Per member and per counterpart" is served from the Hub's roll-ups, by the local control
plane.** Task 37's `Split` serves the owner's **own** figures from Fleet, exactly. The team admin's
view (task 67) needs **every** member's, which only the Hub holds (route 77: an A sees every row,
an M their own, `task-01.md:496`). So `cp/internal/api/teamquotas.go` composes routes 74, 77 and
80 with the Hub's agent records (task 06, for each `ag-…`'s class) into one `QuotaView`, which
ST.8's team read model carries to the desktop. A roll-up is posted every 10 minutes for the
current day, replacing the day's earlier post (`task-01.md:492-493`), so the view is at most 10
minutes behind the gateway; the owner's own line from task 37 stays exact.

**Files:**

*Hub delta, built after the `feat/hub` sync-merge. Every Hub server and store path is Hub task
14's write shape and task 43's `teampolicy.go`, **UNVERIFIED until merge**.*
- `control-plane/internal/hub/hubapi/quota.go` — `OrgQuotas`, `PutOrgQuotasRequest`,
  `DefaultOrgQuotas`, `UsageRollupRows` bounds (new)
- `control-plane/internal/hub/hubapi/team.go` — `UsageRow`'s two token fields become `*int64`;
  `UsageRollupRequest.Day` is the UTC day (modify, task 03's file, one anchored hunk)
- `control-plane/internal/hub/migrate/sql/00NN_quotas_usage.sql` — `hub.org_quotas`,
  `hub.usage_rollups`, numbered after the Hub's last migration at merge (new)
- `control-plane/internal/hub/store/quotas.go`, `.../store/usage.go` (new)
- `control-plane/internal/hub/server/orgquotas.go` — routes 80, 81 (new)
- `control-plane/internal/hub/server/usage.go` — routes 76, 77 (new)
- `control-plane/internal/hub/server/teampolicy.go` — route 75 refuses a quota above the ceiling
  (modify, task 43's file, one anchored hunk)
- `control-plane/internal/hub/server/auth.go` — §5.10 rule 9, the gateway caller (modify, task 05's
  delta on Hub task 13's file)
- `control-plane/internal/hub/server/matrix_integration_test.go` — rows for 76, 77, 80, 81 and the
  gateway principal (modify, task 43's rows on Hub task 16b's file)
- `control-plane/internal/hub/server/{orgquotas,usage,teampolicy_ceiling}_test.go` (new)
- `docs/superpowers/adr/ADR-H1-hub-architecture.md` §5.10 rule 9;
  `docs/superpowers/contracts/2026-09-23-hub-contract.md` §5.3 rows 80–81, §7.25's caller
  sentence, a new §7.27 (modify, task 01's amendments)

*Phase 6 delta, built after the `feat/phase-6-gateway` sync-merge (P6-06, P6-07, P6-09,
**UNVERIFIED until merge**).*
- `control-plane/internal/budget/daily.go`, `daily_test.go` — `DayLimits`, `DaySpent`,
  `DayPolicy`, `DaySlice`, `UTCDay` (new; pure, standard library only, as `budget.go`)
- `control-plane/internal/usage/usage.go` — `Store.SumDay`, `Reader.SpentDay`, `Store.DayRows`
  (modify, P6-09's file, three anchored hunks)
- `control-plane/internal/binding/migrations/00NN_key_spend_day.sql` — `minted_day`, `agent_id`,
  `route`, `counterpart` on `keysmith.key_spend` (new, after P6-09's `0002_key_spend.sql`)
- `control-plane/internal/keysmith/keysmith.go` — `Deps.Days`; `mint` takes the least of `Slice`
  and `DaySlice` (modify, P6-07's file, one anchored hunk after `budget.Slice`)
- `control-plane/internal/teamquota/{hubday.go,rollup.go}`, `teamquota_test.go` — the Hub-backed
  `DayPolicy` and the 10-minute roll-up poster (new)
- `control-plane/cmd/keysmith/main.go` — builds both, `PANACEA_APP_GATEWAY_HUB_*` (modify, P6-08's
  wiring, one anchored hunk; the file name is **UNVERIFIED** against P6-08)

*Local control plane, on `feat/team-mode`.*
- `control-plane/internal/api/teamquotas.go` — `QuotaView`, the composer, the two local routes
  (new; joins ST.8, task 52's `teamview.go` is not edited)
- `control-plane/internal/api/teamquotas_test.go`, `testdata/team-quotas/*.json` (new)

**Interfaces:**
- Produces:
  - Hub routes 80 `GET` and 81 `PUT /v1/orgs/{orgId}/quotas`; routes 76 and 77 built;
  - `budget.DayLimits`, `budget.DaySlice`, `budget.DayPolicy`, consumed by keysmith's mint;
  - `teamquota.HubDay` (a `budget.DayPolicy`) and `teamquota.Rollup`, consumed by `cmd/keysmith`;
  - `api.QuotaView` on `GET /v1/projects/{projectId}/team/quotas` and the team admin's
    `PUT /v1/projects/{projectId}/team/quotas`, consumed by task 67;
  - the refusal code `above_org_ceiling` as `details.reason` on 422 `bad_request`, consumed by 67.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 03: `hubapi.TeamPolicy`, `Quotas`, `PutTeamPolicyRequest`, `UsageRow`,
    `UsageRollupRequest`, `AgentStart.Role`, `AgentRoleOrchestrator` (`task-03.md:892-944`,
    `:149-150`, `:566-579`);
  - task 43: the route-75 handler in `server/teampolicy.go` (**task 43's part is not on disk**;
    its handler name is assumed `putTeamPolicy` and is renamed mechanically);
  - task 05: the app-only branch of `auth.go` that task 01's rule 1 describes
    (`task-01.md:114-117`; **task 05's part is not on disk**);
  - P6-06: `budget.Limits`, `budget.Slice`, `budget.Dimension` (`task-06.md:302-341`, `:435`);
  - P6-07: `Service.mint`, `Deps.Budgets`, `Deps.Spent`, `BudgetExhaustedError`
    (`task-07.md:827-846` there);
  - P6-09: `usage.Store.Sum`, `usage.Reader`, `keysmith.key_spend` (`task-09.md:436-450` there);
  - task 20: the mint's `Counterpart` field (`task-20.md:237-241`);
  - task 52: `TeamReader`'s project mapping and Hub client, phase 2's problem bodies
    (`task-52.md:614-619`).

#### The contract text (the words the Hub's docs gain)

**ADR-H1 §5.10 rule 9, "The gateway caller".** *A token whose `idtyp` is `app` is also accepted
when its `azp` equals `PANACEA_APP_HUB_GATEWAY_CLIENT_ID` **and** its `roles` contain
`Hub.Gateway`. It carries no delegation (the header is refused 403 `forbidden`, `details.required =
"fleet"`, as for a desktop), it is never a principal and never provisioned as a user, and it may
call exactly three routes: 74 (read a team policy), 80 (read its org's quotas) and 76 (post its
org's roll-ups). Any other route answers 403 `forbidden` with `details.required = "member"`. Its
org is the token's `tid` mapped as for every caller; a roll-up for another org is 403. Its audit
events are `v: 1` with `actorUserId = null` and `via = "gateway"`.*

**Contract §5.3, two rows after row 79:**

```text
| 80 | GET | `/v1/orgs/{orgId}/quotas` | any M of any team project in the org (desk); the gateway (rule 9) | — | T66 | §7.27 |
| 81 | PUT | `/v1/orgs/{orgId}/quotas` | org admin only (`Hub.OrgAdmin`) | — | T66 | §7.27 |
```

**Contract §7.25**, the sentence *"Its caller is open: … before any code calls the route."*
becomes: *"Its caller is the gateway's workload identity (§5.10 rule 9). `day` is the UTC day a
row's keys were minted in. `inputTokens` and `outputTokens` are `null` when the gateway could not
read them, never `0`."*

**Contract §7.27 Org quotas.** One record per org, created with the defaults on first read:

```json
{ "orgId": "…", "memberDailyCeilingCents": 5000, "projectDailyCeilingCents": 50000,
  "revision": 1, "updatedAt": "…" }
```

*Cents are US cents, the unit the gateway records. `PUT` (org admin, body `{"revision",
"memberDailyCeilingCents", "projectDailyCeilingCents"}`, a stale `revision` 409 `conflict`) refuses
with 422 `bad_request` and `details.field` any ceiling ≤ 0 or above 10,000,000 (US$100,000 a day).
Lowering a ceiling rewrites no project: the gateway enforces the lower of ceiling and quota. Route
75 refuses a `quotas.memberDailyCents` above `memberDailyCeilingCents`, or a
`quotas.projectDailyCents` above `projectDailyCeilingCents`, with 422 `bad_request`,
`details.field` naming the quota and `details.reason = "above_org_ceiling"`. One audit event,
`org.quotas.updated`, metadata only (the two numbers and the revision).*

#### `hubapi/quota.go` (complete)

```go
package hubapi

// ── §7.27 org quotas ────────────────────────────────────────────────────────

// OrgQuotas is the org admin's ceiling on every team project's daily quotas.
// Cents are US cents, the unit the gateway records (task 66 decision a).
type OrgQuotas struct {
	OrgID                    string `json:"orgId"`
	MemberDailyCeilingCents  int64  `json:"memberDailyCeilingCents"`
	ProjectDailyCeilingCents int64  `json:"projectDailyCeilingCents"`
	Revision                 int    `json:"revision"`
	UpdatedAt                string `json:"updatedAt"`
}

type PutOrgQuotasRequest struct {
	Revision                 int   `json:"revision"`
	MemberDailyCeilingCents  int64 `json:"memberDailyCeilingCents"`
	ProjectDailyCeilingCents int64 `json:"projectDailyCeilingCents"`
}

// DefaultOrgQuotas is what an org reads before its admin writes. PROPOSED,
// not decided: US$50 a member a day, US$500 a project a day. Non-zero (OD-24).
var DefaultOrgQuotas = OrgQuotas{MemberDailyCeilingCents: 5000, ProjectDailyCeilingCents: 50000}

// DefaultQuotas is a new team project's own quotas, inside the default
// ceilings: task 01's example figures (task-01.md:473), PROPOSED.
var DefaultQuotas = Quotas{MemberDailyCents: 2500, ProjectDailyCents: 20000}

const (
	MaxCeilingCents  int64 = 10_000_000 // US$100,000 a day: a typo guard, not a policy
	MaxRollupRows          = 5000       // §7.25
	ReasonAboveCeiling     = "above_org_ceiling"
)

// ValidCeiling is the single bound both ceilings share.
func ValidCeiling(c int64) bool { return c > 0 && c <= MaxCeilingCents }
```

**`hubapi/team.go`, the one hunk** (task 03's `UsageRow` and `UsageRollupRequest`, `:929-944`):

```go
	InputTokens       *int64         `json:"inputTokens"`  // null: the gateway could not read it
	OutputTokens      *int64         `json:"outputTokens"` // never 0 for unknown
	CostCents         int64          `json:"costCents"`    // US cents
}

type UsageRollupRequest struct {
	Day  string     `json:"day"` // YYYY-MM-DD, the UTC day the keys were minted in (task 66)
	Rows []UsageRow `json:"rows"`
}
```

#### `00NN_quotas_usage.sql` (complete)

```sql
-- One ceiling record per org (§7.27). Absent = DefaultOrgQuotas, written on first read.
create table hub.org_quotas (
    org_id                      text        primary key,
    member_daily_ceiling_cents  bigint      not null check (member_daily_ceiling_cents  between 1 and 10000000),
    project_daily_ceiling_cents bigint      not null check (project_daily_ceiling_cents between 1 and 10000000),
    revision                    integer     not null check (revision >= 1),
    updated_at                  timestamptz not null
);

-- The gateway's roll-up rows (§7.25). One post per (org, day) replaces the day's rows.
create table hub.usage_rollups (
    org_id              text    not null,
    day                 date    not null,
    user_id             text    not null,
    team_id             text    not null,
    project_id          text    not null,
    agent_id            text    not null check (agent_id ~ '^ag-'),
    route               text    not null check (route in ('global', 'au')),
    counterpart_user_id text,                         -- null: no single counterpart
    input_tokens        bigint  check (input_tokens  >= 0), -- null: not known
    output_tokens       bigint  check (output_tokens >= 0),
    cost_cents          bigint  not null check (cost_cents >= 0)
);

-- A primary key cannot hold an expression, so the row's identity is a unique index.
create unique index usage_rollups_row on hub.usage_rollups
    (org_id, day, project_id, user_id, agent_id, route, coalesce(counterpart_user_id, ''));
create index usage_rollups_project_day on hub.usage_rollups (project_id, day);
```

The Hub's schema name `hub.` is **UNVERIFIED until merge** (Hub task 07a); match it.

#### `server/auth.go`, the rule 9 hunk (task 05's app-only branch)

```go
// After task 05's Fleet branch. A second app-only identity, three routes only.
case claims.Idtyp == "app" && claims.Azp == s.cfg.GatewayClientID && hasRole(claims.Roles, "Hub.Gateway"):
	if r.Header.Get("X-Panacea-Delegation") != "" {
		return Principal{}, Refuse(403, hubapi.CodeForbidden, "Only Fleet names a delegation.",
			map[string]any{"required": "fleet"})
	}
	if !gatewayRoutes[r.Pattern] {
		return Principal{}, Refuse(403, hubapi.CodeForbidden, "Not a gateway route.",
			map[string]any{"required": "member"})
	}
	return Principal{Caller: CallerGateway, OrgID: s.orgOf(claims.Tid)}, nil
```

```go
// gatewayRoutes is rule 9's whole list, by the mux pattern, so a new route is
// closed to the gateway until it is added here and to the matrix test.
var gatewayRoutes = map[string]bool{
	"GET /v1/projects/{p}/team-policy":   true, // 74
	"GET /v1/orgs/{orgId}/quotas":        true, // 80
	"POST /v1/orgs/{orgId}/usage-rollups": true, // 76
}
```

`s.cfg.GatewayClientID` is `PANACEA_APP_HUB_GATEWAY_CLIENT_ID`, read by the Hub's config beside
task 05's `PANACEA_APP_HUB_FLEET_CLIENT_ID`; empty means rule 9 is off and every gateway token is
refused as before. `Principal.Caller`, `CallerGateway`, `hasRole`, `s.orgOf` and `r.Pattern`
(Go 1.23's `http.Request.Pattern`) are this hunk's names; task 05's spelling wins. `r.Pattern` is
set only once the mux has matched, so the check holds when authentication runs inside the handler
(task 06's `s.Write`, `task-06.md:337`); if Hub task 13 authenticates in middleware ahead of the
mux, the check moves into a per-route wrapper and `TestGatewayRefusedOffItsThreeRoutes` still pins
it.

#### `server/orgquotas.go` (complete)

```go
package server

import (
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

func (s *Server) registerQuotas() {
	s.mux.HandleFunc("GET /v1/orgs/{orgId}/quotas", s.getOrgQuotas)           // 80
	s.mux.HandleFunc("PUT /v1/orgs/{orgId}/quotas", s.putOrgQuotas)           // 81
	s.mux.HandleFunc("POST /v1/orgs/{orgId}/usage-rollups", s.postRollup)     // 76
	s.mux.HandleFunc("GET /v1/projects/{p}/usage", s.getUsage)                // 77
}

// getOrgQuotas: the gateway, or a member of any team project in the org. The
// ceiling is not a secret from members: it is why their quota is what it is.
func (s *Server) getOrgQuotas(w http.ResponseWriter, r *http.Request) {
	s.Read(w, r, func(tx *Tx) (any, error) {
		if err := sameOrg(tx.P, r.PathValue("orgId")); err != nil {
			return nil, err
		}
		if tx.P.Caller != CallerGateway && !store.MemberOfAnyTeamProject(r.Context(), tx, tx.P.UserID) {
			return nil, Refuse(403, hubapi.CodeForbidden, "Not a member here.", map[string]any{"required": "member"})
		}
		return store.OrgQuotas(r.Context(), tx, tx.P.OrgID) // DefaultOrgQuotas, revision 1, when absent
	})
}

func (s *Server) putOrgQuotas(w http.ResponseWriter, r *http.Request) {
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if err := sameOrg(tx.P, r.PathValue("orgId")); err != nil {
			return WriteResult{}, err
		}
		if tx.P.Caller != CallerDesk || !tx.P.OrgAdmin {
			return WriteResult{}, Refuse(403, hubapi.CodeForbidden, "Only an org admin sets ceilings.",
				map[string]any{"required": "org_admin"})
		}
		var req hubapi.PutOrgQuotasRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		for _, f := range [...]struct {
			name string
			v    int64
		}{{"memberDailyCeilingCents", req.MemberDailyCeilingCents},
			{"projectDailyCeilingCents", req.ProjectDailyCeilingCents}} {
			if !hubapi.ValidCeiling(f.v) {
				return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "A ceiling is out of range.",
					map[string]any{"field": f.name})
			}
		}
		q, err := store.PutOrgQuotas(r.Context(), tx, tx.P.OrgID, req) // stale revision: 409 conflict
		if err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "org.quotas.updated", Subject: tx.P.OrgID, Data: map[string]any{
			"memberDailyCeilingCents": q.MemberDailyCeilingCents,
			"projectDailyCeilingCents": q.ProjectDailyCeilingCents, "revision": q.Revision}})
		return WriteResult{Status: 200, Body: q}, nil
	})
}
```

A `PUT` refuses a Fleet caller as well as a member: Fleet is never an org admin, whatever the
delegation's owner is, because rule 3's principal is an owner and ceilings are not an owner's
record. `s.Read`, `sameOrg`, `CallerDesk`, `tx.P.OrgAdmin` and `store.MemberOfAnyTeamProject` are
this part's names for Hub task 13/14 shapes, **UNVERIFIED until merge**. The fields are checked in
a fixed order, so a body with both wrong always names `memberDailyCeilingCents`.

