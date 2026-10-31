package deploymentplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func ownedStoreBlueprint() Blueprint {
	b := collectorTestBlueprint()
	b.ExternalResources = nil
	b.Services[1].Collector.Destination = TelemetryDestination{}
	ref := func(name string) TelemetryTLSReference {
		return TelemetryTLSReference{ResourceName: name, CAKey: "ca.crt", CertKey: "tls.crt", KeyKey: "tls.key"}
	}
	b.Services[1].Collector.OwnedDestination = &OwnedTelemetryDestination{MetricsServiceID: "metrics", TracesServiceID: "traces", ClientTLS: ref("collector-storage")}
	for _, row := range []struct {
		id, kind, profile string
		port              int
	}{{"metrics", "metrics-store", PrometheusStorageProfile, 9090}, {"traces", "trace-store", TempoStorageProfile, 3200}} {
		service := Service{ID: row.id, Kind: row.kind, Image: "registry.example/" + row.id + "@sha256:" + strings.Repeat("a", 64), Port: row.port, Replicas: 1, DeploymentStrategy: "recreate", Resources: ResourceBudget{RequestMilliCPU: 250, LimitMilliCPU: 1000, RequestMemoryMiB: 512, LimitMemoryMiB: 1024}, TelemetryStorage: &TelemetryStorage{Profile: row.profile, ClaimName: row.id + "-data", CapacityGiB: 10, RetentionHours: 168, ServerTLS: ref(row.id + "-server"), QueryTLS: ref(row.id + "-query")}}
		if row.kind == "trace-store" {
			service.TelemetryStorage.Azure = &TelemetryAzureBlocks{AccountName: "ownedtraces", ContainerName: "traces", AccountKey: TelemetryAuthorizationReference{ResourceName: "trace-key", Key: "account.key"}}
		}
		b.Services = append(b.Services, service)
	}
	return b
}
func TestOwnedTelemetryCompilesSeparateDurableBackendsAndClosedSecretFreeConfiguration(t *testing.T) {
	b := ownedStoreBlueprint()
	plan, err := Compile(b)
	if err != nil || plan.State != "compiled-prototype" || len(plan.Artifacts) != 10 {
		t.Fatal("owned stores not connected", err, plan.Decisions)
	}
	configs := 0
	for _, a := range plan.Artifacts {
		if strings.HasSuffix(a.Name, ".configmap.json") {
			configs++
			if a.SHA256 != digest([]byte(a.Content)) {
				t.Fatal("configuration digest mismatch")
			}
			var value map[string]any
			if json.Unmarshal([]byte(a.Content), &value) != nil || value["kind"] != "ConfigMap" {
				t.Fatal("not a real configmap")
			}
		}
	}
	if configs != 2 {
		t.Fatal("both runtime configurations required")
	}
	var collector Service
	for _, s := range plan.Blueprint.Services {
		if s.Collector != nil {
			collector = s
		}
	}
	raw, err := CollectorConfig(plan.Blueprint, collector, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	config := string(raw)
	for _, text := range []string{"otlp_http/metrics", "otlp_http/traces", "/api/v1/otlp", "/luci-otel/storage/cert.pem", "filter/product"} {
		if !strings.Contains(config, text) {
			t.Fatal("missing real pipeline", text)
		}
	}
	if strings.Contains(config, "otlp_http/destination") {
		t.Fatal("external destination silently substituted")
	}
	for _, s := range plan.Blueprint.Services {
		if s.TelemetryStorage == nil {
			continue
		}
		files, e := TelemetryStorageConfig(plan.Blueprint, s)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := json.Marshal(files)
		if s.Kind == "trace-store" && (!strings.Contains(string(body), "storage_account_key") || strings.Contains(string(body), "\"backend\": \"local\"")) {
			t.Fatal("trace blocks not configured in Azure")
		}
	}
}
func TestOwnedTelemetryUnresolvedNewPVCCanBeSavedWithoutInventingResource(t *testing.T) {
	b := ownedStoreBlueprint()
	b.Services[2].TelemetryStorage.ClaimName = ""
	plan, err := Compile(b)
	if err != nil || plan.State != "decisions-required" || len(plan.Artifacts) != 0 {
		t.Fatal("new volume cannot enter resource preparation", err)
	}
	found := false
	for _, d := range plan.Decisions {
		found = found || d.Code == "storage-claim-required"
	}
	if !found {
		t.Fatal("missing explicit prerequisite")
	}
}
func TestOwnedTelemetryRejectsIncompatibleStorageOrExternalFallback(t *testing.T) {
	for _, name := range []string{"replicas", "rolling", "read-only-model", "local-traces", "runtime-command", "empty-client-ca", "external-fallback", "retention"} {
		t.Run(name, func(t *testing.T) {
			b := ownedStoreBlueprint()
			switch name {
			case "replicas":
				b.Services[2].Replicas = 2
			case "rolling":
				b.Services[2].DeploymentStrategy = ""
			case "read-only-model":
				b.Services[2].Model = &ModelReference{Name: "wrong", ClaimName: "model", MountPath: "/model"}
			case "local-traces":
				b.Services[3].TelemetryStorage.Azure = nil
			case "runtime-command":
				b.Services[3].Command = []string{"sh"}
			case "empty-client-ca":
				b.Services[1].Collector.OwnedDestination.ClientTLS.CAKey = ""
			case "external-fallback":
				b.Services[1].Collector.Destination.Endpoint = "https://other.example"
			case "retention":
				b.Services[2].TelemetryStorage.RetentionHours = 745
			}
			if _, e := Compile(b); e == nil {
				t.Fatal("invalid storage accepted")
			}
		})
	}
}
func TestTelemetryBlobTemplateIsDedicatedPrivateAndHasNoCredentialOutput(t *testing.T) {
	spec := AzureTelemetryStorageSpec{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", AccountName: "ownedtraces", ContainerName: "traces", RetentionDays: 7}
	body, err := AzureTelemetryStorageTemplate(spec, "australiaeast", "cp-original")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"supportsHttpsTrafficOnly", "TLS1_2", "allowBlobPublicAccess", "managementPolicies", "daysAfterModificationGreaterThan"} {
		if !strings.Contains(body, value) {
			t.Fatal("missing storage control", value)
		}
	}
	if strings.Contains(body, "listKeys") || strings.Contains(body, "secureString") || strings.Contains(body, "managedClusters") {
		t.Fatal("hidden credential/cluster effect")
	}
	spec.ContainerName = "bad--container"
	if _, e := AzureTelemetryStorageTemplate(spec, "australiaeast", "cp-original"); e == nil {
		t.Fatal("invalid container accepted")
	}
}
