package management

import (
	"net/http"
	"strings"
)

const teamAuditLimit = 200

type teamAuditActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}
type teamAuditEvent struct {
	AuditEvent
	Actor teamAuditActor `json:"actor"`
}

func (g *Gateway) registerTeamAudit(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams/{teamId}/audit", g.teamAudit)
}

// The private management ledger is bounded across the installation. Filter by
// exact owned team first, then bound the response; unrelated actor activity,
// authentication events and other teams never enter this projection.
func (g *Gateway) teamAudit(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil || !access.Permissions.Admin {
		fail(w, 403, "team_admin_required", "Only this team's administrators can inspect its access and service history.")
		return
	}
	events := []teamAuditEvent{}
	for i := len(g.state.Events) - 1; i >= 0 && len(events) < teamAuditLimit; i-- {
		event := g.state.Events[i]
		if event.Target != access.Team.ID || !strings.HasPrefix(event.Action, "team.") {
			continue
		}
		person := teamAuditActor{ID: event.ActorID, DisplayName: "Former account"}
		if user, exists := g.state.Users[event.ActorID]; exists {
			person.DisplayName = user.User.DisplayName
			if strings.TrimSpace(person.DisplayName) == "" {
				person.DisplayName = user.User.Username
			}
		}
		events = append(events, teamAuditEvent{AuditEvent: event, Actor: person})
	}
	jsonReply(w, 200, map[string]any{"teamId": access.Team.ID, "events": events, "limit": teamAuditLimit, "installationRetentionLimit": 1000})
}
