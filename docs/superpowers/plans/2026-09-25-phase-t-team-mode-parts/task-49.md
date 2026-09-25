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
  `store/landapprovals.go`, `migrate/sql/00NN_land_approvals.sql`: routes 84 to 86 (new; Hub delta)
- `control-plane/internal/hub/hubapi/team.go`, `keys.go`, `codes.go` (task 03): `LandApproval`,
  `CreateLandApprovalRequest`, `PrefixLandApproval = "la"`, `CodeLandApprovalNotFound`, the route
  name `land_approvals.read` in the orchestrator class set (modify)
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

#### The migration

```sql
-- control-plane/internal/hub/migrate/sql/00NN_land_approvals.sql  (after task 46's)
SET search_path = hub, pg_temp;
CREATE TABLE land_approvals (
  org_id           text NOT NULL,
  id               text NOT NULL CHECK (id ~ '^la-[0-9a-z]{16}$'),
  project_id       text NOT NULL,
  agent_id         text NOT NULL CHECK (agent_id ~ '^ag-'),
  owner_user_id    text NOT NULL,
  base_sha         text NOT NULL CHECK (base_sha ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
  lane_tip         text NOT NULL CHECK (lane_tip ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
  paths            text[] NOT NULL CHECK (cardinality(paths) BETWEEN 1 AND 50),
  approver_user_id text NULL CHECK (approver_user_id IS NULL OR approver_user_id <> owner_user_id),
  approved_at      timestamptz NULL CHECK ((approver_user_id IS NULL) = (approved_at IS NULL)),
  created_at       timestamptz NOT NULL,
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id)
);
ALTER TABLE land_approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY land_approvals_org ON land_approvals
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON land_approvals TO hub_app;
GRANT UPDATE (approver_user_id, approved_at) ON land_approvals TO hub_app;
```

The `CHECK` that the approver is not the owner is the database's copy of route 85's rule, so a
handler bug still cannot write Alice's approval of Alice. The id pattern, the composite foreign
key and the RLS policy follow the Hub's own tables (Hub task 07a/07b, review I5; **UNVERIFIED**
until merge that `projects` is keyed `(org_id, id)` and that ids are 16 lowercase base-36
characters: take both from the merged `0001_init.sql`).

#### The approver's diff

An approver must see what she approves. In TM1 the lane lives only on its owner's machine, so the
approver's desktop reads the diff **through Fleet**, which reads it from the owner's machine the
way task 22's `TreeSource` reads a tree:

```go
// control-plane/internal/fleet/verbs (task 22's interface, one method added)
type TreeSource interface {
	Tree(ctx context.Context, owner model.Owner, project string) (Tree, error)
	Show(ctx context.Context, owner model.Owner, project, path string) ([]byte, error)
	// Diff is `git diff --no-renames --no-ext-diff --no-textconv <base> <tip> -- <paths>`
	// on the owner's machine, Scope Project, at most MaxDiffBytes (cut on a line
	// boundary, Truncated set). ErrTreeUnavailable when the machine cannot be reached.
	Diff(ctx context.Context, owner model.Owner, project, base, tip string, paths []string) (Diff, error)
}

type Diff struct {
	Text      string `json:"text"`   // shown as monospace text, never rendered as markdown
	Truncated bool   `json:"truncated"`
}

const MaxDiffBytes = 128 << 10
```

**Fleet route `GET /v1/projects/{p}/land-approvals/{la}/diff`**: a member's desktop token (Entra
per request, task 12), never an agent. Fleet reads `la-…` from the Hub (route 86), then calls
`Diff(owner, p, base, laneTip, paths)`. `200 Diff`, or `503 unavailable` with *"<Owner>'s cloud
machine is asleep. Ask them to open the project."* A diff that fails `teamtext.Clean` is still
shown (it is repository content, `Source: "repo"`, task 22's class), but in a pane that renders
C0/C1 and bidi controls as visible escapes. Because task 22 ships `unavailableTree` in TM1-α and
task 34 brings the real source, this route answers 503 until task 34 lands (gap 2).

**The lens** (`TeamLandReview.tsx`, under each queued row of a team project in task 19's
`LaneCard`): *CONFLICTS THIS LANE WAS PART OF* (one line each: tier, `cf-…` in Commit Mono, the
other owner, the paths, the resolution or *"unresolved"*), then, when `agentConfig` is not empty,
*AGENT-CONFIG CHANGES NEED A SECOND MEMBER* with one line per path and its kind, and one action:
**Ask for a second look** (route 84) or, once asked, *"Waiting for a teammate"*; on another
member's desktop the same row shows **Review and approve** (the diff, then route 85). The Land
button stays enabled: the server's refusal names the reason, and task 19's banner shows it.

#### Tests to write first

The first three are the scope paragraph's (`2026-09-25-phase-t-team-mode.md:947-949`). The
`converge` tests run on phase 3's `gitxtest` repositories; the Hub tests are integration tests on
task 07's `team2World`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestALaneTouchingClaudeSettingsCannotLandOnItsOwnersApproval` | a team lane changing `.claude/settings.json`: no approval → 409 `second_approval_required`, `details.paths = ".claude/settings.json"`, the branch ref and the object count unchanged; an approval **by the owner** cannot exist (route 85 → 403 `second_member`; a direct `INSERT` with approver = owner fails the `CHECK`); Grace's approval → the land succeeds | the rule enforced only in the lens; the owner approving herself |
| `TestDependenciesOnlyNeedsNoSecondApprover` | `package.json` with only `dependencies` changed → `AgentConfigChanges` is `[]`, the land needs no approval | every `package.json` change treated as agent config, so the rule is ignored in practice |
| `TestScriptsChangeNeedsASecondApprover` | `package.json` with `scripts.postinstall` added → one change, kind `package-scripts`; `husky` or `lint-staged` changed → the same; key order changed only → `[]` (canonical JSON) | a text diff of `package.json`; a member missed |
| `TestAnUnparseablePackageJSONCounts` | a `package.json` that is not JSON on the tip → one `package-scripts` change | unparseable taken as harmless |
| `TestNestedAndHookPathsCount` | `docs/CLAUDE.md`, `pkg/.claude/commands/x.md`, `.husky/pre-commit`, `.mcp.json` added, `AGENTS.md` deleted → five changes with the right kinds; `docs/claude.md` (lower case) → none | root-only matching; deletions ignored |
| `TestARenameOutOfClaudeCounts` | `git mv .claude/settings.json settings.bak` → a `D` for `.claude/settings.json` | rename detection on, so the change reads as a rename to a harmless path |
| `TestAnApprovalForAnOlderTipDoesNotCount` | approve at tip T1; the lane moves to T2 → 409 `second_approval_required`; the review says `stale` | an approval bound to the lane, not to what was reviewed |
| `TestAnApprovalMustCoverEveryPath` | approved paths `[CLAUDE.md]`; the land also changes `.mcp.json` → refused, `details.paths = ".mcp.json"` | "any approval" instead of coverage |
| `TestHubDownRefusesRatherThanSkips` | `Approvals` errors → 409 `hub_unavailable`, nothing written | a Hub error read as "no approvals needed" |
| `TestSoloProjectsAreUnchanged` | `Team == nil` → phase 3's own land tests pass byte-identical, and `AgentConfigChanges` is never called (the fake machine saw no `diff --name-status`) | the rule leaking into solo projects |
| `TestTheReviewListsEveryConflictWithItsResolution` | two `cf-…` rows naming the lane's agent (one `resolved`/`split`, one `raised`) and one naming another agent → the review lists the first two, oldest first, `resolution` `"split"` and `""` | conflicts filtered to open ones, so the resolved one the owner should see vanishes |
| `TestOnlyTheOwnerMayAskAndOnlyAnotherMemberMayApprove` | route 84 by Grace for Linus's agent → 403 `agent_not_owner`; by Fleet → 403 `required: "desktop"`; route 85 by a viewer → 403; by Grace → 200; again → 409 | Fleet or a viewer granting approvals |
| `TestNoAgentCanReachTheApprovalRoutes` | a delegation listing `land_approvals.approve` → 422 at create (no such route name); route 85 with `X-Panacea-Delegation` → 403 | an Orchestrator approving its teammate's lane on a persuasive message |
| `TestTheApproverSeesTheDiffOrAClearUnavailable` (Fleet) | a fake `TreeSource` with a diff → 200 with the text; `ErrTreeUnavailable` → 503 with the sentence; an agent token → 403 | the approval UI offering "approve" with nothing to look at |

#### Steps

- [ ] **Step 1: Read after the merges.** Phase 3 tasks 09, 16 and 19 as built (`Lander.Land`'s
  steps, the lands handler, `LaneCard`), the Hub's `0001_init.sql` (id shape, `projects` key),
  task 22's `TreeSource` as built.
- [ ] **Step 2: Classifier.** The six `converge` path and `package.json` tests; they fail;
  `agentconfig.go`; they pass.
- [ ] **Step 3: Land.** The land tests; they fail; `TeamLand`, step 9a, the handler's `Team`
  fill (from routes 55 and 86 via `hubclient`), the refusal row; they pass, and phase 3's land
  suite passes unchanged.
- [ ] **Step 4: Hub.** Migration, routes 84 to 86, events, codes, Z5 rows; the Hub tests pass.
- [ ] **Step 5: Review and diff.** `ReviewFor`, the review route, `TreeSource.Diff`, the Fleet
  route; their tests pass.
- [ ] **Step 6: Lens.** `TeamLandReview.tsx` and its test (renders both sections from a fixture;
  no approve button on the owner's own lane); `npm run typecheck` and `npm test` pass.
- [ ] **Step 7: Commit** each group with `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
cd control-plane
go test ./internal/converge/ -run 'TestALaneTouching|TestDependenciesOnly|TestScriptsChange|TestAnUnparseable|TestNestedAndHook|TestARenameOut|TestAnApprovalFor|TestAnApprovalMust|TestHubDown|TestSoloProjects|TestTheReviewLists' -count=1
# ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/converge	…
go test -tags integration ./internal/hub/server/ -run 'TestOnlyTheOwnerMayAsk|TestNoAgentCanReach' -count=1
go test ./internal/fleet/server/ -run TestTheApproverSeesTheDiff -count=1 && go vet ./...
cd ../desktop && npm run typecheck && npm test -- TeamLandReview
```

The two-person run (Linus's lane changes `CLAUDE.md`, Linus's land is refused, Grace reviews the
diff on her desktop and approves, Linus lands) is **deferred to task 57**, because the diff needs
task 34's real tree source on the real environment.

#### What this task does not claim

- **Not the TM2 queue.** From TM2 the land queue moves to Fleet over the lane mirror (§8.3,
  tasks 59–61); that queue must call the same `AgentConfigChanges` and step 9a. Task 61's part
  should say so.
- **Not that the list is complete.** Any file an agent reads as instructions or runs as a hook is
  agent config in spirit; a tool this table does not name (a `Makefile` target a hook calls, a
  `.vscode/tasks.json`) is not caught. The managed settings (OD-23) are the stronger wall.
- **Not a review of the code itself.** A second approver is required for agent-config paths only;
  every other change is its owner's reviewed act, as in phase 3.
- **Not rejection or comments.** There is no reject route; discussion happens between people.

#### Gaps found in the skeleton

1. **No approval record in task 01.** Its §7.0 team table has no land-approval route; this part
   adds routes **84 to 86**, after task 46's 81 to 83. The lower task id keeps a number on any
   clash, and this part renumbers mechanically.
2. **The approver needs to see the diff**, and in TM1 the lane exists only on its owner's machine.
   This part adds `TreeSource.Diff` to task 22's interface and a Fleet desktop route; both depend on
   task 34 adding a real read of the owner's machine (task 22 ships `unavailableTree` in TM1-α).
   Task 34's part should include `diff` beside its `ls-tree` / `cat-file`.
3. **The path list** extends the design's "git-hook files" and "package-script entries" with named
   tools and four `package.json` members; the owner confirms.
4. **Phase 3's lens is desktop-per-project.** A teammate approving another's lane needs that lane in
   her lens; this part shows it as a row on the team project's merge lens, fed by the
   `land_approval.requested` feed kind, not by her own queue.
