package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// TeamService is an operator admission control, not a payment receipt or proof
// of execution isolation. Its zero value preserves unrestricted self-hosted use.
type TeamService struct {
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	Revision  int    `json:"revision"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
}

func effectiveTeamService(value TeamService) TeamService {
	if value == (TeamService{}) {
		value.Mode, value.Status = "self-hosted", "active"
	}
	return value
}

func validateTeamService(value TeamService) error {
	if value == (TeamService{}) {
		return nil
	}
	if (value.Mode != "self-hosted" && value.Mode != "managed") || (value.Status != "active" && value.Status != "suspended") || (value.Mode == "self-hosted" && value.Status != "active") || value.Revision < 1 || strings.TrimSpace(value.Reason) == "" || len(value.Reason) > 240 || value.UpdatedBy == "" {
		return errors.New("invalid team service state")
	}
	if _, err := time.Parse(time.RFC3339Nano, value.UpdatedAt); err != nil {
		return errors.New("invalid team service timestamp")
	}
	return nil
}

func (g *Gateway) registerTeamService(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/team-services", g.listTeamServices)
	mux.HandleFunc("PUT /v1/admin/team-services/{teamId}", g.setTeamService)
	mux.HandleFunc("GET /v1/teams/{teamId}/service", g.getTeamService)
}

func (g *Gateway) listTeamServices(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	if actor.Role != "super-admin" {
		fail(w, 403, "operator_required", "Only a Super Admin can operate managed team service access.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if current, exists := g.state.Users[actor.ID]; g.closed || !exists || current.User.Disabled || current.User.Role != "super-admin" {
		fail(w, 403, "operator_required", "Current Super Admin access is required.")
		return
	}
	rows := []map[string]any{}
	for id, row := range g.state.Teams {
		rows = append(rows, map[string]any{"teamId": id, "name": row.Team.Name, "service": effectiveTeamService(row.Service)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["teamId"].(string) < rows[j]["teamId"].(string) })
	jsonReply(w, 200, map[string]any{"teams": rows, "billingConnected": g.billing != nil, "executionIsolationVerified": false})
}

func (g *Gateway) getTeamService(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil {
		fail(w, 404, "team_unavailable", errTeamAccess.Error())
		return
	}
	jsonReply(w, 200, struct {
		TeamService
		BillingEnabled bool `json:"billingEnabled,omitempty"`
	}{effectiveTeamService(g.state.Teams[access.Team.ID].Service), g.billing != nil})
}

func (g *Gateway) setTeamService(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	if actor.Role != "super-admin" {
		fail(w, 403, "operator_required", "Only a Super Admin can operate managed team service access.")
		return
	}
	var input struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Mode             string `json:"mode"`
		Status           string `json:"status"`
		Reason           string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	value := TeamService{Mode: input.Mode, Status: input.Status, Revision: input.ExpectedRevision + 1, Reason: strings.TrimSpace(input.Reason), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), UpdatedBy: actor.ID}
	if validateTeamService(value) != nil {
		fail(w, 400, "invalid_team_service", "Choose self-hosted active, managed active or managed suspended, and record a reason of 1–240 characters. This does not create a subscription or verify host isolation.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	id := r.PathValue("teamId")
	if current, exists := g.state.Users[actor.ID]; g.closed || !exists || current.User.Disabled || current.User.Role != "super-admin" {
		fail(w, 403, "operator_required", "Current Super Admin access is required.")
		return
	}
	row, exists := g.state.Teams[id]
	if !exists {
		fail(w, 404, "team_unavailable", "The selected team is unavailable.")
		return
	}
	if row.Service.Revision != input.ExpectedRevision {
		fail(w, 409, "team_service_changed", "Team service access changed. Reload its current revision before saving.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row.Service = value
	g.state.Teams[id] = row
	g.auditLocked(actor.ID, "team.service."+value.Status, id, "success")
	if err := g.persistLocked(); err != nil {
		g.state = before
		fail(w, 500, "state_write_failed", "The service change could not be saved. Existing access remains unchanged.")
		return
	}
	// Close live terminals/browser bridges and cancel in-flight requests only
	// after durable suspension. Child workloads and data remain explicitly owned
	// stop operations; this control never promises to stop provider billing.
	if value.Status == "suspended" {
		g.revokeTeamAccessLocked(id, "", "")
	}
	jsonReply(w, 200, value)
}

// Call with g.mu held. Unknown teams fail closed; ordinary self-hosted teams do
// not require a subscription, external account or licence key.
func (g *Gateway) teamServiceAllowsExecutionLocked(teamID string) bool {
	row, exists := g.state.Teams[teamID]
	return exists && effectiveTeamService(row.Service).Status == "active" && g.billingAllowsExecutionLocked(teamID)
}

type serviceCleanupKey struct{}

func teamServiceCleanup(r *http.Request) bool {
	_, required, known := classifyTeamRoute(r)
	if !known {
		return false
	}
	if required&(teamExecute|teamDeploy|teamSecrets) == 0 || (r.Method == "GET" || r.Method == "HEAD") && required&teamExecute == 0 {
		return true
	}
	_, pattern := teamRouteMux.Handler(r)
	switch pattern {
	case "DELETE /v1/sessions/{sessionId}", "DELETE /v1/setup/agents/codex/account",
		"POST /v1/product/projects/{projectId}/assist/{jobId}/cancel",
		"POST /v1/product/projects/{projectId}/checks/{runId}/cancel",
		"POST /v1/product/projects/{projectId}/releases/{releaseId}/stop",
		"POST /v1/product/projects/{projectId}/releases/{releaseId}/cancel",
		"POST /v1/product/projects/{projectId}/releases/{releaseId}/refresh":
		return true
	case "POST /v1/platform/projects/{projectId}/deployments/{releaseId}/{action}":
		return strings.HasSuffix(r.URL.Path, "/stop")
	}
	allowed, _ := r.Context().Value(serviceCleanupKey{}).(bool)
	return allowed
}

func (g *Gateway) teamServiceRequestAllowedLocked(teamID string, r *http.Request) bool {
	return g.teamServiceAllowsExecutionLocked(teamID) || teamServiceCleanup(r)
}

// Inspect only small action bodies whose supported operations include cleanup.
// Restore the exact bytes for the worker's strict decoder; never treat a body
// parameter as authority or weaken the separate route/project capability checks.
func prepareTeamServiceCleanup(w http.ResponseWriter, r *http.Request) {
	_, pattern := teamRouteMux.Handler(r)
	switch pattern {
	case "POST /v1/platform/projects/{projectId}/database", "POST /v1/platform/projects/{projectId}/data", "POST /v1/setup/agent-connections", "POST /v1/setup/agents/anthropic", "POST /v1/setup/agents/openai", "POST /v1/product/projects/{projectId}/configuration":
	default:
		return
	}
	if r.Body == nil {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return
	}
	var action string
	for key := range fields {
		if strings.EqualFold(key, "action") && key != "action" {
			return
		}
	}
	_ = json.Unmarshal(fields["action"], &action)
	allowed := false
	switch pattern {
	case "POST /v1/platform/projects/{projectId}/database", "POST /v1/platform/projects/{projectId}/data":
		allowed = action == "stop" || action == "backup" || action == "refresh"
	case "POST /v1/setup/agent-connections":
		allowed = action == "delete"
	case "POST /v1/setup/agents/anthropic", "POST /v1/setup/agents/openai":
		var key string
		allowed = len(fields) == 1 && json.Unmarshal(fields["apiKey"], &key) == nil && key == ""
	case "POST /v1/product/projects/{projectId}/configuration":
		allowed = serviceConfigurationRemoval(fields)
	}
	if allowed {
		*r = *r.WithContext(context.WithValue(r.Context(), serviceCleanupKey{}, true))
	}
}

func serviceConfigurationRemoval(fields map[string]json.RawMessage) bool {
	for key := range fields {
		if key != "expectedRevision" && key != "updates" {
			return false
		}
	}
	var updates []map[string]json.RawMessage
	if json.Unmarshal(fields["updates"], &updates) != nil || len(updates) == 0 || len(updates) > 128 {
		return false
	}
	for _, update := range updates {
		for key := range update {
			if key != "name" && key != "remove" {
				return false
			}
		}
		var remove bool
		if json.Unmarshal(update["remove"], &remove) != nil || !remove {
			return false
		}
	}
	return true
}

func (g *Gateway) authorizeTeamService(w http.ResponseWriter, r *http.Request, teamID string) bool {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	active := g.teamServiceAllowsExecutionLocked(teamID)
	detail := ""
	if !active {
		detail = g.teamServiceDenialLocked(teamID, actor.ID)
	}
	g.mu.Unlock()
	if active {
		return true
	}
	prepareTeamServiceCleanup(w, r)
	if teamServiceCleanup(r) {
		return true
	}
	fail(w, 403, "team_service_suspended", detail)
	return false
}

// Guidance only: callers have already applied the existing admission decision.
// Read saved state under g.mu without contacting Stripe or changing entitlement.
// A manual suspension takes precedence; paying cannot restore operator access.
func (g *Gateway) teamServiceDenialLocked(teamID, actorID string) string {
	retained := " Context, exports, owned stop/cancel operations and connection removal remain available with your existing permissions. Existing workloads are not stopped automatically."
	row, exists := g.state.Teams[teamID]
	if !exists || effectiveTeamService(row.Service).Status == "suspended" || g.billing == nil {
		return "Managed team execution is suspended. Ask the service operator to restore execution access." + retained
	}
	detail := "New team execution is unavailable because subscription access is not currently verified."
	if b := row.Billing; b != nil && (b.Status == "active" || b.Status == "trialing") && b.PeriodEnd > 0 && b.PeriodEnd <= time.Now().Unix() {
		detail = "New team execution is unavailable because the last verified subscription period has ended."
	}
	if row.Members[actorID].Role == "owner" {
		detail += " Open the team's Service and data settings to review subscription status and verify it with Stripe. Resume an existing checkout or manage the subscription there; do not create another checkout to recover an uncertain payment."
	} else {
		detail += " Ask a team owner to open Service and data, review the subscription and verify it with Stripe."
	}
	return detail + retained
}
