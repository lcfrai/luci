package platform

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/gitcommand"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const labelInstance = "ai.lcfr.luci.platform.instance"
const labelProject = "ai.lcfr.luci.platform.project"
const labelResource = "ai.lcfr.luci.platform.resource"

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type resourceLimits struct{ MemoryMiB, CPUMillis int }

func (s *Service) limits(r *http.Request) resourceLimits {
	_, p, _ := management.ActorFromContext(r.Context())
	m, c := p.CheckMemoryMiB, p.CheckCPUMillis
	if m < 128 {
		m = 1024
	}
	if c < 100 {
		c = 1000
	}
	if s.opts.MemoryMiB > 0 && m > s.opts.MemoryMiB {
		m = s.opts.MemoryMiB
	}
	if s.opts.CPUMillis > 0 && c > s.opts.CPUMillis {
		c = s.opts.CPUMillis
	}
	return resourceLimits{m, c}
}
func (l resourceLimits) args() []string {
	return []string{"--memory", strconv.Itoa(l.MemoryMiB) + "m", "--memory-swap", strconv.Itoa(l.MemoryMiB) + "m", "--cpus", fmt.Sprintf("%.3f", float64(l.CPUMillis)/1000), "--pids-limit", "256"}
}
func (s *Service) docker(ctx context.Context, args ...string) ([]byte, error) {
	return s.opts.Runner(ctx, "docker", args, s.dir, nil)
}
func (s *Service) labels(id, resource string) []string {
	return []string{"--label", labelInstance + "=" + s.instance, "--label", labelProject + "=" + id, "--label", labelResource + "=" + resource}
}
func (s *Service) resource(id, kind string) string {
	h := sha256.Sum256([]byte(id))
	return "luci-platform-" + s.instance + "-" + hex.EncodeToString(h[:6]) + "-" + kind
}
func (s *Service) ensureNetwork(ctx context.Context, id string) (string, error) {
	name := s.resource(id, "net")
	b, e := s.docker(ctx, "network", "inspect", name, "--format", "{{json .Labels}}")
	if e == nil {
		var labels map[string]string
		if json.Unmarshal(b, &labels) != nil || labels[labelInstance] != s.instance || labels[labelProject] != id {
			return "", errors.New("network ownership mismatch")
		}
		return name, nil
	}
	args := append([]string{"network", "create"}, s.labels(id, "network")...)
	args = append(args, name)
	if _, e = s.docker(ctx, args...); e != nil {
		return "", e
	}
	return name, nil
}

type containerInfo struct {
	ID, Image string
	Config    struct {
		Labels map[string]string
		Image  string
		Env    []string
	}
	State struct {
		Running bool
		Health  struct{ Status string }
	}
	NetworkSettings struct {
		Ports map[string][]struct{ HostIP, HostPort string }
	}
	Mounts []struct {
		Type, Name, Destination string
		RW                      bool
	}
}

func (s *Service) inspect(ctx context.Context, id, resource, name string) (containerInfo, error) {
	var all []containerInfo
	b, e := s.docker(ctx, "inspect", name)
	if e != nil {
		return containerInfo{}, e
	}
	if json.Unmarshal(b, &all) != nil || len(all) != 1 {
		return containerInfo{}, errors.New("invalid container response")
	}
	v := all[0]
	if v.Config.Labels[labelInstance] != s.instance || v.Config.Labels[labelProject] != id || v.Config.Labels[labelResource] != resource {
		return v, errors.New("container ownership mismatch")
	}
	return v, nil
}
func loopbackURL(v containerInfo, port int) (string, error) {
	b := v.NetworkSettings.Ports[strconv.Itoa(port)+"/tcp"]
	if len(b) != 1 || b[0].HostIP != "127.0.0.1" {
		return "", errors.New("container must bind only to literal loopback")
	}
	p, e := strconv.Atoi(b[0].HostPort)
	if e != nil || p < 1 || p > 65535 {
		return "", errors.New("invalid container port")
	}
	return "http://127.0.0.1:" + strconv.Itoa(p), nil
}
func validHealthPath(v string) bool {
	u, e := url.Parse(v)
	return e == nil && strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") && u.IsAbs() == false && u.Host == "" && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" && !strings.Contains(v, "\\") && path.Clean(v) == v
}
func (s *Service) healthy(ctx context.Context, address, health string) bool {
	req, e := http.NewRequestWithContext(ctx, "GET", address+health, nil)
	if e != nil {
		return false
	}
	res, e := s.opts.HTTPClient.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode >= 200 && res.StatusCode < 300
}
func (s *Service) waitHealthy(ctx context.Context, d *Deployment) error {
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	for {
		v, e := s.inspect(ctx, d.ProjectID, d.ID, d.Container)
		if e != nil {
			return e
		}
		if !v.State.Running {
			return errors.New("application container is not running")
		}
		address, e := loopbackURL(v, d.Port)
		if e != nil {
			return e
		}
		d.Image = v.Image
		if s.healthy(ctx, address, d.HealthPath) {
			d.URL = address
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("application health did not become ready")
		case <-time.After(500 * time.Millisecond):
		}
	}
}
func (s *Service) environment(id string) (int, map[string]string, error) {
	rev := 0
	values := map[string]string{}
	if s.opts.ApplicationConfiguration != nil {
		var e error
		rev, values, e = s.opts.ApplicationConfiguration(id)
		if e != nil {
			return 0, nil, e
		}
	}
	copyValues := map[string]string{}
	for k, v := range values {
		if !environmentName.MatchString(k) || strings.ContainsAny(v, "\x00\r\n") || len(v) > 16384 {
			return 0, nil, errors.New("invalid runtime configuration")
		}
		copyValues[k] = v
	}
	var secret databaseSecret
	if s.read(id+"-database-secret", &secret) == nil {
		s.mu.Lock()
		state, err := s.load(id)
		s.mu.Unlock()
		if err != nil || !validDatabasePassword(secret.Password) {
			return 0, nil, errors.New("invalid database binding state")
		}
		name, err := activeDatabaseName(state.Database)
		if err != nil {
			return 0, nil, err
		}
		copyValues["DATABASE_URL"] = "postgresql://luci:" + secret.Password + "@database:5432/" + name + "?sslmode=disable"
	}
	dataValues, e := s.dataBindings(id)
	if e != nil {
		return 0, nil, e
	}
	for key, value := range dataValues {
		copyValues[key] = value
	}
	if _, hasLegacyDatabase := copyValues["DATABASE_URL"]; !hasLegacyDatabase && dataValues["SUPABASE_DB_URL"] != "" {
		copyValues["DATABASE_URL"] = dataValues["SUPABASE_DB_URL"]
	}
	return rev, copyValues, nil
}
func (s *Service) envFile(values map[string]string) (string, error) {
	f, e := os.CreateTemp(s.dir, ".runtime-env-")
	if e != nil {
		return "", e
	}
	name := f.Name()
	defer f.Close()
	if e = protect(name); e != nil {
		os.Remove(name)
		return "", e
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !environmentName.MatchString(k) || strings.ContainsAny(values[k], "\r\n\x00") {
			os.Remove(name)
			return "", errors.New("invalid configuration")
		}
		if _, e = fmt.Fprintf(f, "%s=%s\n", k, values[k]); e != nil {
			os.Remove(name)
			return "", e
		}
	}
	return name, nil
}

// archive supplies Docker only committed bytes. There are no host bind mounts,
// submodules, links or credential/state files in the build context.
func (s *Service) archive(ctx context.Context, p project.Project, commit string) ([]byte, error) {
	args := []string{"--git-dir=" + filepath.Join(p.Path, ".git"), "--work-tree=" + p.Path, "-c", "core.bare=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "archive", "--format=tar", commit}
	raw, e := s.opts.Runner(ctx, "git", gitcommand.PlatformArgs(p.Path, args), p.Path, nil)
	if e != nil {
		return nil, e
	}
	reader := tar.NewReader(bytes.NewReader(raw))
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	dockerfile := false
	for {
		h, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		// Git emits this global PAX record before the committed files. It is
		// provenance metadata, not a filesystem entry. Accept only the exact
		// requested commit's comment; no global path, size or link overrides
		// may enter Docker's build context.
		if h.Typeflag == tar.TypeXGlobalHeader {
			if h.Name != "pax_global_header" || len(h.PAXRecords) != 1 || h.PAXRecords["comment"] != commit {
				return nil, errors.New("source archive contains unsupported metadata")
			}
			continue
		}
		clean := path.Clean(h.Name)
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "\\") || h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return nil, errors.New("source archive contains unsupported paths")
		}
		if protectedBuildPath(clean) {
			continue
		}
		if clean == "Dockerfile" {
			dockerfile = true
		}
		if h.Size > 16<<20 || out.Len() > 64<<20 {
			return nil, errors.New("source archive exceeds limit")
		}
		if e = writer.WriteHeader(h); e != nil {
			return nil, e
		}
		if _, e = io.Copy(writer, reader); e != nil {
			return nil, e
		}
	}
	if !dockerfile {
		return nil, errors.New("commit requires a root Dockerfile")
	}
	if e = writer.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
func protectedBuildPath(v string) bool {
	for _, part := range strings.Split(strings.ToLower(v), "/") {
		if part == ".luci" || part == ".git" || part == ".env" || strings.HasPrefix(part, ".env.") || part == ".ssh" || part == ".aws" || part == ".azure" || part == ".npmrc" || part == ".netrc" || part == "credentials" || part == "secrets.json" || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") {
			return true
		}
	}
	return false
}
func (s *Service) saveDeployment(d Deployment) error {
	return s.update(d.ProjectID, func(v *ProjectState) {
		for i := range v.Deployments {
			if v.Deployments[i].ID == d.ID {
				v.Deployments[i] = d
				return
			}
		}
		v.Deployments = append([]Deployment{d}, v.Deployments...)
	})
}
func (s *Service) deploy(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	var in struct {
		Commit     string `json:"commit"`
		Port       int    `json:"port"`
		HealthPath string `json:"healthPath"`
	}
	if !body(w, r, &in) {
		return
	}
	if !commitID.MatchString(in.Commit) || in.Port < 1 || in.Port > 65535 || !validHealthPath(in.HealthPath) {
		problem(w, 400, "invalid_deployment", "Choose the exact commit, application port and an absolute health path.")
		return
	}
	runtime := s.previewRuntime()
	if runtime.CaptureActor == nil || runtime.Start == nil || runtime.Stop == nil || runtime.Observe == nil || s.opts.DeploymentSourceDigest == nil || s.opts.RecordDeploymentProvenance == nil || s.opts.ValidateDeploymentSource == nil {
		problem(w, 503, "preview_runtime_unavailable", "Shared preview admission and source validation are required. No build started.")
		return
	}
	actor, err := runtime.CaptureActor(r.Context(), p.ID)
	if err != nil {
		problem(w, 403, "deployment_forbidden", "Current Edit and Deploy access are required.")
		return
	}
	if err := s.opts.ValidateDeploymentSource(r.Context(), p.ID, in.Commit); err != nil {
		problem(w, 409, "source_not_ready", "Commit the approved project context and all changes before deployment.")
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return
	}
	rev, baselineEnvironment, err := s.environment(p.ID)
	env := make(map[string]string, len(baselineEnvironment))
	for k, v := range baselineEnvironment {
		env[k] = v
	}
	if err == nil {
		err = s.deploymentBrowserData(r, p.ID, env)
	}
	if err != nil {
		s.release(p.ID)
		problem(w, 500, "configuration_unavailable", "Runtime configuration could not be read.")
		return
	}
	d := Deployment{ID: randomID("deploy-"), ProjectID: p.ID, Provider: "docker", Commit: in.Commit, ConfigurationRevision: rev, Status: "building", Message: "Preparing the exact committed source for shared admission.", Port: in.Port, HealthPath: in.HealthPath, CreatedAt: now(), UpdatedAt: now(), Revision: 1}
	if err = s.opts.RecordDeploymentProvenance(r.Context(), p.ID, d.ID, "native-docker", d.Commit); err != nil {
		s.release(p.ID)
		problem(w, 409, "release_provenance_unavailable", "The exact source and private review selections could not be saved. No deployment started.")
		return
	}
	digest, err := s.opts.DeploymentSourceDigest(r.Context(), p.ID, d.ID, d.Commit)
	if err != nil {
		s.release(p.ID)
		problem(w, 409, "source_provenance_unavailable", "The saved source identity is unavailable.")
		return
	}
	d.Runtime = &previewruntime.Outcome{Identity: previewruntime.Identity{ProjectID: p.ID, ReleaseID: d.ID, Target: "native-docker", Commit: d.Commit, SourceDigest: digest}, Phase: "reserved"}
	if err = s.write(d.ID+"-redactions", env); err != nil {
		s.release(p.ID)
		problem(w, 500, "state_unavailable", "Private deployment settings could not be saved.")
		return
	}
	if err = s.saveDeployment(d); err != nil {
		s.release(p.ID)
		problem(w, 500, "state_unavailable", "Deployment could not be recorded.")
		return
	}
	limits := s.limits(r)
	s.wg.Add(1)
	go func(d Deployment) {
		defer s.wg.Done()
		defer s.release(p.ID)
		ctx, cancel := context.WithTimeout(s.ctx, 15*time.Minute)
		defer cancel()
		request := previewruntime.Request{Identity: d.Runtime.Identity, Dockerfile: "Dockerfile", Port: d.Port, Environment: env, Actor: actor, MemoryMiB: limits.MemoryMiB, CPUMillis: limits.CPUMillis,
			Labels: map[string]string{labelInstance: s.instance, labelProject: p.ID, labelResource: d.ID},
			ValidateSource: func(verify context.Context) error {
				if err := s.opts.ValidateDeploymentSource(verify, p.ID, d.Commit); err != nil {
					return err
				}
				return nil
			},
		}
		err := s.buildAndRun(ctx, p, &d, request)
		if err != nil {
			next := d
			next.Status, next.URL, next.Message = "failed", "", "The admitted deployment did not finish. Check its retained receipt and bounded logs."
			if d.Runtime.ResourcesHeld {
				next.Status = "unknown"
				next.Message = "The deployment outcome is uncertain. Its reservation is retained; refresh or stop this exact deployment."
			}
			_ = s.storeAdmittedDeployment(d, next)
		}
	}(d)
	respond(w, 202, d)
}
func (s *Service) buildAndRun(ctx context.Context, p project.Project, d *Deployment, request previewruntime.Request) error {
	archive, err := s.archive(ctx, p, d.Commit)
	if err != nil {
		return err
	}
	request.SourceArchive = archive
	request.Network, err = s.existingDeploymentNetwork(ctx, p.ID, request.Environment)
	if err != nil {
		return err
	}
	out, err := s.previewRuntime().Start(ctx, request, func(value previewruntime.Outcome) error {
		return s.publishDeploymentOutcome(d, value, request.Environment)
	})
	if err != nil {
		return err
	}
	if err = s.publishDeploymentOutcome(d, out, request.Environment); err != nil {
		return err
	}
	if out.Phase != "running" || !out.ResourcesHeld {
		return errors.New("the admitted deployment is not running")
	}
	if err = s.waitAdmittedDeploymentHealthy(ctx, d); err != nil {
		return err
	}
	next := *d
	next.Status, next.Message, next.UpdatedAt = "ready", "The admitted Docker deployment returned a successful HTTP response.", now()
	if err = s.storeAdmittedDeployment(*d, next); err != nil {
		return err
	}
	next.Revision++
	*d = next
	return nil
}
func (s *Service) findDeployment(id, release string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.load(id)
	if e != nil {
		return Deployment{}, e
	}
	for _, d := range v.Deployments {
		if d.ID == release {
			return d, nil
		}
	}
	return Deployment{}, os.ErrNotExist
}
func (s *Service) deploymentAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	action := r.PathValue("action")
	if action != "stop" && action != "restart" && action != "refresh" {
		problem(w, 404, "action_unavailable", "This deployment action is not supported.")
		return
	}
	d, e := s.findDeployment(p.ID, r.PathValue("releaseId"))
	if e != nil || d.Provider != "docker" {
		problem(w, 404, "deployment_unavailable", "This Docker deployment is unavailable.")
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running.")
		return
	}
	defer s.release(p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
	defer cancel()
	if d.Runtime != nil {
		s.admittedDeploymentAction(w, r, ctx, d, action)
		return
	}
	info, e := s.inspect(ctx, p.ID, d.ID, d.Container)
	if e != nil {
		problem(w, 409, "ownership_unverified", "The recorded container is missing or its ownership has changed.")
		return
	}
	if action == "restart" {
		if err := s.checkDeploymentDatabaseBinding(p.ID, d, info); err != nil {
			problem(w, 409, "database_redeployment_required", "Redeploy this application to connect to the selected restored database.")
			return
		}
		problem(w, 409, "redeployment_required", "This historical deployment has no shared reservation. Create a new admitted deployment to restart it.")
		return
	}

	if action != "refresh" {
		if _, e = s.docker(ctx, action, d.Container); e != nil {
			problem(w, 502, "operation_failed", "Docker could not complete the requested operation.")
			return
		}
	}
	d.UpdatedAt = now()
	if action == "stop" {
		d.Status = "stopped"
		d.URL = ""
		d.Message = "Deployment stopped; image and database data are retained."
	} else if e = s.waitHealthy(ctx, &d); e != nil {
		d.Status = "failed"
		d.URL = ""
		d.Message = "The container is not healthy."
	} else {
		d.Status = "ready"
		d.Message = "The owned Docker deployment is healthy."
	}
	if e = s.saveDeployment(d); e != nil {
		problem(w, 500, "state_unavailable", "Deployment state could not be saved.")
		return
	}
	respond(w, 200, s.browserDeployment(d))
}
func (s *Service) logs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	d, e := s.findDeployment(p.ID, r.PathValue("releaseId"))
	if e != nil || d.Provider != "docker" {
		problem(w, 404, "deployment_unavailable", "This Docker deployment is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if _, e = s.inspectDeploymentForLogs(ctx, d); e != nil {
		problem(w, 409, "ownership_unverified", "Container ownership could not be verified.")
		return
	}
	b, e := s.docker(ctx, "logs", "--tail", "200", d.Container)
	if e != nil {
		problem(w, 502, "logs_unavailable", "Application logs could not be read.")
		return
	}
	if len(b) > 128<<10 {
		b = b[len(b)-(128<<10):]
	}
	var values map[string]string
	if e = s.read(d.ID+"-redactions", &values); e != nil {
		problem(w, 500, "configuration_unavailable", "Log redaction settings are unavailable.")
		return
	}
	text := string(b)
	for _, v := range values {
		if v != "" {
			text = strings.ReplaceAll(text, v, "[redacted]")
		}
	}
	var database databaseSecret
	if s.read(p.ID+"-database-secret", &database) == nil && database.Password != "" {
		text = strings.ReplaceAll(text, database.Password, "[redacted]")
	}
	respond(w, 200, map[string]string{"text": text, "notice": "Last 200 stdout lines. Do not write application secrets to logs."})
}
