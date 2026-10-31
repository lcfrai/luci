package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKubernetesReferenceMetadataRequiresProjectActorWithoutDisclosingBody(t *testing.T) {
	a := &API{}
	r := httptest.NewRequest("POST", "/v1/product/projects/p-one/kubernetes/targets/cluster-one/references/read", strings.NewReader(`{"kind":"secret","name":"private-resource"}`)).WithContext(context.Background())
	r.Pattern = "POST /v1/product/projects/{id}/kubernetes/targets/{targetId}/references/read"
	r.SetPathValue("id", "p-one")
	r.SetPathValue("targetId", "cluster-one")
	w := httptest.NewRecorder()
	a.handleKubernetesDeployment(w, r)
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "private-resource") || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("anonymous reference metadata boundary disclosed request", w.Code)
	}
}
