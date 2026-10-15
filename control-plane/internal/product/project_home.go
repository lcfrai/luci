package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const homePreferenceLimit = 1 << 20
const homeStopWindow = 24 * time.Hour

// HomeStop is display evidence from a retained terminal record. Its identity
// binds the source generation and terminal observation, never an assistant claim.
type HomeStop struct {
	ID   string `json:"id"`
	At   string `json:"at"`
	Kind string `json:"kind"`
}

type HomeSignals struct {
	ProjectID  string     `json:"projectId"`
	NeedsInput bool       `json:"needsInput"`
	Working    bool       `json:"working"`
	Queued     bool       `json:"queued"`
	Unknown    bool       `json:"unknown"`
	Stops      []HomeStop `json:"stops"`
}

type HomePreferences struct {
	Version             int                 `json:"version"`
	Revision            int                 `json:"revision"`
	AccountScope        string              `json:"accountScope"`
	WorkspaceScope      string              `json:"workspaceScope"`
	ActorID             string              `json:"actorId"`
	PinnedProjectIDs    []string            `json:"pinnedProjectIds"`
	AcknowledgedStopIDs map[string][]string `json:"acknowledgedStopIds"`
	AcknowledgedAt      map[string]string   `json:"acknowledgedAt,omitempty"`
}

func homeStopID(source, projectID, identity, generation, at string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + projectID + "\x00" + identity + "\x00" + generation + "\x00" + at))
	return "stop-" + hex.EncodeToString(sum[:])
}

func HomeStopEvent(source, projectID, identity, generation, at, kind string, now time.Time) (HomeStop, bool) {
	when, err := time.Parse(time.RFC3339Nano, at)
	if err != nil || when.After(now.Add(time.Minute)) || !when.After(now.Add(-homeStopWindow)) || identity == "" || generation == "" {
		return HomeStop{}, false
	}
	return HomeStop{ID: homeStopID(source, projectID, identity, generation, at), At: when.UTC().Format(time.RFC3339Nano), Kind: kind}, true
}

func (s *Server) homePreferenceScope(ctx context.Context) (HomePreferences, string, error) {
	actor, ok := authenticatedContextActor(ctx)
	if !ok || s.projectRuns == nil {
		return HomePreferences{}, "", errors.New("authenticated project actor required")
	}
	// The caller holds s.mu while resolving the preference path. projectRoot()
	// would try to take that lock again during a pin or acknowledgment write.
	workspace := management.TeamRuntimeIDFromContext(ctx) + "\x00" + s.config.ProjectsDir
	scope := sha256.Sum256([]byte(workspace))
	value := HomePreferences{Version: 1, Revision: 1, AccountScope: s.projectRuns.accountScope,
		WorkspaceScope: hex.EncodeToString(scope[:]), ActorID: actor.ID,
		PinnedProjectIDs: []string{}, AcknowledgedStopIDs: map[string][]string{}, AcknowledgedAt: map[string]string{}}
	name := sha256.Sum256([]byte(value.AccountScope + "\x00" + value.WorkspaceScope + "\x00" + actor.ID))
	path, err := guardedProjectPath(s.stateDir, "home-preferences-"+hex.EncodeToString(name[:])+".json", true)
	return value, path, err
}

func (s *Server) homePreferencesLocked(ctx context.Context) (HomePreferences, string, error) {
	want, path, err := s.homePreferenceScope(ctx)
	if err != nil {
		return want, "", err
	}
	file, err := openOverviewRegular(path, homePreferenceLimit)
	if errors.Is(err, os.ErrNotExist) {
		return want, path, nil
	}
	if err != nil {
		return want, "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, homePreferenceLimit+1))
	if err != nil || len(raw) > homePreferenceLimit {
		return want, "", errors.New("home preferences unavailable")
	}
	var saved HomePreferences
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF || saved.Version != 1 || saved.Revision < 2 ||
		saved.AccountScope != want.AccountScope || saved.WorkspaceScope != want.WorkspaceScope || saved.ActorID != want.ActorID ||
		len(saved.PinnedProjectIDs) > 256 || len(saved.AcknowledgedStopIDs) > 4096 || len(saved.AcknowledgedAt) > 4096 {
		return want, "", errors.New("home preferences invalid")
	}
	seen := map[string]bool{}
	for _, id := range saved.PinnedProjectIDs {
		if !prototypeReleaseID.MatchString(id) || seen[id] {
			return want, "", errors.New("home preferences invalid")
		}
		seen[id] = true
	}
	for projectID, ids := range saved.AcknowledgedStopIDs {
		if !prototypeReleaseID.MatchString(projectID) || len(ids) > 32 {
			return want, "", errors.New("home preferences invalid")
		}
		seen = map[string]bool{}
		for _, id := range ids {
			if len(id) != 69 || id[:5] != "stop-" || seen[id] {
				return want, "", errors.New("home preferences invalid")
			}
			if _, err := hex.DecodeString(id[5:]); err != nil {
				return want, "", errors.New("home preferences invalid")
			}
			seen[id] = true
		}
	}
	if saved.AcknowledgedStopIDs == nil {
		saved.AcknowledgedStopIDs = map[string][]string{}
	}
	if saved.AcknowledgedAt == nil {
		saved.AcknowledgedAt = map[string]string{}
	}
	// Older preferences predate timestamps. Their last durable write is a
	// conservative expiry anchor; an acknowledgment only hides 24-hour stops.
	info, err := file.Stat()
	if err != nil {
		return want, "", err
	}
	for projectID := range saved.AcknowledgedAt {
		if _, ok := saved.AcknowledgedStopIDs[projectID]; !ok {
			return want, "", errors.New("home preferences invalid")
		}
	}
	for projectID := range saved.AcknowledgedStopIDs {
		at := saved.AcknowledgedAt[projectID]
		when := info.ModTime()
		if at != "" {
			when, err = time.Parse(time.RFC3339Nano, at)
			if err != nil || when.After(time.Now().Add(time.Minute)) {
				return want, "", errors.New("home preferences invalid")
			}
		}
		if !when.After(time.Now().Add(-homeStopWindow)) {
			delete(saved.AcknowledgedStopIDs, projectID)
			delete(saved.AcknowledgedAt, projectID)
		} else if at == "" {
			saved.AcknowledgedAt[projectID] = when.UTC().Format(time.RFC3339Nano)
		}
	}
	return saved, path, nil
}

func (s *Server) HomePreferences(ctx context.Context) (HomePreferences, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prefs, _, err := s.homePreferencesLocked(ctx)
	return prefs, err
}

// HomePin changes only this actor's preference. The caller must authorize the
// currently accessible project before entering, including a fresh team grant.
func (s *Server) HomePin(ctx context.Context, projectID string, pinned bool, expected int) (HomePreferences, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return HomePreferences{}, ErrProjectRunInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, path, err := s.homePreferencesLocked(ctx)
	if err != nil {
		return prefs, err
	}
	if expected != prefs.Revision {
		return prefs, ErrProjectRunConflict
	}
	ids := make([]string, 0, len(prefs.PinnedProjectIDs)+1)
	for _, id := range prefs.PinnedProjectIDs {
		if id != projectID {
			ids = append(ids, id)
		}
	}
	if pinned {
		ids = append(ids, projectID)
	}
	if len(ids) > 256 {
		return prefs, ErrProjectRunLimit
	}
	prefs.PinnedProjectIDs = ids
	prefs.Revision++
	if err := saveHomePreferences(path, prefs); err != nil {
		return HomePreferences{}, err
	}
	return prefs, nil
}

// Acknowledgment accepts only exact stop IDs from the current authorized
// projection. A later stop cannot be acknowledged by an older browser view.
func (s *Server) HomeAcknowledge(ctx context.Context, projectID string, expected int, observed, current []string) (HomePreferences, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return HomePreferences{}, ErrProjectRunInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, path, err := s.homePreferencesLocked(ctx)
	if err != nil {
		return prefs, err
	}
	if expected != prefs.Revision {
		return prefs, ErrProjectRunConflict
	}
	if len(observed) == 0 || len(observed) > 32 {
		return prefs, ErrProjectRunInvalid
	}
	available := map[string]bool{}
	for _, id := range current {
		available[id] = true
	}
	added := map[string]bool{}
	for _, id := range prefs.AcknowledgedStopIDs[projectID] {
		added[id] = true
	}
	for _, id := range observed {
		if !available[id] {
			return prefs, ErrProjectRunConflict
		}
		added[id] = true
	}
	ids := make([]string, 0, len(added))
	for _, id := range current {
		if added[id] {
			ids = append(ids, id)
		}
	}
	if len(ids) > 32 {
		ids = ids[len(ids)-32:]
	}
	if _, exists := prefs.AcknowledgedStopIDs[projectID]; !exists && len(prefs.AcknowledgedStopIDs) >= 256 {
		return prefs, ErrProjectRunLimit
	}
	prefs.AcknowledgedStopIDs[projectID] = ids
	prefs.AcknowledgedAt[projectID] = time.Now().UTC().Format(time.RFC3339Nano)
	prefs.Revision++
	if err := saveHomePreferences(path, prefs); err != nil {
		return HomePreferences{}, err
	}
	return prefs, nil
}

func saveHomePreferences(path string, prefs HomePreferences) error {
	raw, err := json.Marshal(prefs)
	if err != nil || len(raw) > homePreferenceLimit {
		return ErrProjectRunLimit
	}
	return durableRunJSON(path, prefs)
}

// HomeProjectSignals reuses bounded overview projections and one scheduler read.
// It never dispatches, observes a child, reads source files or calls a model.
func (s *Server) HomeProjectSignals(ctx context.Context, projects []project.Project, actorID string, now time.Time) (map[string]HomeSignals, error) {
	if len(projects) > 256 {
		return nil, ErrProjectRunLimit
	}
	s.swarmScheduler.mu.Lock()
	scheduler, schedulerErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	out := make(map[string]HomeSignals, len(projects))
	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		signal := HomeSignals{ProjectID: p.ID, Stops: []HomeStop{}}
		view, err := s.cachedProjectOverview(ctx, p)
		if err != nil || view.Status == "unavailable" {
			signal.Unknown = true
		} else {
			signal.NeedsInput = view.Questions.Status == "recorded" && view.Questions.Count != nil && *view.Questions.Count > 0 ||
				view.Attention.Status == "recorded" && view.Attention.Count != nil && *view.Attention.Count > 0
			if view.Questions.Status != "recorded" || view.Attention.Status != "recorded" {
				signal.Unknown = true
			}
			if view.LatestRun.Status == "recorded" {
				switch view.LatestRun.State {
				case "running", "checking", "integrating", "deploying":
					signal.Working = true
				case "unknown":
					signal.Unknown = true
				}
			} else if view.LatestRun.Status == "unavailable" {
				signal.Unknown = true
			}
		}
		if schedulerErr != nil {
			signal.Unknown = true
		} else {
			queue := queueProjectSnapshot(scheduler, p.ID)
			signal.Working = signal.Working || queue.Counts.Running+queue.Counts.Reserved > 0
			signal.Queued = queue.Counts.Queued+queue.Counts.Paused > 0
			signal.NeedsInput = signal.NeedsInput || queue.Counts.Blocked > 0
			signal.Unknown = signal.Unknown || queue.Counts.Unknown > 0
			for _, item := range queue.Items {
				if !swarmQueueTerminal(item.State) {
					continue
				}
				if stop, ok := HomeStopEvent("queue", p.ID, item.ID, item.RunID+":"+item.OperationID+":"+item.State,
					item.UpdatedAt, item.State, now); ok {
					signal.Stops = append(signal.Stops, stop)
				}
			}
		}
		if actorID != "" {
			s.mu.RLock()
			private, privateErr := s.readContinuations(p, actorID)
			s.mu.RUnlock()
			if privateErr != nil {
				signal.Unknown = true
			} else {
				for _, finding := range private.Findings {
					if finding.Status == "question" || finding.Status == "review" {
						signal.NeedsInput = true
					}
					if finding.Status == "unknown" {
						signal.Unknown = true
					}
				}
			}
		}
		sort.Slice(signal.Stops, func(i, j int) bool {
			if signal.Stops[i].At == signal.Stops[j].At {
				return signal.Stops[i].ID < signal.Stops[j].ID
			}
			return signal.Stops[i].At < signal.Stops[j].At
		})
		if len(signal.Stops) > 32 {
			signal.Stops = signal.Stops[len(signal.Stops)-32:]
		}
		out[p.ID] = signal
	}
	return out, nil
}
