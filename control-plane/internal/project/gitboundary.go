package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// WithProjectBoundary is required for an API scanner serving an assigned
// workspace. The older standalone scanner can still enumerate a developer's
// external worktrees, but those must never become another user's API mounts.
func (s *Scanner) WithProjectBoundary() *Scanner {
	s.bounded = true
	if runner, ok := s.git.(*ExecRunner); ok {
		copy := *runner
		copy.ProjectsDir = s.root
		s.git = &copy
	}
	return s
}

func confinedGitPath(root, path string) (string, error) {
	checked, err := hostpath.Confine(root, path)
	if err != nil {
		return "", err
	}
	if _, err := hostpath.ConfineToMounts([]string{root}, path); err != nil {
		return "", err
	}
	return checked, nil
}

func gitMetadataFile(root, path string) ([]byte, error) {
	checked, err := confinedGitPath(root, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(checked)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return nil, errors.New("project: Git metadata must be a bounded regular file")
	}
	if err := hostpath.OneName(checked); err != nil {
		return nil, err
	}
	f, err := os.Open(checked)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 1024*1024+1))
}

var gitIncludeSection = regexp.MustCompile(`(?im)^\s*\[\s*include(?:if)?(?:\s|\])`)

// ManagedGitArguments reuses the catalogue's assigned-workspace boundary for
// trusted controller Git operations. The caller selects a direct subcommand;
// callers must still disable any configured filters before checking out files.
func ManagedGitArguments(root, dir string, args []string) ([]string, error) {
	return boundedGitArgs(root, dir, args)
}

// ManagedGitEnvironment removes inherited Git redirection and external config
// before a controller inspects or changes an owned repository.
func ManagedGitEnvironment(env []string) []string { return boundedGitEnvironment(env) }

// Resolve .git ourselves before asking Git anything. A legitimate linked
// worktree points to an in-root .git/worktrees entry whose commondir points
// back to the same repository; a primary repository cannot redirect commondir.
func boundedGitDirectory(root, dir string) (string, string, error) {
	work, err := confinedGitPath(root, dir)
	if err != nil {
		return "", "", err
	}
	entry := filepath.Join(work, ".git")
	gitdir, err := confinedGitPath(root, entry)
	if err != nil {
		return "", "", err
	}
	info, err := os.Lstat(gitdir)
	if err != nil {
		return "", "", err
	}
	linked := !info.IsDir()
	if linked {
		raw, err := gitMetadataFile(root, entry)
		if err != nil {
			return "", "", err
		}
		line := strings.TrimSpace(string(raw))
		if !strings.HasPrefix(line, "gitdir: ") || strings.ContainsAny(line, "\r\n\x00") {
			return "", "", errors.New("project: invalid linked-worktree metadata")
		}
		gitdir = strings.TrimPrefix(line, "gitdir: ")
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(work, gitdir)
		}
		gitdir, err = confinedGitPath(root, gitdir)
		if err != nil {
			return "", "", err
		}
	}
	common := gitdir
	raw, err := gitMetadataFile(root, filepath.Join(gitdir, "commondir"))
	if err == nil {
		if !linked {
			return "", "", errors.New("project: a primary repository cannot redirect its common Git directory")
		}
		common = strings.TrimSpace(string(raw))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
		common, err = confinedGitPath(root, common)
		if err != nil {
			return "", "", err
		}
		if filepath.Clean(filepath.Dir(filepath.Dir(gitdir))) != common || filepath.Base(filepath.Dir(gitdir)) != "worktrees" {
			return "", "", errors.New("project: linked-worktree metadata is not in its common repository")
		}
	} else if !errors.Is(err, os.ErrNotExist) || linked {
		return "", "", fmt.Errorf("project: invalid Git common directory: %w", err)
	}
	for _, metadata := range []string{filepath.Join(common, "config"), filepath.Join(gitdir, "config.worktree")} {
		raw, err := gitMetadataFile(root, metadata)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		// Git accepts a UTF-8 BOM before the first section. Normalize only
		// that marker so it cannot hide an include from this pre-Git guard.
		if gitIncludeSection.Match(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})) {
			return "", "", errors.New("project: repository Git configuration cannot include host files")
		}
	}
	// An alternate object database is another host-file read path. Managed
	// repositories must own their object store; normal clones/worktrees do.
	if _, err := os.Lstat(filepath.Join(common, "objects", "info", "alternates")); err == nil || !errors.Is(err, os.ErrNotExist) {
		return "", "", errors.New("project: external Git object databases are not supported in managed workspaces")
	}
	for _, path := range []string{filepath.Join(common, "objects"), filepath.Join(common, "refs"), filepath.Join(gitdir, "HEAD")} {
		if _, err := confinedGitPath(root, path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
	}
	return gitdir, work, nil
}

func boundedGitArgs(root, dir string, args []string) ([]string, error) {
	gitdir, work, err := boundedGitDirectory(root, dir)
	if err != nil {
		return nil, fmt.Errorf("project: Git workspace boundary refused: %w", err)
	}
	// Callers choose subcommands, never a second directory or repository. The
	// trusted location flags cannot be overridden by repository core.worktree.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return nil, errors.New("project: managed Git requires a direct subcommand")
	}
	prefix := []string{"--git-dir=" + gitdir, "--work-tree=" + work,
		"-c", "core.worktree=" + work, "-c", "core.bare=false",
		"-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.attributesFile=" + os.DevNull, "-c", "core.excludesFile=" + os.DevNull,
		// A completed controller command must not leave an untracked metadata
		// writer behind. Detached auto-maintenance can remove lock/PID files
		// during the next strict metadata walk; older Git uses gc --auto.
		// These per-command overrides leave explicit native maintenance intact.
		"-c", "maintenance.auto=false", "-c", "gc.auto=0"}
	return append(prefix, args...), nil
}

func boundedGitEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat")
}
