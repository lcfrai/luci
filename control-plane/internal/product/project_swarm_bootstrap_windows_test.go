package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// These tests need real Windows Git and the existing registered fixture owner.
// Future execution must select a short owned TMP root; there is no drive/junction
// fallback or skip that would turn an infeasible fixture into a passing result.
func bootstrapWindowsSource(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, _ := journeyFixture(t)
	source := filepath.Join(s.projectRoot(), "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, source, "init", "--initial-branch=main")
	confinedFixtureGit(t, source, "config", "--local", "core.longpaths", "true")
	return s, source, commitConfinedFixture(t, source, "bootstrap fixture")
}

func TestSwarmCheckoutWindowsLocalLongpathsBeforeFetch(t *testing.T) {
	s, source, _ := bootstrapWindowsSource(t)
	destination := filepath.Join(s.projectRoot(), "task")
	if err := swarmBootstrapFeasibility(destination); err != nil {
		t.Fatalf("this real Windows fixture requires an explicitly short owned TMP root: %v", err)
	}
	relative := strings.Repeat("nested-segment-12345/", 12) + "result.txt"
	file := filepath.Join(source, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	const contents = "exact long tracked file from the reviewed source\n"
	commit := swarmCommitFixture(t, source, relative, contents)
	if len(utf16.Encode([]rune(filepath.Join(destination, filepath.FromSlash(relative))))) <= 260 {
		t.Fatal("fixture did not exercise long tracked content")
	}
	if err := s.createSwarmCheckout(context.Background(), source, destination, commit); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(confinedFixtureGit(t, destination, "config", "--local", "--type=bool", "--get", "core.longpaths")); got != "true" {
		t.Fatal("isolated repository did not retain its own longpaths setting", got)
	}
	if got := strings.TrimSpace(confinedFixtureGit(t, destination, "rev-parse", "HEAD")); got != commit {
		t.Fatal("isolated checkout lost exact source commit", got, commit)
	}
	if data, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative))); err != nil || string(data) != contents {
		t.Fatal("long tracked source bytes were not checked out", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, ".git", "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("isolated repository borrowed source metadata", err)
	}
}

func TestSwarmCheckoutWindowsLocalLongpathsFailureStopsBeforeFetch(t *testing.T) {
	s, source, commit := bootstrapWindowsSource(t)
	for _, failure := range []string{"write-error", "read-error", "false", "missing", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			destination := filepath.Join(s.projectRoot(), "refused-"+failure)
			if err := swarmBootstrapFeasibility(destination); err != nil {
				t.Fatalf("short owned fixture root required: %v", err)
			}
			original := s.runner
			defer func() { s.runner = original }()
			writes, reads, transfers := 0, 0, 0
			s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
				if dir == destination {
					joined := strings.Join(args, "\x00")
					if strings.HasSuffix(joined, "config\x00--local\x00core.longpaths\x00true") {
						writes++
						if failure == "write-error" {
							return "", errors.New("controlled local configuration failure")
						}
					}
					if strings.HasSuffix(joined, "config\x00--local\x00--type=bool\x00--get\x00core.longpaths") {
						reads++
						switch failure {
						case "read-error":
							return "", errors.New("controlled local readback failure")
						case "false":
							return "false\n", nil
						case "missing":
							return "", nil
						case "malformed":
							return "true\nfalse\n", nil
						}
					}
					for _, arg := range args {
						if arg == "fetch" || arg == "checkout" {
							transfers++
						}
					}
				}
				return original(ctx, executable, args, dir)
			}
			if err := s.createSwarmCheckout(context.Background(), source, destination, commit); err == nil {
				t.Fatal("unconfirmed local setting allowed checkout")
			}
			if writes != 1 || transfers != 0 || failure == "write-error" && reads != 0 || failure != "write-error" && reads != 1 {
				t.Fatal("fixed configuration failure did not stop before source transfer", writes, reads, transfers)
			}
			if info, err := os.Lstat(filepath.Join(destination, ".git")); err != nil || !info.IsDir() {
				t.Fatal("partial owned repository was not retained", err)
			}
		})
	}
}

func TestSwarmCheckoutWindowsBootstrapRefusalBeforeMkdir(t *testing.T) {
	s, _ := journeyFixture(t)
	before := auditTree(t, s.projectRoot())
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("deterministic bootstrap refusal invoked Git")
		return "", errors.New("unexpected Git call")
	}
	destination := filepath.Join(s.projectRoot(), strings.Repeat("long-root-", 30), "task")
	if err := s.createSwarmCheckout(context.Background(), "unused-source", destination, strings.Repeat("a", 40)); !errors.Is(err, errSwarmBootstrapPath) {
		t.Fatal("infeasible bootstrap was not refused before allocation", err)
	}
	if !reflect.DeepEqual(before, auditTree(t, s.projectRoot())) {
		t.Fatal("bootstrap refusal created a directory")
	}
}

// Use the real configured-root constructor, not a production path override.
// The primary config fits ordinary Windows Git, but the generated worker's
// fixed-width ID makes its bootstrap metadata exceed our conservative policy.
func bootstrapWindowsHTTPFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, ProjectRun, string, *ProjectRunSelection) {
	t.Helper()
	owned := ownedFixtureTempDir(t)
	padding := 149 - len(utf16.Encode([]rune(owned))) - 1
	if padding < 1 || padding > 200 {
		t.Fatal("handler fixture requires the separately reviewed short owned TMP root")
	}
	projects := filepath.Join(owned, strings.Repeat("p", padding))
	if err := os.Mkdir(projects, 0700); err != nil {
		t.Fatal(err)
	}
	if err := validateOwnedFixtureDirectory(t, projects); err != nil {
		t.Fatal(err)
	}
	// Same harmless Git config isolation as journeyFixture; no global file or
	// TMP value is changed, and no path/feasibility production hook is installed.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "no-global-config"))
	t.Setenv("LUCI_GIT_NAME", "Luci Test Person")
	t.Setenv("LUCI_GIT_EMAIL", "person@example.com")
	s, err := New(Options{StateDir: t.TempDir(), ProjectsDir: projects})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.SetRuntimeResourceSnapshot(setupStorageFixtureSnapshot)
	installControlledProjectCheckRuntime(t, s, nil)
	installControlledPreviewRuntime(t, s)
	s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	mux := http.NewServeMux()
	s.registerProjects(fixtureReviewRegistrar{mux})
	s.registerProjectWork(mux)
	s.registerProjectRuns(mux)
	p := createJourneyProject(t, mux, "run-http")
	if windowsSwarmBootstrapFeasibility(p.Path) != nil || !windowsBootstrapUnitsFit(filepath.ToSlash(p.Path), ".git/config", 260) {
		t.Fatal("primary fixture Git metadata does not fit ordinary bootstrap")
	}
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	base := "/v1/product/projects/" + p.ID
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []ProjectTask{{ID: "task-save", Title: "Preserve the result", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"retained.txt"}, Acceptance: "The intended result is preserved"}}}, 200).Body.Bytes(), &plan)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Save reviewed task inputs"), 200)
	selected := &ProjectRunSelection{ProjectID: p.ID, WorktreeID: project.WorktreeID(p.Path), Path: p.Path, Harness: "codex", Model: "fixture-model", PermissionMode: "bypass", AgentConnectionID: "connection-fixture", AgentConnectionRevision: 1, AgentConnectionKind: "api", AgentConnectionProtocol: "openai-responses", AuthMode: "api-key", Connection: AgentConnectionReceipt{ID: "connection-fixture", Revision: 1, Kind: "api", Protocol: "openai-responses", AuthMode: "api-key", Model: "fixture-model"}}
	s.SetProjectRunSelectionResolver(func(_ context.Context, request ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		if request.ProjectID != p.ID || request.WorktreeID != selected.WorktreeID {
			return ProjectRunSelection{}, errors.New("foreign selected worktree")
		}
		return *selected, nil
	})
	id := selected.AgentConnectionID
	request := projectRunCreateRequest{IdempotencyKey: "bootstrap-http-run-123456789", WorktreeID: selected.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, AgentConnectionID: &id, Outcome: "preview", ImplementationContext: ImplementationContextRequest{ProjectID: p.ID, BriefRevision: spec.BriefRevision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"task-save"}}}
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", request, 201)).ProjectRun
	base += "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, map[string]int{"expectedRevision": run.Revision, "maxParallel": 1}, 201)).ProjectRun
	// Every new operation has the same27 ASCII units. This generated sample is
	// geometry only: it is not a persisted operation or authority to allocate.
	operationID := newID("op-")
	worker, err := s.swarmPath(p.ID, run.ID, operationID)
	if err != nil || len(operationID) != 27 || !errors.Is(windowsSwarmBootstrapFeasibility(worker), errSwarmBootstrapPath) || !windowsBootstrapUnitsFit(filepath.ToSlash(worker), ".git/config", 260) {
		t.Fatal("fixture did not separate valid primary/config from infeasible worker bootstrap", err)
	}
	if len(utf16.Encode([]rune(filepath.Join(worker, ".git", "config")))) != 242 || len(utf16.Encode([]rune(filepath.Join(worker, ".git", "objects", "info")))) != 248 {
		t.Fatal("fixture is not the shortest one-unit policy rejection geometry")
	}
	if _, err = s.swarmGit(context.Background(), p.Path, "rev-parse", "HEAD"); err != nil {
		t.Fatal("real primary Git source is not readable", err)
	}
	return s, mux, p, run, base, selected
}

func assertBootstrapHTTPRefusal(t *testing.T, s *Server, mux *http.ServeMux, p project.Project, run ProjectRun, url string, body map[string]any) {
	t.Helper()
	scope, err := s.projectRuns.projectScope(p.ID, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	beforeRow, err := s.projectRuns.read(p.ID, scope, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, beforeSource := auditTree(t, s.stateDir), auditTree(t, s.projectRoot())
	beforeHandoffs := bootstrapHandoffSnapshot(t, s.projectRuns, p.ID)
	for range 2 {
		response := journeyRequest(t, mux, "POST", url, body, http.StatusConflict)
		var problem struct {
			Error string `json:"error"`
			Message string `json:"message"`
		}
		decodeEvidenceResponse(t, response.Body.Bytes(), &problem)
		if problem.Error != "project_run_conflict" || problem.Message != errSwarmBootstrapPath.Error() {
			t.Fatal("actual Server caller did not preserve fixed path guidance", problem)
		}
		row, readErr := s.projectRuns.read(p.ID, scope, run.ID)
		if readErr != nil || !reflect.DeepEqual(beforeRow, row) || !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeSource, auditTree(t, s.projectRoot())) || !reflect.DeepEqual(beforeHandoffs, bootstrapHandoffSnapshot(t, s.projectRuns, p.ID)) {
			t.Fatal("actual handler refusal mutated attempt, repair, dependency, handoff or original source", readErr)
		}
	}
}

func TestSwarmWindowsNormalHandlerBootstrapRefusal(t *testing.T) {
	s, mux, p, run, base, _ := bootstrapWindowsHTTPFixture(t)
	assertBootstrapHTTPRefusal(t, s, mux, p, run, base+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "normal-long-root-refusal-123456"})
}

func TestSwarmWindowsCheckpointHandlerBootstrapRefusal(t *testing.T) {
	s, mux, p, run, base, selected := bootstrapWindowsHTTPFixture(t)
	// Construct an actual historical stopped workspace at its exact owned path.
	// This setup deliberately uses the existing store API and real confined
	// clone, not the new Server admission; it is not a product bypass or live
	// execution receipt. Its config is below260 even though policy refuses new
	// bootstrap there. Unsupported real clone/setup must fail, never skip.
	t.Log("historical checkpoint setup begins; no checkpoint handler POST has been exercised")
	var originalPath string
	var pathErr error
	run, prior, created, err := s.projectRuns.ReserveSwarmTask(p.ID, p.Path, run.ID, run.Revision, "historical-bootstrap-attempt-123456", "task-save", run.Input.Implementation.SourceCommit, func(operationID string) string {
		originalPath, pathErr = s.swarmPath(p.ID, run.ID, operationID)
		if pathErr != nil {
			return ""
		}
		return project.WorktreeID(originalPath)
	})
	if err != nil || pathErr != nil || !created || !errors.Is(windowsSwarmBootstrapFeasibility(originalPath), errSwarmBootstrapPath) || !windowsBootstrapUnitsFit(filepath.ToSlash(originalPath), ".git/config", 260) {
		t.Fatal("historical fixture reservation/geometry failed", created, err)
	}
	if err = os.MkdirAll(filepath.Dir(originalPath), 0700); err != nil {
		t.Fatal(err)
	}
	// This explicit fixture-only option supports pack filenames after ordinary
	// bootstrap. It is not inherited global config or a product gate bypass.
	confinedFixtureGit(t, filepath.Dir(originalPath), "-c", "core.longpaths=true", "clone", "--no-local", p.Path, originalPath)
	confinedFixtureGit(t, originalPath, "config", "--local", "core.longpaths", "true")
	if got := confinedFixtureGit(t, originalPath, "config", "--local", "--type=bool", "--get", "core.longpaths"); got != "true" {
		t.Fatal("historical fixture setup did not retain local longpaths; handler not exercised", got)
	}
	if got := confinedFixtureGit(t, originalPath, "rev-parse", "HEAD"); got != prior.BaseCommit {
		t.Fatal("historical clone did not preserve exact actual base", got)
	}
	binding, err := s.swarmTaskBinding(context.Background(), run, prior, originalPath)
	if err != nil {
		t.Fatal(err)
	}
	workerRoute := *selected
	workerRoute.WorktreeID = prior.WorktreeID
	run, err = s.projectRuns.CompleteSwarmAllocation(p.ID, p.Path, run.ID, run.Revision, prior.ID, binding, projectRunCandidate(workerRoute, binding), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(originalPath, "retained.txt"), []byte("actual retained checkpoint bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, prior.ID, prior.ChildID, "failed", "controlled historical stopped-attempt fixture")
	if err != nil {
		t.Fatal(err)
	}
	s.SetProjectSwarmCheckpointGuard(func(_ context.Context, request ProjectSwarmRecoveryRequest) error {
		if request.ProjectID != p.ID || request.RunID != run.ID || request.OperationID != prior.ID || request.ChildID != prior.ChildID {
			return errors.New("foreign historical checkpoint identity")
		}
		return nil
	}, func(context.Context, ProjectSwarmRecoveryRequest) (func(), error) { return func() {}, nil })
	url := base + "/operations/" + prior.ID + "/checkpoint"
	review := checkpointReviewHTTP(t, mux, url)
	if !review.CanContinue || len(review.Files) != 1 || review.Files[0].After != "actual retained checkpoint bytes\n" {
		t.Fatal("actual original checkpoint was not reviewable", review)
	}
	t.Log("historical setup and actual checkpoint review completed; exercising real checkpoint POST refusal")
	assertBootstrapHTTPRefusal(t, s, mux, p, run, url, checkpointBody(review, "checkpoint-long-root-refusal-123456"))
}
