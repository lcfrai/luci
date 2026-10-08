package product

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// CLOUD-001 reproduces the installed multi-project failure through the full
// production registrar, not the bare scheduler mux used by store fixtures.
func TestSwarmQueueRemainsAvailableWhileSourceAdmissionIsExclusive(t *testing.T) {
	s, fixtureMux, run, base, _, starts, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, fixtureMux, run, base, []string{"task-a", "task-b", "task-c"})
	mux := http.NewServeMux()
	s.Register(mux)
	queuePath := "/v1/product/projects/" + run.ProjectID + "/swarm-queue"
	projectPath := "/v1/product/projects/" + run.ProjectID
	guard := s.branchGuard(run.ProjectID)
	guard.Lock() // The same exclusive lock held across isolated allocation.
	defer guard.Unlock()
	privateBefore := auditTree(t, s.stateDir)
	sourceBefore := auditTree(t, s.projectRoot())
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := journeyRequest(t, mux, method, queuePath, nil, http.StatusOK)
		if method == http.MethodGet {
			var view ProjectSwarmQueue
			decodeEvidenceResponse(t, w.Body.Bytes(), &view)
			if view.Counts.Queued != 3 || len(view.Items) != 3 || view.Items[0].ProjectID != run.ProjectID {
				t.Fatal("queue did not retain actual project progress", w.Body.String())
			}
		}
	}
	if !reflect.DeepEqual(privateBefore, auditTree(t, s.stateDir)) || !reflect.DeepEqual(sourceBefore, auditTree(t, s.projectRoot())) {
		t.Fatal("queue read repaired receipts or touched source")
	}
	item := items[0]
	for _, action := range []string{"priority", "pause", "resume", "cancel"} {
		beforeRevision := item.Revision
		var response struct {
			Item SwarmQueueItem `json:"item"`
		}
		w := journeyRequest(t, mux, http.MethodPost, queuePath+"/"+item.ID+"/"+action, map[string]any{"expectedRevision": item.Revision, "priority": "high"}, http.StatusOK)
		decodeEvidenceResponse(t, w.Body.Bytes(), &response)
		item = response.Item
		if item.ID != items[0].ID || item.Revision != beforeRevision+1 || item.Priority != "high" {
			t.Fatal("control lost exact item/CAS identity", w.Body.String())
		}
		wantState := map[string]string{"priority": "queued", "pause": "paused", "resume": "queued", "cancel": "cancelled"}[action]
		if item.State != wantState {
			t.Fatalf("%s left state %s instead of %s", action, item.State, wantState)
		}
		beforeStale := auditTree(t, s.stateDir)
		journeyRequest(t, mux, http.MethodPost, queuePath+"/"+item.ID+"/"+action, map[string]any{"expectedRevision": beforeRevision, "priority": "low"}, http.StatusConflict)
		if !reflect.DeepEqual(beforeStale, auditTree(t, s.stateDir)) {
			t.Fatal("stale queue control changed private receipts")
		}
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, base + "/queue"},
		{http.MethodPost, base + "/tasks/task-b/queue"},
		{http.MethodGet, projectPath + "/brief"},
		{http.MethodGet, projectPath + "/files?path=README.md"},
		{http.MethodGet, projectPath + "/runs/" + run.ID},
		{http.MethodGet, projectPath + "/overview"},
		{http.MethodGet, base + "/tasks/task-a/review"},
		{http.MethodPost, base + "/reconcile"},
	} {
		w := journeyRequest(t, mux, route.method, route.path, nil, http.StatusConflict)
		if !strings.Contains(w.Body.String(), `"branch_busy"`) {
			t.Fatalf("source-dependent route did not retain admission: %s %s: %s", route.method, route.path, w.Body.String())
		}
	}
	if *starts != 0 || !reflect.DeepEqual(sourceBefore, auditTree(t, s.projectRoot())) {
		t.Fatal("private queue visibility/control launched work or changed source")
	}
	if _, err := os.Stat(filepath.Join(s.projectRoot(), ".luci-swarms")); !os.IsNotExist(err) {
		t.Fatal("private controls allocated a worker checkout", err)
	}
}
