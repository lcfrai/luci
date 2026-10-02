//go:build !windows

package product

import (
	"os"
	"testing"
)

func assertPrivateFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("credential file permissions are %o", info.Mode().Perm())
	}
}
