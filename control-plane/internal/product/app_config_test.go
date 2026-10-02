package product

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationConfigurationIsRevisionedPrivateAndExcludedFromBackup(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerApplicationConfiguration(mux)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "private-config")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Private runtime settings stay outside this brief"}, 200)
	initial := journeyRequest(t, mux, "GET", base+"/configuration", nil, 200)
	if !strings.Contains(initial.Body.String(), `"variables":[]`) || !strings.Contains(initial.Body.String(), `"revision":0`) {
		t.Fatal(initial.Body.String())
	}
	value := "private-runtime-value-$KEEP_LITERAL-123"
	body := map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": "DATABASE_URL", "value": value}, {"name": "EMPTY", "value": ""}}}
	saved := journeyRequest(t, mux, "POST", base+"/configuration", body, 200)
	for _, output := range []string{saved.Body.String(), journeyRequest(t, mux, "GET", base+"/configuration", nil, 200).Body.String()} {
		if strings.Contains(output, value) || strings.Contains(output, `"values"`) || !strings.Contains(output, `"name":"DATABASE_URL"`) {
			t.Fatal(output)
		}
	}
	snapshot, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil || snapshot.Revision != 1 || snapshot.Values["DATABASE_URL"] != value || !containsApplicationVariable(snapshot.Values, "EMPTY") {
		t.Fatal("snapshot lost its values/revision", err)
	}
	snapshot.Values["DATABASE_URL"] = "caller-mutated-copy"
	before, _ := s.applicationConfigurationSnapshot(p.ID)
	if before.Values["DATABASE_URL"] != value {
		t.Fatal("snapshot shared mutable state")
	}
	path, err := s.privateApplicationStatePath("application-config-" + p.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateFile(t, path)
	privateBytes, _ := os.ReadFile(path)
	journeyRequest(t, mux, "POST", base+"/configuration", body, 409)
	afterBytes, _ := os.ReadFile(path)
	if !bytes.Equal(privateBytes, afterBytes) {
		t.Fatal("stale update changed private state")
	}
	commitPreviewFixture(t, p.Path)
	backup := journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	archive, err := zip.NewReader(bytes.NewReader(backup.Body.Bytes()), int64(backup.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(value)) || strings.Contains(file.Name, "application-config") {
			t.Fatal("private runtime configuration entered a project backup")
		}
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered, err := restarted.applicationConfigurationSnapshot(p.ID)
	if err != nil || recovered.Revision != 1 || recovered.Values["DATABASE_URL"] != value {
		t.Fatal("restart lost private configuration", err)
	}
	journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": 1, "updates": []map[string]any{{"name": "DATABASE_URL", "remove": true}}}, 200)
	removed, _ := s.applicationConfigurationSnapshot(p.ID)
	if removed.Revision != 2 || containsApplicationVariable(removed.Values, "DATABASE_URL") {
		t.Fatal("explicit removal was not saved")
	}
}

func TestApplicationConfigurationRejectsInvalidUpdatesWithoutReflectingValues(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerApplicationConfiguration(mux)
	p := createJourneyProject(t, mux, "config-validation")
	path := "/v1/product/projects/" + p.ID + "/configuration"
	for _, update := range []map[string]any{
		{"name": "BAD-NAME", "value": "sensitive-invalid-value"},
		{"name": "OK", "value": "sensitive-invalid-value\nnext"},
		{"name": "OK", "value": "sensitive-invalid-value\rnext"},
		{"name": "OK", "value": "sensitive-invalid-value\x00next"},
		{"name": "OK", "value": strings.Repeat("s", (16<<10)+1)},
		{"name": "OK"}, {"name": "OK", "value": "secret", "remove": true},
	} {
		w := journeyRequest(t, mux, "POST", path, map[string]any{"expectedRevision": 0, "updates": []map[string]any{update}}, 400)
		if strings.Contains(w.Body.String(), "sensitive-invalid-value") {
			t.Fatal("validation reflected secret content")
		}
	}
	journeyRequest(t, mux, "POST", path, map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": "DUP", "value": "a"}, {"name": "DUP", "value": "b"}}}, 400)
	updates := []map[string]any{}
	for i := 0; i < 17; i++ {
		updates = append(updates, map[string]any{"name": "VALUE_" + string(rune('A'+i)), "value": strings.Repeat("x", 16<<10)})
	}
	journeyRequest(t, mux, "POST", path, map[string]any{"expectedRevision": 0, "updates": updates}, 400)
	r := httptest.NewRequest("POST", path, strings.NewReader(`{"expectedRevision":"sensitive-invalid-value","updates":[]}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 400 || strings.Contains(w.Body.String(), "sensitive-invalid-value") {
		t.Fatal(w.Code, w.Body.String())
	}
	snapshot, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil || snapshot.Revision != 0 || len(snapshot.Values) != 0 {
		t.Fatal("invalid input mutated configuration", err)
	}
}

func TestApplicationConfigurationRejectsHardLinkedPrivateState(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerApplicationConfiguration(mux)
	p := createJourneyProject(t, mux, "config-one-name")
	base := "/v1/product/projects/" + p.ID + "/configuration"
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": "SECRET", "value": "private-fixture-value"}}}, 200)
	path, err := s.privateApplicationStatePath("application-config-" + p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Link(path, filepath.Join(t.TempDir(), "second-name")); err != nil {
		t.Skip("Hard links unavailable:", err)
	}
	w := journeyRequest(t, mux, "GET", base, nil, 500)
	if strings.Contains(w.Body.String(), "private-fixture-value") {
		t.Fatal("unsafe state exposed value")
	}
	if _, err = s.applicationConfigurationSnapshot(p.ID); err == nil {
		t.Fatal("hard-linked state accepted")
	}
}

func TestApplicationConfigurationMetadataNeverHasValueFields(t *testing.T) {
	data, _ := json.Marshal(applicationConfigurationMetadata(applicationConfiguration{Revision: 7, Values: map[string]string{"API_KEY": "fixture-private"}}))
	if bytes.Contains(data, []byte("fixture-private")) || bytes.Contains(data, []byte(`"value`)) {
		t.Fatal(string(data))
	}
}
