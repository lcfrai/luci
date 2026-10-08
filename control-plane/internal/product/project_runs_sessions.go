package product

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// ProjectRunSessionAdmission holds the account dispatch lock until the existing
// session handler has finished its registry write and sandbox allocation.
type ProjectRunSessionAdmission struct {
	SessionID  string
	Existing   *registry.Session
	Revalidate func() error
	Release    func()
}

// ProjectRunSessionRequestDigest binds one attempt's real starting source and
// selected route. Private authentication generations remain outside this public
// digest and are checked against the run's private record separately.
func ProjectRunSessionRequestDigest(candidate registry.Session) string {
	return implementationDigest(struct {
		ProjectID, WorktreeID, Harness, Model, Effort, PermissionMode, AuthMode string
		AgentConnectionID, AgentConnectionKind, AgentConnectionProtocol         string
		AgentConnectionRevision                                                 int
		Implementation                                                          *registry.ImplementationContext
	}{candidate.ProjectID, candidate.WorktreeID, candidate.Harness, candidate.Model, candidate.Effort,
		candidate.PermissionMode, candidate.AuthMode, candidate.AgentConnectionID, candidate.AgentConnectionKind,
		candidate.AgentConnectionProtocol, candidate.AgentConnectionRevision, candidate.ImplementationContext})
}

// SetProjectRunSessionObserver wires the owned private registry through the API
// composition root. The callback observes existing children only.
func (s *Server) SetProjectRunSessionObserver(observer func(context.Context, string, string, string) ([]registry.Session, error)) {
	s.projectRunSessionObserver = observer
}

func projectRunOperation(run ProjectRun, operationID string) (ProjectRunOperation, error) {
	for _, op := range run.Operations {
		if op.ID == operationID && op.Kind == "session" {
			return op, nil
		}
	}
	return ProjectRunOperation{}, ErrProjectRunNotFound
}

func sameRunReviewedInputs(a, b *registry.ImplementationContext) bool {
	if a == nil || b == nil {
		return false
	}
	x, y := a.Clone(), b.Clone()
	x.SourceCommit, x.SourceDigest, x.SourceDirty = "", "", false
	y.SourceCommit, y.SourceDigest, y.SourceDirty = "", "", false
	return reflect.DeepEqual(x, y)
}

func projectRunSessionBindingMatches(run ProjectRun, op ProjectRunOperation, candidate registry.Session) bool {
	route := run.Input.Route
	worktreeID, implementation := run.WorktreeID, &run.Input.Implementation
	if op.TaskID != "" {
		worktreeID, implementation = op.WorktreeID, op.Implementation
	}
	return candidate.ProjectID == run.ProjectID && candidate.WorktreeID == worktreeID &&
		candidate.ProjectRunID == run.ID && candidate.ProjectRunOperationID == op.ID &&
		candidate.Harness == route.Harness && candidate.Model == route.Model &&
		candidate.Effort == route.Effort && candidate.PermissionMode == route.PermissionMode &&
		candidate.AgentConnectionID == route.Connection.ID && candidate.AgentConnectionRevision == route.Connection.Revision &&
		candidate.AgentConnectionKind == route.Connection.Kind && candidate.AgentConnectionProtocol == route.Connection.Protocol &&
		candidate.AuthMode == route.Connection.AuthMode && !candidate.AuthOnly &&
		sameRunReviewedInputs(candidate.ImplementationContext, implementation) &&
		ProjectRunSessionRequestDigest(candidate) == op.RequestDigest
}

func (s *Server) findProjectRunSessionLocked(projectPath string, run ProjectRun, op ProjectRunOperation, lookup func(string) (registry.Session, error)) (*registry.Session, error) {
	if lookup == nil {
		return nil, errors.New("The private session registry is unavailable. Use Check run status before continuing.")
	}
	child, err := lookup(op.ChildID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, errors.New("The reserved coding session could not be read. Its outcome remains unknown.")
	}
	found := err == nil
	if s.projectRunSessionObserver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rows, observationErr := s.projectRunSessionObserver(ctx, run.ProjectID, run.ID, op.ID)
		cancel()
		if observationErr != nil {
			return nil, errors.New("The reserved coding session could not be observed. Use Check run status before continuing.")
		}
		if len(rows) > 1 || len(rows) == 1 && (!found || rows[0].ID != op.ChildID || !projectRunSessionBindingMatches(run, op, rows[0])) || found && len(rows) != 1 {
			return nil, errors.New("Conflicting coding session identities were found. This run cannot dispatch another child.")
		}
	}
	if found {
		generation, err := s.projectRuns.PrivateCodexConnectionID(run.ProjectID, projectPath, run.ID)
		if err != nil || child.ID != op.ChildID || !projectRunSessionBindingMatches(run, op, child) || child.CodexConnectionID != generation {
			return nil, errors.New("The saved coding session does not match this reserved project run operation.")
		}
		copy := child.Clone()
		return &copy, nil
	}
	if op.Status != "reserved" {
		return nil, errors.New("This operation began dispatch but its reserved coding session is missing. Its outcome is unknown; use Check run status instead of replaying it.")
	}
	return nil, nil
}

// FindProjectRunSession lets a duplicate request recover its immutable child
// before mutable connection defaults, source contents or account occupancy are
// considered. It never claims, repairs, persists or allocates an operation.
func (s *Server) FindProjectRunSession(projectID, runID, operationID string, lookup func(string) (registry.Session, error)) (*registry.Session, error) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	p, err := s.resolveProject(projectID)
	if err != nil {
		return nil, ErrProjectRunNotFound
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return nil, err
	}
	op, err := projectRunOperation(run, operationID)
	if err != nil {
		return nil, err
	}
	return s.findProjectRunSessionLocked(p.Path, run, op, lookup)
}

// BeginProjectRunSession grants a persisted reservation once. A crash before the
// caller writes its registry child leaves a dispatching operation; its absence
// cannot become permission to allocate again after restart.
// The caller holds Git/session admission through backend allocation, which also
// fences publication of new orchestrator questions and approved amendments.
func (s *Server) BeginProjectRunSession(projectID, runID, operationID string, candidate registry.Session, lookup func(string) (registry.Session, error)) (ProjectRunSessionAdmission, error) {
	s.projectRunDispatchMu.Lock()
	var once sync.Once
	release := func() { once.Do(s.projectRunDispatchMu.Unlock) }
	fail := func(err error) (ProjectRunSessionAdmission, error) {
		release()
		return ProjectRunSessionAdmission{}, err
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return fail(ErrProjectRunNotFound)
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return fail(err)
	}
	op, err := projectRunOperation(run, operationID)
	if err != nil {
		return fail(err)
	}
	if !projectRunSessionBindingMatches(run, op, candidate) {
		return fail(errors.New("The requested coding inputs or model route differ from the reserved operation. Refresh the saved run before continuing."))
	}
	existing, err := s.findProjectRunSessionLocked(p.Path, run, op, lookup)
	if err != nil {
		return fail(err)
	}
	if existing != nil {
		return ProjectRunSessionAdmission{SessionID: op.ChildID, Existing: existing, Release: release}, nil
	}
	validate := func() error {
		if err := s.OrchestratorExecutionGate(projectID); err != nil {
			return err
		}
		current, err := s.projectRuns.Get(projectID, p.Path, runID)
		if err != nil {
			return err
		}
		if current.State == "cancelled" {
			return ErrProjectRunCancelled
		}
		if current.State == "complete" {
			return ErrProjectRunConflict
		}
		sourcePath := p.Path
		if op.TaskID != "" {
			if current.Swarm == nil || current.Swarm.State != "ready" || op.Allocation != "ready" {
				return ErrProjectRunConflict
			}
			sourcePath, err = s.swarmPath(projectID, runID, operationID)
			if err != nil {
				return err
			}
		}
		if project.WorktreeID(p.Path) != run.WorktreeID {
			return errors.New("The reviewed run requires its original main checkout.")
		}
		if err := s.validateProjectRunApprovals(projectID, run.Input.Approvals); err != nil {
			return err
		}
		if err := s.ValidateImplementationContext(projectID, candidate.WorktreeID, sourcePath, candidate.ImplementationContext); err != nil {
			return err
		}
		selected := run.Input.Route.Connection.ID
		connection, _, err := s.ResolveAgentConnection(run.Input.Route.Harness, &selected)
		if err != nil {
			return err
		}
		if connection.Kind == "legacy" && candidate.CodexConnectionID != "" {
			connection.AuthMode = "chatgpt"
		}
		if connection != run.Input.Route.Connection {
			return errors.New("The run's selected model connection changed. Start a new run with the intended account.")
		}
		generation, err := s.projectRuns.PrivateCodexConnectionID(projectID, p.Path, runID)
		if err != nil || generation != candidate.CodexConnectionID {
			return errors.New("The retained subscription account changed. Review the selected account before starting another run.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		digest, commit, dirty, err := s.checkSource(ctx, sourcePath)
		if err != nil || candidate.ImplementationContext.SourceDigest != digest || candidate.ImplementationContext.SourceCommit != commit || candidate.ImplementationContext.SourceDirty != dirty {
			return errors.New("The source changed after this coding attempt was reserved. Use Check run status before reserving a new attempt.")
		}
		return nil
	}
	if err := validate(); err != nil {
		return fail(err)
	}
	_, claimedOperation, claimed, err := s.projectRuns.ClaimOperation(projectID, p.Path, runID, run.Revision, operationID)
	if err != nil {
		return fail(err)
	}
	if !claimed || claimedOperation.ChildID != op.ChildID {
		return fail(errors.New("This coding operation has already begun. Use Check run status before continuing."))
	}
	return ProjectRunSessionAdmission{SessionID: op.ChildID, Revalidate: validate, Release: release}, nil
}
