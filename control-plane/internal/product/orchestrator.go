package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func ValidateOrchestratorAnswer(snapshot OrchestratorContext, answer OrchestratorAnswer) error {
	return validateOrchestratorAnswer(snapshot, answer, true)
}

// Stored answers were accepted under the rules in force when they completed.
// Keep their bounded structure readable without granting an old proposal new
// application authority; every completion and application uses the current
// quality rules above.
func validateStoredOrchestratorAnswer(snapshot OrchestratorContext, answer OrchestratorAnswer) error {
	return validateOrchestratorAnswer(snapshot, answer, false)
}

func validateOrchestratorAnswer(snapshot OrchestratorContext, answer OrchestratorAnswer, currentQuality bool) error {
	raw, err := json.MarshalIndent(answer, "", "  ")
	if err != nil || len(raw) > orchestratorAnswerLimit {
		return ErrOrchestratorInvalid
	}
	if strings.TrimSpace(answer.Summary) == "" || len(answer.Summary) > 8000 || len(answer.Question) > 4000 || len(answer.Rationale) > 8000 || len(answer.Options) > 6 || len(answer.Impact) > 12 || len(answer.NextAction) > 100 || (answer.Blocking && strings.TrimSpace(answer.Question) == "") || (currentQuality && answer.Blocking && utf8.RuneCountInString(strings.TrimSpace(answer.Question)) < 8) || (currentQuality && !answer.Blocking && strings.TrimSpace(answer.Question) != "") {
		return ErrOrchestratorInvalid
	}
	for _, option := range answer.Options {
		if strings.TrimSpace(option) == "" || len(option) > 1000 {
			return ErrOrchestratorInvalid
		}
	}
	for _, impact := range answer.Impact {
		if strings.TrimSpace(impact) == "" || len(impact) > 2000 {
			return ErrOrchestratorInvalid
		}
	}
	if answer.Proposal == nil {
		return nil
	}
	proposal := answer.Proposal
	count := 0
	for _, present := range []bool{proposal.Specification != nil, proposal.DesignPlan != nil, proposal.Plan != nil} {
		if present {
			count++
		}
	}
	if count != 1 || strings.TrimSpace(proposal.Summary) == "" || len(proposal.Summary) > 8000 {
		return ErrOrchestratorInvalid
	}
	switch proposal.Target {
	case "specification":
		next := proposal.Specification
		if next == nil || next.ProjectID != snapshot.ProjectID || next.Revision != snapshot.Specification.Revision || next.BriefRevision != snapshot.Specification.BriefRevision || next.Status != "draft" || next.Digest != "" {
			return ErrOrchestratorInvalid
		}
		if currentQuality {
			for _, requirement := range next.Requirements {
				criterion := strings.TrimSpace(requirement.Acceptance)
				if utf8.RuneCountInString(criterion) < 12 || strings.EqualFold(criterion, requirement.Priority) || strings.EqualFold(criterion, requirement.Title) {
					return ErrOrchestratorInvalid
				}
			}
		}
		return validateSpecification(*next)
	case "design-plan":
		next := proposal.DesignPlan
		if next == nil || next.ProjectID != snapshot.ProjectID || next.Revision != snapshot.DesignPlan.Revision || next.BriefRevision != snapshot.DesignPlan.BriefRevision || next.SpecificationRevision != snapshot.DesignPlan.SpecificationRevision || next.SketchRevision != snapshot.DesignPlan.SketchRevision || next.Status != "draft" || next.Digest != "" || next.PrototypeReleaseID != snapshot.DesignPlan.PrototypeReleaseID || next.PrototypeCommit != snapshot.DesignPlan.PrototypeCommit {
			return ErrOrchestratorInvalid
		}
		if err := validateDesignPlan(*next); err != nil {
			return err
		}
		if snapshot.TeamStandards != nil && (next.ControlLibraryRevision != 0 && next.ControlLibraryRevision != snapshot.TeamStandards.Revision || next.ControlLibraryDigest != "" && next.ControlLibraryDigest != snapshot.TeamStandards.Digest) {
			return ErrOrchestratorInvalid
		}
		if currentQuality && snapshot.DesignPlan.Revision == 0 && (next.Scene == nil || len(next.Scene.Frames) == 0) {
			return ErrOrchestratorInvalid
		}
		if currentQuality && designSceneOmission(snapshot.DesignPlan, *next) != nil {
			return ErrOrchestratorInvalid
		}
		return validateDesignLinks(*next, snapshot.Specification)
	case "plan":
		next := proposal.Plan
		if next == nil || next.ProjectID != snapshot.ProjectID || next.Revision != snapshot.Plan.Revision || next.BriefRevision != snapshot.Plan.BriefRevision {
			return ErrOrchestratorInvalid
		}
		if err := validatePlan(*next); err != nil {
			return err
		}
		requirements := map[string]bool{}
		for _, r := range snapshot.Specification.Requirements {
			requirements[r.ID] = true
		}
		old := map[string]string{}
		for _, task := range snapshot.Plan.Tasks {
			old[task.ID] = task.Status
		}
		for _, task := range next.Tasks {
			status, exists := old[task.ID]
			if (!exists && task.Status != "todo") || (exists && task.Status != status) {
				return errors.New("An assistant amendment cannot mark implementation tasks complete.")
			}
			for _, id := range task.RequirementIDs {
				if !requirements[id] {
					return ErrOrchestratorInvalid
				}
			}
		}
		return nil
	default:
		return ErrOrchestratorInvalid
	}
}

func (s *Server) registerOrchestrator(mux Registrar) {
	// These handlers own their read/exclusive source admission; do not upgrade
	// the outer source registrar's shared lock when applying an amendment.
	if wrapped, ok := mux.(*branchSourceRegistrar); ok {
		mux = wrapped.base
	}
	s.registerCoordination(mux)
	s.registerCoordinatorContinuations(mux)
	mux.HandleFunc("GET /v1/product/projects/{id}/orchestrator/policy", s.getCoordinatorPolicy)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/policy", s.saveCoordinatorPolicy)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/turns/{turnId}/advance", s.advanceOrchestrator)
	mux.HandleFunc("GET /v1/product/projects/{id}/orchestrator", s.getOrchestrator)
	mux.HandleFunc("GET /v1/product/projects/{id}/orchestrator/export", s.exportOrchestrator)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/rotate", s.rotateOrchestrator)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/turns/{turnId}/apply", s.applyOrchestrator)
}
func orchestratorNext(context OrchestratorContext, questions []OrchestratorQuestion) OrchestratorAction {
	action := func(id, label, reason, lens string) OrchestratorAction {
		return OrchestratorAction{ID: id, Label: label, Reason: reason, Lens: lens}
	}
	runAction := func(runID, id, label, reason, lens string) OrchestratorAction {
		value := action(id, label, reason, lens)
		value.RunID = runID
		return value
	}
	for _, run := range context.ActiveRuns {
		switch run.Activity {
		case "running":
			return runAction(run.ID, "inspect-run", "Inspect active builds", "An agent was last recorded as working on this build. Open its run to check progress or stop it.", "develop")
		case "unknown":
			return runAction(run.ID, "reconcile-run", "Confirm build status", "The last attempt has no confirmed outcome. Open its run and check its current status before starting another attempt.", "develop")
		}
	}
	if len(questions) > 0 {
		return action("answer-question", "Answer the open question", questions[0].Rationale, "discover")
	}
	for _, run := range context.ActiveRuns {
		if run.Activity == "pending" {
			return runAction(run.ID, "inspect-run", "Review prepared work", "This coding work is prepared but has not started. Review its agreed scope and approvals, then continue or cancel it.", "develop")
		}
	}
	for _, run := range context.NeedsAttention {
		if run.Kind == "task-question" {
			return projectRunAttentionAction(run)
		}
		// The current run retains its existing stage/context guidance below.
		// Older work must not disappear merely because a newer plan exists.
		if context.CurrentRun == nil || run.ID != context.CurrentRun.ID {
			return projectRunAttentionAction(run)
		}
		if run.Kind == "task-review" && run.ContextStatus == "current" {
			return runAction(run.ID, "inspect-run", "Review task changes", "Retained agent changes need explicit integration review or retirement. Process exit or cancellation of future dispatch does not resolve that review.", "develop")
		}
	}
	if context.Specification.Revision == 0 {
		return action("define", "Shape your idea", "Clarify the people, outcome and measurable requirements.", "discover")
	}
	if context.Specification.Status != "approved" {
		return action("review-definition", "Review the definition", "Resolve open questions and review the saved definition before implementation.", "discover")
	}
	if context.DesignPlan.Revision == 0 {
		return action("design", "Plan the experience", "Connect the reviewed requirements to screens, states and accessibility.", "design")
	}
	if context.DesignPlan.Status != "approved" {
		return action("review-design", "Review the design", "Review the saved design against the current definition and sketch.", "design")
	}
	if len(context.Plan.Tasks) == 0 || context.Plan.BriefRevision != context.Brief.Revision || context.Plan.SpecificationRevision != context.Specification.Revision || context.Plan.SpecificationDigest != context.Specification.Digest {
		return action("plan", "Review implementation tasks", "Bind a current task plan to the reviewed project definition.", "develop")
	}
	if run := context.CurrentRun; run != nil {
		if freshness := run.ContextFreshness; freshness != nil && freshness.Status != "current" {
			return action("plan", "Review current plan", freshness.Reason, "develop")
		}
		switch run.State {
		case "coding":
			return runAction(run.ID, "inspect-run", "Follow implementation", "Observe the existing coordinated session; do not start a duplicate.", "develop")
		case "awaiting-checks":
			return runAction(run.ID, "check", "Verify the implementation", "Run and join independent checks for the exact reviewed source.", "develop")
		case "awaiting-release":
			return runAction(run.ID, "deploy", "Review verified build", "Review this build and its exact checks, then prepare its preview or deployment and join the release evidence.", "deploy")
		case "complete":
			if run.Valid {
				return runAction(run.ID, "maintain", "Inspect releases and health", "Review current release health and the next amendment; historical acceptance does not guarantee uptime.", "deploy")
			}
			return runAction(run.ID, "inspect-run", "Review current run evidence", run.Reason, "develop")
		case "blocked":
			return runAction(run.ID, "inspect-run", "Resolve the run's next step", run.Reason, "develop")
		}
	}
	return action("start-coding", "Prepare implementation", "Review the current worktree, tasks, provider and outcome, then start one coordinated run.", "develop")
}
func (s *Server) OrchestratorView(ctx context.Context, id string, before int) (OrchestratorView, error) {
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return OrchestratorView{}, err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return OrchestratorView{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return OrchestratorView{}, err
	}
	snapshot, err := s.orchestratorSnapshotLocked(ctx, p)
	if err != nil {
		return OrchestratorView{}, err
	}
	questions := orchestratorQuestions(saved)
	end := len(saved.Turns)
	if before > 0 {
		for i, turn := range saved.Turns {
			if turn.Sequence >= before {
				end = i
				break
			}
		}
	}
	if end < 0 {
		end = 0
	}
	start := end - 20
	if start < 0 {
		start = 0
	}
	turns := []OrchestratorTurn{}
	for _, turn := range saved.Turns[start:end] {
		turns = append(turns, turn.OrchestratorTurn)
	}
	storage, err := s.orchestratorStorageLocked(p, saved)
	if err != nil {
		return OrchestratorView{}, err
	}
	var activity *OrchestratorActivity
	if actor, ok := authenticatedContextActor(ctx); ok {
		activity, err = s.orchestratorActivityLocked(p, actor.ID)
		if err != nil {
			return OrchestratorView{}, err
		}
	}
	cursor := 0
	if len(turns) > 0 {
		cursor = turns[0].Sequence
	}
	return OrchestratorView{StorageDigest: storage.StorageDigest, ArchivePresent: len(storage.Archive) > 0, ProjectID: id, Revision: saved.Revision, Context: snapshot, Turns: turns, NextAction: orchestratorNext(snapshot, questions), BlockingQuestions: questions, HasEarlier: start > 0, BeforeSequence: cursor, Activity: activity}, nil
}
func (s *Server) getOrchestrator(w http.ResponseWriter, r *http.Request) {
	before, err := strconv.Atoi(r.URL.Query().Get("before"))
	if r.URL.Query().Get("before") == "" {
		before, err = 0, nil
	}
	if err != nil || before < 0 {
		journeyError(w, 400, "invalid_cursor", "Choose a valid earlier conversation page.")
		return
	}
	view, err := s.OrchestratorView(r.Context(), r.PathValue("id"), before)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func orchestratorProblem(w http.ResponseWriter, err error) {
	status, code, message := 500, "orchestrator_unavailable", "The private project conversation could not be read or saved. Reload before retrying."
	switch {
	case errors.Is(err, ErrTeamStandardsTooLarge):
		status, code, message = 413, "team_standards_too_large", err.Error()
	case errors.Is(err, errControlWorkbook):
		status, code, message = 409, "team_standards_unavailable", "The current team control library cannot be verified. Review the imported library before asking Luci to design or apply a draft."
	case errors.Is(err, ErrOrchestratorInvalid):
		status, code, message = 400, "invalid_orchestrator_request", err.Error()
	case errors.Is(err, ErrOrchestratorConflict):
		status, code, message = 409, "revision_conflict", err.Error()
	case errors.Is(err, ErrOrchestratorOpposition):
		status, code, message = 409, "conflict", err.Error()
	case errors.Is(err, ErrOrchestratorBusy):
		status, code, message = 409, "orchestrator_busy", err.Error()
	case errors.Is(err, ErrOrchestratorCapacity):
		status, code, message = 409, "orchestrator_capacity", err.Error()
	case errors.Is(err, ErrOrchestratorQuestion):
		status, code, message = 409, "orchestrator_question", err.Error()
	}
	journeyError(w, status, code, message)
}
func (s *Server) applyOrchestrator(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision      int    `json:"expectedRevision"`
		ExpectedContextDigest string `json:"expectedContextDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if err := s.applyOrchestratorProposal(r.Context(), r.PathValue("id"), r.PathValue("turnId"), req.ExpectedRevision, req.ExpectedContextDigest); err != nil {
		orchestratorProblem(w, err)
		return
	}
	view, err := s.OrchestratorView(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func targetSnapshot(state journeyState, target string) (int, any) {
	switch target {
	case "specification":
		if state.Specification != nil {
			return state.Specification.Revision, *state.Specification
		}
	case "design-plan":
		if state.DesignPlan != nil {
			return state.DesignPlan.Revision, *state.DesignPlan
		}
	case "plan":
		if state.Plan != nil {
			return state.Plan.Revision, *state.Plan
		}
	}
	return 0, nil
}

type orchestratorProposalGuard func(project.Project, orchestratorLedger, storedOrchestratorTurn) error

func (s *Server) applyOrchestratorProposal(ctx context.Context, id, turnID string, expected int, digest string) error {
	return s.applyOrchestratorProposalGuarded(ctx, id, turnID, expected, digest, nil)
}
func (s *Server) applyOrchestratorProposalGuarded(ctx context.Context, id, turnID string, expected int, digest string, authorize orchestratorProposalGuard) error {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return ErrOrchestratorInvalid
	}
	if expected < 0 || !hexDigest(digest) {
		return ErrOrchestratorInvalid
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return err
	}
	guard := s.branchGuard(id)
	if !guard.TryLock() {
		return ErrOrchestratorBusy
	}
	defer guard.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	index := -1
	for i := range saved.Turns {
		if saved.Turns[i].ID == turnID {
			index = i
			break
		}
	}
	if index < 0 {
		archived, err := s.archivedCoordinationTurnLocked(p, turnID)
		if err != nil {
			return err
		}
		if archived == nil || archived.AppliedRevision < 1 || archived.ApplyIntent != "" {
			return ErrOrchestratorConflict
		}
		// An archived applied origin can only confirm its immutable receipt.
		// Never append it to current history or replay its canonical mutation.
		if authorize != nil {
			if err := authorize(p, saved, *archived); err != nil {
				return err
			}
		}
		if err := s.validateCoordinationRecoveryLocked(ctx, p, *archived); err != nil {
			return err
		}
		return s.finishCoordinationApplyLocked(ctx, p, *archived)
	}
	turn := &saved.Turns[index]
	if authorize != nil {
		if err := authorize(p, saved, *turn); err != nil {
			return err
		}
	}
	if err := s.validateCoordinationRecoveryLocked(ctx, p, *turn); err != nil {
		return err
	}
	if turn.AppliedRevision > 0 {
		return s.finishCoordinationApplyLocked(ctx, p, *turn)
	}
	if turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Proposal == nil {
		return ErrOrchestratorInvalid
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	proposal := turn.Answer.Proposal
	// A crash after canonical persistence may leave only the private intent.
	// Finalize the same observed target once; never execute an amendment again.
	revision, current := targetSnapshot(state, proposal.Target)
	if turn.ApplyIntent != "" && revision == turn.ApplyTargetRevision && reviewDigest(current) == turn.ApplyTargetDigest {
		turn.AppliedRevision = revision
		turn.ApplyIntent = ""
		turn.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		saved.Revision++
		if err := s.saveOrchestrator(saved); err != nil {
			return err
		}
		return s.finishCoordinationApplyLocked(ctx, p, *turn)
	}
	if turn.ApplyIntent != "" {
		return ErrOrchestratorConflict
	}
	snapshot, err := s.orchestratorSnapshotLocked(ctx, p)
	if err != nil {
		return err
	}
	if expected != saved.Revision || digest != snapshot.Digest {
		return ErrOrchestratorConflict
	}
	answer, err := s.prepareCoordinationApplyLocked(ctx, p, *turn, snapshot)
	if err != nil {
		return err
	}
	proposal = answer.Proposal
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		return ErrOrchestratorInvalid
	}
	if proposal.Target == "design-plan" {
		if err := s.validateDesignAssetBindings(context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir), p.Path, *proposal.DesignPlan); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mutations := []contextMutation{}
	revoke, definition := false, false
	switch proposal.Target {
	case "specification":
		spec := *proposal.Specification
		// Omission preserves retained source anchors; an explicit empty array
		// remains the author's deliberate removal. Validate the exact target
		// against current private histories before changing canonical context.
		if turn.EvidenceExplicitEmpty {
			spec.Evidence = []DiscoveryEvidenceLink{}
		} else if spec.Evidence == nil && state.Specification != nil {
			spec.Evidence = append([]DiscoveryEvidenceLink(nil), state.Specification.Evidence...)
		}
		if err := validateDiscoveryEvidenceTargets(spec); err != nil {
			return err
		}
		if err := s.validateDiscoveryEvidenceLinks(ctx, p, spec.Evidence); err != nil {
			return err
		}
		spec.Status, spec.Digest = "draft", ""
		spec.Revision, spec.BriefRevision = snapshot.Specification.Revision+1, state.Brief.Revision+1
		spec.UpdatedAt = now
		criteria := []string{}
		for _, requirement := range spec.Requirements {
			text := requirement.Title
			if requirement.Acceptance != "" && requirement.Acceptance != text {
				text += " — " + requirement.Acceptance
			}
			criteria = append(criteria, text)
		}
		brief := Brief{ProjectID: id, Revision: spec.BriefRevision, Purpose: spec.Purpose, Audience: spec.Audience, Constraints: spec.Constraints, Criteria: criteria, Structured: true, UpdatedAt: now}
		state.Brief = brief
		state.BriefRevisions = append(state.BriefRevisions, brief)
		state.Specification = &spec
		state.SpecificationRevisions = append(state.SpecificationRevisions, spec)
		mutations = []contextMutation{{"specification", "save"}, {"brief", "save"}}
		revoke, definition = true, true
	case "design-plan":
		plan := *proposal.DesignPlan
		plan.CurrentControlLibraryRevision, plan.CurrentControlLibraryDigest = 0, ""
		if snapshot.TeamStandards != nil {
			plan.ControlLibraryRevision, plan.ControlLibraryDigest = snapshot.TeamStandards.Revision, snapshot.TeamStandards.Digest
		}
		plan.Status, plan.Digest = "draft", ""
		plan.Revision = snapshot.DesignPlan.Revision + 1
		plan.BriefRevision = state.Brief.Revision
		plan.SpecificationRevision = snapshot.Specification.Revision
		plan.SketchRevision = state.Design.Revision
		plan.UpdatedAt = now
		state.DesignPlan = &plan
		state.DesignPlanRevisions = append(state.DesignPlanRevisions, plan)
		mutations = []contextMutation{{"design", "save"}}
		revoke = true
	case "plan":
		plan := *proposal.Plan
		plan.Revision = snapshot.Plan.Revision + 1
		plan.BriefRevision = state.Brief.Revision
		plan.SpecificationRevision, plan.SpecificationDigest = 0, ""
		plan.UpdatedAt = now
		for _, task := range plan.Tasks {
			if len(task.RequirementIDs) > 0 {
				if snapshot.Specification.Status != "approved" {
					return ErrOrchestratorConflict
				}
				expectedSpec := snapshot.Specification.Revision
				binding, err := bindProjectRequirements(state, id, task.RequirementIDs, &expectedSpec, nil, false)
				if err != nil {
					return ErrOrchestratorConflict
				}
				plan.SpecificationRevision, plan.SpecificationDigest = binding.SpecificationRevision, binding.SpecificationDigest
			}
		}
		state.Plan = &plan
		mutations = []contextMutation{{"plan", "save"}}
	}
	revision, current = targetSnapshot(state, proposal.Target)
	turn.ApplyIntent = "applying"
	turn.ApplyTargetRevision, turn.ApplyTargetDigest = revision, reviewDigest(current)
	saved.Revision++
	if err := s.saveOrchestrator(saved); err != nil {
		return err
	}
	if revoke {
		if err := s.revokeJourneyApproval(p.Path, p.ID, definition); err != nil {
			return ErrOrchestratorInvalid
		}
	}
	if err := s.writeAttributedJourney(ctx, p.Path, state, mutations...); err != nil {
		return errors.New("The amendment may have been saved, but its exports or attribution are incomplete. Reload the conversation and Context storage before retrying.")
	}
	turn.AppliedRevision = revision
	turn.ApplyIntent = ""
	turn.UpdatedAt = now
	saved.Revision++
	if err := s.saveOrchestrator(saved); err != nil {
		return err
	}
	return s.finishCoordinationApplyLocked(ctx, p, *turn)
}

// Explicit caller lookup keeps an older material question in the model input
// even when it is outside the recent conversation window.
func (s *Server) OrchestratorTurn(id, turnID string) (OrchestratorTurn, error) {
	p, err := s.resolveProject(id)
	if err != nil {
		return OrchestratorTurn{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return OrchestratorTurn{}, err
	}
	for _, turn := range saved.Turns {
		if turn.ID == turnID {
			return turn.OrchestratorTurn, nil
		}
	}
	archived, err := s.archivedCoordinationTurnLocked(p, turnID)
	if err != nil {
		return OrchestratorTurn{}, err
	}
	if archived != nil {
		return archived.OrchestratorTurn, nil
	}
	return OrchestratorTurn{}, ErrOrchestratorInvalid
}

// Keep proposal copying independent from shared caller pointers.
func cloneOrchestratorAnswer(answer *OrchestratorAnswer) *OrchestratorAnswer {
	if answer == nil {
		return nil
	}
	data, _ := json.Marshal(answer)
	var copy OrchestratorAnswer
	_ = json.Unmarshal(data, &copy)
	return &copy
}
