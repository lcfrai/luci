//go:build !windows

package resourcebudget

import (
	"os"
	"testing"
)

func makeStorageRecordPrivate(t *testing.T, path string) {
	t.Helper()
	if e := os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
}
