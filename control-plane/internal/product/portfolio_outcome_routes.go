package product

import (
	"context"
	"sort"
	"time"
)

const PortfolioOutcomeProjectLimit = 16
const PortfolioOutcomeRouteLimit = 32

// An observation and a navigation intent, never a task, approval, scheduler
// decision or conversation turn. Private goals do not enter project dialogue.
type PortfolioOutcomeRoute struct {
	ID           string                `json:"id"`
	GoalID       string                `json:"goalId"`
	GoalRevision int                   `json:"goalRevision"`
	ProjectID    string                `json:"projectId"`
	Status       string                `json:"status"` // unknown, attention, recorded, not-configured
	Reason       string                `json:"reason"`
	Coordinator  *PortfolioCoordinator `json:"coordinator,omitempty"`
	Evidence     []PortfolioEvidence   `json:"evidence"`
	NextAction   OrchestratorAction    `json:"nextAction"`
}

type PortfolioOutcomeRouting struct {
	Version         int                     `json:"version"`
	Status          string                  `json:"status"` // ready or partial observation, not execution readiness
	ObservedAt      string                  `json:"observedAt"`
	Routes          []PortfolioOutcomeRoute `json:"routes"`
	ReadProjects    int                     `json:"readProjects"`
	OmittedProjects int                     `json:"omittedProjects"`
	OmittedRoutes   int                     `json:"omittedRoutes"`
	UnknownProjects int                     `json:"unknownProjects"`
}

type portfolioOutcomeRead func(context.Context, string, string) (PortfolioProject, []PortfolioIntervention, error)

func (s *Server) ReadPortfolioGoalsWithOutcomes(ctx context.Context, access PortfolioGoalAccess) (PortfolioGoalsView, error) {
	view, err := s.ReadPortfolioGoals(ctx, access)
	if err != nil {
		return PortfolioGoalsView{}, err
	}
	return portfolioGoalOutcomes(ctx, view, access, func(ctx context.Context, id, actor string) (PortfolioProject, []PortfolioIntervention, error) {
		p, err := s.resolveProject(id)
		if err != nil {
			return PortfolioProject{}, nil, err
		}
		// Reuse the current actor's attention/continuation projection. No
		// provider, model, reconciliation or new action engine is introduced.
		return s.ReadPortfolioProject(ctx, p, actor)
	}, time.Now())
}

func validOutcomeAction(action OrchestratorAction) bool {
	if action.RunID != "" && !validRunID(action.RunID) {
		return false
	}
	if len(action.Label) == 0 || len(action.Label) > 200 || !validArtifactStoredText(action.Label) {
		return false
	}
	switch action.ID {
	case "inspect-run":
		return action.Lens == "develop"
	case "answer-question", "review-context", "open-conversation":
		return action.Lens == "discover" && action.RunID == ""
	}
	return false
}

func validOutcomeEvidence(value PortfolioEvidence) bool {
	if !prototypeReleaseID.MatchString(value.ID) || value.Revision < 0 {
		return false
	}
	if value.ObservedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return false
		}
	}
	switch value.Source {
	case "overview", "run", "questions", "run-history", "queue-item", "queue", "coordinator", "finding", "project":
		return true
	}
	return false
}

func outcomeRoute(goal PortfolioGoal, projectID string, row PortfolioProject, items []PortfolioIntervention, unavailable bool, now time.Time) PortfolioOutcomeRoute {
	open := OrchestratorAction{ID: "open-conversation", Label: "Open project Luci", Lens: "discover"}
	route := PortfolioOutcomeRoute{GoalID: goal.ID, GoalRevision: goal.Revision, ProjectID: projectID, Status: "not-configured", Reason: "No saved continuation is recorded for you in this project. Open its existing Luci conversation to review the current work and route.", Evidence: []PortfolioEvidence{{Source: "project", ID: projectID}}, NextAction: open}
	if !unavailable && row.Coordinator.Status == "recorded" {
		c := row.Coordinator
		_, dateErr := time.Parse(time.RFC3339Nano, c.UpdatedAt)
		if !prototypeReleaseID.MatchString(c.ID) || c.Revision < 1 || c.State == "" || len(c.State) > 100 || !validArtifactStoredText(c.State) || dateErr != nil || c.RunID != "" && !validRunID(c.RunID) {
			unavailable = true
		} else {
			route.Coordinator = &c
			route.Status = "recorded"
			route.Reason = "Your existing continuation is recorded. Inspect its current work; this does not establish completion or authorize a new attempt."
			route.Evidence = []PortfolioEvidence{{Source: "coordinator", ID: c.ID, Revision: c.Revision, ObservedAt: c.UpdatedAt}}
			if c.RunID != "" {
				route.NextAction = OrchestratorAction{ID: "inspect-run", Label: "Inspect recorded work", Lens: "develop", RunID: c.RunID}
			}
		}
	}
	// Existing attention ordering supplies the first useful action. Do not
	// duplicate its uncertainty/decision ranking or silently drop omissions.
	if !unavailable {
		view, err := AssemblePortfolio([]PortfolioProject{row}, items, 1, now)
		if err != nil || view.Status != "ready" {
			unavailable = true
		}
		if err == nil && len(view.Interventions) > 0 {
			first := view.Interventions[0]
			valid := validOutcomeAction(first.NextAction) && len(first.Evidence) > 0 && len(first.Evidence) <= 3 && len(first.Reason) <= 1200 && validArtifactStoredText(first.Reason)
			for _, evidence := range first.Evidence {
				valid = valid && validOutcomeEvidence(evidence)
			}
			if valid {
				route.Status, route.Reason, route.NextAction = "attention", first.Reason, first.NextAction
				if first.Kind == "unconfirmed" {
					route.Status = "unknown"
				}
				route.Evidence = append([]PortfolioEvidence{}, first.Evidence...)
				if route.Coordinator != nil {
					route.Evidence = append(route.Evidence, PortfolioEvidence{Source: "coordinator", ID: route.Coordinator.ID, Revision: route.Coordinator.Revision, ObservedAt: route.Coordinator.UpdatedAt})
				}
			} else {
				unavailable = true
			}
		}
	}
	if unavailable {
		route.Status = "unknown"
		route.Reason = "Some current project or owned continuation evidence is unavailable or omitted. Open the project to inspect it; absence here is not proof of idle or completed work."
		// A partial observation may retain a valid known navigation target,
		// but must never retain malformed evidence or an unsupported action.
		if !validOutcomeAction(route.NextAction) {
			route.NextAction = open
		}
	}
	route.ID = reviewDigest(route)
	return route
}

func portfolioGoalOutcomes(ctx context.Context, view PortfolioGoalsView, access PortfolioGoalAccess, read portfolioOutcomeRead, now time.Time) (PortfolioGoalsView, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoalsView{}, err
	}
	if access == nil || read == nil || len(view.Goals) > portfolioGoalLimit || len(view.Applications) > portfolioApplicationLimit {
		return PortfolioGoalsView{}, ErrPortfolioGoalInvalid
	}
	for _, application := range view.Applications {
		if len(application.ProjectIDs) < 1 || len(application.ProjectIDs) > portfolioGoalProjectLimit {
			return PortfolioGoalsView{}, ErrPortfolioGoalInvalid
		}
		for _, id := range application.ProjectIDs {
			if !prototypeReleaseID.MatchString(id) {
				return PortfolioGoalsView{}, ErrPortfolioGoalInvalid
			}
		}
	}
	ids, goals := map[string]bool{}, map[string]bool{}
	pairs := 0
	for _, goal := range view.Goals {
		if validatePortfolioGoal(goal) != nil || goal.ID == "" || goal.Revision < 1 || goals[goal.ID] {
			return PortfolioGoalsView{}, ErrPortfolioGoalInvalid
		}
		goals[goal.ID] = true
		for _, target := range goal.Projects {
			ids[target.ProjectID] = true
			pairs++
		}
	}
	projects := make([]string, 0, len(ids))
	for id := range ids {
		projects = append(projects, id)
	}
	sort.Strings(projects)
	cache := map[string]PortfolioOutcomeRoute{}
	// This is one overall cooperative budget, not a renewed per-project clock.
	bounded, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	for _, id := range projects {
		if len(cache) >= PortfolioOutcomeProjectLimit || bounded.Err() != nil {
			break
		}
		if err := access(ctx, id, false); err != nil {
			return PortfolioGoalsView{}, err
		}
		row, items, readErr := read(bounded, id, actor)
		if err := ctx.Err(); err != nil {
			return PortfolioGoalsView{}, err
		}
		if err := access(ctx, id, false); err != nil {
			return PortfolioGoalsView{}, err
		}
		unavailable := readErr != nil || row.ID != id || row.Status != "ready" || row.Coordinator.Status != "recorded" && row.Coordinator.Status != "none" || len(items) > MaxSwarmScheduledTasks+256
		if unavailable && (readErr != nil || row.ID != id) {
			row = PortfolioProject{ID: id, Status: "partial", Coordinator: PortfolioCoordinator{Status: "unavailable"}}
			items = nil
		}
		cache[id] = outcomeRoute(PortfolioGoal{}, id, row, items, unavailable, now)
	}
	routing := &PortfolioOutcomeRouting{Version: 1, Status: "ready", ObservedAt: now.UTC().Format(time.RFC3339Nano), Routes: []PortfolioOutcomeRoute{}, ReadProjects: len(cache), OmittedProjects: len(projects) - len(cache)}
	unknown := map[string]bool{}
	for _, goal := range view.Goals {
		for _, target := range goal.Projects {
			route, found := cache[target.ProjectID]
			if !found {
				continue
			}
			route.GoalID, route.GoalRevision = goal.ID, goal.Revision
			route.Evidence = append([]PortfolioEvidence{}, route.Evidence...)
			if route.Coordinator != nil {
				copy := *route.Coordinator
				route.Coordinator = &copy
			}
			route.ID = ""
			route.ID = reviewDigest(route)
			if route.Status == "unknown" {
				unknown[target.ProjectID] = true
			}
			if len(routing.Routes) < PortfolioOutcomeRouteLimit {
				routing.Routes = append(routing.Routes, route)
			}
		}
	}
	routing.UnknownProjects = len(unknown)
	routing.OmittedRoutes = pairs - len(routing.Routes)
	if view.Partial || routing.OmittedProjects > 0 || routing.OmittedRoutes > 0 || routing.UnknownProjects > 0 {
		routing.Status = "partial"
	}
	// Fresh deduplicated grants cover the whole returned private view, including
	// original application projects after a goal is retargeted. These are only
	// grant checks, never additional project-source reads (16x16 + 32x16 refs).
	grants := map[string]bool{}
	for _, id := range projects {
		grants[id] = true
	}
	for _, application := range view.Applications {
		for _, id := range application.ProjectIDs {
			grants[id] = true
		}
	}
	for id := range grants {
		if err := access(ctx, id, false); err != nil {
			return PortfolioGoalsView{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return PortfolioGoalsView{}, err
	}
	view.OutcomeRouting = routing
	return view, nil
}
