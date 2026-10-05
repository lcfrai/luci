package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DataStatus contains cached receipts and client-safe configuration only. A GET
// never starts Docker, downloads images, repairs a host, or returns an admin key.
type DataStatus struct {
	ProjectID          string          `json:"projectId"`
	Provider           string          `json:"provider"`
	Version            string          `json:"version"`
	Revision           int             `json:"revision"`
	Status             string          `json:"status"`
	Message            string          `json:"message"`
	URL                string          `json:"url,omitempty"`
	EndpointScope      string          `json:"endpointScope"`
	PublishableKey     string          `json:"publishableKey,omitempty"`
	Services           []DataService   `json:"services"`
	Operation          *DataOperation  `json:"operation,omitempty"`
	Backups            []DataBackup    `json:"backups"`
	Capabilities       map[string]bool `json:"capabilities"`
	Requirements       map[string]int  `json:"requirements"`
	CheckedAt          string          `json:"checkedAt,omitempty"`
	PriorStackRetained bool            `json:"priorStackRetained"`
}
type DataService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Health string `json:"health"`
	Image  string `json:"image"`
}
type DataOperation struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type DataBackup struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"createdAt"`
	Bytes           int64  `json:"bytes"`
	SHA256          string `json:"sha256"`
	IncludesStorage bool   `json:"includesStorage"`
}
type dataInstallation struct {
	ProjectID       string   `json:"projectId"`
	StackID         string   `json:"stackId"`
	StorageTenantID string   `json:"storageTenantId,omitempty"`
	APIPort         int      `json:"apiPort"`
	Previous        []string `json:"previous"`
}

func (i dataInstallation) storageTenant() string {
	if i.StorageTenantID != "" {
		return i.StorageTenantID
	}
	// Existing installations used their stack identity as the file namespace.
	return i.StackID
}

type dataSecret struct {
	Password    string `json:"password"`
	JWT         string `json:"jwt"`
	Anon        string `json:"anon"`
	ServiceRole string `json:"serviceRole"`
	Dashboard   string `json:"dashboard"`
	Realtime    string `json:"realtime"`
	Vault       string `json:"vault"`
	Crypto      string `json:"crypto"`
	SecretBase  string `json:"secretBase"`
	S3ID        string `json:"s3Id"`
	S3Secret    string `json:"s3Secret"`
}

func (s *Service) registerData(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/data", s.dataStatus)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data", s.dataAction)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/backups/{backupId}", s.downloadDataBackup)
	s.registerDataControls(mux)
}
func emptyData(id string) DataStatus {
	return DataStatus{ProjectID: id, Provider: "supabase", Version: "self-hosted/v0.8.2", EndpointScope: "selected-docker-host-loopback", Status: "not-installed", Message: "Optional Auth, REST, Storage, Realtime and Functions use a separate PostgreSQL 17 stack. Existing PostgreSQL 16 data is retained.", Services: []DataService{}, Backups: []DataBackup{}, Capabilities: map[string]bool{"auth": false, "rest": false, "storage": false, "realtime": false, "tables": false, "rowLevelSecurity": false, "functions": false, "backups": false, "restore": false}, Requirements: map[string]int{"memoryMiB": 4096, "cpuMillis": 2000, "diskGiB": 40}}
}
func (s *Service) loadData(id string) (DataStatus, error) {
	v := emptyData(id)
	e := s.read(id+"-data", &v)
	if errors.Is(e, os.ErrNotExist) {
		e = nil
	}
	if v.ProjectID != id {
		return v, errors.New("data project binding mismatch")
	}
	return v, e
}
func (s *Service) changeData(id string, f func(*DataStatus)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadData(id)
	if e != nil {
		return e
	}
	f(&v)
	return s.write(id+"-data", v)
}
func (s *Service) dataStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	v, e := s.loadData(p.ID)
	busy := s.busy[p.ID]
	s.mu.Unlock()
	if e != nil {
		problem(w, 500, "data_state_unavailable", "Data status could not be read.")
		return
	}
	if v.Operation != nil && v.Operation.Status == "running" && !busy {
		v.Operation.Status = "interrupted"
		v.Operation.Message = "Luci stopped during this operation. Refresh verifies retained resources; no operation is automatically repeated."
		v.Status = "interrupted"
		v.Message = v.Operation.Message
	}
	respond(w, 200, s.browserDataStatus(v))
}

type dataActionRequest struct {
	Action               string `json:"action"`
	AcknowledgeResources bool   `json:"acknowledgeResources"`
	BackupID             string `json:"backupId,omitempty"`
	ConfirmRestore       bool   `json:"confirmRestore,omitempty"`
}

func (s *Service) dataAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	var in dataActionRequest
	if !body(w, r, &in) {
		return
	}
	switch in.Action {
	case "install", "start", "stop", "refresh", "backup", "restore":
	default:
		problem(w, 400, "invalid_action", "Choose install, start, stop, refresh, backup or restore.")
		return
	}
	if (in.Action == "install" || in.Action == "start" || in.Action == "restore") && !in.AcknowledgeResources {
		problem(w, 400, "resource_acknowledgement_required", "This optional stack needs at least 4 GiB RAM, 2 CPU cores and 40 GiB SSD, in addition to other workloads. Acknowledge those resources before starting it.")
		return
	}
	if in.Action == "restore" && (!in.ConfirmRestore || !identifier.MatchString(in.BackupID)) {
		problem(w, 400, "restore_confirmation_required", "Choose a recorded backup and confirm that restoring replaces the active data stack. The previous stack is retained.")
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return
	}
	if e := s.changeData(p.ID, func(v *DataStatus) {
		v.Revision++
		if (in.Action == "install" || in.Action == "start") && v.Status != "ready" {
			v.Status = "installing"
		}
		v.Operation = &DataOperation{ID: randomID("data-op-"), Action: in.Action, Status: "running", Stage: "queued", Message: "Data operation accepted.", CreatedAt: now(), UpdatedAt: now()}
		v.Message = v.Operation.Message
	}); e != nil {
		s.release(p.ID)
		problem(w, 500, "data_state_unavailable", "Data operation could not be recorded.")
		return
	}
	s.mu.Lock()
	accepted, _ := s.loadData(p.ID)
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(p.ID)
		ctx, cancel := context.WithTimeout(s.ctx, 20*time.Minute)
		defer cancel()
		e := s.runDataAction(ctx, p.ID, in)
		_ = s.changeData(p.ID, func(v *DataStatus) {
			v.Operation.UpdatedAt = now()
			if e != nil {
				v.Operation.Status = "failed"
				v.Operation.Message = dataError(e)
				v.Message = v.Operation.Message
				if v.Status != "ready" && v.Status != "stopped" {
					v.Status = "failed"
				}
			} else {
				v.Operation.Status = "succeeded"
				v.Operation.Stage = "complete"
				v.Operation.Message = v.Message
			}
		})
	}()
	respond(w, 202, s.browserDataStatus(accepted))
}
func dataError(e error) string {
	var public *dataFailure
	if errors.As(e, &public) {
		return public.message
	}
	return "The data operation failed. Existing data is retained. Refresh to inspect owned services, and check Docker capacity, image access and available disk space."
}

type dataFailure struct{ message string }

func (e *dataFailure) Error() string { return e.message }
func (s *Service) dataStage(id, stage, message string) {
	_ = s.changeData(id, func(v *DataStatus) {
		v.Operation.Stage = stage
		v.Operation.Message = message
		v.Operation.UpdatedAt = now()
		v.Message = message
	})
}
func (s *Service) runDataAction(ctx context.Context, id string, in dataActionRequest) error {
	var install dataInstallation
	e := s.read(id+"-data-installation", &install)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if install.StackID != "" && install.ProjectID != id {
		return errors.New("installation binding mismatch")
	}
	switch in.Action {
	case "install", "start":
		if e := s.dataCapacity(ctx, id, false); e != nil {
			return e
		}
		if install.StackID == "" {
			install = dataInstallation{ProjectID: id, StackID: randomID("stack-"), Previous: []string{}}
			install.APIPort, e = freeDataPort()
			if e != nil {
				return e
			}
			secret := newDataSecret()
			if e = s.write(install.StackID+"-secret", secret); e != nil {
				return e
			}
			if e = s.prepareDataStack(install, secret); e != nil {
				return e
			}
			if e = s.write(id+"-data-installation", install); e != nil {
				return e
			}
		}
		if e = s.verifyDataOwnership(ctx, install, true); e != nil {
			return e
		}
		s.dataStage(id, "pulling", "Downloading the 11 pinned Supabase images. Container images are separately installed, not bundled with Luci.")
		if _, e = s.compose(ctx, install, "pull", "--quiet"); e != nil {
			return e
		}
		s.dataStage(id, "starting", "Starting owned Supabase services and checking their health.")
		if _, e = s.compose(ctx, install, "up", "-d", "--wait", "--wait-timeout", "240"); e != nil {
			return e
		}
		if e = s.attachDataNetwork(ctx, install); e != nil {
			return e
		}
		return s.refreshData(ctx, install, true)
	case "refresh":
		if install.StackID == "" {
			return &dataFailure{"Install the optional data stack before refreshing it."}
		}
		return s.refreshData(ctx, install, false)
	case "stop":
		if install.StackID == "" {
			return &dataFailure{"No data stack is installed for this project."}
		}
		if e = s.verifyDataOwnership(ctx, install, false); e != nil {
			return e
		}
		if _, e = s.compose(ctx, install, "stop", "--timeout", "30"); e != nil {
			return e
		}
		return s.changeData(id, func(v *DataStatus) {
			v.Status = "stopped"
			v.Message = "Data services stopped. Database, files, credentials and backups are retained."
			for i := range v.Services {
				v.Services[i].Status = "stopped"
				v.Services[i].Health = "not-running"
			}
			for k := range v.Capabilities {
				v.Capabilities[k] = false
			}
		})
	case "backup":
		if install.StackID == "" {
			return &dataFailure{"Install and start the data stack before backing it up."}
		}
		return s.backupData(ctx, install)
	case "restore":
		return s.restoreData(ctx, id, install, in.BackupID)
	}
	return errors.New("invalid action")
}
func freeDataPort() (int, error) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	port := l.Addr().(*net.TCPAddr).Port
	e = l.Close()
	return port, e
}
func newDataSecret() dataSecret {
	v := dataSecret{Password: randomID("P"), JWT: randomID("") + randomID(""), Dashboard: randomID("L"), Realtime: randomID("")[:16], Vault: randomID("") + randomID("")[:8], Crypto: randomID("") + randomID("")[:8], SecretBase: randomID("") + randomID("") + randomID(""), S3ID: randomID("") + randomID("")[:8], S3Secret: randomID("") + randomID("") + randomID("")[:16]}
	v.Anon = signDataJWT(v.JWT, "anon")
	v.ServiceRole = signDataJWT(v.JWT, "service_role")
	return v
}
func signDataJWT(secret, role string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]any{"role": role, "iss": "supabase", "iat": time.Now().Unix(), "exp": time.Now().AddDate(10, 0, 0).Unix()})
	value := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return value + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) dataCapacity(ctx context.Context, id string, additional bool) error {
	b, e := s.docker(ctx, "info", "--format", "{{json .}}")
	if e != nil {
		return &dataFailure{"Docker is unavailable. Start the Linux Docker engine and install Docker Compose before installing data services."}
	}
	var info struct {
		OSType   string
		MemTotal int64
		NCPU     int
	}
	if json.Unmarshal(b, &info) != nil || info.OSType != "linux" {
		return &dataFailure{"The Supabase data stack requires a Linux Docker engine."}
	}
	if info.MemTotal < 4<<30 || info.NCPU < 2 {
		return &dataFailure{"Docker needs at least 4 GiB of allocated RAM and 2 CPU cores for this stack, with additional capacity for Luci agents and applications."}
	}
	if _, e = s.docker(ctx, "compose", "version", "--short"); e != nil {
		return &dataFailure{"Install Docker Compose v2 before installing data services."}
	}
	// Running resource limits are separate from the Supabase allocation. The
	// exact free SSD capacity is not reported by Docker info; the user's explicit
	// acknowledgement includes its 40 GiB requirement.
	b, e = s.docker(ctx, "ps", "--quiet")
	if e != nil {
		return e
	}
	usedMem := int64(0)
	usedCPU := int64(0)
	ids := strings.Fields(string(b))
	if len(ids) > 0 {
		args := append([]string{"inspect"}, ids...)
		b, e = s.docker(ctx, args...)
		if e != nil {
			return e
		}
		var running []struct {
			Config     struct{ Labels map[string]string }
			HostConfig struct {
				Memory   int64
				NanoCpus int64
			}
		}
		if json.Unmarshal(b, &running) != nil {
			return errors.New("invalid capacity response")
		}
		for _, v := range running {
			if !additional && v.Config.Labels[labelInstance] == s.instance && v.Config.Labels[labelProject] == id && strings.HasPrefix(v.Config.Labels[labelResource], "data-") {
				continue
			}
			usedMem += v.HostConfig.Memory
			usedCPU += v.HostConfig.NanoCpus
		}
	}
	if info.MemTotal-usedMem < 4<<30 || int64(info.NCPU)*1e9-usedCPU < 2e9 {
		return &dataFailure{"Docker's allocated capacity is already reserved by other running workloads. This data stack needs another 4 GiB RAM and 2 CPU cores; stop a workload or increase Docker capacity first."}
	}
	return nil
}
func (s *Service) dataRoot(install dataInstallation) (string, error) {
	if !identifier.MatchString(install.StackID) || !identifier.MatchString(install.ProjectID) || !identifier.MatchString(install.storageTenant()) {
		return "", errors.New("invalid installation")
	}
	root, e := s.path(install.StackID)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	if e = protect(root); e != nil {
		return "", e
	}
	return root, nil
}
func (s *Service) compose(ctx context.Context, install dataInstallation, args ...string) ([]byte, error) {
	root, e := s.dataRoot(install)
	if e != nil {
		return nil, e
	}
	base := []string{"compose", "--project-name", s.resource(install.ProjectID, install.StackID), "--project-directory", root, "--env-file", filepath.Join(root, "runtime.env"), "-f", filepath.Join(root, "compose.yaml")}
	return s.opts.Runner(ctx, "docker", append(base, args...), root, nil)
}
func (s *Service) dataName(install dataInstallation, service string) string {
	name := s.resource(install.ProjectID, install.StackID+"-"+service)
	if service == "realtime" {
		name = "realtime-dev." + name
	}
	return name
}
func (s *Service) dataCredentials(install dataInstallation) (dataSecret, error) {
	var v dataSecret
	e := s.read(install.StackID+"-secret", &v)
	if e == nil && (len(v.JWT) < 32 || len(v.Password) < 24 || v.Anon == "" || v.ServiceRole == "") {
		e = errors.New("invalid private data credentials")
	}
	return v, e
}
func (s *Service) dataBindings(id string) (map[string]string, error) {
	var install dataInstallation
	e := s.read(id+"-data-installation", &install)
	if errors.Is(e, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if e != nil {
		return nil, e
	}
	if install.ProjectID != id {
		return nil, errors.New("private data binding mismatch")
	}
	secret, e := s.dataCredentials(install)
	if e != nil {
		return nil, e
	}
	publicURL, e := s.browserDataURL(install, secret)
	if e != nil {
		return nil, e
	}
	return map[string]string{"SUPABASE_URL": "http://supabase-api:8000", "SUPABASE_PUBLIC_URL": publicURL, "SUPABASE_ANON_KEY": secret.Anon, "SUPABASE_PUBLISHABLE_KEY": secret.Anon, "SUPABASE_SERVICE_ROLE_KEY": secret.ServiceRole, "SUPABASE_DB_URL": "postgresql://postgres:" + secret.Password + "@supabase-db:5432/postgres?sslmode=disable"}, nil
}
func dataSum(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func dataURL(install dataInstallation) string {
	return fmt.Sprintf("http://127.0.0.1:%d", install.APIPort)
}
