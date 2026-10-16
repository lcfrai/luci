//go:build windows

package dockerbe

import (
	"context"
	"io/fs"
	"sync"
	"syscall"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblySourceGuard struct {
	mu      sync.Mutex
	handles []syscall.Handle
}

func assemblySourceGuardSupported() bool { return true }

func (g *assemblySourceGuard) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var first error
	remaining := []syscall.Handle{}
	for _, h := range g.handles {
		if err := syscall.CloseHandle(h); err != nil {
			remaining = append(remaining, h)
			if first == nil {
				first = err
			}
		}
	}
	g.handles = remaining
	return first
}
func lockAssemblySources(ctx context.Context, spec backend.DocumentImageAssemblySpec) (*assemblySourceGuard, error) {
	g := &assemblySourceGuard{}
	open := func(path string) error {
		if ctx.Err() != nil || len(g.handles) >= 1024 || len(path) > 32760 {
			return backend.ErrUnavailable
		}
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		// Sharing READ only refuses already-open writers and denies subsequent
		// writes/deletion/rename. OPEN_REPARSE_POINT lets us reject redirects.
		h, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return err
		}
		g.handles = append(g.handles, h)
		var info syscall.ByHandleFileInformation
		if err = syscall.GetFileInformationByHandle(h, &info); err != nil {
			return err
		}
		if info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return backend.ErrUnsupported
		}
		return nil
	}
	// Lock directories before walking their children; the complete immutable
	// manifest decides permitted payload files during subsequent verification.
	err := walkAssemblySources(ctx, spec.InputPath, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return open(path)
	})
	if err == nil {
		err = open(spec.HelperPath)
	}
	if err != nil {
		return g, err
	}
	return g, nil
}
