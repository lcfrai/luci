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
