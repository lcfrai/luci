package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/dockerinfo"
)

const dependencyTrustedSourceFixture = `{"Arg":"https://cdn.winget.microsoft.com/cache","Data":"Microsoft.Winget.Source_8wekyb3d8bbwe","Identifier":"Microsoft.Winget.Source_8wekyb3d8bbwe","Name":"winget","TrustLevel":["Trusted","StoreOrigin"],"Type":"Microsoft.PreIndexed.Package"}`

type dependencyProcessCall struct {
	Executable string
	Args       []string
}
type fakeDependencyRunner struct {
	mu            sync.Mutex
	installed     map[string]bool
	versionOutput map[string]string
	sourceTrusted bool
	engineRunning bool
	calls         []dependencyProcessCall
	started       chan string
	release       chan struct{}
	active, peak  int
	platformName  string
	installCode   uint32
	keepMissing   bool
}

func newFakeDependencyRunner() *fakeDependencyRunner {
	return &fakeDependencyRunner{installed: map[string]bool{}, sourceTrusted: true, engineRunning: true, started: make(chan string, 10), platformName: "windows"}
}
func (f *fakeDependencyRunner) platform() string { return f.platformName }
func (f *fakeDependencyRunner) engineInfo(context.Context, string) (dockerinfo.Info, error) {
	f.mu.Lock()
	ready := f.engineRunning
	f.mu.Unlock()
	if !ready {
		return dockerinfo.Info{}, errors.New("private-context-path must never be returned")
	}
	return dockerinfo.Info{ServerVersion: "28.5.1"}, nil
}
func (f *fakeDependencyRunner) installationLease(context.Context) (func(), error) {
	return func() {}, nil
}
func (f *fakeDependencyRunner) lookup(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == "winget" || f.installed[name] {
		return filepath.Join(os.TempDir(), name+".exe"), nil
	}
	return "", errors.New("not installed")
}
func (f *fakeDependencyRunner) run(ctx context.Context, executable string, args ...string) dependencyCommandResult {
	f.mu.Lock()
	f.calls = append(f.calls, dependencyProcessCall{executable, append([]string{}, args...)})
	name := strings.TrimSuffix(filepath.Base(executable), ".exe")
	if name == "winget" && args[0] == "source" {
		trusted := f.sourceTrusted
		f.mu.Unlock()
		if trusted {
			return dependencyCommandResult{Output: dependencyTrustedSourceFixture}
		}
		return dependencyCommandResult{Output: strings.ReplaceAll(dependencyTrustedSourceFixture, microsoftWingetSource, "https://untrusted.example.invalid/packages")}
	}
	if name == "winget" && args[0] == "install" {
		packageID := args[2]
		f.active++
		if f.active > f.peak {
			f.peak = f.active
		}
		f.started <- packageID
		release, code, keepMissing := f.release, f.installCode, f.keepMissing
		f.mu.Unlock()
		if release != nil {
			select {
			case <-release:
			case <-ctx.Done():
				f.mu.Lock()
				f.active--
				f.mu.Unlock()
				return dependencyCommandResult{Code: 1, Err: ctx.Err()}
			}
		}
		f.mu.Lock()
		f.active--
		if code == 0 && !keepMissing {
			for _, definition := range dependencyCatalogue {
				if definition.packageID == packageID {
					f.installed[definition.executable] = true
				}
			}
		}
		f.mu.Unlock()
		return dependencyCommandResult{Code: code, Output: "publisher output is not public"}
	}
	engine := f.engineRunning
	versionOverride, overridden := f.versionOutput[name]
	f.mu.Unlock()
	if name == "docker" && args[0] == "info" {
		if !engine {
			return dependencyCommandResult{Code: 1, Err: errors.New("daemon unavailable"), Output: "private-context-path must never be returned"}
		}
		return dependencyCommandResult{Output: "28.5.1"}
	}
	if overridden {
		return dependencyCommandResult{Output: versionOverride}
	}
	versions := map[string]string{
		"git": "git version 2.51.0.windows.1\n", "docker": "Docker version 28.5.1, build xxx\n",
		"gh": "gh version 2.80.0 (2026-01-01)\n", "aws": "aws-cli/2.31.1 Python/3.13 Windows/11 exe/AMD64\n",
		"az": `{"azure-cli":"2.88.0","azure-cli-core":"2.88.0","azure-cli-telemetry":"1.1.0","extensions":{}}`,
	}
	return dependencyCommandResult{Output: versions[name]}
}
func (f *fakeDependencyRunner) recordedCalls() []dependencyProcessCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]dependencyProcessCall{}, f.calls...)
}

func dependencyFixture(t *testing.T) (*Gateway, http.Handler, string, *fakeDependencyRunner) {
	t.Helper()
	g, h, token, _ := fixture(t, nil)
	if err := g.dependencies.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake := newFakeDependencyRunner()
	m, err := newDependencyManager(g, fake)
	if err != nil {
		t.Fatal(err)
	}
	g.dependencies = m
	return g, h, token, fake
}
func decodeDependencyStatus(t *testing.T, data []byte) dependencyStatus {
	t.Helper()
	var result dependencyStatus
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func decodeDependencyJob(t *testing.T, data []byte) dependencyJob {
	t.Helper()
	var result dependencyJob
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func awaitDependencyJob(t *testing.T, manager *dependencyManager, id, status string) dependencyJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, row := range manager.snapshot(true).Dependencies {
			if row.Job != nil && row.Job.ID == id && row.Job.Status == status {
				return *row.Job
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach %s: %+v", id, status, manager.snapshot(true))
	return dependencyJob{}
}

func TestDependencyEndpointsAreCachedScopedAndRequireExplicitAdministratorAgreement(t *testing.T) {
	_, h, ownerToken, fake := dependencyFixture(t)
	addUser(t, h, ownerToken, "member-tools", "member")
	memberToken := loginAs(t, h, "member-tools")
	request(t, h, "GET", "/v1/setup/dependencies", "", nil, 401)
	member := decodeDependencyStatus(t, request(t, h, "GET", "/api/v1/setup/dependencies", memberToken, nil, 200).Body.Bytes())
	if member.Scope != "control-plane-host" || member.Platform != "windows" || member.CanInstall || len(member.Dependencies) != 5 || member.Installer.Status != "unchecked" {
		t.Fatalf("unexpected initial member status: %+v", member)
	}
	request(t, h, "POST", "/v1/setup/dependencies/check", memberToken, map[string]any{}, 403)
	request(t, h, "POST", "/v1/setup/dependencies/git/install", memberToken, map[string]any{"acceptAgreements": true}, 403)
	request(t, h, "POST", "/v1/setup/dependencies/git/install", ownerToken, map[string]any{"acceptAgreements": false}, 400)
	request(t, h, "POST", "/v1/setup/dependencies/arbitrary/install", ownerToken, map[string]any{"acceptAgreements": true}, 404)
	request(t, h, "POST", "/v1/setup/dependencies/git/install", ownerToken, map[string]any{"acceptAgreements": true, "packageId": "malicious.package"}, 400)
	request(t, h, "POST", "/v1/setup/dependencies/git/install", ownerToken, map[string]any{"acceptAgreements": true}, 409)
	if len(fake.recordedCalls()) != 0 {
		t.Fatal("cached reads or rejected requests ran a host process")
	}
	checked := decodeDependencyStatus(t, request(t, h, "POST", "/v1/setup/dependencies/check", ownerToken, map[string]any{}, 200).Body.Bytes())
	if !checked.CanInstall || checked.Installer.Status != "ready" {
		t.Fatalf("official source not recognized: %+v", checked)
	}
	for _, row := range checked.Dependencies {
		if row.Check.Status != "missing" || row.ActionRequired != "none" || row.Job != nil {
			t.Fatalf("unexpected missing tool: %+v", row)
		}
	}
	before := len(fake.recordedCalls())
	request(t, h, "GET", "/v1/setup/dependencies", ownerToken, nil, 200)
	if len(fake.recordedCalls()) != before {
		t.Fatal("GET performed readiness probes")
	}
}

func TestDependencyCheckDistinguishesDockerCLIFromEngineAndReturnsOnlySafeVersions(t *testing.T) {
	_, h, token, fake := dependencyFixture(t)
	for _, d := range dependencyCatalogue {
		fake.installed[d.executable] = true
	}
	fake.engineRunning = false
	w := request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200)
	if strings.Contains(w.Body.String(), "private-context-path") {
		t.Fatal("raw Docker diagnostics escaped into public status")
	}
	status := decodeDependencyStatus(t, w.Body.Bytes())
	for _, row := range status.Dependencies {
		if row.ID == "docker" {
			if row.Check.Status != "action_required" || row.Check.Version != "28.5.1" || row.ActionRequired != "start_docker" {
				t.Fatalf("Docker CLI treated as missing or engine as ready: %+v", row)
			}
		} else if row.Check.Status != "ready" || row.Check.Version == "" {
			t.Fatalf("tool was not verified: %+v", row)
		}
	}
	request(t, h, "POST", "/v1/setup/dependencies/docker/install", token, map[string]any{"acceptAgreements": true}, 409)
}

func TestDependencyAzureVersionAcceptsExtensionMetadataButRejectsInvalidCLIVersion(t *testing.T) {
	definition, ok := dependencyByID("azure")
	if !ok {
		t.Fatal("Azure CLI must remain in the supported dependency catalogue")
	}
	for _, test := range []struct {
		name, output, version string
	}{
		{"actual bundled version shape", `{"azure-cli":"2.88.0","azure-cli-core":"2.88.0","azure-cli-telemetry":"1.1.0","extensions":{}}`, "2.88.0"},
		{"nested installed extension", `{"azure-cli":"2.88.0","extensions":{"containerapp":{"version":"1.0.0","preview":false}}}`, "2.88.0"},
		{"malformed JSON", `{"azure-cli":"2.88.0","extensions":`, ""},
		{"missing CLI version", `{"extensions":{}}`, ""},
		{"numeric CLI version", `{"azure-cli":2.88,"extensions":{}}`, ""},
		{"object CLI version", `{"azure-cli":{"version":"2.88.0"},"extensions":{}}`, ""},
		{"array CLI version", `{"azure-cli":["2.88.0"],"extensions":{}}`, ""},
		{"boolean CLI version", `{"azure-cli":true,"extensions":{}}`, ""},
		{"null CLI version", `{"azure-cli":null,"extensions":{}}`, ""},
		{"invalid version text", `{"azure-cli":"2.88.0 private-context-path","extensions":{}}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := newFakeDependencyRunner()
			runner.platformName = "linux"
			runner.installed["az"] = true
			runner.versionOutput = map[string]string{"az": test.output}
			manager := &dependencyManager{runner: runner}
			check, action := manager.checkTool(context.Background(), definition)
			if test.version != "" {
				if check.Status != "ready" || check.Version != test.version || action != "none" {
					t.Fatalf("working Azure CLI rejected: %+v, action=%s", check, action)
				}
			} else if check.Status != "action_required" || check.Version != "" || action != "manual_install" {
				t.Fatalf("invalid version treated as installed: %+v, action=%s", check, action)
			}
			if strings.Contains(check.Message, "private-context-path") {
				t.Fatal("untrusted version output escaped into public readiness")
			}
			calls := runner.recordedCalls()
			if len(calls) != 1 || !reflect.DeepEqual(calls[0].Args, []string{"version", "--output", "json"}) {
				t.Fatalf("readiness used an unexpected command: %+v", calls)
			}
		})
	}
}

func TestDependencyQueueSerializesAccountsAndRechecksIssuerAtDispatch(t *testing.T) {
	g, h, ownerToken, fake := dependencyFixture(t)
	admin := addUser(t, h, ownerToken, "tools-admin", "admin")
	adminToken := loginAs(t, h, "tools-admin")
	request(t, h, "POST", "/v1/setup/dependencies/check", adminToken, map[string]any{}, 200)
	fake.release = make(chan struct{})
	first := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/git/install", adminToken, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	select {
	case packageID := <-fake.started:
		if packageID != "Git.Git" {
			t.Fatal(packageID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first installer did not start")
	}
	duplicate := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/git/install", ownerToken, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	if duplicate.ID != first.ID {
		t.Fatal("duplicate installation did not share the active job")
	}
	revoked := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/azure/install", adminToken, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	second := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/aws/install", ownerToken, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	request(t, h, "POST", "/v1/setup/dependencies/check", ownerToken, map[string]any{}, 409)
	request(t, h, "PATCH", "/v1/admin/users/"+admin.ID, ownerToken, map[string]any{"role": "member"}, 200)
	close(fake.release)
	awaitDependencyJob(t, g.dependencies, first.ID, "ready")
	denied := awaitDependencyJob(t, g.dependencies, revoked.ID, "failed")
	if !strings.Contains(denied.Message, "permission") {
		t.Fatal("revoked queued request did not explain why it was refused")
	}
	awaitDependencyJob(t, g.dependencies, second.ID, "ready")
	fake.mu.Lock()
	peak := fake.peak
	fake.mu.Unlock()
	if peak != 1 {
		t.Fatalf("%d host installers ran concurrently", peak)
	}
	installedPackages := []string{}
	for _, call := range fake.recordedCalls() {
		if len(call.Args) > 0 && call.Args[0] == "install" {
			installedPackages = append(installedPackages, call.Args[2])
			expected := []string{"install", "--id", call.Args[2], "--exact", "--source", "winget", "--dependency-source", "winget", "--silent", "--disable-interactivity", "--accept-source-agreements", "--accept-package-agreements", "--no-upgrade"}
			if !reflect.DeepEqual(call.Args, expected) || strings.Contains(strings.Join(call.Args, " "), "reboot") {
				t.Fatalf("installer arguments were not the fixed reviewed vector: %+v", call)
			}
		}
	}
	if !reflect.DeepEqual(installedPackages, []string{"Git.Git", "Amazon.AWSCLI"}) {
		t.Fatalf("revoked or duplicate request invoked installer: %v", installedPackages)
	}
	state, err := os.ReadFile(g.dependencies.path)
	if err != nil || strings.Contains(string(state), "publisher output") {
		t.Fatal("job state contains raw installer diagnostics or was not persisted")
	}
}

func TestDependencySourceIsReverifiedBeforeAnyInstallerRuns(t *testing.T) {
	g, h, token, fake := dependencyFixture(t)
	request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200)
	fake.sourceTrusted = false
	job := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/git/install", token, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	failed := awaitDependencyJob(t, g.dependencies, job.ID, "failed")
	if !strings.Contains(failed.Message, "verify") {
		t.Fatal("source verification failure was not actionable")
	}
	for _, call := range fake.recordedCalls() {
		if call.Args[0] == "install" {
			t.Fatal("a changed source was trusted from its cached status")
		}
	}
	request(t, h, "POST", "/v1/setup/dependencies/git/install", token, map[string]any{"acceptAgreements": true}, 409)
}

func TestDependencyInterruptedJobsRemainDurableAndNeverRunAtStartup(t *testing.T) {
	g, h, token, _ := dependencyFixture(t)
	request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200)
	if err := g.dependencies.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	actorID := g.state.OwnerID
	g.mu.Unlock()
	state := initialDependencyState("windows")
	for i, id := range []string{"git", "aws"} {
		status := "queued"
		if i == 1 {
			status = "installing"
		}
		state.Jobs = append(state.Jobs, storedDependencyJob{ActorID: actorID, Job: dependencyJob{ID: "recorded-" + id, DependencyID: id, Status: status, CreatedAt: dependencyNow(), UpdatedAt: dependencyNow(), ActionRequired: "none"}})
	}
	data, _ := json.Marshal(state)
	if err := atomicPrivate(g.dependencies.path, data); err != nil {
		t.Fatal(err)
	}
	freshFake := newFakeDependencyRunner()
	reopened, err := newDependencyManager(g, freshFake)
	if err != nil {
		t.Fatal(err)
	}
	g.dependencies = reopened
	for _, row := range reopened.snapshot(true).Dependencies {
		if row.ID == "git" || row.ID == "aws" {
			if row.Job == nil || row.Job.Status != "interrupted" || row.Job.ActionRequired != "check" {
				t.Fatalf("startup lost the interrupted operation: %+v", row)
			}
		}
	}
	request(t, h, "GET", "/v1/setup/dependencies", token, nil, 200)
	if len(freshFake.recordedCalls()) != 0 {
		t.Fatal("startup or GET automatically retried a pending installation")
	}
}

func TestDependencyClosingCancelsTheOwnedInstallerAndPreservesItsInterruptedJob(t *testing.T) {
	g, h, token, fake := dependencyFixture(t)
	request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200)
	fake.release = make(chan struct{})
	job := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/git/install", token, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	select {
	case <-fake.started:
	case <-time.After(5 * time.Second):
		t.Fatal("installer did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.Close(ctx); err != nil {
		t.Fatal(err)
	}
	awaitDependencyJob(t, g.dependencies, job.ID, "interrupted")
	fake.mu.Lock()
	active := fake.active
	fake.mu.Unlock()
	if active != 0 {
		t.Fatal("closing the gateway left its installer running")
	}
	data, err := os.ReadFile(g.dependencies.path)
	if err != nil || !strings.Contains(string(data), `"status":"interrupted"`) {
		t.Fatal("cancelled installation outcome was not saved")
	}
}

func TestDependencyNonWindowsHostsCheckToolsWithoutEnablingAnInstaller(t *testing.T) {
	g, h, token, _ := dependencyFixture(t)
	if err := g.dependencies.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake := newFakeDependencyRunner()
	fake.platformName = "linux"
	fake.installed["git"] = true
	manager, err := newDependencyManager(g, fake)
	if err != nil {
		t.Fatal(err)
	}
	g.dependencies = manager
	status := decodeDependencyStatus(t, request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200).Body.Bytes())
	if status.Platform != "linux" || status.CanInstall || status.Installer.Status != "unsupported" || status.Dependencies[0].Check.Status != "ready" {
		t.Fatalf("non-Windows host was represented incorrectly: %+v", status)
	}
	request(t, h, "POST", "/v1/setup/dependencies/docker/install", token, map[string]any{"acceptAgreements": true}, 409)
	for _, call := range fake.recordedCalls() {
		if strings.Contains(call.Executable, "winget") {
			t.Fatal("non-Windows host executed Windows Package Manager")
		}
	}
}

func TestDependencySuccessfulInstallerStillRequiresCommandAvailabilityAndFreshChecks(t *testing.T) {
	g, h, token, fake := dependencyFixture(t)
	request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200)
	fake.keepMissing = true
	job := decodeDependencyJob(t, request(t, h, "POST", "/v1/setup/dependencies/git/install", token, map[string]any{"acceptAgreements": true}, 202).Body.Bytes())
	finished := awaitDependencyJob(t, g.dependencies, job.ID, "action_required")
	if finished.ActionRequired != "restart_app" {
		t.Fatal("successful installer incorrectly claimed the command was ready")
	}
	request(t, h, "POST", "/v1/setup/dependencies/git/install", token, map[string]any{"acceptAgreements": true}, 409)
	fake.mu.Lock()
	fake.installed["git"] = true
	fake.mu.Unlock()
	status := decodeDependencyStatus(t, request(t, h, "POST", "/v1/setup/dependencies/check", token, map[string]any{}, 200).Body.Bytes())
	if status.Dependencies[0].Check.Status != "ready" || status.Dependencies[0].ActionRequired != "none" || status.Dependencies[0].Job != nil {
		t.Fatalf("historical restart action overrode a fresh successful check: %+v", status.Dependencies[0])
	}
}

func TestDependencyInstallerExitCodesExposeActionsWithoutRebootOrElevationBypass(t *testing.T) {
	for _, tc := range []struct {
		code   uint32
		action string
	}{
		{3010, "restart_computer"}, {0x8A150109, "restart_computer"}, {0x8A15010A, "restart_computer"},
		{1223, "elevation"}, {0x8A150019, "elevation"}, {0x8A15010C, "elevation"},
		{1618, "check"}, {0x8A15010F, "manual_install"},
	} {
		status, action, _ := dependencyInstallOutcome(dependencyCommandResult{Code: tc.code, Err: errors.New("exit")})
		if status != "action_required" || action != tc.action {
			t.Fatalf("exit %#x: %s/%s", tc.code, status, action)
		}
	}
	if status, _, _ := dependencyInstallOutcome(dependencyCommandResult{Err: errors.New("process did not start")}); status != "failed" {
		t.Fatal("failure to start a process was treated as successful exit 0")
	}
	base := []string{"PATH=tools", "USERPROFILE=profile", "GH_TOKEN=private-gh", "OPENAI_API_KEY=private-model", "AWS_ACCESS_KEY_ID=private-aws", "PYTHONPATH=untrusted", "LUCI_WORKER_TOKEN=private-worker"}
	got := dependencyProcessEnvironment(base)
	if !reflect.DeepEqual(got, []string{"PATH=tools", "USERPROFILE=profile"}) {
		t.Fatalf("host installer inherited unrelated credentials or executable module paths: %v", got)
	}
}
