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
		if r.Commit == in.Commit && r.CriterionID == p.CriterionID {
			return r, in.Proposal.RecordedAs == r.ID, ErrAlreadySettled
		}
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
