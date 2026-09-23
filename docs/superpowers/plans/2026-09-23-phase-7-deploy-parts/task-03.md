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
	Branch    string `json:"branch"`
	Primary   bool   `json:"primary"`
	Subdomain string `json:"subdomain"`
	Status    string `json:"status"`
	Replicas  *struct {
		Current int `json:"current"`
		Desired int `json:"desired"`
	} `json:"replicas"`
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
| `Environments` | `GET P/environments` (1126) | `listDTO[environmentDTO]` | `Replicas` nil when the key is absent; `Host = Subdomain` |
| `Latest` | `GET P/deploys/latest` (1025) | bare `deploymentDTO` | **404 → `(Deployment{}, false, nil)`**; the watcher calls `Environments` first in the same cycle, so a missing *project* has already surfaced as its own error |
| `Deployments` | `GET P/deployments` (1024) | `listDTO[deploymentDTO]` | order kept as served (newest first, `projects.go:1532-1536`) |
| `Run` | `GET P/pipelines/{run}` (1290) | `dataDTO[pipelineRunDTO]` | 404 → `KindNotFound` (it is also Panacea's cross-project IDOR answer, `projects.go:3595-3601`) |
| `Scans` | `GET P/security-status?deployId=` (1043) | `dataDTO[securityStatusDTO]` | `deployId` is a run name (`handlers/security_status.go:211-214`) |
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

`deploytest.Fake.SameEnvironment` is exact equality only, and `Seeder` gains nothing: the Panacea
equivalence is tested here, against the connector, not through the suite.

#### The recorded bodies (`testdata/`)

Each file is a response body as the cited writer would produce it, hand-written from the source
line, with a first-line JSON comment key `"_source"` removed before serving. They are the ground
truth the mapping tests read.

| File | Route and state |
|---|---|
| `me-approved.json`, `me-pending.json`, `me-no-approved-field.json` | `/me` |
| `projects.json` | two projects, one with `repoUrl`, one without |
| `environments.json` | `production` (primary, `replicas` present), `feat-evidence-uplo` (no `replicas` key) |
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
