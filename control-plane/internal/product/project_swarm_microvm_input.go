package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// ProjectSwarmMicroVMInput is an immutable, controller-selected task input.
// It carries no execution authority, provider credentials or review approval.
type ProjectSwarmMicroVMInput struct {
	Request  e2bbe.Request
	Manifest e2bbe.TransferManifest
	Archive  []byte
}

// BuildProjectSwarmMicroVMInput captures the exact reserved Git base and
// reviewed task context before a VM is allocated. Private Luci exports are
// sent as read-only context, not copied into the writable source tree. A
// tracked credential, unsafe path or unsupported Git object fails closed;
// silently omitting a source file would make returned diffs unreviewable.
func (s *Server) BuildProjectSwarmMicroVMInput(ctx context.Context, projectID, runID, operationID, ownerID string, maxSeconds int, estimatedCostMicros int64) (ProjectSwarmMicroVMInput, error) {
	var result ProjectSwarmMicroVMInput
	if maxSeconds < 1 || maxSeconds > 86400 || estimatedCostMicros < 1 || estimatedCostMicros > 1<<50 {
		return result, ErrProjectRunInvalid
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return result, err
	}
	guard := s.branchGuard(projectID)
	if !guard.TryRLock() {
		return result, errors.New("Project source is busy; retry task input capture after the current source operation.")
	}
	defer guard.RUnlock()
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return result, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.ChildID == "" || op.Implementation == nil || op.Allocation != "ready" || op.Status != "reserved" ||
		run.State == "cancelled" || swarmTask(&run, op.TaskID) == nil {
		return result, ErrProjectRunConflict
	}
	path, err := s.swarmPath(projectID, runID, operationID)
	if err != nil || project.WorktreeID(path) != op.WorktreeID {
		return result, ErrProjectRunInvalid
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return result, err
	}
	before, err := s.captureGitReview(ctx, path, "")
	base := swarmStartingCommit(*op)
	if err != nil || before.Status != "" || before.Head != base {
		return result, errors.New("Task source changed before microVM input capture.")
	}
	capture, err := s.swarmTaskCapture(ctx, run, *op, path)
	if err != nil || capture == nil || capture.Binding == nil || capture.Binding.SourceCommit != base {
		return result, errors.New("The reviewed task context no longer matches its reserved source.")
	}
	tree, err := s.checkpointTree(ctx, path, base)
	if err != nil {
		return result, err
	}
	// Fail before reading any blob when the complete required bundle cannot
	// fit. A partial archive is never a valid cloud task input.
	var total int64
	count := len(capture.Files)
	for name, entry := range tree {
		if name == "LUCI.md" || journeyContextFile(name) {
			continue
		}
		if !safeSwarmWorkspacePath(name) || entry.Size > e2bbe.MaxTransferBytes {
			return result, errors.New("Tracked source contains an unsafe, protected or oversized file; review it before cloud execution.")
		}
		total += entry.Size
		count++
	}
	for _, file := range capture.Files {
		total += int64(len(file.Data))
	}
	if count == 0 || count > e2bbe.MaxTransferFiles || total > e2bbe.MaxTransferBytes {
		return result, errors.New("The complete reviewed source and context exceed the bounded microVM transfer.")
	}
	payloads := make([]e2bbe.Payload, 0, len(tree)+len(capture.Files))
	for name, entry := range tree {
		// Canonical generated exports travel in context/ instead. The original
		// checked-in instructions remain source and cannot be overwritten.
		if name == "LUCI.md" || journeyContextFile(name) {
			continue
		}
		body, readErr := s.swarmGit(ctx, path, "show", base+":"+name)
		if readErr != nil || int64(len(body)) != entry.Size {
			return result, errors.New("A tracked source blob changed or could not be read for cloud execution.")
		}
		mode := uint32(0644)
		if entry.Mode == "100755" {
			mode = 0755
		}
		payloads = append(payloads, microVMInputPayload("source/"+name, []byte(body), mode))
	}
	for _, file := range capture.Files {
		payloads = append(payloads, microVMInputPayload("context/"+file.Path, file.Data, 0644))
	}
	sort.Slice(payloads, func(i, j int) bool { return payloads[i].Path < payloads[j].Path })
	identity := e2bbe.Identity{OwnerID: ownerID, ProjectID: projectID, RunID: runID, TaskID: op.TaskID, AttemptID: op.ID, OperationID: op.ChildID}
	binding := e2bbe.TransferBinding{Identity: identity, SourceDigest: implementationDigest(base), ContextDigest: implementationDigest(op.Implementation), Direction: "input"}
	var archive bytes.Buffer
	manifest, bundleDigest, err := e2bbe.WriteTransfer(&archive, binding, payloads)
	if err != nil {
		return result, err
	}
	// Detect source/context or reservation drift before making these bytes
	// available to a provider. A later dispatcher must still recheck authority.
	after, err := s.captureGitReview(ctx, path, "")
	current, readErr := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil || readErr != nil || after.Digest != before.Digest || after.Head != base || current.Revision != run.Revision {
		return result, ErrProjectRunConflict
	}
	result.Request = e2bbe.Request{Identity: identity, SourceDigest: binding.SourceDigest, ContextDigest: binding.ContextDigest, BundleDigest: bundleDigest, MaxSeconds: maxSeconds, EstimatedCostMicros: estimatedCostMicros}
	if _, err = e2bbe.VerifyTransfer(bytes.NewReader(archive.Bytes()), binding, bundleDigest); err != nil {
		return ProjectSwarmMicroVMInput{}, err
	}
	result.Manifest, result.Archive = manifest, archive.Bytes()
	return result, nil
}

func microVMInputPayload(name string, body []byte, mode uint32) e2bbe.Payload {
	copied := append([]byte(nil), body...)
	hash := sha256.Sum256(copied)
	return e2bbe.Payload{TransferFile: e2bbe.TransferFile{Path: name, Size: int64(len(copied)), SHA256: hex.EncodeToString(hash[:]), Mode: mode}, Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(copied)), nil }}
}
