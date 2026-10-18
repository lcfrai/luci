//go:build !windows

package dockerbe

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestAssemblyNamespaceOtherPlatformRefusesWithoutWeakening(t *testing.T) {
	if a, err := PrepareDocumentImageAssemblyNamespaceAcquisition("/owned/namespace"); a != nil || !errors.Is(err, backend.ErrUnsupported) { t.Fatal(a, err) }
	if g, err := prepareAssemblyOriginalNamespaceGuard("/owned/namespace"); g != nil || !errors.Is(err, backend.ErrUnsupported) { t.Fatal(g, err) }
	if err := acquireAssemblyPreparedNamespace(context.Background(), &assemblySourceGuard{}, "/owned/namespace"); !errors.Is(err, backend.ErrUnsupported) { t.Fatal(err) }
}
