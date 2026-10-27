package product

import (
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedConnectionReferenceSelectionAndStoppedRenameStayFenced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	boot := strings.Repeat("a", 64)
	if e := managedmodel.Configure(path, boot); e != nil {
		t.Fatal(e)
	}
	defer managedmodel.ReleaseAuthority(path, boot)
	observation := managedmodel.Observation{Version: 1, Boot: boot, Revision: 2, Status: managedmodel.Status{State: "available", Generation: "7", ObservedAt: time.Now().UTC(), Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}}
	write := func() {
		t.Helper()
		raw, _ := json.Marshal(observation)
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	write()
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	body := map[string]any{"action": "save", "expectedRevision": 0, "name": "Managed local", "kind": "compatible-api", "enabled": true, "endpoints": AgentConnectionEndpoints{ChatCompletions: observation.Status.Endpoint}, "models": AgentConnectionModels{OpenCode: observation.Status.Model}, "openCodeService": "local-ollama", "openCodeThinking": "off", "managedLocalModel": managedmodel.Reference{Revision: 2}, "apiKey": ""}
	catalogue := connectionSave(t, mux, body)
	managedID := catalogue.Connections[0].ID
	catalogue = connectionSave(t, mux, connectionSaveBody(catalogue.Revision))
	byoID := catalogue.Connections[1].ID
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", map[string]any{"action": "select", "expectedRevision": catalogue.Revision, "id": byoID}, 200)
	catalogue = connectionRead(t, mux)
	observation.Status.State = "stopped"
	write()
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", map[string]any{"action": "select", "expectedRevision": catalogue.Revision, "id": managedID}, 409)
	if connectionRead(t, mux).ActiveConnectionID != byoID {
		t.Fatal("unavailable managed selection replaced existing BYO")
	}
	delete(body, "managedLocalModel")
	body["id"] = managedID
	body["expectedRevision"] = catalogue.Revision
	body["name"] = "Renamed while stopped"
	catalogue = connectionSave(t, mux, body)
	if catalogue.Connections[0].ManagedLocalModel == nil || catalogue.Connections[0].ManagedLocalModel.Revision != 2 || catalogue.ActiveConnectionID != byoID {
		t.Fatal("metadata rename detached managed authority")
	}
	body["expectedRevision"] = catalogue.Revision
	body["managedLocalModel"] = nil
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", body, 409)
	observation.Status.State = "available"
	observation.Status.ObservedAt = time.Now().UTC()
	write()
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", map[string]any{"action": "select", "expectedRevision": catalogue.Revision, "id": managedID}, 200)
	receipt, _, e := s.ResolveAgentConnection("opencode", &managedID)
	if e != nil || receipt.ManagedLocalModel == nil || receipt.ManagedLocalModel.Revision != 2 {
		t.Fatal("selected managed runtime lost exact reference", e)
	}
	observation.Revision++
	write()
	if _, _, e = s.ResolveAgentConnection("opencode", &managedID); e == nil {
		t.Fatal("changed managed config silently accepted")
	}
}
func TestManagedConnectionValidationAndReceiptEqualityPreserveExplicitProfile(t *testing.T) {
	valid := privateAgentConnection{ID: "agent-" + strings.Repeat("a", 24), Name: "Managed", Kind: "compatible-api", Enabled: true, Revision: 1, Endpoints: AgentConnectionEndpoints{ChatCompletions: "http://127.0.0.1:24567/v1"}, Models: AgentConnectionModels{OpenCode: "installed:latest"}, OpenCodeService: AgentConnectionOpenCodeLocalOllama, OpenCodeThinking: AgentConnectionOpenCodeThinkingOff, ManagedLocalModel: &managedmodel.Reference{Revision: 2}}
	if !validateAgentConnection(valid) {
		t.Fatal("typed managed profile refused")
	}
	for _, change := range []func(*privateAgentConnection){func(c *privateAgentConnection) { c.OpenCodeThinking = "low" }, func(c *privateAgentConnection) { c.OpenCodeService = AgentConnectionOpenCodeCompatibleAPI }, func(c *privateAgentConnection) { c.APIKey = "key" }, func(c *privateAgentConnection) { c.Models.Codex = "other" }} {
		c := valid
		change(&c)
		if validateAgentConnection(c) {
			t.Fatal("managed profile lost a required fence")
		}
	}
	a := AgentConnectionReceipt{ID: valid.ID, Revision: 1, ManagedLocalModel: &managedmodel.Reference{Revision: 2}}
	b := a
	b.ManagedLocalModel = &managedmodel.Reference{Revision: 2}
	if !a.Equal(b) {
		t.Fatal("equivalent independently decoded references broke contract equality")
	}
	b.ManagedLocalModel.Revision++
	if a.Equal(b) {
		t.Fatal("changed managed reference accepted as same selection")
	}
}
