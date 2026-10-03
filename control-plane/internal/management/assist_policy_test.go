package management

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestAccountSignInAndDefineAssistantUseCodexPolicyAndSessionQuota(t *testing.T) {
	var active atomic.Bool
	var executions atomic.Int32
	engine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/sessions" {
			status := "exited"
			if active.Load() {
				status = "running"
			}
			jsonReply(w, 200, map[string]any{"sessions": []map[string]string{{"id": "existing", "harness": "codex", "status": status}}})
			return
		}
		executions.Add(1)
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, handler, token, _ := fixture(t, engine)
	paths := []string{"/v1/setup/agents/codex/sign-in", "/v1/product/projects/p-test/assist"}
	request(t, handler, "PATCH", "/v1/admin/policy", token, map[string]any{"allowedHarnesses": []string{"shell"}}, 200)
	for _, path := range paths {
		request(t, handler, "POST", path, token, map[string]any{"harness": "shell"}, 403)
	}
	request(t, handler, "PATCH", "/v1/admin/policy", token, map[string]any{"allowedHarnesses": []string{"codex"}}, 200)
	active.Store(true)
	for _, path := range paths {
		request(t, handler, "POST", path, token, map[string]any{}, 409)
	}
	active.Store(false)
	for _, path := range paths {
		request(t, handler, "POST", path, token, map[string]any{}, 200)
	}
	if executions.Load() != 2 {
		t.Fatal("disabled or over-quota utility execution reached the user engine")
	}
}
