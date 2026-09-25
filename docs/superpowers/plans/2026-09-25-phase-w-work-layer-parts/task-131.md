### Task 131: phase 7 delta: notes by criterion, freeze per environment, path filters, *shipped* on the issue

W4, a delta on phase 7 under **rule H2: phase 7's files win**, and every phase 7 name below is
**UNVERIFIED until `feat/phase-7-deploy` merges**. Seam **SW.9** (the `Panacea-Issue: is-…` trailer
grammar task 71 already walks). Depends on task 71 (issues in the release manifest). Estimate 0.375
(3 h). Closes **L-145** and **L-148**; **L-147** with task 139 (the drafted prose); **L-144** once
the freeze record lands in the Hub (gap 1).

After this task a release's drawer lists what it settled, by criterion, with the issues that served
each and the evidence keys, plus every other issue it shipped; an admin can freeze an environment,
and Ship refuses a frozen one before any Panacea call, saying who froze it and why; a project can
commit `.panacea/release.yaml` so only commits touching its paths add issues to a release; and an
issue lists every environment it reached, newest first, with the confirm time and pinned brief.

**Phase 7 names verified against its plan on `feat/phase-7-deploy` (2026-09-26).** `{P}` is
`/v1/projects/{projectId}/deploy` (P7 `task-11.md:27`). The file map rows the scope cites by line
are at: `ship/ship.go` `2026-09-23-phase-7-deploy.md:585` (task 09), `sessions/sessions.go` `:587`
(task 19), `api/deploy_drill.go` `:593`, `routes_audit_test.go` `:596`, `ReleaseDrawer.tsx` `:613`
(task 17), `GovernancePanel.tsx` `:614` (task 18). The scope's `:578`-`:607` are stale by 7-8 lines.
`Precondition`, `Orchestrator.Ship`, `Readiness` are P7 `task-09.md:134-179`; `sessions.Range`,
`Result`, `Reader.Read` and the *previous release* rule are P7 `task-19.md:38-171`.
`desktop/src/deploy/types.ts` and `api.ts` (`DeployApi`; task 19 adds `ReleaseSessions`) are P7
`:600`, not `:593`; `ShipBar.tsx`, where the frozen sentence shows, is P7 task 16, `:612`.

**Decisions made here, so nobody re-derives them:**

1. **Notes use phase 7's range, not a second one.** *The previous confirmed release in that
   environment* is task 19's `From`: the commit the environment served just before this row went
   live (`task-19.md:108-137`, rollbacks included). So the route is `GET {P}/notes?deploymentId=…`
   beside task 19's `GET {P}/sessions?deploymentId=…`, not the scope's
   `…/releases/{releaseId}/notes`: phase 7 keys every drill by `deploymentId`, and rule H2 says its
   shape wins. When task 19 answers `previous_unknown`, the notes answer the same state and list
   nothing as *settled by this release*: never a guess.
2. **A criterion is settled by this release** when phase 3's `Evaluator.States` (P3
   `task-15.md:159`, *"phase 7 for a release at Deployment.Commit"*) reads it `settled` at
   `Commit`, and not `settled` at `From`. With `From ""` (no earlier release), every criterion
   settled at `Commit` is listed. The criteria are those of the release's pinned brief revision
   (the Hub's pin, P7 task 09's reserve), resolved by phase 4's resolver as task 23 resolves them.
3. **Every shipped issue appears exactly once.** An issue serving a criterion settled by this
   release is listed under that criterion (under each, if it serves several); every other issue
   of the range is in `alsoShipped`, never dropped. Titles are each issue's head revision title
   read from the Hub at request time, `text·H`, never stored in the manifest (task 71's rule
   stands: the manifest holds ids).
4. **No model text in the notes.** The route calls no gateway and returns only Hub text and ids.
   Task 139 drafts prose from this JSON; nothing here does.
5. **A freeze is a Hub record, and SW.20 has none (gap 1).** The shape is proposed below for
   tasks 114, 116 and 124. Until it lands, `ship.go` reads freezes through a one-method interface,
   `Freezes`, satisfied in tests by a map and in the product by the Hub client once 124 adds it;
   with no implementation wired, **no environment is frozen**, and the Governance panel says the
   control is not yet available rather than drawing a switch that does nothing.
6. **The freeze check is a precondition, after the environment is named and before the reserve.**
   P7 task 09 names the environment in step 4 (`task-09.md:308-312`) and reserves after; the freeze
   check sits between them, so a frozen Ship reserves nothing, builds nothing and calls Panacea not
   at all. It answers **`409 deploy_frozen`**, its own code (like `deploy_environment_invalid`),
   with `details {environment, by, at}`. `Readiness` gains `Reason "frozen"` when its own
   `Environment` is resolved and frozen, so the Ship bar says why before the person clicks.
7. **The path filter is versioned with the code.** `.panacea/release.yaml` is read at `Commit`
   with `git show <Commit>:.panacea/release.yaml` (Class Read). Absent file: no filter. A file
   present but unreadable or with a bad glob: the notes and issue walk answer
   `state "filter_invalid"` with the parse error, and Ship is **not** blocked (a filter shapes a
   list, not a release). This home for the filter is this part's choice; 05 names none
   (**UNVERIFIED pending the owner's review**).
8. ***Shipped* is a list of facts, per environment.** For each environment in task 08's view,
   the issue reached it at the first succeeded deployment, oldest first, whose commit has one of
   the issue's trailer commits as an ancestor; `servingNow` says whether the environment's current
   commit still has it (a rollback can take it away). Staging never reads as production: each row
   names its own environment. Newest first by the reach time.

**Files:**
- `control-plane/internal/deploy/notes/notes.go` — `Notes`, `Build`, the release filter (new)
- `control-plane/internal/deploy/notes/glob.go` — `Match` with `**` (new)
- `control-plane/internal/deploy/notes/notes_test.go` — the Go tests below (new)
- `control-plane/internal/deploy/sessions/sessions.go` ⚑ (P7 task 19; task 71's `ReleaseIssue`
  walk) — `--name-only` when a filter is set; `ReleaseIssue` carries `paths` (modified)
- `control-plane/internal/deploy/ship/ship.go` ⚑ (P7 task 09) — `Freezes`, the check between step
  4 and the reserve, `Readiness` reason `frozen` (modified)
- `control-plane/internal/api/deploy_drill.go` ⚑ — `GET {P}/notes`, `GET {P}/shipped` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — two `catalogReadOnlyRoutes` lines (modified)
- `desktop/src/deploy/types.ts`, `api.ts` ⚑ (P7 tasks 13, 19) — `ReleaseNotes`, `IssueShipped`,
  `EnvironmentFreeze`; `releaseNotes`, `issueShipped` (modified)
- `desktop/src/deploy/ReleaseNotes.tsx` (+ test) — in ⚑ `ReleaseDrawer.tsx` (new)
- `desktop/src/deploy/FreezeToggle.tsx` (+ test) — in ⚑ `GovernancePanel.tsx`; the frozen line
  in ⚑ `ShipBar.tsx` (new)
- `desktop/src/deploy/IssueShipped.tsx` (+ test) — the per-environment list under task 71's head
  line in ⚑ `desktop/src/components/issue/IssueView.tsx` (task 46) (new; not in the scope's file
  list, gap 4)
- `desktop/src/components/desk/deskRows.tsx` ⚑ — `issue.shipped`'s second-environment case
  (modified)

#### Interfaces — complete: the proposed Hub freeze record (for tasks 114, 116, 124: gap 1)

```go
// hubapi — proposed. A freeze is a decision others must see (05 §3.1), so it is a Hub record.
type EnvironmentFreeze struct {
	ProjectID   string       `json:"projectId"`   // pj-…
	Environment string       `json:"environment"` // Panacea's own name (P7 task 09 step 4)
	Reason      AuthoredText `json:"reason"`      // text·H, 1-280 characters
	By          User         `json:"by"`          // a team admin
	At          time.Time    `json:"at"`
}

// PUT    /v1/projects/{projectId}/freezes/{env}   body {reason}  → 200 EnvironmentFreeze (admin)
// DELETE /v1/projects/{projectId}/freezes/{env}                  → 204 (admin; 404 freeze_not_found)
// GET    /v1/projects/{projectId}/freezes                         → 200 {items: EnvironmentFreeze[]}
// Events: "freeze.set", "freeze.cleared", subject the project. 403 for a non-admin, a granted
// team included (task 117's read-and-comment grant writes nothing).
```

#### Interfaces — complete: the delta on ⚑ `ship/ship.go`

```go
// Freezes is the one read Ship needs. nil (the default until task 124 wires the Hub client):
// nothing is frozen, and the Governance panel says the control is not available yet (decision 5).
type Freezes interface {
	// Freeze answers nil, nil when env is not frozen. An error fails closed: Ship refuses
	// with 503 upstream_unavailable (details.upstream "hub"), never ships past an unread freeze.
	Freeze(ctx context.Context, hubProjectID, env string) (*hubapi.EnvironmentFreeze, error)
}

// Config gains: Freezes Freezes

// FrozenError is the refusal. Mapped to 409 deploy_frozen, details {environment, by, at}.
type FrozenError struct{ F hubapi.EnvironmentFreeze }

func (e *FrozenError) Error() string {
	return fmt.Sprintf("%s is frozen by %s since %s · %s",
		e.F.Environment, e.F.By.Name, e.F.At.Format("15:04"), e.F.Reason.Text)
}

// checkFreeze runs after step 4 names env and before step 5 reserves (decision 6). Readiness
// calls the same function when its Environment is resolved, with Reason "frozen".
func (o *Orchestrator) checkFreeze(ctx context.Context, hubProjectID, env string) error {
	if o.cfg.Freezes == nil || hubProjectID == "" {
		return nil
	}
	f, err := o.cfg.Freezes.Freeze(ctx, hubProjectID, env)
	if err != nil {
		return &deploy.Error{Code: "upstream_unavailable", Status: 503, Details: map[string]string{"upstream": "hub"}}
	}
	if f != nil {
		return &FrozenError{F: *f}
	}
	return nil
}
```

`deploy.Error`'s field names are P7 task 04's (**UNVERIFIED**; the pattern is `task-09.md:480`'s
error mapping). The message's time is the person's zone in the desktop; the Go sentence is the
log's and uses the control plane's.

#### Interfaces — complete: `control-plane/internal/deploy/notes/notes.go`

```go
// Package notes builds a release's notes by criterion (05 §1 K, L-147) from facts only: phase
// 3's criterion states at two commits, task 71's issue walk, and Hub issue heads. No model text.
package notes

import (
	"context"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/briefref"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/sessions"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/evidence"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

type Notes struct {
	State        string             `json:"state"`   // "ok" | task 19's states | "filter_invalid" | "no_pin"
	Message      string             `json:"message"` // "" for ok
	DeploymentID string             `json:"deploymentId"`
	PinnedBrief  string             `json:"pinnedBrief"` // "brief:bf-…@6"; "" when the release pinned none
	From         string             `json:"from"`
	Commit       string             `json:"commit"`
	Settled      []SettledCriterion `json:"settled"`     // brief order; [] never null
	AlsoShipped  []IssueRef         `json:"alsoShipped"` // walk order; [] never null
	Truncated    bool               `json:"truncated"`   // task 19's MaxCommits reached
}

// SettledCriterion is settled at Commit and was not at From (decision 2).
type SettledCriterion struct {
	CriterionID string         `json:"criterionId"`
	Text        string         `json:"text"`   // the pinned revision's criterion text, text·H
	Issues      []ServingIssue `json:"issues"` // [] when settled by work no shipped issue serves
}

type ServingIssue struct {
	IssueRef
	Evidence []string `json:"evidence"` // "evidence:<id>" keys of the deciding records this issue filed
}

type IssueRef struct {
	IssueID string `json:"issueId"` // is-…
	Alias   string `json:"alias"`   // EP-142
	Title   string `json:"title"`   // the head revision's title text, read now (decision 3)
}

// The four reads Build makes; each is an existing seam, named by its owner.
type (
	// Walk is task 71's trailer walk over task 19's range, with task 131's filter.
	Walk func(ctx context.Context, r sessions.Range) (sessions.IssueResult, error)
	// States is phase 3's Evaluator.States (P3 task-15.md:159).
	States func(ctx context.Context, subject string, criteria []briefref.Criterion) ([]evidence.State, error)
	// Criteria resolves the pinned brief revision to its criteria in brief order, with text
	// (phase 4's resolver, as task 23 calls it; briefref.Criterion is {ID, Digest}, P3 task-03.md:509).
	Criteria func(ctx context.Context, briefKey string) ([]PinnedCriterion, error)
	// Heads reads Hub issue heads and head revisions for the ids.
	Heads func(ctx context.Context, ids []string) (map[string]IssueFacts, error)
	// RecordIssues maps deciding record ids to the issue each cites (task 23's Record.IssueID).
	RecordIssues func(ctx context.Context, recordIDs []string) (map[string]string, error)
)

type PinnedCriterion struct {
	briefref.Criterion
	Text string
}

type IssueFacts struct {
	Alias  string
	Title  string
	Serves []hubapi.Serve // the head revision's, SW.1 (task-02.md:232)
}

type Deps struct {
	Walk         Walk
	States       States
	Criteria     Criteria
	Heads        Heads
	RecordIssues RecordIssues
}

// Build answers the notes for one release row. r is task 19's range, found by task 19's rule.
func Build(ctx context.Context, d Deps, r sessions.Range, deploymentID, pinnedBrief string) (Notes, error) {
	n := Notes{State: "ok", DeploymentID: deploymentID, PinnedBrief: pinnedBrief, From: r.From,
		Commit: r.Commit, Settled: []SettledCriterion{}, AlsoShipped: []IssueRef{}}
	walk, err := d.Walk(ctx, r)
	if err != nil {
		return n, err
	}
	n.Truncated = walk.Truncated
	if walk.State != "ok" { // previous_unknown, commit_not_local, filter_invalid: nothing guessed
		n.State, n.Message = walk.State, walk.Message
		return n, nil
	}
	ids := make([]string, 0, len(walk.Issues))
	for _, ri := range walk.Issues {
		ids = append(ids, ri.IssueID)
	}
	heads, err := d.Heads(ctx, ids)
	if err != nil {
		return n, err
	}
	ref := func(id string) IssueRef { h := heads[id]; return IssueRef{IssueID: id, Alias: h.Alias, Title: h.Title} }
	listed := map[string]bool{}
	if pinnedBrief == "" {
		n.State, n.Message = "no_pin", "This release pinned no brief, so nothing is listed by criterion."
	} else if err := settled(ctx, d, r, pinnedBrief, ids, heads, ref, listed, &n); err != nil {
		return n, err
	}
	for _, id := range ids { // decision 3: every shipped issue appears once
		if !listed[id] {
			n.AlsoShipped = append(n.AlsoShipped, ref(id))
		}
	}
	return n, nil
}

func settled(ctx context.Context, d Deps, r sessions.Range, pin string, ids []string,
	heads map[string]IssueFacts, ref func(string) IssueRef, listed map[string]bool, n *Notes) error {
	crit, err := d.Criteria(ctx, pin)
	if err != nil {
		return err
	}
	bare := make([]briefref.Criterion, len(crit))
	for i, c := range crit {
		bare[i] = c.Criterion
	}
	now, err := d.States(ctx, r.Commit, bare)
	if err != nil {
		return err
	}
	before := map[string]string{}
	if r.From != "" {
		prev, err := d.States(ctx, r.From, bare)
		if err != nil {
			return err
		}
		for _, s := range prev {
			before[s.CriterionID] = s.State
		}
	}
	at := map[string]evidence.State{}
	var recs []string
	for _, s := range now {
		at[s.CriterionID] = s
		recs = append(recs, s.Evidence...)
	}
	cites, err := d.RecordIssues(ctx, recs)
	if err != nil {
		return err
	}
	briefID := briefIDOf(pin)
	for _, c := range crit { // brief order
		s, ok := at[c.ID]
		if !ok || s.State != "settled" || before[c.ID] == "settled" { // decision 2
			continue
		}
		sc := SettledCriterion{CriterionID: c.ID, Text: c.Text, Issues: []ServingIssue{}}
		for _, id := range ids {
			if !serves(heads[id].Serves, briefID, c.ID) {
				continue
			}
			ev := []string{}
			for _, rec := range s.Evidence {
				if cites[rec] == id {
					ev = append(ev, "evidence:"+rec)
				}
			}
			sc.Issues = append(sc.Issues, ServingIssue{IssueRef: ref(id), Evidence: ev})
			listed[id] = true
		}
		n.Settled = append(n.Settled, sc)
	}
	return nil
}

// briefIDOf("brief:bf-1a2b@6") == "bf-1a2b". A serve of any revision of the pinned brief counts:
// criterion ids are positional, and an edited criterion gets a new id (05:754-756).
func briefIDOf(key string) string {
	k := strings.TrimPrefix(key, "brief:")
	if i := strings.IndexByte(k, '@'); i >= 0 {
		k = k[:i]
	}
	return k
}

func serves(ss []hubapi.Serve, briefID, criterionID string) bool {
	for _, s := range ss {
		if briefIDOf(s.BriefKey) == briefID && s.CriterionID == criterionID {
			return true
		}
	}
	return false
}
```
