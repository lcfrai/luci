package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type domainFake struct {
	s                                       *Service
	f                                       *dataFake
	server                                  DataDomainServer
	engine, network, digest, socket         string
	localServer                             DataDomainServer
	calls, providerPublishes, providerStops int
	receipt                                 DataDomainReceipt
	failNextPromotion                       bool
}

func newDomainFake(t *testing.T) (*Service, *domainFake) {
	t.Helper()
	s, f := newDataFake(t)
	_ = s.changeData("owned", func(v *DataStatus) {
		v.Operation = &DataOperation{ID: "fixture-operation", Action: "install", Status: "running"}
	})
	if e := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "install"}); e != nil {
		t.Fatal(e)
	}
	_ = s.changeData("owned", func(v *DataStatus) { v.Operation.Status = "succeeded" })
	var installation dataInstallation
	_ = s.read("owned-data-installation", &installation)
	api := f.containers[s.dataName(installation, "api-gw")]
	api.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"8000/tcp": {{HostIP: "127.0.0.1", HostPort: fmt.Sprint(installation.APIPort)}}}
	f.containers[s.dataName(installation, "api-gw")] = api
	d := &domainFake{s: s, f: f, engine: "native-engine", network: strings.Repeat("a", 64), digest: "sha256:79d5c0443696f2fcb280a4d5a15fcc091a51784e566ab212889a08e424d8187c", socket: "/var/run/docker.sock", server: DataDomainServer{UUID: "server-local", IP: "host.docker.internal", Usable: true, Reachable: true}}
	d.localServer = d.server
	s.nativeSocket = d.socket
	s.opts.DockerHost = "unix://" + d.socket
	s.opts.Runner = d.run
	s.opts.DataDomains = &DataDomainProvider{
		Preflight: func(ctx context.Context, req DataDomainRequest, h DataDomainHost) error {
			return s.VerifyDataDomainHost(ctx, "owned", h, d.server)
		},
		Publish: func(ctx context.Context, req DataDomainRequest, h DataDomainHost) (DataDomainReceipt, error) {
			d.providerPublishes++
			if e := s.VerifyDataDomainHost(ctx, "owned", h, d.server); e != nil {
				return DataDomainReceipt{}, e
			}
			revision := d.receipt.Revision + 1
			d.receipt = DataDomainReceipt{ProjectID: h.ProjectID, StableTenant: h.StableTenant, TargetID: req.TargetID, TargetFingerprint: "target-fingerprint", ServerUUID: d.server.UUID, ProviderProjectUUID: "provider-project", ProviderServiceUUID: "owned-ingress", Domain: req.Domain, AppOrigin: req.AppOrigin, ApplicationNetwork: h.ApplicationNetwork, NetworkID: h.NetworkID, DockerEngineID: h.DockerEngineID, Status: "verifiedHTTPS", Message: "Verified test provider receipt.", Revision: revision, ConfigurationDigest: dataSum([]byte(h.PublishableKey)), ProbePath: "/__luci/data-health"}
			if d.failNextPromotion {
				d.failNextPromotion = false
				d.receipt.Status = "pending"
			}
			return d.receipt, nil
		},
		Refresh: func(ctx context.Context, id string, h DataDomainHost) (DataDomainReceipt, error) {
			if d.receipt.ConfigurationDigest != dataSum([]byte(h.PublishableKey)) {
				return d.receipt, errors.New("old key policy")
			}
			if d.failNextPromotion {
				d.failNextPromotion = false
				result := d.receipt
				result.Status = "pending"
				return result, nil
			}
			return d.receipt, nil
		},
		Stop: func(ctx context.Context, id string, h DataDomainHost) (DataDomainReceipt, error) {
			d.providerStops++
			if e := s.VerifyDataDomainHost(ctx, id, h, d.server); e != nil {
				return d.receipt, e
			}
			d.receipt.Status = "stopped"
			return d.receipt, nil
		},
	}
	return s, d
}

func (d *domainFake) run(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
	d.calls++
	encode := func(v any) []byte { b, _ := json.Marshal(v); return b }
	if exe == "docker" {
		switch args[0] {
		case "info":
			return encode(map[string]any{"ID": d.engine, "OSType": "linux", "NCPU": 8, "MemTotal": int64(16 << 30), "Swarm": map[string]string{"LocalNodeState": "inactive"}}), nil
		case "ps":
			if len(args) == 2 {
				return []byte(strings.Repeat("c", 64)), nil
			}
			return d.f.run(ctx, exe, args, dir, input)
		case "network":
			if args[1] == "inspect" && len(args) == 3 {
				return encode([]any{map[string]any{"Id": d.network, "Name": args[2], "Labels": map[string]string{labelInstance: d.s.instance, labelProject: "owned", labelResource: "network"}}}), nil
			}
		case "inspect":
			if args[1] == strings.Repeat("c", 64) {
				if len(args) == 2 {
					return encode([]any{map[string]any{"HostConfig": map[string]any{"Memory": int64(512 << 20), "NanoCpus": int64(1e9)}}}), nil
				}
				return encode(map[string]any{"ID": args[1], "Image": "sha256:" + strings.Repeat("d", 64), "ConfiguredImage": dataCoolifyImage + ":4.3.23", "Running": true, "Mounts": []any{map[string]any{"Type": "bind", "Source": d.socket, "Destination": "/var/run/docker.sock", "RW": true}}}), nil
			}
			if len(args) > 2 {
				d.f.mu.Lock()
				attached := d.f.attachments[args[1]]
				d.f.mu.Unlock()
				out := map[string]any{}
				for network, yes := range attached {
					if yes {
						out[network] = map[string]any{"NetworkID": d.network, "Aliases": []string{"supabase-api"}}
					}
				}
				return encode(out), nil
			}
		case "image":
			return encode(map[string]any{"ID": "sha256:" + strings.Repeat("d", 64), "OS": "linux", "Architecture": "amd64", "RepoDigests": []string{dataCoolifyImage + "@" + d.digest}}), nil
		case "exec":
			if len(args) > 2 && args[2] == "php" {
				if len(args) != 7 || args[3] != "-r" || args[4] != dataCoolifyServerQuery || args[5] != "--" {
					return nil, errors.New("arbitrary PHP command")
				}
				return encode(d.localServer), nil
			}
		}
	}
	b, e := d.f.run(ctx, exe, args, dir, input)
	if e == nil && args[0] == "compose" {
		for n, arg := range args {
			if arg != "-f" || n+1 >= len(args) {
				continue
			}
			config, _ := os.ReadFile(args[n+1])
			var parsed struct {
				Services map[string]struct {
					ContainerName string   `yaml:"container_name"`
					Ports         []string `yaml:"ports"`
				}
			}
			_ = yaml.Unmarshal(config, &parsed)
			service := parsed.Services["api-gw"]
			if len(service.Ports) == 1 {
				parts := strings.Split(service.Ports[0], ":")
				if len(parts) == 3 {
					d.f.mu.Lock()
					v := d.f.containers[service.ContainerName]
					v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"8000/tcp": {{HostIP: parts[0], HostPort: parts[1]}}}
					d.f.containers[service.ContainerName] = v
					d.f.mu.Unlock()
				}
			}
		}
	}
	return b, e
}

func domainRequest() DataDomainRequest {
	return DataDomainRequest{Action: "publish", TargetID: "saved-cloud", Domain: "https://data.example.com", AppOrigin: "https://app.example.com", AcknowledgePublic: true}
}

func TestDataDomainHostRequiresActualPinnedLocalControllerModel(t *testing.T) {
	s, d := newDomainFake(t)
	ctx := context.Background()
	h, e := s.DataDomainHost(ctx, "owned")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.VerifyDataDomainHost(ctx, "owned", h, d.server); e != nil {
		t.Fatalf("actual native local proof: %v", e)
	}
	d.digest = "sha256:" + strings.Repeat("e", 64)
	if s.VerifyDataDomainHost(ctx, "owned", h, d.server) == nil {
		t.Fatal("arbitrary repo digest admitted")
	}
	d.digest = "sha256:79d5c0443696f2fcb280a4d5a15fcc091a51784e566ab212889a08e424d8187c"
	d.localServer.UUID = "foreign-api-server"
	if s.VerifyDataDomainHost(ctx, "owned", h, d.server) == nil {
		t.Fatal("foreign API local IP without matching local model admitted")
	}
	d.localServer = d.server
	d.socket = "/run/another-engine.sock"
	if s.VerifyDataDomainHost(ctx, "owned", h, d.server) == nil {
		t.Fatal("another Docker socket admitted")
	}
	d.socket = s.nativeSocket
	d.network = strings.Repeat("b", 64)
	if s.VerifyDataDomainHost(ctx, "owned", h, d.server) == nil {
		t.Fatal("replaced project network admitted")
	}
	d.network = h.NetworkID
	d.engine = "replacement-engine"
	if s.VerifyDataDomainHost(ctx, "owned", h, d.server) == nil {
		t.Fatal("another engine admitted")
	}
	d.engine = h.DockerEngineID
	remote := d.server
	remote.IP = "203.0.113.7"
	if s.VerifyDataDomainHost(ctx, "owned", h, remote) == nil {
		t.Fatal("remote server admitted from independent local controller")
	}
}

func TestDataDomainNativeAuthConfigurationAndCachedStatus(t *testing.T) {
	s, d := newDomainFake(t)
	ctx := context.Background()
	status, e := s.DataDomain(ctx, "owned", domainRequest())
	if e != nil || status.Status != "verifiedHTTPS" || status.PublicURL != "https://data.example.com" {
		t.Fatalf("publish %s %v", status.Status, e)
	}
	var active dataInstallation
	_ = s.read("owned-data-installation", &active)
	root, _ := s.dataRoot(active)
	env, _ := os.ReadFile(filepath.Join(root, "runtime.env"))
	for _, want := range []string{"SUPABASE_PUBLIC_URL=https://data.example.com", "API_EXTERNAL_URL=https://data.example.com/auth/v1", "SITE_URL=https://app.example.com", "ADDITIONAL_REDIRECT_URLS=https://app.example.com/**", "SMTP_HOST=", "ENABLE_EMAIL_AUTOCONFIRM=false"} {
		if !strings.Contains(string(env), want) {
			t.Fatalf("Auth configuration missing %s", strings.Split(want, "=")[0])
		}
	}
	before := d.calls
	w := request(t, s, "GET", "/v1/platform/projects/owned/data/domain", "admin", nil)
	if w.Code != 200 || d.calls != before || !strings.Contains(w.Body.String(), `"emailDelivery":"unconfigured"`) {
		t.Fatalf("GET must be cached and honest: %d calls %d", w.Code, d.calls-before)
	}
	secret, _ := s.dataCredentials(active)
	if strings.Contains(w.Body.String(), secret.ServiceRole) || strings.Contains(w.Body.String(), secret.JWT) || strings.Contains(w.Body.String(), secret.Password) {
		t.Fatal("private credentials in domain DTO")
	}
	bindings, e := s.dataBindings("owned")
	if e != nil || bindings["SUPABASE_PUBLIC_URL"] != status.PublicURL || bindings["SUPABASE_URL"] != "http://supabase-api:8000" {
		t.Fatal("public and private bindings were conflated")
	}
	snapshot, e := s.CloudDataSnapshot(ctx, "owned")
	if e != nil || !snapshot.DomainConfigured || snapshot.PublicURL != status.PublicURL || snapshot.PublishableKey != secret.Anon {
		t.Fatal("cloud snapshot did not bind exact active receipt")
	}
	if !s.claim("owned", 1) {
		t.Fatal("fixture claim")
	}
	if _, e = s.CloudDataSnapshot(ctx, "owned"); e == nil {
		t.Fatal("concurrent native operation admitted application release")
	}
	s.release("owned")
	secret.Anon = signDataJWT(strings.Repeat("changed-key", 8), "anon")
	_ = s.write(active.StackID+"-secret", secret)
	status, e = s.DataDomainStatus("owned")
	if e != nil || status.Status != "attention" || status.PublicURL != "" {
		t.Fatal("changed restore key kept a cached public URL")
	}
	if _, e = s.CloudDataSnapshot(ctx, "owned"); e == nil {
		t.Fatal("unverified changed key admitted to cloud app")
	}
}

func TestDataDomainStopWorksWithStoppedNativeDataWithoutRestart(t *testing.T) {
	s, d := newDomainFake(t)
	ctx := context.Background()
	status, e := s.DataDomain(ctx, "owned", domainRequest())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.runDataAction(ctx, "owned", dataActionRequest{Action: "stop"}); e != nil {
		t.Fatal(e)
	}
	d.f.mu.Lock()
	before := len(d.f.calls)
	d.f.mu.Unlock()
	status, e = s.DataDomain(ctx, "owned", DataDomainRequest{Action: "stop", ExpectedRevision: status.Revision})
	if e != nil || status.Status != "stopped" || status.PublicURL != "" || d.providerStops != 1 {
		t.Fatalf("owned stopped stack cannot stop ingress: %s %v", status.Status, e)
	}
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	for _, call := range d.f.calls[before:] {
		if call[0] == "docker" && call[1] == "compose" {
			t.Fatal("public stop restarted native services")
		}
	}
	for _, container := range d.f.containers {
		if container.State.Running {
			t.Fatal("stopped native data was started by ingress stop")
		}
	}
}

func TestDataDomainPOSTQueuesAndRejectsDuplicateMutation(t *testing.T) {
	s, d := newDomainFake(t)
	entered := make(chan struct{})
	resume := make(chan struct{})
	preflight := s.opts.DataDomains.Preflight
	s.opts.DataDomains.Preflight = func(ctx context.Context, r DataDomainRequest, h DataDomainHost) error {
		close(entered)
		select {
		case <-resume:
		case <-ctx.Done():
			return ctx.Err()
		}
		return preflight(ctx, r, h)
	}
	start := time.Now()
	w := request(t, s, "POST", "/v1/platform/projects/owned/data/domain", "admin", domainRequest())
	if w.Code != 202 || time.Since(start) > time.Second || !strings.Contains(w.Body.String(), `"status":"running"`) {
		t.Fatalf("domain mutation was not queued: %d", w.Code)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("preflight did not start")
	}
	before := d.calls
	w = request(t, s, "GET", "/v1/platform/projects/owned/data/domain", "admin", nil)
	if w.Code != 200 || d.calls != before || !strings.Contains(w.Body.String(), `"status":"pending"`) {
		t.Fatal("poll caused Docker/provider work")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/domain", "admin", domainRequest())
	if w.Code != 409 || d.providerPublishes != 0 {
		t.Fatal("duplicate mutation was admitted")
	}
	close(resume)
	s.wg.Wait()
	status, e := s.DataDomainStatus("owned")
	if e != nil || status.Status != "verifiedHTTPS" || status.Operation.Status != "succeeded" || d.providerPublishes != 1 {
		t.Fatal("queued operation did not record exact final outcome")
	}
}

func TestDataDomainRestorePreservesDomainChangesKeyPolicyAndRecoversFailedPromotion(t *testing.T) {
	s, d := newDomainFake(t)
	ctx := context.Background()
	_, e := s.DataDomain(ctx, "owned", domainRequest())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.runDataAction(ctx, "owned", dataActionRequest{Action: "backup"}); e != nil {
		t.Fatal(e)
	}
	before, _ := s.loadData("owned")
	backup := before.Backups[0].ID
	var original dataInstallation
	_ = s.read("owned-data-installation", &original)
	secret, _ := s.dataCredentials(original)
	secret.JWT = strings.Repeat("new-active-signing-key", 3)
	secret.Anon = signDataJWT(secret.JWT, "anon")
	secret.ServiceRole = signDataJWT(secret.JWT, "service_role")
	_ = s.write(original.StackID+"-secret", secret)
	// Explicitly reconcile this simulated active credential rotation first.
	state, _ := s.loadDataDomain("owned")
	state.KeyDigest = dataSum([]byte(secret.Anon))
	state.Receipt.ConfigurationDigest = state.KeyDigest
	d.receipt = state.Receipt
	_ = s.saveDomainState("owned", state)
	count := d.providerPublishes
	if e = s.runDataAction(ctx, "owned", dataActionRequest{Action: "restore", BackupID: backup}); e != nil {
		t.Fatal(e)
	}
	var restored dataInstallation
	_ = s.read("owned-data-installation", &restored)
	status, _ := s.DataDomainStatus("owned")
	if restored.StackID == original.StackID || restored.storageTenant() != original.storageTenant() || status.PublicURL != "https://data.example.com" || d.providerPublishes != count+1 {
		t.Fatal("restore did not retain domain and explicitly update old anon policy")
	}
	oldRestored := restored
	oldStatus := status
	d.failNextPromotion = true
	if e = s.runDataAction(ctx, "owned", dataActionRequest{Action: "restore", BackupID: backup}); e == nil {
		t.Fatal("failed public HTTPS promotion reported restore success")
	}
	_ = s.read("owned-data-installation", &restored)
	status, _ = s.DataDomainStatus("owned")
	if restored.StackID != oldRestored.StackID || status.Status != "verifiedHTTPS" || status.PublicURL != oldStatus.PublicURL {
		t.Fatalf("public/native rollback did not reinstate previous stack: %s", status.Status)
	}
}

func TestDataDomainPublicProbeRequiresTLSExactConfigAndClientCORS(t *testing.T) {
	digest := strings.Repeat("f", 64)
	key := signDataJWT(strings.Repeat("test-public-key", 4), "anon")
	wrongDigest, wrongCORS := false, false
	clientChecks := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__luci/data-health" {
			if r.Header.Get("Authorization") != "" {
				t.Error("project key sent to configuration probe")
			}
			if wrongDigest {
				io.WriteString(w, strings.Repeat("e", 64))
			} else {
				io.WriteString(w, digest)
			}
			return
		}
		clientChecks++
		if r.Header.Get("apikey") != key || r.Header.Get("Authorization") != "Bearer "+key || r.Header.Get("Origin") != "https://app.example.com" {
			t.Error("real project client headers missing")
		}
		if !wrongCORS {
			w.Header().Set("Access-Control-Allow-Origin", "https://app.example.com")
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	client := server.Client()
	transport := client.Transport.(*http.Transport).Clone()
	target := strings.TrimPrefix(server.URL, "https://")
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	client.Transport = transport
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	receipt := DataDomainReceipt{Domain: "https://example.com", AppOrigin: "https://app.example.com", ConfigurationDigest: digest, ProbePath: "/__luci/data-health"}
	if e := probeDataDomain(context.Background(), client, receipt, key); e != nil || clientChecks != 3 {
		t.Fatalf("real TLS client probe: %v", e)
	}
	wrongDigest = true
	before := clientChecks
	if probeDataDomain(context.Background(), client, receipt, key) == nil || clientChecks != before {
		t.Fatal("wrong running configuration digest accepted")
	}
	wrongDigest = false
	wrongCORS = true
	if probeDataDomain(context.Background(), client, receipt, key) == nil {
		t.Fatal("CORS failure accepted")
	}
	for _, address := range []string{"127.0.0.1", "10.1.2.3", "100.64.0.1", "169.254.169.254", "192.0.2.1", "::1", "fe80::1", "2001:db8::1"} {
		if publicDataIP(net.ParseIP(address)) {
			t.Fatalf("nonpublic DNS destination accepted: %s", address)
		}
	}
}
