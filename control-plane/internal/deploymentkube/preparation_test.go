package deploymentkube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestModelSinkReceiptRejectsAmbiguousPartialOrUnboundedJSON(t *testing.T) {
	receipt := ModelSinkReceipt{Protocol: 1, PreparationID: "prep-original", OperationID: "op-original", PVCUID: "pvc-original", Path: "weights/model.gguf", Bytes: 5, SHA256: strings.Repeat("a", 64), State: "stored-reported"}
	raw, _ := json.Marshal(receipt)
	if got, err := DecodeModelSinkReceipt(raw); err != nil || got != receipt {
		t.Fatal(got, err)
	}
	for _, bad := range []string{string(raw[:len(raw)-1]), string(raw) + "{}", strings.Replace(string(raw), "\"protocol\":1", "\"protocol\":1,\"protocol\":1", 1), strings.Replace(string(raw), "\"protocol\":1", "\"extra\":true,\"protocol\":1", 1), strings.Replace(string(raw), "\"protocol\":1", "\"protocol\":[1]", 1), strings.Repeat(" ", 2049)} {
		if _, err := DecodeModelSinkReceipt([]byte(bad)); err == nil {
			t.Fatal("ambiguous candidate accepted")
		}
	}
}
func TestPreparationTransportHasNoRetryAndOnlyNamedRead404MeansAbsent(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("private-provider-error-must-not-escape"))
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL, QPS: 5, Burst: 5, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Client: client, Namespace: "ns"}
	err = PreparationResourceRPC(context.Background(), target, "POST", "secrets", "", &corev1.Secret{}, nil)
	if err == nil || count != 1 || strings.Contains(err.Error(), "private-provider") {
		t.Fatal("retry or sensitive error", count, err)
	}
	absentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer absentServer.Close()
	client, err = kubernetes.NewForConfig(&rest.Config{Host: absentServer.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	target.Client = client
	if err = PreparationResourceRPC(context.Background(), target, "GET", "pods", "original", nil, nil); !errors.Is(err, ErrPreparationAbsent) {
		t.Fatal("named absent not retained", err)
	}
	if err = PreparationResourceRPC(context.Background(), target, "POST", "pods", "", &corev1.Pod{}, nil); errors.Is(err, ErrPreparationAbsent) || err == nil {
		t.Fatal("failed mutation misclassified absent", err)
	}
}
func TestPreparationPodComparisonRejectsAdmissionChangesAndAllowsSchedulerAssignmentOnly(t *testing.T) {
	desired := corev1.PodSpec{Containers: []corev1.Container{{Name: "model-sink", Image: "registry/model@sha256:" + strings.Repeat("a", 64)}}}
	observed := desired.DeepCopy()
	observed.NodeName = "owned-scheduled-node"
	if !PreparationPodSpecMatches(desired, *observed) {
		t.Fatal("scheduler assignment refused")
	}
	observed.Containers = append(observed.Containers, corev1.Container{Name: "injected-sidecar", Image: "unknown"})
	if PreparationPodSpecMatches(desired, *observed) {
		t.Fatal("unreviewed sidecar accepted")
	}
	observed = desired.DeepCopy()
	observed.HostNetwork = true
	if PreparationPodSpecMatches(desired, *observed) {
		t.Fatal("unreviewed host access accepted")
	}
}
