package product

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func retainedWorkspaceFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, ProjectRun, ProjectRunOperation, string, string) {
	t.Helper()
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "retained-workspace-123456789")
	op := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, mux, p, run, op, path, base + "/operations/" + op.ID + "/workspace"
}

func retainedWorkspaceView(t *testing.T, mux *http.ServeMux, endpoint string) ProjectSwarmWorkspace {
	t.Helper()
	var response struct {
		Workspace ProjectSwarmWorkspace `json:"workspace"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", endpoint, nil, 200).Body.Bytes(), &response)
	return response.Workspace
}

func retainedWorkspaceFile(t *testing.T, mux *http.ServeMux, endpoint, path string) ProjectSwarmWorkspaceFile {
	t.Helper()
	var response struct {
		File ProjectSwarmWorkspaceFile `json:"file"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", endpoint+"/file?path="+url.QueryEscape(path), nil, 200).Body.Bytes(), &response)
	return response.File
}

func TestProjectSwarmWorkspaceRetainsCommittedDirtyAndUntrackedOutputWithoutSession(t *testing.T) {
	s, mux, p, run, op, path, endpoint := retainedWorkspaceFixture(t)
	swarmCommitFixture(t, path, "alpha.txt", "committed agent output\n")
	for name, value := range map[string]string{"alpha.txt": "unfinished changed output\n", "untracked.txt": "saved before failure\n", ".env": "PRIVATE_TOKEN_SHOULD_NOT_APPEAR", "credentials.json": "PRIVATE_TOKEN_SHOULD_NOT_APPEAR"} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Retirement preserves operation ownership while allowing a future fresh
	// task attempt. No registry session is constructed for this fixture.
	run, err := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		operation := swarmOperation(run, op.ID)
		operation.Allocation, operation.Status = "abandoned", "cancelled"
		swarmTask(run, op.TaskID).OperationID = ""
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(s.stateDir, "project-runs", p.ID, run.ID+".json")
	before, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	view := retainedWorkspaceView(t, mux, endpoint)
	if !view.Exists || view.OperationID != op.ID || view.WorktreeID != op.WorktreeID || view.TaskID != op.TaskID || view.FilesTruncated || !strings.Contains(view.Patch, "+unfinished changed output") {
		t.Fatalf("retained task result not readable: %#v", view)
	}
	files := map[string]string{}
	for _, item := range view.Files {
		files[item.Path] = item.Status
	}
	if files["alpha.txt"] != " M" || files["untracked.txt"] != "??" {
		t.Fatalf("dirty or untracked output absent: %#v", files)
	}
	if _, exists := files[".env"]; exists {
		t.Fatal("protected file exposed")
	}
	file := retainedWorkspaceFile(t, mux, endpoint, "untracked.txt")
	if file.Content != "saved before failure\n" || file.Omitted || file.Binary || file.OperationID != op.ID || file.WorktreeID != op.WorktreeID {
		t.Fatalf("retained untracked text missing: %#v", file)
	}
	data, _ := json.Marshal(view)
	for _, secret := range []string{path, s.stateDir, "PRIVATE_TOKEN_SHOULD_NOT_APPEAR", ".luci/state.json", ".git/config"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("inspection leaked protected data %q", secret)
		}
	}
	after, _ := os.ReadFile(receipt)
	if string(before) != string(after) {
		t.Fatal("read changed private reservation history")
	}
	if _, err := os.Stat(filepath.Join(p.Path, "alpha.txt")); !os.IsNotExist(err) {
		t.Fatal("inspection integrated task output into main", err)
	}
}

func TestProjectSwarmWorkspaceMissingAndPartialAllocationAreReadOnly(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run, op, _, err := s.projectRuns.ReserveSwarmTask(p.ID, p.Path, run.ID, run.Revision, "partial-allocation-123456789", "task-a", run.Input.Implementation.SourceCommit, func(id string) string { path, _ := s.swarmPath(p.ID, run.ID, id); return project.WorktreeID(path) })
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := base + "/operations/" + op.ID + "/workspace"
	view := retainedWorkspaceView(t, mux, endpoint)
	if view.Exists || len(view.Files) != 0 || view.Patch != "" {
		t.Fatalf("missing allocation became output: %#v", view)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("inspection created allocation directories", err)
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "recovery.txt"), []byte("partial allocation output"), 0600); err != nil {
		t.Fatal(err)
	}
	view = retainedWorkspaceView(t, mux, endpoint)
	if !view.Exists || !reflect.DeepEqual(view.Files, []GitChange{{Path: "recovery.txt", Status: "saved"}}) || !strings.Contains(view.Reason, "No safe complete Git metadata") {
		t.Fatalf("partial workspace hidden or repaired: %#v", view)
	}
	if file := retainedWorkspaceFile(t, mux, endpoint, "recovery.txt"); file.Content != "partial allocation output" {
		t.Fatal(file)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
		t.Fatal("inspection initialized Git", err)
	}
	loaded, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || loaded.Revision != run.Revision || loaded.Operations[0].Allocation != "allocating" {
		t.Fatal("read repaired a reservation", loaded, err)
	}
}

func TestProjectSwarmWorkspaceRejectsProtectedAmbiguousAndForeignPaths(t *testing.T) {
	_, mux, p, run, _, path, endpoint := retainedWorkspaceFixture(t)
	if err := os.Mkdir(filepath.Join(path, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"../outside", "src/../alpha.txt", "/etc/passwd", "C:/secrets", "folder\\secret", "folder//file", "folder/file.", "folder/file ", "CON", "COM¹.txt", "file:stream", ".git/config", ".luci/state.json", ".gıt/config", ".lucı/state.json", ".env", "nested/.env.local", "credentials.json", "credentıals.json", "key.pem", ".ssh/id_rsa", "secrets.json", "CREDEN~1.JSO", "ENVLOC~1", "LUCIST~1/config"} {
		t.Run(relative, func(t *testing.T) {
			journeyRequest(t, mux, "GET", endpoint+"/file?path="+url.QueryEscape(relative), nil, 403)
		})
	}
	for _, query := range []string{"", "?path=a&path=b", "?path=a&other=b", "?bad=a", "?path=%zz"} {
		journeyRequest(t, mux, "GET", endpoint+"/file"+query, nil, 400)
	}
	journeyRequest(t, mux, "GET", endpoint+"?path=alpha.txt", nil, 400)
	journeyRequest(t, mux, "GET", endpoint+"/file?path=directory", nil, 404)
	journeyRequest(t, mux, "GET", endpoint+"/file?path=missing.txt", nil, 404)
	other := createJourneyProject(t, mux, "foreign-retained-output")
	foreign := strings.Replace(endpoint, "/projects/"+p.ID+"/", "/projects/"+other.ID+"/", 1)
	journeyRequest(t, mux, "GET", foreign, nil, 404)
	foreign = strings.Replace(endpoint, run.ID, "run-"+strings.Repeat("d", 24), 1)
	journeyRequest(t, mux, "GET", foreign, nil, 404)
}

func TestProjectSwarmWorkspaceSkipsFileDirectoryAndHardLinks(t *testing.T) {
	_, mux, _, _, _, path, endpoint := retainedWorkspaceFixture(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside-only-value"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, target string
		hard         bool
	}{{"file-link.txt", secret, false}, {"directory-link", outside, false}, {"hard-link.txt", secret, true}} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.hard {
				err = os.Link(tc.target, filepath.Join(path, tc.name))
			} else {
				err = os.Symlink(tc.target, filepath.Join(path, tc.name))
			}
			if err != nil {
				t.Skipf("filesystem link creation unavailable: %v", err)
			}
			view := retainedWorkspaceView(t, mux, endpoint)
			for _, file := range view.Files {
				if file.Path == tc.name || strings.HasPrefix(file.Path, tc.name+"/") {
					t.Fatal("linked output exposed", file)
				}
			}
			requested := tc.name
			if tc.name == "directory-link" {
				requested += "/secret.txt"
			}
			journeyRequest(t, mux, "GET", endpoint+"/file?path="+url.QueryEscape(requested), nil, 404)
			data, _ := json.Marshal(view)
			if strings.Contains(string(data), "outside-only-value") || strings.Contains(string(data), outside) {
				t.Fatal("link target leaked")
			}
		})
	}
}

func TestProjectSwarmWorkspaceTextBoundsBinaryAndIncompleteInventoryAreExplicit(t *testing.T) {
	_, mux, _, _, _, path, endpoint := retainedWorkspaceFixture(t)
	value := strings.Repeat("x", swarmWorkspaceFileLimit-1) + "€tail"
	if err := os.WriteFile(filepath.Join(path, "large.txt"), []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	file := retainedWorkspaceFile(t, mux, endpoint, "large.txt")
	if file.Binary || !file.Omitted || len(file.Content) != swarmWorkspaceFileLimit-1 || file.Bytes != int64(len(value)) {
		t.Fatalf("UTF-8 boundary was corrupted or truncation hidden: %#v", file)
	}
	swarmCommitFixture(t, path, "large.txt", value)
	preview := retainedWorkspaceView(t, mux, endpoint)
	if !preview.PatchTruncated || len(preview.Patch) > swarmWorkspaceFileLimit || !strings.Contains(preview.Reason, "diff is incomplete") {
		t.Fatal("large tracked patch was not bounded explicitly", len(preview.Patch), preview.PatchTruncated, preview.Reason)
	}
	if err := os.WriteFile(filepath.Join(path, "binary.bin"), []byte{0, 1, 2, 0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	file = retainedWorkspaceFile(t, mux, endpoint, "binary.bin")
	if !file.Binary || !file.Omitted || file.Content != "" || file.Bytes != 4 {
		t.Fatal("binary data presented as text", file)
	}
	for i := 0; i < swarmWorkspaceListLimit+1; i++ {
		if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("output-%04d.txt", i)), []byte("retained"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	view := retainedWorkspaceView(t, mux, endpoint)
	if len(view.Files) != swarmWorkspaceListLimit || !view.FilesTruncated || view.FilesOmitted < 1 || !strings.Contains(view.Reason, "inventory is incomplete") {
		t.Fatal("inventory silently dropped output", len(view.Files), view.FilesTruncated, view.FilesOmitted)
	}
	if file := retainedWorkspaceFile(t, mux, endpoint, "output-2000.txt"); file.Content != "retained" {
		t.Fatal("known file beyond listing cap could not be read", file)
	}
}

func TestProjectSwarmWorkspaceReplacingOwnedRootDoesNotFollowLink(t *testing.T) {
	_, mux, _, _, _, path, endpoint := retainedWorkspaceFixture(t)
	retained := path + "-retained"
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	journeyRequest(t, mux, "GET", endpoint, nil, 409)
	journeyRequest(t, mux, "GET", endpoint+"/file?path=secret.txt", nil, 409)
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused inspection wrote to redirected workspace", entries, err)
	}
}

func TestProjectSwarmWorkspacePrioritizesAgentOutputBeforeLargeIgnoredDependencies(t *testing.T) {
	_, mux, _, _, _, path, endpoint := retainedWorkspaceFixture(t)
	swarmCommitFixture(t, path, ".gitignore", "node_modules/\n")
	dependencies := filepath.Join(path, "node_modules")
	if err := os.Mkdir(dependencies, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < swarmWorkspaceListLimit+1; i++ {
		if err := os.WriteFile(filepath.Join(dependencies, fmt.Sprintf("dependency-%04d.js", i)), []byte("ignored package bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	swarmCommitFixture(t, path, "z-produced.txt", "committed agent result\n")
	if err := os.WriteFile(filepath.Join(path, "z-untracked.txt"), []byte("unfinished agent result\n"), 0600); err != nil {
		t.Fatal(err)
	}
	view := retainedWorkspaceView(t, mux, endpoint)
	for _, want := range []string{"z-produced.txt", "z-untracked.txt"} {
		found := false
		for i, file := range view.Files {
			if file.Path == want {
				found = i < 10
			}
		}
		if !found {
			t.Fatalf("%s was hidden behind ignored dependency inventory", want)
		}
	}
	if !view.FilesTruncated || !strings.Contains(view.Patch, "+committed agent result") {
		t.Fatal("bounded inventory hid committed output or omitted truncation", view.FilesTruncated, view.Patch)
	}
}
