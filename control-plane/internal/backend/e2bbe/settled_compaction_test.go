package e2bbe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestSettledCloudAttemptsCompactWithoutRefundOrReplay(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	cfg.Limits.EstimatedCostMicros = 400
	c := openController(t, cfg)
	requests := make([]Request, 4)
	for i := range requests {
		req := taskRequest("project", fmt.Sprintf("task-%d", i))
		requests[i] = req
		r, err := c.Allocate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		r, err = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
		requireAbsent(t, r, err)
		if _, err = c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "no-result"); err != nil {
			t.Fatal(err)
		}
	}
	c.mu.Lock()
	prior, err := json.Marshal(c.journal)
	if err == nil {
		err = c.compactSettledLocked(context.Background(), 4)
	}
	settled := c.journal.Settled
	remaining := len(c.journal.Records)
	c.mu.Unlock()
	if err != nil || len(settled) != 2 || remaining != 2 {
		t.Fatal(err, len(settled), remaining)
	}
	if len(sink.rows) == 0 || string(sink.rows[len(sink.rows)-2]) != string(prior) {
		t.Fatal("complete pre-compaction journal was not acknowledged off-host")
	}
	var archived Request
	for _, req := range requests {
		if _, ok := settled[req.Identity.OperationID]; ok {
			archived = req
			break
		}
	}
	if archived.Identity.OperationID == "" {
		t.Fatal("no exact archived operation")
	}
	if _, err := c.Allocate(context.Background(), archived); !errors.Is(err, ErrUnknown) {
		t.Fatal("archived operation replayed", err)
	}
	duplicate := archived
	duplicate.Identity.OperationID = "new-operation"
	if _, err := c.Allocate(context.Background(), duplicate); !errors.Is(err, ErrConflict) {
		t.Fatal("archived attempt replayed under a new operation", err)
	}
	if _, usage := c.Snapshot(); usage.Active != 0 || usage.EstimatedCostMicros != 400 {
		t.Fatal("settled cloud spending was refunded", usage)
	}
	if _, err := c.Allocate(context.Background(), taskRequest("project", "extra")); !errors.Is(err, ErrCapacity) {
		t.Fatal("archived spending escaped configured budget", err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	c = openController(t, cfg)
	if _, err := c.Allocate(context.Background(), archived); !errors.Is(err, ErrUnknown) {
		t.Fatal("reopen lost archived no-replay marker", err)
	}
	if _, usage := c.Snapshot(); usage.EstimatedCostMicros != 400 {
		t.Fatal("reopen lost settled spending", usage)
	}
}

func TestUnjoinedAbsentCloudAttemptCannotBeCompacted(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	c := openController(t, cfg)
	req := taskRequest("project", "task")
	r, err := c.Allocate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	r, err = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
	requireAbsent(t, r, err)
	before := len(sink.rows)
	c.mu.Lock()
	err = c.compactSettledLocked(context.Background(), 1)
	_, kept := c.journal.Records[req.Identity.OperationID]
	settled := len(c.journal.Settled)
	c.mu.Unlock()
	if !errors.Is(err, ErrCapacity) || !kept || settled != 0 || len(sink.rows) != before {
		t.Fatal("unjoined VM history was removed", err, kept, settled)
	}
}

func TestCompactedHistoryNeedsOffHostAcknowledgementBeforeAnotherCreate(t *testing.T) {
	p := newProvider(t)
	cfg, sink := fixtureConfig(t, p)
	c := openController(t, cfg)
	for i := 0; i < 2; i++ {
		req := taskRequest("project", fmt.Sprintf("task-%d", i))
		r, err := c.Allocate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		r, err = c.Cancel(context.Background(), req.Identity.OperationID, r.Revision)
		requireAbsent(t, r, err)
		if _, err = c.AcknowledgeProductOutcome(context.Background(), req.Identity.OperationID, r.Revision, "no-result"); err != nil {
			t.Fatal(err)
		}
	}
	sink.fail = true
	c.mu.Lock()
	err := c.compactSettledLocked(context.Background(), 2)
	c.mu.Unlock()
	if !errors.Is(err, ErrUnknown) {
		t.Fatal("unacknowledged rollover was treated as durable", err)
	}
	if _, err = c.Allocate(context.Background(), taskRequest("project", "third")); !errors.Is(err, ErrUnknown) || p.creates != 2 {
		t.Fatal("provider Create followed uncertain rollover", err, p.creates)
	}
}
