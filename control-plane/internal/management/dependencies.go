package management

// Dependency installation belongs to the management host, before any per-user
// worker or selected cloud workspace. Package IDs and process arguments are
// compiled here; callers cannot supply executable paths, URLs or shell text.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type dependencyDefinition struct{ id, name, packageID, executable string }

var dependencyCatalogue = []dependencyDefinition{
	{"git", "Git", "Git.Git", "git"},
	{"docker", "Docker Desktop", "Docker.DockerDesktop", "docker"},
	{"github", "GitHub CLI", "GitHub.cli", "gh"},
	{"aws", "AWS CLI", "Amazon.AWSCLI", "aws"},
	{"azure", "Azure CLI", "Microsoft.AzureCLI", "az"},
}

type dependencyCheck struct {
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message"`
	CheckedAt string `json:"checkedAt,omitempty"`
}
type dependencyJob struct {
	ID             string `json:"id"`
	DependencyID   string `json:"dependencyId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	ActionRequired string `json:"actionRequired"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	ExitCode       string `json:"exitCode,omitempty"`
}
type dependencyRow struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	PackageID      string          `json:"packageId"`
	Supported      bool            `json:"supported"`
	Reason         string          `json:"reason"`
	Check          dependencyCheck `json:"check"`
	ActionRequired string          `json:"actionRequired"`
	Job            *dependencyJob  `json:"job"`
}
type dependencyInstaller struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CheckedAt string `json:"checkedAt,omitempty"`
}
type dependencyStatus struct {
	Scope        string              `json:"scope"`
	Platform     string              `json:"platform"`
	CanInstall   bool                `json:"canInstall"`
	Installer    dependencyInstaller `json:"installer"`
	Dependencies []dependencyRow     `json:"dependencies"`
}
type storedDependencyJob struct {
	Job     dependencyJob `json:"job"`
	ActorID string        `json:"actorId"`
}
type dependencyState struct {
	Version int                   `json:"version"`
	Status  dependencyStatus      `json:"status"`
	Jobs    []storedDependencyJob `json:"jobs"`
}
type dependencyCommandResult struct {
	Output string
	Code   uint32
	Err    error
}
type dependencyRunner interface {
	platform() string
	lookup(string) (string, error)
	run(context.Context, string, ...string) dependencyCommandResult
	installationLease(context.Context) (func(), error)
}
type dependencyManager struct {
	mu       sync.Mutex
	gateway  *Gateway
	runner   dependencyRunner
	state    dependencyState
	path     string
	checking bool
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
	notify   chan struct{}
	done     chan struct{}
}

func dependencyByID(id string) (dependencyDefinition, bool) {
	for _, d := range dependencyCatalogue {
		if d.id == id {
			return d, true
		}
	}
	return dependencyDefinition{}, false
}
func activeDependencyJob(status string) bool { return status == "queued" || status == "installing" }
func dependencyNow() string                  { return time.Now().UTC().Format(time.RFC3339Nano) }

func initialDependencyState(platform string) dependencyState {
	state := dependencyState{Version: 1, Jobs: []storedDependencyJob{}, Status: dependencyStatus{
		Scope: "control-plane-host", Platform: platform,
		Installer:    dependencyInstaller{ID: "winget", Status: "unchecked", Message: "Check tools before installing."},
		Dependencies: []dependencyRow{},
	}}
	for _, d := range dependencyCatalogue {
		row := dependencyRow{ID: d.id, Name: d.name, PackageID: d.packageID, Supported: platform == "windows", ActionRequired: "check",
			Check: dependencyCheck{Status: "unchecked", Message: "Check this tool on the computer running Luci's connection service."}}
		if !row.Supported {
			row.Reason = "Automatic installation currently supports Windows Package Manager. Use this tool's official installer on this operating system."
		}
		state.Status.Dependencies = append(state.Status.Dependencies, row)
	}
	if platform != "windows" {
		state.Status.Installer.Status = "unsupported"
		state.Status.Installer.Message = "Use official installers on this operating system; Luci can still check installed tools."
	}
	return state
}

func newDependencyManager(g *Gateway, runner dependencyRunner) (*dependencyManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &dependencyManager{gateway: g, runner: runner, path: filepath.Join(g.dir, "dependencies.json"), ctx: ctx, cancel: cancel, notify: make(chan struct{}, 1), done: make(chan struct{})}
	m.state = initialDependencyState(runner.platform())
	data, err := os.ReadFile(m.path)
	if err == nil {
		var saved dependencyState
		if len(data) > 512<<10 || json.Unmarshal(data, &saved) != nil || saved.Version != 1 || len(saved.Jobs) > 100 || len(saved.Status.Dependencies) != len(dependencyCatalogue) {
			cancel()
			return nil, errors.New("invalid managed dependency state")
		}
		for i, row := range saved.Status.Dependencies {
			if row.ID != dependencyCatalogue[i].id || row.PackageID != dependencyCatalogue[i].packageID {
				cancel()
				return nil, errors.New("managed dependency catalogue has changed")
			}
		}
		// Platform changes discard obsolete probes, but retain operation history.
		if saved.Status.Platform == runner.platform() {
			m.state.Status = saved.Status
		}
		m.state.Jobs = saved.Jobs
		for i := range m.state.Jobs {
			j := &m.state.Jobs[i].Job
			if _, ok := dependencyByID(j.DependencyID); !ok || j.ID == "" || m.state.Jobs[i].ActorID == "" {
				cancel()
				return nil, errors.New("invalid managed dependency job")
			}
			if activeDependencyJob(j.Status) {
				j.Status, j.ActionRequired, j.UpdatedAt = "interrupted", "check", dependencyNow()
				j.Message = "Luci stopped before this installation finished. Check tools before choosing an explicit retry."
			}
		}
		m.attachJobsLocked()
		m.state.Status.CanInstall = false // Recomputed for the requesting actor.
		if err = m.saveLocked(); err != nil {
			cancel()
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		cancel()
		return nil, err
	}
	go m.work()
	return m, nil
}

func (m *dependencyManager) saveLocked() error {
	data, err := json.Marshal(m.state)
	if err != nil {
		return err
	}
	return atomicPrivate(m.path, data)
}
func (m *dependencyManager) attachJobsLocked() {
	for i := range m.state.Status.Dependencies {
		row := &m.state.Status.Dependencies[i]
		row.Job = nil
		for j := len(m.state.Jobs) - 1; j >= 0; j-- {
			if m.state.Jobs[j].Job.DependencyID == row.ID {
				copy := m.state.Jobs[j].Job
				checked, checkErr := time.Parse(time.RFC3339Nano, row.Check.CheckedAt)
				updated, jobErr := time.Parse(time.RFC3339Nano, copy.UpdatedAt)
				// A fresh explicit check supersedes an old installation's next
				// action without rewriting its historical outcome.
				if activeDependencyJob(copy.Status) || checkErr != nil || jobErr != nil || !checked.After(updated) {
					row.Job = &copy
				}
				break
			}
		}
	}
}
func (m *dependencyManager) snapshot(canManage bool) dependencyStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.state.Status
	status.CanInstall = canManage && status.Platform == "windows" && status.Installer.Status == "ready" && !m.closed
	status.Dependencies = append([]dependencyRow{}, status.Dependencies...)
	for i := range status.Dependencies {
		if status.Dependencies[i].Job != nil {
			copy := *status.Dependencies[i].Job
			status.Dependencies[i].Job = &copy
		}
	}
	return status
}
func (m *dependencyManager) close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.cancel()
	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gateway) dependencyStatus(w http.ResponseWriter, r *http.Request) {
	u, p, _ := ActorFromContext(r.Context())
	jsonReply(w, 200, g.dependencies.snapshot(PermissionsFor(u, p).ManageBacking))
}
func (g *Gateway) checkDependencies(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	var req struct{}
	if !decode(w, r, &req) {
		return
	}
	if err := g.dependencies.check(r.Context()); err != nil {
		dependencyError(w, err)
		return
	}
	jsonReply(w, 200, g.dependencies.snapshot(true))
}
func (g *Gateway) installDependency(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if _, valid := dependencyByID(id); !valid {
		fail(w, 404, "unknown_dependency", "Choose one of the listed tools.")
		return
	}
	var req struct {
		AcceptAgreements bool `json:"acceptAgreements"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !req.AcceptAgreements {
		fail(w, 400, "agreements_required", "Accept the selected publisher's package and Microsoft source agreements before installation.")
		return
	}
	job, err := g.dependencies.queue(actor.ID, id)
	if err != nil {
		dependencyError(w, err)
		return
	}
	jsonReply(w, 202, job)
}

type dependencyFailure struct{ code, message string }

func (e dependencyFailure) Error() string { return e.message }
func dependencyError(w http.ResponseWriter, err error) {
	var typed dependencyFailure
	if errors.As(err, &typed) {
		fail(w, 409, typed.code, typed.message)
		return
	}
	fail(w, 500, "dependency_state_failed", "The tool setup state could not be saved. No new installation was started.")
}
func (g *Gateway) dependencyIssuerAuthorized(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	u, ok := g.state.Users[id]
	return !g.closed && ok && !u.User.Disabled && (u.User.Role == "admin" || u.User.Role == "super-admin")
}
func (g *Gateway) auditDependency(actor, job, outcome string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.auditLocked(actor, "dependency.install", job, outcome)
	_ = g.persistLocked()
}

func (m *dependencyManager) queue(actor, id string) (dependencyJob, error) {
	if !m.gateway.dependencyIssuerAuthorized(actor) {
		return dependencyJob{}, dependencyFailure{"identity_changed", "Your administrator role changed. Sign in again before installing tools."}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return dependencyJob{}, dependencyFailure{"connection_closing", "Reopen Luci before installing a tool."}
	}
	if m.checking {
		m.mu.Unlock()
		return dependencyJob{}, dependencyFailure{"dependency_busy", "Wait for the current tool check to finish."}
	}
	for _, item := range m.state.Jobs {
		if item.Job.DependencyID == id && activeDependencyJob(item.Job.Status) {
			m.mu.Unlock()
			return item.Job, nil
		}
	}
	if m.runner.platform() != "windows" || m.state.Status.Installer.Status != "ready" {
		m.mu.Unlock()
		return dependencyJob{}, dependencyFailure{"installer_not_ready", "Check tools and make Microsoft Windows Package Manager available before installation."}
	}
	for _, row := range m.state.Status.Dependencies {
		if row.ID == id && row.Check.Status != "missing" {
			m.mu.Unlock()
			return dependencyJob{}, dependencyFailure{"dependency_not_missing", "Check this tool first. A detected installation requires the displayed action, rather than another installer."}
		}
		if row.ID == id && row.Job != nil {
			m.mu.Unlock()
			return dependencyJob{}, dependencyFailure{"dependency_check_required", "Check tools after the previous installation before choosing an explicit retry."}
		}
	}
	previous := append([]storedDependencyJob{}, m.state.Jobs...)
	if len(m.state.Jobs) == 100 {
		m.state.Jobs = append([]storedDependencyJob{}, m.state.Jobs[1:]...)
	}
	job := dependencyJob{ID: opaqueID("dependency-"), DependencyID: id, Status: "queued", Message: "Queued for installation on this computer.", ActionRequired: "none", CreatedAt: dependencyNow(), UpdatedAt: dependencyNow()}
	m.state.Jobs = append(m.state.Jobs, storedDependencyJob{Job: job, ActorID: actor})
	m.attachJobsLocked()
	if err := m.saveLocked(); err != nil {
		m.state.Jobs = previous
		m.attachJobsLocked()
		m.mu.Unlock()
		return dependencyJob{}, err
	}
	m.mu.Unlock()
	m.gateway.auditDependency(actor, job.ID, "queued")
	select {
	case m.notify <- struct{}{}:
	default:
	}
	return job, nil
}

func (m *dependencyManager) check(parent context.Context) error {
	m.mu.Lock()
	if m.closed || m.checking {
		m.mu.Unlock()
		return dependencyFailure{"dependency_busy", "A tool check is already running or Luci is closing."}
	}
	for _, row := range m.state.Jobs {
		if activeDependencyJob(row.Job.Status) {
			m.mu.Unlock()
			return dependencyFailure{"dependency_busy", "Wait for the queued tool installations to finish, then check again."}
		}
	}
	m.checking = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.checking = false; m.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	// A service shutdown also cancels read-only probes.
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	installer := m.checkInstaller(ctx)
	rows := make([]dependencyRow, 0, len(dependencyCatalogue))
	for _, def := range dependencyCatalogue {
		row := initialDependencyState(m.runner.platform()).Status.Dependencies[len(rows)]
		row.Check, row.ActionRequired = m.checkTool(ctx, def)
		if row.Check.Status == "missing" {
			if installer.Status == "missing" {
				row.ActionRequired = "install_winget"
			} else if installer.Status != "ready" {
				row.ActionRequired = "manual_install"
			}
		}
		rows = append(rows, row)
	}
	if ctx.Err() != nil {
		return dependencyFailure{"dependency_check_incomplete", "The tool check was interrupted or timed out. Check tools again."}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.state.Status
	m.state.Status.Installer, m.state.Status.Dependencies = installer, rows
	m.attachJobsLocked()
	if err := m.saveLocked(); err != nil {
		m.state.Status = before
		return err
	}
	return nil
}

const microsoftWingetSource = "https://cdn.winget.microsoft.com/cache"

func (m *dependencyManager) checkInstaller(parent context.Context) dependencyInstaller {
	installer := dependencyInstaller{ID: "winget", CheckedAt: dependencyNow()}
	if m.runner.platform() != "windows" {
		installer.Status, installer.Message = "unsupported", "Automatic installation uses Microsoft Windows Package Manager. Use official installers on this operating system."
		return installer
	}
	path, err := m.runner.lookup("winget")
	if err != nil {
		installer.Status, installer.Message = "missing", "Microsoft Windows Package Manager is not available. Install Microsoft's App Installer, reopen Luci and check tools."
		return installer
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	result := m.runner.run(ctx, path, "source", "export", "--name", "winget", "--disable-interactivity")
	var source struct {
		Name, Type, Arg, Data, Identifier string
		TrustLevel                        []string
	}
	trusted := result.Err == nil && result.Code == 0 && json.Unmarshal([]byte(strings.TrimPrefix(result.Output, "\ufeff")), &source) == nil && source.Name == "winget" && source.Type == "Microsoft.PreIndexed.Package" && source.Arg == microsoftWingetSource && source.Data == "Microsoft.Winget.Source_8wekyb3d8bbwe" && source.Identifier == "Microsoft.Winget.Source_8wekyb3d8bbwe"
	if trusted {
		trusted = false
		for _, level := range source.TrustLevel {
			if level == "Trusted" {
				trusted = true
			}
		}
	}
	if !trusted {
		installer.Status, installer.Message = "untrusted", "Luci could not verify the official Microsoft WinGet source. Check Windows Package Manager's configured sources; Luci does not change or reset them."
		return installer
	}
	installer.Status, installer.Message = "ready", "Microsoft's official Windows Package Manager source is verified. Each installation requires your agreement."
	return installer
}

var dependencyVersions = map[string]*regexp.Regexp{
	"git":    regexp.MustCompile(`(?m)^git version ([0-9][0-9A-Za-z._+-]{0,63})`),
	"docker": regexp.MustCompile(`(?m)^Docker version ([0-9][0-9A-Za-z._+-]{0,63})`),
	"github": regexp.MustCompile(`(?m)^gh version ([0-9][0-9A-Za-z._+-]{0,63})`),
	"aws":    regexp.MustCompile(`aws-cli/([0-9][0-9A-Za-z._+-]{0,63})`),
}

func (m *dependencyManager) checkTool(parent context.Context, def dependencyDefinition) (dependencyCheck, string) {
	check := dependencyCheck{CheckedAt: dependencyNow()}
	path, err := m.runner.lookup(def.executable)
	if err != nil {
		check.Status, check.Message = "missing", def.name+" is not available to Luci."
		return check, "none"
	}
	args := []string{"--version"}
	if def.id == "azure" {
		args = []string{"version", "--output", "json"}
		// Windows Azure CLI's az.cmd is a wrapper. Execute its fixed adjacent
		// Python module directly; never turn a batch path into shell input.
		if m.runner.platform() == "windows" && strings.EqualFold(filepath.Ext(path), ".cmd") {
			path = filepath.Join(filepath.Dir(filepath.Dir(path)), "python.exe")
			args = append([]string{"-I", "-m", "azure.cli"}, args...)
		}
	}
	ctx, cancel := context.WithTimeout(parent, 7*time.Second)
	defer cancel()
	result := m.runner.run(ctx, path, args...)
	version := ""
	if def.id == "azure" {
		var versions map[string]string
		if json.Unmarshal([]byte(result.Output), &versions) == nil {
			version = versions["azure-cli"]
			if !regexp.MustCompile(`^[0-9][0-9A-Za-z._+-]{0,63}$`).MatchString(version) {
				version = ""
			}
		}
	} else if match := dependencyVersions[def.id].FindStringSubmatch(result.Output); len(match) == 2 {
		version = match[1]
	}
	if result.Err != nil || result.Code != 0 || version == "" {
		check.Status, check.Message = "action_required", def.name+" was found but its version check did not finish successfully. Check the installation, then retry."
		return check, "manual_install"
	}
	check.Version, check.Status, check.Message = version, "ready", def.name+" is installed and its version check succeeded."
	if def.id == "docker" {
		engine := m.runner.run(ctx, path, "info", "--format", "{{.ServerVersion}}")
		if engine.Err != nil || engine.Code != 0 || strings.TrimSpace(engine.Output) == "" {
			check.Status, check.Message = "action_required", "Docker CLI is installed, but its engine is not available. Open Docker Desktop and wait for its engine; workspace readiness checks the selected execution host separately."
			return check, "start_docker"
		}
		check.Message = "Docker CLI is installed and its configured engine answered. Workspace readiness separately checks the Luci runtime image."
	}
	return check, "none"
}

func (m *dependencyManager) work() {
	defer close(m.done)
	for {
		select {
		case <-m.ctx.Done():
			m.interruptPending()
			return
		case <-m.notify:
			for {
				m.mu.Lock()
				index := -1
				for i := range m.state.Jobs {
					if m.state.Jobs[i].Job.Status == "queued" {
						index = i
						break
					}
				}
				if index == -1 || m.closed || m.ctx.Err() != nil {
					m.mu.Unlock()
					break
				}
				stored := m.state.Jobs[index]
				m.mu.Unlock()
				m.install(stored)
			}
		}
	}
}

func (m *dependencyManager) setJob(id, status, action, message, exitCode string, check *dependencyCheck) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Jobs {
		if m.state.Jobs[i].Job.ID != id {
			continue
		}
		before := m.state.Jobs[i].Job
		job := &m.state.Jobs[i].Job
		job.Status, job.ActionRequired, job.Message, job.ExitCode, job.UpdatedAt = status, action, message, exitCode, dependencyNow()
		if check != nil {
			for j := range m.state.Status.Dependencies {
				if m.state.Status.Dependencies[j].ID == job.DependencyID {
					m.state.Status.Dependencies[j].Check = *check
					m.state.Status.Dependencies[j].ActionRequired = action
				}
			}
		}
		m.attachJobsLocked()
		if m.saveLocked() != nil {
			m.state.Jobs[i].Job = before
			m.attachJobsLocked()
			return false
		}
		return true
	}
	return false
}

func (m *dependencyManager) install(stored storedDependencyJob) {
	job := stored.Job
	def, _ := dependencyByID(job.DependencyID)
	finish := func(status, action, message, code string, check *dependencyCheck) {
		if !m.setJob(job.ID, status, action, message, code, check) {
			m.mu.Lock()
			m.closed = true
			m.mu.Unlock()
			m.cancel()
			return
		}
		m.gateway.auditDependency(stored.ActorID, job.ID, status)
	}
	if !m.gateway.dependencyIssuerAuthorized(stored.ActorID) {
		finish("failed", "none", "The administrator who requested this installation no longer has permission. An administrator can choose a fresh installation after checking tools.", "", nil)
		return
	}
	if !m.setJob(job.ID, "installing", "none", "Verifying the official source and installing "+def.name+". Windows may request administrator approval.", "", nil) {
		// Persisting dispatch is a prerequisite to every external mutation.
		m.mu.Lock()
		m.closed = true
		m.mu.Unlock()
		m.cancel()
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, 20*time.Minute)
	defer cancel()
	release, err := m.runner.installationLease(ctx)
	if err != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "interrupted"
		}
		finish(status, "check", "The computer's installation slot was unavailable or interrupted. Check tools before choosing a fresh installation.", "", nil)
		return
	}
	defer release()
	installer := m.checkInstaller(ctx)
	m.mu.Lock()
	m.state.Status.Installer = installer
	m.mu.Unlock()
	if installer.Status != "ready" {
		if ctx.Err() != nil {
			finish("interrupted", "check", "Installation preparation was interrupted. Check tools before an explicit retry.", "", nil)
		} else {
			finish("failed", "check", installer.Message, "", nil)
		}
		return
	}
	check, action := m.checkTool(ctx, def)
	if ctx.Err() != nil {
		finish("interrupted", "check", "Installation preparation was interrupted. Check tools before an explicit retry.", "", nil)
		return
	}
	if check.Status != "missing" {
		status := "action_required"
		if check.Status == "ready" {
			status = "ready"
		}
		finish(status, action, check.Message, "", &check)
		return
	}
	// Recheck after bounded probes, immediately before dispatch. Queued jobs do
	// not inherit authority from an old login or a worker's permission headers.
	if !m.gateway.dependencyIssuerAuthorized(stored.ActorID) {
		finish("failed", "none", "Your administrator permission changed before installation started. No installer was run.", "", nil)
		return
	}
	path, err := m.runner.lookup("winget")
	if err != nil {
		finish("action_required", "install_winget", "Windows Package Manager became unavailable. Reopen Luci and check tools.", "", nil)
		return
	}
	result := m.runner.run(ctx, path, "install", "--id", def.packageID, "--exact", "--source", "winget", "--dependency-source", "winget", "--silent", "--disable-interactivity", "--accept-source-agreements", "--accept-package-agreements", "--no-upgrade")
	code := fmt.Sprintf("0x%08X", result.Code)
	if m.ctx.Err() != nil {
		finish("interrupted", "check", "Luci closed before the installer finished. Check tools before an explicit retry.", code, nil)
		return
	}
	if ctx.Err() != nil {
		finish("interrupted", "check", "The installation exceeded its time limit. Check tools and Windows installer status before an explicit retry.", code, nil)
		return
	}
	status, required, message := dependencyInstallOutcome(result)
	if status == "ready" {
		check, required = m.checkTool(ctx, def)
		if check.Status == "missing" {
			status, required, message = "action_required", "restart_app", "The installer completed, but this running Luci process cannot find the new command. Save your work, reopen Luci and check tools."
		} else {
			message = check.Message
			if check.Status != "ready" {
				status = "action_required"
			}
		}
		finish(status, required, message, code, &check)
		return
	}
	finish(status, required, message, code, nil)
}

func dependencyInstallOutcome(result dependencyCommandResult) (string, string, string) {
	switch result.Code {
	case 3010, 1641, 0x8A150109, 0x8A15010A, 0x8A15010B:
		return "action_required", "restart_computer", "Windows reports that a restart is required. Save your work and restart the computer yourself, then check tools. Luci does not reboot it."
	case 5, 740, 1223, 0x80070005, 0x800704C7, 0x8A150019, 0x8A15010C:
		return "action_required", "elevation", "Windows administrator approval was required or cancelled. Complete any Windows approval prompt, then check tools. If nothing installed, use the official installer with administrator approval."
	case 0x8A150102, 1618:
		return "action_required", "check", "Another Windows installer is running. Wait for it to finish, then check tools before retrying."
	case 0x8A15003A, 0x8A15010F:
		return "action_required", "manual_install", "Windows policy blocked the installer. Ask the computer's administrator to install this tool, then check tools."
	case 0, 0x8A150061, 0x8A15010D:
		if result.Err == nil || result.Code != 0 {
			return "ready", "none", "The installer finished; checking whether the tool is available."
		}
	}
	return "failed", "check", "Windows Package Manager did not complete this installation. Check tools and retry, or use the publisher's official installer. The exit code is available for troubleshooting."
}

func (m *dependencyManager) interruptPending() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Jobs {
		j := &m.state.Jobs[i].Job
		if activeDependencyJob(j.Status) {
			j.Status, j.ActionRequired, j.Message, j.UpdatedAt = "interrupted", "check", "Luci stopped before this installation finished. Check tools before an explicit retry.", dependencyNow()
		}
	}
	m.attachJobsLocked()
	_ = m.saveLocked()
}

// Keep runtime.Platform in this management layer; a cloud worker's OS does not
// decide which installers can run on the computer hosting this gateway.
func dependencyHostPlatform() string { return runtime.GOOS }
