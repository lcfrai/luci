package api_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

// The installed app consumes HTTP JSON, not harness.Catalogue directly. Pin
// that boundary so adding an advertised capability cannot silently disappear
// in the API projection and disable a valid saved route in the actual desktop.
func TestHarnessHTTPPublishesTaskAndInteractiveCapabilities(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/harnesses", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code, rec.Body.String())
	}
	rows := catalogListOf(t, rec.Body.String(), "harnesses")
	for _, row := range rows {
		id, ok := row["id"].(string)
		if !ok {
			t.Fatal("catalog lacks harness identity", row)
		}
		d, ok := harness.Lookup(harness.ID(id))
		if !ok {
			t.Fatal("HTTP catalogue returned an unknown harness", id)
		}
		// Descriptor is explicitly public picker data; every published field
		// must survive the HTTP envelope (available is an additional API field).
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var advertised map[string]any
		if err := json.Unmarshal(data, &advertised); err != nil {
			t.Fatal(err)
		}
		for field, want := range advertised {
			if got, present := row[field]; !present || !reflect.DeepEqual(got, want) {
				t.Errorf("%s.%s disappeared or changed at HTTP boundary: got %#v, want %#v", id, field, got, want)
			}
		}
		modes, ok := row["taskPermissionModes"].([]any)
		if !ok {
			t.Fatalf("%s task permissions must be an array, including unsupported harnesses", id)
		}
		coding := id == string(harness.Codex) || id == string(harness.ClaudeCode) || id == string(harness.OpenCode)
		if row["autonomousTask"] != coding || row["supportsCustomModel"] != coding {
			t.Fatalf("%s has wrong task/custom-model capability: %#v", id, row)
		}
		if coding {
			if len(modes) != 1 || modes[0] != "bypass" || harness.ValidateTask(d.ID, harness.PermissionBypass) != nil {
				t.Fatalf("%s saved Bypass route cannot prepare through the advertised HTTP contract", id)
			}
			notice, _ := row["taskPermissionNotice"].(string)
			if !strings.Contains(notice, "explicitly saved Bypass") || !strings.Contains(notice, "interactive") {
				t.Fatalf("%s omits the permission limitation before preparation: %q", id, notice)
			}
			for _, mode := range []harness.PermissionMode{harness.PermissionAsk, harness.PermissionPlan} {
				if harness.ValidateTask(d.ID, mode) == nil {
					t.Fatalf("%s silently accepts interactive %s for unattended tasks", id, mode)
				}
			}
		} else if len(modes) != 0 {
			t.Fatalf("%s advertises unsupported task permissions", id)
		}
		if d.ID == harness.Codex {
			notice, _ := row["dockerAskNotice"].(string)
			if !strings.Contains(notice, "on-request") || !strings.Contains(notice, "container") {
				t.Fatal("HTTP catalog hid the existing Docker Ask boundary", notice)
			}
		}
	}
}
