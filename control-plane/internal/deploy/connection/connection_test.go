package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea/panaceatest"
)

// errNotFound stands in for credstore.ErrNotFound: this package is declared
// against the three-method Secrets subset so phase 7 compiles before Hub C1.
var errNotFound = errors.New("secrets: not found")

// fakeSecrets is a credential store in a map. It records every Put, because two
// tests assert the COUNT and the target: a second Put is a second copy of the
// token under a name nothing deletes.
type fakeSecrets struct {
	mu      sync.Mutex
	rows    map[string][]byte
	users   map[string]string
	puts    []string
	putErr  error
	getErr  error
	deletes []string
}

func newSecrets() *fakeSecrets {
	return &fakeSecrets{rows: map[string][]byte{}, users: map[string]string{}}
}

func (f *fakeSecrets) Get(target string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	b, ok := f.rows[target]
	if !ok {
		return nil, errNotFound
	}
	return b, nil
}

func (f *fakeSecrets) Put(target, userName string, blob []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, target)
	if f.putErr != nil {
		return f.putErr
	}
	f.rows[target] = blob
	f.users[target] = userName
	return nil
}

func (f *fakeSecrets) Delete(target string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, target)
	delete(f.rows, target)
	return nil
}

func (f *fakeSecrets) putTargets() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.puts...)
}

func (f *fakeSecrets) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

// baseConfig is a Config with nothing wired to the network: every test names the
// Version and NewClient it wants.
func baseConfig(t *testing.T, s *fakeSecrets) (Config, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	return Config{
		Secrets:    s,
		IsNotFound: func(err error) bool { return errors.Is(err, errNotFound) },
		StateDir:   t.TempDir(),
		Now:        func() time.Time { return time.Date(2026, 9, 23, 10, 2, 11, 123e6, time.UTC) },
		Log:        slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Version:    func(context.Context, string) (string, error) { return "v1.163.0", nil },
		NewClient: func(string, panacea.TokenSource) (deploy.Deployer, error) {
			t.Fatal("NewClient was called by a test that named no client")
			return nil, nil
		},
	}, &logs
}

// overServer points NewClient at a panaceatest server: a real request, a real
// bearer, and a real 401 for any token but the server's own.
func overServer(cfg *Config, srv *panaceatest.Server) {
	cfg.NewClient = func(origin string, tokens panacea.TokenSource) (deploy.Deployer, error) {
		c, err := panacea.New(panacea.Config{Origin: origin, Tokens: tokens, HTTPClient: srv.HTTPClient()})
		if err != nil {
			return nil, err
		}
		return PanaceaDeployer{Client: c}, nil
	}
}

func mustNew(t *testing.T, cfg Config) *Manager {
	t.Helper()
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestConnectStoresNothingOn401(t *testing.T) {
	// The whole point of proving the token before storing it: a stored token
	// that does not work shows "connected" at the next start and fails every
	// call after it.
	srv := panaceatest.New(t)
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	m := mustNew(t, cfg)

	_, err := m.Connect(context.Background(), srv.URL(), "pnc_notthisone")

	if got := deploy.KindOf(err); got != deploy.KindUnauthorized {
		t.Fatalf("Connect kind = %q, want %q (err %v)", got, deploy.KindUnauthorized, err)
	}
	if n := s.count(); n != 0 {
		t.Errorf("the credential store holds %d rows after a refused token, want 0", n)
	}
	if st := m.Status(); st.State != NotConnected {
		t.Errorf("state = %q, want %q", st.State, NotConnected)
	}
}

func TestConnectStoresNothingOnBadOrigin(t *testing.T) {
	// A paste into the wrong field is the scenario. If Whoami ran before
	// Version answered, the token would have been sent to whatever address the
	// user typed - which is a leak, not a failed connect.
	srv := panaceatest.New(t)
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	cfg.Version = func(context.Context, string) (string, error) {
		return "", &deploy.Error{Kind: deploy.KindRejected, Op: "version", Message: "That address answered, but not as the Panacea API.", Status: 200}
	}
	m := mustNew(t, cfg)

	_, err := m.Connect(context.Background(), srv.URL(), srv.Token())

	if got := deploy.KindOf(err); got != deploy.KindRejected {
		t.Fatalf("Connect kind = %q, want %q (err %v)", got, deploy.KindRejected, err)
	}
	if reqs := srv.Requests(); len(reqs) != 0 {
		t.Errorf("the token was sent to an address Version refused: %v", reqs)
	}
	if n := s.count(); n != 0 {
		t.Errorf("the credential store holds %d rows, want 0", n)
	}
}

func TestOriginWrittenAfterToken(t *testing.T) {
	// The order is the guarantee: a crash between the two must never leave an
	// origin pointing at a token that is not there, because that start says
	// "connected" and every call 401s.
	srv := panaceatest.New(t)
	s := newSecrets()
	s.putErr = errors.New("credential manager said no")
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	m := mustNew(t, cfg)

	if _, err := m.Connect(context.Background(), srv.URL(), srv.Token()); err == nil {
		t.Fatal("Connect with a failing Put returned nil, want the Put's error")
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, originFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the origin file exists after the token Put failed (stat err %v)", err)
	}

	s.mu.Lock()
	s.putErr, s.puts = nil, nil // the refused attempt is proved above; count the successful one
	s.mu.Unlock()
	if _, err := m.Connect(context.Background(), srv.URL(), srv.Token()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.StateDir, originFile))
	if err != nil {
		t.Fatalf("read the origin file: %v", err)
	}
	var got struct {
		Origin  string    `json:"origin"`
		SavedAt time.Time `json:"savedAt"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the origin file is not JSON: %v (%s)", err, raw)
	}
	want, err := panacea.ParseOrigin(srv.URL())
	if err != nil {
		t.Fatalf("ParseOrigin(%q): %v", srv.URL(), err)
	}
	if got.Origin != want {
		t.Errorf("origin = %q, want %q", got.Origin, want)
	}
	if got.SavedAt.IsZero() {
		t.Error("savedAt is zero; the file records when the address was accepted")
	}
	if strings.Contains(string(raw), "pnc_") {
		t.Errorf("the origin file carries the token: %s", raw)
	}
	// Exactly one Put, to the one row Hub contract 9.1 reserves. A second row -
	// "current", say - would share the <api host> namespace with a token row.
	wantTarget := targetTokenBase + strings.TrimPrefix(want, "https://")
	if puts := s.putTargets(); len(puts) != 1 || puts[0] != wantTarget {
		t.Errorf("Puts = %v, want exactly one to %q", puts, wantTarget)
	}
}

func TestOriginFileMalformedIsAbsent(t *testing.T) {
	// A torn or hand-edited config file must not fail the start, and must not
	// take the token with it: the user re-enters the address, not the key.
	s := newSecrets()
	cfg, logs := baseConfig(t, s)
	row := targetTokenBase + "dev.panacea.kpmg"
	s.rows[row] = []byte("pnc_abcdef0123456789abcdef0123456789")
	if err := os.WriteFile(filepath.Join(cfg.StateDir, originFile), []byte(`{"origin":`), 0o600); err != nil {
		t.Fatalf("seed a truncated origin file: %v", err)
	}
	cfg.Version = func(context.Context, string) (string, error) {
		t.Fatal("New reached the network")
		return "", nil
	}

	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New on a malformed origin file returned %v, want no error", err)
	}
	st := m.Status()
	if st.State != NotConnected {
		t.Errorf("state = %q, want %q", st.State, NotConnected)
	}
	if st.Origin != "" {
		t.Errorf("origin = %q, want empty", st.Origin)
	}
	if _, ok := s.rows[row]; !ok {
		t.Error("the token row was deleted because the ADDRESS was unreadable")
	}
	if n := strings.Count(logs.String(), "level=WARN"); n != 1 {
		t.Errorf("%d warn lines, want exactly 1:\n%s", n, logs.String())
	}
}

func TestLockedOriginRefusesAnother(t *testing.T) {
	// An administrator's PANACEA_APP_PANACEA_URL is not a default the user may
	// override. An origin that is EQUAL after ParseOrigin is not an override.
	srv := panaceatest.New(t)
	locked, err := panacea.ParseOrigin(srv.URL())
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	cfg.LockedOrigin = locked
	m := mustNew(t, cfg)

	_, err = m.Connect(context.Background(), "https://other.panacea.kpmg", srv.Token())

	if !errors.Is(err, ErrOriginLocked) {
		t.Fatalf("Connect to another address = %v, want ErrOriginLocked", err)
	}
	if reqs := srv.Requests(); len(reqs) != 0 {
		t.Errorf("a refused override still sent requests: %v", reqs)
	}
	if n := s.count(); n != 0 {
		t.Errorf("the credential store holds %d rows, want 0", n)
	}

	// The same address in another spelling is the same address.
	loud := strings.ToUpper(strings.TrimPrefix(locked, "https://"))
	if _, err := m.Connect(context.Background(), "HTTPS://"+loud+"/", srv.Token()); err != nil {
		t.Fatalf("Connect with an equal origin in another spelling: %v", err)
	}
	if got := m.Status().Origin; got != locked {
		t.Errorf("origin = %q, want the locked %q", got, locked)
	}
}

func TestAwaitingApproval(t *testing.T) {
	// panaceatest's /me answers approved:false, which is Panacea's answer to an
	// account waiting on its team. The token WORKS, so it is stored; the state
	// says the account cannot act yet.
	srv := panaceatest.New(t)
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	m := mustNew(t, cfg)

	st, err := m.Connect(context.Background(), srv.URL(), srv.Token())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if st.State != AwaitingApproval {
		t.Errorf("state = %q, want %q", st.State, AwaitingApproval)
	}
	if n := s.count(); n != 1 {
		t.Errorf("the credential store holds %d rows, want the working token stored", n)
	}
	if st.APIVersion != "v1.163.0" {
		t.Errorf("apiVersion = %q, want the version Version answered", st.APIVersion)
	}
	if st.CheckedAt.IsZero() {
		t.Error("checkedAt is zero after a Connect that reached Panacea")
	}
}

func TestConnectApprovedIsConnected(t *testing.T) {
	// The approved half. panaceatest names no account, so this one is over the
	// Fake, whose account the test sets.
	f := deploytest.NewFake(time.Now)
	f.SetAccount(deploy.Account{Username: "dmackie1", Email: "dmackie1@kpmg.com.au", Approved: true})
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	cfg.NewClient = func(string, panacea.TokenSource) (deploy.Deployer, error) { return f, nil }
	m := mustNew(t, cfg)

	st, err := m.Connect(context.Background(), "https://dev.panacea.kpmg", "pnc_abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if st.State != Connected {
		t.Fatalf("state = %q, want %q", st.State, Connected)
	}
	if st.Account == nil || st.Account.Username != "dmackie1" {
		t.Errorf("account = %+v, want the one Whoami answered", st.Account)
	}
	if got, err := m.PanaceaToken(context.Background()); err != nil || !strings.HasPrefix(got, "pnc_") {
		t.Errorf("PanaceaToken = %q, %v; want the stored token", got, err)
	}

	if err := m.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if n := s.count(); n != 0 {
		t.Errorf("the credential store holds %d rows after Disconnect, want 0", n)
	}
	if st := m.Status(); st.State != NotConnected || st.Origin != "https://dev.panacea.kpmg" {
		t.Errorf("after Disconnect: state %q, origin %q; want not_connected with the address KEPT", st.State, st.Origin)
	}
	if _, err := m.PanaceaToken(context.Background()); !errors.Is(err, deploy.ErrNotConnected) {
		t.Errorf("PanaceaToken after Disconnect = %v, want deploy.ErrNotConnected", err)
	}
}

func TestNewMakesNoNetworkCall(t *testing.T) {
	// A start that blocks on Panacea is a cockpit that will not open on a
	// corporate VPN that is still negotiating.
	srv := panaceatest.New(t)
	locked, err := panacea.ParseOrigin(srv.URL())
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	s := newSecrets()
	s.rows[targetTokenBase+strings.TrimPrefix(locked, "https://")] = []byte("pnc_abcdef0123456789abcdef0123456789")
	cfg, _ := baseConfig(t, s)
	overServer(&cfg, srv)
	cfg.LockedOrigin = locked
	cfg.Version = func(context.Context, string) (string, error) {
		t.Error("New called Version")
		return "", nil
	}

	m := mustNew(t, cfg)

	if reqs := srv.Requests(); len(reqs) != 0 {
		t.Errorf("New reached Panacea: %v", reqs)
	}
	st := m.Status()
	if !st.CheckedAt.IsZero() {
		t.Errorf("checkedAt = %v, want zero: nothing has been checked yet", st.CheckedAt)
	}
	if !st.OriginLocked {
		t.Error("originLocked is false although LockedOrigin is set")
	}
	if st.Origin != locked {
		t.Errorf("origin = %q, want %q", st.Origin, locked)
	}
}

func TestObserveUnauthorized(t *testing.T) {
	// The watcher saw the 401. Without this the lens keeps saying "connected"
	// while every call fails, and the user has nothing to act on.
	s := newSecrets()
	cfg, _ := baseConfig(t, s)
	f := deploytest.NewFake(time.Now)
	f.SetAccount(deploy.Account{Username: "dmackie1", Approved: true})
	cfg.NewClient = func(string, panacea.TokenSource) (deploy.Deployer, error) { return f, nil }
	m := mustNew(t, cfg)
	if _, err := m.Connect(context.Background(), "https://dev.panacea.kpmg", "pnc_abcdef0123456789abcdef0123456789"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	m.Observe(&deploy.Error{Kind: deploy.KindUnauthorized, Message: "api key revoked or expired"})

	st := m.Status()
	if st.State != TokenRejected {
		t.Errorf("state = %q, want %q", st.State, TokenRejected)
	}
	if st.Message != "api key revoked or expired" {
		t.Errorf("message = %q, want Panacea's sentence", st.Message)
	}

	m.Observe(&deploy.Error{Kind: deploy.KindUnavailable})
	if st := m.Status(); st.State != Unreachable {
		t.Errorf("after an unavailable: state = %q, want %q", st.State, Unreachable)
	}
	if st := m.Status(); st.Message != "" {
		t.Errorf("message = %q, want it cleared: the 401's sentence is not why the host is unreachable", st.Message)
	}
}

// failable is every Deployer method that can fail. SameEnvironment is the one
// exception: it answers exact equality when disconnected, because a comparison
// that cannot reach anything is still a correct comparison.
const notFailable = "SameEnvironment"

func TestCurrentWhenDisconnected(t *testing.T) {
	// By reflection, so a method added to deploy.Deployer later cannot be
	// forgotten here: a forgotten one panics on a nil delegate at the first
	// call, in front of a user, rather than answering "connect first".
	var c Current
	cv := reflect.ValueOf(&c)
	iface := reflect.TypeOf((*deploy.Deployer)(nil)).Elem()
	checked := 0

	for i := 0; i < iface.NumMethod(); i++ {
		name := iface.Method(i).Name
		if name == notFailable {
			continue
		}
		mt := cv.MethodByName(name)
		if !mt.IsValid() {
			t.Errorf("Current has no method %s", name)
			continue
		}
		args := make([]reflect.Value, mt.Type().NumIn())
		for j := range args {
			at := mt.Type().In(j)
			if at == reflect.TypeOf((*context.Context)(nil)).Elem() {
				args[j] = reflect.ValueOf(context.Background())
				continue
			}
			args[j] = reflect.Zero(at)
		}
		out := mt.Call(args)
		last := out[len(out)-1]
		if last.Type() != reflect.TypeOf((*error)(nil)).Elem() {
			t.Errorf("%s's last result is %s, not error", name, last.Type())
			continue
		}
		err, _ := last.Interface().(error)
		if !errors.Is(err, deploy.ErrNotConnected) {
			t.Errorf("%s when disconnected returned %v, want deploy.ErrNotConnected", name, err)
		}
		checked++
	}
	if checked != 24 {
		t.Errorf("checked %d methods, want the 24 that can fail; deploy.Deployer changed shape", checked)
	}
	if c.SameEnvironment("dev", "dev") != true || c.SameEnvironment("dev", "prod") != false {
		t.Error("SameEnvironment when disconnected must answer exact equality")
	}
}
