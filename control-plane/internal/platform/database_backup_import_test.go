package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func databaseImportRequest(s *Service, r *http.Request) *httptest.ResponseRecorder {
	token := strings.Repeat("private-worker-", 4)
	actor, _ := json.Marshal(management.User{ID: "test-user", Role: "member"})
	policy, _ := json.Marshal(management.Policy{InstanceName: "test", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}, AllowCloudProvisioning: true})
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
	r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
	w := httptest.NewRecorder()
	management.WorkerHandler(token, s.Handler()).ServeHTTP(w, r)
	return w
}

func dumpUpload(data []byte) *http.Request {
	r := httptest.NewRequest("POST", "/v1/platform/projects/owned/backups/import", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/octet-stream")
	r.Header.Set("X-Luci-Trust-Backup", "true")
	sum := sha256.Sum256(data)
	r.Header.Set("X-Luci-Backup-SHA256", hex.EncodeToString(sum[:]))
	return r
}

func TestDatabaseImportRetainsPrivateBytesAndRetriesWithoutExecution(t *testing.T) {
	calls := 0
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) {
		calls++
		return nil, errors.New("must not execute")
	})
	data := []byte("PGDMP\x00\xff\nimported-fixture")
	w := databaseImportRequest(s, dumpUpload(data))
	var backup Backup
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &backup) != nil || !backup.Imported || backup.Bytes != int64(len(data)) || backup.Database != "" {
		t.Fatalf("import status/receipt: %d %s", w.Code, w.Body.String())
	}
	file, _ := s.path(backup.ID)
	info, err := os.Stat(file)
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("import was not privately stored", err)
	}
	stored, err := s.databaseBackupBytes(backup)
	if err != nil || !bytes.Equal(stored, data) {
		t.Fatal("import changed binary bytes", err)
	}
	state := databaseFixtureState(t, s)
	if !reflect.DeepEqual(state.Backups, []Backup{backup}) || state.Database.Status != "not-created" || state.DatabaseOperation != nil || calls != 0 {
		t.Fatal("import started a database, restore or external command")
	}
	w = databaseImportRequest(s, dumpUpload(data))
	var retry Backup
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &retry) != nil || retry != backup || len(databaseFixtureState(t, s).Backups) != 1 {
		t.Fatal("identical transport retry allocated another archive")
	}
	w = request(t, s, "GET", "/v1/platform/projects/owned/backups/"+backup.ID, "member", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatal("imported archive cannot be downloaded unchanged")
	}
	if err := os.WriteFile(file, []byte("PGDMP-tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	w = databaseImportRequest(s, dumpUpload(data))
	if w.Code != 409 {
		t.Fatal("retry silently repaired/replaced a changed receipt")
	}
	if changed, _ := os.ReadFile(file); string(changed) != "PGDMP-tampered" {
		t.Fatal("existing private archive overwritten")
	}
}

type failedDumpReader struct{}

func (failedDumpReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type repeatingDumpReader struct{}

func (repeatingDumpReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestDatabaseImportRefusesUntrustedForeignBrokenAndOversizedUploads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		change func(*Service, *http.Request)
	}{
		{"foreign-project", 404, func(_ *Service, r *http.Request) { r.URL.Path = "/v1/platform/projects/foreign/backups/import" }},
		{"no-trust", 400, func(_ *Service, r *http.Request) { r.Header.Del("X-Luci-Trust-Backup") }},
		{"no-checksum", 400, func(_ *Service, r *http.Request) { r.Header.Del("X-Luci-Backup-SHA256") }},
		{"wrong-checksum", 409, func(_ *Service, r *http.Request) { r.Header.Set("X-Luci-Backup-SHA256", strings.Repeat("0", 64)) }},
		{"wrong-content-type", 415, func(_ *Service, r *http.Request) { r.Header.Set("Content-Type", "application/zip") }},
		{"sql-text", 400, func(_ *Service, r *http.Request) { r.Body = io.NopCloser(strings.NewReader("SELECT 1;")) }},
		{"truncated-stream", 400, func(_ *Service, r *http.Request) { r.Body = io.NopCloser(failedDumpReader{}) }},
		{"declared-oversize", 413, func(_ *Service, r *http.Request) { r.ContentLength = databaseBackupLimit + 1 }},
		{"streamed-oversize", 413, func(_ *Service, r *http.Request) {
			r.ContentLength = -1
			r.Body = io.NopCloser(io.LimitReader(repeatingDumpReader{}, databaseBackupLimit+1))
		}},
		{"cancelled", 400, func(_ *Service, r *http.Request) {
			ctx, cancel := context.WithCancel(r.Context())
			cancel()
			*r = *r.WithContext(ctx)
		}},
		{"busy-project", 409, func(s *Service, _ *http.Request) {
			if !s.claim("owned", 1) {
				panic("fixture lock")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) { calls++; return nil, nil })
			r := dumpUpload([]byte("PGDMP-fixture"))
			tc.change(s, r)
			w := databaseImportRequest(s, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			state := databaseFixtureState(t, s)
			if len(state.Backups) != 0 || state.DatabaseOperation != nil || calls != 0 {
				t.Fatal("rejected import mutated resources/history")
			}
			entries, _ := os.ReadDir(s.dir)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "backup-") {
					t.Fatal("rejected upload left private archive")
				}
			}
		})
	}
	s := fixture(t, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, dumpUpload([]byte("PGDMP-fixture")))
	if w.Code != 401 {
		t.Fatal("unauthenticated import admitted")
	}
}

func TestImportedBackupRequiresSeparateRestoreAndRetainsPreviousDatabase(t *testing.T) {
	s, f, _ := newRestoreDatabaseFixture(t)
	f.dump = []byte("PGDMP-imported-from-recovered-controller")
	w := databaseImportRequest(s, dumpUpload(f.dump))
	var backup Backup
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &backup) != nil {
		t.Fatalf("import: %d", w.Code)
	}
	before := databaseFixtureState(t, s)
	if before.Database.Database != "app" || before.DatabaseOperation != nil {
		t.Fatal("import changed active database")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]any{"action": "restore", "backupId": backup.ID})
	if w.Code != 400 {
		t.Fatal("import bypassed restore confirmation")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID))
	if w.Code != 202 {
		t.Fatalf("restore: %d %s", w.Code, w.Body.String())
	}
	after := waitDatabaseFixtureOperation(t, s)
	if after.DatabaseOperation.Status != "succeeded" || after.DatabaseOperation.PreviousDatabase != "app" || after.Database.Database == "app" || !f.candidates["app"] {
		t.Fatal("imported restore did not verify a separate candidate and retain original data")
	}
}
