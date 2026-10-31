package deploymentplan

import (
	"encoding/json"
	"fmt"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"regexp"
	"strings"
)

const PrometheusStorageProfile = "prometheus-3.15.1"
const TempoStorageProfile = "tempo-2.10.10-azure"

// This closed preset creates no identity, downloads no image and carries no
// credential values. The selected immutable image must actually match profile.
type TelemetryStorage struct {
	Profile        string                `json:"profile"`
	ClaimName      string                `json:"claimName"`
	CapacityGiB    int                   `json:"capacityGiB"`
	RetentionHours int                   `json:"retentionHours"`
	ServerTLS      TelemetryTLSReference `json:"serverTLS"`
	QueryTLS       TelemetryTLSReference `json:"queryTLS"`
	Azure          *TelemetryAzureBlocks `json:"azure,omitempty"`
}
type TelemetryAzureBlocks struct {
	AccountName   string                          `json:"accountName"`
	ContainerName string                          `json:"containerName"`
	AccountKey    TelemetryAuthorizationReference `json:"accountKey"`
}
type OwnedTelemetryDestination struct {
	MetricsServiceID string                `json:"metricsServiceId"`
	TracesServiceID  string                `json:"tracesServiceId"`
	ClientTLS        TelemetryTLSReference `json:"clientTLS"`
}

var blobAccountName = regexp.MustCompile(`^[a-z0-9]{3,24}$`)
var blobContainerName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])$`)

func ValidTelemetryBlob(v TelemetryAzureBlocks) bool {
	return blobAccountName.MatchString(v.AccountName) && blobContainerName.MatchString(v.ContainerName) && !strings.Contains(v.ContainerName, "--") && len(kubevalidation.IsDNS1123Subdomain(v.AccountKey.ResourceName)) == 0 && v.AccountKey.ResourceName != "" && v.AccountKey.Key != "" && len(kubevalidation.IsConfigMapKey(v.AccountKey.Key)) == 0
}
func telemetryStorageService(b Blueprint, id, profile string) *Service {
	for i := range b.Services {
		v := &b.Services[i]
		if v.ID == id && v.TelemetryStorage != nil && v.TelemetryStorage.Profile == profile {
			return v
		}
	}
	return nil
}
func storageDNS(b Blueprint, s Service) string {
	return ResourceName(b.ProjectID, s.ID) + "." + b.Infrastructure.Namespace + ".svc"
}
func validateTelemetryStorage(b *Blueprint, decision func(string, string, string)) error {
	profiles := map[string]bool{}
	for i := range b.Services {
		s := &b.Services[i]
		v := s.TelemetryStorage
		if v == nil {
			if s.Kind == "metrics-store" || s.Kind == "trace-store" {
				return fmt.Errorf("storage service requires its closed telemetry profile")
			}
			continue
		}
		if v.ClaimName == "" {
			decision("services."+s.ID+".telemetryStorage.claimName", "storage-claim-required", "Choose an existing Bound writable PVC or review new Immediate-binding storage through resource preparation; no fictional claim name is required.")
		}
		if b.Infrastructure.Kind != "kubernetes" || s.Replicas != 1 || s.DeploymentStrategy != "recreate" || s.Model != nil || s.Collector != nil || s.Telemetry != nil || s.Health != nil || len(s.Environment) != 0 || len(s.Command) != 0 || len(s.Args) != 0 || s.Resources.GPUCount != 0 || s.Resources.RequestMemoryMiB < 256 || v.ClaimName != "" && len(kubevalidation.IsDNS1123Subdomain(v.ClaimName)) != 0 || v.CapacityGiB < 1 || v.CapacityGiB > 1024 || v.RetentionHours < 1 || v.RetentionHours > 744 || !validTLSReference(v.ServerTLS) || !validTLSReference(v.QueryTLS) || profiles[v.Profile] {
			return fmt.Errorf("choose one fixed metrics and trace store, immutable compatible images, existing writable PVCs, explicit retention, mTLS and single-replica Recreate")
		}
		profiles[v.Profile] = true
		switch v.Profile {
		case PrometheusStorageProfile:
			if s.Kind != "metrics-store" || s.Port != 9090 || v.Azure != nil {
				return fmt.Errorf("Prometheus profile requires metrics-store port9090 without Blob settings")
			}
		case TempoStorageProfile:
			if s.Kind != "trace-store" || s.Port != 3200 || v.Azure == nil || !ValidTelemetryBlob(*v.Azure) {
				return fmt.Errorf("Tempo requires trace-store port3200, WAL PVC and exact Azure Blob account/container/private Secret key; local blocks are not a production preset")
			}
		default:
			return fmt.Errorf("unknown telemetry storage profile")
		}
	}
	for i := range b.Services {
		c := b.Services[i].Collector
		if c == nil || c.OwnedDestination == nil {
			continue
		}
		d := c.OwnedDestination
		if c.Destination.ResourceID != "" || c.Destination.Endpoint != "" || c.Destination.Authorization != nil || !validTLSReference(d.ClientTLS) || telemetryStorageService(*b, d.MetricsServiceID, PrometheusStorageProfile) == nil || telemetryStorageService(*b, d.TracesServiceID, TempoStorageProfile) == nil {
			return fmt.Errorf("owned Collector destination requires the exact metrics/trace services, client mTLS and no external destination")
		}
		b.Services[i].DependsOn = uniqueSorted(append(b.Services[i].DependsOn, d.MetricsServiceID, d.TracesServiceID))
	}
	return nil
}

func TelemetryStorageConfig(b Blueprint, s Service) (map[string]string, error) {
	v := s.TelemetryStorage
	if v == nil {
		return nil, fmt.Errorf("storage profile missing")
	}
	files := map[string]string{}
	if v.Profile == PrometheusStorageProfile {
		// Promote only the three generated identity attributes. No arbitrary SDK
		// labels are used by Luci's closed read query or interpreted as business data.
		files["prometheus.yml"] = "global:\n  scrape_interval: 30s\notlp:\n  promote_resource_attributes:\n    - service.name\n    - service.namespace\n    - service.instance.id\nstorage:\n  tsdb:\n    out_of_order_time_window: 30m\n"
		files["web.yml"] = "tls_server_config:\n  cert_file: /luci-store/server/cert.pem\n  key_file: /luci-store/server/key.pem\n  client_auth_type: RequireAndVerifyClientCert\n  client_ca_file: /luci-store/server/ca.pem\n  min_version: TLS12\n"
	} else if v.Profile == TempoStorageProfile && v.Azure != nil {
		// Azure account key is resolved by Tempo's documented env expansion from a
		// private Secret, never interpolated by this compiler or returned as text.
		config := map[string]any{
			"multitenancy_enabled": false,
			"server":               map[string]any{"http_listen_port": 3200, "grpc_listen_address": "127.0.0.1", "grpc_listen_port": 9095, "tls_min_version": "VersionTLS12", "http_tls_config": map[string]any{"cert_file": "/luci-store/server/cert.pem", "key_file": "/luci-store/server/key.pem", "client_auth_type": "RequireAndVerifyClientCert", "client_ca_file": "/luci-store/server/ca.pem"}},
			"distributor":          map[string]any{"receivers": map[string]any{"otlp": map[string]any{"protocols": map[string]any{"http": map[string]any{"endpoint": "0.0.0.0:4318", "tls": map[string]any{"cert_file": "/luci-store/server/cert.pem", "key_file": "/luci-store/server/key.pem", "client_ca_file": "/luci-store/server/ca.pem", "min_version": "1.2"}}}}}},
			"ingester":             map[string]any{"max_block_duration": "5m", "lifecycler": map[string]any{"address": "127.0.0.1", "ring": map[string]any{"replication_factor": 1, "kvstore": map[string]string{"store": "inmemory"}}}},
			"compactor":            map[string]any{"compaction": map[string]any{"block_retention": fmt.Sprintf("%dh", v.RetentionHours)}},
			"storage":              map[string]any{"trace": map[string]any{"backend": "azure", "wal": map[string]any{"path": "/luci-data/wal"}, "azure": map[string]any{"storage_account_name": v.Azure.AccountName, "container_name": v.Azure.ContainerName, "storage_account_key": "${LUCI_TEMPO_AZURE_ACCOUNT_KEY}"}}},
			"usage_report":         map[string]bool{"reporting_enabled": false},
		}
		raw, e := json.MarshalIndent(config, "", "  ")
		if e != nil {
			return nil, e
		}
		files["tempo.json"] = string(raw) + "\n"
	} else {
		return nil, fmt.Errorf("unsupported telemetry storage configuration")
	}
	return files, nil
}

func telemetryStorageManifest(b Blueprint, s Service, planDigest string, container, pod map[string]any) (Artifact, error) {
	v := s.TelemetryStorage
	files, err := TelemetryStorageConfig(b, s)
	if err != nil {
		return Artifact{}, err
	}
	name := ResourceName(b.ProjectID, s.ID) + "-config"
	// A stable compiler-derived ConfigMap is an ordinary reviewed journalled
	// resource. User-controlled files and credential values never enter it.
	labels := map[string]string{"app.kubernetes.io/managed-by": "luci-blueprint", "luci.ai/project": digest([]byte(b.ProjectID))[:12], "luci.ai/component": s.ID}
	value := map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": name, "namespace": b.Infrastructure.Namespace, "labels": labels, "annotations": map[string]string{"luci.ai/plan-digest": planDigest, "luci.ai/source-commit": b.Source.Commit}}, "data": files}
	mounts, _ := container["volumeMounts"].([]any)
	mounts = append(mounts, map[string]any{"name": "luci-store-config", "mountPath": "/luci-store/config", "readOnly": true}, map[string]any{"name": "luci-store-data", "mountPath": "/luci-data"}, map[string]any{"name": "luci-store-server", "mountPath": "/luci-store/server", "readOnly": true})
	container["volumeMounts"] = mounts
	volumes, _ := pod["volumes"].([]any)
	volumes = append(volumes, map[string]any{"name": "luci-store-config", "configMap": map[string]any{"name": name, "optional": false}}, map[string]any{"name": "luci-store-data", "persistentVolumeClaim": map[string]any{"claimName": v.ClaimName, "readOnly": false}}, map[string]any{"name": "luci-store-server", "secret": map[string]any{"secretName": v.ServerTLS.ResourceName, "optional": false, "defaultMode": 292, "items": []any{map[string]string{"key": v.ServerTLS.CAKey, "path": "ca.pem"}, map[string]string{"key": v.ServerTLS.CertKey, "path": "cert.pem"}, map[string]string{"key": v.ServerTLS.KeyKey, "path": "key.pem"}}}})
	pod["volumes"] = volumes
	pod["securityContext"] = map[string]any{"runAsNonRoot": true, "runAsUser": 10001, "runAsGroup": 10001, "fsGroup": 10001, "fsGroupChangePolicy": "OnRootMismatch"}
	if v.Profile == PrometheusStorageProfile {
		container["args"] = []string{"--config.file=/luci-store/config/prometheus.yml", "--web.config.file=/luci-store/config/web.yml", "--web.listen-address=:9090", "--web.enable-otlp-receiver", "--storage.tsdb.path=/luci-data/tsdb", fmt.Sprintf("--storage.tsdb.retention.time=%dh", v.RetentionHours), fmt.Sprintf("--storage.tsdb.retention.size=%dMB", v.CapacityGiB*1024*80/100)}
	} else {
		container["args"] = []string{"-target=all", "-config.file=/luci-store/config/tempo.json", "-config.expand-env=true"}
		env, _ := container["env"].([]any)
		container["env"] = append(env, map[string]any{"name": "LUCI_TEMPO_AZURE_ACCOUNT_KEY", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": v.Azure.AccountKey.ResourceName, "key": v.Azure.AccountKey.Key, "optional": false}}})
		container["ports"] = []any{map[string]any{"name": "service", "containerPort": 3200, "protocol": "TCP"}, map[string]any{"name": "otlp", "containerPort": 4318, "protocol": "TCP"}}
	}
	container["readinessProbe"] = map[string]any{"tcpSocket": map[string]int{"port": s.Port}, "periodSeconds": 10, "timeoutSeconds": 2, "failureThreshold": 3}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Artifact{}, err
	}
	raw = append(raw, '\n')
	return Artifact{Name: name + ".configmap.json", MediaType: "application/json", SHA256: digest(raw), Content: string(raw)}, nil
}
