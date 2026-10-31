package deploymentplan

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func clusterPlanFixture(t *testing.T) (AzureFoundationRequest, AzureClusterSpec, AzureClusterPrincipal) {
	t.Helper()
	b := exampleBlueprint()
	b.Services = b.Services[:1]
	b.Services[0].DependsOn = nil
	b.ExternalResources = nil
	p, e := Compile(b)
	if e != nil {
		t.Fatal(e)
	}
	return AzureFoundationRequest{Blueprint: b, ExpectedBlueprintDigest: p.Digest}, AzureClusterSpec{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", KubernetesVersion: "1.34.1", NodeSKU: "Standard_D2s_v5", NodeCount: 1, AuthorizedAPICIDRs: []string{"8.8.8.8/32"}}, AzureClusterPrincipal{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", ObjectID: "cccccccc-cccc-cccc-cccc-cccccccccccc", Kind: "User", AccountName: "operator@example.com"}
}
func TestAzureClusterTemplateSeparatesAccessGrantAndFixedCPUCapacity(t *testing.T) {
	request, spec, principal := clusterPlanFixture(t)
	fit, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab")
	if err != nil || !fit.Fits || fit.CapacityVerified {
		t.Fatal("nominal plan unavailable", err)
	}
	raw, err := AzureClusterTemplate(fit, "project-east", "eastus", "ssh-ed25519 "+base64.StdEncoding.EncodeToString(make([]byte, 32)), "cp-original")
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Resources []struct {
			Type       string
			Properties struct {
				DisableLocalAccounts bool `json:"disableLocalAccounts"`
				AAD                  struct {
					Managed   bool
					AzureRBAC bool `json:"enableAzureRBAC"`
				} `json:"aadProfile"`
				AgentPools []struct {
					Count int
					Auto  bool `json:"enableAutoScaling"`
				} `json:"agentPoolProfiles"`
			} `json:"properties"`
		}
	}
	if json.Unmarshal([]byte(raw), &value) != nil || len(value.Resources) != 1 || value.Resources[0].Type != "Microsoft.ContainerService/managedClusters" || !value.Resources[0].Properties.DisableLocalAccounts || !value.Resources[0].Properties.AAD.Managed || !value.Resources[0].Properties.AAD.AzureRBAC || value.Resources[0].Properties.AgentPools[0].Count != 1 || value.Resources[0].Properties.AgentPools[0].Auto {
		t.Fatal("template widened cluster capability")
	}
	if strings.Contains(raw, "roleAssignments") || strings.Contains(raw, "adminGroupObjectIDs") || strings.Contains(raw, "clientSecret") || strings.Contains(raw, "monitoring") {
		t.Fatal("hidden authority, credential or analytics resource")
	}
	if fit.RoleDefinitionID != "/subscriptions/"+spec.SubscriptionID+"/providers/Microsoft.Authorization/roleDefinitions/"+AzureAKSClusterAdminRole {
		t.Fatal("wrong bootstrap role")
	}
}
func TestAzureClusterRejectsUnboundedNetworkingAndForgedPrincipal(t *testing.T) {
	request, spec, principal := clusterPlanFixture(t)
	for _, cidr := range []string{"0.0.0.0/0", "10.0.0.0/24", "8.8.8.8/24", "::/0"} {
		copy := spec
		copy.AuthorizedAPICIDRs = []string{cidr}
		if _, e := AzureClusterFoundation(request, copy, principal, "project-east", "0123456789ab"); e == nil {
			t.Fatal("invalid controller API range accepted", cidr)
		}
	}
	principal.TenantID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	if _, e := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab"); e == nil {
		t.Fatal("principal tenant mismatch accepted")
	}
}
func TestAzureClusterRoleNameIsBoundToExactClusterPrincipalAndFixedRole(t *testing.T) {
	a := AzureClusterRoleAssignmentName("/subscriptions/one/cluster/a", "principal-a")
	if !azureUUID.MatchString(a) || a != AzureClusterRoleAssignmentName("/SUBSCRIPTIONS/ONE/CLUSTER/A", "principal-a") || a == AzureClusterRoleAssignmentName("/subscriptions/one/cluster/b", "principal-a") || a == AzureClusterRoleAssignmentName("/subscriptions/one/cluster/a", "principal-b") {
		t.Fatal("role assignment identity not exact")
	}
}
func TestAzureClusterSizingAllowsOnlyItsOwnMissingTargetAndNamespaceDecisions(t *testing.T) {
	request, spec, principal := clusterPlanFixture(t)
	for _, namespace := range []string{"123apps", "apps--blue"} {
		copy := request.Blueprint
		copy.Infrastructure.Namespace = namespace
		compiled, err := Compile(copy)
		if err != nil || compiled.Blueprint.Infrastructure.Namespace != namespace {
			t.Fatal("standard namespace did not roundtrip", namespace, err)
		}
	}
	request.Blueprint.Infrastructure.Reference = ""
	request.Blueprint.Infrastructure.Namespace = ""
	p, err := Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedBlueprintDigest = p.Digest
	if p.State != "decisions-required" {
		t.Fatal("fixture must not invent an existing target")
	}
	if fit, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab"); err != nil || !fit.Fits {
		t.Fatal("new cluster required a fictitious target", err)
	}
	request.Blueprint.Services[0].Image = ""
	p, err = Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedBlueprintDigest = p.Digest
	if _, err = AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab"); err == nil {
		t.Fatal("unresolved workload identity was silently accepted")
	}
}
