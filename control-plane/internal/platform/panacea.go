package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// These transport DTOs are independently implemented against the inspected
// API contract. The private Panacea repository is not distributed with Luci.
type panaceaConnection struct {
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
	CheckedAt string `json:"checkedAt,omitempty"`
}
type RemoteProject struct {
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	RepoURL       string `json:"repoUrl"`
	DefaultBranch string `json:"defaultBranch"`
	Status        string `json:"status"`
	URL           string `json:"url"`
}

var remoteName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var branchName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

func connectionURL(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(u.Host, "\\\r\n") {
		return "", errors.New("Panacea requires an HTTPS origin without credentials, query or path")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}
func (s *Service) connection() (panaceaConnection, error) {
	var c panaceaConnection
	e := s.read("panacea-connection", &c)
	if e != nil {
		return c, e
	}
	if _, e = connectionURL(c.BaseURL); e != nil {
		return c, e
	}
	if len(c.APIKey) < 16 || strings.ContainsAny(c.APIKey, "\r\n\x00") {
		return c, errors.New("invalid private provider credential")
	}
	return c, nil
}
func (c panaceaConnection) binding() string {
	h := sha256.Sum256([]byte(c.BaseURL))
	return hex.EncodeToString(h[:])
}
func (s *Service) provider(ctx context.Context, c panaceaConnection, method, route string, input, output any) error {
	var b []byte
	var e error
	if input != nil {
		b, e = json.Marshal(input)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, c.BaseURL+route, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := s.opts.HTTPClient.Do(req)
	if e != nil {
		return errors.New("Panacea connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("Panacea denied or could not complete this operation")
	}
	if output == nil {
		return nil
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return errors.New("Panacea response exceeded its limit")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, output)
	}
	return json.Unmarshal(raw, output)
}
func (s *Service) registerPanacea(m *http.ServeMux) {
	m.HandleFunc("GET /v1/platform/panacea", s.panaceaStatus)
	m.HandleFunc("PUT /v1/platform/panacea", s.savePanacea)
	m.HandleFunc("DELETE /v1/platform/panacea", s.removePanacea)
	m.HandleFunc("POST /v1/platform/panacea/check", s.checkPanacea)
	m.HandleFunc("GET /v1/platform/panacea/projects", s.panaceaProjects)
	m.HandleFunc("PUT /v1/platform/projects/{id}/panacea", s.mapPanacea)
	m.HandleFunc("POST /v1/platform/projects/{id}/panacea/deploy", s.deployPanacea)
	m.HandleFunc("POST /v1/platform/projects/{id}/panacea/deployments/{releaseId}/refresh", s.refreshPanacea)
}
func (s *Service) panaceaStatus(w http.ResponseWriter, r *http.Request) {
	c, e := s.connection()
	respond(w, 200, map[string]any{"configured": e == nil, "baseUrl": c.BaseURL, "checkedAt": c.CheckedAt, "canManage": admin(r), "source": "External Panacea API; private source is not bundled with Luci."})
}
func (s *Service) savePanacea(w http.ResponseWriter, r *http.Request) {
	if !admin(r) {
		problem(w, 403, "admin_required", "An administrator must connect this workspace to Panacea.")
		return
	}
	var c panaceaConnection
	if !body(w, r, &c) {
		return
	}
	base, e := connectionURL(c.BaseURL)
	if e != nil || len(c.APIKey) < 16 || len(c.APIKey) > 4096 || strings.ContainsAny(c.APIKey, "\r\n\x00") {
		problem(w, 400, "invalid_connection", "Enter the HTTPS Panacea origin and a scoped personal API key.")
		return
	}
	c.BaseURL = base
	c.CheckedAt = ""
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var me struct {
		Username string `json:"username"`
	}
	if e = s.provider(ctx, c, "GET", "/api/v1/me", nil, &me); e != nil || strings.TrimSpace(me.Username) == "" {
		problem(w, 502, "provider_unavailable", "Panacea could not verify the scoped API credential.")
		return
	}
	c.CheckedAt = now()
	s.mu.Lock()
	e = s.write("panacea-connection", c)
	s.mu.Unlock()
	if e != nil {
		problem(w, 500, "state_unavailable", "The private connection could not be saved.")
		return
	}
	s.panaceaStatus(w, r)
}
func (s *Service) removePanacea(w http.ResponseWriter, r *http.Request) {
	if !admin(r) {
		problem(w, 403, "admin_required", "An administrator must disconnect Panacea.")
		return
	}
	file, e := s.path("panacea-connection")
	if e == nil {
		e = os.Remove(file)
	}
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		problem(w, 500, "state_unavailable", "The connection could not be removed.")
		return
	}
	respond(w, 200, map[string]bool{"configured": false})
}
func (s *Service) checkPanacea(w http.ResponseWriter, r *http.Request) {
	c, e := s.connection()
	if e != nil {
		problem(w, 409, "connection_required", "Connect a Panacea API first.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var me struct {
		Username string `json:"username"`
	}
	if e = s.provider(ctx, c, "GET", "/api/v1/me", nil, &me); e != nil || strings.TrimSpace(me.Username) == "" {
		problem(w, 502, "provider_unavailable", "Panacea could not verify this connection.")
		return
	}
	c.CheckedAt = now()
	s.mu.Lock()
	e = s.write("panacea-connection", c)
	s.mu.Unlock()
	if e != nil {
		problem(w, 500, "state_unavailable", "Connection verification could not be saved.")
		return
	}
	s.panaceaStatus(w, r)
}
func (s *Service) panaceaProjects(w http.ResponseWriter, r *http.Request) {
	c, e := s.connection()
	if e != nil {
		problem(w, 409, "connection_required", "Connect a Panacea API first.")
		return
	}
	var projects []RemoteProject
	if e = s.provider(r.Context(), c, "GET", "/api/v1/projects", nil, &projects); e != nil {
		problem(w, 502, "provider_unavailable", "Panacea projects could not be read.")
		return
	}
	if projects == nil {
		projects = []RemoteProject{}
	}
	respond(w, 200, map[string]any{"projects": projects})
}
func (s *Service) mapPanacea(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	var in struct {
		ProjectName string `json:"projectName"`
	}
	if !body(w, r, &in) {
		return
	}
	if !remoteName.MatchString(in.ProjectName) {
		problem(w, 400, "invalid_project", "Choose a Panacea project from the connected account.")
		return
	}
	c, e := s.connection()
	if e != nil {
		problem(w, 409, "connection_required", "Connect a Panacea API first.")
		return
	}
	var remote RemoteProject
	if e = s.provider(r.Context(), c, "GET", "/api/v1/projects/"+in.ProjectName, nil, &remote); e != nil || remote.Name != in.ProjectName {
		problem(w, 403, "project_unavailable", "Panacea did not grant access to that project.")
		return
	}
	if e = s.update(p.ID, func(v *ProjectState) { v.PanaceaProject = in.ProjectName }); e != nil {
		problem(w, 500, "state_unavailable", "The project connection could not be saved.")
		return
	}
	s.projectStatus(w, r)
}
func (s *Service) deployPanacea(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	_, policy, _ := management.ActorFromContext(r.Context())
	if !policy.AllowCloudProvisioning {
		problem(w, 403, "policy_denied", "Cloud provisioning is disabled by the workspace policy.")
		return
	}
	var in struct {
		Commit string `json:"commit"`
		Branch string `json:"branch"`
	}
	if !body(w, r, &in) {
		return
	}
	if !commitID.MatchString(in.Commit) || !branchName.MatchString(in.Branch) || strings.Contains(in.Branch, "..") || strings.HasSuffix(in.Branch, "/") {
		problem(w, 400, "invalid_deployment", "Choose an exact commit and valid branch.")
		return
	}
	if s.opts.ValidateDeploymentSource == nil || s.opts.ValidateDeploymentSource(r.Context(), p.ID, in.Commit) != nil {
		problem(w, 409, "source_not_ready", "Commit the approved context and all source changes before deploying.")
		return
	}
	c, e := s.connection()
	if e != nil {
		problem(w, 409, "connection_required", "Connect a Panacea API first.")
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running.")
		return
	}
	defer s.release(p.ID)
	s.mu.Lock()
	state, e := s.load(p.ID)
	s.mu.Unlock()
	if e != nil || !remoteName.MatchString(state.PanaceaProject) {
		problem(w, 409, "project_required", "Connect this project to a Panacea project first.")
		return
	}
	var result struct {
		Project     string `json:"project"`
		Status      string `json:"status"`
		PipelineRun string `json:"pipelineRun"`
	}
	if e = s.provider(r.Context(), c, "POST", "/api/v1/projects/"+state.PanaceaProject+"/deploy", in, &result); e != nil || result.PipelineRun == "" || !identifier.MatchString(result.PipelineRun) {
		problem(w, 502, "deployment_unconfirmed", "Panacea did not confirm a pipeline. Check its deployment history before retrying.")
		return
	}
	d := Deployment{ID: randomID("deploy-"), ProjectID: p.ID, Provider: "panacea", Commit: in.Commit, Status: "deploying", Message: "Panacea accepted the exact commit. Public readiness has not been verified.", CreatedAt: now(), UpdatedAt: now(), RemoteProject: state.PanaceaProject, PipelineRun: result.PipelineRun, ProviderBinding: c.binding()}
	if e = s.saveDeployment(d); e != nil {
		problem(w, 500, "state_unavailable", "Panacea accepted the build, but its local receipt could not be saved. Check provider history before retrying.")
		return
	}
	respond(w, 202, d)
}
func (s *Service) refreshPanacea(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	d, e := s.findDeployment(p.ID, r.PathValue("releaseId"))
	if e != nil || d.Provider != "panacea" {
		problem(w, 404, "deployment_unavailable", "This Panacea deployment is unavailable.")
		return
	}
	c, e := s.connection()
	if e != nil || c.binding() != d.ProviderBinding {
		problem(w, 409, "connection_changed", "Reconnect the Panacea origin used for this deployment.")
		return
	}
	var history []struct {
		ID                string `json:"id"`
		Commit            string `json:"commit"`
		Status            string `json:"status"`
		Image             string `json:"image"`
		LiveReadyReplicas *int   `json:"liveReadyReplicas"`
	}
	if e = s.provider(r.Context(), c, "GET", "/api/v1/projects/"+d.RemoteProject+"/deployments", nil, &history); e != nil {
		problem(w, 502, "provider_unavailable", "Panacea deployment status could not be read.")
		return
	}
	d.Status = "deploying"
	d.Message = "Waiting for the exact provider commit and verified application health."
	d.URL = ""
	for _, item := range history {
		if item.ID != d.PipelineRun || item.Commit != d.Commit {
			continue
		}
		if strings.EqualFold(item.Status, "failed") {
			d.Status = "failed"
			d.Message = "Panacea reports a failed deployment. Inspect its build logs."
			break
		}
		if !strings.EqualFold(item.Status, "succeeded") || item.LiveReadyReplicas == nil || *item.LiveReadyReplicas < 1 {
			break
		}
		var remote RemoteProject
		if e = s.provider(r.Context(), c, "GET", "/api/v1/projects/"+d.RemoteProject, nil, &remote); e != nil {
			break
		}
		address, e := connectionURL(remote.URL)
		if e != nil || !s.healthy(r.Context(), address, "/") {
			break
		}
		d.Status = "ready"
		d.URL = address
		d.Image = item.Image
		d.Message = "Panacea reports the requested commit with ready replicas, and its HTTPS endpoint answered successfully."
		break
	}
	d.UpdatedAt = now()
	if e = s.saveDeployment(d); e != nil {
		problem(w, 500, "state_unavailable", "Deployment status could not be saved.")
		return
	}
	respond(w, 200, d)
}
