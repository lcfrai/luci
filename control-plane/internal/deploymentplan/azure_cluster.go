package deploymentplan

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
)

// Existing stable ARM APIs; creating a managed cluster is distinct from the
// separately reviewed Kubernetes access grant and namespace bootstrap.
const AzureManagedClusterAPIVersion = "2025-02-01"
const AzureRoleAssignmentAPIVersion = "2022-04-01"

// Microsoft: Azure Kubernetes Service RBAC Cluster Admin. This is NOT the
// similarly named management-plane role that downloads local admin credentials.
const AzureAKSClusterAdminRole = "b1ff04bb-8a4e-4dc4-8eb5-8693973ce19b"

var azureUUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var azureKubernetesVersion = regexp.MustCompile(`^1\.[1-9][0-9]\.[0-9]{1,3}$`)
var azureClusterName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)

type AzureClusterSpec struct {
	GPUPool            *AzureGPUNodePoolSpec `json:"gpuPool,omitempty"`
	SubscriptionID     string                `json:"subscriptionId"`
	TenantID           string                `json:"tenantId"`
	KubernetesVersion  string                `json:"kubernetesVersion"`
	NodeSKU            string                `json:"nodeSku"`
	NodeCount          int                   `json:"nodeCount"`
	PrivateAPI         bool                  `json:"privateApi"`
	AuthorizedAPICIDRs []string              `json:"authorizedApiCidrs"`
}

// Only the server derives this from the current authorized CLI/token response.
// ObjectID is a provider principal, not an assertion of the Luci user's Azure
// identity. No token, password, certificate, group or new identity is supplied.
type AzureClusterPrincipal struct {
	SubscriptionID string `json:"subscriptionId"`
	TenantID       string `json:"tenantId"`
	ObjectID       string `json:"objectId"`
	Kind           string `json:"kind"` // User or ServicePrincipal
	AccountName    string `json:"accountName"`
}
type AzureClusterFit struct {
	Spec                     AzureClusterSpec      `json:"spec"`
	Principal                AzureClusterPrincipal `json:"principal"`
	BlueprintDigest          string                `json:"blueprintDigest"`
	ClusterName              string                `json:"clusterName"`
	ClusterResourceID        string                `json:"clusterResourceId"`
	NodeResourceGroup        string                `json:"nodeResourceGroup"`
	RoleDefinitionID         string                `json:"roleDefinitionId"`
	NominalNodeMilliCPU      int                   `json:"nominalNodeMilliCpu"`
	NominalNodeMemoryMiB     int                   `json:"nominalNodeMemoryMiB"`
	ReservedMilliCPUPerNode  int                   `json:"reservedMilliCpuPerNode"`
	ReservedMemoryMiBPerNode int                   `json:"reservedMemoryMiBPerNode"`
	Fits                     bool                  `json:"fits"`
	CapacityVerified         bool                  `json:"capacityVerified"`
	Decisions                []Decision            `json:"decisions"`
}

func ValidateAzureClusterPrincipal(value AzureClusterPrincipal) bool {
	return azureUUID.MatchString(value.SubscriptionID) && azureUUID.MatchString(value.TenantID) && azureUUID.MatchString(value.ObjectID) && (value.Kind == "User" || value.Kind == "ServicePrincipal") && strings.TrimSpace(value.AccountName) != "" && text(value.AccountName, 512)
}

// This is nominal declared fit, not quota/allocatable scheduling/price evidence.
// The private API option also needs real controller network reachability later.
func AzureClusterFoundation(request AzureFoundationRequest, spec AzureClusterSpec, principal AzureClusterPrincipal, group, nameSuffix string) (AzureClusterFit, error) {
	plan, err := Compile(request.Blueprint)
	if err != nil || plan.Digest != request.ExpectedBlueprintDigest || plan.Blueprint.Infrastructure.Kind != "kubernetes" {
		return AzureClusterFit{}, fmt.Errorf("choose a complete current Kubernetes blueprint")
	}
	// A new cluster must not require a fictitious existing target/namespace. All
	// OTHER unresolved workload/source inputs still block this sizing review.
	for _, d := range plan.Decisions {
		if !(d.Path == "infrastructure.reference" && d.Code == "existing-target-required" || d.Path == "infrastructure.namespace" && d.Code == "namespace-required") {
			return AzureClusterFit{}, fmt.Errorf("resolve workload/source inputs before cluster sizing")
		}
	}
	if !ValidateAzureClusterPrincipal(principal) || spec.SubscriptionID != principal.SubscriptionID || spec.TenantID != principal.TenantID || !azureUUID.MatchString(spec.SubscriptionID) || !azureUUID.MatchString(spec.TenantID) || !azureClusterName.MatchString(group) || !regexp.MustCompile(`^[a-f0-9]{12}$`).MatchString(nameSuffix) || !azureKubernetesVersion.MatchString(spec.KubernetesVersion) || spec.NodeCount < 1 || spec.NodeCount > 5 {
		return AzureClusterFit{}, fmt.Errorf("choose an exact existing Azure account, cluster version and fixed node count")
	}
	spec.AuthorizedAPICIDRs = append([]string{}, spec.AuthorizedAPICIDRs...)
	if spec.PrivateAPI {
		if len(spec.AuthorizedAPICIDRs) != 0 {
			return AzureClusterFit{}, fmt.Errorf("private API has no public authorized ranges")
		}
	} else {
		if len(spec.AuthorizedAPICIDRs) == 0 || len(spec.AuthorizedAPICIDRs) > 8 {
			return AzureClusterFit{}, fmt.Errorf("choose explicit controller IPv4 API ranges")
		}
		seen := map[string]bool{}
		for i, raw := range spec.AuthorizedAPICIDRs {
			prefix, e := netip.ParsePrefix(raw)
			if e != nil || !prefix.Addr().Is4() || prefix.Bits() < 24 || prefix.Bits() > 32 || !prefix.Addr().IsGlobalUnicast() || prefix.Addr().IsPrivate() || prefix.Addr().IsLoopback() || prefix != prefix.Masked() {
				return AzureClusterFit{}, fmt.Errorf("API ranges must be exact public IPv4 /24 through /32 networks")
			}
			canonical := prefix.String()
			if seen[canonical] {
				return AzureClusterFit{}, fmt.Errorf("duplicate API range")
			}
			seen[canonical] = true
			spec.AuthorizedAPICIDRs[i] = canonical
		}
		sort.Strings(spec.AuthorizedAPICIDRs)
	}
	fit := AzureClusterFit{Spec: spec, Principal: principal, BlueprintDigest: plan.Digest, ClusterName: group + "-aks", NodeResourceGroup: group + "-nodes-" + nameSuffix, ReservedMilliCPUPerNode: 500, ReservedMemoryMiBPerNode: 2048, Fits: true, Decisions: []Decision{}}
	switch spec.NodeSKU {
	case "Standard_D2s_v5":
		fit.NominalNodeMilliCPU, fit.NominalNodeMemoryMiB = 2000, 8192
	case "Standard_D4s_v5":
		fit.NominalNodeMilliCPU, fit.NominalNodeMemoryMiB = 4000, 16384
	case "Standard_D8s_v5":
		fit.NominalNodeMilliCPU, fit.NominalNodeMemoryMiB = 8000, 32768
	default:
		return AzureClusterFit{}, fmt.Errorf("choose one of the reviewed CPU system-node SKUs")
	}
	fit.ClusterResourceID = "/subscriptions/" + spec.SubscriptionID + "/resourceGroups/" + group + "/providers/Microsoft.ContainerService/managedClusters/" + fit.ClusterName
	fit.RoleDefinitionID = "/subscriptions/" + spec.SubscriptionID + "/providers/Microsoft.Authorization/roleDefinitions/" + AzureAKSClusterAdminRole
	block := func(path, code, message string) {
		fit.Fits = false
		fit.Decisions = append(fit.Decisions, Decision{path, code, message})
	}
	if fit.Spec.GPUPool != nil {
		if err := azureGPUFit(&fit, plan); err != nil {
			return AzureClusterFit{}, err
		}
	} else {
		cpu, mem := fit.NominalNodeMilliCPU-fit.ReservedMilliCPUPerNode, fit.NominalNodeMemoryMiB-fit.ReservedMemoryMiBPerNode
		for _, service := range plan.Blueprint.Services {
			if service.Resources.RequestMilliCPU > cpu || service.Resources.RequestMemoryMiB > mem {
				block("services."+service.ID, "pod-too-large", "One declared Pod exceeds this node's nominal capacity after the explicit reserve.")
			}
		}
		if plan.ResourceSummary.RequestMilliCPU > cpu*spec.NodeCount || plan.ResourceSummary.RequestMemoryMiB > mem*spec.NodeCount {
			block("services.resources", "nodes-too-small", "Aggregate requests exceed this fixed pool after explicit reserves.")
		}
		if plan.ResourceSummary.GPUCount > 0 {
			block("services.resources.gpuCount", "gpu-pool-not-configured", "This fixed system pool has no reviewed GPU node pool or device plugin.")
		}
	}
	fit.Decisions = append(fit.Decisions, Decision{"infrastructure", "provider-admission-required", "SKU/version availability, quota, actual allocatable resources, scheduling, network reachability and monetary cost are unverified. No automatic larger SKU, autoscaling or retry is selected."}, Decision{"infrastructure.access", "separate-access-grant", "Namespace bootstrap requires a separate explicit grant of the fixed AKS RBAC Cluster Admin role to the captured provider principal, scoped only to this new cluster; operator roleAssignments/write privilege is required."}, Decision{"services", "workloads-not-deployed", "Cluster creation does not create a project namespace, model/PVC, image, GPU plugin, telemetry collector, application or product analytics."})
	if fit.Spec.GPUPool != nil {
		fit.Decisions[len(fit.Decisions)-1].Message = "Cluster creation includes the reviewed managed GPU driver, plugin and DCGM stack. Project namespace, models/PVCs, application images, telemetry storage and workloads remain separate steps."
	}
	return fit, nil
}

func AzureClusterTemplate(fit AzureClusterFit, group, region, sshPublicKey, planID string) (string, error) {
	parts := strings.Fields(sshPublicKey)
	if len(parts) < 2 || len(parts) > 3 || (parts[0] != "ssh-ed25519" && parts[0] != "ssh-rsa" && parts[0] != "ecdsa-sha2-nistp256") {
		return "", fmt.Errorf("choose an existing public SSH key")
	}
	key, e := base64.StdEncoding.DecodeString(parts[1])
	if e != nil || len(key) < 32 || len(key) > 16384 {
		return "", fmt.Errorf("invalid public SSH key")
	}
	if !fit.Fits || fit.CapacityVerified || !ValidateAzureClusterPrincipal(fit.Principal) || !azureClusterName.MatchString(group) || !azureClusterName.MatchString(region) || len(sshPublicKey) > 24000 || !text(sshPublicKey, 24000) || !strings.HasPrefix(fit.ClusterResourceID, "/subscriptions/"+fit.Spec.SubscriptionID+"/resourceGroups/"+group+"/") || !text(planID, 128) || planID == "" {
		return "", fmt.Errorf("invalid reviewed cluster template")
	}
	network := map[string]any{"networkPlugin": "azure", "networkPluginMode": "overlay", "loadBalancerSku": "standard", "outboundType": "loadBalancer"}
	access := map[string]any{"enablePrivateCluster": fit.Spec.PrivateAPI}
	if !fit.Spec.PrivateAPI {
		access["authorizedIPRanges"] = fit.Spec.AuthorizedAPICIDRs
	}
	cluster := map[string]any{"type": "Microsoft.ContainerService/managedClusters", "apiVersion": AzureClusterAPI(fit.Spec), "name": fit.ClusterName, "location": region, "identity": map[string]string{"type": "SystemAssigned"}, "sku": map[string]string{"name": "Base", "tier": "Free"}, "tags": map[string]string{"managed-by": "luci", "luci-plan": planID}, "properties": map[string]any{
		"dnsPrefix": group + "-" + strings.TrimPrefix(fit.NodeResourceGroup, group+"-nodes-"), "kubernetesVersion": fit.Spec.KubernetesVersion, "nodeResourceGroup": fit.NodeResourceGroup, "enableRBAC": true, "disableLocalAccounts": true,
		"aadProfile": map[string]any{"managed": true, "enableAzureRBAC": true, "tenantID": fit.Spec.TenantID}, "apiServerAccessProfile": access, "networkProfile": network,
		"linuxProfile":      map[string]any{"adminUsername": "luci", "ssh": map[string]any{"publicKeys": []any{map[string]string{"keyData": sshPublicKey}}}},
		"agentPoolProfiles": []any{map[string]any{"name": "system", "count": fit.Spec.NodeCount, "vmSize": fit.Spec.NodeSKU, "mode": "System", "osType": "Linux", "osSKU": "Ubuntu", "type": "VirtualMachineScaleSets", "enableAutoScaling": false, "osDiskSizeGB": 128, "osDiskType": "Managed", "enableNodePublicIP": false}},
	}}
	if fit.Spec.GPUPool != nil {
		pool, err := azureGPUAgentPool(fit.Spec.GPUPool)
		if err != nil {
			return "", err
		}
		properties := cluster["properties"].(map[string]any)
		properties["agentPoolProfiles"] = append(properties["agentPoolProfiles"].([]any), pool)
	}
	// No IAM assignment is hidden in cluster creation. The separate commission
	// review shows exact principal/tenant/role/scope and requires its own consent.
	value := map[string]any{"$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#", "contentVersion": "1.0.0.0", "resources": []any{cluster}, "outputs": map[string]any{"clusterResourceId": map[string]string{"type": "string", "value": fit.ClusterResourceID}}}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw) > 128<<10 {
		return "", fmt.Errorf("cluster template is unavailable")
	}
	return string(raw) + "\n", nil
}

// The deterministic name selects ONLY the separately reviewed exact resource
// scope/principal/role. Matching identity alone is not an ownership claim.
func AzureClusterRoleAssignmentName(clusterID, principalID string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(clusterID) + "\x00" + principalID + "\x00" + AzureAKSClusterAdminRole))
	raw := sum[:16]
	raw[6] = (raw[6] & 15) | 80
	raw[8] = (raw[8] & 63) | 128
	encoded := hex.EncodeToString(raw)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
