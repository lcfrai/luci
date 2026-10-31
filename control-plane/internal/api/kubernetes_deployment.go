package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Credentials enter only the explicit target setup body. Neither provider
// errors nor request bodies are logged or returned by this boundary.
func (a *API) handleKubernetesDeployment(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(product.WithKubernetesTargetResolution(r.Context()))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	actor, err := management.CaptureDeploymentActor(r.Context(), r.PathValue("id"))
	if err != nil || actor.Local || actor.UserID == "" {
		httpx.WriteProblem(w, 403, "kubernetes_forbidden", "Current project Deploy access is required.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "kubernetes_unavailable", "Cluster deployment is unavailable.", nil)
		return
	}
	decode := func(value any) bool {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
		decoder.DisallowUnknownFields()
		return decoder.Decode(value) == nil && decoder.Decode(&struct{}{}) == io.EOF
	}
	var value any
	switch r.Pattern {
	case "GET /v1/product/projects/{id}/kubernetes/targets":
		var rows []product.KubernetesTargetDescriptor
		rows, err = a.deps.Product.ListKubernetesTargets(r.Context(), actor)
		value = struct {
			Targets []product.KubernetesTargetDescriptor `json:"targets"`
		}{rows}
	case "PUT /v1/product/projects/{id}/kubernetes/targets":
		var input product.KubernetesTargetSave
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "kubernetes_invalid", "Provide one bounded target using explicit TLS credentials.", nil)
			return
		}
		value, err = a.deps.Product.SaveKubernetesTarget(r.Context(), actor, input)
	case "POST /v1/product/projects/{id}/kubernetes/targets/{targetId}/references/read":
		var input product.KubernetesReferenceRead
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "kubernetes_invalid", "Choose one existing resource kind and optional exact name.", nil)
			return
		}
		value, err = a.deps.Product.ReadKubernetesReferences(r.Context(), actor, r.PathValue("targetId"), input)
	case "GET /v1/product/projects/{id}/kubernetes/deployments":
		var rows []product.KubernetesDeploymentStatus
		rows, err = a.deps.Product.KubernetesDeployments(r.Context(), actor)
		value = struct {
			Deployments []product.KubernetesDeploymentStatus `json:"deployments"`
		}{rows}
	case "POST /v1/product/projects/{id}/kubernetes/review":
		var input product.KubernetesReviewInput
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "kubernetes_invalid", "Select a saved blueprint, existing target and checked run.", nil)
			return
		}
		value, err = a.deps.Product.ReviewKubernetesDeployment(r.Context(), actor, input)
	case "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/telemetry/read":
		var input struct{}
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "kubernetes_invalid", "Read only the exact retained Collector operation.", nil)
			return
		}
		value, err = a.deps.Product.ReadKubernetesTelemetry(r.Context(), actor, r.PathValue("operationId"))
	case "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/apply", "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/observe":
		var input struct {
			AcknowledgeCost            bool `json:"acknowledgeCost"`
			AcknowledgeImageProvenance bool `json:"acknowledgeImageProvenance"`
			AcknowledgeTelemetry       bool `json:"acknowledgeTelemetry"`
		}
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "kubernetes_invalid", "Provide the explicit action acknowledgement.", nil)
			return
		}
		value, err = a.deps.Product.KubernetesDeploymentAction(r.Context(), actor, r.PathValue("operationId"), r.Pattern == "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/apply", input.AcknowledgeCost, input.AcknowledgeImageProvenance, input.AcknowledgeTelemetry)
	default:
		httpx.WriteProblem(w, 404, "not_found", "This cluster operation is unavailable.", nil)
		return
	}
	if err != nil {
		status := 409
		if err == management.ErrExecutionAuthority {
			status = 403
		}
		httpx.WriteProblem(w, status, "kubernetes_changed", "The target, current access or reviewed source could not be confirmed. Any attempted operation remains retained. Read retained deployments before taking another action.", nil)
		return
	}
	// Every product operation performs its final fresh grant check after I/O.
	httpx.WriteJSON(w, 200, value)
}
