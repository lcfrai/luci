package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// x3Link makes a symlink or skips. A security claim this host cannot exercise
// must not read as proven.
func x3Link(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform will not make a symlink at %s: %v", link, err)
	}
}

// x3Env is sandboxEnvNew with a projects directory the test knows, because the
// attack is about where a path sits relative to it.
func x3Env(t *testing.T, base string, tweak func(*Deps)) *sessionsEnv {
	t.Helper()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, base, st, st, func(d *Deps) {
		d.ProjectsDir = base
		tweak(d)
	})
	env.st = st
	return env
}

// X-1 review m-5, with review-2 m-4's likelier case. A dotfile manager has
// linked ~/.config/git/ignore into a dotfiles repository, that repository is
// under the projects directory, and an agent running in it swapped the file
// for a link to a key. Both ends of the chain are outside the projects
// directory, so the resolved path — what the create mounted before X-3 — is
// the key itself.
func TestAGlobalGitFileReachedThroughTheProjectsDirectoryFailsTheCreate(t *testing.T) {
	base := t.TempDir()
	secret := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Inside the worktree this session mounts read-write, which is what makes
	// it the agent's to swap.
	hop := filepath.Join(base, "evidence-upload", "dotfiles", "gitignore")
	x3Link(t, secret, hop)
	named := filepath.Join(t.TempDir(), "config", "git", "ignore")
	x3Link(t, hop, named)

	// The attack, before the guard: this is the path the mount was built from.
	resolved, err := filepath.EvalSymlinks(named)
	if err != nil {
		t.Fatal(err)
	}
	if want, err := filepath.EvalSymlinks(secret); err != nil || resolved != want {
		t.Fatalf("the attack is not set up: %q resolves to %q, want the key at %q (%v)", named, resolved, want, err)
	}

	host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{Excludes: named}}
	env := x3Env(t, base, func(d *Deps) { d.GitGlobalFiles = host.resolve })

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("backend.Create was called %d times, and the key would have been mounted into that sandbox", n)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}

// The case the guard must not cost, and the reason it walks rather than
// refuses links outright: a dotfile manager's link chain that never touches
// the projects directory still resolves, and the real file is what is mounted.
func TestAGlobalGitFileLinkedOutsideTheProjectsDirectoryStillMounts(t *testing.T) {
	base := t.TempDir()
	elsewhere := t.TempDir()
	real := filepath.Join(elsewhere, "dotfiles", "gitignore")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("**/.claude/settings.local.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(elsewhere, "config", "git", "ignore")
	x3Link(t, real, named)

	host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{Excludes: named}}
	env := x3Env(t, base, func(d *Deps) { d.GitGlobalFiles = host.resolve })
	spec := sandboxEnvCreate(t, env, sessionsCreateBody)

	sandboxEnvWantGitFile(t, spec, "core.excludesFile", sessionGitExcludesPath, named, true)
}
