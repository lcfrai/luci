### Task 114: `hubapi` and `hubclient.WorkRoadmap`: the W4 wire types and methods

**Slice:** W4. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 97 (W2's proof), and through it 72-73 (SW.18: `Feature`, `Template`, `FeatureState`)
and 99 (W3's wire); a sync-merge of `feat/hub` into `feat/work-layer` first (rule H1).
**Seams:** defines **SW.20**, the whole W4 wire: every W4 record, request, view and code, and the
`WorkRoadmap` method set. Consumed by tasks 115-141. After this task SW.20 grows only by added
methods, fields or routes; anything else reopens this task and is announced (skeleton, *Start lines*).

W4 has fifteen record kinds and eight route tasks (117-123) on the Hub, and eleven desktop tasks
(125-141) reading them. If each route task declared its own wire, the fake (115), the migration (116)
and the HTTP client (124) would each chase eight moving shapes. So this task fixes every W4 type and
method once, with goldens, and makes the OD-27 rule mechanical: no W4 type may carry a field that
predicts (a percent, a progress, an ETA, a forecast, a remaining).

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`** (under `control-plane/internal/hub/`):
`hubapi/codes.go:1-67` (the codes as constants, each with its status in a trailing comment; no
status map exists), `hubapi/canonical.go:282` (`FieldError`), `hubapi/keys.go:32` (`ValidID`),
`hubapi/envelope.go:394` (`FormatTime`), `hubapi/types.go:95` (`User`), `:196-202` (`Page`,
`PageRequest`), `:536-537` (`Event`, `Seq int64`); `hubclient/hubclient.go:52` (`Briefs`, the
method style), `:101` (`type Hub interface`), `:113` (`Error`). The contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md`: `:253` (`brief_exists`), `:309`
(`hub.org_id`), `:333` (*"There are no per-project grants in v1"*), `:341-359` (the §5.3 matrix),
`:356` (`GET /v1/refs`, *"resolves keys in readable projects"*).

**Read in this branch's parts, not yet code, so UNVERIFIED until merge:** task 02's
`hubapi/workids.go` (`PrefixInitiative = "in"`, `PrefixMilestone = "ml"`, `PrefixTemplate = "tp"`,
`task-02.md:146-157`); task 02's `AuthoredText` (`task-02.md:226-229`); task 04's
`hubclient/work.go` (`type Work interface { WorkIssues }`, `task-04.md:153-233`, whose comment
already names `WorkRoadmap (task 114)`) and `workNotInFakeYet` (`task-04.md:1101`); task 72's
`Feature`, `FeatureState` and `Template` (scope `tasks-61-80.md:398-406`, part not yet written);
Phase T's `nt-` prefix and `PrefixNote` (TM-05, TM-46; Phase T plan, **UNVERIFIED** name). Step 1
takes the merged names; a rename is a find-and-replace in this part, never a new type.

#### Decisions the scope left open, made here

1. **Initiative states reuse the feature's words**, `planned`·`active`·`paused`·`done`·`cancelled`.
   04 §2.5 names an initiative's `state` without values. One vocabulary for two planning records
   means one filter, one chip and one colour rule on the desktop. The type is a new name,
   `InitiativeState`, with the same five constants, so a later split is additive.
2. **`Work` embeds `WorkRoadmap` in this task**, as the scope says, which differs from W2's split
   (72 declared `WorkPlan` standalone; 73 embedded it). To keep `var _ Hub = (*Fake)(nil)`
   compiling between 114 and 115, this task adds `hubclient/fake_work_roadmap_stub.go`: every
   `WorkRoadmap` method on `*Fake` returns `workNotInFakeYet`, and task 115 deletes the file in the
   commit that adds the real ones. The HTTP client gets the same stub (`client_work_roadmap_stub.go`,
   501 `not_in_client_yet`), which task 124 deletes.
3. **The codes carry a status map.** `codes.go` has no map today; the scope's test *"the new codes
   map to their statuses"* needs one. `hubapi.WorkRoadmapCodeStatus` is a `map[string]int` in
   `work_roadmap.go`, read by the fake (115) and the server (117-123) when they write a problem, so
   the status a code carries is written once.
4. **Three not-found codes are added beside the scope's four.** Tasks 117-120 answer 404 for an
   unknown or unreadable initiative, milestone or grant, and the Hub's rule is one `*_not_found`
   per route family (`codes.go:24-33`). `initiative_not_found`, `milestone_not_found` and
   `grant_not_found` are appended; `update_not_found` is not, because an update is a TM-46 note and
   its 404 is TM-46's (**UNVERIFIED** code name, `note_not_found` in the Phase T plan).
5. **Percentiles are pointers.** `InsightsResult.P50Ms` and its siblings are `*int64`, `null` when no
   point was measured. A zero would be a number nobody measured (Q1, OD-27).
6. **Derived fields live on views, never on records.** `Milestone` has no *reached*;
   `FeatureDependency` has no `orderViolated`. Both are computed at read into `MilestoneView` and
   `FeatureDependencyView`. `InitiativeView` (task 118's read) is declared here too, so SW.20 is the
   whole wire and task 118 adds no type.
7. **`MarkSeen` is in the method set.** Task 123's scope adds `POST /v1/me/read-cursors/mark-seen`,
   which 114's scope does not list; it is declared now so SW.20 does not grow after it freezes.

#### Files

| Path (under `control-plane/internal/hub/`) | Change |
|---|---|
| `hubapi/work_roadmap.go` | new: every W4 record, view, request, enum, code and `Validate` |
| `hubapi/work_roadmap_test.go` | new: goldens, validation, the no-prediction reflection test, codes |
| `hubapi/testdata/work-roadmap/*.json` | new: one golden per record and view (seventeen files) |
| `hubapi/codes.go` ⚑ | appended: the seven W4 codes, in the table's style |
| `hubclient/work.go` ⚑ | `Work` embeds `WorkRoadmap`; the `WorkRoadmap` interface, complete |
| `hubclient/fake_work_roadmap_stub.go` | new: `*Fake` satisfies `WorkRoadmap` with 501s (task 115 deletes it) |
| `hubclient/client_work_roadmap_stub.go` | new: the HTTP client's 501s (task 124 deletes it) |
| `hubclient/work_roadmap_test.go` | new: the compile-time embed assertions |

#### `hubapi/work_roadmap.go`, complete: initiatives and status updates

```go
package hubapi

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// InitiativeState reuses the feature's five words (decision 1; 04 §2.5).
type InitiativeState string

const (
	InitiativePlanned   InitiativeState = "planned"
	InitiativeActive    InitiativeState = "active"
	InitiativePaused    InitiativeState = "paused"
	InitiativeDone      InitiativeState = "done"
	InitiativeCancelled InitiativeState = "cancelled"
)

// Initiative groups features toward one outcome. Per team in v1 (05 §3.2).
type Initiative struct {
	ID         string          `json:"id"` // in-…
	TeamID     string          `json:"teamId"`
	Name       string          `json:"name"` // 1-120 characters
	Owner      User            `json:"owner"`
	TargetDate *string         `json:"targetDate"` // YYYY-MM-DD or null
	State      InitiativeState `json:"state"`
	CreatedAt  string          `json:"createdAt"`
	CreatedBy  User            `json:"createdBy"`
	UpdatedAt  string          `json:"updatedAt"`
}

// InitiativeTeams is W4's span (Q12): the owning team first, then each other
// team whose readable features the initiative may list.
type InitiativeTeams struct {
	InitiativeID string   `json:"initiativeId"`
	TeamIDs      []string `json:"teamIds"` // never null; TeamIDs[0] is the owning team
}

// Health is a person's word, never a computed score (Q1, OD-27).
type Health string

const (
	HealthOnTrack  Health = "on_track"
	HealthAtRisk   Health = "at_risk"
	HealthOffTrack Health = "off_track"
)

// SubjectRef names what a status update or a reminder is about.
type SubjectRef struct {
	Kind string `json:"kind"` // SubjectFeature | SubjectInitiative
	ID   string `json:"id"`   // ft-… | in-…
}

const (
	SubjectFeature    = "feature"
	SubjectInitiative = "initiative"
)

// StatusUpdate is a TM-46 note (nt-) with kind "update": immutable once
// posted; erase empties Body.Text and keeps every other field (task 116).
type StatusUpdate struct {
	ID          string       `json:"id"`   // nt-…
	Kind        string       `json:"kind"` // always "update"
	Subject     SubjectRef   `json:"subject"`
	Health      Health       `json:"health"`
	Body        AuthoredText `json:"body"`        // author "H" only (task 119)
	PinnedBrief string       `json:"pinnedBrief"` // brief:bf-…@r, required
	Cites       []string     `json:"cites"`       // Hub keys; never null
	Author      User         `json:"author"`
	At          string       `json:"at"`
	ErasedAt    *string      `json:"erasedAt"`
}
```

#### `work_roadmap.go`, continued: milestones, dependencies, templates, the janitor's policies

```go
// Milestone is a named subset of the feature's branch-brief criteria, with a
// date (04 §2.5). "Reached" is never stored: MilestoneView derives it.
type Milestone struct {
	ID           string   `json:"id"` // ml-…
	FeatureID    string   `json:"featureId"`
	Name         string   `json:"name"` // 1-120 characters
	Date         *string  `json:"date"` // YYYY-MM-DD or null
	CriterionIDs []string `json:"criterionIds"` // never null; each on the feature's brief
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// CriterionWord is one criterion with task 10's thread-state word at the
// branch head: open, waiting, proven, proven_self_attested, and so on.
type CriterionWord struct {
	CriterionID string `json:"criterionId"`
	Word        string `json:"word"`
}

// MilestoneView names which criteria are settled. It prints no count and no
// bar (OD-27; L-72 PARTIAL on purpose); Reached is derived at read.
type MilestoneView struct {
	Milestone Milestone       `json:"milestone"`
	Criteria  []CriterionWord `json:"criteria"` // in Milestone.CriterionIDs order
	Reached   bool            `json:"reached"`  // every listed criterion settled, at read
}

// FeatureDependency is "From blocks Blocks": both ft- in one team's reach.
type FeatureDependency struct {
	From   string `json:"from"`   // ft-… the blocker
	Blocks string `json:"blocks"` // ft-… the blocked
}

// FeatureDependencyView adds the derived signal task 129 colours: the blocked
// feature's target date is before its blocker's. Never stored.
type FeatureDependencyView struct {
	FeatureDependency
	OrderViolated bool `json:"orderViolated"`
}

// BriefTemplate sits beside task 78's issue Template in the tp- space.
type BriefTemplate struct {
	ID               string         `json:"id"` // tp-…
	TeamID           *string        `json:"teamId"`
	Kind             string         `json:"kind"` // always "brief"
	Name             string         `json:"name"`
	Criteria         []AuthoredText `json:"criteria"`         // author "H"; 1-50 items
	IssueTemplateIDs []string       `json:"issueTemplateIds"` // tp-… of kind "issue"; never null
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

// Every is a recurrence: N units apart.
type Every struct {
	N    int    `json:"n"`    // 1-365
	Unit string `json:"unit"` // "day" | "week" | "month"
}

// TemplateSchedule makes an issue template recurring (task 121).
type TemplateSchedule struct {
	TemplateID string `json:"templateId"`
	Every      Every  `json:"every"`
	At         string `json:"at"` // HH:MM, 24-hour, in TZ
	TZ         string `json:"tz"` // an IANA zone, e.g. "Australia/Sydney"
}

// ReminderPolicy asks the lead or owner for an update every EveryDays days.
type ReminderPolicy struct {
	Subject   SubjectRef `json:"subject"`
	EveryDays int        `json:"everyDays"` // 1-90
}

// TriageRotation is a stored policy. The read adds who is on duty and the
// next handover date, dates the policy fixes, not a forecast (task 121).
type TriageRotation struct {
	TeamID     string   `json:"teamId"`
	Members    []string `json:"members"` // u-…, 1-50, no repeats
	PeriodDays int      `json:"periodDays"` // 1-28
	StartsOn   string   `json:"startsOn"`   // YYYY-MM-DD
}

// TriageRotationView is the rotation and its two policy-fixed facts.
type TriageRotationView struct {
	Rotation   TriageRotation `json:"rotation"`
	OnDuty     User           `json:"onDuty"`
	HandoverOn string         `json:"handoverOn"` // YYYY-MM-DD, from StartsOn and PeriodDays
}
```
