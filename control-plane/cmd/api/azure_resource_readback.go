package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const azureARMOrigin = "https://management.azure.com"

type azureARMResource struct {
	ID                string   `json:"id"`
	ManagedBy         string   `json:"managedBy"`
	ManagedByExtended []string `json:"managedByExtended"`
	Properties        struct {
		DiskState         string `json:"diskState"`
		MaxShares         *int   `json:"maxShares"`
		ProvisioningState string `json:"provisioningState"`
	} `json:"properties"`
}

type azureARMReader interface {
	Read(context.Context, string, string, *azureARMResource) error
}

type azureARMClient struct {
	credential *azidentity.ManagedIdentityCredential
	http       *http.Client
}

func newAzureARMReader(clientID string) (azureARMReader, error) {
	if !azureClientIDPattern.MatchString(clientID) {
		return nil, errors.New("Azure controller managed identity is invalid")
	}
	credential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{ID: azidentity.ClientID(clientID)})
	if err != nil {
		return nil, errors.New("Azure controller managed identity is unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &azureARMClient{credential: credential, http: &http.Client{Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Azure ARM redirect refused") }}}, nil
}

func (c *azureARMClient) Read(ctx context.Context, path, apiVersion string, out *azureARMResource) error {
	if out == nil || !strings.HasPrefix(path, "/subscriptions/") || strings.ContainsAny(path, "?#") ||
		apiVersion != "2023-10-02" && apiVersion != "2023-09-01" {
		return errors.New("Azure ARM resource request is unsupported")
	}
	bound, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	token, err := c.credential.GetToken(bound, policy.TokenRequestOptions{Scopes: []string{azureARMOrigin + "/.default"}})
	if err != nil {
		return errors.New("Azure ARM managed identity token is unavailable")
	}
	u := azureARMOrigin + path + "?api-version=" + url.QueryEscape(apiVersion)
	req, err := http.NewRequestWithContext(bound, http.MethodGet, u, nil)
	if err != nil {
		return errors.New("Azure ARM resource request is invalid")
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return errors.New("Azure ARM resource read is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("Azure ARM resource is not current or accessible")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("Azure ARM resource read is incomplete")
	}
	return nil
}

// A configured ARM ID and a mounted UUID are not themselves attachment proof.
// This readback checks the independently managed disk's current controller
// attachment and both host resources before new Create/Start effects. It never
// provisions, detaches, stops or retries an uncertain E2B operation.
func verifyAzureARMOwnership(ctx context.Context, selected e2bOperatorConfig, reader azureARMReader) error {
	if reader == nil || selected.AzureMount == nil ||
		!azureControllerInstancePattern.MatchString(selected.AzureMount.ControllerInstanceID) ||
		!azureDiskIDMatches(selected.Store.AzureDataDiskID, selected.Deployment.AzureSubscriptionID) {
		return errors.New("Azure ARM ownership inputs are invalid")
	}
	var disk azureARMResource
	if err := reader.Read(ctx, selected.Store.AzureDataDiskID, "2023-10-02", &disk); err != nil ||
		strings.ToLower(disk.ID) != selected.Store.AzureDataDiskID ||
		strings.ToLower(disk.ManagedBy) != selected.AzureMount.ControllerInstanceID || disk.Properties.DiskState != "Attached" ||
		// Azure's ordinary nonshared disk leaves maxShares unset; only an
		// explicit value above one enables sharing. Terraform's managed-disk
		// schema likewise permits max_shares only for shared values 2..10.
		disk.Properties.MaxShares != nil && *disk.Properties.MaxShares != 1 ||
		len(disk.ManagedByExtended) > 1 || len(disk.ManagedByExtended) == 1 && strings.ToLower(disk.ManagedByExtended[0]) != selected.AzureMount.ControllerInstanceID {
		return errors.New("Azure controller data disk attachment is unverified")
	}
	var controller azureARMResource
	if err := reader.Read(ctx, selected.AzureMount.ControllerInstanceID, "2023-09-01", &controller); err != nil ||
		strings.ToLower(controller.ID) != selected.AzureMount.ControllerInstanceID || controller.Properties.ProvisioningState != "Succeeded" {
		return errors.New("Azure controller instance is unverified")
	}
	runtimeID := selected.Deployment.AzureScaleSetID + "/virtualMachines/" + selected.Deployment.AzureInstanceID
	var runtime azureARMResource
	if err := reader.Read(ctx, runtimeID, "2023-09-01", &runtime); err != nil ||
		strings.ToLower(runtime.ID) != strings.ToLower(runtimeID) || runtime.Properties.ProvisioningState != "Succeeded" {
		return errors.New("Azure E2B host instance is unverified")
	}
	return nil
}

func azureDiskIDMatches(disk, subscription string) bool {
	return strings.HasPrefix(disk, "/subscriptions/"+subscription+"/resourcegroups/") && strings.Contains(disk, "/providers/microsoft.compute/disks/")
}
