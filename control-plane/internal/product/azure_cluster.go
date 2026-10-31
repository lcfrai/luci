package product

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type AzureClusterBinding struct {
	BlueprintRevision int                            `json:"blueprintRevision"`
	BlueprintDigest   string                         `json:"blueprintDigest"`
	Fit               deploymentplan.AzureClusterFit `json:"fit"`
	SSHPublicKey      string                         `json:"sshPublicKey"` // public key only; no key generation
}

func clusterNameSuffix(request deploymentplan.AzureFoundationRequest, spec deploymentplan.AzureClusterSpec, group string) string {
	return reviewDigest(struct{ BlueprintDigest, SubscriptionID, TenantID, Group string }{request.ExpectedBlueprintDigest, spec.SubscriptionID, spec.TenantID, group})[:12]
}
func clusterFoundationFit(fit deploymentplan.AzureClusterFit, summary deploymentplan.ResourceSummary) deploymentplan.AzureFoundationFit {
	return deploymentplan.AzureFoundationFit{BlueprintDigest: fit.BlueprintDigest, Tier: "aks-fixed-cpu", MilliCPU: fit.NominalNodeMilliCPU * fit.Spec.NodeCount, MemoryMiB: fit.NominalNodeMemoryMiB * fit.Spec.NodeCount, SystemReserveMiB: fit.ReservedMemoryMiBPerNode * fit.Spec.NodeCount, Requested: summary, Fits: fit.Fits, CapacityVerified: false, Decisions: fit.Decisions}
}
func (s *Server) prepareAzureClusterBinding(ctx context.Context, request CloudRequest) (*AzureBlueprintBinding, error) {
	if request.Provider != "azure" || request.Foundation == nil || request.Cluster == nil || request.Domain != "" {
		return nil, errAzureCluster
	}
	plan, err := deploymentplan.Compile(request.Foundation.Blueprint)
	if err != nil || plan.Digest != request.Foundation.ExpectedBlueprintDigest {
		return nil, errAzureCluster
	}
	actor, err := management.CaptureDeploymentActor(ctx, plan.Blueprint.ProjectID)
	if err != nil {
		return nil, err
	}
	current, err := s.foundationAuthorityProfile(ctx, actor, false, 0, false, true)
	if err != nil {
		return nil, err
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	contextDigest, err := s.foundationContextLocked(p, plan.Blueprint)
	_, saved, savedErr := s.readDeploymentBlueprintLedger(p)
	var blueprint SavedDeploymentBlueprint
	if savedErr == nil && len(saved.Revisions) > 0 {
		blueprint = saved.Revisions[len(saved.Revisions)-1].SavedDeploymentBlueprint
	}
	s.mu.RUnlock()
	if err != nil || savedErr != nil || blueprint.Revision < 1 || blueprint.PlanDigest != plan.Digest {
		return nil, errAzureCluster
	}
	bearer, err := s.azureClusterToken(current, actor, *request.Cluster, nil)
	if err != nil {
		return nil, err
	}
	fit, err := deploymentplan.AzureClusterFoundation(*request.Foundation, *request.Cluster, bearer.principal, request.Name, clusterNameSuffix(*request.Foundation, *request.Cluster, request.Name))
	if err != nil || !fit.Fits {
		return nil, errAzureCluster
	}
	if _, err = s.foundationAuthorityProfile(ctx, actor, false, 0, false, true); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(request.Foundation)
	if err != nil {
		return nil, errAzureCluster
	}
	var copied deploymentplan.AzureFoundationRequest
	if json.Unmarshal(raw, &copied) != nil {
		return nil, errAzureCluster
	}
	return &AzureBlueprintBinding{Version: 1, Request: copied, Actor: actor, ContextDigest: contextDigest, Fit: clusterFoundationFit(fit, plan.ResourceSummary), Cluster: &AzureClusterBinding{BlueprintRevision: blueprint.Revision, BlueprintDigest: blueprint.Digest, Fit: fit, SSHPublicKey: request.SSHPublicKey}}, nil
}
func validAzureClusterPlan(plan CloudPlan) bool {
	b := plan.Foundation
	if b == nil || b.Version != 1 || b.Cluster == nil || b.Cluster.BlueprintRevision < 1 || !hexDigest(b.Cluster.BlueprintDigest) || plan.Kind != "aks" || b.Actor.Local || b.Actor.UserID == "" || b.Actor.ProjectID != b.Request.Blueprint.ProjectID || !hexDigest(b.ContextDigest) || !hexDigest(b.PlanDigest) || b.PlanDigest != foundationPlanDigest(plan) || plan.Provider != "azure" || plan.Domain != "" || math.IsNaN(plan.MonthlyBudgetUSD) || math.IsInf(plan.MonthlyBudgetUSD, 0) || plan.MonthlyBudgetUSD <= 0 || plan.MonthlyBudgetUSD > 100000 {
		return false
	}
	known := b.Cluster.Fit
	fit, err := deploymentplan.AzureClusterFoundation(b.Request, known.Spec, known.Principal, plan.Name, clusterNameSuffix(b.Request, known.Spec, plan.Name))
	if err != nil || !fit.Fits || !reflect.DeepEqual(fit, known) {
		return false
	}
	compiled, err := deploymentplan.Compile(b.Request.Blueprint)
	if err != nil || !reflect.DeepEqual(b.Fit, clusterFoundationFit(fit, compiled.ResourceSummary)) {
		return false
	}
	expected, err := deploymentplan.AzureClusterTemplate(fit, plan.Name, plan.Region, b.Cluster.SSHPublicKey, plan.ID)
	return err == nil && len(plan.Files) == 1 && plan.Files[0].Name == "template.json" && plan.Files[0].MediaType == "application/json" && plan.Files[0].Content == expected && plan.Estimate.Monthly == nil && !plan.Estimate.Verified
}
func (s *Server) buildAzureClusterCloudPlan(r CloudRequest, binding AzureBlueprintBinding) (CloudPlan, error) {
	if binding.Cluster == nil {
		return CloudPlan{}, errAzureCluster
	}
	fit := binding.Cluster.Fit
	plan := CloudPlan{Kind: "aks", ID: newID("cp-"), Provider: "azure", Name: r.Name, Region: r.Region, MonthlyBudgetUSD: r.MonthlyBudgetUSD, CreatedAt: time.Now().UTC(), Resources: []string{"One Entra-enabled AKS cluster; local accounts disabled", fit.Spec.NodeSKU + " fixed CPU system pool", "Provider-managed node resource group, networking and disks"}, Estimate: Estimate{Currency: "USD", Explanation: "No verified Azure quote or billing observation. Nodes, disks, outbound networking and other managed resources incur ongoing costs; the reviewed budget is an authority ceiling, not a provider-enforced cap."}, Warnings: []string{"Cluster creation is separate from bootstrap RBAC grant, project namespace creation, usable target admission and D/S deployment.", "No model storage, GPU pool/plugin, autoscaling, image build, Collector instrumentation or product analytics is provisioned."}, Steps: []PlanStep{{ID: "cluster", Title: "Create exact cluster and fixed nodes", Detail: "The existing owned resource-group job validates this exact ARM template before one non-replayed asynchronous creation."}, {ID: "access", Title: "Review bootstrap access separately", Detail: "The captured Azure principal needs a separately acknowledged AKS RBAC Cluster Admin assignment scoped only to this new cluster; operator roleAssignments/write permission is required."}, {ID: "namespace", Title: "Commission one project namespace", Detail: "An explicit review seals either exact existing UID or a separately owned absence/create attempt. No default-namespace auto-sharing."}, {ID: "target", Title: "Use the observed target", Detail: "Only after real authenticated namespace access and exact UID readback can this connection be used by the existing reviewed D/S flow."}}}
	raw, err := deploymentplan.AzureClusterTemplate(fit, r.Name, r.Region, r.SSHPublicKey, plan.ID)
	if err != nil {
		return CloudPlan{}, err
	}
	plan.Files = []PlanFile{{Name: "template.json", Content: raw, MediaType: "application/json"}}
	plan.Commands = []PlanCommand{{Label: "Create isolated group", Executable: "az", Args: []string{"group", "create", "--subscription", fit.Spec.SubscriptionID, "--name", r.Name, "--location", r.Region}}, {Label: "Validate cluster", Executable: "az", Args: []string{"deployment", "group", "validate", "--subscription", fit.Spec.SubscriptionID, "--resource-group", r.Name, "--template-file", "template.json"}}, {Label: "Create cluster without waiting", Executable: "az", Args: []string{"deployment", "group", "create", "--subscription", fit.Spec.SubscriptionID, "--resource-group", r.Name, "--name", r.Name, "--template-file", "template.json", "--no-wait"}}}
	return plan, nil
}

// Provider-account checks are observations around the existing CLI command;
// they are not an atomic lock against an OS operator changing CLI accounts.
// Every command additionally names the exact subscription; no wider retry or
// different principal is selected automatically if the observed source changes.
func (s *Server) cloudJobCommand(ctx context.Context, j CloudJob, exe string, args []string, dir string) (string, error) {
	return s.cloudJobCommandBefore(ctx, j, exe, args, dir, nil)
}
func (s *Server) cloudJobCommandBefore(ctx context.Context, j CloudJob, exe string, args []string, dir string, before func(context.Context) error) (string, error) {
	if j.Plan.Foundation == nil || j.Plan.Foundation.Cluster == nil {
		return s.runner(ctx, exe, args, dir)
	}
	b := j.Plan.Foundation
	current, err := s.foundationAuthorityProfile(ctx, b.Actor, j.Status == "creating", j.Plan.MonthlyBudgetUSD, !b.Cluster.Fit.Spec.PrivateAPI, true)
	if err != nil {
		return "", &foundationAdmissionRefusal{cause: err}
	}
	if _, err = s.azureClusterToken(current, b.Actor, b.Cluster.Fit.Spec, &b.Cluster.Fit.Principal); err != nil {
		return "", &foundationAdmissionRefusal{cause: err}
	}
	if exe != "az" {
		return "", errAzureCluster
	}
	exact := append(append([]string{}, args...), "--subscription", b.Cluster.Fit.Spec.SubscriptionID)
	// A reviewed commissioning write repeats its exact policy/source/cluster
	// admission AFTER account/token I/O, immediately before this one mutation.
	if before != nil {
		if err = before(ctx); err != nil {
			return "", &foundationAdmissionRefusal{cause: err}
		}
	}
	raw, err := s.runner(ctx, exe, exact, dir)
	if err != nil {
		return "", errAzureCluster
	}
	if len(raw) > 256<<10 {
		return "", errAzureCluster
	}
	if _, err = s.foundationAuthorityProfile(ctx, b.Actor, false, 0, false, true); err != nil {
		return "", &foundationAdmissionRefusal{cause: err}
	}
	if _, err = s.azureClusterToken(current, b.Actor, b.Cluster.Fit.Spec, &b.Cluster.Fit.Principal); err != nil {
		return "", &foundationAdmissionRefusal{cause: err}
	}
	return raw, nil
}
