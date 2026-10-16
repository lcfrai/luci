//go:build windows

package dockerbe

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (g *assemblySourceGuard) originalRegistry() *assemblyOriginalRegistry { return g.registry }

// Pure finite path plan. No directory walk, file open, payload/hash or OS query.
// Exact same unique root/ancestor/leaf ordering as the actual collector.
func prepareAssemblyOriginalGuard(objects []backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	var slots []assemblyOriginalSlot
	seen := map[string]int{}
	add := func(path string, directory bool, bytes int64) error {
		volume := filepath.VolumeName(path)
		if len(path) > 32760 || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(volume) != 2 || volume[1] != ':' || strings.Contains(path[len(volume):], ":") || strings.ContainsAny(path, "\x00\r\n") {
			return backend.ErrUnsupported
		}
		key := strings.ToLower(path)
		if ordinal, ok := seen[key]; ok {
			old := slots[ordinal]
			if old.directory != directory || old.bytes != bytes {
				return backend.ErrUnsupported
			}
			return nil
		}
		if len(slots) >= assemblySourceHandleLimit {
			return backend.ErrUnsupported
		}
		seen[key] = len(slots)
		slots = append(slots, assemblyOriginalSlot{path: key, directory: directory, bytes: bytes, state: "planned"})
		return nil
	}
	for _, object := range objects {
		var parents []string
		for parent := filepath.Dir(object.Path); ; parent = filepath.Dir(parent) {
			parents = append(parents, parent)
			if parent == filepath.Dir(parent) {
				break
			}
			if len(parents) > 64 {
				return nil, backend.ErrUnsupported
			}
		}
		for i := len(parents) - 1; i >= 0; i-- {
			if err := add(parents[i], true, 0); err != nil {
				return nil, err
			}
		}
		if err := add(object.Path, false, object.Bytes); err != nil {
			return nil, err
		}
	}
	if len(slots) == 0 {
		return nil, backend.ErrUnsupported
	}
	return &assemblySourceGuard{registry: newAssemblyOriginalRegistry(true, slots)}, nil
}

func acquireAssemblyPreparedSources(ctx context.Context, g *assemblySourceGuard, objects []backend.DocumentImageAssemblyEvidenceObject) error {
	if g == nil || g.registry == nil {
		return backend.ErrLaunchFenceOwnership
	}
	if err := g.registry.start(); err != nil {
		g.registry.stopUnstarted()
		return err
	}
	defer g.registry.stop() // actual returning collector only, never timeout proxy
	collector := newAssemblySourceCollectorForGuard(ctx, g)
	return collectAssemblyEvidenceSources(ctx, collector, objects)
}
