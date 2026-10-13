package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// The reservation is private. An interrupted Git write is observed by exact
// parent/path/blob identity and never replayed just because its HTTP reply was
// lost. It can also remain pending for an operator to inspect.
type CoordinatorContextCommit struct {
	EventID      string            `json:"eventId"`
	BaseCommit   string            `json:"baseCommit"`
	ReviewDigest string            `json:"reviewDigest"`
	PathDigests  map[string]string `json:"pathDigests"`
	Status       string            `json:"status"`
	Commit       string            `json:"commit,omitempty"`
	CreatedAt    string            `json:"createdAt"`
}

func validCoordinatorContextCommit(row CoordinatorContextCommit) bool {
	if !strings.HasPrefix(row.EventID, "coord-event-") || !prototypeReleaseID.MatchString(row.EventID) || !deliverySHA.MatchString(row.BaseCommit) || !hexDigest(row.ReviewDigest) || len(row.PathDigests) == 0 || len(row.PathDigests) > 32 || row.Status != "pending" && row.Status != "confirmed" || row.Status == "confirmed" && !deliverySHA.MatchString(row.Commit) || row.Status == "pending" && row.Commit != "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err != nil {
		return false
	}
	for path, digest := range row.PathDigests {
		if !coordinatorContextGitPath(path) || !hexDigest(digest) {
			return false
		}
	}
	return true
}

func coordinatorContextGitPath(path string) bool {
	return path == "LUCI.md" || path == ".luci/state.json" || journeyContextFile(path)
}

func coordinatorContextBytes(state journeyState, path string) ([]byte, error) {
	if path == ".luci/state.json" {
		data, err := json.Marshal(state)
		return append(data, '\n'), err
	}
	if journeyArchivePath(path) {
		return nil, nil // The digest-addressed archive is checked separately.
	}
	exports, err := journeyContextExports(state)
	if err != nil {
		return nil, err
	}
	for _, item := range exports {
		if item.Path == path {
			return item.Data, nil
		}
	}
	return nil, ErrOrchestratorConflict
}

func coordinatorFileDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Only canonical exports and referenced content-addressed history may be
// included. A concurrent user file, modified export, deletion or unexpected
// staged path leaves the source for explicit review.
func (s *Server) coordinatorContextPaths(p project.Project, state journeyState, summary GitSummary) (map[string]string, error) {
	if summary.Clean || len(summary.Changes) > 32 {
		return nil, ErrOrchestratorConflict
	}
	paths := map[string]string{}
	for _, change := range summary.Changes {
		_, duplicate := paths[change.Path]
		if !coordinatorContextGitPath(change.Path) || strings.ContainsAny(change.Status, "DRC") || duplicate {
			return nil, ErrOrchestratorConflict
		}
		path, err := guardedProjectPath(p.Path, change.Path, false)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > journeyStateLimit {
			return nil, ErrOrchestratorConflict
		}
		if journeyArchivePath(change.Path) {
			if journeyArchiveDigest(data) != strings.TrimSuffix(strings.TrimPrefix(change.Path, ".luci/history/"), ".json") {
				return nil, ErrOrchestratorConflict
			}
		} else {
			expected, e := coordinatorContextBytes(state, change.Path)
			if e != nil || !bytes.Equal(data, expected) {
				return nil, ErrOrchestratorConflict
			}
		}
		paths[change.Path] = coordinatorFileDigest(data)
	}
	return paths, nil
}

func (s *Server) observeCoordinatorContextCommit(ctx context.Context, p project.Project, row CoordinatorContextCommit) (string, bool) {
	summary, err := s.gitSummary(ctx, p.Path)
	if err != nil || !summary.Clean {
		return "", false
	}
	head, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	if err != nil || !deliverySHA.MatchString(head) || head == row.BaseCommit {
		return "", false
	}
	parent, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD^")
	if err != nil || strings.TrimSpace(parent) != row.BaseCommit {
		return "", false
	}
	changed, err := s.journeyGit(ctx, p.Path, "diff", "--no-renames", "--name-only", "-z", row.BaseCommit, head, "--")
	if err != nil {
		return "", false
	}
	actual := []string{}
	for _, path := range strings.Split(changed, "\x00") {
		if path != "" {
			actual = append(actual, path)
		}
	}
	expected := make([]string, 0, len(row.PathDigests))
	for path := range row.PathDigests {
		expected = append(expected, path)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		return "", false
	}
	for _, path := range expected {
		file, err := guardedProjectPath(p.Path, path, false)
		if err != nil {
			return "", false
		}
		data, err := os.ReadFile(file)
		if err != nil || coordinatorFileDigest(data) != row.PathDigests[path] {
			return "", false
		}
		liveBlob, err := s.journeyGit(ctx, p.Path, "hash-object", "--", path)
		if err != nil {
			return "", false
		}
		blob, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD:"+path)
		if err != nil || strings.TrimSpace(blob) != strings.TrimSpace(liveBlob) {
			return "", false
		}
	}
	return head, true
}

func (s *Server) commitAcceptedCoordinatorContext(ctx context.Context, p project.Project, eventID, expectedHead string) (bool, error) {
	guard := s.branchGuard(p.ID)
	if !guard.TryLock() {
		return false, nil
	}
	defer guard.Unlock()
	actor, ok := authenticatedContextActor(ctx)
	if !ok {
		return false, errCoordinatorAuthority
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, actor.ID)
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil || stateErr != nil {
		return false, ErrOrchestratorInvalid
	}
	for i, row := range rows.ContextCommits {
		if row.EventID != eventID {
			continue
		}
		if row.Status == "confirmed" {
			return true, nil
		}
		if commit, found := s.observeCoordinatorContextCommit(ctx, p, row); found {
			s.mu.Lock()
			fresh, e := s.readContinuations(p, rows.ActorID)
			if e == nil && i < len(fresh.ContextCommits) && fresh.ContextCommits[i].EventID == eventID && fresh.ContextCommits[i].Status == "pending" {
				fresh.ContextCommits[i].Status, fresh.ContextCommits[i].Commit = "confirmed", commit
				e = s.saveContinuations(fresh)
			}
			s.mu.Unlock()
			return e == nil, e
		}
		return false, nil // Unknown mutation outcome. No second Git invocation.
	}
	summary, err := s.gitSummary(ctx, p.Path)
	if err != nil || summary.Clean || !hexDigest(summary.ReviewDigest) {
		return false, err
	}
	head, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	if err != nil || head != expectedHead {
		return false, ErrOrchestratorConflict
	}
	paths, err := s.coordinatorContextPaths(p, state, summary)
	if err != nil {
		return false, nil
	}
	// The reviewed event and actor must still be current immediately before a
	// durable mutation reservation. branchGuard excludes competing source
	// writers; s.mu protects the coordination event head and private journal.
	if err := s.coordinatorAuthority(ctx, p.ID); err != nil {
		return false, err
	}
	journal := CoordinatorContextCommit{EventID: eventID, BaseCommit: head, ReviewDigest: summary.ReviewDigest, PathDigests: paths, Status: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.mu.Lock()
	fresh, err := s.readContinuations(p, rows.ActorID)
	if err == nil {
		var view CoordinationView
		view, err = s.coordinationViewLocked(ctx, p)
		if err == nil && view.EventHead != eventID {
			err = ErrOrchestratorConflict
		}
	}
	if err == nil && len(fresh.ContextCommits) < coordinatorContinuationLimit {
		fresh.ContextCommits = append(fresh.ContextCommits, journal)
		err = s.saveContinuations(fresh)
	}
	s.mu.Unlock()
	if err != nil {
		return false, err
	}
	body, _ := json.Marshal(map[string]string{"message": "Apply accepted project context " + eventID, "expectedReviewDigest": summary.ReviewDigest})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.SetPathValue("id", p.ID)
	req.Header.Set("Content-Type", "application/json")
	capture := &coordinatorResponse{header: make(http.Header)}
	s.commitProject(capture, req)
	if capture.status != http.StatusOK {
		return false, nil // Pending journal requires readback, not replay.
	}
	commit, found := s.observeCoordinatorContextCommit(ctx, p, journal)
	if !found {
		return false, nil
	}
	s.mu.Lock()
	fresh, err = s.readContinuations(p, rows.ActorID)
	if err == nil && len(fresh.ContextCommits) > 0 && fresh.ContextCommits[len(fresh.ContextCommits)-1].EventID == eventID {
		fresh.ContextCommits[len(fresh.ContextCommits)-1].Status, fresh.ContextCommits[len(fresh.ContextCommits)-1].Commit = "confirmed", commit
		err = s.saveContinuations(fresh)
	}
	s.mu.Unlock()
	return err == nil, err
}
