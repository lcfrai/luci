//go:build !windows

package dockerbe

import (
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"testing"
)

func TestAssemblyRegistryUnsupportedPlatformCannotPrepareNativeSeal(t *testing.T) {
	objects := []backend.DocumentImageAssemblyEvidenceObject{{Role: "sample", Path: "/unopened", Bytes: 1, Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	if a, e := PrepareDocumentImageAssemblyEvidenceAcquisition(objects); a != nil || e == nil {
		t.Fatal("unsupported evidence acquired a native seal")
	}
	if a, e := PrepareDocumentImageAssemblyPackageAcquisition(objects); a != nil || e == nil {
		t.Fatal("unsupported package acquired a native seal")
	}
}
