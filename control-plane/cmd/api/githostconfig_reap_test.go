package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if got, err := gitGlobalFilesResolver(host)(ctx); err != nil || got != files {
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
