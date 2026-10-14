package product

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Azure Develop setup is deliberately distinct from /v1/setup/cloud/plan,
// which provisions an ordinary Docker application/deployment VM. This plan
// never contacts a provider, executes a command, or changes selected mode.
type azureDevelopPlanRequest struct {
	Name             string  `json:"name"`
	Region           string  `json:"region"`
	MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd"`
	MaxConcurrentVMs int     `json:"maxConcurrentVMs"`
}

type azureSetupBundleManifest struct {
	Version      int    `json:"version"`
	LuciRevision string `json:"luciRevision"`
	Files        map[string]struct {
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	} `json:"files"`
}

var azureSetupSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)
var azureSetupGit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var azureSetupRequired = []string{
	"LICENSE", "azure-setup/installer.py", "azure-setup/credentials.py", "azure-setup/guided.py", "azure-setup/operator_setup.py", "azure-setup/README.md",
	"azure-embed-host/package.py", "azure-embed-host/service.py", "azure-embed-host/deliver.py",
	"azure-controller-disk/package.py", "azure-controller-disk/mount.py", "azure-controller-disk/recover.py",
	"terraform/.terraform.lock.hcl", "terraform/compute.tf", "terraform/fence.tf", "terraform/identity.tf",
	"terraform/network.tf", "terraform/outputs.tf", "terraform/variables.tf", "terraform/versions.tf",
}

func readAzureSetupBundle(runtimeDir, projectRoot string) ([]byte, string, string, error) {
	if !filepath.IsAbs(runtimeDir) {
		return nil, "", "", errors.New("Azure setup bundle is not installed")
	}
	root, err := hostpath.ResolveOutside(projectRoot, runtimeDir)
	if err != nil {
		return nil, "", "", errors.New("Azure setup bundle is not outside projects")
	}
	archive := filepath.Join(root, "azure-setup.tar")
	identity := filepath.Join(root, "azure-setup.sha256")
	for index, path := range []string{archive, identity} {
		info, statErr := os.Lstat(path)
		limit := int64(4 << 20)
		if index == 1 {
			limit = 80
		}
		if statErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
			return nil, "", "", errors.New("Azure setup bundle is incomplete")
		}
	}
	readBounded := func(name string, limit int64) ([]byte, error) {
		original, statErr := os.Lstat(name)
		if statErr != nil || !original.Mode().IsRegular() || original.Size() < 1 || original.Size() > limit {
			return nil, errors.New("Azure setup bundle member is not an ordinary bounded file")
		}
		file, openErr := os.Open(name)
		if openErr != nil {
			return nil, openErr
		}
		defer file.Close()
		opened, statErr := file.Stat()
		if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(original, opened) || opened.Size() < 1 || opened.Size() > limit {
			return nil, errors.New("Azure setup bundle member changed while opening")
		}
		contents, readErr := io.ReadAll(io.LimitReader(file, limit+1))
		if readErr != nil || int64(len(contents)) > limit {
			return nil, errors.New("Azure setup bundle member exceeds its size limit")
		}
		return contents, nil
	}
	data, err := readBounded(archive, 4<<20)
	if err != nil || len(data) == 0 || len(data) > 4<<20 {
		return nil, "", "", errors.New("Azure setup bundle is unavailable")
	}
	expected, err := readBounded(identity, 80)
	if err != nil || len(expected) > 80 || !azureSetupSHA.MatchString(strings.TrimSpace(string(expected))) {
		return nil, "", "", errors.New("Azure setup bundle identity is invalid")
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if actual != strings.TrimSpace(string(expected)) {
		return nil, "", "", errors.New("Azure setup bundle digest changed")
	}
	reader := tar.NewReader(bytes.NewReader(data))
	files := make(map[string][]byte)
	for {
		entry, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil || entry == nil || entry.Typeflag != tar.TypeReg || entry.Size < 1 || entry.Size > 2<<20 ||
			path.IsAbs(entry.Name) || path.Clean(entry.Name) != entry.Name || strings.Contains(entry.Name, `\`) ||
			strings.HasPrefix(entry.Name, "../") || entry.Name == ".." || len(files) > 30 {
			return nil, "", "", errors.New("Azure setup bundle entry is unsafe")
		}
		if _, exists := files[entry.Name]; exists {
			return nil, "", "", errors.New("Azure setup bundle entry is repeated")
		}
		part, readErr := io.ReadAll(io.LimitReader(reader, entry.Size+1))
		if readErr != nil || int64(len(part)) != entry.Size {
			return nil, "", "", errors.New("Azure setup bundle entry is incomplete")
		}
		files[entry.Name] = part
	}
	manifestBytes, exists := files["manifest.json"]
	if !exists {
		return nil, "", "", errors.New("Azure setup bundle manifest is missing")
	}
	var manifest azureSetupBundleManifest
	if json.Unmarshal(manifestBytes, &manifest) != nil || manifest.Version != 1 || !azureSetupGit.MatchString(manifest.LuciRevision) || len(manifest.Files) != len(files)-1 {
		return nil, "", "", errors.New("Azure setup bundle manifest is invalid")
	}
	for name, part := range files {
		if name == "manifest.json" {
			continue
		}
		binding, okay := manifest.Files[name]
		measured := sha256.Sum256(part)
		if !okay || int64(len(part)) != binding.Size || hex.EncodeToString(measured[:]) != binding.SHA256 {
			return nil, "", "", errors.New("Azure setup bundle member changed")
		}
	}
	if len(manifest.Files) != len(azureSetupRequired) {
		return nil, "", "", errors.New("Azure setup bundle inventory is incomplete")
	}
	for _, name := range azureSetupRequired {
		if _, okay := files[name]; !okay {
			return nil, "", "", errors.New("Azure setup bundle member is missing")
		}
	}
	return data, actual, manifest.LuciRevision, nil
}

func (s *Server) getAzureDevelopBundle(w http.ResponseWriter, r *http.Request) {
	data, digest, _, err := readAzureSetupBundle(s.runtimeDir, s.projectRoot())
	if err != nil {
		problem(w, 503, "azure_setup_unavailable", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", `attachment; filename="luci-azure-setup.tar"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Luci-Archive-SHA256", digest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) planAzureDevelop(w http.ResponseWriter, r *http.Request) {
	var requested azureDevelopPlanRequest
	if !readJSON(w, r, &requested) {
		return
	}
	if !slugPattern.MatchString(requested.Name) || !regionPattern.MatchString(requested.Region) ||
		requested.MonthlyBudgetUSD <= 0 || requested.MonthlyBudgetUSD > 100000 ||
		requested.MaxConcurrentVMs < 1 || requested.MaxConcurrentVMs > 1024 {
		problem(w, 400, "invalid_azure_develop_plan", "Choose a valid name, Azure region, positive reviewed budget and configured concurrency.")
		return
	}
	if _, policy, managed := management.ActorFromContext(r.Context()); managed &&
		(!policy.AllowCloudProvisioning || requested.MonthlyBudgetUSD > policy.MonthlyBudgetUSD) {
		problem(w, 403, "budget_policy", "Current account policy does not allow this Azure Develop plan or reviewed budget.")
		return
	}
	archive, digest, revision, err := readAzureSetupBundle(s.runtimeDir, s.projectRoot())
	if err != nil || len(archive) == 0 {
		problem(w, 503, "azure_setup_unavailable", "This Luci package does not include a verified Azure Develop installer.")
		return
	}
	writeJSON(w, 200, map[string]any{
		"version": 1, "status": "needs_prerequisites", "purpose": "develop-microvm", "provider": "azure",
		"setupHostRequirement": "Linux or WSL2 with Python 3, Go, Terraform, Azure CLI and an isolated Ubuntu image build environment; the installed Windows app can download this exact bundle but does not run Linux image preparation in Windows PowerShell.",
		"sourceCommit":         revision, "bundleSHA256": digest, "bundleURL": "/v1/setup/cloud/azure/bundle",
		"sourceRepository":      "https://github.com/lcfrai/luci.git",
		"advancedAccountInputs": []string{"Azure subscription", "existing resource group and durable state Blob", "unused private network", "KVM-capable VM sizes and zone", "SSH public key", "immutable gallery images", "exact private Key Vault references"},
		"advancedSelectionTemplate": map[string]any{
			"version": 1, "mode": "byo-azure-develop", "name": requested.Name, "region": requested.Region,
			"monthlyBudgetUsd": requested.MonthlyBudgetUSD, "maxConcurrentVMs": requested.MaxConcurrentVMs,
			"runtimePackage":      map[string]string{"path": "", "sha256": ""},
			"controllerPackage":   map[string]string{"path": "", "sha256": ""},
			"controllerHealthURL": "http://127.0.0.1:8730/v1/healthz", "nextGeneration": "",
			"stateBackend":       map[string]string{"subscriptionId": "", "resourceGroupName": "", "storageAccountName": "", "containerName": "", "key": ""},
			"terraformVariables": map[string]any{"name": requested.Name, "location": requested.Region, "subscription_id": ""},
		},
		"missingInputs": []string{"existing_durable_Azure_Blob_Terraform_backend", "exact_runtime_and_controller_packages", "immutable_gallery_image_versions", "private_network_and_ssh_settings", "key_vault_exact_version_references", "controller_operator_template", "reviewed_Azure_quote_and_account_capacity"},
		"guidedStages": []map[string]string{
			{"id": "select", "action": "Choose account, private location, capacity and reviewed budget; the installer generates scope and generation IDs."},
			{"id": "credentials", "action": "Generate private TLS and API material locally; explicitly publish and read back exact Key Vault versions, retaining unknown writes."},
			{"id": "images", "action": "Prepare pinned host and controller image roots, then review immutable gallery publication and signed guest template."},
			{"id": "connect", "action": "Apply only after reviewed cost authority, reconcile exact private resources, and verify protected controller connectivity."},
			{"id": "retire", "action": "Settle each owned VM and retained reservation before infrastructure retirement; provider invoice remains a separate later record."},
		},
		"commands": []string{
			"python3 -c 'import hashlib,sys; assert hashlib.sha256(open(sys.argv[1], \"rb\").read()).hexdigest() == sys.argv[2]' luci-azure-setup.tar " + digest,
			"mkdir -m 700 luci-azure-setup", "tar -xf luci-azure-setup.tar -C luci-azure-setup",
			"mkdir -m 700 luci-azure-private",
			"python3 luci-azure-setup/azure-setup/installer.py init-selection --bundle luci-azure-setup --selection luci-azure-private/selection.json --name " + requested.Name + " --region " + requested.Region + " --monthly-budget-usd " + strconv.FormatFloat(requested.MonthlyBudgetUSD, 'f', -1, 64) + " --max-concurrent-vms " + strconv.Itoa(requested.MaxConcurrentVMs),
			"python3 luci-azure-setup/azure-setup/installer.py configure-guided --bundle luci-azure-setup --selection luci-azure-private/selection.json",
			"python3 luci-azure-setup/azure-setup/installer.py plan --bundle luci-azure-setup --selection luci-azure-private/selection.json",
		},
		"note": "The existing deployment-target cloud setup creates an ordinary VM. This Azure plan prepares Develop microVM infrastructure and does not connect or provision anything by itself.",
	})
}
