package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const databaseBackupLimit = 64 << 20

var databaseName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var databasePassword = regexp.MustCompile(`^[a-f0-9]{24}$`)
var databaseDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validDatabasePassword(password string) bool { return databasePassword.MatchString(password) }

func activeDatabaseName(db Database) (string, error) {
	name := db.Database
	if name == "" {
		name = "app" // The original persisted PostgreSQL installation used app.
	}
	if !databaseName.MatchString(name) {
		return "", errors.New("invalid active database name")
	}
	return name, nil
}

// A recorded backup is admitted by project history, private path, regular-file
// identity and immutable bytes. The size bound also applies to legacy records.
func (s *Service) databaseBackupBytes(backup Backup) ([]byte, error) {
	if !identifier.MatchString(backup.ID) || backup.Bytes < 5 || backup.Bytes > databaseBackupLimit || !databaseDigest.MatchString(backup.SHA256) {
		return nil, errors.New("invalid backup receipt")
	}
	file, err := s.path(backup.ID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != backup.Bytes {
		return nil, errors.New("backup size or type changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, backup.Bytes+1))
	if err != nil || int64(len(data)) != backup.Bytes || string(data[:5]) != "PGDMP" {
		return nil, errors.New("backup bytes changed")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != backup.SHA256 {
		return nil, errors.New("backup digest changed")
	}
	return data, nil
}

func observeInterruptedDatabaseRestore(v *ProjectState) {
	if v.DatabaseOperation == nil || v.DatabaseOperation.Status != "running" {
		return
	}
	op := *v.DatabaseOperation
	op.Status, op.Stage = "interrupted", "interrupted"
	op.Message = "Luci stopped during restore. Previous data and candidate databases are retained. Explicitly stop PostgreSQL before retrying; no restore is automatically repeated."
	op.UpdatedAt = now()
	setDatabaseOperation(v, op)
	v.Database.Status = "unknown"
	v.Database.Message = op.Message
}

func setDatabaseOperation(v *ProjectState, op DatabaseOperation) {
	v.DatabaseOperation = &op
	for i := range v.DatabaseRestoreHistory {
		if v.DatabaseRestoreHistory[i].ID == op.ID {
			v.DatabaseRestoreHistory[i] = op
			return
		}
	}
	v.DatabaseRestoreHistory = append([]DatabaseOperation{op}, v.DatabaseRestoreHistory...)
}

func (s *Service) ownedRestoreDatabase(ctx context.Context, id, name string) (containerInfo, error) {
	if name != s.resource(id, "postgres") {
		return containerInfo{}, errors.New("invalid database container")
	}
	info, err := s.inspect(ctx, id, "postgres", name)
	if err != nil || info.Config.Image != postgresImage {
		return containerInfo{}, errors.New("database ownership or image changed")
	}
	var secret databaseSecret
	if err := s.read(id+"-database-secret", &secret); err != nil || !validDatabasePassword(secret.Password) {
		return containerInfo{}, errors.New("database credential state unavailable")
	}
	credential, user := false, false
	for _, env := range info.Config.Env {
		if env == "POSTGRES_PASSWORD="+secret.Password {
			credential = true
		}
		if env == "POSTGRES_USER=luci" {
			user = true
		}
	}
	if !credential || !user {
		return containerInfo{}, errors.New("database credential binding changed")
	}
	volume := s.resource(id, "postgres-data")
	mount := false
	for _, m := range info.Mounts {
		if m.Destination == "/var/lib/postgresql/data" && m.Type == "volume" && m.Name == volume && m.RW {
			mount = true
		}
	}
	if !mount {
		return containerInfo{}, errors.New("database volume changed")
	}
	data, err := s.docker(ctx, "volume", "inspect", volume, "--format", "{{json .Labels}}")
	var labels map[string]string
	if err != nil || json.Unmarshal(data, &labels) != nil || labels[labelInstance] != s.instance || labels[labelProject] != id || labels[labelResource] != "postgres-data" {
		return containerInfo{}, errors.New("database volume ownership changed")
	}
	return info, nil
}

func (s *Service) acceptDatabaseRestore(w http.ResponseWriter, r *http.Request, id, backupID string) {
	s.acceptDatabaseRecovery(w, r, id, backupID, "")
}

func (s *Service) acceptDatabaseRecovery(w http.ResponseWriter, r *http.Request, id, backupID, restoreID string) {
	if !s.claim(id, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return
	}
	accepted := false
	defer func() {
		if !accepted {
			s.release(id)
		}
	}()
	s.mu.Lock()
	state, err := s.load(id)
	s.mu.Unlock()
	if err != nil {
		problem(w, 500, "state_unavailable", "Database history could not be read.")
		return
	}
	previous, err := activeDatabaseName(state.Database)
	if err != nil || state.Database.Status != "stopped" {
		problem(w, 409, "database_stop_required", "Stop the existing PostgreSQL database before restoring. Its data is retained.")
		return
	}
	var backup *Backup
	var prior *DatabaseOperation
	if restoreID != "" {
		for _, entry := range state.DatabaseRestoreHistory {
			if entry.ID == restoreID && entry.Status == "succeeded" && entry.Stage == "complete" && (entry.Action == "restore" || entry.Action == "rollback-restore") && entry.CandidateDatabase == previous && entry.PreviousDatabase != previous && databaseName.MatchString(entry.PreviousDatabase) {
				copy := entry
				prior = &copy
				break
			}
		}
		if prior == nil {
			problem(w, 409, "restore_history_unavailable", "Choose a successful restore belonging to this project's active database. No database selection was changed.")
			return
		}
	} else {
		for _, b := range state.Backups {
			if b.ID == backupID {
				copy := b
				backup = &copy
				break
			}
		}
		if backup == nil {
			problem(w, 404, "backup_unavailable", "Choose a saved backup belonging to this project.")
			return
		}
		if _, err = s.databaseBackupBytes(*backup); err != nil {
			problem(w, 409, "backup_changed", "The private backup no longer matches its recorded size and checksum. No restore was started.")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	info, err := s.ownedRestoreDatabase(ctx, id, state.Database.Container)
	if err != nil {
		problem(w, 409, "database_unverified", "The retained PostgreSQL container, pinned image or private volume could not be verified. No restore was started.")
		return
	}
	if info.State.Running {
		problem(w, 409, "database_stop_required", "The owned PostgreSQL container is still running. Stop it before restoring.")
		return
	}
	for _, d := range state.Deployments {
		if d.Runtime != nil {
			if d.Runtime.ResourcesHeld {
				problem(w, 409, "applications_stop_required", "Stop this project's retained application allocations before restoring. Unknown outcomes keep their resources reserved.")
				return
			}
			// Typed cleanup can remove its exact container. A settled allocation
			// must not fall back to inspecting a historical container name.
			continue
		}
		if d.Provider != "docker" {
			continue
		}
		if d.Status == "ready" || d.Status == "building" || d.Status == "starting" {
			problem(w, 409, "applications_stop_required", "Stop this project's Docker applications before restoring. Redeploy them after restore to use the selected database.")
			return
		}
		if d.Container != "" {
			app, e := s.inspect(ctx, id, d.ID, d.Container)
			if e != nil || app.State.Running {
				problem(w, 409, "applications_stop_required", "An owned Docker application's stopped state could not be verified. Verify and stop it before restoring.")
				return
			}
		}
	}
	if err := ctx.Err(); err != nil {
		problem(w, 409, "database_unverified", "Database verification was interrupted. No restore was started.")
		return
	}
	op := DatabaseOperation{ID: randomID("database-op-"), Action: "restore", Status: "running", Stage: "queued", BackupID: backupID, PreviousDatabase: previous, CandidateDatabase: randomID("luci_restore_"), CreatedAt: now(), UpdatedAt: now(), Message: "Restore accepted. A separate database will be created in the retained volume; previous data is preserved."}
	if prior != nil {
		op.Action, op.RestoreID = "rollback-restore", prior.ID
		op.BackupID, op.CandidateDatabase = prior.BackupID, prior.PreviousDatabase
		op.Message = "Previous-database recovery accepted. Its SQL health will be verified before selection; all databases are retained."
	}
	if err = s.update(id, func(v *ProjectState) {
		observeInterruptedDatabaseRestore(v)
		setDatabaseOperation(v, op)
		v.Database.Status = "restoring"
		v.Database.Message = op.Message
	}); err != nil {
		problem(w, 500, "state_unavailable", "The restore operation could not be saved. No restore was started.")
		return
	}
	limits := s.limits(r)
	s.wg.Add(1)
	accepted = true
	go func() {
		defer s.wg.Done()
		defer s.release(id)
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()
		var err error
		if backup != nil {
			err = s.restoreDatabase(ctx, id, *backup, op, limits)
		} else {
			err = s.rollbackDatabaseRestore(ctx, id, op, limits)
		}
		if err != nil {
			s.failDatabaseRestore(id, op, err)
		}
	}()
	respond(w, 202, op)
}

func (s *Service) databaseRestoreStage(id string, op *DatabaseOperation, stage, message string) error {
	op.Stage, op.Message, op.UpdatedAt = stage, message, now()
	return s.update(id, func(v *ProjectState) {
		setDatabaseOperation(v, *op)
		v.Database.Message = message
	})
}

func (s *Service) restoreDatabase(ctx context.Context, id string, backup Backup, op DatabaseOperation, limits resourceLimits) error {
	data, err := s.databaseBackupBytes(backup)
	if err != nil {
		return &databaseRestoreFailure{"The private backup changed before restore. No candidate was promoted."}
	}
	if err = s.databaseRestoreStage(id, &op, "starting", "Starting only the owned PostgreSQL container. Previous data remains selected."); err != nil {
		return err
	}
	db, err := s.startRecoveryDatabase(ctx, id, op.PreviousDatabase, limits)
	if err != nil {
		return err
	}
	if err = s.databaseRestoreStage(id, &op, "restoring", "Restoring the verified backup into a separate database. Original and prior restored databases are retained."); err != nil {
		return err
	}
	// Generated SQL identifiers are passed as argv, never interpolated into SQL
	// or a shell. No --create is used: the dump cannot choose the target database.
	if _, err = s.docker(ctx, "exec", db.Container, "createdb", "-U", "luci", "--template=template0", op.CandidateDatabase); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &databaseRestoreFailure{"The separate restore database could not be created. Previous data remains selected."}
	}
	// Roles remain in this same PostgreSQL cluster. Retaining ownership and ACLs
	// preserves grants and SECURITY DEFINER semantics from the owned backup.
	if _, err = s.opts.Runner(ctx, "docker", []string{"exec", "--interactive", db.Container, "pg_restore", "-U", "luci", "--dbname=" + op.CandidateDatabase, "--single-transaction", "--exit-on-error"}, s.dir, data); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &databaseRestoreFailure{"PostgreSQL could not restore the backup transaction. Previous data and the unpromoted candidate are retained."}
	}
	return s.promoteRecoveryDatabase(ctx, id, db, op)
}

func (s *Service) rollbackDatabaseRestore(ctx context.Context, id string, op DatabaseOperation, limits resourceLimits) error {
	if err := s.databaseRestoreStage(id, &op, "starting", "Starting the owned PostgreSQL container to verify the retained previous database."); err != nil {
		return err
	}
	db, err := s.startRecoveryDatabase(ctx, id, op.PreviousDatabase, limits)
	if err != nil {
		return err
	}
	return s.promoteRecoveryDatabase(ctx, id, db, op)
}

func (s *Service) startRecoveryDatabase(ctx context.Context, id, previous string, limits resourceLimits) (Database, error) {
	s.mu.Lock()
	state, err := s.load(id)
	s.mu.Unlock()
	if err != nil {
		return Database{}, err
	}
	db := state.Database
	info, err := s.ownedRestoreDatabase(ctx, id, db.Container)
	if err != nil || info.State.Running {
		return db, errors.New("database changed before recovery")
	}
	args := append([]string{"update"}, limits.args()...)
	args = append(args, db.Container)
	if _, err = s.docker(ctx, args...); err != nil {
		return db, err
	}
	if _, err = s.docker(ctx, "start", db.Container); err != nil {
		return db, err
	}
	// Recovery does not depend on the old logical database being healthy. The
	// PostgreSQL maintenance database verifies the retained server itself.
	db.Database = "postgres"
	db, err = s.waitDatabase(ctx, id, db)
	if err != nil {
		return db, err
	}
	info, err = s.ownedRestoreDatabase(ctx, id, db.Container)
	if err != nil || !info.State.Running {
		return db, errors.New("database changed before recovery")
	}
	db.Database = previous
	return db, nil
}

func (s *Service) promoteRecoveryDatabase(ctx context.Context, id string, db Database, op DatabaseOperation) error {
	if err := s.databaseRestoreStage(id, &op, "checking", "Checking the candidate database before switching the active selection."); err != nil {
		return err
	}
	info, err := s.ownedRestoreDatabase(ctx, id, db.Container)
	if err != nil || !info.State.Running {
		return errors.New("database changed before health check")
	}
	// current_database proves that the verified SQL response belongs to the
	// candidate rather than the previously healthy database.
	// Official PostgreSQL exposes POSTGRES_PASSWORD only inside the owned
	// container. A fixed wrapper gives psql that environment privately and uses
	// quoted argv, rather than putting a credential into Docker arguments. TCP
	// checks the same password binding that new application deployments use.
	result, err := s.docker(ctx, "exec", db.Container, "sh", "-c", `PGPASSWORD="$POSTGRES_PASSWORD" exec "$@"`, "luci-postgres", "psql", "-h", "127.0.0.1", "-U", "luci", "-d", op.CandidateDatabase, "-Atqc", "SELECT current_database()")
	if err != nil || strings.TrimSpace(string(result)) != op.CandidateDatabase {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &databaseRestoreFailure{"The candidate database did not pass its SQL health check. Previous data remains selected."}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	info, err = s.ownedRestoreDatabase(ctx, id, db.Container)
	if err != nil || !info.State.Running {
		return errors.New("database changed after health check")
	}
	if err = s.databaseRestoreStage(id, &op, "promoting", "Restore and SQL checks passed. Saving the active database selection."); err != nil {
		return err
	}
	db.Database = op.CandidateDatabase
	db.Status = "ready"
	db.Message = "The verified restored database is active. Previous databases and the original volume are retained. Docker applications remain stopped; redeploy them to use the restored database."
	op.Status, op.Stage, op.Message, op.UpdatedAt = "succeeded", "complete", db.Message, now()
	// The only promotion is this atomic private-state replacement. No database
	// or volume is renamed, dropped or removed, and credentials/alias stay stable.
	return s.update(id, func(v *ProjectState) {
		v.Database = db
		setDatabaseOperation(v, op)
	})
}

type databaseRestoreFailure struct{ message string }

func (e *databaseRestoreFailure) Error() string { return e.message }

func (s *Service) failDatabaseRestore(id string, op DatabaseOperation, failure error) {
	// Close/cancellation still performs bounded rollback of only the verified
	// owned container. It never deletes the volume or an unpromoted database.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := s.resource(id, "postgres")
	info, err := s.ownedRestoreDatabase(ctx, id, name)
	stopped := err == nil && !info.State.Running
	if err == nil && info.State.Running {
		if _, err = s.docker(ctx, "stop", name); err == nil {
			info, err = s.ownedRestoreDatabase(ctx, id, name)
			stopped = err == nil && !info.State.Running
		}
	}
	message := "Restore failed. Previous data and candidate databases are retained. Check Docker capacity, backup validity and available disk before retrying."
	var public *databaseRestoreFailure
	if errors.As(failure, &public) {
		message = public.message
	}
	op.Status, op.Stage = "failed", "failed"
	if s.ctx.Err() != nil || errors.Is(failure, context.Canceled) {
		op.Status, op.Stage = "interrupted", "interrupted"
		message = "Restore was interrupted. Previous data and candidates are retained; no operation is automatically repeated."
	} else if errors.Is(failure, context.DeadlineExceeded) {
		message = "Restore exceeded its five-minute limit. Previous data and candidates are retained. Verify capacity and disk before retrying."
	}
	if !stopped {
		message += " PostgreSQL's stopped state could not be verified. Explicitly stop and verify it before retrying."
	}
	op.Message, op.UpdatedAt = message, now()
	_ = s.update(id, func(v *ProjectState) {
		// Read the latest progress instead of replacing it with the queued copy.
		if v.DatabaseOperation != nil && v.DatabaseOperation.ID == op.ID {
			op.PreviousDatabase = v.DatabaseOperation.PreviousDatabase
		}
		setDatabaseOperation(v, op)
		v.Database.Status = "unknown"
		if stopped {
			v.Database.Status = "stopped"
		}
		v.Database.Message = message
	})
}

// The private saved environment covers existing deployments predating restore.
// Actual owned-container environment is also inspected when present. Neither
// the URL nor its credentials leave this function or appear in public errors.
func (s *Service) checkDeploymentDatabaseBinding(id string, d Deployment, info containerInfo) error {
	var secret databaseSecret
	if err := s.read(id+"-database-secret", &secret); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil || !validDatabasePassword(secret.Password) {
		return errors.New("database binding unavailable")
	}
	s.mu.Lock()
	state, err := s.load(id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	active, err := activeDatabaseName(state.Database)
	if err != nil {
		return err
	}
	var values map[string]string
	if err := s.read(d.ID+"-redactions", &values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	bindings := []string{values["DATABASE_URL"]}
	for _, env := range info.Config.Env {
		if value, ok := strings.CutPrefix(env, "DATABASE_URL="); ok {
			bindings = append(bindings, value)
		}
	}
	for _, binding := range bindings {
		u, err := url.Parse(binding)
		if err != nil || u.Scheme != "postgresql" || u.Host != "database:5432" || u.User == nil || u.User.Username() != "luci" {
			continue // Applications with another database binding are unaffected.
		}
		password, _ := u.User.Password()
		if password != secret.Password {
			continue
		}
		if u.Path != "/"+active || u.RawPath != "" {
			return errors.New("retained application uses another database")
		}
	}
	return nil
}
