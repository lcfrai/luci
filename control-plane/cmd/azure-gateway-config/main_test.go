package main

import (
	"strings"
	"testing"
)

func gatewayFixture() config {
	return config{GatewayHost: "embed.private.example", PrivateListenIP: "10.20.0.4", Scope: "scope-one", Generation: "generation-one",
		ServerCertFile: "/run/luci-gateway/server.crt", ServerKeyFile: "/run/luci-gateway/server.key", ClientCAFile: "/run/luci-gateway/client-ca.crt"}
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
