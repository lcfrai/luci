package deploymentplan

import (
	"encoding/json"
	"fmt"
	"strings"
)

const AzureTelemetryStorageAPI = "2025-06-01"

type AzureTelemetryStorageSpec struct {
	SubscriptionID string `json:"subscriptionId"`
	TenantID       string `json:"tenantId"`
	AccountName    string `json:"accountName"`
	ContainerName  string `json:"containerName"`
	RetentionDays  int    `json:"retentionDays"`
}

func ValidAzureTelemetryStorage(v AzureTelemetryStorageSpec) bool {
	return azureUUID.MatchString(v.SubscriptionID) && azureUUID.MatchString(v.TenantID) && blobAccountName.MatchString(v.AccountName) && blobContainerName.MatchString(v.ContainerName) && !strings.Contains(v.ContainerName, "--") && v.RetentionDays >= 1 && v.RetentionDays <= 31
}

// A separate existing CloudJob creates only a dedicated StorageV2 account,
// private container and lifecycle rule. It grants no IAM, Kubernetes rights,
// public blob access, namespace, image or implicit Secret installation.
func AzureTelemetryStorageTemplate(v AzureTelemetryStorageSpec, region, planID string) (string, error) {
	if !ValidAzureTelemetryStorage(v) || !azureClusterName.MatchString(region) || !text(planID, 128) || planID == "" {
		return "", fmt.Errorf("choose exact Azure subscription, tenant, globally unique account, private container and retention")
	}
	account := map[string]any{"type": "Microsoft.Storage/storageAccounts", "apiVersion": AzureTelemetryStorageAPI, "name": v.AccountName, "location": region, "kind": "StorageV2", "sku": map[string]string{"name": "Standard_LRS"}, "tags": map[string]string{"managed-by": "luci", "luci-plan": planID}, "properties": map[string]any{"minimumTlsVersion": "TLS1_2", "supportsHttpsTrafficOnly": true, "allowBlobPublicAccess": false, "allowSharedKeyAccess": true, "defaultToOAuthAuthentication": true, "publicNetworkAccess": "Enabled", "accessTier": "Hot", "encryption": map[string]any{"keySource": "Microsoft.Storage", "services": map[string]any{"blob": map[string]bool{"enabled": true}}}}}
	container := map[string]any{"type": "Microsoft.Storage/storageAccounts/blobServices/containers", "apiVersion": AzureTelemetryStorageAPI, "name": v.AccountName + "/default/" + v.ContainerName, "dependsOn": []string{"[resourceId('Microsoft.Storage/storageAccounts', '" + v.AccountName + "')]"}, "properties": map[string]string{"publicAccess": "None"}}
	policy := map[string]any{"type": "Microsoft.Storage/storageAccounts/managementPolicies", "apiVersion": AzureTelemetryStorageAPI, "name": v.AccountName + "/default", "dependsOn": []string{"[resourceId('Microsoft.Storage/storageAccounts', '" + v.AccountName + "')]"}, "properties": map[string]any{"policy": map[string]any{"rules": []any{map[string]any{"enabled": true, "name": "luci-trace-retention", "type": "Lifecycle", "definition": map[string]any{"filters": map[string]any{"blobTypes": []string{"blockBlob"}, "prefixMatch": []string{v.ContainerName + "/"}}, "actions": map[string]any{"baseBlob": map[string]any{"delete": map[string]int{"daysAfterModificationGreaterThan": v.RetentionDays + 2}}}}}}}}}
	value := map[string]any{"$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#", "contentVersion": "1.0.0.0", "resources": []any{account, container, policy}, "outputs": map[string]any{"storageAccount": map[string]string{"type": "string", "value": v.AccountName}, "container": map[string]string{"type": "string", "value": v.ContainerName}}}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}
