package project_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestManagedGitPinsWorktreeDespiteRepositoryAndEnvironmentOverrides(t *testing.T) {
	root := longTemp(t)
	repo := filepath.Join(root, "app")
	makeGitRepo(t, repo)
	outside := longTemp(t)
	const secret = "FOREIGN_PRIVATE_CONTENT_DO_NOT_EXPOSE"
	if err := os.WriteFile(filepath.Join(outside, "README.md"), []byte(secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	attributes := filepath.Join(outside, "attributes")
	if err := os.WriteFile(attributes, []byte("README.md -diff\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "config", "core.worktree", filepath.ToSlash(outside))
	// Prove this exact on-disk attack exposes another directory without the guard.
	unsafe := gitIn(t, repo, "diff", "--no-ext-diff", "--no-textconv")
	if !strings.Contains(unsafe, secret) {
		t.Fatal("core.worktree attack fixture did not reproduce")
	}
	gitIn(t, repo, "config", "core.bare", "true")
	gitIn(t, repo, "config", "core.attributesFile", filepath.ToSlash(attributes))
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("assigned workspace edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &project.ExecRunner{ProjectsDir: root, Env: []string{
		"GIT_WORK_TREE=" + outside, "GIT_DIR=" + filepath.Join(outside, ".git"),
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=" + outside,
	}}
	out, err := runner.Run(context.Background(), repo, "diff", "--no-ext-diff", "--no-textconv")
	if err != nil || !strings.Contains(string(out), "+assigned workspace edit") || strings.Contains(string(out), secret) {
		t.Fatalf("managed diff exposed redirected content or lost its own edit: %v %s", err, out)
	}
	if _, err := runner.Run(context.Background(), outside, "status", "--porcelain"); err == nil {
		t.Fatal("managed runner accepted an external invocation directory")
	}
}

func TestManagedScannerAllowsInternalLinkedWorktreesAndRejectsExternalMetadata(t *testing.T) {
	root := longTemp(t)
	repo := filepath.Join(root, "app")
	makeGitRepo(t, repo)
	linked := filepath.Join(repo, ".luci", "worktrees", "feature")
	gitIn(t, repo, "worktree", "add", linked, "evidence-upload")
	outside := filepath.Join(longTemp(t), "outside")
	gitIn(t, repo, "worktree", "add", "--detach", outside, "HEAD")
	scanner := project.NewScanner(root, project.NewExecRunner()).WithProjectBoundary()
	ctx := context.Background()
	got, err := scanner.Path(ctx, project.WorktreeID(linked))
	if err != nil || filepath.Clean(got) != filepath.Clean(linked) {
		t.Fatalf("in-root linked worktree was refused: %q %v", got, err)
	}
	rows, err := scanner.Worktrees(ctx, project.ProjectID(repo))
	if err != nil || len(rows) != 2 {
		t.Fatalf("external registry entry was not filtered: %#v %v", rows, err)
	}
	if path, err := scanner.Path(ctx, project.WorktreeID(outside)); err == nil || path != "" {
		t.Fatalf("external worktree became an API mount: %q %v", path, err)
	}
	runner := &project.ExecRunner{ProjectsDir: root}
	if out, err := runner.Run(ctx, linked, "rev-parse", "--show-toplevel"); err != nil || filepath.Clean(strings.TrimSpace(string(out))) != filepath.Clean(linked) {
		t.Fatalf("linked worktree Git read failed: %s %v", out, err)
	}
	foreign := filepath.Join(longTemp(t), "foreign")
	makeGitRepo(t, foreign)
	if err := os.WriteFile(filepath.Join(repo, ".git", "commondir"), []byte(filepath.ToSlash(filepath.Join(foreign, ".git"))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := runner.Run(ctx, repo, "for-each-ref"); err == nil || len(out) != 0 {
		t.Fatal("primary commondir redirection reached Git")
	}
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: "+filepath.ToSlash(filepath.Join(foreign, ".git"))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := runner.Run(ctx, linked, "for-each-ref"); err == nil || len(out) != 0 {
		t.Fatal("external linked-worktree gitdir reached Git")
	}
}

func TestManagedGitRefusesRepositoryConfigIncludesBeforeReadingHostFile(t *testing.T) {
	root := longTemp(t)
	repo := filepath.Join(root, "app")
	makeGitRepo(t, repo)
	foreign := filepath.Join(longTemp(t), "foreign-config")
	if err := os.WriteFile(foreign, []byte("PRIVATE_HOST_FILE_MARKER\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "config", "include.path", filepath.ToSlash(foreign))
	out, err := (&project.ExecRunner{ProjectsDir: root}).Run(context.Background(), repo, "status", "--porcelain")
	if err == nil || len(out) != 0 || strings.Contains(err.Error(), "PRIVATE_HOST_FILE_MARKER") {
		t.Fatalf("repository include was read or not refused: %s %v", out, err)
	}
}

func TestManagedGitRefusesBOMIncludesInCommonAndWorktreeConfig(t *testing.T) {
	for _, kind := range []string{"include", "conditional-include", "worktree-include"} {
		t.Run(kind, func(t *testing.T) {
			root := longTemp(t)
			repo := filepath.Join(root, "app")
			makeGitRepo(t, repo)
			foreign := filepath.Join(longTemp(t), "foreign-config")
			if err := os.WriteFile(foreign, []byte("[luci]\n marker = included-fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(repo, ".git", "config")
			section := "[include]"
			if kind == "conditional-include" {
				section = "[includeIf \"gitdir:" + filepath.ToSlash(filepath.Join(repo, ".git")) + "\"]"
			}
			if kind == "worktree-include" {
				gitIn(t, repo, "config", "extensions.worktreeConfig", "true")
				file = filepath.Join(repo, ".git", "config.worktree")
			}
			original, err := os.ReadFile(file)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			prefix := "\xef\xbb\xbf" + section + "\n path = " + filepath.ToSlash(foreign) + "\n"
			if err := os.WriteFile(file, append([]byte(prefix), original...), 0600); err != nil {
				t.Fatal(err)
			}
			if value := gitIn(t, repo, "config", "--get", "luci.marker"); !strings.Contains(value, "included-fixture") {
				t.Fatal("real Git did not follow the BOM-prefixed fixture include")
			}
			if err := os.WriteFile(foreign, []byte("PRIVATE_HOST_FILE_MARKER\n"), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := (&project.ExecRunner{ProjectsDir: root}).Run(context.Background(), repo, "status", "--porcelain")
			if err == nil || len(out) != 0 || !strings.Contains(err.Error(), "repository Git configuration cannot include host files") || strings.Contains(err.Error(), "PRIVATE_HOST_FILE_MARKER") {
				t.Fatalf("BOM-prefixed config reached an external include: %s %v", out, err)
			}
		})
	}
}
