package api

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type orchestratorResolutionResponse struct {
	ProjectID string          `json:"projectId"`
	Status    string          `json:"status"`
	Job       *assistance.Job `json:"job,omitempty"`
	TurnID    string          `json:"turnId,omitempty"`
	JobID     string          `json:"jobId,omitempty"`
	Message   string          `json:"message"`
}

// Explicitly resolve one original request, never replay a potentially paid POST.
// This shares send's admission lock so an in-flight reservation cannot be
// mistaken for absence while its first assistant job is still being written.
func (a *API) handleOrchestratorResolve(w http.ResponseWriter, r *http.Request) {
	var req orchestratorRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !validOrchestratorRequest(req) {
		httpx.WriteProblem(w, 400, "invalid_orchestrator_request", "Send the exact original message, turn key, conversation revision, context digest and model selection.", nil)
		return
	}
	id := r.PathValue("id")
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "product_unavailable", "Project context is unavailable.", nil)
		return
	}
	if _, err := a.deps.Product.ResolveProject(id); err != nil {
		httpx.WriteProblem(w, 404, "project_not_found", "No project in your workspace has that id.", nil)
		return
	}
	a.orchestratorAdmissionMu.Lock()
	defer a.orchestratorAdmissionMu.Unlock()
	resolved, err := a.deps.Product.ResolveOrchestratorRequest(r.Context(), id, orchestratorTurnSelection(req))
	out := orchestratorResolutionResponse{ProjectID: id, Status: "unknown", Message: "The exact request could not be confirmed. Keep the original request and check again; no inference was repeated."}
	if err == nil {
		out.Status, out.Message = resolved.Status, resolved.Message
		if resolved.Turn != nil {
			out.TurnID, out.JobID = resolved.Turn.ID, resolved.Turn.JobID
		}
		if out.Status == "accepted" && resolved.Turn != nil {
			turn := resolved.Turn
			job, readErr := a.assistanceStore().Read(turn.JobID)
			var snapshot product.OrchestratorContext
			if readErr != nil || job.ID != turn.JobID || job.ProjectID != id || job.Kind != "orchestrator" || job.OrchestratorTurnID != turn.ID || json.Unmarshal(job.OrchestratorContext, &snapshot) != nil || !reflect.DeepEqual(snapshot, turn.Context) {
				out.Status = "unknown"
				out.Message = "The original request was accepted, but its matching assistant receipt is unavailable. Keep its request and job identity; no job was repaired or repeated."
			} else {
				out.Job = &job
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, out)
}
