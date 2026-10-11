package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const guestLaunchPath = "/luci-work/attempt/launch-kernel.json"
const guestCapturePath = "/luci-transfer/output.json"

func (g *GuestTransport) readLaunchKernel(ctx context.Context, r Record) (CaptureIdentity, error) {
	var id CaptureIdentity
	ctx, done, err := g.beginIO(ctx)
	if err != nil {
		return id, err
	}
	defer done()
	if err = g.guard(ctx, "observe-launch"); err != nil {
		return id, err
	}
	response, err := g.fileRequest(ctx, http.MethodGet, guestLaunchPath, nil, 0)
	if err != nil {
		return id, err
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(contextReader{ctx, response.Body}, 4097))
	if err != nil || len(b) > 4096 {
		return id, ErrUnknown
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&id) != nil || dec.Decode(new(any)) != io.EOF || ValidateCaptureIdentity(id, r) != nil {
		return CaptureIdentity{}, ErrConflict
	}
	if err = g.guard(ctx, "observe-launch"); err != nil {
		return CaptureIdentity{}, err
	}
	return id, nil
}

// ObserveLaunchKernel reads the template-protected identity only after envd's
// exact Start PID acknowledgement and checkpoints it before any native outcome
// can be recorded. It never accepts kernel identity introduced by a collector.
func (c *Controller) ObserveLaunchKernel(ctx context.Context, op string, g *GuestTransport) (Record, error) {
	if g == nil || g.owner != c {
		return Record{}, ErrUnsupported
	}
	ctx, done, err := c.begin(ctx, op)
	if err != nil {
		return Record{}, err
	}
	defer done()
	r, err := c.record(op)
	if err != nil {
		return r, err
	}
	if err = c.config.Authorize(ctx, r.Request, "observe-launch"); err != nil {
		return r, err
	}
	if !sameActiveLaunch(r, g) {
		return r, ErrConflict
	}
	if r.LaunchKernel != (CaptureIdentity{}) {
		if ValidateCaptureIdentity(r.LaunchKernel, r) != nil {
			return r, ErrConflict
		}
		return c.confirmObservationCheckpoint(ctx, r)
	}
	id, err := g.readLaunchKernel(ctx, r)
	if err != nil {
		return r, err
	}
	current, err := c.record(op)
	if err != nil {
		return current, err
	}
	if current.CancelRequested || !sameActiveLaunch(current, g) || current.LaunchDigest != r.LaunchDigest || current.ProcessPID != r.ProcessPID {
		return current, ErrConflict
	}
	if err = c.config.Authorize(ctx, current.Request, "observe-launch"); err != nil {
		return current, err
	}
	current.LaunchKernel = id
	return c.update(ctx, current)
}

func sameActiveLaunch(r Record, g *GuestTransport) bool {
	return r.State == Allocated && !r.CancelRequested && r.ResourcesHeld && r.LaunchRequested && r.ProcessPID != 0 &&
		digestPattern.MatchString(r.LaunchDigest) && r.SandboxID == g.config.Record.SandboxID && r.RequestDigest == g.config.Record.RequestDigest && r.Deployment == g.config.Record.Deployment && r.Template == g.config.Record.Template && r.CreatedAt.Equal(g.config.Record.CreatedAt)
}

// ObserveNativeExit reads existing sandboxd frames without sending stdin or
// another command. Its receipt is independent of collector completion. Neither
// native exit zero nor a disconnected observer releases a VM or its charge.
func (c *Controller) ObserveNativeExit(ctx context.Context, op string, g *GuestTransport, token string) (Record, error) {
	if g == nil || g.owner != c {
		return Record{}, ErrUnsupported
	}
	ctx, done, err := c.begin(ctx, op)
	if err != nil {
		return Record{}, err
	}
	defer done()
	r, err := c.record(op)
	if err != nil {
		return r, err
	}
	if err = c.config.Authorize(ctx, r.Request, "observe-native-exit"); err != nil {
		return r, err
	}
	if !sameActiveLaunch(r, g) || ValidateCaptureIdentity(r.LaunchKernel, r) != nil {
		return r, ErrConflict
	}
	if r.NativeExit != (NativeExitReceipt{}) {
		if ValidateNativeExitReceipt(r.NativeExit, r) != nil {
			return r, ErrConflict
		}
		return c.confirmObservationCheckpoint(ctx, r)
	}
	// A bounded observer owns no mutation transport. Reviewed cancellation can
	// wake this read; Close also cancels it while retaining the controller lock.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.mu.Lock()
	c.nativeObservers[op] = cancel
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.nativeObservers, op); c.mu.Unlock() }()
	// Cancel may have arrived immediately before this observer was registered.
	current, _ := c.record(op)
	if current.CancelRequested {
		cancel()
	}
	receipt, observeErr := g.readNativeExit(ctx, r, token)
	current, err = c.record(op)
	if err != nil {
		return current, err
	}
	if current.CancelRequested {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return current, ErrClosed
		}
		settle, stop := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stop()
		return c.observe(settle, current, true)
	}
	if observeErr != nil {
		return current, observeErr
	}
	if !sameActiveLaunch(current, g) || current.LaunchKernel != r.LaunchKernel {
		return current, ErrConflict
	}
	if err = c.config.Authorize(ctx, current.Request, "observe-native-exit"); err != nil {
		return current, err
	}
	current.NativeExit = receipt
	return c.update(ctx, current)
}

func (g *GuestTransport) readNativeExit(ctx context.Context, r Record, token string) (NativeExitReceipt, error) {
	var receipt NativeExitReceipt
	ctx, done, err := g.beginIO(ctx)
	if err != nil {
		return receipt, err
	}
	defer done()
	if err = g.guard(ctx, "observe-native-exit"); err != nil {
		return receipt, err
	}
	conn, err := g.Attach(ctx, token)
	if err != nil {
		return receipt, err
	}
	defer conn.CloseNow()
	var observedBytes int64
	for {
		kind, b, err := conn.Read(ctx)
		if err != nil {
			return receipt, ErrUnknown
		}
		observedBytes += int64(len(b))
		if kind != websocket.MessageBinary || len(b) == 0 || observedBytes > 64<<20 {
			return receipt, ErrUnknown
		}
		switch b[0] {
		case 0x00: // existing sandboxd stdout/replay; never inspect text as status
			continue
		case 0x03:
			receipt = NativeExitReceipt{Version: 1, SandboxID: r.SandboxID, RequestDigest: r.RequestDigest, LaunchDigest: r.LaunchDigest, ProcessPID: r.ProcessPID, FrameSHA256: bytesDigest(b), Reason: string(b[1:]), ObservedAt: time.Now().UTC()}
			if ValidateNativeExitReceipt(receipt, r) != nil {
				return NativeExitReceipt{}, ErrConflict
			}
			if err = g.guard(ctx, "observe-native-exit"); err != nil {
				return NativeExitReceipt{}, err
			}
			return receipt, nil
		default:
			return receipt, ErrUnknown
		}
	}
}

// An earlier observation may have reached the local journal but failed its
// external checkpoint acknowledgement. Repeated readback never silently
// upgrades that uncertainty; it first reasserts the same immutable bytes.
func (c *Controller) confirmObservationCheckpoint(ctx context.Context, r Record) (Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.journal.Records[r.Request.Identity.OperationID]
	if !ok || current.Revision != r.Revision || current.RequestDigest != r.RequestDigest || current.CancelRequested {
		return current, ErrConflict
	}
	return current, c.persist(ctx)
}
