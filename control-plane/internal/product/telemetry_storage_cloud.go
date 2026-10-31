package product

import (
	"context"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"math"
	"reflect"
	"strings"
	"time"
)

type AzureTelemetryStorageBinding struct {
	Spec              deploymentplan.AzureTelemetryStorageSpec `json:"spec"`
	Principal         deploymentplan.AzureClusterPrincipal     `json:"principal"`
	BlueprintRevision int                                      `json:"blueprintRevision"`
	BlueprintDigest   string                                   `json:"blueprintDigest"`
}

func validateTelemetryStorageCloudRequest(r CloudRequest) error {
	if r.TelemetryStorage == nil || !deploymentplan.ValidAzureTelemetryStorage(*r.TelemetryStorage) || r.Provider != "azure" || r.Foundation == nil || r.Cluster != nil || !slugPattern.MatchString(r.Name) || !regionPattern.MatchString(r.Region) || r.Domain != "" || r.SSHPublicKey != "" || r.Size != "" || r.MonthlyBudgetUSD <= 0 || r.MonthlyBudgetUSD > 100000 || math.IsNaN(r.MonthlyBudgetUSD) || math.IsInf(r.MonthlyBudgetUSD, 0) {
		return errAzureBlueprint
	}
	return nil
}
func storageFoundationFit(plan deploymentplan.Plan) deploymentplan.AzureFoundationFit {
	return deploymentplan.AzureFoundationFit{BlueprintDigest: plan.Digest, Tier: "telemetry-blob", Requested: plan.ResourceSummary, Fits: true, Decisions: []deploymentplan.Decision{{Path: "telemetry.storage", Code: "price-unverified", Message: "Dedicated Blob storage, operations, traffic and retained data incur ongoing charges. Reviewed budget is authority, not a provider-enforced cap."}}}
}
func storageMatchesBlueprint(spec deploymentplan.AzureTelemetryStorageSpec, b deploymentplan.Blueprint) bool {
	for _, s := range b.Services {
		v := s.TelemetryStorage
		if v != nil && v.Profile == deploymentplan.TempoStorageProfile && v.Azure != nil && v.Azure.AccountName == spec.AccountName && v.Azure.ContainerName == spec.ContainerName && spec.RetentionDays == (v.RetentionHours+23)/24 {
			return true
		}
	}
	return false
}
func (s *Server) prepareTelemetryStorageBinding(ctx context.Context, r CloudRequest) (*AzureBlueprintBinding, error) {
	if validateTelemetryStorageCloudRequest(r) != nil {
		return nil, errAzureBlueprint
	}
	plan, e := deploymentplan.Compile(r.Foundation.Blueprint)
	if e != nil || plan.Digest != r.Foundation.ExpectedBlueprintDigest || !storageMatchesBlueprint(*r.TelemetryStorage, plan.Blueprint) {
		return nil, errAzureBlueprint
	}
	actor, e := management.CaptureDeploymentActor(ctx, plan.Blueprint.ProjectID)
	if e != nil {
		return nil, e
	}
	if _, e = s.foundationAuthorityProfile(ctx, actor, false, 0, false, true); e != nil {
		return nil, e
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return nil, e
	}
	s.mu.RLock()
	contextDigest, e := s.foundationContextLocked(p, plan.Blueprint)
	_, ledger, le := s.readDeploymentBlueprintLedger(p)
	s.mu.RUnlock()
	if e != nil || le != nil || len(ledger.Revisions) == 0 {
		return nil, errAzureBlueprint
	}
	saved := ledger.Revisions[len(ledger.Revisions)-1]
	if saved.PlanDigest != plan.Digest {
		return nil, errAzureBlueprint
	}
	principal, e := s.telemetryStoragePrincipal(ctx, actor, *r.TelemetryStorage, nil)
	if e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(r.Foundation)
	var copied deploymentplan.AzureFoundationRequest
	if json.Unmarshal(raw, &copied) != nil {
		return nil, errAzureBlueprint
	}
	return &AzureBlueprintBinding{Version: 1, Request: copied, Actor: actor, ContextDigest: contextDigest, Fit: storageFoundationFit(plan), TelemetryStorage: &AzureTelemetryStorageBinding{Spec: *r.TelemetryStorage, Principal: principal, BlueprintRevision: saved.Revision, BlueprintDigest: saved.Digest}}, nil
}
func validTelemetryStorageCloudPlan(plan CloudPlan) bool {
	b := plan.Foundation
	if b == nil || b.TelemetryStorage == nil || b.Cluster != nil || plan.Kind != "telemetry-storage" || plan.Provider != "azure" || plan.Domain != "" || !deploymentplan.ValidateAzureClusterPrincipal(b.TelemetryStorage.Principal) || b.TelemetryStorage.Spec.SubscriptionID != b.TelemetryStorage.Principal.SubscriptionID || b.TelemetryStorage.Spec.TenantID != b.TelemetryStorage.Principal.TenantID || b.TelemetryStorage.BlueprintRevision < 1 || !hexDigest(b.TelemetryStorage.BlueprintDigest) {
		return false
	}
	compiled, e := deploymentplan.Compile(b.Request.Blueprint)
	if e != nil || compiled.Digest != b.Request.ExpectedBlueprintDigest || !storageMatchesBlueprint(b.TelemetryStorage.Spec, compiled.Blueprint) || !reflect.DeepEqual(b.Fit, storageFoundationFit(compiled)) {
		return false
	}
	raw, e := deploymentplan.AzureTelemetryStorageTemplate(b.TelemetryStorage.Spec, plan.Region, plan.ID)
	return e == nil && len(plan.Files) == 1 && plan.Files[0].Name == "template.json" && plan.Files[0].MediaType == "application/json" && plan.Files[0].Content == raw && plan.Estimate.Monthly == nil && !plan.Estimate.Verified && plan.MonthlyBudgetUSD > 0 && plan.MonthlyBudgetUSD <= 100000 && !math.IsNaN(plan.MonthlyBudgetUSD) && !math.IsInf(plan.MonthlyBudgetUSD, 0)
}
func (s *Server) buildTelemetryStorageCloudPlan(r CloudRequest, b AzureBlueprintBinding) (CloudPlan, error) {
	plan := CloudPlan{Kind: "telemetry-storage", ID: newID("cp-"), Provider: "azure", Name: r.Name, Region: r.Region, MonthlyBudgetUSD: r.MonthlyBudgetUSD, CreatedAt: time.Now().UTC(), Resources: []string{"Dedicated Standard_LRS StorageV2 account with HTTPS-only, private Blob container and lifecycle rule"}, Estimate: Estimate{Currency: "USD", Explanation: "No verified price, quota or usage. Blob bytes, operations and egress incur charges; budget is not a provider billing cap."}, Warnings: []string{"Authenticated Azure public HTTPS endpoint; this does not create a private network endpoint.", "The separate private Kubernetes Secret must hold this dedicated account's access key. No key is returned in plan/artifacts or installed by group creation.", "Deleting this owned resource group irreversibly deletes its trace storage. Export retained data first; workload/PVC deletion is separate."}, Steps: []PlanStep{{ID: "storage", Title: "Review and create dedicated trace storage", Detail: "Existing owned CloudJob validates one ARM template and records its original asynchronous create; unknown outcomes are observed, never replayed."}, {ID: "credentials", Title: "Connect the private storage key", Detail: "Use the exact dedicated account/container and an existing or separately reviewed private Kubernetes Secret. Credential values never enter the blueprint."}, {ID: "deploy", Title: "Review and deploy metrics, traces and Collector", Detail: "Select compatible immutable images, writable PVCs, mTLS refs and retention; readiness does not certify delivery or backup."}}}
	raw, e := deploymentplan.AzureTelemetryStorageTemplate(b.TelemetryStorage.Spec, r.Region, plan.ID)
	if e != nil {
		return CloudPlan{}, e
	}
	plan.Files = []PlanFile{{Name: "template.json", MediaType: "application/json", Content: raw}}
	plan.Commands = []PlanCommand{{Label: "Validate storage", Executable: "az", Args: []string{"deployment", "group", "validate", "--subscription", b.TelemetryStorage.Spec.SubscriptionID, "--resource-group", r.Name, "--template-file", "template.json"}}, {Label: "Create reviewed storage", Executable: "az", Args: []string{"deployment", "group", "create", "--subscription", b.TelemetryStorage.Spec.SubscriptionID, "--resource-group", r.Name, "--name", r.Name, "--template-file", "template.json", "--no-wait"}}}
	return plan, nil
}

// Fixed ARM token metadata correlates the already authenticated provider
// principal. No browser identity assertion, token persistence or IAM mutation.
func (s *Server) telemetryStoragePrincipal(ctx context.Context, actor management.ExecutionActorRef, spec deploymentplan.AzureTelemetryStorageSpec, expected *deploymentplan.AzureClusterPrincipal) (deploymentplan.AzureClusterPrincipal, error) {
	read := func(args ...string) (string, error) {
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", management.ErrExecutionAuthority
		}
		raw, e := s.runner(ctx, "az", args, "")
		if e != nil || len(raw) > 64<<10 || ctx.Err() != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", errAzureBlueprint
		}
		return raw, nil
	}
	account, e := read("account", "show", "--subscription", spec.SubscriptionID, "--output", "json")
	if e != nil {
		return deploymentplan.AzureClusterPrincipal{}, e
	}
	var metadata azureAccountMetadata
	if json.Unmarshal([]byte(account), &metadata) != nil {
		return deploymentplan.AzureClusterPrincipal{}, errAzureBlueprint
	}
	principal, e := azureAccountPrincipal(metadata)
	if e != nil || principal.SubscriptionID != spec.SubscriptionID || principal.TenantID != spec.TenantID {
		return deploymentplan.AzureClusterPrincipal{}, errAzureBlueprint
	}
	raw, e := read("account", "get-access-token", "--subscription", spec.SubscriptionID, "--resource", "https://management.azure.com/", "--output", "json")
	if e != nil {
		return deploymentplan.AzureClusterPrincipal{}, e
	}
	var value struct {
		AccessToken  string `json:"accessToken"`
		TokenType    string `json:"tokenType"`
		Tenant       string `json:"tenant"`
		Subscription string `json:"subscription"`
	}
	if json.Unmarshal([]byte(raw), &value) != nil || value.TokenType != "Bearer" || strings.ToLower(value.Tenant) != spec.TenantID || strings.ToLower(value.Subscription) != spec.SubscriptionID {
		return deploymentplan.AzureClusterPrincipal{}, errAzureBlueprint
	}
	bearer, e := azureBearerClaimsForAudience(value.AccessToken, principal, time.Now(), "https://management.azure.com/")
	if e != nil || expected != nil && bearer.principal != *expected {
		return deploymentplan.AzureClusterPrincipal{}, errAzureBlueprint
	}
	if s.confirmAzureClusterAccount(ctx, actor, bearer.principal) != nil {
		return deploymentplan.AzureClusterPrincipal{}, errAzureBlueprint
	}
	return bearer.principal, nil
}
func (s *Server) telemetryStorageCommand(ctx context.Context, j CloudJob, exe string, args []string, dir string) (string, error) {
	b := j.Plan.Foundation
	if b == nil || b.TelemetryStorage == nil || exe != "az" {
		return "", errAzureBlueprint
	}
	v := b.TelemetryStorage
	if _, e := s.foundationAuthorityProfile(ctx, b.Actor, j.Status == "creating", j.Plan.MonthlyBudgetUSD, false, true); e != nil {
		return "", &foundationAdmissionRefusal{cause: e}
	}
	if _, e := s.telemetryStoragePrincipal(ctx, b.Actor, v.Spec, &v.Principal); e != nil {
		return "", &foundationAdmissionRefusal{cause: e}
	}
	// Account reads may wait. Rejoin allocation/source immediately before a
	// creating job's provider command, preserving the original no-replay marker.
	if j.Status == "creating" {
		if e := s.admitFoundationProvider(ctx, j); e != nil {
			return "", &foundationAdmissionRefusal{cause: e}
		}
	}
	exact := append(append([]string{}, args...), "--subscription", v.Spec.SubscriptionID)
	raw, e := s.runner(ctx, exe, exact, dir)
	if e != nil || len(raw) > 256<<10 || ctx.Err() != nil {
		return "", errAzureBlueprint
	}
	if _, e = s.telemetryStoragePrincipal(ctx, b.Actor, v.Spec, &v.Principal); e != nil {
		return "", &foundationAdmissionRefusal{cause: e}
	}
	return raw, nil
}
