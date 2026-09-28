package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// x3Junction makes a Windows junction (a reparse point that is NOT a symlink)
// or skips. A junction needs no privilege on Windows, unlike a symlink, so it
// is the weaker attacker's tool and the guard has to refuse it too. A security
// claim this host cannot exercise must not read as proven, so a host that will
// not make one skips rather than passes.
func x3Junction(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("a junction is a Windows reparse point")
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("this host will not make a junction at %s: %v: %s", link, err, out)
	}
}

// X-3 review I-1. The load-bearing admin guard is the Lstat, not Confine: with
// the Lstat gone, a .git/worktrees symlink whose target is INSIDE the worktree
// is ACCEPTED, because Confine resolves it to a place within the worktree. The
// one pre-existing attack test pointed its symlink OUTSIDE, which either guard
// catches, so it pinned neither. This one points INSIDE, so ONLY the Lstat
// refuses it: deleting the Lstat (review mutant MUT-A) turns this test red.
func TestAWorktreeAdminSymlinkInsideTheWorktreeStillFailsTheCreate(t *testing.T) {
	env := sandboxEnvNew(t, func(*Deps) {})
	admin := sandboxEnvPrimary(t, env.dir, false)
	decoy := filepath.Join(env.dir, "decoy")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatal(err)
	}
	x3Link(t, decoy, admin) // .git/worktrees -> <worktree>/decoy, inside the worktree

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("backend.Create was called %d times; an in-worktree symlink admin must fail the create, not mount %q read-only over the sandbox's own metadata path", n, decoy)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}

// X-3 fix round (this review's junction finding). A junction (mklink /J, no
// privilege on Windows) is a reparse point os.Lstat reports as irregular but
// NOT a symlink, and filepath.EvalSymlinks does not resolve it. Before the fix
// it passed the symlink-only Lstat AND Confine (which read the junction as its
// own in-worktree location, not its out-of-worktree target), so the host
// bind-mounted the target directory into the sandbox. Refusing os.ModeIrregular
// closes it; this test is red without that mask.
func TestAWorktreeAdminJunctionFailsTheCreateAndMountsNothing(t *testing.T) {
	env := sandboxEnvNew(t, func(*Deps) {})
	admin := sandboxEnvPrimary(t, env.dir, false)
	outside := t.TempDir()
	x3Junction(t, outside, admin) // .git/worktrees -> a directory outside the worktree

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("backend.Create was called %d times, and %q would have been mounted into that sandbox", n, outside)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}
