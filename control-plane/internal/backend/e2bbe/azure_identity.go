package e2bbe

import (
	"net/url"
	"regexp"
	"strings"
)

// GatewayReceiptDigest is a nonsecret exact route-generation contract. It is
// returned by the private mTLS gateway and pinned in the controller config;
// live TLS and E2B API reads remain separate acceptance checks.
func GatewayReceiptDigest(host, scope, generation string) string {
	return digest([]string{"luci-azure-e2b-gateway-v1", host, scope, generation, "api:3000", "guest:3002", "envd:49983", "sandboxd:8080"})
}

var (
	azureSubscriptionPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	azureNamePattern         = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}[a-z0-9]$|^[a-z0-9]$`)
	azureSizePattern         = regexp.MustCompile(`^Standard_[A-Za-z0-9_]{3,48}$`)
	azureRegionPattern       = regexp.MustCompile(`^[a-z][a-z0-9]{1,31}$`)
	azureInstancePattern     = regexp.MustCompile(`^[0-9]{1,12}$`)
	azureGatewayPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,252}$`)
	azureDiskPattern         = regexp.MustCompile(`^/subscriptions/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/resourcegroups/[a-z0-9][a-z0-9_-]{0,63}/providers/microsoft.compute/disks/[a-z0-9][a-z0-9_-]{0,63}$`)
)

// Azure ARM IDs are case-insensitive at the provider, but Luci requires one
// lowercase spelling to keep restore, task metadata and charge identity exact.
// This validates configured identity; it does not claim SKU/region availability,
// KVM support, an attached disk or a provider price.
func ValidAzureDeployment(d Deployment, endpoint string) bool {
	if d.Provider != "azure" || !azureSubscriptionPattern.MatchString(d.AzureSubscriptionID) ||
		!azureNamePattern.MatchString(d.AzureResourceGroup) || !azureRegionPattern.MatchString(d.Region) ||
		!azureSizePattern.MatchString(d.InstanceType) || !azureInstancePattern.MatchString(d.AzureInstanceID) ||
		(d.AzureZone != "1" && d.AzureZone != "2" && d.AzureZone != "3") ||
		!azureGatewayPattern.MatchString(d.AzureGatewayHost) || strings.Contains(d.AzureGatewayHost, "..") {
		return false
	}
	prefix := "/subscriptions/" + d.AzureSubscriptionID + "/resourcegroups/" + d.AzureResourceGroup + "/providers/microsoft.compute/virtualmachinescalesets/"
	if !strings.HasPrefix(d.AzureScaleSetID, prefix) || !azureNamePattern.MatchString(strings.TrimPrefix(d.AzureScaleSetID, prefix)) ||
		d.AzureInstanceID == "" ||
		d.AzureDataDiskID == "" || !strings.HasPrefix(d.AzureDataDiskID, "/subscriptions/"+d.AzureSubscriptionID+"/resourcegroups/") {
		return false
	}
	diskParts := strings.Split(strings.TrimPrefix(d.AzureDataDiskID, "/subscriptions/"+d.AzureSubscriptionID+"/resourcegroups/"), "/providers/microsoft.compute/disks/")
	if len(diskParts) != 2 || !azureNamePattern.MatchString(diskParts[0]) || !azureNamePattern.MatchString(diskParts[1]) {
		return false
	}
	u, err := url.Parse(endpoint)
	// The Azure management endpoint is the protected gateway's fixed listener.
	// Accepting another port could route directly to an E2B service on the same
	// private host and bypass its mTLS and generation-receipt boundary.
	return err == nil && u.Scheme == "https" && u.Host == d.AzureGatewayHost &&
		u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

func hasAzureDeploymentFields(d Deployment) bool {
	return d.AzureSubscriptionID != "" || d.AzureResourceGroup != "" || d.AzureScaleSetID != "" ||
		d.AzureInstanceID != "" || d.AzureDataDiskID != "" || d.AzureZone != "" || d.AzureGatewayHost != ""
}

func validDeployment(c Config) bool {
	d := c.Deployment
	if !validID(d.Scope) || !validID(d.HostGeneration) ||
		d.RuntimeRevision != RuntimeRevision || !digestPattern.MatchString(d.ManagementReceiptDigest) ||
		!digestPattern.MatchString(d.RestoreReceiptDigest) {
		return false
	}
	switch d.Provider {
	case "aws":
		return !hasAzureDeploymentFields(d) && validID(d.Region) && supportedEmbedInstanceType(d.InstanceType)
	case "azure":
		return c.ClientCertificate != nil && ValidAzureDeployment(d, c.Endpoint)
	default:
		return false
	}
}
