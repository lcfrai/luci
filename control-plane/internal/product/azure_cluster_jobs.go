package product

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

// Cluster creation is not access-grant or authenticated namespace readiness.
// ResourceUID is the provider's immutable managed-cluster generation identity,
// distinct from an ARM ID which can be reused after delete/recreate.
type AzureClusterObservation struct {
	ResourceID          string `json:"resourceId"`
	ResourceUID         string `json:"resourceUid"`
	PrincipalID         string `json:"principalId"`
	NodeResourceGroupID string `json:"nodeResourceGroupId"`
	FQDN                string `json:"fqdn"`
	KubernetesVersion   string `json:"kubernetesVersion"`
	State               string `json:"state"`
	ObservedAt          string `json:"observedAt"`
	TargetReady         bool   `json:"targetReady"` // cluster observation alone is always false
	CapacityVerified    bool   `json:"capacityVerified"`
}
type azureManagedCluster struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Location string            `json:"location"`
	Tags     map[string]string `json:"tags"`
	Identity struct {
		Type        string `json:"type"`
		PrincipalID string `json:"principalId"`
		TenantID    string `json:"tenantId"`
	} `json:"identity"`
	Properties struct {
		ResourceUID              string `json:"resourceUID"`
		ProvisioningState        string `json:"provisioningState"`
		KubernetesVersion        string `json:"kubernetesVersion"`
		CurrentKubernetesVersion string `json:"currentKubernetesVersion"`
		NodeResourceGroup        string `json:"nodeResourceGroup"`
		FQDN                     string `json:"fqdn"`
		PrivateFQDN              string `json:"privateFQDN"`
		EnableRBAC               bool   `json:"enableRBAC"`
		DisableLocalAccounts     bool   `json:"disableLocalAccounts"`
		AAD                      struct {
			Managed   bool   `json:"managed"`
			AzureRBAC bool   `json:"enableAzureRBAC"`
			TenantID  string `json:"tenantID"`
		} `json:"aadProfile"`
		Access struct {
			Private bool     `json:"enablePrivateCluster"`
			Ranges  []string `json:"authorizedIPRanges"`
		} `json:"apiServerAccessProfile"`
		Pools []azureManagedClusterPool `json:"agentPoolProfiles"`
	} `json:"properties"`
}

func (s *Server) observeAzureCluster(ctx context.Context, j CloudJob) (AzureClusterObservation, error) {
	if j.Plan.Foundation == nil || j.Plan.Foundation.Cluster == nil || !j.Owned || !validFoundationPlan(j.Plan) {
		return AzureClusterObservation{}, errAzureCluster
	}
	fit := j.Plan.Foundation.Cluster.Fit
	raw, err := s.cloudJobCommand(ctx, j, "az", []string{"resource", "show", "--ids", fit.ClusterResourceID, "--api-version", deploymentplan.AzureClusterAPI(fit.Spec), "--output", "json"}, "")
	if err != nil {
		return AzureClusterObservation{}, err
	}
	return azureClusterObservation(raw, j)
}
func azureClusterObservation(raw string, j CloudJob) (AzureClusterObservation, error) {
	fit := j.Plan.Foundation.Cluster.Fit
	var value azureManagedCluster
	if decodeBoundedAzureJSON(raw, &value) != nil {
		return AzureClusterObservation{}, errAzureCluster
	}
	p := value.Properties
	p.Access.Ranges = append([]string{}, p.Access.Ranges...)
	sort.Strings(p.Access.Ranges)
	if !strings.EqualFold(value.ID, fit.ClusterResourceID) || value.Name != fit.ClusterName || !strings.EqualFold(value.Type, "Microsoft.ContainerService/managedClusters") || value.Location != j.Region || value.Tags["luci-plan"] != j.Plan.ID || value.Identity.Type != "SystemAssigned" || !azureSetupUUID.MatchString(strings.ToLower(value.Identity.PrincipalID)) || strings.ToLower(value.Identity.TenantID) != fit.Spec.TenantID || p.ResourceUID == "" || len(p.ResourceUID) > 128 || !prototypeReleaseID.MatchString(p.ResourceUID) || p.NodeResourceGroup != fit.NodeResourceGroup || p.KubernetesVersion != fit.Spec.KubernetesVersion || !p.EnableRBAC || !p.DisableLocalAccounts || !p.AAD.Managed || !p.AAD.AzureRBAC || strings.ToLower(p.AAD.TenantID) != fit.Spec.TenantID || p.Access.Private != fit.Spec.PrivateAPI || !reflect.DeepEqual(append([]string{}, p.Access.Ranges...), append([]string{}, fit.Spec.AuthorizedAPICIDRs...)) || !azureClusterPoolsMatch(p.Pools, fit.Spec) {
		return AzureClusterObservation{}, errAzureCluster
	}
	fqdn := p.FQDN
	if fit.Spec.PrivateAPI {
		fqdn = p.PrivateFQDN
	}
	if fqdn == "" || len(fqdn) > 253 || strings.ContainsAny(fqdn, "/:@\\ \r\n\t") {
		return AzureClusterObservation{}, errAzureCluster
	}
	if j.Cluster != nil && (j.Cluster.ResourceUID != p.ResourceUID || j.Cluster.PrincipalID != strings.ToLower(value.Identity.PrincipalID) || !strings.EqualFold(j.Cluster.ResourceID, value.ID)) {
		return AzureClusterObservation{}, errAzureCluster
	}
	state := "provisioning"
	switch p.ProvisioningState {
	case "Succeeded":
		state = "created"
	case "Failed", "Canceled":
		state = "failed"
	}
	if fit.Spec.GPUPool != nil && state == "created" {
		for _, pool := range p.Pools {
			if pool.Name == "gpu" && pool.State != "Succeeded" {
				state = "unconfirmed"
			}
		}
	}
	version := p.CurrentKubernetesVersion
	if state == "created" && version != fit.Spec.KubernetesVersion {
		state = "unconfirmed"
	}
	return AzureClusterObservation{ResourceID: fit.ClusterResourceID, ResourceUID: p.ResourceUID, PrincipalID: strings.ToLower(value.Identity.PrincipalID), NodeResourceGroupID: "/subscriptions/" + fit.Spec.SubscriptionID + "/resourceGroups/" + fit.NodeResourceGroup, FQDN: strings.ToLower(fqdn), KubernetesVersion: version, State: state, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
