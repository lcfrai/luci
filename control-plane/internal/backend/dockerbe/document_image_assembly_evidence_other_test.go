//go:build !windows

package dockerbe

import (
	"context"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"testing"
)

func TestAssemblyEvidenceNonWindowsCannotSubstituteAdvisoryGuard(t *testing.T) {
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{{Role: "fixture", Path: "/not-opened", Bytes: 1, Digest: "unused"}})
	if guard != nil || !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal("unsupported platform acquired or guessed ownership", guard, err)
	}
}
