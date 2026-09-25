### Task 49: Phase 3 delta: the team land review: conflicts listed, a second approver for agent-config paths

**Slice TM1. Depends on: 07, P3-09, P3-16, P3-19. About 3 hours. Seam: none new.**

TM1 lands the phase 3 way: each member lands her own lanes in her own project repository
(`05-design.md` §8.3, `:919-921`). This task changes what that land requires **on a team
project** only, in two ways (`07` I1 and I7):

1. **The land review lists every conflict the lane was party to, with its resolution.** A lane's
   agent is party to a conflict when it is `agentA` or `agentB` of a Hub `cf-…` record (task 01
   §7.17, `task-01.md:377-389`). The review shows each one: tier, the other side's owner, the paths,
   the state and the resolution, or *"unresolved"*.
2. **A lane that changes an agent-config path needs a second member's approval to land.** Alice
   reviewing Alice is not enough when the change alters how every teammate's agent behaves
   (`05-design.md:924-927`). The approval is a **Hub record** written by the approver, as herself,
   from her own desktop.

It extends phase 3's `Lander.Land` (task 09, `task-09.md:98-132`), the files read model (task 16)
and the Merge lens (task 19). **None of it is built** on `feat/phase-3-git` (read at `9935278`: no
`converge`, `lanes` or `gitx` code), so every path is **UNVERIFIED until merge**. The Hub half
(two routes) is a Hub delta like task 46's, built after the sync-merge of `feat/hub`.

**The one thing this task must not get wrong is that the rule is enforced in `Land`, not in the
lens.** The desktop shows the requirement, but a request that reaches `POST …/lands` without a
valid approval is a `*Refusal` at a new step 9a, after the merge-tree check and before any object
is written, exactly like phase 3's other refusals. A UI that forgets to ask cannot land the change.

**The second thing is that an approval covers exactly what was reviewed.** It names the lane's
tip sha and the agent-config paths it saw. If the lane moves, or the land would change a path the
approval did not list, it no longer counts. The approver must be a current member of the project
other than the lane's owner; an admin gets no exemption, and the lane's own Orchestrator cannot
write one (the routes are desktop-only).

**What an agent-config path is** (`05-design.md:925`), at any depth, compared on the repo-relative
path with `/` separators, case-sensitively:

| Kind | Matches |
|---|---|
| instructions | `CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md` (any directory) |
| agent settings | anything under a `.claude/` directory; `.mcp.json` |
| git hooks | anything under `.husky/` or `.githooks/`; `lefthook.yml`, `lefthook.yaml`, `.lefthook.yml`, `.pre-commit-config.yaml`, `.simple-git-hooks.json` |
| package scripts | a `package.json` whose `scripts`, `husky`, `simple-git-hooks` or `lint-staged` member differs between base and lane tip, compared as canonical JSON |

A `package.json` change to `dependencies` alone is **not** an agent-config change; one to `scripts`
is, because npm runs lifecycle scripts on install (from knowledge). A file added, deleted or renamed
into or out of any of these counts, because rename detection is off (`--no-renames`). **The list is
the design's plus the git-hook tools the design's "git-hook files" names generically; the owner
confirms it** (gap 3).

**Files:**
- `control-plane/internal/converge/agentconfig.go` + test: `AgentConfigChange`, `AgentConfigChanges`
  (new)
- `control-plane/internal/converge/land.go` (phase 3 task 09): `LandRequest.Team`, step 9a (modify)
- `control-plane/internal/converge/teamreview.go` + test: `TeamReview`, `ReviewFor` (new)
- `control-plane/internal/api/lands.go` (phase 3 task 09's handler): the new refusal's row;
  `GET …/land-review` (modify / new)
- `control-plane/internal/hub/server/landapprovals.go` + `landapprovals_integration_test.go`,
  `store/landapprovals.go`, `migrate/sql/00NN_land_approvals.sql`: routes 84, 85 (new; Hub delta)
- `control-plane/internal/hub/hubapi/team.go` (task 03): `LandApproval`,
  `CreateLandApprovalRequest`, `PrefixLandApproval = "la"`, `CodeLandApprovalRequired` (modify)
- `control-plane/internal/hub/hubclient/*` (task 08): `CreateLandApproval`, `ListLandApprovals`
  and the fake's half (modify)
- `desktop/src/components/lens/merge/TeamLandReview.tsx` + test, and `LaneCard.tsx` (phase 3
  task 19): the conflicts list, the approval requirement, the approver's action (new / modify)
- `control-plane/internal/fleet/server/landdiff.go` + test: the approver's read of the diff (new),
  and `verbs.TreeSource.Diff` (task 22's interface, one method; modify)

#### Interfaces — complete

```go
// control-plane/internal/converge/agentconfig.go
package converge

// AgentConfigChange is one agent-config path the land would change.
type AgentConfigChange struct {
	Path   string `json:"path"`   // repo-relative, "/" separators
	Kind   string `json:"kind"`   // "instructions" | "agent-settings" | "git-hooks" | "package-scripts"
	Status string `json:"status"` // "A" | "M" | "D", from diff --name-status --no-renames
}

// AgentConfigChanges runs, in the project with Scope Project (task-02 §HGH),
//   git diff --name-status -z --no-renames --no-ext-diff --no-textconv <base> <tip>
// and, for every changed package.json, cat-file blob <base>:<path> and <tip>:<path>,
// comparing the four members as canonical JSON (encoding/json decode, re-encode with
// sorted keys). A package.json that does not parse on either side counts as a
// package-scripts change: unparseable is not proof of harmless. Sorted by Path.
// Pure over the gitx.Machine it is given; no write, no hook, no textconv.
func AgentConfigChanges(ctx context.Context, m gitx.Machine, projectPath, base, tip string) ([]AgentConfigChange, error)

// IsAgentConfigPath is the path half of the table, for paths alone (package.json
// is false here: only AgentConfigChanges can see its members).
func IsAgentConfigPath(p string) (kind string, ok bool)
```

```go
// control-plane/internal/converge/land.go (modify)

type LandRequest struct {
	// … phase 3 task 09's fields, unchanged …
	Team *TeamLand // nil for a solo project: every step below is skipped
}

// TeamLand is what a team project's land needs from the Hub. The API handler
// fills it from route 86; Land never calls the Hub, and converge imports no
// Hub package (Approval is its own four fields, copied by the handler).
type TeamLand struct {
	LaneOwner string // the lane agent's ownerUserId
	Approvals func(ctx context.Context) ([]Approval, error)
}

type Approval struct {
	State          string   // "requested" | "approved"
	LaneTip        string
	ApproverUserID string   // "" while requested
	Paths          []string
}

// Step 9a, after 9 (merge-tree clean) and before 10 (no object written yet):
//   changes = AgentConfigChanges(ProjectPath, old, laneTip)          (git_failed)
//   len(changes) > 0 and no approval a with
//       a.State == "approved", a.LaneTip == laneTip,
//       a.ApproverUserID != Team.LaneOwner, and every change's Path in a.Paths
//   → *Refusal{Reason: "second_approval_required",
//              Details: {"paths": detailList(paths), "laneTip": laneTip}}
//   Approvals erroring → *Refusal{Reason: "hub_unavailable"}: never "no approval needed".
```

`detailList` is phase 3's (`task-09.md:115-116`). The approval is looked up at land time, so an
approval written a second ago counts and one for an older tip never does.

#### The Hub routes — complete wire (a Hub delta, after `feat/hub`'s sync-merge)

Three routes after task 46's 81 to 83. Desktop only, no route name, so no delegation can list
them and no agent can request or give an approval. The Hub mints `la-…`.

**Route 84, `POST /v1/projects/{p}/land-approvals`**: the lane owner's desktop, role `member` or
`admin`. The lane owner asks.

```jsonc
{ "agentId": "ag-…",                   // a worker of the caller's (task 06's agents row), else 403 agent_not_owner
  "base": "<40|64 hex>", "laneTip": "<40|64 hex>",
  "paths": [ "CLAUDE.md", ".claude/settings.json", "package.json" ] }   // 1–50, each teamtext glob-valid, no wildcard
// 201
{ "id": "la-…", "projectId": "pj-…", "agentId": "ag-…", "ownerUserId": "u-…",
  "base": "…", "laneTip": "…", "paths": [ … ], "state": "requested",
  "approverUserId": null, "approvedAt": null, "createdAt": "…" }
```

**Route 85, `POST /v1/land-approvals/{landApprovalId}/approve`**: body `{}`, the desktop of a
**current member of the project other than `ownerUserId`**, role `member` or `admin`. The owner →
403 `forbidden`, `required: "second_member"`; a viewer → 403; a second approve → 409 `conflict`.
It sets `state: "approved"`, `approverUserId` from the token and `approvedAt`. There is no
reject: a reviewer who disagrees says so to the owner, and the land stays refused.

**Route 86, `GET /v1/projects/{p}/land-approvals?agentId=&state=`**: V (desktop) or Fleet·O
(`land_approvals.read`, a new orchestrator-class route name, so an Orchestrator can tell its
human that a land waits for a teammate). Newest first, at most 100.

Events, task 01's §8 shape: `land_approval.requested` (`{"agentId","pathCount"}`, feed `yes`, so
every member's strip can show *"Linus asks for a second look at 3 agent-config files"*) and
`land_approval.approved` (`{"agentId","approverUserId"}`, feed `yes`). Paths are in the record,
never in `data`. Record, id prefix `la-` and code `second_member` join task 03's `team.go`, the
contract's §7.0 team table, §3.1 and §8, and the Z5 matrix (task 43's test).

#### The review — complete source

```go
// control-plane/internal/converge/teamreview.go

// TeamReview is what the land review adds on a team project. Everything in it is
// a fact the owner can check: Hub records and git's own output.
type TeamReview struct {
	Conflicts   []ReviewConflict    `json:"conflicts"`   // every cf-… the lane's agent was party to
	AgentConfig []AgentConfigChange `json:"agentConfig"` // [] when none
	Approval    *ReviewApproval     `json:"approval"`    // nil when AgentConfig is empty
}

type ReviewConflict struct {
	ID         string   `json:"id"`         // cf-…
	Tier       string   `json:"tier"`       // P | H | M | S
	OtherOwner string   `json:"otherOwner"` // display name of the other agent's owner
	Paths      []string `json:"paths"`
	State      string   `json:"state"`
	Resolution string   `json:"resolution"` // "" when unresolved
}

type ReviewApproval struct {
	Status   string `json:"status"`             // "missing" | "requested" | "approved" | "stale"
	ID       string `json:"id,omitempty"`       // la-…
	Approver string `json:"approver,omitempty"` // display name
}

// ReviewFor builds the review for one lane: the lane's agents are those whose
// laneRef names the lane (task 06's agents list); conflicts are route 59's rows
// with agentA or agentB among them, every state, oldest first; AgentConfig is
// AgentConfigChanges(branch tip, lane tip); Approval is the newest route-86 row
// for the lane's agent: "stale" when its laneTip is not the lane's tip now or
// its paths do not cover AgentConfig.
func ReviewFor(ctx context.Context, src ReviewSources, projectPath, branchTip, laneTip, laneRef string) (TeamReview, error)
```

`GET /v1/projects/{projectId}/branches/{branchId}/lands/{worktreeId}/review` on the local control
plane answers `TeamReview` (every list `[]`, never `null`, phase 3's rule, `task-09.md:65-70`); for a
solo project, `404 not_a_team_project`, and the lens does not ask.
