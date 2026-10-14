package product

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testAzureSetupArchive(t *testing.T, runtimeDir string) string {
	return testAzureSetupArchiveWithSource(t, runtimeDir, strings.Repeat("a", 40))
}

func testAzureSetupArchiveWithSource(t *testing.T, runtimeDir, sourceRevision string) string {
	t.Helper()
	files := make(map[string][]byte)
	for _, name := range azureSetupRequired {
		files[name] = []byte("verified " + name + "\n")
	}
	source := azureBuilderSourceRef{Version: 1, Kind: "luci-azure-builder-source"}
	source.Luci.Revision = sourceRevision
	source.Luci.Tree = strings.Repeat("b", 40)
	source.E2B.Revision = azureBuilderE2BRevision
	source.E2B.Tree = strings.Repeat("c", 40)
	for _, role := range []struct {
		name string
		ref  *azureBuilderSourceRole
	}{
		{"luci", &source.Luci}, {"e2b", &source.E2B},
	} {
		part := files["builder-source/"+role.name+".pack"]
		sum := sha256.Sum256(part)
		role.ref.Pack.SHA256 = hex.EncodeToString(sum[:])
		role.ref.Pack.Bytes = int64(len(part))
	}
	files["builder-source/source.json"], _ = json.Marshal(source)
	manifest := azureSetupBundleManifest{Version: 1, LuciRevision: strings.Repeat("a", 40)}
	manifest.Files = make(map[string]struct {
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	})
	for name, data := range files {
		sum := sha256.Sum256(data)
		manifest.Files[name] = struct {
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
		}{hex.EncodeToString(sum[:]), int64(len(data))}
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = manifestData
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, data := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "azure-setup.tar"), buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buffer.Bytes())
	digest := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(runtimeDir, "azure-setup.sha256"), []byte(digest+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestAzureSetupBundleRefusesCrossRevisionBuilderSource(t *testing.T) {
	root := t.TempDir()
	testAzureSetupArchiveWithSource(t, root, strings.Repeat("f", 40))
	if _, _, _, err := readAzureSetupBundle(root, filepath.Join(root, "projects")); err == nil || !strings.Contains(err.Error(), "builder source revision") {
		t.Fatalf("cross-revision builder source was accepted: %v", err)
	}
}

func TestAzureDevelopSetupPlanAndBundleAreSourceBoundReads(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	if err := os.Mkdir(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	want := testAzureSetupArchive(t, runtimeDir)
	s, err := New(Options{StateDir: filepath.Join(root, "state"), RuntimeDir: runtimeDir,
		Runner: func(_ context.Context, _ string, _ []string, _ string) (string, error) {
			t.Fatal("plan launched a command")
			return "", nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	s.Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/setup/cloud/azure/plan", strings.NewReader(`{"name":"luci-test","region":"australiaeast","monthlyBudgetUsd":100,"maxConcurrentVMs":4}`)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var plan map[string]any
	if json.Unmarshal(w.Body.Bytes(), &plan) != nil || plan["bundleSHA256"] != want || plan["sourceCommit"] != strings.Repeat("a", 40) || plan["status"] != "needs_prerequisites" {
		t.Fatal("plan lost source binding or pretended readiness", w.Body.String())
	}
	commands, okay := plan["commands"].([]any)
	if !okay || len(commands) < 4 || !strings.Contains(commands[4].(string), "init-selection") ||
		!strings.Contains(commands[4].(string), "--max-concurrent-vms 4") {
		t.Fatal("the small-input setup command is unavailable")
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/setup/cloud/azure/bundle", nil))
	if w.Code != 200 || w.Header().Get("X-Luci-Archive-SHA256") != want {
		t.Fatal("installed archive was not served with its exact digest")
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "azure-setup.sha256"), []byte(strings.Repeat("0", 64)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/setup/cloud/azure/bundle", nil))
	if w.Code != 503 {
		t.Fatal("tampered archive identity was served", w.Code)
	}
}

func TestAzureSetupBundleRejectsOversizedInputsBeforeReading(t *testing.T) {
	root := t.TempDir()
	for _, row := range []struct {
		name string
		size int64
	}{
		{"azure-setup.tar", azureSetupBundleMax + 1},
		{"azure-setup.sha256", 81},
	} {
		runtimeDir := filepath.Join(root, row.name+"-runtime")
		if err := os.Mkdir(runtimeDir, 0700); err != nil {
			t.Fatal(err)
		}
		testAzureSetupArchive(t, runtimeDir)
		file, err := os.OpenFile(filepath.Join(runtimeDir, row.name), os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(row.size); err != nil {
			t.Fatal(err)
		}
		file.Close()
		if _, _, _, err := readAzureSetupBundle(runtimeDir, filepath.Join(runtimeDir, "projects")); err == nil {
			t.Fatalf("oversized %s was read", row.name)
		}
	}
}
