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
- `control-plane/internal/deploy/notes/filter/filter.go` (+ `filter_test.go`) — `ParseFilter`,
  `Touches`, `Match` with `**` (new)
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

#### Interfaces — complete: `control-plane/internal/deploy/notes/filter/filter.go`

```go
// Package filter is the release path filter. Its own package, because both sessions (the walk)
// and notes import it, and notes imports sessions.
package filter

import (
	"path"
	"strings"

	"gopkg.in/yaml.v3" // already in go.mod v3.0.1 as indirect; becomes direct
)

// FilterFile is the committed path: read at the released commit, never from the working tree.
const FilterFile = ".panacea/release.yaml"

// Filter is the file's one key. Empty Paths (or no file) is no filter: every commit counts.
type Filter struct {
	Paths []string `yaml:"paths"`
}

// ParseFilter reads the file's bytes. An unknown key, a non-list or a bad glob is an error the
// walk reports as state "filter_invalid" with this message (decision 7).
func ParseFilter(b []byte) (Filter, error) {
	var f Filter
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return Filter{}, err
	}
	for _, p := range f.Paths {
		for _, seg := range strings.Split(p, "/") {
			if seg == "**" {
				continue
			}
			if _, err := path.Match(seg, seg); err != nil { // ErrBadPattern, e.g. "[a-"
				return Filter{}, err
			}
		}
	}
	return f, nil
}

// Touches reports whether any of files matches any of f.Paths. No paths: always true.
func (f Filter) Touches(files []string) bool {
	if len(f.Paths) == 0 {
		return true
	}
	for _, name := range files {
		for _, p := range f.Paths {
			if ok, _ := Match(p, name); ok {
				return true
			}
		}
	}
	return false
}

// Match reports whether name (repo-relative, slash-separated, as git prints it) matches pattern.
// "**" matches any number of whole segments, none included; any other segment is path.Match.
func Match(pattern, name string) (bool, error) {
	return match(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func match(p, n []string) (bool, error) {
	for len(p) > 0 {
		if p[0] == "**" {
			for i := 0; i <= len(n); i++ {
				if ok, err := match(p[1:], n[i:]); ok || err != nil {
					return ok, err
				}
			}
			return false, nil
		}
		if len(n) == 0 {
			return false, nil
		}
		if ok, err := path.Match(p[0], n[0]); err != nil || !ok {
			return false, err
		}
		p, n = p[1:], n[1:]
	}
	return len(n) == 0, nil
}
```

#### Interfaces — complete: the delta on ⚑ `sessions/sessions.go` (P7 task 19; task 71's walk)

```go
// Task 71 adds ReleaseIssue beside Session; this task adds the filter to the same walk, so
// sessions and issues are read from one git log (decision 7 keeps it one command).
type ReleaseIssue struct {
	IssueID string   `json:"issueId"` // is-…, SW.9's trailer value
	Commits []string `json:"commits"` // the range's commits carrying it, newest first
}

type IssueResult struct {
	State     string         `json:"state"` // task 19's states, plus "filter_invalid"
	Message   string         `json:"message"`
	Issues    []ReleaseIssue `json:"issues"` // [] never null
	Filtered  int            `json:"filtered"` // trailer commits dropped by the filter: a count, shown
	Truncated bool           `json:"truncated"`
}

// ReadIssues runs, after task 19's two cat-file checks:
//  1. git show <Commit>:.panacea/release.yaml   exit 128 "does not exist" → no filter; other bytes →
//     filter.ParseFilter; an error → State "filter_invalid", Message "release.yaml: <err>", no walk.
//  2. the same git log as Read, with the trailer format for TrailerIssue and, only when a filter is
//     set, --name-only, so each record carries its paths. A commit whose paths !f.Touches adds no
//     issue and counts in Filtered. A merge commit lists no paths under --name-only and is kept
//     only when its own trailer names an issue (its parents' commits are walked on their own).
func (rd *Reader) ReadIssues(ctx context.Context, r Range) (IssueResult, error)
```

`gitx.TrailerIssue` and `gitx.TrailerFormat` are task 21's names for SW.9's key
(`task-21.md:56`, `:99`). Step 1's parse is `filter.ParseFilter`, and step 2's test is `Filter.Touches`.

#### Control-plane routes — complete wire, on ⚑ `api/deploy_drill.go`

```
GET {P}/notes?deploymentId=engagement-portal-build-x7k2p
  200 Notes, e.g.
      { "state": "ok", "message": "", "deploymentId": "engagement-portal-build-x7k2p",
        "pinnedBrief": "brief:bf-1a2b3c4d5e6f7a8b@6", "from": "<40 hex>", "commit": "<40 hex>",
        "settled": [ { "criterionId": "done:2", "text": "Uploads survive a dropped connection",
                       "issues": [ { "issueId": "is-…", "alias": "EP-142", "title": "Resume uploads",
                                     "evidence": ["evidence:ev-…"] } ] } ],
        "alsoShipped": [ { "issueId": "is-…", "alias": "EP-150", "title": "Fix footer spacing" } ],
        "truncated": false }
  400 bad_request        deploymentId missing, or outside [a-z0-9.-]            (task 19's check)
  404 deploy_not_found   no row with that deploymentId in the view             (task 19's)
  409 deploy_precondition details.reason "no_commit"                           (task 19's)
  503 upstream_unavailable the watcher has no snapshot (details.upstream "panacea"), or the Hub
                         did not answer for the heads (details.upstream "hub")
  States in a 200 that list nothing as settled: "previous_unknown", "commit_not_local",
  "previous_not_local" (task 19's sentences), "filter_invalid", "no_pin".

GET {P}/shipped?issueId=is-0123456789abcdef
  200 IssueShipped, e.g.
      { "issueId": "is-…", "state": "ok", "message": "",
        "environments": [
          { "environment": "production", "deploymentId": "…-k3m9q", "reachedAt": "2026-09-26T04:02:00Z",
            "pinnedBrief": "brief:bf-…@6", "commit": "<40 hex>", "servingNow": true },
          { "environment": "staging", "deploymentId": "…-p2r7t", "reachedAt": "2026-09-25T23:40:00Z",
            "pinnedBrief": "brief:bf-…@6", "commit": "<40 hex>", "servingNow": true } ] }
  400 bad_request        issueId not ^is-[0-9a-f]{16}$
  503 upstream_unavailable no snapshot yet (details.upstream "panacea")
  A 200 with "environments": [] and state "ok" is *"Landed, not shipped yet"* (task 71's line).
```

```go
// deploy_drill.go ⚑ — the shipped answer (decision 8).
type ShippedEnv struct {
	Environment  string    `json:"environment"`  // the view's environmentRow, else the raw name (task 19's join)
	DeploymentID string    `json:"deploymentId"`
	ReachedAt    time.Time `json:"reachedAt"`    // the deployment's CompletedAt: its go-live, task 19's cut
	PinnedBrief  string    `json:"pinnedBrief"`  // the Hub release's pin; "" when none
	Commit       string    `json:"commit"`
	ServingNow   bool      `json:"servingNow"`   // the environment's current commit still has it
}
type IssueShipped struct {
	IssueID      string       `json:"issueId"`
	State        string       `json:"state"`   // "ok" | "commit_not_local" (a deployment's commit is not in this clone)
	Message      string       `json:"message"`
	Environments []ShippedEnv `json:"environments"` // newest ReachedAt first; [] never null
}
```

**How `shipped` is read**, with no Hub write and at most `1 + 2·E + D` git commands for `E`
environments and `D` deployments tried (each Class Read):
1. `git log --no-color --max-count=201 --format=%H%x00<TrailerFormat(TrailerIssue)>%x1e <newest
   succeeded commit of any environment>` finds the commits carrying the issue's trailer (the
   issue's commits). None: `environments: []`.
2. Per environment, its `succeeded` rows oldest `CompletedAt` first: `git merge-base
   --is-ancestor <issueCommit> <row.Commit>` for the issue's oldest commit; the first yes is the
   reach. Rows past task 08's 200-row history are not read (does-not-claim).
3. `servingNow`: the same test against `deploywatch.CurrentAt(…, env, now)`'s commit.

The two `catalogReadOnlyRoutes` lines, in task 19's form: *"notes: reads criterion states at two
commits with phase 3's evaluator, the release filter with git show, and issue heads from the Hub;
writes nothing."* and *"shipped: reads Panacea-Issue trailers and ancestry from the local clone
with git log and merge-base (Class Read, no optional locks); writes nothing."*

#### Interfaces — complete: the deltas on ⚑ `desktop/src/deploy/types.ts` and `api.ts`

```ts
// types.ts ⚑ — field for field with the Go above.
export type NotesState = "ok" | "previous_unknown" | "commit_not_local" | "previous_not_local"
  | "filter_invalid" | "no_pin";
export interface IssueRef { issueId: string; alias: string; title: string }
export interface ServingIssue extends IssueRef { evidence: string[] }
export interface SettledCriterion { criterionId: string; text: string; issues: ServingIssue[] }
export interface ReleaseNotes {
  state: NotesState; message: string; deploymentId: string; pinnedBrief: string;
  from: string; commit: string; settled: SettledCriterion[]; alsoShipped: IssueRef[]; truncated: boolean;
}
export interface ShippedEnv {
  environment: string; deploymentId: string; reachedAt: string; pinnedBrief: string;
  commit: string; servingNow: boolean;
}
export interface IssueShipped { issueId: string; state: "ok" | "commit_not_local"; message: string; environments: ShippedEnv[] }
/** The proposed Hub record (gap 1); AuthoredText and User are task 37's. */
export interface EnvironmentFreeze { projectId: string; environment: string; reason: AuthoredText; by: User; at: string }
/** Null until tasks 114/124/125 add the freeze rows (decision 5): the toggle then says so. */
export interface FreezeApi {
  freezes(hubProjectId: string): Promise<EnvironmentFreeze[]>;
  set(hubProjectId: string, env: string, reason: string): Promise<EnvironmentFreeze>;
  clear(hubProjectId: string, env: string): Promise<void>;
}
```

```ts
// api.ts ⚑ — DeployApi gains two reads beside task 19's releaseSessions, in its idiom.
  releaseNotes: (projectId: string, deploymentId: string) =>
    get<ReleaseNotes>(`/v1/projects/${enc(projectId)}/deploy/notes?deploymentId=${enc(deploymentId)}`),
  issueShipped: (projectId: string, issueId: string) =>
    get<IssueShipped>(`/v1/projects/${enc(projectId)}/deploy/shipped?issueId=${enc(issueId)}`),
// client.ts's ProblemCode gains "deploy_frozen" (P7 task 13 owns the list).
```

`get` and `enc` are P7 task 13's helper names as its file map implies them (**UNVERIFIED**).

#### Interfaces — complete: `desktop/src/deploy/ReleaseNotes.tsx`

```tsx
// desktop/src/deploy/ReleaseNotes.tsx — the drawer's notes block. Facts only; the drafted prose
// is task 139's and is not rendered here.
import type { JSX } from "react";
import type { ReleaseNotes as Notes } from "./types";
import { KeyLink } from "../components/common/KeyLink";           // as task 127 imports it
import { copyMarkdown } from "../components/editor/copyMarkdown";  // task 92 (UNVERIFIED name)

/** The Markdown a person copies: the Hub key on every line (task 92's rule), no drafted text. */
export function notesMarkdown(n: Notes): string {
  const out = [`Release notes · pinned ${n.pinnedBrief || "no brief"}`, ""];
  for (const c of n.settled) {
    out.push(`## ${c.text} (${c.criterionId})`);
    for (const i of c.issues) out.push(`- ${i.alias} ${i.title} (issue:${i.issueId})${i.evidence.length ? " · " + i.evidence.join(", ") : ""}`);
    if (c.issues.length === 0) out.push("- settled by work no shipped issue serves");
  }
  if (n.alsoShipped.length) {
    out.push("", "## Also shipped");
    for (const i of n.alsoShipped) out.push(`- ${i.alias} ${i.title} (issue:${i.issueId})`);
  }
  return out.join("\n");
}

export function ReleaseNotes({ notes }: { notes: Notes }): JSX.Element {
  if (notes.state !== "ok" && notes.state !== "no_pin") {
    return <section className="rn"><h3>Notes</h3><p className="rn-fact">{notes.message}</p></section>;
  }
  return (
    <section className="rn" aria-label="Release notes">
      <header className="rn-head">
        <h3>Settled by this release</h3>
        <KeyLink k={notes.pinnedBrief} />
        <button type="button" onClick={() => copyMarkdown(notesMarkdown(notes))}>Copy as Markdown</button>
      </header>
      {notes.state === "no_pin" && <p className="rn-fact">{notes.message}</p>}
      {notes.settled.length === 0 && notes.state === "ok" && <p className="rn-fact">No criterion settled with this release.</p>}
      {notes.settled.map((c) => (
        <div key={c.criterionId} className="rn-crit">
          <p className="rn-ctext">{c.text}</p>
          <ul>{c.issues.map((i) => (
            <li key={i.issueId}><span className="rn-alias">{i.alias}</span> {i.title}
              {i.evidence.map((k) => <KeyLink key={k} k={k} />)}</li>
          ))}</ul>
        </div>
      ))}
      {notes.alsoShipped.length > 0 && (
        <div className="rn-also"><h4>Also shipped</h4>
          <ul>{notes.alsoShipped.map((i) => <li key={i.issueId}><span className="rn-alias">{i.alias}</span> {i.title}</li>)}</ul>
        </div>
      )}
      {notes.truncated && <p className="rn-fact">Only the newest 200 commits were read.</p>}
    </section>
  );
}
```

#### Interfaces — complete: `desktop/src/deploy/FreezeToggle.tsx`

```tsx
// desktop/src/deploy/FreezeToggle.tsx — in GovernancePanel, one row per environment. A freeze is
// a person's decision with a reason; lifting it is too. Admins only (the Hub decides; this offers).
import { useState, type JSX } from "react";
import type { EnvironmentFreeze, FreezeApi } from "./types";

/** The Ship bar's line and the panel's: who, since when, why. */
export function frozenLine(f: EnvironmentFreeze, hhmm: (iso: string) => string): string {
  return `${f.environment} is frozen by ${f.by.name} since ${hhmm(f.at)} · ${f.reason.text}`;
}

export function FreezeToggle(p: {
  api: FreezeApi | null; hubProjectId: string; environment: string;
  freeze: EnvironmentFreeze | null; isAdmin: boolean; hhmm(iso: string): string; onChanged(): void;
}): JSX.Element {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  if (p.api === null) {
    return <p className="gv-fact">Freezing {p.environment} is not available yet: the Hub does not hold freezes.</p>;
  }
  const api = p.api;
  if (p.freeze) {
    return (
      <div className="gv-freeze" data-frozen="">
        <p>{frozenLine(p.freeze, p.hhmm)}</p>
        {p.isAdmin && <button type="button" onClick={() =>
          api.clear(p.hubProjectId, p.environment).then(p.onChanged, (e: Error) => setError(e.message))}>Lift freeze</button>}
        {error && <p className="gv-fact" role="status">{error}</p>}
      </div>
    );
  }
  if (!p.isAdmin) return <p className="gv-fact">{p.environment} is not frozen.</p>;
  return (
    <form className="gv-freeze" onSubmit={(e) => {
      e.preventDefault();
      api.set(p.hubProjectId, p.environment, reason.trim()).then(p.onChanged, (err: Error) => setError(err.message));
    }}>
      <label>Reason <input value={reason} maxLength={280} onChange={(e) => setReason(e.target.value)} required /></label>
      <button type="submit" disabled={reason.trim() === ""}>Freeze {p.environment}</button>
      {error && <p className="gv-fact" role="status">{error}</p>}
    </form>
  );
}
```

`ShipBar.tsx` ⚑ (P7 task 16) shows `frozenLine` in place of its readiness sentence when
`readiness.reason === "frozen"`, and the Ship button stays disabled. The Governance panel reads the
admin role from `/v1/me` as task 126's `useIsTeamAdmin` does (`task-126.md:90-93`).

#### Interfaces — complete: `desktop/src/deploy/IssueShipped.tsx`, and the Desk case

```tsx
// desktop/src/deploy/IssueShipped.tsx — under task 71's head line on the issue view. Every
// environment the issue reached, newest first, each with its own name (decision 8).
import type { JSX } from "react";
import type { IssueShipped as Shipped } from "./types";
import { KeyLink } from "../components/common/KeyLink";

export function IssueShipped({ s, hhmm }: { s: Shipped; hhmm(iso: string): string }): JSX.Element {
  if (s.state !== "ok") return <p className="is-fact">{s.message}</p>;
  if (s.environments.length === 0) return <p className="is-fact">Landed, not shipped yet</p>;
  return (
    <ul className="is-shipped" aria-label="Shipped to">
      {s.environments.map((e) => (
        <li key={e.deploymentId}>
          <span>shipped to {e.environment}, </span><time className="is-at" dateTime={e.reachedAt}>{hhmm(e.reachedAt)}</time>
          {e.pinnedBrief && <> · pinned <KeyLink k={e.pinnedBrief} /></>}
          {!e.servingNow && <span className="is-fact"> · no longer serving</span>}
        </li>
      ))}
    </ul>
  );
}
```

`deskRows.tsx` ⚑: task 71's `issue.shipped` row gains its second case. When an issue already
shipped to one environment reaches another, the row names the new one: *"EP-142 shipped to
production, 14:02"* after *"… staging, 09:40"*. The ranking and the hold are task 62's. `.is-at` is
mono with `tabular-nums`.

#### Tests to write first — `control-plane/internal/deploy/notes/notes_test.go`

```go
package notes

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/briefref"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/sessions"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/evidence"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

const pin = "brief:bf-1a2b3c4d5e6f7a8b@6"

// fixture: done:1 settled at both commits; done:2 settled only at Commit; EP-142 serves done:2,
// EP-150 serves nothing, EP-160 serves done:1.
func fixture() Deps {
	st := func(id, s string, ev ...string) evidence.State {
		return evidence.State{CriterionID: id, State: s, Evidence: ev}
	}
	return Deps{
		Walk: func(context.Context, sessions.Range) (sessions.IssueResult, error) {
			return sessions.IssueResult{State: "ok", Issues: []sessions.ReleaseIssue{
				{IssueID: "is-142"}, {IssueID: "is-150"}, {IssueID: "is-160"}}}, nil
		},
		States: func(_ context.Context, at string, _ []briefref.Criterion) ([]evidence.State, error) {
			if at == "c-from" {
				return []evidence.State{st("done:1", "settled"), st("done:2", "open")}, nil
			}
			return []evidence.State{st("done:1", "settled", "ev-1"), st("done:2", "settled", "ev-2", "ev-9")}, nil
		},
		Criteria: func(context.Context, string) ([]PinnedCriterion, error) {
			return []PinnedCriterion{{briefref.Criterion{ID: "done:1"}, "the resume test passes"},
				{briefref.Criterion{ID: "done:2"}, "Uploads survive a dropped connection"}}, nil
		},
		Heads: func(context.Context, []string) (map[string]IssueFacts, error) {
			return map[string]IssueFacts{
				"is-142": {Alias: "EP-142", Title: "Resume uploads", Serves: []hubapi.Serve{{BriefKey: "brief:bf-1a2b3c4d5e6f7a8b@5", CriterionID: "done:2"}}},
				"is-150": {Alias: "EP-150", Title: "Fix footer spacing", Serves: []hubapi.Serve{}},
				"is-160": {Alias: "EP-160", Title: "Resume test", Serves: []hubapi.Serve{{BriefKey: pin, CriterionID: "done:1"}}},
			}, nil
		},
		RecordIssues: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{"ev-2": "is-142", "ev-9": "is-777"}, nil
		},
	}
}

func build(t *testing.T, from string) Notes {
	t.Helper()
	n, err := Build(context.Background(), fixture(), sessions.Range{From: from, Commit: "c-now"}, "d-1", pin)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// mutant killed: "every settled criterion" (done:1 was settled at the previous release)
func TestOnlyCriteriaThisReleaseSettledAreListed(t *testing.T) {
	n := build(t, "c-from")
	if len(n.Settled) != 1 || n.Settled[0].CriterionID != "done:2" {
		t.Fatalf("settled = %+v, want only done:2", n.Settled)
	}
	if got := n.Settled[0].Issues; len(got) != 1 || got[0].Alias != "EP-142" ||
		strings.Join(got[0].Evidence, ",") != "evidence:ev-2" { // ev-9 cites another issue
		t.Fatalf("issues = %+v", got)
	}
	if first := build(t, ""); len(first.Settled) != 2 { // no earlier release: both listed
		t.Fatalf("From \"\": %d settled, want 2", len(first.Settled))
	}
}

// mutant killed: "unserved work vanishes" (EP-150 serves nothing; EP-160's criterion was settled before)
func TestEveryShippedIssueAppearsOnce(t *testing.T) {
	n := build(t, "c-from")
	var also []string
	for _, i := range n.AlsoShipped {
		also = append(also, i.Alias)
	}
	if strings.Join(also, ",") != "EP-150,EP-160" {
		t.Fatalf("alsoShipped = %v, want EP-150,EP-160", also)
	}
}

// mutant killed: "a drafted line passed off as text·H" — the JSON has exactly the fact keys, and
// the package cannot reach the gateway.
func TestNotesCarryNoModelText(t *testing.T) {
	b, _ := json.Marshal(build(t, "c-from"))
	var top map[string]json.RawMessage
	_ = json.Unmarshal(b, &top)
	want := "alsoShipped,commit,deploymentId,from,message,pinnedBrief,settled,state,truncated"
	var got []string
	for k := range top {
		got = append(got, k)
	}
	if strings.Join(sorted(got), ",") != want {
		t.Fatalf("keys = %v", sorted(got))
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "/gateway") || strings.Contains(string(out), "/inference") {
		t.Fatal("notes imports a gateway or inference package")
	}
}

func sorted(s []string) []string { // tiny insertion sort: no sort import for one test
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}
```

#### Tests to write first — the table: `filter_test.go`, ⚑ `sessions_test.go`, ⚑ `ship_test.go`, the routes

| Test | Setup and expectation | Mutant killed |
|---|---|---|
| `TestFilterDropsCommitsOutsideItsPaths` (sessions) | a temp repo; `.panacea/release.yaml` `paths: ["api/**"]` at `Commit`; commit A touches `web/x.ts` with `Panacea-Issue: is-…01`, B touches `api/y.go` with `…02` → `Issues` is `[…02]`, `Filtered` 1 | filter ignored |
| `TestNoFilterFileMeansNoFilter` (sessions) | the same repo without the file → both issues, `Filtered` 0 | no file ships nothing |
| `TestABadFilterNamesItself` (sessions) | `paths: ["[a-"]` → `State "filter_invalid"`, `Message` starts `release.yaml:`, no `git log` run (the runner's call log) | a bad glob silently matching nothing |
| `TestDoubleStarMatchesZeroOrMoreSegments` (filter) | `api/**` matches `api/y.go`, `api/v1/y.go` and not `apix/y.go`; `**/*.sql` matches `db.sql` and `a/b/db.sql` | `**` as one segment; prefix match |
| `TestShipToAFrozenEnvironmentCallsNothing` (ship) | `Freezes` answers a freeze for `production`; Ship → `*FrozenError`, 409 `deploy_frozen`, `details.environment "production"`; the Hub fake's reserve count and the Panacea fake's call count are 0 | freeze checked after deploy |
| `TestAnUnreadFreezeFailsClosed` (ship) | `Freezes` errors → 503 `upstream_unavailable` `details.upstream "hub"`; nothing reserved | shipping past a freeze nobody read |
| `TestReadinessSaysFrozen` (ship) | `Environment` resolved and frozen → `Ready false`, `Reason "frozen"` | the button enabled until the click |
| `TestShippedNamesEachEnvironment` (route) | staging reached 09:40, production 14:02 → two rows, production first, each its own `environment`; an issue in staging only → one row, `staging` | first environment wins |
| `TestShippedAfterARollback` (route) | production reached at d2, then a rollback to d1 → `servingNow false` | shipped read as serving |
| the catalog test (⚑ `routes_audit_test.go`) | both `GET`s have a `catalogReadOnlyRoutes` line | an undocumented read route |

The Ship harness is P7 task 09's (`task-09.md:440-460`, **UNVERIFIED** helper names); each row
reuses it and adds only `Config.Freezes`.

#### Tests to write first — `ReleaseNotes.test.tsx`, `FreezeToggle.test.tsx`, `IssueShipped.test.tsx`

```tsx
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { expectNoPrediction } from "../test/expectNoPrediction";
import { ReleaseNotes, notesMarkdown } from "./ReleaseNotes";
import { FreezeToggle, frozenLine } from "./FreezeToggle";
import { IssueShipped } from "./IssueShipped";
import type { EnvironmentFreeze, ReleaseNotes as Notes } from "./types";

const notes: Notes = { state: "ok", message: "", deploymentId: "d-1", pinnedBrief: "brief:bf-1@6",
  from: "a".repeat(40), commit: "b".repeat(40), truncated: false,
  settled: [{ criterionId: "done:2", text: "Uploads survive a dropped connection",
    issues: [{ issueId: "is-142", alias: "EP-142", title: "Resume uploads", evidence: ["evidence:ev-2"] }] }],
  alsoShipped: [{ issueId: "is-150", alias: "EP-150", title: "Fix footer spacing" }] };
const hhmm = (iso: string) => iso.slice(11, 16);
const PRIYA = { id: "u-1", name: "Priya" } as never;
const freeze: EnvironmentFreeze = { projectId: "pj-1", environment: "production",
  reason: { text: "release week", author: "human" } as never, by: PRIYA, at: "2026-09-26T09:10:00Z" };

// mutant killed: "unserved work vanishes" on the desktop, and a prediction in the block
it("lists the settled criterion with its issue, and the unserved issue under Also shipped", () => {
  const { container } = render(<ReleaseNotes notes={notes} />);
  expect(screen.getByText("Uploads survive a dropped connection")).toBeTruthy();
  expect(screen.getByText("Also shipped")).toBeTruthy();
  expect(container.textContent).toContain("EP-150");
  expectNoPrediction(container);
});

it("copied Markdown carries each issue's Hub key and no drafted line", () => {
  const md = notesMarkdown(notes);
  expect(md).toContain("(issue:is-142)");
  expect(md).toContain("(issue:is-150)");
  expect(md.split("\n").every((l) => l === "" || /issue:|^## |^Release notes|settled by work/.test(l))).toBe(true);
});

it("previous_unknown lists nothing as settled and shows task 19's sentence", () => {
  const { container } = render(<ReleaseNotes notes={{ ...notes, state: "previous_unknown",
    message: "The cockpit cannot tell which release was serving before this one.", settled: [], alsoShipped: [] }} />);
  expect(container.textContent).toContain("cannot tell");
  expect(container.querySelector(".rn-crit")).toBeNull();
});

// mutant killed: "anyone unfreezes" (offered to a non-admin)
it("a non-admin sees who froze it and no Lift control", () => {
  const api = { freezes: async () => [], set: async () => freeze, clear: async () => {} };
  const { container } = render(<FreezeToggle api={api} hubProjectId="pj-1" environment="production"
    freeze={freeze} isAdmin={false} hhmm={hhmm} onChanged={() => {}} />);
  expect(screen.getByText(frozenLine(freeze, hhmm))).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Lift freeze" })).toBeNull();
  expectNoPrediction(container);
});

it("with no Hub freeze record, the panel says so and draws no switch", () => {
  render(<FreezeToggle api={null} hubProjectId="pj-1" environment="production" freeze={null}
    isAdmin hhmm={hhmm} onChanged={() => {}} />);
  expect(screen.getByText(/not available yet/)).toBeTruthy();
  expect(screen.queryByRole("button")).toBeNull();
});

// mutant killed: "first environment wins" (staging read as production)
it("each environment is named as itself, newest first", () => {
  const env = (environment: string, reachedAt: string) => ({ environment, reachedAt,
    deploymentId: environment, pinnedBrief: "brief:bf-1@6", commit: "c".repeat(40), servingNow: true });
  const { container } = render(<IssueShipped hhmm={hhmm} s={{ issueId: "is-142", state: "ok", message: "",
    environments: [env("production", "2026-09-26T14:02:00Z"), env("staging", "2026-09-26T09:40:00Z")] }} />);
  const rows = [...container.querySelectorAll("li")].map((li) => li.textContent);
  expect(rows[0]).toMatch(/^shipped to production, 14:02/);
  expect(rows[1]).toMatch(/^shipped to staging, 09:40/);
  render(<IssueShipped hhmm={hhmm} s={{ issueId: "is-150", state: "ok", message: "",
    environments: [env("staging", "2026-09-26T09:40:00Z")] }} />);
  expect(screen.queryAllByText(/production/).length).toBe(1);   // only the first render's row
});
```

The block is written as one for reading; it is split by component into the three files, each
repeating the fixtures it uses (three, two and one tests).

#### Steps

1. Sync-merge `feat/phase-7-deploy` and task 71's branch (rule H2). Re-read every ⚑ phase 7 file;
   where its names differ from this part's, phase 7's win and this part's code follows them.
2. Write `filter_test.go` and `notes_test.go`; run them; they fail. Write `filter/filter.go`, then
   `notes.go`; they pass.
3. Add the three `sessions` rows; extend ⚑ `sessions.go` with `ReadIssues`'s filter step and the
   `--name-only` form; they pass, and task 19's and task 71's tests pass unchanged.
4. Add the three `ship` rows; add `Freezes`, `FrozenError`, `checkFreeze` to ⚑ `ship.go` between
   step 4 and the reserve, and `Reason "frozen"` to `Readiness`; the P7 task 09 suite passes.
5. Add both routes to ⚑ `deploy_drill.go` with task 19's `deploymentId` check and range rule, and
   their two catalog lines; the route rows pass.
6. Write the three desktop tests; write the three components; mount `ReleaseNotes` in ⚑
   `ReleaseDrawer.tsx` above task 19's sessions, `FreezeToggle` per environment in ⚑
   `GovernancePanel.tsx` with `api={null}` until gap 1 closes, the frozen line in ⚑ `ShipBar.tsx`,
   `IssueShipped` under task 71's head line in ⚑ `IssueView.tsx`; add the Desk case.
7. Run the acceptance; commit.

#### Acceptance

```bash
cd control-plane && go test ./internal/deploy/notes/... ./internal/deploy/sessions/... \
  ./internal/deploy/ship/... ./internal/api/ -run 'Notes|Filter|DoubleStar|Frozen|Freeze|Shipped|Catalog|Readiness'
# expect: ok for each package; 3 notes tests, 4 filter rows, 3 ship rows, 2 route rows passed
cd control-plane && go vet ./internal/deploy/... && go list -deps ./internal/deploy/notes | grep -c -E "gateway|inference"
# expect: vet exit 0; the count is 0
cd desktop && npx vitest run src/deploy/ReleaseNotes.test.tsx src/deploy/FreezeToggle.test.tsx \
  src/deploy/IssueShipped.test.tsx && npx tsc --noEmit
# expect: 3 files, 6 tests passed; tsc exit 0
```

#### What this task does NOT claim

- Drafted release prose (task 139), requester updates (task 149), webhooks (task 156), the CLI
  (task 164).
- **That any environment can be frozen today.** With no Hub record (gap 1), `Freezes` is nil,
  nothing is frozen, and the panel says so. L-144 is closed only when 114, 116 and 124 land it.
- That the notes' *settled* is at the branch head: it is at the released commit, by phase 3's
  evaluator, and is only as good as the evidence recorded there.
- *Shipped* past task 08's 200-row history, or for commits this clone does not have
  (`commit_not_local`).
- That `.panacea/release.yaml` is the right home for the filter (**UNVERIFIED pending the owner's
  review**), or that a filter covers several repositories: each repository's own file filters its
  own releases (the Hub's `remotes`, `hub-contract.md:698-734`, **UNVERIFIED** line range here).

#### Gaps found while writing this part

1. **No freeze record in SW.20.** The scope names it; the proposed `EnvironmentFreeze`, its three
   routes and two events are above, for tasks 114 (types, routes), 116 (the table), 115 (the fake)
   and 124/125 (the relay rows and `FreezeApi`). Until then this task ships the check wired to nil.
2. **The notes route follows phase 7, not the scope.** `GET {P}/notes?deploymentId=…` replaces
   the scope's `…/releases/{releaseId}/notes` (decision 1).
3. **The scope's phase 7 line numbers are stale** by 7-8 lines (verified list at the top).
4. **`IssueShipped.tsx`, `types.ts` and `ShipBar.tsx` are not in the scope's file list**, and
   `GET {P}/shipped` is not named by it; L-148's list needs all four.
5. **No part for tasks 71 or 92** on this tree: `ReleaseIssue` and `copyMarkdown` are their
   scopes' names here.

