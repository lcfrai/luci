//go:build linux || darwin

package api

import (
	"net/http"
	"syscall"
	"testing"
	"time"
)

// Opening a named pipe for reading blocks until something writes to it. The
// source's store is the source agent's ~/.claude/projects, so an agent can
// make its transcript a FIFO; a fork that opened it would hang the create for
// good. The regular-file look refuses it before the open. Unix only: Windows
// has no FIFO in the filesystem.
func TestAForkOfATranscriptThatIsANamedPipeIsRefusedWithoutBlocking(t *testing.T) {
	env := forkNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	path := forkPath(env, "w-4a77b1c9", sessionsConversation)
	forkWrite(t, path+".dir-maker", nil)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot make a FIFO here: %v", err)
	}

	done := make(chan int, 1)
	go func() { done <- env.do(t, http.MethodPost, "/v1/sessions", forkBody("s-0000000a")).Code }()
	select {
	case code := <-done:
		if code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the create blocked opening a named pipe the agent put where its transcript goes")
	}
}
