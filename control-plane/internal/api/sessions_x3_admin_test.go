package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// X-1 review O-2. sessionWorktreeAdmin Lstatted .git and stopped there;
// .git/worktrees went straight to os.MkdirAll, which Stats, follows a link and
// answers nil for a directory. The agent writes this worktree, so it plants
// the link, and the host then bind-mounted whatever it named into the sandbox.
func TestAWorktreeAdminLinkFailsTheCreateAndMountsNothing(t *testing.T) {
	env := sandboxEnvNew(t, func(*Deps) {})
	admin := sandboxEnvPrimary(t, env.dir, false)
	elsewhere := t.TempDir()
	x3Link(t, elsewhere, admin)

	// The attack, before the guard: MkdirAll is content with the link, so the
	// mount source was elsewhere.
	if err := os.MkdirAll(admin, 0o777); err != nil {
		t.Fatalf("the attack is not set up: MkdirAll over the link failed: %v", err)
	}

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("backend.Create was called %d times, and %q would have been mounted into that sandbox", n, elsewhere)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}

// A second refusal, by Confine rather than by the Lstat: a .git/worktrees whose
// real location leaves the worktree. AMENDED by the X-3 fix round (review I-1):
// this pins Confine for the OUT-of-worktree case only and does NOT stand in for
// the Lstat. An in-worktree symlink defeats Confine (its target is inside the
// worktree) and a junction defeats it on Windows (EvalSymlinks does not resolve
// one), so the Lstat is the load-bearing guard — pinned by the tests in
// sessions_x3_admin_pin_test.go.
func TestAWorktreeAdminThatLeavesTheWorktreeNamesTheWorktreeInTheRefusal(t *testing.T) {
	env := sandboxEnvNew(t, func(*Deps) {})
	admin := sandboxEnvPrimary(t, env.dir, false)
	x3Link(t, t.TempDir(), admin)

	if _, err := sessionWorktreeAdmin(env.dir); err == nil {
		t.Fatal("sessionWorktreeAdmin accepted a link out of the worktree")
	} else if !strings.Contains(err.Error(), admin) {
		t.Fatalf("the refusal is %q, and it must name %q so the user can delete it", err, admin)
	}
}

// The legitimate case, unchanged: a real .git/worktrees is made if it is not
// there and mounted read-only over itself, so the sandbox's git can read the
// linked worktrees' metadata and cannot prune the host's.
func TestARealWorktreeAdminDirectoryIsStillMadeAndMountedReadOnly(t *testing.T) {
	env := sandboxEnvNew(t, func(*Deps) {})
	admin := sandboxEnvPrimary(t, env.dir, true)

	spec := sandboxEnvCreate(t, env, sessionsCreateBody)

	got := sandboxEnvAdminMounts(spec)
	if len(got) != 1 || got[0].HostPath != admin || !got[0].ReadOnly {
		t.Fatalf("mounts at %s = %#v, want exactly %q, read-only", sandboxEnvAdminDir, got, admin)
	}
	if info, err := os.Lstat(filepath.Join(admin, "linked", "gitdir")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("the linked worktree's metadata is gone: %v", err)
	}
}
