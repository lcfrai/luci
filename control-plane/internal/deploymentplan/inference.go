package deploymentplan

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

func validateServiceRuntime(service *Service, field string, decision func(string, string, string)) error {
	if err := validateImagePullSecrets(service); err != nil {
		return err
	}
	if service.DeploymentStrategy != "" && service.DeploymentStrategy != "recreate" {
		return fmt.Errorf("unknown deployment strategy")
	}
	if len(service.Environment) > 16 {
		return fmt.Errorf("at most sixteen explicit environment references per service")
	}
	seen := map[string]bool{}
	for index, ref := range service.Environment {
		location := fmt.Sprintf("%s.environment[%d]", field, index)
		if ref.Name == "" || ref.ResourceName == "" || ref.Key == "" {
			decision(location, "reference-required", "Choose an environment name and existing namespaced resource/key.")
		}
		if ref.Name != "" && !environmentName.MatchString(ref.Name) || ref.ResourceName != "" && len(kubevalidation.IsDNS1123Subdomain(ref.ResourceName)) != 0 || ref.Key != "" && len(kubevalidation.IsConfigMapKey(ref.Key)) != 0 {
			return fmt.Errorf("invalid environment reference")
		}
		if ref.Kind != "secret" && ref.Kind != "config-map" {
			return fmt.Errorf("choose a secret or config-map reference")
		}
		if ref.Name != "" && seen[ref.Name] {
			return fmt.Errorf("duplicate environment name")
		}
		seen[ref.Name] = true
		if service.Telemetry != nil && (ref.Name == "OTEL_EXPORTER_OTLP_ENDPOINT" || ref.Name == "OTEL_EXPORTER_OTLP_PROTOCOL" || ref.Name == "OTEL_SERVICE_NAME") {
			return fmt.Errorf("environment reference conflicts with explicit telemetry binding")
		}
		if service.Telemetry != nil && service.Telemetry.ClientTLS != nil && (strings.HasPrefix(ref.Name, "OTEL_") || ref.Name == "LUCI_OTEL_INSTANCE") {
			return fmt.Errorf("managed telemetry owns the explicit SDK/exporter configuration; remove conflicting environment references")
		}
	}
	if len(service.Environment) == 0 {
		service.Environment = nil
	} else {
		sort.Slice(service.Environment, func(i, j int) bool { return service.Environment[i].Name < service.Environment[j].Name })
	}
	if service.Health == nil {
		return nil
	} // legacy manifests/digests remain unchanged
	if service.Health.Startup == nil && service.Health.Readiness == nil {
		decision(field+".health", "probe-required", "Choose a startup/readiness probe or remove health configuration.")
	}
	for _, row := range []struct {
		name  string
		probe *Probe
	}{{"startup", service.Health.Startup}, {"readiness", service.Health.Readiness}} {
		probe := row.probe
		if probe == nil {
			continue
		}
		location := field + ".health." + row.name
		if probe.Kind != "http" && probe.Kind != "tcp" {
			return fmt.Errorf("probe must use HTTP or TCP on the declared container port")
		}
		if probe.InitialDelaySeconds < 0 || probe.InitialDelaySeconds > 600 || probe.PeriodSeconds < 0 || probe.PeriodSeconds > 300 || probe.TimeoutSeconds < 0 || probe.TimeoutSeconds > 30 || probe.FailureThreshold < 0 || probe.FailureThreshold > 120 {
			return fmt.Errorf("probe timing is outside bounded limits")
		}
		if probe.PeriodSeconds == 0 || probe.TimeoutSeconds == 0 || probe.FailureThreshold == 0 {
			decision(location, "probe-timing-required", "Choose a positive period, timeout and failure threshold.")
		}
		if probe.TimeoutSeconds > probe.PeriodSeconds && probe.PeriodSeconds != 0 {
			return fmt.Errorf("probe timeout exceeds its period")
		}
		if probe.Kind == "tcp" {
			if probe.Path != "" {
				return fmt.Errorf("TCP probes have no HTTP path")
			}
			continue
		}
		if probe.Path == "" {
			decision(location+".path", "probe-path-required", "Choose a relative HTTP health path on this container.")
			continue
		}
		parsed, err := url.ParseRequestURI(probe.Path)
		if err != nil || !text(probe.Path, 512) || !text(parsed.Path, 512) || !strings.HasPrefix(probe.Path, "/") || strings.HasPrefix(probe.Path, "//") || strings.HasPrefix(parsed.Path, "//") || strings.ContainsAny(parsed.Path, "?#\\") || strings.ContainsAny(probe.Path, "?#\\") || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
			return fmt.Errorf("HTTP probe requires a relative path without host, query, fragment or credentials")
		}
	}
	return nil
}

// RuntimeReferenceCount bounds actual distinct imported resource observations,
// independent of the number of services sharing a reference.
func RuntimeReferenceCount(blueprint Blueprint) int {
	seen := map[string]bool{}
	for _, service := range blueprint.Services {
		for name := range ServiceSecretKeys(service) {
			seen["secret/"+name] = true
		}
		if service.Model != nil {
			seen["persistent-volume-claim/"+service.Model.ClaimName] = true
		}
		if service.TelemetryStorage != nil {
			seen["persistent-volume-claim/"+service.TelemetryStorage.ClaimName] = true
		}
		for _, ref := range service.Environment {
			seen[ref.Kind+"/"+ref.ResourceName] = true
		}
	}
	if blueprint.PublicRoute != nil {
		seen["secret/"+blueprint.PublicRoute.TLSSecretName] = true
		seen["ingress-class/"+blueprint.PublicRoute.IngressClassName] = true
	}
	return len(seen)
}
