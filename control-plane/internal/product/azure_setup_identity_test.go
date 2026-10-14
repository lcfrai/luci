package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func azurePinFixture() (SSHRequest, string) {
	raw := []byte("\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20" + strings.Repeat("\x01", 32))
	sum := sha256.Sum256(raw)
	prefix := "/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/luci-rg/providers/microsoft.compute/"
	request := SSHRequest{Host: "10.240.1.10", Port: 22, User: "luci", AzureIdentity: &AzureConnectionPin{
		SourceCommit: strings.Repeat("a", 40), Scope: "scope-one", Generation: "gen-one",
		ControllerResourceID: prefix + "virtualmachines/controller", DataDiskID: prefix + "disks/state",
		BootID: "12345678-1234-1234-1234-123456789abc", Fingerprint: "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])}}
	return request, request.Host + " ssh-ed25519 " + base64.StdEncoding.EncodeToString(raw)
}

func TestAzureConnectionPinRefusesPublicAndMismatchedSSHIdentity(t *testing.T) {
	request, _ := azurePinFixture()
	if err := validateSSH(request); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"203.0.113.1", "127.0.0.1", "169.254.169.254", "100.64.0.1", "controller.example", "::1"} {
		changed := request
		changed.Host = host
		if validateSSH(changed) == nil {
			t.Fatal("accepted non-RFC1918 Azure management host", host)
		}
	}
	changed := request
	changed.Port = 443
	if validateSSH(changed) == nil {
		t.Fatal("accepted a runtime gateway as SSH management")
	}
	changed = request
	changed.User = "root"
	if validateSSH(changed) == nil {
		t.Fatal("accepted a different controller account")
	}
}

func TestAzureSSHInspectAcceptsOnlyAuthenticatedReadbackFingerprint(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerConnections(mux)
	request, key := azurePinFixture()
	s.runner = func(context.Context, string, []string, string) (string, error) { return key + "\n", nil }
	w := journeyRequest(t, mux, "POST", "/v1/setup/ssh/inspect", request, 200)
	var scan SSHScan
	if json.Unmarshal(w.Body.Bytes(), &scan) != nil || len(scan.Fingerprints) != 1 || scan.Fingerprints[0] != request.AzureIdentity.Fingerprint {
		t.Fatal("lost pinned host key")
	}
	request.AzureIdentity.Fingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	journeyRequest(t, mux, "POST", "/v1/setup/ssh/inspect", request, 502)
}

func TestAzureWorkspaceActivationRefusesStaleAuthenticatedGeneration(t *testing.T) {
	request, _ := azurePinFixture()
	var stale atomic.Bool
	stale.Store(true)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/me" {
			writeJSON(w, 200, map[string]any{"user": map[string]string{"id": "remote-owner", "role": "super-admin"}})
			return
		}
		if r.URL.Path == "/v1/setup/cloud/azure/identity" {
			if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("r", 43) {
				t.Error("identity readback lost account credential")
			}
			generation := request.AzureIdentity.Generation
			if stale.Load() {
				generation = "gen-stale"
			}
			writeJSON(w, 200, map[string]any{"state": "current_controller_generation_observed", "sourceCommit": request.AzureIdentity.SourceCommit,
				"scope": request.AzureIdentity.Scope, "generation": generation, "controllerResourceID": request.AzureIdentity.ControllerResourceID,
				"dataDiskID": request.AzureIdentity.DataDiskID, "bootID": request.AzureIdentity.BootID})
			return
		}
		writeJSON(w, 200, map[string]string{"product": "Luci", "version": Version})
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, remote.URL)
	tunnel.Request = request
	s.workspaceAuth[workspaceCredentialKey(*tunnel)] = workspaceCredential{Token: strings.Repeat("r", 43)}
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 409)
	if s.workspace.Mode != "local" {
		t.Fatal("stale controller replaced active workspace")
	}
	stale.Store(false)
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 200)
	stale.Store(true)
	if s.workspaceStatus(context.Background(), "GET").Workspace.Ready {
		t.Fatal("stale generation remained ready through a status read")
	}
}
