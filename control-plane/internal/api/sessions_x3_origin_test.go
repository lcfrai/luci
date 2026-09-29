package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wantOriginRefused asserts that a create whose global-file value came from
// origin fails before anything is made: 500, no sandbox, nothing registered.
func wantOriginRefused(t *testing.T, base, excludes, origin string) {
	t.Helper()
	host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{Excludes: excludes, Origins: []string{origin}}}
	env := x3Env(t, base, func(d *Deps) { d.GitGlobalFiles = host.resolve })
	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for the origin %q: %.300s", rec.Code, origin, rec.Body.String())
	}
	if n := sandboxEnvCreates(env); n != 0 {
		t.Errorf("BYPASS: backend.Create was called %d times; the host file %q is mounted as core.excludesFile on the word of an origin nobody checked", n, excludes)
	}
	if list, err := env.st.List(); err != nil || len(list) != 0 {
		t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
	}
}

// secretFile is a host file outside the projects directory, the value an
// attacker's setting names; m-5's own check on the named path passes it.
func secretFile(t *testing.T) string {
	t.Helper()
	secret := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	return secret
}

// X-3 review 4 I-C, FAIL CLOSED: an origin the walk cannot follow to a file
// fails the create, not only one it proves inside the projects directory. git
// read that file an instant earlier, so a name that does not resolve means the
// check did not run. The first case is review 4's measured bypass: the string
// the resolver handed over at c62e220f for an in-project file git C-quoted
// (quotes kept, backslashes doubled) resolved to nothing, and the session
// logged it and mounted the value (201, one create).
func TestAGlobalFileOriginTheWalkCannotFollowFailsTheCreate(t *testing.T) {
	base := t.TempDir()
	b2 := filepath.Join(base, "dotfiles", "b2.inc")
	if err := os.MkdirAll(filepath.Dir(b2), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, origin := range map[string]string{
		"the quoted form git prints without -z": `"` + strings.ReplaceAll(b2, `\`, `\\`) + `"`,
		"a file that is gone":                   filepath.Join(t.TempDir(), "gone", ".gitconfig"),
		"a relative name":                       "gitconfig-global",
		"no name at all":                        "",
	} {
		t.Run(name, func(t *testing.T) {
			secret := secretFile(t)
			if err := os.WriteFile(b2, []byte("[core]\n\texcludesFile = "+filepath.ToSlash(secret)+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			wantOriginRefused(t, base, secret, origin)
		})
	}
}

// X-3 review 4, I-C's second route: the host's global config is a HARD link to
// a file in a dotfiles project. A hard link is a second name for the same file
// and needs no privilege on Windows; the agent edits the in-project name in
// place and the host's name reads the edit. The walk of the host's name passes
// no link and never touches the projects directory, so only the file's link
// count shows that another name exists — and where it is cannot be proved.
func TestAHardLinkedGlobalFileOriginFailsTheCreate(t *testing.T) {
	base := t.TempDir()
	secret := secretFile(t)
	inProject := filepath.Join(base, "dotfiles", ".gitconfig")
	if err := os.MkdirAll(filepath.Dir(inProject), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inProject, []byte("[core]\n\texcludesFile = "+filepath.ToSlash(secret)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hostName := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.Link(inProject, hostName); err != nil {
		t.Skipf("no hard link here: %v", err)
	}
	wantOriginRefused(t, base, secret, hostName)
}
