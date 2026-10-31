package product

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestManagedGPUFeatureRequiresExactSubscriptionAndRegisteredState(t *testing.T) {
	spec := deploymentplan.AzureClusterSpec{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}
	value := map[string]any{"id": "/subscriptions/" + spec.SubscriptionID + "/providers/Microsoft.Features/providers/Microsoft.ContainerService/features/" + deploymentplan.AzureManagedGPUFeature, "name": "Microsoft.ContainerService/" + deploymentplan.AzureManagedGPUFeature, "properties": map[string]string{"state": "Registered"}}
	raw, _ := json.Marshal(value)
	if !managedGPUFeatureRegistered(string(raw), spec) {
		t.Fatal("exact registered feature rejected")
	}
	value["properties"] = map[string]string{"state": "Registering"}
	raw, _ = json.Marshal(value)
	if managedGPUFeatureRegistered(string(raw), spec) {
		t.Fatal("pending feature admitted")
	}
	value["properties"] = map[string]string{"state": "Registered"}
	value["id"] = "/subscriptions/foreign"
	raw, _ = json.Marshal(value)
	if managedGPUFeatureRegistered(string(raw), spec) {
		t.Fatal("foreign subscription admitted")
	}
}
func TestManagedGPUAdmissionNeverRegistersFeatureOrRunsCreationAfterRefusal(t *testing.T) {
	s, ctx, job, base := managedGPUCreatingFixture(t)
	features, writes := 0, 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 0 && args[0] == "feature" {
			features++
			if args[1] != "show" || strings.Contains(strings.Join(args, " "), "register") {
				t.Fatal("implicit registration")
			}
			return "", errors.New("feature not registered")
		}
		if len(args) > 0 && args[0] == "deployment" {
			writes++
			return "should-not-run", nil
		}
		return base(exe, args)
	}
	if _, err := s.cloudJobCommand(ctx, job, "az", []string{"deployment", "group", "create"}, ""); err == nil || writes != 0 || features != 1 {
		t.Fatal("feature refusal did not prevent mutation", err, writes, features)
	}
}
func TestManagedGPUPoolObservationRejectsSubstitutionAndMissingManagedStack(t *testing.T) {
	spec := deploymentplan.AzureClusterSpec{NodeSKU: "Standard_D2s_v5", NodeCount: 1, GPUPool: &deploymentplan.AzureGPUNodePoolSpec{NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 1, AcknowledgeManagedPreview: true}}
	raw := `[{"name":"system","vmSize":"Standard_D2s_v5","count":1,"mode":"System","osType":"Linux","osSKU":"Ubuntu"},{"name":"gpu","vmSize":"Standard_NC4as_T4_v3","count":1,"mode":"User","osType":"Linux","osSKU":"Ubuntu","nodeTaints":["sku=gpu:NoSchedule"],"gpuProfile":{"driver":"Install","nvidia":{"managementMode":"Managed"}}}]`
	var pools []azureManagedClusterPool
	if json.Unmarshal([]byte(raw), &pools) != nil || !azureClusterPoolsMatch(pools, spec) {
		t.Fatal("exact original pools rejected")
	}
	pools[1].GPUProfile.Nvidia.ManagementMode = "Unmanaged"
	if azureClusterPoolsMatch(pools, spec) {
		t.Fatal("driver-only pool accepted as managed stack")
	}
	pools[1].GPUProfile.Nvidia.ManagementMode = "Managed"
	pools[1].Count = 2
	if azureClusterPoolsMatch(pools, spec) {
		t.Fatal("unreviewed expansion accepted")
	}
	pools[1].Count = 1
	pools = append(pools, pools[1])
	if azureClusterPoolsMatch(pools, spec) {
		t.Fatal("hidden extra pool accepted")
	}
}

func managedGPUCreatingFixture(t *testing.T) (*Server, context.Context, CloudJob, func(string, []string) (string, error)) {
	t.Helper()
	s, ctx, old, base := aksProductFixture(t)
	binding := *old.Plan.Foundation
	cluster := *binding.Cluster
	spec := cluster.Fit.Spec
	spec.GPUPool = &deploymentplan.AzureGPUNodePoolSpec{NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 1, AcknowledgeManagedPreview: true}
	fit, err := deploymentplan.AzureClusterFoundation(binding.Request, spec, cluster.Fit.Principal, old.Plan.Name, clusterNameSuffix(binding.Request, spec, old.Plan.Name))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := deploymentplan.Compile(binding.Request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	cluster.Fit = fit
	binding.Cluster = &cluster
	binding.Fit = clusterFoundationFit(fit, compiled.ResourceSummary)
	plan, err := s.buildAzureClusterCloudPlan(CloudRequest{Name: old.Plan.Name, Region: old.Plan.Region, MonthlyBudgetUSD: old.Plan.MonthlyBudgetUSD, SSHPublicKey: cluster.SSHPublicKey}, binding)
	if err != nil {
		t.Fatal(err)
	}
	plan.Foundation = &binding
	binding.PlanDigest = foundationPlanDigest(plan)
	if !validFoundationPlan(plan) {
		t.Fatal("invalid managed GPU fixture")
	}
	job := old
	job.Plan = plan
	job.PlanID = plan.ID
	job.Status = "creating"
	job.Cluster = nil
	s.jobs[job.ID] = job
	if err = s.admitFoundationProvider(ctx, job); err != nil {
		t.Fatal("fixture lacks initial creating admission", err)
	}
	return s, ctx, job, base
}
func TestManagedGPUFeatureReadPolicyLossCannotReachCreation(t *testing.T) {
	s, ctx, job, base := managedGPUCreatingFixture(t)
	deny := false
	original := s.coordinatorDeploymentAuthorize
	s.coordinatorDeploymentAuthorize = func(c context.Context, actor management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		value, err := original(c, actor, allocate)
		value.Policy.AllowCloudProvisioning = !deny
		return value, err
	}
	reads, writes := 0, 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 1 && args[0] == "feature" && args[1] == "show" {
			reads++
			deny = true
			raw, _ := json.Marshal(map[string]any{"id": "/subscriptions/" + job.Plan.Foundation.Cluster.Fit.Spec.SubscriptionID + "/providers/Microsoft.Features/providers/Microsoft.ContainerService/features/" + deploymentplan.AzureManagedGPUFeature, "name": "Microsoft.ContainerService/" + deploymentplan.AzureManagedGPUFeature, "properties": map[string]string{"state": "Registered"}})
			return string(raw), nil
		}
		if len(args) > 0 && args[0] == "deployment" {
			writes++
			return "", nil
		}
		return base(exe, args)
	}
	if _, err := s.cloudJobCommand(ctx, job, "az", []string{"deployment", "group", "create"}, ""); err == nil || reads != 1 || writes != 0 {
		t.Fatal("late allocation-policy loss reached provider mutation", err, reads, writes)
	}
}
