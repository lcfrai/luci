package management

import (
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedRecoveryKeepsOldParticipantsUntilExactReconciliation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leases.json")
	write := func(leases []any) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"version": 1, "leases": leases})
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	unrelated := map[string]any{"id": "byo", "scope": strings.Repeat("b", 64)}
	write([]any{unrelated})
	if e := unresolvedManagedModelLeases(path); e != nil {
		t.Fatal("clean previous close blocked restart", e)
	}
	binding := managedmodel.Binding{Boot: strings.Repeat("a", 64), Generation: "7", Revision: 2, RuntimeScope: strings.Repeat("c", 64), LeaseID: "manual-owned", Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}
	previous := map[string]any{"id": binding.LeaseID, "scope": binding.RuntimeScope, "managedLocalModel": binding}
	write([]any{unrelated, previous})
	if e := unresolvedManagedModelLeases(path); e == nil {
		t.Fatal("abrupt old allocation allowed a replacement model")
	}
	// The owning API has positively reconciled that exact allocation and removed
	// only its lease. Manager never erases another runtime's ownership itself.
	write([]any{unrelated})
	if e := unresolvedManagedModelLeases(path); e != nil {
		t.Fatal("positive reconciliation failed to unblock a new generation", e)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "byo") {
		t.Fatal("unrelated BYO work was changed")
	}
}
