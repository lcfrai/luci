### Task 176: personal auto-assign rules in settings (L-40)

Stream **WL**, about 2 hours (an estimate, not a measurement). Depends on task **174**, and
through it on task 96 (the personal preferences surface), tasks 08 and 10 (the create and move
paths), task 24 (Start) and task 167 (`via` on every W5 principal). 05 L-40 (`05-design.md:149`,
verified 2026-09-26): *"Personal auto-assign automations · MATCH · A personal rule in settings ·
later"*; 05 L-235 lists auto-assign among personal preferences. Hub anchors are the `feat/hub`
worktree at `54b5e746` (`hub-contract:N`). `server/` and `store/` are **planned, not on
`feat/hub`**; parts 24, 96 and 167 are **not written** on 2026-09-26, so names from them are
**UNVERIFIED** until they land (rule H1).

**Decisions made here, so nobody re-derives them:**

1. **A rule lives in the Hub, not on the device.** Task 96's scope lists *"assign issues I create
   to me"* as a desktop preference, which would only ever prefill the desktop's create form: a
   create from the `panacea` verb, from a second device or from the CLI would miss it. A rule in
   the Hub applies to every one of the person's own writes. So this task **moves** that preference
   into the Hub rule `i_create`: task 96's `prefs.ts` loses the key, and a desktop that finds it
   stored `true` sends one `PUT` with the rule and then deletes the key (a one-time migration).
2. **Three moments, one act.** `i_create` (an issue the person creates), `i_accept` (a `hand` move
   the person makes out of `triage` to any state but `cancelled` or `duplicate`, task 11's triage
   exit), `i_start` (Start, task 24, made by the person). One `then`: `assign_me`. At most one rule
   per `when`, so the list has at most three entries.
3. **Applied inside the person's own write, never after it.** When the write has **no assignee
   named** and the head has none, the store sets `assignee_id = the caller` in the **same
   `UPDATE` or `INSERT`**, and the write's one event (decision 6) gains `data.assigneeId` and
   `data.assignCause: "personal_rule"` (a key of its own, because a state move's `data.cause` is
   already the move's cause, task 10). There is no second event: *"exactly one event per changed record"*
   (`hub-contract:1132-1133`).
4. **Only the person's own write.** A write whose caller carries any `via` (an agent through
   Fleet's delegation, an app, a key, the sync principal, Teams, the janitor: task 167's
   `ActorKindOf` returns anything but `person`) gets **no** rule, even when its `actor` is the
   person. An agent that files an issue for Dylan does not make Dylan its assignee.
5. **Never an override.** An assignee named in the write wins; an issue that already has an
   assignee keeps it. Task 03's `CreateIssueRequest.AssigneeID` is a `*string`
   (`task-03.md:427`), so an absent key and an explicit `null` decode alike and **both** let the
   rule fire; the create form shows *"assigned to you by your rule"* beside the assignee chip
   before the person presses create, so they see it and can name someone else. An explicit
   *nobody* on create needs task 03's field to become tri-state like `PatchIssueRequest`'s
   (`task-03.md:47`), recorded as gap 1 rather than changed here.
6. **The events.** `issue.created` (task 08), `issue.state` (task 10, `task-10.md:91`), and the
   `started` fact move task 22 writes for Start (task 24), whatever name task 13 fixes for it
   (SW.5). Each gains two `data` keys when a rule fired, nothing otherwise.
7. **The route has no person field.** `GET` and `PUT /v1/me/work/rules` act on the token's user
   only. There is no `userId` anywhere in the body or path, so a person cannot read or write
   another's rules by construction, and an org admin gets no path to them either.

**Files:**
- `control-plane/internal/hub/hubapi/work.go` ⚑ — `PersonalRule`, `RuleWhen`, `PutPersonalRules`,
  `ApplyPersonalRule` (the pure rule the Hub and the fake call) (additive)
- `control-plane/internal/hub/hubapi/work_personal_rules_test.go` — the pure rule (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_later.sql` — **the WL migration**, created by
  this task (the first WL task that needs a table); later WL tasks append to it in task order
  (new; the next free number at sync time)
- `control-plane/internal/hub/store/work_personal_rules.go`, `server/work_personal_rules.go` (new)
- `control-plane/internal/hub/store/work_issues.go` ⚑ (task 08), `work_states.go` ⚑ (task 10), the
  Start path ⚑ (task 24): one call each to `ApplyPersonalRule`
- `control-plane/internal/hub/server/work_personal_rules_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/work_matrix_rows.go` ⚑ — two rows
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ — the rule in the fake
- `desktop/src/api/work.ts` ⚑ — the wire type and two calls
- `desktop/src/components/settings/AssignMe.tsx` (+ test) — the *Assign me* section in task 96's
  `Preferences.tsx` ⚑ (new)
- `desktop/src/lib/prefs.ts` ⚑ (task 96) — the key removed, the one-time migration added

#### Wire shapes

```jsonc
// GET /v1/me/work/rules           any signed-in person; their own rules; [] when none
{ "rules": [ { "when": "i_create", "then": "assign_me" } ], "updatedAt": "2026-…", "version": 3 }

// PUT /v1/me/work/rules           the whole list (a PUT, not a PATCH); 200 with the stored list
{ "rules": [ { "when": "i_create", "then": "assign_me" }, { "when": "i_accept", "then": "assign_me" } ] }
// two rules with one "when"      → 400 bad_request, field "rules[1].when"
// a "when" or "then" not listed  → 400 bad_request, field "rules[n].when" | "rules[n].then"
// any other key, e.g. "userId"   → 400 bad_request (server.Decode refuses unknown fields)
// a caller with a via (agent, app, key, integration, teams) → 403 forbidden: rules are a person's own

// the create's one event when a rule fired (data is metadata only)
{ "kind": "issue.created", "subject": "is-…", "actor": { "id": "u-dylan" },
  "data": { "assigneeId": "u-dylan", "assignCause": "personal_rule", "…": "task 08's keys" } }
```

A `PUT` that changes nothing is 200 with no event; one that changes the list appends one
`me.work_rules_set` event with `data: {"whens": "i_create,i_accept"}` (a comma-joined string, as
task 11's `team.work_policy_set` does), `teamId` and `projectId` empty, subject the user id. It is
**not in the feed** of any team (a person's settings are theirs), only in the audit chain.

#### `hubapi/work.go`, the additions (complete)

```go
// RuleWhen is the moment a personal rule applies: one of the person's own acts.
type RuleWhen string

const (
	RuleOnCreate RuleWhen = "i_create" // an issue the person creates
	RuleOnAccept RuleWhen = "i_accept" // a hand move the person makes out of triage (not to cancelled or duplicate)
	RuleOnStart  RuleWhen = "i_start"  // Start (task 24), made by the person
)

// RuleThen is what a personal rule does. W-later ships one.
type RuleThen string

const RuleAssignMe RuleThen = "assign_me"

type PersonalRule struct {
	When RuleWhen `json:"when"`
	Then RuleThen `json:"then"`
}

type PersonalRules struct {
	Rules     []PersonalRule `json:"rules"` // never null
	UpdatedAt *time.Time     `json:"updatedAt"`
	Version   int            `json:"version"` // 0 when the person never set any
}

// PutPersonalRules is the PUT body. It has no person field, by design.
type PutPersonalRules struct {
	Rules []PersonalRule `json:"rules"`
}

// ValidatePersonalRules returns the first bad field path, or "" when the list is valid.
func ValidatePersonalRules(rs []PersonalRule) string {
	seen := map[RuleWhen]bool{}
	for i, r := range rs {
		switch r.When {
		case RuleOnCreate, RuleOnAccept, RuleOnStart:
		default:
			return fmt.Sprintf("rules[%d].when", i)
		}
		if r.Then != RuleAssignMe {
			return fmt.Sprintf("rules[%d].then", i)
		}
		if seen[r.When] {
			return fmt.Sprintf("rules[%d].when", i)
		}
		seen[r.When] = true
	}
	return ""
}

// RuleAct describes the write a rule might apply to.
type RuleAct struct {
	When            RuleWhen
	CallerID        string // the token's user
	CallerHasVia    bool   // any via: agent, app, key, integration, teams, janitor (task 167)
	AssigneeNamed   bool   // the write names an assignee (a non-nil AssigneeID)
	CurrentAssignee string // the head's assignee before the write; "" for none or a create
}

// ApplyPersonalRule answers the assignee to set in the same write, or "" for none.
// The Hub's store and the fake both call it; the conformance row holds one rule.
func ApplyPersonalRule(rules []PersonalRule, act RuleAct) string {
	if act.CallerHasVia || act.AssigneeNamed || act.CurrentAssignee != "" || act.CallerID == "" {
		return ""
	}
	for _, r := range rules {
		if r.When == act.When && r.Then == RuleAssignMe {
			return act.CallerID
		}
	}
	return ""
}

// AssignCausePersonalRule is data.assignCause on the write's one event.
const AssignCausePersonalRule = "personal_rule"
```

`server.Decode` refuses unknown fields (`H14:136`; `hub-contract:2076`: *"`DecodeJSON` rejects
unknown fields"*), so a `userId` in the body is a 400, not an ignored key. `me.work_rules_set` is
an **audit-only** kind, the kind of event `hub-contract:1112` already allows in the audit chain.

#### The migration: `NNNN_work_later.sql` (new, complete)

The file opens WL's migration; each later WL task that needs schema appends a section below this
one in task order, before the file is applied anywhere shared (rule H1, as task 06's file is for W1).

```sql
-- ── WL: the design's later rows. Sections are appended in task order. ────────────────
-- ── task 176: personal auto-assign rules (05 L-40). One row per person per moment. ──
CREATE TABLE work_personal_rules (
  org_id     text        NOT NULL,
  user_id    text        NOT NULL,
  rule_when  text        NOT NULL,
  rule_then  text        NOT NULL,
  version    integer     NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, user_id, rule_when),
  CONSTRAINT work_personal_rules_when_known CHECK (rule_when IN ('i_create', 'i_accept', 'i_start')),
  CONSTRAINT work_personal_rules_then_known CHECK (rule_then IN ('assign_me'))
);
-- the per-person version counter survives a PUT of an empty list
CREATE TABLE work_personal_rule_versions (
  org_id     text        NOT NULL,
  user_id    text        NOT NULL,
  version    integer     NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, user_id)
);
ALTER TABLE hub.work_personal_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON hub.work_personal_rules TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
ALTER TABLE hub.work_personal_rule_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON hub.work_personal_rule_versions TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT, DELETE ON work_personal_rules TO hub_app;
GRANT SELECT, INSERT ON work_personal_rule_versions TO hub_app;
GRANT UPDATE (version, updated_at) ON work_personal_rule_versions TO hub_app;
```

The row-level security and grant shape is task 06's (`task-06.md:367-387`). A `PUT` is `DELETE`
of the person's rows then `INSERT` of the new list, inside one transaction, so `work_personal_rules`
needs no `UPDATE` grant. Task 06's `TestWorkGrantsAreExactlyTheList` gains the two tables here.

#### The store (`store/work_personal_rules.go`, signatures complete)

```go
package store

// PersonalRules reads the caller's rules; Version 0 and an empty list when none.
func PersonalRules(ctx context.Context, q Querier, userID string) (hubapi.PersonalRules, error)

// PutPersonalRules replaces the caller's list. changed is false when the stored
// list equals rs as a set, in which case the caller emits no event.
func PutPersonalRules(ctx context.Context, tx pgx.Tx, org, userID string, rs []hubapi.PersonalRule, at time.Time) (out hubapi.PersonalRules, changed bool, err error)
```

**The three call sites** (each in its task's file, edited in task order): before the `INSERT` of a
create (task 08), before the `UPDATE` of a `hand` move out of `triage` (task 10, after task 11's
exit check), and before the `UPDATE` of the `started` fact move a person's Start writes (task 22's
path, reached from task 24), the store calls `PersonalRules(ctx, tx, caller)` and
`hubapi.ApplyPersonalRule`; a non-empty answer is written as `assignee_id` in that same statement,
and the event's `data` gains `assigneeId` and `assignCause`. `CallerHasVia` is task 167's
`ActorKindOf(...) != person` over the request's principal (**UNVERIFIED** until 167 lands; until
then, any `X-Panacea-Delegation` header or non-user principal counts as a via).

#### The routes (`server/work_personal_rules.go`)

```go
func (s *Server) registerWorkPersonalRules(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/me/work/rules", s.getPersonalRules)
	mux.HandleFunc("PUT /v1/me/work/rules", s.putPersonalRules)
}
```

`putPersonalRules`, inside `server.Write`: a caller with a via → `server.Forbidden("person")`;
`server.Decode[hubapi.PutPersonalRules]`; `ValidatePersonalRules` → a path → 400 naming it;
`PutPersonalRules`; unchanged → 200, no event; changed → one `me.work_rules_set`. Matrix rows:
both routes 200 for any signed-in person with no via, 403 with one; there is no outsider row,
because the route reads only the caller's own.

#### Interfaces — complete: the delta on ⚑ `desktop/src/api/work.ts` and the *Assign me* section

```ts
// desktop/src/api/work.ts (appended)
export type RuleWhen = "i_create" | "i_accept" | "i_start";
export type PersonalRule = { when: RuleWhen; then: "assign_me" };
export type PersonalRules = { rules: PersonalRule[]; updatedAt: string | null; version: number };
export function getPersonalRules(): Promise<PersonalRules>;
export function putPersonalRules(rules: PersonalRule[]): Promise<PersonalRules>;

// desktop/src/components/settings/AssignMe.tsx
export type AssignMeProps = {
  rules: PersonalRules;
  save: (rules: PersonalRule[]) => Promise<PersonalRules>;
};
/** Three switches, in the person's words:
 *  "Issues I create", "Issues I accept from Triage", "Issues I start".
 *  Each switch is one optimistic PUT of the whole list with Undo (task 56's Undo). */
export function AssignMe(props: AssignMeProps): JSX.Element;

// desktop/src/lib/prefs.ts (task 96), the one-time migration
/** If the stored preferences hold task 96's assign-to-me key as true, PUT the i_create rule
 *  (merged into the current list) and delete the key; on a failed PUT keep the key and try on
 *  the next start. Never writes a Hub record to disk. */
export function migrateAssignPreference(p: { get: () => Prefs; set: (next: Prefs) => void },
  rules: () => Promise<PersonalRules>, put: (r: PersonalRule[]) => Promise<PersonalRules>): Promise<void>;
```

The section heading is *Assign me*, under task 96's `Preferences.tsx`; there is no team-level
control here (team automation is not this task). The Undo is task 56's (skeleton `:458`,
SW.16's `mutate(op) → {undo}`, `:389`); rules are not a replica kind, so whether the section can use
`mutate` or needs its own undo of the previous list is **UNVERIFIED** until task 56's part is read
at the sync base.

#### Tests to write first, part 1 — `hubapi/work_personal_rules_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheRuleAssignsOnTheOwnersOwnCreate` | rules `[i_create]`, act `{When: i_create, CallerID: u-1}` → `u-1` | a rule that never fires |
| `TestANamedAssigneeWins` | the same with `AssigneeNamed: true` → `""` | a rule that overrides a named assignee |
| `TestAnAssignedIssueKeepsItsAssignee` | `i_accept` with `CurrentAssignee: u-2` → `""` | a rule that reassigns |
| `TestAWriteWithAViaGetsNoRule` | `CallerHasVia: true` (Fleet, an app, a key, sync, Teams) → `""` | agents and integrations triggering a person's rule |
| `TestOnlyTheMatchingMomentFires` | rules `[i_start]`, act `i_create` → `""` | one rule firing on every act |
| `TestValidateRefusesADuplicateWhen` | `[i_create, i_create]` → `rules[1].when`; `then: "assign_to"` → `rules[0].then` | two rules for one moment; an unknown action |

#### Tests to write first, part 2 — `server/work_personal_rules_integration_test.go` (`-tags integration`)

The cast is task 11's (`task-11.md:464-471`). Dylan is a member of Portal; Grace too.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestARuleAssignsTheCreatorInTheSameEvent` | Dylan `PUT [i_create]`; Dylan creates with no assignee → head `assigneeId: dylan`; **one** `issue.created` whose `data.assignCause` is `personal_rule`; no `issue.updated` | two events for one act (`hub-contract:1132-1133`) |
| `TestAcceptFromTriageAssignsAndDeclineDoesNot` | `[i_accept]`; a `hand` move `triage → todo` assigns Dylan; `triage → cancelled` does not | a decline that assigns the decliner |
| `TestADelegatedWriteGetsNoRule` | Dylan's rule set; a create through Fleet's delegation for Dylan (`via: fleet`) → no assignee | an agent triggering its owner's rule |
| `TestTheRouteHasNoPersonField` | `PUT {rules: [], userId: "u-grace"}` → 400 (unknown field); Grace's rules unchanged | a `userId` body field that writes another person's rules |
| `TestOnePersonsRuleNeverTouchesAnothersWrite` | Dylan's rule set; Grace creates → no assignee | a rule read by org or team instead of by caller |
| `TestAnUnchangedPutAppendsNoEvent` | the same `PUT` twice → one `me.work_rules_set`; it is absent from `GET /v1/changes` for Portal | an event per `PUT`; a person's settings in a team's feed |

#### Tests to write first, part 3 — `desktop/src/components/settings/AssignMe.test.tsx`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `three switches in words` | renders the three labels; no `userId` or person picker exists | a control that edits someone else's rules |
| `one PUT of the whole list` | toggling *Issues I accept* calls `save` once with both rules | a PATCH-shaped call that drops the other rule |
| `migrates the old preference once` | `prefs` holds the key `true` → one `put` with `i_create`, the key deleted; a second start sends nothing | a device preference and a Hub rule both applying |
| `no prediction` | `expectNoPrediction(container)` | a "2 of 3 rules on" line |

#### Steps

- [ ] **Step 1: confirm the sync base** (rule H1): `feat/hub` sync-merged; tasks 08, 10, 22, 24,
  56, 96 and 167 committed. Read their parts; follow any move of the create, move or Start path,
  of task 96's preference key name, and of task 167's `ActorKindOf`.
- [ ] **Step 2: the pure tests first.** `hubapi/work_personal_rules_test.go` fails to compile;
  append the `hubapi/work.go` block; the six pass.
- [ ] **Step 3: the migration.** Create `NNNN_work_later.sql` at the next free number with the
  section above; add the two tables to task 06's grants test list.
- [ ] **Step 4: the integration tests**, then the store, the routes and the three call sites; run
  until green. Rerun tasks 08, 10 and 22's integration tests, which must stay green (a create with
  no rule set is byte-for-byte the old event).
- [ ] **Step 5: the fake.** The fake calls `ApplyPersonalRule` at its create, move and `started`
  paths; one conformance row (a rule set, a create with no assignee) runs against both.
- [ ] **Step 6: the desktop.** `AssignMe.test.tsx`, then `AssignMe.tsx`, the `api/work.ts` delta and
  `migrateAssignPreference`; task 96's preference test that lists *"only the four keys"* drops to
  three in this task.
- [ ] **Step 7: run the acceptance commands** and commit by path, each message ending with the
  `Co-Authored-By` line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v \
  -run '^Test(TheRuleAssigns|ANamedAssigneeWins|AnAssignedIssueKeeps|AWriteWithAVia|OnlyTheMatchingMoment|ValidateRefusesADuplicate)' 2>&1 | grep -c "^--- PASS"
# expect: 6
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(ARuleAssignsTheCreator|AcceptFromTriageAssigns|ADelegatedWriteGetsNoRule|TheRouteHasNoPersonField|OnePersonsRuleNever|AnUnchangedPutAppends)' 2>&1 | grep -c "^--- PASS"
# expect: 6
go test -tags integration ./control-plane/internal/hub/... -race -count=1 -run 'WorkIssues|WorkStates|Grants|Matrix|Conformance'
# expect: ok for every package
npm --prefix desktop test -- components/settings/AssignMe lib/prefs
# expect: every test passes (4 in AssignMe; task 96's prefs tests with the key list at three)
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- Team-level automation rules, or any rule that acts on someone else's issue.
- Any `then` beyond `assign_me` (no auto-label, no auto-start: L-168's *"on accept, Start"* is a
  loop, task 163, run by the person's Orchestrator within their grid).
- That an explicit *nobody* on create is honoured: task 03's `*string` cannot carry it (gap 1).
- That the rule fires on Start in team mode: a managed session's `started` is written through
  Fleet's delegation, which carries a `via`, so no rule applies there; that is decision 4 working,
  and whether a person's own `Shift+R` in team mode reaches the Hub without a `via` is **UNVERIFIED**
  until TM-05's delegation lands.

**Parity.** Closes **L-40** (`05-design.md:149`, MATCH, slice `later`); task 187 records its
command and output.

#### Gaps found while writing this part

1. **`CreateIssueRequest.AssigneeID` is a `*string`** (`task-03.md:427`), so an absent key and an
   explicit `null` decode alike; *assign nobody* on create cannot be told from *no assignee named*.
   Task 03 could make it tri-state as `PatchIssueRequest.AssigneeID` already is (`task-03.md:47`).
2. **Task 96 puts *assign issues I create to me* on the device**; this task moves it to the Hub
   (decision 1). Task 96's scope and its *"only the four keys"* test should say three.
3. **WL had no migration task**; this part creates `NNNN_work_later.sql`, as the scope says the
   first WL task needing a table must. Later WL parts append to it.
4. **The file map has no settings row** (task 96's scope already records it) and no WL rows.
5. **The scope's `data.cause: "personal_rule"`** would collide with a state move's own
   `data.cause` (task 10's `{from, to, cause, …}`); this part uses `data.assignCause`.
