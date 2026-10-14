//go:build !windows

package product

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCompanionRefusesBroadUnixStagingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broad-stage")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := prepareWorkspaceStagingDir(path); err == nil {
		t.Fatal("broad existing directory was accepted")
	}
}
