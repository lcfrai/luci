package e2bbe

import "context"

func validProductOutcome(r Record, outcome string) bool {
	switch outcome {
	case "cancelled":
		return true
	case "no-result":
		return r.Capture.ArchiveSHA256 == ""
	case "handoff":
		return ValidateNativeExitReceipt(r.NativeExit, r) == nil &&
			ValidateCaptureReceipt(r.Capture, r.LaunchKernel, r) == nil
	}
	return false
}

// AcknowledgeProductOutcome only records a project-run join that already
// succeeded. It has no provider request, allocation, source write or billing
// effect. A lost journal-checkpoint reply is reconciled by persisting the same
// immutable outcome again, never by changing the product outcome.
func (c *Controller) AcknowledgeProductOutcome(ctx context.Context, op string, expectedRevision int, outcome string) (Record, error) {
	ctx, done, err := c.begin(ctx, op)
	if err != nil {
		return Record{}, err
	}
	defer done()
	r, err := c.record(op)
	if err != nil {
		return r, err
	}
	if r.State != Absent || r.ResourcesHeld || !validProductOutcome(r, outcome) ||
		r.ProductOutcome != "" && r.ProductOutcome != outcome {
		return r, ErrConflict
	}
	if r.ProductOutcome == outcome {
		c.mu.Lock()
		defer c.mu.Unlock()
		current := c.journal.Records[op]
		if current.Revision != r.Revision || current.ProductOutcome != outcome {
			return current, ErrConflict
		}
		return current, c.persist(ctx)
	}
	if r.Revision != expectedRevision {
		return r, ErrConflict
	}
	r.ProductOutcome = outcome
	return c.update(ctx, r)
}
