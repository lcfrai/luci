package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type CheckCommand struct {
	Label      string   `json:"label"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}
type CheckCommandResult struct {
	CheckCommand
	AllocationID     string `json:"allocationId,omitempty"`
	Stage            string `json:"stage,omitempty"`
	ReasonCode       string `json:"reasonCode,omitempty"`
	Message          string `json:"message,omitempty"`
	CleanupConfirmed bool   `json:"cleanupConfirmed"`
	Status           string `json:"status"`
	ExitCode         *int   `json:"exitCode,omitempty"`
	Output           string `json:"output"`
	Truncated        bool   `json:"truncated"`
}
type ProjectCheckRun struct {
	Revision              int                          `json:"revision,omitempty"`
	ResourcesHeld         *bool                        `json:"resourcesHeld"`
	Stage                 string                       `json:"stage,omitempty"`
	ReasonCode            string                       `json:"reasonCode,omitempty"`
	ID                    string                       `json:"id"`
	ProjectID             string                       `json:"projectId"`
	Commit                string                       `json:"commit"`
	Dirty                 bool                         `json:"dirty"`
	Execution             string                       `json:"execution"`
	MemoryMiB             int                          `json:"memoryMiB"`
	CPUMillis             int                          `json:"cpuMillis"`
	SourceDigest          string                       `json:"sourceDigest"`
	SourceChanged         bool                         `json:"sourceChanged"`
	Status                string                       `json:"status"`
	Commands              []CheckCommandResult         `json:"commands"`
	CreatedAt             string                       `json:"createdAt"`
	UpdatedAt             string                       `json:"updatedAt"`
	Message               string                       `json:"message"`
	RequirementIDs        []string                     `json:"requirementIds,omitempty"`
	RequirementSnapshots  []RequirementSnapshot        `json:"requirementSnapshots,omitempty"`
	SpecificationRevision int                          `json:"specificationRevision,omitempty"`
	SpecificationDigest   string                       `json:"specificationDigest,omitempty"`
	DesignPlanRevision    int                          `json:"designPlanRevision,omitempty"`
	DesignPlanDigest      string                       `json:"designPlanDigest,omitempty"`
	EvidenceStatus        string                       `json:"evidenceStatus"`
	StaleReasons          []string                     `json:"staleReasons"`
	ApprovalSelection     *ProjectRunApprovalSelection `json:"approvalSelection,omitempty"`
}
type projectCheckJob struct {
	actor  management.ExecutionActorRef
	run    ProjectCheckRun
	cancel context.CancelFunc
}

var checkExecutable = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.+-]{0,79}$`)

const maxCheckOutput = 256 << 10

func validateCheckCommands(commands []CheckCommand) error {
	if len(commands) < 1 || len(commands) > 8 {
		return errors.New("Choose one to eight commands.")
	}
	for _, c := range commands {
		if strings.TrimSpace(c.Label) == "" || len(c.Label) > 200 || !checkExecutable.MatchString(c.Executable) || len(c.Args) > 100 {
			return errors.New("Use a short label, an executable name and at most 100 separate arguments.")
		}
		total := 0
		for _, a := range c.Args {
			total += len(a)
			if strings.ContainsRune(a, 0) {
				return errors.New("Arguments cannot contain NUL.")
			}
		}
		if total > 32768 {
			return errors.New("Command arguments exceed 32 KiB.")
		}
	}
	return nil
}
func (s *Server) checkHistoryPath(id string) string {
	return filepath.Join(s.stateDir, "project-checks", id+".json")
}
func (s *Server) readCheckHistory(id string) ([]ProjectCheckRun, error) {
	f, err := os.Open(s.checkHistoryPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return []ProjectCheckRun{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil || len(data) > 16<<20 {
		return nil, errors.New("Check history exceeds its limit.")
	}
	var rows []ProjectCheckRun
	if err = json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ProjectID != id {
			return nil, errors.New("Check history belongs to another project.")
		}
		if rows[i].Status == "running" || rows[i].Status == "queued" {
			if _, alive := s.projectChecks[rows[i].ID]; !alive {
				rows[i].Status = "unknown"
				rows[i].Message = "The service restarted before this run finished. Its recorded command output is retained; no command will be replayed. Inspect retained execution before approving another run."
			}
		}
	}
	if rows == nil {
		rows = []ProjectCheckRun{}
	}
	return rows, nil
}

// Caller holds s.mu. Retain twenty bounded runs outside agent-writable project context.
func (s *Server) storeCheckRun(row ProjectCheckRun) error {
	rows, err := s.readCheckHistory(row.ProjectID)
	if err != nil {
		return err
	}
	found := false
	for i := range rows {
		if rows[i].ID == row.ID {
			rows[i] = row
			found = true
			break
		}
	}
	if !found {
		rows = append([]ProjectCheckRun{row}, rows...)
	}
	if len(rows) > 20 {
		keep := make([]ProjectCheckRun, 0, 20)
		for _, candidate := range rows {
			if candidate.Status == "queued" || candidate.Status == "running" || candidate.Status == "unknown" || candidate.ResourcesHeld != nil && *candidate.ResourcesHeld {
				keep = append(keep, candidate)
			}
		}
		if len(keep) > 20 {
			return errors.New("Check history contains too many unresolved runs; review their retained execution before starting another check.")
		}
		for _, candidate := range rows {
			if len(keep) >= 20 {
				break
			}
			found := false
			for _, existing := range keep {
				if existing.ID == candidate.ID {
					found = true
					break
				}
			}
			if !found {
				keep = append(keep, candidate)
			}
		}
		retained := false
		for _, candidate := range keep {
			retained = retained || candidate.ID == row.ID
		}
		if !retained {
			return errors.New("Check history has no safe slot for this result; unresolved execution must remain retained.")
		}
		rows = keep
	}
	return atomicJSON(s.checkHistoryPath(row.ProjectID), rows)
}
func (s *Server) listProjectChecks(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	coverage, rows, err := s.readProjectEvidence(r.Context(), p.ID, p.Path)
	if err != nil {
		problem(w, 500, "checks_unavailable", "The saved check evidence could not be read safely.")
		return
	}
	suggested := []CheckCommand{}
	for _, x := range []struct {
		file string
		cmd  CheckCommand
	}{{"package.json", CheckCommand{"Package tests", "npm", []string{"test", "--", "--run"}}}, {"go.mod", CheckCommand{"Go tests", "go", []string{"test", "./..."}}}, {"pyproject.toml", CheckCommand{"Python tests", "python3", []string{"-m", "pytest"}}}} {
		if _, err := guardedProjectPath(p.Path, x.file, false); err == nil {
			suggested = append(suggested, x.cmd)
		}
	}
	writeJSON(w, 200, map[string]any{"runs": rows, "suggested": suggested, "coverage": coverage})
}
func (s *Server) getProjectCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	_, rows, err := s.readProjectEvidence(r.Context(), p.ID, p.Path)
	if err != nil {
		problem(w, 500, "checks_unavailable", "The saved check evidence could not be read safely.")
		return
	}
	for _, row := range rows {
		if row.ID == r.PathValue("runId") {
			writeJSON(w, 200, row)
			return
		}
	}
	problem(w, 404, "check_missing", "This check run is not part of this project.")
}
func (s *Server) startProjectChecks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Commands                      []CheckCommand `json:"commands"`
		Execution                     string         `json:"execution"`
		Acknowledge                   bool           `json:"acknowledgeHostExecution"`
		RequirementIDs                []string       `json:"requirementIds"`
		ExpectedSpecificationRevision *int           `json:"expectedSpecificationRevision"`
		ExpectedDesignPlanRevision    *int           `json:"expectedDesignPlanRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.Execution == "" {
		req.Execution = "docker"
	}
	if !req.Acknowledge {
		problem(w, 400, "execution_acknowledgement_required", "Review and trust these project commands before running them.")
		return
	}
	if err := validateCheckCommands(req.Commands); err != nil {
		problem(w, 400, "invalid_commands", err.Error())
		return
	}
	if err := validateRequirementIDs(req.RequirementIDs); err != nil {
		writeEvidenceBindingError(w, err)
		return
	}
	if req.Execution != "docker" && req.Execution != "host" {
		problem(w, 400, "invalid_execution", "Choose Docker execution.")
		return
	}
	_, policy, managed := management.ActorFromContext(r.Context())
	memory, cpu := 1024, 1000
	if managed {
		memory = policy.CheckMemoryMiB
		cpu = policy.CheckCPUMillis
	}
	if memory <= 0 {
		memory = 1024
	}
	if cpu <= 0 {
		cpu = 1000
	}
	if memory > s.checkMemoryMiB {
		memory = s.checkMemoryMiB
	}
	if cpu > s.checkCPUMillis {
		cpu = s.checkCPUMillis
	}
	if req.Execution == "host" {
		problem(w, 400, "host_check_resource_limits_unsupported", "Run these checks in Docker to enforce resource limits.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if s.projectCheckRuntime.CaptureActor == nil || s.projectCheckRuntime.Execute == nil {
		problem(w, 503, "check_execution_unavailable", "The shared check execution runtime is not ready. No command was started.")
		return
	}
	executionActor, actorErr := s.projectCheckRuntime.CaptureActor(r.Context(), p.ID)
	if actorErr != nil {
		problem(w, 403, "check_execution_forbidden", management.ErrExecutionAuthority.Error())
		return
	}
	captureCtx, captureCancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer captureCancel()
	digest, commit, dirty, err := s.checkSource(captureCtx, p.Path)
	if err != nil {
		problem(w, 422, "source_unavailable", "The project source could not be captured safely. No check command was started.")
		return
	}
	s.mu.Lock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		s.mu.Unlock()
		problem(w, 500, "history_unavailable", "The saved project definition could not be read.")
		return
	}
	binding, err := bindProjectRequirements(state, p.ID, req.RequirementIDs, req.ExpectedSpecificationRevision, req.ExpectedDesignPlanRevision, true)
	if err != nil {
		s.mu.Unlock()
		writeEvidenceBindingError(w, err)
		return
	}
	limit := 1
	if managed && policy.MaxConcurrentChecksPerUser > 0 {
		limit = policy.MaxConcurrentChecksPerUser
	}
	active := 0
	for _, job := range s.projectChecks {
		if job.cancel != nil {
			active++
			if job.run.ProjectID == p.ID {
				s.mu.Unlock()
				problem(w, 409, "checks_running", "This project already has a running check.")
				return
			}
		}
	}
	if active >= limit {
		s.mu.Unlock()
		problem(w, 409, "check_limit", "Wait for a running check to finish.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	held := false
	row := ProjectCheckRun{Revision: 1, ResourcesHeld: &held, ID: newID("check-"), ProjectID: p.ID, Commit: commit, Dirty: dirty, Execution: req.Execution, SourceDigest: digest, Status: "queued", Commands: []CheckCommandResult{}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: "Commands are queued. Task completion remains independent of check evidence."}
	row.RequirementIDs, row.RequirementSnapshots = binding.RequirementIDs, binding.RequirementSnapshots
	row.SpecificationRevision, row.SpecificationDigest = binding.SpecificationRevision, binding.SpecificationDigest
	row.DesignPlanRevision, row.DesignPlanDigest = binding.DesignPlanRevision, binding.DesignPlanDigest
	if len(binding.RequirementIDs) > 0 {
		selection, selectionErr := projectRunApprovalSelection(state)
		if selectionErr != nil {
			cancel()
			s.mu.Unlock()
			problem(w, 409, "review_approval_required", selectionErr.Error())
			return
		}
		row.ApprovalSelection = &selection
	}
	row.EvidenceStatus, row.StaleReasons = evidenceUnverified, []string{"The commands have not finished."}
	row.MemoryMiB = memory
	row.CPUMillis = cpu
	row.UpdatedAt = row.CreatedAt
	for _, c := range req.Commands {
		c.Args = append([]string{}, c.Args...)
		row.Commands = append(row.Commands, CheckCommandResult{CheckCommand: c, AllocationID: newID("checkexec-"), Status: "queued", Output: ""})
	}
	if s.projectChecks == nil {
		s.projectChecks = map[string]*projectCheckJob{}
	}
	job := &projectCheckJob{run: snapshotProjectCheck(row), cancel: cancel, actor: executionActor}
	s.projectChecks[row.ID] = job
	if err = s.storeCheckRun(row); err != nil {
		delete(s.projectChecks, row.ID)
		cancel()
		s.mu.Unlock()
		problem(w, 500, "check_reservation_failed", "The approved check could not be saved. No check command was started.")
		return
	}
	s.mu.Unlock()
	writeJSON(w, 202, row)
	go s.runProjectChecks(ctx, job, p.Path)
}
func (s *Server) updateProjectCheck(job *projectCheckJob, fn func(*ProjectCheckRun), final ...bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&job.run)
	job.run.Revision++
	job.run.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.storeCheckRun(job.run); err != nil {
		job.run.Message = "Check evidence could not be saved. Inspect the retained exact attempt after resolving private storage; no command will be replayed."
		job.run.Status = "unknown"
		return err
	}
	if len(final) > 0 && final[0] {
		// Final source comparison and result publication are complete before
		// a new check may begin. Remaining ledger ACK work cannot execute or
		// modify project files and may be retried by the background observer.
		if job.cancel != nil {
			job.cancel()
		}
		job.cancel = nil
		delete(s.projectChecks, job.run.ID)
	}
	return nil
}
func (s *Server) runProjectChecks(ctx context.Context, job *projectCheckJob, dir string) {
	defer func() {
		s.mu.Lock()
		if job.cancel != nil {
			job.cancel()
		}
		job.cancel = nil
		delete(s.projectChecks, job.run.ID)
		s.mu.Unlock()
	}()
	if err := s.updateProjectCheck(job, func(row *ProjectCheckRun) {
		row.Status = "running"
		row.Stage = "admission"
		row.Message = "Checking current authority and shared machine capacity."
	}); err != nil {
		return
	}
	final := "passed"
	for index, command := range job.run.Commands {
		if ctx.Err() != nil {
			final = "cancelled"
			break
		}
		i := index
		if err := s.updateProjectCheck(job, func(row *ProjectCheckRun) { row.Commands[i].Status = "running"; row.Commands[i].Stage = "admission" }); err != nil {
			return
		}
		req := ProjectCheckCommandRequest{ProjectID: job.run.ProjectID, CheckID: job.run.ID, CommandIndex: i, AllocationID: command.AllocationID, SourceDirectory: dir, SourceDigest: job.run.SourceDigest, Command: command.CheckCommand, MemoryMiB: job.run.MemoryMiB, CPUMillis: job.run.CPUMillis, Actor: job.actor}
		outcome, err := s.projectCheckRuntime.Execute(ctx, req, func(value ProjectCheckCommandOutcome) error { return s.recordProjectCheckCommand(job, i, value) })
		if err != nil {
			final = "unknown"
			s.updateProjectCheck(job, func(row *ProjectCheckRun) {
				row.Status = "unknown"
				row.Stage = "execution"
				row.ReasonCode = "check_execution_unconfirmed"
				row.Message = "The check execution or result publication is unconfirmed. Its exact attempt is retained for inspection and recovery."
			})
			break
		}
		if outcome.Status != "passed" || !outcome.CleanupConfirmed {
			final = outcome.Status
			if !outcome.CleanupConfirmed && outcome.Status != "blocked" {
				final = "unknown"
			}
			break
		}
	}
	sourceCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	after, afterCommit, _, sourceErr := s.checkSource(sourceCtx, dir)
	cancel()
	if err := s.updateProjectCheck(job, func(row *ProjectCheckRun) {
		row.Status = final
		row.SourceChanged = sourceErr != nil || after != row.SourceDigest || afterCommit != row.Commit
		for i := range row.Commands {
			if row.Commands[i].Status == "queued" {
				row.Commands[i].Status = "skipped"
			}
		}
		if final == "passed" || final == "failed" {
			row.Message = "Commands finished with recorded exit codes and confirmed cleanup. This does not automatically verify completion criteria."
		}
		if row.SourceChanged && final == "passed" {
			row.Status = "unknown"
			row.Message = "Project source changed during this run; rerun against the final source."
		}
	}, true); err != nil {
		return
	}
	if s.projectCheckRuntime.AcknowledgeCompleted != nil {
		ack, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.projectCheckRuntime.AcknowledgeCompleted(ack, job.run.ProjectID, job.run.ID)
		cancel()
	}
}
func (s *Server) cancelProjectCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if s.projectCheckRuntime.AuthorizeControl == nil || s.projectCheckRuntime.AuthorizeControl(r.Context(), p.ID) != nil {
		problem(w, 403, "check_control_forbidden", "Current project edit permission is required to stop this check.")
		return
	}
	s.mu.Lock()
	job := s.projectChecks[r.PathValue("runId")]
	if job == nil || job.run.ProjectID != p.ID {
		s.mu.Unlock()
		s.getProjectCheck(w, r)
		return
	}
	if job.cancel != nil {
		job.cancel()
	}
	row := snapshotProjectCheck(job.run)
	s.mu.Unlock()
	writeJSON(w, 202, row)
}

// Caller holds s.mu. HTTP encoding happens after releasing it, so mutable
// command results and their nested values must not remain shared with a worker.
func snapshotProjectCheck(row ProjectCheckRun) ProjectCheckRun {
	if row.ResourcesHeld != nil {
		v := *row.ResourcesHeld
		row.ResourcesHeld = &v
	}
	if row.ApprovalSelection != nil {
		selection := *row.ApprovalSelection
		row.ApprovalSelection = &selection
	}
	row.RequirementIDs = append([]string(nil), row.RequirementIDs...)
	row.RequirementSnapshots = append([]RequirementSnapshot(nil), row.RequirementSnapshots...)
	row.StaleReasons = append([]string{}, row.StaleReasons...)
	row.Commands = append([]CheckCommandResult(nil), row.Commands...)
	for i := range row.Commands {
		row.Commands[i].Args = append([]string(nil), row.Commands[i].Args...)
		if row.Commands[i].ExitCode != nil {
			code := *row.Commands[i].ExitCode
			row.Commands[i].ExitCode = &code
		}
	}
	return row
}
func (s *Server) closeProjectChecks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.projectChecks {
		if job.cancel != nil {
			job.cancel()
		}
	}
}

// Hash tracked and visible untracked source, excluding ignored dependencies and credentials.
func (s *Server) checkSource(ctx context.Context, dir string) (string, string, bool, error) {
	files, err := s.journeyGit(ctx, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", "", false, err
	}
	head, headErr := s.journeyGit(ctx, dir, "rev-parse", "--verify", "HEAD")
	if headErr != nil {
		// An unborn repository has no commit. Git's diagnostic is not source
		// provenance and must never be retained as if it were a commit hash.
		head = ""
	}
	status, err := s.journeyGit(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", "", false, err
	}
	h := sha256.New()
	paths := strings.Split(files, "\x00")
	sort.Strings(paths)
	seen := map[string]bool{}
	total := int64(0)
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if len(seen) > 10000 {
			return "", "", false, errors.New("Check source exceeds 10,000 files.")
		}
		if blockedProjectFile(path) && !strings.HasPrefix(path, ".luci/") {
			continue
		}
		full, e := guardedProjectPath(dir, path, false)
		if errors.Is(e, os.ErrNotExist) {
			fmt.Fprintf(h, "deleted:%s\x00", path)
			continue
		}
		if e != nil {
			return "", "", false, e
		}
		info, e := os.Lstat(full)
		if e != nil || !info.Mode().IsRegular() {
			return "", "", false, errors.New("Checks require ordinary source files; symbolic links and submodules are unsupported.")
		}
		total += info.Size()
		if total > 100<<20 {
			return "", "", false, errors.New("Check source exceeds 100 MiB.")
		}
		f, e := os.Open(full)
		if e != nil {
			return "", "", false, e
		}
		opened, e := f.Stat()
		if e != nil || !os.SameFile(info, opened) {
			f.Close()
			return "", "", false, errors.New("Source changed while opening; retry.")
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, info.Size())
		_, e = io.Copy(h, io.LimitReader(f, info.Size()+1))
		f.Close()
		if e != nil {
			return "", "", false, e
		}
	}
	return hex.EncodeToString(h.Sum(nil)), strings.TrimSpace(head), status != "", nil
}
