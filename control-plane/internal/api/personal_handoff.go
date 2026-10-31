package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Reuse the exact project orchestrator permission set. Read access to included
// evidence is not Edit/Execute/Secrets authority over the destination.
func (a *API) personalDestination(ctx context.Context, projectID, harness string) (context.Context, error) {
	actor, _, ok := management.ActorFromContext(ctx)
	if !ok || actor.Disabled {
		return ctx, management.ErrExecutionAuthority
	}
	authority, err := a.authorizeExecution(ctx, management.ExecutionActorRef{UserID: actor.ID, TeamID: product.PersonalWorkspaceTeamID(ctx), ProjectID: projectID}, harness, true)
	if err != nil || authority.Actor.ID != actor.ID || authority.Team != nil && !authority.Team.Permissions.Edit {
		return ctx, management.ErrExecutionAuthority
	}
	if authority.Team == nil && product.PersonalWorkspaceTeamID(ctx) != "" || authority.Team != nil && (authority.Team.ProjectID != projectID || authority.Team.Team.ID != product.PersonalWorkspaceTeamID(ctx)) {
		return ctx, management.ErrExecutionAuthority
	}
	return management.WithExecutionAuthority(ctx, authority), nil
}

func (a *API) authorizeDiscussionJob(ctx context.Context, job assistance.Job) error {
	if job.Kind != "orchestrator" || !strings.HasPrefix(job.OrchestratorTurnID, "turn-discussion-") {
		return nil
	}
	if a.deps.Product == nil {
		return management.ErrExecutionAuthority
	}
	turn, err := a.deps.Product.OrchestratorTurn(job.ProjectID, job.OrchestratorTurnID)
	if err != nil || !turn.IsDiscussionOnly() || turn.Actor.ID != job.RequestedBy || turn.JobID != job.ID {
		return management.ErrExecutionAuthority
	}
	authority, err := a.authorizeExecution(ctx, management.ExecutionActorRef{UserID: job.RequestedBy, TeamID: job.DiscussionTeamID, ProjectID: job.ProjectID}, job.Harness, true)
	if err != nil || authority.Actor.ID != job.RequestedBy || authority.Team != nil && (!authority.Team.Permissions.Edit || authority.Team.Team.ID != job.DiscussionTeamID || authority.Team.ProjectID != job.ProjectID) || authority.Team == nil && job.DiscussionTeamID != "" {
		return management.ErrExecutionAuthority
	}
	return ctx.Err()
}
func personalProjectRequest(turn product.PersonalTurn) orchestratorRequest {
	p, h := turn.Context.Preferences, turn.Handoff
	return orchestratorRequest{ExpectedRevision: h.ProjectRevision, ExpectedContextDigest: h.ProjectContextDigest, IdempotencyKey: h.ProjectRequestKey, Message: h.Message, Harness: p.Harness, Model: p.Model, AgentConnectionID: &p.ConnectionID}
}

// A bounded in-process response capture lets the SAME real project admission
// handler retain its job/turn lifecycle. It is not a network call or a second
// engine; the private handoff stores only the returned original identities.
type personalAdmissionResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (w *personalAdmissionResponse) Header() http.Header { return w.header }
func (w *personalAdmissionResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *personalAdmissionResponse) Write(raw []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.body.Len()+len(raw) > assistance.MaxJobBytes {
		w.overflow = true
		return len(raw), nil
	}
	return w.body.Write(raw)
}

func (a *API) handlePersonalHandoff(w http.ResponseWriter, r *http.Request) {
	access, ok := a.personalRequest(w, r)
	if !ok {
		return
	}
	var req product.PersonalHandoffRequest
	if !portfolioGoalsDecode(w, r, &req) {
		return
	}
	turn, err := a.deps.Product.PersonalTurn(r.Context(), r.PathValue("turnId"), access)
	if err != nil {
		personalProblem(w, err)
		return
	}
	destination, err := a.personalDestination(r.Context(), req.ProjectID, turn.Context.Preferences.Harness)
	if err != nil {
		personalProblem(w, err)
		return
	}
	view, err := a.deps.Product.OrchestratorView(destination, req.ProjectID, 0)
	if err != nil || view.Revision != req.ProjectRevision || view.Context.Digest != req.ProjectContextDigest {
		personalProblem(w, product.ErrPersonalChanged)
		return
	}
	turn, existing, err := a.deps.Product.ReservePersonalHandoff(r.Context(), turn.ID, req, access)
	if err != nil {
		personalProblem(w, err)
		return
	}
	if existing {
		a.writePersonalHandoff(w, r, turn, access)
		return
	} // Never resend an accepted/uncertain handoff.
	destination, err = a.personalDestination(r.Context(), req.ProjectID, turn.Context.Preferences.Harness)
	if err != nil {
		personalProblem(w, err)
		return
	}
	if err := product.PersonalContextAccessible(r.Context(), turn.Context, access); err != nil {
		personalProblem(w, err)
		return
	}
	projectRequest := r.Clone(product.PersonalDiscussionContext(destination))
	projectRequest.SetPathValue("id", req.ProjectID)
	response := &personalAdmissionResponse{header: http.Header{}}
	a.startOrchestratorTurn(response, projectRequest, personalProjectRequest(turn))
	var job assistance.Job
	projectTurnID, projectJobID := "", ""
	if !response.overflow && response.status == 202 && json.Unmarshal(response.body.Bytes(), &job) == nil && job.Kind == "orchestrator" && job.PersonalOwner == "" && job.ProjectID == req.ProjectID {
		stored, readErr := a.deps.Product.OrchestratorTurn(req.ProjectID, job.OrchestratorTurnID)
		if readErr == nil && stored.IsDiscussionOnly() && stored.JobID == job.ID && stored.Message == req.Message {
			projectTurnID, projectJobID = stored.ID, job.ID
		}
	}
	if err := a.deps.Product.CompletePersonalHandoff(r.Context(), turn.ID, turn.Handoff.ID, projectTurnID, projectJobID, access); err != nil {
		personalProblem(w, err)
		return
	}
	latest, err := a.deps.Product.PersonalTurn(r.Context(), turn.ID, access)
	if err != nil {
		personalProblem(w, err)
		return
	}
	a.writePersonalHandoff(w, r, latest, access)
}
func (a *API) writePersonalHandoff(w http.ResponseWriter, r *http.Request, turn product.PersonalTurn, access product.PortfolioGoalAccess) {
	if turn.Handoff == nil || access(r.Context(), turn.Handoff.ProjectID, false) != nil || product.PersonalContextAccessible(r.Context(), turn.Context, access) != nil {
		personalProblem(w, management.ErrExecutionAuthority)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"handoff": turn.Handoff, "message": "Only your reviewed message was routed. No amendment or execution was authorized. An unconfirmed result must be checked, never resent automatically."})
}
func (a *API) handlePersonalHandoffResolve(w http.ResponseWriter, r *http.Request) {
	access, ok := a.personalRequest(w, r)
	if !ok {
		return
	}
	turn, err := a.deps.Product.PersonalTurn(r.Context(), r.PathValue("turnId"), access)
	if err != nil || turn.Handoff == nil {
		personalProblem(w, product.ErrPersonalUnavailable)
		return
	}
	destination, err := a.personalDestination(r.Context(), turn.Handoff.ProjectID, turn.Context.Preferences.Harness)
	if err != nil {
		personalProblem(w, err)
		return
	}
	a.orchestratorAdmissionMu.Lock()
	resolved, err := a.deps.Product.ResolveOrchestratorRequest(product.PersonalDiscussionContext(destination), turn.Handoff.ProjectID, orchestratorTurnSelection(personalProjectRequest(turn)))
	a.orchestratorAdmissionMu.Unlock()
	if err != nil {
		personalProblem(w, err)
		return
	}
	if resolved.Status == "accepted" && resolved.Turn != nil && resolved.Turn.IsDiscussionOnly() {
		job, readErr := a.assistanceStore().Read(resolved.Turn.JobID)
		if readErr == nil && job.Kind == "orchestrator" && job.PersonalOwner == "" && job.ProjectID == turn.Handoff.ProjectID && job.OrchestratorTurnID == resolved.Turn.ID {
			if err := a.deps.Product.CompletePersonalHandoff(r.Context(), turn.ID, turn.Handoff.ID, resolved.Turn.ID, job.ID, access); err != nil {
				personalProblem(w, err)
				return
			}
		}
	}
	latest, err := a.deps.Product.PersonalTurn(r.Context(), turn.ID, access)
	if err != nil {
		personalProblem(w, err)
		return
	}
	a.writePersonalHandoff(w, r, latest, access)
}
