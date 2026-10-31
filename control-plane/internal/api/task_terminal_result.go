package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"crypto/sha256"
	"encoding/hex"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const taskTerminalDirectory = ".task-terminal"
const taskTerminalContainerDirectory = "/home/dev/.luci-task-terminal"
const taskTerminalWrapper = "/usr/local/lib/luci/codex-task-result.cjs"
const taskTerminalSchemaDigest = backend.CodexTaskResultSchemaDigest

// Controller-owned correlation, not worker-authored identity or an attestation
// of correctness/EOF. Original backend retirement and source observation remain
// separate prerequisites in lockStoppedTaskOutput and ObserveTaskCompletion.
type taskTerminalContract struct {
	Version                int    `json:"version"`
	Harness                string `json:"harness,omitempty"`
	SessionID              string `json:"sessionId"`
	ProjectID              string `json:"projectId"`
	RunID                  string `json:"runId"`
	OperationID            string `json:"operationId"`
	TaskID                 string `json:"taskId"`
	WorktreeID             string `json:"worktreeId"`
	Attempt                int    `json:"attempt"`
	Role                   string `json:"role"`
	ReviewOfTaskID         string `json:"reviewOfTaskId"`
	ReviewOnly             bool   `json:"reviewOnly"`
	TaskInputDigest        string `json:"taskInputDigest"`
	ReviewAssignmentDigest string `json:"reviewAssignmentDigest"`
	RequestDigest          string `json:"requestDigest"`
	ImplementationDigest   string `json:"implementationDigest"`
	ContextDigest          string `json:"contextDigest"`
	SourceCommit           string `json:"sourceCommit"`
	SourceDigest           string `json:"sourceDigest"`
	SourceDirty            bool   `json:"sourceDirty"`
	NativeDelegation       string `json:"nativeDelegation"`
	Image                  string `json:"image"`
	CommandDigest          string `json:"commandDigest"`
	SchemaDigest           string `json:"schemaDigest"`
}

func terminalCommandDigest(command string, args []string) string {
	h := sha256.New()
	for _, value := range append([]string{command}, args...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func terminalContract(run product.ProjectRun, row registry.Session, descriptor *registry.ReviewedContext, command string, args []string, image string) (taskTerminalContract, error) {
	var c taskTerminalContract
	if !terminalHarness(row.Harness) || row.ImplementationContext == nil || descriptor == nil || !reviewedSessionID.MatchString(row.ID) || run.ProjectID != row.ProjectID || run.ID != row.ProjectRunID || run.Swarm == nil || image == "" {
		return c, errors.New("incomplete terminal task identity")
	}
	var commandErr error
	command, commandErr = terminalExecutable(row.Harness, command, args)
	if commandErr != nil {
		return c, commandErr
	}
	var op *product.ProjectRunOperation
	for i := range run.Operations {
		if run.Operations[i].ID == row.ProjectRunOperationID {
			op = &run.Operations[i]
		}
	}
	if op == nil || op.ChildID != row.ID || op.WorktreeID != row.WorktreeID || op.TaskID == "" || op.Attempt < 1 || op.RequestDigest == "" || op.Implementation == nil || taskOutputImplementationDigest(op.Implementation) != taskOutputImplementationDigest(row.ImplementationContext) {
		return c, errors.New("terminal task operation changed")
	}
	var task *product.ProjectSwarmTask
	for i := range run.Swarm.Tasks {
		if run.Swarm.Tasks[i].ID == op.TaskID {
			task = &run.Swarm.Tasks[i]
		}
	}
	if task == nil {
		return c, errors.New("terminal task role missing")
	}
	role := task.Role
	if role == "" && !task.ReviewOnly {
		role = "coding"
	}
	if role != "coding" && role != "reviewer" || task.ReviewOnly && role != "reviewer" || role == "reviewer" && task.ReviewOfTaskID == "" {
		return c, errors.New("invalid terminal task role")
	}
	profile := ""
	if command == "/usr/local/bin/node" {
		profile = "codex-native-helpers-2-v1"
	}
	b := row.ImplementationContext
	c = taskTerminalContract{Version: 1, SessionID: row.ID, ProjectID: row.ProjectID, RunID: run.ID, OperationID: op.ID, TaskID: op.TaskID, WorktreeID: op.WorktreeID, Attempt: op.Attempt, Role: role, ReviewOfTaskID: task.ReviewOfTaskID, ReviewOnly: task.ReviewOnly, TaskInputDigest: task.TaskInputDigest, ReviewAssignmentDigest: op.ReviewAssignmentDigest, RequestDigest: op.RequestDigest, ImplementationDigest: taskOutputImplementationDigest(b), ContextDigest: descriptor.Digest, SourceCommit: b.SourceCommit, SourceDigest: b.SourceDigest, SourceDirty: b.SourceDirty, NativeDelegation: profile, Image: image, CommandDigest: terminalCommandDigest(command, args), SchemaDigest: taskTerminalSchemaDigest}
	if row.Harness != "codex" {
		c.Version = 2
		c.Harness = row.Harness
	}
	return c, nil
}

func (a *API) taskTerminalPath(sessionID string) (string, error) {
	if !reviewedSessionID.MatchString(sessionID) {
		return "", errors.New("invalid terminal contract owner")
	}
	return filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, taskTerminalDirectory, sessionID), nil
}

// Fresh controller-owned Docker coding/reviewer tasks use this adapter. Direct
// interactive sessions remain unchanged. Missing capability never falls back.
func (a *API) prepareTaskTerminal(ctx context.Context, row registry.Session, descriptor *registry.ReviewedContext, command string, args []string, image string) (string, []string, backend.Mount, string, error) {
	pinnedImage, err := a.taskTerminalHarnessImage(ctx, image, row.Harness)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	run, err := a.deps.Product.GetProjectSwarmRun(row.ProjectID, row.ProjectRunID)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	c, err := terminalContract(run, row, descriptor, command, args, pinnedImage)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	command, err = terminalExecutable(row.Harness, command, args)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	data = append(data, '\n')
	dir, err := a.taskTerminalPath(row.ID)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	if _, err = a.assistDirectory(filepath.Join(sessionConversationOwner, taskTerminalDirectory)); err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	// Partial directories are retained as failed ownership, never adopted/replayed.
	if err = os.Mkdir(dir, 0700); err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	if err = codexPrivateDir(dir); err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	if err = reviewedWriteFile(filepath.Join(dir, "contract.json"), data); err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", nil, backend.Mount{}, "", err
	}
	observed, readErr := reviewedReadFile(root, dir, "contract.json", 16384)
	closeErr := root.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(data, observed) {
		return "", nil, backend.Mount{}, "", errors.New("terminal contract readback failed")
	}
	wrapped := append([]string{taskTerminalWrapper, reviewedHash(data), command}, args...)
	return "/usr/local/bin/node", wrapped, backend.Mount{HostPath: dir, ContainerPath: taskTerminalContainerDirectory, ReadOnly: true}, pinnedImage, nil
}

func terminalSession(row registry.Session) bool {
	return row.Command == "/usr/local/bin/node" && len(row.Args) > 0 && row.Args[0] == taskTerminalWrapper
}

// Called only while the existing stopped-original/output fence remains held.
// The worker receipt binds raw report bytes, not independent check truth, child
// EOF attestation, or permission to integrate/release/retry the task.
func (a *API) verifyTerminalReport(run product.ProjectRun, row registry.Session, root *os.Root, dir string, data []byte) error {
	if !terminalSession(row) {
		return nil
	}
	if row.TaskTerminalImage != "" {
		if !taskContainerNormalZero(row) {
			return errors.New("original task container normal-zero fact is unavailable")
		}
	} else if row.Status != registry.StatusExited || (!row.TaskTerminalExitZero || !row.TaskTerminalObserverStarted) {
		return errors.New("original terminal exit is not known zero")
	}
	if len(row.Args) < 4 || !reviewedDigest.MatchString(row.Args[1]) {
		return errors.New("invalid stored terminal command")
	}
	path, err := a.taskTerminalPath(row.ID)
	if err != nil {
		return err
	}
	if err = codexRegularPath(a.deps.ConversationsDir, path); err != nil {
		return err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), path); err != nil {
		return err
	}
	contractRoot, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	raw, readErr := reviewedReadFile(contractRoot, path, "contract.json", 16384)
	closeErr := contractRoot.Close()
	if readErr != nil || closeErr != nil || reviewedHash(raw) != row.Args[1] {
		return errors.New("terminal contract unavailable")
	}
	var c taskTerminalContract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("terminal contract invalid")
	}
	expected, err := terminalContract(run, row, row.ReviewedContext, row.Args[2], row.Args[3:], c.Image)
	if err != nil || c != expected {
		return errors.New("terminal contract binding changed")
	}
	receiptBytes, err := reviewedReadFile(root, dir, "terminal-receipt.json", 2048)
	if err != nil {
		return err
	}
	var receipt struct {
		Version        int    `json:"version"`
		ContractDigest string `json:"contractDigest"`
		ReportDigest   string `json:"reportDigest"`
	}
	decoder = json.NewDecoder(bytes.NewReader(receiptBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || receipt.Version != 1 || receipt.ContractDigest != row.Args[1] || receipt.ReportDigest != reviewedHash(data) {
		return errors.New("terminal report correlation failed")
	}
	return nil
}

// Installed-image advertisement is bounded/read-only and never executes a probe.
func (a *API) taskTerminalImage(ctx context.Context, image string) (string, error) {
	observer, ok := a.deps.Backend.(backend.CodexTaskResultImageObserver)
	if !ok {
		return "", errors.New("The installed sandbox image cannot advertise structured Codex results. Prepare and review a new runtime image before this task.")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	id, err := observer.ObserveCodexTaskResultImage(bounded, image)
	if err != nil || bounded.Err() != nil || len(id) != 71 || !strings.HasPrefix(id, "sha256:") || !reviewedDigest.MatchString(strings.TrimPrefix(id, "sha256:")) {
		return "", errors.New("The installed sandbox image does not advertise the exact structured Codex result schema. No task was started.")
	}
	return id, nil
}

func (a *API) validateTaskTerminalImage(ctx context.Context, row registry.Session, image string) error {
	if !terminalSession(row) {
		return nil
	}
	id, err := a.taskTerminalHarnessImage(ctx, image, row.Harness)
	if err != nil || id != image {
		return errors.New("The structured task image identity changed before allocation")
	}
	path, err := a.taskTerminalPath(row.ID)
	if err != nil {
		return err
	}
	if err = codexRegularPath(a.deps.ConversationsDir, path); err != nil {
		return err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	raw, readErr := reviewedReadFile(root, path, "contract.json", 16384)
	closeErr := root.Close()
	if readErr != nil || closeErr != nil || len(row.Args) < 2 || reviewedHash(raw) != row.Args[1] {
		return errors.New("The original terminal image contract is unavailable")
	}
	var contract taskTerminalContract
	if json.Unmarshal(raw, &contract) != nil || contract.Image != image || contract.SchemaDigest != taskTerminalSchemaDigest || (row.Harness == "codex" && (contract.Version != 1 || contract.Harness != "")) || (row.Harness != "codex" && (contract.Version != 2 || contract.Harness != row.Harness)) {
		return errors.New("The terminal image contract changed")
	}
	return nil
}

func taskTerminalSelectedImage(pinned, legacy string) string {
	if pinned != "" {
		return pinned
	}
	return legacy
}

func terminalHarness(value string) bool {
	return value == "codex" || value == "claude-code" || value == "opencode"
}
func terminalExecutable(harness, command string, args []string) (string, error) {
	executable := map[string]string{"codex": "codex", "claude-code": "claude", "opencode": "opencode"}[harness]
	if executable == "" {
		return "", errors.New("unsupported terminal harness")
	}
	if command == executable || command == "/usr/local/bin/"+executable {
		return "/usr/local/bin/" + executable, nil
	}
	if harness == "codex" && command == "/usr/local/bin/node" && len(args) > 0 && args[0] == "/usr/local/lib/luci/codex-native-task.cjs" {
		return command, nil
	}
	return "", errors.New("terminal command does not match the selected harness")
}
func (a *API) taskTerminalHarnessImage(ctx context.Context, image, harness string) (string, error) {
	if harness == "codex" {
		return a.taskTerminalImage(ctx, image)
	}
	observer, ok := a.deps.Backend.(backend.HarnessTaskResultImageObserver)
	if !ok || !terminalHarness(harness) {
		return "", errors.New("The installed sandbox image cannot advertise this harness result contract. Prepare the matching runtime image before starting this task.")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	id, err := observer.ObserveHarnessTaskResultImage(bounded, image, harness)
	if err != nil || bounded.Err() != nil || len(id) != 71 || !strings.HasPrefix(id, "sha256:") || !reviewedDigest.MatchString(strings.TrimPrefix(id, "sha256:")) {
		return "", errors.New("The installed sandbox image does not advertise the selected harness result contract. No task was started.")
	}
	return id, nil
}
