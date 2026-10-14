package runtimeoci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

func verifyBootstrapLayer(reader io.Reader, size int64, want BootstrapLayer, deadline time.Time, binaries map[string]bool, m PullManifest) error {
	limited := &io.LimitedReader{R: reader, N: size}
	hash := sha256.New()
	tee := io.TeeReader(limited, hash)
	nested := tar.NewReader(tee)
	var regularTotal int64
	for index := 0; ; index++ {
		if index > 64 || time.Now().After(deadline) {
			return errors.New("runtime bootstrap layer inventory exceeded bound")
		}
		header, err := nested.Next()
		if err == io.EOF {
			break
		}
		if err != nil || !safePullPath(header.Name) || header.Linkname != "" || header.PAXRecords != nil {
			return errors.New("runtime bootstrap layer has unsafe member")
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Size < 1 || header.Size > MaxBootstrapExpandedBytes-regularTotal || binaries[header.Name] {
			return errors.New("runtime bootstrap layer has unexpected file")
		}
		var expected string
		switch header.Name {
		case "luci-registry-gate":
			expected = m.Bootstrap.GateBinarySHA256
		case "usr/local/bin/registry":
			expected = m.Bootstrap.DistributionBinarySHA256
		default:
			return errors.New("runtime bootstrap layer includes unexpected file")
		}
		memberHash := sha256.New()
		n, err := copyPullBound(memberHash, io.LimitReader(nested, header.Size), header.Size, deadline)
		if err != nil || n != header.Size || hex.EncodeToString(memberHash.Sum(nil)) != expected {
			return errors.New("runtime bootstrap binary changed")
		}
		binaries[header.Name] = true
		regularTotal += header.Size
	}
	var tail [32 << 10]byte
	for {
		n, err := tee.Read(tail[:])
		for _, value := range tail[:n] {
			if value != 0 {
				return errors.New("runtime bootstrap layer trailing bytes changed")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil || n == 0 {
			return errors.New("runtime bootstrap layer tail unavailable")
		}
	}
	if limited.N != 0 || hex.EncodeToString(hash.Sum(nil)) != strings.TrimPrefix(want.DiffID, "sha256:") {
		return errors.New("runtime bootstrap layer diffID changed")
	}
	return nil
}

func verifyBootstrapConfig(m PullManifest, data []byte) error {
	if int64(len(data)) != m.Bootstrap.ConfigBytes || sumHex(data) != m.Bootstrap.ConfigSHA256 {
		return errors.New("runtime bootstrap config bytes changed")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return err
	}
	var config struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Config       struct {
			User         string         `json:"User"`
			WorkingDir   string         `json:"WorkingDir"`
			Entrypoint   []string       `json:"Entrypoint"`
			Env          []string       `json:"Env"`
			ExposedPorts map[string]any `json:"ExposedPorts"`
		} `json:"config"`
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	if config.OS != "linux" || config.Architecture != "amd64" || config.Config.User != m.Bootstrap.User || config.Config.WorkingDir != m.Bootstrap.WorkingDir || !equalStrings(config.Config.Entrypoint, m.Bootstrap.Entrypoint) || len(config.Config.Env) != 0 || len(config.Config.ExposedPorts) != 1 || config.Config.ExposedPorts["5000/tcp"] == nil || len(config.RootFS.DiffIDs) != len(m.Bootstrap.Layers) {
		return errors.New("runtime bootstrap config differs from fixed helper")
	}
	for i, layer := range m.Bootstrap.Layers {
		if config.RootFS.DiffIDs[i] != layer.DiffID {
			return errors.New("runtime bootstrap config layer changed")
		}
	}
	return nil
}

func verifyPullBootstrap(b *PullBundle) error {
	m := b.Manifest
	if _, err := b.Bootstrap.Seek(0, io.SeekStart); err != nil {
		return err
	}
	whole := sha256.New()
	tee := io.TeeReader(io.LimitReader(b.Bootstrap, m.Bootstrap.Bytes+1), whole)
	reader := tar.NewReader(tee)
	deadline := time.Now().Add(bootstrapDeadline)
	var configData, manifestData []byte
	var layerNames []string
	binaries := map[string]bool{}
	var expanded int64
	for index := 0; ; index++ {
		if index > 64 || time.Now().After(deadline) {
			return errors.New("runtime bootstrap archive exceeds inventory or time bound")
		}
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil || !safePullPath(header.Name) || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Linkname != "" || header.PAXRecords != nil || header.Size < 1 || header.Size > MaxBootstrapExpandedBytes-expanded {
			return errors.New("runtime bootstrap archive has unsafe member")
		}
		expanded += header.Size
		switch {
		case header.Name == "manifest.json":
			if manifestData != nil || header.Size > maxMetadata {
				return errors.New("runtime bootstrap manifest duplicate or oversized")
			}
			manifestData = make([]byte, header.Size)
			if _, err := io.ReadFull(reader, manifestData); err != nil {
				return err
			}
		case header.Name == strings.TrimPrefix(m.Bootstrap.ImageID, "sha256:")+".json":
			if configData != nil || header.Size != m.Bootstrap.ConfigBytes {
				return errors.New("runtime bootstrap config duplicate or oversized")
			}
			configData = make([]byte, header.Size)
			if _, err := io.ReadFull(reader, configData); err != nil {
				return err
			}
		case strings.HasSuffix(header.Name, "/layer.tar"):
			if len(layerNames) >= len(m.Bootstrap.Layers) || header.Size != m.Bootstrap.Layers[len(layerNames)].UncompressedBytes {
				return errors.New("runtime bootstrap layer size changed")
			}
			if err := verifyBootstrapLayer(reader, header.Size, m.Bootstrap.Layers[len(layerNames)], deadline, binaries, m); err != nil {
				return err
			}
			layerNames = append(layerNames, header.Name)
		default:
			return errors.New("runtime bootstrap archive includes unexpected member")
		}
	}
	if len(layerNames) != len(m.Bootstrap.Layers) || len(binaries) != 2 || !binaries["luci-registry-gate"] || !binaries["usr/local/bin/registry"] {
		return errors.New("runtime bootstrap helper binaries incomplete")
	}
	var saved []struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
		Layers   []string `json:"Layers"`
	}
	if err := decodeStrict(manifestData, &saved); err != nil || len(saved) != 1 || saved[0].Config != strings.TrimPrefix(m.Bootstrap.ImageID, "sha256:")+".json" || len(saved[0].RepoTags) != 0 || !equalStrings(saved[0].Layers, layerNames) {
		return errors.New("runtime bootstrap save manifest changed")
	}
	if err := verifyBootstrapConfig(m, configData); err != nil {
		return err
	}
	var tail [32 << 10]byte
	for {
		n, err := tee.Read(tail[:])
		for _, value := range tail[:n] {
			if value != 0 {
				return errors.New("runtime bootstrap archive trailing bytes changed")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil || n == 0 {
			return errors.New("runtime bootstrap archive tail unavailable")
		}
	}
	if info, err := b.Bootstrap.Stat(); err != nil || info.Size() != m.Bootstrap.Bytes || hex.EncodeToString(whole.Sum(nil)) != m.Bootstrap.SHA256 {
		return errors.New("runtime bootstrap archive hash changed")
	}
	_, err := b.Bootstrap.Seek(0, io.SeekStart)
	return err
}
