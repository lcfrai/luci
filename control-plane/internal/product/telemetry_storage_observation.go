package product

import (
	"context"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"strings"
	"time"
)

type AzureTelemetryStorageObservation struct {
	AccountResourceID   string `json:"accountResourceId"`
	AccountCreatedAt    string `json:"accountCreatedAt"`
	ContainerResourceID string `json:"containerResourceId"`
	ContainerVersion    string `json:"containerVersion"`
	State               string `json:"state"`
	ObservedAt          string `json:"observedAt"`
}

func (s *Server) observeTelemetryStorage(ctx context.Context, j CloudJob) (AzureTelemetryStorageObservation, error) {
	if !j.Owned || j.DeleteAttempted || j.Plan.Foundation == nil || j.Plan.Foundation.TelemetryStorage == nil || !validFoundationPlan(j.Plan) {
		return AzureTelemetryStorageObservation{}, errAzureBlueprint
	}
	v := j.Plan.Foundation.TelemetryStorage.Spec
	accountID := "/subscriptions/" + v.SubscriptionID + "/resourceGroups/" + j.Name + "/providers/Microsoft.Storage/storageAccounts/" + v.AccountName
	raw, e := s.cloudJobCommand(ctx, j, "az", []string{"resource", "show", "--ids", accountID, "--api-version", deploymentplan.AzureTelemetryStorageAPI, "--output", "json"}, "")
	if e != nil {
		return AzureTelemetryStorageObservation{}, e
	}
	var account struct {
		ID, Name, Type, Location string
		Tags                     map[string]string `json:"tags"`
		SKU                      struct {
			Name string `json:"name"`
		} `json:"sku"`
		Kind       string `json:"kind"`
		Properties struct {
			Created   string `json:"creationTime"`
			State     string `json:"provisioningState"`
			TLS       string `json:"minimumTlsVersion"`
			HTTPS     bool   `json:"supportsHttpsTrafficOnly"`
			Public    bool   `json:"allowBlobPublicAccess"`
			SharedKey bool   `json:"allowSharedKeyAccess"`
		} `json:"properties"`
	}
	if json.Unmarshal([]byte(raw), &account) != nil || !strings.EqualFold(account.ID, accountID) || account.Name != v.AccountName || !strings.EqualFold(account.Type, "Microsoft.Storage/storageAccounts") || account.Location != j.Region || account.Kind != "StorageV2" || account.SKU.Name != "Standard_LRS" || account.Tags["luci-plan"] != j.Plan.ID || account.Tags["managed-by"] != "luci" || account.Properties.TLS != "TLS1_2" || !account.Properties.HTTPS || account.Properties.Public || !account.Properties.SharedKey {
		return AzureTelemetryStorageObservation{}, errAzureBlueprint
	}
	created, e := time.Parse(time.RFC3339Nano, account.Properties.Created)
	if e != nil || created.After(time.Now().Add(time.Minute)) || created.Before(j.CreatedAt.Add(-time.Minute)) {
		return AzureTelemetryStorageObservation{}, errAzureBlueprint
	}
	if j.TelemetryStorage != nil && (j.TelemetryStorage.AccountResourceID != accountID || j.TelemetryStorage.AccountCreatedAt != account.Properties.Created) {
		return AzureTelemetryStorageObservation{}, errAzureBlueprint
	}
	containerID := accountID + "/blobServices/default/containers/" + v.ContainerName
	raw, e = s.cloudJobCommand(ctx, j, "az", []string{"resource", "show", "--ids", containerID, "--api-version", deploymentplan.AzureTelemetryStorageAPI, "--output", "json"}, "")
	if e != nil {
		return AzureTelemetryStorageObservation{}, e
	}
	var container struct {
		ID         string `json:"id"`
		ETag       string `json:"etag"`
		Properties struct {
			Public string `json:"publicAccess"`
		} `json:"properties"`
	}
	if json.Unmarshal([]byte(raw), &container) != nil || !strings.EqualFold(container.ID, containerID) || container.ETag == "" || len(container.ETag) > 256 || strings.ContainsAny(container.ETag, "\r\n\x00") || container.Properties.Public != "None" {
		return AzureTelemetryStorageObservation{}, errAzureBlueprint
	}
	state := "provisioning"
	if account.Properties.State == "Succeeded" {
		state = "resources_ready"
	} else if account.Properties.State == "Failed" {
		state = "failed"
	}
	return AzureTelemetryStorageObservation{AccountResourceID: accountID, AccountCreatedAt: account.Properties.Created, ContainerResourceID: containerID, ContainerVersion: container.ETag, State: state, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
