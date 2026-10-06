package product

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func backupUpload(t *testing.T, mux http.Handler, name string, data []byte, want int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/v1/product/projects/restore?name="+name, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/zip")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("restore status%d want%d: %s", w.Code, want, w.Body.String())
	}
	return w
}
func TestBackupRestoresRealGitWorkingFilesAndJourneyWithoutCredentials(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "source")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Retain my real work", "criteria": []string{"Preserve history"}}, 200)
	frames := []Frame{{ID: "home", Name: "My design", Width: 400, Height: 300, Background: "#ffffff", Elements: []Element{}}}
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 0, "frames": frames}, 200)
	frames[0].Name = "Revised design"
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 1, "frames": frames}, 200)
	if err := os.WriteFile(filepath.Join(p.Path, "readme.txt"), []byte("committed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "delete.txt"), []byte("delete later"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "saved project"), 200)
	head, _ := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	for _, args := range [][]string{{"branch", "feature"}, {"tag", "v1"}} {
		if _, err := s.journeyGit(context.Background(), p.Path, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(p.Path, "readme.txt"), []byte("working change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(p.Path, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "draft.txt"), []byte("untracked working file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".env"), []byte("SECRET=do-not-export"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.stateDir, "secrets.json"), []byte("host-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "remote", "add", "origin", "https://token@example.com/private.git"); err != nil {
		t.Fatal(err)
	}
	w := journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	if w.Header().Get("Content-Type") != "application/zip" {
		t.Fatal("not a ZIP")
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if strings.Contains(f.Name, ".env") || strings.Contains(f.Name, "secrets.json") || strings.Contains(f.Name, ".git/") {
			t.Fatalf("secret entry %s", f.Name)
		}
	}
	restoredResponse := backupUpload(t, mux, "restored", w.Body.Bytes(), 201)
	var restored project.Project
	if err = json.Unmarshal(restoredResponse.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ID == p.ID {
		t.Fatal("restored identity not rebound")
	}
	for name, want := range map[string]string{"readme.txt": "working change\n", "draft.txt": "untracked working file\n"} {
		data, err := os.ReadFile(filepath.Join(restored.Path, name))
		if err != nil || string(data) != want {
			t.Fatalf("%s data%q err%v", name, data, err)
		}
	}
	for _, name := range []string{"delete.txt", ".env"} {
		if _, err := os.Stat(filepath.Join(restored.Path, name)); !os.IsNotExist(err) {
			t.Fatalf("excluded/deleted file exists: %s", name)
		}
	}
	actual, _ := s.journeyGit(context.Background(), restored.Path, "rev-parse", "HEAD")
	if actual != head {
		t.Fatalf("history changed: %s vs %s", actual, head)
	}
	origin, _ := s.journeyGit(context.Background(), restored.Path, "config", "--get", "remote.origin.url")
	if origin != "" {
		t.Fatal("credential-bearing remote imported")
	}
	state, err := readJourney(restored.Path, restored.ID)
	if err != nil || state.Brief.Revision != 1 || state.Brief.ProjectID != restored.ID || state.Brief.Purpose != "Retain my real work" {
		t.Fatalf("history %+v %v", state, err)
	}
	if len(state.BriefRevisions) != 1 || state.BriefRevisions[0].ProjectID != restored.ID {
		t.Fatalf("revision history %+v", state.BriefRevisions)
	}
	if state.Design.Revision != 2 || len(state.DesignRevisions) != 2 || state.Design.ProjectID != restored.ID || state.DesignRevisions[0].ProjectID != restored.ID || state.Design.Frames[0].Name != "Revised design" {
		t.Fatalf("design history lost: %+v", state.Design)
	}
	for _, ref := range []string{"refs/heads/feature", "refs/tags/v1"} {
		got, err := s.journeyGit(context.Background(), restored.Path, "rev-parse", ref)
		if err != nil || got != head {
			t.Fatalf("ref%s=%s err%v", ref, got, err)
		}
	}
	status, _ := s.journeyGit(context.Background(), restored.Path, "status", "--porcelain=v1")
	if !strings.Contains(status, "delete.txt") || !strings.Contains(status, "draft.txt") || !strings.Contains(status, "LUCI.md") {
		t.Fatalf("working state not preserved/rebound: %s", status)
	}
	backupUpload(t, mux, "restored", w.Body.Bytes(), 409)
	data, _ := os.ReadFile(filepath.Join(restored.Path, "readme.txt"))
	if string(data) != "working change\n" {
		t.Fatal("existing restore was overwritten")
	}
}

func craftedBackup(t *testing.T, entries map[string][]byte, mutate func(*backupManifest), extra *zip.FileHeader) []byte {
	t.Helper()
	m := backupManifest{Version: 1, Name: "source", ProjectID: "test", Branch: "main", Entries: []backupEntry{}}
	if entries == nil {
		entries = map[string][]byte{"journey.json": []byte(`{"version":1,"brief":{"projectId":"test"},"design":{"projectId":"test"}}`), "files/readme.txt": []byte("hello")}
	}
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	for name, data := range entries {
		hash := sha256.Sum256(data)
		m.Entries = append(m.Entries, backupEntry{Path: name, Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])})
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if mutate != nil {
		mutate(&m)
	}
	w, _ := z.Create("manifest.json")
	json.NewEncoder(w).Encode(m)
	if extra != nil {
		w, err := z.CreateHeader(extra)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, "bad")
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestBackupRejectsTraversalLinksProtectedFilesHashTamperingAndDuplicatePaths(t *testing.T) {
	link := &zip.FileHeader{Name: "files/link", Method: zip.Store}
	link.SetMode(os.ModeSymlink | 0777)
	cases := map[string][]byte{
		"traversal":   craftedBackup(t, map[string][]byte{"journey.json": []byte("{}"), "files/../escape": []byte("bad")}, nil, nil),
		"protected":   craftedBackup(t, map[string][]byte{"journey.json": []byte("{}"), "files/.git/config": []byte("bad")}, nil, nil),
		"credentials": craftedBackup(t, map[string][]byte{"journey.json": []byte("{}"), "files/.env": []byte("bad")}, nil, nil),
		"hash":        craftedBackup(t, nil, func(m *backupManifest) { m.Entries[0].SHA256 = strings.Repeat("0", 64) }, nil),
		"duplicate":   craftedBackup(t, nil, nil, &zip.FileHeader{Name: "files/README.TXT", Method: zip.Store}),
		"link":        craftedBackup(t, nil, nil, link),
		"device":      craftedBackup(t, map[string][]byte{"journey.json": []byte("{}"), "files/CON.txt": []byte("bad")}, nil, nil),
	}
	for name, archive := range cases {
		t.Run(name, func(t *testing.T) {
			s, mux := journeyFixture(t)
			s.registerBackups(mux)
			backupUpload(t, mux, "restored", archive, 400)
			entries, err := os.ReadDir(s.projectRoot())
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid backup mutated projects: %v %v", entries, err)
			}
		})
	}
}
func TestBackupRestoresUncommittedProjectAndRejectsUnsafeName(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "empty")
	w := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/backups", map[string]any{}, 200)
	backupUpload(t, mux, "emptycopy", w.Body.Bytes(), 201)
	backupUpload(t, mux, "..", w.Body.Bytes(), 400)
}
