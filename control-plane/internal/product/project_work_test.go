package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectPlanExportsContextAndKeepsRevision(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "plan")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Create a working solution", "criteria": []string{"The result runs"}}, 200)
	req := map[string]any{"expectedRevision": 0, "tasks": []ProjectTask{{"task-1", "Implement the workflow", "doing"}}}
	journeyRequest(t, mux, "POST", base+"/plan", req, 200)
	journeyRequest(t, mux, "POST", base+"/plan", req, 409)
	state, err := readJourney(p.Path, p.ID)
	if err != nil || state.Plan == nil || state.Plan.BriefRevision != 1 {
		t.Fatal(state, err)
	}
	for _, file := range []string{".luci/plan.json", ".luci/design.json"} {
		data, err := os.ReadFile(filepath.Join(p.Path, filepath.FromSlash(file)))
		if err != nil || !json.Valid(data) {
			t.Fatal(file, err)
		}
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "Save brief and plan"}, 200)
	reloaded, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	other := http.NewServeMux()
	reloaded.registerProjects(other)
	reloaded.registerProjectWork(other)
	row := journeyRequest(t, other, "GET", base+"/plan", nil, 200)
	if !strings.Contains(row.Body.String(), "Implement the workflow") {
		t.Fatal(row.Body.String())
	}
}

func TestProjectChecksRunActualProcessAndPersistEvidence(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is required for real command outcome")
	}
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "checks")
	base := "/v1/product/projects/" + p.ID + "/checks"
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`console.log('verified outcome');process.exit(0)`), 0600); err != nil {
		t.Fatal(err)
	}
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "docker" {
			return original(ctx, name, args, dir)
		}
		if len(args) > 0 && args[0] == "rm" {
			return "", nil
		}
		for i, a := range args {
			if a == "--entrypoint" {
				if args[i+1] != "node" {
					return "", errors.New("unexpected check executable")
				}
				return RunCommand(ctx, node, args[i+3:], dir)
			}
		}
		return "", errors.New("Docker command did not contain an entrypoint")
	}
	req := map[string]any{"commands": []CheckCommand{{"Actual process", "node", []string{"check.js"}}}, "acknowledgeHostExecution": true}
	w := journeyRequest(t, mux, "POST", base, req, 202)
	var row ProjectCheckRun
	json.Unmarshal(w.Body.Bytes(), &row)
	row = waitProjectCheck(t, s, p.ID, row.ID)
	if row.Status != "passed" || row.SourceChanged || row.SourceDigest == "" || row.Commands[0].ExitCode == nil || *row.Commands[0].ExitCode != 0 || !strings.Contains(row.Commands[0].Output, "verified outcome") {
		t.Fatal(row)
	}
	s.mu.RLock()
	history, err := s.readCheckHistory(p.ID)
	s.mu.RUnlock()
	if err != nil || len(history) != 1 || history[0].Status != "passed" {
		t.Fatal(history, err)
	}
	// Changed source with a successful exit must not claim checks pass on final source.
	os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`require('fs').writeFileSync('created.txt','new source');console.log('changed')`), 0600)
	w = journeyRequest(t, mux, "POST", base, req, 202)
	json.Unmarshal(w.Body.Bytes(), &row)
	row = waitProjectCheck(t, s, p.ID, row.ID)
	if row.Status != "unknown" || !row.SourceChanged {
		t.Fatal(row)
	}
}

func TestProjectChecksCancelAndRefuseUnmanagedHost(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "cancel")
	base := "/v1/product/projects/" + p.ID + "/checks"
	req := map[string]any{"commands": []CheckCommand{{"Wait", "node", []string{"check.js"}}}, "acknowledgeHostExecution": true, "execution": "host"}
	journeyRequest(t, mux, "POST", base, req, 403)
	req["execution"] = "docker"
	req["acknowledgeHostExecution"] = false
	journeyRequest(t, mux, "POST", base, req, 400)
	req["acknowledgeHostExecution"] = true
	started := make(chan struct{})
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "docker" {
			return original(ctx, name, args, dir)
		}
		if args[0] == "rm" {
			return "", nil
		}
		close(started)
		<-ctx.Done()
		return "cancelled", ctx.Err()
	}
	w := journeyRequest(t, mux, "POST", base, req, 202)
	var row ProjectCheckRun
	json.Unmarshal(w.Body.Bytes(), &row)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("check did not start")
	}
	journeyRequest(t, mux, "POST", base, req, 409)
	journeyRequest(t, mux, "POST", base+"/"+row.ID+"/cancel", map[string]any{}, 202)
	row = waitProjectCheck(t, s, p.ID, row.ID)
	if row.Status != "cancelled" {
		t.Fatal(row)
	}
}

func waitProjectCheck(t *testing.T, s *Server, projectID, id string) ProjectCheckRun {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		rows, err := s.readCheckHistory(projectID)
		s.mu.RUnlock()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ID == id && row.Status != "queued" && row.Status != "running" {
				return row
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("check did not finish")
	return ProjectCheckRun{}
}
