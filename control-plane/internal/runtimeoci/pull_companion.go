package runtimeoci

import (
	"bytes"
	"encoding/json"
	"errors"
)

// PullCompanionMarker is the small management core's exact binding to the two
// large v2 files carried once by the outer release. It contains no authority
// to start a source build or to select a different Docker transport.
type PullCompanionMarker struct {
	Version                      int    `json:"version"`
	Kind                         string `json:"kind"`
	SourceCommit                 string `json:"sourceCommit"`
	ManifestSHA256               string `json:"manifestSHA256"`
	RegistryStoreSHA256          string `json:"registryStoreSHA256"`
	RegistryStoreBytes           int64  `json:"registryStoreBytes"`
	BootstrapSHA256              string `json:"bootstrapSHA256"`
	BootstrapBytes               int64  `json:"bootstrapBytes"`
	OrdinaryLocalInstall         string `json:"ordinaryLocalInstall"`
	AutomaticSourceBuildFallback bool   `json:"automaticSourceBuildFallback"`
}

// VerifyPullCompanionMarker refuses missing, extra and mixed-transport fields.
// The caller separately verifies the core source members and held payloads.
func VerifyPullCompanionMarker(data []byte, bundle *PullBundle) error {
	if bundle == nil || len(data) == 0 || len(data) > 4096 {
		return errors.New("runtime pull companion marker unavailable or oversized")
	}
	var fields map[string]json.RawMessage
	if err := decodeStrict(data, &fields); err != nil || len(fields) != 10 || !bytes.Equal(bytes.TrimSpace(fields["automaticSourceBuildFallback"]), []byte("false")) {
		return errors.New("runtime pull companion marker fields invalid")
	}
	var marker PullCompanionMarker
	m := bundle.Manifest
	if err := decodeStrict(data, &marker); err != nil || marker.Version != 2 || marker.Kind != "luci-runtime-pull-companion-required" || marker.SourceCommit != m.SourceCommit || marker.ManifestSHA256 != bundle.ManifestSHA256 || marker.RegistryStoreSHA256 != m.RegistryStore.SHA256 || marker.RegistryStoreBytes != m.RegistryStore.Bytes || marker.BootstrapSHA256 != m.Bootstrap.SHA256 || marker.BootstrapBytes != m.Bootstrap.Bytes || marker.OrdinaryLocalInstall != "requires-verified-companion" || marker.AutomaticSourceBuildFallback {
		return errors.New("runtime pull companion marker does not match installed payload")
	}
	return nil
}
