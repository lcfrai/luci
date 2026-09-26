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
