package project_test

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGitFixtureHelpersIgnoreInheritedRepositoryRedirection(t *testing.T) {
	requireGit(t)
	foreign := filepath.Join(longTemp(t), "foreign")
	makeGitRepo(t, foreign)
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
	owned := filepath.Join(longTemp(t), "owned")
	makeGitRepo(t, owned)
	gitIn(t, owned, "config", "luci.fixture", "owned-only")
	if got := gitIn(t, owned, "config", "--get", "luci.fixture"); got != "owned-only\n" {
		t.Fatalf("fixture did not configure its own repository: %q", got)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("fixture changed the disposable foreign repository or its Git metadata")
	}
}
