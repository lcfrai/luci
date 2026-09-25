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
    agent_id            text    not null check (agent_id = '' or agent_id ~ '^ag-'), -- '': not Fleet's (a laptop, OD-20)
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

#### `hubapi/quota.go`, continued (same file): route 77's answer

```go
// UsageDay is one UTC day's rows as the gateway last posted them.
type UsageDay struct {
	Day      string     `json:"day"`
	PostedAt string     `json:"postedAt"` // the replacing post's time: "as of"
	Rows     []UsageRow `json:"rows"`
}

// UsageResponse is GET /v1/projects/{p}/usage?from=&to= (route 77).
type UsageResponse struct {
	ProjectID string     `json:"projectId"`
	Days      []UsageDay `json:"days"` // ascending; a day with no post is absent, not empty
}

const MaxUsageDays = 31
```

#### `server/usage.go` (complete)

```go
package server

import (
	"net/http"
	"regexp"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

var dayRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// postRollup is route 76: the gateway only (rule 9), its own org only. The
// post replaces every row of (org, day) in one transaction.
func (s *Server) postRollup(w http.ResponseWriter, r *http.Request) {
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if tx.P.Caller != CallerGateway {
			return WriteResult{}, Refuse(403, hubapi.CodeForbidden, "Only the gateway posts usage.",
				map[string]any{"required": "gateway"})
		}
		if err := sameOrg(tx.P, r.PathValue("orgId")); err != nil {
			return WriteResult{}, err
		}
		var req hubapi.UsageRollupRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		if !dayRE.MatchString(req.Day) || !validUTCDay(req.Day, s.now()) {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "Not a recent UTC day.", map[string]any{"field": "day"})
		}
		if len(req.Rows) > hubapi.MaxRollupRows {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "Too many rows.", map[string]any{"field": "rows"})
		}
		if i, field, err := validateRows(r.Context(), tx, req.Rows); err != nil {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, err.Error(),
				map[string]any{"field": field, "index": i})
		}
		if err := store.ReplaceUsageDay(r.Context(), tx, tx.P.OrgID, req); err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "usage.rolled_up", Subject: tx.P.OrgID,
			Data: map[string]any{"day": req.Day, "rows": len(req.Rows)}})
		return WriteResult{Status: 204}, nil
	})
}
```

`validUTCDay` accepts today and the two days before it, UTC, and nothing in the future: the
poster's last post for a day lands just after midnight, and a late retry must still replace it.
`validateRows` checks, per row, and names the first failure: `projectId` is a team project of the
token's org (`rows[i].projectId`); `userId` is a member of it (`rows[i].userId`); `agentId` is `''`
or an `ag-…` whose Hub agent record (task 06) has that `ownerUserId` and `projectId`
(`rows[i].agentId`), so the gateway cannot misattribute one person's agent to another;
`counterpartUserId` is null or another member, never the row's own user; `route` is `global` or
`au`; each figure is ≥ 0. A row for a solo project is refused: solo usage stays phase 6's.

```go
// getUsage is route 77: a team admin reads every row, a member their own. A
// member's own rows include those where they are the counterpart, so Bob sees
// what Alice's Orchestrator spent on him, never what she spent on Carol.
func (s *Server) getUsage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("p")
	s.Read(w, r, func(tx *Tx) (any, error) {
		if tx.P.Caller == CallerGateway {
			return nil, Refuse(403, hubapi.CodeForbidden, "Not a gateway route.", map[string]any{"required": "member"})
		}
		role, _, ok, err := ProjectRole(r.Context(), tx, projectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, Refuse(404, hubapi.CodeProjectNotFound, "No such project.", nil)
		}
		if role == RoleViewer {
			return nil, Refuse(403, hubapi.CodeForbidden, "Members only.", map[string]any{"required": "member"})
		}
		from, to, err := usageRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"), s.now())
		if err != nil { // more than MaxUsageDays, to before from, not a day: 422 field "from"/"to"
			return nil, err
		}
		only := tx.P.UserID // own rows, as user or as counterpart
		if role == RoleAdmin && tx.P.Caller == CallerDesk {
			only = "" // every row of the project
		}
		return store.UsageDays(r.Context(), tx, projectID, from, to, only)
	})
}
```

A Fleet caller under a delegation reads as the owner, so it gets the owner's own rows even when the
owner is an admin: an admin's breadth is a desk act, not something a delegation carries.
`ProjectRole`'s first result, `RoleViewer` and `RoleAdmin` are task 06's call (`task-06.md:341`)
with this part's names for its role values, **UNVERIFIED until merge**.

#### `server/teampolicy.go`, the ceiling hunk (task 43's route-75 handler)

After task 43's own range checks (every quota > 0, `task-01.md:480-481`) and before its store call:

```go
		ceil, err := store.OrgQuotas(r.Context(), tx, tx.P.OrgID)
		if err != nil {
			return WriteResult{}, err
		}
		if req.Quotas.MemberDailyCents > ceil.MemberDailyCeilingCents {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "Above the org's ceiling.", map[string]any{
				"field": "quotas.memberDailyCents", "reason": hubapi.ReasonAboveCeiling,
				"ceilingCents": ceil.MemberDailyCeilingCents})
		}
		if req.Quotas.ProjectDailyCents > ceil.ProjectDailyCeilingCents {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "Above the org's ceiling.", map[string]any{
				"field": "quotas.projectDailyCents", "reason": hubapi.ReasonAboveCeiling,
				"ceilingCents": ceil.ProjectDailyCeilingCents})
		}
```

Refused, not clamped, for task 37's reason (`task-37.md:488-489`): a silent clamp tells the admin a
number that is not the one in force. `ceilingCents` lets task 67 say the ceiling in the refusal
without a second read. A member quota above the project quota is **allowed**: a project of one
person may give that person all of it, and keysmith takes the least of the two anyway. Task 43
must create each new team policy with `hubapi.DefaultQuotas`, never zeros (asked in the gaps).

#### `budget/daily.go` (complete; phase 6 delta)

```go
package budget

import (
	"context"
	"fmt"
	"time"
)

// DayLimits are one team project's daily quotas as keysmith enforces them:
// for each, the least of the team admin's quota and the org admin's ceiling.
// US cents. The member figure is per member, in this project.
type DayLimits struct {
	MemberCents  int64 `json:"memberCents"`
	ProjectCents int64 `json:"projectCents"`
}

// DaySpent is what the member spent in this project, and what the project
// spent, on keys minted this UTC day. Task 66's usage.Reader.SpentDay fills it.
type DaySpent struct {
	MemberUSD  float64 `json:"memberUsd"`
	ProjectUSD float64 `json:"projectUsd"`
}

// DayPolicy answers a project's daily limits. ok == false means the project
// has no daily dimension (a solo project); an error fails the mint closed.
type DayPolicy interface {
	DayLimits(ctx context.Context, teamID, projectID string) (l DayLimits, ok bool, err error)
}

// Validate is nil only when both limits are above zero: an unset daily quota
// is the org pool by another name (07 I3), so it is never "unlimited".
func (d DayLimits) Validate() error {
	if d.MemberCents <= 0 || d.ProjectCents <= 0 {
		return fmt.Errorf("%w: daily member %d, project %d cents", ErrInvalidLimits, d.MemberCents, d.ProjectCents)
	}
	return nil
}

// DaySlice is the most one new key may spend today: the least of the
// member's and the project's remainder, in US dollars. When either is spent,
// ok is false and exhausted names it, person before project. Invalid limits
// are (0, None, false), a misconfiguration, never a key without a cap.
func DaySlice(d DayLimits, s DaySpent) (maxUSD float64, exhausted Dimension, ok bool) {
	if d.Validate() != nil {
		return 0, None, false
	}
	member := remaining(float64(d.MemberCents)/100, s.MemberUSD)
	project := remaining(float64(d.ProjectCents)/100, s.ProjectUSD)
	switch {
	case member <= 0:
		return 0, Person, false
	case project <= 0:
		return 0, Project, false
	}
	return min(member, project), None, true
}

// UTCDay is the start of t's UTC day: the day a key's spend is filed under.
func UTCDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
```

`remaining` is P6-06's own helper (`task-06.md:462-467`): a NaN spend (spend it could not read)
leaves nothing, and a negative spend counts as zero; `min` is Go 1.21's builtin. The package stays standard library only, as P6-06 requires
(`task-06.md:7`).

#### `usage.go`, three hunks (P6-09's file)

```go
// In Record, after PeriodStart. Filled by the sweep from the key's binding.
MintedDay   time.Time // UTCDay(binding.CreatedAt)
AgentID     string    // wire.Binding.AgentID; "" when it is not an ag-… id
Route       string    // wire.Binding.RouteAtMint
Counterpart string    // the binding's counterpart (task 20's field); "" for none

// In Store, beside Sum.
// SumDay is the member's spend in the project and the project's spend, on
// keys minted on day (a UTCDay).
SumDay(ctx context.Context, userID, projectID string, day time.Time) (memberUSD, projectUSD float64, err error)
// DayRows groups one day's keys of team projects for the roll-up poster.
DayRows(ctx context.Context, day time.Time, projects []string) ([]DayRow, error)

// DayRow is one roll-up row before it becomes a hubapi.UsageRow.
type DayRow struct {
	UserID, TeamID, ProjectID, AgentID, Route, Counterpart string
	SpendUSD                                              float64
}

// In Reader, beside Spent: the same 60-second cache and the same fail-closed rule.
func (r *Reader) SpentDay(ctx context.Context, owner wire.Owner, team, project string) (budget.DaySpent, error)
```

`AgentID` is kept only when it matches `^ag-`: a laptop session's local id is a machine-local
session id, which rule 4 of the global constraints keeps out of the Hub. `key_spend.user_id` is
LiteLLM's `"<tid>:<oid>"` (`task-03.md:28` there), not the Hub's `u-…`, so the poster sends that
pair as `userId` and **the Hub maps it** to its user at insert (`store.UserByEntra`), refusing an
unknown pair as `rows[i].userId`; the counterpart is the Hub `u-…` Fleet put in the mint (task 20),
**UNVERIFIED** until task 20's counterpart form is fixed.

#### `00NN_key_spend_day.sql` (complete; phase 6 delta)

```sql
alter table keysmith.key_spend
    add column minted_day  date,                               -- null: swept before this migration
    add column agent_id    text not null default '',
    add column route       text not null default '' check (route in ('', 'global', 'au')),
    add column counterpart text not null default '';

create index key_spend_day_member  on keysmith.key_spend (minted_day, project_id, user_id);
```

A row swept before this migration has no `minted_day` and counts in no daily sum; it still counts
in its month. That under-counts at most one day's spend, once, at deploy.

#### `keysmith.go`, the mint hunk (P6-07's `mint`, after `budget.Slice`, `task-07.md:840-846`)

```go
	if s.d.Days != nil {
		day, isTeam, err := s.d.Days.DayLimits(ctx, sub.team, sub.project)
		if err != nil {
			return wire.MintResponse{}, fmt.Errorf("%w: day limits: %v", ErrBudgetUnknown, err)
		}
		if isTeam {
			ds, err := s.d.Spent.SpentDay(ctx, sub.owner, sub.team, sub.project)
			if err != nil {
				return wire.MintResponse{}, fmt.Errorf("%w: day spent: %v", ErrBudgetUnknown, err)
			}
			dayUSD, ddim, dok := budget.DaySlice(day, ds)
			if !dok {
				if ddim == budget.None {
					return wire.MintResponse{}, fmt.Errorf("%w: the day limits are unusable", ErrBudgetUnknown)
				}
				return wire.MintResponse{}, &BudgetExhaustedError{Dimension: ddim, Daily: true}
			}
			maxUSD = min(maxUSD, dayUSD)
		}
	}
```

`Deps.Days budget.DayPolicy` is nil in phase 6's own tests, so every P6-07 test passes unchanged.
`Deps.Spent`'s interface (`SpentReader`) gains `SpentDay`. `BudgetExhaustedError` gains
`Daily bool`; its `Error()` says `"today's "` before the dimension when set, and the mint's 402
body (P6-08, **UNVERIFIED** in its field names) carries `"window": "day"` or `"month"`, so a
session stopped by today's quota does not tell its owner that the month is spent.

#### `teamquota/hubday.go` (complete)

```go
// Package teamquota is the gateway's reader and writer of the Hub's team
// quotas: the daily limits keysmith enforces, and the usage it rolls up.
package teamquota

import (
	"context"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Hub is the three routes rule 9 admits, over the gateway's own token.
type Hub interface {
	TeamPolicy(ctx context.Context, projectID string) (p hubapi.TeamPolicy, isTeam bool, err error) // 74
	OrgQuotas(ctx context.Context) (hubapi.OrgQuotas, error)                                        // 80
	PostRollup(ctx context.Context, req hubapi.UsageRollupRequest) error                            // 76
}

const (
	Fresh = 60 * time.Second // re-read after this
	Stale = 15 * time.Minute // on a Hub error, a known answer this old still serves
)

type entry struct {
	limits budget.DayLimits
	isTeam bool
	at     time.Time
}

// HubDay is a budget.DayPolicy over the Hub. A Hub error serves the last
// known answer up to Stale, then fails closed; it never serves a default.
type HubDay struct {
	Hub Hub
	Now func() time.Time

	mu    sync.Mutex
	cache map[string]entry
}

func (h *HubDay) DayLimits(ctx context.Context, _ string, projectID string) (budget.DayLimits, bool, error) {
	now := h.Now()
	h.mu.Lock()
	e, ok := h.cache[projectID]
	h.mu.Unlock()
	if ok && now.Sub(e.at) < Fresh {
		return e.limits, e.isTeam, nil
	}
	fresh, err := h.read(ctx, projectID, now)
	if err != nil {
		if ok && now.Sub(e.at) < Stale {
			return e.limits, e.isTeam, nil
		}
		return budget.DayLimits{}, false, err
	}
	h.mu.Lock()
	if h.cache == nil {
		h.cache = map[string]entry{}
	}
	h.cache[projectID] = fresh
	h.mu.Unlock()
	return fresh.limits, fresh.isTeam, nil
}

// IsTeam answers from the cache, reading the Hub on a miss; the poster uses it.
func (h *HubDay) IsTeam(ctx context.Context, projectID string) (bool, error) {
	_, isTeam, err := h.DayLimits(ctx, "", projectID)
	return isTeam, err
}

func (h *HubDay) read(ctx context.Context, projectID string, now time.Time) (entry, error) {
	p, isTeam, err := h.Hub.TeamPolicy(ctx, projectID)
	if err != nil || !isTeam {
		return entry{isTeam: false, at: now}, err
	}
	q, err := h.Hub.OrgQuotas(ctx)
	if err != nil {
		return entry{}, err
	}
	return entry{isTeam: true, at: now, limits: budget.DayLimits{
		MemberCents:  min(p.Quotas.MemberDailyCents, q.MemberDailyCeilingCents),
		ProjectCents: min(p.Quotas.ProjectDailyCents, q.ProjectDailyCeilingCents)}}, nil
}
```

`Hub.TeamPolicy` answers `isTeam == false, err == nil` for a project route 74 does not know as a
team project (its 404, whatever code task 43 gives it). A team policy whose quota is 0 (a record task 43 wrote before
`DefaultQuotas`) yields `DayLimits` that `Validate` refuses, so the mint fails closed with
*"the day limits are unusable"* rather than minting without a daily cap. `isTeam == false` is
cached too, so a solo project costs the Hub one read a minute, not one per mint.

#### `teamquota/rollup.go` (complete)

```go
package teamquota

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/usage"
)

const Every = 10 * time.Minute

var ErrTooManyRows = errors.New("teamquota: more than 5,000 rows in one day")

// Rollup posts the current UTC day's team-project usage every Every, and
// the previous day once more on the first tick after midnight. Each post
// replaces the day (route 76), so two keysmith replicas posting is harmless.
type Rollup struct {
	Store usage.Store
	Hub   Hub
	Teams interface {
		IsTeam(ctx context.Context, projectID string) (bool, error)
	}
	Now func() time.Time

	lastDay time.Time
}

func (r *Rollup) Tick(ctx context.Context) error {
	today := budget.UTCDay(r.Now())
	days := []time.Time{today}
	if !r.lastDay.IsZero() && r.lastDay.Before(today) {
		days = []time.Time{r.lastDay, today} // close yesterday first
	}
	for _, d := range days {
		if err := r.post(ctx, d); err != nil {
			return err // the next tick retries; lastDay does not move
		}
	}
	r.lastDay = today
	return nil
}

func (r *Rollup) post(ctx context.Context, day time.Time) error {
	rows, err := r.Store.DayRows(ctx, day, nil)
	if err != nil {
		return err
	}
	req := hubapi.UsageRollupRequest{Day: day.Format(time.DateOnly), Rows: []hubapi.UsageRow{}}
	for _, row := range rows {
		team, err := r.Teams.IsTeam(ctx, row.ProjectID)
		if err != nil {
			return err
		}
		if !team {
			continue // solo usage stays in phase 6's store
		}
		var cp *string
		if row.Counterpart != "" {
			c := row.Counterpart
			cp = &c
		}
		req.Rows = append(req.Rows, hubapi.UsageRow{
			UserID: row.UserID, TeamID: row.TeamID, ProjectID: row.ProjectID, AgentID: row.AgentID,
			Route: hubapi.InferenceRoute(row.Route), CounterpartUserID: cp,
			CostCents: int64(math.Round(row.SpendUSD * 100)), // tokens: null, see the gaps
		})
	}
	if len(req.Rows) > hubapi.MaxRollupRows {
		return ErrTooManyRows // a day this large is reported, not truncated
	}
	return r.Hub.PostRollup(ctx, req)
}
```

A day with no team rows still posts an empty `rows`, which clears a day an earlier post
over-reported. `cmd/keysmith` runs `Tick` every `Every` as one more loop beside P6-09's sweeper,
after the sweep, so a post never races a half-swept minute.

#### `api/teamquotas.go`, the wire types (complete; they join ST.8)

```go
package api

// QuotaView is GET /v1/projects/{projectId}/team/quotas: one project's daily
// quotas and today's spend, per member and per counterpart. US cents only.
// There is no percentage, remainder or "used" field on purpose (OD-27).
type QuotaView struct {
	V         int           `json:"v"`         // 1
	ProjectID string        `json:"projectId"` // the cockpit's id, as every ST.8 view
	Team      bool          `json:"team"`      // false: a solo project, every other field empty
	Day       string        `json:"day"`       // the UTC day, "2026-09-25"
	AsOf      *string       `json:"asOf"`      // the gateway's last post today; null: none yet
	Currency  string        `json:"currency"`  // always "USD" (task 66 decision a)
	CanEdit   bool          `json:"canEdit"`   // the viewer is a team admin; the Hub still decides
	Revision  int           `json:"revision"`  // the team policy's, sent back on PUT
	Ceiling     QuotaCeiling  `json:"ceiling"`
	MemberDaily MemberDaily   `json:"memberDaily"` // the one member quota of this project
	Project     ProjectQuota  `json:"project"`
	Members     []MemberQuota `json:"members"` // an admin: every member; anyone else: themselves
}

// MemberDaily is the project's member quota: one number for every member.
type MemberDaily struct {
	DailyCents    int64 `json:"dailyCents"`    // the team admin's quota
	EnforcedCents int64 `json:"enforcedCents"` // min(quota, the org's member ceiling)
}

// QuotaCeiling is the org admin's (route 80), shown so an admin knows the bound.
type QuotaCeiling struct {
	MemberDailyCents  int64 `json:"memberDailyCents"`
	ProjectDailyCents int64 `json:"projectDailyCents"`
}

type ProjectQuota struct {
	DailyCents     int64  `json:"dailyCents"`     // the team admin's quota
	EnforcedCents  int64  `json:"enforcedCents"`  // min(quota, ceiling): what the gateway enforces
	SpentCents     *int64 `json:"spentCents"`     // today; null when AsOf is null
	Reached        bool   `json:"reached"`        // spent ≥ enforced, as of AsOf
}

// MemberQuota is one person against the project's member quota, which is
// one number for every member of the project (hubapi.Quotas).
type MemberQuota struct {
	UserID            string             `json:"userId"`
	Name              string             `json:"name"`
	EnforcedCents     int64              `json:"enforcedCents"`
	WorkersCents      *int64             `json:"workersCents"`      // the member's workers' keys
	CoordinationCents *int64             `json:"coordinationCents"` // the member's Orchestrator's keys
	OutsideFleetCents *int64             `json:"outsideFleetCents"` // sessions Fleet did not start (OD-20)
	ByCounterpart     []CounterpartSpend `json:"byCounterpart"`     // coordination, split
	Reached           bool               `json:"reached"`
}

// CounterpartSpend is "coordination with Alice: US$0.41 today". UserID ""
// is coordination with no single teammate: human-only turns and turns that
// served several (task 37 decision d).
type CounterpartSpend struct {
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	CostCents    int64  `json:"costCents"`
	InputTokens  *int64 `json:"inputTokens"`  // null until the gateway reads tokens (gap 3)
	OutputTokens *int64 `json:"outputTokens"`
}

// PutQuotasRequest is PUT /v1/projects/{projectId}/team/quotas (a team admin).
type PutQuotasRequest struct {
	Revision          int   `json:"revision"`
	MemberDailyCents  int64 `json:"memberDailyCents"`
	ProjectDailyCents int64 `json:"projectDailyCents"`
}
```

**Composition, in order.** `TeamReader` (task 52) maps the cockpit's project to `pj-…` and reads,
as the user, route 74 (the policy and its `revision`), route 80 (the ceiling), route 77 for today
(the rows), task 06's `GET …/agents` (each `ag-…`'s `role`) and the Hub's member list (names and
the viewer's role; the core Hub route, **UNVERIFIED** by name until merge). Each row goes to
exactly one bucket: `agentId ""` to *outside Fleet*; an `ag-…` whose record's role is
`orchestrator` to *coordination*, and within it to its `counterpartUserId` (null → `""`);
everything else, including an `ag-…` the list lacks, to *workers*, because only an Orchestrator's
keys are coordination (`task-37.md:436`). With a post today, a member with no rows is `0`; without
one, every figure is `null`, never `0` (task 37's rule, `task-37.md:456-458`). Members are sorted by
name; counterparts by cost, descending, then name.

**The `PUT`** reads route 74 once more, copies every field of the current policy into a
`hubapi.PutTeamPolicyRequest` except the two quotas, and sends it to route 75 with the body's
`revision`. On success it answers `200` with a freshly composed `QuotaView`. A refusal is relayed
unchanged: 422 with `reason: "above_org_ceiling"` and
`ceilingCents`, 409 `conflict` on a stale revision, 403 for a non-admin. The local control plane
checks nothing the Hub checks (`task-52.md:42-43`). A solo project answers `GET` with an empty,
`team: false` body and `PUT` with `409 not_team_project`, task 52's rule (`task-52.md:618-619`).

#### The tests, first (each names the mutant it kills)

**Hub** (`server/*_test.go`, against Hub task 16's test server and a real Postgres, as task 06's
tests run):

| Test | Asserts | Mutant killed |
|---|---|---|
| `TestProjectQuotaAboveCeilingRefused` | ceiling 50000; route 75 with `projectDailyCents` 50001: 422, `field` `quotas.projectDailyCents`, `reason` `above_org_ceiling`, `ceilingCents` 50000; the policy's revision unchanged | the scope's test; no ceiling check, or a clamp |
| `TestMemberQuotaAboveCeilingRefused` | the same for `memberDailyCents` over `memberDailyCeilingCents` | a check on one quota only |
| `TestQuotaAtCeilingAccepted` | exactly the ceiling: 200 | `>=` where `>` is meant |
| `TestUnsetOrgQuotasAreTheDefault` | a fresh org: route 80 answers `DefaultOrgQuotas`, revision 1, and both figures > 0 | the scope's test; an absent record read as zero, i.e. unlimited |
| `TestOrgQuotasZeroRefused` | route 81 with each ceiling 0, then −1, then 10,000,001: 422 with that `field` | a ceiling of 0 accepted as "off" |
| `TestOrgQuotasOrgAdminOnly` | route 81 as a team admin without `Hub.OrgAdmin`: 403 `required` `org_admin`; as Fleet under that admin's delegation: 403; as the org admin: 200 and one `org.quotas.updated` event with no free text | a team admin raising their own ceiling; a delegation carrying org-admin power |
| `TestLoweredCeilingRewritesNoProject` | quotas 25000 under a 50000 ceiling; ceiling lowered to 10000: route 74 still says 25000 | a ceiling write that silently rewrites every project |
| `TestGatewayRefusedOffItsThreeRoutes` | a `Hub.Gateway` token: 74, 80, 76 answer; route 75, 77, 81 and task 06's route 46 answer 403 `required` `member`; with an `X-Panacea-Delegation` header: 403 `required` `fleet` | rule 9 as a blanket admit |
| `TestGatewayRoleAndAzpBothRequired` | the right `azp` without `Hub.Gateway`, and the role with another `azp`: both refused | an `||` for the `&&` |
| `TestRollupReplacesTheDay` | two posts for one day with different rows: route 77 returns only the second post's rows, and `postedAt` moves | appending instead of replacing |
| `TestRollupRefusesMisattributedAgent` | a row naming Bob's `ag-…` under Alice's user: 422 `rows[i].agentId`; nothing stored | the gateway attributing one person's agent to another |
| `TestRollupRefusesSoloAndOtherOrg` | a solo project's row: 422 `rows[i].projectId`; an `orgId` not the token's: 403 | team rows mixed with solo usage; a cross-org post |
| `TestRollupNullTokensStayNull` | a row with `inputTokens: null`: route 77 answers `null`, not `0` | unknown shown as free |
| `TestUsageAdminAllMemberOwn` | rows for Alice, Bob and Alice→Bob coordination: an admin reads all three; Bob reads his own and the Alice→Bob row; Carol reads none of them; a viewer: 403 | a member reading a teammate's spend on a third person |
| `TestUsageRangeBounded` | `from` 32 days before `to`: 422 `field` `from` | an unbounded scan |

**Gateway** (`budget/daily_test.go`, pure; `teamquota_test.go` and keysmith's, on fakes):

| Test | Asserts | Mutant killed |
|---|---|---|
| `TestDaySliceIsTheLeastRemainder` | member US$25, spent 20; project US$200, spent 10: 5.0, ok | `max` for `min` |
| `TestDaySliceNamesTheSpentDimension` | member spent 25 of 25: `Person`, false; member fine, project spent: `Project`, false | a fixed dimension name |
| `TestDaySliceZeroLimitIsNeverUnlimited` | `DayLimits{0, 20000}`: (0, None, false) | the scope's test at the gateway: an unset quota minting without a cap |
| `TestDaySliceNaNSpendLeavesNothing` | `MemberUSD` NaN: false | an unreadable spend read as zero |
| `TestUTCDayAtSydneyMorning` | 2026-09-25 08:00 Australia/Sydney (22:00 UTC the 24th): `UTCDay` is the 24th | a local-time day |
| `TestMintTakesTheTighterWindow` | monthly slice US$25, day slice US$3: the key's `MaxBudget` 3.0; monthly US$2, day US$3: 2.0 | the day ignored, or the day replacing the month |
| `TestMintDayExhaustedSaysDay` | day spent: `BudgetExhaustedError{Dimension: person, Daily: true}` and `Error()` contains `today's` | the owner told the month is spent |
| `TestMintSoloHasNoDay` | `DayPolicy` answers `isTeam == false`: `SpentDay` is never called, `MaxBudget` is the monthly slice | a daily cap on solo work |
| `TestHubDayServesStaleThenFailsClosed` | Hub answers, then errors: at 5 min the old limits; at 16 min an error; never `DefaultQuotas` | a default served on a Hub error; failing on the first blip |
| `TestHubDayTakesTheLowerOfQuotaAndCeiling` | quota 25000, ceiling 10000: `ProjectCents` 10000 | a lowered ceiling not enforced |
| `TestRollupClosesYesterdayFirst` | ticks at 23:55 and 00:05 UTC: the second tick posts the 24th, then the 25th | the last ten minutes of each day never posted |
| `TestRollupSkipsSoloAndDropsNothing` | two team rows, one solo row: one post of two rows; 5,001 team rows: `ErrTooManyRows`, no post | solo usage in the Hub; silent truncation |

**Local control plane** (`api/teamquotas_test.go`, on task 52's fake Hub, golden files under
`testdata/team-quotas/`):

| Test | Asserts | Mutant killed |
|---|---|---|
| `TestQuotaViewBucketsEachRowOnce` | one worker row, one Orchestrator row with counterpart Bob, one with none, one `agentId ""`: workers, coordination (two counterparts, `""` included), outside Fleet each hold exactly their row; the four sum to the member's total | a row counted twice, or coordination read from the counterpart field alone |
| `TestQuotaViewNullWithoutAPost` | route 77 has no day: every spend field `null`, `asOf` `null`, `reached` false | an absent post shown as US$0.00 spent |
| `TestQuotaViewZeroWithAPost` | a post today with no row for Carol: Carol's figures `0` | a known zero shown as "not known" |
| `TestQuotaViewEnforcedIsTheLower` | quota 25000, ceiling 10000: `enforcedCents` 10000, `dailyCents` 25000 | the view promising a number the gateway does not enforce |
| `TestQuotaViewHasNoFraction` | reflection over every type in this file: no field name containing `Used`, `Percent`, `Remaining`, `Left` or `Ratio` (task 37's list, `task-37.md:603`, plus `Ratio`) | an "N of M" back through the side door (OD-27) |
| `TestQuotaPutKeepsOtherPolicyFields` | a PUT of two quotas: the route-75 body the fake receives carries the policy's `orchestratorTurnsPerHour`, `escalationFloors`, `messageBudgets` unchanged | an admin's quota edit resetting the escalation floors |
| `TestQuotaPutRelaysCeilingRefusal` | the fake answers 422 `above_org_ceiling`, `ceilingCents` 10000: the local answer is byte-equal | a local re-check that drifts from the Hub's |
| `TestQuotaViewSoloProject` | a solo project: `GET` 200 `team: false`; `PUT` 409 `not_team_project`, no Hub call | a solo project reaching route 75 |

#### Steps

1. **After the Hub sync-merge.** Write the Hub tests above; run them; each fails for its named
   reason (routes 76, 77, 80, 81 answer 404; route 75 accepts 50001).
2. Write `hubapi/quota.go`, the `team.go` hunk, the migration, `store/{quotas,usage}.go`,
   `server/{orgquotas,usage}.go`, the `teampolicy.go` hunk and the rule 9 hunk; add the four routes
   and the gateway principal to the matrix test. Run the Hub suite until green.
3. Amend ADR-H1 §5.10 with rule 9 and the contract with rows 80–81, §7.25's caller sentence and
   §7.27, word for word as above. Commit the Hub half.
4. **After the phase 6 sync-merge.** Write `daily_test.go`, then `daily.go`; the usage and
   migration hunks with their store-suite rows (`SumDay` on both stores); the keysmith hunk with
   its three tests. Run `go test ./internal/budget/ ./internal/usage/ ./internal/keysmith/`.
5. Write `teamquota` with its tests, then wire `cmd/keysmith`: `PANACEA_APP_GATEWAY_HUB_URL`,
   `PANACEA_APP_GATEWAY_HUB_CLIENT_ID` (the gateway's own Entra app, its token by workload
   identity), `Deps.Days = &teamquota.HubDay{…}`, and the `Rollup` loop. Commit the gateway half.
6. Write `api/teamquotas_test.go` and its goldens, then `api/teamquotas.go`; register the two local
   routes beside task 52's. Commit the local half.

#### Acceptance

```bash
cd control-plane
go test ./internal/hub/server/ -run 'Quota|Ceiling|Gateway|Rollup|Usage' -count=1 -v 2>&1 | grep -c '^--- PASS'
# expect 15
go test ./internal/budget/ ./internal/teamquota/ ./internal/keysmith/ -run 'Day|Mint|HubDay|Rollup|UTC' -count=1 -v 2>&1 | grep -c '^--- PASS'
# expect 12 (plus any P6-07 test whose name matches Mint: count those first and add them)
go test ./internal/api/ -run 'Quota' -count=1 -v 2>&1 | grep -c '^--- PASS'
# expect 8
go test ./... && go vet ./...                       # expect ok, no output from vet
go list -deps ./internal/budget/ | grep -c 'kpmg-au-emu'   # expect 1: still standard library only
grep -rn 'Percent\|Remaining\|Ratio' internal/api/teamquotas.go   # expect no output
grep -n 'Australia/Sydney' internal/hub/hubapi/team.go    # expect no output: the day is UTC
```

On the real cluster, deferred to task 69 item 6 (**quotas enforced at the gateway**): lower a test
project's member quota to 5 cents, run one worker until its key stops, and read keysmith's 402
with `"window": "day"`. Nothing in this task can observe LiteLLM enforcing `max_budget`; the unit
tests prove only the number keysmith sends.

#### What this task does NOT claim

- **That a daily quota is a hard ceiling.** keysmith caches spend 60 seconds (P6-09) and the
  sweep reads spend every 60 seconds, and N concurrent sessions can each spend a full slice, so
  the overrun bound is N × the slice plus a minute of spend, as P6-06 states for the month.
- **That the ceiling-then-budget order is an owner decision.** It is `05-design.md` Q3's
  recommendation; `OD-24` decided only "budgets on by default". The defaults (US$50 and US$500 a
  day ceilings; US$25 and US$200 a day quotas) are PROPOSED.
- **A ceiling per named person.** One per-member ceiling applies to every member of the org. A
  person-by-person ceiling is a list on the same record, not built.
- **Token counts in the admin view.** The roll-up posts `null` tokens (gap 3); cost is exact to a
  cent, as LiteLLM reports it, and the owner's own tokens stay exact in task 37's split.
- **That the view is live.** It is at most 10 minutes behind the gateway and says so with `asOf`.
- **Anything about LiteLLM's native user and team budgets.** They stay phase 6's monthly ones.

#### Gaps against the skeleton and the siblings, raised here and not resolved alone

1. **Currency.** Task 37's owner line says *"A$0.50"* (`task-37.md:51-52`) and task 52's `Spend`
   says *"Australia/Sydney day, in cents"* (`task-52.md:576`). This task fixes US cents and the
   UTC day (decisions a, c); tasks 37, 52 and 56 should say `US$` and UTC.
2. **Task 43 is not on disk.** Its route-75 handler must create every team policy with
   `hubapi.DefaultQuotas` and accept this task's ceiling hunk; its name is assumed.
3. **Token counts per key.** P6-09's sweep reads spend from LiteLLM's key info; whether that
   carries input and output tokens is **UNVERIFIED** (P6-18's probe should say). Until it does,
   `inputTokens`/`outputTokens` are `null`, which is why this task made them `*int64`.
4. **The counterpart on a binding.** The sweep copies `Counterpart` from the key's binding; P6-07's
   `wire.Binding` has no such field (`task-07.md:855-857` there). Task 20's gap S20-2 adds it to the
   mint request; the binding must carry it too, or the roll-up has no counterpart.
5. **Task 05 is not on disk.** Rule 9's hunk is written against task 01's description of the
   app-only branch (`task-01.md:114-117`); the Hub's Entra app must also define the `Hub.Gateway`
   role, an infrastructure step for wherever phase 8 registers the Hub's Entra app
   (**UNVERIFIED** which task) that no task yet lists.
6. **The skeleton's file map** lists only `QuotaAdmin.tsx` and task 66's phase 6 row; it has no
   row for `teamquota/`, `api/teamquotas.go` or the Hub files above, nor for routes 80–81.

