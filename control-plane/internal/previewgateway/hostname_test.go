package previewgateway

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestLegacyCapabilityRegistrationKeepsItsKeyWithDNSValidOrigin(t *testing.T) {
	_, registry := testGateway(t, func(_ context.Context, _ Target) error { return nil })
	target := appTarget("http://127.0.0.1:3000")
	key := strings.Repeat("a", 32) + strings.Repeat("b", 32)
	legacy := record{Target: target, Key: key}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry.filename(target), data, 0600); err != nil {
		t.Fatal(err)
	}
	origin, err := registry.Publish(target)
	if err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(origin)
	if err != nil || address.Hostname() != strings.Repeat("a", 32)+"."+strings.Repeat("b", 32)+".localhost" {
		t.Fatal("legacy key did not produce the canonical two-label origin")
	}
	for _, label := range strings.Split(address.Hostname(), ".") {
		if len(label) == 0 || len(label) > 63 {
			t.Fatal("browser origin exceeds a DNS label limit")
		}
	}
	if requestKey(address.Host, 8731) != key {
		t.Fatal("canonical request lost capability entropy")
	}
	retained, err := readRecord(registry.dir, registry.filename(target), 8731)
	if err != nil || retained.Key != key || retained.ProjectID != target.ProjectID || retained.ResourceID != target.ResourceID {
		t.Fatal("migration regenerated the key or rebound the registration")
	}
	if again, err := registry.Publish(target); err != nil || again != origin {
		t.Fatal("canonical address changed when the existing registration refreshed")
	}
}

func TestRequestCapabilityRequiresExactDNSValidCanonicalLabels(t *testing.T) {
	key := strings.Repeat("a", 32) + strings.Repeat("b", 32)
	valid := CapabilityHostname(key)
	if !IsCapabilityHostname(valid) || requestKey(valid+":8731", 8731) != key {
		t.Fatal("canonical browser capability was rejected")
	}
	for _, host := range []string{
		key + ".localhost:8731",
		strings.Repeat("a", 31) + "." + strings.Repeat("b", 33) + ".localhost:8731",
		strings.Repeat("a", 33) + "." + strings.Repeat("b", 31) + ".localhost:8731",
		"extra." + valid + ":8731",
		valid + ".example:8731",
		valid + ":8730",
		strings.ToUpper(valid) + ":8731",
		valid + ".:8731",
		"127.0.0.1:8731",
	} {
		if requestKey(host, 8731) != "" {
			t.Fatal("malformed or different browser origin was accepted")
		}
	}
	if CapabilityHostname(strings.Repeat("a", 63)) != "" || CapabilityHostname(strings.Repeat("g", 64)) != "" {
		t.Fatal("invalid persisted capability was formatted")
	}
}
