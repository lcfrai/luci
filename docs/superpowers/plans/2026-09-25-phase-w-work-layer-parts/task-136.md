### Task 136: assists, part 1: triage suggestions with `serves` and a "why"; the calibrated dial

W4, control plane and desktop; no seam. Depends on task 124 (`hubclient`'s W4 methods) and phase 6's
gateway pieces: P6 task 03 (`wire.MessagesURL`, `wire.RouteSource`), P6 task 13 (the project route
source), P6 task 16 (`fakeanthropic`), all **UNVERIFIED until merge**. Reads task 42's
`suggestCriteria` and `CriterionPicker` (`task-42.md:74-160`), task 46's pickers and `IssueHead`
(`task-46.md:554-613`), task 100's `AgentSwitch` (`task-100.md:48-53`, `:312-318`), task 14's title
search (`task-14.md:284`, `task-16.md:398`) and task 20's `expectNoPrediction`. Estimate 0.375 (3 h).

This is the first gateway-backed assist. Tasks 137 to 140 reuse its service, its routes file and its
test rig. **Tasks 138 and 139 were written first and assumed this part's names**
(`task-138.md:62-68`, `task-139.md:79-89`); every one of them is kept here, so neither renames.

Closes **L-51** and **L-52**; **L-53** once the calibration record lands (gap 2).

**What the reader sees** (05 §1 C L-51 to L-53, §2.4; AX §7, AX Principle 7):

- **In Triage and on the issue page, a suggestion sits beside its picker**, in the agent class:
  *"Suggested: bug · because EP-120 and EP-131, both labelled bug, share 'upload fails'"*, where
  EP-120 and EP-131 are links. A `serves` suggestion names the criterion and its brief: *"Suggested:
  serves C3 'Resume a dropped upload' of Upload brief · because the title says 'resume'"*, with C3
  a link to the criterion. There is no score, no percentage and no confidence word.
- **Accept** is one write, as the person. **Dismiss** hides it for this issue revision.
- **No suggestion ever moves an issue's state.** State is not a property an assist may name.
- **The dial** (team settings → *Assists*, and the `…` on any suggestion): per property, one of
  *Hide*, *Show* (PROPOSE AND WAIT), *Auto-apply* (ACT AND TELL ME). Auto-applied values are
  written as the person and listed under DECIDED WITHOUT YOU on the Desk with *Undo*.
- **Under each dial row, no number**, until the calibration record exists: *"No record yet of how
  you answered label suggestions."* When it exists: *"Label suggestions you answered: 23 unchanged,
  2 changed, none rejected."*
- **Failures are words.** *"Suggestions unavailable: assists are off for this org"*, *"Suggestions
  unavailable: the gateway did not answer"*. Task 42's text-match suggestion for `serves` stays,
  as it was in W1.

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 14, 16, 24, 42, 46,
100, 124, 125, 138 and 139. There is no part for tasks 39's DECIDED WITHOUT YOU filler (task 64),
83, 84 or 99; their names here are their scopes' and **UNVERIFIED**. The desktop's `ProblemCode`
union is `desktop/src/api/client.ts:40-48` (task 138 cites `:59`; the union ends at `:48` today).
`docs/AGENTIC-UX.md` §7 (`:175-194`) names the four levels and the calibration sentence.
`control-plane/internal/api/` has no `workassist.go`; the file map has no row for the assists'
control-plane side (gap 5).

**Decisions made here, so nobody re-derives them:**

1. **One service, one call shape.** `workassist.Service.Complete(ctx, projectID, Call) (Reply,
   error)` is the only function in the product that sends text to a model outside a session. It
   checks the kill switch, then reads the project's effective route, then calls the gateway on that
   route. Every later assist (137-140) goes through it and adds no second path.
2. **The route is the project's, read fresh.** `Routes wire.RouteSource` (P6 task 13) gives
   `ProjectInference.Effective`; an unknown route (`wire.ErrRouteUnknown`) refuses with
   `ErrGatewayDown` and makes no call. An AU-only project's assist goes to AU (OD-19), never Global
   as a fallback.
3. **The kill switch fails closed.** The org switch is task 100's `AgentSwitch.Orchestrators`
   (`task-100.md:314`). While it is `false`, or while it cannot be read, `Complete` returns
   `ErrAssistsOff` **before** any request. There is no separate assists switch in SW.19 (gap 3).
4. **No session, so no session key: the gateway client is an interface, and its real
   implementation is a proposal.** P6's `MintRequest.SessionClass` is `solo`, `worker` or
   `orchestrator` (`task-03.md:386-392`). An assist has no session and no pod. This part writes the
   proposal to the phase 6 owner (gap 1: an `assist` class, minted per person and project, bound to
   the control plane's address, drawn from the person's own budget slice) and **does not apply it**.
   What is built: `Gateway` (the interface) and `DirectGateway`, which posts to one upstream per
   route and is wired only when both upstreams are loopback addresses, i.e. P6 task 16's fakes in
   local mode. Every other deployment builds `Service` with `Gateway: nil`, and every assist answers
   `502 gateway_unavailable` until phase 6 decides. That is the honest state, and the desktop shows
   it in words.
5. **Closed vocabulary, checked twice.** The prompt lists, with short keys, the only values each
   property may take: the team's labels, the project's features, the team's members, the caller's
   readable teams, the four priorities, and the criteria of the project's readable briefs. A value
   outside its list is dropped on the control plane, whatever the model says.
6. **Every item needs a why that points at something it was given.** A why is `{kind, key}` with
   `kind` one of `issue`, `criterion`, `comment`, `guidance`, and `key` one of the keys the prompt
   carried. An item with no why, or whose why names a key not in the request, is **dropped before it
   reaches the desktop** (AX Principle 7). The desktop also refuses to render an item with no why
   (belt and braces: a future route could forget).
7. **What the prompt carries, and what it never does.** The issue's **current** revision's title and
   description (never an erased revision: an erased revision's texts read `""` and `ErasedAt` is
   set, `task-03.md:306-307`, so an erased issue is refused with `issue_not_found` before any
   call); up to 10 of the caller's readable issues from the Hub's title search on the issue's title
   (task 14), each as alias, title and its current labels, priority and feature, as examples the why
   may cite (`kind: issue`); the criteria. **No comment text** in part 1 (the `comment` why kind is
   accepted by the type for 137-139 and dropped here because no comment key is given). No erased
   text, no other team's issue (the Hub's search returns the caller's readable issues only).
8. **The control plane logs no issue text.** Handlers log the route, the issue id, the item count
   and the error class. A test reads the log buffer and finds neither the title nor the reply.
9. **Accepting is the person's write, through the desktop's own client.** The control plane only
   suggests; it never writes. Priority, assignee and labels are one `patchIssue` (task 37's
   `PatchIssueRequest`, `task-37.md:190-192`, which has exactly those three); `serves` is one
   `writeIssueRevision` with `baseRevision` the suggestion's `revision` and the head's content plus
   the criterion (task 09; a stale base is `409 revision_conflict`, `task-37.md:193`); feature and
   team go through their own routes (task 83's feature assignment and task 84's team move, both
   **UNVERIFIED** names, gap 6). The desktop's client carries the person's token, so the actor is
   the person. A suggestion for a revision older than the replica's head is not applied: it reads
   *"The issue changed since this suggestion. Ask again."*
10. **The dial's levels are AX §7's, three of the four.** *Hide*, *Show* = PROPOSE AND WAIT,
    *Auto-apply* = ACT AND TELL ME; FULL (apply and tell nobody) is not offered, because every
    auto-applied value is listed under DECIDED WITHOUT YOU with Undo. The dial is the person's own
    setting, per property, kept on this desktop (`localStorage`, key `panacea.assist.<property>`);
    the default is *Show*. It is not synced (gap 4). `state` is not a property, so it has no row.
11. **Calibration shows a count or nothing.** The words are *"N unchanged, N changed, N rejected"*,
    never *"N of M"* (SW.8's rule) and never a percentage. The count comes from the person's own
    answered suggestions, recorded on the Hub (gap 2); until that route exists the row says there is
    no record, and no number appears anywhere on the dial (AX anti-pattern 12).
12. **Auto-apply writes one property at a time, and never over a person.** An auto-applied value is
    written only when the property is still empty on the replica's head (no label, no assignee,
    priority `none`, no feature, no serves: a person has not set it) and the head's revision is
    still the suggestion's. `patchIssue` carries no base revision, so between that check and the
    write a person's change can race it; the Hub's last write wins and both writes are events with
    their actors. `serves` carries `baseRevision` and races as a `409`, shown in words, not retried.

**Files** (the file map has no row for the assists' control-plane side; these names follow task 24's):

- `control-plane/internal/workassist/assist.go` — `Service`, `Call`, `Reply`, `Gateway`,
  `DirectGateway`, `Complete`, `ErrAssistsOff`, `ErrGatewayDown`, `ErrNotFound` (new)
- `control-plane/internal/workassist/why.go` — `Why`, `WhyKind`, `Keys`, `keepLinked` (new)
- `control-plane/internal/workassist/triage.go` — `TriageRequest`, `TriageSuggestion`, `Item`,
  `Property`, `(*Service).Triage`, the prompt, `vocabulary` (new)
- `control-plane/internal/workassist/{assist,why,triage}_test.go`, `rig_test.go` (the rig tasks
  137-140 reuse), `testdata/triage/*.json` (new)
- `control-plane/internal/api/workassist.go`, `workassist_test.go` — `registerWorkAssist`,
  `postWorkTriage`, `assistProblem`, `readable`, `readableTeam` (new)
- `control-plane/internal/api/api.go` ⚑ — `Deps.WorkAssist *workassist.Service` (optional) and
  `a.registerWorkAssist(guarded)` beside task 24's `a.registerWorkStart(guarded)` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the pattern listed as a `POST` (modified)
- `control-plane/cmd/api/config.go`, `config_test.go`, `main.go` ⚑ — `PANACEA_APP_WORK_ASSIST_*`
  (modified)
- `desktop/src/api/client.ts` ⚑ — `assists_off`, `gateway_unavailable` in `ProblemCode`
  (`:40-48`) (modified)
- `desktop/src/api/work.ts` ⚑ (task 37) — one `WORK_LOCAL` row, the wire types, `assistTriage`
  (modified)
- `desktop/src/work/assist.ts`, `assist.test.ts` — `renderable`, `applyPlan`, `dialLevel`,
  `calibrationWords`, pure (new)
- `desktop/src/components/work/AssistSuggestion.tsx`, `.test.tsx` (new)
- `desktop/src/components/work/AssistDial.tsx`, `.test.tsx` (new)
- `desktop/src/components/work/TriageView.tsx` ⚑ (task 42), `CriterionPicker.tsx` ⚑ (task 42) and
  `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) — the suggestion beside each picker
  (modified)

**What this part fixes for tasks 137 to 140** (the names 138 and 139 already use):

| Name | Shape |
|---|---|
| `workassist.Service` | `Hub hubclient.Work`, `Gateway Gateway`, `Switch SwitchReader`, `Routes wire.RouteSource`, `OrgID string`, `Log *slog.Logger` |
| `(*Service).Complete(ctx, projectID string, c Call) (Reply, error)` | decision 1 |
| `Call{System, User string; MaxTokens int}`, `Reply{Text string; Route dataclass.Route}` | as `task-139.md:86` |
| `ErrAssistsOff`, `ErrGatewayDown` | `503 assists_off`, `502 gateway_unavailable` |
| `Why{Kind, Key}`, `Keys`, `keepLinked` | decision 6; 137 adds `guidance` keys, 139 `comment` keys |
| `Deps.WorkAssist *workassist.Service` | the control plane's dependency |
| `(*API).assistProblem(w, err)`, `(*API).readable(w, r, projectID) bool`, `(*API).readableTeam(w, r, teamID) bool` | the route helpers |
| `registerWorkAssist(rs *routes)` | each later task appends one `register…` call inside it |
| `newRig(t)`, `r.hub`, `r.reply(json)`, `r.switchOn()`, `r.prompts()`, `r.routes()` | the test rig, `rig_test.go` |

`readable` takes `(w, r, projectID)` here and task 139 wrote `readable(ctx, projectID)`
(`task-139.md:89`); the handler form is the one a route needs, and `s.readable(ctx, projectID)
error` on `Service` is the form 139's service code calls. Both exist.

#### Interfaces — complete: `control-plane/internal/workassist/assist.go`

```go
// Package workassist is every gateway-backed assist of the work layer (W4,
// tasks 136-140). Complete is the one function here that sends text to a
// model; nothing in this package writes to the Hub.
package workassist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

var (
	ErrAssistsOff  = errors.New("workassist: assists are off for this org")   // 503 assists_off
	ErrGatewayDown = errors.New("workassist: the gateway did not answer")     // 502 gateway_unavailable
	ErrNotFound    = errors.New("workassist: no such issue or project for this caller") // 404
)

// SwitchReader is task 100's GetAgentSwitch, narrowed.
type SwitchReader interface {
	GetAgentSwitch(ctx context.Context, orgID string) (hubapi.AgentSwitch, error)
}

// Gateway sends one Messages request on one route. The body is Anthropic's
// Messages shape, non-streaming; the reply is the first text block.
type Gateway interface {
	Messages(ctx context.Context, route dataclass.Route, body []byte) (string, error)
}

type Service struct {
	Hub     hubclient.Work    // the relay, as the caller (task 16)
	Gateway Gateway           // nil: every call is ErrGatewayDown (decision 4)
	Switch  SwitchReader      // the org's AgentSwitch (task 100)
	Routes  wire.RouteSource  // P6 task 13
	OrgID   string
	Model   string            // the catalogue id without its route, e.g. "claude-haiku-4-5"
	Log     *slog.Logger
}

type Call struct {
	System    string
	User      string
	MaxTokens int
}

type Reply struct {
	Text  string
	Route dataclass.Route
}

// Complete checks the switch, then the route, then calls. It never logs c's text.
func (s *Service) Complete(ctx context.Context, projectID string, c Call) (Reply, error) {
	sw, err := s.Switch.GetAgentSwitch(ctx, s.OrgID)
	if err != nil || !sw.Orchestrators {
		return Reply{}, ErrAssistsOff // decision 3: unreadable is off
	}
	pi, err := s.Routes.Project(ctx, projectID)
	if err != nil || (pi.Effective != dataclass.RouteGlobal && pi.Effective != dataclass.RouteAU) {
		return Reply{}, ErrGatewayDown // decision 2: no call on an unknown route
	}
	if s.Gateway == nil {
		return Reply{}, ErrGatewayDown
	}
	body, err := json.Marshal(map[string]any{
		"model": string(pi.Effective) + "/" + s.Model, "max_tokens": c.MaxTokens, "system": c.System,
		"messages": []map[string]any{{"role": "user", "content": c.User}},
	})
	if err != nil {
		return Reply{}, err
	}
	text, err := s.Gateway.Messages(ctx, pi.Effective, body)
	if err != nil {
		s.Log.Warn("assist call failed", "projectId", projectID, "route", pi.Effective, "err", errClass(err))
		return Reply{}, ErrGatewayDown
	}
	return Reply{Text: text, Route: pi.Effective}, nil
}
```

The module path is `control-plane/go.mod:1`'s. `dataclass.RouteGlobal` and `dataclass.RouteAU` are P6 task 01's
constants (**UNVERIFIED** names; the values are `"global"` and `"au"`, `task-03.md:512`'s
`RouteAtMint`). The model name is `<route>/<id>`, P6 task 02's catalogue form (`task-02.md:125-127`).
`errClass` returns the error's type and, for a `*url.Error`, its `Op`; never a body.

```go
// DirectGateway posts to one upstream per route: P6 task 16's fakes in local
// mode, and nothing else (decision 4; cmd/api refuses a non-loopback base).
type DirectGateway struct {
	Base map[dataclass.Route]string // "http://127.0.0.1:4900", "http://127.0.0.1:4901"
	HTTP *http.Client               // cmd/api sets a 20 s timeout
}

func (d DirectGateway) Messages(ctx context.Context, route dataclass.Route, body []byte) (string, error) {
	base, ok := d.Base[route]
	if !ok {
		return "", fmt.Errorf("no upstream for route %q", route)
	}
	u, err := wire.MessagesURL(base) // P6 task 03: never concatenated by hand
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	var out struct {
		Content []struct{ Type, Text string } `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("upstream reply: %w", err)
	}
	for _, c := range out.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", errors.New("upstream reply has no text block")
}
```

`DirectGateway` carries no key: the fakes take none. The proposed `KeysmithGateway` (gap 1)
would mint an `assist`-class key per person and project through `gatewayclient` and post to
`gatewayclient.SandboxBaseURL`'s gatekeeper; it is not written here.

#### Interfaces — complete: `control-plane/internal/workassist/why.go`

```go
package workassist

// WhyKind is what a why may point at. Part 1 gives issue and criterion keys;
// 137 adds guidance, 139 comment. A kind with no keys in a request never survives.
type WhyKind string

const (
	WhyIssue     WhyKind = "issue"     // key: an issue alias the prompt listed, e.g. "EP-120"
	WhyCriterion WhyKind = "criterion" // key: "c1"…"cN", mapped back to {briefKey, criterionId}
	WhyComment   WhyKind = "comment"   // key: "ic-…" (task 139)
	WhyGuidance  WhyKind = "guidance"  // key: "gd-…" (task 137)
)

// Why is one link. On the wire the desktop gets the resolved form: Key is the
// alias, the ic-/gd- id, or the criterion's "<briefKey>#<criterionId>".
type Why struct {
	Kind WhyKind `json:"kind"`
	Key  string  `json:"key"`
}

// Keys is every key a request carried, by kind, each mapped to the key the
// desktop may link to. Only keys in Keys survive keepLinked.
type Keys map[WhyKind]map[string]string

func (k Keys) add(kind WhyKind, promptKey, linkKey string) {
	if k[kind] == nil {
		k[kind] = map[string]string{}
	}
	k[kind][promptKey] = linkKey
}

// keepLinked resolves each why against the request's keys and drops the ones
// that name nothing given. It returns nil when none survives: the caller then
// drops the whole item (decision 6).
func keepLinked(k Keys, whys []Why) []Why {
	var out []Why
	seen := map[Why]bool{}
	for _, w := range whys {
		link, ok := k[w.Kind][w.Key]
		if !ok {
			continue
		}
		r := Why{Kind: w.Kind, Key: link}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
```

#### Interfaces — complete: `control-plane/internal/workassist/triage.go`, the wire

```go
// Property is what a triage suggestion may set. There is no state (SW.3).
type Property string

const (
	PropTeam     Property = "team"
	PropFeature  Property = "feature"
	PropAssignee Property = "assignee"
	PropLabels   Property = "labels"
	PropPriority Property = "priority"
	PropServes   Property = "serves"
)

// TriageRequest is POST /v1/work/assist/triage's body.
type TriageRequest struct {
	IssueID string `json:"issueId"`
}

// Item is one suggestion. Value is an id (team tm-, feature ft-, user u-, label
// lb-), a priority word, or for serves "<briefKey>#<criterionId>". Labels is one
// label per item, so each has its own why and its own Accept.
type Item struct {
	Property Property `json:"property"`
	Value    string   `json:"value"`
	Why      []Why    `json:"why"` // never empty on the wire
}

// TriageSuggestion is the 200 body. Revision is the issue revision the prompt
// read; the desktop applies nothing to a later head (decision 9).
type TriageSuggestion struct {
	IssueID  string          `json:"issueId"`
	Revision int             `json:"revision"`
	Items    []Item          `json:"items"` // [] when nothing survived, never null
	Route    dataclass.Route `json:"route"`
}
```

```jsonc
// POST /v1/work/assist/triage            the caller's readable issue only
{ "issueId": "is-0a1b2c3d4e5f6071" }
// 200
{ "issueId": "is-0a1b2c3d4e5f6071", "revision": 2, "route": "au",
  "items": [
    { "property": "labels", "value": "lb-9f0e1d2c3b4a5968",
      "why": [ { "kind": "issue", "key": "EP-120" }, { "kind": "issue", "key": "EP-131" } ] },
    { "property": "serves", "value": "brief:bf-7a2c1d0e9f8a7b6c@4#C3",
      "why": [ { "kind": "criterion", "key": "brief:bf-7a2c1d0e9f8a7b6c@4#C3" } ] } ] }
// 404 issue_not_found (not readable, or erased) · 503 assists_off · 502 gateway_unavailable
// 400 bad_request (not an issue id) · no issue text in any error body
```

#### Interfaces — complete: `triage.go`, the service

Names are chosen apart from task 138's (`Vocabulary`, `VocabularySource`, `Term`, `keyed`,
`lookup`, `pick`) and task 139's, which share the package.

```go
// Criterion is one criterion of a readable brief's current revision.
type Criterion struct {
	BriefKey, CriterionID, Text, BriefTitle string
}

// Choice is one allowed value, the id the Hub knows and the name the model sees.
type Choice struct{ ID, Name string }

// TriageSource is what Triage reads, as the caller. hubTriageSource (below)
// implements it over hubclient.Work; Issue and SearchIssues are task 16's
// (task-16.md:398); the other four names are UNVERIFIED (gap 6).
type TriageSource interface {
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
	SearchIssues(ctx context.Context, q hubapi.IssueSearchQuery) ([]hubapi.IssueHead, error)
	Choices(ctx context.Context, projectID string, p Property) ([]Choice, error) // team, feature, assignee, labels
	Criteria(ctx context.Context, projectID string) ([]Criterion, error)       // readable briefs only
}

var priorities = []Choice{{"urgent", "urgent"}, {"high", "high"}, {"medium", "medium"}, {"low", "low"}}

// Triage reads, prompts, calls once, and keeps only closed, linked items.
func (s *Service) Triage(ctx context.Context, src TriageSource, req TriageRequest) (TriageSuggestion, error) {
	iw, err := src.Issue(ctx, req.IssueID)
	if err != nil || iw.Revision.ErasedAt != nil {
		return TriageSuggestion{}, ErrNotFound // unreadable and erased read the same (decision 7)
	}
	pid := iw.Head.ProjectID
	keys := Keys{}
	allowed := map[Property]map[string]bool{}
	p := triagePrompt{Title: iw.Revision.Content.Title.Text, Description: iw.Revision.Content.Description.Text}
	for _, prop := range []Property{PropTeam, PropFeature, PropAssignee, PropLabels} {
		cs, err := src.Choices(ctx, pid, prop)
		if err != nil {
			return TriageSuggestion{}, ErrNotFound
		}
		p.Choices = append(p.Choices, promptChoices{Property: prop, Values: cs})
		allowed[prop] = ids(cs)
	}
	p.Choices = append(p.Choices, promptChoices{Property: PropPriority, Values: priorities})
	allowed[PropPriority] = ids(priorities)
	crit, err := src.Criteria(ctx, pid)
	if err != nil {
		return TriageSuggestion{}, ErrNotFound
	}
	allowed[PropServes] = map[string]bool{}
	for i, c := range crit {
		k, link := fmt.Sprintf("c%d", i+1), c.BriefKey+"#"+c.CriterionID
		p.Criteria = append(p.Criteria, promptCriterion{Key: k, Text: c.Text, Brief: c.BriefTitle})
		keys.add(WhyCriterion, k, link)
		allowed[PropServes][k] = true
	}
	if ex, err := src.SearchIssues(ctx, hubapi.IssueSearchQuery{ProjectID: pid, Q: searchTerms(p.Title), Limit: 10}); err == nil {
		for _, h := range ex {
			if h.ID != req.IssueID && h.Title != "" { // an erased head's title reads ""
				p.Examples = append(p.Examples, exampleOf(h))
				keys.add(WhyIssue, h.Alias, h.Alias)
			}
		}
	} // a failed search leaves no examples; it does not fail the assist
	user, err := json.Marshal(p)
	if err != nil {
		return TriageSuggestion{}, err
	}
	reply, err := s.Complete(ctx, pid, Call{System: triageSystem, User: string(user), MaxTokens: 800})
	if err != nil {
		return TriageSuggestion{}, err
	}
	items := validateItems(reply.Text, allowed, keys)
	s.Log.Info("triage assist", "issueId", req.IssueID, "route", reply.Route, "items", len(items))
	return TriageSuggestion{IssueID: req.IssueID, Revision: iw.Revision.Revision, Items: items, Route: reply.Route}, nil
}
```

`ids`, `exampleOf` and `searchTerms` are three-line helpers: `ids` makes the set of `Choice.ID`s;
`exampleOf` keeps an example's alias, title, label ids, priority and feature id and nothing else;
`searchTerms` is the title's first 200 characters with task 14's escaping. `hubapi.IssueHead`'s
`Title` field (`task-14.md:285`'s projection) is **UNVERIFIED** as a Go name.

```go
// The prompt is JSON the model reads; every value it may return is listed in it.
type promptChoices struct {
	Property Property `json:"property"`
	Values   []Choice `json:"values"`
}
type promptCriterion struct{ Key, Text, Brief string }
type example struct {
	Alias, Title, Priority, Feature string
	Labels                          []string
}
type triagePrompt struct {
	Title, Description string
	Choices            []promptChoices
	Criteria           []promptCriterion
	Examples           []example
}

const triageSystem = `You suggest properties for one work item. Reply with JSON only:
{"items":[{"property":"team|feature|assignee|labels|priority|serves","value":"...","why":[{"kind":"issue|criterion","key":"..."}]}]}
Use only values listed under Choices (by id) or Criteria (by key). Give one label per item.
Every item needs at least one why naming an Examples alias or a Criteria key you used.
Suggest nothing you cannot tie to one of those. Never suggest a state.`

// validateItems parses the reply and keeps an item only when its property is
// known, its value is allowed for that property, and a why survives
// keepLinked. An unparseable reply keeps nothing: it is not an error.
func validateItems(text string, allowed map[Property]map[string]bool, keys Keys) []Item {
	var raw struct {
		Items []Item `json:"items"`
	}
	if json.Unmarshal([]byte(text), &raw) != nil {
		return []Item{}
	}
	type itemKey struct {
		p Property
		v string
	}
	out := []Item{}
	seen := map[itemKey]bool{}
	for _, it := range raw.Items {
		if !allowed[it.Property][it.Value] { // unknown property (state included) or value
			continue
		}
		it.Why = keepLinked(keys, it.Why)
		if len(it.Why) == 0 {
			continue // AX Principle 7
		}
		if it.Property == PropServes {
			it.Value = keys[WhyCriterion][it.Value] // "c3" → "brief:bf-…@4#C3"
		}
		k := itemKey{it.Property, it.Value}
		if !seen[k] {
			seen[k] = true
			out = append(out, it)
		}
	}
	return out
}
```

`Item` holds a slice and is not comparable, so `seen` keys on `itemKey`. The model's text is never returned,
logged or stored; only validated ids leave this function.

**`hubTriageSource`** (in `triage.go`) implements `TriageSource` over `hubclient.Work`: `Issue`
and `SearchIssues` pass through; `Choices` reads the project's team (the issue head's team),
then `labels(teamId)` (task 04), the team's members (task 84), the project's features (task 72)
and the caller's teams (Phase H's `Teams`), each mapped to `Choice{ID, Name}`; `Criteria` lists the
project's briefs (`Briefs.List`, `hub/hubclient/hubclient.go:55` on `feat/hub`), reads each
head's current revision (`Brief`, `:57`) and decodes its criteria with phase 4's content decoder
(**UNVERIFIED** name, gap 6). A brief the caller cannot read is not in the list the Hub returns,
so its criteria never reach the prompt, and a `serves` naming one cannot survive `allowed`.

#### Interfaces — complete: `control-plane/internal/api/workassist.go`

```go
package api

// registerWorkAssist mounts every assist route. Tasks 137-140 each append one
// register… call inside it; none adds a line to api.go.
func (a *API) registerWorkAssist(rs *routes) {
	if a.deps.WorkAssist == nil {
		return // no service: the routes are absent, and the desktop reads 404 as "not built"
	}
	rs.HandleFunc("POST /v1/work/assist/triage", a.postWorkTriage)
}

func (a *API) postWorkTriage(w http.ResponseWriter, r *http.Request) {
	var req workassist.TriageRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if (wire.WorkRef{IssueID: req.IssueID}).Validate() != nil || req.IssueID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "issueId is not an issue id", nil)
		return
	}
	src := workassist.HubTriageSource{Hub: a.deps.WorkAssist.Hub}
	res, err := a.deps.WorkAssist.Triage(r.Context(), src, req)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// assistProblem maps the service's sentinels, and nothing else leaks: an
// unexpected error is 500 internal with no text from the request or the model.
func (a *API) assistProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workassist.ErrAssistsOff):
		httpx.WriteProblem(w, http.StatusServiceUnavailable, "assists_off", "Assists are off for this org.", nil)
	case errors.Is(err, workassist.ErrGatewayDown):
		httpx.WriteProblem(w, http.StatusBadGateway, "gateway_unavailable", "The gateway did not answer.", nil)
	case errors.Is(err, workassist.ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, "issue_not_found", "No such issue.", nil)
	default:
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal, "The assist failed.", nil)
	}
}

// readable and readableTeam answer the Hub's own 404 when the caller cannot
// read the project or team, and write nothing when they can (tasks 138, 139).
func (a *API) readable(w http.ResponseWriter, r *http.Request, projectID string) bool {
	if err := a.deps.WorkAssist.Readable(r.Context(), projectID); err != nil {
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeProjectNotFound, "No such project.", nil)
		return false
	}
	return true
}

func (a *API) readableTeam(w http.ResponseWriter, r *http.Request, teamID string) bool {
	if err := a.deps.WorkAssist.ReadableTeam(r.Context(), teamID); err != nil {
		httpx.WriteProblem(w, http.StatusNotFound, "team_not_found", "No such team.", nil)
		return false
	}
	return true
}
```

`hubTriageSource` is exported as `HubTriageSource` for this file. `(*Service).Readable(ctx,
projectID) error` reads the project through the relay (`Hub.Project`, Phase H) and
`ReadableTeam(ctx, teamID) error` the team (`Hub.Team`), both **UNVERIFIED** method names;
task 139's `readable(ctx, projectID)` is `Readable`. `httpx.CodeInternal` and
`httpx.CodeProjectNotFound` are on this tree (`control-plane/internal/httpx/problem.go:51`,
`:58`), as is `httpx.DecodeJSON` (`:125`). `assists_off`, `gateway_unavailable`,
`issue_not_found` and `team_not_found` are string literals here, as task 24 writes the Hub's
`issue_not_found`; adding them to `httpx`'s constants is phase 2's file and out of scope.
`routes_audit_test.go` gains `"POST /v1/work/assist/triage"` in its POST list (task 24's test,
`task-24.md:67`), so the no-side-effecting-GET check (§10a) holds.

**`cmd/api` ⚑.** `PANACEA_APP_WORK_ASSIST` (`off` default, `fake`): `off` builds no service,
and the routes are absent; `fake` builds `Service` with `DirectGateway{Base: {global:
PANACEA_APP_WORK_ASSIST_UPSTREAM_GLOBAL, au: PANACEA_APP_WORK_ASSIST_UPSTREAM_AU}}` and exits 1,
naming the variable, when either base is not `http://127.0.0.1:<port>` or `http://localhost:<port>`
(decision 4: no direct path to a real provider). `PANACEA_APP_WORK_ASSIST_MODEL` defaults to
`claude-haiku-4-5`. A third value, `gateway`, is reserved for gap 1 and refused today with
*"PANACEA_APP_WORK_ASSIST=gateway needs phase 6's assist class"*. With `Deps.WorkAssist` absent
the desktop gets `404` on the route and says *"Suggestions are not set up on this machine"*.

#### Interfaces — complete: the desktop wire

```ts
// desktop/src/api/client.ts ⚑ — ProblemCode (:40-48) gains, before the client-raised pair:
  | "assists_off" | "gateway_unavailable" | "issue_not_found" | "team_not_found"

// desktop/src/api/work.ts ⚑ — appended to WORK_LOCAL (task 37, :301-307)
  ["assistTriage", "POST", "/v1/work/assist/triage"],

export type AssistProperty = "team" | "feature" | "assignee" | "labels" | "priority" | "serves";
export const ASSIST_PROPERTIES: readonly AssistProperty[] =
  ["team", "feature", "assignee", "labels", "priority", "serves"];   // no "state", ever
export type WhyKind = "issue" | "criterion" | "comment" | "guidance";
export interface Why { kind: WhyKind; key: string }
export interface AssistItem { property: AssistProperty; value: string; why: Why[] }
export interface TriageSuggestion {
  issueId: string; revision: number; route: "global" | "au"; items: AssistItem[];
}
/** Gap 2's proposed Hub record, read when it exists; absent today. */
export interface AssistCalibration {
  property: AssistProperty; unchanged: number; changed: number; rejected: number;
}

// in WorkClient
  /** Rejects with 503 assists_off, 502 gateway_unavailable, 404 issue_not_found. */
  assistTriage(issueId: string): Promise<TriageSuggestion>;
  /** Gap 2. Rejects with 404 until the Hub serves it; the dial then shows no number. */
  assistCalibration(property: AssistProperty): Promise<AssistCalibration>;

// in makeWorkClient
    assistTriage: (issueId) => local("assistTriage", [], { issueId }),
    assistCalibration: (property) =>
      send("GET", relayPath(`/v1/me/assist-calibration`) + query([["property", property]])) as
        Promise<AssistCalibration>,
```

`assistCalibration` goes through task 37's `relayPath` (the Hub's route, relayed), written as a
direct `send` rather than a `WORK_RELAY` row because the row's Hub route does not exist: adding a
row would make task 37's route-table test assert a route no Hub serves.

#### Interfaces — complete: `desktop/src/work/assist.ts`

```ts
import type { AssistCalibration, AssistItem, AssistProperty, IssueHead, Why } from "../api/work";

export type DialLevel = "hide" | "show" | "auto";
/** AX §7's names, shown beside ours (decision 10). */
export const DIAL_WORDS: Record<DialLevel, string> = {
  hide: "Hide", show: "Show · propose and wait", auto: "Auto-apply · act and tell me",
};

const KEY = (p: AssistProperty) => `panacea.assist.${p}`;
export function dialLevel(p: AssistProperty, store: Pick<Storage, "getItem"> = localStorage): DialLevel {
  const v = store.getItem(KEY(p));
  return v === "hide" || v === "auto" ? v : "show";
}
export function setDialLevel(p: AssistProperty, l: DialLevel, store: Pick<Storage, "setItem"> = localStorage): void {
  store.setItem(KEY(p), l);
}

/** An item the desktop may draw: a why that names something, on this revision. */
export function renderable(item: AssistItem, revision: number, head: IssueHead): boolean {
  return item.why.length > 0 && item.why.every((w) => w.key !== "") &&
         (item.property as string) !== "state" && head.headRevision === revision;
}

/** Empty on the head means a person has not set it (decision 12). */
export function isEmpty(p: AssistProperty, head: IssueHead, serving: number, featureId: string | null): boolean {
  switch (p) {
    case "labels": return head.labels.length === 0;
    case "assignee": return head.assignee === null;
    case "priority": return head.priority === "none";
    case "serves": return serving === 0;
    case "feature": return featureId === null;
    case "team": return false; // an issue always has a team: never auto-applied, only shown
  }
}

/** "23 unchanged, 2 changed, none rejected"; null means no record, so no number (decision 11). */
export function calibrationWords(c: AssistCalibration | null): string | null {
  if (c === null) return null;
  const n = (k: number, w: string) => (k === 0 ? `none ${w}` : `${k} ${w}`);
  return `${n(c.unchanged, "unchanged")}, ${n(c.changed, "changed")}, ${n(c.rejected, "rejected")}`;
}

/** The why as words and links: "EP-120", "C3 of Upload brief", never a number. */
export function whyLabel(w: Why, criterionName: (key: string) => string | undefined): string {
  return w.kind === "criterion" ? (criterionName(w.key) ?? w.key) : w.key;
}
```

`team` is never auto-applied: an issue always has one, so *empty* never holds, and moving an issue
to another team moves it out of this person's triage. Its dial offers *Hide* and *Show* only.

#### Interfaces — complete: `desktop/src/components/work/AssistSuggestion.tsx`

```tsx
import type { JSX } from "react";
import type { AssistItem, IssueHead } from "../../api/work";
import { renderable, whyLabel } from "../../work/assist";

export type AssistState =
  | { kind: "idle" } | { kind: "asking" }
  | { kind: "ready"; revision: number; items: AssistItem[] }
  | { kind: "failed"; words: string };   // "Suggestions unavailable: …"

/** One property's suggestions beside its picker, in the agent class. Draws nothing it cannot link. */
export function AssistSuggestion(props: {
  state: AssistState; property: AssistItem["property"]; head: IssueHead;
  valueName(value: string): string | undefined;           // the label's, person's, feature's name
  criterionName(key: string): string | undefined;
  href(w: AssistItem["why"][number]): string;               // task 44's links: issue, criterion
  onAccept(item: AssistItem): void; onDismiss(item: AssistItem): void;
}): JSX.Element | null {
  const { state, property, head } = props;
  if (state.kind === "failed") return <p className="as-note" role="status">{state.words}</p>;
  if (state.kind !== "ready") return null;       // the global heartbeat shows "asking"; no spinner here
  const mine = state.items.filter((i) => i.property === property && renderable(i, state.revision, head));
  if (mine.length === 0) return null;
  return (
    <ul className="as-list" aria-label={`Suggested ${property}`}>
      {mine.map((i) => {
        const name = props.valueName(i.value);
        if (name === undefined) return null;    // a value the replica cannot name is not drawn
        return (
          <li key={i.value} className="as-item author-agent">
            <span className="as-value">Suggested: {name}</span>
            <span className="as-why"> · because{" "}
              {i.why.map((w, n) => (
                <span key={w.kind + w.key}>{n > 0 && ", "}
                  <a href={props.href(w)}>{whyLabel(w, props.criterionName)}</a></span>
              ))}
            </span>
            <button type="button" className="as-accept" onClick={() => props.onAccept(i)}>Accept</button>
            <button type="button" className="as-dismiss" onClick={() => props.onDismiss(i)}>Dismiss</button>
          </li>
        );
      })}
    </ul>
  );
}
```

`author-agent` is task 49's class for agent-authored text (`authorClass.ts`, **UNVERIFIED**
class name; `desktop/src/components/editor/` does not exist on this tree yet). The *asking* state
is one `StatusDot` in the Triage view's header on the shared heartbeat
(`desktop/src/components/common/StatusDot.tsx`; DESIGN-SYSTEM: never a spinner per row); this
component draws nothing while asking. `.as-*` rules join task 42's stylesheet: a 1px ring, no
flat border, radius from the small-control step, `transition` on `opacity` only.

#### Interfaces — complete: `desktop/src/components/work/AssistDial.tsx`

```tsx
import { useEffect, useState, type JSX } from "react";
import { ASSIST_PROPERTIES, type AssistCalibration, type AssistProperty, type WorkClient } from "../../api/work";
import { calibrationWords, dialLevel, setDialLevel, DIAL_WORDS, type DialLevel } from "../../work/assist";

const NAME: Record<AssistProperty, string> = { team: "Team", feature: "Feature", assignee: "Assignee",
  labels: "Label", priority: "Priority", serves: "Serves" };

export function AssistDial(props: { client: WorkClient; store?: Storage }): JSX.Element {
  const store = props.store ?? localStorage;
  const [levels, setLevels] = useState(() =>
    Object.fromEntries(ASSIST_PROPERTIES.map((p) => [p, dialLevel(p, store)])) as Record<AssistProperty, DialLevel>);
  const [cal, setCal] = useState<Partial<Record<AssistProperty, AssistCalibration | null>>>({});
  useEffect(() => {
    for (const p of ASSIST_PROPERTIES)
      props.client.assistCalibration(p).then((c) => setCal((m) => ({ ...m, [p]: c })),
        () => setCal((m) => ({ ...m, [p]: null })));      // 404 today: no record, no number
  }, [props.client]);
  return (
    <table className="ad-grid" aria-label="Assists">
      <tbody>{ASSIST_PROPERTIES.map((p) => {
        const offered: DialLevel[] = p === "team" ? ["hide", "show"] : ["hide", "show", "auto"];
        const words = calibrationWords(cal[p] ?? null);
        return (
          <tr key={p}>
            <th scope="row">{NAME[p]} suggestions</th>
            <td><div role="radiogroup" aria-label={`${NAME[p]} suggestions`}>{offered.map((l) => (
              <button key={l} type="button" role="radio" aria-checked={levels[p] === l}
                onClick={() => { setDialLevel(p, l, store); setLevels((m) => ({ ...m, [p]: l })); }}>
                {DIAL_WORDS[l]}</button>))}</div>
              <p className="ad-record">{words === null
                ? `No record yet of how you answered ${NAME[p].toLowerCase()} suggestions.`
                : `${NAME[p]} suggestions you answered: ${words}`}</p></td>
          </tr>);
      })}</tbody>
    </table>
  );
}
```

Discrete named buttons, never a slider (AX §7). The inherited default (*Show*) needs no greyed
display here because the dial has one layer, the person's own (gap 4).

#### Interfaces — complete: accept, auto-apply and Undo (`desktop/src/work/assist.ts`, continued)

```ts
import type { IssueRevisionContent, WorkClient } from "../api/work";

/** The one write an Accept makes, as the person (decision 9). Team and feature are gap 6. */
export async function accept(client: WorkClient, head: IssueHead, content: IssueRevisionContent,
                             item: AssistItem, revision: number): Promise<void> {
  if (head.headRevision !== revision) throw new StaleSuggestion();
  switch (item.property) {
    case "labels":   await client.patchIssue(head.id, { labels: [...new Set([...head.labels, item.value])] }); return;
    case "priority": await client.patchIssue(head.id, { priority: item.value as IssueHead["priority"] }); return;
    case "assignee": await client.patchIssue(head.id, { assigneeId: item.value }); return;
    case "serves": {
      const [briefKey, criterionId] = splitServe(item.value);
      await client.writeIssueRevision(head.id, { baseRevision: revision,
        content: { ...content, serves: [...content.serves, { briefKey, criterionId }] } });
      return;
    }
    case "team": case "feature": throw new NoRouteYet(item.property);   // gap 6
  }
}

/** One auto-applied value, kept for DECIDED WITHOUT YOU and its Undo, this session only. */
export interface AppliedAssist {
  issueId: string; alias: string; property: AssistProperty; value: string; before: string | null;
  why: Why[]; at: string;   // the Hub's updatedAt from the write's answer, never a client clock
}

/** Auto-apply runs accept() only where the dial says so and a person has set nothing. */
export async function autoApply(client: WorkClient, head: IssueHead, content: IssueRevisionContent,
    s: TriageSuggestion, featureId: string | null, level: (p: AssistProperty) => DialLevel,
    record: (a: AppliedAssist) => void): Promise<void> {
  const once = new Set<AssistProperty>();
  for (const i of s.items) {
    if (once.has(i.property) || level(i.property) !== "auto" || !renderable(i, s.revision, head)) continue;
    if (!isEmpty(i.property, head, content.serves.length, featureId)) continue;
    once.add(i.property);                   // one value per property, then the person decides the rest
    await accept(client, head, content, i, s.revision);
    record({ issueId: head.id, alias: head.alias, property: i.property, value: i.value, before: null,
             why: i.why, at: (await client.issue(head.id)).head.updatedAt });
  }
}
```

`StaleSuggestion` and `NoRouteYet` are two `Error` subclasses whose messages are the words the
suggestion row shows (decision 9, gap 6). Auto-apply stops at the first write that throws, so a
stale head applies nothing further. **Undo** writes `before` back as the person (`labels`
without the value, priority `none`, assignee `null`, `serves` without the pair through a new
revision) and removes the row; an Undo whose property the person has since changed reads *"You
changed this since; nothing undone."* and writes nothing.

**The hosts.** `TriageView.tsx` ⚑ (task 42): when an issue gains focus, it calls
`client.assistTriage(id)` once per `(issueId, headRevision)`, holds the answer in component state,
runs `autoApply`, and passes the state to an `AssistSuggestion` beside each picker; an
`ApiError` with `assists_off` sets `failed` with *"Suggestions unavailable: assists are off for
this org"*, `gateway_unavailable` *"…: the gateway did not answer"*, a bare 404 *"Suggestions
are not set up on this machine"*. `CriterionPicker.tsx` ⚑ keeps `suggestCriteria` as it is and
lists the assist's `serves` items above it with their why; on failure the text-match notes stay.
`IssueHead.tsx` ⚑ (task 46) renders the same `AssistSuggestion` beside each field button while the
issue is in `triage`, and not otherwise. The Desk's DECIDED WITHOUT YOU (task 39's section,
`task-39.md:257`, `:274`) receives `AppliedAssist` rows through the store task 64 fills it from
(**UNVERIFIED**, no part): *"Labelled EP-150 bug · because EP-120 · Undo"*.

#### The test rig — complete: `control-plane/internal/workassist/rig_test.go`

Tasks 137-140 reuse it (`task-139.md:832-837` assumed exactly these members).

```go
package workassist

// rig is a Service over hubclient's fake and two upstreams that answer like P6
// task 16's fakeanthropic (x-fake-route: global|au, the Messages JSON shape) but
// with a body the test sets, and that keep the request bodies in memory.
// P6-16's own binary answers a fixed "fake <label> reply" and keeps hashes only
// (its _captured), so the canned body and the capture must be the rig's.
type rig struct {
	t       *testing.T
	hub     *hubclient.Fake
	svc     *Service
	mu      sync.Mutex
	body    string            // the next reply's text block
	seen    []string          // every request body, in order
	labels  []string          // the x-fake-route of every answer, in order
	sw      *fakeSwitch
	logs    *bytes.Buffer
}

func newRig(t *testing.T) *rig {
	r := &rig{t: t, hub: hubclient.NewFake(), sw: &fakeSwitch{on: true}, logs: &bytes.Buffer{}}
	up := func(label string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
			b, _ := io.ReadAll(q.Body)
			r.mu.Lock()
			r.seen, r.labels = append(r.seen, string(b)), append(r.labels, label)
			text := r.body
			r.mu.Unlock()
			w.Header().Set("x-fake-route", label)
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "message", "role": "assistant",
				"content": []map[string]string{{"type": "text", "text": text}}})
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	routes := seedRoutes(r.hub) // "pj-global" → global, "pj-au" (data class au-only) → au
	r.svc = &Service{Hub: r.hub, Switch: r.sw, Routes: routes, OrgID: "org-1", Model: "claude-haiku-4-5",
		Gateway: DirectGateway{Base: map[dataclass.Route]string{"global": up("global"), "au": up("au")},
			HTTP: &http.Client{Timeout: 5 * time.Second}},
		Log: slog.New(slog.NewTextHandler(r.logs, nil))}
	return r
}

func (r *rig) reply(text string) { r.mu.Lock(); r.body = text; r.mu.Unlock() }
func (r *rig) switchOn()         { r.sw.on = false } // the kill switch ON: orchestrators false
func (r *rig) prompts() []string { r.mu.Lock(); defer r.mu.Unlock(); return append([]string(nil), r.seen...) }
func (r *rig) routes() []string  { r.mu.Lock(); defer r.mu.Unlock(); return append([]string(nil), r.labels...) }

type fakeSwitch struct{ on bool; err error }

func (f *fakeSwitch) GetAgentSwitch(context.Context, string) (hubapi.AgentSwitch, error) {
	return hubapi.AgentSwitch{Orchestrators: f.on}, f.err
}
```

`switchOn` turns the **kill switch** on, which is `orchestrators: false` (task 100's field is
*agents allowed*). `seedRoutes` seeds two projects in the fake Hub and returns a
`wire.RouteSource` over them (P6 task 13's `File` source over a temp `projects.json`); the seed
names (`EP`, `EP-120`, `EP-131`, `EP-150`, label `bug`, brief *Upload*, criterion `C3`) are the
fixtures every assist test shares. `hubclient.NewFake` and its seeding helpers are Phase H's and
task 05's (**UNVERIFIED** names beyond `hubclient.Fake`, `hub/hubclient/fake.go` on `feat/hub`).

#### Tests to write first — Go, `workassist/{assist,why,triage}_test.go`

```go
const ep150 = "is-0000000000000150" // seeded: "Resume upload fails on Safari", in pj-au

func triage(t *testing.T, r *rig, id string) (TriageSuggestion, error) {
	t.Helper()
	return r.svc.Triage(context.Background(), HubTriageSource{Hub: r.hub}, TriageRequest{IssueID: id})
}

func TestAnItemWithNoWhyIsDropped(t *testing.T) { // kills "an unlinked suggestion"
	r := newRig(t)
	r.reply(`{"items":[{"property":"labels","value":"lb-bug","why":[]},
	                   {"property":"priority","value":"high","why":[{"kind":"issue","key":"EP-999"}]}]}`)
	s, _ := triage(t, r, ep150)
	if len(s.Items) != 0 {
		t.Errorf("items %+v: an empty why and a why naming an unlisted issue must both drop", s.Items)
	}
}

func TestACriterionOnNoBriefIsDropped(t *testing.T) { // kills "a hallucinated serves"
	r := newRig(t)
	r.reply(`{"items":[{"property":"serves","value":"c99","why":[{"kind":"criterion","key":"c99"}]},
	                   {"property":"serves","value":"c1","why":[{"kind":"criterion","key":"c1"}]}]}`)
	s, _ := triage(t, r, ep150)
	if len(s.Items) != 1 || !strings.HasSuffix(s.Items[0].Value, "#C3") {
		t.Errorf("items %+v: want only c1, resolved to the seeded brief's C3", s.Items)
	}
}

func TestStateIsNeverSuggested(t *testing.T) { // kills "a suggested fact move"
	r := newRig(t)
	r.reply(`{"items":[{"property":"state","value":"done","why":[{"kind":"issue","key":"EP-120"}]}]}`)
	if s, _ := triage(t, r, ep150); len(s.Items) != 0 {
		t.Errorf("a state item survived: %+v", s.Items)
	}
}

func TestAnAUOnlyProjectGoesToAU(t *testing.T) { // kills "Global for AU data"
	r := newRig(t)
	r.reply(`{"items":[]}`)
	s, err := triage(t, r, ep150)
	if err != nil || s.Route != "au" || strings.Join(r.routes(), ",") != "au" {
		t.Errorf("route %q, upstreams hit %v, err %v: want au only", s.Route, r.routes(), err)
	}
}

func TestTheKillSwitchStopsTheCall(t *testing.T) { // kills "switch ignored"
	r := newRig(t)
	r.switchOn()
	if _, err := triage(t, r, ep150); !errors.Is(err, ErrAssistsOff) || len(r.prompts()) != 0 {
		t.Errorf("err %v, %d calls: want ErrAssistsOff and none", err, len(r.prompts()))
	}
	r.sw.on, r.sw.err = true, errors.New("hub down") // unreadable is off (decision 3)
	if _, err := triage(t, r, ep150); !errors.Is(err, ErrAssistsOff) {
		t.Errorf("an unreadable switch let the call through: %v", err)
	}
}

func TestThePromptHoldsNoErasedText(t *testing.T) { // kills "erased text sent"
	r := newRig(t)
	eraseComment(r.hub, ep150, "SECRET-CANARY-IN-COMMENT") // an erased comment on the issue
	eraseIssue(r.hub, "is-0000000000000131")               // EP-131, an example candidate
	r.reply(`{"items":[]}`)
	_, _ = triage(t, r, ep150)
	p := strings.Join(r.prompts(), "\n")
	if strings.Contains(p, "SECRET-CANARY") || strings.Contains(p, "EP-131") {
		t.Errorf("the prompt carried erased text or an erased issue:\n%s", p)
	}
	if _, err := triage(t, r, "is-0000000000000131"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an erased issue was triaged: %v", err)
	}
}

func TestNoIssueTextIsLogged(t *testing.T) {
	r := newRig(t)
	r.reply(`{"items":[{"property":"labels","value":"lb-bug","why":[{"kind":"issue","key":"EP-120"}]}]}`)
	_, _ = triage(t, r, ep150)
	if l := r.logs.String(); strings.Contains(l, "Safari") || strings.Contains(l, "lb-bug") {
		t.Errorf("the log holds issue or reply text: %s", l)
	}
}
```

Also: `TestAGatewayFailureIsErrGatewayDown` (the upstream answers 500, then the rig's `Gateway`
is nil; both are `ErrGatewayDown` and no item is returned; kills "blank triage" at its source);
`TestAnUnknownRouteMakesNoCall` (a project missing from the route source; kills "a Global
fallback"); `TestAnUnparseableReplyKeepsNothing` (the reply `fake au reply`, P6-16's own text;
kills "a crash on prose"); `TestKeepLinkedResolvesAndDedupes` (a why twice, and one naming
another kind's key). In `api/workassist_test.go`: the route's four problem codes over a stub
`Service`; a body with a title instead of an id is `400` with no Hub call; and the route is absent
when `Deps.WorkAssist` is nil. In `cmd/api/config_test.go`: `fake` with a non-loopback upstream
exits 1 naming `PANACEA_APP_WORK_ASSIST_UPSTREAM_AU`; `gateway` exits 1 naming gap 1's words.

**One proof on phase 6's own fake.** `TestAURouteOnFakeAnthropic` (build tag `fakeanthropic`,
run by the acceptance line below) builds `control-plane/cmd/fakeanthropic` into `t.TempDir()`,
starts it twice (`-label global`, `-label au`, free ports), points a `DirectGateway` at them,
triages `EP-150`, and asserts one request reached the `au` copy (its `GET /_captured` holds one
entry, the `global` copy's none) and that the reply, P6-16's fixed text, left `items: []`.

#### Tests to write first — the desktop

- `desktop/src/work/assist.test.ts`: `renderable` is false for `why: []`, for a why with `key: ""`,
  for `property: "state"` cast in, and for `revision` behind `head.headRevision` (kills "an unlinked
  suggestion" and "a stale one applied"); `calibrationWords(null)` is `null` and
  `calibrationWords({unchanged: 23, changed: 2, rejected: 0})` is `"23 unchanged, 2 changed, none
  rejected"`, with no `of` and no `%` (kills "N of M"); `dialLevel` defaults to `show` and reads
  back what `setDialLevel` wrote (over a `Map`-backed `Storage`).
- `accept` on a fake `WorkClient`: a labels item makes exactly one `patchIssue` call, `{labels:
  [...existing, value]}`, and no other write (kills "the system as author": the only writer is the
  person's client, and the call count is one); a serves item makes one `writeIssueRevision` with
  `baseRevision` the suggestion's; a stale revision makes no call and throws `StaleSuggestion`.
- `autoApply`: with `labels` at `auto` and the issue unlabelled, one patch and one
  `AppliedAssist`; with the issue already labelled, none (kills "over a person"); with `priority`
  at `show`, none; and `ASSIST_PROPERTIES` has no `state`, so no dial row and no auto path exists
  for it (kills "a suggested fact move").
- `AssistSuggestion.test.tsx`: an item whose why is empty renders no `li` (kills "an unlinked
  suggestion"); each why is an `a` with the rig's `href`; the text has no digit followed by `%`
  and no `score`, `confidence` or `likely`; `failed` renders its words and nothing else;
  `expectNoPrediction(container)` (task 20's, `desktop/src/test/expectNoPrediction`).
- `AssistDial.test.tsx`: with `assistCalibration` rejecting `404`, the rendered text contains **no
  digit at all** (kills "a confidence without its record"); with it resolving, the row reads
  *"Label suggestions you answered: 23 unchanged, 2 changed, none rejected"*; the team row offers
  two levels, the others three; clicking *Auto-apply* stores `auto`.
- `TriageView.test.tsx` ⚑: with `assistTriage` rejecting `gateway_unavailable`, the criterion
  picker still shows task 42's `matches: …` notes and the header says *"Suggestions unavailable:
  the gateway did not answer"* (kills "blank triage"); with `assists_off`, the org words; one call
  per `(issueId, headRevision)` however often the row regains focus.

#### Steps

- [ ] **Step 1: Sync and read.** P6 tasks 01, 03, 13 and 16 and tasks 42, 46, 100 and 124 are
  merged into `feat/work-layer`; read each file this part names. Rename to their names where they
  differ, and record each rename in the commit message; the decisions do not change.
- [ ] **Step 2: The rig and the Go tests.** `go test ./internal/workassist/ ./internal/api/` fails
  on every new test, for the missing symbol.
- [ ] **Step 3: `assist.go`, `why.go`, `triage.go`.** The workassist tests pass.
- [ ] **Step 4: `api/workassist.go`,** `Deps.WorkAssist`, the register call, the audit line; then
  `cmd/api`'s three variables. The api and config tests pass.
- [ ] **Step 5: The desktop wire and `assist.ts`.** Its tests pass.
- [ ] **Step 6: `AssistSuggestion`, `AssistDial`, the three hosts.** Their tests pass.
- [ ] **Step 7: Run the acceptance block,** then commit with `git add` for the new files and
  `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
cd control-plane && go test ./internal/workassist/ ./internal/api/ ./cmd/api/
# expect: ok for each; the rows above among them
cd control-plane && go vet ./internal/workassist/ ./internal/api/ && gofmt -l internal/workassist/ internal/api/
# expect: no output
cd control-plane && go test -tags fakeanthropic -run TestAURouteOnFakeAnthropic ./internal/workassist/
# expect: ok (one request reached the au copy; none the global copy)
cd control-plane && grep -n -E 'Log\.(Info|Warn|Error)\(' internal/workassist/*.go | grep -v _test | grep -E 'Title|Description|Text|reply|body|User'
# expect: no output (decision 8)
cd control-plane && grep -n -E '"state"' internal/workassist/triage.go
# expect: no output (no property is named "state"; the prompt says "Never suggest a state." unquoted)
cd desktop && npx vitest run src/work/assist.test.ts src/components/work
# expect: assist 9 passed; AssistSuggestion 5 passed; AssistDial 4 passed; TriageView's rows plus 3 new passed
cd desktop && npx tsc --noEmit
# expect: exit 0
```

#### What this task does NOT claim

- That suggestions are good. Nothing here measures how often they are right; the calibration
  record (gap 2) will count how the person answered, which is not the same thing.
- An assist class on the gateway, or assists in any deployment but local mode on the fakes
  (decision 4, gap 1). Until phase 6 decides, the product's assists answer *"the gateway did not
  answer"*, and that is shown, not hidden.
- Spend attribution for an assist call (task 135 attributes session keys; an assist has none).
- Accepting a `team` or `feature` suggestion: both need routes this part could not verify (gap 6);
  they are shown with their why and their Accept says why it cannot write yet.
- That the dial follows the person to another machine (gap 4), or that DECIDED WITHOUT YOU keeps
  auto-applied rows across a restart: they live for the session.
- Comment text, guidance or duplicates in the prompt: tasks 137 and 139.

#### Gaps found while writing this part

1. **No gateway path for a sessionless call (proposal to the phase 6 owner, not applied).** Add
   `ClassAssist SessionClass = "assist"`: `MintRequest` with `SessionID` = `as-<person>-<project>`
   and `PodAddr` = the control plane's own address, minted on first use and refreshed like a
   session, its budget a slice of the person's (P6-D8), so the gatekeeper's pod binding and
   keysmith's attribution both hold. `cmd/api`'s `PANACEA_APP_WORK_ASSIST=gateway` is reserved
   for it. Until then assists run only against the fakes.
2. **The calibration record does not exist (a Hub delta for the skeleton).** The person's write
   needs an optional `assist {property, outcome: unchanged | changed | rejected}` datum, and the Hub
   a read, `GET /v1/me/assist-calibration?property=` → `{property, unchanged, changed, rejected}`,
   summing the caller's own data only. SW.20 does not carry it (task 114 names the same gap). It
   should be its own Hub task after task 124, with a desktop line here to send the datum on
   Accept, on a changed value, and on Dismiss.
3. **SW.19's switch has no assists member.** Task 100's `AgentSwitch` is `{orchestrators, by, at,
   reason}` (`task-100.md:314`); this part reads `orchestrators: false` as *assists off* too. If the
   owner wants the two separable, `AgentSwitch` gains `assists` and `Complete` reads it.
4. **The dial is per machine.** `localStorage` only; no Hub record holds a person's assist levels,
   and no team default exists for the greyed inheritance AX §7 describes.
5. **The file map has no row for the assists' control-plane side** (`workassist/`,
   `api/workassist.go`); this part adds them, following task 24's layout.
6. **Unverified reads and writes.** `Choices` needs task 84's team members, task 72's project
   features and Phase H's `Teams`; `Criteria` needs phase 4's brief-content decoder; accepting
   `team` and `feature` needs task 84's team move and task 83's feature assignment. None has a part
   on this tree (2026-09-26).
7. **`ProblemCode`'s line.** Task 138 cites `client.ts:59` for the union; it is `:40-48` today.
