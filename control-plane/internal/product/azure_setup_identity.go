package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type AzureConnectionPin struct {
	SourceCommit         string `json:"sourceCommit"`
	Scope                string `json:"scope"`
	Generation           string `json:"generation"`
	ControllerResourceID string `json:"controllerResourceID"`
	DataDiskID           string `json:"dataDiskID"`
	BootID               string `json:"bootID"`
	Fingerprint          string `json:"fingerprint"`
}

var azureSetupScope = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,127}$`)
var azureSetupUUID = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)
var azureSetupVMID = regexp.MustCompile(`^/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft.compute/virtualmachines/[a-z0-9_-]+$`)
var azureSetupDiskID = regexp.MustCompile(`^/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft.compute/disks/[a-z0-9_-]+$`)

func validateAzureConnectionPin(request SSHRequest) error {
	pin := request.AzureIdentity
	if pin == nil {
		return nil
	}
	ip := net.ParseIP(request.Host)
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(pin.Fingerprint, "SHA256:"))
	if ip == nil || ip.To4() == nil || !ip.IsPrivate() || request.Port != 22 || request.User != "luci" ||
		!azureSetupGit.MatchString(pin.SourceCommit) || !azureSetupScope.MatchString(pin.Scope) || !azureSetupScope.MatchString(pin.Generation) ||
		!azureSetupVMID.MatchString(pin.ControllerResourceID) || !azureSetupDiskID.MatchString(pin.DataDiskID) ||
		!azureSetupUUID.MatchString(pin.BootID) || !strings.HasPrefix(pin.Fingerprint, "SHA256:") || err != nil || len(raw) != 32 ||
		strings.Split(pin.ControllerResourceID, "/providers/")[0] != strings.Split(pin.DataDiskID, "/providers/")[0] {
		return errors.New("Azure management needs the exact private controller, source, generation and authenticated host-key receipt.")
	}
	return nil
}

func readAzurePrivateIdentityFile(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("private Azure identity path is unavailable")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > limit || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private Azure identity file is unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("private Azure identity changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("private Azure identity exceeds its bound")
	}
	return data, nil
}

func (s *Server) getAzureControllerIdentity(w http.ResponseWriter, r *http.Request) {
	if _, _, managed := management.ActorFromContext(r.Context()); !managed || !s.managedWorkspace || s.azureOperatorPath == "" {
		problem(w, 403, "azure_identity_unavailable", "Sign in to this protected Azure workspace before reading its current generation.")
		return
	}
	data, err := readAzurePrivateIdentityFile(s.azureOperatorPath, 16<<10)
	var selected struct {
		SourceCommit string                                                                                                       `json:"sourceCommit"`
		Deployment   struct{ Provider, Scope, HostGeneration string }                                                             `json:"deployment"`
		Mount        struct{ MountPath, Generation, ControllerInstanceID, GenerationReceiptPath, GenerationReceiptDigest string } `json:"azureMount"`
		Store        struct{ AzureDataDiskID string }                                                                             `json:"store"`
	}
	if err != nil || json.Unmarshal(data, &selected) != nil || selected.Deployment.Provider != "azure" ||
		!azureSetupGit.MatchString(selected.SourceCommit) || !azureSetupScope.MatchString(selected.Deployment.Scope) ||
		selected.Mount.Generation != selected.Deployment.HostGeneration || !azureSetupScope.MatchString(selected.Mount.Generation) ||
		!azureSetupVMID.MatchString(selected.Mount.ControllerInstanceID) || !azureSetupDiskID.MatchString(selected.Store.AzureDataDiskID) {
		problem(w, 409, "azure_identity_unavailable", "The current signed Azure controller selection is incomplete.")
		return
	}
	rel, pathErr := filepath.Rel(selected.Mount.MountPath, selected.Mount.GenerationReceiptPath)
	if pathErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		problem(w, 409, "azure_identity_unavailable", "The current generation receipt is outside the selected controller disk.")
		return
	}
	data, err = readAzurePrivateIdentityFile(selected.Mount.GenerationReceiptPath, 4096)
	receiptDigest := sha256.Sum256(data)
	var generation struct{ Generation, ControllerInstanceID, AzureDataDiskID string }
	if err != nil || json.Unmarshal(data, &generation) != nil || generation.Generation != selected.Mount.Generation ||
		generation.ControllerInstanceID != selected.Mount.ControllerInstanceID || generation.AzureDataDiskID != selected.Store.AzureDataDiskID ||
		hex.EncodeToString(receiptDigest[:]) != selected.Mount.GenerationReceiptDigest {
		problem(w, 409, "azure_generation_changed", "The controller generation receipt changed. Obtain fresh authenticated setup readback.")
		return
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil || !azureSetupUUID.MatchString(strings.TrimSpace(string(boot))) {
		problem(w, 409, "azure_identity_unavailable", "The current Linux boot identity is unavailable.")
		return
	}
	writeJSON(w, 200, map[string]any{"version": 1, "state": "current_controller_generation_observed",
		"sourceCommit": selected.SourceCommit, "scope": selected.Deployment.Scope, "generation": generation.Generation,
		"controllerResourceID": generation.ControllerInstanceID, "dataDiskID": generation.AzureDataDiskID,
		"bootID": strings.TrimSpace(string(boot)), "microVMExecutionVerified": false})
}

func verifyAzureWorkspaceIdentity(ctx context.Context, address, token string, pin *AzureConnectionPin) error {
	if pin == nil {
		return nil
	}
	var observed struct {
		State, SourceCommit, Scope, Generation, ControllerResourceID, DataDiskID, BootID string
	}
	if remoteWorkspaceAPI(ctx, address, "GET", "/v1/setup/cloud/azure/identity", token, nil, &observed) != nil ||
		observed.State != "current_controller_generation_observed" || observed.SourceCommit != pin.SourceCommit ||
		observed.Scope != pin.Scope || observed.Generation != pin.Generation || observed.ControllerResourceID != pin.ControllerResourceID ||
		observed.DataDiskID != pin.DataDiskID || observed.BootID != pin.BootID {
		return errors.New("Azure controller source, boot or fenced generation changed. Obtain a fresh authenticated connection descriptor.")
	}
	return nil
}

// Check at the request boundary: a prior status observation is not authority
// to forward another mutation, obtain a socket ticket or open a preview join.
func (s *Server) gateAzureWorkspaceRequest(ctx context.Context, connectionID, address, token string, pin *AzureConnectionPin) error {
	if pin == nil {
		return nil // Retained generic SSH behavior is unchanged.
	}
	if err := verifyAzureWorkspaceIdentity(ctx, address, token, pin); err != nil {
		s.mu.Lock()
		if s.workspace.Mode == "cloud" && s.workspace.ConnectionID == connectionID {
			s.workspace.Ready = false
		}
		s.mu.Unlock()
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tunnel := s.tunnels[connectionID]
	if s.workspace.Mode != "cloud" || s.workspace.ConnectionID != connectionID || tunnel == nil ||
		tunnel.Status != "connected" || tunnel.APIURL != address || tunnel.Request.AzureIdentity == nil || *tunnel.Request.AzureIdentity != *pin ||
		s.workspaceAuth[workspaceCredentialKey(*tunnel)].Token != token {
		return errors.New("The selected Azure management connection changed before this request.")
	}
	return nil
}
