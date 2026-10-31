package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKubernetesAPIBoundaryRejectsAnonymousAndMalformedPrivateRequests(t *testing.T) {
	a := &API{}
	for _, pattern := range []string{"GET /v1/product/projects/{id}/kubernetes/targets", "PUT /v1/product/projects/{id}/kubernetes/targets", "POST /v1/product/projects/{id}/kubernetes/review", "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/apply"} {
		method := strings.Split(pattern, " ")[0]
		r := httptest.NewRequest(method, "/v1/product/projects/p-one/kubernetes/targets", strings.NewReader(`{"credentials":{"token":"fixture-secret"}}`)).WithContext(context.Background())
		r.Pattern = pattern
		r.SetPathValue("id", "p-one")
		w := httptest.NewRecorder()
		a.handleKubernetesDeployment(w, r)
		if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "fixture-secret") || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("anonymous action or credential disclosure", w.Code, w.Body.String())
		}
	}
}
