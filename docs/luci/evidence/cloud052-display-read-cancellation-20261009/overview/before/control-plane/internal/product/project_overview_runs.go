package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const overviewRunHeadLimit = 8 << 10

// Version 3 also indexes unanswered handoffs from completed/older attempts.
const overviewRunHeadVersion = 3
const overviewRunAttentionLimit = 3

type ProjectOverviewAttention struct {
	Status  string                `json:"status"`
	Count   *int                  `json:"count"`
	Items   []ProjectRunAttention `json:"items"`
	Omitted *int                  `json:"omitted"`
}

type ProjectOverviewRun struct {
	Status        string `json:"status"`
	ID            string `json:"id,omitempty"`
	State         string `json:"state,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	ContextStatus string `json:"contextStatus,omitempty"`
	ContextReason string `json:"contextReason,omitempty"`
	// Derived from the one retained run already read below. This controls the
	// overview wording; it is not verification or integration authority.
	taskReviewRequired bool
}
type overviewRunHead struct {
	Version           int      `json:"version"`
	AccountScope      string   `json:"accountScope"`
	ProjectScope      string   `json:"projectScope"`
	ProjectID         string   `json:"projectId"`
	RunID             string   `json:"runId"`
	CreatedAt         string   `json:"createdAt"`
	DirectoryIdentity string   `json:"directoryIdentity"`
	DirectoryModified string   `json:"directoryModified"`
	AttentionIDs      []string `json:"attentionIds,omitempty"`
}

func (s *ProjectRunStore) overviewRunHeadPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrProjectRunInvalid
	}
	root, err := guardedProjectPath(s.stateDir, "project-overview-runs", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if _, err = guardedProjectPath(s.stateDir, "project-overview-runs", false); err != nil {
		return "", err
	}
	if create && runtime.GOOS == "windows" {
		if err = restrictPrivateFile(root); err != nil {
			return "", err
		}
	}
	return guardedProjectPath(s.stateDir, "project-overview-runs/"+id+".json", create)
}
func (s *ProjectRunStore) readOverviewRunHead(projectID, scope string) (overviewRunHead, error) {
	path, err := s.overviewRunHeadPath(projectID, false)
	if err != nil {
		return overviewRunHead{}, err
	}
	file, err := openOverviewRegular(path, overviewRunHeadLimit)
	if err != nil {
		return overviewRunHead{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, overviewRunHeadLimit+1))
	if err != nil || len(data) > overviewRunHeadLimit {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	var head overviewRunHead
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&head) != nil || decoder.Decode(new(any)) != io.EOF || (head.Version < 1 || head.Version > overviewRunHeadVersion) || head.AccountScope != s.accountScope || head.ProjectScope != scope || head.ProjectID != projectID || !validRunID(head.RunID) {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	if len(head.AttentionIDs) > projectRunHistoryLimit || head.Version == 1 && len(head.AttentionIDs) != 0 {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	seen := map[string]bool{}
	for _, id := range head.AttentionIDs {
		if !validRunID(id) || seen[id] {
			return overviewRunHead{}, ErrProjectRunInvalid
		}
		seen[id] = true
	}
	if _, err = time.Parse(time.RFC3339Nano, head.CreatedAt); err != nil {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	dir, err := s.directory(projectID, false)
	if err != nil {
		return overviewRunHead{}, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	identity, err := projectImportDirectoryIdentity(dir)
	if err != nil || identity != head.DirectoryIdentity || info.ModTime().UTC().Format(time.RFC3339Nano) != head.DirectoryModified {
		return overviewRunHead{}, ErrProjectRunInvalid
	}
	return head, nil
}
func (s *ProjectRunStore) writeOverviewRunHead(head overviewRunHead) error {
	dir, err := s.directory(head.ProjectID, false)
	if err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return ErrProjectRunInvalid
	}
	head.DirectoryIdentity, err = projectImportDirectoryIdentity(dir)
	if err != nil {
		return err
	}
	head.DirectoryModified = info.ModTime().UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(head, "", "  ")
	if err != nil || len(data)+1 > overviewRunHeadLimit {
		return ErrProjectRunLimit
	}
	path, err := s.overviewRunHeadPath(head.ProjectID, true)
	if err != nil {
		return err
	}
	if err = hostpath.OneName(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = atomicJSON(path, head); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}

// Creation already has the validated history for idempotency/capacity. Reuse it
// to establish a trustworthy latest pointer, without another history scan.
func (s *ProjectRunStore) overviewHeadAfterCreate(created storedProjectRun, previous []storedProjectRun) {
	rows := append(append([]storedProjectRun{}, previous...), created)
	s.overviewHeadFromHistory(rows)
}

func (s *ProjectRunStore) overviewHeadFromHistory(rows []storedProjectRun) {
	if len(rows) == 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Run.CreatedAt > rows[j].Run.CreatedAt })
	created, previous := rows[0], rows[1:]
	latest := created
	latestAt, _ := time.Parse(time.RFC3339Nano, latest.Run.CreatedAt)
	for _, row := range previous {
		at, err := time.Parse(time.RFC3339Nano, row.Run.CreatedAt)
		if err == nil && at.After(latestAt) {
			latest, latestAt = row, at
		}
	}
	head := overviewRunHead{Version: overviewRunHeadVersion, AccountScope: s.accountScope, ProjectScope: latest.ProjectScope, ProjectID: latest.Run.ProjectID, RunID: latest.Run.ID, CreatedAt: latest.Run.CreatedAt}
	for _, row := range rows {
		if projectRunAttentionKind(row.Run) != "" {
			head.AttentionIDs = append(head.AttentionIDs, row.Run.ID)
		}
	}
	_ = s.writeOverviewRunHead(head)
}

func updateOverviewRunAttention(head *overviewRunHead, run ProjectRun) {
	wanted := projectRunAttentionKind(run) != ""
	for i, id := range head.AttentionIDs {
		if id == run.ID {
			if !wanted {
				head.AttentionIDs = append(head.AttentionIDs[:i], head.AttentionIDs[i+1:]...)
			}
			return
		}
	}
	if wanted {
		head.AttentionIDs = append([]string{run.ID}, head.AttentionIDs...)
	}
}

func (s *Server) readOverviewRuns(p project.Project, state journeyState) (ProjectOverviewRun, ProjectOverviewAttention, error) {
	attention := ProjectOverviewAttention{Status: "unavailable", Items: []ProjectRunAttention{}}
	store := s.projectRuns
	store.mu.Lock()
	defer store.mu.Unlock()
	scope, err := store.projectScope(p.ID, p.Path)
	if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, attention, err
	}
	if _, err = store.directory(p.ID, false); errors.Is(err, os.ErrNotExist) {
		attention.Status, attention.Count, attention.Omitted = "recorded", overviewInt(0), overviewInt(0)
		return ProjectOverviewRun{Status: "none"}, attention, nil
	} else if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, attention, err
	}
	head, err := store.readOverviewRunHead(p.ID, scope)
	if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, attention, err
	}
	// One latest run plus at most three indexed attention runs, each <=1MiB.
	// No historical scan, check/release files, source commands or observations.
	row, err := store.read(p.ID, scope, head.RunID)
	if err != nil || row.Run.CreatedAt != head.CreatedAt {
		return ProjectOverviewRun{Status: "unavailable"}, attention, ErrProjectRunInvalid
	}
	freshness := projectRunContextFreshness(row.Run, state, nil)
	reviewRequired := false
	if row.Run.Swarm != nil {
		for _, task := range row.Run.Swarm.Tasks {
			if task.Integration != nil && task.Integration.State == "integrated" {
				continue
			}
			op := swarmOperation(&row.Run, task.OperationID)
			if op != nil && (op.Status == "review-required" || op.Status == "succeeded") {
				reviewRequired = true
				break
			}
		}
	}
	latest := ProjectOverviewRun{Status: "recorded", ID: row.Run.ID, State: row.Run.State, UpdatedAt: overviewTimestamp(row.Run.UpdatedAt), ContextStatus: freshness.Status, ContextReason: freshness.Reason, taskReviewRequired: reviewRequired}
	if head.Version != overviewRunHeadVersion {
		return latest, attention, ErrProjectRunInvalid
	}
	for _, id := range head.AttentionIDs {
		if len(attention.Items) == overviewRunAttentionLimit {
			break
		}
		selected := row
		if id != row.Run.ID {
			selected, err = store.read(p.ID, scope, id)
			if err != nil {
				return latest, ProjectOverviewAttention{Status: "unavailable", Items: []ProjectRunAttention{}}, err
			}
		}
		if kind := projectRunAttentionKind(selected.Run); kind != "" {
			comparison := projectRunContextFreshness(selected.Run, state, nil)
			attention.Items = append(attention.Items, ProjectRunAttention{ID: id, State: selected.Run.State, Kind: kind, ContextStatus: comparison.Status})
		} else {
			return latest, ProjectOverviewAttention{Status: "unavailable", Items: []ProjectRunAttention{}}, ErrProjectRunInvalid
		}
	}
	attention.Status, attention.Count, attention.Omitted = "recorded", overviewInt(len(head.AttentionIDs)), overviewInt(len(head.AttentionIDs)-len(attention.Items))
	return latest, attention, nil
}
func applyOverviewRunProgress(view *ProjectOverview, run ProjectOverviewRun) {
	view.LatestRun = run
	if run.Status != "recorded" {
		return
	}
	if run.ContextStatus != "current" {
		view.NextAction = OrchestratorAction{ID: "review-plan", Label: "Review changed build context", Lens: "develop", RunID: run.ID, Reason: run.ContextReason}
		return
	}
	action := OrchestratorAction{ID: "inspect-run", Label: "Inspect the latest build", Lens: "develop", RunID: run.ID, Reason: "This stage comes from the saved run. Inspect its source-bound evidence and current status before relying on the result."}
	switch run.State {
	case "coding":
		action.Label = "Follow implementation"
	case "awaiting-checks":
		if run.taskReviewRequired {
			view.Phase = ProjectOverviewPhase{ID: "develop", Label: "Develop"}
			action.Label = "Review task changes"
			action.Reason = "An agent has stopped and its retained changes need integration review before whole-build verification. Process exit does not establish correctness."
		} else {
			view.Phase = ProjectOverviewPhase{ID: "verify", Label: "Verify"}
			action.Label = "Review verification"
		}
	case "awaiting-release":
		view.Phase = ProjectOverviewPhase{ID: "deploy", Label: "Deploy"}
		action.Label = "Review the release"
	case "complete":
		view.Phase = ProjectOverviewPhase{ID: "complete", Label: "Accepted result"}
		action.Label = "Inspect the accepted result"
	case "blocked":
		action.Label = "Review the blocked build"
	case "cancelled":
		action.Label = "Review cancelled work"
	}
	view.NextAction = action
}
