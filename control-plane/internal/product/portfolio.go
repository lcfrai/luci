package product

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const PortfolioProjectLimit = 64
const portfolioInterventionLimit = 128

// Portfolio is a bounded observation, never authority to dispatch or approve.
// Its sources are sampled independently; it is not an atomic execution plan.
type Portfolio struct {
	Version              int                     `json:"version"`
	ObservedAt           string                  `json:"observedAt"`
	Status               string                  `json:"status"`
	TotalProjects        int                     `json:"totalProjects"`
	OmittedProjects      int                     `json:"omittedProjects"`
	OmittedInterventions int                     `json:"omittedInterventions"`
	Projects             []PortfolioProject      `json:"projects"`
	Interventions        []PortfolioIntervention `json:"interventions"`
}

type PortfolioProject struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Status      string               `json:"status"`
	Coordinator PortfolioCoordinator `json:"coordinator"`
}

// Only the current actor's continuation identities/states are included. No
// prompts, paths, model answers, credentials or another actor's dialogue enter it.
type PortfolioCoordinator struct {
	Status     string `json:"status"` // recorded, none, unavailable
	ID         string `json:"id,omitempty"`
	Revision   int    `json:"revision,omitempty"`
	State      string `json:"state,omitempty"`
	RunID      string `json:"runId,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	Unresolved int    `json:"unresolved"`
}

type PortfolioEvidence struct {
	Source     string `json:"source"`
	ID         string `json:"id"`
	Revision   int    `json:"revision,omitempty"`
	ObservedAt string `json:"observedAt,omitempty"`
}

type PortfolioIntervention struct {
	ID         string              `json:"id"`
	ProjectID  string              `json:"projectId"`
	Kind       string              `json:"kind"`
	Title      string              `json:"title"`
	Reason     string              `json:"reason"`
	Blocks     string              `json:"blocks"`
	NextAction OrchestratorAction  `json:"nextAction"`
	Evidence   []PortfolioEvidence `json:"evidence"`
}

// ReadPortfolioProject accepts an already authorized project, like
// HomeProjectSignals. The gateway must recheck the grant before publication.
// All runtime, model, repair and reconciliation paths are deliberately absent.
func (s *Server) ReadPortfolioProject(ctx context.Context, p project.Project, actorID string) (PortfolioProject, []PortfolioIntervention, error) {
	if actorID == "" {
		return PortfolioProject{}, nil, ErrOrchestratorInvalid
	}
	if err := ctx.Err(); err != nil {
		return PortfolioProject{}, nil, err
	}
	view := unavailableProjectOverview(p.ID)
	release, err := s.AcquireGitSessionAdmission(ctx, p.ID)
	if err == nil {
		view, err = s.cachedProjectOverview(ctx, p)
		release()
	}
	if err != nil {
		if ctx.Err() != nil {
			return PortfolioProject{}, nil, ctx.Err()
		}
		view = unavailableProjectOverview(p.ID)
	}
	queue, queueErr := s.ProjectSwarmQueueSnapshot(p.ID)
	s.mu.RLock()
	private, privateErr := s.readContinuations(p, actorID)
	s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return PortfolioProject{}, nil, err
	}
	row, items := portfolioProject(p, view, queue, queueErr, private, privateErr)
	return row, items, nil
}

func portfolioProject(p project.Project, view ProjectOverview, queue ProjectSwarmQueue, queueErr error, private coordinatorContinuations, privateErr error) (PortfolioProject, []PortfolioIntervention) {
	name, _ := overviewText(p.Name, 200)
	row := PortfolioProject{ID: p.ID, Name: name, Status: "ready", Coordinator: PortfolioCoordinator{Status: "none"}}
	items := []PortfolioIntervention{}
	add := func(key, kind, title, reason, blocks string, action OrchestratorAction, evidence PortfolioEvidence) {
		items = append(items, PortfolioIntervention{ID: reviewDigest([]string{p.ID, key}), ProjectID: p.ID, Kind: kind,
			Title: title, Reason: reason, Blocks: blocks, NextAction: action, Evidence: []PortfolioEvidence{evidence}})
	}
	openWork := func(runID string) OrchestratorAction {
		return OrchestratorAction{ID: "inspect-run", Label: "Review project work", Lens: "develop", RunID: runID}
	}
	if view.Status != "ready" {
		row.Status = "partial"
		add("overview-unavailable", "unconfirmed", "Confirm project context", "Some saved project evidence could not be summarized.", "Current context and delivery readiness are unconfirmed.", OrchestratorAction{ID: "review-context", Label: "Open project context", Lens: "discover"}, PortfolioEvidence{Source: "overview", ID: p.ID, ObservedAt: view.ObservedAt})
	}
	if view.LatestRun.Status == "recorded" && view.LatestRun.ContextStatus != "current" {
		add("context/"+view.LatestRun.ID, "context", "Review changed build context", "The retained build does not have confirmed current project context.", "Its old approval cannot authorize a new build against changed context.", openWork(view.LatestRun.ID), PortfolioEvidence{Source: "run", ID: view.LatestRun.ID, ObservedAt: view.LatestRun.UpdatedAt})
	}
	if view.Questions.Count != nil && *view.Questions.Count > 0 {
		add("questions", "question", "Answer a project question", "The project has a recorded unanswered question.", "The affected decision remains unresolved; unrelated queued work has separate authority.", OrchestratorAction{ID: "answer-question", Label: "Review questions", Lens: "discover"}, PortfolioEvidence{Source: "questions", ID: p.ID, ObservedAt: view.ObservedAt})
	}
	for _, attention := range view.Attention.Items {
		kind, title := "review", "Review retained worker output"
		if attention.Kind == "task-question" {
			kind, title = "question", "Answer a worker question"
		}
		add("attention/"+attention.ID, kind, title, "A saved run retains an unresolved review or question.", "This run's output cannot be treated as accepted or integrated.", openWork(attention.ID), PortfolioEvidence{Source: "run", ID: attention.ID, ObservedAt: view.ObservedAt})
	}
	if view.Attention.Status != "recorded" || view.Attention.Count == nil || view.Attention.Omitted == nil || *view.Attention.Omitted > 0 {
		row.Status = "partial"
		kind, reason := "unconfirmed", "The overview cannot establish the full set of retained run obligations."
		if view.Attention.Status == "recorded" && view.Attention.Omitted != nil && *view.Attention.Omitted > 0 {
			kind, reason = "review", "The overview omits additional retained run obligations."
		}
		action := openWork("")
		action.Label = "Open full project work"
		add("attention-incomplete", kind, "Inspect the full build history", reason, "Additional reviews or questions may need attention before their runs can be accepted.", action, PortfolioEvidence{Source: "run-history", ID: p.ID, ObservedAt: view.ObservedAt})
	}
	if queueErr != nil {
		row.Status = "partial"
		add("queue-unavailable", "unconfirmed", "Confirm queued work", "The saved work queue is unavailable.", "Dispatch and worker outcomes are unconfirmed.", openWork(""), PortfolioEvidence{Source: "queue", ID: p.ID})
	} else {
		for _, item := range queue.Items {
			kind, title, reason := "", "", ""
			switch item.State {
			case "unknown":
				kind, title, reason = "unconfirmed", "Confirm a worker outcome", "The scheduler has not confirmed this attempt's outcome."
			case "blocked", "failed":
				kind, title, reason = "blocked", "Review blocked work", "A recorded blocker or failed attempt needs review before this task can continue."
			case "paused":
				kind, title, reason = "paused", "Review paused work", "This waiting task is explicitly paused."
			case "queued":
				if item.ReasonCode == "capacity" {
					kind, title, reason = "capacity", "Review waiting work", "This task is waiting on a shared or project agent limit."
				} else if queue.DispatchPaused {
					kind, title, reason = "paused", "Review paused dispatch", "Shared dispatch is paused; this task has not started."
				}
			}
			if kind == "" {
				continue
			}
			task, _ := overviewText(item.TaskTitle, 200)
			add("queue/"+item.ID, kind, title, reason, "Affected task: "+task, openWork(item.RunID), PortfolioEvidence{Source: "queue-item", ID: item.ID, Revision: item.Revision, ObservedAt: item.UpdatedAt})
		}
		if queue.HasMore {
			row.Status = "partial"
			add("queue-incomplete", "unconfirmed", "Inspect the full work queue", "This bounded queue view omits older entries.", "The intervention list is incomplete; open the project queue for retained work.", openWork(""), PortfolioEvidence{Source: "queue", ID: p.ID, Revision: queue.Revision, ObservedAt: queue.ObservedAt})
		}
	}
	if privateErr != nil {
		row.Status = "partial"
		row.Coordinator.Status = "unavailable"
		add("coordinator-unavailable", "unconfirmed", "Confirm Luci's continuation", "Your saved coordinator records could not be read.", "Automatic continuation is unconfirmed, not idle.", OrchestratorAction{ID: "open-conversation", Label: "Open Luci conversation", Lens: "discover"}, PortfolioEvidence{Source: "coordinator", ID: p.ID})
	} else {
		for _, continuation := range private.Rows {
			c := continuation.CoordinatorContinuation
			if row.Coordinator.Status != "recorded" || c.UpdatedAt > row.Coordinator.UpdatedAt || c.UpdatedAt == row.Coordinator.UpdatedAt && c.ID > row.Coordinator.ID {
				unresolved := row.Coordinator.Unresolved
				row.Coordinator = PortfolioCoordinator{Status: "recorded", ID: c.ID, Revision: c.Revision, State: c.Status, RunID: c.RunID, UpdatedAt: c.UpdatedAt, Unresolved: unresolved}
			}
			kind := ""
			if c.Status == "unknown" {
				kind = "unconfirmed"
			} else if c.Status == "blocked" || c.Status == "refused" {
				kind = "blocked"
			}
			if kind != "" {
				row.Coordinator.Unresolved++
				add("coordinator/"+c.ID, kind, "Review Luci's continuation", "The saved coordinator continuation is "+c.Status+".", "This continuation cannot establish further delivery progress.", openWork(c.RunID), PortfolioEvidence{Source: "coordinator", ID: c.ID, Revision: c.Revision, ObservedAt: c.UpdatedAt})
			}
		}
		for _, finding := range private.Findings {
			if finding.Status != "question" && finding.Status != "review" && finding.Status != "unknown" {
				continue
			}
			kind := "question"
			if finding.Status == "unknown" {
				kind = "unconfirmed"
			} else if finding.Status == "review" {
				kind = "review"
			}
			add("finding/"+finding.ID, kind, "Review your Luci follow-up", "Your conversation has a retained follow-up needing attention.", "Its proposed decision is not accepted project context.", OrchestratorAction{ID: "open-conversation", Label: "Open Luci conversation", Lens: "discover"}, PortfolioEvidence{Source: "finding", ID: finding.ID, ObservedAt: finding.UpdatedAt})
		}
	}
	return row, items
}

// AssemblePortfolio is pure. The caller supplies only freshly authorized rows.
// Ordering prioritizes uncertainty and decisions, not predicted productivity.
func AssemblePortfolio(projects []PortfolioProject, items []PortfolioIntervention, total int, now time.Time) (Portfolio, error) {
	if len(projects) > PortfolioProjectLimit || total < len(projects) {
		return Portfolio{}, errors.New("invalid portfolio bounds")
	}
	ids := map[string]bool{}
	status := "ready"
	for _, p := range projects {
		if p.ID == "" || ids[p.ID] {
			return Portfolio{}, errors.New("invalid portfolio project")
		}
		ids[p.ID] = true
		if p.Status != "ready" {
			status = "partial"
		}
	}
	rank := map[string]int{"unconfirmed": 0, "question": 1, "context": 2, "review": 3, "blocked": 4, "paused": 5, "capacity": 6}
	rows := append([]PortfolioIntervention{}, items...)
	seen := map[string]bool{}
	for _, item := range rows {
		if _, ok := rank[item.Kind]; !ok || !ids[item.ProjectID] || item.ID == "" || seen[item.ID] {
			return Portfolio{}, errors.New("invalid portfolio intervention")
		}
		seen[item.ID] = true
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if rank[a.Kind] != rank[b.Kind] {
			return rank[a.Kind] < rank[b.Kind]
		}
		if a.ProjectID != b.ProjectID {
			return a.ProjectID < b.ProjectID
		}
		return a.ID < b.ID
	})
	omitted := max(0, len(rows)-portfolioInterventionLimit)
	if omitted > 0 {
		rows = rows[:portfolioInterventionLimit]
	}
	if omitted > 0 || total > len(projects) {
		status = "partial"
	}
	return Portfolio{Version: 1, ObservedAt: now.UTC().Format(time.RFC3339Nano), Status: status, TotalProjects: total,
		OmittedProjects: total - len(projects), OmittedInterventions: omitted, Projects: projects, Interventions: rows}, nil
}
