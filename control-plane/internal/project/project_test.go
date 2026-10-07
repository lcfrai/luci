package project_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// project.Scanner must satisfy the Phase 3 seam with no adapter; Phase 4's
// contextwrite.NewWriter takes it directly.
var _ worktree.Provider = (*project.Scanner)(nil)

type fakeGit struct {
	mu      sync.Mutex
	replies map[string]string
	fails   map[string]error
	calls   []string
}

func newFakeGit() *fakeGit {
	return &fakeGit{replies: map[string]string{}, fails: map[string]error{}}
}

func gitKey(dir string, args []string) string { return dir + "\x00" + strings.Join(args, " ") }

func (f *fakeGit) reply(dir, out string, args ...string) {
	f.replies[gitKey(dir, args)] = out
}

func (f *fakeGit) fail(dir string, err error, args ...string) {
	f.fails[gitKey(dir, args)] = err
}

func (f *fakeGit) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, strings.Join(args, " "))
	k := gitKey(dir, args)
	if err, ok := f.fails[k]; ok {
		return nil, err
	}
	out, ok := f.replies[k]
	if !ok {
		return nil, fmt.Errorf("fakeGit: no scripted reply for %q in %q", strings.Join(args, " "), dir)
	}
	return []byte(out), nil
}

func (f *fakeGit) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// makeRepo creates root/name with a .git directory in it. No git binary is
// involved: Scanner decides "is this a repo" from the presence of .git.
func makeRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	return dir
}

// An unconfigured projects directory is a normal state, not a failure: a fresh
// install has one, and the cockpit renders an empty rail rather than an error.
func TestAnUnconfiguredRootReturnsEmptyResultsAndNeverRunsGit(t *testing.T) {
	g := newFakeGit()
	s := project.NewScanner("", g)
	ctx := context.Background()

	projects, err := s.Projects(ctx)
	if err != nil || len(projects) != 0 {
		t.Fatalf("Projects() = %v, %v; want an empty slice and no error", projects, err)
	}
	branches, err := s.Branches(ctx, "p-4a77b1c9")
	if err != nil || len(branches) != 0 {
		t.Fatalf("Branches() = %v, %v; want an empty slice and no error", branches, err)
	}
	worktrees, err := s.Worktrees(ctx, "p-4a77b1c9")
	if err != nil || len(worktrees) != 0 {
		t.Fatalf("Worktrees() = %v, %v; want an empty slice and no error", worktrees, err)
	}
	if calls := g.recorded(); len(calls) != 0 {
		t.Fatalf("git ran %v with no root configured; there is nothing to enumerate", calls)
	}
}

// Depth-1 under root, and nothing else. Deeper recursion over a developer's
// whole disk is how a rail takes forty seconds to paint.
func TestProjectsFindsDepth1RepositoriesOnly(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "luci")
	makeRepo(t, root, "eunomia")
	// A linked worktree has .git as a FILE, not a directory. It is still a
	// repository as far as enumeration is concerned.
	linked := filepath.Join(root, "linked")
	if err := os.MkdirAll(linked, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A dot-prefixed directory holding .git is a repository like any other. It
	// is here to pin the absence of a hidden-name filter: the contract's rule
	// is .git, full stop, and a filter would drop a project the user put there
	// with no way to tell that it had been dropped.
	makeRepo(t, root, ".cache")
	// Not repositories, and must not be reported.
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeRepo(t, root, filepath.Join("nested", "deep")) // depth 2
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Name)
		if !filepath.IsAbs(p.Path) {
			t.Errorf("project %q has a relative path %q", p.Name, p.Path)
		}
		if !strings.HasPrefix(p.ID, "p-") || len(p.ID) != 10 {
			t.Errorf("project %q has id %q, want \"p-\" plus 8 hex characters", p.Name, p.ID)
		}
	}
	// os.ReadDir sorts by filename, and "." sorts before every letter.
	want := []string{".cache", "eunomia", "linked", "luci"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Projects() names = %v, want %v in that order", names, want)
	}
}

// Ids are derived, not stored: a restart produces the same ids with no storage
// and no migration, which is why registry.Session can hold a worktreeId across
// a reboot.
func TestProjectIdsAreDerivedFromThePathAndAreStable(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")

	first, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	second, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("two scanners produced different ids %q and %q for the same path", first[0].ID, second[0].ID)
	}
	if got := project.ProjectID(dir); got != first[0].ID {
		t.Fatalf("ProjectID(%q) = %q, want %q", dir, got, first[0].ID)
	}
	// Windows paths are case-insensitive, so the id is computed from the
	// lowercased path and C:\Users\x and c:\users\x must not be two projects.
	if got := project.ProjectID(strings.ToUpper(dir)); got != first[0].ID {
		t.Fatalf("ProjectID is case-sensitive: %q != %q", got, first[0].ID)
	}
}

func TestBranchesMarksTheCheckedOutBranch(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	g := newFakeGit()
	g.reply(dir, "main\nevidence-upload\nfeat/ready-flag\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "evidence-upload\n", "rev-parse", "--abbrev-ref", "HEAD")

	s := project.NewScanner(root, g)
	got, err := s.Branches(context.Background(), project.ProjectID(dir))
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Branches() = %v, want 3", got)
	}
	head := 0
	for _, b := range got {
		if b.IsHead {
			head++
			if b.Name != "evidence-upload" {
				t.Errorf("IsHead is set on %q, want evidence-upload", b.Name)
			}
		}
		if b.ProjectID != project.ProjectID(dir) {
			t.Errorf("branch %q carries projectId %q", b.Name, b.ProjectID)
		}
	}
	if head != 1 {
		t.Errorf("%d branches are marked IsHead, want exactly 1", head)
	}
	// The id is slugged; the display name is not touched.
	if got[2].Name != "feat/ready-flag" {
		t.Errorf("Branches()[2].Name = %q, want the real ref name %q", got[2].Name, "feat/ready-flag")
	}
	wantID := project.BranchID(project.ProjectID(dir), "feat/ready-flag")
	if got[2].ID != wantID || !strings.Contains(wantID, "-feat-ready-flag-") {
		t.Errorf("Branches()[2].ID = %q, want %q containing the slug", got[2].ID, wantID)
	}
}

// Fix round 1, M7: slug alone collapsed every run of non-[a-z0-9] to one
// dash, so these three names shared one BranchID within a project. Now the
// trailing digest of the raw name disambiguates them.
func TestBranchIDsAreDistinctForNamesThatSlugTheSame(t *testing.T) {
	pid := "p-4a77b1c9"
	names := []string{"feat/a-b", "feat/a/b", "Feat/A B"}
	seen := map[string]string{}
	for _, name := range names {
		id := project.BranchID(pid, name)
		if other, dup := seen[id]; dup {
			t.Fatalf("BranchID(%q) = %q, collides with BranchID(%q); names that slug the same must still get distinct ids", name, id, other)
		}
		seen[id] = name
	}
}

// Non-ASCII collapses harder than ASCII punctuation: both names lose their
// only distinguishing character to the slug's [a-z0-9] filter.
func TestBranchIDsAreDistinctForNonAsciiNamesThatSlugTheSame(t *testing.T) {
	pid := "p-4a77b1c9"
	a := project.BranchID(pid, "fix/é")
	b := project.BranchID(pid, "fix/ü")
	if a == b {
		t.Fatalf("BranchID(fix/é) = BranchID(fix/ü) = %q, want distinct ids", a)
	}
}

// Reproducible from the same two inputs with no storage: two independent
// calls for the same project and name must compute the same id.
func TestBranchIDIsStableAcrossIndependentCalls(t *testing.T) {
	pid := "p-4a77b1c9"
	first := project.BranchID(pid, "feat/ready-flag")
	second := project.BranchID(pid, "feat/ready-flag")
	if first != second {
		t.Fatalf("BranchID computed %q then %q for the same inputs", first, second)
	}
}

func TestBranchesOnAnUnknownProjectIsASentinelTheApiCanMapTo404(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "luci")

	_, err := project.NewScanner(root, newFakeGit()).Branches(context.Background(), "p-deadbeef")
	if !errors.Is(err, project.ErrUnknownProject) {
		t.Fatalf("Branches of an unknown project = %v, want an error wrapping ErrUnknownProject", err)
	}
}

// Mirrors TestBranchesOnAnUnknownProjectIsASentinelTheApiCanMapTo404. Both
// Branches and Worktrees funnel through findProject, but task 10 maps
// GET …/branches and GET …/worktrees to 404 project_not_found independently,
// so each needs its own regression test — an untested path is one edit away
// from becoming a 500.
func TestWorktreesOnAnUnknownProjectIsASentinelTheApiCanMapTo404(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "luci")

	_, err := project.NewScanner(root, newFakeGit()).Worktrees(context.Background(), "p-deadbeef")
	if !errors.Is(err, project.ErrUnknownProject) {
		t.Fatalf("Worktrees of an unknown project = %v, want an error wrapping ErrUnknownProject", err)
	}
}

// git emits forward slashes in porcelain output on every platform, including
// Windows. The scanner must clean them to the OS separator or every derived
// worktree id differs from the one Path computes from a real directory.
func worktreePorcelain(dirs ...string) string {
	var b strings.Builder
	for i, d := range dirs {
		fmt.Fprintf(&b, "worktree %s\n", filepath.ToSlash(d))
		fmt.Fprintf(&b, "HEAD %040d\n", i)
		fmt.Fprintf(&b, "branch refs/heads/wt-%d\n", i)
		if i == 2 {
			// git emits either a branch line or "detached", never both. Record
			// 2 carries both on purpose: with only the realistic shape, a
			// parser that ignores the marker entirely still reports an empty
			// BranchID, so the test could not tell a parser that handles a
			// detached HEAD from one that has never heard of it.
			b.WriteString("detached\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestWorktreesParsesPorcelainOutput(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	detached := filepath.Join(t.TempDir(), "spike")
	for _, d := range []string{linked, detached} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked, detached), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Worktrees(context.Background(), project.ProjectID(dir))
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Worktrees() = %#v, want 3 records", got)
	}
	if !got[0].IsPrimary {
		t.Error("the first record is not marked primary; git lists the main worktree first")
	}
	if got[1].IsPrimary || got[2].IsPrimary {
		t.Error("more than one worktree is marked primary")
	}
	if got[1].Name != "evidence-upload" || got[1].Path != linked {
		t.Errorf("Worktrees()[1] = {Name:%q Path:%q}, want {evidence-upload %q}", got[1].Name, got[1].Path, linked)
	}
	if got[1].BranchID != project.BranchID(project.ProjectID(dir), "wt-1") {
		t.Errorf("Worktrees()[1].BranchID = %q, want the derived id for wt-1", got[1].BranchID)
	}
	// A detached HEAD belongs to no branch. Inventing one would make the rail
	// claim a branch that does not exist, and the fixture puts a branch line in
	// that record so only a parser that acts on "detached" can clear it.
	if got[2].BranchID != "" {
		t.Errorf("a detached worktree carries BranchID %q, want empty", got[2].BranchID)
	}
	if got[2].ID != project.WorktreeID(detached) {
		t.Errorf("Worktrees()[2].ID = %q, want %q", got[2].ID, project.WorktreeID(detached))
	}
}

// AMENDED by X-3 (X-1 review O-1): the fixture now REGISTERS the linked
// worktree under the project's own .git/worktrees, the way git does, because
// that registry — and not `git worktree list`, which an agent steers through
// .git/commondir — is what Path believes. Its place is unchanged, outside the
// projects root, which is where the e2e fixture puts one too.
func TestPathResolvesAWorktreeIdThroughTheProvider(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	x3Register(t, dir, linked)
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked), "worktree", "list", "--porcelain")
	s := project.NewScanner(root, g)

	got, err := s.Path(context.Background(), project.WorktreeID(linked))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q", got, linked)
	}

	if _, err := s.Path(context.Background(), "w-deadbeef"); !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path of an unknown id = %v, want an error wrapping worktree.ErrUnknownID", err)
	}
}

// git's metadata can outlive the directory: delete a worktree folder by hand
// and `git worktree list` still reports it until someone prunes. Resolving it
// must fail here, not inside Docker's bind-mount validation.
func TestPathOnAWorktreeGitStillListsButDiskHasLostIsErrNotADir(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	ghost := filepath.Join(t.TempDir(), "pruned-by-hand")
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, ghost), "worktree", "list", "--porcelain")

	_, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(ghost))
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a listed-but-absent worktree = %v, want an error wrapping worktree.ErrNotADir", err)
	}
}

// Phase 4's Naming contract: never an error, and fall back to the id, because
// a missing display name must not fail a brief. The cache warms as a side
// effect of enumeration, so a cold scanner returns the id and a warm one
// returns the name.
func TestNamesFallBackToTheIdUntilEnumerationHasWarmedTheCache(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	pid := project.ProjectID(dir)
	bid := project.BranchID(pid, "feat/ready-flag")

	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "main\n", "rev-parse", "--abbrev-ref", "HEAD")
	s := project.NewScanner(root, g)

	if got := s.ProjectName(pid); got != pid {
		t.Errorf("cold ProjectName = %q, want the id %q", got, pid)
	}
	if got := s.BranchName(bid); got != bid {
		t.Errorf("cold BranchName = %q, want the id %q", got, bid)
	}
	if got := s.ProjectName("p-deadbeef"); got != "p-deadbeef" {
		t.Errorf("ProjectName of an unknown id = %q, want the id back", got)
	}

	if _, err := s.Projects(context.Background()); err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if _, err := s.Branches(context.Background(), pid); err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if got := s.ProjectName(pid); got != "luci" {
		t.Errorf("warm ProjectName = %q, want %q", got, "luci")
	}
	if got := s.BranchName(bid); got != "feat/ready-flag" {
		t.Errorf("warm BranchName = %q, want %q", got, "feat/ready-flag")
	}
}

// Phase 2 owns resolution and read-only enumeration; docs/PROGRAM.md phase 3
// owns worktree lifecycle. This is the cheap mechanical guard: a whitelist over
// every git invocation the package makes.
func TestEveryGitCommandIsReadOnly(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	pid := project.ProjectID(dir)
	g := newFakeGit()
	g.reply(dir, "main\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "main\n", "rev-parse", "--abbrev-ref", "HEAD")
	g.reply(dir, worktreePorcelain(dir), "worktree", "list", "--porcelain")

	s := project.NewScanner(root, g)
	ctx := context.Background()
	if _, err := s.Projects(ctx); err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if _, err := s.Branches(ctx, pid); err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if _, err := s.Worktrees(ctx, pid); err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if _, err := s.Path(ctx, project.WorktreeID(dir)); err != nil {
		t.Fatalf("Path: %v", err)
	}

	allowed := map[string]bool{
		"for-each-ref --format=%(refname:short) refs/heads": true,
		"rev-parse --abbrev-ref HEAD":                       true,
		// Only on the failure path: the unborn-HEAD confirmation (F4).
		"rev-parse --verify --quiet HEAD":  true,
		"worktree list --porcelain":        true,
		"symbolic-ref --quiet HEAD":        true,
		"check-ref-format refs/heads/main": true,
	}
	calls := g.recorded()
	if len(calls) == 0 {
		t.Fatal("no git commands were recorded; the fake is not wired to the scanner")
	}
	for _, c := range calls {
		if !allowed[c] {
			t.Errorf("git %s is not on the read-only whitelist; Phase 3 owns anything that writes", c)
		}
	}
}

// A git failure must name git's own message, or the rail shows "exit status
// 128" and the user has nothing to act on.
func TestAGitFailureCarriesTheMessageUpward(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	g := newFakeGit()
	g.fail(dir, errors.New("fatal: not a git repository"), "for-each-ref", "--format=%(refname:short)", "refs/heads")

	_, err := project.NewScanner(root, g).Branches(context.Background(), project.ProjectID(dir))
	if err == nil {
		t.Fatal("Branches on a failing git returned no error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %q, want git's own message in it", err)
	}
}
