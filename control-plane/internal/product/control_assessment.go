package product

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const assessmentFileLimit = 2 << 20

type ControlAssessmentResult struct {
	ControlID    string                  `json:"controlId"`
	Status       string                  `json:"status"`
	EvidenceRefs []DiscoveryEvidenceLink `json:"evidenceRefs"`
	Note         string                  `json:"note"`
}
type ControlGapProposal struct {
	ID                 string `json:"id"`
	ControlID          string `json:"controlId"`
	AssessmentRevision int    `json:"assessmentRevision"`
	Target             string `json:"target"`
	Summary            string `json:"summary"`
	State              string `json:"state"` // pending-review, never an approved change
	AppliedRevision    int    `json:"appliedRevision,omitempty"`
	AppliedDigest      string `json:"appliedDigest,omitempty"`
	ActorID            string `json:"actorId"`
	At                 string `json:"at"`
}
type ControlAssessment struct {
	Version         int                       `json:"version"`
	TeamID          string                    `json:"teamId"`
	ProjectID       string                    `json:"projectId"`
	AccountScope    string                    `json:"accountScope"`
	ProjectScope    string                    `json:"projectScope"`
	Revision        int                       `json:"revision"`
	LibraryRevision int                       `json:"libraryRevision"`
	LibraryDigest   string                    `json:"libraryDigest"`
	ContextDigest   string                    `json:"contextDigest"`
	Results         []ControlAssessmentResult `json:"results"`
	Proposals       []ControlGapProposal      `json:"proposals"`
	ActorID         string                    `json:"actorId"`
	At              string                    `json:"at"`
	KeyDigest       string                    `json:"keyDigest"`
	RequestDigest   string                    `json:"requestDigest"`
	PriorKeys       []string                  `json:"priorKeys,omitempty"`
}
type saveAssessmentRequest struct {
	ExpectedRevision int                       `json:"expectedRevision"`
	LibraryRevision  int                       `json:"libraryRevision"`
	LibraryDigest    string                    `json:"libraryDigest"`
	ContextDigest    string                    `json:"contextDigest"`
	IdempotencyKey   string                    `json:"idempotencyKey"`
	Results          []ControlAssessmentResult `json:"results"`
}

func (s *Server) controlAssessmentPath(p project.Project) (string, ControlAssessment, error) {
	scopes, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", ControlAssessment{}, err
	}
	path, err := guardedProjectPath(s.stateDir, "control-assessment-"+p.ID+".json", true)
	return path, ControlAssessment{Version: 1, ProjectID: p.ID, AccountScope: scopes.AccountScope, ProjectScope: scopes.ProjectScope, Results: []ControlAssessmentResult{}, Proposals: []ControlGapProposal{}}, err
}
func (s *Server) readControlAssessment(p project.Project) (string, ControlAssessment, error) {
	path, expected, err := s.controlAssessmentPath(p)
	if err != nil {
		return "", expected, err
	}
	var saved ControlAssessment
	err = externalReadJSON(path, assessmentFileLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, expected, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != expected.ProjectID || saved.AccountScope != expected.AccountScope || saved.ProjectScope != expected.ProjectScope || saved.Revision < 1 || len(saved.Results) > controlRowLimit || len(saved.Proposals) > 100 || len(saved.PriorKeys) > 200 {
		return "", expected, errControlWorkbook
	}
	return path, saved, nil
}
func writeControlAssessment(path string, row ControlAssessment) error {
	data, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(data)+1 > assessmentFileLimit {
		return errControlWorkbook
	}
	return durableRunJSON(path, row)
}
func (s *Server) currentAssessmentContext(p project.Project) (string, error) {
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return "", err
	}
	return assessmentContextDigest(state), nil
}
func assessmentContextDigest(state journeyState) string {
	return reviewDigest(struct {
		Brief         Brief
		Specification *ProjectSpecification
		Design        any
		Plan          any
	}{state.Brief, state.Specification, state.DesignPlan, state.Plan})
}
func assessmentStatus(value string) bool {
	switch value {
	case "supported", "partial", "missing", "not-applicable", "unreviewed", "unknown":
		return true
	}
	return false
}
func (s *Server) assessmentReadAccess(w http.ResponseWriter, r *http.Request, edit bool) (project.Project, management.TeamAccess, bool) {
	if !discoveryDocumentAccess(w, r, edit) {
		return project.Project{}, management.TeamAccess{}, false
	}
	access, ok := management.TeamAccessFromContext(r.Context())
	if !ok {
		journeyError(w, 409, "team_not_configured", "Choose a team workspace for a shared control assessment.")
		return project.Project{}, access, false
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return p, access, false
	}
	return p, access, true
}
func (s *Server) getControlAssessment(w http.ResponseWriter, r *http.Request) {
	p, access, ok := s.assessmentReadAccess(w, r, false)
	if !ok {
		return
	}
	s.mu.RLock()
	_, saved, err := s.readControlAssessment(p)
	contextDigest, contextErr := s.currentAssessmentContext(p)
	s.mu.RUnlock()
	if err != nil || contextErr != nil {
		journeyError(w, 409, "assessment_unavailable", "The current project evidence cannot be verified.")
		return
	}
	library, err := s.currentTeamLibrary(r.Context())
	if err != nil {
		journeyError(w, 409, "library_unavailable", "The team control library cannot be verified.")
		return
	}
	if saved.Revision > 0 && saved.TeamID != access.Team.ID {
		journeyError(w, 403, "assessment_scope_changed", "This assessment belongs to another team scope.")
		return
	}
	status := "not-configured"
	if library != nil {
		status = "unreviewed"
	}
	if saved.Revision > 0 {
		status = "current"
		if library == nil || saved.LibraryRevision != library.Revision || saved.LibraryDigest != library.Digest || saved.ContextDigest != contextDigest {
			status = "stale"
		}
	}
	journeyJSON(w, 200, map[string]any{"state": status, "projectId": p.ID, "contextDigest": contextDigest, "library": library, "assessment": saved})
}

func (s *Server) saveControlAssessment(w http.ResponseWriter, r *http.Request) {
	p, access, ok := s.assessmentReadAccess(w, r, true)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req saveAssessmentRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validRunKey(req.IdempotencyKey) || req.ExpectedRevision < 0 || len(req.Results) > controlRowLimit {
		journeyError(w, 400, "invalid_assessment", "Choose a bounded assessment and an idempotency key.")
		return
	}
	library, err := s.currentTeamLibrary(r.Context())
	if err != nil || library == nil {
		journeyError(w, 409, "library_unavailable", "Import a team library before assessing this project.")
		return
	}
	if req.LibraryRevision != library.Revision || req.LibraryDigest != library.Digest {
		journeyError(w, 409, "library_changed", "The team library changed. Review its current controls before saving.")
		return
	}
	seen := map[string]bool{}
	for _, result := range req.Results {
		if !controlInRevision(library, result.ControlID) || seen[result.ControlID] || !assessmentStatus(result.Status) || len(result.Note) > 2000 || len(result.EvidenceRefs) > 20 || result.Status == "supported" && len(result.EvidenceRefs) == 0 {
			journeyError(w, 422, "invalid_assessment", "A supported control needs exact linked evidence; use unknown or unreviewed until it exists.")
			return
		}
		seen[result.ControlID] = true
	}
	if len(seen) != len(library.Controls) {
		journeyError(w, 422, "incomplete_assessment", "Review every control, including unknown and unreviewed ones.")
		return
	}
	for _, result := range req.Results {
		for _, link := range result.EvidenceRefs {
			if link.TargetKind != "purpose" || link.TargetID != "" {
				journeyError(w, 422, "invalid_assessment", "Link exact document segments to controls through this assessment, without changing their source targets.")
				return
			}
		}
		if err := s.validateDiscoveryEvidenceLinks(r.Context(), p, result.EvidenceRefs); err != nil {
			journeyError(w, 422, "evidence_unavailable", "One linked private document segment cannot be verified.")
			return
		}
	}
	key := reviewDigest(struct{ Project, Actor, Key string }{p.ID, actor.ID, req.IdempotencyKey})
	request := reviewDigest(struct {
		Library string
		Context string
		Results []ControlAssessmentResult
	}{req.LibraryDigest, req.ContextDigest, req.Results})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, latestLedger, libraryErr := s.readTeamControls(access.Team.ID)
	latest := latestControls(latestLedger)
	if libraryErr != nil || latest == nil || latest.Revision != req.LibraryRevision || latest.Digest != req.LibraryDigest {
		journeyError(w, 409, "library_changed", "The team library changed. Review its current controls before saving.")
		return
	}
	path, saved, err := s.readControlAssessment(p)
	currentContext, contextErr := s.currentAssessmentContext(p)
	if err != nil || contextErr != nil {
		journeyError(w, 409, "assessment_unavailable", "The project assessment cannot be verified.")
		return
	}
	if saved.Revision > 0 && saved.TeamID != access.Team.ID {
		journeyError(w, 403, "assessment_scope_changed", "This assessment belongs to another team scope.")
		return
	}
	if saved.KeyDigest == key {
		if saved.RequestDigest != request {
			journeyError(w, 409, "assessment_key_conflict", "That key belongs to different assessment inputs.")
			return
		}
		journeyJSON(w, 200, saved)
		return
	}
	for _, prior := range saved.PriorKeys {
		if prior == key {
			journeyError(w, 409, "assessment_key_consumed", "That request key belongs to an older assessment. Read the latest revision before changing it.")
			return
		}
	}
	if saved.Revision != req.ExpectedRevision || req.ContextDigest != currentContext || r.Context().Err() != nil {
		journeyError(w, 409, "assessment_changed", "Project evidence or the assessment changed. Refresh before saving.")
		return
	}
	if len(saved.PriorKeys) >= 200 {
		journeyError(w, 409, "assessment_history_full", "The retained assessment request history is full.")
		return
	}
	if saved.KeyDigest != "" {
		saved.PriorKeys = append(saved.PriorKeys, saved.KeyDigest)
	}
	saved.TeamID = access.Team.ID
	saved.Revision++
	saved.LibraryRevision = library.Revision
	saved.LibraryDigest = library.Digest
	saved.ContextDigest = currentContext
	saved.Results = req.Results
	saved.Proposals = []ControlGapProposal{}
	saved.ActorID = actor.ID
	saved.At = time.Now().UTC().Format(time.RFC3339Nano)
	saved.KeyDigest = key
	saved.RequestDigest = request
	if err := writeControlAssessment(path, saved); err != nil {
		journeyError(w, 409, "assessment_write_failed", "The assessment could not be saved within its bound.")
		return
	}
	journeyJSON(w, 200, saved)
}

func (s *Server) proposeControlGap(w http.ResponseWriter, r *http.Request) {
	p, access, ok := s.assessmentReadAccess(w, r, true)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		AssessmentRevision int    `json:"assessmentRevision"`
		Target             string `json:"target"`
		Summary            string `json:"summary"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	id := r.PathValue("controlId")
	req.Summary = strings.TrimSpace(req.Summary)
	if !controlIDPattern.MatchString(id) || (req.Target != "discover" && req.Target != "design") || len(req.Summary) < 12 || len(req.Summary) > 2000 {
		journeyError(w, 422, "invalid_gap", "Choose Discover or Design and explain the evidence gap.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, saved, err := s.readControlAssessment(p)
	contextDigest, contextErr := s.currentAssessmentContext(p)
	if err != nil || contextErr != nil || saved.Revision == 0 || saved.TeamID != access.Team.ID {
		journeyError(w, 409, "assessment_unavailable", "Review a current assessment first.")
		return
	}
	_, ledger, libraryErr := s.readTeamControls(access.Team.ID)
	library := latestControls(ledger)
	if libraryErr != nil || library == nil || library.Revision != saved.LibraryRevision || library.Digest != saved.LibraryDigest {
		journeyError(w, 409, "library_changed", "Reassess current team controls before proposing a gap.")
		return
	}
	if saved.Revision != req.AssessmentRevision || saved.ContextDigest != contextDigest {
		journeyError(w, 409, "assessment_changed", "The project assessment changed. Review it again.")
		return
	}
	resultFound := false
	for _, result := range saved.Results {
		if result.ControlID == id && result.Status != "supported" && result.Status != "not-applicable" {
			resultFound = true
		}
	}
	if !resultFound {
		journeyError(w, 422, "gap_unavailable", "Only an unresolved control can become a proposal.")
		return
	}
	idempotentID := "gap-" + reviewDigest(struct {
		Project, Control, Actor, Target, Summary string
		Revision                                 int
	}{p.ID, id, actor.ID, req.Target, req.Summary, saved.Revision})[:24]
	for _, proposal := range saved.Proposals {
		if proposal.ID == idempotentID {
			journeyJSON(w, 200, proposal)
			return
		}
	}
	if len(saved.Proposals) >= 100 {
		journeyError(w, 409, "proposal_limit", "This assessment reached its proposal limit.")
		return
	}
	proposal := ControlGapProposal{ID: idempotentID, ControlID: id, AssessmentRevision: saved.Revision, Target: req.Target, Summary: req.Summary, State: "pending-review", ActorID: actor.ID, At: time.Now().UTC().Format(time.RFC3339Nano)}
	saved.Proposals = append(saved.Proposals, proposal)
	saved.Revision++
	if err := writeControlAssessment(path, saved); err != nil {
		journeyError(w, 409, "proposal_write_failed", "The proposal could not be saved.")
		return
	}
	journeyJSON(w, 200, proposal)
}

// Applying a gap is an explicit, reviewed edit of the existing project
// journey. It creates a draft and revokes the prior approval; it cannot approve
// context, start agents or claim that an evidence gap was resolved. The exact
// proposal marker in the journey lets a retry settle a partial second write
// without applying the same amendment twice.
func (s *Server) applyControlGap(w http.ResponseWriter, r *http.Request) {
	p, access, ok := s.assessmentReadAccess(w, r, true)
	if !ok {
		return
	}
	var req struct {
		ExpectedAssessmentRevision int    `json:"expectedAssessmentRevision"`
		ExpectedContextDigest      string `json:"expectedContextDigest"`
		ExpectedTargetRevision     int    `json:"expectedTargetRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedAssessmentRevision < 1 || len(req.ExpectedContextDigest) != 64 || req.ExpectedTargetRevision < 0 {
		journeyError(w, 400, "invalid_apply", "Review exact current assessment and context revisions before applying.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, saved, err := s.readControlAssessment(p)
	if err != nil || saved.Revision == 0 || saved.TeamID != access.Team.ID {
		journeyError(w, 409, "assessment_unavailable", "Review the current team assessment first.")
		return
	}
	index := -1
	for i, proposal := range saved.Proposals {
		if proposal.ID == r.PathValue("proposalId") && proposal.ControlID == r.PathValue("controlId") {
			index = i
			break
		}
	}
	if index < 0 {
		journeyError(w, 404, "proposal_unavailable", "This control gap proposal is unavailable.")
		return
	}
	proposal := saved.Proposals[index]
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		journeyError(w, 409, "context_unavailable", "The current project context cannot be verified.")
		return
	}
	marker := proposal.ID + ": " + proposal.Summary
	var recorded bool
	var targetRevision int
	var targetDigest string
	if proposal.Target == "discover" {
		for _, prior := range state.SpecificationRevisions {
			for _, question := range prior.Questions {
				if question.ID == proposal.ID && question.Question == proposal.Summary {
					recorded = true
					targetRevision = prior.Revision
					targetDigest = reviewDigest(prior)
				}
			}
		}
		if !recorded {
			targetRevision = currentSpecification(state).Revision
			targetDigest = reviewDigest(currentSpecification(state))
		}
	} else if proposal.Target == "design" {
		for _, prior := range state.DesignPlanRevisions {
			if strings.Contains(prior.ImplementationNotes, "[control-gap:"+marker+"]") {
				recorded = true
				targetRevision = prior.Revision
				targetDigest = reviewDigest(prior)
				break
			}
		}
		if !recorded {
			plan := currentDesignPlan(state)
			targetRevision = plan.Revision
			targetDigest = reviewDigest(plan)
		}
	} else {
		journeyError(w, 409, "proposal_invalid", "The retained proposal target is invalid.")
		return
	}
	if proposal.State == "applied-draft" {
		if !recorded {
			journeyError(w, 409, "proposal_context_changed", "The applied draft changed. Inspect its history.")
			return
		}
		journeyJSON(w, 200, proposal)
		return
	}
	if proposal.State != "pending-review" {
		journeyError(w, 409, "proposal_invalid", "This proposal cannot be applied.")
		return
	}
	// A prior journey write may have succeeded while the assessment receipt did
	// not. Positive exact marker readback resolves that case without a second edit.
	if !recorded {
		if saved.Revision != req.ExpectedAssessmentRevision || saved.ContextDigest != req.ExpectedContextDigest || assessmentContextDigest(state) != req.ExpectedContextDigest || targetRevision != req.ExpectedTargetRevision || r.Context().Err() != nil {
			journeyError(w, 409, "context_changed", "Project context or the assessment changed. Review the gap again.")
			return
		}
		_, ledger, libraryErr := s.readTeamControls(access.Team.ID)
		library := latestControls(ledger)
		if libraryErr != nil || library == nil || saved.LibraryRevision != library.Revision || saved.LibraryDigest != library.Digest {
			journeyError(w, 409, "library_changed", "The control library changed. Reassess the gap first.")
			return
		}
		if proposal.Target == "discover" {
			spec := currentSpecification(state)
			if len(spec.Questions) >= 100 {
				journeyError(w, 409, "question_limit", "The definition reached its question limit.")
				return
			}
			spec.Questions = append(spec.Questions, SpecificationQuestion{ID: proposal.ID, Question: proposal.Summary, Answer: ""})
			spec.Status = "draft"
			spec.Digest = ""
			spec.Revision++
			spec.BriefRevision = state.Brief.Revision + 1
			spec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := validateSpecification(spec); err != nil {
				journeyError(w, 422, "invalid_gap", err.Error())
				return
			}
			brief := state.Brief
			brief.Revision = spec.BriefRevision
			brief.UpdatedAt = spec.UpdatedAt
			if err := s.revokeJourneyApproval(p.Path, p.ID, true); err != nil {
				journeyError(w, 500, "approval_unavailable", "Prior review authority could not be revoked.")
				return
			}
			state.Brief = brief
			state.BriefRevisions = append(state.BriefRevisions, brief)
			state.Specification = &spec
			state.SpecificationRevisions = append(state.SpecificationRevisions, spec)
			if err := s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"specification", "save"}, contextMutation{"brief", "save"}); err != nil {
				journeyError(w, 500, "context_write_failed", "The draft could not be saved. Inspect its history before retrying.")
				return
			}
			if err := writeProjectAtomic(p.Path, "LUCI.md", renderBrief(brief)); err != nil {
				journeyError(w, 500, "context_export_failed", "The draft was saved but its context export needs repair. Inspect the saved definition.")
				return
			}
			targetRevision, targetDigest = spec.Revision, reviewDigest(spec)
		} else {
			plan := currentDesignPlan(state)
			plan.ImplementationNotes = strings.TrimSpace(plan.ImplementationNotes + "\n\n[control-gap:" + marker + "]")
			plan.BriefRevision = state.Brief.Revision
			plan.SpecificationRevision = currentSpecification(state).Revision
			plan.SketchRevision = state.Design.Revision
			plan.ControlLibraryRevision, plan.ControlLibraryDigest = library.Revision, library.Digest
			plan.Status = "draft"
			plan.Digest = ""
			plan.Revision++
			plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := validateDesignPlan(plan); err != nil {
				journeyError(w, 422, "invalid_gap", err.Error())
				return
			}
			if err := s.revokeJourneyApproval(p.Path, p.ID, false); err != nil {
				journeyError(w, 500, "approval_unavailable", "Prior review authority could not be revoked.")
				return
			}
			state.DesignPlan = &plan
			state.DesignPlanRevisions = append(state.DesignPlanRevisions, plan)
			if err := s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"design", "save"}); err != nil {
				journeyError(w, 500, "context_write_failed", "The draft could not be saved. Inspect its history before retrying.")
				return
			}
			targetRevision, targetDigest = plan.Revision, reviewDigest(plan)
		}
	}
	if recorded && proposal.Target == "discover" && state.Specification != nil && state.Specification.Revision == targetRevision {
		if err := writeProjectAtomic(p.Path, "LUCI.md", renderBrief(state.Brief)); err != nil {
			journeyError(w, 500, "context_export_failed", "The saved draft's context export needs repair.")
			return
		}
	}
	proposal.State = "applied-draft"
	proposal.AppliedRevision = targetRevision
	proposal.AppliedDigest = targetDigest
	saved.Proposals[index] = proposal
	saved.Revision++
	if err := writeControlAssessment(path, saved); err != nil {
		journeyError(w, 500, "assessment_receipt_unconfirmed", "The draft may have been saved; inspect it before repeating this exact apply request.")
		return
	}
	journeyJSON(w, 200, proposal)
}
