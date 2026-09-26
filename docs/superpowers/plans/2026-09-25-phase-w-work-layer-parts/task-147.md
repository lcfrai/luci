### Task 147: intake rules, first match wins with the author on the event; form defaults for members and non-members

W5, stream WH; no seam. Depends on task 146 (`createRequest`, whose `routeRequest` this task
fills, and `hubapi.IntakeRoute`). Estimate 0.3 (2.5 h). Closes, jointly with task 170 (the rules
and form editors): **L-49** (*"Intake rules, first match wins, each with its author on the
event"*, 05 §1) and **L-28** (form defaults for members and non-members). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1).

When a request arrives, something has to say where it should go: which project, what priority,
which labels, whether it is important. Two things say it, in this order. The **form's defaults**
(task 148), one set for team members and one for everyone else, and then the team's **intake
rules**, an ordered list whose **first** matching rule overrides the fields it sets. The answer is
a *suggestion* on the request (`route`), which the accept dialog (task 170) fills in; no rule
creates an issue, and no rule sends a request to another team. After this task a team admin can
set the rule list, every request says which rule routed it and **whose rule it was**, and editing
the rules never rewrites a request already routed.

**Task 145 is not written at this writing** (gap 1). The table below is this task's requirement on
it, as task 146 states `work_requests`; task 145's names win when it lands.

Seven decisions the scope left open, made here.

1. **The rule list is one versioned document.** `PUT` replaces the **whole ordered list** in one
   body and writes version *n + 1*; every older version stays, readable with `?version=`. A
   request records `(ruleId, ruleVersion)`, so *"rule `vip` of version 4"* names exactly the words
   that routed it, even after version 9 deleted `vip`. This is what *"editing rules leaves
   already-routed requests as they were"* means, and why no rule row is ever updated: the table
   has an `INSERT` grant and no `UPDATE`.
2. **The PUT is optimistic.** The body carries `baseVersion`; if another admin wrote first it is
   409 `intake_rules_moved {version}`, and the editor reloads. Two admins editing at once never
   lose each other's rules silently.
3. **A rule id is the editor's, not the Hub's.** Rule ids are short slugs, `^[a-z0-9][a-z0-9-]{0,39}$`,
   unique in one version, kept by the editor when a rule moves (task 170 mints `rule-1`, `rule-2`
   …). The Hub mints no id for a rule, so no new id prefix is needed (the skeleton's list,
   `:185-187`, has none for rules). `(ruleId, ruleVersion)` is the key.
4. **A rule matches on four fields, and a rule with none is refused.** `match` holds any of
   `channel`, `formId`, `senderDomain` and `textContains`; a rule matches when **every field it
   sets** holds. A rule that sets none would match everything and make every later rule dead, so
   PUT refuses it (`rules[n].match`); the catch-all is the form's defaults. `senderDomain` matches
   the address's domain or any subdomain of it, case-insensitively (`client.example` matches
   `mail.client.example`, never `evilclient.example`). `textContains` is a case-insensitive
   substring of the quote after `strings.ToLower` on both, **never a regular expression**, so no
   rule can be made slow; its Unicode limits (`ToLower` is not full case folding) are named in
   *does not claim*.
5. **Overrides are per field.** A rule's `route` overrides only the fields it sets: a non-empty
   `projectId` or `priority`, a non-empty `labelIds` (which **replaces** the default list, never
   unions it, so a rule can say exactly which labels), and `important: true`. A rule cannot unset
   *important*; a requester flagged important by a form stays so. A rule's route with no field set
   is refused (`rules[n].route`): it would record a match that did nothing.
6. **Membership is the Hub's.** `isMember` is true exactly when the requester is a `person` whose
   user is in the request's team (`team_members`), read inside the create transaction. An `email`
   or `teams` requester is a non-member, even when the address belongs to a member (task 149
   turns an address that matches an org user into a `person` first, so a member mailing in is a
   member). Nothing in any submission body can set it; a JSON `member: true` is an unknown key and
   task 148's decoder refuses it.
7. **A rule routes only inside its team.** Every `projectId` in a rule is a project of the team
   and every `labelIds` entry a live label of the team, checked at PUT (`rules[n].route.projectId`,
   `rules[n].route.labelIds[k]`). A project archived after the PUT is still in the stored version;
   the accept dialog shows it as archived and suggests nothing, and task 146's accept refuses it as
   it refuses any project not live in the team.

#### Files

- `control-plane/internal/hub/intake/route.go` and `route_test.go` (new): a **pure** package, no
  database, no clock. **Not yet in the File map** (gap 2): the row to add is `intake/route.go` |
  *the intake router: form defaults, then the first matching rule* | new | 147.
- `control-plane/internal/hub/store/work_intake_rules.go` (new).
- `control-plane/internal/hub/server/work_intake_rules.go` (new): the two routes, and the body of
  task 146's `routeRequest`.
- `control-plane/internal/hub/server/work_intake_rules_integration_test.go` (new, `-tags integration`).
- ⚑ `control-plane/internal/hub/server/work_requests.go` (task 146): `routeRequest`'s one-line body
  is replaced by a call into this task's file; nothing else in it changes.
- ⚑ `control-plane/internal/hub/hubapi/work_intake.go` (task 143) and ⚑ `hubapi/workevents.go`
  (task 13): the types and one event kind below.
- ⚑ `control-plane/internal/hub/server/work_matrix_rows_integration_test.go`: key `"147"`.

#### The table this task needs from task 145's migration (requirement; complete)

```sql
-- ── task 147 (in task 145's NNNN_work_w5.sql): intake rules, one row per version ─────
CREATE TABLE work_intake_rule_sets (
  org_id      text        NOT NULL REFERENCES orgs (id),
  team_id     text        NOT NULL,
  version     integer     NOT NULL CHECK (version >= 1),
  rules       jsonb       NOT NULL CHECK (jsonb_typeof(rules) = 'array' AND jsonb_array_length(rules) <= 50),
  written_at  timestamptz NOT NULL,
  written_by  text        NOT NULL,
  PRIMARY KEY (team_id, version),
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, written_by) REFERENCES users (org_id, id)
);
ALTER TABLE work_intake_rule_sets ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_intake_rule_sets FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_intake_rule_sets TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_intake_rule_sets TO hub_app;   -- no UPDATE, no DELETE: decision 1

-- task 146's work_requests.rule_version names a row here; a foreign key makes
-- "a request routed by a version that does not exist" impossible.
ALTER TABLE work_requests ADD CONSTRAINT work_requests_rule_version_exists
  FOREIGN KEY (team_id, rule_version) REFERENCES work_intake_rule_sets (team_id, version);
```

The whole list is one `jsonb` value because it is written and read whole, and because *first
match wins* is a property of the list's order, which one array keeps without a `position` column
to renumber. `rules` holds a team admin's configuration words (`textContains`, a domain): not a
requester's words, so erase does not reach it, and it is never in an event. The foreign key from
`work_requests` is the second layer behind `routeRequest`, which only ever records the version it
read in the same transaction.

#### `hubapi/work_intake.go`, the additions (requirement on task 143; complete)

```go
// ── Task 147: intake rules (SW.21) ──────────────────────────────────────────

// IntakeMatch: a rule matches when every field it sets holds (decision 4).
type IntakeMatch struct {
	Channel      RequestChannel `json:"channel,omitempty"`
	FormID       string         `json:"formId,omitempty"`
	SenderDomain string         `json:"senderDomain,omitempty"` // lower-case; subdomains match
	TextContains string         `json:"textContains,omitempty"` // case-insensitive substring
}

func (m IntakeMatch) Empty() bool {
	return m.Channel == "" && m.FormID == "" && m.SenderDomain == "" && m.TextContains == ""
}

// IntakeRule is one rule. Its place in the list is its position (decision 1).
type IntakeRule struct {
	ID    string      `json:"id"` // the editor's slug, unique in one version (decision 3)
	Match IntakeMatch `json:"match"`
	Route IntakeRoute `json:"route"` // task 146's type; only the fields set override
}

// IntakeRuleSet is GET /v1/teams/{teamId}/intake-rules's 200. Version 0 is the
// team that has never set a rule: rules [], writtenAt "", writtenBy null.
type IntakeRuleSet struct {
	TeamID    string       `json:"teamId"`
	Version   int          `json:"version"`
	Rules     []IntakeRule `json:"rules"`
	WrittenAt string       `json:"writtenAt"`
	WrittenBy *User        `json:"writtenBy"`
}

// PutIntakeRulesRequest replaces the whole list (decision 2).
type PutIntakeRulesRequest struct {
	BaseVersion int          `json:"baseVersion"`
	Rules       []IntakeRule `json:"rules"`
}

// IntakeDefaults are a form's two starting routes (L-28). Task 148's
// IntakeForm carries one; intake.Route reads only this, not the whole form.
type IntakeDefaults struct {
	Member    IntakeRoute `json:"member"`
	NonMember IntakeRoute `json:"nonMember"`
}

const (
	MaxIntakeRules       = 50
	MaxTextContainsChars = 200
	MaxSenderDomainBytes = 253 // RFC 1035's name limit
	EventIntakeRulesSet  = "intake.rules_set"
)

// IntakeRulesSetData: ids and counts only; the rules' words stay in the table.
type IntakeRulesSetData struct {
	Version int64 `json:"version"`
	Count   int64 `json:"count"`
}
// Appended to WorkEventKinds: {EventIntakeRulesSet, IntakeRulesSetData{}, ScopeTeam, VisibleToReaders}
```

`intake.` joins task 13's Work prefix list (`task-13.md:524`) in this commit.

#### `intake/route.go` (complete)

```go
// Package intake decides where a request is suggested to go. It is pure: the
// server reads the rules, the form's defaults and team membership, and hands
// them here inside the request-create transaction (05 §3.5 point 3).
package intake

import (
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Incoming is what a rule may look at. SenderDomain is "" for a person
// requester with no address; Text is the quote.
type Incoming struct {
	Channel      hubapi.RequestChannel
	FormID       string
	SenderDomain string
	Text         string
}

// Decision is the route and, when a rule matched, which one and whose.
type Decision struct {
	Route       hubapi.IntakeRoute
	RuleID      string // "" when no rule matched
	RuleVersion int
	RuleAuthor  string
}

// Route: the form's defaults for a member or a non-member first, then the
// first rule, in list order, whose every set match field holds, overriding
// the fields its route sets. No match keeps the defaults (and the team's
// Triage, which every request lands in whatever the route says).
func Route(set hubapi.IntakeRuleSet, in Incoming, defaults *hubapi.IntakeDefaults, isMember bool) Decision {
	var d Decision
	if defaults != nil {
		if isMember {
			d.Route = clone(defaults.Member)
		} else {
			d.Route = clone(defaults.NonMember)
		}
	}
	for _, r := range set.Rules {
		if !matches(r.Match, in) {
			continue
		}
		d.Route = override(d.Route, r.Route)
		d.RuleID, d.RuleVersion = r.ID, set.Version
		if set.WrittenBy != nil {
			d.RuleAuthor = set.WrittenBy.ID
		}
		return d // first match wins
	}
	return d
}

func matches(m hubapi.IntakeMatch, in Incoming) bool {
	if m.Empty() {
		return false // PUT refuses it; a stored one (never) would match nothing
	}
	if m.Channel != "" && m.Channel != in.Channel {
		return false
	}
	if m.FormID != "" && m.FormID != in.FormID {
		return false
	}
	if m.SenderDomain != "" && !domainWithin(strings.ToLower(in.SenderDomain), m.SenderDomain) {
		return false
	}
	if m.TextContains != "" && !strings.Contains(strings.ToLower(in.Text), strings.ToLower(m.TextContains)) {
		return false
	}
	return true
}

// domainWithin: d is want, or a subdomain of it. "evilclient.example" is not
// within "client.example"; "" is within nothing.
func domainWithin(d, want string) bool {
	return d != "" && (d == want || strings.HasSuffix(d, "."+want))
}

func override(base, r hubapi.IntakeRoute) hubapi.IntakeRoute {
	if r.ProjectID != "" {
		base.ProjectID = r.ProjectID
	}
	if r.Priority != "" {
		base.Priority = r.Priority
	}
	if len(r.LabelIDs) > 0 {
		base.LabelIDs = append([]string(nil), r.LabelIDs...) // replaces, never unions
	}
	if r.Important {
		base.Important = true // a rule cannot unset it (decision 5)
	}
	return base
}

func clone(r hubapi.IntakeRoute) hubapi.IntakeRoute {
	r.LabelIDs = append([]string(nil), r.LabelIDs...)
	return r
}
```

**The author is whoever wrote the version**, not a per-rule author: a version is one admin's act,
and every rule in it, kept or moved, is theirs from that version on (the editor shows *set by Ada,
version 4*). A per-rule author would need the Hub to diff versions to know which rules were
changed, and a diff is a guess about intent; the version's writer is a fact. Scope's
*"`request.created` carries `ruleId`, `ruleVersion` and `ruleAuthor`"* holds.
`TestAKeptRuleTakesTheNewAuthor` pins the choice so it is never changed by accident.

#### Shape validation (`hubapi/work_intake.go`, complete)

```go
var ruleIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
var domainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// ValidateIntakeRules checks the list's shape before anything is read and
// names the first failing field. Team membership of projects and labels is the
// server's check (decision 7), after this passes.
func ValidateIntakeRules(rules []IntakeRule) *FieldError {
	if len(rules) > MaxIntakeRules {
		return &FieldError{"rules", ErrBadEnvelope}
	}
	seen := map[string]bool{}
	for i, r := range rules {
		at := func(f string) *FieldError { return &FieldError{fmt.Sprintf("rules[%d].%s", i, f), ErrBadEnvelope} }
		switch {
		case !ruleIDRe.MatchString(r.ID) || seen[r.ID]:
			return at("id")
		case r.Match.Empty():
			return at("match")
		case r.Match.Channel != "" && !slices.Contains(RequestChannels, r.Match.Channel):
			return at("match.channel")
		case r.Match.SenderDomain != "" && (len(r.Match.SenderDomain) > MaxSenderDomainBytes || !domainRe.MatchString(r.Match.SenderDomain)):
			return at("match.senderDomain") // lower-case only: the editor lowers it, the Hub refuses
		case r.Match.TextContains != "" && (strings.TrimSpace(r.Match.TextContains) != r.Match.TextContains ||
			utf8.RuneCountInString(r.Match.TextContains) > MaxTextContainsChars):
			return at("match.textContains")
		case r.Route.ProjectID == "" && r.Route.Priority == "" && len(r.Route.LabelIDs) == 0 && !r.Route.Important:
			return at("route")
		case r.Route.Priority != "" && !ValidPriority(r.Route.Priority):
			return at("route.priority")
		case len(r.Route.LabelIDs) > MaxImportLabels:
			return at("route.labelIds")
		}
		seen[r.ID] = true
	}
	return nil
}
```

`MaxImportLabels` (50, `task-15.md:182`) is reused as the cap on labels one route may name: the
same number for the same reason, a list a person reads. `formId` is not shape-checked here beyond
being a string: a rule may name a form that task 148 has not created yet only if the server finds
it, so the server checks it against the team's forms (`rules[n].match.formId`), and before task
148 lands any non-empty `formId` is 400.

#### The store (`store/work_intake_rules.go`, signatures complete)

```go
package store

// LatestIntakeRules reads the team's highest version, or version 0 with no
// rules. The create transaction calls it, so the version it routes by is the
// one it records (the foreign key's second layer).
//   SELECT version, rules, written_at, written_by FROM work_intake_rule_sets
//    WHERE team_id = $1 ORDER BY version DESC LIMIT 1
func LatestIntakeRules(ctx context.Context, q Querier, teamID string) (hubapi.IntakeRuleSet, error)

// IntakeRulesAt reads one version; found is false for a version never written.
func IntakeRulesAt(ctx context.Context, q Querier, teamID string, version int) (set hubapi.IntakeRuleSet, found bool, err error)

// InsertIntakeRules writes version base+1. A 23505 on the primary key is
// ErrIntakeRulesMoved: another admin wrote base+1 first (decision 2).
func InsertIntakeRules(ctx context.Context, tx pgx.Tx, org, teamID string, base int, rules []hubapi.IntakeRule,
	by string, at time.Time) (version int, err error)

var ErrIntakeRulesMoved = errors.New("store: the intake rules moved on")

// IsTeamMember: SELECT EXISTS (SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2).
// The Hub's table (H05); its column names are UNVERIFIED until that lands.
func IsTeamMember(ctx context.Context, q Querier, teamID, userID string) (bool, error)

// LiveTeamProjects and LiveTeamLabels answer which of ids belong to the team
// and are not archived, for decision 7's check.
func LiveTeamProjects(ctx context.Context, q Querier, teamID string, ids []string) (map[string]bool, error)
func LiveTeamLabels(ctx context.Context, q Querier, teamID string, ids []string) (map[string]bool, error)
```

`WrittenBy` is filled from `users` by the same join task 146 uses for a requester, so the read
answers the Hub's `User` shape. The PUT's check-then-insert runs under the org's write lock
(`s.Write`), so two PUTs in one org are already serial, and the handler compares `baseVersion`
with the latest version inside the lock: a stale editor gets 409 there. The primary key is the
second layer, for a writer that ever skips the comparison.

#### The routes and the router's caller (`server/work_intake_rules.go`, complete)

```go
package server

func (s *Server) registerWorkIntakeRules() {
	s.mux.HandleFunc("GET /v1/teams/{teamId}/intake-rules", s.getIntakeRules) // viewer; ?version=
	s.mux.HandleFunc("PUT /v1/teams/{teamId}/intake-rules", s.putIntakeRules) // team admin
}

func (s *Server) putIntakeRules(w http.ResponseWriter, r *http.Request) {
	s.Write(w, r, func(ctx context.Context, tx *Tx) (int, any, error) {
		teamID := r.PathValue("teamId")
		if err := Require(ctx, tx, teamID, hubapi.RoleAdmin); err != nil {
			return 0, nil, err // 404 team_not_found, or 403 {required: admin}
		}
		in, err := Decode[hubapi.PutIntakeRulesRequest](r)
		if err != nil {
			return 0, nil, err
		}
		if fe := hubapi.ValidateIntakeRules(in.Rules); fe != nil {
			return 0, nil, Refuse(http.StatusBadRequest, "bad_request", map[string]any{"field": fe.Field})
		}
		if fe, err := s.checkRulesInTeam(ctx, tx, teamID, in.Rules); err != nil || fe != nil {
			return 0, nil, orField(err, fe) // decision 7: rules[n].route.projectId, …labelIds[k], …match.formId
		}
		cur, err := store.LatestIntakeRules(ctx, tx, teamID)
		if err != nil {
			return 0, nil, err
		}
		if in.BaseVersion != cur.Version {
			return 0, nil, Refuse(http.StatusConflict, "intake_rules_moved", map[string]any{"version": cur.Version})
		}
		v, err := store.InsertIntakeRules(ctx, tx, tx.P.OrgID, teamID, cur.Version, in.Rules, tx.P.UserID, tx.Now)
		if errors.Is(err, store.ErrIntakeRulesMoved) {
			return 0, nil, Refuse(http.StatusConflict, "intake_rules_moved", map[string]any{"version": cur.Version + 1})
		}
		if err != nil {
			return 0, nil, err
		}
		if err := emitWork(tx, hubapi.EventIntakeRulesSet, teamID, "", teamID, hubapi.IntakeRulesSetData{
			Version: int64(v), Count: int64(len(in.Rules))}); err != nil {
			return 0, nil, err
		}
		set, _, err := store.IntakeRulesAt(ctx, tx, teamID, v)
		return http.StatusOK, set, err
	})
}

// getIntakeRules: Require(viewer); no ?version= is LatestIntakeRules; a
// ?version= that is not a positive integer is 400 {field: version}; a version
// never written is 404 intake_rules_version_not_found; 200 IntakeRuleSet.
func (s *Server) getIntakeRules(w http.ResponseWriter, r *http.Request)

// routeRequest replaces task 146's empty body. It runs inside createRequest's
// transaction, before the insert, so the version it reads is the one it records.
func (s *Server) routeRequest(ctx context.Context, tx *Tx, rq *store.RequestRow) error {
	set, err := store.LatestIntakeRules(ctx, tx, rq.TeamID)
	if err != nil {
		return err
	}
	member := false
	if rq.RequesterKind == string(hubapi.RequesterPerson) {
		if member, err = store.IsTeamMember(ctx, tx, rq.TeamID, rq.RequesterUserID); err != nil {
			return err
		}
	}
	var defaults *hubapi.IntakeDefaults
	if rq.FormID != "" {
		if defaults, err = s.formDefaults(ctx, tx, rq.FormID); err != nil { // task 148; nil before it lands
			return err
		}
	}
	d := intake.Route(set, intake.Incoming{Channel: hubapi.RequestChannel(rq.Channel), FormID: rq.FormID,
		SenderDomain: domainOf(rq.RequesterEmail), Text: rq.Quote}, defaults, member)
	rq.Route, rq.RuleID, rq.RuleVersion, rq.RuleAuthor = d.Route, d.RuleID, d.RuleVersion, d.RuleAuthor
	rq.Important = rq.Important || d.Route.Important
	return nil
}
```

`domainOf` is the part after the last `@`, lower-cased, or `""`. `s.formDefaults` is task 148's; in
this task's commit it is a stub that returns `nil, nil`, and task 148 replaces its body.
`checkRulesInTeam` runs `LiveTeamProjects`, `LiveTeamLabels` and, once task 148 lands, the team's
form ids, and answers the first failing field by the rules' order. `Require`, `Decode` and
`Refuse` are Hub task 14's and 15's, as task 146 cites them. The `request.created` event is task
146's, and `RuleVersion` and `RuleAuthor` in its data (task 146's `RequestCreatedData`) are now
filled; no type changes here. The subject of `intake.rules_set` is the team id, as task 13's
`team.work_policy_set` names its team (`task-13.md:263`).
