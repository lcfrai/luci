package product

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const projectOverviewTextLimit = 1200
const projectOverviewQuestionLimit = 5

// A compact, recorded view for project switching. Nothing in this response is
// authority to dispatch, approve, integrate or claim that an output was verified.
type ProjectOverview struct {
	ProjectID   string                   `json:"projectId"`
	ObservedAt  string                   `json:"observedAt"`
	Status      string                   `json:"status"`
	Goal        *ProjectOverviewGoal     `json:"goal"`
	Phase       ProjectOverviewPhase     `json:"phase"`
	Context     *ProjectOverviewContext  `json:"context"`
	Questions   ProjectOverviewQuestions `json:"questions"`
	Execution   ProjectOverviewExecution `json:"execution"`
	LatestRun   ProjectOverviewRun       `json:"latestRun"`
	NextAction  OrchestratorAction       `json:"nextAction"`
	Limitations []string                 `json:"limitations"`
}
type ProjectOverviewGoal struct {
	Text      string `json:"text"`
	Revision  int    `json:"revision"`
	UpdatedAt string `json:"updatedAt"`
	Truncated bool   `json:"truncated"`
}
type ProjectOverviewPhase struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type ProjectOverviewContext struct {
	BriefRevision         int                  `json:"briefRevision"`
	SpecificationRevision int                  `json:"specificationRevision"`
	DefinitionStatus      string               `json:"definitionStatus"`
	DesignRevision        int                  `json:"designRevision"`
	DesignStatus          string               `json:"designStatus"`
	PlanRevision          int                  `json:"planRevision"`
	Tasks                 ProjectOverviewTasks `json:"tasks"`
}
type ProjectOverviewTasks struct {
	Total        int `json:"total"`
	RecordedDone int `json:"recordedDone"`
}
type ProjectOverviewQuestion struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}
type ProjectOverviewQuestions struct {
	Status  string                    `json:"status"`
	Count   *int                      `json:"count"`
	Items   []ProjectOverviewQuestion `json:"items"`
	Omitted *int                      `json:"omitted"`
}
type ProjectOverviewExecution struct {
	Scope          string `json:"scope"`
	Status         string `json:"status"`
	ObservedAt     string `json:"observedAt"`
	Queued         *int   `json:"queued"`
	Paused         *int   `json:"paused"`
	Reserved       *int   `json:"reserved"`
	Running        *int   `json:"running"`
	Blocked        *int   `json:"blocked"`
	Unknown        *int   `json:"unknown"`
	DispatchPaused *bool  `json:"dispatchPaused"`
}

func (s *Server) registerProjectOverview(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/overview", s.getProjectOverview)
}
func (s *Server) getProjectOverview(w http.ResponseWriter, r *http.Request) {
	// The gateway checks fresh membership and project grants before this handler.
	// Also fence a directly dispatched worker request before reading private data.
	if access, scoped := management.TeamAccessFromContext(r.Context()); scoped && (!access.Permissions.Read || access.ProjectID != r.PathValue("id")) {
		problem(w, http.StatusForbidden, "project_unavailable", "This project is not available in the current workspace.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	view := s.cachedProjectOverview(p)
	queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	view.Execution = ProjectOverviewExecution{Scope: "swarm-queue", Status: "unavailable"}
	if err != nil {
		view.Limitations = append(view.Limitations, "Queued work could not be read. Open this project's work queue to retry; its counts are unknown.")
		if view.Status == "ready" {
			view.Status = "partial"
		}
	} else {
		c := queue.Counts
		if queue.ObservedAt == "" {
			queue.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		view.Execution = ProjectOverviewExecution{Scope: "swarm-queue", Status: "recorded", ObservedAt: overviewTimestamp(queue.ObservedAt), Queued: &c.Queued, Paused: &c.Paused, Reserved: &c.Reserved, Running: &c.Running, Blocked: &c.Blocked, Unknown: &c.Unknown, DispatchPaused: &queue.DispatchPaused}
		// These are recorded ledger states. Opening the queue performs no implicit
		// reconciliation; the operator chooses any retry or change of execution.
		runContextNeedsReview := view.LatestRun.Status == "recorded" && view.LatestRun.ContextStatus != "current"
		if !runContextNeedsReview && c.Unknown+c.Blocked > 0 {
			view.NextAction = OrchestratorAction{ID: "inspect-queue", Label: "Review work needing attention", Lens: "develop", Reason: "Queued work has a recorded blocker or an unconfirmed outcome."}
		} else if !runContextNeedsReview && !view.LatestRun.taskReviewRequired && c.Running+c.Reserved+c.Queued+c.Paused > 0 && len(view.Questions.Items) == 0 {
			view.NextAction = OrchestratorAction{ID: "inspect-queue", Label: "Follow queued work", Lens: "develop", Reason: "Open the project's queue to inspect recorded progress or change its priority."}
		}
	}
	writeJSON(w, http.StatusOK, view)
}

func overviewText(value string, limit int) (string, bool) {
	value = strings.TrimSpace(value)
	truncated := len(value) > limit
	if truncated {
		value = value[:limit]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	// A short substring must not keep a long prompt/context string in the cache.
	return strings.Clone(value), truncated
}
func overviewInt(value int) *int { return &value }

// Canonical context can contain imported metadata. Never retain or expose an
// unbounded timestamp, and do not replace invalid history with the current time.
func overviewTimestamp(value string) string {
	if len(value) > 40 {
		return ""
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func unavailableProjectOverview(id string) ProjectOverview {
	return ProjectOverview{ProjectID: id, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "unavailable", Phase: ProjectOverviewPhase{ID: "unknown", Label: "Context unavailable"}, Questions: ProjectOverviewQuestions{Status: "unavailable", Items: []ProjectOverviewQuestion{}}, LatestRun: ProjectOverviewRun{Status: "unavailable"}, NextAction: OrchestratorAction{ID: "review-context", Label: "Open project context", Lens: "discover", Reason: "The saved project context could not be summarized."}, Limitations: []string{}}
}
func (s *Server) readProjectOverview(p project.Project) ProjectOverview {
	view := unavailableProjectOverview(p.ID)
	// Invalid private authority remains unapproved. In particular, refuse a
	// non-regular authority entry before a read could open a named pipe/device.
	state, err := readJourney(p.Path, p.ID)
	if err == nil && overviewStamp(s.stateDir, "review-approvals/"+p.ID+".json").state == "present" {
		if authority, authorityErr := s.readApprovalStore(p.Path, p.ID); authorityErr == nil {
			state.approvalAuthority = &authority
		}
	}
	if err != nil {
		view.Limitations = append(view.Limitations, "Saved project context is unavailable or invalid. Open the project to inspect it; no progress has been inferred.")
		return view
	}
	spec, design := currentSpecification(state), currentDesignPlan(state)
	plan := currentPlan(state, p.ID)
	text, truncated := overviewText(state.Brief.Purpose, projectOverviewTextLimit)
	view.Goal = &ProjectOverviewGoal{Text: text, Revision: state.Brief.Revision, UpdatedAt: overviewTimestamp(state.Brief.UpdatedAt), Truncated: truncated}
	view.Context = &ProjectOverviewContext{BriefRevision: state.Brief.Revision, SpecificationRevision: spec.Revision, DefinitionStatus: spec.Status, DesignRevision: design.Revision, DesignStatus: design.Status, PlanRevision: plan.Revision, Tasks: ProjectOverviewTasks{Total: len(plan.Tasks)}}
	for _, task := range plan.Tasks {
		if task.Status == "done" {
			view.Context.Tasks.RecordedDone++
		}
	}
	view.Status = "ready"
	view.Phase = ProjectOverviewPhase{ID: "define", Label: "Define"}
	view.NextAction = OrchestratorAction{ID: "define", Label: "Shape your idea", Lens: "discover", Reason: "Clarify the people, intended outcome and measurable requirements."}
	if spec.Revision > 0 {
		view.NextAction = OrchestratorAction{ID: "review-definition", Label: "Review the definition", Lens: "discover", Reason: "Resolve questions and review the current definition."}
	}
	if spec.Status == "approved" {
		view.Phase = ProjectOverviewPhase{ID: "design", Label: "Design"}
		view.NextAction = OrchestratorAction{ID: "review-design", Label: "Review the design", Lens: "design", Reason: "Shape screens, states and accessibility against the current definition."}
		if design.Status == "approved" {
			view.Phase = ProjectOverviewPhase{ID: "develop", Label: "Develop"}
			view.NextAction = OrchestratorAction{ID: "review-plan", Label: "Review implementation work", Lens: "develop", Reason: "Review the task plan, agent work and verification evidence. Recorded task completion alone is not verification."}
		}
	}
	latestRun, runErr := s.readOverviewLatestRun(p, state)
	applyOverviewRunProgress(&view, latestRun)
	if runErr != nil {
		view.Status = "partial"
		view.Limitations = append(view.Limitations, "The latest run cannot be identified from its compact private index. Open project work to inspect history; no run progress has been inferred.")
	}
	known := 0
	for _, question := range spec.Questions {
		if strings.TrimSpace(question.Answer) != "" {
			continue
		}
		known++
		if len(view.Questions.Items) < projectOverviewQuestionLimit {
			text, truncated := overviewText(question.Question, projectOverviewTextLimit)
			view.Questions.Items = append(view.Questions.Items, ProjectOverviewQuestion{ID: question.ID, Source: "definition", Text: text, Truncated: truncated})
		}
	}
	questions, err := s.readOverviewQuestions(p)
	if err != nil {
		view.Status = "partial"
		view.Questions.Status = "partial"
		view.Limitations = append(view.Limitations, "Private conversation questions could not be summarized within the overview's read budget. Open the conversation to inspect unresolved decisions.")
	} else {
		known += questions.Count
		for _, q := range questions.Items {
			if len(view.Questions.Items) < projectOverviewQuestionLimit {
				view.Questions.Items = append(view.Questions.Items, q)
			}
		}
		view.Questions.Status = "recorded"
		view.Questions.Count = overviewInt(known)
		view.Questions.Omitted = overviewInt(known - len(view.Questions.Items))
	}
	if len(view.Questions.Items) > 0 {
		view.NextAction = OrchestratorAction{ID: "answer-question", Label: "Review open questions", Lens: "discover", Reason: "Resolve the open decisions before starting new implementation work."}
	}
	return view
}
