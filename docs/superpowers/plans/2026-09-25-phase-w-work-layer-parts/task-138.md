### Task 138: assists, part 3: natural-language filters as editable chips

W4, control plane and desktop, about 2.5 hours (an estimate, not a measurement). Depends on task
91 (the view editor, *Save as view*, the replica's filter evaluation) and task 136 (the assist
client, the kill switch refusal `assists_off`, the route by the project's data class). **Neither
part is on this tree on 2026-09-26**, nor is task 83's (the desktop `ViewFilter`) or task 72's (the
Go one): every name this part takes from them is **UNVERIFIED**, listed in the table below, and
their parts win at build. The names from task 136 are the ones task 139 already assumed
(`task-139.md:79-89`), so the two parts rename together. Reads task 20's `expectNoPrediction`.

Closes **L-100** (05 §1 G, `05-design.md:234`: *"Parsed through the gateway and shown as editable
filter chips before they apply"*).

**What the reader sees** (05 §1 G L-100, §2.4; AX Principle 7):

- **The list header gains a bar**, *Filter in words*, above task 91's filter groups. Priya types
  *"bugs assigned to me updated last week"* and presses Enter.
- **The words become chips, pending.** *label is Bug* · *assignee is Priya* · *updated last week
  (21 Sep – 27 Sep)*. Pending chips have a dashed ring and the list below **does not change**. The
  typed text stays in the bar, above the chips, so the person can compare what they said with what
  was read.
- **Each chip edits and removes.** Clicking a chip opens its value in place: the values of that
  field from the team's own vocabulary, or the list of relative periods for a date. `×` removes it.
- **What was not used is said, not hidden.** A line under the chips: *"Not used: no label called
  'perf'."* One sentence per dropped phrase, each naming why.
- ***Apply* makes it an ordinary filter.** The chips become task 91's filter group, the list
  changes, and *Save as view* (task 91) keeps it. *Discard* clears the chips and leaves the text.
- **Failure keeps the words.** *"Could not read that as a filter."* with the text still in the bar
  and the list as it was; with the kill switch on, *"Filters in words are off for this org."* The
  ordinary filter groups keep working in both cases.

**Decisions made here, so nobody re-derives them:**

1. **The prompt carries the team's vocabulary and the person's words, nothing else.** State names,
   label names, member display names, cycle names and feature names of the team, each with a short
   opaque key (`s1`, `l1`, `m1`, `c1`, `f1`). No issue title, description, comment or brief text is
   read to build it. The control plane logs neither the words nor the reply.
2. **The model answers in names; code answers in ids.** The reply is `{op, clauses [{field,
   values?, within?, period?, phrase}]}`. Every value is looked up in the vocabulary by exact,
   case-insensitive name (or by its key); a value with no match is dropped and named in `dropped`
   (*"no label called 'perf'"*), and a clause left with no value is dropped whole.
3. **One flat group.** The bar yields one AND or OR group of clauses; nesting (task 91's L-99) is
   done after *Apply*, in the editor. A reply with nested groups is refused as a whole
   (*"Could not read that as a filter"*), not flattened, so no grouping is invented.
4. **Dates are relative, always.** A date clause (`updated`, `created`) carries exactly one of
   `within` (an ISO-8601 duration `P<n>D` or `P<n>W`, at most 365 days) or `period` (`today`,
   `yesterday`, `this_week`, `last_week`, `this_month`, `last_month`). A clause carrying any other
   key (`on`, `before`, `after`, `from`, `to`, `date`) is dropped with *"dates must be relative"*.
   The desktop's `resolve.ts` turns a relative clause into bounds **in the person's time zone** at
   each evaluation, so a saved view of *last week* stays *last week*.
5. **Weeks run Monday to Sunday**, the ISO week, in the person's zone; the bounds are half-open,
   `[Monday 00:00, next Monday 00:00)`. `within` is a rolling span of exact instants back from now.
6. **The route is the team's strictest project.** The vocabulary includes feature names, which
   belong to projects; the call goes on an `au-only` project's route when any project of the team
   is `au-only` (OD-19), and on the team's first project's route otherwise.
7. **The fields are the eight a person filters by most**: `state`, `label`, `assignee`, `priority`,
   `cycle`, `feature`, `updated`, `created`. Any other field is dropped with *"cannot filter by
   '<field>' in words yet"*; the editor still offers every field task 91 built.
8. **The words are 1 to 300 characters.** Longer or empty is `400 bad_request` (`httpx`'s existing
   code, `control-plane/internal/httpx/problem.go:46`), with no call to the gateway.

**What this part takes from other parts** (all **UNVERIFIED**; the owning part wins at build):

| Name | From | Shape assumed here |
|---|---|---|
| `workassist.Service`, `(*Service).Complete(ctx, projectID, Call) (Reply, error)`, `Call`, `Reply{Text, Route}`, `ErrAssistsOff`, `ErrGatewayDown` | 136 | as `task-139.md:84-88` |
| `(*API).assistProblem(w, err)` | 136 | `ErrAssistsOff` → `503 assists_off`, `ErrGatewayDown` → `502 gateway_unavailable` |
| `(*API).readableTeam(w, r, teamID) bool` | 136's helpers, extended here if absent | the Hub's own 404 for a team the caller cannot read |
| `ViewFilter`, `ViewClause` (desktop `api/work.ts`) | 83 | `{op: "and"\|"or", clauses: ViewClause[], groups?: ViewFilter[]}`; a clause `{field, values?, within?, period?, not?}` |
| `evaluateFilter(filter, issue, now, tz)` | 91 | the replica's evaluator; this part adds relative dates to it through `resolve.ts` |
| `ViewEditor` props `{filter, onChange}` | 91 | the editor's controlled filter |

If task 83 fixes a different clause shape, `nlfilter.go`'s `Clause` and `resolve.ts` change to it;
decisions 2-5 do not.

**Files:**

- `control-plane/internal/workassist/nlfilter.go` — `ParseFilter`, `Vocabulary`, the validator (new)
- `control-plane/internal/workassist/nlfilter_test.go` — the Go tests below (new)
- `control-plane/internal/workassist/testdata/nlfilter/prompt.json` — the prompt golden (new)
- `control-plane/internal/api/workassist.go` ⚑ (task 136) — `POST /v1/work/assist/filter` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the pattern listed (modified)
- `desktop/src/api/work.ts` ⚑ — `NlFilterRequest`, `NlFilterResult`, `parseFilter` (modified)
- `desktop/src/work/filters/resolve.ts` — relative dates to bounds in a time zone (new)
- `desktop/src/work/filters/resolve.test.ts` — its tests (new)
- `desktop/src/components/plan/NlFilterBar.tsx` — the bar, the pending chips, *Apply* (new)
- `desktop/src/components/plan/NlFilterBar.test.tsx` — its tests (new)
- `desktop/src/components/plan/ViewEditor.tsx` ⚑ (task 91) — mounts the bar above the groups (modified)

#### Interfaces — complete: `control-plane/internal/workassist/nlfilter.go`

```go
package workassist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Task 138: natural-language filters (05 §1 G, L-100). The model reads the
// team's vocabulary and the person's words, never an issue's text, and answers
// in names; this file turns names into ids and drops what does not exist.

// Term is one name the person may filter by, with the Hub id it stands for.
type Term struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Vocabulary is everything the prompt may carry about a team.
type Vocabulary struct {
	States, Labels, Members, Cycles, Features []Term
	Projects                                  []ProjectRoute
}

// ProjectRoute is a project of the team and its data class (OD-19).
type ProjectRoute struct {
	ID        string
	DataClass string // "global" | "au-only"
}

// VocabularySource reads a team's vocabulary as the caller, through the Hub.
type VocabularySource interface {
	Vocabulary(ctx context.Context, teamID string) (Vocabulary, error)
}

// Clause is one filter clause, the wire shape task 83's ViewClause fixes.
type Clause struct {
	Field  string   `json:"field"`
	Values []string `json:"values,omitempty"` // Hub ids, or priority words
	Within string   `json:"within,omitempty"` // P<n>D | P<n>W
	Period string   `json:"period,omitempty"`
}

// Filter is one flat group (decision 3).
type Filter struct {
	Op      string   `json:"op"` // "and" | "or"
	Clauses []Clause `json:"clauses"`
}

// Dropped names a phrase that did not become a chip, and why.
type Dropped struct {
	Phrase string `json:"phrase"`
	Reason string `json:"reason"`
}

type FilterRequest struct {
	TeamID string `json:"teamId"`
	Text   string `json:"text"`
}

type FilterResponse struct {
	Filter  Filter    `json:"filter"`
	Dropped []Dropped `json:"dropped"`
	Route   string    `json:"route"`
}

var (
	ErrTextLength  = errors.New("workassist: filter text must be 1 to 300 characters")
	ErrNotAFilter  = errors.New("workassist: the reply is not a flat filter")
	withinRe       = regexp.MustCompile(`^P([1-9][0-9]{0,2})([DW])$`)
	periods        = map[string]bool{"today": true, "yesterday": true, "this_week": true, "last_week": true, "this_month": true, "last_month": true}
	priorities     = map[string]bool{"urgent": true, "high": true, "medium": true, "low": true, "none": true}
	dateFields     = map[string]bool{"updated": true, "created": true}
	absoluteKeys   = []string{"on", "before", "after", "from", "to", "date"}
	ErrNoProject   = errors.New("workassist: the team has no project to route the call by")
)

const filterSystem = `You turn a person's words into one filter over issues.
Use only names from "vocabulary" and "priorities". Reply with JSON only, no prose:
{"op":"and"|"or","clauses":[{"field":"state"|"label"|"assignee"|"priority"|"cycle"|"feature"|"updated"|"created",
"values":["<a name>"],"within":"P<n>D"|"P<n>W","period":"today"|"yesterday"|"this_week"|"last_week"|"this_month"|"last_month",
"phrase":"<the words this clause came from>"}]}
A date clause has "within" or "period", never a calendar date. Leave out what you cannot map.`

type promptTerm struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// filterPrompt is the whole user message (decision 1). Field order is fixed so
// the golden in testdata/nlfilter/prompt.json is byte-stable.
type filterPrompt struct {
	Vocabulary struct {
		State    []promptTerm `json:"state"`
		Label    []promptTerm `json:"label"`
		Assignee []promptTerm `json:"assignee"`
		Cycle    []promptTerm `json:"cycle"`
		Feature  []promptTerm `json:"feature"`
	} `json:"vocabulary"`
	Priorities []string `json:"priorities"`
	Words      string   `json:"words"`
}

// list gives a field's terms, the key prefix and the noun a drop names. "me"
// is the first person and stays "@me": the desktop resolves it to whoever
// evaluates the view, so a saved "assigned to me" works for every reader.
func (v Vocabulary) list(field string) (terms []Term, prefix, noun string, ok bool) {
	switch field {
	case "state":
		return v.States, "s", "state", true
	case "label":
		return v.Labels, "l", "label", true
	case "assignee":
		return append([]Term{{ID: "@me", Name: "me"}}, v.Members...), "m", "person", true
	case "cycle":
		return v.Cycles, "c", "cycle", true
	case "feature":
		return v.Features, "f", "feature", true
	}
	return nil, "", "", false
}

func keyed(v Vocabulary, field string) []promptTerm {
	terms, prefix, _, _ := v.list(field)
	out := make([]promptTerm, 0, len(terms))
	for i, t := range terms {
		out = append(out, promptTerm{Key: prefix + strconv.Itoa(i+1), Name: t.Name})
	}
	return out
}

func buildFilterPrompt(v Vocabulary, words string) (string, error) {
	var p filterPrompt
	p.Vocabulary.State, p.Vocabulary.Label = keyed(v, "state"), keyed(v, "label")
	p.Vocabulary.Assignee, p.Vocabulary.Cycle = keyed(v, "assignee"), keyed(v, "cycle")
	p.Vocabulary.Feature = keyed(v, "feature")
	p.Priorities = []string{"urgent", "high", "medium", "low", "none"}
	p.Words = words
	b, err := json.Marshal(p)
	return string(b), err
}

// routeProject is decision 6: the strictest project of the team.
func routeProject(v Vocabulary) (string, error) {
	for _, p := range v.Projects {
		if p.DataClass == "au-only" {
			return p.ID, nil
		}
	}
	if len(v.Projects) == 0 {
		return "", ErrNoProject
	}
	return v.Projects[0].ID, nil
}

// ParseFilter is POST /v1/work/assist/filter. Nothing is logged, nothing written.
func (s *Service) ParseFilter(ctx context.Context, src VocabularySource, req FilterRequest) (FilterResponse, error) {
	if n := len([]rune(strings.TrimSpace(req.Text))); n == 0 || n > 300 {
		return FilterResponse{}, ErrTextLength
	}
	v, err := src.Vocabulary(ctx, req.TeamID)
	if err != nil {
		return FilterResponse{}, err
	}
	projectID, err := routeProject(v)
	if err != nil {
		return FilterResponse{}, err
	}
	user, err := buildFilterPrompt(v, req.Text)
	if err != nil {
		return FilterResponse{}, err
	}
	reply, err := s.Complete(ctx, projectID, Call{System: filterSystem, User: user, MaxTokens: 400})
	if err != nil {
		return FilterResponse{}, err // ErrAssistsOff before any call; ErrGatewayDown after
	}
	f, dropped, err := validateFilter(reply.Text, v)
	if err != nil {
		return FilterResponse{}, err
	}
	return FilterResponse{Filter: f, Dropped: dropped, Route: string(reply.Route)}, nil
}

func validateFilter(text string, v Vocabulary) (Filter, []Dropped, error) {
	var raw struct {
		Op      string            `json:"op"`
		Clauses []json.RawMessage `json:"clauses"`
		Groups  json.RawMessage   `json:"groups"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &raw) != nil || (raw.Op != "and" && raw.Op != "or") {
		return Filter{}, nil, ErrNotAFilter
	}
	if len(raw.Groups) > 0 && string(raw.Groups) != "null" {
		return Filter{}, nil, ErrNotAFilter // decision 3: refused, never flattened
	}
	f, dropped := Filter{Op: raw.Op, Clauses: []Clause{}}, []Dropped{}
	for _, rc := range raw.Clauses {
		c, ds := validateClause(rc, v)
		dropped = append(dropped, ds...)
		if c != nil {
			f.Clauses = append(f.Clauses, *c)
		}
	}
	return f, dropped, nil
}

func validateClause(rc json.RawMessage, v Vocabulary) (*Clause, []Dropped) {
	var keys map[string]json.RawMessage
	var c struct {
		Field, Within, Period, Phrase string
		Values                        []string
	}
	if json.Unmarshal(rc, &keys) != nil || json.Unmarshal(rc, &c) != nil {
		return nil, []Dropped{{Phrase: "", Reason: "not a filter clause"}}
	}
	for _, k := range absoluteKeys {
		if _, has := keys[k]; has {
			return nil, []Dropped{{Phrase: c.Phrase, Reason: "dates must be relative"}}
		}
	}
	if dateFields[c.Field] {
		return dateClause(c.Field, c.Within, c.Period, len(c.Values), c.Phrase)
	}
	if c.Field == "priority" {
		return pick(c.Field, c.Values, c.Phrase, "priority", func(s string) (string, bool) {
			s = strings.ToLower(s)
			return s, priorities[s]
		})
	}
	terms, prefix, noun, ok := v.list(c.Field)
	if !ok {
		return nil, []Dropped{{Phrase: c.Phrase, Reason: fmt.Sprintf("cannot filter by '%s' in words yet", c.Field)}}
	}
	return pick(c.Field, c.Values, c.Phrase, noun, func(s string) (string, bool) { return lookup(terms, prefix, s) })
}

// pick keeps the values that resolve and names each one that does not
// (decision 2); a clause with no value left is dropped whole.
func pick(field string, values []string, phrase, noun string, resolve func(string) (string, bool)) (*Clause, []Dropped) {
	var ids []string
	var ds []Dropped
	for _, name := range values {
		if id, ok := resolve(name); ok {
			ids = append(ids, id)
		} else {
			ds = append(ds, Dropped{Phrase: phrase, Reason: fmt.Sprintf("no %s called '%s'", noun, name)})
		}
	}
	if len(ids) == 0 {
		if len(ds) == 0 {
			ds = []Dropped{{Phrase: phrase, Reason: "no value to filter by"}}
		}
		return nil, ds
	}
	return &Clause{Field: field, Values: ids}, ds
}

// lookup matches a key the prompt gave ("l3") or an exact name, any case.
func lookup(terms []Term, prefix, s string) (string, bool) {
	for i, t := range terms {
		if s == prefix+strconv.Itoa(i+1) || strings.EqualFold(strings.TrimSpace(s), t.Name) {
			return t.ID, true
		}
	}
	return "", false
}

func dateClause(field, within, period string, nValues int, phrase string) (*Clause, []Dropped) {
	rel := []Dropped{{Phrase: phrase, Reason: "dates must be relative"}}
	if nValues > 0 || (within == "") == (period == "") {
		return nil, rel // a value is a date the model wrote; exactly one of the two is allowed
	}
	if period != "" {
		if !periods[period] {
			return nil, rel
		}
		return &Clause{Field: field, Period: period}, nil
	}
	m := withinRe.FindStringSubmatch(within)
	if m == nil {
		return nil, rel
	}
	days, _ := strconv.Atoi(m[1])
	if m[2] == "W" {
		days *= 7
	}
	if days > 365 {
		return nil, []Dropped{{Phrase: phrase, Reason: "at most 365 days back"}}
	}
	return &Clause{Field: field, Within: within}, nil
}
```

`Service` is task 136's; this file adds one method and no field. The vocabulary source is passed
in, so the tests hand in a fixed `Vocabulary` and the route hands in `HubVocabulary`.

#### Interfaces — complete: `HubVocabulary`, appended to `nlfilter.go`

```go
// HubVocabulary reads a team's vocabulary through the Hub relay as the
// caller, so a team the caller cannot read is the Hub's 404. Labels is task
// 04's method (task-04.md:208); the other five names and their element types
// are UNVERIFIED (tasks 72 and 84 for states, cycles and features, Phase H
// for members and projects) and are renamed to match at build.
type HubVocabulary struct {
	Hub interface {
		Labels(ctx context.Context, teamID string) ([]hubapi.Label, error)
		TeamStates(ctx context.Context, teamID string) ([]hubapi.TeamState, error)
		TeamMembers(ctx context.Context, teamID string) ([]hubapi.TeamMember, error)
		Cycles(ctx context.Context, teamID string) ([]hubapi.Cycle, error)
		Features(ctx context.Context, teamID string) ([]hubapi.Feature, error)
		TeamProjects(ctx context.Context, teamID string) ([]hubapi.Project, error)
	}
}

func (h HubVocabulary) Vocabulary(ctx context.Context, teamID string) (Vocabulary, error) {
	var v Vocabulary
	labels, err := h.Hub.Labels(ctx, teamID)
	if err != nil {
		return v, err
	}
	for _, l := range labels {
		if l.ArchivedAt == nil { // an archived label is not offered as a chip
			v.Labels = append(v.Labels, Term{ID: l.ID, Name: l.Name})
		}
	}
	states, err := h.Hub.TeamStates(ctx, teamID)
	if err != nil {
		return v, err
	}
	for _, s := range states {
		v.States = append(v.States, Term{ID: s.Key, Name: s.Name})
	}
	members, err := h.Hub.TeamMembers(ctx, teamID)
	if err != nil {
		return v, err
	}
	for _, m := range members {
		v.Members = append(v.Members, Term{ID: m.UserID, Name: m.DisplayName})
	}
	cycles, err := h.Hub.Cycles(ctx, teamID)
	if err != nil {
		return v, err
	}
	for _, c := range cycles {
		v.Cycles = append(v.Cycles, Term{ID: c.ID, Name: c.Name})
	}
	features, err := h.Hub.Features(ctx, teamID)
	if err != nil {
		return v, err
	}
	for _, f := range features {
		v.Features = append(v.Features, Term{ID: f.ID, Name: f.Name})
	}
	projects, err := h.Hub.TeamProjects(ctx, teamID)
	for _, p := range projects {
		v.Projects = append(v.Projects, ProjectRoute{ID: p.ID, DataClass: p.DataClass})
	}
	return v, err
}
```

The file's import list gains `"…/internal/hub/hubapi"` (the module path as task 04's files write it).
**No method reads an issue**: the vocabulary is names of states, labels, people, cycles, features
and projects, and the prompt golden is the check (decision 1).
