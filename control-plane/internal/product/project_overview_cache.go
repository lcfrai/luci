package product

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const projectOverviewCacheLimit = 64
const projectOverviewCacheTTL = time.Second

type overviewFileStamp struct {
	info  os.FileInfo
	state string
}
type projectOverviewCacheEntry struct {
	stamps [6]overviewFileStamp
	view   ProjectOverview
	loaded time.Time
	used   uint64
}

// One shared fill at a time bounds transient canonical context decoding too;
// repeated calls for a project share the same compact result. No model input,
// full dialogue, source tree, or run ledger is retained in this cache.
type projectOverviewCache struct {
	mu       sync.Mutex
	entries  map[string]projectOverviewCacheEntry
	sequence uint64
}

func overviewStamp(root, relative string) overviewFileStamp {
	path, err := guardedProjectPath(root, relative, false)
	if errors.Is(err, os.ErrNotExist) {
		return overviewFileStamp{state: "missing"}
	}
	if err != nil {
		return overviewFileStamp{state: "unavailable"}
	}
	if err = hostpath.OneName(path); err != nil {
		return overviewFileStamp{state: "unavailable"}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return overviewFileStamp{state: "missing"}
	}
	if err != nil || !info.Mode().IsRegular() {
		return overviewFileStamp{state: "unavailable"}
	}
	return overviewFileStamp{info: info, state: "present"}
}
func sameOverviewStamps(a, b [6]overviewFileStamp) bool {
	for i := range a {
		if a[i].state != b[i].state {
			return false
		}
		if a[i].state == "present" && (!os.SameFile(a[i].info, b[i].info) || a[i].info.Size() != b[i].info.Size() || a[i].info.Mode() != b[i].info.Mode() || !a[i].info.ModTime().Equal(b[i].info.ModTime())) {
			return false
		}
	}
	return true
}
func (s *Server) overviewStamps(p project.Project) [6]overviewFileStamp {
	return [6]overviewFileStamp{overviewStamp(p.Path, ".luci/state.json"), overviewStamp(s.stateDir, "review-approvals/"+p.ID+".json"), overviewStamp(s.stateDir, "orchestrator/"+p.ID+".json"), overviewStamp(s.stateDir, "project-overview-questions/"+p.ID+".json"), overviewStamp(s.stateDir, "project-overview-runs/"+p.ID+".json"), overviewRunDirectoryStamp(s.stateDir, "project-runs/"+p.ID)}
}
func copyProjectOverview(value ProjectOverview) ProjectOverview {
	value.Questions.Items = append([]ProjectOverviewQuestion{}, value.Questions.Items...)
	value.Limitations = append([]string{}, value.Limitations...)
	return value
}
func (s *Server) cachedProjectOverview(p project.Project) ProjectOverview {
	cache := &s.projectOverview
	cache.mu.Lock()
	defer cache.mu.Unlock()
	now := time.Now()
	key := p.ID + "\x00" + p.Path
	stamps := s.overviewStamps(p)
	cache.sequence++
	if entry, ok := cache.entries[key]; ok && now.Sub(entry.loaded) < projectOverviewCacheTTL && sameOverviewStamps(entry.stamps, stamps) {
		entry.used = cache.sequence
		cache.entries[key] = entry
		return copyProjectOverview(entry.view)
	}
	s.mu.RLock()
	view := s.readProjectOverview(p)
	after := s.overviewStamps(p)
	s.mu.RUnlock()
	if !sameOverviewStamps(stamps, after) {
		delete(cache.entries, key)
		view = unavailableProjectOverview(p.ID)
		view.Limitations = append(view.Limitations, "Project context changed while being summarized. Refresh the overview before relying on its stage.")
		return view
	}
	if cache.entries == nil {
		cache.entries = make(map[string]projectOverviewCacheEntry)
	}
	if _, ok := cache.entries[key]; !ok && len(cache.entries) >= projectOverviewCacheLimit {
		oldest := ""
		var sequence uint64
		for candidate, entry := range cache.entries {
			if oldest == "" || entry.used < sequence {
				oldest = candidate
				sequence = entry.used
			}
		}
		delete(cache.entries, oldest)
	}
	cache.entries[key] = projectOverviewCacheEntry{stamps: after, view: view, loaded: now, used: cache.sequence}
	return copyProjectOverview(view)
}

func overviewRunDirectoryStamp(root, relative string) overviewFileStamp {
	path, err := guardedProjectPath(root, relative, false)
	if errors.Is(err, os.ErrNotExist) {
		return overviewFileStamp{state: "missing"}
	}
	if err != nil {
		return overviewFileStamp{state: "unavailable"}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return overviewFileStamp{state: "unavailable"}
	}
	return overviewFileStamp{state: "present", info: info}
}
