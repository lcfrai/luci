package runtimeoci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	MaxPullStoreBytes         int64 = 4 << 30
	MaxPullImageBytes         int64 = 8 << 30
	MaxBootstrapBytes         int64 = 256 << 20
	MaxBootstrapExpandedBytes int64 = 512 << 20
)

type PullLayer struct {
	CompressedDigest  string `json:"compressedDigest"`
	CompressedBytes   int64  `json:"compressedBytes"`
	DiffID            string `json:"diffID"`
	UncompressedBytes int64  `json:"uncompressedBytes"`
	MediaType         string `json:"mediaType"`
}

type BootstrapLayer struct {
	DiffID            string `json:"diffID"`
	UncompressedBytes int64  `json:"uncompressedBytes"`
}

type PullManifest struct {
	Version      int    `json:"version"`
	Kind         string `json:"kind"`
	SourceCommit string `json:"sourceCommit"`
	Target       struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"target"`
	CanonicalInputAlgorithm string       `json:"canonicalInputAlgorithm"`
	CanonicalInputDigest    string       `json:"canonicalInputDigest"`
	SourceFiles             []SourceFile `json:"sourceFiles"`
	Runtime                 struct {
		User         string   `json:"user"`
		WorkingDir   string   `json:"workingDir"`
		Entrypoint   []string `json:"entrypoint"`
		ExposedPorts []string `json:"exposedPorts"`
		Harnesses    struct {
			Codex      string `json:"codex"`
			ClaudeCode string `json:"claudeCode"`
			OpenCode   string `json:"openCode"`
		} `json:"harnesses"`
	} `json:"runtime"`
	Image struct {
		ID                string      `json:"id"`
		ConfigSHA256      string      `json:"configSHA256"`
		ConfigBytes       int64       `json:"configBytes"`
		ManifestDigest    string      `json:"manifestDigest"`
		ManifestMediaType string      `json:"manifestMediaType"`
		Layers            []PullLayer `json:"layers"`
	} `json:"image"`
	RegistryStore struct {
		FileName   string   `json:"fileName"`
		SHA256     string   `json:"sha256"`
		Bytes      int64    `json:"bytes"`
		Repository string   `json:"repository"`
		Members    []Member `json:"members"`
	} `json:"registryStore"`
	Bootstrap struct {
		FileName                  string           `json:"fileName"`
		SHA256                    string           `json:"sha256"`
		Bytes                     int64            `json:"bytes"`
		ImageID                   string           `json:"imageID"`
		ConfigSHA256              string           `json:"configSHA256"`
		ConfigBytes               int64            `json:"configBytes"`
		Layers                    []BootstrapLayer `json:"layers"`
		User                      string           `json:"user"`
		WorkingDir                string           `json:"workingDir"`
		Entrypoint                []string         `json:"entrypoint"`
		ExposedPorts              []string         `json:"exposedPorts"`
		Environment               []string         `json:"environment"`
		DistributionVersion       string           `json:"distributionVersion"`
		DistributionArchiveSHA256 string           `json:"distributionArchiveSHA256"`
		DistributionBinarySHA256  string           `json:"distributionBinarySHA256"`
		DistributionLicenseSHA256 string           `json:"distributionLicenseSHA256"`
		GateSourceSHA256          string           `json:"gateSourceSHA256"`
		GateBinarySHA256          string           `json:"gateBinarySHA256"`
		BuildRecipeSHA256         string           `json:"buildRecipeSHA256"`
		BuildReceiptSHA256        string           `json:"buildReceiptSHA256"`
	} `json:"bootstrap"`
	Licenses []LicenseFile `json:"licenses"`
	Producer struct {
		OriginalBuildReceiptSHA256 string `json:"originalBuildReceiptSHA256"`
		ConversionReceiptSHA256    string `json:"conversionReceiptSHA256"`
		PullProducerReceiptSHA256  string `json:"pullProducerReceiptSHA256"`
		ReuseEvidenceSHA256        string `json:"reuseEvidenceSHA256,omitempty"`
	} `json:"producer"`
}

// PullBundle retains opened package descriptors. StageStore must be called
// only after the coordinator durably admits and owns the local staging work.
type PullBundle struct {
	Manifest            PullManifest
	Store               *os.File
	Bootstrap           *os.File
	ManifestSHA256      string
	SourceReceiptSHA256 string
}

func (b *PullBundle) Close() error {
	if b == nil {
		return nil
	}
	var err error
	if b.Store != nil {
		err = errors.Join(err, b.Store.Close())
		b.Store = nil
	}
	if b.Bootstrap != nil {
		err = errors.Join(err, b.Bootstrap.Close())
		b.Bootstrap = nil
	}
	return err
}

func verifyPullManifest(m PullManifest) error {
	if m.Version != 2 || m.Kind != "luci-runtime-registry-pull" || !commitPattern.MatchString(m.SourceCommit) || m.Target.OS != "linux" || m.Target.Architecture != "amd64" || m.CanonicalInputAlgorithm != "luci-git-blobs-v1" || !digestPattern.MatchString(m.CanonicalInputDigest) || len(m.SourceFiles) < 2 || len(m.SourceFiles) > 8192 || len(m.Licenses) < 2 || len(m.Licenses) > 4096 {
		return errors.New("runtime pull source identity invalid")
	}
	if m.Runtime.User != "dev" || m.Runtime.WorkingDir != "/home/dev" || !equalStrings(m.Runtime.Entrypoint, []string{"/usr/local/bin/entrypoint.sh"}) || !equalStrings(m.Runtime.ExposedPorts, []string{"8080/tcp"}) || m.Runtime.Harnesses.Codex != "0.160.0" || m.Runtime.Harnesses.ClaudeCode != "2.1.274" || m.Runtime.Harnesses.OpenCode != "1.18.34" {
		return errors.New("runtime pull harness config changed")
	}
	if !imagePattern.MatchString(m.Image.ID) || strings.TrimPrefix(m.Image.ID, "sha256:") != m.Image.ConfigSHA256 || m.Image.ConfigBytes < 1 || m.Image.ConfigBytes > 16<<20 || !imagePattern.MatchString(m.Image.ManifestDigest) || m.Image.ManifestMediaType != "application/vnd.docker.distribution.manifest.v2+json" || len(m.Image.Layers) < 1 || len(m.Image.Layers) > 2048 {
		return errors.New("runtime pull image config invalid")
	}
	var compressedTotal, expandedTotal int64
	seenLayer := map[string]bool{}
	for _, layer := range m.Image.Layers {
		if !imagePattern.MatchString(layer.CompressedDigest) || !imagePattern.MatchString(layer.DiffID) || seenLayer[layer.CompressedDigest] || layer.MediaType != "application/vnd.docker.image.rootfs.diff.tar.gzip" || layer.CompressedBytes < 1 || layer.CompressedBytes > MaxPullStoreBytes-compressedTotal || layer.UncompressedBytes < 1 || layer.UncompressedBytes > MaxPullImageBytes-expandedTotal {
			return errors.New("runtime pull aggregate layer bounds invalid")
		}
		seenLayer[layer.CompressedDigest] = true
		compressedTotal += layer.CompressedBytes
		expandedTotal += layer.UncompressedBytes
	}
	if m.RegistryStore.FileName != "runtime-registry-store-v2.tar" || !digestPattern.MatchString(m.RegistryStore.SHA256) || m.RegistryStore.Bytes < 1024 || m.RegistryStore.Bytes > MaxPullStoreBytes || m.RegistryStore.Repository != "luci/runtime-installed" || len(m.RegistryStore.Members) < 4 || len(m.RegistryStore.Members) > 4096 {
		return errors.New("runtime pull store identity invalid")
	}
	var memberTotal int64
	seenMember := map[string]bool{}
	for i, member := range m.RegistryStore.Members {
		if len(member.Path) > 512 || !strings.HasPrefix(member.Path, "docker/registry/v2/") || !safePullPath(member.Path) || seenMember[member.Path] || i > 0 && m.RegistryStore.Members[i-1].Path >= member.Path || !digestPattern.MatchString(member.SHA256) || member.Bytes < 1 || member.Bytes > MaxPullStoreBytes-memberTotal {
			return errors.New("runtime pull store member invalid")
		}
		seenMember[member.Path] = true
		memberTotal += member.Bytes
	}
	if m.Bootstrap.FileName != "runtime-registry-bootstrap-linux-amd64.tar" || !digestPattern.MatchString(m.Bootstrap.SHA256) || m.Bootstrap.Bytes < 1024 || m.Bootstrap.Bytes > MaxBootstrapBytes || !imagePattern.MatchString(m.Bootstrap.ImageID) || strings.TrimPrefix(m.Bootstrap.ImageID, "sha256:") != m.Bootstrap.ConfigSHA256 || m.Bootstrap.ConfigBytes < 1 || m.Bootstrap.ConfigBytes > 16<<20 || len(m.Bootstrap.Layers) < 1 || len(m.Bootstrap.Layers) > 8 || m.Bootstrap.User != "65532:65532" || m.Bootstrap.WorkingDir != "/" || !equalStrings(m.Bootstrap.Entrypoint, []string{"/luci-registry-gate"}) || !equalStrings(m.Bootstrap.ExposedPorts, []string{"5000/tcp"}) || len(m.Bootstrap.Environment) != 0 || m.Bootstrap.DistributionVersion != "3.0.0" || m.Bootstrap.DistributionArchiveSHA256 != "61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d" {
		return errors.New("runtime pull bootstrap config invalid")
	}
	var bootstrapTotal int64
	for _, layer := range m.Bootstrap.Layers {
		if !imagePattern.MatchString(layer.DiffID) || layer.UncompressedBytes < 1 || layer.UncompressedBytes > MaxBootstrapExpandedBytes-bootstrapTotal {
			return errors.New("runtime pull bootstrap expanded size invalid")
		}
		bootstrapTotal += layer.UncompressedBytes
	}
	for _, hash := range []string{m.Bootstrap.DistributionBinarySHA256, m.Bootstrap.DistributionLicenseSHA256, m.Bootstrap.GateSourceSHA256, m.Bootstrap.GateBinarySHA256, m.Bootstrap.BuildRecipeSHA256, m.Bootstrap.BuildReceiptSHA256, m.Producer.OriginalBuildReceiptSHA256, m.Producer.ConversionReceiptSHA256, m.Producer.PullProducerReceiptSHA256} {
		if !digestPattern.MatchString(hash) {
			return errors.New("runtime pull producer hash missing")
		}
	}
	if m.Producer.ReuseEvidenceSHA256 != "" && !digestPattern.MatchString(m.Producer.ReuseEvidenceSHA256) {
		return errors.New("runtime pull reuse hash invalid")
	}
	return nil
}

func safePullPath(name string) bool {
	if len(name) == 0 || len(name) > 512 || strings.HasPrefix(name, "/") || filepath.ToSlash(filepath.Clean(name)) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' && char < 'A' || char > 'Z' && char < 'a' || char > 'z' && char != '-' && char != '_' && char != '.' {
				return false
			}
		}
	}
	return true
}

func openPullFile(root, name string, size int64) (*os.File, error) {
	if !safePath(name) || size < 1 || rejectLinkedPath(root, name) != nil {
		return nil, errors.New("runtime pull package path unavailable")
	}
	path := filepath.Join(root, name)
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != size {
		return nil, errors.New("runtime pull package size changed")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = f.Close()
		return nil, errors.New("runtime pull package file changed")
	}
	return f, nil
}

func verifyPullSources(root string, m PullManifest) error {
	var fake Manifest
	fake.CanonicalInputDigest = m.CanonicalInputDigest
	fake.SourceFiles = m.SourceFiles
	fake.Licenses = make([]LicenseFile, 0, len(m.Licenses))
	seenLicenses := map[string]bool{}
	for index, license := range m.Licenses {
		if !strings.HasPrefix(license.Path, "licenses/runtime-image/") && !strings.HasPrefix(license.Path, "licenses/runtime-bootstrap/") || seenLicenses[license.Path] || index > 0 && m.Licenses[index-1].Path >= license.Path {
			return errors.New("runtime pull licence inventory changed")
		}
		seenLicenses[license.Path] = true
		if strings.HasPrefix(license.Path, "licenses/runtime-image/") {
			fake.Licenses = append(fake.Licenses, license)
		}
	}
	for _, source := range m.SourceFiles {
		if source.Path == "images/sandbox/Dockerfile" {
			fake.Producer.DockerfileSHA256 = source.SHA256
		}
	}
	if err := verifySources(root, fake); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, license := range m.Licenses {
		if !strings.HasPrefix(license.Path, "licenses/runtime-bootstrap/") {
			continue
		}
		if !safePath(license.Path) || seen[license.Path] || license.Bytes < 1 || license.Bytes > 16<<20 || !digestPattern.MatchString(license.SHA256) || license.Component == "" || license.Version == "" || license.Grant == "" || license.Provenance == "" {
			return errors.New("runtime bootstrap licence inventory invalid")
		}
		data, err := readRegular(filepath.Dir(root), license.Path, 16<<20)
		if err != nil || int64(len(data)) != license.Bytes || sumHex(data) != license.SHA256 {
			return errors.New("runtime bootstrap licence changed")
		}
		seen[license.Path] = true
	}
	if len(seen) == 0 {
		return errors.New("runtime bootstrap licence missing")
	}
	return filepath.WalkDir(filepath.Join(filepath.Dir(root), "licenses", "runtime-bootstrap"), func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(filepath.Dir(root), name)
		if err != nil || !seen[filepath.ToSlash(relative)] || !entry.Type().IsRegular() {
			return errors.New("runtime bootstrap licence has extra or linked member")
		}
		return nil
	})
}

func verifyPullReuse(root string, m PullManifest, sourceHash, originalReceiptHash string) error {
	if m.Producer.ReuseEvidenceSHA256 == "" {
		return nil
	}
	data, err := readRegular(root, "runtime-source-reuse-v1.json", maxMetadata)
	if err != nil || sumHex(data) != m.Producer.ReuseEvidenceSHA256 {
		return errors.New("runtime pull reuse receipt changed")
	}
	var reuse reuseReceipt
	if err := decodeStrict(data, &reuse); err != nil || reuse.Version != 1 || reuse.Kind != "luci-runtime-equivalent-input-reuse" || reuse.InstalledSourceCommit != m.SourceCommit || reuse.InstalledSourceReceiptSHA256 != sourceHash || reuse.CanonicalInputAlgorithm != m.CanonicalInputAlgorithm || reuse.CanonicalInputDigest != m.CanonicalInputDigest || reuse.ImageID != m.Image.ID {
		return errors.New("runtime pull reuse identity changed")
	}
	priorBytes, err := readRegular(root, "runtime-prior-manifest-v1.json", maxMetadata)
	if err != nil || sumHex(priorBytes) != reuse.PriorManifestSHA256 {
		return errors.New("runtime pull prior manifest changed")
	}
	var prior Manifest
	if err := decodeStrict(priorBytes, &prior); err != nil || verifyManifest(prior) != nil || prior.SourceCommit != reuse.PriorBuildSourceCommit || prior.CanonicalInputDigest != m.CanonicalInputDigest || !reflect.DeepEqual(prior.SourceFiles, m.SourceFiles) || prior.Image.ID != m.Image.ID || prior.Archive.SHA256 != reuse.ArchiveSHA256 || prior.Archive.Bytes != reuse.ArchiveBytes || prior.Producer.BuildReceiptSHA256 != originalReceiptHash {
		return errors.New("runtime pull prior source or image changed")
	}
	if prior.Runtime.User != m.Runtime.User || prior.Runtime.WorkingDir != m.Runtime.WorkingDir || !equalStrings(prior.Runtime.Entrypoint, m.Runtime.Entrypoint) || !equalStrings(prior.Runtime.ExposedPorts, m.Runtime.ExposedPorts) || prior.Runtime.Harnesses.Codex != m.Runtime.Harnesses.Codex || prior.Runtime.Harnesses.ClaudeCode != m.Runtime.Harnesses.ClaudeCode || prior.Runtime.Harnesses.OpenCode != m.Runtime.Harnesses.OpenCode || len(prior.Image.DiffIDs) != len(m.Image.Layers) {
		return errors.New("runtime pull prior runtime config changed")
	}
	for index, diffID := range prior.Image.DiffIDs {
		if diffID != m.Image.Layers[index].DiffID {
			return errors.New("runtime pull prior layer diffID changed")
		}
	}
	var currentImageLicenses []LicenseFile
	for _, license := range m.Licenses {
		if strings.HasPrefix(license.Path, "licenses/runtime-image/") {
			currentImageLicenses = append(currentImageLicenses, license)
		}
	}
	if !reflect.DeepEqual(prior.Licenses, currentImageLicenses) {
		return errors.New("runtime pull prior licences changed")
	}
	priorReceipt, err := readRegular(root, "runtime-prior-build-receipt.json", maxMetadata)
	if err != nil || sumHex(priorReceipt) != reuse.PriorBuildReceiptSHA256 || sumHex(priorReceipt) != originalReceiptHash {
		return errors.New("runtime pull prior build receipt changed")
	}
	currentReceipt, err := readRegular(root, "runtime-build-receipt.json", maxMetadata)
	if err != nil || !bytes.Equal(priorReceipt, currentReceipt) {
		return errors.New("runtime pull reused build receipt changed")
	}
	return nil
}

func pullHashFile(f *os.File, size int64, expected string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, size+1))
	if err != nil || n != size || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("runtime pull package bytes changed")
	}
	_, err = f.Seek(0, io.SeekStart)
	return err
}

func pullSourceReceipt(root string, m PullManifest) (string, error) {
	data, err := readRegular(root, "source-receipt.json", 4096)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := decodeStrict(data, &fields); err != nil || len(fields) != 3 || !bytes.Equal(bytes.TrimSpace(fields["sourceDirty"]), []byte("false")) {
		return "", errors.New("runtime pull source receipt invalid")
	}
	var source sourceReceipt
	if err := decodeStrict(data, &source); err != nil || source.Version != 1 || source.SourceDirty || source.SourceCommit != m.SourceCommit {
		return "", errors.New("runtime pull source receipt changed")
	}
	return sumHex(data), nil
}

// OpenPull is a metadata and held-file boundary. It performs no Docker work.
// Full tar, blob and bootstrap checks are carried out by VerifyPullPayload.
func OpenPull(root string) (_ *PullBundle, err error) {
	return OpenPullContext(context.Background(), root)
}

func OpenPullContext(ctx context.Context, root string) (_ *PullBundle, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manifestData, err := readRegular(root, "runtime-oci-pull-manifest-v2.json", maxMetadata)
	if err != nil {
		return nil, err
	}
	var manifest PullManifest
	if err := decodeStrict(manifestData, &manifest); err != nil {
		return nil, err
	}
	if err := verifyPullManifest(manifest); err != nil {
		return nil, err
	}
	sourceHash, err := pullSourceReceipt(root, manifest)
	if err != nil {
		return nil, err
	}
	if err := verifyPullSources(root, manifest); err != nil {
		return nil, err
	}
	originalReceipt, err := readRegular(root, "runtime-build-receipt.json", maxMetadata)
	if err != nil || sumHex(originalReceipt) != manifest.Producer.OriginalBuildReceiptSHA256 {
		return nil, errors.New("runtime pull original build receipt changed")
	}
	var original buildReceipt
	if err := decodeStrict(originalReceipt, &original); err != nil || verifyPullOriginalReceipt(manifest, original) != nil {
		return nil, errors.New("runtime pull original build facts invalid")
	}
	if original.BuildSourceCommit != manifest.SourceCommit && manifest.Producer.ReuseEvidenceSHA256 == "" || original.BuildSourceCommit == manifest.SourceCommit && manifest.Producer.ReuseEvidenceSHA256 != "" {
		return nil, errors.New("runtime pull source reuse mismatch")
	}
	if err := verifyPullReuse(root, manifest, sourceHash, manifest.Producer.OriginalBuildReceiptSHA256); err != nil {
		return nil, err
	}
	// Receipt verification is a separate strict layer so fields cannot be
	// treated as interchangeable with this manifest or a successful CLI exit.
	if err := verifyPullReceipts(root, manifest); err != nil {
		return nil, err
	}
	store, err := openPullFile(root, manifest.RegistryStore.FileName, manifest.RegistryStore.Bytes)
	if err != nil {
		return nil, err
	}
	bundle := &PullBundle{Manifest: manifest, Store: store, ManifestSHA256: sumHex(manifestData), SourceReceiptSHA256: sourceHash}
	defer func() {
		if err != nil {
			_ = bundle.Close()
		}
	}()
	bundle.Bootstrap, err = openPullFile(root, manifest.Bootstrap.FileName, manifest.Bootstrap.Bytes)
	if err != nil {
		return nil, err
	}
	if err = bundle.VerifyPullPayloadContext(ctx); err != nil {
		return nil, err
	}
	return bundle, nil
}
