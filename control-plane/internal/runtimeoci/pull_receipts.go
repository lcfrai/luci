package runtimeoci

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var pullSessionPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type pullConversionReceipt struct {
	Version                int    `json:"version"`
	Kind                   string `json:"kind"`
	SelectedImageID        string `json:"selectedImageID"`
	RegistryManifestDigest string `json:"registryManifestDigest"`
	Config                 struct {
		Digest string `json:"digest"`
		Bytes  int64  `json:"bytes"`
	} `json:"config"`
	Layers []struct {
		CompressedDigest  string `json:"compressedDigest"`
		CompressedBytes   int64  `json:"compressedBytes"`
		DiffID            string `json:"diffID"`
		UncompressedBytes int64  `json:"uncompressedBytes"`
	} `json:"layers"`
	RegistryStore struct {
		SHA256      string `json:"sha256"`
		Bytes       int64  `json:"bytes"`
		MemberCount int    `json:"memberCount"`
	} `json:"registryStore"`
	Verification struct {
		CompressedDigestsVerified bool `json:"compressedDigestsVerified"`
		DiffIDsVerified           bool `json:"diffIDsVerified"`
		ConfigVerified            bool `json:"configVerified"`
	} `json:"verification"`
}

type pullProducerReceipt struct {
	Version                    int    `json:"version"`
	Kind                       string `json:"kind"`
	SourceCommit               string `json:"sourceCommit"`
	CanonicalInputDigest       string `json:"canonicalInputDigest"`
	SelectedImageID            string `json:"selectedImageID"`
	OriginalBuildReceiptSHA256 string `json:"originalBuildReceiptSHA256"`
	ConversionReceiptSHA256    string `json:"conversionReceiptSHA256"`
	RegistryStoreSHA256        string `json:"registryStoreSHA256"`
	RegistryManifestDigest     string `json:"registryManifestDigest"`
	Push                       struct {
		Session                       string `json:"session"`
		Repository                    string `json:"repository"`
		EngineID                      string `json:"engineID"`
		InstanceID                    string `json:"instanceID"`
		ImageID                       string `json:"imageID"`
		CommandExitStatus             int    `json:"commandExitStatus"`
		TerminalStreamSHA256          string `json:"terminalStreamSHA256"`
		ManifestReadbackDigest        string `json:"manifestReadbackDigest"`
		HelperTerminalReceiptSHA256   string `json:"helperTerminalReceiptSHA256"`
		HelperRetirementReceiptSHA256 string `json:"helperRetirementReceiptSHA256"`
	} `json:"push"`
}

type pullBootstrapReceipt struct {
	Version           int    `json:"version"`
	Kind              string `json:"kind"`
	BuildSourceCommit string `json:"buildSourceCommit"`
	GateSourceSHA256  string `json:"gateSourceSHA256"`
	BuildRecipeSHA256 string `json:"buildRecipeSHA256"`
	Distribution      struct {
		Version       string `json:"version"`
		ArchiveSHA256 string `json:"archiveSHA256"`
		BinarySHA256  string `json:"binarySHA256"`
		LicenseSHA256 string `json:"licenseSHA256"`
	} `json:"distribution"`
	Builder struct {
		Argv         []string `json:"argv"`
		Version      string   `json:"version"`
		MemoryMiB    int      `json:"memoryMiB"`
		CPUMillis    int      `json:"cpuMillis"`
		ExitStatus   int      `json:"exitStatus"`
		StdoutSHA256 string   `json:"stdoutSHA256"`
		StderrSHA256 string   `json:"stderrSHA256"`
	} `json:"builder"`
	Image struct {
		ID            string           `json:"id"`
		ConfigSHA256  string           `json:"configSHA256"`
		ConfigBytes   int64            `json:"configBytes"`
		Layers        []BootstrapLayer `json:"layers"`
		ArchiveSHA256 string           `json:"archiveSHA256"`
		ArchiveBytes  int64            `json:"archiveBytes"`
	} `json:"image"`
	Probe struct {
		Argv         []string `json:"argv"`
		ExitStatus   int      `json:"exitStatus"`
		StdoutSHA256 string   `json:"stdoutSHA256"`
		StderrSHA256 string   `json:"stderrSHA256"`
	} `json:"probe"`
}

func pullReceipt(root, file, expectedHash string, out any) error {
	data, err := readRegular(root, file, maxMetadata)
	if err != nil || sumHex(data) != expectedHash {
		return fmt.Errorf("runtime pull %s hash changed", file)
	}
	if err = decodeStrict(data, out); err != nil {
		return err
	}
	// Receipt hashes name one unambiguous compact, sorted-key representation.
	var generic map[string]any
	if err = json.Unmarshal(data, &generic); err != nil {
		return err
	}
	canonical, err := json.Marshal(generic)
	if err != nil || !bytes.Equal(data, canonical) {
		return errors.New("runtime pull receipt is not canonical JSON")
	}
	return nil
}

func validPullArgv(argv []string) bool {
	if len(argv) < 1 || len(argv) > 32 {
		return false
	}
	for _, value := range argv {
		if len(value) < 1 || len(value) > 256 || strings.ContainsAny(value, "\x00\r\n") {
			return false
		}
	}
	return true
}

func verifyPullOriginalReceipt(m PullManifest, receipt buildReceipt) error {
	if receipt.Version != 1 || receipt.Kind != "luci-runtime-producer-build" || !commitPattern.MatchString(receipt.BuildSourceCommit) || receipt.CanonicalInputAlgorithm != m.CanonicalInputAlgorithm || receipt.CanonicalInputDigest != m.CanonicalInputDigest || receipt.Target.OS != "linux" || receipt.Target.Architecture != "amd64" || receipt.ImageID != m.Image.ID || receipt.Archive.Bytes < 1024 || receipt.Archive.Bytes > MaxArchiveBytes || !digestPattern.MatchString(receipt.Archive.SHA256) || !digestPattern.MatchString(receipt.GeneratedContextSHA256) || len(receipt.BaseImages) != 2 || len(receipt.Probes) != 4 || receipt.Builder.Backend != "docker-builder-v1" || receipt.Builder.Version == "" || len(receipt.Builder.Argv) < 3 || len(receipt.Builder.Argv) > 64 || receipt.Builder.MemoryBytes <= 0 || receipt.Builder.CPUQuotaMicros <= 0 || receipt.Builder.ExitCode != 0 || !digestPattern.MatchString(receipt.Builder.StdoutSHA256) || !digestPattern.MatchString(receipt.Builder.StderrSHA256) || !digestPattern.MatchString(receipt.Builder.CacheAssistanceLogSHA256) {
		return errors.New("runtime pull original builder facts invalid")
	}
	if !receipt.Builder.CacheAssistance && receipt.Builder.CacheAssistanceLogSHA256 != sumHex(nil) {
		return errors.New("runtime pull unused cache assistance changed")
	}
	baseRefs := map[string]bool{"golang:1.26-bookworm": false, "node:24-bookworm-slim": false}
	for _, base := range receipt.BaseImages {
		if _, found := baseRefs[base.Reference]; !found || baseRefs[base.Reference] || !imagePattern.MatchString(base.ImageID) || !strings.Contains(base.RepoDigest, "@sha256:") || !digestPattern.MatchString(base.RepoDigest[strings.LastIndex(base.RepoDigest, "@sha256:")+8:]) {
			return errors.New("runtime pull original base receipt invalid")
		}
		baseRefs[base.Reference] = true
	}
	probeVersions := map[string]string{"sandboxd": "0.5.0", "codex": m.Runtime.Harnesses.Codex, "claudeCode": m.Runtime.Harnesses.ClaudeCode, "openCode": m.Runtime.Harnesses.OpenCode}
	for _, probe := range receipt.Probes {
		version, found := probeVersions[probe.Tool]
		if !found || probe.ReportedVersion != version || probe.ExitCode != 0 || len(probe.Argv) < 1 || len(probe.Argv) > 16 || !digestPattern.MatchString(probe.StdoutSHA256) || !digestPattern.MatchString(probe.StderrSHA256) {
			return errors.New("runtime pull original probe invalid")
		}
		delete(probeVersions, probe.Tool)
	}
	if len(probeVersions) != 0 {
		return errors.New("runtime pull original probe missing")
	}
	return nil
}

func verifyPullReceipts(root string, m PullManifest) error {
	var conversion pullConversionReceipt
	if err := pullReceipt(root, "runtime-registry-conversion-receipt-v2.json", m.Producer.ConversionReceiptSHA256, &conversion); err != nil {
		return err
	}
	if conversion.Version != 2 || conversion.Kind != "luci-runtime-registry-conversion" || conversion.SelectedImageID != m.Image.ID || conversion.RegistryManifestDigest != m.Image.ManifestDigest || conversion.Config.Digest != m.Image.ID || conversion.Config.Bytes != m.Image.ConfigBytes || len(conversion.Layers) != len(m.Image.Layers) || conversion.RegistryStore.SHA256 != m.RegistryStore.SHA256 || conversion.RegistryStore.Bytes != m.RegistryStore.Bytes || conversion.RegistryStore.MemberCount != len(m.RegistryStore.Members) || !conversion.Verification.CompressedDigestsVerified || !conversion.Verification.DiffIDsVerified || !conversion.Verification.ConfigVerified {
		return errors.New("runtime pull conversion receipt differs from manifest")
	}
	for i, layer := range conversion.Layers {
		want := m.Image.Layers[i]
		if layer.CompressedDigest != want.CompressedDigest || layer.CompressedBytes != want.CompressedBytes || layer.DiffID != want.DiffID || layer.UncompressedBytes != want.UncompressedBytes {
			return errors.New("runtime pull conversion layer changed")
		}
	}
	var producer pullProducerReceipt
	if err := pullReceipt(root, "runtime-pull-producer-receipt-v2.json", m.Producer.PullProducerReceiptSHA256, &producer); err != nil {
		return err
	}
	if producer.Version != 2 || producer.Kind != "luci-runtime-pull-producer" || producer.SourceCommit != m.SourceCommit || producer.CanonicalInputDigest != m.CanonicalInputDigest || producer.SelectedImageID != m.Image.ID || producer.OriginalBuildReceiptSHA256 != m.Producer.OriginalBuildReceiptSHA256 || producer.ConversionReceiptSHA256 != m.Producer.ConversionReceiptSHA256 || producer.RegistryStoreSHA256 != m.RegistryStore.SHA256 || producer.RegistryManifestDigest != m.Image.ManifestDigest || !pullSessionPattern.MatchString(producer.Push.Session) || producer.Push.Repository != "luci/runtime-export-"+producer.Push.Session || producer.Push.ImageID != m.Image.ID || producer.Push.CommandExitStatus != 0 || producer.Push.ManifestReadbackDigest != m.Image.ManifestDigest {
		return errors.New("runtime pull producer receipt differs from manifest")
	}
	if len(producer.Push.EngineID) == 0 || len(producer.Push.EngineID) > 160 || len(producer.Push.InstanceID) == 0 || len(producer.Push.InstanceID) > 160 || !digestPattern.MatchString(producer.Push.TerminalStreamSHA256) || !digestPattern.MatchString(producer.Push.HelperTerminalReceiptSHA256) || !digestPattern.MatchString(producer.Push.HelperRetirementReceiptSHA256) {
		return errors.New("runtime pull producer terminal identity missing")
	}
	var bootstrap pullBootstrapReceipt
	if err := pullReceipt(root, "runtime-bootstrap-build-receipt-v1.json", m.Bootstrap.BuildReceiptSHA256, &bootstrap); err != nil {
		return err
	}
	if bootstrap.Version != 1 || bootstrap.Kind != "luci-runtime-registry-bootstrap" || !commitPattern.MatchString(bootstrap.BuildSourceCommit) || bootstrap.GateSourceSHA256 != m.Bootstrap.GateSourceSHA256 || bootstrap.BuildRecipeSHA256 != m.Bootstrap.BuildRecipeSHA256 || bootstrap.Distribution.Version != m.Bootstrap.DistributionVersion || bootstrap.Distribution.ArchiveSHA256 != m.Bootstrap.DistributionArchiveSHA256 || bootstrap.Distribution.BinarySHA256 != m.Bootstrap.DistributionBinarySHA256 || bootstrap.Distribution.LicenseSHA256 != m.Bootstrap.DistributionLicenseSHA256 || bootstrap.Image.ID != m.Bootstrap.ImageID || bootstrap.Image.ConfigSHA256 != m.Bootstrap.ConfigSHA256 || bootstrap.Image.ConfigBytes != m.Bootstrap.ConfigBytes || !reflect.DeepEqual(bootstrap.Image.Layers, m.Bootstrap.Layers) || bootstrap.Image.ArchiveSHA256 != m.Bootstrap.SHA256 || bootstrap.Image.ArchiveBytes != m.Bootstrap.Bytes {
		return errors.New("runtime pull bootstrap receipt differs from manifest")
	}
	if !validPullArgv(bootstrap.Builder.Argv) || bootstrap.Builder.Version == "" || bootstrap.Builder.MemoryMiB < 128 || bootstrap.Builder.MemoryMiB > 2048 || bootstrap.Builder.CPUMillis < 100 || bootstrap.Builder.CPUMillis > 4000 || bootstrap.Builder.ExitStatus != 0 || !digestPattern.MatchString(bootstrap.Builder.StdoutSHA256) || !digestPattern.MatchString(bootstrap.Builder.StderrSHA256) || !validPullArgv(bootstrap.Probe.Argv) || bootstrap.Probe.ExitStatus != 0 || !digestPattern.MatchString(bootstrap.Probe.StdoutSHA256) || !digestPattern.MatchString(bootstrap.Probe.StderrSHA256) {
		return errors.New("runtime pull bootstrap builder or probe facts invalid")
	}
	return nil
}
