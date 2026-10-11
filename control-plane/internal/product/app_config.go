package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const maxApplicationConfiguration = 256 << 10

var applicationVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// Snapshot values are private, runtime-only inputs. Never serialize a snapshot
// into a release, project, build command, log or API response.
type ApplicationConfigurationSnapshot struct {
	Revision int
	Values   map[string]string `json:"-"`
}

type applicationConfiguration struct {
	ProjectID string            `json:"projectId"`
	Revision  int               `json:"revision"`
	Values    map[string]string `json:"values"`
	UpdatedAt string            `json:"updatedAt"`
}

type ApplicationConfigurationMetadata struct {
	Revision  int                       `json:"revision"`
	Variables []ApplicationVariableName `json:"variables"`
	UpdatedAt string                    `json:"updatedAt"`
}
type ApplicationVariableName struct {
	Name string `json:"name"`
}
type applicationConfigurationUpdate struct {
	Name   string  `json:"name"`
	Value  *string `json:"value,omitempty"`
	Remove bool    `json:"remove,omitempty"`
}

func (s *Server) registerApplicationConfiguration(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/configuration", s.getApplicationConfiguration)
	mux.HandleFunc("POST /v1/product/projects/{id}/configuration", s.updateApplicationConfiguration)
}

// Flat state files avoid creating any directory on a metadata GET. File and
// parent resolution rejects symlinks; one-name checks reject hard-linked state.
func (s *Server) privateApplicationStatePath(name string) (string, error) {
	return privateApplicationStatePathForRoot(s.projectRoot(), s.stateDir, name)
}

// Callers already holding s.mu pass the protected configuration root directly;
// the filesystem checks themselves neither acquire the mutex nor create state.
func privateApplicationStatePathForRoot(projectsDir, stateDir, name string) (string, error) {
	if !coolifyID.MatchString(name) {
		return "", errors.New("invalid private application state identifier")
	}
	if _, err := hostpath.ResolveOutside(projectsDir, stateDir); err != nil {
		return "", errors.New("application configuration requires private state outside projects")
	}
	path, err := guardedProjectPath(stateDir, name+".json", true)
	if err != nil {
		return "", errors.New("application configuration state path is unsafe")
	}
	if _, err := os.Lstat(path); err == nil {
		if hostpath.OneName(path) != nil {
			return "", errors.New("application configuration requires a private file with one name")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("application configuration state is unavailable")
	}
	return path, nil
}

func validApplicationValues(values map[string]string) bool {
	if len(values) > 100 {
		return false
	}
	total := 0
	for name, value := range values {
		if !applicationVariableName.MatchString(name) || !utf8.ValidString(value) || len(value) > 16<<10 || strings.ContainsAny(value, "\x00\r\n") {
			return false
		}
		total += len(name) + len(value)
	}
	return total <= maxApplicationConfiguration
}

// Caller holds s.mu. Error messages never contain stored values or JSON text.
func readApplicationConfiguration(path, projectID string) (applicationConfiguration, error) {
	empty := applicationConfiguration{ProjectID: projectID, Values: map[string]string{}}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, errors.New("application configuration could not be read")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return empty, errors.New("application configuration exceeds its private state limit")
	}
	var config applicationConfiguration
	if json.Unmarshal(data, &config) != nil || config.ProjectID != projectID || config.Revision < 1 || config.Revision >= 1_000_000_000 || config.Values == nil || !validApplicationValues(config.Values) {
		return empty, errors.New("saved application configuration is invalid")
	}
	return config, nil
}

func applicationConfigurationMetadata(config applicationConfiguration) ApplicationConfigurationMetadata {
	metadata := ApplicationConfigurationMetadata{Revision: config.Revision, UpdatedAt: config.UpdatedAt, Variables: []ApplicationVariableName{}}
	for name := range config.Values {
		metadata.Variables = append(metadata.Variables, ApplicationVariableName{Name: name})
	}
	sort.Slice(metadata.Variables, func(i, j int) bool { return metadata.Variables[i].Name < metadata.Variables[j].Name })
	return metadata
}

func (s *Server) applicationConfigurationSnapshot(projectID string) (ApplicationConfigurationSnapshot, error) {
	if !coolifyID.MatchString(projectID) {
		return ApplicationConfigurationSnapshot{}, errors.New("invalid project identifier")
	}
	path, err := s.privateApplicationStatePath("application-config-" + projectID)
	if err != nil {
		return ApplicationConfigurationSnapshot{}, err
	}
	s.mu.RLock()
	config, err := readApplicationConfiguration(path, projectID)
	s.mu.RUnlock()
	if err != nil {
		return ApplicationConfigurationSnapshot{}, err
	}
	values := make(map[string]string, len(config.Values))
	for name, value := range config.Values {
		values[name] = value
	}
	return ApplicationConfigurationSnapshot{Revision: config.Revision, Values: values}, nil
}

func (s *Server) getApplicationConfiguration(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	path, err := s.privateApplicationStatePath("application-config-" + p.ID)
	if err != nil {
		journeyError(w, 500, "configuration_unavailable", err.Error())
		return
	}
	s.mu.RLock()
	config, err := readApplicationConfiguration(path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "configuration_unavailable", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	journeyJSON(w, 200, applicationConfigurationMetadata(config))
}

func (s *Server) updateApplicationConfiguration(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil || !utf8.Valid(data) {
		journeyError(w, 413, "invalid_configuration", "Send a bounded UTF-8 configuration update.")
		return
	}
	var request struct {
		ExpectedRevision *int                             `json:"expectedRevision"`
		Updates          []applicationConfigurationUpdate `json:"updates"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.ExpectedRevision == nil || *request.ExpectedRevision < 0 || len(request.Updates) == 0 || len(request.Updates) > 100 {
		journeyError(w, 400, "invalid_configuration", "Send an expected revision and 1–100 explicit variable updates.")
		return
	}
	seen := map[string]bool{}
	for _, update := range request.Updates {
		if !applicationVariableName.MatchString(update.Name) || seen[update.Name] || (update.Remove && update.Value != nil) || (!update.Remove && update.Value == nil) || (update.Value != nil && !validApplicationValues(map[string]string{update.Name: *update.Value})) {
			journeyError(w, 400, "invalid_configuration", "Use distinct environment names and either a single-line value of at most 16 KiB or an explicit removal.")
			return
		}
		seen[update.Name] = true
	}
	path, err := s.privateApplicationStatePath("application-config-" + p.ID)
	if err != nil {
		journeyError(w, 500, "configuration_unavailable", err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := readApplicationConfiguration(path, p.ID)
	if err != nil {
		journeyError(w, 500, "configuration_unavailable", err.Error())
		return
	}
	if config.Revision != *request.ExpectedRevision {
		journeyError(w, 409, "configuration_changed", "Application configuration changed. Refresh its names and revision before saving again.")
		return
	}
	changed := false
	for _, update := range request.Updates {
		before, exists := config.Values[update.Name]
		if update.Remove {
			delete(config.Values, update.Name)
			changed = changed || exists
		} else {
			config.Values[update.Name] = *update.Value
			changed = changed || !exists || before != *update.Value
		}
	}
	if !validApplicationValues(config.Values) {
		journeyError(w, 400, "configuration_limit", "Keep at most 100 variables and 256 KiB of names and values in this application configuration.")
		return
	}
	if changed {
		config.Revision++
		config.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if config.Revision >= 1_000_000_000 || atomicJSON(path, config) != nil {
			journeyError(w, 500, "configuration_save_failed", "Private application configuration could not be saved.")
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	journeyJSON(w, 200, applicationConfigurationMetadata(config))
}
