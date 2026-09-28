package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// I-4: the second write of a create — the one that records the endpoint the
// backend just handed back — had no precondition, while watchSessionReady's
// write in the same file has had one since task 13. F1 part 1 widened the
// window from "whatever was in flight when the caller aborted" to the whole
// create grace, because the create now runs to completion on a detached
// context. Anything the user does to the record in that window is overwritten.
//
// The state that costs the most is `exited` + an endpoint: it contradicts Go
// contract §5.3's exited invariant, it is permanent (markExited refuses an
// already-exited record and rule 3a leaves the record alone by design), and it
// defeats handleSessionDelete's alreadyTerminal test, whose comment says that
// test exists to stop a daemon outage trapping a record for ever.
//
// Both tests do exactly what handleSessionDelete does to the record — exited,
// a reason, no endpoint, no token — while backend.Create is held open.

// killTheRecord is handleSessionDelete's write, issued from a test while a
// create is in flight. Transcribed from sessions.go's kill path rather than
// paraphrased, because the point of the test is that this exact state survives.
func killTheRecord(t *testing.T, st registry.Store, id string) {
	t.Helper()
	if _, err := st.Update(id, func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		sess.ExitReason = "killed"
		sess.Endpoint = ""
		sess.AttachToken = ""
		return nil
	}); err != nil {
		t.Fatalf("kill %s: %v", id, err)
	}
}

// assertTerminalRecordSurvived is Go contract §5.3's exited invariant, read
// back from the store after the detached create has finished.
func assertTerminalRecordSurvived(t *testing.T, st registry.Store, id string) {
	t.Helper()
	s, err := st.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	if s.Status != registry.StatusExited || s.ExitReason != "killed" {
		t.Errorf("record is %q/%q, want exited/killed: the detached create wrote over the kill",
			s.Status, s.ExitReason)
	}
	if s.Endpoint != "" {
		t.Errorf("record carries endpoint %q after it went terminal; §5.3 says an exited session has none, "+
			"and handleSessionDelete's alreadyTerminal test needs it empty to purge without re-issuing Destroy",
			s.Endpoint)
	}
	if s.AttachToken != "" {
		t.Errorf("record carries an attach token after it went terminal")
	}
}

// destroyed reports what the fake was asked to remove, under its own lock.
func (b *abandonedCreate) destroyed() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.destroys...)
}

// assertDestroyed fails unless the container the create just made was reclaimed
// by the handler. Rule 3a is the backstop, not the plan: the record the rule
// keys on may have been purged by then, and a container nothing names is rule
// 4's problem a whole create grace later.
func assertDestroyed(t *testing.T, got []string, want string) {
	t.Helper()
	for _, id := range got {
		if id == want {
			return
		}
	}
	t.Errorf("destroyed %v, which does not include %q: the container the abandoned create made is left behind",
		got, want)
}

// Restart first: it is the more exposed path, because it runs on a record that
// was `exited` moments before, and before this test it answered 200 carrying the
// body it had just written — a restart that restarted nothing, reporting
// success with `status: "exited"` in it.
func TestARestartWritesNoEndpointOntoARecordThatWentTerminal(t *testing.T) {
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 2 * time.Second
	})
	sessionsStubReady(t, nil)
	sessionsSeed(t, st, sessionsStarting("s-0000000b"))

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, sessionsTarget("/v1/sessions/s-0000000b/restart"), nil)
	served := make(chan struct{})
	go func() {
		defer close(served)
		env.mux.ServeHTTP(rec, r)
	}()

	select {
	case <-be.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler never reached backend.Create")
	}
	// The restart's first write has already landed, so the record names the
	// container Create is making right now.
	held, err := st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	minted := held.SandboxID
	if minted == "" {
		t.Fatal("the restart's first write never landed")
	}
	killTheRecord(t, st, "s-0000000b")
	close(be.hold)
	<-served

	assertTerminalRecordSurvived(t, st, "s-0000000b")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: the session was ended while this restart's sandbox was being made, "+
			"so the restart restarted nothing and must not answer success", rec.Code)
	}
	assertDestroyed(t, be.destroyed(), minted)
}

// Create's own second write, same shape. The window is the whole create grace
// (120 s by default), not the instant of the abort, because of F1 part 1.
func TestACreateWritesNoEndpointOntoARecordThatWentTerminal(t *testing.T) {
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 2 * time.Second
	})
	sessionsStubReady(t, nil)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, sessionsTarget("/v1/sessions"),
		strings.NewReader(sessionsCreateBody)).WithContext(context.Background())
	r.Header.Set("Content-Type", "application/json")
	served := make(chan struct{})
	go func() {
		defer close(served)
		env.mux.ServeHTTP(rec, r)
	}()

	select {
	case <-be.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler never reached backend.Create")
	}
	sessions, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d session records, want 1: the record is written before the sandbox", len(sessions))
	}
	id, minted := sessions[0].ID, sessions[0].SandboxID
	killTheRecord(t, st, id)
	close(be.hold)
	<-served

	assertTerminalRecordSurvived(t, st, id)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: the session was killed while its sandbox was being made", rec.Code)
	}
	assertDestroyed(t, be.destroyed(), minted)
}
