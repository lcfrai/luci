//go:build windows

package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"syscall"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func lockAssemblyEvidenceSources(ctx context.Context, objects []backend.DocumentImageAssemblyEvidenceObject) (*assemblySourceGuard, error) {
	collector := newAssemblySourceCollector(ctx)
	for _, object := range objects {
		if err := collector.parents(object.Path); err != nil {
			return collector.guard, err
		}
		if err := collector.open(object.Path, false, object.Bytes); err != nil {
			return collector.guard, err
		}
		// The shared ordinary source collector historically treats size0 as
		// unspecified. Evidence logs require an exact size even when empty.
		held := collector.guard.handles[len(collector.guard.handles)-1]
		if !strings.EqualFold(held.path, object.Path) || held.info.FileSizeHigh != uint32(uint64(object.Bytes)>>32) || held.info.FileSizeLow != uint32(object.Bytes) {
			return collector.guard, backend.ErrUnavailable
		}
	}
	if err := ctx.Err(); err != nil {
		return collector.guard, err
	}
	collector.guard.ready = true
	return collector.guard, nil
}

// Synchronous local-file I/O, checked at every finite chunk boundary. Context is
// not claimed to interrupt an in-flight Windows syscall. The exact owner keeps
// the guard/buffer until it returns; no background reader escapes or path opens.
func (g *assemblySourceGuard) readEvidence(ctx context.Context, object backend.DocumentImageAssemblyEvidenceObject, maximum int64) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if object.Bytes < 0 || object.Bytes > maximum || maximum > backend.DocumentImageAssemblyEvidenceMaxObjectBytes || g.checkCurrentLocked(ctx) != nil {
		return nil, backend.ErrUnavailable
	}
	var original *assemblySourceHandle
	for i := range g.handles {
		held := &g.handles[i]
		if strings.EqualFold(held.path, object.Path) {
			original = held
			break
		}
	}
	if original == nil || original.info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 || original.info.FileSizeHigh != uint32(uint64(object.Bytes)>>32) || original.info.FileSizeLow != uint32(object.Bytes) {
		return nil, backend.ErrLaunchFenceOwnership
	}
	position, err := syscall.Seek(original.handle, 0, 0)
	if err != nil || position != 0 {
		return nil, backend.ErrUnavailable
	}
	data := make([]byte, int(object.Bytes))
	for offset := 0; offset < len(data); {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		end := min(offset+65536, len(data))
		var read uint32
		if err = syscall.ReadFile(original.handle, data[offset:end], &read, nil); err != nil || read == 0 || int(read) > end-offset {
			return nil, backend.ErrUnavailable
		}
		offset += int(read)
	}
	var extra [1]byte
	var read uint32
	err = syscall.ReadFile(original.handle, extra[:], &read, nil)
	if err != nil && err != syscall.ERROR_HANDLE_EOF || read != 0 || g.checkCurrentLocked(ctx) != nil {
		return nil, backend.ErrUnavailable
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != object.Digest || ctx.Err() != nil {
		return nil, backend.ErrUnavailable
	}
	return data, nil
}
