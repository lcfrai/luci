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
	"GET /v1/product/projects/{id}/git/branches":                        "Inspects bounded native refs, HEAD and clean state, then reads session state and private receipts; no checkout, sessions or writes.",
	"GET /v1/product/projects/{id}/git/branch-operations/{operationId}": "Reads the private intent/result and may observe matching native HEAD in memory; never retries checkout or updates the journal.",
	"GET /v1/product/projects/{id}/design-plan":                         "Reads saved screen/state handoff and compares in-memory revisions; no writes or execution.",
	"GET /v1/product/projects/{id}/design-plan/history":                 "Reads immutable saved design handoff snapshots without starting resources.",
	"GET /v1/product/projects/{id}/specification":                       "Reads saved definition or derives a legacy summary in memory; no writes, models or processes.",
	"GET /v1/product/projects/{id}/specification/history":               "Reads immutable saved definition snapshots; no resources or writes.",
	"GET /v1/product/projects/{id}/configuration":                       "Reads only names/revision from private runtime configuration; no values, writes or execution.",
	"GET /v1/setup/status":                                              "Reads local setup or bounded remote setup status; no provisioning.",
	"GET /v1/setup/cloud/jobs":                                          "Reads cached provisioning history.",
	"GET /v1/setup/cloud/login":                                         "Reads cached explicit-login jobs.",
	"GET /v1/setup/targets":                                             "Reads saved connection descriptions without their tokens.",
	"GET /v1/setup/ssh/connections":                                     "Reads already-created tunnels; cannot connect or accept a key.",
	"GET /v1/setup/git/identity":                                        "Two bounded git config --global --get calls from host-owned state.",
	"GET /v1/setup/runtime/install":                                     "Reads cached image-build state; cannot run Docker.",
	"GET /v1/setup/workspace/install":                                   "Reads cached explicit server-install jobs.",
	"GET /v1/setup/workspace/auth":                                      "Reads the selected verified SSH backing identity with bounded GET requests; cannot bootstrap or sign in.",
	"GET /v1/setup/workspace/admin/me":                                  "Reads the active backing identity after local administrator and private SSH checks; cannot sign in.",
	"GET /v1/setup/workspace/admin/users":                               "Reads public backing user records through a bounded authenticated GET; cannot create users.",
	"GET /v1/setup/workspace/admin/invitations":                         "Reads bounded invitation metadata on the selected backing; no code readback, creation or redemption.",
	"GET /v1/setup/workspace/admin/policy":                              "Reads backing policy through a bounded authenticated GET; cannot update it.",
	"GET /v1/setup/workspace/admin/audit":                               "Reads existing backing audit events; cannot execute or provision resources.",
	"GET /v1/setup/workspace/admin/users/{id}/runtime":                  "Reads cached backing runtime state; cannot start or stop a worker.",
	"GET /v1/setup/github/login":                                        "Reads cached explicit GitHub login jobs without running CLI commands.",
	"GET /v1/setup/agents":                                              "Reports presence of the private agent credential without its value.",
	"GET /v1/product/projects/{id}/brief":                               "Reads canonical brief history without creating missing state.",
	"GET /v1/product/projects/{id}/design":                              "Reads canonical design history without creating missing state.",
	"GET /v1/product/projects/{id}/design/export":                       "Renders an inert HTML response from saved design state.",
	"GET /v1/product/projects/{id}/files":                               "Reads contained regular files/directories; rejects credential paths.",
	"GET /v1/product/projects/{id}/git":                                 "Bounded Git inspection with hooks, filters and optional writes disabled.",
	"GET /v1/product/projects/{id}/releases":                            "Reads saved release rows; never builds, deploys or opens a listener.",
	"GET /v1/product/projects/{id}/plan":                                "Reads canonical task progress; cannot start checks or complete tasks.",
	"GET /v1/product/projects/{id}/checks":                              "Reads saved bounded execution evidence and suggested command names; no commands execute.",
	"GET /v1/product/projects/{id}/checks/{runId}":                      "Reads saved execution evidence without starting a process.",
	"GET /v1/product/projects/{id}/git/delivery":                        "Inspects current Git branch/commit and saved publication evidence; cannot create or push a repository.",
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
		if len(readArgs) >= 4 && strings.HasPrefix(readArgs[0], "--git-dir=") {
			expected := []string{"--git-dir=" + filepath.Join(p.Path, ".git"), "--work-tree=" + p.Path, "-c", "core.bare=false"}
			if !reflect.DeepEqual(readArgs[:4], expected) {
				t.Fatalf("read route requested an unowned Git worktree: %v", args)
			}
			readArgs = readArgs[4:]
		}
		read := strings.Join(readArgs, "\x00")
		allowed := map[string]bool{
			"config\x00--null\x00--name-only\x00--get-regexp\x00^filter\\.": true,
			"symbolic-ref\x00--quiet\x00--short\x00HEAD":                    true,
			"status\x00--porcelain=v1\x00-z\x00--untracked-files=normal":    true,
			"log\x00-10\x00--format=%H%x00%s%x00":                           true,
			"rev-parse\x00--verify\x00HEAD":                                 true,
			"config\x00--get-all\x00remote.origin.url":                      true,
		}
		if !allowed[read] {
			t.Fatalf("read route requested a new Git operation requiring audit: %v", args)
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
		path = strings.NewReplacer("{id}", p.ID, "{releaseId}", "missing", "{operationId}", "missing-operation", "{path...}", "").Replace(path)
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
			if pattern == "GET /v1/product/projects/{id}/checks/{runId}" {
				status = 404
			}
			if pattern == "GET /v1/product/projects/{id}/git/branch-operations/{operationId}" {
				status = 404
			}
			journeyRequest(t, recorded, readMethod, path, nil, status)
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
