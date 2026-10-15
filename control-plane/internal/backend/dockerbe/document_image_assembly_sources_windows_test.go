//go:build windows

package dockerbe

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred bounded Windows temporary-file acceptance, no daemon or process.
func TestAssemblySourceGuardDeniesWriteAndRenameUntilExactClose(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "inputs")
	if os.Mkdir(input, 0700) != nil {
		t.Fatal("fixture directory")
	}
	file, helper := filepath.Join(input, "file"), filepath.Join(root, "helper")
	if os.WriteFile(file, []byte("one"), 0600) != nil || os.WriteFile(helper, []byte("helper"), 0600) != nil {
		t.Fatal("fixture files")
	}
	guard, err := lockAssemblySources(context.Background(), backend.DocumentImageAssemblySpec{InputPath: input, HelperPath: helper})
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if os.WriteFile(file, []byte("changed"), 0600) == nil || os.Rename(helper, helper+"-renamed") == nil {
		t.Fatal("live pinned source changed while guarded")
	}
	if guard.Close() != nil {
		t.Fatal("exact handles did not close")
	}
	if os.WriteFile(file, []byte("changed"), 0600) != nil || os.Rename(helper, helper+"-renamed") != nil {
		t.Fatal("guard remained after positive close")
	}
}
