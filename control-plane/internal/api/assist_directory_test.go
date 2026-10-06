package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAssistantDirectoryCreatesMissingPrivateRoot(t *testing.T) {
	projects, parent := t.TempDir(), t.TempDir()
	path := filepath.Join(parent, "new-state", "conversations")
	a := &API{deps: Deps{ProjectsDir: projects, ConversationsDir: path}}
	got, err := a.assistDirectory(filepath.Join("assistance", "jobs"))
	if err != nil || got != filepath.Join(path, "assistance", "jobs") {
		t.Fatal("fresh assistant storage could not initialize", err)
	}
	for _, dir := range []string{filepath.Join(parent, "new-state"), path, got} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatal("private directory missing", err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
			t.Fatalf("private directory permissions: %v", info.Mode())
		}
	}
}

func TestAssistantDirectoryRejectsUnsafeRootsBeforeCreatingState(t *testing.T) {
	for _, kind := range []string{"inside-projects", "link-through-projects", "linked-root", "dangling-link", "linked-assistance"} {
		t.Run(kind, func(t *testing.T) {
			projects, outside := t.TempDir(), t.TempDir()
			path := filepath.Join(outside, "conversations")
			unexpected := filepath.Join(projects, "missing")
			link := func(target, name string) {
				t.Helper()
				if err := os.Symlink(target, name); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			switch kind {
			case "inside-projects":
				path = filepath.Join(unexpected, "conversations")
			case "link-through-projects":
				link(outside, filepath.Join(projects, "return-outside"))
				link(filepath.Join(projects, "return-outside"), filepath.Join(outside, "via-project"))
				path = filepath.Join(outside, "via-project", "missing", "conversations")
				unexpected = filepath.Join(outside, "missing")
			case "linked-root":
				link(outside, path)
				unexpected = filepath.Join(outside, "assistance")
			case "dangling-link":
				unexpected = filepath.Join(outside, "missing")
				link(unexpected, path)
				path = filepath.Join(path, "conversations")
			case "linked-assistance":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				link(outside, filepath.Join(path, "assistance"))
				unexpected = filepath.Join(outside, "jobs")
			}
			a := &API{deps: Deps{ProjectsDir: projects, ConversationsDir: path}}
			if _, err := a.assistDirectory(filepath.Join("assistance", "jobs")); err == nil {
				t.Fatal("unsafe private storage path was accepted")
			}
			if _, err := os.Lstat(unexpected); !os.IsNotExist(err) {
				t.Fatal("rejected path created storage before validation", err)
			}
		})
	}
}
