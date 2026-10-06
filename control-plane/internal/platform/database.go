package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Official Docker Library PostgreSQL 16.15 bookworm multi-platform manifest,
// resolved 2026-10-03 from hub.docker.com/v2/repositories/library/postgres/tags/16.15-bookworm.
const postgresImage = "postgres:16.15-bookworm@sha256:efedf3595f1d6f415c08568ba171029bf54052e754cc9f030e3f2412b21f3d67"

type databaseSecret struct {
	Password string `json:"password"`
}

func (s *Service) databaseAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	var in struct {
		Action               string `json:"action"`
		BackupID             string `json:"backupId,omitempty"`
		RestoreID            string `json:"restoreId,omitempty"`
		ConfirmRestore       bool   `json:"confirmRestore,omitempty"`
		AcknowledgeResources bool   `json:"acknowledgeResources,omitempty"`
	}
	if !body(w, r, &in) {
		return
	}
	if in.Action != "start" && in.Action != "stop" && in.Action != "backup" && in.Action != "restore" && in.Action != "rollback-restore" {
		problem(w, 400, "invalid_action", "Choose start, stop, backup, restore or rollback-restore.")
		return
	}
	if in.Action == "restore" || in.Action == "rollback-restore" {
		selected := in.BackupID
		if in.Action == "rollback-restore" {
			selected = in.RestoreID
		}
		if !in.ConfirmRestore || !identifier.MatchString(selected) {
			problem(w, 400, "restore_confirmation_required", "Choose a saved backup or successful restore and confirm switching the active database. Previous data is retained.")
			return
		}
		if !in.AcknowledgeResources {
			problem(w, 400, "resource_acknowledgement_required", "A restore starts PostgreSQL with this workspace's service limits and requires disk space for an additional database. Acknowledge those resources.")
			return
		}
		if in.Action == "rollback-restore" {
			s.acceptDatabaseRecovery(w, r, p.ID, "", in.RestoreID)
		} else {
			s.acceptDatabaseRestore(w, r, p.ID, in.BackupID)
		}
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return
	}
	defer s.release(p.ID)
	if e := s.update(p.ID, observeInterruptedDatabaseRestore); e != nil {
		problem(w, 500, "state_unavailable", "Database status could not be read.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if in.Action == "backup" {
		b, e := s.backupDatabase(ctx, p.ID)
		if e != nil {
			problem(w, 502, "backup_failed", "The owned PostgreSQL database could not be backed up. It must be running and fit within the 64 MiB download limit.")
			return
		}
		respond(w, 201, b)
		return
	}
	var db Database
	var e error
	if in.Action == "start" {
		db, e = s.startDatabase(ctx, p.ID, s.limits(r))
	} else {
		db, e = s.stopDatabase(ctx, p.ID)
	}
	if e != nil {
		problem(w, 502, "database_failed", "The owned PostgreSQL operation failed. Verify Docker readiness and available memory; existing data is retained.")
		return
	}
	if e = s.update(p.ID, func(v *ProjectState) { v.Database = db }); e != nil {
		problem(w, 500, "state_unavailable", "Database status could not be saved.")
		return
	}
	respond(w, 200, db)
}
func (s *Service) startDatabase(ctx context.Context, id string, limits resourceLimits) (Database, error) {
	name := s.resource(id, "postgres")
	s.mu.Lock()
	state, e := s.load(id)
	s.mu.Unlock()
	if e != nil {
		return Database{}, e
	}
	active, e := activeDatabaseName(state.Database)
	if e != nil || (state.Database.Container != "" && state.Database.Container != name) {
		return Database{}, errors.New("invalid database selection")
	}
	db := Database{Status: "starting", Engine: "PostgreSQL 16.15", Container: name, Database: active, Username: "luci", Message: "Starting persistent PostgreSQL."}
	secretName := id + "-database-secret"
	var secret databaseSecret
	e = s.read(secretName, &secret)
	if errors.Is(e, os.ErrNotExist) {
		secret.Password = randomID("")
		if e = s.write(secretName, secret); e != nil {
			return db, e
		}
	} else if e != nil {
		return db, e
	}
	if !validDatabasePassword(secret.Password) {
		return db, errors.New("database credential state is invalid")
	}
	// Only a verified owned container may be resumed. A name collision cannot
	// authorize takeover or removal of another workload.
	b, inspectErr := s.docker(ctx, "inspect", name)
	if inspectErr == nil {
		var found []containerInfo
		if json.Unmarshal(b, &found) != nil || len(found) != 1 {
			return db, errors.New("invalid inspection")
		}
		if _, e = s.inspect(ctx, id, "postgres", name); e != nil {
			return db, e
		}
		args := append([]string{"update"}, limits.args()...)
		args = append(args, name)
		if _, e = s.docker(ctx, args...); e != nil {
			return db, e
		}
		if _, e = s.docker(ctx, "start", name); e != nil {
			return db, e
		}
	} else {
		network, e := s.ensureNetwork(ctx, id)
		if e != nil {
			return db, e
		}
		volume := s.resource(id, "postgres-data")
		b, e = s.docker(ctx, "volume", "inspect", volume, "--format", "{{json .Labels}}")
		if e == nil {
			var labels map[string]string
			if json.Unmarshal(b, &labels) != nil || labels[labelInstance] != s.instance || labels[labelProject] != id {
				return db, errors.New("volume ownership mismatch")
			}
		} else {
			args := append([]string{"volume", "create"}, s.labels(id, "postgres-data")...)
			args = append(args, volume)
			if _, e = s.docker(ctx, args...); e != nil {
				return db, e
			}
		}
		file, e := s.envFile(map[string]string{"POSTGRES_PASSWORD": secret.Password, "POSTGRES_USER": "luci", "POSTGRES_DB": "app"})
		if e != nil {
			return db, e
		}
		defer os.Remove(file)
		args := []string{"run", "--detach", "--name", name, "--restart", "unless-stopped", "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER", "--cap-add", "SETUID", "--cap-add", "SETGID", "--security-opt", "no-new-privileges", "--network", network, "--network-alias", "database", "--publish", "127.0.0.1::5432", "--mount", "type=volume,source=" + volume + ",target=/var/lib/postgresql/data", "--env-file", file}
		args = append(args, limits.args()...)
		args = append(args, s.labels(id, "postgres")...)
		args = append(args, postgresImage)
		if _, e = s.docker(ctx, args...); e != nil {
			return db, e
		}
		_ = os.Remove(file)
	}
	return s.waitDatabase(ctx, id, db)
}
func (s *Service) waitDatabase(ctx context.Context, id string, db Database) (Database, error) {
	for {
		info, e := s.inspect(ctx, id, "postgres", db.Container)
		if e != nil {
			return db, e
		}
		if !info.State.Running {
			return db, errors.New("database container stopped")
		}
		b, e := s.docker(ctx, "exec", db.Container, "psql", "-U", "luci", "-d", db.Database, "-Atqc", "SELECT 1")
		if e == nil && strings.TrimSpace(string(b)) == "1" {
			address, e := loopbackURL(info, 5432)
			if e != nil {
				return db, e
			}
			db.Host = "127.0.0.1"
			db.Port, _ = strconv.Atoi(strings.TrimPrefix(address, "http://127.0.0.1:"))
			db.Status = "ready"
			db.Message = "PostgreSQL answered SELECT 1. New Docker deployments receive its private DATABASE_URL; existing deployments require redeployment."
			return db, nil
		}
		select {
		case <-ctx.Done():
			return db, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
func (s *Service) stopDatabase(ctx context.Context, id string) (Database, error) {
	s.mu.Lock()
	state, e := s.load(id)
	s.mu.Unlock()
	if e != nil {
		return Database{}, e
	}
	db := state.Database
	if _, e = s.inspect(ctx, id, "postgres", db.Container); e != nil {
		return db, e
	}
	if _, e = s.docker(ctx, "stop", db.Container); e != nil {
		return db, e
	}
	info, e := s.inspect(ctx, id, "postgres", db.Container)
	if e != nil || info.State.Running {
		return db, errors.New("database stopped state unverified")
	}
	db.Status = "stopped"
	db.Message = "PostgreSQL stopped. Its private data volume is retained."
	return db, nil
}
func (s *Service) backupDatabase(ctx context.Context, id string) (Backup, error) {
	name := s.resource(id, "postgres")
	s.mu.Lock()
	state, e := s.load(id)
	s.mu.Unlock()
	if e != nil {
		return Backup{}, e
	}
	active, e := activeDatabaseName(state.Database)
	if e != nil {
		return Backup{}, e
	}
	info, e := s.inspect(ctx, id, "postgres", name)
	if e != nil || !info.State.Running {
		return Backup{}, errors.New("database unavailable")
	}
	data, e := s.docker(ctx, "exec", name, "pg_dump", "-U", "luci", "-d", active, "--format=custom")
	if e != nil || len(data) < 5 || len(data) > databaseBackupLimit || string(data[:5]) != "PGDMP" {
		return Backup{}, errors.New("invalid database backup")
	}
	return s.storeDatabaseBackup(id, data, active, false)
}
func (s *Service) downloadBackup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	state, e := s.load(p.ID)
	s.mu.Unlock()
	if e != nil {
		problem(w, 500, "state_unavailable", "Backup history is unavailable.")
		return
	}
	var selected *Backup
	for _, b := range state.Backups {
		if b.ID == r.PathValue("backupId") {
			copy := b
			selected = &copy
			break
		}
	}
	if selected == nil {
		problem(w, 404, "backup_unavailable", "This backup is unavailable in your project.")
		return
	}
	data, e := s.databaseBackupBytes(*selected)
	if e != nil {
		problem(w, 409, "backup_changed", "The backup no longer matches its recorded checksum.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+selected.ID+`.dump"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
