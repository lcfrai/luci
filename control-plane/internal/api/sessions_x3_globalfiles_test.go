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

// X-3 review 3 I-C. m-5 refuses a NAMED file reached through the projects
// directory; this refuses the provenance of the LINE that names it. --includes
// is on (X-1), so under m-5's own premise a dotfiles repository that is also a
// project holds the config file that SETS core.excludesFile, and it can name a
// host file far from the projects directory — one m-5's own check on the named
// path passes. The resolver reports that config file as the value's origin, and
// a create whose origin sits inside the projects directory must fail before
// anything is mounted.
func TestAGlobalFileWhoseSettingComesFromInsideTheProjectsDirectoryFailsTheCreate(t *testing.T) {
	base := t.TempDir()
	// The excludes value is a host file OUTSIDE the projects directory, so the
	// named-path check (m-5) would pass; only the origin catches this.
	named := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(named, []byte("*.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The config file that SET it lives inside a project under the projects dir.
	origin := filepath.Join(base, "dotfiles", ".gitconfig")
	if err := os.MkdirAll(filepath.Dir(origin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(origin, []byte("[core]\n\texcludesFile = "+filepath.ToSlash(named)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{Excludes: named, Origins: []string{origin}}}
	env := x3Env(t, base, func(d *Deps) { d.GitGlobalFiles = host.resolve })

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("backend.Create was called %d times; a setting from inside the projects directory must fail the create", n)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}

// The guard must not cost the normal case: an excludes value the host's own
// ~/.gitconfig sets, that file OUTSIDE the projects directory, still mounts.
func TestAGlobalFileSetOutsideTheProjectsDirectoryStillMounts(t *testing.T) {
	base := t.TempDir()
	named := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(named, []byte("*.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(origin, []byte("[core]\n\texcludesFile = "+filepath.ToSlash(named)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{Excludes: named, Origins: []string{origin}}}
	env := x3Env(t, base, func(d *Deps) { d.GitGlobalFiles = host.resolve })
	spec := sandboxEnvCreate(t, env, sessionsCreateBody)

	sandboxEnvWantGitFile(t, spec, "core.excludesFile", sessionGitExcludesPath, named, true)
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
