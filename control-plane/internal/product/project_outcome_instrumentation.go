package product

import (
	"net/http"
	"strconv"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These are private recorded facts, not usage estimates or acceptance evidence.
// Generic/legacy sessions without an original daemon fact remain unmeasured.
type ProjectRunRuntimeTiming struct {
	Method                 string `json:"method"`
	ChildID                string `json:"childId"`
	OriginalIdentityDigest string `json:"originalIdentityDigest"`
	StartedAt              string `json:"startedAt"`
	FinishedAt             string `json:"finishedAt"`
	ObservedAt             string `json:"observedAt"`
	ElapsedMilliseconds    *int64 `json:"elapsedMilliseconds"`
}
type runHumanAction struct {
	ActorID       string `json:"actorId"`
	TeamID        string `json:"teamId,omitempty"`
	Action        string `json:"action"`
	RequestDigest string `json:"requestDigest"`
}
type ProjectRunHumanAction struct {
	runHumanAction
	Revision int    `json:"revision"`
	At       string `json:"at"`
}
type ProjectRunMeasurements struct {
	Version             int                     `json:"version"`
	SinceRevision       int                     `json:"sinceRevision"`
	HumanActions        []ProjectRunHumanAction `json:"humanActions"`
	OmittedHumanActions int                     `json:"omittedHumanActions"`
}

func humanRunAction(r *http.Request, action string, revision int) []runHumanAction {
	user, _, managed := management.ActorFromContext(r.Context())
	if !managed || user.Disabled || user.ID == "" {
		return nil
	}
	teamID := ""
	if team, ok := management.TeamAccessFromContext(r.Context()); ok {
		if team.ProjectID != r.PathValue("id") || team.Member.UserID != user.ID || !team.Permissions.Read || !team.Permissions.Edit {
			return nil
		}
		teamID = team.Team.ID
	} else if management.TeamRuntimeIDFromContext(r.Context()) != "" {
		return nil
	}
	return []runHumanAction{{ActorID: user.ID, TeamID: teamID, Action: action, RequestDigest: reviewDigest([]string{r.Method, r.Pattern, r.PathValue("id"), r.PathValue("runId"), action, strconv.Itoa(revision), user.ID, teamID})}}
}
func validHumanRunAction(action runHumanAction) bool {
	if action.ActorID == "" || len(action.ActorID) > 128 || len(action.TeamID) > 128 || !hexDigest(action.RequestDigest) {
		return false
	}
	switch action.Action {
	case "start-coding", "cancel", "checks", "releases", "accept", "reconcile", "swarm-pause", "swarm-resume", "swarm-cancel":
		return true
	}
	return false
}
func appendHumanRunAction(run *ProjectRun, actions []runHumanAction, at string) error {
	if len(actions) > 1 {
		return ErrProjectRunInvalid
	}
	if run.Measurements == nil {
		run.Measurements = &ProjectRunMeasurements{Version: 1, SinceRevision: run.Revision + 1, HumanActions: []ProjectRunHumanAction{}}
	}
	if len(actions) == 0 {
		return nil
	}
	action := actions[0]
	if !validHumanRunAction(action) {
		return ErrProjectRunInvalid
	}
	for _, old := range run.Measurements.HumanActions {
		if old.RequestDigest == action.RequestDigest {
			return nil
		}
	}
	run.Measurements.HumanActions = append(run.Measurements.HumanActions, ProjectRunHumanAction{runHumanAction: action, Revision: run.Revision + 1, At: at})
	if len(run.Measurements.HumanActions) > 256 {
		run.Measurements.HumanActions = append([]ProjectRunHumanAction{}, run.Measurements.HumanActions[1:]...)
		if run.Measurements.OmittedHumanActions < 1e9 {
			run.Measurements.OmittedHumanActions++
		}
	}
	return nil
}
func validRunMeasurements(run ProjectRun) bool {
	if run.Measurements != nil {
		m := run.Measurements
		if m.Version != 1 || m.SinceRevision < 1 || m.SinceRevision > run.Revision || m.HumanActions == nil || len(m.HumanActions) > 256 || m.OmittedHumanActions < 0 || m.OmittedHumanActions > 1e9 {
			return false
		}
		seen := map[string]bool{}
		previous := 0
		for _, event := range m.HumanActions {
			if !validHumanRunAction(event.runHumanAction) || event.Revision < m.SinceRevision || event.Revision > run.Revision || event.Revision <= previous || seen[event.RequestDigest] {
				return false
			}
			if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
				return false
			}
			seen[event.RequestDigest] = true
			previous = event.Revision
		}
	}
	for _, operation := range run.Operations {
		timing := operation.RuntimeTiming
		if timing == nil {
			continue
		}
		if timing.Method != registry.TaskTerminalResultMethod || timing.ChildID != operation.ChildID || operation.Kind != "session" || operation.TaskID == "" || !hexDigest(timing.OriginalIdentityDigest) {
			return false
		}
		start, a := time.Parse(time.RFC3339Nano, timing.StartedAt)
		end, b := time.Parse(time.RFC3339Nano, timing.FinishedAt)
		if a != nil || b != nil || start.IsZero() || end.Before(start) {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, timing.ObservedAt); err != nil {
			return false
		}
		elapsed := end.Sub(start)
		if timing.ElapsedMilliseconds != nil && (elapsed < 0 || elapsed > 365*24*time.Hour || *timing.ElapsedMilliseconds != elapsed.Milliseconds()) {
			return false
		}
	}
	return true
}

// Called only after existing controller code rejoined the exact private registry
// child and original daemon terminal result. A retry cannot replace that fact.
// Measurement failure never launches, retries, accepts or changes worker limits.
func taskRuntimeTiming(observed registry.Session) (ProjectRunRuntimeTiming, error) {
	if observed.Status != registry.StatusExited || !observed.HasTaskContainerResult() {
		return ProjectRunRuntimeTiming{}, ErrProjectRunInvalid
	}
	fact := observed.TaskTerminalResult
	timing := ProjectRunRuntimeTiming{Method: fact.Method, ChildID: observed.ID, OriginalIdentityDigest: reviewDigest(fact), StartedAt: fact.StartedAt.UTC().Format(time.RFC3339Nano), FinishedAt: fact.FinishedAt.UTC().Format(time.RFC3339Nano), ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	elapsed := fact.FinishedAt.Sub(fact.StartedAt)
	if elapsed >= 0 && elapsed <= 365*24*time.Hour {
		milliseconds := elapsed.Milliseconds()
		timing.ElapsedMilliseconds = &milliseconds
	}
	return timing, nil
}

func (s *ProjectRunStore) RecordTaskRuntimeTiming(projectID, path, id string, revision int, observed registry.Session) (ProjectRun, error) {
	timing, err := taskRuntimeTiming(observed)
	if err != nil {
		return ProjectRun{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, path, id, revision, func(row *storedProjectRun) (bool, error) {
		operation := swarmOperation(&row.Run, observed.ProjectRunOperationID)
		if operation == nil || operation.TaskID == "" || operation.ChildID != observed.ID || !projectRunSessionBindingMatches(row.Run, *operation, observed) {
			return false, ErrProjectRunInvalid
		}
		if operation.RuntimeTiming != nil {
			old := *operation.RuntimeTiming
			next := timing
			old.ObservedAt = ""
			next.ObservedAt = ""
			if reviewDigest(old) != reviewDigest(next) {
				return false, ErrProjectRunConflict
			}
			return false, nil
		}
		operation.RuntimeTiming = &timing
		return true, nil
	})
}
