package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// gitNeutral runs git in dir with the developer's own config neutralised, so
// hooks, signing and templates cannot change what the test builds.
func gitNeutral(t *testing.T, dir string, args ...string) {
	t.Helper()
	cfg := t.TempDir()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// longTempDir is t.TempDir() in the long form git prints. A worktree id is a
// hash of the path, and this host's temporary directory is an 8.3 name.
func longTempDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

var listeningAddr = regexp.MustCompile(`msg="control plane listening" addr=(\S+)`)

// waitListening returns the address serve logged it listens on.
func waitListening(t *testing.T, logs *lockedBuf, done <-chan error) string {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		if m := listeningAddr.FindStringSubmatch(logs.String()); m != nil {
			return m[1]
		}
		select {
		case err := <-done:
			t.Fatalf("serve returned (%v) before it listened; logs:\n%s", err, logs.String())
		case <-deadline:
			t.Fatalf("serve did not listen within 20s; logs:\n%s", logs.String())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// X-3 review 5 C-1, serve's half. newScanner's own test sees its body, not
// that serve builds the scanner with it: serve making its own, without the
// conversation stores' root, passed every other test (MUT-INLINE). So this
// runs serve itself over a fake Docker API. A real repository's real linked
// worktree is inside the stores' root, and it is moved out and its location
// swapped for a junction to where it now is, which git still lists. A session
// create on it must stop at contract §5.1 step 1 on the reparse point.
func TestServeRefusesAWorktreeSwappedInsideAConversationStore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	clearDockerEnv(t)
	state := longTempDir(t)
	writeInstanceID(t, state, fakeOurs)
	cfg := fakeDaemonConfig(t, state, newFakeDaemon(t, nil))
	cfg.ProjectsDir = longTempDir(t)
	app := filepath.Join(cfg.ProjectsDir, "app")
	loc := filepath.Join(cfg.ConversationsDir, "local", "wt-app")
	for _, d := range []string{app, filepath.Dir(loc)} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	gitNeutral(t, app, "init", "-q", "-b", "main")
	gitNeutral(t, app, "commit", "-q", "--allow-empty", "-m", "initial")
	gitNeutral(t, app, "worktree", "add", "-q", "--detach", loc)
	moved := filepath.Join(longTempDir(t), "wt-app")
	if err := os.Rename(loc, moved); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, moved, loc)

	logs := &lockedBuf{}
	log := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, log) }()
	addr := waitListening(t, logs, done)

	id := project.WorktreeID(loc)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("http://"+addr+"/v1/sessions", "application/json",
		strings.NewReader(`{"worktreeId":"`+id+`"}`))
	if err != nil {
		t.Fatalf("POST /v1/sessions: %v", err)
	}
	_ = resp.Body.Close()
	got := logs.String()
	if resp.StatusCode != http.StatusInternalServerError ||
		!strings.Contains(got, "api: resolve worktree "+id+" failed") ||
		!strings.Contains(got, hostpath.ErrReparsePoint.Error()) {
		t.Errorf("create on %s, swapped for a junction inside a conversation store = %d; want 500, refused at resolve on %q; logs:\n%s",
			loc, resp.StatusCode, hostpath.ErrReparsePoint, got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve = %v after cancel, want nil", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not return within 20s of its context being cancelled")
	}
}
