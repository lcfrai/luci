### Task 137: assists, part 2: duplicates, composer hints, guidance layers

W4, control plane and desktop; no seam. Depends on task 136 (`workassist.Service.Complete`,
`Why`, `Keys`, `keepLinked`, `registerWorkAssist`, `assistProblem`, the rig: `task-136.md`, its
table *"What this part fixes for tasks 137 to 140"*). Reads task 14's title search
(`task-14.md:284`, `hubclient.Work.SearchIssues`, `task-16.md:398`), task 37's relations
(`RelationKind`, `addRelation`, `task-37.md:95`, `:334`), task 100's `ResolveGuidance`
(`task-100.md:231-234`, `:310`), task 109's guidance types (`task-109.md:174-189`) and task 20's
`expectNoPrediction`. Estimate 0.375 (3 h).

Closes **L-54** (duplicate and related detection), **L-55** (layered guidance, most local wins)
and **L-56** (light composer suggestions), 05 §1 C.

**What the reader sees:**

- **In Triage and on a triage issue's page:** *"Possible duplicate: EP-128"*, EP-128 a link, and
  under it both titles as the Hub holds them: *"'Resume upload fails on Safari' · 'Upload resume
  broken in Safari'"*. Or *"Related: EP-131"*, the same way. No similarity score, no percentage,
  no *likely*. **Mark duplicate** and **Mark related** write one relation, as the person; neither
  closes, declines or moves either issue. Declining a duplicate stays task 42's `2` key, a
  separate act.
- **While composing a new issue:** after the title and description hold 12 characters and the
  typing has paused for 800 ms, chips appear under the editor: *"Label: bug · because EP-120"*
  and *"Possible duplicate: EP-128"*. A chip is a suggestion: clicking a label chip adds the
  label to the draft, clicking a duplicate chip opens EP-128 in a peek. Nothing is written until
  the person creates the issue.
- **Guidance is visible where it was used.** A why may read *"because team guidance v3"*, a link
  to that version in task 109's history. At feature scope the brief is the guidance: *"because
  Upload brief @4"*.
- **Failures are words:** *"Duplicate check unavailable: the gateway did not answer"*; the
  composer shows no chips and says nothing while it simply has no hint.

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 14, 16, 37, 42, 100,
109 and 136. There is **no part for task 68** (the composer, `NewIssue.tsx`,
`…-scopes/tasks-61-80.md:269-285`) or task 11's relation route beyond task 37's client
(`addRelation`, `task-37.md:428`); their names here are their scopes' and **UNVERIFIED**. Task
37's `RelationKind` is `"blocks" | "duplicates" | "relates"` (`task-37.md:95`): the scope's
*duplicate* and *related* are these two words.

**Decisions made here, so nobody re-derives them:**

1. **Retrieval is the Hub's title search, not embeddings.** Candidates are the result of
   `SearchIssues{ProjectID: the issue's project, Q: the title's terms, Limit: 20}` (task 14): the
   caller's readable issues only, in that project only, so never another team's. Nothing here
   embeds, indexes or stores text. A duplicate in another project of the same team is not found;
   that is stated, not hidden.
2. **The model judges; the Hub quotes.** The prompt lists the candidates as `k1…kN` with their
   titles. The model returns `[{key, relation: duplicates|relates}]`. The control plane maps each
   key back to the candidate and fills both titles **from the Hub's records**, never from the
   model's text, so the quote is always exactly what the issues say. A key not in the list, or a
   relation outside the two words, is dropped.
3. **An erased issue is never a candidate.** An erased head's title reads `""`
   (`task-03.md:306-307`); the candidate list drops any head whose title is empty, and the issue
   itself. An erased issue asked about is `404 issue_not_found`, as in task 136.
4. **Accepting writes a relation and nothing else.** `addRelation(issueId, {kind, issueId:
   other})` through the person's client (task 37). No state move, no decline, no close.
5. **The composer's timing is a small state machine, tested with fake timers.** A call is due when
   the draft (title plus description) has at least 12 characters and 800 ms have passed since the
   last keystroke. At most one call is in flight. A keystroke while one is in flight marks its
   answer stale; a stale answer is discarded when it lands, and the next call waits for the pause
   again. Typing 30 characters in a burst, pausing, typing more and pausing makes two calls.
6. **The composer's hint call carries the draft, which has no author yet.** It is the person's
   text, typed now, sent to their project's route under the kill switch like every assist. It is
   never stored: the control plane keeps nothing, and the desktop drops the hints when the
   composer closes.
7. **Guidance layers, most local last.** `guidanceFor(ctx, scope)` returns
   `[{level, key, text}]` ordered org, team, person (task 100's `ResolveGuidance`, which answers
   most local first, reversed), then, at feature scope, the feature's brief at its current
   revision. The last entry is marked `winning: true`. SW.19 has **no project layer**
   (`GuidanceScope = "org" | "team" | "person"`, `task-109.md:176`); the scope's `project` level is
   not invented here (gap 1).
8. **Guidance enters an assist's prompt, and only its prompt.** Each assist's prompt gains the
   layers with keys `g1…gN`, and `Keys` maps each to its link key (`gd-…@v<version>`, or the brief
   key), so a why may cite the guidance it used. Guidance is person-written, `text·H`
   (`task-109.md:16-17`). Whether guidance may enter a *session agent's* context is task 99's
   not-claim and stays so (gap 4).
9. **Numbers are counts of things, never of confidence.** No hint, chip or row shows a score, a
   percentage, a rank number or an ordinal (*"top match"*) (AX anti-pattern 12).

**Files:**

- `control-plane/internal/workassist/guidance.go` — `GuidanceLevel`, `GuidanceEntry`,
  `GuidanceScope`, `(*Service).GuidanceFor`, `withGuidance` (new)
- `control-plane/internal/workassist/duplicates.go` — `DuplicatesRequest`, `Candidate`,
  `Duplicates`, `(*Service).Duplicates`, `candidates` (new)
- `control-plane/internal/workassist/hints.go` — `HintsRequest`, `Hints`, `(*Service).Hints` (new)
- `control-plane/internal/workassist/{guidance,duplicates,hints}_test.go` (new), over task 136's rig
- `control-plane/internal/workassist/triage.go` ⚑ (task 136) — the prompt gains the guidance
  layers through `withGuidance` (modified; the scope's file list omits it, gap 5)
- `control-plane/internal/api/workassist.go` ⚑ (task 136) — `registerWorkDuplicates`, two routes
  (modified); `routes_audit_test.go` ⚑ — the two patterns as `POST` (modified)
- `desktop/src/api/work.ts` ⚑ — two `WORK_LOCAL` rows, the wire types, two client methods
  (modified)
- `desktop/src/work/hintClock.ts`, `hintClock.test.ts` — the composer's timing, decision 5 (new)
- `desktop/src/components/work/DuplicateHint.tsx`, `.test.tsx` (new)
- `desktop/src/components/work/ComposerHints.tsx`, `.test.tsx` (new)
- `desktop/src/components/work/NewIssue.tsx` ⚑ (task 68, no part) and `TriageView.tsx` ⚑
  (task 42) — the hosts (modified)

#### Interfaces — complete: `control-plane/internal/workassist/guidance.go`

```go
package workassist

// GuidanceLevel is where a guidance entry comes from. SW.19 has org, team and
// person (task-109.md:176); brief is the feature's brief (05 §1 C L-55).
// There is no project level (gap 1).
type GuidanceLevel string

const (
	LevelOrg    GuidanceLevel = "org"
	LevelTeam   GuidanceLevel = "team"
	LevelPerson GuidanceLevel = "person"
	LevelBrief  GuidanceLevel = "brief"
)

// GuidanceEntry is one layer. Key is the link: "gd-…@v3" for a guidance
// version, the brief key "brief:bf-…@4" for a brief. Winning is set on the last.
type GuidanceEntry struct {
	Level   GuidanceLevel `json:"level"`
	Key     string        `json:"key"`
	Text    string        `json:"text"`
	Winning bool          `json:"winning"`
}

// GuidanceScope is what the guidance is for: always a team, and a feature
// when the work is under one.
type GuidanceScope struct {
	TeamID    string
	FeatureID string // "" for none
}

// GuidanceFor returns the layers most local last, the last marked winning.
// A layer the Hub does not hold is absent, not empty.
func (s *Service) GuidanceFor(ctx context.Context, sc GuidanceScope) ([]GuidanceEntry, error) {
	gv, err := s.Hub.ResolveGuidance(ctx, sc.TeamID) // most local first: person, team, org
	if err != nil {
		return nil, err
	}
	var out []GuidanceEntry
	for i := len(gv.Layers) - 1; i >= 0; i-- {
		g := gv.Layers[i]
		out = append(out, GuidanceEntry{Level: GuidanceLevel(g.Scope),
			Key: fmt.Sprintf("%s@v%d", g.ID, g.Version), Text: g.Text})
	}
	if sc.FeatureID != "" {
		if b, ok, err := s.featureBrief(ctx, sc.FeatureID); err != nil {
			return nil, err
		} else if ok {
			out = append(out, b)
		}
	}
	if len(out) > 0 {
		out[len(out)-1].Winning = true
	}
	return out, nil
}

// withGuidance adds the layers to a prompt as g1…gN and their keys to k, so
// each assist's why may cite the guidance it used (decision 8).
func withGuidance(k Keys, layers []GuidanceEntry) []promptGuidance {
	out := make([]promptGuidance, 0, len(layers))
	for i, g := range layers {
		key := fmt.Sprintf("g%d", i+1)
		k.add(WhyGuidance, key, g.Key)
		out = append(out, promptGuidance{Key: key, Level: string(g.Level), Text: g.Text, Winning: g.Winning})
	}
	return out
}

type promptGuidance struct {
	Key, Level, Text string
	Winning          bool
}
```

`featureBrief` reads the feature (task 83's `Feature`, its `briefId`; **UNVERIFIED**) and the
brief's current revision through `Brief` (`hub/hubclient/hubclient.go:57` on `feat/hub`), and
renders its criteria as the text with phase 4's decoder (task 136's gap 6). An erased or
unreadable brief is absent (`ok == false`), never an error that stops the assist.
`hubapi.GuidanceView{Layers []Guidance}` and `Guidance{ID, Scope, ScopeID, Text, Version}` are
task 100's (`task-100.md:231-234`, `:310`); the Go field names are **UNVERIFIED** there.

**`triage.go` ⚑ (task 136).** `triagePrompt` gains `Guidance []promptGuidance`; `Triage`, after
the criteria, calls `s.GuidanceFor(ctx, GuidanceScope{TeamID: iw.Head.TeamID, FeatureID:
iw.Head.FeatureID})` and sets `p.Guidance = withGuidance(keys, layers)`. A guidance read that
fails leaves `p.Guidance` empty and does not fail the assist. `triageSystem` gains one line:
*"Guidance lists rules, most local last; the last one wins where they differ. A why may name a
Guidance key."* `iw.Head.TeamID` and `FeatureID` are task 83's head fields (**UNVERIFIED**).

#### Interfaces — complete: `control-plane/internal/workassist/duplicates.go`

```go
package workassist

// DuplicatesRequest is POST /v1/work/assist/duplicates's body: an existing issue.
type DuplicatesRequest struct {
	IssueID string `json:"issueId"`
}

// Candidate is one issue the model judged a duplicate or related. Both titles
// are the Hub's own text (decision 2); the model supplies only Key and Relation.
type Candidate struct {
	IssueID   string `json:"issueId"`
	Alias     string `json:"alias"`
	Relation  string `json:"relation"`  // "duplicates" | "relates" (task 37's RelationKind)
	ThisTitle string `json:"thisTitle"` // the asked-about issue's, or the draft's
	ThatTitle string `json:"thatTitle"`
	Why       []Why  `json:"why"`       // at least the candidate itself; guidance may follow
}

// Duplicates is the 200 body for both routes. Revision is 0 for a draft.
type Duplicates struct {
	IssueID    string          `json:"issueId"`
	Revision   int             `json:"revision"`
	Candidates []Candidate     `json:"candidates"` // [] when none, never null
	Route      dataclass.Route `json:"route"`
}

// candidates is the title search, narrowed: this project, readable, not
// erased (empty title), not the issue itself; at most 20 (decisions 1 and 3).
func candidates(ctx context.Context, hub TriageSource, projectID, title, self string) ([]hubapi.IssueHead, error) {
	hs, err := hub.SearchIssues(ctx, hubapi.IssueSearchQuery{ProjectID: projectID, Q: searchTerms(title), Limit: 20})
	if err != nil {
		return nil, err
	}
	out := hs[:0:0]
	for _, h := range hs {
		if h.ID != self && h.Title != "" && h.ProjectID == projectID {
			out = append(out, h)
		}
	}
	return out, nil
}

const duplicatesSystem = `You compare one work item with a list of others. Reply with JSON only:
{"candidates":[{"key":"k3","relation":"duplicates|relates","why":[{"kind":"guidance","key":"g1"}]}]}
"duplicates" means the same problem or request; "relates" means connected but different.
Name only keys from Others. List nothing you are unsure of. Never give a score.`

// judge asks the model once and keeps each well-formed, listed, distinct pick.
func (s *Service) judge(ctx context.Context, projectID, title, desc string, others []hubapi.IssueHead,
	guidance []GuidanceEntry) ([]Candidate, dataclass.Route, error) {
	if len(others) == 0 {
		return []Candidate{}, "", nil // no candidates: no call, and no cost
	}
	keys := Keys{}
	byKey := map[string]hubapi.IssueHead{}
	type other struct{ Key, Title string }
	var list []other
	for i, h := range others {
		k := fmt.Sprintf("k%d", i+1)
		byKey[k] = h
		list = append(list, other{k, h.Title})
	}
	user, err := json.Marshal(struct {
		Title, Description string
		Others             []other
		Guidance           []promptGuidance
	}{title, desc, list, withGuidance(keys, guidance)})
	if err != nil {
		return nil, "", err
	}
	reply, err := s.Complete(ctx, projectID, Call{System: duplicatesSystem, User: string(user), MaxTokens: 400})
	if err != nil {
		return nil, "", err
	}
	return pickCandidates(reply.Text, byKey, keys, title), reply.Route, nil
}

// pickCandidates maps keys back to the Hub's heads and fills the titles from
// them. The candidate's own alias is always its first why.
func pickCandidates(text string, byKey map[string]hubapi.IssueHead, keys Keys, thisTitle string) []Candidate {
	var raw struct {
		Candidates []struct {
			Key, Relation string
			Why           []Why
		} `json:"candidates"`
	}
	out := []Candidate{}
	if json.Unmarshal([]byte(text), &raw) != nil {
		return out
	}
	seen := map[string]bool{}
	for _, c := range raw.Candidates {
		h, ok := byKey[c.Key]
		if !ok || seen[h.ID] || (c.Relation != "duplicates" && c.Relation != "relates") {
			continue
		}
		seen[h.ID] = true
		why := append([]Why{{Kind: WhyIssue, Key: h.Alias}}, keepLinked(keys, c.Why)...)
		out = append(out, Candidate{IssueID: h.ID, Alias: h.Alias, Relation: c.Relation,
			ThisTitle: thisTitle, ThatTitle: h.Title, Why: why})
	}
	return out
}

// gate is the kill switch alone, read before any Hub search, so a route under
// the switch answers assists_off even when it would have made no call.
func (s *Service) gate(ctx context.Context) error {
	sw, err := s.Switch.GetAgentSwitch(ctx, s.OrgID)
	if err != nil || !sw.Orchestrators {
		return ErrAssistsOff
	}
	return nil
}

// Duplicates is the triage path: an existing, readable, unerased issue.
func (s *Service) Duplicates(ctx context.Context, src TriageSource, req DuplicatesRequest) (Duplicates, error) {
	if err := s.gate(ctx); err != nil {
		return Duplicates{}, err
	}
	iw, err := src.Issue(ctx, req.IssueID)
	if err != nil || iw.Revision.ErasedAt != nil {
		return Duplicates{}, ErrNotFound
	}
	c := iw.Revision.Content
	pid := iw.Head.ProjectID
	others, err := candidates(ctx, src, pid, c.Title.Text, req.IssueID)
	if err != nil {
		return Duplicates{}, ErrGatewayDown // a failed search reads as "unavailable", in words
	}
	g, _ := s.GuidanceFor(ctx, GuidanceScope{TeamID: iw.Head.TeamID, FeatureID: iw.Head.FeatureID})
	picked, route, err := s.judge(ctx, pid, c.Title.Text, c.Description.Text, others, g)
	if err != nil {
		return Duplicates{}, err
	}
	s.Log.Info("duplicates assist", "issueId", req.IssueID, "candidates", len(others), "picked", len(picked))
	return Duplicates{IssueID: req.IssueID, Revision: iw.Revision.Revision, Candidates: picked, Route: route}, nil
}
```

`ErrGatewayDown` for a failed search is deliberate: the person reads *"Duplicate check
unavailable"* either way, and the Hub's error text never reaches them through this route.

#### Interfaces — complete: `control-plane/internal/workassist/hints.go`

```go
package workassist

// HintsRequest is POST /v1/work/assist/hints's body: the composer's draft.
// Nothing here is stored; the draft is the person's own text, typed now.
type HintsRequest struct {
	ProjectID   string `json:"projectId"`
	Title       string `json:"title"`       // 1-200 characters
	Description string `json:"description"` // 0-4000 characters; more is cut, never sent whole
}

// Hints is the 200 body: label chips and duplicate chips, each linked.
type Hints struct {
	Labels     []Item          `json:"labels"`     // property "labels" only
	Candidates []Candidate     `json:"candidates"` // as Duplicates'
	Route      dataclass.Route `json:"route"`
}

var ErrDraftLength = errors.New("workassist: the draft's title must be 1 to 200 characters")

// Hints makes at most two calls: the duplicate judgement and one labels-only
// triage prompt. It never writes, and it keeps no copy of the draft.
func (s *Service) Hints(ctx context.Context, src TriageSource, teamID string, req HintsRequest) (Hints, error) {
	if n := utf8.RuneCountInString(req.Title); n < 1 || n > 200 {
		return Hints{}, ErrDraftLength
	}
	if err := s.gate(ctx); err != nil {
		return Hints{}, err
	}
	desc := firstRunes(req.Description, 4000)
	others, err := candidates(ctx, src, req.ProjectID, req.Title, "")
	if err != nil {
		return Hints{}, ErrGatewayDown
	}
	g, _ := s.GuidanceFor(ctx, GuidanceScope{TeamID: teamID})
	picked, route, err := s.judge(ctx, req.ProjectID, req.Title, desc, others, g)
	if err != nil {
		return Hints{}, err
	}
	labels, lr, err := s.labelsFor(ctx, src, req.ProjectID, req.Title, desc, g)
	if err != nil {
		return Hints{}, err
	}
	if route == "" {
		route = lr
	}
	return Hints{Labels: labels, Candidates: picked, Route: route}, nil
}
```

`labelsFor` is task 136's `Triage` narrowed to one property: the same vocabulary for `labels`
only, the same examples, `validateItems` with `allowed` holding labels alone, over a draft in
place of an issue revision. It is a split of `Triage`'s body into a helper both call, so 136's
tests keep passing unchanged. `firstRunes` cuts at a rune boundary. `teamID` is the project's
team, which the handler reads with `(*Service).ProjectTeam(ctx, projectID) (string, error)`,
added in `hints.go` over the relay's project read (Phase H's `Project`, **UNVERIFIED** name); an
unreadable project is `404 project_not_found`. The caller never supplies the team.

#### Interfaces — complete: the routes, appended to `control-plane/internal/api/workassist.go` ⚑

```go
// Task 137. Called from task 136's registerWorkAssist, after its triage route.
func (a *API) registerWorkDuplicates(rs *routes) {
	rs.HandleFunc("POST /v1/work/assist/duplicates", a.postWorkDuplicates)
	rs.HandleFunc("POST /v1/work/assist/hints", a.postWorkHints)
}

func (a *API) postWorkDuplicates(w http.ResponseWriter, r *http.Request) {
	var req workassist.DuplicatesRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.IssueID == "" || (wire.WorkRef{IssueID: req.IssueID}).Validate() != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "issueId is not an issue id", nil)
		return
	}
	res, err := a.deps.WorkAssist.Duplicates(r.Context(), workassist.HubTriageSource{Hub: a.deps.WorkAssist.Hub}, req)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a *API) postWorkHints(w http.ResponseWriter, r *http.Request) {
	var req workassist.HintsRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	team, err := a.deps.WorkAssist.ProjectTeam(r.Context(), req.ProjectID)
	if err != nil {
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeProjectNotFound, "No such project.", nil)
		return
	}
	res, err := a.deps.WorkAssist.Hints(r.Context(), workassist.HubTriageSource{Hub: a.deps.WorkAssist.Hub}, team, req)
	if errors.Is(err, workassist.ErrDraftLength) {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "The title must be 1 to 200 characters.", nil)
		return
	}
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
```

Both are `POST` (they send text, they are not reads a proxy may cache or a link may fire), listed
so in `routes_audit_test.go`. Neither handler logs the draft, the titles or the reply.

```jsonc
// POST /v1/work/assist/duplicates   { "issueId": "is-0000000000000150" }
// POST /v1/work/assist/hints        { "projectId": "pj-…", "title": "Upload resume broken", "description": "" }
// 200 (duplicates)
{ "issueId": "is-0000000000000150", "revision": 2, "route": "au",
  "candidates": [ { "issueId": "is-0000000000000128", "alias": "EP-128", "relation": "duplicates",
      "thisTitle": "Resume upload fails on Safari", "thatTitle": "Upload resume broken in Safari",
      "why": [ { "kind": "issue", "key": "EP-128" }, { "kind": "guidance", "key": "gd-…@v3" } ] } ] }
// 200 (hints): { "labels": [Item], "candidates": [Candidate], "route": "au" }
// 400 bad_request · 404 issue_not_found | project_not_found · 503 assists_off · 502 gateway_unavailable
```

#### Interfaces — complete: the desktop wire (`desktop/src/api/work.ts` ⚑)

```ts
// appended to WORK_LOCAL
  ["assistDuplicates", "POST", "/v1/work/assist/duplicates"],
  ["assistHints", "POST", "/v1/work/assist/hints"],

export interface DuplicateCandidate {
  issueId: string; alias: string; relation: "duplicates" | "relates";
  thisTitle: string; thatTitle: string; why: Why[];
}
export interface Duplicates {
  issueId: string; revision: number; route: "global" | "au"; candidates: DuplicateCandidate[];
}
export interface HintsRequest { projectId: string; title: string; description: string }
export interface Hints { labels: AssistItem[]; candidates: DuplicateCandidate[]; route: "global" | "au" }

// in WorkClient
  assistDuplicates(issueId: string): Promise<Duplicates>;
  assistHints(req: HintsRequest): Promise<Hints>;
// in makeWorkClient
    assistDuplicates: (issueId) => local("assistDuplicates", [], { issueId }),
    assistHints: (req) => local("assistHints", [], req),
```

`Why`, `AssistItem` and the two problem codes are task 136's. `Why.kind` already admits
`"guidance"`; the desktop links a guidance key `gd-…@v3` to task 109's history row for that
version (`AgentGuidance.tsx`, `task-109.md:66`) and a brief key to the brief view.

#### Interfaces — complete: `desktop/src/work/hintClock.ts`

```ts
/** The composer's timing (decision 5): at least MIN_CHARS, PAUSE_MS still, one call in flight,
 *  stale answers dropped. Timers are injected so the tests drive them (vi.useFakeTimers). */
export const MIN_CHARS = 12;
export const PAUSE_MS = 800;

export interface HintClock {
  /** Every keystroke: the whole draft as it now reads. */
  typed(draft: { title: string; description: string }): void;
  /** The composer closed: nothing further is called or delivered. */
  close(): void;
}

export function hintClock<T>(opts: {
  ask(draft: { title: string; description: string }): Promise<T>;
  deliver(answer: T, forDraft: { title: string; description: string }): void;
  fail(e: unknown): void;
  setTimer?: (f: () => void, ms: number) => unknown;
  clearTimer?: (t: unknown) => void;
}): HintClock {
  const setT = opts.setTimer ?? ((f, ms) => setTimeout(f, ms));
  const clearT = opts.clearTimer ?? ((t) => clearTimeout(t as ReturnType<typeof setTimeout>));
  let timer: unknown = null;
  let latest = { title: "", description: "" };
  let version = 0;            // bumped on every keystroke
  let inFlight = false;
  let closed = false;
  let pending = false;        // the pause ended while a call was in flight

  const fire = () => {
    timer = null;
    if (closed) return;
    if (inFlight) { pending = true; return; }                       // one call in flight, at most
    if ((latest.title + latest.description).length < MIN_CHARS) return;
    const asked = version, draft = latest;
    inFlight = true;
    opts.ask(draft).then(
      (a) => { if (!closed && asked === version) opts.deliver(a, draft); },   // stale: dropped
      (e) => { if (!closed && asked === version) opts.fail(e); },
    ).finally(() => {
      inFlight = false;
      if (pending && !closed) { pending = false; timer = setT(fire, PAUSE_MS); }
    });
  };

  return {
    typed(draft) {
      if (closed) return;
      latest = draft; version += 1; pending = false;
      if (timer !== null) clearT(timer);
      timer = setT(fire, PAUSE_MS);
    },
    close() { closed = true; if (timer !== null) clearT(timer); timer = null; },
  };
}
```

A keystroke while a call is in flight bumps `version`, so that call's answer is dropped when it
lands; the pause timer still runs, and if it ends before the call settles, `pending` makes one
more call after it, after a fresh pause. The call itself is not aborted: task 37's `WorkSend`
takes no signal (`task-37.md:399`), so dropping is the honest cancel (gap 3).

#### Interfaces — complete: `desktop/src/components/work/DuplicateHint.tsx`

```tsx
import type { JSX } from "react";
import type { DuplicateCandidate } from "../../api/work";

const WORD: Record<DuplicateCandidate["relation"], string> = { duplicates: "Possible duplicate", relates: "Related" };

/** One candidate, its why as links, both titles quoted from the Hub. No score, ever. */
export function DuplicateHint(props: {
  c: DuplicateCandidate; href(key: string, kind: string): string;
  onMark?(c: DuplicateCandidate): void;   // absent in the composer: nothing to relate yet
}): JSX.Element | null {
  const { c } = props;
  if (c.why.length === 0 || c.thatTitle === "") return null;   // unlinked or erased: not drawn
  return (
    <div className="dh-row author-agent" data-relation={c.relation}>
      <span>{WORD[c.relation]}: <a href={props.href(c.alias, "issue")}>{c.alias}</a></span>
      <q className="dh-quote">{c.thisTitle}</q><span aria-hidden="true"> · </span>
      <q className="dh-quote">{c.thatTitle}</q>
      {c.why.slice(1).map((w) => (
        <span key={w.key} className="dh-why"> · because <a href={props.href(w.key, w.kind)}>{w.key}</a></span>
      ))}
      {props.onMark && (
        <button type="button" onClick={() => props.onMark!(c)}>
          {c.relation === "duplicates" ? "Mark duplicate" : "Mark related"}
        </button>
      )}
    </div>
  );
}
```

`onMark` is `(c) => client.addRelation(issueId, { kind: c.relation, issueId: c.issueId })` in the
host, and nothing else (decision 4). A guidance why shows as its key today (*"gd-…@v3"*); task 109's
`LAYER_WORD` (`task-109.md:299`) names it *"team guidance v3"* once the host resolves the layer.

#### Interfaces — complete: `desktop/src/components/work/ComposerHints.tsx`

```tsx
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { ApiError } from "../../api/client";
import type { Hints, WorkClient } from "../../api/work";
import { hintClock } from "../../work/hintClock";
import { DuplicateHint } from "./DuplicateHint";

/** Chips under the composer's editor. Nothing here writes to the Hub. */
export function ComposerHints(props: {
  client: WorkClient; projectId: string; title: string; description: string;
  labelName(id: string): string | undefined;
  href(key: string, kind: string): string;
  onAddLabel(labelId: string): void;           // adds to the draft only (decision 6)
}): JSX.Element | null {
  const [hints, setHints] = useState<Hints | null>(null);
  const [words, setWords] = useState<string | null>(null);
  const { client, projectId } = props;
  const clock = useMemo(() => hintClock<Hints>({
    ask: (d) => client.assistHints({ projectId, title: d.title, description: d.description }),
    deliver: (h) => { setHints(h); setWords(null); },
    fail: (e) => {
      setHints(null);
      const code = e instanceof ApiError ? e.code : "";
      // The composer is quiet about a missing hint; it speaks only when assists are off.
      setWords(code === "assists_off" ? "Hints unavailable: assists are off for this org" : null);
    },
  }), [client, projectId]);
  const first = useRef(true);
  useEffect(() => {
    if (first.current) { first.current = false; return; }   // opening the composer is not typing
    clock.typed({ title: props.title, description: props.description });
  }, [clock, props.title, props.description]);
  useEffect(() => () => clock.close(), [clock]);

  if (words !== null) return <p className="ch-note" role="status">{words}</p>;
  if (hints === null) return null;
  const labels = hints.labels.filter((i) => i.why.length > 0 && props.labelName(i.value) !== undefined);
  if (labels.length === 0 && hints.candidates.length === 0) return null;
  return (
    <div className="ch-chips" aria-label="Hints">
      {labels.map((i) => (
        <button key={i.value} type="button" className="ch-chip author-agent"
          onClick={() => props.onAddLabel(i.value)}>
          Label: {props.labelName(i.value)} · because {i.why.map((w) => w.key).join(", ")}
        </button>
      ))}
      {hints.candidates.map((c) => <DuplicateHint key={c.issueId} c={c} href={props.href} />)}
    </div>
  );
}
```

**The hosts.** `NewIssue.tsx` ⚑ (task 68's composer, no part: its props are its scope's,
**UNVERIFIED**) renders `ComposerHints` under the editor with the draft's title and description
as plain text and `onAddLabel` adding to the draft's labels; closing the composer unmounts it, and
the clock's `close` drops any answer in flight. `TriageView.tsx` ⚑ (task 42) calls
`client.assistDuplicates(id)` once per `(issueId, headRevision)`, beside task 136's triage call,
and renders one `DuplicateHint` per candidate with `onMark` wired to `addRelation`; a rejection
shows *"Duplicate check unavailable: the gateway did not answer"* or the org words, and the rest
of Triage is untouched. Task 42's `2` (decline as duplicate) stays the person's separate key.

#### Tests to write first — Go, over task 136's rig

The rig seeds project `EP` in team `tm-ep` with EP-120, EP-128, EP-131 and EP-150, and a second
team `tm-ops` whose project `OPS` holds OPS-7 titled *"Upload resume broken in Safari"*, readable
by another person only.

```go
func TestCandidatesAreReadableOnly(t *testing.T) { // kills "cross-team text in the prompt"
	r := newRig(t)
	r.reply(`{"candidates":[]}`)
	_, _ = r.svc.Duplicates(ctx, HubTriageSource{Hub: r.hub}, DuplicatesRequest{IssueID: ep150})
	if p := strings.Join(r.prompts(), "\n"); strings.Contains(p, "OPS-7") || strings.Count(p, `"Key":"k`) > 20 {
		t.Errorf("the prompt holds another team's issue or more than 20 candidates:\n%s", p)
	}
}

func TestAnErasedIssueIsNeverACandidate(t *testing.T) { // kills "erased text resurfaces"
	r := newRig(t)
	eraseIssue(r.hub, "is-0000000000000128") // EP-128
	r.reply(`{"candidates":[{"key":"k1","relation":"duplicates"}]}`)
	d, _ := r.svc.Duplicates(ctx, HubTriageSource{Hub: r.hub}, DuplicatesRequest{IssueID: ep150})
	for _, c := range d.Candidates {
		if c.Alias == "EP-128" {
			t.Error("an erased issue came back as a candidate")
		}
	}
	if strings.Contains(strings.Join(r.prompts(), ""), "EP-128") {
		t.Error("an erased issue's key reached the prompt")
	}
}

func TestTitlesComeFromTheHub(t *testing.T) { // kills "the model's words quoted as the issue's"
	r := newRig(t)
	r.reply(`{"candidates":[{"key":"k1","relation":"duplicates","thatTitle":"INVENTED"}]}`)
	d, _ := r.svc.Duplicates(ctx, HubTriageSource{Hub: r.hub}, DuplicatesRequest{IssueID: ep150})
	if len(d.Candidates) != 1 || d.Candidates[0].ThatTitle == "INVENTED" || d.Candidates[0].ThatTitle == "" {
		t.Errorf("candidates %+v: want one, titled from the Hub", d.Candidates)
	}
}

func TestProjectGuidanceWins(t *testing.T) { // kills "org wins"
	r := newRig(t)
	putGuidance(r.hub, "org", "or-1", "Label every crash 'crash'.")
	putGuidance(r.hub, "team", "tm-ep", "Label crashes 'bug'.")
	g, err := r.svc.GuidanceFor(ctx, GuidanceScope{TeamID: "tm-ep", FeatureID: "ft-ep"})
	if err != nil || len(g) != 3 {
		t.Fatalf("layers %+v, err %v: want org, team, the brief", g, err)
	}
	if g[0].Level != LevelOrg || g[1].Level != LevelTeam || g[2].Level != LevelBrief ||
		!g[2].Winning || g[0].Winning || g[1].Winning {
		t.Errorf("order or winner wrong: %+v", g)
	}
	g2, _ := r.svc.GuidanceFor(ctx, GuidanceScope{TeamID: "tm-ep"})
	if !g2[len(g2)-1].Winning || g2[len(g2)-1].Level != LevelTeam {
		t.Errorf("without a feature the team layer must win: %+v", g2)
	}
}

func TestHintsRefuseUnderTheKillSwitch(t *testing.T) { // kills "switch ignored" on the new routes
	r := newRig(t)
	r.switchOn()
	_, e1 := r.svc.Duplicates(ctx, HubTriageSource{Hub: r.hub}, DuplicatesRequest{IssueID: ep150})
	_, e2 := r.svc.Hints(ctx, HubTriageSource{Hub: r.hub}, "tm-ep", HintsRequest{ProjectID: "pj-au", Title: "Upload resume broken"})
	if !errors.Is(e1, ErrAssistsOff) || !errors.Is(e2, ErrAssistsOff) || len(r.prompts()) != 0 {
		t.Errorf("errs %v, %v, %d calls: want assists_off twice and no call", e1, e2, len(r.prompts()))
	}
}
```

The scope names the guidance mutant *"project guidance follows team and org"*; SW.19 has no
project layer (decision 7), so the test proves the same ordering on the layers that exist: the
most local present layer (the brief at feature scope, else the team) is last and the only
`winning`. Also: `TestAWhyNamingUnlistedGuidanceIsDropped` (a why `g9` with two layers given);
`TestNoCandidatesMakesNoCall` (an empty search result: `judge` sends nothing); in
`api/workassist_test.go`, both new patterns are `POST` in the audit list, and a hints body with an
empty title is `400` with no Hub call.

#### Tests to write first — the desktop

```ts
// desktop/src/work/hintClock.test.ts — kills "a call per keystroke" and "stale hints"
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { hintClock, PAUSE_MS } from "./hintClock";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());
const type = (c: ReturnType<typeof hintClock>, text: string) => {
  for (let i = 1; i <= text.length; i++) { c.typed({ title: text.slice(0, i), description: "" }); vi.advanceTimersByTime(60); }
};

it("typing 30 characters with one pause in the middle makes at most two calls", async () => {
  const ask = vi.fn(async () => "h");
  const c = hintClock({ ask, deliver: () => {}, fail: () => {} });
  type(c, "Upload resume ");                        // 14 characters, no pause yet
  await vi.advanceTimersByTimeAsync(PAUSE_MS);      // call 1
  type(c, "Upload resume broken in Safari".slice(0, 30));
  await vi.advanceTimersByTimeAsync(PAUSE_MS);      // call 2
  expect(ask.mock.calls.length).toBeLessThanOrEqual(2);
  expect(ask).toHaveBeenCalledTimes(2);
});

it("under 12 characters makes no call", async () => {
  const ask = vi.fn(async () => "h");
  const c = hintClock({ ask, deliver: () => {}, fail: () => {} });
  type(c, "Upload fail");                            // 11
  await vi.advanceTimersByTimeAsync(PAUSE_MS * 3);
  expect(ask).not.toHaveBeenCalled();
});

it("an answer for superseded text is discarded", async () => {
  let resolve!: (v: string) => void;
  const ask = vi.fn(() => new Promise<string>((r) => { resolve = r; }));
  const deliver = vi.fn();
  const c = hintClock({ ask, deliver, fail: () => {} });
  c.typed({ title: "Upload resume broken", description: "" });
  await vi.advanceTimersByTimeAsync(PAUSE_MS);      // in flight
  c.typed({ title: "Upload resume broken in Safari", description: "" });
  resolve("old");
  await vi.runAllTimersAsync();
  expect(deliver).not.toHaveBeenCalledWith("old", expect.anything());
});
```

- `DuplicateHint.test.tsx`: *"Possible duplicate: EP-128"* with both titles in `q` elements; the
  rendered text matches no `/\d+\s*%|score|confidence|likely|top match/i` (kills AX
  anti-pattern 12); *Mark duplicate* calls `onMark` once, and the host's handler makes exactly one
  `addRelation` call and **no** `moveIssueState` or `patchIssue` call (kills "auto-close": both
  issues' states unchanged on the fake client); a candidate with `thatTitle: ""` renders nothing.
- `ComposerHints.test.tsx`: with fake timers, typing a 20-character title shows chips after the
  pause and not before; a label chip calls `onAddLabel` and makes no client write;
  `assists_off` shows the org words, `gateway_unavailable` shows nothing;
  `expectNoPrediction(container)` (task 20's) on the composer with chips shown.
- `TriageView.test.tsx` ⚑: a rejected `assistDuplicates` leaves task 42's rows and number keys
  working and shows *"Duplicate check unavailable: the gateway did not answer"*.

#### Steps

- [ ] **Step 1: Sync.** Task 136 is merged (its part and code); tasks 68, 100 and 109 are on the
  sync base. Re-read `NewIssue.tsx`'s props and task 100's `GuidanceView` fields; rename here.
- [ ] **Step 2: The Go tests,** over task 136's rig with the second team seeded. They fail.
- [ ] **Step 3: `guidance.go`,** then the one-line change to `triage.go`; task 136's triage tests
  still pass.
- [ ] **Step 4: `duplicates.go`, `hints.go`,** the `labelsFor` split in `triage.go`. Go tests pass.
- [ ] **Step 5: The two routes** and their audit lines. The api tests pass.
- [ ] **Step 6: The desktop wire, `hintClock.ts`, `DuplicateHint`, `ComposerHints`,** the two
  hosts. The Vitest rows pass.
- [ ] **Step 7: The acceptance block,** then commit with `git add` for the new files and
  `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
cd control-plane && go test ./internal/workassist/ ./internal/api/
# expect: ok for both; task 136's tests unchanged and passing
cd control-plane && go vet ./internal/workassist/ && gofmt -l internal/workassist/
# expect: no output
cd control-plane && grep -n -i -E 'score|similarity|embedding|cosine' internal/workassist/duplicates.go internal/workassist/hints.go
# expect: only the system prompt's "Never give a score." (decision 1: no embedding search)
cd desktop && npx vitest run src/work/hintClock.test.ts src/components/work
# expect: hintClock 3 passed; DuplicateHint 4 passed; ComposerHints 4 passed; TriageView's rows plus 1 new passed
cd desktop && npx tsc --noEmit
# expect: exit 0
```

#### What this task does NOT claim

- Embedding or similarity search: none is designed. Retrieval is the Hub's title search, so a
  duplicate whose title shares no term with this one is not found.
- Detection quality: nothing measures how often a *possible duplicate* is one.
- Duplicates across projects, even in one team (decision 1), or across teams (never).
- A project guidance layer (gap 1), or that guidance enters a session agent's context (gap 4).
- That a hint call is cancelled on the wire: its answer is dropped (gap 3).
- Anything about the gateway beyond task 136: in every deployment but local mode on the fakes,
  these routes answer `gateway_unavailable` until phase 6 decides task 136's gap 1.

#### Gaps found while writing this part

1. **No project guidance layer.** The scope and 05 §1 C L-55 say org → team → project; SW.19's
   `GuidanceScope` is org, team, person (`task-109.md:176`, task 99's scope). Adding `project` is a
   Hub delta on task 99's records and task 109's editor; the skeleton should carry it as a task.
   Until then the brief is the only layer below the team.
2. **The scope's file list omits `triage.go`.** Guidance in the triage prompt (so a triage why may
   cite guidance, as the scope asks) edits task 136's file; this part lists it ⚑.
3. **`WorkSend` takes no abort signal** (`task-37.md:399`). Cancelling a hint call on the wire needs
   an optional `signal` on task 37's client; this part drops stale answers instead.
4. **Guidance in prompts vs task 99's not-claim.** Task 99's scope does not claim that guidance may
   enter an *agent's* context (Phase T's policy layers). An assist prompt is not a session agent's
   context, and L-55 is exactly guidance for assists; the Phase T owner should confirm that reading.
5. **No part for task 68.** `NewIssue.tsx`'s props, and where its editor's plain text is read, are
   its scope's (`…-scopes/tasks-61-80.md:269-285`) and **UNVERIFIED**.
6. **`IssueHead.TeamID` and `FeatureID`** on the control plane's `hubapi.IssueHead` are task 83's
   additions (**UNVERIFIED**, no part); `GuidanceFor` needs both.
