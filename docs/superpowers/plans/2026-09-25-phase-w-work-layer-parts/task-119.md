### Task 119: status updates `nt-`: health word, the pinned brief revision, the facts they cite

W4, stream WH; no seam. Depends on task 118 (initiatives, the second subject) and Phase T task 46
(notes and their tombstones). Hub anchors were read in the `feat/hub` worktree at `1271ddb`: the
contract `docs/superpowers/contracts/2026-09-23-hub-contract.md` (cited `hub-contract:N`) and the
built `hubapi/types.go`. Phase T anchors were read on `feat/team-mode` at `64ab2c4`: the plan
(`phase-t-team-mode.md:912-917`, cited `T:N`) and TM task 46's part
(`2026-09-25-phase-t-team-mode-parts/task-46.md`, cited `TM46:N`). Every `server/` and `store/`
path, the `notes` table and TM-46's store functions are **UNVERIFIED until merge** (rules H1, H2):
TM-46 writes that task 04's part, which owns the `notes` columns, *"is not yet written"*
(`TM46:63-66`).

A **status update** is a person's short word on how a feature or an initiative is going: one of
three health words, a few lines of their own text, the **brief revision it was written against**,
and the facts it cites. It is how a lead learns the state of work without asking, and the input
the Desk's reminders (task 121) and the initiative view (task 118) read. Linear posts updates with a
health word and free text; ours adds the pin, so a reader can see that an *on track* was written
against revision 3 of a spec that is now at revision 5 (L-76, EXCEED).

Seven decisions the scope left open, made here.

1. **An update is a TM-46 note that never announces itself as one.** The words live in TM-46's
   `notes` table as an `nt-` note whose target is a **project** (`TM46:85`), a target TM-46
   already has, so that table is not altered; the Work half (`subject`, `health`, `pinned_brief`,
   `cites`) lives in task 116's `work_update_meta`, keyed by the note id. The handler inserts both
   rows in one transaction and appends **`work.update_posted`**, never `note.created`. That is the
   whole of the "not an inbox note" rule: Fleet learns of a note **only** from `note.created` on its
   per-org feed (`TM46:264-266`), so it never learns of an update.
2. **Route 73 stops listing updates.** Fleet rebuilds its unread set after a restart from TM-46's
   route 73 (`TM46:267-268`), which lists every note of a project. Its query gains one clause, `AND
   NOT EXISTS (SELECT 1 FROM work_update_meta m WHERE m.note_id = n.id)`, in TM-46's store file (⚑,
   rule H2: the Phase T file wins and the clause is re-applied on top of it). Without it, the first
   Fleet restart after an update would deliver it to every member's Orchestrator.
3. **The pin.** For a feature, `pinnedBrief` defaults to the current head revision of the feature's
   branch brief and may name an earlier revision of **that** brief; any other brief, or a revision
   that does not exist, is 422 `bad_request`, `field: "pinnedBrief"`. For an initiative, the author
   pins **each feature they report on**, and one field cannot hold that: task 114's `StatusUpdate`
   carries one `pinnedBrief` (its scope, `tasks-101-120.md:361-363`). **Gap G1, fixed in 114:**
   `StatusUpdate` gains `featurePins [{featureId, pinnedBrief}]`, required and non-empty for an
   initiative subject, forbidden for a feature subject, and `Validate` requires `pinnedBrief` for
   a feature subject only. This task appends the child table `work_update_pins` to task 116's
   migration (the pattern task 14 uses on task 06's file), because 116's scope has one
   `pinned_brief` column.
4. **Cites resolve as `/v1/refs` resolves them.** Each key is resolved by the Hub's own `/v1/refs`
   path under the author's principal (`hub-contract:356`: *"resolves keys in readable projects;
   every other key is `missing`"*); a `missing` key is 422 `bad_request`, `field: "cites"`,
   `index: i`. An unreadable key and a key that does not exist give the same answer, so the refusal
   leaks nothing (`hub-contract:273`). The resolver's Go name is the Hub plan's and is **UNVERIFIED**
   here; this task calls it and does not copy it. At most 20 cites; duplicates are refused.
5. **A person posts; an agent never does.** The route takes TM task 07's `RequireDesktop` and a
   `user` principal; Fleet under any delegation and a session token are 403 `forbidden`,
   `required: "desktop"`, as TM-46's route 72 (`TM46:80-82`). An agent's draft (task 139) stays
   `text·A` on the desktop until a person edits and posts it.
6. **Immutable, and the attempt is named.** `PATCH /v1/updates/{updateId}` is registered only to
   answer 409 `update_immutable` (after the read check, so an unreadable id stays 404), so a client
   that tries gets a code rather than 405. TM-46's `guard_note` and 116's trigger refuse the
   `UPDATE` underneath it. Erase is TM-46's route 81 unchanged (`TM46:109-117`): the text goes, the
   id, health, pins, cites and audit event stay.
7. **An initiative update needs a project row to live in.** TM-46's note belongs to a project
   (`TM46:88`); an initiative is a team's (task 118). The note is stored on the project of the
   **first** entry of `featurePins`, as a storage anchor only: every read of an initiative update
   goes through the initiative routes and 118's rule, never through route 73. A Phase T note with
   no project would be cleaner; that is a note for the sync with Phase T, not a change here.

**The one thing this task must not get wrong** is an update reaching an Orchestrator. A status
update is written for people who look; an Orchestrator that reads *"at risk"* in its inbox would act
on a person's opinion as if it were an instruction. Decisions 1 and 2 close both doors Fleet has,
and `TestNoOrchestratorEverReceivesAnUpdate` walks both.

**Files:**

- `control-plane/internal/hub/migrate/sql/NNNN_work_roadmap.sql` ⚑ (task 116's file; a section
  appended, or the next free number if 116 has merged into `feat/hub`, rule H1): `work_update_pins`
- `control-plane/internal/hub/store/work_updates.go` (new): the meta, pins and list queries
- `control-plane/internal/hub/server/work_updates.go` (new): five routes
- `control-plane/internal/hub/server/work_updates_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/store/notes.go` ⚑ (TM task 46's file, **UNVERIFIED** name; rule H2):
  route 73's list query gains decision 2's clause, and nothing else
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ (task 07's `workMatrixRows`):
  the `"119"` rows, and `"119"` appended to `workMatrixLive`

**Interfaces:**

- Produces:
  - routes (SW.20): `POST /v1/features/{featureId}/updates`, `GET /v1/features/{featureId}/updates`,
    `POST /v1/initiatives/{initiativeId}/updates`, `GET /v1/initiatives/{initiativeId}/updates`,
    `PATCH /v1/updates/{updateId}` (409 `update_immutable` only)
  - `store.InsertUpdateMeta`, `store.ListUpdates`, `store.UpdateByID`, `store.LatestUpdateAt`
    (task 121's reminder pass and task 118's `latestUpdate` read it)
  - the kind `work.update_posted` (declared by task 114; gap G2 below)
- Consumes:
  - task 114: `hubapi.StatusUpdate`, `hubapi.Health`, `CodeUpdateImmutable`; task 116:
    `work_update_meta` and its trigger; task 118: `store.InitiativeByID`, its read rule, and the
    feature's `initiative_id`
  - TM task 46: the `notes` table, its `guard_note`, `tombstone_note` and route 81 (`TM46:160-202`,
    `:109-117`); TM task 07: `RequireDesktop` (`TM46:67`)
  - Hub task 20: the `/v1/refs` resolver in `store/refs.go` (`phase-h-hub.md:302`, `:423`), which
    task 18's `issue:` case extends; Hub task 14: `server.Write`, `server.Read`, `server.Decode`
  - task 13: `emitWork` (`task-13.md:332-346`); task 08: the feature's branch brief id (`ft-` →
    `bf-`), read through the feature row
- Consumed by: task 118 (`latestUpdate`), task 121 (the reminder pass), task 124 (`hubclient`'s
  `PostUpdate` over HTTP and the relay), task 127 (the composer), task 139 (a drafted update is
  posted through this route by the person who accepts it)

**Two gaps in task 114, listed here and fixed there.** **G1** is decision 3's `featurePins`.
**G2:** 114's scope names the W4 wire types and methods but no W4 **event kinds**, while task 13's
registry says *"W2-W5 append their kinds (tasks 72, 99, 114, 143)"* (`task-13.md:250`). This task
needs one, written below as 114 must declare it. Until 114 carries both, the handler cannot
compile, and the part says so rather than declaring them here.

#### The migration section (appended to `NNNN_work_roadmap.sql`, complete)

```sql
-- ── task 119: an initiative update pins each feature it reports on (gap G1) ──
SET search_path = hub, pg_temp;

CREATE TABLE work_update_pins (
  org_id       text NOT NULL,
  note_id      text NOT NULL REFERENCES work_update_meta (note_id),
  ord          int  NOT NULL CHECK (ord >= 0 AND ord < 50),
  feature_id   text NOT NULL REFERENCES work_features (id),
  pinned_brief text NOT NULL CHECK (pinned_brief ~ '^brief:bf-[0-9a-f]{16}@[1-9][0-9]*$'),
  PRIMARY KEY (org_id, note_id, ord),
  UNIQUE (org_id, note_id, feature_id)
);
ALTER TABLE work_update_pins ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_update_pins FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_update_pins
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_update_pins TO hub_app;

-- A pin is as immutable as its update: 116's guard function, reused, not copied.
CREATE TRIGGER work_update_pins_immutable
  BEFORE UPDATE OR DELETE ON work_update_pins
  FOR EACH ROW EXECUTE FUNCTION guard_work_update_meta();
```

The table name `work_features`, the key regex and the guard function's name
`guard_work_update_meta` are task 116's and task 02's (`brief:` keys: Hub key grammar, `bf-` + 16
hex, **UNVERIFIED** against `hubapi/keys.go` until the build) and are renamed mechanically at build
time if those parts chose others. No `DELETE` grant: an update and its pins are never deleted, and
erase touches only the note's text (`TM46:191`). `org_id` is written from `hub.org_id` by the store,
as every Hub table.

#### The wire, as task 114 must carry it (G1 and G2; complete)

```go
// control-plane/internal/hub/hubapi/work_roadmap.go (task 114's file; the two additions)

// StatusUpdate, with G1's field. Everything else is 114's scope as written.
type StatusUpdate struct {
	ID          string        `json:"id"`   // nt-…
	Kind        string        `json:"kind"` // always "update"
	Subject     UpdateSubject `json:"subject"`
	Health      Health        `json:"health"`
	Body        *teamtext.Text `json:"body"`       // text·H (TM task 02's Text); null once erased
	PinnedBrief string        `json:"pinnedBrief"` // feature subject: brief:bf-…@r; else ""
	FeaturePins []FeaturePin  `json:"featurePins"` // initiative subject: ≥ 1; else []
	Cites       []string      `json:"cites"`       // ≤ 20 keys, as the author sent them
	Author      User          `json:"author"`
	At          string        `json:"at"`
	ErasedAt    *string       `json:"erasedAt"` // TM-46's tombstonedAt, renamed on the Work wire
}

type UpdateSubject struct {
	Kind string `json:"kind"` // "feature" | "initiative"
	ID   string `json:"id"`   // ft-… | in-…
}

type FeaturePin struct {
	FeatureID   string `json:"featureId"`
	PinnedBrief string `json:"pinnedBrief"`
}

type Health string

const (
	HealthOnTrack  Health = "on_track"
	HealthAtRisk   Health = "at_risk"
	HealthOffTrack Health = "off_track"
)

// PostUpdateRequest is the body of both POST routes. The author, the subject
// and the time come from the principal and the path, never from the body.
type PostUpdateRequest struct {
	Health      Health       `json:"health"`
	Body        string       `json:"body"`        // 1–1,000 runes after G3 hygiene (TM-46's rule)
	PinnedBrief string       `json:"pinnedBrief"` // feature: optional, defaults to the head revision
	FeaturePins []FeaturePin `json:"featurePins"` // initiative: required
	Cites       []string     `json:"cites"`
}

const MaxUpdateCites = 20

// G2: the kind and its data. Scalars only (task 13's EventData rule).
const EventWorkUpdatePosted = "work.update_posted"

type WorkUpdatePostedData struct {
	SubjectKind string `json:"subjectKind"` // "feature" | "initiative"
	SubjectID   string `json:"subjectId"`
	Health      string `json:"health"`      // a closed word, never text
	PinnedBrief string `json:"pinnedBrief"` // "" for an initiative
	Pins        int64  `json:"pins"`        // len(featurePins)
	Cites       int64  `json:"cites"`       // a count; the keys stay in the record
}

// Appended to WorkEventKinds by task 114. ScopeTeam, because an initiative has
// no project; a feature update's event still carries the team, and the feed's
// team rule (hub-contract:1103-1104) shows it to that team's members only.
//   {EventWorkUpdatePosted, WorkUpdatePostedData{}, ScopeTeam, VisibleToReaders},
```

`Body` reuses TM-46's stamped text shape on the way out (`TM46:90-91`; `teamtext` is TM task 02's
package, `control-plane/internal/teamtext/text.go`, **UNVERIFIED until merge**) and a plain string
on the way in, as TM-46's conventions say (`TM46:77-78`). `ErasedAt` is a rename of the note's
`tombstonedAt` so the Work wire reads like task 14's erased issues; the store maps one to the other.
**ScopeTeam leaks nothing across teams, but a feature update then reaches a granted team's feed
only through task 117's widening of `server/changes.go`**; 117 names the Work kinds it forwards,
and must name this one (a note for 117, not a change here).
