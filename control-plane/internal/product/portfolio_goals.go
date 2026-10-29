package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const portfolioGoalFileLimit = 2 << 20
const portfolioGoalLimit = 16
const portfolioGoalProjectLimit = 16
const portfolioPriorityActionLimit = 64
const portfolioApplicationLimit = 32

var ErrPortfolioGoalInvalid = errors.New("The portfolio goal or selected projects are invalid.")
var ErrPortfolioGoalConflict = errors.New("The saved goal or reviewed queue changed. Keep your edits and review the current plan.")
var ErrPortfolioGoalUnavailable = errors.New("The private portfolio record could not be verified. Its previous outcomes remain unconfirmed.")

// These are personal intentions, not project approval, machine policy or
// execution consent. Dependencies and requested slots are explicitly advisory.
type PortfolioGoalProject struct {
	ProjectID      string   `json:"projectId"`
	Priority       string   `json:"priority"`
	RequestedSlots int      `json:"requestedSlots"`
	DependsOn      []string `json:"dependsOn"`
}

type PortfolioGoal struct {
	ID        string                 `json:"id"`
	Revision  int                    `json:"revision"`
	Title     string                 `json:"title"`
	Outcome   string                 `json:"outcome"`
	Deadline  string                 `json:"deadline,omitempty"`
	Projects  []PortfolioGoalProject `json:"projects"`
	UpdatedAt string                 `json:"updatedAt"`
}

type PortfolioGoalsSave struct {
	ExpectedRevision int           `json:"expectedRevision"`
	Goal             PortfolioGoal `json:"goal"`
}

// The gateway supplies fresh project read/control checks. A browser cannot
// supply this callback or substitute stored roles for current grants.
type PortfolioGoalAccess func(context.Context, string, bool) error

type PortfolioPriorityAction struct {
	ProjectID        string `json:"projectId"`
	QueueID          string `json:"queueId"`
	RunID            string `json:"runId"`
	TaskID           string `json:"taskId"`
	TaskTitle        string `json:"taskTitle"`
	ExpectedRevision int    `json:"expectedRevision"`
	FromPriority     string `json:"fromPriority"`
	ToPriority       string `json:"toPriority"`
}

type PortfolioAllocation struct {
	ProjectID      string   `json:"projectId"`
	Priority       string   `json:"priority"`
	RequestedSlots int      `json:"requestedSlots"`
	RecordedActive int      `json:"recordedActive"`
	Waiting        int      `json:"waiting"`
	DependsOn      []string `json:"dependsOn"`
	Explanation    string   `json:"explanation"`
}

type PortfolioGoalPlan struct {
	GoalID                string                    `json:"goalId"`
	GoalRevision          int                       `json:"goalRevision"`
	GoalDigest            string                    `json:"goalDigest"`
	Digest                string                    `json:"digest"`
	Actions               []PortfolioPriorityAction `json:"actions"`
	Allocations           []PortfolioAllocation     `json:"allocations"`
	Decisions             []string                  `json:"decisions"`
	ExecutionStarted      bool                      `json:"executionStarted"`
	CapacityPolicyChanged bool                      `json:"capacityPolicyChanged"`
}

type PortfolioPriorityResult struct {
	Action          PortfolioPriorityAction `json:"action"`
	Status          string                  `json:"status"` // not_attempted, pending, applied, blocked, unknown
	Message         string                  `json:"message"`
	AppliedRevision int                     `json:"appliedRevision,omitempty"`
	// The marker is a private ownership witness, not client authority.
	IntentDigest string `json:"intentDigest"`
}

type PortfolioGoalApplication struct {
	ID            string                    `json:"id"`
	GoalID        string                    `json:"goalId"`
	GoalRevision  int                       `json:"goalRevision"`
	ProjectIDs    []string                  `json:"projectIds"`
	PlanDigest    string                    `json:"planDigest"`
	KeyDigest     string                    `json:"keyDigest"`
	RequestDigest string                    `json:"requestDigest"`
	Status        string                    `json:"status"`
	Results       []PortfolioPriorityResult `json:"results"`
	CreatedAt     string                    `json:"createdAt"`
}

type PortfolioGoalApply struct {
	GoalID               string `json:"goalId"`
	ExpectedGoalRevision int    `json:"expectedGoalRevision"`
	ExpectedPlanDigest   string `json:"expectedPlanDigest"`
	IdempotencyKey       string `json:"idempotencyKey"`
}

type PortfolioGoalsView struct {
	Revision     int                        `json:"revision"`
	Goals        []PortfolioGoal            `json:"goals"`
	Applications []PortfolioGoalApplication `json:"applications"`
	Partial      bool                       `json:"partial"`
}

type portfolioGoalsLedger struct {
	Version      int    `json:"version"`
	AccountScope string `json:"accountScope"`
	ActorID      string `json:"actorId"`
	PortfolioGoalsView
}

func portfolioGoalActor(ctx context.Context) (string, error) {
	actor, _, ok := management.ActorFromContext(ctx)
	if !ok || actor.Disabled || !validConnectionToken(actor.ID, 128, false) {
		return "", management.ErrExecutionAuthority
	}
	return actor.ID, ctx.Err()
}

func validatePortfolioGoal(goal PortfolioGoal) error {
	if goal.ID != "" && !prototypeReleaseID.MatchString(goal.ID) || goal.Revision < 0 || strings.TrimSpace(goal.Title) == "" || len(goal.Title) > 200 || !validArtifactStoredText(goal.Title) || strings.TrimSpace(goal.Outcome) == "" || len(goal.Outcome) > 4000 || !validArtifactStoredText(goal.Outcome) || goal.Projects == nil || len(goal.Projects) < 1 || len(goal.Projects) > portfolioGoalProjectLimit {
		return ErrPortfolioGoalInvalid
	}
	if goal.Deadline != "" {
		date, err := time.Parse("2006-01-02", goal.Deadline)
		if err != nil || date.Format("2006-01-02") != goal.Deadline {
			return ErrPortfolioGoalInvalid
		}
	}
	projects := map[string]PortfolioGoalProject{}
	for _, item := range goal.Projects {
		if !prototypeReleaseID.MatchString(item.ProjectID) || !swarmQueuePriority(item.Priority) || item.RequestedSlots < 0 || item.RequestedSlots > MaxSwarmParallelTasks || item.DependsOn == nil || len(item.DependsOn) >= len(goal.Projects) {
			return ErrPortfolioGoalInvalid
		}
		if _, duplicate := projects[item.ProjectID]; duplicate {
			return ErrPortfolioGoalInvalid
		}
		projects[item.ProjectID] = item
	}
	visited, visiting := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if visited[id] {
			return true
		}
		visiting[id] = true
		seen := map[string]bool{}
		for _, dependency := range projects[id].DependsOn {
			if _, ok := projects[dependency]; !ok || dependency == id || seen[dependency] || !visit(dependency) {
				return false
			}
			seen[dependency] = true
		}
		visiting[id] = false
		visited[id] = true
		return true
	}
	for id := range projects {
		if !visit(id) {
			return ErrPortfolioGoalInvalid
		}
	}
	return nil
}

func (s *Server) portfolioGoalsPath(actor string, create bool) (string, error) {
	if !validConnectionToken(actor, 128, false) {
		return "", ErrPortfolioGoalInvalid
	}
	dir, err := guardedProjectPath(s.stateDir, "portfolio-goals", create)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(dir)
		} else {
			err = os.Chmod(dir, 0700)
		}
		if err != nil {
			return "", err
		}
	}
	dir, err = guardedProjectPath(s.stateDir, "portfolio-goals", false)
	if err != nil {
		return "", err
	}
	path, err := guardedProjectPath(dir, reviewDigest([]string{s.projectRuns.accountScope, actor})+".json", create)
	if err == nil {
		if e := hostpath.OneName(path); e != nil && !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
	}
	return path, err
}

func (s *Server) readPortfolioGoals(actor string) (portfolioGoalsLedger, error) {
	empty := portfolioGoalsLedger{Version: 1, AccountScope: s.projectRuns.accountScope, ActorID: actor, PortfolioGoalsView: PortfolioGoalsView{Goals: []PortfolioGoal{}, Applications: []PortfolioGoalApplication{}}}
	path, err := s.portfolioGoalsPath(actor, false)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, ErrPortfolioGoalUnavailable
	}
	var saved portfolioGoalsLedger
	if err = externalReadJSON(path, portfolioGoalFileLimit, &saved); errors.Is(err, os.ErrNotExist) {
		return empty, nil
	} else if err != nil {
		return empty, ErrPortfolioGoalUnavailable
	}
	if saved.Version != 1 || saved.AccountScope != empty.AccountScope || saved.ActorID != actor || saved.Revision < 0 || saved.Goals == nil || saved.Applications == nil || len(saved.Goals) > portfolioGoalLimit || len(saved.Applications) > portfolioApplicationLimit || saved.Partial {
		return empty, ErrPortfolioGoalUnavailable
	}
	seen := map[string]bool{}
	for _, goal := range saved.Goals {
		if validatePortfolioGoal(goal) != nil || goal.ID == "" || goal.Revision < 1 || seen[goal.ID] {
			return empty, ErrPortfolioGoalUnavailable
		}
		if _, err := time.Parse(time.RFC3339Nano, goal.UpdatedAt); err != nil {
			return empty, ErrPortfolioGoalUnavailable
		}
		seen[goal.ID] = true
	}
	keys, applicationIDs := map[string]bool{}, map[string]bool{}
	for _, application := range saved.Applications {
		if !prototypeReleaseID.MatchString(application.ID) || !seen[application.GoalID] || applicationIDs[application.ID] || application.GoalRevision < 1 || !hexDigest(application.PlanDigest) || !hexDigest(application.KeyDigest) || !hexDigest(application.RequestDigest) || keys[application.KeyDigest] || application.Results == nil || len(application.Results) > portfolioPriorityActionLimit || len(application.ProjectIDs) < 1 || len(application.ProjectIDs) > portfolioGoalProjectLimit {
			return empty, ErrPortfolioGoalUnavailable
		}
		if goal := portfolioGoalByID(&saved, application.GoalID); goal == nil || application.GoalRevision > goal.Revision {
			return empty, ErrPortfolioGoalUnavailable
		}
		projects := map[string]bool{}
		for _, id := range application.ProjectIDs {
			if !prototypeReleaseID.MatchString(id) || projects[id] {
				return empty, ErrPortfolioGoalUnavailable
			}
			projects[id] = true
		}
		if _, err := time.Parse(time.RFC3339Nano, application.CreatedAt); err != nil {
			return empty, ErrPortfolioGoalUnavailable
		}
		queueIDs := map[string]bool{}
		for index, result := range application.Results {
			queueKey := result.Action.ProjectID + ":" + result.Action.QueueID
			if queueIDs[queueKey] || !projects[result.Action.ProjectID] || !validPortfolioPriorityResult(result) || result.IntentDigest != reviewDigest([]string{actor, application.ID, fmt.Sprint(index), application.PlanDigest}) {
				return empty, ErrPortfolioGoalUnavailable
			}
			queueIDs[queueKey] = true
		}
		if application.Status != portfolioApplicationStatus(application.Results) {
			return empty, ErrPortfolioGoalUnavailable
		}
		keys[application.KeyDigest] = true
		applicationIDs[application.ID] = true
	}
	return saved, nil
}

func (s *Server) savePortfolioGoals(actor string, saved *portfolioGoalsLedger) error {
	path, err := s.portfolioGoalsPath(actor, true)
	if err != nil {
		return ErrPortfolioGoalUnavailable
	}
	saved.Revision++
	data, err := json.Marshal(saved)
	if err != nil || len(data) > portfolioGoalFileLimit {
		return ErrPortfolioGoalUnavailable
	}
	if err = atomicJSON(path, saved); err != nil {
		return ErrPortfolioGoalUnavailable
	}
	return nil
}

func portfolioGoalByID(saved *portfolioGoalsLedger, id string) *PortfolioGoal {
	for i := range saved.Goals {
		if saved.Goals[i].ID == id {
			return &saved.Goals[i]
		}
	}
	return nil
}

func authorizePortfolioGoal(ctx context.Context, goal PortfolioGoal, access PortfolioGoalAccess, edit bool) error {
	if access == nil {
		return management.ErrExecutionAuthority
	}
	for _, target := range goal.Projects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := access(ctx, target.ProjectID, edit); err != nil {
			return err
		}
	}
	return nil
}

func authorizePortfolioApplication(ctx context.Context, application PortfolioGoalApplication, access PortfolioGoalAccess) error {
	if access == nil {
		return management.ErrExecutionAuthority
	}
	for _, projectID := range application.ProjectIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := access(ctx, projectID, false); err != nil {
			return err
		}
	}
	return nil
}

// Read never creates directories, repairs receipts or advances a coordinator.
func (s *Server) ReadPortfolioGoals(ctx context.Context, access PortfolioGoalAccess) (PortfolioGoalsView, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoalsView{}, err
	}
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	saved, err := s.readPortfolioGoals(actor)
	if err != nil {
		return PortfolioGoalsView{}, err
	}
	view := PortfolioGoalsView{Revision: saved.Revision, Goals: []PortfolioGoal{}, Applications: []PortfolioGoalApplication{}}
	allowed := map[string]bool{}
	for _, goal := range saved.Goals {
		if authorizePortfolioGoal(ctx, goal, access, false) != nil {
			view.Partial = true
			continue
		}
		view.Goals = append(view.Goals, goal)
		allowed[goal.ID] = true
	}
	for _, application := range saved.Applications {
		if allowed[application.GoalID] && authorizePortfolioApplication(ctx, application, access) == nil {
			view.Applications = append(view.Applications, application)
		} else {
			view.Partial = true
		}
	}
	for _, goal := range view.Goals {
		if err := authorizePortfolioGoal(ctx, goal, access, false); err != nil {
			return PortfolioGoalsView{}, err
		}
	}
	for _, application := range view.Applications {
		if err := authorizePortfolioApplication(ctx, application, access); err != nil {
			return PortfolioGoalsView{}, err
		}
	}
	return view, ctx.Err()
}

func (s *Server) SavePortfolioGoal(ctx context.Context, req PortfolioGoalsSave, access PortfolioGoalAccess) (PortfolioGoal, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoal{}, err
	}
	if validatePortfolioGoal(req.Goal) != nil {
		return PortfolioGoal{}, ErrPortfolioGoalInvalid
	}
	if err := authorizePortfolioGoal(ctx, req.Goal, access, false); err != nil {
		return PortfolioGoal{}, err
	}
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	saved, err := s.readPortfolioGoals(actor)
	if err != nil {
		return PortfolioGoal{}, err
	}
	if req.ExpectedRevision != saved.Revision {
		return PortfolioGoal{}, ErrPortfolioGoalConflict
	}
	previous := portfolioGoalByID(&saved, req.Goal.ID)
	if previous == nil && req.Goal.ID != "" || previous != nil && previous.Revision != req.Goal.Revision || previous == nil && req.Goal.Revision != 0 {
		return PortfolioGoal{}, ErrPortfolioGoalConflict
	}
	goal := req.Goal
	goal.Projects = append([]PortfolioGoalProject(nil), req.Goal.Projects...)
	for i := range goal.Projects {
		goal.Projects[i].DependsOn = append([]string{}, req.Goal.Projects[i].DependsOn...)
	}
	if previous == nil {
		if len(saved.Goals) >= portfolioGoalLimit {
			return PortfolioGoal{}, ErrPortfolioGoalInvalid
		}
		goal.ID = newID("goal-")
		goal.Revision = 1
	} else {
		goal.Revision = previous.Revision + 1
	}
	goal.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	for i := range goal.Projects {
		sort.Strings(goal.Projects[i].DependsOn)
	}
	sort.Slice(goal.Projects, func(i, j int) bool { return goal.Projects[i].ProjectID < goal.Projects[j].ProjectID })
	if previous == nil {
		saved.Goals = append(saved.Goals, goal)
	} else {
		*previous = goal
	}
	if err := authorizePortfolioGoal(ctx, goal, access, false); err != nil {
		return PortfolioGoal{}, err
	}
	if err := s.savePortfolioGoals(actor, &saved); err != nil {
		return PortfolioGoal{}, err
	}
	if err := authorizePortfolioGoal(ctx, goal, access, false); err != nil {
		return goal, err
	}
	return goal, nil
}

func (s *Server) reviewPortfolioGoal(ctx context.Context, goal PortfolioGoal, access PortfolioGoalAccess) (PortfolioGoalPlan, error) {
	if err := authorizePortfolioGoal(ctx, goal, access, false); err != nil {
		return PortfolioGoalPlan{}, err
	}
	plan := PortfolioGoalPlan{GoalID: goal.ID, GoalRevision: goal.Revision, GoalDigest: reviewDigest(goal), Actions: []PortfolioPriorityAction{}, Allocations: []PortfolioAllocation{}, Decisions: []string{}}
	for _, target := range goal.Projects {
		queue, err := s.ProjectSwarmQueueSnapshot(target.ProjectID)
		if err != nil {
			return PortfolioGoalPlan{}, ErrPortfolioGoalUnavailable
		}
		allocation := PortfolioAllocation{ProjectID: target.ProjectID, Priority: target.Priority, RequestedSlots: target.RequestedSlots, RecordedActive: queue.Counts.Reserved + queue.Counts.Running + queue.Counts.Unknown, Waiting: queue.Counts.Queued, DependsOn: target.DependsOn, Explanation: "Requested slots and cross-project dependencies are planning intentions. Existing machine/account/provider limits and task dependencies remain authoritative. Apply changes only the exact reviewed waiting-item priorities; it does not enqueue, resume or start work."}
		plan.Allocations = append(plan.Allocations, allocation)
		if queue.HasMore {
			plan.Decisions = append(plan.Decisions, "The queue for "+target.ProjectID+" omits older entries. Inspect its full queue before applying portfolio priorities.")
			continue
		}
		if access(ctx, target.ProjectID, true) != nil {
			plan.Decisions = append(plan.Decisions, "Current queue-control permission is missing for "+target.ProjectID+".")
			continue
		}
		for _, item := range queue.Items {
			if swarmQueueActive(item.State) || swarmQueueTerminal(item.State) || item.Priority == target.Priority {
				continue
			}
			plan.Actions = append(plan.Actions, PortfolioPriorityAction{ProjectID: target.ProjectID, QueueID: item.ID, RunID: item.RunID, TaskID: item.TaskID, TaskTitle: item.TaskTitle, ExpectedRevision: item.Revision, FromPriority: item.Priority, ToPriority: target.Priority})
			if len(plan.Actions) > portfolioPriorityActionLimit {
				return PortfolioGoalPlan{}, ErrPortfolioGoalInvalid
			}
		}
	}
	sort.Slice(plan.Actions, func(i, j int) bool {
		return plan.Actions[i].ProjectID+"/"+plan.Actions[i].QueueID < plan.Actions[j].ProjectID+"/"+plan.Actions[j].QueueID
	})
	plan.Digest = reviewDigest(plan)
	if err := authorizePortfolioGoal(ctx, goal, access, false); err != nil {
		return PortfolioGoalPlan{}, err
	}
	return plan, nil
}

func (s *Server) ReviewPortfolioGoal(ctx context.Context, goalID string, revision int, access PortfolioGoalAccess) (PortfolioGoalPlan, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoalPlan{}, err
	}
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	saved, err := s.readPortfolioGoals(actor)
	if err != nil {
		return PortfolioGoalPlan{}, err
	}
	goal := portfolioGoalByID(&saved, goalID)
	if goal == nil || goal.Revision != revision {
		return PortfolioGoalPlan{}, ErrPortfolioGoalConflict
	}
	return s.reviewPortfolioGoal(ctx, *goal, access)
}

func validPortfolioPriorityResult(result PortfolioPriorityResult) bool {
	a := result.Action
	if !prototypeReleaseID.MatchString(a.ProjectID) || !prototypeReleaseID.MatchString(a.QueueID) || !validRunID(a.RunID) || !designIDPattern.MatchString(a.TaskID) || a.ExpectedRevision < 1 || !swarmQueuePriority(a.FromPriority) || !swarmQueuePriority(a.ToPriority) || !hexDigest(result.IntentDigest) || len(a.TaskTitle) > 2000 || !validArtifactStoredText(a.TaskTitle) || len(result.Message) > 300 || !validArtifactStoredText(result.Message) || result.AppliedRevision < 0 || result.Status == "applied" && result.AppliedRevision != a.ExpectedRevision+1 || result.Status != "applied" && result.AppliedRevision != 0 {
		return false
	}
	return result.Status == "not_attempted" || result.Status == "pending" || result.Status == "applied" || result.Status == "blocked" || result.Status == "unknown"
}

func portfolioApplicationStatus(results []PortfolioPriorityResult) string {
	if len(results) == 0 {
		return "no_changes"
	}
	applied := 0
	for _, result := range results {
		if result.Status == "applied" {
			applied++
		}
		if result.Status == "pending" || result.Status == "unknown" {
			return "unknown"
		}
	}
	if applied == len(results) {
		return "applied"
	}
	if applied > 0 {
		return "partial"
	}
	return "not_applied"
}

// A returned pending/unknown write is never automatically retried. Reusing the
// same key returns retained evidence only; a new effect requires a new review.
func (s *Server) ApplyPortfolioGoal(ctx context.Context, req PortfolioGoalApply, access PortfolioGoalAccess) (PortfolioGoalApplication, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoalApplication{}, err
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedPlanDigest) {
		return PortfolioGoalApplication{}, ErrPortfolioGoalInvalid
	}
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	saved, err := s.readPortfolioGoals(actor)
	if err != nil {
		return PortfolioGoalApplication{}, err
	}
	goal := portfolioGoalByID(&saved, req.GoalID)
	if goal == nil {
		return PortfolioGoalApplication{}, ErrPortfolioGoalConflict
	}
	if err := authorizePortfolioGoal(ctx, *goal, access, false); err != nil {
		return PortfolioGoalApplication{}, err
	}
	key := reviewDigest([]string{actor, req.IdempotencyKey})
	requestDigest := reviewDigest(req)
	for _, old := range saved.Applications {
		if old.KeyDigest == key {
			if old.RequestDigest != requestDigest {
				return PortfolioGoalApplication{}, ErrPortfolioGoalConflict
			}
			if err := authorizePortfolioApplication(ctx, old, access); err != nil {
				return PortfolioGoalApplication{}, err
			}
			out := s.observePortfolioApplication(old)
			if err := authorizePortfolioApplication(ctx, old, access); err != nil {
				return PortfolioGoalApplication{}, err
			}
			return out, ctx.Err()
		}
	}
	if goal.Revision != req.ExpectedGoalRevision || len(saved.Applications) >= portfolioApplicationLimit {
		return PortfolioGoalApplication{}, ErrPortfolioGoalConflict
	}
	plan, err := s.reviewPortfolioGoal(ctx, *goal, access)
	if err != nil {
		return PortfolioGoalApplication{}, err
	}
	if plan.Digest != req.ExpectedPlanDigest || len(plan.Decisions) != 0 {
		return PortfolioGoalApplication{}, ErrPortfolioGoalConflict
	}
	application := PortfolioGoalApplication{ID: newID("portfolio-"), GoalID: goal.ID, GoalRevision: goal.Revision, PlanDigest: plan.Digest, KeyDigest: key, RequestDigest: requestDigest, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Results: []PortfolioPriorityResult{}}
	for _, target := range goal.Projects {
		application.ProjectIDs = append(application.ProjectIDs, target.ProjectID)
	}
	for i, action := range plan.Actions {
		application.Results = append(application.Results, PortfolioPriorityResult{Action: action, Status: "not_attempted", Message: "Not attempted.", IntentDigest: reviewDigest([]string{actor, application.ID, fmt.Sprint(i), plan.Digest})})
	}
	application.Status = portfolioApplicationStatus(application.Results)
	saved.Applications = append(saved.Applications, application)
	index := len(saved.Applications) - 1
	if err := s.savePortfolioGoals(actor, &saved); err != nil {
		return PortfolioGoalApplication{}, err
	}
	for i := range saved.Applications[index].Results {
		result := &saved.Applications[index].Results[i]
		if ctx.Err() != nil {
			break
		}
		if err := access(ctx, result.Action.ProjectID, true); err != nil {
			result.Status = "blocked"
			result.Message = "Current project queue-control permission changed."
		} else {
			result.Status = "pending"
			result.Message = "Priority write reserved; result not yet confirmed."
			saved.Applications[index].Status = portfolioApplicationStatus(saved.Applications[index].Results)
			if err := s.savePortfolioGoals(actor, &saved); err != nil {
				return saved.Applications[index], err
			}
			revision, attempted, err := s.applyPortfolioPriority(ctx, result.Action, result.IntentDigest, access)
			if err == nil {
				result.Status = "applied"
				result.AppliedRevision = revision
				result.Message = "Exact reviewed queue priority saved. No work was started."
			} else if attempted {
				result.Status = "unknown"
				result.Message = "Write outcome unconfirmed. Refresh the retained application; do not retry automatically."
			} else {
				result.Status = "blocked"
				result.Message = "The reviewed queue item or current permission changed. Review a fresh plan."
			}
		}
		saved.Applications[index].Status = portfolioApplicationStatus(saved.Applications[index].Results)
		if err := s.savePortfolioGoals(actor, &saved); err != nil {
			return saved.Applications[index], err
		}
		if result.Status == "unknown" {
			break
		}
	}
	saved.Applications[index].Status = portfolioApplicationStatus(saved.Applications[index].Results)
	if err := s.savePortfolioGoals(actor, &saved); err != nil {
		return saved.Applications[index], err
	}
	if err := authorizePortfolioApplication(ctx, saved.Applications[index], access); err != nil {
		return saved.Applications[index], err
	}
	return saved.Applications[index], nil
}

func (s *Server) applyPortfolioPriority(ctx context.Context, action PortfolioPriorityAction, intent string, access PortfolioGoalAccess) (int, bool, error) {
	// This is the ordinary queue-bookkeeping contract, not a new execution
	// admission. Journey edits need not serialize with priority preferences;
	// future dispatch independently checks current source, approvals and limits.
	// In particular do not hold s.mu around scheduler reads: shared policy reads
	// use projectRoot, which takes s.mu.RLock itself.
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return 0, false, err
	}
	for i := range row.Items {
		item := &row.Items[i]
		if item.ID != action.QueueID || item.ProjectID != action.ProjectID {
			continue
		}
		if item.RunID != action.RunID || item.TaskID != action.TaskID || item.TaskTitle != action.TaskTitle || item.Priority != action.FromPriority {
			return 0, false, ErrPortfolioGoalConflict
		}
		if err := access(ctx, action.ProjectID, true); err != nil {
			return 0, false, err
		}
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		if err := setSwarmQueuePriority(item, action.ExpectedRevision, action.ToPriority, intent); err != nil {
			return 0, false, err
		}
		item.Revision++
		item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
		// Deliberately no Wake/Resume/Enqueue/Dispatch call. The existing
		// independently authorized scheduler owns any later task admission.
		return item.Revision, true, s.saveSwarmScheduler(&row)
	}
	return 0, false, ErrPortfolioGoalConflict
}

func (s *Server) observePortfolioApplication(application PortfolioGoalApplication) PortfolioGoalApplication {
	// This is read-only recovery, not a replay or repair of either ledger.
	data, _ := json.Marshal(application)
	var out PortfolioGoalApplication
	_ = json.Unmarshal(data, &out)
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	for i := range out.Results {
		result := &out.Results[i]
		if result.Status != "pending" && result.Status != "unknown" {
			continue
		}
		result.Status = "unknown"
		result.Message = "The retained write result remains unconfirmed; no replay was issued."
		if err != nil {
			continue
		}
		for _, item := range row.Items {
			if item.ProjectID == result.Action.ProjectID && item.ID == result.Action.QueueID && item.RunID == result.Action.RunID && item.TaskID == result.Action.TaskID && item.PriorityChangeDigest == result.IntentDigest && item.Priority == result.Action.ToPriority && item.Revision >= result.Action.ExpectedRevision+1 {
				result.Status = "applied"
				result.AppliedRevision = result.Action.ExpectedRevision + 1
				result.Message = "The private queue's exact intent marker confirms the priority write. Current execution outcomes are separate."
			}
		}
	}
	out.Status = portfolioApplicationStatus(out.Results)
	return out
}

func (s *Server) ReadPortfolioApplication(ctx context.Context, applicationID string, access PortfolioGoalAccess) (PortfolioGoalApplication, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioGoalApplication{}, err
	}
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	saved, err := s.readPortfolioGoals(actor)
	if err != nil {
		return PortfolioGoalApplication{}, err
	}
	for _, application := range saved.Applications {
		if application.ID != applicationID {
			continue
		}
		goal := portfolioGoalByID(&saved, application.GoalID)
		if goal == nil {
			return PortfolioGoalApplication{}, ErrPortfolioGoalUnavailable
		}
		if err := authorizePortfolioGoal(ctx, *goal, access, false); err != nil {
			return PortfolioGoalApplication{}, err
		}
		if err := authorizePortfolioApplication(ctx, application, access); err != nil {
			return PortfolioGoalApplication{}, err
		}
		out := s.observePortfolioApplication(application)
		if err := authorizePortfolioGoal(ctx, *goal, access, false); err != nil {
			return PortfolioGoalApplication{}, err
		}
		if err := authorizePortfolioApplication(ctx, application, access); err != nil {
			return PortfolioGoalApplication{}, err
		}
		return out, ctx.Err()
	}
	return PortfolioGoalApplication{}, ErrPortfolioGoalConflict
}
