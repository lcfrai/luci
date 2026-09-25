### Task 68: Release attribution on a team project (phase 7's thread)

**Slice:** TM2. **Focused time:** 2 h. **dependsOn:** 61 (the team land queue, the verified
push, the `Panacea-Owner` trailer), H23 (reserve and the pin), H24 (confirm, abandon, the
deployment join); 03 (`Task.Reason.BriefRef`, `AgentStart.TaskID`), 05 and 06 (the delegated
write shape) and 08 (the fake) through them. **Seam:** none new; it extends the Hub contract's
§7.9.

The product spine is one provenance thread: the brief section written in Define reaches the
release in Deploy (`CLAUDE.md`, *Product spine*). On a solo project the Hub's pin already carries
the brief to the release (contract §7.9, `hub-contract.md:1011-1016` on `feat/hub`). On a team
project that is not enough: a release carries several people's work, and `05-design.md` §11.5
asks that the thread reach the release **with its people**. So a team release lists, per lane
landed into it, the person whose agent made it, the `tk-…` task it served and that task's brief
section. Phase 7, which calls Panacea's API, reads the list from the release; phase 7 has no plan
on disk yet (skeleton `:1113-1114`), so this part defines **the Hub side only**.

Hub tasks 23 and 24 are planned, not built: the `Release` type exists (`hubapi/types.go:459-481`
on `feat/hub`, checked), and `server/releases.go` and `store/releases.go` are task 23's new files
(`phase-h-hub-parts/task-23.md:12-15`). Every hunk on them is **UNVERIFIED until merge**.

Five decisions the scope left open, made here so nobody re-derives them.

**(a) A land is a Hub record, and it is the only source.** The Hub has no git, so it cannot read
task 61's `Panacea-Owner` trailer (`task-61.md:45-47`). What it can hold is an accountability fact:
*this person's lane landed into this branch as this commit*. That is a Hub record by global
constraint 1 (losing it would lose who shipped what). Fleet writes it, **under the lane owner's
delegation, only after the push is verified** (task 61 decision d), so the owner is the
delegation's, never the body's (constraint 3). The trailer stays: it is the git-side witness a
reader outside the Hub can check, and Fleet refuses to write a land whose merge commit's trailer
names anyone but the delegation's owner.

**(b) The land names its agent; the Hub derives the task and the section.** The body carries the
worker's `ag-…`. The Hub reads that agent's record (`AgentStart.TaskID`, `task-03.md:566-579`) and
that task's `Reason.BriefRef` (`task-03.md:497-500`), inside the write's transaction. A land
therefore cannot claim a task or a brief section its agent did not serve. A lane whose agent had no
task lands with `taskId: null` and `briefRef: null`, and the release says so rather than guess.

**(c) Attribution is taken in the reservation's transaction, then never again**, exactly as the pin
is (`hub-contract.md:1011`, *"in the reservation's transaction, then never again"*). A land that
arrives after the reservation changes no reserved release. That keeps P2, *never pinned by
observation*, true for people too.

**(d) "Landed into it" is decided by commit identity along the queue, never by ancestry.** The
Hub numbers lands per `(project, branchRef)` with a `sequence`. A `run` release whose `commit`
equals some land's `mergeCommit` on its `branchRef` lists every land on that branch with a
sequence after the previous **confirmed** run release's last land and up to and including that
one. A release commit that no land produced (someone pushed outside the queue, or a later commit
on top) gets `status: "none"`, `reason: "commit_not_landed"`, and an empty list: the Hub cannot tell
what is in it and does not pretend. At most 200 lands are listed; beyond that `more: true` and
`GET …/lands?branchRef=&after=` pages the rest.

**(e) A rollback inherits.** A `rollback`, or a `run` with `sourceDeploymentId`, copies the
attribution of the source release it already copies the pin from (`hub-contract.md:1016`), with
`status: "inherited"`; no source release is `none` / `source_unknown`. A solo project's release is
`none` / `solo_project`, so every release carries the field and a reader never branches on its
absence.

**Files** (Hub delta, built after the `feat/hub` sync-merge and after Hub tasks 23–24):
- `control-plane/internal/hub/hubapi/lands.go` — `Land`, `RecordLandRequest`, `Attribution`,
  `AttributedLand`, the prefix, route names and reasons (new)
- `control-plane/internal/hub/hubapi/types.go` — `Release.Attribution` (modify, one field after
  `DesignKey` at `:472`)
- `control-plane/internal/hub/migrate/sql/00NN_lands.sql` — `hub.lands`, `hub.release_lands` and
  three columns on `hub.releases` (new, after task 04's team migration)
- `control-plane/internal/hub/store/lands.go` (new); `.../store/releases.go` — attribution in
  `Reserve`, read back in `GetRelease` and `ListReleases` (modify, Hub task 23's file)
- `control-plane/internal/hub/server/lands.go` — routes 82 and 83 (new)
- `control-plane/internal/hub/server/releases.go` — nothing in the handler changes; the store call
  does the work in the same transaction (unmodified, named so nobody edits it)
- `control-plane/internal/hub/server/{lands,releases_attribution}_integration_test.go` (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` — rows 82, 83 (modify)
- `control-plane/internal/hub/hubclient/{hubclient.go,fake.go}` — `RecordLand`, `Lands`; the fake's
  half (modify, task 08's team methods)
- `control-plane/internal/fleet/landq/record.go` — after a verified push, the land record through
  task 11's outbox (new, in task 61's package)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` — §5.3 rows 82–83, §7.9's `Release`
  gains `attribution`, a new §7.28 (modify)

**Interfaces:**
- Produces: `hubapi.Land`, `hubapi.Attribution` on every `Release`; routes 82
  `POST /v1/projects/{projectId}/lands` (Fleet·O, route name `lands.record`) and 83
  `GET /v1/projects/{projectId}/lands?branchRef=&after=` (V, or Fleet·O, `lands.read`);
  `hubclient.RecordLand`, `hubclient.Lands`; consumed by phase 7 (unplanned) and task 69 item 3.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - Hub task 23: `store.Reserve(ctx, tx, projectID, req) (hubapi.Release, error)` and its pin step
    (`phase-h-hub-parts/task-23.md:12-15`; the function name is **UNVERIFIED**);
  - task 03: `AgentStart.TaskID`, `Task.Reason.BriefRef`, `ValidSha`, `ValidBranchRef`,
    `ValidID`, the prefix block (`task-03.md:497-515`, `:566-579`, `:459-460`, `:1400-1417`);
  - task 06: `s.Write`, `RequireFleet`, `ProjectRole`, `OwnedAgent`, `Refuse`, `tx.Emit`
    (`task-06.md:335-368`);
  - task 11: the outbox's `Enqueue` (the durable Hub write path; name **UNVERIFIED** against
    task 11's part);
  - task 61: the entry's `pushed` → verified transition (`task-61.md:58-63`), the lane's worker
    `ag-…`, the merge and lane-tip shas, the branch;
  - task 21: the delegation's route list, which gains `lands.record` and `lands.read`.

#### `hubapi/lands.go` (complete)

```go
package hubapi

// ── §7.28 lands and release attribution (Phase T task 68) ───────────────────

const PrefixLand = "ld" // Fleet mints it with ids.Wide, like the other team ids

const (
	RouteLandsRecord RouteName = "lands.record"
	RouteLandsRead   RouteName = "lands.read"
)

// Land is one lane landed by the team land queue, recorded after its push was
// verified. Owner, task and brief section are the Hub's, never the body's.
type Land struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"projectId"`
	OwnerUserID string  `json:"ownerUserId"` // the delegation's owner
	AgentID     string  `json:"agentId"`     // the lane's worker, the owner's
	TaskID      *string `json:"taskId"`      // from the agent record; null: none
	BriefRef    *string `json:"briefRef"`    // from the task's reason; null: none
	BranchRef   string  `json:"branchRef"`   // refs/heads/…
	MergeCommit string  `json:"mergeCommit"` // the pushed head
	LaneTip     string  `json:"laneTip"`
	Sequence    int     `json:"sequence"`    // per (project, branchRef), from 1, gap-free
	LandedAt    string  `json:"landedAt"`
}

type RecordLandRequest struct {
	ID          string `json:"id"`
	AgentID     string `json:"agentId"`
	BranchRef   string `json:"branchRef"`
	MergeCommit string `json:"mergeCommit"`
	LaneTip     string `json:"laneTip"`
}

type AttributionStatus string

const (
	AttributionListed    AttributionStatus = "listed"
	AttributionInherited AttributionStatus = "inherited"
	AttributionNone      AttributionStatus = "none"
)

type AttributionReason string

const (
	AttrTeamLands       AttributionReason = "team_lands"
	AttrInherited       AttributionReason = "inherited"
	AttrSoloProject     AttributionReason = "solo_project"
	AttrCommitNotLanded AttributionReason = "commit_not_landed"
	AttrSourceUnknown   AttributionReason = "source_unknown"
)

// Attribution is fixed in the reservation's transaction, as the pin is.
type Attribution struct {
	Status AttributionStatus `json:"status"`
	Reason AttributionReason `json:"reason"`
	Lands  []AttributedLand  `json:"lands"` // sequence order; [] when none
	More   bool              `json:"more"`  // more than MaxAttributedLands
}

// AttributedLand is one person's lane in a release: who, which task, which
// brief section. No title and no free text: Task.Title is Fleet·O only.
type AttributedLand struct {
	LandID      string  `json:"landId"`
	Owner       User    `json:"owner"` // contract §3's User, as reservedBy
	AgentID     string  `json:"agentId"`
	TaskID      *string `json:"taskId"`
	BriefRef    *string `json:"briefRef"`
	MergeCommit string  `json:"mergeCommit"`
	Sequence    int     `json:"sequence"`
	LandedAt    string  `json:"landedAt"`
}

const MaxAttributedLands = 200
```

**`types.go`, the one field** (after `DesignKey` in `Release`, `:472`):

```go
	Attribution        Attribution      `json:"attribution"` // Phase T task 68; never absent
```

Hub task 23's golden `Release` fixtures gain the field; a solo fixture's value is
`{"status":"none","reason":"solo_project","lands":[],"more":false}`.

#### `00NN_lands.sql` (complete)

```sql
create table hub.lands (
    id           text        primary key check (id ~ '^ld-[0-9a-f]{16}$'),
    project_id   text        not null,
    owner_user_id text       not null,
    agent_id     text        not null,
    task_id      text,                                  -- null: the agent had no task
    brief_ref    text,                                  -- null: the task had none, or no task
    branch_ref   text        not null check (branch_ref like 'refs/heads/%'),
    merge_commit text        not null check (merge_commit ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
    lane_tip     text        not null check (lane_tip     ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
    sequence     integer     not null check (sequence >= 1),
    landed_at    timestamptz not null,
    unique (project_id, branch_ref, sequence),
    unique (project_id, branch_ref, merge_commit)       -- one land per pushed head
);

-- The lands a release lists, fixed at reserve. position keeps sequence order.
create table hub.release_lands (
    release_id text    not null references hub.releases (id),
    land_id    text    not null references hub.lands (id),
    position   integer not null check (position between 1 and 200),
    primary key (release_id, land_id)
);

alter table hub.releases
    add column attribution_status text    not null default 'none'
        check (attribution_status in ('listed', 'inherited', 'none')),
    add column attribution_reason text    not null default 'solo_project',
    add column attribution_more   boolean not null default false,
    add column attribution_last_sequence integer;   -- the last land listed; the next release starts after it
```

A release reserved before this migration reads as `none` / `solo_project`, which is true of every
release that existed then: no team land could precede the table. `hub.releases (id)` is Hub task
23's table name, **UNVERIFIED until merge**.

#### `store/lands.go`, the two writes that matter (complete)

```go
// RecordLand inserts one land in the caller's transaction. The sequence is
// the branch's next number, taken under a row lock on the project so two
// lands on one branch cannot share one. A replayed id with the same body
// returns the stored land (the outbox retries); another body is 409 conflict.
func RecordLand(ctx context.Context, tx *Tx, projectID, owner string, req hubapi.RecordLandRequest) (hubapi.Land, error) {
	if l, ok, err := landByID(ctx, tx, req.ID); err != nil || ok {
		if ok && !sameLand(l, projectID, owner, req) {
			return hubapi.Land{}, conflict(req.ID)
		}
		return l, err
	}
	agent, err := agentByID(ctx, tx, projectID, req.AgentID) // task 06's store read
	if err != nil {
		return hubapi.Land{}, err
	}
	var briefRef *string
	if agent.TaskID != nil {
		t, err := taskByID(ctx, tx, projectID, *agent.TaskID)
		if err != nil {
			return hubapi.Land{}, err
		}
		if t.Reason.BriefRef != "" {
			briefRef = &t.Reason.BriefRef
		}
	}
	if _, err := tx.Exec(ctx, `select 1 from hub.projects where id = $1 for update`, projectID); err != nil {
		return hubapi.Land{}, err
	}
	var seq int
	if err := tx.QueryRow(ctx, `select coalesce(max(sequence), 0) + 1 from hub.lands
		where project_id = $1 and branch_ref = $2`, projectID, req.BranchRef).Scan(&seq); err != nil {
		return hubapi.Land{}, err
	}
	l := hubapi.Land{ID: req.ID, ProjectID: projectID, OwnerUserID: owner, AgentID: req.AgentID,
		TaskID: agent.TaskID, BriefRef: briefRef, BranchRef: req.BranchRef,
		MergeCommit: req.MergeCommit, LaneTip: req.LaneTip, Sequence: seq, LandedAt: tx.Now()}
	return l, insertLand(ctx, tx, l) // the unique merge_commit index answers 409 conflict
}
```

`landByID`, `sameLand`, `agentByID`, `taskByID`, `insertLand`, `conflict`, `tx.Exec`, `tx.QueryRow`
and `tx.Now` are this part's names for Hub task 14's store helpers and pgx's calls, **UNVERIFIED
until merge**; `hub.projects` is the Hub's project table. The ownership check is not here: the
handler has already run `OwnedAgent` (rule 5) before the store call, as task 06's handlers do.

#### `store/releases.go`, the attribution step (Hub task 23's `Reserve`, after its pin step)

```go
// attribute fixes a release's attribution inside the reservation's
// transaction (decision c). src is the source release the pin step already
// resolved for a rollback or a run with sourceDeploymentId; nil otherwise.
func attribute(ctx context.Context, tx *Tx, projectID string, req hubapi.ReserveRequest,
	team bool, src *releaseRow) (hubapi.Attribution, *int, []string, error) {
	none := func(r hubapi.AttributionReason) (hubapi.Attribution, *int, []string, error) {
		return hubapi.Attribution{Status: hubapi.AttributionNone, Reason: r, Lands: []hubapi.AttributedLand{}}, nil, nil, nil
	}
	switch {
	case !team:
		return none(hubapi.AttrSoloProject)
	case req.SourceDeploymentID != nil && src == nil:
		return none(hubapi.AttrSourceUnknown)
	case req.SourceDeploymentID != nil: // rollback, or a re-run: copy the source's, as the pin is copied
		a, ids, err := attributionOf(ctx, tx, src.ID)
		if err != nil {
			return hubapi.Attribution{}, nil, nil, err
		}
		a.Status, a.Reason = hubapi.AttributionInherited, hubapi.AttrInherited
		return a, src.LastSequence, ids, nil
	case req.Commit == nil || req.BranchRef == nil:
		return none(hubapi.AttrCommitNotLanded)
	}
	branch := *req.BranchRef
	head, ok, err := landByCommit(ctx, tx, projectID, branch, *req.Commit)
	if err != nil {
		return hubapi.Attribution{}, nil, nil, err
	}
	if !ok {
		return none(hubapi.AttrCommitNotLanded)
	}
	from := 0 // the previous confirmed release's last land, for this app and environment
	if prev, ok, err := lastAttributed(ctx, tx, projectID, req.PanaceaProject, req.Environment, branch); err != nil {
		return hubapi.Attribution{}, nil, nil, err
	} else if ok && prev < head.Sequence {
		from = prev
	}
	lands, more, err := landsBetween(ctx, tx, projectID, branch, from, head.Sequence, hubapi.MaxAttributedLands)
	if err != nil {
		return hubapi.Attribution{}, nil, nil, err
	}
	ids := make([]string, len(lands))
	for i, l := range lands {
		ids[i] = l.LandID
	}
	last := head.Sequence
	return hubapi.Attribution{Status: hubapi.AttributionListed, Reason: hubapi.AttrTeamLands,
		Lands: lands, More: more}, &last, ids, nil
}
```

`Reserve` writes the three columns and `hub.release_lands` (one row per id, `position` from 1)
in the same transaction as the pin. `landsBetween` answers the lands with `from < sequence ≤ to`,
the **latest** `limit` of them in ascending order, and `more` when earlier ones exist; each land's
`Owner` is joined from the Hub's users as `reservedBy` is. `lastAttributed` answers the
`attribution_last_sequence` of the latest `confirmed` release of the project with the same
`panaceaProject`, `branchRef` and requested `environment` (both null counts as the same). When
that boundary is at or past this head (an older commit shipped after a newer one), `from` stays 0
and the release lists the latest 200 lands up to its head, with `more`: re-shipping older work
names everyone in it rather than nobody. Only a `confirmed` release moves the boundary: an
abandoned or expired reservation attributed nothing that shipped. `hubapi.ReserveRequest` is the
built type (`hubapi/types.go:428-444` on `feat/hub`, `BranchRef` and `Environment` both
`*string`); `releaseRow`, `LastSequence` and the store helpers are Hub task 23's names as assumed
here.

#### `server/lands.go` (complete)

```go
package server

import (
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

func (s *Server) registerLands() {
	s.mux.HandleFunc("POST /v1/projects/{p}/lands", s.recordLand) // 82
	s.mux.HandleFunc("GET /v1/projects/{p}/lands", s.listLands)   // 83
}

// recordLand: Fleet·O only, the lane owner's delegation, task 06's order:
// caller class, then the project, then the body, then ownership.
func (s *Server) recordLand(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("p")
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if err := RequireFleet(tx.P, projectID); err != nil {
			return WriteResult{}, err
		}
		if _, _, ok, err := ProjectRole(r.Context(), tx, projectID); err != nil {
			return WriteResult{}, err
		} else if !ok {
			return WriteResult{}, Refuse(404, hubapi.CodeProjectNotFound, "No such project.", nil)
		}
		var req hubapi.RecordLandRequest
		if err := Decode(body, &req); err != nil { // strict: an owner or actor field is 400
			return WriteResult{}, err
		}
		if field := validateLand(req); field != "" {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "A land field is invalid.", map[string]any{"field": field})
		}
		if err := OwnedAgent(r.Context(), tx, req.AgentID); err != nil { // 403 agent_not_owner
			return WriteResult{}, err
		}
		if !store.IsWorker(r.Context(), tx, projectID, req.AgentID) {
			return WriteResult{}, Refuse(422, hubapi.CodeBadRequest, "A lane is a worker's.", map[string]any{"field": "agentId"})
		}
		l, err := store.RecordLand(r.Context(), tx, projectID, tx.P.UserID, req)
		if err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "land.recorded", ProjectID: projectID, Subject: l.ID, Data: map[string]any{
			"agentId": l.AgentID, "branchRef": l.BranchRef, "mergeCommit": l.MergeCommit, "sequence": l.Sequence}})
		return WriteResult{Status: 201, Body: l}, nil
	})
}

// validateLand names the first bad field: id (ld-, ValidID), agentId (ag-),
// branchRef (ValidBranchRef, refs/heads/ only), mergeCommit and laneTip
// (ValidSha), and laneTip == mergeCommit (a land is a merge, never a no-op).
func validateLand(req hubapi.RecordLandRequest) string {
	switch {
	case !hubapi.ValidIDWithPrefix(req.ID, hubapi.PrefixLand):
		return "id"
	case !hubapi.ValidIDWithPrefix(req.AgentID, hubapi.PrefixAgent):
		return "agentId"
	case !hubapi.ValidBranchRef(req.BranchRef):
		return "branchRef"
	case !hubapi.ValidSha(req.MergeCommit):
		return "mergeCommit"
	case !hubapi.ValidSha(req.LaneTip) || req.LaneTip == req.MergeCommit:
		return "laneTip"
	}
	return ""
}

// listLands: V (desk) or Fleet·O; ?branchRef= required, ?after=<sequence>,
// 100 a page in sequence order, next = the last sequence when more remain.
func (s *Server) listLands(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("p")
	s.Read(w, r, func(tx *Tx) (any, error) {
		if _, _, ok, err := ProjectRole(r.Context(), tx, projectID); err != nil || !ok {
			return nil, orNotFound(err)
		}
		branch, after, err := landsQuery(r.URL.Query()) // 422 field "branchRef" / "after"
		if err != nil {
			return nil, err
		}
		return store.LandsPage(r.Context(), tx, projectID, branch, after, 100) // hubapi.Page[hubapi.Land]
	})
}
```

`ValidIDWithPrefix` is task 03's `ValidID` with its prefix argument, as its name is at merge;
`orNotFound`, `landsQuery` and `store.IsWorker` (the agent record's `role == worker`) are this
part's names. A `land.recorded` event carries ids and shas only, no free text, as every team event
does (contract §3.6's metadata-only rule).

#### `fleet/landq/record.go` (complete; task 61's package)

```go
package landq

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// ErrTrailerOwner: the pushed merge names another principal in its
// Panacea-Owner trailer than the entry's owner. Nothing is recorded.
var ErrTrailerOwner = errors.New("landq: the merge's Panacea-Owner trailer is not the lane owner")

// Outbox is task 11's durable path: the write reaches the Hub exactly once,
// under the named owner's delegation, retried with the same id.
type Outbox interface {
	Enqueue(ctx context.Context, owner, route string, projectID string, body any) error
}

// Recorded is called once per entry, after the push is verified in the
// mirror (task 61 decision d) and before members are told of the new head.
func (q *Queue) Recorded(ctx context.Context, e Entry, trailerOwner string) error {
	if trailerOwner != e.Owner {
		return ErrTrailerOwner
	}
	if e.LandID == "" {
		e.LandID = ids.Wide(hubapi.PrefixLand) // minted once, stored on the entry, reused on retry
		if err := q.store.SetLandID(ctx, e.ID, e.LandID); err != nil {
			return err
		}
	}
	return q.outbox.Enqueue(ctx, e.Owner, string(hubapi.RouteLandsRecord), e.ProjectID, hubapi.RecordLandRequest{
		ID: e.LandID, AgentID: e.AgentID, BranchRef: e.BranchRef,
		MergeCommit: e.MergeCommit, LaneTip: e.LaneTip})
}
```

`Entry`, `Queue`, `q.store` and `q.outbox` are task 61's (`task-61.md`; its part is being written,
so these names are **UNVERIFIED** against it); `Entry` gains `LandID` and the trailer's principal
comes from the mirror's typed result for the merge (task 59, ST.7, one added field
`ownerTrailer`). `ids.Wide` is the Hub branch's addition to phase 2's `control-plane/internal/ids`
(`ids.go:61` on `feat/hub`, checked; absent on `feat/team-mode` until the sync-merge). A trailer mismatch is logged with the entry id and both principals, the land still
counts as pushed (the push was the owner's own act), and the release it goes into will say
`commit_not_landed`, which is the honest answer when the record and the commit disagree.

#### The contract text

**§5.3, two rows after task 66's row 81:**

```text
| 82 | POST | `/v1/projects/{projectId}/lands` | Fleet·O, the lane owner's delegation | `lands.record` | T68 | §7.28 |
| 83 | GET | `/v1/projects/{projectId}/lands?branchRef=&after=` | V (desk), or Fleet·O | `lands.read` | T68 | §7.28 |
```

**§7.9**, the `Release` shape gains, after `designKey`:
`"attribution": { "status": "listed" | "inherited" | "none", "reason": "team_lands" | "inherited" |
"solo_project" | "commit_not_landed" | "source_unknown", "lands": [AttributedLand], "more": false }`,
and the pin table gains one sentence: *"The attribution is taken in the same transaction as the
pin, by §7.28's rule, and never changes."*

**§7.28 Lands and release attribution.** *A land is written by Fleet under the lane owner's
delegation after the owner's push is verified. The owner is the delegation's; the task and the
brief section are the Hub's, read from the agent record and its task. `sequence` numbers the
lands of one `(project, branchRef)` from 1 with no gaps. A `run` release whose `commit` is a land's
`mergeCommit` on its `branchRef` lists the lands after the previous confirmed release of the same
Panacea project, environment and branch, up to that land, at most 200, latest first kept; any other
run is `none` / `commit_not_landed`; a rollback or a run with `sourceDeploymentId` inherits its
source's. An `AttributedLand` carries no title and no free text.*

#### The tests, first (each names the mutant it kills)

Integration tests against Hub task 16's test server and a real Postgres, with task 08's fake
Fleet delegation helper; one land-queue test on task 61's fakes.

| Test | Asserts | Mutant killed |
|---|---|---|
| `TestReleaseListsTwoPeoplesLanes` | Alice's worker (task `tk-a`, brief ref `…#auth`) and Bob's (task `tk-b`, `…#billing`) land in turn; a run release at Bob's merge commit: `listed`, two lands in sequence order, owners Alice and Bob, each with its own `taskId` and `briefRef` | the scope's test; one person per release, or the reserver credited for everyone |
| `TestSecondReleaseListsOnlyNewLands` | release 1 at land 2 confirmed; lands 3, 4; release 2 at land 4: lands 3 and 4 only | every release listing all of history |
| `TestAbandonedReleaseMovesNoBoundary` | release 1 at land 2 abandoned; release 2 at land 3: lands 1–3 | an abandoned reservation hiding lands from the next release |
| `TestOtherEnvironmentHasItsOwnBoundary` | staging confirmed at land 3; production at land 3: lands 1–3 | "new since the last release anywhere" |
| `TestCommitNotLandedSaysSo` | a run whose commit no land produced: `none` / `commit_not_landed`, `lands: []` | a guessed attribution by nearest land |
| `TestAttributionFixedAtReserve` | reserve at land 2; land 3 recorded; confirm; `GET` the release: still lands 1–2 | attribution read at GET time (P2 broken for people) |
| `TestRollbackInherits` | a rollback to release 1's deployment: `inherited`, release 1's lands; unknown source: `none` / `source_unknown` | a rollback re-deriving from the branch head |
| `TestSoloReleaseIsNone` | a solo project's release: `none` / `solo_project`, the field present | a missing field a reader must special-case |
| `TestLandOwnerIsTheDelegations` | a body with `ownerUserId`: 400 `bad_request`; Fleet under Alice's delegation naming Bob's `ag-…`: 403 `agent_not_owner`; a desk token: 403 `required` `fleet` | an owner from the body; a land in a teammate's name |
| `TestLandTaskAndSectionAreDerived` | the land body has no task field; the stored land's `taskId` and `briefRef` equal the agent's task's | a caller choosing the brief section its lane "served" |
| `TestLandSequenceGapFreeUnderRace` | two concurrent lands on one branch: sequences 1 and 2, never 1 and 1; a replay of land 1's id and body: 201 with the stored land, no third row | a lost row lock; a retry that double-counts |
| `TestLandRefusesOrchestratorAndNoOp` | an Orchestrator's `ag-…`: 422 `agentId`; `laneTip == mergeCommit`: 422 `laneTip` | a coordination agent credited with a lane |
| `TestAttributedLandHasNoTitle` | reflection over `AttributedLand` and the JSON of a listed release: no `title`, no `why`, no `teamtext.Text` | Fleet·O-only text leaking into a release every viewer reads |
| `TestAttributionCapsAt200` | 205 lands, one release at the last: 200 lands (6 to 205), `more: true`; route 83 pages the rest | an unbounded release body |
| `TestTrailerMismatchRecordsNothing` (`landq`) | the mirror reports trailer principal Bob for Alice's entry: `ErrTrailerOwner`, no `Enqueue` | a land recorded against a commit that names someone else |

#### Steps

1. **After the Hub sync-merge, and after Hub tasks 23 and 24 are merged there.** Write the
   integration tests; run them; each fails (routes 82–83 answer 404; `Release` has no
   `attribution`).
2. Write `hubapi/lands.go` and the `types.go` field; update Hub task 23's golden `Release`
   fixtures with the solo value; `go test ./internal/hub/hubapi/` green.
3. Write the migration, `store/lands.go` and the attribution step in `store/releases.go`; then
   `server/lands.go` and `registerLands()` beside task 06's registrations; add rows 82–83 to the
   matrix test.
4. Add `RecordLand` and `Lands` to `hubclient` and its fake (task 08's pattern); add `lands.record`
   and `lands.read` to the delegation task 21 creates for a worker-owning session.
5. Write `landq/record.go` and its test; call `Recorded` from task 61's verified-push step.
6. Amend the contract (§5.3 rows 82–83, §7.9, §7.28) word for word as above. Commit.

#### Acceptance

```bash
cd control-plane
go test ./internal/hub/server/ -run 'Release|Land|Attribut|Solo|Rollback|Commit|Environment|Abandoned' -count=1 -v 2>&1 | grep -c '^--- PASS'
# expect at least 14 (this task's 14 Hub tests, plus Hub task 23-24's that match)
go test ./internal/fleet/landq/ -run TrailerMismatch -count=1   # expect ok
go test ./... && go vet ./...                                   # expect ok; vet prints nothing
grep -n '"title"\|Why' internal/hub/hubapi/lands.go             # expect no output
```

The live proof, a real release on the cluster naming two people, is task 69 item 3's last
observation; it needs phase 7's Ship call, which has no plan yet, so task 69 reserves the release
with a direct Hub call and says so.

#### What this task does NOT claim

- **Anything on phase 7's side.** How the Deploy lens shows the list, and whether Panacea stores it,
  are phase 7's to plan; this part fixes only what the Hub answers.
- **That a release's contents are known when the queue was bypassed.** A push outside the queue,
  or a commit made on top of a land, yields `commit_not_landed`. The Hub has no git and does not
  walk history.
- **Per-commit authorship inside a lane.** A lane is one owner's worktree; its commits are
  attributed to that owner as a whole, by principal, never by git author (`04` §7.3).
- **That the brief section is still current.** `briefRef` is the section the task named when it
  was created; if the brief was revised since, the reader resolves the revision through the pin's
  brief key, as the Deploy lens does for a solo release.

#### Gaps, raised here and not resolved alone

1. **Task 61 expects this task to read the `Panacea-Owner` trailer** (`task-61.md:45-47`). The Hub
   cannot read git, so this task reads a land record instead and uses the trailer only as Fleet's
   pre-write check. Task 59's typed result (ST.7) must carry the merge's trailer principal
   (`ownerTrailer`), which it does not list today.
2. **Task 21's delegation route list** must include `lands.record` and `lands.read`; task 21's
   part is written without them.
3. **The skeleton's file map** has no row for `hubapi/lands.go`, `server/lands.go`,
   `landq/record.go` or routes 82–83; its delta row for task 68 (`:262`) names only "per-lane
   attribution on a team release".
4. **The release's environment.** `Release.Environment` can be null until a join names it
   (HA7-1); the boundary rule treats two null environments as one. If phase 7 decides otherwise,
   `lastAttributed` changes and `TestOtherEnvironmentHasItsOwnBoundary` with it.

