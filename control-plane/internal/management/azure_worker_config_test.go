package management

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAzureWorkerSelectionKeepsIdentityAndSeparatesDurableState(t *testing.T) {
	mount := t.TempDir()
	selection := filepath.Join(mount, "operator.json")
	input := map[string]any{
		"deployment":         map[string]any{"provider": "azure", "scope": "scope-one"},
		"azureMount":         map[string]any{"mountPath": mount, "generation": "gen-one"},
		"azureSecrets":       map[string]any{"clientId": "selected-managed-identity", "directory": "/run/luci"},
		"store":              map[string]any{"ownerID": "controller-owner", "runtimeScope": "scope-one", "directory": "unselected"},
		"controllerStateDir": "unselected", "stagingDir": "unselected",
	}
	encoded, _ := json.Marshal(input)
	if err := os.WriteFile(selection, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"LUCI_AZURE_OPERATOR_TEMPLATE=" + selection, "LUCI_E2B_CONFIG=foreign-user-config", "OPENAI_API_KEY=parent-secret"}
	paths := map[string]bool{}
	for _, user := range []string{"alice", "bob"} {
		root := filepath.Join(mount, "users", user, "runtime")
		if err := os.MkdirAll(filepath.Join(root, "state"), 0700); err != nil {
			t.Fatal(err)
		}
		path, err := prepareAzureWorkerConfig(base, root)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var selected map[string]any
		if err := json.Unmarshal(data, &selected); err != nil {
			t.Fatal(err)
		}
		store := selected["store"].(map[string]any)
		if store["ownerID"] != "controller-owner" || store["runtimeScope"] != "scope-one" {
			t.Fatal("disk identity changed")
		}
		for _, value := range []string{selected["controllerStateDir"].(string), selected["stagingDir"].(string), store["directory"].(string)} {
			if paths[value] {
				t.Fatal("users share durable runtime state")
			}
			paths[value] = true
		}
		if selected["azureSecrets"].(map[string]any)["clientId"] != "selected-managed-identity" {
			t.Fatal("managed identity changed")
		}
		env := isolatedEnvironment(base, root, "private-worker-token", filepath.Join(root, "address"), User{ID: user, Role: "member"})
		for _, entry := range env {
			if entry == "LUCI_E2B_CONFIG=foreign-user-config" || entry == "OPENAI_API_KEY=parent-secret" {
				t.Fatal("inherited foreign capability")
			}
		}
	}
	if _, err := prepareAzureWorkerConfig(base, t.TempDir()); err == nil {
		t.Fatal("accepted state outside fenced mount")
	}
}

func TestAzureWorkerSelectionRefusesInlineSecretsAndDuplicateSelection(t *testing.T) {
	mount := t.TempDir()
	selection := filepath.Join(mount, "operator.json")
	data, _ := json.Marshal(map[string]any{"deployment": map[string]any{"provider": "azure"},
		"azureMount": map[string]any{"mountPath": mount}, "store": map[string]any{}, "azureSecrets": map[string]any{}, "apiKeyFile": "/foreign/secret"})
	if err := os.WriteFile(selection, data, 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"LUCI_AZURE_OPERATOR_TEMPLATE=" + selection}
	root := filepath.Join(mount, "users", "one")
	if err := os.MkdirAll(filepath.Join(root, "state"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareAzureWorkerConfig(base, root); err == nil {
		t.Fatal("accepted inline credentials")
	}
	if _, err := prepareAzureWorkerConfig(append(base, base...), filepath.Join(mount, "users", "one")); err == nil {
		t.Fatal("accepted duplicate source")
	}
	if got, err := prepareAzureWorkerConfig(nil, mount); err != nil || got != "" {
		t.Fatal("changed non-Azure launcher")
	}
}

func TestAzureWorkerSelectionRefusesDeferredTaskOperator(t *testing.T) {
	mount := t.TempDir()
	selection := filepath.Join(mount, "operator.json")
	if err := os.WriteFile(selection, []byte(`{"setupDeferred":{"version":1}}`), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(mount, "users", "one")
	if err := os.MkdirAll(filepath.Join(root, "state"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareAzureWorkerConfig([]string{"LUCI_AZURE_OPERATOR_TEMPLATE=" + selection}, root); err == nil || !strings.Contains(err.Error(), "deferred") {
		t.Fatal("deferred management bootstrap could start a task operator", err)
	}
}
