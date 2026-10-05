package platform

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Configurations and their original licence are compiled into every platform
// binary, including the selected cloud worker. Images are installed separately.
//
//go:embed all:supabase_assets
var dataAssets embed.FS

type dataAssetManifest struct {
	Revision string `json:"revision"`
	Version  string `json:"version"`
	Files    []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	} `json:"files"`
	Images map[string]string `json:"images"`
}

var dataServiceNames = []string{"db", "studio", "api-gw", "auth", "rest", "realtime", "storage", "imgproxy", "meta", "functions", "supavisor"}
var dataMemory = map[string]int{"db": 1024, "studio": 512, "api-gw": 128, "auth": 256, "rest": 128, "realtime": 512, "storage": 256, "imgproxy": 128, "meta": 256, "functions": 512, "supavisor": 384}
var dataCPU = map[string]int{"db": 500, "studio": 250, "api-gw": 100, "auth": 100, "rest": 100, "realtime": 300, "storage": 150, "imgproxy": 100, "meta": 100, "functions": 150, "supavisor": 150}

func reviewedDataAssets() (dataAssetManifest, error) {
	var m dataAssetManifest
	b, e := dataAssets.ReadFile("supabase_assets/manifest.json")
	if e != nil {
		return m, e
	}
	if json.Unmarshal(b, &m) != nil || m.Revision != "564eab8ad7840b13324f68b1bfac074ef8d51c21" || len(m.Images) != 11 {
		return m, errors.New("invalid Supabase provenance")
	}
	for _, f := range m.Files {
		if filepath.ToSlash(filepath.Clean(f.Path)) != f.Path || strings.HasPrefix(f.Path, "../") || filepath.IsAbs(f.Path) {
			return m, errors.New("invalid asset path")
		}
		b, e = dataAssets.ReadFile("supabase_assets/" + f.Path)
		if e != nil || len(b) != f.Bytes || dataSum(b) != f.SHA256 {
			return m, errors.New("Supabase asset differs from retained provenance")
		}
	}
	for original, pin := range m.Images {
		if !strings.HasPrefix(pin, original+"@sha256:") || len(strings.TrimPrefix(pin, original+"@sha256:")) != 64 {
			return m, errors.New("invalid immutable image")
		}
	}
	return m, nil
}
func (s *Service) prepareDataStack(install dataInstallation, secret dataSecret) error {
	m, e := reviewedDataAssets()
	if e != nil {
		return e
	}
	root, e := s.dataRoot(install)
	if e != nil {
		return e
	}
	for _, f := range m.Files {
		if !strings.HasPrefix(f.Path, "volumes/") {
			continue
		}
		b, _ := dataAssets.ReadFile("supabase_assets/" + f.Path)
		// Retain original source in the compiled assets. These runtime changes
		// isolate projects and remove a floating dependency version.
		if f.Path == "volumes/api/envoy/cds.yaml" {
			b = []byte(strings.ReplaceAll(string(b), "realtime-dev.supabase-realtime", s.dataName(install, "realtime")))
		}
		if f.Path == "volumes/functions/main/index.ts" {
			b = []byte(strings.ReplaceAll(string(b), "jsr:@panva/jose@6", "../_vendor/jose/dist/webapi/index.js"))
		}
		if f.Path == "volumes/functions/hello/index.ts" {
			b = []byte("// Luci's dependency-free example handler. Authentication is enforced by the main worker.\nexport default { fetch: () => Response.json({ message: 'Hello from Luci Functions!' }) };\n")
		}
		if f.Path == "volumes/functions/deno.jsonc" {
			b = []byte("{\"imports\":{}}\n")
		}
		target := filepath.Join(root, filepath.FromSlash(f.Path))
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(target, b, 0644); e != nil {
			return e
		}
	}
	b, _ := dataAssets.ReadFile("supabase_assets/docker-compose.yml")
	var compose map[string]any
	if e = yaml.Unmarshal(b, &compose); e != nil {
		return e
	}
	compose["name"] = s.resource(install.ProjectID, install.StackID)
	services, ok := compose["services"].(map[string]any)
	if !ok || len(services) != 11 {
		return errors.New("unexpected Supabase services")
	}
	labels := map[string]string{labelInstance: s.instance, labelProject: install.ProjectID, labelResource: "data-" + install.StackID}
	for _, name := range dataServiceNames {
		service, ok := services[name].(map[string]any)
		if !ok {
			return errors.New("missing Supabase service")
		}
		image, _ := service["image"].(string)
		pin := m.Images[image]
		if pin == "" {
			return errors.New("unreviewed image")
		}
		service["image"] = pin
		service["container_name"] = s.dataName(install, name)
		service["labels"] = map[string]string{labelInstance: s.instance, labelProject: install.ProjectID, labelResource: "data-" + install.StackID + "-" + name}
		service["mem_limit"] = strconv.Itoa(dataMemory[name]) + "m"
		service["memswap_limit"] = strconv.Itoa(dataMemory[name]) + "m"
		service["cpus"] = float64(dataCPU[name]) / 1000
		service["pids_limit"] = 1024
		// At the lowest resource allocation, the inherited postgres-meta
		// healthcheck can exhaust its three five-second attempts before Node
		// has started. Compose then aborts even when the next check succeeds.
		// Preserve the upstream tests and intervals, allowing bounded startup
		// grace for every service, including image-inherited healthchecks.
		health, ok := service["healthcheck"].(map[string]any)
		if !ok {
			health = map[string]any{}
		}
		health["start_period"] = "120s"
		service["healthcheck"] = health
		delete(service, "ports")
		if name == "api-gw" {
			service["ports"] = []string{fmt.Sprintf("127.0.0.1:%d:8000", install.APIPort)}
		}
		if name == "realtime" {
			service["networks"] = map[string]any{"default": map[string]any{"aliases": []string{s.dataName(install, name)}}}
		}
		if name == "db" {
			volumes := service["volumes"].([]any)
			for i, v := range volumes {
				value, _ := v.(string)
				if strings.HasPrefix(value, "./volumes/db/data:") {
					volumes[i] = "postgres-data:/var/lib/postgresql/data"
				}
			}
			service["volumes"] = volumes
		}
		if name == "storage" || name == "imgproxy" {
			service["volumes"] = []string{"storage-data:/var/lib/storage"}
		}
		if name == "studio" {
			service["volumes"] = []string{"snippets:/app/snippets", "./volumes/functions:/app/edge-functions:ro"}
		}
	}
	volumes := map[string]any{}
	for _, name := range []string{"postgres-data", "storage-data", "db-config", "deno-cache", "snippets"} {
		volumes[name] = map[string]any{"name": s.resource(install.ProjectID, install.StackID+"-"+name), "labels": labels}
	}
	compose["volumes"] = volumes
	compose["networks"] = map[string]any{"default": map[string]any{"name": s.resource(install.ProjectID, install.StackID+"-net"), "labels": labels}}
	b, e = yaml.Marshal(compose)
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(root, "compose.yaml"), b, 0600); e != nil {
		return e
	}
	if e = protect(filepath.Join(root, "compose.yaml")); e != nil {
		return e
	}
	values := map[string]string{"POSTGRES_PASSWORD": secret.Password, "JWT_SECRET": secret.JWT, "ANON_KEY": secret.Anon, "SERVICE_ROLE_KEY": secret.ServiceRole, "DASHBOARD_USERNAME": "luci", "DASHBOARD_PASSWORD": secret.Dashboard, "SECRET_KEY_BASE": secret.SecretBase, "REALTIME_DB_ENC_KEY": secret.Realtime, "VAULT_ENC_KEY": secret.Vault, "PG_META_CRYPTO_KEY": secret.Crypto, "S3_PROTOCOL_ACCESS_KEY_ID": secret.S3ID, "S3_PROTOCOL_ACCESS_KEY_SECRET": secret.S3Secret, "SUPABASE_PUBLIC_URL": dataURL(install), "API_EXTERNAL_URL": dataURL(install) + "/auth/v1", "POSTGRES_HOST": "db", "POSTGRES_DB": "postgres", "POSTGRES_PORT": "5432", "POOLER_PROXY_PORT_TRANSACTION": "6543", "POOLER_DEFAULT_POOL_SIZE": "10", "POOLER_MAX_CLIENT_CONN": "100", "POOLER_TENANT_ID": install.StackID, "POOLER_DB_POOL_SIZE": "5", "STUDIO_DEFAULT_ORGANIZATION": "Luci", "STUDIO_DEFAULT_PROJECT": install.ProjectID, "OPENAI_API_KEY": "", "SITE_URL": "http://localhost:3000", "ADDITIONAL_REDIRECT_URLS": "", "JWT_EXPIRY": "3600", "DISABLE_SIGNUP": "false", "ENABLE_EMAIL_SIGNUP": "true", "ENABLE_EMAIL_AUTOCONFIRM": "false", "ENABLE_ANONYMOUS_USERS": "false", "ENABLE_PHONE_SIGNUP": "false", "ENABLE_PHONE_AUTOCONFIRM": "false", "SMTP_ADMIN_EMAIL": "", "SMTP_HOST": "", "SMTP_PORT": "587", "SMTP_USER": "", "SMTP_PASS": "", "SMTP_SENDER_NAME": "Luci", "MAILER_URLPATHS_CONFIRMATION": "/auth/v1/verify", "MAILER_URLPATHS_INVITE": "/auth/v1/verify", "MAILER_URLPATHS_RECOVERY": "/auth/v1/verify", "MAILER_URLPATHS_EMAIL_CHANGE": "/auth/v1/verify", "GLOBAL_S3_BUCKET": "luci", "REGION": "local", "STORAGE_TENANT_ID": install.storageTenant(), "FUNCTIONS_VERIFY_JWT": "true", "PGRST_DB_SCHEMAS": "public,graphql_public", "PGRST_DB_MAX_ROWS": "1000", "PGRST_DB_EXTRA_SEARCH_PATH": "public", "IMGPROXY_AUTO_WEBP": "true", "SUPABASE_PUBLISHABLE_KEY": "", "SUPABASE_SECRET_KEY": "", "JWT_JWKS": "", "ANON_KEY_ASYMMETRIC": "", "SERVICE_ROLE_KEY_ASYMMETRIC": ""}
	publicURL, e := s.browserDataURL(install, secret)
	if e != nil {
		return e
	}
	values["SUPABASE_PUBLIC_URL"] = publicURL
	values["API_EXTERNAL_URL"] = publicURL + "/auth/v1"
	file, e := s.envFile(values)
	if e != nil {
		return e
	}
	defer os.Remove(file)
	return os.Rename(file, filepath.Join(root, "runtime.env"))
}
func (s *Service) verifyDataOwnership(ctx context.Context, install dataInstallation, allowMissing bool) error {
	for _, name := range dataServiceNames {
		_, e := s.inspect(ctx, install.ProjectID, "data-"+install.StackID+"-"+name, s.dataName(install, name))
		if e != nil {
			if allowMissing {
				b, probe := s.docker(ctx, "inspect", s.dataName(install, name))
				if probe != nil && len(b) == 0 {
					continue
				}
			}
			return e
		}
	}
	for _, kind := range []string{"postgres-data", "storage-data", "db-config", "deno-cache", "snippets", "net"} {
		exe := "volume"
		if kind == "net" {
			exe = "network"
		}
		b, e := s.docker(ctx, exe, "inspect", s.resource(install.ProjectID, install.StackID+"-"+kind), "--format", "{{json .Labels}}")
		if e != nil {
			if allowMissing {
				continue
			}
			return e
		}
		var labels map[string]string
		if json.Unmarshal(b, &labels) != nil || labels[labelInstance] != s.instance || labels[labelProject] != install.ProjectID || labels[labelResource] != "data-"+install.StackID {
			return errors.New("data resource ownership mismatch")
		}
	}
	return nil
}
func (s *Service) attachDataNetwork(ctx context.Context, install dataInstallation) error {
	network, e := s.ensureNetwork(ctx, install.ProjectID)
	if e != nil {
		return e
	}
	for _, name := range []string{"api-gw", "db"} {
		container := s.dataName(install, name)
		if _, e = s.inspect(ctx, install.ProjectID, "data-"+install.StackID+"-"+name, container); e != nil {
			return e
		}
		b, e := s.docker(ctx, "inspect", container, "--format", "{{json .NetworkSettings.Networks}}")
		if e != nil {
			return e
		}
		var attached map[string]json.RawMessage
		if json.Unmarshal(b, &attached) != nil {
			return errors.New("invalid network response")
		}
		if _, ok := attached[network]; ok {
			continue
		}
		alias := "supabase-api"
		if name == "db" {
			alias = "supabase-db"
		}
		if _, e = s.docker(ctx, "network", "connect", "--alias", alias, network, container); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) refreshData(ctx context.Context, install dataInstallation, requireHealthy bool) error {
	secret, e := s.dataCredentials(install)
	if e != nil {
		return e
	}
	healthy := true
	allStopped := true
	services := []DataService{}
	for _, name := range dataServiceNames {
		info, e := s.inspect(ctx, install.ProjectID, "data-"+install.StackID+"-"+name, s.dataName(install, name))
		entry := DataService{Name: name, Status: "unavailable", Health: "unknown"}
		if e == nil {
			entry.Image = info.Config.Image
			entry.Status = "stopped"
			entry.Health = "not-running"
			if info.State.Running {
				allStopped = false
				entry.Status = "running"
				entry.Health = info.State.Health.Status
				if entry.Health == "" {
					entry.Health = "no-container-healthcheck"
				}
				if entry.Health == "unhealthy" || entry.Health == "starting" {
					healthy = false
				}
			} else {
				healthy = false
			}
		} else {
			healthy = false
		}
		services = append(services, entry)
	}
	if !allStopped {
		b, e := s.dataSQL(ctx, install, "SELECT 1;")
		if e != nil || strings.TrimSpace(string(b)) != "1" {
			healthy = false
		}
		for _, path := range []string{"/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
			if _, e = s.dataHTTP(ctx, install, secret, "GET", path, nil, ""); e != nil {
				healthy = false
			}
		}
	}
	status := "degraded"
	message := "One or more owned services did not pass health checks. Refresh after inspecting Docker capacity and image startup."
	if healthy {
		status = "ready"
		message = "Auth, REST, Storage and Realtime are running on this Docker host. New deployments receive private server bindings. Email delivery requires SMTP configuration; the local endpoint is not public hosting."
	}
	if allStopped {
		status = "stopped"
		message = "Data services are stopped. Their database and storage volumes are retained."
	}
	if e = s.changeData(install.ProjectID, func(v *DataStatus) {
		v.Status = status
		v.Message = message
		v.URL = dataURL(install)
		v.PublishableKey = secret.Anon
		v.Services = services
		v.CheckedAt = now()
		v.PriorStackRetained = len(install.Previous) > 0
		for k := range v.Capabilities {
			v.Capabilities[k] = healthy
		}
		v.Capabilities["restore"] = len(v.Backups) > 0
	}); e != nil {
		return e
	}
	if requireHealthy && !healthy {
		return &dataFailure{message}
	}
	return nil
}
func (s *Service) dataSQL(ctx context.Context, install dataInstallation, sql string) ([]byte, error) {
	if _, e := s.inspect(ctx, install.ProjectID, "data-"+install.StackID+"-db", s.dataName(install, "db")); e != nil {
		return nil, e
	}
	return s.opts.Runner(ctx, "docker", []string{"exec", "--interactive", s.dataName(install, "db"), "psql", "-U", "postgres", "-d", "postgres", "-At", "-v", "ON_ERROR_STOP=1"}, s.dir, []byte(sql))
}
func (s *Service) dataHTTP(ctx context.Context, install dataInstallation, secret dataSecret, method, path string, input []byte, contentType string) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("invalid service path")
	}
	req, e := http.NewRequestWithContext(ctx, method, dataURL(install)+path, strings.NewReader(string(input)))
	if e != nil {
		return nil, e
	}
	req.Header.Set("apikey", secret.ServiceRole)
	req.Header.Set("Authorization", "Bearer "+secret.ServiceRole)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, e := s.opts.HTTPClient.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	b, e := readDataResponse(res)
	if e != nil {
		return nil, e
	}
	return b, nil
}
