//go:build !windows

package dockerbe

import "github.com/lcfrai/luci/control-plane/internal/backend"

type assemblyBinaryOriginalNative struct{}

func bindAssemblyBinaryOriginal(*assemblyBinaryOriginalBinding) error { return backend.ErrUnsupported }
func (b *DocumentImageAssemblyBinaryOriginalBorrow) NativeReadHandle() (uintptr, error) {
	_, ctx, err := b.beginLookup()
	if err != nil {
		return 0, err
	}
	return 0, b.finishLookup(ctx, nil, backend.ErrUnsupported)
}
