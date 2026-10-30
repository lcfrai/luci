package management

import (
	"encoding/json"
	"errors"
	"net/http"
)

// These projections avoid a dependency from management back into product.
// Every response carries explicit project references, including applications
// with zero actions and applications whose goal was later retargeted.
func portfolioGoalResponseProjects(data []byte) ([]string, error) {
	if len(data) > 2<<20 {
		return nil, errors.New("portfolio response over limit")
	}
	all := map[string]bool{}
	var collect func(json.RawMessage, int) error
	collect = func(raw json.RawMessage, depth int) error {
		if depth > 2 {
			return errors.New("portfolio response nesting")
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal(raw, &envelope) != nil || envelope == nil {
			return errors.New("invalid portfolio response")
		}
		add := func(id string) error {
			if !teamProjectIDPattern.MatchString(id) {
				return errors.New("invalid project reference")
			}
			all[id] = true
			return nil
		}
		if goals, ok := envelope["goals"]; ok {
			var rows, applications []json.RawMessage
			if json.Unmarshal(goals, &rows) != nil || rows == nil || len(rows) > 16 || json.Unmarshal(envelope["applications"], &applications) != nil || applications == nil || len(applications) > 32 {
				return errors.New("invalid portfolio collection")
			}
			for _, row := range append(rows, applications...) {
				if err := collect(row, depth+1); err != nil {
					return err
				}
			}
			if raw, present := envelope["outcomeRouting"]; present {
				var routing struct {
					Routes []struct {
						GoalID       string `json:"goalId"`
						GoalRevision int    `json:"goalRevision"`
						ProjectID    string `json:"projectId"`
					} `json:"routes"`
				}
				if json.Unmarshal(raw, &routing) != nil || routing.Routes == nil || len(routing.Routes) > 32 {
					return errors.New("invalid outcome routes")
				}
				type goalRef struct {
					ID       string `json:"id"`
					Revision int    `json:"revision"`
					Projects []struct {
						ProjectID string `json:"projectId"`
					} `json:"projects"`
				}
				known := map[string]goalRef{}
				for _, raw := range rows {
					var goal goalRef
					if json.Unmarshal(raw, &goal) != nil {
						return errors.New("invalid outcome goal")
					}
					known[goal.ID] = goal
				}
				for _, route := range routing.Routes {
					goal, found := known[route.GoalID]
					linked := false
					for _, project := range goal.Projects {
						linked = linked || project.ProjectID == route.ProjectID
					}
					if !found || goal.Revision != route.GoalRevision || !linked {
						return errors.New("orphan or stale outcome route")
					}
					if err := add(route.ProjectID); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if projects, ok := envelope["projects"]; ok {
			var rows []struct {
				ProjectID string `json:"projectId"`
			}
			if json.Unmarshal(projects, &rows) != nil || len(rows) < 1 || len(rows) > 16 {
				return errors.New("invalid goal projects")
			}
			for _, row := range rows {
				if err := add(row.ProjectID); err != nil {
					return err
				}
			}
			return nil
		}
		if projects, ok := envelope["projectIds"]; ok {
			var ids []string
			if json.Unmarshal(projects, &ids) != nil || len(ids) < 1 || len(ids) > 16 {
				return errors.New("invalid application projects")
			}
			for _, id := range ids {
				if err := add(id); err != nil {
					return err
				}
			}
			var results []struct {
				Action struct {
					ProjectID string `json:"projectId"`
				} `json:"action"`
			}
			if json.Unmarshal(envelope["results"], &results) != nil || results == nil || len(results) > 64 {
				return errors.New("invalid application actions")
			}
			selected := map[string]bool{}
			for _, id := range ids {
				selected[id] = true
			}
			for _, row := range results {
				if !selected[row.Action.ProjectID] {
					return errors.New("orphan application action")
				}
			}
			return nil
		}
		if allocations, ok := envelope["allocations"]; ok {
			var rows []struct {
				ProjectID string `json:"projectId"`
			}
			if json.Unmarshal(allocations, &rows) != nil || len(rows) < 1 || len(rows) > 16 {
				return errors.New("invalid review allocations")
			}
			selected := map[string]bool{}
			for _, row := range rows {
				if err := add(row.ProjectID); err != nil {
					return err
				}
				selected[row.ProjectID] = true
			}
			var actions []struct {
				ProjectID string `json:"projectId"`
			}
			if json.Unmarshal(envelope["actions"], &actions) != nil || actions == nil || len(actions) > 64 {
				return errors.New("invalid review actions")
			}
			for _, action := range actions {
				if !selected[action.ProjectID] {
					return errors.New("orphan review action")
				}
			}
			return nil
		}
		return errors.New("unknown portfolio response shape")
	}
	if err := collect(data, 0); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	return ids, nil
}

func (g *Gateway) recheckTeamPortfolioGoals(w http.ResponseWriter, capture *teamCapturedResponse, teamID, actorID string, mutation bool) {
	if capture.status != http.StatusOK {
		copyTeamResponse(w, capture)
		return
	}
	ids, err := portfolioGoalResponseProjects(capture.body.Bytes())
	if err != nil {
		fail(w, 503, "team_portfolio_unavailable", "The saved portfolio response could not be verified.")
		return
	}
	for _, id := range ids {
		access, err := g.resolveTeamAccess(teamID, actorID, id)
		if err != nil || !access.Permissions.Read {
			message := "Project access changed. The private portfolio response was withheld."
			if mutation {
				message += " Earlier saved changes are not rolled back. Read the retained application when access is restored; do not replay it automatically."
			}
			fail(w, 403, "team_access_changed", message)
			return
		}
	}
	copyTeamResponse(w, capture)
}
