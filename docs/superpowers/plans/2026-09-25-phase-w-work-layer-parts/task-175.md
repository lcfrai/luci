### Task 175: *Copy from team…* on templates, views and policies (L-7)

Stream **WL** (Hub and desktop), about 2.5 hours (an estimate, not a measurement). Depends on task
**174**, and through it on tasks 11 (team policy), 75 (custom states inside the human bands), 78
(templates `tp-`), 80 (views `vw-`), 89 (the templates list) and 91 (the views list). 05 L-7
(`05-design.md:111`, verified 2026-09-26): *"'Copy from team…' on templates, views and policies"*,
slice `later`. Hub anchors were read in the `feat/hub` worktree at `54b5e746` (cited
`hub-contract:N`). `server/` and `store/` are **planned, not on `feat/hub`**, and parts 75, 78, 80,
89 and 91 are **not written** on 2026-09-26, so every name taken from them here comes from their
scope paragraphs and is **UNVERIFIED** until they land (rule H1); a different spelling there is a
find-and-replace here.

A team that has tuned a template, a view or its work policy should not rebuild it by hand in the
next team. This task copies one record from a team the person can read into a team they can
write, as a **new, independent record**, and says out loud what it could not bring.

**Decisions made here, so nobody re-derives them:**

1. **A copy, never a reference.** The target gets a new id in the source kind's prefix (`tp-`,
   `vw-`) or, for a policy, a new policy version on the target team. The copy's creation event
   carries `data.copiedFromTeamId` and `data.copiedFromId`; nothing stores a link from which a later
   source edit could flow. Task 78 already makes a template a client prefill with no live link
   (L-26), so this is the same rule one level up.
2. **Names are matched; nothing is created.** A label is matched by name (case-insensitive, live
   labels only) in the target team or the org's labels (task 78); a label group by name; a
   template a view filters by, by name; a custom sub-state (task 75) by key first, then by its
   **band** (the clause widens to the band, and `dropped` says so). Anything unmatched is left out and listed
   in `dropped` with its reason. The copy never creates a label, group, state or template in the
   target: a copy that invents vocabulary in someone else's team is a write they did not make.
3. **The thread is never carried across teams.** A template's pre-filled `serves` names criteria
   of a brief in the source team's project. Those are always dropped (`reason:
   "criterion_other_team"`), because a criterion belongs to one brief on one thread; the copied
   template asks for its criteria in the target, where the picker (task 42) offers them.
4. **Who may copy.** The caller must **read** the source: a source outside the caller's readable
   set is 404 (`hub-contract:273-275`: a 403 would confirm the id exists). The caller must **write**
   the target: a `member` for templates and views, a team `admin` for a policy (a policy write is
   admin-only in task 11's `putWorkPolicy`). An org admin with no team role reads no project
   content (`hub-contract` §5.2) and gets no special path here.
5. **Personal views are not in the Hub.** Task 80 and task 91 keep a personal view as a desktop
   preference that is *"never sent to the Hub"*. So the Hub copies **shared team views** only, and
   the scope's rule *"a personal view is copied only by its owner"* holds by construction: nobody
   else's personal view is reachable. The dialog offers the person's own personal views as a
   local copy (a preference write, no route), with the same matching and `dropped` list.
6. **Idempotency is the header.** The scope's body `requestId` is the Hub's `Idempotency-Key`
   header (`hub-contract:50`): a double press sends one key, and the second call replays the first
   reply. The body carries no `requestId`.
7. **One write, one event.** A copy is one transaction and exactly one event on the target:
   `template.created`, `view.created` or `team.work_policy_set`, the source kind's own create event
   with the two `copiedFrom` fields added. There is no `copy.*` event kind.

**Files:**
- `control-plane/internal/hub/hubapi/work.go` ⚑ — `CopyKind`, `CopyRequest`, `CopyReply`,
  `DroppedField`, the reasons (additive)
- `control-plane/internal/hub/hubapi/work_copy.go` — the pure matcher `PlanCopy` both the Hub and
  the fake call (new)
- `control-plane/internal/hub/hubapi/work_copy_test.go` — the pure rules (new)
- `control-plane/internal/hub/store/work_copy.go`, `server/work_copy.go` — the route (new)
- `control-plane/internal/hub/server/work_copy_integration_test.go` — `-tags integration` (new)
- `control-plane/internal/hub/server/work_matrix_rows.go` ⚑ — one row (task 07's file)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ — the route in the fake, calling `PlanCopy`
- `desktop/src/api/work.ts` ⚑ — the copy types and `copyFromTeam` (task 37's file, appended)
- `desktop/src/components/plan/CopyFromTeam.tsx` (+ `CopyFromTeam.test.tsx`) — the dialog (new)
- ⚑ task 89's templates list, task 91's views list and task 11's policy settings — one
  *Copy from team…* action each

#### Wire shapes

```jsonc
// POST /v1/teams/tm-dst…/work/copy      Idempotency-Key: 3f1c…   member (template, view); admin (policy)
{ "kind": "template", "sourceTeamId": "tm-src…", "sourceId": "tp-7a…" }
// → 201
{ "id": "tp-91…", "kind": "template",
  "dropped": [
    { "field": "labels", "value": "Payments", "reason": "no_label_in_target" },
    { "field": "serves", "value": "c-2", "reason": "criterion_other_team" },
    { "field": "estimate", "value": "5", "reason": "not_on_target_scale" } ] }

// a policy: the target's policy is replaced as a whole, as task 11's PUT does
{ "kind": "policy", "sourceTeamId": "tm-src…", "sourceId": "tm-src…" }
// → 201 { "id": "tm-dst…", "kind": "policy", "dropped": [] }

// a view whose filter names a sub-state the target lacks, and a source-team project
{ "kind": "view", "sourceTeamId": "tm-src…", "sourceId": "vw-3c…" }
// → 201 { "id": "vw-e0…", "kind": "view", "dropped": [
//   { "field": "filter.subState", "value": "ready_for_qa", "reason": "state_by_band" },
//   { "field": "filter.project", "value": "pr-src…", "reason": "project_other_team" } ] }

// refusals (each appends no event)
// the source outside the caller's readable set  → 404 copy_source_not_found { "sourceId": "tp-7a…" }
// a member copying a policy                     → 403 forbidden (server.Forbidden("admin"))
// kind "initiative" or any other word           → 400 bad_request, field "kind"
// sourceTeamId == the target team               → 400 bad_request, field "sourceTeamId"
```

For a policy `sourceId` is the source team's id, because a team has one policy (task 11). A policy
names only team-agnostic values (task 11's three switches, task 72's estimate scale and stale and
archive days; `defaultState` is always one of the built-in initial states), so a policy copy
normally drops nothing. A `state_by_band` entry means the clause was **kept, but widened**: a view
clause on a sub-state (task 75) the target does not have becomes a clause on the sub-state's band,
and the entry says so, because the copied view now shows more rows than the source did. Every
other reason means the value was left out.

#### `hubapi/work.go`, the additions (complete)

```go
// CopyKind is what "Copy from team…" can copy. Initiatives, features and
// issues are not copyable (05 L-7 names templates, views and policies only).
type CopyKind string

const (
	CopyTemplate CopyKind = "template"
	CopyView     CopyKind = "view"
	CopyPolicy   CopyKind = "policy"
)

func ValidCopyKind(k CopyKind) bool {
	return k == CopyTemplate || k == CopyView || k == CopyPolicy
}

// CopyRequest is the body of POST /v1/teams/{teamId}/work/copy. The target team
// is the path's; idempotency is the Idempotency-Key header (hub-contract:50).
type CopyRequest struct {
	Kind         CopyKind `json:"kind"`
	SourceTeamID string   `json:"sourceTeamId"`
	SourceID     string   `json:"sourceId"`
}

// DropReason says why a field of the source did not arrive as it was.
type DropReason string

const (
	DropNoLabel        DropReason = "no_label_in_target"
	DropNoLabelGroup   DropReason = "no_label_group_in_target"
	DropNoTemplate     DropReason = "no_template_in_target"
	DropNoState        DropReason = "no_state_in_target"
	DropStateByBand    DropReason = "state_by_band" // kept, mapped to the band default
	DropCriterion      DropReason = "criterion_other_team"
	DropProject        DropReason = "project_other_team"
	DropNotOnScale     DropReason = "not_on_target_scale"
)

type DroppedField struct {
	Field  string     `json:"field"`
	Value  string     `json:"value"`
	Reason DropReason `json:"reason"`
}

// CopyReply is the 201 body. ID is the new record's id; for a policy, the target team's.
type CopyReply struct {
	ID      string         `json:"id"`
	Kind    CopyKind       `json:"kind"`
	Dropped []DroppedField `json:"dropped"` // never null: [] when nothing was dropped
}

// CodeCopySourceNotFound is the one 404 for any unreadable source kind, so the
// code itself does not say which kinds exist in a team the caller cannot read.
const CodeCopySourceNotFound = "copy_source_not_found"
```

#### `hubapi/work_copy.go`, the pure matcher (complete)

The store extracts every reference the source record makes into `CopyRef`s (a template's labels,
`serves` and estimate; a view filter's labels, label groups, templates, sub-states and projects),
reads the target's vocabulary into `TargetVocab` inside the copy's transaction, and asks
`PlanCopy` what arrives. The Hub and the fake both call it, so the conformance row (task 05's
table) holds one rule, not two.

```go
package hubapi

import "strings"

// RefKind is the kind of team vocabulary a copied field points at.
type RefKind string

const (
	RefLabel      RefKind = "label"
	RefLabelGroup RefKind = "label_group"
	RefTemplate   RefKind = "template"
	RefSubState   RefKind = "sub_state"
	RefCriterion  RefKind = "criterion"
	RefProject    RefKind = "project"
	RefEstimate   RefKind = "estimate"
)

// CopyRef is one reference inside the source record, as the store extracted it.
type CopyRef struct {
	Field string     // the wire path in the source: "labels", "serves", "filter.label", …
	Kind  RefKind
	Value string     // the source's value as a person reads it: a name, key, id or number
	Band  IssueState // RefSubState only: the human state the source's sub-state sits in
}

// TargetVocab is the target team's vocabulary, read in the copy's transaction.
type TargetVocab struct {
	LabelIDByName    map[string]string // lower-cased name → id; the team's and the org's live labels
	GroupIDByName    map[string]string // lower-cased name → id
	TemplateIDByName map[string]string // lower-cased name → id; live templates of the team and org
	SubStateKeys     map[string]bool   // the target team's custom sub-state keys (task 75)
	EstimateOK       func(v string) bool
}

// Resolution is what one reference becomes in the target.
type Resolution struct {
	Ref  CopyRef
	Keep bool
	To   string // the target's id or key; for a widened sub-state, its band
}

// PlanCopy resolves every reference. It never invents vocabulary: a name with no
// match in the target is dropped with its reason. Criteria and projects are
// always dropped, because each belongs to one team's thread (decision 3).
func PlanCopy(refs []CopyRef, t TargetVocab) ([]Resolution, []DroppedField) {
	out := make([]Resolution, 0, len(refs))
	dropped := []DroppedField{}
	drop := func(r CopyRef, why DropReason) {
		out = append(out, Resolution{Ref: r})
		dropped = append(dropped, DroppedField{Field: r.Field, Value: r.Value, Reason: why})
	}
	byName := func(m map[string]string, name string) (string, bool) {
		id, ok := m[strings.ToLower(strings.TrimSpace(name))]
		return id, ok
	}
	for _, r := range refs {
		switch r.Kind {
		case RefLabel:
			if id, ok := byName(t.LabelIDByName, r.Value); ok {
				out = append(out, Resolution{Ref: r, Keep: true, To: id})
			} else {
				drop(r, DropNoLabel)
			}
		case RefLabelGroup:
			if id, ok := byName(t.GroupIDByName, r.Value); ok {
				out = append(out, Resolution{Ref: r, Keep: true, To: id})
			} else {
				drop(r, DropNoLabelGroup)
			}
		case RefTemplate:
			if id, ok := byName(t.TemplateIDByName, r.Value); ok {
				out = append(out, Resolution{Ref: r, Keep: true, To: id})
			} else {
				drop(r, DropNoTemplate)
			}
		case RefSubState:
			switch {
			case t.SubStateKeys[r.Value]:
				out = append(out, Resolution{Ref: r, Keep: true, To: r.Value})
			case r.Band != "":
				out = append(out, Resolution{Ref: r, Keep: true, To: string(r.Band)})
				dropped = append(dropped, DroppedField{Field: r.Field, Value: r.Value, Reason: DropStateByBand})
			default:
				drop(r, DropNoState)
			}
		case RefEstimate:
			if t.EstimateOK != nil && t.EstimateOK(r.Value) {
				out = append(out, Resolution{Ref: r, Keep: true, To: r.Value})
			} else {
				drop(r, DropNotOnScale)
			}
		case RefCriterion:
			drop(r, DropCriterion)
		default: // RefProject, and any kind a later task adds before teaching PlanCopy it
			drop(r, DropProject)
		}
	}
	return out, dropped
}
```

The `default` arm drops, so a reference kind added later and not taught here is left out and
listed, never copied as a dangling id into another team.

#### The store (`store/work_copy.go`, signatures complete)

```go
package store

// CopySource is the source record read inside the caller's readable set, with
// every reference it makes already extracted for PlanCopy.
type CopySource struct {
	Kind   hubapi.CopyKind
	TeamID string
	ID     string
	Refs   []hubapi.CopyRef
	Body   json.RawMessage // the record's own fields, rewritten by ApplyCopy
}

// ReadCopySource reads a template (task 78), a shared view (task 80) or a team
// policy (task 11) through the caller's RLS scope. found is false when the row
// does not exist OR the caller cannot read it: the handler answers 404 either way.
func ReadCopySource(ctx context.Context, q Querier, kind hubapi.CopyKind, teamID, id string) (src CopySource, found bool, err error)

// TargetVocab reads the target team's labels, groups, templates, sub-states and
// estimate scale, team and org, live only, lower-cased names (hubapi.TargetVocab).
func TargetVocab(ctx context.Context, q Querier, teamID string) (hubapi.TargetVocab, error)

// ApplyCopy writes the new record in the target: a new tp- or vw- id with
// created_by = the caller, or task 11's PutTeamPolicy for a policy. It writes
// only the Keep resolutions; it inserts no label, group, template or state.
func ApplyCopy(ctx context.Context, tx pgx.Tx, org, targetTeamID string, src CopySource, res []hubapi.Resolution, by string, at time.Time) (newID string, err error)
```

A copied view keeps the source's name with no suffix; a live view of that name in the target is
refused by task 80's own name rule, which this task does not change (its code, `view_exists` or
another, is **UNVERIFIED** until task 80's part names it). A copied template is the same under task
78's name rule.

#### The route (`server/work_copy.go`)

```go
func (s *Server) registerWorkCopy(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/teams/{teamId}/work/copy", s.postWorkCopy)
}
```

**`postWorkCopy`**, inside `server.Write` (the reply holds ids, field paths, names and reasons, no
issue text, so the stored replay needs no rebuild; `H14:103` claims keys for `POST`), in this order,
each refusal rolling back and appending nothing:

1. `server.Decode[hubapi.CopyRequest]`; `!ValidCopyKind` → 400 `field: "kind"`; `sourceTeamId`
   equal to the path's team → 400 `field: "sourceTeamId"` ("That is this team."); a malformed id →
   400 naming it.
2. The target team outside the caller's teams → 404 `team_not_found`; the caller's role on it
   below `member` (template, view) or below `admin` (policy) → `server.Forbidden(role)`.
3. `ReadCopySource` → `found == false` → 404 `copy_source_not_found`, `details.sourceId`. The
   same answer for a missing id and an unreadable one (decision 4).
4. `TargetVocab`, then `hubapi.PlanCopy(src.Refs, vocab)`.
5. `ApplyCopy` with the `Keep` resolutions.
6. `Emit` the kind's own create event on the target team (`template.created`, `view.created`,
   `team.work_policy_set`) with its usual `data` plus `copiedFromTeamId` and `copiedFromId` and
   `dropped` as a count (`data` carries no arrays, hub-contract §8's last paragraph, as task 11
   notes). The dropped **values** are in the reply only: a label name is team metadata (task 11's
   precedent), but a reason list on the event would outlive the reply for no reader.
7. 201 `CopyReply`.

**Matrix row** (task 07's file): `POST /v1/teams/{teamId}/work/copy`: viewer 403, member 201 for
`template` and `view` and 403 for `policy`, admin 201, outsider 404, org admin with no team role
404.

#### Interfaces — complete: the delta on ⚑ `desktop/src/api/work.ts` (task 37's file)

```ts
export type CopyKind = "template" | "view" | "policy";
export type DropReason =
  | "no_label_in_target" | "no_label_group_in_target" | "no_template_in_target"
  | "no_state_in_target" | "state_by_band" | "criterion_other_team"
  | "project_other_team" | "not_on_target_scale";
export type DroppedField = { field: string; value: string; reason: DropReason };
export type CopyReply = { id: string; kind: CopyKind; dropped: DroppedField[] };

/** POST /v1/hub/work/teams/{teamId}/work/copy through the relay (contract §4.3, unchanged). */
export function copyFromTeam(
  teamId: string,
  req: { kind: CopyKind; sourceTeamId: string; sourceId: string },
  idempotencyKey: string,
): Promise<CopyReply>;
```

The relay path follows the skeleton's `/v1/hub/work/*` rule; the exact prefix is task 16's, and a
different one is a find-and-replace. Task 37 is where the W1 desktop wire file is created; a WL
addition appends to it.

#### Interfaces — complete: `desktop/src/components/plan/CopyFromTeam.tsx`

```tsx
import type { CopyKind, CopyReply, DroppedField } from "../../api/work";

export type CopySourceItem = { teamId: string; teamName: string; id: string; name: string };

export type CopyFromTeamProps = {
  kind: CopyKind;
  targetTeamId: string;
  /** Records of this kind in the teams the person can read, other than the target. */
  sources: CopySourceItem[];
  /** Personal views only: the person's own, copied locally (decision 5). */
  personal?: CopySourceItem[];
  copy: (req: { kind: CopyKind; sourceTeamId: string; sourceId: string }, idempotencyKey: string) => Promise<CopyReply>;
  onDone: (reply: CopyReply) => void;
  onClose: () => void;
};

/** Words for each reason, in the person's terms. One source of truth for the list. */
export const DROP_WORDS: Record<DroppedField["reason"], string> = {
  no_label_in_target: "no label with this name in this team",
  no_label_group_in_target: "no label group with this name in this team",
  no_template_in_target: "no template with this name in this team",
  no_state_in_target: "no such state in this team",
  state_by_band: "this team has no such state; the view now shows the whole band",
  criterion_other_team: "a criterion belongs to its own team's brief; choose this team's",
  project_other_team: "that project belongs to the other team",
  not_on_target_scale: "not on this team's estimate scale",
};

export function CopyFromTeam(props: CopyFromTeamProps): JSX.Element;
```

The dialog lists sources grouped by team, names only (no counts). Pressing *Copy* takes one
`Idempotency-Key` for the dialog's life (`crypto.randomUUID()` on open), so a double press sends
the same key. On the reply the dialog **stays open** and shows *"Copied as a new template in
Payments."* and, when `dropped` is not empty, a heading *"Left out"* over one line per entry,
`field · value · reason words`, before a *Done* button that calls `onDone`. An empty `dropped`
reads *"Everything arrived."* and still waits for *Done*: the person sees the outcome before the
dialog goes away.

#### Tests to write first, part 1 — `hubapi/work_copy_test.go` (pure, no database)

```go
package hubapi

import "testing"

func vocab() TargetVocab {
	return TargetVocab{
		LabelIDByName:    map[string]string{"bug": "lb-t1"},
		GroupIDByName:    map[string]string{"area": "lg-t1"},
		TemplateIDByName: map[string]string{"incident": "tp-t1"},
		SubStateKeys:     map[string]bool{"in_review": true},
		EstimateOK:       func(v string) bool { return v == "1" || v == "2" || v == "3" },
	}
}

func TestPlanCopyMatchesALabelByNameAndDropsAMissingOne(t *testing.T) {
	res, dropped := PlanCopy([]CopyRef{
		{Field: "labels", Kind: RefLabel, Value: "Bug"},
		{Field: "labels", Kind: RefLabel, Value: "Payments"},
	}, vocab())
	if !res[0].Keep || res[0].To != "lb-t1" {
		t.Fatalf("Bug should match lb-t1 case-insensitively, got %+v", res[0])
	}
	if res[1].Keep {
		t.Fatalf("Payments has no label in the target and must not be kept")
	}
	if len(dropped) != 1 || dropped[0] != (DroppedField{"labels", "Payments", DropNoLabel}) {
		t.Fatalf("the missing label must be listed with its reason, got %+v", dropped)
	}
}

func TestPlanCopyAlwaysDropsCriteriaAndProjects(t *testing.T) {
	_, dropped := PlanCopy([]CopyRef{
		{Field: "serves", Kind: RefCriterion, Value: "c-2"},
		{Field: "filter.project", Kind: RefProject, Value: "pr-src"},
	}, vocab())
	if len(dropped) != 2 || dropped[0].Reason != DropCriterion || dropped[1].Reason != DropProject {
		t.Fatalf("a criterion and a project never cross teams, got %+v", dropped)
	}
}

func TestPlanCopyWidensAMissingSubStateToItsBandAndSaysSo(t *testing.T) {
	res, dropped := PlanCopy([]CopyRef{
		{Field: "filter.subState", Kind: RefSubState, Value: "in_review", Band: "started"},
		{Field: "filter.subState", Kind: RefSubState, Value: "ready_for_qa", Band: "started"},
	}, vocab())
	if !res[0].Keep || res[0].To != "in_review" {
		t.Fatalf("a matching key is kept as itself, got %+v", res[0])
	}
	if !res[1].Keep || res[1].To != "started" {
		t.Fatalf("a missing key widens to its band, got %+v", res[1])
	}
	if len(dropped) != 1 || dropped[0].Reason != DropStateByBand {
		t.Fatalf("the widening must be listed, got %+v", dropped)
	}
}

func TestPlanCopyDropsAnEstimateOffTheTargetScale(t *testing.T) {
	_, dropped := PlanCopy([]CopyRef{{Field: "estimate", Kind: RefEstimate, Value: "5"}}, vocab())
	if len(dropped) != 1 || dropped[0].Reason != DropNotOnScale {
		t.Fatalf("5 is not on a 1-3 scale, got %+v", dropped)
	}
}

func TestPlanCopyDropsAnUnknownRefKind(t *testing.T) {
	res, dropped := PlanCopy([]CopyRef{{Field: "filter.cycle", Kind: RefKind("cycle"), Value: "cy-1"}}, vocab())
	if res[0].Keep || len(dropped) != 1 {
		t.Fatalf("a kind PlanCopy was not taught must be dropped, got %+v %+v", res, dropped)
	}
}

func TestPlanCopyDroppedIsNeverNil(t *testing.T) {
	_, dropped := PlanCopy(nil, vocab())
	if dropped == nil {
		t.Fatal("dropped must be [] on the wire, never null")
	}
}
```

#### Tests to write first, part 2 — `server/work_copy_integration_test.go` (`//go:build integration`)

The cast is task 11's (`task-11.md:464-471`): tokens from `devissuer.MintAccessToken`, a Hub
`httptest.Server` over `hubtest.Migrated`, `as(name)`. Ada is admin of Portal (`EP`) and a member
of Billing (`PAY`); Grace is a member of both; Linus is admin of Billing only. Billing has labels
`Bug` and `Payments`, template `Incident` (labels `Bug`, `Payments`; `serves` one criterion of a
`PAY` brief; estimate 5 on a Fibonacci scale) and a shared view `Hot` (filter: label `Payments`,
sub-state `ready_for_qa` in band `started`). Portal has label `bug`, a linear scale, and no
sub-states. Events are counted from `hub.audit_events` as the owner, before and after.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACopiedTemplateDropsWhatTheTargetLacksAndCreatesNothing` | Grace copies `Incident` into Portal → 201; the new `tp-` has label Portal's `bug` only; `dropped` holds `labels · Payments · no_label_in_target`, `serves · c-… · criterion_other_team`, `estimate · 5 · not_on_target_scale`; Portal's label count is unchanged | a label created silently in the target; a value dropped silently |
| `TestEditingTheSourceLeavesTheCopyUnchanged` | after the copy, Linus renames `Incident` and adds a label; `GET` the copy → name and labels as copied | a reference, not a copy |
| `TestAMemberCannotCopyAPolicy` | Grace `kind: policy` into Portal → 403 `required: admin`; Ada → 201; no event on Grace's call | members copying policy |
| `TestAnUnreadableSourceIs404` | a team Grace is not in: its template id → 404 `copy_source_not_found`; a random `tp-` id → the same body byte for byte | 403 leaks existence; two different 404s tell the ids apart |
| `TestOneIdempotencyKeyMakesOneCopy` | two `POST`s with one `Idempotency-Key` → the same reply, one new `tp-`, one `template.created` | a double press making two copies |
| `TestACopiedViewWidensAMissingSubStateAndSaysSo` | Grace copies `Hot` into Portal → `dropped` holds `filter.label · Payments` and `filter.subState · ready_for_qa · state_by_band`; the copy's filter reads `state = started` | a sub-state clause copied as a dangling key |
| `TestTheCopyIsOneEventWithItsSource` | one `template.created` whose `data.copiedFromTeamId` is Billing's id and `copiedFromId` the source's; `data` holds no `Payments` string | a second `copy.*` event; the dropped names on the feed |
| `TestCopyingIntoTheSameTeamIs400` | `sourceTeamId` equal to the path's team → 400 `field: sourceTeamId` | a same-team "copy" that duplicates by accident |

#### Tests to write first, part 3 — `desktop/src/components/plan/CopyFromTeam.test.tsx`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `lists dropped before closing` | a reply with two `dropped` entries: the dialog stays open, shows *Left out* and both lines in `DROP_WORDS` words, and `onDone` is called only on *Done* | a dialog that closes on success and hides what was left out |
| `says everything arrived` | an empty `dropped`: *"Everything arrived."*, still waits for *Done* | silence read as success |
| `one key per dialog` | two clicks on *Copy* call `copy` with the same `idempotencyKey` | a fresh key per click |
| `names only, no counts` | the source list renders team and record names; no digit appears in it | a count per team in the picker |
| `personal views copy locally` | with `personal`, choosing one calls no `copy` and writes the preference | a personal view sent to the Hub (task 91's rule) |
| `no prediction` | `expectNoPrediction(container)` after a reply with dropped entries | a "2 of 5 fields copied" line |

`expectNoPrediction` is task 20's helper (`desktop/src/test/expectNoPrediction.ts`,
`task-20.md:33`). The dialog's motion is none: it swaps its body in place, `transform`/`opacity` only if any.

#### Steps

- [ ] **Step 1: confirm the sync base** (rule H1): `feat/hub` sync-merged, tasks 11, 75, 78, 80, 89
  and 91 committed. Read their parts; replace every name this part took from their scopes
  (`Template` fields, the view filter's shape, the sub-state key, the name-clash codes) with the
  parts' spelling.
- [ ] **Step 2: the pure tests first.** Write `hubapi/work_copy_test.go`; it fails to compile.
  Append the `hubapi/work.go` block and write `hubapi/work_copy.go`; the six tests pass.
- [ ] **Step 3: the integration tests.** Write `server/work_copy_integration_test.go`; every row
  fails with 404 `route_not_found`.
- [ ] **Step 4: the store and route.** Write `store/work_copy.go` (the extractors per kind, then
  `TargetVocab`, then `ApplyCopy`) and `server/work_copy.go`; register it; add the matrix row. Run
  until green.
- [ ] **Step 5: the fake.** Add the route to `hubclient`'s fake, calling `PlanCopy`; add one
  conformance row (a template copy with one missing label) and run it against the fake and the Hub.
- [ ] **Step 6: the dialog.** Write `CopyFromTeam.test.tsx`, then `CopyFromTeam.tsx`; add the
  *Copy from team…* action to task 89's templates list, task 91's views list and task 11's policy
  settings (the policy action shown to team admins only; the Hub's 403 is the second guard).
- [ ] **Step 7: run the acceptance commands** and commit by path, each message ending with the
  `Co-Authored-By` line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -run 'TestPlanCopy' -v 2>&1 | grep -c "^--- PASS"
# expect: 6
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(ACopiedTemplateDrops|EditingTheSourceLeaves|AMemberCannotCopyAPolicy|AnUnreadableSourceIs404|OneIdempotencyKeyMakesOneCopy|ACopiedViewWidens|TheCopyIsOneEvent|CopyingIntoTheSameTeam)' \
  2>&1 | grep -c "^--- PASS"
# expect: 8
go test -tags integration ./control-plane/internal/hub/... -race -count=1 -run 'Matrix|Conformance'
# expect: ok for every package
npm --prefix desktop test -- components/plan/CopyFromTeam
# expect: 6 tests pass
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- A copy between orgs, or of initiatives, features, issues, loops or intake rules.
- A live link: a copy never follows its source, and nothing offers to "update from source".
- That a widened view (`state_by_band`) shows what its source showed; the reply says it does not.
- That a copied template's criteria are chosen: they are always left out, and the person picks this
  team's in the create form (decision 3).
- Any name this part took from the unwritten parts 75, 78, 80, 89 and 91 (step 1 checks them).

**Parity.** Closes **L-7** (`05-design.md:111`, MATCH, slice `later`), with the command and output
recorded by task 187.

#### Gaps found while writing this part

1. **The scope's body `requestId`** is not how the Hub does idempotency; the contract's
   `Idempotency-Key` header is (`hub-contract:50`). Decision 6 follows the contract; the scope
   should be corrected.
2. **The scope's *"a personal view is copied only by its owner"*** assumes personal views are Hub
   records; tasks 80 and 91 keep them as desktop preferences never sent to the Hub. Decision 5
   makes the copy local; task 177's scope makes the same assumption for personal dashboards.
3. **The file map has no WL rows** (the scope says so): `hubapi/work_copy.go`, `store/` and
   `server/work_copy.go`, and `components/plan/CopyFromTeam.tsx` should be listed.
4. **The name-clash code** for a copied view or template whose name the target already uses is
   whatever tasks 78 and 80 define; neither part exists yet, so this part names none.
