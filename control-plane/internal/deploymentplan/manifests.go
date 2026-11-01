package deploymentplan

import (
	"encoding/json"
	"fmt"
)

// Kubernetes accepts JSON manifests. Keeping them as JSON avoids a YAML
// dependency and preserves a deterministic byte representation for inspection.
func manifests(b Blueprint, planDigest string) ([]Artifact, error) {
	out := []Artifact{}
	projectLabel := digest([]byte(b.ProjectID))[:12]
	names := map[string]bool{}
	for _, s := range b.Services {
		name := ResourceName(b.ProjectID, s.ID)
		if names[name] {
			return nil, fmt.Errorf("generated Kubernetes resource name collision")
		}
		names[name] = true
		labels := map[string]string{"app.kubernetes.io/managed-by": "luci-blueprint", "luci.ai/project": projectLabel, "luci.ai/component": s.ID}
		annotations := map[string]string{"luci.ai/plan-digest": planDigest, "luci.ai/source-commit": b.Source.Commit}
		metadata := func() map[string]any {
			return map[string]any{"name": name, "namespace": b.Infrastructure.Namespace, "labels": labels, "annotations": annotations}
		}
		requests := map[string]string{"cpu": fmt.Sprintf("%dm", s.Resources.RequestMilliCPU), "memory": fmt.Sprintf("%dMi", s.Resources.RequestMemoryMiB)}
		limits := map[string]string{"cpu": fmt.Sprintf("%dm", s.Resources.LimitMilliCPU), "memory": fmt.Sprintf("%dMi", s.Resources.LimitMemoryMiB)}
		if s.Resources.GPUCount > 0 {
			requests["nvidia.com/gpu"] = fmt.Sprint(s.Resources.GPUCount)
			limits["nvidia.com/gpu"] = fmt.Sprint(s.Resources.GPUCount)
		}
		container := map[string]any{
			"name": s.ID, "image": s.Image, "imagePullPolicy": "IfNotPresent",
			"ports":           []any{map[string]any{"name": "service", "containerPort": s.Port, "protocol": "TCP"}},
			"resources":       map[string]any{"requests": requests, "limits": limits},
			"securityContext": map[string]any{"allowPrivilegeEscalation": false, "capabilities": map[string]any{"drop": []string{"ALL"}}, "seccompProfile": map[string]any{"type": "RuntimeDefault"}},
		}
		if len(s.Command) > 0 {
			container["command"] = s.Command
		}
		if len(s.Args) > 0 {
			container["args"] = s.Args
		}
		environment := []any{}
		for _, ref := range s.Environment {
			source := "secretKeyRef"
			if ref.Kind == "config-map" {
				source = "configMapKeyRef"
			}
			environment = append(environment, map[string]any{"name": ref.Name, "valueFrom": map[string]any{source: map[string]any{"name": ref.ResourceName, "key": ref.Key, "optional": false}}})
		}
		if s.Telemetry != nil {
			environment = append(environment, []any{
				map[string]string{"name": "OTEL_EXPORTER_OTLP_ENDPOINT", "value": TelemetryEndpoint(b, s)},
				map[string]string{"name": "OTEL_EXPORTER_OTLP_PROTOCOL", "value": "http/protobuf"},
				map[string]string{"name": "OTEL_SERVICE_NAME", "value": name},
			}...)
		}
		if len(environment) > 0 {
			container["env"] = environment
		}
		if s.Health != nil {
			for _, probe := range []struct {
				name  string
				value *Probe
			}{{"startupProbe", s.Health.Startup}, {"readinessProbe", s.Health.Readiness}} {
				if probe.value == nil {
					continue
				}
				value := probe.value
				result := map[string]any{"initialDelaySeconds": value.InitialDelaySeconds, "periodSeconds": value.PeriodSeconds, "timeoutSeconds": value.TimeoutSeconds, "failureThreshold": value.FailureThreshold, "successThreshold": 1}
				if value.Kind == "http" {
					result["httpGet"] = map[string]any{"path": value.Path, "port": s.Port, "scheme": "HTTP"}
				} else {
					result["tcpSocket"] = map[string]any{"port": s.Port}
				}
				container[probe.name] = result
			}
		}
		podSpec := map[string]any{"automountServiceAccountToken": false, "containers": []any{container}}
		if len(s.ImagePullSecrets) > 0 {
			pulls := []any{}
			for _, name := range s.ImagePullSecrets {
				pulls = append(pulls, map[string]string{"name": name})
			}
			podSpec["imagePullSecrets"] = pulls
		}
		if s.Resources.GPUCount > 0 {
			podSpec["tolerations"] = []any{map[string]string{"key": "sku", "operator": "Equal", "value": "gpu", "effect": "NoSchedule"}}
		}
		if s.Model != nil {
			if s.Model.AccessGroup == 1000 {
				podSpec["securityContext"] = map[string]any{"fsGroup": int64(1000), "fsGroupChangePolicy": "OnRootMismatch"}
			}
			mount := map[string]any{"name": "model", "mountPath": s.Model.MountPath, "readOnly": true}
			if s.Model.RelativePath != "" {
				mount["subPath"] = s.Model.RelativePath
			}
			container["volumeMounts"] = []any{mount}
			podSpec["volumes"] = []any{map[string]any{"name": "model", "persistentVolumeClaim": map[string]any{"claimName": s.Model.ClaimName, "readOnly": true}}}
		}
		if err := configureTelemetryContainer(b, s, planDigest, container, podSpec); err != nil {
			return nil, err
		}
		if s.TelemetryStorage != nil {
			config, err := telemetryStorageManifest(b, s, planDigest, container, podSpec)
			if err != nil {
				return nil, err
			}
			out = append(out, config)
		}
		deployment := map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment", "metadata": metadata(),
			"spec": map[string]any{"replicas": s.Replicas, "selector": map[string]any{"matchLabels": labels}, "template": map[string]any{"metadata": map[string]any{"labels": labels, "annotations": annotations}, "spec": podSpec}},
		}
		if s.DeploymentStrategy == "recreate" {
			deployment["spec"].(map[string]any)["strategy"] = map[string]any{"type": "Recreate"}
		}
		service := map[string]any{
			"apiVersion": "v1", "kind": "Service", "metadata": metadata(),
			"spec": map[string]any{"type": "ClusterIP", "selector": labels, "ports": []any{map[string]any{"name": "service", "port": s.Port, "targetPort": "service", "protocol": "TCP"}}},
		}
		if s.Collector != nil {
			service["spec"].(map[string]any)["ports"] = []any{map[string]any{"name": "service", "port": s.Port, "targetPort": "service", "protocol": "TCP"}, map[string]any{"name": "metrics", "port": CollectorMetricsPort, "targetPort": "metrics", "protocol": "TCP"}}
		}
		if s.TelemetryStorage != nil && s.TelemetryStorage.Profile == TempoStorageProfile {
			service["spec"].(map[string]any)["ports"] = []any{map[string]any{"name": "service", "port": 3200, "targetPort": "service", "protocol": "TCP"}, map[string]any{"name": "otlp", "port": 4318, "targetPort": "otlp", "protocol": "TCP"}}
		}
		for _, item := range []struct {
			suffix string
			value  map[string]any
		}{{"deployment", deployment}, {"service", service}} {
			raw, err := json.MarshalIndent(item.value, "", "  ")
			if err != nil {
				return nil, err
			}
			raw = append(raw, '\n')
			out = append(out, Artifact{Name: name + "." + item.suffix + ".json", MediaType: "application/json", SHA256: digest(raw), Content: string(raw)})
		}
	}
	if b.PublicRoute != nil && b.PublicRoute.Profile != ManagedPublicRouteProfile {
		route, err := publicIngressArtifact(b, planDigest)
		if err != nil {
			return nil, err
		}
		out = append(out, route)
	}
	return out, nil
}
