package runtimeoci

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPullCompanionMarkerBindsBothHeldV2Payloads(t *testing.T) {
	b := &PullBundle{ManifestSHA256: strings.Repeat("a", 64)}
	b.Manifest.SourceCommit = strings.Repeat("b", 40)
	b.Manifest.RegistryStore.FileName = "runtime-registry-store-v2.tar"
	b.Manifest.RegistryStore.SHA256 = strings.Repeat("c", 64)
	b.Manifest.RegistryStore.Bytes = 1234
	b.Manifest.Bootstrap.FileName = "runtime-registry-bootstrap-linux-amd64.tar"
	b.Manifest.Bootstrap.SHA256 = strings.Repeat("d", 64)
	b.Manifest.Bootstrap.Bytes = 2345
	marker := map[string]any{
		"version": 2, "kind": "luci-runtime-pull-companion-required", "sourceCommit": b.Manifest.SourceCommit,
		"manifestSHA256":       b.ManifestSHA256,
		"registryStoreSHA256":  b.Manifest.RegistryStore.SHA256,
		"registryStoreBytes":   b.Manifest.RegistryStore.Bytes,
		"bootstrapSHA256":      b.Manifest.Bootstrap.SHA256,
		"bootstrapBytes":       b.Manifest.Bootstrap.Bytes,
		"ordinaryLocalInstall": "requires-verified-companion", "automaticSourceBuildFallback": false,
	}
	valid, _ := json.Marshal(marker)
	if err := VerifyPullCompanionMarker(valid, b); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(){
		"wrong store":     func() { marker["registryStoreSHA256"] = strings.Repeat("e", 64) },
		"v1 version":      func() { marker["version"] = 1 },
		"source fallback": func() { marker["automaticSourceBuildFallback"] = true },
		"extra member":    func() { marker["archiveSHA256"] = strings.Repeat("f", 64) },
	} {
		var restored map[string]any
		if err := json.Unmarshal(valid, &restored); err != nil {
			t.Fatal(err)
		}
		marker = restored
		edit()
		bad, _ := json.Marshal(marker)
		if VerifyPullCompanionMarker(bad, b) == nil {
			t.Fatal("accepted", name)
		}
	}
	if VerifyPullCompanionMarker([]byte(`{"version":2,"version":2}`), b) == nil {
		t.Fatal("duplicate JSON key accepted")
	}
}
