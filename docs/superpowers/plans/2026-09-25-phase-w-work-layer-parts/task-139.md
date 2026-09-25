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
	Subject string   `json:"subject"` // "issue:is-…", "feature:ft-…" or "brief:bf-…#C3"
	Alias   string   `json:"alias"`   // "EP-142" when the subject is an issue; display only
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
	case "evidence.recorded": // hub-contract.md:1160: briefKey, criterionId, verdict, commit
		f.Kind, f.Subject = FactEvidence, str("briefKey")+"#"+str("criterionId")
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
`criterion_word` fact, *"C3 settled"*, for each criterion named by an `evidence` fact in the
period: a statement of the word **now**, never a predicted one. `actorName` is `DisplayName` or
`""` for a `nil` actor (the janitor); `hasField` splits task 13's comma-joined `fields`; `plural`
is two words. `Alias` is filled by the route from the replica-independent issue head read, as
`EP-142` from `project.key` and the issue's number (skeleton rule 1: never a key).
