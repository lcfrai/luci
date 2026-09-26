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

// findingDTO mirrors handlers.DiagnoseFinding (handlers/diagnose.go:60-67). The write half
// follows it.
type findingDTO struct {
	Problem      string   `json:"problem"`
	LikelyCause  string   `json:"likelyCause"`
	SuggestedFix string   `json:"suggestedFix"`
	Evidence     []string `json:"evidence"`
	Severity     string   `json:"severity"`
	Owner        string   `json:"owner"`
}

// The write half (task 04), at 406d0da95; every cited line re-read at eb85f0d9f.

// shipRequestDTO is the subset of models.TriggerDeployOptions phase 7 sends
// (models/models.go:2342-2383). It has NO scan-mode field on purpose (brief B2): turning a
// scanner off from a desktop is a security-posture change, and Panacea's rule is "off means
// skip, never fabricate". NoCache is a pointer exactly as Panacea's is (:2349-2352).
// TriggerDeploy decodes leniently (handlers/projects.go:2967-2968), so an unknown field would be
// silently dropped — which is why no speculative "metadata" field is sent (seam S7-N2).
type shipRequestDTO struct {
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
	NoCache *bool  `json:"noCache,omitempty"`
}

// shipResponseDTO mirrors TriggerDeploy's 202 data (handlers/projects.go:2998-3005).
type shipResponseDTO struct {
	Project     string `json:"project"`
	Status      string `json:"status"`
	PipelineRun string `json:"pipelineRun"`
}

// cancelResponseDTO mirrors CancelDeploy's data (handlers/projects.go:3670-3687).
type cancelResponseDTO struct {
	Status  string `json:"status"` // "cancelled" | "already_completed"
	Message string `json:"message"`
}

// rollbackResponseDTO is Rollback's data (handlers/projects.go:1636-1639): the Deployment
// RollbackDeployment returns, enriched with image/commit/branch/environment (:1619-1623).
// Its ID is the deployment rolled back TO (clients/kubernetes.go:5982, :6030).
type rollbackResponseDTO = deploymentDTO

// autoDeployDTO mirrors GetBranchAutoDeploy's data (handlers/projects.go:4136-4143) and
// SetBranchAutoDeploy's (:4192-4198): the same five keys in a data envelope. On the PUT,
// enabled is the bool just written (*body.Enabled, :4194), never null (review M6).
type autoDeployDTO struct {
	Branch             string `json:"branch"`
	Enabled            *bool  `json:"enabled"` // GET: null = no explicit label; PUT: always set
	ProjectDefault     bool   `json:"projectDefault"`
	Effective          bool   `json:"effective"`
	IsProductionBranch bool   `json:"isProductionBranch"`
}

// setAutoDeployDTO is SetBranchAutoDeploy's body (handlers/projects.go:4159-4167).
type setAutoDeployDTO struct {
	Enabled *bool `json:"enabled"` // required; always non-nil when sent
}

// postureDTO mirrors GetBranchAccessMode's data (handlers/project_security.go:668-677).
type postureDTO struct {
	Branch           string   `json:"branch"`
	Env              string   `json:"env"`
	AccessMode       string   `json:"accessMode"`
	IPWhitelist      []string `json:"ipWhitelist"`
	PasswordSet      bool     `json:"passwordSet"`
	BypassTokenCount int      `json:"bypassTokenCount"`
}

// setPostureDTO mirrors branchAccessModeRequest (handlers/project_security.go:681-691).
// BypassTokens (caller-supplied raw tokens) is deliberately NOT declared: the cockpit never
// supplies a token, it only asks Panacea to mint one.
type setPostureDTO struct {
	AccessMode         string `json:"accessMode"`
	Password           string `json:"password,omitempty"`
	RotateBypassTokens bool   `json:"rotateBypassTokens,omitempty"`
}

// setPostureResponseDTO mirrors SetBranchAccessMode's data (handlers/project_security.go:824-845).
// The envelope's message is carried only when enforced is true ("access mode set to …", :835).
// When enforced is false Panacea's sentence embeds the reconcile error's err.Error() (:818-821,
// :837-839), so it is logged at warn and never carried; SetPosture writes notEnforcedSentence
// (governance.go). BypassTokens is present whenever one was minted, enforced or not: bypass-token
// with no supplied token always mints (:786-797), and it is returned ONCE (:830-833).
type setPostureResponseDTO struct {
	Branch       string   `json:"branch"`
	Env          string   `json:"env"`
	AccessMode   string   `json:"accessMode"`
	Enforced     bool     `json:"enforced"`
	BypassTokens []string `json:"bypassTokens"`
}

// accessBindingDTO mirrors models.AccessBinding (models/models.go:2004-2015) as ListProjectAccess
// fills it (clients/keycloak.go:143-159): a person's row has group == grantee; a legacy row has
// group = the Keycloak group's name and grantee = its panacea-group attribute. The model's
// comment says a legacy grantee is "" (:2011); the code never sends that (review M30).
type accessBindingDTO struct {
	Group   string `json:"group"`
	Role    string `json:"role"`
	Grantee string `json:"grantee"`
}

// grantDTO mirrors models.AddAccessRequest (models/models.go:2016-2019). "group" carries a
// username or email: AddProjectAccess resolves it to one Keycloak user (clients/keycloak.go:180-190).
type grantDTO struct {
	Group string `json:"group"`
	Role  string `json:"role"`
}

// urlPrefixDTO is setBranchURLPrefixRequest (handlers/projects.go:4263-4265).
type urlPrefixDTO struct {
	Prefix string `json:"prefix"`
}

// urlPrefixResponseDTO mirrors SetBranchURLPrefix's data (handlers/projects.go:4365-4371).
type urlPrefixResponseDTO struct {
	Branch string `json:"branch"`
	Prefix string `json:"prefix"`
	Host   string `json:"host"`
}

// clearURLPrefixDTO mirrors ClearBranchURLPrefix's data {branch, routeReverted}
// (handlers/projects.go:4448-4451). RouteReverted is a pointer: an answer without it has not
// confirmed the revert, and reads as not reverted.
type clearURLPrefixDTO struct {
	Branch        string `json:"branch"`
	RouteReverted *bool  `json:"routeReverted"`
}
