package panacea

import "time"

// The wire DTOs: Panacea's JSON as phase 7 reads it. Each declares ONLY the fields phase 7 reads
// and names the Panacea line it mirrors, at 406d0da95 (the plan's commit; every cited line is
// unchanged at b02424691 and at 7e771df25). No Panacea source is copied (OD-13): these are
// re-declarations of a wire format. Task 04 adds the write half below the read half.
//
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

// meDTO mirrors handlers/me.go:48-59. Approved is a pointer: me.go:36-37 says the signal is
// read STRICTLY (approved === true) and a partial response must read as NOT approved.
type meDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"isAdmin"`
	Role     string `json:"role"`
	Approved *bool  `json:"approved"`
}

// projectDTO mirrors the fields of models.Project (models/models.go:563-623) phase 7 reads.
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

// branchEnvDTO mirrors the ONE field EnvironmentFor reads from GetBranchAccessMode's data
// (handlers/project_security.go:668-677): Panacea's own environment name for any branch.
type branchEnvDTO struct {
	Env string `json:"env"`
}

// deploymentDTO mirrors models.Deployment (models/models.go:665-770). BuildLogs is NOT declared:
// it can be large and phase 7 never shows it from this field.
type deploymentDTO struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	Image        string    `json:"image"`
	Version      string    `json:"version"`
	Commit       string    `json:"commit"`
	Branch       string    `json:"branch"`
	Environment  string    `json:"environment"`
	FailedStep   string    `json:"failedStep"`
	ErrorMessage string    `json:"errorMessage"`
	CreatedAt    time.Time `json:"createdAt"`
	// Duration is Go's time.Duration.String() of completionTime − creationTimestamp, rounded to a
	// second ("4m12s"), and "" while the run has no completionTime (clients/kubernetes.go:5683-5689;
	// kept by the durable store, clients/deployment_store.go:83; absent on a backfilled row,
	// clients/deployment_backfill.go:167-175). The mapper sets Deployment.CompletedAt =
	// CreatedAt + time.ParseDuration(Duration); "" or an unparseable value → zero, never a guess
	// (review I3). Exact only when CreatedAt is the PipelineRun's creation: the store keeps the
	// earliest CreatedAt (deployment_store.go:88), so a backfilled row (CreatedAt = MintedAt,
	// deployment_backfill.go:172) merged with a live one reads early (task 08 does-not-claim).
	Duration          string                 `json:"duration"`
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
