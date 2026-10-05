package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const branchExecutionBudget = 45 * time.Second

var branchOperationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{7,79}$`)
var branchProjectID = regexp.MustCompile(`^p-[0-9a-f]{8}$`)

type BranchState struct {
	project.Branch
	Commit     string `json:"commit"`
	CheckedOut bool   `json:"checkedOut"`
	CanSwitch  bool   `json:"canSwitch"`
	Message    string `json:"message"`
}

type BranchCatalogue struct {
	primary         project.Worktree
	ProjectID       string           `json:"projectId"`
	CurrentBranchID string           `json:"currentBranchId"`
	CurrentBranch   string           `json:"currentBranch"`
	Commit          string           `json:"commit"`
	Clean           bool             `json:"clean"`
	CanCreate       bool             `json:"canCreate"`
	CanSwitch       bool             `json:"canSwitch"`
	Message         string           `json:"message"`
	Branches        []BranchState    `json:"branches"`
	Operation       *BranchOperation `json:"operation,omitempty"`
}

type BranchOperation struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	BranchID       string `json:"branchId"`
	Branch         string `json:"branch"`
	SourceBranchID string `json:"sourceBranchId"`
	SourceCommit   string `json:"sourceCommit"`
	TargetCommit   string `json:"targetCommit"`
	Commit         string `json:"commit,omitempty"`
	Message        string `json:"message"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type BranchWorktree struct {
	project.Worktree
	SessionCount int  `json:"sessionCount"`
	Ready        bool `json:"ready"`
}

type BranchResult struct {
	Operation BranchOperation `json:"operation"`
	Branch    *project.Branch `json:"branch,omitempty"`
	Worktree  *BranchWorktree `json:"worktree,omitempty"`
	Commit    string          `json:"commit,omitempty"`
}

type branchMutation struct {
	Name                 string `json:"name,omitempty"`
	BranchID             string `json:"branchId,omitempty"`
	ExpectedBranchID     string `json:"expectedBranchId"`
	ExpectedCommit       string `json:"expectedCommit"`
	ExpectedTargetCommit string `json:"expectedTargetCommit,omitempty"`
	OperationID          string `json:"operationId"`
}

type branchReceipt struct {
	Request branchMutation `json:"request"`
	Result  BranchResult   `json:"result"`
}

func (s *Server) registerProjectBranches(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/git/branches", s.getProjectBranches)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/branches", s.createProjectBranch)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/switch", s.switchProjectBranch)
	mux.HandleFunc("GET /v1/product/projects/{id}/git/branch-operations/{operationId}", s.getBranchOperation)
}

type branchSourceRegistrar struct {
	base   Registrar
	server *Server
}

// All existing source routes share checkout admission. Their underlying
// handlers still own their normal state locks; a checkout never takes the
// broad journey mutex or makes an unrelated project's source wait for Git.
func (s *Server) guardProjectSourceRoutes(mux Registrar) Registrar {
	if _, wrapped := mux.(*branchSourceRegistrar); wrapped {
		return mux
	}
	return &branchSourceRegistrar{base: mux, server: s}
}

func (g *branchSourceRegistrar) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if !strings.Contains(pattern, "/v1/product/projects/{id}/") || strings.Contains(pattern, "/git/branches") || strings.HasSuffix(pattern, "/git/switch") || strings.Contains(pattern, "/git/branch-operations/") || strings.Contains(pattern, "/releases/{releaseId}/") || strings.HasSuffix(pattern, "/configuration") || strings.Contains(pattern, "/checks/{runId}") {
		g.base.HandleFunc(pattern, handler)
		return
	}
	g.base.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		release, err := g.server.AcquireGitSessionAdmission(r.Context(), r.PathValue("id"))
		if err != nil {
			if errors.Is(err, project.ErrUnknownProject) {
				journeyError(w, 404, "project_unavailable", "This project does not exist in your workspace.")
			} else {
				journeyError(w, 409, "branch_busy", "The project source is switching branches. Check its operation before continuing.")
			}
			return
		}
		defer release()
		handler(w, r)
	})
}

func (s *Server) branchGuard(id string) *sync.RWMutex {
	s.branchGuardMu.Lock()
	defer s.branchGuardMu.Unlock()
	if s.branchGuards == nil {
		s.branchGuards = map[string]*sync.RWMutex{}
	}
	guard := s.branchGuards[id]
	if guard == nil {
		guard = &sync.RWMutex{}
		s.branchGuards[id] = guard
	}
	return guard
}

// AcquireGitSessionAdmission is shared by create, restart and branch checkout.
// The caller must hold admission until its starting record is visible, and
// re-resolve the branch after acquiring it. Failed admission never queues a CLI.
func (s *Server) AcquireGitSessionAdmission(ctx context.Context, projectID string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !branchProjectID.MatchString(projectID) {
		return nil, project.ErrUnknownProject
	}
	guard := s.branchGuard(projectID)
	if !guard.TryRLock() {
		return nil, errors.New("The project is switching branches. Wait for it to finish before starting a session.")
	}
	if _, err := s.resolveProject(projectID); err != nil {
		guard.RUnlock()
		if errors.Is(err, project.ErrUnknownProject) {
			return nil, project.ErrUnknownProject
		}
		return nil, errors.New("The session's project no longer resolves in your workspace.")
	}
	var once sync.Once
	return func() { once.Do(guard.RUnlock) }, nil
}

// ValidateSessionBranch prevents an old primary session being restarted on a
// different checkout. Existing linked sessions retain their separate branch.
func (s *Server) ValidateSessionBranch(ctx context.Context, projectID, worktreeID, expectedBranchID string) error {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return errors.New("The session's project no longer resolves in your workspace.")
	}
	primary, err := s.branchPrimaryWorktree(ctx, s.projectRoot(), p)
	if err != nil {
		return errors.New("The current project checkout could not be checked safely.")
	}
	if primary.ID != worktreeID && project.WorktreeID(p.Path) != worktreeID {
		return nil
	}
	raw, err := s.branchGit(ctx, s.projectRoot(), p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || project.BranchID(p.ID, strings.TrimSpace(raw)) != expectedBranchID {
		return errors.New("This session belongs to a different branch. Switch to its branch before restarting it.")
	}
	return nil
}

func (s *Server) branchPrimaryWorktree(ctx context.Context, root string, p project.Project) (project.Worktree, error) {
	out, err := s.branchGit(ctx, root, p.Path, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return project.Worktree{}, err
	}
	parts := strings.Split(out, "\x00")
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "worktree ") {
		return project.Worktree{}, errors.New("primary worktree unavailable")
	}
	path := filepath.Clean(strings.TrimPrefix(parts[0], "worktree "))
	if _, err := hostpath.ConfineToMounts([]string{root}, path); err != nil {
		return project.Worktree{}, err
	}
	a, err := os.Stat(p.Path)
	if err != nil {
		return project.Worktree{}, err
	}
	b, err := os.Stat(path)
	if err != nil || !os.SameFile(a, b) {
		return project.Worktree{}, errors.New("Git named a different primary checkout")
	}
	return project.Worktree{ID: project.WorktreeID(path), ProjectID: p.ID, Name: p.Name, Path: path, IsPrimary: true}, nil
}

// These operations never inherit host Git redirection, includes or credential
// helpers. Git itself is reused; no repository hook, filter, pager, fsmonitor,
// submodule command or network protocol runs on the controller.
func (s *Server) branchGit(ctx context.Context, root, dir string, args ...string) (string, error) {
	bounded, err := project.ManagedGitArguments(root, dir, args)
	if err != nil {
		return "", err
	}
	prefix := append([]string{}, journeyGitFlags...)
	prefix = append(prefix, "-c", "submodule.recurse=false", "-c", "protocol.allow=never", "-c", "credential.helper=")
	prefix = append(prefix, bounded[:len(bounded)-len(args)]...)
	runner := s.branchRunner
	if runner == nil {
		runner = func(ctx context.Context, exe string, argv []string, cwd string) (string, error) {
			return runCommandEnvironment(ctx, exe, argv, cwd, project.ManagedGitEnvironment(os.Environ()))
		}
	}
	if len(args) != 0 && args[0] != "config" {
		probe := append(append([]string{}, prefix...), "config", "--null", "--name-only", "--get-regexp", `^filter\.`)
		filters, probeErr := runner(ctx, "git", probe, dir)
		if probeErr != nil {
			var exit *exec.ExitError
			if ctx.Err() != nil || strings.TrimSpace(filters) != "" || !errors.As(probeErr, &exit) || exit.ExitCode() != 1 {
				return "", probeErr
			}
		}
		seen := map[string]bool{}
		for _, key := range strings.Split(filters, "\x00") {
			if key == "" {
				continue
			}
			end := strings.LastIndexByte(key, '.')
			if !strings.HasPrefix(key, "filter.") || end <= len("filter.") || strings.ContainsAny(key, "\r\n") {
				return "", errors.New("Git filter configuration cannot be confined safely")
			}
			driver := key[:end]
			if seen[driver] {
				continue
			}
			seen[driver] = true
			for _, setting := range []string{"clean=", "smudge=", "process=", "required=false"} {
				prefix = append(prefix, "-c", driver+"."+setting)
			}
		}
	}
	return runner(ctx, "git", append(prefix, args...), dir)
}

func validateBranchMetadata(ctx context.Context, root, dir string) error {
	if _, err := project.ManagedGitArguments(root, dir, []string{"status"}); err != nil {
		return err
	}
	// Git objects and refs may themselves have been replaced by a session.
	// Refuse links throughout the metadata store before Git reads or writes it.
	return filepath.WalkDir(filepath.Join(dir, ".git"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("Git metadata contains an unsafe path")
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return errors.New("Git metadata contains a special file")
			}
			return hostpath.OneName(path)
		}
		return nil
	})
}

func (s *Server) branchSnapshot(ctx context.Context, root string, p project.Project) (BranchCatalogue, error) {
	result := BranchCatalogue{ProjectID: p.ID, Branches: []BranchState{}}
	if err := validateBranchMetadata(ctx, root, p.Path); err != nil {
		return result, err
	}
	primary, err := s.branchPrimaryWorktree(ctx, root, p)
	if err != nil {
		return result, err
	}
	result.primary = primary
	branch, branchErr := s.branchGit(ctx, root, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr == nil {
		result.CurrentBranch = strings.TrimSpace(branch)
		result.CurrentBranchID = project.BranchID(p.ID, result.CurrentBranch)
	}
	head, headErr := s.branchGit(ctx, root, p.Path, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if headErr == nil && deliverySHA.MatchString(strings.TrimSpace(head)) {
		result.Commit = strings.TrimSpace(head)
	} else if ctx.Err() != nil {
		return result, ctx.Err()
	}
	status, err := s.branchGit(ctx, root, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return result, err
	}
	result.Clean = status == ""
	refs, err := s.branchGit(ctx, root, p.Path, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(worktreepath)", "refs/heads")
	if err != nil {
		return result, err
	}
	if len(refs) >= 2<<20 {
		return result, errors.New("branch catalogue is too large")
	}
	for _, line := range strings.Split(refs, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(strings.TrimRight(line, "\r"), "\x00")
		if len(parts) != 3 || !strings.HasPrefix(parts[0], "refs/heads/") || !deliverySHA.MatchString(parts[1]) {
			return result, errors.New("invalid branch catalogue")
		}
		name := strings.TrimPrefix(parts[0], "refs/heads/")
		result.Branches = append(result.Branches, BranchState{Branch: project.Branch{ID: project.BranchID(p.ID, name), ProjectID: p.ID, Name: name, IsHead: name == result.CurrentBranch}, Commit: parts[1], CheckedOut: parts[2] != ""})
	}
	if result.Commit == "" && result.CurrentBranch != "" {
		result.Branches = append(result.Branches, BranchState{Branch: project.Branch{ID: result.CurrentBranchID, ProjectID: p.ID, Name: result.CurrentBranch, IsHead: true}, CheckedOut: true})
	}
	result.Message = "Create a branch or switch to continue work from its committed files."
	s.mu.RLock()
	busySource := s.deliveryBusy[p.ID]
	for _, job := range s.projectChecks {
		if job.run.ProjectID == p.ID && job.cancel != nil {
			busySource = true
			break
		}
	}
	s.mu.RUnlock()
	switch {
	case result.Commit == "":
		result.Message = "Create the first commit in Git before creating or switching branches."
	case result.CurrentBranch == "":
		result.Message = "HEAD is detached. Check out a named local branch in your CLI before using branch controls."
	case !result.Clean:
		result.Message = "Commit your source and project context changes before switching branches."
	case busySource:
		result.Message = "Wait for this project's checks or delivery operation to finish before switching branches."
	case s.branchSessions == nil:
		result.Message = "Session state is unavailable. Branch changes are disabled until the workspace can check it."
	default:
		active, err := s.branchSessions(ctx, p.ID, primary.ID)
		if err != nil {
			result.Message = "Current session state could not be checked. Refresh before changing branches."
		} else if active {
			result.Message = "Stop this project's active sessions before creating or switching branches."
		} else {
			result.CanCreate, result.CanSwitch = true, true
		}
	}
	for i := range result.Branches {
		b := &result.Branches[i]
		b.CanSwitch = result.CanSwitch && !b.CheckedOut
		b.Message = result.Message
		if b.IsHead {
			b.Message = "This is the current branch."
		} else if b.CheckedOut {
			b.Message = "This branch is already checked out in another worktree."
		}
	}
	return result, nil
}

func (s *Server) branchProject(w http.ResponseWriter, r *http.Request) (project.Project, bool) {
	p, err := s.resolveProject(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, project.ErrUnknownProject) || errors.Is(err, os.ErrNotExist) {
			journeyError(w, 404, "project_unavailable", "This project does not exist in your workspace.")
		} else {
			journeyError(w, 403, "project_unavailable", "This project's source could not be resolved safely. Refresh after its metadata stops changing.")
		}
		return project.Project{}, false
	}
	return p, true
}

func (s *Server) getProjectBranches(w http.ResponseWriter, r *http.Request) {
	p, ok := s.branchProject(w, r)
	if !ok {
		return
	}
	guard := s.branchGuard(p.ID)
	if !guard.TryRLock() {
		journeyError(w, 409, "branch_busy", "A branch change is in progress. Check its operation before continuing.")
		return
	}
	defer guard.RUnlock()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result, err := s.branchSnapshot(ctx, s.projectRoot(), p)
	if err != nil {
		journeyError(w, 403, "unsafe_git", "The repository could not be safely inspected. Resolve its Git metadata or refresh after it stops changing.")
		return
	}
	if receipt, err := s.latestBranchReceipt(p.ID); err == nil && receipt != nil {
		observed := s.observeBranchReceipt(ctx, s.projectRoot(), p, *receipt)
		result.Operation = &observed.Operation
	} else if err != nil {
		journeyError(w, 500, "branch_history_unavailable", "The previous branch operation could not be read safely.")
		return
	}
	journeyJSON(w, 200, result)
}

func (s *Server) branchOperationPath(projectID, id string, create bool) (string, error) {
	if !branchOperationID.MatchString(id) {
		return "", errors.New("invalid operation ID")
	}
	parent := s.stateDir
	for _, part := range []string{"branch-operations", projectID} {
		path, err := guardedProjectPath(parent, part, create)
		if err != nil {
			return "", err
		}
		if create {
			if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", errors.New("branch operation directory is unavailable")
		}
		parent = path
	}
	return guardedProjectPath(parent, id+".json", create)
}

func (s *Server) readBranchReceipt(projectID, id string) (branchReceipt, error) {
	var receipt branchReceipt
	path, err := s.branchOperationPath(projectID, id, false)
	if err != nil {
		return receipt, err
	}
	if err = hostpath.OneName(path); err != nil {
		return receipt, err
	}
	f, err := os.Open(path)
	if err != nil {
		return receipt, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 32<<10+1))
	if err != nil || len(data) > 32<<10 || json.Unmarshal(data, &receipt) != nil {
		return receipt, errors.New("invalid branch receipt")
	}
	if receipt.Request.OperationID != id || receipt.Result.Operation.ID != id || receipt.Result.Operation.ProjectID != projectID {
		return receipt, errors.New("foreign branch receipt")
	}
	return receipt, nil
}

func (s *Server) latestBranchReceipt(projectID string) (*branchReceipt, error) {
	path, err := s.branchOperationPath(projectID, "last-operation", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := hostpath.OneName(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || len(data) > 1024 {
		return nil, errors.New("invalid latest branch receipt")
	}
	var pointer struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &pointer) != nil {
		return nil, errors.New("invalid latest branch receipt")
	}
	receipt, err := s.readBranchReceipt(projectID, pointer.ID)
	return &receipt, err
}

func (s *Server) saveBranchReceipt(receipt branchReceipt, latest bool) error {
	op := receipt.Result.Operation
	path, err := s.branchOperationPath(op.ProjectID, op.ID, true)
	if err != nil {
		return err
	}
	if err := atomicJSON(path, receipt); err != nil {
		return err
	}
	if latest {
		path, err = s.branchOperationPath(op.ProjectID, "last-operation", true)
		if err != nil {
			return err
		}
		return atomicJSON(path, map[string]string{"id": op.ID})
	}
	return nil
}

func branchSuccess(p project.Project, primary project.Worktree, operation BranchOperation) BranchResult {
	branch := project.Branch{ID: operation.BranchID, ProjectID: p.ID, Name: operation.Branch, IsHead: true}
	primary.BranchID = branch.ID
	worktree := BranchWorktree{Worktree: primary}
	return BranchResult{Operation: operation, Branch: &branch, Worktree: &worktree, Commit: operation.TargetCommit}
}

// A GET can prove an interrupted checkout completed, but never updates its
// journal, re-executes Git, starts a session or changes a checkout.
func (s *Server) observeBranchReceipt(ctx context.Context, root string, p project.Project, receipt branchReceipt) BranchResult {
	result := receipt.Result
	if result.Operation.Status != "running" && result.Operation.Status != "uncertain" {
		return result
	}
	catalogue, err := s.branchSnapshot(ctx, root, p)
	if err == nil && catalogue.Clean && catalogue.CurrentBranchID == result.Operation.BranchID && catalogue.Commit == result.Operation.TargetCommit {
		op := result.Operation
		op.Status, op.Commit = "succeeded", catalogue.Commit
		op.Message = "Git confirms that the requested branch and commit are checked out."
		return branchSuccess(p, catalogue.primary, op)
	}
	result.Operation.Status = "uncertain"
	result.Operation.Message = "The requested checkout has not been confirmed. Refresh Git before choosing another action; existing branches and files were retained."
	return result
}

func (s *Server) getBranchOperation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.branchProject(w, r)
	if !ok {
		return
	}
	id := r.PathValue("operationId")
	if !branchOperationID.MatchString(id) {
		journeyError(w, 400, "invalid_operation", "Choose a valid branch operation ID.")
		return
	}
	receipt, err := s.readBranchReceipt(p.ID, id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			journeyError(w, 404, "operation_missing", "This branch operation does not exist in this project.")
		} else {
			journeyError(w, 500, "branch_history_unavailable", "This branch operation could not be read safely.")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	guard := s.branchGuard(p.ID)
	if !guard.TryRLock() {
		journeyJSON(w, 200, receipt.Result)
		return
	}
	defer guard.RUnlock()
	journeyJSON(w, 200, s.observeBranchReceipt(ctx, s.projectRoot(), p, receipt))
}

func (s *Server) createProjectBranch(w http.ResponseWriter, r *http.Request) {
	s.mutateProjectBranch(w, r, "create")
}
func (s *Server) switchProjectBranch(w http.ResponseWriter, r *http.Request) {
	s.mutateProjectBranch(w, r, "switch")
}

func (s *Server) mutateProjectBranch(w http.ResponseWriter, r *http.Request, action string) {
	var req branchMutation
	if !journeyDecode(w, r, &req) {
		return
	}
	if !branchOperationID.MatchString(req.OperationID) || req.OperationID == "last-operation" || !deliverySHA.MatchString(req.ExpectedCommit) || req.ExpectedBranchID == "" {
		journeyError(w, 400, "invalid_branch_request", "Choose an inspected branch and full source commit, with a fresh operation ID.")
		return
	}
	if action == "create" {
		if !deliveryBranch.MatchString(req.Name) || req.BranchID != "" || req.ExpectedTargetCommit != "" {
			journeyError(w, 400, "invalid_branch", "Use a branch name of 1–200 letters, numbers, dots, slashes, hyphens or underscores, starting with a letter or number.")
			return
		}
	} else if req.Name != "" || req.BranchID == "" || !deliverySHA.MatchString(req.ExpectedTargetCommit) {
		journeyError(w, 400, "invalid_branch_request", "Choose the inspected target branch and its full commit.")
		return
	}
	p, ok := s.branchProject(w, r)
	if !ok {
		return
	}
	guard := s.branchGuard(p.ID)
	if !guard.TryLock() {
		journeyError(w, 409, "branch_busy", "A session is starting or a branch operation is in progress. Wait for it to finish before switching.")
		return
	}
	defer guard.Unlock()
	root := s.projectRoot()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), branchExecutionBudget)
	defer cancel()
	if receipt, err := s.readBranchReceipt(p.ID, req.OperationID); err == nil {
		if receipt.Request != req || receipt.Result.Operation.Action != action {
			journeyError(w, 409, "operation_conflict", "This operation ID already describes a different branch action.")
		} else {
			journeyJSON(w, 200, s.observeBranchReceipt(ctx, root, p, receipt))
		}
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		journeyError(w, 500, "branch_history_unavailable", "The branch operation history could not be read safely.")
		return
	}
	// The per-project admission barrier covers journey, files, Git, releases
	// and Functions source. Other projects and setup remain available.
	snapshot, err := s.branchSnapshot(ctx, root, p)
	if err != nil {
		journeyError(w, 403, "unsafe_git", "The repository could not be safely inspected. Resolve its Git metadata before switching branches.")
		return
	}
	if snapshot.Commit == "" {
		journeyError(w, 409, "initial_commit_required", snapshot.Message)
		return
	}
	if snapshot.CurrentBranchID != req.ExpectedBranchID || snapshot.Commit != req.ExpectedCommit {
		journeyError(w, 409, "source_changed", "The current branch or commit changed. Refresh Git and review the source before switching.")
		return
	}
	if !snapshot.Clean {
		journeyError(w, 409, "commit_required", snapshot.Message)
		return
	}
	if !snapshot.CanSwitch {
		journeyError(w, 409, "sessions_active", snapshot.Message)
		return
	}
	name, target := req.Name, req.ExpectedCommit
	if action == "create" {
		if _, err = s.branchGit(ctx, root, p.Path, "check-ref-format", "--branch", req.Name); err != nil {
			journeyError(w, 400, "invalid_branch", "This name is not a valid local Git branch. Choose a different name.")
			return
		}
		for _, b := range snapshot.Branches {
			if b.Name == name {
				journeyError(w, 409, "branch_exists", "A branch with this name already exists. Select it to switch instead.")
				return
			}
		}
	} else {
		var found bool
		for _, b := range snapshot.Branches {
			if b.ID != req.BranchID {
				continue
			}
			found, name, target = true, b.Name, b.Commit
			if b.CheckedOut {
				journeyError(w, 409, "branch_checked_out", b.Message)
				return
			}
		}
		if !found {
			journeyError(w, 404, "source_missing", "The target branch no longer exists in this project.")
			return
		}
		if target != req.ExpectedTargetCommit {
			journeyError(w, 409, "source_changed", "The target branch commit changed. Refresh Git before switching.")
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	receipt := branchReceipt{Request: req, Result: BranchResult{Operation: BranchOperation{ID: req.OperationID, ProjectID: p.ID, Action: action, Status: "running", BranchID: project.BranchID(p.ID, name), Branch: name, SourceBranchID: snapshot.CurrentBranchID, SourceCommit: snapshot.Commit, TargetCommit: target, Message: "Checking out the selected committed files.", CreatedAt: now, UpdatedAt: now}}}
	if err := s.saveBranchReceipt(receipt, true); err != nil {
		journeyError(w, 500, "branch_record_failed", "The branch operation could not be recorded safely. No checkout was started.")
		return
	}
	// Readiness belonged to the old source. Clear it before checkout so an
	// interrupted result cannot carry it onto a different branch. GET recovery
	// stays read-only; a storage failure here never starts Git.
	if s.clearBranchReady != nil {
		if err := s.clearBranchReady(ctx, snapshot.primary.ID); err != nil {
			receipt.Result.Operation.Status = "failed"
			receipt.Result.Operation.Message = "The previous ready marker could not be cleared. No checkout started. Resolve workspace storage before trying again."
			receipt.Result.Operation.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := s.saveBranchReceipt(receipt, false); err != nil {
				journeyError(w, 500, "branch_record_failed", "No checkout started, but its failed readiness check could not be saved. Check the existing operation before continuing.")
				return
			}
			journeyJSON(w, 200, receipt.Result)
			return
		}
	}
	args := []string{"switch", "--no-guess", "--no-recurse-submodules", "--no-overwrite-ignore"}
	if action == "create" {
		args = append(args, "-c", name, target)
	} else {
		args = append(args, "--", name)
	}
	_, executeErr := s.branchGit(ctx, root, p.Path, args...)
	verify, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verifyCancel()
	observed := s.observeBranchReceipt(verify, root, p, receipt)
	observed.Operation.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if observed.Operation.Status != "succeeded" {
		observed.Operation.Status = "uncertain"
		observed.Operation.Message = "Git did not confirm the checkout. Check this operation and refresh Git before continuing; no branch cleanup or reset was attempted."
		if executeErr == nil {
			observed.Operation.Message = "Git returned, but its resulting branch and commit could not be confirmed. Check this operation before continuing."
		} else if after, err := s.branchSnapshot(verify, root, p); err == nil && branchSourceUnchanged(snapshot, after) {
			observed.Operation.Status = "failed"
			observed.Operation.Message = "Git refused the checkout. The original branch, commit and clean source are unchanged. Resolve any conflicting local files or Git locks, then try again with a new operation."
		}
	}
	receipt.Result = observed
	if err := s.saveBranchReceipt(receipt, false); err != nil {
		journeyError(w, 500, "branch_record_failed", "The checkout may have completed, but its result could not be saved. Check the existing operation before starting another.")
		return
	}
	journeyJSON(w, 200, observed)
}

func branchSourceUnchanged(before, after BranchCatalogue) bool {
	if !after.Clean || before.CurrentBranchID != after.CurrentBranchID || before.Commit != after.Commit || len(before.Branches) != len(after.Branches) {
		return false
	}
	for i, row := range before.Branches {
		other := after.Branches[i]
		if row.ID != other.ID || row.Commit != other.Commit || row.CheckedOut != other.CheckedOut {
			return false
		}
	}
	return true
}
