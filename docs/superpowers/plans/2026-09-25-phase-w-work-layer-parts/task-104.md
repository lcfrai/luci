### Task 104: proposed evidence under each criterion in the Merge lens; *Record* as the person

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on task
23 (the done evaluator and *observed* evidence), task 100 (SW.19 on the real Hub and the W3 relay)
and phase 3 task 20 (the Merge lens's `EvidenceDialog`; phase 3 plan `:558-559`, **UNVERIFIED
until merge**). Uses SW.19's `evidence_proposal` comment (task 99), phase 3's `evidence.Store`
(`task-15.md:68-75` in the phase 3 worktree), phase 4's `DefinitionOfDone` and its planned
`onRecordEvidence?(criterionId)` prop (S4-P3.3, phase 4 plan `:483`), the Hub's `POST /v1/evidence`
(`hub-contract.md:1062-1083`: *"`recordedBy` is the principal"*, `:1074`) and SW.8.

An agent may **propose** evidence; only a person **records** it (05 §2.1 point 4, 04 §4.6). This
part draws the proposal where the person already judges criteria, under the criterion it names,
and makes *Record* the one way a proposal becomes evidence. Three rules hold it:

1. **A proposal is a comment, never a record.** It lives in the Hub's comment table as kind
   `evidence_proposal` (SW.19) and never in the evidence table (task 100's guard). Phase 4's
   convergence word for the criterion is computed from `evidence.Store` alone, so a proposal
   cannot move it; this part adds nothing to that computation.
2. **Record is written by the person, at the proposal's commit, with a because.** The desktop
   calls phase 3's `recordEvidence` on the loopback control plane, which holds only the signed-in
   person's Hub session (`OD-6`); no agent token exists on the desktop to send. The record's
   `recordedBy` is the person's identity (phase 3 `Record.RecordedBy`, the Hub's principal).
3. **Observed evidence needs no click.** Where task 23's `Observer` already wrote an `Observed`
   record for that criterion at that commit (`task-23.md:126`), the row reads *observed* and has
   no Record: the fact is there already, and a second, typed record would add nothing.

**Decisions made here, so nobody re-derives them:**

1. **Status is resolved in the control plane, not guessed on the desktop.** One new read,
   `GET /v1/projects/{p}/evidence/proposals?issue=|brief=`, returns each proposal with a
   `status` of `open`, `observed` or `recorded` and the `evidenceId` behind the last two. It reads
   the Hub's proposals (task 99's `ListEvidenceProposals`) and this project's `evidence.Store`
   (`ByDigest`) together, so the answer is the same in the issue view and the Merge lens.
2. **Recorded means any record at that commit for that criterion**, the proposal's own
   `recordedAs` or not. A person who recorded the same criterion at the same commit by hand has
   already judged it; offering Record again would make a second record of one judgement.
3. **The link back is a second write, and its failure never offers a second Record.** Phase 3's
   record body gains one optional field, `proposalId` (an added field on the loopback API, not a
   Hub edit). With it, the control plane checks the proposal's criterion and commit equal the
   body's, records, then calls task 100's link route as the person. If the link fails, the answer
   is `{record, linked: false}`; the row reads *recorded as ev-… · not yet linked on the Hub* with
   **Link again**, and decision 2 already reads it as `recorded`.
4. **The commit is the proposal's.** *Record* opens `EvidenceDialog` with the commit fixed to the
   proposal's and shown in mono. A person who wants another commit closes it and uses phase 3's own
   *Record evidence…*, which sends no `proposalId`.
5. **The because is prefilled, editable and marked.** The dialog opens with the agent's `because`
   under the `text·A` mark (§2.1 point 3); the mark drops as soon as the person edits a character.
   Recording it unchanged is the person adopting those words: the record carries no author class,
   and this part says so rather than claim otherwise.
6. **A local-only store records locally and does not link.** Phase 3 gives a `local:` repo the
   `FileStore` (`task-15.md:50`); the Hub cannot check a record it cannot see, so no link is sent
   and the row reads *recorded on this machine only* (phase 3 task 20's sentence, `task-20.md:119`).
7. **Under its own criterion and nowhere else.** The Merge lens renders proposals through one
   added optional prop on phase 4's `DefinitionOfDone`, `criterionFooter?(criterionId)`, an ask to
   phase 4 beside S4-P3.3. Until it lands, phase 3 task 20's own fallback shape is used: under the
   component, one block per criterion with its text verbatim and its proposals (`task-20.md:112-115`).

**Files:**
- `control-plane/internal/evidence/proposals.go` (+ `proposals_test.go`) — `ResolveProposals`,
  `RecordProposal` (new, in phase 3's package ⚑; skeleton delta row `:283`)
- `control-plane/internal/api/evidence.go` ⚑ (phase 3's evidence routes; path **UNVERIFIED until
  merge**) — the proposals read and the record body's `proposalId` (modified);
  `routes_audit_test.go` ⚑ — one catalog row (modified)
- `desktop/src/api/work.ts` ⚑ (task 37's) — the W3 proposal types, appended (modified; see gap 1)
- `desktop/src/api/client.ts` ⚑ — `listEvidenceProposals`, `EvidenceInput.proposalId?` (modified)
- `desktop/src/components/issue/ProposedEvidence.tsx` (+ `ProposedEvidence.test.tsx`) — the row (new)
- `desktop/src/components/issue/Serves.tsx` ⚑ (task 47's) — the rows under each served criterion
- `desktop/src/components/lens/merge/EvidenceDialog.tsx` ⚑, `LaneCard.tsx` ⚑ (phase 3 tasks 19-20)
  — the `proposal` seed and the mount (modified; `EvidenceDialog.proposal.test.tsx` new)
- `desktop/src/provenance/DefinitionOfDone.tsx` ⚑ (phase 4 task 17) — `criterionFooter?` (modified)

#### Interfaces — complete: `control-plane/internal/evidence/proposals.go`

```go
package evidence

import (
	"context"
	"errors"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// ProposalStatus is what the person sees on a proposal row. Closed set.
type ProposalStatus string

const (
	ProposalOpen     ProposalStatus = "open"     // no record at this commit for this criterion: Record offered
	ProposalObserved ProposalStatus = "observed" // an Observed record exists (task 23): no Record
	ProposalRecorded ProposalStatus = "recorded" // a person's record exists, linked or not: no Record
	ProposalGone     ProposalStatus = "criterion_gone" // the brief no longer has this criterion: no Record
)

// ProposalView is one proposal with its resolved status. Proposal is task 99's wire type, kept
// whole so the desktop renders its attribution (L-173) and its because as text·A.
type ProposalView struct {
	Proposal   hubapi.EvidenceProposalComment `json:"proposal"`
	Status     ProposalStatus                 `json:"status"`
	EvidenceID string                         `json:"evidenceId,omitempty"` // "" when open
	Linked     bool                           `json:"linked"`               // Proposal.RecordedAs == EvidenceID
	Shared     bool                           `json:"shared"`               // Store.Shared(): false reads "on this machine only"
}

// DigestOf answers a criterion's digest from the brief the proposal names; it is the resolver
// phase 3's evidenceStates route already uses (P3 task 16; its name is read on the sync base).
type DigestOf func(ctx context.Context, briefKey, criterionID string) (string, bool, error)

// ResolveProposals reads the store once per distinct digest and gives every proposal a status.
// A proposal whose criterion no longer resolves is returned as criterion_gone, never dropped:
// the row says so, and nothing disappears silently.
func ResolveProposals(ctx context.Context, s Store, repo string, digest DigestOf,
	ps []hubapi.EvidenceProposalComment) ([]ProposalView, error)

// ProposalRecord is the person's Record of one proposal. Identity is the signed-in person's git
// identity ("Name <email>", phase 3's RecordedBy); it never comes from the proposal.
type ProposalRecord struct {
	Repo, ProjectID string
	Proposal        hubapi.EvidenceProposalComment
	CriterionDigest string
	Commit          string // must equal Proposal.Proposal.Commit (decision 4)
	Verdict         Verdict
	Because         string // trimmed; 1-2000 characters (phase 3's P12)
	Identity        string
}

// Linker is the one Hub call this file makes: task 100's link route, sent with the person's own
// Hub session. Its name is task 99's; this is the shape the part needs (gap 2).
type Linker interface {
	LinkEvidenceProposal(ctx context.Context, proposalID, evidenceID string) (hubapi.EvidenceProposalComment, error)
}

var (
	ErrProposalMismatch = errors.New("evidence: the body's criterion or commit is not the proposal's")
	ErrAlreadySettled   = errors.New("evidence: this criterion already has a record at this commit")
)

// RecordProposal checks the body against the proposal, refuses a second record of one judgement,
// appends through Store.Record, then links when Store.Shared(). A link failure is returned as
// linked == false with a nil error: the record stands, and Record is never offered again.
func RecordProposal(ctx context.Context, s Store, l Linker, in ProposalRecord) (rec Record, linked bool, err error) {
	p := in.Proposal.Proposal
	if in.Commit != p.Commit || in.Proposal.Kind != hubapi.CommentEvidenceProposal {
		return Record{}, false, ErrProposalMismatch
	}
	prior, err := s.ByDigest(ctx, in.Repo, []string{in.CriterionDigest})
	if err != nil {
		return Record{}, false, err
	}
	for _, r := range prior {
		if r.Commit != in.Commit || r.CriterionID != p.CriterionID {
			continue
		}
		linked = in.Proposal.RecordedAs == r.ID
		// Link again (the row's retry): the person's own unlinked record is linked, never re-written.
		if !linked && s.Shared() && r.RecordedBy == in.Identity {
			_, lerr := l.LinkEvidenceProposal(ctx, in.Proposal.ID, r.ID)
			linked = lerr == nil
		}
		return r, linked, ErrAlreadySettled
	}
	rec, err = s.Record(ctx, Record{Repo: in.Repo, ProjectID: in.ProjectID, BriefKey: p.BriefKey,
		CriterionID: p.CriterionID, CriterionDigest: in.CriterionDigest, Commit: in.Commit,
		Verdict: in.Verdict, Because: strings.TrimSpace(in.Because), RecordedBy: in.Identity,
		IssueID: p.IssueID}) // Observed stays false: a person typed this (task 23's rule)
	if err != nil || !s.Shared() {
		return rec, false, err
	}
	if _, lerr := l.LinkEvidenceProposal(ctx, in.Proposal.ID, rec.ID); lerr != nil {
		return rec, false, nil
	}
	return rec, true, nil
}
```

`ResolveProposals`'s body is the rule table below, one pass over the records for each digest:

| Records at `(criterionDigest, proposal.commit)` for `proposal.criterionId` | `Status` | `EvidenceID` |
|---|---|---|
| none | `open` | `""` |
| at least one with `Observed` | `observed` | the first observed record's id |
| only records without `Observed` | `recorded` | `proposal.recordedAs` if it is among them, else the oldest |
| the criterion no longer resolves (`DigestOf` answers `false`) | `criterion_gone` | `""` |

**The route**, appended to phase 3's evidence routes (`internal/api/evidence.go`, **UNVERIFIED
until merge**) with one `routes_audit_test.go` catalog row:

```text
GET /v1/projects/{p}/evidence/proposals?issue=<is-…>      exactly one of issue, brief
GET /v1/projects/{p}/evidence/proposals?brief=<brief key>
  200 {"proposals": [ProposalView, …]}   oldest first; [] when none
  400 {"code": "bad_query"}              neither or both parameters
  503 {"code": "hub_unavailable"}        the Hub read failed; the desktop keeps its last answer
POST /v1/projects/{p}/evidence           phase 3's body + "proposalId": "ic-…" (optional, added)
  201 {"record": Record, "linked": bool}
  409 {"code": "proposal_mismatch"} | {"code": "already_settled", "evidenceId": "ev-…"}
```

`already_settled` returns the existing id so the row can link to it; it is the answer to a
double click, a second window, or a teammate who recorded first.


#### Tests to write first — `control-plane/internal/evidence/proposals_test.go`

```go
package evidence

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

const (
	repo   = "github.com/example/app"
	c40    = "c-40aa17c3"
	dig40  = "sha256:40"
	tip    = "7c1f0a2e5b9d4c3a2f1e0d9c8b7a6f5e4d3c2b1a"
	other  = "0000000000000000000000000000000000000001"
	person = "Dylan Example <dylan@example.test>"
)

func proposal(commit string) hubapi.EvidenceProposalComment {
	return hubapi.EvidenceProposalComment{ID: "ic-00000000000000p1", Kind: hubapi.CommentEvidenceProposal,
		Proposal: hubapi.EvidenceProposal{BriefKey: "brief:bf-00000000000000a2@2", CriterionID: c40,
			IssueID: "is-3f2a91c07b1e4d2a", Commit: commit, Because: "pytest -q tests/upload passed"},
		Attribution: hubapi.AgentAttribution{OwnerID: "u-d1", AgentID: "ag-2", Model: "m", SessionID: "se-9"}}
}

// memStore is Store in memory; shared decides whether a link is sent.
type memStore struct{ recs []Record; shared bool }

func (m *memStore) Record(_ context.Context, r Record) (Record, error) {
	r.ID = "ev-00000000000" + string(rune('a'+len(m.recs)))
	m.recs = append(m.recs, r)
	return r, nil
}
func (m *memStore) ByDigest(_ context.Context, _ string, _ []string) ([]Record, error) { return m.recs, nil }
func (m *memStore) Shared() bool                                                   { return m.shared }

type linker struct{ calls []string; fail bool }

func (l *linker) LinkEvidenceProposal(_ context.Context, p, ev string) (hubapi.EvidenceProposalComment, error) {
	l.calls = append(l.calls, p+"→"+ev)
	if l.fail {
		return hubapi.EvidenceProposalComment{}, errors.New("hub down")
	}
	return hubapi.EvidenceProposalComment{ID: p, RecordedAs: ev}, nil
}

func in(commit string) ProposalRecord {
	return ProposalRecord{Repo: repo, ProjectID: "p1", Proposal: proposal(tip), CriterionDigest: dig40,
		Commit: commit, Verdict: Settled, Because: "  I ran it; upload works  ", Identity: person}
}

func TestRecordIsThePersonsAndNeverTheAgents(t *testing.T) {
	s, l := &memStore{shared: true}, &linker{}
	rec, linked, err := RecordProposal(context.Background(), s, l, in(tip))
	if err != nil || !linked || rec.RecordedBy != person || rec.Observed {
		t.Fatalf("got %+v linked=%v err=%v; want the person's, linked, not observed", rec, linked, err)
	}
	if rec.Because != "I ran it; upload works" || rec.IssueID != "is-3f2a91c07b1e4d2a" {
		t.Fatalf("because %q issue %q", rec.Because, rec.IssueID)
	}
}

func TestAnotherCommitIsNotThisProposal(t *testing.T) {
	_, _, err := RecordProposal(context.Background(), &memStore{shared: true}, &linker{}, in(other))
	if !errors.Is(err, ErrProposalMismatch) {
		t.Fatalf("err = %v, want ErrProposalMismatch", err)
	}
}

func TestASecondRecordOfOneJudgementIsRefused(t *testing.T) {
	s, l := &memStore{shared: true}, &linker{}
	first, _, _ := RecordProposal(context.Background(), s, l, in(tip))
	again, _, err := RecordProposal(context.Background(), s, l, in(tip))
	if !errors.Is(err, ErrAlreadySettled) || again.ID != first.ID || len(s.recs) != 1 {
		t.Fatalf("err=%v id=%s records=%d; want ErrAlreadySettled, the first id, one record", err, again.ID, len(s.recs))
	}
}

func TestALinkFailureKeepsTheRecordAndReportsUnlinked(t *testing.T) {
	s := &memStore{shared: true}
	rec, linked, err := RecordProposal(context.Background(), s, &linker{fail: true}, in(tip))
	if err != nil || linked || rec.ID == "" || len(s.recs) != 1 {
		t.Fatalf("rec=%+v linked=%v err=%v", rec, linked, err)
	}
}

func TestALocalStoreRecordsAndNeverLinks(t *testing.T) {
	l := &linker{}
	if _, linked, err := RecordProposal(context.Background(), &memStore{}, l, in(tip)); err != nil || linked || len(l.calls) != 0 {
		t.Fatalf("linked=%v calls=%v err=%v", linked, l.calls, err)
	}
}
```

```go
func digestOf(_ context.Context, _, id string) (string, bool, error) { return dig40, id == c40, nil }

func TestObservedEvidenceNeedsNoRecord(t *testing.T) {
	s := &memStore{recs: []Record{{ID: "ev-obs", CriterionID: c40, Commit: tip, Observed: true}}}
	v, err := ResolveProposals(context.Background(), s, repo, digestOf, []hubapi.EvidenceProposalComment{proposal(tip)})
	if err != nil || v[0].Status != ProposalObserved || v[0].EvidenceID != "ev-obs" {
		t.Fatalf("got %+v err=%v; want observed ev-obs", v, err)
	}
}

func TestAHandRecordAtTheSameCommitCountsAsRecorded(t *testing.T) {
	s := &memStore{shared: true, recs: []Record{{ID: "ev-hand", CriterionID: c40, Commit: tip}}}
	v, _ := ResolveProposals(context.Background(), s, repo, digestOf, []hubapi.EvidenceProposalComment{proposal(tip)})
	if v[0].Status != ProposalRecorded || v[0].EvidenceID != "ev-hand" || v[0].Linked {
		t.Fatalf("got %+v; want recorded ev-hand, not linked", v[0])
	}
}

func TestARecordAtAnotherCommitLeavesTheProposalOpen(t *testing.T) {
	s := &memStore{recs: []Record{{ID: "ev-x", CriterionID: c40, Commit: other}}}
	v, _ := ResolveProposals(context.Background(), s, repo, digestOf, []hubapi.EvidenceProposalComment{proposal(tip)})
	if v[0].Status != ProposalOpen || v[0].EvidenceID != "" {
		t.Fatalf("got %+v; want open", v[0])
	}
}

func TestAGoneCriterionIsReturnedNotDropped(t *testing.T) {
	p := proposal(tip)
	p.Proposal.CriterionID = "c-deadbeef"
	v, _ := ResolveProposals(context.Background(), &memStore{}, repo, digestOf, []hubapi.EvidenceProposalComment{p})
	if len(v) != 1 || v[0].Status != ProposalGone {
		t.Fatalf("got %+v; want one criterion_gone row", v)
	}
}
```

| Test | Mutant it kills |
|---|---|
| `TestRecordIsThePersonsAndNeverTheAgents` | `RecordedBy` taken from the proposal's attribution ("recorded as the agent"); `Observed` set on a typed record; the because stored untrimmed |
| `TestAnotherCommitIsNotThisProposal` | a proposal linked to a record at a commit the agent never named |
| `TestASecondRecordOfOneJudgementIsRefused` | "double record": two records of one criterion at one commit |
| `TestALinkFailureKeepsTheRecordAndReportsUnlinked` | a link failure rolled back as an error, which re-offers Record |
| `TestALocalStoreRecordsAndNeverLinks` | a link to a record the Hub cannot see |
| `TestObservedEvidenceNeedsNoRecord` | "observed evidence needs a click" (§2.1 point 4) |
| `TestAHandRecordAtTheSameCommitCountsAsRecorded` | status read only from `recordedAs`, so a hand record leaves Record offered |
| `TestARecordAtAnotherCommitLeavesTheProposalOpen` | status matched by criterion alone, ignoring the commit |
| `TestAGoneCriterionIsReturnedNotDropped` | a proposal silently dropped when its brief moved on |

The route's own tests (`internal/api/evidence_test.go` ⚑, phase 3's file, three rows appended):
`?issue=` and `?brief=` together are 400 `bad_query` (mutant: one silently wins); a body with
`proposalId` and another commit is 409 `proposal_mismatch` (mutant: the check skipped at the
route); the catalog row exists (`routes_audit_test.go`, mutant: an unlisted route).

#### Interfaces — complete: appended to `desktop/src/api/work.ts` and `client.ts`

```ts
// work.ts — the W3 proposal wire, as task 99 names it (SW.19). The first W3 desktop task to land
// adds these; a later one re-reads and keeps them (gap 1).
export type CommentKind = "comment" | "note" | "evidence_proposal" | "negotiation";
export interface AgentAttribution { ownerId: string; agentId: string; model: string; sessionId: string }
export interface EvidenceProposal {
  briefKey: string; criterionId: string; issueId: string; commit: string;
  artefact: { kind: string; ref: string };   // task 99's closed set; rendered verbatim in mono
  because: string;                           // always text·A: an agent wrote it
}
export interface EvidenceProposalComment {
  id: string; issueId: string; kind: "evidence_proposal"; proposal: EvidenceProposal;
  attribution: AgentAttribution; recordedAs: string | null; createdAt: string; erasedAt: string | null;
}
export type ProposalStatus = "open" | "observed" | "recorded" | "criterion_gone";
export interface ProposalView {
  proposal: EvidenceProposalComment; status: ProposalStatus; evidenceId?: string; linked: boolean; shared: boolean;
}
```

```ts
// client.ts — additive. EvidenceInput is phase 3's (task-18.md:139); one optional field.
//   export interface EvidenceInput { …; proposalId?: string }
//   recordEvidence answers { record: EvidenceRecord; linked: boolean } when proposalId is set.
listEvidenceProposals(projectId: ProjectId, q: { issueId: string } | { briefKey: string }): Promise<ProposalView[]>;
```

#### Interfaces — complete: `desktop/src/components/issue/ProposedEvidence.tsx`

```tsx
// The status is the control plane's (proposals.go), never computed here. The because is an
// agent's words and always carries the text·A mark. No terminal, PTY or transcript import (OD-8).
import { useCallback, useEffect, useState, type JSX } from "react";
import { api } from "../../api";
import type { ProposalView } from "../../api/work";
import { AuthorClass } from "./AuthorClass";

export type ProposalQuery = { issueId: string } | { briefKey: string };
const short = (sha: string): string => sha.slice(0, 7);
const shortId = (id: string): string => `${id.slice(0, 6)}…`; // c-40aa17c3 → "c-40aa…"

/** One read per surface, again whenever `stamp` changes (the replica's cursor for the issue, or
 *  the Merge lens's poller tick). A failed read keeps the last answer and sets `down`. */
export function useProposals(projectId: string, q: ProposalQuery, stamp: unknown):
  { views: ProposalView[]; down: boolean; reload(): void } {
  const [views, setViews] = useState<ProposalView[]>([]);
  const [down, setDown] = useState(false);
  const key = "issueId" in q ? `i:${q.issueId}` : `b:${q.briefKey}`;
  const reload = useCallback(() => {
    api.listEvidenceProposals(projectId, q).then((v) => { setViews(v); setDown(false); }, () => setDown(true));
  }, [projectId, key]); // eslint-disable-line react-hooks/exhaustive-deps -- q is keyed by `key`
  useEffect(reload, [reload, stamp]);
  return { views, down, reload };
}

/** Under its own criterion and nowhere else; an erased proposal is not shown. */
export function proposalsFor(views: ProposalView[], criterionId: string): ProposalView[] {
  return views.filter((v) => v.proposal.proposal.criterionId === criterionId && v.proposal.erasedAt === null);
}

function StatusPart(props: { v: ProposalView; onRecord(): void; onLinkAgain(): void }): JSX.Element {
  const { v } = props;
  const ev = <span className="iv-fact" data-evidence={v.evidenceId}>{v.evidenceId}</span>;
  switch (v.status) {
    case "open": return <button type="button" className="pe-record" onClick={props.onRecord}>Record</button>;
    case "observed": return <span className="pe-status">observed · {ev}</span>;
    case "criterion_gone": return <span className="pe-status">this criterion is no longer in the brief</span>;
    case "recorded":
      if (!v.shared) return <span className="pe-status">recorded on this machine only · {ev}</span>;
      if (v.linked) return <span className="pe-status">recorded as {ev}</span>;
      return (
        <span className="pe-status">recorded as {ev} · not yet linked on the Hub{" "}
          <button type="button" className="iv-link" onClick={props.onLinkAgain}>Link again</button></span>
      );
  }
}

export function ProposedEvidence(props: {
  views: ProposalView[]; criterionId: string; onRecord(v: ProposalView): void; onLinkAgain(v: ProposalView): void;
}): JSX.Element | null {
  const rows = proposalsFor(props.views, props.criterionId);
  if (rows.length === 0) return null;
  return (
    <ul className="pe" aria-label={`Proposed evidence for ${props.criterionId}`}>
      {rows.map((v) => {
        const p = v.proposal.proposal, a = v.proposal.attribution;
        return (
          <li key={v.proposal.id} className="pe-row" data-status={v.status} data-proposal={v.proposal.id}>
            <span className="pe-lead">proposed evidence</span>{" "}
            <span className="iv-fact">{shortId(p.criterionId)}</span> at{" "}
            <span className="iv-fact">{short(p.commit)}</span> ·{" "}
            <span className="iv-fact">{p.artefact.kind} {p.artefact.ref}</span> ·{" "}
            <q className="pe-because"><AuthorClass author="A" />{p.because}</q>{" "}
            <span className="iv-fact pe-by">{a.agentId} · {a.model}</span>{" "}
            <StatusPart v={v} onRecord={() => props.onRecord(v)} onLinkAgain={() => props.onLinkAgain(v)} />
          </li>
        );
      })}
    </ul>
  );
}
```

The row is 05 §2.1's sketch, `▸ proposed evidence c-40aa… at 7c1f0a2 · "…" [Record]`, with the
artefact and the agent's byline added as facts. *Link again* re-sends the record body with
`proposalId`: the control plane answers `already_settled` with the same id and then links, so a
retry never writes a second record. Every id, sha, model and artefact is Commit Mono at
`--t-fact` (`.iv-fact`, task 46); the `.pe*` rules are appended to `issue.css` and add no motion.

#### Interfaces — complete: the edits to phase 3's `EvidenceDialog.tsx` and `LaneCard.tsx`

```tsx
// EvidenceDialog.tsx — the props become a union; phase 3's own call sites are unchanged.
export interface ProposalSeed { id: string; commit: string; because: string }
type Base = { briefKey: string; criterion: { id: string; digest: string; text: string };
  onClose(): void; onRecorded(): void; projectId?: string };
export type EvidenceDialogProps =
  | (Base & { conv: Convergence; proposal?: undefined })   // phase 3's, as task 20 wrote it
  | (Base & { conv?: Convergence; proposal: ProposalSeed; projectId: string });
export function EvidenceDialog(props: EvidenceDialogProps): JSX.Element;

// With `proposal` set, three things differ and nothing else does:
//  1. the commit is proposal.commit, shown in mono as "at 7c1f0a2, the commit the agent named";
//     phase 3's head/worktree chooser is not rendered;
//  2. Because starts as proposal.because with <AuthorClass author="A"/> beside the field; the
//     first edit removes the mark (`edited` state) and the field is never re-filled;
//  3. Record sends api.recordEvidence(projectId, {…phase 3's input, commit: proposal.commit,
//     proposalId: proposal.id}); a 409 already_settled calls onRecorded() (the row re-reads and
//     reads recorded), and proposal_mismatch shows the control plane's message verbatim.
// The Record button's rule is phase 3's, unchanged: disabled until the trimmed because has
// 1-2000 characters (task-20.md:118, :143).
```

```tsx
// LaneCard.tsx — where phase 3 task 19's DoDSlot mounts DefinitionOfDone (read on the sync base;
// UNVERIFIED until merge). One read per card, re-read on the poller's good tick (task 18's
// onGoodTick subscription), and one dialog at a time.
const { views, reload } = useProposals(project.id, { briefKey }, tick);
const [seed, setSeed] = useState<{ v: ProposalView; c: { id: string; digest: string; text: string } } | null>(null);
<DefinitionOfDone ref={ref}
  onRecordEvidence={phase3OnRecordEvidence}                       // S4-P3.3, unchanged
  criterionFooter={(id) => (
    <ProposedEvidence views={views} criterionId={id}
      onRecord={(v) => setSeed({ v, c: criterionOf(ref, id) })}
      onLinkAgain={(v) => void api.recordEvidence(project.id, relinkBody(v, criterionOf(ref, id))).finally(reload)} />
  )} />
{seed && <EvidenceDialog projectId={project.id} briefKey={briefKey} criterion={seed.c}
  proposal={{ id: seed.v.proposal.id, commit: seed.v.proposal.proposal.commit, because: seed.v.proposal.proposal.because }}
  onClose={() => setSeed(null)} onRecorded={() => { setSeed(null); reload(); }} />}
```

`criterionOf(ref, id)` reads the criterion's `{id, digest, text}` from the `Ref` the card already
holds (phase 4's `RefCriterion`); `relinkBody` is the proposal's criterion, commit and the
because as last sent, with `proposalId`. Neither is new state.

#### Interfaces — complete: phase 4's `DefinitionOfDone.tsx` and task 47's `Serves.tsx`

```tsx
// DefinitionOfDone.tsx — one added optional prop (an ask to phase 4 beside S4-P3.3). It renders
// under each criterion's row, after the state word, inside that criterion's <li>. Absent → the
// component renders byte for byte as phase 4 task 17 left it (its snapshot test holds that).
criterionFooter?: (criterionId: string) => ReactNode;
```

```tsx
// Serves.tsx — ServeRow gains two props from Serves, which reads once per issue:
//   const { views, reload } = useProposals(localProjectId, { issueId }, cursor);
// `cursor` is the replica's per-issue event cursor (SW.16; its field is read on the sync base,
// gap 3) and `localProjectId` the local project bound to the issue's Hub project (task 24's
// binding). With no bound project the rows show and Record reads "clone this project on this
// machine to record evidence" as a fact, not a disabled button with no reason.
<ProposedEvidence views={views} criterionId={serve.criterionId}
  onRecord={(v) => openDialog(v, c)} onLinkAgain={(v) => relink(v, c).finally(reload)} />
```

The issue view opens the same `EvidenceDialog` with `proposal` set and no `conv`: the dialog is
phase 3's one Record, reached from two places, and never a second form.

#### Tests to write first — `desktop/src/components/issue/ProposedEvidence.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { ProposalView } from "../../api/work";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { BRIEF_KEY, WORK_IDS } from "../../test/workFixtures";
import { ProposedEvidence } from "./ProposedEvidence";
import src from "./ProposedEvidence.tsx?raw";

const TIP = "7c1f0a2e5b9d4c3a2f1e0d9c8b7a6f5e4d3c2b1a";
const view = (over: Partial<ProposalView> = {}, criterionId: string = WORK_IDS.criterionUpload): ProposalView => ({
  status: "open", linked: false, shared: true,
  proposal: { id: "ic-00000000000000p1", issueId: WORK_IDS.ep142, kind: "evidence_proposal", recordedAs: null,
    createdAt: "2026-09-25T01:00:00Z", erasedAt: null,
    attribution: { ownerId: WORK_IDS.dylan, agentId: "ag-0000000000000002", model: "model-x", sessionId: "se-9" },
    proposal: { briefKey: BRIEF_KEY, criterionId, issueId: WORK_IDS.ep142, commit: TIP,
      artefact: { kind: "test_run", ref: "pytest -q tests/upload" }, because: "upload tests pass at this commit" } },
  ...over,
});
const noop = (): void => undefined;

describe("ProposedEvidence", () => {
  it("shows a proposal under its own criterion and nowhere else", () => {
    const views = [view(), view({}, WORK_IDS.criterionResume)];
    const { container } = render(<ProposedEvidence views={views} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expect(container.querySelectorAll(".pe-row")).toHaveLength(1);
    expect(container.textContent).toContain("c-40aa…");
    expect(container.textContent).not.toContain("c-91e2");
  });
  it("marks the because as an agent's words and names the agent and model", () => {
    const { container } = render(<ProposedEvidence views={[view()]} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expect(container.querySelector(".pe-because [data-author='A']")).not.toBeNull();
    expect(container.textContent).toContain("ag-0000000000000002 · model-x");
    expect(container.textContent).toContain("7c1f0a2");
  });
  it("offers Record only while open, and hands the proposal to the caller", () => {
    const onRecord = vi.fn();
    render(<ProposedEvidence views={[view()]} criterionId={WORK_IDS.criterionUpload} onRecord={onRecord} onLinkAgain={noop} />);
    fireEvent.click(screen.getByRole("button", { name: "Record" }));
    expect(onRecord.mock.calls[0][0].proposal.id).toBe("ic-00000000000000p1");
  });
  it("after Record names the evidence and offers no second Record", () => {
    const v = view({ status: "recorded", evidenceId: "ev-00000000000a", linked: true });
    const { container } = render(<ProposedEvidence views={[v]} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expect(screen.queryByRole("button", { name: "Record" })).toBeNull();
    expect(container.querySelector("[data-evidence='ev-00000000000a']")).not.toBeNull();
  });
  it("reads observed evidence as observed, with no Record", () => {
    render(<ProposedEvidence views={[view({ status: "observed", evidenceId: "ev-obs" })]} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expect(screen.getByText(/observed/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Record" })).toBeNull();
  });
  it("an unlinked record offers Link again, never Record", () => {
    render(<ProposedEvidence views={[view({ status: "recorded", evidenceId: "ev-1", linked: false })]} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expect(screen.getByRole("button", { name: "Link again" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Record" })).toBeNull();
  });
  it("predicts nothing, and reads nothing from a terminal", () => {
    const { container } = render(<ProposedEvidence views={[view(), view({ status: "observed", evidenceId: "ev-2" })]} criterionId={WORK_IDS.criterionUpload} onRecord={noop} onLinkAgain={noop} />);
    expectNoPrediction(container);
    expect(src).not.toMatch(/attachRegistry|xterm|Terminal|transcript|setInterval|setTimeout/);
  });
});
```

| Test | Mutant it kills |
|---|---|
| under its own criterion | "grouped by issue": every proposal on the issue drawn under each criterion |
| the because as an agent's words | agent text unlabelled (§2.1 point 3); model or agent dropped (L-173) |
| Record only while open | Record wired to nothing, or to a different proposal |
| no second Record | "double record" |
| observed, with no Record | "observed evidence needs a click" (§2.1 point 4) |
| Link again, never Record | a link failure that re-offers Record |
| predicts nothing | an "N of M criteria proposed" line (`OD-27`); PTY-derived meaning (`OD-8`) |

#### Tests to write first — `desktop/src/components/lens/merge/EvidenceDialog.proposal.test.tsx`

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { api } from "../../../api";
import { DefinitionOfDone } from "../../../provenance/DefinitionOfDone";
import { aRef } from "../../../provenance/testRefs";        // phase 4 task 17's fixture; name read on the sync base
import { expectNoPrediction } from "../../../test/expectNoPrediction";
import { BRIEF_KEY, WORK_IDS } from "../../../test/workFixtures";
import { EvidenceDialog } from "./EvidenceDialog";

const TIP = "7c1f0a2e5b9d4c3a2f1e0d9c8b7a6f5e4d3c2b1a";
const crit = { id: WORK_IDS.criterionUpload, digest: "sha256:40", text: "an upload survives a restart" };
const seed = { id: "ic-00000000000000p1", commit: TIP, because: "upload tests pass at this commit" };
const open = (onRecorded = vi.fn()) => render(<EvidenceDialog projectId="p1" briefKey={BRIEF_KEY}
  criterion={crit} proposal={seed} onClose={vi.fn()} onRecorded={onRecorded} />);
beforeEach(() => vi.restoreAllMocks());

describe("EvidenceDialog with a proposal", () => {
  it("records as the person at the proposal's commit, carrying proposalId and no agent field", async () => {
    const rec = vi.spyOn(api, "recordEvidence").mockResolvedValue({ record: { id: "ev-1" }, linked: true } as never);
    open();
    fireEvent.click(screen.getByRole("button", { name: "Record" }));
    await waitFor(() => expect(rec).toHaveBeenCalledOnce());
    const body = rec.mock.calls[0][1] as unknown as Record<string, unknown>;
    expect(body).toMatchObject({ commit: TIP, proposalId: seed.id, criterionId: crit.id });
    expect(Object.keys(body)).not.toEqual(expect.arrayContaining(["agentId"]));
    expect(JSON.stringify(body)).not.toMatch(/ag-|sessionId|onBehalfOf|via/);
  });
  it("refuses an empty because", () => {
    open();
    fireEvent.change(screen.getByLabelText("Because"), { target: { value: "   " } });
    expect((screen.getByRole("button", { name: "Record" }) as HTMLButtonElement).disabled).toBe(true);
  });
  it("marks the prefilled because as the agent's until the person edits it", () => {
    const { container } = open();
    expect(container.querySelector("[data-author='A']")).not.toBeNull();
    fireEvent.change(screen.getByLabelText("Because"), { target: { value: "I ran it myself" } });
    expect(container.querySelector("[data-author='A']")).toBeNull();
  });
  it("offers no commit chooser: the commit is the one the agent named", () => {
    open();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.getByText(/7c1f0a2/)).toBeTruthy();
  });
  it("leaves the criterion's word unchanged while a proposal waits", () => {
    const ref = aRef({ criteria: [crit], criterionStates: [{ criterionId: crit.id, state: "open" }] });
    const { container } = render(<DefinitionOfDone ref={ref} criterionFooter={() => <span>proposed evidence</span>} />);
    expect(container.querySelector(`[data-criterion='${crit.id}'] [data-state]`)?.getAttribute("data-state")).toBe("open");
    expectNoPrediction(container);
  });
});
```

| Test | Mutant it kills |
|---|---|
| records as the person | "recorded as the agent": the agent's attribution in the body, or Record sent through a delegated path |
| refuses an empty because | "reason optional" (phase 3's `3-D5`) |
| marks the prefilled because | an agent's words shown as the person's before the person has touched them |
| no commit chooser | a proposal recorded at the branch head the agent never named |
| the criterion's word unchanged | "a proposal settles" (§2.1 point 4) |

#### Steps

- [ ] **Step 1: read on the sync base** phase 3's `internal/evidence` (task 15's `Store`, task
  23's `Observed`, `IssueID`), its evidence route file and `EvidenceDialog.tsx`/`LaneCard.tsx`/
  `DoDSlot`, phase 4's `DefinitionOfDone.tsx` and its test fixtures, task 99's `hubapi` proposal
  names and task 100's link route. Rename in this part's files only; a missing seam waits (H2).
- [ ] **Step 2: the Go tests**; `go test ./control-plane/internal/evidence/... -race -count=1`
  fails on the missing symbols.
- [ ] **Step 3: `proposals.go`**, then the route, its three tests and the catalog row.
- [ ] **Step 4: the desktop tests**; `npm --prefix desktop test -- ProposedEvidence EvidenceDialog`
  fails on the missing module and props.
- [ ] **Step 5: `work.ts`/`client.ts` appends, `ProposedEvidence.tsx`, the `EvidenceDialog`,
  `DefinitionOfDone`, `LaneCard` and `Serves` edits**, and the `.pe*` rules in `issue.css`.
- [ ] **Step 6: the acceptance commands.** Commit by path after steps 3 and 5, each message ending
  with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/evidence/... ./control-plane/internal/api/... -race -count=1
# expect: ok for both; proposals_test.go runs 9 tests, evidence_test.go gains 3 rows
npm --prefix desktop test -- ProposedEvidence EvidenceDialog DefinitionOfDone issue/Serves
# expect: every file passes; this part adds 2 files and 12 tests (ProposedEvidence 7, dialog 5);
#   phase 3's EvidenceDialog.test.tsx and phase 4's DefinitionOfDone tests stay green unchanged
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "attachRegistry|xterm|transcript" desktop/src/components/issue/ProposedEvidence.tsx | wc -l
# expect: 0
```

Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- That people press *Record* with judgement rather than by habit (05 §2.1 *Unproven*); AX §7's
  calibration record is the later check. Recording the prefilled because unchanged is allowed.
- That an agent's proposal is correct, or that its artefact still exists: the row shows the
  agent's claim as `text·A`, and the person decides.
- Browser testing inside the sandbox as a kind of observed evidence: it stays *later*, and
  **UNVERIFIED** on Kata.
- That phase 3's dialog, lane card and evidence routes, or phase 4's `DefinitionOfDone`, have the
  paths and names used here: all are **UNVERIFIED until merge** (rule H2), and step 1 re-reads them.
- That the Hub's link route exists or refuses a link to another person's record: task 99's method
  list has no link method (gap 2).
- Recording from the cloud: with no local project bound to the issue's Hub project, the row says so
  and records nothing (the team-mode path through Fleet is not this task's).

**Parity.** Closes the W3 half of **L-166** (with task 101's `panacea evidence propose`).

#### Gaps found while writing this part

1. **No W3 desktop wire task.** The file map gives `desktop/src/api/work.ts` to tasks 37, 83, 125
   and 169 (W1, W2, W4, W5) and none to W3, so SW.19's desktop types have no owner; this part
   appends the proposal types, and tasks 102, 103, 106 and 107 need the rest. The skeleton should
   add a W3 desktop wire row (or give it to task 100).
2. **Task 99's method list has no link from a proposal to its record.** `recordedAs` needs
   `LinkEvidenceProposal(proposalId, evidenceId)`, checked on the Hub (the evidence exists, same
   criterion and commit, recorded by the caller); tasks 99 and 100 should add it, with a matrix row.
3. **SW.16 names no per-issue event cursor**, which a surface needs to re-read data the replica
   does not hold (comments, proposals). Tasks 55-58 should expose one; until then the issue view
   re-reads on mount only.
4. **Phase 4's `DefinitionOfDone` has no per-criterion slot.** `criterionFooter?` is an ask to
   phase 4 beside S4-P3.3; until it lands the fallback of phase 3 task 20 (`task-20.md:112-115`)
   is used.
5. **The skeleton's delta row for the Merge lens (`:284`) says "paths UNVERIFIED"**, but phase 3's
   plan does name them (`:558-559`, `components/lens/merge/*`); the row can cite them.
6. **The local-to-Hub project binding** this part reads (task 24's) has no named desktop selector;
   task 37 or 61 should export one.
