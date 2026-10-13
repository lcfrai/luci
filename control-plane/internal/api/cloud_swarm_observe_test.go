package api

import (
	"testing"

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
