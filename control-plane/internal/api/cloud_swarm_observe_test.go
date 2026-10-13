package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestCloudRetirementDoesNotInventProductCancellation(t *testing.T) {
	record := e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{
		AttemptID: "attempt-one", OperationID: "child-one",
	}}, State: e2bbe.Absent, CancelRequested: true}
	run := product.ProjectRun{State: "ready", Operations: []product.ProjectRunOperation{{
		ID: "attempt-one", ChildID: "child-one", Status: "review-required",
	}}}
	if cloudProductTaskCancelled(run, record) {
		t.Fatal("normal VM retirement discarded a captured task result")
	}
	run.Operations[0].Status = "cancelled"
	if !cloudProductTaskCancelled(run, record) {
		t.Fatal("the exact cancelled operation was not recognized")
	}
	run.Operations[0].ID = "another-attempt"
	if cloudProductTaskCancelled(run, record) {
		t.Fatal("another attempt cancelled this task")
	}
	run.State = "cancelled"
	if !cloudProductTaskCancelled(run, record) {
		t.Fatal("run-level cancellation was lost")
	}
}

func TestCloudObservationBatchAllowsOtherProjectWhileFirstGuestIsSlow(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	finished := make(chan error, 1)
	rows := []e2bbe.Record{
		{Request: e2bbe.Request{Identity: e2bbe.Identity{ProjectID: "project-a"}}},
		{Request: e2bbe.Request{Identity: e2bbe.Identity{ProjectID: "project-b"}}},
	}
	go func() {
		finished <- runCloudObservationBatch(context.Background(), rows, func(ctx context.Context, row e2bbe.Record) error {
			started <- row.Request.Identity.ProjectID
			if row.Request.Identity.ProjectID == "project-a" {
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}()
	defer unblock()
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case project := <-started:
			seen[project] = true
		case <-time.After(2 * time.Second):
			t.Fatal("slow guest prevented an independent project observation")
		}
	}
	select {
	case err := <-finished:
		t.Fatal("batch finished before the slow guest settled", err)
	default:
	}
	unblock()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bounded batch did not settle")
	}
}

func TestCloudCompletionScanSkipsSettledHistoryButKeepsPendingJoins(t *testing.T) {
	rows := make([]e2bbe.Record, 256)
	for i := range rows {
		rows[i] = e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{
			ProjectID: "project-one", RunID: "run-one", OperationID: "settled",
		}}, State: e2bbe.Absent, ProductOutcome: "handoff"}
	}
	rows[100].Request.Identity.OperationID = "pending-join"
	rows[100].ProductOutcome = ""
	rows[200].Request.Identity.OperationID = "running"
	rows[200].State = e2bbe.Allocated
	rows[200].ProductOutcome = ""
	rows[230].Request.Identity.OperationID = "foreign-unbound"
	rows[230].Request.Identity.RunID = ""
	got := cloudObservationCandidates(rows)
	if len(got) != 2 || got[0].Request.Identity.OperationID != "pending-join" || got[1].Request.Identity.OperationID != "running" {
		t.Fatal("settled history delayed exact active or pending work", got)
	}
}
