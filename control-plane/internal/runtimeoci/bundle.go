// Package runtimeoci verifies the installed, source-bound runtime distribution
// before an ordinary local Docker effect. It never follows package symlinks or
// buffers the potentially multi-gigabyte image archive.
package runtimeoci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const MaxArchiveBytes int64 = 8 << 30
const maxMetadata int64 = 1 << 20
const maxInputs int64 = 64 << 20

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var memberPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,256}$`)
var imagePattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type Member struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type SourceFile struct {
	Member
	Mode string `json:"mode"`
}
type LicenseFile struct {
	Member
	Component  string `json:"component"`
	Version    string `json:"version"`
	Grant      string `json:"grant"`
	Provenance string `json:"provenance"`
}
type Manifest struct {
	Version                 int    `json:"version"`
	Kind                    string `json:"kind"`
	SourceCommit            string `json:"sourceCommit"`
	CanonicalInputAlgorithm string `json:"canonicalInputAlgorithm"`
	CanonicalInputDigest    string `json:"canonicalInputDigest"`
	Target                  struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"target"`
	Archive struct {
		FileName string   `json:"fileName"`
		SHA256   string   `json:"sha256"`
		Bytes    int64    `json:"bytes"`
		Members  []Member `json:"members"`
	} `json:"archive"`
	Image struct {
		ID         string   `json:"id"`
		ConfigPath string   `json:"configPath"`
		LayerPaths []string `json:"layerPaths"`
		DiffIDs    []string `json:"diffIDs"`
		RepoTags   []string `json:"repoTags"`
	} `json:"image"`
	Runtime struct {
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
	SourceFiles []SourceFile  `json:"sourceFiles"`
	Licenses    []LicenseFile `json:"licenses"`
	Producer    struct {
		DockerfileSHA256    string `json:"dockerfileSHA256"`
		BuildReceiptSHA256  string `json:"buildReceiptSHA256"`
		BuildSourceCommit   string `json:"buildSourceCommit"`
		ReuseEvidenceSHA256 string `json:"reuseEvidenceSHA256,omitempty"`
	} `json:"producer"`
}
type buildReceipt struct {
	Version                 int    `json:"version"`
	Kind                    string `json:"kind"`
	BuildSourceCommit       string `json:"buildSourceCommit"`
	CanonicalInputAlgorithm string `json:"canonicalInputAlgorithm"`
	CanonicalInputDigest    string `json:"canonicalInputDigest"`
	Target                  struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"target"`
	GeneratedContextSHA256 string `json:"generatedContextSHA256"`
	BaseImages             []struct {
		Reference  string `json:"reference"`
		ImageID    string `json:"imageID"`
		RepoDigest string `json:"repoDigest"`
	} `json:"baseImages"`
	Builder struct {
		Backend                  string   `json:"backend"`
		Version                  string   `json:"version"`
		Argv                     []string `json:"argv"`
		MemoryBytes              int64    `json:"memoryBytes"`
		CPUQuotaMicros           int64    `json:"cpuQuotaMicros"`
		ExitCode                 int      `json:"exitCode"`
		StdoutSHA256             string   `json:"stdoutSHA256"`
		StderrSHA256             string   `json:"stderrSHA256"`
		CacheAssistance          bool     `json:"cacheAssistance"`
		CacheAssistanceLogSHA256 string   `json:"cacheAssistanceLogSHA256"`
	} `json:"builder"`
	ImageID string `json:"imageID"`
	Archive struct {
		SHA256 string `json:"sha256"`
		Bytes  int64  `json:"bytes"`
	} `json:"archive"`
	Probes []struct {
		Tool            string   `json:"tool"`
		Argv            []string `json:"argv"`
		ExitCode        int      `json:"exitCode"`
		ReportedVersion string   `json:"reportedVersion"`
		StdoutSHA256    string   `json:"stdoutSHA256"`
		StderrSHA256    string   `json:"stderrSHA256"`
	} `json:"probes"`
}
type sourceReceipt struct {
	Version      int    `json:"version"`
	SourceCommit string `json:"sourceCommit"`
	SourceDirty  bool   `json:"sourceDirty"`
}

// Bundle owns the already-verified descriptor. The caller must close it. Seek
// to zero immediately before Docker ImageLoad; never reopen by pathname.
type Bundle struct {
	Manifest            Manifest
	Archive             *os.File
	SourceReceiptSHA256 string
}

func (b *Bundle) Close() error {
	if b == nil || b.Archive == nil {
		return nil
	}
	return b.Archive.Close()
}

func safePath(name string) bool {
	if !memberPattern.MatchString(name) || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." || part == "" {
			return false
		}
	}
	return true
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func sumHex(data []byte) string { d := sha256.Sum256(data); return hex.EncodeToString(d[:]) }
func readRegular(root, name string, limit int64) ([]byte, error) {
	if !safePath(name) {
		return nil, errors.New("unsafe packaged path")
	}
	filename := filepath.Join(root, filepath.FromSlash(name))
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		return nil, errors.New("packaged file missing, linked or oversized")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("packaged file changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) != info.Size() || int64(len(data)) > limit {
		return nil, errors.New("packaged file changed or exceeded bound")
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(info, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, errors.New("packaged file changed")
	}
	return data, nil
}
func decodeStrict(data []byte, out any) error {
	// Decoder.DisallowUnknownFields does not reject duplicate keys. The token
	// walk rejects duplicates at every object depth before typed decoding.
	if err := uniqueJSON(json.NewDecoder(strings.NewReader(string(data)))); err != nil {
		return err
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}
func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	switch v := token.(type) {
	case json.Delim:
		switch v {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate or invalid JSON key")
				}
				seen[name] = true
				if err = uniqueJSON(d); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err = uniqueJSON(d); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	return nil
}

func verifyManifest(m Manifest) error {
	if m.Version != 1 || m.Kind != "luci-runtime-docker-save" || !commitPattern.MatchString(m.SourceCommit) || m.CanonicalInputAlgorithm != "luci-git-blobs-v1" || !digestPattern.MatchString(m.CanonicalInputDigest) || m.Target.OS != "linux" || m.Target.Architecture != "amd64" || m.Archive.FileName != "runtime-image-linux-amd64.tar" || m.Archive.Bytes < 1024 || m.Archive.Bytes > MaxArchiveBytes || !digestPattern.MatchString(m.Archive.SHA256) || !imagePattern.MatchString(m.Image.ID) || m.Image.ConfigPath != strings.TrimPrefix(m.Image.ID, "sha256:")+".json" || len(m.Image.RepoTags) != 0 || len(m.Image.LayerPaths) < 1 || len(m.Image.LayerPaths) > 2048 || len(m.Image.LayerPaths) != len(m.Image.DiffIDs) || len(m.SourceFiles) < 2 || len(m.SourceFiles) > 8192 || len(m.Licenses) < 1 || len(m.Licenses) > 4096 || !digestPattern.MatchString(m.Producer.DockerfileSHA256) || !digestPattern.MatchString(m.Producer.BuildReceiptSHA256) || !commitPattern.MatchString(m.Producer.BuildSourceCommit) {
		return errors.New("runtime manifest identity or bounds invalid")
	}
	if m.Producer.ReuseEvidenceSHA256 != "" || m.Producer.BuildSourceCommit != m.SourceCommit {
		return errors.New("runtime source reuse requires separately verified prior package evidence")
	}
	if m.Runtime.User != "dev" || m.Runtime.WorkingDir != "/home/dev" || !equalStrings(m.Runtime.Entrypoint, []string{"/usr/local/bin/entrypoint.sh"}) || !equalStrings(m.Runtime.ExposedPorts, []string{"8080/tcp"}) || m.Runtime.Harnesses.Codex != "0.160.0" || m.Runtime.Harnesses.ClaudeCode != "2.1.274" || m.Runtime.Harnesses.OpenCode != "1.18.34" {
		return errors.New("runtime config differs from pinned harness contract")
	}
	paths := map[string]bool{}
	for i, v := range m.Archive.Members {
		if !safePath(v.Path) || v.Bytes < 1 || !digestPattern.MatchString(v.SHA256) || paths[v.Path] || i > 0 && m.Archive.Members[i-1].Path >= v.Path {
			return errors.New("runtime archive inventory invalid")
		}
		paths[v.Path] = true
	}
	if len(paths) != len(m.Image.LayerPaths)+2 || !paths["manifest.json"] || !paths[m.Image.ConfigPath] {
		return errors.New("runtime archive member count invalid")
	}
	for i, layer := range m.Image.LayerPaths {
		if !safePath(layer) || !strings.HasSuffix(layer, "/layer.tar") || paths[layer] == false || !imagePattern.MatchString(m.Image.DiffIDs[i]) {
			return errors.New("runtime layer inventory invalid")
		}
		for _, v := range m.Archive.Members {
			if v.Path == layer && v.SHA256 != strings.TrimPrefix(m.Image.DiffIDs[i], "sha256:") {
				return errors.New("runtime uncompressed layer digest changed")
			}
		}
	}
	return nil
}

func verifySources(root string, m Manifest) error {
	seen := map[string]bool{}
	total := int64(0)
	h := sha256.New()
	for i, v := range m.SourceFiles {
		if !safePath(v.Path) || v.Mode != "0644" || v.Bytes < 0 || !digestPattern.MatchString(v.SHA256) || seen[v.Path] || i > 0 && m.SourceFiles[i-1].Path >= v.Path {
			return errors.New("runtime source inventory invalid")
		}
		if v.Path != "sandboxd/go.mod" && v.Path != "sandboxd/go.sum" && v.Path != "images/sandbox/Dockerfile" && v.Path != "images/sandbox/entrypoint.sh" && !strings.HasPrefix(v.Path, "sandboxd/cmd/") && !strings.HasPrefix(v.Path, "sandboxd/internal/") && !strings.HasPrefix(v.Path, "sandboxd/vendor/") {
			return errors.New("unapproved runtime source file")
		}
		data, err := readRegular(root, v.Path, maxInputs-total)
		if err != nil || int64(len(data)) != v.Bytes || sumHex(data) != v.SHA256 {
			return errors.New("runtime source bytes changed")
		}
		total += v.Bytes
		seen[v.Path] = true
		_, _ = io.WriteString(h, v.Path+"\x00"+v.Mode+"\x00"+strconv.FormatInt(v.Bytes, 10)+"\x00"+v.SHA256+"\n")
	}
	for _, required := range []string{"sandboxd/go.mod", "sandboxd/go.sum", "sandboxd/vendor/modules.txt", "images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh"} {
		if !seen[required] {
			return errors.New("runtime source incomplete")
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != m.CanonicalInputDigest {
		return errors.New("runtime canonical input digest changed")
	}
	for _, tree := range []string{"sandboxd", "images/sandbox"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(tree)), func(name string, entry os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.IsDir() {
				return nil
			}
			relative, e := filepath.Rel(root, name)
			if e != nil || !seen[filepath.ToSlash(relative)] || entry.Type()&os.ModeSymlink != 0 {
				return errors.New("runtime source has unexpected or linked member")
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for i, v := range m.Licenses {
		if !strings.HasPrefix(v.Path, "licenses/") || !safePath(v.Path) || v.Bytes < 1 || !digestPattern.MatchString(v.SHA256) || v.Component == "" || v.Version == "" || v.Grant == "" || v.Provenance == "" || i > 0 && m.Licenses[i-1].Path >= v.Path {
			return errors.New("runtime licence inventory invalid")
		}
		data, e := readRegular(filepath.Dir(root), v.Path, 16<<20)
		if e != nil || int64(len(data)) != v.Bytes || sumHex(data) != v.SHA256 {
			return errors.New("runtime licence bytes changed")
		}
	}
	if !seen["images/sandbox/Dockerfile"] {
		return errors.New("runtime Dockerfile missing")
	}
	for _, v := range m.SourceFiles {
		if v.Path == "images/sandbox/Dockerfile" && v.SHA256 != m.Producer.DockerfileSHA256 {
			return errors.New("runtime Dockerfile digest changed")
		}
	}
	return nil
}

// Open validates metadata, source and licences, then scans the entire tar once
// through its held descriptor. No Docker call should precede a successful Open.
func Open(root string) (*Bundle, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("runtime package root unavailable")
	}
	manifestBytes, err := readRegular(root, "runtime-oci-manifest-v1.json", maxMetadata)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err = decodeStrict(manifestBytes, &m); err != nil {
		return nil, err
	}
	if err = verifyManifest(m); err != nil {
		return nil, err
	}
	sourceBytes, err := readRegular(root, "source-receipt.json", 4096)
	if err != nil {
		return nil, err
	}
	var source sourceReceipt
	if err = decodeStrict(sourceBytes, &source); err != nil || source.Version != 1 || source.SourceDirty || source.SourceCommit != m.SourceCommit {
		return nil, errors.New("installed runtime source receipt changed")
	}
	receiptBytes, err := readRegular(root, "runtime-build-receipt.json", maxMetadata)
	if err != nil || sumHex(receiptBytes) != m.Producer.BuildReceiptSHA256 {
		return nil, errors.New("runtime producer receipt changed")
	}
	var receipt buildReceipt
	if err = decodeStrict(receiptBytes, &receipt); err != nil || receipt.Version != 1 || receipt.Kind != "luci-runtime-producer-build" || receipt.BuildSourceCommit != m.SourceCommit || receipt.CanonicalInputAlgorithm != m.CanonicalInputAlgorithm || receipt.CanonicalInputDigest != m.CanonicalInputDigest || receipt.Target.OS != "linux" || receipt.Target.Architecture != "amd64" || receipt.ImageID != m.Image.ID || receipt.Archive.SHA256 != m.Archive.SHA256 || receipt.Archive.Bytes != m.Archive.Bytes || receipt.Builder.ExitCode != 0 || receipt.Builder.Backend != "docker-builder-v1" || !digestPattern.MatchString(receipt.GeneratedContextSHA256) || len(receipt.BaseImages) != 2 || len(receipt.Probes) != 4 {
		return nil, errors.New("runtime producer result is invalid or mismatched")
	}
	if receipt.Builder.Version == "" || len(receipt.Builder.Argv) < 3 || len(receipt.Builder.Argv) > 64 || receipt.Builder.MemoryBytes < 0 || receipt.Builder.CPUQuotaMicros < 0 || !digestPattern.MatchString(receipt.Builder.StdoutSHA256) || !digestPattern.MatchString(receipt.Builder.StderrSHA256) || !digestPattern.MatchString(receipt.Builder.CacheAssistanceLogSHA256) || !receipt.Builder.CacheAssistance && receipt.Builder.CacheAssistanceLogSHA256 != sumHex(nil) {
		return nil, errors.New("runtime producer builder receipt is invalid")
	}
	baseRefs := map[string]bool{"golang:1.26-bookworm": false, "node:24-bookworm-slim": false}
	for _, base := range receipt.BaseImages {
		if _, ok := baseRefs[base.Reference]; !ok || baseRefs[base.Reference] || !imagePattern.MatchString(base.ImageID) || !strings.Contains(base.RepoDigest, "@sha256:") || !digestPattern.MatchString(base.RepoDigest[strings.LastIndex(base.RepoDigest, "@sha256:")+8:]) {
			return nil, errors.New("runtime producer base receipt is invalid")
		}
		baseRefs[base.Reference] = true
	}
	probeVersions := map[string]string{"sandboxd": "0.5.0", "codex": m.Runtime.Harnesses.Codex, "claudeCode": m.Runtime.Harnesses.ClaudeCode, "openCode": m.Runtime.Harnesses.OpenCode}
	for _, probe := range receipt.Probes {
		version, ok := probeVersions[probe.Tool]
		if !ok || version == "" || probe.ReportedVersion != version || probe.ExitCode != 0 || len(probe.Argv) < 1 || len(probe.Argv) > 16 || !digestPattern.MatchString(probe.StdoutSHA256) || !digestPattern.MatchString(probe.StderrSHA256) {
			return nil, errors.New("runtime harness probe receipt is invalid")
		}
		delete(probeVersions, probe.Tool)
	}
	if len(probeVersions) != 0 {
		return nil, errors.New("runtime harness probe inventory is incomplete")
	}
	if err = verifySources(root, m); err != nil {
		return nil, err
	}
	archivePath := filepath.Join(root, m.Archive.FileName)
	before, err := os.Lstat(archivePath)
	if err != nil || !before.Mode().IsRegular() || before.Size() != m.Archive.Bytes {
		return nil, errors.New("runtime archive unavailable or changed")
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		f.Close()
		return nil, errors.New("runtime archive changed")
	}
	b := &Bundle{Manifest: m, Archive: f, SourceReceiptSHA256: sumHex(sourceBytes)}
	if err = b.verifyArchive(); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (b *Bundle) verifyArchive() error {
	if _, err := b.Archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	whole := sha256.New()
	bounded := io.LimitReader(b.Archive, b.Manifest.Archive.Bytes+1)
	tee := io.TeeReader(bounded, whole)
	reader := tar.NewReader(tee)
	var configBytes, manifestBytes []byte
	for i, want := range b.Manifest.Archive.Members {
		h, err := reader.Next()
		if err != nil || h.Name != want.Path || !safePath(h.Name) || h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA || h.Size != want.Bytes || h.Linkname != "" || h.PAXRecords != nil {
			return fmt.Errorf("runtime archive member %d changed", i)
		}
		hash := sha256.New()
		var capture io.Writer = hash
		if h.Name == b.Manifest.Image.ConfigPath || h.Name == "manifest.json" {
			if h.Size > maxMetadata {
				return errors.New("runtime archive metadata oversized")
			}
			var buf strings.Builder
			capture = io.MultiWriter(hash, &buf)
			_, err = io.CopyN(capture, reader, h.Size)
			if err == nil {
				if h.Name == "manifest.json" {
					manifestBytes = []byte(buf.String())
				} else {
					configBytes = []byte(buf.String())
				}
			}
		} else {
			_, err = io.CopyN(capture, reader, h.Size)
		}
		if err != nil || hex.EncodeToString(hash.Sum(nil)) != want.SHA256 {
			return errors.New("runtime archive member digest changed")
		}
	}
	if _, err := reader.Next(); err != io.EOF {
		return errors.New("runtime archive has extra members")
	}
	count, err := io.Copy(io.Discard, tee)
	if err != nil || count < 0 {
		return errors.New("runtime archive cannot be fully read")
	}
	info, err := b.Archive.Stat()
	if err != nil || info.Size() != b.Manifest.Archive.Bytes || hex.EncodeToString(whole.Sum(nil)) != b.Manifest.Archive.SHA256 {
		return errors.New("runtime archive bytes changed")
	}
	if err = verifyImageMetadata(b.Manifest, configBytes, manifestBytes); err != nil {
		return err
	}
	_, err = b.Archive.Seek(0, io.SeekStart)
	return err
}

func verifyImageMetadata(m Manifest, configBytes, manifestBytes []byte) error {
	if sumHex(configBytes) != strings.TrimPrefix(m.Image.ID, "sha256:") {
		return errors.New("runtime image config ID changed")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(configBytes))); err != nil {
		return errors.New("runtime image config has ambiguous JSON")
	}
	var saved []struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
		Layers   []string `json:"Layers"`
	}
	if err := decodeStrict(manifestBytes, &saved); err != nil || len(saved) != 1 || saved[0].Config != m.Image.ConfigPath || len(saved[0].RepoTags) != 0 || !equalStrings(saved[0].Layers, m.Image.LayerPaths) {
		return errors.New("runtime saved image manifest changed or is tagged")
	}
	var config struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Config       struct {
			User         string                     `json:"User"`
			WorkingDir   string                     `json:"WorkingDir"`
			Entrypoint   []string                   `json:"Entrypoint"`
			ExposedPorts map[string]json.RawMessage `json:"ExposedPorts"`
			Volumes      map[string]json.RawMessage `json:"Volumes"`
			OnBuild      []string                   `json:"OnBuild"`
			Labels       map[string]string          `json:"Labels"`
		} `json:"config"`
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil || config.OS != "linux" || config.Architecture != "amd64" || config.Config.User != m.Runtime.User || config.Config.WorkingDir != m.Runtime.WorkingDir || !equalStrings(config.Config.Entrypoint, m.Runtime.Entrypoint) || len(config.Config.ExposedPorts) != 1 || config.Config.ExposedPorts["8080/tcp"] == nil || len(config.Config.Volumes) != 0 || len(config.Config.OnBuild) != 0 || !equalStrings(config.RootFS.DiffIDs, m.Image.DiffIDs) {
		return errors.New("runtime image configuration changed")
	}
	for key := range config.Config.Labels {
		if strings.HasPrefix(key, "luci.runtime-image") || key == "luci.instance" {
			return errors.New("shared runtime payload has execution ownership labels")
		}
	}
	return nil
}
