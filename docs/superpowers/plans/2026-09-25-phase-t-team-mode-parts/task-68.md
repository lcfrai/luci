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

