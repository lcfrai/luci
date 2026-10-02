package product

import (
	"archive/tar"
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
	"sort"
	"strconv"
	"strings"
	"time"
)

type Release struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"projectId"`
	BriefRevision         int    `json:"briefRevision"`
	ConfigurationRevision int    `json:"configurationRevision,omitempty"`
	Commit                string `json:"commit"`
	Target                string `json:"target"`
	Status                string `json:"status"`
	URL                   string `json:"url"`
	Message               string `json:"message"`
	CreatedAt             string `json:"createdAt"`
}

type storedRelease struct {
	Release
	ContainerID string `json:"containerId"`
	Image       string `json:"image"`
}

func (s *Server) registerReleases(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/releases", s.listProjectReleases)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases", s.createLocalRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/{releaseId}/stop", s.stopLocalRelease)
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
	for _, row := range rows {
		result = append(result, row.Release)
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
		Target     string `json:"target"`
		Port       int    `json:"port"`
		Dockerfile string `json:"dockerfile"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.Target != "local" {
		journeyError(w, 400, "unsupported_target", "Choose local preview. Cloud publishing is a separate explicit deployment flow.")
		return
	}
	if req.Port == 0 {
		req.Port = 3000
	}
	if req.Dockerfile == "" {
		req.Dockerfile = "Dockerfile"
	}
	if req.Port < 1 || req.Port > 65535 || blockedProjectFile(req.Dockerfile) {
		journeyError(w, 400, "invalid_preview", "Use a container port from 1 to 65535 and a project Dockerfile.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if _, err := guardedProjectPath(p.Path, req.Dockerfile, false); err != nil {
		journeyError(w, 400, "dockerfile_unavailable", err.Error())
		return
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
	s.mu.RLock()
	state, err := readJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "history_unavailable", err.Error())
		return
	}
	if state.Brief.Revision > 0 {
		committedBrief, briefErr := s.journeyGit(ctx, p.Path, "show", commit+":LUCI.md")
		if briefErr != nil || committedBrief != string(renderBrief(state.Brief)) {
			journeyError(w, 409, "brief_commit_required", "Commit the current LUCI.md before previewing. The release must bind the brief that is actually in its commit.")
			return
		}
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
	row := storedRelease{Release: Release{ID: newID("rel-"), ProjectID: p.ID, BriefRevision: state.Brief.Revision, ConfigurationRevision: configuration.Revision, Commit: commit, Target: "local", Status: "building", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	row.Image = "luci/preview:" + row.ID
	if err := s.storeRelease(row); err != nil {
		journeyError(w, 500, "release_reservation_failed", err.Error())
		return
	}
	failed := func(message string) {
		message = redactPreviewConfiguration(message, configuration.Values)
		row.Status = "failed"
		row.Message = message
		row.URL = ""
		if row.ContainerID != "" {
			cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = s.runner(cleanup, "docker", []string{"rm", "--force", row.ContainerID}, "")
			c()
		}
		if err := s.storeRelease(row); err != nil {
			journeyError(w, 500, "release_record_failed", message+"; also could not save the release: "+err.Error())
			return
		}
		journeyJSON(w, 422, row.Release)
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
	dockerfile, err := guardedProjectPath(contextDir, req.Dockerfile, false)
	if err != nil {
		failed("The chosen Dockerfile is missing from the committed snapshot: " + err.Error())
		return
	}
	if _, err := s.runner(ctx, "docker", []string{"build", "--tag", row.Image, "--label", "luci.preview=true", "--file", dockerfile, contextDir}, contextDir); err != nil {
		failed("Image build failed: " + err.Error())
		return
	}
	args := []string{"run", "--detach", "--init", "--name", "luci-preview-" + row.ID, "--label", "luci.preview=true", "--label", "luci.release-id=" + row.ID, "--label", "luci.project-id=" + p.ID, "--cpus", "1", "--memory", "1g", "--pids-limit", "256", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--publish", fmt.Sprintf("127.0.0.1::%d", req.Port)}
	envFile, err := s.writePreviewEnvironment(configuration.Values)
	if err != nil {
		failed("Could not prepare private application runtime configuration.")
		return
	}
	if envFile != "" {
		defer os.Remove(envFile)
		args = append(args, "--env-file", envFile)
	}
	args = append(args, row.Image)
	containerID, err := s.runner(ctx, "docker", args, "")
	var cleanupErr error
	if envFile != "" {
		cleanupErr = os.Remove(envFile)
	}
	if err != nil {
		failed("Container start failed: " + err.Error())
		return
	}
	row.ContainerID = strings.TrimSpace(containerID)
	if !regexp.MustCompile(`^[a-f0-9]{12,64}$`).MatchString(row.ContainerID) {
		row.ContainerID = ""
		failed("Docker returned an invalid container ID; inspect Docker for this release's labelled container.")
		return
	}
	if cleanupErr != nil {
		failed("Could not remove the private application environment file after Docker read it.")
		return
	}
	if err := s.storeRelease(row); err != nil {
		failed("Could not record the started container: " + err.Error())
		return
	}
	mapping, err := s.runner(ctx, "docker", []string{"port", row.ContainerID, strconv.Itoa(req.Port) + "/tcp"}, "")
	if err != nil {
		failed("Could not discover the local preview port: " + err.Error())
		return
	}
	url, err := localPreviewURL(mapping)
	if err != nil {
		failed(err.Error())
		return
	}
	healthCtx, healthCancel := context.WithTimeout(ctx, 20*time.Second)
	err = waitPreviewHealthy(healthCtx, url)
	healthCancel()
	if err != nil {
		failed("The container did not answer a successful HTTP health check: " + err.Error())
		return
	}
	row.Status = "ready"
	row.URL = url
	row.Message = "Local preview returned a successful HTTP response. Bound to this computer only."
	if err := s.storeRelease(row); err != nil {
		failed("Health passed but its release record could not be saved: " + err.Error())
		return
	}
	journeyJSON(w, 201, row.Release)
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
	args := append(append([]string{}, journeyGitFlags...), "archive", "--format=tar", commit)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
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
		target := filepath.Join(destination, filepath.FromSlash(name))
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if _, err := guardedProjectPath(destination, name, true); err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if header.Mode&0111 != 0 {
			mode = 0700
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(file, archive, header.Size)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}
