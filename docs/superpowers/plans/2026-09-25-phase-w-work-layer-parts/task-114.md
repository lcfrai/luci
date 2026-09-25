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
   every method `ErrNotConfigured`, task 04's client idiom, `task-04.md:138-147`), which task 124
   deletes.
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
	"encoding/json"
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

#### `work_roadmap.go`, continued: insights, read cursors, grants, the initiative view

```go
// Insight measures, each read only from recorded state moves (task 122).
const (
	MeasureCount       = "count"
	MeasureCycleTime   = "cycle_time"   // first move into started → the standing done
	MeasureLeadTime    = "lead_time"    // creation → the standing done
	MeasureTriageTime  = "triage_time"  // creation → the first move out of triage
)

// InsightsQuery is a window over one team, optionally one project.
type InsightsQuery struct {
	TeamID    string  `json:"teamId"`
	ProjectID *string `json:"projectId"`
	From      string  `json:"from"` // RFC 3339, the Hub's clock
	To        string  `json:"to"`
	Measure   string  `json:"measure"`
	GroupBy   *string `json:"groupBy"` // "state" | "assignee" | "label" | "feature" | "cycle" | null
}

// InsightGroup is one group's measured count.
type InsightGroup struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// InsightPoint is one issue's measured duration; the scatter opens it.
type InsightPoint struct {
	Issue string `json:"issue"` // is-…
	Ms    int64  `json:"ms"`
}

// InsightsResult is measured only (Q1). The percentiles are null when no
// point was measured (decision 5); they are nearest-rank over Points.
type InsightsResult struct {
	Groups []InsightGroup `json:"groups"` // never null
	Points []InsightPoint `json:"points"` // never null; empty for "count"
	P50Ms  *int64         `json:"p50Ms"`
	P75Ms  *int64         `json:"p75Ms"`
	P90Ms  *int64         `json:"p90Ms"`
}

// ScopeRef is what a read cursor watches (task 123).
type ScopeRef struct {
	Kind string `json:"kind"` // "project" | "feature" | "issue"
	ID   string `json:"id"`
}

// ReadCursor is one person's last-seen feed seq on one scope. It only
// advances: a lower Seq is ignored and the stored cursor returned.
type ReadCursor struct {
	Scope ScopeRef `json:"scope"`
	Seq   int64    `json:"seq"` // an Event.Seq (types.go:537); 0 is "never looked"
}

// MarkSeenRequest advances several cursors to one seq (task 123).
type MarkSeenRequest struct {
	Scopes []ScopeRef `json:"scopes"` // 1-200
	Seq    int64      `json:"seq"`
}

// ProjectGrant lets a second team read a project's Work objects and comment
// on them (Q12). The scope has one value in v1.
type ProjectGrant struct {
	ProjectID string `json:"projectId"`
	TeamID    string `json:"teamId"`
	Scope     string `json:"scope"` // GrantReadComment
	GrantedBy User   `json:"grantedBy"`
	At        string `json:"at"`
}

const GrantReadComment = "read_comment"

// InitiativeFeature is one feature's row in the initiative's read: its own
// latest health word and date, never a score (L-84), and its criteria words.
type InitiativeFeature struct {
	FeatureID    string          `json:"featureId"`
	Name         string          `json:"name"`
	Lead         User            `json:"lead"`
	LatestUpdate *UpdateStamp    `json:"latestUpdate"` // null when never updated
	Criteria     []CriterionWord `json:"criteria"`
}

// UpdateStamp is the health and date of a subject's latest status update.
type UpdateStamp struct {
	Health Health `json:"health"`
	At     string `json:"at"`
}

// InitiativeView is task 118's read. It has no aggregate health (L-84).
type InitiativeView struct {
	Initiative Initiative          `json:"initiative"`
	Teams      []string            `json:"teams"`    // InitiativeTeams.TeamIDs
	Features   []InitiativeFeature `json:"features"` // only those the caller may read
}
```

#### `work_roadmap.go`, continued: request bodies

Nullable fields that a PATCH may clear are `json.RawMessage`, task 03's idiom
(`PatchIssueRequest.AssigneeID`, `task-03.md:436-446`): absent leaves the field, `null` clears it.

```go
type CreateInitiativeRequest struct {
	Name       string           `json:"name"`
	OwnerID    string           `json:"ownerId"` // u-…, a member of the team (task 118)
	TargetDate *string          `json:"targetDate"`
	State      *InitiativeState `json:"state,omitempty"` // default planned
}

type PatchInitiativeRequest struct {
	Name       *string          `json:"name,omitempty"`
	OwnerID    *string          `json:"ownerId,omitempty"`
	TargetDate json.RawMessage  `json:"targetDate,omitempty"` // "YYYY-MM-DD" | null
	State      *InitiativeState `json:"state,omitempty"`
}

// SetFeatureInitiativeRequest: a feature points at one initiative (05 §3.1).
type SetFeatureInitiativeRequest struct {
	InitiativeID *string `json:"initiativeId"` // null detaches
}

type SetInitiativeTeamsRequest struct {
	TeamIDs []string `json:"teamIds"` // 1-10; [0] must be the owning team
}

type PostUpdateRequest struct {
	Health      Health       `json:"health"`
	Body        AuthoredText `json:"body"`        // author "H", 1-1000 characters (TM-46)
	PinnedBrief string       `json:"pinnedBrief"` // brief:bf-…@r
	Cites       []string     `json:"cites"`       // 0-20 Hub keys; never null
}

type CreateMilestoneRequest struct {
	Name         string   `json:"name"`
	Date         *string  `json:"date"`
	CriterionIDs []string `json:"criterionIds"` // 1-100
}

type PatchMilestoneRequest struct {
	Name *string         `json:"name,omitempty"`
	Date json.RawMessage `json:"date,omitempty"`
}

type SetMilestoneCriteriaRequest struct {
	CriterionIDs []string `json:"criterionIds"` // replaces the list; 1-100
}

type FeatureDependencyRequest struct {
	Blocks string `json:"blocks"` // ft-…
}

type CreateBriefTemplateRequest struct {
	Name             string         `json:"name"`
	Criteria         []AuthoredText `json:"criteria"`
	IssueTemplateIDs []string       `json:"issueTemplateIds"`
}

type PatchBriefTemplateRequest struct {
	Name             *string         `json:"name,omitempty"`
	Criteria         *[]AuthoredText `json:"criteria,omitempty"`
	IssueTemplateIDs *[]string       `json:"issueTemplateIds,omitempty"`
}

// CreateFeatureFromTemplateRequest creates the feature, its branch brief at
// revision 1 from the template's criteria, and one issue per issue template,
// in one transaction (task 120).
type CreateFeatureFromTemplateRequest struct {
	TemplateID string `json:"templateId"`
	Name       string `json:"name"`
	BranchRef  string `json:"branchRef"` // refs/heads/<slug>
}

type GrantProjectRequest struct {
	TeamID string `json:"teamId"`
}
```

#### `work_roadmap.go`, continued: codes and validation

```go
// WorkRoadmapCodeStatus is the status each W4 code carries (decision 3).
// The fake (115) and the server (117-123) read it when writing a problem.
var WorkRoadmapCodeStatus = map[string]int{
	CodeInitiativeNotFound:  404,
	CodeMilestoneNotFound:   404,
	CodeGrantNotFound:       404,
	CodeUpdateImmutable:     409,
	CodeGrantExists:         409,
	CodeDependencyCycle:     422,
	CodeCriterionNotOnBrief: 422,
}

var (
	ErrHealth       = errors.New(`hubapi: health is "on_track", "at_risk" or "off_track"`)
	ErrPinnedBrief  = errors.New("hubapi: a status update pins a brief revision, brief:bf-…@r")
	ErrUpdateBody   = errors.New(`hubapi: an update body is 1 to 1000 characters, author "H"`)
	ErrRoadmapName  = errors.New("hubapi: a name is 1 to 120 characters")
	ErrRoadmapDate  = errors.New("hubapi: a date is YYYY-MM-DD")
	ErrRoadmapList  = errors.New("hubapi: a list is never null and within its bounds")
	ErrRoadmapEnum  = errors.New("hubapi: a value outside its fixed words")
	ErrRoadmapRange = errors.New("hubapi: a number outside its range")
)

var (
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

func validDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func validName(s string) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n >= 1 && n <= 120
}

// ValidHealth is the three words and nothing else (kills "free-text health").
func ValidHealth(h Health) bool {
	return h == HealthOnTrack || h == HealthAtRisk || h == HealthOffTrack
}

func (r PostUpdateRequest) Validate() error {
	if !ValidHealth(r.Health) {
		return &FieldError{"health", ErrHealth}
	}
	if n := len([]rune(strings.TrimSpace(r.Body.Text))); n < 1 || n > 1000 || r.Body.Author != AuthorHuman {
		return &FieldError{"body", ErrUpdateBody}
	}
	if _, err := ParseBriefKey(r.PinnedBrief); err != nil { // keys.go:69: the @r is required
		return &FieldError{"pinnedBrief", ErrPinnedBrief}
	}
	if r.Cites == nil || len(r.Cites) > 20 {
		return &FieldError{"cites", ErrRoadmapList}
	}
	for i, k := range r.Cites {
		if _, err := ParseKey(k); err != nil {
			return &FieldError{"cites[" + strconv.Itoa(i) + "]", ErrInvalidKey}
		}
	}
	return nil
}

func (r CreateInitiativeRequest) Validate() error {
	switch {
	case !validName(r.Name):
		return &FieldError{"name", ErrRoadmapName}
	case !ValidID(PrefixUser, r.OwnerID):
		return &FieldError{"ownerId", ErrInvalidKey}
	case r.TargetDate != nil && !validDate(*r.TargetDate):
		return &FieldError{"targetDate", ErrRoadmapDate}
	case r.State != nil && !validInitiativeState(*r.State):
		return &FieldError{"state", ErrRoadmapEnum}
	}
	return nil
}

func validInitiativeState(s InitiativeState) bool {
	switch s {
	case InitiativePlanned, InitiativeActive, InitiativePaused, InitiativeDone, InitiativeCancelled:
		return true
	}
	return false
}
```

```go
func (r CreateMilestoneRequest) Validate() error {
	switch {
	case !validName(r.Name):
		return &FieldError{"name", ErrRoadmapName}
	case r.Date != nil && !validDate(*r.Date):
		return &FieldError{"date", ErrRoadmapDate}
	}
	return validCriterionList("criterionIds", r.CriterionIDs)
}

func (r SetMilestoneCriteriaRequest) Validate() error {
	return validCriterionList("criterionIds", r.CriterionIDs)
}

// validCriterionList: 1-100 ids, none repeated. Whether each is on the
// feature's brief is the Hub's check (criterion_not_on_brief), not the wire's.
func validCriterionList(field string, ids []string) error {
	if len(ids) < 1 || len(ids) > 100 {
		return &FieldError{field, ErrRoadmapList}
	}
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		if n := len(id); n < 1 || n > 128 || seen[id] {
			return &FieldError{field + "[" + strconv.Itoa(i) + "]", ErrCriterionID}
		}
		seen[id] = true
	}
	return nil
}

func (s TemplateSchedule) Validate() error {
	switch {
	case s.Every.N < 1 || s.Every.N > 365:
		return &FieldError{"every.n", ErrRoadmapRange}
	case s.Every.Unit != "day" && s.Every.Unit != "week" && s.Every.Unit != "month":
		return &FieldError{"every.unit", ErrRoadmapEnum}
	case !clockRe.MatchString(s.At):
		return &FieldError{"at", ErrRoadmapEnum}
	}
	if _, err := time.LoadLocation(s.TZ); err != nil || s.TZ == "" || s.TZ == "Local" {
		return &FieldError{"tz", ErrRoadmapEnum}
	}
	return nil
}

func (p ReminderPolicy) Validate() error {
	if p.Subject.Kind != SubjectFeature && p.Subject.Kind != SubjectInitiative {
		return &FieldError{"subject.kind", ErrRoadmapEnum}
	}
	if p.EveryDays < 1 || p.EveryDays > 90 {
		return &FieldError{"everyDays", ErrRoadmapRange}
	}
	return nil
}

func (r TriageRotation) Validate() error {
	if len(r.Members) < 1 || len(r.Members) > 50 {
		return &FieldError{"members", ErrRoadmapList}
	}
	seen := map[string]bool{}
	for i, m := range r.Members {
		if !ValidID(PrefixUser, m) || seen[m] {
			return &FieldError{"members[" + strconv.Itoa(i) + "]", ErrInvalidKey}
		}
		seen[m] = true
	}
	switch {
	case r.PeriodDays < 1 || r.PeriodDays > 28:
		return &FieldError{"periodDays", ErrRoadmapRange}
	case !validDate(r.StartsOn):
		return &FieldError{"startsOn", ErrRoadmapDate}
	}
	return nil
}

func (q InsightsQuery) Validate() error {
	switch q.Measure {
	case MeasureCount, MeasureCycleTime, MeasureLeadTime, MeasureTriageTime:
	default:
		return &FieldError{"measure", ErrRoadmapEnum}
	}
	from, e1 := time.Parse(time.RFC3339, q.From)
	to, e2 := time.Parse(time.RFC3339, q.To)
	switch {
	case e1 != nil:
		return &FieldError{"from", ErrRoadmapDate}
	case e2 != nil || !to.After(from) || to.Sub(from) > 366*24*time.Hour:
		return &FieldError{"to", ErrRoadmapRange}
	}
	if q.GroupBy != nil {
		switch *q.GroupBy {
		case "state", "assignee", "label", "feature", "cycle":
		default:
			return &FieldError{"groupBy", ErrRoadmapEnum}
		}
	}
	return nil
}

func (s ScopeRef) Validate() error {
	prefix := map[string]string{"project": PrefixProject, "feature": PrefixFeature, "issue": PrefixIssue}[s.Kind]
	if prefix == "" || !ValidID(prefix, s.ID) {
		return &FieldError{"scope", ErrInvalidKey}
	}
	return nil
}
```

#### `hubapi/codes.go` ⚑, appended (complete)

```go
// Work layer, W4 (task 114). Statuses are WorkRoadmapCodeStatus's.
const (
	CodeInitiativeNotFound  = "initiative_not_found"   // 404, details.initiativeId
	CodeMilestoneNotFound   = "milestone_not_found"    // 404, details.milestoneId
	CodeGrantNotFound       = "grant_not_found"        // 404, details.projectId, details.teamId
	CodeUpdateImmutable     = "update_immutable"       // 409, details.updateId
	CodeGrantExists         = "grant_exists"           // 409, details.projectId, details.teamId
	CodeDependencyCycle     = "dependency_cycle"       // 422, details.from, details.blocks
	CodeCriterionNotOnBrief = "criterion_not_on_brief" // 422, details.field "criterionIds[n]"
)
```

#### `hubclient/work.go` ⚑: `Work` embeds `WorkRoadmap` (complete)

`Work` becomes `interface { WorkIssues; WorkPlan; WorkAgents; WorkRoadmap }` (the first three as
tasks 04, 73 and 99 leave it). The interface, appended to the file:

```go
// WorkRoadmap is W4 (SW.20): initiatives, updates, milestones, dependencies,
// brief templates, the janitor's policies, insights, read cursors and grants.
// Every route is in the table below; task 124 is the HTTP half.
type WorkRoadmap interface {
	CreateInitiative(ctx context.Context, teamID string, req hubapi.CreateInitiativeRequest) (hubapi.Initiative, error)
	Initiatives(ctx context.Context, teamID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Initiative], error)
	// Initiative is the view: each readable feature's latest health and date.
	Initiative(ctx context.Context, initiativeID string) (hubapi.InitiativeView, error)
	PatchInitiative(ctx context.Context, initiativeID string, req hubapi.PatchInitiativeRequest) (hubapi.Initiative, error)
	SetFeatureInitiative(ctx context.Context, featureID string, req hubapi.SetFeatureInitiativeRequest) (hubapi.Feature, error)
	SetInitiativeTeams(ctx context.Context, initiativeID string, req hubapi.SetInitiativeTeamsRequest) (hubapi.InitiativeTeams, error)

	// PostUpdate mints an immutable nt- update; a second write is update_immutable.
	PostUpdate(ctx context.Context, subject hubapi.SubjectRef, req hubapi.PostUpdateRequest) (hubapi.StatusUpdate, error)
	// Updates lists a subject's updates, newest first; erased ones included.
	Updates(ctx context.Context, subject hubapi.SubjectRef, page hubapi.PageRequest) (hubapi.Page[hubapi.StatusUpdate], error)

	CreateMilestone(ctx context.Context, featureID string, req hubapi.CreateMilestoneRequest) (hubapi.MilestoneView, error)
	Milestones(ctx context.Context, featureID string) ([]hubapi.MilestoneView, error)
	PatchMilestone(ctx context.Context, milestoneID string, req hubapi.PatchMilestoneRequest) (hubapi.MilestoneView, error)
	SetMilestoneCriteria(ctx context.Context, milestoneID string, req hubapi.SetMilestoneCriteriaRequest) (hubapi.MilestoneView, error)

	AddFeatureDependency(ctx context.Context, featureID string, req hubapi.FeatureDependencyRequest) (hubapi.FeatureDependencyView, error)
	RemoveFeatureDependency(ctx context.Context, featureID, blocksID string) error
	FeatureDependencies(ctx context.Context, projectID string) ([]hubapi.FeatureDependencyView, error)

	CreateBriefTemplate(ctx context.Context, teamID string, req hubapi.CreateBriefTemplateRequest) (hubapi.BriefTemplate, error)
	BriefTemplates(ctx context.Context, teamID string) ([]hubapi.BriefTemplate, error)
	PatchBriefTemplate(ctx context.Context, templateID string, req hubapi.PatchBriefTemplateRequest) (hubapi.BriefTemplate, error)
	CreateFeatureFromTemplate(ctx context.Context, projectID string, req hubapi.CreateFeatureFromTemplateRequest) (hubapi.Feature, error)

	SetTemplateSchedule(ctx context.Context, templateID string, s hubapi.TemplateSchedule) (hubapi.TemplateSchedule, error)
	ClearTemplateSchedule(ctx context.Context, templateID string) error
	SetReminderPolicy(ctx context.Context, p hubapi.ReminderPolicy) (hubapi.ReminderPolicy, error)
	SetTriageRotation(ctx context.Context, teamID string, r hubapi.TriageRotation) (hubapi.TriageRotationView, error)
	TriageRotation(ctx context.Context, teamID string) (hubapi.TriageRotationView, error)

	// QueryInsights reads recorded state moves only; never an estimate.
	QueryInsights(ctx context.Context, q hubapi.InsightsQuery) (hubapi.InsightsResult, error)

	// GetReadCursor is Seq 0 for a scope never looked at.
	GetReadCursor(ctx context.Context, scope hubapi.ScopeRef) (hubapi.ReadCursor, error)
	// AdvanceReadCursor ignores a lower Seq and returns the stored cursor.
	AdvanceReadCursor(ctx context.Context, c hubapi.ReadCursor) (hubapi.ReadCursor, error)
	MarkSeen(ctx context.Context, req hubapi.MarkSeenRequest) ([]hubapi.ReadCursor, error)

	GrantProject(ctx context.Context, projectID string, req hubapi.GrantProjectRequest) (hubapi.ProjectGrant, error)
	RevokeProjectGrant(ctx context.Context, projectID, teamID string) error
	ProjectGrants(ctx context.Context, projectID string) ([]hubapi.ProjectGrant, error)
}
```

Thirty-one methods. A granted team's comment is task 12's `CreateComment`, unchanged: the grant
widens who may call it (task 117), not its shape.

#### The routes (SW.20), one per method; the route task registers each

`V` viewer, `M` member, `A` admin of the owning team; **G** a member of a granted team (task 117).
Paths the scopes of 117-123 fix are theirs; the three marked † are this task's, since those scopes
name the method and not the path.

| Method | Route | Task | Who |
|---|---|---|---|
| `CreateInitiative` / `Initiatives` | `POST` / `GET /v1/teams/{teamId}/initiatives` | 118 | M / V |
| `Initiative` / `PatchInitiative` | `GET` / `PATCH /v1/initiatives/{initiativeId}` | 118 | V / M |
| `SetFeatureInitiative` | `PUT /v1/features/{featureId}/initiative` | 118 | M |
| `SetInitiativeTeams` | `PUT /v1/initiatives/{initiativeId}/teams` | 117 | A |
| `PostUpdate` / `Updates` | `POST` / `GET /v1/features/{featureId}/updates` and `…/v1/initiatives/{initiativeId}/updates` | 119 | M, a person / V, G |
| `CreateMilestone` / `Milestones` | `POST` / `GET /v1/features/{featureId}/milestones` | 120 | M / V, G |
| `PatchMilestone` | `PATCH /v1/milestones/{milestoneId}` | 120 | M |
| `SetMilestoneCriteria` † | `PUT /v1/milestones/{milestoneId}/criteria` | 120 | M |
| `AddFeatureDependency` | `POST /v1/features/{featureId}/dependencies` | 120 | M |
| `RemoveFeatureDependency` | `DELETE /v1/features/{featureId}/dependencies/{blocks}` | 120 | M |
| `FeatureDependencies` † | `GET /v1/projects/{projectId}/feature-dependencies` | 120 | V, G |
| `CreateBriefTemplate` / `BriefTemplates` | `POST` / `GET /v1/teams/{teamId}/templates` (`kind: "brief"`; task 78's route) | 120 | A / V |
| `PatchBriefTemplate` | `PATCH /v1/templates/{templateId}` (task 78's route) | 120 | A |
| `CreateFeatureFromTemplate` | `POST /v1/projects/{projectId}/features:fromTemplate` | 120 | M |
| `SetTemplateSchedule` / `ClearTemplateSchedule` | `PUT` / `DELETE /v1/templates/{templateId}/schedule` | 121 | A |
| `SetReminderPolicy` | `PUT /v1/reminder-policies` | 121 | the lead, the owner, or A |
| `SetTriageRotation` / `TriageRotation` | `PUT` / `GET /v1/teams/{teamId}/triage-rotation` | 121 | A / V |
| `QueryInsights` | `POST /v1/teams/{teamId}/insights/query` | 122 | V |
| `GetReadCursor` / `AdvanceReadCursor` | `GET /v1/me/read-cursors?scope={kind}:{id}` / `PUT /v1/me/read-cursors` | 123 | self |
| `MarkSeen` | `POST /v1/me/read-cursors/mark-seen` | 123 | self |
| `GrantProject` / `ProjectGrants` | `POST` / `GET /v1/projects/{projectId}/grants` | 117 | A / V |
| `RevokeProjectGrant` † | `DELETE /v1/projects/{projectId}/grants/{teamId}` | 117 | A |

† `RevokeProjectGrant`'s path is 117's (`…/grants/{teamId}`); the dagger marks only that its
`{teamId}` segment name is fixed here. `features:fromTemplate` is one literal path segment, which
Go 1.22's `ServeMux` accepts (a wildcard must be a whole segment; a literal with a colon is not a
wildcard).

#### `hubclient/fake_work_roadmap_stub.go`, complete

One line per method; gofmt keeps a one-statement body on one line.

```go
package hubclient

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Deleted by task 115 in the commit that adds fake_work_roadmap.go. Every
// method is task 04's 501 not_in_fake_yet (task-04.md:1101).

type pr = hubapi.PageRequest

func (f *Fake) CreateInitiative(_ context.Context, t string, r hubapi.CreateInitiativeRequest) (hubapi.Initiative, error) { return hubapi.Initiative{}, f.workNotInFakeYet("CreateInitiative", t, r) }
func (f *Fake) Initiatives(_ context.Context, t string, p pr) (hubapi.Page[hubapi.Initiative], error) { return hubapi.Page[hubapi.Initiative]{}, f.workNotInFakeYet("Initiatives", t, p) }
func (f *Fake) Initiative(_ context.Context, id string) (hubapi.InitiativeView, error) { return hubapi.InitiativeView{}, f.workNotInFakeYet("Initiative", id) }
func (f *Fake) PatchInitiative(_ context.Context, id string, r hubapi.PatchInitiativeRequest) (hubapi.Initiative, error) { return hubapi.Initiative{}, f.workNotInFakeYet("PatchInitiative", id, r) }
func (f *Fake) SetFeatureInitiative(_ context.Context, id string, r hubapi.SetFeatureInitiativeRequest) (hubapi.Feature, error) { return hubapi.Feature{}, f.workNotInFakeYet("SetFeatureInitiative", id, r) }
func (f *Fake) SetInitiativeTeams(_ context.Context, id string, r hubapi.SetInitiativeTeamsRequest) (hubapi.InitiativeTeams, error) { return hubapi.InitiativeTeams{}, f.workNotInFakeYet("SetInitiativeTeams", id, r) }
func (f *Fake) PostUpdate(_ context.Context, s hubapi.SubjectRef, r hubapi.PostUpdateRequest) (hubapi.StatusUpdate, error) { return hubapi.StatusUpdate{}, f.workNotInFakeYet("PostUpdate", s, r) }
func (f *Fake) Updates(_ context.Context, s hubapi.SubjectRef, p pr) (hubapi.Page[hubapi.StatusUpdate], error) { return hubapi.Page[hubapi.StatusUpdate]{}, f.workNotInFakeYet("Updates", s, p) }
func (f *Fake) CreateMilestone(_ context.Context, id string, r hubapi.CreateMilestoneRequest) (hubapi.MilestoneView, error) { return hubapi.MilestoneView{}, f.workNotInFakeYet("CreateMilestone", id, r) }
func (f *Fake) Milestones(_ context.Context, id string) ([]hubapi.MilestoneView, error) { return nil, f.workNotInFakeYet("Milestones", id) }
func (f *Fake) PatchMilestone(_ context.Context, id string, r hubapi.PatchMilestoneRequest) (hubapi.MilestoneView, error) { return hubapi.MilestoneView{}, f.workNotInFakeYet("PatchMilestone", id, r) }
func (f *Fake) SetMilestoneCriteria(_ context.Context, id string, r hubapi.SetMilestoneCriteriaRequest) (hubapi.MilestoneView, error) { return hubapi.MilestoneView{}, f.workNotInFakeYet("SetMilestoneCriteria", id, r) }
func (f *Fake) AddFeatureDependency(_ context.Context, id string, r hubapi.FeatureDependencyRequest) (hubapi.FeatureDependencyView, error) { return hubapi.FeatureDependencyView{}, f.workNotInFakeYet("AddFeatureDependency", id, r) }
func (f *Fake) RemoveFeatureDependency(_ context.Context, id, b string) error { return f.workNotInFakeYet("RemoveFeatureDependency", id, b) }
func (f *Fake) FeatureDependencies(_ context.Context, id string) ([]hubapi.FeatureDependencyView, error) { return nil, f.workNotInFakeYet("FeatureDependencies", id) }
func (f *Fake) CreateBriefTemplate(_ context.Context, t string, r hubapi.CreateBriefTemplateRequest) (hubapi.BriefTemplate, error) { return hubapi.BriefTemplate{}, f.workNotInFakeYet("CreateBriefTemplate", t, r) }
func (f *Fake) BriefTemplates(_ context.Context, t string) ([]hubapi.BriefTemplate, error) { return nil, f.workNotInFakeYet("BriefTemplates", t) }
func (f *Fake) PatchBriefTemplate(_ context.Context, id string, r hubapi.PatchBriefTemplateRequest) (hubapi.BriefTemplate, error) { return hubapi.BriefTemplate{}, f.workNotInFakeYet("PatchBriefTemplate", id, r) }
func (f *Fake) CreateFeatureFromTemplate(_ context.Context, id string, r hubapi.CreateFeatureFromTemplateRequest) (hubapi.Feature, error) { return hubapi.Feature{}, f.workNotInFakeYet("CreateFeatureFromTemplate", id, r) }
func (f *Fake) SetTemplateSchedule(_ context.Context, id string, s hubapi.TemplateSchedule) (hubapi.TemplateSchedule, error) { return hubapi.TemplateSchedule{}, f.workNotInFakeYet("SetTemplateSchedule", id, s) }
func (f *Fake) ClearTemplateSchedule(_ context.Context, id string) error { return f.workNotInFakeYet("ClearTemplateSchedule", id) }
func (f *Fake) SetReminderPolicy(_ context.Context, p hubapi.ReminderPolicy) (hubapi.ReminderPolicy, error) { return hubapi.ReminderPolicy{}, f.workNotInFakeYet("SetReminderPolicy", p) }
func (f *Fake) SetTriageRotation(_ context.Context, t string, r hubapi.TriageRotation) (hubapi.TriageRotationView, error) { return hubapi.TriageRotationView{}, f.workNotInFakeYet("SetTriageRotation", t, r) }
func (f *Fake) TriageRotation(_ context.Context, t string) (hubapi.TriageRotationView, error) { return hubapi.TriageRotationView{}, f.workNotInFakeYet("TriageRotation", t) }
func (f *Fake) QueryInsights(_ context.Context, q hubapi.InsightsQuery) (hubapi.InsightsResult, error) { return hubapi.InsightsResult{}, f.workNotInFakeYet("QueryInsights", q) }
func (f *Fake) GetReadCursor(_ context.Context, s hubapi.ScopeRef) (hubapi.ReadCursor, error) { return hubapi.ReadCursor{}, f.workNotInFakeYet("GetReadCursor", s) }
func (f *Fake) AdvanceReadCursor(_ context.Context, c hubapi.ReadCursor) (hubapi.ReadCursor, error) { return hubapi.ReadCursor{}, f.workNotInFakeYet("AdvanceReadCursor", c) }
func (f *Fake) MarkSeen(_ context.Context, r hubapi.MarkSeenRequest) ([]hubapi.ReadCursor, error) { return nil, f.workNotInFakeYet("MarkSeen", r) }
func (f *Fake) GrantProject(_ context.Context, id string, r hubapi.GrantProjectRequest) (hubapi.ProjectGrant, error) { return hubapi.ProjectGrant{}, f.workNotInFakeYet("GrantProject", id, r) }
func (f *Fake) RevokeProjectGrant(_ context.Context, id, t string) error { return f.workNotInFakeYet("RevokeProjectGrant", id, t) }
func (f *Fake) ProjectGrants(_ context.Context, id string) ([]hubapi.ProjectGrant, error) { return nil, f.workNotInFakeYet("ProjectGrants", id) }
```

**`hubclient/client_work_roadmap_stub.go`** is the same thirty-one lines with the receiver
`(c *Client)` and every body `return <zero>, ErrNotConfigured` (task 04's client idiom,
`task-04.md:1169`), and no `pr` alias (it reuses this file's, same package). Step 3 derives it
from this file and checks it with `go vet`; task 124 deletes it.

#### Tests first

`hubapi/work_roadmap_test.go`, in `types_test.go`'s styles. Every row fails before the code exists
and fails again on its mutant.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorkRoadmapGoldens` | each of the seventeen goldens decodes strictly (`DisallowUnknownFields`) and re-encodes byte for byte | a field renamed or retagged (`target_date`, `pinned_brief`) |
| `TestWorkRoadmapEmptyListsAreArrays` | a zero `InitiativeTeams`, `StatusUpdate`, `Milestone`, `InsightsResult` built by its constructor encodes `[]`, never `null` | `omitempty` on a list |
| `TestHealthIsThreeWords` | `ValidHealth` true for the three; `"green"`, `""`, `"On_Track"` false; `PostUpdateRequest.Validate` names `health` | free-text health |
| `TestAnUpdateIsPinned` | `pinnedBrief` `""`, `brief:bf-…` with no `@r`, and `issue:is-…@1` each fail on `pinnedBrief` | an unpinned update |
| `TestAnUpdateIsWrittenByAPerson` | a body with `author: "A"` fails on `body` | an agent-authored update |
| `TestNoPredictionFields` | the reflection walk below over every W4 type finds no field named for a prediction | a prediction field (OD-27) |
| `TestPercentilesAreNullWhenUnmeasured` | `InsightsResult{Groups: []…{}, Points: []…{}}` encodes `"p50Ms":null` | an invented zero percentile |
| `TestScheduleBounds` | `every.n` 0 and 366, unit `"year"`, `at` `"24:00"`, tz `"Local"` and `"Mars/Base"` each fail on its field | an unbounded or local-clock schedule |
| `TestInsightsWindow` | `to` before `from`, and a window over 366 days, fail on `to`; `groupBy: "priority"` fails | an unbounded query |
| `TestScopeKinds` | `ScopeRef{"cycle", …}` fails; a `feature` scope with an `is-` id fails | a scope of the wrong kind |
| `TestWorkRoadmapCodes` | each of the seven codes is in `WorkRoadmapCodeStatus` with its table status, and no code is also in another Work map | a wrong status; a code declared twice |

The reflection test, complete, because it is the rule OD-27 makes mechanical:

```go
var predictionWords = map[string]bool{
	"percent": true, "progress": true, "eta": true, "forecast": true, "remaining": true,
}

// camelWords splits "latestUpdate" or "p50Ms" into lower-case words.
func camelWords(s string) []string {
	var out []string
	start := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || (unicode.IsUpper(rune(s[i])) && !unicode.IsUpper(rune(s[i-1]))) {
			out = append(out, strings.ToLower(s[start:i]))
			start = i
		}
	}
	return out
}

func TestNoPredictionFields(t *testing.T) {
	types := []any{Initiative{}, InitiativeTeams{}, StatusUpdate{}, Milestone{}, MilestoneView{},
		FeatureDependency{}, FeatureDependencyView{}, BriefTemplate{}, TemplateSchedule{},
		ReminderPolicy{}, TriageRotation{}, TriageRotationView{}, InsightsQuery{}, InsightsResult{},
		InsightGroup{}, InsightPoint{}, ReadCursor{}, ProjectGrant{}, InitiativeView{},
		InitiativeFeature{}, UpdateStamp{}, CriterionWord{}}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(rt reflect.Type, at string) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			for _, name := range []string{f.Name, tag} {
				for _, w := range camelWords(name) {
					if predictionWords[w] {
						t.Errorf("%s.%s: %q is a prediction word (OD-27)", at, f.Name, w)
					}
				}
			}
			walk(f.Type, at+"."+f.Name)
		}
	}
	for _, v := range types {
		walk(reflect.TypeOf(v), reflect.TypeOf(v).Name())
	}
}
```

Word matching, not substring matching: `metadata` and `beta` hold `eta` and must pass;
`etaMs` and `percentDone` must fail. `TestNoPredictionFieldsCatchesOne` runs `walk` over a local
`struct{ EtaMs int64 \`json:"etaMs"\` }` and wants exactly one error, so the test cannot pass
by never matching.
