package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

type taskHandoffAnswers struct {
	Version      int                 `json:"version"`
	ReportDigest string              `json:"reportDigest"`
	Answers      []TaskHandoffAnswer `json:"answers"`
	Actors       []string            `json:"actors"`
}

func (s *ProjectRunStore) handoffDirectory(projectID string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return "", ErrProjectRunInvalid
	}
	for _, relative := range []string{"task-handoffs", "task-handoffs/" + projectID} {
		dir, err := guardedProjectPath(s.stateDir, relative, create)
		if err != nil {
			return "", err
		}
		if create {
			if err = os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(dir)
			} else {
				err = os.Chmod(dir, 0700)
			}
			if err != nil {
				return "", err
			}
			if err = syncProjectRunDirectory(filepath.Dir(dir)); err != nil {
				return "", err
			}
		}
		if _, err = guardedProjectPath(s.stateDir, relative, false); err != nil {
			return "", err
		}
	}
	return guardedProjectPath(s.stateDir, "task-handoffs/"+projectID, false)
}

func (s *ProjectRunStore) writeHandoffObject(projectID string, value any, limit int) (string, error) {
	s.handoffMu.Lock()
	defer s.handoffMu.Unlock()
	data, err := json.Marshal(value)
	if err != nil || len(data) > limit {
		return "", errors.New("The captured task handoff exceeds its retained context limit.")
	}
	digest := implementationDigest(value)
	dir, err := s.handoffDirectory(projectID, true)
	if err != nil {
		return "", err
	}
	path, err := guardedProjectPath(dir, digest+".json", true)
	if err != nil {
		return "", err
	}
	if _, err = os.Lstat(path); err == nil {
		var existing json.RawMessage
		if err = s.readHandoffObject(projectID, digest, &existing, limit); err != nil {
			return "", err
		}
		var compact bytes.Buffer
		if json.Compact(&compact, existing) != nil || !bytes.Equal(compact.Bytes(), data) {
			return "", ErrProjectRunInvalid
		}
		return digest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Immutable objects include harmless interrupted-publication orphans. Bound
	// both their count and bytes instead of silently pruning retained evidence.
	directory, err := os.Open(dir)
	if err != nil {
		return "", err
	}
	entries, readErr := directory.ReadDir(4097)
	directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", readErr
	}
	if len(entries) >= 4096 {
		return "", errors.New("This project's retained handoff history has reached 4,096 objects. Export and retain its evidence before adding more.")
	}
	var retained int64
	for _, entry := range entries {
		info, e := entry.Info()
		if e != nil || !info.Mode().IsRegular() {
			return "", ErrProjectRunInvalid
		}
		retained += info.Size()
		if retained > 256<<20 {
			return "", errors.New("This project's retained handoff history exceeds its 256 MiB limit.")
		}
	}
	published, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	if retained+int64(len(published)+1) > 256<<20 {
		return "", errors.New("The new handoff would exceed this project's 256 MiB retained history limit.")
	}
	// Publication is atomic and durable; interruption leaves only an unreferenced
	// object or temporary file. The run pointer is committed afterward.
	if err = durableRunJSON(path, value); err != nil {
		return "", err
	}
	if err = syncProjectRunDirectory(dir); err != nil {
		return "", err
	}
	return digest, nil
}

func (s *ProjectRunStore) readHandoffObject(projectID, digest string, out any, limit int) error {
	if !hexDigest(digest) {
		return ErrProjectRunInvalid
	}
	dir, err := s.handoffDirectory(projectID, false)
	if err != nil {
		return err
	}
	path, err := guardedProjectPath(dir, digest+".json", false)
	if err != nil {
		return err
	}
	if err = hostpath.OneName(path); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	before, err := root.Lstat(digest + ".json")
	if err != nil || !before.Mode().IsRegular() || before.Size() > int64(limit*2) {
		return ErrProjectRunInvalid
	}
	f, err := root.Open(digest + ".json")
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return ErrProjectRunInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit*2+1)))
	if err != nil || len(data) > limit*2 {
		return ErrProjectRunInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF || implementationDigest(out) != digest {
		return ErrProjectRunInvalid
	}
	canonical, err := json.Marshal(out)
	if err != nil || len(canonical) > limit {
		return ErrProjectRunInvalid
	}
	return nil
}

func validTaskHandoffReference(op ProjectRunOperation) bool {
	if d := op.DependencyContext; d != nil && (!hexDigest(d.Digest) || d.Count < 0 || d.Count > 200) {
		return false
	}
	h := op.Handoff
	if h == nil {
		return true
	}
	if !hexDigest(h.Digest) || h.AnswersDigest != "" && !hexDigest(h.AnswersDigest) || h.Questions < 0 || h.Questions > 8 || h.Unanswered < 0 || h.Unanswered > h.Questions || !handoffText(h.Summary, 1000) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, h.CapturedAt); err != nil {
		return false
	}
	return h.Status == "captured" || h.Status == "missing" || h.Status == "invalid" || h.Status == "unavailable"
}

func taskHandoffIntegration(run ProjectRun, op ProjectRunOperation) *ProjectSwarmIntegration {
	if op.Integration != nil {
		return op.Integration
	}
	if task := swarmTask(&run, op.TaskID); task != nil && task.OperationID == op.ID {
		return task.Integration
	}
	return nil
}

func (s *ProjectRunStore) taskHandoffView(run ProjectRun, op ProjectRunOperation) (TaskHandoffView, error) {
	v := TaskHandoffView{ProjectID: run.ProjectID, RunID: run.ID, TaskID: op.TaskID, OperationID: op.ID, Status: "not-recorded", Answers: []TaskHandoffAnswer{}, QuestionIDs: []string{}, RecordedChecks: []ProjectRunCheckEvidence{}, Integration: taskHandoffIntegration(run, op)}
	if op.Handoff == nil {
		v.RecordedChecks = taskHandoffRecordedChecks(run, nil, v.Integration)
		return v, nil
	}
	var receipt TaskHandoffReceipt
	if err := s.readHandoffObject(run.ProjectID, op.Handoff.Digest, &receipt, TaskHandoffReportLimit+16384); err != nil {
		return v, err
	}
	if receipt.Version != 1 || receipt.ProjectID != run.ProjectID || receipt.RunID != run.ID || receipt.OperationID != op.ID || receipt.TaskID != op.TaskID || receipt.ChildID != op.ChildID || receipt.WorktreeID != op.WorktreeID || receipt.RequestDigest != op.RequestDigest || receipt.ImplementationDigest != implementationDigest(op.Implementation) || receipt.Status != op.Handoff.Status || receipt.CapturedAt != op.Handoff.CapturedAt {
		return v, ErrProjectRunInvalid
	}
	if !deliverySHA.MatchString(receipt.SourceCommit) || !hexDigest(receipt.SourceDigest) || receipt.ContextDigest != "" && !hexDigest(receipt.ContextDigest) {
		return v, ErrProjectRunInvalid
	}
	v.Status, v.Digest, v.Receipt = receipt.Status, op.Handoff.Digest, &receipt
	if receipt.Reported != nil {
		data, _ := json.Marshal(receipt.Reported)
		normalized, err := DecodeTaskHandoffReport(data)
		if err != nil || receipt.Status != "captured" {
			return v, ErrProjectRunInvalid
		}
		receipt.Reported = normalized
		for i := range receipt.Reported.Questions {
			v.QuestionIDs = append(v.QuestionIDs, reviewDigest(v.Digest+":"+strconv.Itoa(i)))
		}
	} else if receipt.Status == "captured" {
		return v, ErrProjectRunInvalid
	}
	if len(v.QuestionIDs) != op.Handoff.Questions {
		return v, ErrProjectRunInvalid
	}
	if op.Handoff.AnswersDigest != "" {
		var answers taskHandoffAnswers
		if err := s.readHandoffObject(run.ProjectID, op.Handoff.AnswersDigest, &answers, 64<<10); err != nil {
			return v, err
		}
		if answers.Version != 1 || answers.ReportDigest != v.Digest || len(answers.Answers) != len(answers.Actors) || len(answers.Answers) > len(v.QuestionIDs) {
			return v, ErrProjectRunInvalid
		}
		seen := map[string]bool{}
		allowed := map[string]bool{}
		for _, id := range v.QuestionIDs {
			allowed[id] = true
		}
		for i, answer := range answers.Answers {
			_, timeErr := time.Parse(time.RFC3339Nano, answer.AnsweredAt)
			if !allowed[answer.QuestionID] || seen[answer.QuestionID] || !handoffText(answer.Answer, 4000) || strings.TrimSpace(answer.Answer) == "" || timeErr != nil || answer.Attribution != "project-editor" || !handoffText(answers.Actors[i], 512) || strings.TrimSpace(answers.Actors[i]) == "" {
				return v, ErrProjectRunInvalid
			}
			seen[answer.QuestionID] = true
		}
		v.Answers = answers.Answers
	}
	if len(v.QuestionIDs)-len(v.Answers) != op.Handoff.Unanswered {
		return v, ErrProjectRunInvalid
	}
	v.RecordedChecks = taskHandoffRecordedChecks(run, &receipt, v.Integration)
	return v, nil
}

// Recorded verification is independent of a worker report. An integrated
// attempt can retain matching controller checks even when no report exists.
func taskHandoffRecordedChecks(run ProjectRun, receipt *TaskHandoffReceipt, integration *ProjectSwarmIntegration) []ProjectRunCheckEvidence {
	checks := []ProjectRunCheckEvidence{}
	integrated := integration != nil && integration.State == "integrated"
	if !integrated && (receipt == nil || receipt.SourceDirty) {
		return checks
	}
	commit := ""
	if integrated {
		commit = integration.Commit
	} else {
		commit = receipt.SourceCommit
	}
	for _, check := range run.Checks {
		if check.ProjectID == run.ProjectID && check.Commit == commit && reflect.DeepEqual(check.Approvals, run.Input.Approvals) && (integrated || check.SourceDigest == receipt.SourceDigest) {
			checks = append(checks, check)
		}
	}
	return checks
}

// Called inside the existing reservation CAS. These are immutable direct
// dependency inputs, not a later reconstruction from mutable task summaries.
func (s *ProjectRunStore) freezeTaskDependencies(run ProjectRun, op ProjectRunOperation) (*TaskDependencyReference, error) {
	task := swarmTask(&run, op.TaskID)
	if task == nil {
		return nil, ErrProjectRunInvalid
	}
	packet := TaskDependencyPacket{Version: 1, ProjectID: run.ProjectID, RunID: run.ID, TaskID: op.TaskID, OperationID: op.ID, Dependencies: []TaskHandoffView{}}
	if op.Checkpoint != nil {
		prior := swarmOperation(&run, op.Checkpoint.SourceOperationID)
		if prior == nil || prior.TaskID != op.TaskID || prior.ChildID != op.Checkpoint.SourceChildID {
			return nil, ErrProjectRunInvalid
		}
		view, err := s.taskHandoffView(run, *prior)
		if err != nil {
			return nil, err
		}
		packet.PreviousAttempt = &view
	}
	for _, id := range task.DependsOn {
		dep := swarmTask(&run, id)
		if dep == nil {
			if reused := reusedSwarmDependency(run, id); reused != nil {
				view, err := s.reusedDependencyHandoff(run, *reused)
				if err != nil {
					return nil, err
				}
				packet.Dependencies = append(packet.Dependencies, view)
				continue
			}
		}
		if dep == nil || dep.Integration == nil || dep.Integration.State != "integrated" {
			return nil, ErrProjectRunPending
		}
		prior := swarmOperation(&run, dep.OperationID)
		if prior == nil {
			return nil, ErrProjectRunInvalid
		}
		view, err := s.taskHandoffView(run, *prior)
		if err != nil {
			return nil, err
		}
		packet.Dependencies = append(packet.Dependencies, view)
	}
	delivered, err := json.MarshalIndent(packet, "", "  ")
	if err != nil || len(delivered)+1 > TaskDependencyPacketLimit {
		return nil, errors.New("The complete dependency context exceeds its 8 MiB limit. Split the dependency assignment before allocation.")
	}
	digest, err := s.writeHandoffObject(run.ProjectID, packet, TaskDependencyPacketLimit)
	if err != nil {
		return nil, err
	}
	return &TaskDependencyReference{Digest: digest, Count: len(packet.Dependencies)}, nil
}

// Caller first authorizes the project/orchestrator context. This bounded reader
// does not observe a backend, scan history, mutate state or follow client paths.
func (s *Server) TaskHandoffForOperation(projectID, runID, operationID string) (TaskHandoffView, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return TaskHandoffView{}, err
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return TaskHandoffView{}, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.TaskID == "" {
		return TaskHandoffView{}, ErrProjectRunNotFound
	}
	return s.projectRuns.taskHandoffView(run, *op)
}

func (s *ProjectRunStore) taskDependencyPacket(run ProjectRun, op ProjectRunOperation) (*TaskDependencyPacket, error) {
	if op.DependencyContext == nil {
		return nil, nil
	}
	var packet TaskDependencyPacket
	if err := s.readHandoffObject(run.ProjectID, op.DependencyContext.Digest, &packet, TaskDependencyPacketLimit); err != nil {
		return nil, err
	}
	if packet.Version != 1 || packet.ProjectID != run.ProjectID || packet.RunID != run.ID || packet.TaskID != op.TaskID || packet.OperationID != op.ID || len(packet.Dependencies) != op.DependencyContext.Count {
		return nil, ErrProjectRunInvalid
	}
	return &packet, nil
}
