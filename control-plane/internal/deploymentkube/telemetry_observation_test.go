package deploymentkube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func telemetryFixture(t *testing.T) (*Connector, *fake.Clientset, *memoryJournal, Request, string) {
	t.Helper()
	c, client, journal, request := workloadFixture(t)
	b := request.Blueprint
	dependencies := map[string]string{"@opentelemetry/api": "1.9.0", "@opentelemetry/sdk-node": "0.200.0", "@opentelemetry/sdk-metrics": "2.0.1", "@opentelemetry/resources": "2.0.1", "@opentelemetry/exporter-trace-otlp-http": "0.200.0", "@opentelemetry/exporter-metrics-otlp-http": "0.200.0"}
	b.ExternalResources = []deploymentplan.ExternalResource{{ID: "analytics", Kind: "analytics", Reference: "owned-backend"}}
	tlsRef := func(name string) *deploymentplan.TelemetryTLSReference {
		return &deploymentplan.TelemetryTLSReference{ResourceName: name, CAKey: "ca", CertKey: "cert", KeyKey: "key"}
	}
	b.Services[0].Telemetry = &deploymentplan.TelemetryBinding{ResourceID: "collector", ClientTLS: tlsRef("client-tls"), Instrumentation: &deploymentplan.NodeInstrumentation{Runtime: "nodejs", Dependencies: dependencies}}
	b.Services = append(b.Services, deploymentplan.Service{ID: "collector", Kind: "collector", Image: "registry.example/collector@sha256:" + strings.Repeat("d", 64), Port: 4318, Replicas: 1, Resources: deploymentplan.ResourceBudget{RequestMilliCPU: 100, LimitMilliCPU: 500, RequestMemoryMiB: 128, LimitMemoryMiB: 256}, Collector: &deploymentplan.CollectorConfiguration{Profile: deploymentplan.CollectorProfile, Destination: deploymentplan.TelemetryDestination{ResourceID: "analytics", Endpoint: "https://owned.example/otlp"}, ReceiverTLS: *tlsRef("server-tls"), Events: []deploymentplan.ProductEvent{{ID: "activation", Description: "Core outcome completed"}}}})
	plan, err := deploymentplan.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	request.Blueprint = plan.Blueprint
	request.Identity.PlanDigest = plan.Digest
	request.Expected = nil
	for _, artifact := range plan.Artifacts {
		var row struct {
			Kind     string
			Metadata struct{ Name string }
		}
		if json.Unmarshal([]byte(artifact.Content), &row) != nil {
			t.Fatal("invalid compiler artifact")
		}
		request.Expected = append(request.Expected, ExpectedResource{Kind: row.Kind, Name: row.Metadata.Name})
	}
	for _, name := range []string{"server-tls", "client-tls"} {
		if err := client.Tracker().Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "apps", UID: types.UID(name + "-uid"), ResourceVersion: "1"}, Data: map[string][]byte{"ca": []byte("fixture-only-ca"), "cert": []byte("fixture-only-certificate"), "key": []byte("fixture-only-key")}}); err != nil {
			t.Fatal(err)
		}
	}
	target, err := c.target(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.References, err = ReadReferences(context.Background(), target, request.Blueprint, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var collectorPod *corev1.Pod
	var collectorService *corev1.Service
	for _, service := range request.Blueprint.Services {
		name := deploymentplan.ResourceName(b.ProjectID, service.ID)
		deployment, err := client.AppsV1().Deployments("apps").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		set := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "rs-" + service.ID, Namespace: "apps", UID: types.UID("rs-" + service.ID), ResourceVersion: "1", Labels: deployment.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: pointer(true)}}}, Spec: appsv1.ReplicaSetSpec{Template: *deployment.Spec.Template.DeepCopy()}}
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-" + service.ID, Namespace: "apps", UID: types.UID("pod-" + service.ID), ResourceVersion: "1", CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute)), Labels: deployment.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: set.Name, UID: set.UID, Controller: pointer(true)}}}, Spec: *set.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIPs: []corev1.PodIP{{IP: "10.0.0.3"}}, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Name: service.ID, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-time.Minute))}}}}}}
		if err = client.Tracker().Add(set); err != nil {
			t.Fatal(err)
		}
		if err = client.Tracker().Add(pod); err != nil {
			t.Fatal(err)
		}
		if service.Collector != nil {
			collectorPod = pod
			collectorService, err = client.CoreV1().Services("apps").Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	sliceName := "collector-endpoints"
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: sliceName, Namespace: "apps", UID: "slice-one", ResourceVersion: "1", Labels: map[string]string{"kubernetes.io/service-name": collectorService.Name}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: collectorService.Name, UID: collectorService.UID, Controller: pointer(true)}}}, AddressType: discoveryv1.AddressTypeIPv4, Ports: []discoveryv1.EndpointPort{{Name: pointer("metrics"), Protocol: pointer(corev1.ProtocolTCP), Port: pointer(int32(deploymentplan.CollectorMetricsPort))}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.3"}, Conditions: discoveryv1.EndpointConditions{Ready: pointer(true)}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: collectorPod.Name, Namespace: "apps", UID: collectorPod.UID}}}}
	if err := client.Tracker().Add(slice); err != nil {
		t.Fatal(err)
	}
	legacy := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: collectorService.Name, Namespace: "apps", UID: "legacy-endpoints-one", ResourceVersion: "1", Labels: collectorService.Labels}, Subsets: []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.3", TargetRef: &corev1.ObjectReference{Kind: "Pod", Namespace: "apps", Name: collectorPod.Name, UID: collectorPod.UID}}}, Ports: []corev1.EndpointPort{{Name: "service", Port: 4318, Protocol: corev1.ProtocolTCP}, {Name: "metrics", Port: deploymentplan.CollectorMetricsPort, Protocol: corev1.ProtocolTCP}}}}}
	if err := client.Tracker().Add(legacy); err != nil {
		t.Fatal(err)
	}
	return c, client, journal, request, sliceName
}
func productMetric(request Request, count string) string {
	return fmt.Sprintf("luci_product_events_total{job=\"p-one/web\",instance=\"pod-web\",event=\"activation\",outcome=\"success\",luci_project=\"p-one\",luci_plan=\"%s\"} %s\n", request.Identity.PlanDigest, count)
}

func TestTelemetryCollectorObservationUsesOriginalOwnedLineageWithoutApplyingWork(t *testing.T) {
	c, client, journal, request, _ := telemetryFixture(t)
	before := writes(client)
	reads := 0
	count := "7"
	c.metricsRead = func(ctx context.Context, _ Target, name string) (io.ReadCloser, error) {
		reads++
		if name != deploymentplan.ResourceName(request.Blueprint.ProjectID, "collector") {
			t.Fatal("proxy changed resource")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("proxy stream lacks original bounded deadline")
		}
		return io.NopCloser(strings.NewReader(productMetric(request, count))), nil
	}
	op, err := c.ReadTelemetry(context.Background(), request.Identity)
	if err != nil || op.Telemetry == nil || op.Telemetry.State != "recorded" || op.Telemetry.Samples[0].ReportedCount != 7 || op.Telemetry.Continuity != "baseline" || op.ApplicationHealthVerified || writes(client) != before || reads != 1 {
		t.Fatal("reported sample missing or read mutated workloads", op.Telemetry, err)
	}
	if err := ValidateOperation(op); err != nil {
		t.Fatal("retained attributed sample invalid", err)
	}
	corrupt := op
	copyObservation := *op.Telemetry
	corrupt.Telemetry = &copyObservation
	for _, resource := range op.Resources {
		if resource.Kind == "Service" && resource.Name == deploymentplan.ResourceName(request.Blueprint.ProjectID, "web") {
			copyObservation.ServiceUID = resource.ObservedUID
		}
	}
	copyObservation.EvidenceDigest = telemetryDigest(copyObservation)
	if ValidateOperation(corrupt) == nil {
		t.Fatal("another original service UID was substituted for the Collector")
	}
	count = "2"
	again, err := c.ReadTelemetry(context.Background(), request.Identity)
	if err != nil || again.Telemetry.Continuity != "reset" || again.Telemetry.Samples[0].ReportedCount != 2 || journal.operation.Telemetry == nil {
		t.Fatal("decrease was treated as continuous/lifetime data", err)
	}
	pod, err := client.CoreV1().Pods("apps").Get(context.Background(), "pod-web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod.Status.ContainerStatuses[0].Ready = false
	pod.Status.Conditions[0].Status = corev1.ConditionFalse
	if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), pod, "apps"); err != nil {
		t.Fatal(err)
	}
	partial, err := c.ReadTelemetry(context.Background(), request.Identity)
	if err != nil || partial.Telemetry == nil || partial.Telemetry.State != "partial" || !partial.Telemetry.Omitted || partial.Telemetry.Samples[0].ReportedCount != 2 {
		t.Fatal("running but not-ready app Pod implied complete/readied telemetry", err)
	}
}
func TestTelemetryObservationRefusesRotatedReferencesOrChangedProxyOwnership(t *testing.T) {
	for _, scenario := range []string{"rotated-secret", "foreign-endpoint", "changed-after-read", "pagination", "foreign-legacy-endpoint"} {
		t.Run(scenario, func(t *testing.T) {
			c, client, journal, request, sliceName := telemetryFixture(t)
			reads := 0
			mutateSlice := func() {
				row, err := client.DiscoveryV1().EndpointSlices("apps").Get(context.Background(), sliceName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				row.Endpoints[0].TargetRef.UID = "foreign-pod"
				if err = client.Tracker().Update(discoveryv1.SchemeGroupVersion.WithResource("endpointslices"), row, "apps"); err != nil {
					t.Fatal(err)
				}
			}
			c.metricsRead = func(context.Context, Target, string) (io.ReadCloser, error) {
				reads++
				if scenario == "changed-after-read" {
					mutateSlice()
				}
				return io.NopCloser(strings.NewReader(productMetric(request, "3"))), nil
			}
			switch scenario {
			case "foreign-legacy-endpoint":
				row, err := client.CoreV1().Endpoints("apps").Get(context.Background(), deploymentplan.ResourceName(request.Blueprint.ProjectID, "collector"), metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				row.Subsets[0].Addresses[0].TargetRef.UID = "foreign-pod"
				row.Subsets[0].Addresses[0].IP = "10.0.0.99"
				if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("endpoints"), row, "apps"); err != nil {
					t.Fatal(err)
				}
			case "rotated-secret":
				row, err := client.CoreV1().Secrets("apps").Get(context.Background(), "client-tls", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				row.ResourceVersion = "2"
				if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), row, "apps"); err != nil {
					t.Fatal(err)
				}
			case "foreign-endpoint":
				mutateSlice()
			case "pagination":
				client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, &corev1.PodList{ListMeta: metav1.ListMeta{Continue: "more"}}, nil
				})
			}
			if _, err := c.ReadTelemetry(context.Background(), request.Identity); err == nil || journal.operation.Telemetry != nil {
				t.Fatal("uncertain source/lineage became a retained valid observation", err)
			}
			if scenario != "changed-after-read" && reads != 0 {
				t.Fatal("unconfirmed source reached proxy")
			}
		})
	}
}
func TestProductTelemetryParserRejectsUnknownLabelsAndMarksReplacedPodCacheAsOmitted(t *testing.T) {
	_, _, _, request, _ := telemetryFixture(t)
	collector := *deploymentplan.ManagedCollector(request.Blueprint, "collector")
	instances := map[string]string{"pod-web": "web"}
	for _, raw := range []string{strings.Replace(productMetric(request, "3"), "event=", `customer_email="PRIVATE@example.test",event=`, 1), productMetric(request, "NaN"), productMetric(request, "1.5"), productMetric(request, "-1"), strings.Replace(productMetric(request, "3"), "p-one/web", "web", 1)} {
		if _, _, err := parseProductMetrics([]byte(raw), request, collector, instances); err == nil {
			t.Fatal("noncanonical or identifying metrics accepted")
		}
	}
	stale := strings.Replace(productMetric(request, "4"), "pod-web", "pod-replaced", 1)
	rows, omitted, err := parseProductMetrics([]byte(stale), request, collector, instances)
	if err != nil || !omitted || len(rows) != 0 {
		t.Fatal("expired instance counted as current", rows, omitted, err)
	}
	rows, omitted, err = parseProductMetrics([]byte("# no configured product samples\n"), request, collector, instances)
	if err != nil || omitted || len(rows) != 0 {
		t.Fatal("missing samples inferred a zero or failure", err)
	}
}
func TestTelemetryLateGrantDenialKeepsSavedSampleButWithholdsResponse(t *testing.T) {
	c, _, journal, request, _ := telemetryFixture(t)
	denied := false
	c.authorize = func(context.Context, Request, bool) error {
		if denied {
			return ErrRefused
		}
		return nil
	}
	c.metricsRead = func(context.Context, Target, string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(productMetric(request, "5"))), nil
	}
	journal.afterSave = func() { denied = true }
	if _, err := c.ReadTelemetry(context.Background(), request.Identity); err == nil || journal.operation.Telemetry == nil {
		t.Fatal("late denial leaked response or falsely rolled back retained observation", err)
	}
}
