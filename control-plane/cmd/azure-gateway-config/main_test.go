package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func gatewayFixture() config {
	return config{GatewayHost: "embed.private.example", PrivateListenIP: "10.20.0.4", ControllerIP: "10.20.0.5", Scope: "scope-one", Generation: "generation-one",
		ServerCertFile: "/run/luci-gateway/server.crt", ServerKeyFile: "/run/luci-gateway/server.key", ClientCAFile: "/run/luci-gateway/client-ca.crt"}
}

func TestAzureGatewayRequiresCurrentPrivateCertificateMaterials(t *testing.T) {
	c := gatewayFixture()
	c.ServerCertFile = filepath.Join(t.TempDir(), "server.crt")
	c.ServerKeyFile = filepath.Join(filepath.Dir(c.ServerCertFile), "server.key")
	c.ClientCAFile = filepath.Join(filepath.Dir(c.ServerCertFile), "client-ca.crt")
	now := time.Now().UTC()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cert := func(names []string, ca bool) []byte {
		t.Helper()
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private-gateway"}, DNSNames: names,
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: ca, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	write(c.ServerKeyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKey}))
	write(c.ClientCAFile, cert(nil, true))
	write(c.ServerCertFile, cert([]string{c.GatewayHost, "*." + c.GatewayHost}, false))
	if err := validateGatewayMaterials(c, now); err != nil {
		t.Fatal("current exact TLS materials refused", err)
	}
	write(c.ServerCertFile, cert([]string{c.GatewayHost}, false))
	if err := validateGatewayMaterials(c, now); err == nil {
		t.Fatal("server certificate without guest origins accepted")
	}
	write(c.ServerCertFile, cert([]string{c.GatewayHost, "*." + c.GatewayHost}, false))
	if err := validateGatewayMaterials(c, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired gateway certificate accepted")
	}
	if err := os.Chmod(c.ClientCAFile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateGatewayMaterials(c, now); err == nil {
		t.Fatal("shared-readable gateway trust root accepted")
	}
}

func TestAzureGatewayRendersExactPrivateManagementAndGuestBoundary(t *testing.T) {
	data, err := render(gatewayFixture())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, required := range []string{
		"address: '10.20.0.4', port_value: 443", "require_client_certificate: true",
		"spiffe://luci/controller/scope-one/generation-one", "trusted_ca:",
		e2bbe.GatewayReceiptDigest("embed.private.example", "scope-one", "generation-one"),
		"name: 'x-admin-token'", "name: 'x-api-key'", "name: 'e2b-traffic-access-token'",
		`^49983-[a-z0-9]+\.embed\.private\.example$`, `^8080-[a-z0-9]+\.embed\.private\.example$`,
		"port_value: 3000", "port_value: 3002", "safe_regex: {google_re2: {}, regex: '^/nodes$'}",
	} {
		if !strings.Contains(s, required) {
			t.Fatal("private route or authentication guard missing", required)
		}
	}
	if strings.Contains(s, "5008") || strings.Contains(s, "0.0.0.0") || strings.Contains(s, "port_value: 3001") ||
		strings.Count(s, "cluster: e2b-api, timeout") != len(managementRoutes) ||
		strings.Count(s, "cluster: e2b-client-proxy, timeout") != 4 ||
		strings.Count(s, "cluster: e2b-client-proxy, timeout: 0s") != 2 {
		t.Fatal("gateway exposes an unexpected upstream or route")
	}
}

func TestAzureGatewayRejectsPublicListenerAndConfigurationInjection(t *testing.T) {
	for _, change := range []func(*config){
		func(c *config) { c.PrivateListenIP = "0.0.0.0" },
		func(c *config) { c.PrivateListenIP = "8.8.8.8" },
		func(c *config) { c.ControllerIP = "0.0.0.0" },
		func(c *config) { c.ControllerIP = "8.8.8.8" },
		func(c *config) { c.ControllerIP = c.PrivateListenIP },
		func(c *config) { c.GatewayHost += "'\npublic-route" },
		func(c *config) { c.ServerCertFile += "'\npublic-route" },
		func(c *config) { c.Generation = "other/generation" },
		func(c *config) { c.ClientCAFile = c.ServerKeyFile },
	} {
		c := gatewayFixture()
		change(&c)
		if _, err := render(c); err == nil {
			t.Fatal("unsafe gateway configuration was accepted", c)
		}
	}
}

func TestAzureGatewayDedicatedHostFirewallDeniesDirectRuntimePorts(t *testing.T) {
	data, err := renderFirewall(gatewayFixture())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, required := range []string{
		"table inet luci_embed", "policy drop", `iifname "lo" accept`,
		"ct state established,related accept", "ip saddr 10.20.0.5/32 ip daddr 10.20.0.4 tcp dport 443 accept",
	} {
		if !strings.Contains(s, required) {
			t.Fatal("dedicated-host ingress rule missing", required)
		}
	}
	if strings.Contains(s, "3000") || strings.Contains(s, "3002") || strings.Contains(s, "5008") || strings.Contains(s, "0.0.0.0/0") {
		t.Fatal("direct E2B host-network port or unrestricted peer was opened")
	}
}
