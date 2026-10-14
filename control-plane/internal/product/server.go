// Package product supplies Luci's local setup and durable project journey.
package product

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/gitcommand"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

const Version = "0.5.0"

type Config struct {
	ProjectsDir      string  `json:"projectsDir"`
	Mode             string  `json:"mode"`
	CloudProvider    string  `json:"cloudProvider"`
	Region           string  `json:"region"`
	Domain           string  `json:"domain"`
	MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd"`
}

type Registrar interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}
type Runner func(context.Context, string, []string, string) (string, error)
type Options struct {
	SwarmSchedulerPolicyDir string
	NativePrototypeReleases func(context.Context, string) ([]Release, error)
	ClearBranchReady        func(context.Context, string) error
	BranchSessions          func(context.Context, string, string) (bool, error)
	CloudDataBindings       func(context.Context, CloudDataBindingRequest) (CloudDataBindingSnapshot, error)
	BrowserPreviewPort      int
	BrowserPreviewDomain    string
	ManagedWorkspace        bool
	CloudBundle             string
	Shutdown                func()
	RuntimeDir              string
	AzureOperatorPath       string
	StateDir                string
	ProjectsDir             string
	SandboxImage            string
	CheckMemoryMiB          int
	CheckCPUMillis          int
	Configure               func(string) error
	Runner                  Runner
}

type Server struct {
	coordinatorAuthorize            func(context.Context, string) error
	coordinatorExecuteAuthorize     func(context.Context, string, string, bool) error
	coordinatorDeploymentAuthorize  management.DeploymentAuthorizer
	cloudReleaseHealthObserver      func(context.Context, string) error
	coordinatorActions              map[string]bool
	coordinatorBackground           coordinatorBackground
	externalHandoffAuthorize        func(context.Context, string) error
	projectSwarmCheckpointAuthorize func(context.Context, ProjectSwarmRecoveryRequest) error
	projectSwarmCheckpointGuard     func(context.Context, ProjectSwarmRecoveryRequest) (func(), error)
	projectSwarmMicroVMImportGuard  func(context.Context, ProjectSwarmMicroVMImportScope) (func(), error)
	cloudMicroVMAvailable           func() bool
	cloudTaskCapacity               func(context.Context, string, string) (CoordinatorCloudCapacity, error)
	taskHandoffAuthorize            func(context.Context, ProjectSwarmRecoveryRequest) error
	taskHandoffCollector            func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error)
	projectSwarmPendingLaunch       func(context.Context, string, string, string, string) (bool, error)
	projectSwarmFenceAuthorize      func(context.Context, ProjectSwarmFenceRecoveryRequest) error
	projectSwarmFenceAvailability   func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryAvailability, error)
	projectSwarmFenceAcquire        func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error)
	projectSwarmFenceProve          func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error)
	projectSwarmFenceFinalize       func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error
	projectSwarmRecoveryProve       func(context.Context, ProjectSwarmRecoveryRequest) (ProjectSwarmRecoveryProof, error)
	projectSwarmRecoveryFinalize    func(context.Context, ProjectSwarmRecoveryRequest, ProjectSwarmRecoveryProof) error
	swarmSchedulerPolicyDir         string
	projectOverview                 projectOverviewCache
	swarmScheduler                  swarmScheduler
	cloudTaskObserver               cloudTaskObserver
	projectSwarmWorktreeObserver    func(context.Context, string, string) (bool, error)
	projectRuns                     *ProjectRunStore
	projectRunDispatchMu            sync.Mutex
	projectRunSelectionResolver     func(context.Context, ProjectRunSelectionRequest) (ProjectRunSelection, error)
	projectRunSessionObserver       func(context.Context, string, string, string) ([]registry.Session, error)
	projectRunNativeReleaseObserver func(context.Context, string, string) (ProjectRunNativeReleaseObservation, error)
	nativePrototypeReleases         func(context.Context, string) ([]Release, error)
	clearBranchReady                func(context.Context, string) error
	branchSessions                  func(context.Context, string, string) (bool, error)
	branchGuardMu                   sync.Mutex
	branchGuards                    map[string]*sync.RWMutex
	branchRunner                    Runner
	cloudDataBindings               func(context.Context, CloudDataBindingRequest) (CloudDataBindingSnapshot, error)
	browserRegistry                 *previewgateway.Registry
	cloudBundle                     string
	workspaceCompanionOpen          func() (*workspaceCompanion, error)
	workspacePullCompanionOpen      func(context.Context) (*workspaceCompanion, error)
	workspaceTransferAvailable      func(string) (uint64, error)
	workspace                       Workspace
	workspaceInstalls               map[string]WorkspaceInstall
	workspaceCancel                 context.CancelFunc
	workspaceContext                context.Context
	remoteStatus                    *Status
	workspaceAuth                   map[string]workspaceCredential
	previews                        map[string]*workspacePreview
	shutdown                        func()
	agentAPIKey                     string
	agentOpenAIKey                  string
	agentLocal                      LocalAgentConfig
	agentConnections                agentConnectionStore
	activeLocalReleases             map[string]bool
	localPreviewOperations          map[string]*localPreviewOperation
	localPreviewsClosed             bool
	deliveryBusy                    map[string]bool
	importMu                        sync.Mutex
	importNames                     map[string]bool
	importActive                    map[string]bool
	projectChecks                   map[string]*projectCheckJob
	projectCheckRuntime             ProjectCheckRuntime
	previewRuntime                  previewruntime.Runtime
	runtimeInstall                  runtimeInstallState
	runtimeJob                      RuntimeJob
	runtimeResourceSnapshot         func() resourcebudget.Snapshot
	authJobs                        map[string]*ProviderLoginJob
	runtimeDir                      string
	azureOperatorPath               string
	mu                              sync.RWMutex
	functionSourceMu                sync.RWMutex
	config                          Config
	setupChoice                     SetupChoice
	stateDir                        string
	runner                          Runner
	configure                       func(string) error
	sandboxImage                    string
	checkMemoryMiB                  int
	checkCPUMillis                  int
	configured                      bool
	managedWorkspace                bool
	checks                          []Check
	plans                           map[string]CloudPlan
	jobs                            map[string]CloudJob
	targets                         map[string]DeploymentTarget
	scans                           map[string]SSHScan
	tunnels                         map[string]*Tunnel
}

func New(opts Options) (*Server, error) {
	if !filepath.IsAbs(opts.StateDir) {
		return nil, errors.New("Luci's state directory must be absolute")
	}
	root := opts.ProjectsDir
	if opts.ManagedWorkspace && !filepath.IsAbs(root) {
		return nil, errors.New("managed workspaces require an assigned absolute projects folder")
	}
	if root == "" {
		root = filepath.Join(opts.StateDir, "projects")
	}
	s := &Server{swarmSchedulerPolicyDir: opts.SwarmSchedulerPolicyDir, nativePrototypeReleases: opts.NativePrototypeReleases, clearBranchReady: opts.ClearBranchReady, branchSessions: opts.BranchSessions, cloudDataBindings: opts.CloudDataBindings, stateDir: opts.StateDir, runtimeDir: opts.RuntimeDir, cloudBundle: opts.CloudBundle, shutdown: opts.Shutdown, runner: opts.Runner, configure: opts.Configure,
		sandboxImage: opts.SandboxImage, plans: make(map[string]CloudPlan),
		managedWorkspace: opts.ManagedWorkspace, configured: opts.ManagedWorkspace,
		config: Config{ProjectsDir: root, Mode: "local", MonthlyBudgetUSD: 30}}
	s.azureOperatorPath = opts.AzureOperatorPath
	if opts.BrowserPreviewPort != 0 {
		var err error
		s.browserRegistry, err = previewgateway.NewRegistryWithDomain(opts.StateDir, opts.BrowserPreviewPort, opts.BrowserPreviewDomain)
		if err != nil {
			return nil, err
		}
	}
	if s.runner == nil {
		s.runner = RunCommand
	}
	if s.sandboxImage == "" {
		s.sandboxImage = "luci/sandbox:dev"
	}
	s.checkMemoryMiB = opts.CheckMemoryMiB
	if s.checkMemoryMiB <= 0 {
		s.checkMemoryMiB = 4096
	}
	s.checkCPUMillis = opts.CheckCPUMillis
	if s.checkCPUMillis <= 0 {
		s.checkCPUMillis = 2000
	}
	data, err := os.ReadFile(filepath.Join(opts.StateDir, "luci.json"))
	if err == nil {
		if err = json.Unmarshal(data, &s.config); err != nil {
			return nil, fmt.Errorf("read Luci configuration: %w", err)
		}
		if err = validateConfig(s.config); err != nil {
			return nil, err
		}
		s.configured = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if opts.ManagedWorkspace && filepath.Clean(s.config.ProjectsDir) != filepath.Clean(root) {
		return nil, errors.New("managed workspace configuration must retain its assigned projects folder")
	}
	if !filepath.IsAbs(s.config.ProjectsDir) {
		return nil, errors.New("projects folder must be absolute")
	}
	if err := os.MkdirAll(s.config.ProjectsDir, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.stateDir, 0700); err != nil {
		return nil, err
	}
	if err := stateOutsideProjects(s.config.ProjectsDir, s.stateDir); err != nil {
		return nil, err
	}
	if err := s.loadSetupChoice(); err != nil {
		return nil, err
	}
	if s.configure != nil {
		if err := s.configure(s.config.ProjectsDir); err != nil {
			return nil, err
		}
	}
	s.checks = defaultChecks()
	if err := s.initSetupTools(); err != nil {
		return nil, fmt.Errorf("read setup tools: %w", err)
	}
	if err := s.initConnections(); err != nil {
		return nil, fmt.Errorf("read connections: %w", err)
	}
	if err := s.initCloudJobs(); err != nil {
		return nil, fmt.Errorf("read cloud history: %w", err)
	}
	if err := s.initWorkspace(); err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}
	var runErr error
	s.projectRuns, runErr = NewProjectRunStore(s.stateDir)
	if runErr != nil {
		return nil, fmt.Errorf("open project run storage: %w", runErr)
	}
	if err := s.initSwarmPolicyDirectory(); err != nil {
		return nil, fmt.Errorf("open shared execution policy: %w", err)
	}
	s.migrateOverviewRunHeads()
	return s, nil
}

func (s *Server) Register(mux Registrar) {
	mux = s.guardProjectSourceRoutes(mux)
	mux.HandleFunc("POST /v1/setup/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if s.shutdown == nil {
			problem(w, 501, "unsupported", "This service is managed externally.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "stopping"})
		go s.shutdown()
	})
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	mux.HandleFunc("POST /v1/setup/check", s.handleCheck)
	mux.HandleFunc("POST /v1/setup/config", s.handleConfig)
	mux.HandleFunc("PUT /v1/setup/choice", s.handleSetupChoice)
	mux.HandleFunc("POST /v1/setup/cloud/plan", s.handleCloudPlan)
	mux.HandleFunc("POST /v1/setup/cloud/azure/plan", s.planAzureDevelop)
	mux.HandleFunc("GET /v1/setup/cloud/azure/bundle", s.getAzureDevelopBundle)
	mux.HandleFunc("GET /v1/setup/cloud/azure/identity", s.getAzureControllerIdentity)
	s.registerProjects(mux)
	s.registerCloudJobs(mux)
	s.registerConnections(mux)
	s.registerCloudReleases(mux)
	s.registerPagesReleases(mux)
	s.registerSetupTools(mux)
	s.registerProviderLogin(mux)
	s.registerWorkspace(mux)
	s.registerPreviewProxy(mux)
	s.registerDelivery(mux)
	s.registerBackups(mux)
	s.registerProjectWork(mux)
	s.registerProjectRuns(mux)
	s.registerTaskHandoffs(mux)
	s.registerSwarmScheduler(mux)
	s.registerProjectOverview(mux)
	s.registerApplicationConfiguration(mux)
}

func (s *Server) projectRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.ProjectsDir
}
func (s *Server) Config() Config { s.mu.RLock(); defer s.mu.RUnlock(); return s.config }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		problem(w, 400, "invalid_json", "Please check the submitted fields: "+err.Error())
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		problem(w, 400, "invalid_json", "Send one JSON object")
		return false
	}
	return true
}

func atomicJSON(path string, value any) error {
	return publishJSON(path, value, os.Rename)
}

func durableRunJSON(path string, value any) error {
	return publishJSON(path, value, publishRunReceipt)
}

func publishJSON(path string, value any, publish func(string, string) error) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".luci-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := restrictPrivateFile(name); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return publish(name, path)
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b[:])
}

// RunCommand never invokes a shell. Output and duration are bounded, including child pipe drains.
func RunCommand(ctx context.Context, executable string, args []string, dir string) (string, error) {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "AWS_PAGER=", "AZURE_CORE_ONLY_SHOW_ERRORS=true")
	if executable == "git" {
		env = journeyGitEnvironment(ctx, env)
	}
	return runCommandEnvironment(ctx, executable, args, dir, env)
}

func runCommandEnvironment(ctx context.Context, executable string, args []string, dir string, env []string) (string, error) {
	limit := commandExecutionBudget(ctx, executable, args)
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if executable == "git" {
		args = gitcommand.PlatformArgs(dir, args)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	hideProcess(cmd)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = env
	output := &limitedBuffer{limit: 2 << 20}
	stderr := &limitedBuffer{limit: 2 << 20}
	cmd.Stdout = output
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return output.String(), fmt.Errorf("%s: %s: %w", executable, detail, err)
	}
	return output.String(), nil
}

type limitedBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *limitedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }

type Check struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Action  string `json:"action"`
}
type Provider struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Strategy    string `json:"strategy"`
	Description string `json:"description"`
}
type SetupAccess struct {
	User struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
	ManageBacking      bool `json:"manageBacking"`
	ProjectsDirManaged bool `json:"projectsDirManaged"`
}

type Status struct {
	SetupChoice    SetupChoice  `json:"setupChoice"`
	Access         *SetupAccess `json:"access,omitempty"`
	Workspace      Workspace    `json:"workspace"`
	Product        string       `json:"product"`
	Version        string       `json:"version"`
	Platform       string       `json:"platform"`
	Configured     bool         `json:"configured"`
	Config         Config       `json:"config"`
	Checks         []Check      `json:"checks"`
	CloudProviders []Provider   `json:"cloudProviders"`
}

func (s *Server) statusForActor(ctx context.Context) Status {
	status := s.status()
	s.mu.RLock()
	status.SetupChoice = s.setupChoice
	s.mu.RUnlock()
	if user, policy, ok := management.ActorFromContext(ctx); ok {
		status.Access = &SetupAccess{ManageBacking: management.PermissionsFor(user, policy).ManageBacking, ProjectsDirManaged: s.managedWorkspace}
		status.Access.User.Username = user.Username
		status.Access.User.Role = user.Role
	}
	return status
}

func defaultChecks() []Check {
	return []Check{
		{ID: "git", Label: "Git", Status: "unknown", Message: "Check that Git is installed"},
		{ID: "docker", Label: "Docker engine", Status: "unknown", Message: "Required for isolated agent sessions and local previews"},
		{ID: "sandbox-image", Label: "Agent runtime", Status: "unknown", Message: "Check that Luci's agent image is available"},
		{ID: "aws", Label: "AWS connection", Status: "unknown", Message: "Optional; use your existing AWS sign-in"},
		{ID: "azure", Label: "Azure connection", Status: "unknown", Message: "Optional; use your existing Azure sign-in"},
	}
}
func (s *Server) status() Status {
	s.mu.RLock()
	workspace := s.workspace
	if workspace.Mode == "cloud" {
		t := s.tunnels[workspace.ConnectionID]
		workspace.Ready = workspace.Ready && t != nil && t.Status == "connected"
	}
	status := Status{Workspace: workspace, Product: "Luci", Version: Version, Platform: runtime.GOOS, Configured: s.configured, Config: s.config, Checks: append([]Check{}, s.checks...), CloudProviders: []Provider{
		{ID: "aws", Name: "AWS", Strategy: "Lightsail", Description: "One Linux machine with bundle pricing"},
		{ID: "azure", Name: "Azure", Strategy: "Virtual machine", Description: "One Linux machine, persistent storage and HTTPS"},
	}}
	s.mu.RUnlock()
	return s.withRuntimeStorageReadiness(status)
}
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.workspaceStatus(r.Context(), "GET"))
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	checks := defaultChecks()
	s.mu.RLock()
	cloud := s.workspace.Mode == "cloud"
	s.mu.RUnlock()
	probes := []struct {
		exe    string
		args   []string
		action string
	}{
		{"git", []string{"--version"}, "Install Git, then run this check again."},
		{"docker", []string{"info", "--format", "{{.ServerVersion}}"}, "Open Docker Desktop or start your Docker engine, then check again."},
		{"docker", []string{"image", "inspect", s.sandboxImage, "--format", "{{.Id}}"}, "Install Luci's agent runtime from the local runtime setup."},
		{"aws", []string{"sts", "get-caller-identity", "--query", "Account", "--output", "text"}, "Install AWS CLI and sign in with aws login or your configured SSO profile."},
		{"az", []string{"account", "show", "--query", "name", "--output", "tsv"}, "Install Azure CLI and use az login to connect your own account."},
	}
	var pending sync.WaitGroup
	for i, p := range probes {
		if cloud && i < 3 {
			continue
		}
		pending.Add(1)
		go func() {
			defer pending.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()
			output, err := s.runner(ctx, p.exe, p.args, "")
			// Each independent probe owns one stable result slot. Publish the
			// complete ordered snapshot only after every probe has settled.
			// Zero exit, empty output or late proof after cancellation is not
			// evidence that Docker or a provider account is ready.
			if err != nil || ctx.Err() != nil || strings.TrimSpace(output) == "" {
				checks[i].Status = "missing"
				checks[i].Message = "This connection is not ready."
				checks[i].Action = p.action
			} else {
				checks[i].Status = "ready"
				checks[i].Message = "Ready"
				if i == 0 || i == 1 {
					checks[i].Message = strings.TrimSpace(output)
				}
				if i == 3 {
					checks[i].Message = "Signed in to your AWS account"
				}
				if i == 4 {
					checks[i].Message = "Signed in to " + strings.TrimSpace(output)
				}
			}
		}()
	}
	pending.Wait()
	s.mu.Lock()
	// A superseded or disconnected browser may cancel its generation while
	// the probes drain. Its result must not replace a newer readiness snapshot.
	if r.Context().Err() != nil {
		s.mu.Unlock()
		return
	}
	s.checks = checks
	s.mu.Unlock()
	writeJSON(w, 200, s.workspaceStatus(r.Context(), "POST"))
}

func validateConfig(c Config) error {
	if !filepath.IsAbs(c.ProjectsDir) || strings.TrimSpace(c.ProjectsDir) == "" {
		return errors.New("choose an absolute projects folder")
	}
	if c.Mode != "local" && c.Mode != "cloud" {
		return errors.New("choose local or cloud mode")
	}
	if c.CloudProvider != "" && c.CloudProvider != "aws" && c.CloudProvider != "azure" {
		return errors.New("choose AWS or Azure")
	}
	if c.Mode == "cloud" && c.CloudProvider == "" {
		return errors.New("choose a cloud provider")
	}
	if c.MonthlyBudgetUSD < 0 || c.MonthlyBudgetUSD > 100000 {
		return errors.New("enter a monthly budget between 0 and 100000 USD")
	}
	if c.Domain != "" && !validDomain(c.Domain) {
		return errors.New("enter a domain name without https:// or a path")
	}
	return nil
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	var c Config
	if !readJSON(w, r, &c) {
		return
	}
	c.ProjectsDir = filepath.Clean(c.ProjectsDir)
	if s.managedWorkspace && c.ProjectsDir != s.projectRoot() {
		problem(w, 403, "workspace_path_fixed", "Managed users keep their projects inside their assigned workspace.")
		return
	}
	c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
	if err := validateConfig(c); err != nil {
		problem(w, 400, "bad_request", err.Error())
		return
	}
	if err := os.MkdirAll(c.ProjectsDir, 0700); err != nil {
		problem(w, 400, "folder_unavailable", err.Error())
		return
	}
	if err := stateOutsideProjects(c.ProjectsDir, s.stateDir); err != nil {
		problem(w, 409, "unsafe_projects_folder", err.Error())
		return
	}
	s.mu.Lock()
	if s.configure != nil {
		if err := s.configure(c.ProjectsDir); err != nil {
			s.mu.Unlock()
			problem(w, 409, "folder_unavailable", err.Error())
			return
		}
	}
	if err := atomicJSON(filepath.Join(s.stateDir, "luci.json"), c); err != nil {
		if s.configure != nil {
			_ = s.configure(s.config.ProjectsDir)
		}
		s.mu.Unlock()
		problem(w, 500, "save_failed", "Could not save Luci settings")
		return
	}
	s.config = c
	s.configured = true
	s.mu.Unlock()
	writeJSON(w, 200, s.statusForActor(r.Context()))
}
