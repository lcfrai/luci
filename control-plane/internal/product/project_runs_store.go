package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const projectRunFileLimit = 1 << 20
const projectRunHistoryLimit = 200
const projectRunOperationLimit = 100

var (
	ErrProjectRunNotFound    = errors.New("project run not found")
	ErrProjectRunConflict    = errors.New("project run changed; reload before continuing")
	ErrProjectRunKeyConflict = errors.New("idempotency key already names different project run inputs")
	ErrProjectRunInvalid     = errors.New("private project run state is invalid or belongs to another workspace")
	ErrProjectRunCancelled   = errors.New("project run is cancelled")
	ErrProjectRunPending     = errors.New("the previous operation needs observation before another attempt can start")
	ErrProjectRunLimit       = errors.New("project run history is full; retain/export existing evidence before starting more runs")
)

type ProjectRunRoute struct {
	Harness        string                 `json:"harness"`
	Model          string                 `json:"model"`
	PermissionMode string                 `json:"permissionMode"`
	Effort         string                 `json:"effort,omitempty"`
	Connection     AgentConnectionReceipt `json:"connection"`
}

// ProjectRunInput is constructed by admission code, never decoded from a client.
// Its snapshot is immutable even when the coding child changes the source.
type ProjectRunInput struct {
	Implementation registry.ImplementationContext `json:"implementation"`
	Approvals      ProjectRunApprovalSelection    `json:"approvals"`
	Route          ProjectRunRoute                `json:"route"`
	Outcome        string                         `json:"outcome"`
	Requirements   []RequirementSnapshot          `json:"requirements"`
}

type ProjectRunOperation struct {
	Handoff             *TaskHandoffReference           `json:"handoff,omitempty"`
	DependencyContext   *TaskDependencyReference        `json:"dependencyContext,omitempty"`
	Checkpoint          *ProjectSwarmCheckpoint         `json:"checkpoint,omitempty"`
	UnclaimedRetirement bool                            `json:"unclaimedRetirement,omitempty"`
	LaunchRecovery      *ProjectSwarmRecoveryIntent     `json:"launchRecovery,omitempty"`
	LaunchRecoveryProof *ProjectSwarmRecoveryProof      `json:"launchRecoveryProof,omitempty"`
	Recovery            *ProjectSwarmRecoveryView       `json:"recovery,omitempty"`
	AbsenceRetirement   *ProjectSwarmRecoveryProof      `json:"absenceRetirement,omitempty"`
	Integration         *ProjectSwarmIntegration        `json:"integration,omitempty"`
	TaskID              string                          `json:"taskId,omitempty"`
	WorktreeID          string                          `json:"worktreeId,omitempty"`
	BaseCommit          string                          `json:"baseCommit,omitempty"`
	Implementation      *registry.ImplementationContext `json:"implementation,omitempty"`
	Allocation          string                          `json:"allocation,omitempty"`
	ID                  string                          `json:"id"`
	Kind                string                          `json:"kind"`
	ChildID             string                          `json:"childId"`
	Attempt             int                             `json:"attempt"`
	Status              string                          `json:"status"`
	RequestDigest       string                          `json:"requestDigest"`
	CreatedAt           string                          `json:"createdAt"`
	UpdatedAt           string                          `json:"updatedAt"`
	Message             string                          `json:"message,omitempty"`
}

type ProjectRunCommandEvidence struct {
	CheckCommand
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
}

// Evidence deliberately excludes raw command output, environment and transcripts.
// The join adapter independently resolves and validates private child records.
type ProjectRunCheckEvidence struct {
	CheckID           string                      `json:"checkId"`
	ProjectID         string                      `json:"projectId"`
	Commit            string                      `json:"commit"`
	SourceDigest      string                      `json:"sourceDigest"`
	Approvals         ProjectRunApprovalSelection `json:"approvals"`
	Requirements      []RequirementSnapshot       `json:"requirements"`
	Commands          []ProjectRunCommandEvidence `json:"commands"`
	Execution         string                      `json:"execution"`
	CreatedAt         string                      `json:"createdAt"`
	JoinedAt          string                      `json:"joinedAt"`
	OriginalAvailable bool                        `json:"originalAvailable"`
}

type ProjectRunReleaseEvidence struct {
	ReleaseID    string                      `json:"releaseId"`
	ProjectID    string                      `json:"projectId"`
	CheckID      string                      `json:"checkId"`
	Target       string                      `json:"target"`
	Outcome      string                      `json:"outcome"`
	Commit       string                      `json:"commit"`
	SourceDigest string                      `json:"sourceDigest"`
	Approvals    ProjectRunApprovalSelection `json:"approvals"`
	ObservedAt   string                      `json:"observedAt"`
	URL          string                      `json:"url"`
	Limitations  string                      `json:"limitations"`
	JoinedAt     string                      `json:"joinedAt"`
}

type ProjectRunObservation struct {
	State       string `json:"state"`
	Reason      string `json:"reason"`
	ObservedAt  string `json:"observedAt"`
	Valid       bool   `json:"valid"`
	OperationID string `json:"operationId,omitempty"`
	ChildID     string `json:"childId,omitempty"`
	ChildStatus string `json:"childStatus,omitempty"`
}

type ProjectRunAcceptance struct {
	CheckID    string `json:"checkId"`
	ReleaseID  string `json:"releaseId"`
	AcceptedAt string `json:"acceptedAt"`
}

type ProjectRun struct {
	Swarm       *ProjectSwarm               `json:"swarm,omitempty"`
	ID          string                      `json:"id"`
	ProjectID   string                      `json:"projectId"`
	WorktreeID  string                      `json:"worktreeId"`
	Revision    int                         `json:"revision"`
	CreatedAt   string                      `json:"createdAt"`
	UpdatedAt   string                      `json:"updatedAt"`
	State       string                      `json:"state"`
	Reason      string                      `json:"reason"`
	Input       ProjectRunInput             `json:"input"`
	Operations  []ProjectRunOperation       `json:"operations"`
	Checks      []ProjectRunCheckEvidence   `json:"checks"`
	Releases    []ProjectRunReleaseEvidence `json:"releases"`
	Observation *ProjectRunObservation      `json:"observation,omitempty"`
	Acceptance  *ProjectRunAcceptance       `json:"acceptance,omitempty"`
}

type storedProjectRun struct {
	Version             int               `json:"version"`
	AccountScope        string            `json:"accountScope"`
	ProjectScope        string            `json:"projectScope"`
	CreateKeyDigest     string            `json:"createKeyDigest"`
	CreateRequestDigest string            `json:"createRequestDigest"`
	InputDigest         string            `json:"inputDigest"`
	CodexConnectionID   string            `json:"codexConnectionId,omitempty"`
	OperationKeys       map[string]string `json:"operationKeys"`
	Run                 ProjectRun        `json:"run"`
}

// One store belongs to one execution service. Its mutex serializes all run
// admissions for that account. Callers must reuse this instance, not construct
// one per HTTP request. The store never invokes an executor or repairs on read.
type ProjectRunStore struct {
	handoffMu    sync.Mutex
	mu           sync.Mutex
	stateDir     string
	accountScope string
}

func NewProjectRunStore(stateDir string) (*ProjectRunStore, error) {
	path, err := guardedProjectPath(stateDir, ".", false)
	if err != nil {
		return nil, err
	}
	path, err = hostpath.Real(path)
	if err != nil {
		return nil, err
	}
	return &ProjectRunStore{stateDir: path, accountScope: reviewDigest(path)}, nil
}

func (s *ProjectRunStore) projectScope(id, dir string) (string, error) {
	if !prototypeReleaseID.MatchString(id) || !filepath.IsAbs(dir) || project.ProjectID(dir) != id || stateOutsideProjects(dir, s.stateDir) != nil {
		return "", ErrProjectRunInvalid
	}
	path, err := guardedProjectPath(dir, ".", false)
	if err != nil {
		return "", err
	}
	path, err = hostpath.Real(path)
	if err != nil {
		return "", err
	}
	return reviewDigest(path), nil
}

func (s *ProjectRunStore) directory(projectID string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return "", ErrProjectRunInvalid
	}
	for _, relative := range []string{"project-runs", "project-runs/" + projectID} {
		path, err := guardedProjectPath(s.stateDir, relative, create)
		if err != nil {
			return "", err
		}
		if create {
			if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if _, err := guardedProjectPath(s.stateDir, relative, false); err != nil {
				return "", err
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(path)
			} else {
				err = os.Chmod(path, 0700)
			}
			if err != nil {
				return "", err
			}
			if err := syncProjectRunDirectory(filepath.Dir(path)); err != nil {
				return "", err
			}
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", ErrProjectRunInvalid
		}
	}
	return filepath.Join(s.stateDir, "project-runs", projectID), nil
}

func validRunID(id string) bool {
	return strings.HasPrefix(id, "run-") && len(id) == 28 && prototypeReleaseID.MatchString(id)
}

func (s *ProjectRunStore) read(projectID, scope, id string) (storedProjectRun, error) {
	var row storedProjectRun
	if !validRunID(id) {
		return row, ErrProjectRunNotFound
	}
	dir, err := s.directory(projectID, false)
	if errors.Is(err, os.ErrNotExist) {
		return row, ErrProjectRunNotFound
	}
	if err != nil {
		return row, err
	}
	path, err := guardedProjectPath(dir, id+".json", false)
	if errors.Is(err, os.ErrNotExist) {
		return row, ErrProjectRunNotFound
	}
	if err != nil {
		return row, err
	}
	if err = hostpath.OneName(path); err != nil {
		return row, ErrProjectRunInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return row, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > projectRunFileLimit {
		return row, ErrProjectRunInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, projectRunFileLimit+1))
	if err != nil {
		return row, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > projectRunFileLimit || decoder.Decode(&row) != nil || decoder.Decode(new(any)) != io.EOF || row.Version != 1 || row.AccountScope != s.accountScope || row.ProjectScope != scope || row.Run.ID != id || row.Run.ProjectID != projectID || validateStoredProjectRun(row) != nil {
		return storedProjectRun{}, ErrProjectRunInvalid
	}
	return row, nil
}

func (s *ProjectRunStore) save(row storedProjectRun) error {
	// Capture the compact head against the pre-write directory generation.
	// Legacy/stale heads are rebuilt only on explicit writes from a complete,
	// validated history. A GET never scans or repairs historical run payloads.
	overviewHead, overviewHeadErr := s.readOverviewRunHead(row.Run.ProjectID, row.ProjectScope)
	var overviewHistory []storedProjectRun
	if overviewHeadErr != nil || overviewHead.Version != overviewRunHeadVersion {
		overviewHistory, _ = s.list(row.Run.ProjectID, row.ProjectScope)
	}
	if err := validateStoredProjectRun(row); err != nil {
		return err
	}
	data, err := json.MarshalIndent(row, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > projectRunFileLimit {
		return ErrProjectRunLimit
	}
	dir, err := s.directory(row.Run.ProjectID, true)
	if err != nil {
		return err
	}
	path, err := guardedProjectPath(dir, row.Run.ID+".json", true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := hostpath.OneName(path); err != nil {
			return ErrProjectRunInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := durableRunJSON(path, row); err != nil {
		return err
	}
	if err := syncProjectRunDirectory(dir); err != nil {
		return err
	}
	if overviewHeadErr == nil && overviewHead.Version == overviewRunHeadVersion {
		updateOverviewRunAttention(&overviewHead, row.Run)
		_ = s.writeOverviewRunHead(overviewHead)
	} else if overviewHistory != nil {
		found := false
		for i := range overviewHistory {
			if overviewHistory[i].Run.ID == row.Run.ID {
				overviewHistory[i], found = row, true
				break
			}
		}
		if !found {
			overviewHistory = append(overviewHistory, row)
		}
		s.overviewHeadFromHistory(overviewHistory)
	}
	return nil
}

// Sync directory entries where supported. Dispatch receipts additionally use
// publishRunReceipt's write-through Windows replacement boundary.
func syncProjectRunDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *ProjectRunStore) list(projectID, scope string) ([]storedProjectRun, error) {
	dir, err := s.directory(projectID, false)
	if errors.Is(err, os.ErrNotExist) {
		return []storedProjectRun{}, nil
	}
	if err != nil {
		return nil, err
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(projectRunHistoryLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > projectRunHistoryLimit {
		return nil, ErrProjectRunLimit
	}
	rows := []storedProjectRun{}
	for _, entry := range entries {
		// An interrupted atomic write may leave a private temporary file. It is
		// never a run and is not removed as a side effect of observation.
		if strings.HasPrefix(entry.Name(), ".luci-") {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, ErrProjectRunInvalid
		}
		row, err := s.read(projectID, scope, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Run.CreatedAt > rows[j].Run.CreatedAt })
	return rows, nil
}

func (s *ProjectRunStore) Get(projectID, projectPath, id string) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, err
	}
	row, err := s.read(projectID, scope, id)
	return row.Run, err
}

func (s *ProjectRunStore) List(projectID, projectPath string) ([]ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return nil, err
	}
	rows, err := s.list(projectID, scope)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Run)
	}
	return out, nil
}

// Create is persistence only. The returned bool says whether this request
// created a record; even true supplies no authority to start a child.
func (s *ProjectRunStore) Create(projectID, projectPath, key string, input ProjectRunInput, codexConnectionID string, normalizedRequestDigest ...string) (ProjectRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, false, err
	}
	if !validRunKey(key) || validateRunInput(projectID, input) != nil || len(codexConnectionID) > 256 {
		return ProjectRun{}, false, ErrProjectRunInvalid
	}
	rows, err := s.list(projectID, scope)
	if err != nil {
		return ProjectRun{}, false, err
	}
	keyDigest, requestDigest := reviewDigest(key), reviewDigest(struct {
		Input             ProjectRunInput
		CodexConnectionID string
	}{input, codexConnectionID})
	inputDigest := requestDigest
	if len(normalizedRequestDigest) > 1 {
		return ProjectRun{}, false, ErrProjectRunInvalid
	}
	if len(normalizedRequestDigest) == 1 {
		if !hexDigest(normalizedRequestDigest[0]) {
			return ProjectRun{}, false, ErrProjectRunInvalid
		}
		requestDigest = normalizedRequestDigest[0]
	}
	for _, row := range rows {
		if row.CreateKeyDigest != keyDigest {
			continue
		}
		if row.CreateRequestDigest != requestDigest {
			return ProjectRun{}, false, ErrProjectRunKeyConflict
		}
		return row.Run, false, nil
	}
	if len(rows) >= projectRunHistoryLimit {
		return ProjectRun{}, false, ErrProjectRunLimit
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := storedProjectRun{Version: 1, AccountScope: s.accountScope, ProjectScope: scope, CreateKeyDigest: keyDigest, CreateRequestDigest: requestDigest, InputDigest: inputDigest, CodexConnectionID: codexConnectionID, OperationKeys: map[string]string{}, Run: ProjectRun{ID: newID("run-"), ProjectID: projectID, WorktreeID: input.Implementation.WorktreeID, Revision: 1, CreatedAt: now, UpdatedAt: now, State: "ready", Reason: "Reviewed inputs are saved. Start the coding session when ready.", Input: input, Operations: []ProjectRunOperation{}, Checks: []ProjectRunCheckEvidence{}, Releases: []ProjectRunReleaseEvidence{}}}
	if err := s.save(row); err != nil {
		return ProjectRun{}, false, err
	}
	s.overviewHeadAfterCreate(row, rows)
	// Return a fresh decode so caller-owned slices cannot alter the snapshot.
	saved, err := s.read(projectID, scope, row.Run.ID)
	return saved.Run, err == nil, err
}

// FindByKey runs before resolving new immutable inputs. A retried normalized
// client selection returns its original run even if that run edited source in
// the meantime. requestDigest is computed by the controller, never the client.
func (s *ProjectRunStore) FindByKey(projectID, projectPath, key, requestDigest string) (ProjectRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunKey(key) || !hexDigest(requestDigest) {
		return ProjectRun{}, false, ErrProjectRunInvalid
	}
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, false, err
	}
	rows, err := s.list(projectID, scope)
	if err != nil {
		return ProjectRun{}, false, err
	}
	for _, row := range rows {
		if row.CreateKeyDigest == reviewDigest(key) {
			if row.CreateRequestDigest != requestDigest {
				return ProjectRun{}, false, ErrProjectRunKeyConflict
			}
			return row.Run, true, nil
		}
	}
	return ProjectRun{}, false, nil
}

// FindOperationByKey precedes source revalidation for an operation retry. The
// public operation selection contains only kind; its route and reviewed task
// selection are already immutable in the run. Returning a reservation never
// grants dispatch ownership: callers still need ClaimOperation.
func (s *ProjectRunStore) FindOperationByKey(projectID, projectPath, id, key, kind string) (ProjectRun, ProjectRunOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunKey(key) || kind != "session" {
		return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunInvalid
	}
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	row, err := s.read(projectID, scope, id)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	operationID, exists := row.OperationKeys[reviewDigest(key)]
	if !exists {
		return row.Run, ProjectRunOperation{}, false, nil
	}
	for _, op := range row.Run.Operations {
		if op.ID == operationID {
			if op.Kind != kind {
				return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunKeyConflict
			}
			return row.Run, op, true, nil
		}
	}
	return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunInvalid
}

func validRunKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func validRunApprovals(a ProjectRunApprovalSelection) bool {
	return strings.HasPrefix(a.SpecificationApprovalID, "approval-") && len(a.SpecificationApprovalID) == 33 && strings.HasPrefix(a.DesignApprovalID, "approval-") && len(a.DesignApprovalID) == 33 && a.SpecificationRevision > 0 && a.DesignRevision > 0 && hexDigest(a.SpecificationDigest) && hexDigest(a.DesignDigest) && hexDigest(a.BriefDigest) && hexDigest(a.SketchDigest)
}

func validateRunInput(projectID string, in ProjectRunInput) error {
	c := in.Implementation
	if c.ProjectID != projectID || !prototypeReleaseID.MatchString(c.WorktreeID) || c.PlanRevision < 1 || len(c.TaskIDs) == 0 || len(c.TaskIDs) > 200 || len(c.TaskIDs) != len(c.TaskTitles) || !hexDigest(c.SourceDigest) || !hexDigest(c.BriefDigest) || !hexDigest(c.PlanDigest) || !validRunApprovals(in.Approvals) || (in.Outcome != "preview" && in.Outcome != "publish") || len(in.Requirements) > 100 {
		return ErrProjectRunInvalid
	}
	if c.SourceCommit != "" && !deliverySHA.MatchString(c.SourceCommit) {
		return ErrProjectRunInvalid
	}
	if c.BriefDigest != in.Approvals.BriefDigest || c.SpecificationRevision != in.Approvals.SpecificationRevision || c.SpecificationDigest != in.Approvals.SpecificationDigest || c.DesignPlanRevision != in.Approvals.DesignRevision || c.DesignPlanDigest != in.Approvals.DesignDigest {
		return ErrProjectRunInvalid
	}
	if !validConnectionToken(in.Route.Harness, 128, false) || !validConnectionToken(in.Route.Model, 256, false) || (in.Route.PermissionMode != "ask" && in.Route.PermissionMode != "plan" && in.Route.PermissionMode != "bypass") {
		return ErrProjectRunInvalid
	}
	if len(in.Route.Connection.ID) > 128 || len(in.Route.Connection.Endpoint) > 2048 || len(in.Route.Connection.Kind) > 128 || len(in.Route.Connection.Protocol) > 128 || len(in.Route.Connection.AuthMode) > 128 || in.Route.Connection.Revision < 0 {
		return ErrProjectRunInvalid
	}
	if len(in.Route.Effort) > 32 || len(in.Route.Connection.Model) > 256 {
		return ErrProjectRunInvalid
	}
	tasks := map[string]bool{}
	for i, id := range c.TaskIDs {
		if !designIDPattern.MatchString(id) || tasks[id] || strings.TrimSpace(c.TaskTitles[i]) == "" || len(c.TaskTitles[i]) > 2000 {
			return ErrProjectRunInvalid
		}
		tasks[id] = true
	}
	if validateRequirementIDs(c.RequirementIDs) != nil {
		return ErrProjectRunInvalid
	}
	seen := map[string]bool{}
	for _, requirement := range in.Requirements {
		if !designIDPattern.MatchString(requirement.ID) || seen[requirement.ID] || len(requirement.Title) > 2000 || len(requirement.Acceptance) > 16000 {
			return ErrProjectRunInvalid
		}
		seen[requirement.ID] = true
	}
	for _, id := range c.RequirementIDs {
		if !seen[id] {
			return ErrProjectRunInvalid
		}
	}
	return nil
}

func validateStoredProjectRun(row storedProjectRun) error {
	r := row.Run
	if err := validateProjectSwarm(r); err != nil {
		return err
	}
	if row.Version != 1 || !hexDigest(row.AccountScope) || !hexDigest(row.ProjectScope) || !hexDigest(row.CreateKeyDigest) || !hexDigest(row.CreateRequestDigest) || !validRunID(r.ID) || r.Revision < 1 || r.WorktreeID != r.Input.Implementation.WorktreeID || validateRunInput(r.ProjectID, r.Input) != nil || len(r.Operations) > projectRunOperationLimit || len(r.Checks) > 100 || len(r.Releases) > 100 || row.OperationKeys == nil || len(row.OperationKeys) != len(r.Operations) {
		return ErrProjectRunInvalid
	}
	if !validRunState(r.State) || len(r.Reason) > 4000 {
		return ErrProjectRunInvalid
	}
	if row.InputDigest != reviewDigest(struct {
		Input             ProjectRunInput
		CodexConnectionID string
	}{r.Input, row.CodexConnectionID}) {
		return ErrProjectRunInvalid
	}
	for _, at := range []string{r.CreatedAt, r.UpdatedAt} {
		if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
			return ErrProjectRunInvalid
		}
	}
	seen := map[string]bool{}
	children := map[string]bool{}
	keyed := map[string]bool{}
	attempts := map[string]int{}
	for _, op := range r.Operations {
		attempts[op.Kind]++
		if !strings.HasPrefix(op.ID, "op-") || len(op.ID) != 27 || !prototypeReleaseID.MatchString(op.ID) || seen[op.ID] || op.Kind != "session" || !strings.HasPrefix(op.ChildID, "s-") || len(op.ChildID) != 10 || !prototypeReleaseID.MatchString(op.ChildID) || children[op.ChildID] || op.Attempt != attempts[op.Kind] || !hexDigest(op.RequestDigest) || !validOperationStatus(op.Status) || len(op.Message) > 4000 {
			return ErrProjectRunInvalid
		}
		for _, at := range []string{op.CreatedAt, op.UpdatedAt} {
			if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
				return ErrProjectRunInvalid
			}
		}
		seen[op.ID] = true
		children[op.ChildID] = true
	}
	for key, id := range row.OperationKeys {
		if !hexDigest(key) || !seen[id] || keyed[id] {
			return ErrProjectRunInvalid
		}
		keyed[id] = true
	}
	checks := map[string]ProjectRunCheckEvidence{}
	for _, check := range r.Checks {
		if _, exists := checks[check.CheckID]; exists || validProjectRunCheck(r.Input, check) != nil {
			return ErrProjectRunInvalid
		}
		checks[check.CheckID] = check
	}
	releases := map[string]ProjectRunReleaseEvidence{}
	for _, release := range r.Releases {
		if _, exists := releases[release.ReleaseID]; exists || validProjectRunRelease(r.Input, checks[release.CheckID], release) != nil {
			return ErrProjectRunInvalid
		}
		releases[release.ReleaseID] = release
	}
	if r.Observation != nil {
		if !validRunState(r.Observation.State) || len(r.Observation.Reason) > 4000 || len(r.Observation.ChildStatus) > 32 {
			return ErrProjectRunInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, r.Observation.ObservedAt); err != nil {
			return ErrProjectRunInvalid
		}
		if r.Observation.OperationID != "" && !seen[r.Observation.OperationID] {
			return ErrProjectRunInvalid
		}
		if r.Observation.ChildID != "" && !children[r.Observation.ChildID] {
			return ErrProjectRunInvalid
		}
	}
	if (r.State == "complete") != (r.Acceptance != nil) {
		return ErrProjectRunInvalid
	}
	if r.Acceptance != nil {
		release, exists := releases[r.Acceptance.ReleaseID]
		if !exists || release.CheckID != r.Acceptance.CheckID {
			return ErrProjectRunInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, r.Acceptance.AcceptedAt); err != nil {
			return ErrProjectRunInvalid
		}
	}
	return nil
}

func validRunState(state string) bool {
	switch state {
	case "ready", "coding", "awaiting-checks", "awaiting-release", "blocked", "cancelled", "complete":
		return true
	}
	return false
}

func validOperationStatus(status string) bool {
	switch status {
	case "reserved", "dispatching", "running", "unknown", "succeeded", "review-required", "failed", "cancelled":
		return true
	}
	return false
}

func projectRunOperationTerminal(status string) bool {
	return status == "succeeded" || status == "review-required" || status == "failed" || status == "cancelled"
}

func (s *ProjectRunStore) mutate(projectID, projectPath, id string, expectedRevision int, change func(*storedProjectRun) (bool, error)) (ProjectRun, error) {
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, err
	}
	row, err := s.read(projectID, scope, id)
	if err != nil {
		return ProjectRun{}, err
	}
	if expectedRevision != row.Run.Revision {
		return ProjectRun{}, ErrProjectRunConflict
	}
	changed, err := change(&row)
	if err != nil {
		return ProjectRun{}, err
	}
	if !changed {
		return row.Run, nil
	}
	row.Run.Revision++
	row.Run.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = s.save(row); err != nil {
		return ProjectRun{}, err
	}
	return row.Run, nil
}

// ReserveOperation persists its child ID before any session record or sandbox
// exists. Duplicate keys return the original reservation, even with an old
// revision. Changed payloads under the same key fail rather than retrying.
func (s *ProjectRunStore) ReserveOperation(projectID, projectPath, id string, expectedRevision int, key, kind, requestDigest string) (ProjectRun, ProjectRunOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunKey(key) || kind != "session" || !hexDigest(requestDigest) {
		return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunInvalid
	}
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	row, err := s.read(projectID, scope, id)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	if operationID, ok := row.OperationKeys[reviewDigest(key)]; ok {
		for _, op := range row.Run.Operations {
			if op.ID == operationID {
				if op.Kind != kind || op.RequestDigest != requestDigest {
					return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunKeyConflict
				}
				return row.Run, op, false, nil
			}
		}
	}
	var op ProjectRunOperation
	run, err := s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, ErrProjectRunCancelled
		}
		if row.Run.State == "complete" || row.Run.Swarm != nil {
			return false, ErrProjectRunConflict
		}
		if len(row.Run.Operations) >= projectRunOperationLimit {
			return false, ErrProjectRunLimit
		}
		attempt := 1
		for _, prior := range row.Run.Operations {
			if prior.Kind == kind {
				if prior.Status == "reserved" || prior.Status == "dispatching" || prior.Status == "running" || prior.Status == "unknown" {
					return false, ErrProjectRunPending
				}
				attempt++
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		op = ProjectRunOperation{ID: newID("op-"), Kind: kind, ChildID: ids.New("s"), Attempt: attempt, Status: "reserved", RequestDigest: requestDigest, CreatedAt: now, UpdatedAt: now}
		row.Run.Operations = append(row.Run.Operations, op)
		row.OperationKeys[reviewDigest(key)] = op.ID
		row.Run.State = "ready"
		row.Run.Reason = "Coding session is reserved. No coding outcome is verified."
		row.Run.Observation = nil
		return true, nil
	})
	return run, op, err == nil, err
}

// ClaimOperation is the only transition granting dispatch ownership. The
// caller must not dispatch when claimed is false. Unknown/dispatching work is
// observed by identity; it is never automatically replayed after interruption.
func (s *ProjectRunStore) ClaimOperation(projectID, projectPath, id string, expectedRevision int, operationID string) (ProjectRun, ProjectRunOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var op ProjectRunOperation
	claimed := false
	run, err := s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, ErrProjectRunCancelled
		}
		for i := range row.Run.Operations {
			current := &row.Run.Operations[i]
			if current.ID != operationID {
				continue
			}
			op = *current
			if row.Run.Swarm != nil && (row.Run.Swarm.State != "ready" || current.TaskID == "" || current.Allocation != "ready") {
				return false, ErrProjectRunConflict
			}
			if current.Status != "reserved" {
				return false, nil
			}
			current.Status = "dispatching"
			current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			op = *current
			claimed = true
			row.Run.State = "coding"
			row.Run.Reason = "Coding session dispatch is recorded; observe its actual child before retrying."
			return true, nil
		}
		return false, ErrProjectRunNotFound
	})
	return run, op, claimed && err == nil, err
}

func (s *ProjectRunStore) ObserveOperation(projectID, projectPath, id string, expectedRevision int, operationID, childID, status, message string) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validOperationStatus(status) || status == "reserved" || status == "dispatching" || len(message) > 4000 {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		for i := range row.Run.Operations {
			op := &row.Run.Operations[i]
			if op.ID != operationID {
				continue
			}
			if op.ChildID != childID {
				return false, ErrProjectRunInvalid
			}
			if op.LaunchRecovery != nil {
				return false, ErrProjectRunPending
			}
			if projectRunOperationTerminal(op.Status) && op.Status != status {
				return false, ErrProjectRunConflict
			}
			if op.Status == status && op.Message == message {
				return false, nil
			}
			op.Status = status
			op.Message = message
			op.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if row.Run.State != "cancelled" && row.Run.State != "complete" {
				switch status {
				case "running":
					row.Run.State = "coding"
				case "succeeded", "review-required", "cancelled":
					row.Run.State = "awaiting-checks"
				case "failed", "unknown":
					row.Run.State = "blocked"
				}
				row.Run.Reason = message
			}
			return true, nil
		}
		return false, ErrProjectRunNotFound
	})
}

func (s *ProjectRunStore) Cancel(projectID, projectPath, id string, expectedRevision int) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, nil
		}
		if row.Run.State == "complete" {
			return false, ErrProjectRunConflict
		}
		row.Run.State = "cancelled"
		row.Run.Reason = "Future dispatch is cancelled. Use the child's explicit stop control to stop work already running."
		for i := range row.Run.Operations {
			if row.Run.Operations[i].Status == "reserved" {
				row.Run.Operations[i].Status = "cancelled"
				row.Run.Operations[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			}
		}
		return true, nil
	})
}

// Observe records an explicit reconciliation, never a GET or a boot repair.
// Completion is reserved for Accept, after valid check and release joins.
func (s *ProjectRunStore) Observe(projectID, projectPath, id string, expectedRevision int, observation ProjectRunObservation) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunState(observation.State) || observation.State == "complete" || observation.State == "cancelled" || len(observation.Reason) > 4000 {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	observation.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		row.Run.Observation = &observation
		if row.Run.State != "complete" && row.Run.State != "cancelled" {
			row.Run.State = observation.State
			row.Run.Reason = observation.Reason
		}
		return true, nil
	})
}

func (s *ProjectRunStore) PrivateCodexConnectionID(projectID, projectPath, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope, err := s.projectScope(projectID, projectPath)
	if err != nil {
		return "", err
	}
	row, err := s.read(projectID, scope, id)
	if err != nil {
		return "", err
	}
	return row.CodexConnectionID, nil
}

func validProjectRunCheck(input ProjectRunInput, check ProjectRunCheckEvidence) error {
	if !prototypeReleaseID.MatchString(check.CheckID) || check.ProjectID != input.Implementation.ProjectID || !deliverySHA.MatchString(check.Commit) || !hexDigest(check.SourceDigest) || check.Approvals != input.Approvals || len(check.Commands) == 0 || len(check.Commands) > 8 || len(input.Requirements) == 0 || len(check.Requirements) > 100 {
		return runEvidenceError("check lacks complete source, review and command evidence")
	}
	if check.Execution != "docker" && check.Execution != "host" {
		return runEvidenceError("check execution is unknown")
	}
	coverage := map[string]RequirementSnapshot{}
	for _, requirement := range check.Requirements {
		if _, exists := coverage[requirement.ID]; exists {
			return runEvidenceError("check requirement coverage is ambiguous")
		}
		coverage[requirement.ID] = requirement
	}
	for _, requirement := range input.Requirements {
		if coverage[requirement.ID] != requirement {
			return runEvidenceError("check does not cover every selected requirement")
		}
	}
	for _, command := range check.Commands {
		if command.Status != "passed" || command.ExitCode != 0 || len(command.Label) > 2000 || !checkExecutable.MatchString(command.Executable) || len(command.Args) > 64 {
			return runEvidenceError("every recorded command must have passed")
		}
		for _, arg := range command.Args {
			if len(arg) > 16000 || strings.ContainsRune(arg, 0) {
				return runEvidenceError("invalid recorded command arguments")
			}
		}
	}
	return nil
}

func validProjectRunRelease(input ProjectRunInput, check ProjectRunCheckEvidence, release ProjectRunReleaseEvidence) error {
	if !prototypeReleaseID.MatchString(release.ReleaseID) || release.ProjectID != input.Implementation.ProjectID || release.Approvals != input.Approvals || release.CheckID != check.CheckID || release.Commit != check.Commit || release.SourceDigest != check.SourceDigest || release.Outcome != input.Outcome || !prototypeReleaseID.MatchString(release.Target) || len(release.URL) > 4096 || len(release.Limitations) > 4000 {
		return runEvidenceError("release does not match the checked candidate and intended outcome")
	}
	local := release.Target == "local" || release.Target == "native-docker"
	if input.Outcome == "preview" && !local {
		return runEvidenceError("a preview run needs a local preview")
	}
	if input.Outcome == "publish" && local {
		return runEvidenceError("a local preview is not a public deployment")
	}
	if _, err := time.Parse(time.RFC3339Nano, release.ObservedAt); err != nil {
		return runEvidenceError("release lacks a supported health observation")
	}
	return nil
}

// JoinCheck accepts only the bounded snapshot produced by the private evidence
// adapter. It cannot execute checks, and does not rely on ordinary history's
// later retention. A second candidate is appended, never substituted for one.
func (s *ProjectRunStore) JoinCheck(projectID, projectPath, id string, expectedRevision int, check ProjectRunCheckEvidence) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, ErrProjectRunCancelled
		}
		if row.Run.State == "complete" {
			return false, ErrProjectRunConflict
		}
		if err := validProjectRunCheck(row.Run.Input, check); err != nil {
			return false, err
		}
		for _, existing := range row.Run.Checks {
			if existing.CheckID == check.CheckID {
				copy := check
				copy.JoinedAt = existing.JoinedAt
				copy.OriginalAvailable = existing.OriginalAvailable
				if reviewDigest(copy) != reviewDigest(existing) {
					return false, runEvidenceError("the recorded check identity changed")
				}
				return false, nil
			}
		}
		if len(row.Run.Checks) >= 100 {
			return false, ErrProjectRunLimit
		}
		check.JoinedAt = time.Now().UTC().Format(time.RFC3339Nano)
		row.Run.Checks = append(row.Run.Checks, check)
		row.Run.State = "awaiting-release"
		row.Run.Reason = "Checks are linked to the reviewed requirements and source. Join a matching observed release next."
		// Previous observations cannot certify this newly selected candidate.
		row.Run.Observation = nil
		return true, nil
	})
}

// JoinRelease is observation-only. The adapter must establish fresh ownership,
// health and current reviews before passing this private snapshot to the store.
func (s *ProjectRunStore) JoinRelease(projectID, projectPath, id string, expectedRevision int, release ProjectRunReleaseEvidence) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, ErrProjectRunCancelled
		}
		if row.Run.State == "complete" {
			return false, ErrProjectRunConflict
		}
		if len(row.Run.Checks) == 0 {
			return false, runEvidenceError("join a verified check before its release")
		}
		check := row.Run.Checks[len(row.Run.Checks)-1]
		if release.CheckID == "" {
			release.CheckID = check.CheckID
		}
		if err := validProjectRunRelease(row.Run.Input, check, release); err != nil {
			return false, err
		}
		observed, _ := time.Parse(time.RFC3339Nano, release.ObservedAt)
		if time.Since(observed) > 5*time.Minute || observed.After(time.Now().Add(time.Minute)) {
			return false, runEvidenceError("refresh the release before joining it")
		}
		for _, existing := range row.Run.Releases {
			if existing.ReleaseID == release.ReleaseID {
				copy := release
				copy.JoinedAt = existing.JoinedAt
				copy.ObservedAt = existing.ObservedAt
				if reviewDigest(copy) != reviewDigest(existing) {
					return false, runEvidenceError("the recorded release identity changed")
				}
				return false, nil
			}
		}
		if len(row.Run.Releases) >= 100 {
			return false, ErrProjectRunLimit
		}
		release.JoinedAt = time.Now().UTC().Format(time.RFC3339Nano)
		row.Run.Releases = append(row.Run.Releases, release)
		row.Run.State = "awaiting-release"
		row.Run.Reason = "A matching release has been observed. Review the behavior and explicitly accept this outcome."
		row.Run.Observation = &ProjectRunObservation{State: row.Run.State, Reason: row.Run.Reason, ObservedAt: release.ObservedAt, Valid: true}
		return true, nil
	})
}

// Accept records explicit human acceptance after the controller has revalidated
// current source/reviews and freshly observed the selected release. No client
// completion flag can replace the immutable technical evidence joins.
func (s *ProjectRunStore) Accept(projectID, projectPath, id string, expectedRevision int) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, projectPath, id, expectedRevision, func(row *storedProjectRun) (bool, error) {
		if row.Run.State == "cancelled" {
			return false, ErrProjectRunCancelled
		}
		if row.Run.State == "complete" {
			return false, nil
		}
		if len(row.Run.Checks) == 0 || len(row.Run.Releases) == 0 || row.Run.Observation == nil || !row.Run.Observation.Valid {
			return false, runEvidenceError("current check and release evidence is required before acceptance")
		}
		for _, op := range row.Run.Operations {
			if op.Status == "reserved" || op.Status == "dispatching" || op.Status == "running" || op.Status == "unknown" {
				return false, ErrProjectRunPending
			}
		}
		observed, err := time.Parse(time.RFC3339Nano, row.Run.Observation.ObservedAt)
		if err != nil || time.Since(observed) > 5*time.Minute || observed.After(time.Now().Add(time.Minute)) {
			return false, runEvidenceError("refresh the release before accepting this outcome")
		}
		check := row.Run.Checks[len(row.Run.Checks)-1]
		release := row.Run.Releases[len(row.Run.Releases)-1]
		if err := validProjectRunCheck(row.Run.Input, check); err != nil {
			return false, err
		}
		if err := validProjectRunRelease(row.Run.Input, check, release); err != nil {
			return false, err
		}
		row.Run.Acceptance = &ProjectRunAcceptance{CheckID: check.CheckID, ReleaseID: release.ReleaseID, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		row.Run.State = "complete"
		row.Run.Reason = "The selected outcome has matching technical evidence and explicit acceptance."
		return true, nil
	})
}

func runEvidenceError(message string) error { return fmt.Errorf("project run evidence: %s", message) }
