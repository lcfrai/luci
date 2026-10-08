package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const overviewRunHeadLimit = 8 << 10

type ProjectOverviewRun struct {
	Status        string `json:"status"`
	ID            string `json:"id,omitempty"`
	State         string `json:"state,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	ContextStatus string `json:"contextStatus,omitempty"`
	ContextReason string `json:"contextReason,omitempty"`
}
type overviewRunHead struct {
	Version           int    `json:"version"`
	AccountScope      string `json:"accountScope"`
	ProjectScope      string `json:"projectScope"`
	ProjectID         string `json:"projectId"`
	RunID             string `json:"runId"`
	CreatedAt         string `json:"createdAt"`
	DirectoryIdentity string `json:"directoryIdentity"`
	DirectoryModified string `json:"directoryModified"`
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
	if decoder.Decode(&head) != nil || decoder.Decode(new(any)) != io.EOF || head.Version != 1 || head.AccountScope != s.accountScope || head.ProjectScope != scope || head.ProjectID != projectID || !validRunID(head.RunID) {
		return overviewRunHead{}, ErrProjectRunInvalid
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
	latest := created
	latestAt, _ := time.Parse(time.RFC3339Nano, latest.Run.CreatedAt)
	for _, row := range previous {
		at, err := time.Parse(time.RFC3339Nano, row.Run.CreatedAt)
		if err == nil && at.After(latestAt) {
			latest, latestAt = row, at
		}
	}
	_ = s.writeOverviewRunHead(overviewRunHead{Version: 1, AccountScope: s.accountScope, ProjectScope: latest.ProjectScope, ProjectID: latest.Run.ProjectID, RunID: latest.Run.ID, CreatedAt: latest.Run.CreatedAt})
}
func (s *Server) readOverviewLatestRun(p project.Project, state journeyState) (ProjectOverviewRun, error) {
	store := s.projectRuns
	store.mu.Lock()
	defer store.mu.Unlock()
	scope, err := store.projectScope(p.ID, p.Path)
	if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, err
	}
	if _, err = store.directory(p.ID, false); errors.Is(err, os.ErrNotExist) {
		return ProjectOverviewRun{Status: "none"}, nil
	} else if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, err
	}
	head, err := store.readOverviewRunHead(p.ID, scope)
	if err != nil {
		return ProjectOverviewRun{Status: "unavailable"}, err
	}
	// Exactly one canonical run, at most1MiB. No checks/release files, source
	// commands, child observations, or historical run list are read here.
	row, err := store.read(p.ID, scope, head.RunID)
	if err != nil || row.Run.CreatedAt != head.CreatedAt {
		return ProjectOverviewRun{Status: "unavailable"}, ErrProjectRunInvalid
	}
	freshness := projectRunContextFreshness(row.Run, state, nil)
	return ProjectOverviewRun{Status: "recorded", ID: row.Run.ID, State: row.Run.State, UpdatedAt: overviewTimestamp(row.Run.UpdatedAt), ContextStatus: freshness.Status, ContextReason: freshness.Reason}, nil
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
		view.Phase = ProjectOverviewPhase{ID: "verify", Label: "Verify"}
		action.Label = "Review verification"
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
