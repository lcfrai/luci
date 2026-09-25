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
```
