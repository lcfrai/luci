### Task 115: the fake's W4 rules and conformance rows

**Slice:** W4. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 114 (SW.20: every W4 type, `WorkRoadmap`, the stubs, the event kinds). **Seams:**
**SW.20**, frozen here the way task 73 froze SW.18: after this task only added methods, fields or
routes. Consumed by 117-124 (the server passes these rows in task 124) and 125-141 (the desktop's
contract tests run on this fake).

Task 114 made `*Fake` satisfy `WorkRoadmap` with 501s. Every W4 handler and every desktop W4
surface is tested against the fake first, so the fake must refuse what the Hub refuses, for the
same reason, with the same code. This task writes those rules once, in memory, and writes each as a
conformance row, so task 124 can run the identical rows against the real Hub and a rule the two
disagree on fails in one place.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`** (under
`control-plane/internal/hub/hubclient/`): `fake.go:56-70` (`fakeWorld`: `now`, `users`, `teams`,
`projects`, `briefs`), `:139` (`enter`), `:172` (`stamp`), `:175` (`next`), `:184-193`
(`problem`), `:195-197` (`badRequest`), `:199-202` (`forbidden`), `:204-223` (`rank`, `need`: an
unknown role ranks 0); `fake_briefs.go:18-29` (`fakeBrief`: `projectID`, `branchRef`,
`revisions`), `:33-43` (`fakeRevision.erasedAt`), `:49-56` (`briefOf`: unreadable is 404),
`:466-498` (`Resolve`: a key in an unreadable project is `missing`). `hubapi/keys.go:69`
(`ParseBriefKey`), `:96` (`ParseKey`).

**Read in this branch's parts, UNVERIFIED until merge:** task 04's `fake_work.go` (`workWorlds`
and `wk()`, `task-04.md:306-315`; `workEvent`, `:319-325`; `issueOf`, `:333-340`; the
`issue.state` data `{from, to, cause}`, `:1032-1034`); task 05's `conformance/work.go` head
(`WorkEnv`, `WorkRow`, `RunWork`, `wRefused`, `wKinds`, `wHead`, `task-05.md:87-212`); task 114's
`work_roadmap.go`, event kinds and stub file (`task-114.md`); task 73's feature state in
`fake_work_plan.go` (scope `tasks-61-80.md:434-458`; no part yet); H05's feed (`appendEvent`,
and a read of the fake's events, named `feedEvents` here). Task 119's part fixes the pin and cite
refusals (`task-119.md:34-49`). Step 1 takes the merged names.

#### Decisions the scope left open, made here

1. **W4's state lives beside the world, not in it.** `roadmapWorlds sync.Map` maps a
   `*fakeWorld` to a `*fakeRoadmap`, task 04's decision (i) again, so neither `fakeWorld` nor task
   04's `fakeWork` nor task 73's plan state gains a field.
2. **A grant is a role the ranking does not know.** `roleGranted hubapi.Role = "granted"` ranks
   0 in `fake.go:205-214`, so every existing `need(role, …)` refuses a granted-only caller with 403
   `forbidden`, and no W1-W3 write needs a new check to stay closed. Two call sites change (⚑):
   task 04's `issueOf` falls back to a live grant on the issue's project and returns
   `roleGranted`; task 05's `CreateComment` accepts `roleGranted` before its `need`. Every other
   Work read that takes a project (task 73's feature and milestone reads) switches from H's
   `projectOf` to this file's `workProjectOf`. **H's `projectOf` is not touched**, so briefs,
   `/v1/refs` and designs stay team-only (task 117's gap: the brief's cross-team read is a
   contract proposal, not a fake rule).
3. **The immutability row is a shape row.** The Hub answers a second write to an `nt-` with 409
   `update_immutable` on `PATCH /v1/updates/{id}` (task 119, decision 6), a route with no client
   method. The fake has no route, so its row asserts what the client can: `WorkRoadmap` has no
   method that writes an existing update (a reflection check on method names), and an update reads
   byte-equal across later writes. The 409 itself is task 119's HTTP test and task 124's row.
4. **Pin and cite refusals are task 119's.** A feature pin to another brief, or to a revision past
   the head, is 422 with `field: "pinnedBrief"`; a cite that `Resolve` would call `missing` is 422
   with `field: "cites"` and `index`. The code is 119's `bad_request` (`task-119.md:35-36`, `:46`),
   copied, not chosen here (a gap: `bad_request` is 400 elsewhere).
5. **"Teams its owner's team may name"** is read as: the owning team first, and each other team
   linked to it by a live grant in either direction (it granted one of the owning team's projects,
   or holds a grant on one). Anything else is 422 `bad_request`, `field: "teamIds[i]"`.
6. **Insights read the feed, not the heads.** `QueryInsights` walks the fake's events of kind
   `issue.state` and `issue.created` in the window, never a head field, so a percentile is only
   ever a measured move. Nearest-rank percentiles; `nil` when no point was measured.

#### Files

| Path (under `control-plane/internal/hub/hubclient/`) | Change |
|---|---|
| `fake_work_roadmap.go` | new: W4's state, the grant reach, and all twenty-nine `WorkRoadmap` methods |
| `fake_work_roadmap_stub.go` | **deleted** (task 114's), in the same commit |
| `fake_work.go` ⚑ | task 04's `issueOf`: the grant fallback (decision 2), four lines |
| `fake_work_social.go` ⚑ | task 05's `CreateComment`: `roleGranted` may comment, two lines |
| `fake_work_plan.go` ⚑ | task 73's feature and milestone reads call `workProjectOf` (UNVERIFIED names) |
| `fake_work_roadmap_test.go` | new: the fake-only tests (the helpers, the percentile rule) |
| `conformance/work_roadmap.go` | new: `WorkRowsW4`, fourteen rows |
| `conformance/work_roadmap_fake_test.go` | new: `RunWork(t, fakeEnv, WorkRowsW4...)` |

#### `fake_work_roadmap.go`, complete: state and the grant reach

```go
package hubclient

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// roleGranted is a caller whose only reach into a project is a grant. It
// ranks 0 (fake.go:204-214), so every need(role, …) refuses it (decision 2).
const roleGranted hubapi.Role = "granted"

type fakeRoadmap struct {
	grants      map[[2]string]*hubapi.ProjectGrant   // (projectId, teamId)
	initiatives map[string]*hubapi.Initiative        // by in- id
	spans       map[string][]string                  // initiative id → team ids, owner first
	featureIn   map[string]string                    // feature id → initiative id
	updates     map[string][]*hubapi.StatusUpdate    // "feature:ft-…" → newest last
	milestones  map[string]*hubapi.Milestone         // by ml- id
	deps        map[[2]string]bool                   // (from, blocks)
	schedules   map[string]hubapi.TemplateSchedule   // by template id
	reminders   map[string]hubapi.ReminderPolicy     // by subject key
	rotations   map[string]hubapi.TriageRotation     // by team id
	cursors     map[[2]string]int64                  // (user id, scope string) → seq
}

var roadmapWorlds sync.Map // *fakeWorld → *fakeRoadmap

// rm is w's W4 state, made on first use. Call it with w.mu held.
func (w *fakeWorld) rm() *fakeRoadmap {
	v, _ := roadmapWorlds.LoadOrStore(w, &fakeRoadmap{
		grants: map[[2]string]*hubapi.ProjectGrant{}, initiatives: map[string]*hubapi.Initiative{},
		spans: map[string][]string{}, featureIn: map[string]string{},
		updates: map[string][]*hubapi.StatusUpdate{}, milestones: map[string]*hubapi.Milestone{},
		deps: map[[2]string]bool{}, schedules: map[string]hubapi.TemplateSchedule{},
		reminders: map[string]hubapi.ReminderPolicy{}, rotations: map[string]hubapi.TriageRotation{},
		cursors: map[[2]string]int64{}})
	return v.(*fakeRoadmap)
}

// grantedTo reports whether me is a member of a team holding a live grant on
// projectID. A revoked grant is deleted, so revocation is immediate.
func (w *fakeWorld) grantedTo(projectID, me string) bool {
	for k := range w.rm().grants {
		if k[0] == projectID && w.teams[k[1]] != nil && w.teams[k[1]].members[me] != nil {
			return true
		}
	}
	return false
}

// workProjectOf is projectOf for Work reads only: the caller's role in the
// owning team, else roleGranted through a grant, else 404 as projectOf is.
func (w *fakeWorld) workProjectOf(projectID, me string) (*fakeProject, hubapi.Role, error) {
	if p, role, err := w.projectOf(projectID, me); err == nil {
		return p, role, nil
	}
	if p := w.projects[projectID]; p != nil && w.grantedTo(projectID, me) {
		return p, roleGranted, nil
	}
	return w.projectOf(projectID, me) // the same 404, byte for byte
}

// featureOf is a feature the caller may read, through its team or a grant.
// task 73's state is read through one adapter, w.planFeature (UNVERIFIED
// name: Step 1 binds it to fake_work_plan.go's map of features).
func (w *fakeWorld) featureOf(featureID, me string) (hubapi.Feature, hubapi.Role, error) {
	if f, ok := w.planFeature(featureID); ok {
		if _, role, err := w.workProjectOf(f.ProjectID, me); err == nil {
			return f, role, nil
		}
	}
	return hubapi.Feature{}, "", problem(404, "feature_not_found", // task 76's code
		"There is no such feature that you can see.", "featureId", featureID)
}

// rmEvent is the one event every W4 write appends (SW.5; task 114's kinds).
func (w *fakeWorld) rmEvent(me, kind, teamID, projectID, subject string, data map[string]any) {
	w.workEvent(me, kind, teamID, projectID, subject, data) // task 04's helper
}

func rmProblem(code, message string, kv ...string) error {
	return problem(hubapi.WorkRoadmapCodeStatus[code], code, message, kv...)
}
```

#### `fake_work_roadmap.go`, continued: grants and an initiative's span (task 117's rules)

```go
func (f *Fake) GrantProject(_ context.Context, projectID, teamID string) (hubapi.ProjectGrant, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("GrantProject", projectID, teamID)
	if err != nil {
		return hubapi.ProjectGrant{}, err
	}
	p, role, err := w.projectOf(projectID, me) // a granted caller cannot grant onward
	if err != nil {
		return hubapi.ProjectGrant{}, err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return hubapi.ProjectGrant{}, err
	}
	if w.teams[teamID] == nil || teamID == p.teamID {
		return hubapi.ProjectGrant{}, badRequest("teamId", "Grant a different team of this org.")
	}
	k := [2]string{projectID, teamID}
	if w.rm().grants[k] != nil {
		return hubapi.ProjectGrant{}, rmProblem(hubapi.CodeGrantExists,
			"That team already has this project.", "projectId", projectID, "teamId", teamID)
	}
	g := &hubapi.ProjectGrant{ProjectID: projectID, TeamID: teamID, Scope: hubapi.GrantReadComment,
		GrantedBy: w.userRef(me), At: w.stamp()}
	w.rm().grants[k] = g
	w.rmEvent(me, hubapi.EventWorkGrantAdded, p.teamID, projectID, projectID,
		map[string]any{"projectId": projectID, "teamId": teamID})
	return *g, nil
}

func (f *Fake) RevokeProjectGrant(_ context.Context, projectID, teamID string) error {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RevokeProjectGrant", projectID, teamID)
	if err != nil {
		return err
	}
	p, role, err := w.projectOf(projectID, me)
	if err != nil {
		return err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return err
	}
	k := [2]string{projectID, teamID}
	if w.rm().grants[k] == nil {
		return rmProblem(hubapi.CodeGrantNotFound, "That team has no grant on this project.",
			"projectId", projectID, "teamId", teamID)
	}
	delete(w.rm().grants, k) // reads and the feed stop at once
	w.rmEvent(me, hubapi.EventWorkGrantRevoked, p.teamID, projectID, projectID,
		map[string]any{"projectId": projectID, "teamId": teamID})
	return nil
}

func (f *Fake) ProjectGrants(_ context.Context, projectID string) ([]hubapi.ProjectGrant, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ProjectGrants", projectID)
	if err != nil {
		return nil, err
	}
	if _, _, err := w.projectOf(projectID, me); err != nil { // the owning team only
		return nil, err
	}
	out := []hubapi.ProjectGrant{}
	for k, g := range w.rm().grants {
		if k[0] == projectID {
			out = append(out, *g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeamID < out[j].TeamID })
	return out, nil
}

// linked: a live grant in either direction between the two teams (decision 5).
func (w *fakeWorld) linked(owner, other string) bool {
	for k := range w.rm().grants {
		pt := w.projects[k[0]].teamID
		if (pt == owner && k[1] == other) || (pt == other && k[1] == owner) {
			return true
		}
	}
	return false
}

func (f *Fake) SetInitiativeTeams(_ context.Context, initiativeID string, teamIDs []string) (hubapi.InitiativeTeams, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.InitiativeTeams
	me, err := f.enter("SetInitiativeTeams", initiativeID, teamIDs)
	if err != nil {
		return zero, err
	}
	in, role, err := w.initiativeOf(initiativeID, me)
	if err != nil {
		return zero, err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return zero, err
	}
	if len(teamIDs) < 1 || len(teamIDs) > 10 || teamIDs[0] != in.TeamID {
		return zero, badRequest("teamIds", "The owning team comes first; at most ten teams.")
	}
	for i, t := range teamIDs[1:] {
		if slices.Index(teamIDs, t) != i+1 || !w.linked(in.TeamID, t) {
			return zero, problem(422, hubapi.CodeBadRequest, "Name only teams this team shares a project with.",
				"field", "teamIds["+strconv.Itoa(i+1)+"]")
		}
	}
	w.rm().spans[initiativeID] = slices.Clone(teamIDs)
	w.rmEvent(me, hubapi.EventWorkInitiativeTeamsSet, in.TeamID, "", initiativeID,
		map[string]any{"initiativeId": initiativeID, "teams": int64(len(teamIDs))})
	return hubapi.InitiativeTeams{InitiativeID: initiativeID, TeamIDs: slices.Clone(teamIDs)}, nil
}
```

The two ⚑ call sites, complete:

```go
// fake_work.go, task 04's issueOf, after its member check and before the 404:
		if w.grantedTo(is.head.ProjectID, me) {
			return is, roleGranted, nil
		}

// fake_work_social.go, task 05's CreateComment, replacing its need(role, RoleMember):
	if role != roleGranted {
		if err := need(role, hubapi.RoleMember); err != nil {
			return hubapi.Comment{}, err
		}
	}
```

#### `fake_work_roadmap.go`, continued: initiatives and the view (task 118's rules)

```go
func (w *fakeWorld) initiativeOf(id, me string) (*hubapi.Initiative, hubapi.Role, error) {
	if in := w.rm().initiatives[id]; in != nil {
		if m := w.teams[in.TeamID].members[me]; m != nil {
			return in, m.role, nil
		}
		for _, t := range w.rm().spans[id][1:] { // a spanned team reads, never writes
			if w.teams[t] != nil && w.teams[t].members[me] != nil {
				return in, roleGranted, nil
			}
		}
	}
	return nil, "", rmProblem(hubapi.CodeInitiativeNotFound, "There is no such initiative that you can see.",
		"initiativeId", id)
}

func (f *Fake) CreateInitiative(_ context.Context, teamID string, req hubapi.CreateInitiativeRequest) (hubapi.Initiative, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateInitiative", teamID, req)
	if err != nil {
		return hubapi.Initiative{}, err
	}
	t := w.teams[teamID]
	if t == nil || t.members[me] == nil {
		return hubapi.Initiative{}, problem(404, hubapi.CodeTeamNotFound, "There is no such team that you can see.")
	}
	if err := need(t.members[me].role, hubapi.RoleMember); err != nil {
		return hubapi.Initiative{}, err
	}
	if err := req.Validate(); err != nil {
		return hubapi.Initiative{}, contentError("", err) // task 04's FieldError → 400
	}
	if t.members[req.OwnerID] == nil {
		return hubapi.Initiative{}, badRequest("ownerId", "The owner must be a member of the team.")
	}
	state := hubapi.InitiativePlanned
	if req.State != nil {
		state = *req.State
	}
	now := w.stamp()
	in := &hubapi.Initiative{ID: ids.Wide(hubapi.PrefixInitiative), TeamID: teamID, Name: req.Name,
		Owner: w.userRef(req.OwnerID), TargetDate: req.TargetDate, State: state,
		CreatedAt: now, CreatedBy: w.userRef(me), UpdatedAt: now}
	w.rm().initiatives[in.ID] = in
	w.rm().spans[in.ID] = []string{teamID}
	w.rmEvent(me, hubapi.EventWorkInitiativeCreated, teamID, "", in.ID,
		map[string]any{"initiativeId": in.ID, "state": string(state)})
	return *in, nil
}

// Initiative is task 118's view: each readable feature's own latest health
// word and date, never a score; unreadable features are absent, not named.
func (f *Fake) Initiative(_ context.Context, id string) (hubapi.InitiativeView, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Initiative", id)
	if err != nil {
		return hubapi.InitiativeView{}, err
	}
	in, _, err := w.initiativeOf(id, me)
	if err != nil {
		return hubapi.InitiativeView{}, err
	}
	v := hubapi.InitiativeView{Initiative: *in, Teams: slices.Clone(w.rm().spans[id]),
		Features: []hubapi.InitiativeFeature{}}
	for fid, iid := range w.rm().featureIn {
		ft, _, err := w.featureOf(fid, me)
		if iid != id || err != nil {
			continue // the name of a feature the caller cannot read never leaves the fake
		}
		row := hubapi.InitiativeFeature{FeatureID: fid, Name: ft.Name, Lead: ft.Lead,
			Criteria: w.criterionWords(ft)}
		if us := w.rm().updates[hubapi.UpdateSubjectFeature+":"+fid]; len(us) > 0 {
			last := us[len(us)-1]
			row.LatestUpdate = &hubapi.UpdateStamp{Health: last.Health, At: last.At}
		}
		v.Features = append(v.Features, row)
	}
	sort.Slice(v.Features, func(i, j int) bool { return v.Features[i].FeatureID < v.Features[j].FeatureID })
	return v, nil
}

// criterionWords is each criterion of the feature's branch brief at its head
// with task 10's word. Two adapters, both UNVERIFIED names bound at Step 1:
// w.briefCriterionIDs (the head revision's criterion ids, from the envelope)
// and w.criterionWord (task 10's thread-state word; "open" with no evidence).
func (w *fakeWorld) criterionWords(ft hubapi.Feature) []hubapi.CriterionWord {
	out := []hubapi.CriterionWord{}
	for _, c := range w.briefCriterionIDs(ft.BriefID) {
		out = append(out, hubapi.CriterionWord{CriterionID: c, Word: w.criterionWord(ft.BriefID, c)})
	}
	return out
}
```

`Initiatives` (the team's, newest first, `pageOf`), `PatchInitiative` (member of the owning team;
`ownerId` must be a member; `targetDate` `null` clears, task 03's `json.RawMessage` rule; one
`work.initiative_updated` event naming the changed fields) and `SetFeatureInitiative` follow the
same shape and are not repeated. `SetFeatureInitiative`'s two rules: the caller is a member of the
**feature's** team (`need(role, RoleMember)`, so `roleGranted` is refused), and the initiative's
span (`spans[id]`) includes that team, else 422 `bad_request`, `field: "initiativeId"`; `nil`
deletes `featureIn[featureID]`.

#### `fake_work_roadmap.go`, continued: status updates (task 119's rules)

```go
func subjectKey(s hubapi.UpdateSubject) string { return s.Kind + ":" + s.ID }

func (f *Fake) PostUpdate(_ context.Context, s hubapi.UpdateSubject, req hubapi.PostUpdateRequest) (hubapi.StatusUpdate, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.StatusUpdate
	me, err := f.enter("PostUpdate", s, req)
	if err != nil {
		return zero, err
	}
	teamID, projectID, pin, err := w.updateTarget(s, me, req)
	if err != nil {
		return zero, err
	}
	if err := req.ValidateFor(s); err != nil {
		return zero, contentError("", err)
	}
	for i, k := range req.Cites { // Resolve's rule: a key in an unreadable project is missing
		if !w.resolvable(k, me) {
			return zero, problem(422, hubapi.CodeBadRequest, "A cited key does not resolve for you.",
				"field", "cites", "index", strconv.Itoa(i))
		}
	}
	u := &hubapi.StatusUpdate{ID: ids.Wide("nt"), Kind: "update", Subject: s, Health: req.Health,
		Body: req.Body, PinnedBrief: pin, FeaturePins: slices.Clone(req.FeaturePins),
		Cites: slices.Clone(req.Cites), Author: w.userRef(me), At: w.stamp()}
	if u.FeaturePins == nil {
		u.FeaturePins = []hubapi.FeaturePin{}
	}
	w.rm().updates[subjectKey(s)] = append(w.rm().updates[subjectKey(s)], u)
	w.rmEvent(me, hubapi.EventWorkUpdatePosted, teamID, projectID, u.ID, map[string]any{
		"subjectKind": s.Kind, "subjectId": s.ID, "health": string(u.Health),
		"pinnedBrief": map[bool]string{true: pin, false: ""}[s.Kind == hubapi.UpdateSubjectFeature],
		"pins": int64(len(u.FeaturePins)), "cites": int64(len(u.Cites))})
	return *u, nil
}
```
