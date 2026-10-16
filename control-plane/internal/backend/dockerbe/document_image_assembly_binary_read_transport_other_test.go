//go:build !windows

package dockerbe

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"testing"
)

func TestAssemblyBinaryTransportNonWindowsCannotExposeNativeReadHandle(t *testing.T) {
	transport, _, _, _, _ := assemblyBinaryTransportMetadataFixture(t)
	defer func() { transport.Retire(); _ = transport.ReleaseRetiredOriginalBorrows() }()
	borrow, e := transport.BorrowOriginalForDuplicate(context.Background(), 0)
	if e != nil {
		t.Fatal(e)
	}
	if h, e := borrow.NativeReadHandle(); h != 0 || e != backend.ErrUnsupported {
		t.Fatal("unsupported platform minted handle", h, e)
	}
	if borrow.ReturnFromDuplicate() != nil {
		t.Fatal("declined metadata operation not returned")
	}
}
