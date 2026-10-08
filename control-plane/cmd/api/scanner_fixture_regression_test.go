package main

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScannerGitFixtureIgnoresInheritedRepositoryRedirection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git required")
	}
	foreign := longTempDir(t)
	gitNeutral(t, foreign, "init", "-b", "main")
	gitNeutral(t, foreign, "commit", "--allow-empty", "-m", "foreign baseline")
	snapshot := func() map[string][32]byte {
		t.Helper()
		files := make(map[string][32]byte)
		if err := filepath.WalkDir(foreign, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := os.ReadFile(path)
			if err == nil {
				files[path] = sha256.Sum256(data)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	for key, value := range map[string]string{
		"GIT_DIR": filepath.Join(foreign, ".git"), "GIT_WORK_TREE": foreign,
		"GIT_COMMON_DIR": filepath.Join(foreign, ".git"), "GIT_INDEX_FILE": filepath.Join(foreign, ".git", "index"),
		"GIT_OBJECT_DIRECTORY": filepath.Join(foreign, ".git", "objects"),
		"GIT_CONFIG_COUNT":     "1", "GIT_CONFIG_KEY_0": "core.worktree", "GIT_CONFIG_VALUE_0": foreign,
	} {
		t.Setenv(key, value)
	}
	owned := longTempDir(t)
	gitNeutral(t, owned, "init", "-b", "main")
	gitNeutral(t, owned, "commit", "--allow-empty", "-m", "owned commit")
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("scanner fixture changed the disposable foreign repository or its Git metadata")
	}
	if _, err := os.Stat(filepath.Join(owned, ".git", "refs", "heads", "main")); err != nil {
		t.Fatal("scanner fixture did not commit to its own repository", err)
	}
}
