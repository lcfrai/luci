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
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	Status    string `json:"status"`
	ExitCode  *int   `json:"exitCode,omitempty"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}
type ProjectCheckRun struct {
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
				rows[i].Message = "The service restarted before this run finished. Rerun checks to obtain current evidence."
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
		rows = rows[:20]
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
		problem(w, 500, "checks_unavailable", err.Error())
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
		problem(w, 500, "checks_unavailable", err.Error())
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
	actor, policy, managed := management.ActorFromContext(r.Context())
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
	if req.Execution == "host" && (!managed || actor.Role != "super-admin" || !policy.AllowHostCommands) {
		problem(w, 403, "host_execution_disabled", "Host commands require a Super Admin and an enabled host-command policy.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	captureCtx, captureCancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer captureCancel()
	digest, commit, dirty, err := s.checkSource(captureCtx, p.Path)
	if err != nil {
		problem(w, 422, "source_unavailable", err.Error())
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
	row := ProjectCheckRun{ID: newID("check-"), ProjectID: p.ID, Commit: commit, Dirty: dirty, Execution: req.Execution, SourceDigest: digest, Status: "queued", Commands: []CheckCommandResult{}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: "Commands are queued. Task completion remains independent of check evidence."}
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
		row.Commands = append(row.Commands, CheckCommandResult{CheckCommand: c, Status: "queued", Output: ""})
	}
	if s.projectChecks == nil {
		s.projectChecks = map[string]*projectCheckJob{}
	}
	job := &projectCheckJob{run: snapshotProjectCheck(row), cancel: cancel}
	s.projectChecks[row.ID] = job
	if err = s.storeCheckRun(row); err != nil {
		delete(s.projectChecks, row.ID)
		cancel()
		s.mu.Unlock()
		problem(w, 500, "check_reservation_failed", err.Error())
		return
	}
	s.mu.Unlock()
	writeJSON(w, 202, row)
	go s.runProjectChecks(ctx, job, p.Path)
}
func (s *Server) updateProjectCheck(job *projectCheckJob, fn func(*ProjectCheckRun)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&job.run)
	job.run.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.storeCheckRun(job.run); err != nil {
		job.run.Message = "Check evidence could not be saved. Rerun after resolving storage: " + err.Error()
		job.run.Status = "unknown"
	}
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
	s.updateProjectCheck(job, func(row *ProjectCheckRun) { row.Status = "running"; row.Message = "Running trusted project commands." })
	final := "passed"
	for i, c := range job.run.Commands {
		if ctx.Err() != nil {
			final = "cancelled"
			break
		}
		s.updateProjectCheck(job, func(row *ProjectCheckRun) { row.Commands[i].Status = "running" })
		name, args := c.Executable, c.Args
		container := "luci-check-" + job.run.ID + "-" + strconv.Itoa(i)
		if job.run.Execution == "docker" {
			name = "docker"
			args = []string{"run", "--rm", "--init", "--name", container, "--label", "luci.check=true", "--label", "luci.check-id=" + job.run.ID, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--cpus", strconv.FormatFloat(float64(job.run.CPUMillis)/1000, 'f', 3, 64), "--memory", strconv.Itoa(job.run.MemoryMiB) + "m", "--pids-limit", "256", "--tmpfs", "/tmp:rw,nosuid,nodev,size=1g", "--env", "HOME=/tmp", "--env", "GOCACHE=/tmp/go-cache", "--env", "GOMODCACHE=/tmp/go-mod", "--env", "npm_config_cache=/tmp/npm-cache", "--mount", "type=bind,src=" + dir + ",dst=/workspace", "--workdir", "/workspace"}
			uid, gid := os.Getuid(), os.Getgid()
			if uid < 0 {
				uid = 1001
			}
			if gid < 0 {
				gid = 1001
			}
			args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid), "--entrypoint", c.Executable, s.sandboxImage)
			args = append(args, c.Args...)
		}
		output, err := s.runner(ctx, name, args, dir)
		if job.run.Execution == "docker" {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = s.runner(cleanup, "docker", []string{"rm", "--force", container}, "")
			cancel()
		}
		code := 0
		status := "passed"
		if err != nil {
			status = "failed"
			code = -1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			}
			output += "\n" + err.Error()
			final = "failed"
		}
		if ctx.Err() != nil {
			status = "cancelled"
			final = "cancelled"
		}
		truncated := len(output) > maxCheckOutput
		if truncated {
			output = output[:maxCheckOutput]
		}
		s.updateProjectCheck(job, func(row *ProjectCheckRun) {
			row.Commands[i].Status = status
			row.Commands[i].ExitCode = &code
			row.Commands[i].Output = output
			row.Commands[i].Truncated = truncated
		})
		if final != "passed" {
			break
		}
	}
	sourceCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	after, afterCommit, _, sourceErr := s.checkSource(sourceCtx, dir)
	cancel()
	s.updateProjectCheck(job, func(row *ProjectCheckRun) {
		row.Status = final
		row.SourceChanged = sourceErr != nil || after != row.SourceDigest || afterCommit != row.Commit
		for i := range row.Commands {
			if row.Commands[i].Status == "queued" {
				row.Commands[i].Status = "skipped"
			}
		}
		row.Message = "Commands finished with recorded exit codes. This does not automatically verify the brief's completion criteria."
		if row.SourceChanged {
			row.Message += " Project source changed during this run; rerun against the final source."
			if final == "passed" {
				row.Status = "unknown"
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			row.Message = "The ten-minute run limit was reached."
		}
	})
}
func (s *Server) cancelProjectCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
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
