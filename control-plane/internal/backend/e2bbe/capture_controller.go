package e2bbe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"connectrpc.com/connect"
	process "github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess/processconnect"
)

// CollectGuestOutput invokes the fixed root collector at most once for the
// previously observed native attempt. A lost collector response is reconciled
// from its exact protected receipt and output bytes; it never sends Start
// again. Persisted output is still untrusted coding work until the product
// reviews it against the original source/context and runs independent checks.
func (c *Controller) CollectGuestOutput(ctx context.Context, op string, g *GuestTransport, store *DirectoryStore) (Record, error) {
	if g == nil || g.owner != c || store == nil {
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
	if !sameActiveLaunch(r, g) || ValidateCaptureIdentity(r.LaunchKernel, r) != nil || ValidateNativeExitReceipt(r.NativeExit, r) != nil {
		return r, ErrConflict
	}
	if err = c.config.Authorize(ctx, r.Request, "capture-output"); err != nil {
		return r, err
	}
	if r.Capture != (CaptureReceipt{}) {
		if ValidateCaptureReceipt(r.Capture, r.LaunchKernel, r) != nil {
			return r, ErrConflict
		}
		binding := TransferBinding{Identity: r.Request.Identity, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, Direction: "output"}
		if _, err := store.CopyTransfer(ctx, binding, r.Capture.ArchiveSHA256, io.Discard); err != nil {
			return r, ErrUnknown
		}
		return c.confirmObservationCheckpoint(ctx, r)
	}
	if !r.CaptureRequested {
		r.CaptureRequested = true
		r, err = c.update(ctx, r)
		if err != nil {
			return r, err
		}
		// The collector may continue after a transport timeout. Only this fresh
		// reservation invokes it; subsequent calls use readback alone.
		_ = g.invokeCollector(ctx, r)
	}
	receipt, err := g.readCaptureReceipt(ctx, r)
	if err != nil {
		return r, ErrUnknown
	}
	manifest, err := g.ReceiveOutput(ctx, receipt.ArchiveSHA256, store)
	if err != nil || !captureManifestMatches(receipt, manifest) {
		return r, ErrUnknown
	}
	archiveBytes := &captureByteCounter{}
	binding := TransferBinding{Identity: r.Request.Identity, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, Direction: "output"}
	if _, err = store.CopyTransfer(ctx, binding, receipt.ArchiveSHA256, archiveBytes); err != nil || archiveBytes.count != receipt.ArchiveBytes {
		return r, ErrUnknown
	}
	current, err := c.record(op)
	if err != nil {
		return current, err
	}
	if !sameActiveLaunch(current, g) || !current.CaptureRequested || current.LaunchKernel != r.LaunchKernel || current.NativeExit != r.NativeExit {
		return current, ErrConflict
	}
	if err = c.config.Authorize(ctx, current.Request, "capture-output"); err != nil {
		return current, err
	}
	current.Capture = receipt
	return c.update(ctx, current)
}

type captureByteCounter struct{ count int64 }

func (c *captureByteCounter) Write(p []byte) (int, error) {
	c.count += int64(len(p))
	return len(p), nil
}

func captureManifestMatches(receipt CaptureReceipt, manifest TransferManifest) bool {
	source, changes := false, false
	for _, file := range manifest.Files {
		if file.Path == "output/source-manifest.json" && file.SHA256 == receipt.SourceManifestSHA256 {
			source = true
		}
		if file.Path == "output/changes.json" && file.SHA256 == receipt.ChangeManifestSHA256 {
			changes = true
		}
	}
	return source && changes
}

func (g *GuestTransport) invokeCollector(ctx context.Context, r Record) error {
	ctx, done, err := g.beginIO(ctx)
	if err != nil {
		return err
	}
	defer done()
	if err = g.guard(ctx, "capture-output"); err != nil {
		return err
	}
	identity, _ := json.Marshal(r.LaunchKernel)
	encoded := base64.StdEncoding.EncodeToString(identity)
	cwd, tag, stdin := "/luci-transfer", r.Request.Identity.OperationID+"-capture", false
	req := connect.NewRequest(&process.StartRequest{Process: &process.ProcessConfig{Cmd: "/usr/local/bin/luci-microvm-collect", Args: []string{"--identity-base64", encoded}, Cwd: &cwd}, Tag: &tag, Stdin: &stdin})
	req.Header().Set("Authorization", "Basic cm9vdDo=")
	req.Header().Set("X-Access-Token", g.config.EnvdAccessToken)
	req.Header().Set("e2b-traffic-access-token", g.config.TrafficAccessToken)
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := processconnect.NewProcessClient(g.attach, g.envd.String(), connect.WithReadMaxBytes(64<<10), connect.WithSendMaxBytes(16<<10))
	stream, err := client.Start(bounded, req)
	if err != nil {
		return ErrUnknown
	}
	defer stream.Close()
	if !stream.Receive() || stream.Msg() == nil || stream.Msg().Event.GetStart() == nil || stream.Msg().Event.GetStart().Pid == 0 {
		return ErrUnknown
	}
	for stream.Receive() {
		msg := stream.Msg()
		if msg == nil || msg.Event == nil {
			return ErrUnknown
		}
		if end := msg.Event.GetEnd(); end != nil {
			if end.GetExitCode() != 0 {
				return ErrUnknown
			}
			return nil
		}
		if msg.Event.GetData() == nil && msg.Event.GetKeepalive() == nil {
			return ErrUnknown
		}
	}
	return ErrUnknown
}

func (g *GuestTransport) readCaptureReceipt(ctx context.Context, r Record) (CaptureReceipt, error) {
	var receipt CaptureReceipt
	ctx, done, err := g.beginIO(ctx)
	if err != nil {
		return receipt, err
	}
	defer done()
	if err = g.guard(ctx, "capture-output"); err != nil {
		return receipt, err
	}
	response, err := g.fileRequest(ctx, http.MethodGet, guestCapturePath, nil, 0)
	if err != nil {
		return receipt, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(contextReader{ctx, response.Body}, 8193))
	if err != nil || len(body) > 8192 {
		return receipt, ErrUnknown
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || ValidateCaptureReceipt(receipt, r.LaunchKernel, r) != nil {
		return CaptureReceipt{}, ErrConflict
	}
	if err = g.guard(ctx, "capture-output"); err != nil {
		return CaptureReceipt{}, err
	}
	return receipt, nil
}
