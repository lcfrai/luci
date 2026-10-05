package product

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"strings"
)

// ResolveProject gives owned integrations the same catalogue confinement as
// the project journey. A caller's ID is never interpreted as a filesystem path.
func (s *Server) ResolveProject(id string) (project.Project, error) {
	return s.resolveProject(id)
}

// ApplicationConfiguration supplies runtime-only values to owned service
// runners. Integrations must never put them into logs or public records.
func (s *Server) ApplicationConfiguration(id string) (int, map[string]string, error) {
	snapshot, err := s.applicationConfigurationSnapshot(id)
	return snapshot.Revision, snapshot.Values, err
}

// SpecificationSnapshot is a bounded read of the current project definition.
func (s *Server) SpecificationSnapshot(id string) (ProjectSpecification, error) {
	p, err := s.resolveProject(id)
	if err != nil {
		return ProjectSpecification{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		return ProjectSpecification{}, err
	}
	return currentSpecification(state), nil
}

// ValidateDeploymentSource ties an application release to the exact reviewed
// commit, including structured definition bytes. No build is started here.
func (s *Server) ValidateDeploymentSource(ctx context.Context, id, commit string) error {
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	if !deliverySHA.MatchString(commit) {
		return errors.New("Choose a full source commit.")
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return err
	}
	head, err := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(head) != commit {
		return errors.New("The source commit changed. Review the current revision before deploying.")
	}
	status, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || status != "" {
		return errors.New("Commit all source and context changes before deploying.")
	}
	s.mu.RLock()
	state, err := readJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if state.Brief.Revision == 0 {
		return errors.New("Save a project definition before deploying.")
	}
	text, err := s.journeyGit(ctx, p.Path, "show", commit+":LUCI.md")
	if err != nil || text != string(renderBrief(state.Brief)) {
		return errors.New("Commit the current LUCI.md before deploying.")
	}
	if state.Specification != nil {
		spec := currentSpecification(state)
		if spec.Status != "approved" {
			return errors.New("Approve the current product definition before deploying.")
		}
		text, err := s.journeyGit(ctx, p.Path, "show", commit+":.luci/specification.json")
		var committed ProjectSpecification
		if err != nil || json.Unmarshal([]byte(text), &committed) != nil || committed.Digest != spec.Digest || committed.Revision != spec.Revision || committed.ProjectID != id {
			return errors.New("Commit the approved specification snapshot before deploying.")
		}
		canonical, _ := json.Marshal(spec)
		actual, _ := json.Marshal(committed)
		if string(canonical) != string(actual) {
			return errors.New("The source definition differs from the approved snapshot.")
		}
	}
	if state.DesignPlan != nil {
		plan := currentDesignPlan(state)
		if plan.Status != "approved" {
			return errors.New("Approve the current design handoff before deploying.")
		}
		text, err := s.journeyGit(ctx, p.Path, "show", commit+":.luci/design-handoff.json")
		var committed ProjectDesignPlan
		if err != nil || json.Unmarshal([]byte(text), &committed) != nil {
			return errors.New("Commit the approved design handoff before deploying.")
		}
		canonical, _ := json.Marshal(plan)
		actual, _ := json.Marshal(committed)
		if string(canonical) != string(actual) {
			return errors.New("The source design differs from the approved handoff.")
		}
	}
	return nil
}
