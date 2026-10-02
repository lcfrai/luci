package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/api"
)

// x1OwnTemp points the OS temp directory at one this test owns, so what
// hostGit makes there, and what a reaper removes, touches nothing else.
func x1OwnTemp(t *testing.T) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "os-temp")
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(v, tmp)
	}
	return tmp
}

// Review I-1: hostGit's directory is in the OS temp directory, and temp
// reapers (systemd-tmpfiles, macOS's /var/folders cleaner, Windows Storage
// Sense) remove entries there while the control plane runs. Every create and
// restart asks the line-ending question, so a directory made once at startup
// and gone later fails every create with a 500 until a restart. Removing
// every directory git has run in so far must not change the next answer.
func TestHostGitAnswersAfterAReaperRemovesItsDirectory(t *testing.T) {
	x1Host(t)
	tmp := x1OwnTemp(t)
	spy := &x1Spy{}
	host := x1HostGit(t, spy)
	ctx := context.Background()

	eol, err := gitLineEndingsResolverWith(host, "crlf")(ctx)
	if err != nil {
		t.Fatalf("line endings, before: %v", err)
	}
	files, err := gitGlobalFilesResolver(host)(ctx)
	if err != nil {
		t.Fatalf("global files, before: %v", err)
	}
	id, err := gitIdentityResolver("", "", host)(ctx)
	if err != nil {
		t.Fatalf("identity, before: %v", err)
	}

	// The reaper: every directory git ran in, all of them inside tmp.
	for _, c := range spy.calls {
		if rel, err := filepath.Rel(tmp, c.dir); err != nil || strings.HasPrefix(rel, "..") {
			t.Fatalf("git ran in %q, outside this test's temp directory %q", c.dir, tmp)
		}
		if err := os.RemoveAll(c.dir); err != nil {
			t.Fatal(err)
		}
	}

	if got, err := gitLineEndingsResolverWith(host, "crlf")(ctx); err != nil || got != eol {
		t.Errorf("line endings after the reaper: %#v, %v; want %#v as before", got, err, eol)
	}
	if got, err := gitGlobalFilesResolver(host)(ctx); err != nil || !reflect.DeepEqual(got, files) {
		t.Errorf("global files after the reaper: %#v, %v; want %#v as before", got, err, files)
	}
	if got, err := gitIdentityResolver("", "", host)(ctx); err != nil || got != id {
		t.Errorf("identity after the reaper: %#v, %v; want %#v as before", got, err, id)
	}
}

// A directory made for one git is removed when that git is done, so a
// control plane that answers thousands of creates leaves nothing in the OS
// temp directory, and a directory a reaper might take is never one in use.
func TestHostGitLeavesNothingInTheTempDirectory(t *testing.T) {
	x1Host(t)
	tmp := x1OwnTemp(t)
	host := x1HostGit(t, &x1Spy{})
	ctx := context.Background()

	if _, err := gitLineEndingsResolverWith(host, "crlf")(ctx); err != nil {
		t.Fatalf("line endings: %v", err)
	}
	if _, err := gitGlobalFilesResolver(host)(ctx); err != nil {
		t.Fatalf("global files: %v", err)
	}
	if _, err := gitIdentityResolver("", "", host)(ctx); err != nil {
		t.Fatalf("identity: %v", err)
	}
	left, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range left {
		t.Errorf("%s is still in the temp directory after every git finished", e.Name())
	}
}

// Each git gets a directory no other git had, at the same moment or before.
// A shared, reused name is one a concurrent create's cleanup removes from
// under a running git, and on a shared /tmp one another local user can make
// first, with a .git of their own in it.
func TestEveryHostGitRunsInADirectoryOfItsOwn(t *testing.T) {
	x1Host(t)
	x1OwnTemp(t)
	spy := &x1Spy{}
	host := x1HostGit(t, spy)
	ctx := context.Background()

	const creates = 6
	errs := make(chan error, creates)
	for i := 0; i < creates; i++ {
		go func() {
			_, err := gitLineEndingsResolverWith(host, "crlf")(ctx)
			errs <- err
		}()
	}
	for i := 0; i < creates; i++ {
		if err := <-errs; err != nil {
			t.Errorf("line endings, %d creates at once: %v", creates, err)
		}
	}
	seen := map[string]bool{}
	for _, c := range spy.calls {
		if seen[c.dir] {
			t.Errorf("git %q ran in %q, which another git ran in too", strings.Join(c.args, " "), c.dir)
		}
		seen[c.dir] = true
	}
}

// git ignores a GIT_CEILING_DIRECTORIES entry that is not absolute, so a
// relative temp directory must be made absolute before it is the ceiling.
// Here it is relative to the working directory, which is the agent's
// repository: with a relative ceiling git finds that repository, and the
// identity is the agent's user.name. On Windows os.TempDir already answers
// an absolute path for a relative TMP (the mutant without filepath.Abs
// passes here), so this guards Linux and macOS, where $TMPDIR is returned
// as it is.
func TestHostGitMakesARelativeTempDirectoryAbsolute(t *testing.T) {
	worktree, _, _ := x1Host(t)
	if err := os.MkdirAll(filepath.Join(worktree, "rel-tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(worktree); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	for _, v := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(v, "rel-tmp")
	}

	host := x1HostGit(t, &x1Spy{})
	got, err := gitIdentityResolver("", "", host)(context.Background())
	if want := (api.GitIdentity{Name: "Host Person", Email: "host@example.invalid"}); err != nil || got != want {
		t.Errorf("identity = %#v, %v; want the host's own %#v: git found the repository around a relative temp directory", got, err, want)
	}
}
