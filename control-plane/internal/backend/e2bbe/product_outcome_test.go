package e2bbe

import (
	"context"
	"errors"
	"testing"
)

func TestProductOutcomeSkipsOnlyExactlySettledCloudAttemptAfterRestart(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	req.Identity.RunID = "run-one"
	r, err := c.Allocate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "no-result"); !errors.Is(err, ErrConflict) {
		t.Fatal("live charged VM acquired a product outcome", err)
	}
	r, err = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
	if err != nil || r.State != Absent || r.ResourcesHeld {
		t.Fatal(r, err)
	}
	if _, err = c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "handoff"); !errors.Is(err, ErrConflict) {
		t.Fatal("missing verified output acquired a handoff", err)
	}
	confirmed, err := c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "no-result")
	if err != nil || confirmed.ProductOutcome != "no-result" || p.creates != 1 || p.deletes != 1 {
		t.Fatal(confirmed, err, p.creates, p.deletes)
	}
	again, err := c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "no-result")
	if err != nil || again.Revision != confirmed.Revision {
		t.Fatal("same product join changed history", again, err)
	}
	if _, err = c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, confirmed.Revision, "cancelled"); !errors.Is(err, ErrConflict) {
		t.Fatal("a conflicting product result replaced the first join", err)
	}
	if err = c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	c = openController(t, cfg)
	rows, usage := c.Snapshot()
	if len(rows) != 1 || rows[0].ProductOutcome != "no-result" || usage.Active != 0 || p.creates != 1 || p.deletes != 1 {
		t.Fatal("exact product join was not durable", rows, usage)
	}
}
