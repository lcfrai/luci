package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
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

func TestAzureSetupRetirementReadsExactPrivateOwnershipJournals(t *testing.T) {
	root := t.TempDir()
	input := azureSetupToolInput{MountPath: root, Scope: "luci-fixture"}
	selected := e2bOperatorConfig{}
	selected.Store.OwnerID = "owner-one"
	path := filepath.Join(root, "luci", "control", "control-plane", "users", "user-one", "runtime", "state", "cloud-controller", "attempts.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"ownerId":"owner-one","scope":"luci-fixture","records":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	digests, err := azureSetupRetirementJournals(input, selected)
	if err != nil || len(digests) != 1 {
		t.Fatal("settled private journal refused", err, digests)
	}
	selected.Store.OwnerID = "foreign-owner"
	if _, err := azureSetupRetirementJournals(input, selected); err == nil {
		t.Fatal("foreign journal accepted")
	}
	selected.Store.OwnerID = "owner-one"
	lease := filepath.Join(root, "luci", "execution", "leases.json")
	if err := os.MkdirAll(filepath.Dir(lease), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lease, []byte(`{"version":1,"leases":[],"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := azureSetupRetirementJournals(input, selected); err == nil {
		t.Fatal("invalid shared reservation ledger was ignored")
	}
}
