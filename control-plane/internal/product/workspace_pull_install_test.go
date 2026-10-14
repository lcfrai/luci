package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

func TestWorkspacePullInstallTransfersExactSeparatePayloadsAndRetainsUncertainEffect(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, Status{Product: "Luci", Version: Version})
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, remote.URL)
	s.cloudBundle = cloudWorkspaceBundle(t)
	s.workspaceTransferAvailable = func(string) (uint64, error) { return 100 << 30, nil }
	root := t.TempDir()
	data := map[string][]byte{"metadata": []byte("metadata"), "store": []byte("verified compressed store"), "bootstrap": []byte("verified bootstrap")}
	for name, content := range data {
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s.workspacePullCompanionOpen = func(context.Context) (*workspaceCompanion, error) {
		core, err := os.Open(s.cloudBundle)
		if err != nil {
			return nil, err
		}
		metadata, err := os.Open(filepath.Join(root, "metadata"))
		if err != nil {
			core.Close()
			return nil, err
		}
		store, err := os.Open(filepath.Join(root, "store"))
		if err != nil {
			core.Close()
			metadata.Close()
			return nil, err
		}
		bootstrap, err := os.Open(filepath.Join(root, "bootstrap"))
		if err != nil {
			core.Close()
			metadata.Close()
			store.Close()
			return nil, err
		}
		info, err := core.Stat()
		if err != nil {
			return nil, err
		}
		pull := &runtimeoci.PullBundle{Store: store, Bootstrap: bootstrap}
		pull.Manifest.RegistryStore.FileName, pull.Manifest.RegistryStore.SHA256, pull.Manifest.RegistryStore.Bytes = "runtime-registry-store-v2.tar", hashWorkspaceBytes(data["store"]), int64(len(data["store"]))
		pull.Manifest.Bootstrap.FileName, pull.Manifest.Bootstrap.SHA256, pull.Manifest.Bootstrap.Bytes = "runtime-registry-bootstrap-linux-amd64.tar", hashWorkspaceBytes(data["bootstrap"]), int64(len(data["bootstrap"]))
		return &workspaceCompanion{core: core, metadata: metadata, pull: pull, coreHash: mustHashWorkspaceFile(t, core, info.Size()), coreSize: info.Size(), metadataHash: hashWorkspaceBytes(data["metadata"]), metadataSize: int64(len(data["metadata"])), markerHash: strings.Repeat("a", 64)}, nil
	}
	var calls atomic.Int32
	var failBootstrap atomic.Bool
	var transferArgs [][]string
	var script string
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		calls.Add(1)
		if exe == "scp" {
			transferArgs = append(transferArgs, append([]string{}, args...))
			if failBootstrap.Load() && strings.HasSuffix(args[len(args)-1], ".runtime-registry-bootstrap-linux-amd64.tar") {
				return "", errors.New("lost bootstrap transfer reply")
			}
			return "", nil
		}
		if exe == "ssh" {
			script = args[len(args)-1]
			return "", nil
		}
		return "", errors.New("unexpected executable")
	}
	path := "/v1/setup/ssh/connections/" + tunnel.ID + "/install-luci"
	s.workspaceTransferAvailable = func(string) (uint64, error) { return 1 << 30, nil }
	w := journeyRequest(t, mux, "POST", path, map[string]any{"acknowledgeInstall": true}, 202)
	var refused WorkspaceInstall
	_ = json.Unmarshal(w.Body.Bytes(), &refused)
	if job := awaitWorkspaceInstall(t, s, refused.ID); job.Status != "failed" || !strings.Contains(job.Message, "Waiting for storage") || calls.Load() != 0 {
		t.Fatal("under-capacity v2 staging reached remote work", job)
	}
	s.workspaceTransferAvailable = func(string) (uint64, error) { return 100 << 30, nil }
	w = journeyRequest(t, mux, "POST", path, map[string]any{"acknowledgeInstall": true}, 202)
	var accepted WorkspaceInstall
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	if job := awaitWorkspaceInstall(t, s, accepted.ID); job.Status != "ready" || job.RuntimeTransport != "v2" || calls.Load() != 5 {
		t.Fatalf("v2 transfer did not complete: %#v, calls %d", job, calls.Load())
	}
	if len(transferArgs) != 4 || !strings.HasSuffix(transferArgs[2][len(transferArgs[2])-1], ".runtime-registry-store-v2.tar") || !strings.HasSuffix(transferArgs[3][len(transferArgs[3])-1], ".runtime-registry-bootstrap-linux-amd64.tar") {
		t.Fatal("large payloads were not separate and exact", transferArgs)
	}
	if !strings.Contains(script, hashWorkspaceBytes(data["store"])) || !strings.Contains(script, hashWorkspaceBytes(data["bootstrap"])) || !strings.Contains(script, "runtime/runtime-payload-companion.json") || strings.Contains(script, "runtime-image-linux-amd64.tar") {
		t.Fatal("remote script did not bind both v2 files and marker")
	}
	for _, args := range transferArgs {
		if !strings.Contains(strings.Join(args, "\n"), "StrictHostKeyChecking=yes") {
			t.Fatal("v2 transfer lost pinned SSH host checking")
		}
	}
	failBootstrap.Store(true)
	tunnel = workspaceTunnel(s, remote.URL)
	s.mu.Lock()
	delete(s.tunnels, tunnel.ID)
	tunnel.ID = "tunnel-second"
	s.tunnels[tunnel.ID] = tunnel
	s.mu.Unlock()
	path = "/v1/setup/ssh/connections/" + tunnel.ID + "/install-luci"
	w = journeyRequest(t, mux, "POST", path, map[string]any{"acknowledgeInstall": true}, 202)
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	if job := awaitWorkspaceInstall(t, s, accepted.ID); job.Status != "unknown" {
		t.Fatal("lost bootstrap transfer was cleared", job)
	}
	before := calls.Load()
	w = journeyRequest(t, mux, "POST", path, map[string]any{"acknowledgeInstall": true}, 200)
	var retained WorkspaceInstall
	_ = json.Unmarshal(w.Body.Bytes(), &retained)
	if retained.ID != accepted.ID || calls.Load() != before {
		t.Fatal("uncertain v2 transfer was replayed")
	}
}

func mustHashWorkspaceFile(t *testing.T, file *os.File, size int64) string {
	t.Helper()
	hash, err := hashWorkspaceFile(file, size, "")
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
