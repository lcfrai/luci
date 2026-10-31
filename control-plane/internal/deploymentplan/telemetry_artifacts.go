package deploymentplan

import (
	"encoding/json"
	"fmt"
)

func configureTelemetryContainer(b Blueprint, s Service, planDigest string, container, pod map[string]any) error {
	addEnv := func(value any) { rows, _ := container["env"].([]any); container["env"] = append(rows, value) }
	addTLS := func(name, path string, ref TelemetryTLSReference) {
		mounts, _ := container["volumeMounts"].([]any)
		container["volumeMounts"] = append(mounts, map[string]any{"name": name, "mountPath": path, "readOnly": true})
		volumes, _ := pod["volumes"].([]any)
		pod["volumes"] = append(volumes, map[string]any{"name": name, "secret": map[string]any{"secretName": ref.ResourceName, "defaultMode": 292, "optional": false, "items": []any{map[string]string{"key": ref.CAKey, "path": "ca.pem"}, map[string]string{"key": ref.CertKey, "path": "cert.pem"}, map[string]string{"key": ref.KeyKey, "path": "key.pem"}}}})
	}
	if s.Collector != nil {
		config, err := CollectorConfig(b, s, planDigest)
		if err != nil {
			return err
		}
		container["args"] = []string{"--config=env:LUCI_OTEL_CONFIG"}
		addEnv(map[string]string{"name": "LUCI_OTEL_CONFIG", "value": string(config)})
		if a := s.Collector.Destination.Authorization; a != nil {
			addEnv(map[string]any{"name": "LUCI_OTEL_DESTINATION_AUTHORIZATION", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": a.ResourceName, "key": a.Key, "optional": false}}})
		}
		container["ports"] = []any{map[string]any{"name": "service", "containerPort": 4318, "protocol": "TCP"}, map[string]any{"name": "metrics", "containerPort": CollectorMetricsPort, "protocol": "TCP"}}
		container["readinessProbe"] = map[string]any{"tcpSocket": map[string]any{"port": 4318}, "periodSeconds": 10, "timeoutSeconds": 2, "failureThreshold": 3, "successThreshold": 1}
		addTLS("luci-otel-server", "/luci-otel/server", s.Collector.ReceiverTLS)
		if d := s.Collector.OwnedDestination; d != nil {
			addTLS("luci-otel-storage", "/luci-otel/storage", d.ClientTLS)
		}
	}
	if s.Telemetry != nil && s.Telemetry.ClientTLS != nil {
		addTLS("luci-otel-client", "/luci-otel/client", *s.Telemetry.ClientTLS)
		for _, row := range []struct{ name, value string }{{"OTEL_EXPORTER_OTLP_CERTIFICATE", "/luci-otel/client/ca.pem"}, {"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE", "/luci-otel/client/cert.pem"}, {"OTEL_EXPORTER_OTLP_CLIENT_KEY", "/luci-otel/client/key.pem"}, {"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE", "cumulative"}} {
			addEnv(map[string]string{"name": row.name, "value": row.value})
		}
		addEnv(map[string]any{"name": "LUCI_OTEL_INSTANCE", "valueFrom": map[string]any{"fieldRef": map[string]string{"apiVersion": "v1", "fieldPath": "metadata.uid"}}})
	}
	return nil
}

func telemetrySupportingArtifacts(b Blueprint, planDigest string) ([]Artifact, error) {
	var out []Artifact
	add := func(name, media string, raw []byte) {
		raw = append(raw, '\n')
		out = append(out, Artifact{Name: name, MediaType: media, SHA256: digest(raw), Content: string(raw)})
	}
	for _, s := range b.Services {
		if s.Collector != nil {
			raw, err := CollectorConfig(b, s, planDigest)
			if err != nil {
				return nil, err
			}
			add(s.ID+".collector-config.json", "application/json", raw)
		}
		if s.Telemetry == nil || s.Telemetry.Instrumentation == nil {
			continue
		}
		collector := ManagedCollector(b, s.Telemetry.ResourceID)
		if collector == nil {
			return nil, fmt.Errorf("instrumentation Collector missing")
		}
		eventIDs := []string{}
		for _, event := range collector.Collector.Events {
			eventIDs = append(eventIDs, event.ID)
		}
		events, _ := json.Marshal(eventIDs)
		serviceID, _ := json.Marshal(s.ID)
		projectID, _ := json.Marshal(b.ProjectID)
		bootstrap := fmt.Sprintf(`// Generated explicit Node.js instrumentation. Review and include in your app image.
import { NodeSDK } from '@opentelemetry/sdk-node';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http';
const instance = process.env.LUCI_OTEL_INSTANCE;
if (!instance || !/^[a-zA-Z0-9-]{1,128}$/.test(instance)) throw new Error('The deployed Pod identity is required for Luci telemetry.');
const sdk = new NodeSDK({
  autoDetectResources: false,
  resource: resourceFromAttributes({ 'service.name': %s, 'service.namespace': %s, 'service.instance.id': instance }),
  traceExporter: new OTLPTraceExporter({ timeoutMillis: 5000 }),
  metricReader: new PeriodicExportingMetricReader({ exporter: new OTLPMetricExporter({ timeoutMillis: 5000 }), exportIntervalMillis: 10000, exportTimeoutMillis: 5000 }),
  instrumentations: [],
  logRecordProcessors: [],
});
sdk.start();
// Call from YOUR existing graceful-shutdown handler; this module never exits the app.
export const shutdownTelemetry = () => sdk.shutdown();`, serviceID, projectID)
		helper := fmt.Sprintf(`// No identifiers, arbitrary attributes, request bodies, URLs or exception messages.
import { metrics, trace, SpanStatusCode } from '@opentelemetry/api';
const allowed = new Set(%s);
const counter = metrics.getMeter('luci-product-outcomes').createCounter('luci_product_events', { description: 'Reported occurrences of explicitly configured product outcomes' });
export function recordOutcome(event, outcome) {
  if (!allowed.has(event) || !['success', 'failure'].includes(outcome)) throw new Error('Choose a configured event and a success/failure outcome.');
  counter.add(1, { event, outcome });
}
export async function withLuciOperation(event, work) {
  if (!allowed.has(event) || typeof work !== 'function') throw new Error('Choose a configured operation.');
  return trace.getTracer('luci-product-outcomes').startActiveSpan('product.' + event, async span => {
    try { const value = await work(); recordOutcome(event, 'success'); span.setStatus({ code: SpanStatusCode.OK }); return value; }
    catch (error) { recordOutcome(event, 'failure'); span.setStatus({ code: SpanStatusCode.ERROR }); throw error; }
    finally { span.end(); }
  });
}`, events)
		dependencies, _ := json.MarshalIndent(map[string]any{"private": true, "dependencies": s.Telemetry.Instrumentation.Dependencies}, "", "  ")
		destination := collector.Collector.Destination.Endpoint
		if owned := collector.Collector.OwnedDestination; owned != nil {
			destination = "the separately planned private stores " + owned.MetricsServiceID + " (closed product metrics) and " + owned.TracesServiceID + " (operation traces), using the reviewed backend client mTLS reference"
		}
		instructions := fmt.Sprintf(`# Include telemetry in %s

1. Review the exact dependency fragment against your existing application lockfile. It is NOT an installation, lockfile or compatible-image verification. Resolve incompatible SDK versions in your normal source/build process.
2. Include %s.bootstrap.mjs in the app and load it BEFORE application code (for example Node's --import option). Import %s.events.mjs at the actual business operation. Use recordOutcome for explicit activation/conversion/purchase occurrences, or withLuciOperation to wrap one operation. Do not call both for the same outcome unless intentionally counting both.
3. Never add personal identifiers, emails, URLs, free-form attributes or transaction/revenue amounts to this schema. No automatic browser/session/request capture is installed. Respect application consent and retention policies before recording customer behavior.
4. Supply existing server/client mTLS Secrets to the selected namespace; certificate names must cover %s. The server CA key is the CA used to trust CLIENT certificates; the client CA key trusts the SERVER certificate. They need not be the same CA. Luci binds names/keys and UID/resourceVersion, not certificate correctness or secret values.
5. Rebuild and check the application normally, then explicitly select its immutable instrumented image digest and review/apply the blueprint. Generated code and environment variables alone are NOT proof instrumentation is deployed or producing data.
6. The Collector distribution must match profile %s; review/validate the supplied image/config independently. The chosen image digest is user-supplied, not acquired or attested by Luci. The OTLP destination is %s; credentials are only existing Secret references. No raw telemetry is sent to Luci.
7. In Deploy, explicitly Read recorded event counters. Samples are reported cumulative values from current owned Pod identities, not unique people, conversion rates, funnels, revenue or lifetime totals. A new Pod/Collector or decreasing counter starts a new baseline. The backend is responsible for durable retention; the Collector's metrics cache expires after five minutes without updates.
8. Call shutdownTelemetry from the app's EXISTING graceful shutdown path. This adapter does not own process exit or application control.
`, s.ID, s.ID, s.ID, ResourceName(b.ProjectID, collector.ID)+"."+b.Infrastructure.Namespace+".svc", CollectorProfile, destination)
		add(s.ID+".bootstrap.mjs", "text/javascript", []byte(bootstrap))
		add(s.ID+".events.mjs", "text/javascript", []byte(helper))
		add(s.ID+".dependencies.json", "application/json", dependencies)
		add(s.ID+".instrumentation.md", "text/markdown", []byte(instructions))
	}
	return out, nil
}

// PreparationArtifacts validates the complete blueprint with the normal
// compiler, then returns only nonsecret app SDK files. Missing image decisions
// remain unresolved; this does not synthesize a deployable plan or image proof.
func PreparationArtifacts(b Blueprint) ([]Artifact, error) {
	plan, err := Compile(b)
	if err != nil {
		return nil, err
	}
	all, err := telemetrySupportingArtifacts(plan.Blueprint, plan.Digest)
	if err != nil {
		return nil, err
	}
	out := []Artifact{}
	for _, a := range all {
		for _, s := range plan.Blueprint.Services {
			if s.Kind != "app" || s.Telemetry == nil || s.Telemetry.Instrumentation == nil {
				continue
			}
			if a.Name == s.ID+".bootstrap.mjs" || a.Name == s.ID+".events.mjs" || a.Name == s.ID+".dependencies.json" || a.Name == s.ID+".instrumentation.md" {
				out = append(out, a)
			}
		}
	}
	return out, nil
}
