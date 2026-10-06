package management

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"time"
)

// This is an owner's reversible request for operator review, never evidence of
// deletion or a command to stop workloads, cancel billing or erase backups.
type TeamDataLifecycle struct {
	Revision    int    `json:"revision"`
	Status      string `json:"status"`
	RequestID   string `json:"requestId,omitempty"`
	RequestedBy string `json:"requestedBy,omitempty"`
	RequestedAt string `json:"requestedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	CancelledBy string `json:"cancelledBy,omitempty"`
	CancelledAt string `json:"cancelledAt,omitempty"`
}

var deletionRequestID = regexp.MustCompile(`^delete-[A-Za-z0-9_-]{22}$`)

func effectiveTeamDataLifecycle(value TeamDataLifecycle) TeamDataLifecycle {
	if value == (TeamDataLifecycle{}) {
		value.Status = "none"
	}
	return value
}
func validateTeamDataLifecycle(value TeamDataLifecycle) error {
	if value == (TeamDataLifecycle{}) {
		return nil
	}
	invalid := errors.New("invalid team data request state")
	if value.Revision < 1 || !deletionRequestID.MatchString(value.RequestID) || value.RequestedBy == "" || (value.Status != "requested" && value.Status != "cancelled") {
		return invalid
	}
	requested, err := time.Parse(time.RFC3339Nano, value.RequestedAt)
	if err != nil {
		return invalid
	}
	updated, err := time.Parse(time.RFC3339Nano, value.UpdatedAt)
	if err != nil || updated.Before(requested) {
		return invalid
	}
	if value.Status == "requested" {
		if value.CancelledBy != "" || value.CancelledAt != "" || value.UpdatedAt != value.RequestedAt {
			return invalid
		}
	} else {
		cancelled, err := time.Parse(time.RFC3339Nano, value.CancelledAt)
		if err != nil || value.CancelledBy == "" || cancelled.Before(requested) || value.UpdatedAt != value.CancelledAt {
			return invalid
		}
	}
	return nil
}
func (g *Gateway) registerTeamDataLifecycle(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams/{teamId}/data-lifecycle", g.getTeamDataLifecycle)
	mux.HandleFunc("POST /v1/teams/{teamId}/data-lifecycle/request", g.requestTeamDataDeletion)
	mux.HandleFunc("POST /v1/teams/{teamId}/data-lifecycle/cancel", g.cancelTeamDataDeletion)
	mux.HandleFunc("GET /v1/admin/team-data-requests", g.listTeamDataRequests)
}
func (g *Gateway) teamDataOwnerLocked(w http.ResponseWriter, r *http.Request) (TeamAccess, bool) {
	actor, _, _ := ActorFromContext(r.Context())
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil || access.Member.Role != "owner" {
		fail(w, 403, "team_owner_required", "Only a current team owner can request or cancel review of permanent data deletion.")
		return TeamAccess{}, false
	}
	return access, true
}
func (g *Gateway) getTeamDataLifecycle(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.teamDataOwnerLocked(w, r)
	if !ok {
		return
	}
	jsonReply(w, 200, map[string]any{"team": access.Team, "dataLifecycle": effectiveTeamDataLifecycle(g.state.Teams[access.Team.ID].DataLifecycle)})
}
func (g *Gateway) requestTeamDataDeletion(w http.ResponseWriter, r *http.Request) {
	g.changeTeamDataRequest(w, r, false)
}
func (g *Gateway) cancelTeamDataDeletion(w http.ResponseWriter, r *http.Request) {
	g.changeTeamDataRequest(w, r, true)
}
func (g *Gateway) changeTeamDataRequest(w http.ResponseWriter, r *http.Request, cancel bool) {
	var input struct {
		ExpectedRevision    int    `json:"expectedRevision"`
		AcknowledgeDeletion bool   `json:"acknowledgeDeletion,omitempty"`
		RequestID           string `json:"requestId,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !cancel && !input.AcknowledgeDeletion {
		fail(w, 400, "deletion_acknowledgement_required", "Confirm that you want an operator to review permanent deletion of this team's data.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.teamDataOwnerLocked(w, r)
	if !ok {
		return
	}
	if input.ExpectedRevision < 1 || input.ExpectedRevision != access.Team.Revision {
		fail(w, 409, "team_revision_conflict", "Team access changed. Reload before changing its data request.")
		return
	}
	row := g.state.Teams[access.Team.ID]
	current := row.DataLifecycle
	if cancel && (current.Status != "requested" || input.RequestID != current.RequestID) {
		fail(w, 409, "data_request_changed", "This deletion request is no longer pending. Refresh its current status.")
		return
	}
	if !cancel && current.Status == "requested" {
		fail(w, 409, "data_request_pending", "A deletion request is already awaiting operator review. Refresh or cancel that request.")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	action := "team.data.deletion.request"
	if cancel {
		current.Revision++
		current.Status = "cancelled"
		current.CancelledBy = access.Member.UserID
		current.CancelledAt = now
		current.UpdatedAt = now
		action = "team.data.deletion.cancel"
	} else {
		current = TeamDataLifecycle{Revision: current.Revision + 1, Status: "requested", RequestID: opaqueID("delete-"), RequestedBy: access.Member.UserID, RequestedAt: now, UpdatedAt: now}
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row.DataLifecycle = current
	bumpTeam(&row)
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, action, row.Team.ID) {
		return
	}
	jsonReply(w, 200, map[string]any{"team": row.Team, "dataLifecycle": current})
}
func (g *Gateway) listTeamDataRequests(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	user, exists := g.state.Users[actor.ID]
	if g.closed || !exists || user.User.Disabled || user.User.Role != "super-admin" {
		fail(w, 403, "operator_required", "Only a current Super Admin can review pending team data requests.")
		return
	}
	rows := []map[string]any{}
	for id, row := range g.state.Teams {
		if row.DataLifecycle.Status == "requested" && (g.managedTeamID == "" || g.managedTeamID == id) {
			rows = append(rows, map[string]any{"team": row.Team, "dataLifecycle": row.DataLifecycle})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["team"].(Team).ID < rows[j]["team"].(Team).ID })
	jsonReply(w, 200, map[string]any{"requests": rows})
}
