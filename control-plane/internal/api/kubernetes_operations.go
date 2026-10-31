package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func (a *API) handleKubernetesOperations(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(product.WithKubernetesTargetResolution(r.Context()))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	actor, err := management.CaptureDeploymentActor(r.Context(), r.PathValue("id"))
	if err != nil || actor.Local || actor.UserID == "" {
		httpx.WriteProblem(w, 403, "operations_forbidden", "Current project Edit, Deploy and Secrets access is required.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "operations_unavailable", "Operational inspection is unavailable.", nil)
		return
	}
	decode := func(value any) bool {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
		decoder.DisallowUnknownFields()
		return decoder.Decode(value) == nil && decoder.Decode(new(any)) == io.EOF
	}
	var value any
	switch r.Pattern {
	case "GET /v1/product/projects/{id}/kubernetes/targets/{targetId}/tempo-query":
		value, err = a.deps.Product.KubernetesTempo(r.Context(), actor, r.PathValue("targetId"))
	case "PUT /v1/product/projects/{id}/kubernetes/targets/{targetId}/tempo-query":
		var input product.KubernetesTempoSave
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Provide the existing Tempo HTTPS query endpoint and explicit private CA/authentication.", nil)
			return
		}
		value, err = a.deps.Product.SaveKubernetesTempo(r.Context(), actor, r.PathValue("targetId"), input)
	case "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/operations/pods/read":
		var input struct {
			ComponentID string `json:"componentId"`
		}
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Read one service from the selected original deployment.", nil)
			return
		}
		value, err = a.deps.Product.ReadKubernetesOperationalPods(r.Context(), actor, r.PathValue("operationId"), input.ComponentID)
	case "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/operations/logs/read":
		var input product.KubernetesLogRead
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Choose an owned service/Pod and a bounded non-following log query.", nil)
			return
		}
		value, err = a.deps.Product.ReadKubernetesOperationalLogs(r.Context(), actor, r.PathValue("operationId"), input)
	case "POST /v1/product/projects/{id}/kubernetes/deployments/{operationId}/operations/traces/read":
		var input deploymentkube.TraceQuery
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Choose one owned Pod and a trace window of at most one hour.", nil)
			return
		}
		value, err = a.deps.Product.ReadKubernetesOperationalTraces(r.Context(), actor, r.PathValue("operationId"), input)
	case "GET /v1/product/projects/{id}/kubernetes/operations/evidence/selected":
		value, err = a.deps.Product.ReadOperationalSelection(r.Context(), actor)
	case "POST /v1/product/projects/{id}/kubernetes/operations/evidence/selection/clear":
		var input struct {
			ExpectedDigest string `json:"expectedDigest"`
		}
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Clear only the saved selection you just read.", nil)
			return
		}
		value, err = a.deps.Product.ClearOperationalSelection(r.Context(), actor, input.ExpectedDigest)
	case "POST /v1/product/projects/{id}/kubernetes/operations/evidence/{evidenceId}/selection":
		var input product.OperationalEvidenceSelect
		if !decode(&input) {
			httpx.WriteProblem(w, 400, "operations_invalid", "Explicitly select or remove this original structured observation.", nil)
			return
		}
		value, err = a.deps.Product.SelectOperationalEvidence(r.Context(), actor, r.PathValue("evidenceId"), input)
	default:
		httpx.WriteProblem(w, 404, "not_found", "This operational query is unavailable.", nil)
		return
	}
	if err != nil {
		status := 409
		if errors.Is(err, management.ErrExecutionAuthority) {
			status = 403
		}
		httpx.WriteProblem(w, status, "operations_unconfirmed", "The original workload, current source, query backend or present access could not be confirmed. No workload was changed; retained observation metadata is not rolled back.", nil)
		return
	}
	httpx.WriteJSON(w, 200, value)
}
