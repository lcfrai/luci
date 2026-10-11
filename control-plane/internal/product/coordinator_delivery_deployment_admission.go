package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This policy delegates application delivery on an existing connected host.
// It contains references and approved effects, never provider credentials or
// environment values, and it does not authorize provisioning another host.
type CoordinatorDeploymentPlan struct {
	TargetID                       string `json:"targetId"`
	TargetFingerprint              string `json:"targetFingerprint"`
	ServerUUID                     string `json:"serverUuid"`
	Repository                     string `json:"repository"`
	Branch                         string `json:"branch"`
	Domain                         string `json:"domain"`
	Port                           int    `json:"port"`
	HealthPath                     string `json:"healthPath"`
	Dockerfile                     string `json:"dockerfile"`
	Source                         string `json:"source"`
	PrivateKeyUUID                 string `json:"privateKeyUuid,omitempty"`
	DataClientFramework            string `json:"dataClientFramework"`
	ExpectedConfigurationRevision  int    `json:"expectedConfigurationRevision"`
	ExpectedDataDomainRevision     int    `json:"expectedDataDomainRevision"`
	ExpectedDataBindingFingerprint string `json:"expectedDataBindingFingerprint"`
	AcknowledgeRepositoryPush      bool   `json:"acknowledgeRepositoryPush"`
	AcknowledgePublic              bool   `json:"acknowledgePublic"`
	AcknowledgeDeploymentCosts     bool   `json:"acknowledgeDeploymentCosts"`
}

type CoordinatorDeploymentAction struct {
	ActionID  string                     `json:"actionId"`
	ReleaseID string                     `json:"releaseId"`
	Status    string                     `json:"status"`
	Message   string                     `json:"message"`
	StartedAt string                     `json:"startedAt"`
	UpdatedAt string                     `json:"updatedAt"`
	Evidence  *ProjectRunReleaseEvidence `json:"evidence,omitempty"`
}

func coordinatorDeploymentRequest(plan *CoordinatorDeploymentPlan, commit string) CloudReleaseRequest {
	return CloudReleaseRequest{TargetID: plan.TargetID, Repository: plan.Repository, Branch: plan.Branch, Commit: commit, Domain: plan.Domain, Port: plan.Port, HealthPath: plan.HealthPath, Dockerfile: plan.Dockerfile, Source: plan.Source, PrivateKeyUUID: plan.PrivateKeyUUID, DataClientFramework: plan.DataClientFramework}
}

func validateCoordinatorDeploymentPlan(plan *CoordinatorDeploymentPlan) error {
	if plan == nil {
		return nil
	}
	if !plan.AcknowledgeRepositoryPush || !plan.AcknowledgePublic || !plan.AcknowledgeDeploymentCosts || !coolifyID.MatchString(plan.TargetID) || !hexDigest(plan.TargetFingerprint) || !coolifyID.MatchString(plan.ServerUUID) || plan.ExpectedConfigurationRevision < 0 || plan.ExpectedConfigurationRevision >= 1_000_000_000 || plan.ExpectedDataDomainRevision < 0 || plan.ExpectedDataDomainRevision >= 1_000_000_000 || (plan.ExpectedDataBindingFingerprint != "" && !hexDigest(plan.ExpectedDataBindingFingerprint)) || (plan.ExpectedDataDomainRevision == 0) != (plan.ExpectedDataBindingFingerprint == "") {
		return errors.New("Save the exact existing target, configuration and Data binding, and acknowledge the exact repository push, public exposure and ongoing deployment costs.")
	}
	if len(plan.Repository) > 2048 || len(plan.Domain) > 2048 || len(plan.HealthPath) > 1024 || plan.Dockerfile == "" || len(plan.Dockerfile) > 1024 || strings.HasPrefix(plan.Dockerfile, "/") || blockedProjectFile(plan.Dockerfile) || strings.HasSuffix(plan.Domain, "/") || !validDataClientFramework(plan.DataClientFramework) || (plan.Source != "public" && plan.Source != "private") || (plan.Source == "public" && plan.PrivateKeyUUID != "") {
		return ErrOrchestratorInvalid
	}
	if _, err := releaseHealthURL(plan.Domain, plan.HealthPath); err != nil {
		return err
	}
	_, err := coolifyGitApplicationPayload(CoolifyGitApplication{ProjectUUID: "pending", ServerUUID: plan.ServerUUID, Name: "luci-delegated-application", Repository: plan.Repository, Branch: plan.Branch, Commit: strings.Repeat("a", 40), Dockerfile: "/" + plan.Dockerfile, Port: plan.Port, Domain: plan.Domain, HealthPath: plan.HealthPath, Source: plan.Source, PrivateKeyUUID: plan.PrivateKeyUUID})
	return err
}

func coordinatorDeploymentID(id string) string {
	return "cloud-rel-" + reviewDigest(id + "-deploy")[:24]
}

func validateCoordinatorDeployment(row coordinatorContinuationRecord) error {
	if err := validateCoordinatorGitPublication(row); err != nil {
		return err
	}
	if err := validateCoordinatorDeploymentPlan(row.DeploymentPlan); err != nil {
		return err
	}
	if row.DeploymentPlan != nil && (row.CheckPlan == nil || row.PreviewPlan == nil || row.Run.Outcome != "publish") {
		return ErrOrchestratorInvalid
	}
	if row.Delivery == nil || row.Delivery.Deployment == nil {
		return nil
	}
	d := row.Delivery.Deployment
	if !coordinatorGitPublished(row) || row.DeploymentPlan == nil || row.Delivery.Status != "verified" || row.Delivery.Preview == nil || row.Delivery.Preview.Status != "ready" || d.ActionID != row.ID+"-deploy" || d.ReleaseID != coordinatorDeploymentID(row.ID) || len(d.Message) > 4000 {
		return ErrOrchestratorInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, d.StartedAt); err != nil {
		return ErrOrchestratorInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, d.UpdatedAt); err != nil {
		return ErrOrchestratorInvalid
	}
	switch d.Status {
	case "pending", "running", "ready", "unknown", "blocked":
	default:
		return ErrOrchestratorInvalid
	}
	if d.Status == "ready" && d.Evidence == nil {
		return ErrOrchestratorInvalid
	}
	if e := d.Evidence; e != nil {
		if e.ReleaseID != d.ReleaseID || e.ProjectID != row.ProjectID || e.Target != row.DeploymentPlan.TargetID || e.Outcome != "publish" || e.CheckID != row.Delivery.CheckID || e.Commit != row.Delivery.Commit || e.SourceDigest != row.Delivery.SourceDigest || !validRunApprovals(e.Approvals) || e.URL != row.DeploymentPlan.Domain || len(e.Limitations) > 4000 {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

type coordinatorDeploymentContextKey struct{}
type coordinatorDeploymentOrigin struct{ ContinuationID, ActorID, ActionID, ReleaseID, BindingDigest string }

func coordinatorDeploymentBinding(row coordinatorContinuationRecord) string {
	return reviewDigest(struct {
		CheckBinding, PreviewReleaseID string
		Plan                           *CoordinatorDeploymentPlan
		ActionID, ReleaseID            string
	}{coordinatorCheckBinding(row), row.Delivery.Preview.ReleaseID, row.DeploymentPlan, row.Delivery.Deployment.ActionID, row.Delivery.Deployment.ReleaseID})
}

// Caller holds s.mu. This check cannot grant Deploy or create any provider
// resource; fresh management authorization is a separate mandatory boundary.
func (s *Server) coordinatorDeploymentAuthorityLocked(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	if err := s.coordinatorDeliveryAuthorityLocked(ctx, p, row); err != nil {
		return err
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return err
	}
	if policy.DeploymentMode != "existing-target" || row.DeploymentPlan == nil || row.Run.Outcome != "publish" || row.Delivery == nil || row.Delivery.Status != "verified" || row.Delivery.Preview == nil || row.Delivery.Preview.Status != "ready" {
		return errCoordinatorPolicy
	}
	plan := row.DeploymentPlan
	if err := validateCoordinatorDeploymentPlan(plan); err != nil {
		return err
	}
	target, found := s.targets[plan.TargetID]
	if !found || target.ServerUUID != plan.ServerUUID || targetFingerprint(target) != plan.TargetFingerprint {
		return ErrOrchestratorConflict
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "application-config-"+p.ID)
	if err != nil {
		return err
	}
	config, err := readApplicationConfiguration(path, p.ID)
	if err != nil {
		return err
	}
	if config.Revision != plan.ExpectedConfigurationRevision {
		return ErrOrchestratorConflict
	}
	return nil
}

func (s *Server) authorizeCoordinatorDeployment(ctx context.Context, p project.Project, row coordinatorContinuationRecord, allocate bool) (context.Context, error) {
	actor, ok := authenticatedContextActor(ctx)
	if !ok || actor.ID != row.Actor.ID {
		return ctx, errCoordinatorAuthority
	}
	return s.reauthorizeCoordinatorDeploymentActor(ctx, p, row, allocate)
}

// Background resumption owns only this stored actor reference. Rehydrate it
// through the current server authorizer outside s.mu; no browser identity or
// captured allocation authority can substitute for this fresh decision.
func (s *Server) reauthorizeCoordinatorDeploymentActor(ctx context.Context, p project.Project, row coordinatorContinuationRecord, allocate bool) (context.Context, error) {
	if s.coordinatorDeploymentAuthorize == nil || row.ExecutionActor.UserID == "" || row.ExecutionActor.Local || row.ExecutionActor.UserID != row.Actor.ID || row.ExecutionActor.ProjectID != p.ID {
		return ctx, errCoordinatorAuthority
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, row.ExecutionActor, allocate)
	if err != nil || !authority.Policy.AllowPublicPublishing {
		return ctx, errCoordinatorAuthority
	}
	current := management.WithExecutionAuthority(ctx, authority)
	captured, err := management.CaptureDeploymentActor(current, p.ID)
	if err != nil || captured != row.ExecutionActor {
		return ctx, errCoordinatorAuthority
	}
	return current, nil
}

// Shared with the typed Git publication step. This reads current authority,
// exact checked source and saved delivery bindings; it never reserves work,
// executes Git network operations, or mutates provider resources.
func (s *Server) coordinatorDeploymentAuthority(ctx context.Context, p project.Project, row coordinatorContinuationRecord, allocate bool) (context.Context, error) {
	current, err := s.authorizeCoordinatorDeployment(ctx, p, row, allocate)
	if err != nil {
		return ctx, err
	}
	s.mu.Lock()
	err = s.coordinatorDeploymentAuthorityLocked(current, p, row)
	s.mu.Unlock()
	if err != nil {
		return ctx, err
	}
	_, _, err = s.validateCoordinatorDeploymentCandidate(current, p, row)
	return current, err
}

func (s *Server) validateCoordinatorDeploymentCandidate(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (ProjectRun, ProjectRunCheckEvidence, error) {
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		return run, ProjectRunCheckEvidence{}, err
	}
	check, err := coordinatorPreviewCheck(run, row)
	if err != nil {
		return run, check, err
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return run, check, err
	}
	if err = s.validateProjectRunCandidate(ctx, p.ID, check); err != nil {
		return run, check, err
	}
	plan := row.DeploymentPlan
	cloud := CloudRelease{Release: Release{ProjectID: p.ID, Commit: check.Commit, Target: plan.TargetID}, CloudReleaseRequest: coordinatorDeploymentRequest(plan, check.Commit), TargetFingerprint: plan.TargetFingerprint, ServerUUID: plan.ServerUUID}
	data, err := s.cloudApplicationDataSnapshot(ctx, cloud)
	if err != nil {
		return run, check, err
	}
	if data.DomainRevision != plan.ExpectedDataDomainRevision || cloudDataFingerprint(data, plan.DataClientFramework) != plan.ExpectedDataBindingFingerprint {
		return run, check, ErrOrchestratorConflict
	}
	return run, check, nil
}

// A typed handler calls this before every mutating Coolify request, including
// individual environment changes. Read-only observations never install it.
func (s *Server) validateCoordinatorDeploymentMutation(ctx context.Context, p project.Project) error {
	origin, internal := ctx.Value(coordinatorDeploymentContextKey{}).(coordinatorDeploymentOrigin)
	if !internal {
		return nil
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	s.mu.RUnlock()
	if err != nil || index < 0 {
		return ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	current, err := s.authorizeCoordinatorDeployment(ctx, p, row, true)
	if err != nil {
		return err
	}
	s.mu.Lock()
	err = s.coordinatorDeploymentAuthorityLocked(current, p, row)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if !coordinatorGitPublished(row) || row.Delivery.Deployment == nil || row.Delivery.Deployment.ActionID != origin.ActionID || row.Delivery.Deployment.ReleaseID != origin.ReleaseID || coordinatorDeploymentBinding(row) != origin.BindingDigest {
		return ErrOrchestratorConflict
	}
	_, _, err = s.validateCoordinatorDeploymentCandidate(current, p, row)
	return err
}

func (s *Server) admitCoordinatorDeployment(ctx context.Context, p project.Project, cloud CloudRelease) (string, string, error) {
	origin, internal := ctx.Value(coordinatorDeploymentContextKey{}).(coordinatorDeploymentOrigin)
	if !internal {
		return "", "", nil
	}
	if err := s.validateCoordinatorDeploymentMutation(ctx, p); err != nil {
		return "", "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	if err != nil || index < 0 {
		return "", "", ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	plan := row.DeploymentPlan
	if row.Delivery == nil || row.Delivery.Deployment == nil || row.Delivery.Deployment.Status != "pending" || cloud.CloudReleaseRequest != coordinatorDeploymentRequest(plan, row.Delivery.Commit) || cloud.ConfigurationRevision != plan.ExpectedConfigurationRevision || cloud.DataDomainRevision != plan.ExpectedDataDomainRevision || cloud.DataBindingFingerprint != plan.ExpectedDataBindingFingerprint || cloud.TargetFingerprint != plan.TargetFingerprint || cloud.ServerUUID != plan.ServerUUID || coordinatorDeploymentBinding(row) != origin.BindingDigest {
		return "", "", ErrOrchestratorConflict
	}
	return origin.ReleaseID, origin.BindingDigest, nil
}

// A previously owned application may have been changed outside Luci. Verify
// its current provider identity before updating configuration or deploying it;
// only a known private prior commit or this exact new commit is admissible.
func (s *Server) validateCoordinatorDeploymentApplication(ctx context.Context, p project.Project, client *CoolifyClient) error {
	origin, internal := ctx.Value(coordinatorDeploymentContextKey{}).(coordinatorDeploymentOrigin)
	if !internal {
		return nil
	}
	s.mu.RLock()
	releases, err := s.readCloudReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	var exact *CloudRelease
	for _, release := range releases {
		if release.ID == origin.ReleaseID && release.CoordinatorOriginDigest == origin.BindingDigest {
			copy := release
			exact = &copy
			break
		}
	}
	if exact == nil {
		return ErrOrchestratorConflict
	}
	if exact.ApplicationID == "" {
		return nil
	}
	var app struct {
		UUID       string `json:"uuid"`
		ServerUUID string `json:"server_uuid"`
		Repository string `json:"git_repository"`
		Branch     string `json:"git_branch"`
		Commit     string `json:"git_commit_sha"`
		Domain     string `json:"fqdn"`
	}
	if err = client.request(ctx, http.MethodGet, "/applications/"+exact.ApplicationID, nil, nil, &app); err != nil {
		return err
	}
	repository := exact.Repository
	if exact.Source == "private" {
		owner, name, parseErr := githubDeploymentRepository(repository)
		if parseErr != nil {
			return parseErr
		}
		repository = "git@github.com:" + owner + "/" + name + ".git"
	}
	if app.UUID != exact.ApplicationID || app.ServerUUID != exact.ServerUUID || app.Repository != repository || app.Branch != exact.Branch || strings.TrimSuffix(app.Domain, "/") != exact.Domain {
		return errors.New("The provider application no longer matches this exact owned deployment target and repository.")
	}
	known := app.Commit == exact.Release.Commit
	for _, previous := range releases {
		if previous.ApplicationID == exact.ApplicationID && previous.TargetFingerprint == exact.TargetFingerprint && previous.Repository == exact.Repository && previous.Branch == exact.Branch && previous.Domain == exact.Domain && previous.Status == "ready" && previous.Release.Commit == app.Commit {
			known = true
		}
	}
	if !known {
		return errors.New("The provider application source changed outside its retained owned release history.")
	}
	return nil
}

func (s *Server) invokeCoordinatorDeployment(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	release, err := s.AcquireGitSessionAdmission(ctx, p.ID)
	if err != nil {
		return errCoordinatorSourceBusy
	}
	defer release()
	current, err := s.authorizeCoordinatorDeployment(ctx, p, row, true)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(coordinatorDeploymentRequest(row.DeploymentPlan, row.Delivery.Commit))
	if err != nil {
		return err
	}
	origin := coordinatorDeploymentOrigin{row.ID, row.Actor.ID, row.Delivery.Deployment.ActionID, row.Delivery.Deployment.ReleaseID, coordinatorDeploymentBinding(row)}
	request, err := http.NewRequestWithContext(context.WithValue(current, coordinatorDeploymentContextKey{}, origin), http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.SetPathValue("id", p.ID)
	request.Header.Set("Content-Type", "application/json")
	response := &coordinatorResponse{header: make(http.Header)}
	s.createCloudRelease(response, request)
	if response.overflow {
		return errors.New("The deployment response was lost. Inspect its exact reserved release; no provider request is replayed.")
	}
	if response.status < 200 || response.status >= 300 {
		return &coordinatorStepError{Status: response.status, Message: "The deployment did not confirm admission. Its exact release and provider resources remain retained."}
	}
	return nil
}

// Pure preparation exposes only current reference fingerprints. It never saves
// delegation, grants consent, creates resources, or returns environment values.
func (s *Server) prepareCoordinatorDeploymentPlan(w http.ResponseWriter, r *http.Request) {
	var plan CoordinatorDeploymentPlan
	if !journeyDecode(w, r, &plan) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ref, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || s.coordinatorDeploymentAuthorize == nil {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	authority, err := s.coordinatorDeploymentAuthorize(r.Context(), ref, false)
	if err != nil {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	current := management.WithExecutionAuthority(r.Context(), authority)
	captured, err := management.CaptureDeploymentActor(current, p.ID)
	if err != nil || captured != ref {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	target, err := s.deploymentTarget(plan.TargetID)
	if err != nil {
		coordinatorPolicyProblem(w, ErrOrchestratorConflict)
		return
	}
	configuration, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	plan.TargetFingerprint, plan.ServerUUID, plan.ExpectedConfigurationRevision = targetFingerprint(target), target.ServerUUID, configuration.Revision
	plan.ExpectedDataDomainRevision, plan.ExpectedDataBindingFingerprint = 0, ""
	plan.AcknowledgeRepositoryPush, plan.AcknowledgePublic, plan.AcknowledgeDeploymentCosts = true, true, true // validation only; never returned as consent
	if err = validateCoordinatorDeploymentPlan(&plan); err != nil {
		journeyError(w, 400, "invalid_deployment_plan", err.Error())
		return
	}
	cloud := CloudRelease{Release: Release{ProjectID: p.ID, Target: plan.TargetID}, CloudReleaseRequest: coordinatorDeploymentRequest(&plan, ""), TargetFingerprint: plan.TargetFingerprint, ServerUUID: plan.ServerUUID}
	data, err := s.cloudApplicationDataSnapshot(current, cloud)
	if err != nil {
		coordinatorPolicyProblem(w, ErrOrchestratorConflict)
		return
	}
	plan.ExpectedDataDomainRevision, plan.ExpectedDataBindingFingerprint = data.DomainRevision, cloudDataFingerprint(data, plan.DataClientFramework)
	plan.AcknowledgeRepositoryPush, plan.AcknowledgePublic, plan.AcknowledgeDeploymentCosts = false, false, false
	if r.Context().Err() != nil {
		return
	}
	authority, err = s.coordinatorDeploymentAuthorize(r.Context(), ref, false)
	if err != nil {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	captured, err = management.CaptureDeploymentActor(management.WithExecutionAuthority(r.Context(), authority), p.ID)
	if err != nil || captured != ref {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	journeyJSON(w, 200, plan)
}

// Dependency injection belongs to the trusted server composition. HTTP input
// and environment variables have no path to replace public DNS/TLS guards.
func (s *Server) cloudReleaseHealth(ctx context.Context, address string) error {
	if s.cloudReleaseHealthObserver != nil {
		return s.cloudReleaseHealthObserver(ctx, address)
	}
	return checkPublicReleaseHealth(ctx, address)
}
