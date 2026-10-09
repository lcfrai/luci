package product

import (
	"context"
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
	fillOnce sync.Once
	fill     chan struct{}
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
func (s *Server) overviewStamps(ctx context.Context, p project.Project) ([6]overviewFileStamp, error) {
	var stamps [6]overviewFileStamp
	paths := [][2]string{{p.Path, ".luci/state.json"}, {s.stateDir, "review-approvals/" + p.ID + ".json"}, {s.stateDir, "orchestrator/" + p.ID + ".json"}, {s.stateDir, "project-overview-questions/" + p.ID + ".json"}, {s.stateDir, "project-overview-runs/" + p.ID + ".json"}, {s.stateDir, "project-runs/" + p.ID}}
	for index, path := range paths {
		if err := ctx.Err(); err != nil {
			return stamps, err
		}
		if index == len(stamps)-1 {
			stamps[index] = overviewRunDirectoryStamp(path[0], path[1])
		} else {
			stamps[index] = overviewStamp(path[0], path[1])
		}
	}
	return stamps, ctx.Err()
}
func copyProjectOverview(value ProjectOverview) ProjectOverview {
	value.Questions.Items = append([]ProjectOverviewQuestion{}, value.Questions.Items...)
	value.Attention.Items = append([]ProjectRunAttention{}, value.Attention.Items...)
	value.Limitations = append([]string{}, value.Limitations...)
	return value
}
func (s *Server) cachedProjectOverview(ctx context.Context, p project.Project) (ProjectOverview, error) {
	cache := &s.projectOverview
	if err := ctx.Err(); err != nil {
		return ProjectOverview{}, err
	}
	// Keep one globally bounded fill, but let obsolete display requests leave
	// its wait without retaining their project's shared source admission.
	cache.fillOnce.Do(func() { cache.fill = make(chan struct{}, 1) })
	select {
	case <-ctx.Done():
		return ProjectOverview{}, ctx.Err()
	case cache.fill <- struct{}{}:
	}
	defer func() { <-cache.fill }()
	if err := ctx.Err(); err != nil {
		return ProjectOverview{}, err
	}
	// The fill gate serializes every production owner of this map mutex.
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ProjectOverview{}, err
	}
	now := time.Now()
	key := p.ID + "\x00" + p.Path
	stamps, err := s.overviewStamps(ctx, p)
	if err != nil {
		return ProjectOverview{}, err
	}
	cache.sequence++
	if entry, ok := cache.entries[key]; ok && now.Sub(entry.loaded) < projectOverviewCacheTTL && sameOverviewStamps(entry.stamps, stamps) {
		entry.used = cache.sequence
		cache.entries[key] = entry
		return copyProjectOverview(entry.view), nil
	}
	s.mu.RLock()
	view, err := s.readProjectOverview(ctx, p)
	if err != nil {
		s.mu.RUnlock()
		return ProjectOverview{}, err
	}
	after, err := s.overviewStamps(ctx, p)
	s.mu.RUnlock()
	if err != nil {
		return ProjectOverview{}, err
	}
	if !sameOverviewStamps(stamps, after) {
		delete(cache.entries, key)
		view = unavailableProjectOverview(p.ID)
		view.Limitations = append(view.Limitations, "Project context changed while being summarized. Refresh the overview before relying on its stage.")
		return view, nil
	}
	if err := ctx.Err(); err != nil {
		return ProjectOverview{}, err
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
	return copyProjectOverview(view), nil
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
