package product

import (
	"context"
	"errors"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

// verifyImportedMicroVMCandidate recovers a lost local Git-commit response by
// reading one exact child of the original task base. It accepts only the
// complete returned manifest, never a plausible commit message or partial
// changed-path subset. No Git write or second commit is made on reentry.
func (s *Server) verifyImportedMicroVMCandidate(ctx context.Context, path, base, head string, writePaths []string, changes e2bbe.SourceChanges) (string, error) {
	if !deliverySHA.MatchString(head) || head == base || len(changes.Changes) == 0 || len(changes.Changes) > swarmCheckpointPathLimit {
		return "", ErrProjectRunConflict
	}
	parents, err := s.swarmGit(ctx, path, "rev-list", "--parents", "-n", "1", head)
	parts := strings.Fields(parents)
	if err != nil || len(parts) != 2 || parts[0] != head || parts[1] != base {
		return "", errors.New("The task checkout is not one exact reviewed child of its original cloud input.")
	}
	before, err := s.checkpointTree(ctx, path, base)
	if err != nil {
		return "", err
	}
	after, err := s.checkpointTree(ctx, path, head)
	if err != nil {
		return "", err
	}
	wanted := map[string]e2bbe.SourceChange{}
	var total int64
	for _, change := range changes.Changes {
		name := change.Path
		if !safeSwarmWorkspacePath(name) || blockedProjectFile(name) || strings.EqualFold(name, "LUCI.md") ||
			!validProjectTaskWritePath(name) || len(writePaths) > 0 && !swarmPathAllowed(name, writePaths) ||
			wanted[name].Path != "" {
			return "", ErrProjectRunConflict
		}
		wanted[name] = change
		old, oldExists := before[name]
		newFile, newExists := after[name]
		if (change.Kind != "added" && change.Kind != "modified" && change.Kind != "deleted") ||
			oldExists != (change.Before != nil) || newExists != (change.After != nil) ||
			change.Kind == "added" && oldExists || change.Kind == "deleted" && newExists ||
			change.Kind == "modified" && (!oldExists || !newExists) {
			return "", ErrProjectRunConflict
		}
		for _, side := range []struct {
			file   *e2bbe.TransferFile
			entry  checkpointTreeFile
			commit string
		}{{change.Before, old, base}, {change.After, newFile, head}} {
			if side.file == nil {
				continue
			}
			if side.file.Path != name || side.file.Size > microVMImportFileLimit || side.entry.Size != side.file.Size ||
				side.entry.Mode != gitMode(side.file.Mode) {
				return "", ErrProjectRunConflict
			}
			body, readErr := s.swarmGit(ctx, path, "show", side.commit+":"+name)
			if readErr != nil || int64(len(body)) != side.file.Size || !sameMicroVMBytes(body, side.file.SHA256) {
				return "", ErrProjectRunConflict
			}
			total += int64(len(body))
			if total > microVMImportTotalLimit {
				return "", ErrProjectRunConflict
			}
		}
	}
	for name, old := range before {
		if wanted[name].Path == "" && after[name] != old {
			return "", errors.New("An undeclared task file changed after the cloud candidate was imported.")
		}
	}
	for name := range after {
		if _, existed := before[name]; !existed && wanted[name].Path == "" {
			return "", errors.New("The task checkout contains an undeclared added file.")
		}
	}
	return head, nil
}
