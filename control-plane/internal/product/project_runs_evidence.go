package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// SnapshotProjectRunCheck resolves a private, completed check and retains a
// bounded copy. A pruned ordinary history row never erases this provenance.
func (s *Server) SnapshotProjectRunCheck(ctx context.Context, projectID, checkID string, approvals ProjectRunApprovalSelection, requirements []RequirementSnapshot) (ProjectRunCheckEvidence, error) {
	if err := s.validateProjectRunApprovals(projectID, approvals); err != nil {
		return ProjectRunCheckEvidence{}, err
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRunCheckEvidence{}, err
	}
	coverage, rows, err := s.readProjectEvidence(ctx, p.ID, p.Path)
	if err != nil {
		return ProjectRunCheckEvidence{}, err
	}
	for _, row := range rows {
		if row.ID != checkID {
			continue
		}
		snapshot, err := projectRunCheckSnapshot(row, projectID, approvals, requirements, coverage.Source)
		if err != nil {
			return ProjectRunCheckEvidence{}, err
		}
		if err = s.validateProjectRunApprovals(projectID, approvals); err != nil {
			return ProjectRunCheckEvidence{}, err
		}
		return snapshot, nil
	}
	return ProjectRunCheckEvidence{}, errors.New("This project has no retained check with that ID. Run the selected checks again.")
}

func projectRunCheckSnapshot(row ProjectCheckRun, projectID string, approvals ProjectRunApprovalSelection, requirements []RequirementSnapshot, source EvidenceSource) (ProjectRunCheckEvidence, error) {
	if row.ProjectID != projectID || row.EvidenceStatus != evidenceLinked || row.Status != "passed" || row.SourceChanged || row.Dirty || !source.Available || source.Dirty || row.Commit != source.Commit || row.SourceDigest != source.SourceDigest || !deliverySHA.MatchString(row.Commit) || !hexDigest(row.SourceDigest) {
		return ProjectRunCheckEvidence{}, errors.New("Checks must pass against this exact clean committed source. Commit changes and rerun the linked commands.")
	}
	if row.ApprovalSelection == nil || *row.ApprovalSelection != approvals {
		return ProjectRunCheckEvidence{}, errors.New("The check has no matching private review selection. Rerun it against the run's current approved inputs.")
	}
	if len(requirements) == 0 || len(requirements) > 100 {
		return ProjectRunCheckEvidence{}, errors.New("Select saved requirements before joining check evidence.")
	}
	covered := map[string]RequirementSnapshot{}
	for _, requirement := range row.RequirementSnapshots {
		if _, duplicate := covered[requirement.ID]; duplicate {
			return ProjectRunCheckEvidence{}, errors.New("The check has ambiguous requirement coverage.")
		}
		covered[requirement.ID] = requirement
	}
	seen := map[string]bool{}
	for _, requirement := range requirements {
		if !designIDPattern.MatchString(requirement.ID) || seen[requirement.ID] || covered[requirement.ID] != requirement {
			return ProjectRunCheckEvidence{}, errors.New("The passing check must cover every selected requirement and its exact acceptance text.")
		}
		seen[requirement.ID] = true
	}
	commands := make([]CheckCommand, 0, len(row.Commands))
	out := ProjectRunCheckEvidence{CheckID: row.ID, ProjectID: projectID, Commit: row.Commit, SourceDigest: row.SourceDigest, Execution: row.Execution, CreatedAt: row.CreatedAt, JoinedAt: time.Now().UTC().Format(time.RFC3339Nano), Requirements: append([]RequirementSnapshot(nil), requirements...), Approvals: approvals, OriginalAvailable: true}
	for _, command := range row.Commands {
		if command.Status != "passed" || command.ExitCode == nil || *command.ExitCode != 0 {
			return ProjectRunCheckEvidence{}, errors.New("Every selected check command needs an actual successful exit code.")
		}
		definition := command.CheckCommand
		definition.Args = append([]string(nil), definition.Args...)
		commands = append(commands, definition)
		out.Commands = append(out.Commands, ProjectRunCommandEvidence{CheckCommand: definition, Status: command.Status, ExitCode: *command.ExitCode})
	}
	if err := validateCheckCommands(commands); err != nil {
		return ProjectRunCheckEvidence{}, err
	}
	return out, nil
}

// ProjectRunCheckOriginalAvailable is a read-only projection. A false result
// changes availability of the original, never the immutable accepted snapshot.
func (s *Server) ProjectRunCheckOriginalAvailable(projectID, checkID string) (bool, error) {
	if _, err := s.resolveProject(projectID); err != nil {
		return false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readCheckHistory(projectID)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.ID == checkID {
			return true, nil
		}
	}
	return false, nil
}

// releaseRunProvenance is a private admission receipt, separate from public
// release DTOs. Old releases deliberately have no receipt and are not backfilled.
type releaseRunProvenance struct {
	Version      int                         `json:"version"`
	AccountScope string                      `json:"accountScope"`
	ProjectScope string                      `json:"projectScope"`
	ProjectID    string                      `json:"projectId"`
	ReleaseID    string                      `json:"releaseId"`
	Target       string                      `json:"target"`
	Commit       string                      `json:"commit"`
	SourceDigest string                      `json:"sourceDigest"`
	Approvals    ProjectRunApprovalSelection `json:"approvals"`
	CreatedAt    string                      `json:"createdAt"`
}

func (s *Server) releaseRunProvenancePath(releaseID string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(releaseID) {
		return "", errors.New("Invalid release evidence identifier.")
	}
	root, err := guardedProjectPath(s.stateDir, "release-provenance", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	root, err = guardedProjectPath(s.stateDir, "release-provenance", false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("Private release evidence is unavailable.")
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
	return guardedProjectPath(s.stateDir, "release-provenance/"+releaseID+".json", create)
}

// RecordProjectRunReleaseProvenance is called only during a new typed release
// admission, before allocating a resource. It cannot repair historical rows.
// Callers serialize the existing release admission and validate reviewed source.
func (s *Server) RecordProjectRunReleaseProvenance(ctx context.Context, projectID, releaseID, target, commit string) error {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	selected, err := s.ApprovalSelection(projectID)
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	digest, currentCommit, dirty, err := s.checkSource(bounded, p.Path)
	if err != nil || dirty || currentCommit != commit || !deliverySHA.MatchString(commit) || !hexDigest(digest) {
		return errors.New("Release provenance requires the exact clean committed source. Commit and review it before deploying.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return err
	}
	current, err := projectRunApprovalSelection(state)
	if err != nil || current != selected {
		return errors.New("Review selections changed during release admission. Refresh before deploying.")
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return err
	}
	path, err := s.releaseRunProvenancePath(releaseID, true)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return errors.New("This release already has admission provenance; it cannot be replaced.")
	}
	return atomicJSON(path, releaseRunProvenance{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: projectID, ReleaseID: releaseID, Target: target, Commit: commit, SourceDigest: digest, Approvals: selected, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Server) readReleaseRunProvenance(projectID, releaseID string) (releaseRunProvenance, error) {
	var out releaseRunProvenance
	p, err := s.resolveProject(projectID)
	if err != nil {
		return out, err
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return out, err
	}
	path, err := s.releaseRunProvenancePath(releaseID, false)
	if err != nil {
		return out, errors.New("This historical release has no private source and review provenance. Create a new release for this run.")
	}
	if err := hostpath.OneName(path); err != nil {
		return out, errors.New("Release provenance must have one private file name.")
	}
	f, err := os.Open(path)
	if err != nil {
		return out, errors.New("This historical release has no private source and review provenance. Create a new release for this run.")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		return out, errors.New("Private release provenance is invalid.")
	}
	data, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return out, errors.New("Private release provenance exceeds its limit.")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || decoder.Decode(new(any)) != io.EOF || out.Version != 1 || out.ProjectID != projectID || out.ReleaseID != releaseID || out.AccountScope != scope.AccountScope || out.ProjectScope != scope.ProjectScope || !deliverySHA.MatchString(out.Commit) || !hexDigest(out.SourceDigest) {
		return releaseRunProvenance{}, errors.New("Private release provenance does not belong to this worker and project.")
	}
	return out, nil
}

// This narrow observation comes from the selected worker's native platform;
// product never depends on the HTTP API or accepts client observation bodies.
type ProjectRunNativeReleaseObservation struct {
	ReleaseID, ProjectID, Target, Commit, URL, ObservedAt, Limitations string
	Ready                                                              bool
}

// SetProjectRunNativeReleaseObserver connects this worker's platform during
// composition. A missing observer is unverified evidence, never ready by fiat.
func (s *Server) SetProjectRunNativeReleaseObserver(observe func(context.Context, string, string) (ProjectRunNativeReleaseObservation, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projectRunNativeReleaseObserver = observe
}

func (s *Server) SnapshotProjectRunRelease(ctx context.Context, projectID, releaseID, target, outcome string, approvals ProjectRunApprovalSelection, check ProjectRunCheckEvidence) (ProjectRunReleaseEvidence, error) {
	if err := s.validateProjectRunApprovals(projectID, approvals); err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	provenance, err := s.readReleaseRunProvenance(projectID, releaseID)
	if err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	if target == "" || provenance.Target != target || provenance.Approvals != approvals || check.ProjectID != projectID || check.Approvals != approvals || check.Commit != provenance.Commit || check.SourceDigest != provenance.SourceDigest || check.CheckID == "" {
		return ProjectRunReleaseEvidence{}, errors.New("The selected release must match this project's checked source, target and private review selection.")
	}
	local := target == "local" || target == "native-docker"
	if outcome != "preview" && outcome != "publish" || outcome == "preview" && !local || outcome == "publish" && local {
		return ProjectRunReleaseEvidence{}, errors.New("Choose a release that matches the run's intended preview or publication outcome.")
	}
	if err = s.validateProjectRunCandidate(ctx, projectID, check); err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	url, limitations, err := s.observeProjectRunRelease(bounded, projectID, releaseID, target, provenance.Commit)
	if err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	if err = s.validateProjectRunApprovals(projectID, approvals); err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	// Provider and health observation can take seconds. Preserve the historical
	// candidate, but do not admit it as current if source changed while waiting.
	if err = s.validateProjectRunCandidate(ctx, projectID, check); err != nil {
		return ProjectRunReleaseEvidence{}, err
	}
	return ProjectRunReleaseEvidence{ReleaseID: releaseID, ProjectID: projectID, Target: target, Outcome: outcome, Commit: provenance.Commit, SourceDigest: provenance.SourceDigest, URL: url, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Limitations: limitations, Approvals: approvals, CheckID: check.CheckID}, nil
}

func (s *Server) validateProjectRunCandidate(ctx context.Context, projectID string, check ProjectRunCheckEvidence) error {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	release, err := s.AcquireGitSessionAdmission(ctx, projectID)
	if err != nil {
		return err
	}
	defer release()
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	digest, commit, dirty, err := s.checkSource(bounded, p.Path)
	if err != nil || dirty || commit != check.Commit || digest != check.SourceDigest {
		return errors.New("The checked candidate changed. Commit the current source and join a fresh passing check.")
	}
	return nil
}

func (s *Server) observeProjectRunRelease(ctx context.Context, projectID, releaseID, target, commit string) (string, string, error) {
	if target == "local" {
		s.mu.RLock()
		rows, err := s.readReleases(projectID)
		s.mu.RUnlock()
		if err != nil {
			return "", "", err
		}
		for _, row := range rows {
			if row.ID == releaseID && row.ProjectID == projectID && row.Target == target && row.Commit == commit && row.Status == "ready" {
				if err = s.verifyReleasePreview(ctx, row); err != nil {
					return "", "", errors.New("The release's owned preview is unavailable or its binding changed.")
				}
				if err = checkPreviewHTTP(ctx, row.URL); err != nil {
					return "", "", errors.New("The owned preview did not pass its fresh HTTP health check.")
				}
				return row.URL, "Local HTTP health and container ownership were observed. This is not a public deployment or full behavior acceptance.", nil
			}
		}
		return "", "", errors.New("This project has no ready local release for the selected commit.")
	}
	if strings.HasPrefix(target, "native-") {
		s.mu.RLock()
		observer := s.projectRunNativeReleaseObserver
		s.mu.RUnlock()
		if observer == nil {
			return "", "", errors.New("The selected workspace's native release observer is unavailable.")
		}
		observed, err := observer(ctx, projectID, releaseID)
		at, timeErr := time.Parse(time.RFC3339Nano, observed.ObservedAt)
		if err != nil || !observed.Ready || observed.ReleaseID != releaseID || observed.ProjectID != projectID || observed.Target != target || observed.Commit != commit || timeErr != nil || time.Since(at) > time.Minute || time.Until(at) > 5*time.Second {
			return "", "", errors.New("The native release's ownership, exact commit and current health were not verified.")
		}
		return observed.URL, observed.Limitations, nil
	}
	if target == "github-pages" {
		return s.observeProjectRunPages(ctx, projectID, releaseID, commit)
	}
	return s.observeProjectRunCloud(ctx, projectID, releaseID, target, commit)
}

func (s *Server) observeProjectRunPages(ctx context.Context, projectID, releaseID, commit string) (string, string, error) {
	s.mu.RLock()
	rows, err := s.readPagesReleases(projectID)
	s.mu.RUnlock()
	if err != nil {
		return "", "", err
	}
	for _, row := range rows {
		if row.ID != releaseID || row.ProjectID != projectID || row.Commit != commit || row.Target != "github-pages" || row.Status != "ready" || !row.OwnsConfiguration {
			continue
		}
		site, code, err := s.pagesSite(ctx, row.Repository)
		if err != nil || code != 200 || !pagesSiteMatches(site, row) {
			return "", "", errors.New("The owned Pages configuration changed or could not be verified.")
		}
		code, data, err := s.pagesAPI(ctx, "GET", "repos/"+row.Repository+"/pages/builds/latest")
		var build pagesBuild
		if err != nil || code != 200 || json.Unmarshal(data, &build) != nil {
			return "", "", errors.New("The current Pages build could not be observed.")
		}
		observed := observePagesBuild(ctx, row, build, verifyPublicPagesIndex)
		if observed.Status != "ready" {
			return "", "", errors.New("Pages has not freshly confirmed this exact commit and its published index bytes.")
		}
		return observed.URL, "Public HTTPS index bytes, exact build commit and owned Pages configuration were observed. Other browser behavior and inbox delivery are not established.", nil
	}
	return "", "", errors.New("This project has no owned ready Pages release for the selected commit.")
}

func (s *Server) observeProjectRunCloud(ctx context.Context, projectID, releaseID, targetID, commit string) (string, string, error) {
	s.mu.RLock()
	rows, err := s.readCloudReleases(projectID)
	s.mu.RUnlock()
	if err != nil {
		return "", "", err
	}
	for _, row := range rows {
		if row.ID != releaseID || row.ProjectID != projectID || row.TargetID != targetID || row.Target != targetID || row.Release.Commit != commit || row.Status != "ready" {
			continue
		}
		for _, other := range rows {
			if other.ID != row.ID && other.TargetID == row.TargetID && other.ApplicationID == row.ApplicationID && other.CreatedAt >= row.CreatedAt && other.Status != "failed" && other.Status != "cancelled" {
				return "", "", errors.New("A later release touched this application. Join the current checked release.")
			}
		}
		target, err := s.deploymentTarget(row.TargetID)
		if err != nil || targetFingerprint(target) != row.TargetFingerprint {
			return "", "", errors.New("The release's deployment target changed or disconnected.")
		}
		client, err := NewCoolifyClient(target.BaseURL, target.Token)
		if err != nil {
			return "", "", err
		}
		if err = verifyProjectRunCloudApplication(ctx, client, row); err != nil {
			return "", "", err
		}
		if err = s.recheckCloudApplicationData(ctx, row); err != nil {
			return "", "", err
		}
		observed, err := observeCloudRelease(ctx, row, client, checkPublicReleaseHealth)
		if err != nil || observed.Status != "ready" {
			return "", "", errors.New("The provider and public health have not freshly confirmed this exact release.")
		}
		return observed.URL, "Provider application identity, configured commit and public HTTPS health were observed. Full behavior and inbox delivery are not established.", nil
	}
	return "", "", errors.New("This project has no ready cloud release for the selected target and commit.")
}

// A historical successful deployment plus a responding URL cannot establish
// that the same application is still serving it. Query the owned application
// and fail closed if the provider cannot supply the current source identity.
func verifyProjectRunCloudApplication(ctx context.Context, client *CoolifyClient, row CloudRelease) error {
	if !coolifyID.MatchString(row.ApplicationID) || !coolifyID.MatchString(row.DeploymentID) {
		return errors.New("The release has no owned application and deployment identifiers.")
	}
	var app struct {
		UUID       string `json:"uuid"`
		Status     string `json:"status"`
		Repository string `json:"git_repository"`
		Branch     string `json:"git_branch"`
		Commit     string `json:"git_commit_sha"`
		Domain     string `json:"fqdn"`
	}
	if err := client.request(ctx, "GET", "/applications/"+row.ApplicationID, nil, nil, &app); err != nil {
		return errors.New("The release's current application ownership could not be observed.")
	}
	if app.UUID != row.ApplicationID || app.Repository != row.Repository || app.Branch != row.Branch || app.Commit != row.Release.Commit || strings.TrimSuffix(app.Domain, "/") != row.Domain || strings.Split(app.Status, ":")[0] != "running" {
		return errors.New("The provider's current application identity or configured source does not match this release.")
	}
	return nil
}
