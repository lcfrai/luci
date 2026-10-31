package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Cluster creation retains the existing setup/cloud plan+job path. These
// project-bound endpoints review two separate, explicitly acknowledged effects:
// a cluster-scoped bootstrap access grant and namespace create/import. Neither
// starts workloads. Provider errors/credential envelopes are never projected.
func (a *API) handleAzureClusterCommission(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	actor, err := management.CaptureDeploymentActor(r.Context(), r.PathValue("id"))
	if err != nil || actor.Local || actor.UserID == "" {
		httpx.WriteProblem(w, 403, "azure_cluster_forbidden", "Current project Deploy and Secrets access is required.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "azure_cluster_unavailable", "Cluster commissioning is unavailable.", nil)
		return
	}
	decode := func(value any) bool {
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
		d.DisallowUnknownFields()
		return d.Decode(value) == nil && d.Decode(new(any)) == io.EOF
	}
	var value any
	switch r.Pattern {
	case "GET /v1/product/projects/{id}/azure-clusters/commissions":
		var rows []product.AzureClusterCommission
		var jobs []product.AzureClusterJobView
		rows, err = a.deps.Product.ListAzureClusterCommissions(r.Context(), actor)
		if err == nil {
			jobs, err = a.deps.Product.ListAzureClusterJobs(r.Context(), actor)
		}
		value = struct {
			Commissions []product.AzureClusterCommission `json:"commissions"`
			Jobs        []product.AzureClusterJobView    `json:"jobs"`
		}{rows, jobs}
	case "POST /v1/product/projects/{id}/azure-clusters/review":
		var input product.AzureClusterCommissionRequest
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "azure_cluster_invalid", "Choose one retained cluster and a project namespace to create or import.", nil)
			return
		}
		value, err = a.deps.Product.ReviewAzureClusterCommission(r.Context(), actor, input)
	case "POST /v1/product/projects/{id}/azure-clusters/commissions/{commissionId}/access", "POST /v1/product/projects/{id}/azure-clusters/commissions/{commissionId}/namespace", "POST /v1/product/projects/{id}/azure-clusters/commissions/{commissionId}/observe":
		var input struct {
			AcknowledgeCost        bool `json:"acknowledgeCost"`
			AcknowledgeAccessGrant bool `json:"acknowledgeAccessGrant"`
		}
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "azure_cluster_invalid", "Review the original operation and explicit effect acknowledgements.", nil)
			return
		}
		if strings.HasSuffix(r.Pattern, "/observe") {
			value, err = a.deps.Product.ObserveAzureClusterCommission(r.Context(), actor, r.PathValue("commissionId"))
		} else {
			action := "namespace"
			if strings.HasSuffix(r.Pattern, "/access") {
				action = "access"
			}
			value, err = a.deps.Product.ApplyAzureClusterCommission(r.Context(), actor, r.PathValue("commissionId"), action, input.AcknowledgeCost, input.AcknowledgeAccessGrant)
		}
	default:
		httpx.WriteProblem(w, 404, "not_found", "This cluster commissioning action is unavailable.", nil)
		return
	}
	if err != nil {
		status := 409
		if err == management.ErrExecutionAuthority {
			status = 403
		}
		httpx.WriteProblem(w, status, "azure_cluster_changed", "The original account, cluster, current access or reviewed source could not be confirmed. An attempted effect remains retained; observe its original record before taking another action.", nil)
		return
	}
	httpx.WriteJSON(w, 200, value)
}
