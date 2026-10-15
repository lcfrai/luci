package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const orchestrationContractLimit = 200
const orchestrationContractFileLimit = 2 << 20

type OrchestrationRouteRequest struct {
	Harness           string  `json:"harness"`
	Model             string  `json:"model"`
	PermissionMode    string  `json:"permissionMode"`
	Effort            string  `json:"effort,omitempty"`
	ExecutionLocation string  `json:"executionLocation"`
	AgentConnectionID *string `json:"agentConnectionId,omitempty"`
}

type OrchestrationPrepareRequest struct {
	IdempotencyKey        string                       `json:"idempotencyKey"`
	WorktreeID            string                       `json:"worktreeId"`
	ImplementationContext ImplementationContextRequest `json:"implementationContext"`
	Route                 OrchestrationRouteRequest    `json:"route"`
	Outcome               string                       `json:"outcome"`
}

type OrchestrationControls struct {
	AuthorityPolicySource     string                       `json:"authorityPolicySource"`
	AuthorityPolicyDigest     string                       `json:"authorityPolicyDigest"`
	ExecutionLimits           OrchestrationExecutionLimits `json:"executionLimits"`
	TeamID                    string                       `json:"teamId,omitempty"`
	TeamRevision              int                          `json:"teamRevision,omitempty"`
	MemberRevision            int                          `json:"memberRevision,omitempty"`
	ProjectGrantRevision      int                          `json:"projectGrantRevision,omitempty"`
	CoordinatorPolicyRevision int                          `json:"coordinatorPolicyRevision"`
	CoordinatorPolicyDigest   string                       `json:"coordinatorPolicyDigest"`
	SchedulerPolicyRevision   int                          `json:"schedulerPolicyRevision"`
	SchedulerPolicyDigest     string                       `json:"schedulerPolicyDigest"`
	SchedulerPolicy           SwarmSchedulerPolicy         `json:"schedulerPolicy"`
}

// Nonsecret numeric and boolean limits are displayed; the full private
// policy is bound by digest and reauthorized at admission and dispatch.
type OrchestrationExecutionLimits struct {
	MonthlyBudgetUSD           float64 `json:"monthlyBudgetUsd"`
	SessionMemoryMiB           int     `json:"sessionMemoryMiB"`
	SessionCPUMillis           int     `json:"sessionCPUMillis"`
	MaxSessionsPerUser         int     `json:"maxSessionsPerUser"`
	MaxConcurrentChecksPerUser int     `json:"maxConcurrentChecksPerUser"`
	AllowCloudProvisioning     bool    `json:"allowCloudProvisioning"`
	AllowPublicPublishing      bool    `json:"allowPublicPublishing"`
}

type OrchestrationStandards struct {
	Status             string `json:"status"` // legacy-unassessed or not-configured
	Source             string `json:"source"`
	Revision           int    `json:"revision"`
	Digest             string `json:"digest"`
	TeamID             string `json:"teamId,omitempty"`
	AssessmentRevision int    `json:"assessmentRevision,omitempty"`
	AssessmentDigest   string `json:"assessmentDigest,omitempty"`
}

type OrchestrationDecisionPolicy struct {
	Routine    string `json:"routine"`
	HumanInput string `json:"humanInput"`
}

type OrchestrationPreparedInput struct {
	Implementation registry.ImplementationContext `json:"implementation"`
	Approvals      ProjectRunApprovalSelection    `json:"approvals"`
	Route          ProjectRunRoute                `json:"route"`
	Outcome        string                         `json:"outcome"`
	Requirements   []RequirementSnapshot          `json:"requirements"`
	Tasks          []ProjectTask                  `json:"tasks"`
	Controls       OrchestrationControls          `json:"controls"`
	Standards      OrchestrationStandards         `json:"standards"`
	DecisionPolicy OrchestrationDecisionPolicy    `json:"decisionPolicy"`
}

type OrchestrationApproval struct {
	ID     string       `json:"id"`
	Actor  ContextActor `json:"actor"`
	At     string       `json:"at"`
	Digest string       `json:"digest"`
}

type OrchestrationContract struct {
	ID        string                     `json:"id"`
	Revision  int                        `json:"revision"`
	Digest    string                     `json:"digest"`
	State     string                     `json:"state"`
	ProjectID string                     `json:"projectId"`
	Actor     ContextActor               `json:"actor"`
	Input     OrchestrationPreparedInput `json:"input"`
	Approval  *OrchestrationApproval     `json:"approval,omitempty"`
	CreatedAt string                     `json:"createdAt"`
}

type OrchestrationContractSelection struct {
	ID         string `json:"id"`
	Revision   int    `json:"revision"`
	Digest     string `json:"digest"`
	ApprovalID string `json:"approvalId"`
}

type OrchestrationContractView struct {
	Contract  OrchestrationContract      `json:"contract"`
	Freshness ProjectRunContextFreshness `json:"freshness"`
}

type orchestrationContractRecord struct {
	Contract      OrchestrationContract `json:"contract"`
	KeyDigest     string                `json:"keyDigest"`
	RequestDigest string                `json:"requestDigest"`
}

type orchestrationContractLedger struct {
	Version      int                           `json:"version"`
	AccountScope string                        `json:"accountScope"`
	ProjectScope string                        `json:"projectScope"`
	ProjectID    string                        `json:"projectId"`
	Records      []orchestrationContractRecord `json:"records"`
}

func (s *Server) orchestrationContractPath(p project.Project) (string, orchestrationContractLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", orchestrationContractLedger{}, err
	}
	path, err := guardedProjectPath(s.stateDir, "orchestration-contract-"+p.ID+".json", true)
	return path, orchestrationContractLedger{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: p.ID, Records: []orchestrationContractRecord{}}, err
}

// The caller holds orchestrationContractMu. All bytes are private, bounded and
// account/project scoped; a malformed record never turns into a fresh approval.
func (s *Server) readOrchestrationContracts(p project.Project) (string, orchestrationContractLedger, error) {
	path, expected, err := s.orchestrationContractPath(p)
	if err != nil {
		return "", expected, err
	}
	var saved orchestrationContractLedger
	err = externalReadJSON(path, orchestrationContractFileLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, expected, nil
	}
	if err != nil || saved.Version != 1 || saved.AccountScope != expected.AccountScope || saved.ProjectScope != expected.ProjectScope || saved.ProjectID != p.ID || len(saved.Records) > orchestrationContractLimit {
		return "", expected, ErrProjectRunInvalid
	}
	ids, keys := map[string]bool{}, map[string]bool{}
	for _, record := range saved.Records {
		c := record.Contract
		if c.ProjectID != p.ID || c.ID == "" || c.Digest != reviewDigest(c.Input) || c.Revision < 1 || c.Revision > 2 || c.State != "prepared" && c.State != "approved" || ids[c.ID] || keys[record.KeyDigest+":"+c.Actor.ID] || len(record.KeyDigest) != 64 || len(record.RequestDigest) != 64 || c.Actor.ID == "" {
			return "", expected, ErrProjectRunInvalid
		}
		if c.State == "approved" && (c.Revision != 2 || c.Approval == nil || c.Approval.Digest != c.Digest || c.Approval.Actor.ID != c.Actor.ID) || c.State == "prepared" && (c.Revision != 1 || c.Approval != nil) {
			return "", expected, ErrProjectRunInvalid
		}
		ids[c.ID], keys[record.KeyDigest+":"+c.Actor.ID] = true, true
	}
	return path, saved, nil
}

func writeOrchestrationContracts(path string, ledger orchestrationContractLedger) error {
	if len(ledger.Records) > orchestrationContractLimit {
		return ErrProjectRunLimit
	}
	// durableRunJSON writes indented JSON plus a newline. Check those exact
	// bytes before publication so a successful write is always readable again.
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(data)+1 > orchestrationContractFileLimit {
		return ErrProjectRunLimit
	}
	return durableRunJSON(path, ledger)
}

func orchestrationAuthorityControls(ctx context.Context) (OrchestrationControls, error) {
	_, policy, ok := management.ActorFromContext(ctx)
	if !ok {
		return OrchestrationControls{}, errCoordinatorAuthority
	}
	out := OrchestrationControls{AuthorityPolicySource: "workspace-private-policy", AuthorityPolicyDigest: reviewDigest(policy), ExecutionLimits: OrchestrationExecutionLimits{MonthlyBudgetUSD: policy.MonthlyBudgetUSD, SessionMemoryMiB: policy.SessionMemoryMiB, SessionCPUMillis: policy.SessionCPUMillis, MaxSessionsPerUser: policy.MaxSessionsPerUser, MaxConcurrentChecksPerUser: policy.MaxConcurrentChecksPerUser, AllowCloudProvisioning: policy.AllowCloudProvisioning, AllowPublicPublishing: policy.AllowPublicPublishing}}
	if team, ok := management.TeamAccessFromContext(ctx); ok {
		out.AuthorityPolicySource = "team-private-policy"
		out.TeamID, out.TeamRevision, out.MemberRevision, out.ProjectGrantRevision = team.Team.ID, team.Team.Revision, team.Member.Revision, team.ProjectRevision
	}
	return out, nil
}

func orchestrationAuthorityMatchesContext(ctx context.Context, saved OrchestrationControls) bool {
	current, err := orchestrationAuthorityControls(ctx)
	return err == nil && current.AuthorityPolicySource == saved.AuthorityPolicySource && current.AuthorityPolicyDigest == saved.AuthorityPolicyDigest && current.TeamID == saved.TeamID && current.TeamRevision == saved.TeamRevision && current.MemberRevision == saved.MemberRevision && current.ProjectGrantRevision == saved.ProjectGrantRevision
}

// At actual admission/dispatch the gateway supplies a fresh management
// authority, never the earlier request's role or policy snapshot.
func OrchestrationAuthorityMatches(run ProjectRun, authority management.ExecutionAuthority) bool {
	if run.Input.OrchestrationContract == nil {
		return true
	}
	saved := run.Input.OrchestrationContract.Input.Controls
	if reviewDigest(authority.Policy) != saved.AuthorityPolicyDigest {
		return false
	}
	if authority.Team == nil {
		return saved.TeamID == ""
	}
	team := authority.Team
	return team.Team.ID == saved.TeamID && team.Team.Revision == saved.TeamRevision && team.Member.Revision == saved.MemberRevision && team.ProjectRevision == saved.ProjectGrantRevision
}

func (s *Server) orchestrationSavedControls(p project.Project) (OrchestrationControls, error) {
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return OrchestrationControls{}, err
	}
	s.swarmScheduler.mu.Lock()
	scheduler, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return OrchestrationControls{}, err
	}
	return OrchestrationControls{CoordinatorPolicyRevision: policy.Revision, CoordinatorPolicyDigest: reviewDigest(policy), SchedulerPolicyRevision: scheduler.PolicyRevision, SchedulerPolicyDigest: reviewDigest(scheduler.Policy), SchedulerPolicy: scheduler.Policy}, nil
}

func (s *Server) orchestrationControls(ctx context.Context, p project.Project) (OrchestrationControls, error) {
	authority, err := orchestrationAuthorityControls(ctx)
	if err != nil {
		return OrchestrationControls{}, err
	}
	saved, err := s.orchestrationSavedControls(p)
	if err != nil {
		return OrchestrationControls{}, err
	}
	saved.AuthorityPolicySource, saved.AuthorityPolicyDigest, saved.ExecutionLimits, saved.TeamID = authority.AuthorityPolicySource, authority.AuthorityPolicyDigest, authority.ExecutionLimits, authority.TeamID
	saved.TeamRevision, saved.MemberRevision, saved.ProjectGrantRevision = authority.TeamRevision, authority.MemberRevision, authority.ProjectGrantRevision
	return saved, nil
}

func (s *Server) prepareOrchestrationInput(ctx context.Context, p project.Project, req OrchestrationPrepareRequest) (OrchestrationPreparedInput, error) {
	var out OrchestrationPreparedInput
	if err := s.OrchestratorExecutionGate(p.ID); err != nil {
		return out, err
	}
	if s.projectRunSelectionResolver == nil {
		return out, errors.New("The saved agent connection is unavailable.")
	}
	selected, err := s.projectRunSelectionResolver(ctx, ProjectRunSelectionRequest{ProjectID: p.ID, WorktreeID: req.WorktreeID, Harness: req.Route.Harness, Model: req.Route.Model, PermissionMode: req.Route.PermissionMode, Effort: req.Route.Effort, AgentConnectionID: req.Route.AgentConnectionID})
	if err != nil {
		return out, err
	}
	if selected.ProjectID != p.ID || selected.WorktreeID != req.WorktreeID || selected.Connection.ID == "" || selected.Connection.Kind == "legacy" && !(selected.AuthMode == "chatgpt" && selected.CodexConnectionID != "") {
		return out, errors.New("Save an identifiable agent connection before preparing an execution contract.")
	}
	implementation, err := s.BindImplementationContext(ctx, req.ImplementationContext, selected.WorktreeID, selected.Path)
	if err != nil {
		return out, err
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	var approvals ProjectRunApprovalSelection
	var binding projectRequirementBinding
	if err == nil {
		approvals, err = projectRunApprovalSelection(state)
	}
	if err == nil {
		binding, err = bindProjectRequirements(state, p.ID, implementation.RequirementIDs, &implementation.SpecificationRevision, &implementation.DesignPlanRevision, true)
	}
	s.mu.RUnlock()
	if err != nil {
		return out, err
	}
	plan := currentPlan(state, p.ID)
	selectedTasks := map[string]bool{}
	for _, id := range implementation.TaskIDs {
		selectedTasks[id] = true
	}
	tasks := []ProjectTask{}
	for _, task := range plan.Tasks {
		if selectedTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	controls, err := s.orchestrationControls(ctx, p)
	if err != nil {
		return out, err
	}
	standards := OrchestrationStandards{Status: "not-configured", Source: "none"}
	notes := ""
	designRevision := 0
	if state.DesignPlan != nil {
		notes, designRevision = state.DesignPlan.ImplementationNotes, state.DesignPlan.Revision
	}
	if strings.TrimSpace(notes) != "" || strings.TrimSpace(state.Brief.Constraints) != "" {
		standards = OrchestrationStandards{Status: "legacy-unassessed", Source: "approved-freeform-context", Revision: designRevision, Digest: reviewDigest(struct{ Constraints, Notes string }{state.Brief.Constraints, notes})}
	}
	if team, scoped := management.TeamAccessFromContext(ctx); scoped {
		standards.TeamID = team.Team.ID
		library, readErr := s.currentTeamLibrary(ctx)
		if readErr != nil {
			return out, readErr
		}
		if library != nil {
			standards = OrchestrationStandards{Status: "team-library-unassessed", Source: "team-control-library", Revision: library.Revision, Digest: library.Digest, TeamID: team.Team.ID}
			_, assessment, readErr := s.readControlAssessment(p)
			if readErr != nil {
				return out, readErr
			}
			if assessment.Revision > 0 && assessment.TeamID == team.Team.ID && assessment.LibraryRevision == library.Revision && assessment.LibraryDigest == library.Digest && assessment.ContextDigest == assessmentContextDigest(state) {
				standards.Status = "team-library-assessed"
				standards.AssessmentRevision = assessment.Revision
				standards.AssessmentDigest = reviewDigest(assessment)
			}
		}
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return out, err
	}
	routine := "review-required"
	if policy.ExecutionMode == "reviewed-tasks" {
		routine = "saved-reviewed-tasks-policy"
	}
	placement := req.Route.ExecutionLocation
	if placement == "local-docker" {
		placement = ""
	} // existing run input convention
	out = OrchestrationPreparedInput{Implementation: *implementation, Approvals: approvals, Route: ProjectRunRoute{Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, Effort: selected.Effort, ExecutionLocation: placement, Connection: selected.Connection}, Outcome: req.Outcome, Requirements: binding.RequirementSnapshots, Tasks: tasks, Controls: controls, Standards: standards, DecisionPolicy: OrchestrationDecisionPolicy{Routine: routine, HumanInput: "outside saved authority, contradictory intent, or missing evidence"}}
	return out, nil
}

func (s *Server) orchestrationFreshness(p project.Project, c OrchestrationContract) ProjectRunContextFreshness {
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return ProjectRunContextFreshness{Status: "unavailable", Reason: "Current approved Design could not be read."}
	}
	current, err := implementationSnapshot(state, *ImplementationContextSelection(&c.Input.Implementation), c.Input.Implementation.WorktreeID)
	if err == nil {
		previous := c.Input.Implementation.Clone()
		previous.SourceCommit, previous.SourceDigest, previous.SourceDirty = "", "", false
		if implementationDigest(current) != implementationDigest(previous) {
			err = ErrProjectRunConflict
		}
	}
	if err == nil {
		var approvals ProjectRunApprovalSelection
		approvals, err = projectRunApprovalSelection(state)
		if err == nil && approvals != c.Input.Approvals {
			err = ErrProjectRunConflict
		}
	}
	if err == nil {
		standards := c.Input.Standards
		if c.Input.Controls.TeamID != standards.TeamID {
			err = ErrProjectRunConflict
		}
		if standards.TeamID != "" {
			var library *TeamControlRevision
			library, err = s.teamLibraryForID(standards.TeamID)
			if err == nil {
				if library == nil {
					if standards.Source == "team-control-library" {
						err = ErrProjectRunConflict
					}
				} else if standards.Source != "team-control-library" || standards.Revision != library.Revision || standards.Digest != library.Digest {
					err = ErrProjectRunConflict
				}
			}
			if err == nil && standards.AssessmentRevision > 0 {
				var assessment ControlAssessment
				_, assessment, err = s.readControlAssessment(p)
				if err == nil && (assessment.Revision != standards.AssessmentRevision || reviewDigest(assessment) != standards.AssessmentDigest || assessment.ContextDigest != assessmentContextDigest(state)) {
					err = ErrProjectRunConflict
				}
			} else if err == nil && standards.Source == "team-control-library" {
				var assessment ControlAssessment
				_, assessment, err = s.readControlAssessment(p)
				if err == nil && library != nil && assessment.Revision > 0 && assessment.TeamID == standards.TeamID && assessment.LibraryRevision == library.Revision && assessment.LibraryDigest == library.Digest && assessment.ContextDigest == assessmentContextDigest(state) {
					err = ErrProjectRunConflict
				}
			}
		}
	}
	if err == nil {
		var controls OrchestrationControls
		controls, err = s.orchestrationSavedControls(p)
		if err == nil && (controls.CoordinatorPolicyRevision != c.Input.Controls.CoordinatorPolicyRevision || controls.CoordinatorPolicyDigest != c.Input.Controls.CoordinatorPolicyDigest || controls.SchedulerPolicyRevision != c.Input.Controls.SchedulerPolicyRevision || controls.SchedulerPolicyDigest != c.Input.Controls.SchedulerPolicyDigest || controls.SchedulerPolicy != c.Input.Controls.SchedulerPolicy) {
			err = ErrProjectRunConflict
		}
	}
	if err != nil {
		return ProjectRunContextFreshness{Status: "changed", Reason: "Approved Design, plan, team control or resource policy changed. Prepare a new contract before starting more work."}
	}
	return ProjectRunContextFreshness{Status: "current", Reason: "Approved Design and saved controls match. Source, route and actor authority are checked before each effect."}
}

func orchestrationContractRecordByID(ledger orchestrationContractLedger, id, actorID string) (*OrchestrationContract, error) {
	for _, row := range ledger.Records {
		if row.Contract.ID == id && row.Contract.Actor.ID == actorID {
			c := row.Contract
			return &c, nil
		}
	}
	return nil, ErrProjectRunNotFound
}

func (s *Server) contractView(ctx context.Context, p project.Project, c OrchestrationContract) OrchestrationContractView {
	freshness := s.orchestrationFreshness(p, c)
	if freshness.Status == "current" && !orchestrationAuthorityMatchesContext(ctx, c.Input.Controls) {
		freshness = ProjectRunContextFreshness{Status: "changed", Reason: "Current team grant or execution policy changed. Review and prepare a new contract."}
	}
	selectedPath := ""
	if freshness.Status == "current" {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		selected, err := s.orchestrationSelectedRoute(bounded, c)
		if err != nil {
			freshness = ProjectRunContextFreshness{Status: "changed", Reason: "The selected worktree or saved agent connection changed. Prepare a new contract."}
		} else {
			selectedPath = selected.Path
		}
	}
	if freshness.Status == "current" {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		digest, commit, dirty, err := s.checkSource(bounded, selectedPath)
		saved := c.Input.Implementation
		if err != nil {
			freshness = ProjectRunContextFreshness{Status: "unavailable", Reason: "The selected source could not be inspected. Approval and new work remain paused."}
		} else if digest != saved.SourceDigest || commit != saved.SourceCommit || dirty != saved.SourceDirty {
			freshness = ProjectRunContextFreshness{Status: "changed", Reason: "The selected source changed. Prepare a new contract before starting more work."}
		}
	}
	return OrchestrationContractView{Contract: c, Freshness: freshness}
}

func normalizeOrchestrationPrepare(req *OrchestrationPrepareRequest) error {
	if !validRunKey(req.IdempotencyKey) || req.WorktreeID == "" || req.Outcome != "preview" && req.Outcome != "publish" || req.Route.ExecutionLocation != "local-docker" && req.Route.ExecutionLocation != "cloud-microvm" {
		return ErrProjectRunInvalid
	}
	sort.Strings(req.ImplementationContext.TaskIDs)
	if req.Route.PermissionMode == "" {
		req.Route.PermissionMode = "ask"
	}
	if req.Route.Harness == "" {
		req.Route.Harness = "claude-code"
	}
	if req.Route.ExecutionLocation == "cloud-microvm" && (req.Route.PermissionMode != "bypass" || req.Route.Harness != "codex" && req.Route.Harness != "claude-code") {
		return ErrProjectRunInvalid
	}
	return nil
}

func (s *Server) contractApprovedForRun(ctx context.Context, p project.Project, selection OrchestrationContractSelection, input ProjectRunInput) (*OrchestrationContract, error) {
	actor, ok := authenticatedContextActor(ctx)
	if !ok {
		return nil, ErrProjectRunConflict
	}
	s.orchestrationContractMu.Lock()
	_, ledger, err := s.readOrchestrationContracts(p)
	s.orchestrationContractMu.Unlock()
	if err != nil {
		return nil, err
	}
	c, err := orchestrationContractRecordByID(ledger, selection.ID, actor.ID)
	if err != nil {
		return nil, err
	}
	if c.State != "approved" || c.Revision != selection.Revision || c.Digest != selection.Digest || c.Approval == nil || c.Approval.ID != selection.ApprovalID || s.contractView(ctx, p, *c).Freshness.Status != "current" {
		return nil, ErrProjectRunConflict
	}
	if !reflect.DeepEqual(c.Input.Implementation, input.Implementation) || c.Input.Approvals != input.Approvals || c.Input.Route != input.Route || c.Input.Outcome != input.Outcome || reviewDigest(c.Input.Requirements) != reviewDigest(input.Requirements) {
		return nil, ErrProjectRunConflict
	}
	return c, nil
}

func (s *Server) orchestrationSelectedRoute(ctx context.Context, c OrchestrationContract) (ProjectRunSelection, error) {
	if s.projectRunSelectionResolver == nil {
		return ProjectRunSelection{}, ErrProjectRunConflict
	}
	route := c.Input.Route
	id := route.Connection.ID
	selected, err := s.projectRunSelectionResolver(ctx, ProjectRunSelectionRequest{ProjectID: c.ProjectID, WorktreeID: c.Input.Implementation.WorktreeID, Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode, Effort: route.Effort, AgentConnectionID: &id})
	if err != nil || selected.ProjectID != c.ProjectID || selected.WorktreeID != c.Input.Implementation.WorktreeID || selected.Harness != route.Harness || selected.Model != route.Model || selected.PermissionMode != route.PermissionMode || selected.Effort != route.Effort || selected.Connection != route.Connection {
		return ProjectRunSelection{}, ErrProjectRunConflict
	}
	return selected, nil
}

func (s *Server) orchestrationSourceCurrent(ctx context.Context, c OrchestrationContract) bool {
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	selected, err := s.orchestrationSelectedRoute(bounded, c)
	if err != nil {
		return false
	}
	digest, commit, dirty, err := s.checkSource(bounded, selected.Path)
	saved := c.Input.Implementation
	return err == nil && digest == saved.SourceDigest && commit == saved.SourceCommit && dirty == saved.SourceDirty
}

func (s *Server) contractCurrentForRun(p project.Project, run ProjectRun) error {
	if run.Input.OrchestrationContract == nil {
		return nil
	}
	s.orchestrationContractMu.Lock()
	_, ledger, err := s.readOrchestrationContracts(p)
	s.orchestrationContractMu.Unlock()
	if err != nil {
		return err
	}
	saved := *run.Input.OrchestrationContract
	for _, record := range ledger.Records {
		c := record.Contract
		if c.ID == saved.ID && reflect.DeepEqual(c, saved) && c.Approval != nil {
			if s.orchestrationFreshness(p, c).Status == "current" {
				return nil
			}
		}
	}
	return ErrProjectRunConflict
}

// ValidateRunExecutionContract is the final new-effect gate used by the API
// allocator. It re-reads the private approval and current reviewed controls;
// an old attempt remains immutable even when this gate now refuses a new one.
func (s *Server) ValidateRunExecutionContract(projectID string, run ProjectRun) error {
	if run.ProjectID != projectID {
		return ErrProjectRunConflict
	}
	if run.Input.OrchestrationContract == nil {
		return nil
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	return s.contractCurrentForRun(p, run)
}

// The coding agent receives a bounded, nonsecret projection inside LUCI.md.
// LUCI.md is an existing ReviewedContextCapture file, so its exact delivered
// bytes enter the session-owned manifest and remain immutable on resume.
// Actor identity, endpoint, credentials and private policy stay in the ledger.
func appendRunExecutionContract(capture *ReviewedContextCapture, run ProjectRun, taskID string) error {
	if run.Input.OrchestrationContract == nil {
		return nil
	}
	c := run.Input.OrchestrationContract
	brief := struct {
		Version        int                   `json:"version"`
		ContractID     string                `json:"contractId"`
		ContractDigest string                `json:"contractDigest"`
		RunID          string                `json:"runId"`
		TaskID         string                `json:"taskId,omitempty"`
		Outcome        string                `json:"outcome"`
		Requirements   []RequirementSnapshot `json:"requirements"`
		Tasks          []ProjectTask         `json:"tasks"`
		Controls       struct {
			AuthorityPolicyDigest     string                       `json:"authorityPolicyDigest"`
			ExecutionLimits           OrchestrationExecutionLimits `json:"executionLimits"`
			CoordinatorPolicyRevision int                          `json:"coordinatorPolicyRevision"`
			CoordinatorPolicyDigest   string                       `json:"coordinatorPolicyDigest"`
			SchedulerPolicyRevision   int                          `json:"schedulerPolicyRevision"`
			SchedulerPolicyDigest     string                       `json:"schedulerPolicyDigest"`
			SchedulerPolicy           SwarmSchedulerPolicy         `json:"schedulerPolicy"`
		} `json:"controls"`
		Standards      OrchestrationStandards      `json:"standards"`
		DecisionPolicy OrchestrationDecisionPolicy `json:"decisionPolicy"`
	}{Version: 1, ContractID: c.ID, ContractDigest: c.Digest, RunID: run.ID, TaskID: taskID, Outcome: c.Input.Outcome, Requirements: c.Input.Requirements, Tasks: c.Input.Tasks, Standards: c.Input.Standards, DecisionPolicy: c.Input.DecisionPolicy}
	brief.Controls.AuthorityPolicyDigest = c.Input.Controls.AuthorityPolicyDigest
	brief.Controls.ExecutionLimits = c.Input.Controls.ExecutionLimits
	brief.Controls.CoordinatorPolicyRevision = c.Input.Controls.CoordinatorPolicyRevision
	brief.Controls.CoordinatorPolicyDigest = c.Input.Controls.CoordinatorPolicyDigest
	brief.Controls.SchedulerPolicyRevision = c.Input.Controls.SchedulerPolicyRevision
	brief.Controls.SchedulerPolicyDigest = c.Input.Controls.SchedulerPolicyDigest
	brief.Controls.SchedulerPolicy = c.Input.Controls.SchedulerPolicy
	data, err := json.MarshalIndent(brief, "", "  ")
	if err != nil || len(data) > 512<<10 {
		return errors.New("The reviewed execution contract is too large for this agent context. Split the selected work into smaller reviewed tasks.")
	}
	addition := []byte("\n\n## Approved execution contract\n\nThe following exact reviewed scope is context, not permission to expand authority or declare completion. Work only within the assigned task and Ask/worker permissions. A worker report is evidence, not an accepted result.\n\n```json\n")
	addition = append(addition, data...)
	addition = append(addition, []byte("\n```\n")...)
	if capture == nil || capture.Binding == nil {
		return ErrProjectRunInvalid
	}
	for i := range capture.Files {
		if capture.Files[i].Path != "LUCI.md" {
			continue
		}
		total := len(addition)
		for _, file := range capture.Files {
			total += len(file.Data)
		}
		if total > ReviewedContextBytesLimit {
			return errors.New("The reviewed documents and approved execution contract exceed the captured context limit.")
		}
		capture.Files[i].Data = append(capture.Files[i].Data, addition...)
		return nil
	}
	return errors.New("The reviewed context has no LUCI.md file for its approved execution contract.")
}

// Project runs and the existing scheduler remain the only owners of task
// allocation and dispatch.
