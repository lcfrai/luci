package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAzureClusterAPINeverReadsCredentialsOrStartsProviderForAnonymousRequests(t *testing.T) {
	a := &API{}
	for _, action := range []string{"list", "review", "access", "namespace", "observe"} {
		t.Run(action, func(t *testing.T) {
			method, pattern := "POST", "POST /v1/product/projects/{id}/azure-clusters/commissions/{commissionId}/"+action
			if action == "list" {
				method, pattern = "GET", "GET /v1/product/projects/{id}/azure-clusters/commissions"
			}
			if action == "review" {
				pattern = "POST /v1/product/projects/{id}/azure-clusters/review"
			}
			r := httptest.NewRequest(method, "/v1/product/projects/p-one/azure-clusters/commissions", strings.NewReader(`{"token":"must-not-echo","acknowledgeAccessGrant":true}`))
			r.Pattern = pattern
			r.SetPathValue("id", "p-one")
			r.SetPathValue("commissionId", "aks-"+strings.Repeat("a", 32))
			w := httptest.NewRecorder()
			a.handleAzureClusterCommission(w, r)
			if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "must-not-echo") || w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("anonymous action or private input echoed", w.Code, w.Body.String())
			}
		})
	}
}
