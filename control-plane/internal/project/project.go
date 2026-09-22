// Package project enumerates the projects, branches and worktrees the cockpit's
// rail shows. It is READ-ONLY by design: docs/PROGRAM.md phase 3 owns worktree
// lifecycle, and a Phase 2 that could create or delete one would be building
// Phase 3 badly. Three git subcommands are used, all of them reads —
// for-each-ref, rev-parse and worktree list — and project_test.go asserts that
// whitelist mechanically.
package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// ErrUnknownProject is returned for a project id under no configured root. The
// contract leaves the error unnamed; task 10 needs a sentinel to map to
// 404 project_not_found without matching on message text.
var ErrUnknownProject = errors.New("project: unknown project id")

type Project struct {
	ID   string `json:"id"`   // "p-" + first 8 hex of sha256(absolute path)
	Name string `json:"name"` // the directory's base name
	Path string `json:"path"` // absolute
}

type Branch struct {
	ID        string `json:"id"` // "b-" + project hash + "-" + slug(name)
	ProjectID string `json:"projectId"`
	Name      string `json:"name"` // e.g. "evidence-upload"
	IsHead    bool   `json:"isHead"`
}

type Worktree struct {
	ID        string `json:"id"` // "w-" + first 8 hex of sha256(absolute path)
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"` // empty for a detached or bare worktree
	Name      string `json:"name"`     // base name of the worktree directory
	Path      string `json:"path"`     // absolute
	IsPrimary bool   `json:"isPrimary"`
}

// Runner runs git. It is an interface so tests need neither git nor a repo.
//
// An implementation returns git's STANDARD OUTPUT ONLY on success. Everything
// in this file splits that on newlines and trusts every line, so stderr — where
// git puts non-fatal warnings while still exiting 0 — must not be in it. On
// failure the error should carry git's own message; see gitrunner.go.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Scanner enumerates one root directory of repositories.
type Scanner struct {
	root string
	git  Runner

	// names is the display-name cache behind ProjectName and BranchName. Those
	// two satisfy Phase 4's Naming, which forbids an error return and takes no
	// context, so they cannot shell out. Enumeration warms this as a side
	// effect; a cold read returns the id, which is the documented fallback.
	mu    sync.RWMutex
	names map[string]string
}

// NewScanner returns a scanner over root. root may be "", in which case every
// method returns an empty slice and no error — an unconfigured projects
// directory is a normal state, not a failure.
func NewScanner(root string, git Runner) *Scanner {
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		root = filepath.Clean(root)
	}
	return &Scanner{root: root, git: git, names: map[string]string{}}
}

// hashPath is the id derivation: absolute, cleaned, lowercased, sha256, first
// 8 hex. Lowercasing is for Windows, where C:\Users\x and c:\users\x are one
// directory and must not become two rail entries. On a case-sensitive
// filesystem it can merge two genuinely different paths; these are rail ids,
// not a security boundary, and the cost of the alternative is a rail that
// duplicates a project after the user types a path with different casing.
func hashPath(path string) string {
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:])[:8]
}

// ProjectID derives a project id from a directory path.
func ProjectID(path string) string { return "p-" + hashPath(path) }

// WorktreeID derives a worktree id from a directory path.
func WorktreeID(path string) string { return "w-" + hashPath(path) }

// BranchID derives a branch id. It reuses the project's hash so a branch id
// carries its project, which is what lets BranchName work without a lookup.
//
// The trailing segment is a digest of the raw ref name, not the slug alone.
// slug collapses every run of non-[a-z0-9] to one dash, so without the digest
// "feat/a-b", "feat/a/b" and "Feat/A B" would all derive the identical id —
// a REST collection with duplicate ids, and BranchName(id) silently returning
// whichever of them enumeration saw last. Appending sha256(name) keeps the id
// reproducible from the same two inputs with no storage and no lookup — the
// property slug's own doc comment protects — while making it unique; the slug
// segment stays so a human skimming raw ids still sees something readable.
func BranchID(projectID, name string) string {
	sum := sha256.Sum256([]byte(name))
	return "b-" + strings.TrimPrefix(projectID, "p-") + "-" + slug(name) + "-" + hex.EncodeToString(sum[:])[:8]
}

// slug lowercases and replaces every run of characters outside [a-z0-9] with a
// single "-", and does nothing else. It is intentionally lossy for
// readability — "feat/x" and "feat-x" slug the same — which is why
// Branch.Name carries the real ref name and BranchID appends a digest of that
// same raw name so two branches that slug identically still get distinct ids.
//
// No trimming of leading or trailing dashes and no fallback for an empty
// result, tempting as both are: an extra rule here would mean any other phase
// deriving an id for the same branch computes a different slug segment, even
// though BranchID's digest segment already disambiguates on its own.
// "-wip-" slugs to "-wip-".
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return b.String()
}

func (s *Scanner) remember(id, name string) {
	if id == "" || name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.names[id] = name
}

func (s *Scanner) cached(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n, ok := s.names[id]; ok && n != "" {
		return n
	}
	return id
}

// ProjectName implements Phase 4's discoverapi.Naming. It never returns an
// error and falls back to the id, because a missing display name must not fail
// a brief.
func (s *Scanner) ProjectName(projectID string) string { return s.cached(projectID) }

// BranchName implements Phase 4's discoverapi.Naming, on the same terms.
func (s *Scanner) BranchName(branchID string) string { return s.cached(branchID) }

// Projects lists the repositories directly under root. Scanning is depth-1:
// subdirectories of root that contain a .git entry. Deeper recursion over a
// developer's whole disk is how a rail takes forty seconds to paint.
//
// The .git entry is the whole test. A dot-prefixed directory is not skipped:
// the rail would be missing a project the user can see in the folder, with
// nothing anywhere saying why.
func (s *Scanner) Projects(ctx context.Context) ([]Project, error) {
	if s.root == "" {
		return []Project{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// A configured root the user has not created yet is the same
			// normal state as an unconfigured one.
			return []Project{}, nil
		}
		return nil, fmt.Errorf("project: read %s: %w", s.root, err)
	}
	out := make([]Project, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.root, name)
		// A linked worktree has .git as a FILE, a normal clone as a directory.
		// Lstat accepts both and costs no subprocess.
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		p := Project{ID: ProjectID(dir), Name: name, Path: dir}
		s.remember(p.ID, p.Name)
		out = append(out, p)
	}
	return out, nil
}

func (s *Scanner) findProject(ctx context.Context, projectID string) (Project, error) {
	projects, err := s.Projects(ctx)
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("%w: %q", ErrUnknownProject, projectID)
}

// Branches lists local heads and marks the checked-out one.
func (s *Scanner) Branches(ctx context.Context, projectID string) ([]Branch, error) {
	if s.root == "" {
		return []Branch{}, nil
	}
	p, err := s.findProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out, err := s.git.Run(ctx, p.Path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("project: list branches of %s: %w", p.Name, err)
	}
	// A failure here is tolerated: a repository with no commits has no HEAD to
	// resolve, and losing the IsHead marker is better than losing the list.
	head := ""
	if raw, err := s.git.Run(ctx, p.Path, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		head = strings.TrimSpace(string(raw))
	}

	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	branches := make([]Branch, 0, len(lines))
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b := Branch{
			ID:        BranchID(p.ID, name),
			ProjectID: p.ID,
			Name:      name,
			IsHead:    name == head,
		}
		s.remember(b.ID, b.Name)
		branches = append(branches, b)
	}
	return branches, nil
}

// Worktrees lists the repository's worktrees. It reports what git reports and
// verifies nothing on disk: Path is where a listed-but-absent worktree is
// caught, because that is the call that is about to hand a path to a mount.
func (s *Scanner) Worktrees(ctx context.Context, projectID string) ([]Worktree, error) {
	if s.root == "" {
		return []Worktree{}, nil
	}
	p, err := s.findProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out, err := s.git.Run(ctx, p.Path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("project: list worktrees of %s: %w", p.Name, err)
	}
	return s.parseWorktrees(p, string(out)), nil
}

// parseWorktrees reads `git worktree list --porcelain`. Records are separated
// by a blank line and each begins with a "worktree <path>" line; git emits
// forward slashes on every platform, including Windows, so the path is cleaned
// to the OS separator before it is hashed — otherwise the derived id would not
// match the one Path computes from a real directory.
func (s *Scanner) parseWorktrees(p Project, out string) []Worktree {
	var res []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			res = append(res, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			path := filepath.Clean(strings.TrimPrefix(line, "worktree "))
			cur = &Worktree{
				ID:        WorktreeID(path),
				ProjectID: p.ID,
				Name:      filepath.Base(path),
				Path:      path,
				// git lists the main worktree first, always.
				IsPrimary: len(res) == 0,
			}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "branch refs/heads/"):
			name := strings.TrimPrefix(line, "branch refs/heads/")
			cur.BranchID = BranchID(p.ID, name)
			s.remember(cur.BranchID, name)
		case line == "detached" || line == "bare":
			// No branch. Inventing one would make the rail claim a branch that
			// does not exist.
			cur.BranchID = ""
		}
	}
	flush()
	return res
}

// Path implements worktree.Provider: it resolves a worktree id to an absolute,
// existing directory and never creates, moves or removes anything.
//
// A project whose `git worktree list` fails is skipped rather than failing the
// whole resolution — one broken checkout under the root must not make every
// other worktree unresolvable.
func (s *Scanner) Path(ctx context.Context, worktreeID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.root == "" {
		return "", fmt.Errorf("%w: %q (no projects directory is configured)", worktree.ErrUnknownID, worktreeID)
	}
	projects, err := s.Projects(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		worktrees, err := s.Worktrees(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, w := range worktrees {
			if w.ID == worktreeID {
				return worktree.Verify(w.Path)
			}
		}
	}
	return "", fmt.Errorf("%w: %q", worktree.ErrUnknownID, worktreeID)
}
