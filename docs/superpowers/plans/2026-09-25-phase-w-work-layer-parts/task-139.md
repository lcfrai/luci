### Task 139: drafts, part 1: drafted updates, thread summaries as link indexes, the property-change digest

W4, control plane and desktop, about 3 hours (an estimate, not a measurement). Depends on task 127
(`UpdateComposer`, its agent-words refusal and `roadmapSelectors.ts`, `task-127.md:155-251`) and
task 136 (the assist client, the kill switch refusal `assists_off`, the route by the project's data
class, `Why`; **scope only, no part on this tree**, `…-scopes/tasks-121-140.md:473-513`, so every
name this part takes from 136 is **UNVERIFIED** and 136's part wins at build). Reads task 123's
`ScopedChanges` (`task-114.md:854-855`, `task-123.md`), task 13's metadata-only event data
(`task-13.md:122-209`), task 49's `authorClass.ts` (`task-49.md:101-129`), task 51's `Comments`
(`task-51.md:526`), task 119's `StatusUpdate` (`task-114.md:155-169`) and task 20's
`expectNoPrediction`.

Closes **L-43** (thread summaries), **L-78** (drafted updates) and **L-79** (the property-change
digest). The drafted prose of L-147's release notes is **not** here (gap 1).

**What the reader sees** (05 §1 B L-43, §1 E L-78 and L-79, §2.4; AX §6.3-6.4, *"an index over
transcripts, never a replacement"*; AX Principle 7):

- **The composer gains a fixed block, *Changes since the last update*,** above the editor, for a
  feature subject. Each line is one fact, made with **no model at all**: *"target 14 Oct → 21 Oct ·
  Priya"*, *"3 issues added"*, *"EP-142 landed · 11:40"*, *"C3 settled"*. Every line is a link to
  the event or record it came from. The block renders the same with the gateway down, with the
  kill switch on and in a project with no gateway at all. An empty period reads *"Nothing recorded
  since 22 Sep."*
- **A *Draft from these* button** sits under the block. It sends the facts (never a transcript,
  never a comment body) through task 136's client and puts the returned sentences into the one
  editor as `text·A`, each followed by its cites as chips. The health choice stays empty: the
  person picks the word, always.
- **Post is refused while an untouched drafted sentence remains.** The line under the editor reads
  *"2 drafted sentences are not yours yet. Edit them or mark them Keep as mine."* Each drafted
  sentence has a *Keep as mine* control in the editor's gutter; editing a sentence makes the edited
  characters the person's (task 49's plugin), and a sentence that still holds any agent span keeps
  the field `A` (`fieldAuthor`), so task 127's refusal stands until each is resolved.
- **A thread past 12 comments gains *Summarise*** above its first comment. The summary is a short
  list of sentences, each marked *drafted* in the agent class and each ending in the `ic-` ids it
  condenses as links, **in that thread only**. It is held in memory, never written to the Hub, and
  carries *"Not saved. Post it as a comment to keep it."* with a *Post as comment* action that opens
  the reply editor with the summary as `text·A` (task 51's reply path, the person's own act).
- **Failures are words, not blanks.** *"Drafting unavailable: assists are off for this org"* (the
  kill switch), *"Drafting unavailable: the gateway did not answer"*; the changes block and the
  thread stay as they were.

**Decisions made here, so nobody re-derives them:**

1. **Facts are mechanical and model-free.** `workassist.Facts` reads task 123's scoped feed for the
   feature (`ReadScope{Kind: "feature"}`) after a cursor, and turns each event into at most one
   `Fact`. It never calls the gateway. The digest (L-79) is these facts and nothing else, so it is
   testable with the gateway fake stopped.
2. **The six kinds.** The scope names five (`landed`, `evidence`, `criterion_word`, `scope`,
   `blocker`). The digest's own first example, *"target 14 Oct → 21 Oct · Priya"*, is a change to
   the feature's properties, which none of the five covers, so a sixth kind, **`property`**, is
   added here (target, lead and state of the feature). It is additive: a consumer that knows five
   kinds drops the sixth.
3. **The period starts at the last posted update, by feed position.** Task 123's scoped read pages
   by `after` (a feed seq), not by time (`task-123.md:40-44`). So the request carries `after`, the
   seq of the newest `work.update_posted` event under the feature that the replica holds, or `""`
   when there is none; `""` gets the Hub's bounded first look (the scope's newest 200 events). The
   control plane reads at most 4 pages of 500 and then stops with `truncated: true`, which the
   block states as *"Older changes not shown."* One request never walks a year of feed.
4. **Every drafted sentence cites a fact it was given, by key.** The model receives facts with
   short opaque keys (`f1`…`fN`) and returns `[{text, cites [fk]}]`. A sentence whose `cites` is
   empty, or names a key not in the request, is dropped on the control plane before the desktop
   sees it (AX Principle 7). The desktop maps each `fk` back to the fact's event link.
5. **The health word is never asked for and never read.** The prompt asks for sentences only; a
   reply field named `health`, or a sentence that is exactly one of the three words, is dropped.
   `DraftUpdateResponse` has no health field (a reflection test pins it).
6. ***Keep as mine* is a person's explicit act.** It removes the `agent` mark from exactly one
   drafted sentence's range (a new `keepAsMine` command beside task 49's plugin). It is the only
   way a drafted sentence becomes `text·H` without being retyped, and it is one click per sentence,
   never *keep all*. The Hub cannot tell afterwards that the sentence was drafted (gap 3).
7. **Summaries cite comments of the one thread.** The prompt carries the thread's non-erased
   comments as `c1`…`cN` with their `ic-` ids held back on the control plane; the reply's cites are
   mapped to ids there, and a cite outside the thread's list is dropped with its sentence. No
   summary is stored anywhere: not the Hub, not the replica, not disk. Unmounting the thread
   forgets it.
8. **Twelve is the threshold** (the scope's number, the owner's to judge): *Summarise* shows when
   the thread holds more than 12 non-erased comments.

**What this part takes from task 136** (its scope, `tasks-121-140.md:473-513`; **UNVERIFIED** names,
136's part wins and this part's code is renamed to match at build):

| Name | Shape assumed here |
|---|---|
| `workassist.Service` | the one assist service in `assist.go`: `Hub hubclient.Work`, `Gateway`, `Switch`, `Routes` |
| `(*Service).Complete(ctx, projectID string, c Call) (Reply, error)` | one gateway call on the project's route; refuses with `ErrAssistsOff` while the org kill switch is on, **before** any request |
| `Call{System, User string; MaxTokens int}`, `Reply{Text string; Route dataclass.Route}` | the request and the model's text, with the route it went on |
| `ErrAssistsOff`, `ErrGatewayDown` | the two sentinels the route maps to `503 assists_off` and `502 gateway_unavailable` |
| `Deps.WorkAssist *workassist.Service` | the control plane's dependency (`internal/api/api.go:68`, `Deps`) |
| `readable(ctx, projectID)` | the caller's readable-project check through the Hub relay |

**Files:**

- `control-plane/internal/workassist/facts.go` — `Fact`, `FactKind`, `Facts` (new)
- `control-plane/internal/workassist/draft_update.go` — `DraftUpdate`, the prompt, `keepCited` (new)
- `control-plane/internal/workassist/summary.go` — `SummariseThread`, the thread-only filter (new)
- `control-plane/internal/workassist/{facts,draft_update,summary}_test.go` — the Go tests below (new)
- `control-plane/internal/workassist/testdata/draft/*.json` — the prompt fixtures (new)
- `control-plane/internal/api/workassist.go` ⚑ (task 136) — three routes (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the three patterns listed (modified)
- `desktop/src/api/work.ts` ⚑ — the wire types and three calls (modified)
- `desktop/src/components/roadmap/ChangesSinceUpdate.tsx` — the fixed block (new)
- `desktop/src/components/issue/ThreadSummary.tsx` — the in-memory summary (new)
- `desktop/src/components/roadmap/UpdateComposer.tsx` ⚑ (task 127) — the block, *Draft from these*, the refusal line (modified)
- `desktop/src/components/roadmap/roadmapSelectors.ts` ⚑ (task 127) — `lastUpdateSeq` (modified)
- `desktop/src/components/issue/Comments.tsx` ⚑ (task 51) — `ThreadSummary` above a long thread (modified)
- `desktop/src/components/editor/authorClass.ts` ⚑ (task 49) — `keepAsMine`, `draftedRanges` (modified; not in the scope's list)
- `desktop/src/components/roadmap/drafts.test.tsx`, `desktop/src/components/issue/ThreadSummary.test.tsx` — the desktop tests (new)

#### Interfaces — complete: `control-plane/internal/workassist/facts.go`

```go
package workassist

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// FactKind is what one mechanical fact records (decision 2: five from the
// scope, plus property).
type FactKind string

const (
	FactLanded        FactKind = "landed"         // issue.state to landed
	FactEvidence      FactKind = "evidence"       // evidence.recorded, or issue.state to done with a proof
	FactCriterionWord FactKind = "criterion_word" // a criterion's word now, for a criterion touched in the period
	FactScope         FactKind = "scope"          // issues created under, or moved into, the feature
	FactBlocker       FactKind = "blocker"        // issue.related with kind blocks
	FactProperty      FactKind = "property"       // the feature's target, lead or state
)

// Fact is one line of the digest and one citable input to a draft. It holds
// keys, words and names only: never a comment body, a description or a
// transcript.
type Fact struct {
	Key     string   `json:"key"`     // f1…fN in feed order; stable within one response
	Kind    FactKind `json:"kind"`
	Seq     string   `json:"seq"`     // the event's feed position, the link target
	At      string   `json:"at"`      // RFC 3339, the event's time
	Subject string   `json:"subject"`   // a citable Hub key: "issue:is-…", "ev-…" or "feature:ft-…"
	Alias   string   `json:"alias"`     // "EP-142" when the subject is an issue; display only
	Criterion string `json:"criterion"` // "C3" when the fact touches a criterion, else ""
	Line    string   `json:"line"`    // the digest's words, built from event data only
	By      string   `json:"by"`      // the actor's display name; "" for the janitor
	Count   int      `json:"count"`   // scope facts: how many issues one line stands for
}

// FactsResult is the digest (L-79) and a draft's only input (L-78).
type FactsResult struct {
	FeatureID string `json:"featureId"`
	After     string `json:"after"`     // the seq the period starts after; "" for a first look
	Facts     []Fact `json:"facts"`     // never null
	Truncated bool   `json:"truncated"` // older events exist than were read (decision 3)
}

const (
	factsPageLimit = 500
	factsMaxPages  = 4
)

// Facts reads the feature's scoped feed after `after` and turns events into
// facts. It never calls the gateway (decision 1), so the digest survives the
// gateway, the kill switch and a project with no gateway at all.
func (s *Service) Facts(ctx context.Context, featureID, after string) (FactsResult, error) {
	scope := hubapi.ReadScope{Kind: "feature", ID: featureID}
	out := FactsResult{FeatureID: featureID, After: after, Facts: []Fact{}}
	scopeCount := map[string]int{} // one "N issues added" line per period, by actor
	cursor := after
	for page := 0; page < factsMaxPages; page++ {
		p, err := s.Hub.ScopedChanges(ctx, scope, cursor, factsPageLimit)
		if err != nil {
			return FactsResult{}, err
		}
		for _, ev := range p.Events {
			if f, ok := factOf(ev); ok {
				if f.Kind == FactScope {
					scopeCount[f.By]++
					continue
				}
				out.Facts = append(out.Facts, f)
			}
		}
		out.Truncated = out.Truncated || p.Truncated
		if !p.More {
			break
		}
		cursor = p.Next
		if page == factsMaxPages-1 {
			out.Truncated = true
		}
	}
	for by, n := range scopeCount { // folded after the walk: counts, never a list of titles
		out.Facts = append(out.Facts, Fact{Kind: FactScope, Subject: "feature:" + featureID,
			Line: fmt.Sprintf("%d %s added", n, plural(n, "issue", "issues")), By: by, Count: n})
	}
	out.Facts = append(out.Facts, s.criterionWords(ctx, featureID, out.Facts)...)
	for i := range out.Facts {
		out.Facts[i].Key = "f" + strconv.Itoa(i+1)
	}
	return out, nil
}

// factOf maps one metadata-only event (task 13, hub-contract.md:1160) to a fact.
// It reads Data's words and ids, never text: events carry none.
func factOf(ev hubapi.Event) (Fact, bool) {
	f := Fact{Seq: strconv.FormatInt(ev.Seq, 10), At: ev.At, Subject: ev.Subject, By: actorName(ev.Actor)}
	str := func(k string) string { v, _ := ev.Data[k].(string); return v }
	switch ev.Kind {
	case hubapi.EventIssueState:
		switch {
		case str("to") == "landed":
			f.Kind, f.Line = FactLanded, "landed"
		case str("to") == "done" && str("proof") != "":
			f.Kind, f.Line = FactEvidence, "done · "+str("proof") // "observed", "attested" (SW.4)
		default:
			return Fact{}, false
		}
	case "evidence.recorded": // hub-contract.md:1160: subject ev-…; briefKey, criterionId, verdict, commit
		f.Kind, f.Criterion = FactEvidence, str("criterionId") // Subject stays the ev- id, a citable key
		f.Line = str("criterionId") + " evidence · " + str("verdict")
	case hubapi.EventIssueRelated:
		if str("kind") != "blocks" {
			return Fact{}, false
		}
		f.Kind, f.Line = FactBlocker, "blocked by issue:"+str("otherIssueId")
	case hubapi.EventIssueCreated:
		f.Kind = FactScope
	case hubapi.EventIssueUpdated:
		if !hasField(str("fields"), "feature") {
			return Fact{}, false
		}
		f.Kind = FactScope
	case eventFeatureUpdated: // task 72's kind; name UNVERIFIED (no part on this tree)
		return propertyFact(f, ev)
	default:
		return Fact{}, false
	}
	return f, true
}
```

`propertyFact` reads task 72's feature-updated event data (`fields`, and for a target move
`from`/`to` dates, **UNVERIFIED**: task 72 has no part on this tree; if its event carries only
`fields`, the line reads *"target changed · Priya"* and the dates come from the feature's two
revisions, a gap). `criterionWords` reads the feature's current convergence (task 45's five words,
through the same Hub read task 125's `InitiativeFeature.criteria[].word` uses) and emits one
`criterion_word` fact, *"C3 settled"*, for each distinct `Criterion` named by an `evidence` fact
in the period, its `Subject` the feature: a statement of the word **now**, never a predicted one. `actorName` is `DisplayName` or
`""` for a `nil` actor (the janitor); `hasField` splits task 13's comma-joined `fields`; `plural`
is two words. `Alias` is filled by the route from the replica-independent issue head read, as
`EP-142` from `project.key` and the issue's number (skeleton rule 1: never a key).

#### Interfaces — complete: `control-plane/internal/workassist/draft_update.go`

```go
package workassist

import (
	"context"
	"encoding/json"
	"strings"
)

// DraftUpdateRequest is POST /v1/work/assist/draft-update's body.
type DraftUpdateRequest struct {
	FeatureID string `json:"featureId"`
	After     string `json:"after"` // decision 3
}

// DraftSentence is one sentence the model wrote, with the fact keys it cites.
// Its text is always agent-class: the desktop inserts it as text·A.
type DraftSentence struct {
	Text  string   `json:"text"`
	Cites []string `json:"cites"` // fact keys (f1…); at least one, each in Facts
}

// DraftUpdateResponse has no health field, by design (decision 5).
type DraftUpdateResponse struct {
	Facts     FactsResult     `json:"facts"`
	Sentences []DraftSentence `json:"sentences"` // never null; may be empty after keepCited
	Dropped   int             `json:"dropped"`   // sentences removed by keepCited, a count
	Route     string          `json:"route"`     // "global" | "au", the route the call went on
}

const draftSystem = `You write short factual sentences for a status update about one feature.
You are given numbered facts. Every sentence must cite, by key, the facts it states.
Do not state anything that no fact says. Do not judge whether the feature is on track.
Answer with JSON only: {"sentences":[{"text":"…","cites":["f1"]}]}.`

// promptFact is what the model sees of a fact: its key, kind, words and time.
// Seq, Subject ids and actor ids stay on the control plane.
type promptFact struct {
	Key  string   `json:"key"`
	Kind FactKind `json:"kind"`
	Line string   `json:"line"`
	Who  string   `json:"who,omitempty"`
	Item string   `json:"item,omitempty"` // "EP-142", the alias, for a sentence to name
	At   string   `json:"at"`
}

// DraftUpdate builds facts, asks the gateway for sentences and keeps only
// cited ones. With no facts it makes no call.
func (s *Service) DraftUpdate(ctx context.Context, projectID string, req DraftUpdateRequest) (DraftUpdateResponse, error) {
	facts, err := s.Facts(ctx, req.FeatureID, req.After)
	if err != nil {
		return DraftUpdateResponse{}, err
	}
	resp := DraftUpdateResponse{Facts: facts, Sentences: []DraftSentence{}}
	if len(facts.Facts) == 0 {
		return resp, nil
	}
	pf := make([]promptFact, 0, len(facts.Facts))
	for _, f := range facts.Facts {
		pf = append(pf, promptFact{Key: f.Key, Kind: f.Kind, Line: f.Line, Who: f.By, Item: f.Alias, At: f.At})
	}
	user, _ := json.Marshal(map[string]any{"facts": pf})
	reply, err := s.Complete(ctx, projectID, Call{System: draftSystem, User: string(user), MaxTokens: 600})
	if err != nil {
		return DraftUpdateResponse{}, err // ErrAssistsOff and ErrGatewayDown pass through
	}
	resp.Route = string(reply.Route)
	resp.Sentences, resp.Dropped = keepCited(reply.Text, facts.Facts)
	return resp, nil
}

// keepCited parses the model's JSON and keeps a sentence only when it cites at
// least one key it was given (decision 4) and is not a health word (decision 5).
// Unparseable output keeps nothing: a draft is optional, a false cite is not.
func keepCited(text string, facts []Fact) ([]DraftSentence, int) {
	given := make(map[string]bool, len(facts))
	for _, f := range facts {
		given[f.Key] = true
	}
	var raw struct {
		Sentences []DraftSentence `json:"sentences"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &raw) != nil {
		return []DraftSentence{}, 0
	}
	kept, dropped := []DraftSentence{}, 0
	for _, s := range raw.Sentences {
		ok := len(s.Cites) > 0 && strings.TrimSpace(s.Text) != "" && !isHealthWord(s.Text)
		for _, c := range s.Cites {
			ok = ok && given[c]
		}
		if !ok {
			dropped++
			continue
		}
		kept = append(kept, DraftSentence{Text: strings.TrimSpace(s.Text), Cites: s.Cites})
	}
	return kept, dropped
}

// isHealthWord catches a sentence that is only a verdict (decision 5).
func isHealthWord(t string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(t), ".!")) {
	case "on track", "at risk", "off track", "on_track", "at_risk", "off_track":
		return true
	}
	return false
}
```

The system prompt is fixed text in the binary, not a template the Hub or a person edits. A
sentence citing one given key and one invented key is dropped whole: a partly true cite list is a
false one. `Dropped` is a plain count so the desktop can say *"1 drafted sentence was left out: it
cited nothing it was given"*; it is never shown as a share.

#### Interfaces — complete: `control-plane/internal/workassist/summary.go`

```go
package workassist

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// SummaryThreshold is decision 8: more than this many non-erased comments.
const SummaryThreshold = 12

// maxThreadComments bounds one prompt; a longer thread summarises its newest.
const maxThreadComments = 200

// ThreadSummaryRequest is POST /v1/work/assist/summarise-thread's body.
// RootID is the top-level comment; "" means the issue's whole activity.
type ThreadSummaryRequest struct {
	IssueID string `json:"issueId"`
	RootID  string `json:"rootId"`
}

// SummaryLine is one sentence and the ic- ids it condenses, all in the thread.
type SummaryLine struct {
	Text     string   `json:"text"`     // agent-class; the desktop marks it drafted
	Comments []string `json:"comments"` // ic-… ids, at least one, each in this thread
}

// ThreadSummary is returned and never stored (decision 7).
type ThreadSummary struct {
	IssueID string        `json:"issueId"`
	RootID  string        `json:"rootId"`
	Lines   []SummaryLine `json:"lines"`   // never null
	Dropped int           `json:"dropped"` // lines removed for a cite outside the thread
	Route   string        `json:"route"`
}

var ErrThreadTooShort = errors.New("workassist: thread has too few comments to summarise")

const summarySystem = `You index a discussion. Each comment has a key like c1.
Write at most 8 short sentences. Each sentence condenses one or more comments and lists their keys.
Quote no one at length. Add nothing the comments do not say.
Answer with JSON only: {"lines":[{"text":"…","comments":["c1","c4"]}]}.`

// SummariseThread reads the thread's comments, sends the non-erased ones with
// local keys, and maps the reply's keys back to ic- ids of this thread only.
func (s *Service) SummariseThread(ctx context.Context, projectID string, req ThreadSummaryRequest) (ThreadSummary, error) {
	cs, err := s.threadComments(ctx, req.IssueID, req.RootID)
	if err != nil {
		return ThreadSummary{}, err
	}
	if len(cs) <= SummaryThreshold {
		return ThreadSummary{}, ErrThreadTooShort
	}
	local := make(map[string]string, len(cs)) // c1 → ic-…; never sent
	type pc struct{ Key, Who, Text string }
	in := make([]pc, 0, len(cs))
	for i, c := range cs {
		k := "c" + strconv.Itoa(i+1)
		local[k] = c.ID
		in = append(in, pc{Key: k, Who: c.CreatedBy.DisplayName, Text: c.Body.Text})
	}
	user, _ := json.Marshal(map[string]any{"comments": in})
	reply, err := s.Complete(ctx, projectID, Call{System: summarySystem, User: string(user), MaxTokens: 700})
	if err != nil {
		return ThreadSummary{}, err
	}
	out := ThreadSummary{IssueID: req.IssueID, RootID: req.RootID, Lines: []SummaryLine{}, Route: string(reply.Route)}
	var raw struct {
		Lines []SummaryLine `json:"lines"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(reply.Text)), &raw) != nil {
		return out, nil
	}
	for _, l := range raw.Lines {
		ids, ok := make([]string, 0, len(l.Comments)), len(l.Comments) > 0 && strings.TrimSpace(l.Text) != ""
		for _, k := range l.Comments {
			id, found := local[k]
			ok = ok && found
			ids = append(ids, id)
		}
		if !ok {
			out.Dropped++
			continue
		}
		out.Lines = append(out.Lines, SummaryLine{Text: strings.TrimSpace(l.Text), Comments: ids})
	}
	return out, nil
}
```

`threadComments` pages task 04's `Comments(ctx, issueID, PageRequest)` (`task-04.md:212`, oldest
first, `Page{Items, Next}`), keeps comments whose `ParentID` is `RootID` or which are the root
(all of them when `RootID` is `""`), **drops every comment whose `ErasedAt` is set** and every
comment whose `Kind` is not a plain comment (so an `edit_proposal` of task 141 or a note never
enters a summary), and keeps the newest `maxThreadComments`. The model's cite is a local key; a key
it invents, or an `ic-` id it copies from a body, is not in `local` and drops the line. No write
follows: the service holds no store, and the route test counts the fake Hub's writes.

#### Interfaces — complete: the routes, appended to `control-plane/internal/api/workassist.go` ⚑

```go
// Task 139: the digest, the drafted update, the thread summary. Registered in
// task 136's registerWorkAssist beside its triage route, on *routes, so the
// audit sees them (api.go:218-262).
func (a *API) registerWorkDrafts(rs *routes) {
	rs.HandleFunc("POST /v1/work/assist/facts", a.postWorkFacts)
	rs.HandleFunc("POST /v1/work/assist/draft-update", a.postDraftUpdate)
	rs.HandleFunc("POST /v1/work/assist/summarise-thread", a.postSummariseThread)
}

// postWorkFacts is POST, not GET: it pages the Hub on the caller's behalf, and
// contract §10a keeps GET for reads of our own state. It never touches the
// gateway, so the kill switch does not apply to it (decision 1).
func (a *API) postWorkFacts(w http.ResponseWriter, r *http.Request) {
	var req workassist.DraftUpdateRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !a.readableFeature(w, r, req.FeatureID) {
		return
	}
	res, err := a.deps.WorkAssist.Facts(r.Context(), req.FeatureID, req.After)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a *API) postDraftUpdate(w http.ResponseWriter, r *http.Request) {
	var req workassist.DraftUpdateRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	projectID, ok := a.featureProject(w, r, req.FeatureID)
	if !ok {
		return
	}
	res, err := a.deps.WorkAssist.DraftUpdate(r.Context(), projectID, req)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a *API) postSummariseThread(w http.ResponseWriter, r *http.Request) {
	var req workassist.ThreadSummaryRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	projectID, ok := a.issueProject(w, r, req.IssueID)
	if !ok {
		return
	}
	res, err := a.deps.WorkAssist.SummariseThread(r.Context(), projectID, req)
	if errors.Is(err, workassist.ErrThreadTooShort) {
		httpx.WriteProblem(w, http.StatusConflict, "thread_too_short",
			"a thread needs more than 12 comments to summarise", nil)
		return
	}
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
```

`readableFeature`, `featureProject`, `issueProject` and `assistProblem` are task 136's helpers
(**UNVERIFIED** names): the first two read the feature through the Hub relay **as the caller** and
answer the Hub's own 404 for an unreadable one, so a feature of another team is not even confirmed
to exist; `assistProblem` maps `ErrAssistsOff` to `503 assists_off` and `ErrGatewayDown` to `502
gateway_unavailable`, and every other error to the relay's problem. `thread_too_short` is a new
problem code, appended to `httpx`'s code list (`problem.go`, the sixteen-codes test,
`problem_test.go:35`, becomes one more). No handler logs a request or a reply body (task 136's
rule: the control plane logs no issue text). `routes_audit_test.go` gains the three patterns, each
`POST`, so §10a's no-side-effecting-GET check holds.

#### Interfaces — complete: `desktop/src/api/work.ts` ⚑, appended

```ts
// Task 139: drafts, part 1. Mirrors control-plane/internal/workassist.
export type FactKind = "landed" | "evidence" | "criterion_word" | "scope" | "blocker" | "property";
export interface Fact {
  key: string; kind: FactKind; seq: string; at: string;
  subject: string; alias: string; criterion: string; line: string; by: string; count: number;
}
export interface FactsResult { featureId: string; after: string; facts: Fact[]; truncated: boolean }
export interface DraftSentence { text: string; cites: string[] }
/** No health field, on purpose (decision 5). */
export interface DraftUpdateResponse {
  facts: FactsResult; sentences: DraftSentence[]; dropped: number; route: "global" | "au";
}
export interface SummaryLine { text: string; comments: string[] }
export interface ThreadSummary {
  issueId: string; rootId: string; lines: SummaryLine[]; dropped: number; route: "global" | "au";
}

// WorkClient gains three calls (the interface is task 37's; appended, not reordered).
export interface WorkDraftsClient {
  workFacts(featureId: string, after: string): Promise<FactsResult>;
  draftUpdate(featureId: string, after: string): Promise<DraftUpdateResponse>;
  summariseThread(issueId: string, rootId: string): Promise<ThreadSummary>;
}
```

`WorkClient` extends `WorkDraftsClient`; each call is a `POST` through task 37's client to the
control plane's `/v1/work/assist/*` (not the Hub relay), and a problem arrives as `ApiError` with
its `code` (`client.ts:58`), so the desktop tells `assists_off` from `gateway_unavailable` by
code, never by message.

#### Interfaces — complete: `desktop/src/components/editor/authorClass.ts` ⚑, appended

```ts
/** Each drafted range still holding agent text, merged, in document order (decision 6). */
export function draftedRanges(spans: readonly AuthorSpan[]): AuthorSpan[] {
  const a = spans.filter((s) => s.cls === "text·A" && s.to > s.from).slice().sort((x, y) => x.from - y.from);
  const out: AuthorSpan[] = [];
  for (const s of a) {
    const last = out[out.length - 1];
    if (last && s.from <= last.to) last.to = Math.max(last.to, s.to);
    else out.push({ ...s });
  }
  return out;
}

/** "Keep as mine": the person adopts exactly [from, to). Only this range loses its mark. */
export function keepAsMine(d: MarkdownDoc, from: number, to: number): MarkdownDoc {
  const authorship: AuthorSpan[] = [];
  for (const s of d.authorship) {
    if (s.to <= from || s.from >= to) { authorship.push(s); continue; }
    if (s.from < from) authorship.push({ ...s, to: from });
    if (s.to > to) authorship.push({ ...s, from: to });
  }
  return { markdown: d.markdown, authorship };
}

/** A drafted sentence's text, placed after the person's text, marked agent. */
export function appendDrafted(d: MarkdownDoc, sentences: readonly string[]): MarkdownDoc {
  let markdown = d.markdown;
  const authorship = [...d.authorship];
  for (const t of sentences) {
    const sep = markdown === "" ? "" : "\n\n";
    const from = markdown.length + sep.length;
    markdown = markdown + sep + t;
    authorship.push({ from, to: from + t.length, cls: "text·A" });
  }
  return { markdown, authorship };
}
```

`keepAsMine` runs as an editor command (the gutter's *Keep as mine* control dispatches a
transaction removing the `agent` mark over the one range); the functions above are its pure form
and what the tests pin. The person's typing inside a drafted sentence already removes the mark from
what they typed (task 49's `personTyped` plugin, `task-49.md:129`); an untouched remainder keeps
it, so `fieldAuthor` stays `A` and task 127's `canPost` stays false until every range is resolved.

#### Interfaces — complete: `desktop/src/components/roadmap/ChangesSinceUpdate.tsx`

```tsx
// desktop/src/components/roadmap/ChangesSinceUpdate.tsx — L-79. Facts, no model.
import { useEffect, useState } from "react";
import type { Fact, FactsResult } from "../../api/work";
import { useWorkClient } from "../../api/useWorkClient";
import { KeyLink } from "../common/KeyLink";

export interface ChangesSinceUpdateProps {
  featureId: string;
  after: string;                           // lastUpdateSeq; "" for none
  sinceLabel: string;                      // "22 Sep", the last update's day, or "the start"
  timeZone: string;
  onFacts(r: FactsResult): void;           // the composer keeps them for Draft from these
}

export function ChangesSinceUpdate(p: ChangesSinceUpdateProps): JSX.Element {
  const work = useWorkClient();
  const [res, setRes] = useState<FactsResult | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    work.workFacts(p.featureId, p.after).then(
      (r) => { if (live) { setRes(r); p.onFacts(r); } },
      () => { if (live) setFailed(true); });
    return () => { live = false; };
  }, [p.featureId, p.after]);   // read once per open; the composer does not re-read while typing

  if (failed) return <p className="csu csu-fail">Changes could not be read from the Hub.</p>;
  if (res === null) return <p className="csu csu-wait" aria-busy="true">Reading changes</p>;
  return (
    <section className="csu" aria-label="Changes since the last update">
      <h3 className="csu-head">Changes since {p.sinceLabel}</h3>
      {res.facts.length === 0
        ? <p className="csu-none">Nothing recorded since {p.sinceLabel}.</p>
        : <ul className="csu-list">{res.facts.map((f) => <FactLine key={f.key} f={f} tz={p.timeZone} />)}</ul>}
      {res.truncated && <p className="csu-more">Older changes not shown.</p>}
    </section>
  );
}

function FactLine({ f, tz }: { f: Fact; tz: string }): JSX.Element {
  const time = new Intl.DateTimeFormat("en-AU", { hour: "2-digit", minute: "2-digit", timeZone: tz }).format(new Date(f.at));
  return (
    <li className="csu-fact" data-kind={f.kind}>
      <KeyLink k={f.subject} />
      {f.alias !== "" && <span className="csu-alias">{f.alias}</span>}
      <span className="csu-line">{f.line}</span>
      {f.by !== "" && <span className="csu-by"> · {f.by}</span>}
      {f.at !== "" && <time className="csu-at" dateTime={f.at}> · {time}</time>}
    </li>
  );
}
```

The block has no *loading* spinner (`aria-busy` text only; DESIGN-SYSTEM's one heartbeat rule).
It never calls the gateway, so it renders the same whatever task 136's switch says. The folded
scope line has `at: ""` and shows no time. Ids and times are Commit Mono (`OD-28`) in `roadmap.css`
(`.csu-alias, .csu-at { font-family: var(--font-mono); font-variant-numeric: tabular-nums }`).

#### Interfaces — complete: the ⚑ `UpdateComposer.tsx` delta (task 127)

```tsx
// Added imports:
import { ChangesSinceUpdate } from "./ChangesSinceUpdate";
import { appendDrafted, draftedRanges, keepAsMine } from "../editor/authorClass";
import { lastUpdateSeq, lastUpdateDay } from "./roadmapSelectors";
import { ApiError } from "../../api/client";
import type { FactsResult } from "../../api/work";

// Inside UpdateComposer, after the body state (feature subjects only):
const isFeature = p.subject.kind === "feature";
const after = useWork((r) => (isFeature ? lastUpdateSeq(r, p.subject.id) : ""));
const sinceLabel = useWork((r) => (isFeature ? lastUpdateDay(r, p.subject.id, "Australia/Sydney") : "the start"));
const [facts, setFacts] = useState<FactsResult | null>(null);
const [draftNote, setDraftNote] = useState<string | null>(null);
const work = useWorkClient();
const drafted = draftedRanges(body.authorship);

const draft = async () => {
  if (!isFeature) return;
  setDraftNote(null);
  try {
    const r = await work.draftUpdate(p.subject.id, after);
    const byKey = new Map(r.facts.facts.map((f) => [f.key, f] as const));
    setBody((b) => appendDrafted(b, r.sentences.map((s) => s.text)));
    setCites((c) => [...new Set([...c, ...r.sentences.flatMap((s) => s.cites.map((k) => byKey.get(k)!.subject))])]);
    if (r.dropped > 0) setDraftNote(`${r.dropped} drafted ${r.dropped === 1 ? "sentence was" : "sentences were"} left out: cited nothing it was given.`);
  } catch (e) {
    const code = e instanceof ApiError ? e.code : "";
    setDraftNote(code === "assists_off" ? "Drafting unavailable: assists are off for this org."
      : "Drafting unavailable: the gateway did not answer.");
  }
};

// In the JSX, before <Editor>:
{isFeature && <ChangesSinceUpdate featureId={p.subject.id} after={after} sinceLabel={sinceLabel}
  timeZone="Australia/Sydney" onFacts={setFacts} />}
{isFeature && <button type="button" onClick={draft} disabled={facts === null || facts.facts.length === 0}>Draft from these</button>}
{draftNote && <p className="ucmp-note" role="status">{draftNote}</p>}

// Replacing task 127's agent-words line:
{drafted.length > 0 && (
  <div className="ucmp-agent">
    <p>{drafted.length} drafted {drafted.length === 1 ? "sentence is" : "sentences are"} not yours yet. Edit them or mark them Keep as mine.</p>
    <ul>{drafted.map((s) => (
      <li key={s.from}><q>{body.markdown.slice(s.from, s.to)}</q>{" "}
        <button type="button" onClick={() => setBody((b) => keepAsMine(b, s.from, s.to))}>Keep as mine</button></li>))}
    </ul>
  </div>
)}
```

The composer's time zone is the person's (`useMe().timeZone`, task 126's harness sets
`Australia/Sydney`); the literal above stands for it. Cites from a draft are the facts' **subjects**
(`issue:is-…`, `ev-…`, `feature:ft-…`), because task 119's `cites` are Hub keys; a fact key `f1` never
leaves the composer. The health radio group is untouched: a draft never sets `health`, and the
`Draft from these` handler has no reference to it (the test reads the radios after a draft).

#### Interfaces — complete: `desktop/src/components/issue/ThreadSummary.tsx`

```tsx
// desktop/src/components/issue/ThreadSummary.tsx — L-43. An index over the thread, never stored.
import { useState } from "react";
import type { ThreadSummary as Summary } from "../../api/work";
import { useWorkClient } from "../../api/useWorkClient";
import { ApiError } from "../../api/client";

export const SUMMARY_THRESHOLD = 12;   // decision 8; mirrors workassist.SummaryThreshold

export interface ThreadSummaryProps {
  issueId: string;
  rootId: string;                     // "" for the whole activity
  liveCount: number;                  // non-erased comments in the thread
  onJump(commentId: string): void;    // scrolls the thread to ic-…, no navigation
  onPostAsComment(text: string): void; // opens task 51's reply editor with text·A
}

export function ThreadSummary(p: ThreadSummaryProps): JSX.Element | null {
  const work = useWorkClient();
  const [s, setS] = useState<Summary | null>(null);   // memory only: gone on unmount (decision 7)
  const [note, setNote] = useState<string | null>(null);
  if (p.liveCount <= SUMMARY_THRESHOLD) return null;

  const run = async () => {
    setNote(null);
    try { setS(await work.summariseThread(p.issueId, p.rootId)); }
    catch (e) {
      const code = e instanceof ApiError ? e.code : "";
      setNote(code === "assists_off" ? "Summaries unavailable: assists are off for this org."
        : "Summaries unavailable: the gateway did not answer.");
    }
  };

  if (s === null) {
    return (
      <div className="ts">
        <button type="button" onClick={run}>Summarise</button>
        {note && <p className="ts-note" role="status">{note}</p>}
      </div>
    );
  }
  return (
    <section className="ts" aria-label="Thread summary" data-author="A">
      <p className="ts-head"><span className="ts-drafted">Drafted</span> · Not saved. Post it as a comment to keep it.</p>
      <ul className="ts-lines">
        {s.lines.map((l, i) => (
          <li key={i} className="ts-line">
            <span className="ts-text">{l.text}</span>{" "}
            {l.comments.map((id) => (
              <a key={id} href={`#${id}`} className="ts-cite"
                onClick={(e) => { e.preventDefault(); p.onJump(id); }}>{id}</a>))}
          </li>))}
      </ul>
      {s.dropped > 0 && <p className="ts-note">{s.dropped} {s.dropped === 1 ? "line was" : "lines were"} left out: cited outside this thread.</p>}
      <button type="button" onClick={() => p.onPostAsComment(s.lines.map((l) => `${l.text} (${l.comments.join(", ")})`).join("\n"))}>
        Post as comment
      </button>
    </section>
  );
}
```

**The ⚑ `Comments.tsx` delta (task 51).** Above a top-level comment whose thread holds more than
`SUMMARY_THRESHOLD` non-erased comments, `Comments` renders `<ThreadSummary issueId rootId
liveCount onJump onPostAsComment />`. `onJump` scrolls to `[data-comment="ic-…"]`
(`task-51.md:467`'s attribute) and focuses it; `onPostAsComment` opens that thread's reply editor
with `docFromAuthored({text, author: "A"})`, so the posted comment is `text·A` unless the person
edits it (task 49's rule). The summary is **not** part of the thread's list, so it never reaches
the replica's comment store and never becomes a Hub write on its own.

#### Tests to write first — Go, `control-plane/internal/workassist/{facts,draft_update,summary}_test.go`

Over task 136's test rig (**UNVERIFIED** name `newRig(t)`): a `hubclient.Fake` seeded with project
`EP` (data class `au-only` in one case), feature `ft-ep`, its brief `bf-7k2`, issues EP-142 and
EP-150; P6 task 16's `fakeanthropic` on two ports labelled `global` and `au`, answering with a
canned body the test sets (`rig.reply(json)`); and task 136's kill switch (`rig.switchOn()`).
`rig.prompts()` returns the request bodies the fake received (P6-16's `_captured` holds hashes
only, so the rig's own capture wraps the gateway client, test-only).

```go
func TestFactsNeedNoGateway(t *testing.T) { // kills "a model in the digest"
	r := newRig(t)
	r.hub.Emit(stateEvent("is-142", "in_review", "landed", "Priya"))
	r.gateway.Stop()
	res, err := r.svc.Facts(ctx, "ft-ep", "")
	require.NoError(t, err)
	require.Len(t, res.Facts, 1)
	require.Equal(t, FactLanded, res.Facts[0].Kind)
	require.Zero(t, r.gateway.Calls())
}

func TestFactsCarryNoText(t *testing.T) { // kills "comment text in a fact"
	r := newRig(t)
	r.hub.Comment("is-142", "the storage quota is the blocker, ask Sam")
	res, _ := r.svc.Facts(ctx, "ft-ep", "")
	b, _ := json.Marshal(res)
	require.NotContains(t, string(b), "storage quota")
}

func TestUncitedAndForeignCitesAreDropped(t *testing.T) { // kills "a claim with no span", "a hallucinated cite"
	r := newRig(t)
	r.hub.Emit(stateEvent("is-142", "in_review", "landed", "Priya"))
	r.reply(`{"sentences":[{"text":"EP-142 landed.","cites":["f1"]},
		{"text":"Morale is high.","cites":[]},{"text":"EP-9 shipped.","cites":["f1","f7"]}]}`)
	res, err := r.svc.DraftUpdate(ctx, "pj-ep", DraftUpdateRequest{FeatureID: "ft-ep"})
	require.NoError(t, err)
	require.Equal(t, []DraftSentence{{Text: "EP-142 landed.", Cites: []string{"f1"}}}, res.Sentences)
	require.Equal(t, 2, res.Dropped)
}

func TestDraftNeverCarriesHealth(t *testing.T) { // kills "the model picks health"
	r := newRig(t)
	r.hub.Emit(stateEvent("is-142", "in_review", "landed", "Priya"))
	r.reply(`{"health":"on_track","sentences":[{"text":"On track.","cites":["f1"]}]}`)
	res, _ := r.svc.DraftUpdate(ctx, "pj-ep", DraftUpdateRequest{FeatureID: "ft-ep"})
	require.Empty(t, res.Sentences)
	_, has := reflect.TypeOf(DraftUpdateResponse{}).FieldByName("Health")
	require.False(t, has)
}

func TestSummaryLinksOnlyItsThread(t *testing.T) { // kills "cross-thread links"
	r := newRig(t)
	root := r.hub.Thread("is-142", 14) // 14 comments, ic-a1…
	other := r.hub.Thread("is-150", 1)[0]
	r.reply(`{"lines":[{"text":"Quota agreed.","comments":["c2","c3"]},
		{"text":"See the other issue.","comments":["` + other.ID + `"]}]}`)
	s, err := r.svc.SummariseThread(ctx, "pj-ep", ThreadSummaryRequest{IssueID: "is-142", RootID: root[0].ID})
	require.NoError(t, err)
	require.Equal(t, []string{root[1].ID, root[2].ID}, s.Lines[0].Comments)
	require.Len(t, s.Lines, 1)
	require.Equal(t, 1, s.Dropped)
	require.Zero(t, r.hub.WritesSince(root[13].ID)) // kills "summary stored"
}

func TestErasedCommentNeverPrompted(t *testing.T) { // kills "erased text sent"
	r := newRig(t)
	cs := r.hub.Thread("is-142", 14)
	r.hub.EraseComment(cs[4].ID, "secret-token-shaped words")
	r.reply(`{"lines":[]}`)
	_, _ = r.svc.SummariseThread(ctx, "pj-ep", ThreadSummaryRequest{IssueID: "is-142", RootID: cs[0].ID})
	require.NotContains(t, strings.Join(r.prompts(), "\n"), "secret-token-shaped words")
	golden.Assert(t, "testdata/draft/summary-prompt.json", r.prompts()[0])
}

func TestKillSwitchAndAURoute(t *testing.T) { // kills "switch ignored", "Global for AU data"
	r := newRig(t, withDataClass("pj-ep", "au-only"))
	r.hub.Emit(stateEvent("is-142", "in_review", "landed", "Priya"))
	r.reply(`{"sentences":[{"text":"EP-142 landed.","cites":["f1"]}]}`)
	res, _ := r.svc.DraftUpdate(ctx, "pj-ep", DraftUpdateRequest{FeatureID: "ft-ep"})
	require.Equal(t, "au", res.Route)
	require.Equal(t, 1, r.gateway.CallsOn("au"))
	r.switchOn()
	_, err := r.svc.DraftUpdate(ctx, "pj-ep", DraftUpdateRequest{FeatureID: "ft-ep"})
	require.ErrorIs(t, err, ErrAssistsOff)
	require.Equal(t, 1, r.gateway.Calls()) // no second call
}
```

A route test in `internal/api/workassist_test.go` adds: `POST /v1/work/assist/summarise-thread`
on a 12-comment thread answers `409 thread_too_short`; `draft-update` under the switch answers
`503 assists_off`; the facts route answers `200` with the switch on (kills "the digest behind
the switch"); a feature of an unreadable project answers the Hub's `404`. The house uses the
standard library's `testing` (the `require` above is shorthand for the rig's `must*` helpers;
the control plane has no testify dependency, `control-plane/go.mod`).

#### Tests to write first — desktop, `drafts.test.tsx` and `ThreadSummary.test.tsx`

Over task 126's `roadmapHarness` with task 127's fixtures (feature `ft-ep`, brief `bf-7k2@6`) and a
`workClient` stub whose three calls the test answers (`assist.facts`, `assist.draft`,
`assist.summary`, each settable to a value or an `ApiError`). Every test ends with
`expectNoPrediction(container)`.

```tsx
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { renderWith, assist, fact } from "../../test/roadmapHarness";
import { UpdateComposer } from "./UpdateComposer";
import { ThreadSummary } from "../issue/ThreadSummary";
import { ApiError } from "../../api/client";
import { keepAsMine, appendDrafted, fieldAuthor } from "../editor/authorClass";

const composer = () => renderWith(<UpdateComposer subject={{ kind: "feature", id: "ft-ep" }}
  featureIds={["ft-ep"]} onPosted={() => {}} onCancel={() => {}} />);

describe("task 139: the digest and the drafted update", () => {
  it("the changes block renders with the gateway down", async () => {
    assist.facts({ facts: [fact({ key: "f1", kind: "property", line: "target 14 Oct → 21 Oct", by: "Priya" })] });
    assist.draft(new ApiError(502, { code: "gateway_unavailable" }));
    const { container } = await composer();
    expect(screen.getByText("target 14 Oct → 21 Oct")).toBeTruthy();     // kills "a model in the digest"
    fireEvent.click(screen.getByRole("button", { name: "Draft from these" }));
    expect(await screen.findByText("Drafting unavailable: the gateway did not answer.")).toBeTruthy();
    expect(screen.getByText("target 14 Oct → 21 Oct")).toBeTruthy();
    expectNoPrediction(container);
  });

  it("a draft enters as text·A, leaves health empty, and Post is refused", async () => {
    assist.facts({ facts: [fact({ key: "f1", kind: "landed", subject: "issue:is-142", alias: "EP-142" })] });
    assist.draft({ sentences: [{ text: "EP-142 landed.", cites: ["f1"] }], dropped: 0, route: "global" });
    const { container } = await composer();
    fireEvent.click(screen.getByRole("button", { name: "Draft from these" }));
    await screen.findByText(/1 drafted sentence is not yours yet/);
    for (const r of screen.getAllByRole("radio")) expect((r as HTMLInputElement).checked).toBe(false); // kills "the model picks health"
    fireEvent.click(screen.getByLabelText(/At risk/));
    expect(screen.getByRole("button", { name: "Post" })).toHaveProperty("disabled", true);   // kills "text·A posted as text·H"
    fireEvent.click(screen.getByRole("button", { name: "Keep as mine" }));
    expect(screen.getByRole("button", { name: "Post" })).toHaveProperty("disabled", false);
    expect(within(container.querySelector(".ucmp-cites")!).getByText(/issue:is-142/)).toBeTruthy(); // the cite is a Hub key, not f1
    expectNoPrediction(container);
  });

  it("keepAsMine adopts one sentence only", () => {
    const d = appendDrafted({ markdown: "", authorship: [] }, ["One.", "Two."]);
    const once = keepAsMine(d, 0, 4);
    expect(fieldAuthor(once.authorship)).toBe("A");                          // kills "keep all"
    expect(fieldAuthor(keepAsMine(once, 6, 10).authorship)).toBe("H");
  });

  it("the kill switch reads as words", async () => {
    assist.facts({ facts: [fact({ key: "f1" })] });
    assist.draft(new ApiError(503, { code: "assists_off" }));
    const { container } = await composer();
    fireEvent.click(screen.getByRole("button", { name: "Draft from these" }));
    expect(await screen.findByText("Drafting unavailable: assists are off for this org.")).toBeTruthy();
    expectNoPrediction(container);
  });
});

describe("task 139: the thread summary", () => {
  it("is absent at 12, offered at 13, and writes nothing", async () => {
    const p = { issueId: "is-142", rootId: "ic-a1", onJump: () => {}, onPostAsComment: () => {} };
    const { container, rerender, hub } = await renderWith(<ThreadSummary {...p} liveCount={12} />);
    expect(screen.queryByRole("button", { name: "Summarise" })).toBeNull();
    assist.summary({ lines: [{ text: "Quota agreed.", comments: ["ic-a2", "ic-a3"] }], dropped: 0, route: "global" });
    rerender(<ThreadSummary {...p} liveCount={13} />);
    fireEvent.click(screen.getByRole("button", { name: "Summarise" }));
    expect(await screen.findByRole("link", { name: "ic-a2" })).toBeTruthy();   // every line links its span
    expect(screen.getByText(/Not saved/)).toBeTruthy();
    expect(hub.writes()).toEqual([]);                                          // kills "summary stored"
    expectNoPrediction(container);
  });
});
```

`assist` and `fact` extend task 126's harness (the stubbed `WorkDraftsClient` and a `Fact`
builder with defaults). `ApiError`'s constructor shape is `client.ts:58`'s, taken at build.

#### Steps

1. Write the Go tests and the prompt golden; they fail: no `facts.go`, `draft_update.go`,
   `summary.go`.
2. Write `facts.go` (`factOf`, `propertyFact`, `criterionWords`); the two facts tests pass.
3. Write `draft_update.go`; the cite and health tests pass. Write `summary.go`; the summary and
   erasure tests pass. Run the switch and AU test against task 136's rig.
4. Append the three routes to `workassist.go`, add `thread_too_short` to `httpx`'s codes, list the
   patterns in `routes_audit_test.go`; `go test ./control-plane/... -race -count=1`.
5. Write the desktop tests; they fail. Append the `work.ts` types and calls and the `authorClass`
   functions; the `keepAsMine` test passes.
6. Write `ChangesSinceUpdate.tsx`, `lastUpdateSeq` and `lastUpdateDay`, and the composer delta;
   the composer tests pass. Write `ThreadSummary.tsx` and the `Comments.tsx` delta; its test passes.
7. `roadmap.css` and `issue.css` additions (mono ids and times, the drafted mark as a shape and a
   word, no hue alone); `npm --prefix desktop run typecheck`; commit with
   `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
go test ./control-plane/internal/workassist/... ./control-plane/internal/api/... -race -count=1
# expect: ok for both packages; TestFactsNeedNoGateway, TestUncitedAndForeignCitesAreDropped,
#         TestDraftNeverCarriesHealth, TestSummaryLinksOnlyItsThread, TestErasedCommentNeverPrompted,
#         TestKillSwitchAndAURoute among them; no FAIL
npm --prefix desktop test -- src/components/roadmap/drafts.test.tsx src/components/issue/ThreadSummary.test.tsx src/components/editor/authorClass.test.ts
# expect: drafts.test.tsx 4 passed; ThreadSummary.test.tsx 1 passed; authorClass.test.ts as before plus none failed
npm --prefix desktop run typecheck
# expect: exit 0
```

Every assist row runs against P6 task 16's fake; with phase 6 not on the sync base, the Go rig
cannot start and the rows are recorded *not run* in task 142, never *passed*. **Visual acceptance
is the owner's eyes**: the drafted mark, the block and the summary are judged in the running app.

#### What this task does NOT claim

- That drafts or summaries read well, are accurate, or save anyone time (unmeasured; the cites and
  the person's edit are the check).
- Drafted release-note prose (L-147's other half): this row's dependsOn lacks task 131 (gap 1).
- That the Hub can tell a posted update was drafted: after *Keep as mine*, the body is `text·H` on
  the wire (gap 3).
- That the digest shows issues **removed** from the feature: task 123 reads membership now
  (`task-123.md:33-34`), so an issue moved out is no longer under the feature's scope (gap 4).
- That the model's routing matches the project's class beyond task 136's route choice, which this
  task reuses and does not re-derive.

#### Gaps found while writing this part

1. **Task 139's dependsOn lacks 131**, so L-147's drafted prose has no task (the scope's gap 3).
2. **Task 136 has no part on this tree**: `Service`, `Complete`, `Call`, `Reply`, `ErrAssistsOff`,
   `ErrGatewayDown`, the rig and the helpers `readableFeature`, `featureProject`, `issueProject`,
   `assistProblem` are assumed names; the file map has no `workassist` rows (scope gap 2).
3. **No drafted-then-adopted marker on the wire.** Task 49's gap (one class per field) means *Keep
   as mine* makes a drafted sentence indistinguishable from typed text; a per-passage class on
   `AuthoredText` is the Hub-contract owner's call.
4. **Scope removals are invisible** to the feature-scoped feed; task 87's `ScopeLog` (no part on
   this tree) may hold them, and the digest should read it when it lands.
5. **Task 72's feature-updated event** (no part on this tree) must carry the `from` and `to` of a
   target move for *"target 14 Oct → 21 Oct"*; if it carries `fields` only, the line cannot name
   the dates without a revision read.
6. **`thread_too_short`** is a new control-plane problem code; `problem_test.go:35`'s count moves.
