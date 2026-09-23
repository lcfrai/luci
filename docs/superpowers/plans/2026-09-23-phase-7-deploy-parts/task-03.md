### Task 03: Panacea reads — the DTOs, the mapping, and a recorded-body server

**Half:** 7a. **Hours:** 3. **Depends on:** 02.

Every read the watcher and the drill-ins make, as `deploy.Identity` and `deploy.Reader` methods on
`*panacea.Client`. Each DTO declares **only the fields phase 7 reads**, and names the Panacea line it
mirrors, so a reviewer can re-check it against `b33d15acb` in one open. No Panacea source is copied
(`OD-13`); these are re-declarations of a wire format.

**Files:**
- `control-plane/internal/deploy/panacea/wire.go` (new) — the read DTOs below
- `control-plane/internal/deploy/panacea/reads.go` (new) — the methods and the mapping
- `control-plane/internal/deploy/panacea/panaceatest/server.go` (new) — a fake `panacea-api` over `httptest` that implements `deploytest.Seeder`
- `control-plane/internal/deploy/panacea/testdata/*.json` (new) — one body per route and state, listed below
- `control-plane/internal/deploy/panacea/reads_test.go` (new)
- `control-plane/internal/deploy/panacea/conformance_test.go` (new) — `deploytest.Conformance` over the connector
- `control-plane/internal/deploy/deploytest/conformance.go` (modified) — the rows below

#### Wire DTOs — complete (`wire.go`, read half)

```go
package panacea

// Envelopes. Panacea uses three, per route (plan §4): bare, list and data.

// listDTO mirrors models.ListResponse (models/models.go:2153).
type listDTO[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
}

// dataDTO mirrors models.APIResponse (models/models.go:2147-2151).
type dataDTO[T any] struct {
	Data    T      `json:"data"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// meDTO mirrors handlers/me.go:48-59. Approved is a pointer: me.go:37-38 says the signal is
// read STRICTLY (approved === true) and a partial response must read as NOT approved.
type meDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"isAdmin"`
	Role     string `json:"role"`
	Approved *bool  `json:"approved"`
}

// projectDTO mirrors the fields of models.Project (models/models.go:563-625) phase 7 reads.
type projectDTO struct {
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	RepoURL       string `json:"repoUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

// environmentDTO mirrors one row of ListEnvironments (handlers/projects.go:2116-2148).
// replicas is present ONLY when the namespace was read (:2125-2140).
type environmentDTO struct {
	Name      string `json:"name"`
	Branch    string `json:"branch"` // a LABEL: == Name for every non-base row (:2109-2111); not the git branch
	Primary   bool   `json:"primary"`
	Subdomain string `json:"subdomain"`
	Status    string `json:"status"`
	Replicas  *struct {
		Current int `json:"current"`
		Desired int `json:"desired"`
	} `json:"replicas"`
}

// branchEnvDTO is the ONE field EnvironmentFor reads from GetBranchAccessMode's data
// (handlers/project_security.go:668-677): Panacea's own environment name for any branch.
type branchEnvDTO struct {
	Env string `json:"env"`
}

// deploymentDTO mirrors models.Deployment (models/models.go:665-770). BuildLogs is NOT declared:
// it can be large and phase 7 never shows it from this field.
type deploymentDTO struct {
	ID                string                 `json:"id"`
	Status            string                 `json:"status"`
	Image             string                 `json:"image"`
	Version           string                 `json:"version"`
	Commit            string                 `json:"commit"`
	Branch            string                 `json:"branch"`
	Environment       string                 `json:"environment"`
	FailedStep        string                 `json:"failedStep"`
	ErrorMessage      string                 `json:"errorMessage"`
	CreatedAt         time.Time              `json:"createdAt"`
	Provisioning      []provisioningDTO      `json:"provisioning"`
	RollbackWarmth    string                 `json:"rollbackWarmth"`
	ColdStartSeconds  int                    `json:"coldStartSeconds"`
	LiveReadyReplicas *int                   `json:"liveReadyReplicas"` // models.go:723 — the pointer is the point
	ScanStatus        map[string]scanGateDTO `json:"scanStatus"`        // nil = no gate ran
	DeployedBy        string                 `json:"deployedBy"`
	Trigger           string                 `json:"trigger"`
	Backfilled        bool                   `json:"backfilled"`
}

// provisioningDTO mirrors models.ProvisioningStatus (models/models.go:794-804).
type provisioningDTO struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

// scanGateDTO mirrors models.ScanGate (models/models.go:782-787).
type scanGateDTO struct {
	Status    string `json:"status"`
	Criticals *int   `json:"criticals"`
	Highs     *int   `json:"highs"`
	Findings  *int   `json:"findings"`
}

// pipelineRunDTO mirrors models.PipelineRunStatus (models/models.go:2385-2410).
type pipelineRunDTO struct {
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	StartTime       string   `json:"startTime"`
	FailedStep      string   `json:"failedStep"`
	ErrorMessage    string   `json:"errorMessage"`
	CompletedSteps  []string `json:"completedSteps"`
	CurrentStep     string   `json:"currentStep"`
	IgnoredFailures []string `json:"ignoredFailures"`
}

// securityStatusDTO mirrors models.SecurityStatus (models/security.go:205-242).
type securityStatusDTO struct {
	Overall         string          `json:"overall"`
	Blocking        []scanNoteDTO   `json:"blocking"`
	Incomplete      []scanNoteDTO   `json:"incomplete"`
	Scans           []scanResultDTO `json:"scans"`
	LastUpdated     string          `json:"lastUpdated"`
	DeployID        string          `json:"deployId"`
	DetailsRetained *bool           `json:"detailsRetained"`
	DetailsSource   string          `json:"detailsSource"`
}

// scanNoteDTO mirrors models.BlockingItem (models/security.go:199-202).
type scanNoteDTO struct {
	Scan   string `json:"scan"`
	Reason string `json:"reason"`
}

// scanResultDTO mirrors models.ScanResult (models/security.go:117-197).
type scanResultDTO struct {
	Scan           string         `json:"scan"`
	Status         string         `json:"status"`
	Mode           string         `json:"mode"`
	Counts         map[string]int `json:"counts"`
	Summary        string         `json:"summary"`
	IsBlocking     bool           `json:"isBlocking"`
	LastScanTime   string         `json:"lastScanTime"`
	SkipReason     string         `json:"skipReason"`
	ErrorReason    string         `json:"errorReason"`
	FindingsSource string         `json:"findingsSource"`
}

// podDTO mirrors clients.PodStatus (clients/pod_introspection.go:47-61).
type podDTO struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	Ready    bool   `json:"ready"`
	Restarts int    `json:"restarts"`
	Reason   string `json:"reason"`
	Message  string `json:"message"`
}

// eventDTO mirrors clients.NamespaceEvent (clients/pod_introspection.go:95-104).
type eventDTO struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Object  string `json:"object"`
	Message string `json:"message"`
	Count   int32  `json:"count"`
}

// diagnoseDTO mirrors handlers.DiagnoseResponse (handlers/diagnose.go:70-79).
type diagnoseDTO struct {
	Health   string       `json:"health"`
	Summary  string       `json:"summary"`
	Findings []findingDTO `json:"findings"`
	Source   string       `json:"source"`
}

// findingDTO mirrors handlers.DiagnoseFinding (handlers/diagnose.go:60-67).
type findingDTO struct {
	Problem      string   `json:"problem"`
	LikelyCause  string   `json:"likelyCause"`
	SuggestedFix string   `json:"suggestedFix"`
	Evidence     []string `json:"evidence"`
	Severity     string   `json:"severity"`
	Owner        string   `json:"owner"`
}
```

#### The routes and their envelopes (`reads.go`)

`P` is `/api/v1/projects/` + `url.PathEscape(string(p))`.

| Method | Route (`routes.go`) | Envelope | Special case |
|---|---|---|---|
| `Whoami` | `GET /api/v1/me` (3038) | bare `meDTO` | `Approved: dto.Approved != nil && *dto.Approved` |
| `Projects` | `GET /api/v1/projects` (1018) | `listDTO[projectDTO]` | — |
| `Environments` | `GET P/environments` (1126) | `listDTO[environmentDTO]` | `Replicas` nil when the key is absent; `Host = Subdomain`; `Branch` is the row's label verbatim — `Env` for every non-base row (`handlers/projects.go:2109-2118`), so never a join key |
| `EnvironmentFor` | `GET P/branches/{branch}/access` (1430), the branch `url.PathEscape`d | `dataDTO[branchEnvDTO]`, where `branchEnvDTO` is `{Env string \`json:"env"\`}` only | returns `data.env` verbatim. Panacea computes it with `branchEnv` → `mapBranchToEnvWithProd` (`handlers/branch_security.go:29-37`, called at `project_security.go:646`, written at `:668-677`), **with no existence check** — so it answers for a branch never deployed. **It is the access handler's mapping, not the deploy's (b02 round 2, R1).** `branchEnv` reads the production branch from the raw `production-branch` LABEL only (`branch_security.go:32`). The deploy resolves it with `ResolveProjectProductionBranch` — label → `primary-branch` annotation → `"main"`, skipping a label whose value is `prod`/`production` (`clients/pipeline.go:233-237`, `clients/kubernetes.go:10179-10204`), and so does the environments list (`kubernetes.go:5189`). So for a project whose production branch is annotation-only or mislabelled, `data.env` disagrees with where the deploy lands: annotation `primary-branch=develop` with no label answers `develop` for `develop` (deployed to `prod`) and `prod` for `main` (deployed to `main`). Phase 7 does not copy the resolver to correct it; it detects the disagreement from the Deployment the run wrote (task 08, rule 5a) and lives with the defect (plan §12). A **404** on this route is rewritten, keeping `KindNotFound`, to the message *"Panacea could not read this branch name. It may contain `/`, which the route cannot carry."* `Environments` has read the project in the same cycle and the route is `authedProject`, so a 404 here is the router's `404 page not found` for a path with an extra segment, never *no such project* (R2). `data.env == ""` or a missing `data` → `KindUnavailable` (Panacea never writes one: `mapBranchToEnv` and `sanitizeBranchEnv` never return `""`, `projects.go:3783-3826`). Its 503 *"kubernetes client unavailable"* (`:641-644`) is the ordinary 5xx rule. Task 04's `Posture` reads the same route into its own `postureDTO`; the two share the route, not a struct |
| `Latest` | `GET P/deploys/latest` (1025) | bare `deploymentDTO` | **404 → `(Deployment{}, false, nil)`**; the watcher calls `Environments` first in the same cycle, so a missing *project* has already surfaced as its own error |
| `Deployments` | `GET P/deployments` (1024) | `listDTO[deploymentDTO]` | order kept as served (newest first, `projects.go:1532-1536`) |
| `Run` | `GET P/pipelines/{run}` (1290) | `dataDTO[pipelineRunDTO]` | 404 → `KindNotFound` (it is also Panacea's cross-project IDOR answer, `projects.go:3595-3601`) |
| `Scans` | `GET P/security-status?env=&deployId=` (1043) | `dataDTO[securityStatusDTO]` | `deployId` is a run name (`handlers/security_status.go:211-214`). **`env` is the deployment's `Environment`, set with `url.Values` whenever it is non-empty** and omitted otherwise, exactly as the portal builds it (`backstage/packages/app/src/plugins/SecurityScanReview.tsx:437-440`). Without it `resolveEnv` reads `"production"` (`security_status.go:192-201`), `collectScans` lists the base namespace (`:232`, `tier_store.go:106-113`) and keeps only ConfigMaps labelled with that env (`:526-534`), so a branch release would show production's verdicts, or `NOT_RUN` rows, as its own (review C3). `"prod"` needs no special case: Panacea maps it to the same namespace |
| `Pods` | `GET P/environments/{env}/pods` (1148) | `listDTO[podDTO]` | — |
| `Events` | `GET P/environments/{env}/events` (1150) | `listDTO[eventDTO]` | `Time` parsed as RFC 3339; unparseable → zero |
| `Diagnosis` | `GET P/environments/{env}/diagnose` (1151) | `dataDTO[diagnoseDTO]` | — |

**The status mapping** — `Deployment.Status` comes from `derivePipelineStatus`
(`clients/pipeline.go:2073-2084`: `True`→`"Succeeded"`, `False`→`"Failed"`, else `"Pending"`),
from `"Unknown"` when a run has no condition yet (`clients/kubernetes.go:5662`), and from
`"historical"` for a reconstructed row (`clients/deployment_backfill.go:173`).
`PipelineRunStatus.Status` is documented as *"Unknown, Running, Succeeded, Failed"*
(`models/models.go:2390`). One table serves both:

| Panacea word | `deploy.BuildState` |
|---|---|
| `Succeeded` | `BuildSucceeded` |
| `Failed` | `BuildFailed` |
| `Pending`, `Running` | `BuildRunning` |
| `Unknown` | `BuildPending` |
| `historical` | `BuildHistorical` |
| anything else | `BuildUnknown`, with `BuildRaw` kept |

**The environment-name equivalence** (plan 7P-8) lives here and nowhere else:

```go
// SameEnvironment implements deploy.Reader.SameEnvironment: exact equality plus the ONE
// equivalence Panacea itself applies — tenantNamespace treats "prod" and "production" as the
// base namespace (clients/kubernetes.go:4210-4217). deploywatch calls it through the interface,
// so the orchestration layer never imports this package.
func (c *Client) SameEnvironment(a, b string) bool
```

`deploytest.Fake.SameEnvironment` is exact equality only, and `Seeder` gains nothing for it: the
Panacea equivalence is tested here, against the connector, not through the suite. (`Seeder` does
gain `BranchEnvironment`, for `EnvironmentFor` — task 01, review b02.)

#### The recorded bodies (`testdata/`)

Each file is a response body as the cited writer would produce it, hand-written from the source
line, with a first-line JSON comment key `"_source"` removed before serving. They are the ground
truth the mapping tests read.

| File | Route and state |
|---|---|
| `me-approved.json`, `me-pending.json`, `me-no-approved-field.json` | `/me` |
| `projects.json` | two projects, one with `repoUrl`, one without |
| `environments.json` | `production` (primary, `branch: "main"`, `replicas` present), and `feat-evidence-upload` with **`branch: "feat-evidence-upload"`** — Panacea's `Env == Branch` for a non-base row (`projects.go:2109-2118`), never `feat/evidence-upload` — and no `replicas` key |
| `branch-access-feat-x.json`, `branch-access-long.json`, `branch-access-main.json` | `branches/{branch}/access` (`project_security.go:668-677`) for `feat/x` → `env: "feat-x"`; `lane/7-deploy-connector-watcher` (31 characters) → `env: "lane-7-deploy-connec"` (`sanitizeBranchEnv`'s 20-character cut, `projects.go:3818-3820`); `main` → `env: "prod"` (`mapBranchToEnv`, `projects.go:3785-3786`). Each `_source` line names the handler line; the values are Panacea's rule applied by hand to write the fixture, and appear nowhere in production code |
| `branch-access-annotation-only.json` | the same route for `develop` on a project whose production branch is the annotation `primary-branch=develop` with no label → `env: "develop"` (`branch_security.go:32` reads the label only; the deploy would land `develop` in `prod`, `pipeline.go:233-237`). Pins the divergence (b02 round 2, R1) |
| `latest.json`, `latest-404.json` | `deploys/latest`; the 404 is `{"error":"no deployments yet for project engagement-portal"}` (`projects.go:1539`) |
| `deployments.json` | four rows: a `Succeeded` newest with `liveReadyReplicas: 0` and a `scanStatus`; a `Failed` with `failedStep`; a `historical` backfilled row; one with an empty `version` |
| `run-running.json`, `run-failed.json` | `pipelines/{run}` |
| `security-status.json` | `FAIL` with one blocking and one incomplete, `detailsSource: "unverified"` |
| `pods.json`, `events.json`, `diagnose-ai.json` | drill-ins; the diagnosis has `source: "rules+ai"` |
| `error-text-plain-403.txt` | `{"error":"no access yet: your account is waiting for team approval — finish onboarding first"}` served as `text/plain` (`rbac.go:398-400`) |

#### `panaceatest.Server` — the harness for the conformance suite

```go
package panaceatest

// Server is a fake panacea-api: an httptest.Server that holds a model and answers the routes of
// plan §4 in Panacea's exact envelopes, rendered from the same DTO shapes. It implements
// deploytest.Seeder, so deploytest.Conformance runs unchanged against the real connector.
type Server struct{ /* unexported */ }

func New(t *testing.T) *Server             // closes with t.Cleanup
func (s *Server) URL() string              // the https origin
func (s *Server) HTTPClient() *http.Client // trusts the server's certificate
func (s *Server) Token() string            // the only token it accepts: "pnc_test"
func (s *Server) Requests() []string       // "METHOD /path" in order

var _ deploytest.Seeder = (*Server)(nil)
```

`Server.BranchEnvironment` fills a table, and `GET P/branches/{branch}/access` answers `data.env`
from it after `url.PathUnescape` of the segment. The server holds **no** naming rule. The route
has **two** seeds, because task 04's `Posture` reads the same body (b02 round 2, R3): the server
answers from both tables — `env` from `BranchEnvironment`, and the posture fields from task 04's
posture seed — and **omits `env`** when only a posture is seeded. A branch seeded in **neither**
is a test bug, answered `500` with `t.Errorf("panaceatest: branch %q seeded in neither table")`,
where real Panacea would answer for any branch. So conformance row `UnknownPostureFailsClosed`
(task 01), which seeds only a posture word, still runs over the connector, and no fixture ever
matches a branch to a row by name. `EnvironmentFor` on a posture-only branch reads the missing
`env` as `KindUnavailable` (the empty-answer rule above).

`Server.Scans` fills a table keyed by `(env, deployId)`, and `GET P/security-status` answers from
it the way Panacea resolves the env (review C3): a request **without `env`**, or with `prod` or
`production`, answers the report seeded under `prod` — `resolveEnv` defaults to `"production"`
(`handlers/security_status.go:192-201`) and `tenantNamespaceForEnv` sends all three to the base
namespace (`handlers/tier_store.go:106-113`). Any other `env` is looked up exactly. This is the one
place the server copies a Panacea rule, because it is the rule the C3 mutant hides behind: a
connector that drops `env` is answered with production's report, as live Panacea would answer it.
A pair seeded in neither form is a test bug, answered `500` with
`t.Errorf("panaceatest: no scans seeded for env %q, deploy %q")`.

#### Conformance rows this task adds (`deploytest/conformance.go`)

| Row | Assertion | Mutant it kills |
|---|---|---|
| `ProvisioningSurvives` | a seeded deployment with one `failed` database provisioning reads back with it | dropping `provisioning[]`, so *"app deployed, database failed"* reads green |
| `EmptyVersionIsUnknown` | `Version: ""` reads back `""`, never `Commit[:7]` | fabricating a version the target did not report |
| `ReplicasAbsentIsNil` | an environment seeded with `Replicas: nil` reads back `nil` | rendering an unread namespace as `0/0 pods` (`projects.go:2125-2137`) |

#### Tests to write first (`reads_test.go`)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestWhoamiApprovedIsStrict` | the three `/me` bodies → `Approved` true, false, **false** | reading a missing field as approved — the pending user then sees a full lens and every act 403s (`me.go:25-36`) |
| `TestLatest404IsNotAnError` | `latest-404.json` → `ok == false`, `err == nil` | a red error banner on a project that has simply never shipped |
| `TestLiveReadyPointer` | the newest row's `liveReadyReplicas: 0` → `*LiveReady == 0`; a row without the key → `nil` | `LiveReadyReplicas int` in the DTO |
| `TestGatesAbsentIsNil` | a row without `scanStatus` → `Gates == nil` | `make(map…)` in the mapper |
| `TestStatusMappingTable` | every row of the status table, both DTOs | mapping `Pending` to `BuildPending`, which leaves the watcher's 3 s cadence on for the whole build |
| `TestSameEnvironment` | `("prod","production")`, `("production","prod")` → true; `("prod","prod-eu")`, `("feat-a","feat-b")` → false; the test's comment quotes `clients/kubernetes.go:4213` | a prefix match that joins `production` to `production-eu` |
| `TestPathEscapesBranchAndEnv` | `Pods(p, "feat/x")` requests `/environments/feat%2Fx/pods` | an unescaped `/` routing to a different Panacea handler |
| `TestEnvironmentForReadsPanaceasName` | the three `branch-access-*.json` bodies: `EnvironmentFor(p, "feat/x")` requests `GET /api/v1/projects/engagement-portal/branches/feat%2Fx/access` and returns `"feat-x"`; the 31-character branch returns `"lane-7-deploy-connec"`; `main` returns `"prod"`; a body with `"env": ""` → `KindUnavailable`; and **no** request to `/environments` is made | returning the branch unchanged, or a fourth `sanitizeBranchEnv` in the connector (review C2) |
| `TestEnvironmentForRouteNotFound` | the route answering Go's router body `404 page not found` → `KindNotFound` with the message *"Panacea could not read this branch name. It may contain `/`, which the route cannot carry."*, never the router's text (b02 round 2, R2) | a refusal that tells the user their project does not exist |
| `TestEnvironmentForIsTheAccessRoutesMapping` | recorded body `branch-access-annotation-only.json` — the access route for `develop` on a project with annotation `primary-branch=develop` and no `production-branch` label, where `branchEnv` passes `prodBranch ""` (`branch_security.go:32`) and `mapBranchToEnv("develop")` answers `develop` — → `EnvironmentFor(p, "develop") == "develop"`, **not** `prod`, although the deploy lands that branch in `prod` (`pipeline.go:233-237`). The test's comment names the divergence and Q-P7-6 (b02 round 2, R1) | a connector that "fixes" Panacea's answer with a copy of `ResolveProductionBranch`, or a test suite that forgets the answer can be wrong |
| `TestEnvironmentForJoinsProduction` | `SameEnvironment(EnvironmentFor(p,"main"), "production")` is true against `environments.json` — the prod row joins | an exact-name join that leaves production's branch with no card (review I8's twin) |
| `TestScansSendsTheEnvironment` | `Scans(p, "feat-x", "d-feat")` requests `GET /api/v1/projects/engagement-portal/security-status?deployId=d-feat&env=feat-x` (`url.Values.Encode` order); `Scans(p, "prod", "d1")` sends `env=prod`; `Scans(p, "", "d1")` sends `?deployId=d1` with **no** `env` key, never `env=` (review C3) | dropping `env`, which shows production's verdict as a branch release's; or sending `env=` empty, which Panacea also reads as production |
| `TestTextPlain403` | `error-text-plain-403.txt` → `KindForbidden` with the full sentence | a decoder gated on `Content-Type: application/json` that loses Panacea's most actionable sentence |
| `TestConformanceOverConnector` | `deploytest.Conformance(t, panaceaHarness)` | the connector and the fake disagreeing about any row |

#### Steps

- [ ] Write the testdata bodies first, each from its cited source line; open the line while writing.
- [ ] Write `reads_test.go` and `conformance_test.go`; run; they fail to compile.
- [ ] Write `wire.go` (read half), `reads.go`, then `panaceatest/server.go`.
- [ ] Add the three conformance rows; run the whole package and `deploytest` green.
- [ ] Commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/... -race -count=1
→ ok for deploy, deploy/deploytest, deploy/panacea, deploy/panacea/panaceatest

grep -c '// mirrors' control-plane/internal/deploy/panacea/wire.go
→ at least 17 (one per DTO above)
```

#### What this task does NOT claim

- That the recorded bodies match live Panacea. They match Panacea's **source** at `b33d15acb`. Task
  20 fetches each route once with the owner's token (OD-12) and diffs the keys.
- That `GET /api/v1/projects` returns `repoUrl` to a non-admin token (plan §12). If it does not,
  task 06's fallback applies.
- Acts. Task 04.
