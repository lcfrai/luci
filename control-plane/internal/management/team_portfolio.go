package management

import (
	"encoding/json"
	"errors"
	"net/http"
)

type teamPortfolio struct {
	Version              int               `json:"version"`
	ObservedAt           string            `json:"observedAt"`
	Status               string            `json:"status"`
	TotalProjects        int               `json:"totalProjects"`
	OmittedProjects      int               `json:"omittedProjects"`
	OmittedInterventions int               `json:"omittedInterventions"`
	Projects             []json.RawMessage `json:"projects"`
	Interventions        []json.RawMessage `json:"interventions"`
}

func filterPortfolioRows(data []byte, canRead func(string) bool) (teamPortfolio, error) {
	var out teamPortfolio
	if json.Unmarshal(data, &out) != nil || out.Version != 1 || (out.Status != "ready" && out.Status != "partial") ||
		out.TotalProjects < 0 || out.OmittedProjects < 0 || out.OmittedInterventions < 0 || len(out.Projects) > 64 || len(out.Interventions) > 128 ||
		out.TotalProjects != len(out.Projects)+out.OmittedProjects {
		return out, errors.New("invalid portfolio collection")
	}
	visible := []json.RawMessage{}
	allowed, seen := map[string]bool{}, map[string]bool{}
	for _, raw := range out.Projects {
		var row struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &row) != nil || !teamProjectIDPattern.MatchString(row.ID) || seen[row.ID] {
			return out, errors.New("invalid portfolio project")
		}
		seen[row.ID] = true
		if canRead(row.ID) {
			allowed[row.ID] = true
			visible = append(visible, raw)
		}
	}
	interventions := []json.RawMessage{}
	itemIDs := map[string]bool{}
	for _, raw := range out.Interventions {
		var item struct {
			ID        string `json:"id"`
			ProjectID string `json:"projectId"`
		}
		if json.Unmarshal(raw, &item) != nil || item.ID == "" || len(item.ID) > 200 || itemIDs[item.ID] || !seen[item.ProjectID] {
			return out, errors.New("invalid portfolio intervention")
		}
		itemIDs[item.ID] = true
		if allowed[item.ProjectID] {
			interventions = append(interventions, raw)
		}
	}
	if len(visible) != len(out.Projects) || out.OmittedProjects > 0 || out.OmittedInterventions > 0 {
		out.Status = "partial"
	}
	// Do not reveal counts belonging to newly revoked or uninspected projects.
	// Partial remains explicit; these counts cover only the returned projection.
	out.Projects, out.Interventions = visible, interventions
	out.TotalProjects, out.OmittedProjects, out.OmittedInterventions = len(visible), 0, 0
	return out, nil
}

func (g *Gateway) filterTeamPortfolio(w http.ResponseWriter, capture *teamCapturedResponse, teamID, actorID string) {
	if capture.status != http.StatusOK {
		copyTeamResponse(w, capture)
		return
	}
	out, err := filterPortfolioRows(capture.body.Bytes(), func(projectID string) bool {
		grant, err := g.resolveTeamAccess(teamID, actorID, projectID)
		return err == nil && grant.Permissions.Read
	})
	if err != nil {
		fail(w, 503, "team_catalogue_unavailable", "Project attention could not be verified.")
		return
	}
	jsonReply(w, 200, out)
}
