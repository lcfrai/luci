package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// This is private operator authority, not a project-supplied ARM template.
type azureElasticSelection struct {
	api.AzureElasticOperator
	HostCustomData      string `json:"hostCustomData"`
	PendingTaskSlots    int    `json:"pendingTaskSlots"`
	VMSSModelDigest     string `json:"vmssModelDigest"`
	HostResourceGroupID string `json:"hostResourceGroupId"`
}
type azureElasticDriver struct {
	selection azureElasticSelection
	base      e2bOperatorConfig
	worker    Config
	projects  *product.Server
	authorize management.ExecutionAuthorizer
	arm       *azureARMClient
}
type elasticVMSS struct {
	ID       string `json:"id"`
	ETag     string `json:"etag"`
	Location string `json:"location"`
	SKU      struct {
		Name     string `json:"name"`
		Tier     string `json:"tier"`
		Capacity int    `json:"capacity"`
	} `json:"sku"`
	Identity   json.RawMessage `json:"identity"`
	Zones      []string        `json:"zones"`
	Properties struct {
		UniqueID              string          `json:"uniqueId"`
		ProvisioningState     string          `json:"provisioningState"`
		VirtualMachineProfile json.RawMessage `json:"virtualMachineProfile"`
	} `json:"properties"`
}
type elasticInstance struct {
	ID         string            `json:"id"`
	ETag       string            `json:"etag"`
	Tags       map[string]string `json:"tags"`
	Properties struct {
		VMID              string `json:"vmId"`
		ProvisioningState string `json:"provisioningState"`
	} `json:"properties"`
}

func elasticHash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func elasticModel(v elasticVMSS) string {
	var profile, identity any
	if json.Unmarshal(v.Properties.VirtualMachineProfile, &profile) != nil || json.Unmarshal(v.Identity, &identity) != nil {
		return ""
	}
	if object, ok := profile.(map[string]any); ok {
		if osProfile, ok := object["osProfile"].(map[string]any); ok {
			delete(osProfile, "customData")
		}
	}
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(false)
	if encoder.Encode([]any{strings.ToLower(v.ID), v.Location, v.SKU.Name, v.SKU.Tier, v.Zones, profile, identity, v.Properties.UniqueID}) != nil {
		return ""
	}
	sum := sha256.Sum256(bytes.TrimSuffix(raw.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

type azureHostNotSent struct{}

func (azureHostNotSent) Error() string { return "Original host request was not sent" }

type azureHostRejected struct{}

func (azureHostRejected) Error() string {
	return "Azure rejected the original host mutation; it was not retried"
}
func (d *azureElasticDriver) call(ctx context.Context, method, path, version string, body any, out any, authorize func() error, record ...func(http.Header)) (bool, error) {
	readTemplate := method == "GET" && strings.EqualFold(path, d.base.Deployment.AzureScaleSetID)
	owned := strings.HasPrefix(strings.ToLower(path), strings.ToLower(d.selection.HostResourceGroupID)+"/providers/")
	if !readTemplate && !owned || strings.ContainsAny(path, "?#") || (version != "2025-05-01" && version != "2025-04-01" && version != "2023-10-02") || (method != "GET" && method != "PUT" && method != "DELETE") {
		return false, e2bbe.ErrUnsupported
	}
	child, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	bearer, err := d.arm.credential.GetToken(child, policy.TokenRequestOptions{Scopes: []string{azureARMOrigin + "/.default"}})
	if err != nil {
		if method == "PUT" {
			return false, azureHostNotSent{}
		}
		return false, e2bbe.ErrUnknown
	}
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return false, err
		}
	}
	req, err := http.NewRequestWithContext(child, method, azureARMOrigin+path+"?api-version="+url.QueryEscape(version), bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		if strings.Contains(strings.ToLower(path), "/providers/microsoft.compute/virtualmachines/") {
			req.Header.Set("If-None-Match", "*")
		}
	}
	// Recheck policy after token/account work and immediately before each new write.
	if method == "PUT" && (d.verifyController(ctx) != nil || authorize == nil || authorize() != nil) {
		return false, azureHostNotSent{}
	}
	response, err := d.arm.http.Do(req)
	if err != nil {
		return false, e2bbe.ErrUnknown
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode <= 202 && len(record) == 1 {
		record[0](response.Header)
	}
	if method == "GET" && response.StatusCode == 404 {
		return true, nil
	}
	if method == "PUT" && response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 && response.StatusCode != 429 {
		return false, azureHostRejected{}
	}
	if response.StatusCode < 200 || response.StatusCode > 202 {
		return false, e2bbe.ErrUnknown
	}
	if out != nil {
		decoder := json.NewDecoder(io.LimitReader(response.Body, (2<<20)+1))
		if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
			return false, e2bbe.ErrUnknown
		}
	}
	return false, nil
}
func (d *azureElasticDriver) model(ctx context.Context) (elasticVMSS, error) {
	var v elasticVMSS
	missing, err := d.call(ctx, "GET", d.base.Deployment.AzureScaleSetID, "2025-04-01", nil, &v, nil)
	if err != nil || missing || v.ETag == "" || strings.ToLower(v.ID) != d.base.Deployment.AzureScaleSetID || v.Properties.ProvisioningState != "Succeeded" || elasticModel(v) != d.selection.VMSSModelDigest {
		return v, e2bbe.ErrConflict
	}
	return v, nil
}
func elasticMap(v any) (map[string]any, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var result map[string]any
	e = json.Unmarshal(raw, &result)
	return result, e
}
func elasticSubset(want, actual any) bool {
	switch value := want.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range value {
			if !elasticSubset(v, a[k]) {
				return false
			}
		}
		return true
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) != len(value) {
			return false
		}
		for i, v := range value {
			if !elasticSubset(v, a[i]) {
				return false
			}
		}
		return true
	case string:
		a, ok := actual.(string)
		if !ok {
			return false
		}
		if strings.HasPrefix(strings.ToLower(value), "/subscriptions/") {
			return strings.EqualFold(value, a)
		}
		return value == a
	default:
		return elasticHash(want) == elasticHash(actual)
	}
}
func (d *azureElasticDriver) Prepare(ctx context.Context, h api.AzureElasticHost) (api.AzureElasticHost, error) {
	if err := verifyAzureDurableMount(*d.base.AzureMount, d.base, d.worker); err != nil {
		return h, err
	}
	model, err := d.model(ctx)
	if err != nil {
		return h, err
	}
	var profile map[string]any
	if json.Unmarshal(model.Properties.VirtualMachineProfile, &profile) != nil {
		return h, e2bbe.ErrUnsupported
	}
	network, _ := profile["networkProfile"].(map[string]any)
	configs, _ := network["networkInterfaceConfigurations"].([]any)
	if len(configs) != 1 {
		return h, e2bbe.ErrUnsupported
	}
	nic, _ := configs[0].(map[string]any)
	props, _ := nic["properties"].(map[string]any)
	ips, _ := props["ipConfigurations"].([]any)
	if len(ips) != 1 {
		return h, e2bbe.ErrUnsupported
	}
	ip, _ := ips[0].(map[string]any)
	ipProps, _ := ip["properties"].(map[string]any)
	subnet, _ := ipProps["subnet"].(map[string]any)
	subnetID, _ := subnet["id"].(string)
	if subnetID == "" || ipProps["publicIPAddressConfiguration"] != nil {
		return h, e2bbe.ErrUnsupported
	}
	if extensions, ok := profile["extensionProfile"].(map[string]any); ok {
		if entries, ok := extensions["extensions"].([]any); ok && len(entries) != 0 {
			return h, e2bbe.ErrUnsupported
		}
	}
	bootstrap, e := base64.StdEncoding.DecodeString(d.selection.HostCustomData)
	if e != nil || len(bootstrap) == 0 || len(d.selection.HostCustomData) >= 65536 || !bytes.HasPrefix(bootstrap, []byte("#cloud-config\n")) {
		return h, e2bbe.ErrUnsupported
	}
	// Names include an unexposed random intent marker, not a caller-selected identifier.
	suffix := elasticHash([]string{h.ID, h.OwnershipMarker})[:32]
	h.ResourceID = strings.ToLower(d.selection.HostResourceGroupID) + "/providers/microsoft.compute/virtualmachines/luci-" + suffix
	h.NICResourceID = strings.ToLower(d.selection.HostResourceGroupID) + "/providers/microsoft.network/networkinterfaces/luci-" + suffix
	h.ScaleSetID = d.base.Deployment.AzureScaleSetID
	h.ModelDigest = d.selection.VMSSModelDigest
	tags := map[string]string{"luci-capacity-intent": elasticHash([]string{h.OwnershipMarker, h.ID, h.ModelDigest}), "luci-capacity-profile": h.ModelDigest}
	nicProps := map[string]any{"ipConfigurations": []any{map[string]any{"name": "private", "properties": map[string]any{"privateIPAllocationMethod": "Dynamic", "subnet": map[string]string{"id": subnetID}}}}}
	if accelerated, ok := props["enableAcceleratedNetworking"].(bool); ok {
		nicProps["enableAcceleratedNetworking"] = accelerated
	}
	networkRequest := map[string]any{"location": model.Location, "tags": tags, "properties": nicProps}
	storageSource, _ := profile["storageProfile"].(map[string]any)
	image, _ := storageSource["imageReference"].(map[string]any)
	imageID, _ := image["id"].(string)
	osDiskSource, _ := storageSource["osDisk"].(map[string]any)
	managed, _ := osDiskSource["managedDisk"].(map[string]any)
	storageType, _ := managed["storageAccountType"].(string)
	osSource, _ := profile["osProfile"].(map[string]any)
	linux, _ := osSource["linuxConfiguration"].(map[string]any)
	admin, _ := osSource["adminUsername"].(string)
	if data := storageSource["dataDisks"]; data != nil {
		disks, ok := data.([]any)
		if !ok || len(disks) != 0 {
			return h, e2bbe.ErrUnsupported
		}
	}
	if imageID == "" || admin == "" || linux["disablePasswordAuthentication"] != true || storageType == "" || osDiskSource["diffDiskSettings"] != nil || osDiskSource["diskEncryptionSet"] != nil {
		return h, e2bbe.ErrUnsupported
	}
	storage := map[string]any{"imageReference": map[string]string{"id": imageID}, "osDisk": map[string]any{"createOption": "FromImage", "name": filepath.Base(h.ResourceID) + "-os", "deleteOption": "Delete", "managedDisk": map[string]string{"storageAccountType": storageType}}}
	disk := storage["osDisk"].(map[string]any)
	for _, key := range []string{"caching", "diskSizeGB"} {
		if value, ok := osDiskSource[key]; ok {
			disk[key] = value
		}
	}
	linuxRequest := map[string]any{"disablePasswordAuthentication": true}
	for _, key := range []string{"ssh", "provisionVMAgent"} {
		if value, ok := linux[key]; ok {
			linuxRequest[key] = value
		}
	}
	osProfile := map[string]any{"computerName": "luci-" + suffix[:10], "adminUsername": admin, "customData": d.selection.HostCustomData, "linuxConfiguration": linuxRequest}
	var identitySource map[string]any
	if json.Unmarshal(model.Identity, &identitySource) != nil || identitySource["type"] != "UserAssigned" {
		return h, e2bbe.ErrUnsupported
	}
	assigned, ok := identitySource["userAssignedIdentities"].(map[string]any)
	if !ok || len(assigned) != 1 {
		return h, e2bbe.ErrUnsupported
	}
	identities := map[string]any{}
	for id := range assigned {
		identities[id] = map[string]any{}
	}
	identity := map[string]any{"type": "UserAssigned", "userAssignedIdentities": identities}
	vmProps := map[string]any{"hardwareProfile": map[string]string{"vmSize": model.SKU.Name}, "storageProfile": storage, "osProfile": osProfile, "networkProfile": map[string]any{"networkInterfaces": []any{map[string]any{"id": h.NICResourceID, "properties": map[string]bool{"primary": true}}}}}
	vmRequest := map[string]any{"location": model.Location, "zones": model.Zones, "tags": tags, "identity": identity, "properties": vmProps}
	h.NetworkRequest, _ = json.Marshal(networkRequest)
	h.VMRequest, _ = json.Marshal(vmRequest)
	// Preexisting exact names are refused, even if an operator supplied similar tags.
	var existing map[string]any
	for _, resource := range []struct{ id, version string }{{h.NICResourceID, "2025-05-01"}, {h.ResourceID, "2025-04-01"}} {
		absent, e := d.call(ctx, "GET", resource.id, resource.version, nil, &existing, nil)
		if e != nil || !absent {
			return h, e2bbe.ErrConflict
		}
	}
	return h, nil
}
func (d *azureElasticDriver) CreateNetwork(ctx context.Context, h api.AzureElasticHost, authorize func() error) (api.AzureElasticHost, error) {
	if !h.NetworkAttempted || h.NICResourceID == "" || len(h.NetworkRequest) == 0 {
		return h, e2bbe.ErrConflict
	}
	if _, err := d.model(ctx); err != nil {
		h.NetworkRejected = true
		return h, err
	}
	var result map[string]any
	_, err := d.call(ctx, "PUT", h.NICResourceID, "2025-05-01", json.RawMessage(h.NetworkRequest), &result, func() error {
		var current map[string]any
		absent, e := d.call(ctx, "GET", h.NICResourceID, "2025-05-01", nil, &current, nil)
		if e != nil || !absent {
			return azureHostNotSent{}
		}
		return authorize()
	}, func(headers http.Header) {
		h.NetworkARMOperation = elasticOperationURL(headers)
		h.NetworkRequestID = elasticRequestID(headers)
	})
	if err == nil {
		props, _ := result["properties"].(map[string]any)
		guid, _ := props["resourceGuid"].(string)
		if azureClientIDPattern.MatchString(strings.ToLower(guid)) {
			h.NICUID = strings.ToLower(guid)
		}
	}
	var rejected azureHostRejected
	var unsent azureHostNotSent
	if errors.As(err, &rejected) || errors.As(err, &unsent) {
		h.NetworkRejected = true
	}
	return h, err
}
func (d *azureElasticDriver) CreateVM(ctx context.Context, h api.AzureElasticHost, authorize func() error) (api.AzureElasticHost, error) {
	if !h.ScaleAttempted || h.NICUID == "" || h.ResourceID == "" || len(h.VMRequest) == 0 {
		return h, e2bbe.ErrConflict
	}
	if _, err := d.model(ctx); err != nil {
		h.CreateRejected = true
		h.State = "draining"
		return h, err
	}
	if _, _, err := d.address(ctx, h); err != nil {
		h.CreateRejected = true
		h.State = "draining"
		return h, err
	}
	var result map[string]any
	_, err := d.call(ctx, "PUT", h.ResourceID, "2025-04-01", json.RawMessage(h.VMRequest), &result, authorize, func(headers http.Header) {
		h.VMARMOperation = elasticOperationURL(headers)
		h.VMRequestID = elasticRequestID(headers)
	})
	if err == nil {
		props, _ := result["properties"].(map[string]any)
		id, _ := props["vmId"].(string)
		if azureClientIDPattern.MatchString(strings.ToLower(id)) {
			h.VMID = strings.ToLower(id)
		}
	}
	var rejected azureHostRejected
	var unsent azureHostNotSent
	if errors.As(err, &rejected) || errors.As(err, &unsent) {
		h.CreateRejected = true
		h.State = "draining"
	}
	return h, err
}
func (d *azureElasticDriver) resource(ctx context.Context, id, version string, want json.RawMessage) (map[string]any, bool, error) {
	var current map[string]any
	absent, err := d.call(ctx, "GET", id, version, nil, &current, nil)
	if err != nil || absent {
		return nil, absent, err
	}
	var expected any
	if json.Unmarshal(want, &expected) != nil {
		return nil, false, e2bbe.ErrConflict
	}
	expected = elasticReadbackProjection(expected)
	if !elasticSubset(expected, current) || !strings.EqualFold(fmt.Sprint(current["id"]), id) {
		return nil, false, e2bbe.ErrConflict
	}
	return current, false, nil
}
func (d *azureElasticDriver) address(ctx context.Context, h api.AzureElasticHost) (string, string, error) {
	row, absent, err := d.resource(ctx, h.NICResourceID, "2025-05-01", h.NetworkRequest)
	if err != nil || absent {
		return "", "", e2bbe.ErrUnknown
	}
	props, _ := row["properties"].(map[string]any)
	guid, _ := props["resourceGuid"].(string)
	ips, _ := props["ipConfigurations"].([]any)
	if !azureClientIDPattern.MatchString(strings.ToLower(guid)) || h.NICUID != "" && !strings.EqualFold(guid, h.NICUID) || len(ips) != 1 {
		return "", "", e2bbe.ErrConflict
	}
	ip, _ := ips[0].(map[string]any)
	ipProps, _ := ip["properties"].(map[string]any)
	address, _ := ipProps["privateIPAddress"].(string)
	parsed := net.ParseIP(address)
	if parsed == nil || parsed.To4() == nil || !parsed.IsPrivate() || ipProps["publicIPAddress"] != nil {
		return "", "", e2bbe.ErrUnknown
	}
	ref, _ := props["virtualMachine"].(map[string]any)
	if len(ref) > 0 && (!h.ScaleAttempted || !strings.EqualFold(fmt.Sprint(ref["id"]), h.ResourceID)) {
		return "", "", e2bbe.ErrConflict
	}
	if h.PrivateIP != "" && h.PrivateIP != address {
		return "", "", e2bbe.ErrConflict
	}
	return strings.ToLower(guid), address, nil
}
func (d *azureElasticDriver) Observe(ctx context.Context, h api.AzureElasticHost) (api.AzureElasticHost, error) {
	d = d.originalGroup(h)
	if h.NetworkRejected {
		return h, nil
	}
	guid, ip, err := d.address(ctx, h)
	if err != nil {
		return h, err
	}
	h.NICUID = guid
	h.PrivateIP = ip
	h.NICID = h.NICResourceID
	if !h.ScaleAttempted {
		return h, nil
	}
	if h.CreateRejected {
		h.State = "draining"
		return h, nil
	}
	row, absent, err := d.resource(ctx, h.ResourceID, "2025-04-01", h.VMRequest)
	if err != nil || absent {
		return h, e2bbe.ErrUnknown
	}
	props, _ := row["properties"].(map[string]any)
	vmID, _ := props["vmId"].(string)
	if !azureClientIDPattern.MatchString(strings.ToLower(vmID)) || h.VMID != "" && !strings.EqualFold(vmID, h.VMID) {
		return h, e2bbe.ErrConflict
	}
	h.VMID = strings.ToLower(vmID)
	h.InstanceID = filepath.Base(h.ResourceID)
	h.State = "booting"
	storage, _ := props["storageProfile"].(map[string]any)
	osDisk, _ := storage["osDisk"].(map[string]any)
	managed, _ := osDisk["managedDisk"].(map[string]any)
	diskID, _ := managed["id"].(string)
	expectedDisk := strings.ToLower(d.selection.HostResourceGroupID) + "/providers/microsoft.compute/disks/" + filepath.Base(h.ResourceID) + "-os"
	if strings.ToLower(diskID) != expectedDisk || osDisk["deleteOption"] != "Delete" || h.DiskID != "" && h.DiskID != expectedDisk {
		return h, e2bbe.ErrUnknown
	}
	var disk map[string]any
	missing, e := d.call(ctx, "GET", expectedDisk, "2023-10-02", nil, &disk, nil)
	if e != nil || missing {
		return h, e2bbe.ErrUnknown
	}
	diskProps, _ := disk["properties"].(map[string]any)
	unique, _ := diskProps["uniqueId"].(string)
	if unique == "" || len(unique) > 128 || !strings.EqualFold(fmt.Sprint(disk["managedBy"]), h.ResourceID) || h.DiskUID != "" && h.DiskUID != unique {
		return h, e2bbe.ErrConflict
	}
	h.DiskID = expectedDisk
	h.DiskUID = unique
	// Persist the original immutable VM/NIC identities before any template effect.
	if h.NodeID == "" {
		config, close, e := d.credentials(ctx)
		if e != nil {
			return h, e
		}
		defer close()
		config.Deployment = d.base.Deployment
		config.Deployment.AzureVMResourceID = h.ResourceID
		config.Deployment.AzureNICResourceID = h.NICResourceID
		config.Deployment.AzureVMID = h.VMID
		config.Deployment.AzurePrivateIP = h.PrivateIP
		node, e := e2bbe.DiscoverAzureHost(ctx, config)
		if e != nil {
			return h, nil
		}
		h.NodeID = node
	}
	return h, nil
}
func (d *azureElasticDriver) credentials(ctx context.Context) (e2bbe.Config, func(), error) {
	if d.base.AzureSecrets == nil {
		return e2bbe.Config{}, nil, e2bbe.ErrUnsupported
	}
	source, err := newAzureSecretSource(*d.base.AzureSecrets)
	if err != nil {
		return e2bbe.Config{}, nil, err
	}
	files, close, err := materializeAzureSecrets(ctx, *d.base.AzureSecrets, source)
	if err != nil {
		return e2bbe.Config{}, nil, err
	}
	fail := func() (e2bbe.Config, func(), error) { close(); return e2bbe.Config{}, nil, e2bbe.ErrUnsupported }
	ca, err := readPrivateE2BFile(d.base.RootCAFile, 64<<10)
	if err != nil {
		return fail()
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return fail()
	}
	certPEM, e1 := readPrivateE2BFile(files.ClientCert, 64<<10)
	key, e2 := readPrivateE2BFile(files.ClientKey, 64<<10)
	certificate, e3 := tls.X509KeyPair(certPEM, key)
	apiKey, e4 := readPrivateE2BFile(files.APIKey, 4096)
	admin, e5 := readPrivateE2BFile(files.AdminKey, 4096)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
		return fail()
	}
	return e2bbe.Config{Endpoint: d.base.Endpoint, APIKey: strings.TrimSpace(string(apiKey)), AdminAPIKey: strings.TrimSpace(string(admin)), RootCAs: roots, ClientCertificate: &certificate, GatewayReceiptDigest: d.base.AzureGatewayReceiptDigest}, close, nil
}
func (d *azureElasticDriver) hostDirectory(id string) (string, error) {
	if len(id) != 37 || !strings.HasPrefix(id, "host-") {
		return "", e2bbe.ErrUnsupported
	}
	if _, err := hex.DecodeString(id[5:]); err != nil {
		return "", err
	}
	base, err := os.OpenRoot(d.base.ControllerStateDir)
	if err != nil {
		return "", err
	}
	defer base.Close()
	for _, relative := range []string{"elastic-hosts", "elastic-hosts/" + id, "elastic-hosts/" + id + "/controller", "elastic-hosts/" + id + "/artifacts", "elastic-hosts/" + id + "/staging"} {
		if err = base.Mkdir(relative, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		info, e := base.Lstat(relative)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return "", e2bbe.ErrUnsupported
		}
	}
	return filepath.Join(d.base.ControllerStateDir, "elastic-hosts", id), nil
}
func (d *azureElasticDriver) Publish(ctx context.Context, h api.AzureElasticHost, authorize func() error) (api.AzureElasticHost, error) {
	if h.NodeID == "" || h.VMID == "" || authorize == nil || authorize() != nil {
		return h, management.ErrExecutionAuthority
	}
	nic, ip, err := d.address(ctx, h)
	if err != nil || nic != h.NICUID || ip != h.PrivateIP {
		return h, e2bbe.ErrConflict
	}
	config, close, err := d.credentials(ctx)
	if err != nil {
		return h, err
	}
	defer close()
	dir, err := d.hostDirectory(h.ID)
	if err != nil {
		return h, err
	}
	creation := e2bbe.TemplateCreation{Endpoint: config.Endpoint, APIKey: config.APIKey, RootCAs: config.RootCAs, ClientCertificate: config.ClientCertificate, PrivateIP: h.PrivateIP, BeforeMutation: func(c context.Context) error { return d.authorizeTemplate(c, h, authorize) }, Name: "luci-elastic-" + h.ID[5:], CPUCount: d.base.Template.CPUMillis / 1000, MemoryMiB: d.base.Template.MemoryMiB, JournalPath: filepath.Join(dir, "template-create.json")}
	created, err := creation.ReadTemplateCreation()
	if err != nil {
		if _, statErr := os.Lstat(creation.JournalPath); errors.Is(statErr, os.ErrNotExist) {
			if authorize() != nil {
				return h, management.ErrExecutionAuthority
			}
			created, err = creation.CreateTemplate(ctx)
		} else {
			created, err = creation.ReconcileTemplateCreation(ctx)
		}
		if err != nil {
			return h, err
		}
	}
	publication := e2bbe.TemplatePublication{Endpoint: config.Endpoint, APIKey: config.APIKey, RootCAs: config.RootCAs, ClientCertificate: config.ClientCertificate, PrivateIP: h.PrivateIP, BeforeMutation: func(c context.Context) error { return d.authorizeTemplate(c, h, authorize) }, TemplateID: created.TemplateID, BuildID: created.BuildID, FromImage: d.base.TemplateFromImage, CreateJournalPath: creation.JournalPath, JournalPath: filepath.Join(dir, "template-build.json")}
	if _, err = publication.ReadTemplateBuildIntent(); err != nil {
		if _, statErr := os.Lstat(publication.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
			return h, e2bbe.ErrUnknown
		}
		if authorize() != nil {
			return h, management.ErrExecutionAuthority
		}
		if _, err = publication.StartTemplateBuild(ctx); err != nil {
			return h, err
		}
	}
	status, err := publication.InspectTemplateBuild(ctx)
	if err != nil || status != "ready" {
		return h, e2bbe.ErrUnknown
	}
	h.TemplateID = created.TemplateID
	h.BuildID = created.BuildID
	return h, nil
}
func (d *azureElasticDriver) Open(ctx context.Context, h api.AzureElasticHost, guard func(context.Context, e2bbe.Request, int64) error) (api.CloudTaskHost, func(), error) {
	dir, err := d.hostDirectory(h.ID)
	if err != nil {
		return api.CloudTaskHost{}, nil, err
	}
	if strings.Count(d.base.TemplateFromImage, "@sha256:") != 1 {
		return api.CloudTaskHost{}, nil, e2bbe.ErrUnsupported
	}
	selected := d.base
	selected.HostNodeID = h.NodeID
	selected.Deployment.AzureVMResourceID = h.ResourceID
	selected.Deployment.AzureNICResourceID = h.NICResourceID
	selected.Deployment.AzureVMID = h.VMID
	selected.Deployment.AzurePrivateIP = h.PrivateIP
	selected.Deployment.ManagementReceiptDigest = elasticHash([]string{h.VMID, h.NICID, h.PrivateIP, h.NodeID, selected.AzureGatewayReceiptDigest})
	selected.ControllerStateDir = filepath.Join(dir, "controller")
	selected.StagingDir = filepath.Join(dir, "staging")
	selected.Store.Directory = filepath.Join(dir, "artifacts")
	selected.TemplateCreateJournalPath = filepath.Join(dir, "template-create.json")
	selected.TemplateBuildJournalPath = filepath.Join(dir, "template-build.json")
	selected.Template.ID = h.TemplateID
	selected.Template.BuildID = h.BuildID
	intent := e2bbe.TemplateBuildIntent{RuntimeRevision: e2bbe.RuntimeRevision, TemplateID: h.TemplateID, BuildID: h.BuildID, FromImage: selected.TemplateFromImage, ImageDigest: strings.Split(selected.TemplateFromImage, "@sha256:")[1]}
	selected.Template.BuildDigest = e2bbe.TemplateImageBinding(intent)
	raw, err := json.Marshal(selected)
	if err != nil {
		return api.CloudTaskHost{}, nil, err
	}
	path := filepath.Join(dir, "operator.json")
	info, statErr := os.Lstat(path)
	if statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return api.CloudTaskHost{}, nil, e2bbe.ErrConflict
		}
		old, e := readPrivateE2BFile(path, 16<<10)
		if e != nil {
			return api.CloudTaskHost{}, nil, e
		}
		if h.ConfigDigest != "" {
			var retained e2bOperatorConfig
			if json.Unmarshal(old, &retained) != nil || elasticHash(retained) != h.ConfigDigest || retained.Deployment.AzureVMResourceID != h.ResourceID || retained.Deployment.AzureNICResourceID != h.NICResourceID || retained.Deployment.AzureVMID != h.VMID || retained.Deployment.AzurePrivateIP != h.PrivateIP || retained.HostNodeID != h.NodeID || retained.Store.OwnerID != d.base.Store.OwnerID || retained.Store.AzureDataDiskID != d.base.Store.AzureDataDiskID || retained.Store.RuntimeScope != d.base.Store.RuntimeScope {
				return api.CloudTaskHost{}, nil, e2bbe.ErrConflict
			}
			selected = retained
			raw = old
		} else if !bytes.Equal(raw, old) {
			return api.CloudTaskHost{}, nil, e2bbe.ErrConflict
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return api.CloudTaskHost{}, nil, statErr
	} else {
		if h.ConfigDigest != "" {
			return api.CloudTaskHost{}, nil, e2bbe.ErrUnknown
		}
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return api.CloudTaskHost{}, nil, e
		}
		n, we := f.Write(raw)
		se := f.Sync()
		ce := f.Close()
		if we != nil || se != nil || ce != nil || n != len(raw) {
			return api.CloudTaskHost{}, nil, e2bbe.ErrUnknown
		}
		parent, e := os.Open(dir)
		if e != nil {
			return api.CloudTaskHost{}, nil, e
		}
		se = parent.Sync()
		ce = parent.Close()
		if se != nil || ce != nil {
			return api.CloudTaskHost{}, nil, e2bbe.ErrUnknown
		}
	}
	cfg := d.worker
	cfg.E2BConfigPath = path
	cleanupOnly := h.Retired || h.DeleteAttempted || h.State == "draining" || d.selection.Disabled || h.OperatorDigest != d.selection.ProfileDigest || time.Now().After(h.ExpiresAt)
	runtime, cleanup, err := openE2BTaskRuntimeSingleMode(ctx, cfg, d.projects, d.authorize, guard, cleanupOnly)
	if err != nil {
		return api.CloudTaskHost{}, nil, err
	}
	return api.CloudTaskHost{ID: h.ID, Name: "Automatic Azure host " + h.InstanceID, ConfigDigest: elasticHash(selected), HostKey: elasticHash([]string{h.VMID, h.NodeID}), Runtime: runtime, Draining: cleanupOnly}, cleanup, nil
}
func (d *azureElasticDriver) Idle(ctx context.Context, h api.AzureElasticHost) (bool, error) {
	d = d.originalGroup(h)
	if h.VMID == "" {
		return !h.ScaleAttempted || h.CreateRejected, nil
	}
	if _, absent, err := d.resource(ctx, h.ResourceID, "2025-04-01", h.VMRequest); err != nil || absent {
		return false, e2bbe.ErrUnknown
	}
	if h.ConfigDigest == "" {
		return true, nil
	}
	config, close, err := d.credentials(ctx)
	if err != nil {
		return false, err
	}
	defer close()
	config.Deployment = d.base.Deployment
	config.Deployment.AzureVMResourceID = h.ResourceID
	config.Deployment.AzureNICResourceID = h.NICResourceID
	config.Deployment.AzureVMID = h.VMID
	config.Deployment.AzurePrivateIP = h.PrivateIP
	node, err := e2bbe.DiscoverAzureHost(ctx, config)
	return err == nil && (h.NodeID == "" || h.NodeID == node), err
}
func (d *azureElasticDriver) Delete(ctx context.Context, h api.AzureElasticHost) error {
	d = d.originalGroup(h)
	if d.verifyController(ctx) != nil || !h.DeleteAttempted || h.State != "delete-unknown" || h.NICUID == "" {
		return e2bbe.ErrConflict
	}
	if h.VMID != "" {
		row, absent, err := d.resource(ctx, h.ResourceID, "2025-04-01", h.VMRequest)
		if err != nil {
			return err
		}
		if !absent {
			props, _ := row["properties"].(map[string]any)
			if !strings.EqualFold(fmt.Sprint(props["vmId"]), h.VMID) {
				return e2bbe.ErrConflict
			}
			_, err = d.call(ctx, "DELETE", h.ResourceID, "2025-04-01", nil, nil, nil)
			return err
		}
	}
	// NIC cleanup waits for positive VM absence and the original NIC GUID. It
	// never deletes a foreign VM or an attached/replaced interface.
	return nil
}
func (d *azureElasticDriver) DeleteNIC(ctx context.Context, h api.AzureElasticHost) error {
	d = d.originalGroup(h)
	if d.verifyController(ctx) != nil {
		return management.ErrExecutionAuthority
	}
	row, absent, err := d.resource(ctx, h.NICResourceID, "2025-05-01", h.NetworkRequest)
	if err != nil || absent {
		return err
	}
	props, _ := row["properties"].(map[string]any)
	if !strings.EqualFold(fmt.Sprint(props["resourceGuid"]), h.NICUID) || props["virtualMachine"] != nil {
		return e2bbe.ErrUnknown
	}
	_, err = d.call(ctx, "DELETE", h.NICResourceID, "2025-05-01", nil, nil, nil)
	return err
}
func (d *azureElasticDriver) Absent(ctx context.Context, h api.AzureElasticHost) (bool, bool, error) {
	d = d.originalGroup(h)
	if !h.DeleteAttempted || h.NICUID == "" {
		return false, false, e2bbe.ErrConflict
	}
	if h.VMID != "" {
		row, missing, err := d.resource(ctx, h.ResourceID, "2025-04-01", h.VMRequest)
		if err != nil {
			return false, false, err
		}
		if !missing {
			props, _ := row["properties"].(map[string]any)
			if !strings.EqualFold(fmt.Sprint(props["vmId"]), h.VMID) {
				return false, false, e2bbe.ErrConflict
			}
			return false, false, nil
		}
	}
	_, nicMissing, err := d.resource(ctx, h.NICResourceID, "2025-05-01", h.NetworkRequest)
	if err != nil {
		return true, false, err
	}
	if h.DiskID != "" {
		var disk map[string]any
		missing, e := d.call(ctx, "GET", h.DiskID, "2023-10-02", nil, &disk, nil)
		if e != nil {
			return true, false, e
		}
		if !missing {
			props, _ := disk["properties"].(map[string]any)
			if fmt.Sprint(props["uniqueId"]) != h.DiskUID {
				return true, false, e2bbe.ErrConflict
			}
			return true, false, nil
		}
	}
	return true, nicMissing, nil
}

func elasticOperationURL(headers http.Header) string {
	for _, key := range []string{"Azure-AsyncOperation", "Location"} {
		raw := headers.Get(key)
		u, e := url.Parse(raw)
		if e == nil && len(raw) <= 2048 && u.Scheme == "https" && u.Host == "management.azure.com" && u.User == nil && u.Fragment == "" && strings.HasPrefix(u.Path, "/subscriptions/") {
			return raw
		}
	}
	return ""
}
func elasticRequestID(headers http.Header) string {
	value := headers.Get("x-ms-request-id")
	if len(value) > 128 || strings.ContainsAny(value, "\r\n\x00") {
		return ""
	}
	return value
}

func (d *azureElasticDriver) verifyController(ctx context.Context) error {
	if d.base.AzureMount == nil || d.base.AzureSecrets == nil {
		return management.ErrExecutionAuthority
	}
	if err := verifyAzureDurableMount(*d.base.AzureMount, d.base, d.worker); err != nil {
		return err
	}
	fence, err := newAzureFenceReader(d.base.AzureSecrets.ClientID)
	if err != nil {
		return err
	}
	return verifyAzureFence(ctx, d.base, fence, false)
}
func (d *azureElasticDriver) authorizeTemplate(ctx context.Context, h api.AzureElasticHost, authorize func() error) error {
	if d.verifyController(ctx) != nil || authorize == nil {
		return management.ErrExecutionAuthority
	}
	row, absent, err := d.resource(ctx, h.ResourceID, "2025-04-01", h.VMRequest)
	if err != nil || absent {
		return e2bbe.ErrUnknown
	}
	props, _ := row["properties"].(map[string]any)
	if !strings.EqualFold(fmt.Sprint(props["vmId"]), h.VMID) {
		return e2bbe.ErrConflict
	}
	guid, ip, err := d.address(ctx, h)
	if err != nil || guid != h.NICUID || ip != h.PrivateIP {
		return e2bbe.ErrConflict
	}
	return authorize()
}

func (d *azureElasticDriver) originalGroup(h api.AzureElasticHost) *azureElasticDriver {
	copy := *d
	prefix := "/subscriptions/" + d.base.Deployment.AzureSubscriptionID + "/resourcegroups/"
	parts := strings.Split(strings.ToLower(h.ResourceID), "/providers/microsoft.compute/virtualmachines/")
	if len(parts) == 2 && strings.HasPrefix(parts[0], prefix) && !strings.Contains(strings.TrimPrefix(parts[0], prefix), "/") {
		copy.selection.HostResourceGroupID = parts[0]
	}
	return &copy
}

// Azure may omit write-only customData in GET. The immutable request retains
// its exact bytes; original marker/spec/VMID are the readback authority.
func elasticReadbackProjection(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if properties, ok := object["properties"].(map[string]any); ok {
		if osProfile, ok := properties["osProfile"].(map[string]any); ok {
			delete(osProfile, "customData")
		}
	}
	return object
}
