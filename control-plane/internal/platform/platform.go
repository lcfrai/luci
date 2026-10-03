// Package platform operates owned persistent application resources and an
// optional external Panacea control plane. It contains no Panacea source.
package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type Options struct {
	StateDir, InstanceID, DockerHost string
	ResolveProject                   func(string) (project.Project, error)
	ApplicationConfiguration         func(string) (int, map[string]string, error)
	ValidateDeploymentSource         func(context.Context, string, string) error
	MemoryMiB, CPUMillis             int
	Runner                           Runner
	HTTPClient                       *http.Client
}
type Deployment struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"projectId"`
	Provider              string `json:"provider"`
	Commit                string `json:"commit"`
	ConfigurationRevision int    `json:"configurationRevision"`
	Status                string `json:"status"`
	URL                   string `json:"url,omitempty"`
	Message               string `json:"message"`
	Image                 string `json:"image,omitempty"`
	Container             string `json:"container,omitempty"`
	Port                  int    `json:"port,omitempty"`
	HealthPath            string `json:"healthPath,omitempty"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
	RemoteProject         string `json:"remoteProject,omitempty"`
	PipelineRun           string `json:"pipelineRun,omitempty"`
	ProviderBinding       string `json:"providerBinding,omitempty"`
}
type Database struct {
	Status    string `json:"status"`
	Engine    string `json:"engine"`
	Container string `json:"container,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Database  string `json:"database,omitempty"`
	Username  string `json:"username,omitempty"`
	Message   string `json:"message"`
}
type Backup struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
}
type ProjectState struct {
	ProjectID      string       `json:"projectId"`
	Deployments    []Deployment `json:"deployments"`
	Database       Database     `json:"database"`
	Backups        []Backup     `json:"backups"`
	PanaceaProject string       `json:"panaceaProject,omitempty"`
}
type Service struct {
	opts          Options
	dir, instance string
	mu            sync.Mutex
	busy          map[string]bool
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var commitID = regexp.MustCompile(`^[a-f0-9]{40}$`)

func now() string { return time.Now().UTC().Format(time.RFC3339) }
func randomID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b)
}
func New(opts Options) (*Service, error) {
	if !filepath.IsAbs(opts.StateDir) || opts.ResolveProject == nil {
		return nil, errors.New("platform requires private absolute state and a project resolver")
	}
	dir := filepath.Join(opts.StateDir, "platform")
	if _, err := hostpath.ConfineToMounts([]string{opts.StateDir}, opts.StateDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if _, err := hostpath.ConfineToMounts([]string{opts.StateDir}, dir); err != nil {
		return nil, err
	}
	if err := protect(dir); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(opts.StateDir + opts.InstanceID))
	instance := hex.EncodeToString(sum[:8])
	ctx, cancel := context.WithCancel(context.Background())
	if opts.Runner == nil {
		opts.Runner = commandRunner(opts.DockerHost)
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = providerHTTPClient()
	}
	return &Service{opts: opts, dir: dir, instance: instance, busy: map[string]bool{}, ctx: ctx, cancel: cancel}, nil
}
func (s *Service) Close() { s.cancel(); s.wg.Wait() }
func (s *Service) path(name string) (string, error) {
	if !identifier.MatchString(name) {
		return "", errors.New("invalid platform identifier")
	}
	if _, err := hostpath.ConfineToMounts([]string{s.dir}, s.dir); err != nil {
		return "", err
	}
	p := filepath.Join(s.dir, name)
	if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
		return p, nil
	} else if err != nil {
		return "", err
	}
	if err := hostpath.OneName(p); err != nil {
		return "", err
	}
	return hostpath.ConfineToMounts([]string{s.dir}, p)
}
func (s *Service) write(name string, value any) error {
	p, e := s.path(name)
	if e != nil {
		return e
	}
	data, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(s.dir, ".write-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = protect(tmp); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func (s *Service) read(name string, value any) error {
	p, e := s.path(name)
	if e != nil {
		return e
	}
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	return json.NewDecoder(io.LimitReader(f, 4<<20)).Decode(value)
}
func (s *Service) load(id string) (ProjectState, error) {
	v := ProjectState{ProjectID: id, Deployments: []Deployment{}, Backups: []Backup{}, Database: Database{Status: "not-created", Engine: "PostgreSQL 16", Message: "A real PostgreSQL database can be started on this Docker engine."}}
	e := s.read(id, &v)
	if errors.Is(e, os.ErrNotExist) {
		e = nil
	}
	if v.ProjectID != id {
		return v, errors.New("platform project binding mismatch")
	}
	return v, e
}
func (s *Service) update(id string, f func(*ProjectState)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.load(id)
	if e != nil {
		return e
	}
	f(&v)
	return s.write(id, v)
}
func (s *Service) claim(id string, limit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 {
		limit = 1
	}
	if s.busy[id] || len(s.busy) >= limit {
		return false
	}
	s.busy[id] = true
	return true
}
func operationLimit(r *http.Request) int {
	_, p, _ := management.ActorFromContext(r.Context())
	return p.MaxConcurrentChecksPerUser
}
func (s *Service) release(id string) { s.mu.Lock(); delete(s.busy, id); s.mu.Unlock() }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]string{"error": code, "message": message})
}
func body(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		problem(w, 400, "invalid_request", "Check the supplied platform settings.")
		return false
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		problem(w, 400, "invalid_request", "Supply one JSON object.")
		return false
	}
	return true
}
func (s *Service) project(w http.ResponseWriter, r *http.Request) (project.Project, bool) {
	p, e := s.opts.ResolveProject(r.PathValue("id"))
	if e != nil {
		problem(w, 404, "project_unavailable", "This project is not available in your workspace.")
		return p, false
	}
	return p, true
}
func admin(r *http.Request) bool {
	u, _, ok := management.ActorFromContext(r.Context())
	return ok && (u.Role == "admin" || u.Role == "super-admin")
}
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/platform/status", s.status)
	mux.HandleFunc("GET /v1/platform/projects/{id}", s.projectStatus)
	mux.HandleFunc("POST /v1/platform/projects/{id}/deployments", s.deploy)
	mux.HandleFunc("POST /v1/platform/projects/{id}/deployments/{releaseId}/{action}", s.deploymentAction)
	mux.HandleFunc("GET /v1/platform/projects/{id}/deployments/{releaseId}/logs", s.logs)
	mux.HandleFunc("POST /v1/platform/projects/{id}/database", s.databaseAction)
	mux.HandleFunc("GET /v1/platform/projects/{id}/backups/{backupId}", s.downloadBackup)
	s.registerPanacea(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := management.ActorFromContext(r.Context()); !ok {
			problem(w, 401, "sign_in_required", "Sign in to use the platform.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Service) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err := s.opts.Runner(ctx, "docker", []string{"info", "--format", "{{.ServerVersion}}"}, s.dir, nil)
	_, policy, _ := management.ActorFromContext(r.Context())
	limits := s.limits(r)
	respond(w, 200, map[string]any{"dockerReady": err == nil, "limits": map[string]int{"memoryMiB": limits.MemoryMiB, "cpuMillis": limits.CPUMillis}, "permissions": map[string]bool{"connectPanacea": admin(r), "deployPanacea": policy.AllowCloudProvisioning}, "capabilities": map[string]bool{"persistentLocalDeployments": err == nil, "postgres": err == nil, "postgresBackups": err == nil, "panaceaConnector": true, "supabaseInstalled": false}, "scope": "Local deployments persist on this Docker host. Public hosting requires a connected hosting provider. Limits apply per running service, not to total host capacity or billing.", "supabase": map[string]any{"status": "not-installed", "message": "Full Supabase auth, storage and realtime are separate from the PostgreSQL service.", "upstream": "https://supabase.com/docs/guides/self-hosting/docker"}})
}
func (s *Service) projectStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	v, e := s.load(p.ID)
	busy := s.busy[p.ID]
	s.mu.Unlock()
	if e != nil {
		problem(w, 500, "state_unavailable", "Platform history could not be read.")
		return
	}
	// A process restart cannot leave a stale building record looking active.
	// Ready records are last-verified receipts; an explicit refresh verifies
	// the current container and HTTP health without silently starting anything.
	if !busy {
		for i := range v.Deployments {
			if v.Deployments[i].Provider == "docker" && v.Deployments[i].Status == "building" {
				v.Deployments[i].Status = "interrupted"
				v.Deployments[i].Message = "Luci stopped during deployment. Verify its owned container before restarting."
			}
		}
	}
	respond(w, 200, map[string]any{"project": v, "busy": busy})
}
