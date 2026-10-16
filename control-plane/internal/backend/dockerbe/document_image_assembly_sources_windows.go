//go:build windows

package dockerbe

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblySourceHandle struct {
	handle syscall.Handle
	info   syscall.ByHandleFileInformation
	path   string
	slotID uint64 // stable through append/Close compaction, never a slice reference
}

type assemblySourceGuard struct {
	mu          sync.Mutex
	handles     []assemblySourceHandle
	closing     bool
	ready       bool
	closeHandle func(syscall.Handle) error // nil in production; exact inert fault seam.
	registry    *assemblyOriginalRegistry  // same owner, metadata only; no native values
	// nil in production; only Windows test sources supply original API outcomes.
	openFile func(*uint16, uint32, uint32, *syscall.SecurityAttributes, uint32, uint32, int32) (syscall.Handle, error)
}

func assemblySourceGuardSupported() bool { return true }

func (g *assemblySourceGuard) Close() error {
	if g.registry != nil {
		if err := g.registry.beginClose(); err != nil {
			return err
		}
		// Retirement prevents new original IO. Wait for existing g.mu IO only
		// after the independent metadata latch; never hold registryMu here.
		g.mu.Lock()
		g.closing = true
		held := append([]assemblySourceHandle(nil), g.handles...)
		g.mu.Unlock()
		var first error
		remaining := []assemblySourceHandle{}
		for _, h := range held {
			if err := g.registry.beginSlotClose(h.slotID); err != nil {
				remaining = append(remaining, h)
				if first == nil {
					first = err
				}
				continue
			}
			closeHandle := g.closeHandle
			if closeHandle == nil {
				closeHandle = syscall.CloseHandle
			}
			err := closeHandle(h.handle)
			g.registry.acknowledgeClose(h.slotID, err == nil)
			if err != nil {
				remaining = append(remaining, h)
				if first == nil {
					first = err
				}
			}
		}
		g.mu.Lock()
		g.handles = remaining
		g.mu.Unlock()
		g.registry.finishClose()
		state, _ := g.registry.lease.RetirementState()
		if !state.AllSlotsClosed && first == nil {
			first = backend.ErrUnavailable
		}
		return first
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closing = true
	var first error
	remaining := []assemblySourceHandle{}
	for _, h := range g.handles {
		closeHandle := g.closeHandle
		if closeHandle == nil {
			closeHandle = syscall.CloseHandle
		}
		if err := closeHandle(h.handle); err != nil {
			remaining = append(remaining, h)
			if first == nil {
				first = err
			}
		}
	}
	g.handles = remaining
	return first
}

func assemblySourceIdentityEqual(a, b syscall.ByHandleFileInformation) bool {
	identity := a.FileAttributes == b.FileAttributes && a.CreationTime == b.CreationTime && a.VolumeSerialNumber == b.VolumeSerialNumber && a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
	if a.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		// Ancestors can contain unrelated siblings. Their content timestamps are
		// not our source identity; held handles still deny rename/reparse replacement.
		return identity
	}
	return identity && a.LastWriteTime == b.LastWriteTime && a.FileSizeHigh == b.FileSizeHigh && a.FileSizeLow == b.FileSizeLow && a.NumberOfLinks == b.NumberOfLinks
}

func (g *assemblySourceGuard) CheckCurrent(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.checkCurrentLocked(ctx)
}

func (g *assemblySourceGuard) checkCurrentLocked(ctx context.Context) error {
	if !g.ready || g.closing || len(g.handles) == 0 || ctx.Err() != nil || g.registry != nil && g.registry.retiring.Load() {
		return backend.ErrUnavailable
	}
	for _, held := range g.handles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var current syscall.ByHandleFileInformation
		if syscall.GetFileInformationByHandle(held.handle, &current) != nil || !assemblySourceIdentityEqual(held.info, current) {
			return backend.ErrUnavailable
		}
	}
	return ctx.Err()
}

type assemblySourceCollector struct {
	guard   *assemblySourceGuard
	open    func(string, bool, int64) error
	parents func(string) error
}

func newAssemblySourceCollector(ctx context.Context) *assemblySourceCollector {
	g := &assemblySourceGuard{registry: newAssemblyOriginalRegistry(false, nil)}
	_ = g.registry.start() // fresh private dynamic legacy collector, no IO
	return newAssemblySourceCollectorForGuard(ctx, g)
}

func newAssemblySourceCollectorForGuard(ctx context.Context, g *assemblySourceGuard) *assemblySourceCollector {
	seen := map[string]syscall.ByHandleFileInformation{}
	open := func(path string, directory bool, bytes int64) error {
		volume := filepath.VolumeName(path)
		if ctx.Err() != nil || g.registry.retiring.Load() || len(path) > 32760 || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(volume) != 2 || volume[1] != ':' || strings.Contains(path[len(volume):], ":") {
			return backend.ErrUnavailable
		}
		key := strings.ToLower(path)
		if info, ok := seen[key]; ok {
			if (info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0) != directory || bytes > 0 && (info.FileSizeHigh != uint32(uint64(bytes)>>32) || info.FileSizeLow != uint32(bytes)) {
				return backend.ErrUnavailable
			}
			return nil
		}
		if len(g.handles) >= assemblySourceHandleLimit {
			return backend.ErrUnavailable
		}
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		slotID, slotErr := g.registry.open(key, directory, bytes)
		if slotErr != nil {
			return slotErr
		}
		if ctx.Err() != nil || g.registry.retiring.Load() {
			g.registry.acknowledgeOpen(slotID, false, true) // this frame did not call the native API
			return backend.ErrUnavailable
		}
		// Sharing READ only refuses already-open writers and denies subsequent
		// writes/deletion/rename. OPEN_REPARSE_POINT lets us reject redirects.
		openFile := g.openFile
		if openFile == nil {
			openFile = syscall.CreateFile
		}
		h, err := openFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		valid := h != 0 && h != syscall.InvalidHandle
		// Retain the exact ACK before metadata/error handling, including a valid
		// handle+error. No Close can run while this collector remains in flight.
		if valid {
			g.handles = append(g.handles, assemblySourceHandle{handle: h, path: path, slotID: slotID})
		}
		g.registry.acknowledgeOpen(slotID, valid, h == syscall.InvalidHandle)
		if err != nil {
			return err
		}
		if !valid {
			return backend.ErrUnavailable
		}
		var info syscall.ByHandleFileInformation
		if err = syscall.GetFileInformationByHandle(h, &info); err != nil {
			return err
		}
		kind, typeErr := syscall.GetFileType(h)
		if typeErr != nil || kind != syscall.FILE_TYPE_DISK || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0) != directory || !directory && info.NumberOfLinks != 1 || bytes > 0 && (info.FileSizeHigh != uint32(uint64(bytes)>>32) || info.FileSizeLow != uint32(bytes)) {
			return backend.ErrUnsupported
		}
		g.handles[len(g.handles)-1].info = info
		seen[key] = info
		return nil
	}
	parents := func(path string) error {
		var paths []string
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			paths = append(paths, parent)
			if parent == filepath.Dir(parent) {
				break
			}
			if len(paths) > 64 {
				return backend.ErrUnsupported
			}
		}
		for i := len(paths) - 1; i >= 0; i-- {
			if err := open(paths[i], true, 0); err != nil {
				return err
			}
		}
		return nil
	}
	return &assemblySourceCollector{g, open, parents}
}

func lockAssemblySources(ctx context.Context, spec backend.DocumentImageAssemblySpec, objects ...backend.DocumentImageAssemblyRetainedObject) (*assemblySourceGuard, error) {
	collector := newAssemblySourceCollector(ctx)
	defer collector.guard.registry.stop()
	g, open, parents := collector.guard, collector.open, collector.parents
	if err := parents(spec.InputPath); err != nil {
		return g, err
	}
	// Lock directories before walking their children; the complete immutable
	// manifest decides permitted payload files during subsequent verification.
	err := walkAssemblySources(ctx, spec.InputPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return open(path, entry.IsDir(), 0)
	})
	if err == nil {
		err = parents(spec.HelperPath)
	}
	if err == nil {
		err = open(spec.HelperPath, false, spec.HelperBytes)
	}
	for _, object := range objects {
		if err != nil {
			break
		}
		err = parents(object.Path)
		if err == nil {
			err = open(object.Path, false, object.Bytes)
		}
	}
	if err != nil {
		return g, err
	}
	if err = ctx.Err(); err != nil {
		return g, err
	}
	g.ready = true
	return g, nil
}
