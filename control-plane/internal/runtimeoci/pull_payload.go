package runtimeoci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const pullLayerDeadline = 10 * time.Minute
const pullTotalDeadline = 30 * time.Minute
const bootstrapDeadline = 5 * time.Minute

func pullBlobPath(digest string) string {
	value := strings.TrimPrefix(digest, "sha256:")
	return "docker/registry/v2/blobs/sha256/" + value[:2] + "/" + value + "/data"
}

func pullLayerLink(digest string) string {
	value := strings.TrimPrefix(digest, "sha256:")
	return "docker/registry/v2/repositories/luci/runtime-installed/_layers/sha256/" + value + "/link"
}

func pullManifestLink(digest string) string {
	value := strings.TrimPrefix(digest, "sha256:")
	return "docker/registry/v2/repositories/luci/runtime-installed/_manifests/revisions/sha256/" + value + "/link"
}

type pullExpectedMember struct {
	bytes int64
	hash  string
	role  string
	value string
}

// gzip.Reader leaves bytes after a gzip member unread only when its source
// implements io.ByteReader. This prevents a layer with an unverified second
// gzip member or trailer from passing a first-member diffID check.
type pullByteReader struct{ io.Reader }

func (r pullByteReader) ReadByte() (byte, error) {
	var one [1]byte
	_, err := io.ReadFull(r.Reader, one[:])
	return one[0], err
}

func expectedPullMembers(m PullManifest) map[string]pullExpectedMember {
	result := make(map[string]pullExpectedMember, len(m.Image.Layers)*2+4)
	addBlob := func(digest string, size int64, role string) {
		result[pullBlobPath(digest)] = pullExpectedMember{size, strings.TrimPrefix(digest, "sha256:"), role, digest}
	}
	addLink := func(path, digest string) {
		result[path] = pullExpectedMember{int64(len(digest)), sumHex([]byte(digest)), "link", digest}
	}
	addBlob(m.Image.ManifestDigest, 0, "manifest") // size checked against inventory and bounded below.
	addLink(pullManifestLink(m.Image.ManifestDigest), m.Image.ManifestDigest)
	addBlob(m.Image.ID, m.Image.ConfigBytes, "config")
	addLink(pullLayerLink(m.Image.ID), m.Image.ID)
	for _, layer := range m.Image.Layers {
		addBlob(layer.CompressedDigest, layer.CompressedBytes, "layer")
		addLink(pullLayerLink(layer.CompressedDigest), layer.CompressedDigest)
	}
	return result
}

func copyPullBound(dst io.Writer, src io.Reader, max int64, deadline time.Time) (int64, error) {
	var total int64
	buf := make([]byte, 64<<10)
	for {
		if time.Now().After(deadline) {
			return total, errors.New("runtime pull verification timed out")
		}
		n, err := src.Read(buf)
		if n > 0 {
			if int64(n) > max-total {
				return total, errors.New("runtime pull expanded bytes exceeded bound")
			}
			written, e := dst.Write(buf[:n])
			if e != nil || written != n {
				return total, errors.New("runtime pull verification write failed")
			}
			total += int64(n)
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrNoProgress
		}
	}
}

func verifyPullLayer(reader io.Reader, compressedSize int64, want PullLayer, deadline time.Time) error {
	limited := &io.LimitedReader{R: reader, N: compressedSize}
	compressedHash := sha256.New()
	tee := io.TeeReader(limited, compressedHash)
	decoded, err := gzip.NewReader(pullByteReader{tee})
	if err != nil {
		return err
	}
	decoded.Multistream(false)
	uncompressedHash := sha256.New()
	n, err := copyPullBound(uncompressedHash, decoded, want.UncompressedBytes, deadline)
	closeErr := decoded.Close()
	if err != nil || closeErr != nil || n != want.UncompressedBytes {
		return errors.New("runtime pull uncompressed layer changed")
	}
	if limited.N != 0 || hex.EncodeToString(compressedHash.Sum(nil)) != strings.TrimPrefix(want.CompressedDigest, "sha256:") {
		return errors.New("runtime pull compressed layer changed")
	}
	if hex.EncodeToString(uncompressedHash.Sum(nil)) != strings.TrimPrefix(want.DiffID, "sha256:") {
		return errors.New("runtime pull layer diffID changed")
	}
	return nil
}

func verifyPullImageConfig(m PullManifest, data []byte) error {
	if int64(len(data)) != m.Image.ConfigBytes || sumHex(data) != m.Image.ConfigSHA256 {
		return errors.New("runtime pull original config bytes changed")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return err
	}
	var config struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Config       struct {
			User         string            `json:"User"`
			WorkingDir   string            `json:"WorkingDir"`
			Entrypoint   []string          `json:"Entrypoint"`
			ExposedPorts map[string]any    `json:"ExposedPorts"`
			Labels       map[string]string `json:"Labels"`
		} `json:"config"`
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	if config.OS != "linux" || config.Architecture != "amd64" || config.Config.User != m.Runtime.User || config.Config.WorkingDir != m.Runtime.WorkingDir || !equalStrings(config.Config.Entrypoint, m.Runtime.Entrypoint) || len(config.Config.ExposedPorts) != 1 || config.Config.ExposedPorts["8080/tcp"] == nil || len(config.RootFS.DiffIDs) != len(m.Image.Layers) {
		return errors.New("runtime pull original config differs from contract")
	}
	for i, layer := range m.Image.Layers {
		if config.RootFS.DiffIDs[i] != layer.DiffID {
			return errors.New("runtime pull config diffID changed")
		}
	}
	for label := range config.Config.Labels {
		if strings.HasPrefix(label, "luci.runtime-image") || label == "luci.instance" {
			return errors.New("runtime pull shared image has allocation label")
		}
	}
	return nil
}

func verifyPullDockerManifest(m PullManifest, data []byte) error {
	if len(data) == 0 || int64(len(data)) > maxMetadata || sumHex(data) != strings.TrimPrefix(m.Image.ManifestDigest, "sha256:") {
		return errors.New("runtime pull Docker manifest bytes changed")
	}
	var manifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Config        struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			MediaType string   `json:"mediaType"`
			Size      int64    `json:"size"`
			Digest    string   `json:"digest"`
			URLs      []string `json:"urls,omitempty"`
		} `json:"layers"`
	}
	if err := decodeStrict(data, &manifest); err != nil {
		return err
	}
	if manifest.SchemaVersion != 2 || manifest.MediaType != m.Image.ManifestMediaType || manifest.Config.MediaType != "application/vnd.docker.container.image.v1+json" || manifest.Config.Size != m.Image.ConfigBytes || manifest.Config.Digest != m.Image.ID || len(manifest.Layers) != len(m.Image.Layers) {
		return errors.New("runtime pull Docker manifest config changed")
	}
	for i, layer := range manifest.Layers {
		want := m.Image.Layers[i]
		if layer.MediaType != want.MediaType || layer.Size != want.CompressedBytes || layer.Digest != want.CompressedDigest || len(layer.URLs) != 0 {
			return errors.New("runtime pull Docker manifest layer changed")
		}
	}
	return nil
}

func verifyPullStore(b *PullBundle) error {
	m := b.Manifest
	if _, err := b.Store.Seek(0, io.SeekStart); err != nil {
		return err
	}
	whole := sha256.New()
	tee := io.TeeReader(io.LimitReader(b.Store, m.RegistryStore.Bytes+1), whole)
	reader := tar.NewReader(tee)
	wanted := expectedPullMembers(m)
	if len(wanted) != len(m.RegistryStore.Members) {
		return errors.New("runtime pull store contains unrecognized links or blobs")
	}
	var configData, manifestData []byte
	deadline := time.Now().Add(pullTotalDeadline)
	for index, inventory := range m.RegistryStore.Members {
		if time.Now().After(deadline) {
			return errors.New("runtime pull store verification timed out")
		}
		header, err := reader.Next()
		want, ok := wanted[inventory.Path]
		if err != nil || !ok || header.Name != inventory.Path || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Size != inventory.Bytes || header.Linkname != "" || header.PAXRecords != nil || want.bytes != 0 && want.bytes != inventory.Bytes || want.hash != inventory.SHA256 {
			return fmt.Errorf("runtime pull store member %d changed", index)
		}
		if want.role == "manifest" && (header.Size < 1 || header.Size > maxMetadata) {
			return errors.New("runtime pull Docker manifest oversized")
		}
		hash := sha256.New()
		if want.role == "layer" {
			var layer PullLayer
			for _, candidate := range m.Image.Layers {
				if candidate.CompressedDigest == want.value {
					layer = candidate
					break
				}
			}
			if err := verifyPullLayer(reader, header.Size, layer, minTime(deadline, time.Now().Add(pullLayerDeadline))); err != nil {
				return err
			}
			continue
		}
		var capture bytes.Buffer
		writer := io.Writer(hash)
		if want.role == "config" || want.role == "manifest" || want.role == "link" {
			writer = io.MultiWriter(hash, &capture)
		}
		n, err := copyPullBound(writer, io.LimitReader(reader, header.Size), header.Size, deadline)
		if err != nil || n != header.Size || hex.EncodeToString(hash.Sum(nil)) != want.hash {
			return errors.New("runtime pull store bytes changed")
		}
		switch want.role {
		case "config":
			configData = append([]byte(nil), capture.Bytes()...)
		case "manifest":
			manifestData = append([]byte(nil), capture.Bytes()...)
		case "link":
			if capture.String() != want.value {
				return errors.New("runtime pull store link changed")
			}
		}
	}
	if _, err := reader.Next(); err != io.EOF {
		return errors.New("runtime pull store has extra member")
	}
	var tail [32 << 10]byte
	for {
		n, err := tee.Read(tail[:])
		for _, value := range tail[:n] {
			if value != 0 {
				return errors.New("runtime pull store has nonzero trailing bytes")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil || n == 0 {
			return errors.New("runtime pull store tail unavailable")
		}
	}
	if info, err := b.Store.Stat(); err != nil || info.Size() != m.RegistryStore.Bytes || hex.EncodeToString(whole.Sum(nil)) != m.RegistryStore.SHA256 {
		return errors.New("runtime pull store archive changed")
	}
	if err := verifyPullDockerManifest(m, manifestData); err != nil {
		return err
	}
	if err := verifyPullImageConfig(m, configData); err != nil {
		return err
	}
	_, err := b.Store.Seek(0, io.SeekStart)
	return err
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// VerifyPullPayload rechecks held bytes immediately before a Docker effect.
func (b *PullBundle) VerifyPullPayload() error {
	if b == nil || b.Store == nil || b.Bootstrap == nil {
		return errors.New("runtime pull package unavailable")
	}
	if err := verifyPullStore(b); err != nil {
		return err
	}
	return verifyPullBootstrap(b)
}
