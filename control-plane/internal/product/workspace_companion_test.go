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

func companionCoreFixture(t *testing.T, bundle *runtimeoci.Bundle, markerOverride func(map[string]any), extraRuntimeGrant bool) *os.File {
	t.Helper()
	m := bundle.Manifest
	marker := map[string]any{"version": 1, "kind": "luci-runtime-companion-required", "sourceCommit": m.SourceCommit, "manifestSHA256": bundle.ManifestSHA256, "archiveSHA256": m.Archive.SHA256, "archiveBytes": m.Archive.Bytes, "fileName": m.Archive.FileName, "ordinaryLocalInstall": "requires-verified-companion", "automaticSourceBuildFallback": false}
	if markerOverride != nil {
		markerOverride(marker)
	}
	markerBytes, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{"luci": []byte("executable"), "runtime/runtime-payload-companion.json": markerBytes, "runtime/source-receipt.json": []byte("source"), "runtime/sandboxd/go.mod": []byte("module fixture\n"), "licenses/frontend/MIT.txt": []byte("unrelated frontend grant\n")}
	if extraRuntimeGrant {
		parts["licenses/runtime-image/extra.txt"] = []byte("unlisted runtime grant")
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "core.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tarred := tar.NewWriter(gz)
	for name, data := range parts {
		if err := tarred.WriteHeader(&tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarred.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, close := range []func() error{tarred.Close, gz.Close, file.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
	file, err = os.Open(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestWorkspaceCompanionMetadataStagesOnlyVerifiedRuntimeMembers(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{
		"runtime/runtime-oci-manifest-v1.json": []byte("manifest fixture"),
		"runtime/runtime-build-receipt.json":   []byte("build fixture"),
		"licenses/runtime-image/MIT.txt":       []byte("runtime grant fixture"),
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
	archivePath := filepath.Join(runtimeDir, "runtime-image-linux-amd64.tar")
	if err := os.WriteFile(archivePath, []byte("image fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	m := runtimeoci.Manifest{}
	m.Producer.BuildReceiptSHA256 = hashWorkspaceBytes(parts["runtime/runtime-build-receipt.json"])
	m.Licenses = []runtimeoci.LicenseFile{{Member: runtimeoci.Member{Path: "licenses/runtime-image/MIT.txt", SHA256: hashWorkspaceBytes(parts["licenses/runtime-image/MIT.txt"]), Bytes: int64(len(parts["licenses/runtime-image/MIT.txt"]))}}}
	bundle := &runtimeoci.Bundle{Manifest: m, Archive: archive, ManifestSHA256: hashWorkspaceBytes(parts["runtime/runtime-oci-manifest-v1.json"])}
	file, name, digest, size, err := buildWorkspaceMetadata(bundle, filepath.Join(root, "private-stage"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { file.Close(); os.Remove(name) }()
	if digest == "" || size == 0 {
		t.Fatal("metadata was not bound")
	}
	reader := tar.NewReader(file)
	got := map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		got[header.Name] = data
	}
	if len(got) != len(parts) {
		t.Fatal("metadata inventory changed", got)
	}
	for name, data := range parts {
		if string(got[name]) != string(data) {
			t.Fatal("runtime metadata changed", name)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "licenses/runtime-image/MIT.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := buildWorkspaceMetadata(bundle, filepath.Join(root, "private-stage")); err == nil {
		t.Fatal("changed runtime grant was transferred")
	}
}

func TestWorkspaceCompanionCoreBindsMarkerRuntimeSourcesAndExcludesDedicatedGrants(t *testing.T) {
	m := runtimeoci.Manifest{SourceCommit: strings.Repeat("a", 40)}
	m.Archive.FileName, m.Archive.SHA256, m.Archive.Bytes = "runtime-image-linux-amd64.tar", strings.Repeat("b", 64), 1024
	m.Producer.BuildReceiptSHA256 = hashWorkspaceBytes([]byte("producer"))
	m.SourceFiles = []runtimeoci.SourceFile{{Member: runtimeoci.Member{Path: "sandboxd/go.mod", SHA256: hashWorkspaceBytes([]byte("module fixture\n")), Bytes: int64(len("module fixture\n"))}, Mode: "0644"}}
	m.Licenses = []runtimeoci.LicenseFile{{Member: runtimeoci.Member{Path: "licenses/runtime-image/MIT.txt", SHA256: hashWorkspaceBytes([]byte("runtime grant\n")), Bytes: int64(len("runtime grant\n"))}}}
	bundle := &runtimeoci.Bundle{Manifest: m, ManifestSHA256: hashWorkspaceBytes([]byte("manifest")), SourceReceiptSHA256: hashWorkspaceBytes([]byte("source"))}
	file := companionCoreFixture(t, bundle, nil, false)
	markerHash, err := scanWorkspaceCore(file, bundle)
	if err != nil || markerHash == "" {
		t.Fatal("verified core and unrelated outer grant refused", markerHash, err)
	}
	file = companionCoreFixture(t, bundle, func(marker map[string]any) { marker["archiveSHA256"] = strings.Repeat("c", 64) }, false)
	if _, err := scanWorkspaceCore(file, bundle); err == nil {
		t.Fatal("wrong companion marker accepted")
	}
	file = companionCoreFixture(t, bundle, nil, true)
	if _, err := scanWorkspaceCore(file, bundle); err == nil {
		t.Fatal("unlisted runtime grant accepted")
	}
}
