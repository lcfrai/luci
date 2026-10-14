package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

type azureSetupToolInput struct {
	Version                int    `json:"version"`
	SourceCommit           string `json:"sourceCommit"`
	Scope                  string `json:"scope"`
	Generation             string `json:"generation"`
	ControllerResourceID   string `json:"controllerResourceID"`
	RuntimeScaleSetID      string `json:"runtimeScaleSetID"`
	DataDiskID             string `json:"dataDiskID"`
	ControllerClientID     string `json:"controllerClientID"`
	FenceBlobURL           string `json:"fenceBlobURL"`
	FilesystemUUID         string `json:"filesystemUUID"`
	MountPath              string `json:"mountPath"`
	OperatorTemplatePath   string `json:"operatorTemplatePath"`
	OperatorTemplateSHA256 string `json:"operatorTemplateSHA256"`
	TemplateName           string `json:"templateName"`
	CPUCount               int64  `json:"cpuCount"`
	MemoryMiB              int64  `json:"memoryMiB"`
	FromImage              string `json:"fromImage"`
}

type azureSetupRuntimeInstance struct {
	ID         string `json:"id"`
	InstanceID string `json:"instanceId"`
	Properties struct {
		ProvisioningState string `json:"provisioningState"`
	} `json:"properties"`
}

type azureSetupNodeReceipt struct {
	ID                   string `json:"id"`
	Status               string `json:"status"`
	SandboxCount         *int64 `json:"sandboxCount"`
	SandboxStartingCount *int64 `json:"sandboxStartingCount"`
	MaxSandboxes         *int64 `json:"maxSandboxes"`
}

func readAzureSetupToolSelection(path string) (azureSetupToolInput, e2bOperatorConfig, error) {
	var input azureSetupToolInput
	var selected e2bOperatorConfig
	data, err := readPrivateE2BFile(path, 16<<10)
	if err != nil {
		return input, selected, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF || input.Version != 1 ||
		!azureGenerationPattern.MatchString(input.Scope) || !azureGenerationPattern.MatchString(input.Generation) ||
		!azureControllerInstancePattern.MatchString(input.ControllerResourceID) || !azureClientIDPattern.MatchString(input.ControllerClientID) ||
		!azureUUIDPattern.MatchString(input.FilesystemUUID) || !azureDigestPattern.MatchString(input.OperatorTemplateSHA256) ||
		input.CPUCount < 1 || input.CPUCount > 16 || input.MemoryMiB < 128 || input.MemoryMiB > 32768 ||
		!filepath.IsAbs(input.MountPath) || filepath.Dir(input.OperatorTemplatePath) != "/etc/luci" || filepath.Clean(input.OperatorTemplatePath) != input.OperatorTemplatePath {
		return input, selected, errors.New("invalid private Azure controller setup selection")
	}
	info, ok := debug.ReadBuildInfo()
	matching := false
	if ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value == input.SourceCommit {
				matching = true
			}
		}
	}
	if !matching {
		return input, selected, errors.New("controller binary differs from the selected source")
	}
	data, err = readPrivateE2BFile(input.OperatorTemplatePath, 16<<10)
	sum := sha256.Sum256(data)
	if err != nil || hex.EncodeToString(sum[:]) != input.OperatorTemplateSHA256 {
		return input, selected, errors.New("image-pinned operator source changed")
	}
	var draft map[string]json.RawMessage
	if json.Unmarshal(data, &draft) != nil {
		return input, selected, errors.New("invalid deferred operator")
	}
	delete(draft, "setupDeferred")
	data, _ = json.Marshal(draft)
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&selected) != nil || selected.SourceCommit != input.SourceCommit || selected.Deployment.Provider != "azure" ||
		selected.Deployment.Scope != input.Scope || selected.Deployment.HostGeneration != input.Generation || selected.AzureSecrets == nil ||
		selected.TemplateFromImage != input.FromImage || selected.Template.MemoryMiB != input.MemoryMiB || selected.Template.CPUMillis != input.CPUCount*1000 {
		return input, selected, errors.New("deferred operator differs from selected source, generation or guest")
	}
	selected.AzureSecrets.ClientID = input.ControllerClientID
	selected.Deployment.AzureScaleSetID = input.RuntimeScaleSetID
	selected.Deployment.AzureDataDiskID = input.DataDiskID
	selected.Store.AzureDataDiskID = input.DataDiskID
	disk, err := readPrivateE2BFile(filepath.Join(input.MountPath, "luci", "disk-receipt.json"), 4096)
	if err != nil {
		return input, selected, err
	}
	diskSum := sha256.Sum256(disk)
	selected.Store.SeparationReceiptDigest = hex.EncodeToString(diskSum[:])
	generationPath := filepath.Join(input.MountPath, "luci", "generation.json")
	generation, err := readPrivateE2BFile(generationPath, 4096)
	if err != nil {
		return input, selected, err
	}
	generationSum := sha256.Sum256(generation)
	selected.AzureMount = &azureDurableMount{MountPath: input.MountPath, FilesystemUUID: input.FilesystemUUID,
		ReceiptPath: filepath.Join(input.MountPath, "luci", "disk-receipt.json"), Generation: input.Generation,
		ControllerInstanceID: input.ControllerResourceID, GenerationReceiptPath: generationPath,
		GenerationReceiptDigest: hex.EncodeToString(generationSum[:]), FenceBlobURL: input.FenceBlobURL}
	selected.AzureGatewayReceiptDigest = e2bbe.GatewayReceiptDigest(selected.Deployment.AzureGatewayHost, input.Scope, input.Generation)
	cfg := Config{StateDir: filepath.Join(input.MountPath, "luci", "control"), ProjectsDir: filepath.Join(input.MountPath, "luci", "projects"), ExecutionStateDir: filepath.Join(input.MountPath, "luci", "execution")}
	if err := verifyAzureDurableMount(*selected.AzureMount, selected, cfg); err != nil {
		return input, selected, err
	}
	return input, selected, nil
}

func azureSetupGatewayClient(host string, roots *x509.CertPool, certificate tls.Certificate) *http.Client {
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}, ResponseHeaderTimeout: 10 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		name, port, err := net.SplitHostPort(address)
		if err != nil || name != host || port != "443" {
			return nil, errors.New("private gateway destination changed")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, name)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("private gateway DNS unavailable")
		}
		for _, ip := range ips {
			if ip.IP.To4() == nil || !ip.IP.IsPrivate() {
				return nil, errors.New("gateway DNS is not RFC1918")
			}
		}
		var last error
		for _, ip := range ips {
			connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			last = err
		}
		return nil, last
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("private gateway redirect refused") }}
}

func azureSetupGatewayRead(ctx context.Context, client *http.Client, host, path, adminKey string) ([]byte, error) {
	request, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+path, nil)
	request.Header.Set("X-Admin-Token", adminKey)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || response.StatusCode != 200 || len(data) > 64<<10 {
		return nil, errors.New("private gateway readback incomplete")
	}
	return data, nil
}

func runAzureSetupCommand(action, path string, output io.Writer) error {
	if runtime.GOOS != "linux" || (action != "template-create" && action != "template-reconcile" && action != "template-start" && action != "template-status" && action != "observe") {
		return errors.New("unsupported protected controller setup action")
	}
	input, selected, err := readAzureSetupToolSelection(path)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	arm, err := newAzureARMReader(input.ControllerClientID)
	if err != nil {
		return err
	}
	var instances struct {
		Value    []azureSetupRuntimeInstance `json:"value"`
		NextLink string                      `json:"nextLink"`
	}
	if arm.Read(ctx, input.RuntimeScaleSetID+"/virtualMachines", "2023-09-01", &instances) != nil || len(instances.Value) != 1 || instances.NextLink != "" || instances.Value[0].Properties.ProvisioningState != "Succeeded" {
		return errors.New("initial runtime instance readback is ambiguous")
	}
	instance := instances.Value[0]
	selected.Deployment.AzureInstanceID = instance.InstanceID
	if !e2bbe.ValidAzureDeployment(selected.Deployment, selected.Endpoint) || !strings.EqualFold(instance.ID, input.RuntimeScaleSetID+"/virtualMachines/"+instance.InstanceID) || verifyAzureARMOwnership(ctx, selected, arm) != nil {
		return errors.New("controller/runtime ARM identity readback differs")
	}
	fence, err := newAzureFenceReader(input.ControllerClientID)
	if err != nil {
		return err
	}
	if err := verifyAzureFence(ctx, selected, fence, false); err != nil {
		return err
	}
	source, err := newAzureSecretSource(*selected.AzureSecrets)
	if err != nil {
		return err
	}
	files, cleanup, err := materializeAzureSecrets(ctx, *selected.AzureSecrets, source)
	if err != nil {
		return err
	}
	defer cleanup()
	apiKey, err := readPrivateE2BFile(files.APIKey, 4096)
	if err != nil {
		return err
	}
	adminKey, err := readPrivateE2BFile(files.AdminKey, 4096)
	if err != nil {
		return err
	}
	ca, err := readPrivateE2BFile(selected.RootCAFile, 1<<20)
	if err != nil {
		return err
	}
	roots, err := cloudProviderRoots(ca, true)
	if err != nil {
		return err
	}
	certData, err := readPrivateE2BFile(files.ClientCert, 1<<20)
	if err != nil {
		return err
	}
	keyData, err := readPrivateE2BFile(files.ClientKey, 1<<20)
	if err != nil {
		return err
	}
	certificate, err := tls.X509KeyPair(certData, keyData)
	if err != nil || len(certificate.Certificate) == 0 {
		return errors.New("private gateway client identity unavailable")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || !azureControllerCertificateMatches(leaf, input.Scope, input.Generation) {
		return errors.New("private client identity generation changed")
	}
	client := azureSetupGatewayClient(selected.Deployment.AzureGatewayHost, roots, certificate)
	defer client.CloseIdleConnections()
	readback, err := azureSetupGatewayRead(ctx, client, selected.Deployment.AzureGatewayHost, "/luci-gateway/receipt", strings.TrimSpace(string(adminKey)))
	if err != nil || string(readback) != selected.AzureGatewayReceiptDigest {
		return errors.New("private gateway generation differs")
	}
	directory := filepath.Join(input.MountPath, "luci", "template")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	creation := e2bbe.TemplateCreation{Endpoint: selected.Endpoint, APIKey: strings.TrimSpace(string(apiKey)), RootCAs: roots, ClientCertificate: &certificate,
		Name: input.TemplateName, CPUCount: input.CPUCount, MemoryMiB: input.MemoryMiB, JournalPath: filepath.Join(directory, "create.json")}
	if action == "template-create" {
		_, err = creation.CreateTemplate(ctx)
	}
	if action == "template-reconcile" {
		_, err = creation.ReconcileTemplateCreation(ctx)
	}
	if err != nil {
		return err
	}
	receipt := map[string]any{"version": 1, "sourceCommit": input.SourceCommit, "scope": input.Scope, "generation": input.Generation,
		"controllerResourceID": input.ControllerResourceID, "dataDiskID": input.DataDiskID,
		"runtimeInstance": instance, "gatewayReceiptDigest": selected.AzureGatewayReceiptDigest, "microVMExecutionVerified": false}
	created, createdErr := creation.ReadTemplateCreation()
	if createdErr == nil {
		publication := e2bbe.TemplatePublication{Endpoint: selected.Endpoint, APIKey: strings.TrimSpace(string(apiKey)), RootCAs: roots, ClientCertificate: &certificate,
			TemplateID: created.TemplateID, BuildID: created.BuildID, FromImage: input.FromImage, CreateJournalPath: creation.JournalPath, JournalPath: filepath.Join(directory, "build.json")}
		if action == "template-start" {
			_, err = publication.StartTemplateBuild(ctx)
			if err != nil {
				return err
			}
		}
		if action == "template-status" {
			status, err := publication.InspectTemplateBuild(ctx)
			if err != nil {
				return err
			}
			receipt["templateStatus"] = status
		}
		for name, file := range map[string]string{"createJournal": creation.JournalPath, "createResponse": creation.JournalPath + ".response", "buildJournal": publication.JournalPath} {
			data, err := readPrivateE2BFile(file, 4096)
			if err == nil {
				var value any
				if json.Unmarshal(data, &value) != nil {
					return errors.New("template journal readback invalid")
				}
				receipt[name] = value
			}
		}
	} else if action == "template-start" || action == "template-status" {
		return createdErr
	}
	nodes, err := azureSetupGatewayRead(ctx, client, selected.Deployment.AzureGatewayHost, "/nodes", strings.TrimSpace(string(adminKey)))
	if err != nil {
		return err
	}
	var nodeRows []azureSetupNodeReceipt
	if json.Unmarshal(nodes, &nodeRows) != nil || len(nodeRows) != 1 || nodeRows[0].Status != "ready" || nodeRows[0].ID == "" ||
		nodeRows[0].SandboxCount == nil || nodeRows[0].SandboxStartingCount == nil || nodeRows[0].MaxSandboxes == nil {
		return errors.New("initial protected runtime node is not ready")
	}
	receipt["nodes"] = nodeRows
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return err
	}
	receipt["bootID"] = strings.TrimSpace(string(boot))
	receipt["state"] = "controller_setup_readback"
	data, err := json.Marshal(receipt)
	if err != nil || len(data) > 32<<10 {
		return errors.New("controller setup receipt exceeds bound")
	}
	_, err = output.Write(append(data, '\n'))
	return err
}
