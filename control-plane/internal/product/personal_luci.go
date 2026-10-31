package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const personalTurnLimit = 64
const personalFileLimit = 4 << 20
const personalContextLimit = 96 << 10

var ErrPersonalChanged = errors.New("Your conversation or included context changed. Your message is still a draft; refresh before sending.")
var ErrPersonalUnavailable = errors.New("Your private conversation could not be verified. No request was repeated.")
var ErrPersonalCapacity = errors.New("Your private conversation reached its retained history limit. Use Continue conversation to archive this part and keep chatting here; no history was deleted.")

type PersonalPreferences struct {
	ComputeProjectID string   `json:"computeProjectId"`
	Harness          string   `json:"harness"`
	Model            string   `json:"model"`
	ConnectionID     string   `json:"connectionId"`
	ProjectIDs       []string `json:"projectIds"`
	GoalIDs          []string `json:"goalIds"`
}
type PersonalProjectContext struct {
	Project   PortfolioProject        `json:"project"`
	Attention []PortfolioIntervention `json:"attention"`
}
type PersonalContext struct {
	Version          int                      `json:"version"`
	Preferences      PersonalPreferences      `json:"preferences"`
	Goals            []PortfolioGoal          `json:"goals"`
	Projects         []PersonalProjectContext `json:"projects"`
	UnknownProjects  int                      `json:"unknownProjects"`
	OmittedAttention int                      `json:"omittedAttention"`
	Digest           string                   `json:"digest"`
}
type PersonalDiscussion struct {
	ProjectID string `json:"projectId"`
	Message   string `json:"message"`
}
type PersonalAnswer struct {
	Summary    string              `json:"summary"`
	Question   string              `json:"question"`
	Options    []string            `json:"options"`
	Discussion *PersonalDiscussion `json:"discussion"`
}
type PersonalHandoff struct {
	ID                   string `json:"id"`
	ProjectID            string `json:"projectId"`
	Message              string `json:"message"`
	ProjectRevision      int    `json:"projectRevision"`
	ProjectContextDigest string `json:"projectContextDigest"`
	ProjectRequestKey    string `json:"projectRequestKey"`
	RequestDigest        string `json:"requestDigest"`
	Status               string `json:"status"` // dispatching, forwarded, unknown; never automatic replay
	ProjectTurnID        string `json:"projectTurnId,omitempty"`
	ProjectJobID         string `json:"projectJobId,omitempty"`
}
type PersonalTurn struct {
	ID            string           `json:"id"`
	Sequence      int              `json:"sequence"`
	Message       string           `json:"message"`
	Context       PersonalContext  `json:"context"`
	JobID         string           `json:"jobId"`
	Status        string           `json:"status"`
	Answer        *PersonalAnswer  `json:"answer,omitempty"`
	Handoff       *PersonalHandoff `json:"handoff,omitempty"`
	CreatedAt     string           `json:"createdAt"`
	KeyDigest     string           `json:"keyDigest"`
	RequestDigest string           `json:"requestDigest"`
	Withheld      bool             `json:"withheld,omitempty"`
}
type PersonalConversation struct {
	Version        int                 `json:"version"`
	Scope          string              `json:"scope"`
	Revision       int                 `json:"revision"`
	Preferences    PersonalPreferences `json:"preferences"`
	Turns          []PersonalTurn      `json:"turns"`
	ConversationID string              `json:"conversationId"`
	Archives       []PersonalArchive   `json:"archives,omitempty"`
}
type PersonalView struct {
	ConversationID string                  `json:"conversationId"`
	Archived       bool                    `json:"archived"`
	Archives       []PersonalArchiveChoice `json:"archives"`
	Revision       int                     `json:"revision"`
	Preferences    PersonalPreferences     `json:"preferences"`
	Context        PersonalContext         `json:"context"`
	Turns          []PersonalTurn          `json:"turns"`
	HasEarlier     bool                    `json:"hasEarlier"`
	BeforeSequence int                     `json:"beforeSequence"`
}
type PersonalTurnRequest struct {
	ExpectedRevision      int    `json:"expectedRevision"`
	ExpectedContextDigest string `json:"expectedContextDigest"`
	IdempotencyKey        string `json:"idempotencyKey"`
	Message               string `json:"message"`
}

// The path binds the actual private worker account, current authenticated actor
// and workspace/team. No browser-supplied actor or synthetic project is used.
func (s *Server) PersonalScope(ctx context.Context) (string, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return "", err
	}
	team := PersonalWorkspaceTeamID(ctx)
	if runtime := management.TeamRuntimeIDFromContext(ctx); runtime != "" {
		if access, ok := management.TeamAccessFromContext(ctx); ok && access.Team.ID != "" && access.Team.ID != runtime {
			return "", management.ErrExecutionAuthority
		}
	}
	return reviewDigest([]string{s.projectRuns.accountScope, actor, team}), ctx.Err()
}
func PersonalWorkspaceTeamID(ctx context.Context) string {
	if team := management.TeamRuntimeIDFromContext(ctx); team != "" {
		return team
	}
	if access, ok := management.TeamAccessFromContext(ctx); ok {
		return access.Team.ID
	}
	return ""
}
func (s *Server) personalPath(scope string, create bool) (string, error) {
	if !hexDigest(scope) {
		return "", ErrPersonalUnavailable
	}
	dir, err := guardedProjectPath(s.stateDir, "personal-luci", create)
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
	path, err := guardedProjectPath(s.stateDir, "personal-luci/"+scope+".json", create)
	if err == nil {
		if e := hostpath.OneName(path); e != nil && !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
	}
	return path, err
}
func emptyPersonal(scope string) PersonalConversation {
	return PersonalConversation{Version: 1, Scope: scope, ConversationID: "personal-chat-" + scope, Preferences: PersonalPreferences{ProjectIDs: []string{}, GoalIDs: []string{}}, Turns: []PersonalTurn{}}
}
func validPersonalPreferences(p PersonalPreferences) bool {
	if !prototypeReleaseID.MatchString(p.ComputeProjectID) || p.Harness != "codex" && p.Harness != "claude-code" && p.Harness != "opencode" || strings.TrimSpace(p.Model) == "" || len(p.Model) > 512 || !validArtifactStoredText(p.Model) || !validConnectionToken(p.ConnectionID, 200, false) || p.ProjectIDs == nil || len(p.ProjectIDs) > 15 || p.GoalIDs == nil || len(p.GoalIDs) > 16 {
		return false
	}
	for _, ids := range [][]string{p.ProjectIDs, p.GoalIDs} {
		seen := map[string]bool{}
		for _, id := range ids {
			if !prototypeReleaseID.MatchString(id) || seen[id] {
				return false
			}
			seen[id] = true
		}
	}
	return true
}
func (s *Server) readPersonal(scope string) (PersonalConversation, error) {
	empty := emptyPersonal(scope)
	path, err := s.personalPath(scope, false)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, ErrPersonalUnavailable
	}
	var row PersonalConversation
	if err := externalReadJSON(path, personalFileLimit, &row); errors.Is(err, os.ErrNotExist) {
		return empty, nil
	} else if err != nil {
		return empty, ErrPersonalUnavailable
	}
	return validatePersonalConversation(row, scope)
}
func validatePersonalConversation(row PersonalConversation, scope string) (PersonalConversation, error) {
	empty := emptyPersonal(scope)
	if row.ConversationID == "" {
		row.ConversationID = empty.ConversationID
	}
	if !prototypeReleaseID.MatchString(row.ConversationID) || !validPersonalArchives(row.Archives) || row.Version != 1 || row.Scope != scope || row.Revision < 0 || row.Turns == nil || len(row.Turns) > personalTurnLimit || row.Revision > 0 && !validPersonalPreferences(row.Preferences) {
		return empty, ErrPersonalUnavailable
	}
	seen, keys := map[string]bool{}, map[string]bool{}
	for i, turn := range row.Turns {
		if !prototypeReleaseID.MatchString(turn.ID) || turn.Sequence != i+1 || seen[turn.ID] || !hexDigest(turn.KeyDigest) || !hexDigest(turn.RequestDigest) || keys[turn.KeyDigest] || turn.Message == "" || len(turn.Message) > 16000 || !validArtifactStoredText(turn.Message) || !prototypeReleaseID.MatchString(turn.JobID) || !validStoredPersonalContext(turn.Context) || !personalTurnStatus(turn.Status) || turn.Withheld {
			return empty, ErrPersonalUnavailable
		}
		if _, err := time.Parse(time.RFC3339Nano, turn.CreatedAt); err != nil {
			return empty, ErrPersonalUnavailable
		}
		if turn.Answer != nil && (turn.Status != "succeeded" || ValidatePersonalAnswer(turn.Context, *turn.Answer) != nil) {
			return empty, ErrPersonalUnavailable
		}
		if turn.Status == "succeeded" && turn.Answer == nil {
			return empty, ErrPersonalUnavailable
		}
		if turn.Handoff != nil && (!prototypeReleaseID.MatchString(turn.Handoff.ID) || !prototypeReleaseID.MatchString(turn.Handoff.ProjectID) || !hexDigest(turn.Handoff.RequestDigest) || !hexDigest(turn.Handoff.ProjectContextDigest) || turn.Handoff.ProjectRevision < 0 || turn.Handoff.Message == "" || len(turn.Handoff.Message) > 16000 || !validArtifactStoredText(turn.Handoff.Message) || !validRunKey(turn.Handoff.ProjectRequestKey) || turn.Handoff.Status != "dispatching" && turn.Handoff.Status != "forwarded" && turn.Handoff.Status != "unknown") {
			return empty, ErrPersonalUnavailable
		}
		if turn.Handoff != nil {
			included := false
			for _, id := range turn.Context.Preferences.ProjectIDs {
				included = included || id == turn.Handoff.ProjectID
			}
			if !included {
				return empty, ErrPersonalUnavailable
			}
			if turn.Handoff.Status == "forwarded" && (!prototypeReleaseID.MatchString(turn.Handoff.ProjectTurnID) || !prototypeReleaseID.MatchString(turn.Handoff.ProjectJobID)) {
				return empty, ErrPersonalUnavailable
			}
		}
		seen[turn.ID], keys[turn.KeyDigest] = true, true
	}
	for _, archive := range row.Archives {
		if archive.ID == row.ConversationID {
			return empty, ErrPersonalUnavailable
		}
		for _, key := range archive.Keys {
			if seen[key.TurnID] || keys[key.KeyDigest] {
				return empty, ErrPersonalUnavailable
			}
		}
	}
	return row, nil
}
func personalTurnStatus(status string) bool {
	return status == "reserved" || status == "succeeded" || status == "failed" || status == "cancelled"
}
func personalContextDigest(c PersonalContext) string { c.Digest = ""; return reviewDigest(c) }
func validStoredPersonalContext(c PersonalContext) bool {
	if c.Version != 1 || !validPersonalPreferences(c.Preferences) || c.Digest != personalContextDigest(c) || c.Goals == nil || len(c.Goals) > 16 || c.Projects == nil || len(c.Projects) != len(c.Preferences.ProjectIDs) || c.UnknownProjects < 0 || c.UnknownProjects > len(c.Projects) || c.OmittedAttention < 0 || len(personalProjects(c)) > 16 {
		return false
	}
	selected := map[string]bool{}
	for _, id := range c.Preferences.ProjectIDs {
		selected[id] = true
	}
	seen := map[string]bool{}
	for _, p := range c.Projects {
		if !selected[p.Project.ID] || seen[p.Project.ID] || len(p.Project.Name) > 200 || p.Attention == nil || len(p.Attention) > 8 {
			return false
		}
		seen[p.Project.ID] = true
	}
	seen = map[string]bool{}
	for _, goal := range c.Goals {
		if validatePortfolioGoal(goal) != nil || goal.ID == "" || goal.Revision < 1 || seen[goal.ID] {
			return false
		}
		seen[goal.ID] = true
		for _, p := range goal.Projects {
			if !selected[p.ProjectID] {
				return false
			}
		}
	}
	if len(c.Goals) != len(c.Preferences.GoalIDs) {
		return false
	}
	for _, id := range c.Preferences.GoalIDs {
		if !seen[id] {
			return false
		}
	}
	raw, err := json.Marshal(c)
	return err == nil && len(raw) <= personalContextLimit
}
func (s *Server) savePersonal(row *PersonalConversation) error {
	path, err := s.personalPath(row.Scope, true)
	if err != nil {
		return ErrPersonalUnavailable
	}
	row.Revision++
	data, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(data)+1 > personalFileLimit || len(row.Turns) > personalTurnLimit {
		return ErrPersonalCapacity
	}
	return atomicJSON(path, row)
}
func personalProjects(c PersonalContext) []string {
	ids := []string{c.Preferences.ComputeProjectID}
	seen := map[string]bool{c.Preferences.ComputeProjectID: true}
	for _, id := range c.Preferences.ProjectIDs {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, goal := range c.Goals {
		for _, p := range goal.Projects {
			if !seen[p.ProjectID] {
				ids = append(ids, p.ProjectID)
				seen[p.ProjectID] = true
			}
		}
	}
	return ids
}
func PersonalContextProjectIDs(c PersonalContext) []string { return personalProjects(c) }
func PersonalContextAccessible(ctx context.Context, c PersonalContext, access PortfolioGoalAccess) error {
	if access == nil {
		return management.ErrExecutionAuthority
	}
	for _, id := range personalProjects(c) {
		if err := access(ctx, id, false); err != nil {
			return err
		}
	}
	return ctx.Err()
}
func (s *Server) PersonalContext(ctx context.Context, preferences PersonalPreferences, access PortfolioGoalAccess) (PersonalContext, error) {
	result := PersonalContext{Version: 1, Preferences: preferences, Goals: []PortfolioGoal{}, Projects: []PersonalProjectContext{}}
	if !validPersonalPreferences(preferences) {
		return result, ErrPortfolioGoalInvalid
	}
	if len(personalProjects(result)) > 16 {
		return result, ErrPortfolioGoalInvalid
	}
	if err := PersonalContextAccessible(ctx, result, access); err != nil {
		return result, err
	}
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return result, err
	}
	if len(preferences.GoalIDs) > 0 {
		view, err := s.ReadPortfolioGoals(ctx, access)
		if err != nil {
			return result, err
		}
		included := map[string]bool{}
		for _, id := range preferences.ProjectIDs {
			included[id] = true
		}
		for _, id := range preferences.GoalIDs {
			found := false
			for _, goal := range view.Goals {
				if goal.ID == id {
					for _, p := range goal.Projects {
						if !included[p.ProjectID] {
							return result, ErrPortfolioGoalInvalid
						}
					}
					result.Goals = append(result.Goals, goal)
					found = true
					break
				}
			}
			if !found {
				return result, management.ErrExecutionAuthority
			}
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for _, id := range preferences.ProjectIDs {
		if err := access(bounded, id, false); err != nil {
			return result, err
		}
		p, err := s.resolveProject(id)
		if err != nil {
			return result, err
		}
		project, attention, err := s.ReadPortfolioProject(bounded, p, actor)
		if err != nil {
			result.UnknownProjects++
			project = PortfolioProject{ID: id, Name: p.Name, Status: "unavailable", Coordinator: PortfolioCoordinator{Status: "unavailable"}}
			attention = []PortfolioIntervention{}
		}
		if len(attention) > 8 {
			result.OmittedAttention += len(attention) - 8
			attention = attention[:8]
		}
		result.Projects = append(result.Projects, PersonalProjectContext{Project: project, Attention: attention})
		if err := access(bounded, id, false); err != nil {
			return result, err
		}
	}
	if err := PersonalContextAccessible(ctx, result, access); err != nil {
		return result, err
	}
	result.Digest = personalContextDigest(result)
	data, err := json.Marshal(result)
	if err != nil || len(data) > personalContextLimit {
		return result, ErrPersonalCapacity
	}
	return result, ctx.Err()
}
func (s *Server) PersonalView(ctx context.Context, before int, access PortfolioGoalAccess) (PersonalView, error) {
	return s.PersonalArchiveView(ctx, before, "", access)
}
func (s *Server) PersonalArchiveView(ctx context.Context, before int, archiveID string, access PortfolioGoalAccess) (PersonalView, error) {
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalView{}, err
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	s.personalMu.Unlock()
	if err != nil {
		return PersonalView{}, err
	}
	choices := personalArchiveChoices(row)
	revision := row.Revision
	if archiveID != "" {
		row, err = s.readPersonalArchiveChoice(row, archiveID)
		if err != nil {
			return PersonalView{}, err
		}
	}
	view := PersonalView{Revision: revision, ConversationID: row.ConversationID, Archived: archiveID != "", Archives: choices, Preferences: row.Preferences, Turns: []PersonalTurn{}}
	if row.Preferences.ComputeProjectID != "" {
		view.Context, err = s.PersonalContext(ctx, row.Preferences, access)
		if errors.Is(err, management.ErrExecutionAuthority) {
			view.Preferences = emptyPersonal(scope).Preferences
			view.Context = PersonalContext{}
		} else if err != nil {
			return PersonalView{}, err
		}
	}
	end := len(row.Turns)
	if before > 0 {
		for i, turn := range row.Turns {
			if turn.Sequence >= before {
				end = i
				break
			}
		}
	}
	start := end - 20
	if start < 0 {
		start = 0
	}
	if before < 0 {
		start = 0
	} // Owner-only export, never a browser page cursor.
	for _, turn := range row.Turns[start:end] {
		if err := PersonalContextAccessible(ctx, turn.Context, access); err != nil {
			turn = PersonalTurn{ID: turn.ID, Sequence: turn.Sequence, Status: turn.Status, CreatedAt: turn.CreatedAt, Withheld: true}
		} else if turn.Handoff != nil && access(ctx, turn.Handoff.ProjectID, false) != nil {
			turn = PersonalTurn{ID: turn.ID, Sequence: turn.Sequence, Status: turn.Status, CreatedAt: turn.CreatedAt, Withheld: true}
		}
		view.Turns = append(view.Turns, turn)
	}
	view.HasEarlier = start > 0
	if len(view.Turns) > 0 {
		view.BeforeSequence = view.Turns[0].Sequence
	}
	// Repeat every displayed source grant after reads; revoke during I/O withholds
	// the entire affected turn, never an unsafe partial redaction of model prose.
	for i, turn := range view.Turns {
		if !turn.Withheld && (PersonalContextAccessible(ctx, turn.Context, access) != nil || turn.Handoff != nil && access(ctx, turn.Handoff.ProjectID, false) != nil) {
			view.Turns[i] = PersonalTurn{ID: turn.ID, Sequence: turn.Sequence, Status: turn.Status, CreatedAt: turn.CreatedAt, Withheld: true}
		}
	}
	if view.Context.Preferences.ComputeProjectID != "" && PersonalContextAccessible(ctx, view.Context, access) != nil {
		view.Preferences = emptyPersonal(scope).Preferences
		view.Context = PersonalContext{}
	}
	return view, ctx.Err()
}
func (s *Server) SavePersonalPreferences(ctx context.Context, revision int, preferences PersonalPreferences, access PortfolioGoalAccess) (PersonalView, error) {
	if _, err := s.PersonalContext(ctx, preferences, access); err != nil {
		return PersonalView{}, err
	}
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalView{}, err
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	if err == nil && row.Revision != revision {
		err = ErrPersonalChanged
	}
	if err == nil {
		for _, turn := range row.Turns {
			if turn.Status == "reserved" {
				err = ErrOrchestratorBusy
			}
		}
	}
	if err == nil {
		row.Preferences = preferences
		err = s.savePersonal(&row)
	}
	s.personalMu.Unlock()
	if err != nil {
		return PersonalView{}, err
	}
	return s.PersonalView(ctx, 0, access)
}
func (s *Server) ReservePersonalTurn(ctx context.Context, req PersonalTurnRequest, jobID string, access PortfolioGoalAccess) (PersonalTurn, bool, error) {
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedContextDigest) || !prototypeReleaseID.MatchString(jobID) || req.Message == "" || strings.TrimSpace(req.Message) == "" || len(req.Message) > 16000 || !validArtifactStoredText(req.Message) {
		return PersonalTurn{}, false, ErrPortfolioGoalInvalid
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	s.personalMu.Unlock()
	if err != nil {
		return PersonalTurn{}, false, err
	}
	key, digest := reviewDigest(req.IdempotencyKey), reviewDigest(req)
	if archived, err := s.personalArchivedByKey(row, key); err != nil {
		return PersonalTurn{}, false, err
	} else if archived != nil {
		if archived.RequestDigest != digest {
			return PersonalTurn{}, false, ErrPersonalChanged
		}
		if err := PersonalContextAccessible(ctx, archived.Context, access); err != nil {
			return PersonalTurn{}, false, err
		}
		return *archived, true, nil
	}
	for _, turn := range row.Turns {
		if turn.KeyDigest == key {
			if turn.RequestDigest != digest {
				return PersonalTurn{}, false, ErrPersonalChanged
			}
			if err := PersonalContextAccessible(ctx, turn.Context, access); err != nil {
				return PersonalTurn{}, false, err
			}
			return turn, true, nil
		}
	}
	if row.Revision != req.ExpectedRevision {
		return PersonalTurn{}, false, ErrPersonalChanged
	}
	for _, turn := range row.Turns {
		if turn.Status == "reserved" {
			return PersonalTurn{}, false, ErrOrchestratorBusy
		}
	}
	snapshot, err := s.PersonalContext(ctx, row.Preferences, access)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	if snapshot.Digest != req.ExpectedContextDigest {
		return PersonalTurn{}, false, ErrPersonalChanged
	}
	s.personalMu.Lock()
	defer s.personalMu.Unlock()
	latest, err := s.readPersonal(scope)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	if latest.Revision != row.Revision {
		return PersonalTurn{}, false, ErrPersonalChanged
	}
	if len(latest.Turns) >= personalTurnLimit {
		return PersonalTurn{}, false, ErrPersonalCapacity
	}
	turn := PersonalTurn{ID: newID("personal-turn-"), Sequence: len(latest.Turns) + 1, Message: req.Message, Context: snapshot, JobID: jobID, Status: "reserved", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), KeyDigest: key, RequestDigest: digest}
	latest.Turns = append(latest.Turns, turn)
	if err := s.savePersonal(&latest); err != nil {
		return PersonalTurn{}, false, err
	}
	return turn, false, nil
}
func ValidatePersonalAnswer(snapshot PersonalContext, answer PersonalAnswer) error {
	if strings.TrimSpace(answer.Summary) == "" || len(answer.Summary) > 12000 || !validArtifactStoredText(answer.Summary) || len(answer.Question) > 2000 || !validArtifactStoredText(answer.Question) || answer.Options == nil || len(answer.Options) > 6 {
		return ErrPersonalUnavailable
	}
	for _, option := range answer.Options {
		if strings.TrimSpace(option) == "" || len(option) > 500 || !validArtifactStoredText(option) {
			return ErrPersonalUnavailable
		}
	}
	if answer.Discussion != nil {
		found := false
		for _, id := range snapshot.Preferences.ProjectIDs {
			found = found || id == answer.Discussion.ProjectID
		}
		if !found || strings.TrimSpace(answer.Discussion.Message) == "" || len(answer.Discussion.Message) > 16000 || !validArtifactStoredText(answer.Discussion.Message) {
			return ErrPersonalUnavailable
		}
	}
	return nil
}
func (s *Server) CompletePersonalTurn(scope, turnID, jobID, status string, answer *PersonalAnswer) error {
	if !personalTurnStatus(status) || status == "reserved" {
		return ErrPersonalUnavailable
	}
	s.personalMu.Lock()
	defer s.personalMu.Unlock()
	row, err := s.readPersonal(scope)
	if err != nil {
		return err
	}
	if archived, err := s.personalArchivedByID(row, turnID); err != nil {
		return err
	} else if archived != nil {
		if archived.JobID == jobID && archived.Status == status && reviewDigest(archived.Answer) == reviewDigest(answer) {
			return nil
		}
		return ErrPersonalChanged
	}
	for i := range row.Turns {
		turn := &row.Turns[i]
		if turn.ID != turnID {
			continue
		}
		if turn.JobID != jobID {
			return ErrPersonalChanged
		}
		if turn.Status != "reserved" {
			if turn.Status == status && reviewDigest(turn.Answer) == reviewDigest(answer) {
				return nil
			}
			return ErrPersonalChanged
		}
		if status == "succeeded" {
			if answer == nil || ValidatePersonalAnswer(turn.Context, *answer) != nil {
				return ErrPersonalUnavailable
			}
		} else if answer != nil {
			return ErrPersonalUnavailable
		}
		turn.Status, turn.Answer = status, answer
		return s.savePersonal(&row)
	}
	return ErrPersonalUnavailable
}

// Internal durable correlation only. It grants no user read or execution
// authority; the controller must still resolve the original actor/current
// source grants before observing or allocating the corresponding utility.
func (s *Server) PersonalRecordedTurn(scope, turnID, jobID string) (PersonalTurn, error) {
	s.personalMu.Lock()
	defer s.personalMu.Unlock()
	row, err := s.readPersonal(scope)
	if err != nil {
		return PersonalTurn{}, err
	}
	for _, turn := range row.Turns {
		if turn.ID == turnID && turn.JobID == jobID {
			return turn, nil
		}
	}
	if turn, err := s.personalArchivedByID(row, turnID); err != nil {
		return PersonalTurn{}, err
	} else if turn != nil && turn.JobID == jobID {
		return *turn, nil
	}
	return PersonalTurn{}, ErrPersonalUnavailable
}
func (s *Server) PersonalTurn(ctx context.Context, id string, access PortfolioGoalAccess) (PersonalTurn, error) {
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalTurn{}, err
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	s.personalMu.Unlock()
	if err != nil {
		return PersonalTurn{}, err
	}
	if turn, err := s.personalArchivedByID(row, id); err != nil {
		return PersonalTurn{}, err
	} else if turn != nil {
		if err := PersonalContextAccessible(ctx, turn.Context, access); err != nil {
			return PersonalTurn{}, err
		}
		if turn.Handoff != nil && access(ctx, turn.Handoff.ProjectID, false) != nil {
			return PersonalTurn{}, management.ErrExecutionAuthority
		}
		return *turn, ctx.Err()
	}
	for _, turn := range row.Turns {
		if turn.ID == id {
			if err := PersonalContextAccessible(ctx, turn.Context, access); err != nil {
				return PersonalTurn{}, err
			}
			if turn.Handoff != nil && access(ctx, turn.Handoff.ProjectID, false) != nil {
				return PersonalTurn{}, management.ErrExecutionAuthority
			}
			return turn, ctx.Err()
		}
	}
	return PersonalTurn{}, ErrPersonalUnavailable
}

func (s *Server) ResolvePersonalRequest(ctx context.Context, req PersonalTurnRequest, access PortfolioGoalAccess) (*PersonalTurn, error) {
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return nil, err
	}
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedContextDigest) || len(req.Message) > 16000 {
		return nil, ErrPortfolioGoalInvalid
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	s.personalMu.Unlock()
	if err != nil {
		return nil, err
	}
	key, digest := reviewDigest(req.IdempotencyKey), reviewDigest(req)

	for _, turn := range row.Turns {
		if turn.KeyDigest == key {
			if turn.RequestDigest != digest {
				return nil, ErrPersonalChanged
			}
			if err := PersonalContextAccessible(ctx, turn.Context, access); err != nil {
				return nil, err
			}
			return &turn, ctx.Err()
		}
	}
	if archived, err := s.personalArchivedByKey(row, key); err != nil {
		return nil, err
	} else if archived != nil {
		if archived.RequestDigest != digest {
			return nil, ErrPersonalChanged
		}
		if err := PersonalContextAccessible(ctx, archived.Context, access); err != nil {
			return nil, err
		}
		return archived, ctx.Err()
	}
	return nil, ctx.Err() // Complete retained ledger; no request/job is created.
}
