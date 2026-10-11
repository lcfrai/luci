package product

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const coordinationIntentLimit = 64
const coordinationFileLimit = 8 << 20

// Origins and events are private authenticated project state, never agent
// editable context. A Luci origin identifies its person, not a task worker/VM.
type CoordinationIntent struct {
	ID                      string       `json:"id"`
	TurnID                  string       `json:"turnId"`
	Actor                   ContextActor `json:"actor"`
	LuciID                  string       `json:"luciId"`
	BaseContextDigest       string       `json:"baseContextDigest"`
	ProposalDigest          string       `json:"proposalDigest"`
	Target                  string       `json:"target"`
	RequestedChange         string       `json:"requestedChange"`
	ChangedPaths            []string     `json:"changedPaths"`
	RequirementIDs          []string     `json:"requirementIds"`
	TaskIDs                 []string     `json:"taskIds"`
	AffectsAllTasks         bool         `json:"affectsAllTasks"`
	DependsOn               []string     `json:"dependsOn"`
	Status                  string       `json:"status"`
	CreatedAt               string       `json:"createdAt"`
	AppliedRevision         int          `json:"appliedRevision,omitempty"`
	ApplyBaseContextDigest  string       `json:"applyBaseContextDigest,omitempty"`
	CandidateProposalDigest string       `json:"candidateProposalDigest,omitempty"`
	AppliedTargetDigest     string       `json:"appliedTargetDigest,omitempty"`
	AcceptedContextDigest   string       `json:"acceptedContextDigest,omitempty"`
	SupersededBy            string       `json:"supersededBy,omitempty"`
	ResolutionReason        string       `json:"resolutionReason,omitempty"`
	KeyDigest               string       `json:"-"`
	RequestDigest           string       `json:"-"`
}

type CoordinationEvent struct {
	ID              string       `json:"id"`
	Sequence        int          `json:"sequence"`
	PreviousEventID string       `json:"previousEventId,omitempty"`
	Kind            string       `json:"kind"`
	IntentID        string       `json:"intentId"`
	Actor           ContextActor `json:"actor"`
	At              string       `json:"at"`
	Causes          []string     `json:"causes"`
	ContextDigest   string       `json:"contextDigest"`
}

type storedCoordinationIntent struct {
	CoordinationIntent
	Key     string `json:"keyDigest"`
	Request string `json:"requestDigest"`
}

type coordinationLedger struct {
	Version      int                        `json:"version"`
	ProjectID    string                     `json:"projectId"`
	AccountScope string                     `json:"accountScope"`
	ProjectScope string                     `json:"projectScope"`
	Revision     int                        `json:"revision"`
	Intents      []storedCoordinationIntent `json:"intents"`
	Events       []CoordinationEvent        `json:"events"`
}

func (s *Server) coordinationPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrOrchestratorInvalid
	}
	root, err := guardedProjectPath(s.stateDir, "coordination", true)
	if err != nil {
		return "", ErrOrchestratorInvalid
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if _, err = guardedProjectPath(s.stateDir, "coordination", false); err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return "", ErrOrchestratorInvalid
	}
	if create {
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(root)
		} else {
			err = os.Chmod(root, 0700)
		}
		if err != nil {
			return "", err
		}
	}
	return guardedProjectPath(s.stateDir, "coordination/"+id+".json", create)
}

func (s *Server) readCoordination(p project.Project) (coordinationLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	out := coordinationLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Intents: []storedCoordinationIntent{}, Events: []CoordinationEvent{}}
	if err != nil {
		return out, ErrOrchestratorInvalid
	}
	path, err := s.coordinationPath(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, ErrOrchestratorInvalid
	}
	var saved coordinationLedger
	if err = externalReadJSON(path, coordinationFileLimit, &saved); errors.Is(err, os.ErrNotExist) {
		return out, nil
	} else if err != nil {
		return out, ErrOrchestratorInvalid
	}
	if saved.Version != 1 || saved.ProjectID != out.ProjectID || saved.AccountScope != out.AccountScope || saved.ProjectScope != out.ProjectScope || saved.Revision < 0 || len(saved.Intents) > coordinationIntentLimit || len(saved.Events) > coordinationIntentLimit*3 {
		return out, ErrOrchestratorInvalid
	}
	seen, turns, keys := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, intent := range saved.Intents {
		if !strings.HasPrefix(intent.ID, "intent-") || !prototypeReleaseID.MatchString(intent.ID) || seen[intent.ID] || turns[intent.TurnID] || keys[intent.Key] || !hexDigest(intent.Key) || !hexDigest(intent.Request) || !hexDigest(intent.BaseContextDigest) || !hexDigest(intent.ProposalDigest) || !prototypeReleaseID.MatchString(intent.TurnID) || !strings.HasPrefix(intent.TurnID, "turn-") || !validConnectionToken(intent.Actor.ID, 128, false) || intent.Actor.DisplayName == "" || len(intent.Actor.DisplayName) > 512 || intent.LuciID != coordinationLuciID(saved.AccountScope, intent.Actor.ID) || len(intent.RequestedChange) > 16000 || len(intent.ChangedPaths) > 1024 || len(intent.DependsOn) > 16 || len(intent.TaskIDs) > 200 || len(intent.RequirementIDs) > 200 {
			return out, ErrOrchestratorInvalid
		}
		if intent.Target != "specification" && intent.Target != "design-plan" && intent.Target != "plan" {
			return out, ErrOrchestratorInvalid
		}
		if intent.Status != "pending" && intent.Status != "applied" && intent.Status != "superseded" {
			return out, ErrOrchestratorInvalid
		}
		if (intent.ApplyBaseContextDigest != "" && !hexDigest(intent.ApplyBaseContextDigest)) || (intent.CandidateProposalDigest != "" && !hexDigest(intent.CandidateProposalDigest)) || (intent.AcceptedContextDigest != "" && !hexDigest(intent.AcceptedContextDigest)) {
			return out, ErrOrchestratorInvalid
		}
		if intent.Status == "applied" && (intent.AppliedRevision < 1 || !hexDigest(intent.AppliedTargetDigest) || !hexDigest(intent.ApplyBaseContextDigest) || !hexDigest(intent.CandidateProposalDigest)) {
			return out, ErrOrchestratorInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, intent.CreatedAt); err != nil {
			return out, ErrOrchestratorInvalid
		}
		for _, dependency := range intent.DependsOn {
			if !seen[dependency] {
				return out, ErrOrchestratorInvalid
			}
		}
		seen[intent.ID], turns[intent.TurnID], keys[intent.Key] = true, true, true
	}
	for i, event := range saved.Events {
		previous := ""
		if i > 0 {
			previous = saved.Events[i-1].ID
		}
		if event.PreviousEventID != previous {
			return out, ErrOrchestratorInvalid
		}
		if event.Sequence != i+1 || !seen[event.IntentID] || !strings.HasPrefix(event.ID, "coord-event-") || !prototypeReleaseID.MatchString(event.ID) || !hexDigest(event.ContextDigest) || (event.Kind != "proposed" && event.Kind != "applied" && event.Kind != "superseded") || !validConnectionToken(event.Actor.ID, 128, false) || len(event.Causes) > 17 {
			return out, ErrOrchestratorInvalid
		}
	}
	if err = s.validateCoordinationBindings(p, saved); err != nil {
		return out, err
	}
	return saved, nil
}

// A valid-shaped row alone cannot satisfy a dependency or resolve opposition.
// Cross-check the causal state machine and the retained original/apply receipt.
func (s *Server) validateCoordinationBindings(p project.Project, saved coordinationLedger) error {
	turns, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	intents := map[string]storedCoordinationIntent{}
	for _, intent := range saved.Intents {
		intents[intent.ID] = intent
		turn := coordinationTurn(turns, intent.TurnID)
		if turn == nil || turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Proposal == nil || turn.Actor != intent.Actor || turn.ContextDigest != intent.BaseContextDigest || turn.Answer.Proposal.Target != intent.Target || reviewDigest(turn.Answer.Proposal) != intent.ProposalDigest || turn.Message != intent.RequestedChange || !reflect.DeepEqual(coordinationPaths(coordinationChanges(turn)), intent.ChangedPaths) {
			return ErrOrchestratorInvalid
		}
		requirements, tasks, all := coordinationImpact(*turn)
		if !reflect.DeepEqual(requirements, intent.RequirementIDs) || !reflect.DeepEqual(tasks, intent.TaskIDs) || all != intent.AffectsAllTasks {
			return ErrOrchestratorInvalid
		}
		if intent.Status == "applied" && (turn.AppliedRevision != intent.AppliedRevision || turn.ApplyTargetRevision != intent.AppliedRevision || turn.ApplyTargetDigest != intent.AppliedTargetDigest || turn.ApplyIntent != "") {
			return ErrOrchestratorInvalid
		}
		if intent.Status != "applied" && (intent.AppliedRevision != 0 || intent.AppliedTargetDigest != "" || intent.AcceptedContextDigest != "") {
			return ErrOrchestratorInvalid
		}
		if intent.Status == "superseded" && (intent.SupersededBy == "" || intent.SupersededBy == intent.ID || strings.TrimSpace(intent.ResolutionReason) == "" || len(intent.ResolutionReason) > 2000 || turn.AppliedRevision > 0 || turn.ApplyIntent != "") {
			return ErrOrchestratorInvalid
		}
		if intent.Status != "superseded" && (intent.SupersededBy != "" || intent.ResolutionReason != "") {
			return ErrOrchestratorInvalid
		}
	}
	states, events := map[string]string{}, map[string]bool{}
	for _, event := range saved.Events {
		intent := intents[event.IntentID]
		if events[event.ID] || event.Actor.DisplayName == "" || len(event.Actor.DisplayName) > 512 {
			return ErrOrchestratorInvalid
		}
		events[event.ID] = true
		if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
			return ErrOrchestratorInvalid
		}
		causes := map[string]bool{}
		for _, cause := range event.Causes {
			if causes[cause] || states[cause] == "" || cause == event.IntentID {
				return ErrOrchestratorInvalid
			}
			causes[cause] = true
		}
		switch event.Kind {
		case "proposed":
			if states[event.IntentID] != "" || event.Actor != intent.Actor {
				return ErrOrchestratorInvalid
			}
			states[event.IntentID] = "pending"
		case "applied":
			if states[event.IntentID] != "pending" || event.ContextDigest != intent.ApplyBaseContextDigest {
				return ErrOrchestratorInvalid
			}
			states[event.IntentID] = "applied"
		case "superseded":
			replacement, ok := intents[intent.SupersededBy]
			if states[event.IntentID] != "pending" || event.Actor.ID != intent.Actor.ID || !ok || replacement.Target != intent.Target || !causes[intent.SupersededBy] {
				return ErrOrchestratorInvalid
			}
			states[event.IntentID] = "superseded"
		}
	}
	for _, intent := range saved.Intents {
		if states[intent.ID] != intent.Status {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

func coordinationLuciID(account, actor string) string {
	return "luci-" + reviewDigest([]string{account, actor})
}

func (s *Server) saveCoordination(saved coordinationLedger) error {
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil || len(data)+1 > coordinationFileLimit || len(saved.Intents) > coordinationIntentLimit || len(saved.Events) > coordinationIntentLimit*3 {
		return ErrOrchestratorCapacity
	}
	path, err := s.coordinationPath(saved.ProjectID, true)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	if err = durableRunJSON(path, saved); err != nil {
		return ErrOrchestratorInvalid
	}
	return nil
}

func coordinationEvent(saved *coordinationLedger, intent CoordinationIntent, kind, digest string, actor ContextActor) {
	causes := append([]string{}, intent.DependsOn...)
	if intent.SupersededBy != "" {
		found := false
		for _, cause := range causes {
			found = found || cause == intent.SupersededBy
		}
		if !found {
			causes = append(causes, intent.SupersededBy)
		}
	}
	previous := ""
	if len(saved.Events) > 0 {
		previous = saved.Events[len(saved.Events)-1].ID
	}
	saved.Events = append(saved.Events, CoordinationEvent{ID: newID("coord-event-"), Sequence: len(saved.Events) + 1, PreviousEventID: previous, Kind: kind, IntentID: intent.ID, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), Causes: causes, ContextDigest: digest})
	saved.Revision++
}
