package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
)

// Import records bytes under a new private, project-owned identity. It never
// interprets archive SQL, creates a database or implies that restore succeeded.
// Restoring remains a separate, explicitly confirmed operation.
func (s *Service) importDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	if r.Header.Get("X-Luci-Trust-Backup") != "true" {
		problem(w, 400, "backup_trust_required", "Confirm that you trust this PostgreSQL dump. Restoring it can execute SQL supplied by its author.")
		return
	}
	expected := r.Header.Get("X-Luci-Backup-SHA256")
	if !databaseDigest.MatchString(expected) {
		problem(w, 400, "backup_checksum_required", "Provide the SHA-256 checksum of the selected dump before importing it.")
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/octet-stream" {
		problem(w, 415, "invalid_backup_type", "Upload the PostgreSQL custom-format dump as application/octet-stream.")
		return
	}
	if r.ContentLength > databaseBackupLimit {
		problem(w, 413, "backup_too_large", "PostgreSQL imports are limited to 64 MiB.")
		return
	}
	if !s.claim(p.ID, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return
	}
	defer s.release(p.ID)
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, databaseBackupLimit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			problem(w, 413, "backup_too_large", "PostgreSQL imports are limited to 64 MiB.")
		} else {
			problem(w, 400, "backup_upload_failed", "The backup upload did not complete. No backup was recorded.")
		}
		return
	}
	if r.Context().Err() != nil {
		problem(w, 400, "backup_upload_failed", "The backup upload was interrupted. No backup was recorded.")
		return
	}
	if len(data) < 5 || string(data[:5]) != "PGDMP" {
		problem(w, 400, "invalid_backup", "Choose a PostgreSQL custom-format dump. SQL text, ZIP files and Supabase archives are not supported here.")
		return
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		problem(w, 409, "backup_checksum_mismatch", "The uploaded dump does not match the expected checksum. No backup was recorded.")
		return
	}
	s.mu.Lock()
	state, err := s.load(p.ID)
	s.mu.Unlock()
	if err != nil {
		problem(w, 500, "state_unavailable", "Backup history could not be read.")
		return
	}
	// A transport retry must not consume storage again. A missing or changed
	// existing archive is reported; importing never silently replaces it.
	for _, existing := range state.Backups {
		if existing.SHA256 == expected && existing.Bytes == int64(len(data)) {
			if _, err := s.databaseBackupBytes(existing); err != nil {
				problem(w, 409, "backup_changed", "A recorded backup with this checksum is missing or changed. Resolve its private storage before retrying.")
				return
			}
			respond(w, 200, existing)
			return
		}
	}
	backup, err := s.storeDatabaseBackup(p.ID, data, "", true)
	if err != nil {
		problem(w, 500, "backup_store_failed", "The uploaded backup could not be saved privately. No database was changed.")
		return
	}
	respond(w, 201, backup)
}

func (s *Service) storeDatabaseBackup(id string, data []byte, database string, imported bool) (Backup, error) {
	b := Backup{ID: randomID("backup-"), CreatedAt: now(), Bytes: int64(len(data)), Database: database, Imported: imported}
	sum := sha256.Sum256(data)
	b.SHA256 = hex.EncodeToString(sum[:])
	file, err := s.path(b.ID)
	if err != nil {
		return b, err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return b, err
	}
	if err = protect(file); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = s.update(id, func(v *ProjectState) { v.Backups = append([]Backup{b}, v.Backups...) })
	}
	if err != nil {
		_ = os.Remove(file)
	}
	return b, err
}
