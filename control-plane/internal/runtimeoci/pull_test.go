package runtimeoci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func pullFixtureManifest() PullManifest {
	hash := strings.Repeat("a", 64)
	id := "sha256:" + hash
	m := PullManifest{Version: 2, Kind: "luci-runtime-registry-pull", SourceCommit: strings.Repeat("b", 40), CanonicalInputAlgorithm: "luci-git-blobs-v1", CanonicalInputDigest: hash}
	m.Target.OS, m.Target.Architecture = "linux", "amd64"
	m.SourceFiles = []SourceFile{{Member: Member{Path: "images/sandbox/Dockerfile", SHA256: hash, Bytes: 1}, Mode: "0644"}, {Member: Member{Path: "sandboxd/go.mod", SHA256: hash, Bytes: 1}, Mode: "0644"}}
	m.Runtime.User, m.Runtime.WorkingDir = "dev", "/home/dev"
	m.Runtime.Entrypoint, m.Runtime.ExposedPorts = []string{"/usr/local/bin/entrypoint.sh"}, []string{"8080/tcp"}
	m.Runtime.Harnesses.Codex, m.Runtime.Harnesses.ClaudeCode, m.Runtime.Harnesses.OpenCode = "0.160.0", "2.1.274", "1.18.34"
	m.Image.ID, m.Image.ConfigSHA256, m.Image.ConfigBytes = id, hash, 5
	m.Image.ManifestDigest, m.Image.ManifestMediaType = id, "application/vnd.docker.distribution.manifest.v2+json"
	m.Image.Layers = []PullLayer{{CompressedDigest: "sha256:" + strings.Repeat("c", 64), CompressedBytes: 8, DiffID: "sha256:" + strings.Repeat("d", 64), UncompressedBytes: 16, MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip"}}
	m.RegistryStore.FileName, m.RegistryStore.SHA256, m.RegistryStore.Bytes, m.RegistryStore.Repository = "runtime-registry-store-v2.tar", hash, 1024, "luci/runtime-installed"
	m.RegistryStore.Members = []Member{{Path: "docker/registry/v2/a", SHA256: hash, Bytes: 1}, {Path: "docker/registry/v2/b", SHA256: hash, Bytes: 1}, {Path: "docker/registry/v2/c", SHA256: hash, Bytes: 1}, {Path: "docker/registry/v2/d", SHA256: hash, Bytes: 1}}
	m.Bootstrap.FileName, m.Bootstrap.SHA256, m.Bootstrap.Bytes = "runtime-registry-bootstrap-linux-amd64.tar", hash, 1024
	m.Bootstrap.ImageID, m.Bootstrap.ConfigSHA256, m.Bootstrap.ConfigBytes = id, hash, 5
	m.Bootstrap.Layers = []BootstrapLayer{{DiffID: id, UncompressedBytes: 16}}
	m.Bootstrap.User, m.Bootstrap.WorkingDir = "65532:65532", "/"
	m.Bootstrap.Entrypoint, m.Bootstrap.ExposedPorts, m.Bootstrap.Environment = []string{"/luci-registry-gate"}, []string{"5000/tcp"}, []string{}
	m.Bootstrap.DistributionVersion, m.Bootstrap.DistributionArchiveSHA256 = "3.0.0", "61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d"
	m.Bootstrap.DistributionBinarySHA256, m.Bootstrap.DistributionLicenseSHA256, m.Bootstrap.GateSourceSHA256, m.Bootstrap.GateBinarySHA256, m.Bootstrap.BuildRecipeSHA256, m.Bootstrap.BuildReceiptSHA256 = hash, hash, hash, hash, hash, hash
	m.Producer.OriginalBuildReceiptSHA256, m.Producer.ConversionReceiptSHA256, m.Producer.PullProducerReceiptSHA256 = hash, hash, hash
	m.Licenses = []LicenseFile{{Member: Member{Path: "licenses/runtime-bootstrap/distribution", SHA256: hash, Bytes: 1}}, {Member: Member{Path: "licenses/runtime-image/fixture", SHA256: hash, Bytes: 1}}}
	return m
}

func TestPullStoreBindsOriginalConfigAndCompressedLayer(t *testing.T) {
	plain := []byte("one verified layer")
	diff := sha256.Sum256(plain)
	var zipped bytes.Buffer
	compressor := gzip.NewWriter(&zipped)
	if _, err := compressor.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := zipped.Bytes()
	compressedHash := sha256.Sum256(compressed)
	layerDigest := "sha256:" + hex.EncodeToString(compressedHash[:])
	diffID := "sha256:" + hex.EncodeToString(diff[:])
	m := pullFixtureManifest()
	m.Image.Layers = []PullLayer{{CompressedDigest: layerDigest, CompressedBytes: int64(len(compressed)), DiffID: diffID, UncompressedBytes: int64(len(plain)), MediaType: m.Image.Layers[0].MediaType}}
	config, err := json.Marshal(map[string]any{"os": "linux", "architecture": "amd64", "config": map[string]any{"User": "dev", "WorkingDir": "/home/dev", "Entrypoint": m.Runtime.Entrypoint, "ExposedPorts": map[string]any{"8080/tcp": map[string]any{}}, "Labels": map[string]string{}}, "rootfs": map[string]any{"diff_ids": []string{diffID}}})
	if err != nil {
		t.Fatal(err)
	}
	configHash := sha256.Sum256(config)
	m.Image.ID, m.Image.ConfigSHA256, m.Image.ConfigBytes = "sha256:"+hex.EncodeToString(configHash[:]), hex.EncodeToString(configHash[:]), int64(len(config))
	manifest, err := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": m.Image.ManifestMediaType, "config": map[string]any{"mediaType": "application/vnd.docker.container.image.v1+json", "size": len(config), "digest": m.Image.ID}, "layers": []map[string]any{{"mediaType": m.Image.Layers[0].MediaType, "size": len(compressed), "digest": layerDigest}}})
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(manifest)
	m.Image.ManifestDigest = "sha256:" + hex.EncodeToString(manifestHash[:])
	content := map[string][]byte{pullBlobPath(m.Image.ID): config, pullBlobPath(m.Image.ManifestDigest): manifest, pullBlobPath(layerDigest): compressed}
	for _, digest := range []string{m.Image.ID, layerDigest} {
		content[pullLayerLink(digest)] = []byte(digest)
	}
	content[pullManifestLink(m.Image.ManifestDigest)] = []byte(m.Image.ManifestDigest)
	paths := make([]string, 0, len(content))
	for name := range content {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	m.RegistryStore.Members = nil
	for _, name := range paths {
		body := content[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(body)
		m.RegistryStore.Members = append(m.RegistryStore.Members, Member{Path: name, SHA256: hex.EncodeToString(hash[:]), Bytes: int64(len(body))})
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	archiveHash := sha256.Sum256(archive.Bytes())
	m.RegistryStore.Bytes, m.RegistryStore.SHA256 = int64(archive.Len()), hex.EncodeToString(archiveHash[:])
	file := filepath.Join(t.TempDir(), "store.tar")
	if err := os.WriteFile(file, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := &PullBundle{Manifest: m, Store: f}
	if err := verifyPullStore(b); err != nil {
		t.Fatalf("exact store rejected: %v", err)
	}
	m.Image.Layers[0].DiffID = "sha256:" + strings.Repeat("f", 64)
	b.Manifest = m
	if err := verifyPullStore(b); err == nil {
		t.Fatal("changed original diffID accepted")
	}
}

func TestPullManifestRefusesAggregateAndOwnershipDrift(t *testing.T) {
	base := pullFixtureManifest()
	if err := verifyPullManifest(base); err != nil {
		t.Fatalf("valid structural fixture refused: %v", err)
	}
	for _, test := range []struct {
		name string
		edit func(*PullManifest)
	}{
		{"expanded-total", func(m *PullManifest) {
			m.Image.Layers = append(m.Image.Layers, PullLayer{CompressedDigest: "sha256:" + strings.Repeat("e", 64), CompressedBytes: 1, DiffID: "sha256:" + strings.Repeat("f", 64), UncompressedBytes: 2, MediaType: m.Image.Layers[0].MediaType})
			m.Image.Layers[0].UncompressedBytes = MaxPullImageBytes
		}},
		{"compressed-total", func(m *PullManifest) {
			m.Image.Layers = append(m.Image.Layers, PullLayer{CompressedDigest: "sha256:" + strings.Repeat("e", 64), CompressedBytes: 1, DiffID: "sha256:" + strings.Repeat("f", 64), UncompressedBytes: 2, MediaType: m.Image.Layers[0].MediaType})
			m.Image.Layers[0].CompressedBytes = MaxPullStoreBytes
		}},
		{"bootstrap-expanded", func(m *PullManifest) {
			m.Bootstrap.Layers = append(m.Bootstrap.Layers, BootstrapLayer{DiffID: "sha256:" + strings.Repeat("e", 64), UncompressedBytes: 1})
			m.Bootstrap.Layers[0].UncompressedBytes = MaxBootstrapExpandedBytes
		}},
		{"linked-path", func(m *PullManifest) { m.RegistryStore.Members[0].Path = "docker/registry/v2/../escape" }},
		{"duplicate-layer", func(m *PullManifest) { m.Image.Layers = append(m.Image.Layers, m.Image.Layers[0]) }},
		{"foreign-repository", func(m *PullManifest) { m.RegistryStore.Repository = "other/runtime" }},
	} {
		m := base
		m.Image.Layers = append([]PullLayer(nil), base.Image.Layers...)
		m.Bootstrap.Layers = append([]BootstrapLayer(nil), base.Bootstrap.Layers...)
		m.RegistryStore.Members = append([]Member(nil), base.RegistryStore.Members...)
		test.edit(&m)
		if err := verifyPullManifest(m); err == nil {
			t.Errorf("accepted %s", test.name)
		}
	}
}

func TestPullLayerRejectsSecondMemberAndTrailingBytes(t *testing.T) {
	plain := []byte("one exact layer")
	var buffer bytes.Buffer
	w := gzip.NewWriter(&buffer)
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := append([]byte(nil), buffer.Bytes()...)
	for _, test := range []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"exact", compressed, true},
		{"trailing", append(append([]byte(nil), compressed...), 0), false},
		{"second-member", append(append([]byte(nil), compressed...), compressed...), false},
	} {
		digest := sha256.Sum256(test.data)
		diff := sha256.Sum256(plain)
		layer := PullLayer{CompressedDigest: "sha256:" + hex.EncodeToString(digest[:]), DiffID: "sha256:" + hex.EncodeToString(diff[:]), UncompressedBytes: int64(len(plain))}
		err := verifyPullLayer(bytes.NewReader(test.data), int64(len(test.data)), layer, time.Now().Add(time.Second))
		if (err == nil) != test.valid {
			t.Errorf("%s: got %v, want valid=%v", test.name, err, test.valid)
		}
	}
}

func TestPullAllowlistBindsOnlyInstalledDigest(t *testing.T) {
	m := pullFixtureManifest()
	m.RegistryStore.Members = []Member{{Path: pullBlobPath(m.Image.ManifestDigest), Bytes: 199}}
	b := &PullBundle{Manifest: m}
	data, err := b.pullAllowlistBytes()
	if err != nil {
		t.Fatal(err)
	}
	var got pullAllowlist
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Repository != "luci/runtime-installed" || got.ManifestDigest != m.Image.ManifestDigest || got.ManifestBytes != 199 || got.ConfigDigest != m.Image.ID || len(got.Layers) != 1 || got.Layers[0].Digest != m.Image.Layers[0].CompressedDigest {
		t.Fatalf("installed allowlist drift: %+v", got)
	}
	b.Manifest.RegistryStore.Members = nil
	if _, err := b.pullAllowlistBytes(); err == nil {
		t.Fatal("missing exact manifest blob accepted")
	}
}

func TestPullStageReceiptRefusesChangedAllowlist(t *testing.T) {
	m := pullFixtureManifest()
	m.RegistryStore.Members = []Member{{Path: pullBlobPath(m.Image.ManifestDigest), Bytes: 199}}
	b := &PullBundle{Manifest: m, ManifestSHA256: strings.Repeat("b", 64)}
	allowlist, err := b.pullAllowlistBytes()
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	stage := filepath.Join(parent, "stage")
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	receipt := PullStageReceipt{Version: 1, ManifestSHA256: b.ManifestSHA256, StoreSHA256: m.RegistryStore.SHA256, Repository: m.RegistryStore.Repository, ManifestDigest: m.Image.ManifestDigest, AllowlistSHA256: sumHex(allowlist), AllowlistBytes: len(allowlist)}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "runtime-pull-stage.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Join(stage, "runtime-pull-allowlist.json")
	if err := os.WriteFile(allowlistPath, allowlist, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadPullStage(stage); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowlistPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadPullStage(stage); err == nil {
		t.Fatal("changed stage allowlist accepted")
	}
}
