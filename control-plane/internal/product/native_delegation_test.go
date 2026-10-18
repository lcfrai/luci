package product

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func nativeFixtureImage() string { return "sha256:" + strings.Repeat("7", 64) }
func nativeFixtureRoute() ProjectRunRoute {
	return ProjectRunRoute{Harness: "codex", Model: "opaque-model", PermissionMode: "bypass", NativeDelegation: NativeDelegationCodexHelpers2, NativeDelegationImage: nativeFixtureImage()}
}
func nativeFixtureRun() ProjectRun {
	route := nativeFixtureRoute()
	c := &OrchestrationContract{State: "approved", Input: OrchestrationPreparedInput{Route: route, Tasks: []ProjectTask{{ID: "task-coding", Role: "coding"}}}}
	c.Digest = reviewDigest(c.Input); c.Approval = &OrchestrationApproval{Digest: c.Digest}
	return ProjectRun{Input: ProjectRunInput{Route: route, OrchestrationContract: c}, Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task-coding", Role: "coding", OperationID: "op-first"}}}, Operations: []ProjectRunOperation{{ID: "op-first", Kind: "session", TaskID: "task-coding", Status: "reserved", Allocation: "ready", Attempt: 1}}}
}

func TestNativeDelegationImmutableImageAndClosedRequest(t *testing.T) {
	for _, image := range []string{nativeFixtureImage(), "registry.example:5000/team/sandbox@" + nativeFixtureImage()} {
		if !ImmutableNativeDelegationImage(image) { t.Fatal("immutable syntax rejected", image) }
	}
	for _, image := range []string{"", "luci/sandbox:dev", "sha256:" + strings.Repeat("A", 64), "sha256:123", "https://example/image@" + nativeFixtureImage(), "repo@@" + nativeFixtureImage(), "repo/../image@" + nativeFixtureImage(), strings.Repeat("a", 256) + "@" + nativeFixtureImage(), " " + nativeFixtureImage()} {
		t.Run(image, func(t *testing.T) { if ImmutableNativeDelegationImage(image) { t.Fatal("unsupported image accepted") } })
	}
	for _, mutate := range []func(*OrchestrationPrepareRequest){
		func(r *OrchestrationPrepareRequest) { r.Route.NativeDelegation = "other" },
		func(r *OrchestrationPrepareRequest) { r.Route.Harness = "claude-code" },
		func(r *OrchestrationPrepareRequest) { r.Route.PermissionMode = "ask" },
		func(r *OrchestrationPrepareRequest) { r.Route.ExecutionLocation = "cloud-microvm" },
	} {
		r := OrchestrationPrepareRequest{IdempotencyKey: "native-delegation-normalize-123", WorktreeID: "w-owned", Outcome: "preview", Route: OrchestrationRouteRequest{NativeDelegation: NativeDelegationCodexHelpers2, Harness: "codex", PermissionMode: "bypass", ExecutionLocation: "local-docker"}}
		mutate(&r); if normalizeOrchestrationPrepare(&r) == nil { t.Fatal("unsupported request normalized", r.Route) }
	}
}

func TestNativeDelegationLegacyBytesAndCapturedDigest(t *testing.T) {
	route := ProjectRunRoute{Harness: "codex", Model: "opaque-model", PermissionMode: "bypass"}
	legacy := struct {
		Harness string `json:"harness"`; Model string `json:"model"`; PermissionMode string `json:"permissionMode"`
		Effort string `json:"effort,omitempty"`; ExecutionLocation string `json:"executionLocation,omitempty"`; Connection AgentConnectionReceipt `json:"connection"`
	}{Harness: route.Harness, Model: route.Model, PermissionMode: route.PermissionMode}
	before, _ := json.Marshal(legacy); after, _ := json.Marshal(route)
	if string(before) != string(after) || reviewDigest(legacy) != reviewDigest(route) { t.Fatal("legacy route bytes/digest changed") }
	s := &Server{sandboxImage: nativeFixtureImage()}
	if err := s.captureNativeDelegation(NativeDelegationCodexHelpers2, &route); err != nil { t.Fatal(err) }
	if reviewDigest(route) == reviewDigest(legacy) || route.NativeDelegationImage != s.sandboxImage { t.Fatal("server profile/image not captured") }
	mutable := &Server{sandboxImage: "luci/sandbox:dev"}; beforeRoute := ProjectRunRoute{Harness: "codex", Model: "opaque-model", PermissionMode: "bypass"}; refused := beforeRoute
	if mutable.captureNativeDelegation(NativeDelegationCodexHelpers2, &refused) == nil || refused != beforeRoute { t.Fatal("mutable configured image produced a partial grant") }
	if ValidateNativeDelegationCurrent(route, "sha256:" + strings.Repeat("8", 64)) == nil { t.Fatal("changed image accepted") }
	for _, mutate := range []func(*ProjectRunRoute){func(r *ProjectRunRoute) { r.NativeDelegation = "" }, func(r *ProjectRunRoute) { r.NativeDelegationImage = "" }, func(r *ProjectRunRoute) { r.NativeDelegation = "unknown" }} {
		changed := route; mutate(&changed); if ValidateNativeDelegationRoute(changed) == nil { t.Fatal("partial/unknown pair accepted") }
	}
}

func TestNativeDelegationExactCodingOperationAndRepairBoundary(t *testing.T) {
	run := nativeFixtureRun()
	if got, err := NativeDelegationForOperation(run, "op-first", nativeFixtureImage()); err != nil || got != NativeDelegationCodexHelpers2 { t.Fatal(got, err) }
	for _, name := range []string{"checkpoint", "repeat", "reviewer", "unknown-role", "wrong-assignment", "starting", "wrong-attempt", "unapproved", "changed-image"} {
		t.Run(name, func(t *testing.T) {
			r := nativeFixtureRun(); expectedError := true
			switch name {
			case "checkpoint": r.Operations[0].Checkpoint = &ProjectSwarmCheckpoint{}; expectedError = false
			case "repeat":
				prior := r.Operations[0]; prior.ID = "op-prior"; prior.Status = "ended"
				r.Operations = append([]ProjectRunOperation{prior}, r.Operations...); r.Operations[1].Attempt = 2; expectedError = false
			case "reviewer":
				r.Swarm.Tasks[0].Role, r.Swarm.Tasks[0].ReviewOnly = "reviewer", true
				r.Input.OrchestrationContract.Input.Tasks[0].Role = "reviewer"
				r.Input.OrchestrationContract.Digest = reviewDigest(r.Input.OrchestrationContract.Input); r.Input.OrchestrationContract.Approval.Digest = r.Input.OrchestrationContract.Digest; expectedError = false
			case "unknown-role": r.Swarm.Tasks[0].Role = ""
			case "wrong-assignment": r.Swarm.Tasks[0].OperationID = "op-other"
			case "starting": r.Operations[0].Status = "starting"
			case "wrong-attempt": r.Operations[0].Attempt = 2
			case "unapproved": r.Input.OrchestrationContract.State = "prepared"
			case "changed-image": r.Input.Route.NativeDelegationImage = "sha256:" + strings.Repeat("8", 64)
			}
			got, err := NativeDelegationForOperation(r, "op-first", nativeFixtureImage())
			if got != "" || (err != nil) != expectedError { t.Fatal("role/attempt boundary widened", name, got, err) }
		})
	}
}

func TestNativeDelegationSavedPreferenceIsNotCapturedAuthority(t *testing.T) {
	r := coordinatorRoute(projectRunCreateRequest{NativeDelegation: NativeDelegationCodexHelpers2, WorktreeID: "w-owned", Harness: "codex", Model: "opaque", PermissionMode: "bypass", Outcome: "preview"})
	saved := &CoordinatorSavedRoute{Revision: 1, MaxParallel: 1, Route: r}
	if !validCoordinatorRoute(saved) { t.Fatal("explicit preference refused") }
	encoded, _ := json.Marshal(saved)
	if strings.Contains(string(encoded), "nativeDelegationImage") { t.Fatal("saved preference acquired server image authority") }
	saved.Route.Harness = "opencode"; if validCoordinatorRoute(saved) { t.Fatal("incompatible saved preference accepted") }
	if validateNativeDelegationInput(ProjectRunInput{Route: nativeFixtureRoute()}) == nil { t.Fatal("preference/profile without approval grants work") }
}

func TestNativeDelegationHTTPApprovedImageBindingAndReplay(t *testing.T) {
	s, handler, req, base, _, selected := contractFixture(t)
	s.sandboxImage = nativeFixtureImage(); selected.PermissionMode = "bypass"; req.PermissionMode = "bypass"
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/plan", map[string]any{
		"expectedRevision": req.ImplementationContext.PlanRevision, "expectedSpecificationRevision": req.ImplementationContext.SpecificationRevision,
		"tasks": []ProjectTask{{ID: "task-save", Title: "Implement saving", Role: "coding", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"save.txt"}, Acceptance: "Saving round trips the authored value."}},
	}, 200).Body.Bytes(), &plan)
	req.ImplementationContext.PlanRevision = plan.Revision
	prepare := contractRequest(req); prepare.Route.NativeDelegation = NativeDelegationCodexHelpers2
	endpoint := base + "/orchestration-contracts"
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	if view.Contract.Input.Route.NativeDelegationImage != nativeFixtureImage() { t.Fatal("configured image not captured") }
	approved := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes())
	req.NativeDelegation = NativeDelegationCodexHelpers2
	req.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	p, err := s.resolveProject(req.ImplementationContext.ProjectID)
	if err != nil { t.Fatal(err) }
	if err := s.coordinatorContractSelectionCurrent(orchestratorActor(t, approved.Contract.Actor.ID), p, approved.Contract.Actor.ID, req); err != nil { t.Fatal("matching continuation refused", err) }
	wrong := req; wrong.NativeDelegation = ""
	if s.coordinatorContractSelectionCurrent(context.Background(), p, approved.Contract.Actor.ID, wrong) == nil { t.Fatal("continuation discarded profile") }
	s.sandboxImage = "sha256:" + strings.Repeat("8", 64)
	journeyRequest(t, handler, "POST", base+"/runs", req, 409)
	s.sandboxImage = nativeFixtureImage()
	without := req; without.NativeDelegation = ""; journeyRequest(t, handler, "POST", base+"/runs", without, 409)
	created := projectRunHTTPResponse(t, journeyRequest(t, handler, "POST", base+"/runs", req, 201))
	if created.Input.Route != approved.Contract.Input.Route { t.Fatal("create lost approved route") }
	capture, err := s.CaptureImplementationContext(context.Background(), req.ImplementationContext, req.WorktreeID, p.Path)
	if err != nil { t.Fatal(err) }
	if err := appendRunExecutionContract(capture, created.ProjectRun, "task-save"); err != nil { t.Fatal(err) }
	delivered := false
	for _, file := range capture.Files {
		if file.Path == "LUCI.md" && strings.Contains(string(file.Data), `"nativeDelegation": "`+NativeDelegationCodexHelpers2+`"`) && strings.Contains(string(file.Data), nativeFixtureImage()) { delivered = true }
	}
	if !delivered { t.Fatal("captured brief lost exact approved profile/image") }
	journeyRequest(t, handler, "POST", base+"/runs/"+created.ID+"/operations", map[string]any{"expectedRevision": created.Revision, "idempotencyKey": "native-direct-operation-refused-123", "kind": "session"}, 409)
	retained, err := s.projectRuns.Get(p.ID, p.Path, created.ID)
	if err != nil || len(retained.Operations) != 0 { t.Fatal("direct helper refusal allocated an operation", err) }
	s.sandboxImage = "luci/sandbox:dev"
	replay := projectRunHTTPResponse(t, journeyRequest(t, handler, "POST", base+"/runs", req, 200))
	if !reflect.DeepEqual(created.Input, replay.Input) { t.Fatal("historical replay changed approved input") }
	changed := contractResponse(t, journeyRequest(t, handler, "GET", endpoint+"/"+approved.Contract.ID, nil, 200).Body.Bytes())
	if changed.Freshness.Status != "changed" { t.Fatal("new-work current-image change hidden") }
}
