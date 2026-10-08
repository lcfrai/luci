package product

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Startup migration is a best-effort read-model upgrade, never execution
// reconciliation. At most 64 catalogue entries and 32 canonical files (32MiB
// maximum) are read. Larger histories remain explicitly unavailable until a
// normal run write rebuilds their index under the existing 200-run bound.
// No GET schedules or performs migration.
func (s *Server) migrateOverviewRunHeads() {
	root, err := os.Open(s.projectRoot())
	if err != nil {
		return
	}
	defer root.Close()
	entries, err := root.ReadDir(projectOverviewCacheLimit)
	if err != nil && !errors.Is(err, io.EOF) {
		return
	}
	store := s.projectRuns
	store.mu.Lock()
	defer store.mu.Unlock()
	remaining := 32
	for _, entry := range entries {
		if remaining == 0 {
			break
		}
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() {
			continue
		}
		path := filepath.Join(s.projectRoot(), entry.Name())
		id := project.ProjectID(path)
		scope, err := store.projectScope(id, path)
		if err != nil {
			continue
		}
		if head, err := store.readOverviewRunHead(id, scope); err == nil && head.Version == overviewRunHeadVersion {
			continue
		}
		directory, err := store.directory(id, false)
		if err != nil {
			continue
		}
		file, err := os.Open(directory)
		if err != nil {
			continue
		}
		before, statErr := file.Stat()
		names, readErr := file.Readdirnames(remaining + 1)
		file.Close()
		if statErr != nil || readErr != nil && !errors.Is(readErr, io.EOF) || len(names) > remaining {
			continue
		}
		remaining -= len(names)
		rows := make([]storedProjectRun, 0, len(names))
		valid := true
		for _, name := range names {
			if strings.HasPrefix(name, ".luci-") {
				continue
			}
			if !strings.HasSuffix(name, ".json") {
				valid = false
				break
			}
			row, err := store.read(id, scope, strings.TrimSuffix(name, ".json"))
			if err != nil {
				valid = false
				break
			}
			rows = append(rows, row)
		}
		after, err := os.Lstat(directory)
		if valid && err == nil && os.SameFile(before, after) && before.ModTime().Equal(after.ModTime()) {
			store.overviewHeadFromHistory(rows)
		}
	}
}
