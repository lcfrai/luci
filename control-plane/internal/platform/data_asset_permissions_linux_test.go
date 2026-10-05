//go:build linux

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDataBundledAssetsRemainReadableWithPrivateWorkerUmask(t *testing.T) {
	const child = "LUCI_DATA_ASSET_UMASK_REGRESSION"
	if os.Getenv(child) != "1" {
		// Umask belongs to the process. Use an isolated test process rather than
		// changing permissions for other package tests or worker state.
		command := exec.Command(os.Args[0], "-test.run=^TestDataBundledAssetsRemainReadableWithPrivateWorkerUmask$", "-test.v")
		command.Env = append(os.Environ(), child+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("private-umask child failed: %v\n%s", err, output)
		}
		return
	}
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	s, _ := newDataFake(t)
	install := dataInstallation{ProjectID: "owned", StackID: "stack-umask", APIPort: 54321}
	secret := newDataSecret()
	manifest, err := reviewedDataAssets()
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.dataRoot(install)
	if err != nil {
		t.Fatal(err)
	}
	assertMode := func(path string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s permission = %04o, want %04o", filepath.Base(path), info.Mode().Perm(), want)
		}
	}
	for pass := 0; pass < 2; pass++ {
		if err := s.prepareDataStack(install, secret); err != nil {
			t.Fatal(err)
		}
		for _, file := range manifest.Files {
			if !strings.HasPrefix(file.Path, "volumes/") {
				continue
			}
			target := filepath.Join(root, filepath.FromSlash(file.Path))
			assertMode(target, 0644)
			for parent := filepath.Dir(target); parent != root; parent = filepath.Dir(parent) {
				assertMode(parent, 0755)
			}
		}
		assertMode(root, 0700)
		assertMode(filepath.Join(root, "compose.yaml"), 0600)
		assertMode(filepath.Join(root, "runtime.env"), 0600)
		// An upgrade must correct existing assets produced by the old masked
		// write path, as well as creating new assets with explicit modes.
		if pass == 0 {
			if err := os.Chmod(filepath.Join(root, "volumes", "db", "webhooks.sql"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Join(root, "volumes", "functions"), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
}
