package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func azureControllerSAN(scope, generation string) string {
	return "spiffe://luci/controller/" + scope + "/" + generation
}

func azureControllerCertificateMatches(leaf *x509.Certificate, scope, generation string) bool {
	if leaf == nil || !azureGenerationPattern.MatchString(generation) || scope == "" {
		return false
	}
	want := azureControllerSAN(scope, generation)
	for _, uri := range leaf.URIs {
		if uri.String() == want {
			return true
		}
	}
	return false
}

// The disk's filesystem UUID is checked against the mounted block device.
// The ARM disk ID is an operator-attested identity bound to the private
// on-disk receipt and store metadata; only live Azure readback can prove the
// ARM attachment. No cloud Create is allowed without this local boundary.
type azureDurableMount struct {
	MountPath               string `json:"mountPath"`
	FilesystemUUID          string `json:"filesystemUuid"`
	ReceiptPath             string `json:"receiptPath"`
	Generation              string `json:"generation"`
	ControllerInstanceID    string `json:"controllerInstanceId"`
	GenerationReceiptPath   string `json:"generationReceiptPath"`
	GenerationReceiptDigest string `json:"generationReceiptDigest"`
}

type azureDiskReceipt struct {
	Version          int    `json:"version"`
	AzureDataDiskID  string `json:"azureDataDiskId"`
	FilesystemUUID   string `json:"filesystemUuid"`
	OwnerID          string `json:"ownerId"`
	Scope            string `json:"scope"`
	ControllerHostID string `json:"controllerHostId"`
	RuntimeHostID    string `json:"runtimeHostId"`
}

// Protected infrastructure writes this when it establishes a controller
// generation. A replacement needs external proof of the previous writer's
// fence and detach. The digest is an operator attestation until live Azure
// readback verifies that provider effect.
type azureGenerationReceipt struct {
	Version                    int    `json:"version"`
	Generation                 string `json:"generation"`
	ControllerInstanceID       string `json:"controllerInstanceId"`
	AzureDataDiskID            string `json:"azureDataDiskId"`
	PreviousGeneration         string `json:"previousGeneration,omitempty"`
	PreviousFenceReceiptDigest string `json:"previousFenceReceiptDigest,omitempty"`
}

var azureUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var azureSandboxIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)
var azureGenerationPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,127}$`)
var azureControllerInstancePattern = regexp.MustCompile(`^/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft.compute/virtualmachines/[a-z0-9_-]+$`)
var azureDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func underAzureMount(mount, path string) bool {
	if mount == "" || path == "" || !filepath.IsAbs(mount) || !filepath.IsAbs(path) ||
		filepath.Clean(mount) != mount || filepath.Clean(path) != path || mount == string(filepath.Separator) {
		return false
	}
	rel, err := filepath.Rel(mount, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Each selected state path is checked against the same mounted device. A
// missing leaf is allowed only when its existing parent is on that device;
// symlinked ancestors are refused, including a moved mount after startup.
func azurePathOnMount(mount, path string, device uint64) bool {
	if !underAzureMount(mount, path) {
		return false
	}
	current := path
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return false
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil || resolved != current {
				return false
			}
			return azurePathDevice(current) == device
		}
		if !errors.Is(err, os.ErrNotExist) || current == mount {
			return false
		}
		current = filepath.Dir(current)
	}
}

// The implementation is Linux-only; non-Linux builds retain the local/AWS
// routes but refuse Azure controller storage.
func verifyAzureDurableMount(m azureDurableMount, selected e2bOperatorConfig, cfg Config) error {
	return verifyAzureDurableMountWithProbe(m, selected, cfg, azureMountedDevice)
}

func verifyAzureDurableMountWithProbe(m azureDurableMount, selected e2bOperatorConfig, cfg Config, mountedDevice func(string, string) (uint64, error)) error {
	if !azureUUIDPattern.MatchString(m.FilesystemUUID) || !underAzureMount(m.MountPath, m.ReceiptPath) ||
		!underAzureMount(m.MountPath, m.GenerationReceiptPath) || m.ReceiptPath == m.GenerationReceiptPath ||
		!azureGenerationPattern.MatchString(m.Generation) || !azureControllerInstancePattern.MatchString(m.ControllerInstanceID) ||
		!azureDigestPattern.MatchString(m.GenerationReceiptDigest) ||
		selected.Store.AzureDataDiskID != selected.Deployment.AzureDataDiskID {
		return errors.New("Azure disk identity or receipt path is invalid")
	}
	device, err := mountedDevice(m.MountPath, m.FilesystemUUID)
	if err != nil {
		return err
	}
	paths := []string{m.ReceiptPath, m.GenerationReceiptPath, selected.Store.Directory, selected.ControllerStateDir, selected.StagingDir,
		cfg.StateDir, cfg.ProjectsDir, cfg.ExecutionStateDir,
		filepath.Dir(selected.TemplateCreateJournalPath), filepath.Dir(selected.TemplateBuildJournalPath)}
	for _, path := range paths {
		if !azurePathOnMount(m.MountPath, path, device) {
			return errors.New("Azure controller state or task artifacts are outside the selected mounted disk")
		}
	}
	data, err := readPrivateE2BFile(m.ReceiptPath, 4096)
	if err != nil {
		return errors.New("Azure mounted-disk identity receipt is unavailable")
	}
	actual := sha256.Sum256(data)
	if hex.EncodeToString(actual[:]) != selected.Store.SeparationReceiptDigest {
		return errors.New("Azure mounted-disk identity receipt changed")
	}
	var receipt azureDiskReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || receipt != (azureDiskReceipt{
		Version: 1, AzureDataDiskID: selected.Store.AzureDataDiskID, FilesystemUUID: m.FilesystemUUID,
		OwnerID: selected.Store.OwnerID, Scope: selected.Store.RuntimeScope,
		ControllerHostID: selected.Store.ControllerHostID, RuntimeHostID: selected.Store.RuntimeHostID,
	}) {
		return fmt.Errorf("Azure mounted-disk identity receipt does not match this controller")
	}
	generationData, err := readPrivateE2BFile(m.GenerationReceiptPath, 4096)
	if err != nil {
		return errors.New("Azure controller generation receipt is unavailable")
	}
	generationHash := sha256.Sum256(generationData)
	if hex.EncodeToString(generationHash[:]) != m.GenerationReceiptDigest {
		return errors.New("Azure controller generation changed or old writer resumed")
	}
	var generation azureGenerationReceipt
	decoder = json.NewDecoder(bytes.NewReader(generationData))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&generation) != nil || decoder.Decode(new(any)) != io.EOF ||
		generation.Version != 1 || generation.Generation != m.Generation ||
		generation.ControllerInstanceID != m.ControllerInstanceID || generation.AzureDataDiskID != selected.Store.AzureDataDiskID ||
		(generation.PreviousGeneration == "") != (generation.PreviousFenceReceiptDigest == "") ||
		generation.PreviousGeneration != "" && (!azureGenerationPattern.MatchString(generation.PreviousGeneration) ||
			generation.PreviousGeneration == generation.Generation || !azureDigestPattern.MatchString(generation.PreviousFenceReceiptDigest)) {
		return errors.New("Azure controller generation or predecessor fence is invalid")
	}
	return nil
}

// Run before opening product/session state. The same check runs again on
// cloud effects, since a disk detached after startup must halt new work.
func preflightAzureControllerMount(cfg Config) error {
	if cfg.E2BConfigPath == "" {
		return nil
	}
	data, err := readPrivateE2BFile(cfg.E2BConfigPath, 16<<10)
	if err != nil {
		return err
	}
	var selected e2bOperatorConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&selected) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("cloud operator configuration is invalid")
	}
	if selected.Deployment.Provider != "azure" {
		return nil
	}
	if selected.AzureMount == nil {
		return errors.New("Azure controller disk is not selected")
	}
	return verifyAzureDurableMount(*selected.AzureMount, selected, cfg)
}
