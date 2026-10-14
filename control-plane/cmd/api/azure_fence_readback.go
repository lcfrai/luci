package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// The movable ext4 disk cannot itself elect a writer while it is detached.
// The private Blob is a separate, ETag-conditional transition journal written
// only by the recovery identity. The ordinary controller identity can read it
// but has no Blob write or VM mutation grant.
type azureFenceRecord struct {
	Version                 int    `json:"version"`
	State                   string `json:"state"`
	DiskID                  string `json:"diskID"`
	FilesystemUUID          string `json:"filesystemUUID"`
	OwnerID                 string `json:"ownerID"`
	Scope                   string `json:"scope"`
	ControllerID            string `json:"controllerID"`
	Generation              string `json:"generation"`
	GenerationReceiptSHA256 string `json:"generationReceiptSHA256"`
	FenceReceiptDigest      string `json:"fenceReceiptDigest"`
	TargetControllerID      string `json:"targetControllerID"`
	TargetGeneration        string `json:"targetGeneration"`
	RetirementPlanDigest    string `json:"retirementPlanDigest"`
	RetirementSourceCommit  string `json:"retirementSourceCommit"`
}

type azureFenceReader interface {
	Read(context.Context, string) (azureFenceRecord, error)
}

type azureFenceClient struct {
	credential *azidentity.ManagedIdentityCredential
	http       *http.Client
}

var azureFenceURLPattern = regexp.MustCompile(`^https://[a-z0-9]{3,24}\.blob\.core\.windows\.net/controller-fence/controller-fence\.json$`)

func validAzureFenceBlobURL(value string) bool { return azureFenceURLPattern.MatchString(value) }

func newAzureFenceReader(clientID string) (azureFenceReader, error) {
	if !azureClientIDPattern.MatchString(clientID) {
		return nil, errors.New("Azure fence reader identity is invalid")
	}
	credential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{ID: azidentity.ClientID(clientID)})
	if err != nil {
		return nil, errors.New("Azure fence reader managed identity is unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &azureFenceClient{credential: credential, http: &http.Client{Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Azure fence redirect refused") }}}, nil
}

func privateAzureBlobAddresses(ctx context.Context, host string) bool {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return false
	}
	blocks := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, address := range addresses {
		found := false
		for _, block := range blocks {
			_, subnet, _ := net.ParseCIDR(block)
			if subnet.Contains(address.IP) && address.IP.To4() != nil {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c *azureFenceClient) Read(ctx context.Context, blobURL string) (azureFenceRecord, error) {
	var record azureFenceRecord
	if !validAzureFenceBlobURL(blobURL) {
		return record, errors.New("Azure fence Blob identity is invalid")
	}
	bound, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	u, _ := url.Parse(blobURL)
	if !privateAzureBlobAddresses(bound, u.Hostname()) {
		return record, errors.New("Azure fence Blob is not on the private endpoint")
	}
	token, err := c.credential.GetToken(bound, policy.TokenRequestOptions{Scopes: []string{"https://storage.azure.com/.default"}})
	if err != nil {
		return record, errors.New("Azure fence reader token is unavailable")
	}
	req, _ := http.NewRequestWithContext(bound, http.MethodGet, blobURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("x-ms-version", "2023-11-03")
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
	response, err := c.http.Do(req)
	if err != nil {
		return record, errors.New("Azure fence Blob read is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("ETag") == "" {
		return record, errors.New("Azure fence Blob is not current or accessible")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(data) == 0 || len(data) > 8192 {
		return record, errors.New("Azure fence Blob is incomplete")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	// The writer retains later stage fields. Decode the exact fields relevant
	// to admission while refusing malformed or trailing JSON.
	if decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF {
		return record, errors.New("Azure fence Blob is malformed")
	}
	return record, nil
}

func verifyAzureFence(ctx context.Context, selected e2bOperatorConfig, reader azureFenceReader, startup bool) error {
	if selected.AzureMount == nil || reader == nil || !validAzureFenceBlobURL(selected.AzureMount.FenceBlobURL) {
		return errors.New("Azure controller fence inputs are invalid")
	}
	record, err := reader.Read(ctx, selected.AzureMount.FenceBlobURL)
	if err != nil {
		return err
	}
	m := selected.AzureMount
	if record.Version != 1 || strings.ToLower(record.DiskID) != selected.Store.AzureDataDiskID ||
		record.FilesystemUUID != m.FilesystemUUID || record.OwnerID != selected.Store.OwnerID ||
		record.Scope != selected.Store.RuntimeScope {
		return errors.New("Azure off-disk controller writer identity changed")
	}
	switch record.State {
	case "active":
		if strings.ToLower(record.ControllerID) != m.ControllerInstanceID || record.Generation != m.Generation ||
			record.GenerationReceiptSHA256 != m.GenerationReceiptDigest {
			return errors.New("Azure off-disk controller generation changed")
		}
	case "bootstrap_mount_pending":
		if !startup || strings.ToLower(record.ControllerID) != m.ControllerInstanceID || record.Generation != m.Generation {
			return errors.New("Azure controller bootstrap is pending; new effects refused")
		}
	case "api_start_pending":
		// The old writer remains in controllerID until the replacement API is
		// healthy. Only its exact target generation may start; no cloud effect
		// may run until the recovery identity commits the active transition.
		if !startup || strings.ToLower(record.TargetControllerID) != m.ControllerInstanceID ||
			record.TargetGeneration != m.Generation || record.ControllerID == record.TargetControllerID ||
			record.GenerationReceiptSHA256 != m.GenerationReceiptDigest {
			return errors.New("Azure controller restoration is pending; new effects refused")
		}
	default:
		return errors.New("Azure controller restoration is pending; new effects refused")
	}
	data, err := readPrivateE2BFile(m.GenerationReceiptPath, 4096)
	if err != nil {
		return errors.New("Azure on-disk generation receipt is unavailable")
	}
	var generation azureGenerationReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&generation) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("Azure on-disk generation receipt is malformed")
	}
	if generation.PreviousGeneration != "" && (!azureDigestPattern.MatchString(record.FenceReceiptDigest) ||
		record.FenceReceiptDigest != generation.PreviousFenceReceiptDigest) {
		return errors.New("Azure prior controller fence is not durably bound")
	}
	return nil
}
