package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Preview configuration is explicit saved project delegation. It cannot grant
// Deploy permission, approve new source, or authorize public publication.
type CoordinatorPreviewPlan struct {
	Dockerfile                    string `json:"dockerfile"`
	Port                          int    `json:"port"`
	ExpectedConfigurationRevision int    `json:"expectedConfigurationRevision"`
	AcknowledgePreviewBuild       bool   `json:"acknowledgePreviewBuild"`
}

type CoordinatorPreviewAction struct {
	ActionID  string                     `json:"actionId"`
	ReleaseID string                     `json:"releaseId"`
	Status    string                     `json:"status"`
	Message   string                     `json:"message"`
	StartedAt string                     `json:"startedAt"`
	UpdatedAt string                     `json:"updatedAt"`
	Evidence  *ProjectRunReleaseEvidence `json:"evidence,omitempty"`
}

func validateCoordinatorPreviewPlan(plan *CoordinatorPreviewPlan) error {
	if plan == nil {
		return nil
	}
	if !plan.AcknowledgePreviewBuild || plan.Port < 1 || plan.Port > 65535 || plan.Dockerfile == "" || len(plan.Dockerfile) > 1024 || blockedProjectFile(plan.Dockerfile) || plan.ExpectedConfigurationRevision < 0 || plan.ExpectedConfigurationRevision >= 1_000_000_000 {
		return errors.New("Save an explicit trusted Dockerfile, application port and current configuration revision before delegating preview builds.")
	}
	return nil
}

func coordinatorPreviewID(id string) string { return "rel-" + reviewDigest(id + "-preview")[:24] }

func validateCoordinatorPreview(row coordinatorContinuationRecord) error {
	if err := validateCoordinatorPreviewPlan(row.PreviewPlan); err != nil {
		return err
	}
	if row.PreviewPlan != nil && row.CheckPlan == nil {
		return ErrOrchestratorInvalid
	}
	if row.Delivery == nil || row.Delivery.Preview == nil {
		return nil
	}
	p := row.Delivery.Preview
	if row.PreviewPlan == nil || row.Delivery.Status != "verified" || p.ActionID != row.ID+"-preview" || p.ReleaseID != coordinatorPreviewID(row.ID) || len(p.Message) > 4000 {
		return ErrOrchestratorInvalid
	}
	switch p.Status {
	case "pending", "running", "ready", "unknown", "blocked":
	default:
		return ErrOrchestratorInvalid
	}
	if p.Status == "ready" && p.Evidence == nil {
		return ErrOrchestratorInvalid
	}
	if evidence := p.Evidence; evidence != nil {
		if evidence.ReleaseID != p.ReleaseID || evidence.ProjectID != row.ProjectID || evidence.Target != "local" || evidence.Outcome != "preview" || evidence.CheckID != row.Delivery.CheckID || evidence.Commit != row.Delivery.Commit || evidence.SourceDigest != row.Delivery.SourceDigest || !validRunApprovals(evidence.Approvals) || len(evidence.URL) > 4096 || len(evidence.Limitations) > 4000 {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

type coordinatorPreviewContextKey struct{}
type coordinatorPreviewOrigin struct {
	ContinuationID, ActorID, ActionID, ReleaseID, BindingDigest string
}

func coordinatorPreviewBinding(row coordinatorContinuationRecord) string {
	return reviewDigest(struct {
		CheckBinding string
		Plan         *CoordinatorPreviewPlan
		ActionID     string
		ReleaseID    string
	}{coordinatorCheckBinding(row), row.PreviewPlan, row.Delivery.Preview.ActionID, row.Delivery.Preview.ReleaseID})
}

// Caller holds s.mu. Current configured delegation is additional to the
// existing execution and Deploy authorizers, never a replacement for either.
func (s *Server) coordinatorPreviewAuthorityLocked(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	if err := s.coordinatorDeliveryAuthorityLocked(ctx, p, row); err != nil {
		return err
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return err
	}
	if policy.PreviewMode != "checked-source" || row.PreviewPlan == nil || row.Delivery == nil || row.Delivery.Status != "verified" {
		return errCoordinatorPolicy
	}
	if err := validateCoordinatorPreviewPlan(row.PreviewPlan); err != nil {
		return err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "application-config-"+p.ID)
	if err != nil {
		return err
	}
	configuration, err := readApplicationConfiguration(path, p.ID)
	if err != nil {
		return err
	}
	if configuration.Revision != row.PreviewPlan.ExpectedConfigurationRevision {
		return ErrOrchestratorConflict
	}
	return nil
}

func coordinatorPreviewCheck(run ProjectRun, row coordinatorContinuationRecord) (ProjectRunCheckEvidence, error) {
	if row.Delivery == nil || row.Delivery.Status != "verified" {
		return ProjectRunCheckEvidence{}, ErrOrchestratorConflict
	}
	integration, ready := coordinatorIntegratedRun(run, row)
	if !ready || integration != row.Delivery.IntegrationDigest {
		return ProjectRunCheckEvidence{}, ErrOrchestratorConflict
	}
	for _, check := range run.Checks {
		if check.CheckID == row.Delivery.CheckID {
			if validProjectRunCheck(run.Input, check) != nil || check.Commit != row.Delivery.Commit || check.SourceDigest != row.Delivery.SourceDigest {
				return ProjectRunCheckEvidence{}, ErrOrchestratorConflict
			}
			return check, nil
		}
	}
	return ProjectRunCheckEvidence{}, ErrOrchestratorConflict
}

// Existing preview handlers call this only for an unexported trusted origin.
// Direct user previews keep their existing request shape and identity behavior.
func (s *Server) admitCoordinatorPreview(ctx context.Context, p project.Project, commit string, configurationRevision, port int, dockerfile string) (string, string, *storedRelease, error) {
	origin, internal := ctx.Value(coordinatorPreviewContextKey{}).(coordinatorPreviewOrigin)
	if !internal {
		return "", "", nil, nil
	}
	if err := s.validateCoordinatorPreviewMutation(ctx, p); err != nil {
		return "", "", nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	if err != nil || index < 0 {
		return "", "", nil, ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	if err = s.coordinatorPreviewAuthorityLocked(ctx, p, row); err != nil {
		return "", "", nil, err
	}
	if row.Delivery.Preview == nil || row.Delivery.Preview.Status != "pending" || origin.ActionID != row.Delivery.Preview.ActionID || origin.ReleaseID != row.Delivery.Preview.ReleaseID || origin.BindingDigest != coordinatorPreviewBinding(row) || row.Delivery.Commit != commit || row.PreviewPlan.ExpectedConfigurationRevision != configurationRevision || row.PreviewPlan.Port != port || row.PreviewPlan.Dockerfile != dockerfile {
		return "", "", nil, ErrOrchestratorConflict
	}
	releases, err := s.readReleases(p.ID)
	if err != nil {
		return "", "", nil, err
	}
	for _, release := range releases {
		if release.ID == origin.ReleaseID {
			if release.CoordinatorOriginDigest != origin.BindingDigest {
				return "", "", nil, ErrOrchestratorConflict
			}
			return release.ID, origin.BindingDigest, &release, nil
		}
	}
	return origin.ReleaseID, origin.BindingDigest, nil, nil
}

// Runs before each new preview runtime mutation in addition to the existing
// runtime's fresh Deploy authorization and reviewed-source validation. It never
// starts or repairs anything while checking current policy/configuration.
func (s *Server) validateCoordinatorPreviewMutation(ctx context.Context, p project.Project) error {
	origin, internal := ctx.Value(coordinatorPreviewContextKey{}).(coordinatorPreviewOrigin)
	if !internal {
		return nil
	}
	runtime := s.PreviewRuntime()
	if runtime.CaptureActor == nil {
		return errCoordinatorAuthority
	}
	if _, err := runtime.CaptureActor(ctx, p.ID); err != nil {
		return errCoordinatorAuthority
	}
	s.mu.Lock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	if err != nil || index < 0 {
		s.mu.Unlock()
		return ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	err = s.coordinatorPreviewAuthorityLocked(ctx, p, row)
	if err == nil && (row.Delivery.Preview == nil || origin.ActionID != row.Delivery.Preview.ActionID || origin.ReleaseID != row.Delivery.Preview.ReleaseID || origin.BindingDigest != coordinatorPreviewBinding(row)) {
		err = ErrOrchestratorConflict
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		return err
	}
	check, err := coordinatorPreviewCheck(run, row)
	if err != nil {
		return err
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return err
	}
	return s.validateProjectRunCandidate(ctx, p.ID, check)
}

func (s *Server) invokeCoordinatorPreview(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	release, err := s.AcquireGitSessionAdmission(ctx, p.ID)
	if err != nil {
		return errCoordinatorSourceBusy
	}
	defer release()
	body := struct {
		ExpectedCommit string `json:"expectedCommit"`
		Target         string `json:"target"`
		Port           int    `json:"port"`
		Dockerfile     string `json:"dockerfile"`
	}{row.Delivery.Commit, "local", row.PreviewPlan.Port, row.PreviewPlan.Dockerfile}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	origin := coordinatorPreviewOrigin{row.ID, row.Actor.ID, row.Delivery.Preview.ActionID, row.Delivery.Preview.ReleaseID, coordinatorPreviewBinding(row)}
	request, err := http.NewRequestWithContext(context.WithValue(ctx, coordinatorPreviewContextKey{}, origin), http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.SetPathValue("id", p.ID)
	request.Header.Set("Content-Type", "application/json")
	response := &coordinatorResponse{header: make(http.Header)}
	s.createLocalRelease(response, request)
	if response.overflow {
		return errors.New("The preview response is unavailable. Observe its reserved exact release without replaying its build.")
	}
	if response.status < 200 || response.status >= 300 {
		return &coordinatorStepError{Status: response.status, Message: "The preview did not confirm completion. Its exact release and any allocation remain retained."}
	}
	return nil
}

func coordinatorPreviewNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }
