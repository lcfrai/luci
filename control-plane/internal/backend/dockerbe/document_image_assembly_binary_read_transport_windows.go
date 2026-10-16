//go:build windows

package dockerbe

import (
	"context"
	"strings"
	"syscall"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblyBinaryOriginalNative struct {
	handle syscall.Handle
	slotID uint64
	info   syscall.ByHandleFileInformation
}

// Metadata-only stable registry/ref lookup; no owner mutex is held by caller.
func bindAssemblyBinaryOriginal(b *assemblyBinaryOriginalBinding) error {
	if b.guard == nil || b.guard.registry == nil {
		return backend.ErrLaunchFenceOwnership
	}
	r := b.guard.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped || r.retiring.Load() || r.closeActive {
		return backend.ErrUnavailable
	}
	id := r.paths[strings.ToLower(b.object.Path)]
	if id == 0 || id > uint64(len(r.slots)) {
		return backend.ErrLaunchFenceOwnership
	}
	s := r.slots[id-1]
	if s.directory || s.bytes != b.object.Bytes || s.state != "valid-ack" {
		return backend.ErrLaunchFenceOwnership
	}
	b.registry, b.slotID = r, id
	return nil
}

// No alias/duplicate/open/read/seek occurs. Only the original selected leaf is
// queried. Existing guarded ancestors stay owned, and793 exclusive borrow
// prevents Close/Transfer/cursor reads. IO holds ONLY the existing native g.mu.
func lookupAssemblyBinaryOriginal(ctx context.Context, b assemblyBinaryOriginalBinding) (*assemblyBinaryOriginalNative, error) {
	g := b.guard
	if g == nil || b.registry == nil || g.registry != b.registry || ctx.Err() != nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || g.closing || b.registry.retiring.Load() || ctx.Err() != nil {
		return nil, backend.ErrUnavailable
	}
	var native *assemblyBinaryOriginalNative
	for _, held := range g.handles {
		if held.slotID == b.slotID && strings.EqualFold(held.path, b.object.Path) {
			native = &assemblyBinaryOriginalNative{handle: held.handle, slotID: held.slotID, info: held.info}
			break
		}
	}
	if native == nil || native.handle == 0 || native.handle == syscall.InvalidHandle || native.info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 || native.info.FileSizeHigh != uint32(uint64(b.object.Bytes)>>32) || native.info.FileSizeLow != uint32(b.object.Bytes) {
		return native, backend.ErrLaunchFenceOwnership
	}
	var current syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(native.handle, &current); err != nil {
		return native, err
	}
	kind, err := syscall.GetFileType(native.handle)
	if err != nil || kind != syscall.FILE_TYPE_DISK || !assemblySourceIdentityEqual(native.info, current) || current.NumberOfLinks != 1 || current.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || ctx.Err() != nil || b.registry.retiring.Load() {
		return native, backend.ErrUnavailable
	}
	return native, nil
}

// Native value OUTPUT only for the fixed private API DuplicateHandle caller.
// It is never an accepted numeric HANDLE/PID/path authority input. At most one
// lookup per exact borrower; partial/late values remain retained on refusal.
func (b *DocumentImageAssemblyBinaryOriginalBorrow) NativeReadHandle() (syscall.Handle, error) {
	binding, ctx, err := b.beginLookup()
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	native, err := lookupAssemblyBinaryOriginal(ctx, binding)
	if err = b.finishLookup(ctx, native, err); err != nil {
		return 0, err
	}
	return native.handle, nil
}
