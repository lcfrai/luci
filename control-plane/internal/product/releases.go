package product

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/gitcommand"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type Release struct {
	Kind                      string `json:"kind,omitempty"`
	Revision                  uint64 `json:"revision,omitempty"`
	CanCancel                 bool   `json:"canCancel,omitempty"`
	CancelRequested           bool   `json:"cancelRequested,omitempty"`
	CanStop                   bool   `json:"canStop"`
	ID                        string `json:"id"`
	ProjectID                 string `json:"projectId"`
	BriefRevision             int    `json:"briefRevision"`
	ConfigurationRevision     int    `json:"configurationRevision,omitempty"`
	Commit                    string `json:"commit"`
	Target                    string `json:"target"`
	Status                    string `json:"status"`
	URL                       string `json:"url"`
	Message                   string `json:"message"`
	CreatedAt                 string `json:"createdAt"`
	BrowserConnectionRequired bool   `json:"browserConnectionRequired,omitempty"`
}

type storedRelease struct {
	CoordinatorOriginDigest string `json:"coordinatorOriginDigest,omitempty"`

	Release
	CancelRevision uint64                  `json:"cancelRevision,omitempty"`
	Runtime        *previewruntime.Outcome `json:"runtime,omitempty"`
	ContainerID    string                  `json:"containerId"`
	Image          string                  `json:"image"`
	operation      *localPreviewOperation
}

func (s *Server) registerReleases(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/releases", s.listProjectReleases)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases", s.createLocalRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/{releaseId}/stop", s.stopLocalRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/{releaseId}/cancel", s.cancelLocalRelease)
}

func (s *Server) releaseFile(projectID string) string {
	return filepath.Join(s.stateDir, "releases", projectID+".json")
}

func (s *Server) readReleases(projectID string) ([]storedRelease, error) {
	file, err := os.Open(s.releaseFile(projectID))
	if errors.Is(err, os.ErrNotExist) {
		return []storedRelease{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, errors.New("release history exceeds 8 MiB")
	}
	var rows []storedRelease
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ProjectID != projectID {
			return nil, errors.New("release history belongs to a different project")
		}
	}
	if rows == nil {
		rows = []storedRelease{}
	}
	return rows, nil
}

func (s *Server) storeRelease(row storedRelease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeReleaseLocked(row)
}

func (s *Server) storeReleaseLocked(row storedRelease) error {
	rows, err := s.readReleases(row.ProjectID)
	if err != nil {
		return err
	}
	found := false
	for i := range rows {
		if rows[i].ID == row.ID {
			rows[i] = row
			found = true
			break
		}
	}
	if !found {
		rows = append([]storedRelease{row}, rows...)
	}
	if len(rows) > 5000 {
		return errors.New("release history is full; archive old release records before creating more")
	}
	return atomicJSON(s.releaseFile(row.ProjectID), rows)
}

func (s *Server) listProjectReleases(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	rows, err := s.readReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "releases_unavailable", err.Error())
		return
	}
	result := []Release{}
	for _, row := range s.observeLocalReleases(r.Context(), rows) {
		result = append(result, s.browserRelease(row))
	}
	cloud, err := s.cloudReleases(p.ID)
	if err != nil {
		journeyError(w, 500, "releases_unavailable", err.Error())
		return
	}
	result = append(result, cloud...)
	pages, err := s.pagesReleases(p.ID)
	if err != nil {
		journeyError(w, 500, "releases_unavailable", "Pages history could not be read.")
		return
	}
	result = append(result, pages...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	journeyJSON(w, 200, map[string]any{"releases": result})
}

func (s *Server) createLocalRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind           string `json:"kind,omitempty"`
		ExpectedCommit string `json:"expectedCommit,omitempty"`
		Target         string `json:"target"`
		Port           int    `json:"port"`
		Dockerfile     string `json:"dockerfile"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedCommit != "" && !deliverySHA.MatchString(req.ExpectedCommit) {
		journeyError(w, 400, "invalid_commit", "Use the exact full commit saved with the checked build.")
		return
	}
	if req.Target != "local" {
		journeyError(w, 400, "unsupported_target", "Choose local preview. Cloud publishing is a separate explicit deployment flow.")
		return
	}
	if req.Port == 0 {
		req.Port = 3000
	}
	if req.Kind != "" && req.Kind != previewruntime.ReviewedSourceKind {
		journeyError(w, 400, "unsupported_preview_kind", "Choose a Dockerfile preview or an explicit reviewed-source static preview.")
		return
	}
	if req.Kind == previewruntime.ReviewedSourceKind && req.ExpectedCommit == "" {
		journeyError(w, 400, "invalid_commit", "Choose the exact full reviewed and checked commit for this source preview.")
		return
	}
	if req.Kind == previewruntime.ReviewedSourceKind && req.Dockerfile != "" {
		journeyError(w, 400, "invalid_preview", "Reviewed-source previews serve static files without building a Dockerfile.")
		return
	}
	if req.Kind == "" && req.Dockerfile == "" {
		req.Dockerfile = "Dockerfile"
	}
	if req.Port < 1 || req.Port > 65535 || req.Kind == "" && blockedProjectFile(req.Dockerfile) {
		journeyError(w, 400, "invalid_preview", "Use a container port from 1 to 65535 and a project Dockerfile.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	runtime := s.PreviewRuntime()
	if runtime.CaptureActor == nil || runtime.Start == nil || runtime.Observe == nil || runtime.Stop == nil {
		journeyError(w, 503, "preview_runtime_unavailable", "Shared preview admission is unavailable. No build started.")
		return
	}
	actor, err := runtime.CaptureActor(r.Context(), p.ID)
	if err != nil {
		journeyError(w, 403, "deployment_forbidden", err.Error())
		return
	}
	if req.Kind == "" {
		if _, err := guardedProjectPath(p.Path, req.Dockerfile, false); err != nil {
			journeyError(w, 400, "dockerfile_unavailable", err.Error())
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	status, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		journeyError(w, 422, "git_failed", err.Error())
		return
	}
	if status != "" {
		journeyError(w, 409, "commit_required", "Commit all changes before previewing so the release identifies exactly what was built.")
		return
	}
	commit, err := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	commit = strings.TrimSpace(commit)
	if err != nil || !regexp.MustCompile(`^[a-f0-9]{40,64}$`).MatchString(commit) {
		journeyError(w, 409, "commit_required", "Create a commit before previewing this project.")
		return
	}
	if req.ExpectedCommit != "" && req.ExpectedCommit != commit {
		journeyError(w, 409, "source_changed", "The project commit changed after this build was checked. Return to the build results and verify the intended source before previewing.")
		return
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "history_unavailable", err.Error())
		return
	}
	if err := s.validateReviewedCommit(ctx, p, state, commit); err != nil {
		problem(w, 409, "review_approval_required", err.Error())
		return
	}
	configuration, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil {
		journeyError(w, 500, "configuration_unavailable", "Could not read private application configuration.")
		return
	}
	if _, err := previewEnvironmentContent(configuration.Values); err != nil {
		journeyError(w, 422, "configuration_invalid", err.Error())
		return
	}
	if req.Kind == previewruntime.ReviewedSourceKind {
		if len(configuration.Values) != 0 {
			journeyError(w, 422, "static_preview_configuration_unsupported", "Reviewed-source previews serve browser assets without private application environment values.")
			return
		}
		// The existing coordinator approval binds a Dockerfile, not a serving
		// kind. Never silently reinterpret it as authority for another profile.
		if _, internal := ctx.Value(coordinatorPreviewContextKey{}).(coordinatorPreviewOrigin); internal {
			journeyError(w, 409, "coordinator_preview_kind_unbound", "The saved automation approval does not bind this preview kind. Review an explicit source-preview policy before automating it.")
			return
		}
	}
	reservedID, originDigest, existing, err := s.admitCoordinatorPreview(ctx, p, commit, configuration.Revision, req.Port, req.Dockerfile)
	if err != nil {
		journeyError(w, 409, "coordinator_preview_changed", "The reserved preview no longer matches current checked source, authority, policy or configuration.")
		return
	}
	if existing != nil {
		journeyJSON(w, 200, s.browserRelease(*existing))
		return
	}
	if reservedID == "" {
		reservedID = newID("rel-")
	}
	row := storedRelease{CoordinatorOriginDigest: originDigest, Release: Release{Kind: req.Kind, ID: reservedID, ProjectID: p.ID, BriefRevision: state.Brief.Revision, ConfigurationRevision: configuration.Revision, Commit: commit, Target: "local", Status: "building", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	if err := s.RecordProjectRunReleaseProvenance(ctx, p.ID, row.ID, row.Target, row.Commit); err != nil {
		problem(w, 409, "release_provenance_unavailable", err.Error())
		return
	}
	digest, err := s.PreviewSourceDigest(ctx, p.ID, row.ID, commit)
	if err != nil {
		journeyError(w, 409, "source_provenance_unavailable", err.Error())
		return
	}
	row.Revision = 1
	row.Runtime = &previewruntime.Outcome{Identity: previewruntime.Identity{Kind: req.Kind, ProjectID: p.ID, ReleaseID: row.ID, Target: row.Target, Commit: commit, SourceDigest: digest}, Phase: "reserved"}
	operation, err := s.beginLocalPreview(&row, cancel)
	if err != nil {
		journeyError(w, 500, "release_reservation_failed", err.Error())
		return
	}
	defer s.finishLocalPreview(row.ID, operation)
	failed := func(message string) {
		next := row
		next.Status, next.URL, next.Message = "failed", "", redactPreviewConfiguration(message, configuration.Values)
		if row.Runtime.ResourcesHeld {
			next.Status = "unknown"
		}
		if err := s.storeAdmittedRelease(row, &next); err != nil {
			journeyError(w, 409, "release_record_changed", "The preview result could not replace its retained receipt.")
			return
		}
		row = next
		journeyJSON(w, 422, s.browserRelease(row))
	}
	contextDir, err := os.MkdirTemp("", "luci-preview-*")
	if err != nil {
		failed(err.Error())
		return
	}
	defer os.RemoveAll(contextDir)
	if err := archiveCommittedProject(ctx, p.Path, commit, contextDir); err != nil {
		failed("Could not prepare a safe committed build context: " + err.Error())
		return
	}
	if req.Kind == "" {
		_, err = guardedProjectPath(contextDir, req.Dockerfile, false)
		if err != nil {
			failed("The chosen Dockerfile is missing from the committed snapshot: " + err.Error())
			return
		}
	}
	var sourceIndex []byte
	if req.Kind == previewruntime.ReviewedSourceKind {
		reader, err := os.Open(filepath.Join(contextDir, "index.html"))
		if err != nil {
			failed("The reviewed-source snapshot requires an index.html entrypoint.")
			return
		}
		sourceIndex, err = io.ReadAll(io.LimitReader(reader, (64<<10)+1))
		closeErr := reader.Close()
		if err != nil || closeErr != nil || len(sourceIndex) > 64<<10 {
			failed("The reviewed entrypoint could not be captured within the static asset limit.")
			return
		}
	}
	request := previewruntime.Request{Identity: row.Runtime.Identity, SourceDirectory: contextDir, Dockerfile: req.Dockerfile, Port: req.Port, Environment: configuration.Values, Actor: actor, Network: "bridge",
		Labels: map[string]string{"luci.release-id": row.ID, "luci.project-id": p.ID},
		ValidateSource: func(verify context.Context) error {
			if err := s.ValidateDeploymentSource(verify, p.ID, commit); err != nil {
				return err
			}
			if req.Kind == previewruntime.ReviewedSourceKind {
				current, err := s.applicationConfigurationSnapshot(p.ID)
				if err != nil || current.Revision != configuration.Revision || len(current.Values) != 0 {
					return errors.New("reviewed-source configuration changed")
				}
			}
			// Retain the trusted origin even when the runtime installs freshly
			// reauthorized actor context for a subsequent allocating effect.
			if origin, internal := ctx.Value(coordinatorPreviewContextKey{}).(coordinatorPreviewOrigin); internal {
				verify = context.WithValue(verify, coordinatorPreviewContextKey{}, origin)
			}
			return s.validateCoordinatorPreviewMutation(verify, p)
		},
	}
	out, startErr := runtime.Start(ctx, request, func(value previewruntime.Outcome) error {
		return s.publishPreviewOutcome(&row, value, configuration.Values)
	})
	if startErr != nil {
		message := row.Message
		if message == "" {
			message = "The admitted preview did not finish. Any uncertain allocation is retained; refresh or stop this exact preview."
		}
		failed(message)
		return
	}
	if err := s.publishPreviewOutcome(&row, out, configuration.Values); err != nil {
		journeyError(w, 409, "release_record_changed", err.Error())
		return
	}
	if out.Phase != "running" || !out.ResourcesHeld || out.ContainerID == "" {
		failed("The preview runtime has no verified running application; its receipt is retained.")
		return
	}
	if _, err := recordedPreviewOrigin(out.URL); err != nil {
		failed("The admitted preview has no verified local origin.")
		return
	}
	url := out.URL
	healthCtx, healthCancel := context.WithTimeout(ctx, 20*time.Second)
	if req.Kind == previewruntime.ReviewedSourceKind {
		err = waitReviewedSourceHealthy(healthCtx, url, sourceIndex, out.SourceDigest, out.SnapshotDigest)
	} else {
		err = waitPreviewHealthy(healthCtx, url)
	}
	healthCancel()
	if err != nil {
		if req.Kind == previewruntime.ReviewedSourceKind {
			failed("The source preview did not return the exact reviewed entrypoint and snapshot receipt. Its allocation remains retained for Stop.")
		} else {
			failed("The container did not answer a successful HTTP health check: " + err.Error())
		}
		return
	}
	s.completeLocalPreview(w, &row, url)
}

func (s *Server) completeLocalPreview(w http.ResponseWriter, row *storedRelease, url string) {
	next := *row
	next.Status, next.URL = "ready", url
	next.Message = "Local preview returned a successful HTTP response. Bound to this computer only."
	if err := s.storeAdmittedRelease(*row, &next); err != nil {
		journeyError(w, 409, "release_record_changed", "The ready result could not replace its retained receipt.")
		return
	}
	*row = next
	if row.Status != "ready" {
		journeyJSON(w, 422, s.browserRelease(*row))
		return
	}
	journeyJSON(w, 201, s.browserRelease(*row))
}

func localPreviewURL(mapping string) (string, error) {
	lines := strings.Split(strings.TrimSpace(mapping), "\n")
	if len(lines) != 1 {
		return "", errors.New("Docker must publish exactly one localhost address")
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(lines[0]))
	if err != nil || host != "127.0.0.1" {
		return "", errors.New("Docker did not bind the preview exclusively to 127.0.0.1")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("Docker returned an invalid preview port")
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func waitPreviewHealthy(ctx context.Context, url string) error {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Static readiness proves the reviewed entrypoint bytes and both distinct
// provenance digests. A generic 2xx or another process at the origin is not a
// reviewed-source readiness receipt. This never confirms resource settlement.
func reviewedSourceResponseMatches(response *http.Response, index []byte, sourceDigest, snapshotDigest string) bool {
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Luci-Source-Digest") != sourceDigest || response.Header.Get("X-Luci-Snapshot-Digest") != snapshotDigest || response.Header.Get("Content-Type") != "text/html; charset=utf-8" || sourceDigest == "" || snapshotDigest == "" || len(index) > 64<<10 {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	return err == nil && len(data) <= 64<<10 && bytes.Equal(data, index)
}

func waitReviewedSourceHealthy(ctx context.Context, url string, index []byte, sourceDigest, snapshotDigest string) error {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil, MaxResponseHeaderBytes: 8192}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err == nil {
			matches := reviewedSourceResponseMatches(response, index, sourceDigest, snapshotDigest)
			_ = response.Body.Close()
			if matches {
				return nil
			}
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *Server) stopLocalRelease(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.PathValue("releaseId"), "pages-") {
		s.pagesReleaseOperation(w, r, true)
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	rows, err := s.readReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "releases_unavailable", err.Error())
		return
	}
	var row storedRelease
	found := false
	for _, candidate := range rows {
		if candidate.ID == r.PathValue("releaseId") {
			row = candidate
			found = true
			break
		}
	}
	if !found {
		s.stopCloudRelease(w, r)
		return
	}
	if row.Runtime != nil {
		s.stopAdmittedRelease(w, r, row)
		return
	}
	if row.Status == "stopped" {
		journeyJSON(w, 200, row.Release)
		return
	}
	if row.Target != "local" || !regexp.MustCompile(`^[a-f0-9]{12,64}$`).MatchString(row.ContainerID) {
		journeyError(w, 409, "preview_not_running", "This release has no recorded local container to stop.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	labels, err := s.runner(ctx, "docker", []string{"inspect", "--format", "{{json .Config.Labels}}", row.ContainerID}, "")
	if err != nil {
		message := strings.ToLower(err.Error())
		if !strings.Contains(message, "no such object") && !strings.Contains(message, "no such container") {
			journeyError(w, 503, "docker_unavailable", err.Error())
			return
		}
	} else {
		var owned map[string]string
		if json.Unmarshal([]byte(labels), &owned) != nil || owned["luci.preview"] != "true" || owned["luci.release-id"] != row.ID || owned["luci.project-id"] != p.ID {
			journeyError(w, 409, "container_ownership_mismatch", "The recorded container's Luci ownership labels do not match. Nothing was stopped.")
			return
		}
		if _, err := s.runner(ctx, "docker", []string{"rm", "--force", row.ContainerID}, ""); err != nil {
			journeyError(w, 503, "stop_failed", err.Error())
			return
		}
	}
	row.Status = "stopped"
	row.URL = ""
	row.Message = "Local preview stopped. The release and its provenance are retained."
	if err := s.storeRelease(row); err != nil {
		journeyError(w, 500, "release_record_failed", err.Error())
		return
	}
	journeyJSON(w, 200, row.Release)
}

// archiveCommittedProject bounds the Git archive pipe before extraction, so a
// repository cannot fill the host disk through an oversized build context.
// Docker still applies the snapshot's .dockerignore to this sanitized context.
func archiveCommittedProject(ctx context.Context, dir, commit, destination string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// Releases archive the primary project. Its Git storage must stay inside
	// that project, just as the source inspected before deployment does.
	bounded, err := journeyGitBoundary(ctx, dir, dir, []string{"archive", "--format=tar", commit})
	if err != nil {
		return err
	}
	args := append(append([]string{}, journeyGitFlags...), bounded...)
	cmd := exec.CommandContext(ctx, "git", gitcommand.PlatformArgs(dir, args)...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = project.ManagedGitEnvironment(os.Environ())
	stderr := &limitedBuffer{limit: 64 << 10}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	waited := false
	defer func() {
		if !waited {
			cancel()
			_ = stdout.Close()
			_ = cmd.Wait()
		}
	}()
	limited := &io.LimitedReader{R: stdout, N: 100<<20 + 1}
	if err := extractPreviewArchive(limited, destination); err != nil {
		return err
	}
	// tar stops after its two zero blocks, before Git's block padding. Drain
	// that bounded padding before Wait so Git cannot block on a full pipe.
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return err
	}
	if limited.N == 0 {
		return errors.New("the build context exceeds 100 MiB")
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		return fmt.Errorf("git archive: %w: %s", err, stderr.String())
	}
	return nil
}

func extractPreviewArchive(reader io.Reader, destination string) error {
	root, err := os.Lstat(destination)
	if err != nil || !root.IsDir() || root.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("the build context requires an ordinary private staging directory")
	}
	if runtime.GOOS != "windows" && root.Mode().Perm() != 0700 {
		return errors.New("the build context staging directory must remain private")
	}
	archive := tar.NewReader(reader)
	total := int64(0)
	files := 0
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// git archive emits a global PAX header carrying its commit ID. It is
		// metadata, never a filesystem entry; each real entry is checked below.
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" {
			continue
		}
		if strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
			return errors.New("the committed tree contains an unsafe archive path")
		}
		for _, component := range strings.Split(name, "/") {
			if component == "" || component == "." || component == ".." || strings.TrimRight(component, ". ") != component {
				return errors.New("the committed tree contains an unsafe archive component")
			}
		}
		files++
		if files > 10000 {
			return errors.New("the build context exceeds 10,000 entries")
		}
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("build contexts cannot contain symlinks, hard links or special files: %s", name)
		}
		if blockedProjectFile(name) {
			continue
		}
		if header.Size < 0 {
			return errors.New("archive entry has a negative size")
		}
		total += header.Size
		if total > 100<<20 {
			return errors.New("the build context exceeds 100 MiB")
		}
		if header.Typeflag == tar.TypeDir {
			if err := previewArchiveDirectory(destination, name); err != nil {
				return err
			}
			continue
		}
		if err := previewArchiveDirectory(destination, filepath.ToSlash(filepath.Dir(filepath.FromSlash(name)))); err != nil {
			return err
		}
		target, err := guardedProjectPath(destination, name, true)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if header.Mode&0111 != 0 {
			mode = 0755
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(file, archive, header.Size)
		// The enclosing MkdirTemp directory stays private. Files inside must
		// retain canonical Git readability/executability when Docker COPY
		// gives them to a non-root application, including under umask 077.
		if copyErr == nil {
			copyErr = file.Chmod(mode)
		}
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

// Create and check each ancestor before changing its permissions. MkdirAll
// alone would follow a pre-existing directory symlink, and a later directory
// header could otherwise chmod a path outside this private build context.
func previewArchiveDirectory(root, relative string) error {
	if relative == "." || relative == "" {
		return nil // Never broaden the private outer staging directory.
	}
	var partial string
	for _, component := range strings.Split(relative, "/") {
		if partial == "" {
			partial = component
		} else {
			partial += "/" + component
		}
		directory, err := guardedProjectPath(root, partial, true)
		if err != nil {
			return err
		}
		if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("the build context contains an unsafe directory")
		}
		if _, err := guardedProjectPath(root, partial, false); err != nil {
			return err
		}
		opened, err := os.Open(directory)
		if err != nil {
			return err
		}
		actual, statErr := opened.Stat()
		if statErr != nil || !os.SameFile(info, actual) {
			_ = opened.Close()
			return errors.New("the build context directory changed during extraction")
		}
		guardErr := func() error {
			if _, err := guardedProjectPath(root, partial, false); err != nil {
				return err
			}
			return opened.Chmod(0755)
		}()
		closeErr := opened.Close()
		if guardErr != nil {
			return guardErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
