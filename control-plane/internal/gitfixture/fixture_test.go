package gitfixture

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDirectoryOwnershipBeforeCreation(t *testing.T) {
	root := TempDir(t)
	foreign := t.TempDir() // An ordinary temp path is not registered authority.
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"", ".", "relative", foreign, filepath.Dir(root), file, filepath.Join(file, "child")} {
		if _, err := DirectoryRoot(t, dir, true); err == nil {
			t.Fatalf("unsafe creation directory accepted: %q", dir)
		}
	}
	missing := filepath.Join(root, "new", "repo")
	if _, err := DirectoryRoot(t, missing, false); err == nil {
		t.Fatal("execution accepted a missing directory")
	}
	if got, err := DirectoryRoot(t, missing, true); err != nil || got != root {
		t.Fatal("owned directory creation refused", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("validation created its target")
	}
	t.Run("parent-owned-root", func(t *testing.T) {
		if _, err := DirectoryRoot(t, root, false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("link-to-foreign-directory", func(t *testing.T) {
		link := filepath.Join(root, "link")
		if err := os.Symlink(foreign, link); err != nil {
			t.Skipf("this host cannot create the symlink fixture: %v", err)
		}
		if _, err := DirectoryRoot(t, filepath.Join(link, "new"), true); err == nil {
			t.Fatal("creation followed an existing link outside the owned root")
		}
	})
}

func TestEnvironmentStopsDiscoveryInsideDisposableParentRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git required")
	}
	parent := TempDir(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	init := exec.CommandContext(ctx, "git", "-c", "init.templateDir=", "init", "-b", "main")
	init.Dir, init.Env = parent, Environment(t, parent, false)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("disposable parent init: %v %s", err, out)
	}
	before, err := os.ReadFile(filepath.Join(parent, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(variable, parent)
	}
	t.Run("owned-root-and-descendant", func(t *testing.T) {
		root := TempDir(t)
		child := filepath.Join(root, "child")
		if err := os.Mkdir(child, 0700); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{root, child} {
			cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
			cmd.Dir, cmd.Env = dir, Environment(t, dir, false)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("scratch directory discovered its parent repository: %s", out)
			}
		}
	})
	after, err := os.ReadFile(filepath.Join(parent, ".git", "HEAD"))
	if err != nil || string(before) != string(after) {
		t.Fatal("discovery check changed the disposable parent HEAD", err)
	}
}
