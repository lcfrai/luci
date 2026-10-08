//go:build !windows

package dockerbe

import (
	"os"
	"testing"
)

func privateStorageTestFile(t *testing.T, path string) {
	t.Helper()
	if e := os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
}
