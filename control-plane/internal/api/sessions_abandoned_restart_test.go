package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// F1 on the OTHER path that mints a container.
//
// Both of F1's fixes were applied to handleSessionRestart as well as to
// handleSessionCreate — sessionCreateContext at sessions.go's restart Create,
// and destroyAfterFailedCreate on its failure — and neither was pinned there:
// deleting destroyAfterFailedCreate from restart left this package green.
// Restart is the more exposed of the two, because it runs on a record that was
// `exited` moments before, which is precisely the shape rule 3a exists for.
//
// The create-side twins of these two are in sessions_abandoned_create_test.go.

// Part 1: a restart's Docker calls do not run on the caller's context either.
func TestARestartFinishesAfterTheCockpitGivesUpOnIt(t *testing.T) {
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	// Same reasoning as the create twin: the create's budget derives from
	// ReadyTimeout, and 2 x 50 ms would end the create before this test can hold
	// it open. Two seconds leaves the caller's abort as the only thing that
	// could end it early, which is the whole point.
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 2 * time.Second
	})
	sessionsStubReady(t, nil)
	watch := sessionsWatchDone(t)
	sessionsSeed(t, st, sessionsStarting("s-0000000b"))

	ctx, abort := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodPost,
		sessionsTarget("/v1/sessions/s-0000000b/restart"), nil).WithContext(ctx)
	rec := httptest.NewRecorder()

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
	// The cockpit gives up at CREATE_TIMEOUT_MS, or the user closes the window:
	// either way the connection goes and r.Context() is cancelled.
	abort()
	select {
	case <-served:
		t.Fatal("the restart returned the moment the cockpit gave up; it ran on the caller's context (F1)")
	case <-time.After(250 * time.Millisecond):
	}
	close(be.hold)
	<-served

	if err := be.contextError(); err != nil {
		t.Errorf("backend.Create's context ended with %v; a restart must not run on the caller's", err)
	}
	s, err := st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Status == registry.StatusExited {
		t.Errorf("the record is exited with reason %q; the abandoned restart finished normally", s.ExitReason)
	}
	if s.Endpoint == "" {
		t.Error("the record carries no endpoint: the restart's second write never happened")
	}
	sessionsAwaitWatch(t, watch)
}

// Part 2: a restart whose create fails destroys the container that create made,
// not only the one the restart replaced. Without it the new container is
// nobody's until rule 3a — and the record naming it is already `exited`, which
// is the state every other reaper rule skips.
func TestARestartThatCannotCreateDestroysTheContainerItJustMade(t *testing.T) {
	env := sessionsNewEnv(t)
	before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	env.be.createErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}

	minted := env.be.spec(t, 0).ID
	if minted == before.SandboxID {
		t.Fatalf("the restart reused sandbox id %q; this test cannot tell the two destroys apart", minted)
	}
	var destroyedTheNewOne bool
	for _, id := range sessionsDestroyed(env.be) {
		if id == minted {
			destroyedTheNewOne = true
		}
	}
	if !destroyedTheNewOne {
		t.Errorf("destroyed %v, which does not include %q: the container the failed restart made is left behind",
			sessionsDestroyed(env.be), minted)
	}
}
