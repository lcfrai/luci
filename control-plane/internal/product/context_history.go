package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const contextHistoryLimit = 2000
const contextHistoryFileLimit = 2 << 20

// ContextActor is a snapshot of the authenticated person's display identity at
// the time of a change. It is never accepted from project JSON or HTTP headers.
type ContextActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// ContextChangeEvent attributes an exact saved revision, not every future byte
// carrying the same revision number. Keep this outside exported agent context.
type ContextChangeEvent struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"projectId"`
	Kind      string       `json:"kind"`
	Action    string       `json:"action"`
	Revision  int          `json:"revision"`
	Digest    string       `json:"digest"`
	At        string       `json:"at"`
	Actor     ContextActor `json:"actor"`
	Status    string       `json:"status"`
}

type contextHistoryStore struct {
	Version      int                  `json:"version"`
	AccountScope string               `json:"accountScope"`
	ProjectScope string               `json:"projectScope"`
	ProjectID    string               `json:"projectId"`
	Pruned       bool                 `json:"pruned"`
	Events       []ContextChangeEvent `json:"events"`
}

type contextMutation struct{ kind, action string }

func authenticatedContextActor(ctx context.Context) (ContextActor, bool) {
	u, _, ok := management.ActorFromContext(ctx)
	if !ok || u.Disabled || !validConnectionToken(u.ID, 128, false) {
		return ContextActor{}, false
	}
	name := strings.TrimSpace(u.DisplayName)
	if name == "" {
		name = strings.TrimSpace(u.Username)
	}
	if name == "" {
		name = u.ID
	}
	if len(name) > 512 || strings.ContainsAny(name, "\x00\r\n") {
		return ContextActor{}, false
	}
	return ContextActor{ID: u.ID, DisplayName: name}, true
}

func (s *Server) contextHistoryPath(projectID string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return "", errors.New("invalid context history project")
	}
	root, err := guardedProjectPath(s.stateDir, "context-history", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	root, err = guardedProjectPath(s.stateDir, "context-history", false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("private context history directory is unavailable")
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
	return guardedProjectPath(s.stateDir, "context-history/"+projectID+".json", create)
}

// Caller holds s.mu. Missing history is readable; corrupt or copied history
// cannot be silently replaced with a new authoritative ledger.
func (s *Server) readContextHistory(dir, projectID string) (contextHistoryStore, error) {
	scope, err := s.approvalScopes(dir, projectID)
	expected := contextHistoryStore{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: projectID, Events: []ContextChangeEvent{}}
	if err != nil {
		return expected, err
	}
	path, err := s.contextHistoryPath(projectID, false)
	if errors.Is(err, os.ErrNotExist) {
		return expected, nil
	}
	if err != nil {
		return expected, err
	}
	if err = hostpath.OneName(path); errors.Is(err, os.ErrNotExist) {
		return expected, nil
	} else if err != nil {
		return expected, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return expected, nil
	}
	if err != nil {
		return expected, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > contextHistoryFileLimit {
		return expected, errors.New("private context history is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(f, contextHistoryFileLimit+1))
	if err != nil || len(data) > contextHistoryFileLimit {
		return expected, errors.New("private context history exceeds its bound")
	}
	var saved contextHistoryStore
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&saved) != nil || d.Decode(new(any)) != io.EOF || saved.Version != 1 || saved.AccountScope != expected.AccountScope || saved.ProjectScope != expected.ProjectScope || saved.ProjectID != projectID || len(saved.Events) > contextHistoryLimit {
		return expected, errors.New("private context history belongs to a different project or workspace")
	}
	seen := map[string]bool{}
	for _, event := range saved.Events {
		if !validContextChangeEvent(event, projectID) || seen[event.ID] {
			return expected, errors.New("private context history contains an invalid event")
		}
		seen[event.ID] = true
	}
	if saved.Events == nil {
		saved.Events = []ContextChangeEvent{}
	}
	return saved, nil
}

func validContextChangeEvent(event ContextChangeEvent, projectID string) bool {
	if event.ProjectID != projectID || !strings.HasPrefix(event.ID, "context-") || len(event.ID) != 32 || !prototypeReleaseID.MatchString(event.ID) || event.Revision < 1 || !hexDigest(event.Digest) || !validConnectionToken(event.Actor.ID, 128, false) || event.Actor.DisplayName == "" || len(event.Actor.DisplayName) > 512 || strings.ContainsAny(event.Actor.DisplayName, "\x00\r\n") || (event.Status != "applied" && event.Status != "unconfirmed") {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return false
	}
	if event.Action != "save" && event.Action != "approve" && event.Action != "apply-analysis" {
		return false
	}
	switch event.Kind {
	case "brief", "specification", "sketch", "design", "plan":
		return true
	default:
		return false
	}
}

func (s *Server) saveContextHistory(saved contextHistoryStore) error {
	path, err := s.contextHistoryPath(saved.ProjectID, true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := hostpath.OneName(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil || len(data)+1 > contextHistoryFileLimit {
		return errors.New("private context history exceeds its bound")
	}
	return atomicJSON(path, saved)
}

func contextRevisionSnapshot(state journeyState, kind string) (int, any, error) {
	switch kind {
	case "brief":
		return state.Brief.Revision, state.Brief, nil
	case "sketch":
		return state.Design.Revision, state.Design, nil
	case "specification":
		if state.Specification != nil {
			return state.Specification.Revision, *state.Specification, nil
		}
	case "design":
		if state.DesignPlan != nil {
			return state.DesignPlan.Revision, *state.DesignPlan, nil
		}
	case "plan":
		if state.Plan != nil {
			return state.Plan.Revision, *state.Plan, nil
		}
	}
	return 0, nil, errors.New("context change has no saved revision")
}

// Caller holds s.mu across revision validation and this write. A private intent
// precedes the project write; failure cannot fabricate an applied event. Legacy
// internal callers without a trusted actor preserve their previous save
// behavior, but acquire no attribution from client headers or exported claims.
func (s *Server) writeAttributedJourney(ctx context.Context, dir string, state journeyState, mutations ...contextMutation) error {
	actor, ok := authenticatedContextActor(ctx)
	if !ok {
		if _, _, present := management.ActorFromContext(ctx); present {
			return errors.New("The authenticated editor identity could not be verified. No context change was saved.")
		}
		return writeJourney(dir, state)
	}
	if len(mutations) == 0 || len(mutations) > 5 {
		return errors.New("The context change needs a bounded revision attribution.")
	}
	saved, err := s.readContextHistory(dir, state.Brief.ProjectID)
	if err != nil {
		return errors.New("The private context history could not be read. No context change was saved.")
	}
	newEvents := make([]ContextChangeEvent, 0, len(mutations))
	for _, mutation := range mutations {
		revision, value, err := contextRevisionSnapshot(state, mutation.kind)
		if err != nil {
			return err
		}
		event := ContextChangeEvent{ID: newID("context-"), ProjectID: state.Brief.ProjectID, Kind: mutation.kind, Action: mutation.action, Revision: revision, Digest: reviewDigest(value), At: time.Now().UTC().Format(time.RFC3339Nano), Actor: actor, Status: "unconfirmed"}
		if !validContextChangeEvent(event, state.Brief.ProjectID) {
			return errors.New("The authenticated context change could not be recorded.")
		}
		newEvents = append(newEvents, event)
	}
	saved.Events = append(newEvents, saved.Events...)
	if len(saved.Events) > contextHistoryLimit {
		saved.Events = saved.Events[:contextHistoryLimit]
		saved.Pruned = true
	}
	// Keep recent authorship usable even when unusually long display names hit
	// the byte bound before the count bound. Only older receipts are discarded.
	encoded, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return errors.New("The context change receipt could not be encoded.")
	}
	if len(encoded)+1 > contextHistoryFileLimit {
		saved.Pruned = true
		all := saved.Events
		low, high := len(newEvents), len(all)
		for low < high {
			mid := (low + high + 1) / 2
			saved.Events = all[:mid]
			encoded, err = json.MarshalIndent(saved, "", "  ")
			if err != nil {
				return errors.New("The context change receipt could not be encoded.")
			}
			if len(encoded)+1 <= contextHistoryFileLimit {
				low = mid
			} else {
				high = mid - 1
			}
		}
		saved.Events = all[:low]
	}
	if err = s.saveContextHistory(saved); err != nil {
		return errors.New("The private context change receipt could not be saved. No context change was saved.")
	}
	if err = writeJourney(dir, state); err != nil {
		return err
	}
	for i := range newEvents {
		saved.Events[i].Status = "applied"
	}
	if err = s.saveContextHistory(saved); err != nil {
		return errors.New("Context was saved, but its authorship receipt remains unconfirmed. Reload the saved revision before retrying.")
	}
	return nil
}

func (s *Server) contextHistory(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	saved, err := s.readContextHistory(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 500, "context_history_unavailable", "Private context attribution could not be verified.")
		return
	}
	writeJSON(w, 200, map[string]any{"events": saved.Events, "pruned": saved.Pruned})
}
