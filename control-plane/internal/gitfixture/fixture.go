// Package gitfixture confines native Git test helpers to directories minted by
// the current test. Production code must not import this test support package.
package gitfixture

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

var owners = struct {
	sync.Mutex
	roots map[*testing.T][]string
}{roots: make(map[*testing.T][]string)}

// TempDir creates, normalizes and registers a fresh test-owned directory.
func TempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owners.Lock()
	owners.roots[t] = append(owners.roots[t], root)
	owners.Unlock()
	t.Cleanup(func() {
		owners.Lock()
		delete(owners.roots, t)
		owners.Unlock()
	})
	return root
}

// DirectoryRoot checks ownership before a caller writes or starts Git. Only
// explicit creation helpers may accept missing descendants of the owned root.
func DirectoryRoot(t *testing.T, dir string, allowMissing bool) (string, error) {
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return "", errors.New("Git fixture directory must be an explicit absolute path")
	}
	owners.Lock()
	var roots []string
	for owner, paths := range owners.roots {
		if owner == t || strings.HasPrefix(t.Name(), owner.Name()+"/") {
			roots = append(roots, paths...)
		}
	}
	owners.Unlock()
	for _, root := range roots {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		// Reject symlinks and Windows reparse points before MkdirAll, including
		// an existing ancestor of a not-yet-created directory.
		current := root
		parts := []string{"."}
		if rel != "." {
			parts = append(parts, strings.Split(rel, string(filepath.Separator))...)
		}
		for index, part := range parts {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if err != nil {
				if index > 0 && allowMissing && os.IsNotExist(err) {
					return root, nil
				}
				return "", errors.New("Git fixture directory is unavailable")
			}
			if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return "", errors.New("Git fixture directory must not traverse a link or file")
			}
		}
		return root, nil
	}
	return "", errors.New("Git fixture directory is not owned by this test")
}

// Environment strips inherited repository/index/config redirection and prevents
// an empty scratch directory from discovering a repository above its owner root.
func Environment(t *testing.T, dir string, allowMissing bool) []string {
	t.Helper()
	root, err := DirectoryRoot(t, dir, allowMissing)
	if err != nil {
		t.Fatal(err)
	}
	return append(project.ManagedGitEnvironment(os.Environ()),
		"GIT_CEILING_DIRECTORIES="+root,
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
}
