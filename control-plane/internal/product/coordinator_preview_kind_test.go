package product

import "testing"

func TestCoordinatorPreviewKindCannotInheritLegacyApproval(t *testing.T) {
	legacy := &CoordinatorPreviewPlan{Dockerfile: "Dockerfile", Port: 3000, AcknowledgePreviewBuild: true}
	source := &CoordinatorPreviewPlan{Kind: "reviewed-source", Dockerfile: "", Port: 3000, AcknowledgePreviewBuild: true}
	if validateCoordinatorPreviewPlan(legacy) != nil || validateCoordinatorPreviewPlan(source) != nil {
		t.Fatal("both explicit plans should be valid")
	}
	if !coordinatorPreviewPolicyMatches("checked-source", legacy) || !coordinatorPreviewPolicyMatches("reviewed-source", source) {
		t.Fatal("each plan must match its own reviewed policy")
	}
	if coordinatorPreviewPolicyMatches("checked-source", source) || coordinatorPreviewPolicyMatches("reviewed-source", legacy) || coordinatorPreviewPolicyMatches("manual", source) {
		t.Fatal("one preview authority must not be reused as the other")
	}
	for _, bad := range []CoordinatorPreviewPlan{
		{Kind: "reviewed-source", Dockerfile: "Dockerfile", Port: 3000, AcknowledgePreviewBuild: true},
		{Kind: "unknown", Dockerfile: "", Port: 3000, AcknowledgePreviewBuild: true},
		{Dockerfile: "", Port: 3000, AcknowledgePreviewBuild: true},
	} {
		if validateCoordinatorPreviewPlan(&bad) == nil {
			t.Fatalf("accepted kind/dockefile substitution: %+v", bad)
		}
	}
	if reviewDigest(legacy) == reviewDigest(source) {
		t.Fatal("origin digest must bind the explicit kind")
	}
	if !coordinatorPreviewRequestMatchesPlan(source, 0, 3000, "reviewed-source", "") ||
		coordinatorPreviewRequestMatchesPlan(source, 0, 3000, "", "") ||
		coordinatorPreviewRequestMatchesPlan(source, 0, 3000, "reviewed-source", "Dockerfile") ||
		coordinatorPreviewRequestMatchesPlan(source, 1, 3000, "reviewed-source", "") ||
		coordinatorPreviewRequestMatchesPlan(legacy, 0, 3000, "reviewed-source", "Dockerfile") ||
		!coordinatorPreviewRequestMatchesPlan(legacy, 0, 3000, "", "Dockerfile") {
		t.Fatal("release request did not retain its exact kind, config revision, port and Dockerfile binding")
	}
}
