package deploymentplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
)

func collectorTestBlueprint() Blueprint {
	base := exampleBlueprint()
	base.Services = base.Services[:1]
	base.Services[0].DependsOn = nil
	dependencies := map[string]string{}
	for _, name := range nodeTelemetryPackages {
		dependencies[name] = "2.0.1"
	}
	dependencies["@opentelemetry/api"] = "1.9.0"
	dependencies["@opentelemetry/sdk-node"] = "0.200.0"
	dependencies["@opentelemetry/exporter-trace-otlp-http"] = "0.200.0"
	dependencies["@opentelemetry/exporter-metrics-otlp-http"] = "0.200.0"
	base.Services[0].Telemetry = &TelemetryBinding{ResourceID: "collector", ClientTLS: &TelemetryTLSReference{ResourceName: "client-tls", CAKey: "ca", CertKey: "cert", KeyKey: "key"}, Instrumentation: &NodeInstrumentation{Runtime: "nodejs", Dependencies: dependencies}}
	base.Services = append(base.Services, Service{ID: "collector", Kind: "collector", Image: "registry.example/otelcol-contrib@sha256:" + strings.Repeat("d", 64), Port: 4318, Replicas: 1, Resources: ResourceBudget{100, 500, 128, 256, 0}, Collector: &CollectorConfiguration{Profile: CollectorProfile, Destination: TelemetryDestination{ResourceID: "telemetry", Endpoint: "https://owned.example/otlp", Authorization: &TelemetryAuthorizationReference{ResourceName: "export-auth", Key: "header"}}, ReceiverTLS: TelemetryTLSReference{ResourceName: "server-tls", CAKey: "ca", CertKey: "cert", KeyKey: "key"}, Events: []ProductEvent{{ID: "purchase", Description: "A completed purchase occurrence, not a revenue amount"}, {ID: "activation", Description: "First successful use of the core product"}}}})
	return base
}
func TestCollectorCompileSeparatesActualPipelineManifestsFromSDKIntegrationArtifacts(t *testing.T) {
	b := collectorTestBlueprint()
	original, _ := json.Marshal(b)
	plan, err := Compile(b)
	if err != nil || plan.State != "compiled-prototype" || len(plan.Artifacts) != 4 || len(plan.SupportingArtifacts) != 5 {
		t.Fatal("closed Collector plan did not compile", err, plan.Decisions)
	}
	again, err := Compile(b)
	if err != nil || plan.Digest != again.Digest || !reflect.DeepEqual(plan.Artifacts, again.Artifacts) || !reflect.DeepEqual(plan.SupportingArtifacts, again.SupportingArtifacts) {
		t.Fatal("telemetry generation is nondeterministic", err)
	}
	after, _ := json.Marshal(b)
	if string(after) != string(original) {
		t.Fatal("compiler mutated caller events/dependencies")
	}
	var app, collector appsv1.Deployment
	for _, artifact := range plan.Artifacts {
		if !strings.HasSuffix(artifact.Name, ".deployment.json") {
			continue
		}
		var value appsv1.Deployment
		if json.Unmarshal([]byte(artifact.Content), &value) != nil {
			t.Fatal("invalid deployment")
		}
		if value.Labels["luci.ai/component"] == "collector" {
			collector = value
		} else {
			app = value
		}
	}
	if len(collector.Spec.Template.Spec.Containers) != 1 || len(collector.Spec.Template.Spec.Containers[0].Ports) != 2 || collector.Spec.Template.Spec.AutomountServiceAccountToken == nil || *collector.Spec.Template.Spec.AutomountServiceAccountToken {
		t.Fatal("Collector listener or token-automount boundary changed")
	}
	c := collector.Spec.Template.Spec.Containers[0]
	if len(c.Args) != 1 || c.Args[0] != "--config=env:LUCI_OTEL_CONFIG" {
		t.Fatal("arbitrary Collector entrypoint/config used", c.Args)
	}
	config := ""
	for _, env := range c.Env {
		if env.Name == "LUCI_OTEL_CONFIG" {
			config = env.Value
		}
		if env.Name == "LUCI_OTEL_DESTINATION_AUTHORIZATION" && (env.Value != "" || env.ValueFrom == nil || env.ValueFrom.SecretKeyRef.Name != "export-auth") {
			t.Fatal("authorization was inlined")
		}
	}
	for _, required := range []string{"memory_limiter", "batch", "client_ca_file", "otlp_http/destination", "prometheus/product", "filter/product", "transform/product", "keep_keys", "metric_conditions", "/luci-otel/server/key.pem"} {
		if !strings.Contains(config, required) {
			t.Fatal("pipeline omitted required behavior", required)
		}
	}
	var endpoint, instance bool
	for _, env := range app.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "OTEL_EXPORTER_OTLP_ENDPOINT" {
			endpoint = env.Value == TelemetryEndpoint(plan.Blueprint, plan.Blueprint.Services[1])
		}
		if env.Name == "LUCI_OTEL_INSTANCE" {
			instance = env.ValueFrom != nil && env.ValueFrom.FieldRef.FieldPath == "metadata.uid"
		}
	}
	if !endpoint || !instance || len(app.Spec.Template.Spec.Volumes) != 1 {
		t.Fatal("application exporter lost mTLS or exact Pod identity")
	}
	for _, artifact := range plan.SupportingArtifacts {
		if artifact.SHA256 != digest([]byte(artifact.Content)) {
			t.Fatal("supporting artifact hash changed")
		}
		if strings.HasSuffix(artifact.Name, ".events.mjs") && (strings.Contains(artifact.Content, "recordException") || !strings.Contains(artifact.Content, "counter.add(1, { event, outcome })")) {
			t.Fatal("event adapter captures arbitrary error/customer data")
		}
	}
	if plan.EffectsApplied || plan.SourceVerified {
		t.Fatal("source-only compiler claimed deployment/instrumentation proof")
	}
}
func TestCollectorRejectsConfigurationInterpolationRuntimeOverridesAndUnboundedEventData(t *testing.T) {
	for _, scenario := range []string{"destination-interpolation", "unreviewed-profile", "runtime-override", "sdk-range", "event-identity", "exporter-env-override"} {
		t.Run(scenario, func(t *testing.T) {
			b := collectorTestBlueprint()
			switch scenario {
			case "destination-interpolation":
				b.Services[1].Collector.Destination.Endpoint = "https://owned.example/${env:SECRET}"
			case "unreviewed-profile":
				b.Services[1].Collector.Profile = "latest"
			case "runtime-override":
				b.Services[1].Command = []string{"/bin/sh"}
			case "sdk-range":
				b.Services[0].Telemetry.Instrumentation.Dependencies["@opentelemetry/api"] = "^1.9.0"
			case "event-identity":
				b.Services[1].Collector.Events[0].ID = "customer@example.test"
			case "exporter-env-override":
				b.Services[0].Environment = []EnvironmentReference{{Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", Kind: "secret", ResourceName: "other", Key: "endpoint"}}
			}
			if _, err := Compile(b); err == nil {
				t.Fatal("unsafe runtime/telemetry intent accepted", scenario)
			}
		})
	}
}
func TestLegacyTelemetryAndResourceNamesRemainUnchangedWithoutCollector(t *testing.T) {
	if ResourceName("p-one", "web") != "web-e608120bd59c" {
		t.Fatal("existing deployment names changed")
	}
	b := exampleBlueprint()
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), "\"collector\":") {
		t.Fatal("optional fields changed legacy blueprint encoding")
	}
	plan, err := Compile(b)
	if err != nil || len(plan.Artifacts) != 4 || plan.SupportingArtifacts != nil {
		t.Fatal("legacy external endpoint acquired generated workloads or SDK files", err)
	}
	encoded, _ := json.Marshal(plan)
	if strings.Contains(string(encoded), "\"supportingArtifacts\"") {
		t.Fatal("legacy plan gained optional supporting artifacts")
	}
}
