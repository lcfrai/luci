package product

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// A caller selects exact prior evidence, never a task ID alone. This reference
// is not a worker allocation, execution authorization or a new integration.
type ProjectSwarmReuseReference struct {
	RunID             string `json:"runId"`
	TaskID            string `json:"taskId"`
	OperationID       string `json:"operationId"`
	IntegrationDigest string `json:"integrationDigest"`
}

type ProjectSwarmReusedDependency struct {
	ProjectSwarmReuseReference
	Task                 ProjectSwarmTask `json:"task"`
	OperationDigest      string           `json:"operationDigest"`
	HandoffViewDigest    string           `json:"handoffViewDigest"`
	AdmittedSourceCommit string           `json:"admittedSourceCommit"`
	handoff              TaskHandoffView
}

var errSwarmReuseProof = errors.New("The prerequisite has no exact current reuse proof. Retain its original integration and review the changed or legacy task inputs; it has not been queued again.")

// The digest excludes board progress and unrelated plan/requirement revisions.
// It includes all global definition/design constraints, the exact task and its
// linked requirements, and recursively each dependency's reviewed inputs.
func swarmTaskInputDigests(state journeyState, id string) (map[string]string, error) {
	plan := currentPlan(state, id)
	if len(plan.Tasks) > 200 || validatePlan(plan) != nil {
		return nil, ErrProjectRunInvalid
	}
	tasks := map[string]ProjectTask{}
	for _, task := range plan.Tasks {
		tasks[task.ID] = task
	}
	result := map[string]string{}
	visiting := map[string]bool{}
	var digest func(string) (string, error)
	digest = func(id string) (string, error) {
		if v := result[id]; v != "" {
			return v, nil
		}
		task, ok := tasks[id]
		if !ok || visiting[id] {
			return "", ErrProjectRunInvalid
		}
		visiting[id] = true
		task.Status = ""
		spec := currentSpecification(state)
		spec.Revision = 0
		spec.BriefRevision = 0
		spec.Status = ""
		spec.Digest = ""
		spec.UpdatedAt = ""
		wanted := map[string]bool{}
		for _, id := range task.RequirementIDs {
			wanted[id] = true
		}
		requirements := []SpecificationRequirement{}
		for _, r := range spec.Requirements {
			if wanted[r.ID] {
				requirements = append(requirements, r)
				delete(wanted, r.ID)
			}
		}
		if len(wanted) != 0 {
			return "", ErrProjectRunInvalid
		}
		spec.Requirements = requirements
		deps := map[string]string{}
		for _, id := range task.DependsOn {
			v, e := digest(id)
			if e != nil {
				return "", e
			}
			deps[id] = v
		}
		design := currentDesignPlan(state)
		design.Revision, design.BriefRevision, design.SpecificationRevision, design.SketchRevision = 0, 0, 0, 0
		design.Status, design.Digest, design.UpdatedAt = "", "", ""
		brief := state.Brief
		brief.Revision = 0
		brief.UpdatedAt = ""
		sketch := state.Design
		sketch.Revision, sketch.BriefRevision = 0, 0
		sketch.UpdatedAt = ""
		v := implementationDigest(struct {
			Version       int
			Task          ProjectTask
			Brief         string
			Specification ProjectSpecification
			Design        ProjectDesignPlan
			Sketch        Design
			Dependencies  map[string]string
		}{1, task, implementationDigest(brief), spec, design, sketch, deps})
		result[id] = v
		delete(visiting, id)
		return v, nil
	}
	for _, task := range plan.Tasks {
		if _, err := digest(task.ID); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func reusedSwarmDependency(run ProjectRun, id string) *ProjectSwarmReusedDependency {
	if run.Swarm != nil {
		for i := range run.Swarm.ReusedDependencies {
			if run.Swarm.ReusedDependencies[i].TaskID == id {
				return &run.Swarm.ReusedDependencies[i]
			}
		}
	}
	return nil
}

func validSwarmReuse(dependency ProjectSwarmReusedDependency, run ProjectRun) bool {
	task := dependency.Task
	in := task.Integration
	if in == nil || !deliverySHA.MatchString(in.SourceCommit) || !deliverySHA.MatchString(in.TargetCommit) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, in.IntegratedAt); err != nil {
		return false
	}
	return validRunID(dependency.RunID) && dependency.RunID != run.ID && dependency.TaskID == task.ID && dependency.OperationID == task.OperationID && strings.HasPrefix(dependency.OperationID, "op-") && prototypeReleaseID.MatchString(dependency.OperationID) && hexDigest(dependency.IntegrationDigest) && hexDigest(dependency.OperationDigest) && hexDigest(dependency.HandoffViewDigest) && hexDigest(task.TaskInputDigest) && len(task.WritePaths) > 0 && in != nil && in.State == "integrated" && deliverySHA.MatchString(in.Commit) && hexDigest(in.ReviewDigest) && implementationDigest(in) == dependency.IntegrationDigest && dependency.AdmittedSourceCommit == run.Input.Implementation.SourceCommit
}

// This read-only resolver is not admission. The caller still owns current
// authority, canonical/source admission and the destination run CAS. Preparing
// a swarm resolves these same references again against the admitted run base.
func (s *Server) ResolveProjectSwarmReuse(ctx context.Context, projectID string, selection ImplementationContextRequest, refs []ProjectSwarmReuseReference) ([]ProjectSwarmReusedDependency, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if selection.ProjectID != projectID {
		return nil, ErrProjectRunInvalid
	}
	if _, err = implementationSnapshot(state, selection, ""); err != nil {
		return nil, err
	}
	_, head, dirty, err := s.checkSource(ctx, p.Path)
	if err != nil || dirty || !deliverySHA.MatchString(head) {
		return nil, errSwarmReuseProof
	}
	return s.resolveSwarmReuse(ctx, p, state, selection.TaskIDs, head, refs)
}

func (s *Server) resolveSwarmReuse(ctx context.Context, p project.Project, state journeyState, selectedIDs []string, head string, refs []ProjectSwarmReuseReference) ([]ProjectSwarmReusedDependency, error) {
	if len(refs) > 200 {
		return nil, ErrProjectRunInvalid
	}
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	tasks := map[string]ProjectTask{}
	for _, task := range currentPlan(state, p.ID).Tasks {
		tasks[task.ID] = task
	}
	wanted := map[string]bool{}
	visiting := map[string]bool{}
	done := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if done[id] {
			return nil
		}
		task, ok := tasks[id]
		if !ok || visiting[id] {
			return ErrProjectRunInvalid
		}
		visiting[id] = true
		defer delete(visiting, id)
		defer func() { done[id] = true }()
		for _, dep := range task.DependsOn {
			if !selected[dep] {
				wanted[dep] = true
			}
			if err := walk(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range selectedIDs {
		if err := walk(id); err != nil {
			return nil, err
		}
	}
	if len(wanted) != len(refs) {
		return nil, errSwarmReuseProof
	}
	digests, err := swarmTaskInputDigests(state, p.ID)
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result := make([]ProjectSwarmReusedDependency, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if !wanted[ref.TaskID] || seen[ref.TaskID] || !validRunID(ref.RunID) || !hexDigest(ref.IntegrationDigest) {
			return nil, errSwarmReuseProof
		}
		seen[ref.TaskID] = true
		old, err := s.projectRuns.Get(p.ID, p.Path, ref.RunID)
		if err != nil {
			return nil, errSwarmReuseProof
		}
		task := swarmTask(&old, ref.TaskID)
		op := swarmOperation(&old, ref.OperationID)
		if task == nil || op == nil || task.OperationID != ref.OperationID || op.TaskID != ref.TaskID || op.Allocation != "ready" || (op.Status != "succeeded" && op.Status != "review-required" && op.Status != "failed") || task.Integration == nil || task.Integration.State != "integrated" || task.TaskInputDigest == "" || task.TaskInputDigest != digests[ref.TaskID] || len(task.WritePaths) == 0 || implementationDigest(task.Integration) != ref.IntegrationDigest {
			return nil, errSwarmReuseProof
		}
		if err = s.validateSwarmReuseSource(bounded, p, *task, head); err != nil {
			return nil, err
		}
		view, err := s.projectRuns.taskHandoffView(old, *op)
		if err != nil {
			return nil, err
		}
		// Retain unavailable reports as unavailable; never manufacture worker claims.
		result = append(result, ProjectSwarmReusedDependency{ProjectSwarmReuseReference: ref, Task: *task, OperationDigest: implementationDigest(op), HandoffViewDigest: implementationDigest(view), AdmittedSourceCommit: head, handoff: view})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TaskID < result[j].TaskID })
	return result, nil
}

func (s *Server) validateSwarmReuseSource(ctx context.Context, p project.Project, task ProjectSwarmTask, head string) error {
	if task.Integration == nil || len(task.WritePaths) == 0 {
		return errSwarmReuseProof
	}
	if _, err := s.journeyGit(ctx, p.Path, "merge-base", "--is-ancestor", task.Integration.Commit, head); err != nil {
		return errSwarmReuseProof
	}
	// Literal pathspecs prevent a saved wildcard/option from broadening this
	// proof. A directory scope checks every tracked entry below that directory.
	args := []string{"diff", "--quiet", task.Integration.Commit, head, "--"}
	for _, scope := range task.WritePaths {
		args = append(args, ":(literal)"+strings.TrimSuffix(scope, "/"))
	}
	if _, err := s.journeyGit(ctx, p.Path, args...); err != nil {
		return errSwarmReuseProof
	}
	return nil
}

func (s *Server) validateProjectSwarmReuseCurrent(ctx context.Context, p project.Project, run ProjectRun) error {
	if run.Swarm == nil || len(run.Swarm.ReusedDependencies) == 0 {
		return nil
	}
	refs := make([]ProjectSwarmReuseReference, 0, len(run.Swarm.ReusedDependencies))
	for _, dep := range run.Swarm.ReusedDependencies {
		if !validSwarmReuse(dep, run) {
			return errSwarmReuseProof
		}
		refs = append(refs, dep.ProjectSwarmReuseReference)
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	_, head, dirty, err := s.checkSource(ctx, p.Path)
	if err != nil || dirty {
		return errSwarmReuseProof
	}
	current, err := s.resolveSwarmReuse(ctx, p, state, run.Input.Implementation.TaskIDs, head, refs)
	if err != nil {
		return err
	}
	for i := range current {
		// New integrations in this run may advance HEAD. All other exact proofs
		// and frozen prior reports must remain identical.
		current[i].AdmittedSourceCommit = run.Input.Implementation.SourceCommit
		current[i].handoff = TaskHandoffView{}
		expected := run.Swarm.ReusedDependencies[i]
		expected.handoff = TaskHandoffView{}
		if !reflect.DeepEqual(current[i], expected) {
			return errSwarmReuseProof
		}
	}
	return nil
}

func (s *ProjectRunStore) reusedDependencyHandoff(run ProjectRun, dep ProjectSwarmReusedDependency) (TaskHandoffView, error) {
	var view TaskHandoffView
	if !validSwarmReuse(dep, run) {
		return view, ErrProjectRunInvalid
	}
	if err := s.readHandoffObject(run.ProjectID, dep.HandoffViewDigest, &view, TaskDependencyPacketLimit); err != nil {
		return view, err
	}
	if view.ProjectID != run.ProjectID || view.RunID != dep.RunID || view.TaskID != dep.TaskID || view.OperationID != dep.OperationID || implementationDigest(view.Integration) != dep.IntegrationDigest {
		return view, ErrProjectRunInvalid
	}
	return view, nil
}

// Every external receipt must be a reachable prerequisite, never extra proof
// that can silently broaden the selected task graph.
func swarmReuseReachable(run ProjectRun) bool {
	if run.Swarm == nil {
		return true
	}
	all := map[string][]string{}
	external := map[string]bool{}
	for _, task := range run.Swarm.Tasks {
		all[task.ID] = task.DependsOn
	}
	for _, dep := range run.Swarm.ReusedDependencies {
		all[dep.TaskID] = dep.Task.DependsOn
		external[dep.TaskID] = true
	}
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		if visited[id] {
			return true
		}
		deps, ok := all[id]
		if !ok {
			return false
		}
		visited[id] = true
		for _, dep := range deps {
			if !walk(dep) {
				return false
			}
		}
		return true
	}
	for _, task := range run.Swarm.Tasks {
		if !walk(task.ID) {
			return false
		}
	}
	for id := range external {
		if !visited[id] {
			return false
		}
	}
	return true
}
