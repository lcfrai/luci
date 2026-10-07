package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// PublicDataDomainRequest is a trusted backend request, not an HTTP body.
// The user supplies only target/domain/application origin/public acknowledgement.
// Project, tenant, provider IDs and network proof come from owned integrations.
type PublicDataDomainRequest struct {
	ProjectID           string
	StableTenant        string
	TargetID            string
	ServerUUID          string
	ProviderProjectUUID string
	Environment         string
	DestinationUUID     string
	Domain              string
	AppOrigin           string
	AcknowledgePublic   bool
	ExpectedRevision    int
}

type PublicDataHost struct {
	ProjectID          string
	StableTenant       string
	ApplicationNetwork string
	NetworkID          string
	DockerEngineID     string
	PublishableKey     string // Public anon JWT only; never a service key or signing secret.
}

type PublicDataDomainPrerequisites struct {
	Data        PublicDataHost
	VerifyHost  func(context.Context, CoolifyServer) error
	VerifyHTTPS func(context.Context, PublicDataDomainBinding) error
}

// PublicDataDomainBinding contains no provider token, database or admin key.
// Public readiness is independent of native/private DataStatus readiness.
type PublicDataDomainBinding struct {
	ProjectID           string `json:"projectId"`
	StableTenant        string `json:"stableTenant"`
	TargetID            string `json:"targetId"`
	TargetFingerprint   string `json:"targetFingerprint"`
	ServerUUID          string `json:"serverUuid"`
	ProviderProjectUUID string `json:"providerProjectUuid,omitempty"`
	ProviderServiceUUID string `json:"providerServiceUuid,omitempty"`
	Domain              string `json:"domain"`
	AppOrigin           string `json:"applicationOrigin"`
	ApplicationNetwork  string `json:"applicationNetwork"`
	NetworkID           string `json:"networkId"`
	DockerEngineID      string `json:"dockerEngineId"`
	ConfigurationDigest string `json:"configurationDigest"`
	ProbePath           string `json:"probePath"`
	Revision            int    `json:"revision"`
	Status              string `json:"status"` // pending, uncertain, verifiedHTTPS, stopped, attention
	Message             string `json:"message"`
	CheckedAt           string `json:"checkedAt,omitempty"`
}

type dataDomainRecord struct {
	PublicDataDomainBinding
	Owner               string `json:"owner"`
	Environment         string `json:"environment"`
	DestinationUUID     string `json:"destinationUuid,omitempty"`
	ComposeHash         string `json:"composeHash"`
	ConfirmedHash       string `json:"confirmedHash,omitempty"`
	ProjectAttempted    bool   `json:"projectAttempted"`
	CreateAttempted     bool   `json:"createAttempted"`
	ActivationAttempted bool   `json:"activationAttempted"`
	StopRequested       bool   `json:"stopRequested"`
}

var dataDomainLocks sync.Map

func (s *Server) dataDomainLock(id string) func() {
	v, _ := dataDomainLocks.LoadOrStore(struct {
		server *Server
		id     string
	}{s, id}, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func publicDataOrigin(value string) (string, error) {
	u, err := coolifyHTTPSURL(value)
	if err != nil || u.RawPath != "" || (u.Path != "" && u.Path != "/") || (u.Port() != "" && u.Port() != "443") {
		return "", errors.New("use a public HTTPS domain without credentials, path, query or custom port")
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("use a public DNS domain for HTTPS")
	}
	return "https://" + host, nil
}

func validDataAnon(key string) bool {
	p := strings.Split(key, ".")
	if len(key) < 32 || len(key) > 4096 || len(p) != 3 {
		return false
	}
	for _, part := range p {
		if part == "" {
			return false
		}
		if _, e := base64.RawURLEncoding.DecodeString(part); e != nil {
			return false
		}
	}
	b, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil {
		return false
	}
	var claim struct {
		Role string `json:"role"`
	}
	return json.Unmarshal(b, &claim) == nil && claim.Role == "anon"
}

func (s *Server) readDataDomain(id string) (dataDomainRecord, error) {
	var v dataDomainRecord
	path, err := s.privateApplicationStatePath("data-domain-" + id)
	if err != nil {
		return v, err
	}
	f, err := os.Open(path)
	if err != nil {
		return v, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 || json.Unmarshal(b, &v) != nil || v.ProjectID != id || !coolifyID.MatchString(v.Owner) || v.Revision < 1 || v.Revision >= 1_000_000_000 || len(v.ComposeHash) != 64 || v.ProbePath != "/__luci/data-health" {
		return dataDomainRecord{}, errors.New("public data domain ownership state is unavailable")
	}
	return v, nil
}
func (s *Server) saveDataDomain(v dataDomainRecord) error {
	path, err := s.privateApplicationStatePath("data-domain-" + v.ProjectID)
	if err != nil {
		return err
	}
	return atomicJSON(path, v)
}

// DataDomainBinding reads cached metadata only. It never contacts a provider.
func (s *Server) DataDomainBinding(id string) (PublicDataDomainBinding, error) {
	if _, err := s.resolveProject(id); err != nil {
		return PublicDataDomainBinding{}, err
	}
	unlock := s.dataDomainLock(id)
	defer unlock()
	v, err := s.readDataDomain(id)
	if errors.Is(err, os.ErrNotExist) {
		return PublicDataDomainBinding{ProjectID: id, Status: "not-configured"}, nil
	}
	return v.PublicDataDomainBinding, err
}

func (s *Server) dataDomainPrerequisites(ctx context.Context, req PublicDataDomainRequest, pre PublicDataDomainPrerequisites) (DeploymentTarget, *CoolifyClient, error) {
	if _, err := s.resolveProject(req.ProjectID); err != nil {
		return DeploymentTarget{}, nil, err
	}
	d := pre.Data
	if !coolifyID.MatchString(req.StableTenant) || d.ProjectID != req.ProjectID || d.StableTenant != req.StableTenant || !coolifyID.MatchString(d.ApplicationNetwork) || !coolifyID.MatchString(d.NetworkID) || !coolifyID.MatchString(d.DockerEngineID) || !validDataAnon(d.PublishableKey) || pre.VerifyHost == nil || pre.VerifyHTTPS == nil {
		return DeploymentTarget{}, nil, errors.New("verified native data host, network, client key and HTTPS checks are required")
	}
	target, err := s.deploymentTarget(req.TargetID)
	if err != nil || target.ServerUUID != req.ServerUUID {
		return DeploymentTarget{}, nil, errors.New("the selected deployment server changed; reconnect and verify it")
	}
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		return target, nil, err
	}
	servers, err := client.ListServers(ctx)
	if err != nil {
		return target, nil, err
	}
	var selected *CoolifyServer
	for i := range servers {
		if servers[i].UUID == req.ServerUUID {
			if selected != nil {
				return target, nil, errors.New("the provider returned an ambiguous server")
			}
			selected = &servers[i]
		}
	}
	if selected == nil || selected.Settings == nil {
		return target, nil, errors.New("Coolify server readiness metadata is unavailable")
	}
	p := selected.Settings
	if p.BuildOnly == nil || p.SwarmManager == nil || p.SwarmWorker == nil || p.Usable == nil || p.Reachable == nil || *p.BuildOnly || *p.SwarmManager || *p.SwarmWorker || !*p.Usable || !*p.Reachable {
		return target, nil, errors.New("verify a reachable native Docker deployment server, not a build-only or Swarm server")
	}
	if err := pre.VerifyHost(ctx, *selected); err != nil {
		return target, nil, errors.New("native data and Coolify must be verified on the same Docker engine before public exposure")
	}
	return target, client, nil
}

func domainConfigurationDigest(v dataDomainRecord, key string) string {
	b, _ := json.Marshal([]string{v.ProjectID, v.StableTenant, v.TargetFingerprint, v.ServerUUID, v.Owner, v.ApplicationNetwork, v.NetworkID, v.DockerEngineID, v.Domain, v.AppOrigin, key})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func requestFromDataDomain(v dataDomainRecord) PublicDataDomainRequest {
	return PublicDataDomainRequest{ProjectID: v.ProjectID, StableTenant: v.StableTenant, TargetID: v.TargetID, ServerUUID: v.ServerUUID, ProviderProjectUUID: v.ProviderProjectUUID, Environment: v.Environment, DestinationUUID: v.DestinationUUID, Domain: v.Domain, AppOrigin: v.AppOrigin, AcknowledgePublic: true, ExpectedRevision: v.Revision}
}

func dataDomainHostMatches(v dataDomainRecord, target DeploymentTarget, d PublicDataHost) bool {
	return v.TargetID == target.ID && v.ServerUUID == target.ServerUUID && v.TargetFingerprint == targetFingerprint(target) && v.ProjectID == d.ProjectID && v.StableTenant == d.StableTenant && v.ApplicationNetwork == d.ApplicationNetwork && v.NetworkID == d.NetworkID && v.DockerEngineID == d.DockerEngineID
}

func (s *Server) domainOutcome(v dataDomainRecord, status, message string) (PublicDataDomainBinding, error) {
	v.Status, v.Message, v.CheckedAt = status, message, time.Now().UTC().Format(time.RFC3339)
	if err := s.saveDataDomain(v); err != nil {
		return v.PublicDataDomainBinding, errors.New("public ingress outcome could not be saved; inspect it before retrying")
	}
	return v.PublicDataDomainBinding, nil
}

func (s *Server) dataDomainProject(ctx context.Context, client *CoolifyClient, v *dataDomainRecord) error {
	if v.ProviderProjectUUID != "" {
		if !coolifyID.MatchString(v.ProviderProjectUUID) {
			return errors.New("invalid verified provider project")
		}
		var observed CoolifyProject
		if err := client.request(ctx, "GET", "/projects/"+v.ProviderProjectUUID, nil, nil, &observed); err != nil {
			return err
		}
		if observed.UUID != v.ProviderProjectUUID {
			return errors.New("the provider returned a different project")
		}
		return nil
	}
	name, marker := "luci-data-"+v.Owner, "Luci data project "+v.Owner
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return err
	}
	var found string
	for _, p := range projects {
		if p.Name == name || p.Description == marker {
			if p.Name != name || p.Description != marker || !coolifyID.MatchString(p.UUID) || found != "" {
				return errors.New("the data provider project ownership is ambiguous")
			}
			found = p.UUID
		}
	}
	if found != "" {
		v.ProviderProjectUUID = found
		return s.saveDataDomain(*v)
	}
	if v.ProjectAttempted {
		return errors.New("the provider project creation outcome is uncertain; inspect the recorded attempt before retrying")
	}
	v.ProjectAttempted = true
	if err := s.saveDataDomain(*v); err != nil {
		return err
	}
	var created CoolifyProject
	mutationErr := client.request(ctx, "POST", "/projects", nil, map[string]string{"name": name, "description": marker}, &created)
	// Never repeat creation after a lost/malformed response. Reconcile by the
	// pre-persisted unpredictable owner marker, including after a restart.
	projects, err = client.ListProjects(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.Name == name || p.Description == marker {
			if p.Name != name || p.Description != marker || !coolifyID.MatchString(p.UUID) || found != "" {
				return errors.New("the created provider project could not be uniquely confirmed")
			}
			found = p.UUID
		}
	}
	if found == "" {
		if mutationErr != nil {
			return mutationErr
		}
		return errors.New("the provider project was not confirmed; inspect before retrying")
	}
	if created.UUID != "" && created.UUID != found {
		return errors.New("the provider project acknowledgement did not match readback")
	}
	v.ProviderProjectUUID = found
	return s.saveDataDomain(*v)
}

func (s *Server) reconcileDataService(ctx context.Context, client *CoolifyClient, v *dataDomainRecord) (coolifyService, error) {
	if v.ProviderServiceUUID != "" {
		observed, err := client.dataService(ctx, v.ProviderServiceUUID)
		if err != nil {
			return observed, err
		}
		return observed, ownedDataService(*v, observed, true)
	}
	list, err := client.dataServices(ctx)
	if err != nil {
		return coolifyService{}, err
	}
	var found *coolifyService
	for i := range list {
		if list[i].Name == dataServiceName(*v) || list[i].Description == dataServiceMarker(*v) {
			if found != nil {
				return coolifyService{}, errors.New("multiple provider services claim this public ingress")
			}
			found = &list[i]
		}
	}
	if found == nil {
		return coolifyService{}, os.ErrNotExist
	}
	if err := ownedDataServiceConfiguration(*v, *found, false); err != nil {
		return coolifyService{}, err
	}
	v.ProviderServiceUUID = found.UUID
	if err := s.saveDataDomain(*v); err != nil {
		return coolifyService{}, err
	}
	// GET includes applications; the list endpoint does not. Confirm URL as
	// part of this exact resource's readback before considering activation.
	return client.dataService(ctx, v.ProviderServiceUUID)
}

// PublishDataDomain is an explicit, serialized operation. Mutation intent is
// durable before every create/activation; GET refresh never repeats mutations.
func (s *Server) PublishDataDomain(ctx context.Context, req PublicDataDomainRequest, pre PublicDataDomainPrerequisites) (PublicDataDomainBinding, error) {
	if !req.AcknowledgePublic {
		return PublicDataDomainBinding{}, errors.New("acknowledge public exposure before creating a data domain")
	}
	domain, err := publicDataOrigin(req.Domain)
	if err != nil {
		return PublicDataDomainBinding{}, err
	}
	app, err := publicDataOrigin(req.AppOrigin)
	if err != nil {
		return PublicDataDomainBinding{}, err
	}
	if domain == app {
		return PublicDataDomainBinding{}, errors.New("use a separate data API domain from the application domain")
	}
	if req.Environment == "" {
		req.Environment = "production"
	}
	if len(req.Environment) > 100 || strings.TrimSpace(req.Environment) != req.Environment || strings.ContainsAny(req.Environment, "\x00\r\n") || (req.DestinationUUID != "" && !coolifyID.MatchString(req.DestinationUUID)) {
		return PublicDataDomainBinding{}, errors.New("invalid provider environment or destination")
	}
	unlock := s.dataDomainLock(req.ProjectID)
	defer unlock()
	target, client, err := s.dataDomainPrerequisites(ctx, req, pre)
	if err != nil {
		return PublicDataDomainBinding{}, err
	}
	v, err := s.readDataDomain(req.ProjectID)
	fresh := errors.Is(err, os.ErrNotExist)
	if err != nil && !fresh {
		return PublicDataDomainBinding{}, err
	}
	if fresh {
		if req.ExpectedRevision != 0 {
			return PublicDataDomainBinding{}, errors.New("the public data configuration revision changed")
		}
		v = dataDomainRecord{PublicDataDomainBinding: PublicDataDomainBinding{ProjectID: req.ProjectID, StableTenant: req.StableTenant, TargetID: req.TargetID, TargetFingerprint: targetFingerprint(target), ServerUUID: req.ServerUUID, ProviderProjectUUID: req.ProviderProjectUUID, Domain: domain, AppOrigin: app, ApplicationNetwork: pre.Data.ApplicationNetwork, NetworkID: pre.Data.NetworkID, DockerEngineID: pre.Data.DockerEngineID, ProbePath: "/__luci/data-health", Revision: 1, Status: "uncertain"}, Owner: newID("ingress-"), Environment: req.Environment, DestinationUUID: req.DestinationUUID}
	} else {
		if !dataDomainHostMatches(v, target, pre.Data) || v.Environment != req.Environment || v.DestinationUUID != req.DestinationUUID || (req.ProviderProjectUUID != "" && req.ProviderProjectUUID != v.ProviderProjectUUID) {
			return v.PublicDataDomainBinding, errors.New("public data ownership or host changed; no provider mutation was made")
		}
	}
	desired := v
	desired.Domain, desired.AppOrigin = domain, app
	desired.ConfigurationDigest = domainConfigurationDigest(desired, pre.Data.PublishableKey)
	compose, err := dataIngressCompose(desired, pre.Data.PublishableKey)
	if err != nil {
		return v.PublicDataDomainBinding, err
	}
	desired.ComposeHash, err = dataComposeHash(compose)
	if err != nil {
		return v.PublicDataDomainBinding, err
	}
	changed := !fresh && desired.ComposeHash != v.ComposeHash
	if changed {
		if req.ExpectedRevision != v.Revision || v.ConfirmedHash != v.ComposeHash || v.StopRequested {
			return v.PublicDataDomainBinding, errors.New("confirm the current public ingress outcome and revision before changing it")
		}
		if _, err := s.reconcileDataService(ctx, client, &v); err != nil {
			return s.domainOutcome(v, "attention", "The existing provider ingress could not be safely confirmed. No update was made.")
		}
		desired.Revision++
		desired.ActivationAttempted = false
		desired.Status = "uncertain"
	}
	v = desired
	if err := s.saveDataDomain(v); err != nil {
		return v.PublicDataDomainBinding, err
	}
	if err := s.dataDomainProject(ctx, client, &v); err != nil {
		return s.domainOutcome(v, "uncertain", "The provider project outcome could not be confirmed. Refresh before retrying; no creation will be repeated.")
	}
	observed, observeErr := s.reconcileDataService(ctx, client, &v)
	if errors.Is(observeErr, os.ErrNotExist) {
		if v.CreateAttempted {
			return s.domainOutcome(v, "uncertain", "The ingress creation outcome is uncertain. Its saved attempt will not be repeated.")
		}
		v.CreateAttempted = true
		if err := s.saveDataDomain(v); err != nil {
			return v.PublicDataDomainBinding, err
		}
		id, mutationErr := client.createDataService(ctx, v, compose)
		if id != "" {
			v.ProviderServiceUUID = id
			if err := s.saveDataDomain(v); err != nil {
				return v.PublicDataDomainBinding, err
			}
		}
		observed, observeErr = s.reconcileDataService(ctx, client, &v)
		if observeErr != nil || (mutationErr != nil && v.ProviderServiceUUID == "") {
			return s.domainOutcome(v, "uncertain", "The provider did not confirm ingress creation. Refresh checks the existing attempt without creating another.")
		}
	} else if observeErr != nil {
		return s.domainOutcome(v, "attention", "Provider ingress ownership could not be confirmed. No mutation was made.")
	}
	if err := ownedDataService(v, observed, true); err != nil {
		return s.domainOutcome(v, "attention", "Provider ingress ownership or domain changed. No mutation was made.")
	}
	observedHash, _ := dataComposeHash(observed.Compose)
	if observedHash != v.ComposeHash {
		_ = client.updateDataService(ctx, v, compose)
		observed, err = client.dataService(ctx, v.ProviderServiceUUID)
		if err != nil || ownedDataService(v, observed, false) != nil {
			return s.domainOutcome(v, "uncertain", "The ingress configuration update was not confirmed. Refresh before retrying.")
		}
	}
	v.ConfirmedHash = v.ComposeHash
	if v.StopRequested {
		v.StopRequested = false
		v.ActivationAttempted = false
	}
	// Only an explicit retry at the current revision may retry a previously
	// observed stopped activation. A refresh or duplicate initial request cannot.
	if v.ActivationAttempted && req.ExpectedRevision == v.Revision && v.Status == "pending" && (strings.HasPrefix(observed.Status, "stopped") || strings.HasPrefix(observed.Status, "exited")) {
		v.ActivationAttempted = false
	}
	if !v.ActivationAttempted {
		v.ActivationAttempted = true
		if err := s.saveDataDomain(v); err != nil {
			return v.PublicDataDomainBinding, err
		}
		if err := client.activateDataService(ctx, v.ProviderServiceUUID, changed || strings.HasPrefix(observed.Status, "running")); err != nil {
			return s.domainOutcome(v, "uncertain", "The ingress activation outcome is uncertain. Refresh observes it without restarting it.")
		}
	}
	return s.observeDataDomain(ctx, v, client, pre)
}

func (s *Server) observeDataDomain(ctx context.Context, v dataDomainRecord, client *CoolifyClient, pre PublicDataDomainPrerequisites) (PublicDataDomainBinding, error) {
	observed, err := s.reconcileDataService(ctx, client, &v)
	if err != nil || ownedDataService(v, observed, false) != nil {
		return s.domainOutcome(v, "attention", "The exact owned ingress could not be confirmed; public readiness is not established.")
	}
	v.ConfirmedHash = v.ComposeHash
	if v.StopRequested {
		if strings.HasPrefix(observed.Status, "exited") || strings.HasPrefix(observed.Status, "stopped") {
			return s.domainOutcome(v, "stopped", "Public ingress is stopped. Native data and its private connections are retained.")
		}
		return s.domainOutcome(v, "pending", "The provider is stopping the owned ingress. Refresh confirms the outcome.")
	}
	if !strings.HasPrefix(observed.Status, "running") || strings.Contains(observed.Status, "unhealthy") {
		return s.domainOutcome(v, "pending", "Ingress is not yet running. Refresh checks provider state and public HTTPS.")
	}
	if err := pre.VerifyHTTPS(ctx, v.PublicDataDomainBinding); err != nil {
		return s.domainOutcome(v, "pending", "Provider ingress is running; public DNS, certificate, current configuration and client API health are not yet verified.")
	}
	return s.domainOutcome(v, "verifiedHTTPS", "The owned data domain, current ingress configuration and public client APIs passed HTTPS checks.")
}

func (s *Server) RefreshDataDomain(ctx context.Context, id string, pre PublicDataDomainPrerequisites) (PublicDataDomainBinding, error) {
	unlock := s.dataDomainLock(id)
	defer unlock()
	v, err := s.readDataDomain(id)
	if err != nil {
		return PublicDataDomainBinding{}, err
	}
	target, client, err := s.dataDomainPrerequisites(ctx, requestFromDataDomain(v), pre)
	if err != nil {
		return s.domainOutcome(v, "attention", "Host co-location or selected target could not be confirmed. No provider mutation was made.")
	}
	if !dataDomainHostMatches(v, target, pre.Data) || domainConfigurationDigest(v, pre.Data.PublishableKey) != v.ConfigurationDigest {
		return s.domainOutcome(v, "attention", "The active data identity, network or client key changed. Reconcile the owned ingress before public use.")
	}
	if v.ProviderServiceUUID == "" {
		if _, err := s.reconcileDataService(ctx, client, &v); err != nil {
			return s.domainOutcome(v, "uncertain", "The saved provider attempt is not yet uniquely confirmed. No creation was repeated.")
		}
	}
	return s.observeDataDomain(ctx, v, client, pre)
}

func (s *Server) StopDataDomain(ctx context.Context, id string, pre PublicDataDomainPrerequisites) (PublicDataDomainBinding, error) {
	unlock := s.dataDomainLock(id)
	defer unlock()
	v, err := s.readDataDomain(id)
	if err != nil {
		return PublicDataDomainBinding{}, err
	}
	target, client, err := s.dataDomainPrerequisites(ctx, requestFromDataDomain(v), pre)
	if err != nil || !dataDomainHostMatches(v, target, pre.Data) {
		return v.PublicDataDomainBinding, errors.New("verify the selected data host before stopping its ingress")
	}
	observed, err := s.reconcileDataService(ctx, client, &v)
	if err != nil || ownedDataService(v, observed, true) != nil {
		return s.domainOutcome(v, "attention", "The provider resource no longer matches its owned ingress. No stop was made.")
	}
	if strings.HasPrefix(observed.Status, "exited") || strings.HasPrefix(observed.Status, "stopped") {
		v.StopRequested = true
		return s.domainOutcome(v, "stopped", "Public ingress is stopped; native data resources are retained.")
	}
	if !v.StopRequested {
		v.StopRequested = true
		v.Status = "uncertain"
		if err := s.saveDataDomain(v); err != nil {
			return v.PublicDataDomainBinding, err
		}
		if err := client.stopDataService(ctx, v.ProviderServiceUUID); err != nil {
			return s.domainOutcome(v, "uncertain", "The ingress stop outcome is uncertain. Refresh observes it; no native data resource was removed.")
		}
	}
	return s.observeDataDomain(ctx, v, client, pre)
}
