package e2bbe

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	process "github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess/processconnect"
)

// LaunchSpec comes only from the trusted upstream adapter/connection policy.
// It is never deserialized from a task context archive or returned model output.
// Secrets and the attach token are transient; the existing registry owns their
// persistence. ConnectionRevision binds the selected credential configuration
// without hashing or recording secret values in the allocation journal.
type LaunchSpec struct {
	Command            string            `json:"-"`
	Args               []string          `json:"-"`
	Environment        map[string]string `json:"-"`
	AttachToken        string            `json:"-"`
	ConnectionRevision string            `json:"-"`
}

func (LaunchSpec) String() string { return "trusted upstream launch (private configuration redacted)" }
func (LaunchSpec) GoString() string {
	return "trusted upstream launch (private configuration redacted)"
}
func validLaunch(l LaunchSpec) bool {
	if (l.Command != "codex" && l.Command != "claude") || !validToken(l.AttachToken) || !validID(l.ConnectionRevision) || len(l.Args) > 256 || len(l.Environment) > 16 {
		return false
	}
	size := len(l.AttachToken)
	for _, arg := range l.Args {
		size += len(arg)
		if strings.ContainsRune(arg, 0) {
			return false
		}
	}
	for k, v := range l.Environment {
		switch k {
		case "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_BASE_URL", "ANTHROPIC_BASE_URL", "OPENAI_ORG_ID", "OPENAI_PROJECT_ID":
		default:
			return false
		}
		size += len(k) + len(v)
		if strings.ContainsRune(v, 0) {
			return false
		}
	}
	return size <= 64<<10
}
func launchDigest(r Record, l LaunchSpec) string {
	return digest(struct {
		Request, Command string
		Args             []string
		Connection       string
	}{r.RequestDigest, l.Command, l.Args, l.ConnectionRevision})
}
func bootstrapArgs(r Record) []string {
	i := r.Request.Identity
	return []string{"--archive", guestInputPath, "--archive-sha256", r.Request.BundleDigest, "--owner", i.OwnerID, "--project", i.ProjectID, "--task", i.TaskID, "--attempt", i.AttemptID, "--operation", i.OperationID, "--source-sha256", r.Request.SourceDigest, "--context-sha256", r.Request.ContextDigest, "--sandbox", r.SandboxID, "--request-sha256", r.RequestDigest, "--launch-sha256", r.LaunchDigest}
}
func (g *GuestTransport) start(ctx context.Context, r Record, l LaunchSpec) (uint32, error) {
	ctx, done, e := g.beginIO(ctx)
	if e != nil {
		return 0, e
	}
	defer done()
	if !validLaunch(l) {
		return 0, ErrUnsupported
	}
	if !r.LaunchRequested || r.ProcessPID != 0 || r.LaunchDigest != launchDigest(r, l) ||
		r.SandboxID != g.config.Record.SandboxID || r.RequestDigest != g.config.Record.RequestDigest ||
		r.Deployment != g.config.Record.Deployment {
		return 0, ErrConflict
	}
	if e := g.guard(ctx, "start"); e != nil {
		return 0, e
	}
	// Pin process lifetime to the allocation expiry. A short ACK deadline must
	// not become envd's process lifetime via Connect-Timeout-Ms. Canceling this
	// stream detaches the observer; the pinned envd starts a bounded independent
	// process. VM cancellation remains an explicit exact-owned controller action.
	caller := ctx
	ctx, cancel := context.WithDeadline(context.WithoutCancel(ctx), r.CreatedAt.Add(time.Duration(r.Request.MaxSeconds)*time.Second))
	defer cancel()
	stop := context.AfterFunc(caller, cancel)
	defer stop()
	timer := time.AfterFunc(15*time.Second, cancel)
	defer timer.Stop()
	env := make(map[string]string, len(l.Environment)+3)
	for k, v := range l.Environment {
		env[k] = v
	}
	args, _ := json.Marshal(l.Args)
	if l.Args == nil {
		args = []byte("[]")
	}
	env[backend.EnvCommand] = l.Command
	env[backend.EnvArgsJSON] = string(args)
	env[backend.EnvToken] = l.AttachToken
	cwd := "/luci-transfer"
	tag := r.Request.Identity.OperationID
	stdin := false
	req := connect.NewRequest(&process.StartRequest{Process: &process.ProcessConfig{Cmd: "/usr/local/bin/luci-microvm-bootstrap", Args: bootstrapArgs(r), Envs: env, Cwd: &cwd}, Tag: &tag, Stdin: &stdin})
	req.Header().Set("Authorization", "Basic cm9vdDo=")
	req.Header().Set("X-Access-Token", g.config.EnvdAccessToken)
	req.Header().Set("e2b-traffic-access-token", g.config.TrafficAccessToken)
	client := processconnect.NewProcessClient(g.attach, g.envd.String(), connect.WithReadMaxBytes(backend.MaxMessageBytes), connect.WithSendMaxBytes(128<<10))
	stream, e := client.Start(ctx, req)
	if e != nil {
		return 0, ErrUnknown
	}
	defer stream.Close()
	// Start must be the first event. No output/end/duplicate stream can stand in
	// for its exact PID acknowledgement, and the library never replays the call.
	if !stream.Receive() {
		return 0, ErrUnknown
	}
	msg := stream.Msg()
	if msg == nil || msg.Event == nil || msg.Event.GetStart() == nil || msg.Event.GetStart().Pid == 0 {
		return 0, ErrUnknown
	}
	return msg.Event.GetStart().Pid, nil
}

// StartGuest persists/checkpoints intent before the one permitted Start RPC.
// A lost ACK cannot replay, even across controller restart. A PID only confirms
// envd fork: sandboxd readiness, upstream native exit and verified results are
// separate observations. Start never releases a VM or claims billing settled.
func (c *Controller) StartGuest(ctx context.Context, op string, expectedRevision int, l LaunchSpec, g *GuestTransport) (Record, error) {
	if !validLaunch(l) || g == nil || g.owner != c {
		return Record{}, ErrUnsupported
	}
	ctx, done, e := c.begin(ctx, op)
	if e != nil {
		return Record{}, e
	}
	defer done()
	r, e := c.record(op)
	if e != nil {
		return r, e
	}
	if e = c.config.Authorize(ctx, r.Request, "start"); e != nil {
		return r, e
	}
	if r.State != Allocated || r.CancelRequested || !r.ResourcesHeld || r.Deployment != c.config.Deployment || g.config.Record.SandboxID != r.SandboxID || g.config.Record.RequestDigest != r.RequestDigest || g.config.Record.Deployment != r.Deployment {
		return r, ErrConflict
	}
	hash := launchDigest(r, l)
	if r.LaunchRequested {
		if r.LaunchDigest != hash {
			return r, ErrConflict
		}
		if r.ProcessPID != 0 {
			return r, nil
		}
		return r, ErrUnknown
	}
	if r.Revision != expectedRevision {
		return r, ErrConflict
	}
	if _, e = g.verifyInput(ctx); e != nil {
		return r, e
	}
	if e = c.config.Authorize(ctx, r.Request, "start"); e != nil {
		return r, e
	}
	r.LaunchRequested, r.LaunchDigest = true, hash
	r, e = c.update(ctx, r)
	if e != nil {
		return r, e
	}
	pid, startErr := g.start(ctx, r, l)
	current, readErr := c.record(op)
	if readErr != nil {
		return current, readErr
	}
	r = current
	if startErr != nil {
		return r, ErrUnknown
	}
	r.ProcessPID = pid
	r, e = c.update(context.WithoutCancel(ctx), r)
	if e != nil {
		return r, e
	}
	if r.CancelRequested {
		return c.observe(ctx, r, true)
	}
	return r, nil
}
