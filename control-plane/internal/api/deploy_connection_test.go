package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
)

// The address every row connects to, and the key it pastes. The key is not a
// real one and never leaves this file: the point of the tests below is that it
// does not leave the control plane either.
const (
	connTestOrigin = "https://dev.panacea.kpmg"
	connTestToken  = "pnc_abcdef01234567890123456789abcdef"
	connTestTarget = "PanaceaApp/panacea/dev.panacea.kpmg"
)

// errConnNotFound stands in for credstore.ErrNotFound. connection.Config is
// declared against a three-method Secrets subset so phase 7 compiles before Hub
// task C1 lands, and the "is this a not-found" question comes in beside it.
var errConnNotFound = errors.New("secrets: not found")

// connSecrets is a credential store in a map, and the assertion surface for the
// one thing this task must never do: store a key that was not proved first.
type connSecrets struct {
	mu   sync.Mutex
	rows map[string][]byte
}

func newConnSecrets() *connSecrets { return &connSecrets{rows: map[string][]byte{}} }

func (s *connSecrets) Get(target string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.rows[target]
	if !ok {
		return nil, errConnNotFound
	}
	return b, nil
}

func (s *connSecrets) Put(target, _ string, blob []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[target] = blob
	return nil
}

func (s *connSecrets) Delete(target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, target)
	return nil
}

func (s *connSecrets) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

// connManager builds a Manager wired to a Fake rather than to a network. No
// test in this file may reach a real Panacea: the Version and the client are
// both injected, and t.TempDir is the whole of its disk.
func connManager(t *testing.T, fake *deploytest.Fake, mutate func(*connection.Config)) (*connection.Manager, *connSecrets) {
	t.Helper()
	secrets := newConnSecrets()
	cfg := connection.Config{
		Secrets:    secrets,
		IsNotFound: func(err error) bool { return errors.Is(err, errConnNotFound) },
		StateDir:   t.TempDir(),
		Log:        catalogQuietLogger(),
		Now:        func() time.Time { return time.Date(2026, 9, 23, 10, 2, 11, 123e6, time.UTC) },
		Version:    func(context.Context, string) (string, error) { return "v1.163.0", nil },
		NewClient: func(string, panacea.TokenSource) (deploy.Deployer, error) {
			return fake, nil
		},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	m, err := connection.New(cfg)
	if err != nil {
		t.Fatalf("connection.New: %v", err)
	}
	return m, secrets
}

// connFake is a Fake that answers Whoami with an approved account.
func connFake() *deploytest.Fake {
	f := deploytest.NewFake(func() time.Time { return time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC) })
	f.SetAccount(deploy.Account{Username: "dmackie1", Email: "dmackie1@kpmg.com.au", Approved: true})
	return f
}

// connAPI mounts one Manager on the real surface, through api.New.
func connAPI(t *testing.T, m *connection.Manager) http.Handler {
	t.Helper()
	return catalogNewAPI(t, func(d *api.Deps) { d.DeployConnection = m })
}

// connDecode reads a JSON object body, and fails the test when the status is
// not the one expected — a wrong status with the right body is still wrong.
func connDecode(t *testing.T, body string, code, want int) map[string]any {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d; body %s", code, want, body)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", body, err)
	}
	return out
}

// TestNilConnectionIs409 pins the code a phase 7 route answers when nothing is
// wired: 409 deploy_not_connected, not the 503 a missing dependency would
// otherwise suggest. The desktop switches on the code, and "this cockpit has no
// Panacea" and "Panacea is down" send a user to different places.
func TestNilConnectionIs409(t *testing.T) {
	h := catalogNewAPI(t, nil)
	for _, tc := range []struct{ name, method, target, body string }{
		{"get", http.MethodGet, "/v1/deploy/connection", ""},
		{"put", http.MethodPut, "/v1/deploy/connection",
			`{"origin":"` + connTestOrigin + `","token":"` + connTestToken + `"}`},
		{"check", http.MethodPost, "/v1/deploy/connection/check", ""},
		{"delete", http.MethodDelete, "/v1/deploy/connection", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := catalogDo(t, h, tc.method, tc.target, tc.body, nil)
			got := connDecode(t, rec.Body.String(), rec.Code, http.StatusConflict)
			if got["error"] != "deploy_not_connected" {
				t.Errorf("error = %v, want deploy_not_connected", got["error"])
			}
			if got["message"] != "Connect to Panacea first." {
				t.Errorf("message = %v, want the connect sentence", got["message"])
			}
		})
	}
}

// TestConnectionGETNeverHasToken is the whole point of the route pair: the key
// enters through the PUT and never comes back out of anything.
func TestConnectionGETNeverHasToken(t *testing.T) {
	m, secrets := connManager(t, connFake(), nil)
	h := connAPI(t, m)

	put := catalogDo(t, h, http.MethodPut, "/v1/deploy/connection",
		`{"origin":"`+connTestOrigin+`","token":"`+connTestToken+`"}`, nil)
	body := connDecode(t, put.Body.String(), put.Code, http.StatusOK)
	if body["state"] != "connected" {
		t.Errorf("state = %v, want connected", body["state"])
	}
	if body["origin"] != connTestOrigin {
		t.Errorf("origin = %v, want %q", body["origin"], connTestOrigin)
	}
	if body["apiVersion"] != "v1.163.0" {
		t.Errorf("apiVersion = %v, want v1.163.0", body["apiVersion"])
	}
	if body["checkedAt"] == nil {
		t.Error("checkedAt is null after a connect that reached Panacea")
	}
	account, ok := body["account"].(map[string]any)
	if !ok || account["username"] != "dmackie1" {
		t.Fatalf("account = %v, want the whoami account", body["account"])
	}
	if secrets.count() != 1 {
		t.Errorf("credential rows = %d, want exactly one", secrets.count())
	}
	if _, err := secrets.Get(connTestTarget); err != nil {
		t.Errorf("no row at %q: %v", connTestTarget, err)
	}

	get := catalogDo(t, h, http.MethodGet, "/v1/deploy/connection", "", nil)
	raw := get.Body.String()
	connDecode(t, raw, get.Code, http.StatusOK)
	for _, leaked := range []string{"pnc_", "abcdef"} {
		if strings.Contains(raw, leaked) {
			t.Errorf("GET body contains %q: %s", leaked, raw)
		}
		if strings.Contains(put.Body.String(), leaked) {
			t.Errorf("PUT body contains %q: %s", leaked, put.Body.String())
		}
	}
}

// TestPutRejectsNonPncToken refuses a pasted JWT before anything is stored and
// before anything is sent: the wrong field on the portal page is a bearer token
// for a different system, and storing it would show "connected" for ever.
func TestPutRejectsNonPncToken(t *testing.T) {
	fake := connFake()
	m, secrets := connManager(t, fake, nil)
	h := connAPI(t, m)

	rec := catalogDo(t, h, http.MethodPut, "/v1/deploy/connection",
		`{"origin":"`+connTestOrigin+`","token":"eyJhbGciOiJIUzI1NiJ9.e30.x"}`, nil)
	got := connDecode(t, rec.Body.String(), rec.Code, http.StatusBadRequest)
	if got["error"] != "bad_request" {
		t.Errorf("error = %v, want bad_request", got["error"])
	}
	const want = "A Panacea API key starts with pnc_. Copy it from the portal: Account → API keys."
	if got["message"] != want {
		t.Errorf("message = %v,\nwant %q", got["message"], want)
	}
	if secrets.count() != 0 {
		t.Errorf("credential rows = %d, want none", secrets.count())
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("calls = %v, want none: the refusal is local", fake.Calls())
	}
}

// TestPutMissingFieldsAreBadRequest covers the wire's other two 400s.
func TestPutMissingFieldsAreBadRequest(t *testing.T) {
	for name, body := range map[string]string{
		"no token":    `{"origin":"` + connTestOrigin + `"}`,
		"empty token": `{"origin":"` + connTestOrigin + `","token":"  "}`,
		"no origin":   `{"token":"` + connTestToken + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			fake := connFake()
			m, secrets := connManager(t, fake, nil)
			rec := catalogDo(t, connAPI(t, m), http.MethodPut, "/v1/deploy/connection", body, nil)
			got := connDecode(t, rec.Body.String(), rec.Code, http.StatusBadRequest)
			if got["error"] != "bad_request" {
				t.Errorf("error = %v, want bad_request", got["error"])
			}
			if secrets.count() != 0 || len(fake.Calls()) != 0 {
				t.Errorf("rows %d, calls %v; want nothing stored and nothing sent",
					secrets.count(), fake.Calls())
			}
		})
	}
}

// TestPutLockedOriginRefusesAnother is the route half of the package's own row.
// An administrator's PANACEA_APP_PANACEA_URL is not a default, and the refusal
// is 400 bad_request - the CALLER sent the wrong address - not the 422 a
// Panacea refusal would be.
func TestPutLockedOriginRefusesAnother(t *testing.T) {
	locked := func(c *connection.Config) { c.LockedOrigin = connTestOrigin }

	fake := connFake()
	m, secrets := connManager(t, fake, locked)
	rec := catalogDo(t, connAPI(t, m), http.MethodPut, "/v1/deploy/connection",
		`{"origin":"https://other.panacea.kpmg","token":"`+connTestToken+`"}`, nil)
	got := connDecode(t, rec.Body.String(), rec.Code, http.StatusBadRequest)
	if got["error"] != "bad_request" {
		t.Errorf("error = %v, want bad_request", got["error"])
	}
	const want = "This cockpit's Panacea address is set by PANACEA_APP_PANACEA_URL. Send the key alone."
	if got["message"] != want {
		t.Errorf("message = %v,\nwant %q", got["message"], want)
	}
	if secrets.count() != 0 || len(fake.Calls()) != 0 {
		t.Errorf("rows %d, calls %v; want nothing stored and nothing sent", secrets.count(), fake.Calls())
	}

	// The connect form SHOWS the locked address, and a form that shows a value
	// sends it back. The same address in another spelling is not an override.
	m2, secrets2 := connManager(t, connFake(), locked)
	rec2 := catalogDo(t, connAPI(t, m2), http.MethodPut, "/v1/deploy/connection",
		`{"origin":"HTTPS://Dev.Panacea.KPMG/","token":"`+connTestToken+`"}`, nil)
	body := connDecode(t, rec2.Body.String(), rec2.Code, http.StatusOK)
	if body["state"] != "connected" || body["originLocked"] != true {
		t.Errorf("state %v, originLocked %v; want connected and locked", body["state"], body["originLocked"])
	}
	if secrets2.count() != 1 {
		t.Errorf("credential rows = %d, want one", secrets2.count())
	}
}

// TestPutTokenRejectedIs409 pins the code and the sentence for Panacea's 401.
// A 401 on THIS surface already means the local caller key is wrong, so a
// rejected Panacea key cannot borrow it.
func TestPutTokenRejectedIs409(t *testing.T) {
	fake := connFake()
	fake.FailNext("Whoami", &deploy.Error{Kind: deploy.KindUnauthorized, Op: "whoami",
		Message: "api key revoked or expired", Status: http.StatusUnauthorized})
	m, secrets := connManager(t, fake, nil)

	rec := catalogDo(t, connAPI(t, m), http.MethodPut, "/v1/deploy/connection",
		`{"origin":"`+connTestOrigin+`","token":"`+connTestToken+`"}`, nil)
	got := connDecode(t, rec.Body.String(), rec.Code, http.StatusConflict)
	if got["error"] != "deploy_token_rejected" {
		t.Errorf("error = %v, want deploy_token_rejected", got["error"])
	}
	if got["message"] != "api key revoked or expired" {
		t.Errorf("message = %v, want Panacea's own sentence", got["message"])
	}
	if secrets.count() != 0 {
		t.Errorf("credential rows = %d, want none: a key that 401s is not stored", secrets.count())
	}
}

// TestConnectionCheckAndDelete walks the other two verbs. Disconnect keeps the
// ADDRESS - reconnecting asks only for a key - and removes the key.
func TestConnectionCheckAndDelete(t *testing.T) {
	m, secrets := connManager(t, connFake(), nil)
	h := connAPI(t, m)
	put := catalogDo(t, h, http.MethodPut, "/v1/deploy/connection",
		`{"origin":"`+connTestOrigin+`","token":"`+connTestToken+`"}`, nil)
	connDecode(t, put.Body.String(), put.Code, http.StatusOK)

	check := catalogDo(t, h, http.MethodPost, "/v1/deploy/connection/check", "", nil)
	body := connDecode(t, check.Body.String(), check.Code, http.StatusOK)
	if body["state"] != "connected" || body["apiVersion"] != "v1.163.0" {
		t.Errorf("check body = %v, want a connected status", body)
	}

	del := catalogDo(t, h, http.MethodDelete, "/v1/deploy/connection", "", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204; body %s", del.Code, del.Body.String())
	}
	if got := del.Body.String(); got != "" {
		t.Errorf("DELETE body = %q, want empty", got)
	}
	if secrets.count() != 0 {
		t.Errorf("credential rows = %d after DELETE, want none", secrets.count())
	}

	get := catalogDo(t, h, http.MethodGet, "/v1/deploy/connection", "", nil)
	after := connDecode(t, get.Body.String(), get.Code, http.StatusOK)
	if after["state"] != "not_connected" {
		t.Errorf("state = %v after DELETE, want not_connected", after["state"])
	}
	if after["origin"] != connTestOrigin {
		t.Errorf("origin = %v after DELETE, want the address kept", after["origin"])
	}
}
