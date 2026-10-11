package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Commands are explicitly delegated once with the continuation. Requirements,
// private approvals and integrated source are resolved from its immutable run.
// Model output and suggested commands are never command consent.
type CoordinatorCheckPlan struct {
	Commands                   []CheckCommand        `json:"commands"`
	RequirementIDs             []string              `json:"requirementIds"`
	RequirementSnapshots       []RequirementSnapshot `json:"requirementSnapshots"`
	AcknowledgeProjectCommands bool                  `json:"acknowledgeProjectCommands"`
}

type CoordinatorDelivery struct {
	GitPublication    *CoordinatorGitAction        `json:"gitPublication,omitempty"`
	Deployment        *CoordinatorDeploymentAction `json:"deployment,omitempty"`
	Preview           *CoordinatorPreviewAction    `json:"preview,omitempty"`
	ActionID          string                       `json:"actionId"`
	CheckID           string                       `json:"checkId"`
	Status            string                       `json:"status"`
	Commit            string                       `json:"commit"`
	SourceDigest      string                       `json:"sourceDigest"`
	IntegrationDigest string                       `json:"integrationDigest"`
	Message           string                       `json:"message"`
	StartedAt         string                       `json:"startedAt"`
	UpdatedAt         string                       `json:"updatedAt"`
}

func validateCoordinatorCheckPlan(plan *CoordinatorCheckPlan) error {
	if plan == nil {
		return nil
	}
	if !plan.AcknowledgeProjectCommands {
		return errors.New("Delegate these exact project commands before continuing independent checks.")
	}
	if len(plan.RequirementIDs) == 0 || len(plan.RequirementIDs) != len(plan.RequirementSnapshots) || validateRequirementIDs(plan.RequirementIDs) != nil {
		return errors.New("Select the reviewed requirements these commands exercise, including their exact acceptance text.")
	}
	for i, snapshot := range plan.RequirementSnapshots {
		if snapshot.ID != plan.RequirementIDs[i] || len(snapshot.Title) > 2000 || len(snapshot.Acceptance) > 8000 {
			return errors.New("The declared command coverage does not match its selected requirement snapshots.")
		}
	}
	return validateCheckCommands(plan.Commands)
}

// A command plan declares coverage of exact reviewed definitions. Successful
// execution does not establish semantic satisfaction. This initial continuation
// supports full selected-run coverage; it never silently adds missing IDs or
// relabels a previous command plan after acceptance text changes.
func validateCoordinatorCheckCoverage(plan *CoordinatorCheckPlan, requirements []RequirementSnapshot) error {
	if plan == nil {
		return nil
	}
	if err := validateCoordinatorCheckPlan(plan); err != nil {
		return err
	}
	if len(plan.RequirementSnapshots) != len(requirements) {
		return ErrOrchestratorConflict
	}
	wanted := make(map[string]RequirementSnapshot, len(requirements))
	for _, requirement := range requirements {
		wanted[requirement.ID] = requirement
	}
	for _, snapshot := range plan.RequirementSnapshots {
		if current, found := wanted[snapshot.ID]; !found || current != snapshot {
			return ErrOrchestratorConflict
		}
		delete(wanted, snapshot.ID)
	}
	if len(wanted) != 0 {
		return ErrOrchestratorConflict
	}
	return nil
}

// Caller holds s.mu. Expected snapshots come from the consented plan, while
// truth comes from the current private approved journey and selected tasks.
func (s *Server) coordinatorCheckCoverageLocked(p project.Project, selection ImplementationContextRequest, plan *CoordinatorCheckPlan) error {
	if plan == nil {
		return nil
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return err
	}
	snapshot, err := implementationSnapshot(state, selection, "")
	if err != nil {
		return err
	}
	binding, err := bindProjectRequirements(state, p.ID, snapshot.RequirementIDs, &snapshot.SpecificationRevision, &snapshot.DesignPlanRevision, true)
	if err != nil {
		return err
	}
	return validateCoordinatorCheckCoverage(plan, binding.RequirementSnapshots)
}

func validateCoordinatorDelivery(row coordinatorContinuationRecord) error {
	if err := validateCoordinatorCheckPlan(row.CheckPlan); err != nil {
		return err
	}
	d := row.Delivery
	if d == nil {
		return nil
	}
	if row.CheckPlan == nil || len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" ||
		d.ActionID != row.ID+"-checks" || d.CheckID != coordinatorCheckID(row.ID) ||
		!deliverySHA.MatchString(d.Commit) || !hexDigest(d.SourceDigest) || !hexDigest(d.IntegrationDigest) || len(d.Message) > 4000 {
		return ErrOrchestratorInvalid
	}
	switch d.Status {
	case "pending", "running", "verified", "unknown", "blocked":
		return nil
	default:
		return ErrOrchestratorInvalid
	}
}

func coordinatorCheckID(id string) string { return "check-" + reviewDigest(id + "-checks")[:24] }

// This unexported context value is installed only by the fixed typed adapter.
// Public check requests cannot select an ID or manufacture coordinator origin.
type coordinatorCheckContextKey struct{}
type coordinatorCheckOrigin struct {
	ContinuationID string
	ActorID        string
	ActionID       string
	CheckID        string
	BindingDigest  string
}

func coordinatorCheckBinding(row coordinatorContinuationRecord) string {
	d := row.Delivery
	return reviewDigest(struct {
		ContinuationID, ActorID, RunID, ContextDigest, CanonicalDigest string
		PolicyRevision                                                 int
		CheckPlan                                                      *CoordinatorCheckPlan
		ActionID, CheckID, Commit, SourceDigest, IntegrationDigest     string
	}{row.ID, row.Actor.ID, row.RunID, row.ContextDigest, row.CanonicalDigest, row.PolicyRevision, row.CheckPlan,
		d.ActionID, d.CheckID, d.Commit, d.SourceDigest, d.IntegrationDigest})
}

// Caller holds s.mu. This is invoked before check ownership is reserved. A
// duplicate internal call may observe its exact receipt but cannot execute it.
func (s *Server) admitCoordinatorCheckLocked(ctx context.Context, p project.Project, commands []CheckCommand, binding projectRequirementBinding, digest, commit string, dirty bool) (string, string, *ProjectCheckRun, error) {
	origin, internal := ctx.Value(coordinatorCheckContextKey{}).(coordinatorCheckOrigin)
	if !internal {
		return "", "", nil, nil
	}
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	if err != nil || index < 0 {
		return "", "", nil, ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	if err = s.coordinatorDeliveryAuthorityLocked(ctx, p, row); err != nil {
		return "", "", nil, err
	}
	if row.Delivery == nil || row.Delivery.Status != "pending" || origin.ActionID != row.Delivery.ActionID || origin.CheckID != row.Delivery.CheckID ||
		origin.BindingDigest != coordinatorCheckBinding(row) || reviewDigest(commands) != reviewDigest(row.CheckPlan.Commands) ||
		dirty || digest != row.Delivery.SourceDigest || commit != row.Delivery.Commit {
		return "", "", nil, ErrOrchestratorConflict
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		return "", "", nil, err
	}
	integration, ready := coordinatorIntegratedRun(run, row)
	if !ready || integration != row.Delivery.IntegrationDigest || reviewDigest(binding.RequirementSnapshots) != reviewDigest(row.CheckPlan.RequirementSnapshots) || validateCoordinatorCheckCoverage(row.CheckPlan, run.Input.Requirements) != nil {
		return "", "", nil, ErrOrchestratorConflict
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return "", "", nil, err
	}
	approvals, err := projectRunApprovalSelection(state)
	if err != nil || approvals != run.Input.Approvals {
		return "", "", nil, ErrOrchestratorConflict
	}
	checks, err := s.readCheckHistory(p.ID)
	if err != nil {
		return "", "", nil, err
	}
	for _, check := range checks {
		if check.ID == origin.CheckID {
			if check.CoordinatorOriginDigest != origin.BindingDigest {
				return "", "", nil, ErrOrchestratorConflict
			}
			copy := snapshotProjectCheck(check)
			return origin.CheckID, origin.BindingDigest, &copy, nil
		}
	}
	return origin.CheckID, origin.BindingDigest, nil, nil
}

func (s *Server) invokeCoordinatorChecks(ctx context.Context, p project.Project, row coordinatorContinuationRecord, run ProjectRun) error {
	body := struct {
		Commands                      []CheckCommand `json:"commands"`
		Execution                     string         `json:"execution"`
		Acknowledge                   bool           `json:"acknowledgeHostExecution"`
		RequirementIDs                []string       `json:"requirementIds"`
		ExpectedSpecificationRevision int            `json:"expectedSpecificationRevision"`
		ExpectedDesignPlanRevision    int            `json:"expectedDesignPlanRevision"`
	}{row.CheckPlan.Commands, "docker", row.CheckPlan.AcknowledgeProjectCommands, row.CheckPlan.RequirementIDs,
		run.Input.Implementation.SpecificationRevision, run.Input.Implementation.DesignPlanRevision}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	origin := coordinatorCheckOrigin{row.ID, row.Actor.ID, row.Delivery.ActionID, row.Delivery.CheckID, coordinatorCheckBinding(row)}
	request, err := http.NewRequestWithContext(context.WithValue(ctx, coordinatorCheckContextKey{}, origin), http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.SetPathValue("id", p.ID)
	request.Header.Set("Content-Type", "application/json")
	response := &coordinatorResponse{header: make(http.Header)}
	s.startProjectChecks(response, request)
	if response.overflow {
		return errors.New("The check response is unavailable. Observe its reserved exact check; do not replay it.")
	}
	if response.status < 200 || response.status >= 300 {
		return &coordinatorStepError{Status: response.status, Message: "The existing check runtime did not confirm admission. Inspect its exact receipt; no command will be replayed."}
	}
	// HTTP success only acknowledges transport/admission. The caller reads the
	// reserved private check receipt and independently joins verified evidence.
	return nil
}
