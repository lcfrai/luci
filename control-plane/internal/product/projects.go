package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const journeyMaxBody = 2 << 20

var projectNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func (s *Server) registerProjects(mux Registrar) {
	s.registerProjectBranches(mux)
	s.registerOrchestrator(mux)
	mux = s.guardProjectSourceRoutes(mux)
	s.registerJourneyContext(mux)
	s.registerSpecification(mux)
	s.registerDesignPlan(mux)
	s.registerReleases(mux)
	mux.HandleFunc("POST /v1/product/projects", s.createProductProject)
	mux.HandleFunc("GET /v1/product/projects/{id}/brief", s.getBrief)
	mux.HandleFunc("GET /v1/product/projects/{id}/context-history", s.contextHistory)
	mux.HandleFunc("POST /v1/product/projects/{id}/brief", s.saveBrief)
	mux.HandleFunc("GET /v1/product/projects/{id}/design", s.getDesign)
	mux.HandleFunc("POST /v1/product/projects/{id}/design", s.saveDesign)
	mux.HandleFunc("GET /v1/product/projects/{id}/design/export", s.exportDesign)
	mux.HandleFunc("GET /v1/product/projects/{id}/files", s.projectFiles)
	mux.HandleFunc("GET /v1/product/projects/{id}/git", s.projectGit)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/commit", s.commitProject)
}

func journeyJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func journeyError(w http.ResponseWriter, status int, code, message string) {
	journeyJSON(w, status, map[string]string{"error": code, "message": message})
}

func journeyDecode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, journeyMaxBody)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		journeyError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON request: "+err.Error())
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		journeyError(w, http.StatusBadRequest, "invalid_request", "Send exactly one JSON object.")
		return false
	}
	return true
}

func validProjectName(name string) bool {
	if !projectNamePattern.MatchString(name) {
		return false
	}
	upper := strings.ToUpper(name)
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" {
		return false
	}
	return !regexp.MustCompile(`^(COM|LPT)[0-9]$`).MatchString(upper)
}

func (s *Server) createProductProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		RemoteURL string `json:"remoteUrl"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validProjectName(req.Name) {
		journeyError(w, 400, "invalid_name", "Use 1–64 letters, numbers, hyphens or underscores, starting with a letter or number.")
		return
	}
	if req.RemoteURL != "" {
		u, err := url.Parse(req.RemoteURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(req.RemoteURL, "\r\n\x00") {
			journeyError(w, 400, "invalid_remote", "Use an HTTPS Git URL without embedded credentials, a query or a fragment.")
			return
		}
	}
	root := s.projectRoot()
	if root == "" || !filepath.IsAbs(root) {
		journeyError(w, 409, "setup_required", "Choose an absolute projects folder in setup first.")
		return
	}
	// No shared server state changes here. Mkdir reserves this project while
	// a potentially slow clone runs, so setup/status reads remain available.
	if err := os.MkdirAll(root, 0o700); err != nil {
		journeyError(w, 500, "projects_unavailable", err.Error())
		return
	}
	dir := filepath.Join(root, req.Name)
	// Mkdir is the reservation: it cannot overwrite a directory or a symlink.
	if err := os.Mkdir(dir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			journeyError(w, 409, "project_exists", "A file or project with that name already exists.")
		} else {
			journeyError(w, 500, "create_failed", err.Error())
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	args := []string{"init", "--initial-branch=main", "."}
	if req.RemoteURL != "" {
		args = []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "core.hooksPath=/dev/null", "clone", "--", req.RemoteURL, "."}
		if runtime.GOOS == "windows" {
			args = []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "core.hooksPath=/dev/null", "clone", "-c", "core.longpaths=true", "--", req.RemoteURL, "."}
		}
	}
	if _, err := s.runner(ctx, "git", args, dir); err != nil {
		// Keep a partial clone for inspection. Never recursively delete a path
		// that an external process might have replaced while Git was running.
		journeyError(w, 422, "git_failed", fmt.Sprintf("Git could not create the project. Its folder was retained at %s: %v", dir, err))
		return
	}
	if runtime.GOOS == "windows" && req.RemoteURL == "" {
		// Git for Windows can cache long-path support before command-line
		// overrides are applied. Initialize this owned repository's setting
		// for later Git children as well; never modify global configuration.
		if _, err := s.runner(ctx, "git", []string{"config", "--local", "core.longpaths", "true"}, dir); err != nil {
			journeyError(w, 422, "git_failed", fmt.Sprintf("Git could not configure the project. Its folder was retained at %s: %v", dir, err))
			return
		}
	}
	journeyJSON(w, http.StatusCreated, project.Project{ID: project.ProjectID(dir), Name: req.Name, Path: dir})
}

// resolveProject uses the existing catalogue's path-derived identifier, and
// searches only immediate repositories under the configured projects folder.
func (s *Server) resolveProject(id string) (project.Project, error) {
	root := s.projectRoot()
	if root == "" || !filepath.IsAbs(root) {
		return project.Project{}, project.ErrUnknownProject
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return project.Project{}, err
	}
	for _, entry := range entries {
		dir := filepath.Join(root, entry.Name())
		if strings.HasPrefix(entry.Name(), ".") || project.ProjectID(dir) != id {
			continue
		}
		if _, err := hostpath.ConfineToMounts([]string{root}, dir); err != nil {
			return project.Project{}, err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return project.Project{}, project.ErrUnknownProject
		}
		gitDir, err := guardedProjectPath(dir, ".git", false)
		if err != nil {
			return project.Project{}, err
		}
		gitInfo, err := os.Lstat(gitDir)
		if err != nil || !gitInfo.IsDir() {
			return project.Project{}, errors.New("the project must have its own Git directory; linked worktrees are managed through Develop")
		}
		return project.Project{ID: id, Name: entry.Name(), Path: dir}, nil
	}
	return project.Project{}, project.ErrUnknownProject
}

func (s *Server) requestProject(w http.ResponseWriter, r *http.Request) (project.Project, bool) {
	p, err := s.resolveProject(r.PathValue("id"))
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, project.ErrUnknownProject) || errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		journeyError(w, status, "project_unavailable", err.Error())
		return project.Project{}, false
	}
	return p, true
}

// guardedProjectPath refuses traversal and symlinks/junctions at every existing
// component, including a missing leaf's parent. Paths use portable '/' syntax.
func guardedProjectPath(root, relative string, missingLeaf bool) (string, error) {
	if relative == "" || relative == "." {
		return hostpath.ConfineToMounts([]string{root}, root)
	}
	if filepath.IsAbs(relative) || strings.ContainsAny(relative, "\\:\x00") {
		return "", errors.New("use a relative path inside the project")
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part {
			return "", errors.New("path traversal and ambiguous path components are refused")
		}
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	checked := path
	if missingLeaf {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			checked = filepath.Dir(path)
		}
	}
	if _, err := hostpath.ConfineToMounts([]string{root}, checked); err != nil {
		return "", err
	}
	return path, nil
}

func writeProjectAtomic(root, relative string, data []byte) error {
	target, err := guardedProjectPath(root, relative, true)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".luci-write-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for attempt := 0; attempt < 5; attempt++ {
		if _, err = guardedProjectPath(root, relative, true); err != nil {
			return err
		}
		if err = os.Rename(tmp, target); err == nil {
			return nil
		}
		if attempt < 4 {
			time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
		}
	}
	return err
}
