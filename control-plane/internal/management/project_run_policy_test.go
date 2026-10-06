package management

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestProjectRunReplayAtSessionQuotaRequiresExactWorkerOwnedPair(t *testing.T) {
	var executions atomic.Int32
	var duplicate atomic.Bool
	engine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/sessions" {
			rows := []map[string]string{{"id": "s-owned", "harness": "codex", "status": "running", "projectRunId": "run-owned", "projectRunOperationId": "op-owned"}}
			if duplicate.Load() {
				rows = append(rows, map[string]string{"id": "s-conflict", "harness": "codex", "status": "running", "projectRunId": "run-owned", "projectRunOperationId": "op-owned"})
			}
			jsonReply(w, 200, map[string]any{"sessions": rows})
			return
		}
		executions.Add(1)
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, handler, token, _ := fixture(t, engine)
	for _, pair := range []map[string]any{
		{"harness": "codex"},
		{"harness": "codex", "projectRunId": "run-owned"},
		{"harness": "codex", "projectRunId": "run-other", "projectRunOperationId": "op-owned"},
		{"harness": "codex", "projectRunId": "run-owned", "projectRunOperationId": "op-other"},
	} {
		request(t, handler, "POST", "/v1/sessions", token, pair, 409)
	}
	replay := map[string]any{"harness": "codex", "projectRunId": "run-owned", "projectRunOperationId": "op-owned"}
	request(t, handler, "POST", "/v1/sessions", token, replay, 200)
	duplicate.Store(true)
	request(t, handler, "POST", "/v1/sessions", token, replay, 409)
	duplicate.Store(false)
	request(t, handler, "PATCH", "/v1/admin/policy", token, map[string]any{"allowedHarnesses": []string{"shell"}}, 200)
	request(t, handler, "POST", "/v1/sessions", token, replay, 403)
	if executions.Load() != 1 {
		t.Fatal("partial, foreign, conflicting or disabled replay reached the worker")
	}
}
