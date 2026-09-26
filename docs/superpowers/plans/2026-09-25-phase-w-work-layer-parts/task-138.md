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
- `desktop/src/components/plan/nlFilter.css` — the bar and the pending ring (new; not in the scope's list)
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

The file's import list gains `"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"`
(the module path is `control-plane/go.mod:1`; the `internal/hub` tree is Phase H's, **planned**).
**No method reads an issue**: the vocabulary is names of states, labels, people, cycles, features
and projects, and the prompt golden is the check (decision 1).

#### Interfaces — complete: the route, appended to `control-plane/internal/api/workassist.go` ⚑

```go
// Task 138: natural-language filters. Registered in task 136's
// registerWorkAssist beside its triage route, on *routes, so the audit sees it.
func (a *API) registerWorkFilter(rs *routes) {
	rs.HandleFunc("POST /v1/work/assist/filter", a.postWorkFilter)
}

func (a *API) postWorkFilter(w http.ResponseWriter, r *http.Request) {
	var req workassist.FilterRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !a.readableTeam(w, r, req.TeamID) {
		return
	}
	src := workassist.HubVocabulary{Hub: a.deps.WorkAssist.Hub}
	res, err := a.deps.WorkAssist.ParseFilter(r.Context(), src, req)
	switch {
	case errors.Is(err, workassist.ErrTextLength):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"the words must be 1 to 300 characters", nil)
	case errors.Is(err, workassist.ErrNoProject):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"the team has no project to route the call by", nil)
	case errors.Is(err, workassist.ErrNotAFilter):
		httpx.WriteProblem(w, http.StatusBadGateway, "gateway_unavailable",
			"the reply could not be read as a filter", nil)
	case err != nil:
		a.assistProblem(w, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}
```

`ErrNotAFilter` shares `502 gateway_unavailable` with a gateway failure on purpose: to the person
both read *"Could not read that as a filter"*, and neither leaks the model's text. The handler
logs neither the words nor the reply. `routes_audit_test.go` gains the pattern as `POST`, so §10a's
no-side-effecting-GET check holds. `a.deps.WorkAssist.Hub` is the relay task 136's `Service` holds
(`task-139.md:84`); if it is not a per-caller relay, `HubVocabulary` is given the caller's relay
instead, so the Hub reads the team as the person.

#### Interfaces — complete: `desktop/src/api/work.ts` ⚑, appended

```ts
// Task 138: natural-language filters. Mirrors control-plane/internal/workassist/nlfilter.go.
export type NlField = "state" | "label" | "assignee" | "priority" | "cycle" | "feature" | "updated" | "created";
export type RelativePeriod = "today" | "yesterday" | "this_week" | "last_week" | "this_month" | "last_month";
/** One clause; the shape is task 83's ViewClause (UNVERIFIED until 83's part). */
export interface NlClause { field: NlField; values?: string[]; within?: string; period?: RelativePeriod }
export interface NlFilter { op: "and" | "or"; clauses: NlClause[] }
export interface NlDropped { phrase: string; reason: string }
export interface NlFilterRequest { teamId: string; text: string }
export interface NlFilterResult { filter: NlFilter; dropped: NlDropped[]; route: "global" | "au" }

// WorkClient gains one call (task 37's interface; appended, not reordered).
export interface WorkFilterClient {
  parseFilter(req: NlFilterRequest): Promise<NlFilterResult>;
}
```

`WorkClient` extends `WorkFilterClient`; the call is a `POST` to the control plane's
`/v1/work/assist/filter` through task 37's client, and a problem arrives as `ApiError` with its
`code`, so the bar tells `assists_off` from every other failure by code, never by message.

#### Interfaces — complete: `desktop/src/work/filters/resolve.ts`

```ts
/**
 * Task 138: relative dates to bounds in the person's time zone (decisions 4, 5).
 * The model never supplies a date; this module computes every one, at each
 * evaluation, so a saved "last week" stays last week. Bounds are half-open.
 */
import type { NlClause, NlField, RelativePeriod } from "../../api/work";

const DAY = 86_400_000;
const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
interface LocalDate { y: number; m: number; d: number }
export interface Bounds { from: string; to: string }
export type ResolvedClause =
  | { field: "updated" | "created"; from: string; to: string }
  | { field: Exclude<NlField, "updated" | "created">; values: string[] };

/** The person's zone, from the OS; a prop overrides it in tests. */
export function personTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

function parts(instant: number, tz: string, opts: Intl.DateTimeFormatOptions): Record<string, string> {
  const f = new Intl.DateTimeFormat("en-US", { timeZone: tz, ...opts });
  return Object.fromEntries(f.formatToParts(new Date(instant)).map((p) => [p.type, p.value]));
}

/** The zone's offset from UTC at an instant, in ms (positive east of Greenwich). */
function offsetMs(instant: number, tz: string): number {
  const p = parts(instant, tz, { hourCycle: "h23", year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", second: "2-digit" });
  const wall = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second);
  return wall - Math.floor(instant / 1000) * 1000;
}

/** The UTC instant of local 00:00 on y-m-d; the offset is re-read at the result, so DST holds. */
export function zoneMidnight(x: LocalDate, tz: string): number {
  const guess = Date.UTC(x.y, x.m - 1, x.d);
  return guess - offsetMs(guess - offsetMs(guess, tz), tz);
}

function localDate(instant: number, tz: string): LocalDate & { dow: number } {
  const p = parts(instant, tz, { year: "numeric", month: "2-digit", day: "2-digit", weekday: "short" });
  return { y: +p.year, m: +p.month, d: +p.day, dow: WEEKDAYS.indexOf(p.weekday) + 1 };
}

/** Calendar arithmetic on the date alone; month and year overflow roll over. */
function addDays(x: LocalDate, n: number): LocalDate {
  const t = new Date(Date.UTC(x.y, x.m - 1, x.d + n));
  return { y: t.getUTCFullYear(), m: t.getUTCMonth() + 1, d: t.getUTCDate() };
}

export function periodBounds(period: RelativePeriod, now: Date, tz: string): Bounds {
  const t = localDate(now.getTime(), tz);
  const today: LocalDate = { y: t.y, m: t.m, d: t.d };
  const monday = addDays(today, 1 - t.dow); // ISO week: Monday is day 1
  const first: LocalDate = { y: t.y, m: t.m, d: 1 };
  const span = (a: LocalDate, b: LocalDate): Bounds => ({
    from: new Date(zoneMidnight(a, tz)).toISOString(),
    to: new Date(zoneMidnight(b, tz)).toISOString(),
  });
  switch (period) {
    case "today": return span(today, addDays(today, 1));
    case "yesterday": return span(addDays(today, -1), today);
    case "this_week": return span(monday, addDays(monday, 7));
    case "last_week": return span(addDays(monday, -7), monday);
    case "this_month": return span(first, addDays({ y: t.y, m: t.m + 1, d: 1 }, 0));
    case "last_month": return span(addDays({ y: t.y, m: t.m - 1, d: 1 }, 0), first);
  }
}

/** A rolling span of exact instants back from now; P<n>D or P<n>W only, as the server allows. */
export function withinBounds(within: string, now: Date): Bounds {
  const m = /^P([1-9][0-9]{0,2})([DW])$/.exec(within);
  if (!m) throw new Error(`not a relative span: ${within}`);
  const days = Number(m[1]) * (m[2] === "W" ? 7 : 1);
  return { from: new Date(now.getTime() - days * DAY).toISOString(), to: now.toISOString() };
}

/** One clause as the evaluator applies it: dates as bounds, "@me" as whoever evaluates. */
export function resolveClause(c: NlClause, now: Date, tz: string, me: string): ResolvedClause {
  if (c.field === "updated" || c.field === "created") {
    const b = c.period ? periodBounds(c.period, now, tz) : withinBounds(c.within ?? "", now);
    return { field: c.field, ...b };
  }
  return { field: c.field, values: (c.values ?? []).map((v) => (v === "@me" ? me : v)) };
}

/** "21 Sep – 27 Sep": the first and last local day a period covers, for the chip. */
export function shownRange(b: Bounds, tz: string): string {
  const f = new Intl.DateTimeFormat("en-AU", { timeZone: tz, day: "numeric", month: "short" });
  return `${f.format(new Date(b.from))} – ${f.format(new Date(Date.parse(b.to) - 1))}`;
}
```

Task 91's evaluator calls `resolveClause` for each clause of an applied filter with the replica's
clock and `personTimeZone()`, then compares `issue.updatedAt` (or `createdAt`) as
`from <= t < to`. The replica holds no dates of its own for a relative clause, and nothing here
writes to disk.

#### Interfaces — complete: `desktop/src/components/plan/NlFilterBar.tsx`

```tsx
/** Task 138: the words become pending chips; the list changes only on Apply (L-100). */
import { useState } from "react";
import type { NlClause, NlDropped, NlField, NlFilter, RelativePeriod, WorkFilterClient } from "../../api/work";
import { ApiError } from "../../api/client";
import { periodBounds, personTimeZone, shownRange } from "../../work/filters/resolve";
import "./nlFilter.css";

export interface Term { id: string; name: string }
export type ChipVocabulary = Record<Exclude<NlField, "priority" | "updated" | "created">, Term[]>;
export interface NlFilterBarProps {
  teamId: string;
  client: WorkFilterClient;
  vocabulary: ChipVocabulary; // from the replica, for chip names and the edit menus
  onApply(filter: NlFilter): void;
  now?: () => Date;
  timeZone?: string;
}

const PERIODS: [RelativePeriod, string][] = [["today", "today"], ["yesterday", "yesterday"],
  ["this_week", "this week"], ["last_week", "last week"], ["this_month", "this month"], ["last_month", "last month"]];
const PRIORITIES = ["urgent", "high", "medium", "low", "none"];
const isDate = (f: NlField) => f === "updated" || f === "created";
type Status = "idle" | "reading" | "pending" | "failed" | "off";

export function NlFilterBar(p: NlFilterBarProps) {
  const tz = p.timeZone ?? personTimeZone();
  const now = p.now ?? (() => new Date());
  const [text, setText] = useState("");
  const [status, setStatus] = useState<Status>("idle");
  const [pending, setPending] = useState<NlFilter | null>(null);
  const [dropped, setDropped] = useState<NlDropped[]>([]);
  const [editing, setEditing] = useState<number | null>(null);

  const nameOf = (c: NlClause, id: string) =>
    id === "@me" ? "me" : (c.field === "priority" ? id
      : p.vocabulary[c.field as keyof ChipVocabulary]?.find((t) => t.id === id)?.name ?? id);
  const label = (c: NlClause): string => {
    if (isDate(c.field)) {
      if (c.within) return `${c.field} within ${c.within.slice(1, -1)} ${c.within.endsWith("W") ? "weeks" : "days"}`;
      const words = PERIODS.find(([k]) => k === c.period)?.[1] ?? "";
      return `${c.field} ${words} (${shownRange(periodBounds(c.period!, now(), tz), tz)})`;
    }
    return `${c.field} is ${(c.values ?? []).map((v) => nameOf(c, v)).join(" or ")}`;
  };

  async function read() {
    const words = text.trim();
    if (!words) return;
    setStatus("reading");
    try {
      const res = await p.client.parseFilter({ teamId: p.teamId, text: words });
      setPending(res.filter); setDropped(res.dropped); setStatus("pending");
    } catch (e) {
      setPending(null); setDropped([]);
      setStatus(e instanceof ApiError && e.code === "assists_off" ? "off" : "failed");
    } // the text is never cleared on failure
  }
  const setClause = (i: number, c: NlClause | null) => {
    if (!pending) return;
    const clauses = pending.clauses.slice();
    if (c) clauses[i] = c; else clauses.splice(i, 1);
    setPending({ ...pending, clauses }); setEditing(null);
  };

  return (
    <section className="nl-bar" aria-label="Filter in words">
      <form onSubmit={(e) => { e.preventDefault(); void read(); }}>
        <input aria-label="Filter in words" value={text} maxLength={300}
          placeholder="Filter in words" onChange={(e) => setText(e.target.value)} />
      </form>
      {status === "reading" && <p className="nl-note">Reading</p>}
      {status === "failed" && <p className="nl-note" role="status">Could not read that as a filter.</p>}
      {status === "off" && <p className="nl-note" role="status">Filters in words are off for this org.</p>}
      {status === "pending" && pending && (
        <>
          <ul className="nl-chips" aria-label="Pending filter">
            {pending.clauses.map((c, i) => (
              <li key={i} className="nl-chip nl-chip--pending">
                <button type="button" onClick={() => setEditing(editing === i ? null : i)}>{label(c)}</button>
                <button type="button" aria-label={`Remove ${label(c)}`} onClick={() => setClause(i, null)}>×</button>
                {editing === i && <ChipEditor clause={c} vocabulary={p.vocabulary} onDone={(n) => setClause(i, n)} />}
              </li>
            ))}
          </ul>
          {dropped.length > 0 && (
            <p className="nl-note">Not used: {dropped.map((d) => d.reason).join("; ")}.</p>
          )}
          <div className="nl-actions">
            <button type="button" disabled={pending.clauses.length === 0}
              onClick={() => { p.onApply(pending); setPending(null); setDropped([]); setStatus("idle"); }}>Apply</button>
            <button type="button" onClick={() => { setPending(null); setDropped([]); setStatus("idle"); }}>Discard</button>
          </div>
        </>
      )}
    </section>
  );
}

/** Edits one chip in place: the field's own values, or a relative period. Never a free date. */
function ChipEditor(p: { clause: NlClause; vocabulary: ChipVocabulary; onDone(c: NlClause): void }) {
  const c = p.clause;
  if (isDate(c.field)) {
    return (
      <select aria-label={`${c.field} period`} value={c.period ?? ""}
        onChange={(e) => p.onDone({ field: c.field, period: e.target.value as RelativePeriod })}>
        {c.within && <option value="">within {c.within.slice(1)}</option>}
        {PERIODS.map(([k, w]) => <option key={k} value={k}>{w}</option>)}
      </select>
    );
  }
  const options = c.field === "priority" ? PRIORITIES.map((x) => ({ id: x, name: x }))
    : [...(c.field === "assignee" ? [{ id: "@me", name: "me" }] : []),
       ...p.vocabulary[c.field as keyof ChipVocabulary]];
  const chosen = new Set(c.values ?? []);
  return (
    <fieldset className="nl-edit" aria-label={`${c.field} values`}>
      {options.map((o) => (
        <label key={o.id}>
          <input type="checkbox" checked={chosen.has(o.id)} onChange={() => {
            const next = new Set(chosen);
            if (next.has(o.id)) next.delete(o.id); else next.add(o.id);
            p.onDone({ field: c.field, values: [...next] });
          }} />
          {o.name}
        </label>
      ))}
    </fieldset>
  );
}
```

A chip whose last value is unticked keeps `values: []` and *Apply* treats it as removed (task 91's
evaluator ignores an empty clause, **UNVERIFIED**; if it does not, `onApply` filters them out
first). *Reading* is plain text, not a spinner (`CLAUDE.md`: one global heartbeat, never a spinner
per row).

#### Interfaces — complete: `desktop/src/components/plan/nlFilter.css`

```css
.nl-bar { display: grid; gap: 6px; }
.nl-bar input { font: inherit; font-family: var(--sans); color: var(--ink); background: var(--surface);
  border-radius: var(--r-ctl); box-shadow: var(--e-hair); padding: 6px 8px; }
.nl-chips { display: flex; flex-wrap: wrap; gap: 4px; list-style: none; margin: 0; padding: 0; }
.nl-chip { display: inline-flex; align-items: center; border-radius: var(--r-chip);
  box-shadow: var(--e-hair); color: var(--ink-2); }
.nl-chip--pending { outline: 1px dashed var(--line-strong); outline-offset: -1px; }
.nl-chip button { font: inherit; background: none; border: 0; color: inherit; padding: 2px 6px; }
.nl-chip button:hover { background: var(--hover); transition: opacity var(--dur-fast) var(--ease); }
.nl-note { color: var(--ink-3); margin: 0; }
.nl-actions { display: flex; gap: 6px; }
```

The pending state is a dashed 1px ring; the applied filter has none, because it becomes task 91's
own group (DESIGN-SYSTEM: elevation is a ring plus shadow, never a flat border). No
`transition: all`. The tokens are the ones in `desktop/src/styles/tokens.css` (checked
2026-09-26: `--r-chip`, `--r-ctl`, `--e-hair`, `--ink-2`, `--ink-3`, `--line-strong`, `--hover`,
`--dur-fast`, `--ease`, `--surface`, `--sans`).

#### Interfaces — complete: the mount in `desktop/src/components/plan/ViewEditor.tsx` ⚑ (task 91)

```tsx
// Task 138: the bar sits above the filter groups; Apply appends one group (task 83's shape).
<NlFilterBar teamId={teamId} client={client} vocabulary={chipVocabulary(replica, teamId)}
  onApply={(nl) => onChange({ ...filter, groups: [...(filter.groups ?? []), nl] })} />
```

`chipVocabulary(replica, teamId)` is a four-line selector beside the editor: the replica's states,
labels, members, cycles and features for the team as `{id, name}`. Applying appends the words'
group to the editor's filter, so an existing filter narrows and is never replaced; *Save as view*
(task 91) then saves it like any other.

#### Tests to write first — Go, `control-plane/internal/workassist/nlfilter_test.go`

Over task 136's rig (**UNVERIFIED** `newRig(t)`, as `task-139.md:832-837`) and a fixed vocabulary
`vocab` (states *Todo*, *In review*; labels *Bug* `lb-bug`, *Docs* `lb-docs`; members *Priya*
`u-priya`, *Sam* `u-sam`; cycle *Cycle 14*; feature *Resume upload*; one project `pj-ep`, `global`). The hub fake is seeded with EP-142 titled
*"Resume upload fails on 20 MB PDFs"*, so the prompt test has an issue title to not find.

```go
func TestAnInventedLabelIsDroppedNotChipped(t *testing.T) { // kills "a chip for a label that does not exist"
	r := newRig(t)
	r.reply(`{"op":"and","clauses":[{"field":"label","values":["Bug","perf"],"phrase":"perf bugs"}]}`)
	res, err := r.svc.ParseFilter(ctx, fixedVocab(vocab), FilterRequest{TeamID: "tm-a", Text: "perf bugs"})
	must(t, err)
	mustEqual(t, []Clause{{Field: "label", Values: []string{"lb-bug"}}}, res.Filter.Clauses)
	mustEqual(t, []Dropped{{Phrase: "perf bugs", Reason: "no label called 'perf'"}}, res.Dropped)
}

func TestModelDatesAreDropped(t *testing.T) { // kills "model-supplied dates"
	r := newRig(t)
	r.reply(`{"op":"and","clauses":[{"field":"updated","after":"2026-09-21","phrase":"since Monday"},
		{"field":"created","values":["2026-09-01"],"phrase":"on the 1st"},{"field":"updated","within":"P7D","phrase":"this week"}]}`)
	res, _ := r.svc.ParseFilter(ctx, fixedVocab(vocab), FilterRequest{TeamID: "tm-a", Text: "x"})
	mustEqual(t, []Clause{{Field: "updated", Within: "P7D"}}, res.Filter.Clauses)
	mustEqual(t, 2, len(res.Dropped))
	mustEqual(t, "dates must be relative", res.Dropped[0].Reason)
}

func TestNestedGroupsAreRefused(t *testing.T) { // kills "invented grouping"
	r := newRig(t)
	r.reply(`{"op":"and","clauses":[],"groups":[{"op":"or","clauses":[]}]}`)
	_, err := r.svc.ParseFilter(ctx, fixedVocab(vocab), FilterRequest{TeamID: "tm-a", Text: "x"})
	mustErrorIs(t, err, ErrNotAFilter)
}

func TestPromptHoldsNoIssueText(t *testing.T) { // kills "issue text sent to parse a filter"
	r := newRig(t)
	r.reply(`{"op":"and","clauses":[]}`)
	_, _ = r.svc.ParseFilter(ctx, HubVocabulary{Hub: r.hub}, FilterRequest{TeamID: "tm-a", Text: "bugs for me"})
	mustNotContain(t, strings.Join(r.prompts(), "\n"), "20 MB PDFs")
	golden.Assert(t, "testdata/nlfilter/prompt.json", r.prompts()[0])
}

func TestAURouteAndKillSwitch(t *testing.T) { // kills "Global for AU data", "switch ignored"
	r := newRig(t)
	v := vocab
	v.Projects = []ProjectRoute{{ID: "pj-ep", DataClass: "global"}, {ID: "pj-hr", DataClass: "au-only"}}
	r.reply(`{"op":"and","clauses":[]}`)
	res, _ := r.svc.ParseFilter(ctx, fixedVocab(v), FilterRequest{TeamID: "tm-a", Text: "x"})
	mustEqual(t, "au", res.Route)
	mustEqual(t, 1, r.gateway.CallsOn("au"))
	r.switchOn()
	_, err := r.svc.ParseFilter(ctx, fixedVocab(v), FilterRequest{TeamID: "tm-a", Text: "x"})
	mustErrorIs(t, err, ErrAssistsOff)
	mustEqual(t, 1, r.gateway.Calls())
}

func TestMeStaysRelative(t *testing.T) { // kills "me frozen to the parser's caller"
	r := newRig(t)
	r.reply(`{"op":"and","clauses":[{"field":"assignee","values":["me"],"phrase":"mine"}]}`)
	res, _ := r.svc.ParseFilter(ctx, fixedVocab(vocab), FilterRequest{TeamID: "tm-a", Text: "mine"})
	mustEqual(t, []string{"@me"}, res.Filter.Clauses[0].Values)
}
```

`fixedVocab` is a three-line test type implementing `VocabularySource` with a fixed value. A route
test in `internal/api/workassist_test.go` adds: 301 characters answers `400 bad_request` with no
gateway call; the switch answers `503 assists_off`; an unreadable team answers the Hub's `404`; a
gateway stop answers `502 gateway_unavailable`. The house uses the standard library's `testing`;
`must*` are the rig's helpers (`task-139.md:922-923`).
