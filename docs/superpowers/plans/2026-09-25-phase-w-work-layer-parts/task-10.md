### Task 10: States, thread state and the done rule on the server

Stream **WH**, seam **SW.4** (defined here: the state move, the done rule's inputs and its result
words), about 3 hours (an estimate, not a measurement). Depends on task 09 (revisions and `serves`)
and Hub task **H25** (evidence, hub-contract §7.10), sync-merged into `feat/work-layer` first (rule
H1). Task 04's fake conforms to SW.4; tasks 22 and 23 are its callers; tasks 48, 63-65 and 71 read
its result words.

This is the task that makes *done means proven* true on the server. The skeleton's rule 2 says
`started`, `in_review`, `landed` and `done` are **facts**: each is written as the person whose act
caused it, for one cause, and `done` needs the issue's **own** evidence at its **own** land commit
for **every** criterion it serves. Task 07 put the cause check in the database; this task puts the
evidence check in Go, records what was cited, and names the result in one word the desktop can show.

**Eight decisions.**

1. **One route, one body, six causes.** `POST /v1/issues/{issueId}/state` takes `StateMove{to,
   cause, reason, landCommit, evidence, evidenceCommit}`. The causes are complete for W1: `hand`
   (a person's move into `triage`, `backlog`, `todo`, `cancelled` or `duplicate`, a reopen, or a
   hand-set `done`); `lane_bound` → `started`; `lane_queued` → `in_review`; `land_trailer` →
   `landed`, carrying `landCommit` (task 22); `evidence_observed` and `evidence_recorded` → `done`,
   carrying `evidence` ids (task 23). The pure table `hubapi.MoveAllowed(from, to, cause)` decides
   legality, and both the Hub and the fake (task 04) call it, so they cannot disagree.
2. **The store sets task 07's two settings before the `UPDATE`.** `hub.work_cause` = the move's
   cause and `hub.work_actor_kind` = `person`, both with `set_config(…, true)` after the Hub's
   `hub.org_id`, in the same transaction. This is the **only** Go that sets `hub.work_cause`
   (task 07's requirement). A refusal from the trigger is a `42501` with the prefix `hub: work:`,
   and the store turns it into 422 `state_not_allowed`; with the Go rule right it is never reached.
3. **The evidence rule** (05 §3.3, revision 2). An issue becomes `done` by evidence only when it is
   `landed` with a recorded land commit and, for **every** `(briefKey, criterionId)` its head
   revision serves, there is a cited evidence record that: is in the **same project**; names that
   brief id **and** revision **and** that criterion id; has `verdict: settled`; was recorded **at or
   after** `landed_at`; and has `commit` equal to the land commit, or to the move's
   `evidenceCommit`. The Hub cannot test git ancestry, so a later commit is accepted only when the
   move names it; **the ancestry check is task 23's claim, not the Hub's**.
4. **The citation is the move's `evidence` list, recorded.** Hub evidence carries no issue id
   (`EvidenceRecord`, `types.go:520-531`), so this task records each cited record in
   `issue_proof_citations`. A record already cited, uncleared, by **another** issue is refused
   unless both issues landed at the same commit (two issues fixed by one commit may share one test
   run). Each record in `evidence` is checked on its own; one that fails any clause of decision 3 is
   422 `evidence_not_applicable`, `details.field = "evidence[i]"` and `details.why` one of
   `project`, `criterion`, `verdict`, `before_landed`, `commit`, `cited_elsewhere`.
5. **Evidence accumulates; `done` is reached when the last criterion is covered.** An evidence move
   whose records are all valid is recorded even when it does not cover every criterion: the issue
   stays `landed` with `proof: waiting` and the answer is 200. The next move adds the rest. So a
   person can record two criteria's evidence in two presses and the second press closes the issue.
6. **Six result words, on `IssueHead.proof`.** `proven` (every cited record `evidence_observed`);
   `proven_self_attested` (some record `evidence_recorded` by the person who created the issue
   **and** made its `started` move; observed evidence is never self-attested); `not_proven` (a
   hand-set `done` on an issue that serves criteria; it needs a non-empty `reason`, 400 naming
   `reason` without one); `no_criteria` (empty `serves`: the `landed` move lands at `done` in the
   same write, unless the team's `stopAtLandedWithoutServes` holds it at `landed`; a hand-set `done`
   with no criteria reads it too); `waiting` (landed, some criterion without evidence); `""` before
   `landed`. A reopen, or a move to `cancelled` or `duplicate`, clears the word, the land commit and
   every citation (marked `cleared_at`, never deleted).
7. **Idempotent facts, refused contradictions.** `lane_bound` on a `started` issue, `lane_queued`
   on an `in_review` issue, and `land_trailer` with the **same** commit on a `landed` issue are 200
   with no write and no event (task 22's hook may fire twice). `land_trailer` with a **different**
   commit on a `landed` or `done` issue is 409 `state_conflict`: a second landing needs a reopen
   first, by hand, so the first proof is never silently replaced.
8. **Thread state is task 08's.** The scope lists it here; task 08's create needed it first, so
   `threadStateOf` is declared there and this task only adds `proof` and `landCommit` beside it in
   `issueHead`.

**Files:**
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` — `issue_proofs`,
  `issue_proof_citations`, their row-level security and grants, and `guard_issue_state()` replaced
  with one added clause, **appended** (or the next number if task 06 has merged, rule H1) (modify)
- `control-plane/internal/hub/hubapi/work.go` ⚑ — the causes, the proof words, `MoveAllowed`,
  `DecideProof`, the `StateMove` and `IssueHead` additions, three codes (modify, additive)
- `control-plane/internal/hub/hubapi/workstates_test.go` — the pure tables (new)
- `control-plane/internal/hub/store/work_states.go` — the settings, the move, the proof record,
  the citations, the evidence read (new)
- `control-plane/internal/hub/server/work_states.go` — the route and the rule (new)
- `control-plane/internal/hub/server/work_states_integration_test.go` — the tests below (new)
- `control-plane/internal/hub/server/work_issues.go` — `issueHead` fills `proof` and `landCommit`
  (task 08's file; modify, two lines)
- `control-plane/internal/hub/migrate/work_w1_integration_test.go` — `TestWorkGrantsAreExactlyTheList`
  gains the two tables (task 06's file; modify)
- `server/server.go` ⚑ — `s.registerWorkStates()`; `work_matrix_rows_integration_test.go` — `"10"`

**Interfaces:**
- Produces (SW.4):
  - `hubapi.StateCause` and its six constants; `hubapi.ProofWord` and its six constants
  - `hubapi.MoveAllowed(from, to IssueState, cause StateCause) error` and
    `hubapi.DecideProof(in ProofInput) ProofWord`, pure, called by the Hub and the fake
  - `StateMove.LandCommit`, `StateMove.Evidence`, `StateMove.EvidenceCommit`;
    `IssueHead.Proof`, `IssueHead.LandCommit`
  - the codes `state_not_allowed` (422), `state_conflict` (409), `evidence_not_applicable` (422)
  - the route `POST /v1/issues/{issueId}/state` → 200 `IssueHead`
  - the event `issue.state` `{from, to, cause, proof, landCommit}` (task 13's name, SW.5)
- Consumes:
  - task 07: `hub.work_cause`, `hub.work_actor_kind`, `guard_issue_state()`, the `42501` prefix
  - task 08: `readableIssue`, `issueHead`, `store.LockIssue`; task 09: the head revision's serves
  - task 06: `team_work_policies.stop_at_landed_without_serves`
  - H25: the `evidence` table (`0001_init.sql`, H07a `:384-402`): `project_id`, `brief_id`,
    `brief_revision`, `criterion_id`, `commit_sha`, `verdict`, `recorded_at`, `recorded_by`
- Requires of later tasks:
  - **Task 22** sends `lane_bound`, `lane_queued` and `land_trailer` (with `landCommit`) as the
    person whose lane it is, never on another person's behalf.
  - **Task 23** sends `evidence_observed` only for a run the platform watched, and names
    `evidenceCommit` only after checking it descends from the land commit.
  - **Task 11** reads its team policy through `store.TeamPolicy`; when it lands, this task's
    one-column read `store.StopAtLanded` is replaced by it (a find-and-replace, one call).
  - **Task 04's fake** calls `MoveAllowed` and `DecideProof`, and records citations the same way.

#### The migration, appended (complete)

```sql
-- NNNN_work_w1.sql, continued (task 10): the proof record.
CREATE TABLE issue_proofs (
  org_id      text        NOT NULL REFERENCES orgs (id),
  issue_id    text        NOT NULL PRIMARY KEY,
  started_by  text,
  started_at  timestamptz,
  land_commit text        CHECK (land_commit ~ '^[0-9a-f]{40}$'),
  landed_at   timestamptz,
  landed_by   text,
  proof       text        NOT NULL DEFAULT '',
  decided_at  timestamptz,
  CONSTRAINT issue_proofs_proof_known CHECK (proof IN ('', 'proven', 'proven_self_attested',
                                                       'not_proven', 'no_criteria', 'waiting')),
  CONSTRAINT issue_proofs_landed_together
    CHECK ((land_commit IS NULL) = (landed_at IS NULL) AND (landed_at IS NULL) = (landed_by IS NULL)),
  FOREIGN KEY (org_id, issue_id)   REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, started_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, landed_by)  REFERENCES users (org_id, id)
);

-- One row per evidence record an issue cited at one landing. Never deleted: a reopen
-- sets cleared_at. evidence_id has no foreign key: evidence carries no UNIQUE (org_id, id)
-- target in 0001 (H07a :384-402), and a single-column key is what review I5 forbids.
CREATE TABLE issue_proof_citations (
  org_id      text        NOT NULL REFERENCES orgs (id),
  issue_id    text        NOT NULL,
  evidence_id text        NOT NULL CHECK (evidence_id ~ '^ev-[0-9a-f]{16}$'),
  land_commit text        NOT NULL CHECK (land_commit ~ '^[0-9a-f]{40}$'),
  cause       text        NOT NULL CHECK (cause IN ('evidence_observed', 'evidence_recorded')),
  cited_at    timestamptz NOT NULL DEFAULT now(),
  cited_by    text        NOT NULL,
  cleared_at  timestamptz,
  PRIMARY KEY (issue_id, evidence_id, land_commit),
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, cited_by) REFERENCES users (org_id, id)
);
CREATE INDEX issue_proof_citations_live ON issue_proof_citations (org_id, evidence_id)
  WHERE cleared_at IS NULL;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['issue_proofs', 'issue_proof_citations'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

GRANT SELECT, INSERT ON issue_proofs, issue_proof_citations TO hub_app;
GRANT UPDATE (started_by, started_at, land_commit, landed_at, landed_by, proof, decided_at)
                                   ON issue_proofs          TO hub_app;
GRANT UPDATE (cleared_at)          ON issue_proof_citations TO hub_app;
-- No DELETE on either: a reopen clears, it never forgets.

-- guard_issue_state, replaced: task 07's function with one clause added. Evidence
-- closes only an issue that has landed; it never skips the landing.
CREATE OR REPLACE FUNCTION guard_issue_state() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE
  v_cause  text := coalesce(current_setting('hub.work_cause', true), '');
  v_kind   text := coalesce(current_setting('hub.work_actor_kind', true), '');
  v_serves boolean := EXISTS (SELECT 1 FROM issue_serves s
                               WHERE s.issue_id = NEW.id AND s.revision = NEW.head_revision);
  v_fact   boolean := NEW.state IN ('started', 'in_review', 'landed', 'done');
BEGIN
  IF v_fact AND v_kind <> 'person' THEN
    RAISE EXCEPTION 'hub: work: only a person moves % into %', NEW.id, NEW.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF v_cause IN ('evidence_observed', 'evidence_recorded') AND OLD.state <> 'landed' THEN
    RAISE EXCEPTION 'hub: work: evidence closes a landed issue, and % is %', NEW.id, OLD.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF NOT CASE NEW.state
       WHEN 'started'   THEN v_cause = 'lane_bound'
       WHEN 'in_review' THEN v_cause = 'lane_queued'
       WHEN 'landed'    THEN v_cause = 'land_trailer'
       WHEN 'done'      THEN v_cause IN ('evidence_observed', 'evidence_recorded')
                          OR (v_cause = 'land_trailer' AND NOT v_serves)
                          OR (v_cause = 'hand' AND (NOT v_serves OR btrim(NEW.state_reason) <> ''))
       ELSE v_cause = 'hand'
     END
  THEN
    RAISE EXCEPTION 'hub: work: % cannot move into % with cause "%"', NEW.id, NEW.state, v_cause
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
```

`CREATE OR REPLACE` keeps task 07's trigger bound to the new body. Task 07's guard tests still pass
against it unchanged (the added clause only refuses what task 07 allowed from a non-`landed`
issue by evidence, which no task 07 test does); one test below pins the new clause.

#### `hubapi/work.go`, the additions (complete)

```go
// ── Task 10: the state move and the done rule (SW.4) ─────────────────────────

type StateCause string

const (
	CauseHand             StateCause = "hand"
	CauseLaneBound        StateCause = "lane_bound"
	CauseLaneQueued       StateCause = "lane_queued"
	CauseLandTrailer      StateCause = "land_trailer"
	CauseEvidenceObserved StateCause = "evidence_observed"
	CauseEvidenceRecorded StateCause = "evidence_recorded"
)

type ProofWord string

const (
	ProofNone         ProofWord = ""
	ProofProven       ProofWord = "proven"
	ProofSelfAttested ProofWord = "proven_self_attested"
	ProofNotProven    ProofWord = "not_proven"
	ProofNoCriteria   ProofWord = "no_criteria"
	ProofWaiting      ProofWord = "waiting"
)

// Additive fields on task 03's structs:
//   StateMove: LandCommit     string   `json:"landCommit,omitempty"`     // 40 hex; land_trailer only
//              Evidence       []string `json:"evidence,omitempty"`       // ev- ids; evidence causes only
//              EvidenceCommit string   `json:"evidenceCommit,omitempty"` // a descendant of the land
//                                                                       // commit, checked by task 23
//   IssueHead: Proof      ProofWord `json:"proof"`      // "" before landed
//              LandCommit string    `json:"landCommit"` // "" before landed

const (
	CodeStateNotAllowed       = "state_not_allowed"       // 422, details.from, details.to, details.cause
	CodeStateConflict         = "state_conflict"          // 409, details.landCommit (the recorded one)
	CodeEvidenceNotApplicable = "evidence_not_applicable" // 422, details.field "evidence[i]", details.why
)

const EventIssueState = "issue.state" // {from, to, cause, proof, landCommit}
```

The two pure functions, whole. They live in `hubapi` so the fake (task 04) and the Hub run one
table and one decision:

```go
// MoveError is MoveAllowed's refusal; the route turns it into 422 state_not_allowed.
type MoveError struct {
	From, To IssueState
	Cause    StateCause
}

func (e *MoveError) Error() string {
	return fmt.Sprintf("hubapi: %s cannot move %s → %s", e.Cause, e.From, e.To)
}

// MoveAllowed is the legality table of decision 1. It knows nothing of evidence
// or policy. The target is the move's "to"; a land_trailer that lands at done
// (no criteria) is still asked as → landed, and the Hub chooses done after.
func MoveAllowed(from, to IssueState, cause StateCause) error {
	in := func(s IssueState, set ...IssueState) bool { return slices.Contains(set, s) }
	ok := false
	switch cause {
	case CauseHand:
		ok = in(to, StateTriage, StateBacklog, StateTodo, StateCancelled, StateDuplicate, StateDone)
	case CauseLaneBound:
		ok = to == StateStarted && in(from, StateTriage, StateBacklog, StateTodo, StateStarted)
	case CauseLaneQueued:
		ok = to == StateInReview && in(from, StateTriage, StateBacklog, StateTodo, StateStarted, StateInReview)
	case CauseLandTrailer:
		ok = to == StateLanded && in(from, StateTriage, StateBacklog, StateTodo, StateStarted,
			StateInReview, StateLanded, StateDone)                   // same-commit repeats: decision 7
	case CauseEvidenceObserved, CauseEvidenceRecorded:
		ok = to == StateDone && from == StateLanded
	}
	if !ok {
		return &MoveError{From: from, To: to, Cause: cause}
	}
	return nil
}

// Cited is one validated, uncleared citation at the current landing.
type Cited struct {
	BriefKey    string     // brief:bf-…@N the evidence names
	CriterionID string
	Cause       StateCause // CauseEvidenceObserved or CauseEvidenceRecorded
	RecordedBy  string     // the evidence record's recorder, u-…
}

type ProofInput struct {
	Serves    []Serve // the head revision's
	Cited     []Cited
	CreatedBy string  // the issue's creator
	StartedBy string  // who made the started move; "" if none was recorded
	HandSet   bool    // a hand-set done
}

// DecideProof is decision 6. Per criterion the strongest citation counts:
// observed, then recorded by someone other than the creator-and-starter, then
// self-recorded. The word is self-attested when any criterion's strongest is
// self-recorded, and waiting when any criterion has none.
func DecideProof(in ProofInput) ProofWord {
	if len(in.Serves) == 0 {
		return ProofNoCriteria
	}
	if in.HandSet {
		return ProofNotProven
	}
	self := false
	for _, s := range in.Serves {
		best := 0 // 0 none, 1 self-recorded, 2 recorded by another, 3 observed
		for _, c := range in.Cited {
			if c.BriefKey != s.BriefKey || c.CriterionID != s.CriterionID {
				continue
			}
			rank := 3
			if c.Cause == CauseEvidenceRecorded {
				rank = 2
				if c.RecordedBy == in.CreatedBy && c.RecordedBy == in.StartedBy {
					rank = 1
				}
			}
			best = max(best, rank)
		}
		if best == 0 {
			return ProofWaiting
		}
		if best == 1 {
			self = true
		}
	}
	if self {
		return ProofSelfAttested
	}
	return ProofProven
}
```

**A reading the scope leaves open, decided here.** The scope defines `proven` as *every record
`evidence_observed`* and `proven_self_attested` as *a record `evidence_recorded` by the creator who
also started it*. Evidence recorded by **someone else** (a reviewer pressing Record) fits neither
sentence. This task reads it as `proven`: an independent person attested it, and the word that
warns is kept for the one case the design names. The owner may want a third word; that would be an
added constant, and the desktop's glyph set (task 28) would need it too.

#### `store/work_states.go` (the contract)

```go
package store

// SetWorkCause runs, in this order, after hub.org_id is set:
//   SELECT set_config('hub.work_cause', $1, true), set_config('hub.work_actor_kind', 'person', true)
// It is the only caller of set_config for these two names in the Go tree.
func SetWorkCause(ctx context.Context, tx pgx.Tx, cause hubapi.StateCause) error

// MoveIssueState: UPDATE issues SET state = $2, state_reason = $3, updated_at = $4 WHERE id = $1.
// A 42501 whose message starts "hub: work:" is ErrStateGuard.
func MoveIssueState(ctx context.Context, tx pgx.Tx, id string, to hubapi.IssueState, reason string, at time.Time) error

var ErrStateGuard = errors.New("store: the state guard refused the move")

type ProofRow struct {
	StartedBy, LandCommit, LandedBy string
	StartedAt, LandedAt, DecidedAt  *time.Time
	Proof                           hubapi.ProofWord
}

// ProofOf reads the row; a zero ProofRow when none exists.
func ProofOf(ctx context.Context, q Querier, issueID string) (ProofRow, error)
// PutProof upserts the whole row (INSERT … ON CONFLICT (issue_id) DO UPDATE).
func PutProof(ctx context.Context, tx pgx.Tx, org, issueID string, p ProofRow) error

// EvidenceByID reads evidence rows by id under row-level security; absent ids are
// absent from the map (another org's, or none).
func EvidenceByID(ctx context.Context, q Querier, ids []string) (map[string]EvidenceRow, error)

type EvidenceRow struct {
	ID, ProjectID, BriefKey, CriterionID, CommitSHA, RecordedBy string
	Verdict                                                     hubapi.Verdict
	RecordedAt                                                  time.Time
}

// LiveCitationsElsewhere returns, for each evidence id, the uncleared citations by
// issues other than issueID, with their land commits.
func LiveCitationsElsewhere(ctx context.Context, q Querier, issueID string, evidenceIDs []string) (map[string][]string, error)
func InsertCitation(ctx context.Context, tx pgx.Tx, org, issueID, evidenceID, landCommit string, cause hubapi.StateCause, by string, at time.Time) error
func LiveCitations(ctx context.Context, q Querier, issueID, landCommit string) ([]hubapi.Cited, error)
func ClearCitations(ctx context.Context, tx pgx.Tx, issueID string, at time.Time) error

// StopAtLanded reads team_work_policies.stop_at_landed_without_serves (false when no row).
// Task 11 replaces the one call with store.TeamPolicy.
func StopAtLanded(ctx context.Context, q Querier, teamID string) (bool, error)
```

#### `server/work_states.go` (the rule, in order)

`POST /v1/issues/{issueId}/state`, under `server.Write` (an `Idempotency-Key` is honoured; the reply
carries no text, so the plain `Write` is enough):

1. `Decode` the `StateMove`; `readableIssue(…, hubapi.RoleMember, lock=true)`; the head revision's
   `serves` from `issue_serves`.
2. `hubapi.MoveAllowed(head.State, move.To, move.Cause)` → 422 `state_not_allowed`. Field checks:
   `landCommit` 40 lower-case hex, and present exactly for `land_trailer`; `evidence` 1-50 `ev-`
   ids, present exactly for the two evidence causes; `evidenceCommit` 40 hex or empty; `reason`
   ≤ 500 characters. A field on the wrong cause is 400 naming it.
3. **No-ops** (decision 7): the target equals the current state and the cause is `hand`,
   `lane_bound` or `lane_queued`; or `land_trailer` with the recorded commit. 200, the head, no
   write, no event. `land_trailer` with a different commit on `landed` or `done` → 409
   `state_conflict`.
4. **By cause:**
   - `hand` into `done`: `reason` required when `serves` is non-empty (400 naming `reason`); proof
     `DecideProof{HandSet: true}`. Into anything else: from a fact state or `cancelled`/`duplicate`
     it is a reopen or a close, so `ClearCitations` and a zeroed proof row.
   - `lane_bound`: `PutProof` with `StartedBy` = the caller, `StartedAt` = now.
   - `lane_queued`: the move only.
   - `land_trailer`: `PutProof` with the land commit, `landed_at` = now, `landed_by` = the caller.
     Empty `serves` and `StopAtLanded` false → the target becomes `done`, proof `no_criteria`; empty
     `serves` and the switch on → `landed`, proof `no_criteria`; otherwise `landed`, proof `waiting`.
   - evidence causes: `EvidenceByID`; each id checked against decision 3, in order, the first
     failure 422 `evidence_not_applicable` naming `evidence[i]` and `why`; then
     `LiveCitationsElsewhere` for decision 4's `cited_elsewhere`; then `InsertCitation` for each;
     then `DecideProof` over `LiveCitations` at this land commit. `proven` or
     `proven_self_attested` → the target `done`; `waiting` → stays `landed` (200, decision 5).
5. `SetWorkCause`, then `MoveIssueState` when the state changes; `PutProof` with the word and
   `decided_at`. `ErrStateGuard` → 422 `state_not_allowed` (never expected; logged when seen).
6. **One** `issue.state` event `{from, to, cause, proof, landCommit}` (`landCommit` `""` when
   none), also for an evidence move that stays `landed` (its proof record changed). Answer 200 with
   `issueHead`, which now fills `proof` and `landCommit` from `ProofOf`.
