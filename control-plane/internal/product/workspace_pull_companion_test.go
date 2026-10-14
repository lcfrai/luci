package product

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

func TestWorkspacePullCompanionBindsCoreAndSeparateV2Metadata(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{
		"runtime/source-receipt.json":                         []byte("source"),
		"runtime/sandboxd/go.mod":                             []byte("module fixture\n"),
		"runtime/runtime-oci-pull-manifest-v2.json":           []byte("manifest"),
		"runtime/runtime-build-receipt.json":                  []byte("build"),
		"runtime/runtime-pull-producer-receipt-v2.json":       []byte("producer"),
		"runtime/runtime-registry-conversion-receipt-v2.json": []byte("conversion"),
		"runtime/runtime-bootstrap-build-receipt-v1.json":     []byte("bootstrap receipt"),
		"licenses/runtime-image/MIT.txt":                      []byte("runtime grant"),
		"licenses/runtime-bootstrap/Apache.txt":               []byte("bootstrap grant"),
	}
	for name, data := range parts {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string][]byte{"runtime-registry-store-v2.tar": []byte("store"), "runtime-registry-bootstrap-linux-amd64.tar": []byte("bootstrap")} {
		if err := os.WriteFile(filepath.Join(runtimeDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := os.Open(filepath.Join(runtimeDir, "runtime-registry-store-v2.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bootstrap, err := os.Open(filepath.Join(runtimeDir, "runtime-registry-bootstrap-linux-amd64.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer bootstrap.Close()
	b := &runtimeoci.PullBundle{Store: store, Bootstrap: bootstrap, ManifestSHA256: hashWorkspaceBytes(parts["runtime/runtime-oci-pull-manifest-v2.json"]), SourceReceiptSHA256: hashWorkspaceBytes(parts["runtime/source-receipt.json"])}
	m := &b.Manifest
	m.SourceCommit = strings.Repeat("a", 40)
	m.SourceFiles = []runtimeoci.SourceFile{{Member: runtimeoci.Member{Path: "sandboxd/go.mod", SHA256: hashWorkspaceBytes(parts["runtime/sandboxd/go.mod"]), Bytes: int64(len(parts["runtime/sandboxd/go.mod"]))}}}
	m.RegistryStore.FileName, m.RegistryStore.SHA256, m.RegistryStore.Bytes = "runtime-registry-store-v2.tar", hashWorkspaceBytes([]byte("store")), 5
	m.Bootstrap.FileName, m.Bootstrap.SHA256, m.Bootstrap.Bytes = "runtime-registry-bootstrap-linux-amd64.tar", hashWorkspaceBytes([]byte("bootstrap")), 9
	m.Producer.OriginalBuildReceiptSHA256 = hashWorkspaceBytes(parts["runtime/runtime-build-receipt.json"])
	m.Producer.PullProducerReceiptSHA256 = hashWorkspaceBytes(parts["runtime/runtime-pull-producer-receipt-v2.json"])
	m.Producer.ConversionReceiptSHA256 = hashWorkspaceBytes(parts["runtime/runtime-registry-conversion-receipt-v2.json"])
	m.Bootstrap.BuildReceiptSHA256 = hashWorkspaceBytes(parts["runtime/runtime-bootstrap-build-receipt-v1.json"])
	for _, name := range []string{"licenses/runtime-image/MIT.txt", "licenses/runtime-bootstrap/Apache.txt"} {
		m.Licenses = append(m.Licenses, runtimeoci.LicenseFile{Member: runtimeoci.Member{Path: name, SHA256: hashWorkspaceBytes(parts[name]), Bytes: int64(len(parts[name]))}})
	}
	marker := map[string]any{"version": 2, "kind": "luci-runtime-pull-companion-required", "sourceCommit": m.SourceCommit, "manifestSHA256": b.ManifestSHA256, "registryStoreSHA256": m.RegistryStore.SHA256, "registryStoreBytes": m.RegistryStore.Bytes, "bootstrapSHA256": m.Bootstrap.SHA256, "bootstrapBytes": m.Bootstrap.Bytes, "ordinaryLocalInstall": "requires-verified-companion", "automaticSourceBuildFallback": false}
	core := func(extra string) *os.File {
		entries := map[string][]byte{"luci": []byte("binary"), "runtime/source-receipt.json": parts["runtime/source-receipt.json"], "runtime/sandboxd/go.mod": parts["runtime/sandboxd/go.mod"]}
		entries["runtime/runtime-payload-companion.json"], _ = json.Marshal(marker)
		if extra != "" {
			entries[extra] = []byte("forbidden")
		}
		file, err := os.Create(filepath.Join(t.TempDir(), "core.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(file)
		tw := tar.NewWriter(gz)
		for name, data := range entries {
			if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(data))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		file, err = os.Open(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		return file
	}
	if markerHash, err := scanWorkspacePullCore(core(""), b); err != nil || markerHash == "" {
		t.Fatal("v2 core refused", err)
	}
	if _, err := scanWorkspacePullCore(core("runtime/runtime-registry-store-v2.tar"), b); err == nil {
		t.Fatal("core duplicated large payload")
	}
	marker["bootstrapSHA256"] = strings.Repeat("f", 64)
	if _, err := scanWorkspacePullCore(core(""), b); err == nil {
		t.Fatal("wrong bootstrap binding accepted")
	}
	metadata, path, _, _, err := buildWorkspacePullMetadata(b, filepath.Join(root, "private-stage"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { metadata.Close(); os.Remove(path) }()
	seen := map[string]bool{}
	reader := tar.NewReader(metadata)
	for {
		h, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[h.Name] = true
	}
	if len(seen) != 7 || seen["runtime/runtime-registry-store-v2.tar"] || seen["runtime/runtime-registry-bootstrap-linux-amd64.tar"] {
		t.Fatal("metadata duplicated payload or omitted receipt", seen)
	}
	if err := os.WriteFile(filepath.Join(root, "licenses/runtime-bootstrap/Apache.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := buildWorkspacePullMetadata(b, filepath.Join(root, "private-stage")); err == nil {
		t.Fatal("changed bootstrap grant accepted")
	}
}
