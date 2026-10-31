package product

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

func TestKubernetesPrivateJournalRetainsOriginalActorCASAndIrreversibleAttemptMarker(t *testing.T) {
	s, p, cloud, _ := azureBlueprintFixture(t)
	plan, err := deploymentplan.Compile(cloud.Foundation.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request := deploymentkube.Request{Identity: deploymentkube.Identity{ProjectID: p.ID, ActorID: "alice", TargetID: "cluster-one", TargetFingerprint: reviewDigest("cluster-one"), OperationID: "workload-one", PlanDigest: plan.Digest}, Blueprint: plan.Blueprint, ApprovalReference: "original-reviewed-action", Expected: []deploymentkube.ExpectedResource{}}
	op := deploymentkube.Operation{Revision: 1, State: "reserved", NamespaceUID: "original-namespace", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Resources: []deploymentkube.ResourceReceipt{}}
	for _, artifact := range plan.Artifacts {
		var resource struct {
			Kind     string
			Metadata struct{ Name string }
		}
		if json.Unmarshal([]byte(artifact.Content), &resource) != nil {
			t.Fatal("bad compiler output")
		}
		expected := deploymentkube.ExpectedResource{Kind: resource.Kind, Name: resource.Metadata.Name}
		request.Expected = append(request.Expected, expected)
		op.Resources = append(op.Resources, deploymentkube.ResourceReceipt{ExpectedResource: expected, DesiredSHA256: artifact.SHA256})
	}
	op.Request, op.RequestDigest = request, reviewDigest(request)
	j := &kubernetesOperationJournal{server: s}
	if _, fresh, err := j.Reserve(context.Background(), op); err != nil || !fresh {
		t.Fatal("durable reservation failed", err)
	}
	if _, fresh, err := j.Reserve(context.Background(), op); err != nil || fresh {
		t.Fatal("reservation duplicated", err)
	}
	foreign := op
	foreign.Request.Identity.ActorID = "bob"
	foreign.RequestDigest = reviewDigest(foreign.Request)
	if _, _, err := j.Reserve(context.Background(), foreign); err == nil {
		t.Fatal("changing actor created or adopted another original operation")
	}
	op.Revision, op.State, op.Resources[0].Attempted = 2, "unknown", true
	if err := j.CompareAndSwap(context.Background(), op, 1); err != nil {
		t.Fatal(err)
	}
	// Read from a fresh journal wrapper, not an in-memory operation cache.
	j = &kubernetesOperationJournal{server: s}
	read, err := j.Load(context.Background(), request.Identity)
	if err != nil || !read.Resources[0].Attempted || read.Revision != 2 {
		t.Fatal("attempt marker did not survive readback", err)
	}
	read.Revision, read.Resources[0].Attempted = 3, false
	if err := j.CompareAndSwap(context.Background(), read, 2); err == nil {
		t.Fatal("attempt marker reset allowed replay")
	}
	if err := j.CompareAndSwap(context.Background(), op, 1); err == nil {
		t.Fatal("stale revision overwrote operation")
	}
	if len(s.jobs) != 0 || len(s.plans) != 0 {
		t.Fatal("private journal created provider work")
	}
}
