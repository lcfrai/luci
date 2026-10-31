package deploymentkube

import (
	"context"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"k8s.io/client-go/kubernetes/fake"
	"strings"
	"testing"
)

func storedVectorFixture() (prometheusVector, Request, deploymentplan.Service) {
	var v prometheusVector
	_ = json.Unmarshal([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"luci_product_events_total","service_namespace":"p-one","service_name":"app","service_instance_id":"pod-original","event":"activation","outcome":"success"},"value":[1793400000,"7"]}]}}`), &v)
	request := Request{Identity: Identity{ProjectID: "p-one"}}
	collector := deploymentplan.Service{ID: "collector", Collector: &deploymentplan.CollectorConfiguration{Events: []deploymentplan.ProductEvent{{ID: "activation", Description: "Core outcome"}}}}
	return v, request, collector
}
func TestStoredCountersNeverConvertOldPodSamplesToCurrentOrLifetimeCounts(t *testing.T) {
	v, r, c := storedVectorFixture()
	samples, omitted, err := parseStoredProductVector(v, r, c, map[string]string{"pod-original": "app"})
	if err != nil || omitted || len(samples) != 1 || samples[0].ReportedCount != 7 {
		t.Fatal("original attributed sample missing", err)
	}
	samples, omitted, err = parseStoredProductVector(v, r, c, map[string]string{"new-pod": "app"})
	if err != nil || !omitted || len(samples) != 0 {
		t.Fatal("old instance counted as current")
	}
	for _, field := range []string{"event", "outcome", "project", "count"} {
		t.Run(field, func(t *testing.T) {
			v, r, c := storedVectorFixture()
			switch field {
			case "event":
				v.Data.Results[0].Metric["event"] = "email@example.com"
			case "outcome":
				v.Data.Results[0].Metric["outcome"] = "unknown"
			case "project":
				v.Data.Results[0].Metric["service_namespace"] = "foreign"
			case "count":
				v.Data.Results[0].Value[1] = json.RawMessage(`"NaN"`)
			}
			if _, _, e := parseStoredProductVector(v, r, c, map[string]string{"pod-original": "app"}); e == nil {
				t.Fatal("invalid sample accepted")
			}
		})
	}
}
func TestStorageRequiresBoundDedicatedWritableCapacityAndClosedTunnel(t *testing.T) {
	s := deploymentplan.Service{ID: "metrics", Replicas: 1, DeploymentStrategy: "recreate", TelemetryStorage: &deploymentplan.TelemetryStorage{CapacityGiB: 10}}
	ref := ReferenceSnapshot{Phase: "Bound", VolumeMode: "Filesystem", VolumeName: "pv-original", AccessModes: []string{"ReadWriteOnce"}, CapacityBytes: "10737418240"}
	if !claimCompatible(ref, []deploymentplan.Service{s}) {
		t.Fatal("valid writable original refused")
	}
	ref.CapacityBytes = "1"
	if claimCompatible(ref, []deploymentplan.Service{s}) {
		t.Fatal("undersized volume accepted")
	}
	ref.CapacityBytes = "10737418240"
	ref.AccessModes = []string{"ReadOnlyMany"}
	if claimCompatible(ref, []deploymentplan.Service{s}) {
		t.Fatal("read-only storage accepted")
	}
	if _, _, err := storageHTTPClient(context.Background(), Target{Client: fake.NewSimpleClientset()}, "original", "owned.apps.svc", 9090, []byte("not-a-ca"), []byte("not-a-cert"), []byte("not-a-key")); err == nil {
		t.Fatal("unbound fake became a generic tunnel")
	}
	if WithStorageForwardClient(nil, nil) != nil {
		t.Fatal("nil client gained capability")
	}
}
func TestStoredSourceCannotSubstituteAnotherCollectorDestination(t *testing.T) {
	r := Request{Identity: Identity{ProjectID: "p-one", PlanDigest: strings.Repeat("a", 64)}}
	source := &StoredTelemetrySource{ServiceID: "not-in-plan", Profile: deploymentplan.PrometheusStorageProfile, PodUID: "pod", ConfigUID: "cfg", ConfigVersion: "1"}
	if validStoredTelemetrySource(source, r, nil) {
		t.Fatal("foreign store accepted")
	}
}
