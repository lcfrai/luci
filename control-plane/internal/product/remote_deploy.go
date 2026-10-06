package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CloudReleaseRequest struct {
	TargetID            string `json:"targetId"`
	Repository          string `json:"repository"`
	Branch              string `json:"branch"`
	Commit              string `json:"commit"`
	Domain              string `json:"domain"`
	Port                int    `json:"port"`
	HealthPath          string `json:"healthPath"`
	Dockerfile          string `json:"dockerfile"`
	Source              string `json:"source,omitempty"`
	PrivateKeyUUID      string `json:"privateKeyUuid,omitempty"`
	DataClientFramework string `json:"dataClientFramework,omitempty"`
}

// CloudRelease stores provider identifiers outside the project repository. It
// never contains the target's token or connection URL.
type CloudRelease struct {
	Release
	CloudReleaseRequest    `json:"-"`
	ProviderProjectID      string `json:"providerProjectId,omitempty"`
	ApplicationID          string `json:"applicationId,omitempty"`
	DeploymentID           string `json:"deploymentId,omitempty"`
	TargetFingerprint      string `json:"targetFingerprint"`
	RollbackOf             string `json:"rollbackOf,omitempty"`
	ServerUUID             string `json:"serverUuid,omitempty"`
	DataDomainRevision     int    `json:"dataDomainRevision,omitempty"`
	DataBindingFingerprint string `json:"dataBindingFingerprint,omitempty"`
	DataBindingStatus      string `json:"dataBindingStatus,omitempty"`
}

// The persisted/public wire shape has one commit, from Release provenance.
// Keep the original request separately in memory for idempotency comparisons.
type cloudReleaseWire struct {
	Release
	TargetID               string `json:"targetId"`
	Repository             string `json:"repository"`
	Branch                 string `json:"branch"`
	Domain                 string `json:"domain"`
	Port                   int    `json:"port"`
	HealthPath             string `json:"healthPath"`
	Dockerfile             string `json:"dockerfile"`
	Source                 string `json:"source,omitempty"`
	PrivateKeyUUID         string `json:"privateKeyUuid,omitempty"`
	ProviderProjectID      string `json:"providerProjectId,omitempty"`
	ApplicationID          string `json:"applicationId,omitempty"`
	DeploymentID           string `json:"deploymentId,omitempty"`
	TargetFingerprint      string `json:"targetFingerprint"`
	RollbackOf             string `json:"rollbackOf,omitempty"`
	ServerUUID             string `json:"serverUuid,omitempty"`
	DataClientFramework    string `json:"dataClientFramework,omitempty"`
	DataDomainRevision     int    `json:"dataDomainRevision,omitempty"`
	DataBindingFingerprint string `json:"dataBindingFingerprint,omitempty"`
	DataBindingStatus      string `json:"dataBindingStatus,omitempty"`
}

func (row CloudRelease) MarshalJSON() ([]byte, error) {
	return json.Marshal(cloudReleaseWire{
		Release: row.Release, TargetID: row.TargetID, Repository: row.Repository,
		Branch: row.Branch, Domain: row.Domain, Port: row.Port,
		HealthPath: row.HealthPath, Dockerfile: row.Dockerfile,
		Source: row.Source, PrivateKeyUUID: row.PrivateKeyUUID,
		ProviderProjectID: row.ProviderProjectID, ApplicationID: row.ApplicationID,
		DeploymentID: row.DeploymentID, TargetFingerprint: row.TargetFingerprint,
		RollbackOf: row.RollbackOf,
		ServerUUID: row.ServerUUID, DataClientFramework: row.DataClientFramework,
		DataDomainRevision: row.DataDomainRevision, DataBindingFingerprint: row.DataBindingFingerprint,
		DataBindingStatus: row.DataBindingStatus,
	})
}

func (row *CloudRelease) UnmarshalJSON(data []byte) error {
	var decoded cloudReleaseWire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*row = CloudRelease{
		Release: decoded.Release,
		CloudReleaseRequest: CloudReleaseRequest{
			TargetID: decoded.TargetID, Repository: decoded.Repository,
			Branch: decoded.Branch, Commit: decoded.Commit, Domain: decoded.Domain,
			Port: decoded.Port, HealthPath: decoded.HealthPath, Dockerfile: decoded.Dockerfile,
			Source: decoded.Source, PrivateKeyUUID: decoded.PrivateKeyUUID,
			DataClientFramework: decoded.DataClientFramework,
		},
		ProviderProjectID: decoded.ProviderProjectID, ApplicationID: decoded.ApplicationID,
		DeploymentID: decoded.DeploymentID, TargetFingerprint: decoded.TargetFingerprint,
		RollbackOf: decoded.RollbackOf,
		ServerUUID: decoded.ServerUUID, DataDomainRevision: decoded.DataDomainRevision, DataBindingFingerprint: decoded.DataBindingFingerprint,
		DataBindingStatus: decoded.DataBindingStatus,
	}
	return nil
}

func (s *Server) registerCloudReleases(mux Registrar) {
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/cloud", s.createCloudRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/{releaseId}/refresh", s.refreshRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/{releaseId}/rollback", s.rollbackCloudRelease)
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/deploy-key", s.prepareDeployKey)
}

func (s *Server) cloudReleaseFile(projectID string) string {
	return filepath.Join(s.stateDir, "cloud-releases", projectID+".json")
}

// readCloudReleases requires the caller to hold s.mu.
func (s *Server) readCloudReleases(projectID string) ([]CloudRelease, error) {
	if !coolifyID.MatchString(projectID) {
		return nil, errors.New("invalid project identifier")
	}
	file, err := os.Open(s.cloudReleaseFile(projectID))
	if errors.Is(err, os.ErrNotExist) {
		return []CloudRelease{}, nil
	}
	if err != nil {
		return nil, errors.New("could not open cloud release history")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return nil, errors.New("cloud release history is unreadable or exceeds 8 MiB")
	}
	rows := []CloudRelease{}
	if json.Unmarshal(data, &rows) != nil {
		return nil, errors.New("cloud release history is invalid")
	}
	for _, row := range rows {
		if row.ProjectID != projectID || !coolifyID.MatchString(row.ID) || !coolifyCommit.MatchString(row.Release.Commit) {
			return nil, errors.New("cloud release history has invalid provenance")
		}
	}
	return rows, nil
}

func (s *Server) cloudReleases(projectID string) ([]Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readCloudReleases(projectID)
	if err != nil {
		return nil, err
	}
	result := []Release{}
	for _, row := range rows {
		result = append(result, row.Release)
	}
	return result, nil
}

func (s *Server) storeCloudRelease(row CloudRelease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readCloudReleases(row.ProjectID)
	if err != nil {
		return err
	}
	for i := range rows {
		if rows[i].ID == row.ID {
			rows[i] = row
			return atomicJSON(s.cloudReleaseFile(row.ProjectID), rows)
		}
	}
	return errors.New("cloud release reservation no longer exists")
}

func targetFingerprint(target DeploymentTarget) string {
	// An authenticated SSH tunnel may have a new local port after reconnecting.
	// The selected target and remote server remain the stable identity.
	sum := sha256.Sum256([]byte(target.ID + "\x00" + target.ServerUUID))
	return hex.EncodeToString(sum[:])
}

// reserveCloudRelease performs no remote mutation. Exact repeated requests are
// returned as the same release, including after a restart or ambiguous failure.
func (s *Server) reserveCloudRelease(row CloudRelease) (CloudRelease, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readCloudReleases(row.ProjectID)
	if err != nil {
		return row, false, err
	}
	for _, previous := range rows {
		if previous.TargetID == row.TargetID && previous.CloudReleaseRequest == row.CloudReleaseRequest && previous.BriefRevision == row.BriefRevision && previous.ConfigurationRevision == row.ConfigurationRevision && previous.DataDomainRevision == row.DataDomainRevision && previous.DataBindingFingerprint == row.DataBindingFingerprint && previous.Status != "failed" && previous.Status != "cancelled" {
			if previous.TargetFingerprint != row.TargetFingerprint {
				return row, false, errors.New("the deployment target changed; reconnect the original target before refreshing")
			}
			return previous, false, nil
		}
		if previous.TargetID == row.TargetID && previous.Domain == row.Domain {
			switch previous.Status {
			case "preparing", "queued", "deploying", "verifying", "unknown", "attention", "stopping":
				return row, false, errors.New("refresh or resolve the existing release for this target and domain before deploying another commit")
			}
			if row.ApplicationID == "" && previous.ApplicationID != "" && previous.Repository == row.Repository && previous.TargetFingerprint == row.TargetFingerprint {
				if (previous.Source == "private") != (row.Source == "private") || previous.PrivateKeyUUID != row.PrivateKeyUUID {
					return row, false, errors.New("the existing application's repository access cannot be changed in place; use a separate domain/application")
				}
				row.ApplicationID, row.ProviderProjectID = previous.ApplicationID, previous.ProviderProjectID
			}
		}
	}
	if len(rows) >= 5000 {
		return row, false, errors.New("cloud release history is full")
	}
	rows = append([]CloudRelease{row}, rows...)
	if err := atomicJSON(s.cloudReleaseFile(row.ProjectID), rows); err != nil {
		return row, false, errors.New("could not reserve the cloud release")
	}
	return row, true, nil
}

func (s *Server) createCloudRelease(w http.ResponseWriter, r *http.Request) {
	var req CloudReleaseRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validDataClientFramework(req.DataClientFramework) {
		journeyError(w, 400, "invalid_data_client_framework", "Choose runtime-only, Vite or Next.js browser data variables.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("cloud-maintenance:" + p.ID) {
		journeyError(w, 409, "release_busy", "Another release operation is in progress.")
		return
	}
	defer s.releaseDelivery("cloud-maintenance:" + p.ID)
	target, err := s.deploymentTarget(req.TargetID)
	if err != nil {
		journeyError(w, 400, "deployment_target_unavailable", "Select a connected deployment target.")
		return
	}
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		journeyError(w, 400, "deployment_target_invalid", err.Error())
		return
	}
	if req.Port == 0 {
		req.Port = 3000
	}
	if req.HealthPath == "" {
		req.HealthPath = "/health"
	}
	if req.Dockerfile == "" {
		req.Dockerfile = "Dockerfile"
	}
	req.Dockerfile = strings.TrimPrefix(req.Dockerfile, "/")
	req.Commit = strings.ToLower(req.Commit)
	req.Domain = strings.TrimSuffix(req.Domain, "/")
	if _, err := releaseHealthURL(req.Domain, req.HealthPath); err != nil {
		journeyError(w, 400, "invalid_health_url", err.Error())
		return
	}
	app := CoolifyGitApplication{ProjectUUID: "pending", ServerUUID: target.ServerUUID, Name: "luci-" + p.ID, Repository: req.Repository, Branch: req.Branch, Commit: req.Commit, Dockerfile: "/" + req.Dockerfile, Port: req.Port, Domain: req.Domain, HealthPath: req.HealthPath, Source: req.Source, PrivateKeyUUID: req.PrivateKeyUUID}
	if _, err := coolifyGitApplicationPayload(app); err != nil {
		journeyError(w, 400, "invalid_cloud_release", err.Error())
		return
	}
	if blockedProjectFile(req.Dockerfile) {
		journeyError(w, 400, "invalid_dockerfile", "Choose a project Dockerfile.")
		return
	}
	if _, err := guardedProjectPath(p.Path, req.Dockerfile, false); err != nil {
		journeyError(w, 400, "invalid_dockerfile", "Choose an existing Dockerfile inside this project.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	status, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		journeyError(w, 422, "git_failed", "Could not verify the local Git worktree.")
		return
	}
	if status != "" {
		journeyError(w, 409, "commit_required", "Commit all project changes before publishing.")
		return
	}
	commit, err := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	commit = strings.TrimSpace(commit)
	if err != nil || commit != req.Commit {
		journeyError(w, 409, "commit_changed", "The requested release commit must match this project's current HEAD.")
		return
	}
	fileType, err := s.journeyGit(ctx, p.Path, "cat-file", "-t", commit+":"+req.Dockerfile)
	if err != nil || strings.TrimSpace(fileType) != "blob" {
		journeyError(w, 409, "dockerfile_commit_required", "Commit the selected Dockerfile before publishing.")
		return
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "history_unavailable", "Could not read the project brief history.")
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
	row := CloudRelease{Release: Release{ID: newID("cloud-rel-"), ProjectID: p.ID, BriefRevision: state.Brief.Revision, ConfigurationRevision: configuration.Revision, Commit: commit, Target: target.ID, Status: "preparing", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, CloudReleaseRequest: req, TargetFingerprint: targetFingerprint(target), ServerUUID: target.ServerUUID}
	data, err := s.cloudApplicationDataSnapshot(ctx, row)
	if err != nil {
		journeyError(w, 409, "data_domain_unverified", err.Error())
		return
	}
	row.DataDomainRevision, row.DataBindingFingerprint = data.DomainRevision, cloudDataFingerprint(data, req.DataClientFramework)
	if row.DataBindingFingerprint != "" {
		row.DataBindingStatus = "current"
	}
	if _, err := cloudApplicationVariables(row, configuration, data); err != nil {
		journeyError(w, 409, "data_configuration_conflict", err.Error())
		return
	}
	if len(dataClientAliases(req.DataClientFramework)) > 0 {
		dockerfile, err := s.journeyGit(ctx, p.Path, "show", commit+":"+req.Dockerfile)
		if err != nil || len(dockerfile) > 1<<20 || requireDataClientDockerfileArgs(dockerfile, req.DataClientFramework) != nil {
			journeyError(w, 409, "data_build_arguments_required", "Declare the selected public Data URL and anon key as Dockerfile ARG inputs in the framework build stage, then commit and push before deploying.")
			return
		}
	}
	row, created, err := s.reserveCloudRelease(row)
	if err != nil {
		journeyError(w, 409, "release_conflict", err.Error())
		return
	}
	if !created {
		journeyJSON(w, http.StatusAccepted, row)
		return
	}
	fail := func(err error) {
		row.Status = "attention"
		row.Message = err.Error() + " No automatic retry was made; inspect the recorded provider resource before retrying."
		if storeErr := s.storeCloudRelease(row); storeErr != nil {
			journeyError(w, 500, "release_record_failed", "The remote outcome is uncertain and its record could not be updated.")
			return
		}
		journeyJSON(w, http.StatusBadGateway, row)
	}
	if err := client.Check(ctx); err != nil {
		// No remote mutation has been attempted, so an explicit repeat after
		// repairing credentials can safely reserve a new attempt.
		row.Status, row.Message = "failed", err.Error()
		if err := s.storeCloudRelease(row); err != nil {
			journeyError(w, 500, "release_record_failed", "Could not save the connection failure.")
			return
		}
		journeyJSON(w, http.StatusBadGateway, row)
		return
	}
	if row.ProviderProjectID == "" {
		remoteProject, err := client.EnsureProject(ctx, "luci-"+p.ID)
		if err != nil {
			fail(err)
			return
		}
		row.ProviderProjectID = remoteProject.UUID
		if err := s.storeCloudRelease(row); err != nil {
			fail(errors.New("could not save the remote project identifier"))
			return
		}
	}
	app.ProjectUUID = row.ProviderProjectID
	if row.ApplicationID == "" {
		row.ApplicationID, err = client.CreateGitApplication(ctx, app)
	} else {
		err = client.UpdateGitApplication(ctx, row.ApplicationID, app)
	}
	if err != nil {
		fail(err)
		return
	}
	if err := s.storeCloudRelease(row); err != nil {
		fail(errors.New("could not save the remote application identifier"))
		return
	}
	if err := s.reconcileCloudApplicationConfiguration(ctx, row, client, configuration, data); err != nil {
		// Environment mutations are reconciled by ownership/revision before a
		// retry. No deployment has been requested, so an explicit new attempt
		// can safely reuse the application after repairing configuration.
		row.Status = "failed"
		row.Message = "Runtime application configuration was not confirmed. No deployment started. Resolve the configuration and retry."
		if s.storeCloudRelease(row) != nil {
			journeyError(w, 500, "release_record_failed", "Could not save the configuration failure; no deployment started.")
			return
		}
		journeyJSON(w, http.StatusBadGateway, row)
		return
	}
	if err := s.recheckCloudApplicationData(ctx, row); err != nil {
		row.Status, row.Message = "failed", err.Error()+" No deployment started."
		row.DataBindingStatus = "redeploy-required"
		if s.storeCloudRelease(row) != nil {
			journeyError(w, 500, "release_record_failed", "Could not save the changed Data binding; no deployment started.")
			return
		}
		journeyJSON(w, 409, row)
		return
	}
	deployment, err := client.Deploy(ctx, row.ApplicationID)
	if err != nil {
		fail(err)
		return
	}
	row.DeploymentID, row.Status = deployment.UUID, deployment.Status
	row.Message = "Deployment queued. Refresh to check the remote build and public HTTPS health."
	if err := s.storeCloudRelease(row); err != nil {
		fail(errors.New("deployment queued but its identifier could not be saved"))
		return
	}
	journeyJSON(w, http.StatusAccepted, row)
}

func (s *Server) refreshCloudRelease(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("cloud-maintenance:" + p.ID) {
		journeyError(w, 409, "release_busy", "Another release operation is in progress.")
		return
	}
	defer s.releaseDelivery("cloud-maintenance:" + p.ID)
	s.mu.RLock()
	rows, err := s.readCloudReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, 500, "release_history_unavailable", err.Error())
		return
	}
	var row CloudRelease
	for _, candidate := range rows {
		if candidate.ID == r.PathValue("releaseId") {
			row = candidate
			break
		}
	}
	if row.ID == "" {
		journeyError(w, 404, "release_missing", "This project has no cloud release with that ID.")
		return
	}
	if row.Status == "ready" || row.Status == "failed" || row.Status == "cancelled" || row.Status == "stopped" {
		// Terminal records describe the historical release. Probing an old URL
		// after a newer commit was deployed would not verify the old revision.
		if row.Status == "ready" && row.DataBindingFingerprint != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
			defer cancel()
			if err := s.recheckCloudApplicationData(ctx, row); err != nil {
				row.DataBindingStatus = "redeploy-required"
				row.Message = "This release's Data domain or client key changed. Verify the current Data domain and deploy again; browser values in the existing image are unchanged."
				if err := s.storeCloudRelease(row); err != nil {
					journeyError(w, 500, "release_record_failed", "Could not save the stale Data binding observation.")
					return
				}
			}
		}
		journeyJSON(w, 200, row)
		return
	}
	if row.DeploymentID == "" {
		journeyError(w, 409, "deployment_unconfirmed", "The provider did not confirm a deployment identifier. Resolve its recorded resources before retrying.")
		return
	}
	target, err := s.deploymentTarget(row.TargetID)
	if err != nil || targetFingerprint(target) != row.TargetFingerprint {
		journeyError(w, 409, "target_changed", "Reconnect the deployment target used by this release before refreshing.")
		return
	}
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		journeyError(w, 400, "target_invalid", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	row, err = observeCloudRelease(ctx, row, client, checkPublicReleaseHealth)
	if err != nil {
		journeyError(w, 502, "deployment_check_failed", err.Error())
		return
	}
	if err := s.storeCloudRelease(row); err != nil {
		journeyError(w, 500, "release_record_failed", "Could not save the observed deployment state.")
		return
	}
	journeyJSON(w, 200, row)
}

func observeCloudRelease(ctx context.Context, row CloudRelease, client *CoolifyClient, health func(context.Context, string) error) (CloudRelease, error) {
	if row.Status == "stopping" {
		status, err := client.ApplicationStatus(ctx, row.ApplicationID)
		if err != nil {
			return row, err
		}
		row.URL = ""
		row.Message = "Stop was requested; waiting for the application to stop."
		base := strings.ToLower(strings.TrimSpace(strings.Split(status, ":")[0]))
		if base == "exited" || base == "stopped" {
			row.Status = "stopped"
			row.Message = "Coolify confirmed that the application stopped. No volumes or networks were pruned."
		}
		return row, nil
	}
	deployment, err := client.Deployment(ctx, row.DeploymentID)
	if err != nil {
		return row, err
	}
	row.Status, row.URL = deployment.Status, ""
	row.Message = "Remote deployment status: " + deployment.Status + "."
	if deployment.Status != "verifying" {
		return row, nil
	}
	if deployment.Commit != row.Release.Commit {
		row.Status = "attention"
		row.Message = "The provider has not confirmed the expected release commit. Public health cannot verify the source revision."
		return row, nil
	}
	healthURL, err := releaseHealthURL(row.Domain, row.HealthPath)
	if err != nil {
		return row, err
	}
	if err := health(ctx, healthURL); err != nil {
		row.Message = "The build completed; waiting for a successful public HTTPS health response. Check DNS, TLS and the application's health path."
		return row, nil
	}
	row.Status, row.URL = "ready", row.Domain
	row.Message = "The provider confirmed this commit and the public HTTPS health check succeeded."
	return row, nil
}

func releaseHealthURL(domain, healthPath string) (string, error) {
	u, err := coolifyHTTPSURL(domain)
	if err != nil || u.RawPath != "" || (u.Path != "" && u.Path != "/") || !publicReleaseHost(u.Hostname()) {
		return "", errors.New("use a public HTTPS application domain")
	}
	path, err := url.Parse(healthPath)
	if err != nil || path.IsAbs() || path.Host != "" || path.RawQuery != "" || path.ForceQuery || path.Fragment != "" || path.RawPath != "" || !strings.HasPrefix(path.Path, "/") || strings.HasPrefix(path.Path, "//") || strings.ContainsAny(path.Path, "\\\r\n\x00") {
		return "", errors.New("use a health path such as /health without a query, fragment or host")
	}
	u.Path, u.RawPath = path.Path, ""
	return u.String(), nil
}

func publicReleaseHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return publicReleaseIP(ip)
	}
	host = strings.ToLower(host)
	return strings.Contains(host, ".") && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local")
}

func publicReleaseIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func checkPublicReleaseHealth(ctx context.Context, address string) error {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid health address")
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.New("health DNS is unavailable")
		}
		for _, candidate := range addresses {
			if !publicReleaseIP(candidate.IP) {
				return nil, errors.New("health DNS must resolve to public addresses")
			}
		}
		// Dial the checked address, not the hostname, to avoid a second DNS lookup.
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}}
	defer transport.CloseIdleConnections()
	return probeReleaseHealth(ctx, address, &http.Client{Timeout: 8 * time.Second, Transport: transport})
}

func probeReleaseHealth(ctx context.Context, address string, client *http.Client) error {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.New("health checks require HTTPS")
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return errors.New("could not prepare HTTPS health check")
	}
	response, err := copyClient.Do(req)
	if err != nil {
		return errors.New("HTTPS health check has not succeeded")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("HTTPS health check returned a non-success status")
	}
	return nil
}
