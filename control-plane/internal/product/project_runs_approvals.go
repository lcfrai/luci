package product

import "errors"

// ProjectRunApprovalSelection contains opaque selection identifiers and content
// digests, never private receipt bodies, actors or account paths. Identical text
// reviewed again is a different selection and cannot revive an older run.
type ProjectRunApprovalSelection struct {
	SpecificationApprovalID string `json:"specificationApprovalId"`
	SpecificationRevision   int    `json:"specificationRevision"`
	SpecificationDigest     string `json:"specificationDigest"`
	DesignApprovalID        string `json:"designApprovalId"`
	DesignRevision          int    `json:"designRevision"`
	DesignDigest            string `json:"designDigest"`
	BriefDigest             string `json:"briefDigest"`
	SketchDigest            string `json:"sketchDigest"`
}

func projectRunApprovalSelection(state journeyState) (ProjectRunApprovalSelection, error) {
	if !specificationApprovalMatches(state) || !designApprovalMatches(state) {
		return ProjectRunApprovalSelection{}, errors.New("Approve the current definition and design before continuing this run.")
	}
	spec, design := state.approvalAuthority.Specification, state.approvalAuthority.Design
	return ProjectRunApprovalSelection{
		SpecificationApprovalID: spec.ID, SpecificationRevision: spec.Revision, SpecificationDigest: spec.ContentDigest,
		DesignApprovalID: design.ID, DesignRevision: design.Revision, DesignDigest: design.ContentDigest,
		BriefDigest: spec.BriefDigest, SketchDigest: design.SketchDigest,
	}, nil
}

// ApprovalSelection resolves current private review authority in this worker's
// owned project. Client digests and project-exported approval claims are unused.
func (s *Server) ApprovalSelection(projectID string) (ProjectRunApprovalSelection, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRunApprovalSelection{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return ProjectRunApprovalSelection{}, err
	}
	return projectRunApprovalSelection(state)
}

func (s *Server) validateProjectRunApprovals(projectID string, selected ProjectRunApprovalSelection) error {
	current, err := s.ApprovalSelection(projectID)
	if err != nil {
		return err
	}
	if selected != current {
		return errors.New("The run's private review selection changed. Start a new run from the current approved inputs.")
	}
	return nil
}
