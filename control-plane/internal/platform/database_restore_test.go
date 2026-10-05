package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// This runner exercises the real admission, durable job and command boundary.
// It is not a substitute for the separately required installed PostgreSQL UAT.
type restoreDatabaseFixture struct {
	s                                                            *Service
	mu                                                           sync.Mutex
	running, appRunning                                          bool
	foreign, wrongImage, wrongVolume, foreignVolume              bool
	wrongCredential                                              bool
	failCreate, failRestore, failHealth, failStop                bool
	dump                                                         []byte
	commands                                                     [][]string
	candidates                                                   map[string]bool
	restoreEntered, restoreRelease, healthEntered, healthRelease chan struct{}
	restoreOnce, healthOnce                                      sync.Once
}

func newRestoreDatabaseFixture(t *testing.T) (*Service, *restoreDatabaseFixture, Backup) {
	t.Helper()
	f := &restoreDatabaseFixture{dump: []byte("PGDMP-owned-snapshot"), candidates: map[string]bool{"app": true, "postgres": true}}
	s := fixture(t, f.run)
	f.s = s
	sum := sha256.Sum256(f.dump)
	backup := Backup{ID: "backup-owned", Database: "app", CreatedAt: now(), Bytes: int64(len(f.dump)), SHA256: hex.EncodeToString(sum[:])}
	file, err := s.path(backup.ID)
	if err != nil || os.WriteFile(file, f.dump, 0600) != nil {
		t.Fatal("could not prepare private backup")
	}
	if err := s.write("owned-database-secret", databaseSecret{Password: strings.Repeat("a", 24)}); err != nil {
		t.Fatal(err)
	}
	if err := s.update("owned", func(v *ProjectState) {
		v.Database = Database{Status: "stopped", Engine: "PostgreSQL 16.15", Database: "app", Username: "luci", Container: s.resource("owned", "postgres")}
		v.Backups = []Backup{backup}
	}); err != nil {
		t.Fatal(err)
	}
	return s, f, backup
}

func (f *restoreDatabaseFixture) run(ctx context.Context, exe string, args []string, _ string, input []byte) ([]byte, error) {
	if exe != "docker" || len(args) == 0 {
		return nil, errors.New("unexpected executable")
	}
	f.mu.Lock()
	f.commands = append(f.commands, append([]string(nil), args...))
	switch args[0] {
	case "inspect":
		v := containerInfo{}
		v.Config.Labels = map[string]string{labelInstance: f.s.instance, labelProject: "owned", labelResource: "postgres"}
		v.Config.Image = postgresImage
		v.Config.Env = []string{"POSTGRES_PASSWORD=" + strings.Repeat("a", 24), "POSTGRES_USER=luci"}
		v.State.Running = f.running
		v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"5432/tcp": {{"127.0.0.1", "54321"}}}
		v.Mounts = append(v.Mounts, struct {
			Type, Name, Destination string
			RW                      bool
		}{"volume", f.s.resource("owned", "postgres-data"), "/var/lib/postgresql/data", true})
		if f.foreign {
			v.Config.Labels[labelProject] = "foreign"
		}
		if f.wrongImage {
			v.Config.Image = "postgres:latest"
		}
		if f.wrongVolume {
			v.Mounts[0].Name = "foreign-data"
		}
		if f.wrongCredential {
			v.Config.Env[0] = "POSTGRES_PASSWORD=" + strings.Repeat("b", 24)
		}
		if args[1] == "app-container" {
			v.Config.Labels[labelResource] = "deploy-owned"
			v.State.Running = f.appRunning
		}
		data, err := json.Marshal([]containerInfo{v})
		f.mu.Unlock()
		return data, err
	case "volume":
		if args[1] != "inspect" {
			f.mu.Unlock()
			return nil, errors.New("volume mutation forbidden")
		}
		labels := map[string]string{labelInstance: f.s.instance, labelProject: "owned", labelResource: "postgres-data"}
		if f.foreignVolume {
			labels[labelProject] = "foreign"
		}
		data, err := json.Marshal(labels)
		f.mu.Unlock()
		return data, err
	case "update":
		f.mu.Unlock()
		return nil, nil
	case "start":
		f.running = true
		f.mu.Unlock()
		return nil, nil
	case "stop":
		if f.failStop {
			f.mu.Unlock()
			return nil, errors.New("private-stop-detail")
		}
		f.running = false
		f.mu.Unlock()
		return nil, nil
	case "exec":
		index := 2
		if args[1] == "--interactive" {
			index = 3
		}
		command := args[index]
		if command == "sh" && args[index+1] == "-c" && args[index+3] == "luci-postgres" {
			command = args[index+4]
		}
		switch command {
		case "createdb":
			candidate := args[len(args)-1]
			if !databaseName.MatchString(candidate) || !strings.HasPrefix(candidate, "luci_restore_") || len(candidate) > 63 {
				f.mu.Unlock()
				return nil, errors.New("unsafe candidate identifier")
			}
			if f.failCreate {
				f.mu.Unlock()
				return nil, errors.New("private-create-detail")
			}
			f.candidates[candidate] = false
			f.mu.Unlock()
			return nil, nil
		case "pg_restore":
			candidate := ""
			for _, arg := range args {
				if strings.HasPrefix(arg, "--dbname=") {
					candidate = strings.TrimPrefix(arg, "--dbname=")
				}
			}
			if candidate == "" || !bytes.Equal(input, f.dump) {
				f.mu.Unlock()
				return nil, errors.New("restore command input mismatch")
			}
			entered, release := f.restoreEntered, f.restoreRelease
			fail := f.failRestore
			f.mu.Unlock()
			if entered != nil {
				f.restoreOnce.Do(func() { close(entered) })
			}
			if release != nil {
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if fail {
				return nil, errors.New("private SQL/password/customer-value must stay private")
			}
			f.mu.Lock()
			f.candidates[candidate] = true
			f.mu.Unlock()
			return nil, nil
		case "psql":
			name := ""
			for i, arg := range args {
				if arg == "-d" {
					name = args[i+1]
				}
			}
			if args[len(args)-1] == "SELECT 1" {
				valid := f.candidates[name]
				f.mu.Unlock()
				if valid {
					return []byte("1\n"), nil
				}
				return nil, errors.New("selected database unavailable")
			}
			entered, release := f.healthEntered, f.healthRelease
			valid := f.candidates[name] && !f.failHealth
			f.mu.Unlock()
			if entered != nil {
				f.healthOnce.Do(func() { close(entered) })
			}
			if release != nil {
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if valid {
				return []byte(name + "\n"), nil
			}
			return []byte("wrong_database\n"), nil
		case "pg_dump":
			data := append([]byte(nil), f.dump...)
			f.mu.Unlock()
			return data, nil
		}
	}
	f.mu.Unlock()
	return nil, errors.New("unexpected Docker mutation")
}

func restoreRequest(backupID string) map[string]any {
	return map[string]any{"action": "restore", "backupId": backupID, "confirmRestore": true, "acknowledgeResources": true}
}

func databaseFixtureState(t *testing.T, s *Service) ProjectState {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.load("owned")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func waitDatabaseFixtureOperation(t *testing.T, s *Service) ProjectState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		busy := s.busy["owned"]
		s.mu.Unlock()
		if !busy {
			return databaseFixtureState(t, s)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("bounded restore job did not settle")
	return ProjectState{}
}

func waitDatabaseFixtureSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("job never reached the real command gate")
	}
}

func TestDatabaseRestoreAdmissionPreservesResources(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		change func(*testing.T, *Service, *restoreDatabaseFixture, map[string]any)
	}{
		{"no-confirmation", 400, func(_ *testing.T, _ *Service, _ *restoreDatabaseFixture, in map[string]any) {
			in["confirmRestore"] = false
		}},
		{"no-resource-acknowledgement", 400, func(_ *testing.T, _ *Service, _ *restoreDatabaseFixture, in map[string]any) {
			in["acknowledgeResources"] = false
		}},
		{"foreign-backup", 404, func(_ *testing.T, _ *Service, _ *restoreDatabaseFixture, in map[string]any) {
			in["backupId"] = "backup-foreign"
		}},
		{"invalid-backup-path", 400, func(_ *testing.T, _ *Service, _ *restoreDatabaseFixture, in map[string]any) {
			in["backupId"] = "../backup-owned"
		}},
		{"running-receipt", 409, func(t *testing.T, s *Service, _ *restoreDatabaseFixture, _ map[string]any) {
			if err := s.update("owned", func(v *ProjectState) { v.Database.Status = "ready" }); err != nil {
				t.Fatal(err)
			}
		}},
		{"actually-running", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.running = true }},
		{"foreign-container", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.foreign = true }},
		{"unpinned-image", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.wrongImage = true }},
		{"changed-private-credential", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.wrongCredential = true }},
		{"foreign-volume-mount", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.wrongVolume = true }},
		{"foreign-volume-labels", 409, func(_ *testing.T, _ *Service, f *restoreDatabaseFixture, _ map[string]any) { f.foreignVolume = true }},
		{"changed-checksum", 409, func(t *testing.T, s *Service, _ *restoreDatabaseFixture, _ map[string]any) {
			file, _ := s.path("backup-owned")
			if err := os.WriteFile(file, []byte("PGDMP-changed-value!"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"oversize-receipt", 409, func(t *testing.T, s *Service, _ *restoreDatabaseFixture, _ map[string]any) {
			if err := s.update("owned", func(v *ProjectState) { v.Backups[0].Bytes = databaseBackupLimit + 1 }); err != nil {
				t.Fatal(err)
			}
		}},
		{"running-app", 409, func(t *testing.T, s *Service, f *restoreDatabaseFixture, _ map[string]any) {
			f.appRunning = true
			if err := s.update("owned", func(v *ProjectState) {
				v.Deployments = []Deployment{{ID: "deploy-owned", Provider: "docker", Status: "stopped", Container: "app-container"}}
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{"building-app", 409, func(t *testing.T, s *Service, _ *restoreDatabaseFixture, _ map[string]any) {
			if err := s.update("owned", func(v *ProjectState) {
				v.Deployments = []Deployment{{ID: "deploy-owned", Provider: "docker", Status: "building"}}
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{"conflicting-operation", 409, func(t *testing.T, s *Service, _ *restoreDatabaseFixture, _ map[string]any) {
			if !s.claim("owned", 1) {
				t.Fatal("could not occupy operation slot")
			}
			t.Cleanup(func() { s.release("owned") })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f, backup := newRestoreDatabaseFixture(t)
			in := restoreRequest(backup.ID)
			tc.change(t, s, f, in)
			w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", in)
			if w.Code != tc.status {
				t.Fatalf("admission status %d, want %d", w.Code, tc.status)
			}
			v := databaseFixtureState(t, s)
			if v.DatabaseOperation != nil || v.Database.Database != "app" {
				t.Fatal("rejected restore changed the selection/history")
			}
			for _, args := range f.commands {
				if args[0] != "inspect" && !(args[0] == "volume" && args[1] == "inspect") {
					t.Fatal("rejected restore mutated Docker")
				}
			}
		})
	}
}

func TestDatabaseRestorePromotesOnlyAfterSQLAndPersistsActiveBinding(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	f.healthEntered, f.healthRelease = make(chan struct{}), make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(f.healthRelease)
		}
	}()
	w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID))
	var accepted DatabaseOperation
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &accepted) != nil || accepted.Status != "running" || accepted.Stage != "queued" {
		t.Fatal("restore was not durably accepted as asynchronous")
	}
	waitDatabaseFixtureSignal(t, f.healthEntered)
	v := databaseFixtureState(t, s)
	if v.Database.Database != "app" || v.DatabaseOperation.Stage != "checking" || len(v.DatabaseRestoreHistory) != 1 {
		t.Fatal("candidate selected before its health response")
	}
	for _, action := range []string{"start", "stop", "backup"} {
		if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": action}); w.Code != 409 {
			t.Fatal("running restore did not hold its project operation lock")
		}
	}
	w = request(t, s, "GET", "/v1/platform/projects/owned", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"busy":true`) || strings.Contains(w.Body.String(), strings.Repeat("a", 24)) {
		t.Fatal("progress response missing or leaked private credentials")
	}
	close(f.healthRelease)
	released = true
	v = waitDatabaseFixtureOperation(t, s)
	if v.Database.Status != "ready" || v.Database.Database != accepted.CandidateDatabase || v.DatabaseOperation.Status != "succeeded" || v.DatabaseOperation.Stage != "complete" {
		t.Fatal("verified candidate was not atomically promoted")
	}
	f.mu.Lock()
	if !f.candidates["app"] || !f.candidates[accepted.CandidateDatabase] || !f.running {
		t.Error("original database or healthy candidate lost")
	}
	commands := append([][]string(nil), f.commands...)
	f.mu.Unlock()
	matchedRestore := false
	matchedAuthenticatedHealth := false
	for _, args := range commands {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "pg_restore") {
			matchedRestore = true
			for _, required := range []string{"--interactive", "--dbname=" + accepted.CandidateDatabase, "--single-transaction", "--exit-on-error"} {
				if !strings.Contains(joined, required) {
					t.Error("restore command missing", required)
				}
			}
			if strings.Contains(joined, "--create") || strings.Contains(joined, "--no-owner") || strings.Contains(joined, "--no-privileges") || strings.Contains(joined, strings.Repeat("a", 24)) {
				t.Error("restore target, ownership/ACL fidelity or credential boundary bypassed")
			}
		}
		if args[0] == "rm" || args[0] == "run" || (args[0] == "volume" && args[1] != "inspect") || strings.Contains(joined, "dropdb") {
			t.Error("restore discarded original resources or created another service")
		}
		if strings.Contains(joined, "SELECT current_database()") {
			matchedAuthenticatedHealth = true
			if !strings.Contains(joined, `PGPASSWORD="$POSTGRES_PASSWORD" exec "$@"`) || !strings.Contains(joined, "psql -h 127.0.0.1 -U luci -d "+accepted.CandidateDatabase) || strings.Contains(joined, strings.Repeat("a", 24)) {
				t.Error("candidate health did not privately verify the application TCP credential binding")
			}
		}
	}
	if !matchedRestore {
		t.Fatal("no PostgreSQL restore command ran")
	}
	if !matchedAuthenticatedHealth {
		t.Fatal("candidate never received its authenticated SQL check")
	}
	_, values, err := s.environment("owned")
	if err != nil || values["DATABASE_URL"] != "postgresql://luci:"+strings.Repeat("a", 24)+"@database:5432/"+accepted.CandidateDatabase+"?sslmode=disable" {
		t.Fatal("promoted active database was not injected privately")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "backup"})
	var next Backup
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &next) != nil || next.Database != accepted.CandidateDatabase {
		t.Fatal("backup did not use the restored selection")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "stop"})
	if w.Code != 200 {
		t.Fatal("restored database stop failed")
	}
	s.Close()
	resumed, err := New(s.opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(resumed.Close)
	w = request(t, resumed, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "start"})
	if w.Code != 200 {
		t.Fatal("restored database restart failed")
	}
	v = databaseFixtureState(t, resumed)
	if v.Database.Database != accepted.CandidateDatabase || v.DatabaseOperation.Status != "succeeded" || len(v.DatabaseRestoreHistory) != 1 {
		t.Fatal("restart lost active database/lineage")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	lastSQL := ""
	lastDump := ""
	for _, args := range f.commands {
		if args[0] == "exec" && len(args) > 6 && args[2] == "psql" && args[len(args)-1] == "SELECT 1" {
			lastSQL = args[6]
		}
		if args[0] == "exec" && args[2] == "pg_dump" {
			lastDump = args[6]
		}
	}
	if lastSQL != accepted.CandidateDatabase || lastDump != accepted.CandidateDatabase {
		t.Fatal("lifecycle returned to the original database")
	}
}

func TestDatabaseRestoreFailureRetainsPreviousAndRetryCreatesNewCandidate(t *testing.T) {
	for _, stage := range []string{"create", "restore", "health"} {
		t.Run(stage, func(t *testing.T) {
			s, f, backup := newRestoreDatabaseFixture(t)
			f.failCreate = stage == "create"
			f.failRestore = stage == "restore"
			f.failHealth = stage == "health"
			w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID))
			if w.Code != 202 {
				t.Fatal("valid restore not accepted")
			}
			v := waitDatabaseFixtureOperation(t, s)
			failed := *v.DatabaseOperation
			if failed.Status != "failed" || v.Database.Database != "app" || v.Database.Status != "stopped" {
				t.Fatal("failed candidate replaced the previous database or left it running")
			}
			if strings.Contains(failed.Message, "private") || strings.Contains(failed.Message, "customer-value") {
				t.Fatal("SQL diagnostics leaked")
			}
			f.mu.Lock()
			f.failCreate = false
			f.failRestore = false
			f.failHealth = false
			if !f.candidates["app"] || f.running {
				t.Error("original data or stop state lost")
			}
			f.mu.Unlock()
			w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID))
			if w.Code != 202 {
				t.Fatal("failed restore could not be retried")
			}
			v = waitDatabaseFixtureOperation(t, s)
			if v.DatabaseOperation.Status != "succeeded" || v.DatabaseOperation.CandidateDatabase == failed.CandidateDatabase || len(v.DatabaseRestoreHistory) != 2 || v.DatabaseRestoreHistory[1].Status != "failed" || v.DatabaseRestoreHistory[1].ID != failed.ID {
				t.Fatal("retry reused candidate or lost failed lineage")
			}
		})
	}
}

func TestDatabaseRestoreFailedStopRequiresExplicitVerification(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	f.failRestore, f.failStop = true, true
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID)); w.Code != 202 {
		t.Fatal("not accepted")
	}
	v := waitDatabaseFixtureOperation(t, s)
	if v.Database.Database != "app" || v.Database.Status != "unknown" || !strings.Contains(v.DatabaseOperation.Message, "stopped state could not be verified") {
		t.Fatal("unverified rollback reported stopped")
	}
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID)); w.Code != 409 {
		t.Fatal("unknown running state allowed retry")
	}
}

func TestDatabaseRestoreCloseCancelsWorkerAndPreservesPrevious(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	f.restoreEntered, f.restoreRelease = make(chan struct{}), make(chan struct{})
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID)); w.Code != 202 {
		t.Fatal("not accepted")
	}
	waitDatabaseFixtureSignal(t, f.restoreEntered)
	s.Close()
	v := databaseFixtureState(t, s)
	if v.DatabaseOperation.Status != "interrupted" || v.Database.Database != "app" || v.Database.Status != "stopped" {
		t.Fatal("shutdown auto-promoted, failed to stop or lost interrupted operation")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running || !f.candidates["app"] {
		t.Fatal("close did not preserve original stopped data")
	}
}

func TestDatabaseRestoreRestartObservationIsReadOnlyAndNeverResumes(t *testing.T) {
	s, f, _ := newRestoreDatabaseFixture(t)
	op := DatabaseOperation{ID: "database-op-owned", Action: "restore", Status: "running", Stage: "restoring", BackupID: "backup-owned", PreviousDatabase: "app", CandidateDatabase: "luci_restore_retained", CreatedAt: now(), UpdatedAt: now()}
	if err := s.update("owned", func(v *ProjectState) { setDatabaseOperation(v, op); v.Database.Status = "restoring" }); err != nil {
		t.Fatal(err)
	}
	file, _ := s.path("owned")
	before, _ := os.ReadFile(file)
	w := request(t, s, "GET", "/v1/platform/projects/owned", "member", nil)
	after, _ := os.ReadFile(file)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"interrupted"`) || !strings.Contains(w.Body.String(), `"status":"unknown"`) || !bytes.Equal(before, after) || len(f.commands) != 0 {
		t.Fatal("restart observation wrote state or resumed a restore")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "stop"})
	if w.Code != 200 {
		t.Fatal("explicit stop could not recover interrupted operation")
	}
	v := databaseFixtureState(t, s)
	if v.DatabaseOperation.Status != "interrupted" || v.DatabaseRestoreHistory[0].Status != "interrupted" || v.Database.Status != "stopped" || v.Database.Database != "app" {
		t.Fatal("explicit recovery lost interruption or selection")
	}
}

func TestDatabaseRestoreAcceptedJobOutlivesHTTPRequest(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	f.restoreEntered, f.restoreRelease = make(chan struct{}), make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(f.restoreRelease)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := httptest.NewRecorder()
	s.acceptDatabaseRestore(w, httptest.NewRequest("POST", "/", nil).WithContext(ctx), "owned", backup.ID)
	if w.Code != 202 {
		t.Fatal("restore not accepted")
	}
	waitDatabaseFixtureSignal(t, f.restoreEntered)
	cancel()
	v := databaseFixtureState(t, s)
	if v.DatabaseOperation.Status != "running" {
		t.Fatal("HTTP return/cancellation canceled the accepted job")
	}
	close(f.restoreRelease)
	released = true
	v = waitDatabaseFixtureOperation(t, s)
	if v.DatabaseOperation.Status != "succeeded" {
		t.Fatal("accepted operation did not finish independently")
	}
}

func TestDatabaseBackupRejectsDirectoryAndLinks(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			s, _, backup := newRestoreDatabaseFixture(t)
			file, _ := s.path(backup.ID)
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			if kind == "directory" {
				if err := os.Mkdir(file, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				outside := t.TempDir() + "/outside.dump"
				if err := os.WriteFile(outside, []byte("PGDMP-owned-snapshot"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, file); err != nil {
					t.Skip("native symlink unavailable")
				}
			}
			if _, err := s.databaseBackupBytes(backup); err == nil {
				t.Fatal("linked/directory backup admitted")
			}
		})
	}
}

func TestDatabaseRollbackUsesOnlySuccessfulActiveHistoryAndSQLHealth(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID)); w.Code != 202 {
		t.Fatal("restore not accepted")
	}
	v := waitDatabaseFixtureOperation(t, s)
	first := *v.DatabaseOperation
	if first.Status != "succeeded" {
		t.Fatal("restore failed")
	}
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "stop"}); w.Code != 200 {
		t.Fatal("stop failed")
	}
	f.mu.Lock()
	before := len(f.commands)
	f.mu.Unlock()
	in := map[string]any{"action": "rollback-restore", "restoreId": first.ID, "confirmRestore": true, "acknowledgeResources": true}
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", in); w.Code != 202 {
		t.Fatal("history-bound recovery was not accepted")
	}
	v = waitDatabaseFixtureOperation(t, s)
	rollback := *v.DatabaseOperation
	if rollback.Status != "succeeded" || rollback.Action != "rollback-restore" || rollback.RestoreID != first.ID || rollback.PreviousDatabase != first.CandidateDatabase || rollback.CandidateDatabase != "app" || v.Database.Database != "app" || v.Database.Status != "ready" || len(v.DatabaseRestoreHistory) != 2 {
		t.Fatal("rollback failed to health-promote the recorded prior selection")
	}
	f.mu.Lock()
	for _, args := range f.commands[before:] {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "createdb") || strings.Contains(joined, "pg_restore") || strings.Contains(joined, "dropdb") {
			t.Error("rollback performed an SQL restore/create/delete instead of selecting proven retained data")
		}
	}
	if !f.candidates["app"] || !f.candidates[first.CandidateDatabase] {
		t.Error("rollback discarded retained databases")
	}
	f.mu.Unlock()
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "stop"}); w.Code != 200 {
		t.Fatal("stop failed")
	}
	// The old restore no longer matches the active database; clients cannot
	// replay it to select an arbitrary or stale previous name.
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", in); w.Code != 409 {
		t.Fatal("stale successful history was admitted")
	}
	in["restoreId"] = rollback.ID
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", in); w.Code != 202 {
		t.Fatal("retained rollback lineage could not recover forward")
	}
	v = waitDatabaseFixtureOperation(t, s)
	if v.Database.Database != first.CandidateDatabase || v.DatabaseOperation.Status != "succeeded" || len(v.DatabaseRestoreHistory) != 3 {
		t.Fatal("previous-database lineage was not durable")
	}
}

func TestDatabaseRollbackRefusesForeignFailedAndUnhealthyPrevious(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*Service, *restoreDatabaseFixture, *DatabaseOperation)
		accepted bool
	}{
		{"foreign-id", func(_ *Service, _ *restoreDatabaseFixture, op *DatabaseOperation) { op.ID = "other-id" }, false},
		{"failed-history", func(_ *Service, _ *restoreDatabaseFixture, op *DatabaseOperation) { op.Status = "failed" }, false},
		{"wrong-active-binding", func(_ *Service, _ *restoreDatabaseFixture, op *DatabaseOperation) {
			op.CandidateDatabase = "luci_restore_other"
		}, false},
		{"unsafe-previous", func(_ *Service, _ *restoreDatabaseFixture, op *DatabaseOperation) {
			op.PreviousDatabase = "app;DROP DATABASE app"
		}, false},
		{"same-previous", func(_ *Service, _ *restoreDatabaseFixture, op *DatabaseOperation) {
			op.PreviousDatabase = "luci_restore_current"
		}, false},
		{"unhealthy-previous", func(_ *Service, f *restoreDatabaseFixture, _ *DatabaseOperation) { f.failHealth = true }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f, _ := newRestoreDatabaseFixture(t)
			op := DatabaseOperation{ID: "database-op-prior", Action: "restore", Status: "succeeded", Stage: "complete", BackupID: "backup-owned", PreviousDatabase: "app", CandidateDatabase: "luci_restore_current", CreatedAt: now(), UpdatedAt: now()}
			f.candidates[op.CandidateDatabase] = true
			tc.change(s, f, &op)
			if err := s.update("owned", func(v *ProjectState) {
				v.Database.Database = "luci_restore_current"
				v.DatabaseRestoreHistory = []DatabaseOperation{op}
			}); err != nil {
				t.Fatal(err)
			}
			w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]any{"action": "rollback-restore", "restoreId": "database-op-prior", "confirmRestore": true, "acknowledgeResources": true})
			if tc.accepted {
				if w.Code != 202 {
					t.Fatal("valid history not accepted for actual SQL verification")
				}
				v := waitDatabaseFixtureOperation(t, s)
				if v.Database.Database != "luci_restore_current" || v.DatabaseOperation.Status != "failed" || v.Database.Status != "stopped" {
					t.Fatal("unhealthy previous selection promoted or failed to retain original")
				}
			} else {
				if w.Code != 409 {
					t.Fatalf("invalid history status %d", w.Code)
				}
				for _, args := range f.commands {
					if args[0] != "inspect" && !(args[0] == "volume" && args[1] == "inspect") {
						t.Fatal("invalid rollback mutated resources")
					}
				}
			}
		})
	}
}

func TestDatabaseRestoredSelectionRefusesStaleApplicationRestart(t *testing.T) {
	for _, source := range []string{"retained-private-environment", "actual-owned-container-environment"} {
		t.Run(source, func(t *testing.T) {
			s, f, _ := newRestoreDatabaseFixture(t)
			d := Deployment{ID: "deploy-owned", ProjectID: "owned", Provider: "docker", Container: "app-container", Status: "stopped"}
			if err := s.saveDeployment(d); err != nil {
				t.Fatal(err)
			}
			if err := s.update("owned", func(v *ProjectState) { v.Database.Database = "luci_restore_current" }); err != nil {
				t.Fatal(err)
			}
			oldURL := "postgresql://luci:" + strings.Repeat("a", 24) + "@database:5432/app?sslmode=disable"
			if source == "retained-private-environment" {
				if err := s.write(d.ID+"-redactions", map[string]string{"DATABASE_URL": oldURL}); err != nil {
					t.Fatal(err)
				}
			} else {
				base := s.opts.Runner
				s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
					data, err := base(ctx, exe, args, dir, input)
					if err == nil && args[0] == "inspect" && args[1] == "app-container" {
						var all []containerInfo
						if err = json.Unmarshal(data, &all); err != nil {
							return nil, err
						}
						all[0].Config.Env = []string{"DATABASE_URL=" + oldURL}
						return json.Marshal(all)
					}
					return data, err
				}
			}
			w := request(t, s, "POST", "/v1/platform/projects/owned/deployments/deploy-owned/restart", "member", map[string]any{})
			if w.Code != 409 || !strings.Contains(w.Body.String(), "database_redeployment_required") || strings.Contains(w.Body.String(), oldURL) || strings.Contains(w.Body.String(), strings.Repeat("a", 24)) {
				t.Fatal("stale automatic binding was allowed or leaked")
			}
			for _, args := range f.commands {
				if args[0] != "inspect" {
					t.Fatal("stale app was mutated before redeployment refusal")
				}
			}
		})
	}
}

func TestDatabaseRestartBindingKeepsUnrelatedApplicationsAndCurrentBinding(t *testing.T) {
	s, _, _ := newRestoreDatabaseFixture(t)
	d := Deployment{ID: "deploy-owned", ProjectID: "owned", Provider: "docker", Container: "app-container", Status: "stopped"}
	if err := s.update("owned", func(v *ProjectState) { v.Database.Database = "luci_restore_current" }); err != nil {
		t.Fatal(err)
	}
	for _, binding := range []string{"", "postgresql://user:private@other:5432/app", "postgresql://luci:" + strings.Repeat("a", 24) + "@database:5432/luci_restore_current?sslmode=disable"} {
		if err := s.write(d.ID+"-redactions", map[string]string{"DATABASE_URL": binding}); err != nil {
			t.Fatal(err)
		}
		if err := s.checkDeploymentDatabaseBinding("owned", d, containerInfo{}); err != nil {
			t.Fatal("unrelated/current application was refused")
		}
	}
}

func TestDatabaseRestoreCanRecoverMissingLogicalDatabase(t *testing.T) {
	s, f, backup := newRestoreDatabaseFixture(t)
	f.candidates["app"] = false
	if w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", restoreRequest(backup.ID)); w.Code != 202 {
		t.Fatal("retained server backup was not accepted")
	}
	v := waitDatabaseFixtureOperation(t, s)
	if v.DatabaseOperation.Status != "succeeded" || v.Database.Database == "app" {
		t.Fatal("restore incorrectly required the missing old logical database to be healthy")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	maintenance := false
	for _, args := range f.commands {
		if args[0] == "exec" && args[2] == "psql" && args[len(args)-1] == "SELECT 1" {
			if args[6] != "postgres" {
				t.Error("recovery readiness still targeted a broken old database")
			}
			maintenance = true
		}
	}
	if !maintenance {
		t.Fatal("retained server maintenance readiness was not checked")
	}
}

func TestDatabaseStopDoesNotClaimSuccessBeforeObservedStop(t *testing.T) {
	s, f, _ := newRestoreDatabaseFixture(t)
	f.running = true
	base := s.opts.Runner
	s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		if args[0] == "stop" {
			return nil, nil
		} // A transport success alone is insufficient.
		return base(ctx, exe, args, dir, input)
	}
	if err := s.update("owned", func(v *ProjectState) { v.Database.Status = "unknown" }); err != nil {
		t.Fatal(err)
	}
	w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "stop"})
	if w.Code != 502 || databaseFixtureState(t, s).Database.Status != "unknown" {
		t.Fatal("unobserved Stop was falsely reported successful")
	}
}
