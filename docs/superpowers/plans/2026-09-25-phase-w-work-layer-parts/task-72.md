### Task 72: `hubapi` and `hubclient.WorkPlan`: the W2 wire types and methods

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 70 (W1's proof), after a sync-merge of `feat/hub` (rule H1). **Seams:** defines
**SW.18**, first half: every W2 Work record, enum, code and request body on the wire, and the
`WorkPlan` method set. SW.18 freezes with task 73; afterwards only added methods, fields,
constants or routes (skeleton, *Seams*).

W2 is planning: custom states, features, cycles, sub-issues, templates, label groups, estimates,
due dates, rank, moves, views and drafts. Eleven route tasks (75-81), the fake (73), the HTTP
client (82) and the desktop (83-94) all speak these shapes. If each declared its own, one would
spell `due_on` or drop an empty `display`, and a view saved on one screen would load wrong on
another. So every W2 shape lives in one file, `hubapi/work_plan.go`, in `types.go`'s idiom, and
every W2 method in one interface, `WorkPlan`, declared standalone here and embedded by task 73.

**Anchors, re-read on 2026-09-26 in the real tree at `feat/hub` `277e6c9a`** (under
`control-plane/internal/hub/hubapi/`): `types_test.go:213` (`jsonFields`, which flattens an
untagged embedded struct, so decision (a) is seen by task 03's tests), `:361` (`decodeInto`);
`codes.go:35` (`CodeBriefExists`, reused for a feature's ref); `keys.go:172` (`ValidBranchRef`).
**Read in this branch's parts, not yet code, so UNVERIFIED until merge:** task 02's `workids.go`
(`task-02.md:157-172`: `PrefixFeature`, `PrefixCycle`, `PrefixLabel`, `PrefixTemplate`,
`PrefixView`, `PrefixDraft`; none for a closure proposal), `AuthoredText` and `Serve`
(`task-02.md:239-256`), `TestIssueRevisionGoldenVectors` (`task-02.md:812`); task 03's `work.go`
(`IssueState` and its lists `task-03.md:98-122`, `Priority` `:128`, `IssueHead` `:282-303`,
`Label` `:339-344`, `TeamPolicy` `:348-355`, `CreateLabelRequest` `:499-502`,
`PutTeamPolicyRequest` `:504-508`) and `work_test.go` (`workExamples` `:644-660`); task 04's
`hubclient/work.go` (`task-04.md:153-233`) and its `client_work_stub.go` idiom (`:138-147`);
task 114's code-status map (`task-114.md:48-51`). Step 1 takes the merged names.

#### Decisions the scope left open, made here

**(a) W2 fields on a W1 struct arrive as one embedded struct each.** `IssueHead` gains
`IssuePlan`, `TeamPolicy` gains `TeamPlanPolicy`, `Label` gains `LabelPlan`,
`PutTeamPolicyRequest` gains `TeamPlanPolicyPatch` and `CreateLabelRequest` gains
`LabelPlanRequest`: five lines in task 03's `work.go`, each an untagged embedded type name, and
every W2 field is in `work_plan.go`. `encoding/json` and `jsonFields` both flatten them, so the
wire is the scope's flat shape. The cost is one spelling: a keyed literal names the embedded
struct (`IssueHead{IssuePlan: IssuePlan{Rank: "i"}}`); reads promote (`h.Rank`).

**(b) `PutTeamPolicyRequest`'s W2 half is a patch.** Its fields are pointers, and absent keeps.
Task 43's settings screen sends W1's three fields; a whole-replacement W2 half would read its
missing `estimateScale` as `""` and refuse it, breaking a W1 client that did nothing wrong.

**(c) `Label.TeamID` stays a `string`; an org label carries `teamId: ""`.** The scope's "nil"
would change a W1 field's type, which SW.1's freeze forbids (task 03's header). `LabelGroup` and
`Template` are new, so their `TeamID` is `*string`, null for org. Task 74's `labels.team_id` is
nullable and the store maps NULL to `""`. Recorded under *Corrections*.

**(d) Tri-state request fields are `json.RawMessage`, read through one generic `Change[T]`.**
Task 03's decision (c) made generic: absent keeps, `null` clears, a value sets. A `*int` cannot
clear an estimate, because `encoding/json` sets a pointer to nil for both absent and `null`.

**(e) The validators live here**, not in the fake or the server: `ValidStateBand`, `BandHolds`,
`ValidCustomStateKey`, `ValidRank`, `ValidDate`, `ValidEstimate`, `FeatureBranchRef` and
`FilterGroup.Validate`. Task 73's fake and tasks 75-80's server call the same functions, and task
74's `CHECK`s are compared with these lists by `TestWorkPlanCheckSetsMatchHubapi`.

**(f) W2's problem codes carry their statuses** in `WorkPlanCodeStatus`, task 114's idiom, so the
fake (73) and the server (75-80) write a code's status once.

**(g) A closure proposal's id has no prefix.** Task 02's twelve are frozen and 05 §3.6 lists none;
the id is an opaque server string the client never parses. A prefix is a skeleton gap, reported.

**(h) Custom states are written through `CustomStateSpec`**, which has no `teamId`: the path names
the team, and a strict decoder refuses a body that names another.

**Files:**

| Path | Change |
|---|---|
| `control-plane/internal/hub/hubapi/work_plan.go` | new: every W2 enum, validator, code, record and request body |
| `control-plane/internal/hub/hubapi/work_plan_test.go` | new: the four `types_test.go` styles, run over the W2 types, and the validators |
| `control-plane/internal/hub/hubapi/work.go` ⚑ | five lines: the embedded W2 halves of decision (a) |
| `control-plane/internal/hub/hubapi/work_test.go` ⚑ | three `workExamples` rows (`IssueHead`, `Label`, `TeamPolicy`) gain their W2 keys |
| `control-plane/internal/hub/hubapi/workids.go` | unchanged: `ft-`, `cy-`, `tp-`, `lb-`, `vw-`, `df-` are task 02's |
| `control-plane/internal/hub/hubclient/work.go` ⚑ | `WorkPlan` declared standalone; `Work` is unchanged until task 73 |

---

#### `work_plan.go`, complete, in six blocks

Block 1, the enums. Each is a typed string with an ordered list, task 03's decision (f), so task
74's `CHECK` sets and task 83's TypeScript unions compare against one Go list.

```go
package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"time"
)

// StateBand is where a team's custom state may sit. Only the two human bands
// exist: nothing sits between started and landed, which are facts (05 §3.3).
type StateBand string

const (
	BandBeforeStarted StateBand = "before_started" // on triage, backlog or todo
	BandAfterLanded   StateBand = "after_landed"   // on landed; holds done until a person clears it
)

var StateBands = []StateBand{BandBeforeStarted, BandAfterLanded}

// FeatureState is set by a person only. What a feature's lanes did (live,
// landed, released where) is derived at read time and never stored (rule 3).
type FeatureState string

const (
	FeaturePlanned   FeatureState = "planned"
	FeatureActive    FeatureState = "active"
	FeaturePaused    FeatureState = "paused"
	FeatureDone      FeatureState = "done"
	FeatureCancelled FeatureState = "cancelled"
)

var FeatureStates = []FeatureState{FeaturePlanned, FeatureActive, FeaturePaused, FeatureDone, FeatureCancelled}

type CycleState string

const (
	CycleUpcoming CycleState = "upcoming"
	CycleCurrent  CycleState = "current"
	CycleClosed   CycleState = "closed"
)

var CycleStates = []CycleState{CycleUpcoming, CycleCurrent, CycleClosed}

type ScopeChange string

const (
	ScopeAdded   ScopeChange = "added"
	ScopeRemoved ScopeChange = "removed"
)

var ScopeChanges = []ScopeChange{ScopeAdded, ScopeRemoved}

// EstimateScale names a team's points. The wire carries integers only; the
// desktop owns the labels (the t-shirt scale reads XS..XL there).
type EstimateScale string

const (
	EstimateNone        EstimateScale = "none" // estimates off: every estimate is refused
	EstimateExponential EstimateScale = "exponential"
	EstimateFibonacci   EstimateScale = "fibonacci"
	EstimateLinear      EstimateScale = "linear"
	EstimateTShirt      EstimateScale = "tshirt"
)

var EstimateScales = []EstimateScale{EstimateNone, EstimateExponential, EstimateFibonacci, EstimateLinear, EstimateTShirt}

// EstimatePoints is each scale's values, zero excluded: TeamPlanPolicy.EstimateZero adds it.
var EstimatePoints = map[EstimateScale][]int{
	EstimateNone:        {},
	EstimateExponential: {1, 2, 4, 8, 16},
	EstimateFibonacci:   {1, 2, 3, 5, 8},
	EstimateLinear:      {1, 2, 3, 4, 5},
	EstimateTShirt:      {1, 2, 3, 4, 5},
}
```

```go
type StaleMode string

const (
	StalePropose StaleMode = "propose" // one ClosureProposal per assignee, on their Desk
	StaleAuto    StaleMode = "auto"    // the janitor closes, naming the policy's author
)

var StaleModes = []StaleMode{StalePropose, StaleAuto}

type TemplateKind string

const TemplateIssue TemplateKind = "issue" // W4 adds brief templates (L-122); W5 forms (L-27)

var TemplateKinds = []TemplateKind{TemplateIssue}

type ViewLayout string

const (
	LayoutList  ViewLayout = "list"
	LayoutBoard ViewLayout = "board"
)

var ViewLayouts = []ViewLayout{LayoutList, LayoutBoard}

type DraftKind string

const (
	DraftIssue   DraftKind = "issue"
	DraftComment DraftKind = "comment"
)

var DraftKinds = []DraftKind{DraftIssue, DraftComment}

// FilterOp joins a group's terms and subgroups.
type FilterOp string

const (
	FilterAnd FilterOp = "and"
	FilterOr  FilterOp = "or"
)

var FilterOps = []FilterOp{FilterAnd, FilterOr}

// FilterTermOp is one term's comparison. empty and not_empty take no values;
// every other op takes at least one.
type FilterTermOp string

const (
	TermIs       FilterTermOp = "is"     // any of values
	TermIsNot    FilterTermOp = "is_not" // none of values
	TermBefore   FilterTermOp = "before" // dates only: one value
	TermAfter    FilterTermOp = "after"  // dates only: one value
	TermEmpty    FilterTermOp = "empty"
	TermNotEmpty FilterTermOp = "not_empty"
)

var FilterTermOps = []FilterTermOp{TermIs, TermIsNot, TermBefore, TermAfter, TermEmpty, TermNotEmpty}

// FilterFields is every field a view may filter on: head fields and the
// thread's words, never a revision's text (task 80 evaluates; 83 mirrors).
var FilterFields = []string{"state", "subState", "priority", "assignee", "label", "feature",
	"cycle", "parent", "estimate", "dueOn", "creator", "source", "threadState", "proof",
	"serves", "createdAt", "updatedAt"}

// FilterDateFields are the fields before and after accept.
var FilterDateFields = []string{"dueOn", "createdAt", "updatedAt"}

// ViewGroupBys and ViewOrderBys are a view's grouping and ordering keys; ""
// is no grouping. ViewDisplayProps are the row properties a view may show.
var (
	ViewGroupBys     = []string{"", "state", "subState", "assignee", "priority", "label", "feature", "cycle", "parent"}
	ViewOrderBys     = []string{"rank", "priority", "number", "createdAt", "updatedAt", "dueOn", "estimate"}
	ViewDisplayProps = []string{"alias", "priority", "assignee", "labels", "estimate", "dueOn",
		"feature", "cycle", "subState", "thread", "proof", "updatedAt"}
)
```

Block 2, the validators, one function per rule, shared by the fake (73) and the server (75-80).

```go
var (
	rankRE           = regexp.MustCompile(`^[0-9a-z]{1,32}$`)
	customStateKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	featureSlugRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

const (
	RankMaxLen     = 32 // past this, task 79's server rebalances; it never lengthens a key
	MaxFilterDepth = 3  // the top group is depth 1
)

func ValidStateBand(b StateBand) bool       { return slices.Contains(StateBands, b) }
func ValidFeatureState(s FeatureState) bool { return slices.Contains(FeatureStates, s) }
func ValidCycleState(s CycleState) bool     { return slices.Contains(CycleStates, s) }
func ValidEstimateScale(s EstimateScale) bool { return slices.Contains(EstimateScales, s) }
func ValidStaleMode(m StaleMode) bool       { return slices.Contains(StaleModes, m) }
func ValidViewLayout(l ViewLayout) bool     { return slices.Contains(ViewLayouts, l) }
func ValidDraftKind(k DraftKind) bool       { return slices.Contains(DraftKinds, k) }
func ValidRank(s string) bool               { return rankRE.MatchString(s) }

// BandHolds reports whether a head in state s may carry a sub-state of band b.
func BandHolds(b StateBand, s IssueState) bool {
	switch b {
	case BandBeforeStarted:
		return slices.Contains(InitialStates, s)
	case BandAfterLanded:
		return s == StateLanded
	}
	return false
}

// ValidCustomStateKey refuses every IssueStates name, so no custom state can
// rename, shadow or stand in for a fact state (05 §3.3).
func ValidCustomStateKey(k string) bool {
	return customStateKeyRE.MatchString(k) && !slices.Contains(IssueStates, IssueState(k))
}

// ValidDate is a calendar day with no time and no zone (dueOn, L-37).
func ValidDate(s string) bool {
	t, err := time.Parse(time.DateOnly, s)
	return err == nil && t.Format(time.DateOnly) == s
}

// ValidEstimate reports whether v is on the team's scale. Scale none refuses
// every value; zero needs EstimateZero.
func ValidEstimate(p TeamPlanPolicy, v int) bool {
	if p.EstimateScale == EstimateNone || !ValidEstimateScale(p.EstimateScale) {
		return false
	}
	if v == 0 {
		return p.EstimateZero
	}
	return slices.Contains(EstimatePoints[p.EstimateScale], v)
}

// FeatureBranchRef is the ref a feature reserves, refs/heads/<slug>. ok is
// false for a slug that is not one lower-case component, or that the Hub's
// ref rules refuse (ValidBranchRef, keys.go:172). The Hub never reads git.
func FeatureBranchRef(slug string) (ref string, ok bool) {
	ref = "refs/heads/" + slug
	return ref, featureSlugRE.MatchString(slug) && ValidBranchRef(ref)
}

// ErrFilterTooDeep is FilterGroup.Validate's answer to a fourth level.
var ErrFilterTooDeep = errors.New("filter: groups nest deeper than 3")

// Validate checks a view's filter: ops, fields, value counts, depth <= 3.
// The error text names the path ("groups[0].terms[1].op") for details.field.
func (g FilterGroup) Validate() error { return g.validate("", 1) }

func (g FilterGroup) validate(path string, depth int) error {
	if depth > MaxFilterDepth {
		return ErrFilterTooDeep
	}
	if !slices.Contains(FilterOps, g.Op) {
		return fmt.Errorf("%sop: %q is not and or or", path, g.Op)
	}
	for i, t := range g.Terms {
		at := fmt.Sprintf("%sterms[%d].", path, i)
		switch {
		case !slices.Contains(FilterFields, t.Field):
			return fmt.Errorf("%sfield: %q", at, t.Field)
		case !slices.Contains(FilterTermOps, t.Op):
			return fmt.Errorf("%sop: %q", at, t.Op)
		case (t.Op == TermEmpty || t.Op == TermNotEmpty) != (len(t.Values) == 0):
			return fmt.Errorf("%svalues: %d for %s", at, len(t.Values), t.Op)
		case (t.Op == TermBefore || t.Op == TermAfter) && (!slices.Contains(FilterDateFields, t.Field) || len(t.Values) != 1 || !ValidDate(t.Values[0])):
			return fmt.Errorf("%svalues: %s takes one date on a date field", at, t.Op)
		}
	}
	for i, sub := range g.Groups {
		if err := sub.validate(fmt.Sprintf("%sgroups[%d].", path, i), depth+1); err != nil {
			return err
		}
	}
	return nil
}
```

Block 3, the codes. The Hub's rule is one `*_not_found` per route family (`codes.go:24-33`);
`brief_exists` is reused, not redeclared, for a second feature on a ref.

```go
const (
	CodeFeatureNotFound     = "feature_not_found"     // 404, details.featureId
	CodeCycleNotFound       = "cycle_not_found"       // 404, details.cycleId
	CodeTemplateNotFound    = "template_not_found"    // 404, details.templateId
	CodeViewNotFound        = "view_not_found"        // 404, details.viewId
	CodeDraftNotFound       = "draft_not_found"       // 404; also another person's draft (L-33)
	CodeProposalNotFound    = "proposal_not_found"    // 404, details.proposalId
	CodeHeldBySubState      = "held_by_substate"      // 409, details.subState
	CodeStateInUse          = "state_in_use"          // 409, details.key, details.count
	CodeChildrenOpen        = "children_open"         // 409, details.open (a count)
	CodeSubStateOutOfBand   = "substate_out_of_band"  // 422, details.key, details.band, details.state
	CodeExclusiveLabelGroup = "exclusive_label_group" // 422, details.groupId, details.labels
	CodeEstimateOffScale    = "estimate_off_scale"    // 422, details.scale, details.value
	CodeParentLoop          = "parent_loop"           // 422, details.parentId
	CodeParentOtherProject  = "parent_other_project"  // 422, details.parentId
	CodeCycleOverlap        = "cycle_overlap"         // 422, details.cycleId (the neighbour)
	CodeFilterInvalid       = "filter_invalid"        // 422, details.field
	CodeDateInvalid         = "date_invalid"          // 422, details.field
	CodeRankNeighbours      = "rank_neighbours"       // 422, details.field "before" | "after"
	CodeSlugInvalid         = "slug_invalid"          // 422, details.slug
)

// WorkPlanCodeStatus is each W2 code's HTTP status, written once.
var WorkPlanCodeStatus = map[string]int{
	CodeFeatureNotFound: http.StatusNotFound, CodeCycleNotFound: http.StatusNotFound,
	CodeTemplateNotFound: http.StatusNotFound, CodeViewNotFound: http.StatusNotFound,
	CodeDraftNotFound: http.StatusNotFound, CodeProposalNotFound: http.StatusNotFound,
	CodeHeldBySubState: http.StatusConflict, CodeStateInUse: http.StatusConflict,
	CodeChildrenOpen: http.StatusConflict,
	CodeSubStateOutOfBand: http.StatusUnprocessableEntity, CodeExclusiveLabelGroup: http.StatusUnprocessableEntity,
	CodeEstimateOffScale: http.StatusUnprocessableEntity, CodeParentLoop: http.StatusUnprocessableEntity,
	CodeParentOtherProject: http.StatusUnprocessableEntity, CodeCycleOverlap: http.StatusUnprocessableEntity,
	CodeFilterInvalid: http.StatusUnprocessableEntity, CodeDateInvalid: http.StatusUnprocessableEntity,
	CodeRankNeighbours: http.StatusUnprocessableEntity, CodeSlugInvalid: http.StatusUnprocessableEntity,
}
```

Block 4, the W2 halves of W1 structs (decision (a)), and `IssueAlias`.

```go
// IssuePlan is IssueHead's W2 half. None of it is in the issue-revision
// canonical form, so planning never re-hashes a revision and task 02's golden
// vectors are unchanged.
type IssuePlan struct {
	Estimate  *int        `json:"estimate"`  // on the team's scale; null is none
	DueOn     *string     `json:"dueOn"`     // ValidDate
	Rank      string      `json:"rank"`      // ValidRank; the server's, never a client's
	ParentID  *string     `json:"parentId"`  // is-… in the same project
	FeatureID *string     `json:"featureId"` // ft-… in the same project
	CycleID   *string     `json:"cycleId"`   // cy-… of the project's team
	SubState  *string     `json:"subState"`  // a CustomState key whose band holds State
	MovedFrom *IssueAlias `json:"movedFrom"` // the alias before the last move, or null
}

// IssueAlias is a number an issue once had. Never deleted, never reused (L-35).
type IssueAlias struct {
	ProjectID  string `json:"projectId"`
	ProjectKey string `json:"projectKey"` // "" while that project has no key
	Number     int    `json:"number"`
}

type TeamPlanPolicy struct {
	EstimateScale    EstimateScale `json:"estimateScale"` // EstimateNone for a team that never set one
	EstimateZero     bool          `json:"estimateZero"`
	StaleAfterDays   int           `json:"staleAfterDays"` // 0: never; else 1-365
	StaleMode        StaleMode     `json:"staleMode"`      // StalePropose for a team that never set one
	ArchiveAfterDays int           `json:"archiveAfterDays"` // 0: never; else 1-3650
}
```

```go
// TeamPlanPolicyPatch is PutTeamPolicyRequest's W2 half (decision (b)): absent
// keeps, so a W1 body means what it meant.
type TeamPlanPolicyPatch struct {
	EstimateScale    *EstimateScale `json:"estimateScale,omitempty"`
	EstimateZero     *bool          `json:"estimateZero,omitempty"`
	StaleAfterDays   *int           `json:"staleAfterDays,omitempty"`
	StaleMode        *StaleMode     `json:"staleMode,omitempty"`
	ArchiveAfterDays *int           `json:"archiveAfterDays,omitempty"`
}

// LabelPlan is Label's W2 half. An org label has Label.TeamID "" (decision (c)).
type LabelPlan struct {
	GroupID *string `json:"groupId"` // lb-… of a LabelGroup, or null
}

// LabelPlanRequest is CreateLabelRequest's W2 half.
type LabelPlanRequest struct {
	GroupID *string `json:"groupId,omitempty"`
}
```

The `work.go` delta, five lines, each an untagged embedded name placed after the struct's last
field (task 03's struct otherwise untouched):

```diff
 type IssueHead struct {
 	…
 	UpdatedAt    string       `json:"updatedAt"`
+	IssuePlan
 }
 type Label struct {
 	…
 	Color  string `json:"color"`
+	LabelPlan
 }
 type TeamPolicy struct {
 	…
 	UpdatedBy                 *User      `json:"updatedBy"`
+	TeamPlanPolicy
 }
 type CreateLabelRequest struct {
 	…
 	Color string `json:"color"` // a token name
+	LabelPlanRequest
 }
 type PutTeamPolicyRequest struct {
 	…
 	StopAtLandedWithoutServes bool       `json:"stopAtLandedWithoutServes"`
+	TeamPlanPolicyPatch
 }
```

The `work_test.go` delta: the three `workExamples` rows gain their W2 keys, in struct order,
so `TestWorkFieldNamesMatchTheExamples` keeps comparing every key. Each row is the task 03 row
with this suffix inserted before its closing `}`:

```text
IssueHead:  ,"estimate":3,"dueOn":"2026-10-02","rank":"i","parentId":null,"featureId":"ft-0123456789abcdef","cycleId":"cy-0123456789abcdef","subState":"in_qa","movedFrom":{"projectId":"pj-00000000000000b1","projectKey":"OPS","number":7}
Label:      ,"groupId":"lb-00000000000000d1"
TeamPolicy: ,"estimateScale":"fibonacci","estimateZero":false,"staleAfterDays":30,"staleMode":"propose","archiveAfterDays":90
```

Block 5, the W2 records. Every list is `[]` on the wire, never `null` and never absent; every
optional scalar is a pointer, `null` when unset.

```go
// CustomState is a team's name for a place inside a human band (05 §3.3).
type CustomState struct {
	Key    string    `json:"key"` // ValidCustomStateKey; unique in the team
	TeamID string    `json:"teamId"`
	Name   string    `json:"name"` // 1-40 characters, shown on the row
	Band   StateBand `json:"band"`
	Order  int       `json:"order"` // within its band, from 0, gapless
}

// CustomStateSpec is one entry of PUT /v1/teams/{teamId}/states (decision (h)).
type CustomStateSpec struct {
	Key   string    `json:"key"`
	Name  string    `json:"name"`
	Band  StateBand `json:"band"`
	Order int       `json:"order"`
}

// Feature is a planned unit on its own branch (04 §2.5). Creating one reserves
// BranchRef by creating its branch brief in the same write.
type Feature struct {
	ID           string       `json:"id"` // ft-…
	ProjectID    string       `json:"projectId"`
	Name         string       `json:"name"`
	BranchRef    string       `json:"branchRef"` // FeatureBranchRef(slug)
	BriefID      string       `json:"briefId"`   // bf-…, the branch brief
	Lead         User         `json:"lead"`      // a member of the project's team
	TargetDate   *string      `json:"targetDate"`
	State        FeatureState `json:"state"`
	Priority     Priority     `json:"priority"`
	Labels       []string     `json:"labels"`
	InitiativeID *string      `json:"initiativeId"` // W4 (task 114); always null in W2
	CreatedAt    string       `json:"createdAt"`
	CreatedBy    User         `json:"createdBy"`
	UpdatedAt    string       `json:"updatedAt"`
}
```

```go
// Cycle is a team's time box. Dates are UTC calendar days, both inclusive;
// numbers are gapless per team from 1 (L-57-L-60).
type Cycle struct {
	ID       string     `json:"id"` // cy-…
	TeamID   string     `json:"teamId"`
	Number   int        `json:"number"`
	Name     *string    `json:"name"`
	StartsOn string     `json:"startsOn"`
	EndsOn   string     `json:"endsOn"`
	State    CycleState `json:"state"`
}

// Cadence is how the janitor makes a team's cycles. LengthDays 0 and a null
// UpdatedAt mean the team never set one: no cycles are made.
type Cadence struct {
	TeamID        string  `json:"teamId"`
	LengthDays    int     `json:"lengthDays"`   // 7-56
	CooldownDays  int     `json:"cooldownDays"` // 0-14; a pause is a gap (L-65)
	Ahead         int     `json:"ahead"`        // 1-15 upcoming cycles kept
	FirstStartsOn string  `json:"firstStartsOn"`
	UpdatedAt     *string `json:"updatedAt"`
	UpdatedBy     *User   `json:"updatedBy"`
}

// ScopeEntry is one line of a cycle's scope log (L-63). By is null when the
// janitor's rollover made the change (actor: null, 05 §3.6).
type ScopeEntry struct {
	CycleID string      `json:"cycleId"`
	IssueID string      `json:"issueId"`
	Change  ScopeChange `json:"change"`
	At      string      `json:"at"`
	By      *User       `json:"by"`
}

// ClosureProposal batches one assignee's stale issues onto their Desk (L-11,
// AX §7). Answering closes only the ids named, as the assignee.
type ClosureProposal struct {
	ID           string   `json:"id"` // opaque, no prefix (decision (g))
	Assignee     User     `json:"assignee"`
	IssueIDs     []string `json:"issueIds"`
	PolicyAuthor User     `json:"policyAuthor"` // who set staleAfterDays
	Reason       string   `json:"reason"`       // "untouched for 30 days"
	CreatedAt    string   `json:"createdAt"`
	AnsweredAt   *string  `json:"answeredAt"`
	Closed       []string `json:"closed"` // the ids the answer closed; [] before it
}

// CalendarToken is minted once and shown once; the feed carries cycle
// numbers, names and dates only (task 77; UNVERIFIED pending the Hub owner).
type CalendarToken struct {
	Token     string `json:"token"`
	FeedPath  string `json:"feedPath"` // "/cal/<token>.ics", outside /v1
	CreatedAt string `json:"createdAt"`
}

// Template pre-fills a new issue on the client; nothing links it to the issues
// made from it, so an edit changes none of them (L-26).
type Template struct {
	ID          string       `json:"id"`     // tp-…
	TeamID      *string      `json:"teamId"` // null: an org template
	Kind        TemplateKind `json:"kind"`
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Description AuthoredText `json:"description"`
	Serves      []Serve      `json:"serves"`
	Labels      []string     `json:"labels"`
	Priority    Priority     `json:"priority"`
	Estimate    *int         `json:"estimate"`
	ArchivedAt  *string      `json:"archivedAt"`
	CreatedAt   string       `json:"createdAt"`
	CreatedBy   User         `json:"createdBy"`
	UpdatedAt   string       `json:"updatedAt"`
}

// LabelGroup holds labels; an exclusive group allows one of them per issue (L-18).
type LabelGroup struct {
	ID        string  `json:"id"`     // lb-…, the label prefix (05 §3.6 lists no other)
	TeamID    *string `json:"teamId"` // null: an org group
	Name      string  `json:"name"`
	Exclusive bool    `json:"exclusive"`
	CreatedAt string  `json:"createdAt"`
}
```

```go
// View is a saved, shared list of a team's issues (L-93-L-95). Personal views
// stay desktop preferences (L-94) and never reach the Hub.
type View struct {
	ID            string      `json:"id"` // vw-…
	TeamID        string      `json:"teamId"`
	ProjectID     *string     `json:"projectId"` // null: every project of the team
	Name          string      `json:"name"`
	Filter        FilterGroup `json:"filter"`
	Layout        ViewLayout  `json:"layout"`
	GroupBy       string      `json:"groupBy"`    // ViewGroupBys
	SubGroupBy    string      `json:"subGroupBy"` // ViewGroupBys; "" when GroupBy is ""
	OrderBy       string      `json:"orderBy"`    // ViewOrderBys
	Display       []string    `json:"display"`    // ViewDisplayProps, in order
	IsTeamDefault bool        `json:"isTeamDefault"` // one per team; set by a team admin only
	ArchivedAt    *string     `json:"archivedAt"`
	CreatedAt     string      `json:"createdAt"`
	CreatedBy     User        `json:"createdBy"`
	UpdatedAt     string      `json:"updatedAt"`
}

// FilterGroup is a view's filter: terms and subgroups joined by Op, at most
// MaxFilterDepth deep. One Go evaluator (task 80) and one TypeScript mirror
// (tasks 83, 91) run the same shared vectors.
type FilterGroup struct {
	Op     FilterOp      `json:"op"`
	Terms  []FilterTerm  `json:"terms"`
	Groups []FilterGroup `json:"groups"`
}

type FilterTerm struct {
	Field  string       `json:"field"` // FilterFields
	Op     FilterTermOp `json:"op"`
	Values []string     `json:"values"` // ids, words or dates; [] for empty and not_empty
}

// Draft is private to its author: every other caller gets 404 (L-33, L-44).
// The janitor deletes it at ExpiresAt (six months after UpdatedAt, task 77).
type Draft struct {
	ID        string       `json:"id"` // df-…
	Kind      DraftKind    `json:"kind"`
	ProjectID *string      `json:"projectId"` // an issue draft's project; null for a comment draft
	IssueID   *string      `json:"issueId"`   // a comment draft's issue; null for an issue draft
	Title     string       `json:"title"`     // "" for a comment draft
	Body      AuthoredText `json:"body"`
	UpdatedAt string       `json:"updatedAt"`
	ExpiresAt string       `json:"expiresAt"`
}

// PromoteResult is POST /v1/issues/{issueId}/promote's answer: the new feature,
// and the issue's head with featureId set. The issue's revision is unchanged.
type PromoteResult struct {
	Feature Feature   `json:"feature"`
	Issue   IssueHead `json:"issue"`
}
```

Block 6, the request bodies and `Change[T]`. Requests may omit a field (`omitempty` only here);
a strict decoder refuses anything unnamed.

```go
// Change is a decoded tri-state field (decision (d)): Present false keeps,
// Null clears, otherwise Value sets.
type Change[T any] struct {
	Present bool
	Null    bool
	Value   T
}

// ChangeOf reads a RawMessage tri-state field.
func ChangeOf[T any](raw json.RawMessage) (Change[T], error) {
	if raw == nil {
		return Change[T]{}, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Change[T]{Present: true, Null: true}, nil
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return Change[T]{}, err
	}
	return Change[T]{Present: true, Value: v}, nil
}

// SetTo and ClearIt write a tri-state field; absent is the zero RawMessage.
func SetTo(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func ClearIt() json.RawMessage     { return json.RawMessage("null") }
```

The bodies, one per route. A `PUT` of one field replaces it, so there absent and `null` both
clear; a `PATCH` keeps what it does not name. The scope's "pointer fields" for
`PatchIssuePlanRequest` become `Change` fields by decision (d): a pointer cannot clear.

```go
// PatchIssuePlanRequest is PATCH /v1/issues/{issueId}/plan (task 79).
type PatchIssuePlanRequest struct {
	Estimate json.RawMessage `json:"estimate,omitempty"` // Change[int]; ValidEstimate
	DueOn    json.RawMessage `json:"dueOn,omitempty"`    // Change[string]; ValidDate
}

// One small body per aggregate field, so tasks 76-78 never share a handler.
type SetFeatureRequest struct {
	FeatureID *string `json:"featureId"` // ft-… of the same project; null leaves the feature
}
type SetCycleRequest struct {
	CycleID *string `json:"cycleId"` // cy-… of the team, not closed; null leaves the cycle
}
type SetParentRequest struct {
	ParentID *string `json:"parentId"` // is-… of the same project; null makes it top level
}
type SetSubStateRequest struct {
	Key *string `json:"key"` // a CustomState key whose band holds the head's state; null clears
}

// RankRequest names the neighbours; the server computes the key (L-14) and
// ignores any rank a client sends. After is the issue above, Before the one
// below; null is that end of the team's order. Non-adjacent or foreign
// neighbours are 422 rank_neighbours.
type RankRequest struct {
	Before *string `json:"before"`
	After  *string `json:"after"`
}

// MoveIssueRequest keeps the is- id and takes the target's next number (L-35).
type MoveIssueRequest struct {
	ToProjectID string `json:"toProjectId"`
}

// PromoteToFeatureRequest is POST /v1/issues/{issueId}/promote (task 76).
type PromoteToFeatureRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"` // FeatureBranchRef; an AU-only project refuses title words
}

// PutCustomStatesRequest replaces the team's list in one write (task 75).
type PutCustomStatesRequest struct {
	States []CustomStateSpec `json:"states"`
}

// PutCadenceRequest is PUT /v1/teams/{teamId}/cadence (task 77).
type PutCadenceRequest struct {
	LengthDays    int    `json:"lengthDays"`   // 7-56
	CooldownDays  int    `json:"cooldownDays"` // 0-14
	Ahead         int    `json:"ahead"`        // 1-15
	FirstStartsOn string `json:"firstStartsOn"` // ValidDate
}

// CycleQuery is GET /v1/teams/{teamId}/cycles?state=; a nil State lists all.
type CycleQuery struct {
	State *CycleState
}

type PatchCycleRequest struct {
	Name     json.RawMessage `json:"name,omitempty"` // Change[string]
	StartsOn *string         `json:"startsOn,omitempty"`
	EndsOn   *string         `json:"endsOn,omitempty"`
}

// AnswerClosureRequest closes exactly the ids named, as the assignee (L-11);
// an id not in the proposal is 422, and [] declines every one.
type AnswerClosureRequest struct {
	Close []string `json:"close"`
}
```

One create and one patch body per record. A patch's `Change` field is tri-state; its pointer
field cannot be cleared, because the record has no empty value for it.

```go
type CreateFeatureRequest struct {
	Name       string   `json:"name"`
	Slug       string   `json:"slug"`   // FeatureBranchRef
	LeadID     string   `json:"leadId"` // us-…, a member of the project's team
	TargetDate *string  `json:"targetDate,omitempty"`
	Priority   Priority `json:"priority"`
	Labels     []string `json:"labels"`
}

// PatchFeatureRequest sets what a person decides. A derived fact (live,
// landed, released) has no field, so a strict decoder answers 422 (task 76).
type PatchFeatureRequest struct {
	Name       *string         `json:"name,omitempty"`
	LeadID     *string         `json:"leadId,omitempty"`
	TargetDate json.RawMessage `json:"targetDate,omitempty"` // Change[string]
	State      *FeatureState   `json:"state,omitempty"`
	Priority   *Priority       `json:"priority,omitempty"`
	Labels     *[]string       `json:"labels,omitempty"` // a whole list
}

type CreateTemplateRequest struct {
	Kind        TemplateKind `json:"kind"`
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Description AuthoredText `json:"description"`
	Serves      []Serve      `json:"serves"`
	Labels      []string     `json:"labels"`
	Priority    Priority     `json:"priority"`
	Estimate    *int         `json:"estimate,omitempty"`
}

type PatchTemplateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Title       *string         `json:"title,omitempty"`
	Description *AuthoredText   `json:"description,omitempty"`
	Serves      *[]Serve        `json:"serves,omitempty"`
	Labels      *[]string       `json:"labels,omitempty"`
	Priority    *Priority       `json:"priority,omitempty"`
	Estimate    json.RawMessage `json:"estimate,omitempty"` // Change[int]
}

type CreateLabelGroupRequest struct {
	Name      string `json:"name"`
	Exclusive bool   `json:"exclusive"`
}

type CreateViewRequest struct {
	ProjectID  *string     `json:"projectId,omitempty"`
	Name       string      `json:"name"`
	Filter     FilterGroup `json:"filter"` // FilterGroup.Validate
	Layout     ViewLayout  `json:"layout"`
	GroupBy    string      `json:"groupBy"`
	SubGroupBy string      `json:"subGroupBy"`
	OrderBy    string      `json:"orderBy"`
	Display    []string    `json:"display"`
}

type PatchViewRequest struct {
	ProjectID  json.RawMessage `json:"projectId,omitempty"` // Change[string]
	Name       *string         `json:"name,omitempty"`
	Filter     *FilterGroup    `json:"filter,omitempty"`
	Layout     *ViewLayout     `json:"layout,omitempty"`
	GroupBy    *string         `json:"groupBy,omitempty"`
	SubGroupBy *string         `json:"subGroupBy,omitempty"`
	OrderBy    *string         `json:"orderBy,omitempty"`
	Display    *[]string       `json:"display,omitempty"`
}

// TeamDefaultRequest is PUT /v1/views/{viewId}/team-default (team admin,
// L-105). true clears the team's previous default in the same write.
type TeamDefaultRequest struct {
	IsTeamDefault bool `json:"isTeamDefault"`
}

// CreateDraftRequest names a project for an issue draft, an issue for a
// comment draft, and never both (422 otherwise).
type CreateDraftRequest struct {
	Kind      DraftKind    `json:"kind"`
	ProjectID *string      `json:"projectId,omitempty"`
	IssueID   *string      `json:"issueId,omitempty"`
	Title     string       `json:"title"`
	Body      AuthoredText `json:"body"`
}

// PutDraftRequest replaces a draft's text and moves ExpiresAt six months on.
type PutDraftRequest struct {
	Title string       `json:"title"`
	Body  AuthoredText `json:"body"`
}
```

---

#### `hubclient/work.go` ⚑, the `WorkPlan` delta, complete

Appended after `WorkIssues` in task 04's file. `Work` is **not** edited here: task 73 adds
`WorkPlan` to it in the same commit as the fake's implementation and the client stub, so no commit
between them fails `var _ hubclient.Work = hubclient.NewFake()`. Forty-six methods; the route of
each is in the table after the interface, and the route task builds it on the server.

```go
// WorkPlan is W2, planning (SW.18). Declared standalone by task 72; Work
// embeds it from task 73. Style of WorkIssues: ctx first, hubapi in and out.
type WorkPlan interface {
	// ── custom states: task 75 ──
	CustomStates(ctx context.Context, teamID string) ([]hubapi.CustomState, error)
	// PutCustomStates replaces the team's list; a key that issues hold and the
	// list drops is 409 state_in_use with the count.
	PutCustomStates(ctx context.Context, teamID string, req hubapi.PutCustomStatesRequest) ([]hubapi.CustomState, error)
	SetSubState(ctx context.Context, issueID string, req hubapi.SetSubStateRequest) (hubapi.IssueHead, error)

	// ── features: task 76 ──
	// CreateFeature reserves refs/heads/<slug> by creating its branch brief in
	// the same write; a second on that ref is 409 brief_exists.
	CreateFeature(ctx context.Context, projectID string, req hubapi.CreateFeatureRequest) (hubapi.Feature, error)
	Features(ctx context.Context, projectID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Feature], error)
	Feature(ctx context.Context, featureID string) (hubapi.Feature, error)
	PatchFeature(ctx context.Context, featureID string, req hubapi.PatchFeatureRequest) (hubapi.Feature, error)
	SetFeature(ctx context.Context, issueID string, req hubapi.SetFeatureRequest) (hubapi.IssueHead, error)
	// PromoteToFeature makes the ft- and its brief, with the issue as its first
	// child. The issue's revision and serves are untouched (L-23).
	PromoteToFeature(ctx context.Context, issueID string, req hubapi.PromoteToFeatureRequest) (hubapi.PromoteResult, error)

	// ── cycles, scope, proposals, calendar: task 77 ──
	Cadence(ctx context.Context, teamID string) (hubapi.Cadence, error)
	PutCadence(ctx context.Context, teamID string, req hubapi.PutCadenceRequest) (hubapi.Cadence, error)
	// Cycles lists a team's cycles by number, ascending.
	Cycles(ctx context.Context, teamID string, q hubapi.CycleQuery) ([]hubapi.Cycle, error)
	PatchCycle(ctx context.Context, cycleID string, req hubapi.PatchCycleRequest) (hubapi.Cycle, error)
	SetCycle(ctx context.Context, issueID string, req hubapi.SetCycleRequest) (hubapi.IssueHead, error)
	// ScopeLog is oldest first; By is nil where the janitor made the change.
	ScopeLog(ctx context.Context, cycleID string, page hubapi.PageRequest) (hubapi.Page[hubapi.ScopeEntry], error)
	// ClosureProposals is the caller's own, unanswered, newest first.
	ClosureProposals(ctx context.Context) ([]hubapi.ClosureProposal, error)
	AnswerClosure(ctx context.Context, proposalID string, req hubapi.AnswerClosureRequest) (hubapi.ClosureProposal, error)
	// MintCalendarToken replaces the caller's token; the old feed is 404 at once.
	MintCalendarToken(ctx context.Context) (hubapi.CalendarToken, error)
	RevokeCalendarToken(ctx context.Context) error
	// CyclesICS is the bearer-only fallback download (text/calendar).
	CyclesICS(ctx context.Context, teamID string) ([]byte, error)

	// ── parents, templates, label groups, org labels: task 78 ──
	SetParent(ctx context.Context, issueID string, req hubapi.SetParentRequest) (hubapi.IssueHead, error)
	// Templates is the team's and the org's, live ones only, by name.
	Templates(ctx context.Context, teamID string) ([]hubapi.Template, error)
	CreateTemplate(ctx context.Context, teamID string, req hubapi.CreateTemplateRequest) (hubapi.Template, error)
	CreateOrgTemplate(ctx context.Context, req hubapi.CreateTemplateRequest) (hubapi.Template, error)
	PatchTemplate(ctx context.Context, templateID string, req hubapi.PatchTemplateRequest) (hubapi.Template, error)
	ArchiveTemplate(ctx context.Context, templateID string) (hubapi.Template, error)
	// LabelGroups is the team's and the org's, by name.
	LabelGroups(ctx context.Context, teamID string) ([]hubapi.LabelGroup, error)
	CreateLabelGroup(ctx context.Context, teamID string, req hubapi.CreateLabelGroupRequest) (hubapi.LabelGroup, error)
	CreateOrgLabelGroup(ctx context.Context, req hubapi.CreateLabelGroupRequest) (hubapi.LabelGroup, error)
	CreateOrgLabel(ctx context.Context, req hubapi.CreateLabelRequest) (hubapi.Label, error)

	// ── estimates, due dates, rank, move: task 79 ──
	PatchIssuePlan(ctx context.Context, issueID string, req hubapi.PatchIssuePlanRequest) (hubapi.IssueHead, error)
	// RankIssue, not Rank: IssueHead.Rank is the field it writes.
	RankIssue(ctx context.Context, issueID string, req hubapi.RankRequest) (hubapi.IssueHead, error)
	// MoveIssue needs membership of both teams; the old alias keeps resolving
	// (IssueByNumber answers the moved head with MovedFrom set).
	MoveIssue(ctx context.Context, issueID string, req hubapi.MoveIssueRequest) (hubapi.IssueHead, error)

	// ── views, drafts, export: task 80 ──
	CreateView(ctx context.Context, teamID string, req hubapi.CreateViewRequest) (hubapi.View, error)
	Views(ctx context.Context, teamID string) ([]hubapi.View, error)
	View(ctx context.Context, viewID string) (hubapi.View, error)
	PatchView(ctx context.Context, viewID string, req hubapi.PatchViewRequest) (hubapi.View, error)
	ArchiveView(ctx context.Context, viewID string) (hubapi.View, error)
	SetTeamDefault(ctx context.Context, viewID string, req hubapi.TeamDefaultRequest) (hubapi.View, error)
	// Drafts are the caller's only; another person's draft is 404 draft_not_found.
	CreateDraft(ctx context.Context, req hubapi.CreateDraftRequest) (hubapi.Draft, error)
	Drafts(ctx context.Context) ([]hubapi.Draft, error)
	Draft(ctx context.Context, draftID string) (hubapi.Draft, error)
	PutDraft(ctx context.Context, draftID string, req hubapi.PutDraftRequest) (hubapi.Draft, error)
	DeleteDraft(ctx context.Context, draftID string) error
	// ExportIssuesCSV is text/csv over readable rows only; viewID nil is every issue.
	ExportIssuesCSV(ctx context.Context, projectID string, viewID *string) ([]byte, error)
	ExportFeaturesCSV(ctx context.Context, projectID string) ([]byte, error)
}
```

The routes, one row per method; task 82's `WorkPlanRoutes` table is these rows. Roles: **V** viewer,
**M** member, **TA** team admin, **OA** org admin, **A** the assignee, **Au** the author.

| Method | Route | Task | Writes |
|---|---|---|---|
| `CustomStates` / `PutCustomStates` | `GET` / `PUT /v1/teams/{teamId}/states` | 75 | V / TA |
| `SetSubState` | `PUT /v1/issues/{issueId}/sub-state` | 75 | M |
| `CreateFeature` / `Features` | `POST` / `GET /v1/projects/{projectId}/features` | 76 | M / V |
| `Feature` / `PatchFeature` | `GET` / `PATCH /v1/features/{featureId}` | 76 | V / M |
| `SetFeature` | `PUT /v1/issues/{issueId}/feature` | 76 | M |
| `PromoteToFeature` | `POST /v1/issues/{issueId}/promote` | 76 | M, a person |
| `Cadence` / `PutCadence` | `GET` / `PUT /v1/teams/{teamId}/cadence` | 77 | V / TA |
| `Cycles` | `GET /v1/teams/{teamId}/cycles?state=` | 77 | V |
| `PatchCycle` | `PATCH /v1/cycles/{cycleId}` | 77 | TA |
| `SetCycle` | `PUT /v1/issues/{issueId}/cycle` | 77 | M |
| `ScopeLog` | `GET /v1/cycles/{cycleId}/scope-log` | 77 | V |
| `ClosureProposals` | `GET /v1/closure-proposals?mine=true` | 77 | the caller's |
| `AnswerClosure` | `POST /v1/closure-proposals/{proposalId}/answer` | 77 | A |
| `MintCalendarToken` / `RevokeCalendarToken` | `POST` / `DELETE /v1/me/calendar-token` | 77 | the caller |
| `CyclesICS` | `GET /v1/teams/{teamId}/cycles.ics` | 77 | V |
| `SetParent` | `PUT /v1/issues/{issueId}/parent` | 78 | M |
| `Templates` / `CreateTemplate` | `GET` / `POST /v1/teams/{teamId}/templates` | 78 | V / TA |
| `CreateOrgTemplate` | `POST /v1/org/templates` | 78 | OA |
| `PatchTemplate` / `ArchiveTemplate` | `PATCH /v1/templates/{templateId}`, `POST …/archive` | 78 | TA; OA for an org one |
| `LabelGroups` / `CreateLabelGroup` | `GET` / `POST /v1/teams/{teamId}/label-groups` | 78 | V / TA |
| `CreateOrgLabelGroup` / `CreateOrgLabel` | `POST /v1/org/label-groups`, `POST /v1/org/labels` | 78 | OA |
| `PatchIssuePlan` | `PATCH /v1/issues/{issueId}/plan` | 79 | M |
| `RankIssue` | `POST /v1/issues/{issueId}/rank` | 79 | M |
| `MoveIssue` | `POST /v1/issues/{issueId}/move` | 79 | M of both teams |
| `CreateView` / `Views` | `POST` / `GET /v1/teams/{teamId}/views` | 80 | M / V |
| `View` / `PatchView` / `ArchiveView` | `GET` / `PATCH /v1/views/{viewId}`, `POST …/archive` | 80 | V / M / M |
| `SetTeamDefault` | `PUT /v1/views/{viewId}/team-default` | 80 | TA |
| `CreateDraft` / `Drafts` | `POST` / `GET /v1/me/drafts` | 80 | Au |
| `Draft` / `PutDraft` / `DeleteDraft` | `GET` / `PUT` / `DELETE /v1/me/drafts/{draftId}` | 80 | Au |
| `ExportIssuesCSV` / `ExportFeaturesCSV` | `GET /v1/projects/{projectId}/issues.csv?viewId=`, `…/features.csv` | 80 | V |

Two routes have no method, because no Go caller exists: the calendar feed `GET /cal/{token}.ics`
and the read-only page `GET /p/issues/{issueId}[@{rev}]` (task 80), both outside `/v1` and both
**UNVERIFIED pending the Hub owner's review**. The sync principal writes none of these rows (task
74's matrix). `GET /v1/teams/{teamId}/cadence` is not in task 77's scope; it is added here because
task 87's settings screen must read the cadence it edits (a gap, reported).

---

#### Tests first: `work_plan_test.go`, complete, in three blocks

Written before `work_plan.go` and run red (`undefined: Feature` and the rest), then green. The
helpers `exampleJSON` (`TIME`, `USER`), `jsonKeyPaths`, `decodeAny`, `jsonFields`, `decodeInto` and
`problemOf` are task 03's and `types_test.go`'s, in the same package. Each test names its mutant.

Block T1, the examples: one per W2 record and body, every field present, every list non-empty.

```go
package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const filterEx = `{"op":"and","terms":[{"field":"state","op":"is","values":["todo"]}],"groups":[{"op":"or","terms":[{"field":"dueOn","op":"before","values":["2026-10-02"]}],"groups":[]}]}`

var workPlanExamples = []struct {
	label  string
	target func() any
	json   string
}{
	{"CustomState", func() any { return new(CustomState) }, `{"key":"in_qa","teamId":"tm-0123456789abcdef","name":"In QA","band":"after_landed","order":0}`},
	{"Feature", func() any { return new(Feature) }, `{"id":"ft-0123456789abcdef","projectId":"pj-0123456789abcdef","name":"Resume","branchRef":"refs/heads/resume","briefId":"bf-0123456789abcdef","lead":USER,"targetDate":"2026-10-30","state":"active","priority":"high","labels":["lb-0123456789abcdef"],"initiativeId":null,"createdAt":TIME,"createdBy":USER,"updatedAt":TIME}`},
	{"Cycle", func() any { return new(Cycle) }, `{"id":"cy-0123456789abcdef","teamId":"tm-0123456789abcdef","number":4,"name":"Hardening","startsOn":"2026-10-05","endsOn":"2026-10-18","state":"upcoming"}`},
	{"Cadence", func() any { return new(Cadence) }, `{"teamId":"tm-0123456789abcdef","lengthDays":14,"cooldownDays":0,"ahead":3,"firstStartsOn":"2026-10-05","updatedAt":TIME,"updatedBy":USER}`},
	{"ScopeEntry", func() any { return new(ScopeEntry) }, `{"cycleId":"cy-0123456789abcdef","issueId":"is-3f2a91c07b1e4d2a","change":"added","at":TIME,"by":USER}`},
	{"ClosureProposal", func() any { return new(ClosureProposal) }, `{"id":"cp9f2","assignee":USER,"issueIds":["is-3f2a91c07b1e4d2a"],"policyAuthor":USER,"reason":"untouched for 30 days","createdAt":TIME,"answeredAt":TIME,"closed":["is-3f2a91c07b1e4d2a"]}`},
	{"CalendarToken", func() any { return new(CalendarToken) }, `{"token":"k3v","feedPath":"/cal/k3v.ics","createdAt":TIME}`},
	{"Template", func() any { return new(Template) }, `{"id":"tp-0123456789abcdef","teamId":null,"kind":"issue","name":"Bug","title":"Bug: ","description":{"text":"Steps","author":"H"},"serves":[{"criterionId":"cr-0123456789abcdef","briefKey":"brief:bf-0123456789abcdef@6"}],"labels":["lb-0123456789abcdef"],"priority":"medium","estimate":2,"archivedAt":null,"createdAt":TIME,"createdBy":USER,"updatedAt":TIME}`},
	{"LabelGroup", func() any { return new(LabelGroup) }, `{"id":"lb-00000000000000d1","teamId":"tm-0123456789abcdef","name":"Area","exclusive":true,"createdAt":TIME}`},
	{"View", func() any { return new(View) }, `{"id":"vw-0123456789abcdef","teamId":"tm-0123456789abcdef","projectId":null,"name":"Due soon","filter":` + filterEx + `,"layout":"board","groupBy":"state","subGroupBy":"assignee","orderBy":"dueOn","display":["alias","dueOn"],"isTeamDefault":false,"archivedAt":null,"createdAt":TIME,"createdBy":USER,"updatedAt":TIME}`},
	{"Draft", func() any { return new(Draft) }, `{"id":"df-0123456789abcdef","kind":"issue","projectId":"pj-0123456789abcdef","issueId":null,"title":"Half an idea","body":{"text":"…","author":"H"},"updatedAt":TIME,"expiresAt":TIME}`},
	{"PutCadenceRequest", func() any { return new(PutCadenceRequest) }, `{"lengthDays":14,"cooldownDays":0,"ahead":3,"firstStartsOn":"2026-10-05"}`},
	{"RankRequest", func() any { return new(RankRequest) }, `{"before":"is-3f2a91c07b1e4d2a","after":null}`},
	{"CreateViewRequest", func() any { return new(CreateViewRequest) }, `{"projectId":"pj-0123456789abcdef","name":"Due soon","filter":` + filterEx + `,"layout":"list","groupBy":"","subGroupBy":"","orderBy":"rank","display":["alias"]}`},
}
```

The `Serve` example's keys are task 02's (`task-02.md:239-256`) and are **UNVERIFIED** until merge;
step 1 corrects them to the merged names.

Block T2, `types_test.go`'s four styles over the W2 types.

```go
var workPlanResponseTypes = []any{CustomState{}, Feature{}, Cycle{}, Cadence{}, ScopeEntry{},
	ClosureProposal{}, CalendarToken{}, Template{}, LabelGroup{}, View{}, FilterGroup{},
	FilterTerm{}, Draft{}, PromoteResult{}, IssueAlias{}, IssuePlan{}, TeamPlanPolicy{}, LabelPlan{}}

var workPlanRequestTypes = []any{PatchIssuePlanRequest{}, SetFeatureRequest{}, SetCycleRequest{},
	SetParentRequest{}, SetSubStateRequest{}, RankRequest{}, MoveIssueRequest{},
	PromoteToFeatureRequest{}, PutCustomStatesRequest{}, CustomStateSpec{}, PutCadenceRequest{},
	PatchCycleRequest{}, AnswerClosureRequest{}, CreateFeatureRequest{}, PatchFeatureRequest{},
	CreateTemplateRequest{}, PatchTemplateRequest{}, CreateLabelGroupRequest{}, CreateViewRequest{},
	PatchViewRequest{}, TeamDefaultRequest{}, CreateDraftRequest{}, PutDraftRequest{}}

// Mutant: a tag spelt due_on, branch_ref or is_team_default, or a field dropped.
func TestWorkPlanFieldNamesMatchTheExamples(t *testing.T) {
	for _, ex := range workPlanExamples {
		t.Run(ex.label, func(t *testing.T) {
			in := []byte(exampleJSON(ex.json))
			dst := ex.target()
			dec := json.NewDecoder(bytes.NewReader(in))
			dec.DisallowUnknownFields()
			if err := dec.Decode(dst); err != nil {
				t.Fatalf("the example does not decode into %T: %v", dst, err)
			}
			out, _ := json.Marshal(dst)
			if got, want := jsonKeyPaths(decodeAny(t, out), nil), jsonKeyPaths(decodeAny(t, in), nil); !slices.Equal(got, want) {
				t.Errorf("%T round trip:\n got keys %v\nwant keys %v", dst, got, want)
			}
		})
	}
}

// Mutant: omitempty on a record; the desktop reads a missing dueOn as undefined.
func TestWorkPlanResponsesNeverOmitAField(t *testing.T) {
	for _, v := range workPlanResponseTypes {
		typ := reflect.TypeOf(v)
		for _, f := range jsonFields(typ) {
			if f.omitempty {
				t.Errorf("%s.%s has omitempty; a response never omits a field", typ.Name(), f.name)
			}
		}
	}
}

// Mutant: a list that re-encodes as null. Decoded [] must come back [].
func TestWorkPlanEmptyListsStayArrays(t *testing.T) {
	for _, tc := range []struct {
		v    any
		in   string
		keys []string
	}{
		{new(Feature), `{"labels":[]}`, []string{"labels"}},
		{new(ClosureProposal), `{"issueIds":[],"closed":[]}`, []string{"issueIds", "closed"}},
		{new(Template), `{"serves":[],"labels":[]}`, []string{"serves", "labels"}},
		{new(View), `{"display":[]}`, []string{"display"}},
		{new(FilterGroup), `{"op":"and","terms":[],"groups":[]}`, []string{"terms", "groups"}},
		{new(FilterTerm), `{"values":[]}`, []string{"values"}},
	} {
		if err := json.Unmarshal([]byte(tc.in), tc.v); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(tc.v)
		var m map[string]json.RawMessage
		_ = json.Unmarshal(b, &m)
		for _, k := range tc.keys {
			if raw, ok := m[k]; !ok || string(raw) != "[]" {
				t.Errorf("%T.%s re-encodes as %q, want []", tc.v, k, raw)
			}
		}
	}
}

// Mutants: a lenient decoder; a client-sent rank; a derived fact stored on a
// feature; a custom state naming another team; a draft naming its author.
func TestWorkPlanRequestsRejectUnknownFields(t *testing.T) {
	extra := map[string]string{"RankRequest": `{"rank":"i"}`, "PatchFeatureRequest": `{"released":true}`,
		"CustomStateSpec": `{"teamId":"tm-0123456789abcdef"}`, "CreateDraftRequest": `{"authorId":"us-1"}`}
	for _, v := range workPlanRequestTypes {
		typ := reflect.TypeOf(v)
		for _, body := range []string{`{"bogus":1}`, extra[typ.Name()]} {
			if body == "" {
				continue
			}
			rec, ok := decodeInto(reflect.New(typ).Interface(), body)
			if ok || rec.Code != http.StatusBadRequest || problemOf(t, rec).Error != CodeBadRequest {
				t.Errorf("%s with %s: ok=%v %d, want 400 bad_request", typ.Name(), body, ok, rec.Code)
			}
		}
	}
}
```

Block T3, the validators and the revision guard.

```go
// Mutant: a custom state inside the fact band (started) or a band unchecked.
func TestStateBandsAreTheHumanBandsOnly(t *testing.T) {
	if ValidStateBand("started") || ValidStateBand("landed") || !ValidStateBand(BandAfterLanded) {
		t.Fatal("only before_started and after_landed are bands")
	}
	for _, s := range IssueStates {
		want := slices.Contains(InitialStates, s)
		if got := BandHolds(BandBeforeStarted, s); got != want {
			t.Errorf("before_started on %s = %v, want %v", s, got, want)
		}
		if got := BandHolds(BandAfterLanded, s); got != (s == StateLanded) {
			t.Errorf("after_landed on %s = %v", s, got)
		}
	}
}

// Mutant: a custom state key that renames a fact state.
func TestCustomStateKeyRefusesStateNames(t *testing.T) {
	for _, s := range IssueStates {
		if ValidCustomStateKey(string(s)) {
			t.Errorf("key %q shadows a state", s)
		}
	}
	for k, want := range map[string]bool{"in_qa": true, "In QA": false, "9lives": false, "": false} {
		if ValidCustomStateKey(k) != want {
			t.Errorf("ValidCustomStateKey(%q) != %v", k, want)
		}
	}
}

// Mutant: unbounded nesting (a fourth level accepted), or depth counted from 0.
func TestFilterGroupDepthIsThree(t *testing.T) {
	leaf := FilterGroup{Op: FilterAnd, Terms: []FilterTerm{}, Groups: []FilterGroup{}}
	wrap := func(g FilterGroup) FilterGroup { return FilterGroup{Op: FilterOr, Terms: []FilterTerm{}, Groups: []FilterGroup{g}} }
	if err := wrap(wrap(leaf)).Validate(); err != nil {
		t.Fatalf("three deep: %v", err)
	}
	if err := wrap(wrap(wrap(leaf))).Validate(); !errors.Is(err, ErrFilterTooDeep) {
		t.Fatalf("four deep: %v, want ErrFilterTooDeep", err)
	}
}

// Mutants: an unknown field; values on empty; before on a non-date field.
func TestFilterTermRules(t *testing.T) {
	for _, tc := range []struct {
		term FilterTerm
		ok   bool
	}{
		{FilterTerm{"state", TermIs, []string{"todo"}}, true},
		{FilterTerm{"title", TermIs, []string{"x"}}, false},
		{FilterTerm{"cycle", TermEmpty, []string{}}, true},
		{FilterTerm{"cycle", TermEmpty, []string{"cy-1"}}, false},
		{FilterTerm{"state", TermIs, []string{}}, false},
		{FilterTerm{"dueOn", TermBefore, []string{"2026-10-02"}}, true},
		{FilterTerm{"priority", TermBefore, []string{"2026-10-02"}}, false},
		{FilterTerm{"dueOn", TermAfter, []string{"2026-10-02T09:00:00Z"}}, false},
	} {
		g := FilterGroup{Op: FilterAnd, Terms: []FilterTerm{tc.term}, Groups: []FilterGroup{}}
		if err := g.Validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err %v, want ok %v", tc.term, err, tc.ok)
		}
	}
}

// Mutants: rank outside [0-9a-z]{1,32}; a due date with a time; zero always allowed.
func TestRankDateAndEstimate(t *testing.T) {
	for s, want := range map[string]bool{"i": true, "0z9": true, "": false, "I": false, "a-b": false, strings.Repeat("a", 33): false} {
		if ValidRank(s) != want {
			t.Errorf("ValidRank(%q) != %v", s, want)
		}
	}
	for s, want := range map[string]bool{"2026-10-02": true, "2026-02-30": false, "2026-10-02T00:00:00Z": false, "2026-10-2": false} {
		if ValidDate(s) != want {
			t.Errorf("ValidDate(%q) != %v", s, want)
		}
	}
	fib := TeamPlanPolicy{EstimateScale: EstimateFibonacci}
	if ValidEstimate(fib, 4) || !ValidEstimate(fib, 5) || ValidEstimate(fib, 0) {
		t.Error("fibonacci takes 1 2 3 5 8, zero only with estimateZero")
	}
	fib.EstimateZero = true
	if !ValidEstimate(fib, 0) || ValidEstimate(TeamPlanPolicy{EstimateScale: EstimateNone}, 1) {
		t.Error("estimateZero admits 0; scale none refuses all")
	}
}
```

```go
// Mutant: a *int in place of Change, so null and absent both keep.
func TestChangeOfIsTriState(t *testing.T) {
	var r PatchIssuePlanRequest
	_ = json.Unmarshal([]byte(`{"estimate":null,"dueOn":"2026-10-02"}`), &r)
	est, _ := ChangeOf[int](r.Estimate)
	due, _ := ChangeOf[string](r.DueOn)
	keep, _ := ChangeOf[int](nil)
	if !est.Present || !est.Null || !due.Present || due.Value != "2026-10-02" || keep.Present {
		t.Fatalf("estimate %+v, dueOn %+v, absent %+v", est, due, keep)
	}
}

// Mutants: a code with no status; a 404 code not spelt *_not_found.
func TestWorkPlanCodeStatus(t *testing.T) {
	for code, st := range WorkPlanCodeStatus {
		if (st == http.StatusNotFound) != strings.HasSuffix(code, "_not_found") {
			t.Errorf("%s is %d", code, st)
		}
	}
	if len(WorkPlanCodeStatus) != 19 {
		t.Errorf("%d codes have a status, want the 19 of block 3", len(WorkPlanCodeStatus))
	}
}

// Mutants: a slug with a slash or capitals; a ref the Hub's rules refuse.
func TestFeatureBranchRef(t *testing.T) {
	if ref, ok := FeatureBranchRef("resume-reads"); !ok || ref != "refs/heads/resume-reads" {
		t.Fatalf("%q %v", ref, ok)
	}
	for _, bad := range []string{"", "a/b", "Resume", "-x", "a..b", strings.Repeat("a", 64)} {
		if _, ok := FeatureBranchRef(bad); ok {
			t.Errorf("slug %q accepted", bad)
		}
	}
}

// Mutant: a plan field added to the revision's content, which would re-hash
// every revision and break task 02's golden vectors.
func TestIssuePlanStaysOutOfTheRevision(t *testing.T) {
	var content []string
	for _, f := range jsonFields(reflect.TypeOf(IssueRevisionContent{})) {
		content = append(content, f.name)
	}
	for _, f := range jsonFields(reflect.TypeOf(IssuePlan{})) {
		if slices.Contains(content, f.name) {
			t.Errorf("%s is in the revision's canonical form", f.name)
		}
	}
}
```

Mutants and the test that kills each:

| Mutant | Killed by |
|---|---|
| `json:"due_on"` or `json:"branch_ref"` | `TestWorkPlanFieldNamesMatchTheExamples` |
| `omitempty` on a record field | `TestWorkPlanResponsesNeverOmitAField` |
| `labels` re-encoding as `null` | `TestWorkPlanEmptyListsStayArrays` |
| `RankRequest` accepting `rank` from a client | `TestWorkPlanRequestsRejectUnknownFields` |
| `ValidStateBand("started")` true | `TestStateBandsAreTheHumanBandsOnly` |
| a custom key `landed` | `TestCustomStateKeyRefusesStateNames` |
| a fourth filter level accepted | `TestFilterGroupDepthIsThree` |
| `before` on `priority` | `TestFilterTermRules` |
| rank `I`, a due date with a time, zero always valid | `TestRankDateAndEstimate` |
| `Estimate *int` | `TestChangeOfIsTriState` |
| `CodeHeldBySubState` with no status | `TestWorkPlanCodeStatus` |
| `rank` inside `IssueRevisionContent` | `TestIssuePlanStaysOutOfTheRevision`, `TestIssueRevisionGoldenVectors` |

#### Steps

1. **Sync.** Sync-merge `feat/hub` into `feat/work-layer` (rule H1); confirm tasks 02-04 are merged
   and that their names match the anchors above (`workids.go`, `work.go`, `hubclient/work.go`).
   Correct any name here that differs, in this part's code before you type it.
2. **Red.** Write `work_plan_test.go` (blocks T1-T3). Run the test command below; expect a build
   failure naming `Feature`, `WorkPlanCodeStatus` and the rest.
3. **Green, types.** Write `work_plan.go` blocks 1-6. Add the five embedded lines to `work.go` and
   the three example suffixes to `work_test.go`.
4. **Green, the interface.** Append `WorkPlan` to `hubclient/work.go`. Leave `Work` unchanged.
5. **Run** every acceptance command; commit `work_plan.go`, `work_plan_test.go`, `work.go`,
   `work_test.go` and `hubclient/work.go` in one commit.

#### Acceptance

```bash
go build ./control-plane/... && go vet ./control-plane/internal/hub/hubapi/ ./control-plane/internal/hub/hubclient/
# (no output)
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v -run 'TestWorkPlan|TestStateBands|TestCustomStateKey|TestFilter|TestRankDate|TestChangeOf|TestFeatureBranchRef|TestIssuePlanStays' 2>&1 | grep -c '^--- PASS'
# 13   (top-level tests only; the examples' subtests print indented)
go test ./control-plane/internal/hub/hubapi/ -count=1 -run 'TestIssueRevisionGoldenVectors|TestWorkFieldNamesMatchTheExamples' -v 2>&1 | grep -c '^--- PASS'
# 2    (W1's golden vectors byte for byte; the three W1 rows with their W2 keys)
go test ./control-plane/internal/hub/... -race -count=1
# ok for every package; the fake and the client still satisfy Work, which is unchanged
grep -c '^	[A-Z][A-Za-z]*(ctx context.Context' control-plane/internal/hub/hubclient/work.go
# 76   (task 04's 30 WorkIssues methods and this task's 46 WorkPlan methods)
git diff --numstat START -- control-plane/internal/hub/hubapi/work.go control-plane/internal/hub/hubapi/workids.go
# 5	0	control-plane/internal/hub/hubapi/work.go   (workids.go does not appear)
```

`START` is the task's start commit. The count of 76 assumes W1 added no `WorkIssues` method
after task 04; step 1 re-counts it on the merged tree and corrects the expected figure.

**What this task does not claim.**

- Any rule's behaviour. The validators are pure functions; the fake's rules and the conformance
  rows are task 73's, the database guards task 74's, and the routes tasks 75-80's.
- That `hubclient.Work` includes `WorkPlan`. It does from task 73.
- The HTTP client's W2 methods (task 82) or the desktop's mirror (task 83).
- Recurring schedules on templates (W4, task 121), initiatives (W4; `initiativeId` is always
  `null` in W2), brief templates (W4, L-122) or form templates (W5, L-27).
- That the calendar feed and the read-only page live outside `/v1`. Both are proposals the Hub
  owner has not reviewed (**UNVERIFIED**, tasks 77 and 80).

**Parity rows served** (each closed by a route task and its desktop surface): L-8's W2 half, L-11,
L-12, L-14, L-16-L-21, L-23, L-25, L-26, L-33, L-35, L-37, L-44, L-57-L-61, L-63-L-65, L-69-L-71,
L-93-L-95, L-99, L-103, L-105 and L-153's W2 half. This task closes none of them alone.

**Corrections to the scope paragraph** (the skeleton row wins; this part records them):
`Label.teamId` stays a string, `""` for an org label (decision (c)); `PatchIssuePlanRequest`'s
fields are `Change` fields, not pointers (d); `IssueAlias` carries `projectId` beside the key and
number, because a project with no key would give an alias that names no project; `CustomState`
is written through `CustomStateSpec` (h); `ClosureProposal` gains `answeredAt` and `closed`, so
the Desk can show an answered one without a second read; `Feature`, `Template` and `View` gain
`createdAt`, `createdBy`, `updatedAt` and, where they archive, `archivedAt`; `Draft` gains
`projectId` and `title` for an issue draft; `Cadence.updatedAt` and `updatedBy` are nullable, for a
team that never set one; and three methods beyond the scope's routes: `Cadence` (read),
`MintCalendarToken`/`RevokeCalendarToken`, and `CyclesICS`.

**Gaps found while writing this part** (for the skeleton's owner and the named tasks):

1. **A closure proposal has no id prefix** (decision (g)). Task 02's twelve prefixes are frozen;
   if the owner wants `cp-`, it is an added prefix, which SW.18's freeze allows.
2. **`GET /v1/teams/{teamId}/cadence` is in no scope.** Task 77's route list lacks it; task 87
   needs it. Added here.
3. **The calendar token needs a table** (`calendar_tokens`), which task 74's scope does not list.
   Task 74's part adds it.
4. **`Labels(teamID)` must return the org's labels too** from task 78, a behaviour change to a W1
   method. Task 78's part must say so, and task 05's conformance row for `Labels` must accept rows
   with `teamId: ""`.
5. **Task 03's `workExamples` rows change**, so task 03's test and this task's commit must land
   together; the three suffixes above are the only edits.
