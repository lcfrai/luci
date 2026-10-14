package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// The provider adapter is trusted application wiring. No client can provide a
// Docker host, a key, a network, an upstream, or a provider ownership receipt.
type DataDomainRequest struct {
	Action              string `json:"action"`
	TargetID            string `json:"targetId"`
	Domain              string `json:"domain"`
	AppOrigin           string `json:"applicationOrigin"`
	AcknowledgePublic   bool   `json:"acknowledgePublic"`
	ExpectedRevision    int    `json:"expectedRevision"`
	ProjectID           string `json:"-"`
	StableTenant        string `json:"-"`
	ServerUUID          string `json:"-"`
	ProviderProjectUUID string `json:"-"`
	Environment         string `json:"-"`
	DestinationUUID     string `json:"-"`
}

type DataDomainHost struct {
	ProjectID          string
	StableTenant       string
	ApplicationNetwork string
	NetworkID          string
	DockerEngineID     string
	PublishableKey     string
	AllowStopped       bool // Trusted stop/recovery verification only; never JSON.
}

type DataDomainServer struct {
	UUID, IP                                                  string
	Usable, Reachable, BuildServer, SwarmManager, SwarmWorker bool
}

// DataDomainReceipt is persisted privately; it contains ownership identifiers
// and public configuration only, never signing or administrative credentials.
type DataDomainReceipt struct {
	ProjectID, StableTenant, TargetID, TargetFingerprint             string
	ServerUUID, ProviderProjectUUID, ProviderServiceUUID             string
	Domain, AppOrigin, ApplicationNetwork, NetworkID, DockerEngineID string
	Status, Message, CheckedAt, ConfigurationDigest, ProbePath       string
	Revision                                                         int
}

type DataDomainProvider struct {
	Preflight func(context.Context, DataDomainRequest, DataDomainHost) error
	Publish   func(context.Context, DataDomainRequest, DataDomainHost) (DataDomainReceipt, error)
	Refresh   func(context.Context, string, DataDomainHost) (DataDomainReceipt, error)
	Stop      func(context.Context, string, DataDomainHost) (DataDomainReceipt, error)
}

type DataDomainStatus struct {
	ProjectID           string         `json:"projectId"`
	Status              string         `json:"status"`
	Message             string         `json:"message"`
	PublicURL           string         `json:"publicUrl,omitempty"`
	Domain              string         `json:"domain,omitempty"`
	AppOrigin           string         `json:"applicationOrigin,omitempty"`
	TargetID            string         `json:"targetId,omitempty"`
	ServerUUID          string         `json:"serverUuid,omitempty"`
	ProviderServiceUUID string         `json:"providerServiceUuid,omitempty"`
	ConfigurationDigest string         `json:"configurationDigest,omitempty"`
	Revision            int            `json:"revision"`
	CheckedAt           string         `json:"checkedAt,omitempty"`
	EmailDelivery       string         `json:"emailDelivery"`
	Operation           *DataOperation `json:"operation,omitempty"`
}

type CloudDataSnapshot struct {
	Installed, DomainConfigured                               bool
	ProjectID, StableTenant, DomainStatus                     string
	TargetID, TargetFingerprint, ServerUUID                   string
	DomainRevision                                            int
	ConfigurationDigest, PublicURL, PublishableKey, AppOrigin string
	ApplicationNetwork, NetworkID, DockerEngineID             string
}

type dataDomainState struct {
	ProjectID, StableTenant               string
	Request                               DataDomainRequest
	Receipt                               DataDomainReceipt
	KeyDigest, Status, Message, CheckedAt string
	Revision                              int
	Operation                             *DataOperation
}

var dataDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var dataDomainName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)

func canonicalDataOrigin(v string) (string, error) {
	u, e := url.Parse(v)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", errors.New("choose a public HTTPS origin without a path or port")
	}
	host := strings.ToLower(u.Hostname())
	if !dataDomainName.MatchString(host) || net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.Contains(host, "..") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("choose a public DNS name")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", errors.New("invalid DNS name")
		}
	}
	return "https://" + host, nil
}

func (s *Service) loadDataDomain(id string) (dataDomainState, error) {
	v := dataDomainState{ProjectID: id, Status: "not-configured", Message: "Data remains private. Publish an owned HTTPS domain to connect a cloud application."}
	e := s.read(id+"-data-domain", &v)
	if errors.Is(e, os.ErrNotExist) {
		e = nil
	}
	if v.ProjectID != id {
		return v, errors.New("data domain project mismatch")
	}
	return v, e
}

// DataDomainStatus reads cached state only. A changed restore key invalidates a
// cached public receipt immediately, even before a provider reconciliation.
func (s *Service) DataDomainStatus(id string) (DataDomainStatus, error) {
	s.mu.Lock()
	v, e := s.loadDataDomain(id)
	s.mu.Unlock()
	if e != nil {
		return DataDomainStatus{}, e
	}
	result := DataDomainStatus{ProjectID: id, Status: v.Status, Message: v.Message, Domain: v.Request.Domain, AppOrigin: v.Request.AppOrigin, TargetID: v.Request.TargetID, ServerUUID: v.Receipt.ServerUUID, ProviderServiceUUID: v.Receipt.ProviderServiceUUID, ConfigurationDigest: v.Receipt.ConfigurationDigest, Revision: v.Revision, CheckedAt: v.CheckedAt, EmailDelivery: s.EmailDeliveryStatus(id), Operation: v.Operation}
	if v.Operation != nil && v.Operation.Status == "running" {
		s.mu.Lock()
		busy := s.busy[id]
		s.mu.Unlock()
		if !busy {
			copy := *v.Operation
			copy.Status = "interrupted"
			copy.Message = "Luci stopped during public domain setup. Refresh reconciles the exact owned ingress; no mutation is automatically repeated."
			result.Operation = &copy
			result.Status = "attention"
			result.Message = copy.Message
		}
	}
	if v.Status == "verifiedHTTPS" {
		var i dataInstallation
		if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id || i.storageTenant() != v.StableTenant {
			result.Status = "attention"
		} else if secret, err := s.dataCredentials(i); err != nil || dataSum([]byte(secret.Anon)) != v.KeyDigest {
			result.Status = "attention"
		} else if native, err := s.loadData(id); err != nil || native.Status != "ready" {
			result.Status = "attention"
		} else {
			result.PublicURL = v.Receipt.Domain
		}
		if result.Status != "verifiedHTTPS" {
			result.Message = "Public data needs a new verification after a data or credential change. The private stack is retained."
		}
	}
	return result, nil
}

func (s *Service) DataDomainBinding(id string) (DataDomainReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataDomain(id)
	return v.Receipt, e
}

func (s *Service) publicDataStatus(v DataStatus) DataStatus {
	domain, e := s.DataDomainStatus(v.ProjectID)
	if e == nil {
		v.PublicDomain = &domain
		v.PublicURL = domain.PublicURL
	}
	return v
}

// DataDomainHost derives its descriptor from owned, running native resources.
// It never creates a network or trusts a browser-supplied Docker identity.
func (s *Service) DataDomainHost(ctx context.Context, id string) (DataDomainHost, error) {
	return s.dataDomainHost(ctx, id, true)
}
func (s *Service) DataDomainOwnedHost(ctx context.Context, id string) (DataDomainHost, error) {
	return s.dataDomainHost(ctx, id, false)
}
func (s *Service) dataDomainHost(ctx context.Context, id string, requireHealthy bool) (DataDomainHost, error) {
	var i dataInstallation
	if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id || !identifier.MatchString(i.StackID) || !identifier.MatchString(i.storageTenant()) {
		return DataDomainHost{}, errors.New("install and start this project's data stack first")
	}
	if e := s.verifyDataOwnership(ctx, i, false); e != nil {
		return DataDomainHost{}, errors.New("native data ownership could not be verified")
	}
	for _, name := range dataServiceNames {
		v, e := s.inspect(ctx, id, "data-"+i.StackID+"-"+name, s.dataName(i, name))
		if e != nil || (requireHealthy && (!v.State.Running || v.State.Health.Status == "unhealthy" || v.State.Health.Status == "starting")) {
			return DataDomainHost{}, errors.New("native data services are not healthy")
		}
		if name == "api-gw" {
			address, e := loopbackURL(v, 8000)
			if e != nil || address != dataURL(i) {
				return DataDomainHost{}, errors.New("native data gateway does not own its private loopback port")
			}
		}
	}
	secret, e := s.dataCredentials(i)
	if e != nil {
		return DataDomainHost{}, errors.New("native data credentials are unavailable")
	}
	if requireHealthy {
		for _, endpoint := range []string{"/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
			if _, e = s.dataHTTP(ctx, i, secret, "GET", endpoint, nil, ""); e != nil {
				return DataDomainHost{}, errors.New("native data API health could not be verified")
			}
		}
	}
	network := s.resource(id, "net")
	b, e := s.docker(ctx, "network", "inspect", network)
	var networks []struct {
		Id, Name string
		Labels   map[string]string
	}
	if e != nil || json.Unmarshal(b, &networks) != nil || len(networks) != 1 || networks[0].Name != network || !dataDigestPattern.MatchString(networks[0].Id) || networks[0].Labels[labelInstance] != s.instance || networks[0].Labels[labelProject] != id || networks[0].Labels[labelResource] != "network" {
		return DataDomainHost{}, errors.New("project network ownership could not be verified")
	}
	b, e = s.docker(ctx, "inspect", s.dataName(i, "api-gw"), "--format", "{{json .NetworkSettings.Networks}}")
	var attached map[string]struct {
		NetworkID string
		Aliases   []string
	}
	if e != nil || json.Unmarshal(b, &attached) != nil || attached[network].NetworkID != networks[0].Id {
		return DataDomainHost{}, errors.New("native data gateway is not attached to the verified project network")
	}
	alias := false
	for _, v := range attached[network].Aliases {
		alias = alias || v == "supabase-api"
	}
	if !alias {
		return DataDomainHost{}, errors.New("native data gateway does not own the stable application alias")
	}
	engine, e := s.engineInfo(ctx)
	if e != nil || engine.OSType != "linux" || engine.ID == "" || len(engine.ID) > 128 || strings.ContainsAny(engine.ID, "\x00\r\n ") || engine.Swarm.LocalNodeState != "inactive" {
		return DataDomainHost{}, errors.New("a verified non-Swarm Linux Docker engine is required")
	}
	return DataDomainHost{ProjectID: id, StableTenant: i.storageTenant(), ApplicationNetwork: network, NetworkID: networks[0].Id, DockerEngineID: engine.ID, PublishableKey: secret.Anon, AllowStopped: !requireHealthy}, nil
}

// Official 4.3.23 index and platform manifests, fetched anonymously from GHCR
// and checked against their raw SHA256 on 2026-10-05. Tag alone is insufficient.
const dataCoolifyImage = "ghcr.io/coollabsio/coolify"

var dataCoolifyDigests = map[string]bool{
	"sha256:79d5c0443696f2fcb280a4d5a15fcc091a51784e566ab212889a08e424d8187c": true,
	"sha256:8c3b04a93f771699f62a3e4178ca0d712087a74413d61d3dccb6689a63ccdb49": true,
	"sha256:1ed918f4b173611aa03525be0214598d4bfed3ce91a212951ea470f0d5ea6d15": true,
}

func sameDataDomainHost(a, b DataDomainHost) bool {
	return a.ProjectID == b.ProjectID && a.StableTenant == b.StableTenant && a.ApplicationNetwork == b.ApplicationNetwork && a.NetworkID == b.NetworkID && a.DockerEngineID == b.DockerEngineID && a.PublishableKey == b.PublishableKey
}
func localDataServer(ip string) bool {
	p := net.ParseIP(ip)
	return ip == "host.docker.internal" || (p != nil && p.IsLoopback())
}

// This fixed read-only query binds the authenticated API's selected UUID to
// the actual local controller model. Matching IP addresses alone proves nothing.
const dataCoolifyServerQuery = `require '/var/www/html/vendor/autoload.php'; $app = require '/var/www/html/bootstrap/app.php'; $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap(); $s=App\Models\Server::where('uuid',$argv[1])->first(); if (!$s || !$s->settings) { exit(3); } $t=$s->settings; echo json_encode(['UUID'=>$s->uuid,'IP'=>$s->ip,'Usable'=>$t->is_usable,'Reachable'=>$t->is_reachable,'BuildServer'=>$t->is_build_server,'SwarmManager'=>$t->is_swarm_manager,'SwarmWorker'=>$t->is_swarm_worker]);`

func (s *Service) VerifyDataDomainHost(ctx context.Context, id string, expected DataDomainHost, server DataDomainServer) error {
	fail := errors.New("Public native data requires the selected Coolify controller and workspace on the same verified native Docker host. Move or connect the workspace to that host before publishing.")
	if !identifier.MatchString(server.UUID) || !localDataServer(server.IP) || !server.Usable || !server.Reachable || server.BuildServer || server.SwarmManager || server.SwarmWorker || s.nativeSocket == "" || path.Clean(s.nativeSocket) != s.nativeSocket {
		return fail
	}
	actual, e := s.dataDomainHost(ctx, id, !expected.AllowStopped)
	if e != nil || !sameDataDomainHost(actual, expected) {
		return fail
	}
	if s.opts.DockerHost == "" {
		b, e := s.docker(ctx, "context", "inspect", "--format", "{{json .Endpoints.docker.Host}}")
		var host string
		if e != nil || json.Unmarshal(b, &host) != nil || host != "unix://"+s.nativeSocket {
			return fail
		}
	}
	b, e := s.docker(ctx, "ps", "--quiet")
	if e != nil {
		return fail
	}
	ids := strings.Fields(string(b))
	if len(ids) > 128 {
		return fail
	}
	for _, container := range ids {
		if !dataDigestPattern.MatchString(container) && !regexp.MustCompile(`^[a-f0-9]{12}$`).MatchString(container) {
			continue
		}
		// Deliberately project only non-secret container metadata; Config.Env is
		// never read, logged, or returned by this verifier.
		b, e = s.docker(ctx, "inspect", container, "--format", `{"ID":{{json .Id}},"Image":{{json .Image}},"ConfiguredImage":{{json .Config.Image}},"Running":{{json .State.Running}},"Mounts":{{json .Mounts}}}`)
		var c struct {
			ID, Image, ConfiguredImage string
			Running                    bool
			Mounts                     []struct {
				Type, Source, Destination string
				RW                        bool
			}
		}
		if e != nil || json.Unmarshal(b, &c) != nil || !c.Running {
			continue
		}
		imageOK := c.ConfiguredImage == dataCoolifyImage+":4.3.23"
		if digest, ok := strings.CutPrefix(c.ConfiguredImage, dataCoolifyImage+"@"); ok {
			imageOK = dataCoolifyDigests[digest]
		}
		if !imageOK {
			continue
		}
		socketOK := false
		for _, m := range c.Mounts {
			socketOK = socketOK || (m.Type == "bind" && m.Source == s.nativeSocket && m.Destination == "/var/run/docker.sock" && m.RW)
		}
		if !socketOK {
			continue
		}
		b, e = s.docker(ctx, "image", "inspect", c.Image, "--format", `{"ID":{{json .Id}},"RepoDigests":{{json .RepoDigests}},"OS":{{json .Os}},"Architecture":{{json .Architecture}}}`)
		var image struct {
			ID, OS, Architecture string
			RepoDigests          []string
		}
		if e != nil || json.Unmarshal(b, &image) != nil || image.ID != c.Image || image.OS != "linux" || (image.Architecture != "amd64" && image.Architecture != "arm64") {
			continue
		}
		pinned := false
		for _, v := range image.RepoDigests {
			d, ok := strings.CutPrefix(v, dataCoolifyImage+"@")
			pinned = pinned || (ok && dataCoolifyDigests[d])
		}
		if !pinned {
			continue
		}
		b, e = s.docker(ctx, "exec", c.ID, "php", "-r", dataCoolifyServerQuery, "--", server.UUID)
		var model struct {
			UUID, IP                                                  string
			Usable, Reachable, BuildServer, SwarmManager, SwarmWorker *bool
		}
		if e != nil || len(b) > 4096 || json.Unmarshal(b, &model) != nil || model.Usable == nil || model.Reachable == nil || model.BuildServer == nil || model.SwarmManager == nil || model.SwarmWorker == nil {
			continue
		}
		local := DataDomainServer{UUID: model.UUID, IP: model.IP, Usable: *model.Usable, Reachable: *model.Reachable, BuildServer: *model.BuildServer, SwarmManager: *model.SwarmManager, SwarmWorker: *model.SwarmWorker}
		if local != server {
			continue
		}
		return nil
	}
	return fail
}

func (s *Service) CloudDataSnapshot(ctx context.Context, id string) (CloudDataSnapshot, error) {
	result := CloudDataSnapshot{ProjectID: id, DomainStatus: "not-configured"}
	var i dataInstallation
	if e := s.read(id+"-data-installation", &i); errors.Is(e, os.ErrNotExist) {
		return result, nil
	} else if e != nil || i.ProjectID != id {
		return result, errors.New("native data state is unavailable")
	}
	result.Installed = true
	status, e := s.DataDomainStatus(id)
	if e != nil {
		return result, e
	}
	result.DomainStatus = status.Status
	result.DomainConfigured = status.Status != "not-configured"
	if !result.DomainConfigured {
		return result, nil
	}
	if status.Status != "verifiedHTTPS" {
		return result, errors.New("public data domain requires verification before application deployment")
	}
	s.mu.Lock()
	native, nativeErr := s.loadData(id)
	busy := s.busy[id]
	s.mu.Unlock()
	if nativeErr != nil || busy || (native.Operation != nil && native.Operation.Status == "running") {
		return result, errors.New("finish the current native data operation before preparing an application release")
	}
	host, e := s.DataDomainHost(ctx, id)
	if e != nil {
		return result, e
	}
	s.mu.Lock()
	state, e := s.loadDataDomain(id)
	s.mu.Unlock()
	if e != nil || !receiptMatchesHost(state.Receipt, host) || dataSum([]byte(host.PublishableKey)) != state.KeyDigest {
		return result, errors.New("public data ownership or credentials changed; verify the domain again")
	}
	result.StableTenant = host.StableTenant
	result.TargetID = state.Receipt.TargetID
	result.TargetFingerprint = state.Receipt.TargetFingerprint
	result.ServerUUID = state.Receipt.ServerUUID
	result.DomainRevision = state.Receipt.Revision
	result.ConfigurationDigest = state.Receipt.ConfigurationDigest
	result.PublicURL = state.Receipt.Domain
	result.AppOrigin = state.Receipt.AppOrigin
	result.PublishableKey = host.PublishableKey
	result.ApplicationNetwork = host.ApplicationNetwork
	result.NetworkID = host.NetworkID
	result.DockerEngineID = host.DockerEngineID
	return result, nil
}

func receiptMatchesHost(r DataDomainReceipt, h DataDomainHost) bool {
	return r.ProjectID == h.ProjectID && r.StableTenant == h.StableTenant && r.ApplicationNetwork == h.ApplicationNetwork && r.NetworkID == h.NetworkID && r.DockerEngineID == h.DockerEngineID
}

// Public probes bypass all proxies and re-resolve only publicly routable DNS
// addresses at connection time. TLS validates the original hostname; redirects
// cannot carry even an anonymous project key to another origin.
var dataReservedNetworks = []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"}

func publicDataIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	for _, cidr := range dataReservedNetworks {
		_, n, _ := net.ParseCIDR(cidr)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
func dataPublicClient() *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 8 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil || port != "443" {
			return nil, errors.New("invalid public data destination")
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil || len(ips) == 0 {
			return nil, errors.New("public DNS is not ready")
		}
		for _, v := range ips {
			if !publicDataIP(v.IP) {
				return nil, errors.New("public data DNS must not resolve to a private address")
			}
		}
		var last error
		for _, v := range ips {
			c, e := dialer.DialContext(ctx, network, net.JoinHostPort(v.IP.String(), port))
			if e == nil {
				return c, nil
			}
			last = e
		}
		return nil, last
	}}
	return &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func (s *Service) VerifyDataDomainHTTPS(ctx context.Context, id string, r DataDomainReceipt) error {
	host, e := s.DataDomainHost(ctx, id)
	if e != nil || !receiptMatchesHost(r, host) {
		return errors.New("public data host changed")
	}
	return probeDataDomain(ctx, dataPublicClient(), r, host.PublishableKey)
}
func probeDataDomain(ctx context.Context, client *http.Client, r DataDomainReceipt, key string) error {
	domain, e := canonicalDataOrigin(r.Domain)
	if e != nil || domain != r.Domain || r.ProbePath != "/__luci/data-health" || !dataDigestPattern.MatchString(r.ConfigurationDigest) {
		return errors.New("public data probe configuration is invalid")
	}
	origin, e := canonicalDataOrigin(r.AppOrigin)
	if e != nil || origin != r.AppOrigin || key == "" {
		return errors.New("public data client configuration is invalid")
	}
	for _, endpoint := range []string{r.ProbePath, "/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
		req, e := http.NewRequestWithContext(ctx, "GET", domain+endpoint, nil)
		if e != nil {
			return e
		}
		if endpoint != r.ProbePath {
			req.Header.Set("apikey", key)
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Origin", origin)
		}
		response, e := client.Do(req)
		if e != nil {
			return errors.New("public data DNS, HTTPS or API health is not ready")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4097))
		response.Body.Close()
		if readErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 || (endpoint == r.ProbePath && len(body) > 4096) {
			return errors.New("public data API did not pass health checks")
		}
		if endpoint == r.ProbePath {
			// Accept the adapter's exact plain-text digest or one explicit JSON
			// field; arbitrary substring matches cannot verify a configuration.
			value := strings.TrimSpace(string(body))
			if value != r.ConfigurationDigest {
				var probe struct {
					ConfigurationDigest string `json:"configurationDigest"`
				}
				if json.Unmarshal(body, &probe) != nil || probe.ConfigurationDigest != r.ConfigurationDigest {
					return errors.New("public ingress is serving another configuration")
				}
			}
		} else if response.Header.Get("Access-Control-Allow-Origin") != origin {
			return errors.New("public data CORS did not authorize the application origin")
		}
	}
	return nil
}

func (s *Service) registerDataDomain(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/domain", func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.project(w, r)
		if !ok {
			return
		}
		v, e := s.DataDomainStatus(p.ID)
		if e != nil {
			problem(w, 500, "data_domain_state_unavailable", "Public data status could not be read.")
			return
		}
		respond(w, 200, v)
	})
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/domain", s.dataDomainAction)
}

func validateDataDomainRequest(in *DataDomainRequest) error {
	switch in.Action {
	case "publish":
		if !in.AcknowledgePublic {
			return errors.New("Acknowledge public exposure before publishing data. This permits client Auth, REST, Storage and Realtime APIs; administrative data access remains private.")
		}
		if !identifier.MatchString(in.TargetID) {
			return errors.New("Choose a saved deployment target.")
		}
		domain, e := canonicalDataOrigin(in.Domain)
		if e != nil {
			return errors.New("Use a public HTTPS data domain without a path or port.")
		}
		origin, e := canonicalDataOrigin(in.AppOrigin)
		if e != nil || origin == domain {
			return errors.New("Use a separate public HTTPS application origin.")
		}
		in.Domain, in.AppOrigin = domain, origin
	case "refresh", "stop":
	default:
		return errors.New("Choose publish, refresh or stop.")
	}
	if in.ExpectedRevision < 0 || in.ExpectedRevision >= 1_000_000_000 {
		return errors.New("The public data configuration revision is invalid.")
	}
	return nil
}

func (s *Service) beginDataDomain(id string, in DataDomainRequest, limit int) (dataDomainState, error) {
	if e := validateDataDomainRequest(&in); e != nil {
		return dataDomainState{}, &dataFailure{e.Error()}
	}
	provider := s.opts.DataDomains
	if provider == nil || provider.Preflight == nil || provider.Publish == nil || provider.Refresh == nil || provider.Stop == nil {
		return dataDomainState{}, &dataFailure{"Connect a supported Coolify deployment target on this workspace's native Docker host before publishing data."}
	}
	if !s.claim(id, limit) {
		return dataDomainState{}, &dataFailure{"A platform operation is already running for this project."}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prior, e := s.loadDataDomain(id)
	if e != nil || prior.Revision != in.ExpectedRevision || (in.Action != "publish" && prior.Status == "not-configured") {
		delete(s.busy, id)
		return prior, &dataFailure{"The public data configuration changed or is not configured. Refresh its status before retrying."}
	}
	queued := prior
	queued.Revision++
	queued.Status = "pending"
	queued.Message = "Public data operation accepted. Readiness requires owned provider resources and actual HTTPS client API checks."
	queued.Operation = &DataOperation{ID: randomID("data-domain-op-"), Action: in.Action, Status: "running", Stage: "queued", Message: queued.Message, CreatedAt: now(), UpdatedAt: now()}
	if e = s.write(id+"-data-domain", queued); e != nil {
		delete(s.busy, id)
		return prior, e
	}
	return prior, nil
}

func (s *Service) dataDomainAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	var in DataDomainRequest
	if !body(w, r, &in) {
		return
	}
	if e := validateDataDomainRequest(&in); e != nil {
		problem(w, 400, "invalid_data_domain", e.Error())
		return
	}
	prior, e := s.beginDataDomain(p.ID, in, operationLimit(r))
	if e != nil {
		problem(w, 409, "data_domain_not_admitted", dataError(e))
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(p.ID)
		ctx, cancel := context.WithTimeout(s.ctx, 8*time.Minute)
		defer cancel()
		s.finishDataDomain(p.ID, s.runDataDomain(ctx, p.ID, in, prior))
	}()
	v, _ := s.DataDomainStatus(p.ID)
	respond(w, 202, v)
}

// DataDomain is the synchronous trusted equivalent used by integration tests
// and internal callers; the HTTP route always queues bounded background work.
func (s *Service) DataDomain(ctx context.Context, id string, in DataDomainRequest) (DataDomainStatus, error) {
	if _, e := s.opts.ResolveProject(id); e != nil {
		return DataDomainStatus{}, e
	}
	if e := validateDataDomainRequest(&in); e != nil {
		return DataDomainStatus{}, e
	}
	prior, e := s.beginDataDomain(id, in, 1)
	if e != nil {
		return DataDomainStatus{}, e
	}
	defer s.release(id)
	e = s.runDataDomain(ctx, id, in, prior)
	s.finishDataDomain(id, e)
	v, readErr := s.DataDomainStatus(id)
	if e != nil {
		return v, e
	}
	return v, readErr
}

func (s *Service) domainStage(id, stage, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataDomain(id)
	if e != nil {
		return
	}
	v.Message = message
	if v.Operation != nil {
		v.Operation.Stage = stage
		v.Operation.Message = message
		v.Operation.UpdatedAt = now()
	}
	_ = s.write(id+"-data-domain", v)
}
func (s *Service) finishDataDomain(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataDomain(id)
	if e != nil {
		return
	}
	if err != nil {
		if v.Status == "pending" {
			v.Status = "attention"
		}
		v.Message = dataError(err)
	}
	if v.Operation != nil {
		v.Operation.Status = "succeeded"
		v.Operation.Stage = "complete"
		if err != nil {
			v.Operation.Status = "failed"
		}
		v.Operation.Message = v.Message
		v.Operation.UpdatedAt = now()
	}
	v.CheckedAt = now()
	_ = s.write(id+"-data-domain", v)
}
func (s *Service) saveDomainState(id string, state dataDomainState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, e := s.loadDataDomain(id)
	if e != nil {
		return e
	}
	state.Revision = current.Revision
	state.Operation = current.Operation
	state.CheckedAt = now()
	return s.write(id+"-data-domain", state)
}

func validateDomainReceipt(r DataDomainReceipt, h DataDomainHost, request DataDomainRequest) error {
	if !receiptMatchesHost(r, h) || r.Domain != request.Domain || r.AppOrigin != request.AppOrigin || r.TargetID != request.TargetID || !identifier.MatchString(r.ServerUUID) || !identifier.MatchString(r.ProviderServiceUUID) || !dataDigestPattern.MatchString(r.ConfigurationDigest) || r.ProbePath != "/__luci/data-health" || r.Revision < 1 {
		return errors.New("provider public data ownership or configuration was not confirmed")
	}
	switch r.Status {
	case "verifiedHTTPS", "pending", "uncertain", "attention", "stopped":
		return nil
	}
	return errors.New("provider public data status is invalid")
}

func (s *Service) configureDataDomain(ctx context.Context, id string, state dataDomainState) error {
	if e := s.saveDomainState(id, state); e != nil {
		return e
	}
	var i dataInstallation
	if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id {
		return errors.New("native data installation is unavailable")
	}
	secret, e := s.dataCredentials(i)
	if e != nil {
		return e
	}
	if e = s.prepareDataStack(i, secret); e != nil {
		return e
	}
	if _, e = s.compose(ctx, i, "up", "-d", "--wait", "--wait-timeout", "240"); e != nil {
		return e
	}
	return s.refreshData(ctx, i, true)
}

func (s *Service) runDataDomain(ctx context.Context, id string, in DataDomainRequest, prior dataDomainState) (err error) {
	s.domainStage(id, "verifying-host", "Verifying owned data services, Docker engine, project network and the selected deployment host.")
	host, e := s.dataDomainHost(ctx, id, in.Action != "stop")
	if e != nil {
		return &dataFailure{"Verify this project's owned native data services before changing public data access. Publishing and HTTPS verification require healthy services."}
	}
	provider := s.opts.DataDomains
	request := prior.Request
	if in.Action == "publish" {
		request = in
	}
	request.ProjectID = id
	request.StableTenant = host.StableTenant
	request.ExpectedRevision = prior.Receipt.Revision
	if e = provider.Preflight(ctx, request, host); e != nil {
		_ = s.saveDomainState(id, prior)
		return &dataFailure{"The selected deployment target could not be verified on this data stack's Docker host. Connect or move the workspace to that native host before publishing."}
	}
	current := prior
	current.ProjectID = id
	current.StableTenant = host.StableTenant
	current.Request = request
	current.Status = "pending"
	nativeChanged := false
	defer func() {
		if err == nil || !nativeChanged {
			return
		}
		// Recovery gets a fresh bounded context even when the failed operation
		// exhausted its deadline. Only exact owned resources are reinstated.
		recovery, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()
		recovered := s.configureDataDomain(recovery, id, prior) == nil
		if recovered && prior.Status == "verifiedHTTPS" {
			oldHost, e := s.DataDomainHost(recovery, id)
			if e == nil {
				oldReq := prior.Request
				oldReq.ProjectID = id
				oldReq.StableTenant = oldHost.StableTenant
				oldReq.AcknowledgePublic = true
				r, e := provider.Publish(recovery, oldReq, oldHost)
				recovered = e == nil && r.Status == "verifiedHTTPS" && validateDomainReceipt(r, oldHost, oldReq) == nil
				if recovered {
					prior.Receipt = r
					prior.KeyDigest = dataSum([]byte(oldHost.PublishableKey))
					_ = s.saveDomainState(id, prior)
				}
			} else {
				recovered = false
			}
		}
		if !recovered {
			current.Status = "attention"
			current.Message = "Public data setup failed and the prior public configuration could not be fully verified. Native resources and ownership receipts are retained; inspect and refresh before retrying."
			_ = s.saveDomainState(id, current)
			err = &dataFailure{current.Message}
		} else {
			err = &dataFailure{"Public data setup failed. The previous native configuration and any previously verified public connection were reinstated."}
		}
	}()
	var receipt DataDomainReceipt
	switch in.Action {
	case "publish":
		s.domainStage(id, "configuring-auth", "Configuring Auth with the data API domain and application redirect origin. Email delivery uses this project's applied SMTP settings.")
		nativeChanged = true
		if e = s.configureDataDomain(ctx, id, current); e != nil {
			return e
		}
		host, e = s.DataDomainHost(ctx, id)
		if e != nil {
			return e
		}
		s.domainStage(id, "publishing", "Creating or reconciling the exact owned public ingress and checking real HTTPS client APIs.")
		receipt, e = provider.Publish(ctx, request, host)
	case "refresh":
		s.domainStage(id, "checking-https", "Reading the owned ingress and checking DNS, HTTPS, the current configuration, Auth, REST, Storage and application CORS.")
		receipt, e = provider.Refresh(ctx, id, host)
	case "stop":
		s.domainStage(id, "stopping-ingress", "Stopping the verified owned public ingress while retaining native data and files.")
		receipt, e = provider.Stop(ctx, id, host)
	}
	if e != nil {
		return e
	}
	if e = validateDomainReceipt(receipt, host, request); e != nil {
		// Uncertain initial creation may legitimately lack a resource UUID.
		// Persist its outcome without pretending that an ingress is verified.
		if receipt.Status != "uncertain" || receipt.ProjectID != id {
			return e
		}
	}
	current.Receipt = receipt
	current.KeyDigest = dataSum([]byte(host.PublishableKey))
	current.Status = receipt.Status
	current.Message = receipt.Message
	if e = s.saveDomainState(id, current); e != nil {
		return e
	}
	if in.Action == "stop" && receipt.Status == "stopped" {
		var i dataInstallation
		if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id {
			return errors.New("native data installation is unavailable")
		}
		secret, e := s.dataCredentials(i)
		if e != nil {
			return e
		}
		// Write the future private configuration without starting, stopping or
		// restarting the native stack. Explicit Start applies it later.
		return s.prepareDataStack(i, secret)
	}
	return nil
}

// Candidate stack generation uses the saved stable public configuration, but
// never changes a provider route. The API alias moves only after native health.
func (s *Service) dataDomainConfiguration(i dataInstallation) (string, string, error) {
	s.mu.Lock()
	v, e := s.loadDataDomain(i.ProjectID)
	s.mu.Unlock()
	if e != nil {
		return "", "", e
	}
	if v.Status == "not-configured" || v.Status == "stopped" || v.Request.Domain == "" {
		return "", "", nil
	}
	if v.StableTenant != i.storageTenant() {
		return "", "", errors.New("public data tenant ownership changed")
	}
	domain, e := canonicalDataOrigin(v.Request.Domain)
	if e != nil || domain != v.Request.Domain {
		return "", "", errors.New("saved public data domain is invalid")
	}
	origin, e := canonicalDataOrigin(v.Request.AppOrigin)
	if e != nil || origin != v.Request.AppOrigin {
		return "", "", errors.New("saved public application origin is invalid")
	}
	return domain, origin, nil
}

func (s *Service) refreshPublishedDataDomain(ctx context.Context, id string, requireVerified bool) error {
	s.mu.Lock()
	state, e := s.loadDataDomain(id)
	s.mu.Unlock()
	if e != nil {
		return e
	}
	if state.Status == "not-configured" || state.Status == "stopped" {
		return nil
	}
	if s.opts.DataDomains == nil || s.opts.DataDomains.Refresh == nil {
		return &dataFailure{"The native data stack is healthy, but its public domain adapter is unavailable. Reconnect the deployment target and verify public access."}
	}
	host, e := s.DataDomainHost(ctx, id)
	if e != nil {
		return e
	}
	r, e := s.opts.DataDomains.Refresh(ctx, id, host)
	if e != nil {
		return e
	}
	if e = validateDomainReceipt(r, host, state.Request); e != nil {
		return e
	}
	state.Receipt = r
	state.Status = r.Status
	state.Message = r.Message
	state.KeyDigest = dataSum([]byte(host.PublishableKey))
	if e = s.saveDomainState(id, state); e != nil {
		return e
	}
	if requireVerified && r.Status != "verifiedHTTPS" {
		return &dataFailure{"Native data is healthy, but its public HTTPS connection did not pass verification."}
	}
	return nil
}

func (s *Service) prepareDataDomainRestore(ctx context.Context, id string) (dataDomainState, error) {
	s.mu.Lock()
	state, e := s.loadDataDomain(id)
	s.mu.Unlock()
	if e != nil {
		return state, e
	}
	if state.Status == "not-configured" || state.Status == "stopped" {
		return state, nil
	}
	if state.Status != "verifiedHTTPS" {
		return state, &dataFailure{"Verify or stop the public data ingress before restoring. An uncertain public configuration cannot be safely promoted."}
	}
	if s.opts.DataDomains == nil || s.opts.DataDomains.Preflight == nil || s.opts.DataDomains.Publish == nil || s.opts.DataDomains.Refresh == nil {
		return state, &dataFailure{"Reconnect this public data deployment target before restoring."}
	}
	host, e := s.DataDomainHost(ctx, id)
	if e != nil {
		return state, e
	}
	request := state.Request
	request.ProjectID = id
	request.StableTenant = host.StableTenant
	if e = s.opts.DataDomains.Preflight(ctx, request, host); e != nil {
		return state, &dataFailure{"The existing public data host could not be verified. No restore promotion was made."}
	}
	return state, nil
}

func (s *Service) promoteDataDomainRestore(ctx context.Context, id string, previous dataDomainState) error {
	if previous.Status == "not-configured" || previous.Status == "stopped" {
		return nil
	}
	host, e := s.DataDomainHost(ctx, id)
	if e != nil {
		return e
	}
	state := previous
	state.Status = "pending"
	state.Message = "Restored native data is healthy; verifying the existing public domain and current client key policy."
	if e = s.saveDomainState(id, state); e != nil {
		return e
	}
	request := previous.Request
	request.ProjectID = id
	request.StableTenant = host.StableTenant
	request.AcknowledgePublic = true
	request.ExpectedRevision = previous.Receipt.Revision
	var r DataDomainReceipt
	if previous.KeyDigest != dataSum([]byte(host.PublishableKey)) {
		// Key policy changes require explicit owned provider configuration,
		// never a GET refresh which could mask an old running proxy.
		r, e = s.opts.DataDomains.Publish(ctx, request, host)
	} else {
		r, e = s.opts.DataDomains.Refresh(ctx, id, host)
	}
	if e != nil {
		return e
	}
	if e = validateDomainReceipt(r, host, request); e != nil || r.Status != "verifiedHTTPS" {
		return &dataFailure{"Restored native data passed health checks, but the existing public domain or client key policy did not verify."}
	}
	state.Receipt = r
	state.Status = r.Status
	state.KeyDigest = dataSum([]byte(host.PublishableKey))
	state.Message = r.Message
	return s.saveDomainState(id, state)
}

func (s *Service) recoverDataDomainRestore(ctx context.Context, id string, previous dataDomainState) error {
	if previous.Status == "not-configured" || previous.Status == "stopped" {
		return s.saveDomainState(id, previous)
	}
	host, e := s.DataDomainHost(ctx, id)
	if e != nil {
		return e
	}
	request := previous.Request
	request.ProjectID = id
	request.StableTenant = host.StableTenant
	request.AcknowledgePublic = true
	r, e := s.opts.DataDomains.Publish(ctx, request, host)
	if e != nil || validateDomainReceipt(r, host, request) != nil || r.Status != "verifiedHTTPS" {
		previous.Status = "attention"
		previous.Message = "The previous native stack was reinstated, but its public domain and client policy still require verification."
		_ = s.saveDomainState(id, previous)
		return errors.New("previous public configuration could not be reinstated")
	}
	previous.Receipt = r
	previous.KeyDigest = dataSum([]byte(host.PublishableKey))
	previous.Status = r.Status
	previous.Message = r.Message
	return s.saveDomainState(id, previous)
}
