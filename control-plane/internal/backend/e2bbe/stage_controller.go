package e2bbe

import (
	"context"
	"io"
)

// StageGuestInput records the exact source/context bundle off the replaceable
// runtime host, then checkpoints a one-shot upload intent before POST. A lost
// reply may be reconciled by GET, never by another upload or VM allocation.
func (c *Controller) StageGuestInput(ctx context.Context, op string, g *GuestTransport, store *DirectoryStore, input io.ReadSeeker) (Record, error) {
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
	if r.State != Allocated || r.CancelRequested || !r.ResourcesHeld || r.LaunchRequested ||
		r.SandboxID != g.config.Record.SandboxID || r.RequestDigest != g.config.Record.RequestDigest || r.Deployment != g.config.Record.Deployment {
		return r, ErrConflict
	}
	if err = c.config.Authorize(ctx, r.Request, "stage-input"); err != nil {
		return r, err
	}
	binding := TransferBinding{Identity: r.Request.Identity, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, Direction: "input"}
	if !r.InputUploadRequested {
		if input == nil {
			return r, ErrTransfer
		}
		if _, err = input.Seek(0, io.SeekStart); err != nil {
			return r, err
		}
		if _, err = store.PutTransfer(ctx, binding, r.Request.BundleDigest, input); err != nil {
			return r, err
		}
		if _, err = input.Seek(0, io.SeekStart); err != nil {
			return r, err
		}
		current, readErr := c.record(op)
		if readErr != nil || current.Revision != r.Revision || current.RequestDigest != r.RequestDigest || current.State != Allocated {
			return current, ErrConflict
		}
		if err = c.config.Authorize(ctx, current.Request, "stage-input"); err != nil {
			return current, err
		}
		current.InputUploadRequested = true
		r, err = c.update(ctx, current)
		if err != nil {
			return r, err
		}
		_, _ = g.StageInput(ctx, input)
	}
	// The input also has to remain in durable controller storage. A guest-only
	// upload is insufficient for return-diff validation after host loss.
	if _, err = store.CopyTransfer(ctx, binding, r.Request.BundleDigest, io.Discard); err != nil {
		return r, ErrUnknown
	}
	if _, err = g.verifyInput(ctx); err != nil {
		return r, ErrUnknown
	}
	current, err := c.record(op)
	if err != nil || current.Revision != r.Revision || !current.InputUploadRequested || current.RequestDigest != r.RequestDigest {
		return current, ErrConflict
	}
	if err = c.config.Authorize(ctx, current.Request, "stage-input"); err != nil {
		return current, err
	}
	return c.confirmObservationCheckpoint(ctx, current)
}
