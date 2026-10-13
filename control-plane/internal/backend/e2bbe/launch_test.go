package e2bbe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	process "github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess/processconnect"
)

type processFixture struct {
	processconnect.UnimplementedProcessHandler
	mu               sync.Mutex
	t                *testing.T
	r                Record
	controller       *Controller
	guest            *guestFixture
	starts           int
	lists            int
	listed           []*process.ProcessInfo
	captures         int
	timeoutMS        int
	fail, badEvent   bool
	entered, release chan struct{}
}

func (p *processFixture) List(_ context.Context, req *connect.Request[process.ListRequest]) (*connect.Response[process.ListResponse], error) {
	if req.Header().Get("Authorization") != "Basic cm9vdDo=" || req.Header().Get("X-Access-Token") != "fixture-envd" || req.Header().Get("e2b-traffic-access-token") != "fixture-traffic" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("wrong fixture credential boundary"))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lists++
	return connect.NewResponse(&process.ListResponse{Processes: p.listed}), nil
}

func listedLaunch(r Record, pid uint32) *process.ProcessInfo {
	tag := r.Request.Identity.OperationID
	cwd := "/luci-transfer"
	return &process.ProcessInfo{Pid: pid, Tag: &tag, Config: &process.ProcessConfig{
		Cmd: "/usr/local/bin/luci-microvm-bootstrap", Args: bootstrapArgs(r), Cwd: &cwd,
		Envs: map[string]string{"OPENAI_API_KEY": "fixture-model-secret"},
	}}
}

func (p *processFixture) Start(ctx context.Context, req *connect.Request[process.StartRequest], stream *connect.ServerStream[process.StartResponse]) error {
	p.mu.Lock()
	p.starts++
	p.timeoutMS, _ = strconv.Atoi(req.Header().Get("Connect-Timeout-Ms"))
	failed, bad := p.fail, p.badEvent
	entered, release := p.entered, p.release
	p.mu.Unlock()
	msg := req.Msg
	row, e := p.controller.record(p.r.Request.Identity.OperationID)
	if msg.GetProcess().GetCmd() == "/usr/local/bin/luci-microvm-collect" {
		p.mu.Lock()
		p.starts-- // Keep original launch-only assertions exact.
		p.captures++
		p.mu.Unlock()
		if e != nil || !row.CaptureRequested || row.Capture != (CaptureReceipt{}) || msg.GetProcess().GetCwd() != "/luci-transfer" || len(msg.GetProcess().GetArgs()) != 2 || msg.GetProcess().GetArgs()[0] != "--identity-base64" || msg.GetTag() != p.r.Request.Identity.OperationID+"-capture" {
			p.t.Error("collector did not follow a durable exact reservation")
			return connect.NewError(connect.CodeInvalidArgument, errors.New("wrong collector"))
		}
		if err := stream.Send(&process.StartResponse{Event: &process.ProcessEvent{Event: &process.ProcessEvent_Start{Start: &process.ProcessEvent_StartEvent{Pid: 322}}}}); err != nil {
			return err
		}
		return stream.Send(&process.StartResponse{Event: &process.ProcessEvent{Event: &process.ProcessEvent_End{End: &process.ProcessEvent_EndEvent{Exited: true, ExitCode: 0}}}})
	}
	if e != nil || !row.LaunchRequested || row.LaunchDigest == "" || row.ProcessPID != 0 {
		p.t.Error("Start preceded durable marker")
	}
	if req.Header().Get("Authorization") != "Basic cm9vdDo=" || req.Header().Get("X-Access-Token") != "fixture-envd" || req.Header().Get("e2b-traffic-access-token") != "fixture-traffic" || req.Header().Get("X-API-Key") != "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("wrong fixture credential boundary"))
	}
	if msg.GetProcess().GetCmd() != "/usr/local/bin/luci-microvm-bootstrap" || !reflect.DeepEqual(msg.GetProcess().GetArgs(), bootstrapArgs(row)) || msg.GetProcess().GetCwd() != "/luci-transfer" || msg.GetTag() != p.r.Request.Identity.OperationID || msg.Stdin == nil || msg.GetStdin() || msg.Pty != nil {
		p.t.Error("wrong pinned process contract")
		return connect.NewError(connect.CodeInvalidArgument, errors.New("wrong process"))
	}
	if msg.Process.Envs[backend.EnvCommand] != "codex" || msg.Process.Envs[backend.EnvToken] != "fixture-attach" || msg.Process.Envs["OPENAI_API_KEY"] != "fixture-model-secret" {
		p.t.Error("wrong trusted launch environment")
	}
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if failed {
		return connect.NewError(connect.CodeUnavailable, errors.New("fixture ACK lost"))
	}
	event := &process.ProcessEvent{Event: &process.ProcessEvent_Start{Start: &process.ProcessEvent_StartEvent{Pid: 321}}}
	if bad {
		event = &process.ProcessEvent{Event: &process.ProcessEvent_End{End: &process.ProcessEvent_EndEvent{Exited: true, ExitCode: 0}}}
	}
	return stream.Send(&process.StartResponse{Event: event})
}
func launchSpec() LaunchSpec {
	return LaunchSpec{Command: "codex", Args: []string{"exec", "a prompt, with commas"}, Environment: map[string]string{"OPENAI_API_KEY": "fixture-model-secret"}, AttachToken: "fixture-attach", ConnectionRevision: "connection-r1"}
}
func launchFixture(t *testing.T) (*Controller, Config, *GuestTransport, *processFixture, *providerFixture) {
	t.Helper()
	f, gcfg, archive := newGuestFixture(t)
	p := newProvider(t)
	p.guestGrant = true
	cfg, _ := fixtureConfig(t, p)
	cfg.RootCAs.AddCert(f.server.Certificate())
	cfg.GuestOrigins = func(string) (string, string, string, error) { return f.server.URL, f.server.URL, EnvdVersion, nil }
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), gcfg.Record.Request)
	if e != nil {
		t.Fatal(e)
	}
	f.record = r
	g, e := c.Guest(context.Background(), r.Request.Identity.OperationID)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(g.Close)
	if _, e = g.StageInput(context.Background(), bytes.NewReader(archive)); e != nil {
		t.Fatal(e)
	}
	proc := &processFixture{t: t, r: r, controller: c, guest: f}
	_, handler := processconnect.NewProcessHandler(proc)
	f.mu.Lock()
	f.rpc = handler
	f.mu.Unlock()
	return c, cfg, g, proc, p
}
func TestGuestStartUsesUpstreamWireAfterDurableFenceAndDoesNotReplayOnRestart(t *testing.T) {
	c, cfg, g, p, provider := launchFixture(t)
	r := p.r
	started, e := c.StartGuest(context.Background(), r.Request.Identity.OperationID, r.Revision, launchSpec(), g)
	if e != nil || started.ProcessPID != 321 || !started.LaunchRequested || !started.ResourcesHeld {
		t.Fatal(started, e)
	}
	if e = c.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	c = openController(t, cfg)
	p.controller = c
	g, e = c.Guest(context.Background(), r.Request.Identity.OperationID)
	if e != nil {
		t.Fatal(e)
	}
	defer g.Close()
	got, e := c.StartGuest(context.Background(), r.Request.Identity.OperationID, 0, launchSpec(), g)
	if e != nil || got.ProcessPID != 321 {
		t.Fatal(got, e)
	}
	changed := launchSpec()
	changed.Args = []string{"different"}
	if _, e = c.StartGuest(context.Background(), r.Request.Identity.OperationID, got.Revision, changed, g); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.starts != 1 || p.timeoutMS < 50000 || p.timeoutMS > 60000 || provider.creates != 1 {
		t.Fatal(p.starts, p.timeoutMS, provider.creates)
	}
	b, e := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"fixture-model-secret", "fixture-attach", "a prompt, with commas"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("private launch content leaked")
		}
	}
}
func TestGuestStartLostACKAndInvalidFirstEventRetainChargeAndNeverReplay(t *testing.T) {
	for _, name := range []string{"lost-ack", "exit-without-start"} {
		t.Run(name, func(t *testing.T) {
			c, cfg, g, p, provider := launchFixture(t)
			p.fail = name == "lost-ack"
			p.badEvent = name == "exit-without-start"
			r, e := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), g)
			if !errors.Is(e, ErrUnknown) || !r.LaunchRequested || r.ProcessPID != 0 || !r.ResourcesHeld {
				t.Fatal(r, e)
			}
			if e = c.Close(context.Background()); e != nil {
				t.Fatal(e)
			}
			c = openController(t, cfg)
			g, e = c.Guest(context.Background(), p.r.Request.Identity.OperationID)
			if e != nil {
				t.Fatal(e)
			}
			defer g.Close()
			if _, e = c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, r.Revision, launchSpec(), g); !errors.Is(e, ErrUnknown) {
				t.Fatal(e)
			}
			if p.starts != 1 || provider.creates != 1 || provider.deletes != 0 {
				t.Fatal("uncertain Start replayed/released")
			}
		})
	}
}

func TestLostStartACKReadbackRecoversOnlyUniqueExactProcess(t *testing.T) {
	c, cfg, g, p, provider := launchFixture(t)
	p.fail = true
	op := p.r.Request.Identity.OperationID
	row, err := c.StartGuest(context.Background(), op, p.r.Revision, launchSpec(), g)
	if !errors.Is(err, ErrUnknown) || !row.LaunchRequested || row.ProcessPID != 0 {
		t.Fatal(row, err)
	}
	if err = c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	c = openController(t, cfg)
	p.controller = c
	g, err = c.Guest(context.Background(), op)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	p.listed = []*process.ProcessInfo{listedLaunch(row, 321)}
	got, err := c.ObserveStartPID(context.Background(), op, g)
	if err != nil || got.ProcessPID != 321 || !got.ResourcesHeld || !got.LaunchRequested || p.starts != 1 || provider.creates != 1 {
		t.Fatal(got, err, p.starts, provider.creates)
	}
	again, err := c.ObserveStartPID(context.Background(), op, g)
	if err != nil || again.ProcessPID != 321 || p.lists != 1 {
		t.Fatal(again, err, p.lists)
	}
	journal, err := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if err != nil || bytes.Contains(journal, []byte("fixture-model-secret")) {
		t.Fatal("readback disclosed the process environment")
	}
}

func TestLostStartACKReadbackRetainsUncertainOrForeignProcess(t *testing.T) {
	for _, name := range []string{"missing", "foreign-tag", "wrong-args", "duplicate", "zero-pid", "revoked"} {
		t.Run(name, func(t *testing.T) {
			c, _, g, p, provider := launchFixture(t)
			p.fail = true
			op := p.r.Request.Identity.OperationID
			row, err := c.StartGuest(context.Background(), op, p.r.Revision, launchSpec(), g)
			if !errors.Is(err, ErrUnknown) {
				t.Fatal(err)
			}
			candidate := listedLaunch(row, 321)
			switch name {
			case "missing":
				candidate = nil
			case "foreign-tag":
				tag := "other-operation"
				candidate.Tag = &tag
			case "wrong-args":
				candidate.Config.Args = []string{"other"}
			case "duplicate":
				p.listed = []*process.ProcessInfo{candidate, listedLaunch(row, 322)}
			case "zero-pid":
				candidate.Pid = 0
			case "revoked":
				c.config.Authorize = func(context.Context, Request, string) error { return backend.ErrUnauthorized }
			}
			if name != "duplicate" && candidate != nil {
				p.listed = []*process.ProcessInfo{candidate}
			}
			if _, err = c.ObserveStartPID(context.Background(), op, g); err == nil {
				t.Fatal("uncertain or foreign process accepted")
			}
			retained, err := c.record(op)
			if err != nil || retained.ProcessPID != 0 || !retained.ResourcesHeld || p.starts != 1 || provider.deletes != 0 {
				t.Fatal(retained, err, p.starts, provider.deletes)
			}
		})
	}
}
func TestGuestStartCancellationDuringACKSettlesSameVMWithoutSecondStart(t *testing.T) {
	c, _, g, p, provider := launchFixture(t)
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), g)
		done <- e
	}()
	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("start missing")
	}
	current, _ := c.record(p.r.Request.Identity.OperationID)
	cancelled, e := c.Cancel(context.Background(), current.Request.Identity.OperationID, current.Revision)
	if e != nil || !cancelled.CancelRequested || !cancelled.ResourcesHeld {
		t.Fatal(cancelled, e)
	}
	close(p.release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	after, _ := c.record(p.r.Request.Identity.OperationID)
	requireAbsent(t, after, nil)
	if after.ProcessPID != 321 || p.starts != 1 || provider.deletes != 1 {
		t.Fatal(after, p.starts, provider.deletes)
	}
}
func TestGuestStartRejectsStaleAuthorityAndUntrustedEnvironmentBeforeEffect(t *testing.T) {
	for _, name := range []string{"revision", "shell", "reserved-env", "loader-env", "oversized", "nul", "permission"} {
		t.Run(name, func(t *testing.T) {
			c, _, g, p, _ := launchFixture(t)
			l := launchSpec()
			revision := p.r.Revision
			switch name {
			case "revision":
				revision--
			case "shell":
				l.Command = "sh"
			case "reserved-env":
				l.Environment[backend.EnvCommand] = "sh"
			case "loader-env":
				l.Environment["LD_PRELOAD"] = "/evil"
			case "oversized":
				l.Args = []string{strings.Repeat("a", 65537)}
			case "nul":
				l.Args = []string{"a\x00b"}
			case "permission":
				c.config.Authorize = func(context.Context, Request, string) error { return backend.ErrUnauthorized }
			}
			if _, e := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, revision, l, g); e == nil {
				t.Fatal("unsafe launch accepted")
			}
			row, _ := c.record(p.r.Request.Identity.OperationID)
			if p.starts != 0 || row.LaunchRequested {
				t.Fatal("unauthorized launch intent/effect")
			}
		})
	}
}

func TestGuestStartCheckpointFailureRetainsIntentWithoutRPCOrReleasedCharge(t *testing.T) {
	c, _, g, p, provider := launchFixture(t)
	sink := c.checkpoint.(*checkpointFixtureWriter).sink
	sink.mu.Lock()
	sink.fail = true
	sink.mu.Unlock()
	r, e := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), g)
	if !errors.Is(e, ErrUnknown) || !r.LaunchRequested || !r.ResourcesHeld || r.ProcessPID != 0 {
		t.Fatal(r, e)
	}
	sink.mu.Lock()
	sink.fail = false
	sink.mu.Unlock()
	if _, e = c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, r.Revision, launchSpec(), g); !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	if p.starts != 0 || provider.deletes != 0 {
		t.Fatal("unconfirmed intent caused effects")
	}
}
func TestGuestStartShortCallerDeadlineDoesNotShortenVMProcessBudget(t *testing.T) {
	c, _, g, p, _ := launchFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, e := c.StartGuest(ctx, p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), g); e != nil {
		t.Fatal(e)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.timeoutMS < 50000 || p.timeoutMS > 60000 {
		t.Fatal("caller ACK deadline became process lifetime", p.timeoutMS)
	}
}
func TestGuestStartRefusesTransportNotIssuedByExactController(t *testing.T) {
	c, _, g, p, _ := launchFixture(t)
	forged := openGuest(t, g.config)
	if _, e := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), forged); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if p.starts != 0 {
		t.Fatal("unowned transport started")
	}
}
