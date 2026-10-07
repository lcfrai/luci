//go:build windows

package management

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
)

func TestUnreconciledLegacyWorkerRemainsReachableBeforeSafeStorageUpgrade(t *testing.T) {
	g, h, ownerToken, _ := fixture(t, nil)
	user := addUser(t, h, ownerToken, "recovering", "member")
	userToken := loginAs(t, h, "recovering")
	worker := filepath.Join(g.dir, "users", user.ID, "runtime")
	legacy := filepath.Join(worker, "state", "conversations")
	migrationWrite(t, legacy, "local/w-fixture/transcript", "preserved legacy conversation")
	migrationWrite(t, worker, "state/sessions/sessions.json", `{"schemaVersion":1,"sessions":[{"status":"running"}]}`)
	response := request(t, h, "GET", "/v1/projects", userToken, nil, 200)
	var proof map[string]string
	if json.Unmarshal(response.Body.Bytes(), &proof) != nil || proof["conversations"] != legacy {
		t.Fatal("unreconciled sessions made the private worker unreachable")
	}
	compact := conversationpath.UserRoot(g.dir, user.ID)
	if _, err := os.Stat(compact); !os.IsNotExist(err) {
		t.Fatal("possibly running CLI homes were copied")
	}
	status := request(t, h, "GET", "/v1/admin/users/"+user.ID+"/runtime", ownerToken, nil, 200)
	if !strings.Contains(status.Body.String(), "existing agent homes") {
		t.Fatal("pending upgrade was not explained")
	}
	request(t, h, "POST", "/v1/admin/users/"+user.ID+"/runtime/stop", ownerToken, map[string]any{}, 200)
	g.mu.Lock()
	rt := g.runtimes[user.ID]
	g.mu.Unlock()
	select {
	case <-rt.done:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy worker did not stop")
	}
	// A backend reconciliation or an explicit session stop owns this state
	// transition. The migration itself never infers it from an old registry.
	migrationWrite(t, worker, "state/sessions/sessions.json", `{"schemaVersion":1,"sessions":[{"status":"exited"}]}`)
	response = request(t, h, "GET", "/v1/projects", userToken, nil, 200)
	if json.Unmarshal(response.Body.Bytes(), &proof) != nil || proof["conversations"] != compact {
		t.Fatal("stopped legacy sessions could not complete the upgrade")
	}
	migrationRead(t, compact, "local/w-fixture/transcript", "preserved legacy conversation")
	migrationRead(t, legacy, "local/w-fixture/transcript", "preserved legacy conversation")
}
