package product

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type productRouteRecorder struct {
	*http.ServeMux
	patterns []string
}

func (r *productRouteRecorder) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
	r.ServeMux.HandleFunc(pattern, handler)
}

// Adding a GET also requires a concrete reason it cannot provision resources,
// change credentials, start a session or allocate a preview listener.
var productReadOnlyRoutes = map[string]string{
	"GET /v1/product/swarm-scheduler":                                                          "Reads private scheduler settings and already-cached resource observations; no host sampling, queue repair, allocation or dispatch.",
	"GET /v1/product/projects/{id}/swarm-queue":                                                "Reads only this project’s durable queue projection; no host sampling, child observation, ledger repair or dispatch.",
	"GET /v1/product/projects/{id}/overview":                                                   "Reads bounded compact canonical context, private question projections and recorded queue counts; never migrates, repairs, reconciles, executes or writes on GET/HEAD.",
	"GET /v1/product/projects/{id}/runs/{runId}/swarm/tasks/{taskId}/review":                   "Reads the private task reservation, stopped-session state and bounded committed Git diff; cannot reserve, integrate, repair or launch a child.",
	"GET /v1/product/projects/{id}/runs/{runId}/swarm/operations/{operationId}/workspace":      "Reads retained ordinary files through pinned directory handles and bounded optional Git inspection; no allocation, repair, integration or writes, including incomplete attempts.",
	"GET /v1/product/projects/{id}/runs/{runId}/swarm/operations/{operationId}/workspace/file": "Reads one bounded ordinary retained file after private ownership, path and opened-handle checks; cannot create a missing checkout or repair its receipt.",
	"GET /v1/product/project-imports/{attemptId}":                                              "Reads an owned private import receipt and matching directory identities; no clone, promotion, retry or writes.",
	"GET /v1/product/projects/{id}/orchestrator":                                               "Reads the private conversation, saved context and bounded Git status in memory; no inference, reconciliation, approval or mutation.",
	"GET /v1/product/projects/{id}/orchestrator/export":                                        "Exports existing bounded private conversation and archive bytes with a current digest; does not rotate, prune, repair or create files.",
	"GET /v1/product/projects/{id}/context-history":                                            "Reads bounded private authenticated edit/review receipts; creates no files, authority, context changes or resources.",
	"GET /v1/product/projects/{id}/context":                                                    "Reads canonical portable context identity and compares generated export bytes without repair or writes.",
	"GET /v1/product/projects/{id}/context/archive":                                            "Reads bounded immutable context history chunks and retention metadata without pruning or writes.",
	"GET /v1/product/projects/{id}/runs":                                                       "Reads bounded private run history and original evidence availability; no observation, repair or dispatch.",
	"GET /v1/product/projects/{id}/runs/{runId}":                                               "Reads one private run and retained evidence availability; no observation, repair or dispatch.",
	"GET /v1/product/projects/{id}/git/branches":                                               "Inspects bounded native refs, HEAD and clean state, then reads session state and private receipts; no checkout, sessions or writes.",
	"GET /v1/product/projects/{id}/git/branch-operations/{operationId}":                        "Reads the private intent/result and may observe matching native HEAD in memory; never retries checkout or updates the journal.",
	"GET /v1/product/projects/{id}/design-plan":                                                "Reads saved screen/state handoff and compares in-memory revisions; no writes or execution.",
	"GET /v1/product/projects/{id}/design-plan/history":                                        "Reads immutable saved design handoff snapshots without starting resources.",
	"GET /v1/product/projects/{id}/specification":                                              "Reads saved definition or derives a legacy summary in memory; no writes, models or processes.",
	"GET /v1/product/projects/{id}/specification/history":                                      "Reads immutable saved definition snapshots; no resources or writes.",
	"GET /v1/product/projects/{id}/configuration":                                              "Reads only names/revision from private runtime configuration; no values, writes or execution.",
	"GET /v1/setup/status":                                                                     "Reads local setup or bounded remote setup status; no provisioning.",
	"GET /v1/setup/cloud/jobs":                                                                 "Reads cached provisioning history.",
	"GET /v1/setup/cloud/login":                                                                "Reads cached explicit-login jobs.",
	"GET /v1/setup/targets":                                                                    "Reads saved connection descriptions without their tokens.",
	"GET /v1/setup/ssh/connections":                                                            "Reads already-created tunnels; cannot connect or accept a key.",
	"GET /v1/setup/git/identity":                                                               "Two bounded git config --global --get calls from host-owned state.",
	"GET /v1/setup/runtime/install":                                                            "Reads cached image-build state; cannot run Docker.",
	"GET /v1/setup/workspace/install":                                                          "Reads cached explicit server-install jobs.",
	"GET /v1/setup/workspace/auth":                                                             "Reads the selected verified SSH backing identity with bounded GET requests; cannot bootstrap or sign in.",
	"GET /v1/setup/workspace/admin/me":                                                         "Reads the active backing identity after local administrator and private SSH checks; cannot sign in.",
	"GET /v1/setup/workspace/admin/users":                                                      "Reads public backing user records through a bounded authenticated GET; cannot create users.",
	"GET /v1/setup/workspace/admin/invitations":                                                "Reads bounded invitation metadata on the selected backing; no code readback, creation or redemption.",
	"GET /v1/setup/workspace/admin/policy":                                                     "Reads backing policy through a bounded authenticated GET; cannot update it.",
	"GET /v1/setup/workspace/admin/audit":                                                      "Reads existing backing audit events; cannot execute or provision resources.",
	"GET /v1/setup/workspace/admin/users/{id}/runtime":                                         "Reads cached backing runtime state; cannot start or stop a worker.",
	"GET /v1/setup/github/login":                                                               "Reads cached explicit GitHub login jobs without running CLI commands.",
	"GET /v1/setup/agents":                                                                     "Reports presence of the private agent credential without its value.",
	"GET /v1/setup/agent-connections":                                                          "Reads public saved route identities, protocol bindings and key presence without secrets or provider requests.",
	"GET /v1/product/projects/{id}/brief":                                                      "Reads canonical brief history without creating missing state.",
	"GET /v1/product/projects/{id}/design":                                                     "Reads canonical design history without creating missing state.",
	"GET /v1/product/projects/{id}/design/export":                                              "Renders an inert HTML response from saved design state.",
	"GET /v1/product/projects/{id}/files":                                                      "Reads contained regular files/directories; rejects credential paths.",
	"GET /v1/product/projects/{id}/git":                                                        "Bounded Git inspection with hooks, filters and optional writes disabled.",
	"GET /v1/product/projects/{id}/releases":                                                   "Reads saved release rows; never builds, deploys or opens a listener.",
	"GET /v1/product/projects/{id}/prototype-releases":                                         "Reads owned release histories and bounded native ownership/health observations; never builds, starts resources, writes state or opens a listener.",
	"GET /v1/product/projects/{id}/plan":                                                       "Reads canonical task progress; cannot start checks or complete tasks.",
	"GET /v1/product/projects/{id}/checks":                                                     "Reads saved bounded execution evidence and suggested command names; no commands execute.",
	"GET /v1/product/projects/{id}/checks/{runId}":                                             "Reads saved execution evidence without starting a process.",
	"GET /v1/product/projects/{id}/git/delivery":                                               "Inspects current Git branch/commit and saved publication evidence; cannot create or push a repository.",
}

func auditTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(" %x", sha256.Sum256(data))
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProductRegisteredReadsCannotMutateProjectsOrStartResources(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "read-only-app")
	recorded := &productRouteRecorder{ServeMux: http.NewServeMux()}
	s.Register(recorded)
	importRootIdentity, err := projectImportDirectoryIdentity(s.projectRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeImportReceipt(&projectImportReceipt{Version: 1, projectImportView: projectImportView{ID: importTestID, Name: "read-only-import", Status: "failed", Message: "Fixture import failed before publication."}, ActorID: "fixture-reviewer", Root: s.projectRoot(), RootIdentity: importRootIdentity, Phase: "reserved"}); err != nil {
		t.Fatal(err)
	}
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	identityDir, err := s.setupHostDir(false)
	if err != nil {
		t.Fatal(err)
	}
	// The only permitted process requests are the documented bounded Git
	// inspections. No real process is started by this audit fixture.
	s.runner = func(_ context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "git" {
			t.Fatalf("read route requested %s %v", exe, args)
		}
		if len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get" && (args[3] == "user.name" || args[3] == "user.email") && dir == identityDir {
			return "", nil
		}
		if len(args) <= len(journeyGitFlags) || !reflect.DeepEqual(args[:len(journeyGitFlags)], journeyGitFlags) || dir != p.Path {
			t.Fatalf("read route requested unsafe Git invocation: %v", args)
		}
		readArgs := args[len(journeyGitFlags):]
		if len(readArgs) > 0 && strings.HasPrefix(readArgs[0], "--git-dir=") {
			bounded, err := project.ManagedGitArguments(s.projectRoot(), p.Path, []string{"status"})
			if err != nil {
				t.Fatal(err)
			}
			expected := bounded[:len(bounded)-1]
			if len(readArgs) < len(expected) || !reflect.DeepEqual(readArgs[:len(expected)], expected) {
				t.Fatalf("read route requested an unowned Git worktree: %v", args)
			}
			readArgs = readArgs[len(expected):]
		}
		read := strings.Join(readArgs, "\x00")
		allowed := map[string]bool{
			"config\x00--null\x00--name-only\x00--get-regexp\x00^filter\\.": true,
			"symbolic-ref\x00--quiet\x00--short\x00HEAD":                    true,
			// These exact argument forms only inspect branch/parent identity,
			// index modes/flags, object format and the existing shared index.
			"symbolic-ref\x00--quiet\x00HEAD":                              true,
			"rev-parse\x00--verify\x00--quiet\x00HEAD^{commit}":            true,
			"rev-parse\x00--show-object-format":                            true,
			"rev-parse\x00--shared-index-path":                             true,
			"status\x00--porcelain=v1\x00-z\x00--untracked-files=all":      true,
			"ls-files\x00--stage\x00-z":                                    true,
			"ls-files\x00-v\x00-z":                                         true,
			"ls-files\x00-z\x00--cached\x00--others\x00--exclude-standard": true,
			"status\x00--porcelain=v1\x00-z\x00--untracked-files=normal":   true,
			"status\x00--porcelain=v1\x00--untracked-files=normal":         true,
			"log\x00-10\x00--format=%H%x00%s%x00":                          true,
			"rev-parse\x00--verify\x00HEAD":                                true,
			"config\x00--get-all\x00remote.origin.url":                     true,
		}
		if !allowed[read] {
			t.Fatalf("read route requested a new Git operation requiring audit: %v", args)
		}
		if read == "rev-parse\x00--show-object-format" {
			return "sha1\n", nil
		}
		return "", nil
	}
	seen := map[string]bool{}
	s.branchRunner = func(_ context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "git" || dir != p.Path || len(args) <= len(journeyGitFlags) || !reflect.DeepEqual(args[:len(journeyGitFlags)], journeyGitFlags) {
			t.Fatalf("branch read requested unsafe invocation: %s %v", exe, args)
		}
		for i, arg := range args {
			switch arg {
			case "config":
				if strings.Join(args[i:], "\x00") != "config\x00--null\x00--name-only\x00--get-regexp\x00^filter\\." {
					t.Fatal("unaudited branch config read", args)
				}
				return "", nil
			case "symbolic-ref":
				return "main", nil
			case "rev-parse", "status", "for-each-ref":
				return "", nil
			case "worktree":
				if strings.Join(args[i:], "\x00") != "worktree\x00list\x00--porcelain\x00-z" {
					t.Fatal("unaudited worktree read", args)
				}
				return "worktree " + p.Path + "\x00\x00", nil
			}
		}
		t.Fatalf("branch read requested an unaudited Git command: %v", args)
		return "", nil
	}
	previewPattern := "GET /v1/product/projects/{id}/releases/{releaseId}/preview/{path...}"
	for _, pattern := range recorded.patterns {
		method, path, hasMethod := strings.Cut(pattern, " ")
		if !hasMethod {
			t.Fatalf("method-agnostic route %q also handles GET; give it an explicit method", pattern)
		}
		if method != "GET" && method != "HEAD" {
			continue
		}
		seen[pattern] = true
		if pattern != previewPattern && productReadOnlyRoutes[pattern] == "" {
			t.Fatalf("unaudited read route %q", pattern)
		}
		path = strings.NewReplacer("{id}", p.ID, "{releaseId}", "missing", "{operationId}", "missing-operation", "{attemptId}", importTestID, "{runId}", "missing-run", "{taskId}", "missing-task", "{path...}", "").Replace(path)
		if strings.HasSuffix(pattern, "/swarm/operations/{operationId}/workspace/file") {
			path += "?path=ordinary.txt"
		}
		for _, readMethod := range []string{"GET", "HEAD"} {
			status := 200
			if pattern == previewPattern {
				// Arbitrary application GETs are NOT classified as read-only.
				// They require the server bridge marker and a separate origin.
				status = 403
			}
			if pattern == "GET /v1/setup/workspace/auth" {
				status = 409
			}
			if strings.HasPrefix(pattern, "GET /v1/setup/workspace/admin/") {
				status = 403 // Standalone fixtures have no trusted management actor.
			}
			if pattern == "GET /v1/product/projects/{id}/checks/{runId}" || pattern == "GET /v1/product/projects/{id}/runs/{runId}" {
				status = 404
			}
			if strings.Contains(pattern, "/swarm/") {
				status = 404
			}
			if pattern == "GET /v1/product/projects/{id}/git/branch-operations/{operationId}" {
				status = 404
			}
			var handler http.Handler = recorded
			if pattern == "GET /v1/product/project-imports/{attemptId}" {
				handler = fixtureReviewHandler(recorded)
			}
			journeyRequest(t, handler, readMethod, path, nil, status)
		}
	}
	for pattern := range productReadOnlyRoutes {
		if !seen[pattern] {
			t.Errorf("stale read-route permission for %q", pattern)
		}
	}
	if !seen[previewPattern] {
		t.Fatal("preview bridge gate was not exercised")
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
		t.Fatal("a read changed the persisted project/state tree")
	}
	if len(s.previews) != 0 || len(s.tunnels) != 0 || len(s.jobs) != 0 || len(s.workspaceInstalls) != 0 || s.runtimeJob.Status != "idle" {
		t.Fatal("a read allocated a preview, tunnel, provisioning job or runtime build")
	}
}

func TestAtomicStateFilesRemainPrivateAcrossReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accepted-connection.json")
	for _, fixture := range []string{"first-private-token", "replacement-private-token"} {
		if err := atomicJSON(path, map[string]string{"token": fixture}); err != nil {
			t.Fatal(err)
		}
		assertPrivateFile(t, path)
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), fixture) {
			t.Fatal("private state replacement lost its payload", err)
		}
	}
}

// The route catalogue above admits no real processes. This separate witness
// exercises Git's actual optional-lock/read behavior with inspectable changes;
// unchanged contents/modes include the existing index and every metadata file.
func TestGitReviewNativeReadsPreserveFilesAndResources(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "native-read-only-review")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Shared context for a reviewed application", "audience": "Teams", "criteria": []string{}}, 200)
	if err := os.WriteFile(filepath.Join(p.Path, "tracked.txt"), []byte("Staged reviewed source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "add", "--", "tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "new.txt"), []byte("Untracked reviewed source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	calls := 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "git" || dir != p.Path {
			t.Fatalf("Git review started an unexpected process: %s %v", exe, args)
		}
		calls++
		return RunCommand(ctx, exe, args, dir)
	}
	for _, method := range []string{"GET", "HEAD"} {
		journeyRequest(t, mux, method, base+"/git", nil, 200)
	}
	if calls == 0 {
		t.Fatal("native Git witness did not execute Git inspection")
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
		t.Fatal("native Git review changed project/index/metadata or private state")
	}
	if len(s.previews) != 0 || len(s.tunnels) != 0 || len(s.jobs) != 0 || len(s.workspaceInstalls) != 0 || s.runtimeJob.Status != "idle" {
		t.Fatal("native Git review allocated resources")
	}
}

// Successful swarm reads need their own native witness: a missing run's 404
// alone cannot prove that inspecting a real candidate or partial allocation is
// free of writes, provider resolution, recovery or resource allocation.
func TestProjectSwarmRegisteredReadsPreserveCandidatesPartialOutputsAndReceipts(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "audit-ready-task-123456789")
	ready := run.Operations[0]
	readyPath, err := s.swarmPath(p.ID, run.ID, ready.ID)
	if err != nil {
		t.Fatal(err)
	}
	swarmCommitFixture(t, readyPath, "alpha.txt", "A committed candidate for read-only review.\n")
	run = swarmFinishFixture(t, s, p, run, ready.ID)
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "audit-partial-task-123456789")
	partial := run.Operations[1]
	partialPath, err := s.swarmPath(p.ID, run.ID, partial.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Model a known incomplete allocation with retained output. Moving its owned
	// fixture metadata preserves every byte while making Git unavailable there.
	if err = os.Rename(filepath.Join(partialPath, ".git"), partialPath+"-retained-metadata"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(partialPath, "partial.txt"), []byte("Retained partial output; do not replay allocation.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		op := swarmOperation(run, partial.ID)
		op.Allocation = "unknown"
		op.Status = "unknown"
		op.Message = "Controlled incomplete allocation for read-only audit."
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeState, beforeProjects := auditTree(t, s.stateDir), auditTree(t, s.projectRoot())
	s.SetProjectRunSelectionResolver(func(context.Context, ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		t.Fatal("read resolved a new provider route")
		return ProjectRunSelection{}, nil
	})
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
		t.Fatal("read reconciled an existing child")
		return nil, nil
	})
	gitReads := 0
	readRunner := func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "git" || (dir != p.Path && dir != readyPath) || len(args) <= len(journeyGitFlags) || !reflect.DeepEqual(args[:len(journeyGitFlags)], journeyGitFlags) {
			t.Fatalf("swarm read requested an unowned process: %s %v in %s", exe, args, dir)
		}
		rest := args[len(journeyGitFlags):]
		for len(rest) > 0 {
			if rest[0] == "-c" && len(rest) >= 2 {
				rest = rest[2:]
				continue
			}
			if strings.HasPrefix(rest[0], "--git-dir=") || strings.HasPrefix(rest[0], "--work-tree=") || rest[0] == "--literal-pathspecs" {
				rest = rest[1:]
				continue
			}
			break
		}
		if len(rest) == 0 {
			t.Fatal("Git read has no subcommand")
		}
		switch rest[0] {
		case "config":
			joined := strings.Join(rest, "\x00")
			if joined != "config\x00--null\x00--name-only\x00--get-regexp\x00^filter\\." && joined != "config\x00--null\x00--list\x00--no-includes" {
				t.Fatal("swarm read requested a configuration mutation", rest)
			}
		case "symbolic-ref":
			if strings.Join(rest, "\x00") != "symbolic-ref\x00--quiet\x00HEAD" && strings.Join(rest, "\x00") != "symbolic-ref\x00--quiet\x00--short\x00HEAD" {
				t.Fatal("swarm read requested a ref mutation", rest)
			}
		case "worktree":
			if strings.Join(rest, "\x00") != "worktree\x00list\x00--porcelain\x00-z" {
				t.Fatal("swarm read requested worktree allocation or repair", rest)
			}
		case "rev-parse", "status", "ls-files", "for-each-ref", "diff", "merge-base":
			for _, arg := range rest[1:] {
				if strings.HasPrefix(arg, "--output") {
					t.Fatal("Git inspection requested an output file", rest)
				}
			}
		default:
			t.Fatal("swarm read requested an unaudited Git subcommand", rest)
		}
		gitReads++
		return RunCommand(ctx, exe, args, dir)
	}
	s.runner, s.branchRunner = readRunner, readRunner
	for _, method := range []string{"GET", "HEAD"} {
		response := journeyRequest(t, mux, method, base+"/tasks/task-a/review", nil, 200)
		var candidate struct {
			Review ProjectSwarmReview `json:"review"`
		}
		decodeEvidenceResponse(t, response.Body.Bytes(), &candidate)
		if !candidate.Review.CanIntegrate || !strings.Contains(candidate.Review.Patch, "committed candidate") {
			t.Fatal("audit did not inspect a real committed candidate", candidate)
		}
		for _, op := range []ProjectRunOperation{ready, partial} {
			response = journeyRequest(t, mux, method, base+"/operations/"+op.ID+"/workspace", nil, 200)
			var workspace struct {
				Workspace ProjectSwarmWorkspace `json:"workspace"`
			}
			decodeEvidenceResponse(t, response.Body.Bytes(), &workspace)
			if !workspace.Workspace.Exists || len(workspace.Workspace.Files) == 0 {
				t.Fatal("audit did not inspect retained output", workspace)
			}
			name := "alpha.txt"
			if op.ID == partial.ID {
				name = "partial.txt"
			}
			response = journeyRequest(t, mux, method, base+"/operations/"+op.ID+"/workspace/file?path="+name, nil, 200)
			var file struct {
				File ProjectSwarmWorkspaceFile `json:"file"`
			}
			decodeEvidenceResponse(t, response.Body.Bytes(), &file)
			if file.File.Path != name || file.File.Content == "" {
				t.Fatal("audit did not read actual retained bytes", response.Body.String())
			}
		}
	}
	if gitReads == 0 {
		t.Fatal("successful swarm read audit never inspected Git")
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProjects, auditTree(t, s.projectRoot())) {
		t.Fatal("swarm GET/HEAD changed a private receipt, retained output or Git metadata")
	}
	loaded, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || loaded.Revision != run.Revision || loaded.Operations[1].Allocation != "unknown" || loaded.Operations[1].Status != "unknown" {
		t.Fatal("inspection repaired or advanced the partial allocation", loaded, err)
	}
	if len(s.previews) != 0 || len(s.tunnels) != 0 || len(s.jobs) != 0 || len(s.workspaceInstalls) != 0 || s.runtimeJob.Status != "idle" {
		t.Fatal("swarm read allocated application resources")
	}
}
