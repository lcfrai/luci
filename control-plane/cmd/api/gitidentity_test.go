package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// identityRunner is a project.Runner double: it records every git invocation
// and answers each one the same way.
type identityRunner struct {
	mu    sync.Mutex
	calls []identityCall
	out   string
	err   error
}

type identityCall struct {
	dir  string
	args []string
}

func (r *identityRunner) Run(_ context.Context, dir string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, identityCall{dir: dir, args: append([]string(nil), args...)})
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.out), nil
}

var _ project.Runner = (*identityRunner)(nil)

// identityHost is a hostGit whose git is r, in a directory hostGit makes for
// it in the OS temp directory.
func identityHost(r *identityRunner) hostGit {
	return hostGit{run: r, root: os.TempDir(), getenv: noXDG}
}

// The explicit pair wins outright, and "outright" is checked as the host never
// being asked — a resolver that asked and then preferred the variables would
// pass an equality check and still run git on every create.
func TestAnExplicitIdentityWinsAndTheHostIsNeverAsked(t *testing.T) {
	host := &identityRunner{out: "Someone Else <else@example.invalid> 1790156728 +1000\n"}
	resolve := gitIdentityResolver("Ada Lovelace", "ada@example.invalid", identityHost(host))

	got, err := resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitIdentity{Name: "Ada Lovelace", Email: "ada@example.invalid"}); got != want {
		t.Errorf("identity = %#v, want %#v", got, want)
	}
	if len(host.calls) != 0 {
		t.Errorf("the host's git was asked %#v with an explicit identity set", host.calls)
	}
}

// Unset, the answer is the host git's own, asked with git var in hostGit's
// directory, outside every repository and never in a worktree (X-1): the
// identity a commit would carry, whatever layer of git supplied it.
func TestWithNothingSetTheHostGitIsAskedOutsideAnyRepository(t *testing.T) {
	host := &identityRunner{out: "Dylan <au-dev-dmackie1@kpmg.com.au> 1790156728 +1000\n"}
	resolve := gitIdentityResolver("", "", identityHost(host))

	got, err := resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitIdentity{Name: "Dylan", Email: "au-dev-dmackie1@kpmg.com.au"}); got != want {
		t.Errorf("identity = %#v, want %#v", got, want)
	}
	if len(host.calls) != 1 {
		t.Fatalf("git was run %d times, want once: %#v", len(host.calls), host.calls)
	}
	if c := host.calls[0]; !madeByHostGit(c.dir) || strings.Join(c.args, " ") != "var GIT_AUTHOR_IDENT" {
		t.Errorf("git ran %q in %q, want \"var GIT_AUTHOR_IDENT\" in a directory hostGit made for it", strings.Join(c.args, " "), c.dir)
	}
}

// Half a pair is not an identity. loadConfig refuses one, so this is the
// resolver's own contract: it asks the host rather than commit as half a
// person.
func TestHalfAnExplicitPairIsNotAnIdentity(t *testing.T) {
	for _, pair := range [][2]string{{"Ada Lovelace", ""}, {"", "ada@example.invalid"}} {
		host := &identityRunner{out: "Dylan <d@example.invalid> 1790156728 +1000\n"}
		got, err := gitIdentityResolver(pair[0], pair[1], identityHost(host))(context.Background())
		if err != nil || got.Name != "Dylan" || len(host.calls) != 1 {
			t.Errorf("pair %q: identity %#v, err %v, host asked %d times; want the host's answer", pair, got, err, len(host.calls))
		}
	}
}

// Neither exists: the outcome is an error a user can act on. It names both
// fixes and carries git's own words, which say what git could not find.
func TestNoIdentityAnywhereIsASentenceNamingBothFixes(t *testing.T) {
	gitSaid := errors.New("git var GIT_AUTHOR_IDENT in /w: exit status 128: Author identity unknown\n\n*** Please tell me who you are.")
	resolve := gitIdentityResolver("", "", identityHost(&identityRunner{err: gitSaid}))

	got, err := resolve(context.Background())
	if err == nil {
		t.Fatalf("resolve returned %#v and no error with no identity anywhere", got)
	}
	if got != (api.GitIdentity{}) {
		t.Errorf("identity = %#v alongside an error, want none", got)
	}
	for _, want := range []string{envGitName, envGitEmail, "git config --global user.name", "Author identity unknown", "a commit in a sandbox will fail"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
}

// Output git var would never print still must not become an identity.
func TestAnIdentLineThatIsNotOneIsRefused(t *testing.T) {
	for line, want := range map[string]api.GitIdentity{
		"Dylan <d@example.invalid> 1790156728 +1000\n":   {Name: "Dylan", Email: "d@example.invalid"},
		"  Ada  Lovelace  < ada@x.invalid > 1 +0000":     {Name: "Ada  Lovelace", Email: "ada@x.invalid"},
		"Ada <ada@x.invalid> 1 +0000 trailing > bracket": {Name: "Ada", Email: "ada@x.invalid"},
		"":                         {},
		"Dylan":                    {},
		"<d@example.invalid> 1 +0": {},
		"Dylan <> 1 +0000":         {},
		"Dylan <d@example.invalid": {},
		"Dylan d@example.invalid>": {},
	} {
		got, ok := parseGitIdent(line)
		if ok != (want != api.GitIdentity{}) || got != want {
			t.Errorf("parseGitIdent(%q) = %#v, %v; want %#v", line, got, ok, want)
		}
	}
	resolve := gitIdentityResolver("", "", identityHost(&identityRunner{out: "not an ident\n"}))
	if _, err := resolve(context.Background()); err == nil || !strings.Contains(err.Error(), envGitName) {
		t.Errorf("an unparseable ident gave err %v, want the sentence naming %s", err, envGitName)
	}
}

// Against the real git, TestARepoLocalIdentityIsNotTheOneCommitsAreMadeAs
// (githostconfig_test.go) is the check: the host's global identity, and not a
// worktree's own user.name, which the agent writes. It replaced the test that
// held the opposite, that a repository's own identity was kept (X-1).
