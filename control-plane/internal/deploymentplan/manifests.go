package deploymentplan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kubernetes accepts JSON manifests. Keeping them as JSON avoids a YAML
// dependency and preserves a deterministic byte representation for inspection.
func manifests(b Blueprint, planDigest string) ([]Artifact, error) {
	out := []Artifact{}
	projectLabel := digest([]byte(b.ProjectID))[:12]
	names := map[string]bool{}
	for _, s := range b.Services {
		name := s.ID
		if len(name) > 49 {
			name = strings.TrimRight(name[:49], "-")
		}
		name += "-" + digest([]byte(b.ProjectID + "\x00" + s.ID))[:12]
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
		if s.Telemetry != nil {
			container["env"] = []any{
				map[string]string{"name": "OTEL_EXPORTER_OTLP_ENDPOINT", "value": s.Telemetry.Endpoint},
				map[string]string{"name": "OTEL_EXPORTER_OTLP_PROTOCOL", "value": "http/protobuf"},
				map[string]string{"name": "OTEL_SERVICE_NAME", "value": name},
			}
		}
		podSpec := map[string]any{"automountServiceAccountToken": false, "containers": []any{container}}
		if s.Model != nil {
			mount := map[string]any{"name": "model", "mountPath": s.Model.MountPath, "readOnly": true}
			if s.Model.RelativePath != "" {
				mount["subPath"] = s.Model.RelativePath
			}
			container["volumeMounts"] = []any{mount}
			podSpec["volumes"] = []any{map[string]any{"name": "model", "persistentVolumeClaim": map[string]any{"claimName": s.Model.ClaimName, "readOnly": true}}}
		}
		deployment := map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment", "metadata": metadata(),
			"spec": map[string]any{"replicas": s.Replicas, "selector": map[string]any{"matchLabels": labels}, "template": map[string]any{"metadata": map[string]any{"labels": labels, "annotations": annotations}, "spec": podSpec}},
		}
		service := map[string]any{
			"apiVersion": "v1", "kind": "Service", "metadata": metadata(),
			"spec": map[string]any{"type": "ClusterIP", "selector": labels, "ports": []any{map[string]any{"name": "service", "port": s.Port, "targetPort": "service", "protocol": "TCP"}}},
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
	return out, nil
}
