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
   carries one `pinnedBrief` (its `StatusUpdate`). **Gap G1, fixed in 114:** `StatusUpdate` and
   `PostUpdateRequest` gain `featurePins [{featureId, pinnedBrief}]`, non-empty for an initiative
   subject and empty for a feature; the stored `pinnedBrief` of an initiative update is its first
   pin, so the record's one field is never empty. This task appends the child table
   `work_update_pins` to task 116's migration (the pattern task 14 uses on task 06's file),
   because 116's scope has one `pinned_brief` column.
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
**G2:** 114's scope, and its part at this writing, name the W4 wire types and methods but no W4
**event kinds**, while task 13's
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
  feature_id   text NOT NULL REFERENCES features (id),
  pinned_brief text NOT NULL CHECK (pinned_brief ~ '^brief:bf-[0-9a-f]{16}@[1-9][0-9]{0,8}$'),
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

The table `features`, the key regex and the guard `guard_work_update_meta` are read from task
116's part (in progress at this writing: its `work_update_meta` uses the same regex and
`REFERENCES features (id)`); the key shape is the Hub's (`bf-` + 16 hex, **UNVERIFIED** against
`hubapi/keys.go` until the build). Names are renamed mechanically at build time if 116 changes
them. No `DELETE` grant: an update and its pins are never deleted, and
erase touches only the note's text (`TM46:191`). `org_id` is written from `hub.org_id` by the store,
as every Hub table.

#### The wire: task 114's types, and what 114 must add (G1 and G2)

Task 114's part (`task-114.md`, in progress at this writing, so cited by type name rather than
line) declares `Health`, `SubjectRef` with `SubjectFeature`/`SubjectInitiative`, `StatusUpdate`
(`Body AuthoredText`, `PinnedBrief` required, `ErasedAt`) and `PostUpdateRequest` (`Body
AuthoredText`, author `H`). This task uses them as written. Two additions it needs are not
there, and are written here as 114 must carry them:

```go
// control-plane/internal/hub/hubapi/work_roadmap.go (task 114's file) — G1, additive

// FeaturePin is one feature an initiative update reports on, pinned to the
// revision of that feature's branch brief the author wrote against.
type FeaturePin struct {
	FeatureID   string `json:"featureId"`   // ft-…
	PinnedBrief string `json:"pinnedBrief"` // brief:bf-…@r of that feature's branch brief
}

// On StatusUpdate:      FeaturePins []FeaturePin `json:"featurePins"` // never null; [] for a feature
// On PostUpdateRequest: FeaturePins []FeaturePin `json:"featurePins"` // initiative: 1-50; feature: []
//
// Validate, for an initiative subject: FeaturePins non-empty, no FeatureID twice,
// and PinnedBrief == "" in the request. The Hub then stores PinnedBrief =
// FeaturePins[0].PinnedBrief, so 114's rule "a StatusUpdate always has a
// PinnedBrief" holds for both subjects and a reader of one field is never wrong.

const MaxUpdateCites = 20 // 114's "0-20 Hub keys" on PostUpdateRequest.Cites, named
```

`PostUpdateRequest.PinnedBrief` may be `""` for a **feature** subject, meaning *the head revision*
(decision 3); the stored record always carries the resolved key. If 114's `Validate` refuses an
empty request pin, that is the one line 114 changes, and it is a request rule, not a record rule.

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

`Body` is 114's `AuthoredText` both ways; the handler maps it to and from TM-46's stamped
`teamtext.Text` at the note insert (`TM46:90-91`; `teamtext` is TM task 02's package,
`control-plane/internal/teamtext/text.go`, **UNVERIFIED until merge**), and an erased update
answers with `body.text` `""`, as task 14's erased issues do. `ErasedAt` is the note's
`tombstonedAt` renamed on the Work wire; the store maps one to the other.
**ScopeTeam leaks nothing across teams, but a feature update then reaches a granted team's feed
only through task 117's widening of `server/changes.go`**; 117 names the Work kinds it forwards,
and must name this one (a note for 117, not a change here).

#### The store (`store/work_updates.go`, signatures complete)

```go
package store

// UpdateMetaRow is one work_update_meta row with its pins, joined to the note.
type UpdateMetaRow struct {
	NoteID, SubjectKind, SubjectID, Health, PinnedBrief string
	Pins                                                []FeaturePinRow
	Cites                                               []string
	AuthorUserID                                        string
	Body                                                *string // notes.text_body; nil once tombstoned
	CreatedAt                                           time.Time
	TombstonedAt                                        *time.Time
}
type FeaturePinRow struct{ FeatureID, PinnedBrief string }

// InsertUpdateMeta writes the meta row and its pins, in the caller's write
// transaction, after TM-46's note insert has minted noteID. The rows are
// never updated: task 116's guard refuses it.
func InsertUpdateMeta(ctx context.Context, tx pgx.Tx, r UpdateMetaRow) error

// ListUpdates is newest first by (created_at DESC, note_id DESC); after is the
// last note id of the previous page; limit 1–100. Tombstoned updates are
// listed with Body nil, never omitted (TM46:106-107).
func ListUpdates(ctx context.Context, q Querier, subjectKind, subjectID, after string, limit int) ([]UpdateMetaRow, error)

// UpdateByID finds one update under row-level security; found is false for an
// id with no meta row, which includes every ordinary note.
func UpdateByID(ctx context.Context, q Querier, noteID string) (r UpdateMetaRow, found bool, err error)

// LatestUpdateAt is the created_at of the subject's newest update, erased or
// not (an erased update still happened), or zero when there is none.
func LatestUpdateAt(ctx context.Context, q Querier, subjectKind, subjectID string) (time.Time, error)

// BriefHeadRevision reads the branch brief's head revision number, for the
// default pin. A feature with no branch brief yet is ErrNoBrief.
func BriefHeadRevision(ctx context.Context, q Querier, briefID string) (int, error)
var ErrNoBrief = errors.New("store: the feature has no branch brief")
```

The note itself is inserted with TM-46's own store function (**UNVERIFIED** name, `InsertNote`
in the sketch below), with target `{kind: "project", id: <project>}`, `author: "human"` and the
author from the principal, so every rule TM-46's route 72 applies to the words (length, G3 hygiene,
the stamp) applies here unchanged, and no second copy of them exists.

#### The routes (`server/work_updates.go`)

```go
mux.HandleFunc("POST /v1/features/{featureId}/updates", s.postFeatureUpdate)
mux.HandleFunc("GET /v1/features/{featureId}/updates", s.listFeatureUpdates)
mux.HandleFunc("POST /v1/initiatives/{initiativeId}/updates", s.postInitiativeUpdate)
mux.HandleFunc("GET /v1/initiatives/{initiativeId}/updates", s.listInitiativeUpdates)
mux.HandleFunc("PATCH /v1/updates/{updateId}", s.patchUpdate)
```

**`postFeatureUpdate`**, inside `s.Write`, in this order:

1. `RequireDesktop` and a `user` principal, else 403 `forbidden`, `required: "desktop"` (decision 5).
2. The feature, read through the caller's team membership: not readable → 404 `feature_not_found`;
   readable only through a task 117 grant, or as a `viewer` → 403 `forbidden`, `required: "member"`.
3. `server.Decode[hubapi.PostUpdateRequest]` (strict; an `author`, `at` or `subject` field is 400);
   `health` one of the three words, else 422 `field: "health"`; `body.author` other than `H` → 422
   `field: "body"` (an agent's words are never posted as a person's); `featurePins` non-empty →
   422 `field: "featurePins"`.
4. The pin: empty → `brief:<the feature's branch brief>@<BriefHeadRevision>`; `ErrNoBrief` → 422
   `field: "pinnedBrief"`, `reason: "no_brief"`; set → it must parse as a `brief:` key of **that**
   brief at a revision between 1 and the head, else 422 `field: "pinnedBrief"`.
5. `cites`: at most `MaxUpdateCites`, no duplicate, each resolved through Hub task 20's resolver
   under the caller's principal; a `missing` answer → 422 `field: "cites"`, `index: i`.
6. TM-46's note insert on the feature's project; `InsertUpdateMeta`; `emitWork(tx,
   EventWorkUpdatePosted, team, "", noteID, WorkUpdatePostedData{…})`; 201 `StatusUpdate`.

**`postInitiativeUpdate`** is the same with task 118's initiative read (404
`initiative_not_found`; `viewer` 403), `pinnedBrief` required empty (else 422), and each
`featurePins` entry checked as step 4 against its own feature, which must belong to the initiative
(else 422 `field: "featurePins"`, `index: i`), with no feature twice. The note's project is the
first pin's feature's project (decision 7).

**The stored reply.** A keyed POST stores its 201 in `idempotency_keys.response` with `body:
null` and rebuilds `body` and `erasedAt` from the rows on replay, TM-46's pattern for route 72
(`TM46:141-147`), so an erase leaves no words in a stored reply.

**`listFeatureUpdates`** and **`listInitiativeUpdates`**, inside `s.Read`: the subject's read rule
(a granted team reads a feature's updates, task 117; an initiative's are read under 118's rule),
`after` and `limit` as `ListUpdates`, `200 Page[StatusUpdate]`. **`patchUpdate`**: `UpdateByID`
under the caller's read rule, else 404 `note_not_found` (TM-46's code, **UNVERIFIED** name, as
task 114's decision 4 says); otherwise 409 `update_immutable`, always,
with nothing written and no event (a refused write changes no record, `hub-contract:1133`).

**G3, codes:** `initiative_not_found` is 114's (its `codes.go` block); an unknown update is TM-46's
`note_not_found`, as 114's decision 4 decides; `feature_not_found` is W2's if task 72 declares it
(its part is not yet written) and 114's to append if not. This task declares none.

#### Tests to write first — `server/work_updates_integration_test.go` (`//go:build integration`)

The cast and harness are task 11's (Ada, Portal's team admin; Grace, a Portal member; Linus, in
Billing only; Xavier, of another org) plus Olga, an org admin in no team (task 14), task 117's
**Gia**, a member of Billing after Ada grants Billing a read-and-comment grant on Portal, and a
Fleet principal with Grace's `orchestrator` delegation (TM task 07's fixture). The subject is
feature FT-1 of project Portal, whose branch brief `bf-A` is at revision 3, and initiative IN-1
holding FT-1 and a Billing feature FT-9 (task 117's span).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnUpdateIsPinnedByDefault` | Grace posts `{health: at_risk, body}` with no pin → 201, `pinnedBrief: brief:bf-A@3`; after the brief moves to revision 4, a post pinning `@2` → 201 `@2`, and `@5` or `brief:bf-B@1` → 422 `field: pinnedBrief` | an unpinned update (L-76's EXCEED); a pin to any brief |
| `TestAFeatureWithNoBriefCannotBeUpdated` | a feature whose brief is not yet created → 422 `reason: no_brief`, nothing written | a pin invented from nothing |
| `TestAnInitiativeUpdatePinsEachFeature` | Ada posts on IN-1 with `featurePins` for FT-1 and FT-9 → 201, two pins in order, and the record's `pinnedBrief` equals FT-1's pin; no pins → 422; FT-1 twice → 422; a feature outside IN-1 → 422 `index: 1`; `pinnedBrief` set → 422 | an initiative update pinned to nothing, or to one brief for all |
| `TestHealthIsOneOfThreeWords` | `"fine"`, `""` and `"On_Track"` → 422 `field: health` | free-text health |
| `TestCitesResolveOrTheUpdateIsRefused` | cites `issue:EP-142`, a `brief:` and an `ev-` of Portal → 201; a Billing issue key from Grace → 422 `field: cites`, `index: 0`, the same answer as a key that exists nowhere; 21 cites, or one twice → 422 | cites that leak or confirm another project's records |
| `TestOnlyAPersonPosts` | the Fleet principal under Grace's delegation, and a session token, → 403 `required: desktop` on both POST routes, nothing written; Grace posting a `body` with author `A` → 422 `field: body` | agents post updates; an agent's draft posted as a person's words |
| `TestWhoMayPostAndRead` | V → POST 403, GET 200; M and A → 201 and 200; Gia (granted) → POST 403 `required: member`, GET 200; Linus, Olga and Xavier → 404 `feature_not_found` on both | a grant that writes; an update readable outside its readers |
| `TestAnUpdateIsImmutable` | `PATCH /v1/updates/{id}` → 409 `update_immutable`, no event; a direct `UPDATE work_update_meta SET health = 'on_track'` as `hub_app` → refused; `PATCH` of an unreadable id → 404 | an editable update |
| `TestAnErasedUpdateKeepsItsFacts` | Olga erases the update through TM-46's route 81 → the list shows it at its place with `body: null`, `erasedAt` set, and the same `health`, `pinnedBrief`, `featurePins` and `cites`; `markerHits` of the body's canary is empty, `idempotency_keys.response` included | erase drops the row; a stored reply keeps the words |
| `TestNoOrchestratorEverReceivesAnUpdate` | after Grace posts: the org feed holds `work.update_posted` and **no** `note.created`; route 73 on Portal, as the Fleet principal, lists none of it; an ordinary TM-46 note posted next is listed by route 73 and reaches Ada's `panacea inbox` (TM-46's own row), the update never does | an update routed as a note (decisions 1 and 2) |
| `TestEachPostAppendsOneEvent` | one post → one `work.update_posted` with the six `data` keys and no text; a keyed replay → `Idempotent-Replay: true` and no second event; a refused post → no event | a write without its event; an event carrying the body |
| `TestUpdatesListNewestFirst` | three posts, then `limit=2` → the two newest, `next` set; the page after it → the oldest; an erased one keeps its place | a list that drops or reorders erased updates |

*Gia* is this part's name for task 117's granted-team member; 117's fixture fixes the name and
this table follows it. `markerHits` is Hub task 27's helper, reused unchanged, as task 14 does.

#### The matrix rows (`workMatrixRows["119"]`, appended)

Columns V M A O N X (`hub-contract:341`); the granted-team member is **task 117's added row**, not
a seventh column, so the grid stays additive.

| Route | V | M | A | O | N | X | Granted member |
|---|---|---|---|---|---|---|---|
| `POST /v1/features/{featureId}/updates` | 403 | 201 | 201 | 404 | 404 | 404 | 403 |
| `GET /v1/features/{featureId}/updates` | 200 | 200 | 200 | 404 | 404 | 404 | 200 |
| `POST /v1/initiatives/{initiativeId}/updates` | 403 | 201 | 201 | 404 | 404 | 404 | 403 |
| `GET /v1/initiatives/{initiativeId}/updates` | 200 | 200 | 200 | 404 | 404 | 404 | as task 118's initiative read |
| `PATCH /v1/updates/{updateId}` | 409 | 409 | 409 | 404 | 404 | 404 | 409 |

`O` is 404 for a Work read, as task 07's `wread` helper has it (`task-07.md:379-380`): an org admin
outside the team reads no Work object, and erases through route 81 by id alone.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): tasks 114, 116 and 118 committed; TM task 46
  merged on the sync base with its `notes` table and route 73 (rule H2: if it has not landed, this
  task waits). Re-read `TM46:80-117` and `:228-270` at the new base; follow any rename.
- [ ] **Step 2:** Check 114 carries G1, G2 and G3. If not, stop and raise it against 114; do not
  declare the fields or the kind here.
- [ ] **Step 3:** Append the migration section; `hub migrate` against the compose stack applies it;
  task 116's migration test still passes on an empty database and on W3's schema.
- [ ] **Step 4:** Write `server/work_updates_integration_test.go` and the matrix rows; every row
  fails.
- [ ] **Step 5:** `store/work_updates.go`, then the five handlers; run until green.
- [ ] **Step 6:** Add decision 2's clause to route 73's query in TM-46's store file; rerun TM-46's
  own integration tests (`TestANoteReachesAnOrchestratorOnlyByInboxQuoted`, `TM46:281`) unchanged
  and green.
- [ ] **Step 7:** Rerun task 115's update conformance rows against the fake; they agree with the
  Hub on every code above.
- [ ] **Step 8:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(AnUpdateIsPinnedByDefault|AFeatureWithNoBriefCannotBeUpdated|AnInitiativeUpdatePinsEachFeature|HealthIsOneOfThreeWords|CitesResolveOrTheUpdateIsRefused|OnlyAPersonPosts|WhoMayPostAndRead|AnUpdateIsImmutable|AnErasedUpdateKeepsItsFacts|NoOrchestratorEverReceivesAnUpdate|EachPostAppendsOneEvent|UpdatesListNewestFirst)$' \
  2>&1 | grep -c "^--- PASS"                                                                    # 12
go test -tags integration ./control-plane/internal/hub/... -race -count=1                      # ok
go test ./control-plane/... -race -count=1                                                      # ok
```

#### What this task does not claim

- That the health word is honest: it is a person's `text·H` opinion, shown as theirs. The pin
  lets a reader see how old the spec behind it is; it does not check the word.
- Update reminders (task 121, L-77), drafted updates (task 139, L-78), or the composer (task 127).
- Delivery anywhere: an update is read by people who look (the feature, the initiative, the Desk
  digest of task 81). It sends no mail, no inbox item and no push.
- That the note's home project for an initiative update (decision 7) is the right long-term
  model; it is a storage anchor pending the sync with Phase T.

**Parity rows served:** **L-76** (updates with health, EXCEED by the pin) and **L-87** (initiative
updates, with task 118). L-77 is task 121's and L-78 task 139's.

**Gaps for the skeleton and siblings:** G1 (`featurePins`) and G2 (the W4 event kinds: none in
114's part at this writing) belong to task 114, and G3 only if task 72 lacks `feature_not_found`; task 117 must forward
`work.update_posted` to a granted team's feed; route 73's clause is a delta on a Phase T file.
