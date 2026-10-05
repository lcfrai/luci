package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// These are links to recorded command outcomes, never a certification that an
// acceptance scenario was exercised or that a requirement is satisfied.
const (
	evidenceLinked     = "linked-test-evidence"
	evidenceStale      = "stale"
	evidenceUnverified = "unverified"
)

type RequirementSnapshot struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Acceptance string `json:"acceptance"`
}

type projectRequirementBinding struct {
	RequirementIDs        []string
	RequirementSnapshots  []RequirementSnapshot
	SpecificationRevision int
	SpecificationDigest   string
	DesignPlanRevision    int
	DesignPlanDigest      string
}

type evidenceBindingError struct {
	status        int
	code, message string
}

func (e *evidenceBindingError) Error() string { return e.message }
func writeEvidenceBindingError(w http.ResponseWriter, err error) {
	var binding *evidenceBindingError
	if errors.As(err, &binding) {
		problem(w, binding.status, binding.code, binding.message)
		return
	}
	problem(w, 400, "invalid_requirement_link", "Choose requirements from this project's current approved specification.")
}

func validateRequirementIDs(ids []string) error {
	if len(ids) > 100 {
		return errors.New("Link at most 100 requirements.")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !designIDPattern.MatchString(id) || seen[id] {
			return errors.New("Requirement links need unique stable IDs.")
		}
		seen[id] = true
	}
	return nil
}

func snapshotProjectTasks(tasks []ProjectTask) []ProjectTask {
	copy := append([]ProjectTask{}, tasks...)
	for i := range copy {
		copy[i].RequirementIDs = append([]string(nil), copy[i].RequirementIDs...)
	}
	return copy
}

// Approvals hash the complete server-saved snapshot with its digest cleared.
// Recompute it so a changed acceptance text or handoff cannot retain an old
// approval merely by retaining its revision and digest fields.
func specificationContentDigest(spec ProjectSpecification) string {
	spec.Digest = ""
	data, _ := json.Marshal(spec)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func designPlanContentDigest(plan ProjectDesignPlan) string {
	plan.Digest = ""
	data, _ := json.Marshal(plan)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func currentEvidenceApprovals(state journeyState, id string) (ProjectSpecification, ProjectDesignPlan) {
	spec, design := currentSpecification(state), currentDesignPlan(state)
	if spec.Status == "approved" && (state.Specification == nil || spec.ProjectID != id || spec.Revision <= 0 || spec.Digest != specificationContentDigest(spec)) {
		spec.Status = "outdated"
	}
	if design.Status == "approved" && (state.DesignPlan == nil || design.ProjectID != id || design.Revision <= 0 || design.Digest != designPlanContentDigest(design)) {
		design.Status = "outdated"
	}
	return spec, design
}

// Caller holds s.mu while reading the authoritative saved journey and reserving
// the plan/run. Client-provided requirement text and approval digests are ignored.
func bindProjectRequirements(state journeyState, id string, ids []string, expectedSpec, expectedDesign *int, requireDesign bool) (projectRequirementBinding, error) {
	binding := projectRequirementBinding{}
	if err := validateRequirementIDs(ids); err != nil {
		return binding, err
	}
	if len(ids) == 0 {
		return binding, nil
	}
	spec, design := currentEvidenceApprovals(state, id)
	if spec.Status != "approved" {
		return binding, &evidenceBindingError{422, "specification_approval_required", "Approve the current saved specification before linking requirement evidence."}
	}
	if expectedSpec == nil || *expectedSpec != spec.Revision {
		return binding, &evidenceBindingError{409, "revision_conflict", "The approved specification changed. Reload it before linking requirements."}
	}
	requirements := map[string]SpecificationRequirement{}
	for _, req := range spec.Requirements {
		requirements[req.ID] = req
	}
	for _, requirementID := range ids {
		req, exists := requirements[requirementID]
		if !exists {
			return binding, &evidenceBindingError{400, "invalid_requirement_link", "Choose requirement IDs from this project's current approved specification."}
		}
		binding.RequirementSnapshots = append(binding.RequirementSnapshots, RequirementSnapshot{ID: req.ID, Title: req.Title, Acceptance: req.Acceptance})
	}
	binding.RequirementIDs = append([]string{}, ids...)
	binding.SpecificationRevision, binding.SpecificationDigest = spec.Revision, spec.Digest
	if requireDesign {
		if design.Status != "approved" {
			return projectRequirementBinding{}, &evidenceBindingError{422, "design_approval_required", "Approve the current saved design before linking test evidence."}
		}
		if expectedDesign == nil || *expectedDesign != design.Revision {
			return projectRequirementBinding{}, &evidenceBindingError{409, "revision_conflict", "The approved design changed. Reload it before linking test evidence."}
		}
		binding.DesignPlanRevision, binding.DesignPlanDigest = design.Revision, design.Digest
	}
	return binding, nil
}

type EvidenceApproval struct {
	Revision int    `json:"revision"`
	Status   string `json:"status"`
	Digest   string `json:"digest"`
}
type EvidenceSource struct {
	Commit       string `json:"commit"`
	SourceDigest string `json:"sourceDigest"`
	Dirty        bool   `json:"dirty"`
	Available    bool   `json:"available"`
}
type RequirementCoverageRow struct {
	RequirementID  string   `json:"requirementId"`
	Title          string   `json:"title"`
	Acceptance     string   `json:"acceptance"`
	TaskIDs        []string `json:"taskIds"`
	DoneTaskIDs    []string `json:"doneTaskIds"`
	EvidenceRunIDs []string `json:"evidenceRunIds"`
	EvidenceStatus string   `json:"evidenceStatus"`
}
type ProjectRequirementCoverage struct {
	ProjectID     string                   `json:"projectId"`
	Specification EvidenceApproval         `json:"specification"`
	Design        EvidenceApproval         `json:"design"`
	Source        EvidenceSource           `json:"source"`
	Rows          []RequirementCoverageRow `json:"rows"`
}

// Hashing only uses confined native Git/file reads. A failed or busy capture
// never launches a process from customer source or guesses current evidence.
func (s *Server) evidenceSource(ctx context.Context, id, dir string, state journeyState, rows []ProjectCheckRun) EvidenceSource {
	spec, design := currentEvidenceApprovals(state, id)
	needed := spec.Status == "approved" && design.Status == "approved" && len(spec.Requirements) > 0
	for _, row := range rows {
		if len(row.RequirementIDs) > 0 {
			needed = true
			break
		}
	}
	if !needed {
		return EvidenceSource{}
	}
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return EvidenceSource{}
	}
	defer release()
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	digest, commit, dirty, err := s.checkSource(bounded, dir)
	if err != nil {
		return EvidenceSource{}
	}
	return EvidenceSource{Commit: commit, SourceDigest: digest, Dirty: dirty, Available: true}
}

func (s *Server) readProjectEvidence(ctx context.Context, id, dir string) (ProjectRequirementCoverage, []ProjectCheckRun, error) {
	s.mu.RLock()
	state, stateErr := readJourney(dir, id)
	rows, err := s.readCheckHistory(id)
	s.mu.RUnlock()
	if stateErr != nil {
		return ProjectRequirementCoverage{}, nil, errors.New("The saved project definition could not be read.")
	}
	if err != nil {
		return ProjectRequirementCoverage{}, nil, err
	}
	source := s.evidenceSource(ctx, id, dir, state, rows)
	// File hashing must not hold the shared journey mutex. Refresh authoritative
	// approvals afterwards so concurrent saves cannot retain superseded evidence.
	s.mu.RLock()
	state, stateErr = readJourney(dir, id)
	rows, err = s.readCheckHistory(id)
	s.mu.RUnlock()
	if stateErr != nil {
		return ProjectRequirementCoverage{}, nil, errors.New("The saved project definition could not be read.")
	}
	if err != nil {
		return ProjectRequirementCoverage{}, nil, err
	}
	coverage, views := projectEvidenceView(state, id, source, rows)
	return coverage, views, nil
}

func evidenceRunView(row ProjectCheckRun, id string, spec ProjectSpecification, design ProjectDesignPlan, source EvidenceSource) ProjectCheckRun {
	row = snapshotProjectCheck(row)
	row.EvidenceStatus, row.StaleReasons = evidenceUnverified, []string{}
	if row.ProjectID != id || len(row.RequirementIDs) == 0 || validateRequirementIDs(row.RequirementIDs) != nil || len(row.RequirementSnapshots) != len(row.RequirementIDs) || row.SpecificationRevision <= 0 || row.DesignPlanRevision <= 0 || !hexDigest(row.SpecificationDigest) || !hexDigest(row.DesignPlanDigest) || !hexDigest(row.SourceDigest) {
		row.StaleReasons = append(row.StaleReasons, "This run has no complete saved requirement and approval binding.")
		return row
	}
	stale := false
	addStale := func(reason string) { stale = true; row.StaleReasons = append(row.StaleReasons, reason) }
	if spec.Status != "approved" {
		addStale("The current specification is not approved.")
	}
	if spec.Revision != row.SpecificationRevision || spec.Digest != row.SpecificationDigest || specificationContentDigest(spec) != row.SpecificationDigest {
		addStale("The specification changed after this run was linked.")
	}
	if design.Status != "approved" {
		addStale("The current design is not approved.")
	}
	if design.Revision != row.DesignPlanRevision || design.Digest != row.DesignPlanDigest || designPlanContentDigest(design) != row.DesignPlanDigest {
		addStale("The design changed after this run was linked.")
	}
	requirements := map[string]SpecificationRequirement{}
	for _, req := range spec.Requirements {
		requirements[req.ID] = req
	}
	for i, snapshot := range row.RequirementSnapshots {
		req, exists := requirements[row.RequirementIDs[i]]
		if snapshot.ID != row.RequirementIDs[i] || !exists || snapshot.Title != req.Title || snapshot.Acceptance != req.Acceptance {
			addStale("A linked requirement or its acceptance text changed after this run.")
			break
		}
	}
	if row.SourceChanged {
		addStale("Project source changed while this run was executing.")
	}
	if !source.Available {
		row.StaleReasons = append(row.StaleReasons, "Current project source could not be read safely; this evidence is unverified.")
	} else if source.SourceDigest != row.SourceDigest || source.Commit != row.Commit {
		addStale("Project source or its Git commit changed after this run.")
	}
	if row.Status != "passed" {
		switch row.Status {
		case "failed":
			row.StaleReasons = append(row.StaleReasons, "The linked commands failed.")
		case "cancelled":
			row.StaleReasons = append(row.StaleReasons, "The linked run was cancelled before passing.")
		case "queued", "running":
			row.StaleReasons = append(row.StaleReasons, "The linked commands have not finished.")
		default:
			row.StaleReasons = append(row.StaleReasons, "The linked run has no confirmed passing outcome.")
		}
		if row.SourceChanged && row.Status == "unknown" {
			row.EvidenceStatus = evidenceStale
		}
		return row
	}
	if len(row.Commands) == 0 {
		row.StaleReasons = append(row.StaleReasons, "This run has no recorded command outcomes.")
		return row
	}
	for _, command := range row.Commands {
		if command.Status != "passed" || command.ExitCode == nil || *command.ExitCode != 0 {
			row.StaleReasons = append(row.StaleReasons, "Not every linked command has a recorded passing exit code.")
			return row
		}
	}
	if stale {
		row.EvidenceStatus = evidenceStale
	} else if source.Available {
		row.EvidenceStatus = evidenceLinked
	}
	return row
}

func projectEvidenceView(state journeyState, id string, source EvidenceSource, runs []ProjectCheckRun) (ProjectRequirementCoverage, []ProjectCheckRun) {
	spec, design := currentEvidenceApprovals(state, id)
	coverage := ProjectRequirementCoverage{ProjectID: id, Specification: EvidenceApproval{spec.Revision, spec.Status, spec.Digest}, Design: EvidenceApproval{design.Revision, design.Status, design.Digest}, Source: source, Rows: []RequirementCoverageRow{}}
	views := make([]ProjectCheckRun, 0, len(runs))
	for _, run := range runs {
		views = append(views, evidenceRunView(run, id, spec, design, source))
	}
	plan := currentPlan(state, id)
	for _, req := range spec.Requirements {
		row := RequirementCoverageRow{RequirementID: req.ID, Title: req.Title, Acceptance: req.Acceptance, TaskIDs: []string{}, DoneTaskIDs: []string{}, EvidenceRunIDs: []string{}, EvidenceStatus: evidenceUnverified}
		for _, task := range plan.Tasks {
			for _, requirementID := range task.RequirementIDs {
				if requirementID == req.ID {
					row.TaskIDs = append(row.TaskIDs, task.ID)
					if task.Status == "done" {
						row.DoneTaskIDs = append(row.DoneTaskIDs, task.ID)
					}
					break
				}
			}
		}
		for _, run := range views {
			for _, requirementID := range run.RequirementIDs {
				if run.ProjectID == id && requirementID == req.ID {
					row.EvidenceRunIDs = append(row.EvidenceRunIDs, run.ID)
					if run.EvidenceStatus == evidenceLinked {
						row.EvidenceStatus = evidenceLinked
					} else if run.EvidenceStatus == evidenceStale && row.EvidenceStatus != evidenceLinked {
						row.EvidenceStatus = evidenceStale
					}
					break
				}
			}
		}
		coverage.Rows = append(coverage.Rows, row)
	}
	return coverage, views
}
