package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// CoolifyClient adapts Coolify v4's API without exposing its credentials or raw
// upstream errors. The caller owns persistence, cloud consent and health checks.
// It deliberately does not retry mutations after an ambiguous network failure.
type CoolifyClient struct {
	beforeMutation func(context.Context) error

	base  *url.URL
	token string
	http  *http.Client
}

type CoolifyProject struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CoolifyServer struct {
	UUID     string                 `json:"uuid"`
	Name     string                 `json:"name"`
	IP       string                 `json:"ip"`
	Settings *CoolifyServerSettings `json:"settings,omitempty"`
}

// Missing metadata is not evidence that a server is suitable for co-location.
type CoolifyServerSettings struct {
	BuildOnly    *bool `json:"is_build_server"`
	SwarmManager *bool `json:"is_swarm_manager"`
	SwarmWorker  *bool `json:"is_swarm_worker"`
	Usable       *bool `json:"is_usable"`
	Reachable    *bool `json:"is_reachable"`
}

// CoolifyGitApplication requires source accessible to the remote server. It does
// not upload local source. Private repository authentication is a separate flow.
type CoolifyGitApplication struct {
	ProjectUUID    string
	ServerUUID     string
	Environment    string
	Name           string
	Repository     string
	Branch         string
	Commit         string
	Dockerfile     string
	Port           int
	Domain         string
	HealthPath     string
	Source         string
	PrivateKeyUUID string
}

type CoolifyDeployment struct {
	UUID           string `json:"uuid"`
	Status         string `json:"status"`
	ProviderStatus string `json:"providerStatus"`
	Commit         string `json:"commit,omitempty"`
}

// CoolifyAPIError contains no upstream response body, URL or credentials.
type CoolifyAPIError struct {
	StatusCode int
}

func (e *CoolifyAPIError) Error() string {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return "Coolify rejected the API token; reconnect this deployment target"
	case http.StatusForbidden:
		return "Coolify denied access; check API access, team permissions and IP allowlist"
	case http.StatusNotFound:
		return "Coolify could not find the requested resource"
	case http.StatusConflict:
		return "Coolify reported a resource or domain conflict; resolve it before retrying"
	case http.StatusTooManyRequests:
		return "Coolify rate limit reached; wait before retrying"
	default:
		return fmt.Sprintf("Coolify request failed (HTTP %d)", e.StatusCode)
	}
}

var coolifyID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var coolifyCommit = regexp.MustCompile(`^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$`)
var coolifyDNSLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// NewCoolifyClient accepts an HTTPS origin, optionally ending in /api/v1.
// HTTP is allowed only to a literal loopback IP for a separately authenticated
// SSH tunnel. Redirects are disabled even within the same origin.
func NewCoolifyClient(baseURL, token string) (*CoolifyClient, error) {
	u, err := coolifyURL(baseURL, true)
	if err != nil || u.RawPath != "" || (u.Path != "" && u.Path != "/" && u.Path != "/api/v1" && u.Path != "/api/v1/") {
		return nil, errors.New("enter an HTTPS Coolify origin or an HTTP loopback SSH tunnel, without credentials, query or fragment")
	}
	if token == "" || len(token) > 4096 || strings.ContainsAny(token, " \t\r\n") {
		return nil, errors.New("enter a complete Coolify API token without whitespace")
	}
	for _, r := range token {
		if r < 33 || r > 126 {
			return nil, errors.New("Coolify API token contains unsupported characters")
		}
	}
	u.Path, u.RawPath = "/api/v1", ""
	return &CoolifyClient{base: u, token: token, http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func coolifyHTTPSURL(value string) (*url.URL, error) {
	return coolifyURL(value, false)
}

func coolifyURL(value string, allowLoopbackHTTP bool) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("invalid HTTPS URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(allowLoopbackHTTP && u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return nil, errors.New("invalid HTTPS URL")
	}
	if u.Port() != "" {
		port, portErr := net.LookupPort("tcp", u.Port())
		if portErr != nil || port < 1 || port > 65535 {
			return nil, errors.New("invalid HTTPS port")
		}
	}
	host := u.Hostname()
	if net.ParseIP(host) == nil {
		if len(host) > 253 {
			return nil, errors.New("invalid HTTPS host")
		}
		for _, label := range strings.Split(host, ".") {
			if !coolifyDNSLabel.MatchString(label) {
				return nil, errors.New("invalid HTTPS host")
			}
		}
	}
	return u, nil
}

func (c *CoolifyClient) request(ctx context.Context, method, path string, query url.Values, body, output any) error {
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return errors.New("could not encode the Coolify request")
		}
		input = bytes.NewReader(data)
	}
	u := *c.base
	u.Path += path
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), input)
	if err != nil {
		return errors.New("could not prepare the Coolify request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead && c.beforeMutation != nil {
		if err := c.beforeMutation(ctx); err != nil {
			return err
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// net/http errors contain the request URL; do not return them to logs/UI.
		return errors.New("could not reach Coolify securely; check its address and TLS certificate")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return errors.New("Coolify returned a redirect; enter its direct HTTPS origin")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &CoolifyAPIError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return errors.New("Coolify returned an unreadable or oversized response")
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return errors.New("Coolify returned an unexpected response")
	}
	return nil
}

func (c *CoolifyClient) Check(ctx context.Context) error {
	var team struct {
		ID int `json:"id"`
	}
	if err := c.request(ctx, http.MethodGet, "/teams/current", nil, nil, &team); err != nil {
		return err
	}
	if team.ID <= 0 {
		return errors.New("Coolify did not return an authenticated team")
	}
	return nil
}

func (c *CoolifyClient) ListServers(ctx context.Context) ([]CoolifyServer, error) {
	items := []CoolifyServer{}
	err := c.request(ctx, http.MethodGet, "/servers", nil, nil, &items)
	return items, err
}

func (c *CoolifyClient) ListProjects(ctx context.Context) ([]CoolifyProject, error) {
	items := []CoolifyProject{}
	err := c.request(ctx, http.MethodGet, "/projects", nil, nil, &items)
	return items, err
}

// EnsureProject reuses an unambiguous exact name. Persist the returned UUID;
// callers should serialize this operation per target to avoid concurrent creates.
func (c *CoolifyClient) EnsureProject(ctx context.Context, name string) (CoolifyProject, error) {
	if strings.TrimSpace(name) != name || name == "" || len(name) > 200 || strings.ContainsAny(name, "\r\n") {
		return CoolifyProject{}, errors.New("provide a project name of 1 to 200 characters")
	}
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return CoolifyProject{}, err
	}
	var found CoolifyProject
	for _, p := range projects {
		if p.Name == name {
			if found.UUID != "" {
				return CoolifyProject{}, errors.New("multiple Coolify projects have this name; select one explicitly")
			}
			if !coolifyID.MatchString(p.UUID) {
				return CoolifyProject{}, errors.New("Coolify returned an invalid project identifier")
			}
			found = p
		}
	}
	if found.UUID != "" {
		return found, nil
	}
	var created CoolifyProject
	if err := c.request(ctx, http.MethodPost, "/projects", nil, map[string]string{"name": name, "description": "Managed by Luci"}, &created); err != nil {
		return CoolifyProject{}, err
	}
	if !coolifyID.MatchString(created.UUID) {
		return CoolifyProject{}, errors.New("Coolify did not return the created project identifier; check before retrying")
	}
	created.Name = name
	return created, nil
}

func coolifyGitApplicationPayload(app CoolifyGitApplication) (map[string]any, error) {
	if !coolifyID.MatchString(app.ProjectUUID) || !coolifyID.MatchString(app.ServerUUID) {
		return nil, errors.New("select a valid Coolify project and server")
	}
	repo, err := coolifyHTTPSURL(app.Repository)
	if err != nil || repo.Path == "" || repo.Path == "/" {
		return nil, errors.New("provide an accessible HTTPS Git repository without embedded credentials")
	}
	if host := repo.Hostname(); strings.EqualFold(host, "localhost") || !strings.Contains(host, ".") || (net.ParseIP(host) != nil && (net.ParseIP(host).IsPrivate() || net.ParseIP(host).IsLoopback() || net.ParseIP(host).IsUnspecified() || net.ParseIP(host).IsLinkLocalUnicast())) {
		return nil, errors.New("the Git repository must be accessible from the remote deployment server")
	}
	if app.Branch == "" || len(app.Branch) > 200 || strings.ContainsAny(app.Branch, "\r\n\x00") || !coolifyCommit.MatchString(app.Commit) {
		return nil, errors.New("provide a Git branch and full release commit hash")
	}
	if app.Environment == "" {
		app.Environment = "production"
	}
	if app.Dockerfile == "" {
		app.Dockerfile = "/Dockerfile"
	}
	if app.HealthPath == "" {
		app.HealthPath = "/health"
	}
	if strings.TrimSpace(app.Name) == "" || len(app.Name) > 200 || len(app.Environment) > 200 || strings.ContainsAny(app.Name+app.Environment+app.Dockerfile+app.HealthPath, "\r\n\x00") {
		return nil, errors.New("provide a valid application name and build configuration")
	}
	if app.Port < 1 || app.Port > 65535 || !strings.HasPrefix(app.HealthPath, "/") || strings.HasPrefix(app.HealthPath, "//") {
		return nil, errors.New("provide a valid application port and health path")
	}
	domain, err := coolifyHTTPSURL(app.Domain)
	if err != nil || (domain.Path != "" && domain.Path != "/") {
		return nil, errors.New("provide the application's HTTPS domain without a path")
	}
	body := map[string]any{
		"project_uuid": app.ProjectUUID, "server_uuid": app.ServerUUID,
		"environment_name": app.Environment, "name": app.Name,
		"git_repository": app.Repository, "git_branch": app.Branch, "git_commit_sha": app.Commit,
		"build_pack": "dockerfile", "dockerfile_location": app.Dockerfile,
		"ports_exposes": fmt.Sprint(app.Port), "domains": strings.TrimSuffix(app.Domain, "/"),
		"instant_deploy": false, "is_auto_deploy_enabled": false,
		"health_check_enabled": true, "health_check_path": app.HealthPath,
		"force_domain_override": false,
	}
	if app.Source != "" && app.Source != "public" && app.Source != "private" {
		return nil, errors.New("choose public or private repository access")
	}
	if app.Source == "private" {
		owner, name, err := githubDeploymentRepository(app.Repository)
		if err != nil || !coolifyID.MatchString(app.PrivateKeyUUID) {
			return nil, errors.New("private GitHub deployment requires its HTTPS repository and a Coolify private key UUID")
		}
		body["git_repository"] = "git@github.com:" + owner + "/" + name + ".git"
		body["private_key_uuid"] = app.PrivateKeyUUID
	}
	return body, nil
}

func (c *CoolifyClient) CreateGitApplication(ctx context.Context, app CoolifyGitApplication) (string, error) {
	body, err := coolifyGitApplicationPayload(app)
	if err != nil {
		return "", err
	}
	var created struct {
		UUID string `json:"uuid"`
	}
	endpoint := "/applications/public"
	if app.Source == "private" {
		endpoint = "/applications/private-deploy-key"
	}
	if err := c.request(ctx, http.MethodPost, endpoint, nil, body, &created); err != nil {
		return "", err
	}
	if !coolifyID.MatchString(created.UUID) {
		return "", errors.New("Coolify did not return the created application identifier; check before retrying")
	}
	return created.UUID, nil
}

// UpdateGitApplication updates the already-owned application for a subsequent
// release. It does not trigger deployment or force domain conflicts.
func (c *CoolifyClient) UpdateGitApplication(ctx context.Context, appUUID string, app CoolifyGitApplication) error {
	if !coolifyID.MatchString(appUUID) {
		return errors.New("invalid Coolify application identifier")
	}
	body, err := coolifyGitApplicationPayload(app)
	if err != nil {
		return err
	}
	// v4.3.23's PATCH schema cannot change a deploy key. Reuse is allowed
	// only for the same saved source/key by the release reservation layer.
	delete(body, "private_key_uuid")
	return c.request(ctx, http.MethodPatch, "/applications/"+appUUID, nil, body, nil)
}

type CoolifyPrivateKey struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func (c *CoolifyClient) PrivateKeys(ctx context.Context) ([]CoolifyPrivateKey, error) {
	var keys []CoolifyPrivateKey
	err := c.request(ctx, http.MethodGet, "/security/keys", nil, nil, &keys)
	return keys, err
}
func (c *CoolifyClient) CreatePrivateKey(ctx context.Context, name, key string) (string, error) {
	var value struct {
		UUID string `json:"uuid"`
	}
	err := c.request(ctx, http.MethodPost, "/security/keys", nil, map[string]string{"name": name, "description": "Luci read-only GitHub deployment access", "private_key": key}, &value)
	if err != nil {
		return "", err
	}
	if !coolifyID.MatchString(value.UUID) {
		return "", errors.New("Coolify did not confirm the created private key; refresh this operation before retrying")
	}
	return value.UUID, nil
}
func (c *CoolifyClient) StopApplication(ctx context.Context, id string) error {
	if !coolifyID.MatchString(id) {
		return errors.New("invalid owned application ID")
	}
	return c.request(ctx, http.MethodPost, "/applications/"+id+"/stop", url.Values{"docker_cleanup": {"false"}}, nil, nil)
}
func (c *CoolifyClient) ApplicationStatus(ctx context.Context, id string) (string, error) {
	if !coolifyID.MatchString(id) {
		return "", errors.New("invalid owned application ID")
	}
	var value struct {
		UUID   string `json:"uuid"`
		Status string `json:"status"`
	}
	if err := c.request(ctx, http.MethodGet, "/applications/"+id, nil, nil, &value); err != nil {
		return "", err
	}
	if value.UUID != id {
		return "", errors.New("Coolify returned a different application")
	}
	return value.Status, nil
}

func (c *CoolifyClient) Deploy(ctx context.Context, resourceUUID string) (CoolifyDeployment, error) {
	if !coolifyID.MatchString(resourceUUID) {
		return CoolifyDeployment{}, errors.New("invalid Coolify resource identifier")
	}
	var result struct {
		Deployments []struct {
			ResourceUUID string `json:"resource_uuid"`
			UUID         string `json:"deployment_uuid"`
		} `json:"deployments"`
	}
	if err := c.request(ctx, http.MethodPost, "/deploy", url.Values{"uuid": {resourceUUID}, "force": {"false"}}, nil, &result); err != nil {
		return CoolifyDeployment{}, err
	}
	if len(result.Deployments) != 1 || result.Deployments[0].ResourceUUID != resourceUUID || !coolifyID.MatchString(result.Deployments[0].UUID) {
		return CoolifyDeployment{}, errors.New("Coolify did not confirm this deployment; check its state before retrying")
	}
	return CoolifyDeployment{UUID: result.Deployments[0].UUID, Status: "queued", ProviderStatus: "queued"}, nil
}

func (c *CoolifyClient) Deployment(ctx context.Context, deploymentUUID string) (CoolifyDeployment, error) {
	if !coolifyID.MatchString(deploymentUUID) {
		return CoolifyDeployment{}, errors.New("invalid Coolify deployment identifier")
	}
	var result struct {
		UUID   string `json:"deployment_uuid"`
		Status string `json:"status"`
		Commit string `json:"commit"`
	}
	if err := c.request(ctx, http.MethodGet, "/deployments/"+deploymentUUID, nil, nil, &result); err != nil {
		return CoolifyDeployment{}, err
	}
	if result.UUID != deploymentUUID {
		return CoolifyDeployment{}, errors.New("Coolify returned a different deployment identifier")
	}
	status, provider := "unknown", "unknown"
	switch result.Status {
	case "queued", "pending":
		status, provider = "queued", result.Status
	case "in_progress", "running":
		status, provider = "deploying", result.Status
	case "finished":
		status, provider = "verifying", result.Status
	case "failed", "error":
		status, provider = "failed", result.Status
	case "cancelled", "canceled", "cancelled-by-user":
		status, provider = "cancelled", result.Status
	}
	if !coolifyCommit.MatchString(result.Commit) {
		result.Commit = ""
	}
	// A provider's finished state is not proof that DNS/TLS/public health works.
	return CoolifyDeployment{UUID: result.UUID, Status: status, ProviderStatus: provider, Commit: result.Commit}, nil
}
