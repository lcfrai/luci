package product

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCompanionStagesInProtectedWindowsDirectory(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "verified-source")
	data := []byte("exact source-bound runtime companion fixture")
	if err := os.WriteFile(sourcePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	stagingDir := filepath.Join(root, "private-runtime-staging")
	staged, err := stageWorkspaceFile(context.Background(), source, int64(len(data)), hashWorkspaceBytes(data), stagingDir, "owned-job")
	if err != nil {
		t.Fatal("normal Windows staging refused before transfer", err)
	}
	defer os.Remove(staged)
	actual, err := os.ReadFile(staged)
	if err != nil || string(actual) != string(data) {
		t.Fatal("staged companion did not retain exact bytes", err)
	}
	if info, err := os.Lstat(stagingDir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("private staging path became linked or unavailable", err)
	}
}
