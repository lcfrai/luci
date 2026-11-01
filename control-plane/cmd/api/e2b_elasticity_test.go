package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestElasticOriginalResourceRequiresMarkerImageAndNetwork(t *testing.T) {
	var want, actual any
	json.Unmarshal([]byte(`{"tags":{"luci-capacity-intent":"private-original"},"properties":{"image":{"id":"immutable-image"},"network":{"subnet":"private-subnet"}}}`), &want)
	json.Unmarshal([]byte(`{"id":"provider-id","tags":{"luci-capacity-intent":"private-original"},"properties":{"provisioningState":"Succeeded","image":{"id":"immutable-image"},"network":{"subnet":"private-subnet","assignedIP":"10.0.0.7"}}}`), &actual)
	if !elasticSubset(want, actual) {
		t.Fatal("provider-assigned metadata prevented original reconciliation")
	}
	for _, raw := range []string{
		`{"tags":{"luci-capacity-intent":"foreign"},"properties":{"image":{"id":"immutable-image"},"network":{"subnet":"private-subnet"}}}`,
		`{"tags":{"luci-capacity-intent":"private-original"},"properties":{"image":{"id":"other-image"},"network":{"subnet":"private-subnet"}}}`,
		`{"tags":{"luci-capacity-intent":"private-original"},"properties":{"image":{"id":"immutable-image"},"network":{"subnet":"other-subnet"}}}`,
	} {
		json.Unmarshal([]byte(raw), &actual)
		if elasticSubset(want, actual) {
			t.Fatal("different original resource admitted", raw)
		}
	}
}

func TestElasticARMOperationRefusesForeignOrCredentialURLs(t *testing.T) {
	for _, value := range []string{"https://evil.example/operation", "https://user@management.azure.com/subscriptions/a/op", "http://management.azure.com/subscriptions/a/op"} {
		h := http.Header{"Azure-Asyncoperation": []string{value}}
		if elasticOperationURL(h) != "" {
			t.Fatal("untrusted ARM continuation retained")
		}
	}
	h := http.Header{}
	h.Set("Azure-AsyncOperation", "https://management.azure.com/subscriptions/one/providers/Microsoft.Compute/locations/east/operations/original?api-version=2025-04-01")
	if elasticOperationURL(h) == "" {
		t.Fatal("original ARM operation identity was discarded")
	}
}

func TestElasticVMReadbackDoesNotRequireWriteOnlyBootstrap(t *testing.T) {
	raw := []byte(`{"tags":{"luci-capacity-intent":"original"},"properties":{"osProfile":{"adminUsername":"luci","customData":"exact-private-source"}}}`)
	var wanted, observed any
	json.Unmarshal(raw, &wanted)
	json.Unmarshal([]byte(`{"tags":{"luci-capacity-intent":"original"},"properties":{"osProfile":{"adminUsername":"luci"},"provisioningState":"Succeeded"}}`), &observed)
	if !elasticSubset(elasticReadbackProjection(wanted), observed) {
		t.Fatal("omitted write-only boot bytes prevented exact original readback")
	}
	json.Unmarshal([]byte(`{"tags":{"luci-capacity-intent":"original"},"properties":{"osProfile":{"adminUsername":"foreign"}}}`), &observed)
	if elasticSubset(wanted, observed) {
		t.Fatal("readback projection ignored readable identity drift")
	}
	if string(raw) != `{"tags":{"luci-capacity-intent":"original"},"properties":{"osProfile":{"adminUsername":"luci","customData":"exact-private-source"}}}` {
		t.Fatal("immutable bootstrap request bytes changed")
	}
}
