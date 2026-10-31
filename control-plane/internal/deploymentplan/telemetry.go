package deploymentplan

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const CollectorProfile = "otelcol-contrib-0.163.0"
const ProductEventMetric = "luci_product_events_total"
const CollectorMetricsPort = 8889

var exactNodeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
var nodeTelemetryPackages = []string{"@opentelemetry/api", "@opentelemetry/sdk-node", "@opentelemetry/sdk-metrics", "@opentelemetry/resources", "@opentelemetry/exporter-trace-otlp-http", "@opentelemetry/exporter-metrics-otlp-http"}
var telemetryProjectID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ResourceName retains the original compiler naming algorithm for ALL services.
func ResourceName(projectID, componentID string) string {
	name := componentID
	if len(name) > 49 {
		name = strings.TrimRight(name[:49], "-")
	}
	return name + "-" + digest([]byte(projectID + "\x00" + componentID))[:12]
}
func ManagedCollector(b Blueprint, id string) *Service {
	for i := range b.Services {
		if b.Services[i].ID == id && b.Services[i].Kind == "collector" && b.Services[i].Collector != nil {
			return &b.Services[i]
		}
	}
	return nil
}
func TelemetryEndpoint(b Blueprint, service Service) string {
	if service.Telemetry == nil {
		return ""
	}
	if c := ManagedCollector(b, service.Telemetry.ResourceID); c != nil {
		return "https://" + ResourceName(b.ProjectID, c.ID) + "." + b.Infrastructure.Namespace + ".svc:4318"
	}
	return service.Telemetry.Endpoint
}
func validTLSReference(ref TelemetryTLSReference) bool {
	if ref.ResourceName == "" || len(kubevalidation.IsDNS1123Subdomain(ref.ResourceName)) != 0 {
		return false
	}
	keys := map[string]bool{}
	for _, key := range []string{ref.CAKey, ref.CertKey, ref.KeyKey} {
		if key == "" || len(kubevalidation.IsConfigMapKey(key)) != 0 || keys[key] {
			return false
		}
		keys[key] = true
	}
	return true
}
func validateTelemetryBlueprint(b *Blueprint, decision func(string, string, string)) error {
	collectors := 0
	for i := range b.Services {
		s := &b.Services[i]
		if s.Kind != "collector" {
			if s.Collector != nil {
				return fmt.Errorf("only collector services accept a collector configuration")
			}
			continue
		}
		collectors++
		if !telemetryProjectID.MatchString(b.ProjectID) {
			return fmt.Errorf("collector project identity must be a closed identifier, never interpolated configuration")
		}
		if collectors > 1 || s.Collector == nil || s.Collector.Profile != CollectorProfile || s.Telemetry != nil || s.Model != nil || len(s.Command) != 0 || len(s.Args) != 0 || len(s.Environment) != 0 || s.Health != nil || s.Resources.GPUCount != 0 || s.Port != 4318 || s.Replicas != 1 || s.Resources.LimitMemoryMiB < 128 {
			return fmt.Errorf("choose one compatible Collector profile, one replica, port 4318, at least 128 MiB and no arbitrary runtime overrides")
		}
		c := s.Collector
		if !validTLSReference(c.ReceiverTLS) {
			return fmt.Errorf("collector requires existing server certificate/key and client CA Secret references")
		}
		found := false
		for _, external := range b.ExternalResources {
			found = found || external.ID == c.Destination.ResourceID && external.Kind == "analytics"
		}
		if !found || !validEndpoint(c.Destination.Endpoint) || strings.Contains(c.Destination.Endpoint, "$") {
			return fmt.Errorf("collector destination requires an existing analytics resource and literal credential-free HTTPS OTLP endpoint without configuration interpolation")
		}
		if a := c.Destination.Authorization; a != nil && (a.ResourceName == "" || len(kubevalidation.IsDNS1123Subdomain(a.ResourceName)) != 0 || a.Key == "" || len(kubevalidation.IsConfigMapKey(a.Key)) != 0) {
			return fmt.Errorf("invalid existing exporter Authorization Secret key reference")
		}
		if len(c.Events) < 1 || len(c.Events) > 16 {
			return fmt.Errorf("choose one to sixteen explicit business-outcome events")
		}
		seen := map[string]bool{}
		for _, event := range c.Events {
			if !validName(event.ID) || seen[event.ID] || event.Description == "" || !text(event.Description, 256) {
				return fmt.Errorf("event IDs must be unique with bounded descriptions; no identifying/freeform event fields")
			}
			seen[event.ID] = true
		}
		sort.Slice(c.Events, func(i, j int) bool { return c.Events[i].ID < c.Events[j].ID })
		s.DependsOn = uniqueSorted(append(s.DependsOn, c.Destination.ResourceID))
	}
	clients := 0
	for _, s := range b.Services {
		if s.Telemetry == nil {
			continue
		}
		binding := s.Telemetry
		c := ManagedCollector(*b, binding.ResourceID)
		if c == nil {
			if binding.ClientTLS != nil || binding.Instrumentation != nil {
				return fmt.Errorf("managed SDK/mTLS configuration requires the exact planned Collector")
			}
			continue
		}
		clients++
		if clients > 8 {
			return fmt.Errorf("at most eight explicitly instrumented services per Collector")
		}
		if s.Kind == "collector" || binding.Endpoint != "" && binding.Endpoint != TelemetryEndpoint(*b, s) || binding.ClientTLS == nil || !validTLSReference(*binding.ClientTLS) {
			return fmt.Errorf("managed telemetry uses the derived Collector HTTPS service and existing client mTLS Secret")
		}
		if binding.Instrumentation == nil {
			decision("services."+s.ID+".telemetry.instrumentation", "instrumentation-required", "Choose Node.js instrumentation and its exact existing lockfile package versions; environment variables alone do not instrument an application.")
			continue
		}
		if binding.Instrumentation.Runtime != "nodejs" || len(binding.Instrumentation.Dependencies) != len(nodeTelemetryPackages) {
			return fmt.Errorf("choose the Node.js SDK adapter and all six exact existing dependency versions")
		}
		for _, name := range nodeTelemetryPackages {
			version := binding.Instrumentation.Dependencies[name]
			if len(version) > 40 || !exactNodeVersion.MatchString(version) {
				return fmt.Errorf("instrumentation dependencies require exact numeric versions from the application lockfile, not ranges or acquisition")
			}
		}
	}
	if collectors > 0 && clients == 0 {
		decision("services.collector", "instrumented-service-required", "Select at least one app/inference service for this Collector and include its SDK adapter in the deployed app.")
	}
	return nil
}

// Configuration is closed, deterministic and contains only nonsecret settings,
// fixed mount paths and an optional env Secret reference. The generated image
// must actually contain the selected distribution; the compiler cannot attest it.
func CollectorConfig(b Blueprint, service Service, planDigest string) ([]byte, error) {
	c := service.Collector
	if c == nil || c.Profile != CollectorProfile {
		return nil, fmt.Errorf("collector configuration missing")
	}
	exporter := map[string]any{"endpoint": c.Destination.Endpoint, "timeout": "5s", "retry_on_failure": map[string]any{"enabled": true, "initial_interval": "1s", "max_interval": "5s", "max_elapsed_time": "10s"}, "sending_queue": map[string]any{"enabled": true, "queue_size": 128, "num_consumers": 1}}
	if c.Destination.Authorization != nil {
		exporter["headers"] = map[string]string{"Authorization": "${env:LUCI_OTEL_DESTINATION_AUTHORIZATION}"}
	}
	config := map[string]any{
		"receivers":  map[string]any{"otlp": map[string]any{"protocols": map[string]any{"http": map[string]any{"endpoint": "0.0.0.0:4318", "max_request_body_size": 1048576, "tls": map[string]any{"cert_file": "/luci-otel/server/cert.pem", "key_file": "/luci-otel/server/key.pem", "client_ca_file": "/luci-otel/server/ca.pem", "min_version": "1.2"}}}}},
		"processors": map[string]any{"memory_limiter": map[string]any{"check_interval": "1s", "limit_mib": service.Resources.LimitMemoryMiB * 3 / 4, "spike_limit_mib": service.Resources.LimitMemoryMiB / 8}, "batch": map[string]any{"timeout": "1s", "send_batch_size": 128, "send_batch_max_size": 256}},
		"exporters":  map[string]any{"otlp_http/destination": exporter, "prometheus/product": map[string]any{"endpoint": "0.0.0.0:8889", "metric_expiration": "5m", "send_timestamps": false, "without_scope_info": true, "translation_strategy": "UnderscoreEscapingWithSuffixes", "resource_constant_labels": map[string]any{"included": []string{"service.name", "service.namespace", "service.instance.id"}}, "const_labels": map[string]string{"luci_project": b.ProjectID, "luci_plan": planDigest}}},
		"service":    map[string]any{"telemetry": map[string]any{"logs": map[string]string{"level": "error"}, "metrics": map[string]any{"level": "none"}}, "pipelines": map[string]any{"traces": map[string]any{"receivers": []string{"otlp"}, "processors": []string{"memory_limiter", "batch"}, "exporters": []string{"otlp_http/destination"}}, "metrics": map[string]any{"receivers": []string{"otlp"}, "processors": []string{"memory_limiter", "batch"}, "exporters": []string{"otlp_http/destination", "prometheus/product"}}}},
	}
	// The product read surface is a separate, schema-restricted branch. General
	// operational metrics may reach the explicitly selected backend, but are
	// never exposed in Luci's product-counter endpoint. No user OTTL is admitted.
	events, services := []string{}, []string{}
	for _, event := range c.Events {
		events = append(events, fmt.Sprintf(`datapoint.attributes["event"] != %q`, event.ID))
	}
	for _, client := range b.Services {
		if client.Telemetry != nil && client.Telemetry.ResourceID == service.ID {
			services = append(services, fmt.Sprintf(`resource.attributes["service.name"] != %q`, client.ID))
		}
	}
	if len(events) == 0 || len(services) == 0 {
		return nil, fmt.Errorf("Collector requires a bounded event schema and instrumented service")
	}
	processors := config["processors"].(map[string]any)
	processors["filter/product"] = map[string]any{"error_mode": "propagate", "metric_conditions": []string{`metric.name != "luci_product_events"`, `metric.type != METRIC_DATA_TYPE_SUM`, `metric.aggregation_temporality != AGGREGATION_TEMPORALITY_CUMULATIVE`, `metric.is_monotonic != true`, `metric.unit != ""`, fmt.Sprintf(`resource.attributes["service.namespace"] != %q`, b.ProjectID), strings.Join(events, " and "), strings.Join(services, " and "), `datapoint.attributes["outcome"] != "success" and datapoint.attributes["outcome"] != "failure"`, `IsMatch(resource.attributes["service.instance.id"], "^[a-zA-Z0-9-]{1,128}$") == false`}}
	processors["transform/product"] = map[string]any{"error_mode": "propagate", "metric_statements": []string{`keep_keys(resource.attributes, ["service.name", "service.namespace", "service.instance.id"])`, `keep_keys(datapoint.attributes, ["event", "outcome"])`, `set(metric.description, "Reported product outcome occurrences")`}}
	pipelines := config["service"].(map[string]any)["pipelines"].(map[string]any)
	pipelines["metrics"].(map[string]any)["exporters"] = []string{"otlp_http/destination"}
	pipelines["metrics/product"] = map[string]any{"receivers": []string{"otlp"}, "processors": []string{"memory_limiter", "filter/product", "transform/product", "batch"}, "exporters": []string{"prometheus/product"}}
	return json.MarshalIndent(config, "", "  ")
}

// Only existing Secret names/keys, never their values, are returned for current
// UID/resourceVersion review and immediate-before-write revalidation.
func TelemetrySecretKeys(service Service) map[string][]string {
	out := map[string][]string{}
	addTLS := func(ref *TelemetryTLSReference) {
		if ref != nil {
			out[ref.ResourceName] = append(out[ref.ResourceName], ref.CAKey, ref.CertKey, ref.KeyKey)
		}
	}
	if service.Collector != nil {
		addTLS(&service.Collector.ReceiverTLS)
		if a := service.Collector.Destination.Authorization; a != nil {
			out[a.ResourceName] = append(out[a.ResourceName], a.Key)
		}
	}
	if service.Telemetry != nil {
		addTLS(service.Telemetry.ClientTLS)
	}
	for name, keys := range out {
		out[name] = uniqueSorted(keys)
	}
	return out
}
