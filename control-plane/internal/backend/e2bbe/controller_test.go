package e2bbe

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type checkpointFixture struct {
	mu   sync.Mutex
	rows [][]byte
	fail bool
}

func (s *checkpointFixture) Claim(context.Context, CheckpointClaim) (CheckpointWriter, error) {
	return &checkpointFixtureWriter{s}, nil
}

type checkpointFixtureWriter struct{ sink *checkpointFixture }

func (w *checkpointFixtureWriter) Commit(ctx context.Context, hash string, b []byte) (string, error) {
	return w.sink.Commit(ctx, hash, b)
}
func (w *checkpointFixtureWriter) Close() error { return nil }
func (s *checkpointFixture) Commit(ctx context.Context, hash string, b []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if s.fail {
		return "", errors.New("checkpoint unavailable")
	}
	s.rows = append(s.rows, append([]byte(nil), b...))
	return hash, nil
}

type providerFixture struct {
	t                            *testing.T
	server                       *httptest.Server
	mu                           sync.Mutex
	rows                         map[string]providerSandbox
	creates, deletes             int
	lostACK, holdDelete, foreign bool
	guestGrant                   bool
	invalidGuestGrant            bool
	entered, release             chan struct{}
}

func newProvider(t *testing.T) *providerFixture {
	p := &providerFixture{t: t, rows: map[string]providerSandbox{}}
	p.server = httptest.NewTLSServer(http.HandlerFunc(p.handle))
	t.Cleanup(p.server.Close)
	return p
}
func (p *providerFixture) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != "test-only-key" {
		p.t.Error("missing API authorization")
		w.WriteHeader(401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Method == "POST" && r.URL.Path == "/v2/sandboxes" {
		var body struct {
			Template   string `json:"templateID"`
			Timeout    int    `json:"timeout"`
			AutoPause  bool   `json:"autoPause"`
			AutoResume struct {
				Enabled bool `json:"enabled"`
			} `json:"autoResume"`
			Network struct {
				Public bool     `json:"allowPublicTraffic"`
				Allow  []string `json:"allowOut"`
				Deny   []string `json:"denyOut"`
			} `json:"network"`
			Metadata map[string]string `json:"metadata"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Template != "template-one" || body.Timeout != 60 || body.AutoPause || body.AutoResume.Enabled || body.Network.Public || len(body.Network.Allow) != 1 || body.Network.Allow[0] != "api.openai.com" || len(body.Network.Deny) != 1 || body.Network.Deny[0] != "0.0.0.0/0" || r.Header.Get("Idempotency-Key") != "" {
			p.t.Error("incorrect pinned create contract")
			w.WriteHeader(400)
			return
		}
		p.creates++
		id := fmt.Sprintf("vm-%d", p.creates)
		row := providerSandbox{ID: id, TemplateID: body.Template, State: "running", CPU: 2, MemoryMiB: 2048, Metadata: body.Metadata}
		if p.guestGrant {
			row.EnvdAccessToken = "fixture-envd"
			row.TrafficAccessToken = "fixture-traffic"
			row.EnvdVersion = EnvdVersion
			if p.invalidGuestGrant {
				row.TrafficAccessToken = ""
			}
		}
		p.rows[id] = row
		entered, release := p.entered, p.release
		if entered != nil {
			close(entered)
			p.entered = nil
			p.mu.Unlock()
			select {
			case <-release:
			case <-r.Context().Done():
			}
			p.mu.Lock()
		}
		if p.lostACK {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(row)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/v2/sandboxes" {
		m, e := url.ParseQuery(r.URL.Query().Get("metadata"))
		if e != nil || r.URL.Query().Get("limit") != "100" {
			p.t.Error("bad metadata lookup")
		}
		out := []providerSandbox{}
		for _, row := range p.rows {
			match := true
			for k, v := range m {
				if len(v) != 1 || row.Metadata[k] != v[0] {
					match = false
				}
			}
			if match {
				out = append(out, row)
			}
		}
		json.NewEncoder(w).Encode(out)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/sandboxes/")
	row, ok := p.rows[id]
	if !ok {
		w.WriteHeader(404)
		return
	}
	if r.Method == "GET" {
		if p.foreign {
			row.Metadata = map[string]string{"luci.owner": "someone-else"}
		}
		json.NewEncoder(w).Encode(row)
		return
	}
	if r.Method == "DELETE" {
		p.deletes++
		if !p.holdDelete {
			delete(p.rows, id)
		}
		w.WriteHeader(204)
		return
	}
	w.WriteHeader(400)
}
func fixtureConfig(t *testing.T, p *providerFixture) (Config, *checkpointFixture) {
	roots := x509.NewCertPool()
	roots.AddCert(p.server.Certificate())
	sink := &checkpointFixture{}
	return Config{Endpoint: p.server.URL, APIKey: "test-only-key", RootCAs: roots, StateDir: filepath.Join(t.TempDir(), "state"), OwnerID: "owner-a", EgressAllow: []string{"api.openai.com"}, Deployment: Deployment{Provider: "aws", Region: "us-east-1", InstanceType: "m8i.xlarge", Scope: "scope-one", HostGeneration: "host-one", RuntimeRevision: RuntimeRevision, ManagementReceiptDigest: strings.Repeat("a", 64), RestoreReceiptDigest: strings.Repeat("b", 64)}, Template: Template{ID: "template-one", BuildDigest: strings.Repeat("c", 64), MemoryMiB: 2048, CPUMillis: 2000}, Limits: Limits{MaxConcurrent: 4, MaxPerProject: 2, MemoryMiB: 8192, CPUMillis: 8000, EstimatedCostMicros: 10000, MaxTaskSeconds: 60}, Checkpoint: sink, Authorize: func(context.Context, Request, string) error { return nil }}, sink
}
func TestConfiguredModelEgressRejectsBroadAndLocalDestinations(t *testing.T) {
	for _, domains := range [][]string{{"*.openai.com"}, {"0.0.0.0/0"}, {"127.0.0.1"}, {"localhost"}, {"model.internal"}, {"API.OPENAI.COM"}, {"api.openai.com", "api.openai.com"}} {
		if ValidEgressDomains(domains) {
			t.Fatalf("unsafe model egress admitted: %v", domains)
		}
	}
	if !ValidEgressDomains([]string{"api.openai.com", "api.anthropic.com"}) {
		t.Fatal("exact upstream model domains refused")
	}
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	cfg.EgressAllow = []string{"*.openai.com"}
	if controller, err := New(cfg); !errors.Is(err, ErrUnsupported) || controller != nil {
		t.Fatal("invalid egress reached controller setup", err)
	}
}
func openController(t *testing.T, c Config) *Controller {
	t.Helper()
	v, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if e := v.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	return v
}
func taskRequest(project, task string) Request {
	return Request{Identity: Identity{OwnerID: "owner-a", ProjectID: project, TaskID: task, AttemptID: "attempt-" + task, OperationID: "op-" + project + "-" + task}, SourceDigest: strings.Repeat("1", 64), ContextDigest: strings.Repeat("2", 64), BundleDigest: strings.Repeat("3", 64), MaxSeconds: 60, EstimatedCostMicros: 100}
}
func requireAbsent(t *testing.T, r Record, e error) {
	t.Helper()
	if e != nil || r.State != Absent || r.ResourcesHeld || r.ProviderBillingSettled {
		t.Fatalf("not exact allocation absence with unsettled billing: %+v %v", r, e)
	}
}
func TestTwelveTasksFourProjectsDistinctVMsAndBoundedCapacity(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	c := openController(t, cfg)
	seen := map[string]bool{}
	for round := 0; round < 3; round++ {
		var rows []Record
		for project := 0; project < 4; project++ {
			r, e := c.Allocate(context.Background(), taskRequest(fmt.Sprintf("project-%d", project), fmt.Sprintf("task-%d", round)))
			if e != nil || r.State != Allocated || seen[r.SandboxID] {
				t.Fatalf("allocation %+v %v", r, e)
			}
			seen[r.SandboxID] = true
			rows = append(rows, r)
		}
		_, usage := c.Snapshot()
		if usage.Active != 4 || usage.MemoryMiB != 8192 || usage.CPUMillis != 8000 {
			t.Fatal(usage)
		}
		if _, e := c.Allocate(context.Background(), taskRequest("project-extra", fmt.Sprintf("extra-%d", round))); !errors.Is(e, ErrCapacity) {
			t.Fatal(e)
		}
		for _, r := range rows {
			after, e := c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
			requireAbsent(t, after, e)
		}
	}
	_, u := c.Snapshot()
	if len(seen) != 12 || p.creates != 12 || p.deletes != 12 || len(p.rows) != 0 || u.Active != 0 || u.EstimatedCostMicros != 1200 {
		t.Fatalf("VMs %d creates %d deletes %d usage %+v", len(seen), p.creates, p.deletes, u)
	}
	if len(sink.rows) < 36 {
		t.Fatal("missing off-host transition acknowledgments")
	}
	for _, b := range sink.rows {
		if bytes.Contains(b, []byte(cfg.APIKey)) {
			t.Fatal("credential persisted")
		}
	}
}
func TestLostCreateACKReopenReconcilesWithoutReplay(t *testing.T) {
	p := newProvider(t)
	p.lostACK = true
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	r, e := c.Allocate(context.Background(), req)
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld {
		t.Fatal(r, e)
	}
	if _, e = c.Allocate(context.Background(), req); !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	if e = c.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	c = openController(t, cfg)
	r, e = c.Reconcile(context.Background(), req.Identity.OperationID)
	if e != nil || r.State != Allocated || p.creates != 1 {
		t.Fatal(r, e, p.creates)
	}
	r, e = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
	requireAbsent(t, r, e)
}
func TestCancelDuringCreateRecordsIntentThenSettlesExactVM(t *testing.T) {
	p := newProvider(t)
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	entered := p.entered
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	done := make(chan error, 1)
	go func() {
		r, e := c.Allocate(context.Background(), req)
		if e == nil && (r.State != Absent || r.ResourcesHeld) {
			e = fmt.Errorf("not settled: %+v", r)
		}
		done <- e
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Create did not start")
	}
	rows, _ := c.Snapshot()
	r, e := c.Cancel(context.Background(), req.Identity.OperationID, rows[0].Revision)
	if e != nil || !r.CancelRequested || !r.ResourcesHeld || r.State != Cancelling {
		t.Fatal(r, e)
	}
	p.mu.Lock()
	deletes := p.deletes
	p.mu.Unlock()
	if deletes != 0 {
		t.Fatal("cancel raced a second provider operation")
	}
	close(p.release)
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Create/cancel did not settle")
	}
	if p.creates != 1 || p.deletes != 1 {
		t.Fatal(p.creates, p.deletes)
	}
}
func TestUnconfirmedDeleteRetainsChargeAndForeignIdentityIsNeverDeleted(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	p.foreign = true
	r, e = c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
	if !errors.Is(e, ErrConflict) || p.deletes != 0 || !r.ResourcesHeld {
		t.Fatal(r, e)
	}
	p.foreign = false
	p.holdDelete = true
	r, e = c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld {
		t.Fatal(r, e)
	}
	p.holdDelete = false
	r, e = c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
	requireAbsent(t, r, e)
}
func TestAuthorityStaleRevisionAndChangedContextCannotAllocateOrDelete(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	denied := false
	cfg.Authorize = func(context.Context, Request, string) error {
		if denied {
			return errors.New("revoked")
		}
		return nil
	}
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	r, e := c.Allocate(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	changed := req
	changed.ContextDigest = strings.Repeat("4", 64)
	if _, e = c.Allocate(context.Background(), changed); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision-1); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	denied = true
	if _, e = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision); e == nil {
		t.Fatal("revocation ignored")
	}
	if p.creates != 1 || p.deletes != 0 {
		t.Fatal(p.creates, p.deletes)
	}
}
func TestUnacknowledgedOffHostCheckpointPreventsProviderCreate(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	sink.fail = true
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if !errors.Is(e, ErrUnknown) || p.creates != 0 || !r.ResourcesHeld {
		t.Fatal(r, e, p.creates)
	}
}
func TestReplacementHostCannotTreatEmptyInventoryAsOldVMDestroyed(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	c.Close(context.Background())
	cfg.Deployment.HostGeneration = "replacement-host"
	cfg.Deployment.RestoreReceiptDigest = strings.Repeat("d", 64)
	c = openController(t, cfg)
	after, e := c.Reconcile(context.Background(), r.Request.Identity.OperationID)
	if !errors.Is(e, ErrUnknown) || !after.ResourcesHeld {
		t.Fatal(after, e)
	}
	if _, e = c.Allocate(context.Background(), taskRequest("beta", "one")); !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	if p.creates != 1 || p.deletes != 0 {
		t.Fatal("old host ownership lost")
	}
}
func TestSecondControllerCannotOwnSameJournal(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	openController(t, cfg)
	other, e := New(cfg)
	if other != nil {
		other.Close(context.Background())
	}
	if e == nil {
		t.Fatal("second controller acquired ownership")
	}
}
func TestUnsafeEndpointAndMissingDurabilityRefuseBeforeNetwork(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	for _, endpoint := range []string{"http://127.0.0.1", "https://127.0.0.1:5008", "https://8.8.8.8", "https://user:password@127.0.0.1", "https://127.0.0.1?x=y"} {
		t.Run(endpoint, func(t *testing.T) {
			c := cfg
			c.Endpoint = endpoint
			if v, e := New(c); v != nil || !errors.Is(e, ErrUnsupported) {
				t.Fatal(v, e)
			}
		})
	}
	cfg.Checkpoint = nil
	if _, e := New(cfg); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}
func TestJournalTamperRefusesWithoutNetwork(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	_, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	c.Close(context.Background())
	file := filepath.Join(cfg.StateDir, "attempts.json")
	b, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	b = bytes.Replace(b, []byte(`"resourcesHeld":true`), []byte(`"resourcesHeld":false`), 1)
	if e = os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	if other, e := New(cfg); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal(other, e)
	}
	if p.creates != 1 {
		t.Fatal("tampered journal caused replay")
	}
}

func TestConfiguredConcurrencyAboveFourAndPerProjectBackpressure(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	cfg.Limits.MaxConcurrent = 6
	cfg.Limits.MemoryMiB = 12288
	cfg.Limits.CPUMillis = 12000
	c := openController(t, cfg)
	for i := 0; i < 2; i++ {
		if _, e := c.Allocate(context.Background(), taskRequest("alpha", fmt.Sprintf("task-%d", i))); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := c.Allocate(context.Background(), taskRequest("alpha", "third")); !errors.Is(e, ErrCapacity) {
		t.Fatal("per-project limit ignored", e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := c.Allocate(context.Background(), taskRequest(fmt.Sprintf("peer-%d", i), "one"))
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	_, u := c.Snapshot()
	if u.Active != 6 || p.creates != 6 {
		t.Fatal("four became product ceiling", u, p.creates)
	}
	if _, e := c.Allocate(context.Background(), taskRequest("seventh", "one")); !errors.Is(e, ErrCapacity) {
		t.Fatal(e)
	}
}
func TestSettledVMDoesNotRefundEstimatedSpendAndLifetimeIsBounded(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	cfg.Limits.EstimatedCostMicros = 100
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	long := req
	long.MaxSeconds = 61
	if _, e := c.Allocate(context.Background(), long); !errors.Is(e, ErrCapacity) {
		t.Fatal(e)
	}
	r, e := c.Allocate(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	r, e = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
	requireAbsent(t, r, e)
	if _, e := c.Allocate(context.Background(), taskRequest("beta", "one")); !errors.Is(e, ErrCapacity) {
		t.Fatal("absence incorrectly refunded estimated spend", e)
	}
	if p.creates != 1 {
		t.Fatal(p.creates)
	}
}
func TestTLSVerificationFailureNeverForwardsCredentialOrReplays(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	cfg.RootCAs = x509.NewCertPool()
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld || p.creates != 0 {
		t.Fatal(r, e, p.creates)
	}
	if _, e = c.Allocate(context.Background(), r.Request); !errors.Is(e, ErrUnknown) || p.creates != 0 {
		t.Fatal(e, p.creates)
	}
}
func TestMissingUnknownCreateAndCancelledCallerKeepOwnership(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	sink.fail = true
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	r, e := c.Allocate(context.Background(), req)
	if !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	sink.fail = false
	r, e = c.Reconcile(context.Background(), req.Identity.OperationID)
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld {
		t.Fatal("empty inventory released ambiguous creation", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.Allocate(ctx, taskRequest("beta", "one")); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if p.creates != 0 {
		t.Fatal(p.creates)
	}
}
func TestCloseDuringCreateKeepsChargeAndReopenObservesExactVM(t *testing.T) {
	p := newProvider(t)
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	entered := p.entered
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	done := make(chan error, 1)
	go func() { _, e := c.Allocate(context.Background(), req); done <- e }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Create absent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if e := c.Close(ctx); e != nil {
		t.Fatal(e)
	}
	if e := <-done; !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	close(p.release)
	c = openController(t, cfg)
	r, e := c.Reconcile(context.Background(), req.Identity.OperationID)
	if e != nil || r.State != Allocated || !r.ResourcesHeld {
		t.Fatal(r, e)
	}
	r, e = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
	requireAbsent(t, r, e)
	if p.creates != 1 {
		t.Fatal("controller restart replayed Create")
	}
}
