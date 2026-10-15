package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestReviewedOpenCodeNativeFinishedTurnWhileProcessLives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/native-turn" || r.Header.Get("Authorization") != "Bearer exact-owned-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"state":"finished","toolClass":"reads-only","turnId":"msg_67d5b111"}`))
	}))
	defer server.Close()
	row := sessionsFullRecord()
	row.Harness = "opencode"
	row.ImplementationContext = &registry.ImplementationContext{ProjectID: row.ProjectID, WorktreeID: row.WorktreeID, TaskIDs: []string{"orchard-task"}}
	row.Status = registry.StatusIdle // The TUI is still alive after assistant finish=stop.
	row.Backend, row.Endpoint, row.AttachToken = "docker", strings.TrimPrefix(server.URL, "http://"), "exact-owned-token"
	view := (&API{}).sessionJSON(row, sessionBasePath)
	if view.ImplementationOutcome == nil || view.ImplementationOutcome.State != "quiet" || view.ImplementationOutcome.NativeTurnID != "msg_67d5b111" || view.ImplementationOutcome.NativeToolClass != "reads-only" {
		t.Fatalf("live native turn = %+v", view.ImplementationOutcome)
	}
	row.AttachToken = "revoked-or-other-sandbox-token"
	if view := (&API{}).sessionJSON(row, sessionBasePath); view.ImplementationOutcome.NativeTurnID != "" {
		t.Fatalf("wrong allocation token attached stale native turn: %+v", view.ImplementationOutcome)
	}
	row.AttachToken = "exact-owned-token"
	row.Harness = "codex"
	if view := (&API{}).sessionJSON(row, sessionBasePath); view.ImplementationOutcome.NativeTurnID != "" {
		t.Fatalf("other harness attached OpenCode observation: %+v", view.ImplementationOutcome)
	}
}
