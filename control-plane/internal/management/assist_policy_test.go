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
		request(t, handler, "POST", path, token, map[string]any{"harness": "codex"}, 403)
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

func TestRepositoryAnalysisUsesChosenHarnessQuotaAndReadOnlyListingIsNotAdmission(t *testing.T) {
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
	path := "/v1/product/projects/p-test/assist/repository"
	request(t, handler, "PATCH", "/v1/admin/policy", token, map[string]any{"allowedHarnesses": []string{"opencode"}}, 200)
	for _, chosen := range []string{"codex", "claude-code"} {
		request(t, handler, "POST", path, token, map[string]any{"harness": chosen}, 403)
	}
	for _, chosen := range []string{"", "shell", "invented"} {
		request(t, handler, "POST", path, token, map[string]any{"harness": chosen}, 400)
	}
	active.Store(true)
	request(t, handler, "POST", path, token, map[string]any{"harness": "opencode"}, 409)
	request(t, handler, "GET", "/v1/product/projects/p-test/assist", token, nil, 200)
	request(t, handler, "POST", "/v1/product/projects/p-test/assist/assist-test/apply", token, map[string]bool{"acknowledgePartial": true}, 200)
	active.Store(false)
	request(t, handler, "POST", path, token, map[string]any{"harness": "opencode"}, 200)
	if executions.Load() != 3 {
		t.Fatal("disabled/invalid/over-quota inference reached the private worker")
	}
}

func TestInterviewUsesActualSelectedHarnessAndRejectsInvalidBodiesBeforeExecution(t *testing.T) {
	var active atomic.Bool
	var executed atomic.Int32
	engine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/sessions" {
			status := "exited"
			if active.Load() {
				status = "running"
			}
			jsonReply(w, 200, map[string]any{"sessions": []map[string]string{{"id": "owned", "harness": "codex", "status": status}}})
			return
		}
		executed.Add(1)
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, handler, token, _ := fixture(t, engine)
	path := "/v1/product/projects/p-owned/assist"
	request(t, handler, "PATCH", "/v1/admin/policy", token, map[string]any{"allowedHarnesses": []string{"claude-code"}}, 200)
	for _, chosen := range []string{"codex", "opencode"} {
		request(t, handler, "POST", path, token, map[string]any{"harness": chosen}, 403)
	}
	for _, chosen := range []string{"shell", "invented"} {
		request(t, handler, "POST", path, token, map[string]any{"harness": chosen}, 400)
	}
	request(t, handler, "POST", path, token, nil, 400)
	active.Store(true)
	request(t, handler, "POST", path, token, map[string]any{"harness": "claude-code"}, 409)
	active.Store(false)
	request(t, handler, "POST", path, token, map[string]any{"harness": "claude-code"}, 200)
	if executed.Load() != 1 {
		t.Fatal("disabled, invalid or over-quota interview reached the worker")
	}
}
