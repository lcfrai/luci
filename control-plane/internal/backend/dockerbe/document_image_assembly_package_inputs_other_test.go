//go:build !windows

package dockerbe

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestAssemblyPackageInputsNonWindowsRefusesWithoutOwnership(t *testing.T) {
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), nil)
	if guard != nil || !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal("unsupported platform minted ownership", guard, err)
	}
	native, err := lockAssemblyPackageInputSources(context.Background(), nil)
	if native != nil || !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal("non-Windows native stub claimed settlement", native, err)
	}
}
