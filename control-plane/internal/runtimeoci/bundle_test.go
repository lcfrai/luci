package runtimeoci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func fixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	out, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func fixtureWrite(t *testing.T, root, name string, data []byte) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(filename), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filename, data, 0600); e != nil {
		t.Fatal(e)
	}
}
func fixtureBundle(t *testing.T, ownedLabel bool) (string, Manifest) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "runtime")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	m := Manifest{Version: 1, Kind: "luci-runtime-docker-save", SourceCommit: strings.Repeat("a", 40), CanonicalInputAlgorithm: "luci-git-blobs-v1"}
	m.Target.OS, m.Target.Architecture = "linux", "amd64"
	m.Archive.FileName = "runtime-image-linux-amd64.tar"
	m.Runtime.User, m.Runtime.WorkingDir = "dev", "/home/dev"
	m.Runtime.Entrypoint = []string{"/usr/local/bin/entrypoint.sh"}
	m.Runtime.ExposedPorts = []string{"8080/tcp"}
	m.Runtime.Harnesses.Codex, m.Runtime.Harnesses.ClaudeCode, m.Runtime.Harnesses.OpenCode = "0.160.0", "2.1.274", "1.18.34"
	source := map[string][]byte{"sandboxd/go.mod": []byte("module fixture\n"), "sandboxd/go.sum": {}, "sandboxd/vendor/modules.txt": []byte("# vendor\n"), "sandboxd/cmd/sandboxd/main.go": []byte("package main\n"), "images/sandbox/Dockerfile": []byte("FROM node:24-bookworm-slim\n"), "images/sandbox/entrypoint.sh": []byte("#!/bin/sh\n")}
	for name, data := range source {
		fixtureWrite(t, root, name, data)
		m.SourceFiles = append(m.SourceFiles, SourceFile{Member: Member{Path: name, SHA256: sumHex(data), Bytes: int64(len(data))}, Mode: "0644"})
	}
	sort.Slice(m.SourceFiles, func(i, j int) bool { return m.SourceFiles[i].Path < m.SourceFiles[j].Path })
	canonical := sha256.New()
	for _, v := range m.SourceFiles {
		canonical.Write([]byte(v.Path + "\x00" + v.Mode + "\x00" + decimal(v.Bytes) + "\x00" + v.SHA256 + "\n"))
		if v.Path == "images/sandbox/Dockerfile" {
			m.Producer.DockerfileSHA256 = v.SHA256
		}
	}
	m.CanonicalInputDigest = hex.EncodeToString(canonical.Sum(nil))
	m.Producer.BuildSourceCommit = m.SourceCommit
	notice := []byte("Fixture licence\n")
	fixtureWrite(t, parent, "licenses/fixture.txt", notice)
	m.Licenses = []LicenseFile{{Member: Member{Path: "licenses/fixture.txt", SHA256: sumHex(notice), Bytes: int64(len(notice))}, Component: "fixture", Version: "1.0", Grant: "MIT", Provenance: "https://example.invalid/LICENSE"}}
	layer := []byte("fixture layer contents")
	m.Image.DiffIDs = []string{"sha256:" + sumHex(layer)}
	m.Image.LayerPaths = []string{"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee/layer.tar"}
	m.Image.RepoTags = []string{}
	labels := map[string]string{"org.example.producer": "fixture"}
	if ownedLabel {
		labels["luci.runtime-image-allocation"] = "foreign"
	}
	config := fixtureJSON(t, map[string]any{"os": "linux", "architecture": "amd64", "config": map[string]any{"User": "dev", "WorkingDir": "/home/dev", "Entrypoint": m.Runtime.Entrypoint, "ExposedPorts": map[string]any{"8080/tcp": map[string]any{}}, "Labels": labels}, "rootfs": map[string]any{"diff_ids": m.Image.DiffIDs}})
	m.Image.ID = "sha256:" + sumHex(config)
	m.Image.ConfigPath = sumHex(config) + ".json"
	saved := fixtureJSON(t, []map[string]any{{"Config": m.Image.ConfigPath, "RepoTags": nil, "Layers": m.Image.LayerPaths}})
	parts := map[string][]byte{m.Image.ConfigPath: config, m.Image.LayerPaths[0]: layer, "manifest.json": saved}
	names := []string{}
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var tarred bytes.Buffer
	writer := tar.NewWriter(&tarred)
	for _, name := range names {
		data := parts[name]
		if e := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}); e != nil {
			t.Fatal(e)
		}
		if _, e := writer.Write(data); e != nil {
			t.Fatal(e)
		}
		m.Archive.Members = append(m.Archive.Members, Member{Path: name, SHA256: sumHex(data), Bytes: int64(len(data))})
	}
	if e := writer.Close(); e != nil {
		t.Fatal(e)
	}
	m.Archive.Bytes = int64(tarred.Len())
	m.Archive.SHA256 = sumHex(tarred.Bytes())
	fixtureWrite(t, root, m.Archive.FileName, tarred.Bytes())
	receipt := map[string]any{"version": 1, "kind": "luci-runtime-producer-build", "buildSourceCommit": m.SourceCommit, "canonicalInputAlgorithm": m.CanonicalInputAlgorithm, "canonicalInputDigest": m.CanonicalInputDigest, "target": m.Target, "generatedContextSHA256": strings.Repeat("b", 64), "baseImages": []map[string]string{{"reference": "golang:1.26-bookworm", "imageID": "sha256:" + strings.Repeat("c", 64), "repoDigest": "docker.io/library/golang@sha256:" + strings.Repeat("d", 64)}, {"reference": "node:24-bookworm-slim", "imageID": "sha256:" + strings.Repeat("e", 64), "repoDigest": "docker.io/library/node@sha256:" + strings.Repeat("f", 64)}}, "builder": map[string]any{"backend": "docker-builder-v1", "version": "28.5.1", "argv": []string{"docker", "build", "."}, "memoryBytes": 1073741824, "cpuQuotaMicros": 100000, "exitCode": 0, "stdoutSHA256": strings.Repeat("1", 64), "stderrSHA256": strings.Repeat("2", 64), "cacheAssistance": false, "cacheAssistanceLogSHA256": sumHex(nil)}, "imageID": m.Image.ID, "archive": map[string]any{"sha256": m.Archive.SHA256, "bytes": m.Archive.Bytes}, "probes": []any{map[string]any{"tool": "sandboxd", "argv": []string{"sandboxd", "--version"}, "exitCode": 0, "reportedVersion": "0.5.0", "stdoutSHA256": strings.Repeat("3", 64), "stderrSHA256": sumHex(nil)}, map[string]any{"tool": "codex", "argv": []string{"codex", "--version"}, "exitCode": 0, "reportedVersion": "0.160.0", "stdoutSHA256": strings.Repeat("4", 64), "stderrSHA256": sumHex(nil)}, map[string]any{"tool": "claudeCode", "argv": []string{"claude", "--version"}, "exitCode": 0, "reportedVersion": "2.1.274", "stdoutSHA256": strings.Repeat("5", 64), "stderrSHA256": sumHex(nil)}, map[string]any{"tool": "openCode", "argv": []string{"opencode", "--version"}, "exitCode": 0, "reportedVersion": "1.18.34", "stdoutSHA256": strings.Repeat("6", 64), "stderrSHA256": sumHex(nil)}}}
	receiptBytes := fixtureJSON(t, receipt)
	m.Producer.BuildReceiptSHA256 = sumHex(receiptBytes)
	fixtureWrite(t, root, "runtime-build-receipt.json", receiptBytes)
	fixtureWrite(t, root, "source-receipt.json", fixtureJSON(t, map[string]any{"version": 1, "sourceCommit": m.SourceCommit, "sourceDirty": false}))
	fixtureWrite(t, root, "runtime-oci-manifest-v1.json", fixtureJSON(t, m))
	return root, m
}

func TestBundleEquivalentInputReuseRequiresExactPriorBytes(t *testing.T) {
	root, m := fixtureBundle(t, false)
	priorManifest := fixtureJSON(t, m)
	priorReceipt, err := os.ReadFile(filepath.Join(root, "runtime-build-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, root, "runtime-prior-manifest-v1.json", priorManifest)
	fixtureWrite(t, root, "runtime-prior-build-receipt.json", priorReceipt)
	m.SourceCommit = strings.Repeat("b", 40)
	source := fixtureJSON(t, map[string]any{"version": 1, "sourceCommit": m.SourceCommit, "sourceDirty": false})
	fixtureWrite(t, root, "source-receipt.json", source)
	reuse := map[string]any{"version": 1, "kind": "luci-runtime-equivalent-input-reuse", "priorManifestSHA256": sumHex(priorManifest), "priorBuildReceiptSHA256": sumHex(priorReceipt), "priorBuildSourceCommit": m.Producer.BuildSourceCommit, "installedSourceCommit": m.SourceCommit, "installedSourceReceiptSHA256": sumHex(source), "canonicalInputAlgorithm": m.CanonicalInputAlgorithm, "canonicalInputDigest": m.CanonicalInputDigest, "imageID": m.Image.ID, "archiveSHA256": m.Archive.SHA256, "archiveBytes": m.Archive.Bytes}
	reuseBytes := fixtureJSON(t, reuse)
	fixtureWrite(t, root, "runtime-source-reuse-v1.json", reuseBytes)
	m.Producer.ReuseEvidenceSHA256 = sumHex(reuseBytes)
	fixtureWrite(t, root, "runtime-oci-manifest-v1.json", fixtureJSON(t, m))
	b, err := Open(root)
	if err != nil {
		t.Fatal("equivalent installed source refused", err)
	}
	b.Close()
	fixtureWrite(t, root, "runtime-prior-build-receipt.json", append(priorReceipt, ' '))
	if b, err = Open(root); err == nil {
		b.Close()
		t.Fatal("changed original producer receipt accepted")
	}
}

func TestBundleRequiresExactSourceReceiptFieldsAndUnlinkedLicenceTree(t *testing.T) {
	root, m := fixtureBundle(t, false)
	fixtureWrite(t, root, "source-receipt.json", fixtureJSON(t, map[string]any{"version": 1, "sourceCommit": m.SourceCommit}))
	if b, err := Open(root); err == nil {
		b.Close()
		t.Fatal("missing sourceDirty accepted")
	}
	fixtureWrite(t, root, "source-receipt.json", fixtureJSON(t, map[string]any{"version": 1, "sourceCommit": m.SourceCommit, "sourceDirty": false}))
	if err := os.Symlink("fixture.txt", filepath.Join(filepath.Dir(root), "licenses", "linked.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if b, err := Open(root); err == nil {
		b.Close()
		t.Fatal("unlisted linked licence accepted")
	}
}

func TestBundleRefusesZeroBuilderLimits(t *testing.T) {
	root, m := fixtureBundle(t, false)
	data, err := os.ReadFile(filepath.Join(root, "runtime-build-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["builder"].(map[string]any)["memoryBytes"] = 0
	data = fixtureJSON(t, receipt)
	fixtureWrite(t, root, "runtime-build-receipt.json", data)
	m.Producer.BuildReceiptSHA256 = sumHex(data)
	fixtureWrite(t, root, "runtime-oci-manifest-v1.json", fixtureJSON(t, m))
	if b, err := Open(root); err == nil {
		b.Close()
		t.Fatal("producer without a hard memory limit accepted")
	}
}
func decimal(v int64) string { return strconv.FormatInt(v, 10) }

func TestBundleOpenVerifiesExactTaglessArchiveAndSources(t *testing.T) {
	root, m := fixtureBundle(t, false)
	b, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if b.Manifest.Image.ID != m.Image.ID || b.SourceReceiptSHA256 == "" {
		t.Fatal("lost exact package identity")
	}
	if _, e = b.Archive.Seek(0, 0); e != nil {
		t.Fatal(e)
	}
}
func TestBundleRefusesChangedInputsAndOwnershipLabels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, string, Manifest)
	}{
		{"source-byte", func(t *testing.T, root string, _ Manifest) {
			fixtureWrite(t, root, "sandboxd/go.mod", []byte("changed"))
		}},
		{"archive-byte", func(t *testing.T, root string, m Manifest) {
			f, e := os.OpenFile(filepath.Join(root, m.Archive.FileName), os.O_WRONLY, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			if _, e = f.WriteAt([]byte("changed"), 0); e != nil {
				t.Fatal(e)
			}
		}},
		{"source-receipt", func(t *testing.T, root string, _ Manifest) {
			fixtureWrite(t, root, "source-receipt.json", []byte(`{"version":1,"sourceCommit":"wrong","sourceDirty":false}`))
		}},
		{"linked-source", func(t *testing.T, root string, _ Manifest) {
			p := filepath.Join(root, "sandboxd/go.mod")
			if e := os.Remove(p); e != nil {
				t.Fatal(e)
			}
			if e := os.Symlink("go.sum", p); e != nil {
				t.Skip(e)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m := fixtureBundle(t, false)
			tc.change(t, root, m)
			if b, e := Open(root); e == nil {
				b.Close()
				t.Fatal("changed package accepted")
			}
		})
	}
	root, _ := fixtureBundle(t, true)
	if b, e := Open(root); e == nil {
		b.Close()
		t.Fatal("shared payload with allocation label accepted")
	}
}
