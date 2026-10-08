package product

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// Register ownership only at creation by testing.T.TempDir. An absolute path,
// a directory named "tmp", or a Git repository alone is not fixture authority.
var fixtureDirectoryOwners = struct {
	sync.Mutex
	roots map[*testing.T][]string
}{roots: make(map[*testing.T][]string)}

func ownedFixtureTempDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	fixtureDirectoryOwners.Lock()
	fixtureDirectoryOwners.roots[t] = append(fixtureDirectoryOwners.roots[t], root)
	fixtureDirectoryOwners.Unlock()
	t.Cleanup(func() {
		fixtureDirectoryOwners.Lock()
		delete(fixtureDirectoryOwners.roots, t)
		fixtureDirectoryOwners.Unlock()
	})
	return root
}

func ownedFixtureRoot(t *testing.T, dir string) (string, error) {
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return "", errors.New("fixture worktree must be an explicit absolute owned path")
	}
	fixtureDirectoryOwners.Lock()
	var roots []string
	for owner, owned := range fixtureDirectoryOwners.roots {
		if owner == t || strings.HasPrefix(t.Name(), owner.Name()+"/") {
			roots = append(roots, owned...)
		}
	}
	fixtureDirectoryOwners.Unlock()
	for _, root := range roots {
		relative, err := filepath.Rel(root, dir)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			continue
		}
		if _, err = hostpath.Confine(root, dir); err != nil {
			return "", err
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return "", errors.New("fixture path must be an existing owned directory")
		}
		return root, nil
	}
	return "", errors.New("fixture worktree was not created in this test's registered temporary directory")
}

func validateOwnedFixtureDirectory(t *testing.T, dir string) error {
	_, err := ownedFixtureRoot(t, dir)
	return err
}

func validateOwnedFixtureWorktree(t *testing.T, dir string) error {
	root, err := ownedFixtureRoot(t, dir)
	if err != nil {
		return err
	}
	metadata := filepath.Join(dir, ".git")
	info, err := os.Lstat(metadata)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("fixture worktree needs its own ordinary Git metadata directory")
	}
	_, err = hostpath.Confine(root, metadata)
	return err
}

func mustSwarmPathFixture(t *testing.T, s *Server, projectID, runID, operationID string) string {
	t.Helper()
	path, err := s.swarmPath(projectID, runID, operationID)
	if err != nil || path == "" || !filepath.IsAbs(path) {
		t.Fatalf("resolve owned swarm fixture path for %s: path=%q error=%v", operationID, path, err)
	}
	return path
}
