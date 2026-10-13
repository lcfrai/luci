package product

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Called under s.mu before reserving any accepted-event continuation. An
// applied team event can be executed by a different authorized person's Luci,
// but only one actor may own its affected run. The actor's own continuation
// journal remains the durable operation/unknown-outcome fence.
func (s *Server) acceptedEventClaimedLocked(p project.Project, eventIDs []string, originTurnID string) (bool, error) {
	root, err := guardedProjectPath(s.stateDir, "coordinator-continuations", false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dir, err := os.Open(root)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(4097)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if len(entries) > 4096 {
		return false, ErrOrchestratorCapacity
	}
	wanted := make(map[string]bool, len(eventIDs))
	for _, id := range eventIDs {
		wanted[id] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), p.ID+"-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path, err := guardedProjectPath(root, entry.Name(), false)
		if err != nil {
			return false, err
		}
		var header coordinatorContinuations
		if err = externalReadJSON(path, coordinatorContinuationBytes, &header); err != nil {
			return false, err
		}
		if header.ProjectID != p.ID {
			return false, ErrOrchestratorInvalid
		}
		expected, err := s.continuationPath(p.ID, header.ActorID, false)
		if err != nil || path != expected {
			return false, ErrOrchestratorInvalid
		}
		rows, err := s.readContinuations(p, header.ActorID)
		if err != nil {
			return false, err
		}
		for _, row := range rows.Rows {
			if row.OriginTurnID == originTurnID {
				return true, nil
			}
			for _, receipt := range row.AcceptedEvents {
				if wanted[receipt.EventID] {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
