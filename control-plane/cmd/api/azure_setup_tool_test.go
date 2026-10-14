package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"runtime/debug"
	"strings"
	"testing"
)

func TestAzureSetupSourceRequiresExplicitCleanBuild(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for _, test := range []struct {
		revision, modified string
		want               bool
	}{
		{revision, "false", true}, {revision, "true", false}, {revision, "", false}, {strings.Repeat("b", 40), "false", false},
	} {
		info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: test.revision}}}
		if test.modified != "" {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.modified", Value: test.modified})
		}
		if azureSetupCleanSource(info, revision) != test.want {
			t.Fatalf("clean source check accepted changed/unknown build: %+v", test)
		}
	}
	if azureSetupCleanSource(nil, revision) {
		t.Fatal("missing build info accepted")
	}
}

func TestAzureSetupCommandRefusalDoesNotStartServiceOrEchoInput(t *testing.T) {
	var output, diagnostic bytes.Buffer
	code := runCommand([]string{"azure-setup", "template-create", "fixture-secret-do-not-print"}, &output, &diagnostic, func() int {
		t.Fatal("explicit setup command started the account service")
		return 99
	})
	if code != 1 || output.Len() != 0 || strings.Contains(diagnostic.String(), "fixture-secret") {
		t.Fatal("setup refusal exposed input or claimed success", code)
	}
}

func TestAzureSetupGatewayRefusesChangedDestinationBeforeDial(t *testing.T) {
	client := azureSetupGatewayClient("scope-one.internal", nil, tls.Certificate{})
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", "public.example:443"); err == nil {
		t.Fatal("private gateway accepted another destination")
	}
	if _, err := transport.DialContext(context.Background(), "tcp", "scope-one.internal:5008"); err == nil {
		t.Fatal("private gateway accepted the direct service port")
	}
}
