package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These are deliberately fake opaque OAuth values, never live provider calls.
const codexFixtureAuth = `{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"fixture-access-original","refresh_token":"fixture-refresh-original"}}`
const codexFixtureRotated = `{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"fixture-access-rotated","refresh_token":"fixture-refresh-rotated"}}`

func codexTestEnv(t *testing.T) *sessionsEnv {
	e := sessionsNewEnv(t)
	e.api.registerCodexConnection(e.mux)
	return e
}

func codexTestSignIn(t *testing.T, e *sessionsEnv, done <-chan string) (sessionJSON, string) {
	t.Helper()
	s := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","authOnly":true}`), 201)
	sessionsAwaitWatch(t, done)
	if !s.AuthOnly || s.AuthMode != "chatgpt" {
		t.Fatal("sign-in metadata is absent")
	}
	record, _ := e.api.deps.Store.Get(s.ID)
	dir, err := e.api.nativeStore(record.WorktreeID, record.Harness, record.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, dir, "auth.json", codexFixtureAuth)
	sessionsDecodeOne(t, e.do(t, "DELETE", "/v1/sessions/"+s.ID, ""), 200)
	return s, dir
}

func codexTestAdopt(t *testing.T, e *sessionsEnv, id string) {
	t.Helper()
	r := e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+id+`"}`)
	if r.Code != 200 {
		t.Fatalf("adopt: %d %s", r.Code, r.Body.String())
	}
	var status codexAccountJSON
	if json.Unmarshal(r.Body.Bytes(), &status) != nil || !status.Configured || status.SourceSessionID != id {
		t.Fatal("missing saved connection metadata")
	}
	if strings.Contains(r.Body.String(), "fixture-") || strings.Contains(r.Body.String(), "tokens") {
		t.Fatal("adoption exposed credentials")
	}
}

func TestCodexAccountFreshSessionRotatesRestartPersistsAndNeverUsesAPIKey(t *testing.T) {
	e := codexTestEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	p, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: e.dir})
	if err != nil {
		t.Fatal(err)
	}
	e.api.deps.Product = p
	p.Register(e.mux)
	key := "fixture-api-key-must-never-replace-chatgpt"
	r := e.do(t, "POST", "/v1/setup/agents/openai", `{"apiKey":"`+key+`"}`)
	if r.Code != 200 {
		t.Fatalf("save fixture API key: %d", r.Code)
	}
	signIn, signInDir := codexTestSignIn(t, e, done)
	codexTestAdopt(t, e, signIn.ID)
	initialStatus := e.do(t, "GET", codexAccountRoute, "")
	if strings.Contains(initialStatus.Body.String(), "activeSessionId") {
		t.Fatal("the saved, stopped sign-in was reported as an active session")
	}
	first := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex"}`), 201)
	sessionsAwaitWatch(t, done)
	spec := e.be.spec(t, 1)
	dir := nativeStateMount(t, spec)
	if dir == signInDir || spec.Env["OPENAI_API_KEY"] != "" || strings.Contains(strings.Join(spec.Args, " "), "luci-openai") || !slices.Contains(spec.Args, `forced_login_method="chatgpt"`) || first.AuthMode != "chatgpt" || first.AuthOnly {
		t.Fatal("fresh subscription execution did not preserve its selected authentication")
	}
	auth, err := os.ReadFile(filepath.Join(dir, "auth.json"))
	if err != nil || string(auth) != codexFixtureAuth {
		t.Fatal("fresh project session lost the saved sign-in")
	}
	nativeWrite(t, dir, "auth.json", codexFixtureRotated)
	sessionsDecodeOne(t, e.do(t, "DELETE", "/v1/sessions/"+first.ID, ""), 200)
	beforeRetry, err := e.api.readCodexConnection()
	if err != nil {
		t.Fatal(err)
	}
	codexTestAdopt(t, e, signIn.ID)
	afterRetry, err := e.api.readCodexConnection()
	if err != nil || afterRetry.ID != beforeRetry.ID || string(afterRetry.Auth) != codexFixtureRotated {
		t.Fatal("retrying save replaced the rotated credential with its stale sign-in source")
	}
	// Reconstruct the API to prove this is durable state, not process memory.
	reopened, err := New(e.api.deps)
	if err != nil {
		t.Fatal(err)
	}
	e.api = reopened
	e.mux = newRoutes()
	reopened.registerSessions(e.mux)
	reopened.registerCodexConnection(e.mux)
	second := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex"}`), 201)
	sessionsAwaitWatch(t, done)
	secondDir := nativeStateMount(t, e.be.spec(t, 2))
	auth, _ = os.ReadFile(filepath.Join(secondDir, "auth.json"))
	if string(auth) != codexFixtureRotated || secondDir == dir {
		t.Fatal("a new process failed to use the rotated credential in a separate home")
	}
	sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions/"+second.ID+"/restart", `{}`), 200)
	sessionsAwaitWatch(t, done)
	restarted := e.be.spec(t, 3)
	if restarted.Env["OPENAI_API_KEY"] != "" || nativeStateMount(t, restarted) == secondDir {
		t.Fatal("restart switched billing or shared a writable home")
	}
	listed := e.do(t, "GET", "/v1/sessions", "")
	status := e.do(t, "GET", codexAccountRoute, "")
	if !strings.Contains(status.Body.String(), second.ID) {
		t.Fatal("active subscription lease was not reported")
	}
	for _, body := range []string{listed.Body.String(), status.Body.String(), strings.Join(restarted.Args, " ")} {
		if strings.Contains(body, "fixture-") || strings.Contains(body, "codexConnectionId") {
			t.Fatal("secret or private connection identity in public evidence")
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(e.api.codexConnectionPath())
		if info.Mode().Perm() != 0600 {
			t.Fatal("retained credential is not owner-only")
		}
	}
}

func TestCodexAccountOwnedStoppedAdoptionAndUserIsolation(t *testing.T) {
	e := codexTestEnv(t)
	other := codexTestEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	before := e.do(t, "GET", codexAccountRoute, "")
	if before.Code != 200 || strings.Contains(before.Body.String(), `"configured":true`) {
		t.Fatal("empty account status")
	}
	if _, err := os.Stat(e.conversations); !os.IsNotExist(err) {
		t.Fatal("GET created private state")
	}
	s := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","authOnly":true}`), 201)
	sessionsAwaitWatch(t, done)
	r := e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`)
	if sessionsProblemCode(t, r, 409) != "codex_account_busy" {
		t.Fatal("running sign-in adopted")
	}
	sessionsProblemCode(t, other.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`), 404)
	sessionsDecodeOne(t, e.do(t, "DELETE", "/v1/sessions/"+s.ID, ""), 200)
	dir := nativeStateMount(t, e.be.spec(t, 0))
	nativeWrite(t, dir, "auth.json", `{"auth_mode":"apikey","OPENAI_API_KEY":"fixture-not-a-subscription"}`)
	sessionsProblemCode(t, e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`), 409)
	nativeWrite(t, dir, "auth.json", codexFixtureAuth)
	codexTestAdopt(t, e, s.ID)
	if other.do(t, "GET", codexAccountRoute, "").Body.String() != before.Body.String() {
		t.Fatal("a different user's API sees saved sign-in")
	}
	foreign := sessionsDecodeOne(t, other.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex"}`), 201)
	sessionsAwaitWatch(t, done)
	if foreign.AuthMode != "" {
		t.Fatal("a different user inherited subscription selection")
	}
	if _, err := os.Stat(filepath.Join(nativeStateMount(t, other.be.spec(t, 0)), "auth.json")); !os.IsNotExist(err) {
		t.Fatal("credential crossed the user boundary")
	}
}

func TestCodexAccountConcurrentStartHasOneLeaseAndDisconnectCannotResurrectIt(t *testing.T) {
	e := codexTestEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	s, _ := codexTestSignIn(t, e, done)
	codexTestAdopt(t, e, s.ID)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex"}`).Code
		}()
	}
	wg.Wait()
	close(codes)
	var results []int
	for c := range codes {
		results = append(results, c)
	}
	slices.Sort(results)
	if !slices.Equal(results, []int{201, 409}) {
		t.Fatalf("overlapping starts: %v", results)
	}
	sessionsAwaitWatch(t, done)
	c, err := e.api.readCodexConnection()
	if err != nil || c == nil {
		t.Fatal("no retained lease")
	}
	active := c.ActiveSessionID
	if r := e.do(t, "DELETE", codexAccountRoute, ""); r.Code != 200 {
		t.Fatal("disconnect failed")
	}
	if _, err := os.Stat(e.api.codexConnectionPath()); !os.IsNotExist(err) {
		t.Fatal("disconnect retained canonical credentials")
	}
	if code := sessionsProblemCode(t, e.do(t, "POST", "/v1/sessions/"+active+"/restart", `{}`), 409); code != "codex_account_required" {
		t.Fatal("disconnected session restarted with another credential")
	}
	// Forget after disconnect still works and cannot recreate the connection.
	if r := e.do(t, "DELETE", "/v1/sessions/"+active+"?purge=true", ""); r.Code != 204 {
		t.Fatalf("forget: %d", r.Code)
	}
	if c, err := e.api.readCodexConnection(); err != nil || c != nil {
		t.Fatal("stopped session resurrected disconnected credentials")
	}
}

func TestCodexAccountStoppedForkUsesLatestCredentialAndSeparateHistory(t *testing.T) {
	e := codexTestEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	s, _ := codexTestSignIn(t, e, done)
	codexTestAdopt(t, e, s.ID)
	first := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex"}`), 201)
	sessionsAwaitWatch(t, done)
	dir := nativeStateMount(t, e.be.spec(t, 1))
	nativeWrite(t, dir, "sessions/2026/rollout.jsonl", "fixture prior conversation")
	nativeWrite(t, dir, "auth.json", codexFixtureRotated)
	sessionsDecodeOne(t, e.do(t, "DELETE", "/v1/sessions/"+first.ID, ""), 200)
	fork := sessionsDecodeOne(t, e.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","resume":"`+first.ID+`"}`), 201)
	sessionsAwaitWatch(t, done)
	forkDir := nativeStateMount(t, e.be.spec(t, 2))
	auth, _ := os.ReadFile(filepath.Join(forkDir, "auth.json"))
	history, _ := os.ReadFile(filepath.Join(forkDir, "sessions/2026/rollout.jsonl"))
	if fork.AuthMode != "chatgpt" || forkDir == dir || string(auth) != codexFixtureRotated || string(history) != "fixture prior conversation" {
		t.Fatal("fork lost selected account or conversation snapshot")
	}
	// Purging an older source must not overwrite the active fork's new token.
	if r := e.do(t, "DELETE", "/v1/sessions/"+first.ID+"?purge=true", ""); r.Code != 204 {
		t.Fatal("purge prior source")
	}
	c, _ := e.api.readCodexConnection()
	if c.ActiveSessionID != fork.ID {
		t.Fatal("old source changed the active credential lease")
	}
}

func TestCodexAccountRejectsHardLinkedCredentialAndNonSignInSession(t *testing.T) {
	e := codexTestEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	s, dir := codexTestSignIn(t, e, done)
	path := filepath.Join(dir, "auth.json")
	outside := filepath.Join(t.TempDir(), "credential.json")
	if err := os.Link(path, outside); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	sessionsProblemCode(t, e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`), 409)
	if err := os.Remove(outside); err != nil {
		t.Fatal(err)
	}
	_, _ = e.st.Update(s.ID, func(s *registry.Session) error { s.AuthOnly = false; return nil })
	sessionsProblemCode(t, e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`), 409)
	if _, err := os.Stat(e.api.codexConnectionPath()); !os.IsNotExist(err) {
		t.Fatal("rejected adoption wrote credentials")
	}
}
