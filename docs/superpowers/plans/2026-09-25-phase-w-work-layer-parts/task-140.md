### Task 140: drafts, part 2: *propose issues per criterion*; *propose the next cycle*

W4, control plane and desktop, about 3 hours (an estimate, not a measurement). Depends on task 87
(`CycleView`, `CycleConvergence`; **scope only, no part on this tree**, `…-scopes/tasks-81-100.md:165-184`)
and task 136 (the assist client; **scope only**, `tasks-121-140.md:473-513`). Every name taken from
either is **UNVERIFIED** and their parts win at build. Reads task 80's `df-` draft routes and task
72's `Draft{id, kind, issueId, body AuthoredText, updatedAt, expiresAt}` (scope,
`tasks-61-80.md:411-412`, `665-676`), task 77's `SetCycleRequest{cycleId *string}` and `PUT
/v1/issues/{issueId}/cycle` (scope, `tasks-61-80.md:417`, `568`), task 03's `CreateIssueRequest`
(`task-03.md:420-430`), task 56's `mutate` batch (`task-56.md:47-48`), task 62's Desk registration
(SW.17, scope `tasks-61-80.md:72-99`, as task 127 uses it, `task-127.md:397-436`), phase 4's
`Criterion{ID, Text}` (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-03.md:114-117`)
and task 139's `Service` usage of 136's `Complete`.

Closes **L-82**'s W4 half (*"propose issues, one per open criterion"*; Discover's half exists) and
**L-66** (the proposed next cycle), with the solo-mode caveat below.

**What the reader sees** (05 §1 D L-66, §1 E L-82, §4.5 Discover row, AX §7.1):

- **A feature's criteria panel gains *Propose issues*.** It is enabled when at least one
  criterion is *uncovered* (no issue serves it, §3.3) and reads *"Propose issues for 3 uncovered
  criteria"*. Pressed, it shows one draft card per uncovered criterion: the criterion's text as the
  card's head, a drafted title and description marked *drafted* in the agent class, and *"Serves
  C3"* in mono. Each card has *Accept*, *Edit* and *Discard*; the list has *Accept all*.
- **Drafts survive a restart.** Each card is a `df-` draft in the Hub, private to the person, so
  closing the app and returning shows the same cards (the Hub keeps them; the desktop keeps
  nothing on disk, `ADR-H1:467-474`).
- **Accept** creates the issue in `triage` **as the person**, with `serves` set to that card's
  criterion, its title and description still `text·A` until someone edits them, and deletes the
  draft. **Accept all** is one batch with one Undo chip: *"3 issues created · Undo"*.
- **The cycle view gains *Propose the next cycle*.** The proposal arrives as **one** Desk item in
  NEEDS YOU on the next Desk entry: *"Proposed for Cycle 14: 7 issues across 3 criteria"*. Opened in
  place, it lists issues **grouped by the criteria they serve** (a group *"Serves no criterion"*
  last), each with a checkbox (checked by default), its priority word, its due date when set and
  its blocker when one exists, and each with a *why* line whose keys are links.
- **No forecast anywhere.** The proposal never says *would settle*, *fits*, *capacity*,
  *velocity* or a percentage; the group header is a fact, *"Serves C3 · 2 issues"*.
- **Accepting the cycle** sets the cycle of each checked issue as the person, one Undo for the set.

**Decisions made here, so nobody re-derives them:**

1. ***Uncovered* is computed mechanically, never by the model.** A criterion of the feature's
   branch brief head is uncovered when no issue of the feature's project that the caller can read,
   in any state but `cancelled`, has a `serves` entry naming it (§3.3: *"a criterion no issue serves
   is a visible gap, named uncovered"*). The model is asked only for words.
2. **`serves` is set by code, never taken from the model.** The prompt lists uncovered criteria as
   `k1`…`kN`; the reply is keyed the same. The draft's `serves` is the criterion the key names, so a
   crossed or invented criterion is impossible by construction. A key not in the list is dropped;
   a key answered twice keeps the first answer; a criterion the model skipped gets **no** card and
   the panel says *"No draft for C4. Write one yourself."*
3. **The draft's body carries the proposal.** Task 72's `Draft` has one `body` field and no title
   or `serves` (gap 1). Until it gains them, a proposal draft is `kind: "issue"` whose
   `body.text` is the canonical JSON of `ProposedIssueDraft` (below), `author: "A"`, and whose
   `issueId` is `null`. `drafts.ts` (task 92) decodes that shape by its `"v": "proposal-1"` marker
   and shows every other issue draft as before.
4. **Assist routes write nothing to the Hub.** The control plane returns proposals; the desktop
   writes the `df-` drafts, the issues and the cycle moves, **as the person**, through task 37's
   client and task 56's `mutate`. So every Hub write has the person as its actor and the assist
   service holds no Hub write path at all (the same rule as task 139).
5. **The backlog read for a cycle proposal** is the team's issues the caller can read in `triage`,
   `backlog` or `todo`, with no cycle or a past cycle, at most 200 by priority then oldest (task
   77's states; `started` and later are already under way, `done` and `cancelled` are closed).
   The prompt carries each issue's alias, title, priority, due date, blocker aliases and the
   criteria it serves. **No estimate, velocity, capacity or history** is sent or computed.
6. **Grouping is mechanical.** The model returns `{issues: [{key, why: [key]}]}`; the control
   plane keeps only candidate keys, then groups the kept issues by their own `serves` (an issue
   serving two criteria appears under each, marked *also serves C2*).
7. **One Desk item per proposal, held until the next entry.** The proposal is held in memory by a
   Desk source (task 127's `reminders.ts` pattern); task 62's frame takes its snapshot on entry, so
   an item made while the Desk is open appears on the next entry and never re-ranks the open Desk.
   A new proposal for the same cycle replaces the older one.
8. **Accept all creates, and Undo cancels.** Issue creation is not a replica op (task 56's
   `WorkOp` has no create kind, `task-56.md:157-168`). *Accept all* creates the issues one by one
   through the client; its Undo is one `mutate([...])` batch of `setState` to `cancelled` with
   cause `undo`, and the drafts are written back from memory. The aliases stay consumed (gap 4).
9. **Solo mode.** The assist client makes the cycle proposal. 05 §1's *"the user's Orchestrator
   proposes"* needs team mode, and this row has no TM dependency, so the Orchestrator path is
   **UNVERIFIED** here and not built.

**Files:**

- `control-plane/internal/workassist/propose_issues.go` — `ProposeIssues`, `uncovered` (new)
- `control-plane/internal/workassist/propose_cycle.go` — `ProposeCycle`, the candidate read, grouping (new)
- `control-plane/internal/workassist/{propose_issues,propose_cycle}_test.go`, `testdata/propose/*.json` (new)
- `control-plane/internal/api/workassist.go` ⚑ (task 136) — two routes (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the two patterns (modified)
- `desktop/src/api/work.ts` ⚑ — the wire types and two calls (modified)
- `desktop/src/work/proposals.ts` — `ProposedIssueDraft` encode and decode, the cycle proposal source (new)
- `desktop/src/components/plan/ProposeIssues.tsx` — the cards, accept, accept all (new)
- `desktop/src/components/plan/CycleProposal.tsx` — the grouped proposal and its accept (new)
- `desktop/src/components/plan/CycleView.tsx` ⚑ (task 87) — *Propose the next cycle* (modified)
- `desktop/src/components/desk/deskRows.tsx` ⚑ (task 62) — registers `cycle.proposal` (modified)
- `desktop/src/work/drafts.ts` ⚑ (task 92) — recognises a proposal draft (modified; not in the scope's list)
- `desktop/src/components/plan/proposals.test.tsx` — the desktop tests (new)

#### Interfaces — complete: `control-plane/internal/workassist/propose_issues.go`

```go
package workassist

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// ProposeIssuesRequest is POST /v1/work/assist/propose-issues's body.
type ProposeIssuesRequest struct {
	FeatureID string `json:"featureId"`
}

// ProposedIssue is one draft for one uncovered criterion. Title and
// Description are always author "A"; Serves is set by code (decision 2).
type ProposedIssue struct {
	Criterion   UncoveredCriterion  `json:"criterion"`
	Title       hubapi.AuthoredText `json:"title"`
	Description hubapi.AuthoredText `json:"description"`
	Serves      []hubapi.Serve      `json:"serves"` // exactly one: {briefKey, criterion.id}
}

// UncoveredCriterion is a criterion of the branch brief's head no readable,
// non-cancelled issue serves (decision 1).
type UncoveredCriterion struct {
	BriefKey string `json:"briefKey"` // brief:bf-…@r, the head the check read
	ID       string `json:"id"`       // phase 4's c-…
	Label    string `json:"label"`    // "C3", its position in the brief, display only
	Text     string `json:"text"`     // the criterion's words, as the brief has them
}

// ProposeIssuesResponse lists one proposal per answered criterion and names
// the criteria the model left unanswered.
type ProposeIssuesResponse struct {
	FeatureID string               `json:"featureId"`
	Uncovered []UncoveredCriterion `json:"uncovered"` // every uncovered criterion, never null
	Proposals []ProposedIssue      `json:"proposals"` // at most one per uncovered criterion
	Missing   []string             `json:"missing"`   // criterion ids with no proposal
	Route     string               `json:"route"`
}

const proposeIssuesSystem = `You draft one work item for each acceptance criterion given.
Each criterion has a key like k1. For each, write a short title (under 80 characters) and a
description of the work needed to meet it. Do not merge criteria. Do not add criteria.
Answer with JSON only: {"items":[{"key":"k1","title":"…","description":"…"}]}.`

// ProposeIssues finds the uncovered criteria and asks for one draft each. With
// none uncovered it makes no call.
func (s *Service) ProposeIssues(ctx context.Context, projectID string, req ProposeIssuesRequest) (ProposeIssuesResponse, error) {
	unc, err := s.uncovered(ctx, projectID, req.FeatureID)
	if err != nil {
		return ProposeIssuesResponse{}, err
	}
	out := ProposeIssuesResponse{FeatureID: req.FeatureID, Uncovered: unc, Proposals: []ProposedIssue{}, Missing: []string{}}
	if len(unc) == 0 {
		return out, nil
	}
	byKey := make(map[string]UncoveredCriterion, len(unc))
	type pc struct{ Key, Criterion string }
	in := make([]pc, 0, len(unc))
	for i, c := range unc {
		k := "k" + strconv.Itoa(i+1)
		byKey[k] = c
		in = append(in, pc{Key: k, Criterion: c.Text})
	}
	user, _ := json.Marshal(map[string]any{"criteria": in})
	reply, err := s.Complete(ctx, projectID, Call{System: proposeIssuesSystem, User: string(user), MaxTokens: 400 * len(unc)})
	if err != nil {
		return ProposeIssuesResponse{}, err
	}
	out.Route = string(reply.Route)
	var raw struct {
		Items []struct{ Key, Title, Description string } `json:"items"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(reply.Text)), &raw) // unparseable: every criterion missing
	done := map[string]bool{}
	for _, it := range raw.Items {
		c, ok := byKey[it.Key]
		title := strings.TrimSpace(it.Title)
		if !ok || done[it.Key] || title == "" || len(title) > 200 {
			continue // decision 2: unknown key, second answer, or no usable title
		}
		done[it.Key] = true
		out.Proposals = append(out.Proposals, ProposedIssue{Criterion: c,
			Title:       hubapi.AuthoredText{Text: title, Author: hubapi.AuthorAgent}, // task-02.md:226-229
			Description: hubapi.AuthoredText{Text: strings.TrimSpace(it.Description), Author: hubapi.AuthorAgent},
			Serves:      []hubapi.Serve{{BriefKey: c.BriefKey, CriterionID: c.ID}}})
	}
	for i, c := range unc {
		if !done["k"+strconv.Itoa(i+1)] {
			out.Missing = append(out.Missing, c.ID)
		}
	}
	return out, nil
}
```

`uncovered` reads the feature (task 76's head, its branch brief id; **UNVERIFIED** field name, the
same gap task 127 names), the brief head through `hubclient.Brief(ctx, briefID)`
(`.claude/worktrees/hub/control-plane/internal/hub/hubclient/hubclient.go:57`, `BriefWithRevision`,
`hubapi/types.go:167-180`), decodes `Revision.Content`'s criteria with phase 4's `Criterion{ID,
Text}` decoder (P4 task 03, **UNVERIFIED until merge**, rule H2), and lists the project's issues
through the relay as the caller (task 72's issue list with a `serves` filter, **UNVERIFIED** method
name). A criterion is kept when no listed issue in a state other than `cancelled` has a `Serve`
whose `CriterionID` equals its id **and** whose `BriefKey` names the same brief at any revision
(an issue serving `@5` still covers the criterion at `@6` when the id is unchanged; a drifted
criterion is task 10's `drifting`, not *uncovered*). An erased brief revision (`Content` null) gives
no criteria and no call.

#### Interfaces — complete: `control-plane/internal/workassist/propose_cycle.go`

```go
package workassist

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// ProposeCycleRequest is POST /v1/work/assist/propose-cycle's body.
type ProposeCycleRequest struct {
	TeamID  string `json:"teamId"`
	CycleID string `json:"cycleId"` // cy-…, the upcoming cycle the proposal is for
}

// CycleCandidate is one backlog issue as the prompt sees it (decision 5).
// There is no estimate, capacity or history field, by design.
type CycleCandidate struct {
	Key       string         `json:"key"`   // i1…iN, local to one call
	IssueID   string         `json:"-"`     // is-…, never sent
	Alias     string         `json:"alias"` // EP-142
	Title     string         `json:"title"`
	Priority  string         `json:"priority"`
	DueOn     *string        `json:"dueOn"`
	BlockedBy []string       `json:"blockedBy"` // aliases of open blockers
	Serves    []hubapi.Serve `json:"-"`         // grouping input, never sent
	Criteria  []string       `json:"criteria"`  // "C3" labels, for the model to read
}

// ProposedCycleIssue is one kept pick, with the keys of its why.
type ProposedCycleIssue struct {
	IssueID string `json:"issueId"`
	Alias   string `json:"alias"`
	Why     []Why  `json:"why"` // task 136's Why {kind, key}; at least one, each resolvable
}

// CriterionGroup is a fact: these picks serve this criterion (decision 6).
type CriterionGroup struct {
	Criterion *hubapi.Serve        `json:"criterion"` // null: "Serves no criterion"
	Label     string               `json:"label"`     // "C3", or ""
	Issues    []ProposedCycleIssue `json:"issues"`
}

// CycleProposal is returned to the desktop, which holds it as one Desk item.
type CycleProposal struct {
	TeamID  string           `json:"teamId"`
	CycleID string           `json:"cycleId"`
	Groups  []CriterionGroup `json:"groups"` // criteria in brief order; the null group last
	Dropped int              `json:"dropped"`
	Route   string           `json:"route"`
	MadeAt  string           `json:"madeAt"`
}

const maxCycleCandidates = 200

const proposeCycleSystem = `You suggest which backlog items a team could take into its next cycle.
Each item has a key like i1, a priority, an optional due date, blockers and the criteria it serves.
Prefer urgent and high priority, near due dates, and items nothing blocks.
For each pick, give the item keys and criterion labels that explain it.
Do not estimate effort, capacity or completion. Answer with JSON only:
{"issues":[{"key":"i3","why":["i3","C3"]}]}.`
```

`ProposeCycle(ctx, projectID string, req) (CycleProposal, error)` does four things in order:

1. **Candidates.** Through the relay as the caller: the team's issues in `triage`, `backlog` or
   `todo` whose cycle is null or a past cycle, sorted by priority (`urgent`, `high`, `medium`,
   `low`, `none`) then `createdAt`, the first `maxCycleCandidates`. The Hub's RLS already hides
   unreadable issues; the service also drops any issue whose project the caller cannot read (task
   136's `readable`), so a relay change cannot leak one. Erased issues (`erasedAt` set) are dropped
   before the prompt.
2. **The call** through `s.Complete` with the candidates' public fields only (`json:"-"` keeps ids
   and serves on the control plane).
3. **Keep.** A pick whose `key` is not a candidate is dropped (counted); each `why` entry must be a
   candidate key (mapped to `{kind: "issue", key: "issue:is-…"}`) or a criterion label present in
   that candidate's `Criteria` (mapped to `{kind: "criterion", key: brief key + criterion id}`);
   a pick left with no why is dropped (AX Principle 7, task 136's rule).
4. **Group** kept picks by each one's own `Serves` (decision 6), in the brief's criterion order,
   the no-criterion group last. `MadeAt` is the control plane's clock.

The words *settle*, *fit*, *capacity*, *velocity*, *estimate*, *points* and a `%` never appear in
`CycleProposal`: it has no free-text field, only aliases, keys and labels, which a reflection test
over its JSON field names pins with the OD-27 list (`percent`, `progress`, `eta`, `forecast`,
`remaining`, `capacity`, `velocity`).

#### Interfaces — complete: the routes, appended to `control-plane/internal/api/workassist.go` ⚑

```go
// Task 140: proposals. Both POST; neither writes to the Hub (decision 4).
func (a *API) registerWorkProposals(rs *routes) {
	rs.HandleFunc("POST /v1/work/assist/propose-issues", a.postProposeIssues)
	rs.HandleFunc("POST /v1/work/assist/propose-cycle", a.postProposeCycle)
}

func (a *API) postProposeIssues(w http.ResponseWriter, r *http.Request) {
	var req workassist.ProposeIssuesRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	projectID, ok := a.featureProject(w, r, req.FeatureID) // task 136; the Hub's 404 when unreadable
	if !ok {
		return
	}
	res, err := a.deps.WorkAssist.ProposeIssues(r.Context(), projectID, req)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a *API) postProposeCycle(w http.ResponseWriter, r *http.Request) {
	var req workassist.ProposeCycleRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	projectID, ok := a.cycleProject(w, r, req.TeamID, req.CycleID) // the team's project for the route
	if !ok {
		return
	}
	res, err := a.deps.WorkAssist.ProposeCycle(r.Context(), projectID, req)
	if err != nil {
		a.assistProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
```

`cycleProject` (new here) reads the cycle through the relay as the caller and answers `404` when
the cycle is not the team's or is unreadable, `409 cycle_not_upcoming` when its `startsOn` is past
(a new control-plane code, appended beside task 139's `thread_too_short`). A team's backlog spans
its projects; **the route is the tightest of them**: when any candidate's project is `au-only`, the
call goes AU (`dataclass.Tighter`, P6 task 01), so AU data never leaves on Global. `cycleProject`
therefore returns the project whose route is tightest, and `ProposeCycle` re-checks it after the
candidate read.

#### Interfaces — complete: `desktop/src/api/work.ts` ⚑, appended

```ts
// Task 140. Mirrors workassist's proposal types.
export interface UncoveredCriterion { briefKey: string; id: string; label: string; text: string }
export interface ProposedIssue {
  criterion: UncoveredCriterion; title: AuthoredText; description: AuthoredText; serves: Serve[];
}
export interface ProposeIssuesResponse {
  featureId: string; uncovered: UncoveredCriterion[]; proposals: ProposedIssue[]; missing: string[];
  route: "global" | "au";
}
export interface ProposedCycleIssue { issueId: string; alias: string; why: Why[] }
export interface CriterionGroup { criterion: Serve | null; label: string; issues: ProposedCycleIssue[] }
export interface CycleProposal {
  teamId: string; cycleId: string; groups: CriterionGroup[]; dropped: number;
  route: "global" | "au"; madeAt: string;
}
export interface WorkProposalsClient {
  proposeIssues(featureId: string): Promise<ProposeIssuesResponse>;
  proposeCycle(teamId: string, cycleId: string): Promise<CycleProposal>;
}
```

`Why` is task 136's desktop type (`{kind, key}`, **UNVERIFIED** until its part); `Serve` and
`AuthoredText` are task 37's mirrors of `task-02.md:226-235`. `WorkClient` extends
`WorkProposalsClient`.

#### Interfaces — complete: `desktop/src/work/proposals.ts`

```ts
// desktop/src/work/proposals.ts — decision 3 (the draft's body) and decision 7 (the Desk source).
import type { AuthoredText, CycleProposal, Draft, ProposedIssue, Serve } from "../api/work";

export const PROPOSAL_V = "proposal-1";

/** What a proposal draft's body.text holds, canonical JSON (sorted keys, no spaces). */
export interface ProposedIssueDraft {
  v: typeof PROPOSAL_V; featureId: string; criterionLabel: string; criterionText: string;
  title: AuthoredText; description: AuthoredText; serves: Serve[];
}

const canon = (v: unknown): string => JSON.stringify(v, (_k, x) =>
  x && typeof x === "object" && !Array.isArray(x)
    ? Object.fromEntries(Object.keys(x).sort().map((k) => [k, (x as Record<string, unknown>)[k]])) : x);

export function toDraftBody(featureId: string, p: ProposedIssue): AuthoredText {
  const d: ProposedIssueDraft = { v: PROPOSAL_V, featureId, criterionLabel: p.criterion.label,
    criterionText: p.criterion.text, title: p.title, description: p.description, serves: p.serves };
  return { text: canon(d), author: "A" };
}

/** A proposal draft, or null for an ordinary issue draft (task 92 keeps showing those). */
export function fromDraft(d: Draft): ProposedIssueDraft | null {
  if (d.kind !== "issue" || d.issueId !== null || d.body.author !== "A") return null;
  try {
    const x = JSON.parse(d.body.text) as Partial<ProposedIssueDraft>;
    if (x.v !== PROPOSAL_V || !Array.isArray(x.serves) || x.serves.length !== 1 || !x.title) return null;
    return x as ProposedIssueDraft;
  } catch { return null; }
}

// Decision 7: one held proposal per cycle, in memory; the Desk reads it on entry.
const byCycle = new Map<string, CycleProposal>();
export const holdCycleProposal = (p: CycleProposal): void => { byCycle.set(p.cycleId, p); };
export const clearCycleProposal = (cycleId: string): void => { byCycle.delete(cycleId); };
export const cycleProposals = (): readonly CycleProposal[] => [...byCycle.values()];
```

#### Interfaces — complete: `desktop/src/components/plan/ProposeIssues.tsx`

```tsx
// desktop/src/components/plan/ProposeIssues.tsx — L-82's W4 half. One draft per uncovered criterion.
import { useEffect, useState } from "react";
import type { Draft } from "../../api/work";
import { useWorkClient } from "../../api/useWorkClient";
import { ApiError } from "../../api/client";
import { mutate } from "../../work/replica/useWork";
import { encodeDraft, fromDraft, toDraftBody, type ProposedIssueDraft } from "../../work/proposals";
import { offerUndo } from "../common/UndoChip";                  // task 56's chip (UNVERIFIED export name)
import { Editor } from "../editor/Editor";
import { docFromAuthored, toAuthored } from "../editor/authorClass";

export interface ProposeIssuesProps { featureId: string; projectId: string; uncoveredCount: number }

type Card = { draftId: string; p: ProposedIssueDraft };

export function ProposeIssues(props: ProposeIssuesProps): JSX.Element {
  const work = useWorkClient();
  const [cards, setCards] = useState<Card[]>([]);
  const [missing, setMissing] = useState<string[]>([]);
  const [note, setNote] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);

  // Decision 3: the cards are the person's df- drafts for this feature, read from the Hub on mount.
  useEffect(() => {
    work.listDrafts().then((ds: Draft[]) => setCards(ds.flatMap((d) => {
      const p = fromDraft(d);
      return p && p.featureId === props.featureId ? [{ draftId: d.id, p }] : [];
    })));
  }, [props.featureId]);

  const propose = async () => {
    setNote(null);
    try {
      const r = await work.proposeIssues(props.featureId);
      const saved = await Promise.all(r.proposals.map((p) =>
        work.saveDraft({ kind: "issue", issueId: null, body: toDraftBody(props.featureId, p) })));
      setCards((c) => [...c, ...saved.map((d) => ({ draftId: d.id, p: fromDraft(d)! }))]);
      setMissing(r.uncovered.filter((u) => r.missing.includes(u.id)).map((u) => u.label));
    } catch (e) {
      setNote(e instanceof ApiError && e.code === "assists_off"
        ? "Proposals unavailable: assists are off for this org." : "Proposals unavailable: the gateway did not answer.");
    }
  };

  const create = async (c: Card) => {
    const issue = await work.createIssue(props.projectId, {   // as the person; task 03's CreateIssueRequest
      title: c.p.title, description: c.p.description, serves: c.p.serves, implements: [],
      state: "triage", priority: "none", assigneeId: null, labels: [], notTiedReason: "" });
    await work.deleteDraft(c.draftId);
    return issue;
  };

  const accept = async (c: Card) => { await create(c); setCards((cs) => cs.filter((x) => x.draftId !== c.draftId)); };

  const acceptAll = async () => {                               // decision 8: one Undo for the set
    const all = cards;
    const made = [];
    for (const c of all) made.push(await create(c));
    setCards([]);
    offerUndo(`${made.length} issues created`, async () => {
      mutate(made.map((i) => ({ kind: "setState", issueId: i.id,
        move: { to: "cancelled", reason: "Undone: proposed issues accepted by mistake" } })));   // one batch
      const back = await Promise.all(all.map((c) => work.saveDraft({ kind: "issue", issueId: null, body: encodeDraft(c.p) })));
      setCards(back.map((d) => ({ draftId: d.id, p: fromDraft(d)! })));
    });
  };

  const discard = async (c: Card) => { await work.deleteDraft(c.draftId); setCards((cs) => cs.filter((x) => x !== c)); };

  return (
    <section className="pi" aria-label="Proposed issues">
      {props.uncoveredCount > 0 && cards.length === 0 && (
        <button type="button" onClick={propose}>
          Propose issues for {props.uncoveredCount} uncovered {props.uncoveredCount === 1 ? "criterion" : "criteria"}
        </button>)}
      {note && <p className="pi-note" role="status">{note}</p>}
      {missing.map((l) => <p key={l} className="pi-note">No draft for {l}. Write one yourself.</p>)}
      {cards.map((c) => (
        <article key={c.draftId} className="pi-card" data-author="A">
          <header className="pi-crit"><span className="pi-label">{c.p.criterionLabel}</span> {c.p.criterionText}</header>
          {editing === c.draftId
            ? <Editor value={docFromAuthored(c.p.title)} ariaLabel="Title" onCommit={(d) => {
                const p = { ...c.p, title: toAuthored(d) };
                void work.saveDraft({ id: c.draftId, kind: "issue", issueId: null, body: encodeDraft(p) });
                setCards((cs) => cs.map((x) => (x.draftId === c.draftId ? { ...x, p } : x)));
                setEditing(null); }} />
            : <h4 className="pi-title"><span className="pi-drafted">Drafted</span> {c.p.title.text}</h4>}
          <p className="pi-serves">Serves {c.p.criterionLabel}</p>
          <div className="pi-acts">
            <button type="button" onClick={() => accept(c)}>Accept</button>
            <button type="button" onClick={() => setEditing(c.draftId)}>Edit</button>
            <button type="button" onClick={() => discard(c)}>Discard</button>
          </div>
        </article>))}
      {cards.length > 1 && <button type="button" onClick={acceptAll}>Accept all</button>}
    </section>
  );
}
```

The editor's commit keeps task 49's rule: a title the person rewrote is `H`, one left as drafted
stays `A` (`toAuthored`), so an accepted issue's revision carries the class the text really has.
`encodeDraft(p)` is `proposals.ts`'s `{text: canon(p), author: "A"}` (added beside `toDraftBody`,
which calls it), so every save of a proposal draft is the same canonical bytes. `listDrafts`,
`saveDraft`, `deleteDraft` and `createIssue` are task 37's client over task 80's `/v1/me/drafts`
routes and task 04's issue create (**UNVERIFIED** method names). The Undo is task 56's chip offered
with a callback, because the inverse of a create is not a `WorkOp` (`task-56.md:157-168` has no
create kind); inside it, the cancel moves are **one** `mutate([...])` batch (`task-56.md:47-48`),
and `cancelled` with a reason is a person's move, never a fact state (skeleton rule 2). Whether the
chip takes a callback is a gap (gap 3).
