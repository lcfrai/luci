package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCloudHostSelectionRequiresExactExplicitCatalogueAndPreservesLegacyOmission(t *testing.T) {
	legacy := ProjectRunRoute{Harness: "codex", Model: "model", PermissionMode: "bypass"}
	raw, _ := json.Marshal(legacy)
	if strings.Contains(string(raw), "cloudHost") || strings.Contains(string(raw), "cloudPool") {
		t.Fatal("legacy route identity changed")
	}
	if !validCloudHostSelection(nil, "", "") || !validCloudHostSelection([]string{"host-a", "host-b"}, strings.Repeat("a", 64), "cloud-microvm") {
		t.Fatal("valid default or explicit host route refused")
	}
	for _, selection := range []struct {
		ids              []string
		digest, location string
	}{
		{[]string{"host-a"}, "", "cloud-microvm"}, {[]string{"host-a"}, strings.Repeat("a", 64), ""},
		{[]string{"host-a", "host-a"}, strings.Repeat("a", 64), "cloud-microvm"}, {[]string{"https://host"}, strings.Repeat("a", 64), "cloud-microvm"},
		{nil, strings.Repeat("a", 64), "cloud-microvm"},
	} {
		if validCloudHostSelection(selection.ids, selection.digest, selection.location) {
			t.Fatal("unreviewed or ambiguous host authority accepted")
		}
	}
	if !sameCloudHosts([]string{"host-b", "host-a"}, []string{"host-a", "host-b"}) || sameCloudHosts([]string{"host-a"}, []string{"host-b"}) {
		t.Fatal("allow-list identity lost exact membership")
	}
}
